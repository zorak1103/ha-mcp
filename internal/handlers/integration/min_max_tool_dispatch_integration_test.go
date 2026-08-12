//go:build integration

package integration

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// MinMaxToolDispatchTestSuite verifies min_max helper creation and update
// through the real manage_helper tool dispatch path (CallTool), not via
// direct Client().CreateHelper calls. The pre-existing
// min_max_integration_test.go builds a homeassistant.HelperConfig by hand
// and calls the client directly, bypassing handleCreate/buildMinMaxConfig
// entirely - per this project's documented integration-test scope, that
// structurally cannot catch a handler-layer bug. It did not catch the
// min_max_type collision: handleCreate reads args["type"] to select the
// helper type ("min_max") before the same args map reached
// buildMinMaxConfig, which used to read args["type"] again for the
// calculation selector - always seeing the already-consumed "min_max"
// value instead of the caller's intended calculation. The tool argument is
// now the distinct name "min_max_type".
type MinMaxToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestMinMaxToolDispatchIntegration(t *testing.T) {
	suite.Run(t, new(MinMaxToolDispatchTestSuite))
}

func (s *MinMaxToolDispatchTestSuite) createSourceNumber(prefix string) string {
	name := GenerateTestID(prefix)
	entityID := BuildEntityID("input_number", name)

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name": name,
			"min":  0.0,
			"max":  1000.0,
		},
	})
	s.Require().NoError(err, "failed to create source input_number")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "source input_number did not appear")

	return entityID
}

// waitForNumericState polls entityID's state until it is within 0.5 of want
// or timeout elapses, returning the last observed numeric value. Home
// Assistant's min_max sensor does not evaluate its sources' current states
// on setup - it only recomputes on a state_changed event fired after it
// starts listening - so a freshly (re)created min_max entity reads
// "unknown" until a source is nudged; this poll accommodates both that
// nudge propagating and the config-entry reload after an update.
func (s *MinMaxToolDispatchTestSuite) waitForNumericState(entityID string, want float64, timeout time.Duration) float64 {
	deadline := time.Now().Add(timeout)
	var last float64
	for time.Now().Before(deadline) {
		entity, err := s.Client().GetState(s.Context(), entityID)
		if err == nil && entity != nil {
			// entity.State is a string (HA's state API always returns state
			// as text, e.g. "20.0" or "unknown") - unlike toFloat (used
			// elsewhere in this package for JSON-decoded config values),
			// which only handles float64/int and would silently read every
			// state as 0.
			if parsed, parseErr := strconv.ParseFloat(entity.State, 64); parseErr == nil {
				last = parsed
				if last >= want-0.5 && last <= want+0.5 {
					return last
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return last
}

// TestCreateViaToolDispatchUsesCorrectCalculationType is the regression test
// for the min_max_type collision: creating with min_max_type: "mean" must
// produce a sensor that actually computes the mean, not one configured with
// the (invalid) calculation type "min_max". Before the fix, HA rejected this
// create outright with a voluptuous "value must be one of ..." error, since
// "min_max" is not a member of the calculation-type select.
func (s *MinMaxToolDispatchTestSuite) TestCreateViaToolDispatchUsesCorrectCalculationType() {
	src1 := s.createSourceNumber("mm_dispatch_src")
	src2 := s.createSourceNumber("mm_dispatch_src")

	minMaxName := GenerateTestID("mm_dispatch")
	minMaxID := BuildEntityID("sensor", minMaxName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), minMaxID) })
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), src1) })
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), src2) })

	result := s.CallTool("manage_helper", map[string]any{
		"action":       "create",
		"type":         "min_max",
		"id":           minMaxName,
		"name":         minMaxName,
		"entity_ids":   []any{src1, src2},
		"min_max_type": "mean",
	})
	s.Require().False(result.IsError, "manage_helper create should succeed, got: %s", resultText(result))

	_, err := s.WaitForEntity(minMaxID, 5*time.Second)
	s.Require().NoError(err, "min_max entity did not appear")

	// Nudge both sources now that the sensor is listening, so it actually
	// recomputes instead of sitting at "unknown".
	s.Require().NoError(s.Client().SetHelperValue(s.Context(), src1, 10.0))
	s.Require().NoError(s.Client().SetHelperValue(s.Context(), src2, 30.0))

	got := s.waitForNumericState(minMaxID, 20.0, 5*time.Second)
	s.InDelta(20.0, got, 0.5,
		"mean of 10 and 30 should be 20 - a wrong value would indicate min_max_type was not forwarded as the calculation type")
}

// TestUpdateViaToolDispatchChangesCalculationType covers the update side of
// the same field: addExtendedConfigEntryFields must map min_max_type to
// HA's "type" config key so an update can actually change the calculation,
// which was unverifiable before this fix (the field was excluded from the
// update path entirely).
func (s *MinMaxToolDispatchTestSuite) TestUpdateViaToolDispatchChangesCalculationType() {
	src1 := s.createSourceNumber("mm_dispatch_upd")
	src2 := s.createSourceNumber("mm_dispatch_upd")

	minMaxName := GenerateTestID("mm_dispatch_upd")
	minMaxID := BuildEntityID("sensor", minMaxName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), minMaxID) })
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), src1) })
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), src2) })

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "min_max",
		Config: map[string]any{
			"name":       minMaxName,
			"entity_ids": []string{src1, src2},
			"type":       "min",
		},
	})
	s.Require().NoError(err, "failed to create min_max")

	_, err = s.WaitForEntity(minMaxID, 5*time.Second)
	s.Require().NoError(err, "min_max entity did not appear")

	s.Require().NoError(s.Client().SetHelperValue(s.Context(), src1, 10.0))
	s.Require().NoError(s.Client().SetHelperValue(s.Context(), src2, 30.0))

	got := s.waitForNumericState(minMaxID, 10.0, 5*time.Second)
	s.InDelta(10.0, got, 0.5, "initial calculation should be min (10)")

	// "name" is deliberately omitted: min_max's Options Flow schema in Home
	// Assistant does not include it (CONFIG_SCHEMA extends OPTIONS_SCHEMA
	// with "name" only for create), so submitting it on update fails with
	// "extra keys not allowed @ data['name']" - the same error class
	// documented for switch_as_x's update path. Unrelated to min_max_type;
	// "name" is optional on manage_helper update.
	result := s.CallTool("manage_helper", map[string]any{
		"action":       "update",
		"entity_id":    minMaxID,
		"min_max_type": "max",
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	// The update reloads the config entry, recreating the sensor - nudge the
	// sources again so it recomputes with the new calculation type instead
	// of sitting at "unknown" post-reload.
	s.Require().NoError(s.Client().SetHelperValue(s.Context(), src1, 10.0))
	s.Require().NoError(s.Client().SetHelperValue(s.Context(), src2, 30.0))

	got = s.waitForNumericState(minMaxID, 30.0, 5*time.Second)
	s.InDelta(30.0, got, 0.5,
		"calculation should now be max (30) - a value of 10 would indicate min_max_type was not forwarded on update")
}
