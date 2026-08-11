//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// HelperPartialUpdateToolDispatchTestSuite verifies the #161 partial-update
// merge (mergeCurrentHelperState, helpers_consolidated.go) against a real
// Home Assistant instance. The merge's correctness rests entirely on
// realHelperStorageConfig, a hand-transcribed model of which fields HA's
// "<platform>/list" (GetHelperConfig) actually returns per WS helper type
// (helpers_consolidated_test.go). Unit tests can only ever check that model
// against itself - CLAUDE.md documents this exact failure mode already
// (script entities' "sequence" attribute: correct against mocks, silently
// wrong against live HA, only caught by a live-HA integration test). Here
// the blast radius is worse than "returns nothing": an update omitting a
// field the model wrongly thinks is recoverable silently resets that field
// to empty in the caller's real Home Assistant instance.
//
// Every test method follows the same shape: create a helper with every
// updatable field set to a distinctive non-default value, update via the
// real manage_helper tool supplying only entity_id plus one unrelated
// field, then re-read and assert every other field still holds its
// original value.
type HelperPartialUpdateToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestHelperPartialUpdateIntegration(t *testing.T) {
	suite.Run(t, new(HelperPartialUpdateToolDispatchTestSuite))
}

func (s *HelperPartialUpdateToolDispatchTestSuite) TestInputNumberPartialUpdatePreservesFields() {
	testName := GenerateTestID("pu_input_number")
	entityID := BuildEntityID("input_number", testName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), entityID) })

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":                testName,
			"min":                 -50.0,
			"max":                 250.0,
			"step":                5.0,
			"initial":             10.0,
			"mode":                "box",
			"unit_of_measurement": "widgets",
			"icon":                "mdi:numeric",
		},
	})
	s.Require().NoError(err, "failed to create input_number")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "input_number did not appear")

	// Update supplying only entity_id, name, and min - everything else
	// (max, step, initial, mode, unit_of_measurement, icon) is omitted and
	// must survive via the merge.
	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      testName,
		"min":       -60.0,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	// Assert via GetHelperConfig, not GetState: "initial" is never exposed
	// as a state attribute for input_number (state reflects the entity's
	// *current* value, not the create-time starting value), so a GetState
	// read cannot catch a merge that silently drops it.
	config, err := s.Client().GetHelperConfig(s.Context(), "input_number", entityID)
	s.Require().NoError(err, "failed to read back input_number config")
	s.InDelta(10.0, toFloat(config["initial"]), 0.001, "initial should survive an update that omitted it")
	s.InDelta(250.0, toFloat(config["max"]), 0.001, "max should survive an update that omitted it")
	s.InDelta(5.0, toFloat(config["step"]), 0.001, "step should survive an update that omitted it")
	s.Equal("box", config["mode"], "mode should survive an update that omitted it")
	s.Equal("widgets", config["unit_of_measurement"], "unit_of_measurement should survive an update that omitted it")
	s.Equal("mdi:numeric", config["icon"], "icon should survive an update that omitted it")

	// Setting -60 should now succeed only if min was actually updated.
	err = s.Client().SetHelperValue(s.Context(), entityID, -60.0)
	s.Require().NoError(err, "setting -60 should succeed only if min was updated to -60")
}

func (s *HelperPartialUpdateToolDispatchTestSuite) TestInputSelectPartialUpdatePreservesFields() {
	testName := GenerateTestID("pu_input_select")
	entityID := BuildEntityID("input_select", testName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), entityID) })

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_select",
		Config: map[string]any{
			"name":    testName,
			"options": []string{"alpha", "beta", "gamma"},
			"initial": "beta",
			"icon":    "mdi:format-list-bulleted",
		},
	})
	s.Require().NoError(err, "failed to create input_select")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "input_select did not appear")

	// Update omits options, initial, and icon entirely, supplying only
	// entity_id and name.
	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      testName,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	// Assert via GetHelperConfig, not GetState: "initial" is never exposed
	// as a state attribute for input_select (state reflects the currently
	// selected option, not the create-time starting selection), so a
	// GetState read cannot catch a merge that silently drops it.
	config, err := s.Client().GetHelperConfig(s.Context(), "input_select", entityID)
	s.Require().NoError(err, "failed to read back input_select config")
	s.Equal("mdi:format-list-bulleted", config["icon"], "icon should survive an update that omitted it")
	s.Equal("beta", config["initial"], "initial should survive an update that omitted it")
	options, ok := config["options"].([]any)
	s.Require().True(ok, "options config field should be a list")
	s.ElementsMatch([]any{"alpha", "beta", "gamma"}, options, "options should survive an update that omitted them")
}

func (s *HelperPartialUpdateToolDispatchTestSuite) TestInputTextPartialUpdatePreservesFields() {
	testName := GenerateTestID("pu_input_text")
	entityID := BuildEntityID("input_text", testName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), entityID) })

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_text",
		Config: map[string]any{
			"name":    testName,
			"min":     2.0,
			"max":     50.0,
			"mode":    "password",
			"pattern": "^[a-z]+$",
			"initial": "hello",
			"icon":    "mdi:form-textbox",
		},
	})
	s.Require().NoError(err, "failed to create input_text")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "input_text did not appear")

	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      testName,
		"max":       80.0,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	// Assert via GetHelperConfig, not GetState: "initial" is never exposed
	// as a state attribute for input_text (state reflects the current text
	// value, not the create-time starting value), so a GetState read cannot
	// catch a merge that silently drops it.
	config, err := s.Client().GetHelperConfig(s.Context(), "input_text", entityID)
	s.Require().NoError(err, "failed to read back input_text config")
	s.InDelta(2.0, toFloat(config["min"]), 0.001, "min should survive an update that omitted it")
	s.Equal("password", config["mode"], "mode should survive an update that omitted it")
	s.Equal("^[a-z]+$", config["pattern"], "pattern should survive an update that omitted it")
	s.Equal("hello", config["initial"], "initial should survive an update that omitted it")
	s.Equal("mdi:form-textbox", config["icon"], "icon should survive an update that omitted it")
}

func (s *HelperPartialUpdateToolDispatchTestSuite) TestInputDatetimePartialUpdatePreservesFields() {
	testName := GenerateTestID("pu_input_datetime")
	entityID := BuildEntityID("input_datetime", testName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), entityID) })

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_datetime",
		Config: map[string]any{
			"name":     testName,
			"has_date": true,
			"has_time": true,
			"initial":  "2024-06-01 08:00:00",
			"icon":     "mdi:calendar-clock",
		},
	})
	s.Require().NoError(err, "failed to create input_datetime")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "input_datetime did not appear")

	// Update omits has_date/has_time/initial/icon entirely, supplying only
	// the identifier and name.
	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      testName,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	// Assert via GetHelperConfig, not GetState: "initial" is never exposed
	// as a state attribute for input_datetime (state reflects the current
	// date/time value, not the create-time starting value), so a GetState
	// read cannot catch a merge that silently drops it.
	config, err := s.Client().GetHelperConfig(s.Context(), "input_datetime", entityID)
	s.Require().NoError(err, "failed to read back input_datetime config")
	s.Equal(true, config["has_date"], "has_date should survive an update that omitted it")
	s.Equal(true, config["has_time"], "has_time should survive an update that omitted it")
	s.Equal("2024-06-01 08:00:00", config["initial"], "initial should survive an update that omitted it")
	s.Equal("mdi:calendar-clock", config["icon"], "icon should survive an update that omitted it")
}

// TestInputBooleanPartialUpdatePreservesFields closes the same coverage gap
// as input_number/input_select/input_text/input_datetime above: "initial"
// is never exposed as a state attribute for input_boolean (state reflects
// the current on/off value, not the create-time starting value), so only a
// GetHelperConfig-based assertion can catch a merge that silently drops it.
func (s *HelperPartialUpdateToolDispatchTestSuite) TestInputBooleanPartialUpdatePreservesFields() {
	testName := GenerateTestID("pu_input_boolean")
	entityID := BuildEntityID("input_boolean", testName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), entityID) })

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name":    testName,
			"initial": true,
			"icon":    "mdi:toggle-switch",
		},
	})
	s.Require().NoError(err, "failed to create input_boolean")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "input_boolean did not appear")

	// Update omits initial/icon entirely, supplying only the identifier and
	// name.
	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      testName,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	config, err := s.Client().GetHelperConfig(s.Context(), "input_boolean", entityID)
	s.Require().NoError(err, "failed to read back input_boolean config")
	s.Equal(true, config["initial"], "initial should survive an update that omitted it")
	s.Equal("mdi:toggle-switch", config["icon"], "icon should survive an update that omitted it")
}

func (s *HelperPartialUpdateToolDispatchTestSuite) TestCounterPartialUpdatePreservesFields() {
	testName := GenerateTestID("pu_counter")
	entityID := BuildEntityID("counter", testName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), entityID) })

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "counter",
		Config: map[string]any{
			"name":    testName,
			"initial": 5.0,
			"step":    2.0,
			"minimum": 0.0,
			"maximum": 1000.0,
			"restore": true,
			"icon":    "mdi:counter",
		},
	})
	s.Require().NoError(err, "failed to create counter")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "counter did not appear")

	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      testName,
		"step":      3.0,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	// Assert via GetHelperConfig, not GetState: this is the exact fixed
	// regression - "restore" was omitted from counter.optionalFields before
	// this fix, and every update silently reset it to HA's default (false),
	// but a GetState-only assertion couldn't have caught that because
	// GetState was never checked for "restore" here to begin with.
	config, err := s.Client().GetHelperConfig(s.Context(), "counter", entityID)
	s.Require().NoError(err, "failed to read back counter config")
	s.InDelta(5.0, toFloat(config["initial"]), 0.001, "initial should survive an update that omitted it")
	s.InDelta(0.0, toFloat(config["minimum"]), 0.001, "minimum should survive an update that omitted it")
	s.InDelta(1000.0, toFloat(config["maximum"]), 0.001, "maximum should survive an update that omitted it")
	s.Equal(true, config["restore"], "restore should survive an update that omitted it")
	s.Equal("mdi:counter", config["icon"], "icon should survive an update that omitted it")
}

// TestTimerPartialUpdatePreservesFields is also the W2 regression check for
// timer.duration/timer.restore round-tripping through mergeCurrentHelperState:
// duration uses a >=24h value to prove HA's _format_timedelta output
// round-trips correctly through cv.time_period on re-submission, and
// restore is left at its non-default (true) value so an omitted "restore"
// must be recovered from state rather than silently reset to HA's
// default=False.
func (s *HelperPartialUpdateToolDispatchTestSuite) TestTimerPartialUpdatePreservesFields() {
	testName := GenerateTestID("pu_timer")
	entityID := BuildEntityID("timer", testName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), entityID) })

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "timer",
		Config: map[string]any{
			"name":     testName,
			"duration": "25:00:00", // >= 24h, to prove _format_timedelta round-trips
			"restore":  true,
			"icon":     "mdi:timer-outline",
		},
	})
	s.Require().NoError(err, "failed to create timer")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "timer did not appear")

	// Update omits duration/restore/icon entirely, supplying only the
	// identifier and name.
	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      testName,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("25:00:00", entity.Attributes["duration"], "duration should survive an update that omitted it, including the >=24h format")
	s.Equal(true, entity.Attributes["restore"], "restore should survive an update that omitted it")
	s.Equal("mdi:timer-outline", entity.Attributes["icon"], "icon should survive an update that omitted it")
}

// TestSchedulePartialUpdatePreservesFields is the highest-value case in this
// suite: an unmerged schedule update wipes every weekday's time blocks
// (HA's schedule/update schema defaults every omitted day to []), so this
// proves the merge actually prevents that specific data-loss failure mode
// against real Home Assistant.
func (s *HelperPartialUpdateToolDispatchTestSuite) TestSchedulePartialUpdatePreservesFields() {
	testName := GenerateTestID("pu_schedule")
	entityID := BuildEntityID("schedule", testName)
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), entityID) })

	mondayBlock := []map[string]any{{"from": "08:00:00", "to": "17:00:00"}}
	fridayBlock := []map[string]any{{"from": "09:00:00", "to": "12:00:00"}}

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "schedule",
		Config: map[string]any{
			"name":   testName,
			"icon":   "mdi:calendar-clock",
			"monday": mondayBlock,
			"friday": fridayBlock,
		},
	})
	s.Require().NoError(err, "failed to create schedule")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "schedule did not appear")

	// Update omits every day and icon, supplying only the identifier and
	// name - if the merge is broken, this wipes monday and friday.
	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      testName,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	config, err := s.Client().GetHelperConfig(s.Context(), "schedule", entityID)
	s.Require().NoError(err, "failed to read back schedule config")

	s.Equal("mdi:calendar-clock", config["icon"], "icon should survive an update that omitted it")
	s.NotEmpty(config["monday"], "monday's time blocks must survive an update that omitted every day - an unmerged update would wipe this to []")
	s.NotEmpty(config["friday"], "friday's time blocks must survive an update that omitted every day - an unmerged update would wipe this to []")
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return 0
	}
}
