package mcp

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

func TestInvalidArguments(t *testing.T) {
	t.Parallel()

	schema := JSONSchema{Properties: map[string]JSONSchema{
		"name":   {Type: "string"},
		"count":  {Type: "integer"},
		"ratio":  {Type: "number"},
		"flag":   {Type: "boolean"},
		"items":  {Type: "array"},
		"config": {Type: "object"},
		"action": {Type: "string", Enum: []string{"create", "delete"}},
		"free":   {},
	}}

	tests := []struct {
		name string
		args map[string]any
		want []string
	}{
		{"nil args", nil, nil},
		{"empty args", map[string]any{}, nil},
		{"every type satisfied", map[string]any{
			"name": "x", "count": float64(3), "ratio": 1.5, "flag": true,
			"items": []any{"a"}, "config": map[string]any{"k": "v"}, "action": "create",
		}, nil},

		// null and the empty string are "not supplied" for the handlers (isSkippable).
		{"null is skipped for every type", map[string]any{
			"name": nil, "count": nil, "ratio": nil, "flag": nil,
			"items": nil, "config": nil, "action": nil,
		}, nil},
		{"empty string is skipped, including for an enum", map[string]any{
			"name": "", "count": "", "action": "",
		}, nil},
		{"empty array is skipped for a scalar property", map[string]any{
			"name": []any{}, "count": []any{}, "flag": []any{}, "action": []any{}, "config": []any{},
		}, nil},
		{"zero and false are real values", map[string]any{"count": float64(0), "flag": false}, nil},

		// Undeclared names are UnknownArguments' job; a property without a type cannot be checked.
		{"undeclared argument is ignored", map[string]any{"bogus": []any{}}, nil},
		{"property without type accepts anything", map[string]any{"free": map[string]any{}}, nil},

		// Values the handlers' own readers coerce stay accepted.
		{"integer accepts whole number string", map[string]any{"count": "42"}, nil},
		{"integer accepts whole float string", map[string]any{"count": "5.0"}, nil},
		{"integer accepts int", map[string]any{"count": 7}, nil},
		{"number accepts numeric string", map[string]any{"ratio": "2.5"}, nil},
		{"number accepts int64", map[string]any{"ratio": int64(2)}, nil},
		{"string accepts a number", map[string]any{"name": float64(3000)}, nil},
		{"boolean accepts yes", map[string]any{"flag": "Yes"}, nil},
		{"boolean accepts off", map[string]any{"flag": "off"}, nil},
		{"boolean accepts 0 and 1", map[string]any{"flag": float64(1)}, nil},

		// Values nothing can coerce are rejected.
		{"string rejects boolean", map[string]any{"name": true}, []string{`argument "name" must be a string, got boolean`}},
		{"string rejects array", map[string]any{"name": []any{"a"}}, []string{`argument "name" must be a string, got array`}},
		{"string rejects object", map[string]any{"name": map[string]any{}}, []string{`argument "name" must be a string, got object`}},
		{"integer rejects fraction", map[string]any{"count": 2.5}, []string{`argument "count" must be an integer, got number 2.5`}},
		{"integer rejects fractional string", map[string]any{"count": "2.5"}, []string{`argument "count" must be an integer, got string "2.5"`}},
		{"integer rejects text", map[string]any{"count": "many"}, []string{`argument "count" must be an integer, got string "many"`}},
		{"integer rejects boolean", map[string]any{"count": true}, []string{`argument "count" must be an integer, got boolean`}},
		{"integer rejects array", map[string]any{"count": []any{1}}, []string{`argument "count" must be an integer, got array`}},
		{"number rejects text", map[string]any{"ratio": "abc"}, []string{`argument "ratio" must be a number, got string "abc"`}},
		{"number rejects overflowing string", map[string]any{"ratio": "1e400"}, []string{`argument "ratio" must be a number, got string "1e400"`}},
		{"number rejects padded string", map[string]any{"ratio": " 5"}, []string{`argument "ratio" must be a number, got string " 5"`}},
		{"number rejects NaN string", map[string]any{"ratio": "NaN"}, []string{`argument "ratio" must be a number, got string "NaN"`}},
		{"number rejects Inf string", map[string]any{"ratio": "+Inf"}, []string{`argument "ratio" must be a number, got string "+Inf"`}},
		{"number rejects object", map[string]any{"ratio": map[string]any{}}, []string{`argument "ratio" must be a number, got object`}},
		{"boolean rejects text", map[string]any{"flag": "maybe"}, []string{`argument "flag" must be a boolean, got string "maybe"`}},
		{"boolean rejects 2", map[string]any{"flag": float64(2)}, []string{`argument "flag" must be a boolean, got number 2`}},
		{"boolean rejects array", map[string]any{"flag": []any{true}}, []string{`argument "flag" must be a boolean, got array`}},
		{"array rejects string", map[string]any{"items": "a"}, []string{`argument "items" must be an array, got string "a"`}},
		{"array rejects object", map[string]any{"items": map[string]any{}}, []string{`argument "items" must be an array, got object`}},
		{"object rejects string", map[string]any{"config": "{}"}, []string{`argument "config" must be an object, got string "{}"`}},
		{"object rejects array", map[string]any{"config": []any{1}}, []string{`argument "config" must be an object, got array`}},

		// Enum.
		{"enum rejects an unlisted value", map[string]any{"action": "explode"},
			[]string{`argument "action" must be one of: create, delete; got "explode"`}},
		{"enum is case sensitive", map[string]any{"action": "Create"},
			[]string{`argument "action" must be one of: create, delete; got "Create"`}},
		{"enum reports a wrong type before the values", map[string]any{"action": true},
			[]string{`argument "action" must be a string, got boolean`}},

		{"violations are sorted by argument name", map[string]any{"name": true, "count": "x", "action": "no"},
			[]string{
				`argument "action" must be one of: create, delete; got "no"`,
				`argument "count" must be an integer, got string "x"`,
				`argument "name" must be a string, got boolean`,
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := InvalidArguments(schema, tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InvalidArguments() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestInvalidArguments_EnumOnUntypedProperty(t *testing.T) {
	t.Parallel()

	schema := JSONSchema{Properties: map[string]JSONSchema{"mode": {Enum: []string{"a", "b"}}}}
	if got := InvalidArguments(schema, map[string]any{"mode": "a"}); got != nil {
		t.Errorf("InvalidArguments(valid) = %#v, want nil", got)
	}
	want := []string{`argument "mode" must be one of: a, b; got "c"`}
	if got := InvalidArguments(schema, map[string]any{"mode": "c"}); !reflect.DeepEqual(got, want) {
		t.Errorf("InvalidArguments(invalid) = %#v, want %#v", got, want)
	}
}

func TestInvalidArguments_EnumOnNumber(t *testing.T) {
	t.Parallel()

	schema := JSONSchema{Properties: map[string]JSONSchema{"level": {Type: "string", Enum: []string{"1", "2"}}}}
	if got := InvalidArguments(schema, map[string]any{"level": float64(2)}); got != nil {
		t.Errorf("a number whose decimal form is listed must pass, got %#v", got)
	}
	want := []string{`argument "level" must be one of: 1, 2; got number 3`}
	if got := InvalidArguments(schema, map[string]any{"level": float64(3)}); !reflect.DeepEqual(got, want) {
		t.Errorf("InvalidArguments() = %#v, want %#v", got, want)
	}
}

func TestInvalidArgumentsMessage_BoundsHostileValues(t *testing.T) {
	t.Parallel()

	schema := JSONSchema{Properties: map[string]JSONSchema{
		"count": {Type: "integer"},
		"mode":  {Type: "string", Enum: []string{"a"}},
	}}

	t.Run("long string value is cut", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("é", 10000)
		got := InvalidArguments(schema, map[string]any{"count": long})
		if len(got) != 1 {
			t.Fatalf("got %d violations, want 1", len(got))
		}
		if n := utf8.RuneCountInString(got[0]); n > 200 {
			t.Errorf("message has %d runes, want a bounded message", n)
		}
		if want := strings.Repeat("é", maxArgValueRunes) + "…"; !strings.Contains(got[0], want) {
			t.Errorf("message = %q, want it to contain the %d-rune cut value plus an ellipsis", got[0], maxArgValueRunes)
		}
	})

	t.Run("control characters are escaped", func(t *testing.T) {
		t.Parallel()
		got := InvalidArguments(schema, map[string]any{"mode": "a\nb\x00"})
		if len(got) != 1 || strings.ContainsAny(got[0], "\n\x00") {
			t.Errorf("message must escape control characters, got %q", got)
		}
	})
}

func TestInvalidArgumentsMessage_CapsViolations(t *testing.T) {
	t.Parallel()

	props := make(map[string]JSONSchema, 50)
	args := make(map[string]any, 50)
	for i := range 50 {
		name := fmt.Sprintf("p%02d", i)
		props[name] = JSONSchema{Type: "boolean"}
		args[name] = []any{1}
	}
	got := InvalidArguments(JSONSchema{Properties: props}, args)
	if len(got) != maxListedInvalidArgs+1 {
		t.Fatalf("got %d lines, want %d violations plus one summary", len(got), maxListedInvalidArgs)
	}
	if want := "(+40 more invalid arguments)"; got[len(got)-1] != want {
		t.Errorf("last line = %q, want %q", got[len(got)-1], want)
	}
}

func TestInvalidArgumentsError(t *testing.T) {
	t.Parallel()

	got := invalidArgumentsMessage("manage_x", []string{`argument "a" must be a string, got boolean`, `argument "b" must be an array, got object`})
	want := `invalid argument(s) for tool "manage_x": argument "a" must be a string, got boolean; argument "b" must be an array, got object`
	if got != want {
		t.Errorf("invalidArgumentsMessage() = %q, want %q", got, want)
	}
}

func TestInvalidArgumentsResult(t *testing.T) {
	t.Parallel()

	schema := JSONSchema{Properties: map[string]JSONSchema{"limit": {Type: "integer"}}}

	if got := InvalidArgumentsResult("t", schema, map[string]any{"limit": "5"}); got != nil {
		t.Errorf("InvalidArgumentsResult(valid) = %+v, want nil", got)
	}

	got := InvalidArgumentsResult("t", schema, map[string]any{"limit": []any{1}})
	if got == nil || !got.IsError || len(got.Content) != 1 {
		t.Fatalf("InvalidArgumentsResult(invalid) = %+v, want one error content block", got)
	}
	want := `invalid argument(s) for tool "t": argument "limit" must be an integer, got array`
	if got.Content[0].Text != want {
		t.Errorf("text = %q, want %q", got.Content[0].Text, want)
	}
}

func TestServer_HandleToolsCall_InvalidArguments(t *testing.T) {
	t.Parallel()

	schema := JSONSchema{Properties: map[string]JSONSchema{
		"limit":  {Type: "integer"},
		"action": {Type: "string", Enum: []string{"list", "get"}},
	}}
	newServer := func(calls *int) *Server {
		registry := NewRegistry()
		registry.RegisterTool(
			Tool{Name: "t", InputSchema: schema},
			func(_ context.Context, _ homeassistant.Client, _ map[string]any) (*ToolsCallResult, error) {
				*calls++
				return &ToolsCallResult{Content: []ContentBlock{NewTextContent("ok")}}, nil
			},
		)
		return newTestServer(&mockHAClient{}, registry, 8080, logging.New(logging.LevelOff))
	}

	t.Run("wrong type is a tool error and the handler is not called", func(t *testing.T) {
		t.Parallel()
		var calls int
		resp, result := callToolT(t, newServer(&calls), map[string]any{"limit": []any{1}})

		if resp.Error != nil {
			t.Fatalf("want a tool result, got JSON-RPC error: %+v", resp.Error)
		}
		if !result.IsError || len(result.Content) == 0 {
			t.Fatalf("want an error result with content, got %+v", result)
		}
		want := `invalid argument(s) for tool "t": argument "limit" must be an integer, got array`
		if !strings.Contains(result.Content[0].Text, want) {
			t.Errorf("text = %q, want it to contain %q", result.Content[0].Text, want)
		}
		if calls != 0 {
			t.Errorf("handler calls = %d, want 0", calls)
		}
	})

	t.Run("out-of-enum value is a tool error", func(t *testing.T) {
		t.Parallel()
		var calls int
		_, result := callToolT(t, newServer(&calls), map[string]any{"action": "nuke"})

		if !result.IsError || len(result.Content) == 0 ||
			!strings.Contains(result.Content[0].Text, `must be one of: list, get; got "nuke"`) {
			t.Fatalf("want an enum error, got %+v", result)
		}
		if calls != 0 {
			t.Errorf("handler calls = %d, want 0", calls)
		}
	})

	t.Run("coercible and valid values reach the handler", func(t *testing.T) {
		t.Parallel()
		var calls int
		_, result := callToolT(t, newServer(&calls), map[string]any{"limit": "10", "action": "get"})

		if result.IsError {
			t.Fatalf("unexpected error result: %+v", result)
		}
		if calls != 1 {
			t.Errorf("handler calls = %d, want 1", calls)
		}
	})

	t.Run("unknown argument is reported before a type error", func(t *testing.T) {
		t.Parallel()
		var calls int
		_, result := callToolT(t, newServer(&calls), map[string]any{"bogus": 1, "limit": []any{}})

		if !result.IsError || len(result.Content) == 0 ||
			!strings.Contains(result.Content[0].Text, "unknown argument(s)") {
			t.Fatalf("want the unknown-argument error first, got %+v", result)
		}
		if calls != 0 {
			t.Errorf("handler calls = %d, want 0", calls)
		}
	})
}
