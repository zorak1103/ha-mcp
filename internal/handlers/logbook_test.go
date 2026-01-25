package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

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
				m.GetLogbookFn = func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
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
				m.GetLogbookFn = func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
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
				m.GetLogbookFn = func(_ context.Context, _, _, entityID string) ([]homeassistant.LogbookEntry, error) {
					if entityID != "light.living_room" {
						t.Errorf("expected entity_id %q, got %q", "light.living_room", entityID)
					}
					return testEntries, nil
				}
			},
			wantError:    false,
			wantContains: []string{"for light.living_room"},
		},
		{
			name: "API error",
			args: map[string]any{},
			setupMock: func(m *UniversalMockClient) {
				m.GetLogbookFn = func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
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

func TestLogbookHandlers_EmptyLogbook(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
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
				GetLogbookFn: func(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
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
