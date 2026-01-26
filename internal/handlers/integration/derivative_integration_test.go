//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

var _ = time.Second          // silence unused import
var _ homeassistant.Client   // silence unused import

type DerivativeIntegrationTestSuite struct {
	HelperTestSuite
}

func TestDerivativeIntegration(t *testing.T) {
	t.Skip("Derivative helpers do not support WebSocket or REST API create/update - requires YAML configuration")
	suite.Run(t, new(DerivativeIntegrationTestSuite))
}

func (s *DerivativeIntegrationTestSuite) TestDerivativeLifecycle() {
	// Create an input_number as source sensor
	sourceName := GenerateTestID("deriv_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	derivativeName := GenerateTestID("derivative")
	derivativeEntityID := BuildEntityID("sensor", derivativeName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), derivativeEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_number - entity ID is derived from name
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":                sourceName,
			"min":                 0.0,
			"max":                 1000.0,
			"initial":             100.0,
			"unit_of_measurement": "L",
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	// Create derivative sensor (rate of change) - entity ID is derived from name
	derivativeConfig := homeassistant.HelperConfig{
		Platform: "derivative",
		Config: map[string]any{
			"name":        derivativeName,
			"source":      sourceEntityID,
			"round":       2,
			"unit_time":   "min",
			"time_window": "00:01:00", // 1 minute window
		},
	}

	err = s.Client().CreateHelper(s.Context(), derivativeConfig)
	s.Require().NoError(err, "Failed to create derivative")

	entity, err := s.WaitForEntity(derivativeEntityID, 5*time.Second)
	s.Require().NoError(err, "Derivative did not appear")
	s.NotNil(entity)

	// Change source value to generate some rate of change
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     150.0,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)

	// Change again
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     200.0,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)

	// Verify derivative sensor updated
	entity, err = s.Client().GetState(s.Context(), derivativeEntityID)
	s.Require().NoError(err)
	s.NotNil(entity)

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), derivativeEntityID)
	s.Require().NoError(err, "Failed to delete derivative")

	err = s.WaitForEntityGone(derivativeEntityID, 5*time.Second)
	s.Require().NoError(err, "Derivative should be deleted")

	// Cleanup source
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *DerivativeIntegrationTestSuite) TestDerivativeWithTimeWindow() {
	sourceName := GenerateTestID("deriv_win_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	derivativeName := GenerateTestID("deriv_win")
	derivativeEntityID := BuildEntityID("sensor", derivativeName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), derivativeEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     1000.0,
			"initial": 50.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create derivative with 5 minute time window - entity ID is derived from name
	derivativeConfig := homeassistant.HelperConfig{
		Platform: "derivative",
		Config: map[string]any{
			"name":        derivativeName,
			"source":      sourceEntityID,
			"time_window": "00:05:00",
			"unit_time":   "h",
		},
	}

	err = s.Client().CreateHelper(s.Context(), derivativeConfig)
	s.Require().NoError(err, "Failed to create derivative")

	entity, err := s.WaitForEntity(derivativeEntityID, 5*time.Second)
	s.Require().NoError(err, "Derivative did not appear")
	s.NotNil(entity)

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), derivativeEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *DerivativeIntegrationTestSuite) TestDerivativeWithUnitPrefix() {
	sourceName := GenerateTestID("deriv_pref_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	derivativeName := GenerateTestID("deriv_pref")
	derivativeEntityID := BuildEntityID("sensor", derivativeName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), derivativeEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":                sourceName,
			"min":                 0.0,
			"max":                 1000000.0,
			"initial":             1000.0,
			"unit_of_measurement": "W",
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create derivative with kilo prefix - entity ID is derived from name
	derivativeConfig := homeassistant.HelperConfig{
		Platform: "derivative",
		Config: map[string]any{
			"name":        derivativeName,
			"source":      sourceEntityID,
			"unit_prefix": "k",
			"unit_time":   "h",
		},
	}

	err = s.Client().CreateHelper(s.Context(), derivativeConfig)
	s.Require().NoError(err, "Failed to create derivative")

	entity, err := s.WaitForEntity(derivativeEntityID, 5*time.Second)
	s.Require().NoError(err, "Derivative did not appear")
	s.NotNil(entity)

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), derivativeEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}
