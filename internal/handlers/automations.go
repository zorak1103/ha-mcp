// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Automation action constants.
const (
	automationActionList   = "list"
	automationActionGet    = "get"
	automationActionCreate = "create"
	automationActionUpdate = "update"
	automationActionDelete = "delete"
	automationActionToggle = "toggle"
)

// AutomationHandlers provides MCP tool handlers for automation operations.
type AutomationHandlers struct{}

// NewAutomationHandlers creates a new AutomationHandlers instance.
func NewAutomationHandlers() *AutomationHandlers {
	return &AutomationHandlers{}
}

// RegisterTools registers the consolidated manage_automation tool with the registry.
func (h *AutomationHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageAutomationTool(), h.handleManageAutomation)
}

// =============================================================================
// Tool Definition
// =============================================================================

//nolint:funlen // Tool schema definition
func (h *AutomationHandlers) manageAutomationTool() mcp.Tool {
	return mcp.Tool{
		Name: "manage_automation",
		Description: `Manage Home Assistant automations - list, get, create, update, delete, or toggle.

Actions:
- list: List automations (optional filters: state, alias, entity_id; supports verbose, limit, cursor)
- get: Get details of a specific automation (requires automation_id)
- create: Create a new automation (requires alias, trigger, action)
- update: Update an existing automation (requires automation_id)
- delete: Delete an automation (requires automation_id)
- toggle: Enable or disable an automation (requires automation_id, enabled)`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Automation management operation",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Operation to perform: list, get, create, update, delete, toggle",
					Enum:        []string{"list", "get", "create", "update", "delete", "toggle"},
				},
				"automation_id": {
					Type:        "string",
					Description: "Automation ID (required for get/update/delete/toggle)",
				},
				"alias": {
					Type:        "string",
					Description: "Human-readable name for the automation (required for create, filter for list)",
				},
				"description": {
					Type:        "string",
					Description: "Description of what the automation does",
				},
				"trigger": {
					Type:        "array",
					Description: "List of triggers that start the automation (required for create)",
				},
				"condition": {
					Type:        "array",
					Description: "Optional conditions that must be met",
				},
				"automation_action": {
					Type:        "array",
					Description: "Actions to perform when triggered (required for create)",
				},
				"mode": {
					Type:        "string",
					Description: "Automation mode: single, restart, queued, parallel",
					Enum:        []string{"single", "restart", "queued", "parallel"},
				},
				"enabled": {
					Type:        "boolean",
					Description: "Whether the automation should be enabled (for toggle action)",
				},
				"state": {
					Type:        "string",
					Description: "Filter by state: 'on' (enabled), 'off' (disabled) (for list action)",
				},
				"entity_id": {
					Type:        "string",
					Description: "Filter by entity used in the automation (for list action)",
				},
				"verbose": {
					Type:        "boolean",
					Description: "If true, return full details including configuration (for list action)",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of automations to return (for list action, max 1000)",
				},
				"cursor": {
					Type:        "string",
					Description: "Pagination cursor from previous response (for list action)",
				},
				"format": {
					Type:        "string",
					Enum:        []string{"natural", "json"},
					Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				},
			},
			Required: []string{"action"},
		},
	}
}

// =============================================================================
// Main Handler
// =============================================================================

func (h *AutomationHandlers) handleManageAutomation(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case automationActionList:
		return h.handleList(ctx, client, args)
	case automationActionGet:
		return h.handleGet(ctx, client, args)
	case automationActionCreate:
		return h.handleCreate(ctx, client, args)
	case automationActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case automationActionDelete:
		return h.handleDelete(ctx, client, args)
	case automationActionToggle:
		return h.handleToggle(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, delete, or toggle)", action)), nil
	}
}

// =============================================================================
// Action Handlers
// =============================================================================

func (h *AutomationHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error listing automations: %v", err)), nil
	}

	filters := parseAutomationFilters(args)
	verbose, _ := args["verbose"].(bool)
	format := formatter.ParseFormat(getString(args, "format"))

	filterResult := applyAutomationFilters(ctx, client, automations, filters)
	sortAutomationsByEntityID(filterResult.automations)

	filtersMap := buildAutomationFiltersMap(filters)
	paginationParams, err := ParsePaginationParams(args, filtersMap)
	if err != nil {
		return errorResult(fmt.Sprintf("Error: %v", err)), nil
	}

	paginated := ApplyPagination(filterResult.automations, paginationParams)
	configs := ensureAutomationConfigs(ctx, client, filterResult.configs, paginated.Items, format, verbose)

	f := formatter.NewAutomationFormatter(format)
	opts := formatter.AutomationListOptions{Verbose: verbose, Limit: paginationParams.Limit}

	output, err := f.FormatList(ctx, paginated.Items, configs, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting automations: %v", err)), nil
	}

	// For JSON format with pagination, wrap in pagination response
	if format == formatter.FormatJSON && paginationParams.Limit > 0 {
		return formatJSONPaginatedAutomations(ctx, client, paginated, configs, verbose)
	}

	// For natural format, include pagination info in text
	if format == formatter.FormatNatural && paginated.Pagination.HasMore {
		output += formatNaturalPaginationNote(paginated)
	}

	return successResult(output), nil
}

// sortAutomationsByEntityID sorts automations by entity_id for stable pagination.
func sortAutomationsByEntityID(automations []homeassistant.Automation) {
	sort.Slice(automations, func(i, j int) bool {
		return automations[i].EntityID < automations[j].EntityID
	})
}

// ensureAutomationConfigs loads configs if needed for formatting.
func ensureAutomationConfigs(ctx context.Context, client homeassistant.Client,
	existingConfigs map[string]*homeassistant.AutomationConfig, items []homeassistant.Automation,
	format formatter.Format, verbose bool) map[string]*homeassistant.AutomationConfig {
	if existingConfigs == nil && (format == formatter.FormatNatural || verbose) {
		return fetchAutomationConfigs(ctx, client, items)
	}
	return existingConfigs
}

// formatJSONPaginatedAutomations formats automations as JSON with pagination.
func formatJSONPaginatedAutomations(ctx context.Context, client homeassistant.Client,
	paginated PaginatedResponse[homeassistant.Automation],
	configs map[string]*homeassistant.AutomationConfig, verbose bool) (*mcp.ToolsCallResult, error) {
	var items json.RawMessage
	if verbose {
		items, _ = buildVerboseAutomationOutput(ctx, client, paginated.Items, configs)
	} else {
		items, _ = buildCompactAutomationOutput(paginated.Items)
	}
	response := buildPaginatedAutomationResponse(paginated, items)
	summary := BuildPaginationSummary(paginated.Pagination, "automations")
	if !verbose {
		summary += VerboseHint
	}
	return successResult(summary + "\n\n" + string(response)), nil
}

// formatNaturalPaginationNote formats pagination note for natural output.
func formatNaturalPaginationNote(paginated PaginatedResponse[homeassistant.Automation]) string {
	return fmt.Sprintf("\n\nShowing %d of %d. Use cursor '%s' to see more.",
		len(paginated.Items), paginated.Pagination.Total, safeDeref(paginated.Pagination.NextCursor))
}

func (h *AutomationHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return errorResult("automation_id is required for get action"), nil
	}

	normalizedID := strings.TrimPrefix(automationID, "automation.")

	automation, err := client.GetAutomation(ctx, normalizedID)
	if err != nil {
		automation, err = h.findAutomationByID(ctx, client, automationID)
		if err != nil {
			return errorResult(fmt.Sprintf("Error getting automation: %v", err)), nil
		}
	}

	format := formatter.ParseFormat(getString(args, "format"))
	f := formatter.NewAutomationFormatter(format)

	output, err := f.FormatDetail(ctx, *automation)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting automation: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *AutomationHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	alias, _ := args["alias"].(string)
	if alias == "" {
		return errorResult("alias is required for create action"), nil
	}

	trigger, _ := args["trigger"].([]any)
	if len(trigger) == 0 {
		return errorResult("trigger is required for create action"), nil
	}

	// Support both "action" and "automation_action" for backwards compatibility
	automationAction, _ := args["automation_action"].([]any)
	if len(automationAction) == 0 {
		// Try legacy "action" field
		automationAction, _ = args["action"].([]any)
	}
	if len(automationAction) == 0 {
		return errorResult("automation_action is required for create action"), nil
	}

	id := generateAutomationID(alias)

	config := homeassistant.AutomationConfig{
		ID:          id,
		Alias:       alias,
		Description: getString(args, "description"),
		Triggers:    trigger,
		Conditions:  getSlice(args, "condition"),
		Actions:     automationAction,
		Mode:        getString(args, "mode"),
	}

	if err := client.CreateAutomation(ctx, config); err != nil {
		return errorResult(fmt.Sprintf("Error creating automation: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Automation '%s' created successfully with ID '%s'", alias, id)), nil
}

func (h *AutomationHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return errorResult("automation_id is required for update action"), nil
	}

	current, err := client.GetAutomation(ctx, automationID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting current automation: %v", err)), nil
	}

	if current.Config == nil {
		current.Config = &homeassistant.AutomationConfig{ID: automationID}
	}
	applyAutomationConfigUpdates(current.Config, args)

	if err := client.UpdateAutomation(ctx, automationID, *current.Config); err != nil {
		return errorResult(fmt.Sprintf("Error updating automation: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Automation '%s' updated successfully", automationID)), nil
}

func (h *AutomationHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return errorResult("automation_id is required for delete action"), nil
	}

	if err := client.DeleteAutomation(ctx, automationID); err != nil {
		return errorResult(fmt.Sprintf("Error deleting automation: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Automation '%s' deleted successfully", automationID)), nil
}

func (h *AutomationHandlers) handleToggle(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return errorResult("automation_id is required for " + automationActionToggle + " action"), nil
	}

	enabled, ok := args["enabled"].(bool)
	if !ok {
		return errorResult("enabled is required for " + automationActionToggle + " action"), nil
	}

	if err := client.ToggleAutomation(ctx, automationID, enabled); err != nil {
		return errorResult(fmt.Sprintf("Error toggling automation: %v", err)), nil
	}

	state := "enabled"
	if !enabled {
		state = "disabled"
	}

	return successResult(fmt.Sprintf("Automation '%s' %s successfully", automationID, state)), nil
}

// =============================================================================
// Helper Types
// =============================================================================

// compactAutomationEntry represents a minimal automation entry for compact output.
type compactAutomationEntry struct {
	EntityID      string `json:"entity_id"`
	State         string `json:"state"`
	Alias         string `json:"alias,omitempty"`
	LastTriggered string `json:"last_triggered,omitempty"`
}

// verboseAutomationEntry represents a full automation entry including configuration.
type verboseAutomationEntry struct {
	EntityID      string                          `json:"entity_id"`
	State         string                          `json:"state"`
	FriendlyName  string                          `json:"friendly_name,omitempty"`
	LastTriggered string                          `json:"last_triggered,omitempty"`
	Config        *homeassistant.AutomationConfig `json:"config,omitempty"`
}

// automationFilters holds all filter parameters for listing automations.
type automationFilters struct {
	state    string
	alias    string
	entityID string
}

// automationListResult holds processed automation list data.
type automationListResult struct {
	automations []homeassistant.Automation
	configs     map[string]*homeassistant.AutomationConfig
}

// paginatedAutomationResponse wraps automation output with pagination metadata.
type paginatedAutomationResponse struct {
	Items      json.RawMessage    `json:"items"`
	Pagination PaginationMetadata `json:"pagination"`
}

// =============================================================================
// Helper Functions
// =============================================================================

// findAutomationByID searches for an automation by various ID formats.
func (h *AutomationHandlers) findAutomationByID(ctx context.Context, client homeassistant.Client, searchID string) (*homeassistant.Automation, error) {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list automations: %w", err)
	}

	if strings.HasPrefix(searchID, "automation.") {
		entityID := searchID
		for _, auto := range automations {
			if auto.EntityID == entityID {
				autoID := strings.TrimPrefix(auto.EntityID, "automation.")
				return client.GetAutomation(ctx, autoID)
			}
		}
	}

	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		fullAuto, getErr := client.GetAutomation(ctx, autoID)
		if getErr != nil {
			continue
		}

		if fullAuto.Config != nil && fullAuto.Config.ID == searchID {
			return fullAuto, nil
		}
	}

	return nil, fmt.Errorf("automation not found with ID: %s (tried as automation_id, entity_id, and config.id)", searchID)
}

// parseAutomationFilters extracts filter parameters from args.
func parseAutomationFilters(args map[string]any) automationFilters {
	return automationFilters{
		state:    getString(args, "state"),
		alias:    getString(args, "alias"),
		entityID: getString(args, "entity_id"),
	}
}

// matchesStateFilter checks if automation matches state filter.
func matchesStateFilter(auto homeassistant.Automation, stateFilter string) bool {
	return stateFilter == "" || auto.State == stateFilter
}

// matchesAliasFilter checks if automation matches alias filter (case-insensitive, partial match).
func matchesAliasFilter(auto homeassistant.Automation, aliasFilter string) bool {
	if aliasFilter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(auto.FriendlyName), strings.ToLower(aliasFilter))
}

// matchesEntityIDFilter checks if automation uses the specified entity ID.
func matchesEntityIDFilter(config *homeassistant.AutomationConfig, entityIDFilter string) bool {
	if entityIDFilter == "" {
		return true
	}
	if config == nil {
		return false
	}
	return searchEntityInAutomationConfig(config, entityIDFilter)
}

// needsConfigForFiltering determines if we need to fetch configs for filtering.
func (f automationFilters) needsConfigForFiltering() bool {
	return f.entityID != ""
}

// fetchAutomationConfigs fetches all automation configs in batch.
func fetchAutomationConfigs(
	ctx context.Context,
	client homeassistant.Client,
	automations []homeassistant.Automation,
) map[string]*homeassistant.AutomationConfig {
	configs := make(map[string]*homeassistant.AutomationConfig, len(automations))

	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		if autoID == auto.EntityID {
			continue
		}

		fullAuto, err := client.GetAutomation(ctx, autoID)
		if err == nil && fullAuto != nil && fullAuto.Config != nil {
			configs[autoID] = fullAuto.Config
		}
	}

	return configs
}

// applyAutomationFilters filters automations based on the provided filters.
func applyAutomationFilters(
	ctx context.Context,
	client homeassistant.Client,
	automations []homeassistant.Automation,
	filters automationFilters,
) automationListResult {
	var configs map[string]*homeassistant.AutomationConfig

	if filters.needsConfigForFiltering() {
		configs = fetchAutomationConfigs(ctx, client, automations)
	}

	filtered := make([]homeassistant.Automation, 0, len(automations))
	for _, auto := range automations {
		if !matchesStateFilter(auto, filters.state) {
			continue
		}
		if !matchesAliasFilter(auto, filters.alias) {
			continue
		}
		if filters.entityID != "" {
			autoID := strings.TrimPrefix(auto.EntityID, "automation.")
			if !matchesEntityIDFilter(configs[autoID], filters.entityID) {
				continue
			}
		}
		filtered = append(filtered, auto)
	}

	return automationListResult{
		automations: filtered,
		configs:     configs,
	}
}

// buildCompactAutomationOutput formats automations as compact JSON.
func buildCompactAutomationOutput(automations []homeassistant.Automation) ([]byte, error) {
	compact := make([]compactAutomationEntry, 0, len(automations))
	for _, auto := range automations {
		compact = append(compact, compactAutomationEntry{
			EntityID:      auto.EntityID,
			State:         auto.State,
			Alias:         auto.FriendlyName,
			LastTriggered: auto.LastTriggered,
		})
	}
	return json.MarshalIndent(compact, "", "  ")
}

// buildVerboseAutomationOutput formats automations with full config as JSON.
func buildVerboseAutomationOutput(
	ctx context.Context,
	client homeassistant.Client,
	automations []homeassistant.Automation,
	existingConfigs map[string]*homeassistant.AutomationConfig,
) ([]byte, error) {
	configs := existingConfigs
	if configs == nil {
		configs = fetchAutomationConfigs(ctx, client, automations)
	}

	verboseList := make([]verboseAutomationEntry, 0, len(automations))
	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		entry := verboseAutomationEntry{
			EntityID:      auto.EntityID,
			State:         auto.State,
			FriendlyName:  auto.FriendlyName,
			LastTriggered: auto.LastTriggered,
			Config:        configs[autoID],
		}
		verboseList = append(verboseList, entry)
	}

	return json.MarshalIndent(verboseList, "", "  ")
}

// buildAutomationFiltersMap creates a map of filter values for pagination hash.
func buildAutomationFiltersMap(filters automationFilters) map[string]any {
	filtersMap := make(map[string]any)
	if filters.state != "" {
		filtersMap["state"] = filters.state
	}
	if filters.alias != "" {
		filtersMap["alias"] = filters.alias
	}
	if filters.entityID != "" {
		filtersMap["entity_id"] = filters.entityID
	}
	return filtersMap
}

// buildPaginatedAutomationResponse creates the final response JSON.
func buildPaginatedAutomationResponse(paginated PaginatedResponse[homeassistant.Automation], itemsOutput []byte) []byte {
	if paginated.Pagination.Limit == 0 {
		return itemsOutput
	}

	response := paginatedAutomationResponse{
		Items:      itemsOutput,
		Pagination: paginated.Pagination,
	}
	result, _ := json.MarshalIndent(response, "", "  ")
	return result
}

// applyAutomationConfigUpdates applies provided fields from args to the automation config.
func applyAutomationConfigUpdates(config *homeassistant.AutomationConfig, args map[string]any) {
	if alias, ok := args["alias"].(string); ok && alias != "" {
		config.Alias = alias
	}
	if desc, ok := args["description"].(string); ok {
		config.Description = desc
	}
	if trigger, ok := args["trigger"].([]any); ok && len(trigger) > 0 {
		config.Triggers = trigger
	}
	if condition, ok := args["condition"].([]any); ok {
		config.Conditions = condition
	}
	// Support both "automation_action" and legacy "action"
	if automationAction, ok := args["automation_action"].([]any); ok && len(automationAction) > 0 {
		config.Actions = automationAction
	} else if action, ok := args["action"].([]any); ok && len(action) > 0 {
		config.Actions = action
	}
	if mode, ok := args["mode"].(string); ok && mode != "" {
		config.Mode = mode
	}
}

// getString safely extracts a string value from a map of arguments.
func getString(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// getSlice safely extracts a slice value from a map of arguments.
func getSlice(args map[string]any, key string) []any {
	if v, ok := args[key].([]any); ok {
		return v
	}
	return nil
}

// safeDeref safely dereferences a string pointer, returning empty string if nil.
func safeDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// searchEntityInAutomationConfig searches for an entity ID in automation triggers, conditions, and actions.
func searchEntityInAutomationConfig(config *homeassistant.AutomationConfig, entityID string) bool {
	if config == nil {
		return false
	}
	return searchInConfigSlice(config.Triggers, entityID) ||
		searchInConfigSlice(config.Conditions, entityID) ||
		searchInConfigSlice(config.Actions, entityID)
}

// searchInConfigSlice recursively searches for an entity ID in a config slice.
func searchInConfigSlice(items []any, entityID string) bool {
	for _, item := range items {
		if searchInConfigValue(item, entityID) {
			return true
		}
	}
	return false
}

// searchInConfigValue recursively searches for an entity ID in any config value.
func searchInConfigValue(val any, entityID string) bool {
	if val == nil {
		return false
	}

	switch v := val.(type) {
	case string:
		return v == entityID
	case []any:
		return searchInConfigSliceValue(v, entityID)
	case map[string]any:
		return searchInConfigMapValue(v, entityID)
	}

	return false
}

// searchInConfigSliceValue searches for entity ID in a slice value.
func searchInConfigSliceValue(items []any, entityID string) bool {
	for _, item := range items {
		if searchInConfigValue(item, entityID) {
			return true
		}
	}
	return false
}

// searchInConfigMapValue searches for entity ID in a map value.
func searchInConfigMapValue(m map[string]any, entityID string) bool {
	for key, subval := range m {
		if searchInConfigMapEntry(key, subval, entityID) {
			return true
		}
	}
	return false
}

// searchInConfigMapEntry checks a single map entry for entity ID.
func searchInConfigMapEntry(key string, subval any, entityID string) bool {
	if key == configKeyEntityID {
		return searchInConfigValue(subval, entityID)
	}

	if key == "target" {
		if found := searchTargetEntityID(subval, entityID); found {
			return true
		}
	}

	return searchInConfigValue(subval, entityID)
}

// searchTargetEntityID searches for entity ID in a target map.
func searchTargetEntityID(val any, entityID string) bool {
	targetMap, ok := val.(map[string]any)
	if !ok {
		return false
	}
	return searchInConfigValue(targetMap[configKeyEntityID], entityID)
}

// generateAutomationID converts an alias to a valid automation ID.
func generateAutomationID(alias string) string {
	var result strings.Builder
	prevUnderscore := false

	for _, r := range alias {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(unicode.ToLower(r))
			prevUnderscore = false
		} else if unicode.IsSpace(r) || r == '-' || r == '_' {
			if !prevUnderscore && result.Len() > 0 {
				result.WriteRune('_')
				prevUnderscore = true
			}
		}
	}

	s := result.String()
	return strings.TrimSuffix(s, "_")
}
