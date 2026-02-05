// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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
)

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
- execute: Execute a script (requires script_id, optional variables)`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Script management operation",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Operation to perform: list, get, create, update, delete, execute",
					Enum:        []string{"list", "get", "create", "update", "delete", "execute"},
				},
				"script_id": {
					Type:        "string",
					Description: "Script ID without 'script.' prefix (required for get/create/update/delete/execute)",
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
			},
			Required: []string{"action"},
		},
	}
}

func (h *ScriptHandlers) callServiceTool() mcp.Tool {
	return mcp.Tool{
		Name:        "call_service",
		Description: "Call any Home Assistant service",
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
					Description: "Service data including entity_id and other parameters",
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
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, delete, or execute)", action)), nil
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
		return errorResult(fmt.Sprintf("Error getting script: %v", err)), nil
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
	if !ok || len(sequence) == 0 {
		return errorResult("sequence is required for create action and must be a non-empty array"), nil
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
	if icon, ok := args["icon"].(string); ok {
		config.Icon = icon
	}
	if fields, ok := args["fields"].(map[string]any); ok {
		config.Fields = fields
	}

	if err := client.CreateScript(ctx, scriptID, config); err != nil {
		return errorResult(fmt.Sprintf("Error creating script: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Script '%s' created successfully", scriptID)), nil
}

func (h *ScriptHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return errorResult("script_id is required for update action"), nil
	}

	// Get current script state to preserve existing values
	entityID := "script." + scriptID
	current, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting current script: %v", err)), nil
	}

	// Build config from current state and args
	config := homeassistant.ScriptConfig{}

	// Get current values from attributes
	if alias, ok := current.Attributes["friendly_name"].(string); ok {
		config.Alias = alias
	}

	// Override with new values from args
	if alias, ok := args["alias"].(string); ok {
		config.Alias = alias
	}
	if description, ok := args["description"].(string); ok {
		config.Description = description
	}
	if mode, ok := args["mode"].(string); ok {
		config.Mode = mode
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

	if err := client.UpdateScript(ctx, scriptID, config); err != nil {
		return errorResult(fmt.Sprintf("Error updating script: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Script '%s' updated successfully", scriptID)), nil
}

func (h *ScriptHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return errorResult("script_id is required for delete action"), nil
	}

	if err := client.DeleteScript(ctx, scriptID); err != nil {
		return errorResult(fmt.Sprintf("Error deleting script: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Script '%s' deleted successfully", scriptID)), nil
}

func (h *ScriptHandlers) handleExecute(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return errorResult("script_id is required for execute action"), nil
	}

	data := map[string]any{
		"entity_id": "script." + scriptID,
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

	entities, err := client.CallService(ctx, domain, service, data)
	if err != nil {
		return errorResult(fmt.Sprintf("Error calling service: %v", err)), nil
	}

	result := map[string]any{
		"success":           true,
		"affected_entities": len(entities),
	}

	if len(entities) > 0 {
		entityIDs := make([]string, 0, len(entities))
		for _, e := range entities {
			entityIDs = append(entityIDs, e.EntityID)
		}
		result["entity_ids"] = entityIDs
	}

	// Try to marshal result to JSON, fall back to simple message if it fails
	if jsonBytes, marshalErr := json.MarshalIndent(result, "", "  "); marshalErr == nil {
		return successResult(string(jsonBytes)), nil
	}

	// Fallback if JSON marshaling fails
	return successResult(fmt.Sprintf("Service called successfully, affected %d entities", len(entities))), nil
}
