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
	}{
		{name: "empty supplied proceeds", supplied: nil, mode: arrayModeReplace, getRegistry: knownRegistry, wantIsError: false},
		{name: "all known proceeds", supplied: []string{"known"}, mode: arrayModeReplace, getRegistry: knownRegistry, wantIsError: false},
		{
			name:     "registry fetch error degrades to unchecked write",
			supplied: []string{"anything"}, mode: arrayModeReplace,
			getRegistry: func(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
				return nil, errors.New("boom")
			},
			wantIsError: false,
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
			name: "unknown label rejected", supplied: []string{"bogus"}, mode: arrayModeReplace, getRegistry: knownRegistry,
			wantIsError: true, wantContains: "unknown label(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &UniversalMockClient{GetLabelRegistryFn: tt.getRegistry}
			result := labelWriteGuardError(context.Background(), client, tt.supplied, tt.mode)

			if tt.wantIsError {
				if result == nil || !result.IsError {
					t.Fatalf("expected an error result, got: %v", result)
				}
				if tt.wantContains != "" && !strings.Contains(result.Content[0].Text, tt.wantContains) {
					t.Errorf("expected result to contain %q, got: %s", tt.wantContains, result.Content[0].Text)
				}
				return
			}
			if result != nil {
				t.Errorf("expected nil (proceed), got: %v", result)
			}
		})
	}
}
