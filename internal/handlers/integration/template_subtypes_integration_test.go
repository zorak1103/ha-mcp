//go:build integration

package integration

import (
	"strings"
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
	// updateArg is one real config field (name, value) exercised on update,
	// beyond the universal "icon" field the sweep already covers. Icon is
	// applied via the Entity Registry, bypassing the Options Flow config
	// builder entirely (addTemplateConfigEntryUpdateFields,
	// resolveTemplateFieldsForDomain, and every haKey rename), so without
	// this the update path's actual field handling had zero coverage.
	// Several cases deliberately pick a renamed field (haKey != arg) -
	// exactly what CLAUDE.md's "manage_helper update field docs" gotcha and
	// this suite's regression target (splitAppliedFields reporting an
	// applied rename as ignored) are about.
	updateArg func(actionTargetEntityID string) (name string, value any)
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
		updateArg: func(string) (string, any) { return "code_arm_required", false },
	},
	{
		helperType: "template_button",
		domain:     "button",
		extraArgs: func(actionTargetEntityID string) map[string]any {
			return map[string]any{"press": toggleAction(actionTargetEntityID)}
		},
		updateArg: func(string) (string, any) { return "availability", "{{ true }}" },
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
		// "set_position" renames to "set_cover_position" - regression target
		// for the update-path haKey rename gap (CLAUDE.md's "manage_helper
		// update field docs" gotcha).
		updateArg: func(actionTargetEntityID string) (string, any) {
			return "set_position", toggleAction(actionTargetEntityID)
		},
	},
	{
		helperType: "template_device_tracker",
		domain:     "device_tracker",
		extraArgs: func(string) map[string]any {
			return map[string]any{"in_zones": "{{ 'home' }}"}
		},
		updateArg: func(string) (string, any) { return "latitude", "{{ 48.0 }}" },
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
		updateArg: func(string) (string, any) { return "event_type", "{{ 'updated_event' }}" },
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
		updateArg: func(string) (string, any) { return "speed_count", 5 },
	},
	{
		helperType: "template_image",
		domain:     "image",
		extraArgs: func(string) map[string]any {
			return map[string]any{"url": "{{ 'https://example.com/image.png' }}"}
		},
		updateArg: func(string) (string, any) { return "verify_ssl", false },
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
		updateArg: func(string) (string, any) { return "level", "{{ 100 }}" },
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
		// "lock_code_format" renames to "code_format" - regression target,
		// same reason as template_cover's "set_position" above.
		updateArg: func(string) (string, any) { return "lock_code_format", "{{ 'number' }}" },
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
		updateArg: func(string) (string, any) { return "step", 2 },
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
		// "options_template" renames to "options" - regression target, same
		// reason as template_cover's "set_position" above.
		updateArg: func(string) (string, any) { return "options_template", "{{ ['a', 'b', 'c'] }}" },
	},
	{
		helperType: "template_switch",
		domain:     "switch",
		extraArgs: func(string) map[string]any {
			return map[string]any{"state": "{{ 'off' }}"}
		},
		// "state" renames to "value_template" for this subtype only -
		// regression target, same reason as template_cover's "set_position"
		// above.
		updateArg: func(string) (string, any) { return "state", "{{ 'on' }}" },
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
		updateArg: func(string) (string, any) { return "title", "{{ 'v2' }}" },
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
		// "fan_speed_list" renames to "fan_speeds" - regression target,
		// same reason as template_cover's "set_position" above.
		updateArg: func(string) (string, any) { return "fan_speed_list", []any{"low", "high"} },
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
		updateArg: func(string) (string, any) { return "forecast_daily", "{{ [] }}" },
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

			updateFieldName, updateFieldValue := tc.updateArg(actionTargetEntityID)
			updateArgs := map[string]any{
				"action":    "update",
				"entity_id": reportedEntityID,
				"icon":      "mdi:test",
			}
			updateArgs[updateFieldName] = updateFieldValue
			updateResult := s.CallTool("manage_helper", updateArgs)
			updateText := resultText(updateResult)
			s.Require().False(updateResult.IsError, "manage_helper update(%s) should succeed, got: %s", tc.helperType, updateText)
			// Assert the update field actually reached Home Assistant - not
			// just "icon", which is applied via the Entity Registry and
			// bypasses the Options Flow config builder entirely (see
			// updateArg's doc comment above).
			applied := parenListAfter(updateText, "(applied: ")
			ignored := parenListAfter(updateText, "(ignored - not accepted by this helper type: ")
			s.Contains(applied, updateFieldName, "manage_helper update(%s) should report %q as applied, got: %s", tc.helperType, updateFieldName, updateText)
			s.NotContains(ignored, updateFieldName, "manage_helper update(%s) should not report %q as ignored, got: %s", tc.helperType, updateFieldName, updateText)

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

// parenListAfter extracts the comma-separated field list manage_helper's
// update success message renders after prefix (e.g. "(applied: "), up to
// the closing ")". Returns nil if prefix isn't present - the message omits
// a section entirely when its list would be empty.
func parenListAfter(msg, prefix string) []string {
	start := strings.Index(msg, prefix)
	if start == -1 {
		return nil
	}
	start += len(prefix)
	end := strings.Index(msg[start:], ")")
	if end == -1 {
		return nil
	}
	parts := strings.Split(msg[start:start+end], ", ")
	// updateSuccessMessage renders each field name via
	// homeassistant.BoundedFieldList, which wraps every name in %q (double
	// quotes) - strip them here so callers compare against plain field
	// names, not the quoted wire format.
	for i, p := range parts {
		parts[i] = strings.Trim(p, `"`)
	}
	return parts
}
