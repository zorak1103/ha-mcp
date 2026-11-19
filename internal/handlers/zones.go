// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Zone action constants.
const (
	zoneActionList   = "list"
	zoneActionGet    = "get"
	zoneActionCreate = "create"
	zoneActionUpdate = "update"
	zoneActionDelete = "delete"
)

// ZoneHandlers provides handlers for zone-related MCP tools.
type ZoneHandlers struct{}

// NewZoneHandlers creates a new ZoneHandlers instance.
func NewZoneHandlers() *ZoneHandlers {
	return &ZoneHandlers{}
}

// RegisterTools registers the consolidated manage_zone tool with the registry.
func (h *ZoneHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageZoneTool(), h.handleManageZone)
}

// =============================================================================
// Tool Definition
// =============================================================================

func (h *ZoneHandlers) manageZoneTool() mcp.Tool {
	schema := h.buildZoneSchema()
	return mcp.Tool{
		Name: "manage_zone",
		Description: `Manage Home Assistant zones - list, get, create, update, or delete.

Zones are geographic areas used for presence detection and geofencing.

Actions:
- list: List all zones (optional filters: name_contains)
- get: Get details of a specific zone (requires zone_id)
- create: Create a new zone (requires name, latitude, longitude, radius)
- update: Update an existing zone (requires zone_id)
- delete: Delete a zone (requires zone_id)`,
		InputSchema: schema,
	}
}

func (h *ZoneHandlers) buildZoneSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type:        "object",
		Description: "Zone management operation",
		Properties: map[string]mcp.JSONSchema{
			"action": {
				Type:        "string",
				Description: "Operation to perform: list, get, create, update, delete",
				Enum:        []string{"list", "get", "create", "update", "delete"},
			},
			"zone_id": {
				Type:        "string",
				Description: "Zone identifier or name. Required for get/update/delete. Accepts exact zone_id or case-insensitive name search.",
			},
			"name": {
				Type:        "string",
				Description: "Zone name (required for create, optional for update)",
			},
			"latitude": {
				Type:        "number",
				Description: "Latitude coordinate (required for create)",
			},
			"longitude": {
				Type:        "number",
				Description: "Longitude coordinate (required for create)",
			},
			"radius": {
				Type:        "number",
				Description: "Radius in meters (required for create)",
			},
			"icon": {
				Type:        "string",
				Description: "Zone icon (e.g., 'mdi:home')",
			},
			"passive": {
				Type:        "boolean",
				Description: "Passive zone (does not trigger automations)",
			},
			"name_contains": {
				Type:        "string",
				Description: "Filter by zone name containing this string (for list action, case-insensitive)",
			},
			"format": {
				Type:        "string",
				Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				Enum:        []string{"natural", "json"},
			},
		},
		Required: []string{"action"},
	}
}

// =============================================================================
// Handler: manage_zone
// =============================================================================

func (h *ZoneHandlers) handleManageZone(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case zoneActionList:
		return h.handleList(ctx, client, args)
	case zoneActionGet:
		return h.handleGet(ctx, client, args)
	case zoneActionCreate:
		return h.handleCreate(ctx, client, args)
	case zoneActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case zoneActionDelete:
		return h.handleDelete(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, or delete)", action)), nil
	}
}

func (h *ZoneHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	nameContains, errMsg := stringArgValue("name_contains", args["name_contains"])
	if errMsg != "" {
		return errorResult(errMsg), nil
	}

	zones, err := client.GetZones(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error listing zones: %v", err)), nil
	}

	// Apply name filter if provided
	if nameContains != "" {
		zones = h.filterZonesByName(zones, nameContains)
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatListJSON(zones)
	}
	return h.formatListNatural(zones)
}

func (h *ZoneHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	zoneID, _ := args["zone_id"].(string)
	if zoneID == "" {
		return errorResult("zone_id is required for get action"), nil
	}

	zones, err := client.GetZones(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error getting zones: %v", err)), nil
	}

	zone, findErr := h.findZoneByInput(zones, zoneID)
	if findErr != nil {
		return errorResult(findErr.Error()), nil
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatGetJSON(zone)
	}
	return h.formatGetNatural(zone)
}

func (h *ZoneHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return errorResult("name is required for create action"), nil
	}

	latitude, hasLat := args["latitude"].(float64)
	if !hasLat {
		return errorResult("latitude is required for create action"), nil
	}

	longitude, hasLong := args["longitude"].(float64)
	if !hasLong {
		return errorResult("longitude is required for create action"), nil
	}

	radius, hasRadius := args["radius"].(float64)
	if !hasRadius {
		return errorResult("radius is required for create action"), nil
	}

	config := h.buildZoneConfig(args)
	config.Name = name
	config.Latitude = &latitude
	config.Longitude = &longitude
	config.Radius = &radius

	zone, err := client.CreateZone(ctx, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error creating zone: %v", err)), nil
	}

	successMsg := fmt.Sprintf("Zone '%s' created successfully with ID: %s", zone.Name, zone.ID)
	entityID := "zone." + zone.ID
	if _, appeared := waitForEntityAppear(ctx, client, entityID); !appeared {
		successMsg += " (warning: zone entity not yet visible)"
	}

	return successResult(successMsg), nil
}

func (h *ZoneHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	zoneID, _ := args["zone_id"].(string)
	if zoneID == "" {
		return errorResult("zone_id is required for update action"), nil
	}

	resolvedID, err := h.resolveZoneID(ctx, client, zoneID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	config := h.buildZoneConfig(args)

	zone, err := client.UpdateZone(ctx, resolvedID, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error updating zone: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Zone '%s' updated successfully", zone.Name)), nil
}

func (h *ZoneHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	zoneID, _ := args["zone_id"].(string)
	if zoneID == "" {
		return errorResult("zone_id is required for delete action"), nil
	}

	resolvedID, err := h.resolveZoneID(ctx, client, zoneID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	if err := client.DeleteZone(ctx, resolvedID); err != nil {
		return errorResult(fmt.Sprintf("error deleting zone: %v", err)), nil
	}

	entityID := "zone." + resolvedID
	successMsg := fmt.Sprintf("Zone '%s' deleted successfully", resolvedID)
	if !waitForEntityDisappear(ctx, client, entityID) {
		successMsg += " (warning: zone entity may still be visible briefly)"
	}

	return successResult(successMsg), nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// findZoneByInput performs two-phase lookup: exact ID match, then case-insensitive name substring match.
func (h *ZoneHandlers) findZoneByInput(zones []homeassistant.ZoneRegistryEntry, input string) (*homeassistant.ZoneRegistryEntry, error) {
	// Phase 1: Exact ID match
	for i := range zones {
		if zones[i].ID == input {
			return &zones[i], nil
		}
	}

	// Phase 2: Case-insensitive name substring match
	lowerInput := strings.ToLower(input)
	for i := range zones {
		if strings.Contains(strings.ToLower(zones[i].Name), lowerInput) {
			return &zones[i], nil
		}
	}

	return nil, fmt.Errorf("zone not found: %s (tried as zone_id and name)", input)
}

// resolveZoneID resolves a zone input (ID or name) to the actual zone ID.
func (h *ZoneHandlers) resolveZoneID(ctx context.Context, client homeassistant.Client, input string) (string, error) {
	zones, err := client.GetZones(ctx)
	if err != nil {
		return "", fmt.Errorf("error fetching zones: %w", err)
	}

	zone, err := h.findZoneByInput(zones, input)
	if err != nil {
		return "", err
	}

	return zone.ID, nil
}

func (h *ZoneHandlers) buildZoneConfig(args map[string]any) homeassistant.ZoneConfig {
	config := homeassistant.ZoneConfig{}

	if name, ok := args["name"].(string); ok && name != "" {
		config.Name = name
	}
	if latitude, ok := args["latitude"].(float64); ok {
		config.Latitude = &latitude
	}
	if longitude, ok := args["longitude"].(float64); ok {
		config.Longitude = &longitude
	}
	if radius, ok := args["radius"].(float64); ok {
		config.Radius = &radius
	}
	if icon, ok := args["icon"].(string); ok && icon != "" {
		config.Icon = icon
	}
	if passive, ok := args["passive"].(bool); ok {
		config.Passive = &passive
	}

	return config
}

func (h *ZoneHandlers) filterZonesByName(zones []homeassistant.ZoneRegistryEntry, nameContains string) []homeassistant.ZoneRegistryEntry {
	filtered := make([]homeassistant.ZoneRegistryEntry, 0)
	lowerSearch := strings.ToLower(nameContains)

	for _, zone := range zones {
		if strings.Contains(strings.ToLower(zone.Name), lowerSearch) {
			filtered = append(filtered, zone)
		}
	}

	return filtered
}

// =============================================================================
// Formatting Methods
// =============================================================================

func (h *ZoneHandlers) formatListJSON(zones []homeassistant.ZoneRegistryEntry) (*mcp.ToolsCallResult, error) {
	return jsonResult(map[string]any{
		"zones": zones,
		"count": len(zones),
	})
}

func (h *ZoneHandlers) formatListNatural(zones []homeassistant.ZoneRegistryEntry) (*mcp.ToolsCallResult, error) {
	if len(zones) == 0 {
		return successResult("No zones found."), nil
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d zone(s):\n", len(zones)))

	for _, zone := range zones {
		line := fmt.Sprintf("• %s (ID: %s)", zone.Name, zone.ID)
		line += fmt.Sprintf("\n  Location: %.6f, %.6f", zone.Latitude, zone.Longitude)
		line += fmt.Sprintf("\n  Radius: %.0fm", zone.Radius)
		if zone.Icon != "" {
			line += fmt.Sprintf(" - Icon: %s", zone.Icon)
		}
		if zone.Passive {
			line += " - Passive"
		}
		parts = append(parts, line)
	}

	return successResult(strings.Join(parts, "\n")), nil
}

func (h *ZoneHandlers) formatGetJSON(zone *homeassistant.ZoneRegistryEntry) (*mcp.ToolsCallResult, error) {
	return jsonResult(zone)
}

func (h *ZoneHandlers) formatGetNatural(zone *homeassistant.ZoneRegistryEntry) (*mcp.ToolsCallResult, error) {
	var parts []string
	parts = append(parts,
		fmt.Sprintf("Zone: %s", zone.Name),
		fmt.Sprintf("ID: %s", zone.ID),
		fmt.Sprintf("Latitude: %.6f", zone.Latitude),
		fmt.Sprintf("Longitude: %.6f", zone.Longitude),
		fmt.Sprintf("Radius: %.0f meters", zone.Radius))

	if zone.Icon != "" {
		parts = append(parts, fmt.Sprintf("Icon: %s", zone.Icon))
	}
	if zone.Passive {
		parts = append(parts, "Passive: true")
	}

	return successResult(strings.Join(parts, "\n")), nil
}
