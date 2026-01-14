package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockLovelaceClient implements homeassistant.Client for testing.
type mockLovelaceClient struct {
	homeassistant.Client
	getLovelaceConfigFn func(ctx context.Context) (map[string]any, error)
}

func (m *mockLovelaceClient) GetLovelaceConfig(ctx context.Context) (map[string]any, error) {
	if m.getLovelaceConfigFn != nil {
		return m.getLovelaceConfigFn(ctx)
	}
	return map[string]any{}, nil
}

func TestNewLovelaceHandlers(t *testing.T) {
	t.Parallel()

	h := NewLovelaceHandlers()
	if h == nil {
		t.Error("NewLovelaceHandlers() returned nil")
	}
}

func TestLovelaceHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewLovelaceHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 1
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	expectedTools := map[string]bool{
		"get_lovelace_config": false,
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

func TestLovelaceHandlers_getLovelaceConfigTool(t *testing.T) {
	t.Parallel()

	h := NewLovelaceHandlers()
	tool := h.getLovelaceConfigTool()

	if tool.Name != "get_lovelace_config" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "get_lovelace_config")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	// Check optional properties exist
	expectedProps := []string{"view", "verbose"}
	for _, prop := range expectedProps {
		if _, ok := tool.InputSchema.Properties[prop]; !ok {
			t.Errorf("Property %q not found in schema", prop)
		}
	}

	// No required fields for this tool
	if len(tool.InputSchema.Required) != 0 {
		t.Errorf("Expected no required fields, got %d", len(tool.InputSchema.Required))
	}
}

func TestLovelaceHandlers_handleGetLovelaceConfig(t *testing.T) {
	t.Parallel()

	sampleConfig := map[string]any{
		"title": "Home",
		"views": []any{
			map[string]any{
				"title": "Overview",
				"path":  "overview",
				"icon":  "mdi:home",
				"cards": []any{
					map[string]any{"type": "entities"},
					map[string]any{"type": "weather-forecast"},
				},
				"badges": []any{
					map[string]any{"entity": "sensor.temperature"},
				},
			},
			map[string]any{
				"title":   "Lights",
				"path":    "lights",
				"icon":    "mdi:lightbulb",
				"subview": true,
				"cards": []any{
					map[string]any{"type": "light"},
				},
			},
			map[string]any{
				"title": "Climate",
				"path":  "climate",
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{"type": "thermostat"},
							map[string]any{"type": "humidifier"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name                  string
		args                  map[string]any
		getLovelaceConfigErr  error
		getLovelaceConfigResp map[string]any
		wantError             bool
		wantContains          string
		wantNotContains       string
	}{
		{
			name:                  "compact mode default",
			args:                  map[string]any{},
			getLovelaceConfigResp: sampleConfig,
			wantError:             false,
			wantContains:          "Found 3 views",
		},
		{
			name: "verbose mode",
			args: map[string]any{
				"verbose": true,
			},
			getLovelaceConfigResp: sampleConfig,
			wantError:             false,
			wantContains:          "Lovelace configuration with 3 views",
		},
		{
			name: "filter by view - exact match",
			args: map[string]any{
				"view": "overview",
			},
			getLovelaceConfigResp: sampleConfig,
			wantError:             false,
			wantContains:          "Found 1 view(s) matching 'overview'",
		},
		{
			name: "filter by view - partial match",
			args: map[string]any{
				"view": "light",
			},
			getLovelaceConfigResp: sampleConfig,
			wantError:             false,
			wantContains:          "Found 1 view(s) matching 'light'",
		},
		{
			name: "filter by view - case insensitive",
			args: map[string]any{
				"view": "CLIMATE",
			},
			getLovelaceConfigResp: sampleConfig,
			wantError:             false,
			wantContains:          "Found 1 view(s) matching 'CLIMATE'",
		},
		{
			name: "filter by view - no match",
			args: map[string]any{
				"view": "nonexistent",
			},
			getLovelaceConfigResp: sampleConfig,
			wantError:             false,
			wantContains:          "No views found matching 'nonexistent'",
		},
		{
			name:                  "empty config",
			args:                  map[string]any{},
			getLovelaceConfigResp: map[string]any{},
			wantError:             false,
			wantContains:          "Found 0 views",
		},
		{
			name:                  "config with nil views",
			args:                  map[string]any{},
			getLovelaceConfigResp: map[string]any{"title": "Home"},
			wantError:             false,
			wantContains:          "Found 0 views",
		},
		{
			name:                 "client error",
			args:                 map[string]any{},
			getLovelaceConfigErr: errors.New("connection failed"),
			wantError:            true,
			wantContains:         "Error getting Lovelace config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockLovelaceClient{
				getLovelaceConfigFn: func(_ context.Context) (map[string]any, error) {
					if tt.getLovelaceConfigErr != nil {
						return nil, tt.getLovelaceConfigErr
					}
					return tt.getLovelaceConfigResp, nil
				},
			}

			h := NewLovelaceHandlers()
			result, err := h.handleGetLovelaceConfig(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleGetLovelaceConfig() returned unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("handleGetLovelaceConfig() returned nil result")
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

			if tt.wantNotContains != "" && contains(content, tt.wantNotContains) {
				t.Errorf("Content = %q, should not contain %q", content, tt.wantNotContains)
			}
		})
	}
}

func TestFilterViewsByQuery(t *testing.T) {
	t.Parallel()

	views := []any{
		map[string]any{"title": "Overview", "path": "overview"},
		map[string]any{"title": "Lights", "path": "lights"},
		map[string]any{"title": "Climate Control", "path": "climate"},
	}

	tests := []struct {
		name      string
		views     []any
		query     string
		wantCount int
	}{
		{
			name:      "exact match by path",
			views:     views,
			query:     "overview",
			wantCount: 1,
		},
		{
			name:      "partial match by title",
			views:     views,
			query:     "control",
			wantCount: 1,
		},
		{
			name:      "case insensitive match",
			views:     views,
			query:     "LIGHTS",
			wantCount: 1,
		},
		{
			name:      "no match",
			views:     views,
			query:     "nonexistent",
			wantCount: 0,
		},
		{
			name:      "empty query matches all",
			views:     views,
			query:     "",
			wantCount: 3,
		},
		{
			name:      "empty views",
			views:     []any{},
			query:     "test",
			wantCount: 0,
		},
		{
			name:      "invalid view type skipped",
			views:     []any{"not a map", map[string]any{"title": "Valid"}},
			query:     "valid",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := filterViewsByQuery(tt.views, tt.query)
			if len(result) != tt.wantCount {
				t.Errorf("filterViewsByQuery() returned %d views, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestCountCardsInView(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		viewMap map[string]any
		want    int
	}{
		{
			name:    "empty view",
			viewMap: map[string]any{},
			want:    0,
		},
		{
			name: "cards only",
			viewMap: map[string]any{
				"cards": []any{
					map[string]any{"type": "entities"},
					map[string]any{"type": "button"},
				},
			},
			want: 2,
		},
		{
			name: "sections only",
			viewMap: map[string]any{
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{"type": "light"},
						},
					},
					map[string]any{
						"cards": []any{
							map[string]any{"type": "sensor"},
							map[string]any{"type": "weather"},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "cards and sections combined",
			viewMap: map[string]any{
				"cards": []any{
					map[string]any{"type": "entities"},
				},
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{"type": "button"},
							map[string]any{"type": "light"},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "section without cards",
			viewMap: map[string]any{
				"sections": []any{
					map[string]any{"title": "Empty Section"},
				},
			},
			want: 0,
		},
		{
			name: "invalid section type skipped",
			viewMap: map[string]any{
				"sections": []any{
					"not a map",
					map[string]any{
						"cards": []any{map[string]any{"type": "button"}},
					},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := countCardsInView(tt.viewMap)
			if got != tt.want {
				t.Errorf("countCardsInView() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildCompactViewEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		viewMap map[string]any
		want    compactViewEntry
	}{
		{
			name:    "empty view",
			viewMap: map[string]any{},
			want:    compactViewEntry{},
		},
		{
			name: "full view",
			viewMap: map[string]any{
				"title":   "Overview",
				"path":    "overview",
				"icon":    "mdi:home",
				"subview": true,
				"cards":   []any{map[string]any{"type": "entities"}},
				"badges":  []any{map[string]any{"entity": "sensor.temp"}},
			},
			want: compactViewEntry{
				Title:      "Overview",
				Path:       "overview",
				Icon:       "mdi:home",
				Subview:    true,
				CardCount:  1,
				BadgeCount: 1,
			},
		},
		{
			name: "view without badges",
			viewMap: map[string]any{
				"title": "Simple",
				"path":  "simple",
				"cards": []any{map[string]any{"type": "button"}, map[string]any{"type": "light"}},
			},
			want: compactViewEntry{
				Title:     "Simple",
				Path:      "simple",
				CardCount: 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildCompactViewEntry(tt.viewMap)
			if got != tt.want {
				t.Errorf("buildCompactViewEntry() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildCompactViews(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		views []any
		want  int
	}{
		{
			name:  "empty views",
			views: []any{},
			want:  0,
		},
		{
			name: "multiple views",
			views: []any{
				map[string]any{"title": "View1"},
				map[string]any{"title": "View2"},
				map[string]any{"title": "View3"},
			},
			want: 3,
		},
		{
			name: "invalid views skipped",
			views: []any{
				"not a map",
				map[string]any{"title": "Valid"},
				42,
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildCompactViews(tt.views)
			if len(got) != tt.want {
				t.Errorf("buildCompactViews() returned %d entries, want %d", len(got), tt.want)
			}
		})
	}
}

func TestLovelaceHandlers_compactViewEntry(t *testing.T) {
	t.Parallel()

	// Test that compact mode correctly counts cards from both "cards" and "sections"
	configWithSections := map[string]any{
		"views": []any{
			map[string]any{
				"title": "Mixed",
				"path":  "mixed",
				"cards": []any{
					map[string]any{"type": "entities"},
				},
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{"type": "button"},
							map[string]any{"type": "light"},
						},
					},
					map[string]any{
						"cards": []any{
							map[string]any{"type": "sensor"},
						},
					},
				},
				"badges": []any{
					map[string]any{"entity": "sensor.temp"},
					map[string]any{"entity": "sensor.humidity"},
				},
			},
		},
	}

	client := &mockLovelaceClient{
		getLovelaceConfigFn: func(_ context.Context) (map[string]any, error) {
			return configWithSections, nil
		},
	}

	h := NewLovelaceHandlers()
	result, err := h.handleGetLovelaceConfig(context.Background(), client, map[string]any{})

	if err != nil {
		t.Fatalf("handleGetLovelaceConfig() error: %v", err)
	}

	content := result.Content[0].Text

	// Should count: 1 card from "cards" + 2 cards from section 1 + 1 card from section 2 = 4 total
	if !contains(content, `"card_count": 4`) {
		t.Errorf("Expected card_count of 4, got content: %s", content)
	}

	// Should count 2 badges
	if !contains(content, `"badge_count": 2`) {
		t.Errorf("Expected badge_count of 2, got content: %s", content)
	}
}
