package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockIntegralClient implements homeassistant.Client for integral handler tests.
type mockIntegralClient struct {
	homeassistant.Client
	CreateHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	DeleteHelperFn func(ctx context.Context, entityID string) error
	CallServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockIntegralClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.CreateHelperFn != nil {
		return m.CreateHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockIntegralClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.DeleteHelperFn != nil {
		return m.DeleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockIntegralClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.CallServiceFn != nil {
		return m.CallServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewIntegralHandlers(t *testing.T) {
	t.Parallel()

	h := NewIntegralHandlers()

	if h == nil {
		t.Error("NewIntegralHandlers() returned nil")
	}
}

func TestIntegralHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewIntegralHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()

	expectedTools := []string{"create_integral", "delete_integral", "reset_integral"}
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

func TestIntegralHandlers_handleCreateIntegral(t *testing.T) {
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
				"id":     "energy_usage",
				"name":   "Energy Usage",
				"source": "sensor.power",
			},
			wantContains: "created successfully as sensor.energy_usage",
			wantError:    false,
		},
		{
			name: "success with all options",
			args: map[string]any{
				"id":          "total_energy",
				"name":        "Total Energy",
				"source":      "sensor.power_meter",
				"method":      "trapezoidal",
				"round":       float64(3),
				"unit_time":   "h",
				"unit_prefix": "k",
				"icon":        "mdi:lightning-bolt",
			},
			wantContains: "created successfully as sensor.total_energy",
			wantError:    false,
		},
		{
			name: "missing id",
			args: map[string]any{
				"name":   "Test Integral",
				"source": "sensor.power",
			},
			wantContains: "id is required",
			wantError:    true,
		},
		{
			name: "missing name",
			args: map[string]any{
				"id":     "test_integral",
				"source": "sensor.power",
			},
			wantContains: "name is required",
			wantError:    true,
		},
		{
			name: "missing source",
			args: map[string]any{
				"id":   "test_integral",
				"name": "Test Integral",
			},
			wantContains: "source (source sensor entity ID) is required",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"id":     "test_integral",
				"name":   "Test Integral",
				"source": "sensor.power",
			},
			clientErr:    errors.New("failed to create helper"),
			wantContains: "Error creating integral",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockIntegralClient{
				CreateHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.clientErr
				},
			}

			h := NewIntegralHandlers()
			result, err := h.handleCreateIntegral(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("handleCreateIntegral() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("handleCreateIntegral() returned nil result")
			}
			if result.IsError != tt.wantError {
				t.Errorf("handleCreateIntegral() IsError = %v, want %v", result.IsError, tt.wantError)
			}
			if len(result.Content) == 0 {
				t.Fatal("handleCreateIntegral() returned empty content")
			}
			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleCreateIntegral() content = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestIntegralHandlers_handleDeleteIntegral(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		clientErr    error
		wantContains string
		wantError    bool
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": "sensor.my_integral",
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
			name: "invalid platform - not sensor",
			args: map[string]any{
				"entity_id": "input_boolean.test",
			},
			wantContains: "must be a sensor entity",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "sensor.my_integral",
			},
			clientErr:    errors.New("delete failed"),
			wantContains: "Error deleting integral",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockIntegralClient{
				DeleteHelperFn: func(_ context.Context, _ string) error {
					return tt.clientErr
				},
			}

			h := NewIntegralHandlers()
			result, err := h.handleDeleteIntegral(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("handleDeleteIntegral() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("handleDeleteIntegral() returned nil result")
			}
			if result.IsError != tt.wantError {
				t.Errorf("handleDeleteIntegral() IsError = %v, want %v", result.IsError, tt.wantError)
			}
			if len(result.Content) == 0 {
				t.Fatal("handleDeleteIntegral() returned empty content")
			}
			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleDeleteIntegral() content = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestIntegralHandlers_handleResetIntegral(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		clientErr    error
		wantContains string
		wantError    bool
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": "sensor.my_integral",
			},
			wantContains: "reset to zero successfully",
			wantError:    false,
		},
		{
			name:         "missing entity_id",
			args:         map[string]any{},
			wantContains: "entity_id is required",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "sensor.my_integral",
			},
			clientErr:    errors.New("service call failed"),
			wantContains: "Error resetting integral",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockIntegralClient{
				CallServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.clientErr
				},
			}

			h := NewIntegralHandlers()
			result, err := h.handleResetIntegral(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("handleResetIntegral() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("handleResetIntegral() returned nil result")
			}
			if result.IsError != tt.wantError {
				t.Errorf("handleResetIntegral() IsError = %v, want %v", result.IsError, tt.wantError)
			}
			if len(result.Content) == 0 {
				t.Fatal("handleResetIntegral() returned empty content")
			}
			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleResetIntegral() content = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestBuildIntegralConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		nameArg string
		source  string
		args    map[string]any
		checkFn func(t *testing.T, config map[string]any)
	}{
		{
			name:    "minimal config",
			nameArg: "Test Integral",
			source:  "sensor.power",
			args:    map[string]any{},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["name"] != "Test Integral" {
					t.Error("name not set correctly")
				}
				if config["source"] != "sensor.power" {
					t.Error("source not set correctly")
				}
			},
		},
		{
			name:    "with method",
			nameArg: "Test",
			source:  "sensor.power",
			args: map[string]any{
				"method": "left",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["method"] != "left" {
					t.Errorf("method = %v, want left", config["method"])
				}
			},
		},
		{
			name:    "with round and unit options",
			nameArg: "Energy",
			source:  "sensor.power",
			args: map[string]any{
				"round":       float64(3),
				"unit_time":   "h",
				"unit_prefix": "k",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["round"] != 3 {
					t.Errorf("round = %v, want 3", config["round"])
				}
				if config["unit_time"] != "h" {
					t.Errorf("unit_time = %v, want h", config["unit_time"])
				}
				if config["unit_prefix"] != "k" {
					t.Errorf("unit_prefix = %v, want k", config["unit_prefix"])
				}
			},
		},
		{
			name:    "unit_prefix none is excluded",
			nameArg: "Test",
			source:  "sensor.power",
			args: map[string]any{
				"unit_prefix": "none",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if _, ok := config["unit_prefix"]; ok {
					t.Error("unit_prefix 'none' should not be included in config")
				}
			},
		},
		{
			name:    "with icon",
			nameArg: "Test",
			source:  "sensor.power",
			args: map[string]any{
				"icon": "mdi:sigma",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["icon"] != "mdi:sigma" {
					t.Errorf("icon = %v, want mdi:sigma", config["icon"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildIntegralConfig(tt.nameArg, tt.source, tt.args)

			tt.checkFn(t, config)
		})
	}
}
