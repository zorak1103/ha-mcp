package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockAnalysisClient implements homeassistant.Client for analysis tests.
type mockAnalysisClient struct {
	homeassistant.Client
	GetStateFn          func(ctx context.Context, entityID string) (*homeassistant.Entity, error)
	ListAutomationsFn   func(ctx context.Context) ([]homeassistant.Automation, error)
	GetAutomationFn     func(ctx context.Context, automationID string) (*homeassistant.Automation, error)
	ListScriptsFn       func(ctx context.Context) ([]homeassistant.Entity, error)
	ListScenesFn        func(ctx context.Context) ([]homeassistant.Entity, error)
	GetStatesFn         func(ctx context.Context) ([]homeassistant.Entity, error)
	GetEntityRegistryFn func(ctx context.Context) ([]homeassistant.EntityRegistryEntry, error)
	GetDeviceRegistryFn func(ctx context.Context) ([]homeassistant.DeviceRegistryEntry, error)
	GetHistoryFn        func(ctx context.Context, entityID string, start, end time.Time) ([][]homeassistant.HistoryEntry, error)
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

func (m *mockAnalysisClient) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]homeassistant.HistoryEntry, error) {
	if m.GetHistoryFn != nil {
		return m.GetHistoryFn(ctx, entityID, start, end)
	}
	return [][]homeassistant.HistoryEntry{}, nil
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
			name: "success",
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
			name: "success with history",
			args: map[string]any{
				"entity_id":       "light.living_room",
				"include_history": true,
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
			name:         "missing entity_id",
			args:         map[string]any{},
			client:       &mockAnalysisClient{},
			wantContains: "entity_id is required",
			wantError:    true,
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
			},
			client:       &mockAnalysisClient{},
			wantContains: "entity_id is required",
			wantError:    true,
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "light.living_room",
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
			if !contains(text, tt.wantContains) {
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
			name: "success automation",
			args: map[string]any{
				"entity_id": "automation.test_automation",
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
			name: "success script",
			args: map[string]any{
				"entity_id": "script.test_script",
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
			name:         "missing entity_id",
			args:         map[string]any{},
			client:       &mockAnalysisClient{},
			wantContains: "entity_id is required",
			wantError:    true,
		},
		{
			name: "invalid entity_id",
			args: map[string]any{
				"entity_id": "light.living_room",
			},
			client:       &mockAnalysisClient{},
			wantContains: "must be an automation or script",
			wantError:    true,
		},
		{
			name: "automation not found",
			args: map[string]any{
				"entity_id": "automation.nonexistent",
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
			if !contains(text, tt.wantContains) {
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
