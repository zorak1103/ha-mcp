// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestAreaHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	handler := NewAreaHandlers()

	handler.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "manage_area" {
		t.Errorf("expected tool 'manage_area', got '%s'", tools[0].Name)
	}
}

func TestManageArea_Schema(t *testing.T) {
	t.Parallel()

	handler := NewAreaHandlers()
	tool := handler.manageAreaTool()

	if tool.Name != "manage_area" {
		t.Errorf("expected tool name 'manage_area', got '%s'", tool.Name)
	}

	// Verify action enum has 5 values: list, get, create, update, delete
	actionProp, ok := tool.InputSchema.Properties["action"]
	if !ok {
		t.Fatal("expected 'action' property in schema")
	}
	if len(actionProp.Enum) != 5 {
		t.Errorf("expected 5 action enum values, got %d", len(actionProp.Enum))
	}

	expectedActions := []string{"list", "get", "create", "update", "delete"}
	for _, expected := range expectedActions {
		found := false
		for _, enum := range actionProp.Enum {
			if enum == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected action '%s' not found in enum", expected)
		}
	}

	// Verify format enum
	formatProp, ok := tool.InputSchema.Properties["format"]
	if !ok {
		t.Fatal("expected 'format' property in schema")
	}
	if len(formatProp.Enum) != 2 {
		t.Errorf("expected 2 format enum values, got %d", len(formatProp.Enum))
	}

	// Verify include_entities and include_automations properties exist
	if _, ok := tool.InputSchema.Properties["include_entities"]; !ok {
		t.Error("expected 'include_entities' property in schema")
	}
	if _, ok := tool.InputSchema.Properties["include_automations"]; !ok {
		t.Error("expected 'include_automations' property in schema")
	}
}

func TestHandleManageArea(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		args        map[string]any
		setupMock   func(*UniversalMockClient)
		wantErr     bool
		wantContain string
	}

	tests := []testCase{
		// =========================
		// List Action Tests
		// =========================
		{
			name: "list - empty",
			args: map[string]any{
				"action": "list",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{}, nil
				}
			},
			wantErr:     false,
			wantContain: `"areas":[]`,
		},
		{
			name: "list - multiple areas",
			args: map[string]any{
				"action": "list",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room", FloorID: "ground_floor"},
						{AreaID: "kitchen", Name: "Kitchen", Icon: "mdi:silverware"},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"area_id":"living_room"`,
		},
		{
			name: "list - with name filter",
			args: map[string]any{
				"action":        "list",
				"name_contains": "kitchen",
				"format":        "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
						{AreaID: "kitchen", Name: "Kitchen"},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"area_id":"kitchen"`,
		},
		{
			name: "list - natural format",
			args: map[string]any{
				"action": "list",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Living Room",
		},

		// =========================
		// Get Action Tests
		// =========================
		{
			name: "get - found with enrichment",
			args: map[string]any{
				"action":  "get",
				"area_id": "living_room",
				"format":  "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room", FloorID: "ground_floor"},
					}, nil
				}
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "device1", AreaID: "living_room"},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", AreaID: "living_room"},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"area_id":"living_room"`,
		},
		{
			name: "get - not found",
			args: map[string]any{
				"action":  "get",
				"area_id": "nonexistent",
				"format":  "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{}, nil
				}
			},
			wantErr:     false,
			wantContain: "not found",
		},
		{
			name: "get - missing area_id",
			args: map[string]any{
				"action": "get",
				"format": "json",
			},
			wantErr:     false,
			wantContain: "required",
		},
		{
			name: "get - natural format",
			args: map[string]any{
				"action":  "get",
				"area_id": "living_room",
				"format":  "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
			},
			wantErr:     false,
			wantContain: "Living Room",
		},

		// =========================
		// Create Action Tests
		// =========================
		{
			name: "create - success",
			args: map[string]any{
				"action": "create",
				"name":   "Bedroom",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateAreaFn = func(_ context.Context, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
					return &homeassistant.AreaRegistryEntry{
						AreaID: "bedroom",
						Name:   config.Name,
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"area_id":"bedroom"`,
		},
		{
			name: "create - with all fields",
			args: map[string]any{
				"action":   "create",
				"name":     "Bedroom",
				"icon":     "mdi:bed",
				"picture":  "/local/bedroom.jpg",
				"floor_id": "first_floor",
				"aliases":  []any{"Master Bedroom", "Main Bedroom"},
				"labels":   []any{"primary", "sleeping"},
				"format":   "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateAreaFn = func(_ context.Context, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
					return &homeassistant.AreaRegistryEntry{
						AreaID:  "bedroom",
						Name:    config.Name,
						Icon:    config.Icon,
						Picture: config.Picture,
						FloorID: config.FloorID,
						Aliases: config.Aliases,
						Labels:  config.Labels,
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"icon":"mdi:bed"`,
		},
		{
			name: "create - missing name",
			args: map[string]any{
				"action": "create",
				"format": "json",
			},
			wantErr:     false,
			wantContain: "required",
		},
		{
			name: "create - natural format",
			args: map[string]any{
				"action": "create",
				"name":   "Bedroom",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateAreaFn = func(_ context.Context, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
					return &homeassistant.AreaRegistryEntry{
						AreaID: "bedroom",
						Name:   config.Name,
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Bedroom",
		},

		// =========================
		// Update Action Tests
		// =========================
		{
			name: "update - success",
			args: map[string]any{
				"action":  "update",
				"area_id": "living_room",
				"name":    "Updated Living Room",
				"format":  "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
						{AreaID: "kitchen", Name: "Kitchen"},
					}, nil
				}
				m.UpdateAreaFn = func(_ context.Context, areaID string, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
					return &homeassistant.AreaRegistryEntry{
						AreaID: areaID,
						Name:   config.Name,
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"area_id":"living_room"`,
		},
		{
			name: "update - partial update",
			args: map[string]any{
				"action":  "update",
				"area_id": "kitchen",
				"icon":    "mdi:pot",
				"format":  "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
						{AreaID: "kitchen", Name: "Kitchen"},
					}, nil
				}
				m.UpdateAreaFn = func(_ context.Context, areaID string, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
					return &homeassistant.AreaRegistryEntry{
						AreaID: areaID,
						Icon:   config.Icon,
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"icon":"mdi:pot"`,
		},
		{
			name: "update - missing area_id",
			args: map[string]any{
				"action": "update",
				"name":   "Updated Name",
				"format": "json",
			},
			wantErr:     false,
			wantContain: "required",
		},
		{
			name: "update - natural format",
			args: map[string]any{
				"action":  "update",
				"area_id": "living_room",
				"name":    "Updated Living Room",
				"format":  "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
				m.UpdateAreaFn = func(_ context.Context, areaID string, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
					return &homeassistant.AreaRegistryEntry{
						AreaID: areaID,
						Name:   config.Name,
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Updated Living Room",
		},

		// =========================
		// Delete Action Tests
		// =========================
		{
			name: "delete - success",
			args: map[string]any{
				"action":  "delete",
				"area_id": "old_room",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "old_room", Name: "Old Room"},
						{AreaID: "new_room", Name: "New Room"},
					}, nil
				}
				m.DeleteAreaFn = func(context.Context, string) error {
					return nil
				}
			},
			wantErr:     false,
			wantContain: "deleted",
		},
		{
			name: "delete - missing area_id",
			args: map[string]any{
				"action": "delete",
			},
			wantErr:     false,
			wantContain: "required",
		},
		{
			name: "delete - error from client",
			args: map[string]any{
				"action":  "delete",
				"area_id": "protected_area",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "protected_area", Name: "Protected Area"},
					}, nil
				}
				m.DeleteAreaFn = func(context.Context, string) error {
					return fmt.Errorf("cannot delete protected area")
				}
			},
			wantErr:     false,
			wantContain: "error",
		},

		// =========================
		// Label/Alias Mode Tests (update)
		// =========================
		{
			name: "update - labels with add mode merges with existing",
			args: map[string]any{
				"action":     "update",
				"area_id":    "living_room",
				"labels":     []any{"new_label"},
				"label_mode": arrayModeAdd,
				"format":     "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room", Labels: []string{"existing_label"}},
					}, nil
				}
				m.UpdateAreaFn = func(_ context.Context, areaID string, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
					if len(config.Labels) != 2 {
						t.Errorf("expected 2 labels after add, got %v", config.Labels)
					}
					return &homeassistant.AreaRegistryEntry{AreaID: areaID, Name: "Living Room", Labels: config.Labels}, nil
				}
			},
			wantErr:     false,
			wantContain: `"area_id":"living_room"`,
		},
		{
			name: "update - aliases with remove mode removes specified",
			args: map[string]any{
				"action":     "update",
				"area_id":    "living_room",
				"aliases":    []any{"old name"},
				"alias_mode": arrayModeRemove,
				"format":     "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room", Aliases: []string{"old name", "keep this"}},
					}, nil
				}
				m.UpdateAreaFn = func(_ context.Context, areaID string, config homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
					if len(config.Aliases) != 1 || config.Aliases[0] != "keep this" {
						t.Errorf("expected [keep this] after remove, got %v", config.Aliases)
					}
					return &homeassistant.AreaRegistryEntry{AreaID: areaID, Name: "Living Room", Aliases: config.Aliases}, nil
				}
			},
			wantErr:     false,
			wantContain: `"area_id":"living_room"`,
		},

		// =========================
		// Invalid Action Test
		// =========================
		{
			name: "invalid action",
			args: map[string]any{
				"action": "invalid",
			},
			wantErr:     false,
			wantContain: "invalid",
		},

		// =========================
		// include_entities Tests
		// =========================
		{
			name: "get - include_entities natural",
			args: map[string]any{
				"action":           "get",
				"area_id":          "living_room",
				"include_entities": true,
				"format":           "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", AreaID: "living_room"},
					}, nil
				}
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{}, nil
				}
				m.GetStatesFn = func(context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{
							EntityID:   "light.living_room",
							State:      "on",
							Attributes: map[string]any{"friendly_name": "Living Room Light"},
						},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Entities in Area",
		},
		{
			name: "get - include_entities json",
			args: map[string]any{
				"action":           "get",
				"area_id":          "living_room",
				"include_entities": true,
				"format":           "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", AreaID: "living_room"},
					}, nil
				}
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{}, nil
				}
				m.GetStatesFn = func(context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{
							EntityID:   "light.living_room",
							State:      "on",
							Attributes: map[string]any{"friendly_name": "Living Room Light"},
						},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"entities"`,
		},
		{
			name: "get - include_entities empty area",
			args: map[string]any{
				"action":           "get",
				"area_id":          "living_room",
				"include_entities": true,
				"format":           "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{}, nil
				}
				m.GetStatesFn = func(context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{EntityID: "light.other_room", State: "off"},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Living Room",
		},

		// =========================
		// include_automations Tests
		// =========================
		{
			name: "get - include_automations with match",
			args: map[string]any{
				"action":              "get",
				"area_id":             "living_room",
				"include_automations": true,
				"format":              "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", AreaID: "living_room"},
					}, nil
				}
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{}, nil
				}
				m.ListAutomationsFn = func(context.Context) ([]homeassistant.Automation, error) {
					return []homeassistant.Automation{
						{EntityID: "automation.turn_on_lights", FriendlyName: "Turn On Lights", State: "on"},
					}, nil
				}
				m.GetAutomationFn = func(_ context.Context, automationID string) (*homeassistant.Automation, error) {
					return &homeassistant.Automation{
						EntityID: automationID,
						Config: &homeassistant.AutomationConfig{
							Actions: []any{
								map[string]any{"entity_id": "light.living_room"},
							},
						},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Automations Referencing Area Entities",
		},
		{
			name: "get - include_automations no match",
			args: map[string]any{
				"action":              "get",
				"area_id":             "living_room",
				"include_automations": true,
				"format":              "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", AreaID: "living_room"},
					}, nil
				}
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{}, nil
				}
				m.ListAutomationsFn = func(context.Context) ([]homeassistant.Automation, error) {
					return []homeassistant.Automation{
						{EntityID: "automation.kitchen_lights", FriendlyName: "Kitchen Lights", State: "on"},
					}, nil
				}
				m.GetAutomationFn = func(_ context.Context, automationID string) (*homeassistant.Automation, error) {
					return &homeassistant.Automation{
						EntityID: automationID,
						Config: &homeassistant.AutomationConfig{
							Actions: []any{
								map[string]any{"entity_id": "light.kitchen"},
							},
						},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Living Room",
		},
		{
			name: "get - both include flags",
			args: map[string]any{
				"action":              "get",
				"area_id":             "living_room",
				"include_entities":    true,
				"include_automations": true,
				"format":              "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", AreaID: "living_room"},
					}, nil
				}
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{}, nil
				}
				m.GetStatesFn = func(context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{
							EntityID:   "light.living_room",
							State:      "on",
							Attributes: map[string]any{"friendly_name": "Living Room Light"},
						},
					}, nil
				}
				m.ListAutomationsFn = func(context.Context) ([]homeassistant.Automation, error) {
					return []homeassistant.Automation{
						{EntityID: "automation.turn_on_lights", FriendlyName: "Turn On Lights", State: "on"},
					}, nil
				}
				m.GetAutomationFn = func(_ context.Context, automationID string) (*homeassistant.Automation, error) {
					return &homeassistant.Automation{
						EntityID: automationID,
						Config: &homeassistant.AutomationConfig{
							Actions: []any{
								map[string]any{"entity_id": "light.living_room"},
							},
						},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Entities in Area",
		},
		{
			name: "get - entity via device in area",
			args: map[string]any{
				"action":           "get",
				"area_id":          "living_room",
				"include_entities": true,
				"format":           "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					// Entity assigned to device (not directly to area)
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "sensor.temp", DeviceID: "device1"},
					}, nil
				}
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "device1", AreaID: "living_room"},
					}, nil
				}
				m.GetStatesFn = func(context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{
							EntityID:   "sensor.temp",
							State:      "22.5",
							Attributes: map[string]any{"friendly_name": "Living Room Temp"},
						},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"entities"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockClient := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			handler := NewAreaHandlers()
			result, err := handler.handleManageArea(context.Background(), mockClient, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleManageArea() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error case
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			// Extract text from result
			var resultText string
			if len(result.Content) > 0 {
				resultText = result.Content[0].Text
			}

			if tt.wantContain != "" && !strings.Contains(resultText, tt.wantContain) {
				t.Errorf("result does not contain %q, got: %s", tt.wantContain, resultText)
			}
		})
	}
}

// TestManageArea_AssignedAutomations tests that include_automations shows both
// automations assigned to the area (via entity registry area_id) and automations
// referencing entities in the area, as separate clearly-labeled sections.
func TestManageArea_AssignedAutomations(t *testing.T) {
	t.Parallel()

	handler := NewAreaHandlers()

	mockClient := &UniversalMockClient{
		GetAreaRegistryFn: func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
			return []homeassistant.AreaRegistryEntry{
				{AreaID: "living_room", Name: "Living Room"},
			}, nil
		},
		// automation.assigned is directly assigned to living_room via entity registry
		// automation.referencing is assigned to kitchen but references a living_room entity
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "light.living_room", AreaID: "living_room"},
				{EntityID: "automation.assigned", AreaID: "living_room"},
				{EntityID: "automation.referencing", AreaID: "kitchen"},
			}, nil
		},
		GetDeviceRegistryFn: func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
			return []homeassistant.DeviceRegistryEntry{}, nil
		},
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			return []homeassistant.Automation{
				{EntityID: "automation.assigned", FriendlyName: "Assigned Auto", State: "on"},
				{EntityID: "automation.referencing", FriendlyName: "Referencing Auto", State: "on"},
			}, nil
		},
		GetAutomationFn: func(_ context.Context, automationID string) (*homeassistant.Automation, error) {
			switch automationID {
			case "automation.assigned":
				// Does not reference any living_room entity
				return &homeassistant.Automation{
					EntityID: automationID,
					Config:   &homeassistant.AutomationConfig{Actions: []any{}},
				}, nil
			case "automation.referencing":
				// References light.living_room (an entity in living_room)
				return &homeassistant.Automation{
					EntityID: automationID,
					Config: &homeassistant.AutomationConfig{
						Actions: []any{map[string]any{"entity_id": "light.living_room"}},
					},
				}, nil
			}
			return nil, fmt.Errorf("automation not found: %s", automationID)
		},
	}

	result, err := handler.handleManageArea(context.Background(), mockClient, map[string]any{
		"action":              "get",
		"area_id":             "living_room",
		"include_automations": true,
		"format":              "natural",
	})
	if err != nil {
		t.Fatalf("handleManageArea() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	text := ""
	if len(result.Content) > 0 {
		text = result.Content[0].Text
	}

	// Should show assigned automations section
	if !strings.Contains(text, "Automations Assigned to Area") {
		t.Errorf("expected 'Automations Assigned to Area' section, got:\n%s", text)
	}
	if !strings.Contains(text, "automation.assigned") {
		t.Errorf("expected automation.assigned in output, got:\n%s", text)
	}

	// Should show relabelled referencing section
	if !strings.Contains(text, "Automations Referencing Area Entities") {
		t.Errorf("expected 'Automations Referencing Area Entities' section, got:\n%s", text)
	}
	if !strings.Contains(text, "automation.referencing") {
		t.Errorf("expected automation.referencing in referencing section, got:\n%s", text)
	}
}
