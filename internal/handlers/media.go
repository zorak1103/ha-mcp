// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
	"unicode"

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
	registry.RegisterTool(h.browseMediaTool(), h.handleBrowseMedia)
}

// signPathTool returns the tool definition for signing media paths.
func (h *MediaHandlers) signPathTool() mcp.Tool {
	return mcp.Tool{
		Name:        "sign_media_path",
		Description: "Sign a media path to generate a temporary authenticated URL. Useful for accessing protected media files like camera snapshots or local media. Only /api/camera_proxy/ and /media/ paths can be signed, for at most 300 seconds.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"path": {
					Type:        "string",
					Description: "The media path to sign. Must start with '/api/camera_proxy/' or '/media/' (e.g., '/api/camera_proxy/camera.front_door')",
				},
				"expires": {
					Type:        "integer",
					Description: "Expiration time in seconds, 1-300 (default: 30)",
				},
			},
			Required: []string{"path"},
		},
	}
}

// sign_media_path limits. The tool is classified as a read tool, so it stays
// available under read_only; bounding expiry and path keeps it from minting
// long-lived, token-free URLs for arbitrary API endpoints there.
const (
	signPathDefaultExpires = 30
	signPathMaxExpires     = 300
)

// signPathAllowedPrefixes lists the only path prefixes sign_media_path signs.
var signPathAllowedPrefixes = []string{"/api/camera_proxy/", "/media/"}

// parseSignPathExpires reads the optional expires argument (seconds). A missing
// or null value yields the default; anything that is not a whole number in
// 1..signPathMaxExpires is rejected.
func parseSignPathExpires(args map[string]any) (int, error) {
	raw, present := args["expires"]
	if !present || raw == nil {
		return signPathDefaultExpires, nil
	}

	f, ok := raw.(float64)
	if !ok || f != math.Trunc(f) {
		return 0, errors.New("'expires' must be a whole number of seconds")
	}
	if f < 1 || f > signPathMaxExpires {
		return 0, fmt.Errorf("'expires' must be between 1 and %d seconds", signPathMaxExpires)
	}

	return int(f), nil
}

// hasUnsafePathChars reports whether s contains a backslash, a fragment marker
// or a control character, none of which belong in a signable path.
func hasUnsafePathChars(s string) bool {
	return strings.ContainsAny(s, "\\#") || strings.ContainsFunc(s, unicode.IsControl)
}

// validateSignPath accepts only paths under signPathAllowedPrefixes. It checks
// the decoded path too, so percent-encoded dot segments cannot climb out of
// the allowed prefix.
func validateSignPath(p string) error {
	if hasUnsafePathChars(p) {
		return errors.New("'path' contains invalid characters")
	}

	if !slices.ContainsFunc(signPathAllowedPrefixes, func(prefix string) bool {
		return strings.HasPrefix(p, prefix)
	}) {
		return fmt.Errorf("'path' is not allowed: must start with one of %s", strings.Join(signPathAllowedPrefixes, ", "))
	}

	u, err := url.Parse(p)
	if err != nil || hasUnsafePathChars(u.Path) {
		return errors.New("'path' contains invalid characters")
	}

	for seg := range strings.SplitSeq(u.Path, "/") {
		if seg == "." || seg == ".." {
			return errors.New("'path' must not contain path traversal segments")
		}
	}

	return nil
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

	expires, err := parseSignPathExpires(args)
	if err != nil {
		return errorResult("Error: " + err.Error()), nil //nolint:nilerr // invalid arguments are reported as a tool error result
	}

	if err = validateSignPath(path); err != nil {
		return errorResult("Error: " + err.Error()), nil //nolint:nilerr // invalid arguments are reported as a tool error result
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
