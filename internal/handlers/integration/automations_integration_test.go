//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/stretchr/testify/suite"
)

type AutomationIntegrationTestSuite struct {
	AutomationTestSuite
}

func TestAutomationIntegration(t *testing.T) {
	suite.Run(t, new(AutomationIntegrationTestSuite))
}

func (s *AutomationIntegrationTestSuite) TestAutomationLifecycle() {
	// Create an input_boolean for the automation to control
	targetName := GenerateTestID("auto_target")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	triggerName := GenerateTestID("auto_trigger")
	triggerEntityID := BuildEntityID("input_button", triggerName)
	automationID := GenerateTestID("automation")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
		_ = s.Client().DeleteHelper(s.Context(), triggerEntityID)
	})

	// Create target input_boolean - entity ID is derived from name
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": targetName, "initial": false},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err, "Failed to create target input_boolean")

	// Create trigger input_button - entity ID is derived from name
	triggerConfig := homeassistant.HelperConfig{
		Platform: "input_button",
		Config:   map[string]any{"name": triggerName},
	}
	err = s.Client().CreateHelper(s.Context(), triggerConfig)
	s.Require().NoError(err, "Failed to create trigger input_button")

	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err)
	_, err = s.WaitForEntity(triggerEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create automation: when button is pressed, toggle the input_boolean
	automationConfig := homeassistant.AutomationConfig{
		ID:          automationID,
		Alias:       "Test Automation",
		Description: "Integration test automation",
		Mode:        "single",
		Triggers: []any{
			map[string]any{
				"platform":  "state",
				"entity_id": triggerEntityID,
			},
		},
		Actions: []any{
			map[string]any{
				"service": "input_boolean.toggle",
				"target": map[string]any{
					"entity_id": targetEntityID,
				},
			},
		},
	}

	err = s.Client().CreateAutomation(s.Context(), automationConfig)
	s.Require().NoError(err, "Failed to create automation")

	// Wait for automation to appear
	automationEntityID := BuildEntityID("automation", automationID)
	entity, err := s.WaitForEntity(automationEntityID, 5*time.Second)
	s.Require().NoError(err, "Automation did not appear")
	s.Equal("on", entity.State, "Automation should be enabled by default")

	// Verify target is off
	target, err := s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("off", target.State, "Target should be off initially")

	// Trigger the automation by pressing the button
	_, err = s.Client().CallService(s.Context(), "input_button", "press", map[string]any{
		"entity_id": triggerEntityID,
	})
	s.Require().NoError(err, "Failed to press trigger button")

	// Wait for automation to execute
	time.Sleep(500 * time.Millisecond)

	// Verify target was toggled
	target, err = s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("on", target.State, "Target should be on after automation triggered")

	// Test toggle automation (disable)
	err = s.Client().ToggleAutomation(s.Context(), automationID, false)
	s.Require().NoError(err, "Failed to disable automation")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), automationEntityID)
	s.Require().NoError(err)
	s.Equal("off", entity.State, "Automation should be disabled")

	// Test toggle automation (enable)
	err = s.Client().ToggleAutomation(s.Context(), automationID, true)
	s.Require().NoError(err, "Failed to enable automation")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), automationEntityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "Automation should be enabled")

	// Test delete
	err = s.Client().DeleteAutomation(s.Context(), automationID)
	s.Require().NoError(err, "Failed to delete automation")

	err = s.WaitForEntityGone(automationEntityID, 5*time.Second)
	s.Require().NoError(err, "Automation should be deleted")

	// Cleanup helpers
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	_ = s.Client().DeleteHelper(s.Context(), triggerEntityID)
}

func (s *AutomationIntegrationTestSuite) TestAutomationUpdate() {
	targetName := GenerateTestID("auto_upd_tgt")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	automationID := GenerateTestID("auto_update")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	// Create target - entity ID is derived from name
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": targetName},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create automation with time trigger
	automationConfig := homeassistant.AutomationConfig{
		ID:          automationID,
		Alias:       "Original Name",
		Description: "Original description",
		Mode:        "single",
		Triggers: []any{
			map[string]any{
				"platform": "time",
				"at":       "23:59:59",
			},
		},
		Actions: []any{
			map[string]any{
				"service": "input_boolean.turn_on",
				"target": map[string]any{
					"entity_id": targetEntityID,
				},
			},
		},
	}

	err = s.Client().CreateAutomation(s.Context(), automationConfig)
	s.Require().NoError(err, "Failed to create automation")

	automationEntityID := BuildEntityID("automation", automationID)
	entity, err := s.WaitForEntity(automationEntityID, 5*time.Second)
	s.Require().NoError(err, "Automation did not appear")

	friendlyName, _ := entity.Attributes["friendly_name"].(string)
	s.Equal("Original Name", friendlyName)

	// Update automation
	updatedConfig := homeassistant.AutomationConfig{
		ID:          automationID,
		Alias:       "Updated Name",
		Description: "Updated description",
		Mode:        "restart",
		Triggers: []any{
			map[string]any{
				"platform": "time",
				"at":       "12:00:00",
			},
		},
		Actions: []any{
			map[string]any{
				"service": "input_boolean.turn_off",
				"target": map[string]any{
					"entity_id": targetEntityID,
				},
			},
		},
	}

	err = s.Client().UpdateAutomation(s.Context(), automationID, updatedConfig)
	s.Require().NoError(err, "Failed to update automation")

	time.Sleep(300 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), automationEntityID)
	s.Require().NoError(err)

	friendlyName, _ = entity.Attributes["friendly_name"].(string)
	s.Equal("Updated Name", friendlyName, "Automation name should be updated")

	// Cleanup
	_ = s.Client().DeleteAutomation(s.Context(), automationID)
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
}

func (s *AutomationIntegrationTestSuite) TestAutomationWithCondition() {
	targetName := GenerateTestID("auto_cond_tgt")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	conditionName := GenerateTestID("auto_cond")
	conditionEntityID := BuildEntityID("input_boolean", conditionName)
	triggerName := GenerateTestID("auto_cond_trg")
	triggerEntityID := BuildEntityID("input_button", triggerName)
	automationID := GenerateTestID("auto_condition")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
		_ = s.Client().DeleteHelper(s.Context(), conditionEntityID)
		_ = s.Client().DeleteHelper(s.Context(), triggerEntityID)
	})

	// Create helpers - entity IDs are derived from names
	for _, cfg := range []homeassistant.HelperConfig{
		{Platform: "input_boolean", Config: map[string]any{"name": targetName, "initial": false}},
		{Platform: "input_boolean", Config: map[string]any{"name": conditionName, "initial": false}},
		{Platform: "input_button", Config: map[string]any{"name": triggerName}},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err)
	}

	_, _ = s.WaitForEntity(targetEntityID, 5*time.Second)
	_, _ = s.WaitForEntity(conditionEntityID, 5*time.Second)
	_, _ = s.WaitForEntity(triggerEntityID, 5*time.Second)

	// Create automation with condition
	automationConfig := homeassistant.AutomationConfig{
		ID:    automationID,
		Alias: "Conditional Automation",
		Mode:  "single",
		Triggers: []any{
			map[string]any{
				"platform":  "state",
				"entity_id": triggerEntityID,
			},
		},
		Conditions: []any{
			map[string]any{
				"condition": "state",
				"entity_id": conditionEntityID,
				"state":     "on",
			},
		},
		Actions: []any{
			map[string]any{
				"service": "input_boolean.turn_on",
				"target": map[string]any{
					"entity_id": targetEntityID,
				},
			},
		},
	}

	err := s.Client().CreateAutomation(s.Context(), automationConfig)
	s.Require().NoError(err, "Failed to create automation")

	automationEntityID := BuildEntityID("automation", automationID)
	_, err = s.WaitForEntity(automationEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Trigger automation while condition is false - target should NOT change
	_, err = s.Client().CallService(s.Context(), "input_button", "press", map[string]any{
		"entity_id": triggerEntityID,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	target, err := s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("off", target.State, "Target should stay off when condition is false")

	// Enable condition
	_, err = s.Client().CallService(s.Context(), "input_boolean", "turn_on", map[string]any{
		"entity_id": conditionEntityID,
	})
	s.Require().NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Trigger automation while condition is true - target SHOULD change
	_, err = s.Client().CallService(s.Context(), "input_button", "press", map[string]any{
		"entity_id": triggerEntityID,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	target, err = s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("on", target.State, "Target should turn on when condition is true")

	// Cleanup
	_ = s.Client().DeleteAutomation(s.Context(), automationID)
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	_ = s.Client().DeleteHelper(s.Context(), conditionEntityID)
	_ = s.Client().DeleteHelper(s.Context(), triggerEntityID)
}
