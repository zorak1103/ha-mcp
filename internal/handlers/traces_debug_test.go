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
					"run_id": "1708329000.123",
					"state":  "stopped",
					"timestamp": map[string]any{
						"start":  now.Add(-1 * time.Hour).Format(time.RFC3339),
						"finish": now.Add(-1*time.Hour + 75*time.Second).Format(time.RFC3339),
					},
					"trigger": map[string]any{"platform": "time", "description": "time (06:30:00)"},
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

// TestHandleDebugTrace_ArrayEntityTriggers is a regression test for issue #99:
// automations with array entity_id and the modern `trigger:` key must populate the
// Trigger Entity States section instead of reporting "No entity-based triggers found".
func TestHandleDebugTrace_ArrayEntityTriggers(t *testing.T) {
	t.Parallel()

	now := time.Now()
	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			// Exact config from the issue report (HA 2024.10+ trigger: key, array entity_id).
			return &homeassistant.Automation{
				EntityID:     "automation.example",
				State:        "on",
				FriendlyName: "Example",
				Config: &homeassistant.AutomationConfig{
					Alias: "Example",
					Mode:  "single",
					Triggers: []any{
						map[string]any{
							"trigger":   "state",
							"entity_id": []any{"binary_sensor.example_sensor_a", "binary_sensor.example_sensor_b"},
							"to":        "on",
						},
						map[string]any{
							"trigger":   "state",
							"entity_id": []any{"binary_sensor.example_sensor_a", "binary_sensor.example_sensor_b"},
							"to":        []any{"off", "unavailable", "unknown"},
							"for":       map[string]any{"minutes": 15},
						},
					},
					Conditions: []any{},
					Actions:    []any{},
				},
			}, nil
		},
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{}, nil
		},
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			stateMap := map[string]string{
				"binary_sensor.example_sensor_a": "off",
				"binary_sensor.example_sensor_b": "unavailable",
			}
			s, ok := stateMap[entityID]
			if !ok {
				return nil, fmt.Errorf("entity not found: %s", entityID)
			}
			return &homeassistant.Entity{
				EntityID:    entityID,
				State:       s,
				LastChanged: now.Add(-15 * time.Minute),
			}, nil
		},
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{}, nil
		},
	}

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "debug",
		"automation_id": "automation.example",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text

	// Both entities from the array triggers must appear, each with their state.
	for _, want := range []string{
		"binary_sensor.example_sensor_a",
		"binary_sensor.example_sensor_b",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Trigger Entity States section missing %q\nfull output:\n%s", want, text)
		}
	}
	if strings.Contains(text, "No entity-based triggers found") {
		t.Errorf("got 'No entity-based triggers found' but entity triggers exist\nfull output:\n%s", text)
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
		// Regression cases for issue #99 ─────────────────────────────────────────
		{
			name: "array entity_id with modern trigger key",
			triggers: []any{
				map[string]any{
					"trigger":   "state",
					"entity_id": []any{"binary_sensor.a", "binary_sensor.b"},
					"to":        "on",
				},
			},
			wantIDs:   []string{"binary_sensor.a", "binary_sensor.b"},
			wantTypes: []string{"state", "state"},
		},
		{
			name: "modern trigger key for type (no platform key)",
			triggers: []any{
				map[string]any{"trigger": "state", "entity_id": "sensor.motion"},
			},
			wantIDs:   []string{"sensor.motion"},
			wantTypes: []string{"state"},
		},
		{
			name: "dedup across array and scalar trigger",
			triggers: []any{
				map[string]any{"trigger": "state", "entity_id": []any{"sensor.a", "sensor.b"}},
				map[string]any{"trigger": "state", "entity_id": "sensor.a"}, // duplicate
				map[string]any{"trigger": "state", "entity_id": "sensor.c"},
			},
			wantIDs:   []string{"sensor.a", "sensor.b", "sensor.c"},
			wantTypes: []string{"state", "state", "state"},
		},
		{
			name: "array entity_id with non-string elements skipped",
			triggers: []any{
				map[string]any{
					"trigger":   "state",
					"entity_id": []any{"sensor.good", 42, nil, "sensor.also_good"},
				},
			},
			wantIDs:   []string{"sensor.good", "sensor.also_good"},
			wantTypes: []string{"state", "state"},
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
		// Regression case for issue #99: modern `trigger:` key (HA 2024.10+)
		{
			name: "modern trigger key (no platform key)",
			triggers: []any{
				map[string]any{"trigger": "state"},
				map[string]any{"trigger": "time"},
				map[string]any{"trigger": "state"}, // duplicate
			},
			want: []string{"state", "time"},
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

// TestFetchLatestTrace_PicksNewestByStartTimestamp verifies that the trace with the latest
// timestamp.start wins, not the first element returned by trace/list (HA yields insertion
// order, oldest first - see components/trace/models.py ActionTrace.as_short_dict).
func TestFetchLatestTrace_PicksNewestByStartTimestamp(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{
				map[string]any{
					"run_id": "oldest",
					"state":  "stopped",
					"timestamp": map[string]any{
						"start":  "2026-02-19T06:00:00Z",
						"finish": "2026-02-19T06:00:01Z",
					},
				},
				map[string]any{
					"run_id": "newest",
					"state":  "stopped",
					"timestamp": map[string]any{
						"start":  "2026-02-19T08:00:00Z",
						"finish": "2026-02-19T08:00:03Z",
					},
				},
				map[string]any{
					"run_id": "middle",
					"state":  "stopped",
					"timestamp": map[string]any{
						"start":  "2026-02-19T07:00:00Z",
						"finish": "2026-02-19T07:00:01Z",
					},
				},
			}, nil
		},
	}

	got, _ := handler.fetchLatestTrace(context.Background(), client, "automation.morning_routine")
	if got == nil {
		t.Fatal("expected a trace, got nil")
	}
	if got.RunID != "newest" {
		t.Errorf("RunID = %q, want %q (picked wrong trace as latest)", got.RunID, "newest")
	}
	if got.Timestamp != "2026-02-19T08:00:00Z" {
		t.Errorf("Timestamp = %q, want the newest trace's start timestamp", got.Timestamp)
	}
	if got.Duration != 3 {
		t.Errorf("Duration = %v, want 3 (derived from finish - start)", got.Duration)
	}
}

// TestFetchLatestTrace_LegacyStringTimestamp verifies backward compatibility with a flat
// ISO-string timestamp (as opposed to HA's real {"start":..,"finish":..} shape), which older
// test fixtures and any future HA response shape drift might still produce.
// TestFetchLatestTrace_MissingTimestamp verifies that a trace with no (or malformed) timestamp
// field degrades to an empty Timestamp and zero Duration rather than panicking - Home Assistant's
// trace short dict always includes {"start":..,"finish":..}, but nothing here should crash if a
// future response shape omits it.
func TestFetchLatestTrace_MissingTimestamp(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		SendHACSCommandFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return []any{
				map[string]any{
					"run_id": "only",
					"state":  "stopped",
				},
			}, nil
		},
	}

	got, _ := handler.fetchLatestTrace(context.Background(), client, "automation.morning_routine")
	if got == nil {
		t.Fatal("expected a trace, got nil")
	}
	if got.Timestamp != "" {
		t.Errorf("Timestamp = %q, want empty", got.Timestamp)
	}
	if got.Duration != 0 {
		t.Errorf("Duration = %v, want 0", got.Duration)
	}
}

// TestFetchLatestTrace_UsesResolvedItemID verifies fetchLatestTrace sends the registry-resolved
// item_id in its trace/list command, not the raw entity_id - the same regression class as
// TestManageTrace_List_EntityIDFilter, but for the debug action's own trace/list call.
func TestFetchLatestTrace_UsesResolvedItemID(t *testing.T) {
	t.Parallel()

	var capturedItemID any
	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		GetEntityRegistryEntryFn: func(_ context.Context, entityID string) (*homeassistant.EntityRegistryEntry, error) {
			return &homeassistant.EntityRegistryEntry{EntityID: entityID, UniqueID: "1700000000001"}, nil
		},
		SendHACSCommandFn: func(_ context.Context, _ string, data map[string]any) (any, error) {
			capturedItemID = data["item_id"]
			return []any{}, nil
		},
	}

	got, resolved := handler.fetchLatestTrace(context.Background(), client, "automation.morning_routine")
	if got != nil {
		t.Errorf("expected nil trace for an empty trace/list response, got: %+v", got)
	}
	if !resolved {
		t.Error("resolved = false, want true (registry lookup succeeded)")
	}
	if capturedItemID != "1700000000001" {
		t.Errorf("data[item_id] = %v, want the resolved unique_id %q", capturedItemID, "1700000000001")
	}
}

// TestHandleDebugTrace_UnresolvedItemIDWarning verifies the debug report's "Latest Trace"
// section surfaces the resolution-failure warning (not the plain "No traces found.") when
// trace/list comes back empty following a failed item_id lookup - same signal as list/get.
func TestHandleDebugTrace_UnresolvedItemIDWarning(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	client := &UniversalMockClient{
		// No GetEntityRegistryEntryFn - lookup misses, resolution degrades to the object_id.
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
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "could not be resolved") {
		t.Errorf("expected the resolution-failure warning in the Latest Trace section, got: %s", result.Content[0].Text)
	}
}

// TestBuildDebugTrace_ScriptExecutionAndError verifies that "script_execution" and "error" -
// present on HA's real trace short dict (components/trace/models.py ActionTrace.as_short_dict) -
// are extracted into AutomationDebugTrace and rendered in the "Latest Trace" section. Before this
// test only run_id/state/timestamp/trigger/not_triggered were read; a failed automation run
// (script_execution="failed_conditions" or an "error" string) was silently indistinguishable from
// a successful one in the debug report.
func TestBuildDebugTrace_ScriptExecutionAndError(t *testing.T) {
	t.Parallel()

	trace := buildDebugTrace(map[string]any{
		"run_id":           "abc123",
		"state":            "stopped",
		"script_execution": "failed_conditions",
		"error":            "condition not met",
		"timestamp": map[string]any{
			"start":  "2026-02-19T06:00:00Z",
			"finish": "2026-02-19T06:00:01Z",
		},
	})

	if trace.ScriptExecution != "failed_conditions" {
		t.Errorf("ScriptExecution = %q, want %q", trace.ScriptExecution, "failed_conditions")
	}
	if trace.Error != "condition not met" {
		t.Errorf("Error = %q, want %q", trace.Error, "condition not met")
	}

	got := formatDebugTraceSection(trace, "")
	for _, want := range []string{"Result: failed_conditions", "Error: condition not met"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

// TestFormatDebugTraceSection_NotTriggered verifies the not_triggered annotation on the Run ID
// line, which was never previously asserted on by any test.
func TestFormatDebugTraceSection_NotTriggered(t *testing.T) {
	t.Parallel()

	trace := buildDebugTrace(map[string]any{
		"run_id":        "abc123",
		"state":         "stopped",
		"not_triggered": true,
	})

	if !trace.NotTriggered {
		t.Fatal("expected NotTriggered = true")
	}

	got := formatDebugTraceSection(trace, "")
	want := "Run ID: abc123 (not triggered - condition evaluated but did not fire)"
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q\nfull output:\n%s", want, got)
	}
}
