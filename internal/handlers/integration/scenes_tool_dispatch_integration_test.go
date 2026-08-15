//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type SceneToolDispatchTestSuite struct {
	SceneTestSuite
}

func TestSceneToolDispatch(t *testing.T) {
	suite.Run(t, new(SceneToolDispatchTestSuite))
}

// TestSceneUpdateViaTool covers a bug where manage_scene update used to rebuild the config from
// GetState (entity attributes only) instead of reading the stored config first, so a partial
// update (name only) silently wiped every other field - entities, icon - because Home
// Assistant's config API replaces the whole scenes.yaml entry rather than merging
// (EditSceneConfigView._write_value: data[index] = updated_value). This test updates only
// "name" via the real manage_scene tool and confirms entities and icon survive.
func (s *SceneToolDispatchTestSuite) TestSceneUpdateViaTool() {
	target1Name := GenerateTestID("scene_td_t1")
	target2Name := GenerateTestID("scene_td_t2")
	target1EntityID := BuildEntityID("input_boolean", target1Name)
	target2EntityID := BuildEntityID("input_boolean", target2Name)
	sceneID := GenerateTestID("scene_td")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScene(s.Context(), sceneID)
		_ = s.Client().DeleteHelper(s.Context(), target1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), target2EntityID)
	})

	for _, cfg := range []homeassistant.HelperConfig{
		{Platform: "input_boolean", Config: map[string]any{"name": target1Name, "initial": false}},
		{Platform: "input_boolean", Config: map[string]any{"name": target2Name, "initial": false}},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err, "Failed to create target helper")
	}
	_, err := s.WaitForEntity(target1EntityID, 10*time.Second)
	s.Require().NoError(err)
	_, err = s.WaitForEntity(target2EntityID, 10*time.Second)
	s.Require().NoError(err)

	// IMPORTANT: Name must match sceneID because HA derives entity_id from name (slugified).
	sceneConfig := homeassistant.SceneConfig{
		Name: sceneID,
		Icon: "mdi:movie-open",
		Entities: map[string]homeassistant.SceneState{
			target1EntityID: {State: "on"},
			target2EntityID: {State: "off"},
		},
		// Metadata exercises the companion fix for that bug: buildSceneData's new "metadata" key
		// (rest_client.go) previously had only mock coverage - this is the sole live-HA
		// assertion that HA's config API round-trips it, mirroring the shape HA's own scene
		// editor writes ({"<entity_id>": {"entity_only": true}}).
		Metadata: map[string]any{
			target1EntityID: map[string]any{"entity_only": true},
		},
	}
	err = s.Client().CreateScene(s.Context(), sceneID, sceneConfig)
	s.Require().NoError(err, "Failed to create scene")

	_, err = s.WaitForScene(sceneID, 10*time.Second)
	s.Require().NoError(err, "Scene did not appear")

	// The action under test: update (name only) via the real manage_scene tool.
	result := s.CallTool("manage_scene", map[string]any{
		"action":   "update",
		"scene_id": sceneID,
		"name":     sceneID + "_renamed",
	})
	s.Require().False(result.IsError, "manage_scene update should succeed, got: %s", resultText(result))

	updated, err := s.Client().GetScene(s.Context(), sceneID)
	s.Require().NoError(err)
	s.Require().NotNil(updated.Config)
	s.Equal(sceneID+"_renamed", updated.Config.Name)
	s.Equal("mdi:movie-open", updated.Config.Icon, "icon must survive an update that only touches name")
	s.Require().Len(updated.Config.Entities, 2, "entities must survive an update that only touches name")

	t1, ok := updated.Config.Entities[target1EntityID]
	s.Require().True(ok, "target1 entity must survive the update")
	s.Equal("on", t1.State)

	t2, ok := updated.Config.Entities[target2EntityID]
	s.Require().True(ok, "target2 entity must survive the update")
	s.Equal("off", t2.State)

	s.Require().NotNil(updated.Config.Metadata, "metadata must survive an update that only touches name")
	entry, ok := updated.Config.Metadata[target1EntityID].(map[string]any)
	s.Require().True(ok, "metadata entry for target1 must round-trip as an object, got: %#v", updated.Config.Metadata[target1EntityID])
	s.Equal(true, entry["entity_only"])
}
