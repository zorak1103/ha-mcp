//go:build integration

package integration

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// LogbookToolDispatchTestSuite covers get_logbook through the real registry +
// handler layer, which is where the entity_ids argument is parsed and turned
// into Home Assistant's comma-separated `entity` query parameter.
type LogbookToolDispatchTestSuite struct {
	IntegrationTestSuite
}

func TestLogbookToolDispatch(t *testing.T) {
	suite.Run(t, new(LogbookToolDispatchTestSuite))
}

// logbookEntityIDs returns the entity_id of every entry in a get_logbook
// entries-mode result ("summary\n\n{json}").
func (s *LogbookToolDispatchTestSuite) logbookEntityIDs(text string) []string {
	_, body, ok := strings.Cut(text, "\n\n")
	s.Require().True(ok, "expected a summary followed by a JSON body, got: %s", text)

	var parsed struct {
		Entries []struct {
			EntityID string `json:"entity_id"`
		} `json:"entries"`
	}
	s.Require().NoError(json.Unmarshal([]byte(body), &parsed), "parsing logbook JSON")

	ids := make([]string, 0, len(parsed.Entries))
	for _, e := range parsed.Entries {
		ids = append(ids, e.EntityID)
	}
	return ids
}

// TestGetLogbookEntityIDsFiltersToRequestedEntities verifies that a
// get_logbook call with entity_ids returns entries for exactly those entities
// and none for a third entity that also changed state, which confirms Home
// Assistant honors the comma-separated entity filter.
func (s *LogbookToolDispatchTestSuite) TestGetLogbookEntityIDsFiltersToRequestedEntities() {
	var entityIDs [3]string
	for i, suffix := range []string{"lb_a", "lb_b", "lb_c"} {
		name := GenerateTestID(suffix)
		entityID := BuildEntityID("input_boolean", name)
		entityIDs[i] = entityID

		s.RegisterCleanup(func() {
			_ = s.Client().DeleteHelper(s.Context(), entityID)
		})

		err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
			Platform: "input_boolean",
			Config:   map[string]any{"name": name, "initial": false},
		})
		s.Require().NoError(err, "failed to create input_boolean %s", suffix)
		_, err = s.WaitForEntity(entityID, 10*time.Second)
		s.Require().NoError(err)
	}

	for _, entityID := range entityIDs {
		_, err := s.Client().CallService(s.Context(), "input_boolean", "toggle", map[string]any{
			"entity_id": entityID,
		})
		s.Require().NoError(err, "failed to toggle %s", entityID)
	}

	idA, idB, idC := entityIDs[0], entityIDs[1], entityIDs[2]
	args := map[string]any{
		"entity_ids": []any{idA, idB},
		"hours":      1.0,
	}

	// The recorder commits asynchronously, so poll until both entities appear.
	var text string
	var seen []string
	deadline := time.Now().Add(15 * time.Second)
	for {
		result := s.CallTool("get_logbook", args)
		s.Require().False(result.IsError, "get_logbook should succeed, got: %s", resultText(result))

		text = resultText(result)
		seen = s.logbookEntityIDs(text)
		if slices.Contains(seen, idA) && slices.Contains(seen, idB) {
			break
		}
		if time.Now().After(deadline) {
			s.Require().Fail("timed out waiting for logbook entries", "entities seen: %v\n%s", seen, text)
		}
		time.Sleep(500 * time.Millisecond)
	}

	s.Contains(text, "for 2 entities", "summary should name the entity count")
	for _, id := range seen {
		s.Contains([]string{idA, idB}, id, "entry for unrequested entity %q returned", id)
	}
	s.NotContains(seen, idC, "the third entity changed state but was not requested")
}
