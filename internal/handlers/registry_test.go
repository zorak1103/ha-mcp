// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
)

// mockClient implements homeassistant.Client for testing.
type mockRegistryClient struct {
	homeassistant.Client
	entityRegistry []homeassistant.EntityRegistryEntry
}

func (m *mockRegistryClient) GetEntityRegistry(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	return m.entityRegistry, nil
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
