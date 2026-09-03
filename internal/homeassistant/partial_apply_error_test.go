package homeassistant

import (
	"strings"
	"testing"
)

// TestPartialApplyError_CreateMessageNamesPlatformAndFields pins the
// create-side wording: it must name the config-entry platform (Platform is
// the real integration platform name on create, unlike update - see
// PartialApplyOp's doc comment) and every unconsumed field.
func TestPartialApplyError_CreateMessageNamesPlatformAndFields(t *testing.T) {
	t.Parallel()

	err := &PartialApplyError{Op: PartialApplyCreate, Platform: "generic_thermostat", Fields: []string{"away_temp"}}

	msg := err.Error()
	if !strings.Contains(msg, "away_temp") {
		t.Errorf("Error() = %q, want it to name the unaccepted field away_temp", msg)
	}
	if !strings.Contains(msg, "generic_thermostat") {
		t.Errorf("Error() = %q, want it to name the config-entry platform", msg)
	}
	if !strings.Contains(msg, "config flow") {
		t.Errorf("Error() = %q, want it to say \"config flow\" for the create case", msg)
	}
	if !strings.Contains(msg, "NOT been applied") {
		t.Errorf("Error() = %q, want it to say the field(s) were NOT applied", msg)
	}
}

// TestPartialApplyError_UpdateMessageOmitsPlatform pins the update-side
// wording: HelperConfig.Platform on the update path is the entity DOMAIN,
// not the real integration platform (CLAUDE.md's ParseHelperEntityID
// gotcha) - naming it here would be actively misleading, so the update
// message must not depend on Platform at all.
func TestPartialApplyError_UpdateMessageOmitsPlatform(t *testing.T) {
	t.Parallel()

	err := &PartialApplyError{Op: PartialApplyUpdate, Platform: "climate", Fields: []string{"nonsense_temp"}}

	msg := err.Error()
	if !strings.Contains(msg, "nonsense_temp") {
		t.Errorf("Error() = %q, want it to name the unaccepted field", msg)
	}
	if strings.Contains(msg, "climate") {
		t.Errorf("Error() = %q, must not name the entity domain as if it were the platform", msg)
	}
	if !strings.Contains(msg, "options flow") {
		t.Errorf("Error() = %q, want it to say \"options flow\" for the update case", msg)
	}
	if !strings.Contains(msg, "NOT been applied") {
		t.Errorf("Error() = %q, want it to say the field(s) were NOT applied", msg)
	}
}
