package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Action constants for manage_update tool.
const (
	updateActionList         = "list"
	updateActionReleaseNotes = "release_notes"
	updateActionInstall      = "install"
	updateActionSkip         = "skip"
)

const updateDomain = "update"

// UpdateHandlers provides handlers for update management operations.
type UpdateHandlers struct{}

// NewUpdateHandlers creates a new update handlers instance.
func NewUpdateHandlers() *UpdateHandlers {
	return &UpdateHandlers{}
}

// RegisterUpdateTools registers update-related tools with the MCP registry.
func RegisterUpdateTools(registry *mcp.Registry) {
	handler := NewUpdateHandlers()

	registry.RegisterTool(mcp.Tool{
		Name:        "manage_update",
		Description: "Manage Home Assistant updates. Supports list (view available updates), release_notes (view release details), install (apply updates), and skip (skip version).",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Action to perform: 'list', 'release_notes', 'install', or 'skip'.",
					Enum:        []string{updateActionList, updateActionReleaseNotes, updateActionInstall, updateActionSkip},
				},
				attrEntityID: {
					Type:        "string",
					Description: "Update entity ID (required for release_notes, install, skip actions, e.g., 'update.hass_os').",
				},
				"pending_only": {
					Type:        "boolean",
					Description: "Filter to show only pending updates (for 'list' action).",
					Default:     false,
				},
				"version": {
					Type:        "string",
					Description: "Specific version to install (optional for 'install' action).",
				},
				"backup": {
					Type:        "boolean",
					Description: "Create backup before installing (default: true).",
					Default:     true,
				},
				"format": {
					Type:        "string",
					Description: "Output format: 'natural' (default, human-readable) or 'json' (structured JSON).",
					Enum:        []string{"natural", "json"},
					Default:     "natural",
				},
			},
			Required: []string{"action"},
		},
	}, handler.HandleManageUpdate)
}

// HandleManageUpdate handles the manage_update tool invocation.
func (h *UpdateHandlers) HandleManageUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	// Extract action
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action parameter is required and must be 'list', 'release_notes', 'install', or 'skip'"), nil
	}

	// Extract format (default: natural)
	format, _ := args["format"].(string)
	if format == "" {
		format = formatNatural
	}

	// Route to action handler
	switch action {
	case updateActionList:
		return h.handleListUpdates(ctx, client, args, format)
	case updateActionReleaseNotes:
		return h.handleReleaseNotes(ctx, client, args, format)
	case updateActionInstall:
		return h.handleInstallUpdate(ctx, client, args, format)
	case updateActionSkip:
		return h.handleSkipUpdate(ctx, client, args, format)
	default:
		return errorResult(fmt.Sprintf("invalid action %q, must be one of: list, release_notes, install, skip", action)), nil
	}
}

// handleListUpdates lists all updates or only pending updates.
func (h *UpdateHandlers) handleListUpdates(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	// Get all states
	states, err := client.GetStates(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get states: %v", err)), nil
	}

	// Filter for update domain
	var updates []homeassistant.Entity
	for _, state := range states {
		if strings.HasPrefix(state.EntityID, updateDomain+".") {
			updates = append(updates, state)
		}
	}

	// Apply pending_only filter if requested
	pendingOnly, _ := args["pending_only"].(bool)
	if pendingOnly {
		var pending []homeassistant.Entity
		for _, update := range updates {
			if update.State == "on" {
				pending = append(pending, update)
			}
		}
		updates = pending
	}

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(updates, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal updates: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format
	return successResult(h.formatUpdatesNatural(updates)), nil
}

// handleReleaseNotes retrieves release notes for an update.
func (h *UpdateHandlers) handleReleaseNotes(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	// Validate entity_id
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for release_notes action"), nil
	}

	// Build command data
	data := map[string]any{
		attrEntityID: entityID,
	}

	// Call update/release_notes WebSocket command
	response, err := client.SendHACSCommand(ctx, "update/release_notes", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get release notes: %v", err)), nil
	}

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal release notes: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format - extract release_notes field if present
	if respMap, ok := response.(map[string]any); ok {
		if notes := getMapString(respMap, "release_notes", ""); notes != "" {
			return successResult(fmt.Sprintf("Release Notes for %s:\n\n%s", entityID, notes)), nil
		}
	}

	return successResult(fmt.Sprintf("Release notes: %v", response)), nil
}

// handleInstallUpdate installs an update.
func (h *UpdateHandlers) handleInstallUpdate(ctx context.Context, client homeassistant.Client, args map[string]any, _ string) (*mcp.ToolsCallResult, error) {
	// Validate entity_id
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for install action"), nil
	}

	// Build service data
	data := map[string]any{
		attrEntityID: entityID,
	}

	// Add optional parameters
	if version, ok := args["version"].(string); ok && version != "" {
		data["version"] = version
	}

	// Handle backup parameter (default: true)
	backup := true
	if b, ok := args["backup"].(bool); ok {
		backup = b
	}
	data["backup"] = backup

	// Call update.install service
	_, err := client.CallService(ctx, updateDomain, "install", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to install update: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Update installation started for %s", entityID)), nil
}

// handleSkipUpdate skips an update version.
func (h *UpdateHandlers) handleSkipUpdate(ctx context.Context, client homeassistant.Client, args map[string]any, _ string) (*mcp.ToolsCallResult, error) {
	// Validate entity_id
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for skip action"), nil
	}

	// Build service data
	data := map[string]any{
		attrEntityID: entityID,
	}

	// Call update.skip service
	_, err := client.CallService(ctx, updateDomain, "skip", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to skip update: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Update skipped for %s", entityID)), nil
}

// formatUpdatesNatural formats updates in natural language.
func (h *UpdateHandlers) formatUpdatesNatural(updates []homeassistant.Entity) string {
	if len(updates) == 0 {
		return "No updates found."
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d update(s):", len(updates)))

	for i, update := range updates {
		name := getMapString(update.Attributes, "friendly_name", update.EntityID)
		installedVer := getMapString(update.Attributes, "installed_version", "unknown")
		latestVer := getMapString(update.Attributes, "latest_version", "unknown")

		parts = append(parts,
			fmt.Sprintf("\n%d. %s", i+1, name),
			fmt.Sprintf("   Current: %s", installedVer),
			fmt.Sprintf("   Latest: %s", latestVer),
		)

		if update.State == "on" {
			parts = append(parts, "   Status: Update available")
		} else {
			parts = append(parts, "   Status: Up to date")
		}

		if summary := getMapString(update.Attributes, "release_summary", ""); summary != "" {
			parts = append(parts, fmt.Sprintf("   Summary: %s", summary))
		}
	}

	return strings.Join(parts, "\n")
}
