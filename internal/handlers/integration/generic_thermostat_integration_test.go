//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type GenericThermostatIntegrationTestSuite struct {
	HelperTestSuite
}

func TestGenericThermostatIntegration(t *testing.T) {
	suite.Run(t, new(GenericThermostatIntegrationTestSuite))
}

// createThermostatSources creates template switch (heater) and template sensor (temp sensor).
// generic_thermostat requires switch entity (not input_boolean) and sensor entity (not input_number).
func (s *GenericThermostatIntegrationTestSuite) createThermostatSources(prefix string, initialTemp float64) (string, string, string, string) {
	// Create input_boolean for heater base
	boolName := GenerateTestID(prefix + "_bool")
	boolEntityID := BuildEntityID("input_boolean", boolName)

	boolConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name":    boolName,
			"initial": false,
		},
	}

	err := s.Client().CreateHelper(s.Context(), boolConfig)
	s.Require().NoError(err, "Failed to create input_boolean")

	_, err = s.WaitForEntity(boolEntityID, 5*time.Second)
	s.Require().NoError(err, "Input_boolean did not appear")

	// Create template switch that wraps the input_boolean
	heaterName := GenerateTestID(prefix + "_heater")
	heaterEntityID := BuildEntityID("switch", heaterName)

	switchConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":     heaterName,
			"turn_on":  map[string]any{"service": "input_boolean.turn_on", "data": map[string]any{"entity_id": boolEntityID}},
			"turn_off": map[string]any{"service": "input_boolean.turn_off", "data": map[string]any{"entity_id": boolEntityID}},
			"type":     "switch", // Menu selection
		},
	}

	err = s.Client().CreateHelper(s.Context(), switchConfig)
	s.Require().NoError(err, "Failed to create template switch")

	_, err = s.WaitForEntity(heaterEntityID, 5*time.Second)
	s.Require().NoError(err, "Template switch did not appear")

	// Create input_number for temperature base
	inputName := GenerateTestID(prefix + "_input")
	inputEntityID := BuildEntityID("input_number", inputName)

	inputConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    inputName,
			"min":     0.0,
			"max":     50.0,
			"initial": initialTemp,
		},
	}

	err = s.Client().CreateHelper(s.Context(), inputConfig)
	s.Require().NoError(err, "Failed to create input_number")

	_, err = s.WaitForEntity(inputEntityID, 5*time.Second)
	s.Require().NoError(err, "Input_number did not appear")

	// Create template sensor that wraps the input_number
	sensorName := GenerateTestID(prefix + "_sensor")
	sensorEntityID := BuildEntityID("sensor", sensorName)

	templateSensorConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  sensorName,
			"state": "{{ states('" + inputEntityID + "') | float }}",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateSensorConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	_, err = s.WaitForEntity(sensorEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	return boolEntityID, heaterEntityID, inputEntityID, sensorEntityID
}

func (s *GenericThermostatIntegrationTestSuite) TestGenericThermostatLifecycle() {
	// Create source entities (input_boolean + template switch, input_number + template sensor)
	boolEntityID, heaterEntityID, inputEntityID, sensorEntityID := s.createThermostatSources("thermo", 20.0)
	thermoName := GenerateTestID("thermostat")
	thermoEntityID := BuildEntityID("climate", thermoName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thermoEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
		_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
		_ = s.Client().DeleteHelper(s.Context(), heaterEntityID)
		_ = s.Client().DeleteHelper(s.Context(), boolEntityID)
	})

	// Create generic thermostat (using API field names). min_temp is a
	// vol.Optional field on generic_thermostat's OPTIONS_SCHEMA - set here
	// so the update step below can prove it survives untouched.
	thermoConfig := homeassistant.HelperConfig{
		Platform: "generic_thermostat",
		Config: map[string]any{
			"name":          thermoName,
			"heater":        heaterEntityID, // API field name
			"target_sensor": sensorEntityID, // API field name
			"ac_mode":       false,          // Required field
			"min_temp":      10.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), thermoConfig)
	s.Require().NoError(err, "Failed to create generic_thermostat")

	entity, err := s.WaitForEntity(thermoEntityID, 5*time.Second)
	s.Require().NoError(err, "Generic thermostat did not appear")
	s.NotEmpty(entity.State, "Thermostat should have a state")

	// Test update - regression coverage for issue #194: generic_thermostat's
	// OPTIONS_FLOW advances through an "init" -> "presets" sequence just
	// like its CONFIG_FLOW, so every update used to fail with "unexpected
	// options flow result type: form" before updateHelperViaOptionsFlow
	// learned to complete the trailing presets step.
	//
	// The presets step's own schema is all-Optional, and Home Assistant
	// deletes any vol.Optional key of a step's schema that its submission
	// omits - so completing that step with an empty map (safe on create,
	// nothing to delete yet) would silently wipe every stored preset
	// temperature on every single update instead of merely failing loudly.
	// Read back min_temp - a *different* step's optional field - after the
	// update to confirm nothing outside the changed field was touched.
	err = s.Client().UpdateHelper(s.Context(), thermoEntityID, homeassistant.HelperConfig{
		Config: map[string]any{
			"cold_tolerance": 0.8,
		},
	})
	s.Require().NoError(err, "Failed to update generic_thermostat")

	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	var configEntryID string
	for _, regEntry := range registry {
		if regEntry.EntityID == thermoEntityID {
			configEntryID = regEntry.ConfigEntryID
			break
		}
	}
	s.Require().NotEmpty(configEntryID, "Generic thermostat should have a config_entry_id")

	options, err := s.Client().GetConfigEntryOptions(s.Context(), configEntryID)
	s.Require().NoError(err, "Failed to get generic_thermostat config entry options")
	s.InDelta(0.8, options["cold_tolerance"], 0.001, "cold_tolerance should reflect the update")
	s.InDelta(10.0, options["min_temp"], 0.001, "min_temp should survive the update untouched")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), thermoEntityID)
	s.Require().NoError(err, "Failed to delete generic_thermostat")

	err = s.WaitForEntityGone(thermoEntityID, 5*time.Second)
	s.Require().NoError(err, "Generic thermostat should be deleted")

	// Cleanup sources
	_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
	_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
	_ = s.Client().DeleteHelper(s.Context(), heaterEntityID)
	_ = s.Client().DeleteHelper(s.Context(), boolEntityID)
}

func (s *GenericThermostatIntegrationTestSuite) TestGenericThermostatWithTolerances() {
	// Create source entities (input_boolean + template switch, input_number + template sensor)
	boolEntityID, heaterEntityID, inputEntityID, sensorEntityID := s.createThermostatSources("thermo_tol", 22.0)
	thermoName := GenerateTestID("thermo_tolerance")
	thermoEntityID := BuildEntityID("climate", thermoName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thermoEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
		_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
		_ = s.Client().DeleteHelper(s.Context(), heaterEntityID)
		_ = s.Client().DeleteHelper(s.Context(), boolEntityID)
	})

	// Create generic thermostat with tolerances (using API field names)
	thermoConfig := homeassistant.HelperConfig{
		Platform: "generic_thermostat",
		Config: map[string]any{
			"name":           thermoName,
			"heater":         heaterEntityID, // API field name
			"target_sensor":  sensorEntityID, // API field name
			"ac_mode":        false,          // Required field
			"cold_tolerance": 0.5,
			"hot_tolerance":  0.5,
		},
	}

	err := s.Client().CreateHelper(s.Context(), thermoConfig)
	s.Require().NoError(err, "Failed to create generic_thermostat with tolerances")

	entity, err := s.WaitForEntity(thermoEntityID, 5*time.Second)
	s.Require().NoError(err, "Generic thermostat did not appear")
	s.NotEmpty(entity.State, "Thermostat should have a state")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), thermoEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
	_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
	_ = s.Client().DeleteHelper(s.Context(), heaterEntityID)
	_ = s.Client().DeleteHelper(s.Context(), boolEntityID)
}
