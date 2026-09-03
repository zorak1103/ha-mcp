package homeassistant

import (
	"math"
	"reflect"
	"testing"
)

func TestParseDurationString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want map[string]int
	}{
		{name: "HH:MM:SS", in: "00:01:30", want: map[string]int{"hours": 0, "minutes": 1, "seconds": 30}},
		{name: "H:MM:SS multi-digit hours", in: "12:00:00", want: map[string]int{"hours": 12, "minutes": 0, "seconds": 0}},
		// A 2-part string is H:MM, matching Home Assistant's own
		// cv.time_period_str - NOT MM:SS. "01:30" is 1h30m, not 90s.
		{name: "H:MM", in: "01:30", want: map[string]int{"hours": 1, "minutes": 30, "seconds": 0}},
		{name: "SS only", in: "90", want: map[string]int{"hours": 0, "minutes": 0, "seconds": 90}},
		{name: "empty string", in: "", want: nil},
		{name: "too many parts", in: "1:2:3:4", want: nil},
		{name: "non-numeric", in: "abc", want: nil},
		{name: "non-numeric in HH:MM:SS", in: "00:aa:30", want: nil},
		{name: "negative hours rejected instead of forwarding a negative duration", in: "-1:30:00", want: nil},
		{name: "negative minutes rejected", in: "1:-30:00", want: nil},
		{name: "negative seconds-only rejected", in: "-90", want: nil},
		{name: "component exceeding MaxInt32 rejected instead of overflowing", in: "99999999999:00:00", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseDurationString(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDurationString(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestToDurationDict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     any
		want   map[string]int
		wantOK bool
	}{
		{
			name: "dict passthrough with float64 values",
			in:   map[string]any{"hours": 0.0, "minutes": 1.0, "seconds": 30.0},
			want: map[string]int{"hours": 0, "minutes": 1, "seconds": 30}, wantOK: true,
		},
		{
			name: "dict passthrough with int values",
			in:   map[string]any{"hours": 0, "minutes": 1, "seconds": 30},
			want: map[string]int{"hours": 0, "minutes": 1, "seconds": 30}, wantOK: true,
		},
		{
			name: "dict with missing keys",
			in:   map[string]any{"minutes": 5.0},
			want: map[string]int{"minutes": 5}, wantOK: true,
		},
		{name: "map[string]int passthrough", in: map[string]int{"hours": 1, "minutes": 0, "seconds": 0}, want: map[string]int{"hours": 1, "minutes": 0, "seconds": 0}, wantOK: true},
		{name: "HH:MM:SS string", in: "00:01:30", want: map[string]int{"hours": 0, "minutes": 1, "seconds": 30}, wantOK: true},
		{name: "invalid string", in: "not-a-duration", want: nil, wantOK: false},
		{name: "bare seconds as float64", in: 90.0, want: map[string]int{"hours": 0, "minutes": 1, "seconds": 30}, wantOK: true},
		{name: "bare seconds as int", in: 3661, want: map[string]int{"hours": 1, "minutes": 1, "seconds": 1}, wantOK: true},
		{name: "bool rejected", in: true, want: nil, wantOK: false},
		{name: "array rejected", in: []any{1, 2}, want: nil, wantOK: false},
		{name: "nil rejected", in: nil, want: nil, wantOK: false},
		{
			name: "dict with wrong-typed recognized key is rejected, not silently dropped",
			in:   map[string]any{"hours": "1", "minutes": 30.0, "seconds": 0.0},
			want: nil, wantOK: false,
		},
		{
			name: "dict with an unrecognized key is rejected, not silently dropped",
			in:   map[string]any{"bogus": "x"},
			want: nil, wantOK: false,
		},
		{
			name: "dict with NaN value is rejected instead of silently overflowing to garbage",
			in:   map[string]any{"hours": math.NaN()},
			want: nil, wantOK: false,
		},
		{
			name: "dict with Inf value is rejected instead of silently overflowing to garbage",
			in:   map[string]any{"hours": math.Inf(1)},
			want: nil, wantOK: false,
		},
		{
			name: "dict with negative value is rejected",
			in:   map[string]any{"hours": -1.0},
			want: nil, wantOK: false,
		},
		{
			name: "dict with huge value is rejected instead of silently overflowing to garbage",
			in:   map[string]any{"hours": 1e300},
			want: nil, wantOK: false,
		},
		{
			name: "dict with days and milliseconds",
			in:   map[string]any{"days": 2.0, "hours": 1.0, "milliseconds": 500.0},
			want: map[string]int{"days": 2, "hours": 1, "milliseconds": 500}, wantOK: true,
		},
		{
			name: "map[string]int with an unrecognized key is rejected",
			in:   map[string]int{"hours": 1, "bogus": 2},
			want: nil, wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := toDurationDict(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("toDurationDict(%v) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}
			if tt.wantOK && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toDurationDict(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestCoerceForField_NameKeyedDurationFallback(t *testing.T) {
	t.Parallel()

	noSelector := OptionsFlowField{Name: "time_window"}
	got := coerceForField(noSelector, "00:05:00", "time_window", "user")
	want := map[string]int{"hours": 0, "minutes": 5, "seconds": 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("coerceForField(time_window, ...) = %v, want %v", got, want)
	}

	// A bare number of seconds for delay_on converts too (widened input
	// forms), where before this fix only an exact "HH:MM:SS" string worked.
	delayOnField := OptionsFlowField{Name: "delay_on"}
	got = coerceForField(delayOnField, 30.0, "delay_on", "user")
	want = map[string]int{"hours": 0, "minutes": 0, "seconds": 30}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("coerceForField(delay_on, 30.0) = %v, want %v", got, want)
	}

	// A non-duration field is passed through untouched.
	stateField := OptionsFlowField{Name: "state"}
	got = coerceForField(stateField, "some template", "state", "user")
	if got != "some template" {
		t.Errorf("coerceForField(state, ...) = %v, want passthrough", got)
	}
}

// TestFilterFlowSteps_SchemaDriven covers, via the schema-driven engine,
// the same six scenarios the deleted buildFilterStepConfig/filterStepFields
// allow-list table used to cover directly. Each subtest supplies a fixture
// schema shaped like HA's real per-step filter schema.
func TestFilterFlowSteps_SchemaDriven(t *testing.T) {
	t.Parallel()

	t.Run("user step keeps name entity_id filter", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{{Name: "name"}, {Name: "entity_id"}, {Name: "filter"}})
		userConfig := map[string]any{"name": "My Filter", "entity_id": "sensor.x", "filter": "outlier"}
		got := buildStepSubmission(flowModeCreate, ix, userConfig, map[string]bool{}, "user")
		want := map[string]any{"name": "My Filter", "entity_id": "sensor.x", "filter": "outlier"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("buildStepSubmission(user) = %v, want %v", got, want)
		}
	})

	t.Run("outlier step drops name, keeps read-only filter, converts nothing", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{
			{Name: "entity_id"}, {Name: "filter"}, {Name: "precision"}, {Name: "window_size"}, {Name: "radius"},
		})
		userConfig := map[string]any{
			"name": "My Filter", "entity_id": "sensor.x", "filter": "outlier",
			"window_size": 5.0, "radius": 2.0,
		}
		got := buildStepSubmission(flowModeCreate, ix, userConfig, map[string]bool{}, "outlier")
		want := map[string]any{"entity_id": "sensor.x", "filter": "outlier", "window_size": 5.0, "radius": 2.0}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("buildStepSubmission(outlier) = %v, want %v", got, want)
		}
	})

	t.Run("time_simple_moving_average step converts window_size to duration dict", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{
			{Name: "entity_id"}, {Name: "filter"}, {Name: "precision"}, {Name: "window_size"},
		})
		userConfig := map[string]any{
			"name": "My Filter", "entity_id": "sensor.x", "filter": "time_simple_moving_average",
			"window_size": "00:01:30",
		}
		got := buildStepSubmission(flowModeCreate, ix, userConfig, map[string]bool{}, "time_simple_moving_average")
		want := map[string]any{
			"entity_id":   "sensor.x",
			"filter":      "time_simple_moving_average",
			"window_size": map[string]int{"hours": 0, "minutes": 1, "seconds": 30},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("buildStepSubmission(time_simple_moving_average) = %v, want %v", got, want)
		}
	})

	t.Run("time_throttle step accepts bare-seconds window_size", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{
			{Name: "entity_id"}, {Name: "filter"}, {Name: "precision"}, {Name: "window_size"},
		})
		userConfig := map[string]any{
			"name": "My Filter", "entity_id": "sensor.x", "filter": "time_throttle",
			"window_size": 90.0,
		}
		got := buildStepSubmission(flowModeCreate, ix, userConfig, map[string]bool{}, "time_throttle")
		want := map[string]any{
			"entity_id":   "sensor.x",
			"filter":      "time_throttle",
			"window_size": map[string]int{"hours": 0, "minutes": 1, "seconds": 30},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("buildStepSubmission(time_throttle) = %v, want %v", got, want)
		}
	})

	t.Run("stray filters key is dropped by the schema allow-list", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{
			{Name: "entity_id"}, {Name: "filter"}, {Name: "precision"}, {Name: "window_size"}, {Name: "radius"},
		})
		userConfig := map[string]any{
			"entity_id": "sensor.x", "filter": "outlier",
			"filters": []any{map[string]any{"filter": "outlier"}},
		}
		got := buildStepSubmission(flowModeCreate, ix, userConfig, map[string]bool{}, "outlier")
		if _, present := got["filters"]; present {
			t.Errorf("buildStepSubmission(outlier) kept unexpected key \"filters\": %v", got)
		}
	})

	t.Run("a step whose schema could not be inspected forwards everything", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema(nil)
		userConfig := map[string]any{"entity_id": "sensor.x", "filter": "outlier", "unexpected_future_field": "x"}
		got := buildStepSubmission(flowModeCreate, ix, userConfig, map[string]bool{}, "some_future_filter_type")
		if _, present := got["unexpected_future_field"]; !present {
			t.Errorf("buildStepSubmission with an uninspectable schema should forward everything, got %v", got)
		}
	})
}

// TestFilterFlowSteps_UnconsumedFieldsAreCollected is the placementOf-based
// replacement for the deleted restrictToSchemaFields: a field the step's
// schema doesn't declare is left out of the submission and reported via
// unconsumedUserFields rather than silently dropped with no trace.
func TestFilterFlowSteps_UnconsumedFieldsAreCollected(t *testing.T) {
	t.Parallel()

	ix := indexStepSchema([]OptionsFlowField{{Name: "radius"}, {Name: "precision"}})
	userConfig := map[string]any{"radius": 3.0, "name": "should be dropped", "filters": "should be dropped too"}
	consumed := map[string]bool{}

	got := buildStepSubmission(flowModeCreate, ix, userConfig, consumed, "outlier")
	if len(got) != 1 || got["radius"] != 3.0 {
		t.Errorf("buildStepSubmission() = %v, want only radius=3.0", got)
	}

	dropped := unconsumedUserFields(userConfig, consumed)
	wantDropped := []string{"filters", "name"}
	if !reflect.DeepEqual(dropped, wantDropped) {
		t.Errorf("unconsumedUserFields() = %v, want %v", dropped, wantDropped)
	}
}
