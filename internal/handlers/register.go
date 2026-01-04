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
func RegisterHelperTools(registry *mcp.Registry) {
	h := NewHelperHandlers()
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

// Note: RegisterScriptTools is defined in scripts.go
// Note: RegisterSceneTools is defined in scenes.go
// Note: RegisterTargetTools is defined in targets.go

// RegisterAllTools registers all available tool handlers with the registry.
// All handlers use the WebSocket API for communication with Home Assistant.
func RegisterAllTools(registry *mcp.Registry) {
	// Core entity and automation handlers
	RegisterEntityTools(registry)
	RegisterAutomationTools(registry)
	RegisterHelperTools(registry)
	RegisterScriptTools(registry)
	RegisterSceneTools(registry)

	// Registry, media, and advanced handlers
	RegisterRegistryTools(registry)
	RegisterMediaTools(registry)
	RegisterStatisticsTools(registry)
	RegisterLovelaceTools(registry)
	RegisterTargetTools(registry)
}
