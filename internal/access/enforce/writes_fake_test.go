package enforce

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// writeCall is one write that reached the fake inner client, with the exact
// arguments it received.
type writeCall struct {
	method, domain, service string
	data                    map[string]any
}

func (f *readFake) recordWrite(method, domain, service string, data map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, writeCall{method: method, domain: domain, service: service, data: data})
}

func (f *readFake) writeCalls() []writeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.writes)
}

func (f *readFake) CallService(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	f.recordWrite("CallService", domain, service, data)
	return []homeassistant.Entity{}, nil
}

func (f *readFake) CallServiceWithResponse(_ context.Context, domain, service string, data map[string]any) (map[string]any, error) {
	f.recordWrite("CallServiceWithResponse", domain, service, data)
	return map[string]any{"response": "of " + domain + "." + service}, nil
}

func (f *readFake) SetState(_ context.Context, entityID string, st homeassistant.StateUpdate) (*homeassistant.Entity, error) {
	f.recordWrite("SetState", "", "", map[string]any{"entity_id": entityID, "state": st.State})
	return &homeassistant.Entity{EntityID: entityID, State: st.State}, nil
}

func (f *readFake) SetHelperValue(_ context.Context, entityID string, value any) error {
	f.recordWrite("SetHelperValue", "", "", map[string]any{"entity_id": entityID, "value": value})
	return nil
}

func (f *readFake) ToggleAutomation(_ context.Context, entityID string, enabled bool) error {
	f.recordWrite("ToggleAutomation", "", "", map[string]any{"entity_id": entityID, "enabled": enabled})
	return nil
}

func (f *readFake) ClearStatistics(_ context.Context, ids []string) error {
	f.recordWrite("ClearStatistics", "", "", map[string]any{"ids": slices.Clone(ids)})
	return nil
}

func (f *readFake) ClearSystemLog(context.Context) error {
	f.recordWrite("ClearSystemLog", "", "", nil)
	return nil
}

func (f *readFake) DeleteConfigEntry(_ context.Context, entryID string) (bool, error) {
	f.recordWrite("DeleteConfigEntry", "", "", map[string]any{"entry_id": entryID})
	return false, nil
}

func (f *readFake) SendHACSCommand(_ context.Context, command string, data map[string]any) (any, error) {
	f.recordWrite("SendHACSCommand", command, "", maps.Clone(data))
	if f.hacsFn != nil {
		return f.hacsFn(command, data)
	}
	return []any{}, nil
}

// traceHA answers trace and release-notes commands the way Home Assistant
// does: a known trace key yields its traces, an unknown one an empty list
// (trace/list) or a not_found error (trace/get); an unknown update entity a
// not_found error.
func (f *readFake) traceHA(command string, data map[string]any) (any, error) {
	known := map[string]bool{}
	for _, r := range f.registry {
		if r.Platform == "automation" || r.Platform == "script" {
			known[r.Platform+"."+r.UniqueID] = true
		}
	}
	key := fmt.Sprintf("%v.%v", data["domain"], data["item_id"])
	switch command {
	case "trace/list":
		if _, ok := data["item_id"]; !ok {
			return []any{map[string]any{"item_id": "all"}}, nil
		}
		if known[key] {
			return []any{map[string]any{"item_id": data["item_id"], "run_id": "r1"}}, nil
		}
		return []any{}, nil
	case "trace/get":
		if known[key] {
			return map[string]any{"item_id": data["item_id"]}, nil
		}
		return nil, fmt.Errorf("hacs command failed: %w", errors.New("command failed: not_found - The trace could not be found"))
	case "update/release_notes":
		if id, _ := data["entity_id"].(string); f.hasState(id) {
			return "notes", nil
		}
		return nil, fmt.Errorf("hacs command failed: %w", errors.New("command failed: not_found - Entity not found"))
	}
	return []any{}, nil
}

// buildWriteFake is the home of the write tests. Without secrets it is the
// same home minus every hidden object.
//
//	floor ground: area hall (light.hall)
//	floor upstairs: area office (switch.office, read-only)
//	area kitchen: light.kitchen, light.secret (hidden), device dev_k
//	area labeled_area (label lbl_area): light.arealabel
//	device dev_l (label lbl_dev): light.devlight
//	label lbl_ro: switch.ro (read-only)
//	states only (no registry entry): input_boolean.yaml_only, cover.yaml_ro
func buildWriteFake(withSecret bool) *readFake {
	f := &readFake{}
	add := func(id, platform, unique string, mutate func(*homeassistant.EntityRegistryEntry)) {
		if !withSecret && strings.Contains(id, ".secret") {
			return
		}
		f.states = append(f.states, homeassistant.Entity{EntityID: id, State: "on"})
		e := homeassistant.EntityRegistryEntry{EntityID: id, Platform: platform, UniqueID: unique}
		if mutate != nil {
			mutate(&e)
		}
		f.registry = append(f.registry, e)
	}
	area := func(a string) func(*homeassistant.EntityRegistryEntry) {
		return func(e *homeassistant.EntityRegistryEntry) { e.AreaID = a }
	}
	add("light.kitchen", "demo", "u1", func(e *homeassistant.EntityRegistryEntry) { e.AreaID, e.DeviceID = "kitchen", "dev_k" })
	add("light.secret", "demo", "u2", area("kitchen"))
	add("light.hall", "demo", "u3", area("hall"))
	add("switch.office", "demo", "u4", area("office"))
	add("light.arealabel", "demo", "u5", area("labeled_area"))
	add("light.devlight", "demo", "u6", func(e *homeassistant.EntityRegistryEntry) { e.DeviceID = "dev_l" })
	add("switch.ro", "demo", "u7", func(e *homeassistant.EntityRegistryEntry) { e.Labels = []string{"lbl_ro"} })
	add("sensor.secret_temp", "demo", "u8", nil)
	add("input_boolean.a", "input_boolean", "a", nil)
	add("automation.heating", "automation", "cfg_heating", nil)
	add("automation.secret_auto", "automation", "cfg_secret", nil)
	add("script.ok", "script", "ok", nil)
	add("script.ro", "script", "ro", nil)
	add("script.secret_script", "script", "secret_script", nil)
	add("scene.ok", "homeassistant", "scene_ok_cfg", nil)
	add("group.existing", "group", "existing", nil)
	add("input_number.ok", "input_number", "ok", nil)
	add("input_number.secret_num", "input_number", "secret_num", nil)
	add("media_player.living", "demo", "m1", nil)
	add("media_player.ro", "demo", "m2", nil)
	add("media_player.secret_mp", "demo", "m3", nil)
	add("tts.engine", "demo", "t1", nil)
	add("camera.front", "demo", "c1", nil)
	add("todo.list", "demo", "td1", nil)
	add("todo.secret_list", "demo", "td2", nil)
	add("weather.home", "demo", "w1", nil)
	add("update.core", "demo", "up1", nil)
	add("update.secret_up", "demo", "up2", nil)
	// Entities without a registry entry exist in the state machine only.
	f.states = append(f.states,
		homeassistant.Entity{EntityID: "input_boolean.yaml_only", State: "off"},
		homeassistant.Entity{EntityID: "cover.yaml_ro", State: "open"},
	)
	f.devices = []homeassistant.DeviceRegistryEntry{
		{ID: "dev_k", AreaID: "kitchen"},
		{ID: "dev_l", Labels: []string{"lbl_dev"}},
	}
	f.areas = []homeassistant.AreaRegistryEntry{
		{AreaID: "kitchen", Name: "Kitchen", FloorID: "ground"},
		{AreaID: "hall", Name: "Hall", FloorID: "ground_hall"},
		{AreaID: "office", Name: "Office", FloorID: "upstairs"},
		{AreaID: "labeled_area", Name: "Labeled", Labels: []string{"lbl_area"}},
	}
	f.floors = []homeassistant.FloorRegistryEntry{{FloorID: "ground"}, {FloorID: "ground_hall"}, {FloorID: "upstairs"}}
	f.labels = []homeassistant.LabelRegistryEntry{{LabelID: "lbl_ro"}, {LabelID: "lbl_dev"}, {LabelID: "lbl_area"}}
	return f
}

// writePolicy: default read; every *.secret* entity hidden; control on
// lights, input booleans, input numbers, media players, tts, cameras,
// automations and script.ok; configure on scenes and groups.
func writePolicy(t *testing.T, system ...string) *access.Policy {
	t.Helper()
	cfg := config.AccessConfig{
		Default: "read",
		Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{
				"light.*", "input_boolean.*", "input_number.*", "media_player.*", "tts.*",
				"camera.*", "automation.*", "script.ok",
			}}, Allow: config.CapList{"control"}},
			{Match: config.AccessMatch{Entities: []string{"scene.*", "group.*"}}, Allow: config.CapList{"configure"}},
			{Match: config.AccessMatch{Entities: []string{"*.secret*"}}, Allow: config.CapList{"none"}},
		},
	}
	if system != nil {
		list := config.CapList(system)
		cfg.System = &list
	}
	return mustCompile(t, cfg)
}

// denialFor is the error text of a recorded object denial.
func denialFor(object, capability string) string {
	return access.Denial{Tool: "some_tool", Object: object, Capability: capability}.Message()
}

// genericDenial is the error text of a denial that names no object.
func genericDenial() string {
	return access.Denial{Tool: "some_tool"}.Message()
}

// systemDenial is the error text of a refused system operation.
func systemDenial(v access.SystemValue) string {
	return access.Denial{Tool: "some_tool", System: v}.Message()
}
