//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// GetDetailsRemediationTestSuite covers the manage_helper get_details paths
// touched by the adversarial-review remediation of the issue #216 fix
// (natural-format get_details for climate/humidifier/select and template
// subtypes): finding G1 (handleGetDetails' dispatch missing switch_as_x's
// siren/valve targets), finding C1 (a group helper's entity_id state
// attribute silently overwriting the real entity_id), and finding W5
// (get_details' template-config enrichment not covering the 15 template_*
// subtype domains). These exercise the real manage_helper tool end-to-end
// against a live Home Assistant instance - the corresponding unit tests use
// a mock client and cannot catch a routing or Options Flow regression the
// way these can (see CLAUDE.md's "Integration test scope").
type GetDetailsRemediationTestSuite struct {
	HelperTestSuite
}

func TestGetDetailsRemediation(t *testing.T) {
	suite.Run(t, new(GetDetailsRemediationTestSuite))
}

// createTemplateSwitchWrapper creates an input_boolean plus a template
// switch wrapping it, mirroring SwitchAsXIntegrationTestSuite.createSourceSwitch -
// switch_as_x requires a switch entity to wrap, not an input_boolean directly.
func (s *GetDetailsRemediationTestSuite) createTemplateSwitchWrapper(prefix string) (boolEntityID, switchEntityID string) {
	boolName := GenerateTestID(prefix + "_bool")
	boolEntityID = BuildEntityID("input_boolean", boolName)
	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": boolName, "initial": false},
	})
	s.Require().NoError(err, "Failed to create input_boolean")
	_, err = s.WaitForEntity(boolEntityID, 5*time.Second)
	s.Require().NoError(err, "input_boolean did not appear")

	switchName := GenerateTestID(prefix + "_switch")
	switchEntityID = BuildEntityID("switch", switchName)
	err = s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":     switchName,
			"turn_on":  map[string]any{"service": "input_boolean.turn_on", "data": map[string]any{"entity_id": boolEntityID}},
			"turn_off": map[string]any{"service": "input_boolean.turn_off", "data": map[string]any{"entity_id": boolEntityID}},
			"type":     "switch",
		},
	})
	s.Require().NoError(err, "Failed to create template switch wrapper")
	_, err = s.WaitForEntity(switchEntityID, 5*time.Second)
	s.Require().NoError(err, "template switch wrapper did not appear")

	return boolEntityID, switchEntityID
}

// TestGetDetailsSirenValve pins finding G1: handleGetDetails' dispatch used
// a hand-maintained domain list that never included siren/valve - the two
// switch_as_x target domains beyond cover/fan/light/lock - so get_details
// on those wrapper entities errored with "not supported for helper type"
// even though update/delete worked fine on the same entity.
func (s *GetDetailsRemediationTestSuite) TestGetDetailsSirenValve() {
	for _, targetDomain := range []string{"siren", "valve"} {
		s.Run(targetDomain, func() {
			boolEntityID, switchEntityID := s.createTemplateSwitchWrapper("gd_" + targetDomain)

			wrapperName := GenerateTestID("gd_" + targetDomain + "_wrap")
			wrapperEntityID, err := s.Client().CreateHelperEntity(s.Context(), homeassistant.HelperConfig{
				Platform: "switch_as_x",
				Config: map[string]any{
					"name":          wrapperName,
					"entity_id":     switchEntityID,
					"target_domain": targetDomain,
				},
			})
			s.Require().NoError(err, "Failed to create switch_as_x as %s", targetDomain)
			s.Require().NotEmpty(wrapperEntityID, "expected the real entity_id to be resolved via the entity registry")

			s.RegisterCleanup(func() {
				_ = s.Client().DeleteHelper(s.Context(), wrapperEntityID)
				_ = s.Client().DeleteHelper(s.Context(), switchEntityID)
				_ = s.Client().DeleteHelper(s.Context(), boolEntityID)
			})

			_, err = s.WaitForEntity(wrapperEntityID, 5*time.Second)
			s.Require().NoError(err, "%s wrapper did not appear", targetDomain)

			// The action under test: get_details through the real
			// manage_helper tool, both formats.
			naturalResult := s.CallTool("manage_helper", map[string]any{
				"action":    "get_details",
				"entity_id": wrapperEntityID,
				"format":    "natural",
			})
			naturalText := resultText(naturalResult)
			s.Require().False(naturalResult.IsError, "get_details(natural) on %s should succeed, got: %s", targetDomain, naturalText)
			s.NotContains(naturalText, "not supported for helper type", "finding G1 regression")

			jsonResult := s.CallTool("manage_helper", map[string]any{
				"action":    "get_details",
				"entity_id": wrapperEntityID,
				"format":    "json",
			})
			jsonText := resultText(jsonResult)
			s.Require().False(jsonResult.IsError, "get_details(json) on %s should succeed, got: %s", targetDomain, jsonText)
			s.NotContains(jsonText, "not supported for helper type", "finding G1 regression")
		})
	}
}

// TestGetDetailsTemplateLight pins C1/W5: widening get_details' template
// config enrichment to the 15 template_* subtype domains must not let the
// subtype's Jinja "state" template field overwrite the entity's real
// runtime state - template_light is one of the nine subtypes that reuse the
// arg name "state" for its template field, which is exactly issue #216's C1
// collision, reintroduced inside this very fix if written unsuffixed. It
// must also surface the entity's actual template config (state_template),
// not just its runtime attributes, against a real Options Flow round-trip.
func (s *GetDetailsRemediationTestSuite) TestGetDetailsTemplateLight() {
	targetName := GenerateTestID("gd_tpl_light_target")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": targetName, "initial": false},
	})
	s.Require().NoError(err, "Failed to create action-target input_boolean")
	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err, "action-target input_boolean did not appear")
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), targetEntityID) })

	lightName := GenerateTestID("gd_tpl_light")
	lightEntityID := BuildEntityID("light", lightName)
	createResult := s.CallTool("manage_helper", map[string]any{
		"action": "create",
		"type":   "template_light",
		"id":     lightName,
		"name":   lightName,
		"state":  "{{ 'off' }}",
		"turn_on": map[string]any{
			"action": "input_boolean.turn_on",
			"target": map[string]any{"entity_id": targetEntityID},
		},
		"turn_off": map[string]any{
			"action": "input_boolean.turn_off",
			"target": map[string]any{"entity_id": targetEntityID},
		},
	})
	s.Require().False(createResult.IsError, "create template_light should succeed, got: %s", resultText(createResult))
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), lightEntityID) })

	_, err = s.WaitForEntity(lightEntityID, 5*time.Second)
	s.Require().NoError(err, "template_light did not appear")

	// The action under test: get_details, natural format.
	naturalResult := s.CallTool("manage_helper", map[string]any{
		"action":    "get_details",
		"entity_id": lightEntityID,
		"format":    "natural",
	})
	naturalText := resultText(naturalResult)
	s.Require().False(naturalResult.IsError, "get_details(natural) on template_light should succeed, got: %s", naturalText)
	s.Contains(naturalText, "State: off", "the entity's real runtime state must survive, not be overwritten by the state_template")
	s.Contains(naturalText, "State template:", "the configured state template should be surfaced separately")
	s.Contains(naturalText, "{{ 'off' }}", "the configured state template's actual value should be surfaced")

	jsonResult := s.CallTool("manage_helper", map[string]any{
		"action":    "get_details",
		"entity_id": lightEntityID,
		"format":    "json",
	})
	jsonText := resultText(jsonResult)
	s.Require().False(jsonResult.IsError, "get_details(json) on template_light should succeed, got: %s", jsonText)
	s.Contains(jsonText, `"state": "off"`, "the entity's real runtime state must survive in JSON too")
	s.Contains(jsonText, `"state_template"`, "the configured state template should be surfaced separately in JSON")
}

// TestGetDetailsGroupEntityIDCollision pins finding C1 against real Home
// Assistant behavior: HA's group integration documents the group entity's
// own "entity_id" state attribute as the list of member entity_ids
// (https://www.home-assistant.io/integrations/group/). A group_type
// override routes the group's entity through get_details' generic fallback
// path (light/cover/switch/fan/lock have no dedicated detail builder,
// unlike the plain "group" domain), so this collision is reachable in
// practice - the real entity_id must not be silently overwritten by the
// member-list attribute.
func (s *GetDetailsRemediationTestSuite) TestGetDetailsGroupEntityIDCollision() {
	targetName := GenerateTestID("gd_grp_light_target")
	targetEntityID := BuildEntityID("input_boolean", targetName)
	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": targetName, "initial": false},
	})
	s.Require().NoError(err, "Failed to create action-target input_boolean")
	_, err = s.WaitForEntity(targetEntityID, 5*time.Second)
	s.Require().NoError(err, "action-target input_boolean did not appear")
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), targetEntityID) })

	lightName := GenerateTestID("gd_grp_light_member")
	lightEntityID := BuildEntityID("light", lightName)
	createLightResult := s.CallTool("manage_helper", map[string]any{
		"action": "create",
		"type":   "template_light",
		"id":     lightName,
		"name":   lightName,
		"state":  "{{ 'off' }}",
		"turn_on": map[string]any{
			"action": "input_boolean.turn_on",
			"target": map[string]any{"entity_id": targetEntityID},
		},
		"turn_off": map[string]any{
			"action": "input_boolean.turn_off",
			"target": map[string]any{"entity_id": targetEntityID},
		},
	})
	s.Require().False(createLightResult.IsError, "create template_light group member should succeed, got: %s", resultText(createLightResult))
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), lightEntityID) })
	_, err = s.WaitForEntity(lightEntityID, 5*time.Second)
	s.Require().NoError(err, "template_light group member did not appear")

	groupName := GenerateTestID("gd_grp_light")
	groupEntityID := BuildEntityID("light", groupName)
	err = s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "group",
		Config: map[string]any{
			"name":       groupName,
			"entities":   []string{lightEntityID},
			"group_type": "light",
		},
	})
	s.Require().NoError(err, "Failed to create light group")
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), groupEntityID) })
	_, err = s.WaitForEntity(groupEntityID, 5*time.Second)
	s.Require().NoError(err, "light group did not appear")

	// The action under test: get_details, both formats.
	naturalResult := s.CallTool("manage_helper", map[string]any{
		"action":    "get_details",
		"entity_id": groupEntityID,
		"format":    "natural",
	})
	naturalText := resultText(naturalResult)
	s.Require().False(naturalResult.IsError, "get_details(natural) on light group should succeed, got: %s", naturalText)
	s.Contains(naturalText, groupEntityID, "the real entity_id must still appear in the header")
	s.Contains(naturalText, "Members:", "the member list must be rescued under Members, not silently dropped")
	s.Contains(naturalText, lightEntityID, "the member list should name the actual member")

	jsonResult := s.CallTool("manage_helper", map[string]any{
		"action":    "get_details",
		"entity_id": groupEntityID,
		"format":    "json",
	})
	jsonText := resultText(jsonResult)
	s.Require().False(jsonResult.IsError, "get_details(json) on light group should succeed, got: %s", jsonText)
	s.Contains(jsonText, `"entity_id": "`+groupEntityID+`"`, "entity_id must not be overwritten by the member-list attribute")
	s.Contains(jsonText, `"members"`, "member list must be rescued under members")
}
