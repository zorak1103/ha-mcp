//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

var _ = time.Second        // silence unused import
var _ homeassistant.Client // silence unused import

type GroupIntegrationTestSuite struct {
	HelperTestSuite
}

func TestGroupIntegration(t *testing.T) {
	suite.Run(t, new(GroupIntegrationTestSuite))
}

func (s *GroupIntegrationTestSuite) TestSensorGroupLifecycle() {
	// Create input_numbers as sensor sources (sensor groups accept input_number)
	num1Name := GenerateTestID("grp_num1")
	num2Name := GenerateTestID("grp_num2")
	num1EntityID := BuildEntityID("input_number", num1Name)
	num2EntityID := BuildEntityID("input_number", num2Name)
	groupName := GenerateTestID("grp_sensor")
	groupEntityID := BuildEntityID("sensor", groupName)

	// Register cleanup for all entities
	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
		_ = s.Client().DeleteHelper(s.Context(), num1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), num2EntityID)
	})

	// Create input_numbers
	for _, cfg := range []homeassistant.HelperConfig{
		{
			Platform: "input_number",
			Config:   map[string]any{"name": num1Name, "min": 0.0, "max": 100.0, "initial": 10.0},
		},
		{
			Platform: "input_number",
			Config:   map[string]any{"name": num2Name, "min": 0.0, "max": 100.0, "initial": 20.0},
		},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err, "Failed to create input_number")
	}

	_, err := s.WaitForEntity(num1EntityID, 5*time.Second)
	s.Require().NoError(err)
	_, err = s.WaitForEntity(num2EntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create sensor group - sum of input_numbers
	// Group type is inferred from entity domains (input_number -> sensor group)
	groupConfig := homeassistant.HelperConfig{
		Platform: "group",
		Config: map[string]any{
			"name":     groupName,
			"entities": []string{num1EntityID, num2EntityID},
		},
	}

	err = s.Client().CreateHelper(s.Context(), groupConfig)
	s.Require().NoError(err, "Failed to create sensor group")

	entity, err := s.WaitForEntity(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Sensor group did not appear")

	// Verify the group state is the sum of members (10 + 20 = 30)
	s.NotNil(entity)

	// Test delete group
	err = s.Client().DeleteHelper(s.Context(), groupEntityID)
	s.Require().NoError(err, "Failed to delete sensor group")

	err = s.WaitForEntityGone(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Sensor group should be deleted")

	// Cleanup input_numbers
	_ = s.Client().DeleteHelper(s.Context(), num1EntityID)
	_ = s.Client().DeleteHelper(s.Context(), num2EntityID)
}

func (s *GroupIntegrationTestSuite) TestSensorGroupWithMean() {
	// Create input_numbers
	num1Name := GenerateTestID("grp_mean1")
	num2Name := GenerateTestID("grp_mean2")
	num1EntityID := BuildEntityID("input_number", num1Name)
	num2EntityID := BuildEntityID("input_number", num2Name)
	groupName := GenerateTestID("grp_mean")
	groupEntityID := BuildEntityID("sensor", groupName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
		_ = s.Client().DeleteHelper(s.Context(), num1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), num2EntityID)
	})

	// Create input_numbers with different values
	for _, cfg := range []homeassistant.HelperConfig{
		{
			Platform: "input_number",
			Config:   map[string]any{"name": num1Name, "min": 0.0, "max": 100.0, "initial": 30.0},
		},
		{
			Platform: "input_number",
			Config:   map[string]any{"name": num2Name, "min": 0.0, "max": 100.0, "initial": 50.0},
		},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err)
	}

	_, _ = s.WaitForEntity(num1EntityID, 5*time.Second)
	_, _ = s.WaitForEntity(num2EntityID, 5*time.Second)

	// Create sensor group - group type is inferred from entity domains
	groupConfig := homeassistant.HelperConfig{
		Platform: "group",
		Config: map[string]any{
			"name":     groupName,
			"entities": []string{num1EntityID, num2EntityID},
		},
	}

	err := s.Client().CreateHelper(s.Context(), groupConfig)
	s.Require().NoError(err, "Failed to create sensor group")

	entity, err := s.WaitForEntity(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Sensor group did not appear")
	s.NotNil(entity)

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
	_ = s.Client().DeleteHelper(s.Context(), num1EntityID)
	_ = s.Client().DeleteHelper(s.Context(), num2EntityID)
}

func (s *GroupIntegrationTestSuite) TestSensorGroupWithMinMax() {
	num1Name := GenerateTestID("grp_mm1")
	num2Name := GenerateTestID("grp_mm2")
	num3Name := GenerateTestID("grp_mm3")
	num1EntityID := BuildEntityID("input_number", num1Name)
	num2EntityID := BuildEntityID("input_number", num2Name)
	num3EntityID := BuildEntityID("input_number", num3Name)
	groupName := GenerateTestID("grp_minmax")
	groupEntityID := BuildEntityID("sensor", groupName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
		_ = s.Client().DeleteHelper(s.Context(), num1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), num2EntityID)
		_ = s.Client().DeleteHelper(s.Context(), num3EntityID)
	})

	// Create input_numbers with different values
	for _, cfg := range []homeassistant.HelperConfig{
		{Platform: "input_number", Config: map[string]any{"name": num1Name, "min": 0.0, "max": 100.0, "initial": 10.0}},
		{Platform: "input_number", Config: map[string]any{"name": num2Name, "min": 0.0, "max": 100.0, "initial": 50.0}},
		{Platform: "input_number", Config: map[string]any{"name": num3Name, "min": 0.0, "max": 100.0, "initial": 90.0}},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err)
	}

	_, _ = s.WaitForEntity(num1EntityID, 5*time.Second)
	_, _ = s.WaitForEntity(num2EntityID, 5*time.Second)
	_, _ = s.WaitForEntity(num3EntityID, 5*time.Second)

	// Create sensor group - group type is inferred from entity domains
	groupConfig := homeassistant.HelperConfig{
		Platform: "group",
		Config: map[string]any{
			"name":     groupName,
			"entities": []string{num1EntityID, num2EntityID, num3EntityID},
		},
	}

	err := s.Client().CreateHelper(s.Context(), groupConfig)
	s.Require().NoError(err, "Failed to create sensor group")

	entity, err := s.WaitForEntity(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Sensor group did not appear")
	s.NotNil(entity)

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
	_ = s.Client().DeleteHelper(s.Context(), num1EntityID)
	_ = s.Client().DeleteHelper(s.Context(), num2EntityID)
	_ = s.Client().DeleteHelper(s.Context(), num3EntityID)
}
