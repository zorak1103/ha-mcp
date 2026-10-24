package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/jsonorder"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// =============================================================================
// Issue #311: create/update preserve key order (scripts)
// =============================================================================

// TestScriptCreate_RawArgsSendOrderedBytes: fields and sequence supplied as
// raw JSON are written in the caller's key order.
func TestScriptCreate_RawArgsSendOrderedBytes(t *testing.T) {
	t.Parallel()

	var writes []string
	client := &UniversalMockClient{}
	client.CreateScriptFn = func(_ context.Context, _ string, cfg homeassistant.ScriptConfig) error {
		b, err := json.Marshal(cfg)
		if err != nil {
			t.Errorf("marshaling written config: %v", err)
		}
		writes = append(writes, string(b))
		return nil
	}
	raw := `{"action":"create","script_id":"heat","alias":"Heat",` +
		`"fields":{"zone":{"selector":{"text":{}},"name":"Zone"},"temp":{"required":true,"name":"Temp"}},` +
		`"sequence":[{"variables":{"zeta":"1","alpha":"{{ zeta }}"}},{"target":{"entity_id":"light.a"},"action":"light.turn_on"}]}`
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})
	ctx, args := ctxWithRawArgs(t, ctx, raw)
	result, err := (&ScriptHandlers{}).handleManageScript(ctx, client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
	got := singleWrite(t, writes)
	for _, want := range []string{
		`"fields":{"zone":{"selector":{"text":{}},"name":"Zone"},"temp":{"required":true,"name":"Temp"}}`,
		`"sequence":[{"variables":{"zeta":"1","alpha":"{{ zeta }}"}},{"target":{"entity_id":"light.a"},"action":"light.turn_on"}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("written config lost the caller's key order\n got: %s\nwant substring: %s", got, want)
		}
	}
}

// TestScriptUpdate_UpdateDescriptionOnlyKeepsOrder: an unrelated update writes
// the whole config in HA's stored order.
func TestScriptUpdate_UpdateDescriptionOnlyKeepsOrder(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","script_id":"heat","description":"new"}`
	for _, withOrder := range []bool{true, false} {
		_, writes := orderedScriptPatch(t, issue311ScriptJSON, raw, withOrder)
		got := singleWrite(t, writes)
		want := compactJSON(t, strings.Replace(issue311ScriptJSON, `"old"`, `"new"`, 1))
		if got != want {
			t.Errorf("withOrder=%v: written config lost HA's key order\n got: %s\nwant: %s", withOrder, got, want)
		}
	}
}

// TestScriptUpdate_UpdateActionsTakesCallerOrder: a replaced sequence and
// fields map are written in the caller's order.
func TestScriptUpdate_UpdateActionsTakesCallerOrder(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","script_id":"heat",` +
		`"fields":{"temp":{"name":"Temp","required":true},"zone":{"name":"Zone"}},` +
		`"sequence":[{"variables":{"zz":"1","mm":"{{ zz }}","aa":"{{ mm }}"}}]}`
	_, writes := orderedScriptPatch(t, issue311ScriptJSON, raw, true)
	got := singleWrite(t, writes)
	for _, want := range []string{
		`"fields":{"temp":{"name":"Temp","required":true},"zone":{"name":"Zone"}}`,
		`"sequence":[{"variables":{"zz":"1","mm":"{{ zz }}","aa":"{{ mm }}"}}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("written config has the wrong key order\n got: %s\nwant substring: %s", got, want)
		}
	}
}

// TestScriptUpdate_GoMapArgDropsStaleOrder pins Review Focus trap 3 for
// scripts: a sequence/fields replaced from a Go map (no caller order) is
// sorted, never ordered by HA's stale subtree applied by index/key.
func TestScriptUpdate_GoMapArgDropsStaleOrder(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","script_id":"heat",` +
		`"fields":{"zone":{"selector":{"text":{}},"name":"Z2"},"temp":{"required":false,"name":"T2"}},` +
		`"sequence":[{"variables":{"zeta":"x","alpha":"y"}}]}`
	_, writes := orderedScriptPatch(t, issue311ScriptJSON, raw, false)
	got := singleWrite(t, writes)
	for _, want := range []string{
		`"fields":{"temp":{"name":"T2","required":false},"zone":{"name":"Z2","selector":{"text":{}}}}`,
		`"sequence":[{"variables":{"alpha":"y","zeta":"x"}}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stale HA order leaked into a caller-replaced subtree\n got: %s\nwant substring: %s", got, want)
		}
	}
}

// TestScriptUpdate_ReorderOnlySequenceWrites: the same content in a different
// key order is a real change (Review Focus trap 2).
func TestScriptUpdate_ReorderOnlySequenceWrites(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","script_id":"heat","sequence":[` +
		`{"variables":{"alpha":"{{ zeta }}","zeta":"1"}},` +
		`{"action":"light.turn_on","target":{"entity_id":"light.a"}},` +
		`{"action":"light.turn_on","target":{"entity_id":"light.b"}}]}`
	_, writes := orderedScriptPatch(t, issue311ScriptJSON, raw, true)
	got := singleWrite(t, writes)
	if want := `{"variables":{"alpha":"{{ zeta }}","zeta":"1"}}`; !strings.Contains(got, want) {
		t.Errorf("reorder-only update not written in caller order\n got: %s\nwant substring: %s", got, want)
	}
}

// TestScriptUpdate_IdenticalUpdateSkipsWrite: a no-op update still skips the
// write.
func TestScriptUpdate_IdenticalUpdateSkipsWrite(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","script_id":"heat","description":"old"}`
	result, writes := orderedScriptPatch(t, issue311ScriptJSON, raw, true)
	if len(writes) != 0 {
		t.Fatalf("expected no write, got %v", writes)
	}
	if !strings.Contains(result.Content[0].Text, "no changes detected") {
		t.Errorf("expected no-op message, got %q", result.Content[0].Text)
	}
}

// TestScriptUpdate_DoesNotMutateFetchedKeyOrder pins Review Focus trap 4:
// handleUpdate copies the fetched struct, so its KeyOrder pointer is shared;
// composing the new order must be copy-on-write.
func TestScriptUpdate_DoesNotMutateFetchedKeyOrder(t *testing.T) {
	t.Parallel()

	var fetched homeassistant.ScriptConfig
	if err := json.Unmarshal([]byte(issue311ScriptJSON), &fetched); err != nil {
		t.Fatal(err)
	}
	snapshot := fetched.KeyOrder.Clone()

	client := &UniversalMockClient{}
	client.GetScriptFn = func(context.Context, string) (*homeassistant.Script, error) {
		return &homeassistant.Script{EntityID: "script.heat", Config: &fetched}, nil
	}
	client.UpdateScriptFn = func(context.Context, string, homeassistant.ScriptConfig) error { return nil }
	client.GetEntityRegistryFn = storageManagedRegistry("script.heat")

	raw := `{"action":"update","script_id":"heat",` +
		`"fields":{"a":{"name":"A"}},"sequence":[{"variables":{"z":"1","y":"2"}}]}`
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})
	ctx, args := ctxWithRawArgs(t, ctx, raw)
	result, err := (&ScriptHandlers{}).handleManageScript(ctx, client, args)
	if err != nil || result.IsError {
		t.Fatalf("unexpected failure: %v %v", err, result)
	}
	if diff := cmp.Diff(snapshot, fetched.KeyOrder); diff != "" {
		t.Errorf("fetched KeyOrder was mutated (-before +after):\n%s", diff)
	}
}

func TestScriptArgsOrder(t *testing.T) {
	t.Parallel()

	baseJSON := `{"alias":"A","fields":{"b":{"name":"B"},"a":{"name":"A"}},"sequence":[{"variables":{"b":1,"a":2}}]}`
	argOrder := mustCapture(t, `{"fields":{"z":{"name":"Z"},"y":{}},"sequence":[{"variables":{"z":1,"y":2}}]}`)
	someList := []any{map[string]any{}}
	someMap := map[string]any{"x": map[string]any{}}

	tests := []struct {
		name     string
		base     *jsonorder.Order
		argOrder *jsonorder.Order
		args     map[string]any
		want     func(base *jsonorder.Order) *jsonorder.Order
	}{
		{
			name: "no subtree args keeps base",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"alias": "B", "variables": someMap},
			want: func(b *jsonorder.Order) *jsonorder.Order { return b },
		},
		{
			name: "nil base and no caller order stays nil",
			args: map[string]any{"fields": someMap, "sequence": someList},
			want: func(*jsonorder.Order) *jsonorder.Order { return nil },
		},
		{
			name: "fields and sequence take caller order",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"fields": someMap, "sequence": someList},
			want: func(b *jsonorder.Order) *jsonorder.Order {
				return b.WithField("fields", argOrder.Field("fields")).WithField("sequence", argOrder.Field("sequence"))
			},
		},
		{
			name:     "nil base with caller order builds a root",
			argOrder: argOrder,
			args:     map[string]any{"sequence": someList},
			want: func(*jsonorder.Order) *jsonorder.Order {
				return (*jsonorder.Order)(nil).WithField("sequence", argOrder.Field("sequence"))
			},
		},
		{
			name: "no caller order drops stale subtrees",
			base: mustCapture(t, baseJSON),
			args: map[string]any{"fields": someMap, "sequence": someList},
			want: func(b *jsonorder.Order) *jsonorder.Order {
				return b.WithField("fields", nil).WithField("sequence", nil)
			},
		},
		{
			name: "wrong-typed subtree args are ignored",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"fields": someList, "sequence": someMap},
			want: func(b *jsonorder.Order) *jsonorder.Order { return b },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			baseBefore := tt.base.Clone()
			got := scriptArgsOrder(tt.base, tt.argOrder, tt.args)
			if diff := cmp.Diff(tt.want(baseBefore), got); diff != "" {
				t.Errorf("scriptArgsOrder() mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(baseBefore, tt.base); diff != "" {
				t.Errorf("base was mutated (-before +after):\n%s", diff)
			}
		})
	}
}
