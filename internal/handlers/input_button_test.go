package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockInputButtonClient implements homeassistant.Client for testing.
type mockInputButtonClient struct {
	homeassistant.Client
	createHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	deleteHelperFn func(ctx context.Context, entityID string) error
	callServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockInputButtonClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.createHelperFn != nil {
		return m.createHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockInputButtonClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.deleteHelperFn != nil {
		return m.deleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockInputButtonClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewInputButtonHandlers(t *testing.T) {
	t.Parallel()

	h := NewInputButtonHandlers()
	if h == nil {
		t.Error("NewInputButtonHandlers() returned nil")
	}
}

func TestInputButtonHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewInputButtonHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 3 {
		t.Errorf("RegisterTools() registered %d tools, want 3", len(tools))
	}

	expectedTools := map[string]bool{
		"create_input_button": false,
		"delete_input_button": false,
		"press_input_button":  false,
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

func TestInputButtonHandlers_createInputButtonTool(t *testing.T) {
	t.Parallel()

	h := NewInputButtonHandlers()
	tool := h.createInputButtonTool()

	if tool.Name != "create_input_button" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "create_input_button")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	if len(tool.InputSchema.Required) != 2 {
		t.Errorf("InputSchema.Required length = %d, want 2", len(tool.InputSchema.Required))
	}

	requiredFields := map[string]bool{"id": false, "name": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}
}

func TestInputButtonHandlers_handleCreateInputButton(t *testing.T) {
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
				"id":   "test_button",
				"name": "Test Button",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with icon",
			args: map[string]any{
				"id":   "test_button",
				"name": "Test Button",
				"icon": "mdi:gesture-tap-button",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing id",
			args: map[string]any{
				"name": "Test Button",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "empty id",
			args: map[string]any{
				"id":   "",
				"name": "Test Button",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"id": "test_button",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"id":   "test_button",
				"name": "",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"id":   "test_button",
				"name": "Test Button",
			},
			createHelperErr: errors.New("connection failed"),
			wantError:       true,
			wantContains:    "Error creating input_button",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputButtonClient{
				createHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewInputButtonHandlers()
			result, err := h.handleCreateInputButton(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateInputButton() returned error: %v", err)
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

func TestInputButtonHandlers_handleDeleteInputButton(t *testing.T) {
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
				"entity_id": "input_button.test_button",
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
				"entity_id": "input_boolean.test_switch",
			},
			wantError:    true,
			wantContains: "must be an input_button entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_button.test_button",
			},
			deleteHelperErr: errors.New("not found"),
			wantError:       true,
			wantContains:    "Error deleting input_button",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputButtonClient{
				deleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewInputButtonHandlers()
			result, err := h.handleDeleteInputButton(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteInputButton() returned error: %v", err)
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

func TestInputButtonHandlers_handlePressInputButton(t *testing.T) {
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
				"entity_id": "input_button.test_button",
			},
			wantError:    false,
			wantContains: "pressed successfully",
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
				"entity_id": "input_number.test_number",
			},
			wantError:    true,
			wantContains: "must be an input_button entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_button.test_button",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error pressing input_button",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputButtonClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewInputButtonHandlers()
			result, err := h.handlePressInputButton(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handlePressInputButton() returned error: %v", err)
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

func TestBuildInputButtonHelperConfig(t *testing.T) {
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
			inputName: "Test Button",
			args:      map[string]any{},
			wantKeys:  []string{"name"},
			wantValues: map[string]any{
				"name": "Test Button",
			},
		},
		{
			name:      "with icon",
			inputName: "Test Button",
			args: map[string]any{
				"icon": "mdi:gesture-tap-button",
			},
			wantKeys: []string{"name", "icon"},
			wantValues: map[string]any{
				"name": "Test Button",
				"icon": "mdi:gesture-tap-button",
			},
		},
		{
			name:      "empty icon ignored",
			inputName: "Test Button",
			args: map[string]any{
				"icon": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Button",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildInputButtonHelperConfig(tt.inputName, tt.args)

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
