package formatter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// =============================================================================
// Test Data Helpers
// =============================================================================

func testHelperEntity(entityID, state string, attrs map[string]any) homeassistant.Entity {
	if attrs == nil {
		attrs = make(map[string]any)
	}
	return homeassistant.Entity{
		EntityID:    entityID,
		State:       state,
		Attributes:  attrs,
		LastChanged: time.Now().Add(-1 * time.Hour),
		LastUpdated: time.Now().Add(-30 * time.Minute),
	}
}

func createTestHelpers() []homeassistant.Entity {
	return []homeassistant.Entity{
		testHelperEntity("counter.visitors", "42", map[string]any{
			"friendly_name": "Visitor Counter",
			"step":          float64(1),
			"minimum":       float64(0),
			"maximum":       float64(1000),
		}),
		testHelperEntity("counter.button_presses", "156", map[string]any{
			"friendly_name": "Button Press Counter",
		}),
		testHelperEntity("input_boolean.vacation_mode", "off", map[string]any{
			"friendly_name": "Vacation Mode",
			"icon":          "mdi:beach",
		}),
		testHelperEntity("input_boolean.guest_mode", "on", map[string]any{
			"friendly_name": "Guest Mode",
		}),
		testHelperEntity("input_number.temperature", "22.5", map[string]any{
			"friendly_name":       "Temperature Target",
			"unit_of_measurement": "°C",
			"min":                 float64(15),
			"max":                 float64(30),
		}),
		testHelperEntity("timer.pomodoro", "idle", map[string]any{
			"friendly_name": "Pomodoro Timer",
			"duration":      "00:25:00",
		}),
		testHelperEntity("timer.laundry", "active", map[string]any{
			"friendly_name": "Laundry Timer",
			"duration":      "01:00:00",
			"remaining":     "00:15:32",
		}),
		testHelperEntity("input_select.house_mode", "Home", map[string]any{
			"friendly_name": "House Mode",
			"options":       []any{"Home", "Away", "Night", "Vacation"},
		}),
		testHelperEntity("schedule.work_hours", "on", map[string]any{
			"friendly_name": "Work Hours",
			"next_event":    "2024-01-15T06:00:00",
		}),
	}
}

// =============================================================================
// NewHelperFormatter Tests
// =============================================================================

func TestNewHelperFormatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		format      Format
		wantNatural bool
	}{
		{"natural format", FormatNatural, true},
		{"json format", FormatJSON, false},
		{"empty format defaults to natural", "", true},
		{"unknown format defaults to natural", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			formatter := NewHelperFormatter(tt.format)
			if formatter == nil {
				t.Fatal("NewHelperFormatter returned nil")
			}
			_, isNatural := formatter.(*NaturalHelperFormatter)
			if isNatural != tt.wantNatural {
				t.Errorf("NewHelperFormatter(%q) isNatural = %v, want %v",
					tt.format, isNatural, tt.wantNatural)
			}
		})
	}
}

// =============================================================================
// NaturalHelperFormatter - FormatList Tests
// =============================================================================

func TestNaturalHelperFormatter_FormatList_Empty(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	result, err := formatter.FormatList(context.Background(), nil, HelperListOptions{})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}
	if result != MsgNoHelpersFound {
		t.Errorf("FormatList(nil) = %q, want %q", result, MsgNoHelpersFound)
	}
}

func TestNaturalHelperFormatter_FormatList_EmptySlice(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	result, err := formatter.FormatList(context.Background(), []homeassistant.Entity{}, HelperListOptions{})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}
	if result != MsgNoHelpersFound {
		t.Errorf("FormatList([]) = %q, want %q", result, MsgNoHelpersFound)
	}
}

func TestNaturalHelperFormatter_FormatList_Summary(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := createTestHelpers()
	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Check summary line
	if !strings.Contains(result, "9 helpers") {
		t.Errorf("FormatList() missing count summary, got:\n%s", result)
	}

	// Check type breakdown
	if !strings.Contains(result, "types") {
		t.Errorf("FormatList() missing type breakdown, got:\n%s", result)
	}
}

func TestNaturalHelperFormatter_FormatList_GroupByType(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := createTestHelpers()
	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{GroupByType: true})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Check type sections are present (using formatted names)
	expectedSections := []string{"Counters", "Input Booleans", "Input Numbers", "Timers", "Input Selects", "Schedules"}
	for _, section := range expectedSections {
		if !strings.Contains(result, section) {
			t.Errorf("FormatList(GroupByType=true) missing section %q, got:\n%s", section, result)
		}
	}
}

func TestNaturalHelperFormatter_FormatList_HelperStates(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := createTestHelpers()
	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{GroupByType: true})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Check helper names appear
	expectedNames := []string{"Visitor Counter", "Vacation Mode", "Pomodoro Timer"}
	for _, name := range expectedNames {
		if !strings.Contains(result, name) {
			t.Errorf("FormatList() missing helper name %q, got:\n%s", name, result)
		}
	}

	// Check entity_ids appear alongside names, so entities with colliding
	// friendly names remain distinguishable
	expectedIDs := []string{"counter.visitors", "input_boolean.vacation_mode", "timer.pomodoro"}
	for _, id := range expectedIDs {
		if !strings.Contains(result, id) {
			t.Errorf("FormatList() missing entity_id %q, got:\n%s", id, result)
		}
	}
}

// TestNaturalHelperFormatter_FormatList_NoFriendlyName verifies that a helper with no
// friendly_name renders its full entity_id once, rather than duplicating it as both a
// truncated object_id and the full id in parentheses.
func TestNaturalHelperFormatter_FormatList_NoFriendlyName(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()
	helpers := []homeassistant.Entity{
		testHelperEntity("input_boolean.vacation_mode", "off", map[string]any{}),
	}

	result, err := f.FormatList(context.Background(), helpers, HelperListOptions{})
	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	if strings.Contains(result, "(input_boolean.vacation_mode)") {
		t.Errorf("FormatList() = %q, expected no parenthesized duplication when there is no friendly name", result)
	}
	if count := strings.Count(result, "input_boolean.vacation_mode"); count != 1 {
		t.Errorf("FormatList() = %q, expected entity_id to appear exactly once, got %d", result, count)
	}
}

func TestNaturalHelperFormatter_FormatList_Verbose(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := createTestHelpers()

	normalResult, _ := formatter.FormatList(context.Background(), helpers, HelperListOptions{})
	verboseResult, _ := formatter.FormatList(context.Background(), helpers, HelperListOptions{Verbose: true})

	// Verbose should have more detail (longer output)
	if len(verboseResult) <= len(normalResult) {
		t.Errorf("FormatList(Verbose=true) should produce more output than normal")
	}
}

func TestNaturalHelperFormatter_FormatList_TimerStates(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := []homeassistant.Entity{
		testHelperEntity("timer.active_timer", "active", map[string]any{
			"friendly_name": "Active Timer",
			"remaining":     "00:10:00",
		}),
		testHelperEntity("timer.idle_timer", "idle", map[string]any{
			"friendly_name": "Idle Timer",
			"duration":      "00:30:00",
		}),
		testHelperEntity("timer.paused_timer", "paused", map[string]any{
			"friendly_name": "Paused Timer",
			"remaining":     "00:05:00",
		}),
	}

	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{GroupByType: true})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Active timers should show remaining time
	if !strings.Contains(result, "Active Timer") {
		t.Errorf("FormatList() missing Active Timer, got:\n%s", result)
	}
}

func TestNaturalHelperFormatter_FormatList_CounterValues(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := []homeassistant.Entity{
		testHelperEntity("counter.test", "42", map[string]any{
			"friendly_name": "Test Counter",
		}),
	}

	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{GroupByType: true})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Counter should show its value
	if !strings.Contains(result, "42") {
		t.Errorf("FormatList() should show counter value, got:\n%s", result)
	}
}

func TestNaturalHelperFormatter_FormatList_InputBooleanStates(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := []homeassistant.Entity{
		testHelperEntity("input_boolean.on_switch", "on", map[string]any{
			"friendly_name": "On Switch",
		}),
		testHelperEntity("input_boolean.off_switch", "off", map[string]any{
			"friendly_name": "Off Switch",
		}),
	}

	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{GroupByType: true})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Should indicate on/off states
	if !strings.Contains(strings.ToLower(result), "on") || !strings.Contains(strings.ToLower(result), "off") {
		t.Errorf("FormatList() should show input_boolean states, got:\n%s", result)
	}
}

// =============================================================================
// NaturalHelperFormatter - FormatScheduleDetail Tests
// =============================================================================

func TestNaturalHelperFormatter_FormatScheduleDetail(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	detail := map[string]any{
		"entity_id":     "schedule.work_hours",
		"state":         "on",
		"friendly_name": "Work Hours",
		"next_event":    "Mon 06:00",
		"schedule": map[string][]map[string]string{
			"monday":    {{"from": "06:00", "to": "08:00"}, {"from": "18:00", "to": "22:00"}},
			"tuesday":   {{"from": "06:00", "to": "08:00"}},
			"wednesday": {{"from": "06:00", "to": "08:00"}},
			"thursday":  {{"from": "06:00", "to": "08:00"}},
			"friday":    {{"from": "06:00", "to": "08:00"}},
		},
	}

	result, err := formatter.FormatScheduleDetail(context.Background(), detail)

	if err != nil {
		t.Fatalf("FormatScheduleDetail() error = %v", err)
	}

	// Check schedule name
	if !strings.Contains(result, "Work Hours") {
		t.Errorf("FormatScheduleDetail() missing schedule name, got:\n%s", result)
	}

	// Check state indicator
	if !strings.Contains(result, "on") {
		t.Errorf("FormatScheduleDetail() missing state, got:\n%s", result)
	}
}

func TestNaturalHelperFormatter_FormatScheduleDetail_Empty(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	detail := map[string]any{
		"entity_id":     "schedule.empty",
		"state":         "off",
		"friendly_name": "Empty Schedule",
	}

	result, err := formatter.FormatScheduleDetail(context.Background(), detail)

	if err != nil {
		t.Fatalf("FormatScheduleDetail() error = %v", err)
	}

	if !strings.Contains(result, "Empty Schedule") {
		t.Errorf("FormatScheduleDetail() missing name, got:\n%s", result)
	}
}

// TestNaturalHelperFormatter_DetailRenderers_IncludeEntityID pins the canonical
// "Name (entity_id)" shape across every get_details renderer in this file. Before this
// fix, Schedule/Toggle(input_boolean)/Group used "(state)" for the parenthesized field
// instead of the id, and Counter/Timer/Sensor/Binary Sensor omitted the entity_id
// entirely — disagreeing with manage_helper's `list` action, which already carries the
// id, within the same tool.
func TestNaturalHelperFormatter_DetailRenderers_IncludeEntityID(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()

	tests := []struct {
		name       string
		helperType string
		detail     map[string]any
		wantLine   string
	}{
		{
			name:       "input_boolean (Toggle)",
			helperType: "input_boolean",
			detail: map[string]any{
				"entity_id":     "input_boolean.vacation_mode",
				"friendly_name": "Vacation Mode",
				"state":         "on",
				"editable":      true,
			},
			wantLine: "Toggle: Vacation Mode (input_boolean.vacation_mode) is on",
		},
		{
			name:       "group",
			helperType: "group",
			detail: map[string]any{
				"entity_id":     "group.downstairs",
				"friendly_name": "Downstairs",
				"state":         "on",
			},
			wantLine: "Group: Downstairs (group.downstairs) is on",
		},
		{
			name:       "sensor",
			helperType: "sensor",
			detail: map[string]any{
				"entity_id":     "sensor.outdoor_temp",
				"friendly_name": "Outdoor Temp",
				"state":         "21.5",
			},
			wantLine: "Sensor: Outdoor Temp (sensor.outdoor_temp)",
		},
		{
			name:       "binary_sensor",
			helperType: "binary_sensor",
			detail: map[string]any{
				"entity_id":     "binary_sensor.door",
				"friendly_name": "Front Door",
				"state":         "off",
			},
			wantLine: "Binary Sensor: Front Door (binary_sensor.door)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.FormatHelperDetail(context.Background(), tt.helperType, tt.detail)
			if err != nil {
				t.Fatalf("FormatHelperDetail(%q) error = %v", tt.helperType, err)
			}
			if !strings.Contains(result, tt.wantLine) {
				t.Errorf("FormatHelperDetail(%q) = %q, want it to contain %q", tt.helperType, result, tt.wantLine)
			}
		})
	}
}

// TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback pins the
// fallback renderer added for issue #216: domains with no dedicated case
// (climate, humidifier, select, and the 15 template_* subtypes) must render
// natural-format output instead of erroring with "unsupported helper type".
func TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()

	detail := map[string]any{
		"entity_id":           "climate.thermostat",
		"friendly_name":       "Wohnzimmer",
		"state":               "heat",
		"current_temperature": 21.5,
		"temperature":         22.0,
		"hvac_modes":          []any{"heat", "off"},
		"min_temp":            float64(7),
		"max_temp":            float64(35),
	}

	result, err := f.FormatHelperDetail(context.Background(), "climate", detail)
	if err != nil {
		t.Fatalf("FormatHelperDetail(climate) error = %v, want nil", err)
	}

	wantLines := []string{
		"Climate: Wohnzimmer (climate.thermostat)",
		"State: heat",
		"Current temperature: 21.5",
		"Hvac modes: heat, off",
		"Max temp: 35",
		"Min temp: 7",
		"Temperature: 22",
	}
	for _, line := range wantLines {
		if !strings.Contains(result, line) {
			t.Errorf("FormatHelperDetail(climate) = %q, want it to contain %q", result, line)
		}
	}
}

// TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_KeyOrder pins
// that the fallback renderer sorts attribute keys - Go randomizes map
// iteration order per range statement, so an unsorted implementation would
// produce a different attribute order on a later call within the same run.
func TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_KeyOrder(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()

	detail := map[string]any{
		"entity_id":     "select.mode",
		"friendly_name": "Mode",
		"state":         "eco",
		"zeta":          "1",
		"alpha":         "2",
		"mike":          "3",
	}

	first, err := f.FormatHelperDetail(context.Background(), "select", detail)
	if err != nil {
		t.Fatalf("FormatHelperDetail(select) error = %v, want nil", err)
	}

	for i := 0; i < 20; i++ {
		again, err := f.FormatHelperDetail(context.Background(), "select", detail)
		if err != nil {
			t.Fatalf("FormatHelperDetail(select) error = %v, want nil", err)
		}
		if again != first {
			t.Fatalf("FormatHelperDetail(select) not deterministic across calls:\ncall 1: %q\ncall %d: %q", first, i+2, again)
		}
	}

	alphaIdx := strings.Index(first, "Alpha:")
	mikeIdx := strings.Index(first, "Mike:")
	zetaIdx := strings.Index(first, "Zeta:")
	if alphaIdx == -1 || mikeIdx == -1 || zetaIdx == -1 {
		t.Fatalf("FormatHelperDetail(select) missing expected keys, got: %q", first)
	}
	if alphaIdx >= mikeIdx || mikeIdx >= zetaIdx {
		t.Errorf("FormatHelperDetail(select) attributes not sorted alphabetically, got: %q", first)
	}
}

// TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_OmitsNilAttribute
// pins finding W1 of the issue #216 remediation: HA reports many optional
// attributes as JSON null (e.g. climate.hvac_action when unsupported). The
// generic fallback must omit that line entirely rather than rendering the
// literal "<nil>", while a genuinely empty string attribute is still shown -
// nil and "" are distinct states, not both "absent".
func TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_OmitsNilAttribute(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()

	detail := map[string]any{
		"entity_id":     "climate.thermostat",
		"friendly_name": "Wohnzimmer",
		"state":         "heat",
		"hvac_action":   nil,
		"note":          "",
	}

	result, err := f.FormatHelperDetail(context.Background(), "climate", detail)
	if err != nil {
		t.Fatalf("FormatHelperDetail(climate) error = %v, want nil", err)
	}

	if strings.Contains(result, "<nil>") {
		t.Errorf("FormatHelperDetail(climate) = %q, must not render a nil attribute as <nil>", result)
	}
	if strings.Contains(result, "Hvac action") {
		t.Errorf("FormatHelperDetail(climate) = %q, must omit the nil hvac_action line entirely", result)
	}
	if !strings.Contains(result, "Note:") {
		t.Errorf("FormatHelperDetail(climate) = %q, must still render an empty-string attribute (distinct from nil)", result)
	}
}

// TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_SanitizesValues
// pins finding W3: an attribute value is user/integration-controlled data
// (e.g. a template helper's rendered state, or a third-party integration's
// text attribute) and must not be able to forge additional "Key: value"
// lines via an embedded newline - the same hazard FormatNameWithID already
// guards against for entity names.
func TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_SanitizesValues(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()

	detail := map[string]any{
		"entity_id":     "select.mode",
		"friendly_name": "Mode",
		"state":         "eco\nDevice class: safe",
		"note":          "line one\nline two",
	}

	result, err := f.FormatHelperDetail(context.Background(), "select", detail)
	if err != nil {
		t.Fatalf("FormatHelperDetail(select) error = %v, want nil", err)
	}

	if strings.Contains(result, "State: eco\n") {
		t.Errorf("FormatHelperDetail(select) = %q, State line must not contain a raw newline", result)
	}
	if !strings.Contains(result, "line one line two") {
		t.Errorf("FormatHelperDetail(select) = %q, want sanitized single-line attribute value", result)
	}
}

// TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_SanitizesKeys
// pins a second review round's finding: formatGenericDetail emits
// sentenceCaseKey(k) directly into a "%s: %s\n" line without sanitizing the
// key, while the value on the same line already goes through
// sanitizeDisplayValue. An attribute key containing a newline (e.g. from an
// integration or template-provided attribute) must not be able to forge an
// extra "Key: value" line in the natural-format response - the same
// line-injection guarantee sanitizeDisplayValue already gives values.
func TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_SanitizesKeys(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()

	detail := map[string]any{
		"entity_id":     "select.mode",
		"friendly_name": "Mode",
		"state":         "eco",
		"note\nForged":  "attacker-controlled",
	}

	result, err := f.FormatHelperDetail(context.Background(), "select", detail)
	if err != nil {
		t.Fatalf("FormatHelperDetail(select) error = %v, want nil", err)
	}

	if strings.Contains(result, "\nForged:") {
		t.Errorf("FormatHelperDetail(select) = %q, an attribute key must not be able to forge a standalone additional line", result)
	}
	if !strings.Contains(result, "Note Forged: attacker-controlled") {
		t.Errorf("FormatHelperDetail(select) = %q, want sanitized single-line key rendered on one line", result)
	}

	lines := strings.Split(result, "\n")
	wantLines := 3 // header, State, Note Forged
	if len(lines) != wantLines {
		t.Errorf("FormatHelperDetail(select) produced %d lines %q, want %d - a forged key must not add a line", len(lines), lines, wantLines)
	}
}

// TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_ParensSurvive
// documents that formatDetailValue's sanitization is deliberately narrower
// than sanitizeDisplayName's: it strips newlines but not parentheses, since
// "(eco)" is a legitimate attribute value with no id-forging risk the way a
// forged "(entity_id)" suffix in a name would have.
func TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_ParensSurvive(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()

	detail := map[string]any{
		"entity_id":     "select.mode",
		"friendly_name": "Mode",
		"state":         "eco",
		"preset":        "Auto (eco)",
	}

	result, err := f.FormatHelperDetail(context.Background(), "select", detail)
	if err != nil {
		t.Fatalf("FormatHelperDetail(select) error = %v, want nil", err)
	}
	if !strings.Contains(result, "Auto (eco)") {
		t.Errorf("FormatHelperDetail(select) = %q, want attribute value parentheses preserved", result)
	}
}

// TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_NoFriendlyName
// pins finding N5: when an entity has no friendly_name, the header must
// collapse to the bare entity_id via FormatNameWithID rather than rendering
// "id (id)".
func TestNaturalHelperFormatter_FormatHelperDetail_GenericFallback_NoFriendlyName(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()

	detail := map[string]any{
		"entity_id": "select.mode",
		"state":     "eco",
	}

	result, err := f.FormatHelperDetail(context.Background(), "select", detail)
	if err != nil {
		t.Fatalf("FormatHelperDetail(select) error = %v, want nil", err)
	}
	if !strings.Contains(result, "Select: select.mode\n") {
		t.Errorf("FormatHelperDetail(select) = %q, want header to collapse to the bare entity_id", result)
	}
	if strings.Contains(result, "select.mode (select.mode)") {
		t.Errorf("FormatHelperDetail(select) = %q, must not duplicate the id in parentheses", result)
	}
}

// TestDedicatedDetailTypes_MatchesFormatHelperDetailDispatch pins finding W4:
// DedicatedDetailTypes() must report exactly the helper types that get a
// dedicated (non-generic) natural-format renderer, so
// TestHelperDetailBuilders_MatchDedicatedRenderers in the handlers package
// can assert the two packages' registries stay in sync instead of silently
// drifting the way builder/renderer coverage drifted before issue #216.
func TestDedicatedDetailTypes_MatchesFormatHelperDetailDispatch(t *testing.T) {
	t.Parallel()

	want := []string{
		"binary_sensor", "group", "input_boolean", "input_button", "input_datetime",
		"input_number", "input_select", "input_text", "sensor",
	}

	got := DedicatedDetailTypes()
	if len(got) != len(want) {
		t.Fatalf("DedicatedDetailTypes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DedicatedDetailTypes() = %v, want %v", got, want)
		}
	}
}

func TestNaturalHelperFormatter_FormatCounterDetail_IncludesEntityID(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()
	detail := map[string]any{
		"entity_id":     "counter.visitors",
		"friendly_name": "Visitors",
		"state":         "3",
	}

	result, err := f.FormatCounterDetail(context.Background(), detail)
	if err != nil {
		t.Fatalf("FormatCounterDetail() error = %v", err)
	}
	if !strings.Contains(result, "Counter: Visitors (counter.visitors)") {
		t.Errorf("FormatCounterDetail() = %q, expected entity_id alongside name", result)
	}
}

func TestNaturalHelperFormatter_FormatTimerDetail_IncludesEntityID(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()
	detail := map[string]any{
		"entity_id":     "timer.pomodoro",
		"friendly_name": "Pomodoro",
		"state":         "active",
	}

	result, err := f.FormatTimerDetail(context.Background(), detail)
	if err != nil {
		t.Fatalf("FormatTimerDetail() error = %v", err)
	}
	if !strings.Contains(result, "Timer: Pomodoro (timer.pomodoro)") {
		t.Errorf("FormatTimerDetail() = %q, expected entity_id alongside name", result)
	}
}

func TestNaturalHelperFormatter_FormatScheduleDetail_NameOrder(t *testing.T) {
	t.Parallel()

	f := NewNaturalHelperFormatter()
	detail := map[string]any{
		"entity_id":     "schedule.work_hours",
		"friendly_name": "Work Hours",
		"state":         "on",
	}

	result, err := f.FormatScheduleDetail(context.Background(), detail)
	if err != nil {
		t.Fatalf("FormatScheduleDetail() error = %v", err)
	}
	if !strings.Contains(result, "Schedule: Work Hours (schedule.work_hours) is on") {
		t.Errorf("FormatScheduleDetail() = %q, expected Name (entity_id) is state shape", result)
	}
}

// =============================================================================
// JSONHelperFormatter - FormatList Tests
// =============================================================================

func TestJSONHelperFormatter_FormatList_Empty(t *testing.T) {
	t.Parallel()

	formatter := NewJSONHelperFormatter()
	result, err := formatter.FormatList(context.Background(), nil, HelperListOptions{})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Should be valid JSON empty array
	var parsed []any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("FormatList(nil) returned invalid JSON: %v", err)
	}
	if len(parsed) != 0 {
		t.Errorf("FormatList(nil) should return empty array, got %d elements", len(parsed))
	}
}

func TestJSONHelperFormatter_FormatList_ValidJSON(t *testing.T) {
	t.Parallel()

	formatter := NewJSONHelperFormatter()
	helpers := createTestHelpers()
	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Should be valid JSON
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("FormatList() returned invalid JSON: %v\nOutput:\n%s", err, result)
	}

	// Should have correct number of helpers
	if len(parsed) != len(helpers) {
		t.Errorf("FormatList() returned %d helpers, want %d", len(parsed), len(helpers))
	}
}

func TestJSONHelperFormatter_FormatList_ContainsEntityIDs(t *testing.T) {
	t.Parallel()

	formatter := NewJSONHelperFormatter()
	helpers := createTestHelpers()
	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Check that entity_ids are present
	for _, helper := range helpers {
		if !strings.Contains(result, helper.EntityID) {
			t.Errorf("FormatList() missing entity_id %q", helper.EntityID)
		}
	}
}

func TestJSONHelperFormatter_FormatList_Structure(t *testing.T) {
	t.Parallel()

	formatter := NewJSONHelperFormatter()
	helpers := []homeassistant.Entity{
		testHelperEntity("counter.test", "42", map[string]any{
			"friendly_name": "Test Counter",
		}),
	}
	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	var parsed []map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatList() returned invalid JSON: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("FormatList() returned %d helpers, want 1", len(parsed))
	}

	helper := parsed[0]

	// Check required fields
	if helper["entity_id"] != "counter.test" {
		t.Errorf("entity_id = %v, want 'counter.test'", helper["entity_id"])
	}
	if helper["state"] != "42" {
		t.Errorf("state = %v, want '42'", helper["state"])
	}
}

// =============================================================================
// JSONHelperFormatter - FormatScheduleDetail Tests
// =============================================================================

func TestJSONHelperFormatter_FormatScheduleDetail(t *testing.T) {
	t.Parallel()

	formatter := NewJSONHelperFormatter()
	detail := map[string]any{
		"entity_id":     "schedule.work_hours",
		"state":         "on",
		"friendly_name": "Work Hours",
		"schedule": map[string][]map[string]string{
			"monday": {{"from": "06:00", "to": "08:00"}},
		},
	}

	result, err := formatter.FormatScheduleDetail(context.Background(), detail)

	if err != nil {
		t.Fatalf("FormatScheduleDetail() error = %v", err)
	}

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("FormatScheduleDetail() returned invalid JSON: %v", err)
	}

	// Check fields are present
	if parsed["entity_id"] != "schedule.work_hours" {
		t.Errorf("entity_id = %v, want 'schedule.work_hours'", parsed["entity_id"])
	}
}

func TestJSONHelperFormatter_FormatScheduleDetail_Empty(t *testing.T) {
	t.Parallel()

	formatter := NewJSONHelperFormatter()
	detail := map[string]any{}

	result, err := formatter.FormatScheduleDetail(context.Background(), detail)

	if err != nil {
		t.Fatalf("FormatScheduleDetail() error = %v", err)
	}

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("FormatScheduleDetail() returned invalid JSON: %v", err)
	}
}

// =============================================================================
// HelperListOptions Tests
// =============================================================================

func TestHelperListOptions_Defaults(t *testing.T) {
	t.Parallel()

	opts := HelperListOptions{}

	if opts.Verbose {
		t.Error("Default Verbose should be false")
	}
	if opts.GroupByType {
		t.Error("Default GroupByType should be false")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestHelperFormatter_UnknownDomain(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := []homeassistant.Entity{
		testHelperEntity("unknown_domain.test", "value", map[string]any{
			"friendly_name": "Unknown Helper",
		}),
	}

	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{GroupByType: true})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Should still include the helper
	if !strings.Contains(result, "Unknown Helper") {
		t.Errorf("FormatList() should include unknown domain helper, got:\n%s", result)
	}
}

func TestHelperFormatter_MissingFriendlyName(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := []homeassistant.Entity{
		testHelperEntity("counter.no_name", "5", nil),
	}

	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{GroupByType: true})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Should use entity_id when friendly_name is missing
	if !strings.Contains(result, "no_name") {
		t.Errorf("FormatList() should show entity_id when no friendly_name, got:\n%s", result)
	}
}

func TestHelperFormatter_AllHelperTypes(t *testing.T) {
	t.Parallel()

	formatter := NewNaturalHelperFormatter()
	helpers := []homeassistant.Entity{
		testHelperEntity("input_boolean.test", "on", nil),
		testHelperEntity("input_number.test", "42", nil),
		testHelperEntity("input_text.test", "hello", nil),
		testHelperEntity("input_select.test", "option1", nil),
		testHelperEntity("input_datetime.test", "2024-01-15 10:00:00", nil),
		testHelperEntity("input_button.test", "unknown", nil),
		testHelperEntity("counter.test", "10", nil),
		testHelperEntity("timer.test", "idle", nil),
		testHelperEntity("schedule.test", "on", nil),
		testHelperEntity("group.test", "on", nil),
		testHelperEntity("sensor.template_test", "100", nil),
		testHelperEntity("binary_sensor.template_test", "on", nil),
	}

	result, err := formatter.FormatList(context.Background(), helpers, HelperListOptions{GroupByType: true})

	if err != nil {
		t.Fatalf("FormatList() error = %v", err)
	}

	// Should have 12 helpers
	if !strings.Contains(result, "12 helpers") {
		t.Errorf("FormatList() should show 12 helpers, got:\n%s", result)
	}
}
