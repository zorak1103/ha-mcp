//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/stretchr/testify/suite"
)

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

	// Create source input_number - entity ID is derived from name
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

	// Create template sensor that converts Celsius to Fahrenheit - entity ID is derived from name
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":                templateName,
			"state":               "{{ (states('" + sourceEntityID + "') | float * 9/5 + 32) | round(1) }}",
			"unit_of_measurement": "°F",
			"device_class":        "temperature",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	// 20°C = 68°F
	s.Equal("68.0", entity.State, "Template should convert 20°C to 68°F")

	// Update source and verify template updates
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     30.0, // 30°C = 86°F
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), templateEntityID)
	s.Require().NoError(err)
	s.Equal("86.0", entity.State, "Template should update to 86°F for 30°C")

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

	// Create source input_number - entity ID is derived from name
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

	// Create template binary sensor (on when source > 50) - entity ID is derived from name
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

	// Create template sensor with state_class for long-term statistics - entity ID is derived from name
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

func (s *TemplateHelperIntegrationTestSuite) TestTemplateBinarySensorWithDelay() {
	sourceName := GenerateTestID("tmpl_dly_src")
	sourceEntityID := BuildEntityID("input_boolean", sourceName)
	templateName := GenerateTestID("tmpl_delay")
	templateEntityID := BuildEntityID("binary_sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name":    sourceName,
			"initial": false,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create template binary sensor with delay_on - entity ID is derived from name
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":     templateName,
			"state":    "{{ is_state('" + sourceEntityID + "', 'on') }}",
			"delay_on": "00:00:01", // 1 second delay
			"type":     "binary_sensor",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template binary sensor")

	entity, err := s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template binary sensor did not appear")
	s.Equal("off", entity.State)

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}
