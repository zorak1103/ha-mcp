package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TestManageTraceSchema verifies the schema for manage_trace tool.
func TestManageTraceSchema(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterTraceTools(registry)

	tool, exists := registry.GetTool("manage_trace")
	if !exists {
		t.Fatal("manage_trace tool not registered")
	}

	// Verify basic properties
	if tool.Name != "manage_trace" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_trace")
	}
	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	// Verify schema properties
	schema := tool.InputSchema
	props := schema.Properties

	// Check action field
	actionSchema, ok := props["action"]
	if !ok {
		t.Fatal("action property missing from schema")
	}
	if actionSchema.Type != "string" {
		t.Errorf("action type = %q, want %q", actionSchema.Type, "string")
	}
	if len(actionSchema.Enum) != 3 {
		t.Errorf("action enum count = %d, want 3", len(actionSchema.Enum))
	}

	// Check domain field
	domainSchema, ok := props["domain"]
	if !ok {
		t.Fatal("domain property missing from schema")
	}
	if len(domainSchema.Enum) != 2 {
		t.Errorf("domain enum count = %d, want 2 (automation, script)", len(domainSchema.Enum))
	}

	// Check automation_id field
	if _, ok := props["automation_id"]; !ok {
		t.Error("automation_id property missing from schema")
	}

	// Check hours field
	hoursSchema, ok := props["hours"]
	if !ok {
		t.Fatal("hours property missing from schema")
	}
	if hoursSchema.Type != "number" {
		t.Errorf("hours type = %q, want %q", hoursSchema.Type, "number")
	}

	// Check format field
	formatSchema, ok := props["format"]
	if !ok {
		t.Fatal("format property missing from schema")
	}
	if len(formatSchema.Enum) != 2 {
		t.Errorf("format enum count = %d, want 2", len(formatSchema.Enum))
	}

	// Check wait field (opt-in polling for async trace recording)
	if _, ok := props["wait"]; !ok {
		t.Error("wait property missing from schema")
	}

	// Check required fields
	if len(schema.Required) != 1 {
		t.Errorf("required count = %d, want 1 (action)", len(schema.Required))
	}
	if schema.Required[0] != "action" {
		t.Errorf("required[0] = %q, want %q", schema.Required[0], "action")
	}
}

// TestManageTrace_MissingAction verifies validation when action is missing.
func TestManageTrace_MissingAction(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewTraceHandlers()

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result with error message")
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
}

// TestManageTrace_InvalidAction verifies validation for invalid action.
func TestManageTrace_InvalidAction(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewTraceHandlers()

	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action": "invalid_action",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result with error message")
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
}

// TestManageTrace_List verifies list action.
func TestManageTrace_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		mockResponse any
		wantFormat   string
		wantContain  string
	}{
		{
			name: "list automations natural format",
			args: map[string]any{
				"action": "list",
				"domain": "automation",
			},
			mockResponse: []map[string]any{
				{
					"run_id":    "abc123",
					"state":     "stopped",
					"timestamp": "2024-01-15T10:30:00Z",
					"duration":  1.5,
				},
			},
			wantFormat:  "natural",
			wantContain: "abc123",
		},
		{
			name: "list scripts json format",
			args: map[string]any{
				"action": "list",
				"domain": "script",
				"format": "json",
			},
			mockResponse: []map[string]any{
				{
					"run_id":    "xyz789",
					"state":     "running",
					"timestamp": "2024-01-15T11:00:00Z",
				},
			},
			wantFormat:  "json",
			wantContain: "xyz789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				SendHACSCommandFn: func(_ context.Context, cmd string, data map[string]any) (any, error) {
					if cmd != "trace/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					if domain, _ := tt.args["domain"].(string); domain != "" {
						if data["domain"] != domain {
							return nil, fmt.Errorf("data[domain] = %v, want %q", data["domain"], domain)
						}
					}
					return tt.mockResponse, nil
				},
			}

			handler := NewTraceHandlers()
			result, err := handler.HandleManageTrace(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil || len(result.Content) == 0 {
				t.Fatal("expected result content")
			}

			text := result.Content[0].Text
			if tt.wantContain != "" && !strings.Contains(text, tt.wantContain) {
				t.Errorf("result text does not contain %q: %s", tt.wantContain, text)
			}
		})
	}
}

// TestManageTrace_Get verifies get action.
func TestManageTrace_Get(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "automation.test", UniqueID: "1700000000001"},
			}, nil
		},
		SendHACSCommandFn: func(_ context.Context, cmd string, data map[string]any) (any, error) {
			if cmd != "trace/get" {
				t.Errorf("cmd = %q, want %q", cmd, "trace/get")
			}
			if data["item_id"] != "1700000000001" {
				t.Errorf("data[item_id] = %v, want %q", data["item_id"], "1700000000001")
			}
			if _, exists := data["entity_id"]; exists {
				t.Errorf("data should not contain entity_id (found: %v)", data["entity_id"])
			}
			if data["run_id"] != "abc123" {
				t.Errorf("data[run_id] = %v, want %q", data["run_id"], "abc123")
			}
			return map[string]any{
				"trace": map[string]any{
					"trigger":    "manual",
					"conditions": []any{},
					"actions": []any{
						map[string]any{"service": "light.turn_on"},
					},
				},
			}, nil
		},
	}

	handler := NewTraceHandlers()
	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":    "get",
		"domain":    "automation",
		"entity_id": "automation.test",
		"run_id":    "abc123",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}
}

// TestManageTrace_Get_DomainEntityIDMismatch verifies that a domain/entity_id prefix conflict
// is rejected rather than silently resolving against the wrong domain (which could return
// another entity's traces - see resolveTraceListParams's identical check for the list action).
func TestManageTrace_Get_DomainEntityIDMismatch(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		SendHACSCommandFn: func(context.Context, string, map[string]any) (any, error) {
			t.Fatal("trace/get should not be called when domain and entity_id conflict")
			return nil, nil
		},
	}

	handler := NewTraceHandlers()
	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":    "get",
		"domain":    "automation",
		"entity_id": "script.foo",
		"run_id":    "abc123",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "domain") {
		t.Errorf("error text does not mention domain conflict: %s", result.Content[0].Text)
	}
}

// TestManageTrace_List_EntityIDFilter verifies that passing entity_id to the list
// action automatically derives the domain and sets item_id for server-side filtering.
// Regression test for issue #73: entity_id was silently ignored, causing the WS call
// to omit domain and return "id @ data['domain']. Got None".
func TestManageTrace_List_EntityIDFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        map[string]any
		wantDomain  string
		wantItemID  string
		wantError   bool
		wantContain string
	}{
		{
			name:       "entity_id automation prefix derives domain and item_id",
			args:       map[string]any{"action": "list", "entity_id": "automation.ev_charging"},
			wantDomain: "automation",
			wantItemID: "ev_charging",
		},
		{
			name:       "entity_id script prefix derives domain and item_id",
			args:       map[string]any{"action": "list", "entity_id": "script.morning_routine"},
			wantDomain: "script",
			wantItemID: "morning_routine",
		},
		{
			name:        "entity_id conflicts with explicit domain",
			args:        map[string]any{"action": "list", "entity_id": "automation.foo", "domain": "script"},
			wantError:   true,
			wantContain: "domain",
		},
		{
			name:        "entity_id with unknown prefix returns validation error",
			args:        map[string]any{"action": "list", "entity_id": "light.living_room"},
			wantError:   true,
			wantContain: "entity_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedData map[string]any
			client := &UniversalMockClient{
				SendHACSCommandFn: func(_ context.Context, cmd string, data map[string]any) (any, error) {
					if cmd != "trace/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					capturedData = data
					return []any{}, nil
				},
			}

			handler := NewTraceHandlers()
			result, err := handler.HandleManageTrace(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantError {
				if !result.IsError {
					t.Errorf("expected error result, got: %s", result.Content[0].Text)
				}
				if tt.wantContain != "" && !strings.Contains(result.Content[0].Text, tt.wantContain) {
					t.Errorf("error text does not contain %q: %s", tt.wantContain, result.Content[0].Text)
				}
				return
			}

			if result.IsError {
				t.Fatalf("unexpected error result: %s", result.Content[0].Text)
			}
			if capturedData["domain"] != tt.wantDomain {
				t.Errorf("data[domain] = %v, want %q", capturedData["domain"], tt.wantDomain)
			}
			if capturedData["item_id"] != tt.wantItemID {
				t.Errorf("data[item_id] = %v, want %q", capturedData["item_id"], tt.wantItemID)
			}
		})
	}
}

// TestResolveTraceItemID verifies entity_id → trace item_id mapping:
// scripts resolve via the entity registry unique_id (renamed entities keep their
// original unique_id, which is what HA keys traces by), automations via the config ID.
func TestResolveTraceItemID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		domain      string
		entityID    string
		registry    []homeassistant.EntityRegistryEntry
		registryErr error
		wantItem    string
	}{
		{
			name:     "script uses registry unique_id when entity was renamed",
			domain:   "script",
			entityID: "script.bedtime",
			registry: []homeassistant.EntityRegistryEntry{
				{EntityID: "script.bedtime", UniqueID: "labeled_feature_sleep_timeout"},
			},
			wantItem: "labeled_feature_sleep_timeout",
		},
		{
			name:     "script falls back to object_id when registry lookup misses",
			domain:   "script",
			entityID: "script.morning_routine",
			wantItem: "morning_routine",
		},
		{
			name:     "automation uses registry unique_id (config ID)",
			domain:   "automation",
			entityID: "automation.ev_charging",
			registry: []homeassistant.EntityRegistryEntry{
				{EntityID: "automation.ev_charging", UniqueID: "1700000000001"},
			},
			wantItem: "1700000000001",
		},
		{
			name:     "automation falls back to object_id when registry lookup misses",
			domain:   "automation",
			entityID: "automation.ev_charging",
			wantItem: "ev_charging",
		},
		{
			name:        "script falls back to object_id when registry fetch errors",
			domain:      "script",
			entityID:    "script.morning_routine",
			registryErr: errors.New("websocket disconnected"),
			wantItem:    "morning_routine",
		},
		{
			name:        "automation falls back to object_id when registry fetch errors",
			domain:      "automation",
			entityID:    "automation.ev_charging",
			registryErr: errors.New("websocket disconnected"),
			wantItem:    "ev_charging",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return tt.registry, tt.registryErr
				},
			}

			got := resolveTraceItemID(context.Background(), client, tt.domain, tt.entityID)
			if got != tt.wantItem {
				t.Errorf("resolveTraceItemID() = %q, want %q", got, tt.wantItem)
			}
		})
	}
}

func TestManageTrace_List_MissingDomainAndEntityID(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		SendHACSCommandFn: func(context.Context, string, map[string]any) (any, error) {
			t.Fatal("HA should not be called when domain cannot be resolved")
			return nil, nil
		},
	}

	handler := NewTraceHandlers()
	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{"action": "list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "domain") {
		t.Errorf("error text does not contain %q: %s", "domain", result.Content[0].Text)
	}
}

// TestManageTrace_GetMissingParams verifies validation for get action.
func TestManageTrace_GetMissingParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "missing domain",
			args: map[string]any{
				"action":    "get",
				"entity_id": "automation.test",
				"run_id":    "abc123",
			},
		},
		{
			name: "missing entity_id",
			args: map[string]any{
				"action": "get",
				"domain": "automation",
				"run_id": "abc123",
			},
		},
		{
			name: "missing run_id",
			args: map[string]any{
				"action":    "get",
				"domain":    "automation",
				"entity_id": "automation.test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{}
			handler := NewTraceHandlers()

			result, err := handler.HandleManageTrace(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil || len(result.Content) == 0 {
				t.Fatal("expected result with error message")
			}
			if !result.IsError {
				t.Error("expected IsError to be true")
			}
		})
	}
}

// TestManageTrace_List_EmptyMessage verifies that the empty-traces message
// informs the user that traces may not be immediately available.
func TestManageTrace_List_EmptyMessage(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		SendHACSCommandFn: func(_ context.Context, cmd string, _ map[string]any) (any, error) {
			if cmd == "trace/list" {
				return []any{}, nil // empty list
			}
			return nil, fmt.Errorf("wrong command: %s", cmd)
		},
	}

	handler := NewTraceHandlers()
	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action": "list",
		"domain": "automation",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	// Message must mention async lag or suggest retry / wait param
	if !strings.Contains(text, "async") && !strings.Contains(text, "wait") && !strings.Contains(text, "available") {
		t.Errorf("empty trace message should mention async lag or wait param, got: %s", text)
	}
}

// TestManageTrace_List_WaitPolls verifies that wait=true polls trace/list until
// traces appear, returning the non-empty result.
func TestManageTrace_List_WaitPolls(t *testing.T) {
	t.Parallel()

	callCount := 0
	client := &UniversalMockClient{
		SendHACSCommandFn: func(_ context.Context, cmd string, _ map[string]any) (any, error) {
			if cmd != "trace/list" {
				return nil, fmt.Errorf("wrong command: %s", cmd)
			}
			callCount++
			if callCount < 3 {
				return []any{}, nil // empty on first two calls
			}
			return []any{
				map[string]any{"run_id": "trace999", "state": "stopped"},
			}, nil
		},
	}

	// Inject short WaitConfig so the test doesn't take 5s
	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      200 * time.Millisecond,
		PollInterval: 10 * time.Millisecond,
	})

	handler := NewTraceHandlers()
	result, err := handler.HandleManageTrace(ctx, client, map[string]any{
		"action": "list",
		"domain": "automation",
		"wait":   true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "trace999") {
		t.Errorf("expected trace999 in wait=true result, got: %s", text)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 poll calls, got %d", callCount)
	}
}

// TestFormatTraceNatural_ScriptSequenceSteps verifies that nested trace paths under a single
// top-level sequence step (e.g. a repeat/choose block) are counted once, not once per nested
// key - countPrefixedKeys previously counted every distinct dict key, inflating the count for
// any step containing nested action blocks.
func TestFormatTraceNatural_ScriptSequenceSteps(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	response := map[string]any{
		"trace": map[string]any{
			"sequence/0":                       []any{map[string]any{}},
			"sequence/1":                       []any{map[string]any{}},
			"sequence/1/repeat/sequence/0":     []any{map[string]any{}},
			"sequence/1/repeat/sequence/1":     []any{map[string]any{}, map[string]any{}},
			"sequence/2/choose/0/conditions/0": []any{map[string]any{}},
			"sequence/2/choose/0/sequence/0":   []any{map[string]any{}},
		},
	}

	got := handler.formatTraceNatural(response)
	want := "Sequence steps: 3 executed"
	if !strings.Contains(got, want) {
		t.Errorf("output = %q, want it to contain %q (3 top-level steps: 0, 1, 2)", got, want)
	}
}

// TestFormatTraceNatural_Automation verifies automation traces (keyed "action/N"/"condition/N",
// never the "actions"/"conditions" list keys the old code checked) render non-empty output, and
// that the trigger description is read from the top-level short dict, not nested under "trace".
func TestFormatTraceNatural_Automation(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	response := map[string]any{
		// HA's short_dict stores the trigger description as a plain string, computed from
		// trigger variables at run time (automation/__init__.py: set_trigger_description) -
		// never as a nested {"platform":..,"description":..} dict.
		"trigger": "time (06:30:00)",
		"trace": map[string]any{
			"action/0":    []any{map[string]any{}},
			"action/1":    []any{map[string]any{}},
			"condition/0": []any{map[string]any{}},
		},
	}

	got := handler.formatTraceNatural(response)
	for _, want := range []string{"Trigger: time (06:30:00)", "Actions: 2 executed", "Conditions: 1 evaluated"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

// TestFormatTracesNatural_DictTimestamp verifies the list view reads HA's real
// {"start":..,"finish":..} timestamp shape instead of always finding an empty string.
func TestFormatTracesNatural_DictTimestamp(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	traces := []any{
		map[string]any{
			"run_id": "abc123",
			"state":  "stopped",
			"timestamp": map[string]any{
				"start":  "2026-02-19T06:00:00Z",
				"finish": "2026-02-19T06:00:03Z",
			},
		},
	}

	got := handler.formatTracesNatural(traces)
	for _, want := range []string{"Timestamp: 2026-02-19T06:00:00Z", "Duration: 3.00s"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}
