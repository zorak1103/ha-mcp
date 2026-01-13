package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockInputDatetimeClient implements homeassistant.Client for testing.
type mockInputDatetimeClient struct {
	homeassistant.Client
	createHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	deleteHelperFn func(ctx context.Context, entityID string) error
	callServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockInputDatetimeClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.createHelperFn != nil {
		return m.createHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockInputDatetimeClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.deleteHelperFn != nil {
		return m.deleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockInputDatetimeClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewInputDatetimeHandlers(t *testing.T) {
	t.Parallel()

	h := NewInputDatetimeHandlers()
	if h == nil {
		t.Error("NewInputDatetimeHandlers() returned nil")
	}
}

func TestInputDatetimeHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewInputDatetimeHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 3 {
		t.Errorf("RegisterTools() registered %d tools, want 3", len(tools))
	}

	expectedTools := map[string]bool{
		"create_input_datetime": false,
		"delete_input_datetime": false,
		"set_input_datetime":    false,
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

func TestInputDatetimeHandlers_createInputDatetimeTool(t *testing.T) {
	t.Parallel()

	h := NewInputDatetimeHandlers()
	tool := h.createInputDatetimeTool()

	if tool.Name != "create_input_datetime" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "create_input_datetime")
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

	// Check optional properties exist
	optionalProps := []string{"icon", "has_date", "has_time", "initial"}
	for _, prop := range optionalProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Optional property %q not found in schema", prop)
		}
	}
}

func TestInputDatetimeHandlers_handleCreateInputDatetime(t *testing.T) {
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
				"id":   "test_datetime",
				"name": "Test Datetime",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with optional fields",
			args: map[string]any{
				"id":       "test_datetime",
				"name":     "Test Datetime",
				"icon":     "mdi:calendar-clock",
				"has_date": true,
				"has_time": true,
				"initial":  "2024-01-01 12:00:00",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with date only",
			args: map[string]any{
				"id":       "date_picker",
				"name":     "Date Picker",
				"has_date": true,
				"has_time": false,
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with time only",
			args: map[string]any{
				"id":       "time_picker",
				"name":     "Time Picker",
				"has_date": false,
				"has_time": true,
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing id",
			args: map[string]any{
				"name": "Test Datetime",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "empty id",
			args: map[string]any{
				"id":   "",
				"name": "Test Datetime",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"id": "test_datetime",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"id":   "test_datetime",
				"name": "",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"id":   "test_datetime",
				"name": "Test Datetime",
			},
			createHelperErr: errors.New("connection failed"),
			wantError:       true,
			wantContains:    "Error creating input_datetime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputDatetimeClient{
				createHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewInputDatetimeHandlers()
			result, err := h.handleCreateInputDatetime(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateInputDatetime() returned error: %v", err)
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

func TestInputDatetimeHandlers_handleDeleteInputDatetime(t *testing.T) {
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
				"entity_id": "input_datetime.test_datetime",
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
			wantContains: "must be an input_datetime entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_datetime.test_datetime",
			},
			deleteHelperErr: errors.New("not found"),
			wantError:       true,
			wantContains:    "Error deleting input_datetime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputDatetimeClient{
				deleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewInputDatetimeHandlers()
			result, err := h.handleDeleteInputDatetime(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteInputDatetime() returned error: %v", err)
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

func TestInputDatetimeHandlers_handleSetInputDatetime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		callServiceErr error
		wantError      bool
		wantContains   string
	}{
		{
			name: "success with datetime",
			args: map[string]any{
				"entity_id": "input_datetime.test_datetime",
				"datetime":  "2024-01-15 14:30:00",
			},
			wantError:    false,
			wantContains: "set successfully",
		},
		{
			name: "success with date only",
			args: map[string]any{
				"entity_id": "input_datetime.date_picker",
				"date":      "2024-01-15",
			},
			wantError:    false,
			wantContains: "set successfully",
		},
		{
			name: "success with time only",
			args: map[string]any{
				"entity_id": "input_datetime.time_picker",
				"time":      "14:30:00",
			},
			wantError:    false,
			wantContains: "set successfully",
		},
		{
			name: "success with multiple values",
			args: map[string]any{
				"entity_id": "input_datetime.test_datetime",
				"date":      "2024-01-15",
				"time":      "14:30:00",
			},
			wantError:    false,
			wantContains: "set successfully",
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
				"datetime":  "2024-01-15 14:30:00",
			},
			wantError:    true,
			wantContains: "must be an input_datetime entity",
		},
		{
			name: "missing value - no datetime date or time",
			args: map[string]any{
				"entity_id": "input_datetime.test_datetime",
			},
			wantError:    true,
			wantContains: "At least one of datetime, date, or time is required",
		},
		{
			name: "empty values",
			args: map[string]any{
				"entity_id": "input_datetime.test_datetime",
				"datetime":  "",
				"date":      "",
				"time":      "",
			},
			wantError:    true,
			wantContains: "At least one of datetime, date, or time is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "input_datetime.test_datetime",
				"datetime":  "2024-01-15 14:30:00",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error setting input_datetime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockInputDatetimeClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewInputDatetimeHandlers()
			result, err := h.handleSetInputDatetime(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleSetInputDatetime() returned error: %v", err)
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

func TestBuildInputDatetimeHelperConfig(t *testing.T) {
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
			inputName: "Test Datetime",
			args:      map[string]any{},
			wantKeys:  []string{"name"},
			wantValues: map[string]any{
				"name": "Test Datetime",
			},
		},
		{
			name:      "with icon",
			inputName: "Test Datetime",
			args: map[string]any{
				"icon": "mdi:calendar-clock",
			},
			wantKeys: []string{"name", "icon"},
			wantValues: map[string]any{
				"name": "Test Datetime",
				"icon": "mdi:calendar-clock",
			},
		},
		{
			name:      "with has_date true",
			inputName: "Date Picker",
			args: map[string]any{
				"has_date": true,
			},
			wantKeys: []string{"name", "has_date"},
			wantValues: map[string]any{
				"name":     "Date Picker",
				"has_date": true,
			},
		},
		{
			name:      "with has_date false",
			inputName: "Time Only",
			args: map[string]any{
				"has_date": false,
			},
			wantKeys: []string{"name", "has_date"},
			wantValues: map[string]any{
				"name":     "Time Only",
				"has_date": false,
			},
		},
		{
			name:      "with has_time true",
			inputName: "Time Picker",
			args: map[string]any{
				"has_time": true,
			},
			wantKeys: []string{"name", "has_time"},
			wantValues: map[string]any{
				"name":     "Time Picker",
				"has_time": true,
			},
		},
		{
			name:      "with has_time false",
			inputName: "Date Only",
			args: map[string]any{
				"has_time": false,
			},
			wantKeys: []string{"name", "has_time"},
			wantValues: map[string]any{
				"name":     "Date Only",
				"has_time": false,
			},
		},
		{
			name:      "with initial value",
			inputName: "Test Datetime",
			args: map[string]any{
				"initial": "2024-01-01",
			},
			wantKeys: []string{"name", "initial"},
			wantValues: map[string]any{
				"name":    "Test Datetime",
				"initial": "2024-01-01",
			},
		},
		{
			name:      "all options datetime",
			inputName: "Full Datetime",
			args: map[string]any{
				"icon":     "mdi:calendar-clock",
				"has_date": true,
				"has_time": true,
				"initial":  "2024-01-01 12:00:00",
			},
			wantKeys: []string{"name", "icon", "has_date", "has_time", "initial"},
			wantValues: map[string]any{
				"name":     "Full Datetime",
				"icon":     "mdi:calendar-clock",
				"has_date": true,
				"has_time": true,
				"initial":  "2024-01-01 12:00:00",
			},
		},
		{
			name:      "empty icon ignored",
			inputName: "Test Datetime",
			args: map[string]any{
				"icon": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Datetime",
			},
		},
		{
			name:      "empty initial ignored",
			inputName: "Test Datetime",
			args: map[string]any{
				"initial": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Datetime",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildInputDatetimeHelperConfig(tt.inputName, tt.args)

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
