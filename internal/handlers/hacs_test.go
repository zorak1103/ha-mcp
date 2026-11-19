package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

func TestHACSHandlers_RegisterTools(t *testing.T) {
	t.Parallel()
	registry := mcp.NewRegistry()
	handlers := NewHACSHandlers()
	handlers.RegisterTools(registry)

	tool, found := registry.GetTool("manage_hacs")
	if !found {
		t.Fatal("manage_hacs tool not registered")
	}

	// Verify schema
	schema := tool.InputSchema
	if schema.Type != "object" {
		t.Errorf("expected schema type object, got %s", schema.Type)
	}

	// Verify required fields
	requiredFields := []string{"action"}
	for _, field := range requiredFields {
		found := false
		for _, req := range schema.Required {
			if req == field {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required field %s not found in schema", field)
		}
	}

	// Verify action enum has 12 values
	actionProp := schema.Properties["action"]
	if len(actionProp.Enum) != 12 {
		t.Errorf("expected 12 actions, got %d", len(actionProp.Enum))
	}

	// Verify format enum
	formatProp := schema.Properties["format"]
	if len(formatProp.Enum) != 2 {
		t.Errorf("expected 2 format options, got %d", len(formatProp.Enum))
	}

	// Verify search property exists
	if _, found := schema.Properties["search"]; !found {
		t.Error("search property not found in schema")
	}
}

func TestHACSHandlers_HandleManageHACS(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		// =============================================================================
		// Read Actions - Info
		// =============================================================================
		{
			name: "info_natural_format",
			args: map[string]any{
				"action": "info",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/info" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return map[string]any{
						"version":       "1.34.0",
						"lovelace_mode": "storage",
					}, nil
				}
			},
			wantContains: []string{"Version:", "1.34.0", "Lovelace mode:", "storage"},
		},
		{
			name: "info_json_format",
			args: map[string]any{
				"action": "info",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/info" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return map[string]any{
						"version":       "1.34.0",
						"lovelace_mode": "storage",
					}, nil
				}
			},
			wantContains: []string{`"version"`, `"1.34.0"`, `"lovelace_mode"`, `"storage"`},
		},

		// =============================================================================
		// Read Actions - List
		// =============================================================================
		{
			name: "list_all_natural",
			args: map[string]any{
				"action": "list",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/repositories/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return []any{
						map[string]any{
							"id":        "123456",
							"name":      "hacs-frontend",
							"category":  "integration",
							"installed": true,
							"status":    "installed",
						},
					}, nil
				}
			},
			wantContains: []string{"hacs-frontend", "integration", "installed"},
		},
		{
			name: "list_installed_only",
			args: map[string]any{
				"action":         "list",
				"installed_only": true,
				"format":         "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, _ string, data map[string]any) (any, error) {
					// Verify filter is NOT passed to API (client-side filter)
					if _, exists := data["installed_only"]; exists {
						return nil, fmt.Errorf("installed_only should not be sent to API")
					}
					return []any{
						map[string]any{
							"id":        "1",
							"name":      "installed-repo",
							"installed": true,
						},
						map[string]any{
							"id":        "2",
							"name":      "not-installed-repo",
							"installed": false,
						},
					}, nil
				}
			},
			wantContains:    []string{"installed-repo"},
			wantNotContains: []string{"not-installed-repo"},
		},
		{
			name: "list_pending_update",
			args: map[string]any{
				"action":         "list",
				"pending_update": true,
				"format":         "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, _ string, data map[string]any) (any, error) {
					// Verify filter is NOT passed to API (client-side filter)
					if _, exists := data["pending_update"]; exists {
						return nil, fmt.Errorf("pending_update should not be sent to API")
					}
					return []any{
						map[string]any{
							"id":              "1",
							"name":            "pending-upgrade-repo",
							"pending_upgrade": true,
						},
						map[string]any{
							"id":              "2",
							"name":            "no-pending-repo",
							"pending_upgrade": false,
						},
					}, nil
				}
			},
			wantContains:    []string{"pending-upgrade-repo"},
			wantNotContains: []string{"no-pending-repo"},
		},
		{
			name: "list_category_filter",
			args: map[string]any{
				"action":   "list",
				"category": "plugin",
				"format":   "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, _ string, data map[string]any) (any, error) {
					// Verify category is NOT sent to API (client-side filter only)
					if _, exists := data["category"]; exists {
						return nil, fmt.Errorf("category should not be sent to API - client-side filter only")
					}
					// Return repos with mixed categories
					return []any{
						map[string]any{
							"id":       "1",
							"name":     "plugin-repo",
							"category": "plugin",
						},
						map[string]any{
							"id":       "2",
							"name":     "integration-repo",
							"category": "integration",
						},
					}, nil
				}
			},
			wantContains:    []string{"plugin-repo"},
			wantNotContains: []string{"integration-repo"},
		},
		{
			name: "list_search_filter",
			args: map[string]any{
				"action": "list",
				"search": "TRV",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/repositories/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return []any{
						map[string]any{
							"id":          "1",
							"name":        "Better TRV Controls",
							"category":    "integration",
							"description": "Advanced heating control",
						},
						map[string]any{
							"id":          "2",
							"name":        "Lovelace Card",
							"category":    "plugin",
							"description": "Custom dashboard card",
						},
					}, nil
				}
			},
			wantContains:    []string{"Better TRV Controls"},
			wantNotContains: []string{"Lovelace Card"},
		},
		{
			name: "list_search_by_description",
			args: map[string]any{
				"action": "list",
				"search": "heating",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/repositories/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return []any{
						map[string]any{
							"id":          "1",
							"name":        "TRV Controller",
							"category":    "integration",
							"description": "Smart heating management",
						},
						map[string]any{
							"id":          "2",
							"name":        "Weather Card",
							"category":    "plugin",
							"description": "Display weather info",
						},
					}, nil
				}
			},
			wantContains:    []string{"TRV Controller"},
			wantNotContains: []string{"Weather Card"},
		},
		{
			name: "list_combined_filters",
			args: map[string]any{
				"action":   "list",
				"category": "integration",
				"search":   "climate",
				"format":   "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/repositories/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return []any{
						map[string]any{
							"id":          "1",
							"name":        "Climate Controller",
							"category":    "integration",
							"description": "Climate control",
						},
						map[string]any{
							"id":          "2",
							"name":        "Climate Card",
							"category":    "plugin",
							"description": "Display climate info",
						},
						map[string]any{
							"id":          "3",
							"name":        "Light Control",
							"category":    "integration",
							"description": "Smart lighting",
						},
					}, nil
				}
			},
			wantContains:    []string{"Climate Controller"},
			wantNotContains: []string{"Climate Card", "Light Control"},
		},
		{
			name: "list_no_results_after_filter",
			args: map[string]any{
				"action":   "list",
				"category": "theme",
				"format":   "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/repositories/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return []any{
						map[string]any{
							"id":       "1",
							"name":     "plugin-repo",
							"category": "plugin",
						},
					}, nil
				}
			},
			wantContains: []string{"No repositories found"},
		},
		{
			name: "list_combined_server_and_client_filters",
			args: map[string]any{
				"action":         "list",
				"installed_only": true,
				"category":       "integration",
				"format":         "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, _ string, data map[string]any) (any, error) {
					// Verify NO filters are sent to API (all are client-side)
					if _, exists := data["installed_only"]; exists {
						return nil, fmt.Errorf("installed_only should not be sent to API")
					}
					if _, exists := data["category"]; exists {
						return nil, fmt.Errorf("category should not be sent to API")
					}
					return []any{
						map[string]any{
							"id":        "1",
							"name":      "installed-integration",
							"category":  "integration",
							"installed": true,
						},
						map[string]any{
							"id":        "2",
							"name":      "installed-plugin",
							"category":  "plugin",
							"installed": true,
						},
						map[string]any{
							"id":        "3",
							"name":      "not-installed-integration",
							"category":  "integration",
							"installed": false,
						},
					}, nil
				}
			},
			wantContains:    []string{"installed-integration"},
			wantNotContains: []string{"installed-plugin", "not-installed-integration"},
		},

		// =============================================================================
		// Read Actions - Get, Releases, Release Notes, Critical
		// =============================================================================
		{
			name: "get_repository_natural",
			args: map[string]any{
				"action":        "get",
				"repository_id": "123456",
				"format":        "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, data map[string]any) (any, error) {
					if cmd != "hacs/repository/info" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					if data["repository_id"] != "123456" {
						return nil, fmt.Errorf("data[repository_id] = %v, want %q", data["repository_id"], "123456")
					}
					return map[string]any{
						"name":     "hacs-frontend",
						"category": "integration",
						"status":   "installed",
					}, nil
				}
			},
			wantContains: []string{"hacs-frontend", "integration", "installed"},
		},
		{
			name: "releases_json",
			args: map[string]any{
				"action":        "releases",
				"repository_id": "123456",
				"format":        "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, data map[string]any) (any, error) {
					if cmd != "hacs/repository/releases" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					if data["repository_id"] != "123456" {
						return nil, fmt.Errorf("data[repository_id] = %v, want %q", data["repository_id"], "123456")
					}
					return []any{
						map[string]any{"tag": "1.0.0"},
					}, nil
				}
			},
			wantContains: []string{`"tag"`, `"1.0.0"`},
		},
		{
			name: "release_notes_natural",
			args: map[string]any{
				"action":        "release_notes",
				"repository_id": "123456",
				"format":        "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, data map[string]any) (any, error) {
					if cmd != "hacs/repository/release_notes" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					if data["repository_id"] != "123456" {
						return nil, fmt.Errorf("data[repository_id] = %v, want %q", data["repository_id"], "123456")
					}
					return map[string]any{
						"tag":  "1.0.0",
						"body": "Bug fixes",
					}, nil
				}
			},
			wantContains: []string{"1.0.0", "Bug fixes"},
		},
		{
			name: "critical_list",
			args: map[string]any{
				"action": "critical",
				"format": "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/critical/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return []any{}, nil
				}
			},
			wantContains: []string{"[]"},
		},

		// =============================================================================
		// Write Actions - Download, Uninstall, Add, Remove, Refresh, Toggle Beta
		// =============================================================================
		{
			name: "download_repository",
			args: map[string]any{
				"action":        "download",
				"repository_id": "123456",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, command string, data map[string]any) (any, error) {
					if command != "hacs/repository/download" {
						return nil, fmt.Errorf("wrong command: %s", command)
					}
					if data["repository"] != "123456" {
						return nil, fmt.Errorf("expected repository field, got: %v", data)
					}
					return map[string]any{"success": true}, nil
				}
			},
			wantContains: []string{"downloaded", "123456"},
		},
		{
			name: "download_with_version",
			args: map[string]any{
				"action":        "download",
				"repository_id": "123456",
				"version":       "1.0.0",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, _ string, data map[string]any) (any, error) {
					if data["version"] != "1.0.0" {
						return nil, fmt.Errorf("expected version parameter")
					}
					return map[string]any{"success": true}, nil
				}
			},
			wantContains: []string{"downloaded", "123456", "1.0.0"},
		},
		{
			name: "uninstall_repository",
			args: map[string]any{
				"action":        "uninstall",
				"repository_id": "123456",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, command string, _ map[string]any) (any, error) {
					if command != "hacs/repository/remove" {
						return nil, fmt.Errorf("wrong command: %s", command)
					}
					return map[string]any{"success": true}, nil
				}
			},
			wantContains: []string{"uninstalled", "123456"},
		},
		{
			name: "add_repository",
			args: map[string]any{
				"action":     "add_repository",
				"repository": "owner/repo",
				"category":   "integration",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, command string, data map[string]any) (any, error) {
					if command != "hacs/repositories/add" {
						return nil, fmt.Errorf("wrong command: %s", command)
					}
					if data["repository"] != "owner/repo" || data["category"] != "integration" {
						return nil, fmt.Errorf("missing required parameters")
					}
					return map[string]any{"success": true}, nil
				}
			},
			wantContains: []string{"added", "owner/repo"},
		},
		{
			name: "remove_repository",
			args: map[string]any{
				"action":        "remove_repository",
				"repository_id": "123456",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, command string, _ map[string]any) (any, error) {
					if command != "hacs/repositories/remove" {
						return nil, fmt.Errorf("wrong command: %s", command)
					}
					return map[string]any{"success": true}, nil
				}
			},
			wantContains: []string{"removed", "123456"},
		},
		{
			name: "refresh_repository",
			args: map[string]any{
				"action":        "refresh",
				"repository_id": "123456",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, command string, _ map[string]any) (any, error) {
					if command != "hacs/repository/refresh" {
						return nil, fmt.Errorf("wrong command: %s", command)
					}
					return map[string]any{"success": true}, nil
				}
			},
			wantContains: []string{"refreshed", "123456"},
		},
		{
			name: "toggle_beta_enable",
			args: map[string]any{
				"action":        "toggle_beta",
				"repository_id": "123456",
				"show_beta":     true,
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, command string, data map[string]any) (any, error) {
					if command != "hacs/repository/beta" {
						return nil, fmt.Errorf("wrong command: %s", command)
					}
					if data["show_beta"] != true {
						return nil, fmt.Errorf("expected show_beta parameter")
					}
					return map[string]any{"success": true}, nil
				}
			},
			wantContains: []string{"beta", "enabled", "123456"},
		},

		// =============================================================================
		// Error Cases
		// =============================================================================
		{
			name:         "missing_action",
			args:         map[string]any{},
			wantContains: []string{"action", "required"},
		},
		{
			name: "invalid_action",
			args: map[string]any{
				"action": "invalid",
			},
			wantContains: []string{"invalid", "action"},
		},
		{
			name: "get_missing_repository_id",
			args: map[string]any{
				"action": "get",
			},
			wantContains: []string{"repository_id", "required"},
		},
		{
			name: "add_missing_repository",
			args: map[string]any{
				"action":   "add_repository",
				"category": "integration",
			},
			wantContains: []string{"repository", "required"},
		},
		{
			name: "add_missing_category",
			args: map[string]any{
				"action":     "add_repository",
				"repository": "owner/repo",
			},
			wantContains: []string{"category", "required"},
		},
		{
			name: "toggle_beta_missing_show_beta",
			args: map[string]any{
				"action":        "toggle_beta",
				"repository_id": "123456",
			},
			wantContains: []string{"show_beta", "required"},
		},
		{
			name: "hacs_not_installed",
			args: map[string]any{
				"action": "info",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/info" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return nil, fmt.Errorf("unknown_command")
				}
			},
			wantContains: []string{"HACS", "not installed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			mock := &UniversalMockClient{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			handlers := NewHACSHandlers()
			result, err := handlers.HandleManageHACS(ctx, mock, tt.args)

			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}

			resultText := getResultText(t, result)
			for _, want := range tt.wantContains {
				if !containsText(resultText, want) {
					t.Errorf("result does not contain %q:\n%s", want, resultText)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if containsText(resultText, notWant) {
					t.Errorf("result should not contain %q:\n%s", notWant, resultText)
				}
			}
		})
	}
}

func TestHACSHandlers_FormatOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		format       string
		data         any
		wantContains []string
	}{
		{
			name:   "natural_format_map",
			format: "natural",
			data: map[string]any{
				"version": "1.34.0",
				"status":  "ready",
			},
			wantContains: []string{"Version:", "1.34.0", "Status:", "ready"},
		},
		{
			name:   "json_format_map",
			format: "json",
			data: map[string]any{
				"version": "1.34.0",
			},
			wantContains: []string{`"version"`, `"1.34.0"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handlers := NewHACSHandlers()

			// Call a private formatter method through HandleManageHACS
			ctx := context.Background()
			mock := &UniversalMockClient{
				SendHACSCommandFn: func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/info" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return tt.data, nil
				},
			}

			result, err := handlers.HandleManageHACS(ctx, mock, map[string]any{
				"action": "info",
				"format": tt.format,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resultText := getResultText(t, result)
			for _, want := range tt.wantContains {
				if !containsText(resultText, want) {
					t.Errorf("result does not contain %q:\n%s", want, resultText)
				}
			}
		})
	}
}

// Helper to extract text from MCP result.
func getResultText(t *testing.T, result *mcp.ToolsCallResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}

// Helper to check if text contains substring (case-insensitive).
func containsText(text, substr string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(substr))
}

// TestHACSHandlers_GetReleasesAndCritical covers formatReleases, formatCritical and related error paths.
func TestHACSHandlers_GetReleasesAndCritical(t *testing.T) {
	t.Parallel()

	tests := []handlerTestCase{
		{
			name: "get_natural_format",
			args: map[string]any{
				"action":        "get",
				"repository_id": "repo123",
				"format":        "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/repository/info" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return map[string]any{
						"name":     "my-integration",
						"category": "integration",
						"status":   "installed",
					}, nil
				}
			},
			wantContains: []string{"my-integration"},
		},
		{
			name: "get_json_format",
			args: map[string]any{
				"action":        "get",
				"repository_id": "repo123",
				"format":        "json",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/repository/info" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return map[string]any{
						"name": "my-integration",
					}, nil
				}
			},
			wantContains: []string{`"name"`},
		},
		{
			name: "get_error",
			args: map[string]any{
				"action":        "get",
				"repository_id": "repo123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(context.Context, string, map[string]any) (any, error) {
					return nil, fmt.Errorf("unknown_command")
				}
			},
			wantError:    true,
			wantContains: []string{"HACS is not installed"},
		},
		{
			name: "releases_natural_with_releases",
			args: map[string]any{
				"action":        "releases",
				"repository_id": "repo123",
				"format":        "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/repository/releases" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return []any{
						map[string]any{"tag": "v1.0.0"},
						map[string]any{"tag": "v0.9.0"},
					}, nil
				}
			},
			wantContains: []string{"v1.0.0", "v0.9.0"},
		},
		{
			name: "releases_error",
			args: map[string]any{
				"action":        "releases",
				"repository_id": "repo123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(context.Context, string, map[string]any) (any, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			wantError:    true,
			wantContains: []string{"HACS operation failed"},
		},
		{
			name: "critical_natural_with_items",
			args: map[string]any{
				"action": "critical",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/critical/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return []any{
						map[string]any{"repository": "bad-integration/repo"},
					}, nil
				}
			},
			wantContains: []string{"bad-integration/repo"},
		},
		{
			name: "critical_empty",
			args: map[string]any{
				"action": "critical",
				"format": "natural",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(_ context.Context, cmd string, _ map[string]any) (any, error) {
					if cmd != "hacs/critical/list" {
						return nil, fmt.Errorf("wrong command: %s", cmd)
					}
					return []any{}, nil
				}
			},
			wantContains: []string{"No critical"},
		},
		{
			name: "uninstall_error",
			args: map[string]any{
				"action":        "uninstall",
				"repository_id": "repo123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(context.Context, string, map[string]any) (any, error) {
					return nil, fmt.Errorf("operation failed")
				}
			},
			wantError:    true,
			wantContains: []string{"HACS operation failed"},
		},
		{
			name: "remove_repository_error",
			args: map[string]any{
				"action":        "remove_repository",
				"repository_id": "repo123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(context.Context, string, map[string]any) (any, error) {
					return nil, fmt.Errorf("operation failed")
				}
			},
			wantError:    true,
			wantContains: []string{"HACS operation failed"},
		},
		{
			name: "refresh_error",
			args: map[string]any{
				"action":        "refresh",
				"repository_id": "repo123",
			},
			setupMock: func(m *UniversalMockClient) {
				m.SendHACSCommandFn = func(context.Context, string, map[string]any) (any, error) {
					return nil, fmt.Errorf("operation failed")
				}
			},
			wantError:    true,
			wantContains: []string{"HACS operation failed"},
		},
	}

	h := NewHACSHandlers()
	runHandlerTestCases(t, tests, func(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
		return h.HandleManageHACS(ctx, client, args)
	})
}

func TestHACSList_RejectsNonStringSearch(t *testing.T) {
	t.Parallel()

	runFilterTypeCases(t, NewHACSHandlers().HandleManageHACS, []filterTypeCase{
		{
			name: "numeric search",
			args: map[string]any{"action": "list", "search": float64(1)},
			want: "search must be a string, got float64",
			setupMock: func(t *testing.T, c *UniversalMockClient) {
				t.Helper()
				c.SendHACSCommandFn = func(context.Context, string, map[string]any) (any, error) {
					t.Errorf("HA must not be called for an invalid filter")
					return []any{}, nil
				}
			},
		},
		{
			name: "empty search means no filter",
			args: map[string]any{"action": "list", "search": ""},
			setupMock: func(_ *testing.T, c *UniversalMockClient) {
				c.SendHACSCommandFn = func(context.Context, string, map[string]any) (any, error) {
					return []any{}, nil
				}
			},
		},
	})
}
