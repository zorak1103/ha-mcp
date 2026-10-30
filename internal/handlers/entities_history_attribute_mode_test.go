package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

const (
	attrHistTS = 1700000000
	attrEntity = "climate.office"
)

// attrHistoryCall records what the handler asked the client for.
type attrHistoryCall struct {
	mu   sync.Mutex
	ids  []string
	opts homeassistant.HistoryOptions
	n    int
}

func (c *attrHistoryCall) client(rows map[string][]homeassistant.HistoryEntry, existing ...string) *UniversalMockClient {
	return &UniversalMockClient{
		GetHistoryFn: func(_ context.Context, ids []string, _, _ time.Time, opts homeassistant.HistoryOptions) (map[string][]homeassistant.HistoryEntry, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.ids = slices.Clone(ids)
			c.opts = opts
			c.n++
			return rows, nil
		},
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			if slices.Contains(existing, entityID) {
				return &homeassistant.Entity{EntityID: entityID, State: "ok"}, nil
			}
			return nil, errors.New("entity not found")
		},
	}
}

// presetRows returns climate rows where preset_mode flips twice. The last_changed of every
// row is the stale time of the last state change, like HA sends it for attribute-only rows.
func presetRows(name string) map[string][]homeassistant.HistoryEntry {
	attrs := func(preset string) map[string]any {
		a := map[string]any{"preset_mode": preset, "current_temperature": 20.5}
		if name != "" {
			a["friendly_name"] = name
		}
		return a
	}
	return map[string][]homeassistant.HistoryEntry{
		attrEntity: {
			{State: "heat", Attributes: attrs("comfort"), LastUpdated: attrHistTS, LastChanged: attrHistTS - 7200},
			{State: "heat", Attributes: attrs("comfort"), LastUpdated: attrHistTS + 60, LastChanged: attrHistTS - 7200},
			{State: "heat", Attributes: attrs("eco"), LastUpdated: attrHistTS + 900, LastChanged: attrHistTS - 7200},
			{State: "heat", Attributes: attrs("comfort"), LastUpdated: attrHistTS + 3600, LastChanged: attrHistTS - 7200},
		},
	}
}

func minuteAt(ts int64) string { return time.Unix(ts, 0).Format("2006-01-02 15:04") }

func TestHandleHistory_Attribute_NaturalTransitions(t *testing.T) {
	t.Parallel()

	var call attrHistoryCall
	text, isErr := runMultiHistory(t, call.client(presetRows("Office Thermostat")), map[string]any{
		"entity_id": attrEntity, "attribute": "preset_mode", "format": "natural",
	})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}

	want := "Attribute preset_mode of Office Thermostat (climate.office): 2 changes.\n" +
		minuteAt(attrHistTS) + " (start) → comfort\n" +
		minuteAt(attrHistTS+900) + " comfort → eco\n" +
		minuteAt(attrHistTS+3600) + " eco → comfort"
	if text != want {
		t.Errorf("output mismatch\ngot:\n%s\nwant:\n%s", text, want)
	}
	if !call.opts.AllUpdates {
		t.Error("attribute mode must request every state update (significant_changes_only=false)")
	}
}

func TestHandleHistory_WithoutAttribute_KeepsDefaultHAQuery(t *testing.T) {
	t.Parallel()

	var call attrHistoryCall
	_, isErr := runMultiHistory(t, call.client(presetRows("")), map[string]any{"entity_id": attrEntity})
	if isErr {
		t.Fatal("unexpected error result")
	}
	if call.opts.AllUpdates {
		t.Error("default history must not change the HA query")
	}
}

func TestHandleHistory_Attribute_JSONSingleEntity(t *testing.T) {
	t.Parallel()

	var call attrHistoryCall
	text, isErr := runMultiHistory(t, call.client(presetRows("")), map[string]any{
		"entity_id": attrEntity, "attribute": "preset_mode", "format": "json",
	})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}

	summary, body, found := strings.Cut(text, "\n\n")
	if !found {
		t.Fatalf("expected summary and body separated by a blank line, got: %s", text)
	}
	if want := "Found 2 changes of attribute preset_mode for climate.office"; summary != want {
		t.Errorf("summary = %q, want %q", summary, want)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(body), &rows); err != nil {
		t.Fatalf("body is not a bare json array: %v\n%s", err, body)
	}
	wantAt := func(ts int64) string { return time.Unix(ts, 0).Format(time.RFC3339) }
	want := []map[string]any{
		{"at": wantAt(attrHistTS), "start": true, "new": "comfort"},
		{"at": wantAt(attrHistTS + 900), "old": "comfort", "new": "eco"},
		{"at": wantAt(attrHistTS + 3600), "old": "eco", "new": "comfort"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %s", len(rows), len(want), body)
	}
	for i := range want {
		if fmt.Sprint(rows[i]) != fmt.Sprint(want[i]) {
			t.Errorf("row[%d] = %v, want %v", i, rows[i], want[i])
		}
	}
}

func TestHandleHistory_Attribute_MultiEntityJSON(t *testing.T) {
	t.Parallel()

	rows := presetRows("")
	var call attrHistoryCall
	text, isErr := runMultiHistory(t, call.client(rows, "climate.gone"), map[string]any{
		"entity_ids": []any{"climate.gone", attrEntity, "climate.missing"},
		"attribute":  "preset_mode", "format": "json",
	})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	if !call.opts.AllUpdates {
		t.Error("attribute mode must request every state update")
	}

	summary, body, found := strings.Cut(text, "\n\n")
	if !found {
		t.Fatalf("expected summary and body separated by a blank line, got: %s", text)
	}
	var blocks []struct {
		EntityID string           `json:"entity_id"`
		Changes  []map[string]any `json:"changes"`
	}
	if err := json.Unmarshal([]byte(body), &blocks); err != nil {
		t.Fatalf("body: %v\n%s", err, body)
	}
	gotIDs := make([]string, 0, len(blocks))
	for _, b := range blocks {
		gotIDs = append(gotIDs, b.EntityID)
		if b.Changes == nil {
			t.Errorf("%s: changes must be an array, never null", b.EntityID)
		}
	}
	if !slices.Equal(gotIDs, []string{"climate.gone", attrEntity, "climate.missing"}) {
		t.Errorf("sections = %v, want request order", gotIDs)
	}
	if len(blocks[1].Changes) != 3 {
		t.Errorf("climate.office has %d rows, want 3 (start + 2 changes)", len(blocks[1].Changes))
	}
	if !strings.Contains(summary, "climate.missing") || !strings.Contains(summary, "entity not found in Home Assistant") {
		t.Errorf("summary must flag the unknown entity, got:\n%s", summary)
	}
}

func TestHandleHistory_Attribute_NotPresentInAnyRow(t *testing.T) {
	t.Parallel()

	var call attrHistoryCall
	text, isErr := runMultiHistory(t, call.client(presetRows("")), map[string]any{
		"entity_id": attrEntity, "attribute": "fan_mode", "format": "natural",
	})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	if !strings.HasPrefix(text, "Attribute fan_mode is not present in any history row of climate.office. ") {
		t.Errorf("got %q, want the not-present note", text)
	}
	if !strings.Contains(text, "recorder may not store it") {
		t.Errorf("note must mention that the recorder can exclude attributes, got %q", text)
	}

	jsonText, _ := runMultiHistory(t, call.client(presetRows("")), map[string]any{
		"entity_id": attrEntity, "attribute": "fan_mode", "format": "json",
	})
	summary, body, _ := strings.Cut(jsonText, "\n\n")
	if !strings.HasPrefix(summary, "Attribute fan_mode not present in any history row of climate.office") ||
		!strings.Contains(summary, "recorder may not store it") || body != "[]" {
		t.Errorf("json summary/body = %q / %q", summary, body)
	}
}

func TestHandleHistory_Attribute_NoRowsReusesNotFoundText(t *testing.T) {
	t.Parallel()

	var call attrHistoryCall
	text, isErr := runMultiHistory(t, call.client(map[string][]homeassistant.HistoryEntry{}), map[string]any{
		"entity_id": "climate.nope", "attribute": "preset_mode", "format": "natural",
	})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	if !strings.Contains(text, "Entity climate.nope was not found in Home Assistant") {
		t.Errorf("got %q, want the not-found text", text)
	}
}

func TestHandleHistory_Attribute_LimitDefaultsTo50AndCountsChanges(t *testing.T) {
	t.Parallel()

	const flips = 60
	rows := make([]homeassistant.HistoryEntry, 0, flips+1)
	for i := 0; i <= flips; i++ {
		rows = append(rows, homeassistant.HistoryEntry{
			State: "heat", Attributes: map[string]any{"n": float64(i)}, LastUpdated: float64(attrHistTS + i*60),
		})
	}
	data := map[string][]homeassistant.HistoryEntry{attrEntity: rows}

	var call attrHistoryCall
	text, isErr := runMultiHistory(t, call.client(data), map[string]any{
		"entity_id": attrEntity, "attribute": "n", "format": "natural",
	})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	if got := strings.Count(text, " → "); got != 50 {
		t.Errorf("default shows %d rows, want 50", got)
	}
	if !strings.Contains(text, "60 changes.") || !strings.Contains(text, "[Showing 50 of 60 changes. Use limit parameter for more.]") {
		t.Errorf("missing totals or truncation notice:\n%s", text)
	}
	// 60 changes 1..60; the newest 50 are 11..60, so the oldest shown is "10 → 11".
	if !strings.Contains(text, " 10 → 11") || !strings.Contains(text, " 59 → 60") || strings.Contains(text, " 9 → 10") {
		t.Errorf("must keep exactly the newest 50 changes:\n%s", text)
	}
	if strings.Contains(text, "(start)") {
		t.Error("a truncated list must drop the start row")
	}

	text, _ = runMultiHistory(t, call.client(data), map[string]any{
		"entity_id": attrEntity, "attribute": "n", "format": "natural", "limit": float64(5),
	})
	if got := strings.Count(text, " → "); got != 5 {
		t.Errorf("explicit limit shows %d rows, want 5", got)
	}
}

func TestHandleHistory_Attribute_RejectedArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "state filter", args: map[string]any{"entity_id": attrEntity, "attribute": "preset_mode", "state": "heat"}, want: "state"},
		{name: "non-string attribute", args: map[string]any{"entity_id": attrEntity, "attribute": float64(3)}, want: "attribute must be a non-empty string"},
		{name: "blank attribute", args: map[string]any{"entity_id": attrEntity, "attribute": "   "}, want: "attribute must be a non-empty string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var call attrHistoryCall
			text, isErr := runMultiHistory(t, call.client(presetRows("")), tt.args)
			if !isErr {
				t.Fatalf("expected an error result, got: %s", text)
			}
			if !strings.Contains(text, tt.want) {
				t.Errorf("error = %q, want it to contain %q", text, tt.want)
			}
			if call.n != 0 {
				t.Error("GetHistory must not be called for a rejected argument")
			}
		})
	}
}

func TestQueryEntities_AttributeOnlyForHistoryMode(t *testing.T) {
	t.Parallel()

	result, err := NewConsolidatedEntityQueryHandlers().handleQueryEntities(context.Background(), &UniversalMockClient{},
		map[string]any{"mode": "current", "attribute": "preset_mode"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].Text, "attribute") {
		t.Errorf("mode=current with attribute must be rejected, got: %+v", result)
	}
}
