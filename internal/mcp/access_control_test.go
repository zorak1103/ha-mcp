package mcp

import (
	"testing"
)

// TestAccessControlMapCompleteness verifies that all expected tools are classified.
func TestAccessControlMapCompleteness(t *testing.T) {
	t.Parallel()

	// Expected tools count based on CLAUDE.md documentation
	expectedTools := []string{
		// State and query
		"get_state",
		"query_entities",
		"query_devices",
		"get_entity_dependencies",
		"analyze_entity",
		"find_references",
		"analyze_target",
		"get_registry",
		"get_logbook",
		"get_datetime",

		// Service operations
		"call_service",

		// Manageable objects
		"manage_automation",
		"manage_script",
		"manage_scene",
		"manage_helper",
		"helper_action",
		"manage_entity",
		"manage_device",
		"manage_area",
		"manage_label",
		"manage_floor",
		"manage_zone",
		"manage_person",
		"manage_tag",
		"manage_dashboard",
		"manage_config_entry",
		"manage_hacs",
		"manage_trace",
		"manage_blueprint",
		"manage_update",
		"manage_todo",
		"manage_calendar",
		"manage_camera",
		"manage_system_log",

		// Media and rendering
		"render_template",
		"validate_config",

		// Skill guidance (tool-only client fallback)
		"get_skill",
	}

	accessMap := buildAccessControlMap()

	if len(accessMap) != len(expectedTools) {
		t.Errorf("Access control map has %d tools, expected %d", len(accessMap), len(expectedTools))
	}

	for _, toolName := range expectedTools {
		if _, exists := accessMap[toolName]; !exists {
			t.Errorf("Tool %q is missing from access control map", toolName)
		}
	}
}

// TestPureReadTools verifies that pure read tools are correctly classified.
func TestPureReadTools(t *testing.T) {
	t.Parallel()

	accessMap := buildAccessControlMap()

	pureReadTools := []string{
		"get_state",
		"get_entity_dependencies",
		"analyze_entity",
		"find_references",
		"get_datetime",
		"validate_config",
		"render_template",
		"get_skill",
	}

	for _, toolName := range pureReadTools {
		classification, exists := accessMap[toolName]
		if !exists {
			t.Errorf("Tool %q not found in access control map", toolName)
			continue
		}

		if classification.ParamName != "" {
			t.Errorf("Tool %q should have empty ParamName, got %q", toolName, classification.ParamName)
		}

		if classification.PureCategory != CategoryRead {
			t.Errorf("Tool %q should be CategoryRead, got %q", toolName, classification.PureCategory)
		}

		if len(classification.Actions) != 0 {
			t.Errorf("Tool %q should have empty Actions map, got %d entries", toolName, len(classification.Actions))
		}
	}
}

// TestPureWriteTools verifies that pure write tools are correctly classified.
func TestPureWriteTools(t *testing.T) {
	t.Parallel()

	accessMap := buildAccessControlMap()

	pureWriteTools := []string{
		"call_service",
		"helper_action",
	}

	for _, toolName := range pureWriteTools {
		classification, exists := accessMap[toolName]
		if !exists {
			t.Errorf("Tool %q not found in access control map", toolName)
			continue
		}

		if classification.PureCategory != CategoryWrite {
			t.Errorf("Tool %q should be CategoryWrite, got %q", toolName, classification.PureCategory)
		}
	}
}

// TestManagedToolsActions verifies action-based tools have correct read/write classification.
func TestManagedToolsActions(t *testing.T) {
	t.Parallel()

	accessMap := buildAccessControlMap()

	tests := []struct {
		tool         string
		paramName    string
		readActions  []string
		writeActions []string
	}{
		{
			tool:         "manage_automation",
			paramName:    "action",
			readActions:  []string{"list", "get", "coverage"},
			writeActions: []string{"create", "update", "delete", "toggle"},
		},
		{
			tool:         "manage_script",
			paramName:    "action",
			readActions:  []string{"list", "get"},
			writeActions: []string{"create", "update", "delete", "execute"},
		},
		{
			tool:         "manage_scene",
			paramName:    "action",
			readActions:  []string{"list", "get"},
			writeActions: []string{"create", "update", "delete", "activate"},
		},
		{
			tool:         "manage_helper",
			paramName:    "action",
			readActions:  []string{"list", "get_details"},
			writeActions: []string{"create", "update", "delete"},
		},
		{
			tool:         "manage_area",
			paramName:    "action",
			readActions:  []string{"list", "get"},
			writeActions: []string{"create", "update", "delete"},
		},
		{
			tool:         "manage_label",
			paramName:    "action",
			readActions:  []string{"list", "get"},
			writeActions: []string{"create", "update", "delete"},
		},
		{
			tool:         "manage_floor",
			paramName:    "action",
			readActions:  []string{"list", "get"},
			writeActions: []string{"create", "update", "delete"},
		},
		{
			tool:         "manage_zone",
			paramName:    "action",
			readActions:  []string{"list", "get"},
			writeActions: []string{"create", "update", "delete"},
		},
		{
			tool:         "manage_person",
			paramName:    "action",
			readActions:  []string{"list", "get"},
			writeActions: []string{"create", "update", "delete"},
		},
		{
			tool:         "manage_tag",
			paramName:    "action",
			readActions:  []string{"list", "get"},
			writeActions: []string{"create", "update", "delete"},
		},
		{
			tool:         "manage_entity",
			paramName:    "action",
			readActions:  []string{"get"},
			writeActions: []string{"update", "delete"},
		},
		{
			tool:         "manage_device",
			paramName:    "action",
			readActions:  []string{"get"},
			writeActions: []string{"update", "delete"},
		},
		{
			tool:         "manage_dashboard",
			paramName:    "action",
			readActions:  []string{"list", "get", "find"},
			writeActions: []string{"create", "update", "delete", "save_config", "patch"},
		},
		{
			tool:         "manage_config_entry",
			paramName:    "action",
			readActions:  []string{"list", "get"},
			writeActions: []string{},
		},
		{
			tool:         "manage_system_log",
			paramName:    "action",
			readActions:  []string{"list"},
			writeActions: []string{"clear"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			classification, exists := accessMap[tt.tool]
			if !exists {
				t.Fatalf("Tool %q not found in access control map", tt.tool)
			}

			if classification.ParamName != tt.paramName {
				t.Errorf("ParamName = %q, want %q", classification.ParamName, tt.paramName)
			}

			// Verify read actions
			for _, action := range tt.readActions {
				category, exists := classification.Actions[action]
				if !exists {
					t.Errorf("Action %q not found in tool %q", action, tt.tool)
					continue
				}
				if category != CategoryRead {
					t.Errorf("Action %q should be CategoryRead, got %q", action, category)
				}
			}

			// Verify write actions
			for _, action := range tt.writeActions {
				category, exists := classification.Actions[action]
				if !exists {
					t.Errorf("Action %q not found in tool %q", action, tt.tool)
					continue
				}
				if category != CategoryWrite {
					t.Errorf("Action %q should be CategoryWrite, got %q", action, category)
				}
			}
		})
	}
}

// TestHACSActions verifies HACS tool has correct action classification.
func TestHACSActions(t *testing.T) {
	t.Parallel()

	accessMap := buildAccessControlMap()
	classification := accessMap["manage_hacs"]

	if classification.ParamName != "action" {
		t.Errorf("ParamName = %q, want %q", classification.ParamName, "action")
	}

	readActions := []string{"info", "list", "get", "releases", "release_notes", "critical"}
	writeActions := []string{"download", "uninstall", "add_repository", "remove_repository", "refresh", "toggle_beta"}

	for _, action := range readActions {
		if classification.Actions[action] != CategoryRead {
			t.Errorf("HACS action %q should be CategoryRead, got %q", action, classification.Actions[action])
		}
	}

	for _, action := range writeActions {
		if classification.Actions[action] != CategoryWrite {
			t.Errorf("HACS action %q should be CategoryWrite, got %q", action, classification.Actions[action])
		}
	}
}

// TestRegistryTypesActions verifies registry tools have correct type classification.
func TestRegistryTypesActions(t *testing.T) {
	t.Parallel()

	accessMap := buildAccessControlMap()
	classification := accessMap["get_registry"]

	if classification.ParamName != "type" {
		t.Errorf("ParamName = %q, want %q", classification.ParamName, "type")
	}

	types := []string{"entities", "devices", "areas", "all"}
	for _, typeVal := range types {
		if classification.Actions[typeVal] != CategoryRead {
			t.Errorf("Registry type %q should be CategoryRead, got %q", typeVal, classification.Actions[typeVal])
		}
	}
}

// TestAnalyzeTargetInfo verifies analyze_target has correct info classification.
func TestAnalyzeTargetInfo(t *testing.T) {
	t.Parallel()

	accessMap := buildAccessControlMap()
	classification := accessMap["analyze_target"]

	if classification.ParamName != "info" {
		t.Errorf("ParamName = %q, want %q", classification.ParamName, "info")
	}

	infoTypes := []string{"triggers", "conditions", "services", "entities", "all"}
	for _, info := range infoTypes {
		if classification.Actions[info] != CategoryRead {
			t.Errorf("Target info %q should be CategoryRead, got %q", info, classification.Actions[info])
		}
	}
}

// TestQueryEntityModes verifies query_entities modes are all read-only.
func TestQueryEntityModes(t *testing.T) {
	t.Parallel()

	accessMap := buildAccessControlMap()
	classification := accessMap["query_entities"]

	if classification.ParamName != "mode" {
		t.Errorf("ParamName = %q, want %q", classification.ParamName, "mode")
	}

	readModes := []string{"current", "history", "statistics", "domains", "presence", "health"}
	for _, mode := range readModes {
		if classification.Actions[mode] != CategoryRead {
			t.Errorf("Mode %q should be CategoryRead, got %q", mode, classification.Actions[mode])
		}
	}

	if len(classification.SubActions) != 0 {
		t.Errorf("query_entities should have no SubActions, got %d", len(classification.SubActions))
	}
}

// TestQueryDeviceModes verifies query_devices modes are all read-only.
func TestQueryDeviceModes(t *testing.T) {
	t.Parallel()

	accessMap := buildAccessControlMap()
	classification := accessMap["query_devices"]

	if classification.ParamName != "mode" {
		t.Errorf("ParamName = %q, want %q", classification.ParamName, "mode")
	}

	if classification.Actions["health"] != CategoryRead {
		t.Errorf("Mode %q should be CategoryRead, got %q", "health", classification.Actions["health"])
	}

	if len(classification.SubActions) != 0 {
		t.Errorf("query_devices should have no SubActions, got %d", len(classification.SubActions))
	}
}

// TestLogbookModes verifies get_logbook modes.
func TestLogbookModes(t *testing.T) {
	t.Parallel()

	accessMap := buildAccessControlMap()
	classification := accessMap["get_logbook"]

	if classification.ParamName != "mode" {
		t.Errorf("ParamName = %q, want %q", classification.ParamName, "mode")
	}

	modes := []string{"entries", "correlation"}
	for _, mode := range modes {
		if classification.Actions[mode] != CategoryRead {
			t.Errorf("Logbook mode %q should be CategoryRead, got %q", mode, classification.Actions[mode])
		}
	}
}
