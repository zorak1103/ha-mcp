package handlers

import (
	"testing"
)

func TestVerboseHint(t *testing.T) {
	t.Parallel()

	if VerboseHint == "" {
		t.Error("VerboseHint is empty, want non-empty")
	}
}

func TestHelperPlatforms(t *testing.T) {
	t.Parallel()

	if len(HelperPlatforms) == 0 {
		t.Error("HelperPlatforms is empty, want non-empty")
	}

	// Check that expected platforms are present
	expectedPlatforms := []string{
		"input_boolean",
		"input_number",
		"input_text",
		"input_select",
		"input_datetime",
		"input_button",
		"counter",
		"timer",
		"schedule",
	}

	for _, expected := range expectedPlatforms {
		found := false
		for _, platform := range HelperPlatforms {
			if platform == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("HelperPlatforms missing expected platform %q", expected)
		}
	}
}

func TestParseHelperEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		entityID     string
		wantPlatform string
		wantID       string
	}{
		{
			name:         "input_boolean entity",
			entityID:     "input_boolean.my_switch",
			wantPlatform: "input_boolean",
			wantID:       "my_switch",
		},
		{
			name:         "input_number entity",
			entityID:     "input_number.temperature_setting",
			wantPlatform: "input_number",
			wantID:       "temperature_setting",
		},
		{
			name:         "input_text entity",
			entityID:     "input_text.user_name",
			wantPlatform: "input_text",
			wantID:       "user_name",
		},
		{
			name:         "input_select entity",
			entityID:     "input_select.mode_selector",
			wantPlatform: "input_select",
			wantID:       "mode_selector",
		},
		{
			name:         "input_datetime entity",
			entityID:     "input_datetime.alarm_time",
			wantPlatform: "input_datetime",
			wantID:       "alarm_time",
		},
		{
			name:         "input_button entity",
			entityID:     "input_button.restart_button",
			wantPlatform: "input_button",
			wantID:       "restart_button",
		},
		{
			name:         "counter entity",
			entityID:     "counter.visit_count",
			wantPlatform: "counter",
			wantID:       "visit_count",
		},
		{
			name:         "timer entity",
			entityID:     "timer.cooking_timer",
			wantPlatform: "timer",
			wantID:       "cooking_timer",
		},
		{
			name:         "schedule entity",
			entityID:     "schedule.weekly_schedule",
			wantPlatform: "schedule",
			wantID:       "weekly_schedule",
		},
		{
			name:         "group entity",
			entityID:     "group.all_lights",
			wantPlatform: "group",
			wantID:       "all_lights",
		},
		{
			name:         "threshold entity",
			entityID:     "threshold.temperature_high",
			wantPlatform: "threshold",
			wantID:       "temperature_high",
		},
		{
			name:         "derivative entity",
			entityID:     "derivative.power_rate",
			wantPlatform: "derivative",
			wantID:       "power_rate",
		},
		{
			name:         "integration entity",
			entityID:     "integration.energy_total",
			wantPlatform: "integration",
			wantID:       "energy_total",
		},
		{
			name:         "template entity",
			entityID:     "template.calculated_value",
			wantPlatform: "template",
			wantID:       "calculated_value",
		},
		// Entity platforms created by the 15 new template_* subtypes (issue #206).
		{
			name:         "alarm_control_panel entity (template_alarm_control_panel)",
			entityID:     "alarm_control_panel.my_alarm",
			wantPlatform: "alarm_control_panel",
			wantID:       "my_alarm",
		},
		{
			name:         "button entity (template_button)",
			entityID:     "button.my_button",
			wantPlatform: "button",
			wantID:       "my_button",
		},
		{
			name:         "cover entity (template_cover)",
			entityID:     "cover.my_cover",
			wantPlatform: "cover",
			wantID:       "my_cover",
		},
		{
			name:         "device_tracker entity (template_device_tracker)",
			entityID:     "device_tracker.my_tracker",
			wantPlatform: "device_tracker",
			wantID:       "my_tracker",
		},
		{
			name:         "event entity (template_event)",
			entityID:     "event.my_event",
			wantPlatform: "event",
			wantID:       "my_event",
		},
		{
			name:         "fan entity (template_fan)",
			entityID:     "fan.my_fan",
			wantPlatform: "fan",
			wantID:       "my_fan",
		},
		{
			name:         "image entity (template_image)",
			entityID:     "image.my_image",
			wantPlatform: "image",
			wantID:       "my_image",
		},
		{
			name:         "light entity (template_light)",
			entityID:     "light.my_light",
			wantPlatform: "light",
			wantID:       "my_light",
		},
		{
			name:         "lock entity (template_lock)",
			entityID:     "lock.my_lock",
			wantPlatform: "lock",
			wantID:       "my_lock",
		},
		{
			name:         "number entity (template_number)",
			entityID:     "number.my_number",
			wantPlatform: "number",
			wantID:       "my_number",
		},
		{
			name:         "switch entity (template_switch)",
			entityID:     "switch.my_switch",
			wantPlatform: "switch",
			wantID:       "my_switch",
		},
		{
			name:         "update entity (template_update)",
			entityID:     "update.my_update",
			wantPlatform: "update",
			wantID:       "my_update",
		},
		{
			name:         "vacuum entity (template_vacuum)",
			entityID:     "vacuum.my_vacuum",
			wantPlatform: "vacuum",
			wantID:       "my_vacuum",
		},
		{
			name:         "weather entity (template_weather)",
			entityID:     "weather.my_weather",
			wantPlatform: "weather",
			wantID:       "my_weather",
		},
		// switch_as_x's remaining target domains (issue #206 side fix).
		{
			name:         "siren entity (switch_as_x)",
			entityID:     "siren.my_siren",
			wantPlatform: "siren",
			wantID:       "my_siren",
		},
		{
			name:         "valve entity (switch_as_x)",
			entityID:     "valve.my_valve",
			wantPlatform: "valve",
			wantID:       "my_valve",
		},
		{
			name:         "unknown platform",
			entityID:     "media_player.living_room",
			wantPlatform: "",
			wantID:       "",
		},
		{
			name:         "empty entity_id",
			entityID:     "",
			wantPlatform: "",
			wantID:       "",
		},
		{
			name:         "no dot separator",
			entityID:     "input_boolean_my_switch",
			wantPlatform: "",
			wantID:       "",
		},
		{
			name:         "entity with multiple dots",
			entityID:     "input_boolean.my.switch.test",
			wantPlatform: "input_boolean",
			wantID:       "my.switch.test",
		},
		{
			name:         "entity with underscores",
			entityID:     "input_number.my_long_entity_name",
			wantPlatform: "input_number",
			wantID:       "my_long_entity_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotPlatform, gotID := ParseHelperEntityID(tt.entityID)

			if gotPlatform != tt.wantPlatform {
				t.Errorf("ParseHelperEntityID(%q) platform = %q, want %q", tt.entityID, gotPlatform, tt.wantPlatform)
			}
			if gotID != tt.wantID {
				t.Errorf("ParseHelperEntityID(%q) id = %q, want %q", tt.entityID, gotID, tt.wantID)
			}
		})
	}
}

// TestHelperPlatforms_CoversEveryHelperTypeValidEntityDomain pins that
// HelperPlatforms (this file) lists every entity domain any helperTypes
// entry declares in validEntityDomains (helpers_consolidated.go). A domain
// missing from HelperPlatforms breaks ParseHelperEntityID for that type's
// entities outright (issue #211's regression class); a domain present here
// but never checked against buildNewlyWidenedHelperDomains'
// preExistingHelperOnlyDomains would silently widen checkHelperOnlyDomain's
// gated set too - this test is the trip wire for the first half of that,
// so a future helper type addition can't forget HelperPlatforms the way
// this branch had to add 16 domains for by hand.
func TestHelperPlatforms_CoversEveryHelperTypeValidEntityDomain(t *testing.T) {
	t.Parallel()

	platforms := make(map[string]bool, len(HelperPlatforms))
	for _, p := range HelperPlatforms {
		platforms[p] = true
	}
	for typeName, meta := range helperTypes {
		for _, domain := range meta.validEntityDomains {
			if !platforms[domain] {
				t.Errorf("helper type %q declares validEntityDomains domain %q, missing from HelperPlatforms", typeName, domain)
			}
		}
	}
}
