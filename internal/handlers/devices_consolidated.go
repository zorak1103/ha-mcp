// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Device query modes.
const (
	deviceModeHealth = "health"
)

// Device health issue categories.
const (
	deviceCategoryDisabled            = "disabled"
	deviceCategoryOrphanedConfigEntry = "orphaned_config_entry"
	deviceCategoryConfigEntryError    = "config_entry_error"
	deviceCategoryNoEntities          = "no_entities"
	deviceCategoryNoConfigEntries     = "no_config_entries"
)

// DeviceQueryHandlers provides handlers for device query tools.
type DeviceQueryHandlers struct{}

// NewDeviceQueryHandlers creates a new DeviceQueryHandlers instance.
func NewDeviceQueryHandlers() *DeviceQueryHandlers {
	return &DeviceQueryHandlers{}
}

// RegisterTools registers device query tools with the MCP registry.
func (h *DeviceQueryHandlers) RegisterTools(registry *mcp.Registry) {
	queryDevicesTool := mcp.Tool{
		Name:        "query_devices",
		Description: "Query Home Assistant devices with different modes: health check (detect problematic devices)",
		InputSchema: buildQueryDevicesSchema(),
	}

	registry.RegisterTool(queryDevicesTool, h.handleQueryDevices)
}

func buildQueryDevicesSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type: "object",
		Properties: map[string]mcp.JSONSchema{
			"mode": {
				Type:        "string",
				Description: "Query mode: health (device health check)",
				Enum:        []string{deviceModeHealth},
			},
			"format": {
				Type:        "string",
				Description: "Output format: natural (LLM-friendly), json (structured)",
				Enum:        []string{"natural", "json"},
			},
			"categories": {
				Type:        "array",
				Description: "Filter health check categories (empty = all)",
				Items: &mcp.JSONSchema{
					Type: "string",
					Enum: []string{
						deviceCategoryDisabled,
						deviceCategoryOrphanedConfigEntry,
						deviceCategoryConfigEntryError,
						deviceCategoryNoEntities,
						deviceCategoryNoConfigEntries,
					},
				},
			},
			"manufacturer": {
				Type:        "string",
				Description: "Filter by manufacturer name",
			},
		},
		Required: []string{"mode"},
	}
}

// handleQueryDevices routes query_devices requests to the appropriate handler.
func (h *DeviceQueryHandlers) handleQueryDevices(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	mode, ok := args["mode"].(string)
	if !ok {
		return errorResult("mode parameter is required"), nil
	}

	switch mode {
	case deviceModeHealth:
		return h.handleDeviceHealth(ctx, client, args)
	default:
		return errorResult("mode must be one of: health"), nil
	}
}

// handleDeviceHealth is implemented in devices_health.go
