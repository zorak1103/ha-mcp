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

func TestJSONFormatter_FormatServiceResponse(t *testing.T) {
	f := NewJSONFormatter()
	response := map[string]any{"weather.home": map[string]any{"forecast": []any{"sunny"}}}

	result, err := f.FormatServiceResponse(context.Background(), "weather", "get_forecasts", []string{"weather.home"}, response)
	if err != nil {
		t.Fatalf("FormatServiceResponse() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatServiceResponse() returned invalid JSON: %v", err)
	}
	if parsed["success"] != true {
		t.Errorf("success = %v, want true", parsed["success"])
	}
	if parsed["affected_entities"] != float64(1) {
		t.Errorf("affected_entities = %v, want 1", parsed["affected_entities"])
	}
	if entityIDs, ok := parsed["entity_ids"].([]any); !ok || len(entityIDs) != 1 || entityIDs[0] != "weather.home" {
		t.Errorf("entity_ids = %#v, want [weather.home]", parsed["entity_ids"])
	}
	if parsed["state_changes_polled"] != false {
		t.Errorf("state_changes_polled = %v, want false", parsed["state_changes_polled"])
	}
	if parsed["response"] == nil {
		t.Error("response is nil, want payload")
	}
}

func TestJSONFormatter_FormatServiceResponse_MarksOversizedPayload(t *testing.T) {
	f := NewJSONFormatter()
	response := map[string]any{"weather.home": map[string]any{"forecast": strings.Repeat("x", 10000)}}

	result, err := f.FormatServiceResponse(context.Background(), "weather", "get_forecasts", nil, response)
	if err != nil {
		t.Fatalf("FormatServiceResponse() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatServiceResponse() returned invalid JSON: %v", err)
	}
	if parsed["response_complete"] != true {
		t.Errorf("response_complete = %v, want true", parsed["response_complete"])
	}
	if responseBytes, ok := parsed["response_bytes"].(float64); !ok || responseBytes <= 10000 {
		t.Errorf("response_bytes = %#v, want payload size above 10000", parsed["response_bytes"])
	}
	if parsed["response"] == nil {
		t.Error("response is nil, want complete payload")
	}
}

func TestJSONFormatter_FormatServiceResponse_ExactLimitIsNotMarked(t *testing.T) {
	f := NewJSONFormatter()
	response := map[string]any{"value": strings.Repeat("x", maxServiceResponseChars)}
	var payload []byte
	for size := 0; size <= maxServiceResponseChars; size++ {
		response["value"] = strings.Repeat("x", size)
		var err error
		payload, err = json.Marshal(response)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if len(payload) == maxServiceResponseChars {
			break
		}
	}
	if len(payload) != maxServiceResponseChars {
		t.Fatalf("could not build exact-limit payload; got %d bytes", len(payload))
	}

	result, err := f.FormatServiceResponse(context.Background(), "test", "respond", nil, response)
	if err != nil {
		t.Fatalf("FormatServiceResponse() error = %v", err)
	}
	if strings.Contains(result, "response_bytes") {
		t.Errorf("FormatServiceResponse() marked exact-limit payload oversized: %s", result)
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
