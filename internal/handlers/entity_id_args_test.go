package handlers

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestParseEntityIDOrIDs(t *testing.T) {
	t.Parallel()

	ids101 := make([]any, 101)
	for i := range ids101 {
		ids101[i] = fmt.Sprintf("light.l%d", i)
	}

	tests := []struct {
		name    string
		args    map[string]any
		want    []string
		wantErr string
	}{
		{name: "neither", args: map[string]any{}, want: nil},
		{name: "single", args: map[string]any{"entity_id": "a"}, want: []string{"a"}},
		{name: "empty string entity_id is absent", args: map[string]any{"entity_id": ""}, want: nil},
		{name: "nil entity_id is absent", args: map[string]any{"entity_id": nil}, want: nil},
		{name: "array", args: map[string]any{"entity_ids": []any{"a", "b"}}, want: []string{"a", "b"}},
		{name: "dedup keeps first occurrence", args: map[string]any{"entity_ids": []any{"b", "a", "b"}}, want: []string{"b", "a"}},
		{
			name:    "both supplied",
			args:    map[string]any{"entity_id": "a", "entity_ids": []any{"b"}},
			wantErr: "Cannot specify both entity_id and entity_ids. Use one or the other.",
		},
		{
			name: "empty entity_id placeholder plus array",
			args: map[string]any{"entity_id": "", "entity_ids": []any{"a"}},
			want: []string{"a"},
		},
		{name: "empty array is absent", args: map[string]any{"entity_ids": []any{}}, want: nil},
		{name: "nil entity_ids is absent", args: map[string]any{"entity_ids": nil}, want: nil},
		{name: "non-string entity_id", args: map[string]any{"entity_id": 5.0}, wantErr: "expected a string, got float64"},
		{name: "entity_ids not an array", args: map[string]any{"entity_ids": "a"}, wantErr: "entity_ids must be an array"},
		{
			name:    "non-string element",
			args:    map[string]any{"entity_ids": []any{"a", 3.0}},
			wantErr: "entity_ids[1] must be a non-empty string, got float64",
		},
		{
			name:    "empty element",
			args:    map[string]any{"entity_ids": []any{"a", ""}},
			wantErr: "entity_ids[1] must be a non-empty string",
		},
		{
			name:    "too many ids",
			args:    map[string]any{"entity_ids": ids101},
			wantErr: "entity_ids has 101 elements; at most 100 are allowed per call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseEntityIDOrIDs(tt.args)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (result %v)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseEntityIDOrIDs_AtLimitAccepted(t *testing.T) {
	t.Parallel()

	ids := make([]any, maxEntityIDsPerCall)
	for i := range ids {
		ids[i] = fmt.Sprintf("light.l%d", i)
	}
	got, err := parseEntityIDOrIDs(map[string]any{"entity_ids": ids})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != maxEntityIDsPerCall {
		t.Errorf("got %d ids, want %d", len(got), maxEntityIDsPerCall)
	}
}

func TestParseStrictEntityIDList_LimitCountsBeforeDedup(t *testing.T) {
	t.Parallel()

	raw := make([]any, maxEntityIDsPerCall+1)
	for i := range raw {
		raw[i] = "light.same"
	}
	if _, err := parseStrictEntityIDList("entity_ids", raw); err == nil {
		t.Error("expected an error: the element cap applies to the supplied array, not the deduplicated one")
	}
}

func TestIsAbsentArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    any
		want bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"empty array", []any{}, true},
		{"zero number", 0.0, false},
		{"false", false, false},
		{"non-empty string", "x", false},
		{"non-empty array", []any{1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isAbsentArg(tt.v); got != tt.want {
				t.Errorf("isAbsentArg(%#v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}
