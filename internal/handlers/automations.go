// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// AutomationHandlers provides MCP tool handlers for automation operations.
type AutomationHandlers struct{}

// NewAutomationHandlers creates a new AutomationHandlers instance.
func NewAutomationHandlers() *AutomationHandlers {
	return &AutomationHandlers{}
}

// RegisterTools registers all automation-related tools with the registry.
func (h *AutomationHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.listAutomationsTool(), h.handleListAutomations)
	registry.RegisterTool(h.getAutomationTool(), h.handleGetAutomation)
	registry.RegisterTool(h.createAutomationTool(), h.handleCreateAutomation)
	registry.RegisterTool(h.updateAutomationTool(), h.handleUpdateAutomation)
	registry.RegisterTool(h.deleteAutomationTool(), h.handleDeleteAutomation)
	registry.RegisterTool(h.toggleAutomationTool(), h.handleToggleAutomation)
}

func (h *AutomationHandlers) listAutomationsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_automations",
		Description: "List all automations in Home Assistant. By default returns a compact list. Use filters to narrow down results and 'verbose' for full details including configuration.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Filter and output options for automations list",
			Properties: map[string]mcp.JSONSchema{
				"state": {
					Type:        "string",
					Description: "Filter by state: 'on' (enabled), 'off' (disabled), or omit for all",
				},
				"alias": {
					Type:        "string",
					Description: "Filter by alias/name (case-insensitive, partial match)",
				},
				"entity_id": {
					Type:        "string",
					Description: "Filter by entity used in the automation (searches triggers, conditions, and actions)",
				},
				"verbose": {
					Type:        "boolean",
					Description: "If true, return full details including configuration. Default: false (compact output with entity_id, state, alias, last_triggered)",
				},
			},
		},
	}
}

func (h *AutomationHandlers) getAutomationTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_automation",
		Description: "Get details of a specific automation",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Parameters for getting automation details",
			Properties: map[string]mcp.JSONSchema{
				"automation_id": {
					Type:        "string",
					Description: "The automation ID",
				},
			},
			Required: []string{"automation_id"},
		},
	}
}

func (h *AutomationHandlers) createAutomationTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_automation",
		Description: "Create a new automation in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Automation configuration",
			Properties: map[string]mcp.JSONSchema{
				"alias": {
					Type:        "string",
					Description: "Human-readable name for the automation",
				},
				"description": {
					Type:        "string",
					Description: "Description of what the automation does",
				},
				"trigger": {
					Type:        "array",
					Description: "List of triggers that start the automation",
				},
				"condition": {
					Type:        "array",
					Description: "Optional conditions that must be met",
				},
				"action": {
					Type:        "array",
					Description: "Actions to perform when triggered",
				},
				"mode": {
					Type:        "string",
					Description: "Automation mode: single, restart, queued, parallel",
				},
			},
			Required: []string{"alias", "trigger", "action"},
		},
	}
}

func (h *AutomationHandlers) updateAutomationTool() mcp.Tool {
	return mcp.Tool{
		Name:        "update_automation",
		Description: "Update an existing automation",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Automation ID and updated configuration",
			Properties: map[string]mcp.JSONSchema{
				"automation_id": {
					Type:        "string",
					Description: "The automation ID to update",
				},
				"alias": {
					Type:        "string",
					Description: "Human-readable name for the automation",
				},
				"description": {
					Type:        "string",
					Description: "Description of what the automation does",
				},
				"trigger": {
					Type:        "array",
					Description: "List of triggers that start the automation",
				},
				"condition": {
					Type:        "array",
					Description: "Optional conditions that must be met",
				},
				"action": {
					Type:        "array",
					Description: "Actions to perform when triggered",
				},
				"mode": {
					Type:        "string",
					Description: "Automation mode: single, restart, queued, parallel",
				},
			},
			Required: []string{"automation_id"},
		},
	}
}

func (h *AutomationHandlers) deleteAutomationTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_automation",
		Description: "Delete an automation from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Automation ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"automation_id": {
					Type:        "string",
					Description: "The automation ID to delete",
				},
			},
			Required: []string{"automation_id"},
		},
	}
}

func (h *AutomationHandlers) toggleAutomationTool() mcp.Tool {
	return mcp.Tool{
		Name:        "toggle_automation",
		Description: "Enable or disable an automation",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Automation ID and enabled state",
			Properties: map[string]mcp.JSONSchema{
				"automation_id": {
					Type:        "string",
					Description: "The automation ID",
				},
				"enabled": {
					Type:        "boolean",
					Description: "Whether the automation should be enabled",
				},
			},
			Required: []string{"automation_id", "enabled"},
		},
	}
}

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
// Requires the automation config to be loaded.
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
// This is more efficient than fetching one-by-one for entity_id filtering or verbose output.
func fetchAutomationConfigs(
	ctx context.Context,
	client homeassistant.Client,
	automations []homeassistant.Automation,
) map[string]*homeassistant.AutomationConfig {
	configs := make(map[string]*homeassistant.AutomationConfig, len(automations))

	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		if autoID == auto.EntityID {
			continue // Invalid entity_id format
		}

		fullAuto, err := client.GetAutomation(ctx, autoID)
		if err == nil && fullAuto.Config != nil {
			configs[autoID] = fullAuto.Config
		}
	}

	return configs
}

// applyAutomationFilters filters automations based on the provided filters.
// Returns filtered automations and their configs (if fetched).
func applyAutomationFilters(
	ctx context.Context,
	client homeassistant.Client,
	automations []homeassistant.Automation,
	filters automationFilters,
) automationListResult {
	var configs map[string]*homeassistant.AutomationConfig

	// Pre-fetch configs if needed for entity_id filtering
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
// Uses pre-fetched configs if available, otherwise fetches them.
func buildVerboseAutomationOutput(
	ctx context.Context,
	client homeassistant.Client,
	automations []homeassistant.Automation,
	existingConfigs map[string]*homeassistant.AutomationConfig,
) ([]byte, error) {
	// Ensure we have configs
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

// buildAutomationSummary creates the summary message for automation results.
func buildAutomationSummary(count int, verbose bool) string {
	summary := fmt.Sprintf("Found %d automations", count)
	if !verbose {
		summary += VerboseHint
	}
	return summary
}

func (h *AutomationHandlers) handleListAutomations(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	// Fetch all automations
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error listing automations: %v", err))},
			IsError: true,
		}, nil
	}

	// Parse filters and verbose flag
	filters := parseAutomationFilters(args)
	verbose, _ := args["verbose"].(bool)

	// Apply filters
	result := applyAutomationFilters(ctx, client, automations, filters)

	// Format output
	var output []byte
	if verbose {
		output, err = buildVerboseAutomationOutput(ctx, client, result.automations, result.configs)
	} else {
		output, err = buildCompactAutomationOutput(result.automations)
	}

	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting automations: %v", err))},
			IsError: true,
		}, nil
	}

	summary := buildAutomationSummary(len(result.automations), verbose)

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(summary + "\n\n" + string(output))},
	}, nil
}

func (h *AutomationHandlers) handleGetAutomation(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("automation_id is required")},
			IsError: true,
		}, nil
	}

	// Normalize the automation ID - strip "automation." prefix if present
	normalizedID := strings.TrimPrefix(automationID, "automation.")

	// First, try to get automation directly with the provided ID
	automation, err := client.GetAutomation(ctx, normalizedID)
	if err != nil {
		// If direct lookup failed, try to find by unique_id or entity_id
		automation, err = h.findAutomationByID(ctx, client, automationID)
		if err != nil {
			return &mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting automation: %v", err))},
				IsError: true,
			}, nil
		}
	}

	output, err := json.MarshalIndent(automation, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting automation: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(output))},
	}, nil
}

// findAutomationByID searches for an automation by various ID formats:
// - entity_id (automation.xxx)
// - unique_id (numeric ID from entity registry)
// - automation ID (the id field in automation config)
func (h *AutomationHandlers) findAutomationByID(ctx context.Context, client homeassistant.Client, searchID string) (*homeassistant.Automation, error) {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list automations: %w", err)
	}

	// Check if searchID matches entity_id pattern
	if strings.HasPrefix(searchID, "automation.") {
		entityID := searchID
		for _, auto := range automations {
			if auto.EntityID == entityID {
				autoID := strings.TrimPrefix(auto.EntityID, "automation.")
				return client.GetAutomation(ctx, autoID)
			}
		}
	}

	// Try to find by iterating through automations and checking config ID or unique_id
	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		fullAuto, getErr := client.GetAutomation(ctx, autoID)
		if getErr != nil {
			continue
		}

		// Check if config ID matches
		if fullAuto.Config != nil && fullAuto.Config.ID == searchID {
			return fullAuto, nil
		}
	}

	return nil, fmt.Errorf("automation not found with ID: %s (tried as automation_id, entity_id, and config.id)", searchID)
}

func (h *AutomationHandlers) handleCreateAutomation(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	alias, _ := args["alias"].(string)
	if alias == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("alias is required")},
			IsError: true,
		}, nil
	}

	trigger, _ := args["trigger"].([]any)
	if len(trigger) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("trigger is required")},
			IsError: true,
		}, nil
	}

	action, _ := args["action"].([]any)
	if len(action) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("action is required")},
			IsError: true,
		}, nil
	}

	// Generate ID from alias (lowercase, underscores)
	id := generateAutomationID(alias)

	config := homeassistant.AutomationConfig{
		ID:          id,
		Alias:       alias,
		Description: getString(args, "description"),
		Triggers:    trigger,
		Conditions:  getSlice(args, "condition"),
		Actions:     action,
		Mode:        getString(args, "mode"),
	}

	if err := client.CreateAutomation(ctx, config); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating automation: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Automation '%s' created successfully with ID '%s'", alias, id))},
	}, nil
}

func (h *AutomationHandlers) handleUpdateAutomation(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("automation_id is required")},
			IsError: true,
		}, nil
	}

	// Get current automation first
	current, err := client.GetAutomation(ctx, automationID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting current automation: %v", err))},
			IsError: true,
		}, nil
	}

	// Ensure Config exists
	if current.Config == nil {
		current.Config = &homeassistant.AutomationConfig{
			ID: automationID,
		}
	}

	// Update only provided fields in Config
	if alias, ok := args["alias"].(string); ok && alias != "" {
		current.Config.Alias = alias
	}
	if desc, ok := args["description"].(string); ok {
		current.Config.Description = desc
	}
	if trigger, ok := args["trigger"].([]any); ok && len(trigger) > 0 {
		current.Config.Triggers = trigger
	}
	if condition, ok := args["condition"].([]any); ok {
		current.Config.Conditions = condition
	}
	if action, ok := args["action"].([]any); ok && len(action) > 0 {
		current.Config.Actions = action
	}
	if mode, ok := args["mode"].(string); ok && mode != "" {
		current.Config.Mode = mode
	}

	if err := client.UpdateAutomation(ctx, automationID, *current.Config); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error updating automation: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Automation '%s' updated successfully", automationID))},
	}, nil
}

func (h *AutomationHandlers) handleDeleteAutomation(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("automation_id is required")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteAutomation(ctx, automationID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting automation: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Automation '%s' deleted successfully", automationID))},
	}, nil
}

func (h *AutomationHandlers) handleToggleAutomation(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("automation_id is required")},
			IsError: true,
		}, nil
	}

	enabled, ok := args["enabled"].(bool)
	if !ok {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("enabled is required")},
			IsError: true,
		}, nil
	}

	if err := client.ToggleAutomation(ctx, automationID, enabled); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error toggling automation: %v", err))},
			IsError: true,
		}, nil
	}

	state := "enabled"
	if !enabled {
		state = "disabled"
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Automation '%s' %s successfully", automationID, state))},
	}, nil
}

// getString safely extracts a string value from a map of arguments.
// It returns an empty string if the key doesn't exist or the value is not a string.
// This is a common pattern for handling optional parameters in MCP tool calls.
func getString(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// getSlice safely extracts a slice value from a map of arguments.
// It returns nil if the key doesn't exist or the value is not a slice.
// This is used for handling optional array parameters like conditions.
func getSlice(args map[string]any, key string) []any {
	if v, ok := args[key].([]any); ok {
		return v
	}
	return nil
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
	// Check entity_id fields directly
	if key == configKeyEntityID {
		return searchInConfigValue(subval, entityID)
	}

	// Check target.entity_id
	if key == "target" {
		if found := searchTargetEntityID(subval, entityID); found {
			return true
		}
	}

	// Recursively search all nested structures
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
// It transforms the alias to lowercase and replaces spaces/special characters with underscores.
// Example: "Turn On Living Room Lights" -> "turn_on_living_room_lights"
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

	// Trim trailing underscore
	s := result.String()
	return strings.TrimSuffix(s, "_")
}
