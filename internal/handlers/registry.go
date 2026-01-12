// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
		Description: "List entries in the Home Assistant entity registry. By default returns a compact list with only entity_id. Use filters to narrow down results and 'verbose' for full details. Note: Most entity info can also be obtained via get_states.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Filter and output options for entity registry",
			Properties: map[string]mcp.JSONSchema{
				"domain": {
					Type:        "string",
					Description: "Filter by domain (e.g., 'light', 'switch', 'sensor')",
				},
				"platform": {
					Type:        "string",
					Description: "Filter by platform/integration (e.g., 'fritz', 'hue', 'mqtt')",
				},
				"device_id": {
					Type:        "string",
					Description: "Filter by device ID to get all entities of a specific device",
				},
				"area_id": {
					Type:        "string",
					Description: "Filter by area ID to get all entities in a specific area",
				},
				"verbose": {
					Type:        "boolean",
					Description: "If true, return full details (platform, device_id, config_entry_id, unique_id, etc.). Default: false (compact output with entity_id only)",
				},
				"include_disabled": {
					Type:        "boolean",
					Description: "If true, include disabled entities. Default: false",
				},
			},
		},
	}
}

// compactEntityEntry represents a minimal entity registry entry for compact output.
type compactEntityEntry struct {
	EntityID string `json:"entity_id"`
	DeviceID string `json:"device_id,omitempty"`
	AreaID   string `json:"area_id,omitempty"`
}

// handleListEntityRegistry handles requests to list entity registry entries.
func (h *RegistryHandlers) handleListEntityRegistry(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
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

	// Parse filter parameters
	domainFilter, _ := args["domain"].(string)
	platformFilter, _ := args["platform"].(string)
	deviceIDFilter, _ := args["device_id"].(string)
	areaIDFilter, _ := args["area_id"].(string)
	verbose, _ := args["verbose"].(bool)
	includeDisabled, _ := args["include_disabled"].(bool)

	// Filter entries
	filtered := make([]homeassistant.EntityRegistryEntry, 0, len(entries))
	for _, entry := range entries {
		// Skip disabled entries unless explicitly requested
		if !includeDisabled && entry.DisabledBy != "" {
			continue
		}

		// Apply domain filter
		if domainFilter != "" {
			domain := extractDomain(entry.EntityID)
			if domain != domainFilter {
				continue
			}
		}

		// Apply platform filter
		if platformFilter != "" && entry.Platform != platformFilter {
			continue
		}

		// Apply device_id filter
		if deviceIDFilter != "" && entry.DeviceID != deviceIDFilter {
			continue
		}

		// Apply area_id filter
		if areaIDFilter != "" && entry.AreaID != areaIDFilter {
			continue
		}

		filtered = append(filtered, entry)
	}

	// Format output based on verbose flag
	var output []byte
	if verbose {
		output, err = json.MarshalIndent(filtered, "", "  ")
	} else {
		// Compact output: only entity_id, device_id (if set), area_id (if set)
		compact := make([]compactEntityEntry, 0, len(filtered))
		for _, entry := range filtered {
			compact = append(compact, compactEntityEntry{
				EntityID: entry.EntityID,
				DeviceID: entry.DeviceID,
				AreaID:   entry.AreaID,
			})
		}
		output, err = json.MarshalIndent(compact, "", "  ")
	}

	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	// Add summary info
	summary := fmt.Sprintf("Found %d entities", len(filtered))
	if !verbose {
		summary += " (use verbose=true for full details)"
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(summary + "\n\n" + string(output)),
		},
	}, nil
}

// extractDomain extracts the domain from an entity_id (e.g., "light" from "light.living_room").
func extractDomain(entityID string) string {
	for i, c := range entityID {
		if c == '.' {
			return entityID[:i]
		}
	}
	return ""
}

// listDeviceRegistryTool returns the tool definition for listing device registry entries.
func (h *RegistryHandlers) listDeviceRegistryTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_device_registry",
		Description: "List entries in the Home Assistant device registry. By default returns a compact list with id, name, manufacturer, model, and area_id. Use filters to narrow down results and 'verbose' for full details.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Filter and output options for device registry",
			Properties: map[string]mcp.JSONSchema{
				"area_id": {
					Type:        "string",
					Description: "Filter by area ID to get all devices in a specific area",
				},
				"manufacturer": {
					Type:        "string",
					Description: "Filter by manufacturer name (case-insensitive, partial match)",
				},
				"model": {
					Type:        "string",
					Description: "Filter by model name (case-insensitive, partial match)",
				},
				"verbose": {
					Type:        "boolean",
					Description: "If true, return full details (connections, identifiers, sw_version, hw_version, config_entries, etc.). Default: false (compact output)",
				},
				"include_disabled": {
					Type:        "boolean",
					Description: "If true, include disabled devices. Default: false",
				},
			},
		},
	}
}

// compactDeviceEntry represents a minimal device registry entry for compact output.
type compactDeviceEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	AreaID       string `json:"area_id,omitempty"`
}

// handleListDeviceRegistry handles requests to list device registry entries.
func (h *RegistryHandlers) handleListDeviceRegistry(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
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

	// Parse filter parameters
	areaIDFilter, _ := args["area_id"].(string)
	manufacturerFilter, _ := args["manufacturer"].(string)
	modelFilter, _ := args["model"].(string)
	verbose, _ := args["verbose"].(bool)
	includeDisabled, _ := args["include_disabled"].(bool)

	// Normalize filters for case-insensitive matching
	manufacturerFilterLower := strings.ToLower(manufacturerFilter)
	modelFilterLower := strings.ToLower(modelFilter)

	// Filter entries
	filtered := make([]homeassistant.DeviceRegistryEntry, 0, len(entries))
	for _, entry := range entries {
		// Skip disabled entries unless explicitly requested
		if !includeDisabled && entry.DisabledBy != "" {
			continue
		}

		// Apply area_id filter
		if areaIDFilter != "" && entry.AreaID != areaIDFilter {
			continue
		}

		// Apply manufacturer filter (case-insensitive, partial match)
		if manufacturerFilter != "" && !strings.Contains(strings.ToLower(entry.Manufacturer), manufacturerFilterLower) {
			continue
		}

		// Apply model filter (case-insensitive, partial match)
		if modelFilter != "" && !strings.Contains(strings.ToLower(string(entry.Model)), modelFilterLower) {
			continue
		}

		filtered = append(filtered, entry)
	}

	// Format output based on verbose flag
	var output []byte
	if verbose {
		output, err = json.MarshalIndent(filtered, "", "  ")
	} else {
		// Compact output: only essential fields
		compact := make([]compactDeviceEntry, 0, len(filtered))
		for _, entry := range filtered {
			compact = append(compact, compactDeviceEntry{
				ID:           entry.ID,
				Name:         entry.Name,
				Manufacturer: entry.Manufacturer,
				Model:        string(entry.Model),
				AreaID:       entry.AreaID,
			})
		}
		output, err = json.MarshalIndent(compact, "", "  ")
	}

	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	// Add summary info
	summary := fmt.Sprintf("Found %d devices", len(filtered))
	if !verbose {
		summary += " (use verbose=true for full details)"
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(summary + "\n\n" + string(output)),
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
