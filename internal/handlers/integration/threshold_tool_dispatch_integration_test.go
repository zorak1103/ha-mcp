//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type ThresholdToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestThresholdToolDispatch(t *testing.T) {
	suite.Run(t, new(ThresholdToolDispatchTestSuite))
}

// TestThresholdUpdateViaTool generalizes the verification of the fix for
// manage_helper update passing a bare object_id instead of the full entity_id
// to config-entry helper routing (which caused "unknown_command" failures)
// beyond template helpers to another config-entry helper platform (threshold
// lives on the binary_sensor domain), driven through the real manage_helper
// tool.
func (s *ThresholdToolDispatchTestSuite) TestThresholdUpdateViaTool() {
	inputName := GenerateTestID("thresh_td_input")
	inputEntityID := BuildEntityID("input_number", inputName)
	sensorName := GenerateTestID("thresh_td_sensor")
	sensorEntityID := BuildEntityID("sensor", sensorName)
	thresholdName := GenerateTestID("thresh_td")
	thresholdEntityID := BuildEntityID("binary_sensor", thresholdName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
		_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
	})

	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_number",
		Config:   map[string]any{"name": inputName, "min": 0.0, "max": 100.0, "initial": 25.0},
	})
	s.Require().NoError(err, "Failed to create input_number")
	_, err = s.WaitForEntity(inputEntityID, 5*time.Second)
	s.Require().NoError(err)

	err = s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "template",
		Config:   map[string]any{"name": sensorName, "state": "{{ states('" + inputEntityID + "') | float }}"},
	})
	s.Require().NoError(err, "Failed to create source template sensor")
	_, err = s.WaitForEntity(sensorEntityID, 5*time.Second)
	s.Require().NoError(err)

	err = s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "threshold",
		Config: map[string]any{
			"name":      thresholdName,
			"entity_id": sensorEntityID,
			"upper":     50.0,
		},
	})
	s.Require().NoError(err, "Failed to create threshold")
	_, err = s.WaitForEntity(thresholdEntityID, 5*time.Second)
	s.Require().NoError(err, "Threshold did not appear")

	// The action under test: update via the real manage_helper tool.
	result := s.CallTool("manage_helper", map[string]any{
		"action":     "update",
		"entity_id":  thresholdEntityID,
		"upper":      80.0,
		"hysteresis": 2.0,
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(2 * time.Second)

	entity, err := s.Client().GetState(s.Context(), thresholdEntityID)
	s.Require().NoError(err)

	upper, ok := entity.Attributes["upper"].(float64)
	s.Require().True(ok, "upper attribute missing or wrong type: %#v", entity.Attributes["upper"])
	s.Equal(80.0, upper, "upper threshold should be updated to 80.0")

	hysteresis, ok := entity.Attributes["hysteresis"].(float64)
	s.Require().True(ok, "hysteresis attribute missing or wrong type: %#v", entity.Attributes["hysteresis"])
	s.Equal(2.0, hysteresis, "hysteresis should be updated to 2.0")
}
