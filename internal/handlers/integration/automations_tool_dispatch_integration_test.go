//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type AutomationToolDispatchTestSuite struct {
	AutomationTestSuite
}

func TestAutomationToolDispatch(t *testing.T) {
	suite.Run(t, new(AutomationToolDispatchTestSuite))
}

// TestAutomationUpdateAndPatchViaTool covers the documented config-ID-vs-
// entity-slug normalization gotcha (CLAUDE.md: "manage_automation update ...
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
