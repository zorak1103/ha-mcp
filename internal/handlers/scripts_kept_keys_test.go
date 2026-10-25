package handlers

import (
	"fmt"
	"testing"
)

// =============================================================================
// Issue #314: script update and patch keep config keys the struct does not model
// =============================================================================

func scriptKeptKeysAfterUnrelatedChange() map[string]any {
	return map[string]any{
		"max_exceeded": "warning",
		"trace":        map[string]any{"stored_traces": float64(10)},
	}
}

func TestScriptUpdateAndPatch_Issue314_KeepUnmodeledKeys(t *testing.T) {
	t.Parallel()

	tests := []keptKeyCase{
		{
			name:            "patch unrelated replace keeps both keys",
			raw:             `{"action":"patch","script_id":"heat","operations":[{"op":"replace","path":"/description","value":"new"}]}`,
			wantKept:        scriptKeptKeysAfterUnrelatedChange(),
			wantDescription: "new",
		},
		{
			name:            "update description only keeps both keys",
			raw:             `{"action":"update","script_id":"heat","description":"new"}`,
			wantKept:        scriptKeptKeysAfterUnrelatedChange(),
			wantDescription: "new",
		},
		{
			name: "patch replace trace stored_traces writes the new value",
			raw:  `{"action":"patch","script_id":"heat","operations":[{"op":"replace","path":"/trace/stored_traces","value":25}]}`,
			wantKept: map[string]any{
				"max_exceeded": "warning",
				"trace":        map[string]any{"stored_traces": float64(25)},
			},
			wantDescription: "old",
		},
		{
			name: "patch remove max_exceeded removes only that key",
			raw:  `{"action":"patch","script_id":"heat","operations":[{"op":"remove","path":"/max_exceeded"}]}`,
			wantKept: map[string]any{
				"trace": map[string]any{"stored_traces": float64(10)},
			},
			wantAbsent:      []string{"max_exceeded"},
			wantDescription: "old",
		},
	}
	for _, tt := range tests {
		for _, withOrder := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/withOrder=%v", tt.name, withOrder), func(t *testing.T) {
				t.Parallel()
				_, writes := orderedScriptPatch(t, issue314ScriptJSON, tt.raw, withOrder)
				assertKeptKeys(t, singleWrite(t, writes), tt)
			})
		}
	}
}

// TestScriptPatch_Issue314_TestOnlyPatchSkipsWrite: the no-op check covers
// kept keys, so a patch of only test ops writes nothing.
func TestScriptPatch_Issue314_TestOnlyPatchSkipsWrite(t *testing.T) {
	t.Parallel()

	raw := `{"action":"patch","script_id":"heat","operations":[` +
		`{"op":"test","path":"/trace/stored_traces","value":10},` +
		`{"op":"test","path":"/description","value":"old"}]}`
	result, writes := orderedScriptPatch(t, issue314ScriptJSON, raw, true)
	assertNoWrite(t, result.Content[0].Text, writes)
}

// TestScriptUpdate_Issue314_IdenticalUpdateSkipsWrite: an update that changes
// nothing still skips the write when the config carries kept keys.
func TestScriptUpdate_Issue314_IdenticalUpdateSkipsWrite(t *testing.T) {
	t.Parallel()

	raw := `{"action":"update","script_id":"heat","description":"old"}`
	result, writes := orderedScriptPatch(t, issue314ScriptJSON, raw, true)
	assertNoWrite(t, result.Content[0].Text, writes)
}
