package handlers

import (
	"encoding/json"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestParseOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		wantNilOps   bool
		wantErrFrag  string
		wantOpsCount int
	}{
		{
			name:        "missing operations",
			args:        map[string]any{},
			wantNilOps:  true,
			wantErrFrag: "operations is required",
		},
		{
			name:        "operations not array",
			args:        map[string]any{"operations": "notarray"},
			wantNilOps:  true,
			wantErrFrag: "operations must be a JSON array",
		},
		{
			name:        "empty operations",
			args:        map[string]any{"operations": []any{}},
			wantNilOps:  true,
			wantErrFrag: "must contain at least one",
		},
		{
			name: "valid single op",
			args: map[string]any{
				"operations": []any{
					map[string]any{"op": "replace", "path": "/mode", "value": "queued"},
				},
			},
			wantOpsCount: 1,
		},
		{
			name: "valid multiple ops",
			args: map[string]any{
				"operations": []any{
					map[string]any{"op": "replace", "path": "/mode", "value": "queued"},
					map[string]any{"op": "add", "path": "/actions/-", "value": map[string]any{"action": "light.turn_on"}},
					map[string]any{"op": "remove", "path": "/conditions/0"},
				},
			},
			wantOpsCount: 3,
		},
		{
			name: "operation not a map",
			args: map[string]any{
				"operations": []any{"not-a-map"},
			},
			wantNilOps:  true,
			wantErrFrag: "must be an object",
		},
		{
			name: "invalid op value",
			args: map[string]any{
				"operations": []any{
					map[string]any{"op": "invalid_op", "path": "/mode"},
				},
			},
			wantNilOps:  true,
			wantErrFrag: "invalid operation",
		},
		{
			name: "move op requires from",
			args: map[string]any{
				"operations": []any{
					map[string]any{"op": "move", "path": "/b"},
				},
			},
			wantNilOps:  true,
			wantErrFrag: "from is required",
		},
		{
			name: "value key present with nil value",
			args: map[string]any{
				"operations": []any{
					map[string]any{"op": "replace", "path": "/mode", "value": nil},
				},
			},
			wantOpsCount: 1, // nil is a valid JSON value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ops, errResult := parseOperations(tt.args)

			if tt.wantNilOps {
				if ops != nil {
					t.Errorf("expected nil ops, got %v", ops)
				}
				if errResult == nil {
					t.Fatal("expected error result, got nil")
				}
				if len(errResult.Content) == 0 {
					t.Fatal("error result has no content")
				}
				content := errResult.Content[0].Text
				if tt.wantErrFrag != "" && !containsStrHandlers(content, tt.wantErrFrag) {
					t.Errorf("error content = %q, want to contain %q", content, tt.wantErrFrag)
				}
			} else {
				if errResult != nil {
					t.Fatalf("unexpected error result: %v", errResult.Content[0].Text)
				}
				if len(ops) != tt.wantOpsCount {
					t.Errorf("len(ops) = %d, want %d", len(ops), tt.wantOpsCount)
				}
			}
		})
	}
}

func containsStrHandlers(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestParseOperations_ValuePreservation(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"operations": []any{
			map[string]any{"op": "replace", "path": "/mode", "value": "queued"},
		},
	}

	ops, errResult := parseOperations(args)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Content[0].Text)
	}
	if ops[0].Value != "queued" {
		t.Errorf("op.Value = %v, want 'queued'", ops[0].Value)
	}
}

func TestConfigToMap(t *testing.T) {
	t.Parallel()

	config := homeassistant.AutomationConfig{
		ID:          "morning_routine",
		Alias:       "Morning Routine",
		Description: "Turn on lights",
		Mode:        "parallel",
		Max:         5,
		Triggers:    []any{map[string]any{"trigger": "time"}},
	}

	m, err := configToMap(config)
	if err != nil {
		t.Fatalf("configToMap() error = %v", err)
	}

	if m["id"] != "morning_routine" {
		t.Errorf("id = %v, want 'morning_routine'", m["id"])
	}
	if m["alias"] != "Morning Routine" {
		t.Errorf("alias = %v, want 'Morning Routine'", m["alias"])
	}
	if m["max"] != float64(5) {
		t.Errorf("max = %v, want float64(5)", m["max"])
	}
	if m["mode"] != "parallel" {
		t.Errorf("mode = %v, want 'parallel'", m["mode"])
	}
}

func TestMapToStruct(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"id":    "morning_routine",
		"alias": "Morning Routine",
		"mode":  "queued",
		"max":   float64(7),
	}

	var config homeassistant.AutomationConfig
	if err := mapToStruct(m, &config); err != nil {
		t.Fatalf("mapToStruct() error = %v", err)
	}

	if config.ID != "morning_routine" {
		t.Errorf("ID = %v, want 'morning_routine'", config.ID)
	}
	if config.Alias != "Morning Routine" {
		t.Errorf("Alias = %v, want 'Morning Routine'", config.Alias)
	}
	if config.Mode != "queued" {
		t.Errorf("Mode = %v, want 'queued'", config.Mode)
	}
	if config.Max != 7 {
		t.Errorf("Max = %d, want 7", config.Max)
	}
}

func TestPatchOperationsSchema(t *testing.T) {
	t.Parallel()

	schema := patchOperationsSchema()

	if schema.Type != "array" {
		t.Errorf("schema.Type = %q, want 'array'", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("schema.Items is nil")
	}
	if schema.Items.Type != "object" {
		t.Errorf("schema.Items.Type = %q, want 'object'", schema.Items.Type)
	}

	// Check op enum
	opSchema, ok := schema.Items.Properties["op"]
	if !ok {
		t.Fatal("schema.Items.Properties missing 'op'")
	}
	expectedOps := []string{"add", "remove", "replace", "move", "copy", "test"}
	if len(opSchema.Enum) != len(expectedOps) {
		t.Errorf("op enum count = %d, want %d", len(opSchema.Enum), len(expectedOps))
	}

	// value intentionally has no type (accepts any JSON value)
	valueSchema, ok := schema.Items.Properties["value"]
	if !ok {
		t.Fatal("schema.Items.Properties missing 'value'")
	}
	if valueSchema.Type != "" {
		t.Errorf("value.Type = %q, want empty (any type)", valueSchema.Type)
	}
}

func TestParseOneOperation_FromField(t *testing.T) {
	t.Parallel()

	raw := map[string]any{
		"op":   "move",
		"from": "/a",
		"path": "/b",
	}

	op, err := parseOneOperation(raw, 0)
	if err != nil {
		t.Fatalf("parseOneOperation() error = %v", err)
	}
	if op.From != "/a" {
		t.Errorf("op.From = %q, want '/a'", op.From)
	}
	if op.Path != "/b" {
		t.Errorf("op.Path = %q, want '/b'", op.Path)
	}
}

func TestPatchActionConstant(t *testing.T) {
	t.Parallel()

	if patchAction != "patch" {
		t.Errorf("patchAction = %q, want 'patch'", patchAction)
	}
}

// Ensure the operations schema produces a valid jsonpatch.Operation via parseOperations.
func TestParseOperations_RoundTrip(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"operations": []any{
			map[string]any{
				"op":    "add",
				"path":  "/actions/-",
				"value": map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": "light.kitchen"}},
			},
			map[string]any{
				"op":   "remove",
				"path": "/conditions/0",
			},
			map[string]any{
				"op":    "test",
				"path":  "/mode",
				"value": "single",
			},
		},
	}

	ops, errResult := parseOperations(args)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Content[0].Text)
	}
	if len(ops) != 3 {
		t.Fatalf("len(ops) = %d, want 3", len(ops))
	}
	if ops[0].Op != "add" || ops[0].Path != "/actions/-" {
		t.Errorf("op[0] unexpected: %+v", ops[0])
	}
	if ops[1].Op != "remove" || ops[1].Path != "/conditions/0" {
		t.Errorf("op[1] unexpected: %+v", ops[1])
	}

	// Verify these ops would apply successfully to a doc
	doc := map[string]any{
		"mode":       "single",
		"actions":    []any{},
		"conditions": []any{map[string]any{"condition": "time"}},
	}

	resultMap, applyErr := applyPatchWithSemantics(doc, ops)
	if applyErr != nil {
		t.Fatalf("applyPatchWithSemantics() error = %v", applyErr)
	}
	actions := resultMap["actions"].([]any)
	if len(actions) != 1 {
		t.Errorf("len(actions) = %d, want 1", len(actions))
	}
	conditions := resultMap["conditions"].([]any)
	if len(conditions) != 0 {
		t.Errorf("len(conditions) = %d, want 0", len(conditions))
	}
}

// TestParseOperations_AlternativeInputTypes verifies that parseOperations accepts
// operations encoded as json.RawMessage or []map[string]any in addition to []any.
// Regression test for issue #75: MCP clients that pre-encode arguments may deliver
// the operations array as json.RawMessage, causing a hard type-assertion failure.
func TestParseOperations_AlternativeInputTypes(t *testing.T) {
	t.Parallel()

	validOp := map[string]any{"op": "replace", "path": "/mode", "value": "queued"}

	tests := []struct {
		name         string
		operations   any
		wantOpsCount int
		wantErrFrag  string
	}{
		{
			name:         "json.RawMessage array",
			operations:   json.RawMessage(`[{"op":"replace","path":"/mode","value":"queued"}]`),
			wantOpsCount: 1,
		},
		{
			name:         "[]map[string]any",
			operations:   []map[string]any{{"op": "replace", "path": "/mode", "value": "queued"}},
			wantOpsCount: 1,
		},
		{
			name:         "[]any (existing happy path still works)",
			operations:   []any{validOp},
			wantOpsCount: 1,
		},
		{
			name:        "int - invalid type reports Go type name",
			operations:  42,
			wantErrFrag: "int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ops, errResult := parseOperations(map[string]any{"operations": tt.operations})

			if tt.wantErrFrag != "" {
				if errResult == nil {
					t.Fatal("expected error result, got nil")
				}
				if !containsStrHandlers(errResult.Content[0].Text, tt.wantErrFrag) {
					t.Errorf("error %q does not contain %q", errResult.Content[0].Text, tt.wantErrFrag)
				}
				return
			}

			if errResult != nil {
				t.Fatalf("unexpected error: %s", errResult.Content[0].Text)
			}
			if len(ops) != tt.wantOpsCount {
				t.Errorf("len(ops) = %d, want %d", len(ops), tt.wantOpsCount)
			}
			if len(ops) > 0 && ops[0].Op != "replace" {
				t.Errorf("ops[0].Op = %q, want 'replace'", ops[0].Op)
			}
		})
	}
}

// TestParseOperations_DeepValueObject verifies that a deeply-nested value (e.g. a
// full choose block) passes through the parser and round-trips to the engine unchanged.
// Regression test for issue #75: deep value objects were the reported payload.
func TestParseOperations_DeepValueObject(t *testing.T) {
	t.Parallel()

	chooseBlock := map[string]any{
		"choose": []any{
			map[string]any{
				"conditions": []any{
					map[string]any{"condition": "state", "entity_id": "input_text.ev_vehicle", "state": "Unbekannt"},
				},
				"sequence": []any{
					map[string]any{"action": "script.ioniq6_force_refresh"},
					map[string]any{"variables": map[string]any{"detected": "{{ 'Ioniq 6' }}"}},
				},
			},
		},
	}

	args := map[string]any{
		"operations": []any{
			map[string]any{"op": "add", "path": "/actions/1", "value": chooseBlock},
		},
	}

	ops, errResult := parseOperations(args)
	if errResult != nil {
		t.Fatalf("unexpected error parsing deep value: %s", errResult.Content[0].Text)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}

	// Verify the value round-trips through the engine.
	doc := map[string]any{
		"actions": []any{
			map[string]any{"action": "light.turn_on"},
			map[string]any{"action": "light.turn_off"},
		},
	}
	result, err := applyPatchWithSemantics(doc, ops)
	if err != nil {
		t.Fatalf("applyPatchWithSemantics error: %v", err)
	}
	actions, _ := result["actions"].([]any)
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions after insert, got %d", len(actions))
	}
	inserted, _ := actions[1].(map[string]any)
	if _, ok := inserted["choose"]; !ok {
		t.Errorf("inserted action missing 'choose' key; got: %v", inserted)
	}
}
