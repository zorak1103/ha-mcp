//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// ManageStatisticsIntegrationTestSuite covers the recorder statistics
// operations backing the manage_statistics tool.
type ManageStatisticsIntegrationTestSuite struct {
	HelperTestSuite
}

func TestManageStatisticsIntegration(t *testing.T) {
	suite.Run(t, new(ManageStatisticsIntegrationTestSuite))
}

// TestListStatisticIDs verifies recorder/list_statistic_ids round-trips and
// parses. The list may be empty on a fresh HA instance, but the call must
// succeed and every entry must carry id + source.
func (s *ManageStatisticsIntegrationTestSuite) TestListStatisticIDs() {
	metas, err := s.Client().ListStatisticIDs(s.Context(), "")
	s.Require().NoError(err, "ListStatisticIDs should not return an error")
	for _, m := range metas {
		s.NotEmpty(m.StatisticID, "each entry should have a statistic_id")
		s.NotEmpty(m.Source, "each entry should have a source")
	}
}

// TestListStatisticIDs_Filtered verifies both statistic_type filters are
// accepted by HA.
func (s *ManageStatisticsIntegrationTestSuite) TestListStatisticIDs_Filtered() {
	for _, statisticType := range []string{"mean", "sum"} {
		_, err := s.Client().ListStatisticIDs(s.Context(), statisticType)
		s.Require().NoError(err, "ListStatisticIDs(%q) should not return an error", statisticType)
	}
}

// TestValidateStatistics verifies recorder/validate_statistics round-trips.
// An empty result is normal for a healthy recorder DB.
func (s *ManageStatisticsIntegrationTestSuite) TestValidateStatistics() {
	issues, err := s.Client().ValidateStatistics(s.Context())
	s.Require().NoError(err, "ValidateStatistics should not return an error")
	for id, list := range issues {
		s.NotEmpty(id, "issue map keys are statistic ids")
		for _, issue := range list {
			s.NotEmpty(issue.Type, "each issue must have a type")
		}
	}
}

// TestManageStatisticsLifecycle creates a real statistics-recording sensor
// (input_number source + template sensor with state_class), verifies it
// appears in list_statistic_ids, and exercises clear_statistics on it —
// the find-orphaned-stats -> clean-up workflow against
// test-owned data only.
//
// NOTE: long-term statistics rows are only written on the recorder's
// ~5-minute boundary, but list_statistic_ids merges integration-provided
// metadata, so the id should appear shortly after the entity exists. The
// clear call is verified to succeed; row-level disappearance can't be
// asserted deterministically without waiting out a recorder cycle.
func (s *ManageStatisticsIntegrationTestSuite) TestManageStatisticsLifecycle() {
	ctx := s.Context()

	inputName := GenerateTestID("stats_mgmt_input")
	inputEntityID := BuildEntityID("input_number", inputName)
	templateName := GenerateTestID("stats_mgmt_sensor")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(ctx, templateEntityID)
		_ = s.Client().DeleteHelper(ctx, inputEntityID)
	})

	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":                inputName,
			"min":                 0.0,
			"max":                 100.0,
			"initial":             20.0,
			"unit_of_measurement": "kWh",
		},
	}
	s.Require().NoError(s.Client().CreateHelper(ctx, sourceConfig), "Failed to create source input_number")
	_, err := s.WaitForEntity(inputEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":                templateName,
			"state":               "{{ states('" + inputEntityID + "') | float }}",
			"unit_of_measurement": "kWh",
			"device_class":        "energy",
			"state_class":         "total_increasing",
		},
	}
	s.Require().NoError(s.Client().CreateHelper(ctx, templateConfig), "Failed to create template sensor")
	_, err = s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	// The statistics metadata should list the new sensor's id shortly after
	// the entity exists (integration-provided metadata, no DB row needed).
	s.Eventually(func() bool {
		metas, err := s.Client().ListStatisticIDs(ctx, "")
		if err != nil {
			return false
		}
		for _, m := range metas {
			if m.StatisticID == templateEntityID {
				return true
			}
		}
		return false
	}, 30*time.Second, time.Second, "template sensor statistic id should appear in list_statistic_ids")

	// clear_statistics must succeed on the test-owned id (HA waits for the
	// recorder queue acknowledgement before responding).
	s.Require().NoError(s.Client().ClearStatistics(ctx, []string{templateEntityID}),
		"ClearStatistics should accept the test sensor's statistic id")

	// A synthetic unknown id must also be accepted without error — HA filters
	// clear_statistics to ids it knows.
	s.Require().NoError(s.Client().ClearStatistics(ctx, []string{GenerateTestID("stats_mgmt_unknown")}),
		"ClearStatistics should accept unknown statistic ids without error")
}

// TestManageStatisticsToolDispatch exercises the MCP tool surface end to end
// (registration, dispatch, arg validation) through the real registry.
func (s *ManageStatisticsIntegrationTestSuite) TestManageStatisticsToolDispatch() {
	result := s.CallTool("manage_statistics", map[string]any{"action": "list"})
	s.Require().NotNil(result)
	s.False(result.IsError, "manage_statistics action=list should not error: %v", result.Content)

	result = s.CallTool("manage_statistics", map[string]any{"action": "bogus"})
	s.Require().NotNil(result)
	s.True(result.IsError, "unknown action should be rejected")

	result = s.CallTool("manage_statistics", map[string]any{"action": "validate"})
	s.Require().NotNil(result)
	s.False(result.IsError, "manage_statistics action=validate should not error: %v", result.Content)

	result = s.CallTool("manage_statistics", map[string]any{"action": "clear"})
	s.Require().NotNil(result)
	s.True(result.IsError, "clear without statistic_ids should be rejected")

	result = s.CallTool("manage_statistics", map[string]any{
		"action":        "clear",
		"statistic_ids": []any{"not-a-valid-id"},
	})
	s.Require().NotNil(result)
	s.True(result.IsError, "clear with a malformed statistic id should be rejected")

	// A well-formed but unknown test-owned id is accepted end to end through
	// the handler's stricter validation - HA filters clear_statistics to ids
	// it actually knows, so this exercises the tool dispatch path without
	// needing a real recorder row.
	unknownID := BuildEntityID("sensor", GenerateTestID("stats_dispatch_unknown"))
	result = s.CallTool("manage_statistics", map[string]any{
		"action":        "clear",
		"statistic_ids": []any{unknownID},
	})
	s.Require().NotNil(result)
	s.False(result.IsError, "manage_statistics action=clear with a well-formed test id should not error: %v", result.Content)
}
