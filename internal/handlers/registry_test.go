// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// mockClient implements homeassistant.Client for testing.
type mockRegistryClient struct {
	homeassistant.Client
	entityRegistry []homeassistant.EntityRegistryEntry
	deviceRegistry []homeassistant.DeviceRegistryEntry
}

func (m *mockRegistryClient) GetEntityRegistry(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	return m.entityRegistry, nil
}

func (m *mockRegistryClient) GetDeviceRegistry(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	return m.deviceRegistry, nil
}

func TestHandleListEntityRegistry(t *testing.T) {
	testEntries := []homeassistant.EntityRegistryEntry{
		{EntityID: "switch.test1", Platform: "fritz", DeviceID: "dev1", AreaID: "area1"},
		{EntityID: "switch.test2", Platform: "hue", DeviceID: "dev2", AreaID: "area1"},
		{EntityID: "light.test3", Platform: "hue", DeviceID: "dev2", AreaID: "area2"},
		{EntityID: "sensor.disabled", Platform: "mqtt", DeviceID: "dev3", DisabledBy: "user"},
	}

	tests := []struct {
		name             string
		args             map[string]any
		wantEntityCount  int
		wantContains     []string
		wantNotContains  []string
		wantVerboseField bool
	}{
		{
			name:            "no filters - compact output, excludes disabled",
			args:            map[string]any{},
			wantEntityCount: 3,
			wantContains:    []string{"switch.test1", "switch.test2", "light.test3"},
			wantNotContains: []string{"sensor.disabled", "platform", "unique_id"},
		},
		{
			name:            "domain filter - switch only",
			args:            map[string]any{"domain": "switch"},
			wantEntityCount: 2,
			wantContains:    []string{"switch.test1", "switch.test2"},
			wantNotContains: []string{"light.test3"},
		},
		{
			name:            "platform filter - hue only",
			args:            map[string]any{"platform": "hue"},
			wantEntityCount: 2,
			wantContains:    []string{"switch.test2", "light.test3"},
			wantNotContains: []string{"switch.test1"},
		},
		{
			name:            "device_id filter",
			args:            map[string]any{"device_id": "dev2"},
			wantEntityCount: 2,
			wantContains:    []string{"switch.test2", "light.test3"},
			wantNotContains: []string{"switch.test1"},
		},
		{
			name:            "area_id filter",
			args:            map[string]any{"area_id": "area1"},
			wantEntityCount: 2,
			wantContains:    []string{"switch.test1", "switch.test2"},
			wantNotContains: []string{"light.test3"},
		},
		{
			name:            "include_disabled",
			args:            map[string]any{"include_disabled": true},
			wantEntityCount: 4,
			wantContains:    []string{"sensor.disabled"},
		},
		{
			name:             "verbose mode includes platform",
			args:             map[string]any{"verbose": true, "domain": "switch"},
			wantEntityCount:  2,
			wantContains:     []string{"platform", "fritz", "hue"},
			wantVerboseField: true,
		},
		{
			name:            "combined filters",
			args:            map[string]any{"domain": "switch", "platform": "fritz"},
			wantEntityCount: 1,
			wantContains:    []string{"switch.test1"},
			wantNotContains: []string{"switch.test2", "light.test3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &RegistryHandlers{}
			client := &mockRegistryClient{entityRegistry: testEntries}

			result, err := h.handleListEntityRegistry(context.Background(), client, tt.args)
			if err != nil {
				t.Fatalf("handleListEntityRegistry() error = %v", err)
			}

			if result.IsError {
				t.Fatalf("handleListEntityRegistry() returned error: %v", result.Content)
			}

			if len(result.Content) == 0 {
				t.Fatal("handleListEntityRegistry() returned no content")
			}

			content := result.Content[0].Text

			// Check entity count in summary
			expectedSummary := "Found " + string(rune('0'+tt.wantEntityCount)) + " entities"
			if tt.wantEntityCount >= 10 {
				expectedSummary = "Found " + string(rune('0'+tt.wantEntityCount/10)) + string(rune('0'+tt.wantEntityCount%10)) + " entities"
			}
			if !strings.Contains(content, expectedSummary) {
				// Extract actual count from content for better error message
				t.Errorf("Expected summary %q not found in content:\n%s", expectedSummary, content[:min(200, len(content))])
			}

			// Check contains
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, but it didn't.\nContent: %s", want, content)
				}
			}

			// Check not contains
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q, but it did.\nContent: %s", notWant, content)
				}
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		entityID string
		want     string
	}{
		{"light.living_room", "light"},
		{"switch.kitchen", "switch"},
		{"sensor.temperature", "sensor"},
		{"invalid", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.entityID, func(t *testing.T) {
			got := extractDomain(tt.entityID)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("extractDomain() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCompactEntityEntryOmitsEmpty(t *testing.T) {
	entry := compactEntityEntry{
		EntityID: "light.test",
		DeviceID: "",
		AreaID:   "",
	}

	output, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result := string(output)
	if strings.Contains(result, "device_id") {
		t.Errorf("Expected empty device_id to be omitted, got: %s", result)
	}
	if strings.Contains(result, "area_id") {
		t.Errorf("Expected empty area_id to be omitted, got: %s", result)
	}
	if !strings.Contains(result, "entity_id") {
		t.Errorf("Expected entity_id to be present, got: %s", result)
	}
}

func TestNewEntityRegistryFilterFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want *entityRegistryFilter
	}{
		{
			name: "empty args",
			args: map[string]any{},
			want: &entityRegistryFilter{
				deviceIDsInArea: make(map[string]bool),
			},
		},
		{
			name: "all filters set",
			args: map[string]any{
				"domain":           "light",
				"platform":         "hue",
				"device_id":        "dev1",
				"area_id":          "area1",
				"include_disabled": true,
			},
			want: &entityRegistryFilter{
				domain:          "light",
				platform:        "hue",
				deviceID:        "dev1",
				areaID:          "area1",
				includeDisabled: true,
				deviceIDsInArea: make(map[string]bool),
			},
		},
		{
			name: "wrong types ignored",
			args: map[string]any{
				"domain":           123,
				"include_disabled": "true",
			},
			want: &entityRegistryFilter{
				deviceIDsInArea: make(map[string]bool),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newEntityRegistryFilterFromArgs(tt.args)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(entityRegistryFilter{})); diff != "" {
				t.Errorf("newEntityRegistryFilterFromArgs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEntityRegistryFilterMatches(t *testing.T) {
	tests := []struct {
		name   string
		filter *entityRegistryFilter
		entry  homeassistant.EntityRegistryEntry
		want   bool
	}{
		{
			name:   "empty filter matches enabled entry",
			filter: &entityRegistryFilter{deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test"},
			want:   true,
		},
		{
			name:   "empty filter excludes disabled entry",
			filter: &entityRegistryFilter{deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test", DisabledBy: "user"},
			want:   false,
		},
		{
			name:   "include_disabled matches disabled entry",
			filter: &entityRegistryFilter{includeDisabled: true, deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test", DisabledBy: "user"},
			want:   true,
		},
		{
			name:   "domain filter matches",
			filter: &entityRegistryFilter{domain: "light", deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test"},
			want:   true,
		},
		{
			name:   "domain filter no match",
			filter: &entityRegistryFilter{domain: "switch", deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test"},
			want:   false,
		},
		{
			name:   "platform filter matches",
			filter: &entityRegistryFilter{platform: "hue", deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test", Platform: "hue"},
			want:   true,
		},
		{
			name:   "platform filter no match",
			filter: &entityRegistryFilter{platform: "mqtt", deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test", Platform: "hue"},
			want:   false,
		},
		{
			name:   "device_id filter matches",
			filter: &entityRegistryFilter{deviceID: "dev1", deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test", DeviceID: "dev1"},
			want:   true,
		},
		{
			name:   "device_id filter no match",
			filter: &entityRegistryFilter{deviceID: "dev2", deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test", DeviceID: "dev1"},
			want:   false,
		},
		{
			name:   "area_id direct match",
			filter: &entityRegistryFilter{areaID: "area1", deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test", AreaID: "area1"},
			want:   true,
		},
		{
			name:   "area_id via device match",
			filter: &entityRegistryFilter{areaID: "area1", deviceIDsInArea: map[string]bool{"dev1": true}},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test", DeviceID: "dev1"},
			want:   true,
		},
		{
			name:   "area_id no match",
			filter: &entityRegistryFilter{areaID: "area1", deviceIDsInArea: make(map[string]bool)},
			entry:  homeassistant.EntityRegistryEntry{EntityID: "light.test", AreaID: "area2"},
			want:   false,
		},
		{
			name: "combined filters all match",
			filter: &entityRegistryFilter{
				domain:          "light",
				platform:        "hue",
				deviceIDsInArea: make(map[string]bool),
			},
			entry: homeassistant.EntityRegistryEntry{EntityID: "light.test", Platform: "hue"},
			want:  true,
		},
		{
			name: "combined filters one fails",
			filter: &entityRegistryFilter{
				domain:          "switch",
				platform:        "hue",
				deviceIDsInArea: make(map[string]bool),
			},
			entry: homeassistant.EntityRegistryEntry{EntityID: "light.test", Platform: "hue"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.matches(tt.entry)
			if got != tt.want {
				t.Errorf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntityRegistryFilterFilterEntityRegistry(t *testing.T) {
	entries := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.one", Platform: "hue"},
		{EntityID: "switch.two", Platform: "fritz"},
		{EntityID: "light.three", Platform: "hue", DisabledBy: "user"},
	}

	tests := []struct {
		name      string
		filter    *entityRegistryFilter
		wantCount int
		wantIDs   []string
	}{
		{
			name:      "no filter excludes disabled",
			filter:    &entityRegistryFilter{deviceIDsInArea: make(map[string]bool)},
			wantCount: 2,
			wantIDs:   []string{"light.one", "switch.two"},
		},
		{
			name:      "domain filter",
			filter:    &entityRegistryFilter{domain: "light", deviceIDsInArea: make(map[string]bool)},
			wantCount: 1,
			wantIDs:   []string{"light.one"},
		},
		{
			name:      "include disabled",
			filter:    &entityRegistryFilter{includeDisabled: true, deviceIDsInArea: make(map[string]bool)},
			wantCount: 3,
			wantIDs:   []string{"light.one", "switch.two", "light.three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.filterEntityRegistry(entries)

			if len(got) != tt.wantCount {
				t.Errorf("filterEntityRegistry() returned %d entries, want %d", len(got), tt.wantCount)
			}

			gotIDs := make([]string, len(got))
			for i, e := range got {
				gotIDs[i] = e.EntityID
			}
			if diff := cmp.Diff(tt.wantIDs, gotIDs); diff != "" {
				t.Errorf("filterEntityRegistry() IDs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatEntityRegistryOutput(t *testing.T) {
	entries := []homeassistant.EntityRegistryEntry{
		{EntityID: "light.test", DeviceID: "dev1", AreaID: "area1", Platform: "hue"},
	}

	t.Run("compact output", func(t *testing.T) {
		output, err := formatEntityRegistryOutput(entries, false)
		if err != nil {
			t.Fatalf("formatEntityRegistryOutput() error = %v", err)
		}

		if !strings.Contains(output, "entity_id") {
			t.Error("compact output should contain entity_id")
		}
		if strings.Contains(output, "platform") {
			t.Error("compact output should not contain platform")
		}
	})

	t.Run("verbose output", func(t *testing.T) {
		output, err := formatEntityRegistryOutput(entries, true)
		if err != nil {
			t.Fatalf("formatEntityRegistryOutput() error = %v", err)
		}

		if !strings.Contains(output, "entity_id") {
			t.Error("verbose output should contain entity_id")
		}
		if !strings.Contains(output, "platform") {
			t.Error("verbose output should contain platform")
		}
	})

	t.Run("empty entries", func(t *testing.T) {
		output, err := formatEntityRegistryOutput([]homeassistant.EntityRegistryEntry{}, false)
		if err != nil {
			t.Fatalf("formatEntityRegistryOutput() error = %v", err)
		}

		if output != "[]" {
			t.Errorf("empty entries should return [], got: %s", output)
		}
	})
}
