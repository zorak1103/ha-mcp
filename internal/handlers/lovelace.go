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

// filterViewsByQuery filters views by title or path (case-insensitive, partial match).
func filterViewsByQuery(views []any, query string) []any {
	queryLower := strings.ToLower(query)
	filtered := make([]any, 0)

	for _, v := range views {
		viewMap, ok := v.(map[string]any)
		if !ok {
			continue
		}

		title, _ := viewMap["title"].(string)
		path, _ := viewMap["path"].(string)

		if strings.Contains(strings.ToLower(title), queryLower) ||
			strings.Contains(strings.ToLower(path), queryLower) {
			filtered = append(filtered, viewMap)
		}
	}

	return filtered
}

// countCardsInView counts all cards in a view, including cards in sections.
func countCardsInView(viewMap map[string]any) int {
	count := 0

	if cards, ok := viewMap["cards"].([]any); ok {
		count = len(cards)
	}

	if sections, ok := viewMap["sections"].([]any); ok {
		for _, section := range sections {
			if sectionMap, ok := section.(map[string]any); ok {
				if sectionCards, ok := sectionMap["cards"].([]any); ok {
					count += len(sectionCards)
				}
			}
		}
	}

	return count
}

// buildCompactViewEntry creates a compact view entry from a view map.
func buildCompactViewEntry(viewMap map[string]any) compactViewEntry {
	entry := compactViewEntry{}
	entry.Title, _ = viewMap["title"].(string)
	entry.Path, _ = viewMap["path"].(string)
	entry.Icon, _ = viewMap["icon"].(string)
	entry.Subview, _ = viewMap["subview"].(bool)
	entry.CardCount = countCardsInView(viewMap)

	if badges, ok := viewMap["badges"].([]any); ok {
		entry.BadgeCount = len(badges)
	}

	return entry
}

// buildCompactViews converts views to compact format.
func buildCompactViews(views []any) []compactViewEntry {
	compact := make([]compactViewEntry, 0, len(views))

	for _, v := range views {
		viewMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		compact = append(compact, buildCompactViewEntry(viewMap))
	}

	return compact
}

// formatLovelaceResponse creates a ToolsCallResult with JSON-formatted data.
func formatLovelaceResponse(data any, summary string) (*mcp.ToolsCallResult, error) {
	output, err := json.MarshalIndent(data, "", "  ")
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
			mcp.NewTextContent(summary + "\n\n" + string(output)),
		},
	}, nil
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

	viewFilter, _ := args["view"].(string)
	verbose, _ := args["verbose"].(bool)
	views, _ := config["views"].([]any)

	if viewFilter != "" {
		return h.handleFilteredViews(views, viewFilter)
	}

	if verbose {
		summary := fmt.Sprintf("Lovelace configuration with %d views", len(views))
		return formatLovelaceResponse(config, summary)
	}

	return h.handleCompactViews(views)
}

// handleFilteredViews handles requests with a view filter.
func (h *LovelaceHandlers) handleFilteredViews(views []any, filter string) (*mcp.ToolsCallResult, error) {
	filteredViews := filterViewsByQuery(views, filter)

	if len(filteredViews) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("No views found matching '%s'", filter)),
			},
		}, nil
	}

	summary := fmt.Sprintf("Found %d view(s) matching '%s'", len(filteredViews), filter)
	return formatLovelaceResponse(filteredViews, summary)
}

// handleCompactViews handles requests for compact view output.
func (h *LovelaceHandlers) handleCompactViews(views []any) (*mcp.ToolsCallResult, error) {
	compact := buildCompactViews(views)
	summary := fmt.Sprintf("Found %d views", len(compact)) + VerboseHint
	return formatLovelaceResponse(compact, summary)
}
