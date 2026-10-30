package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const (
	triggerTestEntityID = "automation.morning"
	triggerTestItemID   = "cfg_morning"
)

// recordedService is one CallService invocation seen by the trigger test client.
type recordedService struct {
	domain  string
	service string
	data    map[string]any
}

// triggerRecorder counts what the handler asked of Home Assistant.
type triggerRecorder struct {
	mu        sync.Mutex
	services  []recordedService
	listCalls int
}

func (r *triggerRecorder) recordService(domain, service string, data map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services = append(r.services, recordedService{domain, service, data})
}

// nextList returns how many trace/list calls came before this one (0 = the baseline lookup).
func (r *triggerRecorder) nextList() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listCalls++
	return r.listCalls - 1
}

func (r *triggerRecorder) serviceCalls() []recordedService {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.services)
}

func (r *triggerRecorder) traceListCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.listCalls
}

// triggerTrace builds a trace/list entry as HA reports it.
func triggerTrace(runID, start, state, scriptExecution string) map[string]any {
	trace := map[string]any{
		"run_id":    runID,
		"state":     state,
		"timestamp": map[string]any{"start": start},
	}
	if scriptExecution != "" {
		trace["script_execution"] = scriptExecution
	}
	return trace
}

// newTriggerClient returns a mock for automation.morning. listFn receives the 0-based index of
// the trace/list call (0 is the baseline lookup before the service call). callFn, when set,
// decides how the automation.trigger call behaves; nil means it returns at once.
func newTriggerClient(listFn func(call int) []any, callFn func(ctx context.Context) error) (*UniversalMockClient, *triggerRecorder) {
	rec := &triggerRecorder{}
	client := &UniversalMockClient{
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			if entityID != triggerTestEntityID {
				return nil, fmt.Errorf("entity %s not found", entityID)
			}
			return &homeassistant.Entity{EntityID: entityID, State: "on"}, nil
		},
		GetEntityRegistryEntryFn: func(_ context.Context, entityID string) (*homeassistant.EntityRegistryEntry, error) {
			if entityID != triggerTestEntityID {
				return nil, fmt.Errorf("wrong entity %s", entityID)
			}
			return &homeassistant.EntityRegistryEntry{EntityID: entityID, UniqueID: triggerTestItemID}, nil
		},
		SendHACSCommandFn: func(_ context.Context, command string, data map[string]any) (any, error) {
			if command != "trace/list" {
				return nil, fmt.Errorf("wrong command: %s", command)
			}
			if data["domain"] != "automation" || data["item_id"] != triggerTestItemID {
				return nil, fmt.Errorf("wrong trace/list data: %v", data)
			}
			// A run cannot exist before automation.trigger was called, so until then the
			// list stays at the baseline view. This also keeps the handler from returning
			// before its goroutine has reached CallService.
			call := rec.nextList()
			if call > 0 && len(rec.serviceCalls()) == 0 {
				call = 0
			}
			return listFn(call), nil
		},
		CallServiceFn: func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "automation" || service != "trigger" {
				return nil, fmt.Errorf("wrong service: %s.%s", domain, service)
			}
			rec.recordService(domain, service, data)
			if callFn != nil {
				return nil, callFn(ctx)
			}
			return nil, nil
		},
	}
	return client, rec
}

// newRunAfterBaseline reports the old run r1 for the baseline lookup and the extra run from then on.
func newRunAfterBaseline(extra map[string]any) func(call int) []any {
	oldRun := triggerTrace("r1", "2026-10-30T08:00:00Z", "stopped", "finished")
	return func(call int) []any {
		if call == 0 {
			return []any{oldRun}
		}
		return []any{oldRun, extra}
	}
}

// blockingCall returns a callFn that blocks until the returned release func is called (also
// registered as cleanup), then fails with err.
func blockingCall(t *testing.T, err error) (callFn func(context.Context) error, release func()) {
	t.Helper()
	gate := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(gate) }) }
	t.Cleanup(release)
	return func(context.Context) error {
		<-gate
		return err
	}, release
}

func runTrigger(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	if args["action"] == nil {
		args["action"] = "trigger"
	}
	return NewAutomationHandlers().handleManageAutomation(ctx, client, args)
}

func resultText(t *testing.T, result *mcp.ToolsCallResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}
	return result.Content[0].Text
}

func TestTrigger_RequiresAutomationID(t *testing.T) {
	t.Parallel()

	client, rec := newTriggerClient(newRunAfterBaseline(nil), nil)
	result, err := runTrigger(injectFastWait(time.Second), client, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "automation_id is required") {
		t.Errorf("want an automation_id-required error, got %q", resultText(t, result))
	}
	if len(rec.serviceCalls()) != 0 {
		t.Error("CallService must not be called without automation_id")
	}
}

func TestTrigger_RejectsNonAutomationID(t *testing.T) {
	t.Parallel()

	client, rec := newTriggerClient(newRunAfterBaseline(nil), nil)
	result, err := runTrigger(injectFastWait(time.Second), client, map[string]any{"automation_id": "script.morning"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "only supports automations") {
		t.Errorf("want an automations-only error, got %q", resultText(t, result))
	}
	if len(rec.serviceCalls()) != 0 {
		t.Error("CallService must not be called for a non-automation id")
	}
}

func TestTrigger_UnknownEntityIsAnError(t *testing.T) {
	t.Parallel()

	client, rec := newTriggerClient(newRunAfterBaseline(nil), nil)
	result, err := runTrigger(injectFastWait(time.Second), client, map[string]any{"automation_id": "no_such_automation"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// HA accepts automation.trigger for an unknown entity_id and does nothing, so the handler
	// has to catch it before the call.
	if !result.IsError || !strings.Contains(resultText(t, result), "automation.no_such_automation") {
		t.Errorf("want an error naming the entity, got %q", resultText(t, result))
	}
	if len(rec.serviceCalls()) != 0 {
		t.Error("CallService must not be called for an unknown entity")
	}
}

func TestTrigger_ServiceCallShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     map[string]any
		wantSkip any // nil = skip_condition must be absent from the service data
	}{
		{name: "bare id, no skip_condition", args: map[string]any{"automation_id": "morning"}},
		{name: "prefixed id", args: map[string]any{"automation_id": triggerTestEntityID}},
		{name: "skip_condition true", args: map[string]any{"automation_id": "morning", "skip_condition": true}, wantSkip: true},
		{name: "skip_condition false", args: map[string]any{"automation_id": "morning", "skip_condition": false}, wantSkip: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, rec := newTriggerClient(newRunAfterBaseline(triggerTrace("r2", "2026-10-30T09:00:00Z", "stopped", "finished")), nil)
			result, err := runTrigger(injectFastWait(time.Second), client, tt.args)
			if err != nil || result.IsError {
				t.Fatalf("unexpected failure: %v / %q", err, resultText(t, result))
			}

			calls := rec.serviceCalls()
			if len(calls) != 1 {
				t.Fatalf("CallService called %d times, want 1", len(calls))
			}
			data := calls[0].data
			if data["entity_id"] != triggerTestEntityID {
				t.Errorf("entity_id = %v, want %s", data["entity_id"], triggerTestEntityID)
			}
			got, present := data["skip_condition"]
			switch {
			case tt.wantSkip == nil && present:
				t.Errorf("skip_condition = %v, want it absent (HA defaults it to true)", got)
			case tt.wantSkip != nil && got != tt.wantSkip:
				t.Errorf("skip_condition = %v, want %v", got, tt.wantSkip)
			}
			if len(data) > 2 {
				t.Errorf("unexpected extra service data: %v", data)
			}
		})
	}
}

func TestTrigger_ReportsNewRun(t *testing.T) {
	t.Parallel()

	// The mock counts trace/list calls to tell the baseline lookup apart, so each subtest needs its own.
	newClient := func() *UniversalMockClient {
		client, _ := newTriggerClient(newRunAfterBaseline(triggerTrace("r2", "2026-10-30T09:00:00Z", "stopped", "finished")), nil)
		return client
	}

	t.Run("natural", func(t *testing.T) {
		t.Parallel()

		client := newClient()
		result, err := runTrigger(injectFastWait(time.Second), client, map[string]any{"automation_id": "morning"})
		if err != nil || result.IsError {
			t.Fatalf("unexpected failure: %v / %q", err, resultText(t, result))
		}
		text := resultText(t, result)
		for _, want := range []string{triggerTestEntityID, "r2", "finished"} {
			if !strings.Contains(text, want) {
				t.Errorf("output %q should contain %q", text, want)
			}
		}
		if strings.Contains(text, "r1") {
			t.Errorf("output %q must name the new run, not the baseline", text)
		}
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()

		client := newClient()
		result, err := runTrigger(injectFastWait(time.Second), client, map[string]any{"automation_id": "morning", "format": "json"})
		if err != nil || result.IsError {
			t.Fatalf("unexpected failure: %v / %q", err, resultText(t, result))
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(resultText(t, result)), &got); err != nil {
			t.Fatalf("output is not JSON: %v", err)
		}
		if got["run_id"] != "r2" || got["entity_id"] != triggerTestEntityID || got["script_execution"] != "finished" {
			t.Errorf("unexpected json: %v", got)
		}
		if _, hasWarning := got["warning"]; hasWarning {
			t.Errorf("a followed run must not carry a warning: %v", got)
		}
	})
}

// A rejected run gets its own run_id, so a new trace is no proof that the actions ran.
func TestTrigger_RejectedRunIsNotReportedAsRan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		state           string
		scriptExecution string
		errText         string
		wantAll         []string
	}{
		{name: "failed_single", state: "stopped", scriptExecution: "failed_single", wantAll: []string{"rejected", "single", "did not run"}},
		{name: "failed_max_runs", state: "stopped", scriptExecution: "failed_max_runs", wantAll: []string{"rejected", "max", "did not run"}},
		{name: "failed_conditions", state: "stopped", scriptExecution: "failed_conditions", wantAll: []string{"conditions", "did not run", "skip_condition"}},
		{name: "error", state: "stopped", scriptExecution: "error", errText: "boom", wantAll: []string{"failed", "boom"}},
		{name: "still running", state: "running", wantAll: []string{"still running", "manage_trace"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			run := triggerTrace("r2", "2026-10-30T09:00:00Z", tt.state, tt.scriptExecution)
			if tt.errText != "" {
				run["error"] = tt.errText
			}
			client, _ := newTriggerClient(newRunAfterBaseline(run), nil)

			result, err := runTrigger(injectFastWait(time.Second), client, map[string]any{"automation_id": "morning"})
			if err != nil || result.IsError {
				t.Fatalf("a delivered trigger is not a tool error: %v / %q", err, resultText(t, result))
			}
			text := resultText(t, result)
			for _, want := range tt.wantAll {
				if !strings.Contains(text, want) {
					t.Errorf("output %q should contain %q", text, want)
				}
			}
			if strings.Contains(text, "finished") {
				t.Errorf("output %q must not claim the run finished", text)
			}
		})
	}
}

func TestTrigger_TimeoutIsSuccessWithWarning(t *testing.T) {
	t.Parallel()

	oldRunOnly := func(int) []any {
		return []any{triggerTrace("r1", "2026-10-30T08:00:00Z", "stopped", "finished")}
	}
	callFn, _ := blockingCall(t, nil)
	client, rec := newTriggerClient(oldRunOnly, callFn)

	result, err := runTrigger(injectFastWait(80*time.Millisecond), client, map[string]any{"automation_id": "morning"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("a wait timeout is a warning, not an error: %q", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "warning") || !strings.Contains(text, "manage_trace") || !strings.Contains(text, triggerTestEntityID) {
		t.Errorf("want a warning pointing to manage_trace get for %s, got %q", triggerTestEntityID, text)
	}
	if len(rec.serviceCalls()) != 1 {
		t.Errorf("CallService called %d times, want 1", len(rec.serviceCalls()))
	}
}

func TestTrigger_CallFailureBeforeAnyTraceIsAnError(t *testing.T) {
	t.Parallel()

	oldRunOnly := func(int) []any {
		return []any{triggerTrace("r1", "2026-10-30T08:00:00Z", "stopped", "finished")}
	}
	client, _ := newTriggerClient(oldRunOnly, func(context.Context) error { return errors.New("service rejected") })

	// A long wait proves the poll is cut short by the failed call instead of running it out.
	start := time.Now()
	result, err := runTrigger(injectFastWait(10*time.Second), client, map[string]any{"automation_id": "morning"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "service rejected") {
		t.Errorf("want an error carrying the call failure, got %q", resultText(t, result))
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("handler waited %v for a trace after the call had already failed", elapsed)
	}
}

// automation.trigger blocks for the whole run, so the handler must not wait for the call.
func TestTrigger_DoesNotWaitForLongRunningCall(t *testing.T) {
	t.Parallel()

	// The call stays blocked and then fails the way a >30s run does in ha-mcp (request timeout).
	callFn, release := blockingCall(t, errors.New("request timeout"))
	client, _ := newTriggerClient(newRunAfterBaseline(triggerTrace("r2", "2026-10-30T09:00:00Z", "running", "")), callFn)

	done := make(chan *mcp.ToolsCallResult, 1)
	go func() {
		result, _ := runTrigger(injectFastWait(5*time.Second), client, map[string]any{"automation_id": "morning"})
		done <- result
	}()

	select {
	case result := <-done:
		if result.IsError {
			t.Fatalf("trace was found, so the trigger is a success: %q", resultText(t, result))
		}
		if text := resultText(t, result); !strings.Contains(text, "r2") {
			t.Errorf("output %q should name run r2", text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler is waiting for the blocking automation.trigger call to return")
	}

	// The late failure after the trace was found must be dropped, not panic or leak.
	release()
}

func TestTrigger_UnresolvableTraceKeyStillTriggers(t *testing.T) {
	t.Parallel()

	client, rec := newTriggerClient(newRunAfterBaseline(nil), nil)
	client.GetEntityRegistryEntryFn = func(context.Context, string) (*homeassistant.EntityRegistryEntry, error) {
		return nil, errors.New("not registered")
	}

	result, err := runTrigger(injectFastWait(time.Second), client, map[string]any{"automation_id": "morning"})
	if err != nil || result.IsError {
		t.Fatalf("unexpected failure: %v / %q", err, resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "could not be resolved") {
		t.Errorf("want a note that the run cannot be followed, got %q", resultText(t, result))
	}
	if len(rec.serviceCalls()) != 1 {
		t.Errorf("CallService called %d times, want 1", len(rec.serviceCalls()))
	}
	if rec.traceListCalls() != 0 {
		t.Errorf("trace/list called %d times with an unresolved item_id, want 0", rec.traceListCalls())
	}
}

func TestTrigger_UnresolvableTraceKeyReportsCallFailure(t *testing.T) {
	t.Parallel()

	client, _ := newTriggerClient(newRunAfterBaseline(nil), func(context.Context) error { return errors.New("service rejected") })
	client.GetEntityRegistryEntryFn = func(context.Context, string) (*homeassistant.EntityRegistryEntry, error) {
		return nil, errors.New("not registered")
	}

	result, err := runTrigger(injectFastWait(time.Second), client, map[string]any{"automation_id": "morning"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(t, result), "service rejected") {
		t.Errorf("want the call failure as an error, got %q", resultText(t, result))
	}
}

func TestDescribeTriggerRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		trace map[string]any
		want  string
	}{
		{name: "finished", trace: triggerTrace("r2", "t", "stopped", "finished"), want: "finished"},
		{name: "unknown script_execution", trace: triggerTrace("r2", "t", "stopped", "aborted"), want: "aborted"},
		{name: "no script_execution, stopped", trace: triggerTrace("r2", "t", "stopped", ""), want: "stopped"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := describeTriggerRun(tt.trace, triggerTestEntityID); !strings.Contains(got, tt.want) || !strings.Contains(got, "r2") {
				t.Errorf("describeTriggerRun = %q, want it to name r2 and %q", got, tt.want)
			}
		})
	}
}

func TestManageAutomationSchema_TriggerAction(t *testing.T) {
	t.Parallel()

	tool := NewAutomationHandlers().manageAutomationTool()

	if !slices.Contains(tool.InputSchema.Properties["action"].Enum, automationActionTrigger) {
		t.Errorf("action enum %v should contain %q", tool.InputSchema.Properties["action"].Enum, automationActionTrigger)
	}
	if got := tool.InputSchema.Properties["skip_condition"].Type; got != "boolean" {
		t.Errorf("skip_condition type = %q, want boolean", got)
	}
	if !strings.Contains(tool.Description, "- trigger:") {
		t.Error("tool description should document the trigger action")
	}
}
