// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Floor action constants.
const (
	floorActionList   = "list"
	floorActionGet    = "get"
	floorActionCreate = "create"
	floorActionUpdate = "update"
	floorActionDelete = "delete"
)

// FloorHandlers provides handlers for floor-related MCP tools.
type FloorHandlers struct{}

// NewFloorHandlers creates a new FloorHandlers instance.
func NewFloorHandlers() *FloorHandlers {
	return &FloorHandlers{}
}

// RegisterTools registers the consolidated manage_floor tool with the registry.
func (h *FloorHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageFloorTool(), h.handleManageFloor)
}

// =============================================================================
// Tool Definition
// =============================================================================

func (h *FloorHandlers) manageFloorTool() mcp.Tool {
	schema := h.buildFloorSchema()
	return mcp.Tool{
		Name: "manage_floor",
		Description: `Manage Home Assistant floors - list, get, create, update, or delete.

Floors are building levels that contain areas. They help organize your home vertically.

Actions:
- list: List all floors with area counts (optional filters: name_contains)
- get: Get details of a specific floor with area list (requires floor_id)
- create: Create a new floor (requires name)
- update: Update an existing floor (requires floor_id); aliases use alias_mode ('add' default)
- delete: Delete a floor (requires floor_id)`,
		InputSchema: schema,
	}
}

func (h *FloorHandlers) buildFloorSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type:        "object",
		Description: "Floor management operation",
		Properties: map[string]mcp.JSONSchema{
			"action": {
				Type:        "string",
				Description: "Operation to perform: list, get, create, update, delete",
				Enum:        []string{"list", "get", "create", "update", "delete"},
			},
			"floor_id": {
				Type:        "string",
				Description: "Floor identifier or name. Required for get/update/delete. Accepts exact floor_id or case-insensitive name search.",
			},
			"name": {
				Type:        "string",
				Description: "Floor name (required for create, optional for update)",
			},
			"level": {
				Type:        "integer",
				Description: "Floor level (e.g., 0 for ground floor, 1 for first floor, -1 for basement)",
			},
			"icon": {
				Type:        "string",
				Description: "Floor icon (e.g., 'mdi:home-floor-0')",
			},
			"aliases": {
				Type:        "array",
				Description: "Alternative names for the floor; use alias_mode to control merge behavior",
				Items: &mcp.JSONSchema{
					Type: "string",
				},
			},
			"alias_mode": arrayModeSchema("aliases"),
			"name_contains": {
				Type:        "string",
				Description: "Filter by floor name containing this string (for list action, case-insensitive)",
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
// Handler: manage_floor
// =============================================================================

func (h *FloorHandlers) handleManageFloor(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case floorActionList:
		return h.handleList(ctx, client, args)
	case floorActionGet:
		return h.handleGet(ctx, client, args)
	case floorActionCreate:
		return h.handleCreate(ctx, client, args)
	case floorActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case floorActionDelete:
		return h.handleDelete(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, or delete)", action)), nil
	}
}

func (h *FloorHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	floors, err := client.GetFloorRegistry(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error listing floors: %v", err)), nil
	}

	// Apply name filter if provided
	nameContains, _ := args["name_contains"].(string)
	if nameContains != "" {
		floors = h.filterFloorsByName(floors, nameContains)
	}

	// Get area registry to count areas per floor
	areas, err := client.GetAreaRegistry(ctx)
	if err != nil {
		// Non-fatal: proceed without area counts
		areas = []homeassistant.AreaRegistryEntry{}
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatListJSON(floors, areas)
	}
	return h.formatListNatural(floors, areas)
}

func (h *FloorHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	floorID, _ := args["floor_id"].(string)
	if floorID == "" {
		return errorResult("floor_id is required for get action"), nil
	}

	floors, err := client.GetFloorRegistry(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error getting floors: %v", err)), nil
	}

	floor, findErr := h.findFloorByInput(floors, floorID)
	if findErr != nil {
		return errorResult(findErr.Error()), nil
	}

	// Get areas for this floor
	areas, err := client.GetAreaRegistry(ctx)
	if err != nil {
		areas = []homeassistant.AreaRegistryEntry{}
	}

	floorAreas := h.getAreasForFloor(floor.FloorID, areas)

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatGetJSON(floor, floorAreas)
	}
	return h.formatGetNatural(floor, floorAreas)
}

func (h *FloorHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return errorResult("name is required for create action"), nil
	}

	config := h.buildFloorConfig(args)
	config.Name = name
	if aliases, ok := args["aliases"].([]any); ok {
		config.Aliases = convertToStringSlice(aliases)
	}

	floor, err := client.CreateFloor(ctx, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error creating floor: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Floor '%s' created successfully with ID: %s", floor.Name, floor.FloorID)), nil
}

func (h *FloorHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	floorID, _ := args["floor_id"].(string)
	if floorID == "" {
		return errorResult("floor_id is required for update action"), nil
	}

	currentFloor, err := h.resolveFloorEntry(ctx, client, floorID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	config := h.buildFloorConfig(args)

	// Apply alias mode, merging with current values as needed.
	aliasMode, err := getArrayMode(args, "alias_mode")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	if aliases, hasAliases := getStringSlice(args, "aliases"); hasAliases {
		config.Aliases = applyArrayMode(currentFloor.Aliases, aliases, aliasMode)
	}

	floor, err := client.UpdateFloor(ctx, currentFloor.FloorID, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error updating floor: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Floor '%s' updated successfully", floor.Name)), nil
}

func (h *FloorHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	floorID, _ := args["floor_id"].(string)
	if floorID == "" {
		return errorResult("floor_id is required for delete action"), nil
	}

	resolvedID, err := h.resolveFloorID(ctx, client, floorID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	if err := client.DeleteFloor(ctx, resolvedID); err != nil {
		return errorResult(fmt.Sprintf("error deleting floor: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Floor '%s' deleted successfully", resolvedID)), nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// findFloorByInput performs two-phase lookup: exact ID match, then case-insensitive name substring match.
func (h *FloorHandlers) findFloorByInput(floors []homeassistant.FloorRegistryEntry, input string) (*homeassistant.FloorRegistryEntry, error) {
	// Phase 1: Exact ID match
	for i := range floors {
		if floors[i].FloorID == input {
			return &floors[i], nil
		}
	}

	// Phase 2: Case-insensitive name substring match
	lowerInput := strings.ToLower(input)
	for i := range floors {
		if strings.Contains(strings.ToLower(floors[i].Name), lowerInput) {
			return &floors[i], nil
		}
	}

	return nil, fmt.Errorf("floor not found: %s (tried as floor_id and name)", input)
}

// resolveFloorID resolves a floor input (ID or name) to the actual floor ID.
func (h *FloorHandlers) resolveFloorID(ctx context.Context, client homeassistant.Client, input string) (string, error) {
	floors, err := client.GetFloorRegistry(ctx)
	if err != nil {
		return "", fmt.Errorf("error fetching floors: %w", err)
	}

	floor, err := h.findFloorByInput(floors, input)
	if err != nil {
		return "", err
	}

	return floor.FloorID, nil
}

// resolveFloorEntry resolves a floor input (ID or name) to the full FloorRegistryEntry.
func (h *FloorHandlers) resolveFloorEntry(ctx context.Context, client homeassistant.Client, input string) (*homeassistant.FloorRegistryEntry, error) {
	floors, err := client.GetFloorRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching floors: %w", err)
	}

	floor, err := h.findFloorByInput(floors, input)
	if err != nil {
		return nil, err
	}

	return floor, nil
}

func (h *FloorHandlers) buildFloorConfig(args map[string]any) homeassistant.FloorConfig {
	config := homeassistant.FloorConfig{}

	if name, ok := args["name"].(string); ok && name != "" {
		config.Name = name
	}
	if level, ok := args["level"].(float64); ok {
		levelInt := int(level)
		config.Level = &levelInt
	}
	if icon, ok := args["icon"].(string); ok && icon != "" {
		config.Icon = icon
	}
	if aliases, ok := args["aliases"].([]any); ok {
		config.Aliases = convertToStringSlice(aliases)
	}

	return config
}

func (h *FloorHandlers) filterFloorsByName(floors []homeassistant.FloorRegistryEntry, nameContains string) []homeassistant.FloorRegistryEntry {
	filtered := make([]homeassistant.FloorRegistryEntry, 0)
	lowerSearch := strings.ToLower(nameContains)

	for _, floor := range floors {
		if strings.Contains(strings.ToLower(floor.Name), lowerSearch) {
			filtered = append(filtered, floor)
		}
	}

	return filtered
}

func (h *FloorHandlers) getAreasForFloor(floorID string, areas []homeassistant.AreaRegistryEntry) []homeassistant.AreaRegistryEntry {
	floorAreas := make([]homeassistant.AreaRegistryEntry, 0)
	for _, area := range areas {
		if area.FloorID == floorID {
			floorAreas = append(floorAreas, area)
		}
	}
	return floorAreas
}

func (h *FloorHandlers) countAreasByFloor(areas []homeassistant.AreaRegistryEntry) map[string]int {
	counts := make(map[string]int)
	for _, area := range areas {
		if area.FloorID != "" {
			counts[area.FloorID]++
		}
	}
	return counts
}

// =============================================================================
// Formatting Methods
// =============================================================================

func (h *FloorHandlers) formatListJSON(floors []homeassistant.FloorRegistryEntry, areas []homeassistant.AreaRegistryEntry) (*mcp.ToolsCallResult, error) {
	areaCounts := h.countAreasByFloor(areas)

	type floorWithCount struct {
		homeassistant.FloorRegistryEntry
		AreaCount int `json:"area_count"`
	}

	floorsWithCounts := make([]floorWithCount, len(floors))
	for i, floor := range floors {
		floorsWithCounts[i] = floorWithCount{
			FloorRegistryEntry: floor,
			AreaCount:          areaCounts[floor.FloorID],
		}
	}

	return jsonResult(map[string]any{
		"floors": floorsWithCounts,
		"count":  len(floors),
	})
}

func (h *FloorHandlers) formatListNatural(floors []homeassistant.FloorRegistryEntry, areas []homeassistant.AreaRegistryEntry) (*mcp.ToolsCallResult, error) {
	if len(floors) == 0 {
		return successResult("No floors found."), nil
	}

	areaCounts := h.countAreasByFloor(areas)

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d floor(s):\n", len(floors)))

	for _, floor := range floors {
		line := fmt.Sprintf("• %s (ID: %s)", floor.Name, floor.FloorID)
		if floor.Level != nil {
			line += fmt.Sprintf(" - Level: %d", *floor.Level)
		}
		if floor.Icon != "" {
			line += fmt.Sprintf(" - Icon: %s", floor.Icon)
		}
		areaCount := areaCounts[floor.FloorID]
		if areaCount > 0 {
			line += fmt.Sprintf(" - Areas: %d", areaCount)
		}
		if len(floor.Aliases) > 0 {
			line += fmt.Sprintf("\n  Aliases: %s", strings.Join(floor.Aliases, ", "))
		}
		parts = append(parts, line)
	}

	return successResult(strings.Join(parts, "\n")), nil
}

func (h *FloorHandlers) formatGetJSON(floor *homeassistant.FloorRegistryEntry, areas []homeassistant.AreaRegistryEntry) (*mcp.ToolsCallResult, error) {
	return jsonResult(map[string]any{
		"floor":      floor,
		"areas":      areas,
		"area_count": len(areas),
	})
}

func (h *FloorHandlers) formatGetNatural(floor *homeassistant.FloorRegistryEntry, areas []homeassistant.AreaRegistryEntry) (*mcp.ToolsCallResult, error) {
	var parts []string
	parts = append(parts,
		fmt.Sprintf("Floor: %s", floor.Name),
		fmt.Sprintf("ID: %s", floor.FloorID))

	if floor.Level != nil {
		parts = append(parts, fmt.Sprintf("Level: %d", *floor.Level))
	}
	if floor.Icon != "" {
		parts = append(parts, fmt.Sprintf("Icon: %s", floor.Icon))
	}
	if len(floor.Aliases) > 0 {
		parts = append(parts, fmt.Sprintf("Aliases: %s", strings.Join(floor.Aliases, ", ")))
	}

	parts = append(parts, fmt.Sprintf("\nAreas on this floor: %d", len(areas)))
	if len(areas) > 0 {
		for _, area := range areas {
			parts = append(parts, fmt.Sprintf("  • %s", area.Name))
		}
	}

	return successResult(strings.Join(parts, "\n")), nil
}
