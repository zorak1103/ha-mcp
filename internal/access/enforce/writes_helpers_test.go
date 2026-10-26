package enforce

import (
	"errors"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// buildHelperFake is buildConfigFake plus config-entry helpers:
//
//	binary_sensor.thr_a           threshold, entry ce_thr
//	sensor.um_a + select.um_a     utility_meter, entry ce_um
//	sensor.um_vis + sensor.secret_um (hidden)  utility_meter, entry ce_um2
//	light.hue1                    hue (not a helper), entry ce_hue
//	button.tpl                    template, entry ce_tpl, stored press action
//	device dev_s: light.devs_visible and the hidden light.secret_devent
func buildHelperFake() *readFake {
	f := buildConfigFake()
	add := func(id, platform, entry, device string) {
		f.states = append(f.states, homeassistant.Entity{EntityID: id, State: "on"})
		f.registry = append(f.registry, homeassistant.EntityRegistryEntry{
			EntityID: id, Platform: platform, UniqueID: "u_" + id, ConfigEntryID: entry, DeviceID: device,
		})
	}
	add("binary_sensor.thr_a", "threshold", "ce_thr", "")
	add("sensor.um_a", "utility_meter", "ce_um", "")
	add("select.um_a", "utility_meter", "ce_um", "")
	add("sensor.um_vis", "utility_meter", "ce_um2", "")
	add("sensor.secret_um", "utility_meter", "ce_um2", "")
	add("light.hue1", "hue", "ce_hue", "")
	add("button.tpl", "template", "ce_tpl", "")
	add("light.devs_visible", "demo", "", "dev_s")
	add("light.secret_devent", "demo", "", "dev_s")
	f.devices = append(f.devices, homeassistant.DeviceRegistryEntry{ID: "dev_s"})
	f.more.entryOptions = map[string]map[string]any{
		"ce_tpl": {
			"name": "Tpl",
			"press": []any{
				map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": "light.kitchen"}},
				map[string]any{"action": "switch.turn_on", "target": map[string]any{"entity_id": "switch.ro"}},
			},
		},
	}
	for _, e := range []string{"ce_thr", "ce_um", "ce_um2", "ce_hue", "ce_tpl"} {
		f.more.configEntries = append(f.more.configEntries, homeassistant.ConfigEntryFull{EntryID: e, Domain: "x"})
	}
	return f
}

// helperPolicy: default read; write on input_boolean, input_number,
// binary_sensor, sensor, button and select.um_a; control on lights and
// switch.office; every *.secret* hidden; plus extra before the hiding rule.
func helperPolicy(t *testing.T, extra ...config.AccessRule) *access.Policy {
	t.Helper()
	rules := []config.AccessRule{
		{Match: ents("input_boolean.*", "input_number.*", "binary_sensor.*", "sensor.*", "button.*", "select.um_a"), Allow: caps("write")},
		{Match: ents("light.*", "switch.office"), Allow: caps("control")},
	}
	rules = append(rules, extra...)
	rules = append(rules, config.AccessRule{Match: ents("*.secret*"), Allow: caps("none")})
	return mustCompile(t, config.AccessConfig{Default: "read", Rules: rules})
}

func helper(platform string, conf map[string]any) homeassistant.HelperConfig {
	return homeassistant.HelperConfig{Platform: platform, Config: conf}
}

func threshold(source any) homeassistant.HelperConfig {
	return helper("threshold", map[string]any{"name": "Kitchen Thr", "entity_id": source, "upper": 5.0})
}

func TestHelperCreate_SourceEntitiesNeedControl(t *testing.T) {
	t.Run("controlled source passes, inner once with the original config", func(t *testing.T) {
		f := buildHelperFake()
		c, tr, ctx := wrapFake(t, helperPolicy(t), f)
		cfg := threshold("light.kitchen")
		if _, err := c.CreateHelperEntity(ctx, cfg); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "CreateHelperEntity", "", "", map[string]any{"helper": cfg})
		noDenial(t, tr)
	})
	t.Run("read-only source is named", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		errText(t, c.CreateHelper(ctx, threshold("switch.ro")), denialFor("switch.ro", "control"))
		noInnerWrite(t, f)
	})
	t.Run("hidden and nonexistent sources answer alike", func(t *testing.T) {
		for _, id := range []string{"light.secret", "light.nope"} {
			f := buildHelperFake()
			c, _, ctx := wrapFake(t, helperPolicy(t), f)
			err := c.CreateHelper(ctx, threshold(id))
			errText(t, err, homeassistant.NewEntityNotFoundError(id).Error())
			noInnerWrite(t, f)
		}
	})
	t.Run("every list member and every source key is checked", func(t *testing.T) {
		cases := map[string]homeassistant.HelperConfig{
			"group entities": helper("group", map[string]any{"name": "Grp", "entities": []any{"light.kitchen", "switch.ro"}}),
			"min_max ids":    helper("min_max", map[string]any{"name": "Mm", "entity_ids": []string{"switch.ro"}}),
			"utility source": helper("utility_meter", map[string]any{"name": "Um", "source": "switch.ro"}),
			"thermostat":     helper("generic_thermostat", map[string]any{"name": "Th", "heater": "light.kitchen", "target_sensor": "switch.ro"}),
			"hygrostat":      helper("generic_hygrostat", map[string]any{"name": "Hy", "humidifier_entity_id": "switch.ro"}),
		}
		for name, cfg := range cases {
			t.Run(name, func(t *testing.T) {
				f := buildHelperFake()
				c, _, ctx := wrapFake(t, helperPolicy(t, config.AccessRule{Match: ents("climate.*", "humidifier.*", "light.grp"), Allow: caps("write")}), f)
				errText(t, c.CreateHelper(ctx, cfg), denialFor("switch.ro", "control"))
				noInnerWrite(t, f)
			})
		}
	})
	t.Run("unknown entity key or shape is refused", func(t *testing.T) {
		cases := map[string]homeassistant.HelperConfig{
			"unknown entity key": helper("threshold", map[string]any{"name": "T", "entity_id": "light.kitchen", "other_entity_id": "light.hall"}),
			"number source":      threshold(5),
			"map source":         threshold(map[string]any{"x": "light.kitchen"}),
		}
		for name, cfg := range cases {
			t.Run(name, func(t *testing.T) {
				f := buildHelperFake()
				c, _, ctx := wrapFake(t, helperPolicy(t), f)
				errText(t, c.CreateHelper(ctx, cfg), genericDenial())
				noInnerWrite(t, f)
			})
		}
	})
}

func TestSwitchAsX_RequiresDomainWideAndSourceControl(t *testing.T) {
	asX := func(target, source string) homeassistant.HelperConfig {
		conf := map[string]any{"entity_id": source}
		if target != "" {
			conf["target_domain"] = target
		}
		return helper("switch_as_x", conf)
	}
	lightWide := func(t *testing.T, extra ...config.AccessRule) *access.Policy {
		t.Helper()
		rules := append([]config.AccessRule{
			{Match: ents("light.*"), Allow: caps("write")},
			{Match: ents("switch.office"), Allow: caps("control")},
		}, extra...)
		return mustCompile(t, config.AccessConfig{Default: "read", Rules: rules})
	}
	t.Run("domain-wide configure and source control pass", func(t *testing.T) {
		f := buildHelperFake()
		c, tr, ctx := wrapFake(t, lightWide(t), f)
		cfg := asX("light", "switch.office")
		if err := c.CreateHelper(ctx, cfg); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "CreateHelper", "", "", map[string]any{"helper": cfg})
		noDenial(t, tr)
	})
	t.Run("a rule restricting part of the domain refuses", func(t *testing.T) {
		f := buildHelperFake()
		p := lightWide(t, config.AccessRule{Match: ents("light.vault_*"), Deny: caps("configure")})
		c, _, ctx := wrapFake(t, p, f)
		errText(t, c.CreateHelper(ctx, asX("light", "switch.office")), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("target domain without domain-wide configure refuses", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, lightWide(t), f)
		errText(t, c.CreateHelper(ctx, asX("cover", "switch.office")), genericDenial())
		errText(t, c.CreateHelper(ctx, asX("", "switch.office")), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("source without control is named", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, lightWide(t), f)
		errText(t, c.CreateHelper(ctx, asX("light", "switch.ro")), denialFor("switch.ro", "control"))
		noInnerWrite(t, f)
	})
}

func TestHelperCreate_GrantFirstThenCollision(t *testing.T) {
	ib := func(name string) homeassistant.HelperConfig {
		return helper("input_boolean", map[string]any{"name": name})
	}
	t.Run("caller with rights gets the neutral collision answer", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		errText(t, c.CreateHelper(ctx, ib("A")), errUnavailable("input_boolean.a").Error())
		noInnerWrite(t, f)
	})
	t.Run("caller without rights cannot tell taken from free", func(t *testing.T) {
		ro := mustCompile(t, config.AccessConfig{Default: "read"})
		for name, want := range map[string]string{"A": "input_boolean.a", "Zz": "input_boolean.zz"} {
			f := buildHelperFake()
			c, _, ctx := wrapFake(t, ro, f)
			errText(t, c.CreateHelper(ctx, ib(name)), denialFor(want, "configure"))
			noInnerWrite(t, f)
		}
	})
	t.Run("create false is a create denial", func(t *testing.T) {
		f := buildHelperFake()
		p := helperPolicy(t, config.AccessRule{Match: ents("input_boolean.*"), Allow: caps("configure"), Create: boolPtr(false)})
		c, _, ctx := wrapFake(t, p, f)
		errText(t, c.CreateHelper(ctx, ib("Zz")), denialFor("input_boolean.zz", "create"))
	})
	t.Run("refused generically", func(t *testing.T) {
		cases := map[string]homeassistant.HelperConfig{
			"hidden predicted id":      ib("Secret X"),
			"unmappable name":          ib("Café"),
			"no name":                  helper("input_boolean", map[string]any{}),
			"unknown platform":         helper("config/area_registry", map[string]any{"name": "x"}),
			"group_type overrides":     helper("template", map[string]any{"name": "Tz", "template_type": "sensor", "group_type": "lock", "state": "1"}),
			"storage id of another":    ib("ok"),
			"utility meter tariffs":    helper("utility_meter", map[string]any{"name": "Um2", "source": "light.kitchen", "tariffs": []any{"peak"}}),
			"device of hidden entity":  helper("threshold", map[string]any{"name": "Dv", "entity_id": "light.kitchen", "device_id": "dev_s"}),
			"unknown device":           helper("threshold", map[string]any{"name": "Dv", "entity_id": "light.kitchen", "device_id": "dev_nope"}),
			"group_type on non-menu":   helper("threshold", map[string]any{"name": "Gt", "entity_id": "light.kitchen", "group_type": "binary_sensor"}),
			"non-string group_type":    helper("group", map[string]any{"name": "Gg", "entities": []any{"light.kitchen"}, "group_type": 1}),
			"template name not string": helper("template", map[string]any{"name": 3, "template_type": "button"}),
		}
		for name, cfg := range cases {
			t.Run(name, func(t *testing.T) {
				f := buildHelperFake()
				if name == "storage id of another" {
					// input_number.ok is stored as "ok"; an input_number named
					// "ok" would get input_number.ok_2.
					cfg.Platform = "input_number"
					f.registry = append(f.registry, homeassistant.EntityRegistryEntry{EntityID: "input_number.renamed", Platform: "input_number", UniqueID: "zed"})
					cfg.Config["name"] = "zed"
				}
				c, _, ctx := wrapFake(t, helperPolicy(t), f)
				err := c.CreateHelper(ctx, cfg)
				if name == "storage id of another" {
					errText(t, err, errUnavailable("input_number.zed").Error())
				} else {
					errText(t, err, genericDenial())
				}
				noInnerWrite(t, f)
			})
		}
	})
	t.Run("matching group_type and device with configurable entities pass", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t, config.AccessRule{Match: ents("light.*"), Allow: caps("configure")}), f)
		cfg := helper("group", map[string]any{"name": "Lights", "entities": []any{"light.kitchen"}, "group_type": "light"})
		if err := c.CreateHelper(ctx, cfg); err != nil {
			t.Fatal(err)
		}
		cfg2 := helper("threshold", map[string]any{"name": "Dv", "entity_id": "light.kitchen", "device_id": "dev_k"})
		if err := c.CreateHelper(ctx, cfg2); err != nil {
			t.Fatal(err)
		}
		if got := len(f.writeCalls()); got != 2 {
			t.Fatalf("inner writes = %d, want 2", got)
		}
	})
}

func pressOn(entity string) []any {
	return []any{map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": entity}}}
}

func TestTemplateHelper_EmbeddedActions(t *testing.T) {
	tpl := func(extra map[string]any) homeassistant.HelperConfig {
		conf := map[string]any{"name": "Door Btn", "template_type": "button"}
		for k, v := range extra {
			conf[k] = v
		}
		return helper("template", conf)
	}
	switchRO := []any{map[string]any{"action": "switch.turn_on", "target": map[string]any{"entity_id": "switch.ro"}}}
	t.Run("allowed action passes", func(t *testing.T) {
		f := buildHelperFake()
		c, tr, ctx := wrapFake(t, readPolicyWith(t), f)
		cfg := tpl(map[string]any{"press": pressOn("light.kitchen")})
		if err := c.CreateHelper(ctx, cfg); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "CreateHelper", "", "", map[string]any{"helper": cfg})
		noDenial(t, tr)
	})
	t.Run("blocked or undecidable actions refuse", func(t *testing.T) {
		cases := map[string]struct {
			cfg  homeassistant.HelperConfig
			want string
		}{
			"read-only target":       {tpl(map[string]any{"press": switchRO}), denialFor("switch.ro", "control")},
			"unknown key with calls": {tpl(map[string]any{"frob": switchRO}), denialFor("switch.ro", "control")},
			"templated service":      {tpl(map[string]any{"press": []any{map[string]any{"action": "{{ x }}"}}}), genericDenial()},
			"string action":          {tpl(map[string]any{"press": "light.turn_on"}), genericDenial()},
			"unknown node":           {tpl(map[string]any{"turn_on": []any{map[string]any{"bogus": 1}}}), genericDenial()},
			"bad fan_speeds":         {tpl(map[string]any{"fan_speeds": []any{map[string]any{"a": 1}}}), genericDenial()},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				f := buildHelperFake()
				c, _, ctx := wrapFake(t, readPolicyWith(t), f)
				errText(t, c.CreateHelper(ctx, tc.cfg), tc.want)
				noInnerWrite(t, f)
			})
		}
	})
	t.Run("fan_speeds list of strings is not an action", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, readPolicyWith(t, config.AccessRule{Match: ents("fan.*"), Allow: caps("write")}), f)
		cfg := helper("template", map[string]any{"name": "Fan", "template_type": "fan", "fan_speeds": []string{"low", "high"}, "turn_on": pressOn("light.kitchen")})
		if err := c.CreateHelper(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("templates refused under a hiding policy", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		cfg := helper("template", map[string]any{"name": "Leak", "template_type": "sensor", "state": "{{ states('light.secret') }}"})
		errText(t, c.CreateHelper(ctx, cfg), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("update checks only new calls", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, readPolicyWith(t), f)
		stored := f.more.entryOptions["ce_tpl"]["press"].([]any)
		withNew := append(append([]any(nil), stored...), pressOn("light.hall")...)
		upd := helper("template", map[string]any{"press": withNew})
		if err := c.UpdateHelper(ctx, "button.tpl", upd); err != nil {
			t.Fatalf("legacy switch.ro call must not be rechecked: %v", err)
		}
		callOnce(t, f, "UpdateHelper", "", "", map[string]any{"id": "button.tpl", "helper": upd})

		f2 := buildHelperFake()
		c2, _, ctx2 := wrapFake(t, readPolicyWith(t), f2)
		changed := []any{stored[0], map[string]any{"action": "switch.turn_off", "target": map[string]any{"entity_id": "switch.ro"}}}
		errText(t, c2.UpdateHelper(ctx2, "button.tpl", helper("template", map[string]any{"press": changed})), denialFor("switch.ro", "control"))
		noInnerWrite(t, f2)
	})
	t.Run("update with unreadable stored options checks every call", func(t *testing.T) {
		f := buildHelperFake()
		delete(f.more.entryOptions, "ce_tpl")
		c, _, ctx := wrapFake(t, readPolicyWith(t), f)
		errText(t, c.UpdateHelper(ctx, "button.tpl", helper("template", map[string]any{"press": switchRO})), denialFor("switch.ro", "control"))
	})
}

// readPolicyWith is helperPolicy without hiding rules (templates allowed):
// default read; write on buttons, sensors and binary sensors; control on
// lights and switch.office.
func readPolicyWith(t *testing.T, extra ...config.AccessRule) *access.Policy {
	t.Helper()
	rules := append([]config.AccessRule{
		{Match: ents("button.*", "sensor.*", "binary_sensor.*"), Allow: caps("write")},
		{Match: ents("light.*", "switch.office"), Allow: caps("control")},
	}, extra...)
	return mustCompile(t, config.AccessConfig{Default: "read", Rules: rules})
}

func TestHelperUpdate_HiddenAnswersAsMissing(t *testing.T) {
	upd := helper("input_number", map[string]any{"name": "N", "min": 0.0, "max": 1.0})
	for _, id := range []string{"secret_num", "input_number.secret_num", "nope", "input_number.nope"} {
		t.Run(id, func(t *testing.T) {
			f := buildHelperFake()
			c, tr, ctx := wrapFake(t, helperPolicy(t), f)
			full := id
			if !strings.Contains(id, ".") {
				full = "input_number." + id
			}
			err := c.UpdateHelper(ctx, id, upd)
			sameError(t, err, homeassistant.NewHelperConfigMissingError("input_number", full))
			if !errors.Is(err, homeassistant.ErrHelperNotFoundInStorage) {
				t.Error("missing-helper error must wrap ErrHelperNotFoundInStorage")
			}
			noInnerWrite(t, f)
			noDenial(t, tr)
		})
	}
	t.Run("delete of hidden and nonexistent alike", func(t *testing.T) {
		for _, id := range []string{"input_number.secret_num", "input_number.nope"} {
			f := buildHelperFake()
			c, _, ctx := wrapFake(t, helperPolicy(t), f)
			sameError(t, c.DeleteHelper(ctx, id), homeassistant.NewHelperConfigMissingError("input_number", id))
			noInnerWrite(t, f)
		}
	})
}

func TestHelperUpdate_Rules(t *testing.T) {
	t.Run("bare id is decided under the given platform", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		cfg := helper("input_number", map[string]any{"name": "Renamed"})
		if err := c.UpdateHelper(ctx, "ok", cfg); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "UpdateHelper", "", "", map[string]any{"id": "ok", "helper": cfg})
	})
	t.Run("platform mismatch or non-ws platform refused", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		errText(t, c.UpdateHelper(ctx, "input_number.ok", helper("input_boolean", map[string]any{})), genericDenial())
		errText(t, c.UpdateHelper(ctx, "sensor.unregistered", helper("threshold", map[string]any{})), genericDenial())
		errText(t, c.UpdateHelper(ctx, "binary_sensor.thr_a", helper("input_boolean", map[string]any{})), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("read-only helper is named", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		errText(t, c.UpdateHelper(ctx, "ok", helper("input_datetime", map[string]any{})), denialFor("input_datetime.ok", "configure"))
	})
	t.Run("create then update in one request", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		if err := c.CreateHelper(ctx, helper("input_boolean", map[string]any{"name": "fresh"})); err != nil {
			t.Fatal(err)
		}
		if err := c.UpdateHelper(ctx, "fresh", helper("input_boolean", map[string]any{"name": "Fresh One"})); err != nil {
			t.Fatalf("the helper this request created must be updatable: %v", err)
		}
	})
	t.Run("config-entry helper update needs configure on its whole entry", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		cfg := helper("threshold", map[string]any{"upper": 7.0})
		if err := c.UpdateHelper(ctx, "binary_sensor.thr_a", cfg); err != nil {
			t.Fatal(err)
		}
		errText(t, c.UpdateHelper(ctx, "sensor.um_vis", helper("utility_meter", map[string]any{})), genericDenial())
		errText(t, c.UpdateHelper(ctx, "light.hue1", helper("hue", map[string]any{})), genericDenial())
		errText(t, c.UpdateHelper(ctx, "binary_sensor.thr_a", helper("threshold", map[string]any{"entity_id": "switch.ro"})), denialFor("switch.ro", "control"))
		if got := len(f.writeCalls()); got != 1 {
			t.Fatalf("inner writes = %d, want 1", got)
		}
	})
}

func TestDeleteHelper_ConfigEntryCascade(t *testing.T) {
	t.Run("every entity of the entry needs configure and delete", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		if err := c.DeleteHelper(ctx, "sensor.um_a"); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "DeleteHelper", "", "", map[string]any{"id": "sensor.um_a"})
	})
	t.Run("a sibling without rights refuses generically", func(t *testing.T) {
		f := buildHelperFake()
		p := helperPolicy(t, config.AccessRule{Match: ents("select.um_a"), Allow: caps("configure"), Delete: boolPtr(false)})
		c, _, ctx := wrapFake(t, p, f)
		errText(t, c.DeleteHelper(ctx, "sensor.um_a"), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("a hidden sibling refuses generically", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		errText(t, c.DeleteHelper(ctx, "sensor.um_vis"), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("an integration entity is not a helper", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t, config.AccessRule{Match: ents("light.hue1"), Allow: caps("write")}), f)
		errText(t, c.DeleteHelper(ctx, "light.hue1"), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("delete false is a delete denial", func(t *testing.T) {
		f := buildHelperFake()
		p := helperPolicy(t, config.AccessRule{Match: ents("input_number.ok"), Allow: caps("configure"), Delete: boolPtr(false)})
		c, _, ctx := wrapFake(t, p, f)
		errText(t, c.DeleteHelper(ctx, "input_number.ok"), denialFor("input_number.ok", "delete"))
	})
	t.Run("ws helper delete passes", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, helperPolicy(t), f)
		if err := c.DeleteHelper(ctx, "input_number.ok"); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "DeleteHelper", "", "", map[string]any{"id": "input_number.ok"})
	})
}
