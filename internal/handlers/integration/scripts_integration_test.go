//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type ScriptIntegrationTestSuite struct {
	ScriptTestSuite
}

func TestScriptIntegration(t *testing.T) {
	suite.Run(t, new(ScriptIntegrationTestSuite))
}

func (s *ScriptIntegrationTestSuite) TestScriptLifecycle() {
	// Create an input_boolean for the script to control
	targetName := GenerateTestID("script_target")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	scriptID := GenerateTestID("script")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	// Create target input_boolean - entity ID is derived from name
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": targetName, "initial": false},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err, "Failed to create target input_boolean")

	_, err = s.WaitForEntity(targetEntityID, 10*time.Second)
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
	entity, err := s.WaitForEntity(scriptEntityID, 10*time.Second)
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

	err = s.WaitForEntityGone(scriptEntityID, 10*time.Second)
	s.Require().NoError(err, "Script should be deleted")

	// Cleanup helper
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
}

func (s *ScriptIntegrationTestSuite) TestScriptMaxFieldPreservation() {
	scriptID := GenerateTestID("script_max")
	scriptEntityID := BuildEntityID("script", scriptID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
	})

	// Create a parallel script with max:4
	cfg := homeassistant.ScriptConfig{
		Alias:    scriptID, // Alias drives the entity ID slug
		Mode:     "parallel",
		Max:      4,
		Sequence: []any{map[string]any{"delay": map[string]any{"seconds": 1}}},
	}
	err := s.Client().CreateScript(s.Context(), scriptID, cfg)
	s.Require().NoError(err, "failed to create parallel script")

	_, err = s.WaitForEntity(scriptEntityID, 30*time.Second)
	s.Require().NoError(err, "script entity did not appear after create")

	// Fetch and verify Max
	fetched, err := s.Client().GetScript(s.Context(), scriptEntityID)
	s.Require().NoError(err)
	s.Require().NotNil(fetched.Config, "GetScript must return Config")
	s.Equal(4, fetched.Config.Max, "Max should be 4 after create")

	// Update description only (no max arg — the round-trip preservation test)
	// UpdateScript expects bare scriptID, not the entity ID with "script." prefix
	updatedCfg := *fetched.Config
	updatedCfg.Description = "updated description"
	err = s.Client().UpdateScript(s.Context(), scriptID, updatedCfg)
	s.Require().NoError(err, "failed to update script")

	// Confirm Max survived
	afterUpdate, err := s.Client().GetScript(s.Context(), scriptEntityID)
	s.Require().NoError(err)
	s.Require().NotNil(afterUpdate.Config)
	s.Equal(4, afterUpdate.Config.Max, "Max must survive an update that doesn't change it")
}

func (s *ScriptIntegrationTestSuite) TestScriptWithVariables() {
	// Create an input_number for the script to control
	targetName := GenerateTestID("script_var_tgt")
	targetEntityID := BuildEntityID("input_number", targetName)
	scriptID := GenerateTestID("script_vars")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	// Create target input_number - entity ID is derived from name
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    targetName,
			"min":     0.0,
			"max":     100.0,
			"initial": 0.0,
		},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(targetEntityID, 10*time.Second)
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
	_, err = s.WaitForEntity(scriptEntityID, 10*time.Second)
	s.Require().NoError(err)

	// Execute script with variable via service call
	_, err = s.Client().CallService(s.Context(), "script", "turn_on", map[string]any{
		"entity_id": scriptEntityID,
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
	targetName := GenerateTestID("script_upd_tgt")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	scriptID := GenerateTestID("script_update")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
		// Safety net for #122: if this test ever regresses and an orphan duplicate is created,
		// clean it up too so it doesn't linger in the test HA instance.
		_ = s.Client().DeleteScript(s.Context(), scriptID+"_2")
	})

	// Create target - entity ID is derived from name
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": targetName},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(targetEntityID, 10*time.Second)
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
	entity, err := s.WaitForEntity(scriptEntityID, 10*time.Second)
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

	// Script updates via REST API may require reload to take effect
	_, _ = s.Client().CallService(s.Context(), "script", "reload", nil)
	time.Sleep(2 * time.Second)

	entity, err = s.Client().GetState(s.Context(), scriptEntityID)
	s.Require().NoError(err)

	friendlyName, _ = entity.Attributes["friendly_name"].(string)
	s.Equal("Updated Script", friendlyName, "Script name should be updated")

	// Regression check for #122: updating a storage-managed script must never spawn a
	// duplicate orphan entity (script.<id>_2) - that only happens when the REST config write
	// silently lands on a different underlying config than the one it targeted.
	orphanEntityID := scriptEntityID + "_2"
	scripts, err := s.Client().ListScripts(s.Context())
	s.Require().NoError(err)
	for _, sc := range scripts {
		s.NotEqual(orphanEntityID, sc.EntityID, "update must not create an orphan duplicate entity")
	}

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
	target1Name := GenerateTestID("script_m1")
	target2Name := GenerateTestID("script_m2")
	target1EntityID := BuildEntityID("input_boolean", target1Name)
	target2EntityID := BuildEntityID("input_boolean", target2Name)
	scriptID := GenerateTestID("script_multi")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), target1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), target2EntityID)
	})

	// Create targets - entity IDs are derived from names
	for _, cfg := range []homeassistant.HelperConfig{
		{Platform: "input_boolean", Config: map[string]any{"name": target1Name, "initial": false}},
		{Platform: "input_boolean", Config: map[string]any{"name": target2Name, "initial": false}},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err)
	}

	_, _ = s.WaitForEntity(target1EntityID, 10*time.Second)
	_, _ = s.WaitForEntity(target2EntityID, 10*time.Second)

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
	_, err = s.WaitForEntity(scriptEntityID, 10*time.Second)
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
