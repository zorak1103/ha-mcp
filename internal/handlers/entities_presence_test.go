package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestQueryEntities_Presence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		setupMock    func(*UniversalMockClient)
		wantError    bool
		wantContains []string
	}{
		{
			name: "presence analysis with natural format",
			args: map[string]any{
				"mode":   "presence",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{
							EntityID: "person.john",
							State:    "home",
							Attributes: map[string]any{
								"friendly_name":   "John",
								"device_trackers": []any{"device_tracker.john_phone", "device_tracker.john_watch"},
							},
						},
						{
							EntityID: "person.jane",
							State:    "away",
							Attributes: map[string]any{
								"friendly_name":   "Jane",
								"device_trackers": []any{}, // No trackers
							},
						},
						{
							EntityID: "device_tracker.john_phone",
							State:    "home",
							Attributes: map[string]any{
								"friendly_name": "John's Phone",
							},
						},
						{
							EntityID: "device_tracker.john_watch",
							State:    "home",
							Attributes: map[string]any{
								"friendly_name": "John's Watch",
							},
						},
						{
							EntityID: "device_tracker.guest_wifi",
							State:    "home",
							Attributes: map[string]any{
								"friendly_name": "Guest WiFi",
							},
						},
						{
							EntityID: "device_tracker.old_device",
							State:    "unavailable",
							Attributes: map[string]any{
								"friendly_name": "Old Device",
							},
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Presence Tracking Analysis", "Persons", "Device Trackers", "John", "Jane", "person.john", "person.jane", "device_tracker.guest_wifi"},
		},
		{
			name: "presence analysis with json format",
			args: map[string]any{
				"mode":   "presence",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{
							EntityID: "person.test",
							State:    "home",
							Attributes: map[string]any{
								"friendly_name":   "Test Person",
								"device_trackers": []any{"device_tracker.test_phone"},
							},
						},
						{
							EntityID: "device_tracker.test_phone",
							State:    "home",
							Attributes: map[string]any{
								"friendly_name": "Test Phone",
							},
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"persons", "trackers_without_person", "persons_without_tracker", "statistics"},
		},
		{
			name: "presence analysis with no persons",
			args: map[string]any{
				"mode":   "presence",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetStatesFn = func(_ context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{
						{
							EntityID: "device_tracker.phone",
							State:    "home",
							Attributes: map[string]any{
								"friendly_name": "Phone",
							},
						},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"No persons found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewConsolidatedEntityQueryHandlers()
			client := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(client)
			}

			result, err := h.handleQueryEntities(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleQueryEntities() unexpected error = %v", err)
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
