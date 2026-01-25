//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type InputSelectIntegrationTestSuite struct {
	HelperTestSuite
}

func TestInputSelectIntegration(t *testing.T) {
	suite.Run(t, new(InputSelectIntegrationTestSuite))
}

func (s *InputSelectIntegrationTestSuite) TestInputSelectLifecycle() {
	testName := GenerateTestID("input_sel")
	entityID := BuildEntityID("input_select", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_select - entity ID is derived from name
	config := homeassistant.HelperConfig{
		Platform: "input_select",
		Config: map[string]any{
			"name":    testName,
			"options": []string{"Option A", "Option B", "Option C"},
			"initial": "Option A",
			"icon":    "mdi:format-list-bulleted",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_select")

	// Wait for entity to appear
	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input select did not appear")
	s.Equal("Option A", entity.State, "Initial state should be 'Option A'")

	// Test select_option
	_, err = s.Client().CallService(s.Context(), "input_select", "select_option", map[string]any{
		"entity_id": entityID,
		"option":    "Option B",
	})
	s.Require().NoError(err, "Failed to select option")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("Option B", entity.State, "State should be 'Option B' after select_option")

	// Test select_first
	_, err = s.Client().CallService(s.Context(), "input_select", "select_first", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to select first")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("Option A", entity.State, "State should be 'Option A' after select_first")

	// Test select_last
	_, err = s.Client().CallService(s.Context(), "input_select", "select_last", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to select last")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("Option C", entity.State, "State should be 'Option C' after select_last")

	// Test select_next
	_, err = s.Client().CallService(s.Context(), "input_select", "select_first", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err)
	time.Sleep(200 * time.Millisecond)

	_, err = s.Client().CallService(s.Context(), "input_select", "select_next", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to select next")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("Option B", entity.State, "State should be 'Option B' after select_next")

	// Test select_previous
	_, err = s.Client().CallService(s.Context(), "input_select", "select_previous", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to select previous")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("Option A", entity.State, "State should be 'Option A' after select_previous")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err, "Failed to delete input_select")

	err = s.WaitForEntityGone(entityID, 5*time.Second)
	s.Require().NoError(err, "Input select should be deleted")
}

func (s *InputSelectIntegrationTestSuite) TestInputSelectSetOptions() {
	testName := GenerateTestID("input_sel_opts")
	entityID := BuildEntityID("input_select", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_select
	config := homeassistant.HelperConfig{
		Platform: "input_select",
		Config: map[string]any{
			"name":    testName,
			"options": []string{"First", "Second"},
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_select")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input select did not appear")

	// Test set_options
	_, err = s.Client().CallService(s.Context(), "input_select", "set_options", map[string]any{
		"entity_id": entityID,
		"options":   []string{"New A", "New B", "New C", "New D"},
	})
	s.Require().NoError(err, "Failed to set options")

	time.Sleep(300 * time.Millisecond)
	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)

	// Check that options were updated
	options, ok := entity.Attributes["options"].([]any)
	s.True(ok, "options attribute should exist")
	s.Len(options, 4, "Should have 4 options")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}
