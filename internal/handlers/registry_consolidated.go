// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Registry type constants.
const (
	registryTypeEntities = "entities"
	registryTypeDevices  = "devices"
	registryTypeAreas    = "areas"
	registryTypeAll      = "all"
)

// ConsolidatedRegistryHandlers provides consolidated handlers for Home Assistant registry operations.
// This replaces the individual list_entity_registry, list_device_registry, and list_area_registry tools.
type ConsolidatedRegistryHandlers struct{}

// NewConsolidatedRegistryHandlers creates a new ConsolidatedRegistryHandlers instance.
func NewConsolidatedRegistryHandlers() *ConsolidatedRegistryHandlers {
	return &ConsolidatedRegistryHandlers{}
}

// RegisterTools registers the consolidated get_registry tool.
func (h *ConsolidatedRegistryHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getRegistryTool(), h.handleGetRegistry)
}

// getRegistryTool returns the tool definition for the consolidated registry tool.
func (h *ConsolidatedRegistryHandlers) getRegistryTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_registry",
		Description: getRegistryDescription(),
		InputSchema: mcp.JSONSchema{
			Type:       "object",
			Properties: getRegistryProperties(),
			Required:   []string{"type"},
		},
	}
}

func getRegistryDescription() string {
	return `Query Home Assistant registries (entities, devices, areas).

Actions:
- type=entities: List entity registry entries with optional filters (domain, platform, device_id, area_id)
- type=devices: List device registry entries with optional filters (area_id, manufacturer, model)
- type=areas: List all area registry entries
- type=all: Get summary of all registries combined

Examples:
- Get all lights: {"type": "entities", "domain": "light"}
- Get devices by manufacturer: {"type": "devices", "manufacturer": "Philips"}
- Get all areas: {"type": "areas"}
- Get full registry overview: {"type": "all"}`
}

func getRegistryProperties() map[string]mcp.JSONSchema {
	return map[string]mcp.JSONSchema{
		"type": {
			Type:        "string",
			Enum:        []string{registryTypeEntities, registryTypeDevices, registryTypeAreas, registryTypeAll},
			Description: "Registry type to query: entities, devices, areas, or all",
		},
		"format": {
			Type:        "string",
			Enum:        []string{"natural", "json"},
			Description: "Output format: 'natural' for LLM-optimized human-readable output (default), 'json' for structured JSON",
		},
		"domain": {
			Type:        "string",
			Description: "Filter entities by domain (e.g., 'light', 'switch', 'sensor'). Only for type=entities",
		},
		"platform": {
			Type:        "string",
			Description: "Filter entities by platform/integration (e.g., 'hue', 'mqtt'). Only for type=entities",
		},
		"device_id": {
			Type:        "string",
			Description: "Filter entities by device ID. Only for type=entities",
		},
		"area_id": {
			Type:        "string",
			Description: "Filter by area ID. Works with type=entities and type=devices",
		},
		"manufacturer": {
			Type:        "string",
			Description: "Filter devices by manufacturer (case-insensitive, partial match). Only for type=devices",
		},
		"model": {
			Type:        "string",
			Description: "Filter devices by model (case-insensitive, partial match). Only for type=devices",
		},
		"verbose": {
			Type:        "boolean",
			Description: "If true, return full details. Default: false (compact output)",
		},
		"include_disabled": {
			Type:        "boolean",
			Description: "If true, include disabled entries. Default: false",
		},
		"include_entities": {
			Type:        "boolean",
			Description: "If true, include associated entities for each device. Only for type=devices. Default: false",
		},
		"limit": {
			Type:        "integer",
			Description: "Maximum number of entries to return (max 1000). Use with 'cursor' for pagination.",
		},
		"cursor": {
			Type:        "string",
			Description: "Pagination cursor from previous response to get next page",
		},
	}
}

// handleGetRegistry handles the consolidated get_registry tool.
func (h *ConsolidatedRegistryHandlers) handleGetRegistry(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	registryType, ok := args["type"].(string)
	if !ok || registryType == "" {
		return errorResult("type parameter is required"), nil
	}

	switch registryType {
	case registryTypeEntities:
		return h.handleEntities(ctx, client, args)
	case registryTypeDevices:
		return h.handleDevices(ctx, client, args)
	case registryTypeAreas:
		return h.handleAreas(ctx, client, args)
	case registryTypeAll:
		return h.handleAll(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("Invalid type %q. Must be one of: entities, devices, areas, all", registryType)), nil
	}
}

// handleEntities handles type=entities requests.
func (h *ConsolidatedRegistryHandlers) handleEntities(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting entity registry: %v", err)), nil
	}

	filter := newEntityRegistryFilterFromArgs(args)
	if err = filter.buildDeviceIDsInArea(ctx, client); err != nil {
		return errorResult(fmt.Sprintf("Error getting device registry: %v", err)), nil
	}
	filtered := filter.filterEntityRegistry(entries)

	slices.SortFunc(filtered, func(a, b homeassistant.EntityRegistryEntry) int {
		return cmp.Compare(a.EntityID, b.EntityID)
	})

	filtersMap := buildEntityRegistryFiltersMap(filter)
	paginationParams, err := ParsePaginationParams(args, filtersMap)
	if err != nil {
		return errorResult(fmt.Sprintf("error parsing pagination parameters: %v", err)), nil
	}

	paginated := ApplyPagination(filtered, paginationParams)
	return h.formatEntitiesResponse(ctx, paginated, args)
}

// formatEntitiesResponse renders the paginated entity registry using the requested format.
func (h *ConsolidatedRegistryHandlers) formatEntitiesResponse(
	ctx context.Context,
	paginated PaginatedResponse[homeassistant.EntityRegistryEntry],
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	verbose, _ := args["verbose"].(bool)
	includeDisabled, _ := args["include_disabled"].(bool)

	// Parse format parameter
	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	// Use formatter
	f := formatter.NewRegistryFormatter(format)
	opts := formatter.RegistryOptions{
		Verbose:         verbose,
		IncludeDisabled: includeDisabled,
		Limit:           paginated.Pagination.Limit,
	}
	output, err := f.FormatEntityRegistry(ctx, paginated.Items, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting response: %v", err)), nil
	}

	// Add pagination summary for natural format only
	var response string
	if format == formatter.FormatNatural {
		summary := BuildPaginationSummary(paginated.Pagination, "entities")
		if !verbose {
			summary += VerboseHint
		}
		response = summary + "\n\n" + output
		if paginated.Pagination.HasMore && paginated.Pagination.NextCursor != nil {
			response += fmt.Sprintf("\n\nNext cursor: %s", *paginated.Pagination.NextCursor)
		}
	} else {
		// For JSON format, wrap with pagination metadata
		response = buildPaginatedEntityRegistryResponse(paginated, output)
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(response),
		},
	}, nil
}

// buildDeviceEntityMap loads entity registry and groups entities by device_id.
func buildDeviceEntityMap(ctx context.Context, client homeassistant.Client) (map[string][]formatter.EntityInfo, error) {
	entities, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return nil, err
	}

	entityMap := make(map[string][]formatter.EntityInfo)
	for _, entity := range entities {
		if entity.DeviceID == "" {
			continue // Skip entities without a device
		}
		entityMap[entity.DeviceID] = append(entityMap[entity.DeviceID], formatter.EntityInfo{
			EntityID:     entity.EntityID,
			FriendlyName: entity.Name,
		})
	}

	return entityMap, nil
}

// handleDevices handles type=devices requests.
func (h *ConsolidatedRegistryHandlers) handleDevices(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	entries, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting device registry: %v", err)), nil
	}

	filter := parseDeviceRegistryFilter(args)
	filtered := filterDeviceRegistry(entries, filter)

	slices.SortFunc(filtered, func(a, b homeassistant.DeviceRegistryEntry) int {
		return cmp.Compare(a.ID, b.ID)
	})

	filtersMap := buildDeviceRegistryFiltersMap(filter)
	paginationParams, err := ParsePaginationParams(args, filtersMap)
	if err != nil {
		return errorResult(fmt.Sprintf("error parsing pagination parameters: %v", err)), nil
	}

	paginated := ApplyPagination(filtered, paginationParams)
	verbose, _ := args["verbose"].(bool)
	includeDisabled, _ := args["include_disabled"].(bool)
	includeEntities, _ := args["include_entities"].(bool)

	// Parse format parameter
	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	// Build entity map if requested
	var entityMap map[string][]formatter.EntityInfo
	if includeEntities {
		entityMap, err = buildDeviceEntityMap(ctx, client)
		if err != nil {
			return errorResult(fmt.Sprintf("Error loading entities: %v", err)), nil
		}
	}

	// Use formatter
	f := formatter.NewRegistryFormatter(format)
	opts := formatter.RegistryOptions{
		Verbose:         verbose,
		IncludeDisabled: includeDisabled,
		Limit:           paginationParams.Limit,
		EntityMap:       entityMap,
	}
	output, err := f.FormatDeviceRegistry(ctx, paginated.Items, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting response: %v", err)), nil
	}

	response := buildDeviceRegistryResponse(paginated, output, format, verbose)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(response),
		},
	}, nil
}

// buildDeviceRegistryResponse builds the final response string with pagination.
func buildDeviceRegistryResponse(paginated PaginatedResponse[homeassistant.DeviceRegistryEntry], output string, format formatter.Format, verbose bool) string {
	if format == formatter.FormatNatural {
		summary := BuildPaginationSummary(paginated.Pagination, "devices")
		if !verbose {
			summary += VerboseHint
		}
		response := summary + "\n\n" + output
		if paginated.Pagination.HasMore && paginated.Pagination.NextCursor != nil {
			response += fmt.Sprintf("\n\nNext cursor: %s", *paginated.Pagination.NextCursor)
		}
		return response
	}
	// JSON format: wrap with pagination metadata
	return string(buildPaginatedDeviceRegistryResponse(paginated, []byte(output)))
}

// handleAreas handles type=areas requests.
func (h *ConsolidatedRegistryHandlers) handleAreas(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	entries, err := client.GetAreaRegistry(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting area registry: %v", err)), nil
	}

	// Parse format parameter
	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	// Use formatter
	f := formatter.NewRegistryFormatter(format)
	output, err := f.FormatAreaRegistry(ctx, entries)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting response: %v", err)), nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(output),
		},
	}, nil
}

// handleAll handles type=all requests - returns combined summary of all registries.
func (h *ConsolidatedRegistryHandlers) handleAll(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	includeDisabled, _ := args["include_disabled"].(bool)

	// Parse format parameter
	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	// Fetch all registries
	entities, entitiesErr := client.GetEntityRegistry(ctx)
	devices, devicesErr := client.GetDeviceRegistry(ctx)
	areas, areasErr := client.GetAreaRegistry(ctx)

	// For natural format, we can still show partial results with errors
	if format == formatter.FormatNatural {
		return h.handleAllNatural(entities, entitiesErr, devices, devicesErr, areas, areasErr, includeDisabled)
	}

	// For JSON format, errors are fatal
	return h.handleAllJSON(ctx, entities, entitiesErr, devices, devicesErr, areas, areasErr, includeDisabled)
}

func (h *ConsolidatedRegistryHandlers) handleAllNatural(
	entities []homeassistant.EntityRegistryEntry, entitiesErr error,
	devices []homeassistant.DeviceRegistryEntry, devicesErr error,
	areas []homeassistant.AreaRegistryEntry, areasErr error,
	includeDisabled bool,
) (*mcp.ToolsCallResult, error) {
	var result strings.Builder

	if entitiesErr != nil {
		fmt.Fprintf(&result, "Entities: Error - %v\n\n", entitiesErr)
	} else {
		h.formatEntitiesSummaryWithData(entities, includeDisabled, &result)
	}

	if devicesErr != nil {
		fmt.Fprintf(&result, "Devices: Error - %v\n\n", devicesErr)
	} else {
		h.formatDevicesSummaryWithData(devices, includeDisabled, &result)
	}

	if areasErr != nil {
		fmt.Fprintf(&result, "Areas: Error - %v\n\n", areasErr)
	} else {
		h.formatAreasSummaryWithData(areas, &result)
	}

	return &mcp.ToolsCallResult{Content: []mcp.ContentBlock{mcp.NewTextContent(result.String())}}, nil
}

func (h *ConsolidatedRegistryHandlers) handleAllJSON(
	ctx context.Context,
	entities []homeassistant.EntityRegistryEntry, entitiesErr error,
	devices []homeassistant.DeviceRegistryEntry, devicesErr error,
	areas []homeassistant.AreaRegistryEntry, areasErr error,
	includeDisabled bool,
) (*mcp.ToolsCallResult, error) {
	if entitiesErr != nil {
		return errorResult(fmt.Sprintf("Error getting entity registry: %v", entitiesErr)), nil
	}
	if devicesErr != nil {
		return errorResult(fmt.Sprintf("Error getting device registry: %v", devicesErr)), nil
	}
	if areasErr != nil {
		return errorResult(fmt.Sprintf("Error getting area registry: %v", areasErr)), nil
	}

	f := formatter.NewRegistryFormatter(formatter.FormatJSON)
	opts := formatter.RegistryOptions{IncludeDisabled: includeDisabled}
	output, err := f.FormatAllRegistries(ctx, entities, devices, areas, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting response: %v", err)), nil
	}

	return &mcp.ToolsCallResult{Content: []mcp.ContentBlock{mcp.NewTextContent(output)}}, nil
}

func (h *ConsolidatedRegistryHandlers) formatEntitiesSummaryWithData(
	entities []homeassistant.EntityRegistryEntry,
	includeDisabled bool,
	result *strings.Builder,
) {
	enabledCount := countEnabledEntities(entities, includeDisabled)
	fmt.Fprintf(result, "## Entities\n\nTotal: %d", len(entities))
	if !includeDisabled {
		fmt.Fprintf(result, " (enabled: %d)", enabledCount)
	}
	result.WriteString("\n\n")

	domainCounts := countEntitiesByDomain(entities, includeDisabled)
	writeCountMap(result, "By domain:", domainCounts)
}

func (h *ConsolidatedRegistryHandlers) formatDevicesSummaryWithData(
	devices []homeassistant.DeviceRegistryEntry,
	includeDisabled bool,
	result *strings.Builder,
) {
	enabledCount := countEnabledDevices(devices, includeDisabled)
	fmt.Fprintf(result, "## Devices\n\nTotal: %d", len(devices))
	if !includeDisabled {
		fmt.Fprintf(result, " (enabled: %d)", enabledCount)
	}
	result.WriteString("\n\n")

	manufacturerCounts := countDevicesByManufacturer(devices, includeDisabled)
	writeCountMap(result, "By manufacturer:", manufacturerCounts)
}

func (h *ConsolidatedRegistryHandlers) formatAreasSummaryWithData(
	areas []homeassistant.AreaRegistryEntry,
	result *strings.Builder,
) {
	fmt.Fprintf(result, "## Areas\n\nTotal: %d\n\n", len(areas))
	for _, a := range areas {
		fmt.Fprintf(result, "- %s (%s)\n", a.Name, a.AreaID)
	}
}

func countEnabledEntities(entities []homeassistant.EntityRegistryEntry, includeDisabled bool) int {
	count := 0
	for _, e := range entities {
		if includeDisabled || e.DisabledBy == "" {
			count++
		}
	}
	return count
}

func countEntitiesByDomain(entities []homeassistant.EntityRegistryEntry, includeDisabled bool) map[string]int {
	counts := make(map[string]int)
	for _, e := range entities {
		if !includeDisabled && e.DisabledBy != "" {
			continue
		}
		domain := extractDomain(e.EntityID)
		counts[domain]++
	}
	return counts
}

func countEnabledDevices(devices []homeassistant.DeviceRegistryEntry, includeDisabled bool) int {
	count := 0
	for _, d := range devices {
		if includeDisabled || d.DisabledBy == "" {
			count++
		}
	}
	return count
}

func countDevicesByManufacturer(devices []homeassistant.DeviceRegistryEntry, includeDisabled bool) map[string]int {
	counts := make(map[string]int)
	for _, d := range devices {
		if !includeDisabled && d.DisabledBy != "" {
			continue
		}
		manufacturer := d.Manufacturer
		if manufacturer == "" {
			manufacturer = "(unknown)"
		}
		counts[manufacturer]++
	}
	return counts
}

func writeCountMap(result *strings.Builder, header string, counts map[string]int) {
	var keys []string
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result.WriteString(header + "\n")
	for _, k := range keys {
		fmt.Fprintf(result, "- %s: %d\n", k, counts[k])
	}
	result.WriteString("\n")
}

// RegisterConsolidatedRegistryTools registers the consolidated get_registry tool.
func RegisterConsolidatedRegistryTools(registry *mcp.Registry) {
	h := NewConsolidatedRegistryHandlers()
	h.RegisterTools(registry)
}
