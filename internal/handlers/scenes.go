// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// SceneHandlers provides handlers for scene-related MCP tools.
type SceneHandlers struct{}

// NewSceneHandlers creates a new SceneHandlers instance.
func NewSceneHandlers() *SceneHandlers {
	return &SceneHandlers{}
}

// Tools returns all scene-related tool definitions.
func (h *SceneHandlers) Tools() []mcp.Tool {
	return []mcp.Tool{
		h.listScenesTool(),
		h.getSceneTool(),
		h.createSceneTool(),
		h.updateSceneTool(),
		h.deleteSceneTool(),
		h.activateSceneTool(),
	}
}

// Register registers all scene-related tools with the registry.
func (h *SceneHandlers) Register(registry *mcp.Registry) {
	registry.RegisterTool(h.listScenesTool(), h.HandleListScenes)
	registry.RegisterTool(h.getSceneTool(), h.HandleGetScene)
	registry.RegisterTool(h.createSceneTool(), h.HandleCreateScene)
	registry.RegisterTool(h.updateSceneTool(), h.HandleUpdateScene)
	registry.RegisterTool(h.deleteSceneTool(), h.HandleDeleteScene)
	registry.RegisterTool(h.activateSceneTool(), h.HandleActivateScene)
}

func (h *SceneHandlers) listScenesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_scenes",
		Description: "List all scenes in Home Assistant. Use filters to narrow down results.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Filter options for scenes list",
			Properties: map[string]mcp.JSONSchema{
				"name_contains": {
					Type:        "string",
					Description: "Filter by scene name or entity_id containing this string (case-insensitive)",
				},
				"entity_contains": {
					Type:        "string",
					Description: "Filter to scenes that contain this entity ID in their entity list",
				},
			},
		},
	}
}

func (h *SceneHandlers) getSceneTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_scene",
		Description: "Get details of a specific scene",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"scene_id": {
					Type:        "string",
					Description: "The scene ID (without 'scene.' prefix)",
				},
			},
			Required: []string{"scene_id"},
		},
	}
}

func (h *SceneHandlers) createSceneTool() mcp.Tool {
	return mcp.Tool{
		Name:        "create_scene",
		Description: "Create a new scene in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"scene_id": {
					Type:        "string",
					Description: "Unique ID for the scene (lowercase, underscores allowed)",
				},
				"name": {
					Type:        "string",
					Description: "Friendly name for the scene",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the scene (e.g., mdi:lightbulb)",
				},
				"entities": {
					Type:        "object",
					Description: "Entity states to set when scene is activated. Keys are entity IDs, values are state objects with 'state' and optional 'attributes'",
				},
			},
			Required: []string{"scene_id", "name", "entities"},
		},
	}
}

func (h *SceneHandlers) updateSceneTool() mcp.Tool {
	return mcp.Tool{
		Name:        "update_scene",
		Description: "Update an existing scene in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"scene_id": {
					Type:        "string",
					Description: "The scene ID to update",
				},
				"name": {
					Type:        "string",
					Description: "New friendly name for the scene",
				},
				"icon": {
					Type:        "string",
					Description: "New icon for the scene",
				},
				"entities": {
					Type:        "object",
					Description: "New entity states for the scene",
				},
			},
			Required: []string{"scene_id"},
		},
	}
}

func (h *SceneHandlers) deleteSceneTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_scene",
		Description: "Delete a scene from Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"scene_id": {
					Type:        "string",
					Description: "The scene ID to delete",
				},
			},
			Required: []string{"scene_id"},
		},
	}
}

func (h *SceneHandlers) activateSceneTool() mcp.Tool {
	return mcp.Tool{
		Name:        "activate_scene",
		Description: "Activate a scene in Home Assistant",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"scene_id": {
					Type:        "string",
					Description: "The scene ID to activate (without 'scene.' prefix)",
				},
				"transition": {
					Type:        "number",
					Description: "Transition time in seconds (optional)",
				},
			},
			Required: []string{"scene_id"},
		},
	}
}

// sceneInfo represents scene information for list output.
type sceneInfo struct {
	EntityID     string   `json:"entity_id"`
	State        string   `json:"state"`
	FriendlyName string   `json:"friendly_name,omitempty"`
	EntityIDs    []string `json:"entity_ids,omitempty"`
}

// sceneFilters holds filter parameters for listing scenes.
type sceneFilters struct {
	nameContains   string
	entityContains string
}

// parseSceneFilters extracts filter parameters from args.
func parseSceneFilters(args map[string]any) sceneFilters {
	nameContains, _ := args["name_contains"].(string)
	entityContains, _ := args["entity_contains"].(string)
	return sceneFilters{
		nameContains:   nameContains,
		entityContains: entityContains,
	}
}

// entityToSceneInfo converts an Entity to sceneInfo.
func entityToSceneInfo(s homeassistant.Entity) sceneInfo {
	info := sceneInfo{
		EntityID: s.EntityID,
		State:    s.State,
	}
	if name, ok := s.Attributes["friendly_name"].(string); ok {
		info.FriendlyName = name
	}
	if entityIDs, ok := s.Attributes["entity_id"].([]any); ok {
		info.EntityIDs = extractStringSlice(entityIDs)
	}
	return info
}

// extractStringSlice converts []any to []string.
func extractStringSlice(items []any) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// matchesSceneFilters checks if scene info matches all filters.
func matchesSceneFilters(info sceneInfo, filters sceneFilters) bool {
	if !matchesSceneNameFilter(info, filters.nameContains) {
		return false
	}
	if !matchesSceneEntityFilter(info, filters.entityContains) {
		return false
	}
	return true
}

// matchesSceneNameFilter checks if scene matches name filter.
func matchesSceneNameFilter(info sceneInfo, nameContains string) bool {
	if nameContains == "" {
		return true
	}
	nameLower := strings.ToLower(nameContains)
	return strings.Contains(strings.ToLower(info.EntityID), nameLower) ||
		strings.Contains(strings.ToLower(info.FriendlyName), nameLower)
}

// matchesSceneEntityFilter checks if scene contains the entity.
func matchesSceneEntityFilter(info sceneInfo, entityContains string) bool {
	if entityContains == "" {
		return true
	}
	entityLower := strings.ToLower(entityContains)
	for _, eid := range info.EntityIDs {
		if strings.Contains(strings.ToLower(eid), entityLower) {
			return true
		}
	}
	return false
}

// filterScenes applies filters to scenes and converts to sceneInfo.
func filterScenes(scenes []homeassistant.Entity, filters sceneFilters) []sceneInfo {
	result := make([]sceneInfo, 0, len(scenes))
	for _, s := range scenes {
		info := entityToSceneInfo(s)
		if matchesSceneFilters(info, filters) {
			result = append(result, info)
		}
	}
	return result
}

// HandleListScenes handles the list_scenes tool call.
func (h *SceneHandlers) HandleListScenes(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error listing scenes: %v", err))},
			IsError: true,
		}, nil
	}

	filters := parseSceneFilters(args)
	result := filterScenes(scenes, filters)

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error marshaling scenes: %v", err))},
			IsError: true,
		}, nil
	}

	summary := fmt.Sprintf("Found %d scenes\n\n", len(result))

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(summary + string(jsonBytes))},
	}, nil
}

// HandleGetScene handles the get_scene tool call.
func (h *SceneHandlers) HandleGetScene(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("scene_id is required")},
			IsError: true,
		}, nil
	}

	entityID := "scene." + sceneID
	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting scene: %v", err))},
			IsError: true,
		}, nil
	}

	jsonBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error marshaling scene: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(string(jsonBytes))},
	}, nil
}

// parseSceneState converts raw state data to SceneState.
func parseSceneState(stateRaw any) (homeassistant.SceneState, bool) {
	sceneState := homeassistant.SceneState{}

	switch v := stateRaw.(type) {
	case string:
		sceneState.State = v
		return sceneState, true
	case map[string]any:
		if state, ok := v["state"].(string); ok {
			sceneState.State = state
		}
		if attrs, ok := v["attributes"].(map[string]any); ok {
			sceneState.Attributes = attrs
		}
		return sceneState, true
	default:
		return sceneState, false
	}
}

// parseSceneEntities converts raw entities map to SceneState map.
func parseSceneEntities(entitiesRaw map[string]any) (map[string]homeassistant.SceneState, string) {
	entities := make(map[string]homeassistant.SceneState, len(entitiesRaw))
	for entityID, stateRaw := range entitiesRaw {
		sceneState, ok := parseSceneState(stateRaw)
		if !ok {
			return nil, entityID
		}
		entities[entityID] = sceneState
	}
	return entities, ""
}

// HandleCreateScene handles the create_scene tool call.
func (h *SceneHandlers) HandleCreateScene(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("scene_id is required")},
			IsError: true,
		}, nil
	}

	name, ok := args["name"].(string)
	if !ok || name == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("name is required")},
			IsError: true,
		}, nil
	}

	entitiesRaw, ok := args["entities"].(map[string]any)
	if !ok || len(entitiesRaw) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("entities is required and must be a non-empty object")},
			IsError: true,
		}, nil
	}

	entities, invalidEntity := parseSceneEntities(entitiesRaw)
	if invalidEntity != "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Invalid state format for entity %s", invalidEntity))},
			IsError: true,
		}, nil
	}

	config := homeassistant.SceneConfig{Name: name, Entities: entities}
	if icon, ok := args["icon"].(string); ok {
		config.Icon = icon
	}

	if err := client.CreateScene(ctx, sceneID, config); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error creating scene: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Scene '%s' created successfully", sceneID))},
	}, nil
}

// buildSceneConfigFromArgs builds scene config from current state and args.
func buildSceneConfigFromArgs(current *homeassistant.Entity, args map[string]any) homeassistant.SceneConfig {
	config := homeassistant.SceneConfig{Entities: make(map[string]homeassistant.SceneState)}

	if name, ok := current.Attributes["friendly_name"].(string); ok {
		config.Name = name
	}
	if name, ok := args["name"].(string); ok {
		config.Name = name
	}
	if icon, ok := args["icon"].(string); ok {
		config.Icon = icon
	}

	if entitiesRaw, ok := args["entities"].(map[string]any); ok {
		for eid, stateRaw := range entitiesRaw {
			sceneState, _ := parseSceneState(stateRaw)
			config.Entities[eid] = sceneState
		}
	}

	return config
}

// HandleUpdateScene handles the update_scene tool call.
func (h *SceneHandlers) HandleUpdateScene(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("scene_id is required")},
			IsError: true,
		}, nil
	}

	entityID := "scene." + sceneID
	current, err := client.GetState(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting current scene: %v", err))},
			IsError: true,
		}, nil
	}

	config := buildSceneConfigFromArgs(current, args)

	if err := client.UpdateScene(ctx, sceneID, config); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error updating scene: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Scene '%s' updated successfully", sceneID))},
	}, nil
}

// HandleDeleteScene handles the delete_scene tool call.
func (h *SceneHandlers) HandleDeleteScene(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("scene_id is required")},
			IsError: true,
		}, nil
	}

	if err := client.DeleteScene(ctx, sceneID); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error deleting scene: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Scene '%s' deleted successfully", sceneID))},
	}, nil
}

// HandleActivateScene handles the activate_scene tool call.
func (h *SceneHandlers) HandleActivateScene(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("scene_id is required")},
			IsError: true,
		}, nil
	}

	data := map[string]any{
		"entity_id": "scene." + sceneID,
	}

	if transition, ok := args["transition"].(float64); ok {
		data["transition"] = transition
	}

	if _, err := client.CallService(ctx, "scene", "turn_on", data); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error activating scene: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Scene '%s' activated successfully", sceneID))},
	}, nil
}

// RegisterSceneTools registers all scene-related tools with the registry.
//
// Deprecated: Use NewSceneHandlers().Register(registry) instead.
func RegisterSceneTools(registry *mcp.Registry) {
	h := NewSceneHandlers()
	h.Register(registry)
}
