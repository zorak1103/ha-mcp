package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TestManageTodoSchema verifies the schema for manage_todo tool.
func TestManageTodoSchema(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterTodoTools(registry)

	tool, exists := registry.GetTool("manage_todo")
	if !exists {
		t.Fatal("manage_todo tool not registered")
	}

	// Verify basic properties
	if tool.Name != "manage_todo" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_todo")
	}

	// Verify schema
	schema := tool.InputSchema
	props := schema.Properties

	// Check action field
	actionSchema, ok := props["action"]
	if !ok {
		t.Fatal("action property missing from schema")
	}
	if len(actionSchema.Enum) != 5 {
		t.Errorf("action enum count = %d, want 5 (list, get_items, add_item, update_item, remove_item)", len(actionSchema.Enum))
	}

	// Check status enum
	statusSchema, ok := props["status"]
	if !ok {
		t.Fatal("status property missing from schema")
	}
	if len(statusSchema.Enum) != 2 {
		t.Errorf("status enum count = %d, want 2 (needs_action, completed)", len(statusSchema.Enum))
	}
}

// TestManageTodo_List verifies list action.
func TestManageTodo_List(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetStatesFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{
				{
					EntityID: "todo.shopping_list",
					State:    "5",
					Attributes: map[string]any{
						"friendly_name": "Shopping List",
					},
				},
				{
					EntityID: "todo.tasks",
					State:    "2",
					Attributes: map[string]any{
						"friendly_name": "Tasks",
					},
				},
			}, nil
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action": "list",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Shopping List") {
		t.Errorf("result text does not contain 'Shopping List': %s", text)
	}
}

// TestManageTodo_GetItems verifies get_items action.
func TestManageTodo_GetItems(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceWithResponseFn: func(_ context.Context, domain, service string, _ map[string]any) (map[string]any, error) {
			if domain != "todo" {
				return nil, fmt.Errorf("wrong domain: %s", domain)
			}
			if service != "get_items" {
				return nil, fmt.Errorf("wrong service: %s", service)
			}
			return map[string]any{
				"todo.shopping_list": map[string]any{
					"items": []any{
						map[string]any{
							"uid":     "item1",
							"summary": "Buy milk",
							"status":  "needs_action",
						},
						map[string]any{
							"uid":     "item2",
							"summary": "Buy eggs",
							"status":  "completed",
						},
					},
				},
			}, nil
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action":    "get_items",
		"entity_id": "todo.shopping_list",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Buy milk") {
		t.Errorf("result text does not contain 'Buy milk': %s", text)
	}
}

// TestManageTodo_GetItemsStatusFilter verifies status_filter parameter.
func TestManageTodo_GetItemsStatusFilter(t *testing.T) {
	t.Parallel()

	var capturedData map[string]any
	client := &UniversalMockClient{
		CallServiceWithResponseFn: func(_ context.Context, domain, service string, data map[string]any) (map[string]any, error) {
			if domain != "todo" {
				return nil, fmt.Errorf("wrong domain: %s", domain)
			}
			if service != "get_items" {
				return nil, fmt.Errorf("wrong service: %s", service)
			}
			capturedData = data
			return map[string]any{
				"todo.tasks": map[string]any{
					"items": []any{},
				},
			}, nil
		},
	}

	handler := NewTodoHandlers()
	_, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action":        "get_items",
		"entity_id":     "todo.tasks",
		"status_filter": "needs_action",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify status parameter was passed
	if capturedData["status"] != "needs_action" {
		t.Errorf("status = %v, want 'needs_action'", capturedData["status"])
	}
}

// TestManageTodo_AddItem verifies add_item action.
func TestManageTodo_AddItem(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "todo" {
				return nil, fmt.Errorf("wrong domain: %s", domain)
			}
			if service != "add_item" {
				return nil, fmt.Errorf("wrong service: %s", service)
			}
			if data["entity_id"] != "todo.shopping_list" {
				return nil, fmt.Errorf("data[entity_id] = %v, want %q", data["entity_id"], "todo.shopping_list")
			}
			if data["item"] != "Buy bread" {
				return nil, fmt.Errorf("data[item] = %v, want %q", data["item"], "Buy bread")
			}
			return nil, nil
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action":    "add_item",
		"entity_id": "todo.shopping_list",
		"item":      "Buy bread",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}
}

// TestManageTodo_UpdateItem verifies update_item action.
func TestManageTodo_UpdateItem(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "todo" {
				return nil, fmt.Errorf("wrong domain: %s", domain)
			}
			if service != "update_item" {
				return nil, fmt.Errorf("wrong service: %s", service)
			}
			if data["entity_id"] != "todo.shopping_list" {
				return nil, fmt.Errorf("data[entity_id] = %v, want %q", data["entity_id"], "todo.shopping_list")
			}
			if data["item"] != "item1" {
				return nil, fmt.Errorf("data[item] = %v, want %q", data["item"], "item1")
			}
			return nil, nil
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action":    "update_item",
		"entity_id": "todo.shopping_list",
		"uid":       "item1",
		"status":    "completed",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}
}

// TestManageTodo_RemoveItem verifies remove_item action.
func TestManageTodo_RemoveItem(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "todo" {
				return nil, fmt.Errorf("wrong domain: %s", domain)
			}
			if service != "remove_item" {
				return nil, fmt.Errorf("wrong service: %s", service)
			}
			if data["entity_id"] != "todo.shopping_list" {
				return nil, fmt.Errorf("data[entity_id] = %v, want %q", data["entity_id"], "todo.shopping_list")
			}
			if data["item"] != "item1" {
				return nil, fmt.Errorf("data[item] = %v, want %q", data["item"], "item1")
			}
			return nil, nil
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action":    "remove_item",
		"entity_id": "todo.shopping_list",
		"uid":       "item1",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}
}

// TestManageTodo_MissingRequiredParams verifies validation for actions requiring parameters.
func TestManageTodo_MissingRequiredParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "get_items missing entity_id",
			args: map[string]any{
				"action": "get_items",
			},
		},
		{
			name: "add_item missing entity_id",
			args: map[string]any{
				"action": "add_item",
				"item":   "Test",
			},
		},
		{
			name: "add_item missing item",
			args: map[string]any{
				"action":    "add_item",
				"entity_id": "todo.test",
			},
		},
		{
			name: "update_item missing uid",
			args: map[string]any{
				"action":    "update_item",
				"entity_id": "todo.test",
			},
		},
		{
			name: "remove_item missing uid",
			args: map[string]any{
				"action":    "remove_item",
				"entity_id": "todo.test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{}
			handler := NewTodoHandlers()

			result, err := handler.HandleManageTodo(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil || !result.IsError {
				t.Error("expected error result")
			}
		})
	}
}

// TestManageTodo_List_JSONFormat verifies list action with JSON format.
func TestManageTodo_List_JSONFormat(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetStatesFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{
				{
					EntityID: "todo.shopping_list",
					State:    "3",
					Attributes: map[string]any{
						"friendly_name": "Shopping List",
					},
				},
			}, nil
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action": "list",
		"format": "json",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "todo.shopping_list") {
		t.Errorf("JSON result does not contain entity_id: %s", text)
	}
}

// TestManageTodo_List_Error verifies list action handles client error.
func TestManageTodo_List_Error(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetStatesFn: func(context.Context) ([]homeassistant.Entity, error) {
			return nil, fmt.Errorf("connection failed")
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action": "list",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageTodo_GetItems_Error verifies get_items handles client error.
func TestManageTodo_GetItems_Error(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceWithResponseFn: func(context.Context, string, string, map[string]any) (map[string]any, error) {
			return nil, fmt.Errorf("service call failed")
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action":    "get_items",
		"entity_id": "todo.shopping_list",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageTodo_GetItems_JSONFormat verifies get_items JSON format path.
func TestManageTodo_GetItems_JSONFormat(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceWithResponseFn: func(context.Context, string, string, map[string]any) (map[string]any, error) {
			return map[string]any{
				"todo.tasks": map[string]any{
					"items": []any{
						map[string]any{"uid": "item1", "summary": "Task 1", "status": "needs_action"},
					},
				},
			}, nil
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action":    "get_items",
		"entity_id": "todo.tasks",
		"format":    "json",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "item1") {
		t.Errorf("JSON result does not contain uid: %s", text)
	}
}

// TestManageTodo_UpdateItem_Error verifies update_item handles client error.
func TestManageTodo_UpdateItem_Error(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceFn: func(context.Context, string, string, map[string]any) ([]homeassistant.Entity, error) {
			return nil, fmt.Errorf("update failed")
		},
	}

	handler := NewTodoHandlers()
	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action":    "update_item",
		"entity_id": "todo.tasks",
		"uid":       "item1",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageTodo_UpdateItem_MissingEntityID verifies update_item validation.
func TestManageTodo_UpdateItem_MissingEntityID(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewTodoHandlers()

	result, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action": "update_item",
		"uid":    "item1",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageTodo_AddItem_WithOptionalFields verifies add_item with description and due date.
func TestManageTodo_AddItem_WithOptionalFields(t *testing.T) {
	t.Parallel()

	var capturedData map[string]any
	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "todo" || service != "add_item" {
				return nil, fmt.Errorf("wrong call: %s.%s", domain, service)
			}
			capturedData = data
			return nil, nil
		},
	}

	handler := NewTodoHandlers()
	_, err := handler.HandleManageTodo(context.Background(), client, map[string]any{
		"action":      "add_item",
		"entity_id":   "todo.tasks",
		"item":        "Buy groceries",
		"description": "Weekly groceries",
		"due_date":    "2024-01-20",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedData["description"] != "Weekly groceries" {
		t.Errorf("description = %v, want 'Weekly groceries'", capturedData["description"])
	}
	if capturedData["due_date"] != "2024-01-20" {
		t.Errorf("due_date = %v, want '2024-01-20'", capturedData["due_date"])
	}
}

// TestExtractTodoItems_EmptyResponse verifies extractTodoItems with mismatched entity key.
func TestExtractTodoItems_EmptyResponse(t *testing.T) {
	t.Parallel()

	// Response doesn't match entity_id
	response := map[string]any{
		"todo.other_list": map[string]any{
			"items": []any{map[string]any{"uid": "item1"}},
		},
	}

	items := extractTodoItems(response, "todo.shopping_list")
	if len(items) != 0 {
		t.Errorf("extractTodoItems() = %d items, want 0 (key mismatch)", len(items))
	}
}
