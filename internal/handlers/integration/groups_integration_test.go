//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/stretchr/testify/suite"
)

type GroupIntegrationTestSuite struct {
	HelperTestSuite
}

func TestGroupIntegration(t *testing.T) {
	suite.Run(t, new(GroupIntegrationTestSuite))
}

func (s *GroupIntegrationTestSuite) TestGroupLifecycle() {
	// First, create some input_booleans to group
	bool1ID := GenerateTestID("grp_bool1")
	bool2ID := GenerateTestID("grp_bool2")
	bool1EntityID := BuildEntityID("input_boolean", bool1ID)
	bool2EntityID := BuildEntityID("input_boolean", bool2ID)
	groupID := GenerateTestID("group")
	groupEntityID := BuildEntityID("group", groupID)

	// Register cleanup for all entities
	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
		_ = s.Client().DeleteHelper(s.Context(), bool1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), bool2EntityID)
	})

	// Create input_booleans
	for _, cfg := range []homeassistant.HelperConfig{
		{
			Platform: "input_boolean",
			ID:       bool1ID,
			Config:   map[string]any{"name": "Group Bool 1", "initial": false},
		},
		{
			Platform: "input_boolean",
			ID:       bool2ID,
			Config:   map[string]any{"name": "Group Bool 2", "initial": false},
		},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err, "Failed to create input_boolean")
	}

	_, err := s.WaitForEntity(bool1EntityID, 5*time.Second)
	s.Require().NoError(err)
	_, err = s.WaitForEntity(bool2EntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create group
	groupConfig := homeassistant.HelperConfig{
		Platform: "group",
		ID:       groupID,
		Config: map[string]any{
			"name":     "Test Group",
			"entities": []string{bool1EntityID, bool2EntityID},
			"all":      false, // Group is on if ANY member is on
		},
	}

	err = s.Client().CreateHelper(s.Context(), groupConfig)
	s.Require().NoError(err, "Failed to create group")

	entity, err := s.WaitForEntity(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Group did not appear")
	s.Equal("off", entity.State, "Group should be off when all members are off")

	// Turn on one member
	_, err = s.Client().CallService(s.Context(), "input_boolean", "turn_on", map[string]any{
		"entity_id": bool1EntityID,
	})
	s.Require().NoError(err)

	time.Sleep(300 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), groupEntityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "Group should be on when any member is on")

	// Turn off the member
	_, err = s.Client().CallService(s.Context(), "input_boolean", "turn_off", map[string]any{
		"entity_id": bool1EntityID,
	})
	s.Require().NoError(err)

	time.Sleep(300 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), groupEntityID)
	s.Require().NoError(err)
	s.Equal("off", entity.State, "Group should be off when all members are off")

	// Test delete group
	err = s.Client().DeleteHelper(s.Context(), groupEntityID)
	s.Require().NoError(err, "Failed to delete group")

	err = s.WaitForEntityGone(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Group should be deleted")

	// Cleanup input_booleans
	_ = s.Client().DeleteHelper(s.Context(), bool1EntityID)
	_ = s.Client().DeleteHelper(s.Context(), bool2EntityID)
}

func (s *GroupIntegrationTestSuite) TestGroupWithAllMode() {
	// Create input_booleans to group
	bool1ID := GenerateTestID("grpall_b1")
	bool2ID := GenerateTestID("grpall_b2")
	bool1EntityID := BuildEntityID("input_boolean", bool1ID)
	bool2EntityID := BuildEntityID("input_boolean", bool2ID)
	groupID := GenerateTestID("group_all")
	groupEntityID := BuildEntityID("group", groupID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
		_ = s.Client().DeleteHelper(s.Context(), bool1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), bool2EntityID)
	})

	// Create input_booleans
	for _, cfg := range []homeassistant.HelperConfig{
		{
			Platform: "input_boolean",
			ID:       bool1ID,
			Config:   map[string]any{"name": "All Group Bool 1", "initial": false},
		},
		{
			Platform: "input_boolean",
			ID:       bool2ID,
			Config:   map[string]any{"name": "All Group Bool 2", "initial": false},
		},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err)
	}

	_, _ = s.WaitForEntity(bool1EntityID, 5*time.Second)
	_, _ = s.WaitForEntity(bool2EntityID, 5*time.Second)

	// Create group with all=true (group is on only when ALL members are on)
	groupConfig := homeassistant.HelperConfig{
		Platform: "group",
		ID:       groupID,
		Config: map[string]any{
			"name":     "Test All Group",
			"entities": []string{bool1EntityID, bool2EntityID},
			"all":      true,
		},
	}

	err := s.Client().CreateHelper(s.Context(), groupConfig)
	s.Require().NoError(err, "Failed to create group")

	entity, err := s.WaitForEntity(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Group did not appear")
	s.Equal("off", entity.State, "Group should be off initially")

	// Turn on one member - group should still be off
	_, err = s.Client().CallService(s.Context(), "input_boolean", "turn_on", map[string]any{
		"entity_id": bool1EntityID,
	})
	s.Require().NoError(err)

	time.Sleep(300 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), groupEntityID)
	s.Require().NoError(err)
	s.Equal("off", entity.State, "Group should still be off when only one member is on (all=true)")

	// Turn on second member - group should now be on
	_, err = s.Client().CallService(s.Context(), "input_boolean", "turn_on", map[string]any{
		"entity_id": bool2EntityID,
	})
	s.Require().NoError(err)

	time.Sleep(300 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), groupEntityID)
	s.Require().NoError(err)
	s.Equal("on", entity.State, "Group should be on when all members are on")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
	_ = s.Client().DeleteHelper(s.Context(), bool1EntityID)
	_ = s.Client().DeleteHelper(s.Context(), bool2EntityID)
}

func (s *GroupIntegrationTestSuite) TestGroupSetEntities() {
	// Create input_booleans
	bool1ID := GenerateTestID("grpset_b1")
	bool2ID := GenerateTestID("grpset_b2")
	bool3ID := GenerateTestID("grpset_b3")
	bool1EntityID := BuildEntityID("input_boolean", bool1ID)
	bool2EntityID := BuildEntityID("input_boolean", bool2ID)
	bool3EntityID := BuildEntityID("input_boolean", bool3ID)
	groupID := GenerateTestID("group_set")
	groupEntityID := BuildEntityID("group", groupID)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
		_ = s.Client().DeleteHelper(s.Context(), bool1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), bool2EntityID)
		_ = s.Client().DeleteHelper(s.Context(), bool3EntityID)
	})

	// Create input_booleans
	for _, cfg := range []homeassistant.HelperConfig{
		{Platform: "input_boolean", ID: bool1ID, Config: map[string]any{"name": "Set Bool 1"}},
		{Platform: "input_boolean", ID: bool2ID, Config: map[string]any{"name": "Set Bool 2"}},
		{Platform: "input_boolean", ID: bool3ID, Config: map[string]any{"name": "Set Bool 3"}},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err)
	}

	_, _ = s.WaitForEntity(bool1EntityID, 5*time.Second)
	_, _ = s.WaitForEntity(bool2EntityID, 5*time.Second)
	_, _ = s.WaitForEntity(bool3EntityID, 5*time.Second)

	// Create group with first two members
	groupConfig := homeassistant.HelperConfig{
		Platform: "group",
		ID:       groupID,
		Config: map[string]any{
			"name":     "Test Set Group",
			"entities": []string{bool1EntityID, bool2EntityID},
		},
	}

	err := s.Client().CreateHelper(s.Context(), groupConfig)
	s.Require().NoError(err, "Failed to create group")

	entity, err := s.WaitForEntity(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "Group did not appear")

	// Verify initial entities
	entityList, ok := entity.Attributes["entity_id"].([]any)
	s.True(ok, "entity_id attribute should exist")
	s.Len(entityList, 2, "Should have 2 entities initially")

	// Test set (add entity)
	_, err = s.Client().CallService(s.Context(), "group", "set", map[string]any{
		"object_id":   groupID,
		"add_entities": []string{bool3EntityID},
	})
	s.Require().NoError(err, "Failed to add entity to group")

	time.Sleep(300 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), groupEntityID)
	s.Require().NoError(err)

	entityList, ok = entity.Attributes["entity_id"].([]any)
	s.True(ok)
	s.Len(entityList, 3, "Should have 3 entities after add")

	// Test set (remove entity)
	_, err = s.Client().CallService(s.Context(), "group", "set", map[string]any{
		"object_id":      groupID,
		"remove_entities": []string{bool1EntityID},
	})
	s.Require().NoError(err, "Failed to remove entity from group")

	time.Sleep(300 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), groupEntityID)
	s.Require().NoError(err)

	entityList, ok = entity.Attributes["entity_id"].([]any)
	s.True(ok)
	s.Len(entityList, 2, "Should have 2 entities after remove")

	// Cleanup
	_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
	_ = s.Client().DeleteHelper(s.Context(), bool1EntityID)
	_ = s.Client().DeleteHelper(s.Context(), bool2EntityID)
	_ = s.Client().DeleteHelper(s.Context(), bool3EntityID)
}
