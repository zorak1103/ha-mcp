//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type TemplateHelperToolDispatchTestSuite struct {
	HelperTestSuite
}

func TestTemplateHelperToolDispatch(t *testing.T) {
	suite.Run(t, new(TemplateHelperToolDispatchTestSuite))
}

// TestTemplateSensorUpdateViaTool reproduces issue #135: manage_helper update
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
