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
// returned slice's order (and, for two arg names sharing one HA config
// key - "code_format"/"lock_code_format" both write "code_format" for
// template_lock - which one is processed, and therefore wins, last) would
// vary from one call to the next.
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

// TestResolveTemplateFieldsForDomain_LockCodeFormatRenameWins pins the
// specific collision the sort fix resolves: template_lock's own
// "lock_code_format" (haKey "code_format") and template_alarm_control_panel's
// unrelated, unambiguous "code_format" (no haKey) both resolve to the same
// HA config key "code_format" once matched/included for domain "lock".
// Sorting by arg name makes "code_format" (alarm's, alphabetically first)
// process before "lock_code_format" (lock's own field) - so lock's own
// field deterministically wins as the last write, which is what
// addTemplateConfigEntryUpdateFields relies on for a template_lock update.
func TestResolveTemplateFieldsForDomain_LockCodeFormatRenameWins(t *testing.T) {
	t.Parallel()

	fields := resolveTemplateFieldsForDomain("lock")
	var sawCodeFormat, sawLockCodeFormat bool
	var codeFormatIdx, lockCodeFormatIdx int
	for i, f := range fields {
		switch f.arg {
		case "code_format":
			sawCodeFormat = true
			codeFormatIdx = i
		case "lock_code_format":
			sawLockCodeFormat = true
			lockCodeFormatIdx = i
			if f.configKey() != "code_format" {
				t.Errorf("lock_code_format.configKey() = %q, want %q", f.configKey(), "code_format")
			}
		}
	}
	if !sawCodeFormat || !sawLockCodeFormat {
		t.Fatalf("resolveTemplateFieldsForDomain(%q) = %+v, want both code_format and lock_code_format present", "lock", fields)
	}
	if lockCodeFormatIdx < codeFormatIdx {
		t.Errorf("lock_code_format at index %d, code_format at index %d - lock's own field must be processed last to win", lockCodeFormatIdx, codeFormatIdx)
	}
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
