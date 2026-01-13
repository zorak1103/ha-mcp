package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockInputSelectClient implements homeassistant.Client for testing.
type mockInputSelectClient struct {
	homeassistant.Client
	createHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	deleteHelperFn func(ctx context.Context, entityID string) error
	callServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockInputSelectClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.createHelperFn != nil {
		return m.createHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockInputSelectClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.deleteHelperFn != nil {
		return m.deleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockInputSelectClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewInputSelectHandlers(t *testing.T) {
	t.Parallel()

	h := NewInputSelectHandlers()
	if h == nil {
		t.Error("NewInputSelectHandlers() returned nil")
	}
}

func TestInputSelectHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewInputSelectHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 4 {
		t.Errorf("RegisterTools() registered %d tools, want 4", len(tools))
	}

	expectedTools := map[string]bool{
		"create_input_select": false,
		"delete_input_select": false,
		"select_option":       false,
		"set_options":         false,
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

func TestInputSelectHandlers_createInputSelectTool(t *testing.T) {
	t.Parallel()

	h := NewInputSelectHandlers()
	tool := h.createInputSelectTool()

	if tool.Name != "create_input_select" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "create_input_select")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	if len(tool.InputSchema.Required) != 3 {
		t.Errorf("InputSchema.Required length = %d, want 3", len(tool.InputSchema.Required))
	}

	requiredFields := map[string]bool{"id": false, "name": false, "options": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}
}

func TestInputSelectHandlers_handleCreateInputSelect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		createHelperErr error
		wantError       bool
		wantContains    string
	}{
		{
			name: "success",
			args: map[string]any{
				"id":      "test_dropdown",
				"name":    "Test Dropdown",
				"options": []any{"option1", "option2", "option3"},
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with optional fields",
			args: map[string]any{
				"id":      "test_dropdown",
				"name":    "Test Dropdown",
				"options": []any{"opt1", "opt2"},
				"icon":    "mdi:format-list-bulleted",
				"initial": "opt1",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing id",
			args: map[string]any{
				"name":    "Test Dropdown",
				"options": []any{"opt1", "opt2"},
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "empty id",
			args: map[string]any{
				"id":      "",
				"name":    "Test Dropdown",
				"options": []any{"opt1", "opt2"},
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"id":      "test_dropdown",
				"options": []any{"opt1", "opt2"},
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"id":      "test_dropdown",
				"name":    "",
				"options": []any{"opt1", "opt2"},
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "missing options",
			args: map[string]any{
				"id":   "test_dropdown",
				"name": "Test Dropdown",
			},
			wantError:    true,
			wantContains: "options is required",
		},
		{
			name: "empty options array",
			args: map[string]any{
				"id":      "test_dropdown",
				"name":    "Test Dropdown",
				"options": []any{},
			},
			wantError:    true,
			wantContains: "options is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"id":      "test_dropdown",
				"name":    "Test Dropdown",
				"options": []any{"opt1", "opt2"},
			},
			createHelperErr: errors.New("connection failed"),
			wantError:       true,
			wantContains:    "Error creating input_select",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputSelectClient{
				createHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewInputSelectHandlers()
			result, err := h.handleCreateInputSelect(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateInputSelect() returned error: %v", err)
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
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestInputSelectHandlers_handleDeleteInputSelect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		deleteHelperErr error
		wantError       bool
		wantContains    string
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": "input_select.test_dropdown",
			},
			wantError:    false,
			wantContains: "deleted successfully",
		},
		{
			name:         "missing entity_id",
			args:         map[string]any{},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "input_text.test_text",
			},
			wantError:    true,
			wantContains: "must be an input_select entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_select.test_dropdown",
			},
			deleteHelperErr: errors.New("not found"),
			wantError:       true,
			wantContains:    "Error deleting input_select",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputSelectClient{
				deleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewInputSelectHandlers()
			result, err := h.handleDeleteInputSelect(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteInputSelect() returned error: %v", err)
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
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestInputSelectHandlers_handleSelectOption(t *testing.T) {
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
				"entity_id": "input_select.test_dropdown",
				"option":    "option2",
			},
			wantError:    false,
			wantContains: "selected successfully",
		},
		{
			name: "missing entity_id",
			args: map[string]any{
				"option": "option1",
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
				"option":    "option1",
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "input_boolean.test_switch",
				"option":    "option1",
			},
			wantError:    true,
			wantContains: "must be an input_select entity",
		},
		{
			name: "missing option",
			args: map[string]any{
				"entity_id": "input_select.test_dropdown",
			},
			wantError:    true,
			wantContains: "option is required",
		},
		{
			name: "empty option",
			args: map[string]any{
				"entity_id": "input_select.test_dropdown",
				"option":    "",
			},
			wantError:    true,
			wantContains: "option is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_select.test_dropdown",
				"option":    "option1",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error selecting option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputSelectClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewInputSelectHandlers()
			result, err := h.handleSelectOption(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleSelectOption() returned error: %v", err)
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
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestInputSelectHandlers_handleSetOptions(t *testing.T) {
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
				"entity_id": "input_select.test_dropdown",
				"options":   []any{"new_opt1", "new_opt2", "new_opt3"},
			},
			wantError:    false,
			wantContains: "Options updated successfully",
		},
		{
			name: "missing entity_id",
			args: map[string]any{
				"options": []any{"opt1", "opt2"},
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
				"options":   []any{"opt1", "opt2"},
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "input_number.test_number",
				"options":   []any{"opt1", "opt2"},
			},
			wantError:    true,
			wantContains: "must be an input_select entity",
		},
		{
			name: "missing options",
			args: map[string]any{
				"entity_id": "input_select.test_dropdown",
			},
			wantError:    true,
			wantContains: "options is required",
		},
		{
			name: "empty options array",
			args: map[string]any{
				"entity_id": "input_select.test_dropdown",
				"options":   []any{},
			},
			wantError:    true,
			wantContains: "options is required",
		},
		{
			name: "options with no strings",
			args: map[string]any{
				"entity_id": "input_select.test_dropdown",
				"options":   []any{123, 456},
			},
			wantError:    true,
			wantContains: "must contain at least one string value",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_select.test_dropdown",
				"options":   []any{"opt1", "opt2"},
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error setting options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputSelectClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewInputSelectHandlers()
			result, err := h.handleSetOptions(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleSetOptions() returned error: %v", err)
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
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestBuildInputSelectHelperConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputName  string
		args       map[string]any
		wantKeys   []string
		wantValues map[string]any
	}{
		{
			name:      "name only",
			inputName: "Test Dropdown",
			args:      map[string]any{},
			wantKeys:  []string{"name"},
			wantValues: map[string]any{
				"name": "Test Dropdown",
			},
		},
		{
			name:      "with icon",
			inputName: "Test Dropdown",
			args: map[string]any{
				"icon": "mdi:format-list-bulleted",
			},
			wantKeys: []string{"name", "icon"},
			wantValues: map[string]any{
				"name": "Test Dropdown",
				"icon": "mdi:format-list-bulleted",
			},
		},
		{
			name:      "with options",
			inputName: "Test Dropdown",
			args: map[string]any{
				"options": []any{"opt1", "opt2", "opt3"},
			},
			wantKeys: []string{"name", "options"},
		},
		{
			name:      "with initial",
			inputName: "Test Dropdown",
			args: map[string]any{
				"initial": "opt2",
			},
			wantKeys: []string{"name", "initial"},
			wantValues: map[string]any{
				"name":    "Test Dropdown",
				"initial": "opt2",
			},
		},
		{
			name:      "all options",
			inputName: "Test Dropdown",
			args: map[string]any{
				"icon":    "mdi:format-list-bulleted",
				"options": []any{"opt1", "opt2"},
				"initial": "opt1",
			},
			wantKeys: []string{"name", "icon", "options", "initial"},
		},
		{
			name:      "empty icon ignored",
			inputName: "Test Dropdown",
			args: map[string]any{
				"icon": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Dropdown",
			},
		},
		{
			name:      "empty initial ignored",
			inputName: "Test Dropdown",
			args: map[string]any{
				"initial": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Dropdown",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildInputSelectHelperConfig(tt.inputName, tt.args)

			for _, key := range tt.wantKeys {
				if _, ok := config[key]; !ok {
					t.Errorf("Config missing key %q", key)
				}
			}

			for key, wantVal := range tt.wantValues {
				if gotVal, ok := config[key]; !ok {
					t.Errorf("Config missing key %q", key)
				} else if gotVal != wantVal {
					t.Errorf("Config[%q] = %v, want %v", key, gotVal, wantVal)
				}
			}
		})
	}
}
