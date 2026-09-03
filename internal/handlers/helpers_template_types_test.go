package handlers

import (
	"reflect"
	"slices"
	"testing"
)

// TestTemplateSubtypeTable_CoversHATemplateTypes pins the templateSubtypes
// table's shape: exactly the 15 domains HA's TEMPLATE_TYPES declares
// beyond the existing template_sensor/template_binary_sensor, and internal
// consistency for any arg name shared by more than one subtype (a name
// declared with two different kinds, for example, would silently produce
// an inconsistent schema property depending on map iteration order).
func TestTemplateSubtypeTable_CoversHATemplateTypes(t *testing.T) {
	t.Parallel()

	wantDomains := map[string]bool{
		"alarm_control_panel": true,
		"button":              true,
		"cover":               true,
		"device_tracker":      true,
		"event":               true,
		"fan":                 true,
		"image":               true,
		"light":               true,
		"lock":                true,
		"number":              true,
		"select":              true,
		"switch":              true,
		"update":              true,
		"vacuum":              true,
		"weather":             true,
	}

	if len(templateSubtypes) != len(wantDomains) {
		t.Fatalf("templateSubtypes has %d entries, want %d", len(templateSubtypes), len(wantDomains))
	}

	gotDomains := make(map[string]bool, len(templateSubtypes))
	for typeName, subtype := range templateSubtypes {
		if typeName != "template_"+subtype.domain {
			t.Errorf("type %q has domain %q, want type name template_%s", typeName, subtype.domain, subtype.domain)
		}
		if gotDomains[subtype.domain] {
			t.Errorf("domain %q is declared by more than one type", subtype.domain)
		}
		gotDomains[subtype.domain] = true
	}
	for domain := range wantDomains {
		if !gotDomains[domain] {
			t.Errorf("missing template subtype for HA domain %q", domain)
		}
	}
	for domain := range gotDomains {
		if !wantDomains[domain] {
			t.Errorf("unexpected template subtype for domain %q - not in HA's TEMPLATE_TYPES, or wantDomains needs updating", domain)
		}
	}
}

// TestTemplateSubtypeTable_SharedArgNamesAgree asserts that whenever two
// subtypes declare a field with the same arg name, they agree on kind -
// declaring the SAME name with two different kinds would make
// templateHelperProperties()'s dedup silently pick one arbitrarily,
// producing a wrong schema Type for whichever subtype lost the race.
// Different haKey/required values for the same arg ARE expected (that's
// exactly the ambiguity resolveTemplateFieldsForDomain resolves) and are
// not checked here.
func TestTemplateSubtypeTable_SharedArgNamesAgree(t *testing.T) {
	t.Parallel()

	kindByArg := make(map[string]templateFieldKind)
	for typeName, subtype := range templateSubtypes {
		for _, f := range subtype.fields {
			if existing, seen := kindByArg[f.arg]; seen && existing != f.kind {
				t.Errorf("arg %q has kind %v in an earlier subtype but kind %v in %q - schema Type would be ambiguous", f.arg, existing, f.kind, typeName)
				continue
			}
			kindByArg[f.arg] = f.kind
		}
	}
}

// TestTemplateSubtypeTable_InclusivePairsReferenceDeclaredFields pins that
// every inclusivePairs entry names an arg the same subtype actually
// declares in its fields list - subtypeConfigKey silently falls back to the
// arg name itself for an unknown field, which would make a typo in
// inclusivePairs fail to compile but also fail to ever be checked.
func TestTemplateSubtypeTable_InclusivePairsReferenceDeclaredFields(t *testing.T) {
	t.Parallel()

	for typeName, subtype := range templateSubtypes {
		declared := make(map[string]bool, len(subtype.fields))
		for _, f := range subtype.fields {
			declared[f.arg] = true
		}
		for _, pair := range subtype.inclusivePairs {
			for _, arg := range pair {
				if !declared[arg] {
					t.Errorf("%s: inclusivePairs references undeclared field %q", typeName, arg)
				}
			}
		}
	}
}

// TestBuildTemplateHelperConfig_CoverOpenCloseInclusivePair pins the C1 fix:
// checkInclusivePairs must resolve "open"/"close" to their renamed HA
// config keys ("open_cover"/"close_cover") before checking presence - an
// earlier version checked the caller-facing arg names directly against the
// post-rename config map and could never find them, silently accepting an
// open-without-close config that HA's own vol.Inclusive schema rejects.
func TestBuildTemplateHelperConfig_CoverOpenCloseInclusivePair(t *testing.T) {
	t.Parallel()

	build := buildTemplateHelperConfig("template_cover")
	openAction := map[string]any{"action": "switch.turn_on"}
	closeAction := map[string]any{"action": "switch.turn_off"}

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{
			name:    "open without close fails",
			args:    map[string]any{"state": "{{ 1 }}", "open": openAction},
			wantErr: true,
		},
		{
			name:    "close without open fails",
			args:    map[string]any{"state": "{{ 1 }}", "close": closeAction},
			wantErr: true,
		},
		{
			name:    "both supplied succeeds",
			args:    map[string]any{"state": "{{ 1 }}", "open": openAction, "close": closeAction},
			wantErr: false,
		},
		{
			name:    "neither supplied succeeds",
			args:    map[string]any{"state": "{{ 1 }}"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := map[string]any{}
			err := build(config, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildTemplateHelperConfig(%q)(...) error = %v, wantErr %v", "template_cover", err, tt.wantErr)
			}
		})
	}
}

// TestResolveTemplateFieldsForDomain_IncludesUnambiguousFieldNotOwnedByAnySubtypeDomain
// pins the non-ambiguous branch of resolveTemplateFieldsForDomain's
// len(configKeysByArg[arg]) > 1 check: "fan_speed_list" is declared by
// exactly one subtype (template_vacuum), so it has exactly one config key
// regardless of which subtype "first" comes from - it must still be
// resolved (not skipped) for a domain that owns none of the 15 subtypes,
// same as a real template_sensor/template_binary_sensor update. A ">="
// mutant here would wrongly treat "exactly one config key" as ambiguous
// and drop every single-owner field whenever the domain doesn't match.
func TestResolveTemplateFieldsForDomain_IncludesUnambiguousFieldNotOwnedByAnySubtypeDomain(t *testing.T) {
	t.Parallel()

	fields := resolveTemplateFieldsForDomain("sensor")
	for _, f := range fields {
		if f.arg == "fan_speed_list" {
			return
		}
	}
	t.Fatalf("resolveTemplateFieldsForDomain(%q) dropped unambiguous field %q; fields = %+v", "sensor", "fan_speed_list", fields)
}

// TestResolveTemplateFieldsForDomain_ReturnsFieldsSortedByArgName pins the
// determinism fix for resolveTemplateFieldsForDomain's result order:
// firstByArg/matchedByArg are maps, so without an explicit sort the
// returned slice's order - and, for two arg names that are both single-owner
// but resolve to the same HA config key for a domain that owns neither
// (dropped by the collision check either way, but which one gets processed
// first still matters if a future change relaxes that check) - would vary
// from one call to the next.
func TestResolveTemplateFieldsForDomain_ReturnsFieldsSortedByArgName(t *testing.T) {
	t.Parallel()

	for _, domain := range append(templateSubtypeNames(), "sensor", "climate") {
		fields := resolveTemplateFieldsForDomain(domain)
		args := make([]string, len(fields))
		for i, f := range fields {
			args[i] = f.arg
		}
		if !slices.IsSorted(args) {
			t.Errorf("resolveTemplateFieldsForDomain(%q) args = %v, want sorted by arg name", domain, args)
		}
	}
}

// TestResolveTemplateFieldsForDomain_DeterministicAcrossCalls guards
// against exactly the class of bug TestResolveTemplateFieldsForDomain_ReturnsFieldsSortedByArgName
// pins from the other direction: Go's map iteration order is randomized
// per range statement, not just per process, so a flake here would only
// have shown up intermittently before the sort fix - repeat the call many
// times in one test run rather than relying on a single comparison.
func TestResolveTemplateFieldsForDomain_DeterministicAcrossCalls(t *testing.T) {
	t.Parallel()

	want := resolveTemplateFieldsForDomain("lock")
	for i := range 50 {
		got := resolveTemplateFieldsForDomain("lock")
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("resolveTemplateFieldsForDomain(%q) call %d = %+v, want %+v", "lock", i, got, want)
		}
	}
}

// TestResolveTemplateFieldsForDomain_DropsCollidingUnmatchedField pins the
// W5 fix (issue #206 adversarial review): template_alarm_control_panel's
// bare "code_format" (single-owner, so it would otherwise pass the
// ambiguity check) resolves to the same HA config key ("code_format") as
// template_lock's own "lock_code_format" (haKey "code_format") for domain
// "lock". An earlier version kept both and relied on sorted-processing
// order ("code_format" < "lock_code_format") for lock's own field to win as
// the last write - which meant a caller updating a lock via the plain
// "code_format" arg name got it written straight through instead of being
// reported as unresolved, defeating the very rename template_lock exists
// for. resolveTemplateFieldsForDomain must now drop the colliding
// non-matched field entirely rather than merely order it first.
func TestResolveTemplateFieldsForDomain_DropsCollidingUnmatchedField(t *testing.T) {
	t.Parallel()

	fields := resolveTemplateFieldsForDomain("lock")
	var sawCodeFormat, sawLockCodeFormat bool
	for _, f := range fields {
		switch f.arg {
		case "code_format":
			sawCodeFormat = true
		case "lock_code_format":
			sawLockCodeFormat = true
			if f.configKey() != "code_format" {
				t.Errorf("lock_code_format.configKey() = %q, want %q", f.configKey(), "code_format")
			}
		}
	}
	if sawCodeFormat {
		t.Errorf("resolveTemplateFieldsForDomain(%q) includes colliding unmatched field %q; fields = %+v", "lock", "code_format", fields)
	}
	if !sawLockCodeFormat {
		t.Fatalf("resolveTemplateFieldsForDomain(%q) is missing the domain's own field %q; fields = %+v", "lock", "lock_code_format", fields)
	}
}

// TestResolveTemplateFieldsForDomain_AlarmControlPanelKeepsItsOwnCodeFormat
// pins the other side of the W5 fix: for the domain that actually owns the
// bare "code_format" arg, it must still be resolved, not dropped by the
// collision check (matchedConfigKeys is built from THIS domain's matched
// fields, so alarm's own "code_format" naturally collides with itself and
// must not be treated as a foreign collision).
func TestResolveTemplateFieldsForDomain_AlarmControlPanelKeepsItsOwnCodeFormat(t *testing.T) {
	t.Parallel()

	fields := resolveTemplateFieldsForDomain("alarm_control_panel")
	for _, f := range fields {
		if f.arg == "code_format" {
			return
		}
	}
	t.Fatalf("resolveTemplateFieldsForDomain(%q) dropped its own field %q; fields = %+v", "alarm_control_panel", "code_format", fields)
}

// TestTemplateHelperProperties_DeterministicAcrossCalls guards against
// templateHelperProperties' addField "first description wins" dedup
// depending on map iteration order (templateSubtypes is a map) - several
// arg names ("turn_on", "turn_off", "open", "stop", "temperature") are
// declared by more than one subtype with a different desc, so an
// unsorted iteration would make the emitted manage_helper schema text
// vary from one call to the next.
func TestTemplateHelperProperties_DeterministicAcrossCalls(t *testing.T) {
	t.Parallel()

	want := templateHelperProperties()
	for i := range 50 {
		got := templateHelperProperties()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("templateHelperProperties() call %d differs from first call", i)
		}
	}
}
