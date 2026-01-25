// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

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
		Description: "Get logbook entries showing what happened in Home Assistant. By default returns compact output with time, entity, and state change. Use filters to narrow down results.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"start_time": {
					Type:        "string",
					Description: "Start time in ISO 8601 format (e.g., '2024-01-15T10:00:00'). Default: 24 hours ago",
				},
				"end_time": {
					Type:        "string",
					Description: "End time in ISO 8601 format. Default: now",
				},
				"entity_id": {
					Type:        "string",
					Description: "Filter by entity ID (e.g., 'light.living_room')",
				},
				"hours": {
					Type:        "number",
					Description: "Number of hours to look back from now (alternative to start_time, e.g., 6 for last 6 hours)",
				},
			},
		},
	}
}

// logbookResponse represents a compact view of logbook entries.
type logbookResponse struct {
	EntryCount int                   `json:"entry_count"`
	TimeRange  string                `json:"time_range"`
	Entries    []logbookCompactEntry `json:"entries"`
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

// buildLogbookResponse builds the response from logbook entries.
func buildLogbookResponse(entries []homeassistant.LogbookEntry, startTime, endTime time.Time, entityID string) (string, error) {
	compactEntries := make([]logbookCompactEntry, 0, len(entries))
	for _, entry := range entries {
		compactEntries = append(compactEntries, logbookCompactEntry{
			When:     entry.When,
			Name:     entry.Name,
			EntityID: entry.EntityID,
			State:    entry.State,
			Message:  entry.Message,
		})
	}

	response := logbookResponse{
		EntryCount: len(entries),
		TimeRange:  fmt.Sprintf("%s to %s", startTime.Format("2006-01-02 15:04"), endTime.Format("2006-01-02 15:04")),
		Entries:    compactEntries,
	}

	output, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("formatting response: %w", err)
	}

	summary := fmt.Sprintf("Found %d logbook entries", len(entries))
	if entityID != "" {
		summary += fmt.Sprintf(" for %s", entityID)
	}

	return summary + "\n\n" + string(output), nil
}

// handleGetLogbook handles requests to retrieve logbook entries.
func (h *LogbookHandlers) handleGetLogbook(
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

	entityID := getString(args, "entity_id")

	entries, err := client.GetLogbook(ctx, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), entityID)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("Error getting logbook: %v", err)),
			},
			IsError: true,
		}, nil
	}

	output, err := buildLogbookResponse(entries, startTime, endTime, entityID)
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
