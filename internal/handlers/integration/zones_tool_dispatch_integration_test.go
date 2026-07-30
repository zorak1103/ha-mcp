//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// ZoneToolDispatchTestSuite covers manage_zone's list action through the
// real registry+handler+HybridClient dispatch path (CallTool), not just the
// Client() method used by ZoneIntegrationTestSuite's create/update/delete
// lifecycle tests. This mirrors the person tool dispatch coverage:
// GetZones had the identical wrong "config/zone/list" WebSocket
// command prefix (HA registers it as "zone/list"), also misdiagnosed as a
// version incompatibility.
type ZoneToolDispatchTestSuite struct {
	ZoneTestSuite
}

func TestZoneToolDispatch(t *testing.T) {
	suite.Run(t, new(ZoneToolDispatchTestSuite))
}

// TestZoneListViaTool calls the read-only list action, so it never creates,
// modifies, or deletes anything and needs no RegisterCleanup.
func (s *ZoneToolDispatchTestSuite) TestZoneListViaTool() {
	result := s.CallTool("manage_zone", map[string]any{"action": "list", "format": "json"})
	s.Require().False(result.IsError, "manage_zone list should succeed, got: %s", resultText(result))
}
