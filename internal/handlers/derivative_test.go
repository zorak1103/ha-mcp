package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockDerivativeClient implements homeassistant.Client for derivative tests.
type mockDerivativeClient struct {
	homeassistant.Client
	CreateHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	DeleteHelperFn func(ctx context.Context, entityID string) error
}

func (m *mockDerivativeClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.CreateHelperFn != nil {
		return m.CreateHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockDerivativeClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.DeleteHelperFn != nil {
		return m.DeleteHelperFn(ctx, entityID)
	}
	return nil
}

func TestNewDerivativeHandlers(t *testing.T) {
	t.Parallel()

	h := NewDerivativeHandlers()
	if h == nil {
		t.Error("NewDerivativeHandlers() returned nil, want non-nil")
	}
}

func TestDerivativeHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewDerivativeHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 2 {
		t.Errorf("RegisterTools() registered %d tools, want 2", len(tools))
	}

	expectedTools := map[string]bool{
		"create_derivative": false,
		"delete_derivative": false,
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

func TestDerivativeHandlers_handleCreateDerivative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		createHelperErr error
		wantContains    string
		wantError       bool
	}{
		{
			name: "success",
			args: map[string]any{
				"id":     "power_rate",
				"name":   "Power Rate",
				"source": "sensor.power",
			},
			wantContains: "created successfully",
			wantError:    false,
		},
		{
			name: "success with options",
			args: map[string]any{
				"id":          "power_rate",
				"name":        "Power Rate",
				"source":      "sensor.power",
				"round":       float64(2),
				"time_window": "00:05:00",
				"unit_time":   "h",
			},
			wantContains: "created successfully",
			wantError:    false,
		},
		{
			name: "missing id",
			args: map[string]any{
				"name":   "Power Rate",
				"source": "sensor.power",
			},
			wantContains: "id is required",
			wantError:    true,
		},
		{
			name: "missing name",
			args: map[string]any{
				"id":     "power_rate",
				"source": "sensor.power",
			},
			wantContains: "name is required",
			wantError:    true,
		},
		{
			name: "missing source",
			args: map[string]any{
				"id":   "power_rate",
				"name": "Power Rate",
			},
			wantContains: "source",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"id":     "power_rate",
				"name":   "Power Rate",
				"source": "sensor.power",
			},
			createHelperErr: errors.New("connection failed"),
			wantContains:    "Error creating derivative",
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockDerivativeClient{
				CreateHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewDerivativeHandlers()
			result, err := h.handleCreateDerivative(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateDerivative() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleCreateDerivative() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleCreateDerivative() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleCreateDerivative() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestDerivativeHandlers_handleDeleteDerivative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		deleteHelperErr error
		wantContains    string
		wantError       bool
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": "sensor.power_rate",
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
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "light.living_room",
			},
			wantContains: "must be a sensor entity",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "sensor.power_rate",
			},
			deleteHelperErr: errors.New("not found"),
			wantContains:    "Error deleting derivative",
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockDerivativeClient{
				DeleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewDerivativeHandlers()
			result, err := h.handleDeleteDerivative(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteDerivative() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleDeleteDerivative() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleDeleteDerivative() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleDeleteDerivative() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestBuildDerivativeConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputName  string
		source     string
		args       map[string]any
		wantKeys   []string
		wantValues map[string]any
	}{
		{
			name:      "name and source only",
			inputName: "Power Rate",
			source:    "sensor.power",
			args:      map[string]any{},
			wantKeys:  []string{"name", "source"},
		},
		{
			name:      "with round",
			inputName: "Power Rate",
			source:    "sensor.power",
			args: map[string]any{
				"round": float64(3),
			},
			wantKeys: []string{"name", "source", "round"},
			wantValues: map[string]any{
				"round": 3,
			},
		},
		{
			name:      "with time_window",
			inputName: "Power Rate",
			source:    "sensor.power",
			args: map[string]any{
				"time_window": "00:05:00",
			},
			wantKeys: []string{"name", "source", "time_window"},
			wantValues: map[string]any{
				"time_window": "00:05:00",
			},
		},
		{
			name:      "with unit_time",
			inputName: "Power Rate",
			source:    "sensor.power",
			args: map[string]any{
				"unit_time": "h",
			},
			wantKeys: []string{"name", "source", "unit_time"},
			wantValues: map[string]any{
				"unit_time": "h",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildDerivativeConfig(tt.inputName, tt.source, tt.args)

			for _, key := range tt.wantKeys {
				if _, ok := config[key]; !ok {
					t.Errorf("buildDerivativeConfig() missing key %q", key)
				}
			}

			for key, wantVal := range tt.wantValues {
				if gotVal, ok := config[key]; !ok {
					t.Errorf("buildDerivativeConfig() missing key %q", key)
				} else if gotVal != wantVal {
					t.Errorf("buildDerivativeConfig()[%q] = %v, want %v", key, gotVal, wantVal)
				}
			}
		})
	}
}
