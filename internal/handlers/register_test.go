package handlers

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestRegisterEntityTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterEntityTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterEntityTools() registered no tools")
	}
}

func TestRegisterAutomationTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterAutomationTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterAutomationTools() registered no tools")
	}
}

func TestRegisterConsolidatedHelperTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterConsolidatedHelperTools(registry)

	tools := registry.ListTools()
	if len(tools) != 2 {
		t.Errorf("RegisterConsolidatedHelperTools() registered %d tools, want 2", len(tools))
	}

	// Verify both consolidated tools are registered
	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	if !toolMap["manage_helper"] {
		t.Error("RegisterConsolidatedHelperTools() did not register manage_helper")
	}
	if !toolMap["helper_action"] {
		t.Error("RegisterConsolidatedHelperTools() did not register helper_action")
	}
}

func TestRegisterMediaTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterMediaTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterMediaTools() registered no tools")
	}
}

func TestRegisterConsolidatedEntityQueryTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterConsolidatedEntityQueryTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterConsolidatedEntityQueryTools() registered %d tools, want 1", len(tools))
	}

	// Verify query_entities is registered
	if len(tools) > 0 && tools[0].Name != "query_entities" {
		t.Errorf("RegisterConsolidatedEntityQueryTools() registered %q, want %q", tools[0].Name, "query_entities")
	}
}

func TestRegisterLovelaceTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterLovelaceTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterLovelaceTools() registered no tools")
	}
}

func TestRegisterDatetimeTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterDatetimeTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterDatetimeTools() registered %d tools, want 1", len(tools))
	}

	// Verify get_datetime is registered
	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	if !toolMap["get_datetime"] {
		t.Error("RegisterDatetimeTools() did not register get_datetime")
	}
}

func TestRegisterEntityManageTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterEntityManageTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterEntityManageTools() registered %d tools, want 1", len(tools))
	}

	if len(tools) > 0 && tools[0].Name != "manage_entity" {
		t.Errorf("RegisterEntityManageTools() registered %q, want %q", tools[0].Name, "manage_entity")
	}
}

func TestRegisterDeviceManageTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterDeviceManageTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterDeviceManageTools() registered %d tools, want 1", len(tools))
	}

	if len(tools) > 0 && tools[0].Name != "manage_device" {
		t.Errorf("RegisterDeviceManageTools() registered %q, want %q", tools[0].Name, "manage_device")
	}
}

func TestRegisterSkillTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterSkillTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterSkillTools() registered %d tools, want 1", len(tools))
	}

	if len(tools) > 0 && tools[0].Name != "get_skill" {
		t.Errorf("RegisterSkillTools() registered %q, want get_skill", tools[0].Name)
	}
}

func TestRegisterAllTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterAllTools(registry)

	tools := registry.ListTools()

	// RegisterAllTools should register a significant number of tools
	// At minimum, we expect tools from all major handler categories
	// After consolidation: 42+ helper tools reduced to 2, +2 for entity/device manage, config entries 2→1
	const minExpectedTools = 25 // Conservative minimum after tool consolidation
	if len(tools) < minExpectedTools {
		t.Errorf("RegisterAllTools() registered %d tools, want at least %d", len(tools), minExpectedTools)
	}

	// Verify some key tools are registered
	expectedKeyTools := []string{
		// Entity tools
		"get_state",
		"call_service",
		// Consolidated automation/script/scene tools
		"manage_automation",
		"manage_script",
		"manage_scene",
		// Consolidated helper tools (manage_helper replaces 14 create_* tools)
		"manage_helper",
		"helper_action",
		// Consolidated registry tools (get_registry replaces list_entity/device/area_registry)
		"get_registry",
		// Registry management tools
		"manage_entity",
		"manage_device",
		// Media
		"browse_media",
		// Consolidated entity query tools (query_entities replaces get_states/get_history/get_statistics/list_domains)
		"query_entities",
		// Dashboard (replaces get_lovelace_config)
		"manage_dashboard",
		// Consolidated target tools (analyze_target replaces get_triggers/conditions/services_for_target)
		"analyze_target",
		// Date/Time
		"get_datetime",
		// Config entries (consolidated)
		"manage_config_entry",
		// Skill guidance (tool-only client fallback)
		"get_skill",
	}

	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	for _, expected := range expectedKeyTools {
		if !toolMap[expected] {
			t.Errorf("RegisterAllTools() did not register expected tool %q", expected)
		}
	}
}

func TestRegisterAllTools_NoDuplicates(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterAllTools(registry)

	tools := registry.ListTools()

	// Check for duplicate tool names
	seen := make(map[string]bool)
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Errorf("RegisterAllTools() registered duplicate tool %q", tool.Name)
		}
		seen[tool.Name] = true
	}
}

func TestRegisterAllTools_AllToolsHaveDescriptions(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterAllTools(registry)

	tools := registry.ListTools()

	for _, tool := range tools {
		if tool.Description == "" {
			t.Errorf("Tool %q has no description", tool.Name)
		}
	}
}

func TestRegisterAllTools_AllToolsHaveInputSchema(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterAllTools(registry)

	tools := registry.ListTools()

	for _, tool := range tools {
		if tool.InputSchema.Type == "" {
			t.Errorf("Tool %q has no input schema type", tool.Name)
		}
	}
}

// validJSONSchemaTypes is the set of valid JSON Schema type values per draft 2020-12.
var validJSONSchemaTypes = map[string]bool{
	"null":    true,
	"boolean": true,
	"object":  true,
	"array":   true,
	"number":  true,
	"string":  true,
	"integer": true,
}

// validateSchemaNode recursively validates a schema node for JSON Schema draft 2020-12 compliance.
// It reports violations as formatted strings containing the property path.
func validateSchemaNode(node map[string]any, path string, violations *[]string) {
	typeVal, hasType := node["type"]
	if hasType {
		typeStr, ok := typeVal.(string)
		if !ok {
			*violations = append(*violations, fmt.Sprintf("%s: 'type' is not a string: %T", path, typeVal))
		} else if typeStr == "" {
			*violations = append(*violations, fmt.Sprintf("%s: 'type' is empty string (must be omitted or a valid JSON Schema type)", path))
		} else if !validJSONSchemaTypes[typeStr] {
			*violations = append(*violations, fmt.Sprintf("%s: 'type' = %q is not a valid JSON Schema type", path, typeStr))
		}

		if typeStr == "array" {
			if _, hasItems := node["items"]; !hasItems {
				*violations = append(*violations, fmt.Sprintf("%s: type=array is missing 'items'", path))
			}
		}
	}

	if items, ok := node["items"]; ok {
		if itemsMap, ok := items.(map[string]any); ok {
			validateSchemaNode(itemsMap, path+"/items", violations)
		}
	}

	if props, ok := node["properties"]; ok {
		if propsMap, ok := props.(map[string]any); ok {
			for propName, propVal := range propsMap {
				if propMap, ok := propVal.(map[string]any); ok {
					validateSchemaNode(propMap, path+"/properties/"+propName, violations)
				}
			}
		}
	}
}

func TestRegisterAllTools_SchemaCompliance(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterAllTools(registry)
	tools := registry.ListTools()

	var allViolations []string

	for _, tool := range tools {
		// Serialize and re-parse so we see exactly what the API would receive.
		schemaData, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Errorf("Tool %q: failed to marshal InputSchema: %v", tool.Name, err)
			continue
		}
		var schemaMap map[string]any
		if err := json.Unmarshal(schemaData, &schemaMap); err != nil {
			t.Errorf("Tool %q: failed to unmarshal InputSchema JSON: %v", tool.Name, err)
			continue
		}

		var violations []string
		validateSchemaNode(schemaMap, tool.Name, &violations)
		allViolations = append(allViolations, violations...)
	}

	for _, v := range allViolations {
		t.Error(v)
	}
}
