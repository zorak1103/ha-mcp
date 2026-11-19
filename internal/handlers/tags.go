// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Tag action constants.
const (
	tagActionList   = "list"
	tagActionGet    = "get"
	tagActionCreate = "create"
	tagActionUpdate = "update"
	tagActionDelete = "delete"
)

// TagHandlers provides handlers for tag-related MCP tools.
type TagHandlers struct{}

// NewTagHandlers creates a new TagHandlers instance.
func NewTagHandlers() *TagHandlers {
	return &TagHandlers{}
}

// RegisterTools registers the consolidated manage_tag tool with the registry.
func (h *TagHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageTagTool(), h.handleManageTag)
}

// =============================================================================
// Tool Definition
// =============================================================================

func (h *TagHandlers) manageTagTool() mcp.Tool {
	schema := h.buildTagSchema()
	return mcp.Tool{
		Name: "manage_tag",
		Description: `Manage Home Assistant tags - list, get, create, update, or delete.

Tags represent NFC tags or QR codes that can trigger automations when scanned.

Actions:
- list: List all tags (optional filters: name_contains)
- get: Get details of a specific tag (requires tag_id)
- create: Create a new tag (requires name; tag_id optional for deterministic ID)
- update: Update an existing tag (requires tag_id)
- delete: Delete a tag (requires tag_id)`,
		InputSchema: schema,
	}
}

func (h *TagHandlers) buildTagSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type:        "object",
		Description: "Tag management operation",
		Properties: map[string]mcp.JSONSchema{
			"action": {
				Type:        "string",
				Description: "Operation to perform: list, get, create, update, delete",
				Enum:        []string{"list", "get", "create", "update", "delete"},
			},
			"tag_id": {
				Type:        "string",
				Description: "Tag identifier or name (required for get/update/delete; optional for create to set deterministic ID). Accepts exact tag_id or case-insensitive name search.",
			},
			"name": {
				Type:        "string",
				Description: "Tag name (required for create, optional for update)",
			},
			"description": {
				Type:        "string",
				Description: "Tag description",
			},
			"name_contains": {
				Type:        "string",
				Description: "Filter by tag name containing this string (for list action, case-insensitive)",
			},
			"format": {
				Type:        "string",
				Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				Enum:        []string{"natural", "json"},
			},
		},
		Required: []string{"action"},
	}
}

// =============================================================================
// Handler: manage_tag
// =============================================================================

func (h *TagHandlers) handleManageTag(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case tagActionList:
		return h.handleList(ctx, client, args)
	case tagActionGet:
		return h.handleGet(ctx, client, args)
	case tagActionCreate:
		return h.handleCreate(ctx, client, args)
	case tagActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case tagActionDelete:
		return h.handleDelete(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, or delete)", action)), nil
	}
}

func (h *TagHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	nameContains, errMsg := stringArgValue("name_contains", args["name_contains"])
	if errMsg != "" {
		return errorResult(errMsg), nil
	}

	tags, err := client.GetTags(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error listing tags: %v", err)), nil
	}

	// Apply name filter if provided
	if nameContains != "" {
		tags = h.filterTagsByName(tags, nameContains)
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatListJSON(tags)
	}
	return h.formatListNatural(tags)
}

func (h *TagHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	tagID, _ := args["tag_id"].(string)
	if tagID == "" {
		return errorResult("tag_id is required for get action"), nil
	}

	tags, err := client.GetTags(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error getting tags: %v", err)), nil
	}

	tag, findErr := h.findTagByInput(tags, tagID)
	if findErr != nil {
		return errorResult(findErr.Error()), nil
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatGetJSON(tag)
	}
	return h.formatGetNatural(tag)
}

func (h *TagHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return errorResult("name is required for create action"), nil
	}

	config := h.buildTagConfig(args)
	config.Name = name

	tag, err := client.CreateTag(ctx, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error creating tag: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Tag '%s' created successfully with ID: %s", tag.Name, tag.TagID)), nil
}

func (h *TagHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	tagID, _ := args["tag_id"].(string)
	if tagID == "" {
		return errorResult("tag_id is required for update action"), nil
	}

	resolvedID, err := h.resolveTagID(ctx, client, tagID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	config := h.buildTagConfig(args)

	tag, err := client.UpdateTag(ctx, resolvedID, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error updating tag: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Tag '%s' updated successfully", tag.Name)), nil
}

func (h *TagHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	tagID, _ := args["tag_id"].(string)
	if tagID == "" {
		return errorResult("tag_id is required for delete action"), nil
	}

	resolvedID, err := h.resolveTagID(ctx, client, tagID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	if err := client.DeleteTag(ctx, resolvedID); err != nil {
		return errorResult(fmt.Sprintf("error deleting tag: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Tag '%s' deleted successfully", resolvedID)), nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// findTagByInput performs two-phase lookup: exact ID match, then case-insensitive name substring match.
func (h *TagHandlers) findTagByInput(tags []homeassistant.TagRegistryEntry, input string) (*homeassistant.TagRegistryEntry, error) {
	// Phase 1: Exact ID match
	for i := range tags {
		if tags[i].TagID == input {
			return &tags[i], nil
		}
	}

	// Phase 2: Case-insensitive name substring match
	lowerInput := strings.ToLower(input)
	for i := range tags {
		if strings.Contains(strings.ToLower(tags[i].Name), lowerInput) {
			return &tags[i], nil
		}
	}

	return nil, fmt.Errorf("tag not found: %s (tried as tag_id and name)", input)
}

// resolveTagID resolves a tag input (ID or name) to the actual tag ID.
func (h *TagHandlers) resolveTagID(ctx context.Context, client homeassistant.Client, input string) (string, error) {
	tags, err := client.GetTags(ctx)
	if err != nil {
		return "", fmt.Errorf("error fetching tags: %w", err)
	}

	tag, err := h.findTagByInput(tags, input)
	if err != nil {
		return "", err
	}

	return tag.TagID, nil
}

func (h *TagHandlers) buildTagConfig(args map[string]any) homeassistant.TagConfig {
	config := homeassistant.TagConfig{}

	if tagID, ok := args["tag_id"].(string); ok && tagID != "" {
		config.TagID = tagID
	}
	if name, ok := args["name"].(string); ok && name != "" {
		config.Name = name
	}
	if description, ok := args["description"].(string); ok && description != "" {
		config.Description = description
	}

	return config
}

func (h *TagHandlers) filterTagsByName(tags []homeassistant.TagRegistryEntry, nameContains string) []homeassistant.TagRegistryEntry {
	filtered := make([]homeassistant.TagRegistryEntry, 0)
	lowerSearch := strings.ToLower(nameContains)

	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag.Name), lowerSearch) {
			filtered = append(filtered, tag)
		}
	}

	return filtered
}

// =============================================================================
// Formatting Methods
// =============================================================================

func (h *TagHandlers) formatListJSON(tags []homeassistant.TagRegistryEntry) (*mcp.ToolsCallResult, error) {
	return jsonResult(map[string]any{
		"tags":  tags,
		"count": len(tags),
	})
}

func (h *TagHandlers) formatListNatural(tags []homeassistant.TagRegistryEntry) (*mcp.ToolsCallResult, error) {
	if len(tags) == 0 {
		return successResult("No tags found."), nil
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d tag(s):\n", len(tags)))

	for _, tag := range tags {
		line := fmt.Sprintf("• %s (ID: %s)", tag.Name, tag.TagID)
		if tag.Description != "" {
			line += fmt.Sprintf("\n  Description: %s", tag.Description)
		}
		if tag.LastScanned != "" {
			line += fmt.Sprintf("\n  Last scanned: %s", tag.LastScanned)
		}
		parts = append(parts, line)
	}

	return successResult(strings.Join(parts, "\n")), nil
}

func (h *TagHandlers) formatGetJSON(tag *homeassistant.TagRegistryEntry) (*mcp.ToolsCallResult, error) {
	return jsonResult(tag)
}

func (h *TagHandlers) formatGetNatural(tag *homeassistant.TagRegistryEntry) (*mcp.ToolsCallResult, error) {
	var parts []string
	parts = append(parts,
		fmt.Sprintf("Tag: %s", tag.Name),
		fmt.Sprintf("ID: %s", tag.TagID))

	if tag.Description != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", tag.Description))
	}
	if tag.LastScanned != "" {
		parts = append(parts, fmt.Sprintf("Last scanned: %s", tag.LastScanned))
	}

	return successResult(strings.Join(parts, "\n")), nil
}
