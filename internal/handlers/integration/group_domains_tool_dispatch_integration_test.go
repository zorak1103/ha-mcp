//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type GroupDomainsToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestGroupDomainsToolDispatch(t *testing.T) {
	suite.Run(t, new(GroupDomainsToolDispatchTestSuite))
}

// TestConfigEntryGroupUpdateDeleteViaTool pins that manage_helper update and
// delete accept a config-entry group helper in the light, cover, fan, lock and
// switch domains (issues #345, #350): checkHelperOnlyDomain only allowed the
// group platform for sensor/binary_sensor, so these were rejected as "not a
// helper entity". Group members only need to be valid entity IDs of the group's
// domain, so no real member entities are created.
func (s *GroupDomainsToolDispatchTestSuite) TestConfigEntryGroupUpdateDeleteViaTool() {
	for _, domain := range []string{"light", "cover", "fan", "lock", "switch"} {
		s.Run(domain, func() {
			member1 := BuildEntityID(domain, GenerateTestID("grpd_"+domain+"_m1"))
			member2 := BuildEntityID(domain, GenerateTestID("grpd_"+domain+"_m2"))
			groupName := GenerateTestID("grpd_" + domain)
			groupEntityID := BuildEntityID(domain, groupName)

			s.RegisterCleanup(func() {
				_ = s.Client().DeleteHelper(s.Context(), groupEntityID)
			})

			err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
				Platform: "group",
				Config: map[string]any{
					"name":     groupName,
					"entities": []string{member1, member2},
				},
			})
			s.Require().NoError(err, "failed to create %s group", domain)
			_, err = s.WaitForEntity(groupEntityID, 5*time.Second)
			s.Require().NoError(err, "%s group did not appear", domain)

			updateResult := s.CallTool("manage_helper", map[string]any{
				"action":    "update",
				"entity_id": groupEntityID,
				"entities":  []any{member1},
			})
			s.Require().False(updateResult.IsError, "manage_helper update on %s group should succeed, got: %s",
				domain, resultText(updateResult))

			deleteResult := s.CallTool("manage_helper", map[string]any{
				"action":    "delete",
				"entity_id": groupEntityID,
			})
			s.Require().False(deleteResult.IsError, "manage_helper delete on %s group should succeed, got: %s",
				domain, resultText(deleteResult))

			s.Require().NoError(s.WaitForEntityGone(groupEntityID, 5*time.Second), "%s group should be deleted", domain)
		})
	}
}
