package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockMediaClient implements homeassistant.Client for testing.
type mockMediaClient struct {
	homeassistant.Client
	signPathFn    func(ctx context.Context, path string, expires int) (string, error)
	browseMediaFn func(ctx context.Context, mediaContentID string) (*homeassistant.MediaBrowseResult, error)
}

func (m *mockMediaClient) SignPath(ctx context.Context, path string, expires int) (string, error) {
	if m.signPathFn != nil {
		return m.signPathFn(ctx, path, expires)
	}
	return "", nil
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
	const expectedToolCount = 2
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	expectedTools := map[string]bool{
		"sign_media_path": false,
		"browse_media":    false,
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
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
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
			if tt.wantContains != "" && !strings.Contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestMediaHandlers_handleSignPath_RejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{"zero expires", map[string]any{"path": "/media/local/a.mp3", "expires": float64(0)}, "'expires' must be between 1 and 300"},
		{"negative expires", map[string]any{"path": "/media/local/a.mp3", "expires": float64(-5)}, "'expires' must be between 1 and 300"},
		{"expires above cap", map[string]any{"path": "/media/local/a.mp3", "expires": float64(301)}, "'expires' must be between 1 and 300"},
		{"huge expires", map[string]any{"path": "/media/local/a.mp3", "expires": float64(1e18)}, "'expires' must be between 1 and 300"},
		{"fractional expires", map[string]any{"path": "/media/local/a.mp3", "expires": 30.5}, "'expires' must be a whole number"},
		{"string expires", map[string]any{"path": "/media/local/a.mp3", "expires": "30"}, "'expires' must be a whole number"},
		{"arbitrary api path", map[string]any{"path": "/api/states"}, "not allowed"},
		{"api config path", map[string]any{"path": "/api/config/core/check_config"}, "not allowed"},
		{"prefix without trailing slash", map[string]any{"path": "/media"}, "not allowed"},
		{"look-alike prefix", map[string]any{"path": "/media_evil/x"}, "not allowed"},
		{"relative path", map[string]any{"path": "api/camera_proxy/camera.front_door"}, "not allowed"},
		{"absolute url", map[string]any{"path": "http://evil.example/media/local/a.mp3"}, "not allowed"},
		{"scheme-relative url", map[string]any{"path": "//evil.example/media/local/a.mp3"}, "not allowed"},
		{"dot-dot traversal", map[string]any{"path": "/media/../api/states"}, "traversal"},
		{"encoded dot-dot traversal", map[string]any{"path": "/media/%2e%2e/api/states"}, "traversal"},
		{"encoded slash traversal", map[string]any{"path": "/media/..%2fapi/states"}, "traversal"},
		{"backslash", map[string]any{"path": "/media/..\api/states"}, "invalid"},
		{"fragment", map[string]any{"path": "/media/local/a.mp3#x"}, "invalid"},
		{"control character", map[string]any{"path": "/media/local/a\n.mp3"}, "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockMediaClient{
				signPathFn: func(_ context.Context, path string, _ int) (string, error) {
					t.Errorf("SignPath must not be called for rejected input, got path %q", path)
					return "", nil
				},
			}

			result, err := NewMediaHandlers().handleSignPath(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleSignPath() returned unexpected error: %v", err)
			}
			if result == nil || !result.IsError || len(result.Content) == 0 {
				t.Fatalf("expected error result, got %+v", result)
			}
			if got := result.Content[0].Text; !strings.Contains(got, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}

func TestMediaHandlers_handleSignPath_AcceptsValidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        map[string]any
		wantPath    string
		wantExpires int
	}{
		{"camera proxy default expires", map[string]any{"path": "/api/camera_proxy/camera.front_door"}, "/api/camera_proxy/camera.front_door", 30},
		{"null expires uses default", map[string]any{"path": "/media/local/a.mp3", "expires": nil}, "/media/local/a.mp3", 30},
		{"minimum expires", map[string]any{"path": "/media/local/a.mp3", "expires": float64(1)}, "/media/local/a.mp3", 1},
		{"maximum expires", map[string]any{"path": "/media/local/a.mp3", "expires": float64(300)}, "/media/local/a.mp3", 300},
		{"query string", map[string]any{"path": "/api/camera_proxy/camera.front_door?width=640"}, "/api/camera_proxy/camera.front_door?width=640", 30},
		{"dots inside a file name", map[string]any{"path": "/media/local/my..song.mp3"}, "/media/local/my..song.mp3", 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string
			var gotExpires int
			client := &mockMediaClient{
				signPathFn: func(_ context.Context, path string, expires int) (string, error) {
					gotPath, gotExpires = path, expires
					return path + "?authSig=x", nil
				},
			}

			result, err := NewMediaHandlers().handleSignPath(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleSignPath() returned unexpected error: %v", err)
			}
			if result == nil || result.IsError {
				t.Fatalf("expected success, got %+v", result)
			}
			if gotPath != tt.wantPath || gotExpires != tt.wantExpires {
				t.Errorf("SignPath called with (%q, %d), want (%q, %d)", gotPath, gotExpires, tt.wantPath, tt.wantExpires)
			}
		})
	}
}

func TestMediaHandlers_signPathTool_DocumentsLimits(t *testing.T) {
	t.Parallel()

	tool := NewMediaHandlers().signPathTool()

	if desc := tool.InputSchema.Properties["expires"].Description; !strings.Contains(desc, "1-300") {
		t.Errorf("expires description %q does not document the 1-300 range", desc)
	}
	for _, prefix := range []string{"/api/camera_proxy/", "/media/"} {
		if !strings.Contains(tool.InputSchema.Properties["path"].Description, prefix) {
			t.Errorf("path description does not mention allowed prefix %q", prefix)
		}
	}
}
