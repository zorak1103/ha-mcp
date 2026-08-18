// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Area action constants.
const (
	areaActionList   = "list"
	areaActionGet    = "get"
	areaActionCreate = "create"
	areaActionUpdate = "update"
	areaActionDelete = "delete"

	formatJSON = "json"
)

// areaDetailEnrichment holds optional enrichment data for area get requests.
type areaDetailEnrichment struct {
	entities            []compactEntityState
	assignedAutomations []areaAssignedAutomation
	automations         []areaAutomationMatch
}

// areaAssignedAutomation represents an automation directly assigned to an area
// via its entity registry area_id.
type areaAssignedAutomation struct {
	EntityID     string `json:"entity_id"`
	FriendlyName string `json:"friendly_name,omitempty"`
	State        string `json:"state"`
}

// areaAutomationMatch represents an automation that references entities in an area.
type areaAutomationMatch struct {
	EntityID        string   `json:"entity_id"`
	FriendlyName    string   `json:"friendly_name,omitempty"`
	State           string   `json:"state"`
	MatchedEntities []string `json:"matched_entities"`
}

// AreaHandlers provides handlers for area-related MCP tools.
type AreaHandlers struct{}

// NewAreaHandlers creates a new AreaHandlers instance.
func NewAreaHandlers() *AreaHandlers {
	return &AreaHandlers{}
}

// RegisterTools registers the consolidated manage_area tool with the registry.
func (h *AreaHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageAreaTool(), h.handleManageArea)
}

// =============================================================================
// Tool Definition
// =============================================================================

func (h *AreaHandlers) manageAreaTool() mcp.Tool {
	schema := h.buildAreaSchema()
	return mcp.Tool{
		Name: "manage_area",
		Description: `Manage Home Assistant areas - list, get, create, update, or delete.

Actions:
- list: List all areas (optional filters: name_contains)
- get: Get details of a specific area with device/entity counts (requires area_id); use include_entities/include_automations to enrich with live data
- create: Create a new area (requires name)
- update: Update an existing area (requires area_id); labels/aliases use label_mode/alias_mode ('add' default)
- delete: Delete an area (requires area_id)`,
		InputSchema: schema,
	}
}

func (h *AreaHandlers) buildAreaSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type:        "object",
		Description: "Area management operation",
		Properties: map[string]mcp.JSONSchema{
			"action": {
				Type:        "string",
				Description: "Operation to perform: list, get, create, update, delete",
				Enum:        []string{"list", "get", "create", "update", "delete"},
			},
			"area_id": {
				Type:        "string",
				Description: "Area identifier or name. Required for get/update/delete. Accepts exact area_id or case-insensitive name search.",
			},
			"name": {
				Type:        "string",
				Description: "Area name (required for create, optional for update)",
			},
			"icon":    {Type: "string", Description: "Area icon (e.g., 'mdi:sofa')"},
			"picture": {Type: "string", Description: "Area picture URL"},
			"floor_id": {
				Type:        "string",
				Description: "Floor identifier this area belongs to",
			},
			"aliases": {
				Type:        "array",
				Description: "Alternative names for the area",
				Items: &mcp.JSONSchema{
					Type: "string",
				},
			},
			"labels": {
				Type:        "array",
				Description: "Labels for categorizing the area; use label_mode to control merge behavior",
				Items: &mcp.JSONSchema{
					Type: "string",
				},
			},
			"label_mode": arrayModeSchema("labels"),
			"alias_mode": arrayModeSchema("aliases"),
			"name_contains": {
				Type:        "string",
				Description: "Filter by area name containing this string (for list action, case-insensitive)",
			},
			"include_entities": {
				Type:        "boolean",
				Description: "Include compact entity list (entity_id, state, friendly_name) for all entities in the area. Only for get action.",
			},
			"include_automations": {
				Type:        "boolean",
				Description: "Include automation sections for this area. Only for get action. Returns two sections: 'Assigned to Area' (automations whose entity registry area_id matches this area) and 'Referencing Area Entities' (automations that reference entities located in this area).",
			},
			"format": {
				Type:        "string",
				Enum:        []string{"natural", formatJSON},
				Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
			},
		},
		Required: []string{"action"},
	}
}

// =============================================================================
// Main Handler
// =============================================================================

func (h *AreaHandlers) handleManageArea(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case areaActionList:
		return h.handleList(ctx, client, args)
	case areaActionGet:
		return h.handleGet(ctx, client, args)
	case areaActionCreate:
		return h.handleCreate(ctx, client, args)
	case areaActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case areaActionDelete:
		return h.handleDelete(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, or delete)", action)), nil
	}
}

// =============================================================================
// Action Handlers
// =============================================================================

func (h *AreaHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	areas, err := client.GetAreaRegistry(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error listing areas: %v", err)), nil
	}

	// Apply name filter if provided
	nameContains, _ := args["name_contains"].(string)
	if nameContains != "" {
		filtered := make([]homeassistant.AreaRegistryEntry, 0)
		nameLower := strings.ToLower(nameContains)
		for _, area := range areas {
			if strings.Contains(strings.ToLower(area.Name), nameLower) ||
				strings.Contains(strings.ToLower(area.AreaID), nameLower) {
				filtered = append(filtered, area)
			}
		}
		areas = filtered
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatListJSON(areas)
	}
	return h.formatListNatural(areas)
}

func (h *AreaHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	areaID, ok := args["area_id"].(string)
	if !ok || areaID == "" {
		return errorResult("area_id is required for get action"), nil
	}

	// Get area from registry
	areas, err := client.GetAreaRegistry(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error getting areas: %v", err)), nil
	}

	found, findErr := h.findAreaByInput(areas, areaID)
	if findErr != nil {
		return errorResult(findErr.Error()), nil
	}

	includeEntities := getBoolArg(args, "include_entities")
	includeAutomations := getBoolArg(args, "include_automations")
	enrichment, deviceCount, entityCount := h.buildAreaEnrichment(ctx, client, found.AreaID, includeEntities, includeAutomations)

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatDetailJSON(*found, deviceCount, entityCount, enrichment)
	}
	return h.formatDetailNatural(*found, deviceCount, entityCount, enrichment)
}

func (h *AreaHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return errorResult("name is required for create action"), nil
	}

	config := h.buildAreaConfig(args)
	config.Name = name
	config.Aliases = toStringArray(args["aliases"])
	config.Labels = toStringArray(args["labels"])

	entry, err := client.CreateArea(ctx, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error creating area: %v", err)), nil
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatDetailJSON(*entry, 0, 0, nil)
	}
	return h.formatCreateNatural(*entry)
}

func (h *AreaHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	areaID, ok := args["area_id"].(string)
	if !ok || areaID == "" {
		return errorResult("area_id is required for update action"), nil
	}

	currentArea, err := h.resolveAreaEntry(ctx, client, areaID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	config := h.buildAreaConfig(args)

	// Apply label/alias modes, merging with current values as needed.
	labelMode := getArrayMode(args, "label_mode")
	if labels, hasLabels := getStringSlice(args, "labels"); hasLabels {
		config.Labels = applyArrayMode(currentArea.Labels, labels, labelMode)
	}

	aliasMode := getArrayMode(args, "alias_mode")
	if aliases, hasAliases := getStringSlice(args, "aliases"); hasAliases {
		config.Aliases = applyArrayMode(currentArea.Aliases, aliases, aliasMode)
	}

	entry, err := client.UpdateArea(ctx, currentArea.AreaID, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error updating area: %v", err)), nil
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatDetailJSON(*entry, 0, 0, nil)
	}
	return h.formatUpdateNatural(*entry)
}

func (h *AreaHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	areaID, ok := args["area_id"].(string)
	if !ok || areaID == "" {
		return errorResult("area_id is required for delete action"), nil
	}

	resolvedID, err := h.resolveAreaID(ctx, client, areaID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	err = client.DeleteArea(ctx, resolvedID)
	if err != nil {
		return errorResult(fmt.Sprintf("error deleting area: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Area '%s' deleted successfully", resolvedID)), nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// findAreaByInput performs two-phase lookup: exact ID match, then case-insensitive name substring match.
func (h *AreaHandlers) findAreaByInput(areas []homeassistant.AreaRegistryEntry, input string) (*homeassistant.AreaRegistryEntry, error) {
	// Phase 1: Exact ID match
	for i := range areas {
		if areas[i].AreaID == input {
			return &areas[i], nil
		}
	}

	// Phase 2: Case-insensitive name substring match
	lowerInput := strings.ToLower(input)
	for i := range areas {
		if strings.Contains(strings.ToLower(areas[i].Name), lowerInput) {
			return &areas[i], nil
		}
	}

	return nil, fmt.Errorf("area not found: %s (tried as area_id and name)", input)
}

// resolveAreaID resolves an area input (ID or name) to the actual area ID.
func (h *AreaHandlers) resolveAreaID(ctx context.Context, client homeassistant.Client, input string) (string, error) {
	areas, err := client.GetAreaRegistry(ctx)
	if err != nil {
		return "", fmt.Errorf("error fetching areas: %w", err)
	}

	area, err := h.findAreaByInput(areas, input)
	if err != nil {
		return "", err
	}

	return area.AreaID, nil
}

// resolveAreaEntry resolves an area input (ID or name) to the full AreaRegistryEntry.
func (h *AreaHandlers) resolveAreaEntry(ctx context.Context, client homeassistant.Client, input string) (*homeassistant.AreaRegistryEntry, error) {
	areas, err := client.GetAreaRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching areas: %w", err)
	}

	area, err := h.findAreaByInput(areas, input)
	if err != nil {
		return nil, err
	}

	return area, nil
}

func (h *AreaHandlers) buildAreaConfig(args map[string]any) homeassistant.AreaConfig {
	cfg := homeassistant.AreaConfig{}

	if name, ok := args["name"].(string); ok && name != "" {
		cfg.Name = name
	}
	if icon, ok := args["icon"].(string); ok && icon != "" {
		cfg.Icon = icon
	}
	if picture, ok := args["picture"].(string); ok && picture != "" {
		cfg.Picture = picture
	}
	if floorID, ok := args["floor_id"].(string); ok && floorID != "" {
		cfg.FloorID = floorID
	}

	return cfg
}

// toStringArray converts []any to []string, filtering out non-string values.
func toStringArray(value any) []string {
	arr, ok := value.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func (h *AreaHandlers) getAreaCounts(ctx context.Context, client homeassistant.Client, areaID string) (int, int) {
	deviceCount := 0
	if devices, err := client.GetDeviceRegistry(ctx); err == nil {
		for _, d := range devices {
			if d.AreaID == areaID {
				deviceCount++
			}
		}
	}

	entityCount := 0
	if entities, err := client.GetEntityRegistry(ctx); err == nil {
		for _, e := range entities {
			if e.AreaID == areaID {
				entityCount++
			}
		}
	}

	return deviceCount, entityCount
}

// =============================================================================
// Enrichment Functions
// =============================================================================

// buildAreaEnrichment returns optional enrichment and device/entity counts for an area.
// When neither flag is set, it delegates to getAreaCounts and returns nil enrichment.
func (h *AreaHandlers) buildAreaEnrichment(
	ctx context.Context, client homeassistant.Client, areaID string,
	includeEntities, includeAutomations bool,
) (*areaDetailEnrichment, int, int) {
	if !includeEntities && !includeAutomations {
		deviceCount, entityCount := h.getAreaCounts(ctx, client, areaID)
		return nil, deviceCount, entityCount
	}

	entityIDsInArea, err := buildEntityIDsInArea(ctx, client, areaID)
	if err != nil {
		deviceCount, entityCount := h.getAreaCounts(ctx, client, areaID)
		return nil, deviceCount, entityCount
	}

	deviceCount := h.countDevicesInArea(ctx, client, areaID)
	entityCount := len(entityIDsInArea)

	enrichment := &areaDetailEnrichment{}
	if includeEntities {
		enrichment.entities = collectAreaEntities(ctx, client, entityIDsInArea)
	}
	if includeAutomations {
		enrichment.assignedAutomations = findAssignedAutomations(ctx, client, areaID)
		enrichment.automations = findAreaAutomations(ctx, client, entityIDsInArea)
	}

	return enrichment, deviceCount, entityCount
}

// countDevicesInArea returns the number of devices directly assigned to the area.
func (h *AreaHandlers) countDevicesInArea(ctx context.Context, client homeassistant.Client, areaID string) int {
	devices, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		return 0
	}
	count := 0
	for _, d := range devices {
		if d.AreaID == areaID {
			count++
		}
	}
	return count
}

// collectAreaEntities returns compact entity states for all entities in the area, sorted by entity_id.
// collectAreaEntities returns compact entity states for all entities in the area, sorted by entity_id.
func collectAreaEntities(ctx context.Context, client homeassistant.Client, entityIDsInArea map[string]bool) []compactEntityState {
	states, err := client.GetStates(ctx)
	if err != nil {
		return nil
	}
	var result []compactEntityState
	for _, state := range states {
		if !entityIDsInArea[state.EntityID] {
			continue
		}
		entry := compactEntityState{
			EntityID: state.EntityID,
			State:    state.State,
		}
		if fn, ok := state.Attributes["friendly_name"].(string); ok {
			entry.FriendlyName = fn
		}
		result = append(result, entry)
	}
	slices.SortFunc(result, func(a, b compactEntityState) int {
		return cmp.Compare(a.EntityID, b.EntityID)
	})
	return result
}

// findAssignedAutomations returns automations directly assigned to the area via
// entity registry area_id (automation.* entries whose AreaID == areaID).
// findAssignedAutomations returns automations directly assigned to the area via
// entity registry area_id (automation.* entries whose AreaID == areaID).
func findAssignedAutomations(ctx context.Context, client homeassistant.Client, areaID string) []areaAssignedAutomation {
	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return nil
	}
	// Collect the entity_ids of automation.* entries assigned to this area.
	assignedIDs := make(map[string]bool)
	for _, e := range entries {
		if strings.HasPrefix(e.EntityID, "automation.") && e.AreaID == areaID {
			assignedIDs[e.EntityID] = true
		}
	}
	if len(assignedIDs) == 0 {
		return nil
	}
	// Fetch live states to get FriendlyName + State.
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return nil
	}
	var result []areaAssignedAutomation
	for _, a := range automations {
		if !assignedIDs[a.EntityID] {
			continue
		}
		result = append(result, areaAssignedAutomation{
			EntityID:     a.EntityID,
			FriendlyName: a.FriendlyName,
			State:        a.State,
		})
	}
	slices.SortFunc(result, func(a, b areaAssignedAutomation) int {
		return cmp.Compare(a.EntityID, b.EntityID)
	})
	return result
}

// findAreaAutomations returns automations that reference any entity in the area.
func findAreaAutomations(ctx context.Context, client homeassistant.Client, entityIDsInArea map[string]bool) []areaAutomationMatch {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return nil
	}
	var matches []areaAutomationMatch
	for _, auto := range automations {
		full, getErr := client.GetAutomation(ctx, auto.EntityID)
		if getErr != nil || full == nil || full.Config == nil {
			continue
		}
		entities := extractEntitiesFromAutomation(full.Config)
		overlapping := findOverlappingEntities(entities, entityIDsInArea)
		if len(overlapping) == 0 {
			continue
		}
		sort.Strings(overlapping)
		matches = append(matches, areaAutomationMatch{
			EntityID:        auto.EntityID,
			FriendlyName:    auto.FriendlyName,
			State:           auto.State,
			MatchedEntities: overlapping,
		})
	}
	return matches
}

// findOverlappingEntities returns entity IDs from entityIDs that exist in entitySet.
func findOverlappingEntities(entityIDs []string, entitySet map[string]bool) []string {
	var overlap []string
	for _, id := range entityIDs {
		if entitySet[id] {
			overlap = append(overlap, id)
		}
	}
	return overlap
}

// =============================================================================
// Formatting Functions (private, domain-specific)
// =============================================================================

func (h *AreaHandlers) formatListJSON(areas []homeassistant.AreaRegistryEntry) (*mcp.ToolsCallResult, error) {
	return jsonResult(map[string]any{
		"areas": areas,
		"count": len(areas),
	})
}

func (h *AreaHandlers) formatListNatural(areas []homeassistant.AreaRegistryEntry) (*mcp.ToolsCallResult, error) {
	if len(areas) == 0 {
		return successResult("No areas found"), nil
	}

	var output strings.Builder
	fmt.Fprintf(&output, "Found %d area(s):\n\n", len(areas))

	for _, area := range areas {
		fmt.Fprintf(&output, "• %s (ID: %s)\n", area.Name, area.AreaID)
		if area.FloorID != "" {
			fmt.Fprintf(&output, "  Floor: %s\n", area.FloorID)
		}
		if area.Icon != "" {
			fmt.Fprintf(&output, "  Icon: %s\n", area.Icon)
		}
		if len(area.Aliases) > 0 {
			fmt.Fprintf(&output, "  Aliases: %s\n", strings.Join(area.Aliases, ", "))
		}
		if len(area.Labels) > 0 {
			fmt.Fprintf(&output, "  Labels: %s\n", strings.Join(area.Labels, ", "))
		}
	}

	return successResult(output.String()), nil
}

func (h *AreaHandlers) formatDetailJSON(area homeassistant.AreaRegistryEntry, deviceCount, entityCount int, enrichment *areaDetailEnrichment) (*mcp.ToolsCallResult, error) {
	result := map[string]any{
		"area_id": area.AreaID,
		"name":    area.Name,
	}

	if area.Icon != "" {
		result["icon"] = area.Icon
	}
	if area.Picture != "" {
		result["picture"] = area.Picture
	}
	if area.FloorID != "" {
		result["floor_id"] = area.FloorID
	}
	if len(area.Aliases) > 0 {
		result["aliases"] = area.Aliases
	}
	if len(area.Labels) > 0 {
		result["labels"] = area.Labels
	}
	if deviceCount > 0 || entityCount > 0 {
		result["device_count"] = deviceCount
		result["entity_count"] = entityCount
	}
	if enrichment != nil {
		if enrichment.entities != nil {
			result["entities"] = enrichment.entities
		}
		if enrichment.assignedAutomations != nil {
			result["assigned_automations"] = enrichment.assignedAutomations
		}
		if enrichment.automations != nil {
			result["automations"] = enrichment.automations
		}
	}

	return jsonResult(result)
}

func (h *AreaHandlers) formatDetailNatural(area homeassistant.AreaRegistryEntry, deviceCount, entityCount int, enrichment *areaDetailEnrichment) (*mcp.ToolsCallResult, error) {
	var output strings.Builder

	fmt.Fprintf(&output, "Area: %s\n", area.Name)
	fmt.Fprintf(&output, "ID: %s\n", area.AreaID)

	if area.FloorID != "" {
		fmt.Fprintf(&output, "Floor: %s\n", area.FloorID)
	}
	if area.Icon != "" {
		fmt.Fprintf(&output, "Icon: %s\n", area.Icon)
	}
	if area.Picture != "" {
		fmt.Fprintf(&output, "Picture: %s\n", area.Picture)
	}
	if len(area.Aliases) > 0 {
		fmt.Fprintf(&output, "Aliases: %s\n", strings.Join(area.Aliases, ", "))
	}
	if len(area.Labels) > 0 {
		fmt.Fprintf(&output, "Labels: %s\n", strings.Join(area.Labels, ", "))
	}
	if deviceCount > 0 || entityCount > 0 {
		fmt.Fprintf(&output, "\nDevices: %d\n", deviceCount)
		fmt.Fprintf(&output, "Entities: %d\n", entityCount)
	}
	h.writeEnrichmentNatural(&output, enrichment)

	return successResult(output.String()), nil
}

// writeEnrichmentNatural appends enrichment sections to the output builder.
func (h *AreaHandlers) writeEnrichmentNatural(output *strings.Builder, enrichment *areaDetailEnrichment) {
	if enrichment == nil {
		return
	}
	if len(enrichment.entities) > 0 {
		fmt.Fprintf(output, "\nEntities in Area (%d):\n", len(enrichment.entities))
		for _, e := range enrichment.entities {
			fmt.Fprintf(output, "  - %s (%s) [%s]\n", e.EntityID, e.FriendlyName, e.State)
		}
	}
	if len(enrichment.assignedAutomations) > 0 {
		fmt.Fprintf(output, "\nAutomations Assigned to Area (%d):\n", len(enrichment.assignedAutomations))
		for _, a := range enrichment.assignedAutomations {
			fmt.Fprintf(output, "  - %s [%s]\n", a.EntityID, a.State)
		}
	}
	if len(enrichment.automations) > 0 {
		fmt.Fprintf(output, "\nAutomations Referencing Area Entities (%d):\n", len(enrichment.automations))
		for _, a := range enrichment.automations {
			fmt.Fprintf(output, "  - %s [%s] (matches: %s)\n", a.EntityID, a.State, strings.Join(a.MatchedEntities, ", "))
		}
	}
}

func (h *AreaHandlers) formatCreateNatural(area homeassistant.AreaRegistryEntry) (*mcp.ToolsCallResult, error) {
	return successResult(fmt.Sprintf("Area '%s' created successfully (ID: %s)", area.Name, area.AreaID)), nil
}

func (h *AreaHandlers) formatUpdateNatural(area homeassistant.AreaRegistryEntry) (*mcp.ToolsCallResult, error) {
	return successResult(fmt.Sprintf("Area '%s' updated successfully", area.Name)), nil
}
