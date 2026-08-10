package formatter

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestNaturalFormatter_FormatEntity(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	f := NewNaturalFormatter().WithNow(now)

	tests := []struct {
		name     string
		entity   homeassistant.Entity
		contains []string
	}{
		{
			name: "light on with brightness",
			entity: homeassistant.Entity{
				EntityID: "light.living_room",
				State:    "on",
				Attributes: map[string]any{
					"friendly_name": "Living Room Light",
					"brightness":    204, // 80%
				},
				LastChanged: now.Add(-2 * time.Hour),
			},
			contains: []string{"Living Room Light", "light.living_room", "is on", "80% brightness", "2 hours ago"},
		},
		{
			name: "light off",
			entity: homeassistant.Entity{
				EntityID: "light.bedroom",
				State:    "off",
				Attributes: map[string]any{
					"friendly_name": "Bedroom Light",
				},
				LastChanged: now.Add(-30 * time.Minute),
			},
			contains: []string{"Bedroom Light", "light.bedroom", "is off", "30 mins ago"},
		},
		{
			name: "climate heating",
			entity: homeassistant.Entity{
				EntityID: "climate.thermostat",
				State:    "heat",
				Attributes: map[string]any{
					"friendly_name":       "Thermostat",
					"current_temperature": 21.5,
					"temperature":         23.0,
				},
				LastChanged: now.Add(-1 * time.Hour),
			},
			contains: []string{"Thermostat", "climate.thermostat", "is heat", "21.5°", "target: 23.0°"},
		},
		{
			name: "sensor with unit",
			entity: homeassistant.Entity{
				EntityID: "sensor.temperature",
				State:    "22.5",
				Attributes: map[string]any{
					"friendly_name":       "Temperature Sensor",
					"unit_of_measurement": "°C",
				},
				LastChanged: now.Add(-5 * time.Minute),
			},
			contains: []string{"Temperature Sensor", "sensor.temperature", "is 22.5 °C"},
		},
		{
			name: "binary_sensor motion detected",
			entity: homeassistant.Entity{
				EntityID: "binary_sensor.motion",
				State:    "on",
				Attributes: map[string]any{
					"friendly_name": "Motion Sensor",
					"device_class":  "motion",
				},
				LastChanged: now.Add(-1 * time.Minute),
			},
			contains: []string{"Motion Sensor", "binary_sensor.motion", "detected motion", "1 min ago"},
		},
		{
			name: "binary_sensor door open",
			entity: homeassistant.Entity{
				EntityID: "binary_sensor.front_door",
				State:    "on",
				Attributes: map[string]any{
					"friendly_name": "Front Door",
					"device_class":  "door",
				},
				LastChanged: now.Add(-10 * time.Minute),
			},
			contains: []string{"Front Door", "binary_sensor.front_door", "is open"},
		},
		{
			name: "media player playing",
			entity: homeassistant.Entity{
				EntityID: "media_player.speaker",
				State:    "playing",
				Attributes: map[string]any{
					"friendly_name": "Living Room Speaker",
					"media_title":   "Bohemian Rhapsody",
					"media_artist":  "Queen",
				},
				LastChanged: now.Add(-15 * time.Minute),
			},
			contains: []string{"Living Room Speaker", "media_player.speaker", "is playing", "Bohemian Rhapsody", "by Queen"},
		},
		{
			name: "update available",
			entity: homeassistant.Entity{
				EntityID: "update.home_assistant_core",
				State:    "on",
				Attributes: map[string]any{
					"title":             "Home Assistant Core Update",
					"installed_version": "2024.1.0",
					"latest_version":    "2024.1.5",
				},
				LastChanged: now.Add(-1 * time.Hour),
			},
			contains: []string{"Home Assistant Core Update", "update.home_assistant_core", "Update available", "2024.1.0", "2024.1.5"},
		},
		{
			name: "update up to date",
			entity: homeassistant.Entity{
				EntityID: "update.hacs",
				State:    "off",
				Attributes: map[string]any{
					"title":             "HACS",
					"installed_version": "1.34.0",
					"latest_version":    "1.34.0",
				},
				LastChanged: now.Add(-2 * time.Hour),
			},
			contains: []string{"HACS", "update.hacs", "Up to date", "1.34.0"},
		},
		{
			name: "update in progress",
			entity: homeassistant.Entity{
				EntityID: "update.esphome",
				State:    "on",
				Attributes: map[string]any{
					"title":             "ESPHome",
					"installed_version": "2023.12.0",
					"latest_version":    "2024.1.0",
					"in_progress":       true,
				},
				LastChanged: now.Add(-5 * time.Minute),
			},
			contains: []string{"ESPHome", "update.esphome", "Installing update"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.FormatEntity(context.Background(), tt.entity)
			if err != nil {
				t.Fatalf("FormatEntity() error = %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("FormatEntity() = %q, want to contain %q", result, expected)
				}
			}
		})
	}
}

func TestNaturalFormatter_FormatEntity_CanonicalShape(t *testing.T) {
	t.Parallel()
	f := NewNaturalFormatter()

	entity := homeassistant.Entity{
		EntityID:   "light.living_room",
		State:      "on",
		Attributes: map[string]any{"friendly_name": "Living Room Light"},
	}

	result, err := f.FormatEntity(context.Background(), entity)
	if err != nil {
		t.Fatalf("FormatEntity() error = %v", err)
	}

	nameIdx := strings.Index(result, "Living Room Light")
	idIdx := strings.Index(result, "(light.living_room)")
	stateIdx := strings.Index(result, "is on")
	if nameIdx == -1 || idIdx == -1 || stateIdx == -1 {
		t.Fatalf("expected name, parenthesized id, and state all present, got: %q", result)
	}
	if !(nameIdx < idIdx && idIdx < stateIdx) {
		t.Errorf("expected order name < (id) < state, got positions %d, %d, %d in: %q", nameIdx, idIdx, stateIdx, result)
	}
}

func TestNaturalFormatter_FormatEntities_Empty(t *testing.T) {
	f := NewNaturalFormatter()
	result, err := f.FormatEntities(context.Background(), nil, EntityListOptions{})
	if err != nil {
		t.Fatalf("FormatEntities() error = %v", err)
	}
	if result != "No entities found." {
		t.Errorf("FormatEntities() = %q, want %q", result, "No entities found.")
	}
}

func TestNaturalFormatter_FormatEntities_Summary(t *testing.T) {
	f := NewNaturalFormatter()

	entities := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "light.bedroom", State: "off"},
		{EntityID: "sensor.temperature", State: "22.5"},
		{EntityID: "sensor.humidity", State: "unavailable"},
	}

	result, err := f.FormatEntities(context.Background(), entities, EntityListOptions{
		IncludeSummary: true,
	})
	if err != nil {
		t.Fatalf("FormatEntities() error = %v", err)
	}

	// Should contain total count
	if !strings.Contains(result, "4 entities total") {
		t.Errorf("FormatEntities() should contain total count, got %q", result)
	}

	// Should contain domain counts
	if !strings.Contains(result, "light: 2") {
		t.Errorf("FormatEntities() should contain light count, got %q", result)
	}

	// Should contain unavailable warning
	if !strings.Contains(result, "1 entities unavailable") {
		t.Errorf("FormatEntities() should contain unavailable warning, got %q", result)
	}
}

func TestNaturalFormatter_FormatEntities_GroupByDomain(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	f := NewNaturalFormatter().WithNow(now)

	entities := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on", Attributes: map[string]any{"friendly_name": "Living Room Light"}},
		{EntityID: "light.bedroom", State: "off", Attributes: map[string]any{"friendly_name": "Bedroom Light"}},
		{EntityID: "sensor.temperature", State: "22.5", Attributes: map[string]any{"friendly_name": "Temperature"}},
	}

	result, err := f.FormatEntities(context.Background(), entities, EntityListOptions{
		GroupByDomain: true,
		Verbose:       true,
	})
	if err != nil {
		t.Fatalf("FormatEntities() error = %v", err)
	}

	// Should contain domain headers
	if !strings.Contains(result, "**light**") {
		t.Errorf("FormatEntities() should contain light header, got %q", result)
	}
	if !strings.Contains(result, "(1 on, 1 off)") {
		t.Errorf("FormatEntities() should contain on/off count, got %q", result)
	}
}

func TestNaturalFormatter_FormatHistory_Empty_EntityExists(t *testing.T) {
	f := NewNaturalFormatter()
	result, err := f.FormatHistory(context.Background(), "light.test", nil, HistoryOptions{EntityExists: true})
	if err != nil {
		t.Fatalf("FormatHistory() error = %v", err)
	}

	expectedPhrases := []string{
		"No history found",
		"Possible reasons",
		"excluded from recorder",
		"newly created",
		"No state changes",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(result, phrase) {
			t.Errorf("FormatHistory() = %q, want to contain %q", result, phrase)
		}
	}

	// Should NOT contain entity not found message
	if strings.Contains(result, "was not found") {
		t.Errorf("FormatHistory() should not contain 'was not found' for existing entity, got %q", result)
	}
}

func TestNaturalFormatter_FormatHistory_Empty_EntityNotFound(t *testing.T) {
	f := NewNaturalFormatter()
	result, err := f.FormatHistory(context.Background(), "light.test", nil, HistoryOptions{EntityExists: false})
	if err != nil {
		t.Fatalf("FormatHistory() error = %v", err)
	}

	expectedPhrases := []string{
		"was not found",
		"verify",
		"entity_id",
		"spelled correctly",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(result, phrase) {
			t.Errorf("FormatHistory() = %q, want to contain %q", result, phrase)
		}
	}

	// Should NOT contain generic "no history" tips
	if strings.Contains(result, "excluded from recorder") {
		t.Errorf("FormatHistory() should not contain recorder tips for non-existent entity, got %q", result)
	}
}

func TestNaturalFormatter_FormatHistory_WithEntries(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	f := NewNaturalFormatter().WithNow(now)

	entries := []homeassistant.HistoryEntry{
		{
			EntityID:    "light.living_room",
			State:       "on",
			Attributes:  map[string]any{"friendly_name": "Living Room Light"},
			LastChanged: float64(now.Add(-1 * time.Hour).Unix()),
		},
		{
			EntityID:    "light.living_room",
			State:       "off",
			Attributes:  map[string]any{"friendly_name": "Living Room Light"},
			LastChanged: float64(now.Add(-2 * time.Hour).Unix()),
		},
	}

	result, err := f.FormatHistory(context.Background(), "light.living_room", entries, HistoryOptions{Limit: 10})
	if err != nil {
		t.Fatalf("FormatHistory() error = %v", err)
	}

	if !strings.Contains(result, "Living Room Light") {
		t.Errorf("FormatHistory() should contain friendly name, got %q", result)
	}
	if !strings.Contains(result, "2 state changes") {
		t.Errorf("FormatHistory() should contain state change count, got %q", result)
	}
	// Entries always shown with absolute timestamps
	if !strings.Contains(result, "→ on") {
		t.Errorf("FormatHistory() should contain '→ on', got %q", result)
	}
	if !strings.Contains(result, "2024-01-15 11:00") {
		t.Errorf("FormatHistory() should contain timestamp '2024-01-15 11:00', got %q", result)
	}
}

func TestNaturalFormatter_FormatHistory_VerboseShowsAttributes(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	f := NewNaturalFormatter().WithNow(now)

	entries := []homeassistant.HistoryEntry{
		{
			State: "on",
			Attributes: map[string]any{
				"friendly_name": "Kitchen Light",
				"brightness":    float64(200),
				"color_temp":    float64(350),
			},
			LastChanged: float64(now.Add(-30 * time.Minute).Unix()),
		},
	}

	result, err := f.FormatHistory(context.Background(), "light.kitchen", entries, HistoryOptions{Verbose: true, Limit: 10})
	if err != nil {
		t.Fatalf("FormatHistory() error = %v", err)
	}

	if !strings.Contains(result, "brightness=200") {
		t.Errorf("FormatHistory() verbose should contain 'brightness=200', got %q", result)
	}
	if !strings.Contains(result, "color_temp=350") {
		t.Errorf("FormatHistory() verbose should contain 'color_temp=350', got %q", result)
	}
	// friendly_name should be in header but not as attribute
	if !strings.Contains(result, "Kitchen Light") {
		t.Errorf("FormatHistory() should contain friendly name in header, got %q", result)
	}
}

func TestNaturalFormatter_FormatHistory_LimitTruncation(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	f := NewNaturalFormatter().WithNow(now)

	entries := make([]homeassistant.HistoryEntry, 5)
	for i := range 5 {
		entries[i] = homeassistant.HistoryEntry{
			State:       fmt.Sprintf("state_%d", i),
			LastChanged: float64(now.Add(-time.Duration(i) * time.Hour).Unix()),
		}
	}

	result, err := f.FormatHistory(context.Background(), "sensor.test", entries, HistoryOptions{Limit: 3})
	if err != nil {
		t.Fatalf("FormatHistory() error = %v", err)
	}

	if !strings.Contains(result, "…and 2 more") {
		t.Errorf("FormatHistory() should contain '…and 2 more', got %q", result)
	}
	if strings.Contains(result, "state_3") || strings.Contains(result, "state_4") {
		t.Errorf("FormatHistory() should not show entries beyond limit, got %q", result)
	}
}

func TestNaturalFormatter_FormatServiceSuccess(t *testing.T) {
	f := NewNaturalFormatter()

	tests := []struct {
		name     string
		domain   string
		service  string
		targets  []string
		data     map[string]any
		contains string
	}{
		{
			name:     "turn_on single target",
			domain:   "light",
			service:  "turn_on",
			targets:  []string{"light.living_room"},
			data:     nil,
			contains: "Turned on light.living_room",
		},
		{
			name:     "turn_off multiple targets",
			domain:   "light",
			service:  "turn_off",
			targets:  []string{"light.living_room", "light.bedroom"},
			data:     nil,
			contains: "Turned off 2 entities",
		},
		{
			name:     "no targets",
			domain:   "homeassistant",
			service:  "reload",
			targets:  nil,
			data:     nil,
			contains: "Reloaded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.FormatServiceSuccess(context.Background(), tt.domain, tt.service, tt.targets, tt.data)
			if err != nil {
				t.Fatalf("FormatServiceSuccess() error = %v", err)
			}
			if !strings.Contains(result, tt.contains) {
				t.Errorf("FormatServiceSuccess() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

func TestNaturalFormatter_FormatError(t *testing.T) {
	f := NewNaturalFormatter()
	err := &testError{msg: "connection refused"}
	result := f.FormatError(context.Background(), err)

	if !strings.Contains(result, "Error:") {
		t.Errorf("FormatError() = %q, want to contain 'Error:'", result)
	}
	if !strings.Contains(result, "connection refused") {
		t.Errorf("FormatError() = %q, want to contain error message", result)
	}
}

// testError is a simple error for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// TestNaturalFormatter_BinarySensorDeviceClasses verifies device-class-specific
// formatting for binary_sensor entities that were not yet covered.
func TestNaturalFormatter_BinarySensorDeviceClasses(t *testing.T) {
	t.Parallel()

	f := NewNaturalFormatter()

	tests := []struct {
		name        string
		deviceClass string
		state       string
		wantContain string
	}{
		{
			name:        "window open",
			deviceClass: "window",
			state:       "on",
			wantContain: "is open",
		},
		{
			name:        "window closed",
			deviceClass: "window",
			state:       "off",
			wantContain: "is closed",
		},
		{
			name:        "occupancy occupied",
			deviceClass: "occupancy",
			state:       "on",
			wantContain: "is occupied",
		},
		{
			name:        "occupancy not occupied",
			deviceClass: "occupancy",
			state:       "off",
			wantContain: "is not occupied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entity := homeassistant.Entity{
				EntityID: "binary_sensor.test",
				State:    tt.state,
				Attributes: map[string]any{
					"friendly_name": "Test Sensor",
					"device_class":  tt.deviceClass,
				},
			}

			result, err := f.FormatEntity(context.Background(), entity)
			if err != nil {
				t.Fatalf("FormatEntity() error = %v", err)
			}
			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("FormatEntity() = %q, want to contain %q", result, tt.wantContain)
			}
		})
	}
}

// TestNaturalFormatter_ClimateOff verifies climate entity formatting when state is "off".
func TestNaturalFormatter_ClimateOff(t *testing.T) {
	t.Parallel()

	f := NewNaturalFormatter()

	entity := homeassistant.Entity{
		EntityID: "climate.thermostat",
		State:    "off",
		Attributes: map[string]any{
			"friendly_name": "Thermostat",
		},
	}

	result, err := f.FormatEntity(context.Background(), entity)
	if err != nil {
		t.Fatalf("FormatEntity() error = %v", err)
	}
	if !strings.Contains(result, "is off") {
		t.Errorf("FormatEntity() = %q, want to contain %q", result, "is off")
	}
}

func TestNaturalFormatter_FormatEntities_CompactList(t *testing.T) {
	f := NewNaturalFormatter()

	entities := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on", Attributes: map[string]any{"friendly_name": "Living Room"}},
		{EntityID: "light.bedroom", State: "off", Attributes: map[string]any{"friendly_name": "Bedroom"}},
	}

	result, err := f.FormatEntities(context.Background(), entities, EntityListOptions{
		CompactList: true,
	})
	if err != nil {
		t.Fatalf("FormatEntities(CompactList=true) error = %v", err)
	}

	// Should include entity_ids
	if !strings.Contains(result, "light.living_room") {
		t.Errorf("FormatEntities(CompactList=true) should contain entity_id, got %q", result)
	}
	if !strings.Contains(result, "light.bedroom") {
		t.Errorf("FormatEntities(CompactList=true) should contain entity_id, got %q", result)
	}

	// Should include friendly names
	if !strings.Contains(result, "Living Room") {
		t.Errorf("FormatEntities(CompactList=true) should contain friendly name, got %q", result)
	}

	// Should still include summary
	if !strings.Contains(result, "2 entities") {
		t.Errorf("FormatEntities(CompactList=true) should contain entity count, got %q", result)
	}

	// After retiring formatEntityCompact, compact lines use the same domain-aware
	// phrasing as verbose/domain-grouped output, not a bare "— state" suffix.
	if !strings.Contains(result, "is on") {
		t.Errorf("FormatEntities(CompactList=true) should use domain-aware phrasing ('is on'), got %q", result)
	}
}

func TestNaturalFormatter_FormatEntities_CompactListCap(t *testing.T) {
	f := NewNaturalFormatter()

	// Build more than compactListCap entities
	entities := make([]homeassistant.Entity, compactListCap+5)
	for i := range entities {
		entities[i] = homeassistant.Entity{
			EntityID:   fmt.Sprintf("light.room_%03d", i),
			State:      "on",
			Attributes: map[string]any{"friendly_name": fmt.Sprintf("Room %d", i)},
		}
	}

	result, err := f.FormatEntities(context.Background(), entities, EntityListOptions{
		CompactList: true,
	})
	if err != nil {
		t.Fatalf("FormatEntities(CompactList=true, over cap) error = %v", err)
	}

	// Should include an overflow note mentioning "more"
	if !strings.Contains(result, "more") {
		t.Errorf("FormatEntities(CompactList=true, over cap) should contain overflow note, got %q", result)
	}

	// Should NOT list all entities (room_055 would be above cap)
	if strings.Contains(result, "light.room_055") {
		t.Errorf("FormatEntities(CompactList=true, over cap) should not list entities beyond cap, got %q", result)
	}
}

func TestNaturalFormatter_FormatEntities_CompactListNotVerbose(t *testing.T) {
	// CompactList=false and Verbose=false should NOT list entity_ids (existing behavior)
	f := NewNaturalFormatter()

	entities := []homeassistant.Entity{
		{EntityID: "light.living_room", State: "on", Attributes: map[string]any{"friendly_name": "Living Room"}},
		{EntityID: "light.bedroom", State: "off", Attributes: map[string]any{"friendly_name": "Bedroom"}},
	}

	result, err := f.FormatEntities(context.Background(), entities, EntityListOptions{
		IncludeSummary: true,
	})
	if err != nil {
		t.Fatalf("FormatEntities() error = %v", err)
	}

	// Summary-only should NOT include entity_ids (backward compat for non-query_entities callers)
	if strings.Contains(result, "light.living_room") {
		t.Errorf("FormatEntities(default/summary-only) should NOT contain entity_ids, got %q", result)
	}
}
