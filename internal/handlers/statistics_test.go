package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockStatisticsClient implements homeassistant.Client for testing.
type mockStatisticsClient struct {
	homeassistant.Client
	getStatisticsFn func(ctx context.Context, statIDs []string, period string) ([]homeassistant.StatisticsResult, error)
}

func (m *mockStatisticsClient) GetStatistics(ctx context.Context, statIDs []string, period string) ([]homeassistant.StatisticsResult, error) {
	if m.getStatisticsFn != nil {
		return m.getStatisticsFn(ctx, statIDs, period)
	}
	return []homeassistant.StatisticsResult{}, nil
}

func TestNewStatisticsHandlers(t *testing.T) {
	t.Parallel()

	h := NewStatisticsHandlers()
	if h == nil {
		t.Error("NewStatisticsHandlers() returned nil")
	}
}

func TestStatisticsHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewStatisticsHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 1
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	expectedTools := map[string]bool{
		"get_statistics": false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("Tool %q not registered", name)
		}
	}
}

func TestStatisticsHandlers_getStatisticsTool(t *testing.T) {
	t.Parallel()

	h := NewStatisticsHandlers()
	tool := h.getStatisticsTool()

	if tool.Name != "get_statistics" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "get_statistics")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	requiredFields := map[string]bool{"statistic_ids": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}
}

func TestStatisticsHandlers_handleGetStatistics(t *testing.T) {
	t.Parallel()

	meanVal := 100.5
	minVal := 50.0
	maxVal := 150.0

	tests := []struct {
		name             string
		args             map[string]any
		getStatisticsErr error
		getStatistics    []homeassistant.StatisticsResult
		wantError        bool
		wantContains     string
	}{
		{
			name: "success",
			args: map[string]any{
				"statistic_ids": []any{"sensor.energy_consumption"},
			},
			getStatistics: []homeassistant.StatisticsResult{
				{
					StatisticID: "sensor.energy_consumption",
					Start:       1704067200, // 2024-01-01T00:00:00 UTC
					Mean:        &meanVal,
				},
			},
			wantError: false,
		},
		{
			name: "success with period",
			args: map[string]any{
				"statistic_ids": []any{"sensor.energy_consumption", "sensor.temperature"},
				"period":        "day",
			},
			getStatistics: []homeassistant.StatisticsResult{
				{StatisticID: "sensor.energy_consumption", Start: 1704067200, Mean: &meanVal},
				{StatisticID: "sensor.temperature", Start: 1704067200, Min: &minVal, Max: &maxVal},
			},
			wantError: false,
		},
		{
			name: "success with different periods",
			args: map[string]any{
				"statistic_ids": []any{"sensor.power"},
				"period":        "5minute",
			},
			getStatistics: []homeassistant.StatisticsResult{},
			wantError:     false,
		},
		{
			name:      "missing statistic_ids",
			args:      map[string]any{},
			wantError: true,
		},
		{
			name: "statistic_ids not array",
			args: map[string]any{
				"statistic_ids": "sensor.energy",
			},
			wantError: true,
		},
		{
			name: "empty statistic_ids array",
			args: map[string]any{
				"statistic_ids": []any{},
			},
			wantError: true,
		},
		{
			name: "client error",
			args: map[string]any{
				"statistic_ids": []any{"sensor.energy_consumption"},
			},
			getStatisticsErr: errors.New("statistics not available"),
			wantError:        true,
			wantContains:     "Failed to get statistics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockStatisticsClient{
				getStatisticsFn: func(_ context.Context, _ []string, _ string) ([]homeassistant.StatisticsResult, error) {
					if tt.getStatisticsErr != nil {
						return nil, tt.getStatisticsErr
					}
					return tt.getStatistics, nil
				},
			}

			h := NewStatisticsHandlers()
			result, err := h.handleGetStatistics(context.Background(), client, tt.args)

			// For some validation errors, the handler returns an error directly
			if err != nil {
				if !tt.wantError {
					t.Errorf("handleGetStatistics() returned unexpected error: %v", err)
				}
				return
			}

			if result == nil {
				if tt.wantError {
					return // Expected error path with nil result
				}
				t.Error("handleGetStatistics() returned nil result")
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}
