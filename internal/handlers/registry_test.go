// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

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
