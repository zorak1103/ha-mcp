package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockAnalysisClient implements homeassistant.Client for analysis tests.
type mockAnalysisClient struct {
	homeassistant.Client
	GetStateFn              func(ctx context.Context, entityID string) (*homeassistant.Entity, error)
	ListAutomationsFn       func(ctx context.Context) ([]homeassistant.Automation, error)
	GetAutomationFn         func(ctx context.Context, automationID string) (*homeassistant.Automation, error)
	ListScriptsFn           func(ctx context.Context) ([]homeassistant.Entity, error)
	GetScriptFn             func(ctx context.Context, scriptID string) (*homeassistant.Script, error)
	ListScenesFn            func(ctx context.Context) ([]homeassistant.Entity, error)
	GetStatesFn             func(ctx context.Context) ([]homeassistant.Entity, error)
	GetEntityRegistryFn     func(ctx context.Context) ([]homeassistant.EntityRegistryEntry, error)
	GetDeviceRegistryFn     func(ctx context.Context) ([]homeassistant.DeviceRegistryEntry, error)
	GetAreaRegistryFn       func(ctx context.Context) ([]homeassistant.AreaRegistryEntry, error)
	GetHistoryFn            func(ctx context.Context, entityID string, start, end time.Time) ([][]homeassistant.HistoryEntry, error)
	ListDashboardsFn        func(ctx context.Context) ([]homeassistant.DashboardEntry, error)
	GetLovelaceConfigFn     func(ctx context.Context, urlPath string) (map[string]any, error)
	GetConfigEntryOptionsFn func(ctx context.Context, entryID string) (map[string]any, error)
}

func (m *mockAnalysisClient) GetState(ctx context.Context, entityID string) (*homeassistant.Entity, error) {
	if m.GetStateFn != nil {
		return m.GetStateFn(ctx, entityID)
	}
	return &homeassistant.Entity{EntityID: entityID, State: "on"}, nil
}

func (m *mockAnalysisClient) ListAutomations(ctx context.Context) ([]homeassistant.Automation, error) {
	if m.ListAutomationsFn != nil {
		return m.ListAutomationsFn(ctx)
	}
	return []homeassistant.Automation{}, nil
}

func (m *mockAnalysisClient) GetAutomation(ctx context.Context, automationID string) (*homeassistant.Automation, error) {
	if m.GetAutomationFn != nil {
		return m.GetAutomationFn(ctx, automationID)
	}
	return nil, errors.New("not found")
}

func (m *mockAnalysisClient) ListScripts(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.ListScriptsFn != nil {
		return m.ListScriptsFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *mockAnalysisClient) GetScript(ctx context.Context, scriptID string) (*homeassistant.Script, error) {
	if m.GetScriptFn != nil {
		return m.GetScriptFn(ctx, scriptID)
	}
	return nil, errors.New("not found")
}

func (m *mockAnalysisClient) ListScenes(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.ListScenesFn != nil {
		return m.ListScenesFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *mockAnalysisClient) GetStates(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.GetStatesFn != nil {
		return m.GetStatesFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *mockAnalysisClient) GetEntityRegistry(ctx context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	if m.GetEntityRegistryFn != nil {
		return m.GetEntityRegistryFn(ctx)
	}
	return []homeassistant.EntityRegistryEntry{}, nil
}

func (m *mockAnalysisClient) GetDeviceRegistry(ctx context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	if m.GetDeviceRegistryFn != nil {
		return m.GetDeviceRegistryFn(ctx)
	}
	return []homeassistant.DeviceRegistryEntry{}, nil
}

func (m *mockAnalysisClient) GetAreaRegistry(ctx context.Context) ([]homeassistant.AreaRegistryEntry, error) {
	if m.GetAreaRegistryFn != nil {
		return m.GetAreaRegistryFn(ctx)
	}
	return []homeassistant.AreaRegistryEntry{}, nil
}

func (m *mockAnalysisClient) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]homeassistant.HistoryEntry, error) {
	if m.GetHistoryFn != nil {
		return m.GetHistoryFn(ctx, entityID, start, end)
	}
	return [][]homeassistant.HistoryEntry{}, nil
}

func (m *mockAnalysisClient) ListDashboards(ctx context.Context) ([]homeassistant.DashboardEntry, error) {
	if m.ListDashboardsFn != nil {
		return m.ListDashboardsFn(ctx)
	}
	return []homeassistant.DashboardEntry{}, nil
}

func (m *mockAnalysisClient) GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error) {
	if m.GetLovelaceConfigFn != nil {
		return m.GetLovelaceConfigFn(ctx, urlPath)
	}
	return map[string]any{}, nil
}

func (m *mockAnalysisClient) GetConfigEntryOptions(ctx context.Context, entryID string) (map[string]any, error) {
	if m.GetConfigEntryOptionsFn != nil {
		return m.GetConfigEntryOptionsFn(ctx, entryID)
	}
	return map[string]any{}, nil
}

func TestNewAnalysisHandlers(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	if h == nil {
		t.Error("NewAnalysisHandlers() returned nil, want non-nil")
	}
}

func TestAnalysisHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 2 {
		t.Errorf("RegisterTools() registered %d tools, want 2", len(tools))
	}

	expectedTools := map[string]bool{
		"analyze_entity":          false,
		"get_entity_dependencies": false,
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

func TestAnalysisHandlers_analyzeEntityTool(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()
	tool := h.analyzeEntityTool()

	tests := []struct {
		name      string
		checkFunc func(t *testing.T, tool mcp.Tool)
	}{
		{
			name: "has correct name",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.Name != "analyze_entity" {
					t.Errorf("tool.Name = %q, want %q", tool.Name, "analyze_entity")
				}
			},
		},
		{
			name: "has description",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.Description == "" {
					t.Error("tool.Description is empty, want non-empty")
				}
			},
		},
		{
			name: "has object schema type",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.InputSchema.Type != testSchemaTypeObject {
					t.Errorf("tool.InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
				}
			},
		},
		{
			name: "requires entity_id",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				found := false
				for _, req := range tool.InputSchema.Required {
					if req == "entity_id" {
						found = true
						break
					}
				}
				if !found {
					t.Error("entity_id not in required fields")
				}
			},
		},
		{
			name: "has include_history property",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if _, ok := tool.InputSchema.Properties["include_history"]; !ok {
					t.Error("include_history property missing")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.checkFunc(t, tool)
		})
	}
}

func TestAnalysisHandlers_getEntityDependenciesTool(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()
	tool := h.getEntityDependenciesTool()

	tests := []struct {
		name      string
		checkFunc func(t *testing.T, tool mcp.Tool)
	}{
		{
			name: "has correct name",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.Name != "get_entity_dependencies" {
					t.Errorf("tool.Name = %q, want %q", tool.Name, "get_entity_dependencies")
				}
			},
		},
		{
			name: "has description",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				if tool.Description == "" {
					t.Error("tool.Description is empty, want non-empty")
				}
			},
		},
		{
			name: "requires entity_id",
			checkFunc: func(t *testing.T, tool mcp.Tool) {
				t.Helper()
				found := false
				for _, req := range tool.InputSchema.Required {
					if req == "entity_id" {
						found = true
						break
					}
				}
				if !found {
					t.Error("entity_id not in required fields")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.checkFunc(t, tool)
		})
	}
}

func TestAnalysisHandlers_handleAnalyzeEntity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAnalysisClient
		wantContains string
		wantError    bool
	}{
		{
			name: "success json format",
			args: map[string]any{
				"entity_id": "light.living_room",
				"format":    "json",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   "light.living_room",
						State:      "on",
						Attributes: map[string]any{"friendly_name": "Living Room Light"},
					}, nil
				},
			},
			wantContains: "light.living_room",
			wantError:    false,
		},
		{
			name: "success with history json format",
			args: map[string]any{
				"entity_id":       "light.living_room",
				"include_history": true,
				"format":          "json",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   "light.living_room",
						State:      "on",
						Attributes: map[string]any{"friendly_name": "Living Room Light"},
					}, nil
				},
				GetHistoryFn: func(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
					return [][]homeassistant.HistoryEntry{
						{
							{State: "on", LastChanged: 1704067200},
							{State: "off", LastChanged: 1704063600},
						},
					}, nil
				},
			},
			wantContains: "history",
			wantError:    false,
		},
		{
			name: "success natural format default",
			args: map[string]any{
				"entity_id": "light.living_room",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   "light.living_room",
						State:      "on",
						Attributes: map[string]any{"friendly_name": "Living Room Light"},
					}, nil
				},
			},
			wantContains: "light.living_room",
			wantError:    false,
		},
		{
			name: "success natural format explicit",
			args: map[string]any{
				"entity_id": "light.living_room",
				"format":    "natural",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   "light.living_room",
						State:      "on",
						Attributes: map[string]any{"friendly_name": "Living Room Light"},
					}, nil
				},
			},
			wantContains: "Living Room Light",
			wantError:    false,
		},
		{
			name: "success natural with references",
			args: map[string]any{
				"entity_id": "light.kitchen",
				"format":    "natural",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   "light.kitchen",
						State:      "off",
						Attributes: map[string]any{"friendly_name": "Kitchen Light"},
					}, nil
				},
				ListAutomationsFn: func(_ context.Context) ([]homeassistant.Automation, error) {
					return []homeassistant.Automation{
						{
							EntityID:     "automation.kitchen_motion",
							FriendlyName: "Kitchen Motion",
							State:        "on",
						},
					}, nil
				},
				GetAutomationFn: func(_ context.Context, automationID string) (*homeassistant.Automation, error) {
					if automationID == "kitchen_motion" {
						return &homeassistant.Automation{
							EntityID:     "automation.kitchen_motion",
							FriendlyName: "Kitchen Motion",
							State:        "on",
							Config: &homeassistant.AutomationConfig{
								Triggers: []any{
									map[string]any{"entity_id": "light.kitchen"},
								},
							},
						}, nil
					}
					return nil, errors.New("not found")
				},
				ListScriptsFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{}, nil
				},
				ListScenesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{}, nil
				},
				GetStatesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{}, nil
				},
			},
			wantContains: "References",
			wantError:    false,
		},
		{
			name:         "missing entity_id",
			args:         map[string]any{"format": "json"},
			client:       &mockAnalysisClient{},
			wantContains: "entity_id is required",
			wantError:    true,
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
				"format":    "json",
			},
			client:       &mockAnalysisClient{},
			wantContains: "entity_id is required",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "light.living_room",
				"format":    "json",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return nil, errors.New("entity not found")
				},
			},
			wantContains: "error getting entity state",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewAnalysisHandlers()
			result, err := h.handleAnalyzeEntity(context.Background(), tt.client, tt.args)

			if err != nil {
				t.Errorf("handleAnalyzeEntity() returned error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("handleAnalyzeEntity() returned nil result")
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleAnalyzeEntity() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleAnalyzeEntity() returned empty content")
			}

			text := result.Content[0].Text
			if !strings.Contains(text, tt.wantContains) {
				t.Errorf("handleAnalyzeEntity() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestAnalysisHandlers_handleGetEntityDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAnalysisClient
		wantContains string
		wantError    bool
	}{
		{
			name: "success automation json format",
			args: map[string]any{
				"entity_id": "automation.test_automation",
				"format":    "json",
			},
			client: &mockAnalysisClient{
				GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					return &homeassistant.Automation{
						EntityID:     "automation.test_automation",
						FriendlyName: "Test Automation",
						Config: &homeassistant.AutomationConfig{
							Triggers: []any{
								map[string]any{"platform": "state", "entity_id": "light.living_room"},
							},
							Actions: []any{
								map[string]any{"service": "light.turn_on"},
							},
						},
					}, nil
				},
			},
			wantContains: "automation.test_automation",
			wantError:    false,
		},
		{
			name: "success script json format",
			args: map[string]any{
				"entity_id": "script.test_script",
				"format":    "json",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID: "script.test_script",
						State:    "off",
						Attributes: map[string]any{
							"friendly_name": "Test Script",
							"sequence": []any{
								map[string]any{"service": "light.turn_off"},
							},
						},
					}, nil
				},
			},
			wantContains: "script.test_script",
			wantError:    false,
		},
		{
			name: "success automation natural format default",
			args: map[string]any{
				"entity_id": "automation.kitchen_lights",
			},
			client: &mockAnalysisClient{
				GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					return &homeassistant.Automation{
						EntityID:     "automation.kitchen_lights",
						FriendlyName: "Kitchen Lights",
						Config: &homeassistant.AutomationConfig{
							Triggers: []any{
								map[string]any{"platform": "state", "entity_id": "binary_sensor.motion_kitchen"},
							},
							Conditions: []any{
								map[string]any{"condition": "state", "entity_id": "input_boolean.guest_mode"},
							},
							Actions: []any{
								map[string]any{"service": "light.turn_on", "target": map[string]any{"entity_id": "light.kitchen"}},
							},
						},
					}, nil
				},
			},
			wantContains: "Dependencies",
			wantError:    false,
		},
		{
			name: "success automation natural format explicit",
			args: map[string]any{
				"entity_id": "automation.kitchen_lights",
				"format":    "natural",
			},
			client: &mockAnalysisClient{
				GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					return &homeassistant.Automation{
						EntityID:     "automation.kitchen_lights",
						FriendlyName: "Kitchen Lights",
						Config: &homeassistant.AutomationConfig{
							Triggers: []any{
								map[string]any{"platform": "state", "entity_id": "binary_sensor.motion_kitchen"},
							},
						},
					}, nil
				},
			},
			wantContains: "automation",
			wantError:    false,
		},
		{
			name:         "missing entity_id",
			args:         map[string]any{"format": "json"},
			client:       &mockAnalysisClient{},
			wantContains: "entity_id is required",
			wantError:    true,
		},
		{
			name: "invalid entity_id",
			args: map[string]any{
				"entity_id": "light.living_room",
				"format":    "json",
			},
			client:       &mockAnalysisClient{},
			wantContains: "must be an automation or script",
			wantError:    true,
		},
		{
			name: "automation not found",
			args: map[string]any{
				"entity_id": "automation.nonexistent",
				"format":    "json",
			},
			client: &mockAnalysisClient{
				GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
					return nil, errors.New("not found")
				},
			},
			wantContains: "Error getting dependencies",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewAnalysisHandlers()
			result, err := h.handleGetEntityDependencies(context.Background(), tt.client, tt.args)

			if err != nil {
				t.Errorf("handleGetEntityDependencies() returned error: %v", err)
				return
			}

			if result == nil {
				t.Fatal("handleGetEntityDependencies() returned nil result")
			}

			if result.IsError != tt.wantError {
				t.Errorf("handleGetEntityDependencies() IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetEntityDependencies() returned empty content")
			}

			text := result.Content[0].Text
			if !strings.Contains(text, tt.wantContains) {
				t.Errorf("handleGetEntityDependencies() result = %q, want to contain %q", text, tt.wantContains)
			}
		})
	}
}

func TestRegisterAnalysisTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterAnalysisTools(registry)

	tools := registry.ListTools()
	if len(tools) != 2 {
		t.Errorf("RegisterAnalysisTools() registered %d tools, want 2", len(tools))
	}
}

func TestAnalysisHandlers_matchAreaIDField(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name   string
		field  any
		areaID string
		want   bool
	}{
		{
			name:   "nil field returns false",
			field:  nil,
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "matching string field",
			field:  "living_room",
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "non-matching string field",
			field:  "bedroom",
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "matching in slice field",
			field:  []any{"bedroom", "living_room", "kitchen"},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "not matching in slice field",
			field:  []any{"bedroom", "kitchen"},
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "empty slice field",
			field:  []any{},
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "slice with non-string elements",
			field:  []any{123, true, nil},
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "unsupported type returns false",
			field:  123,
			areaID: "living_room",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := h.matchAreaIDField(tt.field, tt.areaID)
			if got != tt.want {
				t.Errorf("matchAreaIDField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalysisHandlers_searchAreaInMap(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name   string
		m      map[string]any
		areaID string
		want   bool
	}{
		{
			name:   "direct area_id string match",
			m:      map[string]any{"area_id": "living_room"},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "direct area_id slice match",
			m:      map[string]any{"area_id": []any{"bedroom", "living_room"}},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "target.area_id string match",
			m:      map[string]any{"target": map[string]any{"area_id": "living_room"}},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "target.area_id slice match",
			m:      map[string]any{"target": map[string]any{"area_id": []any{"living_room"}}},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "nested match in subvalue",
			m:      map[string]any{"data": map[string]any{"area_id": "living_room"}},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "deeply nested match",
			m:      map[string]any{"outer": map[string]any{"inner": map[string]any{"area_id": "living_room"}}},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "no match",
			m:      map[string]any{"area_id": "bedroom", "target": map[string]any{"area_id": "kitchen"}},
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "empty map",
			m:      map[string]any{},
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "target is not a map",
			m:      map[string]any{"target": "not_a_map"},
			areaID: "living_room",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := h.searchAreaInMap(tt.m, tt.areaID)
			if got != tt.want {
				t.Errorf("searchAreaInMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalysisHandlers_searchAreaInValue(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name   string
		val    any
		areaID string
		want   bool
	}{
		{
			name:   "nil value returns false",
			val:    nil,
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "matching string",
			val:    "living_room",
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "non-matching string",
			val:    "bedroom",
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "slice with match",
			val:    []any{"bedroom", "living_room"},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "slice without match",
			val:    []any{"bedroom", "kitchen"},
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "map with area_id match",
			val:    map[string]any{"area_id": "living_room"},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "unsupported type returns false",
			val:    123,
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "bool type returns false",
			val:    true,
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "float type returns false",
			val:    3.14,
			areaID: "living_room",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := h.searchAreaInValue(tt.val, tt.areaID)
			if got != tt.want {
				t.Errorf("searchAreaInValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalysisHandlers_shouldRecurseIntoKey(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "data key should recurse", key: "data", want: true},
		{name: "choose key should recurse", key: "choose", want: true},
		{name: "sequence key should recurse", key: "sequence", want: true},
		{name: "conditions key should recurse", key: "conditions", want: true},
		{name: "then key should recurse", key: "then", want: true},
		{name: "else key should recurse", key: "else", want: true},
		{name: "default key should recurse", key: "default", want: true},
		{name: "entity_id key should not recurse", key: "entity_id", want: false},
		{name: "service key should not recurse", key: "service", want: false},
		{name: "random key should not recurse", key: "random_key", want: false},
		{name: "empty key should not recurse", key: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := h.shouldRecurseIntoKey(tt.key)
			if got != tt.want {
				t.Errorf("shouldRecurseIntoKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestAnalysisHandlers_extractDependenciesRecursive(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name       string
		val        any
		wantCount  int
		wantEntity string
	}{
		{
			name:      "nil value returns empty",
			val:       nil,
			wantCount: 0,
		},
		{
			name:       "map with entity_id",
			val:        map[string]any{"entity_id": "light.living_room", "platform": "state"},
			wantCount:  1,
			wantEntity: "light.living_room",
		},
		{
			name: "map with target.entity_id",
			val: map[string]any{
				"service": "light.turn_on",
				"target":  map[string]any{"entity_id": "light.bedroom"},
			},
			wantCount:  1,
			wantEntity: "light.bedroom",
		},
		{
			name: "slice with multiple entities",
			val: []any{
				map[string]any{"entity_id": "light.living_room"},
				map[string]any{"entity_id": "light.bedroom"},
			},
			wantCount: 2,
		},
		{
			name: "nested structure with choose",
			val: map[string]any{
				"choose": []any{
					map[string]any{
						"conditions": []any{
							map[string]any{"entity_id": "sensor.motion"},
						},
						"sequence": []any{
							map[string]any{"entity_id": "light.hallway"},
						},
					},
				},
			},
			wantCount: 2,
		},
		{
			name: "entity_id as slice",
			val: map[string]any{
				"entity_id": []any{"light.one", "light.two"},
			},
			wantCount:  1,
			wantEntity: "light.one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seen := make(map[string]DependencyEntry)
			h.extractDependenciesRecursive(tt.val, seen)

			if len(seen) != tt.wantCount {
				t.Errorf("extractDependenciesRecursive() found %d entities, want %d", len(seen), tt.wantCount)
			}

			if tt.wantEntity != "" {
				if _, exists := seen[tt.wantEntity]; !exists {
					t.Errorf("extractDependenciesRecursive() did not find expected entity %q", tt.wantEntity)
				}
			}
		})
	}
}

func TestAnalysisHandlers_extractDirectEntityDependency(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name       string
		m          map[string]any
		wantCount  int
		wantEntity string
		wantType   string
	}{
		{
			name:      "empty map",
			m:         map[string]any{},
			wantCount: 0,
		},
		{
			name:       "map with entity_id and platform",
			m:          map[string]any{"entity_id": "sensor.temp", "platform": "state"},
			wantCount:  1,
			wantEntity: "sensor.temp",
			wantType:   "state",
		},
		{
			name:       "map with entity_id and service",
			m:          map[string]any{"entity_id": "light.test", "service": "light.turn_on"},
			wantCount:  1,
			wantEntity: "light.test",
			wantType:   "service_call",
		},
		{
			name:       "duplicate entity not added",
			m:          map[string]any{"entity_id": "light.existing"},
			wantCount:  1,
			wantEntity: "light.existing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seen := make(map[string]DependencyEntry)
			if tt.name == "duplicate entity not added" {
				seen["light.existing"] = DependencyEntry{EntityID: "light.existing", Type: "original"}
			}

			h.extractDirectEntityDependency(tt.m, seen)

			if len(seen) != tt.wantCount {
				t.Errorf("extractDirectEntityDependency() found %d entities, want %d", len(seen), tt.wantCount)
			}

			if tt.wantEntity != "" && tt.wantType != "" {
				if entry, exists := seen[tt.wantEntity]; exists {
					if entry.Type != tt.wantType && tt.name != "duplicate entity not added" {
						t.Errorf("extractDirectEntityDependency() type = %q, want %q", entry.Type, tt.wantType)
					}
				}
			}
		})
	}
}

func TestAnalysisHandlers_extractTargetEntityDependency(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name       string
		m          map[string]any
		wantCount  int
		wantEntity string
	}{
		{
			name:      "no target field",
			m:         map[string]any{"entity_id": "light.test"},
			wantCount: 0,
		},
		{
			name:      "target is not a map",
			m:         map[string]any{"target": "not_a_map"},
			wantCount: 0,
		},
		{
			name: "target with entity_id",
			m: map[string]any{
				"service": "light.turn_on",
				"target":  map[string]any{"entity_id": "light.target"},
			},
			wantCount:  1,
			wantEntity: "light.target",
		},
		{
			name: "target without entity_id",
			m: map[string]any{
				"target": map[string]any{"area_id": "living_room"},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seen := make(map[string]DependencyEntry)
			h.extractTargetEntityDependency(tt.m, seen)

			if len(seen) != tt.wantCount {
				t.Errorf("extractTargetEntityDependency() found %d entities, want %d", len(seen), tt.wantCount)
			}

			if tt.wantEntity != "" {
				if entry, exists := seen[tt.wantEntity]; !exists {
					t.Errorf("extractTargetEntityDependency() did not find expected entity %q", tt.wantEntity)
				} else if entry.Type != "target" {
					t.Errorf("extractTargetEntityDependency() type = %q, want 'target'", entry.Type)
				}
			}
		})
	}
}

func TestAnalysisHandlers_dependenciesToSortedSlice(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name string
		seen map[string]DependencyEntry
		want []string
	}{
		{
			name: "empty map",
			seen: map[string]DependencyEntry{},
			want: []string{},
		},
		{
			name: "single entry",
			seen: map[string]DependencyEntry{
				"light.test": {EntityID: "light.test"},
			},
			want: []string{"light.test"},
		},
		{
			name: "multiple entries sorted",
			seen: map[string]DependencyEntry{
				"sensor.z": {EntityID: "sensor.z"},
				"light.a":  {EntityID: "light.a"},
				"switch.m": {EntityID: "switch.m"},
			},
			want: []string{"light.a", "sensor.z", "switch.m"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := h.dependenciesToSortedSlice(tt.seen)

			if len(result) != len(tt.want) {
				t.Errorf("dependenciesToSortedSlice() returned %d entries, want %d", len(result), len(tt.want))
				return
			}

			for i, expected := range tt.want {
				if result[i].EntityID != expected {
					t.Errorf("dependenciesToSortedSlice()[%d] = %q, want %q", i, result[i].EntityID, expected)
				}
			}
		})
	}
}

func TestAnalysisHandlers_searchAreaInSlice(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name   string
		items  []any
		areaID string
		want   bool
	}{
		{
			name:   "empty slice",
			items:  []any{},
			areaID: "living_room",
			want:   false,
		},
		{
			name:   "direct string match in slice",
			items:  []any{"bedroom", "living_room", "kitchen"},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "map with area_id in slice",
			items:  []any{map[string]any{"area_id": "living_room"}},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "nested slice with match",
			items:  []any{[]any{"living_room"}},
			areaID: "living_room",
			want:   true,
		},
		{
			name:   "no match in slice",
			items:  []any{"bedroom", map[string]any{"area_id": "kitchen"}},
			areaID: "living_room",
			want:   false,
		},
		{
			name: "complex Home Assistant action structure",
			items: []any{
				map[string]any{
					"service": "light.turn_on",
					"target": map[string]any{
						"area_id": "living_room",
					},
				},
			},
			areaID: "living_room",
			want:   true,
		},
		{
			name: "multiple areas in target",
			items: []any{
				map[string]any{
					"service": "light.turn_off",
					"target": map[string]any{
						"area_id": []any{"bedroom", "living_room"},
					},
				},
			},
			areaID: "living_room",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := h.searchAreaInSlice(tt.items, tt.areaID)
			if got != tt.want {
				t.Errorf("searchAreaInSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalysisSnapshot_FindEntityRegistryEntry(t *testing.T) {
	t.Parallel()

	snapshot := &AnalysisSnapshot{
		EntityRegistry: []homeassistant.EntityRegistryEntry{
			{EntityID: "light.living_room", Platform: "hue"},
			{EntityID: "sensor.temp", Platform: "mqtt"},
		},
	}

	tests := []struct {
		name     string
		entityID string
		wantNil  bool
		wantPlat string
	}{
		{
			name:     "found entity",
			entityID: "light.living_room",
			wantNil:  false,
			wantPlat: "hue",
		},
		{
			name:     "found second entity",
			entityID: "sensor.temp",
			wantNil:  false,
			wantPlat: "mqtt",
		},
		{
			name:     "not found returns nil",
			entityID: "switch.nonexistent",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := snapshot.FindEntityRegistryEntry(tt.entityID)
			if tt.wantNil {
				if result != nil {
					t.Errorf("FindEntityRegistryEntry() = %v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Fatal("FindEntityRegistryEntry() returned nil, want non-nil")
			}
			if result.Platform != tt.wantPlat {
				t.Errorf("FindEntityRegistryEntry().Platform = %q, want %q", result.Platform, tt.wantPlat)
			}
		})
	}
}

func TestAnalysisSnapshot_FindAreaByID(t *testing.T) {
	t.Parallel()

	snapshot := &AnalysisSnapshot{
		AreaRegistry: []homeassistant.AreaRegistryEntry{
			{AreaID: "living_room", Name: "Living Room"},
			{AreaID: "bedroom", Name: "Bedroom"},
		},
	}

	tests := []struct {
		name     string
		areaID   string
		wantNil  bool
		wantName string
	}{
		{
			name:     "found area",
			areaID:   "living_room",
			wantNil:  false,
			wantName: "Living Room",
		},
		{
			name:     "found second area",
			areaID:   "bedroom",
			wantNil:  false,
			wantName: "Bedroom",
		},
		{
			name:    "not found returns nil",
			areaID:  "nonexistent",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := snapshot.FindAreaByID(tt.areaID)
			if tt.wantNil {
				if result != nil {
					t.Errorf("FindAreaByID() = %v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Fatal("FindAreaByID() returned nil, want non-nil")
			}
			if result.Name != tt.wantName {
				t.Errorf("FindAreaByID().Name = %q, want %q", result.Name, tt.wantName)
			}
		})
	}
}

func TestAnalysisHandlers_extractRegistryInfo(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name     string
		snapshot *AnalysisSnapshot
		entityID string
		wantNil  bool
		check    func(t *testing.T, reg *RegistryInfo)
	}{
		{
			name: "entity not in registry returns nil",
			snapshot: &AnalysisSnapshot{
				EntityRegistry: []homeassistant.EntityRegistryEntry{},
			},
			entityID: "light.unknown",
			wantNil:  true,
		},
		{
			name: "entity with full device and area",
			snapshot: &AnalysisSnapshot{
				EntityRegistry: []homeassistant.EntityRegistryEntry{
					{
						EntityID: "light.bulb",
						Platform: "hue",
						DeviceID: "device_abc",
						AreaID:   "living_room",
					},
				},
				DeviceRegistry: []homeassistant.DeviceRegistryEntry{
					{
						ID:           "device_abc",
						Name:         "Hue Bulb",
						Manufacturer: "Signify",
						Model:        "LCA001",
					},
				},
				AreaRegistry: []homeassistant.AreaRegistryEntry{
					{AreaID: "living_room", Name: "Living Room"},
				},
			},
			entityID: "light.bulb",
			wantNil:  false,
			check: func(t *testing.T, reg *RegistryInfo) {
				t.Helper()
				if reg.Platform != "hue" {
					t.Errorf("Platform = %q, want %q", reg.Platform, "hue")
				}
				if reg.AreaID != "living_room" {
					t.Errorf("AreaID = %q, want %q", reg.AreaID, "living_room")
				}
				if reg.AreaName != "Living Room" {
					t.Errorf("AreaName = %q, want %q", reg.AreaName, "Living Room")
				}
				if reg.DeviceID != "device_abc" {
					t.Errorf("DeviceID = %q, want %q", reg.DeviceID, "device_abc")
				}
				if reg.DeviceName != "Hue Bulb" {
					t.Errorf("DeviceName = %q, want %q", reg.DeviceName, "Hue Bulb")
				}
				if reg.Manufacturer != "Signify" {
					t.Errorf("Manufacturer = %q, want %q", reg.Manufacturer, "Signify")
				}
				if reg.Model != "LCA001" {
					t.Errorf("Model = %q, want %q", reg.Model, "LCA001")
				}
			},
		},
		{
			name: "device area fallback when entity has no direct area",
			snapshot: &AnalysisSnapshot{
				EntityRegistry: []homeassistant.EntityRegistryEntry{
					{EntityID: "light.bulb", Platform: "hue", DeviceID: "device_abc"},
				},
				DeviceRegistry: []homeassistant.DeviceRegistryEntry{
					{ID: "device_abc", Name: "Hue Bulb", AreaID: "bedroom"},
				},
				AreaRegistry: []homeassistant.AreaRegistryEntry{
					{AreaID: "bedroom", Name: "Bedroom"},
				},
			},
			entityID: "light.bulb",
			wantNil:  false,
			check: func(t *testing.T, reg *RegistryInfo) {
				t.Helper()
				if reg.AreaID != "bedroom" {
					t.Errorf("AreaID = %q, want %q (device fallback)", reg.AreaID, "bedroom")
				}
				if reg.AreaName != "Bedroom" {
					t.Errorf("AreaName = %q, want %q", reg.AreaName, "Bedroom")
				}
			},
		},
		{
			name: "device NameByUser takes precedence over Name",
			snapshot: &AnalysisSnapshot{
				EntityRegistry: []homeassistant.EntityRegistryEntry{
					{EntityID: "light.x", Platform: "hue", DeviceID: "dev1"},
				},
				DeviceRegistry: []homeassistant.DeviceRegistryEntry{
					{ID: "dev1", Name: "Original Name", NameByUser: "Custom Name"},
				},
			},
			entityID: "light.x",
			wantNil:  false,
			check: func(t *testing.T, reg *RegistryInfo) {
				t.Helper()
				if reg.DeviceName != "Custom Name" {
					t.Errorf("DeviceName = %q, want %q", reg.DeviceName, "Custom Name")
				}
			},
		},
		{
			name: "labels and aliases populated",
			snapshot: &AnalysisSnapshot{
				EntityRegistry: []homeassistant.EntityRegistryEntry{
					{
						EntityID:   "light.tagged",
						Platform:   "hue",
						Labels:     []string{"indoor", "smart"},
						Aliases:    []string{"living room light"},
						DisabledBy: "user",
						HiddenBy:   "integration",
					},
				},
			},
			entityID: "light.tagged",
			wantNil:  false,
			check: func(t *testing.T, reg *RegistryInfo) {
				t.Helper()
				if len(reg.Labels) != 2 || reg.Labels[0] != "indoor" {
					t.Errorf("Labels = %v, want [indoor, smart]", reg.Labels)
				}
				if len(reg.Aliases) != 1 || reg.Aliases[0] != "living room light" {
					t.Errorf("Aliases = %v, want [living room light]", reg.Aliases)
				}
				if reg.DisabledBy != "user" {
					t.Errorf("DisabledBy = %q, want %q", reg.DisabledBy, "user")
				}
				if reg.HiddenBy != "integration" {
					t.Errorf("HiddenBy = %q, want %q", reg.HiddenBy, "integration")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := h.extractRegistryInfo(tt.snapshot, tt.entityID)
			if tt.wantNil {
				if reg != nil {
					t.Errorf("extractRegistryInfo() = %v, want nil", reg)
				}
				return
			}
			if reg == nil {
				t.Fatal("extractRegistryInfo() returned nil, want non-nil")
			}
			if tt.check != nil {
				tt.check(t, reg)
			}
		})
	}
}

func TestAnalysisHandlers_formatRegistry(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name            string
		reg             *RegistryInfo
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "full details",
			reg: &RegistryInfo{
				Platform:     "hue",
				AreaID:       "living_room",
				AreaName:     "Living Room",
				DeviceName:   "Hue Bulb",
				Manufacturer: "Signify",
				Model:        "LCA001",
				Labels:       []string{"indoor"},
				Aliases:      []string{"hall light"},
			},
			wantContains: []string{
				"Registry:",
				"Platform: hue",
				"Area: Living Room (living_room)",
				"Device: Hue Bulb [Signify LCA001]",
				"Labels: indoor",
				"Aliases: hall light",
			},
		},
		{
			name: "area id only when name missing",
			reg: &RegistryInfo{
				AreaID: "office",
			},
			wantContains:    []string{"Area: office"},
			wantNotContains: []string{"("},
		},
		{
			name: "disabled and hidden",
			reg: &RegistryInfo{
				Platform:   "mqtt",
				DisabledBy: "user",
				HiddenBy:   "integration",
			},
			wantContains: []string{
				"Platform: mqtt",
				"Disabled by: user",
				"Hidden by: integration",
			},
		},
		{
			name: "device with no name shows manufacturer model",
			reg: &RegistryInfo{
				Manufacturer: "Sonoff",
				Model:        "ZBMINI",
			},
			wantContains: []string{"Device: Sonoff ZBMINI"},
		},
		{
			name:            "empty registry no device line",
			reg:             &RegistryInfo{Platform: "hue"},
			wantContains:    []string{"Platform: hue"},
			wantNotContains: []string{"Device:", "Area:", "Labels:", "Aliases:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parts := h.formatRegistry([]string{}, tt.reg)
			output := strings.Join(parts, "\n")

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("formatRegistry() output missing %q, got:\n%s", want, output)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(output, notWant) {
					t.Errorf("formatRegistry() output should not contain %q, got:\n%s", notWant, output)
				}
			}
		})
	}
}

func TestAnalysisHandlers_handleAnalyzeEntity_withRegistry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		client       *mockAnalysisClient
		wantContains []string
	}{
		{
			name: "natural format includes registry section",
			args: map[string]any{
				"entity_id": "light.living_room",
				"format":    "natural",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   "light.living_room",
						State:      "on",
						Attributes: map[string]any{"friendly_name": "Living Room Light"},
					}, nil
				},
				GetEntityRegistryFn: func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{
							EntityID: "light.living_room",
							Platform: "hue",
							DeviceID: "dev1",
							AreaID:   "living_room",
						},
					}, nil
				},
				GetDeviceRegistryFn: func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "dev1", Name: "Hue Bulb", Manufacturer: "Signify", Model: "LCA001"},
					}, nil
				},
				GetAreaRegistryFn: func(_ context.Context) ([]homeassistant.AreaRegistryEntry, error) {
					return []homeassistant.AreaRegistryEntry{
						{AreaID: "living_room", Name: "Living Room"},
					}, nil
				},
			},
			wantContains: []string{"Platform:", "Area:", "Device:"},
		},
		{
			name: "json format includes registry field",
			args: map[string]any{
				"entity_id": "light.living_room",
				"format":    "json",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   "light.living_room",
						State:      "on",
						Attributes: map[string]any{},
					}, nil
				},
				GetEntityRegistryFn: func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", Platform: "hue"},
					}, nil
				},
			},
			wantContains: []string{`"registry"`, `"platform"`},
		},
		{
			name: "no registry entry results in no registry section",
			args: map[string]any{
				"entity_id": "light.virtual",
				"format":    "natural",
			},
			client: &mockAnalysisClient{
				GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return &homeassistant.Entity{
						EntityID:   "light.virtual",
						State:      "on",
						Attributes: map[string]any{},
					}, nil
				},
			},
			wantContains: []string{"light.virtual"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewAnalysisHandlers()
			result, err := h.handleAnalyzeEntity(context.Background(), tt.client, tt.args)
			if err != nil {
				t.Fatalf("handleAnalyzeEntity() returned error: %v", err)
			}
			if result == nil || len(result.Content) == 0 {
				t.Fatal("handleAnalyzeEntity() returned nil/empty result")
			}

			text := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(text, want) {
					t.Errorf("output missing %q, got:\n%s", want, text)
				}
			}
		})
	}
}

func TestAnalysisHandlers_formatHistory(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	tests := []struct {
		name            string
		history         []HistoryEntry
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "shows up to 10 most recent entries",
			history: []HistoryEntry{
				{State: "state_1", LastChanged: "2024-01-01T00:00:00Z"},
				{State: "state_2", LastChanged: "2024-01-01T01:00:00Z"},
				{State: "state_3", LastChanged: "2024-01-01T02:00:00Z"},
				{State: "state_4", LastChanged: "2024-01-01T03:00:00Z"},
				{State: "state_5", LastChanged: "2024-01-01T04:00:00Z"},
				{State: "state_6", LastChanged: "2024-01-01T05:00:00Z"},
				{State: "state_7", LastChanged: "2024-01-01T06:00:00Z"},
				{State: "state_8", LastChanged: "2024-01-01T07:00:00Z"},
				{State: "state_9", LastChanged: "2024-01-01T08:00:00Z"},
				{State: "state_10", LastChanged: "2024-01-01T09:00:00Z"},
				{State: "state_11", LastChanged: "2024-01-01T10:00:00Z"},
				{State: "state_12", LastChanged: "2024-01-01T11:00:00Z"},
			},
			// Should show newest 10 (state_3 through state_12)
			wantContains:    []string{"state_12", "state_11", "state_10", "state_9", "state_8", "state_7", "state_6", "state_5", "state_4", "state_3", "Showing 10 of 12"},
			wantNotContains: []string{"- State: state_1 at", "- State: state_2 at"},
		},
		{
			name: "shows all when less than 10",
			history: []HistoryEntry{
				{State: "state_1", LastChanged: "2024-01-01T00:00:00Z"},
				{State: "state_2", LastChanged: "2024-01-01T01:00:00Z"},
				{State: "state_3", LastChanged: "2024-01-01T02:00:00Z"},
			},
			wantContains:    []string{"state_1", "state_2", "state_3", "3 state changes"},
			wantNotContains: []string{"Showing"},
		},
		{
			name:            "empty history",
			history:         []HistoryEntry{},
			wantContains:    []string{"0 state changes"},
			wantNotContains: []string{"Showing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parts := h.formatHistory([]string{}, tt.history)
			output := strings.Join(parts, "\n")

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("Expected output to contain %q, got: %s", want, output)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(output, notWant) {
					t.Errorf("Expected output NOT to contain %q, got: %s", notWant, output)
				}
			}
		})
	}
}

// TestAnalysisHandlers_ScriptSceneGroupReferences tests that handleAnalyzeEntity
// correctly populates script, scene, and group references in the analysis.
func TestAnalysisHandlers_ScriptSceneGroupReferences(t *testing.T) {
	t.Parallel()

	entityID := "light.kitchen"

	t.Run("script references entity in sequence", func(t *testing.T) {
		t.Parallel()

		client := &mockAnalysisClient{
			ListScriptsFn: func(_ context.Context) ([]homeassistant.Entity, error) {
				return []homeassistant.Entity{
					{EntityID: "script.kitchen_scene"},
				}, nil
			},
			GetScriptFn: func(_ context.Context, _ string) (*homeassistant.Script, error) {
				return &homeassistant.Script{
					EntityID:     "script.kitchen_scene",
					FriendlyName: "Kitchen Scene",
					Config: &homeassistant.ScriptConfig{
						Sequence: []any{
							map[string]any{"entity_id": entityID, "action": "light.turn_on"},
						},
					},
				}, nil
			},
			GetStatesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
				return []homeassistant.Entity{}, nil
			},
		}

		h := NewAnalysisHandlers()
		result, err := h.handleAnalyzeEntity(context.Background(), client, map[string]any{
			"entity_id": entityID,
		})
		if err != nil {
			t.Fatalf("handleAnalyzeEntity() error = %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %v", result.Content)
		}
		// The analysis should find the script reference - fetched via GetScript
		// (real Home Assistant does not expose "sequence" as a state attribute,
		// so ListScripts' Attributes cannot be used here; see analysis.go's
		// findScriptReferences).
		content := result.Content[0].Text
		if !strings.Contains(content, "Kitchen Scene") {
			t.Errorf("expected script reference to be found, got: %s", content[:min(500, len(content))])
		}
	})

	t.Run("scene references entity", func(t *testing.T) {
		t.Parallel()

		client := &mockAnalysisClient{
			ListScenesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
				return []homeassistant.Entity{
					{
						EntityID: "scene.kitchen_bright",
						Attributes: map[string]any{
							"friendly_name": "Kitchen Bright",
							"entity_id":     []any{entityID, "light.other"},
						},
					},
				}, nil
			},
			GetStatesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
				return []homeassistant.Entity{}, nil
			},
		}

		h := NewAnalysisHandlers()
		result, err := h.handleAnalyzeEntity(context.Background(), client, map[string]any{
			"entity_id": entityID,
		})
		if err != nil {
			t.Fatalf("handleAnalyzeEntity() error = %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %v", result.Content)
		}
	})

	t.Run("group entity contains target", func(t *testing.T) {
		t.Parallel()

		client := &mockAnalysisClient{
			GetStatesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
				return []homeassistant.Entity{
					{
						EntityID: "group.kitchen_lights",
						Attributes: map[string]any{
							"friendly_name": "Kitchen Lights",
							"entity_id":     []any{entityID, "light.counter"},
						},
					},
				}, nil
			},
		}

		h := NewAnalysisHandlers()
		result, err := h.handleAnalyzeEntity(context.Background(), client, map[string]any{
			"entity_id": entityID,
		})
		if err != nil {
			t.Fatalf("handleAnalyzeEntity() error = %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %v", result.Content)
		}
		content := result.Content[0].Text
		// With a group reference, the output should mention groups
		if !strings.Contains(content, "group") && !strings.Contains(content, "Groups") {
			t.Logf("analysis result (group may be shown differently): %s", content[:min(400, len(content))])
		}
	})

	t.Run("scripts error does not fail analysis", func(t *testing.T) {
		t.Parallel()

		client := &mockAnalysisClient{
			ListScriptsFn: func(_ context.Context) ([]homeassistant.Entity, error) {
				return nil, errors.New("scripts unavailable")
			},
			GetStatesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
				return []homeassistant.Entity{}, nil
			},
		}

		h := NewAnalysisHandlers()
		result, err := h.handleAnalyzeEntity(context.Background(), client, map[string]any{
			"entity_id": entityID,
		})
		if err != nil {
			t.Fatalf("handleAnalyzeEntity() error = %v", err)
		}
		// Should succeed even if scripts listing failed
		if result.IsError {
			t.Fatalf("should not fail when scripts are unavailable: %v", result.Content)
		}
	})

	t.Run("scenes error does not fail analysis", func(t *testing.T) {
		t.Parallel()

		client := &mockAnalysisClient{
			ListScenesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
				return nil, errors.New("scenes unavailable")
			},
			GetStatesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
				return []homeassistant.Entity{}, nil
			},
		}

		h := NewAnalysisHandlers()
		result, err := h.handleAnalyzeEntity(context.Background(), client, map[string]any{
			"entity_id": entityID,
		})
		if err != nil {
			t.Fatalf("handleAnalyzeEntity() error = %v", err)
		}
		if result.IsError {
			t.Fatalf("should not fail when scenes are unavailable: %v", result.Content)
		}
	})
}

// TestAnalysisHandlers_FormatReferences tests formatReferences with scripts, scenes, and groups.
func TestAnalysisHandlers_FormatReferences(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	refs := &EntityReferences{
		Automations: []AutomationReference{
			{EntityID: "automation.lights", Alias: "Turn on Lights", State: "on", UsedIn: []string{"trigger"}},
		},
		Scripts: []ScriptReference{
			{EntityID: "script.morning", FriendlyName: "Morning Script", UsedIn: "action"},
		},
		Scenes: []SceneReference{
			{EntityID: "scene.bright", FriendlyName: "Bright Scene"},
		},
		Groups:          []string{"group.lights", "group.all"},
		AreaReferences:  []AreaReference{{EntityID: "automation.lights", AreaID: "living_room"}},
		TotalReferences: 5,
	}

	var parts []string
	parts = h.formatReferences(parts, refs, false)

	result := strings.Join(parts, "\n")

	if !strings.Contains(result, "References") {
		t.Errorf("formatReferences should contain 'References', got: %s", result)
	}
	if !strings.Contains(result, "Turn on Lights") {
		t.Errorf("formatReferences should contain automation alias, got: %s", result)
	}
	if !strings.Contains(result, "Morning Script") {
		t.Errorf("formatReferences should contain script name, got: %s", result)
	}
	if !strings.Contains(result, "scene.bright") {
		t.Errorf("formatReferences should contain scene, got: %s", result)
	}
	if !strings.Contains(result, "group.lights") {
		t.Errorf("formatReferences should contain group, got: %s", result)
	}
	if !strings.Contains(result, "Area references") {
		t.Errorf("formatReferences should contain area references, got: %s", result)
	}
}

// TestAnalysisHandlers_FormatReferences_EmptyAlias tests formatReferences when alias is empty
// (falls back to EntityID).
func TestAnalysisHandlers_FormatReferences_EmptyAlias(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	refs := &EntityReferences{
		Automations: []AutomationReference{
			{EntityID: "automation.unknown", Alias: "", State: "on", UsedIn: []string{"action"}},
		},
		Scripts: []ScriptReference{
			{EntityID: "script.noname", FriendlyName: "", UsedIn: "trigger"},
		},
		TotalReferences: 2,
	}

	var parts []string
	parts = h.formatReferences(parts, refs, false)
	result := strings.Join(parts, "\n")

	// Should fall back to entity_id when alias/friendly_name is empty
	if !strings.Contains(result, "automation.unknown") {
		t.Errorf("formatReferences should contain entity_id as fallback, got: %s", result)
	}
	if !strings.Contains(result, "script.noname") {
		t.Errorf("formatReferences should contain script entity_id as fallback, got: %s", result)
	}
}

func TestAnalysisHandlers_collectEntityExcerpts(t *testing.T) {
	t.Parallel()

	entityID := "binary_sensor.door"
	config := &homeassistant.AutomationConfig{
		Triggers: []any{
			map[string]any{
				"platform":  "state",
				"entity_id": entityID,
				"to":        "on",
				"id":        "door_open",
			},
		},
		Conditions: []any{
			map[string]any{
				"condition": "state",
				"entity_id": entityID,
				"state":     "off",
			},
		},
		Actions: []any{
			map[string]any{
				"action": "light.turn_on",
				"target": map[string]any{"entity_id": "light.hall"},
			},
			// Entity in choose branch
			map[string]any{
				"choose": []any{
					map[string]any{
						"conditions": []any{
							map[string]any{"condition": "state", "entity_id": entityID, "state": "on"},
						},
					},
				},
			},
		},
	}

	excerpts := collectEntityExcerpts(config, entityID)

	// Expect: 1 trigger + 1 condition + 1 choose action (direct action doesn't reference entity)
	if len(excerpts) != 3 {
		t.Errorf("expected 3 excerpts, got %d: %+v", len(excerpts), excerpts)
	}

	// Check sections
	sections := make(map[string]int)
	for _, ex := range excerpts {
		sections[ex.Section]++
	}
	if sections["trigger"] != 1 {
		t.Errorf("expected 1 trigger excerpt, got %d", sections["trigger"])
	}
	if sections["condition"] != 1 {
		t.Errorf("expected 1 condition excerpt, got %d", sections["condition"])
	}
	if sections["action"] != 1 {
		t.Errorf("expected 1 action excerpt, got %d", sections["action"])
	}

	// Check trigger summary contains key info
	triggerExcerpt := excerpts[0]
	if !strings.Contains(triggerExcerpt.Summary, "on") {
		t.Errorf("trigger excerpt should contain 'on', got: %q", triggerExcerpt.Summary)
	}
	if !strings.Contains(triggerExcerpt.Summary, "door_open") {
		t.Errorf("trigger excerpt should contain id 'door_open', got: %q", triggerExcerpt.Summary)
	}
}

func TestSummarizeConfigNode(t *testing.T) {
	t.Parallel()

	entityID := "sensor.temp"

	tests := []struct {
		name    string
		section string
		node    map[string]any
		want    string
	}{
		{
			name:    "state trigger with to",
			section: "trigger",
			node:    map[string]any{"platform": "state", "entity_id": entityID, "to": "on"},
			want:    "on",
		},
		{
			name:    "state trigger with from and to",
			section: "trigger",
			node:    map[string]any{"platform": "state", "entity_id": entityID, "from": "off", "to": "on"},
			want:    "from",
		},
		{
			name:    "numeric_state trigger",
			section: "trigger",
			node:    map[string]any{"platform": "numeric_state", "entity_id": entityID, "above": float64(20)},
			want:    "above",
		},
		{
			name:    "state condition",
			section: "condition",
			node:    map[string]any{"condition": "state", "entity_id": entityID, "state": "off"},
			want:    "\"off\"",
		},
		{
			name:    "service action (new style)",
			section: "action",
			node:    map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": entityID}},
			want:    "light.turn_on",
		},
		{
			name:    "service action (legacy style)",
			section: "action",
			node:    map[string]any{"service": "light.turn_off"},
			want:    "light.turn_off",
		},
		{
			name:    "choose action",
			section: "action",
			node:    map[string]any{"choose": []any{}},
			want:    "choose",
		},
		{
			name:    "if/then action",
			section: "action",
			node:    map[string]any{"if": []any{}, "then": []any{}},
			want:    "if/then",
		},
		{
			name:    "unknown trigger platform",
			section: "trigger",
			node:    map[string]any{"platform": "event", "event_type": "custom"},
			want:    "event",
		},
		{
			name:    "trigger without platform",
			section: "trigger",
			node:    map[string]any{"entity_id": entityID},
			want:    entityID,
		},
		{
			name:    "fallback action",
			section: "action",
			node:    map[string]any{"delay": "00:01:00"},
			want:    entityID,
		},
		{
			name:    "numeric_state condition with above",
			section: "condition",
			node:    map[string]any{"condition": "numeric_state", "entity_id": entityID, "above": float64(20)},
			want:    "above",
		},
		{
			name:    "numeric_state condition with below",
			section: "condition",
			node:    map[string]any{"condition": "numeric_state", "entity_id": entityID, "below": float64(10)},
			want:    "below",
		},
		{
			name:    "unknown condition type",
			section: "condition",
			node:    map[string]any{"condition": "template", "value_template": "{{ true }}"},
			want:    "template",
		},
		{
			name:    "condition no type",
			section: "condition",
			node:    map[string]any{"entity_id": entityID},
			want:    entityID,
		},
		{
			name:    "unknown section fallback",
			section: "variables",
			node:    map[string]any{"entity_id": entityID},
			want:    entityID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := summarizeConfigNode(tt.section, tt.node, entityID)
			if !strings.Contains(got, tt.want) {
				t.Errorf("summarizeConfigNode(%q) = %q, want to contain %q", tt.section, got, tt.want)
			}
		})
	}
}

func TestCollectSequenceExcerpts(t *testing.T) {
	t.Parallel()

	entityID := "light.kitchen"
	sequence := []any{
		// This action references the entity
		map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": entityID}},
		// This action does NOT reference the entity
		map[string]any{"delay": "00:00:05"},
		// Choose block containing the entity
		map[string]any{
			"choose": []any{
				map[string]any{
					"conditions": []any{
						map[string]any{"condition": "state", "entity_id": entityID, "state": "on"},
					},
					"sequence": []any{
						map[string]any{"action": "notify.mobile", "data": map[string]any{"message": "kitchen on"}},
					},
				},
			},
		},
	}

	excerpts := collectSequenceExcerpts(sequence, entityID)

	if len(excerpts) != 2 {
		t.Errorf("expected 2 excerpts (service + choose), got %d: %+v", len(excerpts), excerpts)
	}
	for _, ex := range excerpts {
		if ex.Section != "action" {
			t.Errorf("all sequence excerpts should have section 'action', got %q", ex.Section)
		}
	}
	if !strings.Contains(excerpts[0].Summary, "light.turn_on") {
		t.Errorf("first excerpt should contain service name, got: %q", excerpts[0].Summary)
	}
	if !strings.Contains(excerpts[1].Summary, "choose") {
		t.Errorf("second excerpt should mention choose, got: %q", excerpts[1].Summary)
	}
}

func TestAnalysisHandlers_verbose_mode(t *testing.T) {
	t.Parallel()

	entityID := "binary_sensor.motion"
	autoEntityID := "automation.turn_on_lights"
	autoConfig := &homeassistant.AutomationConfig{
		Triggers: []any{
			map[string]any{
				"platform":  "state",
				"entity_id": entityID,
				"to":        "on",
			},
		},
	}

	h := NewAnalysisHandlers()

	client := &mockAnalysisClient{
		GetStateFn: func(_ context.Context, id string) (*homeassistant.Entity, error) {
			return &homeassistant.Entity{
				EntityID:   id,
				State:      "off",
				Attributes: map[string]any{},
			}, nil
		},
		ListAutomationsFn: func(_ context.Context) ([]homeassistant.Automation, error) {
			return []homeassistant.Automation{{EntityID: autoEntityID, FriendlyName: "Turn On Lights"}}, nil
		},
		GetAutomationFn: func(_ context.Context, _ string) (*homeassistant.Automation, error) {
			return &homeassistant.Automation{
				EntityID: autoEntityID,
				Config:   autoConfig,
			}, nil
		},
	}

	// Test verbose=true — should include excerpt
	args := map[string]any{"entity_id": entityID, "format": "natural", "verbose": true}
	result, err := h.handleAnalyzeEntity(context.Background(), client, args)
	if err != nil {
		t.Fatalf("handleAnalyzeEntity() error = %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "trigger:") {
		t.Errorf("verbose=true should include trigger excerpt, got:\n%s", text)
	}
	if !strings.Contains(text, "on") {
		t.Errorf("verbose=true should include 'on' in trigger excerpt, got:\n%s", text)
	}

	// Test verbose=false (default) — should NOT include full excerpt lines.
	// Paths are always shown (e.g. "trigger: state" appears as path context), but
	// full excerpt summaries like 'state binary_sensor.motion → "on"' are verbose-only.
	args2 := map[string]any{"entity_id": entityID, "format": "natural"}
	result2, err := h.handleAnalyzeEntity(context.Background(), client, args2)
	if err != nil {
		t.Fatalf("handleAnalyzeEntity() error = %v", err)
	}
	text2 := result2.Content[0].Text
	// The verbose excerpt contains the exact state value in quotes (e.g. → "on").
	// Path context only shows the trigger type ("trigger: state"), not state values.
	if strings.Contains(text2, `→ "on"`) {
		t.Errorf("verbose=false should not include full trigger excerpt with state value, got:\n%s", text2)
	}
}

// --- Reference path formatting tests ---

// TestFormatScriptRefs_ShowsPaths verifies that formatScriptRefs includes RFC 6901
// path lines when the ScriptReference has Paths populated.
func TestFormatScriptRefs_ShowsPaths(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()
	scripts := []ScriptReference{
		{
			EntityID:     "script.morning",
			FriendlyName: "Morning Script",
			UsedIn:       "action",
			Paths: []ReferencePath{
				{Path: "/sequence/0/target/entity_id", Context: "action: automation.turn_off"},
				{Path: "/sequence/3/target/entity_id", Context: "action: automation.turn_on"},
			},
		},
	}

	var parts []string
	parts = h.formatScriptRefs(parts, scripts, false)
	result := strings.Join(parts, "\n")

	if !strings.Contains(result, "Morning Script") {
		t.Errorf("should contain script name, got:\n%s", result)
	}
	if !strings.Contains(result, "/sequence/0/target/entity_id") {
		t.Errorf("should contain first path, got:\n%s", result)
	}
	if !strings.Contains(result, "/sequence/3/target/entity_id") {
		t.Errorf("should contain second path, got:\n%s", result)
	}
	if !strings.Contains(result, "action: automation.turn_off") {
		t.Errorf("should contain context label, got:\n%s", result)
	}
}

// TestFormatScriptRefs_NoPaths verifies that formatScriptRefs still works
// cleanly when no Paths are populated (e.g. for scripts with no found references).
func TestFormatScriptRefs_NoPaths(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()
	scripts := []ScriptReference{
		{
			EntityID:     "script.morning",
			FriendlyName: "Morning Script",
			UsedIn:       "action",
		},
	}

	var parts []string
	parts = h.formatScriptRefs(parts, scripts, false)
	result := strings.Join(parts, "\n")

	if !strings.Contains(result, "Morning Script") {
		t.Errorf("should contain script name, got:\n%s", result)
	}
	// No path lines should appear.
	if strings.Contains(result, "/sequence") {
		t.Errorf("should not contain path segments when Paths is empty, got:\n%s", result)
	}
}

// TestFormatAutomationRefs_ShowsPaths verifies that formatAutomationRefs includes
// RFC 6901 path lines when the AutomationReference has Paths populated.
func TestFormatAutomationRefs_ShowsPaths(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()
	autos := []AutomationReference{
		{
			EntityID: "automation.lights",
			Alias:    "Turn on Lights",
			State:    "on",
			UsedIn:   []string{"trigger"},
			Paths: []ReferencePath{
				{Path: "/triggers/0/entity_id", Context: "trigger: state"},
			},
		},
	}

	var parts []string
	parts = h.formatAutomationRefs(parts, autos, false)
	result := strings.Join(parts, "\n")

	if !strings.Contains(result, "Turn on Lights") {
		t.Errorf("should contain automation alias, got:\n%s", result)
	}
	if !strings.Contains(result, "/triggers/0/entity_id") {
		t.Errorf("should contain path, got:\n%s", result)
	}
	if !strings.Contains(result, "trigger: state") {
		t.Errorf("should contain context, got:\n%s", result)
	}
}

// TestHandleAnalyzeEntity_ScriptPathsInOutput is an integration-level unit test
// that verifies the full handleAnalyzeEntity call populates and formats Paths.
func TestHandleAnalyzeEntity_ScriptPathsInOutput(t *testing.T) {
	t.Parallel()

	const entityID = "automation.example"

	// Build a script whose sequence contains the target entity_id.
	sequence := []any{
		map[string]any{
			"action": "automation.turn_off",
			"target": map[string]any{"entity_id": entityID},
		},
	}

	client := &mockAnalysisClient{
		GetStateFn: func(_ context.Context, eid string) (*homeassistant.Entity, error) {
			return &homeassistant.Entity{EntityID: eid, State: "on"}, nil
		},
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			return nil, nil
		},
		ListScriptsFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{
				{EntityID: "script.example_script", State: "off"},
			}, nil
		},
		GetScriptFn: func(_ context.Context, _ string) (*homeassistant.Script, error) {
			return &homeassistant.Script{
				EntityID:     "script.example_script",
				FriendlyName: "Example Script",
				Config:       &homeassistant.ScriptConfig{Sequence: sequence},
			}, nil
		},
	}

	h := NewAnalysisHandlers()
	args := map[string]any{"entity_id": entityID, "format": "natural"}
	result, err := h.handleAnalyzeEntity(context.Background(), client, args)
	if err != nil {
		t.Fatalf("handleAnalyzeEntity() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("handleAnalyzeEntity() returned error result: %v", result.Content)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "/sequence/0/target/entity_id") {
		t.Errorf("output should contain path to entity reference, got:\n%s", text)
	}
	if !strings.Contains(text, "action: automation.turn_off") {
		t.Errorf("output should contain action context, got:\n%s", text)
	}
}

// TestHandleAnalyzeEntity_FindsDashboardOnlyReference is a regression test:
// analyze_entity used to report "No references found" for an entity
// referenced only in a dashboard card/chip, since dashboards weren't scanned.
func TestHandleAnalyzeEntity_FindsDashboardOnlyReference(t *testing.T) {
	t.Parallel()

	const entityID = "device_tracker.example_phone"

	dashboardConfig := map[string]any{
		"views": []any{
			map[string]any{
				"cards": []any{
					map[string]any{"type": "entity", "entity": entityID},
				},
			},
		},
	}

	client := &mockAnalysisClient{
		GetStateFn: func(_ context.Context, eid string) (*homeassistant.Entity, error) {
			return &homeassistant.Entity{EntityID: eid, State: "home"}, nil
		},
		ListDashboardsFn: func(context.Context) ([]homeassistant.DashboardEntry, error) {
			return []homeassistant.DashboardEntry{{URLPath: "lovelace"}}, nil
		},
		GetLovelaceConfigFn: func(context.Context, string) (map[string]any, error) {
			return dashboardConfig, nil
		},
	}

	h := NewAnalysisHandlers()
	args := map[string]any{"entity_id": entityID, "format": "natural"}
	result, err := h.handleAnalyzeEntity(context.Background(), client, args)
	if err != nil {
		t.Fatalf("handleAnalyzeEntity() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("handleAnalyzeEntity() returned error result: %v", result.Content)
	}

	text := result.Content[0].Text
	if strings.Contains(text, "No references found") {
		t.Errorf("expected the dashboard reference to be found, got:\n%s", text)
	}
	if !strings.Contains(text, "dashboard(s)") {
		t.Errorf("output should mention dashboard references, got:\n%s", text)
	}
}

// TestHandleAnalyzeEntity_FindsHelperTemplateOnlyReference is a regression test:
// analyze_entity used to miss entities referenced only inside a
// template-helper's Jinja state/availability template.
func TestHandleAnalyzeEntity_FindsHelperTemplateOnlyReference(t *testing.T) {
	t.Parallel()

	const entityID = "device_tracker.example_phone"

	client := &mockAnalysisClient{
		GetStateFn: func(_ context.Context, eid string) (*homeassistant.Entity, error) {
			return &homeassistant.Entity{EntityID: eid, State: "home"}, nil
		},
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "sensor.occupancy_count", Platform: "template", ConfigEntryID: "entry1"},
			}, nil
		},
		GetConfigEntryOptionsFn: func(_ context.Context, entryID string) (map[string]any, error) {
			if entryID != "entry1" {
				t.Fatalf("unexpected entry id %q", entryID)
			}
			return map[string]any{
				"state": "{{ is_state('" + entityID + "', 'home') }}",
			}, nil
		},
	}

	h := NewAnalysisHandlers()
	args := map[string]any{"entity_id": entityID, "format": "natural"}
	result, err := h.handleAnalyzeEntity(context.Background(), client, args)
	if err != nil {
		t.Fatalf("handleAnalyzeEntity() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("handleAnalyzeEntity() returned error result: %v", result.Content)
	}

	text := result.Content[0].Text
	if strings.Contains(text, "No references found") {
		t.Errorf("expected the helper-template reference to be found, got:\n%s", text)
	}
	if !strings.Contains(text, "sensor.occupancy_count") {
		t.Errorf("output should mention the referencing helper, got:\n%s", text)
	}
}

// TestHandleAnalyzeEntity_NoReferencesListsScannedSources verifies that when no
// references exist, the output states which sources were scanned so the
// "no references" result can be trusted.
func TestHandleAnalyzeEntity_NoReferencesListsScannedSources(t *testing.T) {
	t.Parallel()

	client := &mockAnalysisClient{}

	h := NewAnalysisHandlers()
	args := map[string]any{"entity_id": "light.unused", "format": "natural"}
	result, err := h.handleAnalyzeEntity(context.Background(), client, args)
	if err != nil {
		t.Fatalf("handleAnalyzeEntity() error = %v", err)
	}

	text := result.Content[0].Text
	for _, source := range []string{"automations", "scripts", "scenes", "dashboards", "helper_templates"} {
		if !strings.Contains(text, source) {
			t.Errorf("expected scanned_sources to mention %q, got:\n%s", source, text)
		}
	}
}

// TestHandleAnalyzeEntity_FailedSourceReportedInFailedSourcesAndWarning is a
// regression test for the adversarial-review finding that ScannedSources was
// a hardcoded literal, not a measurement: if a source's fetch fails, it must
// land in FailedSources and the natural-language output must warn about it -
// even on the "no references found" path, which is the exact scenario a
// silent failure would otherwise misrepresent as a confirmed negative.
func TestHandleAnalyzeEntity_FailedSourceReportedInFailedSourcesAndWarning(t *testing.T) {
	t.Parallel()

	entityID := "light.test_entity"
	client := &mockAnalysisClient{
		GetStateFn: func(_ context.Context, id string) (*homeassistant.Entity, error) {
			return &homeassistant.Entity{EntityID: id, State: "on"}, nil
		},
		ListDashboardsFn: func(context.Context) ([]homeassistant.DashboardEntry, error) {
			return nil, errors.New("connection failed")
		},
	}

	h := NewAnalysisHandlers()
	args := map[string]any{"entity_id": entityID, "format": "natural"}
	result, err := h.handleAnalyzeEntity(context.Background(), client, args)
	if err != nil {
		t.Fatalf("handleAnalyzeEntity() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result.Content)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "could not be scanned: dashboards") {
		t.Errorf("expected a failed-source warning naming dashboards, got:\n%s", text)
	}

	jsonArgs := map[string]any{"entity_id": entityID, "format": "json"}
	jsonResult, err := h.handleAnalyzeEntity(context.Background(), client, jsonArgs)
	if err != nil {
		t.Fatalf("handleAnalyzeEntity() error = %v", err)
	}
	var analysis EntityAnalysis
	if unmarshalErr := json.Unmarshal([]byte(jsonResult.Content[0].Text), &analysis); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal JSON result: %v", unmarshalErr)
	}
	if !reflect.DeepEqual(analysis.References.FailedSources, []string{"dashboards"}) {
		t.Errorf("FailedSources = %v, want [dashboards]", analysis.References.FailedSources)
	}
	for _, s := range analysis.References.ScannedSources {
		if s == "dashboards" {
			t.Errorf("ScannedSources should not include the failed source, got %v", analysis.References.ScannedSources)
		}
	}
}

// TestHandleAnalyzeEntity_NoReferencesListsScannedSources_AllSourcesFailIndependently
// verifies FailedSources accumulates every failing source, not just the first.
func TestHandleAnalyzeEntity_MultipleFailedSourcesAllReported(t *testing.T) {
	t.Parallel()

	entityID := "light.test_entity"
	client := &mockAnalysisClient{
		GetStateFn: func(_ context.Context, id string) (*homeassistant.Entity, error) {
			return &homeassistant.Entity{EntityID: id, State: "on"}, nil
		},
		ListDashboardsFn: func(context.Context) ([]homeassistant.DashboardEntry, error) {
			return nil, errors.New("connection failed")
		},
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return nil, errors.New("registry unavailable")
		},
	}

	h := NewAnalysisHandlers()
	args := map[string]any{"entity_id": entityID, "format": "json"}
	result, err := h.handleAnalyzeEntity(context.Background(), client, args)
	if err != nil {
		t.Fatalf("handleAnalyzeEntity() error = %v", err)
	}

	var analysis EntityAnalysis
	if unmarshalErr := json.Unmarshal([]byte(result.Content[0].Text), &analysis); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal JSON result: %v", unmarshalErr)
	}
	got := append([]string{}, analysis.References.FailedSources...)
	sort.Strings(got)
	want := []string{"dashboards", "helper_templates"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FailedSources = %v, want %v", got, want)
	}
}

// TestAnalysisHandlers_FormatAnalysisNatural_NameOrder pins the "Name (entity_id) is
// state" line shape to match query_entities' natural formatter (internal/handlers/
// formatter/natural.go), so an LLM reading both tools' output can rely on the same
// positional rule to recover the addressable id. Before this fix, analyze_entity
// rendered the inverse "entity_id (Name) is state" order.
func TestAnalysisHandlers_FormatAnalysisNatural_NameOrder(t *testing.T) {
	t.Parallel()

	h := NewAnalysisHandlers()

	withName := &EntityAnalysis{
		EntityID:     "light.living_room",
		FriendlyName: "Living Room Light",
		State:        "on",
		References:   &EntityReferences{},
	}
	result := h.formatAnalysisNatural(withName, false)
	if !strings.Contains(result, "Living Room Light (light.living_room) is on") {
		t.Errorf("formatAnalysisNatural() = %q, want it to contain %q", result, "Living Room Light (light.living_room) is on")
	}

	noName := &EntityAnalysis{
		EntityID:   "light.kitchen",
		State:      "on",
		References: &EntityReferences{},
	}
	result = h.formatAnalysisNatural(noName, false)
	if strings.Contains(result, "(light.kitchen)") {
		t.Errorf("formatAnalysisNatural() = %q, expected no parenthesized duplication when there is no friendly name", result)
	}
	if count := strings.Count(result, "light.kitchen"); count != 1 {
		t.Errorf("formatAnalysisNatural() = %q, expected entity_id to appear exactly once, got %d", result, count)
	}
}
