//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type ThresholdIntegrationTestSuite struct {
	HelperTestSuite
}

func TestThresholdIntegration(t *testing.T) {
	suite.Run(t, new(ThresholdIntegrationTestSuite))
}

// createSourceSensor creates an input_number and a template sensor that reads it.
// The template sensor (sensor domain) can be used as a threshold source.
// Returns (inputNumberEntityID, templateSensorEntityID).
func (s *ThresholdIntegrationTestSuite) createSourceSensor(prefix string, initialValue float64) (string, string) {
	inputName := GenerateTestID(prefix + "_input")
	inputEntityID := BuildEntityID("input_number", inputName)
	sensorName := GenerateTestID(prefix + "_sensor")
	sensorEntityID := BuildEntityID("sensor", sensorName)

	// Create input_number
	inputConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    inputName,
			"min":     0.0,
			"max":     100.0,
			"initial": initialValue,
		},
	}

	err := s.Client().CreateHelper(s.Context(), inputConfig)
	s.Require().NoError(err, "Failed to create input_number")

	_, err = s.WaitForEntity(inputEntityID, 5*time.Second)
	s.Require().NoError(err, "Input_number did not appear")

	// Create template sensor that reads the input_number
	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  sensorName,
			"state": "{{ states('" + inputEntityID + "') | float }}",
		},
	}

	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	_, err = s.WaitForEntity(sensorEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	return inputEntityID, sensorEntityID
}

func (s *ThresholdIntegrationTestSuite) TestThresholdLifecycle() {
	// Create source sensor (input_number + template sensor wrapper)
	inputEntityID, sensorEntityID := s.createSourceSensor("thresh_src", 25.0)
	thresholdName := GenerateTestID("threshold")
	thresholdEntityID := BuildEntityID("binary_sensor", thresholdName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
		_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
	})

	// Create threshold sensor (turns on when source is above 50)
	thresholdConfig := homeassistant.HelperConfig{
		Platform: "threshold",
		Config: map[string]any{
			"name":       thresholdName,
			"entity_id":  sensorEntityID, // Use template sensor (sensor domain)
			"upper":      50.0,
			"hysteresis": 0.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), thresholdConfig)
	s.Require().NoError(err, "Failed to create threshold")

	entity, err := s.WaitForEntity(thresholdEntityID, 5*time.Second)
	s.Require().NoError(err, "Threshold did not appear")
	s.Equal("off", entity.State, "Threshold should be off when source (25) is below upper (50)")

	// Set source above threshold (via input_number)
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": inputEntityID,
		"value":     75.0,
	})
	s.Require().NoError(err, "Failed to set source value")

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), thresholdEntityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "Threshold should be on when source (75) is above upper (50)")

	// Set source below threshold
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": inputEntityID,
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

	// Cleanup source entities
	_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
	_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
}

func (s *ThresholdIntegrationTestSuite) TestThresholdLower() {
	// Create source sensor
	inputEntityID, sensorEntityID := s.createSourceSensor("thresh_low", 50.0)
	thresholdName := GenerateTestID("thresh_low")
	thresholdEntityID := BuildEntityID("binary_sensor", thresholdName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
		_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
	})

	// Create threshold sensor (turns on when source is below 30)
	thresholdConfig := homeassistant.HelperConfig{
		Platform: "threshold",
		Config: map[string]any{
			"name":      thresholdName,
			"entity_id": sensorEntityID,
			"lower":     30.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), thresholdConfig)
	s.Require().NoError(err, "Failed to create threshold")

	entity, err := s.WaitForEntity(thresholdEntityID, 5*time.Second)
	s.Require().NoError(err, "Threshold did not appear")
	s.Equal("off", entity.State, "Threshold should be off when source (50) is above lower (30)")

	// Set source below lower threshold
	_, err = s.Client().CallService(s.Context(), "input_number", "set_value", map[string]any{
		"entity_id": inputEntityID,
		"value":     20.0,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), thresholdEntityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "Threshold should be on when source (20) is below lower (30)")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
	_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
	_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
}

func (s *ThresholdIntegrationTestSuite) TestThresholdWithHysteresis() {
	// Create source sensor
	inputEntityID, sensorEntityID := s.createSourceSensor("thresh_hyst", 30.0)
	thresholdName := GenerateTestID("thresh_hyst")
	thresholdEntityID := BuildEntityID("binary_sensor", thresholdName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
		_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
	})

	// Create threshold with hysteresis of 5
	// Upper threshold at 50, turns on at 50, turns off at 45 (50-5)
	thresholdConfig := homeassistant.HelperConfig{
		Platform: "threshold",
		Config: map[string]any{
			"name":       thresholdName,
			"entity_id":  sensorEntityID,
			"upper":      50.0,
			"hysteresis": 5.0,
		},
	}

	err := s.Client().CreateHelper(s.Context(), thresholdConfig)
	s.Require().NoError(err, "Failed to create threshold with hysteresis")

	entity, err := s.WaitForEntity(thresholdEntityID, 5*time.Second)
	s.Require().NoError(err, "Threshold did not appear")
	s.Equal("off", entity.State, "Threshold should be off initially when source (30) < upper (50)")

	// Verify the hysteresis attribute was set correctly
	if hysteresis, ok := entity.Attributes["hysteresis"].(float64); ok {
		s.Equal(5.0, hysteresis, "Hysteresis attribute should be 5.0")
	}

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), thresholdEntityID)
	s.Require().NoError(err, "Failed to delete threshold")

	err = s.WaitForEntityGone(thresholdEntityID, 5*time.Second)
	s.Require().NoError(err, "Threshold should be deleted")

	// Cleanup source entities
	_ = s.Client().DeleteHelper(s.Context(), sensorEntityID)
	_ = s.Client().DeleteHelper(s.Context(), inputEntityID)
}
