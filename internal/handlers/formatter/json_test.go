package formatter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestJSONFormatter_FormatEntity(t *testing.T) {
	f := NewJSONFormatter()

	entity := homeassistant.Entity{
		EntityID: "light.living_room",
		State:    "on",
		Attributes: map[string]any{
			"friendly_name": "Living Room Light",
			"brightness":    255,
		},
		LastChanged: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}

	result, err := f.FormatEntity(context.Background(), entity)
	if err != nil {
		t.Fatalf("FormatEntity() error = %v", err)
	}

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatEntity() returned invalid JSON: %v", err)
	}

	// Should contain entity_id
	if parsed["entity_id"] != "light.living_room" {
		t.Errorf("FormatEntity() entity_id = %v, want %q", parsed["entity_id"], "light.living_room")
	}

	// Should contain state
	if parsed["state"] != "on" {
		t.Errorf("FormatEntity() state = %v, want %q", parsed["state"], "on")
	}
}

func TestJSONFormatter_FormatEntities_Verbose(t *testing.T) {
	f := NewJSONFormatter()

	entities := []homeassistant.Entity{
		{
			EntityID: "light.living_room",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Living Room Light",
				"brightness":    255,
			},
		},
	}

	result, err := f.FormatEntities(context.Background(), entities, EntityListOptions{Verbose: true})
	if err != nil {
		t.Fatalf("FormatEntities() error = %v", err)
	}

	// Should contain full attributes
	if !strings.Contains(result, "brightness") {
		t.Errorf("FormatEntities(verbose=true) should contain attributes, got %q", result)
	}
}

func TestJSONFormatter_FormatEntities_Compact(t *testing.T) {
	f := NewJSONFormatter()

	entities := []homeassistant.Entity{
		{
			EntityID: "light.living_room",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Living Room Light",
				"brightness":    255,
			},
		},
	}

	result, err := f.FormatEntities(context.Background(), entities, EntityListOptions{Verbose: false})
	if err != nil {
		t.Fatalf("FormatEntities() error = %v", err)
	}

	// Should be valid JSON array
	var parsed []compactEntity
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatEntities() returned invalid JSON: %v", err)
	}

	// Should have compact format
	if len(parsed) != 1 {
		t.Fatalf("FormatEntities() length = %d, want 1", len(parsed))
	}

	if parsed[0].EntityID != "light.living_room" {
		t.Errorf("FormatEntities() entity_id = %q, want %q", parsed[0].EntityID, "light.living_room")
	}

	// Should NOT contain brightness (compact mode)
	if strings.Contains(result, "brightness") {
		t.Errorf("FormatEntities(verbose=false) should not contain attributes, got %q", result)
	}
}

func TestJSONFormatter_FormatHistory(t *testing.T) {
	f := NewJSONFormatter()

	now := time.Now()
	entries := []homeassistant.HistoryEntry{
		{
			EntityID:    "light.living_room",
			State:       "on",
			LastChanged: float64(now.Unix()),
		},
	}

	result, err := f.FormatHistory(context.Background(), "light.living_room", entries, HistoryOptions{})
	if err != nil {
		t.Fatalf("FormatHistory() error = %v", err)
	}

	// Should be valid JSON
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatHistory() returned invalid JSON: %v", err)
	}
}

func TestJSONFormatter_FormatServiceSuccess(t *testing.T) {
	f := NewJSONFormatter()

	result, err := f.FormatServiceSuccess(context.Background(), "light", "turn_on", []string{"light.living_room"}, nil)
	if err != nil {
		t.Fatalf("FormatServiceSuccess() error = %v", err)
	}

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatServiceSuccess() returned invalid JSON: %v", err)
	}

	// Should have success
	if parsed["success"] != true {
		t.Errorf("FormatServiceSuccess() success = %v, want true", parsed["success"])
	}

	// Should have domain
	if parsed["domain"] != "light" {
		t.Errorf("FormatServiceSuccess() domain = %v, want %q", parsed["domain"], "light")
	}
}

func TestJSONFormatter_FormatError(t *testing.T) {
	f := NewJSONFormatter()
	err := &testError{msg: "connection refused"}
	result := f.FormatError(context.Background(), err)

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatError() returned invalid JSON: %v", err)
	}

	// Should have error flag
	if parsed["error"] != true {
		t.Errorf("FormatError() error = %v, want true", parsed["error"])
	}

	// Should have message
	if parsed["message"] != "connection refused" {
		t.Errorf("FormatError() message = %v, want %q", parsed["message"], "connection refused")
	}
}
