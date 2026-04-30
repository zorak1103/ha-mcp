package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// mockAutomationForDebug is a helper that builds a well-formed Automation for debug tests.
func mockAutomationForDebug() *homeassistant.Automation {
	return &homeassistant.Automation{
		EntityID:      "automation.morning_routine",
		State:         "on",
		FriendlyName:  "Morning Routine",
		LastTriggered: "2026-02-19T06:30:00Z",
		Config: &homeassistant.AutomationConfig{
			Alias: "Morning Routine",
			Mode:  "single",
			Triggers: []any{
				map[string]any{"platform": "state", "entity_id": "sensor.bedroom_motion"},
				map[string]any{"platform": "time", "at": "06:30:00"},
			},
			Conditions: []any{
				map[string]any{"condition": "state"},
			},
			Actions: []any{
				map[string]any{"service": "light.turn_on"},
				map[string]any{"service": "media_player.play_media"},
				map[string]any{"service": "notify.mobile"},
			},
		},
	}
}

// TestHandleDebugTrace_MissingAutomationID verifies that missing automation_id returns an error.
func TestHandleDebugTrace_MissingAutomationID(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action": "debug",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
	if !strings.Contains(result.Content[0].Text, "automation_id") {
		t.Errorf("expected error about automation_id, got: %s", result.Content[0].Text)
	}
}

// TestHandleDebugTrace_NonAutomation verifies that non-automation entity IDs are rejected.
func TestHandleDebugTrace_NonAutomation(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "light.living_room",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
	if !strings.Contains(result.Content[0].Text, "debug action only supports automations") {
		t.Errorf("expected non-automation error, got: %s", result.Content[0].Text)
	}
}

// TestHandleDebugTrace_AutomationNotFound verifies that a missing automation returns an error.
func TestHandleDebugTrace_AutomationNotFound(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			return nil, fmt.Errorf("automation not found")
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "automation.nonexistent",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
}

// TestHandleDebugTrace_NaturalFormat verifies the full natural language debug report.
func TestHandleDebugTrace_NaturalFormat(t *testing.T) {
	t.Parallel()

	now := time.Now()
	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			return mockAutomationForDebug(), nil
		},
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{
				map[string]any{
					"run_id":    "1708329000.123",
					"state":     "stopped",
					"timestamp": now.Add(-1 * time.Hour).Format(time.RFC3339),
					"duration":  1.25,
					"trigger":   map[string]any{"platform": "time", "description": "time (06:30:00)"},
				},
			}, nil
		},
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			return &homeassistant.Entity{
				EntityID:    entityID,
				State:       "on",
				LastChanged: now.Add(-5 * time.Minute),
			}, nil
		},
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{
				{When: now.Add(-30 * time.Minute).Format(time.RFC3339), Name: "Morning Routine", State: "triggered"},
			}, nil
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "automation.morning_routine",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error result: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	for _, want := range []string{
		"=== Automation Debug Report ===",
		"Morning Routine",
		"automation.morning_routine",
		"--- Latest Trace ---",
		"1708329000.123",
		"--- Trigger Entity States ---",
		"sensor.bedroom_motion",
		"--- Logbook (last 6 hours) ---",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, text)
		}
	}
}

// TestHandleDebugTrace_JSONFormat verifies the JSON format output structure.
func TestHandleDebugTrace_JSONFormat(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			return mockAutomationForDebug(), nil
		},
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{}, nil
		},
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{}, nil
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "automation.morning_routine",
		"format":        "json",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error result: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	for _, want := range []string{"config", "logbook_hours", "Morning Routine"} {
		if !strings.Contains(text, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, text)
		}
	}
}

// TestHandleDebugTrace_NoTraces verifies the "No traces found" message when no traces exist.
func TestHandleDebugTrace_NoTraces(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			return mockAutomationForDebug(), nil
		},
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{}, nil
		},
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{}, nil
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "automation.morning_routine",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error result: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "No traces found") {
		t.Errorf("expected 'No traces found', got: %s", result.Content[0].Text)
	}
}

// TestHandleDebugTrace_NoEntityTriggers verifies the message when no entity-based triggers exist.
func TestHandleDebugTrace_NoEntityTriggers(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			auto := mockAutomationForDebug()
			// Only a time trigger — no entity_id
			auto.Config.Triggers = []any{
				map[string]any{"platform": "time", "at": "06:30:00"},
			}
			return auto, nil
		},
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{}, nil
		},
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{}, nil
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "morning_routine",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error result: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "No entity-based triggers found") {
		t.Errorf("expected 'No entity-based triggers found', got: %s", result.Content[0].Text)
	}
}

// TestHandleDebugTrace_EmptyLogbook verifies the "No logbook entries found" message.
func TestHandleDebugTrace_EmptyLogbook(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			return mockAutomationForDebug(), nil
		},
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{}, nil
		},
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
			return nil, nil
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "automation.morning_routine",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error result: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "No logbook entries found") {
		t.Errorf("expected 'No logbook entries found', got: %s", result.Content[0].Text)
	}
}

// TestHandleDebugTrace_CustomHours verifies the custom hours parameter appears in output.
func TestHandleDebugTrace_CustomHours(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			return mockAutomationForDebug(), nil
		},
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{}, nil
		},
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{}, nil
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "automation.morning_routine",
		"hours":         float64(24),
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error result: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "last 24 hours") {
		t.Errorf("expected '24 hours' in output, got: %s", result.Content[0].Text)
	}
}

// TestExtractTriggerEntityIDs verifies extraction of entity IDs from trigger configurations.
func TestExtractTriggerEntityIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		triggers  []any
		wantIDs   []string
		wantTypes []string
	}{
		{
			name: "state trigger with entity_id",
			triggers: []any{
				map[string]any{"platform": "state", "entity_id": "sensor.motion"},
			},
			wantIDs:   []string{"sensor.motion"},
			wantTypes: []string{"state"},
		},
		{
			name: "time trigger without entity_id",
			triggers: []any{
				map[string]any{"platform": "time", "at": "06:30:00"},
			},
			wantIDs: nil,
		},
		{
			name: "multiple triggers with deduplication",
			triggers: []any{
				map[string]any{"platform": "state", "entity_id": "sensor.motion"},
				map[string]any{"platform": "state", "entity_id": "sensor.motion"}, // duplicate
				map[string]any{"platform": "state", "entity_id": "sensor.door"},
			},
			wantIDs:   []string{"sensor.motion", "sensor.door"},
			wantTypes: []string{"state", "state"},
		},
		{
			name:     "empty triggers",
			triggers: nil,
			wantIDs:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := extractTriggerEntityIDs(tt.triggers)
			if len(result) != len(tt.wantIDs) {
				t.Errorf("got %d infos, want %d", len(result), len(tt.wantIDs))
				return
			}
			for i, info := range result {
				if info.entityID != tt.wantIDs[i] {
					t.Errorf("[%d] entityID = %q, want %q", i, info.entityID, tt.wantIDs[i])
				}
				if tt.wantTypes != nil && info.triggerType != tt.wantTypes[i] {
					t.Errorf("[%d] triggerType = %q, want %q", i, info.triggerType, tt.wantTypes[i])
				}
			}
		})
	}
}

// TestHandleDebugTrace_StateFromGetState verifies that the automation runtime state
// is read via GetState (not from GetAutomation, which only returns config).
// Regression test for issue #74: debug report shows empty State field.
func TestHandleDebugTrace_StateFromGetState(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		// GetAutomation returns config only — State is empty, as in the real HA API.
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			auto := mockAutomationForDebug()
			auto.State = "" // real HA config endpoint does not populate State
			return auto, nil
		},
		// GetState returns the runtime state — this is the authoritative source.
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			return &homeassistant.Entity{
				EntityID: entityID,
				State:    "on",
			}, nil
		},
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{}, nil
		},
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{}, nil
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "automation.morning_routine",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "State: on") {
		t.Errorf("expected 'State: on' in output (state must come from GetState, not GetAutomation config)\nfull output:\n%s", text)
	}
}

// TestHandleDebugTrace_StateGetStateFails verifies that a GetState failure is handled
// gracefully: State may appear empty but no error is returned (best-effort).
func TestHandleDebugTrace_StateGetStateFails(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			auto := mockAutomationForDebug()
			auto.State = ""
			return auto, nil
		},
		GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
			return nil, fmt.Errorf("entity not found")
		},
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{}, nil
		},
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{}, nil
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "automation.morning_routine",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must not return an error result — GetState failure is best-effort.
	if result.IsError {
		t.Errorf("GetState failure should not produce error result: %s", result.Content[0].Text)
	}
}

// TestExtractTriggerTypes verifies extraction of deduplicated trigger platform types.
func TestExtractTriggerTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		triggers []any
		want     []string
	}{
		{
			name: "single platform",
			triggers: []any{
				map[string]any{"platform": "state"},
			},
			want: []string{"state"},
		},
		{
			name: "mixed platforms deduped",
			triggers: []any{
				map[string]any{"platform": "state"},
				map[string]any{"platform": "time"},
				map[string]any{"platform": "state"}, // duplicate
			},
			want: []string{"state", "time"},
		},
		{
			name:     "empty triggers",
			triggers: nil,
			want:     nil,
		},
		{
			name: "trigger without platform",
			triggers: []any{
				map[string]any{"at": "06:30:00"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := extractTriggerTypes(tt.triggers)
			if len(result) != len(tt.want) {
				t.Errorf("got %v, want %v", result, tt.want)
				return
			}
			for i, typ := range result {
				if typ != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, typ, tt.want[i])
				}
			}
		})
	}
}
