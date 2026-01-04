// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// HelperHandlers provides MCP tool handlers for input helper operations.
type HelperHandlers struct{}

// NewHelperHandlers creates a new HelperHandlers instance.
func NewHelperHandlers() *HelperHandlers {
	return &HelperHandlers{}
}

// RegisterTools registers all helper-related tools with the registry.
func (h *HelperHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.listHelpersTool(), h.handleListHelpers)
	registry.RegisterTool(h.createHelperTool(), h.handleCreateHelper)
	registry.RegisterTool(h.updateHelperTool(), h.handleUpdateHelper)
	registry.RegisterTool(h.deleteHelperTool(), h.handleDeleteHelper)
	registry.RegisterTool(h.setHelperValueTool(), h.handleSetHelperValue)
}

func (h *HelperHandlers) listHelpersTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_helpers",
		Description: "List all input helpers in Home Assistant (input_boolean, input_number, input_text, input_select, input_datetime)",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "No parameters required",
		},
	}
}

func (h *HelperHandlers) createHelperTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_helper",
		Description: "Create a new input helper in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Helper configuration",
			Properties: map[string]mcp.JSONSchema{
				"platform": {
					Type:        "string",
					Description: "Helper type: input_boolean, input_number, input_text, input_select, input_datetime",
				},
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
				"min": {
					Type:        "number",
					Description: "Minimum value (for input_number) or min length (for input_text)",
				},
				"max": {
					Type:        "number",
					Description: "Maximum value (for input_number) or max length (for input_text)",
				},
				"step": {
					Type:        "number",
					Description: "Step value (for input_number)",
				},
				"initial": {
					Type:        "string",
					Description: "Initial value",
				},
				"options": {
					Type:        "array",
					Description: "List of options (for input_select)",
				},
				"has_date": {
					Type:        "boolean",
					Description: "Include date component (for input_datetime)",
				},
				"has_time": {
					Type:        "boolean",
					Description: "Include time component (for input_datetime)",
				},
				"mode": {
					Type:        "string",
					Description: "Display mode: 'box' or 'slider' (for input_number), 'text' or 'password' (for input_text)",
				},
				"unit_of_measurement": {
					Type:        "string",
					Description: "Unit of measurement (for input_number)",
				},
			},
			Required: []string{"platform", "id", "name"},
		},
	}
}

func (h *HelperHandlers) updateHelperTool() mcp.Tool {
	return mcp.Tool{
		Name:        "update_helper",
		Description: "Update an existing input helper",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Helper ID and updated configuration",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the helper (e.g., input_boolean.my_switch)",
				},
				"name": {
					Type:        "string",
					Description: "Updated name for the helper",
				},
				"icon": {
					Type:        "string",
					Description: "Updated icon for the helper",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *HelperHandlers) deleteHelperTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_helper",
		Description: "Delete an input helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Helper entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the helper (e.g., input_boolean.my_switch)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *HelperHandlers) setHelperValueTool() mcp.Tool {
	return mcp.Tool{
		Name:        "set_helper_value",
		Description: "Set the value of an input helper",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Helper entity ID and new value",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the helper",
				},
				"value": {
					Type:        "string",
					Description: "New value for the helper (boolean for input_boolean, number for input_number, etc.)",
				},
			},
			Required: []string{"entity_id", "value"},
		},
	}
}

func (h *HelperHandlers) handleListHelpers(ctx context.Context, client homeassistant.Client, _ map[string]any) (*mcp.ToolsCallResult, error) {
	helpers, err := client.ListHelpers(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error listing helpers: %v", err))},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(helpers, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting helpers: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}

func (h *HelperHandlers) handleCreateHelper(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	platform, _ := args["platform"].(string)
	if platform == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("platform is required")},
			IsError: true,
		}, nil
	}

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

	// Build config based on platform
	config := buildHelperConfig(platform, name, args)
	if config == nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Unsupported platform: %s", platform))},
			IsError: true,
		}, nil
	}

	helper := homeassistant.HelperConfig{
		Platform: platform,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating helper: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("%s.%s", platform, id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Helper '%s' created successfully as %s", name, entityID))},
	}, nil
}

// buildHelperConfig builds the configuration map for a helper based on platform type.
// Returns nil if the platform is not supported.
func buildHelperConfig(platform, name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	switch platform {
	case "input_boolean":
		buildInputBooleanConfig(config, args)
	case "input_number":
		buildInputNumberConfig(config, args)
	case "input_text":
		buildInputTextConfig(config, args)
	case "input_select":
		buildInputSelectConfig(config, args)
	case "input_datetime":
		buildInputDatetimeConfig(config, args)
	default:
		return nil
	}

	return config
}

// buildInputBooleanConfig adds input_boolean specific fields to config.
func buildInputBooleanConfig(config map[string]any, args map[string]any) {
	if initial, ok := args["initial"].(bool); ok {
		config["initial"] = initial
	}
}

// buildInputNumberConfig adds input_number specific fields to config.
func buildInputNumberConfig(config map[string]any, args map[string]any) {
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
}

// buildInputTextConfig adds input_text specific fields to config.
func buildInputTextConfig(config map[string]any, args map[string]any) {
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
}

// buildInputSelectConfig adds input_select specific fields to config.
func buildInputSelectConfig(config map[string]any, args map[string]any) {
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
}

// buildInputDatetimeConfig adds input_datetime specific fields to config.
func buildInputDatetimeConfig(config map[string]any, args map[string]any) {
	if hasDate, ok := args["has_date"].(bool); ok {
		config["has_date"] = hasDate
	}
	if hasTime, ok := args["has_time"].(bool); ok {
		config["has_time"] = hasTime
	}
	if initial, ok := args["initial"].(string); ok && initial != "" {
		config["initial"] = initial
	}
}

func (h *HelperHandlers) handleUpdateHelper(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Extract platform and ID from entity_id
	platform, id := parseEntityID(entityID)
	if platform == "" || id == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("Invalid entity_id format")},
			IsError: true,
		}, nil
	}

	config := make(map[string]any)
	if name, ok := args["name"].(string); ok && name != "" {
		config["name"] = name
	}
	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	helper := homeassistant.HelperConfig{
		Platform: platform,
		ID:       id,
		Config:   config,
	}

	if err := client.UpdateHelper(ctx, id, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error updating helper: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Helper '%s' updated successfully", entityID))},
	}, nil
}

func (h *HelperHandlers) handleDeleteHelper(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting helper: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Helper '%s' deleted successfully", entityID))},
	}, nil
}

func (h *HelperHandlers) handleSetHelperValue(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	value, ok := args["value"]
	if !ok {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("value is required")},
			IsError: true,
		}, nil
	}

	if err := client.SetHelperValue(ctx, entityID, value); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error setting helper value: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Helper '%s' value set successfully", entityID))},
	}, nil
}

// parseEntityID extracts platform and ID from an entity_id like "input_boolean.my_switch".
// It iterates through known helper platforms to find a matching prefix.
// Returns empty strings if the entity_id doesn't match any known helper platform.
func parseEntityID(entityID string) (platform, id string) {
	platforms := []string{"input_boolean", "input_number", "input_text", "input_select", "input_datetime"}
	for _, p := range platforms {
		prefix := p + "."
		if len(entityID) > len(prefix) && entityID[:len(prefix)] == prefix {
			return p, entityID[len(prefix):]
		}
	}
	return "", ""
}
