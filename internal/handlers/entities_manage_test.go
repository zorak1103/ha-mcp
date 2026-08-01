package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestEntityManageHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	handler := NewEntityManageHandlers()

	handler.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "manage_entity" {
		t.Errorf("expected tool 'manage_entity', got '%s'", tools[0].Name)
	}
}

func TestManageEntity_Schema(t *testing.T) {
	t.Parallel()

	handler := NewEntityManageHandlers()
	tool := handler.manageEntityTool()

	if tool.Name != "manage_entity" {
		t.Errorf("expected tool name 'manage_entity', got '%s'", tool.Name)
	}

	// Verify action enum has 3 values: get, update, delete
	actionProp, ok := tool.InputSchema.Properties["action"]
	if !ok {
		t.Fatal("expected 'action' property in schema")
	}
	if len(actionProp.Enum) != 3 {
		t.Errorf("expected 3 action enum values, got %d", len(actionProp.Enum))
	}

	expectedActions := []string{"get", "update", "delete"}
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

	// Verify Required field contains "action"
	if len(tool.InputSchema.Required) == 0 {
		t.Error("expected Required field to contain action")
	}
	foundAction := false
	for _, req := range tool.InputSchema.Required {
		if req == "action" {
			foundAction = true
			break
		}
	}
	if !foundAction {
		t.Error("expected 'action' in Required field")
	}

	// Verify disabled_by enum has 2 values
	disabledByProp, ok := tool.InputSchema.Properties["disabled_by"]
	if !ok {
		t.Fatal("expected 'disabled_by' property in schema")
	}
	if len(disabledByProp.Enum) != 2 {
		t.Errorf("expected 2 disabled_by enum values, got %d", len(disabledByProp.Enum))
	}

	// Verify hidden_by enum has 2 values
	hiddenByProp, ok := tool.InputSchema.Properties["hidden_by"]
	if !ok {
		t.Fatal("expected 'hidden_by' property in schema")
	}
	if len(hiddenByProp.Enum) != 2 {
		t.Errorf("expected 2 hidden_by enum values, got %d", len(hiddenByProp.Enum))
	}

	// Verify required properties count (action, entity_id, name, icon, area_id, disabled_by, hidden_by, labels, aliases, new_entity_id, format)
	if len(tool.InputSchema.Properties) < 11 {
		t.Errorf("expected at least 11 properties in schema, got %d", len(tool.InputSchema.Properties))
	}
}

func TestHandleManageEntity(t *testing.T) {
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
		// Get Action Tests
		// =========================
		{
			name: "get - found with natural format",
			args: map[string]any{
				"action":    "get",
				"entity_id": "light.living_room",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{
							EntityID:   "light.living_room",
							Name:       "Living Room Light",
							Platform:   "hue",
							AreaID:     "living_room",
							DisabledBy: "",
							HiddenBy:   "",
							Icon:       "mdi:lightbulb",
							Labels:     []string{"bright", "smart"},
							Aliases:    []string{"main light"},
						},
					}, nil
				}
			},
			wantContain: "Living Room Light",
		},
		{
			name: "get - found with json format",
			args: map[string]any{
				"action":    "get",
				"entity_id": "light.living_room",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{
							EntityID: "light.living_room",
							Name:     "Living Room Light",
							Platform: "hue",
						},
					}, nil
				}
			},
			wantContain: `"entity_id":"light.living_room"`,
		},
		{
			name: "get - not found",
			args: map[string]any{
				"action":    "get",
				"entity_id": "light.nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
			},
			wantContain: "not found",
		},
		{
			name: "get - missing entity_id",
			args: map[string]any{
				"action": "get",
			},
			wantContain: "entity_id is required",
		},
		{
			name: "get - API error",
			args: map[string]any{
				"action":    "get",
				"entity_id": "light.living_room",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return nil, errors.New("API connection failed")
				}
			},
			wantErr: true,
		},

		// =========================
		// Update Action Tests
		// =========================
		{
			name: "update - name",
			args: map[string]any{
				"action":    "update",
				"entity_id": "light.living_room",
				"name":      "New Name",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, entityID string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if entityID != "light.living_room" {
						t.Errorf("expected entity_id 'light.living_room', got '%s'", entityID)
					}
					if config.Name == nil || *config.Name != "New Name" {
						t.Error("expected name to be 'New Name'")
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID: entityID,
						Name:     "New Name",
					}, nil
				}
			},
			wantContain: "New Name",
		},
		{
			name: "update - icon",
			args: map[string]any{
				"action":    "update",
				"entity_id": "light.living_room",
				"icon":      "mdi:lamp",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if config.Icon == nil || *config.Icon != "mdi:lamp" {
						t.Error("expected icon to be 'mdi:lamp'")
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.living_room",
						Icon:     "mdi:lamp",
					}, nil
				}
			},
			wantContain: "mdi:lamp",
		},
		{
			name: "update - area_id",
			args: map[string]any{
				"action":    "update",
				"entity_id": "light.living_room",
				"area_id":   "bedroom",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if config.AreaID == nil || *config.AreaID != "bedroom" {
						t.Error("expected area_id to be 'bedroom'")
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.living_room",
						AreaID:   "bedroom",
					}, nil
				}
			},
			wantContain: "bedroom",
		},
		{
			name: "update - disable",
			args: map[string]any{
				"action":      "update",
				"entity_id":   "light.living_room",
				"disabled_by": "user",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if config.DisabledBy == nil || *config.DisabledBy != "user" {
						t.Error("expected disabled_by to be 'user'")
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID:   "light.living_room",
						DisabledBy: "user",
					}, nil
				}
			},
			wantContain: "user",
		},
		{
			name: "update - enable (none maps to empty string)",
			args: map[string]any{
				"action":      "update",
				"entity_id":   "light.living_room",
				"disabled_by": "none",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if config.DisabledBy == nil || *config.DisabledBy != "" {
						t.Error("expected disabled_by to be empty string (mapped from 'none')")
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID:   "light.living_room",
						DisabledBy: "",
					}, nil
				}
			},
			wantContain: "updated successfully",
		},
		{
			name: "update - hide",
			args: map[string]any{
				"action":    "update",
				"entity_id": "light.living_room",
				"hidden_by": "user",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if config.HiddenBy == nil || *config.HiddenBy != "user" {
						t.Error("expected hidden_by to be 'user'")
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.living_room",
						HiddenBy: "user",
					}, nil
				}
			},
			wantContain: "user",
		},
		{
			name: "update - show (none maps to empty string)",
			args: map[string]any{
				"action":    "update",
				"entity_id": "light.living_room",
				"hidden_by": "none",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if config.HiddenBy == nil || *config.HiddenBy != "" {
						t.Error("expected hidden_by to be empty string (mapped from 'none')")
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.living_room",
						HiddenBy: "",
					}, nil
				}
			},
			wantContain: "updated successfully",
		},
		{
			name: "update - labels with replace mode",
			args: map[string]any{
				"action":     "update",
				"entity_id":  "light.living_room",
				"labels":     []any{"bright", "smart"},
				"label_mode": arrayModeReplace,
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if len(config.Labels) != 2 || config.Labels[0] != "bright" || config.Labels[1] != "smart" {
						t.Errorf("expected labels ['bright', 'smart'], got %v", config.Labels)
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.living_room",
						Labels:   config.Labels,
					}, nil
				}
			},
			wantContain: "bright",
		},
		{
			name: "update - aliases with replace mode",
			args: map[string]any{
				"action":     "update",
				"entity_id":  "light.living_room",
				"aliases":    []any{"main light", "big light"},
				"alias_mode": arrayModeReplace,
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if len(config.Aliases) != 2 || config.Aliases[0] != "main light" || config.Aliases[1] != "big light" {
						t.Errorf("expected aliases ['main light', 'big light'], got %v", config.Aliases)
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.living_room",
						Aliases:  config.Aliases,
					}, nil
				}
			},
			wantContain: "main light",
		},
		{
			name: "update - missing entity_id",
			args: map[string]any{
				"action": "update",
				"name":   "New Name",
			},
			wantContain: "entity_id is required",
		},
		{
			name: "update - no fields provided",
			args: map[string]any{
				"action":    "update",
				"entity_id": "light.living_room",
			},
			wantContain: "at least one field",
		},
		{
			name: "update - API error",
			args: map[string]any{
				"action":    "update",
				"entity_id": "light.living_room",
				"name":      "New Name",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(context.Context, string, homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					return nil, errors.New("API connection failed")
				}
			},
			wantErr: true,
		},
		{
			name: "update - json format",
			args: map[string]any{
				"action":    "update",
				"entity_id": "light.living_room",
				"name":      "New Name",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(context.Context, string, homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.living_room",
						Name:     "New Name",
					}, nil
				}
			},
			wantContain: `"entity_id":"light.living_room"`,
		},

		// =========================
		// Label/Alias Mode Tests
		// =========================
		{
			name: "update - labels with add mode merges with existing",
			args: map[string]any{
				"action":     "update",
				"entity_id":  "light.living_room",
				"labels":     []any{"new_label"},
				"label_mode": arrayModeAdd,
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", Labels: []string{"existing_label"}},
					}, nil
				}
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if len(config.Labels) != 2 {
						t.Errorf("expected 2 labels after add, got %v", config.Labels)
					}
					return &homeassistant.EntityRegistryEntry{EntityID: "light.living_room", Labels: config.Labels}, nil
				}
			},
			wantContain: "existing_label",
		},
		{
			name: "update - labels with remove mode removes specified",
			args: map[string]any{
				"action":     "update",
				"entity_id":  "light.living_room",
				"labels":     []any{"remove_me"},
				"label_mode": arrayModeRemove,
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", Labels: []string{"keep_me", "remove_me"}},
					}, nil
				}
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if len(config.Labels) != 1 || config.Labels[0] != "keep_me" {
						t.Errorf("expected ['keep_me'] after remove, got %v", config.Labels)
					}
					return &homeassistant.EntityRegistryEntry{EntityID: "light.living_room", Labels: config.Labels}, nil
				}
			},
			wantContain: "keep_me",
		},
		{
			name: "update - aliases with add mode merges with existing",
			args: map[string]any{
				"action":     "update",
				"entity_id":  "light.living_room",
				"aliases":    []any{"new alias"},
				"alias_mode": arrayModeAdd,
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetEntityRegistryFn = func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "light.living_room", Aliases: []string{"old alias"}},
					}, nil
				}
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if len(config.Aliases) != 2 {
						t.Errorf("expected 2 aliases after add, got %v", config.Aliases)
					}
					return &homeassistant.EntityRegistryEntry{EntityID: "light.living_room", Aliases: config.Aliases}, nil
				}
			},
			wantContain: "old alias",
		},
		// =========================
		// Entity ID Rename Tests
		// =========================
		{
			name: "update - rename entity_id (happy path)",
			args: map[string]any{
				"action":        "update",
				"entity_id":     "light.living_room",
				"new_entity_id": "light.main_room",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, entityID string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if entityID != "light.living_room" {
						t.Errorf("expected entity_id 'light.living_room', got '%s'", entityID)
					}
					if config.NewEntityID == nil || *config.NewEntityID != "light.main_room" {
						t.Error("expected new_entity_id to be 'light.main_room'")
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.main_room",
						Name:     "Living Room Light",
					}, nil
				}
			},
			wantContain: "renamed from light.living_room to light.main_room",
		},
		{
			name: "update - rename + other fields combined",
			args: map[string]any{
				"action":        "update",
				"entity_id":     "light.living_room",
				"new_entity_id": "light.main_room",
				"name":          "Main Room Light",
				"icon":          "mdi:ceiling-light",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					if config.NewEntityID == nil || *config.NewEntityID != "light.main_room" {
						t.Error("expected new_entity_id to be 'light.main_room'")
					}
					if config.Name == nil || *config.Name != "Main Room Light" {
						t.Error("expected name to be 'Main Room Light'")
					}
					if config.Icon == nil || *config.Icon != "mdi:ceiling-light" {
						t.Error("expected icon to be 'mdi:ceiling-light'")
					}
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.main_room",
						Name:     "Main Room Light",
						Icon:     "mdi:ceiling-light",
					}, nil
				}
			},
			wantContain: "renamed from light.living_room to light.main_room",
		},
		{
			name: "update - invalid new_entity_id format (no dot)",
			args: map[string]any{
				"action":        "update",
				"entity_id":     "light.living_room",
				"new_entity_id": "invalid_format",
			},
			wantContain: "must be in format 'domain.object_id'",
		},
		{
			name: "update - invalid new_entity_id format (invalid chars)",
			args: map[string]any{
				"action":        "update",
				"entity_id":     "light.living_room",
				"new_entity_id": "light.UPPERCASE",
			},
			wantContain: "contains invalid characters",
		},
		{
			name: "update - rename counts as has fields",
			args: map[string]any{
				"action":        "update",
				"entity_id":     "light.living_room",
				"new_entity_id": "light.main_room",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(context.Context, string, homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.main_room",
					}, nil
				}
			},
			wantContain: "renamed from light.living_room to light.main_room",
		},
		{
			name: "update - json format with rename",
			args: map[string]any{
				"action":        "update",
				"entity_id":     "light.living_room",
				"new_entity_id": "light.main_room",
				"format":        "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateEntityRegistryEntryFn = func(context.Context, string, homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
					return &homeassistant.EntityRegistryEntry{
						EntityID: "light.main_room",
						Name:     "Living Room Light",
					}, nil
				}
			},
			wantContain: `"entity_id":"light.main_room"`,
		},

		// =========================
		// Delete Action Tests
		// =========================
		{
			name: "delete - success natural format",
			args: map[string]any{
				"action":    "delete",
				"entity_id": "sensor.dead_sensor",
			},
			setupMock: func(m *UniversalMockClient) {
				m.RemoveEntityRegistryEntryFn = func(_ context.Context, entityID string) error {
					if entityID != "sensor.dead_sensor" {
						t.Errorf("expected entity_id 'sensor.dead_sensor', got '%s'", entityID)
					}
					return nil
				}
			},
			wantContain: "deleted from registry",
		},
		{
			name: "delete - success json format",
			args: map[string]any{
				"action":    "delete",
				"entity_id": "sensor.dead_sensor",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.RemoveEntityRegistryEntryFn = func(context.Context, string) error {
					return nil
				}
			},
			wantContain: `"deleted":true`,
		},
		{
			name: "delete - missing entity_id",
			args: map[string]any{
				"action": "delete",
			},
			wantContain: "entity_id is required",
		},
		{
			name: "delete - API error",
			args: map[string]any{
				"action":    "delete",
				"entity_id": "sensor.dead_sensor",
			},
			setupMock: func(m *UniversalMockClient) {
				m.RemoveEntityRegistryEntryFn = func(context.Context, string) error {
					return errors.New("API connection failed")
				}
			},
			wantErr: true,
		},

		// =========================
		// Invalid Action Tests
		// =========================
		{
			name: "invalid action",
			args: map[string]any{
				"action": "invalid",
			},
			wantContain: "unsupported action",
		},
		{
			name:        "missing action",
			args:        map[string]any{},
			wantContain: "action is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mock := &UniversalMockClient{}
			if tc.setupMock != nil {
				tc.setupMock(mock)
			}

			handler := NewEntityManageHandlers()
			result, err := handler.handleManageEntity(context.Background(), mock, tc.args)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil || len(result.Content) == 0 {
				t.Fatal("expected result with content")
			}

			resultText := result.Content[0].Text
			if tc.wantContain != "" && !strings.Contains(resultText, tc.wantContain) {
				t.Errorf("expected result to contain '%s', got: %s", tc.wantContain, resultText)
			}
		})
	}
}
