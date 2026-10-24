//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

var _ = time.Second        // silence unused import
var _ homeassistant.Client // silence unused import

type TemplateHelperIntegrationTestSuite struct {
	HelperTestSuite
}

func TestTemplateHelperIntegration(t *testing.T) {
	suite.Run(t, new(TemplateHelperIntegrationTestSuite))
}

func (s *TemplateHelperIntegrationTestSuite) TestTemplateSensorLifecycle() {
	// Create an input_number to use in template
	sourceName := GenerateTestID("tmpl_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	templateName := GenerateTestID("tmpl_sensor")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_number
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":                sourceName,
			"min":                 0.0,
			"max":                 100.0,
			"initial":             20.0,
			"unit_of_measurement": "°C",
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	// Create template sensor that reads the source value
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":                templateName,
			"state":               "{{ states('" + sourceEntityID + "') | float }}",
			"unit_of_measurement": "°C",
			"device_class":        "temperature",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")
	s.NotNil(entity)

	// Verify the template sensor was created (don't check exact state value due to timing)
	s.NotEmpty(entity.State, "Template sensor should have a state")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), templateEntityID)
	s.Require().NoError(err, "Failed to delete template sensor")

	err = s.WaitForEntityGone(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor should be deleted")

	// Cleanup source
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *TemplateHelperIntegrationTestSuite) TestTemplateBinarySensorLifecycle() {
	// Create an input_number to use in template
	sourceName := GenerateTestID("tmpl_bin_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	templateName := GenerateTestID("tmpl_binary")
	templateEntityID := BuildEntityID("binary_sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_number
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     100.0,
			"initial": 30.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	// Create template binary sensor (on when source > 50)
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":         templateName,
			"state":        "{{ states('" + sourceEntityID + "') | float > 50 }}",
			"device_class": "problem",
			"type":         "binary_sensor",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template binary sensor")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template binary sensor did not appear")
	s.Equal("off", entity.State, "Binary sensor should be off when source (30) <= 50")

	// Set source above threshold
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     75.0,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), templateEntityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "Binary sensor should be on when source (75) > 50")

	// Set source below threshold
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     40.0,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), templateEntityID)
	s.Require().NoError(err)
	s.Equal("off", entity.State, "Binary sensor should be off when source (40) <= 50")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), templateEntityID)
	s.Require().NoError(err, "Failed to delete template binary sensor")

	err = s.WaitForEntityGone(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template binary sensor should be deleted")

	// Cleanup source
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *TemplateHelperIntegrationTestSuite) TestTemplateSensorWithStateClass() {
	sourceName := GenerateTestID("tmpl_sc_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	templateName := GenerateTestID("tmpl_state_cls")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     1000.0,
			"initial": 100.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create template sensor with state_class for long-term statistics
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":                templateName,
			"state":               "{{ states('" + sourceEntityID + "') | float }}",
			"unit_of_measurement": "kWh",
			"device_class":        "energy",
			"state_class":         "total_increasing",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")
	s.NotNil(entity)

	// Verify state class attribute if present
	if stateClass, ok := entity.Attributes["state_class"].(string); ok {
		s.Equal("total_increasing", stateClass)
	}

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

// TestTemplateBinarySensorDelayRoundTrip pins issue #292: a template
// binary_sensor's delay_on/delay_off are settable through manage_helper, live
// inside an HA "expandable" options-flow section, and must be visible again in
// get_details. It also checks, against real Home Assistant, the section layout
// the flattening in GetConfigEntryOptions relies on.
func (s *TemplateHelperIntegrationTestSuite) TestTemplateBinarySensorDelayRoundTrip() {
	name := GenerateTestID("tmpl_bin_delay")
	entityID := BuildEntityID("binary_sensor", name)

	createResult := s.CallTool("manage_helper", map[string]any{
		"action": "create",
		"type":   "template_binary_sensor",
		"id":     name,
		"name":   name,
		"state":  "{{ true }}",
	})
	s.Require().False(createResult.IsError, "create template_binary_sensor should succeed, got: %s", resultText(createResult))
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), entityID) })

	_, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "template binary sensor did not appear")

	updateResult := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"delay_off": "00:03:00",
	})
	s.Require().False(updateResult.IsError, "update delay_off should succeed, got: %s", resultText(updateResult))

	// Live section check: HA nests delay_on/delay_off/availability under an
	// "expandable" additional_options section in the options flow schema.
	// GetConfigEntryOptions must hand them back flat, with the duration still
	// in HA's {hours, minutes, seconds} dict shape.
	regEntry, err := s.Client().GetEntityRegistryEntry(s.Context(), entityID)
	s.Require().NoError(err, "Failed to get entity registry entry")
	s.Require().NotEmpty(regEntry.ConfigEntryID, "template binary sensor should have a config_entry_id")

	// Poll briefly so a residual gap between the options-flow update and the
	// stored entry becoming readable cannot flake the test; the assertions
	// below run on the last options map either way.
	var options map[string]any
	optionsDeadline := time.Now().Add(10 * time.Second)
	for {
		options, err = s.Client().GetConfigEntryOptions(s.Context(), regEntry.ConfigEntryID)
		s.Require().NoError(err, "Failed to get template config entry options")
		if delay, isMap := options["delay_off"].(map[string]any); isMap {
			if minutes, isNum := delay["minutes"].(float64); isNum && minutes >= 3 {
				break
			}
		}
		if time.Now().After(optionsDeadline) {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	s.T().Logf("template_binary_sensor options: %+v", options)

	delayOff, ok := options["delay_off"].(map[string]any)
	s.Require().True(ok, "delay_off should be a flat duration dict, got %T: %v (all options: %+v)", options["delay_off"], options["delay_off"], options)
	s.InDelta(3.0, delayOff["minutes"], 0.001, "delay_off should be 3 minutes (all options: %+v)", options)
	s.NotContains(options, "additional_options", "the expandable section must be flattened away (all options: %+v)", options)

	// The action under test: get_details shows the stored delay in both formats.
	naturalText := s.getDetailsEventually(entityID, "natural", "Delay before turning off: 00:03:00")
	s.Contains(naturalText, "Delay before turning off: 00:03:00", "the stored delay_off must be visible in get_details")

	jsonText := s.getDetailsEventually(entityID, "json", `"delay_off"`)
	s.Contains(jsonText, `"delay_off"`, "the stored delay_off must be a key of the JSON output")
}

func (s *TemplateHelperIntegrationTestSuite) TestTemplateSensorWithIcon() {
	// Test that icons are correctly set for Config Entry Flow helpers
	// Icons are filtered from the create flow and set via Entity Registry after creation
	sourceName := GenerateTestID("tmpl_icon_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	templateName := GenerateTestID("tmpl_icon")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_number
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     100.0,
			"initial": 42.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	// Create template sensor with custom icon. No device_class is set here,
	// so HA's device-class/unit validation never fires and any unit string
	// is accepted - unlike the update tests above, this "units" literal is
	// not one of the invalid pairs fixed by issue #218.
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":                templateName,
			"state":               "{{ states('" + sourceEntityID + "') | float }}",
			"icon":                "mdi:thermometer",
			"unit_of_measurement": "units",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor with icon")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")
	s.NotNil(entity)

	// Verify the sensor exists and has a state
	s.NotEmpty(entity.State, "Template sensor should have a state")

	// Wait a bit for registry update to be visible
	time.Sleep(2 * time.Second)

	// Verify icon was set via Entity Registry
	// Get entity registry to check icon field
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	var foundEntry *homeassistant.EntityRegistryEntry
	for i := range registry {
		if registry[i].EntityID == templateEntityID {
			foundEntry = &registry[i]
			break
		}
	}

	s.Require().NotNil(foundEntry, "Template sensor should exist in entity registry")
	s.T().Logf("Registry entry: EntityID=%s, Icon=%s, Platform=%s", foundEntry.EntityID, foundEntry.Icon, foundEntry.Platform)
	s.Equal("mdi:thermometer", foundEntry.Icon, "Icon should be set to mdi:thermometer")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *TemplateHelperIntegrationTestSuite) TestTemplateBinarySensorWithIcon() {
	// Test that icons work for binary sensors too
	sourceName := GenerateTestID("tmpl_bin_icon_src")
	sourceEntityID := BuildEntityID("input_boolean", sourceName)
	templateName := GenerateTestID("tmpl_bin_icon")
	templateEntityID := BuildEntityID("binary_sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_boolean
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name":    sourceName,
			"initial": true,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_boolean")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_boolean did not appear")

	// Create template binary sensor with custom icon
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":         templateName,
			"state":        "{{ is_state('" + sourceEntityID + "', 'on') }}",
			"icon":         "mdi:alert-circle",
			"device_class": "problem",
			"type":         "binary_sensor",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template binary sensor with icon")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template binary sensor did not appear")
	s.NotNil(entity)

	// Verify the sensor exists
	s.NotEmpty(entity.State, "Template binary sensor should have a state")

	// Verify icon was set via Entity Registry
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	var foundEntry *homeassistant.EntityRegistryEntry
	for i := range registry {
		if registry[i].EntityID == templateEntityID {
			foundEntry = &registry[i]
			break
		}
	}

	s.Require().NotNil(foundEntry, "Template binary sensor should exist in entity registry")
	s.Equal("mdi:alert-circle", foundEntry.Icon, "Icon should be set to mdi:alert-circle")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *TemplateHelperIntegrationTestSuite) TestTemplateSensorUpdate() {
	// Test updating a template sensor via Options Flow
	sourceName := GenerateTestID("tmpl_upd_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	templateName := GenerateTestID("tmpl_update")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_number
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     100.0,
			"initial": 50.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	// Create initial template sensor
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":                templateName,
			"state":               "{{ states('" + sourceEntityID + "') | float }}",
			"unit_of_measurement": "°C",
			"device_class":        "temperature",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")
	s.NotNil(entity)

	// Update the template sensor with a new state formula
	updateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"state": "{{ states('" + sourceEntityID + "') | float * 2 }}",
		},
	}

	err = s.Client().UpdateHelper(s.Context(), templateEntityID, updateConfig)
	s.Require().NoError(err, "Failed to update template sensor")

	// Wait for update to propagate
	time.Sleep(2 * time.Second)

	// Verify the updated formula works (source is 50, so result should be 100)
	entity, err = s.Client().GetState(s.Context(), templateEntityID)
	s.Require().NoError(err)
	s.Equal("100.0", entity.State, "Template sensor should show doubled value after update")

	// Verify other fields were preserved (device_class, unit_of_measurement).
	// Type-assert with Require rather than a soft `if ok` guard: a missing or
	// mistyped attribute here would mean the field wasn't actually preserved,
	// and a skipped assertion must not read as a passing test.
	deviceClass, ok := entity.Attributes["device_class"].(string)
	s.Require().True(ok, "device_class attribute should be present as a string")
	s.Equal("temperature", deviceClass, "Device class should be preserved")

	uom, ok := entity.Attributes["unit_of_measurement"].(string)
	s.Require().True(ok, "unit_of_measurement attribute should be present as a string")
	s.Equal("°C", uom, "Unit of measurement should be preserved")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *TemplateHelperIntegrationTestSuite) TestTemplateSensorUpdatePartial() {
	// Test partial update (change device_class + unit_of_measurement together,
	// preserve state template). A device_class-only update isn't viable here -
	// see AGENTS.md's "Template sensor device_class requires a matching
	// unit_of_measurement" gotcha: HA rejects a device_class change unless the
	// existing unit is already valid for the new class, so the two fields must
	// be changed together.
	sourceName := GenerateTestID("tmpl_part_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	templateName := GenerateTestID("tmpl_partial")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_number
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     100.0,
			"initial": 25.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	// Create initial template sensor
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":                templateName,
			"state":               "{{ states('" + sourceEntityID + "') | float + 10 }}",
			"unit_of_measurement": "°C",
			"device_class":        "temperature",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")
	s.NotNil(entity)

	// Update only device_class + unit_of_measurement (partial update should
	// preserve state template)
	updateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"device_class":        "power",
			"unit_of_measurement": "W",
		},
	}

	err = s.Client().UpdateHelper(s.Context(), templateEntityID, updateConfig)
	s.Require().NoError(err, "Failed to update template sensor")

	// Wait for update to propagate
	time.Sleep(2 * time.Second)

	// Verify the state template still works (source is 25, so result should be 35)
	entity, err = s.Client().GetState(s.Context(), templateEntityID)
	s.Require().NoError(err)
	s.Equal("35.0", entity.State, "Template sensor should still use original formula (25 + 10 = 35)")

	// Verify device_class and unit_of_measurement were updated. Type-assert
	// with Require rather than a soft `if ok` guard: a missing or mistyped
	// attribute means the update didn't actually take effect, and a skipped
	// assertion must not read as a passing test.
	deviceClass, ok := entity.Attributes["device_class"].(string)
	s.Require().True(ok, "device_class attribute should be present as a string")
	s.Equal("power", deviceClass, "Device class should be updated to power")

	uom, ok := entity.Attributes["unit_of_measurement"].(string)
	s.Require().True(ok, "unit_of_measurement attribute should be present as a string")
	s.Equal("W", uom, "Unit of measurement should be updated to W")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *TemplateHelperIntegrationTestSuite) TestTemplateSensorUpdateWithIcon() {
	// Test that icon updates work via Entity Registry
	sourceName := GenerateTestID("tmpl_icon_upd_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	templateName := GenerateTestID("tmpl_icon_upd")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_number
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     100.0,
			"initial": 42.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	// Create template sensor with initial icon
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  templateName,
			"state": "{{ states('" + sourceEntityID + "') | float }}",
			"icon":  "mdi:thermometer",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")
	s.NotNil(entity)

	// Wait for registry update
	time.Sleep(2 * time.Second)

	// Verify initial icon
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	var foundEntry *homeassistant.EntityRegistryEntry
	for i := range registry {
		if registry[i].EntityID == templateEntityID {
			foundEntry = &registry[i]
			break
		}
	}
	s.Require().NotNil(foundEntry, "Template sensor should exist in entity registry")
	s.Equal("mdi:thermometer", foundEntry.Icon, "Initial icon should be mdi:thermometer")

	// Update the icon
	updateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"icon": "mdi:fire",
		},
	}

	err = s.Client().UpdateHelper(s.Context(), templateEntityID, updateConfig)
	s.Require().NoError(err, "Failed to update template sensor icon")

	// Wait for update to propagate
	time.Sleep(2 * time.Second)

	// Verify updated icon
	registry, err = s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	foundEntry = nil
	for i := range registry {
		if registry[i].EntityID == templateEntityID {
			foundEntry = &registry[i]
			break
		}
	}
	s.Require().NotNil(foundEntry, "Template sensor should still exist in entity registry")
	s.Equal("mdi:fire", foundEntry.Icon, "Icon should be updated to mdi:fire")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}
