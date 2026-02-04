package handlers

import (
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

func TestRegisterHelperTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterHelperTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterHelperTools() registered no tools")
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

func TestRegisterRegistryTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterRegistryTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterRegistryTools() registered no tools")
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

func TestRegisterStatisticsTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterStatisticsTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterStatisticsTools() registered no tools")
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

func TestRegisterAllTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterAllTools(registry)

	tools := registry.ListTools()

	// RegisterAllTools should register a significant number of tools
	// At minimum, we expect tools from all major handler categories
	// After consolidation: 42+ helper tools reduced to 2
	const minExpectedTools = 25 // Conservative minimum after consolidation
	if len(tools) < minExpectedTools {
		t.Errorf("RegisterAllTools() registered %d tools, want at least %d", len(tools), minExpectedTools)
	}

	// Verify some key tools are registered
	expectedKeyTools := []string{
		// Entity tools
		"get_state",
		"call_service",
		// Automation tools
		"list_automations",
		// Helper tools (generic list)
		"list_helpers",
		// Consolidated helper tools (manage_helper replaces 14 create_* tools)
		"manage_helper",
		"helper_action",
		// Media
		"browse_media",
		// Statistics
		"get_statistics",
		// Lovelace
		"get_lovelace_config",
		// Targets
		"get_triggers_for_target",
		// Config entries
		"list_config_entries",
		"get_config_entry",
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
