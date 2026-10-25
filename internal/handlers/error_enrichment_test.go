// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestEnrichConfigError_ScriptTemplateSyntaxError(t *testing.T) {
	tests := []struct {
		name           string
		msg            string
		err            error
		wantContain    string
		wantNotContain string
	}{
		{
			name: "template syntax error triggers specific split-expression hint",
			msg:  "Error updating script: HA error",
			err: &homeassistant.APIError{
				StatusCode: 400,
				Message:    "invalid script config: {\"message\":\"Message malformed: invalid template (TemplateSyntaxError: unexpected '}') for dictionary value @ data['sequence'][0]['data']['message']\"}",
			},
			wantContain: "{% set %}",
		},
		{
			name: "generic invalid template still falls back",
			msg:  "Error updating script: HA error",
			err: &homeassistant.APIError{
				StatusCode: 400,
				Message:    "invalid script config: {\"message\":\"invalid template\"}",
			},
			wantContain:    "Jinja2",
			wantNotContain: "{% set %}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := enrichConfigError(tt.msg, tt.err, scriptErrorHints)

			if tt.wantContain != "" && !strings.Contains(result, tt.wantContain) {
				t.Errorf("expected result to contain %q, got %q", tt.wantContain, result)
			}
			if tt.wantNotContain != "" && strings.Contains(result, tt.wantNotContain) {
				t.Errorf("expected result to NOT contain %q, got %q", tt.wantNotContain, result)
			}
		})
	}
}

// TestEnrichConfigError_TriggerShapeHintIsAutomationOnly pins that the legacy
// trigger-shape hint lives in automationErrorHints only: scripts and scenes have
// no triggers, so the same HA body must pass through unchanged.
func TestEnrichConfigError_TriggerShapeHintIsAutomationOnly(t *testing.T) {
	const msg = "Error saving: HA error"
	err := &homeassistant.APIError{
		StatusCode: 400,
		Message:    "invalid config: {\"message\":\"Message malformed: not a valid option at 'options'\"}",
	}

	tests := []struct {
		name  string
		hints []configErrorHint
	}{
		{name: "script hints", hints: scriptErrorHints},
		{name: "scene hints", hints: sceneErrorHints},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := enrichConfigError(msg, err, tt.hints); got != msg {
				t.Errorf("expected message unchanged, got %q", got)
			}
		})
	}
}
