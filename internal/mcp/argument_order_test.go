//nolint:errcheck // Test file - deferred body.Close() is acceptable
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/jsonorder"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

// callToolWithRawParams drives tools/call through the HTTP path with the given
// raw params JSON (so the wire key order is under the test's control) and
// returns the argument order the handler observed.
func callToolWithRawParams(t *testing.T, rawParams string, logger *logging.Logger) (*jsonorder.Order, *Response) {
	t.Helper()

	var got *jsonorder.Order
	registry := NewRegistry()
	registry.RegisterTool(
		Tool{Name: "order_tool", InputSchema: JSONSchema{Properties: map[string]JSONSchema{
			"variables":  {Type: "object"},
			"operations": {Type: "array"},
		}}},
		func(ctx context.Context, _ homeassistant.Client, _ map[string]any) (*ToolsCallResult, error) {
			got = ArgumentOrderFromContext(ctx)
			return &ToolsCallResult{Content: []ContentBlock{NewTextContent("ok")}}, nil
		},
	)
	s := newTestServer(&mockHAClient{}, registry, 8080, logger)

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodToolsCall,
		Params:  json.RawMessage(rawParams),
	}
	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()
	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}
	return got, &jsonResp
}

func TestToolsCall_PassesArgumentOrderViaContext(t *testing.T) {
	t.Parallel()

	raw := `{"name":"order_tool","arguments":{` +
		`"variables":{"zeta":1,"alpha":2,"mid":3},` +
		`"operations":[{"op":"add","value":{"z":1,"a":2}}]}}`

	got, resp := callToolWithRawParams(t, raw, logging.New(logging.LevelOff))
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	want := &jsonorder.Order{
		Keys: []string{"variables", "operations"},
		Fields: map[string]*jsonorder.Order{
			"variables": {Keys: []string{"zeta", "alpha", "mid"}},
			"operations": {Items: []*jsonorder.Order{
				{
					Keys: []string{"op", "value"},
					Fields: map[string]*jsonorder.Order{
						"value": {Keys: []string{"z", "a"}},
					},
				},
			}},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("argument order mismatch (-want +got):\n%s", diff)
	}
	if v := got.Field("operations").Item(0).Field("value"); v == nil || v.Keys[0] != "z" {
		t.Errorf("operations[0].value order = %+v, want keys z,a", v)
	}
}

func TestToolsCall_NoArgumentsGivesNilOrder(t *testing.T) {
	t.Parallel()

	got, resp := callToolWithRawParams(t, `{"name":"order_tool"}`, logging.New(logging.LevelOff))
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if got != nil {
		t.Errorf("ArgumentOrderFromContext() = %+v, want nil", got)
	}
}

func TestCaptureArgumentOrder_EmptyAndInvalidGiveNil(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))
	for name, raw := range map[string][]byte{
		"nil params":     nil,
		"empty params":   {},
		"invalid params": []byte(`{"arguments":{"a":`),
	} {
		if got := s.captureArgumentOrder(raw); got != nil {
			t.Errorf("%s: captureArgumentOrder() = %+v, want nil", name, got)
		}
	}
}

func TestArgumentOrderFromContext_PlainContextIsNil(t *testing.T) {
	t.Parallel()

	if got := ArgumentOrderFromContext(context.Background()); got != nil {
		t.Errorf("ArgumentOrderFromContext(plain) = %+v, want nil", got)
	}
}

func TestWithArgumentOrder_RoundTrip(t *testing.T) {
	t.Parallel()

	o := &jsonorder.Order{Keys: []string{"b", "a"}}
	ctx := WithArgumentOrder(context.Background(), o)
	if got := ArgumentOrderFromContext(ctx); got != o {
		t.Errorf("ArgumentOrderFromContext() = %p, want %p", got, o)
	}
}
