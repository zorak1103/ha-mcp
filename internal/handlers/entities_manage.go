package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Entity action constants.
const (
	entityActionGet    = "get"
	entityActionUpdate = "update"
	entityActionDelete = "delete"

	formatNatural = "natural"
	noneValue     = "none"
)

// EntityManageHandlers provides handlers for entity registry management.
type EntityManageHandlers struct{}

// NewEntityManageHandlers creates a new EntityManageHandlers instance.
func NewEntityManageHandlers() *EntityManageHandlers {
	return &EntityManageHandlers{}
}

// RegisterTools registers the manage_entity tool with the registry.
func (h *EntityManageHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageEntityTool(), h.handleManageEntity)
}

// =============================================================================
// Tool Definition
// =============================================================================

func (h *EntityManageHandlers) manageEntityTool() mcp.Tool {
	return mcp.Tool{
		Name: "manage_entity",
		Description: `Manage Home Assistant entity registry entries - get, update, or delete.

Actions:
- get: Get entity registry details (requires entity_id)
- update: Update entity registry entry (requires entity_id and at least one field to update)
- delete: Delete entity from registry (requires entity_id)

Safe fields that can be updated:
- name: Custom display name. Precedence: registry name (this field) > automation alias > auto-generated slug.
  Empty string removes the registry override — HA falls back to the auto-slug, NOT the automation alias.
  When renaming via new_entity_id, always set name= in the same call to avoid the slug flashing as the display name.
- icon: Custom icon like 'mdi:lightbulb' (empty string removes override)
- area_id: Area assignment (empty string removes)
- disabled_by: 'user' to disable, 'none' to enable
- hidden_by: 'user' to hide, 'none' to show
- labels: Array of label strings; use label_mode to control merge behavior
- aliases: Array of alternative name strings; use alias_mode to control merge behavior
- label_mode: 'add' (default, append), 'remove' (subtract), 'replace' (full replacement)
- alias_mode: 'add' (default, append), 'remove' (subtract), 'replace' (full replacement)
- new_entity_id: Rename entity ID (must be valid format 'domain.object_id')`,
		InputSchema: h.buildEntityManageSchema(),
	}
}

func (h *EntityManageHandlers) buildEntityManageSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type:        "object",
		Description: "Entity registry management operation",
		Properties: map[string]mcp.JSONSchema{
			"action": {
				Type:        "string",
				Description: "Operation to perform: get, update, delete",
				Enum:        []string{"get", "update", "delete"},
			},
			attrEntityID: {
				Type:        "string",
				Description: "Entity ID (e.g., 'light.living_room'). Required for all actions.",
			},
			"name": {
				Type:        "string",
				Description: "Custom display name (update only). Takes highest precedence: registry name > automation alias > auto-slug. Empty string removes override — falls back to auto-slug, not alias. Set alongside new_entity_id to avoid slug flashing as display name.",
			},
			"icon": {
				Type:        "string",
				Description: "Custom icon (update only, e.g., 'mdi:lightbulb', empty string removes override)",
			},
			"area_id": {
				Type:        "string",
				Description: "Area ID (update only, empty string removes assignment)",
			},
			"disabled_by": {
				Type:        "string",
				Description: "Disable status (update only): 'user' to disable, 'none' to enable",
				Enum:        []string{"user", "none"},
			},
			"hidden_by": {
				Type:        "string",
				Description: "Hidden status (update only): 'user' to hide, 'none' to show",
				Enum:        []string{"user", "none"},
			},
			"labels": {
				Type:        "array",
				Description: "Labels array (update only); use label_mode to control merge behavior",
				Items:       &mcp.JSONSchema{Type: "string"},
			},
			"aliases": {
				Type:        "array",
				Description: "Aliases array (update only); use alias_mode to control merge behavior",
				Items:       &mcp.JSONSchema{Type: "string"},
			},
			"label_mode": arrayModeSchema("labels"),
			"alias_mode": arrayModeSchema("aliases"),
			"new_entity_id": {
				Type:        "string",
				Description: "New entity ID for renaming (update only, must be in format 'domain.object_id')",
			},
			"format": {
				Type:        "string",
				Description: "Output format: 'natural' (human-readable, default) or 'json' (structured)",
				Enum:        []string{"natural", "json"},
			},
		},
		Required: []string{"action"},
	}
}

// =============================================================================
// Handler Implementation
// =============================================================================

func (h *EntityManageHandlers) handleManageEntity(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return errorResult("action is required and must be a string (get, update, or delete)"), nil
	}

	format := formatNatural
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	switch action {
	case entityActionGet:
		return h.handleGetEntity(ctx, client, args, format)
	case entityActionUpdate:
		return h.handleUpdateEntity(ctx, client, args, format)
	case entityActionDelete:
		return h.handleDeleteEntity(ctx, client, args, format)
	default:
		return errorResult(fmt.Sprintf("unsupported action '%s'. Valid actions: get, update, delete", action)), nil
	}
}

func (h *EntityManageHandlers) handleGetEntity(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	entityID, ok := args[attrEntityID].(string)
	if !ok || entityID == "" {
		return errorResult("entity_id is required for get action"), nil
	}

	// Get entity registry
	registry, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity registry: %w", err)
	}

	// Find entity
	for _, entry := range registry {
		if entry.EntityID == entityID {
			if format == formatJSON {
				return h.formatEntityJSON(&entry)
			}
			return h.formatEntityNatural(&entry), nil
		}
	}

	return errorResult(fmt.Sprintf("entity '%s' not found in registry", entityID)), nil
}

func (h *EntityManageHandlers) handleUpdateEntity(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	entityID, ok := args[attrEntityID].(string)
	if !ok || entityID == "" {
		return errorResult("entity_id is required for update action"), nil
	}

	// Validate new_entity_id if provided
	if newEntityID, ok := args["new_entity_id"].(string); ok && newEntityID != "" {
		if err := ValidateEntityID(newEntityID); err != nil {
			return errorResult(fmt.Sprintf("invalid new_entity_id: %v", err)), nil
		}
	}

	oldEntityID := entityID
	config, hasFields := h.buildEntityUpdateConfig(args)

	labelMode := getArrayMode(args, "label_mode")
	aliasMode := getArrayMode(args, "alias_mode")
	labels, hasLabels := getStringSlice(args, "labels")
	aliases, hasAliases := getStringSlice(args, "aliases")

	var currentEntry homeassistant.EntityRegistryEntry
	if hasLabels || hasAliases {
		entry, fetchErr := h.fetchEntityForMerge(ctx, client, entityID, labelMode, aliasMode, hasLabels, hasAliases)
		if fetchErr != nil {
			return errorResult(fetchErr.Error()), nil
		}
		currentEntry = *entry
	}

	if hasLabels {
		hasFields = true
		config.Labels = applyArrayMode(currentEntry.Labels, labels, labelMode)
	}

	if hasAliases {
		hasFields = true
		config.Aliases = applyArrayMode(currentEntry.Aliases, aliases, aliasMode)
	}

	if !hasFields {
		return errorResult("at least one field must be provided for update (name, icon, area_id, disabled_by, hidden_by, labels, aliases, new_entity_id)"), nil
	}

	// Update entity
	updated, err := client.UpdateEntityRegistryEntry(ctx, entityID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to update entity: %w", err)
	}

	if format == formatJSON {
		return h.formatEntityJSON(updated)
	}
	return h.formatEntityNaturalWithSuccess(updated, oldEntityID), nil
}

func (h *EntityManageHandlers) handleDeleteEntity(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	entityID, ok := args[attrEntityID].(string)
	if !ok || entityID == "" {
		return errorResult("entity_id is required for delete action"), nil
	}

	if err := client.RemoveEntityRegistryEntry(ctx, entityID); err != nil {
		return nil, fmt.Errorf("failed to delete entity: %w", err)
	}

	if format == formatJSON {
		data, err := json.Marshal(map[string]any{attrEntityID: entityID, "deleted": true})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
		return textResult(string(data)), nil
	}
	return textResult(fmt.Sprintf("Entity '%s' deleted from registry.", entityID)), nil
}

// fetchEntityForMerge fetches the entity registry entry for label/alias merge.
// Skips the registry call when both label and alias modes are replace (no current
// values needed). Returns a not-found error for add/remove mode when the entity
// is absent from the registry. Never returns a nil pointer when err is nil.
func (h *EntityManageHandlers) fetchEntityForMerge(
	ctx context.Context,
	client homeassistant.Client,
	entityID string,
	labelMode, aliasMode string,
	hasLabels, hasAliases bool,
) (*homeassistant.EntityRegistryEntry, error) {
	needsFetch := (hasLabels && labelMode != arrayModeReplace) ||
		(hasAliases && aliasMode != arrayModeReplace)
	if !needsFetch {
		return &homeassistant.EntityRegistryEntry{}, nil
	}
	registry, err := client.GetEntityRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity registry: %w", err)
	}
	for i := range registry {
		if registry[i].EntityID == entityID {
			return &registry[i], nil
		}
	}
	return nil, fmt.Errorf("entity '%s' not found in registry", entityID)
}

// =============================================================================
// Helper Functions
// =============================================================================

func (h *EntityManageHandlers) buildEntityUpdateConfig(args map[string]any) (homeassistant.EntityRegistryUpdateConfig, bool) {
	config := homeassistant.EntityRegistryUpdateConfig{}
	hasFields := false

	if name, ok := args["name"].(string); ok {
		config.Name = &name
		hasFields = true
	}

	if icon, ok := args["icon"].(string); ok {
		config.Icon = &icon
		hasFields = true
	}

	if areaID, ok := args["area_id"].(string); ok {
		config.AreaID = &areaID
		hasFields = true
	}

	if disabledBy, ok := args["disabled_by"].(string); ok {
		// Map "none" to empty string for HA API
		if disabledBy == noneValue {
			disabledBy = ""
		}
		config.DisabledBy = &disabledBy
		hasFields = true
	}

	if hiddenBy, ok := args["hidden_by"].(string); ok {
		// Map "none" to empty string for HA API
		if hiddenBy == noneValue {
			hiddenBy = ""
		}
		config.HiddenBy = &hiddenBy
		hasFields = true
	}

	if newEntityID, ok := args["new_entity_id"].(string); ok && newEntityID != "" {
		config.NewEntityID = &newEntityID
		hasFields = true
	}

	return config, hasFields
}

// =============================================================================
// Formatters
// =============================================================================

func (h *EntityManageHandlers) formatEntityNatural(entry *homeassistant.EntityRegistryEntry) *mcp.ToolsCallResult {
	var parts []string
	parts = append(parts, fmt.Sprintf("Entity ID: %s", entry.EntityID))

	if entry.Name != "" {
		parts = append(parts, fmt.Sprintf("Name: %s", entry.Name))
	}

	if entry.Platform != "" {
		parts = append(parts, fmt.Sprintf("Platform: %s", entry.Platform))
	}

	if entry.AreaID != "" {
		parts = append(parts, fmt.Sprintf("Area ID: %s", entry.AreaID))
	}

	if entry.Icon != "" {
		parts = append(parts, fmt.Sprintf("Icon: %s", entry.Icon))
	}

	if entry.DisabledBy != "" {
		parts = append(parts, fmt.Sprintf("Disabled by: %s", entry.DisabledBy))
	}

	if entry.HiddenBy != "" {
		parts = append(parts, fmt.Sprintf("Hidden by: %s", entry.HiddenBy))
	}

	if len(entry.Labels) > 0 {
		parts = append(parts, fmt.Sprintf("Labels: %s", strings.Join(entry.Labels, ", ")))
	}

	if len(entry.Aliases) > 0 {
		parts = append(parts, fmt.Sprintf("Aliases: %s", strings.Join(entry.Aliases, ", ")))
	}

	if entry.DeviceID != "" {
		parts = append(parts, fmt.Sprintf("Device ID: %s", entry.DeviceID))
	}

	if entry.ConfigEntryID != "" {
		parts = append(parts, fmt.Sprintf("Config Entry ID: %s", entry.ConfigEntryID))
	}

	return textResult(strings.Join(parts, "\n"))
}

func (h *EntityManageHandlers) formatEntityNaturalWithSuccess(entry *homeassistant.EntityRegistryEntry, oldEntityID string) *mcp.ToolsCallResult {
	details := h.formatEntityNatural(entry).Content[0].Text

	// Show rename info if entity ID changed
	if oldEntityID != entry.EntityID {
		return textResult(fmt.Sprintf("Entity renamed from %s to %s.\n\n%s", oldEntityID, entry.EntityID, details))
	}

	return textResult(fmt.Sprintf("Entity '%s' updated successfully.\n\n%s", entry.EntityID, details))
}

func (h *EntityManageHandlers) formatEntityJSON(entry *homeassistant.EntityRegistryEntry) (*mcp.ToolsCallResult, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entity: %w", err)
	}
	return textResult(string(data)), nil
}
