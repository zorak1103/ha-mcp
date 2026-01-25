//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type InputTextIntegrationTestSuite struct {
	HelperTestSuite
}

func TestInputTextIntegration(t *testing.T) {
	suite.Run(t, new(InputTextIntegrationTestSuite))
}

func (s *InputTextIntegrationTestSuite) TestInputTextLifecycle() {
	testName := GenerateTestID("input_txt")
	entityID := BuildEntityID("input_text", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_text - entity ID is derived from name
	config := homeassistant.HelperConfig{
		Platform: "input_text",
		Config: map[string]any{
			"name":    testName,
			"initial": "initial value",
			"min":     0,
			"max":     100,
			"mode":    "text",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_text")

	// Wait for entity to appear
	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input text did not appear")
	s.Equal("initial value", entity.State, "Initial state should be 'initial value'")

	// Test set_value
	_, err = s.Client().CallService(s.Context(), "input_text", "set_value", map[string]any{
		"entity_id": entityID,
		"value":     "new value",
	})
	s.Require().NoError(err, "Failed to set input_text value")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("new value", entity.State, "State should be 'new value' after set_value")

	// Test with special characters
	_, err = s.Client().CallService(s.Context(), "input_text", "set_value", map[string]any{
		"entity_id": entityID,
		"value":     "Hello, World! 你好",
	})
	s.Require().NoError(err, "Failed to set input_text with special characters")

	time.Sleep(200 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("Hello, World! 你好", entity.State, "State should handle special characters")

	// Test delete
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err, "Failed to delete input_text")

	err = s.WaitForEntityGone(entityID, 5*time.Second)
	s.Require().NoError(err, "Input text should be deleted")
}

func (s *InputTextIntegrationTestSuite) TestInputTextPasswordMode() {
	testName := GenerateTestID("input_txt_pwd")
	entityID := BuildEntityID("input_text", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_text in password mode
	config := homeassistant.HelperConfig{
		Platform: "input_text",
		Config: map[string]any{
			"name":    testName,
			"initial": "",
			"min":     0,
			"max":     64,
			"mode":    "password",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_text")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input text did not appear")

	// Set a password value
	_, err = s.Client().CallService(s.Context(), "input_text", "set_value", map[string]any{
		"entity_id": entityID,
		"value":     "secret123",
	})
	s.Require().NoError(err)

	time.Sleep(200 * time.Millisecond)
	entity, err := s.Client().GetState(s.Context(), entityID)
	s.Require().NoError(err)
	s.Equal("secret123", entity.State, "Password value should be stored")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

func (s *InputTextIntegrationTestSuite) TestInputTextWithPattern() {
	testName := GenerateTestID("input_txt_pat")
	entityID := BuildEntityID("input_text", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_text with pattern
	config := homeassistant.HelperConfig{
		Platform: "input_text",
		Config: map[string]any{
			"name":    testName,
			"initial": "test@example.com",
			"min":     5,
			"max":     100,
			"pattern": "[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}",
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create input_text")

	entity, err := s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Input text did not appear")
	s.Equal("test@example.com", entity.State)

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}
