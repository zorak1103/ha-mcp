// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
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
		Description: "Get historical statistics for entities (long-term data like energy consumption, temperature averages). Supports pagination via 'limit' and 'cursor' parameters.",
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
				"limit": {
					Type:        "integer",
					Description: "Maximum number of statistics results to return (max 1000, default: no limit). Use with 'cursor' for pagination.",
				},
				"cursor": {
					Type:        "string",
					Description: "Pagination cursor from previous response to get next page of results",
				},
			},
			Required: []string{"statistic_ids"},
		},
	}
}

// parseStatisticIDs extracts and validates statistic_ids from args.
func parseStatisticIDs(args map[string]any) ([]string, error) {
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

	return statIDs, nil
}

// handleGetStatistics retrieves historical statistics for specified entities.
func (h *StatisticsHandlers) handleGetStatistics(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	statIDs, err := parseStatisticIDs(args)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(err.Error())},
			IsError: true,
		}, nil
	}

	period := "hour"
	if p, ok := args["period"].(string); ok && p != "" {
		period = p
	}

	statistics, err := client.GetStatistics(ctx, statIDs, period)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Failed to get statistics: %v", err))},
			IsError: true,
		}, nil
	}

	filtersMap := buildStatisticsFiltersMap(statIDs, period)
	paginationParams, err := ParsePaginationParams(args, filtersMap)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	paginated := ApplyPagination(statistics, paginationParams)

	result, err := json.MarshalIndent(paginated.Items, "", "  ")
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("Failed to marshal statistics result: %v", err))},
			IsError: true,
		}, nil
	}

	summary := BuildPaginationSummary(paginated.Pagination, "statistics results")
	response := buildPaginatedStatisticsResponse(paginated, result)

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(summary + "\n\n" + string(response))},
	}, nil
}

// buildStatisticsFiltersMap creates a map of filter values for pagination hash.
func buildStatisticsFiltersMap(statIDs []string, period string) map[string]any {
	filters := make(map[string]any)
	filters["statistic_ids"] = statIDs
	filters["period"] = period
	return filters
}

// paginatedStatisticsResponse wraps statistics output with pagination metadata.
type paginatedStatisticsResponse struct {
	Items      json.RawMessage    `json:"items"`
	Pagination PaginationMetadata `json:"pagination"`
}

// buildPaginatedStatisticsResponse creates the final response JSON.
func buildPaginatedStatisticsResponse(paginated PaginatedResponse[homeassistant.StatisticsResult], itemsOutput []byte) []byte {
	// If no pagination was applied (limit=0), return items directly for backwards compatibility
	if paginated.Pagination.Limit == 0 {
		return itemsOutput
	}

	response := paginatedStatisticsResponse{
		Items:      itemsOutput,
		Pagination: paginated.Pagination,
	}
	result, _ := json.MarshalIndent(response, "", "  ")
	return result
}
