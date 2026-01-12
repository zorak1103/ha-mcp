// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const platformSchedule = "schedule"

// ScheduleHandlers provides MCP tool handlers for schedule helper operations.
type ScheduleHandlers struct{}

// NewScheduleHandlers creates a new ScheduleHandlers instance.
func NewScheduleHandlers() *ScheduleHandlers {
	return &ScheduleHandlers{}
}

// RegisterTools registers all schedule-related tools with the registry.
func (h *ScheduleHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createScheduleTool(), h.handleCreateSchedule)
	registry.RegisterTool(h.deleteScheduleTool(), h.handleDeleteSchedule)
	registry.RegisterTool(h.reloadScheduleTool(), h.handleReloadSchedule)
}

func (h *ScheduleHandlers) createScheduleTool() mcp.Tool {
	timeRangeSchema := mcp.JSONSchema{
		Type:        "object",
		Description: "Time range with from and to times",
		Properties: map[string]mcp.JSONSchema{
			"from": {
				Type:        "string",
				Description: "Start time in HH:MM:SS format (e.g., 08:00:00)",
			},
			"to": {
				Type:        "string",
				Description: "End time in HH:MM:SS format (e.g., 17:00:00)",
			},
		},
		Required: []string{"from", "to"},
	}

	daySchema := mcp.JSONSchema{
		Type:        "array",
		Description: "Array of time ranges for this day",
		Items:       &timeRangeSchema,
	}

	return mcp.Tool{
		Name:        "create_schedule",
		Description: "Create a new schedule helper in Home Assistant. A schedule defines time blocks for each day of the week. Useful for automation conditions based on time schedules.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Schedule configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the schedule",
				},
				"monday":    daySchema,
				"tuesday":   daySchema,
				"wednesday": daySchema,
				"thursday":  daySchema,
				"friday":    daySchema,
				"saturday":  daySchema,
				"sunday":    daySchema,
				"icon": {
					Type:        "string",
					Description: "Icon for the helper (e.g., mdi:calendar-clock)",
				},
			},
			Required: []string{"id", "name"},
		},
	}
}

func (h *ScheduleHandlers) deleteScheduleTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_schedule",
		Description: "Delete a schedule helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Schedule entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the schedule (e.g., schedule.work_hours)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *ScheduleHandlers) reloadScheduleTool() mcp.Tool {
	return mcp.Tool{
		Name:        "reload_schedule",
		Description: "Reload all schedule helpers from configuration. Use this after manually editing schedule configuration files.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "No parameters required",
			Properties:  map[string]mcp.JSONSchema{},
		},
	}
}

func (h *ScheduleHandlers) handleCreateSchedule(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	config := buildScheduleHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: platformSchedule,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating schedule: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("%s.%s", platformSchedule, id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Schedule '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *ScheduleHandlers) handleDeleteSchedule(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformSchedule {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a schedule entity (e.g., schedule.work_hours)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting schedule: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Schedule '%s' deleted successfully", entityID))},
	}, nil
}

func (h *ScheduleHandlers) handleReloadSchedule(ctx context.Context, client homeassistant.Client, _ map[string]any) (*mcp.ToolsCallResult, error) {
	serviceData := map[string]any{}

	if _, err := client.CallService(ctx, platformSchedule, "reload", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error reloading schedules: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent("Schedules reloaded successfully")},
	}, nil
}

// buildScheduleHelperConfig builds the configuration map for a schedule helper.
func buildScheduleHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	// Process each day of the week
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for _, day := range days {
		if daySchedule, ok := args[day].([]any); ok && len(daySchedule) > 0 {
			config[day] = daySchedule
		}
	}

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	return config
}
