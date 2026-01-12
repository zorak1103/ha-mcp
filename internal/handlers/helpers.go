// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// HelperHandlers provides MCP tool handlers for generic helper operations.
// Individual helper types (input_boolean, input_number, etc.) have their own handlers.
type HelperHandlers struct{}

// NewHelperHandlers creates a new HelperHandlers instance.
func NewHelperHandlers() *HelperHandlers {
	return &HelperHandlers{}
}

// RegisterTools registers all generic helper-related tools with the registry.
// This only includes list_helpers. Type-specific tools are in separate handler files.
func (h *HelperHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.listHelpersTool(), h.handleListHelpers)
}

func (h *HelperHandlers) listHelpersTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_helpers",
		Description: "List all input helpers in Home Assistant (input_boolean, input_number, input_text, input_select, input_datetime)",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "No parameters required",
		},
	}
}

func (h *HelperHandlers) handleListHelpers(ctx context.Context, client homeassistant.Client, _ map[string]any) (*mcp.ToolsCallResult, error) {
	helpers, err := client.ListHelpers(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error listing helpers: %v", err))},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(helpers, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting helpers: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}
