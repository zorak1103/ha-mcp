//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// keptVariablesNote matches a "kept from previous config" note that lists the
// variables key, however many other keys it names.
var keptVariablesNote = regexp.MustCompile(`kept from previous config:[^)]*\bvariables\b`)

// UpdateKeptKeysTestSuite verifies issue #319 end to end: manage_automation and
// manage_script update stay merges, name the top-level keys they carried over,
// and manage_automation's variables argument replaces or removes the block.
// Every config assertion reads the stored config back from HA.
type UpdateKeptKeysTestSuite struct {
	AutomationTestSuite
}

func TestUpdateKeptKeysIntegration(t *testing.T) {
	suite.Run(t, new(UpdateKeptKeysTestSuite))
}

// TestUpdateReportsAndReplacesVariables walks a storage automation carrying a
// variables block through three updates: description only (block kept and
// named), a new variables object (block replaced), and an empty object (block
// removed).
func (s *UpdateKeptKeysTestSuite) TestUpdateReportsAndReplacesVariables() {
	automationID := GenerateTestID("kept_auto")
	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
	})

	// Alias must equal ID so HA slugifies it to the same entity ID (project
	// convention).
	cfgJSON := fmt.Sprintf(`{
		"id": %q,
		"alias": %q,
		"description": "original description",
		"mode": "single",
		"triggers": [{"platform": "event", "event_type": "mcptest_kept_trigger"}],
		"actions": [{"event": "mcptest_kept_done"}],
		"variables": {"first": "1", "second": "2"}
	}`, automationID, automationID)
	var cfg homeassistant.AutomationConfig
	s.Require().NoError(json.Unmarshal([]byte(cfgJSON), &cfg), "decoding the fixture config")

	s.Require().NoError(s.Client().CreateAutomation(s.Context(), cfg), "failed to create automation")

	_, err := s.WaitForAutomation(automationID, 30*time.Second)
	s.Require().NoError(err, "automation did not appear after create")

	// HA schedules its own async reload after the config POST; a DELETE landing
	// inside it orphans the entity (#305). Reload synchronously, as the tools do.
	_, err = s.Client().CallService(s.Context(), "automation", "reload", map[string]any{"id": automationID})
	s.Require().NoError(err, "failed to reload automation after create")

	stored, err := s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err, "GetAutomation after create")
	s.Require().NotNil(stored.Config)
	s.Require().Equal(map[string]any{"first": "1", "second": "2"}, stored.Config.Variables, "fixture variables after create")

	// 1. Update of the description only: the block is carried over and named.
	result := s.CallToolJSON("manage_automation", fmt.Sprintf(`{
		"action": "update",
		"automation_id": %q,
		"description": "updated description"
	}`, automationID))
	s.Require().False(result.IsError, "update description should succeed, got: %s", resultText(result))
	s.Contains(resultText(result), "kept from previous config: variables", "update must name the carried-over variables block")

	stored, err = s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err, "GetAutomation after description update")
	s.Require().NotNil(stored.Config)
	s.Equal("updated description", stored.Config.Description)
	s.Equal(map[string]any{"first": "1", "second": "2"}, stored.Config.Variables, "variables must survive a description-only update")

	// 2. Update with a new variables object: the block is replaced, not merged,
	// and is no longer reported as kept.
	result = s.CallToolJSON("manage_automation", fmt.Sprintf(`{
		"action": "update",
		"automation_id": %q,
		"variables": {"fresh": "1"}
	}`, automationID))
	s.Require().False(result.IsError, "update variables should succeed, got: %s", resultText(result))
	s.False(keptVariablesNote.MatchString(resultText(result)), "a replaced variables block must not be listed as kept, got: %s", resultText(result))

	stored, err = s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err, "GetAutomation after variables replace")
	s.Require().NotNil(stored.Config)
	s.Equal(map[string]any{"fresh": "1"}, stored.Config.Variables, "variables must be replaced wholesale")

	// 3. Update with an empty object: the block is removed.
	result = s.CallToolJSON("manage_automation", fmt.Sprintf(`{
		"action": "update",
		"automation_id": %q,
		"variables": {}
	}`, automationID))
	s.Require().False(result.IsError, "update with empty variables should succeed, got: %s", resultText(result))
	s.False(keptVariablesNote.MatchString(resultText(result)), "a removed variables block must not be listed as kept, got: %s", resultText(result))

	stored, err = s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err, "GetAutomation after variables removal")
	s.Require().NotNil(stored.Config)
	s.Empty(stored.Config.Variables, "an empty variables object must remove the block")
}

// TestScriptUpdateReportsKeptKeys checks that manage_script update names the
// variables block it carried over. The script tool has no variables argument
// (its variables argument on execute is not config), so the block can only ever
// be kept.
func (s *UpdateKeptKeysTestSuite) TestScriptUpdateReportsKeptKeys() {
	scriptID := GenerateTestID("kept_script")
	scriptEntityID := BuildEntityID("script", scriptID)
	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
	})

	cfgJSON := fmt.Sprintf(`{
		"alias": %q,
		"description": "original description",
		"mode": "single",
		"sequence": [{"event": "mcptest_kept_done"}],
		"variables": {"first": "1"}
	}`, scriptID)
	var cfg homeassistant.ScriptConfig
	s.Require().NoError(json.Unmarshal([]byte(cfgJSON), &cfg), "decoding the fixture config")

	// CreateScript takes the bare ID; GetScript takes the full entity ID.
	s.Require().NoError(s.Client().CreateScript(s.Context(), scriptID, cfg), "failed to create script")

	_, err := s.WaitForEntity(scriptEntityID, 30*time.Second)
	s.Require().NoError(err, "script did not appear after create")

	// Same create-then-immediate-delete race as for automations (#305).
	_, err = s.Client().CallService(s.Context(), "script", "reload", nil)
	s.Require().NoError(err, "failed to reload scripts after create")

	result := s.CallToolJSON("manage_script", fmt.Sprintf(`{
		"action": "update",
		"script_id": %q,
		"description": "updated description"
	}`, scriptID))
	s.Require().False(result.IsError, "update description should succeed, got: %s", resultText(result))
	s.Contains(resultText(result), "kept from previous config: variables", "update must name the carried-over variables block")

	stored, err := s.Client().GetScript(s.Context(), scriptEntityID)
	s.Require().NoError(err, "GetScript after update")
	s.Require().NotNil(stored.Config)
	s.Equal("updated description", stored.Config.Description)
	s.Equal(map[string]any{"first": "1"}, stored.Config.Variables, "variables must survive a description-only update")
}
