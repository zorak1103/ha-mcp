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
