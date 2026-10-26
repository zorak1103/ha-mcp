package mcp_test

import (
	"slices"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/handlers"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// registeredTools returns every tool the server registers, keyed by name.
func registeredTools(t *testing.T) map[string]mcp.Tool {
	t.Helper()

	registry := mcp.NewRegistry()
	handlers.RegisterAllTools(registry)

	tools := make(map[string]mcp.Tool)
	for _, tool := range registry.ListTools() {
		tools[tool.Name] = tool
	}

	return tools
}

// TestAccessControlMapMatchesRegistry fails when the access map and the real tool registry drift:
// a registered tool without a classification is denied at runtime under any filter, and a
// classification without a tool is dead config.
func TestAccessControlMapMatchesRegistry(t *testing.T) {
	t.Parallel()

	tools := registeredTools(t)
	accessMap := mcp.BuildAccessControlMap()

	for name := range tools {
		if _, ok := accessMap[name]; !ok {
			t.Errorf("tool %q is registered but missing from the access control map (internal/mcp/access_control.go)", name)
		}
	}

	for name := range accessMap {
		if _, ok := tools[name]; !ok {
			t.Errorf("tool %q is in the access control map but not registered", name)
		}
	}
}

// TestAccessControlActionsMatchSchemaEnums compares each action-based tool's classified actions
// with the enum its schema advertises, in both directions.
func TestAccessControlActionsMatchSchemaEnums(t *testing.T) {
	t.Parallel()

	tools := registeredTools(t)

	for name, classification := range mcp.BuildAccessControlMap() {
		tool, registered := tools[name]
		if !registered {
			continue // reported by TestAccessControlMapMatchesRegistry
		}

		if classification.ParamName == "" {
			if classification.PureCategory != mcp.CategoryRead && classification.PureCategory != mcp.CategoryWrite {
				t.Errorf("pure tool %q has no read/write category", name)
			}
			if len(classification.Actions) != 0 {
				t.Errorf("pure tool %q must not classify actions", name)
			}
			continue
		}

		prop, ok := tool.InputSchema.Properties[classification.ParamName]
		if !ok {
			t.Errorf("tool %q: schema has no %q property", name, classification.ParamName)
			continue
		}

		for action, category := range classification.Actions {
			if category != mcp.CategoryRead && category != mcp.CategoryWrite {
				t.Errorf("tool %q action %q has invalid category %q", name, action, category)
			}
			if !slices.Contains(prop.Enum, action) {
				t.Errorf("tool %q: classified action %q is not in the schema enum %v", name, action, prop.Enum)
			}
		}

		for _, value := range prop.Enum {
			if _, ok := classification.Actions[value]; !ok {
				t.Errorf("tool %q: schema enum value %q is not classified", name, value)
			}
		}
	}
}
