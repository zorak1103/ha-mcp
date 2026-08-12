package handlers

import (
	"context"
	"errors"
	"strings"
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
	getSceneFn    func(ctx context.Context, sceneID string) (*homeassistant.Scene, error)
	callServiceFn func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
	getStateFn    func(ctx context.Context, entityID string) (*homeassistant.Entity, error)

	// Track IDs passed to methods for verification
	lastUpdateSceneID string
	lastDeleteSceneID string
	lastGetStateID    string
	lastGetSceneID    string

	// entityDeleted tracks whether DeleteScene was successfully called.
	// Used to make GetState return "not found" after delete (for fast waitForEntityDisappear in tests).
	entityDeleted bool
}

// ConfigFileEntryExists defaults to "present" so existing tests that never set up the
// config-file write guard continue to exercise the write path unchanged; tests that need to
// exercise the guard itself use UniversalMockClient (see TestSceneHandlers_Update_*).
func (m *mockSceneClient) ConfigFileEntryExists(context.Context, string, string) (bool, error) {
	return true, nil
}

func (m *mockSceneClient) ListScenes(ctx context.Context) ([]homeassistant.Entity, error) {
	if m.listScenesFn != nil {
		return m.listScenesFn(ctx)
	}
	return []homeassistant.Entity{}, nil
}

func (m *mockSceneClient) CreateScene(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error {
	if m.createSceneFn != nil {
		err := m.createSceneFn(ctx, sceneID, config)
		if err == nil {
			m.entityDeleted = false
		}
		return err
	}
	m.entityDeleted = false
	return nil
}

func (m *mockSceneClient) UpdateScene(ctx context.Context, sceneID string, config homeassistant.SceneConfig) error {
	m.lastUpdateSceneID = sceneID
	if m.updateSceneFn != nil {
		return m.updateSceneFn(ctx, sceneID, config)
	}
	return nil
}

func (m *mockSceneClient) DeleteScene(ctx context.Context, sceneID string) error {
	m.lastDeleteSceneID = sceneID
	if m.deleteSceneFn != nil {
		err := m.deleteSceneFn(ctx, sceneID)
		if err == nil {
			m.entityDeleted = true
		}
		return err
	}
	m.entityDeleted = true
	return nil
}

func (m *mockSceneClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func (m *mockSceneClient) GetState(ctx context.Context, entityID string) (*homeassistant.Entity, error) {
	m.lastGetStateID = entityID
	if m.getStateFn != nil {
		return m.getStateFn(ctx, entityID)
	}
	if m.entityDeleted {
		return nil, errors.New("entity not found")
	}
	return &homeassistant.Entity{
		EntityID:   entityID,
		State:      "scening",
		Attributes: map[string]any{"friendly_name": "Test Scene"},
	}, nil
}

func (m *mockSceneClient) GetScene(ctx context.Context, sceneID string) (*homeassistant.Scene, error) {
	m.lastGetSceneID = sceneID
	if m.getSceneFn != nil {
		return m.getSceneFn(ctx, sceneID)
	}
	return &homeassistant.Scene{
		EntityID: "scene." + sceneID,
		Config: &homeassistant.SceneConfig{
			Name: "Movie Time",
			Entities: map[string]homeassistant.SceneState{
				"light.living_room": {State: "on"},
			},
		},
	}, nil
}

func TestNewSceneHandlers(t *testing.T) {
	t.Parallel()

	h := NewSceneHandlers()
	if h == nil {
		t.Error("NewSceneHandlers() returned nil")
	}
}

func TestSceneHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewSceneHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 1 // manage_scene
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	if tools[0].Name != "manage_scene" {
		t.Errorf("Expected tool name 'manage_scene', got %q", tools[0].Name)
	}
}

func TestSceneHandlers_ManageScene_List(t *testing.T) {
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
			args:       map[string]any{"action": "list"},
			listScenes: []homeassistant.Entity{},
			wantError:  false,
		},
		{
			name: "success with scenes (json)",
			args: map[string]any{"action": "list", "format": "json"},
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
			name: "success with scenes (natural)",
			args: map[string]any{"action": "list"},
			listScenes: []homeassistant.Entity{
				{
					EntityID: "scene.movie_time",
					State:    "scening",
					Attributes: map[string]any{
						"friendly_name": "Movie Time",
						"entity_id":     []any{"light.living_room", "media_player.tv"},
					},
				},
			},
			wantError:    false,
			wantContains: "Movie Time",
		},
		{
			name: "success with name filter (json)",
			args: map[string]any{
				"action":        "list",
				"name_contains": "movie",
				"format":        "json",
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
			name: "success with entity filter (json)",
			args: map[string]any{
				"action":          "list",
				"entity_contains": "light",
				"format":          "json",
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
			args:          map[string]any{"action": "list"},
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
			result, err := h.handleManageScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScene() returned error: %v", err)
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
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_ManageScene_Get(t *testing.T) {
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
				"action":   "get",
				"scene_id": "movie_time",
			},
			wantError: false,
		},
		{
			name: "missing scene_id",
			args: map[string]any{
				"action": "get",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "empty scene_id",
			args: map[string]any{
				"action":   "get",
				"scene_id": "",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"action":   "get",
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
			result, err := h.handleManageScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScene() returned error: %v", err)
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
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_ManageScene_Create(t *testing.T) {
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
				"action":   "create",
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
				"action":   "create",
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
				"action": "create",
				"name":   "Movie Time",
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
				"action":   "create",
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
				"action":   "create",
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
				"action":   "create",
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
				"action":   "create",
				"scene_id": "movie_time",
				"name":     "Movie Time",
			},
			wantError:    true,
			wantContains: "entities is required",
		},
		{
			name: "empty entities",
			args: map[string]any{
				"action":   "create",
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
				"action":   "create",
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
				"action":   "create",
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
		{
			name: "mismatched scene_id and name shows both ids",
			args: map[string]any{
				"action":   "create",
				"scene_id": "cozy_evening",
				"name":     "Gemütlicher Abend",
				"entities": map[string]any{
					"light.living_room": "on",
				},
			},
			wantError:    false,
			wantContains: "entity_id: scene.gemutlicher_abend",
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
			result, err := h.handleManageScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScene() returned error: %v", err)
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
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_ManageScene_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		getSceneErr    error
		getStateErr    error
		updateSceneErr error
		wantError      bool
		wantContains   string
	}{
		{
			name: "success",
			args: map[string]any{
				"action":   "update",
				"scene_id": "movie_time",
				"name":     "Updated Movie Time",
			},
			wantError:    false,
			wantContains: "updated successfully",
		},
		{
			name: "success with entities",
			args: map[string]any{
				"action":   "update",
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
			name: "missing scene_id",
			args: map[string]any{
				"action": "update",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "empty scene_id",
			args: map[string]any{
				"action":   "update",
				"scene_id": "",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "get scene transient error",
			args: map[string]any{
				"action":   "update",
				"scene_id": "movie_time",
			},
			getSceneErr:  errors.New("connection timeout"),
			wantError:    true,
			wantContains: "Error getting current scene",
		},
		{
			name: "get scene not found, entity also gone",
			args: map[string]any{
				"action":   "update",
				"scene_id": "movie_time",
			},
			getSceneErr:  errors.New("scene not found: movie_time"),
			getStateErr:  errors.New("entity not found"),
			wantError:    true,
			wantContains: "scene not found",
		},
		{
			name: "update error",
			args: map[string]any{
				"action":   "update",
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
				getSceneFn: func(_ context.Context, sceneID string) (*homeassistant.Scene, error) {
					if tt.getSceneErr != nil {
						return nil, tt.getSceneErr
					}
					return &homeassistant.Scene{
						EntityID: "scene." + sceneID,
						Config: &homeassistant.SceneConfig{
							Name: "Movie Time",
							Entities: map[string]homeassistant.SceneState{
								"light.living_room": {State: "on"},
							},
						},
					}, nil
				},
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
			result, err := h.handleManageScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScene() returned error: %v", err)
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
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_Update_RefusesWhenConfigFileEntryMissing(t *testing.T) {
	t.Parallel()
	updateCalled := false
	client := &UniversalMockClient{}
	client.GetSceneFn = func(context.Context, string) (*homeassistant.Scene, error) {
		return nil, errors.New("scene not found: movie_night")
	}
	client.GetStateFn = func(context.Context, string) (*homeassistant.Entity, error) {
		return &homeassistant.Entity{EntityID: "scene.movie_night", Attributes: map[string]any{"entity_id": []any{"light.living_room"}}}, nil
	}
	client.UpdateSceneFn = func(context.Context, string, homeassistant.SceneConfig) error {
		updateCalled = true
		return nil
	}

	h := &SceneHandlers{}
	args := map[string]any{"action": "update", "scene_id": "movie_night", "name": "Movie Night 2"}
	result, err := h.handleManageScene(context.Background(), client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected refusal, got success: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "scenes.yaml") {
		t.Errorf("expected refusal to name scenes.yaml, got: %s", result.Content[0].Text)
	}
	if updateCalled {
		t.Error("UpdateScene must NOT be called when the id is confirmed absent from the file")
	}
}

func TestSceneHandlers_Update_ProceedsWhenConfigFileEntryExists(t *testing.T) {
	t.Parallel()
	client := &UniversalMockClient{}
	client.GetSceneFn = func(context.Context, string) (*homeassistant.Scene, error) {
		return &homeassistant.Scene{
			EntityID: "scene.movie_night",
			Config: &homeassistant.SceneConfig{
				Name:     "Movie Night",
				Entities: map[string]homeassistant.SceneState{"light.living_room": {State: "on"}},
			},
		}, nil
	}
	client.UpdateSceneFn = func(context.Context, string, homeassistant.SceneConfig) error { return nil }

	h := &SceneHandlers{}
	args := map[string]any{"action": "update", "scene_id": "movie_night", "name": "Movie Night 2"}
	result, err := h.handleManageScene(context.Background(), client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
}

func TestSceneHandlers_Update_PreservesUnspecifiedFields(t *testing.T) {
	t.Parallel()
	client := &UniversalMockClient{}
	client.GetSceneFn = func(context.Context, string) (*homeassistant.Scene, error) {
		return &homeassistant.Scene{
			EntityID: "scene.movie_night",
			Config: &homeassistant.SceneConfig{
				Name: "Movie Night",
				Icon: "mdi:movie",
				Entities: map[string]homeassistant.SceneState{
					"light.living_room": {State: "on", Attributes: map[string]any{"brightness": float64(120)}},
					"switch.tv":         {State: "on"},
				},
				Metadata: map[string]any{"light.living_room": map[string]any{"entity_only": true}},
			},
		}, nil
	}
	var lastConfig homeassistant.SceneConfig
	client.UpdateSceneFn = func(_ context.Context, _ string, config homeassistant.SceneConfig) error {
		lastConfig = config
		return nil
	}

	h := &SceneHandlers{}
	args := map[string]any{"action": "update", "scene_id": "movie_night", "name": "Movie Night 2"}
	result, err := h.handleManageScene(context.Background(), client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	if lastConfig.Name != "Movie Night 2" {
		t.Errorf("Name = %q, want %q", lastConfig.Name, "Movie Night 2")
	}
	if lastConfig.Icon != "mdi:movie" {
		t.Errorf("Icon = %q, want preserved %q", lastConfig.Icon, "mdi:movie")
	}
	if len(lastConfig.Entities) != 2 {
		t.Errorf("Entities = %v, want 2 preserved entries", lastConfig.Entities)
	}
	if got := lastConfig.Entities["light.living_room"].Attributes["brightness"]; got != float64(120) {
		t.Errorf("brightness attribute = %v, want preserved 120", got)
	}
	if lastConfig.Metadata == nil {
		t.Error("Metadata was dropped, want preserved")
	}
}

func TestSceneHandlers_Update_EntitiesReplaceWholesale(t *testing.T) {
	t.Parallel()
	client := &UniversalMockClient{}
	client.GetSceneFn = func(context.Context, string) (*homeassistant.Scene, error) {
		return &homeassistant.Scene{
			EntityID: "scene.movie_night",
			Config: &homeassistant.SceneConfig{
				Name: "Movie Night",
				Entities: map[string]homeassistant.SceneState{
					"light.living_room": {State: "on"},
					"switch.tv":         {State: "on"},
				},
			},
		}, nil
	}
	var lastConfig homeassistant.SceneConfig
	client.UpdateSceneFn = func(_ context.Context, _ string, config homeassistant.SceneConfig) error {
		lastConfig = config
		return nil
	}

	h := &SceneHandlers{}
	args := map[string]any{
		"action":   "update",
		"scene_id": "movie_night",
		"entities": map[string]any{"light.bedroom": "off"},
	}
	result, err := h.handleManageScene(context.Background(), client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}
	if len(lastConfig.Entities) != 1 {
		t.Fatalf("Entities = %v, want wholesale replacement with exactly 1 entry", lastConfig.Entities)
	}
	if _, ok := lastConfig.Entities["light.bedroom"]; !ok {
		t.Error("expected light.bedroom in replaced entities map")
	}
}

func TestSceneHandlers_Update_RefusesWhenConfigNil(t *testing.T) {
	t.Parallel()
	updateCalled := false
	client := &UniversalMockClient{}
	client.GetSceneFn = func(context.Context, string) (*homeassistant.Scene, error) {
		return &homeassistant.Scene{EntityID: "scene.movie_night", Config: nil}, nil
	}
	client.UpdateSceneFn = func(context.Context, string, homeassistant.SceneConfig) error {
		updateCalled = true
		return nil
	}

	h := &SceneHandlers{}
	args := map[string]any{"action": "update", "scene_id": "movie_night", "name": "Movie Night 2"}
	result, err := h.handleManageScene(context.Background(), client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected refusal, got success: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "no configuration") {
		t.Errorf("expected 'no configuration' in message, got: %s", result.Content[0].Text)
	}
	if updateCalled {
		t.Error("UpdateScene must NOT be called when Config is nil")
	}
}

func TestSceneHandlers_ManageScene_Delete(t *testing.T) {
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
				"action":   "delete",
				"scene_id": "movie_time",
			},
			wantError:    false,
			wantContains: "deleted successfully",
		},
		{
			name: "missing scene_id",
			args: map[string]any{
				"action": "delete",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "empty scene_id",
			args: map[string]any{
				"action":   "delete",
				"scene_id": "",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"action":   "delete",
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
			result, err := h.handleManageScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScene() returned error: %v", err)
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
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_ManageScene_Activate(t *testing.T) {
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
				"action":   "activate",
				"scene_id": "movie_time",
			},
			wantError:    false,
			wantContains: "activated successfully",
		},
		{
			name: "success with transition",
			args: map[string]any{
				"action":     "activate",
				"scene_id":   "movie_time",
				"transition": 2.5,
			},
			wantError:    false,
			wantContains: "activated successfully",
		},
		{
			name: "missing scene_id",
			args: map[string]any{
				"action": "activate",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "empty scene_id",
			args: map[string]any{
				"action":   "activate",
				"scene_id": "",
			},
			wantError:    true,
			wantContains: "scene_id is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"action":   "activate",
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
				callServiceFn: func(_ context.Context, domain, service string, _ map[string]any) ([]homeassistant.Entity, error) {
					if domain != "scene" {
						t.Errorf("wrong domain: %s", domain)
					}
					if service != "turn_on" {
						t.Errorf("wrong service: %s", service)
					}
					return nil, tt.callServiceErr
				},
			}

			h := NewSceneHandlers()
			result, err := h.handleManageScene(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleManageScene() returned error: %v", err)
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
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestSceneHandlers_ManageScene_InvalidAction(t *testing.T) {
	t.Parallel()

	h := NewSceneHandlers()
	client := &mockSceneClient{}

	// Test missing action
	result, err := h.handleManageScene(context.Background(), client, map[string]any{})
	if err != nil {
		t.Errorf("handleManageScene() returned error: %v", err)
		return
	}
	if !result.IsError {
		t.Error("Expected error for missing action")
	}
	if !strings.Contains(result.Content[0].Text, "action is required") {
		t.Errorf("Expected 'action is required' error, got: %s", result.Content[0].Text)
	}

	// Test invalid action
	result, err = h.handleManageScene(context.Background(), client, map[string]any{"action": "invalid"})
	if err != nil {
		t.Errorf("handleManageScene() returned error: %v", err)
		return
	}
	if !result.IsError {
		t.Error("Expected error for invalid action")
	}
	if !strings.Contains(result.Content[0].Text, "invalid action") {
		t.Errorf("Expected 'invalid action' error, got: %s", result.Content[0].Text)
	}
}

func TestFindSceneByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		searchID     string
		scenes       []homeassistant.Entity
		stateMap     map[string]*homeassistant.Entity
		wantFound    bool
		wantEntityID string
	}{
		{
			name:     "find by entity_id",
			searchID: "scene.movie_time",
			scenes: []homeassistant.Entity{
				{EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			stateMap: map[string]*homeassistant.Entity{
				"scene.movie_time": {EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			wantFound:    true,
			wantEntityID: "scene.movie_time",
		},
		{
			name:     "find by friendly_name - partial match",
			searchID: "movie time",
			scenes: []homeassistant.Entity{
				{EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			stateMap: map[string]*homeassistant.Entity{
				"scene.movie_time": {EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			wantFound:    true,
			wantEntityID: "scene.movie_time",
		},
		{
			name:     "find by friendly_name - case insensitive",
			searchID: "MOVIE TIME",
			scenes: []homeassistant.Entity{
				{EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			stateMap: map[string]*homeassistant.Entity{
				"scene.movie_time": {EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			wantFound:    true,
			wantEntityID: "scene.movie_time",
		},
		{
			name:     "find by friendly_name - partial match with 'Scene' suffix",
			searchID: "Scene",
			scenes: []homeassistant.Entity{
				{EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			stateMap: map[string]*homeassistant.Entity{
				"scene.movie_time": {EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			wantFound:    true,
			wantEntityID: "scene.movie_time",
		},
		{
			name:     "not found - no matching friendly_name",
			searchID: "nonexistent",
			scenes: []homeassistant.Entity{
				{EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			stateMap: map[string]*homeassistant.Entity{
				"scene.movie_time": {EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockSceneClient{
				listScenesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
					return tt.scenes, nil
				},
				getStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
					if e, ok := tt.stateMap[entityID]; ok {
						return e, nil
					}
					return nil, errors.New("not found")
				},
			}

			h := &SceneHandlers{}
			result, err := h.findSceneByID(context.Background(), client, tt.searchID)

			if tt.wantFound {
				if err != nil {
					t.Errorf("findSceneByID() unexpected error = %v", err)
					return
				}
				if result == nil {
					t.Error("findSceneByID() returned nil, want scene")
					return
				}
				if result.EntityID != tt.wantEntityID {
					t.Errorf("findSceneByID() EntityID = %q, want %q", result.EntityID, tt.wantEntityID)
				}
			} else {
				if err == nil {
					t.Error("findSceneByID() expected error, got nil")
				}
			}
		})
	}
}

func TestManageScene_GetByFriendlyName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		setupClient  func() *mockSceneClient
		wantError    bool
		wantContains string
	}{
		{
			name: "get by friendly_name - partial match",
			args: map[string]any{"action": "get", "scene_id": "movie time"},
			setupClient: func() *mockSceneClient {
				return &mockSceneClient{
					getStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
						if entityID == "scene.movie time" {
							return nil, errors.New("not found")
						}
						if entityID == "scene.movie_time" {
							return &homeassistant.Entity{
								EntityID: "scene.movie_time",
								State:    "scening",
								Attributes: map[string]any{
									"friendly_name": "Movie Time Scene",
								},
							}, nil
						}
						return nil, errors.New("not found")
					},
					listScenesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
						return []homeassistant.Entity{
							{EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
						}, nil
					},
				}
			},
			wantContains: "Movie Time Scene",
		},
		{
			name: "get by friendly_name - case insensitive",
			args: map[string]any{"action": "get", "scene_id": "MOVIE"},
			setupClient: func() *mockSceneClient {
				return &mockSceneClient{
					getStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
						if entityID == "scene.MOVIE" {
							return nil, errors.New("not found")
						}
						if entityID == "scene.movie_time" {
							return &homeassistant.Entity{
								EntityID: "scene.movie_time",
								State:    "scening",
								Attributes: map[string]any{
									"friendly_name": "Movie Time Scene",
								},
							}, nil
						}
						return nil, errors.New("not found")
					},
					listScenesFn: func(_ context.Context) ([]homeassistant.Entity, error) {
						return []homeassistant.Entity{
							{EntityID: "scene.movie_time", State: "scening", Attributes: map[string]any{"friendly_name": "Movie Time Scene"}},
						}, nil
					},
				}
			},
			wantContains: "Movie Time Scene",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewSceneHandlers()
			result, err := h.handleManageScene(context.Background(), tt.setupClient(), tt.args)
			if err != nil {
				t.Fatalf("handleManageScene() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Expected content to contain %q, got: %s", tt.wantContains, content)
			}
		})
	}
}

// TestSceneHandlers_IDNormalization tests that scene_id inputs are properly normalized
// to avoid double-prefix bugs (e.g., scene.scene.movie_night) and to resolve config IDs.
func TestSceneHandlers_IDNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		action            string
		inputID           string
		wantGetSceneID    string // For update action (configID passed to GetScene)
		wantUpdateSceneID string
		wantDeleteSceneID string
		additionalArgs    map[string]any
	}{
		{
			name:              "update - with scene. prefix",
			action:            "update",
			inputID:           "scene.movie_night",
			wantGetSceneID:    "movie_night", // GetScene takes the bare configID
			wantUpdateSceneID: "movie_night", // Should strip prefix for REST API
			additionalArgs: map[string]any{
				"name": "Updated Movie Night",
			},
		},
		{
			name:              "update - without prefix",
			action:            "update",
			inputID:           "movie_night",
			wantGetSceneID:    "movie_night",
			wantUpdateSceneID: "movie_night",
			additionalArgs: map[string]any{
				"name": "Updated Movie Night",
			},
		},
		{
			name:              "delete - with scene. prefix",
			action:            "delete",
			inputID:           "scene.movie_night",
			wantDeleteSceneID: "movie_night", // Should strip prefix for REST API
		},
		{
			name:              "delete - without prefix",
			action:            "delete",
			inputID:           "movie_night",
			wantDeleteSceneID: "movie_night",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockSceneClient{}
			h := &SceneHandlers{}

			args := map[string]any{
				"action":   tt.action,
				"scene_id": tt.inputID,
			}
			for k, v := range tt.additionalArgs {
				args[k] = v
			}

			_, err := h.handleManageScene(context.Background(), client, args)
			if err != nil {
				t.Fatalf("handleManageScene() unexpected error = %v", err)
			}

			// Verify correct IDs were used
			if tt.wantGetSceneID != "" && client.lastGetSceneID != tt.wantGetSceneID {
				t.Errorf("GetScene called with ID %q, want %q", client.lastGetSceneID, tt.wantGetSceneID)
			}
			if tt.wantUpdateSceneID != "" && client.lastUpdateSceneID != tt.wantUpdateSceneID {
				t.Errorf("UpdateScene called with ID %q, want %q", client.lastUpdateSceneID, tt.wantUpdateSceneID)
			}
			if tt.wantDeleteSceneID != "" && client.lastDeleteSceneID != tt.wantDeleteSceneID {
				t.Errorf("DeleteScene called with ID %q, want %q", client.lastDeleteSceneID, tt.wantDeleteSceneID)
			}
		})
	}
}

func TestManageScene_Patch(t *testing.T) {
	t.Parallel()

	baseConfig := &homeassistant.SceneConfig{
		Name: "Movie Night",
		Entities: map[string]homeassistant.SceneState{
			"light.living_room": {State: "on", Attributes: map[string]any{"brightness": float64(200)}},
		},
	}

	h := &SceneHandlers{}

	tests := []handlerTestCase{
		{
			name: "patch - missing scene_id",
			args: map[string]any{
				"action":     "patch",
				"operations": []any{map[string]any{"op": "replace", "path": "/name", "value": "Updated Movie Night"}},
			},
			wantError:    true,
			wantContains: []string{"scene_id is required"},
		},
		{
			name: "patch - missing operations",
			args: map[string]any{
				"action":   "patch",
				"scene_id": "movie_night",
			},
			wantError:    true,
			wantContains: []string{"operations is required"},
		},
		{
			name: "patch - success replace name",
			args: map[string]any{
				"action":   "patch",
				"scene_id": "movie_night",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/name", "value": "Updated Movie Night"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetSceneFn = func(_ context.Context, _ string) (*homeassistant.Scene, error) {
					cfg := *baseConfig
					return &homeassistant.Scene{EntityID: "scene.movie_night", Config: &cfg}, nil
				}
				m.UpdateSceneFn = func(_ context.Context, _ string, _ homeassistant.SceneConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully", "1 operations"},
		},
		{
			name: "patch - success add entity",
			args: map[string]any{
				"action":   "patch",
				"scene_id": "movie_night",
				"operations": []any{
					map[string]any{"op": "add", "path": "/entities/light.bedroom", "value": map[string]any{"state": "on"}},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetSceneFn = func(_ context.Context, _ string) (*homeassistant.Scene, error) {
					cfg := *baseConfig
					return &homeassistant.Scene{EntityID: "scene.movie_night", Config: &cfg}, nil
				}
				m.UpdateSceneFn = func(_ context.Context, _ string, _ homeassistant.SceneConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully"},
		},
		{
			name: "patch - nil config returns error",
			args: map[string]any{
				"action":   "patch",
				"scene_id": "movie_night",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/name", "value": "Updated"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetSceneFn = func(_ context.Context, _ string) (*homeassistant.Scene, error) {
					return &homeassistant.Scene{EntityID: "scene.movie_night", Config: nil}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"no configuration to patch"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageScene)
}

func TestManageScene_Patch_PreservesEntityAttributes(t *testing.T) {
	t.Parallel()

	h := &SceneHandlers{}
	client := &UniversalMockClient{}
	client.GetSceneFn = func(context.Context, string) (*homeassistant.Scene, error) {
		return &homeassistant.Scene{
			EntityID: "scene.movie_night",
			Config: &homeassistant.SceneConfig{
				Name: "Movie Night",
				Entities: map[string]homeassistant.SceneState{
					"light.living_room": {State: "on", Attributes: map[string]any{"brightness": float64(200)}},
				},
			},
		}, nil
	}
	var lastConfig homeassistant.SceneConfig
	client.UpdateSceneFn = func(_ context.Context, _ string, config homeassistant.SceneConfig) error {
		lastConfig = config
		return nil
	}

	args := map[string]any{
		"action":   "patch",
		"scene_id": "movie_night",
		"operations": []any{
			map[string]any{"op": "replace", "path": "/name", "value": "Updated Movie Night"},
		},
	}
	result, err := h.handleManageScene(context.Background(), client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	got := lastConfig.Entities["light.living_room"].Attributes["brightness"]
	if got != float64(200) {
		t.Errorf("brightness attribute = %v, want preserved 200 through patch round-trip", got)
	}
}

func TestManageScene_SemanticPatch(t *testing.T) {
	t.Parallel()

	// Scene entities are stored as a map[string]SceneState, not an array.
	// Standard JSON Pointer ops work directly: /entities/light.living_room/brightness.
	// Semantic match with 'section' still fails gracefully since entities is not an array.
	baseConfig := &homeassistant.SceneConfig{
		Name: "Movie Night",
		Entities: map[string]homeassistant.SceneState{
			"light.living_room": {State: "on", Attributes: map[string]any{"brightness": float64(200)}},
		},
	}

	h := &SceneHandlers{}

	tests := []handlerTestCase{
		{
			name: "standard patch still works - backward compat",
			args: map[string]any{
				"action":   "patch",
				"scene_id": "movie_night",
				"operations": []any{
					map[string]any{"op": "replace", "path": "/name", "value": "Cinema Night"},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetSceneFn = func(_ context.Context, _ string) (*homeassistant.Scene, error) {
					cfg := *baseConfig
					return &homeassistant.Scene{EntityID: "scene.movie_night", Config: &cfg}, nil
				}
				m.UpdateSceneFn = func(_ context.Context, _ string, _ homeassistant.SceneConfig) error {
					return nil
				}
			},
			wantError:    false,
			wantContains: []string{"patched successfully"},
		},
		{
			name: "semantic patch on map section returns clear error",
			args: map[string]any{
				"action":   "patch",
				"scene_id": "movie_night",
				"operations": []any{
					map[string]any{
						"op":      "replace",
						"match":   map[string]any{"state": "on"},
						"section": "entities",
						"field":   "state",
						"value":   "off",
					},
				},
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetSceneFn = func(_ context.Context, _ string) (*homeassistant.Scene, error) {
					cfg := *baseConfig
					return &homeassistant.Scene{EntityID: "scene.movie_night", Config: &cfg}, nil
				}
			},
			wantError:    true,
			wantContains: []string{"error applying patch", "is not an array"},
		},
	}

	runHandlerTestCases(t, tests, h.handleManageScene)
}
