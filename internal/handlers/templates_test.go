package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestTemplateHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewTemplateHandlers()
	registry := mcp.NewRegistry()
	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "render_template" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected render_template tool to be registered")
	}
}

func TestTemplateHandlers_RenderTemplateTool(t *testing.T) {
	t.Parallel()

	h := NewTemplateHandlers()
	tool := h.renderTemplateTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "render_template",
		RequiredParams:  []string{"template"},
		OptionalParams:  []string{},
		WantDescription: true,
	})
}

func TestTemplateHandlers_HandleRenderTemplate(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "successful render",
			args: map[string]any{"template": "{{ states('light.living_room') }}"},
			setupMock: func(m *UniversalMockClient) {
				m.RenderTemplateFn = func(_ context.Context, _ string) (string, error) {
					return "on", nil
				}
			},
			wantError:    false,
			wantContains: []string{"on"},
		},
		{
			name:         "missing template",
			args:         map[string]any{},
			wantError:    true,
			wantContains: []string{"template is required"},
		},
		{
			name:         "empty template",
			args:         map[string]any{"template": ""},
			wantError:    true,
			wantContains: []string{"template is required"},
		},
		{
			name: "API error",
			args: map[string]any{"template": "{{ invalid }}"},
			setupMock: func(m *UniversalMockClient) {
				m.RenderTemplateFn = func(_ context.Context, _ string) (string, error) {
					return "", errors.New("template error: undefined variable")
				}
			},
			wantError:    true,
			wantContains: []string{"Error rendering template", "undefined variable"},
		},
	}

	h := NewTemplateHandlers()
	runHandlerTestCases(t, tests, h.handleRenderTemplate)
}

func TestTemplateHandlers_ComplexTemplate(t *testing.T) {
	t.Parallel()

	template := "The living room light is {{ states('light.living_room') }} and the temperature is {{ state_attr('sensor.temp', 'temperature') }}°C"
	expectedResult := "The living room light is on and the temperature is 22°C"

	client := &UniversalMockClient{
		RenderTemplateFn: func(_ context.Context, _ string) (string, error) {
			return expectedResult, nil
		},
	}

	h := NewTemplateHandlers()
	result, err := h.handleRenderTemplate(context.Background(), client, map[string]any{
		"template": template,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("expected no error in result")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, expectedResult) {
		t.Errorf("expected output to contain %q, got %q", expectedResult, content)
	}
}
