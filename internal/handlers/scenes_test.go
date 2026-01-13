package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockSceneClient implements homeassistant.Client for testing.
type mockSceneClient struct {
	homeassistant.Client
	listScenesFn  func(ctx context.Context) ([]homeassistant.Entity, error)
	createSceneFn func(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error
	updateSceneFn func(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error
	deleteSceneFn func(ctx context.Context, sceneID string) error
	callServiceFn func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
	getStateFn    func(ctx context.Context, entityID string) (*homeassistant.Entity, error)
}

func (m *mockSceneClient) ListScenes(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.listScenesFn != nil {
		return m.listScenesFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *mockSceneClient) CreateScene(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error {
	if m.createSceneFn != nil {
		return m.createSceneFn(ctx, sceneID, config)
	}
	return nil
}

func (m *mockSceneClient) UpdateScene(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error {
	if m.updateSceneFn != nil {
		return m.updateSceneFn(ctx, sceneID, config)
	}
	return nil
}

func (m *mockSceneClient) DeleteScene(ctx context.Context, sceneID string) error {
	if m.deleteSceneFn != nil {
		return m.deleteSceneFn(ctx, sceneID)
	}
	return nil
}

func (m *mockSceneClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func (m *mockSceneClient) GetState(ctx context.Context, entityID string) (*homeassistant.Entity, error) {
	if m.getStateFn != nil {
		return m.getStateFn(ctx, entityID)
	}
	return &homeassistant.Entity{
		EntityID:   entityID,
		State:      "scening",
		Attributes: map[string]any{"friendly_name": "Test Scene"},
	}, nil
}

func TestNewSceneHandlers(t *testing.T) {
	t.Parallel()

	h := NewSceneHandlers()
	if h == nil {
		t.Error("NewSceneHandlers() returned nil")
	}
}

func TestSceneHandlers_Register(t *testing.T) {
	t.Parallel()

	h := NewSceneHandlers()
	registry := mcp.NewRegistry()

	h.Register(registry)

	tools := registry.ListTools()
	const expectedToolCount = 6
	if len(tools) != expectedToolCount {
		t.Errorf("Register() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	expectedTools := map[string]bool{
		"list_scenes":    false,
		"get_scene":      false,
		"create_scene":   false,
		"update_scene":   false,
		"delete_scene":   false,
		"activate_scene": false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("Tool %q not registered", name)
		}
	}
}

func TestSceneHandlers_Tools(t *testing.T) {
	t.Parallel()

	h := NewSceneHandlers()
	tools := h.Tools()

	const expectedToolCount = 6
	if len(tools) != expectedToolCount {
		t.Errorf("Tools() returned %d tools, want %d", len(tools), expectedToolCount)
	}
}

func TestSceneHandlers_HandleListScenes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          map[string]any
		listScenesErr error
		listScenes    []homeassistant.Entity
		wantError     bool
		wantContains  string
	}{
		{
			name:       "success empty",
			args:       map[string]any{},
			listScenes: []homeassistant.Entity{},
			wantError:  false,
		},
		{
			name: "success with scenes",
			args: map[string]any{},
			listScenes: []homeassistant.Entity{
				{
					EntityID: "scene.movie_time",
					State:    "scening",
					Attributes: map[string]any{
						"friendly_name": "Movie Time",
						"entity_id":     []any{"light.living_room", "media_player.tv"},
					},
				},
				{
					EntityID:   "scene.night_mode",
					State:      "scening",
					Attributes: map[string]any{"friendly_name": "Night Mode"},
				},
			},
			wantError:    false,
			wantContains: "movie_time",
		},
		{
			name: "success with name filter",
			args: map[string]any{
				"name_contains": "movie",
			},
			listScenes: []homeassistant.Entity{
				{
					EntityID:   "scene.movie_time",
					State:      "scening",
					Attributes: map[string]any{"friendly_name": "Movie Time"},
				},
				{
					EntityID:   "scene.night_mode",
					State:      "scening",
					Attributes: map[string]any{"friendly_name": "Night Mode"},
				},
			},
			wantError:    false,
			wantContains: "movie_time",
		},
		{
			name: "success with entity filter",
			args: map[string]any{
				"entity_contains": "light",
			},
			listScenes: []homeassistant.Entity{
				{
					EntityID: "scene.movie_time",
					State:    "scening",
					Attributes: map[string]any{
						"friendly_name": "Movie Time",
						"entity_id":     []any{"light.living_room"},
					},
				},
				{
					EntityID:   "scene.night_mode",
					State:      "scening",
					Attributes: map[string]any{"friendly_name": "Night Mode"},
				},
			},
			wantError:    false,
			wantContains: "movie_time",
		},
		{
			name:          "client error",
			args:          map[string]any{},
			listScenesErr: errors.New("connection failed"),
			wantError:     true,
			wantContains:  "Error listing scenes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockSceneClient{
				listScenesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					if tt.listScenesErr != nil {
						return nil, tt.listScenesErr
					}
					return tt.listScenes, nil
				},
			}

			h := NewSceneHandlers()
			result, err := h.HandleListScenes(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("HandleListScenes() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_HandleGetScene(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		getStateErr  error
		wantError    bool
		wantContains string
	}{
		{
			name: "success",
			args: map[string]any{
				"scene_id": "movie_time",
			},
			wantError: false,
		},
		{
			name:         "missing scene_id",
			args:         map[string]any{},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "empty scene_id",
			args: map[string]any{
				"scene_id": "",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"scene_id": "movie_time",
			},
			getStateErr:  errors.New("not found"),
			wantError:    true,
			wantContains: "Error getting scene",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockSceneClient{
				getStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					if tt.getStateErr != nil {
						return nil, tt.getStateErr
					}
					return &homeassistant.Entity{
						EntityID:   entityID,
						State:      "scening",
						Attributes: map[string]any{"friendly_name": "Movie Time"},
					}, nil
				},
			}

			h := NewSceneHandlers()
			result, err := h.HandleGetScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("HandleGetScene() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_HandleCreateScene(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		createSceneErr error
		wantError      bool
		wantContains   string
	}{
		{
			name: "success",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "Movie Time",
				"entities": map[string]any{
					"light.living_room": "off",
				},
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with detailed entities",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "Movie Time",
				"icon":     "mdi:movie",
				"entities": map[string]any{
					"light.living_room": map[string]any{
						"state": "on",
						"attributes": map[string]any{
							"brightness": 50,
							"color_temp": 400,
						},
					},
					"media_player.tv": "on",
				},
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing scene_id",
			args: map[string]any{
				"name": "Movie Time",
				"entities": map[string]any{
					"light.living_room": "off",
				},
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "empty scene_id",
			args: map[string]any{
				"scene_id": "",
				"name":     "Movie Time",
				"entities": map[string]any{
					"light.living_room": "off",
				},
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"scene_id": "movie_time",
				"entities": map[string]any{
					"light.living_room": "off",
				},
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "",
				"entities": map[string]any{
					"light.living_room": "off",
				},
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "missing entities",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "Movie Time",
			},
			wantError:    true,
			wantContains: "entities is required",
		},
		{
			name: "empty entities",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "Movie Time",
				"entities": map[string]any{},
			},
			wantError:    true,
			wantContains: "entities is required",
		},
		{
			name: "invalid entity state format",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "Movie Time",
				"entities": map[string]any{
					"light.living_room": 123,
				},
			},
			wantError:    true,
			wantContains: "Invalid state format",
		},
		{
			name: "client error",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "Movie Time",
				"entities": map[string]any{
					"light.living_room": "off",
				},
			},
			createSceneErr: errors.New("creation failed"),
			wantError:      true,
			wantContains:   "Error creating scene",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockSceneClient{
				createSceneFn: func(_ context.Context, _ string, _ homeassistant.SceneConfig) error {
					return tt.createSceneErr
				},
			}

			h := NewSceneHandlers()
			result, err := h.HandleCreateScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("HandleCreateScene() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_HandleUpdateScene(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		getStateErr    error
		updateSceneErr error
		wantError      bool
		wantContains   string
	}{
		{
			name: "success",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "Updated Movie Time",
			},
			wantError:    false,
			wantContains: "updated successfully",
		},
		{
			name: "success with entities",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "Updated Movie Time",
				"icon":     "mdi:movie-open",
				"entities": map[string]any{
					"light.living_room": "on",
				},
			},
			wantError:    false,
			wantContains: "updated successfully",
		},
		{
			name:         "missing scene_id",
			args:         map[string]any{},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "empty scene_id",
			args: map[string]any{
				"scene_id": "",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "get state error",
			args: map[string]any{
				"scene_id": "movie_time",
			},
			getStateErr:  errors.New("not found"),
			wantError:    true,
			wantContains: "Error getting current scene",
		},
		{
			name: "update error",
			args: map[string]any{
				"scene_id": "movie_time",
				"name":     "Updated",
			},
			updateSceneErr: errors.New("update failed"),
			wantError:      true,
			wantContains:   "Error updating scene",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockSceneClient{
				getStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					if tt.getStateErr != nil {
						return nil, tt.getStateErr
					}
					return &homeassistant.Entity{
						EntityID:   entityID,
						State:      "scening",
						Attributes: map[string]any{"friendly_name": "Movie Time"},
					}, nil
				},
				updateSceneFn: func(_ context.Context, _ string, _ homeassistant.SceneConfig) error {
					return tt.updateSceneErr
				},
			}

			h := NewSceneHandlers()
			result, err := h.HandleUpdateScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("HandleUpdateScene() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_HandleDeleteScene(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		deleteSceneErr error
		wantError      bool
		wantContains   string
	}{
		{
			name: "success",
			args: map[string]any{
				"scene_id": "movie_time",
			},
			wantError:    false,
			wantContains: "deleted successfully",
		},
		{
			name:         "missing scene_id",
			args:         map[string]any{},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "empty scene_id",
			args: map[string]any{
				"scene_id": "",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"scene_id": "movie_time",
			},
			deleteSceneErr: errors.New("deletion failed"),
			wantError:      true,
			wantContains:   "Error deleting scene",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockSceneClient{
				deleteSceneFn: func(_ context.Context, _ string) error {
					return tt.deleteSceneErr
				},
			}

			h := NewSceneHandlers()
			result, err := h.HandleDeleteScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("HandleDeleteScene() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_HandleActivateScene(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		callServiceErr error
		wantError      bool
		wantContains   string
	}{
		{
			name: "success",
			args: map[string]any{
				"scene_id": "movie_time",
			},
			wantError:    false,
			wantContains: "activated successfully",
		},
		{
			name: "success with transition",
			args: map[string]any{
				"scene_id":   "movie_time",
				"transition": 2.5,
			},
			wantError:    false,
			wantContains: "activated successfully",
		},
		{
			name:         "missing scene_id",
			args:         map[string]any{},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "empty scene_id",
			args: map[string]any{
				"scene_id": "",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"scene_id": "movie_time",
			},
			callServiceErr: errors.New("activation failed"),
			wantError:      true,
			wantContains:   "Error activating scene",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockSceneClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewSceneHandlers()
			result, err := h.HandleActivateScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("HandleActivateScene() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}
