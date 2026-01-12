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

// HandleListScenes handles the list_scenes tool call.
func (h *SceneHandlers) HandleListScenes(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error listing scenes: %v", err))},
			IsError: true,
		}, nil
	}

	// Parse filter parameters
	nameContains, _ := args["name_contains"].(string)
	entityContains, _ := args["entity_contains"].(string)

	// Normalize filters for case-insensitive matching
	nameContainsLower := strings.ToLower(nameContains)
	entityContainsLower := strings.ToLower(entityContains)

	type sceneInfo struct {
		EntityID     string   `json:"entity_id"`
		State        string   `json:"state"`
		FriendlyName string   `json:"friendly_name,omitempty"`
		EntityIDs    []string `json:"entity_ids,omitempty"`
	}

	result := make([]sceneInfo, 0, len(scenes))
	for _, s := range scenes {
		info := sceneInfo{
			EntityID: s.EntityID,
			State:    s.State,
		}
		if name, ok := s.Attributes["friendly_name"].(string); ok {
			info.FriendlyName = name
		}
		if entityIDs, ok := s.Attributes["entity_id"].([]any); ok {
			for _, eid := range entityIDs {
				if id, ok := eid.(string); ok {
					info.EntityIDs = append(info.EntityIDs, id)
				}
			}
		}

		// Apply name_contains filter
		if nameContains != "" {
			matchesName := strings.Contains(strings.ToLower(info.EntityID), nameContainsLower) ||
				strings.Contains(strings.ToLower(info.FriendlyName), nameContainsLower)
			if !matchesName {
				continue
			}
		}

		// Apply entity_contains filter
		if entityContains != "" {
			containsEntity := false
			for _, eid := range info.EntityIDs {
				if strings.Contains(strings.ToLower(eid), entityContainsLower) {
					containsEntity = true
					break
				}
			}
			if !containsEntity {
				continue
			}
		}

		result = append(result, info)
	}

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

	// Convert entities to SceneState map
	entities := make(map[string]homeassistant.SceneState)
	for entityID, stateRaw := range entitiesRaw {
		sceneState := homeassistant.SceneState{}

		switch v := stateRaw.(type) {
		case string:
			// Simple state value
			sceneState.State = v
		case map[string]any:
			// Full state object with state and attributes
			if state, ok := v["state"].(string); ok {
				sceneState.State = state
			}
			if attrs, ok := v["attributes"].(map[string]any); ok {
				sceneState.Attributes = attrs
			}
		default:
			return &mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Invalid state format for entity %s", entityID))},
				IsError: true,
			}, nil
		}

		entities[entityID] = sceneState
	}

	config := homeassistant.SceneConfig{
		Name:     name,
		Entities: entities,
	}

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

// HandleUpdateScene handles the update_scene tool call.
func (h *SceneHandlers) HandleUpdateScene(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	sceneID, ok := args["scene_id"].(string)
	if !ok || sceneID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent("scene_id is required")},
			IsError: true,
		}, nil
	}

	// Get current scene state to preserve existing values
	entityID := "scene." + sceneID
	current, err := client.GetState(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error getting current scene: %v", err))},
			IsError: true,
		}, nil
	}

	// Build config from current state and args
	config := homeassistant.SceneConfig{
		Entities: make(map[string]homeassistant.SceneState),
	}

	// Get current name from attributes
	if name, ok := current.Attributes["friendly_name"].(string); ok {
		config.Name = name
	}

	// Override with new values from args
	if name, ok := args["name"].(string); ok {
		config.Name = name
	}
	if icon, ok := args["icon"].(string); ok {
		config.Icon = icon
	}

	// Handle entities update
	if entitiesRaw, ok := args["entities"].(map[string]any); ok {
		for eid, stateRaw := range entitiesRaw {
			sceneState := homeassistant.SceneState{}

			switch v := stateRaw.(type) {
			case string:
				sceneState.State = v
			case map[string]any:
				if state, ok := v["state"].(string); ok {
					sceneState.State = state
				}
				if attrs, ok := v["attributes"].(map[string]any); ok {
					sceneState.Attributes = attrs
				}
			}

			config.Entities[eid] = sceneState
		}
	}

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
