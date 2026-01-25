//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type InputBooleanIntegrationTestSuite struct {
	HelperTestSuite
}

func TestInputBooleanIntegration(t *testing.T) {
	suite.Run(t, new(InputBooleanIntegrationTestSuite))
}

func (s *InputBooleanIntegrationTestSuite) TestInputBooleanLifecycle() {
	// Entity ID is derived from the name, so we use testName as both
	testName := GenerateTestID("input_bool")
	entityID := BuildEntityID("input_boolean", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_boolean - entity ID is derived from name
	config := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name":    testName,
			"initial": false,
			"icon":    "mdi:lightbulb",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_boolean")

	// Wait for entity to appear
	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input boolean did not appear")
	s.Equal("off", entity.State, "Initial state should be off")

	// Test turn_on
	_, err = s.Client().CallService(s.Context(), "input_boolean", "turn_on", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to turn on input_boolean")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "State should be on after turn_on")

	// Test turn_off
	_, err = s.Client().CallService(s.Context(), "input_boolean", "turn_off", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to turn off input_boolean")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("off", entity.State, "State should be off after turn_off")

	// Test toggle
	_, err = s.Client().CallService(s.Context(), "input_boolean", "toggle", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to toggle input_boolean")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "State should be on after toggle")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err, "Failed to delete input_boolean")

	err = s.WaitForEntityGone(entityID, 5*time.Second)
	s.Require().NoError(err, "Input boolean should be deleted")
}

func (s *InputBooleanIntegrationTestSuite) TestInputBooleanWithInitialOn() {
	testName := GenerateTestID("input_bool_on")
	entityID := BuildEntityID("input_boolean", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_boolean with initial state on
	config := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name":    testName,
			"initial": true,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_boolean")

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input boolean did not appear")
	s.Equal("on", entity.State, "Initial state should be on")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}
