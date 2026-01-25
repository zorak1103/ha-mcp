//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type InputButtonIntegrationTestSuite struct {
	HelperTestSuite
}

func TestInputButtonIntegration(t *testing.T) {
	suite.Run(t, new(InputButtonIntegrationTestSuite))
}

func (s *InputButtonIntegrationTestSuite) TestInputButtonLifecycle() {
	testName := GenerateTestID("input_btn")
	entityID := BuildEntityID("input_button", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_button - entity ID is derived from name
	config := homeassistant.HelperConfig{
		Platform: "input_button",
		Config: map[string]any{
			"name": testName,
			"icon": "mdi:gesture-tap-button",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_button")

	// Wait for entity to appear
	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input button did not appear")
	s.Equal("unknown", entity.State, "Initial state should be 'unknown'")

	// Test press
	_, err = s.Client().CallService(s.Context(), "input_button", "press", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to press input_button")

	// After pressing, state changes to a timestamp
	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.NotEqual("unknown", entity.State, "State should change after press")

	// Record timestamp and press again
	firstPress := entity.State
	time.Sleep(100 * time.Millisecond)

	_, err = s.Client().CallService(s.Context(), "input_button", "press", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to press input_button second time")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.NotEqual(firstPress, entity.State, "State should change on each press")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err, "Failed to delete input_button")

	err = s.WaitForEntityGone(entityID, 5*time.Second)
	s.Require().NoError(err, "Input button should be deleted")
}

func (s *InputButtonIntegrationTestSuite) TestInputButtonWithIcon() {
	testName := GenerateTestID("input_btn_icon")
	entityID := BuildEntityID("input_button", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_button with custom icon
	config := homeassistant.HelperConfig{
		Platform: "input_button",
		Config: map[string]any{
			"name": testName,
			"icon": "mdi:bell-ring",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_button")

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input button did not appear")

	// Check icon attribute
	icon, ok := entity.Attributes["icon"].(string)
	s.True(ok, "icon attribute should exist")
	s.Equal("mdi:bell-ring", icon, "Icon should be mdi:bell-ring")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}
