// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const platformInputButton = "input_button"

// InputButtonHandlers provides MCP tool handlers for input_button helper operations.
type InputButtonHandlers struct{}

// NewInputButtonHandlers creates a new InputButtonHandlers instance.
func NewInputButtonHandlers() *InputButtonHandlers {
	return &InputButtonHandlers{}
}

// RegisterTools registers all input_button-related tools with the registry.
func (h *InputButtonHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createInputButtonTool(), h.handleCreateInputButton)
	registry.RegisterTool(h.deleteInputButtonTool(), h.handleDeleteInputButton)
	registry.RegisterTool(h.pressInputButtonTool(), h.handlePressInputButton)
}

func (h *InputButtonHandlers) createInputButtonTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_input_button",
		Description: "Create a new input_button helper in Home Assistant. An input button is a virtual button that can be pressed to trigger automations.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input button configuration",
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
					Description: "Icon for the helper (e.g., mdi:gesture-tap-button)",
				},
			},
			Required: []string{"id", "name"},
		},
	}
}

func (h *InputButtonHandlers) deleteInputButtonTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_input_button",
		Description: "Delete an input_button helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input button entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_button (e.g., input_button.my_button)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *InputButtonHandlers) pressInputButtonTool() mcp.Tool {
	return mcp.Tool{
		Name:        "press_input_button",
		Description: "Press an input_button helper. This triggers any automations listening for this button press.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input button entity ID to press",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_button (e.g., input_button.my_button)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *InputButtonHandlers) handleCreateInputButton(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	config := buildInputButtonHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: platformInputButton,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating input_button: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("%s.%s", platformInputButton, id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input button '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *InputButtonHandlers) handleDeleteInputButton(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformInputButton {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_button entity (e.g., input_button.my_button)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting input_button: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input button '%s' deleted successfully", entityID))},
	}, nil
}

func (h *InputButtonHandlers) handlePressInputButton(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformInputButton {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_button entity (e.g., input_button.my_button)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	if _, err := client.CallService(ctx, platformInputButton, "press", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error pressing input_button: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input button '%s' pressed successfully", entityID))},
	}, nil
}

// buildInputButtonHelperConfig builds the configuration map for an input_button helper.
func buildInputButtonHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	return config
}
