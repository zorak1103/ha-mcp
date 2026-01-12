// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// MediaHandlers provides handlers for Home Assistant media operations.
type MediaHandlers struct{}

// NewMediaHandlers creates a new MediaHandlers instance.
func NewMediaHandlers() *MediaHandlers {
	return &MediaHandlers{}
}

// RegisterTools registers all media-related tools with the registry.
func (h *MediaHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.signPathTool(), h.handleSignPath)
	registry.RegisterTool(h.getCameraStreamTool(), h.handleGetCameraStream)
	registry.RegisterTool(h.browseMediaTool(), h.handleBrowseMedia)
}

// signPathTool returns the tool definition for signing media paths.
func (h *MediaHandlers) signPathTool() mcp.Tool {
	return mcp.Tool{
		Name:        "sign_media_path",
		Description: "Sign a media path to generate a temporary authenticated URL. Useful for accessing protected media files like camera snapshots or local media.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"path": {
					Type:        "string",
					Description: "The media path to sign (e.g., '/api/camera_proxy/camera.front_door')",
				},
				"expires": {
					Type:        "integer",
					Description: "Expiration time in seconds (default: 30)",
				},
			},
			Required: []string{"path"},
		},
	}
}

// handleSignPath handles requests to sign a media path.
func (h *MediaHandlers) handleSignPath(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent("Error: 'path' parameter is required"),
			},
			IsError: true,
		}, nil
	}

	expires := 30 // default
	if exp, ok := args["expires"].(float64); ok {
		expires = int(exp)
	}

	signedPath, err := client.SignPath(ctx, path, expires)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error signing path: %v", err)),
			},
			IsError: true,
		}, nil
	}

	result := map[string]string{
		"signed_path": signedPath,
		"expires_in":  fmt.Sprintf("%d seconds", expires),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(string(output)),
		},
	}, nil
}

// getCameraStreamTool returns the tool definition for getting camera stream URLs.
func (h *MediaHandlers) getCameraStreamTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_camera_stream",
		Description: "Get the streaming URL for a camera entity. Returns HLS stream URL for live camera feeds.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The camera entity ID (e.g., 'camera.front_door')",
				},
			},
			Required: []string{"entity_id"},
		},
	}
}

// handleGetCameraStream handles requests to get camera stream information.
func (h *MediaHandlers) handleGetCameraStream(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	entityID, ok := args["entity_id"].(string)
	if !ok || entityID == "" {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent("Error: 'entity_id' parameter is required"),
			},
			IsError: true,
		}, nil
	}

	streamInfo, err := client.GetCameraStream(ctx, entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting camera stream: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(streamInfo, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(string(output)),
		},
	}, nil
}

// browseMediaTool returns the tool definition for browsing media.
func (h *MediaHandlers) browseMediaTool() mcp.Tool {
	return mcp.Tool{
		Name:        "browse_media",
		Description: "Browse media content in Home Assistant. Navigate through media sources, folders, and files available in the media browser.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"media_content_id": {
					Type:        "string",
					Description: "The media content ID to browse. Leave empty to list root media sources.",
				},
			},
		},
	}
}

// handleBrowseMedia handles requests to browse media content.
func (h *MediaHandlers) handleBrowseMedia(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	mediaContentID := ""
	if id, ok := args["media_content_id"].(string); ok {
		mediaContentID = id
	}

	result, err := client.BrowseMedia(ctx, mediaContentID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error browsing media: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(string(output)),
		},
	}, nil
}
