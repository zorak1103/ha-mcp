package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/jsonpatch"
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

	resultMap, _, applyErr := applyPatchWithSemantics(doc, ops)
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
	result, _, err := applyPatchWithSemantics(doc, ops)
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

// dryRunPatchResult tests (issue #142: dry_run must return a compact diff,
// not the entire patched config)

func TestDryRunPatchResult_CompactDiff(t *testing.T) {
	t.Parallel()

	original := map[string]any{
		"mode":              "single",
		"unrelated_field":   "should-not-appear-in-diff-output",
		"another_unrelated": map[string]any{"nested": "also-should-not-appear"},
		"triggers":          []any{map[string]any{"platform": "time", "at": "07:00"}},
	}
	resolved := []jsonpatch.Operation{
		{Op: "replace", Path: "/mode", Value: "queued"},
	}

	result, err := dryRunPatchResult(original, resolved, "automation", "morning_routine", len(resolved))
	if err != nil {
		t.Fatalf("dryRunPatchResult() error = %v", err)
	}
	text := result.Content[0].Text

	if !strings.Contains(text, "/mode") {
		t.Errorf("output missing affected path /mode: %s", text)
	}
	if !strings.Contains(text, "single") {
		t.Errorf("output missing before value 'single': %s", text)
	}
	if !strings.Contains(text, "queued") {
		t.Errorf("output missing after value 'queued': %s", text)
	}
	if strings.Contains(text, "should-not-appear-in-diff-output") {
		t.Errorf("output leaked untouched field value: %s", text)
	}
	if strings.Contains(text, "also-should-not-appear") {
		t.Errorf("output leaked untouched nested field value: %s", text)
	}
}

func TestDryRunPatchResult_RemoveShowsRemoved(t *testing.T) {
	t.Parallel()

	original := map[string]any{
		"triggers": []any{map[string]any{"platform": "state", "entity_id": "binary_sensor.door"}},
	}
	resolved := []jsonpatch.Operation{
		{Op: "remove", Path: "/triggers/0"},
	}

	result, err := dryRunPatchResult(original, resolved, "automation", "test_id", len(resolved))
	if err != nil {
		t.Fatalf("dryRunPatchResult() error = %v", err)
	}
	text := result.Content[0].Text

	if !strings.Contains(text, "(removed)") {
		t.Errorf("output missing '(removed)' marker: %s", text)
	}
	if !strings.Contains(text, "binary_sensor.door") {
		t.Errorf("output missing removed element's before value: %s", text)
	}
}

func TestDryRunPatchResult_TruncatesLongValues(t *testing.T) {
	t.Parallel()

	longValue := strings.Repeat("x", 1000)
	original := map[string]any{"field": "short"}
	resolved := []jsonpatch.Operation{
		{Op: "replace", Path: "/field", Value: longValue},
	}

	result, err := dryRunPatchResult(original, resolved, "script", "test_id", len(resolved))
	if err != nil {
		t.Fatalf("dryRunPatchResult() error = %v", err)
	}
	text := result.Content[0].Text

	if strings.Contains(text, longValue) {
		t.Errorf("output contains untruncated 1000-char value")
	}
	if !strings.Contains(text, "…") {
		t.Errorf("output missing truncation marker: %s", text)
	}
}

// TestDryRunPatchResult_TruncatesAtRuneBoundary verifies truncation never cuts
// a multi-byte UTF-8 rune in half (N1) — HA entity/friendly names in this
// project's primarily German-locale deployments commonly contain umlauts.
func TestDryRunPatchResult_TruncatesAtRuneBoundary(t *testing.T) {
	t.Parallel()

	// The marshaled JSON is `"` + 198 x's + ü's + `"`; dryRunValueTruncateLen
	// (200) bytes in lands on the leading byte of the first "ü" (a 2-byte
	// UTF-8 rune), so a naive byte-slice truncation splits it in half.
	longValue := strings.Repeat("x", 198) + strings.Repeat("ü", 50)
	original := map[string]any{"field": "short"}
	resolved := []jsonpatch.Operation{
		{Op: "replace", Path: "/field", Value: longValue},
	}

	result, err := dryRunPatchResult(original, resolved, "script", "test_id", len(resolved))
	if err != nil {
		t.Fatalf("dryRunPatchResult() error = %v", err)
	}
	text := result.Content[0].Text

	if !utf8.ValidString(text) {
		t.Errorf("output contains invalid UTF-8 (rune cut in half): %q", text)
	}
	if strings.Contains(text, longValue) {
		t.Errorf("output contains untruncated value")
	}
	if !strings.Contains(text, "…") {
		t.Errorf("output missing truncation marker: %s", text)
	}
}

func TestDryRunPatchResult_LargeConfigStaysCompact(t *testing.T) {
	t.Parallel()

	// Simulate a large dashboard: many views, each with padding content
	// unrelated to the patch (issue #142 reproduction).
	views := make([]any, 200)
	for i := range views {
		views[i] = map[string]any{
			"title": "padding",
			"cards": []any{
				map[string]any{"type": "entities", "entities": strings.Repeat("e", 500)},
			},
		}
	}
	original := map[string]any{"views": views}

	fullConfigSize := len(mustMarshal(t, original))

	resolved := []jsonpatch.Operation{
		{Op: "replace", Path: "/views/0/title", Value: "updated"},
	}

	result, err := dryRunPatchResult(original, resolved, "dashboard", "lovelace", len(resolved))
	if err != nil {
		t.Fatalf("dryRunPatchResult() error = %v", err)
	}
	text := result.Content[0].Text

	if len(text) >= fullConfigSize {
		t.Errorf("dry-run diff (%d bytes) is not smaller than the full config (%d bytes)", len(text), fullConfigSize)
	}
	if len(text) > 1000 {
		t.Errorf("dry-run diff for a single-field change is unexpectedly large: %d bytes", len(text))
	}
}

// TestDryRunPatchResult_MultiOpSequential verifies before/after values reflect
// true sequential application: when two ops touch the same path, the second
// op's "before" must equal the first op's "after" — not the pristine
// pre-patch snapshot (W2: a snapshot-based diff misrepresents chained ops).
func TestDryRunPatchResult_MultiOpSequential(t *testing.T) {
	t.Parallel()

	original := map[string]any{"mode": "single"}
	resolved := []jsonpatch.Operation{
		{Op: "replace", Path: "/mode", Value: "restart"},
		{Op: "replace", Path: "/mode", Value: "queued"},
	}

	result, err := dryRunPatchResult(original, resolved, "automation", "morning_routine", len(resolved))
	if err != nil {
		t.Fatalf("dryRunPatchResult() error = %v", err)
	}
	text := result.Content[0].Text

	const op1Block = `1. replace /mode
   before: "single"
   after:  "restart"`
	const op2Block = `2. replace /mode
   before: "restart"
   after:  "queued"`
	if !strings.Contains(text, op1Block) {
		t.Errorf("op 1's before value should be the pristine pre-patch value (\"single\"), got:\n%s", text)
	}
	if !strings.Contains(text, op2Block) {
		t.Errorf("op 2's before value should be op 1's after value (\"restart\"), not the pristine snapshot, got:\n%s", text)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return b
}
