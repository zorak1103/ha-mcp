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

// Note: TestTemplateBinarySensorWithDelay removed because delay_on/delay_off
// are not supported by the Config Entry Flow API (only available in YAML config)

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

	// Create template sensor with custom icon
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

	// Verify other fields were preserved (device_class, unit_of_measurement)
	if deviceClass, ok := entity.Attributes["device_class"].(string); ok {
		s.Equal("temperature", deviceClass, "Device class should be preserved")
	}
	if uom, ok := entity.Attributes["unit_of_measurement"].(string); ok {
		s.Equal("°C", uom, "Unit of measurement should be preserved")
	}

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *TemplateHelperIntegrationTestSuite) TestTemplateSensorUpdatePartial() {
	// Test partial update (change device_class + unit_of_measurement together,
	// preserve state template). A device_class-only update isn't viable here:
	// HA's template sensor schema doesn't expose unit_of_measurement at all for
	// percentage-native classes like "battery"/"humidity" (the unit is implied),
	// so submitting one always fails as an unclaimed field - there is no pair of
	// device classes that both accept an explicit unit and share the same one.
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

	// Verify device_class was updated
	if deviceClass, ok := entity.Attributes["device_class"].(string); ok {
		s.Equal("power", deviceClass, "Device class should be updated to power")
	}

	// Verify unit_of_measurement was updated
	if uom, ok := entity.Attributes["unit_of_measurement"].(string); ok {
		s.Equal("W", uom, "Unit of measurement should be updated to W")
	}

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
