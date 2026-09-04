package formatter

import (
	"fmt"
	"strconv"
	"strings"
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

// TestFormatNameWithID covers the canonical "Name (entity_id)" join used across
// natural-format renderers, including a later consistency fix that aligned every
// renderer on this shape. name is user-controlled (HA friendly_name/title), so the
// sanitization cases matter as much as the happy path.
func TestFormatNameWithID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		entityID string
		expected string
	}{
		{
			name:     "distinct friendly name",
			input:    "Living Room Light",
			entityID: "light.living_room",
			expected: "Living Room Light (light.living_room)",
		},
		{
			name:     "name equals entity_id - no duplication",
			input:    "light.kitchen",
			entityID: "light.kitchen",
			expected: "light.kitchen",
		},
		{
			name:     "name containing parentheses is stripped so it cannot forge a fake id",
			input:    "Kitchen (light.hallway) is off",
			entityID: "light.kitchen",
			expected: "Kitchen light.hallway is off (light.kitchen)",
		},
		{
			name:     "name containing newline is collapsed to a single line",
			input:    "Kitchen\nlight.hallway is off",
			entityID: "light.kitchen",
			expected: "Kitchen light.hallway is off (light.kitchen)",
		},
		{
			name:     "name containing carriage return is collapsed to a single line",
			input:    "Kitchen\r\nAttack",
			entityID: "light.kitchen",
			expected: "Kitchen Attack (light.kitchen)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatNameWithID(tt.input, tt.entityID)
			if result != tt.expected {
				t.Errorf("FormatNameWithID(%q, %q) = %q, want %q", tt.input, tt.entityID, result, tt.expected)
			}
			if strings.Contains(result, "\n") || strings.Contains(result, "\r") {
				t.Errorf("FormatNameWithID(%q, %q) = %q, must not contain raw newlines", tt.input, tt.entityID, result)
			}
			// The real entity_id must be the only parenthesized token, so a caller can
			// always recover it positionally.
			if got := strings.Count(result, "("); got > 1 {
				t.Errorf("FormatNameWithID(%q, %q) = %q, expected at most one '(' (the real id), got %d", tt.input, tt.entityID, result, got)
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

// TestTitleCaseWords pins the rune-safe title-casing used to render helper
// domains ("alarm_control_panel" -> "Alarm Control Panel") and attribute
// keys, replacing byte-slicing that mangled multi-byte first runes.
func TestTitleCaseWords(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		sep      string
		expected string
	}{
		{
			name:     "domain with underscores",
			s:        "alarm_control_panel",
			sep:      "_",
			expected: "Alarm Control Panel",
		},
		{
			name:     "single word",
			s:        "climate",
			sep:      "_",
			expected: "Climate",
		},
		{
			name:     "empty string",
			s:        "",
			sep:      "_",
			expected: "",
		},
		{
			name:     "non-ASCII first rune does not mangle",
			s:        "ätzend_modus",
			sep:      "_",
			expected: "Ätzend Modus",
		},
		{
			name:     "leading separator produces empty segment",
			s:        "_leading",
			sep:      "_",
			expected: " Leading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := titleCaseWords(tt.s, tt.sep)
			if result != tt.expected {
				t.Errorf("titleCaseWords(%q, %q) = %q, want %q", tt.s, tt.sep, result, tt.expected)
			}
		})
	}
}

// TestSanitizeDisplayValue pins the newline-collapsing applied to natural-format
// attribute values (issue #216 remediation, finding W3): a value must not be able
// to forge additional "Key: value" lines, but unlike sanitizeDisplayName (used for
// entity names), parentheses are legitimate in attribute values and must survive.
func TestSanitizeDisplayValue(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		expected string
	}{
		{
			name:     "plain value unchanged",
			s:        "on",
			expected: "on",
		},
		{
			name:     "parentheses preserved",
			s:        "Auto (eco)",
			expected: "Auto (eco)",
		},
		{
			name:     "newline forging additional lines is collapsed",
			s:        "on\nState: off\nDevice class: safe",
			expected: "on State: off Device class: safe",
		},
		{
			name:     "carriage return collapsed",
			s:        "on\r\noff",
			expected: "on off",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeDisplayValue(tt.s)
			if result != tt.expected {
				t.Errorf("sanitizeDisplayValue(%q) = %q, want %q", tt.s, result, tt.expected)
			}
			if strings.Contains(result, "\n") || strings.Contains(result, "\r") {
				t.Errorf("sanitizeDisplayValue(%q) = %q, must not contain raw newlines", tt.s, result)
			}
		})
	}
}

// TestTruncateRunes pins rune-safe truncation (not byte-slicing, which can split
// a multi-byte rune and corrupt output).
func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxRunes int
		expected string
	}{
		{
			name:     "under limit unchanged",
			s:        "short",
			maxRunes: 10,
			expected: "short",
		},
		{
			name:     "exactly at limit unchanged",
			s:        "12345",
			maxRunes: 5,
			expected: "12345",
		},
		{
			name:     "over limit truncated with ellipsis",
			s:        "0123456789",
			maxRunes: 5,
			expected: "01234...",
		},
		{
			name:     "multi-byte runes not split",
			s:        strings.Repeat("ä", 10),
			maxRunes: 5,
			expected: strings.Repeat("ä", 5) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateRunes(tt.s, tt.maxRunes)
			if result != tt.expected {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.s, tt.maxRunes, result, tt.expected)
			}
		})
	}
}

// TestFormatDetailValue pins the generic attribute-value renderer added for
// issue #216's remediation (findings W1/W2/W3/N2): nil is omitted (empty
// string, callers check for this to skip the whole line), strings pass
// through sanitized+truncated, lists are comma-joined and length-capped,
// nested maps render as compact JSON at any depth, and the truncation cap
// applies once at the top level - not per list element - so the item cap
// remains reachable instead of every element being pre-truncated first.
func TestFormatDetailValue(t *testing.T) {
	longList := make([]any, 30)
	for i := range longList {
		longList[i] = i
	}

	tests := []struct {
		name     string
		v        any
		expected string
	}{
		{
			name:     "nil renders empty",
			v:        nil,
			expected: "",
		},
		{
			name:     "string passthrough",
			v:        "heat",
			expected: "heat",
		},
		{
			name:     "string with newline sanitized",
			v:        "on\nState: off",
			expected: "on State: off",
		},
		{
			name:     "list of strings comma-joined",
			v:        []any{"heat", "off"},
			expected: "heat, off",
		},
		{
			name:     "nested map renders as compact JSON",
			v:        map[string]any{"a": float64(1)},
			expected: `{"a":1}`,
		},
		{
			name:     "list containing a nested map renders that element as JSON, not Go syntax",
			v:        []any{map[string]any{"a": float64(1)}},
			expected: `{"a":1}`,
		},
		{
			name:     "long list capped by item count",
			v:        longList,
			expected: joinIntsWithMore(20, 10),
		},
		{
			name:     "long string truncated",
			v:        strings.Repeat("x", maxDetailValueChars+50),
			expected: strings.Repeat("x", maxDetailValueChars) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDetailValue(tt.v)
			if result != tt.expected {
				t.Errorf("formatDetailValue(%v) = %q, want %q", tt.v, result, tt.expected)
			}
		})
	}
}

// joinIntsWithMore builds the expected "0, 1, ..., n-1, … +more more" string
// for a 0..29 int list capped at `shown` items.
func joinIntsWithMore(shown, more int) string {
	var parts []string
	for i := 0; i < shown; i++ {
		parts = append(parts, strconv.Itoa(i))
	}
	return strings.Join(parts, ", ") + fmt.Sprintf(", … +%d more", more)
}

// TestFormatDetailValue_ListCapBeforeCharBudget guards the double-truncation
// trap: a long list of long strings must be capped by item count first, then
// the joined result truncated by character budget once - not each element
// truncated individually before joining, which would make the item cap
// effectively unreachable behind the character cap.
func TestFormatDetailValue_ListCapBeforeCharBudget(t *testing.T) {
	items := make([]any, 30)
	for i := range items {
		items[i] = strings.Repeat("x", 50)
	}

	result := formatDetailValue(items)

	if !strings.Contains(result, "+10 more") {
		t.Errorf("formatDetailValue(30 long items) = %q, want it to contain the item-count cap marker", result)
	}
}

// TestRenderDetailValue_DepthBoundary pins the exact maxDetailValueDepth
// cutoff: at depth == maxDetailValueDepth, recursion must stop and fall
// back to a raw %v rendering - even for a compound value that would
// otherwise recurse further and produce a different (comma-joined) string.
// A plain string leaf can't distinguish this boundary (%v of a string
// equals the string itself either way), so the probe value is itself a
// slice.
func TestRenderDetailValue_DepthBoundary(t *testing.T) {
	probe := []any{"a", "b"}

	atLimit := renderDetailValue(probe, maxDetailValueDepth)
	wantAtLimit := fmt.Sprintf("%v", probe)
	if atLimit != wantAtLimit {
		t.Errorf("renderDetailValue(_, maxDetailValueDepth) = %q, want %q (raw %%v fallback, no further recursion)", atLimit, wantAtLimit)
	}

	belowLimit := renderDetailValue(probe, maxDetailValueDepth-1)
	if belowLimit != "a, b" {
		t.Errorf("renderDetailValue(_, maxDetailValueDepth-1) = %q, want %q (normal recursion one level below the limit)", belowLimit, "a, b")
	}
}

// TestRenderDetailValue_NestedListMoreBoundary pins the "more > 0" check in
// renderDetailValue's own []any case (a list nested inside another list, at
// depth > 0 - distinct from formatDetailListValue's top-level list
// handling, which has its own equivalent check already covered by
// TestFormatDetailValue_ListCapBeforeCharBudget). At exactly
// maxDetailListItems items, no item is omitted and no "more" suffix must
// appear; one item over, exactly one must be reported omitted.
func TestRenderDetailValue_NestedListMoreBoundary(t *testing.T) {
	exact := make([]any, maxDetailListItems)
	for i := range exact {
		exact[i] = i
	}
	got := renderDetailValue(exact, 1)
	if strings.Contains(got, "more") {
		t.Errorf("renderDetailValue(exactly maxDetailListItems items) = %q, want no more-suffix", got)
	}

	over := make([]any, maxDetailListItems+1)
	for i := range over {
		over[i] = i
	}
	got = renderDetailValue(over, 1)
	if !strings.Contains(got, "+1 more") {
		t.Errorf("renderDetailValue(maxDetailListItems+1 items) = %q, want a +1 more suffix", got)
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
