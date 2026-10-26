package access

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// parseDef mimics the caller: a JSON round trip of the typed config.
func parseDef(t *testing.T, src string) map[string]any {
	t.Helper()
	var def map[string]any
	if err := json.Unmarshal([]byte(src), &def); err != nil {
		t.Fatalf("bad test json: %v", err)
	}
	return def
}

func mustExtract(t *testing.T, src string) []Call {
	t.Helper()
	calls, err := ExtractCalls(parseDef(t, src))
	if err != nil {
		t.Fatalf("ExtractCalls: %v", err)
	}
	return calls
}

func callNames(calls []Call) []string {
	names := make([]string, 0, len(calls))
	for _, c := range calls {
		names = append(names, c.Domain+"."+c.Service)
	}
	return names
}

func TestExtract_NestedChooseIfRepeatParallel(t *testing.T) {
	calls := mustExtract(t, `{
	  "alias": "x",
	  "triggers": [{"trigger": "state", "entity_id": "binary_sensor.door"}],
	  "actions": [
	    {"service": "light.turn_on", "target": {"entity_id": "light.a"}},
	    {"if": [{"condition": "state", "entity_id": "light.a", "state": "on"}],
	     "then": [{"action": "switch.turn_on", "target": {"entity_id": ["switch.b", "switch.c"]}}],
	     "else": [{"action": "switch.turn_off", "data": {"entity_id": "switch.b"}}]},
	    {"choose": [
	       {"conditions": [], "sequence": [{"service": "cover.open_cover", "target": {"area_id": "garage"}}]}
	     ],
	     "default": [{"service": "notify.mobile", "data": {"message": "hi"}}]},
	    {"repeat": {"count": 3, "sequence": [{"service": "fan.toggle", "target": {"entity_id": "fan.x"}}]}},
	    {"parallel": [
	       {"service": "lock.lock", "target": {"entity_id": "lock.front"}},
	       {"sequence": [{"service": "lock.unlock", "target": {"device_id": "dev1"}}]}
	    ]},
	    {"delay": "00:00:05"},
	    {"wait_template": "{{ is_state('light.a', 'on') }}"},
	    {"variables": {"x": "{{ 1 }}"}},
	    {"condition": "state", "entity_id": "light.a", "state": "on"}
	  ]
	}`)
	want := []string{
		"light.turn_on", "switch.turn_on", "switch.turn_off", "cover.open_cover",
		"notify.mobile", "fan.toggle", "lock.lock", "lock.unlock",
	}
	if got := callNames(calls); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	if got := calls[1].Targets.EntityIDs; !reflect.DeepEqual(got, []string{"switch.b", "switch.c"}) {
		t.Errorf("switch.turn_on entity ids = %v", got)
	}
	if got := calls[2].Targets.EntityIDs; !reflect.DeepEqual(got, []string{"switch.b"}) {
		t.Errorf("data.entity_id targets = %v", got)
	}
	if got := calls[3].Targets.AreaIDs; !reflect.DeepEqual(got, []string{"garage"}) {
		t.Errorf("area ids = %v", got)
	}
	if got := calls[7].Targets.DeviceIDs; !reflect.DeepEqual(got, []string{"dev1"}) {
		t.Errorf("device ids = %v", got)
	}
}

func TestExtract_ScriptSequenceAndLegacyAction(t *testing.T) {
	calls := mustExtract(t, `{"sequence": [{"service": "light.turn_on", "entity_id": "light.a"}]}`)
	if len(calls) != 1 || !reflect.DeepEqual(calls[0].Targets.EntityIDs, []string{"light.a"}) {
		t.Errorf("script sequence/top-level entity_id: %+v", calls)
	}
	calls = mustExtract(t, `{"action": [{"service": "light.turn_on"}, {"service": "light.turn_off"}]}`)
	if len(calls) != 2 {
		t.Errorf("legacy action list: %+v", calls)
	}
	calls = mustExtract(t, `{"action": {"service": "light.turn_on"}}`)
	if len(calls) != 1 {
		t.Errorf("legacy action map: %+v", calls)
	}
	calls = mustExtract(t, `{"alias": "scene", "entities": {"light.a": {"state": "on"}}}`)
	if len(calls) != 0 {
		t.Errorf("definition without actions yields calls: %+v", calls)
	}
}

func TestExtract_NormalizesServiceName(t *testing.T) {
	calls := mustExtract(t, `{"actions": [{"service": " Light.Turn_On ", "target": {"entity_id": " Light.A "}}]}`)
	if calls[0].Domain != "light" || calls[0].Service != "turn_on" {
		t.Errorf("not normalized: %+v", calls[0])
	}
	if !reflect.DeepEqual(calls[0].Targets.EntityIDs, []string{"light.a"}) {
		t.Errorf("targets not normalized: %v", calls[0].Targets.EntityIDs)
	}
}

func TestExtract_TemplateServiceName_Undecidable(t *testing.T) {
	for _, src := range []string{
		`{"actions": [{"service": "{{ 'light.turn_on' }}"}]}`,
		`{"actions": [{"action": "light.{% if true %}turn_on{% endif %}"}]}`,
		`{"actions": [{"service": "light.turn_on", "service_template": "x"}]}`,
	} {
		if _, err := ExtractCalls(parseDef(t, src)); err == nil {
			t.Errorf("expected error for %s", src)
		}
	}
}

func TestExtract_UseBlueprint_Undecidable(t *testing.T) {
	_, err := ExtractCalls(parseDef(t, `{"use_blueprint": {"path": "a.yaml", "input": {}}}`))
	if err == nil {
		t.Fatal("expected error for use_blueprint")
	}
	_, err = ExtractCalls(parseDef(t, `{"use_blueprint": null, "actions": []}`))
	if err == nil {
		t.Fatal("expected error for a null use_blueprint key as well")
	}
}

func TestExtract_SceneShorthandAndDeviceAction(t *testing.T) {
	calls := mustExtract(t, `{"actions": [
	  {"scene": "scene.movie"},
	  {"scene": "Evening"},
	  {"device_id": "abc123", "domain": "Light", "entity_id": "reg-uuid", "type": "turn_on"},
	  {"event": "my_event", "event_data": {"a": 1}}
	]}`)
	if len(calls) != 4 {
		t.Fatalf("got %d calls: %+v", len(calls), calls)
	}
	if calls[0].Domain != "scene" || calls[0].Service != "turn_on" ||
		!reflect.DeepEqual(calls[0].Targets.EntityIDs, []string{"scene.movie"}) {
		t.Errorf("scene shorthand: %+v", calls[0])
	}
	if !reflect.DeepEqual(calls[1].Targets.EntityIDs, []string{"scene.evening"}) {
		t.Errorf("scene prefix not added: %+v", calls[1].Targets)
	}
	if calls[2].Domain != "light" || calls[2].Service != "device_action" ||
		!reflect.DeepEqual(calls[2].Targets.DeviceIDs, []string{"abc123"}) {
		t.Errorf("device action: %+v", calls[2])
	}
	if calls[3].Domain != "homeassistant" || calls[3].Service != "event" || len(calls[3].Targets.EntityIDs) != 0 {
		t.Errorf("event: %+v", calls[3])
	}
}

func TestExtract_DeviceConditionIsNoCall(t *testing.T) {
	calls := mustExtract(t, `{"actions": [
	  {"condition": "device", "device_id": "d", "domain": "light", "type": "is_on", "entity_id": "x"}
	]}`)
	if len(calls) != 0 {
		t.Errorf("device condition produced calls: %+v", calls)
	}
}

func TestExtract_TemplatedMessageDataIsFine(t *testing.T) {
	calls := mustExtract(t, `{"actions": [
	  {"service": "notify.mobile", "data": {"message": "{{ states('sensor.x') }}", "title": "{% if true %}a{% endif %}"}},
	  {"service": "light.turn_on", "target": {"entity_id": "light.a"}, "data": {"brightness_template": "{{ 5 }}"}}
	]}`)
	if len(calls) != 2 {
		t.Fatalf("got %d calls", len(calls))
	}
}

func TestExtract_TemplatedTarget_Undecidable(t *testing.T) {
	for name, src := range map[string]string{
		"target entity":     `{"actions": [{"service": "light.turn_on", "target": {"entity_id": "{{ x }}"}}]}`,
		"target list":       `{"actions": [{"service": "light.turn_on", "target": {"entity_id": ["light.a", "{{ x }}"]}}]}`,
		"target area":       `{"actions": [{"service": "light.turn_on", "target": {"area_id": "{% x %}"}}]}`,
		"target device":     `{"actions": [{"service": "light.turn_on", "target": {"device_id": "{{ x }}"}}]}`,
		"target floor":      `{"actions": [{"service": "light.turn_on", "target": {"floor_id": "{{ x }}"}}]}`,
		"target label":      `{"actions": [{"service": "light.turn_on", "target": {"label_id": "{{ x }}"}}]}`,
		"target whole":      `{"actions": [{"service": "light.turn_on", "target": "{{ x }}"}]}`,
		"data entity":       `{"actions": [{"service": "light.turn_on", "data": {"entity_id": "{{ x }}"}}]}`,
		"data nested":       `{"actions": [{"service": "light.turn_on", "data": {"target": {"entity_id": "{{ x }}"}}}]}`,
		"data_template":     `{"actions": [{"service": "light.turn_on", "data_template": {"entity_id": "{{ x }}"}}]}`,
		"top-level entity":  `{"actions": [{"service": "light.turn_on", "entity_id": "{{ x }}"}]}`,
		"scene entities":    `{"actions": [{"service": "scene.apply", "data": {"entities": {"{{ x }}": {"state": "on"}}}}]}`,
		"scene list":        `{"actions": [{"service": "scene.apply", "data": {"entities": ["{{ x }}"]}}]}`,
		"scene shorthand":   `{"actions": [{"scene": "{{ x }}"}]}`,
		"device action tpl": `{"actions": [{"device_id": "{{ x }}", "domain": "light", "type": "turn_on"}]}`,
		"data as template":  `{"actions": [{"service": "light.turn_on", "data": "{{ x }}"}]}`,
	} {
		if _, err := ExtractCalls(parseDef(t, src)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestExtract_SceneApplyEntitiesMap(t *testing.T) {
	calls := mustExtract(t, `{"actions": [{"service": "scene.apply", "data": {
	  "entities": {"Light.B": {"state": "{{ 'on' }}", "brightness": 5}, "light.a": "on"}}}]}`)
	if len(calls) != 1 {
		t.Fatalf("got %d calls", len(calls))
	}
	if !reflect.DeepEqual(calls[0].EntityData, []string{"light.a", "light.b"}) {
		t.Errorf("EntityData = %v", calls[0].EntityData)
	}
	if calls[0].DataMalformed {
		t.Error("DataMalformed set on a decidable call")
	}
}

func TestExtract_Malformed_Undecidable(t *testing.T) {
	for name, src := range map[string]string{
		"target number":    `{"actions": [{"service": "light.turn_on", "target": {"entity_id": 5}}]}`,
		"target not map":   `{"actions": [{"service": "light.turn_on", "target": ["light.a"]}]}`,
		"target null":      `{"actions": [{"service": "light.turn_on", "target": {"entity_id": null}}]}`,
		"data entity list": `{"actions": [{"service": "light.turn_on", "data": {"entity_id": [1]}}]}`,
		"entities number":  `{"actions": [{"service": "scene.apply", "data": {"entities": 5}}]}`,
		"data not map":     `{"actions": [{"service": "light.turn_on", "data": [1]}]}`,
		"device id number": `{"actions": [{"device_id": 5, "domain": "light", "type": "turn_on"}]}`,
		"device domain":    `{"actions": [{"device_id": "d", "domain": 5, "type": "turn_on"}]}`,
		"service number":   `{"actions": [{"service": 5}]}`,
		"scene number":     `{"actions": [{"scene": 5}]}`,
		"event number":     `{"actions": [{"event": 5}]}`,
	} {
		if _, err := ExtractCalls(parseDef(t, src)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestExtract_DotlessOrEmptyService_Undecidable(t *testing.T) {
	for _, name := range []string{"turn_on", "", ".turn_on", "light.", " . "} {
		def := map[string]any{"actions": []any{map[string]any{"service": name}}}
		if _, err := ExtractCalls(def); err == nil {
			t.Errorf("service %q: expected error", name)
		}
	}
}

func TestExtract_UnknownNode_Undecidable(t *testing.T) {
	for name, src := range map[string]string{
		"unknown key":      `{"actions": [{"frobnicate": "x"}]}`,
		"empty map":        `{"actions": [{}]}`,
		"string node":      `{"actions": ["light.turn_on"]}`,
		"number node":      `{"actions": [5]}`,
		"null node":        `{"actions": [null]}`,
		"actions string":   `{"actions": "x"}`,
		"then without if":  `{"actions": [{"then": []}]}`,
		"device partial":   `{"actions": [{"device_id": "d"}]}`,
		"nested unknown":   `{"actions": [{"if": [], "then": [{"nope": 1}]}]}`,
		"choose option":    `{"actions": [{"choose": [{"sequence": [{"nope": 1}]}]}]}`,
		"choose not list":  `{"actions": [{"choose": "x"}]}`,
		"choose bad entry": `{"actions": [{"choose": [5]}]}`,
		"repeat not map":   `{"actions": [{"repeat": 3}]}`,
		"repeat inner":     `{"actions": [{"repeat": {"sequence": [{"nope": 1}]}}]}`,
		"parallel entry":   `{"actions": [{"parallel": [{"nope": 1}]}]}`,
		"default null":     `{"actions": [{"choose": [], "default": null}]}`,
	} {
		if _, err := ExtractCalls(parseDef(t, src)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestExtract_MixedNodeKeysAreAllWalked(t *testing.T) {
	calls := mustExtract(t, `{"actions": [{
	  "service": "light.turn_on",
	  "sequence": [{"service": "switch.turn_on"}],
	  "enabled": false, "alias": "a"
	}]}`)
	if got := callNames(calls); !reflect.DeepEqual(got, []string{"light.turn_on", "switch.turn_on"}) {
		t.Errorf("calls = %v", got)
	}
}

func nestedSequence(depth int) map[string]any {
	node := map[string]any{"service": "light.turn_on"}
	for i := 0; i < depth; i++ {
		node = map[string]any{"sequence": []any{node}}
	}
	return map[string]any{"actions": []any{node}}
}

func TestExtract_DepthLimit(t *testing.T) {
	if _, err := ExtractCalls(nestedSequence(5)); err != nil {
		t.Fatalf("shallow nesting rejected: %v", err)
	}
	if _, err := ExtractCalls(nestedSequence(100000)); err == nil {
		t.Fatal("pathological nesting accepted")
	}
}

func TestExtract_ToleratesTypedMapSlices(t *testing.T) {
	def := map[string]any{"actions": []map[string]any{
		{"service": "light.turn_on", "target": map[string]any{"entity_id": []string{"light.a"}}},
	}}
	calls, err := ExtractCalls(def)
	if err != nil || len(calls) != 1 {
		t.Fatalf("calls=%+v err=%v", calls, err)
	}
	if !reflect.DeepEqual(calls[0].Targets.EntityIDs, []string{"light.a"}) {
		t.Errorf("targets = %v", calls[0].Targets.EntityIDs)
	}
}

func TestExtract_RawIsCanonical(t *testing.T) {
	a := mustExtract(t, `{"actions": [{"service": "light.turn_on", "target": {"entity_id": "light.a"}, "data": {"b": 1, "a": 2}}]}`)
	b := mustExtract(t, `{"actions": [{"data": {"a": 2, "b": 1}, "target": {"entity_id": "light.a"}, "service": "light.turn_on"}]}`)
	if a[0].Raw == "" || a[0].Raw != b[0].Raw {
		t.Errorf("raw differs: %q vs %q", a[0].Raw, b[0].Raw)
	}
	if !strings.Contains(a[0].Raw, `"service":"light.turn_on"`) {
		t.Errorf("raw is not the node encoding: %q", a[0].Raw)
	}
}

func TestNewCalls_OnlyAddedOrChanged(t *testing.T) {
	oldCalls := mustExtract(t, `{"actions": [
	  {"service": "light.turn_on", "target": {"entity_id": "light.a"}},
	  {"service": "switch.turn_off", "target": {"entity_id": "switch.b"}}
	]}`)
	reordered := mustExtract(t, `{"actions": [
	  {"service": "switch.turn_off", "target": {"entity_id": "switch.b"}},
	  {"service": "light.turn_on", "target": {"entity_id": "light.a"}}
	]}`)
	if got := NewCalls(oldCalls, reordered); len(got) != 0 {
		t.Errorf("reordered untouched actions yielded %+v", got)
	}
	changed := mustExtract(t, `{"actions": [
	  {"service": "switch.turn_off", "target": {"entity_id": "switch.b"}},
	  {"service": "light.turn_on", "target": {"entity_id": "light.z"}},
	  {"service": "lock.unlock", "target": {"entity_id": "lock.f"}}
	]}`)
	got := NewCalls(oldCalls, changed)
	if names := callNames(got); !reflect.DeepEqual(names, []string{"light.turn_on", "lock.unlock"}) {
		t.Errorf("new calls = %v", names)
	}
	if got := NewCalls(nil, oldCalls); len(got) != 2 {
		t.Errorf("creation must yield all calls, got %d", len(got))
	}
	if got := NewCalls(oldCalls, nil); len(got) != 0 {
		t.Errorf("removal must yield none, got %d", len(got))
	}
}

func TestNewCalls_RespectsMultiplicity(t *testing.T) {
	one := mustExtract(t, `{"actions": [{"service": "light.turn_on", "target": {"entity_id": "light.a"}}]}`)
	two := mustExtract(t, `{"actions": [
	  {"service": "light.turn_on", "target": {"entity_id": "light.a"}},
	  {"service": "light.turn_on", "target": {"entity_id": "light.a"}}
	]}`)
	if got := NewCalls(one, two); len(got) != 1 {
		t.Errorf("duplicated call: got %d new, want 1", len(got))
	}
	if got := NewCalls(two, one); len(got) != 0 {
		t.Errorf("reduced duplicates: got %d new, want 0", len(got))
	}
}
