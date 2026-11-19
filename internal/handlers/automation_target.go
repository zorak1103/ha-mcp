package handlers

import (
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// automationEntityPrefix is the entity ID domain prefix of automations.
const automationEntityPrefix = "automation."

// resolveAutomationTarget picks the automation a single-item action works on.
// automation_id is used as given when present, otherwise entity_id; callers keep
// their own downstream handling (normalize, GetAutomation, findAutomationByID).
//
// entity_id is only accepted as a target when it carries the "automation."
// prefix: on list the same argument filters by referenced entity, so a value
// such as "light.kitchen" must never silently fall back to filtering or to an
// alias search. When both arguments are given they must name the same
// automation, otherwise the call is rejected instead of silently picking one.
// A non-string value is rejected as well, because the server's type check lets
// a JSON number through for a string property. action and missingHint only
// shape the error texts; args stays the last parameter for the schema audit.
//
// The returned error result is non-nil exactly when target is empty.
func resolveAutomationTarget(action, missingHint string, args map[string]any) (target string, errResult *mcp.ToolsCallResult) {
	automationID, errMsg := stringIDValue("automation_id", args["automation_id"])
	if errMsg != "" {
		return "", errorResult(errMsg)
	}
	entityID, errMsg := stringIDValue(attrEntityID, args[attrEntityID])
	if errMsg != "" {
		return "", errorResult(errMsg)
	}

	if entityID != "" && !strings.HasPrefix(entityID, automationEntityPrefix) {
		return "", errorResult(fmt.Sprintf(
			"entity_id %q is not an automation: action=%s takes the automation as automation_id or entity_id 'automation.<id>'",
			entityID, action))
	}

	switch {
	case automationID != "" && entityID != "":
		if resolved, _ := normalizeAutomationID(automationID); resolved != entityID {
			return "", errorResult(fmt.Sprintf(
				"automation_id %q conflicts with entity_id %q: pass only one of them", automationID, entityID))
		}
		return automationID, nil
	case automationID != "":
		return automationID, nil
	case entityID != "":
		return entityID, nil
	}

	msg := fmt.Sprintf("automation_id is required for %s action (or pass entity_id 'automation.<id>')", action)
	if missingHint != "" {
		msg += ". " + missingHint
	}
	return "", errorResult(msg)
}

// stringIDValue turns a raw argument value into a string. An absent value (see
// isAbsentArg) yields ("", ""); anything that is not a string yields an error
// message naming the argument and the offending type.
func stringIDValue(name string, v any) (value, errMsg string) {
	if isAbsentArg(v) {
		return "", ""
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Sprintf("%s must be a string, got %T", name, v)
	}
	return s, ""
}
