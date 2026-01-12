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

// LovelaceHandlers provides handlers for Home Assistant Lovelace dashboard operations.
type LovelaceHandlers struct{}

// NewLovelaceHandlers creates a new LovelaceHandlers instance.
func NewLovelaceHandlers() *LovelaceHandlers {
	return &LovelaceHandlers{}
}

// RegisterTools registers all lovelace-related tools with the registry.
func (h *LovelaceHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getLovelaceConfigTool(), h.handleGetLovelaceConfig)
}

// getLovelaceConfigTool returns the tool definition for getting Lovelace configuration.
func (h *LovelaceHandlers) getLovelaceConfigTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_lovelace_config",
		Description: "Get the Lovelace dashboard configuration. By default returns a compact overview of views. Use 'view' filter to get a specific view, and 'verbose' for full details.",
		InputSchema: mcp.JSONSchema{
			Type:        "object",
			Description: "Filter and output options for Lovelace configuration",
			Properties: map[string]mcp.JSONSchema{
				"view": {
					Type:        "string",
					Description: "Filter by view path or title (case-insensitive, partial match). Returns full details for matching views.",
				},
				"verbose": {
					Type:        "boolean",
					Description: "If true, return full configuration including all cards. Default: false (compact overview with view names and card counts)",
				},
			},
		},
	}
}

// compactViewEntry represents a minimal view entry for compact output.
type compactViewEntry struct {
	Title      string `json:"title,omitempty"`
	Path       string `json:"path,omitempty"`
	Icon       string `json:"icon,omitempty"`
	CardCount  int    `json:"card_count"`
	BadgeCount int    `json:"badge_count,omitempty"`
	Subview    bool   `json:"subview,omitempty"`
}

// handleGetLovelaceConfig handles requests to get Lovelace dashboard configuration.
func (h *LovelaceHandlers) handleGetLovelaceConfig(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	config, err := client.GetLovelaceConfig(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting Lovelace config: %v", err)),
			},
			IsError: true,
		}, nil
	}

	// Parse filter parameters
	viewFilter, _ := args["view"].(string)
	verbose, _ := args["verbose"].(bool)

	// Extract views from config (config is already map[string]any)
	views, _ := config["views"].([]any)

	// If view filter is specified, return matching views with full details
	if viewFilter != "" {
		viewFilterLower := strings.ToLower(viewFilter)
		filteredViews := make([]any, 0)

		for _, v := range views {
			viewMap, ok := v.(map[string]any)
			if !ok {
				continue
			}

			title, _ := viewMap["title"].(string)
			path, _ := viewMap["path"].(string)

			// Match by title or path (case-insensitive, partial match)
			if strings.Contains(strings.ToLower(title), viewFilterLower) ||
				strings.Contains(strings.ToLower(path), viewFilterLower) {
				filteredViews = append(filteredViews, viewMap)
			}
		}

		if len(filteredViews) == 0 {
			return &mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{
					mcp.NewTextContent(fmt.Sprintf("No views found matching '%s'", viewFilter)),
				},
			}, nil
		}

		output, marshalErr := json.MarshalIndent(filteredViews, "", "  ")
		if marshalErr != nil {
			return &mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{
					mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", marshalErr)),
				},
				IsError: true,
			}, nil
		}

		summary := fmt.Sprintf("Found %d view(s) matching '%s'", len(filteredViews), viewFilter)
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(summary + "\n\n" + string(output)),
			},
		}, nil
	}

	// Verbose mode: return full configuration
	if verbose {
		output, marshalErr := json.MarshalIndent(config, "", "  ")
		if marshalErr != nil {
			return &mcp.ToolsCallResult{
				Content: []mcp.ContentBlock{
					mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", marshalErr)),
				},
				IsError: true,
			}, nil
		}

		summary := fmt.Sprintf("Lovelace configuration with %d views", len(views))
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(summary + "\n\n" + string(output)),
			},
		}, nil
	}

	// Compact mode: return view overview
	compact := make([]compactViewEntry, 0, len(views))
	for _, v := range views {
		viewMap, ok := v.(map[string]any)
		if !ok {
			continue
		}

		entry := compactViewEntry{}
		entry.Title, _ = viewMap["title"].(string)
		entry.Path, _ = viewMap["path"].(string)
		entry.Icon, _ = viewMap["icon"].(string)
		entry.Subview, _ = viewMap["subview"].(bool)

		// Count cards (from both "cards" and "sections")
		if cards, ok := viewMap["cards"].([]any); ok {
			entry.CardCount = len(cards)
		}
		if sections, ok := viewMap["sections"].([]any); ok {
			for _, section := range sections {
				if sectionMap, ok := section.(map[string]any); ok {
					if sectionCards, ok := sectionMap["cards"].([]any); ok {
						entry.CardCount += len(sectionCards)
					}
				}
			}
		}

		// Count badges
		if badges, ok := viewMap["badges"].([]any); ok {
			entry.BadgeCount = len(badges)
		}

		compact = append(compact, entry)
	}

	output, marshalErr := json.MarshalIndent(compact, "", "  ")
	if marshalErr != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error formatting response: %v", marshalErr)),
			},
			IsError: true,
		}, nil
	}

	summary := fmt.Sprintf("Found %d views", len(compact))
	summary += VerboseHint

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{
			mcp.NewTextContent(summary + "\n\n" + string(output)),
		},
	}, nil
}
