//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

var _ = time.Second          // silence unused import
var _ homeassistant.Client   // silence unused import

type ThresholdIntegrationTestSuite struct {
	HelperTestSuite
}

func TestThresholdIntegration(t *testing.T) {
	t.Skip("Threshold helpers do not support WebSocket or REST API create/update - requires YAML configuration")
	suite.Run(t, new(ThresholdIntegrationTestSuite))
}

func (s *ThresholdIntegrationTestSuite) TestThresholdLifecycle() {
	// First create an input_number to use as source
	sourceName := GenerateTestID("thresh_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	thresholdName := GenerateTestID("threshold")
	thresholdEntityID := BuildEntityID("binary_sensor", thresholdName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Create source input_number - entity ID is derived from name
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     100.0,
			"initial": 25.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	// Create threshold sensor (turns on when source is above 50) - entity ID is derived from name
	thresholdConfig := homeassistant.HelperConfig{
		Platform: "threshold",
		Config: map[string]any{
			"name":       thresholdName,
			"entity_id":  sourceEntityID,
			"upper":      50.0,
			"hysteresis": 0.0,
		},
	}

	err = s.Client().CreateHelper(s.Context(), thresholdConfig)
	s.Require().NoError(err, "Failed to create threshold")

	entity, err := s.WaitForEntity(thresholdEntityID, 5*time.Second)
	s.Require().NoError(err, "Threshold did not appear")
	s.Equal("off", entity.State, "Threshold should be off when source (25) is below upper (50)")

	// Set source above threshold
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     75.0,
	})
	s.Require().NoError(err, "Failed to set source value")

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), thresholdEntityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "Threshold should be on when source (75) is above upper (50)")

	// Set source below threshold
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     30.0,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), thresholdEntityID)
	s.Require().NoError(err)
	s.Equal("off", entity.State, "Threshold should be off when source (30) is below upper (50)")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
	s.Require().NoError(err, "Failed to delete threshold")

	err = s.WaitForEntityGone(thresholdEntityID, 5*time.Second)
	s.Require().NoError(err, "Threshold should be deleted")

	// Cleanup source
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *ThresholdIntegrationTestSuite) TestThresholdLower() {
	// Create source input_number
	sourceName := GenerateTestID("thresh_low_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	thresholdName := GenerateTestID("thresh_low")
	thresholdEntityID := BuildEntityID("binary_sensor", thresholdName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     100.0,
			"initial": 50.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create threshold sensor (turns on when source is below 30) - entity ID is derived from name
	thresholdConfig := homeassistant.HelperConfig{
		Platform: "threshold",
		Config: map[string]any{
			"name":      thresholdName,
			"entity_id": sourceEntityID,
			"lower":     30.0,
		},
	}

	err = s.Client().CreateHelper(s.Context(), thresholdConfig)
	s.Require().NoError(err, "Failed to create threshold")

	entity, err := s.WaitForEntity(thresholdEntityID, 5*time.Second)
	s.Require().NoError(err, "Threshold did not appear")
	s.Equal("off", entity.State, "Threshold should be off when source (50) is above lower (30)")

	// Set source below lower threshold
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     20.0,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), thresholdEntityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "Threshold should be on when source (20) is below lower (30)")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}

func (s *ThresholdIntegrationTestSuite) TestThresholdWithHysteresis() {
	sourceName := GenerateTestID("thresh_hyst_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	thresholdName := GenerateTestID("thresh_hyst")
	thresholdEntityID := BuildEntityID("binary_sensor", thresholdName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     100.0,
			"initial": 45.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create threshold with hysteresis of 5
	// Upper threshold at 50, turns on at 50, turns off at 45 (50-5)
	thresholdConfig := homeassistant.HelperConfig{
		Platform: "threshold",
		Config: map[string]any{
			"name":       thresholdName,
			"entity_id":  sourceEntityID,
			"upper":      50.0,
			"hysteresis": 5.0,
		},
	}

	err = s.Client().CreateHelper(s.Context(), thresholdConfig)
	s.Require().NoError(err, "Failed to create threshold")

	entity, err := s.WaitForEntity(thresholdEntityID, 5*time.Second)
	s.Require().NoError(err, "Threshold did not appear")
	s.Equal("off", entity.State, "Threshold should be off initially")

	// Set source above upper threshold
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     55.0,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), thresholdEntityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "Threshold should be on when above upper")

	// Set source between upper and upper-hysteresis (should stay on due to hysteresis)
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     48.0, // Between 45 and 50
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), thresholdEntityID)
	s.Require().NoError(err)
	// Due to hysteresis, should stay on until below 45
	s.Equal("on", entity.State, "Threshold should stay on due to hysteresis")

	// Set source below hysteresis threshold
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": sourceEntityID,
		"value":     40.0, // Below 45
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), thresholdEntityID)
	s.Require().NoError(err)
	s.Equal("off", entity.State, "Threshold should turn off below hysteresis")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
}
