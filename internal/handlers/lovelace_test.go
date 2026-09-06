package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/jsonpatch"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestNewDashboardHandlers(t *testing.T) {
	t.Parallel()

	h := NewDashboardHandlers()
	if h == nil {
		t.Error("NewDashboardHandlers() returned nil")
	}
}

func TestDashboardHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewDashboardHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 1
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	if tools[0].Name != "manage_dashboard" {
		t.Errorf("Tool name = %q, want %q", tools[0].Name, "manage_dashboard")
	}
}

func TestDashboardHandlers_Schema(t *testing.T) {
	t.Parallel()

	h := NewDashboardHandlers()
	tool := h.manageDashboardTool()

	if tool.Name != "manage_dashboard" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "manage_dashboard")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Verify action enum has 8 values: list, get, create, update, delete, save_config, patch, find
	actionProp, ok := tool.InputSchema.Properties["action"]
	if !ok {
		t.Fatal("expected 'action' property in schema")
	}
	if len(actionProp.Enum) != 8 {
		t.Errorf("expected 8 action enum values, got %d", len(actionProp.Enum))
	}

	expectedActions := []string{"list", "get", "create", "update", "delete", "save_config", "patch", "find"}
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

	// Verify mode enum has 2 values
	modeProp, ok := tool.InputSchema.Properties["mode"]
	if !ok {
		t.Fatal("expected 'mode' property in schema")
	}
	if len(modeProp.Enum) != 2 {
		t.Errorf("expected 2 mode enum values, got %d", len(modeProp.Enum))
	}
}

func TestHandleManageDashboard(t *testing.T) {
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
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return []homeassistant.DashboardEntry{}, nil
				}
			},
			wantErr:     false,
			wantContain: "0 dashboards",
		},
		{
			name: "list - multiple dashboards",
			args: map[string]any{
				"action": "list",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return []homeassistant.DashboardEntry{
						{ID: "lovelace-1", URLPath: "energy", Title: "Energy", Icon: "mdi:lightning-bolt", Mode: "storage", RequireAdmin: false, ShowInSidebar: true},
						{ID: "lovelace-2", URLPath: "security", Title: "Security", Icon: "mdi:security", Mode: "storage", RequireAdmin: true, ShowInSidebar: true},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"url_path"`,
		},
		{
			name: "list - natural format",
			args: map[string]any{
				"action": "list",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return []homeassistant.DashboardEntry{
						{ID: "lovelace-1", URLPath: "energy", Title: "Energy", Icon: "mdi:lightning-bolt", Mode: "storage", RequireAdmin: false, ShowInSidebar: true},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Energy",
		},
		{
			name: "list - API error",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantErr:     true,
			wantContain: "error listing dashboards",
		},

		// =========================
		// Get Action Tests
		// =========================
		{
			name: "get - default dashboard compact",
			args: map[string]any{
				"action": "get",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return map[string]any{
						"title": "Home",
						"views": []any{
							map[string]any{
								"title": "Overview",
								"path":  "overview",
								"cards": []any{
									map[string]any{"type": "entities"},
								},
							},
						},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Found 1 views",
		},
		{
			name: "get - specific dashboard by url_path",
			args: map[string]any{
				"action":   "get",
				"url_path": "energy",
				"format":   "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(_ context.Context, urlPath string) (map[string]any, error) {
					if urlPath == "energy" {
						return map[string]any{
							"title": "Energy Dashboard",
							"views": []any{
								map[string]any{"title": "Energy", "path": "energy"},
							},
						}, nil
					}
					return nil, fmt.Errorf("dashboard not found")
				}
			},
			wantErr:     false,
			wantContain: "Energy",
		},
		{
			name: "get - with view filter",
			args: map[string]any{
				"action": "get",
				"view":   "overview",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return map[string]any{
						"views": []any{
							map[string]any{"title": "Overview", "path": "overview"},
							map[string]any{"title": "Lights", "path": "lights"},
						},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Found 1 view(s) matching 'overview'",
		},
		{
			name: "get - verbose mode",
			args: map[string]any{
				"action":  "get",
				"verbose": true,
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return map[string]any{
						"title": "Home",
						"views": []any{
							map[string]any{"title": "Overview"},
						},
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Lovelace configuration with 1 views",
		},
		{
			name: "get - API error",
			args: map[string]any{
				"action": "get",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantErr:     true,
			wantContain: "error getting dashboard",
		},

		// =========================
		// Create Action Tests
		// =========================
		{
			name: "create - minimal dashboard",
			args: map[string]any{
				"action":   "create",
				"url_path": "test",
				"title":    "Test Dashboard",
				"mode":     "storage",
				"format":   "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateDashboardFn = func(_ context.Context, config homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
					// Verify defaults are set
					if config.RequireAdmin == nil {
						return nil, fmt.Errorf("RequireAdmin should not be nil")
					}
					if config.ShowInSidebar == nil {
						return nil, fmt.Errorf("ShowInSidebar should not be nil")
					}
					return &homeassistant.DashboardEntry{
						ID:            "lovelace-test",
						URLPath:       config.URLPath,
						Title:         config.Title,
						Mode:          "storage",
						RequireAdmin:  false,
						ShowInSidebar: true,
					}, nil
				}
			},
			wantErr:     false,
			wantContain: `"url_path"`,
		},
		{
			name: "create - full dashboard config",
			args: map[string]any{
				"action":          "create",
				"url_path":        "admin",
				"title":           "Admin Panel",
				"icon":            "mdi:shield-account",
				"mode":            "storage",
				"require_admin":   true,
				"show_in_sidebar": false,
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateDashboardFn = func(_ context.Context, config homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
					return &homeassistant.DashboardEntry{
						ID:            "lovelace-admin",
						URLPath:       config.URLPath,
						Title:         config.Title,
						Icon:          config.Icon,
						Mode:          config.Mode,
						RequireAdmin:  *config.RequireAdmin,
						ShowInSidebar: *config.ShowInSidebar,
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Admin Panel",
		},
		{
			name: "create - missing url_path",
			args: map[string]any{
				"action": "create",
				"title":  "Test",
			},
			wantErr:     true,
			wantContain: "url_path is required",
		},
		{
			name: "create - missing title",
			args: map[string]any{
				"action":   "create",
				"url_path": "test",
			},
			wantErr:     true,
			wantContain: "title is required",
		},
		{
			name: "create - missing mode",
			args: map[string]any{
				"action":   "create",
				"url_path": "test",
				"title":    "Test",
			},
			wantErr:     true,
			wantContain: "mode is required",
		},
		{
			name: "create - API error",
			args: map[string]any{
				"action":   "create",
				"url_path": "test",
				"title":    "Test",
				"mode":     "storage",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateDashboardFn = func(context.Context, homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
					return nil, fmt.Errorf("creation failed")
				}
			},
			wantErr:     true,
			wantContain: "error creating dashboard",
		},

		// =========================
		// Update Action Tests
		// =========================
		{
			name: "update - title only",
			args: map[string]any{
				"action":       "update",
				"dashboard_id": "lovelace-1",
				"title":        "Updated Title",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDashboardFn = func(_ context.Context, dashboardID string, config homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
					return &homeassistant.DashboardEntry{
						ID:      dashboardID,
						URLPath: "energy",
						Title:   config.Title,
						Mode:    "storage",
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "Updated Title",
		},
		{
			name: "update - multiple fields",
			args: map[string]any{
				"action":          "update",
				"dashboard_id":    "lovelace-1",
				"title":           "New Title",
				"icon":            "mdi:new-icon",
				"require_admin":   true,
				"show_in_sidebar": false,
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDashboardFn = func(_ context.Context, dashboardID string, config homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
					return &homeassistant.DashboardEntry{
						ID:            dashboardID,
						Title:         config.Title,
						Icon:          config.Icon,
						RequireAdmin:  *config.RequireAdmin,
						ShowInSidebar: *config.ShowInSidebar,
					}, nil
				}
			},
			wantErr:     false,
			wantContain: "New Title",
		},
		{
			name: "update - missing dashboard_id",
			args: map[string]any{
				"action": "update",
				"title":  "Test",
			},
			wantErr:     true,
			wantContain: "dashboard_id is required",
		},
		{
			name: "update - API error",
			args: map[string]any{
				"action":       "update",
				"dashboard_id": "lovelace-1",
				"title":        "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDashboardFn = func(context.Context, string, homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
					return nil, fmt.Errorf("update failed")
				}
			},
			wantErr:     true,
			wantContain: "error updating dashboard",
		},

		// =========================
		// Delete Action Tests
		// =========================
		{
			name: "delete - success",
			args: map[string]any{
				"action":       "delete",
				"dashboard_id": "lovelace-1",
			},
			setupMock: func(m *UniversalMockClient) {
				m.DeleteDashboardFn = func(context.Context, string) error {
					return nil
				}
			},
			wantErr:     false,
			wantContain: "Dashboard deleted successfully",
		},
		{
			name: "delete - missing dashboard_id",
			args: map[string]any{
				"action": "delete",
			},
			wantErr:     true,
			wantContain: "dashboard_id is required",
		},
		{
			name: "delete - API error",
			args: map[string]any{
				"action":       "delete",
				"dashboard_id": "lovelace-1",
			},
			setupMock: func(m *UniversalMockClient) {
				m.DeleteDashboardFn = func(context.Context, string) error {
					return fmt.Errorf("deletion failed")
				}
			},
			wantErr:     true,
			wantContain: "error deleting dashboard",
		},

		// =========================
		// Save Config Action Tests
		// =========================
		{
			name: "save_config - default dashboard",
			args: map[string]any{
				"action": "save_config",
				"config": map[string]any{
					"title": "Updated Home",
					"views": []any{
						map[string]any{"title": "New View"},
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.SaveLovelaceConfigFn = func(context.Context, string, map[string]any) error {
					return nil
				}
			},
			wantErr:     false,
			wantContain: "Dashboard configuration saved",
		},
		{
			name: "save_config - specific dashboard",
			args: map[string]any{
				"action":   "save_config",
				"url_path": "energy",
				"config": map[string]any{
					"title": "Energy",
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.SaveLovelaceConfigFn = func(_ context.Context, urlPath string, _ map[string]any) error {
					if urlPath != "energy" {
						return fmt.Errorf("wrong url_path")
					}
					return nil
				}
			},
			wantErr:     false,
			wantContain: "Dashboard configuration saved",
		},
		{
			name: "save_config - missing config",
			args: map[string]any{
				"action": "save_config",
			},
			wantErr:     true,
			wantContain: "config is required",
		},
		{
			name: "save_config - API error",
			args: map[string]any{
				"action": "save_config",
				"config": map[string]any{"title": "Test"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.SaveLovelaceConfigFn = func(context.Context, string, map[string]any) error {
					return fmt.Errorf("save failed")
				}
			},
			wantErr:     true,
			wantContain: "error saving dashboard",
		},

		// =========================
		// Invalid Action Tests
		// =========================
		{
			name: "invalid action",
			args: map[string]any{
				"action": "invalid",
			},
			wantErr:     true,
			wantContain: "invalid action",
		},
		{
			name:        "missing action",
			args:        map[string]any{},
			wantErr:     true,
			wantContain: "action is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(client)
			}

			h := NewDashboardHandlers()
			result, err := h.handleManageDashboard(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageDashboard() returned unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("handleManageDashboard() returned nil result")
				return
			}

			if result.IsError != tt.wantErr {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantErr)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContain != "" && !strings.Contains(strings.ToLower(content), strings.ToLower(tt.wantContain)) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContain)
			}
		})
	}
}

func TestHandleManageDashboard_Patch(t *testing.T) {
	t.Parallel()

	baseConfig := map[string]any{
		"title": "Home",
		"views": []any{
			map[string]any{
				"title": "Overview",
				"path":  "overview",
				"cards": []any{},
			},
		},
	}

	h := NewDashboardHandlers()

	tests := []handlerTestCase{
		{
			name: "patch - missing operations",
			args: map[string]any{
				"action": "patch",
			},
			wantError:    true,
			wantContains: []string{"operations is required"},
		},
		{
			name: "patch - success add view",
			args: map[string]any{
				"action": "patch",
				"operations": []any{
					map[string]any{
						"op":    "add",
						"path":  "/views/-",
						"value": map[string]any{"title": "New View", "path": "new"},
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					return deepCopyMap(baseConfig), nil
				}
				m.SaveLovelaceConfigFn = func(_ context.Context, _ string, _ map[string]any) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully", "1 operations"},
		},
		{
			name: "patch - success with url_path",
			args: map[string]any{
				"action":   "patch",
				"url_path": "energy",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/title", "value": "Updated Energy"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return deepCopyMap(baseConfig), nil
				}
				m.SaveLovelaceConfigFn = func(_ context.Context, _ string, _ map[string]any) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully", "energy"},
		},
		{
			name: "patch - get error",
			args: map[string]any{
				"action": "patch",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/title", "value": "New"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"error getting dashboard"},
		},
		{
			name: "patch - save error",
			args: map[string]any{
				"action": "patch",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/title", "value": "New"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					return deepCopyMap(baseConfig), nil
				}
				m.SaveLovelaceConfigFn = func(_ context.Context, _ string, _ map[string]any) error {
					return fmt.Errorf("save failed")
				}
			},
			wantError:    true,
			wantContains: []string{"error saving patched dashboard"},
		},
		{
			name: "patch - invalid patch op",
			args: map[string]any{
				"action": "patch",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/nonexistent/0", "value": "x"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					return deepCopyMap(baseConfig), nil
				}
			},
			wantError:    true,
			wantContains: []string{"error applying patch"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageDashboard)
}

// TestHandleManageDashboard_Patch_ViewOrderPreserved is a dedicated regression
// test verifying that replace /views/N must not reorder sibling views.
func TestHandleManageDashboard_Patch_ViewOrderPreserved(t *testing.T) {
	t.Parallel()

	fourViewConfig := map[string]any{
		"title": "Home",
		"views": []any{
			map[string]any{"title": "Übersicht", "path": "overview"},
			map[string]any{"title": "Twingo", "path": "twingo"},
			map[string]any{"title": "IONIQ 6", "path": "ioniq"},
			map[string]any{"title": "Wallbox", "path": "wallbox"},
		},
	}

	h := NewDashboardHandlers()

	var savedConfig map[string]any
	m := &UniversalMockClient{
		GetLovelaceConfigFn: func(context.Context, string) (map[string]any, error) {
			return deepCopyMap(fourViewConfig), nil
		},
		SaveLovelaceConfigFn: func(_ context.Context, _ string, cfg map[string]any) error {
			savedConfig = cfg
			return nil
		},
	}

	ctx := context.Background()
	result, err := h.handleManageDashboard(ctx, m, map[string]any{
		"action": "patch",
		"operations": []any{
			map[string]any{
				"op":    "replace",
				"path":  "/views/2",
				"value": map[string]any{"title": "IONIQ 6 Pro", "path": "ioniq"},
			},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result == nil {
		t.Fatal("handler returned nil result")
	}

	if savedConfig == nil {
		t.Fatal("SaveLovelaceConfig was not called")
	}

	views, ok := savedConfig["views"].([]any)
	if !ok {
		t.Fatalf("saved config 'views' is not []any, got %T", savedConfig["views"])
	}

	want := []string{"Übersicht", "Twingo", "IONIQ 6 Pro", "Wallbox"}
	if len(views) != len(want) {
		t.Fatalf("len(views) = %d, want %d", len(views), len(want))
	}
	for i, v := range views {
		vm, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("views[%d] is not a map, got %T", i, v)
		}
		if vm["title"] != want[i] {
			t.Errorf("views[%d].title = %v, want %q (sibling order changed)", i, vm["title"], want[i])
		}
	}
}

// TestBuildViewMoveOps verifies that the move-op generator produces ops that
// correctly restore an intended view order from a scrambled actual order.
func TestBuildViewMoveOps(t *testing.T) {
	t.Parallel()

	view := func(path string) any {
		return map[string]any{"title": path, "path": path}
	}

	tests := []struct {
		name     string
		intended []any
		actual   []any
		// wantOrder is the final order after applying the generated ops via jsonpatch.Apply.
		wantOrder []string
	}{
		{
			name:      "already in order - no ops needed",
			intended:  []any{view("a"), view("b"), view("c")},
			actual:    []any{view("a"), view("b"), view("c")},
			wantOrder: []string{"a", "b", "c"},
		},
		{
			name:      "two adjacent views swapped",
			intended:  []any{view("a"), view("b"), view("c"), view("d")},
			actual:    []any{view("b"), view("a"), view("c"), view("d")},
			wantOrder: []string{"a", "b", "c", "d"},
		},
		{
			name:      "first view moved to last",
			intended:  []any{view("a"), view("b"), view("c")},
			actual:    []any{view("b"), view("c"), view("a")},
			wantOrder: []string{"a", "b", "c"},
		},
		{
			name:      "full reverse",
			intended:  []any{view("a"), view("b"), view("c"), view("d")},
			actual:    []any{view("d"), view("c"), view("b"), view("a")},
			wantOrder: []string{"a", "b", "c", "d"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ops := buildViewMoveOps(tc.intended, tc.actual)

			// Apply the generated move ops to the actual doc.
			doc := map[string]any{"views": tc.actual}
			result, err := jsonpatch.Apply(doc, ops)
			if err != nil {
				// No ops needed → Apply should be a no-op on the unchanged doc.
				if len(ops) == 0 {
					result = doc
				} else {
					t.Fatalf("Apply(moveOps) error: %v", err)
				}
			}

			views := result.(map[string]any)["views"].([]any)
			if len(views) != len(tc.wantOrder) {
				t.Fatalf("len(views) = %d, want %d", len(views), len(tc.wantOrder))
			}
			for i, v := range views {
				got := v.(map[string]any)["path"]
				if got != tc.wantOrder[i] {
					t.Errorf("views[%d].path = %v, want %q", i, got, tc.wantOrder[i])
				}
			}
		})
	}
}

// TestHandleManageDashboard_Patch_HAReordersCorrected verifies that when HA
// returns a reordered views array after save, the handler detects and corrects it.
func TestHandleManageDashboard_Patch_HAReordersCorrected(t *testing.T) {
	t.Parallel()

	// Initial config with 4 views in intended order.
	initial := map[string]any{
		"title": "Home",
		"views": []any{
			map[string]any{"title": "A", "path": "a"},
			map[string]any{"title": "B", "path": "b"},
			map[string]any{"title": "C", "path": "c"},
			map[string]any{"title": "D", "path": "d"},
		},
	}

	// Simulated HA response after save: views[0] and views[1] swapped (the bug).
	haReordered := map[string]any{
		"title": "Home",
		"views": []any{
			map[string]any{"title": "B", "path": "b"},
			map[string]any{"title": "A", "path": "a"},
			map[string]any{"title": "C updated", "path": "c"},
			map[string]any{"title": "D", "path": "d"},
		},
	}

	h := NewDashboardHandlers()

	getCallCount := 0
	var savedConfigs []map[string]any
	m := &UniversalMockClient{
		GetLovelaceConfigFn: func(_ context.Context, _ string) (map[string]any, error) {
			getCallCount++
			if getCallCount == 1 {
				return deepCopyMap(initial), nil
			}
			// Second call (post-save read-back) returns HA's reordered version.
			return deepCopyMap(haReordered), nil
		},
		SaveLovelaceConfigFn: func(_ context.Context, _ string, cfg map[string]any) error {
			savedConfigs = append(savedConfigs, cfg)
			return nil
		},
	}

	ctx := context.Background()
	result, err := h.handleManageDashboard(ctx, m, map[string]any{
		"action": "patch",
		"operations": []any{
			map[string]any{
				"op":    "replace",
				"path":  "/views/2/title",
				"value": "C updated",
			},
		},
	})

	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result == nil {
		t.Fatal("handler returned nil result")
	}

	// Should have called SaveLovelaceConfig twice: once for the patch, once for the correction.
	if len(savedConfigs) != 2 {
		t.Fatalf("SaveLovelaceConfig called %d times, want 2 (patch + correction)", len(savedConfigs))
	}

	// The correction save (second call) must have the intended view order.
	corrected := savedConfigs[1]["views"].([]any)
	wantPaths := []string{"a", "b", "c", "d"}
	if len(corrected) != len(wantPaths) {
		t.Fatalf("corrected views len = %d, want %d", len(corrected), len(wantPaths))
	}
	for i, v := range corrected {
		vm, _ := v.(map[string]any)
		if vm["path"] != wantPaths[i] {
			t.Errorf("corrected views[%d].path = %v, want %q", i, vm["path"], wantPaths[i])
		}
	}

	// Success message must mention the correction.
	var msgText string
	for _, cb := range result.Content {
		msgText += cb.Text
	}
	if !strings.Contains(msgText, "reordered") {
		t.Errorf("success message should mention the reorder correction, got: %q", msgText)
	}
}

func TestHandleManageDashboard_SemanticPatch(t *testing.T) {
	t.Parallel()

	baseConfig := map[string]any{
		"views": []any{
			map[string]any{"title": "Overview", "path": "overview", "type": "sections"},
			map[string]any{"title": "Lights", "path": "lights", "type": "sections"},
		},
	}

	h := &DashboardHandlers{}

	tests := []handlerTestCase{
		{
			name: "semantic patch - replace view type by title",
			args: map[string]any{
				"action": "patch",
				"operations": []any{
					map[string]any{
						"op":      "replace",
						"match":   map[string]any{"title": "Lights"},
						"section": "views",
						"field":   "path",
						"value":   "lights-updated",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					cfg := make(map[string]any)
					for k, v := range baseConfig {
						cfg[k] = v
					}
					return cfg, nil
				}
				m.SaveLovelaceConfigFn = func(_ context.Context, _ string, _ map[string]any) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully"},
		},
		{
			name: "semantic patch - no matching view",
			args: map[string]any{
				"action": "patch",
				"operations": []any{
					map[string]any{
						"op":      "replace",
						"match":   map[string]any{"title": "Nonexistent View"},
						"section": "views",
						"field":   "path",
						"value":   "x",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLovelaceConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					cfg := make(map[string]any)
					for k, v := range baseConfig {
						cfg[k] = v
					}
					return cfg, nil
				}
			},
			wantError:    true,
			wantContains: []string{"error applying patch", "no elements"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageDashboard)
}

// deepCopyMap creates a shallow copy of a map to avoid test interference.
func deepCopyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Test helper functions (keep existing tests)

func TestFilterViewsByQuery(t *testing.T) {
	t.Parallel()

	views := []any{
		map[string]any{"title": "Overview", "path": "overview"},
		map[string]any{"title": "Lights", "path": "lights"},
		map[string]any{"title": "Climate Control", "path": "climate"},
	}

	tests := []struct {
		name      string
		views     []any
		query     string
		wantCount int
	}{
		{
			name:      "exact match by path",
			views:     views,
			query:     "overview",
			wantCount: 1,
		},
		{
			name:      "partial match by title",
			views:     views,
			query:     "control",
			wantCount: 1,
		},
		{
			name:      "case insensitive match",
			views:     views,
			query:     "LIGHTS",
			wantCount: 1,
		},
		{
			name:      "no match",
			views:     views,
			query:     "nonexistent",
			wantCount: 0,
		},
		{
			name:      "empty query matches all",
			views:     views,
			query:     "",
			wantCount: 3,
		},
		{
			name:      "empty views",
			views:     []any{},
			query:     "test",
			wantCount: 0,
		},
		{
			name:      "invalid view type skipped",
			views:     []any{"not a map", map[string]any{"title": "Valid"}},
			query:     "valid",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := filterViewsByQuery(tt.views, tt.query)
			if len(result) != tt.wantCount {
				t.Errorf("filterViewsByQuery() returned %d views, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestCountCardsInView(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		viewMap map[string]any
		want    int
	}{
		{
			name:    "empty view",
			viewMap: map[string]any{},
			want:    0,
		},
		{
			name: "cards only",
			viewMap: map[string]any{
				"cards": []any{
					map[string]any{"type": "entities"},
					map[string]any{"type": "button"},
				},
			},
			want: 2,
		},
		{
			name: "sections only",
			viewMap: map[string]any{
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{"type": "light"},
						},
					},
					map[string]any{
						"cards": []any{
							map[string]any{"type": "sensor"},
							map[string]any{"type": "weather"},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "cards and sections combined",
			viewMap: map[string]any{
				"cards": []any{
					map[string]any{"type": "entities"},
				},
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{"type": "button"},
							map[string]any{"type": "light"},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "section without cards",
			viewMap: map[string]any{
				"sections": []any{
					map[string]any{"title": "Empty Section"},
				},
			},
			want: 0,
		},
		{
			name: "invalid section type skipped",
			viewMap: map[string]any{
				"sections": []any{
					"not a map",
					map[string]any{
						"cards": []any{map[string]any{"type": "button"}},
					},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := countCardsInView(tt.viewMap)
			if got != tt.want {
				t.Errorf("countCardsInView() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildCompactViewEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		viewMap map[string]any
		want    compactViewEntry
	}{
		{
			name:    "empty view",
			viewMap: map[string]any{},
			want:    compactViewEntry{},
		},
		{
			name: "full view",
			viewMap: map[string]any{
				"title":   "Overview",
				"path":    "overview",
				"icon":    "mdi:home",
				"subview": true,
				"cards":   []any{map[string]any{"type": "entities"}},
				"badges":  []any{map[string]any{"entity": "sensor.temp"}},
			},
			want: compactViewEntry{
				Title:      "Overview",
				Path:       "overview",
				Icon:       "mdi:home",
				Subview:    true,
				CardCount:  1,
				BadgeCount: 1,
			},
		},
		{
			name: "view without badges",
			viewMap: map[string]any{
				"title": "Simple",
				"path":  "simple",
				"cards": []any{map[string]any{"type": "button"}, map[string]any{"type": "light"}},
			},
			want: compactViewEntry{
				Title:     "Simple",
				Path:      "simple",
				CardCount: 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildCompactViewEntry(tt.viewMap)
			if got != tt.want {
				t.Errorf("buildCompactViewEntry() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildCompactViews(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		views []any
		want  int
	}{
		{
			name:  "empty views",
			views: []any{},
			want:  0,
		},
		{
			name: "multiple views",
			views: []any{
				map[string]any{"title": "View1"},
				map[string]any{"title": "View2"},
				map[string]any{"title": "View3"},
			},
			want: 3,
		},
		{
			name: "invalid views skipped",
			views: []any{
				"not a map",
				map[string]any{"title": "Valid"},
				42,
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildCompactViews(tt.views)
			if len(got) != tt.want {
				t.Errorf("buildCompactViews() returned %d entries, want %d", len(got), tt.want)
			}
		})
	}
}

func TestDashboardHandlers_CreateDashboardInitializesSectionLayout(t *testing.T) {
	t.Parallel()

	// Test that handleCreate attempts to initialize section-based layout
	// We verify this by checking that SaveLovelaceConfig is called with the correct structure

	h := NewDashboardHandlers()
	ctx := context.Background()

	var savedConfig map[string]any
	var savedURLPath string
	saveCalled := false

	mock := &UniversalMockClient{
		CreateDashboardFn: func(context.Context, homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
			return &homeassistant.DashboardEntry{
				ID:            "lovelace-test",
				URLPath:       "test-dashboard",
				Title:         "Test Dashboard",
				Mode:          "storage",
				RequireAdmin:  false,
				ShowInSidebar: false,
			}, nil
		},
		SaveLovelaceConfigFn: func(_ context.Context, urlPath string, config map[string]any) error {
			saveCalled = true
			savedURLPath = urlPath
			savedConfig = config
			return nil
		},
	}

	args := map[string]any{
		"action":   "create",
		"url_path": "test-dashboard",
		"title":    "Test Dashboard",
		"mode":     "storage",
	}

	result, err := h.handleManageDashboard(ctx, mock, args)
	if err != nil {
		t.Fatalf("handleManageDashboard() error = %v", err)
	}

	if result.IsError {
		t.Fatalf("handleManageDashboard() returned error result: %s", result.Content[0].Text)
	}

	// Verify SaveLovelaceConfig was called
	if !saveCalled {
		t.Error("SaveLovelaceConfig was not called during dashboard creation")
	}

	// Verify URL path matches
	if savedURLPath != "test-dashboard" {
		t.Errorf("SaveLovelaceConfig called with url_path = %q, want %q", savedURLPath, "test-dashboard")
	}

	// Verify config structure has modern section layout
	views, ok := savedConfig["views"].([]any)
	if !ok || len(views) != 1 {
		t.Fatalf("Config should have exactly 1 view, got: %v", savedConfig)
	}

	view := views[0].(map[string]any)

	// Check view title
	if view["title"] != "Home" {
		t.Errorf("Default view title = %q, want %q", view["title"], "Home")
	}

	// Check view type is "sections"
	viewType, ok := view["type"].(string)
	if !ok || viewType != "sections" {
		t.Errorf("View type = %q, want %q (modern section layout)", viewType, "sections")
	}

	// Check sections exist
	sections, ok := view["sections"].([]any)
	if !ok || len(sections) != 1 {
		t.Fatalf("View should have exactly 1 section, got: %v", view)
	}

	section := sections[0].(map[string]any)

	// Check section type is grid
	if section["type"] != "grid" {
		t.Errorf("Section type = %q, want %q", section["type"], "grid")
	}

	// Check section has empty cards array
	cards, ok := section["cards"].([]any)
	if !ok {
		t.Error("Section should have cards array")
	}
	if len(cards) != 0 {
		t.Errorf("Section should have empty cards array, got %d cards", len(cards))
	}

	// Verify view does NOT have root-level cards (legacy format)
	if _, hasCards := view["cards"]; hasCards {
		t.Error("View should not have root-level 'cards' field (legacy format)")
	}
}

// TestHandleManageDashboard_Find covers locating content in a
// dashboard too large to fetch wholesale, including content nested several
// card levels deep.
func TestHandleManageDashboard_Find(t *testing.T) {
	t.Parallel()

	nestedConfig := map[string]any{
		"title": "Home",
		"views": []any{
			map[string]any{
				"title": "Home",
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{
								"type": "vertical-stack",
								"cards": []any{
									map[string]any{
										"type": "tile",
										"chips": []any{
											map[string]any{"type": "entity", "entity": "light.desk"},
											map[string]any{"type": "entity", "entity": "device_tracker.example_phone"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	h := NewDashboardHandlers()

	tests := []handlerTestCase{
		{
			name: "find - missing search",
			args: map[string]any{
				"action": "find",
			},
			wantError:    true,
			wantContains: []string{"search is required"},
		},
		{
			name: "find - locates deeply nested chip across all dashboards",
			args: map[string]any{
				"action": "find",
				"search": "device_tracker.example_phone",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return []homeassistant.DashboardEntry{{URLPath: "lovelace"}}, nil
				}
				m.GetLovelaceConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					return deepCopyMap(nestedConfig), nil
				}
			},
			// Searches both the default ("") dashboard and the listed "lovelace" one.
			wantError:    false,
			wantContains: []string{"Found 2 match(es)", "views/0/sections/0/cards/0/cards/0/chips/1/entity"},
		},
		{
			name: "find - explicit url_path skips dashboard listing",
			args: map[string]any{
				"action":   "find",
				"search":   "device_tracker.example_phone",
				"url_path": "energy",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					t.Fatal("should not list dashboards when url_path is given")
					return nil, nil
				}
				m.GetLovelaceConfigFn = func(_ context.Context, urlPath string) (map[string]any, error) {
					if urlPath != "energy" {
						t.Fatalf("expected url_path 'energy', got %q", urlPath)
					}
					return deepCopyMap(nestedConfig), nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 1 match"},
		},
		{
			name: "find - no matches",
			args: map[string]any{
				"action": "find",
				"search": "device_tracker.nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, nil
				}
				m.GetLovelaceConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					return deepCopyMap(nestedConfig), nil
				}
			},
			wantError:    false,
			wantContains: []string{"No matches"},
		},
		{
			name: "find - list dashboards error",
			args: map[string]any{
				"action": "find",
				"search": "device_tracker.example_phone",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"error listing dashboards"},
		},
		{
			name: "find - all dashboard fetches fail returns error, not false no-matches",
			args: map[string]any{
				"action": "find",
				"search": "device_tracker.example_phone",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return []homeassistant.DashboardEntry{{URLPath: "lovelace"}}, nil
				}
				m.GetLovelaceConfigFn = func(_ context.Context, _ string) (map[string]any, error) {
					return nil, fmt.Errorf("ws timeout")
				}
			},
			wantError:    true,
			wantContains: []string{"Could not search any dashboard", "default", "lovelace"},
		},
		{
			name: "find - some dashboard fetches fail, success message includes warning",
			args: map[string]any{
				"action": "find",
				"search": "device_tracker.example_phone",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return []homeassistant.DashboardEntry{{URLPath: "lovelace"}}, nil
				}
				m.GetLovelaceConfigFn = func(_ context.Context, urlPath string) (map[string]any, error) {
					if urlPath == "lovelace" {
						return nil, fmt.Errorf("ws timeout")
					}
					return deepCopyMap(nestedConfig), nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 1 match", "could not be scanned: lovelace (ws timeout)"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageDashboard)
}
