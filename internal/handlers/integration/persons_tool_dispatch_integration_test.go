//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// PersonToolDispatchTestSuite covers manage_person's list action through the
// real registry+handler+HybridClient dispatch path (CallTool), not just the
// Client() method used by PersonTestSuite's create/update/delete lifecycle
// tests. This is the layer where issue #145 lived: GetPersons sent the
// WebSocket command "config/person/list" instead of the HA-registered
// "person/list", failing with unknown_command on every Home Assistant
// version - misdiagnosed as a version incompatibility until traced to the
// wrong command prefix.
type PersonToolDispatchTestSuite struct {
	PersonTestSuite
}

func TestPersonToolDispatch(t *testing.T) {
	suite.Run(t, new(PersonToolDispatchTestSuite))
}

// TestPersonListViaTool calls the read-only list action, so it never
// creates, modifies, or deletes anything and needs no RegisterCleanup.
func (s *PersonToolDispatchTestSuite) TestPersonListViaTool() {
	result := s.CallTool("manage_person", map[string]any{"action": "list", "format": "json"})
	s.Require().False(result.IsError, "manage_person list should succeed, got: %s", resultText(result))
}
