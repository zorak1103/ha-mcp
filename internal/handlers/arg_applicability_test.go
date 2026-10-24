package handlers

import (
	"testing"
)

func TestRejectInapplicableArgs(t *testing.T) {
	t.Parallel()

	onlyFor := map[string][]string{
		"max_time_delta_seconds": {"correlation"},
		"limit":                  {"entries"},
	}

	tests := []struct {
		name     string
		args     map[string]any
		active   string
		onlyFor  map[string][]string
		hint     string
		wantText string // empty means nil result expected
	}{
		{
			name:    "all applicable",
			args:    map[string]any{"limit": 5.0},
			active:  "entries",
			onlyFor: onlyFor,
		},
		{
			name:     "one inapplicable",
			args:     map[string]any{"max_time_delta_seconds": 30.0},
			active:   "entries",
			onlyFor:  onlyFor,
			wantText: "argument(s) not applicable to mode=entries: max_time_delta_seconds (only for mode=correlation).",
		},
		{
			name:     "two sorted by name",
			args:     map[string]any{"max_time_delta_seconds": 30.0, "limit": 5.0},
			active:   "x",
			onlyFor:  onlyFor,
			wantText: "argument(s) not applicable to mode=x: limit (only for mode=entries), max_time_delta_seconds (only for mode=correlation).",
		},
		{
			name:    "placeholders are absent",
			args:    map[string]any{"max_time_delta_seconds": nil, "limit": ""},
			active:  "x",
			onlyFor: onlyFor,
		},
		{
			name:     "zero is a value",
			args:     map[string]any{"max_time_delta_seconds": 0.0},
			active:   "entries",
			onlyFor:  onlyFor,
			wantText: "argument(s) not applicable to mode=entries: max_time_delta_seconds (only for mode=correlation).",
		},
		{
			name:     "false is a value",
			args:     map[string]any{"limit": false},
			active:   "correlation",
			onlyFor:  onlyFor,
			wantText: "argument(s) not applicable to mode=correlation: limit (only for mode=entries).",
		},
		{
			name:    "unlisted arg ignored",
			args:    map[string]any{"format": "json", "verbose": true},
			active:  "entries",
			onlyFor: onlyFor,
		},
		{
			name:     "modes listed in declared order",
			args:     map[string]any{"cursor": "abc"},
			active:   "correlation",
			onlyFor:  map[string][]string{"cursor": {"entries", "history", "statistics"}},
			wantText: "argument(s) not applicable to mode=correlation: cursor (only for mode=entries|history|statistics).",
		},
		{
			name:     "hint appended",
			args:     map[string]any{"limit": 5.0},
			active:   "correlation",
			onlyFor:  onlyFor,
			hint:     "Use get_state.",
			wantText: "argument(s) not applicable to mode=correlation: limit (only for mode=entries). Use get_state.",
		},
		{
			name:    "nil onlyFor",
			args:    map[string]any{"limit": 5.0},
			active:  "entries",
			onlyFor: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := rejectInapplicableArgs(tt.args, "mode", tt.active, tt.onlyFor, tt.hint)
			if tt.wantText == "" {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected error %q, got nil", tt.wantText)
			}
			if !got.IsError {
				t.Error("expected IsError")
			}
			if text := got.Content[0].Text; text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
		})
	}
}
