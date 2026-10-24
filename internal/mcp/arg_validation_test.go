package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

func TestUnknownArguments(t *testing.T) {
	t.Parallel()

	formatOnly := JSONSchema{Properties: map[string]JSONSchema{"format": {Type: "string"}}}

	tests := []struct {
		name   string
		schema JSONSchema
		args   map[string]any
		want   []string
	}{
		{"nil args", formatOnly, nil, nil},
		{"all known", formatOnly, map[string]any{"format": "json"}, nil},
		{
			"unknown names are sorted",
			formatOnly,
			map[string]any{"z_y": 1, "a_x": 1, "format": "json"},
			[]string{"a_x", "z_y"},
		},
		{"no properties and empty args", JSONSchema{}, map[string]any{}, nil},
		{"no properties and an argument", JSONSchema{}, map[string]any{"x": nil}, []string{"x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := UnknownArguments(tt.schema, tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnknownArguments() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestUnknownArgumentsMessage(t *testing.T) {
	t.Parallel()

	t.Run("lists unknown and sorted valid arguments", func(t *testing.T) {
		t.Parallel()
		schema := JSONSchema{Properties: map[string]JSONSchema{"z": {}, "x": {}, "y": {}}}
		got := unknownArgumentsMessage("t", []string{"a"}, schema)
		want := `unknown argument(s) "a" for tool "t"; valid arguments: x, y, z`
		if got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("several unknown names", func(t *testing.T) {
		t.Parallel()
		schema := JSONSchema{Properties: map[string]JSONSchema{"x": {}}}
		got := unknownArgumentsMessage("t", []string{"a", "b"}, schema)
		want := `unknown argument(s) "a", "b" for tool "t"; valid arguments: x`
		if got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("property-less schema", func(t *testing.T) {
		t.Parallel()
		got := unknownArgumentsMessage("t", []string{"a"}, JSONSchema{})
		if !strings.HasSuffix(got, "valid arguments: (none)") {
			t.Errorf("message = %q, want suffix %q", got, "valid arguments: (none)")
		}
	})
}

func TestUnknownArgumentsMessage_BoundsHostileNames(t *testing.T) {
	t.Parallel()

	schema := JSONSchema{Properties: map[string]JSONSchema{"format": {}}}

	t.Run("newline is escaped", func(t *testing.T) {
		t.Parallel()
		msg := unknownArgumentsMessage("t", []string{"a\nb"}, schema)
		if strings.Contains(msg, "\n") {
			t.Errorf("message contains a raw newline: %q", msg)
		}
		if !strings.Contains(msg, `"a\nb"`) {
			t.Errorf("message = %q, want the escaped name %q", msg, `"a\nb"`)
		}
	})

	t.Run("long name is truncated", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("é", 10000)
		msg := unknownArgumentsMessage("t", []string{long}, schema)
		if got := utf8.RuneCountInString(msg); got > 200 {
			t.Errorf("message has %d runes, want a bounded message", got)
		}
		want := `"` + strings.Repeat("é", maxArgNameRunes) + `…"`
		if !strings.Contains(msg, want) {
			t.Errorf("message = %q, want it to contain the %d-rune cut name plus an ellipsis", msg, maxArgNameRunes)
		}
	})

	t.Run("name of exactly the limit is not cut", func(t *testing.T) {
		t.Parallel()
		exact := strings.Repeat("a", maxArgNameRunes)
		msg := unknownArgumentsMessage("t", []string{exact}, schema)
		if strings.Contains(msg, "…") {
			t.Errorf("a name of %d runes must not be truncated: %q", maxArgNameRunes, msg)
		}
	})

	t.Run("many names are capped", func(t *testing.T) {
		t.Parallel()
		names := make([]string, 500)
		for i := range names {
			names[i] = fmt.Sprintf("k%03d", i)
		}
		msg := unknownArgumentsMessage("t", names, schema)
		if !strings.Contains(msg, "(+490 more)") {
			t.Errorf("message = %q, want it to contain %q", msg, "(+490 more)")
		}
		if len(msg) >= 2048 {
			t.Errorf("len(message) = %d, want < 2048", len(msg))
		}
		if strings.Contains(msg, `"k010"`) {
			t.Errorf("only %d names may be listed, but k010 appears: %q", maxListedUnknownArgs, msg)
		}
	})
}

// callToolT sends a tools/call request through the server and decodes the response.
func callToolT(t *testing.T, s *Server, args map[string]any) (Response, ToolsCallResult) {
	t.Helper()

	paramsJSON, _ := json.Marshal(ToolsCallParams{Name: "t", Arguments: args})
	reqBodyJSON, _ := json.Marshal(Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodToolsCall,
		Params:  paramsJSON,
	})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()
	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}
	var result ToolsCallResult
	if jsonResp.Error == nil {
		resultJSON, _ := json.Marshal(jsonResp.Result)
		if err := json.Unmarshal(resultJSON, &result); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
	}
	return jsonResp, result
}

func TestServer_HandleToolsCall_UnknownArguments(t *testing.T) {
	t.Parallel()

	newRegistry := func(calls *int, schema JSONSchema) *Registry {
		registry := NewRegistry()
		registry.RegisterTool(
			Tool{Name: "t", InputSchema: schema},
			func(_ context.Context, _ homeassistant.Client, _ map[string]any) (*ToolsCallResult, error) {
				*calls++
				return &ToolsCallResult{Content: []ContentBlock{NewTextContent("ok")}}, nil
			},
		)
		return registry
	}
	formatSchema := JSONSchema{Properties: map[string]JSONSchema{"format": {Type: "string"}}}

	t.Run("unknown argument is an error result and the handler is not called", func(t *testing.T) {
		t.Parallel()
		var calls int
		s := newTestServer(&mockHAClient{}, newRegistry(&calls, formatSchema), 8080, logging.New(logging.LevelOff))

		resp, result := callToolT(t, s, map[string]any{"format": "json", "bogus": 1})

		if resp.Error != nil {
			t.Fatalf("want a tool result, got JSON-RPC error: %+v", resp.Error)
		}
		if !result.IsError {
			t.Error("IsError = false, want true")
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}
		want := `unknown argument(s) "bogus" for tool "t"; valid arguments: format`
		if !strings.Contains(result.Content[0].Text, want) {
			t.Errorf("text = %q, want it to contain %q", result.Content[0].Text, want)
		}
		if calls != 0 {
			t.Errorf("handler calls = %d, want 0", calls)
		}
	})

	t.Run("declared arguments reach the handler", func(t *testing.T) {
		t.Parallel()
		var calls int
		s := newTestServer(&mockHAClient{}, newRegistry(&calls, formatSchema), 8080, logging.New(logging.LevelOff))

		resp, result := callToolT(t, s, map[string]any{"format": "json"})

		if resp.Error != nil || result.IsError {
			t.Fatalf("unexpected failure: error=%+v isError=%v", resp.Error, result.IsError)
		}
		if calls != 1 {
			t.Errorf("handler calls = %d, want 1", calls)
		}
	})

	t.Run("property-less tool without arguments reaches the handler", func(t *testing.T) {
		t.Parallel()
		var calls int
		s := newTestServer(&mockHAClient{}, newRegistry(&calls, JSONSchema{}), 8080, logging.New(logging.LevelOff))

		resp, result := callToolT(t, s, nil)

		if resp.Error != nil || result.IsError {
			t.Fatalf("unexpected failure: error=%+v isError=%v", resp.Error, result.IsError)
		}
		if calls != 1 {
			t.Errorf("handler calls = %d, want 1", calls)
		}
	})

	t.Run("filter block wins over unknown arguments", func(t *testing.T) {
		t.Parallel()
		var calls int
		s := newTestServer(&mockHAClient{}, newRegistry(&calls, formatSchema), 8080, logging.New(logging.LevelOff))
		s.SetToolFilter(NewToolFilterEngine(ToolFilterConfig{Blacklist: []string{"t"}}, false))

		resp, _ := callToolT(t, s, map[string]any{"bogus": 1})

		if resp.Error == nil {
			t.Fatal("want the filter's JSON-RPC error")
		}
		if resp.Error.Code != ToolExecutionErr || !strings.Contains(resp.Error.Message, "action blocked by server filter") {
			t.Errorf("error = %+v, want the filter block error", resp.Error)
		}
		if calls != 0 {
			t.Errorf("handler calls = %d, want 0", calls)
		}
	})
}
