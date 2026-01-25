// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TemplateHandlers provides handlers for Home Assistant template rendering operations.
type TemplateHandlers struct{}

// NewTemplateHandlers creates a new TemplateHandlers instance.
func NewTemplateHandlers() *TemplateHandlers {
	return &TemplateHandlers{}
}

// RegisterTools registers all template-related tools with the registry.
func (h *TemplateHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.renderTemplateTool(), h.handleRenderTemplate)
}

// renderTemplateTool returns the tool definition for rendering a Jinja2 template.
func (h *TemplateHandlers) renderTemplateTool() mcp.Tool {
	return mcp.Tool{
		Name:        "render_template",
		Description: "Render a Jinja2 template using current Home Assistant state. Templates can access entity states, attributes, and use Home Assistant template functions.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"template": {
					Type:        "string",
					Description: "Jinja2 template string to render (e.g., '{{ states(\"light.living_room\") }}' or 'The temperature is {{ state_attr(\"sensor.temp\", \"temperature\") }}°C')",
				},
			},
			Required: []string{"template"},
		},
	}
}

// handleRenderTemplate handles requests to render a Jinja2 template.
func (h *TemplateHandlers) handleRenderTemplate(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	template := getString(args, "template")
	if template == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent("template is required"),
			},
			IsError: true,
		}, nil
	}

	result, err := client.RenderTemplate(ctx, template)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error rendering template: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(result),
		},
	}, nil
}
