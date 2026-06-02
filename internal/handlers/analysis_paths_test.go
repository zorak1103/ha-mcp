package handlers

import (
	"reflect"
	"sort"
	"testing"
)

// --- collectEntityPaths ---

func TestCollectEntityPaths_StringLeafMatch(t *testing.T) {
	t.Parallel()

	got := collectEntityPaths("sensor.x", "sensor.x", "/prefix")
	want := []string{"/prefix"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectEntityPaths_StringLeafNoMatch(t *testing.T) {
	t.Parallel()

	got := collectEntityPaths("sensor.y", "sensor.x", "/prefix")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestCollectEntityPaths_NilNode(t *testing.T) {
	t.Parallel()

	got := collectEntityPaths(nil, "sensor.x", "/prefix")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestCollectEntityPaths_ArrayWithMatchAtIndex(t *testing.T) {
	t.Parallel()

	node := []any{"other", "sensor.x"}
	got := collectEntityPaths(node, "sensor.x", "/items")
	want := []string{"/items/1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectEntityPaths_ArrayWithMultipleMatches(t *testing.T) {
	t.Parallel()

	node := []any{"sensor.x", "other", "sensor.x"}
	got := collectEntityPaths(node, "sensor.x", "/ids")
	want := []string{"/ids/0", "/ids/2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectEntityPaths_MapWithEntityIDMatch(t *testing.T) {
	t.Parallel()

	node := map[string]any{
		"entity_id": "sensor.x",
		"platform":  "state",
	}
	got := collectEntityPaths(node, "sensor.x", "/step")
	want := []string{"/step/entity_id"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectEntityPaths_NestedTargetEntityID(t *testing.T) {
	t.Parallel()

	node := map[string]any{
		"action": "automation.turn_off",
		"target": map[string]any{
			"entity_id": "sensor.x",
		},
	}
	got := collectEntityPaths(node, "sensor.x", "/step")
	want := []string{"/step/target/entity_id"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectEntityPaths_EntityIDAsArray(t *testing.T) {
	t.Parallel()

	node := map[string]any{
		"entity_id": []any{"sensor.a", "sensor.x"},
	}
	got := collectEntityPaths(node, "sensor.x", "/step")
	want := []string{"/step/entity_id/1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectEntityPaths_MultipleOccurrences(t *testing.T) {
	t.Parallel()

	// entity appears both in entity_id and target.entity_id
	node := map[string]any{
		"entity_id": "sensor.x",
		"target": map[string]any{
			"entity_id": "sensor.x",
		},
	}
	got := collectEntityPaths(node, "sensor.x", "/step")
	sort.Strings(got)
	want := []string{"/step/entity_id", "/step/target/entity_id"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectEntityPaths_NestedChooseBlock(t *testing.T) {
	t.Parallel()

	// entity inside a choose branch
	node := map[string]any{
		"choose": []any{
			map[string]any{
				"sequence": []any{
					map[string]any{"entity_id": "sensor.x"},
				},
			},
		},
	}
	got := collectEntityPaths(node, "sensor.x", "/step")
	want := []string{"/step/choose/0/sequence/0/entity_id"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectEntityPaths_KeyEscaping(t *testing.T) {
	t.Parallel()

	// Map key containing "/" must be RFC 6901 escaped
	node := map[string]any{
		"a/b": "sensor.x",
	}
	got := collectEntityPaths(node, "sensor.x", "/step")
	want := []string{"/step/a~1b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCollectEntityPaths_NoMatch(t *testing.T) {
	t.Parallel()

	node := map[string]any{
		"entity_id": "sensor.y",
		"platform":  "state",
	}
	got := collectEntityPaths(node, "sensor.x", "/step")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// --- referencePathContext ---

func TestReferencePathContext_ActionWithActionKey(t *testing.T) {
	t.Parallel()

	node := map[string]any{"action": "automation.turn_off", "target": map[string]any{}}
	got := referencePathContext("action", node)
	want := "action: automation.turn_off"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReferencePathContext_ActionWithServiceKey(t *testing.T) {
	t.Parallel()

	node := map[string]any{"service": "light.turn_on"}
	got := referencePathContext("action", node)
	want := "action: light.turn_on"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReferencePathContext_ActionChoose(t *testing.T) {
	t.Parallel()

	node := map[string]any{"choose": []any{}}
	got := referencePathContext("action", node)
	want := "action: choose"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReferencePathContext_ActionIfThen(t *testing.T) {
	t.Parallel()

	node := map[string]any{"if": []any{}}
	got := referencePathContext("action", node)
	want := "action: if/then"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReferencePathContext_ActionBare(t *testing.T) {
	t.Parallel()

	node := map[string]any{"parallel": []any{}}
	got := referencePathContext("action", node)
	want := "action"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReferencePathContext_TriggerWithPlatformKey(t *testing.T) {
	t.Parallel()

	node := map[string]any{"platform": "state"}
	got := referencePathContext("trigger", node)
	want := "trigger: state"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReferencePathContext_TriggerWithTriggerKey(t *testing.T) {
	t.Parallel()

	node := map[string]any{"trigger": "time"}
	got := referencePathContext("trigger", node)
	want := "trigger: time"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReferencePathContext_TriggerBare(t *testing.T) {
	t.Parallel()

	node := map[string]any{"unknown": "something"}
	got := referencePathContext("trigger", node)
	want := "trigger"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReferencePathContext_ConditionWithConditionKey(t *testing.T) {
	t.Parallel()

	node := map[string]any{"condition": "state"}
	got := referencePathContext("condition", node)
	want := "condition: state"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReferencePathContext_ConditionBare(t *testing.T) {
	t.Parallel()

	node := map[string]any{"unknown": "something"}
	got := referencePathContext("condition", node)
	want := "condition"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- collectSectionReferencePaths ---

func TestCollectSectionReferencePaths_SingleActionMatch(t *testing.T) {
	t.Parallel()

	section := []any{
		map[string]any{
			"action": "automation.turn_off",
			"target": map[string]any{"entity_id": "sensor.x"},
		},
	}
	got := collectSectionReferencePaths(section, "sequence", "action", "sensor.x")
	if len(got) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(got), got)
	}
	if got[0].Path != "/sequence/0/target/entity_id" {
		t.Errorf("path = %q, want %q", got[0].Path, "/sequence/0/target/entity_id")
	}
	if got[0].Context != "action: automation.turn_off" {
		t.Errorf("context = %q, want %q", got[0].Context, "action: automation.turn_off")
	}
}

func TestCollectSectionReferencePaths_MultipleStepsMultiplePaths(t *testing.T) {
	t.Parallel()

	section := []any{
		map[string]any{
			"action": "automation.turn_off",
			"target": map[string]any{"entity_id": "sensor.x"},
		},
		map[string]any{
			"action": "automation.turn_on",
			"target": map[string]any{"entity_id": "sensor.x"},
		},
	}
	got := collectSectionReferencePaths(section, "sequence", "action", "sensor.x")
	if len(got) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(got), got)
	}
	if got[0].Path != "/sequence/0/target/entity_id" {
		t.Errorf("path[0] = %q, want %q", got[0].Path, "/sequence/0/target/entity_id")
	}
	if got[1].Path != "/sequence/1/target/entity_id" {
		t.Errorf("path[1] = %q, want %q", got[1].Path, "/sequence/1/target/entity_id")
	}
}

func TestCollectSectionReferencePaths_NoMatchReturnsEmpty(t *testing.T) {
	t.Parallel()

	section := []any{
		map[string]any{"action": "light.turn_on", "entity_id": "light.desk"},
	}
	got := collectSectionReferencePaths(section, "sequence", "action", "sensor.x")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestCollectSectionReferencePaths_TriggerSection(t *testing.T) {
	t.Parallel()

	section := []any{
		map[string]any{"platform": "state", "entity_id": "sensor.x", "to": "on"},
	}
	got := collectSectionReferencePaths(section, "triggers", "trigger", "sensor.x")
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d: %v", len(got), got)
	}
	if got[0].Path != "/triggers/0/entity_id" {
		t.Errorf("path = %q, want /triggers/0/entity_id", got[0].Path)
	}
	if got[0].Context != "trigger: state" {
		t.Errorf("context = %q, want 'trigger: state'", got[0].Context)
	}
}
