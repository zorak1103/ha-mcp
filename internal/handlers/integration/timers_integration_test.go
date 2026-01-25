//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type TimerIntegrationTestSuite struct {
	HelperTestSuite
}

func TestTimerIntegration(t *testing.T) {
	suite.Run(t, new(TimerIntegrationTestSuite))
}

func (s *TimerIntegrationTestSuite) TestTimerLifecycle() {
	testName := GenerateTestID("timer")
	entityID := BuildEntityID("timer", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create timer - entity ID is derived from name
	config := homeassistant.HelperConfig{
		Platform: "timer",
		Config: map[string]any{
			"name":     testName,
			"duration": "00:01:00", // 1 minute
			"icon":     "mdi:timer",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create timer")

	// Wait for entity to appear
	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Timer did not appear")
	s.Equal("idle", entity.State, "Initial state should be idle")

	// Test start
	_, err = s.Client().CallService(s.Context(), "timer", "start", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to start timer")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("active", entity.State, "State should be active after start")

	// Test pause
	_, err = s.Client().CallService(s.Context(), "timer", "pause", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to pause timer")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("paused", entity.State, "State should be paused after pause")

	// Test resume (start on paused timer)
	_, err = s.Client().CallService(s.Context(), "timer", "start", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to resume timer")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("active", entity.State, "State should be active after resume")

	// Test cancel
	_, err = s.Client().CallService(s.Context(), "timer", "cancel", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to cancel timer")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("idle", entity.State, "State should be idle after cancel")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err, "Failed to delete timer")

	err = s.WaitForEntityGone(entityID, 5*time.Second)
	s.Require().NoError(err, "Timer should be deleted")
}

func (s *TimerIntegrationTestSuite) TestTimerFinish() {
	testName := GenerateTestID("timer_fin")
	entityID := BuildEntityID("timer", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create timer
	config := homeassistant.HelperConfig{
		Platform: "timer",
		Config: map[string]any{
			"name":     testName,
			"duration": "00:05:00",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create timer")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Timer did not appear")

	// Start timer
	_, err = s.Client().CallService(s.Context(), "timer", "start", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err)

	time.Sleep(200 * time.Millisecond)

	// Test finish (immediately finish timer)
	_, err = s.Client().CallService(s.Context(), "timer", "finish", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to finish timer")

	time.Sleep(200 * time.Millisecond)
	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("idle", entity.State, "State should be idle after finish")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

func (s *TimerIntegrationTestSuite) TestTimerChange() {
	testName := GenerateTestID("timer_chg")
	entityID := BuildEntityID("timer", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create timer
	config := homeassistant.HelperConfig{
		Platform: "timer",
		Config: map[string]any{
			"name":     testName,
			"duration": "00:01:00",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create timer")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Timer did not appear")

	// Start timer
	_, err = s.Client().CallService(s.Context(), "timer", "start", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err)

	time.Sleep(200 * time.Millisecond)

	// Test change (add time)
	_, err = s.Client().CallService(s.Context(), "timer", "change", map[string]any{
		"entity_id": entityID,
		"duration":  "00:00:30", // Add 30 seconds
	})
	s.Require().NoError(err, "Failed to change timer")

	time.Sleep(200 * time.Millisecond)
	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("active", entity.State, "Timer should still be active after change")

	// Cancel and cleanup
	_, _ = s.Client().CallService(s.Context(), "timer", "cancel", map[string]any{
		"entity_id": entityID,
	})
	time.Sleep(100 * time.Millisecond)

	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

func (s *TimerIntegrationTestSuite) TestTimerWithCustomDuration() {
	testName := GenerateTestID("timer_dur")
	entityID := BuildEntityID("timer", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create timer with default duration
	config := homeassistant.HelperConfig{
		Platform: "timer",
		Config: map[string]any{
			"name":     testName,
			"duration": "00:10:00",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create timer")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Timer did not appear")

	// Start timer with custom duration (override default)
	_, err = s.Client().CallService(s.Context(), "timer", "start", map[string]any{
		"entity_id": entityID,
		"duration":  "00:00:30", // Start with 30 seconds instead of default 10 minutes
	})
	s.Require().NoError(err, "Failed to start timer with custom duration")

	time.Sleep(200 * time.Millisecond)
	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("active", entity.State)

	// Cancel and cleanup
	_, _ = s.Client().CallService(s.Context(), "timer", "cancel", map[string]any{
		"entity_id": entityID,
	})
	time.Sleep(100 * time.Millisecond)

	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}
