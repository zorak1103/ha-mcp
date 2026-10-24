package jsonorder

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// decode parses raw JSON with UseNumber into generic Go values, the same way
// the rest of the codebase builds map[string]any trees.
func decode(t *testing.T, raw string) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	return v
}

func compact(t *testing.T, raw string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(raw)); err != nil {
		t.Fatalf("compact %q: %v", raw, err)
	}
	return buf.String()
}

func TestMarshal_NilOrderMatchesEncodingJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    any
	}{
		{"nil", nil},
		{"empty map", map[string]any{}},
		{"empty slice", []any{}},
		{"nil map", map[string]any(nil)},
		{"nil slice", []any(nil)},
		{"scalars", []any{"a", 1.5, true, nil}},
		{"nested maps and slices", map[string]any{
			"z": []any{map[string]any{"b": 1, "a": 2}, []any{"x", "y"}},
			"a": map[string]any{"y": nil, "x": map[string]any{}},
		}},
		{"html escaping", map[string]any{"k<>&": "<tag>&amp;"}},
		{"unicode", map[string]any{"ключ": "wert äöü ✓  "}},
		{"json.Number", map[string]any{"n": json.Number("1.50"), "m": json.Number("10")}},
		{"typed slice falls through", map[string]any{"s": []string{"b", "a"}, "m": map[string]int{"y": 1, "x": 2}}},
		{"string", "plain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			got, err := Marshal(tt.v, nil)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("Marshal(nil order) = %s, want %s", got, want)
			}
		})
	}
}

func TestMarshal_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    any
	}{
		{"unsupported top-level", make(chan int)},
		{"unsupported map value", map[string]any{"c": make(chan int)}},
		{"unsupported slice element", []any{make(chan int)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Marshal(tt.v, nil); err == nil {
				t.Error("Marshal() error = nil, want error")
			}
		})
	}
}

func TestMarshal_RoundTripProperty(t *testing.T) {
	t.Parallel()

	// Sorted input (what json.Marshal produces): capture then marshal is identity.
	values := []any{
		map[string]any{"b": []any{map[string]any{"y": 1, "x": 2}}, "a": "<&>"},
		map[string]any{},
		[]any{},
		[]any{map[string]any{"k": []any{}}, "s", nil},
	}
	for _, v := range values {
		want, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		o, err := Capture(want)
		if err != nil {
			t.Fatalf("Capture(%s): %v", want, err)
		}
		got, err := Marshal(v, o)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("Marshal(v, Capture(json.Marshal(v))) = %s, want %s", got, want)
		}
	}
}

func TestMarshal_PreservesDocumentOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string // expected output; defaults to the compacted raw document
	}{
		{
			name: "issue 311 variables block",
			raw: `{"alias":"Heizung","variables":{"heizzeit":"{{ now() }}","desired":"{{ heizzeit }}",
				"abc":1},"triggers":[{"trigger":"state","entity_id":"a.b"}]}`,
		},
		{
			name: "object array object",
			raw:  `{"z":[{"y":1,"x":[{"b":1,"a":2}],"w":3},{"d":1,"c":2}],"a":{"q":1,"p":2}}`,
		},
		{
			name: "top-level array",
			raw:  `[{"b":1,"a":2},{"d":[{"z":1,"y":2}],"c":null}]`,
		},
		{
			name: "html and unicode",
			raw:  `{"z<":"<&>","ä":"✓","a":{"y":1,"x":"b"}}`,
			// Same HTML escaping as json.Marshal; order is the document's.
			// "\x5c" is a backslash: the expected output holds literal < escapes.
			want: "{\"z\x5cu003c\":\"\x5cu003c\x5cu0026\x5cu003e\",\"ä\":\"✓\",\"a\":{\"y\":1,\"x\":\"b\"}}",
		},
		{
			name: "numbers",
			raw:  `{"b":1.50,"a":10,"c":[3.0,1e3]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			o, err := Capture([]byte(tt.raw))
			if err != nil {
				t.Fatalf("Capture: %v", err)
			}
			got, err := Marshal(decode(t, tt.raw), o)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			want := tt.want
			if want == "" {
				want = compact(t, tt.raw)
			}
			if string(got) != want {
				t.Errorf("Marshal() =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestMarshal_UnrecordedKeysSortedAfterRecorded(t *testing.T) {
	t.Parallel()

	o := &Order{Keys: []string{"m", "k", "gone", "a", "m"}} // duplicate recorded key is emitted once
	v := map[string]any{"z": 1, "b": 2, "a": 3, "k": 4, "m": 5, "c": 6}
	got, err := Marshal(v, o)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"m":5,"k":4,"a":3,"b":2,"c":6,"z":1}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestMarshal_ShapeMismatchFallsBack(t *testing.T) {
	t.Parallel()

	objectNode := &Order{Keys: []string{"b", "a"}, Fields: map[string]*Order{"x": {Keys: []string{"q"}}}}
	arrayNode := &Order{Items: []*Order{{Keys: []string{"b", "a"}}, nil}}

	tests := []struct {
		name string
		v    any
		o    *Order
		want string
	}{
		{"object node on array value", []any{map[string]any{"a": 1, "b": 2}, "s"}, objectNode, `[{"a":1,"b":2},"s"]`},
		{"array node on object value", map[string]any{"b": 1, "a": []any{1}}, arrayNode, `{"a":[1],"b":1}`},
		{"array node on scalar", "s", arrayNode, `"s"`},
		{"object node on scalar", 1, objectNode, `1`},
		{"field node on scalar value", map[string]any{"x": 5, "b": 1}, objectNode, `{"b":1,"x":5}`},
		{"item node on shorter slice", []any{1}, arrayNode, `[1]`},
		{"item node on longer slice", []any{1, 2, 3}, arrayNode, `[1,2,3]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Marshal(tt.v, tt.o)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCapture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want *Order
	}{
		{"string scalar", `"x"`, nil},
		{"number scalar", `1.5`, nil},
		{"bool scalar", `true`, nil},
		{"null scalar", `null`, nil},
		{"empty object", `{}`, &Order{}},
		{"empty array", `[]`, &Order{}},
		{"flat object", `{"b":1,"a":"x","c":null}`, &Order{Keys: []string{"b", "a", "c"}}},
		{
			name: "nested",
			raw:  `{"b":{"y":1,"x":2},"a":[1,{"q":1,"p":[]},[2]],"s":3}`,
			want: &Order{
				Keys: []string{"b", "a", "s"},
				Fields: map[string]*Order{
					"b": {Keys: []string{"y", "x"}},
					"a": {Items: []*Order{nil, {Keys: []string{"q", "p"}, Fields: map[string]*Order{"p": {}}}, {Items: []*Order{nil}}}},
				},
			},
		},
		{
			name: "duplicate keys: first position, last subtree",
			raw:  `{"a":{"x":1},"b":1,"a":{"y":1,"z":2}}`,
			want: &Order{
				Keys:   []string{"a", "b"},
				Fields: map[string]*Order{"a": {Keys: []string{"y", "z"}}},
			},
		},
		{
			name: "duplicate keys: last value is scalar drops subtree",
			raw:  `{"a":{"x":1},"a":5}`,
			want: &Order{Keys: []string{"a"}},
		},
		{"surrounding whitespace", " \n{\"a\":1}\t", &Order{Keys: []string{"a"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Capture([]byte(tt.raw))
			if err != nil {
				t.Fatalf("Capture: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Capture() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCapture_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{"empty", ``},
		{"garbage", `not json`},
		{"truncated object", `{"a":1`},
		{"truncated array", `[1,2`},
		{"truncated nested", `{"a":[{"b":`},
		{"bad key", `{1:2}`},
		{"trailing data", `{"a":1} {"b":2}`},
		{"trailing garbage", `{"a":1}x`},
		{"missing colon", `{"a" 1}`},
		{"trailing comma", `[1,]`},
		{"mismatched delimiter", `{"a":[1}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Capture([]byte(tt.raw)); err == nil {
				t.Errorf("Capture(%q) error = nil, want error", tt.raw)
			}
		})
	}
}

func TestOrder_NilSafeAccessors(t *testing.T) {
	t.Parallel()

	var nilOrder *Order
	if got := nilOrder.Field("a"); got != nil {
		t.Errorf("nil.Field = %v, want nil", got)
	}
	if got := nilOrder.Item(0); got != nil {
		t.Errorf("nil.Item = %v, want nil", got)
	}
	if got := nilOrder.Clone(); got != nil {
		t.Errorf("nil.Clone = %v, want nil", got)
	}

	child := &Order{Keys: []string{"k"}}
	o := &Order{
		Fields: map[string]*Order{"a": child},
		Items:  []*Order{child, nil},
	}
	if o.Field("a") != child {
		t.Error("Field(a) did not return the child")
	}
	if o.Field("missing") != nil {
		t.Error("Field(missing) != nil")
	}
	if o.Item(0) != child {
		t.Error("Item(0) did not return the child")
	}
	for _, i := range []int{1, 2, -1, 100} {
		if o.Item(i) != nil {
			t.Errorf("Item(%d) != nil", i)
		}
	}
	if (&Order{}).Field("a") != nil || (&Order{}).Item(0) != nil {
		t.Error("empty order accessors must return nil")
	}
}

func TestOrder_CloneIsDeep(t *testing.T) {
	t.Parallel()

	orig := &Order{
		Keys: []string{"a", "b"},
		Fields: map[string]*Order{
			"a": {Keys: []string{"x", "y"}},
			"b": {Items: []*Order{{Keys: []string{"p"}}, nil}},
		},
	}
	want := &Order{
		Keys: []string{"a", "b"},
		Fields: map[string]*Order{
			"a": {Keys: []string{"x", "y"}},
			"b": {Items: []*Order{{Keys: []string{"p"}}, nil}},
		},
	}

	clone := orig.Clone()
	if diff := cmp.Diff(orig, clone); diff != "" {
		t.Fatalf("clone differs from original (-orig +clone):\n%s", diff)
	}

	clone.Keys[0] = "mutated"
	clone.Fields["a"].Keys[0] = "mutated"
	clone.Fields["b"].Items[0].Keys[0] = "mutated"
	clone.Fields["b"].Items[1] = &Order{}
	clone.Fields["new"] = &Order{}

	if diff := cmp.Diff(want, orig); diff != "" {
		t.Errorf("original changed after mutating clone (-want +got):\n%s", diff)
	}

	// Empty-but-non-nil containers keep their nil-ness.
	empty := (&Order{Keys: []string{}, Fields: map[string]*Order{}, Items: []*Order{}}).Clone()
	if empty.Keys == nil || empty.Fields == nil || empty.Items == nil {
		t.Error("Clone turned empty non-nil containers into nil")
	}
	zero := (&Order{}).Clone()
	if zero.Keys != nil || zero.Fields != nil || zero.Items != nil {
		t.Error("Clone turned nil containers into non-nil")
	}
}

func TestOrder_WithField(t *testing.T) {
	t.Parallel()

	sub := &Order{Keys: []string{"q"}}
	other := &Order{Keys: []string{"r"}}

	t.Run("nil receiver creates a new root", func(t *testing.T) {
		t.Parallel()
		var o *Order
		got := o.WithField("a", sub)
		want := &Order{Fields: map[string]*Order{"a": sub}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("nil receiver and nil sub", func(t *testing.T) {
		t.Parallel()
		var o *Order
		got := o.WithField("a", nil)
		if got == nil {
			t.Fatal("WithField on nil receiver returned nil, want a new root")
		}
		if got.Field("a") != nil {
			t.Error("Field(a) != nil")
		}
	})

	t.Run("receiver is left untouched", func(t *testing.T) {
		t.Parallel()
		orig := &Order{
			Keys:   []string{"a", "b"},
			Fields: map[string]*Order{"a": other},
			Items:  []*Order{other},
		}
		want := &Order{
			Keys:   []string{"a", "b"},
			Fields: map[string]*Order{"a": other},
			Items:  []*Order{other},
		}
		got := orig.WithField("a", sub).WithField("c", sub)
		if diff := cmp.Diff(want, orig); diff != "" {
			t.Errorf("receiver changed (-want +got):\n%s", diff)
		}
		if got.Field("a") != sub || got.Field("c") != sub {
			t.Error("result does not carry the new fields")
		}
		if diff := cmp.Diff(want.Keys, got.Keys); diff != "" {
			t.Errorf("Keys changed (-want +got):\n%s", diff)
		}
		if len(got.Items) != 1 || got.Items[0] != other {
			t.Error("Items not carried over")
		}

		// Mutating the result's slices must not reach the receiver.
		got.Keys[0] = "mutated"
		got.Items[0] = nil
		if diff := cmp.Diff(want, orig); diff != "" {
			t.Errorf("receiver changed via result slices (-want +got):\n%s", diff)
		}
	})

	t.Run("nil sub removes the field", func(t *testing.T) {
		t.Parallel()
		orig := &Order{Fields: map[string]*Order{"a": sub, "b": other}}
		got := orig.WithField("a", nil)
		if got.Field("a") != nil || got.Field("b") != other {
			t.Errorf("unexpected fields after removal: %v", got.Fields)
		}
		if orig.Field("a") != sub {
			t.Error("receiver lost its field")
		}
		last := got.WithField("b", nil)
		if last.Fields != nil {
			t.Errorf("Fields = %v, want nil once empty", last.Fields)
		}
	})
}

func TestMarshal_SharedSubtreeUsedTwice(t *testing.T) {
	t.Parallel()

	shared := &Order{Keys: []string{"b", "a"}}
	o := &Order{Items: []*Order{shared, shared}}
	v := []any{map[string]any{"a": 1, "b": 2}, map[string]any{"a": 3, "b": 4}}
	got, err := Marshal(v, o)
	if err != nil {
		t.Fatal(err)
	}
	if want := `[{"b":2,"a":1},{"b":4,"a":3}]`; string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}
