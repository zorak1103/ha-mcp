// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import "gitlab.com/zorak1103/ha-mcp/internal/mcp"

// RegisterEntityTools registers all entity-related tools with the registry.
func RegisterEntityTools(registry *mcp.Registry) {
	h := NewEntityHandlers()
	h.RegisterTools(registry)
}

// RegisterAutomationTools registers all automation-related tools with the registry.
func RegisterAutomationTools(registry *mcp.Registry) {
	h := NewAutomationHandlers()
	h.RegisterTools(registry)
}

// RegisterHelperTools registers all helper-related tools with the registry.
// This registers the generic list_helpers tool.
func RegisterHelperTools(registry *mcp.Registry) {
	h := NewHelperHandlers()
	h.RegisterTools(registry)
}

// RegisterInputBooleanTools registers all input_boolean helper tools with the registry.
func RegisterInputBooleanTools(registry *mcp.Registry) {
	h := NewInputBooleanHandlers()
	h.RegisterTools(registry)
}

// RegisterInputNumberTools registers all input_number helper tools with the registry.
func RegisterInputNumberTools(registry *mcp.Registry) {
	h := NewInputNumberHandlers()
	h.RegisterTools(registry)
}

// RegisterInputTextTools registers all input_text helper tools with the registry.
func RegisterInputTextTools(registry *mcp.Registry) {
	h := NewInputTextHandlers()
	h.RegisterTools(registry)
}

// RegisterInputSelectTools registers all input_select helper tools with the registry.
func RegisterInputSelectTools(registry *mcp.Registry) {
	h := NewInputSelectHandlers()
	h.RegisterTools(registry)
}

// RegisterInputDatetimeTools registers all input_datetime helper tools with the registry.
func RegisterInputDatetimeTools(registry *mcp.Registry) {
	h := NewInputDatetimeHandlers()
	h.RegisterTools(registry)
}

// RegisterInputButtonTools registers all input_button helper tools with the registry.
func RegisterInputButtonTools(registry *mcp.Registry) {
	h := NewInputButtonHandlers()
	h.RegisterTools(registry)
}

// RegisterCounterTools registers all counter helper tools with the registry.
func RegisterCounterTools(registry *mcp.Registry) {
	h := NewCounterHandlers()
	h.RegisterTools(registry)
}

// RegisterTimerTools registers all timer helper tools with the registry.
func RegisterTimerTools(registry *mcp.Registry) {
	h := NewTimerHandlers()
	h.RegisterTools(registry)
}

// RegisterScheduleTools registers all schedule helper tools with the registry.
func RegisterScheduleTools(registry *mcp.Registry) {
	h := NewScheduleHandlers()
	h.RegisterTools(registry)
}

// RegisterGroupTools registers all group helper tools with the registry.
func RegisterGroupTools(registry *mcp.Registry) {
	h := NewGroupHandlers()
	h.RegisterTools(registry)
}

// RegisterRegistryTools registers all registry-related tools (entity/device/area registries).
func RegisterRegistryTools(registry *mcp.Registry) {
	h := NewRegistryHandlers()
	h.RegisterTools(registry)
}

// RegisterMediaTools registers all media-related tools with the registry.
func RegisterMediaTools(registry *mcp.Registry) {
	h := NewMediaHandlers()
	h.RegisterTools(registry)
}

// RegisterStatisticsTools registers all statistics-related tools with the registry.
func RegisterStatisticsTools(registry *mcp.Registry) {
	h := NewStatisticsHandlers()
	h.RegisterTools(registry)
}

// RegisterLovelaceTools registers all Lovelace dashboard-related tools with the registry.
func RegisterLovelaceTools(registry *mcp.Registry) {
	h := NewLovelaceHandlers()
	h.RegisterTools(registry)
}

// RegisterAllTools registers all available tool handlers with the registry.
// All handlers use the WebSocket API for communication with Home Assistant.
func RegisterAllTools(registry *mcp.Registry) {
	// Core entity and automation handlers
	RegisterEntityTools(registry)
	RegisterAutomationTools(registry)
	RegisterScriptTools(registry)
	RegisterSceneTools(registry)

	// Helper tools (generic list_helpers)
	RegisterHelperTools(registry)

	// Input helper handlers (create, delete, actions)
	RegisterInputBooleanTools(registry)
	RegisterInputNumberTools(registry)
	RegisterInputTextTools(registry)
	RegisterInputSelectTools(registry)
	RegisterInputDatetimeTools(registry)

	// Tier 1 helper handlers (input_button, counter, timer)
	RegisterInputButtonTools(registry)
	RegisterCounterTools(registry)
	RegisterTimerTools(registry)

	// Tier 2 helper handlers (schedule, group)
	RegisterScheduleTools(registry)
	RegisterGroupTools(registry)

	// Registry, media, and advanced handlers
	RegisterRegistryTools(registry)
	RegisterMediaTools(registry)
	RegisterStatisticsTools(registry)
	RegisterLovelaceTools(registry)
	RegisterTargetTools(registry)
}
