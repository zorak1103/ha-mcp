package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockCounterClient implements homeassistant.Client for testing.
type mockCounterClient struct {
	homeassistant.Client
	createHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	deleteHelperFn func(ctx context.Context, entityID string) error
	callServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockCounterClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.createHelperFn != nil {
		return m.createHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockCounterClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.deleteHelperFn != nil {
		return m.deleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockCounterClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewCounterHandlers(t *testing.T) {
	t.Parallel()

	h := NewCounterHandlers()
	if h == nil {
		t.Error("NewCounterHandlers() returned nil")
	}
}

func TestCounterHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewCounterHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 6
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	expectedTools := map[string]bool{
		"create_counter":    false,
		"delete_counter":    false,
		"increment_counter": false,
		"decrement_counter": false,
		"reset_counter":     false,
		"set_counter_value": false,
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

func TestCounterHandlers_createCounterTool(t *testing.T) {
	t.Parallel()

	h := NewCounterHandlers()
	tool := h.createCounterTool()

	if tool.Name != "create_counter" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "create_counter")
	}

	if tool.InputSchema.Type != "object" {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, "object")
	}

	const expectedRequiredCount = 2
	if len(tool.InputSchema.Required) != expectedRequiredCount {
		t.Errorf("InputSchema.Required length = %d, want %d", len(tool.InputSchema.Required), expectedRequiredCount)
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
	optionalProps := []string{"initial", "step", "minimum", "maximum", "icon"}
	for _, prop := range optionalProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Optional property %q not found in schema", prop)
		}
	}
}

func TestCounterHandlers_setCounterValueTool(t *testing.T) {
	t.Parallel()

	h := NewCounterHandlers()
	tool := h.setCounterValueTool()

	if tool.Name != "set_counter_value" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "set_counter_value")
	}

	const expectedRequiredCount = 2
	if len(tool.InputSchema.Required) != expectedRequiredCount {
		t.Errorf("InputSchema.Required length = %d, want %d", len(tool.InputSchema.Required), expectedRequiredCount)
	}

	requiredFields := map[string]bool{"entity_id": false, "value": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}
}

func TestCounterHandlers_handleCreateCounter(t *testing.T) {
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
				"id":   "test_counter",
				"name": "Test Counter",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with all options",
			args: map[string]any{
				"id":      "test_counter",
				"name":    "Test Counter",
				"initial": float64(10),
				"step":    float64(5),
				"minimum": float64(0),
				"maximum": float64(100),
				"icon":    "mdi:counter",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing id",
			args: map[string]any{
				"name": "Test Counter",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "empty id",
			args: map[string]any{
				"id":   "",
				"name": "Test Counter",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"id": "test_counter",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"id":   "test_counter",
				"name": "",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"id":   "test_counter",
				"name": "Test Counter",
			},
			createHelperErr: errors.New("connection failed"),
			wantError:       true,
			wantContains:    "Error creating counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockCounterClient{
				createHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewCounterHandlers()
			result, err := h.handleCreateCounter(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateCounter() returned error: %v", err)
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

func TestCounterHandlers_handleDeleteCounter(t *testing.T) {
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
				"entity_id": "counter.test_counter",
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
				"entity_id": "timer.test_timer",
			},
			wantError:    true,
			wantContains: "must be a counter entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "counter.test_counter",
			},
			deleteHelperErr: errors.New("not found"),
			wantError:       true,
			wantContains:    "Error deleting counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockCounterClient{
				deleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewCounterHandlers()
			result, err := h.handleDeleteCounter(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteCounter() returned error: %v", err)
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

func TestCounterHandlers_handleIncrementCounter(t *testing.T) {
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
				"entity_id": "counter.test_counter",
			},
			wantError:    false,
			wantContains: "incremented successfully",
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
			wantContains: "must be a counter entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "counter.test_counter",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error incrementing counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockCounterClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewCounterHandlers()
			result, err := h.handleIncrementCounter(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleIncrementCounter() returned error: %v", err)
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

func TestCounterHandlers_handleDecrementCounter(t *testing.T) {
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
				"entity_id": "counter.test_counter",
			},
			wantError:    false,
			wantContains: "decremented successfully",
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
			wantContains: "must be a counter entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "counter.test_counter",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error decrementing counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockCounterClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewCounterHandlers()
			result, err := h.handleDecrementCounter(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDecrementCounter() returned error: %v", err)
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

func TestCounterHandlers_handleResetCounter(t *testing.T) {
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
				"entity_id": "counter.test_counter",
			},
			wantError:    false,
			wantContains: "reset successfully",
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
				"entity_id": "input_select.test_select",
			},
			wantError:    true,
			wantContains: "must be a counter entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "counter.test_counter",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error resetting counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockCounterClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewCounterHandlers()
			result, err := h.handleResetCounter(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleResetCounter() returned error: %v", err)
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

func TestCounterHandlers_handleSetCounterValue(t *testing.T) {
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
				"entity_id": "counter.test_counter",
				"value":     float64(42),
			},
			wantError:    false,
			wantContains: "set to 42 successfully",
		},
		{
			name: "success with zero",
			args: map[string]any{
				"entity_id": "counter.test_counter",
				"value":     float64(0),
			},
			wantError:    false,
			wantContains: "set to 0 successfully",
		},
		{
			name: "success with negative",
			args: map[string]any{
				"entity_id": "counter.test_counter",
				"value":     float64(-10),
			},
			wantError:    false,
			wantContains: "set to -10 successfully",
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
				"value":     float64(10),
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "input_text.test_text",
				"value":     float64(10),
			},
			wantError:    true,
			wantContains: "must be a counter entity",
		},
		{
			name: "missing value",
			args: map[string]any{
				"entity_id": "counter.test_counter",
			},
			wantError:    true,
			wantContains: "value is required",
		},
		{
			name: "invalid value type",
			args: map[string]any{
				"entity_id": "counter.test_counter",
				"value":     "not a number",
			},
			wantError:    true,
			wantContains: "value is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "counter.test_counter",
				"value":     float64(10),
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error setting counter value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockCounterClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewCounterHandlers()
			result, err := h.handleSetCounterValue(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleSetCounterValue() returned error: %v", err)
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

func TestBuildCounterHelperConfig(t *testing.T) {
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
			inputName: "Test Counter",
			args:      map[string]any{},
			wantKeys:  []string{"name"},
			wantValues: map[string]any{
				"name": "Test Counter",
			},
		},
		{
			name:      "with initial",
			inputName: "Test Counter",
			args: map[string]any{
				"initial": float64(10),
			},
			wantKeys: []string{"name", "initial"},
			wantValues: map[string]any{
				"name":    "Test Counter",
				"initial": 10,
			},
		},
		{
			name:      "with step",
			inputName: "Test Counter",
			args: map[string]any{
				"step": float64(5),
			},
			wantKeys: []string{"name", "step"},
			wantValues: map[string]any{
				"name": "Test Counter",
				"step": 5,
			},
		},
		{
			name:      "with minimum",
			inputName: "Test Counter",
			args: map[string]any{
				"minimum": float64(0),
			},
			wantKeys: []string{"name", "minimum"},
			wantValues: map[string]any{
				"name":    "Test Counter",
				"minimum": 0,
			},
		},
		{
			name:      "with maximum",
			inputName: "Test Counter",
			args: map[string]any{
				"maximum": float64(100),
			},
			wantKeys: []string{"name", "maximum"},
			wantValues: map[string]any{
				"name":    "Test Counter",
				"maximum": 100,
			},
		},
		{
			name:      "with icon",
			inputName: "Test Counter",
			args: map[string]any{
				"icon": "mdi:counter",
			},
			wantKeys: []string{"name", "icon"},
			wantValues: map[string]any{
				"name": "Test Counter",
				"icon": "mdi:counter",
			},
		},
		{
			name:      "all options",
			inputName: "Test Counter",
			args: map[string]any{
				"initial": float64(10),
				"step":    float64(2),
				"minimum": float64(0),
				"maximum": float64(100),
				"icon":    "mdi:numeric",
			},
			wantKeys: []string{"name", "initial", "step", "minimum", "maximum", "icon"},
			wantValues: map[string]any{
				"name":    "Test Counter",
				"initial": 10,
				"step":    2,
				"minimum": 0,
				"maximum": 100,
				"icon":    "mdi:numeric",
			},
		},
		{
			name:      "empty icon ignored",
			inputName: "Test Counter",
			args: map[string]any{
				"icon": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Counter",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildCounterHelperConfig(tt.inputName, tt.args)

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
