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
// Issue #311: create/update preserve key order (automations)
// =============================================================================

// TestAutomationCreate_RawArgsSendOrderedBytes: a create whose arguments
// arrive as raw JSON writes nested objects (a variables action, a trigger) in
// the order the caller sent them, not sorted.
func TestAutomationCreate_RawArgsSendOrderedBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		actionKey string
	}{
		{"automation_action", "automation_action"},
		{"legacy action", "action"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var writes []string
			client := &UniversalMockClient{}
			client.CreateAutomationFn = func(_ context.Context, cfg homeassistant.AutomationConfig) error {
				b, err := json.Marshal(cfg)
				if err != nil {
					t.Errorf("marshaling written config: %v", err)
				}
				writes = append(writes, string(b))
				return nil
			}
			raw := `{"action":"create","alias":"Heating",` +
				`"trigger":[{"trigger":"state","entity_id":"sensor.a"}],` +
				`"condition":[{"state":"on","condition":"state","entity_id":"sensor.a"}],` +
				`"` + tt.actionKey + `":[{"variables":{"zeta":"1","alpha":"{{ zeta }}"}},{"target":{"entity_id":"light.a"},"action":"light.turn_on"}]}`
			if tt.actionKey == "action" {
				// "action" is also the tool's action selector; handleCreate is
				// called directly, so drop the selector and let the legacy
				// actions array own the key.
				raw = strings.Replace(raw, `"action":"create",`, ``, 1)
			}
			ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})
			ctx, args := ctxWithRawArgs(t, ctx, raw)
			result, err := (&AutomationHandlers{}).handleCreate(ctx, client, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected success, got error: %s", result.Content[0].Text)
			}
			got := singleWrite(t, writes)
			for _, want := range []string{
				`"triggers":[{"trigger":"state","entity_id":"sensor.a"}]`,
				`"conditions":[{"state":"on","condition":"state","entity_id":"sensor.a"}]`,
				`"actions":[{"variables":{"zeta":"1","alpha":"{{ zeta }}"}},{"target":{"entity_id":"light.a"},"action":"light.turn_on"}]`,
			} {
				if !strings.Contains(got, want) {
					t.Errorf("written config lost the caller's key order\n got: %s\nwant substring: %s", got, want)
				}
			}
		})
	}
}

// TestAutomationCreate_GoMapArgsKeepSortedBytes: without a recorded argument
// order (an internal caller passing a Go map) create writes exactly what it
// wrote before the fix.
func TestAutomationCreate_GoMapArgsKeepSortedBytes(t *testing.T) {
	t.Parallel()

	var got homeassistant.AutomationConfig
	client := &UniversalMockClient{}
	client.CreateAutomationFn = func(_ context.Context, cfg homeassistant.AutomationConfig) error {
		got = cfg
		return nil
	}
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})
	args := map[string]any{
		"alias":             "Heating",
		"trigger":           []any{map[string]any{"trigger": "state", "entity_id": "sensor.a"}},
		"automation_action": []any{map[string]any{"variables": map[string]any{"zeta": "1", "alpha": "2"}}},
	}
	result, err := (&AutomationHandlers{}).handleCreate(ctx, client, args)
	if err != nil || result.IsError {
		t.Fatalf("unexpected failure: %v %v", err, result)
	}
	if got.KeyOrder != nil {
		t.Errorf("KeyOrder = %+v, want nil without caller order", got.KeyOrder)
	}
}

// TestAutomationUpdate_UpdateDescriptionOnlyKeepsOrder: updating an unrelated
// field writes the whole config in HA's stored order (issue #311 via update).
func TestAutomationUpdate_UpdateDescriptionOnlyKeepsOrder(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating","description":"new"}`
	for _, withOrder := range []bool{true, false} {
		_, writes := orderedAutomationPatch(t, issue311AutomationJSON, raw, withOrder)
		got := singleWrite(t, writes)
		want := compactJSON(t, strings.Replace(issue311AutomationJSON, `"old"`, `"new"`, 1))
		if got != want {
			t.Errorf("withOrder=%v: written config lost HA's key order\n got: %s\nwant: %s", withOrder, got, want)
		}
	}
}

// TestAutomationUpdate_UpdateActionsTakesCallerOrder: a replaced actions array
// is written in the caller's order while the rest keeps HA's order.
func TestAutomationUpdate_UpdateActionsTakesCallerOrder(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating",` +
		`"automation_action":[{"variables":{"zz":"1","mm":"{{ zz }}","aa":"{{ mm }}"}}]}`
	_, writes := orderedAutomationPatch(t, issue311AutomationJSON, raw, true)
	got := singleWrite(t, writes)
	for _, want := range []string{
		`"actions":[{"variables":{"zz":"1","mm":"{{ zz }}","aa":"{{ mm }}"}}]`,
		`"variables":{"heizzeit":"{{ 5 }}","desired":"{{ heizzeit * 2 }}"}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("written config has the wrong key order\n got: %s\nwant substring: %s", got, want)
		}
	}
}

// TestAutomationUpdate_GoMapArgDropsStaleOrder pins Review Focus trap 3: an
// actions array replaced from a Go-map argument (no caller order) must not
// inherit HA's old order by index. HA's old actions[0] is
// {"variables":{"zeta","alpha"}}; the new element lists the same names, which
// would come out zeta-first if the stale order were applied.
func TestAutomationUpdate_GoMapArgDropsStaleOrder(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating",` +
		`"automation_action":[{"variables":{"zeta":"x","alpha":"y"}}],` +
		`"trigger":[{"trigger":"state","entity_id":"sensor.c","for":{"seconds":2}}]}`
	_, writes := orderedAutomationPatch(t, issue311AutomationJSON, raw, false)
	got := singleWrite(t, writes)
	for _, want := range []string{
		`"actions":[{"variables":{"alpha":"y","zeta":"x"}}]`,
		`"triggers":[{"entity_id":"sensor.c","for":{"seconds":2},"trigger":"state"}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stale HA order leaked into a caller-replaced subtree\n got: %s\nwant substring: %s", got, want)
		}
	}
}

// TestAutomationUpdate_IdenticalUpdateSkipsWrite: an update that changes
// nothing still skips the write (ordered-bytes no-op check).
func TestAutomationUpdate_IdenticalUpdateSkipsWrite(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating","description":"old"}`
	result, writes := orderedAutomationPatch(t, issue311AutomationJSON, raw, true)
	if len(writes) != 0 {
		t.Fatalf("expected no write, got %v", writes)
	}
	if !strings.Contains(result.Content[0].Text, "no changes detected") {
		t.Errorf("expected no-op message, got %q", result.Content[0].Text)
	}
}

// TestAutomationUpdate_ReorderOnlyActionsWrites: replacing actions with the
// same content in a different key order is a real change (Review Focus trap 2).
func TestAutomationUpdate_ReorderOnlyActionsWrites(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating",` +
		`"automation_action":[{"variables":{"alpha":"{{ zeta }}","zeta":"1"}},{"action":"light.turn_on"}]}`
	_, writes := orderedAutomationPatch(t, issue311AutomationJSON, raw, true)
	got := singleWrite(t, writes)
	if want := `"actions":[{"variables":{"alpha":"{{ zeta }}","zeta":"1"}},{"action":"light.turn_on"}]`; !strings.Contains(got, want) {
		t.Errorf("reorder-only update not written in caller order\n got: %s\nwant substring: %s", got, want)
	}
}

func mustCapture(t *testing.T, raw string) *jsonorder.Order {
	t.Helper()
	o, err := jsonorder.Capture([]byte(raw))
	if err != nil {
		t.Fatalf("capture %s: %v", raw, err)
	}
	return o
}

func TestAutomationArgsOrder(t *testing.T) {
	t.Parallel()

	baseJSON := `{"alias":"A","triggers":[{"trigger":"state","entity_id":"x"}],` +
		`"conditions":[{"condition":"state","entity_id":"x"}],"actions":[{"variables":{"b":1,"a":2}}]}`
	argsJSON := `{"trigger":[{"entity_id":"y","trigger":"state"}],"condition":[{"entity_id":"y","condition":"state"}],` +
		`"automation_action":[{"variables":{"z":1,"y":2}}],"action":[{"variables":{"q":1,"p":2}}]}`
	argOrder := mustCapture(t, argsJSON)
	someList := []any{map[string]any{}}

	tests := []struct {
		name       string
		base       *jsonorder.Order
		argOrder   *jsonorder.Order
		args       map[string]any
		autoFilled bool
		want       func(base *jsonorder.Order) *jsonorder.Order
	}{
		{
			name: "no subtree args keeps base",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"alias": "B", "description": "d"},
			want: func(b *jsonorder.Order) *jsonorder.Order { return b },
		},
		{
			name: "nil base and no subtree args stays nil",
			args: map[string]any{"alias": "B"},
			want: func(*jsonorder.Order) *jsonorder.Order { return nil },
		},
		{
			name: "nil base and no caller order stays nil",
			args: map[string]any{"trigger": someList, "automation_action": someList},
			want: func(*jsonorder.Order) *jsonorder.Order { return nil },
		},
		{
			name: "trigger takes caller order",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"trigger": someList},
			want: func(b *jsonorder.Order) *jsonorder.Order {
				return b.WithField("triggers", argOrder.Field("trigger"))
			},
		},
		{
			name: "empty trigger drops order",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"trigger": []any{}},
			want: func(b *jsonorder.Order) *jsonorder.Order { return b.WithField("triggers", nil) },
		},
		{
			name: "auto-filled trigger drops order",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"trigger": someList}, autoFilled: true,
			want: func(b *jsonorder.Order) *jsonorder.Order { return b.WithField("triggers", nil) },
		},
		{
			name: "condition takes caller order",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"condition": someList},
			want: func(b *jsonorder.Order) *jsonorder.Order {
				return b.WithField("conditions", argOrder.Field("condition"))
			},
		},
		{
			name: "automation_action wins over action",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"automation_action": someList, "action": someList},
			want: func(b *jsonorder.Order) *jsonorder.Order {
				return b.WithField("actions", argOrder.Field("automation_action"))
			},
		},
		{
			name: "legacy action used alone",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"action": someList},
			want: func(b *jsonorder.Order) *jsonorder.Order {
				return b.WithField("actions", argOrder.Field("action"))
			},
		},
		{
			name: "action selector string is not an actions array",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"action": "update"},
			want: func(b *jsonorder.Order) *jsonorder.Order { return b },
		},
		{
			name: "no caller order drops stale subtrees",
			base: mustCapture(t, baseJSON),
			args: map[string]any{"trigger": someList, "condition": someList, "automation_action": someList},
			want: func(b *jsonorder.Order) *jsonorder.Order {
				return b.WithField("triggers", nil).WithField("conditions", nil).WithField("actions", nil)
			},
		},
		{
			name: "wrong-typed subtree arg is ignored",
			base: mustCapture(t, baseJSON), argOrder: argOrder,
			args: map[string]any{"trigger": "nope", "condition": map[string]any{}},
			want: func(b *jsonorder.Order) *jsonorder.Order { return b },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			baseBefore := tt.base.Clone()
			got := automationArgsOrder(tt.base, tt.argOrder, tt.args, tt.autoFilled)
			if diff := cmp.Diff(tt.want(baseBefore), got); diff != "" {
				t.Errorf("automationArgsOrder() mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(baseBefore, tt.base); diff != "" {
				t.Errorf("base was mutated (-before +after):\n%s", diff)
			}
		})
	}
}
