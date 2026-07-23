//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type ScriptToolDispatchTestSuite struct {
	ScriptTestSuite
}

func TestScriptToolDispatch(t *testing.T) {
	suite.Run(t, new(ScriptToolDispatchTestSuite))
}

// TestScriptUpdateViaTool covers the documented GetScript-vs-GetState gotcha
// (CLAUDE.md: "GetState returns only state + friendly_name, not full script
// config") by updating only the description via the real manage_script tool
// and confirming the sequence (untouched) survives - if the handler ever
// regressed to building its base config from GetState instead of GetScript,
// this would silently wipe the sequence.
func (s *ScriptToolDispatchTestSuite) TestScriptUpdateViaTool() {
	targetName := GenerateTestID("script_td_target")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	scriptID := GenerateTestID("script_td")
	scriptEntityID := BuildEntityID("script", scriptID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": targetName, "initial": false},
	})
	s.Require().NoError(err, "Failed to create target input_boolean")
	_, err = s.WaitForEntity(targetEntityID, 10*time.Second)
	s.Require().NoError(err)

	scriptConfig := homeassistant.ScriptConfig{
		Alias:       "Tool Dispatch Test Script",
		Description: "original description",
		Mode:        "single",
		Sequence: []any{
			map[string]any{
				"service": "input_boolean.turn_on",
				"target":  map[string]any{"entity_id": targetEntityID},
			},
		},
	}
	err = s.Client().CreateScript(s.Context(), scriptID, scriptConfig)
	s.Require().NoError(err, "Failed to create script")

	_, err = s.WaitForEntity(scriptEntityID, 10*time.Second)
	s.Require().NoError(err, "Script did not appear")

	// The action under test: update (description only) via the real manage_script tool.
	result := s.CallTool("manage_script", map[string]any{
		"action":      "update",
		"script_id":   scriptID,
		"description": "updated via tool dispatch test",
	})
	s.Require().False(result.IsError, "manage_script update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	updated, err := s.Client().GetScript(s.Context(), scriptEntityID)
	s.Require().NoError(err)
	s.Require().NotNil(updated.Config)
	s.Equal("updated via tool dispatch test", updated.Config.Description)
	s.Require().Len(updated.Config.Sequence, 1, "sequence must survive an update that only touches description")

	step, ok := updated.Config.Sequence[0].(map[string]any)
	s.Require().True(ok, "sequence step should be a map")

	// Home Assistant normalizes the "service" key to "action" for service-call
	// sequence steps on newer versions when it saves/reads back a script -
	// accept either key so this test doesn't depend on the HA version's schema.
	actionValue, hasAction := step["action"].(string)
	serviceValue, hasService := step["service"].(string)
	switch {
	case hasAction:
		s.Equal("input_boolean.turn_on", actionValue, "sequence content must be preserved, not wiped")
	case hasService:
		s.Equal("input_boolean.turn_on", serviceValue, "sequence content must be preserved, not wiped")
	default:
		s.Fail("sequence step is missing both \"action\" and \"service\" keys", "step: %#v", step)
	}
}
