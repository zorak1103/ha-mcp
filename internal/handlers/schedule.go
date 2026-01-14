// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const platformSchedule = "schedule"

// TimeBlock represents a time range with from and to times.
type TimeBlock struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ScheduleDetails contains detailed information about a schedule helper.
type ScheduleDetails struct {
	EntityID     string                 `json:"entity_id"`
	State        string                 `json:"state"`
	FriendlyName string                 `json:"friendly_name,omitempty"`
	Icon         string                 `json:"icon,omitempty"`
	NextEvent    string                 `json:"next_event,omitempty"`
	Days         map[string][]TimeBlock `json:"days,omitempty"`
}

// ScheduleHandlers provides MCP tool handlers for schedule helper operations.
type ScheduleHandlers struct{}

// NewScheduleHandlers creates a new ScheduleHandlers instance.
func NewScheduleHandlers() *ScheduleHandlers {
	return &ScheduleHandlers{}
}

// RegisterTools registers all schedule-related tools with the registry.
func (h *ScheduleHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getScheduleDetailsTool(), h.handleGetScheduleDetails)
	registry.RegisterTool(h.createScheduleTool(), h.handleCreateSchedule)
	registry.RegisterTool(h.deleteScheduleTool(), h.handleDeleteSchedule)
	registry.RegisterTool(h.reloadScheduleTool(), h.handleReloadSchedule)
}

func (h *ScheduleHandlers) getScheduleDetailsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_schedule_details",
		Description: "Get detailed information about a schedule helper including all time blocks for each day of the week. Shows when the schedule is active.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Schedule entity ID to get details for",
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

// weekdays contains all days of the week for schedule processing.
var weekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

// validateScheduleEntityID validates that the entity ID is a schedule entity.
func validateScheduleEntityID(entityID string) error {
	if entityID == "" {
		return fmt.Errorf("entity_id is required")
	}
	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformSchedule {
		return fmt.Errorf("entity_id must be a schedule entity (e.g., schedule.work_hours)")
	}
	return nil
}

// extractScheduleAttributes extracts friendly_name, icon, and next_event from state attributes.
func extractScheduleAttributes(attributes map[string]any) (friendlyName, icon, nextEvent string) {
	if name, ok := attributes["friendly_name"].(string); ok {
		friendlyName = name
	}
	if ic, ok := attributes["icon"].(string); ok {
		icon = ic
	}
	if ne, ok := attributes["next_event"].(string); ok {
		nextEvent = ne
	}
	return friendlyName, icon, nextEvent
}

// parseTimeBlocks extracts time blocks for all weekdays from config.
func parseTimeBlocks(config map[string]any) map[string][]TimeBlock {
	result := make(map[string][]TimeBlock)
	for _, day := range weekdays {
		blocks, ok := config[day].([]any)
		if !ok {
			continue
		}
		dayBlocks := parseBlocksForDay(blocks)
		if len(dayBlocks) > 0 {
			result[day] = dayBlocks
		}
	}
	return result
}

// parseBlocksForDay parses a single day's time blocks from raw config data.
func parseBlocksForDay(blocks []any) []TimeBlock {
	var dayBlocks []TimeBlock
	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}
		tb := TimeBlock{}
		if from, ok := blockMap["from"].(string); ok {
			tb.From = from
		}
		if to, ok := blockMap["to"].(string); ok {
			tb.To = to
		}
		dayBlocks = append(dayBlocks, tb)
	}
	return dayBlocks
}

// buildScheduleDetails constructs a ScheduleDetails from state and time blocks.
func buildScheduleDetails(state *homeassistant.Entity, timeBlocks map[string][]TimeBlock) ScheduleDetails {
	friendlyName, icon, nextEvent := extractScheduleAttributes(state.Attributes)
	return ScheduleDetails{
		EntityID:     state.EntityID,
		State:        state.State,
		FriendlyName: friendlyName,
		Icon:         icon,
		NextEvent:    nextEvent,
		Days:         timeBlocks,
	}
}

func (h *ScheduleHandlers) handleGetScheduleDetails(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	if err := validateScheduleEntityID(entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(err.Error())},
			IsError: true,
		}, nil
	}

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting schedule state: %v", err))},
			IsError: true,
		}, nil
	}

	config, configErr := client.GetScheduleConfig(ctx, entityID)
	if configErr != nil {
		config = make(map[string]any)
	}

	timeBlocks := parseTimeBlocks(config)
	details := buildScheduleDetails(state, timeBlocks)

	output, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting schedule: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
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
