// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// InputTextHandlers provides MCP tool handlers for input_text helper operations.
type InputTextHandlers struct{}

// NewInputTextHandlers creates a new InputTextHandlers instance.
func NewInputTextHandlers() *InputTextHandlers {
	return &InputTextHandlers{}
}

// RegisterTools registers all input_text-related tools with the registry.
func (h *InputTextHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createInputTextTool(), h.handleCreateInputText)
	registry.RegisterTool(h.deleteInputTextTool(), h.handleDeleteInputText)
	registry.RegisterTool(h.setInputTextValueTool(), h.handleSetInputTextValue)
}

func (h *InputTextHandlers) createInputTextTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_input_text",
		Description: "Create a new input_text helper in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input text configuration",
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
					Description: "Icon for the helper (e.g., mdi:text)",
				},
				"min": {
					Type:        "number",
					Description: "Minimum length of text",
				},
				"max": {
					Type:        "number",
					Description: "Maximum length of text",
				},
				"initial": {
					Type:        "string",
					Description: "Initial value",
				},
				"mode": {
					Type:        "string",
					Description: "Display mode: 'text' or 'password'",
				},
				"pattern": {
					Type:        "string",
					Description: "Regex pattern for validation",
				},
			},
			Required: []string{"id", "name"},
		},
	}
}

func (h *InputTextHandlers) deleteInputTextTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_input_text",
		Description: "Delete an input_text helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input text entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_text (e.g., input_text.my_text)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *InputTextHandlers) setInputTextValueTool() mcp.Tool {
	return mcp.Tool{
		Name:        "set_input_text_value",
		Description: "Set the value of an input_text helper",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input text entity ID and new value",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_text (e.g., input_text.my_text)",
				},
				"value": {
					Type:        "string",
					Description: "New text value for the input_text",
				},
			},
			Required: []string{"entity_id", "value"},
		},
	}
}

func (h *InputTextHandlers) handleCreateInputText(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	config := buildInputTextHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: "input_text",
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating input_text: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("input_text.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input text '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *InputTextHandlers) handleDeleteInputText(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "input_text" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_text entity (e.g., input_text.my_text)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting input_text: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input text '%s' deleted successfully", entityID))},
	}, nil
}

func (h *InputTextHandlers) handleSetInputTextValue(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "input_text" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_text entity (e.g., input_text.my_text)")},
			IsError: true,
		}, nil
	}

	value, ok := args["value"].(string)
	if !ok {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("value is required and must be a string")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"value":     value,
	}

	if _, err := client.CallService(ctx, "input_text", "set_value", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error setting input_text value: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input text '%s' value set successfully", entityID))},
	}, nil
}

// buildInputTextHelperConfig builds the configuration map for an input_text helper.
func buildInputTextHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	if minVal, ok := args["min"].(float64); ok {
		config["min"] = int(minVal)
	}

	if maxVal, ok := args["max"].(float64); ok {
		config["max"] = int(maxVal)
	}

	if initial, ok := args["initial"].(string); ok && initial != "" {
		config["initial"] = initial
	}

	if mode, ok := args["mode"].(string); ok && mode != "" {
		config["mode"] = mode
	}

	if pattern, ok := args["pattern"].(string); ok && pattern != "" {
		config["pattern"] = pattern
	}

	return config
}
