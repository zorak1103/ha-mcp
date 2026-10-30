package handlers

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// script_execution / state values of a trace that handleTrigger words specially
// (components/automation/trace.py, helpers/script.py ScriptRunResult).
const (
	traceExecFinished       = "finished"
	traceExecFailedSingle   = "failed_single"
	traceExecFailedMaxRuns  = "failed_max_runs"
	traceExecFailedConds    = "failed_conditions"
	traceStateRunning       = "running"
	triggerRunSummaryFormat = "Run %s %s."
)

// automationTriggerResult is the json output of the trigger action. RunID and the fields derived
// from the trace are empty when the run could not be observed; Warning then says why.
type automationTriggerResult struct {
	EntityID        string `json:"entity_id"`
	RunID           string `json:"run_id,omitempty"`
	State           string `json:"state,omitempty"`
	ScriptExecution string `json:"script_execution,omitempty"`
	Error           string `json:"error,omitempty"`
	Summary         string `json:"summary,omitempty"`
	Warning         string `json:"warning,omitempty"`
}

// handleTrigger fires an automation through automation.trigger and reports the run it started.
//
// automation.trigger is a blocking WS call: Home Assistant answers only when the whole run has
// ended, which for a delay or wait_for_trigger action is longer than ha-mcp's WS request timeout,
// and it also accepts an unknown entity_id without error. So the call runs in a goroutine while
// trace/list is polled for a run that was not there before; the handler never gates on the call
// returning. HA stores a run's trace when the run starts, so the new run shows up within
// milliseconds, whether it later finishes, is rejected (failed_single) or fails.
func (h *AutomationHandlers) handleTrigger(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	entityID, errResult := parseTriggerTarget(args)
	if errResult != nil {
		return errResult, nil
	}
	format, _ := args["format"].(string)

	if _, err := client.GetState(ctx, entityID); err != nil {
		return errorResult(fmt.Sprintf("Cannot trigger %s: automation not found (%v). Use 'list' action to find IDs", entityID, err)), nil
	}

	serviceData := map[string]any{attrEntityID: entityID}
	if skip, ok := args["skip_condition"].(bool); ok {
		serviceData["skip_condition"] = skip
	}

	traceData, baselineRunID, followable := triggerTraceBaseline(ctx, client, entityID)
	waitCtx, stopWait, done := startTriggerCall(ctx, client, serviceData)
	defer stopWait()

	if followable {
		if run, found := waitForNewTrace(waitCtx, client, traceData, baselineRunID); found {
			return triggerOutput(newTriggerRunResult(entityID, run), format), nil
		}
	}
	if err := ctx.Err(); err != nil {
		return errorResult(fmt.Sprintf("Trigger of %s interrupted before a run was observed (%v); the automation may still have been triggered", entityID, err)), nil
	}

	finished, callErr := collectTriggerCall(ctx, done, !followable)
	if callErr != nil {
		return errorResult(fmt.Sprintf("Error triggering automation: %v", callErr)), nil
	}
	return triggerOutput(automationTriggerResult{
		EntityID: entityID,
		Warning:  triggerWarning(entityID, followable, finished, pollerConfigFromContext(ctx).Timeout),
	}, format), nil
}

// parseTriggerTarget validates automation_id and returns the automation's entity_id.
func parseTriggerTarget(args map[string]any) (string, *mcp.ToolsCallResult) {
	automationID, _ := args["automation_id"].(string)
	if automationID == "" {
		return "", errorResult("automation_id is required for trigger action. Use 'list' action to find IDs (shown in [brackets])")
	}
	if strings.Contains(automationID, ".") && !strings.HasPrefix(automationID, "automation.") {
		return "", errorResult(fmt.Sprintf("trigger action only supports automations, got: %q", automationID))
	}
	entityID, _ := normalizeAutomationID(automationID)
	return entityID, nil
}

// triggerTraceBaseline captures what the new run is compared against: the trace/list query for the
// automation and the run_id of its newest run (empty when it never ran). followable is false when
// the run cannot be observed at all - the entity_id does not resolve to a trace item_id (no
// registry unique_id, so HA stores no trace) or trace/list fails.
func triggerTraceBaseline(ctx context.Context, client homeassistant.Client, entityID string) (data map[string]any, baselineRunID string, followable bool) {
	itemID, resolved := resolveTraceItemID(ctx, client, traceDomainAutomation, entityID)
	if !resolved {
		return nil, "", false
	}
	data = map[string]any{"domain": traceDomainAutomation, "item_id": itemID}
	response, err := client.SendHACSCommand(ctx, "trace/list", data)
	if err != nil {
		return nil, "", false
	}
	if latest := latestTraceEntry(parseTraceListResponse(response)); latest != nil {
		baselineRunID = getMapString(latest, "run_id", "")
	}
	return data, baselineRunID, true
}

// startTriggerCall runs automation.trigger in a goroutine. done (buffered, so the goroutine never
// leaks) receives the call's result. waitCtx is canceled as soon as the call fails, which ends a
// trace poll that could never succeed; the result is sent before the cancel, so a poll that ends
// that way always finds it in done. A failure that arrives after a run was already observed is
// dropped, since the trigger demonstrably happened.
func startTriggerCall(ctx context.Context, client homeassistant.Client, serviceData map[string]any) (waitCtx context.Context, stop context.CancelFunc, done <-chan error) {
	waitCtx, stop = context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		_, err := client.CallService(ctx, "automation", "trigger", serviceData)
		result <- err
		if err != nil {
			slog.DebugContext(ctx, "automation.trigger call returned an error", attrEntityID, serviceData[attrEntityID], "error", err)
			stop()
		}
	}()
	return waitCtx, stop, result
}

// collectTriggerCall reads the outcome of the trigger call after no run was observed. finished is
// false while the call is still running. With block set it waits up to the configured wait
// timeout for the call, which is the only signal left when the run cannot be followed in the
// trace list; otherwise the poll has already used up that time, so it only peeks.
func collectTriggerCall(ctx context.Context, done <-chan error, block bool) (finished bool, err error) {
	if !block {
		select {
		case err = <-done:
			return true, err
		default:
			return false, nil
		}
	}

	timer := time.NewTimer(pollerConfigFromContext(ctx).Timeout)
	defer timer.Stop()
	select {
	case err = <-done:
		return true, err
	case <-timer.C:
		return false, nil
	case <-ctx.Done():
		return false, nil
	}
}

// triggerWarning explains why no new run was observed. It is a warning, not an error: the trigger
// itself was delivered (Smart Wait rule).
func triggerWarning(entityID string, followable, finished bool, timeout time.Duration) string {
	if !followable {
		msg := "the run could not be followed: the trace lookup is unavailable (the entity_id could not be " +
			"resolved to Home Assistant's trace item_id, or trace/list failed)"
		if !finished {
			msg += fmt.Sprintf("; the service call had not returned after %s, so the run is probably still in progress", timeout)
		}
		return msg
	}
	callState := " (the service call is still running)"
	if finished {
		callState = " although the service call returned"
	}
	return fmt.Sprintf("no new run appeared in the trace list within %s%s; check manage_trace action=get automation_id=%s",
		timeout, callState, entityID)
}

// newTriggerRunResult derives the trigger output from the new run's trace/list entry.
func newTriggerRunResult(entityID string, run map[string]any) automationTriggerResult {
	return automationTriggerResult{
		EntityID:        entityID,
		RunID:           getMapString(run, "run_id", ""),
		State:           getMapString(run, "state", ""),
		ScriptExecution: getMapString(run, "script_execution", ""),
		Error:           getMapString(run, "error", ""),
		Summary:         describeTriggerRun(run, entityID),
	}
}

// describeTriggerRun states what happened to the run in plain words. A rejected run
// (failed_single, failed_max_runs, failed_conditions) gets its own run_id and trace, so the
// existence of a new trace does not mean the actions ran - the sentence must say so.
func describeTriggerRun(run map[string]any, entityID string) string {
	runID := getMapString(run, "run_id", "")
	state := getMapString(run, "state", "")

	if errText := getMapString(run, "error", ""); errText != "" {
		return fmt.Sprintf("Run %s failed: %s.", runID, errText)
	}

	switch execution := getMapString(run, "script_execution", ""); execution {
	case traceExecFinished:
		return fmt.Sprintf(triggerRunSummaryFormat, runID, "finished")
	case traceExecFailedSingle:
		return fmt.Sprintf("Run %s was rejected: the automation's mode is 'single' and an earlier run is still in progress, so its actions did not run.", runID)
	case traceExecFailedMaxRuns:
		return fmt.Sprintf("Run %s was rejected: the automation's max concurrent runs limit is reached, so its actions did not run.", runID)
	case traceExecFailedConds:
		return fmt.Sprintf("Run %s stopped at the conditions, so its actions did not run (skip_condition=false was in effect; pass true to bypass them).", runID)
	case "":
		if state == traceStateRunning {
			return fmt.Sprintf("Run %s is still running; fetch its result with manage_trace action=get automation_id=%s run_id=%s.", runID, entityID, runID)
		}
		return fmt.Sprintf(triggerRunSummaryFormat, runID, "ended with state "+cmp.Or(state, "unknown"))
	default:
		return fmt.Sprintf(triggerRunSummaryFormat, runID, fmt.Sprintf("ended with script_execution %s (state: %s)", execution, cmp.Or(state, "unknown")))
	}
}

// triggerOutput renders the trigger result as natural text (default) or json.
func triggerOutput(result automationTriggerResult, format string) *mcp.ToolsCallResult {
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal trigger result: %v", err))
		}
		return successResult(string(jsonData))
	}

	text := "Triggered " + result.EntityID + "."
	if result.Summary != "" {
		text += " " + result.Summary
	}
	if result.Warning != "" {
		text += " (warning: " + result.Warning + ")"
	}
	return successResult(text)
}
