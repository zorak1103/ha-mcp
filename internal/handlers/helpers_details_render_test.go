package handlers

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// naturalRendererFixtures are realistic state attribute sets for every helper
// type with a dedicated natural renderer, covering every attribute its
// add*Details builder copies.
var naturalRendererFixtures = map[string]*homeassistant.Entity{
	"input_boolean": {EntityID: "input_boolean.b", State: "on", Attributes: map[string]any{
		"friendly_name": "B", "icon": "mdi:toggle-switch", "editable": true,
	}},
	"input_number": {EntityID: "input_number.n", State: "5.0", Attributes: map[string]any{
		"friendly_name": "N", "min": 0.0, "max": 100.0, "step": 0.5, "mode": "box",
		"unit_of_measurement": "W", "editable": true,
	}},
	"input_text": {EntityID: "input_text.t", State: "hello", Attributes: map[string]any{
		"friendly_name": "T", "min": 0.0, "max": 100.0, "mode": "text", "pattern": "[a-z]+", "editable": true,
	}},
	"input_select": {EntityID: "input_select.s", State: "a", Attributes: map[string]any{
		"friendly_name": "S", "options": []any{"a", "b", "c"}, "editable": true,
	}},
	"input_datetime": {EntityID: "input_datetime.d", State: "2026-10-24 08:30:00", Attributes: map[string]any{
		"friendly_name": "D", "has_date": true, "has_time": true, "year": 2026.0, "month": 10.0, "day": 24.0,
		"hour": 8.0, "minute": 30.0, "second": 0.0, "timestamp": 1792830600.0, "editable": true,
	}},
	"input_button": {EntityID: "input_button.p", State: "2026-10-24T08:00:00+00:00", Attributes: map[string]any{
		"friendly_name": "P", "icon": "mdi:gesture-tap-button", "editable": true,
	}},
	"group": {EntityID: "light.grp", State: "on", Attributes: map[string]any{
		"friendly_name": "Grp", "entity_id": []any{"light.a", "light.b"}, "all": true,
	}},
	"sensor": {EntityID: "sensor.s", State: "12.5", Attributes: map[string]any{
		"friendly_name": "S", "unit_of_measurement": "W", "device_class": "power",
		"state_class": "measurement", "source": "sensor.src",
	}},
	"binary_sensor": {EntityID: "binary_sensor.thr", State: "off", Attributes: map[string]any{
		"friendly_name": "Thr", "device_class": "heat", "entity_id": "sensor.src",
		"hysteresis": 0.5, "sensor_value": 18.2,
	}},
}

// naturalRendererUnprintedKeys lists the detail keys a dedicated natural
// renderer intentionally does not print as their own value, per helper type,
// each with the reason. None of them hides a stored, writable value: a stored
// key deduplicated by shownAtTopLevel must always be printed (or rendered as
// an equivalent line, as noted).
var naturalRendererUnprintedKeys = map[string]map[string]string{
	"*": {
		"entity_id":     "identity; part of (or implied by) the header line",
		"state":         "runtime value; rendered as the current-value line",
		"friendly_name": "identity; the header line",
	},
	"input_number": {"editable": "runtime UI flag (storage- vs YAML-managed), not stored config"},
	"input_text":   {"editable": "runtime UI flag (storage- vs YAML-managed), not stored config"},
	"input_select": {"editable": "runtime UI flag (storage- vs YAML-managed), not stored config"},
	"input_button": {"editable": "runtime UI flag (storage- vs YAML-managed), not stored config"},
	"input_datetime": {
		"editable":  "runtime UI flag (storage- vs YAML-managed), not stored config",
		"has_date":  "rendered as the Type: line (Date and time / Date only / Time only)",
		"has_time":  "rendered as the Type: line (Date and time / Date only / Time only)",
		"year":      "decomposition of the current value, already on the Current value: line",
		"month":     "decomposition of the current value, already on the Current value: line",
		"day":       "decomposition of the current value, already on the Current value: line",
		"hour":      "decomposition of the current value, already on the Current value: line",
		"minute":    "decomposition of the current value, already on the Current value: line",
		"second":    "decomposition of the current value, already on the Current value: line",
		"timestamp": "decomposition of the current value, already on the Current value: line",
	},
	"group":         {"all": "rendered as the Mode: line (all / any)"},
	"binary_sensor": {"sensor_value": "runtime reading of the threshold source, not stored config"},
}

// TestNaturalRenderers_PrintEveryDetailKey pins that, for every dedicated
// helper renderer, each detail key the real builder produces is visible in
// the natural output. shownAtTopLevel drops a stored config value whenever
// the same value sits in the detail map, so a detail key the renderer never
// prints would make that stored value vanish from natural output entirely.
func TestNaturalRenderers_PrintEveryDetailKey(t *testing.T) {
	t.Parallel()

	types := formatter.DedicatedDetailTypes()
	for _, helperType := range types {
		t.Run(helperType, func(t *testing.T) {
			t.Parallel()
			state, ok := naturalRendererFixtures[helperType]
			if !ok {
				t.Fatalf("no fixture for dedicated renderer %q", helperType)
			}
			details := buildHelperDetails(state, helperType)
			out, err := formatter.NewHelperFormatter(formatter.FormatNatural).FormatHelperDetail(context.Background(), helperType, details)
			if err != nil {
				t.Fatalf("FormatHelperDetail() error = %v", err)
			}
			for key, value := range details {
				if _, skip := naturalRendererUnprintedKeys["*"][key]; skip {
					continue
				}
				if _, skip := naturalRendererUnprintedKeys[helperType][key]; skip {
					continue
				}
				for _, want := range detailValueTexts(value) {
					if !strings.Contains(out, want) {
						t.Errorf("detail key %q value %q not printed in natural output:\n%s", key, want, out)
					}
				}
			}
		})
	}

	for helperType := range naturalRendererFixtures {
		if !slices.Contains(types, helperType) {
			t.Errorf("fixture %q has no dedicated renderer; remove it", helperType)
		}
	}
}

// detailValueTexts returns the texts a rendered detail value must contain:
// every element of a list, or the value itself.
func detailValueTexts(value any) []string {
	if list, ok := value.([]any); ok {
		texts := make([]string, 0, len(list))
		for _, v := range list {
			texts = append(texts, fmt.Sprint(v))
		}
		return texts
	}
	return []string{fmt.Sprint(value)}
}

func TestGetDetails_InputButtonStoredIconShownInNatural(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("2026-10-24T08:00:00+00:00", map[string]any{"friendly_name": "P", "icon": "mdi:gesture-tap"}),
		GetHelperConfigFn: storageFn(map[string]any{
			"id": "p", "name": "P", "icon": "mdi:gesture-tap",
		}),
	}

	out := runGetDetails(t, m, "input_button.p", "natural").Content[0].Text
	if !strings.Contains(out, "Icon: mdi:gesture-tap") {
		t.Errorf("natural output missing %q:\n%s", "Icon: mdi:gesture-tap", out)
	}
}

func TestGetDetails_ThresholdStoredHysteresisShownInNatural(t *testing.T) {
	t.Parallel()

	m := &UniversalMockClient{
		GetStateFn: stateFn("off", map[string]any{
			"friendly_name": "Thr", "entity_id": "sensor.src", "hysteresis": 0.5, "sensor_value": 18.2,
		}),
		GetEntityRegistryFn: registryFn(homeassistant.EntityRegistryEntry{
			EntityID: "binary_sensor.thr", Platform: "threshold", ConfigEntryID: "ce",
		}),
		GetConfigEntryOptionsFn: optionsFn(map[string]any{
			"entity_id": "sensor.src", "hysteresis": 0.5, "lower": 10.0,
		}),
	}

	out := runGetDetails(t, m, "binary_sensor.thr", "natural").Content[0].Text
	if !strings.Contains(out, "Hysteresis: 0.5") {
		t.Errorf("natural output missing %q:\n%s", "Hysteresis: 0.5", out)
	}
}
