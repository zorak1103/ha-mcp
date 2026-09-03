package homeassistant

import "fmt"

// PartialApplyOp distinguishes the create-flow and update-flow wordings of
// PartialApplyError.Error(). Create's HelperConfig.Platform is the real
// config-entry platform name and safe to name in the message; update's is
// the entity DOMAIN (e.g. "climate"), not the platform (see CLAUDE.md's
// ParseHelperEntityID gotcha - handleUpdate's call site never resolves the
// real platform before reaching updateHelperViaOptionsFlow), so naming it
// as if it were the platform would be actively misleading.
type PartialApplyOp string

// PartialApplyCreate and PartialApplyUpdate select PartialApplyError's
// create-flow vs. update-flow message wording.
const (
	PartialApplyCreate PartialApplyOp = "create"
	PartialApplyUpdate PartialApplyOp = "update"
)

// PartialApplyError reports a Config Entry Flow mutation that COMMITTED on
// Home Assistant's side while some caller-supplied fields were rejected by
// every step's schema and were never applied. It exists so
// internal/handlers can tell this outcome apart from a genuine failure -
// the entity/config entry already exists, so a caller must not retry the
// mutation - and surface it as a successful result carrying a warning
// rather than as an error. Callers unwrap it with errors.As.
type PartialApplyError struct {
	Op       PartialApplyOp
	Platform string   // config-entry platform - meaningful for PartialApplyCreate only
	Fields   []string // config keys no flow step claimed
}

func (e *PartialApplyError) Error() string {
	if e.Op == PartialApplyUpdate {
		return fmt.Sprintf("field(s) %s were not accepted by any step of its options flow and have NOT been applied",
			BoundedFieldList(e.Fields))
	}
	return fmt.Sprintf("field(s) %s were not accepted by any step of the %s config flow and have NOT been applied",
		BoundedFieldList(e.Fields), e.Platform)
}
