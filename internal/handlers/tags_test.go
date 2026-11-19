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

func TestTagHandlers_ManageTagToolSchema(t *testing.T) {
	t.Parallel()

	h := NewTagHandlers()
	tool := h.manageTagTool()

	if tool.Name != "manage_tag" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_tag")
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

func TestManageTag_List(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "list tags with natural format",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "nfc_001", Name: "Front Door", Description: "Main entrance", LastScanned: "2024-01-15T10:30:00"},
						{TagID: "nfc_002", Name: "Garage", Description: "Garage access"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 tag(s)", "Front Door", "Garage", "nfc_001", "nfc_002", "Main entrance"},
		},
		{
			name: "list tags with json format",
			args: map[string]any{
				"action": "list",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "tag_1", Name: "Test Tag"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"tag_1", "Test Tag"},
		},
		{
			name: "list tags empty",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No tags found"},
		},
		{
			name: "list tags with name filter",
			args: map[string]any{
				"action":        "list",
				"name_contains": "door",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "tag_front", Name: "Front Door"},
						{TagID: "tag_back", Name: "Back Door"},
						{TagID: "tag_garage", Name: "Garage"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 tag(s)", "Front Door", "Back Door"},
		},
		{
			name: "list tags API error",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "listing"},
		},
	}

	h := NewTagHandlers()
	runHandlerTestCases(t, tests, h.handleManageTag)
}

// =============================================================================
// Get Tests
// =============================================================================

func TestManageTag_Get(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "get tag with natural format",
			args: map[string]any{
				"action": "get",
				"tag_id": "nfc_001",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{
							TagID:       "nfc_001",
							Name:        "Front Door",
							Description: "Main entrance tag",
							LastScanned: "2024-01-15T10:30:00",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Tag: Front Door", "ID: nfc_001", "Description: Main entrance", "Last scanned: 2024-01-15"},
		},
		{
			name: "get tag with json format",
			args: map[string]any{
				"action": "get",
				"tag_id": "tag_1",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "tag_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"tag_1", "Test"},
		},
		{
			name: "get tag not found",
			args: map[string]any{
				"action": "get",
				"tag_id": "nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "tag_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"tag not found", "nonexistent"},
		},
		{
			name: "get tag without tag_id",
			args: map[string]any{
				"action": "get",
			},
			wantError:    true,
			wantContains: []string{"tag_id", "required"},
		},
	}

	h := NewTagHandlers()
	runHandlerTestCases(t, tests, h.handleManageTag)
}

// =============================================================================
// Create Tests
// =============================================================================

func TestManageTag_Create(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create tag successfully",
			args: map[string]any{
				"action": "create",
				"name":   "Kitchen Tag",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateTagFn = func(_ context.Context, config homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
					return &homeassistant.TagRegistryEntry{
						TagID: "tag_kitchen",
						Name:  config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully", "Kitchen Tag", "tag_kitchen"},
		},
		{
			name: "create tag with all fields",
			args: map[string]any{
				"action":      "create",
				"tag_id":      "nfc_custom_001",
				"name":        "Custom Tag",
				"description": "Custom NFC tag",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateTagFn = func(_ context.Context, config homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
					if config.TagID != "nfc_custom_001" || config.Name != "Custom Tag" {
						return nil, fmt.Errorf("unexpected config")
					}
					return &homeassistant.TagRegistryEntry{
						TagID: config.TagID,
						Name:  config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully"},
		},
		{
			name: "create tag without name",
			args: map[string]any{
				"action": "create",
			},
			wantError:    true,
			wantContains: []string{"name", "required"},
		},
		{
			name: "create tag API error",
			args: map[string]any{
				"action": "create",
				"name":   "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateTagFn = func(context.Context, homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
					return nil, fmt.Errorf("database error")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "creating"},
		},
	}

	h := NewTagHandlers()
	runHandlerTestCases(t, tests, h.handleManageTag)
}

// =============================================================================
// Update Tests
// =============================================================================

func TestManageTag_Update(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "update tag name",
			args: map[string]any{
				"action": "update",
				"tag_id": "tag_1",
				"name":   "Updated Name",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "tag_1", Name: "Test Tag"},
						{TagID: "tag_2", Name: "Test Tag 2"},
					}, nil
				}
				m.UpdateTagFn = func(_ context.Context, tagID string, config homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
					if tagID != "tag_1" {
						return nil, fmt.Errorf("unexpected tag_id: %s", tagID)
					}
					return &homeassistant.TagRegistryEntry{
						TagID: tagID,
						Name:  config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully", "Updated Name"},
		},
		{
			name: "update tag description",
			args: map[string]any{
				"action":      "update",
				"tag_id":      "tag_2",
				"description": "New description",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "tag_1", Name: "Test Tag"},
						{TagID: "tag_2", Name: "Test"},
					}, nil
				}
				m.UpdateTagFn = func(_ context.Context, tagID string, _ homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
					return &homeassistant.TagRegistryEntry{
						TagID: tagID,
						Name:  "Test",
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully"},
		},
		{
			name: "update tag without tag_id",
			args: map[string]any{
				"action": "update",
				"name":   "New Name",
			},
			wantError:    true,
			wantContains: []string{"tag_id", "required"},
		},
		{
			name: "update tag API error",
			args: map[string]any{
				"action": "update",
				"tag_id": "tag_1",
				"name":   "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "tag_1", Name: "Test Tag"},
					}, nil
				}
				m.UpdateTagFn = func(context.Context, string, homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
					return nil, fmt.Errorf("not found")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "updating"},
		},
	}

	h := NewTagHandlers()
	runHandlerTestCases(t, tests, h.handleManageTag)
}

// =============================================================================
// Delete Tests
// =============================================================================

func TestManageTag_Delete(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "delete tag successfully",
			args: map[string]any{
				"action": "delete",
				"tag_id": "tag_old",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "tag_old", Name: "Old Tag"},
						{TagID: "tag_new", Name: "New Tag"},
					}, nil
				}
				m.DeleteTagFn = func(_ context.Context, tagID string) error {
					if tagID != "tag_old" {
						return fmt.Errorf("unexpected tag_id: %s", tagID)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"deleted successfully", "tag_old"},
		},
		{
			name: "delete tag without tag_id",
			args: map[string]any{
				"action": "delete",
			},
			wantError:    true,
			wantContains: []string{"tag_id", "required"},
		},
		{
			name: "delete tag API error",
			args: map[string]any{
				"action": "delete",
				"tag_id": "tag_1",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
					return []homeassistant.TagRegistryEntry{
						{TagID: "tag_1", Name: "Test Tag"},
					}, nil
				}
				m.DeleteTagFn = func(context.Context, string) error {
					return fmt.Errorf("tag in use")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "deleting"},
		},
	}

	h := NewTagHandlers()
	runHandlerTestCases(t, tests, h.handleManageTag)
}

// =============================================================================
// Validation Tests
// =============================================================================

func TestManageTag_ValidationErrors(t *testing.T) {
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

	h := NewTagHandlers()
	runHandlerTestCases(t, tests, h.handleManageTag)
}

// =============================================================================
// Registration Tests
// =============================================================================

func TestTagHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewTagHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	// Verify manage_tag is registered
	tools := registry.ListTools()
	found := false

	for _, tool := range tools {
		if tool.Name == "manage_tag" {
			found = true
			break
		}
	}

	if !found {
		t.Error("manage_tag tool not registered")
	}
}

func TestTagList_RejectsNonStringFilters(t *testing.T) {
	t.Parallel()

	mustNotList := func(t *testing.T, c *UniversalMockClient) {
		t.Helper()
		c.GetTagsFn = func(context.Context) ([]homeassistant.TagRegistryEntry, error) {
			t.Errorf("HA must not be called for an invalid filter")
			return nil, nil
		}
	}

	runFilterTypeCases(t, NewTagHandlers().handleManageTag, []filterTypeCase{
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
