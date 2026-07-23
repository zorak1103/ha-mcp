//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type GroupToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestGroupToolDispatch(t *testing.T) {
	suite.Run(t, new(GroupToolDispatchTestSuite))
}

// TestGroupDeleteViaTool exercises the new wsClientImpl.DeleteHelper guard
// (isWSHelperPlatform) added alongside the #135 fix: "group" is a config-entry
// platform recognized by extractPlatform, so without the guard a delete
// falling through to the WS layer would build the nonexistent "group/delete"
// command instead of failing clearly or routing correctly.
func (s *GroupToolDispatchTestSuite) TestGroupDeleteViaTool() {
	num1Name := GenerateTestID("grp_td1")
	num2Name := GenerateTestID("grp_td2")
	num1EntityID := BuildEntityID("input_number", num1Name)
	num2EntityID := BuildEntityID("input_number", num2Name)
	groupName := GenerateTestID("grp_td")
	groupEntityID := BuildEntityID("sensor", groupName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
		_ = s.Client().DeleteHelper(s.Context(), num1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), num2EntityID)
	})

	for _, cfg := range []homeassistant.HelperConfig{
		{Platform: "input_number", Config: map[string]any{"name": num1Name, "min": 0.0, "max": 100.0, "initial": 10.0}},
		{Platform: "input_number", Config: map[string]any{"name": num2Name, "min": 0.0, "max": 100.0, "initial": 20.0}},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err, "Failed to create input_number")
	}
	_, err := s.WaitForEntity(num1EntityID, 5*time.Second)
	s.Require().NoError(err)
	_, err = s.WaitForEntity(num2EntityID, 5*time.Second)
	s.Require().NoError(err)

	err = s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "group",
		Config: map[string]any{
			"name":     groupName,
			"entities": []string{num1EntityID, num2EntityID},
		},
	})
	s.Require().NoError(err, "Failed to create sensor group")
	_, err = s.WaitForEntity(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Sensor group did not appear")

	// The action under test: delete via the real manage_helper tool.
	result := s.CallTool("manage_helper", map[string]any{
		"action":    "delete",
		"entity_id": groupEntityID,
	})
	s.Require().False(result.IsError, "manage_helper delete should succeed, got: %s", resultText(result))

	err = s.WaitForEntityGone(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Sensor group should be deleted")
}
