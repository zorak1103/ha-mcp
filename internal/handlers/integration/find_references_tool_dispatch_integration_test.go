//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// FindReferencesToolDispatchTestSuite covers the find_references (#141) and
// manage_dashboard action=find (#143) tools through the real registry +
// handler layer. Both are read-only, but the fixtures they search (a script
// and a dashboard referencing a shared target entity) are created/torn down
// via direct client calls, following the existing *_tool_dispatch pattern.
type FindReferencesToolDispatchTestSuite struct {
	DashboardTestSuite
}

func TestFindReferencesToolDispatch(t *testing.T) {
	suite.Run(t, new(FindReferencesToolDispatchTestSuite))
}

// TestFindReferencesAcrossScriptAndDashboard is a regression test for issue
// #141: a single find_references call should surface a target entity
// referenced from both a script (structured JSON) and a dashboard card
// (nested inside views/cards), which previously required listing/getting
// every config type individually.
func (s *FindReferencesToolDispatchTestSuite) TestFindReferencesAcrossScriptAndDashboard() {
	targetName := GenerateTestID("fr_target")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	scriptID := GenerateTestID("fr_script")
	scriptEntityID := BuildEntityID("script", scriptID)
	urlPath := generateDashboardURLPath("fr-dash")

	var dashboardID string

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
		if dashboardID != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
		}
	})

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": targetName, "initial": false},
	})
	s.Require().NoError(err, "Failed to create target input_boolean")
	_, err = s.WaitForEntity(targetEntityID, 10*time.Second)
	s.Require().NoError(err)

	scriptConfig := homeassistant.ScriptConfig{
		Alias: "Find References Dispatch Test Script",
		Mode:  "single",
		Sequence: []any{
			map[string]any{
				"service": "input_boolean.turn_on",
				"target":  map[string]any{"entity_id": targetEntityID},
			},
		},
	}
	err = s.Client().CreateScript(s.Context(), scriptID, scriptConfig)
	s.Require().NoError(err, "Failed to create script")
	_, err = s.WaitForEntity(scriptEntityID, 10*time.Second)
	s.Require().NoError(err, "Script did not appear")

	requireAdmin := false
	showInSidebar := false
	created, err := s.Client().CreateDashboard(s.Context(), homeassistant.DashboardConfig{
		URLPath:       urlPath,
		Title:         "Find References Dispatch Test Dashboard",
		Icon:          "mdi:view-dashboard",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	})
	s.Require().NoError(err, "Failed to create dashboard")
	dashboardID = created.ID

	dashboardConfig := map[string]any{
		"views": []any{
			map[string]any{
				"title": "Home",
				"cards": []any{
					map[string]any{
						"type": "vertical-stack",
						"cards": []any{
							map[string]any{"type": "entity", "entity": targetEntityID},
						},
					},
				},
			},
		},
	}
	err = s.Client().SaveLovelaceConfig(s.Context(), urlPath, dashboardConfig)
	s.Require().NoError(err, "Failed to save dashboard config")

	// The action under test: find_references via the real tool dispatch layer.
	result := s.CallTool("find_references", map[string]any{
		"search": targetEntityID,
	})
	s.Require().False(result.IsError, "find_references should succeed, got: %s", resultText(result))

	text := resultText(result)
	s.Contains(text, scriptEntityID, "expected the script reference to be found")
	s.Contains(text, urlPath, "expected the dashboard reference to be found")
}

// DashboardFindToolDispatchTestSuite covers manage_dashboard action=find
// (issue #143) through the real registry + handler layer.
type DashboardFindToolDispatchTestSuite struct {
	DashboardTestSuite
}

func TestDashboardFindToolDispatch(t *testing.T) {
	suite.Run(t, new(DashboardFindToolDispatchTestSuite))
}

// TestDashboardFindLocatesDeeplyNestedCard verifies action=find locates an
// entity several card levels deep - the scenario action=get's compact/verbose
// fallbacks cannot handle on a dashboard too large to fetch wholesale.
func (s *DashboardFindToolDispatchTestSuite) TestDashboardFindLocatesDeeplyNestedCard() {
	targetName := GenerateTestID("df_target")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	urlPath := generateDashboardURLPath("df-dash")

	var dashboardID string

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
		if dashboardID != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
		}
	})

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": targetName, "initial": false},
	})
	s.Require().NoError(err, "Failed to create target input_boolean")
	_, err = s.WaitForEntity(targetEntityID, 10*time.Second)
	s.Require().NoError(err)

	requireAdmin := false
	showInSidebar := false
	created, err := s.Client().CreateDashboard(s.Context(), homeassistant.DashboardConfig{
		URLPath:       urlPath,
		Title:         "Dashboard Find Dispatch Test",
		Icon:          "mdi:view-dashboard",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	})
	s.Require().NoError(err, "Failed to create dashboard")
	dashboardID = created.ID

	dashboardConfig := map[string]any{
		"views": []any{
			map[string]any{
				"title": "Home",
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{
								"type": "vertical-stack",
								"cards": []any{
									map[string]any{
										"type": "tile",
										"chips": []any{
											map[string]any{"type": "entity", "entity": targetEntityID},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	err = s.Client().SaveLovelaceConfig(s.Context(), urlPath, dashboardConfig)
	s.Require().NoError(err, "Failed to save dashboard config")

	// The action under test: manage_dashboard action=find via the real tool dispatch layer.
	result := s.CallTool("manage_dashboard", map[string]any{
		"action":   "find",
		"url_path": urlPath,
		"search":   targetEntityID,
	})
	s.Require().False(result.IsError, "manage_dashboard find should succeed, got: %s", resultText(result))

	text := resultText(result)
	s.Contains(text, targetEntityID, "expected the deeply nested chip reference to be found")
	s.Contains(text, "views/0/sections/0/cards/0/cards/0/chips/0/entity",
		"expected the full nested JSON pointer path in the result")
}
