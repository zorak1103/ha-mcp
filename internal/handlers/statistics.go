// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/mcp"
)

// StatisticsHandlers provides MCP tools for Home Assistant statistics operations.
type StatisticsHandlers struct{}

// NewStatisticsHandlers creates a new StatisticsHandlers instance.
func NewStatisticsHandlers() *StatisticsHandlers {
	return &StatisticsHandlers{}
}

// RegisterTools registers all statistics-related tools with the registry.
func (h *StatisticsHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getStatisticsTool(), h.handleGetStatistics)
}

// getStatisticsTool returns the tool definition for getting statistics.
func (h *StatisticsHandlers) getStatisticsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_statistics",
		Description: "Get historical statistics for entities (long-term data like energy consumption, temperature averages)",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"statistic_ids": {
					Type:        "array",
					Description: "List of statistic IDs to retrieve (e.g., 'sensor.energy_consumption')",
				},
				"period": {
					Type:        "string",
					Description: "Statistics period granularity: 5minute, hour, day, week, or month (default: hour)",
				},
			},
			Required: []string{"statistic_ids"},
		},
	}
}

// handleGetStatistics retrieves historical statistics for specified entities.
func (h *StatisticsHandlers) handleGetStatistics(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	// Extract statistic_ids (required)
	statIDsRaw, ok := args["statistic_ids"]
	if !ok {
		return nil, fmt.Errorf("statistic_ids is required")
	}

	statIDsSlice, ok := statIDsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("statistic_ids must be an array")
	}

	statIDs := make([]string, 0, len(statIDsSlice))
	for _, id := range statIDsSlice {
		if s, ok := id.(string); ok {
			statIDs = append(statIDs, s)
		}
	}

	if len(statIDs) == 0 {
		return nil, fmt.Errorf("at least one statistic_id is required")
	}

	// Extract period (optional, default to "hour")
	period := "hour"
	if p, ok := args["period"].(string); ok && p != "" {
		period = p
	}

	// Call the client method
	statistics, err := client.GetStatistics(ctx, statIDs, period)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Failed to get statistics: %v", err),
			}},
			IsError: true,
		}, nil
	}

	// Format response
	result, err := json.MarshalIndent(statistics, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling statistics result: %w", err)
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{
			Type: "text",
			Text: string(result),
		}},
	}, nil
}
