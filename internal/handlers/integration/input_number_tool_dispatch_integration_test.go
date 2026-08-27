//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type InputNumberToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestInputNumberToolDispatch(t *testing.T) {
	suite.Run(t, new(InputNumberToolDispatchTestSuite))
}

// TestInputNumberUpdateViaTool is a regression check for the backward-compatible
// id-strip added to wsClientImpl.UpdateHelper as part of the fix for manage_helper
// update passing the wrong identifier to config-entry routing: WS
// helpers (input_number, counter, timer, ...) must keep working when the
// handler now passes a full entity_id instead of a bare object_id.
func (s *InputNumberToolDispatchTestSuite) TestInputNumberUpdateViaTool() {
	testName := GenerateTestID("input_number_td")
	entityID := BuildEntityID("input_number", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    testName,
			"min":     0.0,
			"max":     100.0,
			"initial": 50.0,
		},
	})
	s.Require().NoError(err, "Failed to create input_number")

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input number did not appear")
	s.Equal("50.0", entity.State)

	// The action under test: update via the real manage_helper tool.
	// name is required on every WS helper update, even when unchanged - see
	// CLAUDE.md's "WebSocket helper updates require ALL mandatory fields".
	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": entityID,
		"name":      testName,
		"min":       0.0,
		"max":       200.0,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(1 * time.Second)

	// Old max (100) would reject 150; if the update didn't apply, this fails.
	err = s.Client().SetHelperValue(s.Context(), entityID, 150.0)
	s.Require().NoError(err, "Setting 150 should succeed only if max was updated to 200 via the tool call")

	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("150.0", entity.State)
}

// TestInputNumberInitialPersistsViaTool is the regression test for
// tmp/issue.md: manage_helper create with type=input_number and a numeric
// initial silently dropped the field, because the tool's JSON schema
// declared "initial" as a string while buildInputNumberConfig read it via a
// setter that demanded float64 - a schema-conformant client sending the
// number as JSON float64 (this test) or as a string (the string subtest
// below) both used to be discarded with no error, leaving the entity at
// min instead of the requested initial value.
func (s *InputNumberToolDispatchTestSuite) TestInputNumberInitialPersistsViaTool() {
	testName := GenerateTestID("input_number_initial")
	entityID := BuildEntityID("input_number", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	result := s.CallTool("manage_helper", map[string]any{
		"action":  "create",
		"type":    "input_number",
		"id":      testName,
		"name":    testName,
		"min":     500.0,
		"max":     6000.0,
		"step":    50.0,
		"initial": 3000.0,
	})
	s.Require().False(result.IsError, "manage_helper create should succeed, got: %s", resultText(result))

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "input_number did not appear")
	s.Equal("3000.0", entity.State, "state should reflect initial, not min - a dropped initial would come up at min (500)")
	initialAttr, ok := entity.Attributes["initial"].(float64)
	s.Require().True(ok, "initial attribute should be a persisted number, not null/absent (got %v)", entity.Attributes["initial"])
	s.InDelta(3000.0, initialAttr, 0.01, "initial attribute should be persisted, not null")
}

// TestInputNumberInitialAsStringPersistsViaTool covers the exact
// reproduction in tmp/issue.md: a client obeying the tool's (now corrected)
// schema may send initial as a JSON string. buildInputNumberConfig's
// argReader.num must coerce it rather than silently dropping it.
func (s *InputNumberToolDispatchTestSuite) TestInputNumberInitialAsStringPersistsViaTool() {
	testName := GenerateTestID("input_number_initial_str")
	entityID := BuildEntityID("input_number", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	result := s.CallTool("manage_helper", map[string]any{
		"action":  "create",
		"type":    "input_number",
		"id":      testName,
		"name":    testName,
		"min":     500.0,
		"max":     6000.0,
		"initial": "3000",
	})
	s.Require().False(result.IsError, "manage_helper create should succeed, got: %s", resultText(result))

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "input_number did not appear")
	s.Equal("3000.0", entity.State, "a string-valued initial must be coerced, not dropped")
}

// TestInputNumberInitialWrongTypeReturnsError covers the other half of the
// fix: a value argReader.num cannot coerce (e.g. a bool) must fail the
// tool call loudly instead of silently creating the helper without
// initial, which is exactly what happened before this fix (issue.md).
func (s *InputNumberToolDispatchTestSuite) TestInputNumberInitialWrongTypeReturnsError() {
	testName := GenerateTestID("input_number_initial_bad")

	result := s.CallTool("manage_helper", map[string]any{
		"action":  "create",
		"type":    "input_number",
		"id":      testName,
		"name":    testName,
		"min":     0.0,
		"max":     100.0,
		"initial": true,
	})
	s.Require().True(result.IsError, "manage_helper create should fail for a non-coercible initial value")
	s.Contains(resultText(result), "initial")
}
