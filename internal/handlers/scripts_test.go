package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockScriptClient implements homeassistant.Client for testing.
type mockScriptClient struct {
	homeassistant.Client
	listScriptsFn  func(ctx context.Context) ([]homeassistant.Entity, error)
	getScriptFn    func(ctx context.Context, scriptID string) (*homeassistant.Script, error)
	createScriptFn func(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error
	updateScriptFn func(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error
	deleteScriptFn func(ctx context.Context, scriptID string) error
	callServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
	getStateFn     func(ctx context.Context, entityID string) (*homeassistant.Entity, error)

	// Track IDs and configs passed to methods for verification
	lastGetScriptID    string
	lastUpdateScriptID string
	lastUpdateConfig   *homeassistant.ScriptConfig
	lastDeleteScriptID string
	lastServiceData    map[string]any

	// entityDeleted tracks whether DeleteScript was successfully called.
	// Used to make GetState return "not found" after delete (for fast waitForEntityDisappear in tests).
	entityDeleted bool
}

func (m *mockScriptClient) ListScripts(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.listScriptsFn != nil {
		return m.listScriptsFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *mockScriptClient) GetScript(ctx context.Context, scriptID string) (*homeassistant.Script, error) {
	m.lastGetScriptID = scriptID
	if m.getScriptFn != nil {
		return m.getScriptFn(ctx, scriptID)
	}
	return &homeassistant.Script{}, nil
}

func (m *mockScriptClient) CreateScript(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error {
	if m.createScriptFn != nil {
		err := m.createScriptFn(ctx, scriptID, config)
		if err == nil {
			m.entityDeleted = false
		}
		return err
	}
	m.entityDeleted = false
	return nil
}

func (m *mockScriptClient) UpdateScript(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error {
	m.lastUpdateScriptID = scriptID
	configCopy := config
	m.lastUpdateConfig = &configCopy
	if m.updateScriptFn != nil {
		return m.updateScriptFn(ctx, scriptID, config)
	}
	return nil
}

func (m *mockScriptClient) DeleteScript(ctx context.Context, scriptID string) error {
	m.lastDeleteScriptID = scriptID
	if m.deleteScriptFn != nil {
		err := m.deleteScriptFn(ctx, scriptID)
		if err == nil {
			m.entityDeleted = true
		}
		return err
	}
	m.entityDeleted = true
	return nil
}

func (m *mockScriptClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	m.lastServiceData = data
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func (m *mockScriptClient) GetState(ctx context.Context, entityID string) (*homeassistant.Entity, error) {
	m.lastGetScriptID = entityID
	if m.getStateFn != nil {
		return m.getStateFn(ctx, entityID)
	}
	// Non-script entities (e.g., light.x in call_service data) are not tracked by this mock.
	// Returning not-found prevents snapshotEntities from capturing them for waitForStateChanges.
	if !strings.HasPrefix(entityID, "script.") {
		return nil, errors.New("entity not found")
	}
	if m.entityDeleted {
		return nil, errors.New("entity not found")
	}
	return &homeassistant.Entity{
		EntityID:   entityID,
		State:      "off",
		Attributes: map[string]any{"friendly_name": "Test Script"},
	}, nil
}

func TestNewScriptHandlers(t *testing.T) {
	t.Parallel()

	h := NewScriptHandlers()
	if h == nil {
		t.Error("NewScriptHandlers() returned nil")
	}
}

func TestScriptHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewScriptHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 2 // manage_script + call_service
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	expectedTools := map[string]bool{
		"manage_script": false,
		"call_service":  false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("Tool %q not registered", name)
		}
	}
}

func TestScriptHandlers_ManageScript_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		listScriptsErr error
		listScripts    []homeassistant.Entity
		wantError      bool
		wantContains   string
	}{
		{
			name:        "success empty",
			args:        map[string]any{"action": "list"},
			listScripts: []homeassistant.Entity{},
			wantError:   false,
		},
		{
			name: "success with scripts (json)",
			args: map[string]any{"action": "list", "format": "json"},
			listScripts: []homeassistant.Entity{
				{
					EntityID: "script.morning_routine",
					State:    "off",
					Attributes: map[string]any{
						"friendly_name":  "Morning Routine",
						"last_triggered": "2024-01-15T07:00:00",
					},
				},
				{
					EntityID:   "script.night_mode",
					State:      "off",
					Attributes: map[string]any{"friendly_name": "Night Mode"},
				},
			},
			wantError:    false,
			wantContains: "morning_routine",
		},
		{
			name: "success with scripts (natural)",
			args: map[string]any{"action": "list"},
			listScripts: []homeassistant.Entity{
				{
					EntityID: "script.morning_routine",
					State:    "off",
					Attributes: map[string]any{
						"friendly_name":  "Morning Routine",
						"last_triggered": "2024-01-15T07:00:00",
					},
				},
			},
			wantError:    false,
			wantContains: "Morning Routine",
		},
		{
			name:           "client error",
			args:           map[string]any{"action": "list"},
			listScriptsErr: errors.New("connection failed"),
			wantError:      true,
			wantContains:   "Error listing scripts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScriptClient{
				listScriptsFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					if tt.listScriptsErr != nil {
						return nil, tt.listScriptsErr
					}
					return tt.listScripts, nil
				},
			}

			h := NewScriptHandlers()
			result, err := h.handleManageScript(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScript() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestScriptHandlers_ManageScript_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		getScriptErr error
		script       *homeassistant.Script
		wantError    bool
		wantContains string
	}{
		{
			name: "success",
			args: map[string]any{
				"action":    "get",
				"script_id": "morning_routine",
			},
			script: &homeassistant.Script{
				EntityID: "script.morning_routine",
				State:    "off",
				Config: &homeassistant.ScriptConfig{
					Alias:       "Morning Routine",
					Description: "Runs in the morning",
					Mode:        "single",
					Sequence:    []any{map[string]any{"service": "light.turn_on"}},
				},
			},
			wantError: false,
		},
		{
			name: "missing script_id",
			args: map[string]any{
				"action": "get",
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "empty script_id",
			args: map[string]any{
				"action":    "get",
				"script_id": "",
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"action":    "get",
				"script_id": "morning_routine",
			},
			getScriptErr: errors.New("not found"),
			wantError:    true,
			wantContains: "error getting script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScriptClient{
				getScriptFn: func(_ context.Context, _ string) (*homeassistant.Script, error) {
					if tt.getScriptErr != nil {
						return nil, tt.getScriptErr
					}
					return tt.script, nil
				},
			}

			h := NewScriptHandlers()
			result, err := h.handleManageScript(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScript() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestScriptHandlers_ManageScript_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		createScriptErr error
		wantError       bool
		wantContains    string
	}{
		{
			name: "success",
			args: map[string]any{
				"action":    "create",
				"script_id": "morning_routine",
				"alias":     "Morning Routine",
				"sequence":  []any{map[string]any{"service": "light.turn_on"}},
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with all options",
			args: map[string]any{
				"action":      "create",
				"script_id":   "morning_routine",
				"alias":       "Morning Routine",
				"description": "Runs in the morning",
				"mode":        "restart",
				"icon":        "mdi:weather-sunny",
				"sequence":    []any{map[string]any{"service": "light.turn_on"}},
				"fields": map[string]any{
					"brightness": map[string]any{"description": "Light brightness"},
				},
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing script_id",
			args: map[string]any{
				"action":   "create",
				"alias":    "Morning Routine",
				"sequence": []any{map[string]any{"service": "light.turn_on"}},
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "empty script_id",
			args: map[string]any{
				"action":    "create",
				"script_id": "",
				"alias":     "Morning Routine",
				"sequence":  []any{map[string]any{"service": "light.turn_on"}},
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "missing alias",
			args: map[string]any{
				"action":    "create",
				"script_id": "morning_routine",
				"sequence":  []any{map[string]any{"service": "light.turn_on"}},
			},
			wantError:    true,
			wantContains: "alias is required",
		},
		{
			name: "empty alias",
			args: map[string]any{
				"action":    "create",
				"script_id": "morning_routine",
				"alias":     "",
				"sequence":  []any{map[string]any{"service": "light.turn_on"}},
			},
			wantError:    true,
			wantContains: "alias is required",
		},
		{
			name: "missing sequence",
			args: map[string]any{
				"action":    "create",
				"script_id": "morning_routine",
				"alias":     "Morning Routine",
			},
			wantError:    true,
			wantContains: "sequence is required",
		},
		{
			name: "empty sequence",
			args: map[string]any{
				"action":    "create",
				"script_id": "morning_routine",
				"alias":     "Morning Routine",
				"sequence":  []any{},
			},
			wantError:    true,
			wantContains: "at least one action",
		},
		{
			name: "client error",
			args: map[string]any{
				"action":    "create",
				"script_id": "morning_routine",
				"alias":     "Morning Routine",
				"sequence":  []any{map[string]any{"service": "light.turn_on"}},
			},
			createScriptErr: errors.New("creation failed"),
			wantError:       true,
			wantContains:    "Error creating script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScriptClient{
				createScriptFn: func(_ context.Context, _ string, _ homeassistant.ScriptConfig) error {
					return tt.createScriptErr
				},
			}

			h := NewScriptHandlers()
			result, err := h.handleManageScript(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScript() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestScriptHandlers_ManageScript_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		getScriptErr    error
		updateScriptErr error
		wantError       bool
		wantContains    string
	}{
		{
			name: "success",
			args: map[string]any{
				"action":    "update",
				"script_id": "morning_routine",
				"alias":     "Updated Morning Routine",
			},
			wantError:    false,
			wantContains: "updated successfully",
		},
		{
			name: "success with all options",
			args: map[string]any{
				"action":      "update",
				"script_id":   "morning_routine",
				"alias":       "Updated Morning Routine",
				"description": "Updated description",
				"mode":        "queued",
				"icon":        "mdi:script",
				"sequence":    []any{map[string]any{"service": "light.turn_off"}},
				"fields":      map[string]any{"test": "value"},
			},
			wantError:    false,
			wantContains: "updated successfully",
		},
		{
			name: "missing script_id",
			args: map[string]any{
				"action": "update",
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "empty script_id",
			args: map[string]any{
				"action":    "update",
				"script_id": "",
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "get script error",
			args: map[string]any{
				"action":    "update",
				"script_id": "morning_routine",
			},
			getScriptErr: errors.New("not found"),
			wantError:    true,
			wantContains: "Error getting current script",
		},
		{
			name: "update error",
			args: map[string]any{
				"action":    "update",
				"script_id": "morning_routine",
				"alias":     "Updated",
			},
			updateScriptErr: errors.New("update failed"),
			wantError:       true,
			wantContains:    "Error updating script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScriptClient{
				getScriptFn: func(context.Context, string) (*homeassistant.Script, error) {
					if tt.getScriptErr != nil {
						return nil, tt.getScriptErr
					}
					return &homeassistant.Script{
						EntityID:     "script.morning_routine",
						State:        "off",
						FriendlyName: "Morning Routine",
						Config: &homeassistant.ScriptConfig{
							Alias: "Morning Routine",
						},
					}, nil
				},
				updateScriptFn: func(_ context.Context, _ string, _ homeassistant.ScriptConfig) error {
					return tt.updateScriptErr
				},
			}

			h := NewScriptHandlers()
			result, err := h.handleManageScript(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScript() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestScriptHandlers_ManageScript_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		deleteScriptErr error
		wantError       bool
		wantContains    string
	}{
		{
			name: "success",
			args: map[string]any{
				"action":    "delete",
				"script_id": "morning_routine",
			},
			wantError:    false,
			wantContains: "deleted successfully",
		},
		{
			name: "missing script_id",
			args: map[string]any{
				"action": "delete",
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "empty script_id",
			args: map[string]any{
				"action":    "delete",
				"script_id": "",
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"action":    "delete",
				"script_id": "morning_routine",
			},
			deleteScriptErr: errors.New("deletion failed"),
			wantError:       true,
			wantContains:    "Error deleting script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScriptClient{
				deleteScriptFn: func(_ context.Context, _ string) error {
					return tt.deleteScriptErr
				},
			}

			h := NewScriptHandlers()
			result, err := h.handleManageScript(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScript() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestScriptHandlers_ManageScript_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		callServiceErr error
		wantError      bool
		wantContains   string
	}{
		{
			name: "success",
			args: map[string]any{
				"action":    "execute",
				"script_id": "morning_routine",
			},
			wantError:    false,
			wantContains: "executed successfully",
		},
		{
			name: "success with variables",
			args: map[string]any{
				"action":    "execute",
				"script_id": "morning_routine",
				"variables": map[string]any{
					"brightness": 100,
					"color":      "warm",
				},
			},
			wantError:    false,
			wantContains: "executed successfully",
		},
		{
			name: "missing script_id",
			args: map[string]any{
				"action": "execute",
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "empty script_id",
			args: map[string]any{
				"action":    "execute",
				"script_id": "",
			},
			wantError:    true,
			wantContains: "script_id is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"action":    "execute",
				"script_id": "morning_routine",
			},
			callServiceErr: errors.New("execution failed"),
			wantError:      true,
			wantContains:   "Error executing script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScriptClient{
				callServiceFn: func(_ context.Context, domain, service string, _ map[string]any) ([]homeassistant.Entity, error) {
					if domain != "script" {
						t.Errorf("wrong domain: %s", domain)
					}
					if service != "turn_on" {
						t.Errorf("wrong service: %s", service)
					}
					return nil, tt.callServiceErr
				},
			}

			h := NewScriptHandlers()
			result, err := h.handleManageScript(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScript() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestScriptHandlers_ManageScript_InvalidAction(t *testing.T) {
	t.Parallel()

	h := NewScriptHandlers()
	client := &mockScriptClient{}

	// Test missing action
	result, err := h.handleManageScript(context.Background(), client, map[string]any{})
	if err != nil {
		t.Errorf("handleManageScript() returned error: %v", err)
		return
	}
	if !result.IsError {
		t.Error("Expected error for missing action")
	}
	if !strings.Contains(result.Content[0].Text, "action is required") {
		t.Errorf("Expected 'action is required' error, got: %s", result.Content[0].Text)
	}

	// Test invalid action
	result, err = h.handleManageScript(context.Background(), client, map[string]any{"action": "invalid"})
	if err != nil {
		t.Errorf("handleManageScript() returned error: %v", err)
		return
	}
	if !result.IsError {
		t.Error("Expected error for invalid action")
	}
	if !strings.Contains(result.Content[0].Text, "invalid action") {
		t.Errorf("Expected 'invalid action' error, got: %s", result.Content[0].Text)
	}
}

func TestManageScript_Patch(t *testing.T) {
	t.Parallel()

	baseConfig := &homeassistant.ScriptConfig{
		Alias:    "Morning Routine",
		Mode:     "single",
		Sequence: []any{map[string]any{"action": "light.turn_on"}},
	}

	h := &ScriptHandlers{}

	tests := []handlerTestCase{
		{
			name: "patch - missing script_id",
			args: map[string]any{
				"action":     "patch",
				"operations": []any{map[string]any{"op": "replace", "path": "/mode", "value": "queued"}},
			},
			wantError:    true,
			wantContains: []string{"script_id is required"},
		},
		{
			name: "patch - missing operations",
			args: map[string]any{
				"action":    "patch",
				"script_id": "morning_routine",
			},
			wantError:    true,
			wantContains: []string{"operations is required"},
		},
		{
			name: "patch - success replace mode",
			args: map[string]any{
				"action":    "patch",
				"script_id": "morning_routine",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/mode", "value": "queued"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetScriptFn = func(_ context.Context, _ string) (*homeassistant.Script, error) {
					cfg := *baseConfig
					return &homeassistant.Script{EntityID: "script.morning_routine", Config: &cfg}, nil
				}
				m.UpdateScriptFn = func(_ context.Context, _ string, _ homeassistant.ScriptConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully", "1 operations"},
		},
		{
			name: "patch - success add sequence step",
			args: map[string]any{
				"action":    "patch",
				"script_id": "morning_routine",
				"operations": []any{
					map[string]any{"op": "add", "path": "/sequence/-", "value": map[string]any{"action": "light.turn_off"}},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetScriptFn = func(_ context.Context, _ string) (*homeassistant.Script, error) {
					cfg := *baseConfig
					return &homeassistant.Script{EntityID: "script.morning_routine", Config: &cfg}, nil
				}
				m.UpdateScriptFn = func(_ context.Context, _ string, _ homeassistant.ScriptConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully"},
		},
		{
			name: "patch - nil config returns error",
			args: map[string]any{
				"action":    "patch",
				"script_id": "morning_routine",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/mode", "value": "queued"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetScriptFn = func(_ context.Context, _ string) (*homeassistant.Script, error) {
					return &homeassistant.Script{EntityID: "script.morning_routine", Config: nil}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"no configuration to patch"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageScript)
}

func TestManageScript_SemanticPatch(t *testing.T) {
	t.Parallel()

	baseConfig := &homeassistant.ScriptConfig{
		Alias: "Morning Routine",
		Mode:  "single",
		Sequence: []any{
			map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": "light.kitchen"}},
			map[string]any{"delay": map[string]any{"seconds": float64(5)}},
			map[string]any{"action": "light.turn_off", "target": map[string]any{"entity_id": "light.kitchen"}},
		},
	}

	h := &ScriptHandlers{}

	tests := []handlerTestCase{
		{
			name: "semantic patch - replace action target",
			args: map[string]any{
				"action":    "patch",
				"script_id": "morning_routine",
				"operations": []any{
					map[string]any{
						"op":      "replace",
						"match":   map[string]any{"action": "light.turn_on"},
						"section": "sequence",
						"field":   "action",
						"value":   "light.toggle",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetScriptFn = func(_ context.Context, _ string) (*homeassistant.Script, error) {
					cfg := *baseConfig
					return &homeassistant.Script{EntityID: "script.morning_routine", Config: &cfg}, nil
				}
				m.UpdateScriptFn = func(_ context.Context, _ string, _ homeassistant.ScriptConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully"},
		},
		{
			name: "semantic patch - no match in sequence",
			args: map[string]any{
				"action":    "patch",
				"script_id": "morning_routine",
				"operations": []any{
					map[string]any{
						"op":      "replace",
						"match":   map[string]any{"action": "nonexistent.action"},
						"section": "sequence",
						"field":   "action",
						"value":   "x",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetScriptFn = func(_ context.Context, _ string) (*homeassistant.Script, error) {
					cfg := *baseConfig
					return &homeassistant.Script{EntityID: "script.morning_routine", Config: &cfg}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"error applying patch", "no elements"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageScript)
}

// TestManageScript_PatchReload verifies that a successful patch write triggers
// script.reload so the change is immediately visible to a subsequent get (#126).
func TestManageScript_PatchReload(t *testing.T) {
	t.Parallel()

	baseConfig := &homeassistant.ScriptConfig{
		Alias:    "Morning Routine",
		Mode:     "single",
		Sequence: []any{map[string]any{"action": "light.turn_on"}},
	}

	h := &ScriptHandlers{}
	patchArgs := map[string]any{
		"action":    "patch",
		"script_id": "morning_routine",
		"operations": []any{
			map[string]any{"op": "replace", "path": "/mode", "value": "queued"},
		},
	}

	t.Run("patch reloads script domain after a successful write", func(t *testing.T) {
		t.Parallel()
		var reloadDomain, reloadService string
		client := &UniversalMockClient{}
		client.GetScriptFn = func(context.Context, string) (*homeassistant.Script, error) {
			cfg := *baseConfig
			return &homeassistant.Script{EntityID: "script.morning_routine", Config: &cfg}, nil
		}
		client.UpdateScriptFn = func(context.Context, string, homeassistant.ScriptConfig) error { return nil }
		client.CallServiceFn = func(_ context.Context, domain, service string, _ map[string]any) ([]homeassistant.Entity, error) {
			reloadDomain, reloadService = domain, service
			return nil, nil
		}

		result, err := h.handleManageScript(context.Background(), client, patchArgs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content[0].Text)
		}
		if reloadDomain != "script" || reloadService != "reload" {
			t.Errorf("expected script.reload to be called, got domain=%q service=%q", reloadDomain, reloadService)
		}
	})

	t.Run("patch reports a warning when the reload call fails", func(t *testing.T) {
		t.Parallel()
		client := &UniversalMockClient{}
		client.GetScriptFn = func(context.Context, string) (*homeassistant.Script, error) {
			cfg := *baseConfig
			return &homeassistant.Script{EntityID: "script.morning_routine", Config: &cfg}, nil
		}
		client.UpdateScriptFn = func(context.Context, string, homeassistant.ScriptConfig) error { return nil }
		client.CallServiceFn = func(context.Context, string, string, map[string]any) ([]homeassistant.Entity, error) {
			return nil, errors.New("ws down")
		}

		result, err := h.handleManageScript(context.Background(), client, patchArgs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success (patch persisted even if reload failed), got error: %s", result.Content[0].Text)
		}
		if !strings.Contains(result.Content[0].Text, "reload after save failed") {
			t.Errorf("expected reload-failed warning, got: %s", result.Content[0].Text)
		}
	})

	t.Run("no-op patch does not write or trigger a reload", func(t *testing.T) {
		t.Parallel()
		updateCalled := false
		reloadCalled := false
		client := &UniversalMockClient{}
		client.GetScriptFn = func(context.Context, string) (*homeassistant.Script, error) {
			cfg := *baseConfig
			return &homeassistant.Script{EntityID: "script.morning_routine", Config: &cfg}, nil
		}
		client.UpdateScriptFn = func(context.Context, string, homeassistant.ScriptConfig) error {
			updateCalled = true
			return nil
		}
		client.CallServiceFn = func(context.Context, string, string, map[string]any) ([]homeassistant.Entity, error) {
			reloadCalled = true
			return nil, nil
		}

		noOpArgs := map[string]any{
			"action":    "patch",
			"script_id": "morning_routine",
			"operations": []any{
				map[string]any{"op": "replace", "path": "/mode", "value": "single"}, // matches existing value
			},
		}
		result, err := h.handleManageScript(context.Background(), client, noOpArgs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content[0].Text)
		}
		if updateCalled {
			t.Error("UpdateScript must NOT be called for a no-op patch")
		}
		if reloadCalled {
			t.Error("no-op patch must not trigger a reload")
		}
		if !strings.Contains(result.Content[0].Text, "no changes") {
			t.Errorf("expected 'no changes' in response, got: %s", result.Content[0].Text)
		}
	})
}

// TestManageScript_UpdateReload verifies that a successful update write triggers
// script.reload so the change is immediately visible to a subsequent get (#126).
func TestManageScript_UpdateReload(t *testing.T) {
	t.Parallel()

	makeScript := func() *homeassistant.Script {
		return &homeassistant.Script{
			EntityID:     "script.morning_routine",
			FriendlyName: "Morning Routine",
			Config:       &homeassistant.ScriptConfig{Alias: "Morning Routine"},
		}
	}
	h := NewScriptHandlers()
	updateArgs := map[string]any{"action": "update", "script_id": "morning_routine", "alias": "Updated"}

	t.Run("update reloads script domain after a successful write", func(t *testing.T) {
		t.Parallel()
		var reloadDomain, reloadService string
		client := &UniversalMockClient{}
		client.GetScriptFn = func(context.Context, string) (*homeassistant.Script, error) { return makeScript(), nil }
		client.UpdateScriptFn = func(context.Context, string, homeassistant.ScriptConfig) error { return nil }
		client.CallServiceFn = func(_ context.Context, domain, service string, _ map[string]any) ([]homeassistant.Entity, error) {
			reloadDomain, reloadService = domain, service
			return nil, nil
		}

		result, err := h.handleManageScript(context.Background(), client, updateArgs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content[0].Text)
		}
		if reloadDomain != "script" || reloadService != "reload" {
			t.Errorf("expected script.reload to be called, got domain=%q service=%q", reloadDomain, reloadService)
		}
	})

	t.Run("update reports a warning when the reload call fails", func(t *testing.T) {
		t.Parallel()
		client := &UniversalMockClient{}
		client.GetScriptFn = func(context.Context, string) (*homeassistant.Script, error) { return makeScript(), nil }
		client.UpdateScriptFn = func(context.Context, string, homeassistant.ScriptConfig) error { return nil }
		client.CallServiceFn = func(context.Context, string, string, map[string]any) ([]homeassistant.Entity, error) {
			return nil, errors.New("ws down")
		}

		result, err := h.handleManageScript(context.Background(), client, updateArgs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success (update persisted even if reload failed), got error: %s", result.Content[0].Text)
		}
		if !strings.Contains(result.Content[0].Text, "reload after save failed") {
			t.Errorf("expected reload-failed warning, got: %s", result.Content[0].Text)
		}
	})

	t.Run("no-op update does not write or trigger a reload", func(t *testing.T) {
		t.Parallel()
		updateCalled := false
		reloadCalled := false
		client := &UniversalMockClient{}
		client.GetScriptFn = func(context.Context, string) (*homeassistant.Script, error) { return makeScript(), nil }
		client.UpdateScriptFn = func(context.Context, string, homeassistant.ScriptConfig) error {
			updateCalled = true
			return nil
		}
		client.CallServiceFn = func(context.Context, string, string, map[string]any) ([]homeassistant.Entity, error) {
			reloadCalled = true
			return nil, nil
		}

		noOpArgs := map[string]any{"action": "update", "script_id": "morning_routine", "alias": "Morning Routine"}
		result, err := h.handleManageScript(context.Background(), client, noOpArgs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Content[0].Text)
		}
		if updateCalled {
			t.Error("UpdateScript must NOT be called for a no-op update")
		}
		if reloadCalled {
			t.Error("no-op update must not trigger a reload")
		}
		if !strings.Contains(result.Content[0].Text, "no changes") {
			t.Errorf("expected 'no changes' in response, got: %s", result.Content[0].Text)
		}
	})
}

func TestScriptHandlers_CallService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		callServiceErr error
		callServiceRes []homeassistant.Entity
		wantError      bool
		wantContains   string
	}{
		{
			name: "success json format",
			args: map[string]any{
				"domain":  "light",
				"service": "turn_on",
				"format":  "json",
			},
			wantError:    false,
			wantContains: "success",
		},
		{
			name: "success with data json format",
			args: map[string]any{
				"domain":  "light",
				"service": "turn_on",
				"format":  "json",
				"data": map[string]any{
					"entity_id":  "light.living_room",
					"brightness": 255,
				},
			},
			callServiceRes: []homeassistant.Entity{
				{EntityID: "light.living_room", State: "on"},
			},
			wantError:    false,
			wantContains: "success",
		},
		{
			name: "success natural format default",
			args: map[string]any{
				"domain":  "light",
				"service": "turn_on",
			},
			wantError:    false,
			wantContains: "OK Turned on",
		},
		{
			name: "success natural format explicit",
			args: map[string]any{
				"domain":  "light",
				"service": "turn_on",
				"format":  "natural",
			},
			wantError:    false,
			wantContains: "OK Turned on",
		},
		{
			name: "success natural with single entity",
			args: map[string]any{
				"domain":  "light",
				"service": "turn_off",
				"format":  "natural",
				"data": map[string]any{
					"entity_id": "light.bedroom",
				},
			},
			callServiceRes: []homeassistant.Entity{
				{EntityID: "light.bedroom", State: "off"},
			},
			wantError:    false,
			wantContains: "OK Turned off light.bedroom",
		},
		{
			name: "success natural with multiple entities",
			args: map[string]any{
				"domain":  "light",
				"service": "toggle",
				"format":  "natural",
				"data": map[string]any{
					"entity_id": []any{"light.bedroom", "light.kitchen"},
				},
			},
			callServiceRes: []homeassistant.Entity{
				{EntityID: "light.bedroom", State: "off"},
				{EntityID: "light.kitchen", State: "on"},
			},
			wantError:    false,
			wantContains: "OK Toggled 2 entities",
		},
		{
			name: "missing domain",
			args: map[string]any{
				"service": "turn_on",
				"format":  "json",
			},
			wantError:    true,
			wantContains: "domain is required",
		},
		{
			name: "empty domain",
			args: map[string]any{
				"domain":  "",
				"service": "turn_on",
				"format":  "json",
			},
			wantError:    true,
			wantContains: "domain is required",
		},
		{
			name: "missing service",
			args: map[string]any{
				"domain": "light",
				"format": "json",
			},
			wantError:    true,
			wantContains: "service is required",
		},
		{
			name: "empty service",
			args: map[string]any{
				"domain":  "light",
				"service": "",
				"format":  "json",
			},
			wantError:    true,
			wantContains: "service is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"domain":  "light",
				"service": "turn_on",
				"format":  "json",
			},
			callServiceErr: errors.New("service call failed"),
			wantError:      true,
			wantContains:   "Error calling service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScriptClient{
				callServiceFn: func(_ context.Context, domain, service string, _ map[string]any) ([]homeassistant.Entity, error) {
					if wantDomain, _ := tt.args["domain"].(string); domain != wantDomain {
						t.Errorf("domain = %q, want %q", domain, wantDomain)
					}
					if wantService, _ := tt.args["service"].(string); service != wantService {
						t.Errorf("service = %q, want %q", service, wantService)
					}
					if tt.callServiceErr != nil {
						return nil, tt.callServiceErr
					}
					return tt.callServiceRes, nil
				},
			}

			h := NewScriptHandlers()
			result, err := h.handleCallService(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCallService() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestFindScriptByID(t *testing.T) {
	t.Parallel()

	scriptWithAlias := &homeassistant.Script{
		EntityID:     "script.morning_routine",
		State:        "off",
		FriendlyName: "Morning Routine Script",
		Config: &homeassistant.ScriptConfig{
			Alias: "Morning Routine Script",
		},
	}

	tests := []struct {
		name         string
		searchID     string
		scripts      []homeassistant.Entity
		scriptMap    map[string]*homeassistant.Script
		wantFound    bool
		wantEntityID string
	}{
		{
			name:     "find by entity_id",
			searchID: "script.morning_routine",
			scripts: []homeassistant.Entity{
				{EntityID: "script.morning_routine", State: "off", Attributes: map[string]any{"friendly_name": "Morning Routine Script"}},
			},
			scriptMap: map[string]*homeassistant.Script{
				"morning_routine": scriptWithAlias,
			},
			wantFound:    true,
			wantEntityID: "script.morning_routine",
		},
		{
			name:     "find by alias - partial match",
			searchID: "morning routine",
			scripts: []homeassistant.Entity{
				{EntityID: "script.morning_routine", State: "off", Attributes: map[string]any{"friendly_name": "Morning Routine Script"}},
			},
			scriptMap: map[string]*homeassistant.Script{
				"morning_routine": scriptWithAlias,
			},
			wantFound:    true,
			wantEntityID: "script.morning_routine",
		},
		{
			name:     "find by alias - case insensitive",
			searchID: "MORNING ROUTINE",
			scripts: []homeassistant.Entity{
				{EntityID: "script.morning_routine", State: "off", Attributes: map[string]any{"friendly_name": "Morning Routine Script"}},
			},
			scriptMap: map[string]*homeassistant.Script{
				"morning_routine": scriptWithAlias,
			},
			wantFound:    true,
			wantEntityID: "script.morning_routine",
		},
		{
			name:     "find by friendly_name - partial match",
			searchID: "Script",
			scripts: []homeassistant.Entity{
				{EntityID: "script.morning_routine", State: "off", Attributes: map[string]any{"friendly_name": "Morning Routine Script"}},
			},
			scriptMap: map[string]*homeassistant.Script{
				"morning_routine": scriptWithAlias,
			},
			wantFound:    true,
			wantEntityID: "script.morning_routine",
		},
		{
			name:     "not found - no matching alias or friendly_name",
			searchID: "nonexistent",
			scripts: []homeassistant.Entity{
				{EntityID: "script.morning_routine", State: "off", Attributes: map[string]any{"friendly_name": "Morning Routine Script"}},
			},
			scriptMap: map[string]*homeassistant.Script{
				"morning_routine": scriptWithAlias,
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScriptClient{
				listScriptsFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					return tt.scripts, nil
				},
				getScriptFn: func(_ context.Context, scriptID string) (*homeassistant.Script, error) {
					if s, ok := tt.scriptMap[scriptID]; ok {
						return s, nil
					}
					return nil, errors.New("not found")
				},
			}

			h := &ScriptHandlers{}
			result, err := h.findScriptByID(context.Background(), client, tt.searchID)

			if tt.wantFound {
				if err != nil {
					t.Errorf("findScriptByID() unexpected error = %v", err)
					return
				}
				if result == nil {
					t.Error("findScriptByID() returned nil, want script")
					return
				}
				if result.EntityID != tt.wantEntityID {
					t.Errorf("findScriptByID() EntityID = %q, want %q", result.EntityID, tt.wantEntityID)
				}
			} else {
				if err == nil {
					t.Error("findScriptByID() expected error, got nil")
				}
			}
		})
	}
}

func TestManageScript_GetByFriendlyName(t *testing.T) {
	t.Parallel()

	scriptWithAlias := &homeassistant.Script{
		EntityID:     "script.morning_routine",
		State:        "off",
		FriendlyName: "Morning Routine Script",
		Config: &homeassistant.ScriptConfig{
			Alias:    "Morning Routine Script",
			Sequence: []any{map[string]any{"service": "light.turn_on"}},
		},
	}

	tests := []struct {
		name         string
		args         map[string]any
		setupClient  func() *mockScriptClient
		wantError    bool
		wantContains string
	}{
		{
			name: "get by alias - partial match",
			args: map[string]any{"action": "get", "script_id": "morning routine"},
			setupClient: func() *mockScriptClient {
				return &mockScriptClient{
					getScriptFn: func(_ context.Context, scriptID string) (*homeassistant.Script, error) {
						if scriptID == "morning routine" {
							return nil, errors.New("not found")
						}
						if scriptID == "morning_routine" {
							return scriptWithAlias, nil
						}
						return nil, errors.New("not found")
					},
					listScriptsFn: func(_ context.Context) ([]homeassistant.Entity, error) {
						return []homeassistant.Entity{
							{EntityID: "script.morning_routine", State: "off", Attributes: map[string]any{"friendly_name": "Morning Routine Script"}},
						}, nil
					},
				}
			},
			wantContains: "Morning Routine Script",
		},
		{
			name: "get by friendly_name - case insensitive",
			args: map[string]any{"action": "get", "script_id": "MORNING"},
			setupClient: func() *mockScriptClient {
				return &mockScriptClient{
					getScriptFn: func(_ context.Context, scriptID string) (*homeassistant.Script, error) {
						if scriptID == "MORNING" {
							return nil, errors.New("not found")
						}
						if scriptID == "morning_routine" {
							return scriptWithAlias, nil
						}
						return nil, errors.New("not found")
					},
					listScriptsFn: func(_ context.Context) ([]homeassistant.Entity, error) {
						return []homeassistant.Entity{
							{EntityID: "script.morning_routine", State: "off", Attributes: map[string]any{"friendly_name": "Morning Routine Script"}},
						}, nil
					},
				}
			},
			wantContains: "Morning Routine Script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewScriptHandlers()
			result, err := h.handleManageScript(context.Background(), tt.setupClient(), tt.args)
			if err != nil {
				t.Fatalf("handleManageScript() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Expected content to contain %q, got: %s", tt.wantContains, content)
			}
		})
	}
}

func TestExtractEntityTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     map[string]any
		expected []string
	}{
		{
			name:     "nil data",
			data:     nil,
			expected: []string{},
		},
		{
			name:     "no entity_id",
			data:     map[string]any{"brightness": 255},
			expected: []string{},
		},
		{
			name:     "single string entity_id",
			data:     map[string]any{"entity_id": "light.bedroom"},
			expected: []string{"light.bedroom"},
		},
		{
			name: "[]any slice entity_id",
			data: map[string]any{
				"entity_id": []any{"light.bedroom", "light.kitchen"},
			},
			expected: []string{"light.bedroom", "light.kitchen"},
		},
		{
			name: "[]string slice entity_id",
			data: map[string]any{
				"entity_id": []string{"light.bedroom", "light.kitchen"},
			},
			expected: []string{"light.bedroom", "light.kitchen"},
		},
		{
			name:     "empty string entity_id",
			data:     map[string]any{"entity_id": ""},
			expected: []string{},
		},
		{
			name: "empty []any slice",
			data: map[string]any{
				"entity_id": []any{},
			},
			expected: []string{},
		},
		{
			name: "mixed valid and invalid in []any",
			data: map[string]any{
				"entity_id": []any{"light.bedroom", 123, "light.kitchen"},
			},
			expected: []string{"light.bedroom", "light.kitchen"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractEntityTargets(tt.data)
			if len(got) != len(tt.expected) {
				t.Errorf("extractEntityTargets() length = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i, exp := range tt.expected {
				if got[i] != exp {
					t.Errorf("extractEntityTargets()[%d] = %q, want %q", i, got[i], exp)
				}
			}
		})
	}
}

// TestScriptHandlers_IDNormalization tests that script_id inputs are properly normalized
// to avoid double-prefix bugs (e.g., script.script.morning_routine).
func TestScriptHandlers_IDNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		action              string
		inputID             string
		wantGetScriptID     string // For update action (now uses GetScript)
		wantUpdateScriptID  string
		wantDeleteScriptID  string
		wantServiceEntityID string // For execute action
		additionalArgs      map[string]any
	}{
		{
			name:               "update - with script. prefix",
			action:             "update",
			inputID:            "script.morning_routine",
			wantGetScriptID:    "script.morning_routine", // Should NOT be script.script.morning_routine
			wantUpdateScriptID: "morning_routine",        // Should strip prefix for REST API
			additionalArgs: map[string]any{
				"alias": "Updated Morning Routine",
			},
		},
		{
			name:               "update - without prefix",
			action:             "update",
			inputID:            "morning_routine",
			wantGetScriptID:    "script.morning_routine", // Should add prefix for GetScript
			wantUpdateScriptID: "morning_routine",
			additionalArgs: map[string]any{
				"alias": "Updated Morning Routine",
			},
		},
		{
			name:               "delete - with script. prefix",
			action:             "delete",
			inputID:            "script.morning_routine",
			wantDeleteScriptID: "morning_routine", // Should strip prefix for REST API
		},
		{
			name:               "delete - without prefix",
			action:             "delete",
			inputID:            "morning_routine",
			wantDeleteScriptID: "morning_routine",
		},
		{
			name:                "execute - with script. prefix",
			action:              "execute",
			inputID:             "script.morning_routine",
			wantServiceEntityID: "script.morning_routine", // Should NOT be script.script.morning_routine
		},
		{
			name:                "execute - without prefix",
			action:              "execute",
			inputID:             "morning_routine",
			wantServiceEntityID: "script.morning_routine", // Should add prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScriptClient{}
			h := &ScriptHandlers{}

			args := map[string]any{
				"action":    tt.action,
				"script_id": tt.inputID,
			}
			for k, v := range tt.additionalArgs {
				args[k] = v
			}

			_, err := h.handleManageScript(context.Background(), client, args)
			if err != nil {
				t.Fatalf("handleManageScript() unexpected error = %v", err)
			}

			// Verify correct IDs were used
			if tt.wantGetScriptID != "" && client.lastGetScriptID != tt.wantGetScriptID {
				t.Errorf("GetScript called with ID %q, want %q", client.lastGetScriptID, tt.wantGetScriptID)
			}
			if tt.wantUpdateScriptID != "" && client.lastUpdateScriptID != tt.wantUpdateScriptID {
				t.Errorf("UpdateScript called with ID %q, want %q", client.lastUpdateScriptID, tt.wantUpdateScriptID)
			}
			if tt.wantDeleteScriptID != "" && client.lastDeleteScriptID != tt.wantDeleteScriptID {
				t.Errorf("DeleteScript called with ID %q, want %q", client.lastDeleteScriptID, tt.wantDeleteScriptID)
			}
			if tt.wantServiceEntityID != "" {
				if entityID, ok := client.lastServiceData["entity_id"].(string); ok {
					if entityID != tt.wantServiceEntityID {
						t.Errorf("CallService entity_id %q, want %q", entityID, tt.wantServiceEntityID)
					}
				} else {
					t.Error("CallService data missing entity_id")
				}
			}
		})
	}
}

// TestScriptHandlers_UpdateDataPreservation tests that update preserves existing script data
// when using GetScript instead of GetState.
func TestScriptHandlers_UpdateDataPreservation(t *testing.T) {
	t.Parallel()

	// Full script configuration that should be preserved
	existingScript := &homeassistant.Script{
		EntityID:     "script.test_script",
		State:        "off",
		FriendlyName: "Original Name",
		Config: &homeassistant.ScriptConfig{
			Alias:       "Original Name",
			Description: "Original description",
			Mode:        "single",
			Icon:        "mdi:script-text",
			Sequence: []any{
				map[string]any{"service": "light.turn_on"},
				map[string]any{"delay": "00:00:05"},
			},
			Fields: map[string]any{
				"brightness": map[string]any{"description": "Brightness level"},
			},
		},
	}

	client := &mockScriptClient{
		getScriptFn: func(context.Context, string) (*homeassistant.Script, error) {
			return existingScript, nil
		},
	}

	h := &ScriptHandlers{}
	args := map[string]any{
		"action":    "update",
		"script_id": "test_script",
		"alias":     "Updated Name", // Only update the alias
	}

	_, err := h.handleManageScript(context.Background(), client, args)
	if err != nil {
		t.Fatalf("handleManageScript() unexpected error = %v", err)
	}

	// Verify that existing data was preserved
	if client.lastUpdateConfig == nil {
		t.Fatal("UpdateScript was not called with a config")
	}

	// Check that existing fields were preserved
	if client.lastUpdateConfig.Description != "Original description" {
		t.Errorf("Description not preserved: got %q, want %q", client.lastUpdateConfig.Description, "Original description")
	}
	if client.lastUpdateConfig.Mode != "single" {
		t.Errorf("Mode not preserved: got %q, want %q", client.lastUpdateConfig.Mode, "single")
	}
	if client.lastUpdateConfig.Icon != "mdi:script-text" {
		t.Errorf("Icon not preserved: got %q, want %q", client.lastUpdateConfig.Icon, "mdi:script-text")
	}
	if len(client.lastUpdateConfig.Sequence) != 2 {
		t.Errorf("Sequence not preserved: got %d items, want 2", len(client.lastUpdateConfig.Sequence))
	}
	if client.lastUpdateConfig.Fields == nil {
		t.Error("Fields not preserved: got nil")
	}

	// Check that the alias was updated
	if client.lastUpdateConfig.Alias != "Updated Name" {
		t.Errorf("Alias not updated: got %q, want %q", client.lastUpdateConfig.Alias, "Updated Name")
	}
}

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
