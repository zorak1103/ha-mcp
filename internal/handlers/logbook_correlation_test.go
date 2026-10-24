package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

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
