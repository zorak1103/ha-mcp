package handlers

import "testing"

func TestActionValue_AcceptsSingleActionDict(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	args := map[string]any{
		"turn_on": map[string]any{
			"action": "switch.turn_on",
			"target": map[string]any{"entity_id": "switch.x"},
		},
	}

	r := newArgReader(config, args)
	r.actionValue("turn_on")
	if err := r.err(); err != nil {
		t.Fatalf("actionValue() error = %v", err)
	}

	got, ok := config["turn_on"].(map[string]any)
	if !ok {
		t.Fatalf("config[turn_on] = %#v, want map[string]any", config["turn_on"])
	}
	if got["action"] != "switch.turn_on" {
		t.Errorf("config[turn_on][action] = %v, want switch.turn_on", got["action"])
	}
}

func TestActionValue_AcceptsActionList(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	args := map[string]any{
		"turn_on": []any{
			map[string]any{"action": "switch.turn_on", "target": map[string]any{"entity_id": "switch.a"}},
			map[string]any{"action": "switch.turn_on", "target": map[string]any{"entity_id": "switch.b"}},
		},
	}

	r := newArgReader(config, args)
	r.actionValue("turn_on")
	if err := r.err(); err != nil {
		t.Fatalf("actionValue() error = %v", err)
	}

	got, ok := config["turn_on"].([]any)
	if !ok || len(got) != 2 {
		t.Fatalf("config[turn_on] = %#v, want a 2-element list", config["turn_on"])
	}
}

func TestActionValue_RejectsScalar(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	args := map[string]any{"turn_on": "not an action"}

	r := newArgReader(config, args)
	r.actionValue("turn_on")
	if err := r.err(); err == nil {
		t.Fatal("expected an error for a scalar action value, got nil")
	}
}

func TestActionValue_RejectsExcessiveDepth(t *testing.T) {
	t.Parallel()

	// Build a chain of nested maps deeper than maxActionDepth.
	var deep any = "leaf"
	for i := 0; i < maxActionDepth+5; i++ {
		deep = map[string]any{"nested": deep}
	}

	config := map[string]any{}
	args := map[string]any{"turn_on": deep}

	r := newArgReader(config, args)
	r.actionValue("turn_on")
	if err := r.err(); err == nil {
		t.Fatal("expected an error for excessive nesting depth, got nil")
	}
}

func TestActionValue_RejectsExcessiveNodeCount(t *testing.T) {
	t.Parallel()

	// A flat list with more entries than maxActionNodes allows.
	huge := make([]any, maxActionNodes+10)
	for i := range huge {
		huge[i] = map[string]any{"action": "x.y"}
	}

	config := map[string]any{}
	args := map[string]any{"turn_on": huge}

	r := newArgReader(config, args)
	r.actionValue("turn_on")
	if err := r.err(); err == nil {
		t.Fatal("expected an error for excessive node count, got nil")
	}
}

func TestActionValue_SkipsAbsentAndNull(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	args := map[string]any{"turn_on": nil}

	r := newArgReader(config, args)
	r.actionValue("turn_on")
	r.actionValue("turn_off") // absent entirely
	if err := r.err(); err != nil {
		t.Fatalf("actionValue() error = %v", err)
	}
	if _, present := config["turn_on"]; present {
		t.Errorf("config[turn_on] should be absent for a null value, got %#v", config["turn_on"])
	}
	if _, present := config["turn_off"]; present {
		t.Errorf("config[turn_off] should be absent when the arg key is missing, got %#v", config["turn_off"])
	}
}
