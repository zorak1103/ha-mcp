// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// ScriptHandlers provides handlers for script-related MCP tools.
type ScriptHandlers struct{}

// NewScriptHandlers creates a new ScriptHandlers instance.
func NewScriptHandlers() *ScriptHandlers {
	return &ScriptHandlers{}
}

// Tools returns all script-related tool definitions.
func (h *ScriptHandlers) Tools() []mcp.Tool {
	return []mcp.Tool{
		h.listScriptsTool(),
		h.getScriptTool(),
		h.createScriptTool(),
		h.updateScriptTool(),
		h.deleteScriptTool(),
		h.executeScriptTool(),
		h.callServiceTool(),
	}
}

// Register registers all script-related tools with the registry.
func (h *ScriptHandlers) Register(registry *mcp.Registry) {
	registry.RegisterTool(h.listScriptsTool(), h.HandleListScripts)
	registry.RegisterTool(h.getScriptTool(), h.HandleGetScript)
	registry.RegisterTool(h.createScriptTool(), h.HandleCreateScript)
	registry.RegisterTool(h.updateScriptTool(), h.HandleUpdateScript)
	registry.RegisterTool(h.deleteScriptTool(), h.HandleDeleteScript)
	registry.RegisterTool(h.executeScriptTool(), h.HandleExecuteScript)
	registry.RegisterTool(h.callServiceTool(), h.HandleCallService)
}

func (h *ScriptHandlers) listScriptsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_scripts",
		Description: "List all scripts in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:       "object",
			Properties: map[string]mcp.JSONSchema{},
		},
	}
}

func (h *ScriptHandlers) getScriptTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_script",
		Description: "Get details of a specific script",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"script_id": {
					Type:        "string",
					Description: "The script ID (without 'script.' prefix)",
				},
			},
			Required: []string{"script_id"},
		},
	}
}

func (h *ScriptHandlers) createScriptTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_script",
		Description: "Create a new script in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"script_id": {
					Type:        "string",
					Description: "Unique ID for the script (lowercase, underscores allowed)",
				},
				"alias": {
					Type:        "string",
					Description: "Friendly name for the script",
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
					Description: "Array of actions to execute",
					Items: &mcp.JSONSchema{
						Type:        "object",
						Description: "Action object",
					},
				},
				"fields": {
					Type:        "object",
					Description: "Input fields for the script",
				},
			},
			Required: []string{"script_id", "alias", "sequence"},
		},
	}
}

func (h *ScriptHandlers) updateScriptTool() mcp.Tool {
	return mcp.Tool{
		Name:        "update_script",
		Description: "Update an existing script in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"script_id": {
					Type:        "string",
					Description: "The script ID to update",
				},
				"alias": {
					Type:        "string",
					Description: "New friendly name for the script",
				},
				"description": {
					Type:        "string",
					Description: "New description",
				},
				"mode": {
					Type:        "string",
					Description: "Script mode: single, restart, queued, parallel",
					Enum:        []string{"single", "restart", "queued", "parallel"},
				},
				"icon": {
					Type:        "string",
					Description: "New icon for the script",
				},
				"sequence": {
					Type:        "array",
					Description: "New array of actions to execute",
					Items: &mcp.JSONSchema{
						Type:        "object",
						Description: "Action object",
					},
				},
				"fields": {
					Type:        "object",
					Description: "New input fields for the script",
				},
			},
			Required: []string{"script_id"},
		},
	}
}

func (h *ScriptHandlers) deleteScriptTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_script",
		Description: "Delete a script from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"script_id": {
					Type:        "string",
					Description: "The script ID to delete",
				},
			},
			Required: []string{"script_id"},
		},
	}
}

func (h *ScriptHandlers) executeScriptTool() mcp.Tool {
	return mcp.Tool{
		Name:        "execute_script",
		Description: "Execute a script in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"script_id": {
					Type:        "string",
					Description: "The script ID to execute (without 'script.' prefix)",
				},
				"variables": {
					Type:        "object",
					Description: "Variables to pass to the script",
				},
			},
			Required: []string{"script_id"},
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

// HandleListScripts handles the list_scripts tool call.
func (h *ScriptHandlers) HandleListScripts(ctx context.Context, client homeassistant.Client, _ map[string]any) (*mcp.ToolsCallResult, error) {
	scripts, err := client.ListScripts(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error listing scripts: %v", err))},
			IsError: true,
		}, nil
	}

	type scriptInfo struct {
		EntityID      string `json:"entity_id"`
		State         string `json:"state"`
		FriendlyName  string `json:"friendly_name,omitempty"`
		LastTriggered string `json:"last_triggered,omitempty"`
	}

	result := make([]scriptInfo, 0, len(scripts))
	for _, s := range scripts {
		info := scriptInfo{
			EntityID: s.EntityID,
			State:    s.State,
		}
		if name, ok := s.Attributes["friendly_name"].(string); ok {
			info.FriendlyName = name
		}
		if lastTriggered, ok := s.Attributes["last_triggered"].(string); ok {
			info.LastTriggered = lastTriggered
		}
		result = append(result, info)
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error marshaling scripts: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(jsonBytes))},
	}, nil
}

// HandleGetScript handles the get_script tool call.
func (h *ScriptHandlers) HandleGetScript(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("script_id is required")},
			IsError: true,
		}, nil
	}

	entityID := "script." + scriptID
	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting script: %v", err))},
			IsError: true,
		}, nil
	}

	jsonBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error marshaling script: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(jsonBytes))},
	}, nil
}

// HandleCreateScript handles the create_script tool call.
func (h *ScriptHandlers) HandleCreateScript(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("script_id is required")},
			IsError: true,
		}, nil
	}

	alias, ok := args["alias"].(string)
	if !ok || alias == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("alias is required")},
			IsError: true,
		}, nil
	}

	sequence, ok := args["sequence"].([]any)
	if !ok || len(sequence) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("sequence is required and must be a non-empty array")},
			IsError: true,
		}, nil
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
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating script: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Script '%s' created successfully", scriptID))},
	}, nil
}

// HandleUpdateScript handles the update_script tool call.
func (h *ScriptHandlers) HandleUpdateScript(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("script_id is required")},
			IsError: true,
		}, nil
	}

	// Get current script state to preserve existing values
	entityID := "script." + scriptID
	current, err := client.GetState(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting current script: %v", err))},
			IsError: true,
		}, nil
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
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error updating script: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Script '%s' updated successfully", scriptID))},
	}, nil
}

// HandleDeleteScript handles the delete_script tool call.
func (h *ScriptHandlers) HandleDeleteScript(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("script_id is required")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteScript(ctx, scriptID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting script: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Script '%s' deleted successfully", scriptID))},
	}, nil
}

// HandleExecuteScript handles the execute_script tool call.
func (h *ScriptHandlers) HandleExecuteScript(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scriptID, ok := args["script_id"].(string)
	if !ok || scriptID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("script_id is required")},
			IsError: true,
		}, nil
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
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error executing script: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Script '%s' executed successfully", scriptID))},
	}, nil
}

// HandleCallService handles the call_service tool call.
func (h *ScriptHandlers) HandleCallService(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	domain, ok := args["domain"].(string)
	if !ok || domain == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("domain is required")},
			IsError: true,
		}, nil
	}

	service, ok := args["service"].(string)
	if !ok || service == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("service is required")},
			IsError: true,
		}, nil
	}

	var data map[string]any
	if d, ok := args["data"].(map[string]any); ok {
		data = d
	}

	entities, err := client.CallService(ctx, domain, service, data)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error calling service: %v", err))},
			IsError: true,
		}, nil
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
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(string(jsonBytes))},
		}, nil
	}

	// Fallback if JSON marshaling fails
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Service called successfully, affected %d entities", len(entities)))},
	}, nil
}

// RegisterScriptTools registers all script-related tools with the registry.
//
// Deprecated: Use NewScriptHandlers().Register(registry) instead.
func RegisterScriptTools(registry *mcp.Registry) {
	h := NewScriptHandlers()
	h.Register(registry)
}
