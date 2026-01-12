// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const platformInputSelect = "input_select"

// InputSelectHandlers provides MCP tool handlers for input_select helper operations.
type InputSelectHandlers struct{}

// NewInputSelectHandlers creates a new InputSelectHandlers instance.
func NewInputSelectHandlers() *InputSelectHandlers {
	return &InputSelectHandlers{}
}

// RegisterTools registers all input_select-related tools with the registry.
func (h *InputSelectHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createInputSelectTool(), h.handleCreateInputSelect)
	registry.RegisterTool(h.deleteInputSelectTool(), h.handleDeleteInputSelect)
	registry.RegisterTool(h.selectOptionTool(), h.handleSelectOption)
	registry.RegisterTool(h.setOptionsTool(), h.handleSetOptions)
}

func (h *InputSelectHandlers) createInputSelectTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_input_select",
		Description: "Create a new input_select helper in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input select configuration",
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
					Description: "Icon for the helper (e.g., mdi:format-list-bulleted)",
				},
				"options": {
					Type:        "array",
					Description: "List of selectable options",
				},
				"initial": {
					Type:        "string",
					Description: "Initial selected option",
				},
			},
			Required: []string{"id", "name", "options"},
		},
	}
}

func (h *InputSelectHandlers) deleteInputSelectTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_input_select",
		Description: "Delete an input_select helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input select entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_select (e.g., input_select.my_dropdown)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *InputSelectHandlers) selectOptionTool() mcp.Tool {
	return mcp.Tool{
		Name:        "select_option",
		Description: "Select an option in an input_select helper",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input select entity ID and option to select",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_select (e.g., input_select.my_dropdown)",
				},
				"option": {
					Type:        "string",
					Description: "The option to select",
				},
			},
			Required: []string{"entity_id", "option"},
		},
	}
}

func (h *InputSelectHandlers) setOptionsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "set_options",
		Description: "Set the available options for an input_select helper",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input select entity ID and new options list",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_select (e.g., input_select.my_dropdown)",
				},
				"options": {
					Type:        "array",
					Description: "New list of selectable options",
				},
			},
			Required: []string{"entity_id", "options"},
		},
	}
}

func (h *InputSelectHandlers) handleCreateInputSelect(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	options, ok := args["options"].([]any)
	if !ok || len(options) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("options is required and must be a non-empty array")},
			IsError: true,
		}, nil
	}

	config := buildInputSelectHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: platformInputSelect,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating input_select: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("input_select.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input select '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *InputSelectHandlers) handleDeleteInputSelect(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformInputSelect {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_select entity (e.g., input_select.my_dropdown)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting input_select: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input select '%s' deleted successfully", entityID))},
	}, nil
}

func (h *InputSelectHandlers) handleSelectOption(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformInputSelect {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_select entity (e.g., input_select.my_dropdown)")},
			IsError: true,
		}, nil
	}

	option, ok := args["option"].(string)
	if !ok || option == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("option is required")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"option":    option,
	}

	if _, err := client.CallService(ctx, platformInputSelect, "select_option", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error selecting option: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Option '%s' selected successfully for '%s'", option, entityID))},
	}, nil
}

func (h *InputSelectHandlers) handleSetOptions(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformInputSelect {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_select entity (e.g., input_select.my_dropdown)")},
			IsError: true,
		}, nil
	}

	options, ok := args["options"].([]any)
	if !ok || len(options) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("options is required and must be a non-empty array")},
			IsError: true,
		}, nil
	}

	// Convert options to string slice
	strOptions := make([]string, 0, len(options))
	for _, opt := range options {
		if s, ok := opt.(string); ok {
			strOptions = append(strOptions, s)
		}
	}

	if len(strOptions) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("options must contain at least one string value")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"options":   strOptions,
	}

	if _, err := client.CallService(ctx, platformInputSelect, "set_options", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error setting options: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Options updated successfully for '%s'", entityID))},
	}, nil
}

// buildInputSelectHelperConfig builds the configuration map for an input_select helper.
func buildInputSelectHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	if options, ok := args["options"].([]any); ok && len(options) > 0 {
		strOptions := make([]string, 0, len(options))
		for _, opt := range options {
			if s, ok := opt.(string); ok {
				strOptions = append(strOptions, s)
			}
		}
		config["options"] = strOptions
	}

	if initial, ok := args["initial"].(string); ok && initial != "" {
		config["initial"] = initial
	}

	return config
}
