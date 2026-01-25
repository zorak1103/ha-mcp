//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/stretchr/testify/suite"
)

type SceneIntegrationTestSuite struct {
	SceneTestSuite
}

func TestSceneIntegration(t *testing.T) {
	suite.Run(t, new(SceneIntegrationTestSuite))
}

func (s *SceneIntegrationTestSuite) TestSceneLifecycle() {
	// Create input_booleans for the scene to control
	target1ID := GenerateTestID("scene_t1")
	target2ID := GenerateTestID("scene_t2")
	target1EntityID := BuildEntityID("input_boolean", target1ID)
	target2EntityID := BuildEntityID("input_boolean", target2ID)
	sceneID := GenerateTestID("scene")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScene(s.Context(), sceneID)
		_ = s.Client().DeleteHelper(s.Context(), target1EntityID)
		_ = s.Client().DeleteHelper(s.Context(), target2EntityID)
	})

	// Create targets
	for _, cfg := range []homeassistant.HelperConfig{
		{Platform: "input_boolean", ID: target1ID, Config: map[string]any{"name": "Scene Target 1", "initial": false}},
		{Platform: "input_boolean", ID: target2ID, Config: map[string]any{"name": "Scene Target 2", "initial": true}},
	} {
		err := s.Client().CreateHelper(s.Context(), cfg)
		s.Require().NoError(err)
	}

	_, err := s.WaitForEntity(target1EntityID, 5*time.Second)
	s.Require().NoError(err)
	_, err = s.WaitForEntity(target2EntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create scene that sets target1 on and target2 off
	sceneConfig := homeassistant.SceneConfig{
		Name: "Test Scene",
		Entities: map[string]homeassistant.SceneState{
			target1EntityID: {
				State: "on",
			},
			target2EntityID: {
				State: "off",
			},
		},
	}

	err = s.Client().CreateScene(s.Context(), sceneID, sceneConfig)
	s.Require().NoError(err, "Failed to create scene")

	// Wait for scene to appear
	sceneEntityID := BuildEntityID("scene", sceneID)
	_, err = s.WaitForEntity(sceneEntityID, 5*time.Second)
	s.Require().NoError(err, "Scene did not appear")

	// Verify initial states
	target1, err := s.Client().GetState(s.Context(), target1EntityID)
	s.Require().NoError(err)
	s.Equal("off", target1.State, "Target 1 should be off initially")

	target2, err := s.Client().GetState(s.Context(), target2EntityID)
	s.Require().NoError(err)
	s.Equal("on", target2.State, "Target 2 should be on initially")

	// Activate the scene via service call
	_, err = s.Client().CallService(s.Context(), "scene", "turn_on", map[string]any{
		"entity_id": sceneEntityID,
	})
	s.Require().NoError(err, "Failed to activate scene")

	time.Sleep(500 * time.Millisecond)

	// Verify states changed
	target1, err = s.Client().GetState(s.Context(), target1EntityID)
	s.Require().NoError(err)
	s.Equal("on", target1.State, "Target 1 should be on after scene activation")

	target2, err = s.Client().GetState(s.Context(), target2EntityID)
	s.Require().NoError(err)
	s.Equal("off", target2.State, "Target 2 should be off after scene activation")

	// Test delete
	err = s.Client().DeleteScene(s.Context(), sceneID)
	s.Require().NoError(err, "Failed to delete scene")

	err = s.WaitForEntityGone(sceneEntityID, 5*time.Second)
	s.Require().NoError(err, "Scene should be deleted")

	// Cleanup helpers
	_ = s.Client().DeleteHelper(s.Context(), target1EntityID)
	_ = s.Client().DeleteHelper(s.Context(), target2EntityID)
}

func (s *SceneIntegrationTestSuite) TestSceneUpdate() {
	targetID := GenerateTestID("scene_upd_tgt")
	targetEntityID := BuildEntityID("input_boolean", targetID)
	sceneID := GenerateTestID("scene_update")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScene(s.Context(), sceneID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	// Create target
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		ID:       targetID,
		Config:   map[string]any{"name": "Update Scene Target", "initial": false},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create scene that turns on
	sceneConfig := homeassistant.SceneConfig{
		Name: "Original Scene",
		Entities: map[string]homeassistant.SceneState{
			targetEntityID: {
				State: "on",
			},
		},
	}

	err = s.Client().CreateScene(s.Context(), sceneID, sceneConfig)
	s.Require().NoError(err)

	sceneEntityID := BuildEntityID("scene", sceneID)
	entity, err := s.WaitForEntity(sceneEntityID, 5*time.Second)
	s.Require().NoError(err)

	friendlyName, _ := entity.Attributes["friendly_name"].(string)
	s.Equal("Original Scene", friendlyName)

	// Update scene to turn off instead
	updatedConfig := homeassistant.SceneConfig{
		Name: "Updated Scene",
		Entities: map[string]homeassistant.SceneState{
			targetEntityID: {
				State: "off",
			},
		},
	}

	err = s.Client().UpdateScene(s.Context(), sceneID, updatedConfig)
	s.Require().NoError(err, "Failed to update scene")

	time.Sleep(300 * time.Millisecond)
	entity, err = s.Client().GetState(s.Context(), sceneEntityID)
	s.Require().NoError(err)

	friendlyName, _ = entity.Attributes["friendly_name"].(string)
	s.Equal("Updated Scene", friendlyName, "Scene name should be updated")

	// Turn on target manually
	_, err = s.Client().CallService(s.Context(), "input_boolean", "turn_on", map[string]any{
		"entity_id": targetEntityID,
	})
	s.Require().NoError(err)
	time.Sleep(200 * time.Millisecond)

	// Activate updated scene - should turn off
	_, err = s.Client().CallService(s.Context(), "scene", "turn_on", map[string]any{
		"entity_id": sceneEntityID,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)

	target, err := s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("off", target.State, "Updated scene should turn off target")

	// Cleanup
	_ = s.Client().DeleteScene(s.Context(), sceneID)
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
}

func (s *SceneIntegrationTestSuite) TestSceneWithInputNumber() {
	targetID := GenerateTestID("scene_num_tgt")
	targetEntityID := BuildEntityID("input_number", targetID)
	sceneID := GenerateTestID("scene_number")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScene(s.Context(), sceneID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	// Create target input_number
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		ID:       targetID,
		Config: map[string]any{
			"name":    "Scene Number Target",
			"min":     0.0,
			"max":     100.0,
			"initial": 50.0,
		},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create scene that sets number to 75
	sceneConfig := homeassistant.SceneConfig{
		Name: "Number Scene",
		Entities: map[string]homeassistant.SceneState{
			targetEntityID: {
				State: "75.0",
			},
		},
	}

	err = s.Client().CreateScene(s.Context(), sceneID, sceneConfig)
	s.Require().NoError(err, "Failed to create scene")

	sceneEntityID := BuildEntityID("scene", sceneID)
	_, err = s.WaitForEntity(sceneEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Verify initial state
	target, err := s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("50.0", target.State, "Target should be 50.0 initially")

	// Activate scene
	_, err = s.Client().CallService(s.Context(), "scene", "turn_on", map[string]any{
		"entity_id": sceneEntityID,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)

	target, err = s.Client().GetState(s.Context(), targetEntityID)
	s.Require().NoError(err)
	s.Equal("75.0", target.State, "Target should be 75.0 after scene activation")

	// Cleanup
	_ = s.Client().DeleteScene(s.Context(), sceneID)
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
}

func (s *SceneIntegrationTestSuite) TestMultipleScenes() {
	targetID := GenerateTestID("scene_multi_tgt")
	targetEntityID := BuildEntityID("input_boolean", targetID)
	scene1ID := GenerateTestID("scene_a")
	scene2ID := GenerateTestID("scene_b")

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScene(s.Context(), scene1ID)
		_ = s.Client().DeleteScene(s.Context(), scene2ID)
		_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
	})

	// Create target
	targetConfig := homeassistant.HelperConfig{
		Platform: "input_boolean",
		ID:       targetID,
		Config:   map[string]any{"name": "Multi Scene Target", "initial": false},
	}
	err := s.Client().CreateHelper(s.Context(), targetConfig)
	s.Require().NoError(err)

	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err)

	// Create scene A (turns on)
	scene1Config := homeassistant.SceneConfig{
		Name: "Scene A - On",
		Entities: map[string]homeassistant.SceneState{
			targetEntityID: {
				State: "on",
			},
		},
	}
	err = s.Client().CreateScene(s.Context(), scene1ID, scene1Config)
	s.Require().NoError(err)

	// Create scene B (turns off)
	scene2Config := homeassistant.SceneConfig{
		Name: "Scene B - Off",
		Entities: map[string]homeassistant.SceneState{
			targetEntityID: {
				State: "off",
			},
		},
	}
	err = s.Client().CreateScene(s.Context(), scene2ID, scene2Config)
	s.Require().NoError(err)

	scene1EntityID := BuildEntityID("scene", scene1ID)
	scene2EntityID := BuildEntityID("scene", scene2ID)
	_, _ = s.WaitForEntity(scene1EntityID, 5*time.Second)
	_, _ = s.WaitForEntity(scene2EntityID, 5*time.Second)

	// Activate scene A
	_, err = s.Client().CallService(s.Context(), "scene", "turn_on", map[string]any{
		"entity_id": scene1EntityID,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	target, _ := s.Client().GetState(s.Context(), targetEntityID)
	s.Equal("on", target.State, "Target should be on after Scene A")

	// Activate scene B
	_, err = s.Client().CallService(s.Context(), "scene", "turn_on", map[string]any{
		"entity_id": scene2EntityID,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	target, _ = s.Client().GetState(s.Context(), targetEntityID)
	s.Equal("off", target.State, "Target should be off after Scene B")

	// Activate scene A again
	_, err = s.Client().CallService(s.Context(), "scene", "turn_on", map[string]any{
		"entity_id": scene1EntityID,
	})
	s.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)
	target, _ = s.Client().GetState(s.Context(), targetEntityID)
	s.Equal("on", target.State, "Target should be on again after Scene A")

	// Cleanup
	_ = s.Client().DeleteScene(s.Context(), scene1ID)
	_ = s.Client().DeleteScene(s.Context(), scene2ID)
	_ = s.Client().DeleteHelper(s.Context(), targetEntityID)
}
