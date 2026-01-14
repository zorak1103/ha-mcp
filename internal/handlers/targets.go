// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TargetHandlers provides handlers for Home Assistant target operations.
// These operations help discover applicable triggers, conditions, and services
// for specified targets (entities, devices, areas, labels).
type TargetHandlers struct{}

// NewTargetHandlers creates a new TargetHandlers instance.
func NewTargetHandlers() *TargetHandlers {
	return &TargetHandlers{}
}

// RegisterTools registers all target-related tools with the registry.
func (h *TargetHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getTriggersForTargetTool(), h.handleGetTriggersForTarget)
	registry.RegisterTool(h.getConditionsForTargetTool(), h.handleGetConditionsForTarget)
	registry.RegisterTool(h.getServicesForTargetTool(), h.handleGetServicesForTarget)
	registry.RegisterTool(h.extractFromTargetTool(), h.handleExtractFromTarget)
}

// targetInputSchema returns the common input schema for target operations.
func (h *TargetHandlers) targetInputSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type: "object",
		Properties: map[string]mcp.JSONSchema{
			"entity_id": {
				Type:        "array",
				Description: "List of entity IDs (e.g., ['light.living_room', 'switch.kitchen'])",
				Items: &mcp.JSONSchema{
					Type: "string",
				},
			},
			"device_id": {
				Type:        "array",
				Description: "List of device IDs",
				Items: &mcp.JSONSchema{
					Type: "string",
				},
			},
			"area_id": {
				Type:        "array",
				Description: "List of area IDs",
				Items: &mcp.JSONSchema{
					Type: "string",
				},
			},
			"label_id": {
				Type:        "array",
				Description: "List of label IDs",
				Items: &mcp.JSONSchema{
					Type: "string",
				},
			},
			"expand_group": {
				Type:        "boolean",
				Description: "When true (default), group entities are expanded to their members",
			},
		},
		Description: "Target specification with at least one of: entity_id, device_id, area_id, or label_id",
	}
}

// extractStringArray extracts a string array from parameters by key.
// Returns nil if the key doesn't exist or is not a valid array of strings.
func (h *TargetHandlers) extractStringArray(params map[string]any, key string) []string {
	value, ok := params[key]
	if !ok {
		return nil
	}

	arr, ok := value.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}

	return result
}

// parseTargetParams extracts target and expand_group from parameters.
func (h *TargetHandlers) parseTargetParams(params map[string]any) (homeassistant.Target, *bool, error) {
	target := homeassistant.Target{
		EntityID: h.extractStringArray(params, "entity_id"),
		DeviceID: h.extractStringArray(params, "device_id"),
		AreaID:   h.extractStringArray(params, "area_id"),
		LabelID:  h.extractStringArray(params, "label_id"),
	}

	// Check if at least one target type is specified
	if len(target.EntityID) == 0 && len(target.DeviceID) == 0 &&
		len(target.AreaID) == 0 && len(target.LabelID) == 0 {
		return target, nil, fmt.Errorf("at least one of entity_id, device_id, area_id, or label_id is required")
	}

	var expandGroup *bool
	if eg, ok := params["expand_group"]; ok {
		if b, ok := eg.(bool); ok {
			expandGroup = &b
		}
	}

	return target, expandGroup, nil
}

// getTriggersForTargetTool returns the tool definition for getting triggers for a target.
func (h *TargetHandlers) getTriggersForTargetTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_triggers_for_target",
		Description: "Get all applicable automation triggers for the specified target. Returns trigger types that can be used in automations for the given entities, devices, areas, or labels.",
		InputSchema: h.targetInputSchema(),
	}
}

// handleGetTriggersForTarget handles requests to get triggers for a target.
func (h *TargetHandlers) handleGetTriggersForTarget(
	ctx context.Context,
	client homeassistant.Client,
	params map[string]any,
) (*mcp.ToolsCallResult, error) {
	target, expandGroup, err := h.parseTargetParams(params)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Invalid parameters: %v", err)),
			},
			IsError: true,
		}, nil
	}

	triggers, err := client.GetTriggersForTarget(ctx, target, expandGroup)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting triggers for target: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(triggers, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(string(output)),
		},
	}, nil
}

// getConditionsForTargetTool returns the tool definition for getting conditions for a target.
func (h *TargetHandlers) getConditionsForTargetTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_conditions_for_target",
		Description: "Get all applicable automation conditions for the specified target. Returns condition types that can be used in automations for the given entities, devices, areas, or labels.",
		InputSchema: h.targetInputSchema(),
	}
}

// handleGetConditionsForTarget handles requests to get conditions for a target.
func (h *TargetHandlers) handleGetConditionsForTarget(
	ctx context.Context,
	client homeassistant.Client,
	params map[string]any,
) (*mcp.ToolsCallResult, error) {
	target, expandGroup, err := h.parseTargetParams(params)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Invalid parameters: %v", err)),
			},
			IsError: true,
		}, nil
	}

	conditions, err := client.GetConditionsForTarget(ctx, target, expandGroup)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting conditions for target: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(conditions, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(string(output)),
		},
	}, nil
}

// getServicesForTargetTool returns the tool definition for getting services for a target.
func (h *TargetHandlers) getServicesForTargetTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_services_for_target",
		Description: "Get all applicable services for the specified target. Returns services that can be called for the given entities, devices, areas, or labels.",
		InputSchema: h.targetInputSchema(),
	}
}

// handleGetServicesForTarget handles requests to get services for a target.
func (h *TargetHandlers) handleGetServicesForTarget(
	ctx context.Context,
	client homeassistant.Client,
	params map[string]any,
) (*mcp.ToolsCallResult, error) {
	target, expandGroup, err := h.parseTargetParams(params)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Invalid parameters: %v", err)),
			},
			IsError: true,
		}, nil
	}

	services, err := client.GetServicesForTarget(ctx, target, expandGroup)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting services for target: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(string(output)),
		},
	}, nil
}

// extractFromTargetTool returns the tool definition for extracting entities from a target.
func (h *TargetHandlers) extractFromTargetTool() mcp.Tool {
	return mcp.Tool{
		Name:        "extract_from_target",
		Description: "Extract entities, devices, and areas from the specified target. Resolves all referenced entities, devices, and areas while also reporting any missing devices, areas, floors, or labels.",
		InputSchema: h.targetInputSchema(),
	}
}

// handleExtractFromTarget handles requests to extract entities from a target.
func (h *TargetHandlers) handleExtractFromTarget(
	ctx context.Context,
	client homeassistant.Client,
	params map[string]any,
) (*mcp.ToolsCallResult, error) {
	target, expandGroup, err := h.parseTargetParams(params)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Invalid parameters: %v", err)),
			},
			IsError: true,
		}, nil
	}

	result, err := client.ExtractFromTarget(ctx, target, expandGroup)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error extracting from target: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(string(output)),
		},
	}, nil
}

// RegisterTargetTools registers all target-related tools with the registry.
func RegisterTargetTools(registry *mcp.Registry) {
	h := NewTargetHandlers()
	h.RegisterTools(registry)
}
