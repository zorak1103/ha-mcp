package homeassistant

import (
	"reflect"
	"testing"
)

func TestIndexStepSchema(t *testing.T) {
	t.Parallel()

	t.Run("empty schema", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema(nil)
		if !ix.empty {
			t.Errorf("indexStepSchema(nil).empty = false, want true")
		}
	})

	t.Run("top-level fields only", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{
			{Name: "a"},
			{Name: "b"},
		})
		if ix.empty {
			t.Errorf("indexStepSchema(...).empty = true, want false")
		}
		if _, _, ok := ix.placementOf("a"); !ok {
			t.Errorf("placementOf(a) not found")
		}
		if _, _, ok := ix.placementOf("z"); ok {
			t.Errorf("placementOf(z) unexpectedly found")
		}
	})

	t.Run("section with children", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{
			{Name: "state"},
			{
				Name: "additional_options",
				Type: "expandable",
				Schema: []OptionsFlowField{
					{Name: "availability"},
				},
			},
		})
		section, field, ok := ix.placementOf("availability")
		if !ok {
			t.Fatalf("placementOf(availability) not found")
		}
		if section != "additional_options" {
			t.Errorf("placementOf(availability) section = %q, want additional_options", section)
		}
		if field.Name != "availability" {
			t.Errorf("placementOf(availability) field.Name = %q, want availability", field.Name)
		}

		topSection, _, ok := ix.placementOf("state")
		if !ok {
			t.Fatalf("placementOf(state) not found")
		}
		if topSection != "" {
			t.Errorf("placementOf(state) section = %q, want empty (top-level)", topSection)
		}
	})
}

func TestBuildStepSubmission_CreateOmitsUnsuppliedOptionals(t *testing.T) {
	t.Parallel()

	ix := indexStepSchema([]OptionsFlowField{{Name: "a"}, {Name: "b"}})
	consumed := map[string]bool{}
	got := buildStepSubmission(flowModeCreate, ix, map[string]any{"a": "x"}, consumed, "user")

	want := map[string]any{"a": "x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildStepSubmission() = %v, want %v", got, want)
	}
	if !consumed["a"] {
		t.Errorf("consumed[a] = false, want true")
	}
	if consumed["b"] {
		t.Errorf("consumed[b] = true, want false (never supplied)")
	}
}

func TestBuildStepSubmission_UpdateRoundTripsSuggestedValues(t *testing.T) {
	t.Parallel()

	ix := indexStepSchema([]OptionsFlowField{
		{Name: "a", Description: map[string]any{"suggested_value": 1.0}},
		{Name: "b", Description: map[string]any{"suggested_value": 2.0}},
		{Name: "c", Description: map[string]any{"suggested_value": nil}},
	})
	consumed := map[string]bool{}
	got := buildStepSubmission(flowModeUpdate, ix, map[string]any{"a": 9.0}, consumed, "init")

	want := map[string]any{"a": 9.0, "b": 2.0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildStepSubmission() = %v, want %v (c must stay absent, not null)", got, want)
	}
}

func TestBuildStepSubmission_UpdateAlwaysReEmitsSection(t *testing.T) {
	t.Parallel()

	ix := indexStepSchema([]OptionsFlowField{
		{
			Name: "additional_options",
			Type: "expandable",
			Schema: []OptionsFlowField{
				{Name: "availability", Description: map[string]any{"suggested_value": "some template"}},
			},
		},
	})
	consumed := map[string]bool{}
	got := buildStepSubmission(flowModeUpdate, ix, map[string]any{}, consumed, "sensor")

	want := map[string]any{"additional_options": map[string]any{"availability": "some template"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildStepSubmission() = %v, want %v (section must always be re-emitted)", got, want)
	}
}

func TestBuildStepSubmission_RoutesUserFieldIntoSection(t *testing.T) {
	t.Parallel()

	ix := indexStepSchema([]OptionsFlowField{
		{
			Name: "additional_options",
			Type: "expandable",
			Schema: []OptionsFlowField{
				{Name: "availability"},
			},
		},
	})
	consumed := map[string]bool{}
	got := buildStepSubmission(flowModeCreate, ix, map[string]any{"availability": "{{ true }}"}, consumed, "sensor")

	want := map[string]any{"additional_options": map[string]any{"availability": "{{ true }}"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildStepSubmission() = %v, want %v", got, want)
	}
	if !consumed["availability"] {
		t.Errorf("consumed[availability] = false, want true")
	}
}

func TestBuildStepSubmission_MarksConsumedAndSkipsAlreadyConsumed(t *testing.T) {
	t.Parallel()

	// Two steps both declare "name" - only the first should take it.
	step1 := indexStepSchema([]OptionsFlowField{{Name: "name"}, {Name: "entity_id"}})
	step2 := indexStepSchema([]OptionsFlowField{{Name: "name"}, {Name: "sampling_size"}})

	consumed := map[string]bool{}
	userConfig := map[string]any{"name": "My Helper", "entity_id": "sensor.x", "sampling_size": 20.0}

	got1 := buildStepSubmission(flowModeCreate, step1, userConfig, consumed, "user")
	want1 := map[string]any{"name": "My Helper", "entity_id": "sensor.x"}
	if !reflect.DeepEqual(got1, want1) {
		t.Errorf("step1 buildStepSubmission() = %v, want %v", got1, want1)
	}

	got2 := buildStepSubmission(flowModeCreate, step2, userConfig, consumed, "options")
	want2 := map[string]any{"sampling_size": 20.0}
	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("step2 buildStepSubmission() = %v, want %v (name already consumed by step1)", got2, want2)
	}
}

func TestBuildStepSubmission_DurationFromSelector(t *testing.T) {
	t.Parallel()

	ix := indexStepSchema([]OptionsFlowField{
		{Name: "max_age", Selector: map[string]any{"duration": map[string]any{}}},
		{Name: "sampling_size", Selector: map[string]any{"number": map[string]any{}}},
	})
	consumed := map[string]bool{}
	got := buildStepSubmission(flowModeCreate, ix, map[string]any{
		"max_age":       90.0,
		"sampling_size": 90.0,
	}, consumed, "options")

	want := map[string]any{
		"max_age":       map[string]int{"hours": 0, "minutes": 1, "seconds": 30},
		"sampling_size": 90.0,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildStepSubmission() = %v, want %v", got, want)
	}
}

func TestBuildStepSubmission_DurationFallbackWhenNoSelector(t *testing.T) {
	t.Parallel()

	t.Run("name-keyed fallback", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{{Name: "time_window"}})
		consumed := map[string]bool{}
		got := buildStepSubmission(flowModeCreate, ix, map[string]any{"time_window": "00:05:00"}, consumed, "user")
		want := map[string]any{"time_window": map[string]int{"hours": 0, "minutes": 5, "seconds": 0}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("buildStepSubmission() = %v, want %v", got, want)
		}
	})

	t.Run("window_size on a duration filter step", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{{Name: "window_size"}})
		consumed := map[string]bool{}
		got := buildStepSubmission(flowModeCreate, ix, map[string]any{"window_size": 30.0}, consumed, "time_throttle")
		want := map[string]any{"window_size": map[string]int{"hours": 0, "minutes": 0, "seconds": 30}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("buildStepSubmission() = %v, want %v", got, want)
		}
	})

	t.Run("window_size on a non-duration filter step is left alone", func(t *testing.T) {
		t.Parallel()
		ix := indexStepSchema([]OptionsFlowField{{Name: "window_size"}})
		consumed := map[string]bool{}
		got := buildStepSubmission(flowModeCreate, ix, map[string]any{"window_size": 4.0}, consumed, "outlier")
		want := map[string]any{"window_size": 4.0}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("buildStepSubmission() = %v, want %v", got, want)
		}
	})
}

func TestBuildStepSubmission_EmptySchemaForwardsEverything(t *testing.T) {
	t.Parallel()

	ix := indexStepSchema(nil)
	consumed := map[string]bool{}
	got := buildStepSubmission(flowModeCreate, ix, map[string]any{"a": "x", "time_window": "00:00:05"}, consumed, "unknown")

	want := map[string]any{"a": "x", "time_window": map[string]int{"hours": 0, "minutes": 0, "seconds": 5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildStepSubmission() = %v, want %v", got, want)
	}
	if !consumed["a"] || !consumed["time_window"] {
		t.Errorf("consumed = %v, want both keys marked consumed", consumed)
	}
}

func TestBuildStepSubmission_ReadOnlyFieldRejectedOnUpdate(t *testing.T) {
	t.Parallel()

	ix := indexStepSchema([]OptionsFlowField{
		{
			Name:     "entity_id",
			Selector: map[string]any{"entity": map[string]any{"read_only": true}},
			Description: map[string]any{
				"suggested_value": "sensor.x",
			},
		},
	})
	consumed := map[string]bool{}
	got := buildStepSubmission(flowModeUpdate, ix, map[string]any{"entity_id": "sensor.y"}, consumed, "init")

	// The read-only field keeps its round-tripped value; the caller's
	// override is rejected, not silently applied, and not marked consumed
	// (so the caller sees it as an unhandled field, not a silent no-op).
	want := map[string]any{"entity_id": "sensor.x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildStepSubmission() = %v, want %v", got, want)
	}
	if consumed["entity_id"] {
		t.Errorf("consumed[entity_id] = true, want false (read-only fields are never consumed)")
	}
}

func TestSeedConsumedRoutingKeys(t *testing.T) {
	t.Parallel()

	consumed := seedConsumedRoutingKeys(HelperConfig{
		Platform: platformGenericThermostat,
		Config: map[string]any{
			"heater_entity_id":        "switch.x",
			"target_sensor_entity_id": "sensor.x",
			"icon":                    "mdi:thermostat",
		},
	})

	for _, key := range []string{"heater_entity_id", "target_sensor_entity_id", "icon", "group_type"} {
		if !consumed[key] {
			t.Errorf("seedConsumedRoutingKeys()[%q] = false, want true", key)
		}
	}
}

// TestSeedConsumedRoutingKeys_LeavesRealStepFieldsClaimable is a regression
// test: platformSkipFields historically blanket-stripped some keys under
// the assumption they were pure routing markers HA's schema never
// declares. That assumption is wrong for two of them - HA's real schema
// requires both as actual submitted fields on a specific step - so
// pre-consuming them globally made that step's own schema unable to ever
// claim them, and HA rejected the submission with "required key not
// provided". Unlike template_type/type/group_type (which truly never
// appear in any step's schema), these must be left unconsumed so
// buildStepSubmission's normal per-step schema routing picks them up.
func TestSeedConsumedRoutingKeys_LeavesRealStepFieldsClaimable(t *testing.T) {
	t.Parallel()

	t.Run("statistics state_characteristic (HA's dedicated step requires it)", func(t *testing.T) {
		t.Parallel()
		consumed := seedConsumedRoutingKeys(HelperConfig{
			Platform: platformStatistics,
			Config:   map[string]any{"state_characteristic": "mean", "entity_id": "sensor.x"},
		})
		if consumed["state_characteristic"] {
			t.Error(`seedConsumedRoutingKeys()["state_characteristic"] = true, want false - HA's state_characteristic step requires this field`)
		}
	})

	t.Run("switch_as_x target_domain (HA's single user step requires it)", func(t *testing.T) {
		t.Parallel()
		consumed := seedConsumedRoutingKeys(HelperConfig{
			Platform: platformSwitchAsX,
			Config:   map[string]any{"target_domain": "light", "entity_id": "switch.x"},
		})
		if consumed["target_domain"] {
			t.Error(`seedConsumedRoutingKeys()["target_domain"] = true, want false - HA's switch_as_x "user" step requires this field`)
		}
	})
}

func TestFindMatchingMenuOption_PrefersExactMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		menuOptions  []string
		entityDomain string
		want         string
	}{
		{
			name:         "exact match preferred over substring collision (issue: sensor is a substring of binary_sensor)",
			menuOptions:  []string{"binary_sensor", "sensor"},
			entityDomain: "sensor",
			want:         "sensor",
		},
		{
			name:         "exact match preferred regardless of menu order",
			menuOptions:  []string{"sensor", "binary_sensor"},
			entityDomain: "binary_sensor",
			want:         "binary_sensor",
		},
		{
			name:         "falls back to substring match when no exact match exists",
			menuOptions:  []string{"binary_sensor"},
			entityDomain: "sensor",
			want:         "binary_sensor",
		},
		{
			name:         "no match at all returns empty",
			menuOptions:  []string{"switch", "fan"},
			entityDomain: "sensor",
			want:         "",
		},
		{
			name:         "full 17-option template menu, exact match wins",
			entityDomain: "sensor",
			menuOptions: []string{
				"alarm_control_panel", "binary_sensor", "button", "cover",
				"device_tracker", "event", "fan", "image", "light", "lock",
				"number", "select", "sensor", "switch", "update", "vacuum",
				"weather",
			},
			want: "sensor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findMatchingMenuOption(tt.menuOptions, tt.entityDomain)
			if got != tt.want {
				t.Errorf("findMatchingMenuOption(%v, %q) = %q, want %q", tt.menuOptions, tt.entityDomain, got, tt.want)
			}
		})
	}
}
