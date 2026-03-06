package mcp

import (
	"strings"
	"testing"
)

// TestValidateFilterConfig verifies startup validation rejects invalid filter entries.
func TestValidateFilterConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         ToolFilterConfig
		wantErr     bool
		errContains []string
	}{
		{
			name:    "disabled (empty)",
			cfg:     ToolFilterConfig{},
			wantErr: false,
		},
		{
			name:    "valid whitelist",
			cfg:     ToolFilterConfig{Whitelist: []string{"get_state", "manage_automation:list"}},
			wantErr: false,
		},
		{
			name:    "valid blacklist glob",
			cfg:     ToolFilterConfig{Blacklist: []string{"manage_*:delete"}},
			wantErr: false,
		},
		{
			name:    "category expansion",
			cfg:     ToolFilterConfig{Blacklist: []string{"*:write"}},
			wantErr: false,
		},
		{
			name:    "bare wildcard",
			cfg:     ToolFilterConfig{Blacklist: []string{"*"}},
			wantErr: false,
		},
		{
			name:        "old sub-action (removed)",
			cfg:         ToolFilterConfig{Blacklist: []string{"query_entities:health:remove"}},
			wantErr:     true,
			errContains: []string{"query_entities", "sub-action", "remove"},
		},
		{
			name:        "nonexistent tool",
			cfg:         ToolFilterConfig{Blacklist: []string{"manage_nonexistent"}},
			wantErr:     true,
			errContains: []string{"manage_nonexistent", "no tools match"},
		},
		{
			name:        "nonexistent action",
			cfg:         ToolFilterConfig{Blacklist: []string{"manage_entity:frobnicate"}},
			wantErr:     true,
			errContains: []string{"manage_entity", "frobnicate"},
		},
		{
			name:        "pure tool with action specified",
			cfg:         ToolFilterConfig{Blacklist: []string{"get_state:list"}},
			wantErr:     true,
			errContains: []string{"get_state", "no action parameter"},
		},
		{
			name:        "glob action on no matching tool",
			cfg:         ToolFilterConfig{Blacklist: []string{"get_*:create"}},
			wantErr:     true,
			errContains: []string{"create", "get_*"},
		},
		{
			name:        "multiple errors reported together",
			cfg:         ToolFilterConfig{Blacklist: []string{"bad_tool", "manage_entity:bad"}},
			wantErr:     true,
			errContains: []string{"bad_tool", "manage_entity", "bad"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateFilterConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilterConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				for _, want := range tt.errContains {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("expected error to contain %q, got: %s", want, err.Error())
					}
				}
			}
		})
	}
}

// TestToolFilterEngine_ReadOnly verifies read-only mode blocks all write operations.
func TestToolFilterEngine_ReadOnly(t *testing.T) {
	t.Parallel()

	cfg := ToolFilterConfig{}
	filter := NewToolFilterEngine(cfg, true) // readOnly = true

	if !filter.IsEnabled() {
		t.Fatal("Filter should be enabled in read-only mode")
	}

	// Test write actions are blocked
	if filter.IsActionAllowed("call_service", nil) {
		t.Error("call_service (pure write) should be blocked in read-only mode")
	}

	if filter.IsActionAllowed("manage_automation", map[string]any{"action": "create"}) {
		t.Error("manage_automation:create should be blocked in read-only mode")
	}

	// Test read actions are allowed
	if !filter.IsActionAllowed("get_state", nil) {
		t.Error("get_state (pure read) should be allowed in read-only mode")
	}

	if !filter.IsActionAllowed("manage_automation", map[string]any{"action": "list"}) {
		t.Error("manage_automation:list should be allowed in read-only mode")
	}
}

// TestToolFilterEngine_Blacklist_Tool verifies blacklisting entire tools.
func TestToolFilterEngine_Blacklist_Tool(t *testing.T) {
	t.Parallel()

	cfg := ToolFilterConfig{
		Blacklist: []string{"get_state", "manage_automation"},
	}
	filter := NewToolFilterEngine(cfg, false)

	// Blacklisted tools should be blocked
	if filter.IsActionAllowed("get_state", nil) {
		t.Error("get_state should be blocked by blacklist")
	}

	if filter.IsActionAllowed("manage_automation", map[string]any{"action": "list"}) {
		t.Error("manage_automation should be blocked by blacklist")
	}

	// Non-blacklisted tools should be allowed
	if !filter.IsActionAllowed("get_datetime", nil) {
		t.Error("get_datetime should be allowed (not blacklisted)")
	}
}

// TestToolFilterEngine_Blacklist_Action verifies blacklisting specific actions.
func TestToolFilterEngine_Blacklist_Action(t *testing.T) {
	t.Parallel()

	cfg := ToolFilterConfig{
		Blacklist: []string{"manage_automation:create", "manage_automation:delete"},
	}
	filter := NewToolFilterEngine(cfg, false)

	// Blacklisted actions should be blocked
	if filter.IsActionAllowed("manage_automation", map[string]any{"action": "create"}) {
		t.Error("manage_automation:create should be blocked")
	}

	if filter.IsActionAllowed("manage_automation", map[string]any{"action": "delete"}) {
		t.Error("manage_automation:delete should be blocked")
	}

	// Other actions should be allowed
	if !filter.IsActionAllowed("manage_automation", map[string]any{"action": "list"}) {
		t.Error("manage_automation:list should be allowed")
	}

	if !filter.IsActionAllowed("manage_automation", map[string]any{"action": "get"}) {
		t.Error("manage_automation:get should be allowed")
	}
}

// TestToolFilterEngine_Blacklist_Glob verifies glob pattern matching.
func TestToolFilterEngine_Blacklist_Glob(t *testing.T) {
	t.Parallel()

	cfg := ToolFilterConfig{
		Blacklist: []string{"manage_*:delete"},
	}
	filter := NewToolFilterEngine(cfg, false)

	// All manage_* tools' delete actions should be blocked
	managedTools := []string{
		"manage_automation",
		"manage_script",
		"manage_scene",
		"manage_area",
		"manage_label",
	}

	for _, tool := range managedTools {
		if filter.IsActionAllowed(tool, map[string]any{"action": "delete"}) {
			t.Errorf("%s:delete should be blocked by glob pattern", tool)
		}

		// Other actions should still work
		if !filter.IsActionAllowed(tool, map[string]any{"action": "list"}) {
			t.Errorf("%s:list should be allowed", tool)
		}
	}

	// Non-matching tools should not be affected
	if !filter.IsActionAllowed("get_state", nil) {
		t.Error("get_state should not be affected by manage_* glob")
	}
}

// TestToolFilterEngine_Blacklist_Category verifies category-based filtering.
func TestToolFilterEngine_Blacklist_Category(t *testing.T) {
	t.Parallel()

	cfg := ToolFilterConfig{
		Blacklist: []string{"*:write"},
	}
	filter := NewToolFilterEngine(cfg, false)

	// All write operations should be blocked
	if filter.IsActionAllowed("call_service", nil) {
		t.Error("call_service (pure write) should be blocked by *:write")
	}

	if filter.IsActionAllowed("manage_automation", map[string]any{"action": "create"}) {
		t.Error("manage_automation:create should be blocked by *:write")
	}

	if filter.IsActionAllowed("manage_script", map[string]any{"action": "execute"}) {
		t.Error("manage_script:execute should be blocked by *:write")
	}

	// Read operations should be allowed
	if !filter.IsActionAllowed("get_state", nil) {
		t.Error("get_state (pure read) should be allowed")
	}

	if !filter.IsActionAllowed("manage_automation", map[string]any{"action": "list"}) {
		t.Error("manage_automation:list should be allowed")
	}
}

// TestToolFilterEngine_Whitelist verifies whitelist-based filtering.
func TestToolFilterEngine_Whitelist(t *testing.T) {
	t.Parallel()

	cfg := ToolFilterConfig{
		Whitelist: []string{"get_state", "manage_automation:list", "manage_automation:get"},
	}
	filter := NewToolFilterEngine(cfg, false)

	// Whitelisted items should be allowed
	if !filter.IsActionAllowed("get_state", nil) {
		t.Error("get_state should be allowed (whitelisted)")
	}

	if !filter.IsActionAllowed("manage_automation", map[string]any{"action": "list"}) {
		t.Error("manage_automation:list should be allowed (whitelisted)")
	}

	if !filter.IsActionAllowed("manage_automation", map[string]any{"action": "get"}) {
		t.Error("manage_automation:get should be allowed (whitelisted)")
	}

	// Non-whitelisted items should be blocked
	if filter.IsActionAllowed("get_datetime", nil) {
		t.Error("get_datetime should be blocked (not whitelisted)")
	}

	if filter.IsActionAllowed("manage_automation", map[string]any{"action": "create"}) {
		t.Error("manage_automation:create should be blocked (not whitelisted)")
	}
}

// TestToolFilterEngine_ReadOnly_Composition verifies read-only + user blacklist composition.
func TestToolFilterEngine_ReadOnly_Composition(t *testing.T) {
	t.Parallel()

	cfg := ToolFilterConfig{
		Blacklist: []string{"get_state"},
	}
	filter := NewToolFilterEngine(cfg, true) // readOnly + blacklist

	// Write operations blocked by read-only
	if filter.IsActionAllowed("call_service", nil) {
		t.Error("call_service should be blocked by read-only")
	}

	// Read operation blocked by user blacklist
	if filter.IsActionAllowed("get_state", nil) {
		t.Error("get_state should be blocked by user blacklist")
	}

	// Other read operations allowed
	if !filter.IsActionAllowed("get_datetime", nil) {
		t.Error("get_datetime should be allowed")
	}
}

// TestToolFilterEngine_ApplyToRegistry_RemoveTool verifies complete tool removal.
func TestToolFilterEngine_ApplyToRegistry_RemoveTool(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterTool(Tool{Name: "get_state"}, nil)
	registry.RegisterTool(Tool{Name: "call_service"}, nil)
	registry.RegisterTool(Tool{Name: "get_datetime"}, nil)

	cfg := ToolFilterConfig{
		Blacklist: []string{"get_state"},
	}
	filter := NewToolFilterEngine(cfg, false)

	removed := filter.ApplyToRegistry(registry)

	if removed != 1 {
		t.Errorf("Expected 1 tool removed, got %d", removed)
	}

	// Verify get_state is removed
	_, exists := registry.GetTool("get_state")
	if exists {
		t.Error("get_state should be removed from registry")
	}

	// Verify other tools remain
	_, exists = registry.GetTool("call_service")
	if !exists {
		t.Error("call_service should still exist")
	}

	_, exists = registry.GetTool("get_datetime")
	if !exists {
		t.Error("get_datetime should still exist")
	}
}

// TestToolFilterEngine_ApplyToRegistry_ModifySchema verifies schema modification for partial blocks.
func TestToolFilterEngine_ApplyToRegistry_ModifySchema(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	originalTool := Tool{
		Name:        "manage_automation",
		Description: "Manage Home Assistant automations - list, get, create, update, delete, toggle, coverage, or patch.\n\nActions:\n- list: List all automations\n- get: Get automation details\n- create: Create new automation\n- update: Update automation\n- delete: Delete automation\n- toggle: Enable/disable automation\n- coverage: Analyze automation coverage\n- patch: Apply JSON Patch operations",
		InputSchema: JSONSchema{
			Type: "object",
			Properties: map[string]JSONSchema{
				"action": {
					Type:        "string",
					Enum:        []string{"list", "get", "create", "update", "delete", "toggle", "coverage", "patch"},
					Description: "Operation to perform: list, get, create, update, delete, toggle, coverage, patch",
				},
			},
		},
	}
	registry.RegisterTool(originalTool, nil)

	cfg := ToolFilterConfig{
		Blacklist: []string{"manage_automation:create", "manage_automation:update", "manage_automation:delete"},
	}
	filter := NewToolFilterEngine(cfg, false)

	removed := filter.ApplyToRegistry(registry)

	if removed != 0 {
		t.Errorf("Expected 0 tools removed (partial block), got %d", removed)
	}

	// Verify tool still exists but is modified
	modifiedTool, exists := registry.GetTool("manage_automation")
	if !exists {
		t.Fatal("manage_automation should still exist in registry")
	}

	// Check enum is filtered (create, update, delete removed; patch kept as it's not blacklisted)
	actionEnum := modifiedTool.InputSchema.Properties["action"].Enum
	expectedEnum := []string{"list", "get", "toggle", "coverage", "patch"}

	if len(actionEnum) != len(expectedEnum) {
		t.Errorf("Action enum length = %d, want %d", len(actionEnum), len(expectedEnum))
	}

	for _, expected := range expectedEnum {
		found := false
		for _, actual := range actionEnum {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected action %q not found in enum", expected)
		}
	}

	// Check description is updated
	if strings.Contains(modifiedTool.Description, "create") {
		t.Error("Tool description should not contain 'create' (blocked action)")
	}

	if !strings.Contains(modifiedTool.Description, "list") {
		t.Error("Tool description should contain 'list' (allowed action)")
	}
}

// TestToolFilterEngine_IsActionAllowed_ManageEntityDelete verifies manage_entity delete filtering.
func TestToolFilterEngine_IsActionAllowed_ManageEntityDelete(t *testing.T) {
	t.Parallel()

	cfg := ToolFilterConfig{
		Blacklist: []string{"manage_entity:delete"},
	}
	filter := NewToolFilterEngine(cfg, false)

	// delete should be blocked
	if filter.IsActionAllowed("manage_entity", map[string]any{"action": "delete"}) {
		t.Error("manage_entity:delete should be blocked")
	}

	// get and update should still be allowed
	if !filter.IsActionAllowed("manage_entity", map[string]any{"action": "get"}) {
		t.Error("manage_entity:get should be allowed")
	}

	if !filter.IsActionAllowed("manage_entity", map[string]any{"action": "update"}) {
		t.Error("manage_entity:update should be allowed")
	}
}

// TestToolFilterEngine_Disabled verifies disabled filter allows everything.
func TestToolFilterEngine_Disabled(t *testing.T) {
	t.Parallel()

	cfg := ToolFilterConfig{}
	filter := NewToolFilterEngine(cfg, false)

	if filter.IsEnabled() {
		t.Error("Filter should be disabled with empty config and readOnly=false")
	}

	// All operations should be allowed
	if !filter.IsActionAllowed("call_service", nil) {
		t.Error("call_service should be allowed when filter is disabled")
	}

	if !filter.IsActionAllowed("manage_automation", map[string]any{"action": "delete"}) {
		t.Error("manage_automation:delete should be allowed when filter is disabled")
	}
}

// TestToolFilterEngine_ApplyToRegistry_ReadOnly verifies read-only removes all write tools.
func TestToolFilterEngine_ApplyToRegistry_ReadOnly(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterTool(Tool{Name: "get_state"}, nil)
	registry.RegisterTool(Tool{Name: "call_service"}, nil)
	registry.RegisterTool(Tool{Name: "helper_action"}, nil)
	registry.RegisterTool(Tool{Name: "get_datetime"}, nil)

	cfg := ToolFilterConfig{}
	filter := NewToolFilterEngine(cfg, true) // readOnly = true

	removed := filter.ApplyToRegistry(registry)

	// Should remove pure write tools
	if removed < 2 {
		t.Errorf("Expected at least 2 tools removed (call_service, helper_action), got %d", removed)
	}

	// Verify write tools are removed
	_, exists := registry.GetTool("call_service")
	if exists {
		t.Error("call_service should be removed in read-only mode")
	}

	_, exists = registry.GetTool("helper_action")
	if exists {
		t.Error("helper_action should be removed in read-only mode")
	}

	// Verify read tools remain
	_, exists = registry.GetTool("get_state")
	if !exists {
		t.Error("get_state should remain in read-only mode")
	}
}
