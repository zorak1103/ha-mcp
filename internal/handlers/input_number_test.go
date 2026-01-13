package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockInputNumberClient implements homeassistant.Client for testing.
type mockInputNumberClient struct {
	homeassistant.Client
	createHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	deleteHelperFn func(ctx context.Context, entityID string) error
	callServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockInputNumberClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.createHelperFn != nil {
		return m.createHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockInputNumberClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.deleteHelperFn != nil {
		return m.deleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockInputNumberClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewInputNumberHandlers(t *testing.T) {
	t.Parallel()

	h := NewInputNumberHandlers()
	if h == nil {
		t.Error("NewInputNumberHandlers() returned nil")
	}
}

func TestInputNumberHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewInputNumberHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 3 {
		t.Errorf("RegisterTools() registered %d tools, want 3", len(tools))
	}

	expectedTools := map[string]bool{
		"create_input_number":    false,
		"delete_input_number":    false,
		"set_input_number_value": false,
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

func TestInputNumberHandlers_createInputNumberTool(t *testing.T) {
	t.Parallel()

	h := NewInputNumberHandlers()
	tool := h.createInputNumberTool()

	if tool.Name != "create_input_number" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "create_input_number")
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

	// Check optional properties exist
	optionalProps := []string{"icon", "min", "max", "step", "initial", "mode", "unit_of_measurement"}
	for _, prop := range optionalProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Optional property %q not found in schema", prop)
		}
	}
}

func TestInputNumberHandlers_handleCreateInputNumber(t *testing.T) {
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
				"id":   "test_number",
				"name": "Test Number",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with all options",
			args: map[string]any{
				"id":                  "test_number",
				"name":                "Test Number",
				"icon":                "mdi:counter",
				"min":                 0.0,
				"max":                 100.0,
				"step":                1.0,
				"initial":             50.0,
				"mode":                "slider",
				"unit_of_measurement": "°C",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with box mode",
			args: map[string]any{
				"id":   "test_number",
				"name": "Test Number",
				"mode": "box",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing id",
			args: map[string]any{
				"name": "Test Number",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "empty id",
			args: map[string]any{
				"id":   "",
				"name": "Test Number",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"id": "test_number",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"id":   "test_number",
				"name": "",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"id":   "test_number",
				"name": "Test Number",
			},
			createHelperErr: errors.New("connection failed"),
			wantError:       true,
			wantContains:    "Error creating input_number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputNumberClient{
				createHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewInputNumberHandlers()
			result, err := h.handleCreateInputNumber(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateInputNumber() returned error: %v", err)
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

func TestInputNumberHandlers_handleDeleteInputNumber(t *testing.T) {
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
				"entity_id": "input_number.test_number",
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
			name: "invalid platform - input_boolean",
			args: map[string]any{
				"entity_id": "input_boolean.test_switch",
			},
			wantError:    true,
			wantContains: "must be an input_number entity",
		},
		{
			name: "invalid platform - input_text",
			args: map[string]any{
				"entity_id": "input_text.test_text",
			},
			wantError:    true,
			wantContains: "must be an input_number entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_number.test_number",
			},
			deleteHelperErr: errors.New("not found"),
			wantError:       true,
			wantContains:    "Error deleting input_number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputNumberClient{
				deleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewInputNumberHandlers()
			result, err := h.handleDeleteInputNumber(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteInputNumber() returned error: %v", err)
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

func TestInputNumberHandlers_handleSetInputNumberValue(t *testing.T) {
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
				"entity_id": "input_number.test_number",
				"value":     42.0,
			},
			wantError:    false,
			wantContains: "value set to",
		},
		{
			name: "success with zero value",
			args: map[string]any{
				"entity_id": "input_number.test_number",
				"value":     0.0,
			},
			wantError:    false,
			wantContains: "value set to",
		},
		{
			name: "success with negative value",
			args: map[string]any{
				"entity_id": "input_number.test_number",
				"value":     -10.5,
			},
			wantError:    false,
			wantContains: "value set to",
		},
		{
			name: "success with decimal value",
			args: map[string]any{
				"entity_id": "input_number.test_number",
				"value":     23.75,
			},
			wantError:    false,
			wantContains: "value set to",
		},
		{
			name: "missing entity_id",
			args: map[string]any{
				"value": 42.0,
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
				"value":     42.0,
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "input_boolean.test_switch",
				"value":     42.0,
			},
			wantError:    true,
			wantContains: "must be an input_number entity",
		},
		{
			name: "missing value",
			args: map[string]any{
				"entity_id": "input_number.test_number",
			},
			wantError:    true,
			wantContains: "value is required",
		},
		{
			name: "value wrong type - string",
			args: map[string]any{
				"entity_id": "input_number.test_number",
				"value":     "not a number",
			},
			wantError:    true,
			wantContains: "value is required and must be a number",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_number.test_number",
				"value":     42.0,
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error setting input_number value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputNumberClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewInputNumberHandlers()
			result, err := h.handleSetInputNumberValue(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleSetInputNumberValue() returned error: %v", err)
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

func TestBuildInputNumberHelperConfig(t *testing.T) {
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
			inputName: "Test Number",
			args:      map[string]any{},
			wantKeys:  []string{"name"},
			wantValues: map[string]any{
				"name": "Test Number",
			},
		},
		{
			name:      "with icon",
			inputName: "Test Number",
			args: map[string]any{
				"icon": "mdi:counter",
			},
			wantKeys: []string{"name", "icon"},
			wantValues: map[string]any{
				"name": "Test Number",
				"icon": "mdi:counter",
			},
		},
		{
			name:      "with min value",
			inputName: "Test Number",
			args: map[string]any{
				"min": 0.0,
			},
			wantKeys: []string{"name", "min"},
			wantValues: map[string]any{
				"name": "Test Number",
				"min":  0.0,
			},
		},
		{
			name:      "with max value",
			inputName: "Test Number",
			args: map[string]any{
				"max": 100.0,
			},
			wantKeys: []string{"name", "max"},
			wantValues: map[string]any{
				"name": "Test Number",
				"max":  100.0,
			},
		},
		{
			name:      "with step",
			inputName: "Test Number",
			args: map[string]any{
				"step": 0.5,
			},
			wantKeys: []string{"name", "step"},
			wantValues: map[string]any{
				"name": "Test Number",
				"step": 0.5,
			},
		},
		{
			name:      "with initial value",
			inputName: "Test Number",
			args: map[string]any{
				"initial": 50.0,
			},
			wantKeys: []string{"name", "initial"},
			wantValues: map[string]any{
				"name":    "Test Number",
				"initial": 50.0,
			},
		},
		{
			name:      "with slider mode",
			inputName: "Test Number",
			args: map[string]any{
				"mode": "slider",
			},
			wantKeys: []string{"name", "mode"},
			wantValues: map[string]any{
				"name": "Test Number",
				"mode": "slider",
			},
		},
		{
			name:      "with box mode",
			inputName: "Test Number",
			args: map[string]any{
				"mode": "box",
			},
			wantKeys: []string{"name", "mode"},
			wantValues: map[string]any{
				"name": "Test Number",
				"mode": "box",
			},
		},
		{
			name:      "with unit of measurement",
			inputName: "Temperature",
			args: map[string]any{
				"unit_of_measurement": "°C",
			},
			wantKeys: []string{"name", "unit_of_measurement"},
			wantValues: map[string]any{
				"name":                "Temperature",
				"unit_of_measurement": "°C",
			},
		},
		{
			name:      "all options",
			inputName: "Full Config Number",
			args: map[string]any{
				"icon":                "mdi:thermometer",
				"min":                 -20.0,
				"max":                 50.0,
				"step":                0.1,
				"initial":             20.0,
				"mode":                "slider",
				"unit_of_measurement": "°C",
			},
			wantKeys: []string{"name", "icon", "min", "max", "step", "initial", "mode", "unit_of_measurement"},
			wantValues: map[string]any{
				"name":                "Full Config Number",
				"icon":                "mdi:thermometer",
				"min":                 -20.0,
				"max":                 50.0,
				"step":                0.1,
				"initial":             20.0,
				"mode":                "slider",
				"unit_of_measurement": "°C",
			},
		},
		{
			name:      "empty icon ignored",
			inputName: "Test Number",
			args: map[string]any{
				"icon": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Number",
			},
		},
		{
			name:      "empty mode ignored",
			inputName: "Test Number",
			args: map[string]any{
				"mode": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Number",
			},
		},
		{
			name:      "empty unit ignored",
			inputName: "Test Number",
			args: map[string]any{
				"unit_of_measurement": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Number",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildInputNumberHelperConfig(tt.inputName, tt.args)

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
