//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

type TemplateSubtypesIntegrationTestSuite struct {
	HelperTestSuite
}

func TestTemplateSubtypesIntegration(t *testing.T) {
	suite.Run(t, new(TemplateSubtypesIntegrationTestSuite))
}

// templateSubtypeCase is one row of the table-driven sweep below: enough
// to create the simplest valid instance of one of the 15 new template_*
// subtypes (issue #206) through the real manage_helper tool, and enough to
// prove the entity it reports actually exists.
type templateSubtypeCase struct {
	helperType string
	domain     string
	// extraArgs supplies every field HA's CONFIG_FLOW schema requires for
	// this subtype, beyond the universal id/name/type. Action fields use a
	// harmless input_boolean.toggle action against a single shared
	// fixture entity created once for the whole suite.
	extraArgs func(actionTargetEntityID string) map[string]any
}

func toggleAction(entityID string) map[string]any {
	return map[string]any{
		"action": "input_boolean.toggle",
		"target": map[string]any{"entity_id": entityID},
	}
}

var templateSubtypeCases = []templateSubtypeCase{
	{
		helperType: "template_alarm_control_panel",
		domain:     "alarm_control_panel",
		extraArgs: func(string) map[string]any {
			return map[string]any{"state": "{{ 'disarmed' }}"}
		},
	},
	{
		helperType: "template_button",
		domain:     "button",
		extraArgs: func(actionTargetEntityID string) map[string]any {
			return map[string]any{"press": toggleAction(actionTargetEntityID)}
		},
	},
	{
		helperType: "template_cover",
		domain:     "cover",
		extraArgs: func(actionTargetEntityID string) map[string]any {
			return map[string]any{
				"state": "{{ 'closed' }}",
				"open":  toggleAction(actionTargetEntityID),
				"close": toggleAction(actionTargetEntityID),
			}
		},
	},
	{
		helperType: "template_device_tracker",
		domain:     "device_tracker",
		extraArgs: func(string) map[string]any {
			return map[string]any{"in_zones": "{{ 'home' }}"}
		},
	},
	{
		helperType: "template_event",
		domain:     "event",
		extraArgs: func(string) map[string]any {
			return map[string]any{
				"event_type":  "{{ 'my_event' }}",
				"event_types": "{{ ['my_event'] }}",
			}
		},
	},
	{
		helperType: "template_fan",
		domain:     "fan",
		extraArgs: func(actionTargetEntityID string) map[string]any {
			return map[string]any{
				"state":    "{{ 'off' }}",
				"turn_on":  toggleAction(actionTargetEntityID),
				"turn_off": toggleAction(actionTargetEntityID),
			}
		},
	},
	{
		helperType: "template_image",
		domain:     "image",
		extraArgs: func(string) map[string]any {
			return map[string]any{"url": "{{ 'https://example.com/image.png' }}"}
		},
	},
	{
		helperType: "template_light",
		domain:     "light",
		extraArgs: func(actionTargetEntityID string) map[string]any {
			return map[string]any{
				"state":    "{{ 'off' }}",
				"turn_on":  toggleAction(actionTargetEntityID),
				"turn_off": toggleAction(actionTargetEntityID),
			}
		},
	},
	{
		helperType: "template_lock",
		domain:     "lock",
		extraArgs: func(actionTargetEntityID string) map[string]any {
			return map[string]any{
				"state":  "{{ 'locked' }}",
				"lock":   toggleAction(actionTargetEntityID),
				"unlock": toggleAction(actionTargetEntityID),
			}
		},
	},
	{
		helperType: "template_number",
		domain:     "number",
		extraArgs: func(actionTargetEntityID string) map[string]any {
			return map[string]any{
				"state":     "{{ 0 }}",
				"set_value": toggleAction(actionTargetEntityID),
			}
		},
	},
	{
		helperType: "template_select",
		domain:     "select",
		extraArgs: func(string) map[string]any {
			return map[string]any{
				"state":            "{{ 'a' }}",
				"options_template": "{{ ['a', 'b'] }}",
			}
		},
	},
	{
		helperType: "template_switch",
		domain:     "switch",
		extraArgs: func(string) map[string]any {
			return map[string]any{"state": "{{ 'off' }}"}
		},
	},
	{
		helperType: "template_update",
		domain:     "update",
		extraArgs: func(string) map[string]any {
			return map[string]any{
				"installed_version": "{{ '1.0' }}",
				"latest_version":    "{{ '1.0' }}",
			}
		},
	},
	{
		helperType: "template_vacuum",
		domain:     "vacuum",
		extraArgs: func(actionTargetEntityID string) map[string]any {
			return map[string]any{
				"state": "{{ 'docked' }}",
				"start": toggleAction(actionTargetEntityID),
			}
		},
	},
	{
		helperType: "template_weather",
		domain:     "weather",
		extraArgs: func(string) map[string]any {
			return map[string]any{
				"condition":   "{{ 'sunny' }}",
				"humidity":    "{{ 50 }}",
				"temperature": "{{ 20 }}",
			}
		},
	},
}

// TestAllTemplateSubtypesLifecycle sweeps every one of the 15 new
// template_* helper types (issue #206) through the real manage_helper
// tool: create with the minimal field set HA's CONFIG_FLOW schema
// requires, assert the reported entity id actually resolves (the #211
// regression this generalizes), update one field through the real tool,
// then delete. template_sensor/template_binary_sensor already have their
// own dedicated suites and are not repeated here.
func (s *TemplateSubtypesIntegrationTestSuite) TestAllTemplateSubtypesLifecycle() {
	actionTargetName := GenerateTestID("tmpl_subtypes_target")
	actionTargetEntityID := BuildEntityID("input_boolean", actionTargetName)
	err := s.Client().CreateHelper(s.Context(), homeassistant.HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": actionTargetName, "initial": false},
	})
	s.Require().NoError(err, "Failed to create shared action-target input_boolean")
	_, err = s.WaitForEntity(actionTargetEntityID, 5*time.Second)
	s.Require().NoError(err, "Shared action-target input_boolean did not appear")
	s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), actionTargetEntityID) })

	for _, tc := range templateSubtypeCases {
		s.Run(tc.helperType, func() {
			name := GenerateTestID(tc.helperType)

			createArgs := map[string]any{
				"action": "create",
				"type":   tc.helperType,
				"id":     name,
				"name":   name,
			}
			for k, v := range tc.extraArgs(actionTargetEntityID) {
				createArgs[k] = v
			}

			result := s.CallTool("manage_helper", createArgs)
			s.Require().False(result.IsError, "manage_helper create(%s) should succeed, got: %s", tc.helperType, resultText(result))

			match := createdEntityIDPattern.FindStringSubmatch(resultText(result))
			s.Require().Len(match, 2, "could not parse reported entity id from: %s", resultText(result))
			reportedEntityID := match[1]
			wantEntityID := BuildEntityID(tc.domain, name)
			s.Equal(wantEntityID, reportedEntityID, "manage_helper should report a %s-domain entity id for %s", tc.domain, tc.helperType)

			s.RegisterCleanup(func() { _ = s.Client().DeleteHelper(s.Context(), reportedEntityID) })

			_, err := s.WaitForEntity(reportedEntityID, 5*time.Second)
			s.Require().NoError(err, "%s: reported entity id %q did not resolve to a live entity", tc.helperType, reportedEntityID)

			updateResult := s.CallTool("manage_helper", map[string]any{
				"action":    "update",
				"entity_id": reportedEntityID,
				"icon":      "mdi:test",
			})
			s.Require().False(updateResult.IsError, "manage_helper update(%s) should succeed, got: %s", tc.helperType, resultText(updateResult))

			deleteResult := s.CallTool("manage_helper", map[string]any{
				"action":    "delete",
				"entity_id": reportedEntityID,
			})
			s.Require().False(deleteResult.IsError, "manage_helper delete(%s) should succeed, got: %s", tc.helperType, resultText(deleteResult))

			err = s.WaitForEntityGone(reportedEntityID, 5*time.Second)
			s.Require().NoError(err, "%s: entity %q should be deleted", tc.helperType, reportedEntityID)
		})
	}
}
