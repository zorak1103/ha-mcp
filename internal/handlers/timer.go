// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const platformTimer = "timer"

// TimerHandlers provides MCP tool handlers for timer helper operations.
type TimerHandlers struct{}

// NewTimerHandlers creates a new TimerHandlers instance.
func NewTimerHandlers() *TimerHandlers {
	return &TimerHandlers{}
}

// RegisterTools registers all timer-related tools with the registry.
func (h *TimerHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createTimerTool(), h.handleCreateTimer)
	registry.RegisterTool(h.deleteTimerTool(), h.handleDeleteTimer)
	registry.RegisterTool(h.startTimerTool(), h.handleStartTimer)
	registry.RegisterTool(h.pauseTimerTool(), h.handlePauseTimer)
	registry.RegisterTool(h.cancelTimerTool(), h.handleCancelTimer)
	registry.RegisterTool(h.finishTimerTool(), h.handleFinishTimer)
	registry.RegisterTool(h.changeTimerTool(), h.handleChangeTimer)
}

func (h *TimerHandlers) createTimerTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_timer",
		Description: "Create a new timer helper in Home Assistant. A timer counts down from a duration and can trigger automations when finished.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Timer configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the helper",
				},
				"duration": {
					Type:        "string",
					Description: "Default duration in HH:MM:SS format (e.g., '00:05:00' for 5 minutes)",
				},
				"restore": {
					Type:        "boolean",
					Description: "Whether to restore the timer state after restart (default: false)",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the helper (e.g., mdi:timer)",
				},
			},
			Required: []string{"id", "name"},
		},
	}
}

func (h *TimerHandlers) deleteTimerTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_timer",
		Description: "Delete a timer helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Timer entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the timer (e.g., timer.my_timer)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *TimerHandlers) startTimerTool() mcp.Tool {
	return mcp.Tool{
		Name:        "start_timer",
		Description: "Start a timer. If the timer is paused, it will resume. Optionally specify a duration to override the default.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Timer entity ID and optional duration",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the timer (e.g., timer.my_timer)",
				},
				"duration": {
					Type:        "string",
					Description: "Optional duration in HH:MM:SS format to override the default",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *TimerHandlers) pauseTimerTool() mcp.Tool {
	return mcp.Tool{
		Name:        "pause_timer",
		Description: "Pause a running timer. The remaining time is preserved and can be resumed with start.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Timer entity ID to pause",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the timer (e.g., timer.my_timer)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *TimerHandlers) cancelTimerTool() mcp.Tool {
	return mcp.Tool{
		Name:        "cancel_timer",
		Description: "Cancel a running or paused timer. The timer is reset and no finish event is triggered.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Timer entity ID to cancel",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the timer (e.g., timer.my_timer)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *TimerHandlers) finishTimerTool() mcp.Tool {
	return mcp.Tool{
		Name:        "finish_timer",
		Description: "Finish a running timer immediately. This triggers the timer.finished event as if the timer had naturally expired.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Timer entity ID to finish",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the timer (e.g., timer.my_timer)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *TimerHandlers) changeTimerTool() mcp.Tool {
	return mcp.Tool{
		Name:        "change_timer",
		Description: "Change the duration of a running timer by adding or subtracting time.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Timer entity ID and duration change",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the timer (e.g., timer.my_timer)",
				},
				"duration": {
					Type:        "string",
					Description: "Duration to add (positive) or subtract (negative) in HH:MM:SS format (e.g., '00:01:00' to add 1 minute, '-00:00:30' to subtract 30 seconds)",
				},
			},
			Required: []string{"entity_id", "duration"},
		},
	}
}

func (h *TimerHandlers) handleCreateTimer(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
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

	config := buildTimerHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: platformTimer,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating timer: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("%s.%s", platformTimer, id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Timer '%s' created successfully as %s", name, entityID))},
	}, nil
}

func (h *TimerHandlers) handleDeleteTimer(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformTimer {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a timer entity (e.g., timer.my_timer)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting timer: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Timer '%s' deleted successfully", entityID))},
	}, nil
}

func (h *TimerHandlers) handleStartTimer(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformTimer {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a timer entity (e.g., timer.my_timer)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	if duration, ok := args["duration"].(string); ok && duration != "" {
		serviceData["duration"] = duration
	}

	if _, err := client.CallService(ctx, platformTimer, "start", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error starting timer: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Timer '%s' started successfully", entityID))},
	}, nil
}

func (h *TimerHandlers) handlePauseTimer(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformTimer {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a timer entity (e.g., timer.my_timer)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	if _, err := client.CallService(ctx, platformTimer, "pause", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error pausing timer: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Timer '%s' paused successfully", entityID))},
	}, nil
}

func (h *TimerHandlers) handleCancelTimer(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformTimer {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a timer entity (e.g., timer.my_timer)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	if _, err := client.CallService(ctx, platformTimer, "cancel", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error canceling timer: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Timer '%s' canceled successfully", entityID))},
	}, nil
}

func (h *TimerHandlers) handleFinishTimer(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformTimer {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a timer entity (e.g., timer.my_timer)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	if _, err := client.CallService(ctx, platformTimer, "finish", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error finishing timer: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Timer '%s' finished successfully", entityID))},
	}, nil
}

func (h *TimerHandlers) handleChangeTimer(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformTimer {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a timer entity (e.g., timer.my_timer)")},
			IsError: true,
		}, nil
	}

	duration, ok := args["duration"].(string)
	if !ok || duration == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("duration is required")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
		"duration":  duration,
	}

	if _, err := client.CallService(ctx, platformTimer, "change", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error changing timer: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Timer '%s' duration changed by %s successfully", entityID, duration))},
	}, nil
}

// buildTimerHelperConfig builds the configuration map for a timer helper.
func buildTimerHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if duration, ok := args["duration"].(string); ok && duration != "" {
		config["duration"] = duration
	}

	if restore, ok := args["restore"].(bool); ok {
		config["restore"] = restore
	}

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	return config
}
