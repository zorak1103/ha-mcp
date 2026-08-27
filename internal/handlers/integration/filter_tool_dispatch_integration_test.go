//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// FilterToolDispatchTestSuite is the regression suite for tmp/issue2.md:
// manage_helper create with type=filter and filter=time_simple_moving_average
// had no way to supply the mandatory window_size field, and every documented
// workaround (a top-level window_size argument, or the filters array
// parameter) either wasn't wired to a real config builder or was rejected by
// Home Assistant outright when combined with the top-level "filter" field.
// These tests go through s.CallTool("manage_helper", ...) rather than
// s.Client().CreateHelper() directly - the pre-existing filter_integration_test.go
// bypasses handleCreate/buildFilterConfig entirely and so cannot catch a
// handler-layer regression, per this project's documented integration-test
// scope (see min_max_tool_dispatch_integration_test.go for the same
// reasoning applied to a different helper type).
type FilterToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestFilterToolDispatchIntegration(t *testing.T) {
	suite.Run(t, new(FilterToolDispatchTestSuite))
}

// createSourceSensor creates an input_number and wraps it with a template
// sensor, since filter requires a sensor.* source entity, not input_number.
func (s *FilterToolDispatchTestSuite) createSourceSensor(prefix string, initialValue float64) (inputEntityID, sensorEntityID string) {
	inputName := GenerateTestID(prefix + "_input")
	inputEntityID = BuildEntityID("input_number", inputName)

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    inputName,
			"min":     0.0,
			"max":     1000.0,
			"initial": initialValue,
		},
	})
	s.Require().NoError(err, "failed to create source input_number")

	_, err = s.WaitForEntity(inputEntityID, 5*time.Second)
	s.Require().NoError(err, "source input_number did not appear")

	sensorName := GenerateTestID(prefix + "_sensor")
	sensorEntityID = BuildEntityID("sensor", sensorName)

	err = s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  sensorName,
			"state": "{{ states('" + inputEntityID + "') | float }}",
		},
	})
	s.Require().NoError(err, "failed to create template sensor wrapper")

	_, err = s.WaitForEntity(sensorEntityID, 5*time.Second)
	s.Require().NoError(err, "template sensor did not appear")

	return inputEntityID, sensorEntityID
}

func (s *FilterToolDispatchTestSuite) registerSourceCleanup(inputEntityID, sensorEntityID string) {
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), sensorEntityID) })
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), inputEntityID) })
}

// TestCreateOutlierViaToolDispatch covers the simple, previously-working
// case (no time-based fields) plus the two new outlier-specific parameters.
func (s *FilterToolDispatchTestSuite) TestCreateOutlierViaToolDispatch() {
	inputID, sensorID := s.createSourceSensor("filter_td_outlier", 42.0)
	s.registerSourceCleanup(inputID, sensorID)

	filterName := GenerateTestID("filter_td_outlier")
	filterID := BuildEntityID("sensor", filterName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), filterID) })

	result := s.CallTool("manage_helper", map[string]any{
		"action":      "create",
		"type":        "filter",
		"id":          filterName,
		"name":        filterName,
		"entity_id":   sensorID,
		"filter":      "outlier",
		"window_size": 4.0,
		"radius":      2.5,
		"precision":   1.0,
	})
	s.Require().False(result.IsError, "manage_helper create should succeed, got: %s", resultText(result))

	entity, err := s.WaitForEntity(filterID, 5*time.Second)
	s.Require().NoError(err, "filter entity did not appear")
	s.NotEmpty(entity.State, "filter should have a state")
}

// TestCreateRangeViaToolDispatch covers the range filter type's
// lower_bound/upper_bound parameters, which had no create path before this fix.
func (s *FilterToolDispatchTestSuite) TestCreateRangeViaToolDispatch() {
	inputID, sensorID := s.createSourceSensor("filter_td_range", 50.0)
	s.registerSourceCleanup(inputID, sensorID)

	filterName := GenerateTestID("filter_td_range")
	filterID := BuildEntityID("sensor", filterName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), filterID) })

	result := s.CallTool("manage_helper", map[string]any{
		"action":      "create",
		"type":        "filter",
		"id":          filterName,
		"name":        filterName,
		"entity_id":   sensorID,
		"filter":      "range",
		"lower_bound": 0.0,
		"upper_bound": 100.0,
	})
	s.Require().False(result.IsError, "manage_helper create should succeed, got: %s", resultText(result))

	_, err := s.WaitForEntity(filterID, 5*time.Second)
	s.Require().NoError(err, "filter entity did not appear")
}

// TestCreateTimeSimpleMovingAverageViaToolDispatch is the exact reproduction
// from tmp/issue2.md: window_size is REQUIRED for this filter type and must
// be submitted to Home Assistant as a DurationSelector dict, not the plain
// number/string previously silently dropped or forwarded unconverted.
// Exercises all three accepted window_size input forms in one create/update
// cycle: HH:MM:SS string, bare seconds, and an explicit dict.
func (s *FilterToolDispatchTestSuite) TestCreateTimeSimpleMovingAverageViaToolDispatch() {
	inputID, sensorID := s.createSourceSensor("filter_td_sma", 10.0)
	s.registerSourceCleanup(inputID, sensorID)

	filterName := GenerateTestID("filter_td_sma")
	filterID := BuildEntityID("sensor", filterName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), filterID) })

	result := s.CallTool("manage_helper", map[string]any{
		"action":      "create",
		"type":        "filter",
		"id":          filterName,
		"name":        filterName,
		"entity_id":   sensorID,
		"filter":      "time_simple_moving_average",
		"window_size": "00:01:30",
	})
	s.Require().False(result.IsError, "manage_helper create (HH:MM:SS window_size) should succeed, got: %s", resultText(result))

	_, err := s.WaitForEntity(filterID, 5*time.Second)
	s.Require().NoError(err, "filter entity did not appear")

	// Update with window_size as a bare number of seconds.
	result = s.CallTool("manage_helper", map[string]any{
		"action":      "update",
		"entity_id":   filterID,
		"window_size": 120.0,
	})
	s.Require().False(result.IsError, "manage_helper update (bare-seconds window_size) should succeed, got: %s", resultText(result))

	// Update with window_size as an explicit duration object.
	result = s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": filterID,
		"window_size": map[string]any{
			"hours": 0.0, "minutes": 2.0, "seconds": 0.0,
		},
	})
	s.Require().False(result.IsError, "manage_helper update (dict window_size) should succeed, got: %s", resultText(result))
}

// TestCreateTimeThrottleViaToolDispatch covers the other duration-window
// filter type.
func (s *FilterToolDispatchTestSuite) TestCreateTimeThrottleViaToolDispatch() {
	inputID, sensorID := s.createSourceSensor("filter_td_throttle", 10.0)
	s.registerSourceCleanup(inputID, sensorID)

	filterName := GenerateTestID("filter_td_throttle")
	filterID := BuildEntityID("sensor", filterName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), filterID) })

	result := s.CallTool("manage_helper", map[string]any{
		"action":      "create",
		"type":        "filter",
		"id":          filterName,
		"name":        filterName,
		"entity_id":   sensorID,
		"filter":      "time_throttle",
		"window_size": 90.0,
	})
	s.Require().False(result.IsError, "manage_helper create should succeed, got: %s", resultText(result))

	_, err := s.WaitForEntity(filterID, 5*time.Second)
	s.Require().NoError(err, "filter entity did not appear")
}

// TestCreateTimeSimpleMovingAverageMissingWindowSizeFails is the exact
// failure reproduced in tmp/issue2.md before this fix: HA rejects the
// create with "window_size: required key not provided" because the field
// wasn't declared or forwarded at all. It must still fail without
// window_size (the field is genuinely required by HA), but the tool must
// no longer be the reason a caller has no way to supply it.
func (s *FilterToolDispatchTestSuite) TestCreateTimeSimpleMovingAverageMissingWindowSizeFails() {
	inputID, sensorID := s.createSourceSensor("filter_td_sma_missing", 10.0)
	s.registerSourceCleanup(inputID, sensorID)

	filterName := GenerateTestID("filter_td_sma_missing")

	result := s.CallTool("manage_helper", map[string]any{
		"action":    "create",
		"type":      "filter",
		"id":        filterName,
		"name":      filterName,
		"entity_id": sensorID,
		"filter":    "time_simple_moving_average",
	})
	s.Require().True(result.IsError, "create without window_size should still fail - it is genuinely required by Home Assistant for this filter type")
}

// TestCreateFilterInvalidWindowSizeReturnsToolError covers the negative
// case: a window_size the tool can't make sense of should fail the tool
// call with a message naming the field, not silently create a
// misconfigured helper or surface an opaque Home Assistant error.
func (s *FilterToolDispatchTestSuite) TestCreateFilterInvalidWindowSizeReturnsToolError() {
	inputID, sensorID := s.createSourceSensor("filter_td_bad_window", 10.0)
	s.registerSourceCleanup(inputID, sensorID)

	filterName := GenerateTestID("filter_td_bad_window")

	result := s.CallTool("manage_helper", map[string]any{
		"action":      "create",
		"type":        "filter",
		"id":          filterName,
		"name":        filterName,
		"entity_id":   sensorID,
		"filter":      "outlier",
		"window_size": true,
	})
	s.Require().True(result.IsError, "a boolean window_size should be rejected")
	s.Contains(resultText(result), "window_size")
}

// TestUpdateFilterRadiusViaOptionsFlow covers the options-flow update path
// for a plain numeric field, and is also the regression check for the
// generic options-flow field-filtering fix: submitting only "radius" must
// not resubmit the removed "filters" parameter or any other stray key that
// this filter's Options Flow schema doesn't declare.
func (s *FilterToolDispatchTestSuite) TestUpdateFilterRadiusViaOptionsFlow() {
	inputID, sensorID := s.createSourceSensor("filter_td_update", 10.0)
	s.registerSourceCleanup(inputID, sensorID)

	filterName := GenerateTestID("filter_td_update")
	filterID := BuildEntityID("sensor", filterName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), filterID) })

	result := s.CallTool("manage_helper", map[string]any{
		"action":    "create",
		"type":      "filter",
		"id":        filterName,
		"name":      filterName,
		"entity_id": sensorID,
		"filter":    "outlier",
		"radius":    2.0,
	})
	s.Require().False(result.IsError, "manage_helper create should succeed, got: %s", resultText(result))

	_, err := s.WaitForEntity(filterID, 5*time.Second)
	s.Require().NoError(err, "filter entity did not appear")

	result = s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": filterID,
		"radius":    5.0,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))
}
