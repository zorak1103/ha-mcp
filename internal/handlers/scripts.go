// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Script action constants.
const (
	scriptActionList    = "list"
	scriptActionGet     = "get"
	scriptActionCreate  = "create"
	scriptActionUpdate  = "update"
	scriptActionDelete  = "delete"
	scriptActionExecute = "execute"
	scriptActionPatch   = "patch"
)

// scriptReloadFailedWarning is appended to update/patch success messages when the post-write
// script.reload service call itself fails. The config write to REST already succeeded, so the
// change is not lost — but until a reload succeeds (manual or automatic retry), get() may keep
// returning the pre-change config (#126).
const scriptReloadFailedWarning = " (warning: reload after save failed; changes may not be " +
	"visible or active until a manual script.reload)"

// ScriptHandlers provides handlers for script-related MCP tools.
type ScriptHandlers struct{}

// NewScriptHandlers creates a new ScriptHandlers instance.
func NewScriptHandlers() *ScriptHandlers {
	return &ScriptHandlers{}
}

// RegisterTools registers the consolidated manage_script tool with the registry.
func (h *ScriptHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageScriptTool(), h.handleManageScript)
	registry.RegisterTool(h.callServiceTool(), h.handleCallService)
}

// =============================================================================
// Tool Definitions
// =============================================================================

//nolint:funlen // Tool schema definition
func (h *ScriptHandlers) manageScriptTool() mcp.Tool {
	return mcp.Tool{
		Name: "manage_script",
		Description: `Manage Home Assistant scripts - list, get, create, update, delete, or execute.

Actions:
- list: List all scripts in Home Assistant
- get: Get details of a specific script (requires script_id)
- create: Create a new script (requires script_id, alias, sequence)
- update: Update an existing script (requires script_id)
- delete: Delete a script (requires script_id)
- execute: Execute a script (requires script_id, optional variables)

- patch: Apply RFC 6902 JSON Patch operations (requires script_id, operations)`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Script management operation",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Operation to perform: list, get, create, update, delete, execute, patch",
					Enum:        []string{"list", "get", "create", "update", "delete", "execute", "patch"},
				},
				"script_id": {
					Type:        "string",
					Description: "Script identifier. For create: use bare ID without 'script.' prefix (e.g., 'morning_routine'). For other actions: accepts entity_id (script.xyz) or alias/friendly_name (case-insensitive partial match).",
				},
				"alias": {
					Type:        "string",
					Description: "Friendly name for the script (required for create)",
				},
				"description": {
					Type:        "string",
					Description: "Description of what the script does",
				},
				"mode": {
					Type:        "string",
					Description: "Script mode: single, restart, queued, parallel",
					Enum:        []string{"single", "restart", "queued", "parallel"},
					Default:     "single",
				},
				"max": {
					Type:        "integer",
					Description: "Concurrent run limit (minimum 1, HA default 10). Only applies when mode is 'parallel' or 'queued'.",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the script (e.g., mdi:script)",
				},
				"sequence": {
					Type:        "array",
					Description: "Array of actions to execute (required for create)",
					Items: &mcp.JSONSchema{
						Type:        "object",
						Description: "Action object",
					},
				},
				"fields": {
					Type:        "object",
					Description: "Input fields for the script",
				},
				"variables": {
					Type:        "object",
					Description: "Variables to pass when executing the script",
				},
				"format": {
					Type:        "string",
					Enum:        []string{"natural", "json"},
					Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				},
				"operations": patchOperationsSchema(),
				"dry_run":    dryRunSchema(),
			},
			Required: []string{"action"},
		},
	}
}

func (h *ScriptHandlers) callServiceTool() mcp.Tool {
	return mcp.Tool{
		Name:        "call_service",
		Description: "Call any Home Assistant service. Pass entity_id and service parameters inside the 'data' object.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"domain": {
					Type:        "string",
					Description: "Service domain (e.g., light, switch, climate)",
				},
				"service": {
					Type:        "string",
					Description: "Service name (e.g., turn_on, turn_off, toggle)",
				},
				"data": {
					Type:        "object",
					Description: "Service data object. Include entity_id and parameters here (e.g., {\"entity_id\": \"light.living_room\", \"brightness\": 255})",
				},
				"format": {
					Type:        "string",
					Enum:        []string{"natural", "json"},
					Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				},
			},
			Required: []string{"domain", "service"},
		},
	}
}

// =============================================================================
// Main Handler
// =============================================================================

func (h *ScriptHandlers) handleManageScript(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case scriptActionList:
		return h.handleList(ctx, client, args)
	case scriptActionGet:
		return h.handleGet(ctx, client, args)
	case scriptActionCreate:
		return h.handleCreate(ctx, client, args)
	case scriptActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case scriptActionDelete:
		return h.handleDelete(ctx, client, args)
	case scriptActionExecute:
		return h.handleExecute(ctx, client, args)
	case scriptActionPatch:
		return h.handlePatch(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, delete, execute, or patch)", action)), nil
	}
}

// =============================================================================
// Action Handlers
// =============================================================================

func (h *ScriptHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scripts, err := client.ListScripts(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error listing scripts: %v", err)), nil
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	// Convert Entity list to Script list with configs
	scriptList := make([]homeassistant.Script, 0, len(scripts))
	for _, s := range scripts {
		script := homeassistant.Script{
			EntityID: s.EntityID,
			State:    s.State,
		}
		if name, ok := s.Attributes["friendly_name"].(string); ok {
			script.FriendlyName = name
		}
		if lastTriggered, ok := s.Attributes["last_triggered"].(string); ok {
			script.LastTriggered = lastTriggered
		}
		// Try to get full script config for natural output
		if format == formatter.FormatNatural {
			scriptID := strings.TrimPrefix(s.EntityID, "script.")
			if fullScript, getErr := client.GetScript(ctx, scriptID); getErr == nil {
				script.Config = fullScript.Config
			}
		}
		scriptList = append(scriptList, script)
	}

	f := formatter.NewScriptFormatter(format)
	opts := formatter.ScriptListOptions{}

	output, err := f.FormatList(ctx, scriptList, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting scripts: %v", err)), nil
	}

	return successResult(output), nil
}

func (h *ScriptHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return errorResult("script_id is required for get action"), nil
	}

	script, err := client.GetScript(ctx, scriptID)
	if err != nil {
		// Fallback: search by alias/friendly_name
		script, err = h.findScriptByID(ctx, client, scriptID)
		if err != nil {
			return errorResult(fmt.Sprintf("error getting script: %v", err)), nil
		}
	}

	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)
	f := formatter.NewScriptFormatter(format)

	output, err := f.FormatDetail(ctx, *script)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting script: %v", err)), nil
	}

	return successResult(output), nil
}

// normalizeScriptID normalizes script ID inputs to handle prefix variations.
// Returns: (entityID with "script." prefix, configID without prefix)
// Examples:
//   - "script.morning_routine" -> ("script.morning_routine", "morning_routine")
//   - "morning_routine" -> ("script.morning_routine", "morning_routine")
func normalizeScriptID(scriptID string) (entityID, configID string) {
	if strings.HasPrefix(scriptID, "script.") {
		return scriptID, strings.TrimPrefix(scriptID, "script.")
	}
	return "script." + scriptID, scriptID
}

// findScriptByID searches for a script by various ID formats.
// Search order: entity_id (script.xyz), alias, friendly_name (case-insensitive partial match).
func (h *ScriptHandlers) findScriptByID(ctx context.Context, client homeassistant.Client, searchID string) (*homeassistant.Script, error) {
	scripts, err := client.ListScripts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list scripts: %w", err)
	}

	// 1. Try as entity_id (script.xyz)
	if strings.HasPrefix(searchID, "script.") {
		for _, s := range scripts {
			if s.EntityID == searchID {
				scriptID := strings.TrimPrefix(s.EntityID, "script.")
				return client.GetScript(ctx, scriptID)
			}
		}
	}

	// 2. Search by alias/friendly_name (case-insensitive partial match)
	searchLower := strings.ToLower(searchID)
	for _, s := range scripts {
		scriptID := strings.TrimPrefix(s.EntityID, "script.")
		fullScript, getErr := client.GetScript(ctx, scriptID)
		if getErr != nil {
			continue
		}

		// Check Config.Alias
		if fullScript.Config != nil &&
			strings.Contains(strings.ToLower(fullScript.Config.Alias), searchLower) {
			return fullScript, nil
		}

		// Check FriendlyName
		if strings.Contains(strings.ToLower(fullScript.FriendlyName), searchLower) {
			return fullScript, nil
		}
	}

	return nil, fmt.Errorf("script not found: %s (tried as entity_id and alias/friendly_name)", searchID)
}

func (h *ScriptHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return errorResult("script_id is required for create action"), nil
	}

	alias, ok := args["alias"].(string)
	if !ok || alias == "" {
		return errorResult("alias is required for create action"), nil
	}

	sequence, ok := args["sequence"].([]any)
	if !ok {
		return errorResult("sequence is required for create action and must be an array"), nil
	}
	if len(sequence) == 0 {
		return errorResult("sequence must contain at least one action"), nil
	}

	config := homeassistant.ScriptConfig{
		Alias:    alias,
		Sequence: sequence,
	}

	if description, ok := args["description"].(string); ok {
		config.Description = description
	}
	if mode, ok := args["mode"].(string); ok {
		config.Mode = mode
	}
	if maxVal, ok := args["max"].(float64); ok {
		config.Max = int(maxVal)
	}
	if icon, ok := args["icon"].(string); ok {
		config.Icon = icon
	}
	if fields, ok := args["fields"].(map[string]any); ok {
		config.Fields = fields
	}

	if err := client.CreateScript(ctx, scriptID, config); err != nil {
		return errorResult(fmt.Sprintf("Error creating script: %v", err)), nil
	}

	entityID, _ := normalizeScriptID(scriptID)
	successMsg := fmt.Sprintf("Script '%s' created successfully", scriptID)
	if _, appeared := reloadAndWaitForEntity(ctx, client, "script", entityID); !appeared {
		successMsg += " (warning: script entity not yet visible, reload may be pending)"
	}

	return successResult(successMsg), nil
}

// applyScriptConfigUpdates overrides config fields with any values present in args, leaving
// fields absent from args untouched so existing values are preserved. Extracted from
// handleUpdate as a pure function (no ctx/client) to keep the handler's cyclomatic complexity
// down; mirrors applyAutomationConfigUpdates in automations.go.
func applyScriptConfigUpdates(config *homeassistant.ScriptConfig, args map[string]any) {
	if alias, ok := args["alias"].(string); ok {
		config.Alias = alias
	}
	if description, ok := args["description"].(string); ok {
		config.Description = description
	}
	if mode, ok := args["mode"].(string); ok {
		config.Mode = mode
	}
	if maxVal, ok := args["max"].(float64); ok {
		config.Max = int(maxVal)
	}
	if icon, ok := args["icon"].(string); ok {
		config.Icon = icon
	}
	if sequence, ok := args["sequence"].([]any); ok {
		config.Sequence = sequence
	}
	if fields, ok := args["fields"].(map[string]any); ok {
		config.Fields = fields
	}
}

func (h *ScriptHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return errorResult("script_id is required for update action"), nil
	}

	// Normalize ID to handle prefix variations
	entityID, configID := normalizeScriptID(scriptID)

	// Get current script configuration to preserve existing values
	current, err := client.GetScript(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting current script: %v", err)), nil
	}

	// Start with existing config to preserve all fields
	config := homeassistant.ScriptConfig{}
	if current.Config != nil {
		config = *current.Config
	} else {
		// Fallback: at least preserve the alias from friendly_name
		config.Alias = current.FriendlyName
	}

	// Snapshot config before mutation so we can detect no-op updates.
	// No-op writes cause a needless script.reload.
	beforeMap, _ := configToMap(config)

	applyScriptConfigUpdates(&config, args)

	afterMap, _ := configToMap(config)
	if reflect.DeepEqual(beforeMap, afterMap) {
		return successResult(fmt.Sprintf("Script '%s': no changes detected, skipping write (reload avoided)", scriptID)), nil
	}

	// Refuse to write YAML-defined scripts: the config API silently creates a duplicate
	// orphan entity instead of updating them (#122).
	checkEntityID := resolveWriteCheckEntityID(entityID, current.EntityID)
	if guardErr := yamlWriteGuardError(ctx, client, "script", "update", scriptID, checkEntityID); guardErr != nil {
		return guardErr, nil
	}

	// Use configID (without prefix) for REST API
	if err := client.UpdateScript(ctx, configID, config); err != nil {
		msg := fmt.Sprintf("Error updating script: %v", err)
		return errorResult(enrichConfigError(msg, err, scriptErrorHints)), nil
	}

	successMsg := fmt.Sprintf("Script '%s' updated successfully", scriptID)
	if !reloadDomain(ctx, client, "script") {
		successMsg += scriptReloadFailedWarning
	}
	return successResult(successMsg), nil
}

func (h *ScriptHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return errorResult("script_id is required for delete action"), nil
	}

	// Normalize ID to handle prefix variations - use configID for REST API
	entityID, configID := normalizeScriptID(scriptID)

	viaRegistry, errMsg := deleteScriptWithRegistryFallback(ctx, client, configID, entityID)
	if errMsg != "" {
		return errorResult(errMsg), nil
	}

	_, _ = client.CallService(ctx, "script", "reload", nil)
	successMsg := fmt.Sprintf("Script '%s' deleted successfully", scriptID)
	if viaRegistry {
		successMsg += " (removed via entity registry; script was not storage-managed)"
	}
	if !waitForEntityDisappear(ctx, client, entityID) {
		successMsg += " (warning: script entity may still be visible until reload completes)"
	}

	return successResult(successMsg), nil
}

// deleteScriptWithRegistryFallback deletes a script via the storage-config API, falling back to
// entity-registry removal when the config API reports the resource as not found. The storage-
// config API only knows storage-managed scripts; YAML-defined or orphan-duplicate scripts
// (entity object_id differs from the storage key) 404/400 here even though the entity exists
// and is readable via get/list (#123). Returns viaRegistry=true if the fallback path was used,
// or a non-empty errMsg describing the failure.
func deleteScriptWithRegistryFallback(ctx context.Context, client homeassistant.Client, configID, entityID string) (viaRegistry bool, errMsg string) {
	err := client.DeleteScript(ctx, configID)
	if err == nil {
		return false, ""
	}
	if !isNotFoundError(err) {
		return false, fmt.Sprintf("Error deleting script: %v", err)
	}
	if regErr := deleteScriptViaRegistry(ctx, client, entityID); regErr != nil {
		return false, fmt.Sprintf("Error deleting script: %v (registry fallback also failed: %v)", err, regErr)
	}
	return true, ""
}

// isNotFoundError reports whether err indicates the target resource does not exist. Covers
// both HA's 404 form ("script not found: X") and the observed 400 body
// ("unexpected status 400: {\"message\":\"Resource not found\"}") — see #123.
func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}

// deleteScriptViaRegistry removes a script by its entity-registry entry. This is the fallback
// for scripts the storage-config DELETE API can't find (YAML-defined or orphan-duplicate
// entities whose object_id differs from the storage key). Mirrors the registry-based deletion
// path used by manage_entity delete and HybridClient.DeleteHelper.
func deleteScriptViaRegistry(ctx context.Context, client homeassistant.Client, entityID string) error {
	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.EntityID == entityID {
			return client.RemoveEntityRegistryEntry(ctx, entityID)
		}
	}
	return fmt.Errorf("script %q not found in entity registry", entityID)
}

func (h *ScriptHandlers) handleExecute(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return errorResult("script_id is required for execute action"), nil
	}

	// Normalize ID to handle prefix variations
	entityID, _ := normalizeScriptID(scriptID)

	data := map[string]any{
		"entity_id": entityID,
	}

	if variables, ok := args["variables"].(map[string]any); ok {
		for k, v := range variables {
			data[k] = v
		}
	}

	if _, err := client.CallService(ctx, "script", "turn_on", data); err != nil {
		return errorResult(fmt.Sprintf("Error executing script: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Script '%s' executed successfully", scriptID)), nil
}

//nolint:funlen // Patch handler
func (h *ScriptHandlers) handlePatch(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return errorResult("script_id is required for patch action"), nil
	}

	ops, errResult := parseOperations(args)
	if errResult != nil {
		return errResult, nil
	}

	entityID, configID := normalizeScriptID(scriptID)

	current, err := client.GetScript(ctx, entityID)
	if err != nil {
		current, err = h.findScriptByID(ctx, client, scriptID)
		if err != nil {
			return errorResult(fmt.Sprintf("error getting script: %v", err)), nil
		}
	}

	if current.Config == nil {
		return errorResult(fmt.Sprintf("script '%s' has no configuration to patch", scriptID)), nil
	}

	configMap, err := configToMap(current.Config)
	if err != nil {
		return errorResult(fmt.Sprintf("error processing script config: %v", err)), nil
	}

	patchedMap, patchErr := applyPatchWithSemantics(configMap, ops)
	if patchErr != nil {
		return errorResult(fmt.Sprintf("error applying patch: %v", patchErr)), nil
	}

	if dryRun, _ := args["dry_run"].(bool); dryRun {
		return dryRunPatchResult(patchedMap, "script", scriptID, len(ops))
	}

	// current.EntityID reflects the entity actually resolved above (findScriptByID may have
	// matched a different entity than a bare entityID guess).
	checkEntityID := resolveWriteCheckEntityID(entityID, current.EntityID)

	return applyPatchedScriptWrite(ctx, client, scriptID, checkEntityID, configID, configMap, patchedMap, len(ops))
}

// applyPatchedScriptWrite writes the patched config to HA and returns the success result. It
// skips the write entirely when configMap and patchedMap are deep-equal — otherwise every no-op
// patch would trigger a needless script.reload — and refuses to write YAML-defined scripts,
// which the config API would otherwise silently duplicate into an orphan entity (#122). Mirrors
// applyPatchedAutomationWrite in automations.go.
func applyPatchedScriptWrite(
	ctx context.Context,
	client homeassistant.Client,
	scriptID, entityID, configID string,
	configMap, patchedMap map[string]any,
	numOps int,
) (*mcp.ToolsCallResult, error) {
	if reflect.DeepEqual(configMap, patchedMap) {
		return successResult(fmt.Sprintf("Script '%s': no changes detected, skipping write (reload avoided)", scriptID)), nil
	}

	if guardErr := yamlWriteGuardError(ctx, client, "script", "patch", scriptID, entityID); guardErr != nil {
		return guardErr, nil
	}

	var newConfig homeassistant.ScriptConfig
	if err := mapToStruct(patchedMap, &newConfig); err != nil {
		return errorResult(fmt.Sprintf("error parsing patched config: %v", err)), nil
	}

	if err := client.UpdateScript(ctx, configID, newConfig); err != nil {
		msg := fmt.Sprintf("error saving patched script: %v", err)
		return errorResult(enrichConfigError(msg, err, scriptErrorHints)), nil
	}

	successMsg := fmt.Sprintf("Script '%s' patched successfully (%d operations applied)", scriptID, numOps)
	if !reloadDomain(ctx, client, "script") {
		successMsg += scriptReloadFailedWarning
	}
	return successResult(successMsg), nil
}

// =============================================================================
// call_service Handler (separate tool)
// =============================================================================

func (h *ScriptHandlers) handleCallService(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	domain, ok := args["domain"].(string)
	if !ok || domain == "" {
		return errorResult("domain is required"), nil
	}

	service, ok := args["service"].(string)
	if !ok || service == "" {
		return errorResult("service is required"), nil
	}

	var data map[string]any
	if d, ok := args["data"].(map[string]any); ok {
		data = d
	}

	// Extract targets and snapshot state before calling the service
	targets := extractEntityTargets(data)
	snapshots := snapshotEntities(ctx, client, targets)

	_, err := client.CallService(ctx, domain, service, data)
	if err != nil {
		return errorResult(fmt.Sprintf("Error calling service: %v", err)), nil
	}

	// Wait for state changes and build diff summary
	diffs, allChanged := waitForStateChanges(ctx, client, snapshots)
	stateSummary := formatStateDiffs(diffs, !allChanged)

	format := formatter.ParseFormat(getStringArg(args, "format"))
	f := formatter.New(format)

	output, err := f.FormatServiceSuccess(ctx, domain, service, targets, data)
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting result: %v", err)), nil
	}

	return successResult(output + stateSummary), nil
}

// extractEntityTargets extracts entity IDs from the data map.
// Handles: string, []any, []string formats for data["entity_id"].
func extractEntityTargets(data map[string]any) []string {
	if data == nil {
		return []string{}
	}

	entityIDVal, ok := data["entity_id"]
	if !ok {
		return []string{}
	}

	// Handle string
	if entityID, ok := entityIDVal.(string); ok {
		if entityID != "" {
			return []string{entityID}
		}
		return []string{}
	}

	// Handle []string
	if entityIDs, ok := entityIDVal.([]string); ok {
		result := make([]string, 0, len(entityIDs))
		for _, id := range entityIDs {
			if id != "" {
				result = append(result, id)
			}
		}
		return result
	}

	// Handle []any
	if entityIDs, ok := entityIDVal.([]any); ok {
		result := make([]string, 0, len(entityIDs))
		for _, id := range entityIDs {
			if strID, ok := id.(string); ok && strID != "" {
				result = append(result, strID)
			}
		}
		return result
	}

	return []string{}
}

//nolint:funlen // Patch handler
