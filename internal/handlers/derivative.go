// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const platformDerivative = "derivative"

// platformSensor is used for entity validation since derivative creates sensor entities.
const platformSensor = "sensor"

// DerivativeHandlers provides MCP tool handlers for derivative helper operations.
type DerivativeHandlers struct{}

// NewDerivativeHandlers creates a new DerivativeHandlers instance.
func NewDerivativeHandlers() *DerivativeHandlers {
	return &DerivativeHandlers{}
}

// RegisterTools registers all derivative-related tools with the registry.
func (h *DerivativeHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createDerivativeTool(), h.handleCreateDerivative)
	registry.RegisterTool(h.deleteDerivativeTool(), h.handleDeleteDerivative)
}

func (h *DerivativeHandlers) createDerivativeTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_derivative",
		Description: "Create a new derivative helper in Home Assistant. A derivative sensor tracks the rate of change of a source sensor over time.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Derivative sensor configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the derivative sensor",
				},
				"source": {
					Type:        "string",
					Description: "The source sensor entity ID to track (e.g., sensor.energy)",
				},
				"round": {
					Type:        "number",
					Description: "Number of decimal places to round the derivative value (default: 2)",
				},
				"time_window": {
					Type:        "string",
					Description: "Time window for calculating the derivative in HH:MM:SS format (e.g., '00:05:00' for 5 minutes)",
				},
				"unit_time": {
					Type:        "string",
					Description: "Time unit for the derivative: 's' (seconds), 'min' (minutes), 'h' (hours), 'd' (days)",
					Enum:        []string{"s", "min", "h", "d"},
				},
				"unit_prefix": {
					Type:        "string",
					Description: "Unit prefix for the result: 'k' (kilo), 'M' (mega), 'G' (giga), 'T' (tera), 'none'",
					Enum:        []string{"none", "k", "M", "G", "T"},
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the derivative sensor (e.g., mdi:chart-line)",
				},
			},
			Required: []string{"id", "name", "source"},
		},
	}
}

func (h *DerivativeHandlers) deleteDerivativeTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_derivative",
		Description: "Delete a derivative helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Derivative entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the derivative sensor (e.g., sensor.my_derivative)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *DerivativeHandlers) handleCreateDerivative(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	source, _ := args["source"].(string)
	if source == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("source (source sensor entity ID) is required")},
			IsError: true,
		}, nil
	}

	config := buildDerivativeConfig(name, source, args)

	helper := homeassistant.HelperConfig{
		Platform: platformDerivative,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating derivative: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("sensor.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Derivative sensor '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *DerivativeHandlers) handleDeleteDerivative(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Derivative creates sensor entities
	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformSensor {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a sensor entity (e.g., sensor.my_derivative)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting derivative: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Derivative sensor '%s' deleted successfully", entityID))},
	}, nil
}

// buildDerivativeConfig builds the configuration map for a derivative helper.
func buildDerivativeConfig(name, source string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name
	config["source"] = source

	if round, ok := args["round"].(float64); ok {
		config["round"] = int(round)
	}

	if timeWindow, ok := args["time_window"].(string); ok && timeWindow != "" {
		config["time_window"] = timeWindow
	}

	if unitTime, ok := args["unit_time"].(string); ok && unitTime != "" {
		config["unit_time"] = unitTime
	}

	if unitPrefix, ok := args["unit_prefix"].(string); ok && unitPrefix != "" && unitPrefix != "none" {
		config["unit_prefix"] = unitPrefix
	}

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	return config
}
