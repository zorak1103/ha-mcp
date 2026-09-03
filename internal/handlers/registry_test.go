// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
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

func TestBuildDeviceIDsInArea(t *testing.T) {
	tests := []struct {
		name            string
		areaID          string
		getDeviceRegFn  func(ctx context.Context) ([]homeassistant.DeviceRegistryEntry, error)
		wantErr         bool
		wantDeviceIDsIn map[string]bool
	}{
		{
			name:   "empty area_id skips client call",
			areaID: "",
			getDeviceRegFn: func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
				t.Fatal("GetDeviceRegistry should not be called when areaID is empty")
				return nil, nil
			},
			wantErr:         false,
			wantDeviceIDsIn: map[string]bool{},
		},
		{
			name:   "device registry error propagates",
			areaID: "area1",
			getDeviceRegFn: func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
				return nil, errors.New("connection refused")
			},
			wantErr:         true,
			wantDeviceIDsIn: map[string]bool{},
		},
		{
			name:   "matching devices populate map",
			areaID: "area1",
			getDeviceRegFn: func(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
				return []homeassistant.DeviceRegistryEntry{
					{ID: "dev1", AreaID: "area1"},
					{ID: "dev2", AreaID: "area2"},
					{ID: "dev3", AreaID: "area1"},
				}, nil
			},
			wantErr:         false,
			wantDeviceIDsIn: map[string]bool{"dev1": true, "dev3": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &entityRegistryFilter{areaID: tt.areaID, deviceIDsInArea: make(map[string]bool)}
			client := &UniversalMockClient{GetDeviceRegistryFn: tt.getDeviceRegFn}

			err := filter.buildDeviceIDsInArea(context.Background(), client)

			if (err != nil) != tt.wantErr {
				t.Fatalf("buildDeviceIDsInArea() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.wantDeviceIDsIn, filter.deviceIDsInArea); diff != "" {
				t.Errorf("deviceIDsInArea mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildEntityRegistryFiltersMap(t *testing.T) {
	tests := []struct {
		name   string
		filter *entityRegistryFilter
		want   map[string]any
	}{
		{
			name:   "empty filter produces empty map",
			filter: &entityRegistryFilter{},
			want:   map[string]any{},
		},
		{
			name: "all fields set",
			filter: &entityRegistryFilter{
				domain:          "light",
				platform:        "hue",
				deviceID:        "dev1",
				areaID:          "area1",
				includeDisabled: true,
			},
			want: map[string]any{
				"domain":           "light",
				"platform":         "hue",
				"device_id":        "dev1",
				"area_id":          "area1",
				"include_disabled": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildEntityRegistryFiltersMap(tt.filter)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("buildEntityRegistryFiltersMap() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildPaginatedEntityRegistryResponse(t *testing.T) {
	tests := []struct {
		name        string
		paginated   PaginatedResponse[homeassistant.EntityRegistryEntry]
		itemsOutput string
		wantJSON    bool
	}{
		{
			name:        "limit zero returns items verbatim",
			paginated:   PaginatedResponse[homeassistant.EntityRegistryEntry]{Pagination: PaginationMetadata{Limit: 0}},
			itemsOutput: "raw entity output",
			wantJSON:    false,
		},
		{
			name: "limit set wraps with pagination metadata",
			paginated: PaginatedResponse[homeassistant.EntityRegistryEntry]{
				Pagination: PaginationMetadata{Limit: 10, Total: 3, Count: 3},
			},
			itemsOutput: `[{"entity_id":"light.one"}]`,
			wantJSON:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPaginatedEntityRegistryResponse(tt.paginated, tt.itemsOutput)

			if !tt.wantJSON {
				if diff := cmp.Diff(tt.itemsOutput, got); diff != "" {
					t.Errorf("buildPaginatedEntityRegistryResponse() mismatch (-want +got):\n%s", diff)
				}
				return
			}

			var response paginatedEntityRegistryResponse
			if err := json.Unmarshal([]byte(got), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if response.Pagination.Limit != tt.paginated.Pagination.Limit {
				t.Errorf("Pagination.Limit = %d, want %d", response.Pagination.Limit, tt.paginated.Pagination.Limit)
			}
			var wantItems, gotItems any
			if err := json.Unmarshal([]byte(tt.itemsOutput), &wantItems); err != nil {
				t.Fatalf("failed to unmarshal want items: %v", err)
			}
			if err := json.Unmarshal(response.Items, &gotItems); err != nil {
				t.Fatalf("failed to unmarshal got items: %v", err)
			}
			if diff := cmp.Diff(wantItems, gotItems); diff != "" {
				t.Errorf("Items mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeviceRegistryFilterMatches(t *testing.T) {
	tests := []struct {
		name   string
		filter deviceRegistryFilter
		entry  homeassistant.DeviceRegistryEntry
		want   bool
	}{
		{
			name:   "empty filter excludes disabled entry",
			filter: deviceRegistryFilter{},
			entry:  homeassistant.DeviceRegistryEntry{ID: "dev1", DisabledBy: "user"},
			want:   false,
		},
		{
			name:   "include_disabled matches disabled entry",
			filter: deviceRegistryFilter{includeDisabled: true},
			entry:  homeassistant.DeviceRegistryEntry{ID: "dev1", DisabledBy: "user"},
			want:   true,
		},
		{
			name:   "area_id no match",
			filter: deviceRegistryFilter{areaID: "area1"},
			entry:  homeassistant.DeviceRegistryEntry{ID: "dev1", AreaID: "area2"},
			want:   false,
		},
		{
			name:   "manufacturer matches case-insensitive substring",
			filter: deviceRegistryFilter{manufacturer: "philips"},
			entry:  homeassistant.DeviceRegistryEntry{ID: "dev1", Manufacturer: "Philips Hue"},
			want:   true,
		},
		{
			name:   "manufacturer no match",
			filter: deviceRegistryFilter{manufacturer: "philips"},
			entry:  homeassistant.DeviceRegistryEntry{ID: "dev1", Manufacturer: "Fritz"},
			want:   false,
		},
		{
			name:   "model matches case-insensitive substring",
			filter: deviceRegistryFilter{model: "hue bulb"},
			entry:  homeassistant.DeviceRegistryEntry{ID: "dev1", Model: "Hue Bulb A19"},
			want:   true,
		},
		{
			name:   "model no match",
			filter: deviceRegistryFilter{model: "hue bulb"},
			entry:  homeassistant.DeviceRegistryEntry{ID: "dev1", Model: "Fritz DECT"},
			want:   false,
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

func TestBuildDeviceRegistryFiltersMap(t *testing.T) {
	tests := []struct {
		name   string
		filter deviceRegistryFilter
		want   map[string]any
	}{
		{
			name:   "empty filter produces empty map",
			filter: deviceRegistryFilter{},
			want:   map[string]any{},
		},
		{
			name: "all fields set",
			filter: deviceRegistryFilter{
				areaID:          "area1",
				manufacturer:    "philips",
				model:           "hue bulb",
				includeDisabled: true,
			},
			want: map[string]any{
				"area_id":          "area1",
				"manufacturer":     "philips",
				"model":            "hue bulb",
				"include_disabled": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDeviceRegistryFiltersMap(tt.filter)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("buildDeviceRegistryFiltersMap() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildPaginatedDeviceRegistryResponse(t *testing.T) {
	tests := []struct {
		name        string
		paginated   PaginatedResponse[homeassistant.DeviceRegistryEntry]
		itemsOutput []byte
		wantJSON    bool
	}{
		{
			name:        "limit zero returns items verbatim",
			paginated:   PaginatedResponse[homeassistant.DeviceRegistryEntry]{Pagination: PaginationMetadata{Limit: 0}},
			itemsOutput: []byte("raw device output"),
			wantJSON:    false,
		},
		{
			name: "limit set wraps with pagination metadata",
			paginated: PaginatedResponse[homeassistant.DeviceRegistryEntry]{
				Pagination: PaginationMetadata{Limit: 10, Total: 2, Count: 2},
			},
			itemsOutput: []byte(`[{"id":"dev1"}]`),
			wantJSON:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPaginatedDeviceRegistryResponse(tt.paginated, tt.itemsOutput)

			if !tt.wantJSON {
				if diff := cmp.Diff(tt.itemsOutput, got); diff != "" {
					t.Errorf("buildPaginatedDeviceRegistryResponse() mismatch (-want +got):\n%s", diff)
				}
				return
			}

			var response paginatedDeviceRegistryResponse
			if err := json.Unmarshal(got, &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if response.Pagination.Limit != tt.paginated.Pagination.Limit {
				t.Errorf("Pagination.Limit = %d, want %d", response.Pagination.Limit, tt.paginated.Pagination.Limit)
			}
			var wantItems, gotItems any
			if err := json.Unmarshal(tt.itemsOutput, &wantItems); err != nil {
				t.Fatalf("failed to unmarshal want items: %v", err)
			}
			if err := json.Unmarshal(response.Items, &gotItems); err != nil {
				t.Fatalf("failed to unmarshal got items: %v", err)
			}
			if diff := cmp.Diff(wantItems, gotItems); diff != "" {
				t.Errorf("Items mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
