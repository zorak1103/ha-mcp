package formatter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestNaturalSceneFormatter_FormatList(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalSceneFormatter()

	scenes := []SceneInfo{
		{
			EntityID:     "scene.movie_night",
			State:        "scening",
			FriendlyName: "Movie Night",
			EntityIDs:    []string{"light.living", "light.tv", "switch.lamp", "media_player.tv"},
		},
		{
			EntityID:     "scene.bright",
			State:        "scening",
			FriendlyName: "Bright",
			EntityIDs:    []string{"light.living", "light.kitchen", "light.bedroom", "light.bathroom"},
		},
		{
			EntityID:     "scene.bedtime",
			State:        "scening",
			FriendlyName: "Bedtime",
			EntityIDs:    []string{"light.bedroom", "light.bathroom", "light.hallway"},
		},
	}

	opts := SceneListOptions{}
	result, err := f.FormatList(ctx, scenes, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify summary line
	if !strings.Contains(result, "3 scenes") {
		t.Errorf("expected '3 scenes' in output, got: %s", result)
	}

	// Verify domain breakdown
	if !strings.Contains(result, "By affected domains:") {
		t.Errorf("expected 'By affected domains:' in output, got: %s", result)
	}

	// Verify scene names
	if !strings.Contains(result, "Movie Night") {
		t.Errorf("expected 'Movie Night' in output, got: %s", result)
	}
}

func TestNaturalSceneFormatter_FormatList_Empty(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalSceneFormatter()

	result, err := f.FormatList(ctx, nil, SceneListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != MsgNoScenesFound {
		t.Errorf("expected '%s', got: %s", MsgNoScenesFound, result)
	}
}

func TestNaturalSceneFormatter_FormatDetail(t *testing.T) {
	ctx := context.Background()
	f := NewNaturalSceneFormatter()

	scene := homeassistant.Entity{
		EntityID: "scene.movie_night",
		State:    "scening",
		Attributes: map[string]any{
			"friendly_name": "Movie Night",
			"icon":          "mdi:movie",
			"entity_id":     []any{"light.living", "light.tv", "switch.lamp"},
		},
	}

	result, err := f.FormatDetail(ctx, scene)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify header
	if !strings.Contains(result, "Movie Night") {
		t.Errorf("expected 'Movie Night' in output, got: %s", result)
	}

	// Verify entity list
	if !strings.Contains(result, "Entities") {
		t.Errorf("expected 'Entities' section in output, got: %s", result)
	}
	if !strings.Contains(result, "light.living") {
		t.Errorf("expected 'light.living' in output, got: %s", result)
	}
}

func TestJSONSceneFormatter_FormatList(t *testing.T) {
	ctx := context.Background()
	f := NewJSONSceneFormatter()

	scenes := []SceneInfo{
		{
			EntityID:     "scene.test",
			State:        "scening",
			FriendlyName: "Test",
		},
	}

	result, err := f.FormatList(ctx, scenes, SceneListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("expected valid JSON array, got error: %v", err)
	}

	if len(parsed) != 1 {
		t.Errorf("expected 1 item in JSON, got: %d", len(parsed))
	}
}

func TestJSONSceneFormatter_FormatList_Empty(t *testing.T) {
	ctx := context.Background()
	f := NewJSONSceneFormatter()

	result, err := f.FormatList(ctx, nil, SceneListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "[]" {
		t.Errorf("expected '[]', got: %s", result)
	}
}

func TestJSONSceneFormatter_FormatDetail(t *testing.T) {
	ctx := context.Background()
	f := NewJSONSceneFormatter()

	scene := homeassistant.Entity{
		EntityID: "scene.test",
		State:    "scening",
		Attributes: map[string]any{
			"friendly_name": "Test",
		},
	}

	result, err := f.FormatDetail(ctx, scene)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("expected valid JSON object, got error: %v", err)
	}

	if parsed["entity_id"] != "scene.test" {
		t.Errorf("expected entity_id 'scene.test', got: %v", parsed["entity_id"])
	}
}

func TestNewSceneFormatter(t *testing.T) {
	natural := NewSceneFormatter(FormatNatural)
	if _, ok := natural.(*NaturalSceneFormatter); !ok {
		t.Errorf("expected NaturalSceneFormatter for natural format")
	}

	jsonFmt := NewSceneFormatter(FormatJSON)
	if _, ok := jsonFmt.(*JSONSceneFormatter); !ok {
		t.Errorf("expected JSONSceneFormatter for json format")
	}

	defaultFmt := NewSceneFormatter("")
	if _, ok := defaultFmt.(*NaturalSceneFormatter); !ok {
		t.Errorf("expected NaturalSceneFormatter for empty format")
	}
}
