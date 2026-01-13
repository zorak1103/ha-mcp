package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Test constants to avoid goconst warnings.
const (
	testTemplateTypeSensor       = "sensor"
	testTemplateTypeBinarySensor = "binary_sensor"
)

// mockTemplateHelperClient implements homeassistant.Client for template helper tests.
type mockTemplateHelperClient struct {
	homeassistant.Client
	CreateHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	DeleteHelperFn func(ctx context.Context, entityID string) error
}

func (m *mockTemplateHelperClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.CreateHelperFn != nil {
		return m.CreateHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockTemplateHelperClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.DeleteHelperFn != nil {
		return m.DeleteHelperFn(ctx, entityID)
	}
	return nil
}

func TestNewTemplateHelperHandlers(t *testing.T) {
	t.Parallel()

	h := NewTemplateHelperHandlers()

	if h == nil {
		t.Error("NewTemplateHelperHandlers() returned nil")
	}
}

func TestTemplateHelperHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewTemplateHelperHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()

	expectedTools := []string{"create_template_sensor", "create_template_binary_sensor", "delete_template_helper"}
	for _, expected := range expectedTools {
		found := false
		for _, tool := range tools {
			if tool.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RegisterTools() did not register tool %q", expected)
		}
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), len(expectedTools))
	}
}

func TestTemplateHelperHandlers_handleCreateTemplateSensor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		clientErr    error
		wantContains string
		wantError    bool
	}{
		{
			name: "success with required fields",
			args: map[string]any{
				"id":    "avg_temp",
				"name":  "Average Temperature",
				"state": "{{ (states('sensor.temp1') | float + states('sensor.temp2') | float) / 2 }}",
			},
			wantContains: "created successfully as sensor.avg_temp",
			wantError:    false,
		},
		{
			name: "success with all options",
			args: map[string]any{
				"id":                  "power_calc",
				"name":                "Calculated Power",
				"state":               "{{ states('sensor.voltage') | float * states('sensor.current') | float }}",
				"unit_of_measurement": "W",
				"device_class":        "power",
				"state_class":         "measurement",
				"icon":                "mdi:flash",
			},
			wantContains: "created successfully as sensor.power_calc",
			wantError:    false,
		},
		{
			name: "missing id",
			args: map[string]any{
				"name":  "Test Sensor",
				"state": "{{ 42 }}",
			},
			wantContains: "id is required",
			wantError:    true,
		},
		{
			name: "missing name",
			args: map[string]any{
				"id":    "test_sensor",
				"state": "{{ 42 }}",
			},
			wantContains: "name is required",
			wantError:    true,
		},
		{
			name: "missing state",
			args: map[string]any{
				"id":   "test_sensor",
				"name": "Test Sensor",
			},
			wantContains: "state (Jinja2 template) is required",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"id":    "test_sensor",
				"name":  "Test Sensor",
				"state": "{{ 42 }}",
			},
			clientErr:    errors.New("failed to create helper"),
			wantContains: "Error creating template sensor",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTemplateHelperClient{
				CreateHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.clientErr
				},
			}

			h := NewTemplateHelperHandlers()
			result, err := h.handleCreateTemplateSensor(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("handleCreateTemplateSensor() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("handleCreateTemplateSensor() returned nil result")
			}
			if result.IsError != tt.wantError {
				t.Errorf("handleCreateTemplateSensor() IsError = %v, want %v", result.IsError, tt.wantError)
			}
			if len(result.Content) == 0 {
				t.Fatal("handleCreateTemplateSensor() returned empty content")
			}
			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleCreateTemplateSensor() content = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestTemplateHelperHandlers_handleCreateTemplateBinarySensor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		clientErr    error
		wantContains string
		wantError    bool
	}{
		{
			name: "success with required fields",
			args: map[string]any{
				"id":    "is_home",
				"name":  "Someone Home",
				"state": "{{ states('device_tracker.phone') == 'home' }}",
			},
			wantContains: "created successfully as binary_sensor.is_home",
			wantError:    false,
		},
		{
			name: "success with all options",
			args: map[string]any{
				"id":           "motion_active",
				"name":         "Motion Active",
				"state":        "{{ states('binary_sensor.motion') == 'on' }}",
				"device_class": "motion",
				"delay_on":     "00:00:05",
				"delay_off":    "00:00:30",
				"icon":         "mdi:motion-sensor",
			},
			wantContains: "created successfully as binary_sensor.motion_active",
			wantError:    false,
		},
		{
			name: "missing id",
			args: map[string]any{
				"name":  "Test Binary",
				"state": "{{ true }}",
			},
			wantContains: "id is required",
			wantError:    true,
		},
		{
			name: "missing name",
			args: map[string]any{
				"id":    "test_binary",
				"state": "{{ true }}",
			},
			wantContains: "name is required",
			wantError:    true,
		},
		{
			name: "missing state",
			args: map[string]any{
				"id":   "test_binary",
				"name": "Test Binary",
			},
			wantContains: "state (Jinja2 template) is required",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"id":    "test_binary",
				"name":  "Test Binary",
				"state": "{{ true }}",
			},
			clientErr:    errors.New("failed to create helper"),
			wantContains: "Error creating template binary sensor",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTemplateHelperClient{
				CreateHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.clientErr
				},
			}

			h := NewTemplateHelperHandlers()
			result, err := h.handleCreateTemplateBinarySensor(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("handleCreateTemplateBinarySensor() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("handleCreateTemplateBinarySensor() returned nil result")
			}
			if result.IsError != tt.wantError {
				t.Errorf("handleCreateTemplateBinarySensor() IsError = %v, want %v", result.IsError, tt.wantError)
			}
			if len(result.Content) == 0 {
				t.Fatal("handleCreateTemplateBinarySensor() returned empty content")
			}
			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleCreateTemplateBinarySensor() content = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestTemplateHelperHandlers_handleDeleteTemplateHelper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		clientErr    error
		wantContains string
		wantError    bool
	}{
		{
			name: "success with sensor",
			args: map[string]any{
				"entity_id": "sensor.my_template",
			},
			wantContains: "deleted successfully",
			wantError:    false,
		},
		{
			name: "success with binary_sensor",
			args: map[string]any{
				"entity_id": "binary_sensor.my_template",
			},
			wantContains: "deleted successfully",
			wantError:    false,
		},
		{
			name:         "missing entity_id",
			args:         map[string]any{},
			wantContains: "entity_id is required",
			wantError:    true,
		},
		{
			name: "invalid platform - not sensor or binary_sensor",
			args: map[string]any{
				"entity_id": "input_boolean.test",
			},
			wantContains: "must be a template sensor or binary_sensor",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "sensor.my_template",
			},
			clientErr:    errors.New("delete failed"),
			wantContains: "Error deleting template helper",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockTemplateHelperClient{
				DeleteHelperFn: func(_ context.Context, _ string) error {
					return tt.clientErr
				},
			}

			h := NewTemplateHelperHandlers()
			result, err := h.handleDeleteTemplateHelper(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("handleDeleteTemplateHelper() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("handleDeleteTemplateHelper() returned nil result")
			}
			if result.IsError != tt.wantError {
				t.Errorf("handleDeleteTemplateHelper() IsError = %v, want %v", result.IsError, tt.wantError)
			}
			if len(result.Content) == 0 {
				t.Fatal("handleDeleteTemplateHelper() returned empty content")
			}
			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleDeleteTemplateHelper() content = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestBuildTemplateSensorConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		nameArg string
		state   string
		args    map[string]any
		checkFn func(t *testing.T, config map[string]any)
	}{
		{
			name:    "minimal config",
			nameArg: "Test Sensor",
			state:   "{{ 42 }}",
			args:    map[string]any{},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["name"] != "Test Sensor" {
					t.Error("name not set correctly")
				}
				if config["state"] != "{{ 42 }}" {
					t.Error("state not set correctly")
				}
				if config["template_type"] != testTemplateTypeSensor {
					t.Error("template_type should be sensor")
				}
			},
		},
		{
			name:    "with unit_of_measurement",
			nameArg: "Power",
			state:   "{{ 100 }}",
			args: map[string]any{
				"unit_of_measurement": "W",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["unit_of_measurement"] != "W" {
					t.Errorf("unit_of_measurement = %v, want W", config["unit_of_measurement"])
				}
			},
		},
		{
			name:    "with device_class and state_class",
			nameArg: "Energy",
			state:   "{{ 50 }}",
			args: map[string]any{
				"device_class": "energy",
				"state_class":  "total_increasing",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["device_class"] != "energy" {
					t.Errorf("device_class = %v, want energy", config["device_class"])
				}
				if config["state_class"] != "total_increasing" {
					t.Errorf("state_class = %v, want total_increasing", config["state_class"])
				}
			},
		},
		{
			name:    "with icon",
			nameArg: "Test",
			state:   "{{ 0 }}",
			args: map[string]any{
				"icon": "mdi:thermometer",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["icon"] != "mdi:thermometer" {
					t.Errorf("icon = %v, want mdi:thermometer", config["icon"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildTemplateSensorConfig(tt.nameArg, tt.state, tt.args)

			tt.checkFn(t, config)
		})
	}
}

func TestBuildTemplateBinarySensorConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		nameArg string
		state   string
		args    map[string]any
		checkFn func(t *testing.T, config map[string]any)
	}{
		{
			name:    "minimal config",
			nameArg: "Test Binary",
			state:   "{{ true }}",
			args:    map[string]any{},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["name"] != "Test Binary" {
					t.Error("name not set correctly")
				}
				if config["state"] != "{{ true }}" {
					t.Error("state not set correctly")
				}
				if config["template_type"] != testTemplateTypeBinarySensor {
					t.Error("template_type should be binary_sensor")
				}
			},
		},
		{
			name:    "with device_class",
			nameArg: "Motion",
			state:   "{{ true }}",
			args: map[string]any{
				"device_class": "motion",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["device_class"] != "motion" {
					t.Errorf("device_class = %v, want motion", config["device_class"])
				}
			},
		},
		{
			name:    "with delay_on and delay_off",
			nameArg: "Delayed",
			state:   "{{ true }}",
			args: map[string]any{
				"delay_on":  "00:00:05",
				"delay_off": "00:00:30",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["delay_on"] != "00:00:05" {
					t.Errorf("delay_on = %v, want 00:00:05", config["delay_on"])
				}
				if config["delay_off"] != "00:00:30" {
					t.Errorf("delay_off = %v, want 00:00:30", config["delay_off"])
				}
			},
		},
		{
			name:    "with icon",
			nameArg: "Test",
			state:   "{{ false }}",
			args: map[string]any{
				"icon": "mdi:motion-sensor",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["icon"] != "mdi:motion-sensor" {
					t.Errorf("icon = %v, want mdi:motion-sensor", config["icon"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildTemplateBinarySensorConfig(tt.nameArg, tt.state, tt.args)

			tt.checkFn(t, config)
		})
	}
}
