package handlers

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestArgReader_Str(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		wantVal any
		wantSet bool
		wantErr bool
	}{
		{name: "absent key skips", args: map[string]any{}, wantSet: false},
		{name: "nil value skips", args: map[string]any{"k": nil}, wantSet: false},
		{name: "empty string skips", args: map[string]any{"k": ""}, wantSet: false},
		{name: "string passes through", args: map[string]any{"k": "hello"}, wantVal: "hello", wantSet: true},
		{name: "float64 coerces to decimal string", args: map[string]any{"k": 3000.0}, wantVal: "3000", wantSet: true},
		{name: "float64 with fraction coerces", args: map[string]any{"k": 1.5}, wantVal: "1.5", wantSet: true},
		{name: "int coerces", args: map[string]any{"k": 42}, wantVal: "42", wantSet: true},
		{name: "bool errors", args: map[string]any{"k": true}, wantErr: true},
		{name: "array errors", args: map[string]any{"k": []any{"x"}}, wantErr: true},
		{name: "map errors", args: map[string]any{"k": map[string]any{}}, wantErr: true},
		{
			name: "oversized string errors instead of being stored verbatim",
			args: map[string]any{"k": strings.Repeat("x", maxScalarStringLen+1)}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := map[string]any{}
			r := newArgReader(config, tt.args)
			r.str("k")
			checkReader(t, r, config, "k", tt.wantVal, tt.wantSet, tt.wantErr)
		})
	}
}

func TestArgReader_StrAs(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	r := newArgReader(config, map[string]any{"heater_entity_id": "switch.x"})
	r.strAs("heater_entity_id", "heater")
	if err := r.err(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config["heater"] != "switch.x" {
		t.Errorf("config[heater] = %v, want switch.x", config["heater"])
	}
	if _, present := config["heater_entity_id"]; present {
		t.Errorf("config should not contain the source arg key")
	}
}

func TestArgReader_Num(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		wantVal any
		wantSet bool
		wantErr bool
	}{
		{name: "absent skips", args: map[string]any{}, wantSet: false},
		{name: "nil skips", args: map[string]any{"k": nil}, wantSet: false},
		{name: "empty string skips", args: map[string]any{"k": ""}, wantSet: false},
		{name: "float64 passes through", args: map[string]any{"k": 3.5}, wantVal: 3.5, wantSet: true},
		{name: "int coerces", args: map[string]any{"k": 5}, wantVal: 5.0, wantSet: true},
		{name: "numeric string coerces", args: map[string]any{"k": "3000"}, wantVal: 3000.0, wantSet: true},
		{name: "decimal string coerces", args: map[string]any{"k": "1.5"}, wantVal: 1.5, wantSet: true},
		{name: "unparseable string errors", args: map[string]any{"k": "abc"}, wantErr: true},
		{name: "bool errors", args: map[string]any{"k": true}, wantErr: true},
		{name: "NaN errors", args: map[string]any{"k": math.NaN()}, wantErr: true},
		{name: "array errors", args: map[string]any{"k": []any{1.0}}, wantErr: true},
		{
			name: "oversized numeric string errors before being parsed",
			args: map[string]any{"k": strings.Repeat("9", maxNumericStringLen+1)}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := map[string]any{}
			r := newArgReader(config, tt.args)
			r.num("k")
			checkReader(t, r, config, "k", tt.wantVal, tt.wantSet, tt.wantErr)
		})
	}
}

func TestArgReader_Integer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		wantVal any
		wantSet bool
		wantErr bool
	}{
		{name: "absent skips", args: map[string]any{}, wantSet: false},
		{name: "nil skips", args: map[string]any{"k": nil}, wantSet: false},
		{name: "empty string skips", args: map[string]any{"k": ""}, wantSet: false},
		{name: "integral float64 coerces", args: map[string]any{"k": 2.0}, wantVal: 2, wantSet: true},
		{name: "fractional float64 errors", args: map[string]any{"k": 2.7}, wantErr: true},
		{name: "int passes through", args: map[string]any{"k": 7}, wantVal: 7, wantSet: true},
		{name: "whole number string coerces", args: map[string]any{"k": "8"}, wantVal: 8, wantSet: true},
		{name: "fractional string errors", args: map[string]any{"k": "8.5"}, wantErr: true},
		{name: "bool errors", args: map[string]any{"k": false}, wantErr: true},
		{
			name: "huge float64 errors instead of silently overflowing to garbage",
			args: map[string]any{"k": 1e20}, wantErr: true,
		},
		{
			name: "huge negative float64 errors instead of silently overflowing to garbage",
			args: map[string]any{"k": -1e20}, wantErr: true,
		},
		{
			name: "huge numeric string errors instead of silently overflowing to garbage",
			args: map[string]any{"k": "100000000000000000000"}, wantErr: true,
		},
		{
			name: "numeric string exceeding int32 via Atoi's fast path still errors",
			args: map[string]any{"k": "5000000000"}, wantErr: true,
		},
		{
			name: "negative numeric string exceeding int32 via Atoi's fast path still errors",
			args: map[string]any{"k": "-5000000000"}, wantErr: true,
		},
		{
			name: "int value exceeding int32 errors, consistent with the float64/string branches",
			args: map[string]any{"k": 5000000000}, wantErr: true,
		},
		{
			name: "int64 value exceeding int32 errors, consistent with the float64/string branches",
			args: map[string]any{"k": int64(5000000000)}, wantErr: true,
		},
		{
			name: "oversized numeric string errors before being parsed",
			args: map[string]any{"k": strings.Repeat("9", maxNumericStringLen+1)}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := map[string]any{}
			r := newArgReader(config, tt.args)
			r.integer("k")
			checkReader(t, r, config, "k", tt.wantVal, tt.wantSet, tt.wantErr)
		})
	}
}

func TestArgReader_Boolean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		wantVal any
		wantSet bool
		wantErr bool
	}{
		{name: "absent skips", args: map[string]any{}, wantSet: false},
		{name: "nil skips", args: map[string]any{"k": nil}, wantSet: false},
		{name: "empty string skips", args: map[string]any{"k": ""}, wantSet: false},
		{name: "bool true passes through", args: map[string]any{"k": true}, wantVal: true, wantSet: true},
		{name: "bool false passes through", args: map[string]any{"k": false}, wantVal: false, wantSet: true},
		{name: "string true coerces", args: map[string]any{"k": "true"}, wantVal: true, wantSet: true},
		{name: "string TRUE coerces case-insensitively", args: map[string]any{"k": "TRUE"}, wantVal: true, wantSet: true},
		{name: "string yes coerces", args: map[string]any{"k": "yes"}, wantVal: true, wantSet: true},
		{name: "string on coerces", args: map[string]any{"k": "on"}, wantVal: true, wantSet: true},
		{name: "string no coerces", args: map[string]any{"k": "no"}, wantVal: false, wantSet: true},
		{name: "string off coerces", args: map[string]any{"k": "off"}, wantVal: false, wantSet: true},
		{name: "number 0 coerces", args: map[string]any{"k": 0.0}, wantVal: false, wantSet: true},
		{name: "number 1 coerces", args: map[string]any{"k": 1.0}, wantVal: true, wantSet: true},
		{name: "number 2 errors", args: map[string]any{"k": 2.0}, wantErr: true},
		{name: "unrecognized string errors", args: map[string]any{"k": "maybe"}, wantErr: true},
		{name: "array errors", args: map[string]any{"k": []any{}}, wantErr: true},
		{
			name: "oversized string errors before being parsed",
			args: map[string]any{"k": strings.Repeat("x", maxNumericStringLen+1)}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := map[string]any{}
			r := newArgReader(config, tt.args)
			r.boolean("k")
			checkReader(t, r, config, "k", tt.wantVal, tt.wantSet, tt.wantErr)
		})
	}
}

func TestArgReader_StrSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		wantVal any
		wantSet bool
		wantErr bool
	}{
		{name: "absent skips", args: map[string]any{}, wantSet: false},
		{name: "nil skips", args: map[string]any{"k": nil}, wantSet: false},
		{
			name: "array of strings passes through", args: map[string]any{"k": []any{"a", "b"}},
			wantVal: []string{"a", "b"}, wantSet: true,
		},
		{
			name: "numeric elements coerce", args: map[string]any{"k": []any{"a", 2.0}},
			wantVal: []string{"a", "2"}, wantSet: true,
		},
		{name: "non-array errors", args: map[string]any{"k": "x"}, wantErr: true},
		{name: "bool element errors", args: map[string]any{"k": []any{true}}, wantErr: true},
		{name: "map element errors", args: map[string]any{"k": []any{map[string]any{}}}, wantErr: true},
		{
			name: "oversized string element errors instead of being stored verbatim",
			args: map[string]any{"k": []any{strings.Repeat("x", maxScalarStringLen+1)}}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := map[string]any{}
			r := newArgReader(config, tt.args)
			r.strSlice("k")
			if tt.wantErr {
				if r.err() == nil {
					t.Fatal("expected an error, got none")
				}
				return
			}
			if err := r.err(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, present := config["k"]
			if present != tt.wantSet {
				t.Fatalf("config[k] present = %v, want %v", present, tt.wantSet)
			}
			if !tt.wantSet {
				return
			}
			gotSlice, ok := got.([]string)
			if !ok {
				t.Fatalf("config[k] is %T, want []string", got)
			}
			wantSlice, _ := tt.wantVal.([]string)
			if len(gotSlice) != len(wantSlice) {
				t.Fatalf("config[k] = %v, want %v", gotSlice, wantSlice)
			}
			for i := range gotSlice {
				if gotSlice[i] != wantSlice[i] {
					t.Errorf("config[k][%d] = %v, want %v", i, gotSlice[i], wantSlice[i])
				}
			}
		})
	}
}

func TestArgReader_AnySlice(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	r := newArgReader(config, map[string]any{"k": []any{map[string]any{"from": "08:00"}}})
	r.anySlice("k")
	if err := r.err(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := config["k"].([]any); !ok {
		t.Fatalf("config[k] = %v, want []any", config["k"])
	}

	config2 := map[string]any{}
	r2 := newArgReader(config2, map[string]any{"k": "not-an-array"})
	r2.anySlice("k")
	if r2.err() == nil {
		t.Fatal("expected an error for non-array value")
	}
}

func TestArgReader_Raw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		wantVal any
		wantSet bool
		wantErr bool
	}{
		{name: "absent skips", args: map[string]any{}, wantSet: false},
		{name: "nil skips", args: map[string]any{"k": nil}, wantSet: false},
		{name: "empty string skips", args: map[string]any{"k": ""}, wantSet: false},
		{name: "number passes through untouched", args: map[string]any{"k": 90.0}, wantVal: 90.0, wantSet: true},
		{name: "string passes through untouched", args: map[string]any{"k": "00:01:30"}, wantVal: "00:01:30", wantSet: true},
		{
			name: "map passes through untouched", args: map[string]any{"k": map[string]any{"minutes": 1.0}},
			wantVal: map[string]any{"minutes": 1.0}, wantSet: true,
		},
		{name: "bool errors", args: map[string]any{"k": true}, wantErr: true},
		{name: "array errors", args: map[string]any{"k": []any{}}, wantErr: true},
		{name: "oversized map errors", args: map[string]any{"k": oversizedMap()}, wantErr: true},
		{
			name: "map with nested map value errors instead of being carried through unbounded",
			args: map[string]any{"k": map[string]any{"minutes": map[string]any{"nested": 1.0}}}, wantErr: true,
		},
		{
			name: "map with nested array value errors instead of being carried through unbounded",
			args: map[string]any{"k": map[string]any{"minutes": []any{1.0, 2.0}}}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := map[string]any{}
			r := newArgReader(config, tt.args)
			r.raw("k")
			if tt.wantErr {
				if r.err() == nil {
					t.Fatal("expected an error, got none")
				}
				return
			}
			if err := r.err(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, present := config["k"]
			if present != tt.wantSet {
				t.Fatalf("config[k] present = %v, want %v", present, tt.wantSet)
			}
			if tt.wantSet && !reflect.DeepEqual(got, tt.wantVal) {
				t.Errorf("config[k] = %v, want %v", got, tt.wantVal)
			}
		})
	}
}

func TestArgReader_AccumulatesMultipleErrors(t *testing.T) {
	t.Parallel()

	config := map[string]any{}
	r := newArgReader(config, map[string]any{"a": true, "b": []any{}})
	r.str("a")
	r.num("b")
	err := r.err()
	if err == nil {
		t.Fatal("expected a joined error")
	}
	if !containsAll(err.Error(), []string{`"a"`, `"b"`}) {
		t.Errorf("joined error %q should mention both failing keys", err.Error())
	}
}

// oversizedMap returns a map with one more entry than maxArrayElements
// permits, for exercising argReader.raw's map size guard.
func oversizedMap() map[string]any {
	m := make(map[string]any, maxArrayElements+1)
	for i := range maxArrayElements + 1 {
		m[strconv.Itoa(i)] = i
	}
	return m
}

func checkReader(t *testing.T, r *argReader, config map[string]any, key string, wantVal any, wantSet, wantErr bool) {
	t.Helper()

	if wantErr {
		if r.err() == nil {
			t.Fatal("expected an error, got none")
		}
		return
	}
	if err := r.err(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, present := config[key]
	if present != wantSet {
		t.Fatalf("config[%s] present = %v, want %v (value: %v)", key, present, wantSet, got)
	}
	if wantSet && got != wantVal {
		t.Errorf("config[%s] = %v (%T), want %v (%T)", key, got, got, wantVal, wantVal)
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// TestArgTypeError_ContainersAreSummarizedNotFullyRendered guards against a
// caller sending an oversized array/map to a scalar-typed field: the error
// path must describe the container by kind and size, not render every
// element via fmt.Sprintf("%v", ...) - doing so would defeat
// maxErrorValueLen's own stated purpose (bounding the cost of reporting a
// bad value) by paying the full render cost before truncating the result.
func TestArgTypeError_ContainersAreSummarizedNotFullyRendered(t *testing.T) {
	t.Parallel()

	hugeArray := make([]any, 200000)
	for i := range hugeArray {
		hugeArray[i] = i
	}

	config := map[string]any{}
	r := newArgReader(config, map[string]any{"k": hugeArray})
	r.str("k")

	err := r.err()
	if err == nil {
		t.Fatal("expected an error for an array sent to a string field")
	}
	msg := err.Error()
	if len(msg) > 500 {
		t.Fatalf("error message is %d bytes, want a bounded summary, not a rendering of every element: %.100s...", len(msg), msg)
	}
	if !strings.Contains(msg, "200000") {
		t.Errorf("error message %q should describe the container's size", msg)
	}
}
