// internal/handlers/skill_test.go
package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/handlers/skills"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// ─── RegisterAllResources ───────────────────────────────────────────────────

func TestRegisterAllResources_Count(t *testing.T) {
	t.Parallel()
	registry := mcp.NewRegistry()
	RegisterAllResources(registry)
	got := registry.ResourceCount()
	want := len(skills.Catalog)
	if got != want {
		t.Errorf("ResourceCount() = %d, want %d", got, want)
	}
}

func TestRegisterAllResources_URIsAndMimeType(t *testing.T) {
	t.Parallel()
	registry := mcp.NewRegistry()
	RegisterAllResources(registry)
	seen := make(map[string]bool)
	for _, r := range registry.ListResources() {
		if seen[r.URI] {
			t.Errorf("duplicate resource URI %q", r.URI)
		}
		seen[r.URI] = true
		if !strings.HasPrefix(r.URI, skills.URIPrefix) {
			t.Errorf("resource URI %q does not start with %q", r.URI, skills.URIPrefix)
		}
		if r.MimeType != "text/markdown" {
			t.Errorf("resource %q MimeType = %q, want text/markdown", r.URI, r.MimeType)
		}
		if r.Name == "" {
			t.Errorf("resource %q has empty Name", r.URI)
		}
		if r.Description == "" {
			t.Errorf("resource %q has empty Description", r.URI)
		}
	}
}

func TestRegisterAllResources_HandlersReturnContent(t *testing.T) {
	t.Parallel()
	registry := mcp.NewRegistry()
	RegisterAllResources(registry)
	for _, r := range registry.ListResources() {
		handler, ok := registry.GetResourceHandler(r.URI)
		if !ok {
			t.Errorf("no handler found for resource %q", r.URI)
			continue
		}
		result, err := handler(context.Background(), &UniversalMockClient{}, r.URI)
		if err != nil {
			t.Errorf("handler for %q returned error: %v", r.URI, err)
			continue
		}
		if len(result.Contents) == 0 {
			t.Errorf("handler for %q returned no contents", r.URI)
			continue
		}
		if result.Contents[0].Text == "" {
			t.Errorf("handler for %q returned empty text", r.URI)
		}
		if result.Contents[0].URI != r.URI {
			t.Errorf("handler for %q returned content with URI %q, want %q", r.URI, result.Contents[0].URI, r.URI)
		}
		if result.Contents[0].MimeType != "text/markdown" {
			t.Errorf("handler for %q returned content MimeType %q, want text/markdown", r.URI, result.Contents[0].MimeType)
		}
	}
}

// ─── SkillHandlers tool registration ────────────────────────────────────────

func TestSkillHandlers_RegisterTools(t *testing.T) {
	t.Parallel()
	h := NewSkillHandlers()
	registry := mcp.NewRegistry()
	h.RegisterTools(registry)
	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Fatalf("RegisterTools() registered %d tools, want 1", len(tools))
	}
	if tools[0].Name != "get_skill" {
		t.Errorf("tool name = %q, want get_skill", tools[0].Name)
	}
}

func TestSkillHandlers_Schema(t *testing.T) {
	t.Parallel()
	h := NewSkillHandlers()
	tool := h.getSkillTool()
	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "get_skill",
		RequiredParams:  []string{"action"},
		OptionalParams:  []string{"skill", "format"},
		WantDescription: true,
	})
}

// ─── handleGetSkill behavior ─────────────────────────────────────────────────

func TestSkillHandlers_HandleGetSkill(t *testing.T) {
	t.Parallel()
	h := NewSkillHandlers()
	tests := []handlerTestCase{
		{
			name:      "list natural lists all 7 slugs",
			args:      map[string]any{"action": "list"},
			wantError: false,
			wantContains: []string{
				"format-selection",
				"automation-patterns",
				"template-resilience",
				"helper-selection",
				"dashboard-safety",
				"entity-renaming",
				"debugging-workflow",
			},
		},
		{
			name:         "list json returns structured output with slug field",
			args:         map[string]any{"action": "list", "format": "json"},
			wantError:    false,
			wantContains: []string{"format-selection", `"slug"`},
		},
		{
			name:         "read valid skill returns non-empty content",
			args:         map[string]any{"action": "read", "skill": "format-selection"},
			wantError:    false,
			wantContains: []string{"natural", "json"},
		},
		{
			name:         "read automation-patterns returns mode table",
			args:         map[string]any{"action": "read", "skill": "automation-patterns"},
			wantError:    false,
			wantContains: []string{"restart", "queued", "parallel"},
		},
		{
			name:         "read json format returns slug and content fields",
			args:         map[string]any{"action": "read", "skill": "debugging-workflow", "format": "json"},
			wantError:    false,
			wantContains: []string{`"slug"`, `"content"`, "debugging-workflow"},
		},
		{
			name:         "read unknown slug returns IsError with slug name and valid slugs listed",
			args:         map[string]any{"action": "read", "skill": "bogus-unknown-slug"},
			wantError:    true,
			wantContains: []string{"bogus-unknown-slug", "format-selection"},
		},
		{
			name:         "read missing skill param returns IsError",
			args:         map[string]any{"action": "read"},
			wantError:    true,
			wantContains: []string{"skill is required"},
		},
		{
			name:         "invalid action returns IsError",
			args:         map[string]any{"action": "delete"},
			wantError:    true,
			wantContains: []string{"invalid action", "delete"},
		},
		{
			name:         "missing action treated as invalid",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"invalid action"},
		},
	}
	runHandlerTestCases(t, tests, h.handleGetSkill)
}
