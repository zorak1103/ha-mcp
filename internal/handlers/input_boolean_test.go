package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockInputBooleanClient implements homeassistant.Client for testing.
type mockInputBooleanClient struct {
	homeassistant.Client
	createHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	deleteHelperFn func(ctx context.Context, entityID string) error
	callServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockInputBooleanClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.createHelperFn != nil {
		return m.createHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockInputBooleanClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.deleteHelperFn != nil {
		return m.deleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockInputBooleanClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewInputBooleanHandlers(t *testing.T) {
	t.Parallel()

	h := NewInputBooleanHandlers()
	if h == nil {
		t.Error("NewInputBooleanHandlers() returned nil")
	}
}

func TestInputBooleanHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewInputBooleanHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 3 {
		t.Errorf("RegisterTools() registered %d tools, want 3", len(tools))
	}

	expectedTools := map[string]bool{
		"create_input_boolean": false,
		"delete_input_boolean": false,
		"toggle_input_boolean": false,
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

func TestInputBooleanHandlers_createInputBooleanTool(t *testing.T) {
	t.Parallel()

	h := NewInputBooleanHandlers()
	tool := h.createInputBooleanTool()

	if tool.Name != "create_input_boolean" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "create_input_boolean")
	}

	if tool.InputSchema.Type != "object" {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, "object")
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

func TestInputBooleanHandlers_handleCreateInputBoolean(t *testing.T) {
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
				"id":   "test_switch",
				"name": "Test Switch",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with optional fields",
			args: map[string]any{
				"id":      "test_switch",
				"name":    "Test Switch",
				"icon":    "mdi:lightbulb",
				"initial": true,
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing id",
			args: map[string]any{
				"name": "Test Switch",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "empty id",
			args: map[string]any{
				"id":   "",
				"name": "Test Switch",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"id": "test_switch",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"id":   "test_switch",
				"name": "",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"id":   "test_switch",
				"name": "Test Switch",
			},
			createHelperErr: errors.New("connection failed"),
			wantError:       true,
			wantContains:    "Error creating input_boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputBooleanClient{
				createHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewInputBooleanHandlers()
			result, err := h.handleCreateInputBoolean(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateInputBoolean() returned error: %v", err)
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

func TestInputBooleanHandlers_handleDeleteInputBoolean(t *testing.T) {
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
				"entity_id": "input_boolean.test_switch",
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
			wantContains: "must be an input_boolean entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_boolean.test_switch",
			},
			deleteHelperErr: errors.New("not found"),
			wantError:       true,
			wantContains:    "Error deleting input_boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputBooleanClient{
				deleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewInputBooleanHandlers()
			result, err := h.handleDeleteInputBoolean(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteInputBoolean() returned error: %v", err)
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

func TestInputBooleanHandlers_handleToggleInputBoolean(t *testing.T) {
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
				"entity_id": "input_boolean.test_switch",
			},
			wantError:    false,
			wantContains: "toggled successfully",
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
			wantContains: "must be an input_boolean entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_boolean.test_switch",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error toggling input_boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputBooleanClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewInputBooleanHandlers()
			result, err := h.handleToggleInputBoolean(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleToggleInputBoolean() returned error: %v", err)
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

func TestBuildInputBooleanHelperConfig(t *testing.T) {
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
			inputName: "Test Switch",
			args:      map[string]any{},
			wantKeys:  []string{"name"},
			wantValues: map[string]any{
				"name": "Test Switch",
			},
		},
		{
			name:      "with icon",
			inputName: "Test Switch",
			args: map[string]any{
				"icon": "mdi:lightbulb",
			},
			wantKeys: []string{"name", "icon"},
			wantValues: map[string]any{
				"name": "Test Switch",
				"icon": "mdi:lightbulb",
			},
		},
		{
			name:      "with initial true",
			inputName: "Test Switch",
			args: map[string]any{
				"initial": true,
			},
			wantKeys: []string{"name", "initial"},
			wantValues: map[string]any{
				"name":    "Test Switch",
				"initial": true,
			},
		},
		{
			name:      "with initial false",
			inputName: "Test Switch",
			args: map[string]any{
				"initial": false,
			},
			wantKeys: []string{"name", "initial"},
			wantValues: map[string]any{
				"name":    "Test Switch",
				"initial": false,
			},
		},
		{
			name:      "all options",
			inputName: "Test Switch",
			args: map[string]any{
				"icon":    "mdi:lightbulb",
				"initial": true,
			},
			wantKeys: []string{"name", "icon", "initial"},
			wantValues: map[string]any{
				"name":    "Test Switch",
				"icon":    "mdi:lightbulb",
				"initial": true,
			},
		},
		{
			name:      "empty icon ignored",
			inputName: "Test Switch",
			args: map[string]any{
				"icon": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Switch",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildInputBooleanHelperConfig(tt.inputName, tt.args)

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

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
