package formatter

import (
	"context"
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
			contains: []string{"Living Room Light", "is on", "80% brightness", "2 hours ago"},
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
			contains: []string{"Bedroom Light", "is off", "30 mins ago"},
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
			contains: []string{"Thermostat", "is heat", "21.5°", "target: 23.0°"},
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
			contains: []string{"Temperature Sensor", "is 22.5 °C"},
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
			contains: []string{"Motion Sensor", "detected motion", "1 min ago"},
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
			contains: []string{"Front Door", "is open"},
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
			contains: []string{"Living Room Speaker", "is playing", "Bohemian Rhapsody", "by Queen"},
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

func TestNaturalFormatter_FormatHistory_Empty(t *testing.T) {
	f := NewNaturalFormatter()
	result, err := f.FormatHistory(context.Background(), "light.test", nil, HistoryOptions{})
	if err != nil {
		t.Fatalf("FormatHistory() error = %v", err)
	}
	if !strings.Contains(result, "No history found") {
		t.Errorf("FormatHistory() = %q, want to contain 'No history found'", result)
	}
}

func TestNaturalFormatter_FormatHistory_WithEntries(t *testing.T) {
	// Create a time that will give us a known "X hours ago" result
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	f := NewNaturalFormatter().WithNow(now)

	// HistoryEntry uses float64 Unix timestamps
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

	result, err := f.FormatHistory(context.Background(), "light.living_room", entries, HistoryOptions{
		Verbose: true,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("FormatHistory() error = %v", err)
	}

	if !strings.Contains(result, "Living Room Light") {
		t.Errorf("FormatHistory() should contain friendly name, got %q", result)
	}
	if !strings.Contains(result, "2 state changes") {
		t.Errorf("FormatHistory() should contain state change count, got %q", result)
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
