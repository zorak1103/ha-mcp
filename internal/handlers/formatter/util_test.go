package formatter

import (
	"testing"
	"time"
)

func TestFormatTimeSince(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		t        time.Time
		expected string
	}{
		{
			name:     "just now (under 1 minute)",
			t:        now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "1 minute ago",
			t:        now.Add(-1 * time.Minute),
			expected: "1 min ago",
		},
		{
			name:     "5 minutes ago",
			t:        now.Add(-5 * time.Minute),
			expected: "5 mins ago",
		},
		{
			name:     "59 minutes ago",
			t:        now.Add(-59 * time.Minute),
			expected: "59 mins ago",
		},
		{
			name:     "1 hour ago",
			t:        now.Add(-1 * time.Hour),
			expected: "1 hour ago",
		},
		{
			name:     "2 hours ago",
			t:        now.Add(-2 * time.Hour),
			expected: "2 hours ago",
		},
		{
			name:     "23 hours ago",
			t:        now.Add(-23 * time.Hour),
			expected: "23 hours ago",
		},
		{
			name:     "1 day ago",
			t:        now.Add(-24 * time.Hour),
			expected: "1 day ago",
		},
		{
			name:     "2 days ago",
			t:        now.Add(-48 * time.Hour),
			expected: "2 days ago",
		},
		{
			name:     "7 days ago",
			t:        now.Add(-7 * 24 * time.Hour),
			expected: "7 days ago",
		},
		{
			name:     "zero time",
			t:        time.Time{},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimeSince(tt.t, now)
			if result != tt.expected {
				t.Errorf("FormatTimeSince() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		entityID string
		expected string
	}{
		{
			name:     "light domain",
			entityID: "light.living_room",
			expected: "light",
		},
		{
			name:     "sensor domain",
			entityID: "sensor.temperature",
			expected: "sensor",
		},
		{
			name:     "binary_sensor domain",
			entityID: "binary_sensor.motion",
			expected: "binary_sensor",
		},
		{
			name:     "climate domain",
			entityID: "climate.thermostat",
			expected: "climate",
		},
		{
			name:     "no domain separator",
			entityID: "invalid",
			expected: "unknown",
		},
		{
			name:     "empty string",
			entityID: "",
			expected: "unknown",
		},
		{
			name:     "multiple dots",
			entityID: "sensor.outdoor.temperature",
			expected: "sensor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractDomain(tt.entityID)
			if result != tt.expected {
				t.Errorf("ExtractDomain(%q) = %q, want %q", tt.entityID, result, tt.expected)
			}
		})
	}
}

func TestGetFriendlyName(t *testing.T) {
	tests := []struct {
		name       string
		entityID   string
		attributes map[string]any
		expected   string
	}{
		{
			name:       "has friendly_name",
			entityID:   "light.living_room",
			attributes: map[string]any{"friendly_name": "Living Room Light"},
			expected:   "Living Room Light",
		},
		{
			name:       "no friendly_name uses entity_id",
			entityID:   "light.living_room",
			attributes: map[string]any{},
			expected:   "light.living_room",
		},
		{
			name:       "nil attributes uses entity_id",
			entityID:   "light.living_room",
			attributes: nil,
			expected:   "light.living_room",
		},
		{
			name:       "friendly_name not string uses entity_id",
			entityID:   "light.living_room",
			attributes: map[string]any{"friendly_name": 123},
			expected:   "light.living_room",
		},
		{
			name:       "empty friendly_name uses entity_id",
			entityID:   "light.living_room",
			attributes: map[string]any{"friendly_name": ""},
			expected:   "light.living_room",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFriendlyName(tt.entityID, tt.attributes)
			if result != tt.expected {
				t.Errorf("GetFriendlyName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestColorTempToDescription(t *testing.T) {
	tests := []struct {
		name     string
		kelvin   int
		expected string
	}{
		{
			name:     "warm white (2700K)",
			kelvin:   2700,
			expected: "warm white",
		},
		{
			name:     "warm white boundary (3000K)",
			kelvin:   3000,
			expected: "warm white",
		},
		{
			name:     "neutral (4000K)",
			kelvin:   4000,
			expected: "neutral",
		},
		{
			name:     "neutral boundary (5000K)",
			kelvin:   5000,
			expected: "neutral",
		},
		{
			name:     "cool white (6000K)",
			kelvin:   6000,
			expected: "cool white",
		},
		{
			name:     "daylight (6500K)",
			kelvin:   6500,
			expected: "daylight",
		},
		{
			name:     "daylight (7000K)",
			kelvin:   7000,
			expected: "daylight",
		},
		{
			name:     "zero kelvin",
			kelvin:   0,
			expected: "",
		},
		{
			name:     "negative kelvin",
			kelvin:   -100,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorTempToDescription(tt.kelvin)
			if result != tt.expected {
				t.Errorf("ColorTempToDescription(%d) = %q, want %q", tt.kelvin, result, tt.expected)
			}
		})
	}
}

func TestBrightnessToPercent(t *testing.T) {
	tests := []struct {
		name       string
		brightness int
		expected   int
	}{
		{
			name:       "full brightness",
			brightness: 255,
			expected:   100,
		},
		{
			name:       "half brightness",
			brightness: 128,
			expected:   50,
		},
		{
			name:       "off",
			brightness: 0,
			expected:   0,
		},
		{
			name:       "quarter brightness",
			brightness: 64,
			expected:   25,
		},
		{
			name:       "over max",
			brightness: 300,
			expected:   100,
		},
		{
			name:       "negative",
			brightness: -10,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BrightnessToPercent(tt.brightness)
			if result != tt.expected {
				t.Errorf("BrightnessToPercent(%d) = %d, want %d", tt.brightness, result, tt.expected)
			}
		})
	}
}

func TestGetStringAttr(t *testing.T) {
	tests := []struct {
		name     string
		attrs    map[string]any
		key      string
		expected string
	}{
		{
			name:     "string value",
			attrs:    map[string]any{"key": "value"},
			key:      "key",
			expected: "value",
		},
		{
			name:     "missing key",
			attrs:    map[string]any{"other": "value"},
			key:      "key",
			expected: "",
		},
		{
			name:     "nil map",
			attrs:    nil,
			key:      "key",
			expected: "",
		},
		{
			name:     "non-string value",
			attrs:    map[string]any{"key": 123},
			key:      "key",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStringAttr(tt.attrs, tt.key)
			if result != tt.expected {
				t.Errorf("GetStringAttr() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetIntAttr(t *testing.T) {
	tests := []struct {
		name     string
		attrs    map[string]any
		key      string
		expected int
	}{
		{
			name:     "int value",
			attrs:    map[string]any{"key": 42},
			key:      "key",
			expected: 42,
		},
		{
			name:     "float64 value (from JSON)",
			attrs:    map[string]any{"key": float64(42)},
			key:      "key",
			expected: 42,
		},
		{
			name:     "missing key",
			attrs:    map[string]any{"other": 42},
			key:      "key",
			expected: 0,
		},
		{
			name:     "nil map",
			attrs:    nil,
			key:      "key",
			expected: 0,
		},
		{
			name:     "string value",
			attrs:    map[string]any{"key": "42"},
			key:      "key",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetIntAttr(tt.attrs, tt.key)
			if result != tt.expected {
				t.Errorf("GetIntAttr() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestGetFloatAttr(t *testing.T) {
	tests := []struct {
		name     string
		attrs    map[string]any
		key      string
		expected float64
	}{
		{
			name:     "float64 value",
			attrs:    map[string]any{"key": 3.14},
			key:      "key",
			expected: 3.14,
		},
		{
			name:     "int value",
			attrs:    map[string]any{"key": 42},
			key:      "key",
			expected: 42.0,
		},
		{
			name:     "missing key",
			attrs:    map[string]any{"other": 3.14},
			key:      "key",
			expected: 0,
		},
		{
			name:     "nil map",
			attrs:    nil,
			key:      "key",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFloatAttr(tt.attrs, tt.key)
			if result != tt.expected {
				t.Errorf("GetFloatAttr() = %f, want %f", result, tt.expected)
			}
		})
	}
}
