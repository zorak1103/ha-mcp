//go:build integration

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/jsonorder"
)

// KeyOrderTestSuite verifies issue #311 end to end: Home Assistant renders an
// automation's/script's "variables" sequentially in document order, so the
// tools must write maps back in the order HA stored (or the caller sent), never
// alphabetically. Every assertion reads the stored config back from HA.
type KeyOrderTestSuite struct {
	AutomationTestSuite
}

func TestKeyOrderIntegration(t *testing.T) {
	suite.Run(t, new(KeyOrderTestSuite))
}

// keyOrderEvent is fired by the last action of every test config so that the
// config contains a template that depends on an earlier variable ("alpha").
const keyOrderEvent = "mcptest_key_order_done"

// keysOf returns the document-order keys of an order node (nil-safe).
func keysOf(o *jsonorder.Order) []string {
	if o == nil {
		return nil
	}
	return o.Keys
}

// expectedOrders lists the expected key order of every map under test; nil
// means the map must not exist (or at least records no keys).
type expectedOrders struct {
	topVariables     []string // config-level "variables"
	nestedVariables  []string // "variables" of the first action/sequence step
	triggerVariables []string // "variables" of the first trigger (automations only)
	fields           []string // script "fields" (scripts only)
}

// automationOrders extracts the orders under test from a stored automation config.
func automationOrders(o *jsonorder.Order) expectedOrders {
	return expectedOrders{
		topVariables:     keysOf(o.Field("variables")),
		nestedVariables:  keysOf(o.Field("actions").Item(0).Field("variables")),
		triggerVariables: keysOf(o.Field("triggers").Item(0).Field("variables")),
	}
}

// scriptOrders extracts the orders under test from a stored script config.
func scriptOrders(o *jsonorder.Order) expectedOrders {
	return expectedOrders{
		topVariables:    keysOf(o.Field("variables")),
		nestedVariables: keysOf(o.Field("sequence").Item(0).Field("variables")),
		fields:          keysOf(o.Field("fields")),
	}
}

func (s *KeyOrderTestSuite) readAutomationOrders(configID string) (expectedOrders, map[string]any, string) {
	auto, err := s.Client().GetAutomation(s.Context(), configID)
	s.Require().NoError(err, "GetAutomation(%s)", configID)
	s.Require().NotNil(auto.Config, "stored automation config must be returned")
	s.Require().NotNil(auto.Config.KeyOrder, "stored automation config must carry its key order")
	return automationOrders(auto.Config.KeyOrder), auto.Config.Variables, auto.Config.Description
}

func (s *KeyOrderTestSuite) readScriptOrders(entityID string) (expectedOrders, map[string]any, string) {
	scr, err := s.Client().GetScript(s.Context(), entityID)
	s.Require().NoError(err, "GetScript(%s)", entityID)
	s.Require().NotNil(scr.Config, "stored script config must be returned")
	s.Require().NotNil(scr.Config.KeyOrder, "stored script config must carry its key order")
	return scriptOrders(scr.Config.KeyOrder), scr.Config.Variables, scr.Config.Description
}

// TestAutomationKeyOrder walks the #311 scenario: variables written in an
// order that is NOT alphabetical must stay in that order across create, patch
// (including a patch of an unrelated field), update and a caller-reordered update.
func (s *KeyOrderTestSuite) TestAutomationKeyOrder() {
	automationID := GenerateTestID("key_order_auto")
	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
	})

	// 1. Create: trigger variables and an action "variables" step in caller order.
	create := fmt.Sprintf(`{
		"action": "create",
		"automation_id": %q,
		"alias": %q,
		"description": "original description",
		"mode": "single",
		"trigger": [{"platform": "event", "event_type": "mcptest_key_order_trigger",
			"variables": {"z_t": 1, "a_t": "{{ z_t }}"}}],
		"automation_action": [
			{"variables": {"zeta": 1, "alpha": "{{ zeta + 1 }}"}},
			{"event": %q, "event_data": {"value": "{{ alpha }}"}}
		]
	}`, automationID, automationID, keyOrderEvent)
	result := s.CallToolJSON("manage_automation", create)
	s.Require().False(result.IsError, "create should succeed, got: %s", resultText(result))

	_, err := s.WaitForAutomation(automationID, 30*time.Second)
	s.Require().NoError(err, "Automation did not appear")

	got, _, _ := s.readAutomationOrders(automationID)
	s.Equal(expectedOrders{
		nestedVariables:  []string{"zeta", "alpha"},
		triggerVariables: []string{"z_t", "a_t"},
	}, got, "create must keep the caller's variables order")

	// 2. Patch: add top-level variables in a non-alphabetical order.
	patchAdd := fmt.Sprintf(`{
		"action": "patch",
		"automation_id": %q,
		"operations": [{"op": "add", "path": "/variables",
			"value": {"zeta_top": 1, "alpha_top": "{{ zeta_top }}"}}]
	}`, automationID)
	result = s.CallToolJSON("manage_automation", patchAdd)
	s.Require().False(result.IsError, "patch add /variables should succeed, got: %s", resultText(result))

	want := expectedOrders{
		topVariables:     []string{"zeta_top", "alpha_top"},
		nestedVariables:  []string{"zeta", "alpha"},
		triggerVariables: []string{"z_t", "a_t"},
	}
	got, topVars, _ := s.readAutomationOrders(automationID)
	s.Equal(want, got, "patch add /variables must keep all variables orders")
	s.Equal("{{ zeta_top }}", topVars["alpha_top"], "added variable value must be stored")

	// 3. The #311 scenario: patching an UNRELATED field must not reorder any variables.
	patchDesc := fmt.Sprintf(`{
		"action": "patch",
		"automation_id": %q,
		"operations": [{"op": "replace", "path": "/description", "value": "patched description"}]
	}`, automationID)
	result = s.CallToolJSON("manage_automation", patchDesc)
	s.Require().False(result.IsError, "patch replace /description should succeed, got: %s", resultText(result))

	got, _, desc := s.readAutomationOrders(automationID)
	s.Equal("patched description", desc)
	s.Equal(want, got, "patching /description must not reorder any variables map (issue #311)")

	// 4. Update of the description only: same guarantee through the update path.
	updateDesc := fmt.Sprintf(`{
		"action": "update",
		"automation_id": %q,
		"description": "updated description"
	}`, automationID)
	result = s.CallToolJSON("manage_automation", updateDesc)
	s.Require().False(result.IsError, "update description should succeed, got: %s", resultText(result))

	got, _, desc = s.readAutomationOrders(automationID)
	s.Equal("updated description", desc)
	s.Equal(want, got, "updating only the description must not reorder any variables map")

	// 5. Update with a reordered automation_action: the caller's order wins for
	// the replaced actions; the untouched trigger and top-level variables keep theirs.
	updateActions := fmt.Sprintf(`{
		"action": "update",
		"automation_id": %q,
		"automation_action": [
			{"variables": {"beta_b": 2, "zeta": 1, "alpha": "{{ zeta + beta_b }}"}},
			{"event": %q, "event_data": {"value": "{{ alpha }}"}}
		]
	}`, automationID, keyOrderEvent)
	result = s.CallToolJSON("manage_automation", updateActions)
	s.Require().False(result.IsError, "update automation_action should succeed, got: %s", resultText(result))

	got, _, _ = s.readAutomationOrders(automationID)
	s.Equal(expectedOrders{
		topVariables:     []string{"zeta_top", "alpha_top"},
		nestedVariables:  []string{"beta_b", "zeta", "alpha"},
		triggerVariables: []string{"z_t", "a_t"},
	}, got, "update must take the caller's order for a replaced automation_action")
}

// TestScriptKeyOrder is the script counterpart: "fields", a "variables"
// sequence step and the top-level "variables" must keep their order.
func (s *KeyOrderTestSuite) TestScriptKeyOrder() {
	scriptID := GenerateTestID("key_order_script")
	scriptEntityID := BuildEntityID("script", scriptID)
	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
	})

	// 1. Create: fields and a "variables" step in caller order.
	create := fmt.Sprintf(`{
		"action": "create",
		"script_id": %q,
		"alias": %q,
		"description": "original description",
		"mode": "single",
		"fields": {
			"zeta_f": {"name": "Zeta", "selector": {"text": {}}},
			"alpha_f": {"name": "Alpha", "selector": {"text": {}}}
		},
		"sequence": [
			{"variables": {"zeta": 1, "alpha": "{{ zeta + 1 }}"}},
			{"event": %q, "event_data": {"value": "{{ alpha }}"}}
		]
	}`, scriptID, scriptID, keyOrderEvent)
	result := s.CallToolJSON("manage_script", create)
	s.Require().False(result.IsError, "create should succeed, got: %s", resultText(result))

	_, err := s.WaitForEntity(scriptEntityID, 30*time.Second)
	s.Require().NoError(err, "Script did not appear")

	got, _, _ := s.readScriptOrders(scriptEntityID)
	s.Equal(expectedOrders{
		nestedVariables: []string{"zeta", "alpha"},
		fields:          []string{"zeta_f", "alpha_f"},
	}, got, "create must keep the caller's fields and variables order")

	// 2. Patch: add top-level variables in a non-alphabetical order.
	patchAdd := fmt.Sprintf(`{
		"action": "patch",
		"script_id": %q,
		"operations": [{"op": "add", "path": "/variables",
			"value": {"zeta_top": 1, "alpha_top": "{{ zeta_top }}"}}]
	}`, scriptID)
	result = s.CallToolJSON("manage_script", patchAdd)
	s.Require().False(result.IsError, "patch add /variables should succeed, got: %s", resultText(result))

	want := expectedOrders{
		topVariables:    []string{"zeta_top", "alpha_top"},
		nestedVariables: []string{"zeta", "alpha"},
		fields:          []string{"zeta_f", "alpha_f"},
	}
	got, topVars, _ := s.readScriptOrders(scriptEntityID)
	s.Equal(want, got, "patch add /variables must keep all orders")
	s.Equal("{{ zeta_top }}", topVars["alpha_top"], "added variable value must be stored")

	// 3. Patching an unrelated field must not reorder anything (issue #311).
	patchDesc := fmt.Sprintf(`{
		"action": "patch",
		"script_id": %q,
		"operations": [{"op": "replace", "path": "/description", "value": "patched description"}]
	}`, scriptID)
	result = s.CallToolJSON("manage_script", patchDesc)
	s.Require().False(result.IsError, "patch replace /description should succeed, got: %s", resultText(result))

	got, _, desc := s.readScriptOrders(scriptEntityID)
	s.Equal("patched description", desc)
	s.Equal(want, got, "patching /description must not reorder any map (issue #311)")

	// 4. Update of the description only.
	updateDesc := fmt.Sprintf(`{
		"action": "update",
		"script_id": %q,
		"description": "updated description"
	}`, scriptID)
	result = s.CallToolJSON("manage_script", updateDesc)
	s.Require().False(result.IsError, "update description should succeed, got: %s", resultText(result))

	got, _, desc = s.readScriptOrders(scriptEntityID)
	s.Equal("updated description", desc)
	s.Equal(want, got, "updating only the description must not reorder any map")

	// 5. Update with reordered sequence and fields: the caller's order wins for
	// both replaced subtrees; the untouched top-level variables keep theirs.
	updateBoth := fmt.Sprintf(`{
		"action": "update",
		"script_id": %q,
		"fields": {
			"mid_f": {"name": "Mid", "selector": {"text": {}}},
			"zeta_f": {"name": "Zeta", "selector": {"text": {}}},
			"alpha_f": {"name": "Alpha", "selector": {"text": {}}}
		},
		"sequence": [
			{"variables": {"beta_b": 2, "zeta": 1, "alpha": "{{ zeta + beta_b }}"}},
			{"event": %q, "event_data": {"value": "{{ alpha }}"}}
		]
	}`, scriptID, keyOrderEvent)
	result = s.CallToolJSON("manage_script", updateBoth)
	s.Require().False(result.IsError, "update sequence/fields should succeed, got: %s", resultText(result))

	got, _, _ = s.readScriptOrders(scriptEntityID)
	s.Equal(expectedOrders{
		topVariables:    []string{"zeta_top", "alpha_top"},
		nestedVariables: []string{"beta_b", "zeta", "alpha"},
		fields:          []string{"mid_f", "zeta_f", "alpha_f"},
	}, got, "update must take the caller's order for replaced sequence and fields")
}
