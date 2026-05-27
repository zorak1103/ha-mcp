package formatter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestNaturalScriptFormatter_FormatList(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalScriptFormatter()

	scripts := []homeassistant.Script{
		{
			EntityID:      "script.morning_routine",
			State:         "off",
			FriendlyName:  "Morning Routine",
			LastTriggered: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			Config: &homeassistant.ScriptConfig{
				Mode:     "single",
				Sequence: []any{map[string]any{"service": "light.turn_on"}, map[string]any{"service": "scene.turn_on"}},
			},
		},
		{
			EntityID:      "script.bedtime",
			State:         "off",
			FriendlyName:  "Bedtime",
			LastTriggered: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			Config: &homeassistant.ScriptConfig{
				Mode:     "queued",
				Sequence: []any{map[string]any{"service": "light.turn_off"}},
			},
		},
		{
			EntityID:     "script.quick_clean",
			State:        "off",
			FriendlyName: "Quick Clean",
			Config: &homeassistant.ScriptConfig{
				Mode:     "parallel",
				Sequence: []any{map[string]any{"service": "vacuum.start"}},
			},
		},
	}

	opts := ScriptListOptions{}
	result, err := f.FormatList(ctx, scripts, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify summary line
	if !strings.Contains(result, "3 scripts") {
		t.Errorf("expected '3 scripts' in output, got: %s", result)
	}

	// Verify mode breakdown
	if !strings.Contains(result, "By mode:") {
		t.Errorf("expected 'By mode:' in output, got: %s", result)
	}

	// Verify script names
	if !strings.Contains(result, "Morning Routine") {
		t.Errorf("expected 'Morning Routine' in output, got: %s", result)
	}
}

func TestNaturalScriptFormatter_FormatList_Empty(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalScriptFormatter()

	result, err := f.FormatList(ctx, nil, ScriptListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != MsgNoScriptsFound {
		t.Errorf("expected '%s', got: %s", MsgNoScriptsFound, result)
	}
}

func TestNaturalScriptFormatter_FormatList_Verbose(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalScriptFormatter()

	scripts := []homeassistant.Script{
		{
			EntityID:     "script.test",
			State:        "off",
			FriendlyName: "Test Script",
			Config: &homeassistant.ScriptConfig{
				Mode:     "single",
				Sequence: []any{map[string]any{"service": "light.turn_on"}, map[string]any{"delay": "00:00:05"}},
			},
		},
	}

	opts := ScriptListOptions{Verbose: true}
	result, err := f.FormatList(ctx, scripts, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verbose should include step count
	if !strings.Contains(result, "step") {
		t.Errorf("verbose output should mention steps, got: %s", result)
	}
}

func TestNaturalScriptFormatter_FormatDetail(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalScriptFormatter()

	script := homeassistant.Script{
		EntityID:      "script.morning_routine",
		State:         "off",
		FriendlyName:  "Morning Routine",
		LastTriggered: time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		Config: &homeassistant.ScriptConfig{
			Alias:       "Morning Routine",
			Description: "Starts the morning activities",
			Mode:        "single",
			Icon:        "mdi:weather-sunny",
			Sequence: []any{
				map[string]any{"service": "light.turn_on", "target": map[string]any{"entity_id": "light.bedroom"}},
				map[string]any{"delay": "00:00:30"},
				map[string]any{"service": "media_player.play_media", "target": map[string]any{"entity_id": "media_player.speaker"}},
			},
		},
	}

	result, err := f.FormatDetail(ctx, script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify header
	if !strings.Contains(result, "Morning Routine") {
		t.Errorf("expected 'Morning Routine' in output, got: %s", result)
	}

	// Verify mode
	if !strings.Contains(result, "single") {
		t.Errorf("expected mode 'single' in output, got: %s", result)
	}

	// Verify sequence steps
	if !strings.Contains(result, "Sequence") {
		t.Errorf("expected 'Sequence' section in output, got: %s", result)
	}
	if !strings.Contains(result, "light.turn_on") {
		t.Errorf("expected 'light.turn_on' in output, got: %s", result)
	}
}

func TestNaturalScriptFormatter_ModernActionKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := NewNaturalScriptFormatter()

	script := homeassistant.Script{
		EntityID:     "script.dishwasher_on",
		State:        "off",
		FriendlyName: "Dishwasher On",
		Config: &homeassistant.ScriptConfig{
			Alias: "Dishwasher On",
			Sequence: []any{
				map[string]any{
					"action": "script.turn_on",
					"target": map[string]any{"entity_id": "script.kitchen_dishwasher_on"},
				},
				map[string]any{
					"action": "homeassistant.reload_config_entry",
				},
			},
		},
	}

	result, err := f.FormatDetail(ctx, script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "script.turn_on") {
		t.Errorf("expected 'script.turn_on' in output, got: %s", result)
	}
	if !strings.Contains(result, "script.kitchen_dishwasher_on") {
		t.Errorf("expected target 'script.kitchen_dishwasher_on' in output, got: %s", result)
	}
	if !strings.Contains(result, "homeassistant.reload_config_entry") {
		t.Errorf("expected 'homeassistant.reload_config_entry' in output, got: %s", result)
	}
}

func TestJSONScriptFormatter_FormatList(t *testing.T) {
	ctx := context.Background()
	f := NewJSONScriptFormatter()

	scripts := []homeassistant.Script{
		{
			EntityID:     "script.test",
			State:        "off",
			FriendlyName: "Test",
		},
	}

	result, err := f.FormatList(ctx, scripts, ScriptListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("expected valid JSON array, got error: %v", err)
	}

	if len(parsed) != 1 {
		t.Errorf("expected 1 item in JSON, got: %d", len(parsed))
	}
}

func TestJSONScriptFormatter_FormatList_Empty(t *testing.T) {
	ctx := context.Background()
	f := NewJSONScriptFormatter()

	result, err := f.FormatList(ctx, nil, ScriptListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "[]" {
		t.Errorf("expected '[]', got: %s", result)
	}
}

func TestJSONScriptFormatter_FormatDetail(t *testing.T) {
	ctx := context.Background()
	f := NewJSONScriptFormatter()

	script := homeassistant.Script{
		EntityID:     "script.test",
		State:        "off",
		FriendlyName: "Test",
		Config: &homeassistant.ScriptConfig{
			Mode: "single",
		},
	}

	result, err := f.FormatDetail(ctx, script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("expected valid JSON object, got error: %v", err)
	}

	if parsed["entity_id"] != "script.test" {
		t.Errorf("expected entity_id 'script.test', got: %v", parsed["entity_id"])
	}
}

func TestNewScriptFormatter(t *testing.T) {
	natural := NewScriptFormatter(FormatNatural)
	if _, ok := natural.(*NaturalScriptFormatter); !ok {
		t.Errorf("expected NaturalScriptFormatter for natural format")
	}

	jsonFmt := NewScriptFormatter(FormatJSON)
	if _, ok := jsonFmt.(*JSONScriptFormatter); !ok {
		t.Errorf("expected JSONScriptFormatter for json format")
	}

	defaultFmt := NewScriptFormatter("")
	if _, ok := defaultFmt.(*NaturalScriptFormatter); !ok {
		t.Errorf("expected NaturalScriptFormatter for empty format")
	}
}

func TestNaturalScriptFormatter_FormatDetail_Max(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalScriptFormatter()

	t.Run("max shown for parallel mode", func(t *testing.T) {
		script := homeassistant.Script{
			EntityID: "script.parallel_test",
			State:    "on",
			Config: &homeassistant.ScriptConfig{
				Alias:    "Parallel Test",
				Mode:     "parallel",
				Max:      3,
				Sequence: []any{},
			},
		}
		result, err := f.FormatDetail(ctx, script)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "(max: 3)") {
			t.Errorf("expected '(max: 3)' in output, got: %s", result)
		}
	})

	t.Run("max not shown when zero", func(t *testing.T) {
		script := homeassistant.Script{
			EntityID: "script.single_test",
			State:    "on",
			Config: &homeassistant.ScriptConfig{
				Alias:    "Single Test",
				Mode:     "single",
				Sequence: []any{},
			},
		}
		result, err := f.FormatDetail(ctx, script)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(result, "max:") || strings.Contains(result, "(max") {
			t.Errorf("expected no max in output for zero Max, got: %s", result)
		}
	})
}
