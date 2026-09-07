//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type AreaIntegrationTestSuite struct {
	AreaTestSuite
}

func TestAreaIntegration(t *testing.T) {
	suite.Run(t, new(AreaIntegrationTestSuite))
}

func (s *AreaIntegrationTestSuite) TestAreaLifecycle() {
	areaName := GenerateTestID("area")

	s.RegisterCleanup(func() {
		areas, _ := s.Client().GetAreaRegistry(s.Context())
		for _, area := range areas {
			if area.Name == areaName {
				_ = s.Client().DeleteArea(s.Context(), area.AreaID)
			}
		}
	})

	// Create area
	areaConfig := homeassistant.AreaConfig{
		Name: areaName,
		Icon: "mdi:home",
	}

	created, err := s.Client().CreateArea(s.Context(), areaConfig)
	s.Require().NoError(err, "Failed to create area")
	s.Require().NotNil(created)
	s.Equal(areaName, created.Name)
	s.Equal("mdi:home", created.Icon)

	areaID := created.AreaID

	// Allow time for registry to update
	time.Sleep(500 * time.Millisecond)

	// Verify area appears in registry
	area, err := s.FindAreaByID(areaID)
	s.Require().NoError(err, "Area should appear in registry")
	s.Equal(areaName, area.Name)
	s.Equal("mdi:home", area.Icon)

	// Update area (name + icon)
	updatedName := GenerateTestID("area_updated")
	updateConfig := homeassistant.AreaConfig{
		Name: updatedName,
		Icon: "mdi:office",
	}

	updated, err := s.Client().UpdateArea(s.Context(), areaID, updateConfig)
	s.Require().NoError(err, "Failed to update area")
	s.Equal(updatedName, updated.Name)
	s.Equal("mdi:office", updated.Icon)

	time.Sleep(500 * time.Millisecond)

	// Verify update
	area, err = s.FindAreaByID(areaID)
	s.Require().NoError(err)
	s.Equal(updatedName, area.Name)
	s.Equal("mdi:office", area.Icon)

	// Delete area
	err = s.Client().DeleteArea(s.Context(), areaID)
	s.Require().NoError(err, "Failed to delete area")

	time.Sleep(500 * time.Millisecond)

	// Verify area is gone
	_, err = s.FindAreaByID(areaID)
	s.Error(err, "Area should be deleted from registry")
}

func (s *AreaIntegrationTestSuite) TestAreaWithAllFields() {
	areaName := GenerateTestID("area_full")
	aliases := []string{"test_alias_1", "test_alias_2"}
	labels := []string{s.CreateTestLabel("afull_l1"), s.CreateTestLabel("afull_l2")}

	s.RegisterCleanup(func() {
		areas, _ := s.Client().GetAreaRegistry(s.Context())
		for _, area := range areas {
			if area.Name == areaName {
				_ = s.Client().DeleteArea(s.Context(), area.AreaID)
			}
		}
	})

	// Create area with all fields (except floor_id to avoid dependency on floors)
	areaConfig := homeassistant.AreaConfig{
		Name:    areaName,
		Icon:    "mdi:test-tube",
		Picture: "/local/test_picture.png",
		Aliases: aliases,
		Labels:  labels,
	}

	created, err := s.Client().CreateArea(s.Context(), areaConfig)
	s.Require().NoError(err, "Failed to create area with all fields")
	s.Require().NotNil(created)

	areaID := created.AreaID

	time.Sleep(500 * time.Millisecond)

	// Verify all fields
	area, err := s.FindAreaByID(areaID)
	s.Require().NoError(err)
	s.Equal(areaName, area.Name)
	s.Equal("mdi:test-tube", area.Icon)
	s.Equal("/local/test_picture.png", area.Picture)
	s.ElementsMatch(aliases, area.Aliases)
	s.ElementsMatch(labels, area.Labels)

	// Cleanup
	_ = s.Client().DeleteArea(s.Context(), areaID)
}

func (s *AreaIntegrationTestSuite) TestAreaUpdatePartial() {
	areaName := GenerateTestID("area_partial")

	s.RegisterCleanup(func() {
		areas, _ := s.Client().GetAreaRegistry(s.Context())
		for _, area := range areas {
			if area.Name == areaName {
				_ = s.Client().DeleteArea(s.Context(), area.AreaID)
			}
		}
	})

	// Create area with icon
	areaConfig := homeassistant.AreaConfig{
		Name: areaName,
		Icon: "mdi:home",
	}

	created, err := s.Client().CreateArea(s.Context(), areaConfig)
	s.Require().NoError(err)

	areaID := created.AreaID

	time.Sleep(500 * time.Millisecond)

	// Update only aliases (icon should remain)
	updateConfig := homeassistant.AreaConfig{
		Aliases: []string{"new_alias"},
	}

	_, err = s.Client().UpdateArea(s.Context(), areaID, updateConfig)
	s.Require().NoError(err, "Failed to update area with partial config")

	time.Sleep(500 * time.Millisecond)

	// Verify icon unchanged, aliases updated
	area, err := s.FindAreaByID(areaID)
	s.Require().NoError(err)
	s.Equal("mdi:home", area.Icon, "Icon should remain unchanged")
	s.Contains(area.Aliases, "new_alias", "Aliases should be updated")

	// Cleanup
	_ = s.Client().DeleteArea(s.Context(), areaID)
}

func (s *AreaIntegrationTestSuite) TestAreaLabelAndAliasUpdate() {
	areaName := GenerateTestID("area_lblalias")
	labelA := s.CreateTestLabel("area_lbl_a")
	labelB := s.CreateTestLabel("area_lbl_b")
	labelC := s.CreateTestLabel("area_lbl_c")

	s.RegisterCleanup(func() {
		areas, _ := s.Client().GetAreaRegistry(s.Context())
		for _, area := range areas {
			if area.Name == areaName {
				_ = s.Client().DeleteArea(s.Context(), area.AreaID)
			}
		}
	})

	// Create area without labels or aliases
	created, err := s.Client().CreateArea(s.Context(), homeassistant.AreaConfig{
		Name: areaName,
		Icon: "mdi:home",
	})
	s.Require().NoError(err, "failed to create area")
	areaID := created.AreaID

	time.Sleep(500 * time.Millisecond)

	// Update: set labels and aliases
	initialLabels := []string{labelA, labelB}
	initialAliases := []string{"first alias", "second alias"}
	_, err = s.Client().UpdateArea(s.Context(), areaID, homeassistant.AreaConfig{
		Labels:  initialLabels,
		Aliases: initialAliases,
	})
	s.Require().NoError(err, "failed to set labels and aliases")

	time.Sleep(500 * time.Millisecond)
	area, err := s.FindAreaByID(areaID)
	s.Require().NoError(err)
	s.ElementsMatch(initialLabels, area.Labels, "initial labels should be set")
	s.ElementsMatch(initialAliases, area.Aliases, "initial aliases should be set")

	// Update: replace labels and aliases with new values
	newLabels := []string{labelC}
	newAliases := []string{"only alias"}
	_, err = s.Client().UpdateArea(s.Context(), areaID, homeassistant.AreaConfig{
		Labels:  newLabels,
		Aliases: newAliases,
	})
	s.Require().NoError(err, "failed to replace labels and aliases")

	time.Sleep(500 * time.Millisecond)
	area, err = s.FindAreaByID(areaID)
	s.Require().NoError(err)
	s.ElementsMatch(newLabels, area.Labels, "labels should be replaced")
	s.ElementsMatch(newAliases, area.Aliases, "aliases should be replaced")
	s.Equal("mdi:home", area.Icon, "icon should remain unchanged")

	// Cleanup
	_ = s.Client().DeleteArea(s.Context(), areaID)
}

func (s *AreaIntegrationTestSuite) TestMultipleAreas() {
	area1Name := GenerateTestID("area_1")
	area2Name := GenerateTestID("area_2")

	var area1ID, area2ID string

	s.RegisterCleanup(func() {
		if area1ID != "" {
			_ = s.Client().DeleteArea(s.Context(), area1ID)
		}
		if area2ID != "" {
			_ = s.Client().DeleteArea(s.Context(), area2ID)
		}
	})

	// Create first area
	config1 := homeassistant.AreaConfig{
		Name: area1Name,
		Icon: "mdi:numeric-1",
	}

	created1, err := s.Client().CreateArea(s.Context(), config1)
	s.Require().NoError(err, "Failed to create area 1")
	area1ID = created1.AreaID

	// Create second area
	config2 := homeassistant.AreaConfig{
		Name: area2Name,
		Icon: "mdi:numeric-2",
	}

	created2, err := s.Client().CreateArea(s.Context(), config2)
	s.Require().NoError(err, "Failed to create area 2")
	area2ID = created2.AreaID

	time.Sleep(500 * time.Millisecond)

	// Verify both areas exist in registry
	area1, err := s.FindAreaByID(area1ID)
	s.Require().NoError(err, "Area 1 should exist")
	s.Equal(area1Name, area1.Name)

	area2, err := s.FindAreaByID(area2ID)
	s.Require().NoError(err, "Area 2 should exist")
	s.Equal(area2Name, area2.Name)

	// Delete both areas
	err = s.Client().DeleteArea(s.Context(), area1ID)
	s.Require().NoError(err, "Failed to delete area 1")

	err = s.Client().DeleteArea(s.Context(), area2ID)
	s.Require().NoError(err, "Failed to delete area 2")

	time.Sleep(500 * time.Millisecond)

	// Verify both are gone
	_, err = s.FindAreaByID(area1ID)
	s.Error(err, "Area 1 should be deleted")

	_, err = s.FindAreaByID(area2ID)
	s.Error(err, "Area 2 should be deleted")
}

// TestAreaUnknownLabelRejected verifies that manage_area rejects a label id that does not
// exist in the label registry, rather than silently dropping it the way Home Assistant's
// config/area_registry/{create,update} APIs do (issue #242).
func (s *AreaIntegrationTestSuite) TestAreaUnknownLabelRejected() {
	areaName := GenerateTestID("area_badlbl")

	s.RegisterCleanup(func() {
		areas, _ := s.Client().GetAreaRegistry(s.Context())
		for _, area := range areas {
			if area.Name == areaName {
				_ = s.Client().DeleteArea(s.Context(), area.AreaID)
			}
		}
	})

	res := s.CallTool("manage_area", map[string]any{
		"action": "create",
		"name":   areaName,
		"labels": []any{"definitely_not_a_real_label_id"},
	})
	s.True(res.IsError, "expected manage_area to reject an unknown label")
	s.Contains(resultText(res), "unknown label(s)")
}
