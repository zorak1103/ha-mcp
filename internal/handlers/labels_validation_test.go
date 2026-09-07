package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestUnknownLabelIDs(t *testing.T) {
	t.Parallel()

	registry := []homeassistant.LabelRegistryEntry{
		{LabelID: "kitchen_lights", Name: "Kitchen Lights"},
		{LabelID: "important", Name: "Important"},
	}

	tests := []struct {
		name     string
		labels   []homeassistant.LabelRegistryEntry
		supplied []string
		want     []string
	}{
		{name: "all known", labels: registry, supplied: []string{"important", "kitchen_lights"}, want: nil},
		{
			name:     "some unknown preserves supplied order",
			labels:   registry,
			supplied: []string{"bogus", "kitchen_lights", "also_bogus"},
			want:     []string{"bogus", "also_bogus"},
		},
		{name: "empty supplied", labels: registry, supplied: nil, want: nil},
		{name: "empty registry", labels: nil, supplied: []string{"important"}, want: []string{"important"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := unknownLabelIDs(tt.labels, tt.supplied)
			if len(got) != len(tt.want) {
				t.Fatalf("unknownLabelIDs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("unknownLabelIDs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindLabelIDByName(t *testing.T) {
	t.Parallel()

	registry := []homeassistant.LabelRegistryEntry{
		{LabelID: "kitchen_lights", Name: "Kitchen Lights"},
	}

	tests := []struct {
		name      string
		input     string
		wantID    string
		wantFound bool
	}{
		{name: "exact case-insensitive name match", input: "kitchen lights", wantID: "kitchen_lights", wantFound: true},
		{name: "exact case match", input: "Kitchen Lights", wantID: "kitchen_lights", wantFound: true},
		{name: "substring of name does not match", input: "Kitchen", wantFound: false},
		{name: "label_id input does not match against name", input: "kitchen_lights", wantFound: false},
		{name: "no match", input: "nope", wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotID, gotFound := findLabelIDByName(registry, tt.input)
			if gotFound != tt.wantFound || (gotFound && gotID != tt.wantID) {
				t.Errorf("findLabelIDByName(%q) = (%q, %v), want (%q, %v)", tt.input, gotID, gotFound, tt.wantID, tt.wantFound)
			}
		})
	}
}

func TestUnknownLabelsMessage(t *testing.T) {
	t.Parallel()

	registry := []homeassistant.LabelRegistryEntry{
		{LabelID: "kitchen_lights", Name: "Kitchen Lights"},
	}

	t.Run("suggests label_id when unknown value matches a name", func(t *testing.T) {
		t.Parallel()
		msg := unknownLabelsMessage(registry, []string{"Kitchen Lights"})
		if !strings.Contains(msg, "did you mean") || !strings.Contains(msg, "kitchen_lights") {
			t.Errorf("expected a did-you-mean hint naming kitchen_lights, got: %s", msg)
		}
	})

	t.Run("no hint when nothing matches", func(t *testing.T) {
		t.Parallel()
		msg := unknownLabelsMessage(registry, []string{"totally_bogus"})
		if strings.Contains(msg, "did you mean") {
			t.Errorf("did not expect a did-you-mean hint, got: %s", msg)
		}
	})

	t.Run("points at manage_label", func(t *testing.T) {
		t.Parallel()
		msg := unknownLabelsMessage(registry, []string{"totally_bogus"})
		if !strings.Contains(msg, "manage_label") {
			t.Errorf("expected message to mention manage_label, got: %s", msg)
		}
	})
}

func TestLabelWriteGuardError(t *testing.T) {
	t.Parallel()

	knownRegistry := func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
		return []homeassistant.LabelRegistryEntry{{LabelID: "known", Name: "Known"}}, nil
	}

	tests := []struct {
		name         string
		supplied     []string
		mode         string
		getRegistry  func(context.Context) ([]homeassistant.LabelRegistryEntry, error)
		wantIsError  bool
		wantContains string
		wantWarning  bool
	}{
		{name: "empty supplied proceeds", supplied: nil, mode: arrayModeReplace, getRegistry: knownRegistry, wantIsError: false},
		{name: "all known proceeds", supplied: []string{"known"}, mode: arrayModeReplace, getRegistry: knownRegistry, wantIsError: false},
		{
			name:     "registry fetch error degrades to unchecked write with a warning",
			supplied: []string{"anything"}, mode: arrayModeReplace,
			getRegistry: func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
				return nil, errors.New("boom")
			},
			wantIsError: false,
			wantWarning: true,
		},
		{
			name:     "remove mode is never validated even with unknown labels",
			supplied: []string{"unknown"}, mode: arrayModeRemove,
			getRegistry: func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
				t.Fatal("remove mode must not consult the label registry")
				return nil, nil
			},
			wantIsError: false,
		},
		{
			name:     "unknown label rejected after a retry against a fresh registry still fails",
			supplied: []string{"bogus"}, mode: arrayModeReplace, getRegistry: knownRegistry,
			wantIsError: true, wantContains: "unknown label(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &UniversalMockClient{GetLabelRegistryFn: tt.getRegistry}
			result := labelWriteGuardError(context.Background(), client, tt.supplied, tt.mode)

			if tt.wantIsError {
				if result.Refusal == nil || !result.Refusal.IsError {
					t.Fatalf("expected a refusal, got: %+v", result)
				}
				if tt.wantContains != "" && !strings.Contains(result.Refusal.Content[0].Text, tt.wantContains) {
					t.Errorf("expected refusal to contain %q, got: %s", tt.wantContains, result.Refusal.Content[0].Text)
				}
				return
			}
			if result.Refusal != nil {
				t.Errorf("expected no refusal, got: %v", result.Refusal)
			}
			if tt.wantWarning && result.Warning == "" {
				t.Error("expected a non-empty warning")
			}
			if !tt.wantWarning && result.Warning != "" {
				t.Errorf("expected no warning, got: %q", result.Warning)
			}
		})
	}
}

// noCacheClient wraps a homeassistant.Client through the interface's own (static) method set,
// so promoted methods are exactly homeassistant.Client's - even if the embedded concrete value
// also implements InvalidateLabelRegistryCache(), that extra method is not promoted and a type
// assertion for it fails. This simulates an uncached HybridClient, which has no such method,
// without needing a second hand-written implementation of the full Client interface.
type noCacheClient struct {
	homeassistant.Client
}

func TestLabelWriteGuardError_CacheRetry(t *testing.T) {
	t.Parallel()

	t.Run("stale cache is refreshed and retried once, then succeeds", func(t *testing.T) {
		t.Parallel()
		calls := 0
		invalidated := false
		client := &UniversalMockClient{
			GetLabelRegistryFn: func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
				calls++
				if calls == 1 {
					return []homeassistant.LabelRegistryEntry{{LabelID: "other"}}, nil
				}
				return []homeassistant.LabelRegistryEntry{{LabelID: "other"}, {LabelID: "fresh"}}, nil
			},
			InvalidateLabelRegistryCacheFn: func() { invalidated = true },
		}

		result := labelWriteGuardError(context.Background(), client, []string{"fresh"}, arrayModeReplace)

		if result.Refusal != nil {
			t.Fatalf("expected the retry to succeed, got refusal: %v", result.Refusal)
		}
		if !invalidated {
			t.Error("expected the cache to be invalidated before the retry")
		}
		if calls != 2 {
			t.Errorf("expected exactly 2 registry fetches (initial + retry), got %d", calls)
		}
	})

	t.Run("still unknown after retry refuses", func(t *testing.T) {
		t.Parallel()
		calls := 0
		client := &UniversalMockClient{
			GetLabelRegistryFn: func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
				calls++
				return []homeassistant.LabelRegistryEntry{{LabelID: "other"}}, nil
			},
		}

		result := labelWriteGuardError(context.Background(), client, []string{"bogus"}, arrayModeReplace)

		if result.Refusal == nil {
			t.Fatal("expected a refusal")
		}
		if calls != 2 {
			t.Errorf("expected exactly 2 registry fetches (initial + retry), got %d", calls)
		}
	})

	t.Run("retry refetch failure degrades to unchecked write with a warning, not a refusal", func(t *testing.T) {
		t.Parallel()
		calls := 0
		client := &UniversalMockClient{
			GetLabelRegistryFn: func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
				calls++
				if calls == 1 {
					return []homeassistant.LabelRegistryEntry{{LabelID: "other"}}, nil
				}
				return nil, errors.New("still unavailable")
			},
		}

		result := labelWriteGuardError(context.Background(), client, []string{"bogus"}, arrayModeReplace)

		if result.Refusal != nil {
			t.Errorf("expected no refusal, got: %v", result.Refusal)
		}
		if result.Warning == "" {
			t.Error("expected a non-empty warning")
		}
	})

	t.Run("client without cache-invalidation support refuses on first pass, no retry", func(t *testing.T) {
		t.Parallel()
		calls := 0
		client := noCacheClient{Client: &UniversalMockClient{
			GetLabelRegistryFn: func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
				calls++
				return []homeassistant.LabelRegistryEntry{{LabelID: "other"}}, nil
			},
		}}

		result := labelWriteGuardError(context.Background(), client, []string{"bogus"}, arrayModeReplace)

		if result.Refusal == nil {
			t.Fatal("expected a refusal")
		}
		if calls != 1 {
			t.Errorf("expected exactly 1 registry fetch (no retry possible), got %d", calls)
		}
	})
}

func TestParseLabelsArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		wantLabels   []string
		wantHas      bool
		wantRefusal  bool
		wantContains string
	}{
		{name: "absent key", args: map[string]any{}, wantHas: false},
		{name: "nil value", args: map[string]any{"labels": nil}, wantHas: false},
		{name: "empty array", args: map[string]any{"labels": []any{}}, wantLabels: []string{}, wantHas: true},
		{
			name: "valid string array", args: map[string]any{"labels": []any{"a", "b"}},
			wantLabels: []string{"a", "b"}, wantHas: true,
		},
		{
			name:        "bare string is refused, not silently ignored",
			args:        map[string]any{"labels": "kitchen"},
			wantRefusal: true, wantContains: "array of strings",
		},
		{
			name:        "non-string element is refused, not silently dropped",
			args:        map[string]any{"labels": []any{"a", 123, "b"}},
			wantRefusal: true, wantContains: "labels[1]",
		},
		{
			name:        "object value is refused",
			args:        map[string]any{"labels": map[string]any{"id": "x"}},
			wantRefusal: true, wantContains: "array of strings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			labels, hasLabels, refusal := parseLabelsArg(tt.args)

			if tt.wantRefusal {
				if refusal == nil || !refusal.IsError {
					t.Fatalf("expected a refusal, got labels=%v hasLabels=%v refusal=%v", labels, hasLabels, refusal)
				}
				if tt.wantContains != "" && !strings.Contains(refusal.Content[0].Text, tt.wantContains) {
					t.Errorf("expected refusal to contain %q, got: %s", tt.wantContains, refusal.Content[0].Text)
				}
				return
			}
			if refusal != nil {
				t.Fatalf("expected no refusal, got: %v", refusal)
			}
			if hasLabels != tt.wantHas {
				t.Errorf("hasLabels = %v, want %v", hasLabels, tt.wantHas)
			}
			if tt.wantHas && len(labels) != len(tt.wantLabels) {
				t.Errorf("labels = %v, want %v", labels, tt.wantLabels)
			}
			for i := range tt.wantLabels {
				if i < len(labels) && labels[i] != tt.wantLabels[i] {
					t.Errorf("labels[%d] = %q, want %q", i, labels[i], tt.wantLabels[i])
				}
			}
		})
	}
}

func TestAppendResultWarning(t *testing.T) {
	t.Parallel()

	t.Run("appends a new content block to a successful result", func(t *testing.T) {
		t.Parallel()
		res := successResult("done")
		out := appendResultWarning(res, "be careful")
		if len(out.Content) != 2 {
			t.Fatalf("expected 2 content blocks, got %d", len(out.Content))
		}
		if out.Content[0].Text != "done" {
			t.Errorf("expected first block unchanged, got %q", out.Content[0].Text)
		}
		if out.Content[1].Text != "WARNING: be careful" {
			t.Errorf("expected warning block, got %q", out.Content[1].Text)
		}
	})

	t.Run("json content block is left untouched, warning appended separately", func(t *testing.T) {
		t.Parallel()
		res := successResult(`{"area_id":"kitchen"}`)
		out := appendResultWarning(res, "be careful")
		if out.Content[0].Text != `{"area_id":"kitchen"}` {
			t.Errorf("expected json block unmodified, got %q", out.Content[0].Text)
		}
		if len(out.Content) != 2 {
			t.Fatalf("expected 2 content blocks, got %d", len(out.Content))
		}
	})

	t.Run("empty warning is a no-op", func(t *testing.T) {
		t.Parallel()
		res := successResult("done")
		out := appendResultWarning(res, "")
		if len(out.Content) != 1 {
			t.Fatalf("expected 1 content block, got %d", len(out.Content))
		}
	})

	t.Run("error result is never annotated with a warning", func(t *testing.T) {
		t.Parallel()
		res := errorResult("boom")
		out := appendResultWarning(res, "be careful")
		if len(out.Content) != 1 {
			t.Fatalf("expected error result untouched, got %d content blocks", len(out.Content))
		}
	})
}
