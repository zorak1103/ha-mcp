package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestQueryDevices_Health_Analyze(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         map[string]any
		setupMock    func(*UniversalMockClient)
		wantError    bool
		wantContains []string
	}{
		{
			name: "detect disabled devices",
			args: map[string]any{
				"mode":       "health",
				"action":     "analyze",
				"categories": []any{"disabled"},
				"format":     "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "device1", Name: "Disabled Device", DisabledBy: "user"},
						{ID: "device2", Name: "Enabled Device"},
					}, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Device Health Report", "Disabled", "device1"},
		},
		{
			name: "detect orphaned config entries",
			args: map[string]any{
				"mode":       "health",
				"action":     "analyze",
				"categories": []any{"orphaned_config_entry"},
				"format":     "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{
							ID:            "device1",
							Name:          "Device with Orphaned Entry",
							ConfigEntries: []string{"config_entry_1", "config_entry_nonexistent"},
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{
						{EntryID: "config_entry_1"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Device Health Report", "Orphaned Config Entries", "device1"},
		},
		{
			name: "detect config entry errors",
			args: map[string]any{
				"mode":       "health",
				"action":     "analyze",
				"categories": []any{"config_entry_error"},
				"format":     "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{
							ID:            "device1",
							Name:          "Device with Failed Entry",
							ConfigEntries: []string{"config_entry_1"},
						},
					}, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{
						{EntryID: "config_entry_1", State: "setup_retry"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Device Health Report", "Config Entry Error", "device1", "setup_retry"},
		},
		{
			name: "detect devices with no entities",
			args: map[string]any{
				"mode":       "health",
				"action":     "analyze",
				"categories": []any{"no_entities"},
				"format":     "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "device1", Name: "Device Without Entities"},
						{ID: "device2", Name: "Device With Entities"},
					}, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "sensor.test", DeviceID: "device2"},
					}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Device Health Report", "No Entities", "device1"},
		},
		{
			name: "detect devices with no config entries",
			args: map[string]any{
				"mode":       "health",
				"action":     "analyze",
				"categories": []any{"no_config_entries"},
				"format":     "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "device1", Name: "Device Without Config", ConfigEntries: []string{}},
						{ID: "device2", Name: "Device With Config", ConfigEntries: []string{"entry1"}},
					}, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{
						{EntryID: "entry1"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Device Health Report", "No Config Entries", "device1"},
		},
		{
			name: "multiple categories combined",
			args: map[string]any{
				"mode":       "health",
				"action":     "analyze",
				"categories": []any{"disabled", "no_entities"},
				"format":     "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "device1", Name: "Disabled Device", DisabledBy: "user"},
						{ID: "device2", Name: "Device Without Entities"},
						{ID: "device3", Name: "Healthy Device"},
					}, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "sensor.test", DeviceID: "device3"},
					}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Device Health Report", "Disabled", "device1", "No Entities", "device2"},
		},
		{
			name: "no issues detected",
			args: map[string]any{
				"mode":   "health",
				"action": "analyze",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "device1", Name: "Healthy Device", ConfigEntries: []string{"entry1"}},
					}, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{
						{EntityID: "sensor.test", DeviceID: "device1"},
					}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{
						{EntryID: "entry1", State: "loaded"},
					}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Device Health Report", "No issues detected"},
		},
		{
			name: "manufacturer filter",
			args: map[string]any{
				"mode":         "health",
				"action":       "analyze",
				"manufacturer": "ACME",
				"format":       "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "device1", Name: "ACME Device", Manufacturer: "ACME", DisabledBy: "user"},
						{ID: "device2", Name: "Other Device", Manufacturer: "Other", DisabledBy: "user"},
					}, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{"Device Health Report", "ACME Device", "device1"},
		},
		{
			name: "JSON format output",
			args: map[string]any{
				"mode":       "health",
				"action":     "analyze",
				"categories": []any{"disabled"},
				"format":     "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.GetDeviceRegistryFn = func(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
					return []homeassistant.DeviceRegistryEntry{
						{ID: "device1", Name: "Disabled Device", DisabledBy: "user"},
					}, nil
				}
				m.GetEntityRegistryFn = func(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
					return []homeassistant.EntityRegistryEntry{}, nil
				}
				m.GetConfigEntriesFn = func(_ context.Context, _ string) ([]homeassistant.ConfigEntryFull, error) {
					return []homeassistant.ConfigEntryFull{}, nil
				}
			},
			wantError:    false,
			wantContains: []string{`"issues"`, `"statistics"`, `"category": "disabled"`, `"device_id": "device1"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockClient := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			h := NewDeviceQueryHandlers()
			result, err := h.handleQueryDevices(context.Background(), mockClient, tt.args)

			if tt.wantError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result != nil && len(result.Content) > 0 {
				content := result.Content[0].Text
				for _, want := range tt.wantContains {
					if !strings.Contains(content, want) {
						t.Errorf("output missing expected string %q\nGot: %s", want, content)
					}
				}
			}
		})
	}
}
