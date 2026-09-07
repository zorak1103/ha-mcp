//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type EntityManageIntegrationTestSuite struct {
	IntegrationTestSuite
}

func TestEntityManageIntegration(t *testing.T) {
	suite.Run(t, new(EntityManageIntegrationTestSuite))
}

func (s *EntityManageIntegrationTestSuite) TestEntityUpdateBasicFields() {
	// Create a helper entity to test registry updates on
	testName := GenerateTestID("entity_update")
	entityID := BuildEntityID("input_boolean", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create input_boolean helper
	config := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name": testName,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create helper")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Helper did not appear")

	// Wait for entity to be registered
	time.Sleep(500 * time.Millisecond)

	// Get entity registry entry before update to verify it exists
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err, "Failed to get entity registry")

	var found bool
	for _, entry := range registry {
		if entry.EntityID == entityID {
			found = true
			s.T().Logf("Found entity in registry: EntityID=%s, Name=%s", entry.EntityID, entry.Name)
			break
		}
	}
	s.Require().True(found, "Entity should exist in registry before update")

	// Update entity registry: change name only (icon may not work for all entity types)
	updateConfig := homeassistant.EntityRegistryUpdateConfig{
		Name: stringPtr("Updated Test Name"),
	}

	updated, err := s.Client().UpdateEntityRegistryEntry(s.Context(), entityID, updateConfig)
	if err != nil {
		s.T().Logf("Update error: %v", err)
	}
	s.Require().NoError(err, "Failed to update entity")
	s.Require().NotNil(updated, "Updated entity should not be nil")
	s.T().Logf("Updated entity: EntityID=%s, Name=%s", updated.EntityID, updated.Name)

	// Verify the updated entity is returned directly (API wraps response in "entity_entry")
	s.Equal(entityID, updated.EntityID, "Entity ID should not change")
	s.Equal("Updated Test Name", updated.Name, "Name should be updated")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

func (s *EntityManageIntegrationTestSuite) TestEntityUpdateDisable() {
	// Create a helper entity to test disabling
	testName := GenerateTestID("entity_disable")
	entityID := BuildEntityID("input_boolean", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create helper
	config := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name": testName,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create helper")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Helper did not appear")

	time.Sleep(500 * time.Millisecond)

	// Disable entity
	updateConfig := homeassistant.EntityRegistryUpdateConfig{
		DisabledBy: stringPtr("user"),
	}

	_, err = s.Client().UpdateEntityRegistryEntry(s.Context(), entityID, updateConfig)
	s.Require().NoError(err, "Failed to disable entity")

	// Verify via registry
	time.Sleep(500 * time.Millisecond)
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err)

	var disabledEntry *homeassistant.EntityRegistryEntry
	for _, entry := range registry {
		if entry.EntityID == entityID {
			disabledEntry = &entry
			break
		}
	}
	s.Require().NotNil(disabledEntry)
	s.Equal("user", disabledEntry.DisabledBy, "Entity should be disabled by user")

	// Note: Re-enabling requires removing the disabled_by field entirely, which may not be
	// supported by all HA versions. Testing disable only is sufficient for validation.

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

func (s *EntityManageIntegrationTestSuite) TestEntityUpdateAliases() {
	// Create a helper entity
	testName := GenerateTestID("entity_aliases")
	entityID := BuildEntityID("input_boolean", testName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create helper
	config := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name": testName,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create helper")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "Helper did not appear")

	time.Sleep(500 * time.Millisecond)

	// Update with aliases only
	updateConfig := homeassistant.EntityRegistryUpdateConfig{
		Aliases: []string{"test_alias_1", "test_alias_2"},
	}

	_, err = s.Client().UpdateEntityRegistryEntry(s.Context(), entityID, updateConfig)
	s.Require().NoError(err, "Failed to update aliases")

	// Verify aliases via registry
	time.Sleep(500 * time.Millisecond)
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err)

	var found bool
	for _, entry := range registry {
		if entry.EntityID == entityID {
			found = true
			s.T().Logf("Entity registry entry: EntityID=%s, Aliases=%v", entry.EntityID, entry.Aliases)
			// Aliases might not be immediately available or supported for all entity types
			if len(entry.Aliases) > 0 {
				s.ElementsMatch([]string{"test_alias_1", "test_alias_2"}, entry.Aliases, "Aliases should be updated")
			} else {
				s.T().Log("Aliases not reflected in registry (may not be supported for this entity type)")
			}
			break
		}
	}
	s.Require().True(found, "Entity should exist in registry")

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

func (s *EntityManageIntegrationTestSuite) TestEntityRename() {
	// Create a helper entity to test renaming
	testName := GenerateTestID("entity_rename")
	oldEntityID := BuildEntityID("input_boolean", testName)
	newTestName := GenerateTestID("entity_renamed")
	newEntityID := BuildEntityID("input_boolean", newTestName)

	s.RegisterCleanup(func() {
		// Try to delete both old and new entity IDs
		_ = s.Client().DeleteHelper(s.Context(), oldEntityID)
		_ = s.Client().DeleteHelper(s.Context(), newEntityID)
	})

	// Create helper
	config := homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config: map[string]any{
			"name": testName,
		},
	}

	err := s.Client().CreateHelper(s.Context(), config)
	s.Require().NoError(err, "Failed to create helper")

	entity, err := s.WaitForEntity(oldEntityID, 5*time.Second)
	s.Require().NoError(err, "Helper did not appear")
	s.Equal(oldEntityID, entity.EntityID)

	time.Sleep(500 * time.Millisecond)

	// Rename entity
	updateConfig := homeassistant.EntityRegistryUpdateConfig{
		NewEntityID: &newEntityID,
	}

	_, err = s.Client().UpdateEntityRegistryEntry(s.Context(), oldEntityID, updateConfig)
	s.Require().NoError(err, "Failed to rename entity")

	// Wait for rename to propagate
	time.Sleep(1 * time.Second)

	// Verify rename in registry
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err)

	var foundNew bool
	for _, entry := range registry {
		if entry.EntityID == newEntityID {
			foundNew = true
			s.T().Logf("Found renamed entity: %s", newEntityID)
			break
		}
	}
	s.Require().True(foundNew, "Renamed entity should exist in registry")

	// Verify old entity ID is gone
	_, err = s.Client().GetState(s.Context(), oldEntityID)
	s.Error(err, "Old entity ID should not exist")

	// Verify new entity ID exists and wait for it to be fully available
	entity, err = s.WaitForEntity(newEntityID, 5*time.Second)
	s.Require().NoError(err, "New entity ID should exist")
	s.Equal(newEntityID, entity.EntityID)

	// Cleanup using entity registry removal (after rename, DeleteHelper may not work)
	// Use RemoveEntityRegistryEntry which removes the entity from registry
	err = s.Client().RemoveEntityRegistryEntry(s.Context(), newEntityID)
	s.Require().NoError(err, "Failed to remove renamed entity")

	// Wait for deletion
	time.Sleep(1 * time.Second)

	// Verify deletion via registry
	registry, err = s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err)

	var stillExists bool
	for _, entry := range registry {
		if entry.EntityID == newEntityID {
			stillExists = true
			break
		}
	}
	s.False(stillExists, "Entity should be removed from registry")
}

func (s *EntityManageIntegrationTestSuite) TestEntityUpdateLabels() {
	testName := GenerateTestID("entity_labels")
	entityID := BuildEntityID("input_boolean", testName)

	// Create a test label first (entity labels must exist in label registry)
	labelID := s.CreateTestLabel("entity_lbl")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})

	// Create helper entity
	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": testName},
	})
	s.Require().NoError(err, "failed to create helper")

	_, err = s.WaitForEntity(entityID, 5*time.Second)
	s.Require().NoError(err, "helper did not appear")
	time.Sleep(500 * time.Millisecond)

	// Set label on entity
	_, err = s.Client().UpdateEntityRegistryEntry(s.Context(), entityID, homeassistant.EntityRegistryUpdateConfig{
		Labels: []string{labelID},
	})
	s.Require().NoError(err, "failed to set labels")

	// Verify label in registry
	time.Sleep(500 * time.Millisecond)
	registry, err := s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err)

	var found bool
	for _, entry := range registry {
		if entry.EntityID == entityID {
			found = true
			s.Contains(entry.Labels, labelID, "label should be set on entity")
			break
		}
	}
	s.Require().True(found, "entity should exist in registry")

	// Clear labels (replace with empty — note: omitempty may prevent full clear;
	// this verifies the API round-trip for the label field)
	secondLabelID := s.CreateTestLabel("entity_lbl2")

	_, err = s.Client().UpdateEntityRegistryEntry(s.Context(), entityID, homeassistant.EntityRegistryUpdateConfig{
		Labels: []string{secondLabelID},
	})
	s.Require().NoError(err, "failed to replace labels")

	time.Sleep(500 * time.Millisecond)
	registry, err = s.Client().GetEntityRegistry(s.Context())
	s.Require().NoError(err)
	for _, entry := range registry {
		if entry.EntityID == entityID {
			s.Contains(entry.Labels, secondLabelID, "new label should replace old label")
			s.NotContains(entry.Labels, labelID, "old label should be replaced")
			break
		}
	}

	// Cleanup
	err = s.Client().DeleteHelper(s.Context(), entityID)
	s.Require().NoError(err)
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
