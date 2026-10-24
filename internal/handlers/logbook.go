// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// get_logbook modes.
const (
	logbookModeEntries     = "entries"
	logbookModeCorrelation = "correlation"
)

// logbookArgModes maps each mode-specific get_logbook argument to the only
// modes that read it; a supplied value for any other mode is rejected.
// start_time, end_time, hours, entity_ids, format and mode apply to both modes
// and are deliberately absent.
var logbookArgModes = map[string][]string{
	attrEntityID:             {logbookModeEntries},
	"limit":                  {logbookModeEntries},
	"cursor":                 {logbookModeEntries},
	"offset":                 {logbookModeEntries},
	"max_time_delta_seconds": {logbookModeCorrelation},
}

// LogbookHandlers provides handlers for Home Assistant logbook operations.
type LogbookHandlers struct{}

// NewLogbookHandlers creates a new LogbookHandlers instance.
func NewLogbookHandlers() *LogbookHandlers {
	return &LogbookHandlers{}
}

// RegisterTools registers all logbook-related tools with the registry.
func (h *LogbookHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.getLogbookTool(), h.handleGetLogbook)
}

// getLogbookTool returns the tool definition for retrieving logbook entries.
func (h *LogbookHandlers) getLogbookTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_logbook",
		Description: "Get logbook entries showing what happened in Home Assistant. Supports two modes: 'entries' (default) for chronological events, and 'correlation' for analyzing cause-effect relationships across multiple entities.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"mode": {
					Type:        "string",
					Enum:        []string{"entries", "correlation"},
					Description: "Query mode: 'entries' for chronological logbook (default), 'correlation' for cause-effect analysis across entities",
				},
				"start_time": {
					Type:        "string",
					Description: "Start time in ISO 8601 format (e.g., '2024-01-15T10:00:00'). Default: 24 hours ago",
				},
				"end_time": {
					Type:        "string",
					Description: "End time in ISO 8601 format. Default: now",
				},
				attrEntityID: {
					Type:        "string",
					Description: "Filter by a single entity ID (mode=entries). Use entity_id OR entity_ids, not both.",
				},
				attrEntityIDs: {
					Type:        "array",
					Description: "Entity IDs (max 100). mode=entries: filter to these entities (alternative to entity_id). mode=correlation: required.",
					Items:       &mcp.JSONSchema{Type: "string"},
				},
				"hours": {
					Type:        "number",
					Description: "Number of hours to look back from now (alternative to start_time, e.g., 6 for last 6 hours)",
				},
				"max_time_delta_seconds": {
					Type:        "integer",
					Description: "Maximum time difference in seconds to group correlated events (default: 60). Only for mode=correlation; rejected in mode=entries",
				},
				"format": {
					Type:        "string",
					Enum:        []string{"natural", "json"},
					Description: "Output format: 'natural' for LLM-optimized output (default), 'json' for structured JSON",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of logbook entries to return (max 1000, default: no limit). Use with 'cursor' for pagination. Only for mode=entries; rejected in mode=correlation",
				},
				"offset": logbookOffsetSchema(),
				"cursor": {
					Type:        "string",
					Description: "Pagination cursor from previous response to get next page of results. Only for mode=entries; rejected in mode=correlation",
				},
			},
		},
	}
}

// logbookOffsetSchema is the shared offset schema with get_logbook's mode note.
func logbookOffsetSchema() mcp.JSONSchema {
	schema := paginationOffsetSchema()
	schema.Description += ". Only for mode=entries; rejected in mode=correlation"
	return schema
}

// logbookCompactEntry represents a compact logbook entry.
type logbookCompactEntry struct {
	When     string `json:"when"`
	Name     string `json:"name"`
	EntityID string `json:"entity_id,omitempty"`
	State    string `json:"state,omitempty"`
	Message  string `json:"message,omitempty"`
}

// parseTimeString parses a time string in RFC3339 or ISO format.
func parseTimeString(timeStr string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return parsed, nil
	}
	// Try without timezone
	return time.Parse("2006-01-02T15:04:05", timeStr)
}

// getHoursFromArgs extracts hours value from args as float64.
func getHoursFromArgs(args map[string]any) float64 {
	hours, ok := args["hours"]
	if !ok {
		return 0
	}
	switch v := hours.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

// parseLogbookTimes parses start and end times from arguments.
func parseLogbookTimes(args map[string]any) (startTime, endTime time.Time, err error) {
	now := time.Now()

	// Handle hours parameter
	if hoursFloat := getHoursFromArgs(args); hoursFloat > 0 {
		startTime = now.Add(-time.Duration(hoursFloat) * time.Hour)
	}

	// Handle start_time parameter (overrides hours if both specified)
	if startTimeStr := getString(args, "start_time"); startTimeStr != "" {
		startTime, err = parseTimeString(startTimeStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start_time format: %w", err)
		}
	}

	// Default to 24 hours ago if no start time specified
	if startTime.IsZero() {
		startTime = now.Add(-24 * time.Hour)
	}

	// Handle end_time parameter
	endTime = now
	if endTimeStr := getString(args, "end_time"); endTimeStr != "" {
		endTime, err = parseTimeString(endTimeStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end_time format: %w", err)
		}
	}

	return startTime, endTime, nil
}

// handleGetLogbook handles requests to retrieve logbook entries.
func (h *LogbookHandlers) handleGetLogbook(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	mode, bad := parseModeArg(args, "mode", logbookModeEntries, []string{logbookModeEntries, logbookModeCorrelation})
	if bad != nil {
		return bad, nil
	}

	if rejected := rejectInapplicableArgs(args, "mode", mode, logbookArgModes, ""); rejected != nil {
		return rejected, nil
	}

	if mode == logbookModeCorrelation {
		return h.handleCorrelation(ctx, client, args)
	}
	return h.handleEntriesMode(ctx, client, args)
}

// handleEntriesMode handles logbook entries mode (default mode).
func (h *LogbookHandlers) handleEntriesMode(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	startTime, endTime, err := parseLogbookTimes(args)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Invalid time format: %v", err)),
			},
			IsError: true,
		}, nil
	}

	entityIDs, err := parseEntityIDOrIDs(args)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	entries, err := client.GetLogbook(ctx, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), entityIDs)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting logbook: %v", err)),
			},
			IsError: true,
		}, nil
	}

	// Build filters map for pagination hash
	filtersMap := buildLogbookFiltersMap(startTime, endTime, entityIDs)

	// Parse pagination params
	paginationParams, err := ParsePaginationParams(args, filtersMap)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	// Apply pagination
	paginated := ApplyPagination(entries, paginationParams)

	output, err := buildLogbookResponsePaginated(paginated, startTime, endTime, entityIDs)
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
			mcp.NewTextContent(output),
		},
	}, nil
}

// buildLogbookFiltersMap creates a map of filter values for pagination hash.
// The entity IDs are hashed as a sorted copy so that a cursor stays valid when
// the caller reorders the same set; the caller's slice is left untouched.
func buildLogbookFiltersMap(startTime, endTime time.Time, entityIDs []string) map[string]any {
	filters := make(map[string]any)
	filters["start_time"] = startTime.Format(time.RFC3339)
	filters["end_time"] = endTime.Format(time.RFC3339)
	if len(entityIDs) > 0 {
		filters[attrEntityIDs] = slices.Sorted(slices.Values(entityIDs))
	}
	return filters
}

// paginatedLogbookResponse wraps logbook entries with pagination metadata.
type paginatedLogbookResponse struct {
	EntryCount int                   `json:"entry_count"`
	TimeRange  string                `json:"time_range"`
	Entries    []logbookCompactEntry `json:"entries"`
	Pagination *PaginationMetadata   `json:"pagination,omitempty"`
}

// buildLogbookResponsePaginated builds the response from paginated logbook entries.
func buildLogbookResponsePaginated(paginated PaginatedResponse[homeassistant.LogbookEntry], startTime, endTime time.Time, entityIDs []string) (string, error) {
	compactEntries := make([]logbookCompactEntry, 0, len(paginated.Items))
	for _, entry := range paginated.Items {
		compactEntries = append(compactEntries, logbookCompactEntry{
			When:     entry.When,
			Name:     entry.Name,
			EntityID: entry.EntityID,
			State:    entry.State,
			Message:  entry.Message,
		})
	}

	var paginationMeta *PaginationMetadata
	if paginated.Pagination.Limit > 0 {
		paginationMeta = &paginated.Pagination
	}

	response := paginatedLogbookResponse{
		EntryCount: paginated.Pagination.Count,
		TimeRange:  fmt.Sprintf("%s to %s", startTime.Format("2006-01-02 15:04"), endTime.Format("2006-01-02 15:04")),
		Entries:    compactEntries,
		Pagination: paginationMeta,
	}

	output, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("formatting response: %w", err)
	}

	summary := BuildPaginationSummary(paginated.Pagination, "logbook entries")
	switch {
	case len(entityIDs) == 1:
		summary += fmt.Sprintf(" for %s", entityIDs[0])
	case len(entityIDs) > 1:
		summary += fmt.Sprintf(" for %d entities", len(entityIDs))
	}

	return summary + "\n\n" + string(output), nil
}
