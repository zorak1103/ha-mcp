package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestLogbookHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewLogbookHandlers()
	registry := mcp.NewRegistry()
	h.RegisterTools(registry)

	tools := registry.ListTools()
	if len(tools) != 1 {
		t.Errorf("RegisterTools() registered %d tools, want 1", len(tools))
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "get_logbook" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected get_logbook tool to be registered")
	}
}

func TestLogbookHandlers_GetLogbookTool(t *testing.T) {
	t.Parallel()

	h := NewLogbookHandlers()
	tool := h.getLogbookTool()

	verifyToolSchema(t, tool, toolSchemaExpectation{
		ExpectedName:    "get_logbook",
		RequiredParams:  []string{},
		OptionalParams:  []string{"start_time", "end_time", "entity_id", "hours"},
		WantDescription: true,
	})
}

func TestLogbookHandlers_HandleGetLogbook(t *testing.T) {
	t.Parallel()

	testEntries := []homeassistant.LogbookEntry{
		{
			When:     "2024-01-15T10:00:00Z",
			Name:     "Living Room Light",
			EntityID: "light.living_room",
			State:    "on",
		},
		{
			When:     "2024-01-15T10:05:00Z",
			Name:     "Living Room Light",
			EntityID: "light.living_room",
			State:    "off",
		},
	}

	tests := []handlerTestCase{
		{
			name: "successful logbook fetch with defaults",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetLogbookFn = func(_ context.Context, _, _ string, _ []string) ([]homeassistant.LogbookEntry, error) {
					return testEntries, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 logbook entries", "Living Room Light", "entry_count"},
		},
		{
			name: "with hours parameter",
			args: map[string]any{"hours": 6.0},
			setupMock: func(m *UniversalMockClient) {
				m.GetLogbookFn = func(_ context.Context, _, _ string, _ []string) ([]homeassistant.LogbookEntry, error) {
					return testEntries, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Found 2 logbook entries"},
		},
		{
			name: "with entity_id filter",
			args: map[string]any{"entity_id": "light.living_room"},
			setupMock: func(m *UniversalMockClient) {
				m.GetLogbookFn = func(_ context.Context, _, _ string, entityIDs []string) ([]homeassistant.LogbookEntry, error) {
					if want := []string{"light.living_room"}; !slices.Equal(entityIDs, want) {
						t.Errorf("expected entity ids %v, got %v", want, entityIDs)
					}
					return testEntries, nil
				}
			},
			wantError:    false,
			wantContains: []string{"for light.living_room"},
		},
		{
			name: "with entity_ids filter",
			args: map[string]any{"entity_ids": []any{"light.a", "switch.b"}},
			setupMock: func(m *UniversalMockClient) {
				m.GetLogbookFn = func(_ context.Context, _, _ string, entityIDs []string) ([]homeassistant.LogbookEntry, error) {
					if want := []string{"light.a", "switch.b"}; !slices.Equal(entityIDs, want) {
						t.Errorf("expected entity ids %v, got %v", want, entityIDs)
					}
					return testEntries, nil
				}
			},
			wantError:    false,
			wantContains: []string{"for 2 entities"},
		},
		{
			name: "entity_id and entity_ids",
			args: map[string]any{"entity_id": "light.a", "entity_ids": []any{"switch.b"}},
			setupMock: func(m *UniversalMockClient) {
				m.GetLogbookFn = func(context.Context, string, string, []string) ([]homeassistant.LogbookEntry, error) {
					t.Error("GetLogbook must not be called when the arguments are invalid")
					return nil, nil
				}
			},
			wantError:    true,
			wantContains: []string{"Cannot specify both"},
		},
		{
			name: "API error",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetLogbookFn = func(_ context.Context, _, _ string, _ []string) ([]homeassistant.LogbookEntry, error) {
					return nil, errors.New("connection refused")
				}
			},
			wantError:    true,
			wantContains: []string{"Error getting logbook", "connection refused"},
		},
		{
			name:         "invalid start_time format",
			args:         map[string]any{"start_time": "invalid-date"},
			wantError:    true,
			wantContains: []string{"Invalid time format", "start_time"},
		},
		{
			name:         "invalid end_time format",
			args:         map[string]any{"end_time": "invalid-date"},
			wantError:    true,
			wantContains: []string{"Invalid time format", "end_time"},
		},
	}

	h := NewLogbookHandlers()
	runHandlerTestCases(t, tests, h.handleGetLogbook)
}

func TestLogbookHandlers_ModeValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		wantError    bool
		wantCall     bool
		wantContains []string
	}{
		{
			name:         "unknown mode",
			args:         map[string]any{"mode": "bogus"},
			wantError:    true,
			wantContains: []string{`Invalid mode "bogus". Must be one of: entries, correlation`},
		},
		{
			name:         "correlation-only arg in default entries mode",
			args:         map[string]any{"max_time_delta_seconds": 30.0},
			wantError:    true,
			wantContains: []string{"max_time_delta_seconds (only for mode=correlation)", "mode=entries"},
		},
		{
			name:         "entries-only limit in correlation mode",
			args:         map[string]any{"mode": "correlation", "entity_ids": []any{"light.a", "light.b"}, "limit": 5.0},
			wantError:    true,
			wantContains: []string{"limit (only for mode=entries)"},
		},
		{
			name:         "entries-only entity_id in correlation mode",
			args:         map[string]any{"mode": "correlation", "entity_id": "light.a", "entity_ids": []any{"light.a"}},
			wantError:    true,
			wantContains: []string{"entity_id (only for mode=entries)"},
		},
		{
			name:         "numeric mode",
			args:         map[string]any{"mode": 5.0},
			wantError:    true,
			wantContains: []string{"Invalid mode: expected a string, got float64. Must be one of: entries, correlation"},
		},
		{
			name:         "boolean mode",
			args:         map[string]any{"mode": true},
			wantError:    true,
			wantContains: []string{"Invalid mode: expected a string, got bool. Must be one of: entries, correlation"},
		},
		{
			name:         "array mode",
			args:         map[string]any{"mode": []any{"correlation"}},
			wantError:    true,
			wantContains: []string{"Invalid mode: expected a string, got []interface {}. Must be one of: entries, correlation"},
		},
		{
			name:         "nil mode means entries",
			args:         map[string]any{"mode": nil},
			wantCall:     true,
			wantContains: []string{"Found 0 logbook entries"},
		},
		{
			name:         "empty mode means entries",
			args:         map[string]any{"mode": ""},
			wantCall:     true,
			wantContains: []string{"Found 0 logbook entries"},
		},
		{
			name:         "placeholder for other-mode arg is accepted",
			args:         map[string]any{"max_time_delta_seconds": nil},
			wantCall:     true,
			wantContains: []string{"Found 0 logbook entries"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			client := &UniversalMockClient{
				GetLogbookFn: func(context.Context, string, string, []string) ([]homeassistant.LogbookEntry, error) {
					called = true
					return []homeassistant.LogbookEntry{}, nil
				},
			}
			res, err := NewLogbookHandlers().handleGetLogbook(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.IsError != tt.wantError {
				t.Fatalf("IsError = %v, want %v: %+v", res.IsError, tt.wantError, res)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(res.Content[0].Text, want) {
					t.Errorf("result %q does not contain %q", res.Content[0].Text, want)
				}
			}
			if called != tt.wantCall {
				t.Errorf("GetLogbook called = %v, want %v", called, tt.wantCall)
			}
		})
	}
}

func TestLogbookEntries_CursorStableAcrossIDOrder(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetLogbookFn: func(context.Context, string, string, []string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{
				{When: "2024-01-15T10:00:00Z", Name: "A", EntityID: "light.a", State: "on"},
				{When: "2024-01-15T10:01:00Z", Name: "B", EntityID: "light.b", State: "on"},
				{When: "2024-01-15T10:02:00Z", Name: "A", EntityID: "light.a", State: "off"},
			}, nil
		},
	}
	h := NewLogbookHandlers()
	// Fixed time window so the cursor's filter hash is not affected by the clock.
	window := map[string]any{"start_time": "2024-01-15T00:00:00Z", "end_time": "2024-01-16T00:00:00Z"}
	with := func(extra map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range window {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}

	first, err := h.handleGetLogbook(context.Background(), client,
		with(map[string]any{"entity_ids": []any{"light.a", "light.b"}, "limit": 1.0}))
	if err != nil || first.IsError {
		t.Fatalf("first page failed: err=%v result=%+v", err, first)
	}
	_, body, ok := strings.Cut(first.Content[0].Text, "\n\n")
	if !ok {
		t.Fatalf("no JSON body in %q", first.Content[0].Text)
	}
	var parsed struct {
		Pagination struct {
			NextCursor string `json:"next_cursor"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("parsing first page: %v", err)
	}
	cursor := parsed.Pagination.NextCursor
	if cursor == "" {
		t.Fatal("expected a next_cursor on the first page")
	}

	t.Run("reordered ids keep the cursor valid", func(t *testing.T) {
		res, err := h.handleGetLogbook(context.Background(), client,
			with(map[string]any{"entity_ids": []any{"light.b", "light.a"}, "cursor": cursor}))
		if err != nil || res.IsError {
			t.Fatalf("expected success, got err=%v result=%+v", err, res)
		}
	})
	t.Run("duplicated ids keep the cursor valid", func(t *testing.T) {
		res, err := h.handleGetLogbook(context.Background(), client,
			with(map[string]any{"entity_ids": []any{"light.a", "light.b", "light.a"}, "cursor": cursor}))
		if err != nil || res.IsError {
			t.Fatalf("expected success, got err=%v result=%+v", err, res)
		}
	})
	t.Run("different id set invalidates the cursor", func(t *testing.T) {
		res, err := h.handleGetLogbook(context.Background(), client,
			with(map[string]any{"entity_ids": []any{"light.a"}, "cursor": cursor}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError || !strings.Contains(res.Content[0].Text, "filters have changed") {
			t.Fatalf("expected a filters-changed error, got %+v", res)
		}
	})
}

func TestBuildLogbookFiltersMap_SortedCopyDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	now := time.Now()
	ids := []string{"switch.b", "light.a"}
	filters := buildLogbookFiltersMap(now, now, ids)

	if want := []string{"switch.b", "light.a"}; !slices.Equal(ids, want) {
		t.Errorf("caller slice mutated: got %v, want %v", ids, want)
	}
	got, ok := filters["entity_ids"].([]string)
	if !ok || !slices.Equal(got, []string{"light.a", "switch.b"}) {
		t.Errorf("filters[entity_ids] = %#v, want sorted copy", filters["entity_ids"])
	}
	if _, present := buildLogbookFiltersMap(now, now, nil)["entity_ids"]; present {
		t.Error("entity_ids must be absent from the filters map when no ids are supplied")
	}
}

func TestLogbookHandlers_EmptyLogbook(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetLogbookFn: func(_ context.Context, _, _ string, _ []string) ([]homeassistant.LogbookEntry, error) {
			return []homeassistant.LogbookEntry{}, nil
		},
	}

	h := NewLogbookHandlers()
	result, err := h.handleGetLogbook(context.Background(), client, map[string]any{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("expected no error in result")
	}

	content := result.Content[0].Text
	if !strings.Contains(content, "Found 0 logbook entries") {
		t.Error("expected 'Found 0 logbook entries' in output")
	}
	if !strings.Contains(content, `"entry_count": 0`) {
		t.Error("expected entry_count to be 0")
	}
}

func TestLogbookHandlers_TimeFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      map[string]any
		wantError bool
	}{
		{
			name:      "RFC3339 format",
			args:      map[string]any{"start_time": "2024-01-15T10:00:00Z"},
			wantError: false,
		},
		{
			name:      "ISO format without timezone",
			args:      map[string]any{"start_time": "2024-01-15T10:00:00"},
			wantError: false,
		},
		{
			name:      "hours as int",
			args:      map[string]any{"hours": 6},
			wantError: false,
		},
		{
			name:      "hours as float",
			args:      map[string]any{"hours": 6.5},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{
				GetLogbookFn: func(_ context.Context, _, _ string, _ []string) ([]homeassistant.LogbookEntry, error) {
					return []homeassistant.LogbookEntry{}, nil
				},
			}

			h := NewLogbookHandlers()
			result, err := h.handleGetLogbook(context.Background(), client, tt.args)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}
		})
	}
}
