package handlers

import "testing"

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
