package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
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
		Description: "View automation and script execution traces. Supports list (view all traces), get (view one trace's details: pass automation_id or entity_id; run_id defaults to the newest run), and debug (consolidated automation debug report: config, latest trace, trigger entity states, and logbook).",
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
					Description: "Domain to query traces for: 'automation' or 'script'. Required for the 'list' action unless entity_id is provided (which auto-derives it). Optional for 'get' (derived from entity_id/automation_id; must agree when given).",
					Enum:        []string{traceDomainAutomation, traceDomainScript},
				},
				attrEntityID: {
					Type:        "string",
					Description: "Entity ID, e.g. 'automation.morning_routine' or 'script.wake_up'. For 'get': required unless automation_id is given (auto-derives domain). For 'list': optional, auto-derives domain and filters traces by item.",
				},
				"automation_id": {
					Type:        "string",
					Description: "Automation entity ID or bare ID, e.g. 'automation.morning_routine' or 'morning_routine'. Required for 'debug'; for 'get' an alternative to entity_id (both must name the same automation if both are given).",
				},
				"run_id": {
					Type:        "string",
					Description: "Trace run ID for 'get'. Optional: when omitted, the newest run is fetched and reported as the selected run.",
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
	derived, errMsg := validateTraceEntityID(entityID, domain)
	if errMsg != "" {
		return "", "", errMsg
	}
	return derived, entityID, ""
}

// validateTraceEntityID checks an entity_id intended for a trace lookup: it must be
// "automation.<id>" or "script.<id>", and - when domain is already known (get always supplies
// it; list only when explicitly passed) - the prefixes must agree. Shared by handleGetTrace and
// resolveTraceListParams so the two actions can't drift on what counts as a valid entity_id
// (get's own inline check used to accept a bare id or one starting with "." unchecked).
func validateTraceEntityID(entityID, domain string) (derivedDomain, errMsg string) {
	dotIdx := strings.Index(entityID, ".")
	if dotIdx <= 0 {
		return "", fmt.Sprintf("entity_id %q is invalid: must be 'automation.<id>' or 'script.<id>'", entityID)
	}
	derived := entityID[:dotIdx]
	if derived != traceDomainAutomation && derived != traceDomainScript {
		return "", fmt.Sprintf("entity_id prefix %q is not supported for trace lookups; use 'automation' or 'script'", derived)
	}
	if domain != "" && domain != derived {
		return "", fmt.Sprintf("entity_id prefix %q conflicts with explicit domain %q", derived, domain)
	}
	return derived, ""
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
// Uses GetEntityRegistryEntry (a targeted "config/entity_registry/get" WS call) rather than
// GetEntityRegistry, which would fetch and unmarshal every registered entity - one of the
// heaviest WS reads available - just to find one entry, on every entity-scoped trace lookup.
//
// A failed lookup or a missing unique_id degrades to the object_id and logs a warning: HA
// returns an empty trace list rather than an error for a wrong key, so a silently wrong item_id
// is otherwise indistinguishable from "no traces recorded yet". The returned resolved=false lets
// callers distinguish the two cases in the tool response too, not just the server log - see
// unresolvedItemIDWarning.
func resolveTraceItemID(ctx context.Context, client homeassistant.Client, domain, entityID string) (itemID string, resolved bool) {
	objectID := entityID
	if dotIdx := strings.Index(entityID, "."); dotIdx >= 0 {
		objectID = entityID[dotIdx+1:]
	}

	entry, err := client.GetEntityRegistryEntry(ctx, entityID)
	if err != nil {
		slog.WarnContext(ctx, "trace item_id resolution degraded to object_id: entity registry lookup failed",
			attrEntityID, entityID, "domain", domain, "error", err)
		return objectID, false
	}
	if entry.UniqueID == "" {
		slog.WarnContext(ctx, "trace item_id resolution degraded to object_id: entity has no unique_id in registry",
			attrEntityID, entityID, "domain", domain)
		return objectID, false
	}
	return entry.UniqueID, true
}

// unresolvedItemIDWarning explains an empty trace result that followed a failed item_id
// resolution, so the caller doesn't mistake a known-wrong lookup key for "no traces yet" and
// retry (or poll via wait=true) something that can never succeed.
func unresolvedItemIDWarning(entityID string) string {
	return fmt.Sprintf("No traces found for %s, but this may be because the entity_id could not be "+
		"resolved to Home Assistant's trace item_id (registry lookup failed or the entity is not "+
		"registered) rather than an absence of traces - retrying or passing wait=true will not help. "+
		"Check the entity_id is correct and the entity exists.", entityID)
}

// handleListTraces lists all traces for a domain.
func (h *TraceHandlers) handleListTraces(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	domain, _ := args["domain"].(string)
	entityID, _ := args[attrEntityID].(string)
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
	resolved := true
	if traceEntityID != "" {
		var itemID string
		itemID, resolved = resolveTraceItemID(ctx, client, domain, traceEntityID)
		data["item_id"] = itemID
	}

	// Call trace/list WebSocket command
	response, err := client.SendHACSCommand(ctx, "trace/list", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to list traces: %v", err)), nil
	}

	// Parse response - convert to []any for consistent handling
	traces := parseTraceListResponse(response)

	// Opt-in polling: if wait=true and no traces returned yet, poll until traces appear.
	// Skipped when item_id resolution failed - polling a key already known to be wrong just
	// burns the full wait timeout with no chance of ever finding a trace.
	if wait && len(traces) == 0 && resolved {
		if polled, found := waitForTraces(ctx, client, data); found {
			traces = polled
		}
	}

	warning := ""
	if !resolved && len(traces) == 0 {
		warning = unresolvedItemIDWarning(traceEntityID)
	}

	if format == formatJSON {
		return formatTraceListJSON(response, traces, wait, warning)
	}

	// Natural format
	return successResult(h.formatTracesNatural(traces, warning)), nil
}

// formatTraceListJSON renders the list action's JSON output: the resolution-failure warning
// (wrapped alongside an empty traces array) takes precedence, then a wait-polled non-empty
// result, falling back to the raw trace/list response.
func formatTraceListJSON(response any, traces []any, wait bool, warning string) (*mcp.ToolsCallResult, error) {
	if warning != "" {
		jsonData, err := json.MarshalIndent(map[string]any{"traces": traces, "warning": warning}, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal traces: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	toMarshal := response
	if wait && len(traces) > 0 {
		toMarshal = traces
	}
	jsonData, err := json.MarshalIndent(toMarshal, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("failed to marshal traces: %v", err)), nil
	}
	return successResult(string(jsonData)), nil
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

// latestTraceEntry returns the trace with the most recent start timestamp, or nil when the list
// holds no trace maps. ISO timestamps sort lexicographically. HA's trace/list returns traces in
// insertion order (oldest first), so the first element is NOT the latest - it must be resolved by
// comparing timestamps. Entries flagged not_triggered (a trigger fired but its conditions stopped
// the run before it started) are skipped: they are not runs and carry no run to inspect.
func latestTraceEntry(traces []any) map[string]any {
	var latest map[string]any
	var latestStart string
	for _, t := range traces {
		traceMap, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if notTriggered, _ := traceMap["not_triggered"].(bool); notTriggered {
			continue
		}
		start, _ := traceTimestamps(traceMap)
		if latest == nil || start > latestStart {
			latest = traceMap
			latestStart = start
		}
	}
	return latest
}

// resolveTraceGetTarget turns the get action's automation_id / entity_id / domain arguments into
// one trace target. automation_id (bare or "automation."-prefixed) is an alternative spelling of
// an automation entity_id; when both are given they must name the same entity. The domain is
// derived from the entity_id prefix, and an explicit domain must agree with it - a mismatch would
// otherwise resolve item_id against the wrong domain and return an unrelated entity's traces.
func resolveTraceGetTarget(args map[string]any) (domain, entityID, errMsg string) {
	explicitDomain, _ := args["domain"].(string)
	entityID, _ = args[attrEntityID].(string)

	if automationID, _ := args["automation_id"].(string); automationID != "" {
		if strings.Contains(automationID, ".") && !strings.HasPrefix(automationID, "automation.") {
			return "", "", fmt.Sprintf("automation_id %q is invalid: must be 'automation.<id>' or a bare automation id", automationID)
		}
		autoEntityID, _ := normalizeAutomationID(automationID)
		if entityID != "" && entityID != autoEntityID {
			return "", "", fmt.Sprintf("automation_id %q conflicts with entity_id %q", automationID, entityID)
		}
		entityID = autoEntityID
	}

	if entityID == "" {
		return "", "", "entity_id or automation_id is required for get action"
	}

	derived, errMsg := validateTraceEntityID(entityID, explicitDomain)
	if errMsg != "" {
		return "", "", errMsg
	}
	return derived, entityID, ""
}

// handleGetTrace retrieves one trace by automation_id or entity_id. Without run_id it lists the
// traces under the same item_id and fetches the newest one.
func (h *TraceHandlers) handleGetTrace(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	domain, entityID, errMsg := resolveTraceGetTarget(args)
	if errMsg != "" {
		return errorResult(errMsg), nil
	}

	itemID, resolved := resolveTraceItemID(ctx, client, domain, entityID)

	runID, _ := args["run_id"].(string)
	selectedRunID := ""
	if runID == "" {
		latest, result := latestTraceRunID(ctx, client, domain, itemID, entityID, resolved, format)
		if result != nil {
			return result, nil
		}
		runID, selectedRunID = latest, latest
	}

	// Build command data
	data := map[string]any{
		"domain":  domain,
		"item_id": itemID,
		"run_id":  runID,
	}

	// Call trace/get WebSocket command
	response, err := client.SendHACSCommand(ctx, "trace/get", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get trace: %v", err)), nil
	}

	// A wrong item_id returns an empty/nil result rather than an error (same trap as list) - flag
	// it instead of rendering "Invalid trace data." with no indication the lookup itself is suspect.
	warning := ""
	if !resolved && response == nil {
		warning = unresolvedItemIDWarning(entityID)
	}

	if format == formatJSON {
		return formatTraceGetJSON(response, selectedRunID, warning)
	}

	// Natural format
	natural := h.formatTraceNatural(response)
	if selectedRunID != "" {
		natural = fmt.Sprintf("Run ID: %s (latest)\n\n%s", selectedRunID, natural)
	}
	if warning != "" {
		natural += "\n\n" + warning
	}
	return successResult(natural), nil
}

// latestTraceRunID lists the traces under itemID and returns the newest run_id. When there is
// none (or trace/list fails) it returns the finished tool result instead: an error for a failed
// call, otherwise a non-error "no traces" message - the unresolved-item_id warning when the
// lookup key itself is suspect, so the caller doesn't retry something that can never succeed.
func latestTraceRunID(ctx context.Context, client homeassistant.Client, domain, itemID, entityID string, resolved bool, format string) (string, *mcp.ToolsCallResult) {
	response, err := client.SendHACSCommand(ctx, "trace/list", map[string]any{
		"domain":  domain,
		"item_id": itemID,
	})
	if err != nil {
		return "", errorResult(fmt.Sprintf("failed to list traces: %v", err))
	}

	if latest := latestTraceEntry(parseTraceListResponse(response)); latest != nil {
		if runID := getMapString(latest, "run_id", ""); runID != "" {
			return runID, nil
		}
	}

	warning := unresolvedItemIDWarning(entityID)
	if resolved {
		warning = fmt.Sprintf("No traces found for %s. Home Assistant stores a run's trace the moment the run "+
			"starts, so no run is recorded for it. To wait for an upcoming run, poll with "+
			"manage_trace action=list wait=true.", entityID)
	}
	if format != formatJSON {
		return "", successResult(warning)
	}
	jsonData, err := json.MarshalIndent(map[string]any{"trace": nil, "warning": warning}, "", "  ")
	if err != nil {
		return "", errorResult(fmt.Sprintf("failed to marshal trace: %v", err))
	}
	return "", successResult(string(jsonData))
}

// formatTraceGetJSON renders the get action's JSON output. A warning wraps the response as
// {"trace", "warning"}. A defaulted run adds "selected_run_id" next to the trace's own keys, so the
// shape stays the same as an explicit-run_id call; an explicit call is marshaled unchanged.
func formatTraceGetJSON(response any, selectedRunID, warning string) (*mcp.ToolsCallResult, error) {
	toMarshal := response
	switch {
	case warning != "":
		wrapped := map[string]any{"trace": response, "warning": warning}
		if selectedRunID != "" {
			wrapped["selected_run_id"] = selectedRunID
		}
		toMarshal = wrapped
	case selectedRunID != "":
		if traceMap, ok := response.(map[string]any); ok {
			withRun := make(map[string]any, len(traceMap)+1)
			maps.Copy(withRun, traceMap)
			withRun["selected_run_id"] = selectedRunID
			toMarshal = withRun
		} else {
			toMarshal = map[string]any{"trace": response, "selected_run_id": selectedRunID}
		}
	}
	jsonData, err := json.MarshalIndent(toMarshal, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("failed to marshal trace: %v", err)), nil
	}
	return successResult(string(jsonData)), nil
}

// formatTracesNatural formats a list of traces in natural language.
func (h *TraceHandlers) formatTracesNatural(traces []any, warning string) string {
	if len(traces) == 0 {
		if warning != "" {
			return warning
		}
		return "No traces found. Home Assistant stores a run's trace the moment the run starts, so no run is recorded for this selection. To wait for an upcoming run, pass wait=true to poll automatically."
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
		duration := traceDuration(start, finish)

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

	// state/script_execution/error/not_triggered live on the top-level short dict fields that
	// AutomationTrace.as_extended_dict() includes verbatim alongside "trace" (components/trace/models.py).
	if state := getMapString(traceMap, "state", ""); state != "" {
		parts = append(parts, fmt.Sprintf("\nState: %s", state))
	}

	// The trigger description lives on the top-level short dict (AutomationTrace.as_short_dict
	// sets result["trigger"]), not nested under "trace".
	if trigger := getMapString(traceMap, "trigger", ""); trigger != "" {
		parts = append(parts, fmt.Sprintf("\nTrigger: %s", trigger))
	}

	if execResult := getMapString(traceMap, "script_execution", ""); execResult != "" {
		parts = append(parts, fmt.Sprintf("\nResult: %s", execResult))
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

	if nt, ok := traceMap["not_triggered"].(bool); ok && nt {
		parts = append(parts, "\nNot triggered: condition evaluated but did not fire")
	}
	if errText := getMapString(traceMap, "error", ""); errText != "" {
		parts = append(parts, fmt.Sprintf("\nError: %s", errText))
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
// Home Assistant's trace API always returns this as {"start": <iso>, "finish": <iso>}
// (components/trace/models.py ActionTrace.as_short_dict) - there is no flat-string variant.
func traceTimestamps(m map[string]any) (start, finish string) {
	v, ok := m["timestamp"].(map[string]any)
	if !ok {
		return "", ""
	}
	return getMapString(v, "start", ""), getMapString(v, "finish", "")
}

// traceDuration returns a trace's execution duration in seconds, derived from the finish/start
// timestamps - Home Assistant's trace short dict has no "duration" key of its own.
func traceDuration(start, finish string) float64 {
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
