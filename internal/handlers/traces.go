package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Action constants for manage_trace tool.
const (
	traceActionList  = "list"
	traceActionGet   = "get"
	traceActionDebug = "debug"
)

// Domain constants for trace operations.
const (
	traceDomainAutomation = "automation"
	traceDomainScript     = "script"
)

// TraceHandlers provides handlers for automation and script trace operations.
type TraceHandlers struct{}

// NewTraceHandlers creates a new trace handlers instance.
func NewTraceHandlers() *TraceHandlers {
	return &TraceHandlers{}
}

// RegisterTraceTools registers trace-related tools with the MCP registry.
func RegisterTraceTools(registry *mcp.Registry) {
	handler := NewTraceHandlers()

	registry.RegisterTool(mcp.Tool{
		Name:        "manage_trace",
		Description: "View automation and script execution traces. Supports list (view all traces), get (view specific trace details), and debug (consolidated automation debug report: config, latest trace, trigger entity states, and logbook).",
		InputSchema: mcp.JSONSchema{
			Type: "object",
			Properties: map[string]mcp.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Action to perform: 'list' (list all traces), 'get' (get specific trace details), or 'debug' (consolidated automation debug report).",
					Enum:        []string{traceActionList, traceActionGet, traceActionDebug},
				},
				"domain": {
					Type:        "string",
					Description: "Domain to query traces for: 'automation' or 'script'.",
					Enum:        []string{traceDomainAutomation, traceDomainScript},
				},
				"entity_id": {
					Type:        "string",
					Description: "Entity ID for 'get' action (required) or 'list' action (optional, auto-derives domain and filters traces by item, e.g., 'automation.morning_routine').",
				},
				"automation_id": {
					Type:        "string",
					Description: "Automation entity ID or bare ID (required for 'debug' action, e.g., 'automation.morning_routine' or 'morning_routine').",
				},
				"run_id": {
					Type:        "string",
					Description: "Trace run ID (required for 'get' action).",
				},
				"hours": {
					Type:        "number",
					Description: "Number of hours to look back for logbook entries in 'debug' action (default: 6).",
				},
				"format": {
					Type:        "string",
					Description: "Output format: 'natural' (default, human-readable) or 'json' (structured JSON).",
					Enum:        []string{"natural", "json"},
					Default:     "natural",
				},
			},
			Required: []string{"action"},
		},
	}, handler.HandleManageTrace)
}

// HandleManageTrace handles the manage_trace tool invocation.
func (h *TraceHandlers) HandleManageTrace(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	// Extract action
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action parameter is required and must be 'list', 'get', or 'debug'"), nil
	}

	// Extract format (default: natural)
	format, _ := args["format"].(string)
	if format == "" {
		format = formatNatural
	}

	// Route to action handler
	switch action {
	case traceActionList:
		return h.handleListTraces(ctx, client, args, format)
	case traceActionGet:
		return h.handleGetTrace(ctx, client, args, format)
	case traceActionDebug:
		return h.handleDebugTrace(ctx, client, args, format)
	default:
		return errorResult(fmt.Sprintf("invalid action %q, must be one of: list, get, debug", action)), nil
	}
}

// resolveTraceListParams derives domain and item_id from entity_id (when provided),
// validates the prefix, and checks for conflicts with an explicit domain parameter.
func resolveTraceListParams(entityID, domain string) (resolvedDomain, itemID, errMsg string) {
	if entityID == "" {
		return domain, "", ""
	}
	dotIdx := strings.Index(entityID, ".")
	if dotIdx <= 0 {
		return "", "", fmt.Sprintf("entity_id %q is invalid for list action: must be 'automation.<id>' or 'script.<id>'", entityID)
	}
	derived := entityID[:dotIdx]
	if derived != traceDomainAutomation && derived != traceDomainScript {
		return "", "", fmt.Sprintf("entity_id prefix %q is not supported for trace list; use 'automation' or 'script'", derived)
	}
	if domain != "" && domain != derived {
		return "", "", fmt.Sprintf("entity_id prefix %q conflicts with explicit domain %q", derived, domain)
	}
	return derived, entityID, ""
}

// handleListTraces lists all traces for a domain.
func (h *TraceHandlers) handleListTraces(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	domain, _ := args["domain"].(string)
	entityID, _ := args["entity_id"].(string)

	domain, itemID, errMsg := resolveTraceListParams(entityID, domain)
	if errMsg != "" {
		return errorResult(errMsg), nil
	}

	// Build command data
	data := make(map[string]any)
	if domain != "" {
		data["domain"] = domain
	}
	if itemID != "" {
		data["item_id"] = itemID
	}

	// Call trace/list WebSocket command
	response, err := client.SendHACSCommand(ctx, "trace/list", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to list traces: %v", err)), nil
	}

	// Parse response - convert to []any for consistent handling
	var traces []any
	switch v := response.(type) {
	case []any:
		traces = v
	case []map[string]any:
		// Convert []map[string]any to []any
		traces = make([]any, len(v))
		for i, item := range v {
			traces[i] = item
		}
	case map[string]any:
		// Response might be wrapped
		if traceData, ok := v["traces"].([]any); ok {
			traces = traceData
		}
	}

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal traces: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format
	return successResult(h.formatTracesNatural(traces)), nil
}

// handleGetTrace retrieves a specific trace by entity_id and run_id.
func (h *TraceHandlers) handleGetTrace(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	// Validate required parameters
	domain, _ := args["domain"].(string)
	if domain == "" {
		return errorResult("domain is required for get action"), nil
	}

	entityID, _ := args["entity_id"].(string)
	if entityID == "" {
		return errorResult("entity_id is required for get action"), nil
	}

	runID, _ := args["run_id"].(string)
	if runID == "" {
		return errorResult("run_id is required for get action"), nil
	}

	// Build command data
	data := map[string]any{
		"domain":  domain,
		"item_id": entityID,
		"run_id":  runID,
	}

	// Call trace/get WebSocket command
	response, err := client.SendHACSCommand(ctx, "trace/get", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get trace: %v", err)), nil
	}

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal trace: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format
	return successResult(h.formatTraceNatural(response)), nil
}

// formatTracesNatural formats a list of traces in natural language.
func (h *TraceHandlers) formatTracesNatural(traces []any) string {
	if len(traces) == 0 {
		return "No traces found."
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d trace(s):", len(traces)))

	for i, trace := range traces {
		traceMap, ok := trace.(map[string]any)
		if !ok {
			continue
		}

		runID := getMapString(traceMap, "run_id", "")
		state := getMapString(traceMap, "state", "")
		timestamp := getMapString(traceMap, "timestamp", "")

		var duration float64
		if v, ok := traceMap["duration"].(float64); ok {
			duration = v
		}

		parts = append(parts, fmt.Sprintf("\n%d. Run ID: %s", i+1, runID))
		if state != "" {
			parts = append(parts, fmt.Sprintf("   State: %s", state))
		}
		if timestamp != "" {
			parts = append(parts, fmt.Sprintf("   Timestamp: %s", timestamp))
		}
		if duration > 0 {
			parts = append(parts, fmt.Sprintf("   Duration: %.2fs", duration))
		}
	}

	return strings.Join(parts, "\n")
}

// formatTraceNatural formats a single trace in natural language.
func (h *TraceHandlers) formatTraceNatural(response any) string {
	traceMap, ok := response.(map[string]any)
	if !ok {
		return "Invalid trace data."
	}

	var parts []string
	parts = append(parts, "Trace Execution Path:")

	// Extract trace details
	if trace, ok := traceMap["trace"].(map[string]any); ok {
		trigger := getMapString(trace, "trigger", "")
		if trigger != "" {
			parts = append(parts, fmt.Sprintf("\nTrigger: %s", trigger))
		}

		if conditions, ok := trace["conditions"].([]any); ok {
			parts = append(parts, fmt.Sprintf("\nConditions: %d evaluated", len(conditions)))
		}

		if actions, ok := trace["actions"].([]any); ok {
			parts = append(parts, fmt.Sprintf("\nActions: %d executed", len(actions)))
		}
	}

	return strings.Join(parts, "\n")
}
