package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockMediaClient implements homeassistant.Client for testing.
type mockMediaClient struct {
	homeassistant.Client
	signPathFn        func(ctx context.Context, path string, expires int) (string, error)
	getCameraStreamFn func(ctx context.Context, entityID string) (*homeassistant.StreamInfo, error)
	browseMediaFn     func(ctx context.Context, mediaContentID string) (*homeassistant.MediaBrowseResult, error)
}

func (m *mockMediaClient) SignPath(ctx context.Context, path string, expires int) (string, error) {
	if m.signPathFn != nil {
		return m.signPathFn(ctx, path, expires)
	}
	return "", nil
}

func (m *mockMediaClient) GetCameraStream(ctx context.Context, entityID string) (*homeassistant.StreamInfo, error) {
	if m.getCameraStreamFn != nil {
		return m.getCameraStreamFn(ctx, entityID)
	}
	return &homeassistant.StreamInfo{}, nil
}

func (m *mockMediaClient) BrowseMedia(ctx context.Context, mediaContentID string) (*homeassistant.MediaBrowseResult, error) {
	if m.browseMediaFn != nil {
		return m.browseMediaFn(ctx, mediaContentID)
	}
	return &homeassistant.MediaBrowseResult{}, nil
}

func TestNewMediaHandlers(t *testing.T) {
	t.Parallel()

	h := NewMediaHandlers()
	if h == nil {
		t.Error("NewMediaHandlers() returned nil")
	}
}

func TestMediaHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewMediaHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 3
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	expectedTools := map[string]bool{
		"sign_media_path":   false,
		"get_camera_stream": false,
		"browse_media":      false,
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

func TestMediaHandlers_signPathTool(t *testing.T) {
	t.Parallel()

	h := NewMediaHandlers()
	tool := h.signPathTool()

	if tool.Name != "sign_media_path" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "sign_media_path")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Check required fields
	requiredFields := map[string]bool{"path": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}

	// Check optional properties exist
	if _, ok := tool.InputSchema.Properties["expires"]; !ok {
		t.Error("Property 'expires' not found in schema")
	}
}

func TestMediaHandlers_getCameraStreamTool(t *testing.T) {
	t.Parallel()

	h := NewMediaHandlers()
	tool := h.getCameraStreamTool()

	if tool.Name != "get_camera_stream" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "get_camera_stream")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Check required fields
	requiredFields := map[string]bool{"entity_id": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}
}

func TestMediaHandlers_browseMediaTool(t *testing.T) {
	t.Parallel()

	h := NewMediaHandlers()
	tool := h.browseMediaTool()

	if tool.Name != "browse_media" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "browse_media")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Check optional properties exist
	if _, ok := tool.InputSchema.Properties["media_content_id"]; !ok {
		t.Error("Property 'media_content_id' not found in schema")
	}

	// No required fields for browse_media
	if len(tool.InputSchema.Required) != 0 {
		t.Errorf("Expected no required fields, got %d", len(tool.InputSchema.Required))
	}
}

func TestMediaHandlers_handleSignPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		signPathErr  error
		signPathResp string
		wantError    bool
		wantContains string
	}{
		{
			name: "success with default expires",
			args: map[string]any{
				"path": "/api/camera_proxy/camera.front_door",
			},
			signPathResp: "/api/camera_proxy/camera.front_door?authSig=abc123",
			wantError:    false,
			wantContains: "signed_path",
		},
		{
			name: "success with custom expires",
			args: map[string]any{
				"path":    "/api/camera_proxy/camera.front_door",
				"expires": float64(60),
			},
			signPathResp: "/api/camera_proxy/camera.front_door?authSig=abc123&expires=60",
			wantError:    false,
			wantContains: "60 seconds",
		},
		{
			name:         "missing path",
			args:         map[string]any{},
			wantError:    true,
			wantContains: "'path' parameter is required",
		},
		{
			name: "empty path",
			args: map[string]any{
				"path": "",
			},
			wantError:    true,
			wantContains: "'path' parameter is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"path": "/api/camera_proxy/camera.front_door",
			},
			signPathErr:  errors.New("signing failed"),
			wantError:    true,
			wantContains: "Error signing path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockMediaClient{
				signPathFn: func(_ context.Context, _ string, _ int) (string, error) {
					if tt.signPathErr != nil {
						return "", tt.signPathErr
					}
					return tt.signPathResp, nil
				},
			}

			h := NewMediaHandlers()
			result, err := h.handleSignPath(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleSignPath() returned unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("handleSignPath() returned nil result")
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

func TestMediaHandlers_handleGetCameraStream(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		args                map[string]any
		getCameraStreamErr  error
		getCameraStreamResp *homeassistant.StreamInfo
		wantError           bool
		wantContains        string
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": "camera.front_door",
			},
			getCameraStreamResp: &homeassistant.StreamInfo{
				URL: "http://192.168.1.100:8123/api/hls/camera.front_door/master.m3u8",
			},
			wantError:    false,
			wantContains: "url",
		},
		{
			name:         "missing entity_id",
			args:         map[string]any{},
			wantError:    true,
			wantContains: "'entity_id' parameter is required",
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
			},
			wantError:    true,
			wantContains: "'entity_id' parameter is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "camera.front_door",
			},
			getCameraStreamErr: errors.New("camera unavailable"),
			wantError:          true,
			wantContains:       "Error getting camera stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockMediaClient{
				getCameraStreamFn: func(_ context.Context, _ string) (*homeassistant.StreamInfo, error) {
					if tt.getCameraStreamErr != nil {
						return nil, tt.getCameraStreamErr
					}
					return tt.getCameraStreamResp, nil
				},
			}

			h := NewMediaHandlers()
			result, err := h.handleGetCameraStream(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleGetCameraStream() returned unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("handleGetCameraStream() returned nil result")
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

func TestMediaHandlers_handleBrowseMedia(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		browseMediaErr  error
		browseMediaResp *homeassistant.MediaBrowseResult
		wantError       bool
		wantContains    string
	}{
		{
			name: "browse root",
			args: map[string]any{},
			browseMediaResp: &homeassistant.MediaBrowseResult{
				Title:      "Media",
				MediaClass: "directory",
				CanExpand:  true,
				Children: []*homeassistant.MediaBrowseResult{
					{
						Title:          "Local Media",
						MediaClass:     "directory",
						MediaContentID: "media-source://media_source/local",
						CanExpand:      true,
					},
				},
			},
			wantError:    false,
			wantContains: "Media",
		},
		{
			name: "browse with content_id",
			args: map[string]any{
				"media_content_id": "media-source://media_source/local",
			},
			browseMediaResp: &homeassistant.MediaBrowseResult{
				Title:          "Local Media",
				MediaClass:     "directory",
				MediaContentID: "media-source://media_source/local",
				CanExpand:      true,
				Children: []*homeassistant.MediaBrowseResult{
					{
						Title:            "Music",
						MediaClass:       "music",
						MediaContentID:   "media-source://media_source/local/music",
						MediaContentType: "audio/mpeg",
						CanPlay:          true,
					},
				},
			},
			wantError:    false,
			wantContains: "Local Media",
		},
		{
			name: "browse playable item",
			args: map[string]any{
				"media_content_id": "media-source://media_source/local/music/song.mp3",
			},
			browseMediaResp: &homeassistant.MediaBrowseResult{
				Title:            "Song",
				MediaClass:       "music",
				MediaContentID:   "media-source://media_source/local/music/song.mp3",
				MediaContentType: "audio/mpeg",
				CanPlay:          true,
				CanExpand:        false,
				Thumbnail:        "/api/media_source/local/thumbnail/song.jpg",
			},
			wantError:    false,
			wantContains: "can_play",
		},
		{
			name: "client error",
			args: map[string]any{
				"media_content_id": "invalid://source",
			},
			browseMediaErr: errors.New("media source not found"),
			wantError:      true,
			wantContains:   "Error browsing media",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockMediaClient{
				browseMediaFn: func(_ context.Context, _ string) (*homeassistant.MediaBrowseResult, error) {
					if tt.browseMediaErr != nil {
						return nil, tt.browseMediaErr
					}
					return tt.browseMediaResp, nil
				},
			}

			h := NewMediaHandlers()
			result, err := h.handleBrowseMedia(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleBrowseMedia() returned unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("handleBrowseMedia() returned nil result")
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
