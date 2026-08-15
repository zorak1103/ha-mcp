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
