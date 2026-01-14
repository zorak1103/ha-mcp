package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockTargetsClient implements homeassistant.Client for targets tests.
type mockTargetsClient struct {
	homeassistant.Client
	GetTriggersForTargetFn   func(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error)
	GetConditionsForTargetFn func(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error)
	GetServicesForTargetFn   func(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error)
	ExtractFromTargetFn      func(ctx context.Context, target homeassistant.Target, expandGroup *bool) (*homeassistant.ExtractFromTargetResult, error)
}

func (m *mockTargetsClient) GetTriggersForTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	if m.GetTriggersForTargetFn != nil {
		return m.GetTriggersForTargetFn(ctx, target, expandGroup)
	}
	return []string{}, nil
}

func (m *mockTargetsClient) GetConditionsForTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	if m.GetConditionsForTargetFn != nil {
		return m.GetConditionsForTargetFn(ctx, target, expandGroup)
	}
	return []string{}, nil
}

func (m *mockTargetsClient) GetServicesForTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) ([]string, error) {
	if m.GetServicesForTargetFn != nil {
		return m.GetServicesForTargetFn(ctx, target, expandGroup)
	}
	return []string{}, nil
}

func (m *mockTargetsClient) ExtractFromTarget(ctx context.Context, target homeassistant.Target, expandGroup *bool) (*homeassistant.ExtractFromTargetResult, error) {
	if m.ExtractFromTargetFn != nil {
		return m.ExtractFromTargetFn(ctx, target, expandGroup)
	}
	return &homeassistant.ExtractFromTargetResult{}, nil
}

func TestNewTargetHandlers(t *testing.T) {
	t.Parallel()

	h := NewTargetHandlers()

	if h == nil {
		t.Error("NewTargetHandlers() returned nil, want non-nil")
	}
}

func TestTargetHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewTargetHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 4 {
		t.Errorf("RegisterTools() registered %d tools, want 4", len(tools))
	}

	expectedTools := map[string]bool{
		"get_triggers_for_target":   false,
		"get_conditions_for_target": false,
		"get_services_for_target":   false,
		"extract_from_target":       false,
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

func TestTargetHandlers_targetInputSchema(t *testing.T) {
	t.Parallel()

	h := NewTargetHandlers()
	schema := h.targetInputSchema()

	tests := []struct {
		name      string
		checkFunc func(t *testing.T, schema mcp.JSONSchema)
	}{
		{
			name: "has object type",
			checkFunc: func(t *testing.T, schema mcp.JSONSchema) {
				t.Helper()
				if schema.Type != testSchemaTypeObject {
					t.Errorf("schema.Type = %q, want %q", schema.Type, testSchemaTypeObject)
				}
			},
		},
		{
			name: "has entity_id property",
			checkFunc: func(t *testing.T, schema mcp.JSONSchema) {
				t.Helper()
				if _, ok := schema.Properties["entity_id"]; !ok {
					t.Error("entity_id property missing")
				}
			},
		},
		{
			name: "has device_id property",
			checkFunc: func(t *testing.T, schema mcp.JSONSchema) {
				t.Helper()
				if _, ok := schema.Properties["device_id"]; !ok {
					t.Error("device_id property missing")
				}
			},
		},
		{
			name: "has area_id property",
			checkFunc: func(t *testing.T, schema mcp.JSONSchema) {
				t.Helper()
				if _, ok := schema.Properties["area_id"]; !ok {
					t.Error("area_id property missing")
				}
			},
		},
		{
			name: "has label_id property",
			checkFunc: func(t *testing.T, schema mcp.JSONSchema) {
				t.Helper()
				if _, ok := schema.Properties["label_id"]; !ok {
					t.Error("label_id property missing")
				}
			},
		},
		{
			name: "has expand_group property",
			checkFunc: func(t *testing.T, schema mcp.JSONSchema) {
				t.Helper()
				if _, ok := schema.Properties["expand_group"]; !ok {
					t.Error("expand_group property missing")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.checkFunc(t, schema)
		})
	}
}

func TestTargetHandlers_extractStringArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params map[string]any
		key    string
		want   []string
	}{
		{
			name:   "key exists with valid strings",
			params: map[string]any{"ids": []any{"a", "b", "c"}},
			key:    "ids",
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "key does not exist",
			params: map[string]any{"other": []any{"a"}},
			key:    "ids",
			want:   nil,
		},
		{
			name:   "empty array",
			params: map[string]any{"ids": []any{}},
			key:    "ids",
			want:   []string{},
		},
		{
			name:   "mixed types in array - only strings extracted",
			params: map[string]any{"ids": []any{"a", 123, "b", true}},
			key:    "ids",
			want:   []string{"a", "b"},
		},
		{
			name:   "value is not an array",
			params: map[string]any{"ids": "not_an_array"},
			key:    "ids",
			want:   nil,
		},
		{
			name:   "nil params",
			params: nil,
			key:    "ids",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewTargetHandlers()
			got := h.extractStringArray(tt.params, tt.key)

			if tt.want == nil {
				if got != nil {
					t.Errorf("extractStringArray() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("extractStringArray() length = %d, want %d", len(got), len(tt.want))
				return
			}

			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("extractStringArray()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestTargetHandlers_parseTargetParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		params        map[string]any
		wantEntityIDs int
		wantDeviceIDs int
		wantAreaIDs   int
		wantLabelIDs  int
		wantExpandNil bool
		wantExpandVal bool
		wantError     bool
	}{
		{
			name: "entity_id only",
			params: map[string]any{
				"entity_id": []any{"light.living_room", "switch.kitchen"},
			},
			wantEntityIDs: 2,
			wantExpandNil: true,
			wantError:     false,
		},
		{
			name: "device_id only",
			params: map[string]any{
				"device_id": []any{"device_123"},
			},
			wantDeviceIDs: 1,
			wantExpandNil: true,
			wantError:     false,
		},
		{
			name: "area_id only",
			params: map[string]any{
				"area_id": []any{"living_room", "kitchen"},
			},
			wantAreaIDs:   2,
			wantExpandNil: true,
			wantError:     false,
		},
		{
			name: "label_id only",
			params: map[string]any{
				"label_id": []any{"outdoor"},
			},
			wantLabelIDs:  1,
			wantExpandNil: true,
			wantError:     false,
		},
		{
			name: "all target types",
			params: map[string]any{
				"entity_id": []any{"light.living_room"},
				"device_id": []any{"device_123"},
				"area_id":   []any{"living_room"},
				"label_id":  []any{"outdoor"},
			},
			wantEntityIDs: 1,
			wantDeviceIDs: 1,
			wantAreaIDs:   1,
			wantLabelIDs:  1,
			wantExpandNil: true,
			wantError:     false,
		},
		{
			name: "with expand_group true",
			params: map[string]any{
				"entity_id":    []any{"light.living_room"},
				"expand_group": true,
			},
			wantEntityIDs: 1,
			wantExpandNil: false,
			wantExpandVal: true,
			wantError:     false,
		},
		{
			name: "with expand_group false",
			params: map[string]any{
				"entity_id":    []any{"light.living_room"},
				"expand_group": false,
			},
			wantEntityIDs: 1,
			wantExpandNil: false,
			wantExpandVal: false,
			wantError:     false,
		},
		{
			name:      "no target specified",
			params:    map[string]any{},
			wantError: true,
		},
		{
			name: "empty arrays",
			params: map[string]any{
				"entity_id": []any{},
				"device_id": []any{},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewTargetHandlers()
			target, expandGroup, err := h.parseTargetParams(tt.params)

			if tt.wantError {
				if err == nil {
					t.Error("parseTargetParams() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("parseTargetParams() unexpected error: %v", err)
				return
			}

			if len(target.EntityID) != tt.wantEntityIDs {
				t.Errorf("target.EntityID count = %d, want %d", len(target.EntityID), tt.wantEntityIDs)
			}
			if len(target.DeviceID) != tt.wantDeviceIDs {
				t.Errorf("target.DeviceID count = %d, want %d", len(target.DeviceID), tt.wantDeviceIDs)
			}
			if len(target.AreaID) != tt.wantAreaIDs {
				t.Errorf("target.AreaID count = %d, want %d", len(target.AreaID), tt.wantAreaIDs)
			}
			if len(target.LabelID) != tt.wantLabelIDs {
				t.Errorf("target.LabelID count = %d, want %d", len(target.LabelID), tt.wantLabelIDs)
			}

			if tt.wantExpandNil {
				if expandGroup != nil {
					t.Errorf("expandGroup = %v, want nil", *expandGroup)
				}
			} else {
				if expandGroup == nil {
					t.Error("expandGroup = nil, want non-nil")
				} else if *expandGroup != tt.wantExpandVal {
					t.Errorf("expandGroup = %v, want %v", *expandGroup, tt.wantExpandVal)
				}
			}
		})
	}
}

func TestTargetHandlers_handleGetTriggersForTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockTargetsClient
		wantContains string
		wantError    bool
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": []any{"light.living_room"},
			},
			client: &mockTargetsClient{
				GetTriggersForTargetFn: func(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
					return []string{"state", "numeric_state"}, nil
				},
			},
			wantContains: "state",
			wantError:    false,
		},
		{
			name:         "missing target",
			args:         map[string]any{},
			client:       &mockTargetsClient{},
			wantContains: "Invalid parameters",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": []any{"light.living_room"},
			},
			client: &mockTargetsClient{
				GetTriggersForTargetFn: func(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
					return nil, errors.New("connection failed")
				},
			},
			wantContains: "Error getting triggers",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewTargetHandlers()
			result, err := h.handleGetTriggersForTarget(context.Background(), tt.client, tt.args)

			if err != nil {
				t.Errorf("handleGetTriggersForTarget() returned error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("handleGetTriggersForTarget() returned nil result")
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleGetTriggersForTarget() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetTriggersForTarget() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleGetTriggersForTarget() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestTargetHandlers_handleGetConditionsForTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockTargetsClient
		wantContains string
		wantError    bool
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": []any{"light.living_room"},
			},
			client: &mockTargetsClient{
				GetConditionsForTargetFn: func(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
					return []string{"state", "numeric_state"}, nil
				},
			},
			wantContains: "state",
			wantError:    false,
		},
		{
			name:         "missing target",
			args:         map[string]any{},
			client:       &mockTargetsClient{},
			wantContains: "Invalid parameters",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": []any{"light.living_room"},
			},
			client: &mockTargetsClient{
				GetConditionsForTargetFn: func(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
					return nil, errors.New("connection failed")
				},
			},
			wantContains: "Error getting conditions",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewTargetHandlers()
			result, err := h.handleGetConditionsForTarget(context.Background(), tt.client, tt.args)

			if err != nil {
				t.Errorf("handleGetConditionsForTarget() returned error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("handleGetConditionsForTarget() returned nil result")
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleGetConditionsForTarget() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetConditionsForTarget() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleGetConditionsForTarget() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestTargetHandlers_handleGetServicesForTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockTargetsClient
		wantContains string
		wantError    bool
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": []any{"light.living_room"},
			},
			client: &mockTargetsClient{
				GetServicesForTargetFn: func(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
					return []string{"light.turn_on", "light.turn_off"}, nil
				},
			},
			wantContains: "light.turn_on",
			wantError:    false,
		},
		{
			name:         "missing target",
			args:         map[string]any{},
			client:       &mockTargetsClient{},
			wantContains: "Invalid parameters",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": []any{"light.living_room"},
			},
			client: &mockTargetsClient{
				GetServicesForTargetFn: func(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
					return nil, errors.New("connection failed")
				},
			},
			wantContains: "Error getting services",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewTargetHandlers()
			result, err := h.handleGetServicesForTarget(context.Background(), tt.client, tt.args)

			if err != nil {
				t.Errorf("handleGetServicesForTarget() returned error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("handleGetServicesForTarget() returned nil result")
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleGetServicesForTarget() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetServicesForTarget() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleGetServicesForTarget() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestTargetHandlers_handleExtractFromTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockTargetsClient
		wantContains string
		wantError    bool
	}{
		{
			name: "success",
			args: map[string]any{
				"area_id": []any{"living_room"},
			},
			client: &mockTargetsClient{
				ExtractFromTargetFn: func(_ context.Context, _ homeassistant.Target, _ *bool) (*homeassistant.ExtractFromTargetResult, error) {
					return &homeassistant.ExtractFromTargetResult{
						ReferencedEntities: []string{"light.living_room_1", "light.living_room_2"},
						ReferencedDevices:  []string{"device_123"},
						ReferencedAreas:    []string{"living_room"},
					}, nil
				},
			},
			wantContains: "living_room",
			wantError:    false,
		},
		{
			name:         "missing target",
			args:         map[string]any{},
			client:       &mockTargetsClient{},
			wantContains: "Invalid parameters",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"area_id": []any{"living_room"},
			},
			client: &mockTargetsClient{
				ExtractFromTargetFn: func(_ context.Context, _ homeassistant.Target, _ *bool) (*homeassistant.ExtractFromTargetResult, error) {
					return nil, errors.New("connection failed")
				},
			},
			wantContains: "Error extracting from target",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewTargetHandlers()
			result, err := h.handleExtractFromTarget(context.Background(), tt.client, tt.args)

			if err != nil {
				t.Errorf("handleExtractFromTarget() returned error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("handleExtractFromTarget() returned nil result")
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleExtractFromTarget() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleExtractFromTarget() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleExtractFromTarget() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestRegisterTargetTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterTargetTools(registry)

	tools := registry.ListTools()
	if len(tools) != 4 {
		t.Errorf("RegisterTargetTools() registered %d tools, want 4", len(tools))
	}
}
