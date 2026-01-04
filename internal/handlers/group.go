// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

const platformGroup = "group"

// GroupHandlers provides MCP tool handlers for group helper operations.
type GroupHandlers struct{}

// NewGroupHandlers creates a new GroupHandlers instance.
func NewGroupHandlers() *GroupHandlers {
	return &GroupHandlers{}
}

// RegisterTools registers all group-related tools with the registry.
func (h *GroupHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.createGroupTool(), h.handleCreateGroup)
	registry.RegisterTool(h.deleteGroupTool(), h.handleDeleteGroup)
	registry.RegisterTool(h.setGroupEntitiesTool(), h.handleSetGroupEntities)
	registry.RegisterTool(h.reloadGroupTool(), h.handleReloadGroup)
}

func (h *GroupHandlers) createGroupTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_group",
		Description: "Create a new group helper in Home Assistant. A group combines multiple entities into one. The group state is determined by the member entities' states.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Group configuration",
			Properties: map[string]mcp.JSONSchema{
				"id": {
					Type:        "string",
					Description: "Unique identifier for the helper (without platform prefix)",
				},
				"name": {
					Type:        "string",
					Description: "Human-readable name for the group",
				},
				"entities": {
					Type:        "array",
					Description: "List of entity IDs to include in the group",
					Items: &mcp.JSONSchema{
						Type: "string",
					},
				},
				"all": {
					Type:        "boolean",
					Description: "If true, the group is 'on' only when ALL members are on. If false (default), the group is 'on' when ANY member is on.",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the helper (e.g., mdi:lightbulb-group)",
				},
			},
			Required: []string{"id", "name", "entities"},
		},
	}
}

func (h *GroupHandlers) deleteGroupTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_group",
		Description: "Delete a group helper from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Group entity ID to delete",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the group (e.g., group.living_room_lights)",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *GroupHandlers) setGroupEntitiesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "set_group_entities",
		Description: "Add or remove entities from an existing group. You can specify entities to add and/or remove.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Group entity modification",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The full entity ID of the group (e.g., group.living_room_lights)",
				},
				"add_entities": {
					Type:        "array",
					Description: "List of entity IDs to add to the group",
					Items: &mcp.JSONSchema{
						Type: "string",
					},
				},
				"remove_entities": {
					Type:        "array",
					Description: "List of entity IDs to remove from the group",
					Items: &mcp.JSONSchema{
						Type: "string",
					},
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

func (h *GroupHandlers) reloadGroupTool() mcp.Tool {
	return mcp.Tool{
		Name:        "reload_group",
		Description: "Reload all group helpers from configuration. Use this after manually editing group configuration files.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "No parameters required",
			Properties:  map[string]mcp.JSONSchema{},
		},
	}
}

func (h *GroupHandlers) handleCreateGroup(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("id is required")},
			IsError: true,
		}, nil
	}

	name, _ := args["name"].(string)
	if name == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("name is required")},
			IsError: true,
		}, nil
	}

	entities, ok := args["entities"].([]any)
	if !ok || len(entities) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entities is required and must contain at least one entity ID")},
			IsError: true,
		}, nil
	}

	config := buildGroupHelperConfig(name, args)

	helper := homeassistant.HelperConfig{
		Platform: platformGroup,
		ID:       id,
		Config:   config,
	}

	if err := client.CreateHelper(ctx, helper); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating group: %v", err))},
			IsError: true,
		}, nil
	}

	entityID := fmt.Sprintf("%s.%s", platformGroup, id)
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Group '%s' created successfully as %s with %d entities", name, entityID, len(entities)))},
	}, nil
}

func (h *GroupHandlers) handleDeleteGroup(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformGroup {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a group entity (e.g., group.living_room_lights)")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteHelper(ctx, entityID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting group: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Group '%s' deleted successfully", entityID))},
	}, nil
}

func (h *GroupHandlers) handleSetGroupEntities(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id is required")},
			IsError: true,
		}, nil
	}

	platform, _ := ParseHelperEntityID(entityID)
	if platform != platformGroup {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entity_id must be a group entity (e.g., group.living_room_lights)")},
			IsError: true,
		}, nil
	}

	serviceData := map[string]any{
		"entity_id": entityID,
	}

	addEntities, hasAdd := args["add_entities"].([]any)
	removeEntities, hasRemove := args["remove_entities"].([]any)

	if !hasAdd && !hasRemove {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("at least one of add_entities or remove_entities is required")},
			IsError: true,
		}, nil
	}

	if hasAdd && len(addEntities) > 0 {
		serviceData["add_entities"] = addEntities
	}

	if hasRemove && len(removeEntities) > 0 {
		serviceData["remove_entities"] = removeEntities
	}

	if _, err := client.CallService(ctx, platformGroup, "set", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error modifying group entities: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Group '%s' entities updated successfully", entityID))},
	}, nil
}

func (h *GroupHandlers) handleReloadGroup(ctx context.Context, client homeassistant.Client, _ map[string]any) (*mcp.ToolsCallResult, error) {
	serviceData := map[string]any{}

	if _, err := client.CallService(ctx, platformGroup, "reload", serviceData); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error reloading groups: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent("Groups reloaded successfully")},
	}, nil
}

// buildGroupHelperConfig builds the configuration map for a group helper.
func buildGroupHelperConfig(name string, args map[string]any) map[string]any {
	config := make(map[string]any)
	config["name"] = name

	if entities, ok := args["entities"].([]any); ok {
		config["entities"] = entities
	}

	if all, ok := args["all"].(bool); ok {
		config["all"] = all
	}

	if icon, ok := args["icon"].(string); ok && icon != "" {
		config["icon"] = icon
	}

	return config
}
