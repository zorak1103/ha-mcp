//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type IntegralIntegrationTestSuite struct {
	HelperTestSuite
}

func TestIntegralIntegration(t *testing.T) {
	suite.Run(t, new(IntegralIntegrationTestSuite))
}

func (s *IntegralIntegrationTestSuite) TestIntegralLifecycle() {
	// Create an input_number as source sensor
	sourceName := GenerateTestID("integ_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	integralName := GenerateTestID("integral")
	integralEntityID := BuildEntityID("sensor", integralName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), integralEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_number (simulating power sensor) - entity ID is derived from name
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":                sourceName,
			"min":                 0.0,
			"max":                 10000.0,
			"initial":             100.0,
			"unit_of_measurement": "W",
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	// Create integral sensor - entity ID is derived from name
	integralConfig := homeassistant.HelperConfig{
		Platform: "integration",
		Config: map[string]any{
			"name":        integralName,
			"source":      sourceEntityID,
			"method":      "trapezoidal",
			"round":       2,
			"unit_prefix": "k",
			"unit_time":   "h",
		},
	}

	err = s.Client().CreateHelper(s.Context(), integralConfig)
	s.Require().NoError(err, "Failed to create integral")

	entity, err := s.WaitForEntity(integralEntityID, 5*time.Second)
	s.Require().NoError(err, "Integral did not appear")

	// Verify the integral sensor exists
	s.NotNil(entity)

	// Verify unit of measurement
	unit, ok := entity.Attributes["unit_of_measurement"].(string)
	if ok {
		s.Contains(unit, "kWh", "Unit should be kWh")
	}

	// Test reset (if supported)
	// Note: reset may not be available for all integral sensors, so we don't require it to succeed
	_, _ = s.Client().CallService(s.Context(), "utility_meter", "reset", map[string]any{
		"entity_id": integralEntityID,
	})

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), integralEntityID)
	s.Require().NoError(err, "Failed to delete integral")

	err = s.WaitForEntityGone(integralEntityID, 5*time.Second)
	s.Require().NoError(err, "Integral should be deleted")

	// Cleanup source
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *IntegralIntegrationTestSuite) TestIntegralWithLeftMethod() {
	sourceName := GenerateTestID("integ_left_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	integralName := GenerateTestID("integ_left")
	integralEntityID := BuildEntityID("sensor", integralName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), integralEntityID)
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

	// Create integral with left method - entity ID is derived from name
	integralConfig := homeassistant.HelperConfig{
		Platform: "integration",
		Config: map[string]any{
			"name":   integralName,
			"source": sourceEntityID,
			"method": "left",
		},
	}

	err = s.Client().CreateHelper(s.Context(), integralConfig)
	s.Require().NoError(err, "Failed to create integral")

	entity, err := s.WaitForEntity(integralEntityID, 5*time.Second)
	s.Require().NoError(err, "Integral did not appear")
	s.NotNil(entity)

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), integralEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *IntegralIntegrationTestSuite) TestIntegralWithDifferentTimeUnits() {
	sourceName := GenerateTestID("integ_time_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	integralName := GenerateTestID("integ_time")
	integralEntityID := BuildEntityID("sensor", integralName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), integralEntityID)
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

	// Create integral with minutes as time unit - entity ID is derived from name
	integralConfig := homeassistant.HelperConfig{
		Platform: "integration",
		Config: map[string]any{
			"name":      integralName,
			"source":    sourceEntityID,
			"method":    "trapezoidal",
			"unit_time": "min",
		},
	}

	err = s.Client().CreateHelper(s.Context(), integralConfig)
	s.Require().NoError(err, "Failed to create integral")

	entity, err := s.WaitForEntity(integralEntityID, 5*time.Second)
	s.Require().NoError(err, "Integral did not appear")
	s.NotNil(entity)

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), integralEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}
