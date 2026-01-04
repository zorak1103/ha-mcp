// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// InputDatetimeHandlers provides MCP tool handlers for input_datetime helper operations.
type InputDatetimeHandlers struct{}

// NewInputDatetimeHandlers creates a new InputDatetimeHandlers instance.
func NewInputDatetimeHandlers() *InputDatetimeHandlers {
	return &InputDatetimeHandlers{}
}

// RegisterTools registers all input_datetime-related tools with the registry.
func (h *InputDatetimeHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createInputDatetimeTool(), h.handleCreateInputDatetime)
	registry.RegisterTool(h.deleteInputDatetimeTool(), h.handleDeleteInputDatetime)
	registry.RegisterTool(h.setInputDatetimeTool(), h.handleSetInputDatetime)
}

func (h *InputDatetimeHandlers) createInputDatetimeTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_input_datetime",
		Description: "Create a new input_datetime helper in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input datetime configuration",
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
					Description: "Icon for the helper (e.g., mdi:calendar-clock)",
				},
				"has_date": {
					Type:        "boolean",
					Description: "Include date component (default: true)",
				},
				"has_time": {
					Type:        "boolean",
					Description: "Include time component (default: false)",
				},
				"initial": {
					Type:        "string",
					Description: "Initial value (format depends on has_date/has_time)",
				},
			},
			Required: []string{"id", "name"},
		},
	}
}

func (h *InputDatetimeHandlers) deleteInputDatetimeTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_input_datetime",
		Description: "Delete an input_datetime helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input datetime entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_datetime (e.g., input_datetime.my_datetime)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *InputDatetimeHandlers) setInputDatetimeTool() mcp.Tool {
	return mcp.Tool{
		Name:        "set_input_datetime",
		Description: "Set the value of an input_datetime helper",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Input datetime entity ID and new value",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the input_datetime (e.g., input_datetime.my_datetime)",
				},
				"datetime": {
					Type:        "string",
					Description: "Full datetime value (YYYY-MM-DD HH:MM:SS)",
				},
				"date": {
					Type:        "string",
					Description: "Date value (YYYY-MM-DD), use when has_date=true",
				},
				"time": {
					Type:        "string",
					Description: "Time value (HH:MM:SS), use when has_time=true",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *InputDatetimeHandlers) handleCreateInputDatetime(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	config := buildInputDatetimeHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: "input_datetime",
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating input_datetime: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("input_datetime.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input datetime '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *InputDatetimeHandlers) handleDeleteInputDatetime(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "input_datetime" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_datetime entity (e.g., input_datetime.my_datetime)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting input_datetime: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input datetime '%s' deleted successfully", entityID))},
	}, nil
}

func (h *InputDatetimeHandlers) handleSetInputDatetime(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Validate entity_id format
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "input_datetime" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be an input_datetime entity (e.g., input_datetime.my_datetime)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	// Check for datetime, date, or time values
	hasValue := false
	if datetime, ok := args["datetime"].(string); ok && datetime != "" {
		serviceData["datetime"] = datetime
		hasValue = true
	}
	if date, ok := args["date"].(string); ok && date != "" {
		serviceData["date"] = date
		hasValue = true
	}
	if time, ok := args["time"].(string); ok && time != "" {
		serviceData["time"] = time
		hasValue = true
	}

	if !hasValue {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("At least one of datetime, date, or time is required")},
			IsError: true,
		}, nil
	}

	if _, err := client.CallService(ctx, "input_datetime", "set_datetime", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error setting input_datetime: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Input datetime '%s' value set successfully", entityID))},
	}, nil
}

// buildInputDatetimeHelperConfig builds the configuration map for an input_datetime helper.
func buildInputDatetimeHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	if hasDate, ok := args["has_date"].(bool); ok {
		config["has_date"] = hasDate
	}

	if hasTime, ok := args["has_time"].(bool); ok {
		config["has_time"] = hasTime
	}

	if initial, ok := args["initial"].(string); ok && initial != "" {
		config["initial"] = initial
	}

	return config
}
