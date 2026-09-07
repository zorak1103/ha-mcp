package handlers

import (
	"reflect"
	"strings"
	"testing"
)

func TestApplyArrayMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		current  []string
		provided []string
		mode     string
		want     []string
	}{
		// Add mode
		{name: "add to nil current", current: nil, provided: []string{"a"}, mode: arrayModeAdd, want: []string{"a"}},
		{name: "add to existing", current: []string{"a"}, provided: []string{"b"}, mode: arrayModeAdd, want: []string{"a", "b"}},
		{name: "add deduplicates", current: []string{"a", "b"}, provided: []string{"b", "c"}, mode: arrayModeAdd, want: []string{"a", "b", "c"}},
		{name: "add empty provided is no-op", current: []string{"a"}, provided: []string{}, mode: arrayModeAdd, want: []string{"a"}},
		{name: "add nil provided is no-op", current: []string{"a"}, provided: nil, mode: arrayModeAdd, want: []string{"a"}},
		{name: "add with nil current and nil provided", current: nil, provided: nil, mode: arrayModeAdd, want: nil},
		{name: "add with nil current and empty provided", current: nil, provided: []string{}, mode: arrayModeAdd, want: nil},
		// Remove mode
		{name: "remove existing item", current: []string{"a", "b", "c"}, provided: []string{"b"}, mode: arrayModeRemove, want: []string{"a", "c"}},
		{name: "remove nonexistent item is silent no-op", current: []string{"a", "b"}, provided: []string{"c"}, mode: arrayModeRemove, want: []string{"a", "b"}},
		{name: "remove all items", current: []string{"a"}, provided: []string{"a"}, mode: arrayModeRemove, want: []string{}},
		{name: "remove from nil current", current: nil, provided: []string{"a"}, mode: arrayModeRemove, want: nil},
		{name: "remove with empty provided is no-op", current: []string{"a"}, provided: []string{}, mode: arrayModeRemove, want: []string{"a"}},
		{name: "remove multiple items", current: []string{"a", "b", "c", "d"}, provided: []string{"a", "c"}, mode: arrayModeRemove, want: []string{"b", "d"}},
		// Replace mode
		{name: "replace with values", current: []string{"a", "b"}, provided: []string{"c", "d"}, mode: arrayModeReplace, want: []string{"c", "d"}},
		{name: "replace with empty clears", current: []string{"a", "b"}, provided: []string{}, mode: arrayModeReplace, want: []string{}},
		{name: "replace with nil", current: []string{"a"}, provided: nil, mode: arrayModeReplace, want: nil},
		// Unknown mode defaults to add
		{name: "unknown mode defaults to add", current: []string{"a"}, provided: []string{"b"}, mode: "unknown", want: []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := applyArrayMode(tt.current, tt.provided, tt.mode)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyArrayMode(%v, %v, %q) = %v, want %v", tt.current, tt.provided, tt.mode, got, tt.want)
			}
		})
	}
}

func TestGetStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		key     string
		wantVal []string
		wantOK  bool
	}{
		{
			name:    "key not present",
			args:    map[string]any{},
			key:     "labels",
			wantVal: nil,
			wantOK:  false,
		},
		{
			name:    "key present with nil value",
			args:    map[string]any{"labels": nil},
			key:     "labels",
			wantVal: nil,
			wantOK:  false,
		},
		{
			name:    "key present with empty array",
			args:    map[string]any{"labels": []any{}},
			key:     "labels",
			wantVal: []string{},
			wantOK:  true,
		},
		{
			name:    "key present with values",
			args:    map[string]any{"labels": []any{"a", "b", "c"}},
			key:     "labels",
			wantVal: []string{"a", "b", "c"},
			wantOK:  true,
		},
		{
			name:    "key present with wrong type",
			args:    map[string]any{"labels": "not_a_slice"},
			key:     "labels",
			wantVal: nil,
			wantOK:  false,
		},
		{
			name:    "non-string items filtered out",
			args:    map[string]any{"labels": []any{"a", 42, "b"}},
			key:     "labels",
			wantVal: []string{"a", "b"},
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotVal, gotOK := getStringSlice(tt.args, tt.key)
			if gotOK != tt.wantOK {
				t.Errorf("getStringSlice() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if !reflect.DeepEqual(gotVal, tt.wantVal) {
				t.Errorf("getStringSlice() val = %v, want %v", gotVal, tt.wantVal)
			}
		})
	}
}

func TestGetArrayMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        map[string]any
		key         string
		want        string
		wantErr     bool
		wantErrText string
	}{
		{
			name: "key not present defaults to add",
			args: map[string]any{},
			key:  "label_mode",
			want: arrayModeAdd,
		},
		{
			name: "empty string defaults to add",
			args: map[string]any{"label_mode": ""},
			key:  "label_mode",
			want: arrayModeAdd,
		},
		{
			name: "explicit add",
			args: map[string]any{"label_mode": arrayModeAdd},
			key:  "label_mode",
			want: arrayModeAdd,
		},
		{
			name: "explicit remove",
			args: map[string]any{"label_mode": arrayModeRemove},
			key:  "label_mode",
			want: arrayModeRemove,
		},
		{
			name: "explicit replace",
			args: map[string]any{"label_mode": arrayModeReplace},
			key:  "label_mode",
			want: arrayModeReplace,
		},
		{
			name:        "unrecognized value is rejected, not silently treated as add",
			args:        map[string]any{"label_mode": "Replace"},
			key:         "label_mode",
			wantErr:     true,
			wantErrText: "label_mode",
		},
		{
			name:        "non-string value is rejected",
			args:        map[string]any{"label_mode": 42},
			key:         "label_mode",
			wantErr:     true,
			wantErrText: "label_mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getArrayMode(tt.args, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got mode %q", got)
				}
				if tt.wantErrText != "" && !strings.Contains(err.Error(), tt.wantErrText) {
					t.Errorf("expected error to mention %q, got: %v", tt.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("getArrayMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestArrayModeSchema(t *testing.T) {
	t.Parallel()

	schema := arrayModeSchema("labels")

	if schema.Type != "string" {
		t.Errorf("arrayModeSchema().Type = %q, want %q", schema.Type, "string")
	}
	if len(schema.Enum) != 3 {
		t.Errorf("arrayModeSchema().Enum length = %d, want 3", len(schema.Enum))
	}

	enumSet := make(map[string]bool, len(schema.Enum))
	for _, v := range schema.Enum {
		enumSet[v] = true
	}
	for _, mode := range []string{arrayModeAdd, arrayModeRemove, arrayModeReplace} {
		if !enumSet[mode] {
			t.Errorf("arrayModeSchema() Enum missing %q", mode)
		}
	}

	if !strings.Contains(schema.Description, "labels") {
		t.Errorf("arrayModeSchema(%q) Description should mention field name, got: %q", "labels", schema.Description)
	}
}
