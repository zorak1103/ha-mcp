package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// historySection is the processed history of one requested entity.
type historySection struct {
	entityID string
	result   historyResult
	// exists is false only when the entity has no rows and GetState cannot find it.
	exists bool
}

// historySectionJSON is one entity's block in the multi-entity json output.
type historySectionJSON struct {
	EntityID string `json:"entity_id"`
	Entries  any    `json:"entries"`
}

// handleHistory handles mode=history requests (adapted from handleGetHistory).
// One entity renders exactly as it always did; several entities render one section
// each, in request order. state and limit apply to every entity separately.
func (h *ConsolidatedEntityQueryHandlers) handleHistory(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	params, err := parseHistoryParams(args)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	history, err := client.GetHistory(ctx, params.entityIDs, params.startTime, params.endTime)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting history: %v", err)), nil
	}

	sections := buildHistorySections(ctx, client, history, params)

	var text string
	if params.format == formatter.FormatNatural {
		text, err = formatHistorySectionsNatural(ctx, sections, params)
	} else {
		text, err = formatHistorySectionsJSON(sections, params)
	}
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting history: %v", err)), nil
	}

	return &mcp.ToolsCallResult{Content: []mcp.ContentBlock{mcp.NewTextContent(text)}}, nil
}

// buildHistorySections applies state and limit to each requested entity's rows. GetState
// is consulted only for entities left without rows, to tell "not found" from "no changes".
func buildHistorySections(
	ctx context.Context,
	client homeassistant.Client,
	history map[string][]homeassistant.HistoryEntry,
	params *historyParams,
) []historySection {
	sections := make([]historySection, 0, len(params.entityIDs))
	for _, id := range params.entityIDs {
		result := processHistoryEntries(history[id], params.stateFilter, params.limit)
		exists := true
		if len(result.entries) == 0 {
			_, stateErr := client.GetState(ctx, id)
			exists = stateErr == nil
		}
		sections = append(sections, historySection{entityID: id, result: result, exists: exists})
	}
	return sections
}

// formatHistorySectionsNatural renders each entity with the natural formatter, separated by a blank line.
func formatHistorySectionsNatural(ctx context.Context, sections []historySection, params *historyParams) (string, error) {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		part, err := formatHistorySectionNatural(ctx, section, params)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n\n"), nil
}

// formatHistorySectionNatural formats one entity's history using the natural language formatter.
func formatHistorySectionNatural(ctx context.Context, section historySection, params *historyParams) (string, error) {
	opts := formatter.HistoryOptions{
		Verbose:      params.verbose,
		Limit:        params.limit,
		EntityExists: section.exists,
	}
	if opts.Limit == 0 {
		opts.Limit = 20 // Default limit for natural language history output
	}

	output, err := formatter.NewNaturalFormatter().FormatHistory(ctx, section.entityID, section.result.entries, opts)
	if err != nil {
		return "", err
	}

	// Add filter info if filtered
	if params.stateFilter != "" {
		output += fmt.Sprintf(" (filtered by state='%s')", params.stateFilter)
	}

	// Add truncation notice if applicable
	if section.result.totalCount > len(section.result.entries) {
		output += fmt.Sprintf("\n\n[Showing %d of %d entries. Use limit parameter for more.]",
			len(section.result.entries), section.result.totalCount)
	}

	return output, nil
}

// formatHistorySectionsJSON renders the json format: a summary, a blank line, then the entries.
// A single entity keeps the original bare array; several entities become an array of
// {entity_id, entries} objects in request order.
func formatHistorySectionsJSON(sections []historySection, params *historyParams) (string, error) {
	if len(sections) == 1 {
		output, err := formatHistoryOutput(sections[0].result.entries, params.verbose)
		if err != nil {
			return "", err
		}
		summary := buildHistorySummary(sections[0].entityID, sections[0].result, params.stateFilter, params.verbose)
		return summary + "\n\n" + string(output), nil
	}

	blocks := make([]historySectionJSON, 0, len(sections))
	lines := make([]string, 0, len(sections))
	for _, section := range sections {
		entries := section.result.entries
		if entries == nil {
			entries = []homeassistant.HistoryEntry{} // an array per entity, never null
		}
		blocks = append(blocks, historySectionJSON{
			EntityID: section.entityID,
			Entries:  historyEntriesView(entries, params.verbose),
		})
		line := historySummaryLine(section.entityID, section.result, params.stateFilter)
		if !section.exists {
			line += " (entity not found in Home Assistant)"
		}
		lines = append(lines, line)
	}

	output, err := json.MarshalIndent(blocks, "", "  ")
	if err != nil {
		return "", err
	}
	summary := strings.Join(lines, "\n")
	if !params.verbose {
		summary += VerboseHint
	}
	return summary + "\n\n" + string(output), nil
}
