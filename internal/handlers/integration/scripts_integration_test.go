//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/stretchr/testify/suite"
)

type ScriptIntegrationTestSuite struct {
	ScriptTestSuite
}

func TestScriptIntegration(t *testing.T) {
	suite.Run(t, new(ScriptIntegrationTestSuite))
}

func (s *ScriptIntegrationTestSuite) TestScriptLifecycle() {
	// Create an input_boolean for the script to control
	targetID := GenerateTestID("script_target")
	targetEntityID := BuildEntityID("input_boolean", targetID)
	scriptID := GenerateTestID("script")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	// Create target input_boolean
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		ID:       targetID,
		Config:   map[string]any{"name": "Script Target", "initial": false},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err, "Failed to create target input_boolean")

	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create script
	scriptConfig := homeassistant.ScriptConfig{
		Alias:       "Test Script",
		Description: "Integration test script",
		Mode:        "single",
		Sequence: []any{
			map[string]any{
				"service": "input_boolean.turn_on",
				"target": map[string]any{
					"entity_id": targetEntityID,
				},
			},
		},
	}

	err = s.Client().CreateScript(s.Context(), scriptID, scriptConfig)
	s.Require().NoError(err, "Failed to create script")

	// Wait for script to appear
	scriptEntityID := BuildEntityID("script", scriptID)
	entity, err := s.WaitForEntity(scriptEntityID, 5*time.Second)
	s.Require().NoError(err, "Script did not appear")
	s.Equal("off", entity.State, "Script should be idle (off)")

	// Verify target is off
	target, err := s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("off", target.State, "Target should be off initially")

	// Execute the script via service call
	_, err = s.Client().CallService(s.Context(), "script", "turn_on", map[string]any{
		"entity_id": scriptEntityID,
	})
	s.Require().NoError(err, "Failed to execute script")

	// Wait for script to complete
	time.Sleep(500 * time.Millisecond)

	// Verify target was turned on
	target, err = s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("on", target.State, "Target should be on after script executed")

	// Test delete
	err = s.Client().DeleteScript(s.Context(), scriptID)
	s.Require().NoError(err, "Failed to delete script")

	err = s.WaitForEntityGone(scriptEntityID, 5*time.Second)
	s.Require().NoError(err, "Script should be deleted")

	// Cleanup helper
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
}

func (s *ScriptIntegrationTestSuite) TestScriptWithVariables() {
	// Create an input_number for the script to control
	targetID := GenerateTestID("script_var_tgt")
	targetEntityID := BuildEntityID("input_number", targetID)
	scriptID := GenerateTestID("script_vars")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	// Create target input_number
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		ID:       targetID,
		Config: map[string]any{
			"name":    "Script Variable Target",
			"min":     0.0,
			"max":     100.0,
			"initial": 0.0,
		},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create script with variables/fields
	scriptConfig := homeassistant.ScriptConfig{
		Alias:       "Script with Variables",
		Description: "Sets a value based on input variable",
		Mode:        "single",
		Fields: map[string]any{
			"target_value": map[string]any{
				"name":        "Target Value",
				"description": "The value to set",
				"required":    true,
				"selector": map[string]any{
					"number": map[string]any{
						"min": 0,
						"max": 100,
					},
				},
			},
		},
		Sequence: []any{
			map[string]any{
				"service": "input_number.set_value",
				"target": map[string]any{
					"entity_id": targetEntityID,
				},
				"data": map[string]any{
					"value": "{{ target_value }}",
				},
			},
		},
	}

	err = s.Client().CreateScript(s.Context(), scriptID, scriptConfig)
	s.Require().NoError(err, "Failed to create script")

	scriptEntityID := BuildEntityID("script", scriptID)
	_, err = s.WaitForEntity(scriptEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Execute script with variable via service call
	_, err = s.Client().CallService(s.Context(), "script", "turn_on", map[string]any{
		"entity_id":    scriptEntityID,
		"variables": map[string]any{
			"target_value": 42.0,
		},
	})
	s.Require().NoError(err, "Failed to execute script with variables")

	time.Sleep(500 * time.Millisecond)

	// Verify target was set to 42
	target, err := s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("42.0", target.State, "Target should be 42.0 after script with variable")

	// Execute again with different value
	_, err = s.Client().CallService(s.Context(), "script", "turn_on", map[string]any{
		"entity_id": scriptEntityID,
		"variables": map[string]any{
			"target_value": 75.0,
		},
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)

	target, err = s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("75.0", target.State, "Target should be 75.0 after second execution")

	// Cleanup
	_ = s.Client().DeleteScript(s.Context(), scriptID)
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
}

func (s *ScriptIntegrationTestSuite) TestScriptUpdate() {
	targetID := GenerateTestID("script_upd_tgt")
	targetEntityID := BuildEntityID("input_boolean", targetID)
	scriptID := GenerateTestID("script_update")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	// Create target
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		ID:       targetID,
		Config:   map[string]any{"name": "Update Script Target"},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create script that turns on
	scriptConfig := homeassistant.ScriptConfig{
		Alias:       "Original Script",
		Description: "Turns on the target",
		Mode:        "single",
		Sequence: []any{
			map[string]any{
				"service": "input_boolean.turn_on",
				"target": map[string]any{
					"entity_id": targetEntityID,
				},
			},
		},
	}

	err = s.Client().CreateScript(s.Context(), scriptID, scriptConfig)
	s.Require().NoError(err)

	scriptEntityID := BuildEntityID("script", scriptID)
	entity, err := s.WaitForEntity(scriptEntityID, 5*time.Second)
	s.Require().NoError(err)

	friendlyName, _ := entity.Attributes["friendly_name"].(string)
	s.Equal("Original Script", friendlyName)

	// Update script to turn off instead
	updatedConfig := homeassistant.ScriptConfig{
		Alias:       "Updated Script",
		Description: "Now turns off the target",
		Mode:        "restart",
		Sequence: []any{
			map[string]any{
				"service": "input_boolean.turn_off",
				"target": map[string]any{
					"entity_id": targetEntityID,
				},
			},
		},
	}

	err = s.Client().UpdateScript(s.Context(), scriptID, updatedConfig)
	s.Require().NoError(err, "Failed to update script")

	time.Sleep(300 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), scriptEntityID)
	s.Require().NoError(err)

	friendlyName, _ = entity.Attributes["friendly_name"].(string)
	s.Equal("Updated Script", friendlyName, "Script name should be updated")

	// Turn on target manually
	_, err = s.Client().CallService(s.Context(), "input_boolean", "turn_on", map[string]any{
		"entity_id": targetEntityID,
	})
	s.Require().NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Execute updated script - should turn off
	_, err = s.Client().CallService(s.Context(), "script", "turn_on", map[string]any{
		"entity_id": scriptEntityID,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)

	target, err := s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("off", target.State, "Updated script should turn off target")

	// Cleanup
	_ = s.Client().DeleteScript(s.Context(), scriptID)
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
}

func (s *ScriptIntegrationTestSuite) TestScriptWithMultipleActions() {
	target1ID := GenerateTestID("script_m1")
	target2ID := GenerateTestID("script_m2")
	target1EntityID := BuildEntityID("input_boolean", target1ID)
	target2EntityID := BuildEntityID("input_boolean", target2ID)
	scriptID := GenerateTestID("script_multi")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), target1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), target2EntityID)
	})

	// Create targets
	for _, cfg := range []homeassistant.HelperConfig{
		{Platform: "input_boolean", ID: target1ID, Config: map[string]any{"name": "Multi Target 1", "initial": false}},
		{Platform: "input_boolean", ID: target2ID, Config: map[string]any{"name": "Multi Target 2", "initial": false}},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err)
	}

	_, _ = s.WaitForEntity(target1EntityID, 5*time.Second)
	_, _ = s.WaitForEntity(target2EntityID, 5*time.Second)

	// Create script with multiple actions
	scriptConfig := homeassistant.ScriptConfig{
		Alias: "Multi-Action Script",
		Mode:  "single",
		Sequence: []any{
			map[string]any{
				"service": "input_boolean.turn_on",
				"target": map[string]any{
					"entity_id": target1EntityID,
				},
			},
			map[string]any{
				"delay": map[string]any{
					"milliseconds": 100,
				},
			},
			map[string]any{
				"service": "input_boolean.turn_on",
				"target": map[string]any{
					"entity_id": target2EntityID,
				},
			},
		},
	}

	err := s.Client().CreateScript(s.Context(), scriptID, scriptConfig)
	s.Require().NoError(err, "Failed to create script")

	scriptEntityID := BuildEntityID("script", scriptID)
	_, err = s.WaitForEntity(scriptEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Execute script
	_, err = s.Client().CallService(s.Context(), "script", "turn_on", map[string]any{
		"entity_id": scriptEntityID,
	})
	s.Require().NoError(err)

	time.Sleep(700 * time.Millisecond)

	// Verify both targets are on
	target1, err := s.Client().GetState(s.Context(), target1EntityID)
	s.Require().NoError(err)
	s.Equal("on", target1.State, "Target 1 should be on")

	target2, err := s.Client().GetState(s.Context(), target2EntityID)
	s.Require().NoError(err)
	s.Equal("on", target2.State, "Target 2 should be on")

	// Cleanup
	_ = s.Client().DeleteScript(s.Context(), scriptID)
	_ = s.Client().DeleteHelper(s.Context(), target1EntityID)
	_ = s.Client().DeleteHelper(s.Context(), target2EntityID)
}
