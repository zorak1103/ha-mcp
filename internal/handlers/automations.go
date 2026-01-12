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
	EntityID      string                        `json:"entity_id"`
	State         string                        `json:"state"`
	FriendlyName  string                        `json:"friendly_name,omitempty"`
	LastTriggered string                        `json:"last_triggered,omitempty"`
	Config        *homeassistant.AutomationConfig `json:"config,omitempty"`
}

func (h *AutomationHandlers) handleListAutomations(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error listing automations: %v", err))},
			IsError: true,
		}, nil
	}

	// Parse filter parameters
	stateFilter, _ := args["state"].(string)
	aliasFilter, _ := args["alias"].(string)
	entityIDFilter, _ := args["entity_id"].(string)
	verbose, _ := args["verbose"].(bool)

	// Normalize filter for case-insensitive matching
	aliasFilterLower := strings.ToLower(aliasFilter)

	// Filter automations
	filtered := make([]homeassistant.Automation, 0, len(automations))
	for _, automation := range automations {
		// Apply state filter
		if stateFilter != "" && automation.State != stateFilter {
			continue
		}

		// Apply alias filter (case-insensitive, partial match)
		if aliasFilter != "" && !strings.Contains(strings.ToLower(automation.FriendlyName), aliasFilterLower) {
			continue
		}

		// Apply entity_id filter (requires fetching full config)
		if entityIDFilter != "" {
			autoID := strings.TrimPrefix(automation.EntityID, "automation.")
			fullAuto, err := client.GetAutomation(ctx, autoID)
			if err != nil || fullAuto.Config == nil {
				continue
			}
			// Search in triggers, conditions, and actions
			found := searchEntityInAutomationConfig(fullAuto.Config, entityIDFilter)
			if !found {
				continue
			}
		}

		filtered = append(filtered, automation)
	}

	// Format output based on verbose flag
	var output []byte
	if verbose {
		// Verbose output: fetch full configuration for each automation
		verboseList := make([]verboseAutomationEntry, 0, len(filtered))
		for _, automation := range filtered {
			entry := verboseAutomationEntry{
				EntityID:      automation.EntityID,
				State:         automation.State,
				FriendlyName:  automation.FriendlyName,
				LastTriggered: automation.LastTriggered,
			}

			// Extract automation ID from entity_id (automation.xxx -> xxx)
			automationID := strings.TrimPrefix(automation.EntityID, "automation.")
			if automationID != automation.EntityID {
				// Fetch full automation config
				fullAutomation, err := client.GetAutomation(ctx, automationID)
				if err == nil && fullAutomation.Config != nil {
					entry.Config = fullAutomation.Config
				}
			}

			verboseList = append(verboseList, entry)
		}
		output, err = json.MarshalIndent(verboseList, "", "  ")
	} else {
		// Compact output: only essential fields
		compact := make([]compactAutomationEntry, 0, len(filtered))
		for _, automation := range filtered {
			compact = append(compact, compactAutomationEntry{
				EntityID:      automation.EntityID,
				State:         automation.State,
				Alias:         automation.FriendlyName,
				LastTriggered: automation.LastTriggered,
			})
		}
		output, err = json.MarshalIndent(compact, "", "  ")
	}

	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error formatting automations: %v", err))},
			IsError: true,
		}, nil
	}

	// Add summary info
	summary := fmt.Sprintf("Found %d automations", len(filtered))
	if !verbose {
		summary += VerboseHint
	}

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

	automation, err := client.GetAutomation(ctx, automationID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting automation: %v", err))},
			IsError: true,
		}, nil
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
		for _, item := range v {
			if searchInConfigValue(item, entityID) {
				return true
			}
		}
	case map[string]any:
		for key, subval := range v {
			// Check entity_id fields directly
			if key == configKeyEntityID {
				if searchInConfigValue(subval, entityID) {
					return true
				}
			}
			// Check target.entity_id
			if key == "target" {
				if targetMap, ok := subval.(map[string]any); ok {
					if searchInConfigValue(targetMap[configKeyEntityID], entityID) {
						return true
					}
				}
			}
			// Recursively search all nested structures
			if searchInConfigValue(subval, entityID) {
				return true
			}
		}
	}
	return false
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
