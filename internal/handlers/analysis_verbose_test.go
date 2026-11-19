package handlers

import "testing"

func TestSummarizeTriggerNode_State(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		node     map[string]any
		entityID string
		want     string
	}{
		{
			name:     "string to",
			node:     map[string]any{"trigger": "state", "entity_id": "binary_sensor.a", "to": "on"},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a → "on"`,
		},
		{
			name:     "list to",
			node:     map[string]any{"trigger": "state", "entity_id": "binary_sensor.a", "to": []any{"on", "unavailable"}},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a → "on" or "unavailable"`,
		},
		{
			name:     "list from, string to",
			node:     map[string]any{"trigger": "state", "entity_id": "binary_sensor.a", "from": []any{"off", "unknown"}, "to": "on"},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a from "off" or "unknown" → "on"`,
		},
		{
			name:     "single-item list to with id",
			node:     map[string]any{"trigger": "state", "entity_id": "binary_sensor.a", "to": []any{"on"}, "id": "t1"},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a → "on" (id: "t1")`,
		},
		{
			name:     "list entity_id falls back to analyzed entity",
			node:     map[string]any{"trigger": "state", "entity_id": []any{"binary_sensor.a", "binary_sensor.b"}, "to": "on"},
			entityID: "binary_sensor.b",
			want:     `state binary_sensor.b → "on"`,
		},
		{
			name:     "empty list to reads as absent",
			node:     map[string]any{"trigger": "state", "entity_id": "binary_sensor.a", "to": []any{}},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a`,
		},
		{
			// Known simplification: HA reads to: null as "state changes only, not
			// attribute changes"; the excerpt does not distinguish it from an absent to.
			name:     "null to reads as absent",
			node:     map[string]any{"trigger": "state", "entity_id": "binary_sensor.a", "to": nil},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a`,
		},
		{
			name:     "legacy platform key with list to",
			node:     map[string]any{"platform": "state", "entity_id": "binary_sensor.a", "to": []any{"on", "off"}},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a → "on" or "off"`,
		},
		{
			name:     "mixed list to keeps string items",
			node:     map[string]any{"trigger": "state", "entity_id": "binary_sensor.a", "to": []any{"on", 5}},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a → "on"`,
		},
		{
			name:     "all non-string list to reads as absent",
			node:     map[string]any{"trigger": "state", "entity_id": "binary_sensor.a", "to": []any{1, 2}},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a`,
		},
		{
			// Deliberate: an empty string counts as absent.
			name:     "empty string from reads as absent",
			node:     map[string]any{"trigger": "state", "entity_id": "binary_sensor.a", "from": "", "to": "on"},
			entityID: "binary_sensor.a",
			want:     `state binary_sensor.a → "on"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := summarizeTriggerNode(tt.node, tt.entityID); got != tt.want {
				t.Errorf("summarizeTriggerNode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummarizeConditionNode_State(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node map[string]any
		want string
	}{
		{
			name: "string state",
			node: map[string]any{"condition": "state", "entity_id": "binary_sensor.b", "state": "on"},
			want: `state binary_sensor.b = "on"`,
		},
		{
			name: "list entity_id and list state",
			node: map[string]any{"condition": "state", "entity_id": []any{"binary_sensor.b", "binary_sensor.c"}, "state": []any{"on", "open"}},
			want: `state binary_sensor.b = "on" or "open"`,
		},
		{
			name: "empty list state reads as absent",
			node: map[string]any{"condition": "state", "entity_id": "binary_sensor.b", "state": []any{}},
			want: `state binary_sensor.b`,
		},
		{
			name: "single-item list state",
			node: map[string]any{"condition": "state", "entity_id": "binary_sensor.b", "state": []any{"on"}},
			want: `state binary_sensor.b = "on"`,
		},
		{
			name: "mixed list state keeps string items",
			node: map[string]any{"condition": "state", "entity_id": "binary_sensor.b", "state": []any{"on", 5}},
			want: `state binary_sensor.b = "on"`,
		},
		{
			name: "all non-string list state reads as absent",
			node: map[string]any{"condition": "state", "entity_id": "binary_sensor.b", "state": []any{1, 2}},
			want: `state binary_sensor.b`,
		},
		{
			name: "null state reads as absent",
			node: map[string]any{"condition": "state", "entity_id": "binary_sensor.b", "state": nil},
			want: `state binary_sensor.b`,
		},
		{
			// Deliberate: an empty string counts as absent, matching get_entity_dependencies.
			name: "empty string state reads as absent",
			node: map[string]any{"condition": "state", "entity_id": "binary_sensor.b", "state": ""},
			want: `state binary_sensor.b`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := summarizeConditionNode(tt.node, "binary_sensor.b"); got != tt.want {
				t.Errorf("summarizeConditionNode() = %q, want %q", got, tt.want)
			}
		})
	}
}
