// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Tests use UniversalMockClient from testing_helpers_test.go

func TestHandleGetState(t *testing.T) {
	t.Parallel()

	testEntityData := &homeassistant.Entity{
		EntityID: "light.living_room",
		State:    "on",
		Attributes: map[string]any{
			"friendly_name": "Living Room Light",
			"brightness":    255,
		},
		LastChanged: time.Now(),
		LastUpdated: time.Now(),
	}

	tests := []struct {
		name         string
		args         map[string]any
		setupMock    func(*UniversalMockClient)
		wantError    bool
		wantContains []string
	}{
		{
			name: "success - returns entity state",
			args: map[string]any{"entity_id": "light.living_room", "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return testEntityData, nil
				}
			},
			wantError:    false,
			wantContains: []string{"light.living_room", "on", "Living Room Light", "brightness", "255"},
		},
		{
			name:         "error - missing entity_id",
			args:         map[string]any{},
			setupMock:    nil,
			wantError:    true,
			wantContains: []string{"entity_id or entity_ids is required"},
		},
		{
			name:         "error - empty entity_id",
			args:         map[string]any{"entity_id": ""},
			setupMock:    nil,
			wantError:    true,
			wantContains: []string{"entity_id is required"},
		},
		{
			name: "error - client error",
			args: map[string]any{"entity_id": "light.nonexistent"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStateFn = func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					return nil, errors.New("entity not found")
				}
			},
			wantError:    true,
			wantContains: []string{"Error getting state", "entity not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &EntityHandlers{}
			client := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(client)
			}

			result, err := h.handleGetState(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleGetState() unexpected error = %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleGetState() returned no content")
			}

			content := result.Content[0].Text
			assertContainsAll(t, content, tt.wantContains)
		})
	}
}

func TestEntityHandlers_GetState_Batch(t *testing.T) {
	t.Parallel()

	testStates := []homeassistant.Entity{
		{
			EntityID: "light.living_room",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Living Room Light",
			},
		},
		{
			EntityID: "light.bedroom",
			State:    "off",
			Attributes: map[string]any{
				"friendly_name": "Bedroom Light",
			},
		},
		{
			EntityID: "switch.kitchen",
			State:    "on",
			Attributes: map[string]any{
				"friendly_name": "Kitchen Switch",
			},
		},
	}

	tests := []handlerTestCase{
		{
			name: "multiple entities - natural format",
			args: map[string]any{"entity_ids": []any{"light.living_room", "light.bedroom", "switch.kitchen"}, "format": "natural"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Living Room Light (light.living_room) is on", "Bedroom Light (light.bedroom) is off", "Kitchen Switch (switch.kitchen) is on"},
		},
		{
			name: "multiple entities - json format",
			args: map[string]any{"entity_ids": []any{"light.living_room", "light.bedroom"}, "format": "json"},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"light.living_room", "light.bedroom", "\"state\""},
		},
		{
			name: "entity not found in batch",
			args: map[string]any{"entity_ids": []any{"light.living_room", "light.nonexistent"}},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return testStates, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Living Room Light", "light.nonexistent: not found"},
		},
		{
			name:         "both entity_id and entity_ids provided",
			args:         map[string]any{"entity_id": "light.test", "entity_ids": []any{"light.other"}},
			wantError:    true,
			wantContains: []string{"Cannot specify both", "entity_id", "entity_ids"},
		},
		{
			name:         "invalid entity_ids type",
			args:         map[string]any{"entity_ids": "not-an-array"},
			wantError:    true,
			wantContains: []string{"entity_ids", "array"},
		},
		{
			name: "client error in batch",
			args: map[string]any{"entity_ids": []any{"light.test"}},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return nil, errors.New("connection refused")
				}
			},
			wantError:    true,
			wantContains: []string{"Error", "connection refused"},
		},
	}

	h := NewEntityHandlers()
	runHandlerTestCases(t, tests, h.handleGetState)
}
