// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/jsonorder"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Automation action constants.
const (
	automationActionList     = "list"
	automationActionGet      = "get"
	automationActionCreate   = "create"
	automationActionUpdate   = "update"
	automationActionDelete   = "delete"
	automationActionToggle   = "toggle"
	automationActionCoverage = "coverage"
	automationActionPatch    = "patch"
	automationActionSchema   = "schema"
)

// automationForTimerReloadWarning is appended to patch/update success messages when the
// automation has triggers with a "for:" duration. Update/patch trigger an explicit
// automation.reload scoped to the edited automation's config id after a successful config write
// (see reloadDomainTargeted in waiter.go) so the change is immediately visible to a subsequent
// get — but that reload tears down the edited automation's own trigger runtime state
// (including any in-flight "for:" timer); HA rebuilds triggers on reload even for a single
// entity, and a state-trigger's "for:" countdown only starts on a state transition, so it cannot
// be resumed. Scoping the reload to just this automation's config id means OTHER
// automations' in-flight "for:" timers are no longer reset as collateral damage — only the one
// actually being edited is affected. If its monitored entity is already in the target state, no
// new transition occurs and the timer never re-fires — leaving actuators stuck until the next
// full state cycle.
const automationForTimerReloadWarning = " (warning: saving reloads the automation in Home Assistant, " +
	"which resets any in-flight 'for:' trigger timer; if the monitored entity is already in the " +
	"target state the trigger will not re-fire until the next state transition — verify dependent " +
	"actuators after this change)"

// automationReloadFailedWarning is appended to update/patch success messages when the
// post-write automation.reload service call itself fails. The config write to REST already
// succeeded, so the change is not lost — but until a reload succeeds (manual or automatic
// retry), get() may keep returning the pre-change config.
const automationReloadFailedWarning = " (warning: reload after save failed; changes may not be " +
	"visible or active until a manual automation.reload)"

// manualOnlyTrigger is a placeholder trigger for manual-only automations.
var manualOnlyTrigger = []any{
	map[string]any{"trigger": "event", "event_type": "ha_mcp_manual_only_placeholder"},
}

// triggersHaveForTimer reports whether any trigger in the slice defines a non-empty "for:" duration.
// Both string form ("00:15:00") and map form ({"minutes": 15}) are accepted.
// A for: duration means that HA resets an active countdown whenever the automation reloads,
// which happens on every config write — even a no-op write.
func triggersHaveForTimer(triggers []any) bool {
	for _, t := range triggers {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		switch v := tm["for"].(type) {
		case string:
			if v != "" {
				return true
			}
		case map[string]any:
			if len(v) > 0 {
				return true
			}
		}
	}
	return false
}

// resolveAutomationTriggers resolves trigger array from args, returning placeholder for empty arrays.
// Returns (triggers, autoFilled) where autoFilled indicates if placeholder was used.
func resolveAutomationTriggers(args map[string]any) ([]any, bool) {
	trigger, provided := args["trigger"].([]any)
	if !provided {
		return nil, false
	}
	if len(trigger) == 0 {
		return manualOnlyTrigger, true
	}
	return trigger, false
}

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
		Description: `Manage Home Assistant automations - list, get, create, update, delete, toggle, or analyze coverage.

Actions:
- list: List automations (optional filters: state, alias, entity_id; supports verbose, limit, cursor)
- get: Get details of a specific automation (requires automation_id)
- create: Create a new automation (requires alias, trigger, automation_action; trigger=[] for manual-only)
- update: Update an existing automation (requires automation_id); fields not passed are kept. variables replaces the whole block ({} or null removes it)
- delete: Delete an automation (requires automation_id)
- toggle: Enable or disable an automation (requires automation_id, enabled)
- coverage: Analyze which areas/entities lack automation coverage
- patch: Apply RFC 6902 JSON Patch operations (requires automation_id, operations)
- schema: Return reference for all valid trigger, condition, and action types with required/optional fields

Trigger shape: legacy types (state, numeric_state, ...) use flat keys (entity_id, for); purpose-specific triggers (e.g. power.crossed_threshold) use trigger/target/options. get returns the stored shape; write the shape that matches each trigger's type. See action=schema.`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Automation management operation",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Operation to perform: list, get, create, update, delete, toggle, coverage, patch, schema",
					Enum:        []string{"list", "get", "create", "update", "delete", "toggle", automationActionCoverage, "patch", "schema"},
				},
				"automation_id": {
					Type:        "string",
					Description: "Automation identifier. For create: optional config/storage ID (e.g. 'my_automation') — controls the REST API key, not the entity_id (HA always derives entity_id from the slugified alias). Useful when you need a stable config ID independent of the display name. For other actions: accepts entity_id (automation.xyz), config.id (UUID), or alias/friendly_name (case-insensitive partial match). Required for get/update/delete/toggle.",
				},
				"alias": {
					Type:        "string",
					Description: "Human-readable name for the automation (required for create, filter for list). This becomes the display name unless overridden by a registry custom name set via manage_entity(name=). Registry name takes precedence over alias.",
				},
				"description": {
					Type:        "string",
					Description: "Description of what the automation does",
				},
				"trigger": {
					Type:        "array",
					Description: "Triggers that start the automation (required for create; pass empty array [] for manual-only). Use action=schema to see all trigger types with required/optional fields.",
					Items:       &mcp.JSONSchema{Type: "object"},
				},
				"condition": {
					Type:        "array",
					Description: "Optional conditions that must be met before actions execute. Use action=schema to see all condition types with required/optional fields.",
					Items:       &mcp.JSONSchema{Type: "object"},
				},
				"automation_action": {
					Type:        "array",
					Description: "Actions to perform when triggered (required for create). Use action=schema to see all action types with required/optional fields.",
					Items:       &mcp.JSONSchema{Type: "object"},
				},
				"mode": {
					Type:        "string",
					Description: "Automation mode: single, restart, queued, parallel",
					Enum:        []string{"single", "restart", "queued", "parallel"},
				},
				"max": {
					Type:        "integer",
					Description: "Concurrent run limit (minimum 1, HA default 10). Only applies when mode is 'parallel' or 'queued'.",
				},
				"variables": {
					Type:        schemaTypeObject,
					Description: "Top-level automation variables, rendered by Home Assistant in key order (a later variable can reference an earlier one). On create it sets the block. On update it replaces the whole existing block; {} or null removes it; omit to keep the existing block.",
				},
				"enabled": {
					Type:        "boolean",
					Description: "Whether the automation should be enabled (for toggle action)",
				},
				"state": {
					Type:        "string",
					Description: "Filter by state: 'on' (enabled), 'off' (disabled) (for list action)",
				},
				attrEntityID: {
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
				"offset": paginationOffsetSchema(),
				"cursor": {
					Type:        "string",
					Description: "Pagination cursor from previous response (for list action)",
				},
				"format": {
					Type:        "string",
					Enum:        []string{"natural", formatJSON},
					Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				},
				"operations": patchOperationsSchema(),
				"dry_run":    dryRunSchema(),
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
	case automationActionCoverage:
		return h.handleCoverage(ctx, client, args)
	case automationActionPatch:
		return h.handlePatch(ctx, client, args)
	case automationActionSchema:
		return h.handleSchema()
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, delete, toggle, coverage, patch, or schema)", action)), nil
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
		return errorResult(fmt.Sprintf("error parsing pagination parameters: %v", err)), nil
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
	slices.SortFunc(automations, func(a, b homeassistant.Automation) int {
		return cmp.Compare(a.EntityID, b.EntityID)
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
		return errorResult("automation_id is required for get action. Use 'list' action to find IDs (shown in [brackets])"), nil
	}

	_, configID := normalizeAutomationID(automationID)

	automation, err := client.GetAutomation(ctx, configID)
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
	variables, _, err := parseVariablesArg(args)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	alias, _ := args["alias"].(string)
	if alias == "" {
		return errorResult("alias is required for create action"), nil
	}

	trigger, autoFilled := resolveAutomationTriggers(args)
	if trigger == nil {
		return errorResult("trigger is required for create action (pass empty array [] for manual-only automations)"), nil
	}

	automationAction, actionErr := createActions(args)
	if actionErr != "" {
		return errorResult(actionErr), nil
	}

	var id string
	if explicitID, ok := args["automation_id"].(string); ok && explicitID != "" {
		_, id = normalizeAutomationID(explicitID)
		if !isValidSlugID(id) {
			return errorResult("automation_id must contain only lowercase letters (a-z), digits, and underscores"), nil
		}
	} else {
		id = generateAutomationID(alias)
	}

	config := homeassistant.AutomationConfig{
		ID:          id,
		Alias:       alias,
		Description: getString(args, "description"),
		Triggers:    trigger,
		Conditions:  getSlice(args, "condition"),
		Actions:     automationAction,
		Mode:        getString(args, "mode"),
		Max:         getInt(args, "max"),
		Variables:   variables,
	}
	config.KeyOrder = automationArgsOrder(nil, mcp.ArgumentOrderFromContext(ctx), args, autoFilled)

	if err = client.CreateAutomation(ctx, config); err != nil {
		return errorResult(enrichAutomationError(fmt.Sprintf("Error creating automation: %v", err), err)), nil
	}

	// HA derives entity_id from alias (slugified), not from config ID.
	aliasSlug := generateAutomationID(alias)
	entityID := "automation." + aliasSlug
	successMsg := fmt.Sprintf("Automation '%s' created successfully (entity_id: %s, config_id: %s)", alias, entityID, id)
	if autoFilled {
		successMsg += " (manual-only: placeholder trigger inserted)"
	}

	if _, appeared := reloadTargetedAndWaitForEntity(ctx, client, "automation", id, entityID); !appeared {
		successMsg += " (warning: automation entity not yet visible, reload may be pending)"
	}

	return successResult(successMsg), nil
}

// createActions reads the actions of a create call, supporting both
// "automation_action" and the legacy "action" for backwards compatibility. It
// returns a non-empty error message when they are missing or empty.
func createActions(args map[string]any) ([]any, string) {
	automationAction, ok := args["automation_action"].([]any)
	if !ok {
		automationAction, ok = args["action"].([]any)
	}
	if !ok {
		return nil, "automation_action is required for create action"
	}
	if len(automationAction) == 0 {
		return nil, "automation_action must contain at least one action"
	}
	return automationAction, ""
}

func (h *AutomationHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	if _, _, err := parseVariablesArg(args); err != nil {
		return errorResult(err.Error()), nil
	}

	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return errorResult("automation_id is required for update action. Use 'list' action to find IDs (shown in [brackets])"), nil
	}

	entityID, configID := normalizeAutomationID(automationID)

	// Fetch current automation with fallback search
	current, err := client.GetAutomation(ctx, configID)
	if err != nil {
		// Fallback: try comprehensive search by entity_id, UUID, or alias
		current, err = h.findAutomationByID(ctx, client, automationID)
		if err != nil {
			return errorResult(fmt.Sprintf("Error getting current automation: %v", err)), nil
		}
	}

	if current.Config == nil {
		current.Config = &homeassistant.AutomationConfig{ID: configID}
	}

	// Detect no-op updates on the ordered encoding: no-op writes cause a needless
	// HA reload that resets in-flight "for:" trigger timers.
	if !applyAutomationUpdateOrdered(ctx, current.Config, args) {
		return successResult(fmt.Sprintf("Automation '%s': no changes detected, skipping write (reload and for: timer reset avoided)", automationID)), nil
	}

	// Resolve actual config ID for REST API (may differ from entity_id suffix)
	actualConfigID := configID
	if current.Config.ID != "" && current.Config.ID != configID {
		actualConfigID = current.Config.ID
	}

	// current.EntityID reflects the entity actually resolved above (findAutomationByID may have
	// matched a different entity than a bare entityID guess). Refuse to write an automation
	// whose id is not present in automations.yaml: the config API silently creates a duplicate
	// orphan entity instead of updating the existing one.
	checkEntityID := resolveWriteCheckEntityID(entityID, current.EntityID)
	if guardErr := configWriteGuardError(ctx, client, "automation", "update", automationID, checkEntityID, actualConfigID); guardErr != nil {
		return guardErr, nil
	}

	if err := client.UpdateAutomation(ctx, actualConfigID, *current.Config); err != nil {
		return errorResult(enrichAutomationError(fmt.Sprintf("Error updating automation: %v", err), err)), nil
	}

	successMsg := fmt.Sprintf("Automation '%s' updated successfully", automationID) +
		keptKeysNote(current.Config, automationOwnedKeys, variablesReplaced(args)...)
	if !reloadDomainTargeted(ctx, client, "automation", actualConfigID) {
		successMsg += automationReloadFailedWarning
	} else if triggersHaveForTimer(current.Config.Triggers) {
		successMsg += automationForTimerReloadWarning
	}
	return successResult(successMsg), nil
}

func (h *AutomationHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return errorResult("automation_id is required for delete action. Use 'list' action to find IDs (shown in [brackets])"), nil
	}

	_, configID := normalizeAutomationID(automationID)

	// Fetch automation to resolve actual config ID
	current, err := client.GetAutomation(ctx, configID)
	if err != nil {
		// Fallback: try comprehensive search by entity_id, UUID, or alias
		current, err = h.findAutomationByID(ctx, client, automationID)
		if err != nil {
			return errorResult(fmt.Sprintf("Error getting automation for deletion: %v", err)), nil
		}
	}

	// Resolve actual config ID for REST API (may differ from entity_id suffix)
	deleteID := configID
	if current.Config != nil && current.Config.ID != "" && current.Config.ID != configID {
		deleteID = current.Config.ID
	}

	if err := client.DeleteAutomation(ctx, deleteID); err != nil {
		return errorResult(fmt.Sprintf("Error deleting automation: %v", err)), nil
	}

	entityID, _ := normalizeAutomationID(automationID)
	_, _ = client.CallService(ctx, "automation", "reload", nil)
	successMsg := fmt.Sprintf("Automation '%s' deleted successfully", automationID)
	if !waitForEntityDisappear(ctx, client, entityID) {
		successMsg += " (warning: automation entity may still be visible until reload completes)"
	}

	return successResult(successMsg), nil
}

func (h *AutomationHandlers) handleToggle(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return errorResult("automation_id is required for toggle action. Use 'list' action to find IDs (shown in [brackets])"), nil
	}

	enabled, ok := args["enabled"].(bool)
	if !ok {
		return errorResult("enabled is required for " + automationActionToggle + " action"), nil
	}

	entityID, _ := normalizeAutomationID(automationID)

	if err := client.ToggleAutomation(ctx, entityID, enabled); err != nil {
		return errorResult(fmt.Sprintf("Error toggling automation: %v", err)), nil
	}

	state := "enabled"
	if !enabled {
		state = "disabled"
	}

	return successResult(fmt.Sprintf("Automation '%s' %s successfully", automationID, state)), nil
}

//nolint:funlen // Patch handler
func (h *AutomationHandlers) handlePatch(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	automationID, ok := args["automation_id"].(string)
	if !ok || automationID == "" {
		return errorResult("automation_id is required for patch action"), nil
	}

	ops, errResult := parseOperations(args)
	if errResult != nil {
		return errResult, nil
	}
	attachValueOrders(ops, mcp.ArgumentOrderFromContext(ctx).Field("operations"))

	entityID, configID := normalizeAutomationID(automationID)

	current, err := client.GetAutomation(ctx, configID)
	if err != nil {
		current, err = h.findAutomationByID(ctx, client, automationID)
		if err != nil {
			return errorResult(fmt.Sprintf("error getting automation: %v", err)), nil
		}
	}

	if current.Config == nil {
		current.Config = &homeassistant.AutomationConfig{ID: configID}
	}

	configMap, err := configToMap(current.Config)
	if err != nil {
		return errorResult(fmt.Sprintf("error processing automation config: %v", err)), nil
	}

	configOrder := current.Config.KeyOrder
	patchedMap, patchedOrder, resolvedOps, patchErr := applyPatchWithSemanticsOrdered(configMap, configOrder, ops)
	if patchErr != nil {
		return errorResult(fmt.Sprintf("error applying patch: %v", patchErr)), nil
	}

	if dryRun, _ := args["dry_run"].(bool); dryRun {
		return dryRunPatchResult(configMap, resolvedOps, "automation", automationID, len(ops))
	}

	actualConfigID := configID
	if current.Config.ID != "" && current.Config.ID != configID {
		actualConfigID = current.Config.ID
	}

	// current.EntityID reflects the entity actually resolved above (findAutomationByID may have
	// matched a different entity than a bare entityID guess).
	checkEntityID := resolveWriteCheckEntityID(entityID, current.EntityID)

	return applyPatchedAutomationWrite(ctx, client, automationID, checkEntityID, actualConfigID,
		configMap, patchedMap, configOrder, patchedOrder, current.Config.Triggers, len(ops))
}

// applyPatchedAutomationWrite writes the patched config to HA and returns the success result.
// It skips the write entirely when configMap and patchedMap encode to the same bytes in their
// key orders (configOrder, patchedOrder): every write to /api/config/automation/config/{id}
// causes HA to reload the automation, discarding any in-flight "for:" trigger timers — so a
// no-op write must never reach the REST API. A pure key reorder is NOT a no-op (it is how a
// config whose variables order was destroyed gets repaired, issue #311).
// When the write does happen, the success message includes a for: timer warning when relevant.
func applyPatchedAutomationWrite(
	ctx context.Context,
	client homeassistant.Client,
	automationID, entityID, actualConfigID string,
	configMap, patchedMap map[string]any,
	configOrder, patchedOrder *jsonorder.Order,
	oldTriggers []any,
	numOps int,
) (*mcp.ToolsCallResult, error) {
	if orderedConfigEqual(configMap, configOrder, patchedMap, patchedOrder) {
		return successResult(fmt.Sprintf("Automation '%s': no changes detected, skipping write (reload and for: timer reset avoided)", automationID)), nil
	}

	// Refuse to write an automation whose id is not present in automations.yaml: the config
	// API silently creates a duplicate orphan entity instead of updating it.
	if guardErr := configWriteGuardError(ctx, client, "automation", "patch", automationID, entityID, actualConfigID); guardErr != nil {
		return guardErr, nil
	}

	var newConfig homeassistant.AutomationConfig
	if err := mapToStruct(patchedMap, &newConfig); err != nil {
		return errorResult(fmt.Sprintf("error parsing patched config: %v", err)), nil
	}
	// mapToStruct round-trips through json.Marshal(map), so UnmarshalJSON captured
	// the SORTED key order; restore the patched document order before writing.
	newConfig.KeyOrder = patchedOrder

	if err := client.UpdateAutomation(ctx, actualConfigID, newConfig); err != nil {
		return errorResult(enrichAutomationError(fmt.Sprintf("error saving patched automation: %v", err), err)), nil
	}

	successMsg := fmt.Sprintf("Automation '%s' patched successfully (%d operations applied)", automationID, numOps)
	if !reloadDomainTargeted(ctx, client, "automation", actualConfigID) {
		successMsg += automationReloadFailedWarning
	} else if triggersHaveForTimer(newConfig.Triggers) || triggersHaveForTimer(oldTriggers) {
		// Warn when either the old or the new config uses "for:" triggers — the reload resets any
		// countdown that was already running at the moment of the write.
		successMsg += automationForTimerReloadWarning
	}
	return successResult(successMsg), nil
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
// Search order: entity_id, config.id (UUID), alias, friendly_name (case-insensitive partial match).
func (h *AutomationHandlers) findAutomationByID(ctx context.Context, client homeassistant.Client, searchID string) (*homeassistant.Automation, error) {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list automations: %w", err)
	}

	// 1. Try as entity_id (automation.xyz)
	if strings.HasPrefix(searchID, "automation.") {
		entityID := searchID
		for _, auto := range automations {
			if auto.EntityID == entityID {
				autoID := strings.TrimPrefix(auto.EntityID, "automation.")
				return client.GetAutomation(ctx, autoID)
			}
		}
	}

	// 2. Try as config.id (UUID)
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

	// 3. Search by alias/friendly_name (case-insensitive partial match)
	searchLower := strings.ToLower(searchID)
	for _, auto := range automations {
		autoID := strings.TrimPrefix(auto.EntityID, "automation.")
		fullAuto, getErr := client.GetAutomation(ctx, autoID)
		if getErr != nil {
			continue
		}

		// Check Config.Alias
		if fullAuto.Config != nil &&
			strings.Contains(strings.ToLower(fullAuto.Config.Alias), searchLower) {
			return fullAuto, nil
		}

		// Check FriendlyName
		if strings.Contains(strings.ToLower(fullAuto.FriendlyName), searchLower) {
			return fullAuto, nil
		}
	}

	return nil, fmt.Errorf("automation not found: %s (tried as automation_id, entity_id, config.id, and alias/friendly_name)", searchID)
}

// parseAutomationFilters extracts filter parameters from args.
func parseAutomationFilters(args map[string]any) automationFilters {
	return automationFilters{
		state:    getString(args, "state"),
		alias:    getString(args, "alias"),
		entityID: getString(args, attrEntityID),
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
		filtersMap[attrEntityID] = filters.entityID
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

// automationArgsOrder composes the key order of a config after
// applyAutomationConfigUpdates (or create) replaced subtrees from args. base is
// the config's current order (nil on create), argOrder the tool-call
// arguments' order (nil for a Go-map caller). Each subtree replaced from args
// takes the caller's order for that argument, or none: HA's previous order
// must never be applied by index to elements the caller supplied. Callers
// without argument order (internal Go-map callers) therefore get a nil
// sub-order, i.e. the replaced subtree is written sorted rather than with HA's
// stale order applied by index. The arg precedence mirrors
// applyAutomationConfigUpdates. base is never modified.
func automationArgsOrder(base, argOrder *jsonorder.Order, args map[string]any, autoFilledTrigger bool) *jsonorder.Order {
	order := base
	if trigger, ok := args["trigger"].([]any); ok {
		var sub *jsonorder.Order
		if len(trigger) > 0 && !autoFilledTrigger {
			sub = argOrder.Field("trigger")
		}
		order = withFieldOrder(order, "triggers", sub)
	}
	if _, ok := args["condition"].([]any); ok {
		order = withFieldOrder(order, "conditions", argOrder.Field("condition"))
	}
	if _, ok := args["automation_action"].([]any); ok {
		order = withFieldOrder(order, "actions", argOrder.Field("automation_action"))
	} else if _, ok := args["action"].([]any); ok {
		order = withFieldOrder(order, "actions", argOrder.Field("action"))
	}
	if vars, ok := args["variables"].(map[string]any); ok && len(vars) > 0 {
		order = withFieldOrder(order, "variables", argOrder.Field("variables"))
	}
	return order
}

// parseVariablesArg reads the top-level variables argument. present reports
// whether the key exists in args; nil and an empty object both mean "no
// variables" (vars == nil, i.e. remove the block on update); a non-empty object
// is returned as is. Anything else is an error: the server validates argument
// names, not types, so a string would otherwise be dropped silently.
func parseVariablesArg(args map[string]any) (vars map[string]any, present bool, err error) {
	raw, present := args["variables"]
	if !present || raw == nil {
		return nil, present, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, present, errors.New("variables must be an object (use {} or null to remove the block)")
	}
	if len(obj) == 0 {
		return nil, present, nil
	}
	return obj, present, nil
}

// variablesReplaced names the variables key for keptKeysNote when the caller
// supplied the argument (replaced or removed), so it is not reported as kept.
func variablesReplaced(args map[string]any) []string {
	if _, ok := args["variables"]; ok {
		return []string{"variables"}
	}
	return nil
}

// withFieldOrder sets the order of member key copy-on-write (sub is cloned so
// the result never aliases the caller's argument order). A nil order with a
// nil sub stays nil, so a config built without any recorded order keeps
// marshaling exactly like the plain struct.
func withFieldOrder(order *jsonorder.Order, key string, sub *jsonorder.Order) *jsonorder.Order {
	if order == nil && sub == nil {
		return nil
	}
	return order.WithField(key, sub.Clone())
}

// applyAutomationUpdateOrdered applies args to config, composes its key order
// and reports whether the ordered JSON encoding changed. Comparing ordered
// bytes (not maps) means a reorder-only update counts as a change, so it can
// repair a config whose key order was broken.
func applyAutomationUpdateOrdered(ctx context.Context, config *homeassistant.AutomationConfig, args map[string]any) bool {
	before, errBefore := json.Marshal(*config)
	applyAutomationConfigUpdates(config, args)
	_, autoFilled := resolveAutomationTriggers(args)
	config.KeyOrder = automationArgsOrder(config.KeyOrder, mcp.ArgumentOrderFromContext(ctx), args, autoFilled)
	after, errAfter := json.Marshal(*config)
	return errBefore != nil || errAfter != nil || !bytes.Equal(before, after)
}

// applyAutomationConfigUpdates applies provided fields from args to the automation config.
func applyAutomationConfigUpdates(config *homeassistant.AutomationConfig, args map[string]any) {
	if alias, ok := args["alias"].(string); ok && alias != "" {
		config.Alias = alias
	}
	if desc, ok := args["description"].(string); ok {
		config.Description = desc
	}
	if trigger, ok := args["trigger"].([]any); ok {
		if len(trigger) == 0 {
			config.Triggers = manualOnlyTrigger
		} else {
			config.Triggers = trigger
		}
	}
	if condition, ok := args["condition"].([]any); ok {
		config.Conditions = condition
	}
	// Support both "automation_action" and legacy "action"
	if automationAction, ok := args["automation_action"].([]any); ok {
		config.Actions = automationAction
	} else if action, ok := args["action"].([]any); ok {
		config.Actions = action
	}
	if mode, ok := args["mode"].(string); ok && mode != "" {
		config.Mode = mode
	}
	if maxVal, ok := args["max"].(float64); ok {
		config.Max = int(maxVal)
	}
	// The handlers reject an invalid variables argument before getting here, so
	// the error (and this function's lack of an error return) is moot.
	if vars, present, _ := parseVariablesArg(args); present {
		config.Variables = vars
	}
}

// getString safely extracts a string value from a map of arguments.
func getString(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// getInt safely extracts an integer value from a map of arguments.
// MCP JSON-RPC decodes all numbers as float64.
func getInt(args map[string]any, key string) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return 0
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

// normalizeAutomationID handles both "automation.xyz" and "xyz" formats.
// Returns (entityID, configID) where entityID includes the "automation." prefix
// and configID is the bare ID used for REST API calls.
func normalizeAutomationID(automationID string) (entityID, configID string) {
	if strings.HasPrefix(automationID, "automation.") {
		return automationID, strings.TrimPrefix(automationID, "automation.")
	}
	return "automation." + automationID, automationID
}

// generateAutomationID converts an alias to a valid automation ID that matches
// the entity_id Home Assistant will derive from the alias. HA slugifies the alias
// using python-slugify with text-unidecode, which transliterates accented chars to
// their ASCII base (e.g. u->u, o->o). We replicate this with NFD decomposition:
// decompose accented runes into base letter + combining mark, then drop combining marks.
func generateAutomationID(alias string) string {
	return homeassistant.Slugify(alias)
}

// isValidSlugID reports whether id consists only of lowercase ASCII letters,
// digits, and underscores — the characters HA accepts in a bare automation ID.
func isValidSlugID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

// enrichAutomationError appends actionable hints when HA returns a 400 validation
// error for automation create/update/patch operations. Delegates to enrichConfigError.
func enrichAutomationError(msg string, err error) string {
	return enrichConfigError(msg, err, automationErrorHints)
}
