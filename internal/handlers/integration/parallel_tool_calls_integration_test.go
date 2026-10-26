//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/zorak1103/ha-mcp/internal/mcp"
)

const (
	parallelCallCount   = 24
	parallelCallTimeout = 60 * time.Second

	// helperTemplateBurstIndex is the one burst slot that scans helper_template
	// (see helperTemplateCall). It must be an index of a find_references call.
	helperTemplateBurstIndex = 2
)

// ParallelToolCallsIntegrationTestSuite is a regression guard for #294: a burst
// of concurrent tool calls over one shared client must not trip the WebSocket
// layer (HA's id_reuse rejection, a dropped connection surfacing as
// "not connected"). The bug is already fixed on main, so this test passes there
// by design; it exists to catch a regression, not to demonstrate the bug.
type ParallelToolCallsIntegrationTestSuite struct {
	IntegrationTestSuite
}

func TestParallelToolCalls(t *testing.T) {
	suite.Run(t, new(ParallelToolCallsIntegrationTestSuite))
}

// parallelCall is one read-only tool invocation of the burst.
type parallelCall struct {
	tool string
	args map[string]any
}

// parallelCallOutcome is what one goroutine reports back. Each goroutine owns
// exactly one slot of a pre-sized slice, so no shared mutation is needed.
type parallelCallOutcome struct {
	call    parallelCall
	result  *mcp.ToolsCallResult
	err     error
	missing bool     // tool not registered
	unknown []string // argument names the server would reject
	elapsed time.Duration
}

// readOnlyCalls returns the four read-only calls the burst cycles through.
// Argument names were checked against each tool's InputSchema.
func readOnlyCalls() []parallelCall {
	return []parallelCall{
		{tool: "get_state", args: map[string]any{"entity_id": "sun.sun"}},
		{tool: "get_registry", args: map[string]any{"type": "areas"}},
		// types skips automation and script: scanning those fetches every config one
		// by one (~100 websocket round trips per call on a real instance), which
		// under 24 concurrent goroutines outlasts the per-call timeout.
		{tool: "find_references", args: map[string]any{
			"search": "sun.sun",
			"format": "json",
			"types":  []any{"scene", "dashboard"},
		}},
		{tool: "manage_automation", args: map[string]any{"action": "list"}},
	}
}

// helperTemplateCall is the find_references call that scans helper_template. It
// reads each template helper through its own options flow, about two REST
// requests per helper, and all REST requests share one rate limiter (10 req/s by
// default). On the integration instance one such call takes seconds and several
// at once take proportionally longer: six of them need more than find_references'
// own 25 s scan deadline and correctly report helper_template as a failed
// source. The burst therefore makes this call exactly once (at
// helperTemplateBurstIndex) so the source stays covered under load without
// depending on the rate limiter's throughput.
func helperTemplateCall() parallelCall {
	return parallelCall{tool: "find_references", args: map[string]any{
		"search": "sun.sun",
		"format": "json",
		"types":  []any{"helper_template"},
	}}
}

// burstCall returns the call goroutine i of the burst makes: the four read-only
// calls in rotation, with one find_references slot swapped for helperTemplateCall.
func burstCall(calls []parallelCall, i int) parallelCall {
	if i == helperTemplateBurstIndex {
		return helperTemplateCall()
	}
	return calls[i%len(calls)]
}

// runParallelCall executes a call the way the server would: reject undeclared
// argument names first (mirroring dispatchTool), then invoke the handler. It
// must not touch the testing.T, because it runs off the test goroutine.
func (s *ParallelToolCallsIntegrationTestSuite) runParallelCall(call parallelCall) parallelCallOutcome {
	out := parallelCallOutcome{call: call}

	handler, ok := s.registry.GetHandler(call.tool)
	if !ok {
		out.missing = true
		return out
	}
	tool, _ := s.registry.GetTool(call.tool)
	out.unknown = mcp.UnknownArguments(tool.InputSchema, call.args)
	if len(out.unknown) > 0 {
		return out
	}

	ctx, cancel := context.WithTimeout(s.ctx, parallelCallTimeout)
	defer cancel()

	began := time.Now()
	out.result, out.err = handler(ctx, s.client, call.args)
	out.elapsed = time.Since(began)
	return out
}

// TestBurstOfReadOnlyCalls fires parallelCallCount concurrent read-only tool
// calls and asserts that none fails with a transport-level symptom of #294.
func (s *ParallelToolCallsIntegrationTestSuite) TestBurstOfReadOnlyCalls() {
	calls := readOnlyCalls()
	outcomes := make([]parallelCallOutcome, parallelCallCount)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range outcomes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release all goroutines together to maximize overlap
			outcomes[i] = s.runParallelCall(burstCall(calls, i))
		}(i)
	}
	close(start)
	wg.Wait()

	for i, out := range outcomes {
		label := out.call.tool
		s.Require().Falsef(out.missing, "call %d: tool %q is not registered", i, label)
		s.Require().Emptyf(out.unknown, "call %d: %q has undeclared arguments (the server would reject them)", i, label)
		s.Require().NoErrorf(out.err, "call %d: handler for %q returned a Go error", i, label)
		s.Require().NotNilf(out.result, "call %d: %q returned a nil result", i, label)

		text := resultText(out.result)
		s.T().Logf("call %d: %s took %s", i, label, out.elapsed.Round(time.Millisecond))
		// A call that ran into its deadline may carry a partial result (find_references
		// lists the unfinished sources in failed_sources, other handlers may not), so
		// it cannot be trusted even if it looks fine.
		s.Lessf(out.elapsed, parallelCallTimeout, "call %d: %q ran into its %s deadline; its result may be partial", i, label, parallelCallTimeout)
		s.Falsef(out.result.IsError, "call %d: %q reported an error: %s", i, label, text)
		s.NotContainsf(text, "id_reuse", "call %d: %q hit HA's websocket id_reuse rejection", i, label)
		s.NotContainsf(text, "not connected", "call %d: %q reported a lost connection", i, label)

		if label == "find_references" {
			s.assertNoFailedSources(i, text)
		}
	}
}

// assertNoFailedSources decodes a find_references format=json payload and
// requires failed_sources to be empty: a source that could not be fetched under
// load is exactly the partial failure a concurrency regression would cause.
func (s *ParallelToolCallsIntegrationTestSuite) assertNoFailedSources(i int, text string) {
	var payload struct {
		FailedSources []string `json:"failed_sources"`
	}
	if !s.NoErrorf(json.Unmarshal([]byte(text), &payload), "call %d: find_references output is not JSON: %s", i, text) {
		return
	}
	s.Emptyf(payload.FailedSources, "call %d: find_references could not fetch some sources", i)
}
