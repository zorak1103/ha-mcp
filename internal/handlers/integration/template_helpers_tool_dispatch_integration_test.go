//go:build integration

package integration

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

var createdEntityIDPattern = regexp.MustCompile(`as (\S+)$`)

type TemplateHelperToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestTemplateHelperToolDispatch(t *testing.T) {
	suite.Run(t, new(TemplateHelperToolDispatchTestSuite))
}

// TestTemplateSensorUpdateViaTool reproduces a bug where manage_helper update
// failed with "unknown_command" for config-entry template helpers because the
// handler passed the bare object_id to client.UpdateHelper instead of the
// full entity_id that HybridClient's registry-based routing requires. This
// test drives the update through the real manage_helper tool (not
// s.Client().UpdateHelper directly, unlike every other template helper test
// in this package) so a regression here fails a test again.
func (s *TemplateHelperToolDispatchTestSuite) TestTemplateSensorUpdateViaTool() {
	sourceName := GenerateTestID("tmpl_td_src")
	sourceEntityID := BuildEntityID("input_number", sourceName)
	templateName := GenerateTestID("tmpl_td")
	templateEntityID := BuildEntityID("sensor", templateName)

	s.RegisterCleanup(func() {
		_ = s.Client().DeleteHelper(s.Context(), templateEntityID)
		_ = s.Client().DeleteHelper(s.Context(), sourceEntityID)
	})

	// Fixture setup uses the direct client, exactly like every other test in
	// this package - only the update action under test goes through CallTool.
	sourceConfig := homeassistant.HelperConfig{
		Platform: "input_number",
		Config: map[string]any{
			"name":    sourceName,
			"min":     0.0,
			"max":     100.0,
			"initial": 50.0,
		},
	}
	err := s.Client().CreateHelper(s.Context(), sourceConfig)
	s.Require().NoError(err, "Failed to create source input_number")

	_, err = s.WaitForEntity(sourceEntityID, 5*time.Second)
	s.Require().NoError(err, "Source input_number did not appear")

	templateConfig := homeassistant.HelperConfig{
		Platform: "template",
		Config: map[string]any{
			"name":  templateName,
			"state": "{{ states('" + sourceEntityID + "') | float }}",
		},
	}
	err = s.Client().CreateHelper(s.Context(), templateConfig)
	s.Require().NoError(err, "Failed to create template sensor")

	_, err = s.WaitForEntity(templateEntityID, 5*time.Second)
	s.Require().NoError(err, "Template sensor did not appear")

	// The action under test: update via the real manage_helper tool.
	result := s.CallTool("manage_helper", map[string]any{
		"action":    "update",
		"entity_id": templateEntityID,
		"state":     "{{ states('" + sourceEntityID + "') | float * 2 }}",
	})
	s.Require().False(result.IsError, "manage_helper update should succeed, got: %s", resultText(result))

	time.Sleep(2 * time.Second)

	entity, err := s.Client().GetState(s.Context(), templateEntityID)
	s.Require().NoError(err)
	s.Equal("100.0", entity.State, "Template sensor should show doubled value (source=50) after tool-driven update")
}

// TestTemplateBinarySensorCreateViaTool reproduces issue #211: determineTemplateSubtype
// read a dead config key ("platformTemplate_type" instead of "template_type"),
// so a manage_helper create of type=template_binary_sensor with no
// binary-inferrable device_class fell through to device-class guessing,
// which created a sensor while the tool reported a binary_sensor.* entity id
// that never existed. This drives the create through the real manage_helper
// tool (not s.Client().CreateHelper with an explicit "type" key, unlike the
// createTemplateBinarySensor fixture elsewhere in this package, which
// bypasses the router this bug lives in) and asserts the reported entity id
// actually resolves to a live entity.
func (s *TemplateHelperToolDispatchTestSuite) TestTemplateBinarySensorCreateViaTool() {
	s.Run("without device_class", func() {
		name := GenerateTestID("tmpl_td_bs_plain")

		// Clean up both possible outcomes so a pre-fix run (which creates
		// sensor.<name> while reporting binary_sensor.<name>) never leaks an
		// orphan entity.
		s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), BuildEntityID("sensor", name)) })
		s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), BuildEntityID("binary_sensor", name)) })

		result := s.CallTool("manage_helper", map[string]any{
			"action": "create",
			"type":   "template_binary_sensor",
			"id":     name,
			"name":   name,
			"state":  "{{ true }}",
		})
		s.Require().False(result.IsError, "manage_helper create should succeed, got: %s", resultText(result))

		match := createdEntityIDPattern.FindStringSubmatch(resultText(result))
		s.Require().Len(match, 2, "could not parse reported entity id from: %s", resultText(result))
		reportedEntityID := match[1]
		s.Equal(BuildEntityID("binary_sensor", name), reportedEntityID, "tool should report a binary_sensor entity id")

		_, err := s.WaitForEntity(reportedEntityID, 5*time.Second)
		s.Require().NoError(err, "reported entity id %q did not resolve to a live entity", reportedEntityID)
	})

	s.Run("with binary device_class does not regress", func() {
		name := GenerateTestID("tmpl_td_bs_dc")
		s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), BuildEntityID("sensor", name)) })
		s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), BuildEntityID("binary_sensor", name)) })

		result := s.CallTool("manage_helper", map[string]any{
			"action":       "create",
			"type":         "template_binary_sensor",
			"id":           name,
			"name":         name,
			"state":        "{{ true }}",
			"device_class": "problem",
		})
		s.Require().False(result.IsError, "manage_helper create should succeed, got: %s", resultText(result))

		match := createdEntityIDPattern.FindStringSubmatch(resultText(result))
		s.Require().Len(match, 2, "could not parse reported entity id from: %s", resultText(result))
		reportedEntityID := match[1]
		s.Equal(BuildEntityID("binary_sensor", name), reportedEntityID, "tool should report a binary_sensor entity id")

		_, err := s.WaitForEntity(reportedEntityID, 5*time.Second)
		s.Require().NoError(err, "reported entity id %q did not resolve to a live entity", reportedEntityID)
	})
}
