//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// UnmodeledKeysTestSuite verifies issue #314 end to end: automation and script
// configs carry keys the tools do not model (trigger_variables, initial_state,
// max_exceeded, trace). A patch or update that only touches an unrelated field
// is a read-modify-write of the whole config, and HA's config write is a full
// replace, so every such key must survive it. Every assertion reads the stored
// config back from HA.
type UnmodeledKeysTestSuite struct {
	AutomationTestSuite
}

func TestUnmodeledKeysIntegration(t *testing.T) {
	suite.Run(t, new(UnmodeledKeysTestSuite))
}

// assertKeptKeys checks that cfg (an *AutomationConfig or *ScriptConfig) still
// carries every key of want, compared by value. The unmodeled keys live in an
// unexported field, so the config is compared through its JSON form - which is
// also exactly what a write sends back to HA.
func (s *UnmodeledKeysTestSuite) assertKeptKeys(cfg any, want map[string]any, stage string) {
	raw, err := json.Marshal(cfg)
	s.Require().NoError(err, "%s: marshaling the stored config", stage)

	var got map[string]any
	s.Require().NoError(json.Unmarshal(raw, &got), "%s: decoding the marshaled config", stage)

	for key, wantValue := range want {
		gotValue, present := got[key]
		s.True(present, "%s: key %q was dropped from the stored config", stage, key)
		s.Equal(wantValue, gotValue, "%s: value of key %q changed", stage, key)
	}
}

// decodeWant decodes a JSON object into the generic shape assertKeptKeys sees
// after its own marshal round trip, so numbers compare as float64 on both sides.
func (s *UnmodeledKeysTestSuite) decodeWant(raw string) map[string]any {
	var want map[string]any
	s.Require().NoError(json.Unmarshal([]byte(raw), &want), "invalid expected-keys fixture")
	return want
}

// TestAutomationUnmodeledKeysSurvive creates a storage automation carrying four
// keys AutomationConfig does not model, then runs a patch and an update that
// touch only the description.
func (s *UnmodeledKeysTestSuite) TestAutomationUnmodeledKeysSurvive() {
	automationID := GenerateTestID("unmodeled_auto")
	s.RegisterCleanup(func() {
		_ = s.Client().DeleteAutomation(s.Context(), automationID)
	})

	const keptKeys = `{
		"trigger_variables": {"zeta_tv": 1, "alpha_tv": "kept"},
		"initial_state": false,
		"max_exceeded": "silent",
		"trace": {"stored_traces": 20}
	}`
	want := s.decodeWant(keptKeys)

	// Decode from JSON so the unmodeled keys land in the config the way they do
	// when HA hands one to the server. Alias must equal ID so HA slugifies it to
	// the same entity ID (project convention).
	cfgJSON := fmt.Sprintf(`{
		"id": %q,
		"alias": %q,
		"description": "original description",
		"mode": "single",
		"triggers": [{"platform": "event", "event_type": "mcptest_unmodeled_trigger"}],
		"actions": [{"event": "mcptest_unmodeled_done"}],
		"trigger_variables": {"zeta_tv": 1, "alpha_tv": "kept"},
		"initial_state": false,
		"max_exceeded": "silent",
		"trace": {"stored_traces": 20}
	}`, automationID, automationID)
	var cfg homeassistant.AutomationConfig
	s.Require().NoError(json.Unmarshal([]byte(cfgJSON), &cfg), "decoding the fixture config")

	s.Require().NoError(s.Client().CreateAutomation(s.Context(), cfg), "failed to create automation")

	_, err := s.WaitForAutomation(automationID, 30*time.Second)
	s.Require().NoError(err, "automation did not appear after create")

	// HA schedules its own async reload after the config POST; a DELETE landing
	// inside it orphans the entity (#305). Reload synchronously, as the tools do.
	_, err = s.Client().CallService(s.Context(), "automation", "reload", map[string]any{"id": automationID})
	s.Require().NoError(err, "failed to reload automation after create")

	stored, err := s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err, "GetAutomation after create")
	s.Require().NotNil(stored.Config, "stored automation config must be returned")
	s.assertKeptKeys(stored.Config, want, "after create")

	// 1. Patch of an unrelated field.
	patchDesc := fmt.Sprintf(`{
		"action": "patch",
		"automation_id": %q,
		"operations": [{"op": "replace", "path": "/description", "value": "patched description"}]
	}`, automationID)
	result := s.CallToolJSON("manage_automation", patchDesc)
	s.Require().False(result.IsError, "patch replace /description should succeed, got: %s", resultText(result))

	stored, err = s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err, "GetAutomation after patch")
	s.Require().NotNil(stored.Config)
	s.Equal("patched description", stored.Config.Description)
	s.assertKeptKeys(stored.Config, want, "after patch")

	// 2. Update of the description only.
	updateDesc := fmt.Sprintf(`{
		"action": "update",
		"automation_id": %q,
		"description": "updated description"
	}`, automationID)
	result = s.CallToolJSON("manage_automation", updateDesc)
	s.Require().False(result.IsError, "update description should succeed, got: %s", resultText(result))

	stored, err = s.Client().GetAutomation(s.Context(), automationID)
	s.Require().NoError(err, "GetAutomation after update")
	s.Require().NotNil(stored.Config)
	s.Equal("updated description", stored.Config.Description)
	s.assertKeptKeys(stored.Config, want, "after update")
}

// TestScriptUnmodeledKeysSurvive is the script counterpart: max_exceeded and
// trace must survive a patch and an update of the description.
func (s *UnmodeledKeysTestSuite) TestScriptUnmodeledKeysSurvive() {
	scriptID := GenerateTestID("unmodeled_script")
	scriptEntityID := BuildEntityID("script", scriptID)
	s.RegisterCleanup(func() {
		_ = s.Client().DeleteScript(s.Context(), scriptID)
	})

	want := s.decodeWant(`{
		"max_exceeded": "silent",
		"trace": {"stored_traces": 20}
	}`)

	cfgJSON := fmt.Sprintf(`{
		"alias": %q,
		"description": "original description",
		"mode": "single",
		"sequence": [{"event": "mcptest_unmodeled_done"}],
		"max_exceeded": "silent",
		"trace": {"stored_traces": 20}
	}`, scriptID)
	var cfg homeassistant.ScriptConfig
	s.Require().NoError(json.Unmarshal([]byte(cfgJSON), &cfg), "decoding the fixture config")

	// CreateScript takes the bare ID; GetScript takes the full entity ID.
	s.Require().NoError(s.Client().CreateScript(s.Context(), scriptID, cfg), "failed to create script")

	_, err := s.WaitForEntity(scriptEntityID, 30*time.Second)
	s.Require().NoError(err, "script did not appear after create")

	// Same create-then-immediate-delete race as for automations (#305).
	_, err = s.Client().CallService(s.Context(), "script", "reload", nil)
	s.Require().NoError(err, "failed to reload scripts after create")

	stored, err := s.Client().GetScript(s.Context(), scriptEntityID)
	s.Require().NoError(err, "GetScript after create")
	s.Require().NotNil(stored.Config, "stored script config must be returned")
	s.assertKeptKeys(stored.Config, want, "after create")

	// 1. Patch of an unrelated field.
	patchDesc := fmt.Sprintf(`{
		"action": "patch",
		"script_id": %q,
		"operations": [{"op": "replace", "path": "/description", "value": "patched description"}]
	}`, scriptID)
	result := s.CallToolJSON("manage_script", patchDesc)
	s.Require().False(result.IsError, "patch replace /description should succeed, got: %s", resultText(result))

	stored, err = s.Client().GetScript(s.Context(), scriptEntityID)
	s.Require().NoError(err, "GetScript after patch")
	s.Require().NotNil(stored.Config)
	s.Equal("patched description", stored.Config.Description)
	s.assertKeptKeys(stored.Config, want, "after patch")

	// 2. Update of the description only.
	updateDesc := fmt.Sprintf(`{
		"action": "update",
		"script_id": %q,
		"description": "updated description"
	}`, scriptID)
	result = s.CallToolJSON("manage_script", updateDesc)
	s.Require().False(result.IsError, "update description should succeed, got: %s", resultText(result))

	stored, err = s.Client().GetScript(s.Context(), scriptEntityID)
	s.Require().NoError(err, "GetScript after update")
	s.Require().NotNil(stored.Config)
	s.Equal("updated description", stored.Config.Description)
	s.assertKeptKeys(stored.Config, want, "after update")
}
