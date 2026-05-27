// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import "github.com/zorak1103/ha-mcp/internal/mcp"

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

// RegisterConsolidatedHelperTools registers the consolidated manage_helper and helper_action tools.
// This replaces individual create/delete/action tools for all 14 helper types.
func RegisterConsolidatedHelperTools(registry *mcp.Registry) {
	h := NewConsolidatedHelperHandlers()
	h.RegisterTools(registry)
}

// RegisterMediaTools registers all media-related tools with the registry.
func RegisterMediaTools(registry *mcp.Registry) {
	h := NewMediaHandlers()
	h.RegisterTools(registry)
}

// RegisterLovelaceTools registers all Lovelace dashboard-related tools with the registry.
func RegisterLovelaceTools(registry *mcp.Registry) {
	h := NewDashboardHandlers()
	h.RegisterTools(registry)
}

// RegisterAnalysisTools is defined in analysis.go

// RegisterServiceTools registers all service discovery tools with the registry.
func RegisterServiceTools(registry *mcp.Registry) {
	h := NewServiceHandlers()
	h.RegisterTools(registry)
}

// RegisterSystemTools registers all system information tools with the registry.
func RegisterSystemTools(registry *mcp.Registry) {
	h := NewSystemHandlers()
	h.RegisterTools(registry)
}

// RegisterDatetimeTools registers all date/time tools with the registry.
func RegisterDatetimeTools(registry *mcp.Registry) {
	h := NewDatetimeHandlers()
	h.RegisterTools(registry)
}

// RegisterTemplateTools registers all template rendering tools with the registry.
func RegisterTemplateTools(registry *mcp.Registry) {
	h := NewTemplateHandlers()
	h.RegisterTools(registry)
}

// RegisterLogbookTools registers all logbook tools with the registry.
func RegisterLogbookTools(registry *mcp.Registry) {
	h := NewLogbookHandlers()
	h.RegisterTools(registry)
}

// RegisterSystemLogTools registers the system log management tool with the registry.
func RegisterSystemLogTools(registry *mcp.Registry) {
	h := NewSystemLogHandlers()
	h.RegisterTools(registry)
}

// RegisterConfigTools registers all configuration validation tools with the registry.
func RegisterConfigTools(registry *mcp.Registry) {
	h := NewConfigHandlers()
	h.RegisterTools(registry)
}

// RegisterConfigEntryTools registers all config entry tools with the registry.
func RegisterConfigEntryTools(registry *mcp.Registry) {
	h := NewConfigEntryHandlers()
	h.RegisterTools(registry)
}

// RegisterHACSTools registers all HACS (Home Assistant Community Store) tools with the registry.
func RegisterHACSTools(registry *mcp.Registry) {
	h := NewHACSHandlers()
	h.RegisterTools(registry)
}

// RegisterAreaTools registers all area management tools with the registry.
func RegisterAreaTools(registry *mcp.Registry) {
	h := NewAreaHandlers()
	h.RegisterTools(registry)
}

// RegisterLabelTools registers all label management tools with the registry.
func RegisterLabelTools(registry *mcp.Registry) {
	h := NewLabelHandlers()
	h.RegisterTools(registry)
}

// RegisterFloorTools registers all floor management tools with the registry.
func RegisterFloorTools(registry *mcp.Registry) {
	h := NewFloorHandlers()
	h.RegisterTools(registry)
}

// RegisterZoneTools registers all zone management tools with the registry.
func RegisterZoneTools(registry *mcp.Registry) {
	h := NewZoneHandlers()
	h.RegisterTools(registry)
}

// RegisterPersonTools registers all person management tools with the registry.
func RegisterPersonTools(registry *mcp.Registry) {
	h := NewPersonHandlers()
	h.RegisterTools(registry)
}

// RegisterTagTools registers all tag management tools with the registry.
func RegisterTagTools(registry *mcp.Registry) {
	h := NewTagHandlers()
	h.RegisterTools(registry)
}

// RegisterDeviceQueryTools registers all device query tools with the registry.
func RegisterDeviceQueryTools(registry *mcp.Registry) {
	h := NewDeviceQueryHandlers()
	h.RegisterTools(registry)
}

// RegisterEntityManageTools registers entity registry management tools with the registry.
func RegisterEntityManageTools(registry *mcp.Registry) {
	h := NewEntityManageHandlers()
	h.RegisterTools(registry)
}

// RegisterDeviceManageTools registers device registry management tools with the registry.
func RegisterDeviceManageTools(registry *mcp.Registry) {
	h := NewDeviceManageHandlers()
	h.RegisterTools(registry)
}

// RegisterAllTools registers all available tool handlers with the registry.
// All handlers use the WebSocket API for communication with Home Assistant.
func RegisterAllTools(registry *mcp.Registry) {
	// Core entity and automation handlers
	RegisterEntityTools(registry)
	RegisterAutomationTools(registry)
	NewScriptHandlers().RegisterTools(registry)
	NewSceneHandlers().RegisterTools(registry)

	// Consolidated helper tools (manage_helper, helper_action)
	// Replaces individual tools: create/delete/action for all 14 helper types
	RegisterConsolidatedHelperTools(registry)

	// Registry tools (consolidated: get_registry replaces list_entity/device/area_registry)
	RegisterConsolidatedRegistryTools(registry)

	// Area management tools (consolidated: manage_area for full CRUD)
	RegisterAreaTools(registry)

	// Label management tools (consolidated: manage_label for full CRUD)
	RegisterLabelTools(registry)

	// Floor management tools (consolidated: manage_floor for full CRUD)
	RegisterFloorTools(registry)

	// Zone management tools (consolidated: manage_zone for full CRUD)
	RegisterZoneTools(registry)

	// Person management tools (consolidated: manage_person for full CRUD)
	RegisterPersonTools(registry)

	// Tag management tools (consolidated: manage_tag for full CRUD)
	RegisterTagTools(registry)

	// Entity query tools (consolidated: query_entities replaces get_states/get_history/get_statistics/list_domains)
	RegisterConsolidatedEntityQueryTools(registry)

	// Device query tools (query_devices for device health check)
	RegisterDeviceQueryTools(registry)

	// Entity and device registry management tools
	RegisterEntityManageTools(registry)
	RegisterDeviceManageTools(registry)

	// Media and advanced handlers
	RegisterMediaTools(registry)
	RegisterLovelaceTools(registry)

	// Target tools (consolidated: analyze_target replaces get_triggers/conditions/services/extract_for_target)
	RegisterConsolidatedTargetTools(registry)

	// Analysis tools for entity dependency tracking
	RegisterAnalysisTools(registry)

	// Service discovery and system information
	RegisterServiceTools(registry)
	RegisterSystemTools(registry)
	RegisterDatetimeTools(registry)

	// Template, logbook, system log, and configuration tools
	RegisterTemplateTools(registry)
	RegisterLogbookTools(registry)
	RegisterSystemLogTools(registry)
	RegisterConfigTools(registry)

	// Config entry tools (for accessing full config including template definitions)
	RegisterConfigEntryTools(registry)

	// HACS tools (Home Assistant Community Store management)
	RegisterHACSTools(registry)

	// Trace tools (automation and script execution traces)
	RegisterTraceTools(registry)

	// Blueprint tools (automation and script blueprints)
	RegisterBlueprintTools(registry)

	// Update tools (system and add-on updates)
	RegisterUpdateTools(registry)

	// Todo tools (todo and shopping lists)
	RegisterTodoTools(registry)

	// Calendar tools (calendar event management)
	RegisterCalendarTools(registry)

	// Camera tools (snapshot and stream access)
	RegisterCameraTools(registry)
}
