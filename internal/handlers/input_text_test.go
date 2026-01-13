package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockInputTextClient implements homeassistant.Client for testing.
type mockInputTextClient struct {
	homeassistant.Client
	createHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	deleteHelperFn func(ctx context.Context, entityID string) error
	callServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockInputTextClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.createHelperFn != nil {
		return m.createHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockInputTextClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.deleteHelperFn != nil {
		return m.deleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockInputTextClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewInputTextHandlers(t *testing.T) {
	t.Parallel()

	h := NewInputTextHandlers()
	if h == nil {
		t.Error("NewInputTextHandlers() returned nil")
	}
}

func TestInputTextHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewInputTextHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 3 {
		t.Errorf("RegisterTools() registered %d tools, want 3", len(tools))
	}

	expectedTools := map[string]bool{
		"create_input_text":    false,
		"delete_input_text":    false,
		"set_input_text_value": false,
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

func TestInputTextHandlers_createInputTextTool(t *testing.T) {
	t.Parallel()

	h := NewInputTextHandlers()
	tool := h.createInputTextTool()

	if tool.Name != "create_input_text" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "create_input_text")
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

func TestInputTextHandlers_handleCreateInputText(t *testing.T) {
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
				"id":   "test_text",
				"name": "Test Text",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with all optional fields",
			args: map[string]any{
				"id":      "test_text",
				"name":    "Test Text",
				"icon":    "mdi:text",
				"min":     float64(5),
				"max":     float64(100),
				"initial": "Hello",
				"mode":    "text",
				"pattern": "[a-zA-Z]+",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing id",
			args: map[string]any{
				"name": "Test Text",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "empty id",
			args: map[string]any{
				"id":   "",
				"name": "Test Text",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"id": "test_text",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"id":   "test_text",
				"name": "",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"id":   "test_text",
				"name": "Test Text",
			},
			createHelperErr: errors.New("connection failed"),
			wantError:       true,
			wantContains:    "Error creating input_text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputTextClient{
				createHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewInputTextHandlers()
			result, err := h.handleCreateInputText(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateInputText() returned error: %v", err)
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

func TestInputTextHandlers_handleDeleteInputText(t *testing.T) {
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
				"entity_id": "input_text.test_text",
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
			wantContains: "must be an input_text entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_text.test_text",
			},
			deleteHelperErr: errors.New("not found"),
			wantError:       true,
			wantContains:    "Error deleting input_text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputTextClient{
				deleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewInputTextHandlers()
			result, err := h.handleDeleteInputText(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteInputText() returned error: %v", err)
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

func TestInputTextHandlers_handleSetInputTextValue(t *testing.T) {
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
				"entity_id": "input_text.test_text",
				"value":     "Hello World",
			},
			wantError:    false,
			wantContains: "value set successfully",
		},
		{
			name: "missing entity_id",
			args: map[string]any{
				"value": "Hello World",
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
				"value":     "Hello World",
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "input_boolean.test_switch",
				"value":     "Hello World",
			},
			wantError:    true,
			wantContains: "must be an input_text entity",
		},
		{
			name: "missing value",
			args: map[string]any{
				"entity_id": "input_text.test_text",
			},
			wantError:    true,
			wantContains: "value is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_text.test_text",
				"value":     "Hello World",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error setting input_text value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputTextClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewInputTextHandlers()
			result, err := h.handleSetInputTextValue(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleSetInputTextValue() returned error: %v", err)
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

func TestBuildInputTextHelperConfig(t *testing.T) {
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
			inputName: "Test Text",
			args:      map[string]any{},
			wantKeys:  []string{"name"},
			wantValues: map[string]any{
				"name": "Test Text",
			},
		},
		{
			name:      "with icon",
			inputName: "Test Text",
			args: map[string]any{
				"icon": "mdi:text",
			},
			wantKeys: []string{"name", "icon"},
			wantValues: map[string]any{
				"name": "Test Text",
				"icon": "mdi:text",
			},
		},
		{
			name:      "with min",
			inputName: "Test Text",
			args: map[string]any{
				"min": float64(5),
			},
			wantKeys: []string{"name", "min"},
			wantValues: map[string]any{
				"name": "Test Text",
				"min":  5,
			},
		},
		{
			name:      "with max",
			inputName: "Test Text",
			args: map[string]any{
				"max": float64(100),
			},
			wantKeys: []string{"name", "max"},
			wantValues: map[string]any{
				"name": "Test Text",
				"max":  100,
			},
		},
		{
			name:      "with initial",
			inputName: "Test Text",
			args: map[string]any{
				"initial": "Hello",
			},
			wantKeys: []string{"name", "initial"},
			wantValues: map[string]any{
				"name":    "Test Text",
				"initial": "Hello",
			},
		},
		{
			name:      "with mode",
			inputName: "Test Text",
			args: map[string]any{
				"mode": "password",
			},
			wantKeys: []string{"name", "mode"},
			wantValues: map[string]any{
				"name": "Test Text",
				"mode": "password",
			},
		},
		{
			name:      "with pattern",
			inputName: "Test Text",
			args: map[string]any{
				"pattern": "[a-zA-Z]+",
			},
			wantKeys: []string{"name", "pattern"},
			wantValues: map[string]any{
				"name":    "Test Text",
				"pattern": "[a-zA-Z]+",
			},
		},
		{
			name:      "all options",
			inputName: "Test Text",
			args: map[string]any{
				"icon":    "mdi:text",
				"min":     float64(5),
				"max":     float64(100),
				"initial": "Hello",
				"mode":    "text",
				"pattern": "[a-zA-Z]+",
			},
			wantKeys: []string{"name", "icon", "min", "max", "initial", "mode", "pattern"},
			wantValues: map[string]any{
				"name":    "Test Text",
				"icon":    "mdi:text",
				"min":     5,
				"max":     100,
				"initial": "Hello",
				"mode":    "text",
				"pattern": "[a-zA-Z]+",
			},
		},
		{
			name:      "empty strings ignored",
			inputName: "Test Text",
			args: map[string]any{
				"icon":    "",
				"initial": "",
				"mode":    "",
				"pattern": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildInputTextHelperConfig(tt.inputName, tt.args)

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
