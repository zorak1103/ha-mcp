package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Action constants for manage_todo tool.
const (
	todoActionList       = "list"
	todoActionGetItems   = "get_items"
	todoActionAddItem    = "add_item"
	todoActionUpdateItem = "update_item"
	todoActionRemoveItem = "remove_item"

	todoDomain = "todo"
)

// TodoHandlers provides handlers for todo list operations.
type TodoHandlers struct{}

// NewTodoHandlers creates a new todo handlers instance.
func NewTodoHandlers() *TodoHandlers {
	return &TodoHandlers{}
}

// RegisterTodoTools registers todo-related tools with the MCP registry.
func RegisterTodoTools(registry *mcp.Registry) {
	handler := NewTodoHandlers()
	registry.RegisterTool(buildManageTodoTool(), handler.HandleManageTodo)
}

// buildManageTodoTool builds the schema for manage_todo tool.
func buildManageTodoTool() mcp.Tool {
	return mcp.Tool{
		Name:        "manage_todo",
		Description: "Manage Home Assistant todo and shopping lists. Supports list (view todo lists), get_items (view list items), add_item, update_item, and remove_item.",
		InputSchema: mcp.JSONSchema{
			Type:       "object",
			Properties: buildTodoSchemaProperties(),
			Required:   []string{"action"},
		},
	}
}

// buildTodoSchemaProperties builds the properties map for manage_todo schema.
func buildTodoSchemaProperties() map[string]mcp.JSONSchema {
	return map[string]mcp.JSONSchema{
		"action": {
			Type:        "string",
			Description: "Action to perform: 'list', 'get_items', 'add_item', 'update_item', or 'remove_item'.",
			Enum:        []string{todoActionList, todoActionGetItems, todoActionAddItem, todoActionUpdateItem, todoActionRemoveItem},
		},
		attrEntityID: {
			Type:        "string",
			Description: "Todo list entity ID (required for get_items, add_item, update_item, remove_item, e.g., 'todo.shopping_list').",
		},
		"item": {
			Type:        "string",
			Description: "Item summary text (required for 'add_item').",
		},
		"uid": {
			Type:        "string",
			Description: "Item unique ID (required for 'update_item' and 'remove_item').",
		},
		"rename": {
			Type:        "string",
			Description: "New item summary text (for 'update_item').",
		},
		"status": {
			Type:        "string",
			Description: "Item status: 'needs_action' or 'completed' (for 'update_item').",
			Enum:        []string{"needs_action", "completed"},
		},
		"description": {
			Type:        "string",
			Description: "Item description (for 'add_item' or 'update_item').",
		},
		"due_date": {
			Type:        "string",
			Description: "Due date in YYYY-MM-DD format (for 'add_item' or 'update_item').",
		},
		"due_datetime": {
			Type:        "string",
			Description: "Due datetime in ISO 8601 format (for 'add_item' or 'update_item').",
		},
		"status_filter": {
			Type:        "string",
			Description: "Filter items by status: 'needs_action' or 'completed' (for 'get_items').",
			Enum:        []string{"needs_action", "completed"},
		},
		"format": {
			Type:        "string",
			Description: "Output format: 'natural' (default, human-readable) or 'json' (structured JSON).",
			Enum:        []string{formatNatural, formatJSON},
			Default:     formatNatural,
		},
	}
}

// HandleManageTodo handles the manage_todo tool invocation.
func (h *TodoHandlers) HandleManageTodo(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	// Extract action
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action parameter is required and must be 'list', 'get_items', 'add_item', 'update_item', or 'remove_item'"), nil
	}

	// Extract format (default: natural)
	format, _ := args["format"].(string)
	if format == "" {
		format = formatNatural
	}

	// Route to action handler
	switch action {
	case todoActionList:
		return h.handleListTodos(ctx, client, args, format)
	case todoActionGetItems:
		return h.handleGetItems(ctx, client, args, format)
	case todoActionAddItem:
		return h.handleAddItem(ctx, client, args, format)
	case todoActionUpdateItem:
		return h.handleUpdateItem(ctx, client, args, format)
	case todoActionRemoveItem:
		return h.handleRemoveItem(ctx, client, args, format)
	default:
		return errorResult(fmt.Sprintf("invalid action %q, must be one of: list, get_items, add_item, update_item, remove_item", action)), nil
	}
}

// handleListTodos lists all todo list entities.
func (h *TodoHandlers) handleListTodos(ctx context.Context, client homeassistant.Client, _ map[string]any, format string) (*mcp.ToolsCallResult, error) {
	// Get all states
	states, err := client.GetStates(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get states: %v", err)), nil
	}

	// Filter for todo domain
	var todos []homeassistant.Entity
	for _, state := range states {
		if strings.HasPrefix(state.EntityID, todoDomain+".") {
			todos = append(todos, state)
		}
	}

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(todos, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal todos: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format
	return successResult(h.formatTodosNatural(todos)), nil
}

// handleGetItems retrieves items from a todo list.
func (h *TodoHandlers) handleGetItems(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	// Validate entity_id
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for get_items action"), nil
	}

	// Build service data
	data := map[string]any{
		attrEntityID: entityID,
	}

	// Add optional status filter
	if statusFilter, ok := args["status_filter"].(string); ok && statusFilter != "" {
		data["status"] = statusFilter
	}

	// Call todo.get_items service with return_response
	response, err := client.CallServiceWithResponse(ctx, todoDomain, "get_items", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get todo items: %v", err)), nil
	}

	// Parse response: {"todo.shopping_list": {"items": [...]}}
	items := extractTodoItems(response, entityID)

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal items: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format
	return successResult(h.formatItemsNatural(items)), nil
}

// handleAddItem adds a new item to a todo list.
func (h *TodoHandlers) handleAddItem(ctx context.Context, client homeassistant.Client, args map[string]any, _ string) (*mcp.ToolsCallResult, error) {
	// Validate required parameters
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for add_item action"), nil
	}

	item, _ := args["item"].(string)
	if item == "" {
		return errorResult("item is required for add_item action"), nil
	}

	// Build service data
	data := map[string]any{
		attrEntityID: entityID,
		"item":       item,
	}

	// Add optional parameters
	if desc, ok := args["description"].(string); ok && desc != "" {
		data["description"] = desc
	}
	if dueDate, ok := args["due_date"].(string); ok && dueDate != "" {
		data["due_date"] = dueDate
	}
	if dueDatetime, ok := args["due_datetime"].(string); ok && dueDatetime != "" {
		data["due_datetime"] = dueDatetime
	}

	// Call todo.add_item service
	_, err := client.CallService(ctx, todoDomain, "add_item", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to add item: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Item added to %s: %s", entityID, item)), nil
}

// handleUpdateItem updates an existing todo item.
func (h *TodoHandlers) handleUpdateItem(ctx context.Context, client homeassistant.Client, args map[string]any, _ string) (*mcp.ToolsCallResult, error) {
	// Validate required parameters
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for update_item action"), nil
	}

	uid, _ := args["uid"].(string)
	if uid == "" {
		return errorResult("uid is required for update_item action"), nil
	}

	// Build service data
	data := map[string]any{
		attrEntityID: entityID,
		"item":       uid,
	}

	// Add optional update fields
	if rename, ok := args["rename"].(string); ok && rename != "" {
		data["rename"] = rename
	}
	if status, ok := args["status"].(string); ok && status != "" {
		data["status"] = status
	}
	if desc, ok := args["description"].(string); ok && desc != "" {
		data["description"] = desc
	}
	if dueDate, ok := args["due_date"].(string); ok && dueDate != "" {
		data["due_date"] = dueDate
	}
	if dueDatetime, ok := args["due_datetime"].(string); ok && dueDatetime != "" {
		data["due_datetime"] = dueDatetime
	}

	// Call todo.update_item service
	_, err := client.CallService(ctx, todoDomain, "update_item", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to update item: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Item updated in %s (uid: %s)", entityID, uid)), nil
}

// handleRemoveItem removes an item from a todo list.
func (h *TodoHandlers) handleRemoveItem(ctx context.Context, client homeassistant.Client, args map[string]any, _ string) (*mcp.ToolsCallResult, error) {
	// Validate required parameters
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for remove_item action"), nil
	}

	uid, _ := args["uid"].(string)
	if uid == "" {
		return errorResult("uid is required for remove_item action"), nil
	}

	// Build service data
	data := map[string]any{
		attrEntityID: entityID,
		"item":       uid,
	}

	// Call todo.remove_item service
	_, err := client.CallService(ctx, todoDomain, "remove_item", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to remove item: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Item removed from %s (uid: %s)", entityID, uid)), nil
}

// formatTodosNatural formats todo list entities in natural language.
func (h *TodoHandlers) formatTodosNatural(todos []homeassistant.Entity) string {
	if len(todos) == 0 {
		return "No todo lists found."
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d todo list(s):", len(todos)))

	for i, todo := range todos {
		name := getMapString(todo.Attributes, "friendly_name", todo.EntityID)
		itemCount := todo.State

		parts = append(parts,
			fmt.Sprintf("\n%d. %s", i+1, name),
			fmt.Sprintf("   Entity ID: %s", todo.EntityID),
			fmt.Sprintf("   Items: %s", itemCount))
	}

	return strings.Join(parts, "\n")
}

// formatItemsNatural formats todo items in checklist format.
func (h *TodoHandlers) formatItemsNatural(items []any) string {
	if len(items) == 0 {
		return "No items found."
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d item(s):", len(items)))

	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		uid := getMapString(itemMap, "uid", "")
		summary := getMapString(itemMap, "summary", "")
		status := getMapString(itemMap, "status", "")

		checkbox := "[ ]"
		if status == "completed" {
			checkbox = "[x]"
		}

		itemParts := []string{fmt.Sprintf("\n%s %s (uid: %s)", checkbox, summary, uid)}

		if desc := getMapString(itemMap, "description", ""); desc != "" {
			itemParts = append(itemParts, fmt.Sprintf("   Description: %s", desc))
		}
		if dueDate := getMapString(itemMap, "due", ""); dueDate != "" {
			itemParts = append(itemParts, fmt.Sprintf("   Due: %s", dueDate))
		}

		parts = append(parts, itemParts...)
	}

	return strings.Join(parts, "\n")
}

// extractTodoItems extracts items from the todo service response.
// Response format: {"todo.shopping_list": {"items": [...]}}
func extractTodoItems(response map[string]any, entityID string) []any {
	if entityData, ok := response[entityID].(map[string]any); ok {
		if items, ok := entityData["items"].([]any); ok {
			return items
		}
	}
	return []any{}
}
