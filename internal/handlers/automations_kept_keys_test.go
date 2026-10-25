package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// =============================================================================
// Issue #314: update and patch keep config keys the struct does not model
// =============================================================================

// issue314AutomationJSON is an automation as HA returns it, carrying the four
// keys AutomationConfig has no field for.
const issue314AutomationJSON = `{
	"id": "heating",
	"alias": "Heating",
	"description": "old",
	"mode": "single",
	"trigger_variables": {"limit": "{{ 5 }}"},
	"initial_state": true,
	"max_exceeded": "warning",
	"trace": {"stored_traces": 10},
	"triggers": [{"trigger": "state", "entity_id": "sensor.a"}],
	"actions": [{"action": "light.turn_on"}]
}`

// issue314ScriptJSON is a script as HA returns it, carrying the two keys
// ScriptConfig has no field for.
const issue314ScriptJSON = `{
	"alias": "Heat",
	"description": "old",
	"mode": "single",
	"max_exceeded": "warning",
	"trace": {"stored_traces": 10},
	"sequence": [{"action": "light.turn_on"}]
}`

// keptKeyCase describes one update or patch call and the config it must write.
type keptKeyCase struct {
	name string
	raw  string
	// wantKept lists the kept keys that must be present with exactly this value.
	wantKept map[string]any
	// wantAbsent lists keys that must not be in the written config.
	wantAbsent []string
	// wantDescription is the description the written config must carry.
	wantDescription string
}

// decodeWrittenConfig decodes one written config into a generic map.
func decodeWrittenConfig(t *testing.T, written string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(written), &got); err != nil {
		t.Fatalf("decoding written config %q: %v", written, err)
	}
	return got
}

// assertKeptKeys checks a written config against a keptKeyCase.
func assertKeptKeys(t *testing.T, written string, tt keptKeyCase) {
	t.Helper()
	got := decodeWrittenConfig(t, written)
	for key, want := range tt.wantKept {
		have, ok := got[key]
		if !ok {
			t.Errorf("written config lost %q: %s", key, written)
			continue
		}
		if diff := cmp.Diff(want, have); diff != "" {
			t.Errorf("written config has wrong %q (-want +got):\n%s", key, diff)
		}
	}
	for _, key := range tt.wantAbsent {
		if _, ok := got[key]; ok {
			t.Errorf("written config still carries %q: %s", key, written)
		}
	}
	if got["description"] != tt.wantDescription {
		t.Errorf("description = %v, want %q", got["description"], tt.wantDescription)
	}
}

// assertNoWrite checks that a call skipped the write as a detected no-op.
func assertNoWrite(t *testing.T, text string, writes []string) {
	t.Helper()
	if len(writes) != 0 {
		t.Fatalf("expected no write, got %v", writes)
	}
	if !strings.Contains(text, "no changes detected") {
		t.Errorf("expected no-op message, got %q", text)
	}
}

func automationKeptKeysAfterUnrelatedChange() map[string]any {
	return map[string]any{
		"trigger_variables": map[string]any{"limit": "{{ 5 }}"},
		"initial_state":     true,
		"max_exceeded":      "warning",
		"trace":             map[string]any{"stored_traces": float64(10)},
	}
}

func TestAutomationUpdateAndPatch_Issue314_KeepUnmodeledKeys(t *testing.T) {
	t.Parallel()

	tests := []keptKeyCase{
		{
			name:            "patch unrelated replace keeps all four keys",
			raw:             `{"action":"patch","automation_id":"heating","operations":[{"op":"replace","path":"/description","value":"new"}]}`,
			wantKept:        automationKeptKeysAfterUnrelatedChange(),
			wantDescription: "new",
		},
		{
			name:            "update description only keeps all four keys",
			raw:             `{"action":"update","automation_id":"heating","description":"new"}`,
			wantKept:        automationKeptKeysAfterUnrelatedChange(),
			wantDescription: "new",
		},
		{
			name: "patch replace trace stored_traces writes the new value",
			raw:  `{"action":"patch","automation_id":"heating","operations":[{"op":"replace","path":"/trace/stored_traces","value":25}]}`,
			wantKept: map[string]any{
				"trigger_variables": map[string]any{"limit": "{{ 5 }}"},
				"initial_state":     true,
				"max_exceeded":      "warning",
				"trace":             map[string]any{"stored_traces": float64(25)},
			},
			wantDescription: "old",
		},
		{
			name: "patch remove trigger_variables removes only that key",
			raw:  `{"action":"patch","automation_id":"heating","operations":[{"op":"remove","path":"/trigger_variables"}]}`,
			wantKept: map[string]any{
				"initial_state": true,
				"max_exceeded":  "warning",
				"trace":         map[string]any{"stored_traces": float64(10)},
			},
			wantAbsent:      []string{"trigger_variables"},
			wantDescription: "old",
		},
	}
	for _, tt := range tests {
		for _, withOrder := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/withOrder=%v", tt.name, withOrder), func(t *testing.T) {
				t.Parallel()
				_, writes := orderedAutomationPatch(t, issue314AutomationJSON, tt.raw, withOrder)
				assertKeptKeys(t, singleWrite(t, writes), tt)
			})
		}
	}
}

// TestAutomationPatch_Issue314_TestOnlyPatchSkipsWrite: the no-op check covers
// kept keys, so a patch of only test ops (against a kept key and a modeled one)
// writes nothing.
func TestAutomationPatch_Issue314_TestOnlyPatchSkipsWrite(t *testing.T) {
	t.Parallel()

	raw := `{"action":"patch","automation_id":"heating","operations":[` +
		`{"op":"test","path":"/trace/stored_traces","value":10},` +
		`{"op":"test","path":"/description","value":"old"}]}`
	result, writes := orderedAutomationPatch(t, issue314AutomationJSON, raw, true)
	assertNoWrite(t, result.Content[0].Text, writes)
}

// TestAutomationUpdate_Issue314_IdenticalUpdateSkipsWrite: an update that
// changes nothing still skips the write when the config carries kept keys.
func TestAutomationUpdate_Issue314_IdenticalUpdateSkipsWrite(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating","description":"old"}`
	result, writes := orderedAutomationPatch(t, issue314AutomationJSON, raw, true)
	assertNoWrite(t, result.Content[0].Text, writes)
}

// =============================================================================
// Issue #319: update reports the top-level keys it carried over
// =============================================================================

const keptKeysNotePrefix = "kept from previous config"

// issue319AutomationJSON is issue314AutomationJSON plus a variables block.
const issue319AutomationJSON = `{
	"id": "heating",
	"alias": "Heating",
	"description": "old",
	"mode": "single",
	"variables": {"abt": "climate.x", "desired": "{{ 1 }}"},
	"trigger_variables": {"limit": "{{ 5 }}"},
	"initial_state": true,
	"max_exceeded": "warning",
	"trace": {"stored_traces": 10},
	"triggers": [{"trigger": "state", "entity_id": "sensor.a"}],
	"actions": [{"action": "light.turn_on"}]
}`

// issue319OwnedOnlyAutomationJSON carries only keys update has an argument for.
const issue319OwnedOnlyAutomationJSON = `{
	"id": "heating",
	"alias": "Heating",
	"mode": "single",
	"triggers": [{"trigger": "state", "entity_id": "sensor.a"}],
	"actions": [{"action": "light.turn_on"}]
}`

func TestAutomationUpdate_Issue319_ReportsKeptKeys(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating","description":"new"}`
	result, writes := orderedAutomationPatch(t, issue319AutomationJSON, raw, true)

	want := keptKeysNotePrefix + ": initial_state, max_exceeded, trace, trigger_variables, variables"
	if text := result.Content[0].Text; !strings.Contains(text, want) {
		t.Errorf("result text %q does not contain %q", text, want)
	}
	assertKeptKeys(t, singleWrite(t, writes), keptKeyCase{
		wantKept:        map[string]any{"variables": map[string]any{"abt": "climate.x", "desired": "{{ 1 }}"}},
		wantDescription: "new",
	})
}

func TestAutomationUpdate_Issue319_NoNoteWithoutKeptKeys(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating","description":"new"}`
	result, _ := orderedAutomationPatch(t, issue319OwnedOnlyAutomationJSON, raw, true)
	if text := result.Content[0].Text; strings.Contains(text, keptKeysNotePrefix) {
		t.Errorf("unexpected kept-keys note in %q", text)
	}
}

func TestAutomationUpdate_Issue319_IdenticalUpdateHasNoNote(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating","description":"old"}`
	result, _ := orderedAutomationPatch(t, issue314AutomationJSON, raw, true)
	if text := result.Content[0].Text; strings.Contains(text, keptKeysNotePrefix) {
		t.Errorf("unexpected kept-keys note in no-op result %q", text)
	}
}

// =============================================================================
// Issue #319: manage_automation takes a variables argument
// =============================================================================

// TestAutomationUpdate_Issue319_VariablesReplacesBlock: update with a
// non-empty variables object replaces the whole block, in the caller's key
// order when it reaches the handler, and does not list it as kept.
func TestAutomationUpdate_Issue319_VariablesReplacesBlock(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating","variables":{"zz":"1","aa":"{{ zz }}"}}`
	for _, withOrder := range []bool{true, false} {
		t.Run(fmt.Sprintf("withOrder=%v", withOrder), func(t *testing.T) {
			t.Parallel()
			result, writes := orderedAutomationPatch(t, issue319AutomationJSON, raw, withOrder)
			written := singleWrite(t, writes)

			if withOrder {
				want := `"variables":{"zz":"1","aa":"{{ zz }}"}`
				if !strings.Contains(written, want) {
					t.Errorf("written config lost the caller's variables order\n got: %s\nwant substring: %s", written, want)
				}
			}
			assertKeptKeys(t, written, keptKeyCase{
				wantKept:        map[string]any{"variables": map[string]any{"zz": "1", "aa": "{{ zz }}"}},
				wantDescription: "old",
			})
			if strings.Contains(written, "abt") {
				t.Errorf("old variables survived the replace: %s", written)
			}
			text := result.Content[0].Text
			if !strings.Contains(text, keptKeysNotePrefix+": initial_state, max_exceeded, trace, trigger_variables)") {
				t.Errorf("note should name the other kept keys but not variables: %q", text)
			}
		})
	}
}

// TestAutomationUpdate_Issue319_EmptyVariablesRemovesBlock: {} and null drop
// the block while every other extra key is still carried and named.
func TestAutomationUpdate_Issue319_EmptyVariablesRemovesBlock(t *testing.T) {
	t.Parallel()

	for _, value := range []string{`{}`, `null`} {
		for _, withOrder := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/withOrder=%v", value, withOrder), func(t *testing.T) {
				t.Parallel()
				raw := `{"action":"update","automation_id":"heating","variables":` + value + `}`
				result, writes := orderedAutomationPatch(t, issue319AutomationJSON, raw, withOrder)
				assertKeptKeys(t, singleWrite(t, writes), keptKeyCase{
					wantKept:        automationKeptKeysAfterUnrelatedChange(),
					wantAbsent:      []string{"variables"},
					wantDescription: "old",
				})
				want := keptKeysNotePrefix + ": initial_state, max_exceeded, trace, trigger_variables)"
				if text := result.Content[0].Text; !strings.Contains(text, want) {
					t.Errorf("result text %q does not contain %q", text, want)
				}
			})
		}
	}
}

// TestAutomationUpdate_Issue319_RepeatedVariablesSkipsWrite: sending the block
// the automation already has, in the same order, is a no-op.
func TestAutomationUpdate_Issue319_RepeatedVariablesSkipsWrite(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","automation_id":"heating","variables":{"abt":"climate.x","desired":"{{ 1 }}"}}`
	result, writes := orderedAutomationPatch(t, issue319AutomationJSON, raw, true)
	assertNoWrite(t, result.Content[0].Text, writes)
}

// TestAutomationUpdate_Issue319_VariablesMustBeObject: a non-object value is
// refused before anything is fetched or written.
func TestAutomationUpdate_Issue319_VariablesMustBeObject(t *testing.T) {
	t.Parallel()

	for _, value := range []any{"x", []any{}, float64(1)} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			t.Parallel()
			updates := 0
			client := &UniversalMockClient{}
			client.UpdateAutomationFn = func(context.Context, string, homeassistant.AutomationConfig) error {
				updates++
				return nil
			}
			args := map[string]any{"action": "update", "automation_id": "heating", "variables": value}
			result, err := (&AutomationHandlers{}).handleManageAutomation(context.Background(), client, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result, got %q", result.Content[0].Text)
			}
			if text := result.Content[0].Text; !strings.Contains(text, "variables must be an object") {
				t.Errorf("error text %q does not explain the variables type", text)
			}
			if updates != 0 {
				t.Errorf("UpdateAutomation was called %d times, want 0", updates)
			}
		})
	}
}

// TestAutomationCreate_Issue319_Variables: create sets the block, in the
// caller's order, and refuses a non-object value without creating anything.
func TestAutomationCreate_Issue319_Variables(t *testing.T) {
	t.Parallel()

	waitCtx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})
	base := `"alias":"Heating","trigger":[{"trigger":"state","entity_id":"sensor.a"}],` +
		`"automation_action":[{"action":"light.turn_on"}]`

	t.Run("sets the block in caller order", func(t *testing.T) {
		t.Parallel()
		var got homeassistant.AutomationConfig
		client := &UniversalMockClient{}
		client.CreateAutomationFn = func(_ context.Context, cfg homeassistant.AutomationConfig) error {
			got = cfg
			return nil
		}
		ctx, args := ctxWithRawArgs(t, waitCtx, `{`+base+`,"variables":{"zz":"1","aa":"{{ zz }}"}}`)
		result, err := (&AutomationHandlers{}).handleCreate(ctx, client, args)
		if err != nil || result.IsError {
			t.Fatalf("unexpected failure: %v %v", err, result)
		}
		if got.Variables["zz"] != "1" || got.Variables["aa"] != "{{ zz }}" {
			t.Errorf("Variables = %v, want zz and aa set", got.Variables)
		}
		raw, mErr := json.Marshal(got)
		if mErr != nil {
			t.Fatalf("marshaling created config: %v", mErr)
		}
		if want := `"variables":{"zz":"1","aa":"{{ zz }}"}`; !strings.Contains(string(raw), want) {
			t.Errorf("created config lost the caller's variables order\n got: %s\nwant substring: %s", raw, want)
		}
	})

	t.Run("string is an error and creates nothing", func(t *testing.T) {
		t.Parallel()
		creates := 0
		client := &UniversalMockClient{}
		client.CreateAutomationFn = func(context.Context, homeassistant.AutomationConfig) error {
			creates++
			return nil
		}
		ctx, args := ctxWithRawArgs(t, waitCtx, `{`+base+`,"variables":"x"}`)
		result, err := (&AutomationHandlers{}).handleCreate(ctx, client, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError || !strings.Contains(result.Content[0].Text, "variables must be an object") {
			t.Errorf("expected a variables type error, got %+v", result)
		}
		if creates != 0 {
			t.Errorf("CreateAutomation was called %d times, want 0", creates)
		}
	})
}
