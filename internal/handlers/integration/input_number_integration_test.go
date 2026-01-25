//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/stretchr/testify/suite"
)

type InputNumberIntegrationTestSuite struct {
	HelperTestSuite
}

func TestInputNumberIntegration(t *testing.T) {
	suite.Run(t, new(InputNumberIntegrationTestSuite))
}

func (s *InputNumberIntegrationTestSuite) TestInputNumberLifecycle() {
	testID := GenerateTestID("input_num")
	entityID := BuildEntityID("input_number", testID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_number
	config := homeassistant.HelperConfig{
		Platform: "input_number",
		ID:       testID,
		Config: map[string]any{
			"name":    "Test Input Number",
			"min":     0.0,
			"max":     100.0,
			"step":    1.0,
			"initial": 50.0,
			"mode":    "slider",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_number")

	// Wait for entity to appear
	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input number did not appear")
	s.Equal("50.0", entity.State, "Initial state should be 50.0")

	// Test set_value
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": entityID,
		"value":     75.0,
	})
	s.Require().NoError(err, "Failed to set input_number value")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("75.0", entity.State, "State should be 75.0 after set_value")

	// Test increment
	_, err = s.Client().CallService(s.Context(), "input_number", "increment", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to increment input_number")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("76.0", entity.State, "State should be 76.0 after increment")

	// Test decrement
	_, err = s.Client().CallService(s.Context(), "input_number", "decrement", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to decrement input_number")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("75.0", entity.State, "State should be 75.0 after decrement")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err, "Failed to delete input_number")

	err = s.WaitForEntityGone(entityID, 5*time.Second)
	s.Require().NoError(err, "Input number should be deleted")
}

func (s *InputNumberIntegrationTestSuite) TestInputNumberWithUnit() {
	testID := GenerateTestID("input_num_unit")
	entityID := BuildEntityID("input_number", testID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_number with unit
	config := homeassistant.HelperConfig{
		Platform: "input_number",
		ID:       testID,
		Config: map[string]any{
			"name":                "Test Temperature",
			"min":                 0.0,
			"max":                 50.0,
			"step":                0.5,
			"initial":             20.0,
			"unit_of_measurement": "°C",
			"mode":                "box",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_number")

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input number did not appear")
	s.Equal("20.0", entity.State)

	// Check unit in attributes
	unit, ok := entity.Attributes["unit_of_measurement"].(string)
	s.True(ok, "unit_of_measurement attribute should exist")
	s.Equal("°C", unit, "Unit should be °C")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}
