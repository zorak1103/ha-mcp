//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// DashboardPatchToolDispatchTestSuite exercises manage_dashboard's patch
// action through the real tool-dispatch layer (registry -> handler ->
// HybridClient) against a live Home Assistant instance, covering the
// #142/#144 fixes: semantic match+section addressing that recurses into
// cards nested arbitrarily deep below a section, and a dry-run diff whose
// size stays bounded regardless of how large the matched/removed subtree is.
type DashboardPatchToolDispatchTestSuite struct {
	DashboardTestSuite
}

func TestDashboardPatchToolDispatch(t *testing.T) {
	suite.Run(t, new(DashboardPatchToolDispatchTestSuite))
}

// createTestDashboardWithConfig creates a storage-mode dashboard, saves the
// given baseline config, registers cleanup, and returns the dashboard's
// url_path.
func (s *DashboardPatchToolDispatchTestSuite) createTestDashboardWithConfig(name string, baseline map[string]any) string {
	urlPath := generateDashboardURLPath(name)
	requireAdmin := false
	showInSidebar := false

	created, err := s.Client().CreateDashboard(s.Context(), homeassistant.DashboardConfig{
		URLPath:       urlPath,
		Title:         "Patch Tool Dispatch Test",
		Icon:          "mdi:view-dashboard",
		Mode:          "storage",
		RequireAdmin:  &requireAdmin,
		ShowInSidebar: &showInSidebar,
	})
	s.Require().NoError(err, "Failed to create test dashboard")
	s.Require().NotNil(created)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteDashboard(s.Context(), created.ID)
	})

	time.Sleep(500 * time.Millisecond)

	err = s.Client().SaveLovelaceConfig(s.Context(), urlPath, baseline)
	s.Require().NoError(err, "Failed to save baseline config")

	time.Sleep(500 * time.Millisecond)

	return urlPath
}

// TestNestedCardPatchViaTool is an end-to-end regression test for issue #144:
// manage_dashboard's patch action must resolve match+section addressing into
// a card nested several levels below "views" (sections -> cards -> cards ->
// chips), not just the section's direct elements.
func (s *DashboardPatchToolDispatchTestSuite) TestNestedCardPatchViaTool() {
	trackerID := BuildEntityID("device_tracker", GenerateTestID("nested_patch"))
	trackerIDNew := trackerID + "_new"

	baseline := map[string]any{
		"views": []any{
			map[string]any{
				"title": "Overview",
				"sections": []any{
					map[string]any{
						"cards": []any{
							map[string]any{
								"cards": []any{
									map[string]any{
										"chips": []any{
											map[string]any{
												"entity":  trackerID,
												"content": "Example",
											},
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

	urlPath := s.createTestDashboardWithConfig("dash-nested-patch", baseline)

	result := s.CallTool("manage_dashboard", map[string]any{
		"action":   "patch",
		"url_path": urlPath,
		"operations": []any{
			map[string]any{
				"op":      "replace",
				"match":   map[string]any{"content": "Example", "entity": trackerID},
				"section": "views",
				"field":   "entity",
				"value":   trackerIDNew,
			},
		},
	})
	s.Require().False(result.IsError, "manage_dashboard patch should succeed, got: %s", resultText(result))

	time.Sleep(500 * time.Millisecond)

	after, err := s.Client().GetLovelaceConfig(s.Context(), urlPath)
	s.Require().NoError(err, "Failed to retrieve patched config")

	views, ok := after["views"].([]any)
	s.Require().True(ok, "views should be a slice")
	sections, ok := views[0].(map[string]any)["sections"].([]any)
	s.Require().True(ok, "views[0].sections should be a slice")
	outerCards, ok := sections[0].(map[string]any)["cards"].([]any)
	s.Require().True(ok, "sections[0].cards should be a slice")
	innerCards, ok := outerCards[0].(map[string]any)["cards"].([]any)
	s.Require().True(ok, "cards[0].cards should be a slice")
	chips, ok := innerCards[0].(map[string]any)["chips"].([]any)
	s.Require().True(ok, "cards[0].cards[0].chips should be a slice")
	chip, ok := chips[0].(map[string]any)
	s.Require().True(ok, "chips[0] should be a map")

	s.Equal(trackerIDNew, chip["entity"], "nested chip entity should be updated by the patch (issue #144)")
	s.Equal("Example", chip["content"], "sibling field must be untouched")
}

// TestNestedCardPatchDryRunViaTool is an end-to-end regression test for issue
// #142: a dry-run patch must render a compact per-operation diff instead of
// the entire patched config, so that removing a large nested subtree cannot
// blow up the response into thousands of tokens. It doubles as a regression
// test for #144 since the removed card is located via match+section
// addressing nested below "views", and it confirms dry-run never persists a
// change to the live dashboard.
func (s *DashboardPatchToolDispatchTestSuite) TestNestedCardPatchDryRunViaTool() {
	// A card with 200 dummy chip entries is large enough that dumping it
	// untruncated into the response would run to tens of KB - the token
	// blow-up reported in #142.
	bigChips := make([]any, 200)
	for i := range bigChips {
		bigChips[i] = map[string]any{"type": "weather", "show_temperature": true, "show_conditions": true}
	}

	baseline := map[string]any{
		"views": []any{
			map[string]any{
				"title": "Overview",
				"cards": []any{
					map[string]any{
						"card_marker": "big-card-under-test",
						"chips":       bigChips,
					},
				},
			},
		},
	}

	urlPath := s.createTestDashboardWithConfig("dash-nested-dryrun", baseline)

	result := s.CallTool("manage_dashboard", map[string]any{
		"action":   "patch",
		"url_path": urlPath,
		"dry_run":  true,
		"operations": []any{
			map[string]any{
				"op":      "remove",
				"match":   map[string]any{"card_marker": "big-card-under-test"},
				"section": "views",
			},
		},
	})
	s.Require().False(result.IsError, "dry-run patch should succeed, got: %s", resultText(result))

	text := resultText(result)
	s.Contains(text, "Dry-run result for dashboard", "dry-run response should be labeled")
	s.Contains(text, "NOT saved", "dry-run response should state nothing was saved")
	s.Contains(text, "(removed)", "removed op should render the (removed) marker")
	s.Less(len(text), 2000,
		"dry-run diff must stay compact even for a large matched subtree (issue #142); got %d chars", len(text))

	// Confirm dry-run truly did not persist: the big card must still be there.
	after, err := s.Client().GetLovelaceConfig(s.Context(), urlPath)
	s.Require().NoError(err, "Failed to retrieve config after dry-run")
	views, ok := after["views"].([]any)
	s.Require().True(ok, "views should be a slice")
	cards, ok := views[0].(map[string]any)["cards"].([]any)
	s.Require().True(ok, "views[0].cards should be a slice")
	s.Require().Len(cards, 1, "dry-run must not remove the card from the saved config")
}
