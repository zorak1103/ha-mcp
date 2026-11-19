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

func TestPersonHandlers_ManagePersonToolSchema(t *testing.T) {
	t.Parallel()

	h := NewPersonHandlers()
	tool := h.managePersonTool()

	if tool.Name != "manage_person" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_person")
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

func TestManagePerson_List(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "list persons with natural format",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_john", Name: "John", UserID: "user_123", DeviceTrackers: []string{"device_tracker.john_phone"}},
						{ID: "person_jane", Name: "Jane", DeviceTrackers: []string{"device_tracker.jane_phone", "device_tracker.jane_watch"}},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 person(s)", "John", "Jane", "person_john", "User: user_123", "Device trackers"},
		},
		{
			name: "list persons with json format",
			args: map[string]any{
				"action": "list",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_1", Name: "Test Person"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"person_1", "Test Person"},
		},
		{
			name: "list persons empty",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No persons found"},
		},
		{
			name: "list persons with name filter",
			args: map[string]any{
				"action":        "list",
				"name_contains": "john",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_john", Name: "John"},
						{ID: "person_jane", Name: "Jane"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 1 person(s)", "John"},
		},
		{
			name: "list persons API error",
			args: map[string]any{
				"action": "list",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return nil, fmt.Errorf("connection failed")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "listing"},
		},
	}

	h := NewPersonHandlers()
	runHandlerTestCases(t, tests, h.handleManagePerson)
}

// =============================================================================
// Get Tests
// =============================================================================

func TestManagePerson_Get(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "get person with natural format",
			args: map[string]any{
				"action":    "get",
				"person_id": "person_john",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{
							ID:             "person_john",
							Name:           "John Doe",
							UserID:         "user_123",
							DeviceTrackers: []string{"device_tracker.john_phone"},
							Picture:        "/local/john.jpg",
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Person: John Doe", "ID: person_john", "User ID: user_123", "Device trackers: device_tracker.john_phone", "Picture: /local/john.jpg"},
		},
		{
			name: "get person with json format",
			args: map[string]any{
				"action":    "get",
				"person_id": "person_1",
				"format":    "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"person_1", "Test"},
		},
		{
			name: "get person not found",
			args: map[string]any{
				"action":    "get",
				"person_id": "nonexistent",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_1", Name: "Test"},
					}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"person not found", "nonexistent"},
		},
		{
			name: "get person without person_id",
			args: map[string]any{
				"action": "get",
			},
			wantError:    true,
			wantContains: []string{"person_id", "required"},
		},
	}

	h := NewPersonHandlers()
	runHandlerTestCases(t, tests, h.handleManagePerson)
}

// =============================================================================
// Create Tests
// =============================================================================

func TestManagePerson_Create(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "create person successfully",
			args: map[string]any{
				"action": "create",
				"name":   "Alice",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreatePersonFn = func(_ context.Context, config homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
					return &homeassistant.PersonRegistryEntry{
						ID:   "person_alice",
						Name: config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully", "Alice", "person_alice"},
		},
		{
			name: "create person with all fields",
			args: map[string]any{
				"action":          "create",
				"name":            "Bob",
				"user_id":         "user_456",
				"device_trackers": []any{"device_tracker.bob_phone"},
				"picture":         "/local/bob.jpg",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreatePersonFn = func(_ context.Context, config homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
					if config.Name != "Bob" || config.UserID != "user_456" {
						return nil, fmt.Errorf("unexpected config")
					}
					return &homeassistant.PersonRegistryEntry{
						ID:   "person_bob",
						Name: config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"created successfully"},
		},
		{
			name: "create person without name",
			args: map[string]any{
				"action": "create",
			},
			wantError:    true,
			wantContains: []string{"name", "required"},
		},
		{
			name: "create person API error",
			args: map[string]any{
				"action": "create",
				"name":   "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.CreatePersonFn = func(context.Context, homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
					return nil, fmt.Errorf("database error")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "creating"},
		},
	}

	h := NewPersonHandlers()
	runHandlerTestCases(t, tests, h.handleManagePerson)
}

// =============================================================================
// Update Tests
// =============================================================================

func TestManagePerson_Update(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "update person name",
			args: map[string]any{
				"action":    "update",
				"person_id": "person_1",
				"name":      "Updated Name",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_1", Name: "Test Person"},
						{ID: "person_2", Name: "Test Person 2"},
					}, nil
				}
				m.UpdatePersonFn = func(_ context.Context, personID string, config homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
					if personID != "person_1" {
						return nil, fmt.Errorf("unexpected person_id: %s", personID)
					}
					return &homeassistant.PersonRegistryEntry{
						ID:   personID,
						Name: config.Name,
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully", "Updated Name"},
		},
		{
			name: "update person device trackers",
			args: map[string]any{
				"action":          "update",
				"person_id":       "person_2",
				"device_trackers": []any{"device_tracker.new_phone"},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_1", Name: "Test Person"},
						{ID: "person_2", Name: "Test"},
					}, nil
				}
				m.UpdatePersonFn = func(_ context.Context, personID string, _ homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
					return &homeassistant.PersonRegistryEntry{
						ID:   personID,
						Name: "Test",
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"updated successfully"},
		},
		{
			name: "update person without person_id",
			args: map[string]any{
				"action": "update",
				"name":   "New Name",
			},
			wantError:    true,
			wantContains: []string{"person_id", "required"},
		},
		{
			name: "update person API error",
			args: map[string]any{
				"action":    "update",
				"person_id": "person_1",
				"name":      "Test",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_1", Name: "Test Person"},
					}, nil
				}
				m.UpdatePersonFn = func(context.Context, string, homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
					return nil, fmt.Errorf("not found")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "updating"},
		},
	}

	h := NewPersonHandlers()
	runHandlerTestCases(t, tests, h.handleManagePerson)
}

// =============================================================================
// Delete Tests
// =============================================================================

func TestManagePerson_Delete(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "delete person successfully",
			args: map[string]any{
				"action":    "delete",
				"person_id": "person_old",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_old", Name: "Old Person"},
						{ID: "person_new", Name: "New Person"},
					}, nil
				}
				m.DeletePersonFn = func(_ context.Context, personID string) error {
					if personID != "person_old" {
						return fmt.Errorf("unexpected person_id: %s", personID)
					}
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"deleted successfully", "person_old"},
		},
		{
			name: "delete person without person_id",
			args: map[string]any{
				"action": "delete",
			},
			wantError:    true,
			wantContains: []string{"person_id", "required"},
		},
		{
			name: "delete person API error",
			args: map[string]any{
				"action":    "delete",
				"person_id": "person_1",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
					return []homeassistant.PersonRegistryEntry{
						{ID: "person_1", Name: "Test Person"},
					}, nil
				}
				m.DeletePersonFn = func(context.Context, string) error {
					return fmt.Errorf("person linked to user")
				}
			},
			wantError:    true,
			wantContains: []string{"error", "deleting"},
		},
	}

	h := NewPersonHandlers()
	runHandlerTestCases(t, tests, h.handleManagePerson)
}

// =============================================================================
// Validation Tests
// =============================================================================

func TestManagePerson_ValidationErrors(t *testing.T) {
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

	h := NewPersonHandlers()
	runHandlerTestCases(t, tests, h.handleManagePerson)
}

// =============================================================================
// Registration Tests
// =============================================================================

func TestPersonHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewPersonHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	// Verify manage_person is registered
	tools := registry.ListTools()
	found := false

	for _, tool := range tools {
		if tool.Name == "manage_person" {
			found = true
			break
		}
	}

	if !found {
		t.Error("manage_person tool not registered")
	}
}

func TestPersonList_RejectsNonStringFilters(t *testing.T) {
	t.Parallel()

	mustNotList := func(t *testing.T, c *UniversalMockClient) {
		t.Helper()
		c.GetPersonsFn = func(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
			t.Errorf("HA must not be called for an invalid filter")
			return nil, nil
		}
	}

	runFilterTypeCases(t, NewPersonHandlers().handleManagePerson, []filterTypeCase{
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
