package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestIsYAMLDefinedEntity_StorageManaged(t *testing.T) {
	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "script.morning_routine", UniqueID: "01JABC123"},
			}, nil
		},
	}

	isYAML, checked := isYAMLDefinedEntity(context.Background(), client, "script.morning_routine")
	if !checked {
		t.Fatal("expected checked=true when registry lookup succeeds")
	}
	if isYAML {
		t.Error("expected isYAML=false for a registry entry with a non-empty unique_id")
	}
}

func TestIsYAMLDefinedEntity_EmptyUniqueID(t *testing.T) {
	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "script.example_toggle", UniqueID: ""},
			}, nil
		},
	}

	isYAML, checked := isYAMLDefinedEntity(context.Background(), client, "script.example_toggle")
	if !checked {
		t.Fatal("expected checked=true when registry lookup succeeds")
	}
	if !isYAML {
		t.Error("expected isYAML=true for a registry entry with an empty unique_id")
	}
}

func TestIsYAMLDefinedEntity_NoRegistryEntry(t *testing.T) {
	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "script.other_script", UniqueID: "01JXYZ999"},
			}, nil
		},
	}

	isYAML, checked := isYAMLDefinedEntity(context.Background(), client, "script.example_toggle")
	if !checked {
		t.Fatal("expected checked=true when registry lookup succeeds")
	}
	if !isYAML {
		t.Error("expected isYAML=true when the entity has no registry entry at all")
	}
}

func TestIsYAMLDefinedEntity_RegistryLookupFails(t *testing.T) {
	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return nil, errors.New("registry unavailable")
		},
	}

	isYAML, checked := isYAMLDefinedEntity(context.Background(), client, "script.example_toggle")
	if checked {
		t.Fatal("expected checked=false when registry lookup fails, so caller proceeds with the write")
	}
	if isYAML {
		t.Error("expected isYAML=false when checked=false (value should be ignored, but must not signal YAML)")
	}
}

func TestYamlDefinedWriteError(t *testing.T) {
	msg := yamlDefinedWriteError("script", "example_toggle", "script.example_toggle")

	assertContainsAll(t, msg, []string{
		"example_toggle",
		"script.example_toggle",
		"YAML-defined",
		"script.example_toggle_2",
		"script.reload",
	})
}
