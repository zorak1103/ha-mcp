//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

type AutomationToolDispatchTestSuite struct {
	AutomationTestSuite
}

func TestAutomationToolDispatch(t *testing.T) {
	suite.Run(t, new(AutomationToolDispatchTestSuite))
}

// TestAutomationUpdateAndPatchViaTool covers the documented config-ID-vs-
// entity-slug normalization gotcha (AGENTS.md: "manage_automation update ...
// UI-created automations have numeric config IDs differing from entity_id
// suffix") by driving both update and a semantic patch op through the real
// manage_automation tool.
func (s *AutomationToolDispatchTestSuite) TestAutomationUpdateAndPatchViaTool() {
	triggerName := GenerateTestID("auto_td_trigger")
	triggerEntityID := BuildEntityID("input_button", triggerName)
	automationID := GenerateTestID("auto_td")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
		_ = s.Client().DeleteHelper(s.Context(), triggerEntityID)
	})

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_button",
		Config:   map[string]any{"name": triggerName},
	})
	s.Require().NoError(err, "Failed to create trigger input_button")
	_, err = s.WaitForEntity(triggerEntityID, 30*time.Second)
	s.Require().NoError(err)

	automationConfig := homeassistant.AutomationConfig{
		ID:          automationID,
		Alias:       automationID,
		Description: "original description",
		Mode:        "single",
		Triggers: []any{
			map[string]any{
				"platform":  "state",
				"entity_id": triggerEntityID,
			},
		},
		Actions: []any{
			map[string]any{
				"service": "input_button.press",
				"target":  map[string]any{"entity_id": triggerEntityID},
			},
		},
	}
	err = s.Client().CreateAutomation(s.Context(), automationConfig)
	s.Require().NoError(err, "Failed to create automation")

	_, err = s.WaitForAutomation(automationID, 30*time.Second)
	s.Require().NoError(err, "Automation did not appear")

	// Action under test 1: update via the real manage_automation tool.
	updateResult := s.CallTool("manage_automation", map[string]any{
		"action":        "update",
		"automation_id": automationID,
		"description":   "updated via tool dispatch test",
	})
	s.Require().False(updateResult.IsError, "manage_automation update should succeed, got: %s", resultText(updateResult))

	time.Sleep(1 * time.Second)

	afterUpdate, err := s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err)
	s.Require().NotNil(afterUpdate.Config)
	s.Equal("updated via tool dispatch test", afterUpdate.Config.Description)

	// Action under test 2: patch (one semantic op) via the real manage_automation tool.
	patchResult := s.CallTool("manage_automation", map[string]any{
		"action":        "patch",
		"automation_id": automationID,
		"operations": []any{
			map[string]any{
				"op":      "add",
				"match":   map[string]any{"entity_id": triggerEntityID},
				"section": "triggers",
				"field":   "for",
				"value":   "00:00:05",
			},
		},
	})
	s.Require().False(patchResult.IsError, "manage_automation patch should succeed, got: %s", resultText(patchResult))

	time.Sleep(1 * time.Second)

	afterPatch, err := s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err)
	s.Require().NotNil(afterPatch.Config)
	s.Require().NotEmpty(afterPatch.Config.Triggers)
	trigger, ok := afterPatch.Config.Triggers[0].(map[string]any)
	s.Require().True(ok, "trigger should be a map")
	s.Equal("00:00:05", trigger["for"], "trigger should have 'for' field added via semantic patch")
}

// TestPatchLegacyTriggerWithOptionsGivesHint pins issue #304: a legacy trigger
// (state, numeric_state, ...) written with the purpose-specific `options`
// wrapper is rejected by HA with an opaque schema error, and the tool must name
// the cause and the fix. It then applies the hinted flat shape to prove the fix
// actually works.
func (s *AutomationToolDispatchTestSuite) TestPatchLegacyTriggerWithOptionsGivesHint() {
	triggerName := GenerateTestID("auto_opt_trigger")
	triggerEntityID := BuildEntityID("input_button", triggerName)
	automationID := GenerateTestID("auto_opt")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
		_ = s.Client().DeleteHelper(s.Context(), triggerEntityID)
	})

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_button",
		Config:   map[string]any{"name": triggerName},
	})
	s.Require().NoError(err, "Failed to create trigger input_button")
	_, err = s.WaitForEntity(triggerEntityID, 30*time.Second)
	s.Require().NoError(err)

	err = s.Client().CreateAutomation(s.Context(), homeassistant.AutomationConfig{
		ID:    automationID,
		Alias: automationID,
		Mode:  "single",
		Triggers: []any{
			map[string]any{"platform": "state", "entity_id": triggerEntityID},
		},
		Actions: []any{
			map[string]any{"service": "input_button.press", "target": map[string]any{"entity_id": triggerEntityID}},
		},
	})
	s.Require().NoError(err, "Failed to create automation")
	_, err = s.WaitForAutomation(automationID, 30*time.Second)
	s.Require().NoError(err, "Automation did not appear")

	patchTrigger := func(trigger map[string]any) *mcp.ToolsCallResult {
		return s.CallTool("manage_automation", map[string]any{
			"action":        "patch",
			"automation_id": automationID,
			"operations": []any{
				map[string]any{"op": "add", "path": "/triggers/-", "value": trigger},
			},
		})
	}

	// Wrong shape: options wrapper on a legacy trigger.
	bad := patchTrigger(map[string]any{
		"trigger":   "state",
		"entity_id": triggerEntityID,
		"to":        "on",
		"options":   map[string]any{"for": "00:15:00"},
	})
	s.Require().True(bad.IsError, "legacy trigger with options must be rejected")
	s.Contains(resultText(bad), "Hint:")
	s.Contains(resultText(bad), "flat keys")

	// Wrong shape: target wrapper on a legacy trigger.
	badTarget := patchTrigger(map[string]any{
		"trigger": "state",
		"target":  map[string]any{"entity_id": triggerEntityID},
		"to":      "on",
	})
	s.Require().True(badTarget.IsError, "legacy trigger with target must be rejected")
	s.Contains(resultText(badTarget), "Hint:")
	s.Contains(resultText(badTarget), "flat keys")

	// The hinted fix: flat keys.
	good := patchTrigger(map[string]any{
		"trigger":   "state",
		"entity_id": triggerEntityID,
		"to":        "on",
		"for":       "00:15:00",
	})
	s.Require().False(good.IsError, "flat-shape trigger should be accepted, got: %s", resultText(good))
}

// TestAutomationUpdate_ConfigFileEntryExists_StorageManaged verifies the config-file existence
// guard (added to prevent writes silently orphaning entries not already present in the managed
// config file) does not falsely refuse a storage-managed automation created through the
// tool — the regression this change must not introduce.
func (s *AutomationToolDispatchTestSuite) TestAutomationUpdate_ConfigFileEntryExists_StorageManaged() {
	triggerName := GenerateTestID("auto_probe_trigger")
	triggerEntityID := BuildEntityID("input_button", triggerName)
	automationID := GenerateTestID("auto_probe")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
		_ = s.Client().DeleteHelper(s.Context(), triggerEntityID)
	})

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_button",
		Config:   map[string]any{"name": triggerName},
	})
	s.Require().NoError(err, "Failed to create trigger input_button")
	_, err = s.WaitForEntity(triggerEntityID, 30*time.Second)
	s.Require().NoError(err)

	automationConfig := homeassistant.AutomationConfig{
		ID:          automationID,
		Alias:       automationID,
		Description: "original description",
		Mode:        "single",
		Triggers: []any{
			map[string]any{"platform": "state", "entity_id": triggerEntityID},
		},
		Actions: []any{
			map[string]any{"service": "input_button.press", "target": map[string]any{"entity_id": triggerEntityID}},
		},
	}
	err = s.Client().CreateAutomation(s.Context(), automationConfig)
	s.Require().NoError(err, "Failed to create automation")
	_, err = s.WaitForAutomation(automationID, 30*time.Second)
	s.Require().NoError(err, "Automation did not appear")

	// Confirm the probe itself reports true for a real storage-managed automation.
	exists, err := s.Client().ConfigFileEntryExists(s.Context(), "automation", automationID)
	s.Require().NoError(err)
	s.True(exists, "a storage-managed automation must probe as present in automations.yaml")

	// And the real tool path must still succeed end-to-end.
	updateResult := s.CallTool("manage_automation", map[string]any{
		"action":        "update",
		"automation_id": automationID,
		"description":   "updated via config-file-guard probe test",
	})
	s.Require().False(updateResult.IsError, "update on a storage-managed automation must succeed, got: %s", resultText(updateResult))
}
