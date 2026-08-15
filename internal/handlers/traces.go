package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
					Description: "Domain to query traces for: 'automation' or 'script'. Required for the 'list' action unless entity_id is provided (which auto-derives it).",
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
				"wait": {
					Type:        "boolean",
					Description: "If true and no traces are found immediately, poll until a trace appears or the wait timeout expires. Use after triggering an automation to give Home Assistant time to record the trace asynchronously. Default: false.",
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

// resolveTraceListParams derives domain and the entity_id from entity_id (when provided),
// validates the prefix, and checks for conflicts with an explicit domain parameter.
// The returned entityID must still be mapped to HA's trace item_id via resolveTraceItemID.
func resolveTraceListParams(entityID, domain string) (resolvedDomain, resolvedEntityID, errMsg string) {
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

// resolveTraceItemID maps an entity_id to the item_id Home Assistant's trace API expects.
// HA stores traces under the key "<domain>.<item_id>" (components/trace/models.py
// ActionTrace.__init__: self.key = f"{domain}.{item_id}"), where item_id is the object's
// unique_id, NOT its entity_id. Both automation and script entities set their unique_id to
// exactly that value at creation time (automation/__init__.py, script/__init__.py), so a single
// entity-registry lookup resolves either domain:
//   - Scripts: unique_id matches the entity's object_id unless the entity was renamed after
//     creation. Passing the full entity_id instead produces the nonexistent key "script.script.<id>".
//   - Automations: unique_id is the automation's config id, which differs from the entity
//     object_id for UI-created automations.
//
// A failed registry fetch or a missing entry degrades to the object_id and logs a warning: HA
// returns an empty trace list rather than an error for a wrong key, so a silently wrong item_id
// is otherwise indistinguishable from "no traces recorded yet".
func resolveTraceItemID(ctx context.Context, client homeassistant.Client, domain, entityID string) string {
	objectID := entityID
	if dotIdx := strings.Index(entityID, "."); dotIdx >= 0 {
		objectID = entityID[dotIdx+1:]
	}

	entries, err := client.GetEntityRegistry(ctx)
	if err != nil {
		slog.WarnContext(ctx, "trace item_id resolution degraded to object_id: entity registry fetch failed",
			"entity_id", entityID, "domain", domain, "error", err)
		return objectID
	}
	for _, entry := range entries {
		if entry.EntityID == entityID && entry.UniqueID != "" {
			return entry.UniqueID
		}
	}
	slog.WarnContext(ctx, "trace item_id resolution degraded to object_id: entity not found in registry",
		"entity_id", entityID, "domain", domain)
	return objectID
}

// handleListTraces lists all traces for a domain.
func (h *TraceHandlers) handleListTraces(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	domain, _ := args["domain"].(string)
	entityID, _ := args["entity_id"].(string)
	wait, _ := args["wait"].(bool)

	domain, traceEntityID, errMsg := resolveTraceListParams(entityID, domain)
	if errMsg != "" {
		return errorResult(errMsg), nil
	}
	if domain == "" {
		return errorResult("domain is required for list action: pass domain 'automation' or 'script', or an entity_id like 'automation.morning_routine'"), nil
	}

	// Build command data
	data := make(map[string]any)
	data["domain"] = domain
	if traceEntityID != "" {
		data["item_id"] = resolveTraceItemID(ctx, client, domain, traceEntityID)
	}

	// Call trace/list WebSocket command
	response, err := client.SendHACSCommand(ctx, "trace/list", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to list traces: %v", err)), nil
	}

	// Parse response - convert to []any for consistent handling
	traces := parseTraceListResponse(response)

	// Opt-in polling: if wait=true and no traces returned yet, poll until traces appear
	if wait && len(traces) == 0 {
		if polled, found := waitForTraces(ctx, client, data); found {
			traces = polled
		}
	}

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal traces: %v", err)), nil
		}
		// If wait found traces, marshal them instead of the original empty response
		if wait && len(traces) > 0 {
			jsonData, err = json.MarshalIndent(traces, "", "  ")
			if err != nil {
				return errorResult(fmt.Sprintf("failed to marshal traces: %v", err)), nil
			}
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format
	return successResult(h.formatTracesNatural(traces)), nil
}

// parseTraceListResponse converts a trace/list WebSocket response to []any.
func parseTraceListResponse(response any) []any {
	switch v := response.(type) {
	case []any:
		return v
	case []map[string]any:
		// Convert []map[string]any to []any
		traces := make([]any, len(v))
		for i, item := range v {
			traces[i] = item
		}
		return traces
	case map[string]any:
		// Response might be wrapped
		if traceData, ok := v["traces"].([]any); ok {
			return traceData
		}
	}
	return nil
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

	// Reject a domain/entity_id prefix mismatch rather than silently resolving item_id against
	// the wrong domain, which would return an unrelated entity's traces (mirrors the same check
	// resolveTraceListParams already does for the list action).
	if dotIdx := strings.Index(entityID, "."); dotIdx > 0 {
		if derived := entityID[:dotIdx]; derived != domain {
			return errorResult(fmt.Sprintf("entity_id prefix %q conflicts with explicit domain %q", derived, domain)), nil
		}
	}

	// Build command data
	data := map[string]any{
		"domain":  domain,
		"item_id": resolveTraceItemID(ctx, client, domain, entityID),
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
		return "No traces found. Home Assistant records traces asynchronously — if you just triggered an automation, traces may not be available yet. Try again in a moment, or pass wait=true to poll automatically."
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
		start, finish := traceTimestamps(traceMap)
		duration := traceDuration(traceMap, start, finish)

		parts = append(parts, fmt.Sprintf("\n%d. Run ID: %s", i+1, runID))
		if state != "" {
			parts = append(parts, fmt.Sprintf("   State: %s", state))
		}
		if start != "" {
			parts = append(parts, fmt.Sprintf("   Timestamp: %s", start))
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

	// The trigger description lives on the top-level short dict (AutomationTrace.as_short_dict
	// sets result["trigger"]), not nested under "trace".
	if trigger := getMapString(traceMap, "trigger", ""); trigger != "" {
		parts = append(parts, fmt.Sprintf("\nTrigger: %s", trigger))
	}

	// Extract step counts. HA flattens a trace into path keys - automations use
	// "trigger/N", "condition/N", "action/N"; scripts use "sequence/N". There is no "actions" or
	// "conditions" list key in real trace data.
	if trace, ok := traceMap["trace"].(map[string]any); ok {
		if conditions := countTopLevelTracePaths(trace, "condition"); conditions > 0 {
			parts = append(parts, fmt.Sprintf("\nConditions: %d evaluated", conditions))
		}
		if actions := countTopLevelTracePaths(trace, "action"); actions > 0 {
			parts = append(parts, fmt.Sprintf("\nActions: %d executed", actions))
		}
		if steps := countTopLevelTracePaths(trace, "sequence"); steps > 0 {
			parts = append(parts, fmt.Sprintf("\nSequence steps: %d executed", steps))
		}
	}

	return strings.Join(parts, "\n")
}

// countTopLevelTracePaths counts the distinct top-level step indices under a trace path prefix
// (e.g. "action/0", "action/1"). Home Assistant flattens a trace into path keys where nested
// steps extend the same top-level index ("action/0/choose/0/sequence/0" is still step 0), so a
// plain has-prefix count over-counts any step containing a nested action block.
func countTopLevelTracePaths(m map[string]any, prefix string) int {
	full := prefix + "/"
	seen := make(map[string]bool)
	for k := range m {
		rest, ok := strings.CutPrefix(k, full)
		if !ok {
			continue
		}
		idx, _, _ := strings.Cut(rest, "/")
		if idx != "" {
			seen[idx] = true
		}
	}
	return len(seen)
}

// traceTimestamps extracts the start/finish timestamps from a trace's "timestamp" field.
// Home Assistant's trace API returns this as {"start": <iso>, "finish": <iso>}
// (components/trace/models.py ActionTrace.as_short_dict); a flat ISO string is also accepted
// for backward compatibility with older test fixtures and is treated as start-only.
func traceTimestamps(m map[string]any) (start, finish string) {
	switch v := m["timestamp"].(type) {
	case map[string]any:
		return getMapString(v, "start", ""), getMapString(v, "finish", "")
	case string:
		return v, ""
	}
	return "", ""
}

// traceDuration returns a trace's execution duration in seconds. An explicit "duration" field
// (not present in real HA responses, but used by some fixtures) takes precedence; otherwise the
// duration is derived from the finish/start timestamps.
func traceDuration(m map[string]any, start, finish string) float64 {
	if d, ok := m["duration"].(float64); ok {
		return d
	}
	if start == "" || finish == "" {
		return 0
	}
	startT, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return 0
	}
	finishT, err := time.Parse(time.RFC3339, finish)
	if err != nil {
		return 0
	}
	return finishT.Sub(startT).Seconds()
}
