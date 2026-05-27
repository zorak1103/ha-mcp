# Issue #87 — Add `max` Field to AutomationConfig and ScriptConfig

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the missing `max` concurrency field to `AutomationConfig` and `ScriptConfig` so `manage_automation` and `manage_script` can create, update, and faithfully round-trip automations/scripts that use `mode: parallel` or `mode: queued`.

**Architecture:** Add `Max int` (with `json:"max,omitempty"`) to both structs; add a `max` branch to `AutomationConfig.UnmarshalJSON` (custom unmarshaller must explicitly enumerate every field); wire the field through both handlers' schema, create, and update paths; update both natural-language formatters to render `(max: N)` when set; add unit tests (types, handlers, formatters, patch round-trip) and integration tests that prove the silent-data-loss bug is gone.

**Tech Stack:** Go 1.26+, standard `encoding/json`, `task` (Taskfile), `golangci-lint` v2, `testify` (integration tests), real Home Assistant instance (integration tests only)

---

## File Map

| File | Change |
|------|--------|
| `internal/homeassistant/types.go` | Add `Max int` to `AutomationConfig` + `ScriptConfig`; add `max` branch to `AutomationConfig.UnmarshalJSON` |
| `internal/homeassistant/types_test.go` | New test functions for AutomationConfig and ScriptConfig max round-trips |
| `internal/handlers/automations.go` | Schema (`max` property), `handleCreate` (`Max` field), `applyAutomationConfigUpdates` (`maxVal` branch), new `getInt` helper |
| `internal/handlers/automations_test.go` | Capture `lastUpdatedConfig` in mock; new schema/create/update test cases |
| `internal/handlers/scripts.go` | Schema (`max` property), `handleCreate` + `handleUpdate` (`maxVal` branch) |
| `internal/handlers/scripts_test.go` | New schema/create/update test cases |
| `internal/handlers/formatter/automations.go` | Append `(max: N)` to Mode line in `formatDetail` when `Max > 0` |
| `internal/handlers/formatter/automations_test.go` | New test for max rendering |
| `internal/handlers/formatter/scripts.go` | Same `(max: N)` change in `formatDetail` |
| `internal/handlers/formatter/scripts_test.go` | New test for max rendering |
| `internal/handlers/patch_test.go` | Extend `TestConfigToMap` and `TestMapToStruct` fixtures with `max: 5` |
| `internal/handlers/integration/automations_integration_test.go` | New `TestAutomationMaxFieldPreservation` test |
| `internal/handlers/integration/scripts_integration_test.go` | New `TestScriptMaxFieldPreservation` test |
| `docs/tools.md` | Add `max` parameter row to `manage_automation` and `manage_script` tables |
| `.claude/skills/ha-mcp/ha-mcp-tools/SKILL.md` | Add `max` to create/update parameter reference for both tools |
| `.claude/skills/ha-mcp/ha-mcp-gotchas/SKILL.md` | New gotcha: pre-fix ha-mcp stripped `max` on update |
| `CLAUDE.md` | Extend `AutomationConfig.UnmarshalJSON` gotcha note |

---

## Task 1: Write failing round-trip tests for `AutomationConfig.Max` and `ScriptConfig.Max`

**Files:**
- Modify: `internal/homeassistant/types_test.go`

- [ ] **Step 1: Add the test functions**

Append to `internal/homeassistant/types_test.go`:

```go
func TestAutomationConfig_Max_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantMax int
	}{
		{
			name:    "max field is preserved when set",
			input:   `{"id":"a1","mode":"parallel","max":5}`,
			wantMax: 5,
		},
		{
			name:    "max defaults to zero when absent",
			input:   `{"id":"a2","mode":"parallel"}`,
			wantMax: 0,
		},
		{
			name:    "max preserved with singular keys (WebSocket format)",
			input:   `{"id":"a3","mode":"queued","max":3,"trigger":[],"condition":[],"action":[]}`,
			wantMax: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var cfg AutomationConfig
			if err := json.Unmarshal([]byte(tt.input), &cfg); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if cfg.Max != tt.wantMax {
				t.Errorf("Max = %d, want %d", cfg.Max, tt.wantMax)
			}
		})
	}
}

func TestAutomationConfig_Max_MarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("max is included when non-zero", func(t *testing.T) {
		t.Parallel()
		cfg := AutomationConfig{ID: "a1", Mode: "parallel", Max: 5}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if !strings.Contains(string(data), `"max":5`) {
			t.Errorf("expected JSON to contain max:5, got: %s", data)
		}
	})

	t.Run("max is omitted when zero", func(t *testing.T) {
		t.Parallel()
		cfg := AutomationConfig{ID: "a2", Mode: "single"}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if strings.Contains(string(data), `"max"`) {
			t.Errorf("expected JSON to omit max, got: %s", data)
		}
	})
}

func TestScriptConfig_Max_RoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("max is preserved through marshal/unmarshal", func(t *testing.T) {
		t.Parallel()
		original := ScriptConfig{Alias: "My Script", Mode: "parallel", Max: 7}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if !strings.Contains(string(data), `"max":7`) {
			t.Errorf("expected JSON to contain max:7, got: %s", data)
		}

		var restored ScriptConfig
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if restored.Max != 7 {
			t.Errorf("Max = %d, want 7", restored.Max)
		}
	})

	t.Run("max is omitted when zero", func(t *testing.T) {
		t.Parallel()
		cfg := ScriptConfig{Alias: "My Script", Mode: "single"}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if strings.Contains(string(data), `"max"`) {
			t.Errorf("expected JSON to omit max, got: %s", data)
		}
	})
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
go test ./internal/homeassistant/... -run 'TestAutomationConfig_Max|TestScriptConfig_Max' -v
```

Expected: FAIL — `cfg.Max` is always 0 because the field doesn't exist yet.

---

## Task 2: Add `Max int` to both config structs and fix `AutomationConfig.UnmarshalJSON`

**Files:**
- Modify: `internal/homeassistant/types.go`

- [ ] **Step 1: Add `Max` to `AutomationConfig`**

In `types.go`, find the `AutomationConfig` struct (around line 139). Add `Max` after `Mode`:

```go
type AutomationConfig struct {
	ID          string         `json:"id,omitempty"`
	Alias       string         `json:"alias,omitempty"`
	Description string         `json:"description,omitempty"`
	Mode        string         `json:"mode,omitempty"` // single, restart, queued, parallel
	Max         int            `json:"max,omitempty"`  // concurrent run limit; only meaningful for mode=parallel|queued (HA default: 10)
	Triggers    []any          `json:"triggers,omitempty"`
	Conditions  []any          `json:"conditions,omitempty"`
	Actions     []any          `json:"actions,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
}
```

- [ ] **Step 2: Add `max` branch to `AutomationConfig.UnmarshalJSON`**

In the `UnmarshalJSON` method (around line 186, after the `mode` branch), add:

```go
	if v, ok := raw["max"]; ok {
		if err := json.Unmarshal(v, &c.Max); err != nil {
			return err
		}
	}
```

Place it immediately after the `mode` block and before the `variables` block.

- [ ] **Step 3: Add `Max` to `ScriptConfig`**

In `types.go`, find `ScriptConfig` (around line 225). Add `Max` after `Mode`:

```go
type ScriptConfig struct {
	Alias       string         `json:"alias,omitempty"`
	Description string         `json:"description,omitempty"`
	Mode        string         `json:"mode,omitempty"` // single, restart, queued, parallel
	Max         int            `json:"max,omitempty"`  // concurrent run limit; only meaningful for mode=parallel|queued (HA default: 10)
	Icon        string         `json:"icon,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
	Sequence    []any          `json:"sequence"`
}
```

- [ ] **Step 4: Run the Task 1 tests — they must now pass**

```bash
go test ./internal/homeassistant/... -run 'TestAutomationConfig_Max|TestScriptConfig_Max' -v
```

Expected: PASS for all cases.

- [ ] **Step 5: Run the full package test suite to catch regressions**

```bash
go test ./internal/homeassistant/...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/homeassistant/types.go internal/homeassistant/types_test.go
git commit -m "fix: add Max field to AutomationConfig and ScriptConfig for mode=parallel|queued

Fixes #87. Both structs now declare Max int (json:\"max,omitempty\"). The custom
UnmarshalJSON on AutomationConfig explicitly enumerates fields — without the new
branch, Max was silently dropped on every GetAutomation() call, causing data loss
on subsequent updates."
```

---

## Task 3: Enhance `mockAutomationClient` to capture the updated config

**Files:**
- Modify: `internal/handlers/automations_test.go`

The automation mock's `UpdateAutomation` currently discards the config parameter (third arg is `_`). The upcoming Task 4 update tests need to inspect what config was passed.

- [ ] **Step 1: Add `lastUpdatedConfig` field to the mock struct**

Find the `mockAutomationClient` struct (around line 38). Add the new field:

```go
	lastGetID         string
	lastUpdateID      string
	lastUpdatedConfig *homeassistant.AutomationConfig  // <- add this
	lastDeleteID      string
	lastToggleID      string
	lastCreatedConfig *homeassistant.AutomationConfig
	// ... rest unchanged
```

- [ ] **Step 2: Capture the config in `UpdateAutomation`**

Find the `UpdateAutomation` method (around line 83). Change it from:

```go
func (m *mockAutomationClient) UpdateAutomation(_ context.Context, automationID string, _ homeassistant.AutomationConfig) error {
	m.lastUpdateID = automationID
	m.updateCalled = true
	return m.updateErr
}
```

to:

```go
func (m *mockAutomationClient) UpdateAutomation(_ context.Context, automationID string, config homeassistant.AutomationConfig) error {
	m.lastUpdateID = automationID
	m.updateCalled = true
	configCopy := config
	m.lastUpdatedConfig = &configCopy
	return m.updateErr
}
```

- [ ] **Step 3: Verify existing tests still pass**

```bash
go test ./internal/handlers/... -run 'TestManageAutomation' -v 2>&1 | tail -20
```

Expected: all existing automation tests pass (no behavior change — only captures what was already being discarded).

---

## Task 4: Write failing handler unit tests for automations `max` field

**Files:**
- Modify: `internal/handlers/automations_test.go`

- [ ] **Step 1: Add `"max"` to schema expected-properties test**

Find `TestManageAutomation_Schema` and the `expectedProps` slice (around line 162). Add `"max"`:

```go
expectedProps := []string{"action", "automation_id", "alias", "trigger", "condition", "automation_action", "mode", "max", "enabled", "state", "verbose", "limit", "cursor", "format"}
```

- [ ] **Step 2: Add `max` create test case**

In `TestManageAutomation_Create`, add a new test case that verifies `Max` is passed to `CreateAutomation`. Find the `tests` slice and add:

```go
		{
			name: "create with max sets Max field",
			args: map[string]any{
				"action":            "create",
				"alias":             "Parallel Test",
				"mode":              "parallel",
				"max":               float64(5),
				"trigger":           []any{map[string]any{"platform": "state"}},
				"automation_action": []any{map[string]any{"service": "light.turn_on"}},
			},
			client:       &mockAutomationClient{},
			wantContains: []string{"created successfully"},
		},
```

After the existing table-driven loop, add a separate assertion block for the max case. The cleanest approach is a dedicated sub-test directly after the table loop in `TestManageAutomation_Create`:

```go
func TestManageAutomation_Create_MaxField(t *testing.T) {
	t.Parallel()

	client := &mockAutomationClient{}
	h := &AutomationHandlers{}
	args := map[string]any{
		"action":            "create",
		"alias":             "Parallel Test",
		"mode":              "parallel",
		"max":               float64(5),
		"trigger":           []any{map[string]any{"platform": "state"}},
		"automation_action": []any{map[string]any{"service": "light.turn_on"}},
	}
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})

	result, err := h.handleManageAutomation(ctx, client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
	if client.lastCreatedConfig == nil {
		t.Fatal("expected CreateAutomation to be called")
	}
	if client.lastCreatedConfig.Max != 5 {
		t.Errorf("Max = %d, want 5", client.lastCreatedConfig.Max)
	}
}
```

- [ ] **Step 3: Add update preservation and set tests**

Add two new test functions:

```go
func TestManageAutomation_Update_MaxPreservation(t *testing.T) {
	t.Parallel()

	// Existing automation has Max:10; update changes only the alias.
	// The updated config sent to HA must still have Max:10.
	existing := &homeassistant.Automation{
		EntityID: "automation.test",
		State:    "on",
		Config: &homeassistant.AutomationConfig{
			ID:    "test",
			Alias: "Old Alias",
			Mode:  "parallel",
			Max:   10,
		},
	}
	client := &mockAutomationClient{automation: existing}
	h := &AutomationHandlers{}
	args := map[string]any{
		"action":        "update",
		"automation_id": "test",
		"alias":         "New Alias",
		// deliberately no "max" arg — regression test for silent data loss
	}
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})

	result, err := h.handleManageAutomation(ctx, client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
	if client.lastUpdatedConfig == nil {
		t.Fatal("expected UpdateAutomation to be called")
	}
	if client.lastUpdatedConfig.Max != 10 {
		t.Errorf("Max = %d, want 10 (field must be preserved when not in args)", client.lastUpdatedConfig.Max)
	}
}

func TestManageAutomation_Update_MaxSet(t *testing.T) {
	t.Parallel()

	existing := &homeassistant.Automation{
		EntityID: "automation.test",
		State:    "on",
		Config:   &homeassistant.AutomationConfig{ID: "test", Alias: "Test", Mode: "parallel", Max: 10},
	}
	client := &mockAutomationClient{automation: existing}
	h := &AutomationHandlers{}
	args := map[string]any{
		"action":        "update",
		"automation_id": "test",
		"max":           float64(3),
	}
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})

	result, err := h.handleManageAutomation(ctx, client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
	if client.lastUpdatedConfig == nil {
		t.Fatal("expected UpdateAutomation to be called")
	}
	if client.lastUpdatedConfig.Max != 3 {
		t.Errorf("Max = %d, want 3", client.lastUpdatedConfig.Max)
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail (or schema test for the right reason)**

```bash
go test ./internal/handlers/... -run 'TestManageAutomation_Schema|TestManageAutomation_Create_MaxField|TestManageAutomation_Update_Max' -v
```

Expected:
- `TestManageAutomation_Schema`: FAIL (no "max" in schema yet)
- `TestManageAutomation_Create_MaxField`: FAIL (Max == 0, not 5)
- `TestManageAutomation_Update_Max*`: FAIL or PASS (update preservation may already work due to struct field existing, but create does not pass Max yet)

---

## Task 5: Wire `max` into the automations handler

**Files:**
- Modify: `internal/handlers/automations.go`

- [ ] **Step 1: Add `max` property to the schema**

Find the schema `Properties` map (around line 119, after the `"mode"` entry). Add:

```go
				"max": {
					Type:        "integer",
					Description: "Concurrent run limit (minimum 1, HA default 10). Only applies when mode is 'parallel' or 'queued'.",
				},
```

- [ ] **Step 2: Add `getInt` helper**

Find `getString` and `getSlice` helpers (around line 831). Add `getInt` alongside them:

```go
// getInt safely extracts an integer value from a map of arguments.
// MCP JSON-RPC decodes all numbers as float64.
func getInt(args map[string]any, key string) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return 0
}
```

- [ ] **Step 3: Add `Max` to the create config literal**

Find `handleCreate` where the `AutomationConfig` literal is built (around line 348). Add `Max`:

```go
	config := homeassistant.AutomationConfig{
		ID:          id,
		Alias:       alias,
		Description: getString(args, "description"),
		Triggers:    trigger,
		Conditions:  getSlice(args, "condition"),
		Actions:     automationAction,
		Mode:        getString(args, "mode"),
		Max:         getInt(args, "max"),
	}
```

- [ ] **Step 4: Add `maxVal` branch to `applyAutomationConfigUpdates`**

Find `applyAutomationConfigUpdates` (around line 825, after the `mode` branch). Add:

```go
	if maxVal, ok := args["max"].(float64); ok {
		config.Max = int(maxVal)
	}
```

Note: use `maxVal` not `max` — `max` is a Go built-in and `revive redefines-builtin-id` will fail the lint.

- [ ] **Step 5: Run the Task 4 tests — they must now pass**

```bash
go test ./internal/handlers/... -run 'TestManageAutomation_Schema|TestManageAutomation_Create_MaxField|TestManageAutomation_Update_Max' -v
```

Expected: all PASS.

- [ ] **Step 6: Run lint**

```bash
task lint
```

Expected: no new linter errors. If `goconst` triggers on the `"max"` string, extract to a package-level constant (e.g., `const automationArgMax = "max"`) and use it in schema, `getInt` call, and wherever else it appears.

- [ ] **Step 7: Run full handler tests**

```bash
go test ./internal/handlers/...
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/handlers/automations.go internal/handlers/automations_test.go
git commit -m "feat: expose max field in manage_automation tool

Add max integer parameter to schema, create, and update paths.
Add getInt helper for float64→int conversion (MCP JSON-RPC encodes
all numbers as float64). Regression tests confirm Max is preserved
on update when not explicitly provided."
```

---

## Task 6: Write failing handler unit tests for scripts `max` field

**Files:**
- Modify: `internal/handlers/scripts_test.go`

Note: `mockScriptClient` already captures `lastUpdateConfig` (the field and capture are already in the struct). No mock changes needed.

- [ ] **Step 1: Find the scripts schema test and add `"max"`**

Search for the scripts schema test (look for `expectedProps` or a schema assertion block). Add `"max"` to the expected properties. If no such test exists, create one:

```go
func TestManageScript_Schema(t *testing.T) {
	t.Parallel()
	h := NewScriptHandlers()
	tool := h.manageScriptTool()
	expectedProps := []string{"action", "script_id", "alias", "description", "mode", "max", "icon", "sequence", "fields", "variables", "format"}
	for _, prop := range expectedProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("expected property %q in script tool schema", prop)
		}
	}
}
```

- [ ] **Step 2: Add create test**

```go
func TestManageScript_Create_MaxField(t *testing.T) {
	t.Parallel()

	var capturedConfig *homeassistant.ScriptConfig
	client := &mockScriptClient{
		createScriptFn: func(_ context.Context, _ string, config homeassistant.ScriptConfig) error {
			configCopy := config
			capturedConfig = &configCopy
			return nil
		},
	}
	h := &ScriptHandlers{}
	args := map[string]any{
		"action":    "create",
		"script_id": "parallel_script",
		"alias":     "Parallel Script",
		"mode":      "parallel",
		"max":       float64(4),
		"sequence":  []any{map[string]any{"service": "light.turn_on"}},
	}
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})

	result, err := h.handleManageScript(ctx, client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got: %s", result.Content[0].Text)
	}
	if capturedConfig == nil {
		t.Fatal("expected CreateScript to be called")
	}
	if capturedConfig.Max != 4 {
		t.Errorf("Max = %d, want 4", capturedConfig.Max)
	}
}
```

- [ ] **Step 3: Add update preservation test**

```go
func TestManageScript_Update_MaxPreservation(t *testing.T) {
	t.Parallel()

	// Script update starts from *current.Config, so Max is preserved automatically
	// once the struct field exists. This test confirms the contract holds.
	client := &mockScriptClient{
		getScriptFn: func(_ context.Context, _ string) (*homeassistant.Script, error) {
			return &homeassistant.Script{
				EntityID: "script.test_script",
				Config: &homeassistant.ScriptConfig{
					Alias:    "Test Script",
					Mode:     "parallel",
					Max:      8,
					Sequence: []any{map[string]any{"service": "light.turn_on"}},
				},
			}, nil
		},
	}
	h := &ScriptHandlers{}
	args := map[string]any{
		"action":    "update",
		"script_id": "test_script",
		"alias":     "Updated Script",
		// deliberately no "max" arg
	}
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{Timeout: 50 * time.Millisecond, PollInterval: 5 * time.Millisecond})

	result, err := h.handleManageScript(ctx, client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got: %s", result.Content[0].Text)
	}
	if client.lastUpdateConfig == nil {
		t.Fatal("expected UpdateScript to be called")
	}
	if client.lastUpdateConfig.Max != 8 {
		t.Errorf("Max = %d, want 8 (must be preserved when not in args)", client.lastUpdateConfig.Max)
	}
}
```

- [ ] **Step 4: Run to confirm failures**

```bash
go test ./internal/handlers/... -run 'TestManageScript_Schema|TestManageScript_Create_MaxField|TestManageScript_Update_MaxPreservation' -v
```

Expected: FAIL (max not in schema, Max not set in create).

---

## Task 7: Wire `max` into the scripts handler

**Files:**
- Modify: `internal/handlers/scripts.go`

- [ ] **Step 1: Add `max` property to the schema**

Find the schema `Properties` map (around line 79, after the `"mode"` entry). Add:

```go
				"max": {
					Type:        "integer",
					Description: "Concurrent run limit (minimum 1, HA default 10). Only applies when mode is 'parallel' or 'queued'.",
				},
```

- [ ] **Step 2: Add `Max` to `handleCreate`**

Find the create block (around line 337-339, where mode and icon are set). Add after the `mode` assignment:

```go
	if maxVal, ok := args["max"].(float64); ok {
		config.Max = int(maxVal)
	}
```

- [ ] **Step 3: Add `Max` to `handleUpdate`**

Find the update block (around line 391-394, where mode is applied). Add after the `mode` assignment:

```go
	if maxVal, ok := args["max"].(float64); ok {
		config.Max = int(maxVal)
	}
```

Note: The update starts from `*current.Config` (which already preserves `Max`). This block handles the case where the caller wants to explicitly change the `max` value.

- [ ] **Step 4: Run Task 6 tests — they must pass**

```bash
go test ./internal/handlers/... -run 'TestManageScript_Schema|TestManageScript_Create_MaxField|TestManageScript_Update_MaxPreservation' -v
```

Expected: all PASS.

- [ ] **Step 5: Lint and full handler tests**

```bash
task lint
go test ./internal/handlers/...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/scripts.go internal/handlers/scripts_test.go
git commit -m "feat: expose max field in manage_script tool

Mirror of the manage_automation change: add max to schema, create,
and update paths. Script update starts from current.Config so Max
is preserved automatically; the new branch handles explicit changes."
```

---

## Task 8: Add `max` to automations formatter output

**Files:**
- Modify: `internal/handlers/formatter/automations.go`
- Modify: `internal/handlers/formatter/automations_test.go`

- [ ] **Step 1: Write the failing formatter test**

In `automations_test.go`, add a new test function after `TestNaturalAutomationFormatter_FormatDetail`:

```go
func TestNaturalAutomationFormatter_FormatDetail_Max(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalAutomationFormatter()

	t.Run("max shown for parallel mode", func(t *testing.T) {
		automation := homeassistant.Automation{
			EntityID: "automation.parallel_test",
			State:    "on",
			Config: &homeassistant.AutomationConfig{
				ID:       "parallel_test",
				Alias:    "Parallel Test",
				Mode:     "parallel",
				Max:      5,
				Triggers: []any{},
				Actions:  []any{},
			},
		}
		result, err := f.FormatDetail(ctx, automation)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "(max: 5)") {
			t.Errorf("expected '(max: 5)' in output, got: %s", result)
		}
	})

	t.Run("max not shown when zero", func(t *testing.T) {
		automation := homeassistant.Automation{
			EntityID: "automation.single_test",
			State:    "on",
			Config: &homeassistant.AutomationConfig{
				ID:    "single_test",
				Alias: "Single Test",
				Mode:  "single",
			},
		}
		result, err := f.FormatDetail(ctx, automation)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(result, "max:") || strings.Contains(result, "(max") {
			t.Errorf("expected no max in output for zero Max, got: %s", result)
		}
	})
}
```

- [ ] **Step 2: Run to confirm FAIL**

```bash
go test ./internal/handlers/formatter/... -run 'TestNaturalAutomationFormatter_FormatDetail_Max' -v
```

Expected: FAIL — output doesn't contain "(max: 5)" yet.

- [ ] **Step 3: Update the formatter**

In `formatter/automations.go`, find the Mode rendering in `formatDetail` (around line 145-150):

```go
		mode := automation.Config.Mode
		if mode == "" {
			mode = modeSingle
		}
		result.WriteString("Mode: " + mode)
```

Change to:

```go
		mode := automation.Config.Mode
		if mode == "" {
			mode = modeSingle
		}
		modeStr := mode
		if automation.Config.Max > 0 {
			modeStr = fmt.Sprintf("%s (max: %d)", mode, automation.Config.Max)
		}
		result.WriteString("Mode: " + modeStr)
```

Ensure `"fmt"` is already in the import block (it likely is from `fmt.Fprintf`).

- [ ] **Step 4: Run formatter tests — must pass**

```bash
go test ./internal/handlers/formatter/... -run 'TestNaturalAutomationFormatter' -v
```

Expected: all PASS including the new test.

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/formatter/automations.go internal/handlers/formatter/automations_test.go
git commit -m "feat: render (max: N) in automation natural output when Max is set"
```

---

## Task 9: Add `max` to scripts formatter output

**Files:**
- Modify: `internal/handlers/formatter/scripts.go`
- Modify: `internal/handlers/formatter/scripts_test.go`

- [ ] **Step 1: Write the failing formatter test**

In `formatter/scripts_test.go`, find the appropriate place and add:

```go
func TestNaturalScriptFormatter_FormatDetail_Max(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalScriptFormatter()

	t.Run("max shown for parallel mode", func(t *testing.T) {
		script := homeassistant.Script{
			EntityID: "script.parallel_test",
			State:    "on",
			Config: &homeassistant.ScriptConfig{
				Alias:    "Parallel Test",
				Mode:     "parallel",
				Max:      3,
				Sequence: []any{},
			},
		}
		result, err := f.FormatDetail(ctx, script)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "(max: 3)") {
			t.Errorf("expected '(max: 3)' in output, got: %s", result)
		}
	})

	t.Run("max not shown when zero", func(t *testing.T) {
		script := homeassistant.Script{
			EntityID: "script.single_test",
			State:    "on",
			Config: &homeassistant.ScriptConfig{
				Alias:    "Single Test",
				Mode:     "single",
				Sequence: []any{},
			},
		}
		result, err := f.FormatDetail(ctx, script)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(result, "max:") || strings.Contains(result, "(max") {
			t.Errorf("expected no max in output for zero Max, got: %s", result)
		}
	})
}
```

- [ ] **Step 2: Confirm FAIL**

```bash
go test ./internal/handlers/formatter/... -run 'TestNaturalScriptFormatter_FormatDetail_Max' -v
```

Expected: FAIL.

- [ ] **Step 3: Update the scripts formatter**

In `formatter/scripts.go`, find the Mode rendering in `formatDetail` (around line 108-112):

```go
		mode := script.Config.Mode
		if mode == "" {
			mode = "single"
		}
		result.WriteString("Mode: " + mode)
```

Change to:

```go
		mode := script.Config.Mode
		if mode == "" {
			mode = "single"
		}
		modeStr := mode
		if script.Config.Max > 0 {
			modeStr = fmt.Sprintf("%s (max: %d)", mode, script.Config.Max)
		}
		result.WriteString("Mode: " + modeStr)
```

- [ ] **Step 4: Run and confirm PASS**

```bash
go test ./internal/handlers/formatter/... -v
```

Expected: all PASS.

- [ ] **Step 5: Lint check**

```bash
task lint
```

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/formatter/scripts.go internal/handlers/formatter/scripts_test.go
git commit -m "feat: render (max: N) in script natural output when Max is set"
```

---

## Task 10: Extend patch round-trip tests with `max` field

**Files:**
- Modify: `internal/handlers/patch_test.go`

The patch engine converts `AutomationConfig` to a `map[string]any` (via JSON marshal), applies RFC 6902 operations, then converts back (via JSON unmarshal). With `Max int` in the struct and the `UnmarshalJSON` branch added, the round-trip is automatically correct. This task adds regression coverage.

- [ ] **Step 1: Extend `TestConfigToMap` with a max fixture**

Find `TestConfigToMap` (around line 156) and extend the config:

```go
	config := homeassistant.AutomationConfig{
		ID:          "morning_routine",
		Alias:       "Morning Routine",
		Description: "Turn on lights",
		Mode:        "parallel",
		Max:         5,
		Triggers:    []any{map[string]any{"trigger": "time"}},
	}
```

Add an assertion for `max`:

```go
	if m["max"] != float64(5) {
		t.Errorf("max = %v, want 5", m["max"])
	}
	if m["mode"] != "parallel" {
		t.Errorf("mode = %v, want 'parallel'", m["mode"])
	}
```

- [ ] **Step 2: Extend `TestMapToStruct` with a max fixture**

Find `TestMapToStruct` (around line 183) and extend the map:

```go
	m := map[string]any{
		"id":    "morning_routine",
		"alias": "Morning Routine",
		"mode":  "queued",
		"max":   float64(7),
	}
```

Add an assertion:

```go
	if config.Max != 7 {
		t.Errorf("Max = %d, want 7", config.Max)
	}
```

- [ ] **Step 3: Run patch tests**

```bash
go test ./internal/handlers/... -run 'TestConfigToMap|TestMapToStruct' -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/handlers/patch_test.go
git commit -m "test: extend patch round-trip tests to cover max field"
```

---

## Task 11: Integration tests — end-to-end max field preservation

**Files:**
- Modify: `internal/handlers/integration/automations_integration_test.go`
- Modify: `internal/handlers/integration/scripts_integration_test.go`

These tests run against a real Home Assistant instance and prove the full data-loss bug is fixed. They require env vars — if unset, tests skip gracefully.

- [ ] **Step 1: Add `TestAutomationMaxFieldPreservation` to the automation integration test file**

Add this method to `AutomationIntegrationTestSuite`. Note the integration test conventions: use `GenerateTestID` for unique names, `s.RegisterCleanup` for teardown, and `s.WaitForAutomation` to block until HA processes the create. The automation config ID and alias must match so HA derives the expected entity ID.

```go
func (s *AutomationIntegrationTestSuite) TestAutomationMaxFieldPreservation() {
	automationID := GenerateTestID("auto_max")
	automationEntityID := BuildEntityID("automation", automationID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
	})

	// Step 1: Create a parallel automation with max:5
	// Alias must equal ID so HA slugifies it to the same value (CLAUDE.md gotcha)
	cfg := homeassistant.AutomationConfig{
		ID:    automationID,
		Alias: automationID,
		Mode:  "parallel",
		Max:   5,
		Triggers: []any{
			map[string]any{"platform": "event", "event_type": "test_event"},
		},
		Actions: []any{
			map[string]any{"delay": map[string]any{"seconds": 1}},
		},
	}
	err := s.Client().CreateAutomation(s.Context(), cfg)
	s.Require().NoError(err, "failed to create parallel automation")

	_, err = s.WaitForAutomation(automationID, 30*time.Second)
	s.Require().NoError(err, "automation did not appear after create")
	_ = automationEntityID

	// Step 2: Fetch and verify Max was stored
	fetched, err := s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err)
	s.Require().NotNil(fetched.Config, "GetAutomation must return Config")
	s.Equal(5, fetched.Config.Max, "Max should be 5 after create")

	// Step 3: Update changing only the description (no max arg — regression test)
	updatedCfg := *fetched.Config
	updatedCfg.Description = "updated description"
	err = s.Client().UpdateAutomation(s.Context(), automationID, updatedCfg)
	s.Require().NoError(err, "failed to update automation")

	// Step 4: Fetch again and confirm Max is still 5 (not silently erased)
	afterUpdate, err := s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err)
	s.Require().NotNil(afterUpdate.Config)
	s.Equal(5, afterUpdate.Config.Max, "Max must survive an update that doesn't change it")
}
```

- [ ] **Step 2: Add `TestScriptMaxFieldPreservation` to the scripts integration test file**

```go
func (s *ScriptIntegrationTestSuite) TestScriptMaxFieldPreservation() {
	scriptID := GenerateTestID("script_max")
	scriptEntityID := BuildEntityID("script", scriptID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
	})

	// Step 1: Create a parallel script with max:4
	cfg := homeassistant.ScriptConfig{
		Alias:    scriptID, // Alias drives the entity ID slug
		Mode:     "parallel",
		Max:      4,
		Sequence: []any{map[string]any{"delay": map[string]any{"seconds": 1}}},
	}
	err := s.Client().CreateScript(s.Context(), scriptID, cfg)
	s.Require().NoError(err, "failed to create parallel script")

	_, err = s.WaitForEntity(scriptEntityID, 30*time.Second)
	s.Require().NoError(err, "script entity did not appear after create")

	// Step 2: Fetch and verify Max
	fetched, err := s.Client().GetScript(s.Context(), scriptEntityID)
	s.Require().NoError(err)
	s.Require().NotNil(fetched.Config, "GetScript must return Config")
	s.Equal(4, fetched.Config.Max, "Max should be 4 after create")

	// Step 3: Update description only (no max arg — the round-trip preservation test)
	updatedCfg := *fetched.Config
	updatedCfg.Description = "updated description"
	err = s.Client().UpdateScript(s.Context(), scriptEntityID, updatedCfg)
	s.Require().NoError(err, "failed to update script")

	// Step 4: Confirm Max survived
	afterUpdate, err := s.Client().GetScript(s.Context(), scriptEntityID)
	s.Require().NoError(err)
	s.Require().NotNil(afterUpdate.Config)
	s.Equal(4, afterUpdate.Config.Max, "Max must survive an update that doesn't change it")
}
```

- [ ] **Step 3: Run the integration tests (requires real HA instance)**

In testify suites, individual test methods appear as subtests under the top-level `TestAutomationIntegration` / `TestScriptIntegration` functions. Filter by method name:

```bash
set -a && source .env.integration && set +a
go test -tags=integration -v \
  -run 'TestAutomationIntegration/TestAutomationMaxFieldPreservation|TestScriptIntegration/TestScriptMaxFieldPreservation' \
  ./internal/handlers/integration/...
```

Expected: both PASS.

- [ ] **Step 4: Run the full integration suite to check for regressions**

```bash
set -a && source .env.integration && set +a
go test -tags=integration -v ./internal/handlers/integration/... 2>&1 | grep -E 'PASS|FAIL|SKIP'
```

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/integration/automations_integration_test.go \
        internal/handlers/integration/scripts_integration_test.go
git commit -m "test(integration): add max field preservation tests for automations and scripts

Proves the full round-trip: create with max:N → get → update (no max arg)
→ get → max still N. Catches the silent data loss regression."
```

---

## Task 12: Update documentation

**Files:**
- Modify: `docs/tools.md`
- Modify: `.claude/skills/ha-mcp/ha-mcp-tools/SKILL.md`
- Modify: `.claude/skills/ha-mcp/ha-mcp-gotchas/SKILL.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add `max` to `docs/tools.md` parameter tables**

Find the `manage_automation` parameters table and add a row (in the parameters section, near `mode`):

```
| max         | integer | No       | Concurrent run limit (min 1, HA default 10). Only applies when mode is `parallel` or `queued`. |
```

Add the same row to the `manage_script` parameters table.

- [ ] **Step 2: Update `.claude/skills/ha-mcp/ha-mcp-tools/SKILL.md`**

Find the create/update parameter reference for `manage_automation` and `manage_script`. Add:
```
- max (integer, optional): concurrent run limit; only for mode=parallel|queued (default 10)
```

- [ ] **Step 3: Add gotcha to `.claude/skills/ha-mcp/ha-mcp-gotchas/SKILL.md`**

Add a new entry:

```markdown
## max field silently dropped on automation/script update (pre-fix)

**Symptom:** Parallel automations/scripts lose their `max` concurrency setting after any update through ha-mcp.

**Root cause:** `AutomationConfig.UnmarshalJSON` explicitly enumerates all fields. Before the fix, `max` had no branch, so it was silently dropped every time HA's GetAutomation response was decoded. The zero value was then written back on update.

**Fix:** Landed in ha-mcp v?.?.? — both `AutomationConfig` and `ScriptConfig` now declare `Max int` with a proper unmarshal branch.

**If you see it after the fix:** Verify you're running the patched version; the UnmarshalJSON branch is required (adding the struct field alone is insufficient).
```

- [ ] **Step 4: Update `CLAUDE.md` gotcha note**

Find the `AutomationConfig.UnmarshalJSON` gotcha in the `API & Type Gotchas` section. Extend it:

Add after the existing note about `ContentBlock.MarshalJSON` (or wherever the `AutomationConfig` note currently lives):

```
- **AutomationConfig.UnmarshalJSON field enumeration:** The custom UnmarshalJSON (types.go:165) explicitly handles each field. Any new field added to `AutomationConfig` MUST also get an unmarshal branch — otherwise the value is silently dropped when reading from HA's API. `ScriptConfig` uses default unmarshalling and only needs the struct field.
```

- [ ] **Step 5: Commit docs**

```bash
git add docs/tools.md \
        .claude/skills/ha-mcp/ha-mcp-tools/SKILL.md \
        .claude/skills/ha-mcp/ha-mcp-gotchas/SKILL.md \
        CLAUDE.md
git commit -m "docs: document max field for manage_automation and manage_script

Update tools reference, ha-mcp skill bundle, and CLAUDE.md gotcha section."
```

---

## Final verification

```bash
# Full lint + unit test suite
task lint
task test

# Targeted coverage for changed packages
go test -v ./internal/homeassistant/... -run 'TestAutomationConfig_Max|TestScriptConfig_Max'
go test -v ./internal/handlers/... -run 'TestManageAutomation|TestManageScript'
go test -v ./internal/handlers/formatter/... -run 'TestNaturalAutomationFormatter|TestNaturalScriptFormatter'

# Integration (needs real HA)
set -a && source .env.integration && set +a
go test -tags=integration -v -run 'TestAutomationMaxFieldPreservation|TestScriptMaxFieldPreservation' \
  ./internal/handlers/integration/...
```

Manual smoke test against a real HA:
1. `manage_automation create` with `mode: parallel, max: 3, alias: "Max Test", ...`
2. `manage_automation get automation_id: max_test` — confirm output shows `Mode: parallel (max: 3)`
3. `manage_automation update automation_id: max_test alias: "Max Test Updated"` — no `max` arg
4. `manage_automation get automation_id: max_test_updated` — confirm `(max: 3)` still present
5. Repeat steps 1-4 with `manage_script`
