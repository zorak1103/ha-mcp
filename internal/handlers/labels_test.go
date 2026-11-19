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

func TestLabelHandlers_ManageLabelToolSchema(t *testing.T) {
	t.Parallel()

	h := NewLabelHandlers()
	tool := h.manageLabelTool()

	if tool.Name != "manage_label" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_label")
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

func TestManageLabel_List(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "list labels with natural format",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Important", Color: "red", Icon: "mdi:alert"},
						{LabelID: "label_2", Name: "Work", Description: "Work related"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 label(s)", "Important", "Work", "label_1", "label_2"},
		},
		{
			name: "list labels with json format",
			args: map[string]any{
				"action": "list",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"label_1", "Test", "label_id"},
		},
		{
			name: "list labels empty",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No labels found"},
		},
		{
			name: "list labels with name filter",
			args: map[string]any{
				"action":        "list",
				"name_contains": "work",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Work"},
						{LabelID: "label_2", Name: "Home"},
						{LabelID: "label_3", Name: "Network"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 label(s)", "Work", "Network"},
		},
		{
			name: "list labels API error",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "listing"},
		},
	}

	h := NewLabelHandlers()
	runHandlerTestCases(t, tests, h.handleManageLabel)
}

// =============================================================================
// Get Tests
// =============================================================================

func TestManageLabel_Get(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "get label with natural format",
			args: map[string]any{
				"action":   "get",
				"label_id": "label_important",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{
							LabelID:     "label_important",
							Name:        "Important",
							Color:       "red",
							Icon:        "mdi:alert",
							Description: "High priority items",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Label: Important", "ID: label_important", "Color: red", "Icon: mdi:alert", "Description: High priority"},
		},
		{
			name: "get label with json format",
			args: map[string]any{
				"action":   "get",
				"label_id": "label_work",
				"format":   "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_work", Name: "Work"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"label_work", "Work", "label_id"},
		},
		{
			name: "get label not found",
			args: map[string]any{
				"action":   "get",
				"label_id": "nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"label not found", "nonexistent", "tried as"},
		},
		{
			name: "get label without label_id",
			args: map[string]any{
				"action": "get",
			},
			wantError:    true,
			wantContains: []string{"label_id", "required"},
		},
	}

	h := NewLabelHandlers()
	runHandlerTestCases(t, tests, h.handleManageLabel)
}

// =============================================================================
// Get Tests - Name Fallback
// =============================================================================

func TestManageLabel_GetByNameFallback(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "get label by exact name match",
			args: map[string]any{
				"action":   "get",
				"label_id": "Important",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_important", Name: "Important"},
						{LabelID: "label_work", Name: "Work"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Label: Important", "ID: label_important"},
		},
		{
			name: "get label by partial name match",
			args: map[string]any{
				"action":   "get",
				"label_id": "import",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_important", Name: "Important"},
						{LabelID: "label_work", Name: "Work"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Label: Important", "ID: label_important"},
		},
		{
			name: "get label by case-insensitive name",
			args: map[string]any{
				"action":   "get",
				"label_id": "IMPORTANT",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_important", Name: "Important"},
						{LabelID: "label_work", Name: "Work"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Label: Important", "ID: label_important"},
		},
		{
			name: "ID takes precedence over name",
			args: map[string]any{
				"action":   "get",
				"label_id": "label_work",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_work", Name: "Work"},
						{LabelID: "label_other", Name: "label_work"}, // Name matches input
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Label: Work", "ID: label_work"}, // Should find by ID, not name
		},
		{
			name: "get label not found with fallback - updated error message",
			args: map[string]any{
				"action":   "get",
				"label_id": "nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"label not found", "nonexistent", "tried as"},
		},
	}

	h := NewLabelHandlers()
	runHandlerTestCases(t, tests, h.handleManageLabel)
}

// =============================================================================
// Update Tests - Name Fallback
// =============================================================================

func TestManageLabel_UpdateByNameFallback(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "update label by name fallback",
			args: map[string]any{
				"action":   "update",
				"label_id": "Important",
				"color":    "blue",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_important", Name: "Important"},
					}, nil
				}
				m.UpdateLabelFn = func(_ context.Context, labelID string, config homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
					if labelID != "label_important" {
						return nil, fmt.Errorf("unexpected label_id: %s", labelID)
					}
					return &homeassistant.LabelRegistryEntry{
						LabelID: labelID,
						Name:    "Important",
						Color:   config.Color,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully", "Important"},
		},
		{
			name: "update label by name not found",
			args: map[string]any{
				"action":   "update",
				"label_id": "nonexistent",
				"color":    "blue",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"label not found", "nonexistent", "tried as"},
		},
	}

	h := NewLabelHandlers()
	runHandlerTestCases(t, tests, h.handleManageLabel)
}

// =============================================================================
// Delete Tests - Name Fallback
// =============================================================================

func TestManageLabel_DeleteByNameFallback(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "delete label by name fallback",
			args: map[string]any{
				"action":   "delete",
				"label_id": "Important",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_important", Name: "Important"},
					}, nil
				}
				m.DeleteLabelFn = func(_ context.Context, labelID string) error {
					if labelID != "label_important" {
						return fmt.Errorf("unexpected label_id: %s", labelID)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"deleted successfully", "label_important"},
		},
		{
			name: "delete label by name not found",
			args: map[string]any{
				"action":   "delete",
				"label_id": "nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"label not found", "nonexistent", "tried as"},
		},
	}

	h := NewLabelHandlers()
	runHandlerTestCases(t, tests, h.handleManageLabel)
}

// =============================================================================
// Create Tests
// =============================================================================

func TestManageLabel_Create(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create label successfully",
			args: map[string]any{
				"action": "create",
				"name":   "New Label",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateLabelFn = func(_ context.Context, config homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
					return &homeassistant.LabelRegistryEntry{
						LabelID: "label_new",
						Name:    config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully", "New Label", "label_new"},
		},
		{
			name: "create label with all fields",
			args: map[string]any{
				"action":      "create",
				"name":        "Urgent",
				"color":       "red",
				"icon":        "mdi:fire",
				"description": "Urgent tasks",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateLabelFn = func(_ context.Context, config homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
					if config.Name != "Urgent" || config.Color != "red" || config.Icon != "mdi:fire" {
						return nil, fmt.Errorf("unexpected config")
					}
					return &homeassistant.LabelRegistryEntry{
						LabelID: "label_urgent",
						Name:    config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully"},
		},
		{
			name: "create label without name",
			args: map[string]any{
				"action": "create",
			},
			wantError:    true,
			wantContains: []string{"name", "required"},
		},
		{
			name: "create label API error",
			args: map[string]any{
				"action": "create",
				"name":   "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreateLabelFn = func(context.Context, homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
					return nil, fmt.Errorf("database error")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "creating"},
		},
	}

	h := NewLabelHandlers()
	runHandlerTestCases(t, tests, h.handleManageLabel)
}

// =============================================================================
// Update Tests
// =============================================================================

func TestManageLabel_Update(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "update label name",
			args: map[string]any{
				"action":   "update",
				"label_id": "label_1",
				"name":     "Updated Name",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Test"},
					}, nil
				}
				m.UpdateLabelFn = func(_ context.Context, labelID string, config homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
					if labelID != "label_1" {
						return nil, fmt.Errorf("unexpected label_id: %s", labelID)
					}
					return &homeassistant.LabelRegistryEntry{
						LabelID: labelID,
						Name:    config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully", "Updated Name"},
		},
		{
			name: "update label color and icon",
			args: map[string]any{
				"action":   "update",
				"label_id": "label_2",
				"color":    "blue",
				"icon":     "mdi:check",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_2", Name: "Test"},
					}, nil
				}
				m.UpdateLabelFn = func(_ context.Context, labelID string, config homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
					return &homeassistant.LabelRegistryEntry{
						LabelID: labelID,
						Name:    "Test",
						Color:   config.Color,
						Icon:    config.Icon,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully"},
		},
		{
			name: "update label without label_id",
			args: map[string]any{
				"action": "update",
				"name":   "New Name",
			},
			wantError:    true,
			wantContains: []string{"label_id", "required"},
		},
		{
			name: "update label API error",
			args: map[string]any{
				"action":   "update",
				"label_id": "label_1",
				"name":     "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Test"},
					}, nil
				}
				m.UpdateLabelFn = func(context.Context, string, homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
					return nil, fmt.Errorf("not found")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "updating"},
		},
	}

	h := NewLabelHandlers()
	runHandlerTestCases(t, tests, h.handleManageLabel)
}

// =============================================================================
// Delete Tests
// =============================================================================

func TestManageLabel_Delete(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "delete label successfully",
			args: map[string]any{
				"action":   "delete",
				"label_id": "label_old",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_old", Name: "Old Label"},
					}, nil
				}
				m.DeleteLabelFn = func(_ context.Context, labelID string) error {
					if labelID != "label_old" {
						return fmt.Errorf("unexpected label_id: %s", labelID)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"deleted successfully", "label_old"},
		},
		{
			name: "delete label without label_id",
			args: map[string]any{
				"action": "delete",
			},
			wantError:    true,
			wantContains: []string{"label_id", "required"},
		},
		{
			name: "delete label API error",
			args: map[string]any{
				"action":   "delete",
				"label_id": "label_1",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
					return []homeassistant.LabelRegistryEntry{
						{LabelID: "label_1", Name: "Test"},
					}, nil
				}
				m.DeleteLabelFn = func(context.Context, string) error {
					return fmt.Errorf("label in use")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "deleting"},
		},
	}

	h := NewLabelHandlers()
	runHandlerTestCases(t, tests, h.handleManageLabel)
}

// =============================================================================
// Validation Tests
// =============================================================================

func TestManageLabel_ValidationErrors(t *testing.T) {
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

	h := NewLabelHandlers()
	runHandlerTestCases(t, tests, h.handleManageLabel)
}

// =============================================================================
// Registration Tests
// =============================================================================

func TestLabelHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewLabelHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	// Verify manage_label is registered
	tools := registry.ListTools()
	found := false

	for _, tool := range tools {
		if tool.Name == "manage_label" {
			found = true
			break
		}
	}

	if !found {
		t.Error("manage_label tool not registered")
	}
}

func TestLabelList_RejectsNonStringFilters(t *testing.T) {
	t.Parallel()

	mustNotList := func(t *testing.T, c *UniversalMockClient) {
		t.Helper()
		c.GetLabelRegistryFn = func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
			t.Errorf("HA must not be called for an invalid filter")
			return nil, nil
		}
	}

	runFilterTypeCases(t, NewLabelHandlers().handleManageLabel, []filterTypeCase{
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
