package homeassistant

import "testing"

// TestWSHelperAndConfigEntryPlatforms_AreDisjoint guards the read/write
// platform-gate split found in review: UpdateHelper (and now GetHelperConfig)
// gate on the wsHelperPlatforms allow-list, while the merge path in
// internal/handlers gates on !RequiresConfigEntryFlow (configEntryPlatforms,
// a deny-list). The two maps only cover the same platform space correctly
// because they happen to be exact complements today - nothing in the type
// system enforces that. If a platform were ever added to both (or a typo'd
// duplicate landed in one), a caller relying on RequiresConfigEntryFlow
// returning false to mean "safe to call GetHelperConfig/UpdateHelper" would
// silently regress. This test only proves the two known maps never overlap;
// it is not a check that their union is exhaustive (that a new helper type's
// author is not permitted to forget it) - see CLAUDE.md's "Extending
// Consolidated Tools" checklist for the exhaustiveness diligence needed on
// each new type.
func TestWSHelperAndConfigEntryPlatforms_AreDisjoint(t *testing.T) {
	t.Parallel()

	for platform := range wsHelperPlatforms {
		if configEntryPlatforms[platform] {
			t.Errorf("platform %q is in both wsHelperPlatforms and configEntryPlatforms - "+
				"isWSHelperPlatform and RequiresConfigEntryFlow would both return true for it, "+
				"an invariant the merge gate in internal/handlers relies on holding", platform)
		}
	}
}
