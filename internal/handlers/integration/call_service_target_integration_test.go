//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// CallServiceTargetTestSuite verifies that call_service accepts the HA-standard
// target argument and that a service which leaves its target's state alone
// (automation.trigger) does not run into the Smart Wait timeout warning.
type CallServiceTargetTestSuite struct {
	AutomationTestSuite
}

func TestCallServiceTargetIntegration(t *testing.T) {
	suite.Run(t, new(CallServiceTargetTestSuite))
}

// stateNotConfirmed is the warning Smart Wait appends when a polled entity never
// changed state.
const stateNotConfirmed = "state change not confirmed"

// TestTriggerWithTargetHasNoWarning triggers a storage automation once through
// target and once through data. Triggering runs the actions but leaves the
// automation entity "on", so neither call may carry the state-change warning.
func (s *CallServiceTargetTestSuite) TestTriggerWithTargetHasNoWarning() {
	automationID := GenerateTestID("target_trigger")
	entityID := BuildEntityID("automation", automationID)
	s.RegisterCleanup(func() {
		// Reload first: a DELETE landing inside HA's own async reload orphans the entity (#305).
		_, _ = s.Client().CallService(s.Context(), "automation", "reload", map[string]any{"id": automationID})
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
	})

	// Alias must equal the ID so HA slugifies it to the same entity ID.
	cfgJSON := fmt.Sprintf(`{
		"id": %q,
		"alias": %q,
		"description": "call_service target fixture",
		"mode": "single",
		"triggers": [{"platform": "event", "event_type": "mcptest_target_trigger"}],
		"actions": [{"event": "mcptest_target_done"}]
	}`, automationID, automationID)
	var cfg homeassistant.AutomationConfig
	s.Require().NoError(json.Unmarshal([]byte(cfgJSON), &cfg), "decoding the fixture config")

	s.Require().NoError(s.Client().CreateAutomation(s.Context(), cfg), "failed to create automation")

	_, err := s.WaitForAutomation(automationID, 30*time.Second)
	s.Require().NoError(err, "automation did not appear after create")

	_, err = s.Client().CallService(s.Context(), "automation", "reload", map[string]any{"id": automationID})
	s.Require().NoError(err, "failed to reload automation after create")

	cases := []struct {
		name string
		args map[string]any
	}{
		{
			name: "target",
			args: map[string]any{
				"domain":  "automation",
				"service": "trigger",
				"target":  map[string]any{"entity_id": entityID},
			},
		},
		{
			name: "data",
			args: map[string]any{
				"domain":  "automation",
				"service": "trigger",
				"data":    map[string]any{"entity_id": entityID},
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			result := s.CallTool("call_service", tc.args)
			text := resultText(result)
			s.Require().False(result.IsError, "automation.trigger via %s should succeed: %s", tc.name, text)
			s.Contains(text, entityID, "the result should name the triggered entity")
			s.NotContains(text, stateNotConfirmed, "automation.trigger leaves the state alone, so no warning is expected")
		})
	}
}
