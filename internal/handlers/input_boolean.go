// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// InputBooleanHandlers provides MCP tool handlers for input_boolean helper operations.
type InputBooleanHandlers struct{}

// NewInputBooleanHandlers creates a new InputBooleanHandlers instance.
func NewInputBooleanHandlers() *InputBooleanHandlers {
	return &InputBooleanHandlers{}
}

// RegisterTools registers all input_boolean-related tools with the registry.
func (h *InputBooleanHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createInputBooleanTool(), h.handleCreateInputBoolean)
	registry.RegisterTool(h.deleteInputBooleanTool(), h.handleDeleteInputBoolean)
	registry.RegisterTool(h.toggleInputBooleanTool(), h.handleToggleInputBoolean)
}

func (h *InputBooleanHandlers) createInputBooleanTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_input_boolean",
		Description: "Create a new input_boolean helper in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input boolean configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the helper",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the helper (e.g., mdi:lightbulb)",
				},
				"initial": {
					Type:        "boolean",
					Description: "Initial value (true or false)",
				},
			},
			Required: []string{"id", "name"},
		},
	}
}

func (h *InputBooleanHandlers) deleteInputBooleanTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_input_boolean",
		Description: "Delete an input_boolean helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input boolean entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_boolean (e.g., input_boolean.my_switch)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *InputBooleanHandlers) toggleInputBooleanTool() mcp.Tool {
	return mcp.Tool{
		Name:        "toggle_input_boolean",
		Description: "Toggle an input_boolean helper (switch between on and off)",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input boolean entity ID to toggle",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_boolean (e.g., input_boolean.my_switch)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *InputBooleanHandlers) handleCreateInputBoolean(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	config := buildInputBooleanHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: "input_boolean",
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating input_boolean: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("input_boolean.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input boolean '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *InputBooleanHandlers) handleDeleteInputBoolean(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "input_boolean" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_boolean entity (e.g., input_boolean.my_switch)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting input_boolean: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input boolean '%s' deleted successfully", entityID))},
	}, nil
}

func (h *InputBooleanHandlers) handleToggleInputBoolean(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "input_boolean" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_boolean entity (e.g., input_boolean.my_switch)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	if _, err := client.CallService(ctx, "input_boolean", "toggle", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error toggling input_boolean: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input boolean '%s' toggled successfully", entityID))},
	}, nil
}

// buildInputBooleanHelperConfig builds the configuration map for an input_boolean helper.
func buildInputBooleanHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	if initial, ok := args["initial"].(bool); ok {
		config["initial"] = initial
	}

	return config
}
