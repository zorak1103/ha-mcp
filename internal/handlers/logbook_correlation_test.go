package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestLogbookCorrelation_FetchFailures(t *testing.T) {
	t.Parallel()

	when := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	mockFailing := func(failing ...string) *UniversalMockClient {
		return &UniversalMockClient{
			GetLogbookFn: func(_ context.Context, _, _ string, entityIDs []string) ([]homeassistant.LogbookEntry, error) {
				if len(entityIDs) != 1 {
					return nil, fmt.Errorf("expected one entity per fetch, got %v", entityIDs)
				}
				if slices.Contains(failing, entityIDs[0]) {
					return nil, errors.New("boom")
				}
				return []homeassistant.LogbookEntry{{When: when, Name: "n", State: "on", EntityID: entityIDs[0]}}, nil
			},
		}
	}
	base := func(format string) map[string]any {
		return map[string]any{
			"mode":       "correlation",
			"entity_ids": []any{"light.a", "switch.b"},
			"hours":      float64(1),
			"format":     format,
		}
	}

	t.Run("partial failure adds a warning block", func(t *testing.T) {
		t.Parallel()
		res, err := NewLogbookHandlers().handleGetLogbook(context.Background(), mockFailing("switch.b"), base("natural"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError {
			t.Fatalf("want success, got error: %s", res.Content[0].Text)
		}
		if len(res.Content) != 2 {
			t.Fatalf("len(Content) = %d, want 2", len(res.Content))
		}
		const want = "WARNING: ⚠ 1 source(s) could not be scanned: switch.b (boom)"
		if !strings.HasPrefix(res.Content[1].Text, want) {
			t.Errorf("Content[1] = %q, want prefix %q", res.Content[1].Text, want)
		}
		// The successful entity's event must still be analyzed.
		if !strings.Contains(res.Content[0].Text, "Total Events: 1") {
			t.Errorf("Content[0] dropped the successful entity's event: %s", res.Content[0].Text)
		}
	})

	t.Run("json output stays valid with a warning", func(t *testing.T) {
		t.Parallel()
		res, err := NewLogbookHandlers().handleGetLogbook(context.Background(), mockFailing("switch.b"), base("json"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError || len(res.Content) != 2 {
			t.Fatalf("IsError=%v len(Content)=%d, want success with 2 blocks", res.IsError, len(res.Content))
		}
		if !json.Valid([]byte(res.Content[0].Text)) {
			t.Errorf("Content[0] is not valid JSON: %s", res.Content[0].Text)
		}
		var report CorrelationReport
		if err := json.Unmarshal([]byte(res.Content[0].Text), &report); err != nil {
			t.Fatalf("decoding report: %v", err)
		}
		if report.TotalEvents != 1 {
			t.Errorf("total_events = %d, want 1 (the successful entity's event)", report.TotalEvents)
		}
	})

	t.Run("all fetches failing is an error", func(t *testing.T) {
		t.Parallel()
		res, err := NewLogbookHandlers().handleGetLogbook(context.Background(), mockFailing("light.a", "switch.b"), base("natural"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("want IsError, got: %s", res.Content[0].Text)
		}
		for _, want := range []string{"Could not fetch the logbook for any entity", "light.a (boom)", "switch.b (boom)"} {
			if !strings.Contains(res.Content[0].Text, want) {
				t.Errorf("error %q does not contain %q", res.Content[0].Text, want)
			}
		}
	})

	t.Run("non-string element is rejected", func(t *testing.T) {
		t.Parallel()
		args := base("natural")
		args["entity_ids"] = []any{"light.a", 3.0}
		res, err := NewLogbookHandlers().handleGetLogbook(context.Background(), mockFailing(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError || !strings.Contains(res.Content[0].Text, "entity_ids[1] must be a non-empty string") {
			t.Errorf("IsError=%v text=%q", res.IsError, res.Content[0].Text)
		}
	})
}

// TestHandleCorrelation_SingleEntityEmpty pins the single-entity correlation result: the
// entries are counted, but a group needs at least 2 distinct entities, so correlations stay
// empty even though all three events fall inside the default 60s window. Both formats carry
// a hint that points to mode=entries.
func TestHandleCorrelation_SingleEntityEmpty(t *testing.T) {
	t.Parallel()

	base := time.Now().Add(-30 * time.Minute)
	client := &UniversalMockClient{
		GetLogbookFn: func(_ context.Context, _, _ string, entityIDs []string) ([]homeassistant.LogbookEntry, error) {
			if len(entityIDs) != 1 || entityIDs[0] != "climate.living_room" {
				return nil, fmt.Errorf("unexpected entity_ids %v", entityIDs)
			}
			entries := make([]homeassistant.LogbookEntry, 0, 3)
			for i, state := range []string{"heat", "off", "heat"} {
				entries = append(entries, homeassistant.LogbookEntry{
					When:     base.Add(time.Duration(i*10) * time.Second).Format(time.RFC3339),
					Name:     "Living Room",
					State:    state,
					EntityID: "climate.living_room",
				})
			}
			return entries, nil
		},
	}
	args := func(format string) map[string]any {
		return map[string]any{
			"mode":       "correlation",
			"entity_ids": []any{"climate.living_room"},
			"hours":      float64(1),
			"format":     format,
		}
	}

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		res, err := NewLogbookHandlers().handleGetLogbook(context.Background(), client, args("json"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError || len(res.Content) != 1 {
			t.Fatalf("IsError=%v len(Content)=%d, want success with 1 block", res.IsError, len(res.Content))
		}
		text := res.Content[0].Text
		var report CorrelationReport
		if err := json.Unmarshal([]byte(text), &report); err != nil {
			t.Fatalf("decoding report: %v", err)
		}
		if report.TotalEvents != 3 {
			t.Errorf("total_events = %d, want 3", report.TotalEvents)
		}
		if !strings.Contains(text, `"correlations": []`) {
			t.Errorf("want an empty correlations array (not null), got: %s", text)
		}
		if !slices.Equal(report.EntityIDs, []string{"climate.living_room"}) {
			t.Errorf("entity_ids = %v, want [climate.living_room]", report.EntityIDs)
		}
		if report.Hint != correlationSingleEntityHint {
			t.Errorf("hint = %q, want %q", report.Hint, correlationSingleEntityHint)
		}
	})

	t.Run("natural", func(t *testing.T) {
		t.Parallel()
		res, err := NewLogbookHandlers().handleGetLogbook(context.Background(), client, args("natural"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError || len(res.Content) != 1 {
			t.Fatalf("IsError=%v len(Content)=%d, want success with 1 block", res.IsError, len(res.Content))
		}
		text := res.Content[0].Text
		for _, want := range []string{
			"Entities: climate.living_room\n",
			"Total Events: 3\n",
			"Correlated Groups: 0\n",
			correlationSingleEntityHint,
		} {
			if !strings.Contains(text, want) {
				t.Errorf("output does not contain %q:\n%s", want, text)
			}
		}
		// The hint replaces the generic sentence, which would wrongly blame the time window.
		if strings.Contains(text, "No correlated events found.") {
			t.Errorf("single-entity output still has the generic no-correlation sentence:\n%s", text)
		}
	})
}

// TestHandleCorrelation_TwoEntitiesNoGroupsHasNoHint pins that the single-entity hint is not
// shown when 2 entities are given but their events fall outside the window: there the
// generic "No correlated events found" sentence is the true explanation.
func TestHandleCorrelation_TwoEntitiesNoGroupsHasNoHint(t *testing.T) {
	t.Parallel()

	base := time.Now().Add(-50 * time.Minute)
	client := &UniversalMockClient{
		GetLogbookFn: func(_ context.Context, _, _ string, entityIDs []string) ([]homeassistant.LogbookEntry, error) {
			if len(entityIDs) != 1 {
				return nil, fmt.Errorf("expected one entity per fetch, got %v", entityIDs)
			}
			var offset time.Duration
			if entityIDs[0] == "switch.b" {
				offset = 10 * time.Minute // well outside the default 60s window
			}
			return []homeassistant.LogbookEntry{{
				When:     base.Add(offset).Format(time.RFC3339),
				Name:     entityIDs[0],
				State:    "on",
				EntityID: entityIDs[0],
			}}, nil
		},
	}
	args := func(format string) map[string]any {
		return map[string]any{
			"mode":       "correlation",
			"entity_ids": []any{"light.a", "switch.b"},
			"hours":      float64(1),
			"format":     format,
		}
	}

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		res, err := NewLogbookHandlers().handleGetLogbook(context.Background(), client, args("json"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsError || len(res.Content) != 1 {
			t.Fatalf("IsError=%v len(Content)=%d, want success with 1 block", res.IsError, len(res.Content))
		}
		text := res.Content[0].Text
		if strings.Contains(text, `"hint"`) {
			t.Errorf("want no hint key with 2 entities, got: %s", text)
		}
		if !strings.Contains(text, `"correlations": []`) {
			t.Errorf("want an empty correlations array, got: %s", text)
		}
	})

	t.Run("natural", func(t *testing.T) {
		t.Parallel()
		res, err := NewLogbookHandlers().handleGetLogbook(context.Background(), client, args("natural"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := res.Content[0].Text
		if !strings.Contains(text, "No correlated events found.") {
			t.Errorf("want the generic no-correlation sentence, got:\n%s", text)
		}
		if strings.Contains(text, correlationSingleEntityHint) {
			t.Errorf("want no single-entity hint with 2 entities, got:\n%s", text)
		}
	})
}

func TestLogbook_Correlation(t *testing.T) {
	t.Parallel()

	now := time.Now()
	baseTime := now.Add(-1 * time.Hour)

	tests := []struct {
		name         string
		args         map[string]any
		setupMock    func(*UniversalMockClient)
		wantError    bool
		wantContains []string
	}{
		{
			name: "correlation with natural format",
			args: map[string]any{
				"mode":       "correlation",
				"entity_ids": []any{"binary_sensor.motion", "light.hallway"},
				"hours":      float64(1),
				"format":     "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLogbookFn = func(_ context.Context, _, _ string, entityIDs []string) ([]homeassistant.LogbookEntry, error) {
					entityID := ""
					if len(entityIDs) == 1 {
						entityID = entityIDs[0]
					}
					if entityID == "binary_sensor.motion" {
						return []homeassistant.LogbookEntry{
							{
								When:     baseTime.Format(time.RFC3339),
								Name:     "Motion Sensor",
								State:    "on",
								EntityID: "binary_sensor.motion",
							},
						}, nil
					}
					if entityID == "light.hallway" {
						return []homeassistant.LogbookEntry{
							{
								When:     baseTime.Add(2 * time.Second).Format(time.RFC3339),
								Name:     "Hallway Light",
								State:    "on",
								EntityID: "light.hallway",
							},
						}, nil
					}
					return []homeassistant.LogbookEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Correlation Analysis", "Total Events", "Correlations"},
		},
		{
			name: "correlation with json format",
			args: map[string]any{
				"mode":       "correlation",
				"entity_ids": []any{"sensor.temp", "climate.thermostat"},
				"hours":      float64(1),
				"format":     "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetLogbookFn = func(_ context.Context, _, _ string, _ []string) ([]homeassistant.LogbookEntry, error) {
					return []homeassistant.LogbookEntry{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"entity_ids", "time_range", "total_events", "correlations"},
		},
		{
			name: "correlation without entity_ids",
			args: map[string]any{
				"mode":   "correlation",
				"hours":  float64(1),
				"format": "natural",
			},
			wantError:    true,
			wantContains: []string{"entity_ids", "required"},
		},
		{
			name:         "correlation with null entity_ids counts as absent",
			args:         map[string]any{"mode": "correlation", "entity_ids": nil, "hours": float64(1)},
			wantError:    true,
			wantContains: []string{"entity_ids is required for correlation mode"},
		},
		{
			name:         "correlation with empty-string entity_ids counts as absent",
			args:         map[string]any{"mode": "correlation", "entity_ids": "", "hours": float64(1)},
			wantError:    true,
			wantContains: []string{"entity_ids is required for correlation mode"},
		},
		{
			name:         "correlation with empty entity_ids array counts as absent",
			args:         map[string]any{"mode": "correlation", "entity_ids": []any{}, "hours": float64(1)},
			wantError:    true,
			wantContains: []string{"entity_ids is required for correlation mode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewLogbookHandlers()
			client := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(client)
			}

			result, err := h.handleGetLogbook(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleGetLogbook() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v. Content: %s", result.IsError, tt.wantError, result.Content[0].Text)
			}

			content := result.Content[0].Text
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, got: %s", want, content)
				}
			}
		})
	}
}
