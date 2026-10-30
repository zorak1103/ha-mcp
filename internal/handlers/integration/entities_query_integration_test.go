//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// EntitiesQueryTestSuite covers query_entities through the real registry +
// handler layer. mode=history is the first mode covered; entity_ids is parsed in
// the handler and sent to history/history_during_period as a list.
type EntitiesQueryTestSuite struct {
	IntegrationTestSuite
}

func TestQueryEntitiesHistory(t *testing.T) {
	suite.Run(t, new(EntitiesQueryTestSuite))
}

// createToggledBoolean creates an input_boolean named after suffix and toggles it
// once, so the recorder holds a second state row for it.
func (s *EntitiesQueryTestSuite) createToggledBoolean(suffix string) string {
	name := GenerateTestID(suffix)
	entityID := BuildEntityID("input_boolean", name)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})
	s.Require().NoError(s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": name, "initial": false},
	}), "failed to create input_boolean %s", suffix)
	_, err := s.WaitForEntity(entityID, 10*time.Second)
	s.Require().NoError(err)

	_, err = s.Client().CallService(s.Context(), "input_boolean", "toggle", map[string]any{"entity_id": entityID})
	s.Require().NoError(err, "failed to toggle %s", entityID)
	return entityID
}

type historyJSONSection struct {
	EntityID string `json:"entity_id"`
	Entries  []struct {
		State string `json:"state"`
	} `json:"entries"`
}

// historySections parses the multi-entity json output ("summary\n\n[{entity_id, entries}]").
func (s *EntitiesQueryTestSuite) historySections(text string) []historyJSONSection {
	_, body, ok := strings.Cut(text, "\n\n")
	s.Require().True(ok, "expected a summary followed by a JSON body, got: %s", text)

	var sections []historyJSONSection
	s.Require().NoError(json.Unmarshal([]byte(body), &sections), "parsing history JSON: %s", body)
	return sections
}

// TestQueryEntitiesHistory_SingleAndMultipleEntities verifies that entity_ids returns one
// section per entity in request order, that every section holds that entity's own rows
// (HA's compressed rows carry no entity_id), and that entity_id still returns the bare array.
func (s *EntitiesQueryTestSuite) TestQueryEntitiesHistory_SingleAndMultipleEntities() {
	idA := s.createToggledBoolean("qh_a")
	idB := s.createToggledBoolean("qh_b")

	// The recorder commits asynchronously: poll until both entities show their toggle.
	var sections []historyJSONSection
	deadline := time.Now().Add(15 * time.Second)
	for {
		// Request order is b, a on purpose.
		result := s.CallTool("query_entities", map[string]any{
			"mode": "history", "entity_ids": []any{idB, idA}, "hours": 1.0, "format": "json",
		})
		s.Require().False(result.IsError, "query_entities should succeed, got: %s", resultText(result))

		sections = s.historySections(resultText(result))
		if len(sections) == 2 && len(sections[0].Entries) >= 2 && len(sections[1].Entries) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			s.Require().Fail("timed out waiting for history rows", "%+v", sections)
		}
		time.Sleep(500 * time.Millisecond)
	}

	s.Equal(idB, sections[0].EntityID, "sections must follow request order")
	s.Equal(idA, sections[1].EntityID, "sections must follow request order")
	for _, section := range sections {
		s.Equal("off", section.Entries[0].State, "%s starts off", section.EntityID)
		s.Equal("on", section.Entries[len(section.Entries)-1].State, "%s was toggled on", section.EntityID)
	}

	single := s.CallTool("query_entities", map[string]any{
		"mode": "history", "entity_id": idA, "hours": 1.0, "format": "json",
	})
	s.Require().False(single.IsError, "single-entity query should succeed, got: %s", resultText(single))
	_, body, ok := strings.Cut(resultText(single), "\n\n")
	s.Require().True(ok)
	var rows []struct {
		State string `json:"state"`
	}
	s.Require().NoError(json.Unmarshal([]byte(body), &rows), "entity_id keeps the bare array shape: %s", body)
	s.Len(rows, len(sections[1].Entries), "entity_id and entity_ids must see the same rows")

	both := s.CallTool("query_entities", map[string]any{
		"mode": "history", "entity_id": idA, "entity_ids": []any{idB},
	})
	s.True(both.IsError, "entity_id together with entity_ids must be rejected")
	s.Contains(resultText(both), "Cannot specify both entity_id and entity_ids")
}

// TestQueryEntitiesHistory_UnknownEntityGetsItsOwnNote verifies that an entity HA has no
// rows for is reported on its own without hiding the entity that does have history.
func (s *EntitiesQueryTestSuite) TestQueryEntitiesHistory_UnknownEntityGetsItsOwnNote() {
	idA := s.createToggledBoolean("qh_known")
	unknown := BuildEntityID("input_boolean", GenerateTestID("qh_missing"))

	var text string
	deadline := time.Now().Add(15 * time.Second)
	for {
		result := s.CallTool("query_entities", map[string]any{
			"mode": "history", "entity_ids": []any{idA, unknown}, "hours": 1.0, "format": "natural",
		})
		s.Require().False(result.IsError, "query_entities should succeed, got: %s", resultText(result))

		text = resultText(result)
		if strings.Contains(text, "History for") {
			break
		}
		if time.Now().After(deadline) {
			s.Require().Fail("timed out waiting for history rows", text)
		}
		time.Sleep(500 * time.Millisecond)
	}

	s.Contains(text, "Entity "+unknown+" was not found in Home Assistant.")
	// The natural section is titled with the friendly name, which for a helper is its name.
	s.Contains(text, strings.TrimPrefix(idA, "input_boolean."), "the known entity keeps its own section")
}

// createInputNumber creates an input_number and returns its entity ID and name.
// input_number is not one of the domains HA's default history query keeps attribute-only
// updates for, so it exercises the significant_changes_only=false request. Its
// friendly_name attribute is recorded; min, max, step and mode are not (the integration
// excludes them from the recorder), so they can never show up in history.
func (s *EntitiesQueryTestSuite) createInputNumber(suffix string) (entityID, name string) {
	name = GenerateTestID(suffix)
	entityID = BuildEntityID("input_number", name)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), entityID)
	})
	s.Require().NoError(s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_number",
		Config:   map[string]any{"name": name, "min": 0.0, "max": 100.0, "initial": 50.0, "step": 1.0},
	}), "failed to create input_number %s", suffix)
	_, err := s.WaitForEntity(entityID, 10*time.Second)
	s.Require().NoError(err)
	return entityID, name
}

// renameInputNumber changes the helper's name through a config update: the entity_id and the
// state stay, only the friendly_name attribute changes.
func (s *EntitiesQueryTestSuite) renameInputNumber(entityID, newName string) {
	s.Require().NoError(s.Client().UpdateHelper(s.Context(), ExtractEntityID(entityID), homeassistant.HelperConfig{
		Platform: "input_number",
		Config:   map[string]any{"name": newName, "min": 0.0, "max": 100.0, "step": 1.0},
	}), "failed to rename %s to %s", entityID, newName)
}

type attributeChangeRow struct {
	At    string `json:"at"`
	Start bool   `json:"start"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

// TestQueryEntitiesHistory_AttributeChanges verifies that attribute=<name> reports the
// value at the start plus every later change as old -> new, for a domain whose
// attribute-only updates HA drops by default (input_number), in both formats. Without the
// significant_changes_only=false request only the first row would come back.
func (s *EntitiesQueryTestSuite) TestQueryEntitiesHistory_AttributeChanges() {
	entityID, name := s.createInputNumber("qh_attr")
	second, third := name+"_b", name+"_c"
	s.renameInputNumber(entityID, second)
	time.Sleep(time.Second)
	s.renameInputNumber(entityID, third)

	// The recorder commits asynchronously: poll until both changes are visible.
	var rows []attributeChangeRow
	var text string
	deadline := time.Now().Add(20 * time.Second)
	for {
		result := s.CallTool("query_entities", map[string]any{
			"mode": "history", "entity_id": entityID, "attribute": "friendly_name", "hours": 1.0, "format": "json",
		})
		s.Require().False(result.IsError, "query_entities should succeed, got: %s", resultText(result))

		text = resultText(result)
		_, body, ok := strings.Cut(text, "\n\n")
		s.Require().True(ok, "expected a summary followed by a JSON body, got: %s", text)
		rows = nil
		s.Require().NoError(json.Unmarshal([]byte(body), &rows), "parsing attribute history: %s", body)
		if len(rows) >= 3 {
			break
		}
		if time.Now().After(deadline) {
			s.Require().Fail("timed out waiting for the friendly_name changes", text)
		}
		time.Sleep(500 * time.Millisecond)
	}

	s.Require().Len(rows, 3, "start row plus two changes: %s", text)
	s.True(rows[0].Start)
	s.Equal(name, rows[0].New, "the first row is the value at the start")
	s.Equal(name, rows[1].Old)
	s.Equal(second, rows[1].New)
	s.Equal(second, rows[2].Old)
	s.Equal(third, rows[2].New)
	s.Contains(text, "Found 2 changes of attribute friendly_name for "+entityID)

	natural := s.CallTool("query_entities", map[string]any{
		"mode": "history", "entity_id": entityID, "attribute": "friendly_name", "hours": 1.0,
	})
	s.Require().False(natural.IsError, "natural format should succeed, got: %s", resultText(natural))
	s.Contains(resultText(natural), "(start) → "+name)
	s.Contains(resultText(natural), name+" → "+second)
	s.Contains(resultText(natural), second+" → "+third)

	missing := s.CallTool("query_entities", map[string]any{
		"mode": "history", "entity_id": entityID, "attribute": "no_such_attribute", "hours": 1.0,
	})
	s.Require().False(missing.IsError, "an absent attribute is a note, not an error: %s", resultText(missing))
	s.Contains(resultText(missing), "is not present in any history row")

	both := s.CallTool("query_entities", map[string]any{
		"mode": "history", "entity_id": entityID, "attribute": "friendly_name", "state": "50",
	})
	s.True(both.IsError, "state together with attribute must be rejected")
}
