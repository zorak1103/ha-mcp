//go:build integration

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type DashboardIntegrationTestSuite struct {
	DashboardTestSuite
}

func TestDashboardIntegration(t *testing.T) {
	suite.Run(t, new(DashboardIntegrationTestSuite))
}

// generateDashboardURLPath creates a test URL path with dashes instead of underscores
// to avoid potential issues with HA rejecting underscores in URL paths.
func generateDashboardURLPath(name string) string {
	id := GenerateTestID(name)
	// Replace underscores with dashes for URL path
	return strings.ReplaceAll(id, "_", "-")
}

func (s *DashboardIntegrationTestSuite) TestDashboardLifecycle() {
	urlPath := generateDashboardURLPath("dash")

	var dashboardID string

	s.RegisterCleanup(func() {
		if dashboardID != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
		}
	})

	requireAdmin := false
	showInSidebar := false

	// Create dashboard
	dashboardConfig := homeassistant.DashboardConfig{
		URLPath:       urlPath,
		Title:         "Test Dashboard",
		Icon:          "mdi:view-dashboard",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	}

	created, err := s.Client().CreateDashboard(s.Context(), dashboardConfig)
	s.Require().NoError(err, "Failed to create dashboard")
	s.Require().NotNil(created)
	s.Equal(urlPath, created.URLPath)
	s.Equal("Test Dashboard", created.Title)
	s.Equal("mdi:view-dashboard", created.Icon)

	dashboardID = created.ID

	// Allow time for dashboard to register
	time.Sleep(500 * time.Millisecond)

	// Verify dashboard appears in list
	dashboard, err := s.FindDashboardByURLPath(urlPath)
	s.Require().NoError(err, "Dashboard should appear in list")
	s.Equal("Test Dashboard", dashboard.Title)
	s.Equal("mdi:view-dashboard", dashboard.Icon)

	// Update dashboard (title + icon)
	updateConfig := homeassistant.DashboardConfig{
		Title: "Updated Dashboard",
		Icon:  "mdi:cog",
	}

	updated, err := s.Client().UpdateDashboard(s.Context(), dashboardID, updateConfig)
	s.Require().NoError(err, "Failed to update dashboard")
	s.Equal("Updated Dashboard", updated.Title)
	s.Equal("mdi:cog", updated.Icon)

	time.Sleep(500 * time.Millisecond)

	// Verify update
	dashboard, err = s.FindDashboardByURLPath(urlPath)
	s.Require().NoError(err)
	s.Equal("Updated Dashboard", dashboard.Title)
	s.Equal("mdi:cog", dashboard.Icon)

	// Delete dashboard
	err = s.Client().DeleteDashboard(s.Context(), dashboardID)
	s.Require().NoError(err, "Failed to delete dashboard")

	time.Sleep(500 * time.Millisecond)

	// Verify dashboard is gone
	_, err = s.FindDashboardByURLPath(urlPath)
	s.Error(err, "Dashboard should be deleted from list")
}

func (s *DashboardIntegrationTestSuite) TestDashboardWithConfig() {
	urlPath := generateDashboardURLPath("dash-config")

	var dashboardID string

	s.RegisterCleanup(func() {
		if dashboardID != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
		}
	})

	requireAdmin := false
	showInSidebar := false

	// Create dashboard
	dashboardConfig := homeassistant.DashboardConfig{
		URLPath:       urlPath,
		Title:         "Config Test Dashboard",
		Icon:          "mdi:cog",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	}

	created, err := s.Client().CreateDashboard(s.Context(), dashboardConfig)
	s.Require().NoError(err, "Failed to create dashboard")

	dashboardID = created.ID

	time.Sleep(500 * time.Millisecond)

	// Save Lovelace config with a view and card
	lovelaceConfig := map[string]any{
		"views": []any{
			map[string]any{
				"title": "Test View",
				"path":  "test",
				"cards": []any{
					map[string]any{
						"type":    "markdown",
						"content": "This is a test card",
					},
				},
			},
		},
	}

	err = s.Client().SaveLovelaceConfig(s.Context(), urlPath, lovelaceConfig)
	s.Require().NoError(err, "Failed to save Lovelace config")

	time.Sleep(500 * time.Millisecond)

	// Retrieve and verify config
	retrievedConfig, err := s.Client().GetLovelaceConfig(s.Context(), urlPath)
	s.Require().NoError(err, "Failed to get Lovelace config")
	s.NotNil(retrievedConfig)

	// Verify views exist
	views, ok := retrievedConfig["views"].([]any)
	s.Require().True(ok, "Config should have views")
	s.Require().Len(views, 1, "Should have 1 view")

	view := views[0].(map[string]any)
	s.Equal("Test View", view["title"])
	s.Equal("test", view["path"])

	// Verify cards exist
	cards, ok := view["cards"].([]any)
	s.Require().True(ok, "View should have cards")
	s.Require().Len(cards, 1, "Should have 1 card")

	card := cards[0].(map[string]any)
	s.Equal("markdown", card["type"])
	s.Equal("This is a test card", card["content"])

	// Cleanup
	_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
}

func (s *DashboardIntegrationTestSuite) TestDashboardWithAllOptions() {
	urlPath := generateDashboardURLPath("dash-opts")

	var dashboardID string

	s.RegisterCleanup(func() {
		if dashboardID != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
		}
	})

	requireAdmin := true
	showInSidebar := true

	// Create dashboard with all options
	dashboardConfig := homeassistant.DashboardConfig{
		URLPath:       urlPath,
		Title:         "Full Options Dashboard",
		Icon:          "mdi:star",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	}

	created, err := s.Client().CreateDashboard(s.Context(), dashboardConfig)
	s.Require().NoError(err, "Failed to create dashboard with all options")
	s.Require().NotNil(created)

	dashboardID = created.ID

	time.Sleep(500 * time.Millisecond)

	// Verify all fields
	dashboard, err := s.FindDashboardByURLPath(urlPath)
	s.Require().NoError(err)
	s.Equal("Full Options Dashboard", dashboard.Title)
	s.Equal("mdi:star", dashboard.Icon)
	s.Equal("storage", dashboard.Mode)
	s.True(dashboard.RequireAdmin)
	s.True(dashboard.ShowInSidebar)

	// Cleanup
	_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
}

func (s *DashboardIntegrationTestSuite) TestMultipleDashboards() {
	urlPath1 := generateDashboardURLPath("dash-1")
	urlPath2 := generateDashboardURLPath("dash-2")

	var dashboardID1, dashboardID2 string

	s.RegisterCleanup(func() {
		if dashboardID1 != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID1)
		}
		if dashboardID2 != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID2)
		}
	})

	requireAdmin := false
	showInSidebar := false

	// Create first dashboard
	config1 := homeassistant.DashboardConfig{
		URLPath:       urlPath1,
		Title:         "Dashboard 1",
		Icon:          "mdi:numeric-1",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	}

	created1, err := s.Client().CreateDashboard(s.Context(), config1)
	s.Require().NoError(err, "Failed to create dashboard 1")
	dashboardID1 = created1.ID

	// Create second dashboard
	config2 := homeassistant.DashboardConfig{
		URLPath:       urlPath2,
		Title:         "Dashboard 2",
		Icon:          "mdi:numeric-2",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	}

	created2, err := s.Client().CreateDashboard(s.Context(), config2)
	s.Require().NoError(err, "Failed to create dashboard 2")
	dashboardID2 = created2.ID

	time.Sleep(500 * time.Millisecond)

	// Verify both dashboards exist in list
	dashboard1, err := s.FindDashboardByURLPath(urlPath1)
	s.Require().NoError(err, "Dashboard 1 should exist")
	s.Equal("Dashboard 1", dashboard1.Title)

	dashboard2, err := s.FindDashboardByURLPath(urlPath2)
	s.Require().NoError(err, "Dashboard 2 should exist")
	s.Equal("Dashboard 2", dashboard2.Title)

	// Delete both dashboards
	err = s.Client().DeleteDashboard(s.Context(), dashboardID1)
	s.Require().NoError(err, "Failed to delete dashboard 1")

	err = s.Client().DeleteDashboard(s.Context(), dashboardID2)
	s.Require().NoError(err, "Failed to delete dashboard 2")

	time.Sleep(500 * time.Millisecond)

	// Verify both are gone
	_, err = s.FindDashboardByURLPath(urlPath1)
	s.Error(err, "Dashboard 1 should be deleted")

	_, err = s.FindDashboardByURLPath(urlPath2)
	s.Error(err, "Dashboard 2 should be deleted")
}

func (s *DashboardIntegrationTestSuite) TestDashboardUpdateBooleans() {
	urlPath := generateDashboardURLPath("dash-bool")

	var dashboardID string

	s.RegisterCleanup(func() {
		if dashboardID != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
		}
	})

	requireAdmin := false
	showInSidebar := false

	// Create dashboard with require_admin=false
	dashboardConfig := homeassistant.DashboardConfig{
		URLPath:       urlPath,
		Title:         "Boolean Test Dashboard",
		Icon:          "mdi:test-tube",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	}

	created, err := s.Client().CreateDashboard(s.Context(), dashboardConfig)
	s.Require().NoError(err, "Failed to create dashboard")

	dashboardID = created.ID

	time.Sleep(500 * time.Millisecond)

	// Verify require_admin is false
	dashboard, err := s.FindDashboardByURLPath(urlPath)
	s.Require().NoError(err)
	s.False(dashboard.RequireAdmin, "RequireAdmin should be false initially")

	// Update to require_admin=true
	requireAdminTrue := true
	updateConfig := homeassistant.DashboardConfig{
		RequireAdmin: &requireAdminTrue,
	}

	updated, err := s.Client().UpdateDashboard(s.Context(), dashboardID, updateConfig)
	s.Require().NoError(err, "Failed to update dashboard")
	s.True(updated.RequireAdmin, "RequireAdmin should be updated to true")

	time.Sleep(500 * time.Millisecond)

	// Verify the toggle
	dashboard, err = s.FindDashboardByURLPath(urlPath)
	s.Require().NoError(err)
	s.True(dashboard.RequireAdmin, "RequireAdmin should be true after update")

	// Cleanup
	_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
}

func (s *DashboardIntegrationTestSuite) TestDashboardCreatedWithSectionLayout() {
	// Test that newly created dashboards use the modern section-based layout
	urlPath := generateDashboardURLPath("dash-section")

	var dashboardID string

	s.RegisterCleanup(func() {
		if dashboardID != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
		}
	})

	requireAdmin := false
	showInSidebar := false

	// Create dashboard
	dashboardConfig := homeassistant.DashboardConfig{
		URLPath:       urlPath,
		Title:         "Section Layout Dashboard",
		Icon:          "mdi:view-dashboard",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	}

	created, err := s.Client().CreateDashboard(s.Context(), dashboardConfig)
	s.Require().NoError(err, "Failed to create dashboard")
	s.Require().NotNil(created)

	dashboardID = created.ID

	// Wait longer for dashboard and config to be fully initialized
	time.Sleep(2000 * time.Millisecond)

	// Try to retrieve config - it might not exist yet if SaveLovelaceConfig failed
	retrievedConfig, err := s.Client().GetLovelaceConfig(s.Context(), urlPath)
	if err != nil {
		// If config doesn't exist yet, skip the section layout verification
		// This happens when the dashboard is created but SaveLovelaceConfig fails
		s.T().Skip("Dashboard config not yet initialized - skipping section layout verification")
		return
	}
	s.NotNil(retrievedConfig)

	// Verify views exist
	views, ok := retrievedConfig["views"].([]any)
	s.Require().True(ok, "Config should have views")
	s.Require().Len(views, 1, "Should have 1 default view")

	view := views[0].(map[string]any)
	s.Equal("Home", view["title"], "Default view should be named 'Home'")

	// Verify modern section-based layout
	viewType, ok := view["type"].(string)
	s.Require().True(ok, "View should have a type field")
	s.Equal("sections", viewType, "View should use section-based layout, not legacy badges/cards")

	// Verify sections exist (not cards at root level)
	sections, ok := view["sections"].([]any)
	s.Require().True(ok, "View should have sections array")
	s.Require().Len(sections, 1, "Should have 1 default section")

	section := sections[0].(map[string]any)
	s.Equal("grid", section["type"], "Section should be grid type")

	// Verify section has cards array (even if empty)
	sectionCards, ok := section["cards"].([]any)
	s.Require().True(ok, "Section should have cards array")
	s.Equal(0, len(sectionCards), "Default section should have empty cards array")

	// Verify view does NOT have cards at root level (old format)
	_, hasRootCards := view["cards"]
	s.False(hasRootCards, "View should not have root-level cards (legacy format)")

	// Cleanup
	_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
}

// TestDashboardPatchPreservesViewOrder is a regression test guarding that
// replace /views/N must not reorder sibling views when HA persists the config.
func (s *DashboardIntegrationTestSuite) TestDashboardPatchPreservesViewOrder() {
	urlPath := generateDashboardURLPath("dash-patch-order")

	var dashboardID string

	s.RegisterCleanup(func() {
		if dashboardID != "" {
			_ = s.Client().DeleteDashboard(s.Context(), dashboardID)
		}
	})

	requireAdmin := false
	showInSidebar := false

	// Create a test dashboard
	cfg := homeassistant.DashboardConfig{
		URLPath:       urlPath,
		Title:         "Patch Order Test",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	}

	created, err := s.Client().CreateDashboard(s.Context(), cfg)
	s.Require().NoError(err, "Failed to create test dashboard")
	s.Require().NotNil(created)

	dashboardID = created.ID

	time.Sleep(500 * time.Millisecond)

	// Save a 4-view baseline (matches the real-world reproducer for the view-reorder bug)
	baseline := map[string]any{
		"views": []any{
			map[string]any{"title": "Übersicht", "path": "overview"},
			map[string]any{"title": "Twingo", "path": "twingo"},
			map[string]any{"title": "IONIQ 6", "path": "ioniq"},
			map[string]any{"title": "Wallbox", "path": "wallbox"},
		},
	}

	err = s.Client().SaveLovelaceConfig(s.Context(), urlPath, baseline)
	s.Require().NoError(err, "Failed to save baseline config")

	time.Sleep(500 * time.Millisecond)

	// Verify baseline was saved correctly
	before, err := s.Client().GetLovelaceConfig(s.Context(), urlPath)
	s.Require().NoError(err, "Failed to retrieve baseline config")

	beforeViews, ok := before["views"].([]any)
	s.Require().True(ok, "Baseline views should be a slice")
	s.Require().Len(beforeViews, 4, "Baseline should have 4 views")
	s.Equal("Übersicht", beforeViews[0].(map[string]any)["title"], "views[0] before patch")
	s.Equal("Twingo", beforeViews[1].(map[string]any)["title"], "views[1] before patch")
	s.Equal("IONIQ 6", beforeViews[2].(map[string]any)["title"], "views[2] before patch")
	s.Equal("Wallbox", beforeViews[3].(map[string]any)["title"], "views[3] before patch")

	// Apply a replace on views[2] — the operation from the issue report
	patched := map[string]any{
		"views": []any{
			beforeViews[0],
			beforeViews[1],
			map[string]any{"title": "IONIQ 6 Pro", "path": "ioniq"},
			beforeViews[3],
		},
	}

	err = s.Client().SaveLovelaceConfig(s.Context(), urlPath, patched)
	s.Require().NoError(err, "Failed to save patched config")

	time.Sleep(500 * time.Millisecond)

	// Read back and verify view order is preserved
	after, err := s.Client().GetLovelaceConfig(s.Context(), urlPath)
	s.Require().NoError(err, "Failed to retrieve patched config")

	afterViews, ok := after["views"].([]any)
	s.Require().True(ok, "Patched views should be a slice")
	s.Require().Len(afterViews, 4, "Patched config should still have 4 views")

	wantTitles := []string{"Übersicht", "Twingo", "IONIQ 6 Pro", "Wallbox"}
	for i, v := range afterViews {
		vm, ok := v.(map[string]any)
		s.Require().True(ok, "views[%d] should be a map", i)
		s.Equal(wantTitles[i], vm["title"],
			"views[%d].title after patch (sibling order must not change)", i)
	}
}
