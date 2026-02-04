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

// Scene action constants.
const (
	sceneActionList     = "list"
	sceneActionGet      = "get"
	sceneActionCreate   = "create"
	sceneActionUpdate   = "update"
	sceneActionDelete   = "delete"
	sceneActionActivate = "activate"
)

// SceneHandlers provides handlers for scene-related MCP tools.
type SceneHandlers struct{}

// NewSceneHandlers creates a new SceneHandlers instance.
func NewSceneHandlers() *SceneHandlers {
	return &SceneHandlers{}
}

// RegisterTools registers the consolidated manage_scene tool with the registry.
func (h *SceneHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageSceneTool(), h.handleManageScene)
}

// =============================================================================
// Tool Definition
// =============================================================================

func (h *SceneHandlers) manageSceneTool() mcp.Tool {
	return mcp.Tool{
		Name: "manage_scene",
		Description: `Manage Home Assistant scenes - list, get, create, update, delete, or activate.

Actions:
- list: List all scenes (optional filters: name_contains, entity_contains)
- get: Get details of a specific scene (requires scene_id)
- create: Create a new scene (requires scene_id, name, entities)
- update: Update an existing scene (requires scene_id)
- delete: Delete a scene (requires scene_id)
- activate: Activate a scene (requires scene_id, optional transition)`,
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Scene management operation",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Operation to perform: list, get, create, update, delete, activate",
					Enum:        []string{"list", "get", "create", "update", "delete", "activate"},
				},
				"scene_id": {
					Type:        "string",
					Description: "Scene ID without 'scene.' prefix (required for get/create/update/delete/activate)",
				},
				"name": {
					Type:        "string",
					Description: "Friendly name for the scene (required for create)",
				},
				"icon": {
					Type:        "string",
					Description: "Icon for the scene (e.g., mdi:lightbulb)",
				},
				"entities": {
					Type:        "object",
					Description: "Entity states to set when scene is activated. Keys are entity IDs, values are state objects with 'state' and optional 'attributes'",
				},
				"name_contains": {
					Type:        "string",
					Description: "Filter by scene name or entity_id containing this string (for list action, case-insensitive)",
				},
				"entity_contains": {
					Type:        "string",
					Description: "Filter to scenes that contain this entity ID in their entity list (for list action)",
				},
				"transition": {
					Type:        "number",
					Description: "Transition time in seconds (for activate action)",
				},
			},
			Required: []string{"action"},
		},
	}
}

// =============================================================================
// Main Handler
// =============================================================================

func (h *SceneHandlers) handleManageScene(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case sceneActionList:
		return h.handleList(ctx, client, args)
	case sceneActionGet:
		return h.handleGet(ctx, client, args)
	case sceneActionCreate:
		return h.handleCreate(ctx, client, args)
	case sceneActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case sceneActionDelete:
		return h.handleDelete(ctx, client, args)
	case sceneActionActivate:
		return h.handleActivate(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, delete, or activate)", action)), nil
	}
}

// =============================================================================
// Action Handlers
// =============================================================================

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

func (h *SceneHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error listing scenes: %v", err)), nil
	}

	filters := parseSceneFilters(args)
	result := filterScenes(scenes, filters)

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("Error marshaling scenes: %v", err)), nil
	}

	summary := fmt.Sprintf("Found %d scenes\n\n", len(result))
	return successResult(summary + string(jsonBytes)), nil
}

func (h *SceneHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for get action"), nil
	}

	entityID := "scene." + sceneID
	state, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting scene: %v", err)), nil
	}

	jsonBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("Error marshaling scene: %v", err)), nil
	}

	return successResult(string(jsonBytes)), nil
}

func (h *SceneHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for create action"), nil
	}

	name, ok := args["name"].(string)
	if !ok || name == "" {
		return errorResult("name is required for create action"), nil
	}

	entitiesRaw, ok := args["entities"].(map[string]any)
	if !ok || len(entitiesRaw) == 0 {
		return errorResult("entities is required for create action and must be a non-empty object"), nil
	}

	entities, invalidEntity := parseSceneEntities(entitiesRaw)
	if invalidEntity != "" {
		return errorResult(fmt.Sprintf("Invalid state format for entity %s", invalidEntity)), nil
	}

	config := homeassistant.SceneConfig{Name: name, Entities: entities}
	if icon, ok := args["icon"].(string); ok {
		config.Icon = icon
	}

	if err := client.CreateScene(ctx, sceneID, config); err != nil {
		return errorResult(fmt.Sprintf("Error creating scene: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Scene '%s' created successfully", sceneID)), nil
}

func (h *SceneHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for update action"), nil
	}

	entityID := "scene." + sceneID
	current, err := client.GetState(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting current scene: %v", err)), nil
	}

	config := buildSceneConfigFromArgs(current, args)

	if err := client.UpdateScene(ctx, sceneID, config); err != nil {
		return errorResult(fmt.Sprintf("Error updating scene: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Scene '%s' updated successfully", sceneID)), nil
}

func (h *SceneHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for delete action"), nil
	}

	if err := client.DeleteScene(ctx, sceneID); err != nil {
		return errorResult(fmt.Sprintf("Error deleting scene: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Scene '%s' deleted successfully", sceneID)), nil
}

func (h *SceneHandlers) handleActivate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return errorResult("scene_id is required for activate action"), nil
	}

	data := map[string]any{
		"entity_id": "scene." + sceneID,
	}

	if transition, ok := args["transition"].(float64); ok {
		data["transition"] = transition
	}

	if _, err := client.CallService(ctx, "scene", "turn_on", data); err != nil {
		return errorResult(fmt.Sprintf("Error activating scene: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Scene '%s' activated successfully", sceneID)), nil
}

// =============================================================================
// Helper Functions
// =============================================================================

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
