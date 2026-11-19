// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// =============================================================================
// Tool Schema Tests
// =============================================================================

func TestFloorHandlers_ManageFloorToolSchema(t *testing.T) {
	t.Parallel()

	h := NewFloorHandlers()
	tool := h.manageFloorTool()

	if tool.Name != "manage_floor" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_floor")
	}

	if tool.Description == "" {
		t.Error("tool.Description should not be empty")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Check required parameters
	requiredMap := make(map[string]bool)
	for _, req := range tool.InputSchema.Required {
		requiredMap[req] = true
	}

	if !requiredMap["action"] {
		t.Error("action should be in required parameters")
	}

	// Check action enum has 5 values
	actionSchema := tool.InputSchema.Properties["action"]
	if len(actionSchema.Enum) != 5 {
		t.Errorf("action enum should have 5 values, got %d", len(actionSchema.Enum))
	}
}

// =============================================================================
// List Tests
// =============================================================================

func TestManageFloor_List(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "list floors with natural format",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					level0 := 0
					level1 := 1
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_ground", Name: "Ground Floor", Level: &level0, Icon: "mdi:home-floor-0"},
						{FloorID: "floor_first", Name: "First Floor", Level: &level1},
					}, nil
				}
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room", FloorID: "floor_ground"},
						{AreaID: "bedroom", Name: "Bedroom", FloorID: "floor_first"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 floor(s)", "Ground Floor", "First Floor", "Level: 0", "Level: 1", "Areas: 1"},
		},
		{
			name: "list floors with json format",
			args: map[string]any{
				"action": "list",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_1", Name: "Test Floor"},
					}, nil
				}
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"floor_1", "Test Floor", "floor_id"},
		},
		{
			name: "list floors empty",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{}, nil
				}
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No floors found"},
		},
		{
			name: "list floors with name filter",
			args: map[string]any{
				"action":        "list",
				"name_contains": "first",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_ground", Name: "Ground Floor"},
						{FloorID: "floor_first", Name: "First Floor"},
						{FloorID: "floor_second", Name: "Second Floor"},
					}, nil
				}
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 1 floor(s)", "First Floor"},
		},
		{
			name: "list floors API error",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "listing"},
		},
	}

	h := NewFloorHandlers()
	runHandlerTestCases(t, tests, h.handleManageFloor)
}

// =============================================================================
// Get Tests
// =============================================================================

func TestManageFloor_Get(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "get floor with natural format",
			args: map[string]any{
				"action":   "get",
				"floor_id": "floor_ground",
			},
			setupMock: func(m *UniversalMockClient) {
				level0 := 0
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{
							FloorID: "floor_ground",
							Name:    "Ground Floor",
							Level:   &level0,
							Icon:    "mdi:home-floor-0",
							Aliases: []string{"Main Floor", "Entry Level"},
						},
					}, nil
				}
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room", FloorID: "floor_ground"},
						{AreaID: "kitchen", Name: "Kitchen", FloorID: "floor_ground"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Floor: Ground Floor", "ID: floor_ground", "Level: 0", "Icon: mdi:home-floor-0", "Aliases: Main Floor", "Areas on this floor: 2", "Living Room", "Kitchen"},
		},
		{
			name: "get floor with json format",
			args: map[string]any{
				"action":   "get",
				"floor_id": "floor_1",
				"format":   "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_1", Name: "Test Floor"},
					}, nil
				}
				m.GetAreaRegistryFn = func(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"floor_1", "Test Floor", "floor_id"},
		},
		{
			name: "get floor not found",
			args: map[string]any{
				"action":   "get",
				"floor_id": "nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"floor not found", "nonexistent"},
		},
		{
			name: "get floor without floor_id",
			args: map[string]any{
				"action": "get",
			},
			wantError:    true,
			wantContains: []string{"floor_id", "required"},
		},
	}

	h := NewFloorHandlers()
	runHandlerTestCases(t, tests, h.handleManageFloor)
}

// =============================================================================
// Create Tests
// =============================================================================

func TestManageFloor_Create(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create floor successfully",
			args: map[string]any{
				"action": "create",
				"name":   "New Floor",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateFloorFn = func(_ context.Context, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
					return &homeassistant.FloorRegistryEntry{
						FloorID: "floor_new",
						Name:    config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully", "New Floor", "floor_new"},
		},
		{
			name: "create floor with all fields",
			args: map[string]any{
				"action":  "create",
				"name":    "First Floor",
				"level":   float64(1),
				"icon":    "mdi:home-floor-1",
				"aliases": []any{"Upper Level", "Upstairs"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateFloorFn = func(_ context.Context, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
					if config.Name != "First Floor" || config.Level == nil || *config.Level != 1 {
						return nil, fmt.Errorf("unexpected config")
					}
					return &homeassistant.FloorRegistryEntry{
						FloorID: "floor_first",
						Name:    config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully"},
		},
		{
			name: "create floor without name",
			args: map[string]any{
				"action": "create",
			},
			wantError:    true,
			wantContains: []string{"name", "required"},
		},
		{
			name: "create floor API error",
			args: map[string]any{
				"action": "create",
				"name":   "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateFloorFn = func(context.Context, homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
					return nil, fmt.Errorf("database error")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "creating"},
		},
	}

	h := NewFloorHandlers()
	runHandlerTestCases(t, tests, h.handleManageFloor)
}

// =============================================================================
// Update Tests
// =============================================================================

func TestManageFloor_Update(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "update floor name",
			args: map[string]any{
				"action":   "update",
				"floor_id": "floor_1",
				"name":     "Updated Name",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_1", Name: "Test Floor"},
						{FloorID: "floor_2", Name: "Test Floor 2"},
					}, nil
				}
				m.UpdateFloorFn = func(_ context.Context, floorID string, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
					if floorID != "floor_1" {
						return nil, fmt.Errorf("unexpected floor_id: %s", floorID)
					}
					return &homeassistant.FloorRegistryEntry{
						FloorID: floorID,
						Name:    config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully", "Updated Name"},
		},
		{
			name: "update floor level and icon",
			args: map[string]any{
				"action":   "update",
				"floor_id": "floor_2",
				"level":    float64(-1),
				"icon":     "mdi:home-floor-b",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_1", Name: "Test Floor"},
						{FloorID: "floor_2", Name: "Basement"},
					}, nil
				}
				m.UpdateFloorFn = func(_ context.Context, floorID string, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
					return &homeassistant.FloorRegistryEntry{
						FloorID: floorID,
						Name:    "Basement",
						Level:   config.Level,
						Icon:    config.Icon,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully"},
		},
		{
			name: "update floor without floor_id",
			args: map[string]any{
				"action": "update",
				"name":   "New Name",
			},
			wantError:    true,
			wantContains: []string{"floor_id", "required"},
		},
		{
			name: "update floor API error",
			args: map[string]any{
				"action":   "update",
				"floor_id": "floor_1",
				"name":     "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_1", Name: "Test Floor"},
					}, nil
				}
				m.UpdateFloorFn = func(context.Context, string, homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
					return nil, fmt.Errorf("not found")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "updating"},
		},
	}

	h := NewFloorHandlers()
	runHandlerTestCases(t, tests, h.handleManageFloor)
}

// =============================================================================
// Delete Tests
// =============================================================================

func TestManageFloor_Delete(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "delete floor successfully",
			args: map[string]any{
				"action":   "delete",
				"floor_id": "floor_old",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_old", Name: "Old Floor"},
						{FloorID: "floor_new", Name: "New Floor"},
					}, nil
				}
				m.DeleteFloorFn = func(_ context.Context, floorID string) error {
					if floorID != "floor_old" {
						return fmt.Errorf("unexpected floor_id: %s", floorID)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"deleted successfully", "floor_old"},
		},
		{
			name: "delete floor without floor_id",
			args: map[string]any{
				"action": "delete",
			},
			wantError:    true,
			wantContains: []string{"floor_id", "required"},
		},
		{
			name: "delete floor API error",
			args: map[string]any{
				"action":   "delete",
				"floor_id": "floor_1",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_1", Name: "Test Floor"},
					}, nil
				}
				m.DeleteFloorFn = func(context.Context, string) error {
					return fmt.Errorf("floor has areas")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "deleting"},
		},
	}

	h := NewFloorHandlers()
	runHandlerTestCases(t, tests, h.handleManageFloor)
}

// =============================================================================
// Alias Mode Tests (update)
// =============================================================================

func TestManageFloor_UpdateAliasMode(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "update - aliases with add mode merges with existing",
			args: map[string]any{
				"action":     "update",
				"floor_id":   "floor_1",
				"aliases":    []any{"new alias"},
				"alias_mode": arrayModeAdd,
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_1", Name: "Test Floor", Aliases: []string{"existing alias"}},
					}, nil
				}
				m.UpdateFloorFn = func(_ context.Context, floorID string, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
					if len(config.Aliases) != 2 {
						t.Errorf("expected 2 aliases after add, got %v", config.Aliases)
					}
					return &homeassistant.FloorRegistryEntry{FloorID: floorID, Name: "Test Floor", Aliases: config.Aliases}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully"},
		},
		{
			name: "update - aliases with remove mode removes specified",
			args: map[string]any{
				"action":     "update",
				"floor_id":   "floor_1",
				"aliases":    []any{"remove me"},
				"alias_mode": arrayModeRemove,
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
					return []homeassistant.FloorRegistryEntry{
						{FloorID: "floor_1", Name: "Test Floor", Aliases: []string{"keep me", "remove me"}},
					}, nil
				}
				m.UpdateFloorFn = func(_ context.Context, floorID string, config homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
					if len(config.Aliases) != 1 || config.Aliases[0] != "keep me" {
						t.Errorf("expected [keep me] after remove, got %v", config.Aliases)
					}
					return &homeassistant.FloorRegistryEntry{FloorID: floorID, Name: "Test Floor", Aliases: config.Aliases}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully"},
		},
	}

	h := NewFloorHandlers()
	runHandlerTestCases(t, tests, h.handleManageFloor)
}

// =============================================================================
// Validation Tests
// =============================================================================

func TestManageFloor_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name:         "missing action",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"action", "required"},
		},
		{
			name: "invalid action",
			args: map[string]any{
				"action": "invalid",
			},
			wantError:    true,
			wantContains: []string{"invalid action"},
		},
	}

	h := NewFloorHandlers()
	runHandlerTestCases(t, tests, h.handleManageFloor)
}

// =============================================================================
// Registration Tests
// =============================================================================

func TestFloorHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewFloorHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	// Verify manage_floor is registered
	tools := registry.ListTools()
	found := false

	for _, tool := range tools {
		if tool.Name == "manage_floor" {
			found = true
			break
		}
	}

	if !found {
		t.Error("manage_floor tool not registered")
	}
}

func TestFloorList_RejectsNonStringFilters(t *testing.T) {
	t.Parallel()

	mustNotList := func(t *testing.T, c *UniversalMockClient) {
		t.Helper()
		c.GetFloorRegistryFn = func(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
			t.Errorf("HA must not be called for an invalid filter")
			return nil, nil
		}
	}

	runFilterTypeCases(t, NewFloorHandlers().handleManageFloor, []filterTypeCase{
		{
			name:      "numeric name_contains",
			args:      map[string]any{"action": "list", "name_contains": float64(7)},
			want:      "name_contains must be a string, got float64",
			setupMock: mustNotList,
		},
		{
			name: "empty array name_contains means no filter",
			args: map[string]any{"action": "list", "name_contains": []any{}},
		},
	})
}
