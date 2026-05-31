package mcp

// ActionCategory represents whether an action performs read or write operations.
type ActionCategory string

const (
	// CategoryRead indicates an action that only reads data without modifications.
	CategoryRead ActionCategory = "read"
	// CategoryWrite indicates an action that modifies data.
	CategoryWrite ActionCategory = "write"
)

// ToolClassification describes how a tool's actions are categorized for access control.
type ToolClassification struct {
	// ParamName is the parameter name that determines the action ("action", "mode", "type", "info", or "" for pure tools).
	ParamName string
	// PureCategory is used when the entire tool has a single category (no action parameter).
	PureCategory ActionCategory
	// Actions maps action/mode/type values to their categories.
	Actions map[string]ActionCategory
	// SubActions maps mode values to nested action categories (e.g., query_entities mode=health action=remove).
	SubActions map[string]map[string]ActionCategory
}

// buildAccessControlMap returns a map of all MCP tools with their read/write classifications.
// This is the authoritative source for access control decisions.
func buildAccessControlMap() map[string]ToolClassification {
	result := make(map[string]ToolClassification)

	// Add pure read/write tools
	for name, classification := range buildPureTools() {
		result[name] = classification
	}

	// Add managed tools (CRUD operations)
	for name, classification := range buildManagedTools() {
		result[name] = classification
	}

	// Add query and analysis tools
	for name, classification := range buildQueryTools() {
		result[name] = classification
	}

	return result
}

// buildPureTools returns tools with a single category (no action parameter).
func buildPureTools() map[string]ToolClassification {
	return map[string]ToolClassification{
		// Pure read tools
		"get_state": {
			PureCategory: CategoryRead,
		},
		"get_entity_dependencies": {
			PureCategory: CategoryRead,
		},
		"analyze_entity": {
			PureCategory: CategoryRead,
		},
		"get_datetime": {
			PureCategory: CategoryRead,
		},
		"validate_config": {
			PureCategory: CategoryRead,
		},
		"render_template": {
			PureCategory: CategoryRead,
		},
		"get_skill": {
			PureCategory: CategoryRead,
		},

		// Pure write tools
		"call_service": {
			PureCategory: CategoryWrite,
		},
		"helper_action": {
			PureCategory: CategoryWrite,
		},
	}
}

// buildManagedTools returns tools with action-based CRUD operations.
func buildManagedTools() map[string]ToolClassification {
	result := make(map[string]ToolClassification)

	// Core automation/script/scene tools
	addCoreManagementTools(result)

	// Helper and entity management tools
	addHelperManagementTools(result)

	// Registry management tools (area, label, floor, etc.)
	addRegistryManagementTools(result)

	// Dashboard and HACS tools
	addSpecialManagementTools(result)

	return result
}

// addCoreManagementTools adds automation, script, and scene management tools.
func addCoreManagementTools(result map[string]ToolClassification) {
	result["manage_automation"] = ToolClassification{
		ParamName: "action",
		Actions: map[string]ActionCategory{
			"list": CategoryRead, "get": CategoryRead, "coverage": CategoryRead,
			"create": CategoryWrite, "update": CategoryWrite, "delete": CategoryWrite, "toggle": CategoryWrite, "patch": CategoryWrite,
		},
	}
	result["manage_script"] = ToolClassification{
		ParamName: "action",
		Actions: map[string]ActionCategory{
			"list": CategoryRead, "get": CategoryRead,
			"create": CategoryWrite, "update": CategoryWrite, "delete": CategoryWrite, "execute": CategoryWrite, "patch": CategoryWrite,
		},
	}
	result["manage_scene"] = ToolClassification{
		ParamName: "action",
		Actions: map[string]ActionCategory{
			"list": CategoryRead, "get": CategoryRead,
			"create": CategoryWrite, "update": CategoryWrite, "delete": CategoryWrite, "activate": CategoryWrite, "patch": CategoryWrite,
		},
	}
}

// addHelperManagementTools adds helper and entity/device management tools.
func addHelperManagementTools(result map[string]ToolClassification) {
	result["manage_helper"] = ToolClassification{
		ParamName: "action",
		Actions: map[string]ActionCategory{
			"list": CategoryRead, "get_details": CategoryRead,
			"create": CategoryWrite, "update": CategoryWrite, "delete": CategoryWrite,
		},
	}
	result["manage_entity"] = ToolClassification{
		ParamName: "action",
		Actions:   map[string]ActionCategory{"get": CategoryRead, "update": CategoryWrite, "delete": CategoryWrite},
	}
	result["manage_device"] = ToolClassification{
		ParamName: "action",
		Actions:   map[string]ActionCategory{"get": CategoryRead, "update": CategoryWrite, "delete": CategoryWrite},
	}
	result["manage_config_entry"] = ToolClassification{
		ParamName: "action",
		Actions:   map[string]ActionCategory{"list": CategoryRead, "get": CategoryRead},
	}
}

// addRegistryManagementTools adds area, label, floor, zone, person, and tag management tools.
func addRegistryManagementTools(result map[string]ToolClassification) {
	crudActions := map[string]ActionCategory{
		"list": CategoryRead, "get": CategoryRead,
		"create": CategoryWrite, "update": CategoryWrite, "delete": CategoryWrite,
	}

	for _, tool := range []string{"manage_area", "manage_label", "manage_floor", "manage_zone", "manage_person", "manage_tag"} {
		result[tool] = ToolClassification{ParamName: "action", Actions: crudActions}
	}
}

// addSpecialManagementTools adds dashboard and HACS management tools.
func addSpecialManagementTools(result map[string]ToolClassification) {
	result["manage_dashboard"] = ToolClassification{
		ParamName: "action",
		Actions: map[string]ActionCategory{
			"list": CategoryRead, "get": CategoryRead,
			"create": CategoryWrite, "update": CategoryWrite, "delete": CategoryWrite, "save_config": CategoryWrite, "patch": CategoryWrite,
		},
	}
	result["manage_hacs"] = ToolClassification{
		ParamName: "action",
		Actions: map[string]ActionCategory{
			"info": CategoryRead, "list": CategoryRead, "get": CategoryRead,
			"releases": CategoryRead, "release_notes": CategoryRead, "critical": CategoryRead,
			"download": CategoryWrite, "uninstall": CategoryWrite, "add_repository": CategoryWrite,
			"remove_repository": CategoryWrite, "refresh": CategoryWrite, "toggle_beta": CategoryWrite,
		},
	}
	result["manage_trace"] = ToolClassification{
		ParamName: "action",
		Actions:   map[string]ActionCategory{"list": CategoryRead, "get": CategoryRead, "debug": CategoryRead},
	}
	result["manage_blueprint"] = ToolClassification{
		ParamName: "action",
		Actions:   map[string]ActionCategory{"list": CategoryRead, "import": CategoryWrite},
	}
	result["manage_update"] = ToolClassification{
		ParamName: "action",
		Actions: map[string]ActionCategory{
			"list": CategoryRead, "release_notes": CategoryRead,
			"install": CategoryWrite, "skip": CategoryWrite,
		},
	}
	result["manage_todo"] = ToolClassification{
		ParamName: "action",
		Actions: map[string]ActionCategory{
			"list": CategoryRead, "get_items": CategoryRead,
			"add_item": CategoryWrite, "update_item": CategoryWrite, "remove_item": CategoryWrite,
		},
	}
	result["manage_calendar"] = ToolClassification{
		ParamName: "action",
		Actions: map[string]ActionCategory{
			"list": CategoryRead, "get_events": CategoryRead,
			"create_event": CategoryWrite, "delete_event": CategoryWrite,
		},
	}
	result["manage_camera"] = ToolClassification{
		ParamName: "action",
		Actions:   map[string]ActionCategory{"snapshot": CategoryRead, "stream": CategoryRead},
	}
	result["manage_system_log"] = ToolClassification{
		ParamName: "action",
		Actions:   map[string]ActionCategory{"list": CategoryRead, "clear": CategoryWrite},
	}
}

// buildQueryTools returns query and analysis tools with mode/type/info parameters.
func buildQueryTools() map[string]ToolClassification {
	return map[string]ToolClassification{
		"get_registry": {
			ParamName: "type",
			Actions: map[string]ActionCategory{
				"entities": CategoryRead,
				"devices":  CategoryRead,
				"areas":    CategoryRead,
				"all":      CategoryRead,
			},
		},
		"analyze_target": {
			ParamName: "info",
			Actions: map[string]ActionCategory{
				"triggers":   CategoryRead,
				"conditions": CategoryRead,
				"services":   CategoryRead,
				"entities":   CategoryRead,
				"all":        CategoryRead,
			},
		},
		"get_logbook": {
			ParamName: "mode",
			Actions: map[string]ActionCategory{
				"entries":     CategoryRead,
				"correlation": CategoryRead,
			},
		},
		"query_entities": {
			ParamName: "mode",
			Actions: map[string]ActionCategory{
				"current":    CategoryRead,
				"history":    CategoryRead,
				"statistics": CategoryRead,
				"domains":    CategoryRead,
				"presence":   CategoryRead,
				"health":     CategoryRead,
			},
		},
		"query_devices": {
			ParamName: "mode",
			Actions: map[string]ActionCategory{
				"health": CategoryRead,
			},
		},
	}
}
