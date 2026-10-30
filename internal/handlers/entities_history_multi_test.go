package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

const (
	multiHistTS1 = 1700000000
	multiHistTS2 = 1700003600
)

// multiHistoryRows returns two rows for sensor.a and one for sensor.b.
func multiHistoryRows() map[string][]homeassistant.HistoryEntry {
	return map[string][]homeassistant.HistoryEntry{
		"sensor.a": {
			{EntityID: "sensor.a", State: "20.5", LastChanged: multiHistTS1, Attributes: map[string]any{"friendly_name": "Alpha"}},
			{EntityID: "sensor.a", State: "21.0", LastChanged: multiHistTS2},
		},
		"sensor.b": {
			{EntityID: "sensor.b", State: "on", LastChanged: multiHistTS1, Attributes: map[string]any{"friendly_name": "Beta"}},
		},
	}
}

// historyCall records the entity IDs passed to GetHistory and the entities checked via GetState.
type historyCall struct {
	mu        sync.Mutex
	ids       []string
	stateKeys []string
}

func (c *historyCall) client(rows map[string][]homeassistant.HistoryEntry, existing ...string) *UniversalMockClient {
	return &UniversalMockClient{
		GetHistoryFn: func(_ context.Context, ids []string, _, _ time.Time) (map[string][]homeassistant.HistoryEntry, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.ids = slices.Clone(ids)
			return rows, nil
		},
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.stateKeys = append(c.stateKeys, entityID)
			if slices.Contains(existing, entityID) {
				return &homeassistant.Entity{EntityID: entityID, State: "ok"}, nil
			}
			return nil, errors.New("entity not found")
		},
	}
}

func runMultiHistory(t *testing.T, client *UniversalMockClient, args map[string]any) (string, bool) {
	t.Helper()

	full := map[string]any{"mode": modeHistory}
	maps.Copy(full, args)
	result, err := NewConsolidatedEntityQueryHandlers().handleQueryEntities(context.Background(), client, full)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(result.Content))
	}
	return result.Content[0].Text, result.IsError
}

func TestHandleHistory_MultiEntity_NaturalSectionsInRequestOrder(t *testing.T) {
	t.Parallel()

	var call historyCall
	// Request order is b, a: the output must follow it, not alphabetical order.
	text, isErr := runMultiHistory(t, call.client(multiHistoryRows()), map[string]any{
		"entity_ids": []any{"sensor.b", "sensor.a"}, "format": "natural",
	})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}

	if !slices.Equal(call.ids, []string{"sensor.b", "sensor.a"}) {
		t.Errorf("GetHistory ids = %v, want [sensor.b sensor.a]", call.ids)
	}
	minute := func(ts int64) string { return time.Unix(ts, 0).Format("2006-01-02 15:04") }
	want := "History for Beta: 1 state changes.\n" + minute(multiHistTS1) + " → on" +
		"\n\n" +
		"History for Alpha: 2 state changes.\n" + minute(multiHistTS1) + " → 20.5\n" + minute(multiHistTS2) + " → 21.0"
	if text != want {
		t.Errorf("output mismatch\ngot:\n%s\nwant:\n%s", text, want)
	}
	if len(call.stateKeys) != 0 {
		t.Errorf("GetState must not be called when every entity has history, got %v", call.stateKeys)
	}
}

func TestHandleHistory_MultiEntity_JSONSections(t *testing.T) {
	t.Parallel()

	var call historyCall
	text, isErr := runMultiHistory(t, call.client(multiHistoryRows()), map[string]any{
		"entity_ids": []any{"sensor.b", "sensor.a"}, "format": "json",
	})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}

	summary, body, found := strings.Cut(text, "\n\n")
	if !found {
		t.Fatalf("expected summary and body separated by a blank line, got: %s", text)
	}
	for _, line := range []string{"Found 1 history entries for sensor.b", "Found 2 history entries for sensor.a"} {
		if !strings.Contains(summary, line) {
			t.Errorf("summary %q missing %q", summary, line)
		}
	}
	if strings.Count(summary, VerboseHint) != 1 {
		t.Errorf("summary must carry the verbose hint exactly once, got: %q", summary)
	}

	var sections []struct {
		EntityID string           `json:"entity_id"`
		Entries  []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal([]byte(body), &sections); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, body)
	}
	if len(sections) != 2 || sections[0].EntityID != "sensor.b" || sections[1].EntityID != "sensor.a" {
		t.Fatalf("sections = %+v, want sensor.b then sensor.a", sections)
	}
	if len(sections[0].Entries) != 1 || len(sections[1].Entries) != 2 {
		t.Errorf("entry counts = %d, %d; want 1, 2", len(sections[0].Entries), len(sections[1].Entries))
	}
	if sections[1].Entries[0]["state"] != "20.5" {
		t.Errorf("compact entry = %v, want state 20.5", sections[1].Entries[0])
	}
}

func TestHandleHistory_MultiEntity_VerboseJSONKeepsAttributes(t *testing.T) {
	t.Parallel()

	var call historyCall
	text, _ := runMultiHistory(t, call.client(multiHistoryRows()), map[string]any{
		"entity_ids": []any{"sensor.a", "sensor.b"}, "format": "json", "verbose": true,
	})
	if !strings.Contains(text, `"friendly_name": "Alpha"`) || !strings.Contains(text, `"friendly_name": "Beta"`) {
		t.Errorf("verbose output must keep attributes of every entity, got: %s", text)
	}
	if strings.Contains(text, VerboseHint) {
		t.Errorf("verbose output must not carry the verbose hint, got: %s", text)
	}
}

// A one-element entity_ids list is the same request as entity_id and must render identically.
func TestHandleHistory_SingleEntityIDsMatchesEntityID(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"natural", "json"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			var call historyCall
			single, _ := runMultiHistory(t, call.client(multiHistoryRows()), map[string]any{
				"entity_id": "sensor.a", "format": format,
			})
			list, _ := runMultiHistory(t, call.client(multiHistoryRows()), map[string]any{
				"entity_ids": []any{"sensor.a"}, "format": format,
			})
			if single != list {
				t.Errorf("entity_ids=[x] differs from entity_id=x\nentity_id:\n%s\nentity_ids:\n%s", single, list)
			}
		})
	}
}

func TestHandleHistory_MultiEntity_DuplicateIDsCollapse(t *testing.T) {
	t.Parallel()

	var call historyCall
	text, _ := runMultiHistory(t, call.client(multiHistoryRows()), map[string]any{
		"entity_ids": []any{"sensor.a", "sensor.b", "sensor.a"}, "format": "natural",
	})
	if !slices.Equal(call.ids, []string{"sensor.a", "sensor.b"}) {
		t.Errorf("GetHistory ids = %v, want duplicates dropped", call.ids)
	}
	if strings.Count(text, "History for Alpha") != 1 {
		t.Errorf("sensor.a must appear once, got: %s", text)
	}
}

func TestHandleHistory_MultiEntity_EmptyHistoryChecksOnlyThatEntity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		format       string
		wantContains []string
		notContains  []string
	}{
		{
			name:   "natural",
			format: "natural",
			wantContains: []string{
				"History for Alpha: 2 state changes.",
				"No history found for sensor.quiet.",
				"Entity sensor.nope was not found in Home Assistant.",
			},
		},
		{
			name:   "json",
			format: "json",
			wantContains: []string{
				"Found 0 history entries for sensor.quiet",
				"Found 0 history entries for sensor.nope (entity not found in Home Assistant)",
			},
			notContains: []string{"sensor.quiet (entity not found"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var call historyCall
			client := call.client(multiHistoryRows(), "sensor.quiet")
			text, isErr := runMultiHistory(t, client, map[string]any{
				"entity_ids": []any{"sensor.a", "sensor.quiet", "sensor.nope"}, "format": tt.format,
			})
			if isErr {
				t.Fatalf("unexpected error result: %s", text)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(text, want) {
					t.Errorf("output missing %q\n%s", want, text)
				}
			}
			for _, bad := range tt.notContains {
				if strings.Contains(text, bad) {
					t.Errorf("output must not contain %q\n%s", bad, text)
				}
			}
			if !slices.Equal(call.stateKeys, []string{"sensor.quiet", "sensor.nope"}) {
				t.Errorf("GetState calls = %v, want only the entities without history", call.stateKeys)
			}
		})
	}
}

// state and limit apply to each entity separately, not to the merged rows.
func TestHandleHistory_MultiEntity_StateAndLimitPerEntity(t *testing.T) {
	t.Parallel()

	rows := map[string][]homeassistant.HistoryEntry{
		"sensor.a": {
			{State: "on", LastChanged: 1700000000},
			{State: "off", LastChanged: 1700000100},
			{State: "on", LastChanged: 1700000200},
			{State: "on", LastChanged: 1700000300},
		},
		"sensor.b": {
			{State: "on", LastChanged: 1700000050},
			{State: "off", LastChanged: 1700000150},
		},
	}

	t.Run("limit keeps the newest rows of each entity", func(t *testing.T) {
		t.Parallel()

		var call historyCall
		text, _ := runMultiHistory(t, call.client(rows), map[string]any{
			"entity_ids": []any{"sensor.a", "sensor.b"}, "format": "json", "limit": float64(2),
		})
		if !strings.Contains(text, "Showing 2 of 4 history entries for sensor.a (limited)") {
			t.Errorf("sensor.a must be limited on its own, got: %s", text)
		}
		if !strings.Contains(text, "Found 2 history entries for sensor.b") {
			t.Errorf("sensor.b has exactly the limit and must not be reported as limited, got: %s", text)
		}
	})

	t.Run("state filter applies to each entity", func(t *testing.T) {
		t.Parallel()

		var call historyCall
		text, _ := runMultiHistory(t, call.client(rows), map[string]any{
			"entity_ids": []any{"sensor.a", "sensor.b"}, "format": "json", "state": "on",
		})
		for _, want := range []string{
			"Found 3 history entries for sensor.a (filtered by state='on')",
			"Found 1 history entries for sensor.b (filtered by state='on')",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q\n%s", want, text)
			}
		}
	})

	t.Run("natural default cap is per entity", func(t *testing.T) {
		t.Parallel()

		many := make([]homeassistant.HistoryEntry, 25)
		for i := range many {
			many[i] = homeassistant.HistoryEntry{State: "s", LastChanged: float64(1700000000 + i*60)}
		}
		var call historyCall
		text, _ := runMultiHistory(t, call.client(map[string][]homeassistant.HistoryEntry{"sensor.a": many, "sensor.b": many}), map[string]any{
			"entity_ids": []any{"sensor.a", "sensor.b"}, "format": "natural",
		})
		if got := strings.Count(text, "→ s"); got != 40 {
			t.Errorf("rows shown = %d, want 20 per entity (40)", got)
		}
	})
}

func TestParseHistoryParams_EntityArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		want    []string
		wantErr string
	}{
		{name: "entity_id", args: map[string]any{"entity_id": "sensor.a"}, want: []string{"sensor.a"}},
		{name: "entity_ids", args: map[string]any{"entity_ids": []any{"sensor.a", "sensor.b"}}, want: []string{"sensor.a", "sensor.b"}},
		{
			name:    "both",
			args:    map[string]any{"entity_id": "sensor.a", "entity_ids": []any{"sensor.b"}},
			wantErr: "Cannot specify both entity_id and entity_ids",
		},
		{name: "neither", args: map[string]any{}, wantErr: "entity_id is required"},
		// An empty array counts as absent (isAbsentArg), like an empty entity_id string.
		{name: "empty list", args: map[string]any{"entity_ids": []any{}}, wantErr: "entity_id is required"},
		{name: "non-string element", args: map[string]any{"entity_ids": []any{"sensor.a", 5.0}}, wantErr: "entity_ids[1] must be a non-empty string"},
		{name: "empty entity_id", args: map[string]any{"entity_id": ""}, wantErr: "entity_id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params, err := parseHistoryParams(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(params.entityIDs, tt.want) {
				t.Errorf("entityIDs = %v, want %v", params.entityIDs, tt.want)
			}
		})
	}
}

func TestQueryEntities_EntityIDsArgumentDeclaredForHistoryOnly(t *testing.T) {
	t.Parallel()

	prop, ok := queryEntitiesProperties()[attrEntityIDs]
	if !ok {
		t.Fatal("query_entities schema must declare entity_ids")
	}
	if prop.Type != "array" || prop.Items == nil || prop.Items.Type != "string" {
		t.Errorf("entity_ids schema = %+v, want array of string", prop)
	}
	if modes := queryEntitiesArgModes[attrEntityIDs]; !slices.Equal(modes, []string{queryModeHistory}) {
		t.Errorf("entity_ids modes = %v, want [history]", modes)
	}

	client := &UniversalMockClient{}
	result, err := NewConsolidatedEntityQueryHandlers().handleQueryEntities(context.Background(), client, map[string]any{
		"mode": "current", "entity_ids": []any{"sensor.a"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Errorf("entity_ids must be rejected for mode=current, got: %s", result.Content[0].Text)
	}
}
