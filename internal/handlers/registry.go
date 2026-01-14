// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
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

// entityRegistryFilter encapsulates all filter criteria for entity registry queries.
type entityRegistryFilter struct {
	domain          string
	platform        string
	deviceID        string
	areaID          string
	includeDisabled bool
	deviceIDsInArea map[string]bool
}

// newEntityRegistryFilterFromArgs creates a filter from tool arguments.
func newEntityRegistryFilterFromArgs(args map[string]any) *entityRegistryFilter {
	domain, _ := args["domain"].(string)
	platform, _ := args["platform"].(string)
	deviceID, _ := args["device_id"].(string)
	areaID, _ := args["area_id"].(string)
	includeDisabled, _ := args["include_disabled"].(bool)

	return &entityRegistryFilter{
		domain:          domain,
		platform:        platform,
		deviceID:        deviceID,
		areaID:          areaID,
		includeDisabled: includeDisabled,
		deviceIDsInArea: make(map[string]bool),
	}
}

// matches returns true if the entry passes all filter criteria.
func (f *entityRegistryFilter) matches(entry homeassistant.EntityRegistryEntry) bool {
	if !f.includeDisabled && entry.DisabledBy != "" {
		return false
	}

	if f.domain != "" && extractDomain(entry.EntityID) != f.domain {
		return false
	}

	if f.platform != "" && entry.Platform != f.platform {
		return false
	}

	if f.deviceID != "" && entry.DeviceID != f.deviceID {
		return false
	}

	if f.areaID != "" {
		directMatch := entry.AreaID == f.areaID
		deviceMatch := entry.DeviceID != "" && f.deviceIDsInArea[entry.DeviceID]
		if !directMatch && !deviceMatch {
			return false
		}
	}

	return true
}

// buildDeviceIDsInArea populates the deviceIDsInArea map with devices in the target area.
func (f *entityRegistryFilter) buildDeviceIDsInArea(ctx context.Context, client homeassistant.Client) {
	if f.areaID == "" {
		return
	}

	devices, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		return
	}

	for _, device := range devices {
		if device.AreaID == f.areaID {
			f.deviceIDsInArea[device.ID] = true
		}
	}
}

// filterEntityRegistry applies the filter to a list of entries.
func (f *entityRegistryFilter) filterEntityRegistry(entries []homeassistant.EntityRegistryEntry) []homeassistant.EntityRegistryEntry {
	filtered := make([]homeassistant.EntityRegistryEntry, 0, len(entries))
	for _, entry := range entries {
		if f.matches(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// formatEntityRegistryOutput formats entries as JSON with optional verbosity.
func formatEntityRegistryOutput(entries []homeassistant.EntityRegistryEntry, verbose bool) (string, error) {
	var output []byte
	var err error

	if verbose {
		output, err = json.MarshalIndent(entries, "", "  ")
	} else {
		compact := make([]compactEntityEntry, 0, len(entries))
		for _, entry := range entries {
			compact = append(compact, compactEntityEntry{
				EntityID: entry.EntityID,
				DeviceID: entry.DeviceID,
				AreaID:   entry.AreaID,
			})
		}
		output, err = json.MarshalIndent(compact, "", "  ")
	}

	if err != nil {
		return "", fmt.Errorf("formatting response: %w", err)
	}

	return string(output), nil
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

	filter := newEntityRegistryFilterFromArgs(args)
	filter.buildDeviceIDsInArea(ctx, client)
	filtered := filter.filterEntityRegistry(entries)

	verbose, _ := args["verbose"].(bool)
	output, err := formatEntityRegistryOutput(filtered, verbose)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error %v", err)),
			},
			IsError: true,
		}, nil
	}

	summary := fmt.Sprintf("Found %d entities", len(filtered))
	if !verbose {
		summary += VerboseHint
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(summary + "\n\n" + output),
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

// deviceRegistryFilter encapsulates filter criteria for device registry queries.
type deviceRegistryFilter struct {
	areaID          string
	manufacturer    string
	model           string
	includeDisabled bool
}

// parseDeviceRegistryFilter creates a filter from tool arguments.
func parseDeviceRegistryFilter(args map[string]any) deviceRegistryFilter {
	areaID, _ := args["area_id"].(string)
	manufacturer, _ := args["manufacturer"].(string)
	model, _ := args["model"].(string)
	includeDisabled, _ := args["include_disabled"].(bool)
	return deviceRegistryFilter{
		areaID:          areaID,
		manufacturer:    manufacturer,
		model:           model,
		includeDisabled: includeDisabled,
	}
}

// matches returns true if the entry passes all filter criteria.
func (f deviceRegistryFilter) matches(entry homeassistant.DeviceRegistryEntry) bool {
	if !f.includeDisabled && entry.DisabledBy != "" {
		return false
	}
	if f.areaID != "" && entry.AreaID != f.areaID {
		return false
	}
	if f.manufacturer != "" && !strings.Contains(strings.ToLower(entry.Manufacturer), strings.ToLower(f.manufacturer)) {
		return false
	}
	if f.model != "" && !strings.Contains(strings.ToLower(string(entry.Model)), strings.ToLower(f.model)) {
		return false
	}
	return true
}

// filterDeviceRegistry applies the filter to entries.
func filterDeviceRegistry(entries []homeassistant.DeviceRegistryEntry, f deviceRegistryFilter) []homeassistant.DeviceRegistryEntry {
	filtered := make([]homeassistant.DeviceRegistryEntry, 0, len(entries))
	for _, entry := range entries {
		if f.matches(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// formatDeviceRegistryOutput formats entries as JSON with optional verbosity.
func formatDeviceRegistryOutput(entries []homeassistant.DeviceRegistryEntry, verbose bool) ([]byte, error) {
	if verbose {
		return json.MarshalIndent(entries, "", "  ")
	}
	compact := make([]compactDeviceEntry, 0, len(entries))
	for _, entry := range entries {
		compact = append(compact, compactDeviceEntry{
			ID:           entry.ID,
			Name:         entry.Name,
			Manufacturer: entry.Manufacturer,
			Model:        string(entry.Model),
			AreaID:       entry.AreaID,
		})
	}
	return json.MarshalIndent(compact, "", "  ")
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
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting device registry: %v", err))},
			IsError: true,
		}, nil
	}

	filter := parseDeviceRegistryFilter(args)
	filtered := filterDeviceRegistry(entries, filter)

	verbose, _ := args["verbose"].(bool)
	output, err := formatDeviceRegistryOutput(filtered, verbose)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err))},
			IsError: true,
		}, nil
	}

	summary := fmt.Sprintf("Found %d devices", len(filtered))
	if !verbose {
		summary += VerboseHint
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(summary + "\n\n" + string(output))},
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
