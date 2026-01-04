// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

const platformTemplate = "template"

// platformSensorEntity is used for entity validation.
const platformSensorEntity = "sensor"

// platformBinarySensorEntity is used for entity validation.
const platformBinarySensorEntity = "binary_sensor"

// TemplateHelperHandlers provides MCP tool handlers for template helper operations.
// Named TemplateHelperHandlers to avoid conflicts with potential template engine handlers.
type TemplateHelperHandlers struct{}

// NewTemplateHelperHandlers creates a new TemplateHelperHandlers instance.
func NewTemplateHelperHandlers() *TemplateHelperHandlers {
	return &TemplateHelperHandlers{}
}

// RegisterTools registers all template helper-related tools with the registry.
func (h *TemplateHelperHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createTemplateSensorTool(), h.handleCreateTemplateSensor)
	registry.RegisterTool(h.createTemplateBinarySensorTool(), h.handleCreateTemplateBinarySensor)
	registry.RegisterTool(h.deleteTemplateHelperTool(), h.handleDeleteTemplateHelper)
}

func (h *TemplateHelperHandlers) createTemplateSensorTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_template_sensor",
		Description: "Create a new template sensor helper in Home Assistant. A template sensor calculates its state from a Jinja2 template.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Template sensor configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the sensor",
				},
				"state": {
					Type:        "string",
					Description: "Jinja2 template that defines the sensor state (e.g., '{{ states(\"sensor.temperature\") | float + 5 }}')",
				},
				"unit_of_measurement": {
					Type:        "string",
					Description: "Unit of measurement for the sensor (e.g., '°C', '%', 'kWh')",
				},
				"device_class": {
					Type:        "string",
					Description: "Device class for the sensor (e.g., 'temperature', 'humidity', 'power', 'energy')",
				},
				"state_class": {
					Type:        "string",
					Description: "State class for statistics (measurement, total, total_increasing)",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the sensor (e.g., mdi:thermometer)",
				},
			},
			Required: []string{"id", "name", "state"},
		},
	}
}

func (h *TemplateHelperHandlers) createTemplateBinarySensorTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_template_binary_sensor",
		Description: "Create a new template binary sensor helper in Home Assistant. A template binary sensor determines its on/off state from a Jinja2 template.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Template binary sensor configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the binary sensor",
				},
				"state": {
					Type:        "string",
					Description: "Jinja2 template that determines the on/off state (must evaluate to true/false or on/off)",
				},
				"device_class": {
					Type:        "string",
					Description: "Device class for the binary sensor (e.g., 'motion', 'door', 'window', 'presence', 'problem')",
				},
				"delay_on": {
					Type:        "string",
					Description: "Duration to wait before turning on (e.g., '00:00:05' for 5 seconds)",
				},
				"delay_off": {
					Type:        "string",
					Description: "Duration to wait before turning off (e.g., '00:00:05' for 5 seconds)",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the binary sensor (e.g., mdi:motion-sensor)",
				},
			},
			Required: []string{"id", "name", "state"},
		},
	}
}

func (h *TemplateHelperHandlers) deleteTemplateHelperTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_template_helper",
		Description: "Delete a template helper (sensor or binary_sensor) from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Template entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the template helper (e.g., sensor.my_template or binary_sensor.my_template)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *TemplateHelperHandlers) handleCreateTemplateSensor(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	state, _ := args["state"].(string)
	if state == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("state (Jinja2 template) is required")},
			IsError: true,
		}, nil
	}

	config := buildTemplateSensorConfig(name, state, args)

	helper := homeassistant.HelperConfig{
		Platform: platformTemplate,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating template sensor: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("sensor.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Template sensor '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *TemplateHelperHandlers) handleCreateTemplateBinarySensor(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	state, _ := args["state"].(string)
	if state == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("state (Jinja2 template) is required")},
			IsError: true,
		}, nil
	}

	config := buildTemplateBinarySensorConfig(name, state, args)

	helper := homeassistant.HelperConfig{
		Platform: platformTemplate,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating template binary sensor: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("binary_sensor.%s", id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Template binary sensor '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *TemplateHelperHandlers) handleDeleteTemplateHelper(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	// Template helpers can be sensor.* or binary_sensor.*
	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformSensorEntity && platform != platformBinarySensorEntity {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a template sensor or binary_sensor (e.g., sensor.my_template or binary_sensor.my_template)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting template helper: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Template helper '%s' deleted successfully", entityID))},
	}, nil
}

// buildTemplateSensorConfig builds the configuration map for a template sensor helper.
func buildTemplateSensorConfig(name, state string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name
	config["state"] = state
	config["template_type"] = "sensor"

	if unitOfMeasurement, ok := args["unit_of_measurement"].(string); ok && unitOfMeasurement != "" {
		config["unit_of_measurement"] = unitOfMeasurement
	}

	if deviceClass, ok := args["device_class"].(string); ok && deviceClass != "" {
		config["device_class"] = deviceClass
	}

	if stateClass, ok := args["state_class"].(string); ok && stateClass != "" {
		config["state_class"] = stateClass
	}

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	return config
}

// buildTemplateBinarySensorConfig builds the configuration map for a template binary sensor helper.
func buildTemplateBinarySensorConfig(name, state string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name
	config["state"] = state
	config["template_type"] = "binary_sensor"

	if deviceClass, ok := args["device_class"].(string); ok && deviceClass != "" {
		config["device_class"] = deviceClass
	}

	if delayOn, ok := args["delay_on"].(string); ok && delayOn != "" {
		config["delay_on"] = delayOn
	}

	if delayOff, ok := args["delay_off"].(string); ok && delayOff != "" {
		config["delay_off"] = delayOff
	}

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	return config
}
