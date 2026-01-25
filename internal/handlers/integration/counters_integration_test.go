//go:build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/stretchr/testify/suite"
)

type CounterIntegrationTestSuite struct {
	HelperTestSuite
}

func TestCounterIntegration(t *testing.T) {
	suite.Run(t, new(CounterIntegrationTestSuite))
}

func (s *CounterIntegrationTestSuite) TestCounterLifecycle() {
	// For counters, the entity_id is generated from the name (lowercased, spaces to underscores)
	// So we use the testID as the name to ensure unique entity IDs
	testName := GenerateTestID("counter")
	entityID := BuildEntityID("counter", testName)

	// Register cleanup
	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create counter - note: ID field is ignored for counters, entity ID comes from name
	config := homeassistant.HelperConfig{
		Platform: "counter",
		Config: map[string]any{
			"name":    testName,
			"initial": 0,
			"step":    1,
			"minimum": 0,
			"maximum": 100,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create counter")

	// Wait for entity to appear
	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Counter did not appear")
	s.Equal("0", entity.State, "Initial state should be 0")

	// Test increment
	_, err = s.Client().CallService(s.Context(), "counter", "increment", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to increment counter")

	time.Sleep(200 * time.Millisecond) // Allow state to update
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err, "Failed to get state after increment")
	s.Equal("1", entity.State, "State should be 1 after increment")

	// Test decrement
	_, err = s.Client().CallService(s.Context(), "counter", "decrement", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to decrement counter")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err, "Failed to get state after decrement")
	s.Equal("0", entity.State, "State should be 0 after decrement")

	// Test set_value
	_, err = s.Client().CallService(s.Context(), "counter", "set_value", map[string]any{
		"entity_id": entityID,
		"value":     42,
	})
	s.Require().NoError(err, "Failed to set counter value")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err, "Failed to get state after set_value")
	s.Equal("42", entity.State, "State should be 42 after set_value")

	// Test reset
	_, err = s.Client().CallService(s.Context(), "counter", "reset", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err, "Failed to reset counter")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err, "Failed to get state after reset")
	s.Equal("0", entity.State, "State should be 0 after reset")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err, "Failed to delete counter")

	// Verify deletion
	err = s.WaitForEntityGone(entityID, 5*time.Second)
	s.Require().NoError(err, "Counter should be deleted")
}

func (s *CounterIntegrationTestSuite) TestCounterWithStepValue() {
	testName := GenerateTestID("counter_step")
	entityID := BuildEntityID("counter", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create counter with step of 5
	config := homeassistant.HelperConfig{
		Platform: "counter",
		Config: map[string]any{
			"name":    testName,
			"initial": 10,
			"step":    5,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create counter")

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Counter did not appear")
	s.Equal("10", entity.State, "Initial state should be 10")

	// Increment should add 5
	_, err = s.Client().CallService(s.Context(), "counter", "increment", map[string]any{
		"entity_id": entityID,
	})
	s.Require().NoError(err)

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("15", entity.State, "State should be 15 after increment with step 5")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

func (s *CounterIntegrationTestSuite) TestCounterMinMax() {
	testName := GenerateTestID("counter_minmax")
	entityID := BuildEntityID("counter", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create counter with min/max
	config := homeassistant.HelperConfig{
		Platform: "counter",
		Config: map[string]any{
			"name":    testName,
			"initial": 5,
			"step":    1,
			"minimum": 0,
			"maximum": 10,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create counter")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Counter did not appear")

	// Try to increment beyond max
	for i := 0; i < 10; i++ {
		_, _ = s.Client().CallService(s.Context(), "counter", "increment", map[string]any{
			"entity_id": entityID,
		})
	}

	time.Sleep(300 * time.Millisecond)
	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	// Counter should be capped at max
	s.Equal("10", entity.State, fmt.Sprintf("Counter should be capped at maximum 10, got %s", entity.State))

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}
