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

func TestRegisterInputBooleanTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterInputBooleanTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterInputBooleanTools() registered no tools")
	}
}

func TestRegisterInputNumberTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterInputNumberTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterInputNumberTools() registered no tools")
	}
}

func TestRegisterInputTextTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterInputTextTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterInputTextTools() registered no tools")
	}
}

func TestRegisterInputSelectTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterInputSelectTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterInputSelectTools() registered no tools")
	}
}

func TestRegisterInputDatetimeTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterInputDatetimeTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterInputDatetimeTools() registered no tools")
	}
}

func TestRegisterInputButtonTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterInputButtonTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterInputButtonTools() registered no tools")
	}
}

func TestRegisterCounterTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterCounterTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterCounterTools() registered no tools")
	}
}

func TestRegisterTimerTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterTimerTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterTimerTools() registered no tools")
	}
}

func TestRegisterScheduleTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterScheduleTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterScheduleTools() registered no tools")
	}
}

func TestRegisterGroupTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterGroupTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterGroupTools() registered no tools")
	}
}

func TestRegisterTemplateHelperTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterTemplateHelperTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterTemplateHelperTools() registered no tools")
	}
}

func TestRegisterThresholdTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterThresholdTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterThresholdTools() registered no tools")
	}
}

func TestRegisterDerivativeTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterDerivativeTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterDerivativeTools() registered no tools")
	}
}

func TestRegisterIntegralTools(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterIntegralTools(registry)

	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("RegisterIntegralTools() registered no tools")
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
	const minExpectedTools = 50 // Conservative minimum
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
		// Helper tools
		"list_helpers",
		// Input helpers
		"create_input_boolean",
		"create_input_number",
		"create_input_text",
		"create_input_select",
		"create_input_datetime",
		"create_input_button",
		// Timer/Counter
		"create_counter",
		"create_timer",
		// Schedule/Group
		"create_group",
		// Template/Threshold/Derivative/Integral
		"create_template_sensor",
		"create_threshold",
		"create_derivative",
		"create_integral",
		// Media
		"browse_media",
		// Statistics
		"get_statistics",
		// Lovelace
		"get_lovelace_config",
		// Targets
		"get_triggers_for_target",
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
