package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TestManageCameraSchema verifies the schema for manage_camera tool.
func TestManageCameraSchema(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterCameraTools(registry)

	tool, exists := registry.GetTool("manage_camera")
	if !exists {
		t.Fatal("manage_camera tool not registered")
	}

	// Verify basic properties
	if tool.Name != "manage_camera" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_camera")
	}

	// Verify schema
	schema := tool.InputSchema
	props := schema.Properties

	// Check action field
	actionSchema, ok := props["action"]
	if !ok {
		t.Fatal("action property missing from schema")
	}
	if len(actionSchema.Enum) != 2 {
		t.Errorf("action enum count = %d, want 2 (snapshot, stream)", len(actionSchema.Enum))
	}

	// Check required fields
	if len(schema.Required) != 2 {
		t.Errorf("required count = %d, want 2 (action, entity_id)", len(schema.Required))
	}
}

// TestManageCamera_Snapshot verifies snapshot action returns image content.
func TestManageCamera_Snapshot(t *testing.T) {
	t.Parallel()

	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG header
	client := &UniversalMockClient{
		GetCameraSnapshotFn: func(context.Context, string) ([]byte, string, error) {
			return imageData, "image/jpeg", nil
		},
	}

	handler := NewCameraHandlers()
	result, err := handler.HandleManageCamera(context.Background(), client, map[string]any{
		"action":    "snapshot",
		"entity_id": "camera.front_door",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	// Verify image content block
	content := result.Content[0]
	if content.Type != "image" {
		t.Errorf("content type = %q, want %q", content.Type, "image")
	}
	if content.MimeType != "image/jpeg" {
		t.Errorf("mime type = %q, want %q", content.MimeType, "image/jpeg")
	}

	// Verify base64 encoding
	expectedData := base64.StdEncoding.EncodeToString(imageData)
	if content.Data != expectedData {
		t.Errorf("data mismatch, want base64 encoded image data")
	}
}

// TestManageCamera_Stream verifies stream action.
func TestManageCamera_Stream(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCameraStreamFn: func(context.Context, string) (*homeassistant.StreamInfo, error) {
			return &homeassistant.StreamInfo{
				URL: "http://localhost:8123/api/hls/stream.m3u8",
			}, nil
		},
	}

	handler := NewCameraHandlers()
	result, err := handler.HandleManageCamera(context.Background(), client, map[string]any{
		"action":    "stream",
		"entity_id": "camera.backyard",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "stream.m3u8") {
		t.Errorf("result text does not contain stream URL: %s", text)
	}
}

// TestManageCamera_MissingEntityID verifies validation.
func TestManageCamera_MissingEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "snapshot missing entity_id",
			args: map[string]any{
				"action": "snapshot",
			},
		},
		{
			name: "stream missing entity_id",
			args: map[string]any{
				"action": "stream",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{}
			handler := NewCameraHandlers()

			result, err := handler.HandleManageCamera(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil || !result.IsError {
				t.Error("expected error result")
			}
		})
	}
}

// TestManageCamera_InvalidAction verifies invalid action handling.
func TestManageCamera_InvalidAction(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewCameraHandlers()

	result, err := handler.HandleManageCamera(context.Background(), client, map[string]any{
		"action":    "invalid",
		"entity_id": "camera.test",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageCamera_Stream_JSONFormat verifies stream action with JSON format.
func TestManageCamera_Stream_JSONFormat(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCameraStreamFn: func(context.Context, string) (*homeassistant.StreamInfo, error) {
			return &homeassistant.StreamInfo{
				URL: "http://localhost:8123/api/hls/stream.m3u8",
			}, nil
		},
	}

	handler := NewCameraHandlers()
	result, err := handler.HandleManageCamera(context.Background(), client, map[string]any{
		"action":    "stream",
		"entity_id": "camera.backyard",
		"format":    "json",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "stream.m3u8") {
		t.Errorf("JSON result does not contain stream URL: %s", text)
	}
}

// TestManageCamera_Stream_Error verifies stream action handles client error.
func TestManageCamera_Stream_Error(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCameraStreamFn: func(context.Context, string) (*homeassistant.StreamInfo, error) {
			return nil, fmt.Errorf("stream unavailable")
		},
	}

	handler := NewCameraHandlers()
	result, err := handler.HandleManageCamera(context.Background(), client, map[string]any{
		"action":    "stream",
		"entity_id": "camera.backyard",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageCamera_Snapshot_Error verifies snapshot handles client error.
func TestManageCamera_Snapshot_Error(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCameraSnapshotFn: func(context.Context, string) ([]byte, string, error) {
			return nil, "", fmt.Errorf("snapshot failed")
		},
	}

	handler := NewCameraHandlers()
	result, err := handler.HandleManageCamera(context.Background(), client, map[string]any{
		"action":    "snapshot",
		"entity_id": "camera.front_door",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}
