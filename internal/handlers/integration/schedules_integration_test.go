//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/stretchr/testify/suite"
)

type ScheduleIntegrationTestSuite struct {
	HelperTestSuite
}

func TestScheduleIntegration(t *testing.T) {
	suite.Run(t, new(ScheduleIntegrationTestSuite))
}

func (s *ScheduleIntegrationTestSuite) TestScheduleLifecycle() {
	testID := GenerateTestID("schedule")
	entityID := BuildEntityID("schedule", testID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create schedule with weekday hours
	config := homeassistant.HelperConfig{
		Platform: "schedule",
		ID:       testID,
		Config: map[string]any{
			"name": "Test Schedule",
			"icon": "mdi:calendar-clock",
			"monday": []map[string]any{
				{"from": "09:00:00", "to": "17:00:00"},
			},
			"tuesday": []map[string]any{
				{"from": "09:00:00", "to": "17:00:00"},
			},
			"wednesday": []map[string]any{
				{"from": "09:00:00", "to": "17:00:00"},
			},
			"thursday": []map[string]any{
				{"from": "09:00:00", "to": "17:00:00"},
			},
			"friday": []map[string]any{
				{"from": "09:00:00", "to": "17:00:00"},
			},
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create schedule")

	// Wait for entity to appear
	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Schedule did not appear")

	// State will be "on" or "off" depending on current time
	s.Contains([]string{"on", "off"}, entity.State, "State should be on or off")

	// Verify the schedule has the expected name
	friendlyName, ok := entity.Attributes["friendly_name"].(string)
	s.True(ok, "friendly_name attribute should exist")
	s.Equal("Test Schedule", friendlyName)

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err, "Failed to delete schedule")

	err = s.WaitForEntityGone(entityID, 5*time.Second)
	s.Require().NoError(err, "Schedule should be deleted")
}

func (s *ScheduleIntegrationTestSuite) TestScheduleMultipleTimeBlocks() {
	testID := GenerateTestID("schedule_multi")
	entityID := BuildEntityID("schedule", testID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create schedule with multiple time blocks per day
	config := homeassistant.HelperConfig{
		Platform: "schedule",
		ID:       testID,
		Config: map[string]any{
			"name": "Test Multi Block Schedule",
			"monday": []map[string]any{
				{"from": "08:00:00", "to": "12:00:00"}, // Morning
				{"from": "13:00:00", "to": "17:00:00"}, // Afternoon
				{"from": "19:00:00", "to": "21:00:00"}, // Evening
			},
			"saturday": []map[string]any{
				{"from": "10:00:00", "to": "14:00:00"}, // Weekend morning
			},
			"sunday": []map[string]any{
				{"from": "10:00:00", "to": "14:00:00"},
			},
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create schedule")

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Schedule did not appear")

	// Verify the schedule exists and has correct state
	s.Contains([]string{"on", "off"}, entity.State)

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

func (s *ScheduleIntegrationTestSuite) TestScheduleAllDays() {
	testID := GenerateTestID("schedule_all")
	entityID := BuildEntityID("schedule", testID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create schedule for all days
	config := homeassistant.HelperConfig{
		Platform: "schedule",
		ID:       testID,
		Config: map[string]any{
			"name": "Test All Days Schedule",
			"monday":    []map[string]any{{"from": "00:00:00", "to": "23:59:59"}},
			"tuesday":   []map[string]any{{"from": "00:00:00", "to": "23:59:59"}},
			"wednesday": []map[string]any{{"from": "00:00:00", "to": "23:59:59"}},
			"thursday":  []map[string]any{{"from": "00:00:00", "to": "23:59:59"}},
			"friday":    []map[string]any{{"from": "00:00:00", "to": "23:59:59"}},
			"saturday":  []map[string]any{{"from": "00:00:00", "to": "23:59:59"}},
			"sunday":    []map[string]any{{"from": "00:00:00", "to": "23:59:59"}},
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create schedule")

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Schedule did not appear")

	// This schedule should always be on
	s.Equal("on", entity.State, "24/7 schedule should be on")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}
