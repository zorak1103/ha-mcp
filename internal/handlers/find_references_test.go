package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestFindReferencesHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewFindReferencesHandlers()
	if h == nil {
		t.Fatal("NewFindReferencesHandlers() returned nil")
	}
}

func TestFindReferencesHandlers_Schema(t *testing.T) {
	t.Parallel()

	h := NewFindReferencesHandlers()
	tool := h.findReferencesTool()

	if tool.Name != "find_references" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "find_references")
	}
	if _, ok := tool.InputSchema.Properties["search"]; !ok {
		t.Error("expected 'search' property in schema")
	}
	if len(tool.InputSchema.Required) != 1 || tool.InputSchema.Required[0] != "search" {
		t.Errorf("Required = %v, want [search]", tool.InputSchema.Required)
	}
}

func TestHandleFindReferences(t *testing.T) {
	t.Parallel()

	h := NewFindReferencesHandlers()

	automationConfig := &homeassistant.AutomationConfig{
		Actions: []any{
			map[string]any{
				"action": "light.turn_on",
				"target": map[string]any{"entity_id": "device_tracker.example_phone"},
			},
		},
	}

	scriptEntity := homeassistant.Entity{
		EntityID: "script.example",
	}
	scriptConfig := &homeassistant.Script{
		EntityID: "script.example",
		Config: &homeassistant.ScriptConfig{
			Sequence: []any{
				map[string]any{"action": "notify.mobile_app", "target": map[string]any{"entity_id": "device_tracker.example_phone"}},
			},
		},
	}

	sceneEntity := homeassistant.Entity{
		EntityID:   "scene.example",
		Attributes: map[string]any{"entity_id": []any{"device_tracker.example_phone", "light.desk"}},
	}

	dashboardConfig := map[string]any{
		"views": []any{
			map[string]any{
				"cards": []any{
					map[string]any{"type": "entity", "entity": "device_tracker.example_phone"},
				},
			},
		},
	}

	tests := []handlerTestCase{
		{
			name:         "missing search",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"search is required"},
		},
		{
			name: "substring match across all types by default",
			args: map[string]any{
				"search": "device_tracker.example_phone",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListAutomationsFn = func(context.Context) ([]homeassistant.Automation, error) {
					return []homeassistant.Automation{{EntityID: "automation.example"}}, nil
				}
				m.GetAutomationFn = func(context.Context, string) (*homeassistant.Automation, error) {
					return &homeassistant.Automation{EntityID: "automation.example", Config: automationConfig}, nil
				}
				m.ListScriptsFn = func(context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{scriptEntity}, nil
				}
				m.GetScriptFn = func(context.Context, string) (*homeassistant.Script, error) {
					return scriptConfig, nil
				}
				m.ListScenesFn = func(context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{sceneEntity}, nil
				}
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, nil
				}
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return dashboardConfig, nil
				}
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return nil, nil
				}
			},
			wantError: false,
			wantContains: []string{
				"Found 4 match(es)",
				"automation.example",
				"script.example",
				"scene.example",
			},
		},
		{
			name: "types filter restricts scan to dashboards only",
			args: map[string]any{
				"search": "device_tracker.example_phone",
				"types":  []any{"dashboard"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListAutomationsFn = func(context.Context) ([]homeassistant.Automation, error) {
					t.Fatal("automations should not be scanned when types=[dashboard]")
					return nil, nil
				}
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, nil
				}
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return dashboardConfig, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 1 match(es)"},
		},
		{
			name: "exact match mode does not match template substrings",
			args: map[string]any{
				"search":     "device_tracker.example_phone",
				"match_mode": "exact",
				"types":      []any{"dashboard"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, nil
				}
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return map[string]any{
						"views": []any{
							map[string]any{
								"cards": []any{
									map[string]any{
										"type":       "tile",
										"icon_color": "{{ presence_icon_formatter('device_tracker.example_phone') }}",
									},
								},
							},
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No references found"},
		},
		{
			name: "no matches",
			args: map[string]any{
				"search": "device_tracker.nonexistent",
				"types":  []any{"dashboard"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, nil
				}
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return dashboardConfig, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No references found"},
		},
		{
			name: "json format",
			args: map[string]any{
				"search": "device_tracker.example_phone",
				"types":  []any{"dashboard"},
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, nil
				}
				m.GetLovelaceConfigFn = func(context.Context, string) (map[string]any, error) {
					return dashboardConfig, nil
				}
			},
			wantError:    false,
			wantContains: []string{`"type": "dashboard"`, `"object_id"`},
		},
	}

	runHandlerTestCases(t, tests, h.handleFindReferences)
}

func TestSearchMatchFunc(t *testing.T) {
	t.Parallel()

	substring := searchMatchFunc("substring", "foo")
	if !substring("xfoox") {
		t.Error("substring mode should match a substring")
	}

	exact := searchMatchFunc("exact", "foo")
	if exact("xfoox") {
		t.Error("exact mode should not match a substring")
	}
	if !exact("foo") {
		t.Error("exact mode should match an identical value")
	}

	defaultMode := searchMatchFunc("", "foo")
	if !defaultMode("xfoox") {
		t.Error("empty match_mode should default to substring")
	}
}

func TestFindReferencesRequestedTypes(t *testing.T) {
	t.Parallel()

	allTypes := findReferencesRequestedTypes(map[string]any{})
	if len(allTypes) != len(findReferencesTypes) {
		t.Errorf("expected all %d types by default, got %d", len(findReferencesTypes), len(allTypes))
	}

	filtered := findReferencesRequestedTypes(map[string]any{"types": []any{"dashboard", "script"}})
	if len(filtered) != 2 || !filtered["dashboard"] || !filtered["script"] {
		t.Errorf("expected {dashboard, script}, got %v", filtered)
	}
}

func TestFormatFindReferencesNatural_Empty(t *testing.T) {
	t.Parallel()

	got := formatFindReferencesNatural("sensor.x", nil, nil)
	if !strings.Contains(got, "No references found") {
		t.Errorf("expected no-references message, got %q", got)
	}
}

func TestHandleFindReferences_PartialScanFailureReportedInBothFormats(t *testing.T) {
	t.Parallel()

	h := NewFindReferencesHandlers()

	tests := []handlerTestCase{
		{
			name: "natural format shows failed source warning",
			args: map[string]any{
				"search": "device_tracker.example_phone",
				"types":  []any{"script", "dashboard"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListScriptsFn = func(context.Context) ([]homeassistant.Entity, error) {
					return nil, nil
				}
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, errors.New("connection failed")
				}
			},
			wantError: false,
			wantContains: []string{
				"No references found",
				"1 source(s) could not be scanned: dashboard",
			},
		},
		{
			name: "json format includes scanned_sources and failed_sources",
			args: map[string]any{
				"search": "device_tracker.example_phone",
				"types":  []any{"script", "dashboard"},
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.ListScriptsFn = func(context.Context) ([]homeassistant.Entity, error) {
					return nil, nil
				}
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return nil, errors.New("connection failed")
				}
			},
			wantError: false,
			wantContains: []string{
				`"scanned_sources"`,
				`"script"`,
				`"failed_sources"`,
				`"dashboard"`,
			},
		},
	}

	runHandlerTestCases(t, tests, h.handleFindReferences)
}
