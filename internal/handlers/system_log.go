// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const (
	systemLogActionList  = "list"
	systemLogActionClear = "clear"
)

// SystemLogHandlers provides handlers for Home Assistant system log operations.
type SystemLogHandlers struct{}

// NewSystemLogHandlers creates a new SystemLogHandlers instance.
func NewSystemLogHandlers() *SystemLogHandlers {
	return &SystemLogHandlers{}
}

// RegisterTools registers all system-log-related tools with the registry.
func (h *SystemLogHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.manageSystemLogTool(), h.handleManageSystemLog)
}

// manageSystemLogTool returns the tool definition for system log management.
func (h *SystemLogHandlers) manageSystemLogTool() mcp.Tool {
	return mcp.Tool{
		Name:        "manage_system_log",
		Description: "Read or clear the Home Assistant system log. Use action=list to fetch recent WARNING/ERROR log entries (ideal for investigating errors), action=clear to reset the ring buffer.",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Enum:        []string{systemLogActionList, systemLogActionClear},
					Description: "Action to perform: 'list' to retrieve log entries, 'clear' to empty the ring buffer",
				},
				"level": {
					Type:        "string",
					Enum:        []string{"warning", "error", "critical"},
					Description: "Filter by severity level (case-insensitive). Default: all levels",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of entries to return. Default: 50 (the HA ring buffer size)",
				},
				"integration": {
					Type:        "string",
					Description: "Filter entries by integration name (case-insensitive substring match on 'name' field, e.g. 'mqtt', 'zwave_js')",
				},
				"include_exception": {
					Type:        "boolean",
					Description: "Include exception stack traces in the output. Default: true. Set to false to reduce token usage.",
				},
				"format": {
					Type:        "string",
					Enum:        []string{formatNatural, formatJSON},
					Description: "Output format: 'natural' for readable text (default), 'json' for structured JSON",
				},
			},
			Required: []string{"action"},
		},
	}
}

// handleManageSystemLog dispatches to the appropriate action handler.
func (h *SystemLogHandlers) handleManageSystemLog(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	action := getString(args, "action")
	switch action {
	case systemLogActionList:
		return h.handleSystemLogList(ctx, client, args)
	case systemLogActionClear:
		return h.handleSystemLogClear(ctx, client)
	default:
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{
				mcp.NewTextContent(fmt.Sprintf("invalid action %q: must be one of [list, clear]", action)),
			},
			IsError: true,
		}, nil
	}
}

// handleSystemLogList fetches and formats system log entries.
func (h *SystemLogHandlers) handleSystemLogList(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	entries, err := client.GetSystemLog(ctx)
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("error fetching system log: %v", err))},
			IsError: true,
		}, nil
	}

	entries = filterSystemLogEntries(entries, args)

	includeException := true
	if v, ok := args["include_exception"].(bool); ok {
		includeException = v
	}

	format := getString(args, "format")
	var output string
	if format == formatJSON {
		output, err = formatSystemLogJSON(entries, includeException)
	} else {
		output = formatSystemLogNatural(entries, includeException)
	}
	if err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("error formatting output: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent(output)},
	}, nil
}

// handleSystemLogClear clears the system log ring buffer.
func (h *SystemLogHandlers) handleSystemLogClear(ctx context.Context, client homeassistant.Client) (*mcp.ToolsCallResult, error) {
	if err := client.ClearSystemLog(ctx); err != nil {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{mcp.NewTextContent(fmt.Sprintf("error clearing system log: %v", err))},
			IsError: true,
		}, nil
	}
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.NewTextContent("System log cleared successfully.")},
	}, nil
}

// filterSystemLogEntries applies level, integration, and limit filters.
func filterSystemLogEntries(entries []homeassistant.SystemLogEntry, args map[string]any) []homeassistant.SystemLogEntry {
	level := strings.ToLower(getString(args, "level"))
	integration := strings.ToLower(getString(args, "integration"))
	limitVal := getInt(args, "limit")

	var filtered []homeassistant.SystemLogEntry
	for _, e := range entries {
		if level != "" && !strings.EqualFold(e.Level, level) {
			continue
		}
		if integration != "" && !strings.Contains(strings.ToLower(e.Name), integration) {
			continue
		}
		filtered = append(filtered, e)
	}

	if limitVal > 0 && len(filtered) > limitVal {
		filtered = filtered[:limitVal]
	}
	return filtered
}

// formatSystemLogNatural produces a human-readable log summary.
func formatSystemLogNatural(entries []homeassistant.SystemLogEntry, includeException bool) string {
	if len(entries) == 0 {
		return "No system log entries found matching the given filters."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d system log %s\n", len(entries), pluralize(len(entries), "entry", "entries"))

	for _, e := range entries {
		msg := strings.Join(e.Message, " ")
		firstTime := unixToUTC(e.FirstOccurred)
		fmt.Fprintf(&sb, "\n[%s] %s (%dx, first: %s)\n", strings.ToUpper(e.Level), e.Name, e.Count, firstTime)
		if len(e.Source) >= 2 {
			fmt.Fprintf(&sb, "  Source: %v:%v\n", e.Source[0], e.Source[1])
		}
		fmt.Fprintf(&sb, "  Message: %s\n", msg)
		if includeException && e.Exception != "" {
			fmt.Fprintf(&sb, "  Exception:\n    %s\n", strings.ReplaceAll(e.Exception, "\n", "\n    "))
		}
	}
	return sb.String()
}

// formatSystemLogJSON serializes the entries as indented JSON.
func formatSystemLogJSON(entries []homeassistant.SystemLogEntry, includeException bool) (string, error) {
	if !includeException {
		stripped := make([]homeassistant.SystemLogEntry, len(entries))
		copy(stripped, entries)
		for i := range stripped {
			stripped[i].Exception = ""
		}
		entries = stripped
	}
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// pluralize returns singular or plural form based on count.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// unixToUTC converts a Unix float64 timestamp to a UTC time string.
func unixToUTC(ts float64) string {
	sec := int64(ts)
	nsec := int64((ts - math.Trunc(ts)) * 1e9)
	return time.Unix(sec, nsec).UTC().Format("2006-01-02 15:04:05 UTC")
}
