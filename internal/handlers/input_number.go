// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// InputNumberHandlers provides MCP tool handlers for input_number helper operations.
type InputNumberHandlers struct{}

// NewInputNumberHandlers creates a new InputNumberHandlers instance.
func NewInputNumberHandlers() *InputNumberHandlers {
	return &InputNumberHandlers{}
}

// RegisterTools registers all input_number-related tools with the registry.
func (h *InputNumberHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createInputNumberTool(), h.handleCreateInputNumber)
	registry.RegisterTool(h.deleteInputNumberTool(), h.handleDeleteInputNumber)
	registry.RegisterTool(h.setInputNumberValueTool(), h.handleSetInputNumberValue)
}

func (h *InputNumberHandlers) createInputNumberTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_input_number",
		Description: "Create a new input_number helper in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input number configuration",
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
					Description: "Icon for the helper (e.g., mdi:counter)",
				},
				"min": {
					Type:        "number",
					Description: "Minimum value",
				},
				"max": {
					Type:        "number",
					Description: "Maximum value",
				},
				"step": {
					Type:        "number",
					Description: "Step value for incrementing/decrementing",
				},
				"initial": {
					Type:        "number",
					Description: "Initial value",
				},
				"mode": {
					Type:        "string",
					Description: "Display mode: 'box' or 'slider'",
				},
				"unit_of_measurement": {
					Type:        "string",
					Description: "Unit of measurement (e.g., °C, %, kWh)",
				},
			},
			Required: []string{"id", "name"},
		},
	}
}

func (h *InputNumberHandlers) deleteInputNumberTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_input_number",
		Description: "Delete an input_number helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input number entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_number (e.g., input_number.my_value)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *InputNumberHandlers) setInputNumberValueTool() mcp.Tool {
	return mcp.Tool{
		Name:        "set_input_number_value",
		Description: "Set the value of an input_number helper",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input number entity ID and new value",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_number (e.g., input_number.my_value)",
				},
				"value": {
					Type:        "number",
					Description: "New value for the input_number",
				},
			},
			Required: []string{"entity_id", "value"},
		},
	}
}

func (h *InputNumberHandlers) handleCreateInputNumber(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	config := buildInputNumberHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: "input_number",
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating input_number: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("input_number.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input number '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *InputNumberHandlers) handleDeleteInputNumber(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "input_number" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_number entity (e.g., input_number.my_value)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting input_number: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input number '%s' deleted successfully", entityID))},
	}, nil
}

func (h *InputNumberHandlers) handleSetInputNumberValue(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "input_number" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_number entity (e.g., input_number.my_value)")},
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
		"value":     value,
	}

	if _, err := client.CallService(ctx, "input_number", "set_value", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error setting input_number value: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input number '%s' value set to %v successfully", entityID, value))},
	}, nil
}

// buildInputNumberHelperConfig builds the configuration map for an input_number helper.
func buildInputNumberHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	if minVal, ok := args["min"].(float64); ok {
		config["min"] = minVal
	}

	if maxVal, ok := args["max"].(float64); ok {
		config["max"] = maxVal
	}

	if step, ok := args["step"].(float64); ok {
		config["step"] = step
	}

	if initial, ok := args["initial"].(float64); ok {
		config["initial"] = initial
	}

	if mode, ok := args["mode"].(string); ok && mode != "" {
		config["mode"] = mode
	}

	if unit, ok := args["unit_of_measurement"].(string); ok && unit != "" {
		config["unit_of_measurement"] = unit
	}

	return config
}
