package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestDeviceManageHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	handler := NewDeviceManageHandlers()

	handler.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "manage_device" {
		t.Errorf("expected tool 'manage_device', got '%s'", tools[0].Name)
	}
}

func TestManageDevice_Schema(t *testing.T) {
	t.Parallel()

	handler := NewDeviceManageHandlers()
	tool := handler.manageDeviceTool()

	if tool.Name != "manage_device" {
		t.Errorf("expected tool name 'manage_device', got '%s'", tool.Name)
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

	// Verify required properties count
	if len(tool.InputSchema.Properties) < 6 {
		t.Errorf("expected at least 6 properties in schema, got %d", len(tool.InputSchema.Properties))
	}
}

func TestHandleManageDevice(t *testing.T) {
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
				"device_id": "abc123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{
							ID:           "abc123",
							Name:         "Living Room Hub",
							Manufacturer: "Philips",
							Model:        "Hue Bridge",
							AreaID:       "living_room",
							NameByUser:   "My Hub",
							Labels:       []string{"smart", "hub"},
						},
					}, nil
				}
			},
			wantContain: "Living Room Hub",
		},
		{
			name: "get - found with json format",
			args: map[string]any{
				"action":    "get",
				"device_id": "abc123",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{
							ID:           "abc123",
							Name:         "Living Room Hub",
							Manufacturer: "Philips",
						},
					}, nil
				}
			},
			wantContain: `"id":"abc123"`,
		},
		{
			name: "get - not found",
			args: map[string]any{
				"action":    "get",
				"device_id": "nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{}, nil
				}
			},
			wantContain: "not found",
		},
		{
			name: "get - missing device_id",
			args: map[string]any{
				"action": "get",
			},
			wantContain: "device_id is required",
		},
		{
			name: "get - API error",
			args: map[string]any{
				"action":    "get",
				"device_id": "abc123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return nil, errors.New("API connection failed")
				}
			},
			wantErr: true,
		},

		// =========================
		// Update Action Tests
		// =========================
		{
			name: "update - name_by_user",
			args: map[string]any{
				"action":       "update",
				"device_id":    "abc123",
				"name_by_user": "My Custom Name",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDeviceRegistryEntryFn = func(_ context.Context, deviceID string, config homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
					if deviceID != "abc123" {
						t.Errorf("expected device_id 'abc123', got '%s'", deviceID)
					}
					if config.NameByUser == nil || *config.NameByUser != "My Custom Name" {
						t.Error("expected name_by_user to be 'My Custom Name'")
					}
					return &homeassistant.DeviceRegistryEntry{
						ID:         deviceID,
						NameByUser: "My Custom Name",
					}, nil
				}
			},
			wantContain: "My Custom Name",
		},
		{
			name: "update - area_id",
			args: map[string]any{
				"action":    "update",
				"device_id": "abc123",
				"area_id":   "bedroom",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDeviceRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
					if config.AreaID == nil || *config.AreaID != "bedroom" {
						t.Error("expected area_id to be 'bedroom'")
					}
					return &homeassistant.DeviceRegistryEntry{
						ID:     "abc123",
						AreaID: "bedroom",
					}, nil
				}
			},
			wantContain: "bedroom",
		},
		{
			name: "update - disable",
			args: map[string]any{
				"action":      "update",
				"device_id":   "abc123",
				"disabled_by": "user",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDeviceRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
					if config.DisabledBy == nil || *config.DisabledBy != "user" {
						t.Error("expected disabled_by to be 'user'")
					}
					return &homeassistant.DeviceRegistryEntry{
						ID:         "abc123",
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
				"device_id":   "abc123",
				"disabled_by": "none",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDeviceRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
					if config.DisabledBy == nil || *config.DisabledBy != "" {
						t.Error("expected disabled_by to be empty string (mapped from 'none')")
					}
					return &homeassistant.DeviceRegistryEntry{
						ID:         "abc123",
						DisabledBy: "",
					}, nil
				}
			},
			wantContain: "updated successfully",
		},
		{
			name: "update - labels with replace mode",
			args: map[string]any{
				"action":     "update",
				"device_id":  "abc123",
				"labels":     []any{"smart", "hub"},
				"label_mode": arrayModeReplace,
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDeviceRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
					if len(config.Labels) != 2 || config.Labels[0] != "smart" || config.Labels[1] != "hub" {
						t.Errorf("expected labels ['smart', 'hub'], got %v", config.Labels)
					}
					return &homeassistant.DeviceRegistryEntry{
						ID:     "abc123",
						Labels: config.Labels,
					}, nil
				}
			},
			wantContain: "smart",
		},
		{
			name: "update - missing device_id",
			args: map[string]any{
				"action":       "update",
				"name_by_user": "New Name",
			},
			wantContain: "device_id is required",
		},
		{
			name: "update - no fields provided",
			args: map[string]any{
				"action":    "update",
				"device_id": "abc123",
			},
			wantContain: "at least one field",
		},
		{
			name: "update - API error",
			args: map[string]any{
				"action":       "update",
				"device_id":    "abc123",
				"name_by_user": "New Name",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDeviceRegistryEntryFn = func(context.Context, string, homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
					return nil, errors.New("API connection failed")
				}
			},
			wantErr: true,
		},
		{
			name: "update - json format",
			args: map[string]any{
				"action":       "update",
				"device_id":    "abc123",
				"name_by_user": "New Name",
				"format":       "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.UpdateDeviceRegistryEntryFn = func(context.Context, string, homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
					return &homeassistant.DeviceRegistryEntry{
						ID:         "abc123",
						NameByUser: "New Name",
					}, nil
				}
			},
			wantContain: `"id":"abc123"`,
		},

		// =========================
		// Label Mode Tests
		// =========================
		{
			name: "update - labels with add mode merges with existing",
			args: map[string]any{
				"action":     "update",
				"device_id":  "abc123",
				"labels":     []any{"new_label"},
				"label_mode": arrayModeAdd,
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "abc123", Labels: []string{"existing_label"}},
					}, nil
				}
				m.UpdateDeviceRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
					if len(config.Labels) != 2 {
						t.Errorf("expected 2 labels after add, got %v", config.Labels)
					}
					return &homeassistant.DeviceRegistryEntry{ID: "abc123", Labels: config.Labels}, nil
				}
			},
			wantContain: "existing_label",
		},
		{
			name: "update - labels with remove mode removes specified",
			args: map[string]any{
				"action":     "update",
				"device_id":  "abc123",
				"labels":     []any{"remove_me"},
				"label_mode": arrayModeRemove,
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "abc123", Labels: []string{"keep_me", "remove_me"}},
					}, nil
				}
				m.UpdateDeviceRegistryEntryFn = func(_ context.Context, _ string, config homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
					if len(config.Labels) != 1 || config.Labels[0] != "keep_me" {
						t.Errorf("expected [keep_me] after remove, got %v", config.Labels)
					}
					return &homeassistant.DeviceRegistryEntry{ID: "abc123", Labels: config.Labels}, nil
				}
			},
			wantContain: "keep_me",
		},
		// =========================
		// Delete Action Tests
		// =========================
		{
			name: "delete - success natural format",
			args: map[string]any{
				"action":    "delete",
				"device_id": "abc123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "abc123", Name: "Test Device", ConfigEntries: []string{"entry1"}},
					}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{
						{EntryID: "entry1", Domain: "test_integration", SupportsRemoveDevice: true},
					}, nil
				}
				m.RemoveDeviceConfigEntryFn = func(_ context.Context, deviceID, _ string) error {
					if deviceID != "abc123" {
						t.Errorf("expected device_id 'abc123', got '%s'", deviceID)
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
				"device_id": "abc123",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "abc123", Name: "Test Device", ConfigEntries: []string{"entry1"}},
					}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{
						{EntryID: "entry1", Domain: "test_integration", SupportsRemoveDevice: true},
					}, nil
				}
				m.RemoveDeviceConfigEntryFn = func(context.Context, string, string) error {
					return nil
				}
			},
			wantContain: `"deleted":true`,
		},
		{
			name: "delete - missing device_id",
			args: map[string]any{
				"action": "delete",
			},
			wantContain: "device_id is required",
		},
		{
			name: "delete - device not found",
			args: map[string]any{
				"action":    "delete",
				"device_id": "nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{}, nil
				}
			},
			wantContain: "not found",
		},
		{
			name: "delete - unsupported removal",
			args: map[string]any{
				"action":    "delete",
				"device_id": "abc123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "abc123", Name: "Test Device", ConfigEntries: []string{"entry1"}},
					}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{
						{EntryID: "entry1", Domain: "test_integration", SupportsRemoveDevice: false},
					}, nil
				}
			},
			wantContain: "does not support device removal",
		},
		{
			name: "delete - API error fetching registry",
			args: map[string]any{
				"action":    "delete",
				"device_id": "abc123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return nil, errors.New("API connection failed")
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

			handler := NewDeviceManageHandlers()
			result, err := handler.handleManageDevice(context.Background(), mock, tc.args)

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
