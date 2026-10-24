package jsonpatch

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zorak1103/ha-mcp/internal/jsonorder"
)

// orderedOp describes an operation whose value is given as raw JSON, so the
// test can derive both Value and ValueOrder from the same text.
type orderedOp struct {
	op, path, from, value string
}

// mustCapture captures the key order of raw JSON or fails the test.
func mustCapture(t *testing.T, raw string) *jsonorder.Order {
	t.Helper()
	o, err := jsonorder.Capture([]byte(raw))
	if err != nil {
		t.Fatalf("Capture(%s): %v", raw, err)
	}
	return o
}

// mustDecode decodes raw JSON into a generic value or fails the test.
func mustDecode(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("Unmarshal(%s): %v", raw, err)
	}
	return v
}

// buildOps turns orderedOps into Operations carrying Value and ValueOrder.
func buildOps(t *testing.T, specs []orderedOp) []Operation {
	t.Helper()
	ops := make([]Operation, 0, len(specs))
	for _, s := range specs {
		op := Operation{Op: s.op, Path: s.path, From: s.from}
		if s.value != "" {
			op.Value = mustDecode(t, s.value)
			op.ValueOrder = mustCapture(t, s.value)
		}
		ops = append(ops, op)
	}
	return ops
}

// applyOrdered runs ApplyWithOrder on docJSON (order captured from it unless
// noOrder) and returns the order-aware encoding of the result.
func applyOrdered(t *testing.T, docJSON string, noOrder bool, ops []Operation) string {
	t.Helper()
	var order *jsonorder.Order
	if !noOrder {
		order = mustCapture(t, docJSON)
	}
	result, gotOrder, err := ApplyWithOrder(mustDecode(t, docJSON), order, ops)
	if err != nil {
		t.Fatalf("ApplyWithOrder() error = %v", err)
	}
	out, err := jsonorder.Marshal(result, gotOrder)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return string(out)
}

func TestApplyWithOrder(t *testing.T) {
	t.Parallel()

	const list = `{"l":[{"b":1,"a":1},{"d":1,"c":1},{"f":1,"e":1}]}`

	tests := []struct {
		name    string
		doc     string
		noOrder bool
		ops     []orderedOp
		want    string
	}{
		{
			name: "replace keeps position",
			doc:  `{"b":1,"a":2,"c":3}`,
			ops:  []orderedOp{{op: "replace", path: "/a", value: `9`}},
			want: `{"b":1,"a":9,"c":3}`,
		},
		{
			name: "replace object uses value order",
			doc:  `{"z":1,"v":{"y":1,"x":2}}`,
			ops:  []orderedOp{{op: "replace", path: "/v", value: `{"q":1,"p":2}`}},
			want: `{"z":1,"v":{"q":1,"p":2}}`,
		},
		{
			name: "add existing key keeps position",
			doc:  `{"b":1,"a":2}`,
			ops:  []orderedOp{{op: "add", path: "/b", value: `5`}},
			want: `{"b":5,"a":2}`,
		},
		{
			name: "add new key appends",
			doc:  `{"b":1,"a":2}`,
			ops:  []orderedOp{{op: "add", path: "/c", value: `3`}},
			want: `{"b":1,"a":2,"c":3}`,
		},
		{
			name:    "add into nil-order node seeds sorted then appends",
			doc:     `{"v":{"y":1,"x":2}}`,
			noOrder: true,
			ops:     []orderedOp{{op: "add", path: "/v/a", value: `3`}},
			want:    `{"v":{"x":2,"y":1,"a":3}}`,
		},
		{
			name:    "add creates missing nodes along an array path",
			doc:     `{"l":[{"b":1}]}`,
			noOrder: true,
			ops:     []orderedOp{{op: "add", path: "/l/0/c", value: `{"q":1,"p":1}`}},
			want:    `{"l":[{"b":1,"c":{"q":1,"p":1}}]}`,
		},
		{
			name: "remove key then re-add appends with new order",
			doc:  `{"b":1,"a":{"x":1,"y":1},"c":3}`,
			ops: []orderedOp{
				{op: "remove", path: "/a"},
				{op: "add", path: "/a", value: `{"y":1,"x":1}`},
			},
			want: `{"b":1,"c":3,"a":{"y":1,"x":1}}`,
		},
		{
			name: "array remove shifts items",
			doc:  list,
			ops:  []orderedOp{{op: "remove", path: "/l/0"}},
			want: `{"l":[{"d":1,"c":1},{"f":1,"e":1}]}`,
		},
		{
			name: "array insert at index shifts items",
			doc:  list,
			ops:  []orderedOp{{op: "add", path: "/l/1", value: `{"h":1,"g":1}`}},
			want: `{"l":[{"b":1,"a":1},{"h":1,"g":1},{"d":1,"c":1},{"f":1,"e":1}]}`,
		},
		{
			name: "array dash appends",
			doc:  list,
			ops:  []orderedOp{{op: "add", path: "/l/-", value: `{"h":1,"g":1}`}},
			want: `{"l":[{"b":1,"a":1},{"d":1,"c":1},{"f":1,"e":1},{"h":1,"g":1}]}`,
		},
		{
			name: "array replace at index",
			doc:  list,
			ops:  []orderedOp{{op: "replace", path: "/l/1", value: `{"h":1,"g":1}`}},
			want: `{"l":[{"b":1,"a":1},{"h":1,"g":1},{"f":1,"e":1}]}`,
		},
		{
			name:    "array without recorded items is padded",
			doc:     `{"l":[{"b":1,"a":1},{"d":1,"c":1}]}`,
			noOrder: true,
			ops:     []orderedOp{{op: "add", path: "/l/1", value: `{"h":1,"g":1}`}},
			want:    `{"l":[{"a":1,"b":1},{"h":1,"g":1},{"c":1,"d":1}]}`,
		},
		{
			name: "move keeps subtree order",
			doc:  `{"a":{"y":1,"x":1},"b":1}`,
			ops:  []orderedOp{{op: "move", from: "/a", path: "/c"}},
			want: `{"b":1,"c":{"y":1,"x":1}}`,
		},
		{
			name: "move within array handles index shift",
			doc:  list,
			ops:  []orderedOp{{op: "move", from: "/l/0", path: "/l/2"}},
			want: `{"l":[{"d":1,"c":1},{"f":1,"e":1},{"b":1,"a":1}]}`,
		},
		{
			name: "move from equals path is a no-op",
			doc:  `{"a":2,"b":1}`,
			ops:  []orderedOp{{op: "move", from: "/a", path: "/a"}},
			want: `{"a":2,"b":1}`,
		},
		{
			name: "copy keeps order and is independent",
			doc:  `{"a":{"y":1,"x":1}}`,
			ops: []orderedOp{
				{op: "copy", from: "/a", path: "/b"},
				{op: "add", path: "/b/w", value: `1`},
			},
			want: `{"a":{"y":1,"x":1},"b":{"y":1,"x":1,"w":1}}`,
		},
		{
			name: "root replace uses value order",
			doc:  `{"a":1}`,
			ops:  []orderedOp{{op: "replace", path: "", value: `{"z":1,"a":1}`}},
			want: `{"z":1,"a":1}`,
		},
		{
			name: "move to root takes subtree order",
			doc:  `{"a":{"y":1,"x":1}}`,
			ops:  []orderedOp{{op: "move", from: "/a", path: ""}},
			want: `{"y":1,"x":1}`,
		},
		{
			name: "test leaves order alone",
			doc:  `{"b":1,"a":2}`,
			ops:  []orderedOp{{op: "test", path: "/a", value: `2`}},
			want: `{"b":1,"a":2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := applyOrdered(t, tt.doc, tt.noOrder, buildOps(t, tt.ops))
			if got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

// TestApplyWithOrder_SharedValueOrder verifies that one ValueOrder used by two
// operations is cloned per insert: a later op editing one location must not
// change the other location's order, nor the caller's ValueOrder.
func TestApplyWithOrder_SharedValueOrder(t *testing.T) {
	t.Parallel()

	const raw = `{"y":1,"x":1}`
	shared := mustCapture(t, raw)
	sharedBefore := shared.Clone()
	value := mustDecode(t, raw)
	ops := []Operation{
		{Op: "add", Path: "/a", Value: value, ValueOrder: shared},
		{Op: "add", Path: "/b", Value: value, ValueOrder: shared},
		{Op: "add", Path: "/a/w", Value: float64(1)},
	}

	got := applyOrdered(t, `{}`, false, ops)
	want := `{"a":{"y":1,"x":1,"w":1},"b":{"y":1,"x":1}}`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if diff := cmp.Diff(sharedBefore, shared); diff != "" {
		t.Errorf("caller's ValueOrder mutated (-want +got):\n%s", diff)
	}
}

// TestApplyWithOrder_FailureReturnsOriginals verifies atomicity: a failing
// operation returns the original document and the original, unmodified order.
func TestApplyWithOrder_FailureReturnsOriginals(t *testing.T) {
	t.Parallel()

	const docJSON = `{"b":1,"a":{"y":1,"x":1}}`
	doc := mustDecode(t, docJSON)
	order := mustCapture(t, docJSON)
	orderBefore := order.Clone()
	ops := buildOps(t, []orderedOp{
		{op: "add", path: "/c", value: `{"q":1}`},
		{op: "remove", path: "/a/x"},
		{op: "replace", path: "/missing", value: `1`},
	})

	gotDoc, gotOrder, err := ApplyWithOrder(doc, order, ops)
	if err == nil || !strings.Contains(err.Error(), "operation 2") {
		t.Fatalf("error = %v, want an operation 2 error", err)
	}
	if gotOrder != order {
		t.Errorf("returned order is not the original pointer")
	}
	if diff := cmp.Diff(orderBefore, order); diff != "" {
		t.Errorf("original order mutated (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(mustDecode(t, docJSON), gotDoc); diff != "" {
		t.Errorf("original doc not returned unchanged (-want +got):\n%s", diff)
	}
}

// TestApplyWithOrder_ErrorPaths verifies that every error class returns the
// originals and keeps Apply's error text.
func TestApplyWithOrder_ErrorPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ops     []Operation
		errFrag string
	}{
		{"validation", []Operation{{Op: "bogus", Path: "/a"}}, "invalid operation"},
		{"add bad parent", []Operation{{Op: "add", Path: "/zz/a", Value: 1}}, `key "zz" not found`},
		{"remove missing", []Operation{{Op: "remove", Path: "/zz"}}, `key "zz" not found`},
		{"move child of from", []Operation{{Op: "move", From: "/a", Path: "/a/b"}}, "is a child of from"},
		{"move from missing", []Operation{{Op: "move", From: "/zz", Path: "/b"}}, "from path"},
		{"move same missing", []Operation{{Op: "move", From: "/zz", Path: "/zz"}}, "from path"},
		{"move bad target", []Operation{{Op: "move", From: "/a", Path: "/zz/q"}}, `key "zz" not found`},
		{"copy from missing", []Operation{{Op: "copy", From: "/zz", Path: "/b"}}, "from path"},
		{"test mismatch", []Operation{{Op: "test", Path: "/a", Value: "nope"}}, "test failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			const docJSON = `{"a":{"y":1}}`
			order := mustCapture(t, docJSON)
			_, gotOrder, err := ApplyWithOrder(mustDecode(t, docJSON), order, tt.ops)
			if err == nil || !strings.Contains(err.Error(), tt.errFrag) {
				t.Fatalf("error = %v, want to contain %q", err, tt.errFrag)
			}
			if gotOrder != order {
				t.Errorf("returned order is not the original pointer")
			}
		})
	}
}

// TestApplyWithOrder_NilOrderMatchesApply verifies that with no order at all
// the result equals Apply's and the encoding equals json.Marshal's for
// operations that only touch existing keys.
func TestApplyWithOrder_NilOrderMatchesApply(t *testing.T) {
	t.Parallel()

	const docJSON = `{"b":{"y":1,"x":[1,{"q":1,"p":2}]},"a":2}`
	ops := buildOps(t, []orderedOp{{op: "replace", path: "/b/x/1/q", value: `5`}})

	want, err := Apply(mustDecode(t, docJSON), ops)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, gotOrder, err := ApplyWithOrder(mustDecode(t, docJSON), nil, ops)
	if err != nil {
		t.Fatalf("ApplyWithOrder: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("doc differs from Apply (-want +got):\n%s", diff)
	}
	ordered, _ := jsonorder.Marshal(got, gotOrder)
	plain, _ := json.Marshal(want)
	if string(ordered) != string(plain) {
		t.Errorf("ordered %s != plain %s", ordered, plain)
	}
}
