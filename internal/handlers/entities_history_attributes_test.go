package handlers

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func unix(ts int64) time.Time { return time.Unix(ts, 0) }

// attrRow builds a history row whose update time is lu and whose (stale) last_changed is lc.
func attrRow(lu, lc float64, attrs map[string]any) homeassistant.HistoryEntry {
	return homeassistant.HistoryEntry{State: "heat", Attributes: attrs, LastUpdated: lu, LastChanged: lc}
}

func TestAttributeChanges(t *testing.T) {
	const base = 1_700_000_000
	preset := func(v any) map[string]any { return map[string]any{"preset_mode": v} }

	longList := make([]any, 0, 60)
	for i := range 60 {
		longList = append(longList, i)
	}

	tests := []struct {
		name     string
		entries  []homeassistant.HistoryEntry
		attr     string
		wantSeen bool
		want     []attributeChange // At compared by Unix seconds
	}{
		{name: "no entries", attr: "preset_mode"},
		{
			name: "no change keeps only the start row",
			entries: []homeassistant.HistoryEntry{
				attrRow(base, base, preset("eco")),
				attrRow(base+60, base, preset("eco")),
				attrRow(base+120, base, preset("eco")),
			},
			attr:     "preset_mode",
			wantSeen: true,
			want:     []attributeChange{{At: unix(base), Start: true, New: "eco"}},
		},
		{
			name: "single flip uses update time, not the stale last_changed",
			entries: []homeassistant.HistoryEntry{
				attrRow(base, base, preset("eco")),
				attrRow(base+60, base, preset("eco")),
				attrRow(base+120, base, preset("comfort")),
			},
			attr:     "preset_mode",
			wantSeen: true,
			want: []attributeChange{
				{At: unix(base), Start: true, New: "eco"},
				{At: unix(base + 120), Old: "eco", New: "comfort"},
			},
		},
		{
			name: "falls back to last_changed when last_updated is missing",
			entries: []homeassistant.HistoryEntry{
				attrRow(0, base, preset("eco")),
				attrRow(0, base+30, preset("comfort")),
			},
			attr:     "preset_mode",
			wantSeen: true,
			want: []attributeChange{
				{At: unix(base), Start: true, New: "eco"},
				{At: unix(base + 30), Old: "eco", New: "comfort"},
			},
		},
		{
			name: "missing attribute is a value: appearing and disappearing are changes",
			entries: []homeassistant.HistoryEntry{
				attrRow(base, base, map[string]any{"other": 1}),
				attrRow(base+10, base, preset("eco")),
				attrRow(base+20, base, map[string]any{"other": 1}),
			},
			attr:     "preset_mode",
			wantSeen: true,
			want: []attributeChange{
				{At: unix(base), Start: true},
				{At: unix(base + 10), New: "eco"},
				{At: unix(base + 20), Old: "eco"},
			},
		},
		{
			name: "attribute never present",
			entries: []homeassistant.HistoryEntry{
				attrRow(base, base, map[string]any{"other": 1}),
				attrRow(base+10, base, nil),
			},
			attr: "preset_mode",
			want: []attributeChange{{At: unix(base), Start: true}},
		},
		{
			name: "numbers stay native",
			entries: []homeassistant.HistoryEntry{
				attrRow(base, base, map[string]any{"temperature": float64(21)}),
				attrRow(base+10, base, map[string]any{"temperature": float64(18)}),
			},
			attr:     "temperature",
			wantSeen: true,
			want: []attributeChange{
				{At: unix(base), Start: true, New: float64(21)},
				{At: unix(base + 10), Old: float64(21), New: float64(18)},
			},
		},
		{
			name: "non-scalar compares by value and renders as compact json",
			entries: []homeassistant.HistoryEntry{
				attrRow(base, base, map[string]any{"modes": []any{"a", "b"}}),
				attrRow(base+10, base, map[string]any{"modes": []any{"a", "b"}}),
				attrRow(base+20, base, map[string]any{"modes": []any{"a", "c"}}),
			},
			attr:     "modes",
			wantSeen: true,
			want: []attributeChange{
				{At: unix(base), Start: true, New: `["a","b"]`},
				{At: unix(base + 20), Old: `["a","b"]`, New: `["a","c"]`},
			},
		},
		{
			name: "long non-scalar is cut to the display limit",
			entries: []homeassistant.HistoryEntry{
				attrRow(base, base, map[string]any{"list": longList}),
			},
			attr:     "list",
			wantSeen: true,
			want:     nil, // checked separately below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, seen := attributeChanges(tt.entries, tt.attr)
			if seen != tt.wantSeen {
				t.Errorf("seen = %v, want %v", seen, tt.wantSeen)
			}
			if tt.name == "long non-scalar is cut to the display limit" {
				assertLongValueCut(t, got)
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d changes %+v, want %d %+v", len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i].At.Unix() != tt.want[i].At.Unix() || got[i].Start != tt.want[i].Start ||
					!reflect.DeepEqual(got[i].Old, tt.want[i].Old) || !reflect.DeepEqual(got[i].New, tt.want[i].New) {
					t.Errorf("change[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func assertLongValueCut(t *testing.T, got []attributeChange) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("got %d changes, want 1 (start)", len(got))
	}
	s, ok := got[0].New.(string)
	if !ok {
		t.Fatalf("New = %#v, want a string", got[0].New)
	}
	if n := len([]rune(s)); n != attributeValueMaxLen+1 || !strings.HasSuffix(s, "…") {
		t.Errorf("New = %q (%d runes), want %d runes plus an ellipsis", s, n, attributeValueMaxLen)
	}
}

func TestLimitAttributeChanges(t *testing.T) {
	start := attributeChange{Start: true, New: "a"}
	c1 := attributeChange{Old: "a", New: "b"}
	c2 := attributeChange{Old: "b", New: "c"}
	c3 := attributeChange{Old: "c", New: "d"}

	tests := []struct {
		name      string
		in        []attributeChange
		limit     int
		want      []attributeChange
		wantTotal int
	}{
		{name: "empty", limit: 5},
		{name: "within limit keeps the start row", in: []attributeChange{start, c1, c2}, limit: 2, want: []attributeChange{start, c1, c2}, wantTotal: 2},
		{name: "over limit keeps the newest changes and drops the start row", in: []attributeChange{start, c1, c2, c3}, limit: 2, want: []attributeChange{c2, c3}, wantTotal: 3},
		{name: "start only", in: []attributeChange{start}, limit: 1, want: []attributeChange{start}, wantTotal: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total := limitAttributeChanges(tt.in, tt.limit)
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
