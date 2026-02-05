package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

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
}

func (m *mockScriptClient) ListScripts(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.listScriptsFn != nil {
		return m.listScriptsFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *mockScriptClient) GetScript(ctx context.Context, scriptID string) (*homeassistant.Script, error) {
	if m.getScriptFn != nil {
		return m.getScriptFn(ctx, scriptID)
	}
	return &homeassistant.Script{}, nil
}

func (m *mockScriptClient) CreateScript(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error {
	if m.createScriptFn != nil {
		return m.createScriptFn(ctx, scriptID, config)
	}
	return nil
}

func (m *mockScriptClient) UpdateScript(ctx context.Context, scriptID string, config homeassistant.ScriptConfig) error {
	if m.updateScriptFn != nil {
		return m.updateScriptFn(ctx, scriptID, config)
	}
	return nil
}

func (m *mockScriptClient) DeleteScript(ctx context.Context, scriptID string) error {
	if m.deleteScriptFn != nil {
		return m.deleteScriptFn(ctx, scriptID)
	}
	return nil
}

func (m *mockScriptClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func (m *mockScriptClient) GetState(ctx context.Context, entityID string) (*homeassistant.Entity, error) {
	if m.getStateFn != nil {
		return m.getStateFn(ctx, entityID)
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
			wantContains: "Error getting script",
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
			wantContains: "sequence is required",
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
		getStateErr     error
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
			name: "get state error",
			args: map[string]any{
				"action":    "update",
				"script_id": "morning_routine",
			},
			getStateErr:  errors.New("not found"),
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
				getStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					if tt.getStateErr != nil {
						return nil, tt.getStateErr
					}
					return &homeassistant.Entity{
						EntityID:   "script.morning_routine",
						State:      "off",
						Attributes: map[string]any{"friendly_name": "Morning Routine"},
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
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
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
			name: "success",
			args: map[string]any{
				"domain":  "light",
				"service": "turn_on",
			},
			wantError:    false,
			wantContains: "success",
		},
		{
			name: "success with data",
			args: map[string]any{
				"domain":  "light",
				"service": "turn_on",
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
			name: "missing domain",
			args: map[string]any{
				"service": "turn_on",
			},
			wantError:    true,
			wantContains: "domain is required",
		},
		{
			name: "empty domain",
			args: map[string]any{
				"domain":  "",
				"service": "turn_on",
			},
			wantError:    true,
			wantContains: "domain is required",
		},
		{
			name: "missing service",
			args: map[string]any{
				"domain": "light",
			},
			wantError:    true,
			wantContains: "service is required",
		},
		{
			name: "empty service",
			args: map[string]any{
				"domain":  "light",
				"service": "",
			},
			wantError:    true,
			wantContains: "service is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"domain":  "light",
				"service": "turn_on",
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
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
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
