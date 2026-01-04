// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// LovelaceHandlers provides handlers for Home Assistant Lovelace dashboard operations.
type LovelaceHandlers struct{}

// NewLovelaceHandlers creates a new LovelaceHandlers instance.
func NewLovelaceHandlers() *LovelaceHandlers {
	return &LovelaceHandlers{}
}

// RegisterTools registers all lovelace-related tools with the registry.
func (h *LovelaceHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getLovelaceConfigTool(), h.handleGetLovelaceConfig)
}

// getLovelaceConfigTool returns the tool definition for getting Lovelace configuration.
func (h *LovelaceHandlers) getLovelaceConfigTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_lovelace_config",
		Description: "Get the Lovelace dashboard configuration. Returns the complete dashboard layout including views, cards, and their configurations.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Properties:  map[string]mcp.JSONSchema{},
			Description: "No parameters required",
		},
	}
}

// handleGetLovelaceConfig handles requests to get Lovelace dashboard configuration.
func (h *LovelaceHandlers) handleGetLovelaceConfig(
	ctx context.Context,
	client homeassistant.Client,
	_ map[string]any,
) (*mcp.ToolsCallResult, error) {
	config, err := client.GetLovelaceConfig(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting Lovelace config: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(string(output)),
		},
	}, nil
}
