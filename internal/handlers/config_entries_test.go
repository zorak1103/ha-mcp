package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestConfigEntryHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	h := NewConfigEntryHandlers()
	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	// Verify expected tool exists
	if len(tools) > 0 && tools[0].Name != "manage_config_entry" {
		t.Errorf("Expected tool 'manage_config_entry', got %q", tools[0].Name)
	}
}

func TestConfigEntryHandlers_ManageConfigEntryTool_Schema(t *testing.T) {
	t.Parallel()

	h := NewConfigEntryHandlers()
	tool := h.manageConfigEntryTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "manage_config_entry",
		RequiredParams:  []string{"action"},
		OptionalParams:  []string{"domain", "entry_id", "format"},
		WantDescription: true,
	})

	// Verify action enum has exactly 2 values
	actionSchema, ok := tool.InputSchema.Properties["action"]
	if !ok {
		t.Fatal("action property not found in schema")
	}
	if len(actionSchema.Enum) != 3 {
		t.Errorf("action enum has %d values, want 3", len(actionSchema.Enum))
	}

	// Verify format enum has exactly 2 values
	formatSchema, ok := tool.InputSchema.Properties["format"]
	if !ok {
		t.Fatal("format property not found in schema")
	}
	if len(formatSchema.Enum) != 2 {
		t.Errorf("format enum has %d values, want 2", len(formatSchema.Enum))
	}
}

func TestConfigEntryHandlers_HandleListConfigEntries(t *testing.T) {
	t.Parallel()

	h := NewConfigEntryHandlers()

	tests := []handlerTestCase{
		{
			name: "list all entries - natural format",
			args: map[string]any{"action": "list"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntriesFn = func(_ context.Context, domain string) ([]homeassistant.ConfigEntryFull, error) {
					if domain != "" {
						t.Errorf("expected empty domain, got %q", domain)
					}
					return []homeassistant.ConfigEntryFull{
						{
							EntryID: "abc123",
							Domain:  "template",
							Title:   "My Template Sensor",
							State:   "loaded",
						},
						{
							EntryID: "def456",
							Domain:  "hue",
							Title:   "Philips Hue",
							State:   "loaded",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"My Template Sensor", "template", "Philips Hue", "hue"},
		},
		{
			name: "list all entries - json format",
			args: map[string]any{"action": "list", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntriesFn = func(_ context.Context, domain string) ([]homeassistant.ConfigEntryFull, error) {
					if domain != "" {
						t.Errorf("expected empty domain, got %q", domain)
					}
					return []homeassistant.ConfigEntryFull{
						{
							EntryID: "abc123",
							Domain:  "template",
							Title:   "My Template Sensor",
							State:   "loaded",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"abc123", "template"},
		},
		{
			name: "filter by domain",
			args: map[string]any{"action": "list", "domain": "template"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntriesFn = func(_ context.Context, domain string) ([]homeassistant.ConfigEntryFull, error) {
					if domain != "template" {
						t.Errorf("expected domain 'template', got %q", domain)
					}
					return []homeassistant.ConfigEntryFull{
						{
							EntryID: "abc123",
							Domain:  "template",
							Title:   "My Template Sensor",
							State:   "loaded",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"My Template Sensor", "template"},
		},
		{
			name: "empty results",
			args: map[string]any{"action": "list"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 0 config entries"},
		},
		{
			name: "client error",
			args: map[string]any{"action": "list"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return nil, errors.New("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "connection failed"},
		},
		{
			name:         "missing action",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"action is required"},
		},
		{
			name:         "invalid action",
			args:         map[string]any{"action": "invalid"},
			wantError:    true,
			wantContains: []string{"invalid action"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageConfigEntry)
}

func TestConfigEntryHandlers_HandleGetConfigEntry(t *testing.T) {
	t.Parallel()

	h := NewConfigEntryHandlers()

	tests := []handlerTestCase{
		{
			name: "get entry with options - natural format",
			args: map[string]any{"action": "get", "entry_id": "abc123"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntryFn = func(_ context.Context, entryID string) (*homeassistant.ConfigEntryFull, error) {
					if entryID != "abc123" {
						t.Errorf("expected entry_id 'abc123', got %q", entryID)
					}
					return &homeassistant.ConfigEntryFull{
						EntryID: "abc123",
						Domain:  "template",
						Title:   "My Template Sensor",
						State:   "loaded",
						Options: map[string]any{
							"name":  "Angeschaltete Lichter",
							"state": "{{ states.light | selectattr('state', 'eq', 'on') | list | count }}",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Config Entry", "My Template Sensor", "Domain", "template"},
		},
		{
			name: "get entry with options - json format",
			args: map[string]any{"action": "get", "entry_id": "abc123", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntryFn = func(_ context.Context, entryID string) (*homeassistant.ConfigEntryFull, error) {
					if entryID != "abc123" {
						t.Errorf("expected entry_id 'abc123', got %q", entryID)
					}
					return &homeassistant.ConfigEntryFull{
						EntryID: "abc123",
						Domain:  "template",
						Title:   "My Template Sensor",
						State:   "loaded",
						Options: map[string]any{
							"name":  "Angeschaltete Lichter",
							"state": "{{ states.light | selectattr('state', 'eq', 'on') | list | count }}",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"abc123", "template", "state", "selectattr"},
		},
		{
			name: "entry not found",
			args: map[string]any{"action": "get", "entry_id": "nonexistent"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntryFn = func(_ context.Context, _ string) (*homeassistant.ConfigEntryFull, error) {
					return nil, errors.New("config entry not found")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "config entry not found"},
		},
		{
			name:         "missing entry_id",
			args:         map[string]any{"action": "get"},
			wantError:    true,
			wantContains: []string{"entry_id is required"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageConfigEntry)
}

func TestConfigEntryHandlers_HandleDeleteConfigEntry(t *testing.T) {
	t.Parallel()

	h := NewConfigEntryHandlers()

	tests := []handlerTestCase{
		{
			name: "delete entry - success",
			args: map[string]any{"action": "delete", "entry_id": "abc123"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntryFn = func(_ context.Context, entryID string) (*homeassistant.ConfigEntryFull, error) {
					if entryID != "abc123" {
						t.Errorf("expected entry_id 'abc123', got %q", entryID)
					}
					return &homeassistant.ConfigEntryFull{
						EntryID: "abc123",
						Domain:  "mobile_app",
						Title:   "Old Phone",
						State:   "loaded",
					}, nil
				}
				m.DeleteConfigEntryFn = func(_ context.Context, entryID string) error {
					if entryID != "abc123" {
						t.Errorf("expected entry_id 'abc123', got %q", entryID)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"Old Phone", "mobile_app", "abc123"},
		},
		{
			name: "delete entry - config entry not found on preflight get",
			args: map[string]any{"action": "delete", "entry_id": "nonexistent"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntryFn = func(_ context.Context, _ string) (*homeassistant.ConfigEntryFull, error) {
					return nil, errors.New("config entry not found: nonexistent")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "config entry not found"},
		},
		{
			name: "delete entry - delete call fails",
			args: map[string]any{"action": "delete", "entry_id": "abc123"},
			setupMock: func(m *UniversalMockClient) {
				m.GetConfigEntryFn = func(_ context.Context, _ string) (*homeassistant.ConfigEntryFull, error) {
					return &homeassistant.ConfigEntryFull{EntryID: "abc123", Domain: "mobile_app", Title: "Old Phone"}, nil
				}
				m.DeleteConfigEntryFn = func(_ context.Context, _ string) error {
					return errors.New("forbidden: insufficient permissions to delete config entry")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "forbidden"},
		},
		{
			name:         "missing entry_id",
			args:         map[string]any{"action": "delete"},
			wantError:    true,
			wantContains: []string{"entry_id is required"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageConfigEntry)
}

func TestRegisterConfigEntryTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterConfigEntryTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterConfigEntryTools() registered no tools")
	}
}
