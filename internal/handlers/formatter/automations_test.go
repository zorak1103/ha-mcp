package formatter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestNaturalAutomationFormatter_FormatList(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalAutomationFormatter()

	// Test data
	now := time.Now()
	fiveMinAgo := now.Add(-5 * time.Minute)
	oneHourAgo := now.Add(-1 * time.Hour)

	automations := []homeassistant.Automation{
		{
			EntityID:      "automation.motion_lights",
			State:         "on",
			FriendlyName:  "Motion Lights",
			LastTriggered: fiveMinAgo.Format(time.RFC3339),
		},
		{
			EntityID:      "automation.morning_routine",
			State:         "on",
			FriendlyName:  "Morning Routine",
			LastTriggered: oneHourAgo.Format(time.RFC3339),
		},
		{
			EntityID:     "automation.vacation_mode",
			State:        "off",
			FriendlyName: "Vacation Mode",
		},
	}

	configs := map[string]*homeassistant.AutomationConfig{
		"motion_lights": {
			Mode: "single",
		},
		"morning_routine": {
			Mode: "queued",
		},
		"vacation_mode": {
			Mode: "parallel",
		},
	}

	opts := AutomationListOptions{}
	result, err := f.FormatList(ctx, automations, configs, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify summary line
	if !strings.Contains(result, "3 automations") {
		t.Errorf("expected '3 automations' in output, got: %s", result)
	}
	if !strings.Contains(result, "2 enabled") {
		t.Errorf("expected '2 enabled' in output, got: %s", result)
	}
	if !strings.Contains(result, "1 disabled") {
		t.Errorf("expected '1 disabled' in output, got: %s", result)
	}

	// Verify mode breakdown
	if !strings.Contains(result, "By mode:") {
		t.Errorf("expected 'By mode:' in output, got: %s", result)
	}

	// Verify recently triggered section
	if !strings.Contains(result, "Recently triggered:") {
		t.Errorf("expected 'Recently triggered:' in output, got: %s", result)
	}
	if !strings.Contains(result, "Motion Lights") {
		t.Errorf("expected 'Motion Lights' in output, got: %s", result)
	}

	// Verify disabled section
	if !strings.Contains(result, "Disabled") {
		t.Errorf("expected 'Disabled' section in output, got: %s", result)
	}
	if !strings.Contains(result, "Vacation Mode") {
		t.Errorf("expected 'Vacation Mode' in disabled section, got: %s", result)
	}
}

func TestNaturalAutomationFormatter_FormatList_Empty(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalAutomationFormatter()

	result, err := f.FormatList(ctx, nil, nil, AutomationListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != MsgNoAutomationsFound {
		t.Errorf("expected '%s', got: %s", MsgNoAutomationsFound, result)
	}
}

func TestNaturalAutomationFormatter_FormatList_Verbose(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalAutomationFormatter()

	automations := []homeassistant.Automation{
		{
			EntityID:     "automation.test",
			State:        "on",
			FriendlyName: "Test Automation",
		},
	}

	configs := map[string]*homeassistant.AutomationConfig{
		"test": {
			Mode:     "single",
			Triggers: []any{map[string]any{"platform": "state", "entity_id": "binary_sensor.motion"}},
			Actions:  []any{map[string]any{"service": "light.turn_on", "target": map[string]any{"entity_id": "light.living"}}},
		},
	}

	opts := AutomationListOptions{Verbose: true}
	result, err := f.FormatList(ctx, automations, configs, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verbose should include triggers/actions info
	if !strings.Contains(result, "trigger") || !strings.Contains(result, "action") {
		t.Errorf("verbose output should mention triggers/actions, got: %s", result)
	}
}

func TestNaturalAutomationFormatter_FormatDetail(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalAutomationFormatter()

	automation := homeassistant.Automation{
		EntityID:      "automation.motion_lights",
		State:         "on",
		FriendlyName:  "Motion Lights",
		LastTriggered: time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		Config: &homeassistant.AutomationConfig{
			ID:          "motion_lights",
			Alias:       "Motion Lights",
			Description: "Turn on lights when motion detected",
			Mode:        "single",
			Triggers: []any{
				map[string]any{
					"platform":  "state",
					"entity_id": "binary_sensor.motion_kitchen",
					"from":      "off",
					"to":        "on",
				},
			},
			Actions: []any{
				map[string]any{
					"service": "light.turn_on",
					"target":  map[string]any{"entity_id": "light.kitchen"},
					"data":    map[string]any{"brightness": 255},
				},
			},
		},
	}

	result, err := f.FormatDetail(ctx, automation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify header
	if !strings.Contains(result, "Motion Lights") {
		t.Errorf("expected 'Motion Lights' in output, got: %s", result)
	}
	if !strings.Contains(result, "enabled") {
		t.Errorf("expected 'enabled' state in output, got: %s", result)
	}

	// Verify mode and last triggered
	if !strings.Contains(result, "Mode:") && !strings.Contains(result, "single") {
		t.Errorf("expected mode info in output, got: %s", result)
	}

	// Verify triggers section
	if !strings.Contains(result, "Trigger") {
		t.Errorf("expected 'Triggers' section in output, got: %s", result)
	}
	if !strings.Contains(result, "binary_sensor.motion_kitchen") {
		t.Errorf("expected trigger entity in output, got: %s", result)
	}

	// Verify actions section
	if !strings.Contains(result, "Action") {
		t.Errorf("expected 'Actions' section in output, got: %s", result)
	}
	if !strings.Contains(result, "light.turn_on") {
		t.Errorf("expected action service in output, got: %s", result)
	}
}

func TestNaturalAutomationFormatter_FormatDetail_NoConfig(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalAutomationFormatter()

	automation := homeassistant.Automation{
		EntityID:     "automation.simple",
		State:        "on",
		FriendlyName: "Simple Automation",
	}

	result, err := f.FormatDetail(ctx, automation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Simple Automation") {
		t.Errorf("expected 'Simple Automation' in output, got: %s", result)
	}
}

func TestJSONAutomationFormatter_FormatList(t *testing.T) {
	ctx := context.Background()
	f := NewJSONAutomationFormatter()

	automations := []homeassistant.Automation{
		{
			EntityID:     "automation.test",
			State:        "on",
			FriendlyName: "Test",
		},
	}

	result, err := f.FormatList(ctx, automations, nil, AutomationListOptions{})
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

func TestJSONAutomationFormatter_FormatList_Empty(t *testing.T) {
	ctx := context.Background()
	f := NewJSONAutomationFormatter()

	result, err := f.FormatList(ctx, nil, nil, AutomationListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be empty JSON array
	if result != "[]" {
		t.Errorf("expected '[]', got: %s", result)
	}
}

func TestJSONAutomationFormatter_FormatDetail(t *testing.T) {
	ctx := context.Background()
	f := NewJSONAutomationFormatter()

	automation := homeassistant.Automation{
		EntityID:     "automation.test",
		State:        "on",
		FriendlyName: "Test",
		Config: &homeassistant.AutomationConfig{
			Mode: "single",
		},
	}

	result, err := f.FormatDetail(ctx, automation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("expected valid JSON object, got error: %v", err)
	}

	if parsed["entity_id"] != "automation.test" {
		t.Errorf("expected entity_id 'automation.test', got: %v", parsed["entity_id"])
	}
}

func TestNewAutomationFormatter(t *testing.T) {
	natural := NewAutomationFormatter(FormatNatural)
	if _, ok := natural.(*NaturalAutomationFormatter); !ok {
		t.Errorf("expected NaturalAutomationFormatter for natural format")
	}

	jsonFmt := NewAutomationFormatter(FormatJSON)
	if _, ok := jsonFmt.(*JSONAutomationFormatter); !ok {
		t.Errorf("expected JSONAutomationFormatter for json format")
	}

	defaultFmt := NewAutomationFormatter("")
	if _, ok := defaultFmt.(*NaturalAutomationFormatter); !ok {
		t.Errorf("expected NaturalAutomationFormatter for empty format")
	}
}
