// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// platformIntegration is the Home Assistant platform name (HA calls it "integration", not "integral").
const platformIntegration = "integration"

// IntegralHandlers provides MCP tool handlers for integral (integration) helper operations.
type IntegralHandlers struct{}

// NewIntegralHandlers creates a new IntegralHandlers instance.
func NewIntegralHandlers() *IntegralHandlers {
	return &IntegralHandlers{}
}

// RegisterTools registers all integral-related tools with the registry.
func (h *IntegralHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createIntegralTool(), h.handleCreateIntegral)
	registry.RegisterTool(h.deleteIntegralTool(), h.handleDeleteIntegral)
	registry.RegisterTool(h.resetIntegralTool(), h.handleResetIntegral)
}

func (h *IntegralHandlers) createIntegralTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_integral",
		Description: "Create a new integral (integration) helper in Home Assistant. An integral sensor calculates the integral (sum over time) of a source sensor's values.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Integral sensor configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the integral sensor",
				},
				"source": {
					Type:        "string",
					Description: "The source sensor entity ID to integrate (e.g., sensor.power)",
				},
				"method": {
					Type:        "string",
					Description: "Integration method: 'trapezoidal' (default), 'left', or 'right'",
					Enum:        []string{"trapezoidal", "left", "right"},
				},
				"round": {
					Type:        "number",
					Description: "Number of decimal places to round the integral value (default: 2)",
				},
				"unit_time": {
					Type:        "string",
					Description: "Time unit for integration: 's' (seconds), 'min' (minutes), 'h' (hours), 'd' (days)",
					Enum:        []string{"s", "min", "h", "d"},
				},
				"unit_prefix": {
					Type:        "string",
					Description: "Unit prefix for the result: 'k' (kilo), 'M' (mega), 'G' (giga), 'T' (tera), 'none'",
					Enum:        []string{"none", "k", "M", "G", "T"},
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the integral sensor (e.g., mdi:sigma)",
				},
			},
			Required: []string{"id", "name", "source"},
		},
	}
}

func (h *IntegralHandlers) deleteIntegralTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_integral",
		Description: "Delete an integral (integration) helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Integral entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the integral sensor (e.g., sensor.my_integral)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *IntegralHandlers) resetIntegralTool() mcp.Tool {
	return mcp.Tool{
		Name:        "reset_integral",
		Description: "Reset an integral (integration) sensor to zero",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Integral entity ID to reset",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the integral sensor (e.g., sensor.my_integral)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *IntegralHandlers) handleCreateIntegral(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	config := buildIntegralConfig(name, source, args)

	helper := homeassistant.HelperConfig{
		Platform: platformIntegration,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating integral: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("sensor.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Integral sensor '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *IntegralHandlers) handleDeleteIntegral(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Integral creates sensor entities
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "sensor" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a sensor entity (e.g., sensor.my_integral)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting integral: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Integral sensor '%s' deleted successfully", entityID))},
	}, nil
}

func (h *IntegralHandlers) handleResetIntegral(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	// Home Assistant uses "integration" as the service domain
	_, err := client.CallService(ctx, platformIntegration, "reset", serviceData)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error resetting integral: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Integral sensor '%s' reset to zero successfully", entityID))},
	}, nil
}

// buildIntegralConfig builds the configuration map for an integral helper.
func buildIntegralConfig(name, source string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name
	config["source"] = source

	if method, ok := args["method"].(string); ok && method != "" {
		config["method"] = method
	}

	if round, ok := args["round"].(float64); ok {
		config["round"] = int(round)
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
