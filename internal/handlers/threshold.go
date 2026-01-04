// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

const platformThreshold = "threshold"

// ThresholdHandlers provides MCP tool handlers for threshold helper operations.
type ThresholdHandlers struct{}

// NewThresholdHandlers creates a new ThresholdHandlers instance.
func NewThresholdHandlers() *ThresholdHandlers {
	return &ThresholdHandlers{}
}

// RegisterTools registers all threshold-related tools with the registry.
func (h *ThresholdHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createThresholdTool(), h.handleCreateThreshold)
	registry.RegisterTool(h.deleteThresholdTool(), h.handleDeleteThreshold)
}

func (h *ThresholdHandlers) createThresholdTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_threshold",
		Description: "Create a new threshold helper in Home Assistant. A threshold binary sensor turns on when a source sensor is above the upper threshold or below the lower threshold.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Threshold configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the threshold sensor",
				},
				"entity_id": {
					Type:        "string",
					Description: "The source sensor entity ID to monitor (e.g., sensor.temperature)",
				},
				"lower": {
					Type:        "number",
					Description: "Lower threshold value. Binary sensor turns on when source is below this value.",
				},
				"upper": {
					Type:        "number",
					Description: "Upper threshold value. Binary sensor turns on when source is above this value.",
				},
				"hysteresis": {
					Type:        "number",
					Description: "Hysteresis value to prevent rapid on/off switching (default: 0.0)",
				},
				"device_class": {
					Type:        "string",
					Description: "Device class for the binary sensor (e.g., 'cold', 'heat', 'problem')",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the threshold sensor (e.g., mdi:thermometer-alert)",
				},
			},
			Required: []string{"id", "name", "entity_id"},
		},
	}
}

func (h *ThresholdHandlers) deleteThresholdTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_threshold",
		Description: "Delete a threshold helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Threshold entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the threshold (e.g., binary_sensor.my_threshold)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *ThresholdHandlers) handleCreateThreshold(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	sourceEntityID, _ := args["entity_id"].(string)
	if sourceEntityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id (source sensor) is required")},
			IsError: true,
		}, nil
	}

	// Check that at least lower or upper is provided
	_, hasLower := args["lower"]
	_, hasUpper := args["upper"]
	if !hasLower && !hasUpper {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("at least one of 'lower' or 'upper' threshold must be specified")},
			IsError: true,
		}, nil
	}

	config := buildThresholdConfig(name, sourceEntityID, args)

	helper := homeassistant.HelperConfig{
		Platform: platformThreshold,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating threshold: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("binary_sensor.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Threshold '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *ThresholdHandlers) handleDeleteThreshold(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Threshold creates binary_sensor entities
	platform, _ := ParseHelperEntityID(entityID)
	if platform != "binary_sensor" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a binary_sensor entity (e.g., binary_sensor.my_threshold)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting threshold: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Threshold '%s' deleted successfully", entityID))},
	}, nil
}

// buildThresholdConfig builds the configuration map for a threshold helper.
func buildThresholdConfig(name, sourceEntityID string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name
	config["entity_id"] = sourceEntityID

	if lower, ok := args["lower"].(float64); ok {
		config["lower"] = lower
	}

	if upper, ok := args["upper"].(float64); ok {
		config["upper"] = upper
	}

	if hysteresis, ok := args["hysteresis"].(float64); ok {
		config["hysteresis"] = hysteresis
	}

	if deviceClass, ok := args["device_class"].(string); ok && deviceClass != "" {
		config["device_class"] = deviceClass
	}

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	return config
}
