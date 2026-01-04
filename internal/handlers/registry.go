// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// RegistryHandlers provides handlers for Home Assistant registry operations.
type RegistryHandlers struct{}

// NewRegistryHandlers creates a new RegistryHandlers instance.
func NewRegistryHandlers() *RegistryHandlers {
	return &RegistryHandlers{}
}

// RegisterTools registers all registry-related tools with the registry.
func (h *RegistryHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.listEntityRegistryTool(), h.handleListEntityRegistry)
	registry.RegisterTool(h.listDeviceRegistryTool(), h.handleListDeviceRegistry)
	registry.RegisterTool(h.listAreaRegistryTool(), h.handleListAreaRegistry)
}

// listEntityRegistryTool returns the tool definition for listing entity registry entries.
func (h *RegistryHandlers) listEntityRegistryTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_entity_registry",
		Description: "List all entries in the Home Assistant entity registry. Returns detailed information about entities including their platform, device association, area, and configuration.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Properties:  map[string]mcp.JSONSchema{},
			Description: "No parameters required",
		},
	}
}

// handleListEntityRegistry handles requests to list entity registry entries.
func (h *RegistryHandlers) handleListEntityRegistry(
	ctx context.Context,
	client homeassistant.Client,
	_ map[string]any,
) (*mcp.ToolsCallResult, error) {
	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting entity registry: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(entries, "", "  ")
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

// listDeviceRegistryTool returns the tool definition for listing device registry entries.
func (h *RegistryHandlers) listDeviceRegistryTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_device_registry",
		Description: "List all entries in the Home Assistant device registry. Returns detailed information about devices including manufacturer, model, firmware version, and area assignment.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Properties:  map[string]mcp.JSONSchema{},
			Description: "No parameters required",
		},
	}
}

// handleListDeviceRegistry handles requests to list device registry entries.
func (h *RegistryHandlers) handleListDeviceRegistry(
	ctx context.Context,
	client homeassistant.Client,
	_ map[string]any,
) (*mcp.ToolsCallResult, error) {
	entries, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting device registry: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(entries, "", "  ")
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

// listAreaRegistryTool returns the tool definition for listing area registry entries.
func (h *RegistryHandlers) listAreaRegistryTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_area_registry",
		Description: "List all entries in the Home Assistant area registry. Returns information about defined areas including their names, pictures, and aliases.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Properties:  map[string]mcp.JSONSchema{},
			Description: "No parameters required",
		},
	}
}

// handleListAreaRegistry handles requests to list area registry entries.
func (h *RegistryHandlers) handleListAreaRegistry(
	ctx context.Context,
	client homeassistant.Client,
	_ map[string]any,
) (*mcp.ToolsCallResult, error) {
	entries, err := client.GetAreaRegistry(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting area registry: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(entries, "", "  ")
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
