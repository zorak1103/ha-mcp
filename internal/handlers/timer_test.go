package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockTimerClient implements homeassistant.Client for testing.
type mockTimerClient struct {
	homeassistant.Client
	createHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	deleteHelperFn func(ctx context.Context, entityID string) error
	callServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockTimerClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.createHelperFn != nil {
		return m.createHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockTimerClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.deleteHelperFn != nil {
		return m.deleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockTimerClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewTimerHandlers(t *testing.T) {
	t.Parallel()

	h := NewTimerHandlers()
	if h == nil {
		t.Error("NewTimerHandlers() returned nil")
	}
}

func TestTimerHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewTimerHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 7
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	expectedTools := map[string]bool{
		"create_timer": false,
		"delete_timer": false,
		"start_timer":  false,
		"pause_timer":  false,
		"cancel_timer": false,
		"finish_timer": false,
		"change_timer": false,
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

func TestTimerHandlers_createTimerTool(t *testing.T) {
	t.Parallel()

	h := NewTimerHandlers()
	tool := h.createTimerTool()

	if tool.Name != "create_timer" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "create_timer")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
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
}

func TestTimerHandlers_deleteTimerTool(t *testing.T) {
	t.Parallel()

	h := NewTimerHandlers()
	tool := h.deleteTimerTool()

	if tool.Name != "delete_timer" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "delete_timer")
	}

	requiredFields := map[string]bool{"entity_id": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}
}

func TestTimerHandlers_startTimerTool(t *testing.T) {
	t.Parallel()

	h := NewTimerHandlers()
	tool := h.startTimerTool()

	if tool.Name != "start_timer" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "start_timer")
	}

	requiredFields := map[string]bool{"entity_id": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}

	// Check optional duration property exists
	if _, ok := tool.InputSchema.Properties["duration"]; !ok {
		t.Error("Optional 'duration' property not found in schema")
	}
}

func TestTimerHandlers_changeTimerTool(t *testing.T) {
	t.Parallel()

	h := NewTimerHandlers()
	tool := h.changeTimerTool()

	if tool.Name != "change_timer" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "change_timer")
	}

	const expectedRequiredCount = 2
	if len(tool.InputSchema.Required) != expectedRequiredCount {
		t.Errorf("InputSchema.Required length = %d, want %d", len(tool.InputSchema.Required), expectedRequiredCount)
	}

	requiredFields := map[string]bool{"entity_id": false, "duration": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}
}

func TestTimerHandlers_handleCreateTimer(t *testing.T) {
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
				"id":   "test_timer",
				"name": "Test Timer",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with optional fields",
			args: map[string]any{
				"id":       "test_timer",
				"name":     "Test Timer",
				"duration": "00:05:00",
				"restore":  true,
				"icon":     "mdi:timer",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing id",
			args: map[string]any{
				"name": "Test Timer",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "empty id",
			args: map[string]any{
				"id":   "",
				"name": "Test Timer",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"id": "test_timer",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"id":   "test_timer",
				"name": "",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"id":   "test_timer",
				"name": "Test Timer",
			},
			createHelperErr: errors.New("connection failed"),
			wantError:       true,
			wantContains:    "Error creating timer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTimerClient{
				createHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewTimerHandlers()
			result, err := h.handleCreateTimer(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateTimer() returned error: %v", err)
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

func TestTimerHandlers_handleDeleteTimer(t *testing.T) {
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
				"entity_id": "timer.test_timer",
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
			wantContains: "must be a timer entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "timer.test_timer",
			},
			deleteHelperErr: errors.New("not found"),
			wantError:       true,
			wantContains:    "Error deleting timer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTimerClient{
				deleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewTimerHandlers()
			result, err := h.handleDeleteTimer(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteTimer() returned error: %v", err)
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

func TestTimerHandlers_handleStartTimer(t *testing.T) {
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
				"entity_id": "timer.test_timer",
			},
			wantError:    false,
			wantContains: "started successfully",
		},
		{
			name: "success with duration",
			args: map[string]any{
				"entity_id": "timer.test_timer",
				"duration":  "00:10:00",
			},
			wantError:    false,
			wantContains: "started successfully",
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
			wantContains: "must be a timer entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "timer.test_timer",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error starting timer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTimerClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewTimerHandlers()
			result, err := h.handleStartTimer(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleStartTimer() returned error: %v", err)
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

func TestTimerHandlers_handlePauseTimer(t *testing.T) {
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
				"entity_id": "timer.test_timer",
			},
			wantError:    false,
			wantContains: "paused successfully",
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
			wantContains: "must be a timer entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "timer.test_timer",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error pausing timer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTimerClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewTimerHandlers()
			result, err := h.handlePauseTimer(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handlePauseTimer() returned error: %v", err)
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

func TestTimerHandlers_handleCancelTimer(t *testing.T) {
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
				"entity_id": "timer.test_timer",
			},
			wantError:    false,
			wantContains: "canceled successfully",
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
				"entity_id": "counter.test_counter",
			},
			wantError:    true,
			wantContains: "must be a timer entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "timer.test_timer",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error canceling timer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTimerClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewTimerHandlers()
			result, err := h.handleCancelTimer(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCancelTimer() returned error: %v", err)
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

func TestTimerHandlers_handleFinishTimer(t *testing.T) {
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
				"entity_id": "timer.test_timer",
			},
			wantError:    false,
			wantContains: "finished successfully",
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
			wantContains: "must be a timer entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "timer.test_timer",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error finishing timer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTimerClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewTimerHandlers()
			result, err := h.handleFinishTimer(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleFinishTimer() returned error: %v", err)
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

func TestTimerHandlers_handleChangeTimer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		callServiceErr error
		wantError      bool
		wantContains   string
	}{
		{
			name: "success add time",
			args: map[string]any{
				"entity_id": "timer.test_timer",
				"duration":  "00:01:00",
			},
			wantError:    false,
			wantContains: "duration changed",
		},
		{
			name: "success subtract time",
			args: map[string]any{
				"entity_id": "timer.test_timer",
				"duration":  "-00:00:30",
			},
			wantError:    false,
			wantContains: "duration changed",
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
				"duration":  "00:01:00",
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "input_datetime.test_datetime",
				"duration":  "00:01:00",
			},
			wantError:    true,
			wantContains: "must be a timer entity",
		},
		{
			name: "missing duration",
			args: map[string]any{
				"entity_id": "timer.test_timer",
			},
			wantError:    true,
			wantContains: "duration is required",
		},
		{
			name: "empty duration",
			args: map[string]any{
				"entity_id": "timer.test_timer",
				"duration":  "",
			},
			wantError:    true,
			wantContains: "duration is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "timer.test_timer",
				"duration":  "00:01:00",
			},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error changing timer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTimerClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewTimerHandlers()
			result, err := h.handleChangeTimer(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleChangeTimer() returned error: %v", err)
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

func TestBuildTimerHelperConfig(t *testing.T) {
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
			inputName: "Test Timer",
			args:      map[string]any{},
			wantKeys:  []string{"name"},
			wantValues: map[string]any{
				"name": "Test Timer",
			},
		},
		{
			name:      "with duration",
			inputName: "Test Timer",
			args: map[string]any{
				"duration": "00:05:00",
			},
			wantKeys: []string{"name", "duration"},
			wantValues: map[string]any{
				"name":     "Test Timer",
				"duration": "00:05:00",
			},
		},
		{
			name:      "with restore true",
			inputName: "Test Timer",
			args: map[string]any{
				"restore": true,
			},
			wantKeys: []string{"name", "restore"},
			wantValues: map[string]any{
				"name":    "Test Timer",
				"restore": true,
			},
		},
		{
			name:      "with restore false",
			inputName: "Test Timer",
			args: map[string]any{
				"restore": false,
			},
			wantKeys: []string{"name", "restore"},
			wantValues: map[string]any{
				"name":    "Test Timer",
				"restore": false,
			},
		},
		{
			name:      "with icon",
			inputName: "Test Timer",
			args: map[string]any{
				"icon": "mdi:timer",
			},
			wantKeys: []string{"name", "icon"},
			wantValues: map[string]any{
				"name": "Test Timer",
				"icon": "mdi:timer",
			},
		},
		{
			name:      "all options",
			inputName: "Test Timer",
			args: map[string]any{
				"duration": "00:10:00",
				"restore":  true,
				"icon":     "mdi:timer-outline",
			},
			wantKeys: []string{"name", "duration", "restore", "icon"},
			wantValues: map[string]any{
				"name":     "Test Timer",
				"duration": "00:10:00",
				"restore":  true,
				"icon":     "mdi:timer-outline",
			},
		},
		{
			name:      "empty duration ignored",
			inputName: "Test Timer",
			args: map[string]any{
				"duration": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Timer",
			},
		},
		{
			name:      "empty icon ignored",
			inputName: "Test Timer",
			args: map[string]any{
				"icon": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Test Timer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildTimerHelperConfig(tt.inputName, tt.args)

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
