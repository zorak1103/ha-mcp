package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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
