package handlers

import (
	"context"
	"errors"
	"fmt"
	"maps"
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
					"run_id": "abc123",
					"state":  "stopped",
					"timestamp": map[string]any{
						"start":  "2024-01-15T10:30:00Z",
						"finish": "2024-01-15T10:30:02Z",
					},
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
					"run_id": "xyz789",
					"state":  "running",
					"timestamp": map[string]any{
						"start": "2024-01-15T11:00:00Z",
					},
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
		GetEntityRegistryEntryFn: func(_ context.Context, entityID string) (*homeassistant.EntityRegistryEntry, error) {
			return &homeassistant.EntityRegistryEntry{EntityID: entityID, UniqueID: "1700000000001"}, nil
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

// TestHandleGetTrace_ExplicitParamsBackwardCompat pins the get action's explicit
// domain+entity_id+run_id call: the exact trace/get payload and the exact natural and json
// output. Adding automation_id and a latest-run default to get must keep this path unchanged.
func TestHandleGetTrace_ExplicitParamsBackwardCompat(t *testing.T) {
	t.Parallel()

	response := map[string]any{
		"run_id":           "abc123",
		"state":            "stopped",
		"trigger":          "state of input_button.press",
		"script_execution": "finished",
		"trace": map[string]any{
			"condition/0": []any{},
			"action/0":    []any{},
			"action/1":    []any{},
		},
	}

	tests := []struct {
		name   string
		format string
		want   string
	}{
		{
			name:   "natural",
			format: "",
			want: "Trace Execution Path:\n\nState: stopped\n\nTrigger: state of input_button.press" +
				"\n\nResult: finished\n\nConditions: 1 evaluated\n\nActions: 2 executed",
		},
		{
			name:   "json",
			format: "json",
			want: `{
  "run_id": "abc123",
  "script_execution": "finished",
  "state": "stopped",
  "trace": {
    "action/0": [],
    "action/1": [],
    "condition/0": []
  },
  "trigger": "state of input_button.press"
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int
			client := &UniversalMockClient{
				GetEntityRegistryEntryFn: func(_ context.Context, entityID string) (*homeassistant.EntityRegistryEntry, error) {
					return &homeassistant.EntityRegistryEntry{EntityID: entityID, UniqueID: "1700000000001"}, nil
				},
				SendHACSCommandFn: func(_ context.Context, cmd string, data map[string]any) (any, error) {
					calls++
					if cmd != "trace/get" {
						t.Errorf("cmd = %q, want %q", cmd, "trace/get")
					}
					wantData := map[string]any{"domain": "automation", "item_id": "1700000000001", "run_id": "abc123"}
					if fmt.Sprint(data) != fmt.Sprint(wantData) {
						t.Errorf("data = %v, want %v", data, wantData)
					}
					return response, nil
				},
			}

			args := map[string]any{
				"action":    "get",
				"domain":    "automation",
				"entity_id": "automation.test",
				"run_id":    "abc123",
			}
			if tt.format != "" {
				args["format"] = tt.format
			}

			result, err := NewTraceHandlers().HandleManageTrace(context.Background(), client, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if calls != 1 {
				t.Errorf("SendHACSCommand calls = %d, want 1 (no extra trace/list lookup)", calls)
			}
			if result.IsError {
				t.Fatalf("unexpected error result: %s", result.Content[0].Text)
			}
			if len(result.Content) != 1 {
				t.Fatalf("len(Content) = %d, want 1", len(result.Content))
			}
			if got := result.Content[0].Text; got != tt.want {
				t.Errorf("output mismatch\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
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

// TestManageTrace_Get_RejectsMalformedOrUnsupportedEntityID covers entity_id shapes the get
// action's inline prefix check used to accept unchecked: a bare id (no dot), an id starting with
// ".", and a prefix that matches the supplied domain but isn't automation/script (the get action
// never validated domain against the automation/script enum itself, so a caller-supplied
// domain="sensor" alongside entity_id="sensor.foo" previously sailed through undetected since the
// prefixes trivially agreed).
func TestManageTrace_Get_RejectsMalformedOrUnsupportedEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		domain   string
		entityID string
	}{
		{name: "bare id with no domain prefix", domain: "automation", entityID: "morning_routine"},
		{name: "id starting with a dot", domain: "automation", entityID: ".morning_routine"},
		{name: "prefix matches domain but neither is automation/script", domain: "sensor", entityID: "sensor.foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				SendHACSCommandFn: func(context.Context, string, map[string]any) (any, error) {
					t.Fatal("trace/get should not be called for a malformed or unsupported entity_id")
					return nil, nil
				},
			}

			handler := NewTraceHandlers()
			result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
				"action":    "get",
				"domain":    tt.domain,
				"entity_id": tt.entityID,
				"run_id":    "abc123",
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected error result, got: %s", result.Content[0].Text)
			}
			if !strings.Contains(result.Content[0].Text, "entity_id") {
				t.Errorf("error text does not mention entity_id: %s", result.Content[0].Text)
			}
		})
	}
}

// TestManageTrace_List_EntityIDFilter verifies that passing entity_id to the list
// action automatically derives the domain and sets item_id for server-side filtering.
// Regression test for a bug where entity_id was silently ignored, causing the WS call
// to omit domain and return "id @ data['domain']. Got None".
func TestManageTrace_List_EntityIDFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          map[string]any
		registryEntry *homeassistant.EntityRegistryEntry
		wantDomain    string
		wantItemID    string
		wantError     bool
		wantContain   string
	}{
		{
			// unique_id deliberately differs from the object_id ("ev_charging") so this test
			// fails if resolveTraceItemID is ever replaced by a naive entity_id split - the
			// pre-fix regression this test exists to catch (see TestResolveTraceItemID).
			name:          "entity_id automation prefix derives domain and resolves item_id via registry",
			args:          map[string]any{"action": "list", "entity_id": "automation.ev_charging"},
			registryEntry: &homeassistant.EntityRegistryEntry{EntityID: "automation.ev_charging", UniqueID: "1700000000001"},
			wantDomain:    "automation",
			wantItemID:    "1700000000001",
		},
		{
			name:          "entity_id script prefix derives domain and resolves item_id via registry",
			args:          map[string]any{"action": "list", "entity_id": "script.morning_routine"},
			registryEntry: &homeassistant.EntityRegistryEntry{EntityID: "script.morning_routine", UniqueID: "labeled_feature_sleep_timeout"},
			wantDomain:    "script",
			wantItemID:    "labeled_feature_sleep_timeout",
		},
		{
			// No registry entry - a registry miss falls back to the object_id. Asserted
			// explicitly as the degraded path, not accidentally exercised by every case.
			name:       "entity_id falls back to object_id when registry lookup misses",
			args:       map[string]any{"action": "list", "entity_id": "automation.ev_charging"},
			wantDomain: "automation",
			wantItemID: "ev_charging",
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
				GetEntityRegistryEntryFn: func(_ context.Context, entityID string) (*homeassistant.EntityRegistryEntry, error) {
					if tt.registryEntry == nil {
						return nil, fmt.Errorf("entity %q not found", entityID)
					}
					return tt.registryEntry, nil
				},
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
		name          string
		domain        string
		entityID      string
		registryEntry *homeassistant.EntityRegistryEntry
		registryErr   error
		wantItem      string
		wantResolved  bool
	}{
		{
			name:          "script uses registry unique_id when entity was renamed",
			domain:        "script",
			entityID:      "script.bedtime",
			registryEntry: &homeassistant.EntityRegistryEntry{EntityID: "script.bedtime", UniqueID: "labeled_feature_sleep_timeout"},
			wantItem:      "labeled_feature_sleep_timeout",
			wantResolved:  true,
		},
		{
			name:         "script falls back to object_id when registry lookup misses",
			domain:       "script",
			entityID:     "script.morning_routine",
			registryErr:  errors.New("entity not found"),
			wantItem:     "morning_routine",
			wantResolved: false,
		},
		{
			name:          "automation uses registry unique_id (config ID)",
			domain:        "automation",
			entityID:      "automation.ev_charging",
			registryEntry: &homeassistant.EntityRegistryEntry{EntityID: "automation.ev_charging", UniqueID: "1700000000001"},
			wantItem:      "1700000000001",
			wantResolved:  true,
		},
		{
			name:         "automation falls back to object_id when registry lookup misses",
			domain:       "automation",
			entityID:     "automation.ev_charging",
			registryErr:  errors.New("entity not found"),
			wantItem:     "ev_charging",
			wantResolved: false,
		},
		{
			name:         "script falls back to object_id when registry fetch errors",
			domain:       "script",
			entityID:     "script.morning_routine",
			registryErr:  errors.New("websocket disconnected"),
			wantItem:     "morning_routine",
			wantResolved: false,
		},
		{
			name:         "automation falls back to object_id when registry fetch errors",
			domain:       "automation",
			entityID:     "automation.ev_charging",
			registryErr:  errors.New("websocket disconnected"),
			wantItem:     "ev_charging",
			wantResolved: false,
		},
		{
			name:          "falls back to object_id when the entry has no unique_id",
			domain:        "automation",
			entityID:      "automation.ev_charging",
			registryEntry: &homeassistant.EntityRegistryEntry{EntityID: "automation.ev_charging"},
			wantItem:      "ev_charging",
			wantResolved:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				GetEntityRegistryEntryFn: func(_ context.Context, _ string) (*homeassistant.EntityRegistryEntry, error) {
					return tt.registryEntry, tt.registryErr
				},
			}

			got, resolved := resolveTraceItemID(context.Background(), client, tt.domain, tt.entityID)
			if got != tt.wantItem {
				t.Errorf("resolveTraceItemID() item = %q, want %q", got, tt.wantItem)
			}
			if resolved != tt.wantResolved {
				t.Errorf("resolveTraceItemID() resolved = %v, want %v", resolved, tt.wantResolved)
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
// A missing domain (derived from the entity_id prefix) and a missing run_id (defaults to the
// latest run) are no longer errors - see TestResolveTraceGetTarget and
// TestManageTrace_Get_LatestRun. Only a missing target remains one.
func TestManageTrace_GetMissingParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "missing entity_id and automation_id",
			args: map[string]any{
				"action": "get",
				"domain": "automation",
				"run_id": "abc123",
			},
		},
		{
			name: "missing entity_id, automation_id and run_id",
			args: map[string]any{
				"action": "get",
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
			if !strings.Contains(result.Content[0].Text, "automation_id") {
				t.Errorf("error text should name automation_id as an alternative: %s", result.Content[0].Text)
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

// TestManageTrace_List_UnresolvedItemIDWarning verifies that an empty result following a failed
// item_id resolution surfaces a distinct warning (not the generic "may not be available yet"
// message that tells the caller to retry something that can never succeed), and that wait=true
// does not poll a key already known to be wrong.
func TestManageTrace_List_UnresolvedItemIDWarning(t *testing.T) {
	t.Parallel()

	callCount := 0
	client := &UniversalMockClient{
		// No GetEntityRegistryEntryFn - default mock behavior returns "not found", so the lookup always misses.
		SendHACSCommandFn: func(_ context.Context, cmd string, _ map[string]any) (any, error) {
			if cmd != "trace/list" {
				return nil, fmt.Errorf("wrong command: %s", cmd)
			}
			callCount++
			return []any{}, nil
		},
	}

	ctx := mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      200 * time.Millisecond,
		PollInterval: 10 * time.Millisecond,
	})

	handler := NewTraceHandlers()
	result, err := handler.HandleManageTrace(ctx, client, map[string]any{
		"action":    "list",
		"entity_id": "automation.ev_charging",
		"wait":      true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "could not be resolved") {
		t.Errorf("expected the resolution-failure warning, got: %s", text)
	}
	if strings.Contains(text, "pass wait=true to poll automatically") {
		t.Errorf("expected the unresolved-item_id warning, not the generic wait=true suggestion: %s", text)
	}
	if callCount != 1 {
		t.Errorf("trace/list called %d times, want 1 (wait polling must be skipped when item_id resolution failed)", callCount)
	}
}

// TestManageTrace_Get_UnresolvedItemIDWarning verifies the get action surfaces the same
// resolution-failure warning as list when trace/get returns nothing for an unresolved item_id
// (trace/get, like trace/list, returns an empty result for an unknown key rather than an error).
func TestManageTrace_Get_UnresolvedItemIDWarning(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		// No GetEntityRegistryEntryFn - lookup misses.
		SendHACSCommandFn: func(_ context.Context, cmd string, _ map[string]any) (any, error) {
			if cmd != "trace/get" {
				return nil, fmt.Errorf("wrong command: %s", cmd)
			}
			return nil, nil
		},
	}

	handler := NewTraceHandlers()
	result, err := handler.HandleManageTrace(context.Background(), client, map[string]any{
		"action":    "get",
		"domain":    "automation",
		"entity_id": "automation.ev_charging",
		"run_id":    "abc123",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "could not be resolved") {
		t.Errorf("expected the resolution-failure warning, got: %s", result.Content[0].Text)
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

// TestFormatTraceNatural_ErrorAndScriptExecution verifies that "state", "script_execution",
// "not_triggered", and "error" - fields on HA's top-level short dict (as_extended_dict includes
// as_short_dict verbatim, components/trace/models.py) - are surfaced. Before this test these
// fields were silently dropped even though the get action's raw response always carries them.
func TestFormatTraceNatural_ErrorAndScriptExecution(t *testing.T) {
	t.Parallel()

	handler := NewTraceHandlers()
	response := map[string]any{
		"state":            "stopped",
		"script_execution": "failed_conditions",
		"not_triggered":    true,
		"error":            "Message malformed: extra keys not allowed @ data['foo']",
		"trace":            map[string]any{},
	}

	got := handler.formatTraceNatural(response)
	for _, want := range []string{
		"State: stopped",
		"Result: failed_conditions",
		"Not triggered: condition evaluated but did not fire",
		"Error: Message malformed: extra keys not allowed @ data['foo']",
	} {
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

	got := handler.formatTracesNatural(traces, "")
	for _, want := range []string{"Timestamp: 2026-02-19T06:00:00Z", "Duration: 3.00s"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

// TestResolveTraceGetTarget covers how the get action turns automation_id / entity_id / domain
// into one trace target: automation_id is an alternative spelling of an automation entity_id,
// the domain is derived from the prefix, and any disagreement between the three is an error
// rather than a silent pick.
func TestResolveTraceGetTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		wantDomain   string
		wantEntityID string
		wantErr      string // substring; "" means success
	}{
		{
			name:         "bare automation_id",
			args:         map[string]any{"automation_id": "morning_routine"},
			wantDomain:   "automation",
			wantEntityID: "automation.morning_routine",
		},
		{
			name:         "prefixed automation_id",
			args:         map[string]any{"automation_id": "automation.morning_routine"},
			wantDomain:   "automation",
			wantEntityID: "automation.morning_routine",
		},
		{
			name:         "entity_id without domain derives it",
			args:         map[string]any{"entity_id": "script.wake_up"},
			wantDomain:   "script",
			wantEntityID: "script.wake_up",
		},
		{
			name:         "explicit matching domain and entity_id",
			args:         map[string]any{"domain": "automation", "entity_id": "automation.test"},
			wantDomain:   "automation",
			wantEntityID: "automation.test",
		},
		{
			name:         "automation_id agreeing with entity_id",
			args:         map[string]any{"automation_id": "test", "entity_id": "automation.test"},
			wantDomain:   "automation",
			wantEntityID: "automation.test",
		},
		{
			name:    "automation_id conflicting with entity_id",
			args:    map[string]any{"automation_id": "test", "entity_id": "automation.other"},
			wantErr: "conflicts with entity_id",
		},
		{
			name:    "automation_id with domain script",
			args:    map[string]any{"automation_id": "test", "domain": "script"},
			wantErr: "conflicts with explicit domain",
		},
		{
			name:    "automation_id with a non-automation prefix",
			args:    map[string]any{"automation_id": "light.kitchen"},
			wantErr: "automation_id",
		},
		{
			name:    "no target at all",
			args:    map[string]any{"domain": "automation"},
			wantErr: "entity_id or automation_id is required",
		},
		{
			name:    "entity_id conflicting with domain",
			args:    map[string]any{"domain": "automation", "entity_id": "script.foo"},
			wantErr: "conflicts with explicit domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			domain, entityID, errMsg := resolveTraceGetTarget(tt.args)
			if tt.wantErr != "" {
				if !strings.Contains(errMsg, tt.wantErr) {
					t.Errorf("errMsg = %q, want it to contain %q", errMsg, tt.wantErr)
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected errMsg: %s", errMsg)
			}
			if domain != tt.wantDomain || entityID != tt.wantEntityID {
				t.Errorf("got (%q, %q), want (%q, %q)", domain, entityID, tt.wantDomain, tt.wantEntityID)
			}
		})
	}
}

// TestLatestTraceEntry verifies the newest-by-start selection independent of list order
// (HA returns traces oldest first, but the selection must not rely on that either way).
func TestLatestTraceEntry(t *testing.T) {
	t.Parallel()

	trace := func(runID, start string) map[string]any {
		return map[string]any{"run_id": runID, "timestamp": map[string]any{"start": start}}
	}

	tests := []struct {
		name      string
		traces    []any
		wantRunID string // "" means nil result
	}{
		{name: "nil list", traces: nil},
		{name: "only non-map entries", traces: []any{"junk", 42}},
		{
			name: "newest in the middle",
			traces: []any{
				trace("r1", "2026-10-29T08:00:00Z"),
				trace("r3", "2026-10-29T10:00:00Z"),
				trace("r2", "2026-10-29T09:00:00Z"),
			},
			wantRunID: "r3",
		},
		{
			name:      "non-map entries are skipped",
			traces:    []any{"junk", trace("r1", "2026-10-29T08:00:00Z")},
			wantRunID: "r1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := latestTraceEntry(tt.traces)
			if tt.wantRunID == "" {
				if got != nil {
					t.Errorf("latestTraceEntry = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("latestTraceEntry = nil, want run_id %q", tt.wantRunID)
			}
			if runID := getMapString(got, "run_id", ""); runID != tt.wantRunID {
				t.Errorf("run_id = %q, want %q", runID, tt.wantRunID)
			}
		})
	}
}

// TestManageTrace_Get_LatestRun verifies that get without run_id lists the traces under the same
// resolved item_id, fetches the newest one, and says which run it picked.
func TestManageTrace_Get_LatestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		wantOut []string
	}{
		{
			name:    "bare automation_id, natural",
			args:    map[string]any{"automation_id": "morning_routine"},
			wantOut: []string{"Run ID: r3 (latest)\n\nTrace Execution Path:", "State: stopped"},
		},
		{
			name:    "prefixed automation_id, json",
			args:    map[string]any{"automation_id": "automation.morning_routine", "format": "json"},
			wantOut: []string{`"selected_run_id": "r3"`, `"run_id": "r3"`, `"state": "stopped"`},
		},
		{
			name:    "entity_id without domain or run_id",
			args:    map[string]any{"entity_id": "automation.morning_routine"},
			wantOut: []string{"Run ID: r3 (latest)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cmds []string
			client := &UniversalMockClient{
				GetEntityRegistryEntryFn: func(_ context.Context, entityID string) (*homeassistant.EntityRegistryEntry, error) {
					if entityID != "automation.morning_routine" {
						t.Errorf("registry lookup for %q, want automation.morning_routine", entityID)
					}
					return &homeassistant.EntityRegistryEntry{EntityID: entityID, UniqueID: "1700000000001"}, nil
				},
				SendHACSCommandFn: func(_ context.Context, cmd string, data map[string]any) (any, error) {
					cmds = append(cmds, cmd)
					switch cmd {
					case "trace/list":
						wantData := map[string]any{"domain": "automation", "item_id": "1700000000001"}
						if fmt.Sprint(data) != fmt.Sprint(wantData) {
							t.Errorf("trace/list data = %v, want %v", data, wantData)
						}
						return []any{
							map[string]any{"run_id": "r1", "timestamp": map[string]any{"start": "2026-10-29T08:00:00Z"}},
							map[string]any{"run_id": "r3", "timestamp": map[string]any{"start": "2026-10-29T10:00:00Z"}},
							map[string]any{"run_id": "r2", "timestamp": map[string]any{"start": "2026-10-29T09:00:00Z"}},
						}, nil
					case "trace/get":
						wantData := map[string]any{"domain": "automation", "item_id": "1700000000001", "run_id": "r3"}
						if fmt.Sprint(data) != fmt.Sprint(wantData) {
							t.Errorf("trace/get data = %v, want %v", data, wantData)
						}
						return map[string]any{"run_id": "r3", "state": "stopped"}, nil
					}
					return nil, fmt.Errorf("wrong command: %s", cmd)
				},
			}

			args := map[string]any{"action": "get"}
			maps.Copy(args, tt.args)
			result, err := NewTraceHandlers().HandleManageTrace(context.Background(), client, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("unexpected error result: %s", result.Content[0].Text)
			}
			if fmt.Sprint(cmds) != "[trace/list trace/get]" {
				t.Errorf("commands = %v, want [trace/list trace/get]", cmds)
			}
			text := result.Content[0].Text
			for _, want := range tt.wantOut {
				if !strings.Contains(text, want) {
					t.Errorf("output missing %q\nfull output:\n%s", want, text)
				}
			}
		})
	}
}

// TestManageTrace_Get_LatestRunNoTraces verifies that get without run_id and an empty trace list
// returns an explanatory (non-error) message without calling trace/get, and that a failed item_id
// resolution gets the unresolved-item_id warning instead of the generic "not recorded yet" text.
func TestManageTrace_Get_LatestRunNoTraces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resolved bool
		format   string
		want     string
		dontWant string
	}{
		{name: "resolved, natural", resolved: true, want: "No traces found for automation.ev_charging", dontWant: "could not be resolved"},
		{name: "resolved, json", resolved: true, format: "json", want: `"warning": "No traces found for automation.ev_charging`, dontWant: "could not be resolved"},
		{name: "unresolved, natural", want: "could not be resolved"},
		{name: "unresolved, json", format: "json", want: "could not be resolved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				SendHACSCommandFn: func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "trace/list" {
						t.Errorf("unexpected command %q: trace/get must not run without a trace", cmd)
					}
					return []any{}, nil
				},
			}
			if tt.resolved {
				client.GetEntityRegistryEntryFn = func(_ context.Context, entityID string) (*homeassistant.EntityRegistryEntry, error) {
					return &homeassistant.EntityRegistryEntry{EntityID: entityID, UniqueID: "ev_charging"}, nil
				}
			}

			args := map[string]any{"action": "get", "automation_id": "ev_charging"}
			if tt.format != "" {
				args["format"] = tt.format
			}
			result, err := NewTraceHandlers().HandleManageTrace(context.Background(), client, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("unexpected error result: %s", result.Content[0].Text)
			}
			text := result.Content[0].Text
			if !strings.Contains(text, tt.want) {
				t.Errorf("output missing %q\nfull output:\n%s", tt.want, text)
			}
			if tt.dontWant != "" && strings.Contains(text, tt.dontWant) {
				t.Errorf("output must not contain %q\nfull output:\n%s", tt.dontWant, text)
			}
		})
	}
}

// TestManageTrace_Get_LatestRunListError verifies a failing trace/list surfaces as an error result.
func TestManageTrace_Get_LatestRunListError(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		SendHACSCommandFn: func(_ context.Context, cmd string, _ map[string]any) (any, error) {
			if cmd != "trace/list" {
				t.Errorf("unexpected command %q", cmd)
			}
			return nil, errors.New("connection lost")
		},
	}

	result, err := NewTraceHandlers().HandleManageTrace(context.Background(), client, map[string]any{
		"action":        "get",
		"automation_id": "ev_charging",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "failed to list traces") {
		t.Errorf("error text = %q, want it to mention the trace/list failure", result.Content[0].Text)
	}
}
