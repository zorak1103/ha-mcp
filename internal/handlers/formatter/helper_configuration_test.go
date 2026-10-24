package formatter

import (
	"strings"
	"testing"
)

func TestAppendConfigurationSection_SortedAndFormatted(t *testing.T) {
	t.Parallel()

	detail := map[string]any{
		DetailKeyConfiguration: map[string]any{
			"sampling_size": 20.0,
			"max_age":       map[string]any{"hours": 0.0, "minutes": 10.0, "seconds": 0.0},
			"entity_id":     "sensor.src",
		},
	}

	got := appendConfigurationSection("X", detail)
	want := "X\n\nConfiguration:\nEntity id: sensor.src\nMax age: 00:10:00\nSampling size: 20"
	if got != want {
		t.Errorf("appendConfigurationSection() = %q, want %q", got, want)
	}
}

func TestAppendConfigurationSection_NoBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		detail map[string]any
	}{
		{"no key", map[string]any{"state": "on"}},
		{"empty map", map[string]any{DetailKeyConfiguration: map[string]any{}}},
		{"non-map value", map[string]any{DetailKeyConfiguration: "oops"}},
		{"nil detail", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := appendConfigurationSection("X", tt.detail); got != "X" {
				t.Errorf("appendConfigurationSection() = %q, want %q", got, "X")
			}
		})
	}
}

func TestAppendConfigurationSection_SanitizesAndTruncates(t *testing.T) {
	t.Parallel()

	detail := map[string]any{
		DetailKeyConfiguration: map[string]any{
			"a_note": "line1\nline2",
			"b_long": strings.Repeat("x", 500),
		},
	}

	got := appendConfigurationSection("X", detail)

	lines := strings.Split(got, "\n")
	// "X", "", "Configuration:", plus exactly one line per key.
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5:\n%s", len(lines), got)
	}
	if lines[3] != "A note: line1 line2" {
		t.Errorf("sanitized line = %q, want %q", lines[3], "A note: line1 line2")
	}
	if !strings.HasPrefix(lines[4], "B long: xxx") || !strings.HasSuffix(lines[4], "...") {
		t.Errorf("long line not truncated with ellipsis: %q", lines[4])
	}
	if len([]rune(lines[4])) >= 500 {
		t.Errorf("long line not truncated, %d runes", len([]rune(lines[4])))
	}
}

func TestAppendConfigurationSection_SkipsNilAndEmpty(t *testing.T) {
	t.Parallel()

	detail := map[string]any{
		DetailKeyConfiguration: map[string]any{"a": nil, "b": "", "c": false},
	}

	got := appendConfigurationSection("X", detail)
	want := "X\n\nConfiguration:\nC: false"
	if got != want {
		t.Errorf("appendConfigurationSection() = %q, want %q", got, want)
	}
}

func TestAppendConfigurationSection_OnlySkippedKeys(t *testing.T) {
	t.Parallel()

	detail := map[string]any{
		DetailKeyConfiguration: map[string]any{"a": nil, "b": ""},
	}

	if got := appendConfigurationSection("X", detail); got != "X" {
		t.Errorf("appendConfigurationSection() = %q, want %q (no empty header)", got, "X")
	}
}
