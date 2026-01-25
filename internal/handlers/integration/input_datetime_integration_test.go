//go:build integration

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type InputDatetimeIntegrationTestSuite struct {
	HelperTestSuite
}

func TestInputDatetimeIntegration(t *testing.T) {
	suite.Run(t, new(InputDatetimeIntegrationTestSuite))
}

func (s *InputDatetimeIntegrationTestSuite) TestInputDatetimeFull() {
	testName := GenerateTestID("input_dt")
	entityID := BuildEntityID("input_datetime", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_datetime with both date and time - entity ID is derived from name
	config := homeassistant.HelperConfig{
		Platform: "input_datetime",
		Config: map[string]any{
			"name":     testName,
			"has_date": true,
			"has_time": true,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_datetime")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input datetime did not appear")

	// Test set_datetime with full datetime
	_, err = s.Client().CallService(s.Context(), "input_datetime", "set_datetime", map[string]any{
		"entity_id": entityID,
		"datetime":  "2024-06-15 14:30:00",
	})
	s.Require().NoError(err, "Failed to set datetime")

	time.Sleep(200 * time.Millisecond)
	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Contains(entity.State, "2024-06-15", "State should contain date")
	s.Contains(entity.State, "14:30", "State should contain time")

	// Test set_datetime with date and time separately
	_, err = s.Client().CallService(s.Context(), "input_datetime", "set_datetime", map[string]any{
		"entity_id": entityID,
		"date":      "2024-12-25",
		"time":      "09:00:00",
	})
	s.Require().NoError(err, "Failed to set date and time separately")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Contains(entity.State, "2024-12-25", "State should contain new date")
	s.Contains(entity.State, "09:00", "State should contain new time")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err, "Failed to delete input_datetime")

	err = s.WaitForEntityGone(entityID, 5*time.Second)
	s.Require().NoError(err, "Input datetime should be deleted")
}

func (s *InputDatetimeIntegrationTestSuite) TestInputDatetimeDateOnly() {
	testName := GenerateTestID("input_dt_date")
	entityID := BuildEntityID("input_datetime", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_datetime with date only
	config := homeassistant.HelperConfig{
		Platform: "input_datetime",
		Config: map[string]any{
			"name":     testName,
			"has_date": true,
			"has_time": false,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_datetime")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input datetime did not appear")

	// Test set_datetime with date only
	_, err = s.Client().CallService(s.Context(), "input_datetime", "set_datetime", map[string]any{
		"entity_id": entityID,
		"date":      "2024-01-01",
	})
	s.Require().NoError(err, "Failed to set date")

	time.Sleep(200 * time.Millisecond)
	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("2024-01-01", entity.State, "State should be date only")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

func (s *InputDatetimeIntegrationTestSuite) TestInputDatetimeTimeOnly() {
	testName := GenerateTestID("input_dt_time")
	entityID := BuildEntityID("input_datetime", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_datetime with time only
	config := homeassistant.HelperConfig{
		Platform: "input_datetime",
		Config: map[string]any{
			"name":     testName,
			"has_date": false,
			"has_time": true,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_datetime")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input datetime did not appear")

	// Test set_datetime with time only
	_, err = s.Client().CallService(s.Context(), "input_datetime", "set_datetime", map[string]any{
		"entity_id": entityID,
		"time":      "15:45:00",
	})
	s.Require().NoError(err, "Failed to set time")

	time.Sleep(200 * time.Millisecond)
	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.True(strings.HasPrefix(entity.State, "15:45"), "State should be time only, got: %s", entity.State)

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}
