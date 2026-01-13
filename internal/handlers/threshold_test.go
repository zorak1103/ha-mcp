package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockThresholdClient implements homeassistant.Client for threshold handler tests.
type mockThresholdClient struct {
	homeassistant.Client
	CreateHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	DeleteHelperFn func(ctx context.Context, entityID string) error
}

func (m *mockThresholdClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.CreateHelperFn != nil {
		return m.CreateHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockThresholdClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.DeleteHelperFn != nil {
		return m.DeleteHelperFn(ctx, entityID)
	}
	return nil
}

func TestNewThresholdHandlers(t *testing.T) {
	t.Parallel()

	h := NewThresholdHandlers()

	if h == nil {
		t.Error("NewThresholdHandlers() returned nil")
	}
}

func TestThresholdHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewThresholdHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()

	expectedTools := []string{"create_threshold", "delete_threshold"}
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

func TestThresholdHandlers_handleCreateThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		clientErr    error
		wantContains string
		wantError    bool
	}{
		{
			name: "success with lower threshold",
			args: map[string]any{
				"id":        "cold_alert",
				"name":      "Cold Alert",
				"entity_id": "sensor.temperature",
				"lower":     float64(10.0),
			},
			wantContains: "created successfully as binary_sensor.cold_alert",
			wantError:    false,
		},
		{
			name: "success with upper threshold",
			args: map[string]any{
				"id":        "heat_alert",
				"name":      "Heat Alert",
				"entity_id": "sensor.temperature",
				"upper":     float64(30.0),
			},
			wantContains: "created successfully as binary_sensor.heat_alert",
			wantError:    false,
		},
		{
			name: "success with both thresholds and options",
			args: map[string]any{
				"id":           "temp_range_alert",
				"name":         "Temperature Range Alert",
				"entity_id":    "sensor.temperature",
				"lower":        float64(15.0),
				"upper":        float64(25.0),
				"hysteresis":   float64(1.0),
				"device_class": "problem",
				"icon":         "mdi:thermometer-alert",
			},
			wantContains: "created successfully as binary_sensor.temp_range_alert",
			wantError:    false,
		},
		{
			name: "missing id",
			args: map[string]any{
				"name":      "Test Threshold",
				"entity_id": "sensor.temperature",
				"lower":     float64(10.0),
			},
			wantContains: "id is required",
			wantError:    true,
		},
		{
			name: "missing name",
			args: map[string]any{
				"id":        "test_threshold",
				"entity_id": "sensor.temperature",
				"lower":     float64(10.0),
			},
			wantContains: "name is required",
			wantError:    true,
		},
		{
			name: "missing entity_id",
			args: map[string]any{
				"id":    "test_threshold",
				"name":  "Test Threshold",
				"lower": float64(10.0),
			},
			wantContains: "entity_id (source sensor) is required",
			wantError:    true,
		},
		{
			name: "missing both lower and upper",
			args: map[string]any{
				"id":        "test_threshold",
				"name":      "Test Threshold",
				"entity_id": "sensor.temperature",
			},
			wantContains: "at least one of 'lower' or 'upper' threshold must be specified",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"id":        "test_threshold",
				"name":      "Test Threshold",
				"entity_id": "sensor.temperature",
				"lower":     float64(10.0),
			},
			clientErr:    errors.New("failed to create helper"),
			wantContains: "Error creating threshold",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockThresholdClient{
				CreateHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.clientErr
				},
			}

			h := NewThresholdHandlers()
			result, err := h.handleCreateThreshold(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("handleCreateThreshold() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("handleCreateThreshold() returned nil result")
			}
			if result.IsError != tt.wantError {
				t.Errorf("handleCreateThreshold() IsError = %v, want %v", result.IsError, tt.wantError)
			}
			if len(result.Content) == 0 {
				t.Fatal("handleCreateThreshold() returned empty content")
			}
			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleCreateThreshold() content = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestThresholdHandlers_handleDeleteThreshold(t *testing.T) {
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
				"entity_id": "binary_sensor.my_threshold",
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
			name: "invalid platform - not binary_sensor",
			args: map[string]any{
				"entity_id": "sensor.temperature",
			},
			wantContains: "must be a binary_sensor entity",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "binary_sensor.my_threshold",
			},
			clientErr:    errors.New("delete failed"),
			wantContains: "Error deleting threshold",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockThresholdClient{
				DeleteHelperFn: func(_ context.Context, _ string) error {
					return tt.clientErr
				},
			}

			h := NewThresholdHandlers()
			result, err := h.handleDeleteThreshold(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("handleDeleteThreshold() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("handleDeleteThreshold() returned nil result")
			}
			if result.IsError != tt.wantError {
				t.Errorf("handleDeleteThreshold() IsError = %v, want %v", result.IsError, tt.wantError)
			}
			if len(result.Content) == 0 {
				t.Fatal("handleDeleteThreshold() returned empty content")
			}
			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleDeleteThreshold() content = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestBuildThresholdConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		nameArg        string
		sourceEntityID string
		args           map[string]any
		checkFn        func(t *testing.T, config map[string]any)
	}{
		{
			name:           "with lower threshold only",
			nameArg:        "Cold Alert",
			sourceEntityID: "sensor.temperature",
			args: map[string]any{
				"lower": float64(10.0),
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["name"] != "Cold Alert" {
					t.Error("name not set correctly")
				}
				if config["entity_id"] != "sensor.temperature" {
					t.Error("entity_id not set correctly")
				}
				if config["lower"] != 10.0 {
					t.Errorf("lower = %v, want 10.0", config["lower"])
				}
				if _, ok := config["upper"]; ok {
					t.Error("upper should not be set")
				}
			},
		},
		{
			name:           "with upper threshold only",
			nameArg:        "Heat Alert",
			sourceEntityID: "sensor.temperature",
			args: map[string]any{
				"upper": float64(30.0),
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["upper"] != 30.0 {
					t.Errorf("upper = %v, want 30.0", config["upper"])
				}
				if _, ok := config["lower"]; ok {
					t.Error("lower should not be set")
				}
			},
		},
		{
			name:           "with both thresholds",
			nameArg:        "Range Alert",
			sourceEntityID: "sensor.temperature",
			args: map[string]any{
				"lower": float64(15.0),
				"upper": float64(25.0),
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["lower"] != 15.0 {
					t.Errorf("lower = %v, want 15.0", config["lower"])
				}
				if config["upper"] != 25.0 {
					t.Errorf("upper = %v, want 25.0", config["upper"])
				}
			},
		},
		{
			name:           "with hysteresis",
			nameArg:        "Test",
			sourceEntityID: "sensor.temperature",
			args: map[string]any{
				"lower":      float64(10.0),
				"hysteresis": float64(2.0),
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["hysteresis"] != 2.0 {
					t.Errorf("hysteresis = %v, want 2.0", config["hysteresis"])
				}
			},
		},
		{
			name:           "with device_class and icon",
			nameArg:        "Test",
			sourceEntityID: "sensor.temperature",
			args: map[string]any{
				"lower":        float64(10.0),
				"device_class": "cold",
				"icon":         "mdi:snowflake",
			},
			checkFn: func(t *testing.T, config map[string]any) {
				t.Helper()
				if config["device_class"] != "cold" {
					t.Errorf("device_class = %v, want cold", config["device_class"])
				}
				if config["icon"] != "mdi:snowflake" {
					t.Errorf("icon = %v, want mdi:snowflake", config["icon"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildThresholdConfig(tt.nameArg, tt.sourceEntityID, tt.args)

			tt.checkFn(t, config)
		})
	}
}
