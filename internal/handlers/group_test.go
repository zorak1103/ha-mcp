package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockGroupClient implements homeassistant.Client for group tests.
type mockGroupClient struct {
	homeassistant.Client
	CreateHelperFn func(ctx context.Context, helper homeassistant.HelperConfig) error
	DeleteHelperFn func(ctx context.Context, entityID string) error
	CallServiceFn  func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
}

func (m *mockGroupClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.CreateHelperFn != nil {
		return m.CreateHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockGroupClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.DeleteHelperFn != nil {
		return m.DeleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockGroupClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.CallServiceFn != nil {
		return m.CallServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func TestNewGroupHandlers(t *testing.T) {
	t.Parallel()

	h := NewGroupHandlers()
	if h == nil {
		t.Error("NewGroupHandlers() returned nil, want non-nil")
	}
}

func TestGroupHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewGroupHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 4 {
		t.Errorf("RegisterTools() registered %d tools, want 4", len(tools))
	}

	expectedTools := map[string]bool{
		"create_group":       false,
		"delete_group":       false,
		"set_group_entities": false,
		"reload_group":       false,
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

func TestGroupHandlers_handleCreateGroup(t *testing.T) {
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
				"id":       "living_room_lights",
				"name":     "Living Room Lights",
				"entities": []any{"light.lamp_1", "light.lamp_2"},
			},
			wantContains: "created successfully",
			wantError:    false,
		},
		{
			name: "success with all option",
			args: map[string]any{
				"id":       "all_lights",
				"name":     "All Lights",
				"entities": []any{"light.lamp_1", "light.lamp_2"},
				"all":      true,
			},
			wantContains: "created successfully",
			wantError:    false,
		},
		{
			name: "missing id",
			args: map[string]any{
				"name":     "Living Room Lights",
				"entities": []any{"light.lamp_1"},
			},
			wantContains: "id is required",
			wantError:    true,
		},
		{
			name: "missing name",
			args: map[string]any{
				"id":       "living_room_lights",
				"entities": []any{"light.lamp_1"},
			},
			wantContains: "name is required",
			wantError:    true,
		},
		{
			name: "missing entities",
			args: map[string]any{
				"id":   "living_room_lights",
				"name": "Living Room Lights",
			},
			wantContains: "entities is required",
			wantError:    true,
		},
		{
			name: "empty entities",
			args: map[string]any{
				"id":       "living_room_lights",
				"name":     "Living Room Lights",
				"entities": []any{},
			},
			wantContains: "entities is required",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"id":       "living_room_lights",
				"name":     "Living Room Lights",
				"entities": []any{"light.lamp_1"},
			},
			createHelperErr: errors.New("connection failed"),
			wantContains:    "Error creating group",
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockGroupClient{
				CreateHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewGroupHandlers()
			result, err := h.handleCreateGroup(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateGroup() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleCreateGroup() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleCreateGroup() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleCreateGroup() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestGroupHandlers_handleDeleteGroup(t *testing.T) {
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
				"entity_id": "group.living_room_lights",
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
			wantContains: "must be a group entity",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "group.living_room_lights",
			},
			deleteHelperErr: errors.New("not found"),
			wantContains:    "Error deleting group",
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockGroupClient{
				DeleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewGroupHandlers()
			result, err := h.handleDeleteGroup(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteGroup() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleDeleteGroup() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleDeleteGroup() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleDeleteGroup() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestGroupHandlers_handleSetGroupEntities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		callServiceErr error
		wantContains   string
		wantError      bool
	}{
		{
			name: "success add entities",
			args: map[string]any{
				"entity_id":    "group.living_room_lights",
				"add_entities": []any{"light.lamp_3"},
			},
			wantContains: "updated successfully",
			wantError:    false,
		},
		{
			name: "success remove entities",
			args: map[string]any{
				"entity_id":       "group.living_room_lights",
				"remove_entities": []any{"light.lamp_1"},
			},
			wantContains: "updated successfully",
			wantError:    false,
		},
		{
			name: "success both add and remove",
			args: map[string]any{
				"entity_id":       "group.living_room_lights",
				"add_entities":    []any{"light.lamp_3"},
				"remove_entities": []any{"light.lamp_1"},
			},
			wantContains: "updated successfully",
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
				"entity_id":    "light.living_room",
				"add_entities": []any{"light.lamp_3"},
			},
			wantContains: "must be a group entity",
			wantError:    true,
		},
		{
			name: "no add or remove",
			args: map[string]any{
				"entity_id": "group.living_room_lights",
			},
			wantContains: "at least one of add_entities or remove_entities is required",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id":    "group.living_room_lights",
				"add_entities": []any{"light.lamp_3"},
			},
			callServiceErr: errors.New("service failed"),
			wantContains:   "Error modifying group entities",
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockGroupClient{
				CallServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewGroupHandlers()
			result, err := h.handleSetGroupEntities(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleSetGroupEntities() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleSetGroupEntities() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleSetGroupEntities() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleSetGroupEntities() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestGroupHandlers_handleReloadGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		callServiceErr error
		wantContains   string
		wantError      bool
	}{
		{
			name:         "success",
			wantContains: "reloaded successfully",
			wantError:    false,
		},
		{
			name:           "client error",
			callServiceErr: errors.New("service failed"),
			wantContains:   "Error reloading groups",
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockGroupClient{
				CallServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewGroupHandlers()
			result, err := h.handleReloadGroup(context.Background(), client, nil)

			if err != nil {
				t.Errorf("handleReloadGroup() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleReloadGroup() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleReloadGroup() returned empty content")
			}

			text := result.Content[0].Text
			if !contains(text, tt.wantContains) {
				t.Errorf("handleReloadGroup() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestBuildGroupHelperConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputName  string
		args       map[string]any
		wantKeys   []string
		wantValues map[string]any
	}{
		{
			name:      "name and entities only",
			inputName: "Living Room Lights",
			args: map[string]any{
				"entities": []any{"light.lamp_1", "light.lamp_2"},
			},
			wantKeys: []string{"name", "entities"},
		},
		{
			name:      "with all true",
			inputName: "All Lights",
			args: map[string]any{
				"entities": []any{"light.lamp_1"},
				"all":      true,
			},
			wantKeys: []string{"name", "entities", "all"},
			wantValues: map[string]any{
				"all": true,
			},
		},
		{
			name:      "with all false",
			inputName: "Any Lights",
			args: map[string]any{
				"entities": []any{"light.lamp_1"},
				"all":      false,
			},
			wantKeys: []string{"name", "entities", "all"},
			wantValues: map[string]any{
				"all": false,
			},
		},
		{
			name:      "with icon",
			inputName: "Lights Group",
			args: map[string]any{
				"entities": []any{"light.lamp_1"},
				"icon":     "mdi:lightbulb-group",
			},
			wantKeys: []string{"name", "entities", "icon"},
			wantValues: map[string]any{
				"icon": "mdi:lightbulb-group",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildGroupHelperConfig(tt.inputName, tt.args)

			for _, key := range tt.wantKeys {
				if _, ok := config[key]; !ok {
					t.Errorf("buildGroupHelperConfig() missing key %q", key)
				}
			}

			for key, wantVal := range tt.wantValues {
				if gotVal, ok := config[key]; !ok {
					t.Errorf("buildGroupHelperConfig() missing key %q", key)
				} else if gotVal != wantVal {
					t.Errorf("buildGroupHelperConfig()[%q] = %v, want %v", key, gotVal, wantVal)
				}
			}
		})
	}
}
