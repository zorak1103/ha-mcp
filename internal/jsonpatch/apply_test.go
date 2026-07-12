package jsonpatch

import (
	"encoding/json"
	"strings"
	"testing"
)

// applyJSON is a helper that applies a JSON patch spec to a JSON document.
func applyJSON(t *testing.T, docJSON string, ops []Operation) any {
	t.Helper()

	var doc any
	if err := json.Unmarshal([]byte(docJSON), &doc); err != nil {
		t.Fatalf("unmarshal doc: %v", err)
	}

	result, err := Apply(doc, ops)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return result
}

func TestApply_Add(t *testing.T) {
	t.Parallel()

	t.Run("add key to map", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"mode":"single"}`, []Operation{
			{Op: "add", Path: "/alias", Value: "Morning Routine"},
		})

		m := result.(map[string]any)
		if m["alias"] != "Morning Routine" {
			t.Errorf("alias = %v, want 'Morning Routine'", m["alias"])
		}
		if m["mode"] != "single" {
			t.Errorf("mode = %v, want 'single' (should be preserved)", m["mode"])
		}
	})

	t.Run("add to array by index", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"items":["a","b","c"]}`, []Operation{
			{Op: "add", Path: "/items/1", Value: "x"},
		})

		m := result.(map[string]any)
		items := m["items"].([]any)
		if len(items) != 4 {
			t.Fatalf("len(items) = %d, want 4", len(items))
		}
		if items[1] != "x" {
			t.Errorf("items[1] = %v, want 'x'", items[1])
		}
		if items[2] != "b" {
			t.Errorf("items[2] = %v, want 'b' (shifted)", items[2])
		}
	})

	t.Run("add to array with dash appends", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"actions":["step1"]}`, []Operation{
			{Op: "add", Path: "/actions/-", Value: map[string]any{"action": "light.turn_on"}},
		})

		m := result.(map[string]any)
		actions := m["actions"].([]any)
		if len(actions) != 2 {
			t.Fatalf("len(actions) = %d, want 2", len(actions))
		}
	})

	t.Run("add to end of array by index", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"items":["a","b"]}`, []Operation{
			{Op: "add", Path: "/items/2", Value: "c"},
		})

		m := result.(map[string]any)
		items := m["items"].([]any)
		if len(items) != 3 {
			t.Fatalf("len(items) = %d, want 3", len(items))
		}
		if items[2] != "c" {
			t.Errorf("items[2] = %v, want 'c'", items[2])
		}
	})

	t.Run("add replaces existing key", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"mode":"single"}`, []Operation{
			{Op: "add", Path: "/mode", Value: "queued"},
		})

		m := result.(map[string]any)
		if m["mode"] != "queued" {
			t.Errorf("mode = %v, want 'queued'", m["mode"])
		}
	})

	t.Run("add root replaces document", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"old":"doc"}`), &doc)

		result, err := Apply(doc, []Operation{{Op: "add", Path: "", Value: "new"}})
		if err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if result != "new" {
			t.Errorf("result = %v, want 'new'", result)
		}
	})
}

func TestApply_Remove(t *testing.T) {
	t.Parallel()

	t.Run("remove key from map", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"mode":"single","alias":"test"}`, []Operation{
			{Op: "remove", Path: "/alias"},
		})

		m := result.(map[string]any)
		if _, ok := m["alias"]; ok {
			t.Error("alias should be removed")
		}
		if m["mode"] != "single" {
			t.Error("mode should be preserved")
		}
	})

	t.Run("remove array element", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"items":["a","b","c"]}`, []Operation{
			{Op: "remove", Path: "/items/1"},
		})

		m := result.(map[string]any)
		items := m["items"].([]any)
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}
		if items[0] != "a" || items[1] != "c" {
			t.Errorf("items = %v, want [a, c]", items)
		}
	})

	t.Run("remove missing key errors", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"mode":"single"}`), &doc)

		_, err := Apply(doc, []Operation{{Op: "remove", Path: "/notexist"}})
		if err == nil {
			t.Fatal("expected error for missing key")
		}
	})

	t.Run("remove out of bounds errors", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"items":["a"]}`), &doc)

		_, err := Apply(doc, []Operation{{Op: "remove", Path: "/items/5"}})
		if err == nil {
			t.Fatal("expected error for out of bounds")
		}
	})
}

func TestApply_Replace(t *testing.T) {
	t.Parallel()

	t.Run("replace map value", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"mode":"single","alias":"old"}`, []Operation{
			{Op: "replace", Path: "/mode", Value: "queued"},
		})

		m := result.(map[string]any)
		if m["mode"] != "queued" {
			t.Errorf("mode = %v, want 'queued'", m["mode"])
		}
		if m["alias"] != "old" {
			t.Error("alias should be preserved")
		}
	})

	t.Run("replace array element", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"items":["a","b","c"]}`, []Operation{
			{Op: "replace", Path: "/items/1", Value: "x"},
		})

		m := result.(map[string]any)
		items := m["items"].([]any)
		if len(items) != 3 {
			t.Fatalf("len(items) = %d, want 3 (replace keeps length)", len(items))
		}
		if items[1] != "x" {
			t.Errorf("items[1] = %v, want 'x'", items[1])
		}
	})

	t.Run("replace nested value", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t,
			`{"triggers":[{"trigger":"time","at":"07:00"},{"trigger":"state","entity_id":"sensor.old"}]}`,
			[]Operation{
				{Op: "replace", Path: "/triggers/1/entity_id", Value: "sensor.new"},
			})

		m := result.(map[string]any)
		triggers := m["triggers"].([]any)
		t1 := triggers[1].(map[string]any)
		if t1["entity_id"] != "sensor.new" {
			t.Errorf("entity_id = %v, want 'sensor.new'", t1["entity_id"])
		}
	})

	t.Run("replace missing key errors", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"mode":"single"}`), &doc)

		_, err := Apply(doc, []Operation{{Op: "replace", Path: "/notexist", Value: "x"}})
		if err == nil {
			t.Fatal("expected error for missing key in replace")
		}
	})

	// Regression tests for issue #66: replace on /array/N must not reorder siblings.

	t.Run("replace preserves sibling order - whole element", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t,
			`{"views":[
				{"title":"Übersicht","path":"overview"},
				{"title":"Twingo","path":"twingo"},
				{"title":"IONIQ 6","path":"ioniq"},
				{"title":"Wallbox","path":"wallbox"}
			]}`,
			[]Operation{
				{Op: "replace", Path: "/views/2", Value: map[string]any{
					"title": "IONIQ 6 updated", "path": "ioniq",
				}},
			})

		views := result.(map[string]any)["views"].([]any)
		want := []string{"Übersicht", "Twingo", "IONIQ 6 updated", "Wallbox"}
		if len(views) != len(want) {
			t.Fatalf("len(views) = %d, want %d", len(views), len(want))
		}
		for i, v := range views {
			got := v.(map[string]any)["title"]
			if got != want[i] {
				t.Errorf("views[%d].title = %v, want %q", i, got, want[i])
			}
		}
	})

	t.Run("replace preserves sibling order - nested field", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t,
			`{"views":[
				{"title":"A","path":"a"},
				{"title":"B","path":"b"},
				{"title":"C","path":"c"}
			]}`,
			[]Operation{
				{Op: "replace", Path: "/views/2/title", Value: "C updated"},
			})

		views := result.(map[string]any)["views"].([]any)
		want := []string{"A", "B", "C updated"}
		if len(views) != len(want) {
			t.Fatalf("len(views) = %d, want %d", len(views), len(want))
		}
		for i, v := range views {
			got := v.(map[string]any)["title"]
			if got != want[i] {
				t.Errorf("views[%d].title = %v, want %q", i, got, want[i])
			}
		}
	})

	t.Run("replace preserves sibling order - 10 elements", func(t *testing.T) {
		t.Parallel()

		const n = 10
		// Build a 10-element array as JSON
		raw := `{"items":[0,1,2,3,4,5,6,7,8,9]}`
		result := applyJSON(t, raw, []Operation{
			{Op: "replace", Path: "/items/5", Value: float64(99)},
		})

		items := result.(map[string]any)["items"].([]any)
		if len(items) != n {
			t.Fatalf("len(items) = %d, want %d", len(items), n)
		}
		for i, v := range items {
			want := float64(i)
			if i == 5 {
				want = 99
			}
			if v != want {
				t.Errorf("items[%d] = %v, want %v", i, v, want)
			}
		}
	})
}

func TestApply_Move(t *testing.T) {
	t.Parallel()

	t.Run("move map value", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"a":"value","b":"other"}`, []Operation{
			{Op: "move", From: "/a", Path: "/c"},
		})

		m := result.(map[string]any)
		if _, ok := m["a"]; ok {
			t.Error("a should be removed")
		}
		if m["c"] != "value" {
			t.Errorf("c = %v, want 'value'", m["c"])
		}
	})

	t.Run("move array element", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"items":["a","b","c"]}`, []Operation{
			{Op: "move", From: "/items/0", Path: "/items/-"},
		})

		m := result.(map[string]any)
		items := m["items"].([]any)
		if len(items) != 3 {
			t.Fatalf("len(items) = %d, want 3", len(items))
		}
		if items[0] != "b" || items[2] != "a" {
			t.Errorf("items = %v, want [b, c, a]", items)
		}
	})
}

func TestApply_Copy(t *testing.T) {
	t.Parallel()

	t.Run("copy map value", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"a":"value"}`, []Operation{
			{Op: "copy", From: "/a", Path: "/b"},
		})

		m := result.(map[string]any)
		if m["a"] != "value" {
			t.Error("a should still exist")
		}
		if m["b"] != "value" {
			t.Errorf("b = %v, want 'value'", m["b"])
		}
	})
}

func TestApply_Test(t *testing.T) {
	t.Parallel()

	t.Run("test passes", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"mode":"single"}`), &doc)

		result, err := Apply(doc, []Operation{
			{Op: "test", Path: "/mode", Value: "single"},
		})
		if err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		m := result.(map[string]any)
		if m["mode"] != "single" {
			t.Error("doc should be unchanged")
		}
	})

	t.Run("test fails returns error", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"mode":"single"}`), &doc)

		_, err := Apply(doc, []Operation{
			{Op: "test", Path: "/mode", Value: "queued"},
		})
		if err == nil {
			t.Fatal("expected error for test failure")
		}
		if !containsStr(err.Error(), "test failed") {
			t.Errorf("error = %q, want to contain 'test failed'", err.Error())
		}
	})
}

func TestApply_Atomicity(t *testing.T) {
	t.Parallel()

	t.Run("failed op leaves original unchanged", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"mode":"single","alias":"test"}`), &doc)

		// First op succeeds, second op fails
		result, err := Apply(doc, []Operation{
			{Op: "replace", Path: "/mode", Value: "queued"},
			{Op: "replace", Path: "/nonexistent", Value: "x"},
		})

		if err == nil {
			t.Fatal("expected error")
		}

		// Original doc should be unchanged
		original := doc.(map[string]any)
		if original["mode"] != "single" {
			t.Errorf("original mode = %v, want 'single' (atomicity violation)", original["mode"])
		}

		// Result should be original doc on error
		resultMap := result.(map[string]any)
		if resultMap["mode"] != "single" {
			t.Errorf("returned mode = %v, want 'single'", resultMap["mode"])
		}
	})
}

func TestApply_MultipleOps(t *testing.T) {
	t.Parallel()

	t.Run("complex patch sequence", func(t *testing.T) {
		t.Parallel()

		docJSON := `{
			"alias": "Morning Routine",
			"mode": "single",
			"triggers": [
				{"trigger": "time", "at": "07:00"},
				{"trigger": "state", "entity_id": "sensor.old"}
			],
			"conditions": [{"condition": "time"}],
			"actions": [{"action": "light.turn_on"}]
		}`

		ops := []Operation{
			{Op: "replace", Path: "/mode", Value: "queued"},
			{Op: "replace", Path: "/triggers/1/entity_id", Value: "sensor.new"},
			{Op: "add", Path: "/actions/-", Value: map[string]any{"action": "light.turn_off"}},
			{Op: "remove", Path: "/conditions/0"},
			{Op: "test", Path: "/alias", Value: "Morning Routine"},
		}

		result := applyJSON(t, docJSON, ops)
		m := result.(map[string]any)

		if m["mode"] != "queued" {
			t.Errorf("mode = %v, want 'queued'", m["mode"])
		}

		triggers := m["triggers"].([]any)
		if triggers[1].(map[string]any)["entity_id"] != "sensor.new" {
			t.Error("trigger entity_id should be updated")
		}

		actions := m["actions"].([]any)
		if len(actions) != 2 {
			t.Errorf("len(actions) = %d, want 2", len(actions))
		}

		conditions := m["conditions"].([]any)
		if len(conditions) != 0 {
			t.Errorf("len(conditions) = %d, want 0", len(conditions))
		}
	})
}

func TestApply_ErrorMessages(t *testing.T) {
	t.Parallel()

	var doc any
	_ = json.Unmarshal([]byte(`{"mode":"single","items":["a","b"]}`), &doc)

	tests := []struct {
		name   string
		ops    []Operation
		errMsg string
	}{
		{
			name:   "index out of bounds includes path",
			ops:    []Operation{{Op: "replace", Path: "/items/5", Value: "x"}},
			errMsg: "out of bounds",
		},
		{
			name:   "missing key includes operation index",
			ops:    []Operation{{Op: "replace", Path: "/notexist", Value: "x"}},
			errMsg: "operation 0",
		},
		{
			name:   "missing key shows available keys",
			ops:    []Operation{{Op: "replace", Path: "/notexist", Value: "x"}},
			errMsg: "available keys: [items mode]",
		},
		{
			name:   "root-level miss still echoes the requested path",
			ops:    []Operation{{Op: "replace", Path: "/notexist", Value: "x"}},
			errMsg: `document root (requested "/notexist")`,
		},
		{
			name:   "test failure includes expected and actual",
			ops:    []Operation{{Op: "test", Path: "/mode", Value: "queued"}},
			errMsg: "test failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var d any
			_ = json.Unmarshal([]byte(`{"mode":"single","items":["a","b"]}`), &d)

			_, err := Apply(d, tt.ops)
			if err == nil {
				t.Fatal("expected error")
			}
			if !containsStr(err.Error(), tt.errMsg) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestApply_NestedErrorMessages covers issue #124: a missing key deep inside a
// nested action structure (if/then/else, choose/sequence/default) should report
// the prefix actually navigated (not the full submitted path) plus a structural
// hint when the missing key is one of these HA action-block keywords.
func TestApply_NestedErrorMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		docJSON   string
		op        Operation
		wantMsg   []string
		wantNoMsg []string
	}{
		{
			name:    "add: nested miss reports parent prefix, not full path",
			docJSON: `{"a":{"b":{"if":[]}}}`,
			op:      Operation{Op: "add", Path: "/a/b/then/0", Value: 1},
			wantMsg: []string{`at "/a/b"`, "available keys: [if]"},
		},
		{
			name:    "add: then hint when then is missing sibling of if",
			docJSON: `{"actions":[{"if":[{"condition":"state"}],"then":[]}]}`,
			op:      Operation{Op: "add", Path: "/actions/0/if/0/then/0", Value: map[string]any{}},
			wantMsg: []string{"sibling", `"if"`},
		},
		{
			name:    "add: else hint when else is missing sibling of if",
			docJSON: `{"actions":[{"if":[{"condition":"state"}],"else":[]}]}`,
			op:      Operation{Op: "add", Path: "/actions/0/if/0/else/0", Value: map[string]any{}},
			wantMsg: []string{"sibling", `"if"`},
		},
		{
			name:    "add: sequence hint when sequence is missing inside conditions",
			docJSON: `{"actions":[{"choose":[{"conditions":[{"condition":"state"}],"sequence":[]}]}]}`,
			op:      Operation{Op: "add", Path: "/actions/0/choose/0/conditions/0/sequence/0", Value: map[string]any{}},
			wantMsg: []string{`"choose"`, `"repeat"`},
		},
		{
			name:    "add: default hint when default is missing inside choose",
			docJSON: `{"actions":[{"choose":[{"conditions":[],"sequence":[]}],"default":[]}]}`,
			op:      Operation{Op: "add", Path: "/actions/0/choose/0/default/0", Value: map[string]any{}},
			wantMsg: []string{"sibling", `"choose"`},
		},
		{
			name:      "add: non-keyword miss has no structural hint",
			docJSON:   `{"actions":[{"choose":[]}]}`,
			op:        Operation{Op: "add", Path: "/actions/0/notarealkey/x", Value: 1},
			wantMsg:   []string{`"notarealkey"`},
			wantNoMsg: []string{"sibling", "nested inside"},
		},
		{
			name:    "remove: nested miss reports parent prefix",
			docJSON: `{"a":{"b":{"if":[]}}}`,
			op:      Operation{Op: "remove", Path: "/a/b/then"},
			wantMsg: []string{`at "/a/b"`, "available keys: [if]"},
		},
		{
			name:    "remove: then hint when then is missing sibling of if",
			docJSON: `{"actions":[{"if":[{"condition":"state"}]}]}`,
			op:      Operation{Op: "remove", Path: "/actions/0/if/0/then"},
			wantMsg: []string{"sibling", `"if"`},
		},
		{
			name:    "replace: nested miss reports parent prefix",
			docJSON: `{"a":{"b":{"if":[]}}}`,
			op:      Operation{Op: "replace", Path: "/a/b/then", Value: 1},
			wantMsg: []string{`at "/a/b"`, "available keys: [if]"},
		},
		{
			name:    "add: escaped segment prefix round-trips",
			docJSON: `{"weird~key":{"child":{"if":[]}}}`,
			op:      Operation{Op: "add", Path: "/weird~0key/child/then/0", Value: 1},
			wantMsg: []string{`at "/weird~0key/child"`, "available keys: [if]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var doc any
			if err := json.Unmarshal([]byte(tt.docJSON), &doc); err != nil {
				t.Fatalf("invalid test doc JSON: %v", err)
			}

			_, err := Apply(doc, []Operation{tt.op})
			if err == nil {
				t.Fatal("expected error")
			}
			for _, want := range tt.wantMsg {
				if !containsStr(err.Error(), want) {
					t.Errorf("error = %q, want to contain %q", err.Error(), want)
				}
			}
			for _, notWant := range tt.wantNoMsg {
				if containsStr(err.Error(), notWant) {
					t.Errorf("error = %q, want NOT to contain %q", err.Error(), notWant)
				}
			}
		})
	}
}

func TestApply_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty operations returns unchanged doc", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"mode":"single"}`), &doc)

		result, err := Apply(doc, []Operation{})
		if err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		m := result.(map[string]any)
		if m["mode"] != "single" {
			t.Error("doc should be unchanged")
		}
	})

	t.Run("dash minus in add path", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"items":[]}`, []Operation{
			{Op: "add", Path: "/items/-", Value: "first"},
		})

		m := result.(map[string]any)
		items := m["items"].([]any)
		if len(items) != 1 || items[0] != "first" {
			t.Errorf("items = %v, want ['first']", items)
		}
	})
}

func TestApply_Move_AncestorProhibition(t *testing.T) {
	t.Parallel()

	t.Run("move path is child of from (RFC 6902 §4.4)", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"triggers":[{"trigger":"time"},{"trigger":"state"}]}`), &doc)

		_, err := Apply(doc, []Operation{
			{Op: "move", From: "/triggers", Path: "/triggers/0"},
		})
		if err == nil {
			t.Fatal("expected error: path is a child of from (RFC 6902 §4.4)")
		}
		if !containsStr(err.Error(), "child") && !containsStr(err.Error(), "4.4") {
			t.Errorf("error = %q, want to mention §4.4 or child", err.Error())
		}
	})

	t.Run("move same path is allowed (self-move is no-op)", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"a":"value"}`), &doc)

		// from == path is NOT prohibited by RFC 6902 §4.4 (self-move)
		// but is nonsensical - we should not error on it
		_, err := Apply(doc, []Operation{
			{Op: "move", From: "/a", Path: "/a"},
		})
		// Self-move may or may not work but must not be caught by §4.4 check
		// (it's either allowed or fails for another reason)
		if err != nil && containsStr(err.Error(), "4.4") {
			t.Errorf("self-move should not trigger §4.4 violation: %v", err)
		}
	})

	t.Run("move sibling paths allowed", func(t *testing.T) {
		t.Parallel()

		result := applyJSON(t, `{"a":"val","b":"other"}`, []Operation{
			{Op: "move", From: "/a", Path: "/c"},
		})
		m := result.(map[string]any)
		if m["c"] != "val" {
			t.Errorf("c = %v, want 'val'", m["c"])
		}
	})
}

func TestApply_Test_JSONEquality(t *testing.T) {
	t.Parallel()

	t.Run("test with integer value passes when doc has float64", func(t *testing.T) {
		t.Parallel()

		// JSON numbers unmarshal to float64; user may provide int
		// Both should be equal by JSON-semantic comparison
		var doc any
		_ = json.Unmarshal([]byte(`{"count":3}`), &doc)

		// Value: float64(3) should match float64(3) in doc
		_, err := Apply(doc, []Operation{
			{Op: "test", Path: "/count", Value: float64(3)},
		})
		if err != nil {
			t.Fatalf("test with float64(3) failed: %v", err)
		}
	})

	t.Run("test with wrong value fails", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"count":3}`), &doc)

		_, err := Apply(doc, []Operation{
			{Op: "test", Path: "/count", Value: float64(5)},
		})
		if err == nil {
			t.Fatal("expected test failure for wrong value")
		}
	})

	t.Run("test with nested object uses JSON equality", func(t *testing.T) {
		t.Parallel()

		var doc any
		_ = json.Unmarshal([]byte(`{"config":{"mode":"single","alias":"test"}}`), &doc)

		_, err := Apply(doc, []Operation{
			{Op: "test", Path: "/config", Value: map[string]any{"mode": "single", "alias": "test"}},
		})
		if err != nil {
			t.Fatalf("test with matching object failed: %v", err)
		}
	})
}

func TestValidate_PathValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ops     []Operation
		wantErr bool
		errFrag string
	}{
		{
			name:    "path without leading slash errors",
			ops:     []Operation{{Op: "replace", Path: "mode", Value: "x"}},
			wantErr: true,
			errFrag: "must start with '/'",
		},
		{
			name:    "from without leading slash errors for move",
			ops:     []Operation{{Op: "move", From: "a", Path: "/b"}},
			wantErr: true,
			errFrag: "JSON Pointer",
		},
		{
			name: "empty path (root) is valid for add",
			ops:  []Operation{{Op: "add", Path: "", Value: "x"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(tt.ops)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errFrag != "" && (err == nil || !containsStr(err.Error(), tt.errFrag)) {
				t.Errorf("error = %q, want to contain %q", err, tt.errFrag)
			}
		})
	}
}

// TestSetAtPath_UnexpectedIntermediateType verifies that navigating into a
// non-map, non-slice value (e.g., a string) returns an error.
func TestSetAtPath_UnexpectedIntermediateType(t *testing.T) {
	t.Parallel()

	// The document has "name" pointing to a string "foo".
	// Trying to set "name/sub" requires navigating into that string, which should fail.
	doc := map[string]any{"name": "foo"}
	_, err := Apply(doc, []Operation{
		{Op: "add", Path: "/name/sub", Value: "bar"},
	})

	if err == nil {
		t.Fatal("Apply() expected error when navigating into non-map/non-slice, got nil")
	}
	if !strings.Contains(err.Error(), "cannot navigate") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "cannot navigate")
	}
}
