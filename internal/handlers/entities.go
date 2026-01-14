// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const configKeyEntityID = "entity_id"
const configKeyTarget = "target"

// EntityHandlers provides MCP tool handlers for entity operations.
type EntityHandlers struct{}

// NewEntityHandlers creates a new EntityHandlers instance.
func NewEntityHandlers() *EntityHandlers {
	return &EntityHandlers{}
}

// RegisterTools registers all entity-related tools with the registry.
func (h *EntityHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getStatesTool(), h.handleGetStates)
	registry.RegisterTool(h.getStateTool(), h.handleGetState)
	registry.RegisterTool(h.getHistoryTool(), h.handleGetHistory)
	registry.RegisterTool(h.listDomainsTool(), h.handleListDomains)
	registry.RegisterTool(h.getEntityDependenciesTool(), h.handleGetEntityDependencies)
}

func (h *EntityHandlers) getStatesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_states",
		Description: "Get all entity states from Home Assistant. By default returns a compact list with entity_id, state, and friendly_name. Use 'verbose' for full details including all attributes.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Optional filters for entity states",
			Properties: map[string]mcp.JSONSchema{
				"domain": {
					Type:        "string",
					Description: "Filter by domain (e.g., 'light', 'switch', 'sensor')",
				},
				"state": {
					Type:        "string",
					Description: "Filter by state value (e.g., 'on', 'off', 'unavailable', 'unknown')",
				},
				"state_not": {
					Type:        "string",
					Description: "Exclude entities with this state (e.g., 'unavailable' to exclude unavailable entities)",
				},
				"name_contains": {
					Type:        "string",
					Description: "Filter by entity_id or friendly_name containing this string (case-insensitive)",
				},
				"verbose": {
					Type:        "boolean",
					Description: "If true, return full details (all attributes, timestamps, context). Default: false (compact output with entity_id, state, friendly_name only)",
				},
			},
		},
	}
}

func (h *EntityHandlers) getStateTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_state",
		Description: "Get the state of a specific entity",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for getting entity state",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The entity ID (e.g., 'light.living_room')",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *EntityHandlers) getHistoryTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_history",
		Description: "Get historical state changes for an entity. By default returns compact output with state and timestamp. Use 'verbose' for full details.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for getting entity history",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The entity ID (e.g., 'sensor.temperature')",
				},
				"start_time": {
					Type:        "string",
					Description: "Start time in RFC3339 format (default: 24 hours ago). Alternative: use 'hours' parameter.",
				},
				"end_time": {
					Type:        "string",
					Description: "End time in RFC3339 format (default: now)",
				},
				"hours": {
					Type:        "number",
					Description: "Number of hours to look back from now (e.g., 6 for last 6 hours). Overrides start_time if specified.",
				},
				"state": {
					Type:        "string",
					Description: "Filter to only show entries with this state value",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of entries to return (most recent first). Default: all entries.",
				},
				"verbose": {
					Type:        "boolean",
					Description: "If true, return full details (all attributes). Default: false (compact output with state and timestamp only)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *EntityHandlers) listDomainsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_domains",
		Description: "List all available entity domains in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "No parameters required",
		},
	}
}

func (h *EntityHandlers) getEntityDependenciesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_entity_dependencies",
		Description: "Find all automations that use a specific entity. Shows where the entity is used as trigger, condition, or action target. Useful for understanding the impact of changing or removing an entity.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for finding entity dependencies",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The entity ID to search for (e.g., 'binary_sensor.motion_living_room')",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

// compactEntityState represents a minimal entity state for compact output.
type compactEntityState struct {
	EntityID     string `json:"entity_id"`
	State        string `json:"state"`
	FriendlyName string `json:"friendly_name,omitempty"`
}

// stateFilterParams holds parsed filter parameters for entity states.
type stateFilterParams struct {
	domain         string
	stateFilter    string
	stateNotFilter string
	nameContains   string
	verbose        bool
}

// parseStateFilterParams extracts filter parameters from args.
func parseStateFilterParams(args map[string]any) stateFilterParams {
	return stateFilterParams{
		domain:         getStringArg(args, "domain"),
		stateFilter:    getStringArg(args, "state"),
		stateNotFilter: getStringArg(args, "state_not"),
		nameContains:   getStringArg(args, "name_contains"),
		verbose:        getBoolArg(args, "verbose"),
	}
}

// getStringArg safely extracts a string argument.
func getStringArg(args map[string]any, key string) string {
	val, _ := args[key].(string)
	return val
}

// getBoolArg safely extracts a boolean argument.
func getBoolArg(args map[string]any, key string) bool {
	val, _ := args[key].(bool)
	return val
}

// filterStates applies all filters to a list of entities.
func filterStates(states []homeassistant.Entity, params stateFilterParams) []homeassistant.Entity {
	nameContainsLower := strings.ToLower(params.nameContains)
	filtered := make([]homeassistant.Entity, 0, len(states))

	for _, state := range states {
		if matchesStateFilters(state, params, nameContainsLower) {
			filtered = append(filtered, state)
		}
	}

	return filtered
}

// matchesStateFilters checks if a single entity matches all filters.
func matchesStateFilters(state homeassistant.Entity, params stateFilterParams, nameContainsLower string) bool {
	if params.domain != "" && !strings.HasPrefix(state.EntityID, params.domain+".") {
		return false
	}
	if params.stateFilter != "" && state.State != params.stateFilter {
		return false
	}
	if params.stateNotFilter != "" && state.State == params.stateNotFilter {
		return false
	}
	if params.nameContains != "" && !matchesNameFilter(state, nameContainsLower) {
		return false
	}
	return true
}

// matchesNameFilter checks if entity matches the name filter.
func matchesNameFilter(state homeassistant.Entity, nameContainsLower string) bool {
	if strings.Contains(strings.ToLower(state.EntityID), nameContainsLower) {
		return true
	}
	friendlyName, _ := state.Attributes["friendly_name"].(string)
	return strings.Contains(strings.ToLower(friendlyName), nameContainsLower)
}

// formatStatesOutput formats entity states based on verbose flag.
func formatStatesOutput(states []homeassistant.Entity, verbose bool) ([]byte, error) {
	if verbose {
		return json.MarshalIndent(states, "", "  ")
	}
	return json.MarshalIndent(toCompactStates(states), "", "  ")
}

// toCompactStates converts entities to compact format.
func toCompactStates(states []homeassistant.Entity) []compactEntityState {
	compact := make([]compactEntityState, 0, len(states))
	for _, state := range states {
		entry := compactEntityState{
			EntityID: state.EntityID,
			State:    state.State,
		}
		if friendlyName, ok := state.Attributes["friendly_name"].(string); ok {
			entry.FriendlyName = friendlyName
		}
		compact = append(compact, entry)
	}
	return compact
}

func (h *EntityHandlers) handleGetStates(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	states, err := client.GetStates(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting states: %v", err))},
			IsError: true,
		}, nil
	}

	params := parseStateFilterParams(args)
	states = filterStates(states, params)

	output, err := formatStatesOutput(states, params.verbose)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting states: %v", err))},
			IsError: true,
		}, nil
	}

	summary := fmt.Sprintf("Found %d entities", len(states))
	if !params.verbose {
		summary += VerboseHint
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(summary + "\n\n" + string(output))},
	}, nil
}

func (h *EntityHandlers) handleGetState(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting state: %v", err))},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting state: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}

// entityDependency represents where an entity is used in an automation.
type entityDependency struct {
	AutomationID    string   `json:"automation_id"`
	AutomationAlias string   `json:"automation_alias"`
	UsedIn          []string `json:"used_in"` // "trigger", "condition", "action"
}

// entityDependenciesResult is the result of get_entity_dependencies.
type entityDependenciesResult struct {
	EntityID    string             `json:"entity_id"`
	Automations []entityDependency `json:"automations"`
	TotalUsages int                `json:"total_usages"`
}

func (h *EntityHandlers) handleGetEntityDependencies(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error listing automations: %v", err))},
			IsError: true,
		}, nil
	}

	dependencies := findEntityDependencies(ctx, client, automations, entityID)

	result := entityDependenciesResult{
		EntityID:    entityID,
		Automations: dependencies,
		TotalUsages: len(dependencies),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting result: %v", err))},
			IsError: true,
		}, nil
	}

	summary := fmt.Sprintf("Found %d automations using '%s'", len(dependencies), entityID)

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(summary + "\n\n" + string(output))},
	}, nil
}

// findEntityDependencies searches all automations for entity usage.
func findEntityDependencies(ctx context.Context, client homeassistant.Client, automations []homeassistant.Automation, entityID string) []entityDependency {
	var dependencies []entityDependency

	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		fullAuto, err := client.GetAutomation(ctx, autoID)
		if err != nil {
			continue
		}

		if dep := checkAutomationForEntity(fullAuto, autoID, entityID); dep != nil {
			dependencies = append(dependencies, *dep)
		}
	}

	return dependencies
}

// checkAutomationForEntity checks if an automation uses the entity.
func checkAutomationForEntity(auto *homeassistant.Automation, autoID, entityID string) *entityDependency {
	usedIn := findEntityUsageLocations(auto.Config, entityID)
	if len(usedIn) == 0 {
		return nil
	}

	alias := auto.FriendlyName
	if auto.Config != nil && auto.Config.Alias != "" {
		alias = auto.Config.Alias
	}

	return &entityDependency{
		AutomationID:    autoID,
		AutomationAlias: alias,
		UsedIn:          usedIn,
	}
}

// findEntityUsageLocations finds where an entity is used in automation config.
func findEntityUsageLocations(config *homeassistant.AutomationConfig, entityID string) []string {
	var usedIn []string

	if config == nil {
		return usedIn
	}

	if searchEntityInConfig(config.Triggers, entityID) {
		usedIn = append(usedIn, "trigger")
	}
	if searchEntityInConfig(config.Conditions, entityID) {
		usedIn = append(usedIn, "condition")
	}
	if searchEntityInConfig(config.Actions, entityID) {
		usedIn = append(usedIn, "action")
	}

	return usedIn
}

// searchEntityInConfig recursively searches for an entity ID in a config structure.
func searchEntityInConfig(config any, entityID string) bool {
	if config == nil {
		return false
	}

	switch v := config.(type) {
	case string:
		return v == entityID
	case []any:
		return searchEntityInSlice(v, entityID)
	case map[string]any:
		return searchEntityInMap(v, entityID)
	}

	return false
}

// searchEntityInSlice searches for an entity ID in a slice.
func searchEntityInSlice(items []any, entityID string) bool {
	for _, item := range items {
		if searchEntityInConfig(item, entityID) {
			return true
		}
	}
	return false
}

// searchEntityInMap searches for an entity ID in a map structure.
func searchEntityInMap(m map[string]any, entityID string) bool {
	for key, val := range m {
		if searchEntityInMapEntry(key, val, entityID) {
			return true
		}
	}
	return false
}

// searchEntityInMapEntry checks a single map entry for entity ID references.
func searchEntityInMapEntry(key string, val any, entityID string) bool {
	// Check direct entity_id field
	if key == configKeyEntityID {
		return searchEntityInConfig(val, entityID)
	}

	// Check nested entity_id in target or data fields
	if key == configKeyTarget || key == "data" {
		if found := searchEntityInNestedMap(val, entityID); found {
			return true
		}
	}

	// Recursively search in nested structures
	return searchEntityInConfig(val, entityID)
}

// searchEntityInNestedMap checks for entity_id in a nested map.
func searchEntityInNestedMap(val any, entityID string) bool {
	nestedMap, ok := val.(map[string]any)
	if !ok {
		return false
	}
	return searchEntityInConfig(nestedMap[configKeyEntityID], entityID)
}

// compactHistoryEntry represents a minimal history entry for compact output.
type compactHistoryEntry struct {
	State       string `json:"state"`
	LastChanged string `json:"last_changed"`
}

// historyParams encapsulates all parsed parameters for history queries.
type historyParams struct {
	entityID    string
	startTime   time.Time
	endTime     time.Time
	stateFilter string
	limit       int
	verbose     bool
}

// historyResult encapsulates processed history data.
type historyResult struct {
	entries    []homeassistant.HistoryEntry
	totalCount int
}

// parseHistoryParams extracts and validates all parameters from args.
func parseHistoryParams(args map[string]any) (*historyParams, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return nil, fmt.Errorf("entity_id is required")
	}

	startTime, endTime, err := parseTimeRange(args)
	if err != nil {
		return nil, err
	}

	stateFilter, _ := args["state"].(string)

	limit := 0
	if limitVal, ok := args["limit"].(float64); ok && limitVal > 0 {
		limit = int(limitVal)
	}

	verbose, _ := args["verbose"].(bool)

	return &historyParams{
		entityID:    entityID,
		startTime:   startTime,
		endTime:     endTime,
		stateFilter: stateFilter,
		limit:       limit,
		verbose:     verbose,
	}, nil
}

// parseTimeRange parses start_time, end_time, and hours parameters.
func parseTimeRange(args map[string]any) (start, end time.Time, err error) {
	end = time.Now()
	start = end.Add(-24 * time.Hour)

	// 'hours' parameter takes precedence over 'start_time'
	if hours, ok := args["hours"].(float64); ok && hours > 0 {
		start = end.Add(-time.Duration(hours) * time.Hour)
	} else if startStr, ok := args["start_time"].(string); ok && startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start_time format: %w", err)
		}
	}

	if endStr, ok := args["end_time"].(string); ok && endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end_time format: %w", err)
		}
	}

	return start, end, nil
}

// processHistoryEntries flattens, filters, and limits history entries.
func processHistoryEntries(
	history [][]homeassistant.HistoryEntry,
	stateFilter string,
	limit int,
) historyResult {
	// Flatten history (it's [][]HistoryEntry, typically with one inner array per entity)
	var entries []homeassistant.HistoryEntry
	for _, entityHistory := range history {
		entries = append(entries, entityHistory...)
	}

	// Apply state filter
	if stateFilter != "" {
		filtered := make([]homeassistant.HistoryEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.State == stateFilter {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	totalCount := len(entries)

	// Apply limit (take most recent entries)
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	return historyResult{
		entries:    entries,
		totalCount: totalCount,
	}
}

// formatHistoryOutput formats entries based on verbose flag.
func formatHistoryOutput(entries []homeassistant.HistoryEntry, verbose bool) ([]byte, error) {
	if verbose {
		return json.MarshalIndent(entries, "", "  ")
	}

	compact := make([]compactHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		compact = append(compact, compactHistoryEntry{
			State:       entry.State,
			LastChanged: entry.LastChangedTime().Format(time.RFC3339),
		})
	}

	return json.MarshalIndent(compact, "", "  ")
}

// buildHistorySummary creates the summary message for history results.
func buildHistorySummary(entityID string, result historyResult, stateFilter string, verbose bool) string {
	var summary string

	if result.totalCount > len(result.entries) {
		summary = fmt.Sprintf("Showing %d of %d history entries for %s (limited)",
			len(result.entries), result.totalCount, entityID)
	} else {
		summary = fmt.Sprintf("Found %d history entries for %s", len(result.entries), entityID)
	}

	if stateFilter != "" {
		summary += fmt.Sprintf(" (filtered by state='%s')", stateFilter)
	}

	if !verbose {
		summary += VerboseHint
	}

	return summary
}

func (h *EntityHandlers) handleGetHistory(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	params, err := parseHistoryParams(args)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(err.Error())},
			IsError: true,
		}, nil
	}

	history, err := client.GetHistory(ctx, params.entityID, params.startTime, params.endTime)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting history: %v", err))},
			IsError: true,
		}, nil
	}

	result := processHistoryEntries(history, params.stateFilter, params.limit)

	output, err := formatHistoryOutput(result.entries, params.verbose)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting history: %v", err))},
			IsError: true,
		}, nil
	}

	summary := buildHistorySummary(params.entityID, result, params.stateFilter, params.verbose)

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(summary + "\n\n" + string(output))},
	}, nil
}

func (h *EntityHandlers) handleListDomains(ctx context.Context, client homeassistant.Client, _ map[string]any) (*mcp.ToolsCallResult, error) {
	states, err := client.GetStates(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting states: %v", err))},
			IsError: true,
		}, nil
	}

	// Extract unique domains using index lookup for efficiency
	domainSet := make(map[string]int)
	for _, state := range states {
		if idx := strings.Index(state.EntityID, "."); idx > 0 {
			domainSet[state.EntityID[:idx]]++
		}
	}

	// Build result
	type domainInfo struct {
		Domain string `json:"domain"`
		Count  int    `json:"entity_count"`
	}

	domains := make([]domainInfo, 0, len(domainSet))
	for domain, count := range domainSet {
		domains = append(domains, domainInfo{
			Domain: domain,
			Count:  count,
		})
	}

	output, err := json.MarshalIndent(domains, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting domains: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}
