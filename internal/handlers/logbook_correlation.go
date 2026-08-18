package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// CorrelationReport represents logbook event correlations.
type CorrelationReport struct {
	EntityIDs    []string           `json:"entity_ids"`
	TimeRange    string             `json:"time_range"`
	TotalEvents  int                `json:"total_events"`
	Correlations []CorrelationGroup `json:"correlations"`
}

// CorrelationGroup represents events that occurred within the time window.
type CorrelationGroup struct {
	Timestamp string            `json:"timestamp"`
	Events    []CorrelatedEvent `json:"events"`
}

// CorrelatedEvent represents a single event in a correlation group.
type CorrelatedEvent struct {
	EntityID string `json:"entity_id"`
	State    string `json:"state"`
	When     string `json:"when"`
	Name     string `json:"name"`
	DelayMs  int64  `json:"delay_ms"`
}

// handleCorrelation handles correlation mode requests.
func (h *LogbookHandlers) handleCorrelation(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	// Parse and validate entity_ids
	entityIDs, err := parseEntityIDsArray(args)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	// Parse time range
	startTime, endTime, err := parseLogbookTimes(args)
	if err != nil {
		return errorResult(fmt.Sprintf("Invalid time format: %v", err)), nil
	}

	// Fetch and build correlation report
	maxTimeDelta := getMaxTimeDelta(args)
	allEntries := fetchLogbookEntriesForCorrelation(ctx, client, entityIDs, startTime, endTime)
	correlations := buildCorrelationGroups(allEntries, maxTimeDelta)

	// Build and format report
	report := buildCorrelationReport(entityIDs, startTime, endTime, allEntries, correlations)
	return formatCorrelationOutput(report, args)
}

// parseEntityIDsArray extracts and validates entity_ids from args.
func parseEntityIDsArray(args map[string]any) ([]string, error) {
	entityIDsAny, ok := args["entity_ids"]
	if !ok {
		return nil, fmt.Errorf("entity_ids is required for correlation mode")
	}

	entityIDsSlice, ok := entityIDsAny.([]any)
	if !ok {
		return nil, fmt.Errorf("entity_ids must be an array")
	}

	if len(entityIDsSlice) == 0 {
		return nil, fmt.Errorf("entity_ids array cannot be empty")
	}

	entityIDs := make([]string, 0, len(entityIDsSlice))
	for _, id := range entityIDsSlice {
		if str, ok := id.(string); ok {
			entityIDs = append(entityIDs, str)
		}
	}

	if len(entityIDs) == 0 {
		return nil, fmt.Errorf("entity_ids must contain valid strings")
	}

	return entityIDs, nil
}

// fetchLogbookEntriesForCorrelation fetches and parses logbook entries for all entity IDs.
func fetchLogbookEntriesForCorrelation(
	ctx context.Context,
	client homeassistant.Client,
	entityIDs []string,
	startTime, endTime time.Time,
) []correlationEntry {
	allEntries := make([]correlationEntry, 0)

	for _, entityID := range entityIDs {
		entries, fetchErr := client.GetLogbook(ctx, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), entityID)
		if fetchErr != nil {
			continue
		}

		for _, entry := range entries {
			timestamp, parseErr := parseTimeString(entry.When)
			if parseErr != nil {
				continue
			}

			allEntries = append(allEntries, correlationEntry{
				Timestamp: timestamp,
				EntityID:  entry.EntityID,
				Name:      entry.Name,
				State:     entry.State,
				When:      entry.When,
			})
		}
	}

	// Sort by timestamp
	slices.SortFunc(allEntries, func(a, b correlationEntry) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	return allEntries
}

// buildCorrelationReport creates the correlation report structure.
func buildCorrelationReport(
	entityIDs []string,
	startTime, endTime time.Time,
	allEntries []correlationEntry,
	correlations []CorrelationGroup,
) CorrelationReport {
	timeRangeStr := fmt.Sprintf("%s to %s",
		startTime.Format("2006-01-02 15:04"),
		endTime.Format("2006-01-02 15:04"))

	return CorrelationReport{
		EntityIDs:    entityIDs,
		TimeRange:    timeRangeStr,
		TotalEvents:  len(allEntries),
		Correlations: correlations,
	}
}

// formatCorrelationOutput formats the correlation report based on format parameter.
func formatCorrelationOutput(report CorrelationReport, args map[string]any) (*mcp.ToolsCallResult, error) {
	formatStr := getString(args, "format")
	format := formatter.ParseFormat(formatStr)

	var output string
	if format == formatter.FormatJSON {
		data, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			return errorResult(fmt.Sprintf("Error formatting correlation report: %v", marshalErr)), nil
		}
		output = string(data)
	} else {
		output = formatCorrelationNatural(report)
	}

	return successResult(output), nil
}

// correlationEntry represents a logbook entry with parsed timestamp.
type correlationEntry struct {
	Timestamp time.Time
	EntityID  string
	Name      string
	State     string
	When      string
}

// getMaxTimeDelta extracts max_time_delta_seconds from args.
func getMaxTimeDelta(args map[string]any) time.Duration {
	if delta, ok := args["max_time_delta_seconds"]; ok {
		switch v := delta.(type) {
		case float64:
			return time.Duration(v) * time.Second
		case int:
			return time.Duration(v) * time.Second
		}
	}
	return 60 * time.Second // default: 60 seconds
}

// buildCorrelationGroups groups events within maxTimeDelta of each other.
func buildCorrelationGroups(entries []correlationEntry, maxTimeDelta time.Duration) []CorrelationGroup {
	if len(entries) == 0 {
		return []CorrelationGroup{}
	}

	groups := make([]CorrelationGroup, 0)
	currentGroup := []correlationEntry{entries[0]}
	groupStartTime := entries[0].Timestamp

	for i := 1; i < len(entries); i++ {
		entry := entries[i]
		timeSinceGroupStart := entry.Timestamp.Sub(groupStartTime)

		if timeSinceGroupStart <= maxTimeDelta {
			// Add to current group
			currentGroup = append(currentGroup, entry)
		} else {
			// Finalize current group if it has multiple entities
			if shouldIncludeGroup(currentGroup) {
				groups = append(groups, buildGroup(currentGroup, groupStartTime))
			}

			// Start new group
			currentGroup = []correlationEntry{entry}
			groupStartTime = entry.Timestamp
		}
	}

	// Don't forget the last group
	if shouldIncludeGroup(currentGroup) {
		groups = append(groups, buildGroup(currentGroup, groupStartTime))
	}

	return groups
}

// shouldIncludeGroup checks if a group should be included (multiple entities or interesting pattern).
func shouldIncludeGroup(group []correlationEntry) bool {
	if len(group) < 2 {
		return false
	}

	// Check if group has events from different entities
	entitySet := make(map[string]bool)
	for _, entry := range group {
		entitySet[entry.EntityID] = true
	}

	return len(entitySet) > 1
}

// buildGroup converts a slice of correlation entries into a CorrelationGroup.
func buildGroup(entries []correlationEntry, groupStartTime time.Time) CorrelationGroup {
	events := make([]CorrelatedEvent, 0, len(entries))

	for _, entry := range entries {
		delayMs := entry.Timestamp.Sub(groupStartTime).Milliseconds()
		events = append(events, CorrelatedEvent{
			EntityID: entry.EntityID,
			State:    entry.State,
			When:     entry.When,
			Name:     entry.Name,
			DelayMs:  delayMs,
		})
	}

	return CorrelationGroup{
		Timestamp: groupStartTime.Format(time.RFC3339),
		Events:    events,
	}
}

// formatCorrelationNatural formats correlation report in natural language.
func formatCorrelationNatural(report CorrelationReport) string {
	var result strings.Builder

	fmt.Fprintf(&result, "Logbook Correlation Analysis\n\n")
	fmt.Fprintf(&result, "Entities: %s\n", strings.Join(report.EntityIDs, ", "))
	fmt.Fprintf(&result, "Time Range: %s\n", report.TimeRange)
	fmt.Fprintf(&result, "Total Events: %d\n", report.TotalEvents)
	fmt.Fprintf(&result, "Correlated Groups: %d\n\n", len(report.Correlations))

	if len(report.Correlations) == 0 {
		result.WriteString("No correlated events found. Events from different entities did not occur within the time window.")
		return result.String()
	}

	result.WriteString("Correlations (events within time window):\n\n")
	for i, group := range report.Correlations {
		groupTime, _ := parseTimeString(group.Timestamp)
		fmt.Fprintf(&result, "%d. %s\n", i+1, groupTime.Format("2006-01-02 15:04:05"))

		for _, event := range group.Events {
			if event.DelayMs == 0 {
				fmt.Fprintf(&result, "   - %s: %s\n", event.Name, event.State)
			} else {
				fmt.Fprintf(&result, "   - %s: %s (+%dms)\n", event.Name, event.State, event.DelayMs)
			}
		}
		result.WriteString("\n")
	}

	return strings.TrimSuffix(result.String(), "\n")
}
