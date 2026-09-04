package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Action constants for manage_camera tool.
const (
	cameraActionSnapshot = "snapshot"
	cameraActionStream   = "stream"
)

// CameraHandlers provides handlers for camera operations.
type CameraHandlers struct{}

// NewCameraHandlers creates a new camera handlers instance.
func NewCameraHandlers() *CameraHandlers {
	return &CameraHandlers{}
}

// RegisterCameraTools registers camera-related tools with the MCP registry.
func RegisterCameraTools(registry *mcp.Registry) {
	handler := NewCameraHandlers()

	registry.RegisterTool(mcp.Tool{
		Name:        "manage_camera",
		Description: "Access camera snapshots and streams. Supports snapshot (get current image) and stream (get stream URL).",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Action to perform: 'snapshot' (get current image) or 'stream' (get stream URL).",
					Enum:        []string{cameraActionSnapshot, cameraActionStream},
				},
				attrEntityID: {
					Type:        "string",
					Description: "Camera entity ID (e.g., 'camera.front_door').",
				},
				"format": {
					Type:        "string",
					Description: "Output format: 'natural' (default, human-readable) or 'json' (structured JSON). Note: snapshot always returns image content.",
					Enum:        []string{"natural", "json"},
					Default:     "natural",
				},
			},
			Required: []string{"action", attrEntityID},
		},
	}, handler.HandleManageCamera)
}

// HandleManageCamera handles the manage_camera tool invocation.
func (h *CameraHandlers) HandleManageCamera(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	// Extract action
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action parameter is required and must be 'snapshot' or 'stream'"), nil
	}

	// Validate entity_id
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required"), nil
	}

	// Extract format (default: natural)
	format, _ := args["format"].(string)
	if format == "" {
		format = formatNatural
	}

	// Route to action handler
	switch action {
	case cameraActionSnapshot:
		return h.handleSnapshot(ctx, client, entityID)
	case cameraActionStream:
		return h.handleStream(ctx, client, entityID, format)
	default:
		return errorResult(fmt.Sprintf("invalid action %q, must be one of: snapshot, stream", action)), nil
	}
}

// handleSnapshot retrieves a camera snapshot as an image content block.
func (h *CameraHandlers) handleSnapshot(ctx context.Context, client homeassistant.Client, entityID string) (*mcp.ToolsCallResult, error) {
	// Get snapshot via REST API
	imageData, contentType, err := client.GetCameraSnapshot(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get camera snapshot: %v", err)), nil
	}

	// Base64 encode the image data
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	// Return as image content block
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewImageContent(base64Data, contentType),
		},
	}, nil
}

// handleStream retrieves a camera stream URL.
func (h *CameraHandlers) handleStream(ctx context.Context, client homeassistant.Client, entityID, format string) (*mcp.ToolsCallResult, error) {
	// Get stream via WebSocket
	streamInfo, err := client.GetCameraStream(ctx, entityID)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get camera stream: %v", err)), nil
	}

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(streamInfo, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal stream info: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format
	return successResult(fmt.Sprintf("Stream URL for %s:\n%s", entityID, streamInfo.URL)), nil
}
