package handlers

import "testing"

// perTypeCreateExcludedFields declares, per helper type, any create-time
// field that is legitimately not read by the create builder. Empty today -
// every field in requiredFields/optionalFields is read by buildHelperConfig
// or the config-entry create builder for its type. A future genuine gap
// should be declared here (with a comment explaining why), the same
// discipline perTypeUpdateExcludedFields already applies on the update
// side - not silently tolerated by loosening the test below.
var perTypeCreateExcludedFields = map[string]map[string]bool{}

func isCreateExcludedField(typeName, field string) bool {
	return perTypeCreateExcludedFields[typeName][field]
}

// TestCreatableFields_AreActuallyReadByCreatePath is the create-path mirror
// of TestUpdatableFields_AreActuallyReadByUpdatePath. Before the fix for
// tmp/issue.md, input_number's "initial" was read by buildInputNumberConfig
// but silently dropped whenever the schema's declared "string" type reached
// the builder verbatim (a real MCP client obeying the schema would send
// "3000", not 3000.0) - a bug this specific test could not have caught,
// since it feeds correctly-typed sentinels. It exists to catch the
// simpler, complementary defect: a field declared in helperTypes that no
// builder reads at all, for any type.
//
// Unlike updatableFieldNames, this iterates requiredFields+optionalFields
// directly with NO isUpdateExcludedField filtering - create legitimately
// reads entity_id and filter (the two names update's identifier/per-type
// exclusions strip), since create's args map doesn't yet have an existing
// entity to identify.
func TestCreatableFields_AreActuallyReadByCreatePath(t *testing.T) {
	t.Parallel()

	for name, meta := range helperTypes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fields := append(append([]string{}, meta.requiredFields...), meta.optionalFields...)
			args := make(map[string]any, len(fields))
			for _, field := range fields {
				args[field] = updatableFieldSentinel(name, field)
			}

			config, err := buildHelperConfig(name, "Test Name", args)
			if err != nil {
				t.Fatalf("buildHelperConfig(%q, ...) returned error: %v", name, err)
			}

			for _, field := range fields {
				if isCreateExcludedField(name, field) {
					continue
				}
				key := field
				if alias, ok := updateConfigKeyAliases[field]; ok {
					key = alias
				}
				if _, present := config[key]; !present {
					t.Errorf("field %q (config key %q) is declared for %q but was not read by the create builder - add it to perTypeCreateExcludedFields or fix the builder", field, key, name)
				}
			}
		})
	}
}
