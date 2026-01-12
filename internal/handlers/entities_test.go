// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
)

// mockEntityClient implements homeassistant.Client for testing.
type mockEntityClient struct {
	homeassistant.Client
	states []homeassistant.Entity
}

func (m *mockEntityClient) GetStates(_ context.Context) ([]homeassistant.Entity, error) {
	return m.states, nil
}

func TestHandleGetStates(t *testing.T) {
	testStates := []homeassistant.Entity{
		{
			EntityID: "light.living_room",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Living Room Light",
				"brightness":    255,
			},
		},
		{
			EntityID: "light.bedroom",
			State:    "off",
			Attributes: map[string]any{
				"friendly_name": "Bedroom Light",
			},
		},
		{
			EntityID: "switch.kitchen",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Kitchen Switch",
			},
		},
		{
			EntityID: "sensor.temperature",
			State:    "21.5",
			Attributes: map[string]any{
				"friendly_name":       "Temperature",
				"unit_of_measurement": "°C",
			},
		},
	}

	tests := []struct {
		name            string
		args            map[string]any
		wantEntityCount int
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:            "no filters - compact output",
			args:            map[string]any{},
			wantEntityCount: 4,
			wantContains:    []string{"light.living_room", "Living Room Light", "switch.kitchen", "sensor.temperature"},
			wantNotContains: []string{"brightness", "unit_of_measurement", "last_changed"},
		},
		{
			name:            "domain filter - light only",
			args:            map[string]any{"domain": "light"},
			wantEntityCount: 2,
			wantContains:    []string{"light.living_room", "light.bedroom"},
			wantNotContains: []string{"switch.kitchen", "sensor.temperature"},
		},
		{
			name:            "domain filter - switch only",
			args:            map[string]any{"domain": "switch"},
			wantEntityCount: 1,
			wantContains:    []string{"switch.kitchen", "Kitchen Switch"},
			wantNotContains: []string{"light.living_room", "sensor.temperature"},
		},
		{
			name:            "state filter - on only",
			args:            map[string]any{"state": "on"},
			wantEntityCount: 2,
			wantContains:    []string{"light.living_room", "switch.kitchen"},
			wantNotContains: []string{"light.bedroom"},
		},
		{
			name:            "state filter - off only",
			args:            map[string]any{"state": "off"},
			wantEntityCount: 1,
			wantContains:    []string{"light.bedroom"},
			wantNotContains: []string{"light.living_room", "switch.kitchen", "sensor.temperature"},
		},
		{
			name:            "state_not filter - exclude off",
			args:            map[string]any{"state_not": "off"},
			wantEntityCount: 3,
			wantContains:    []string{"light.living_room", "switch.kitchen", "sensor.temperature"},
			wantNotContains: []string{"light.bedroom"},
		},
		{
			name:            "name_contains filter - by entity_id",
			args:            map[string]any{"name_contains": "living"},
			wantEntityCount: 1,
			wantContains:    []string{"light.living_room"},
			wantNotContains: []string{"light.bedroom", "switch.kitchen"},
		},
		{
			name:            "name_contains filter - by friendly_name",
			args:            map[string]any{"name_contains": "kitchen"},
			wantEntityCount: 1,
			wantContains:    []string{"switch.kitchen", "Kitchen Switch"},
			wantNotContains: []string{"light.living_room"},
		},
		{
			name:            "name_contains filter - case insensitive",
			args:            map[string]any{"name_contains": "BEDROOM"},
			wantEntityCount: 1,
			wantContains:    []string{"light.bedroom"},
		},
		{
			name:            "combined filters - domain and state",
			args:            map[string]any{"domain": "light", "state": "on"},
			wantEntityCount: 1,
			wantContains:    []string{"light.living_room"},
			wantNotContains: []string{"light.bedroom", "switch.kitchen"},
		},
		{
			name:            "verbose mode includes all attributes",
			args:            map[string]any{"verbose": true, "domain": "light"},
			wantEntityCount: 2,
			wantContains:    []string{"brightness", "255", "attributes"},
		},
		{
			name:            "verbose mode includes timestamps",
			args:            map[string]any{"verbose": true, "domain": "sensor"},
			wantEntityCount: 1,
			wantContains:    []string{"unit_of_measurement", "°C", "last_changed"},
		},
		{
			name:            "compact mode shows state",
			args:            map[string]any{"domain": "light"},
			wantEntityCount: 2,
			wantContains:    []string{`"state": "on"`, `"state": "off"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &EntityHandlers{}
			client := &mockEntityClient{states: testStates}

			result, err := h.handleGetStates(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleGetStates() error = %v", err)
			}

			if result.IsError {
				t.Fatalf("handleGetStates() returned error: %v", result.Content)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetStates() returned no content")
			}

			content := result.Content[0].Text

			// Check entity count in summary
			expectedSummary := "Found " + itoa(tt.wantEntityCount) + " entities"
			if !strings.Contains(content, expectedSummary) {
				t.Errorf("Expected summary %q not found in content:\n%s", expectedSummary, content[:min(200, len(content))])
			}

			// Check contains
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, but it didn't.\nContent: %s", want, content)
				}
			}

			// Check not contains
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q, but it did.\nContent: %s", notWant, content)
				}
			}
		})
	}
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func TestCompactEntityStateOmitsEmpty(t *testing.T) {
	entry := compactEntityState{
		EntityID:     "light.test",
		State:        "on",
		FriendlyName: "",
	}

	output, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result := string(output)
	if strings.Contains(result, "friendly_name") {
		t.Errorf("Expected empty friendly_name to be omitted, got: %s", result)
	}
	if !strings.Contains(result, "entity_id") {
		t.Errorf("Expected entity_id to be present, got: %s", result)
	}
	if !strings.Contains(result, "state") {
		t.Errorf("Expected state to be present, got: %s", result)
	}
}

func TestCompactEntityStateIncludesFriendlyName(t *testing.T) {
	entry := compactEntityState{
		EntityID:     "light.test",
		State:        "on",
		FriendlyName: "Test Light",
	}

	output, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "friendly_name") {
		t.Errorf("Expected friendly_name to be present, got: %s", result)
	}
	if !strings.Contains(result, "Test Light") {
		t.Errorf("Expected 'Test Light' value to be present, got: %s", result)
	}
}
