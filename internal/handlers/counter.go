// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

const platformCounter = "counter"

// CounterHandlers provides MCP tool handlers for counter helper operations.
type CounterHandlers struct{}

// NewCounterHandlers creates a new CounterHandlers instance.
func NewCounterHandlers() *CounterHandlers {
	return &CounterHandlers{}
}

// RegisterTools registers all counter-related tools with the registry.
func (h *CounterHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createCounterTool(), h.handleCreateCounter)
	registry.RegisterTool(h.deleteCounterTool(), h.handleDeleteCounter)
	registry.RegisterTool(h.incrementCounterTool(), h.handleIncrementCounter)
	registry.RegisterTool(h.decrementCounterTool(), h.handleDecrementCounter)
	registry.RegisterTool(h.resetCounterTool(), h.handleResetCounter)
	registry.RegisterTool(h.setCounterValueTool(), h.handleSetCounterValue)
}

func (h *CounterHandlers) createCounterTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_counter",
		Description: "Create a new counter helper in Home Assistant. A counter can be incremented, decremented, and reset. Useful for tracking occurrences or counts.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Counter configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the helper",
				},
				"initial": {
					Type:        "number",
					Description: "Initial value when created or reset (default: 0)",
				},
				"step": {
					Type:        "number",
					Description: "Step value for increment/decrement operations (default: 1)",
				},
				"minimum": {
					Type:        "number",
					Description: "Minimum allowed value (optional, no limit if not set)",
				},
				"maximum": {
					Type:        "number",
					Description: "Maximum allowed value (optional, no limit if not set)",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the helper (e.g., mdi:counter)",
				},
			},
			Required: []string{"id", "name"},
		},
	}
}

func (h *CounterHandlers) deleteCounterTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_counter",
		Description: "Delete a counter helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Counter entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the counter (e.g., counter.my_counter)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *CounterHandlers) incrementCounterTool() mcp.Tool {
	return mcp.Tool{
		Name:        "increment_counter",
		Description: "Increment a counter by its step value",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Counter entity ID to increment",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the counter (e.g., counter.my_counter)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *CounterHandlers) decrementCounterTool() mcp.Tool {
	return mcp.Tool{
		Name:        "decrement_counter",
		Description: "Decrement a counter by its step value",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Counter entity ID to decrement",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the counter (e.g., counter.my_counter)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *CounterHandlers) resetCounterTool() mcp.Tool {
	return mcp.Tool{
		Name:        "reset_counter",
		Description: "Reset a counter to its initial value",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Counter entity ID to reset",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the counter (e.g., counter.my_counter)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *CounterHandlers) setCounterValueTool() mcp.Tool {
	return mcp.Tool{
		Name:        "set_counter_value",
		Description: "Set a counter to a specific value",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Counter entity ID and value to set",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the counter (e.g., counter.my_counter)",
				},
				"value": {
					Type:        "number",
					Description: "The value to set the counter to",
				},
			},
			Required: []string{"entity_id", "value"},
		},
	}
}

func (h *CounterHandlers) handleCreateCounter(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("id is required")},
			IsError: true,
		}, nil
	}

	name, _ := args["name"].(string)
	if name == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("name is required")},
			IsError: true,
		}, nil
	}

	config := buildCounterHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: platformCounter,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating counter: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("%s.%s", platformCounter, id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Counter '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *CounterHandlers) handleDeleteCounter(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformCounter {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a counter entity (e.g., counter.my_counter)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting counter: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Counter '%s' deleted successfully", entityID))},
	}, nil
}

func (h *CounterHandlers) handleIncrementCounter(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformCounter {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a counter entity (e.g., counter.my_counter)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	if _, err := client.CallService(ctx, platformCounter, "increment", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error incrementing counter: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Counter '%s' incremented successfully", entityID))},
	}, nil
}

func (h *CounterHandlers) handleDecrementCounter(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformCounter {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a counter entity (e.g., counter.my_counter)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	if _, err := client.CallService(ctx, platformCounter, "decrement", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error decrementing counter: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Counter '%s' decremented successfully", entityID))},
	}, nil
}

func (h *CounterHandlers) handleResetCounter(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformCounter {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a counter entity (e.g., counter.my_counter)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	if _, err := client.CallService(ctx, platformCounter, "reset", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error resetting counter: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Counter '%s' reset successfully", entityID))},
	}, nil
}

func (h *CounterHandlers) handleSetCounterValue(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformCounter {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a counter entity (e.g., counter.my_counter)")},
			IsError: true,
		}, nil
	}

	value, ok := args["value"].(float64)
	if !ok {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("value is required and must be a number")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"value":     int(value),
	}

	if _, err := client.CallService(ctx, platformCounter, "set_value", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error setting counter value: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Counter '%s' set to %d successfully", entityID, int(value)))},
	}, nil
}

// buildCounterHelperConfig builds the configuration map for a counter helper.
func buildCounterHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if initial, ok := args["initial"].(float64); ok {
		config["initial"] = int(initial)
	}

	if step, ok := args["step"].(float64); ok {
		config["step"] = int(step)
	}

	if minimum, ok := args["minimum"].(float64); ok {
		config["minimum"] = int(minimum)
	}

	if maximum, ok := args["maximum"].(float64); ok {
		config["maximum"] = int(maximum)
	}

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	return config
}
