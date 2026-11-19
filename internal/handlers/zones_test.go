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

func TestZoneHandlers_ManageZoneToolSchema(t *testing.T) {
	t.Parallel()

	h := NewZoneHandlers()
	tool := h.manageZoneTool()

	if tool.Name != "manage_zone" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_zone")
	}

	if tool.Description == "" {
		t.Error("tool.Description should not be empty")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
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

func TestManageZone_List(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "list zones with natural format",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_home", Name: "Home", Latitude: 51.5074, Longitude: -0.1278, Radius: 100, Icon: "mdi:home"},
						{ID: "zone_work", Name: "Work", Latitude: 51.5145, Longitude: -0.0932, Radius: 50, Passive: true},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 zone(s)", "Home", "Work", "51.507400", "-0.127800", "100m", "Passive"},
		},
		{
			name: "list zones with json format",
			args: map[string]any{
				"action": "list",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_1", Name: "Test Zone", Latitude: 0, Longitude: 0, Radius: 100},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"zone_1", "Test Zone", "latitude"},
		},
		{
			name: "list zones empty",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No zones found"},
		},
		{
			name: "list zones with name filter",
			args: map[string]any{
				"action":        "list",
				"name_contains": "work",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_home", Name: "Home", Latitude: 0, Longitude: 0, Radius: 100},
						{ID: "zone_work", Name: "Work", Latitude: 1, Longitude: 1, Radius: 50},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 1 zone(s)", "Work"},
		},
		{
			name: "list zones API error",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "listing"},
		},
	}

	h := NewZoneHandlers()
	runHandlerTestCases(t, tests, h.handleManageZone)
}

// =============================================================================
// Get Tests
// =============================================================================

func TestManageZone_Get(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "get zone with natural format",
			args: map[string]any{
				"action":  "get",
				"zone_id": "zone_home",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{
							ID:        "zone_home",
							Name:      "Home",
							Latitude:  51.5074,
							Longitude: -0.1278,
							Radius:    100,
							Icon:      "mdi:home",
							Passive:   false,
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Zone: Home", "ID: zone_home", "Latitude: 51.507400", "Longitude: -0.127800", "Radius: 100 meters", "Icon: mdi:home"},
		},
		{
			name: "get zone with json format",
			args: map[string]any{
				"action":  "get",
				"zone_id": "zone_1",
				"format":  "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_1", Name: "Test", Latitude: 0, Longitude: 0, Radius: 50},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"zone_1", "Test", "latitude"},
		},
		{
			name: "get zone not found",
			args: map[string]any{
				"action":  "get",
				"zone_id": "nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_1", Name: "Test", Latitude: 0, Longitude: 0, Radius: 50},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"zone not found", "nonexistent"},
		},
		{
			name: "get zone without zone_id",
			args: map[string]any{
				"action": "get",
			},
			wantError:    true,
			wantContains: []string{"zone_id", "required"},
		},
	}

	h := NewZoneHandlers()
	runHandlerTestCases(t, tests, h.handleManageZone)
}

// =============================================================================
// Create Tests
// =============================================================================

func TestManageZone_Create(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create zone successfully",
			args: map[string]any{
				"action":    "create",
				"name":      "School",
				"latitude":  float64(51.5145),
				"longitude": float64(-0.0932),
				"radius":    float64(50),
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateZoneFn = func(_ context.Context, config homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
					if config.Name != "School" || config.Latitude == nil || *config.Latitude != 51.5145 {
						return nil, fmt.Errorf("unexpected config")
					}
					return &homeassistant.ZoneRegistryEntry{
						ID:   "zone_school",
						Name: config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully", "School", "zone_school"},
		},
		{
			name: "create zone with all fields",
			args: map[string]any{
				"action":    "create",
				"name":      "Office",
				"latitude":  float64(51.5),
				"longitude": float64(-0.1),
				"radius":    float64(30),
				"icon":      "mdi:office-building",
				"passive":   true,
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateZoneFn = func(_ context.Context, config homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
					return &homeassistant.ZoneRegistryEntry{
						ID:   "zone_office",
						Name: config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully"},
		},
		{
			name: "create zone without name",
			args: map[string]any{
				"action":    "create",
				"latitude":  float64(51.5),
				"longitude": float64(-0.1),
				"radius":    float64(50),
			},
			wantError:    true,
			wantContains: []string{"name", "required"},
		},
		{
			name: "create zone without latitude",
			args: map[string]any{
				"action":    "create",
				"name":      "Test",
				"longitude": float64(-0.1),
				"radius":    float64(50),
			},
			wantError:    true,
			wantContains: []string{"latitude", "required"},
		},
		{
			name: "create zone without longitude",
			args: map[string]any{
				"action":   "create",
				"name":     "Test",
				"latitude": float64(51.5),
				"radius":   float64(50),
			},
			wantError:    true,
			wantContains: []string{"longitude", "required"},
		},
		{
			name: "create zone without radius",
			args: map[string]any{
				"action":    "create",
				"name":      "Test",
				"latitude":  float64(51.5),
				"longitude": float64(-0.1),
			},
			wantError:    true,
			wantContains: []string{"radius", "required"},
		},
		{
			name: "create zone API error",
			args: map[string]any{
				"action":    "create",
				"name":      "Test",
				"latitude":  float64(51.5),
				"longitude": float64(-0.1),
				"radius":    float64(50),
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateZoneFn = func(context.Context, homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
					return nil, fmt.Errorf("database error")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "creating"},
		},
	}

	h := NewZoneHandlers()
	runHandlerTestCases(t, tests, h.handleManageZone)
}

// =============================================================================
// Update Tests
// =============================================================================

func TestManageZone_Update(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "update zone name",
			args: map[string]any{
				"action":  "update",
				"zone_id": "zone_1",
				"name":    "Updated Zone",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_1", Name: "Test Zone", Latitude: 0, Longitude: 0, Radius: 100},
						{ID: "zone_2", Name: "Test Zone 2", Latitude: 0, Longitude: 0, Radius: 100},
					}, nil
				}
				m.UpdateZoneFn = func(_ context.Context, zoneID string, config homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
					if zoneID != "zone_1" {
						return nil, fmt.Errorf("unexpected zone_id: %s", zoneID)
					}
					return &homeassistant.ZoneRegistryEntry{
						ID:   zoneID,
						Name: config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully", "Updated Zone"},
		},
		{
			name: "update zone coordinates",
			args: map[string]any{
				"action":    "update",
				"zone_id":   "zone_2",
				"latitude":  float64(52.0),
				"longitude": float64(-1.0),
				"radius":    float64(75),
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_1", Name: "Test Zone", Latitude: 0, Longitude: 0, Radius: 100},
						{ID: "zone_2", Name: "Test", Latitude: 0, Longitude: 0, Radius: 50},
					}, nil
				}
				m.UpdateZoneFn = func(_ context.Context, zoneID string, _ homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
					return &homeassistant.ZoneRegistryEntry{
						ID:   zoneID,
						Name: "Test",
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully"},
		},
		{
			name: "update zone without zone_id",
			args: map[string]any{
				"action": "update",
				"name":   "New Name",
			},
			wantError:    true,
			wantContains: []string{"zone_id", "required"},
		},
		{
			name: "update zone API error",
			args: map[string]any{
				"action":  "update",
				"zone_id": "zone_1",
				"name":    "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_1", Name: "Test Zone", Latitude: 0, Longitude: 0, Radius: 100},
					}, nil
				}
				m.UpdateZoneFn = func(context.Context, string, homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
					return nil, fmt.Errorf("not found")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "updating"},
		},
	}

	h := NewZoneHandlers()
	runHandlerTestCases(t, tests, h.handleManageZone)
}

// =============================================================================
// Delete Tests
// =============================================================================

func TestManageZone_Delete(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "delete zone successfully",
			args: map[string]any{
				"action":  "delete",
				"zone_id": "zone_old",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_old", Name: "Old Zone", Latitude: 0, Longitude: 0, Radius: 100},
						{ID: "zone_new", Name: "New Zone", Latitude: 0, Longitude: 0, Radius: 100},
					}, nil
				}
				m.DeleteZoneFn = func(_ context.Context, zoneID string) error {
					if zoneID != "zone_old" {
						return fmt.Errorf("unexpected zone_id: %s", zoneID)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"deleted successfully", "zone_old"},
		},
		{
			name: "delete zone without zone_id",
			args: map[string]any{
				"action": "delete",
			},
			wantError:    true,
			wantContains: []string{"zone_id", "required"},
		},
		{
			name: "delete zone API error",
			args: map[string]any{
				"action":  "delete",
				"zone_id": "zone_home",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
					return []homeassistant.ZoneRegistryEntry{
						{ID: "zone_home", Name: "Home Zone", Latitude: 0, Longitude: 0, Radius: 100},
					}, nil
				}
				m.DeleteZoneFn = func(context.Context, string) error {
					return fmt.Errorf("cannot delete home zone")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "deleting"},
		},
	}

	h := NewZoneHandlers()
	runHandlerTestCases(t, tests, h.handleManageZone)
}

// =============================================================================
// Validation Tests
// =============================================================================

func TestManageZone_ValidationErrors(t *testing.T) {
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

	h := NewZoneHandlers()
	runHandlerTestCases(t, tests, h.handleManageZone)
}

// =============================================================================
// Registration Tests
// =============================================================================

func TestZoneHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewZoneHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	// Verify manage_zone is registered
	tools := registry.ListTools()
	found := false

	for _, tool := range tools {
		if tool.Name == "manage_zone" {
			found = true
			break
		}
	}

	if !found {
		t.Error("manage_zone tool not registered")
	}
}

func TestZoneList_RejectsNonStringFilters(t *testing.T) {
	t.Parallel()

	mustNotList := func(t *testing.T, c *UniversalMockClient) {
		t.Helper()
		c.GetZonesFn = func(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
			t.Errorf("HA must not be called for an invalid filter")
			return nil, nil
		}
	}

	runFilterTypeCases(t, NewZoneHandlers().handleManageZone, []filterTypeCase{
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
