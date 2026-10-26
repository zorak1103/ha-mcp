package enforce

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// regPolicy: default read; write on lights and switch.office; *.secret* and
// the label lbl_hide hidden; plus extra before the hiding rules.
func regPolicy(t *testing.T, extra ...config.AccessRule) *access.Policy {
	t.Helper()
	rules := []config.AccessRule{
		{Match: ents("light.*", "switch.office"), Allow: caps("write")},
	}
	rules = append(rules, extra...)
	rules = append(rules,
		config.AccessRule{Match: ents("*.secret*"), Allow: caps("none")},
		config.AccessRule{Match: config.AccessMatch{Labels: []string{"lbl_hide"}}, Allow: caps("none")},
	)
	return mustCompile(t, config.AccessConfig{Default: "read", Rules: rules})
}

// writeDefault: default write with hiding of *.secret* (so not unrestricted)
// plus extra.
func writeDefault(t *testing.T, extra ...config.AccessRule) *access.Policy {
	t.Helper()
	rules := append(append([]config.AccessRule(nil), extra...), config.AccessRule{Match: ents("*.secret*"), Allow: caps("none")})
	return mustCompile(t, config.AccessConfig{Default: "write", Rules: rules})
}

func strPtr(s string) *string { return &s }

func rename(to string) homeassistant.EntityRegistryUpdateConfig {
	return homeassistant.EntityRegistryUpdateConfig{NewEntityID: strPtr(to)}
}

func TestRename_NewIDChecked(t *testing.T) {
	t.Run("rename to a free id with rights passes", func(t *testing.T) {
		f := buildHelperFake()
		c, tr, ctx := wrapFake(t, regPolicy(t), f)
		cfg := rename("light.renamed")
		if _, err := c.UpdateEntityRegistryEntry(ctx, "light.kitchen", cfg); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "UpdateEntityRegistryEntry", "", "", map[string]any{"id": "light.kitchen", "config": cfg})
		noDenial(t, tr)
	})
	cases := map[string]struct {
		extra    []config.AccessRule
		from, to string
		want     string
	}{
		"new id denied configure": {
			extra: []config.AccessRule{{Match: ents("light.locked_*"), Deny: caps("configure")}},
			from:  "light.kitchen", to: "light.locked_x", want: denialFor("light.locked_x", "configure"),
		},
		"new id without create": {
			extra: []config.AccessRule{{Match: ents("light.nocreate_*"), Allow: caps("configure"), Create: boolPtr(false)}},
			from:  "light.kitchen", to: "light.nocreate_x", want: denialFor("light.nocreate_x", "create"),
		},
		"old id without delete": {
			extra: []config.AccessRule{{Match: ents("light.kitchen"), Allow: caps("configure"), Delete: boolPtr(false)}},
			from:  "light.kitchen", to: "light.renamed", want: denialFor("light.kitchen", "delete"),
		},
		"rename out of a read-only rule into a permissive glob": {
			from: "switch.ro", to: "light.free", want: denialFor("switch.ro", "configure"),
		},
		"domain change":      {from: "light.kitchen", to: "switch.kitchen", want: genericDenial()},
		"non-plain new id":   {from: "light.kitchen", to: "Light.Kitchen2", want: genericDenial()},
		"hidden taken id":    {from: "light.kitchen", to: "light.secret", want: genericDenial()},
		"hidden free id":     {from: "light.kitchen", to: "light.secret_free", want: genericDenial()},
		"visible taken id":   {from: "light.kitchen", to: "light.hall", want: errUnavailable("light.hall").Error()},
		"label-hidden taken": {from: "light.kitchen", to: "light.lblhidden", want: errUnavailable("light.lblhidden").Error()},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := buildHelperFake()
			f.states = append(f.states, homeassistant.Entity{EntityID: "light.lblhidden"})
			f.registry = append(f.registry, homeassistant.EntityRegistryEntry{EntityID: "light.lblhidden", Labels: []string{"lbl_hide"}})
			c, _, ctx := wrapFake(t, regPolicy(t, tc.extra...), f)
			_, err := c.UpdateEntityRegistryEntry(ctx, tc.from, rename(tc.to))
			errText(t, err, tc.want)
			noInnerWrite(t, f)
		})
	}
}

func TestEntityRegistry_AttributeChangesNeedOnlyThatEntity(t *testing.T) {
	f := buildHelperFake()
	c, _, ctx := wrapFake(t, regPolicy(t), f)
	for _, cfg := range []homeassistant.EntityRegistryUpdateConfig{
		{AreaID: strPtr("office")},
		{Labels: []string{"lbl_ro"}},
		{Name: strPtr("Cooker"), DisabledBy: strPtr("")},
	} {
		if _, err := c.UpdateEntityRegistryEntry(ctx, "light.kitchen", cfg); err != nil {
			t.Fatalf("%+v: %v", cfg, err)
		}
	}
	if got := len(f.writeCalls()); got != 3 {
		t.Fatalf("inner writes = %d, want 3", got)
	}
	_, err := c.UpdateEntityRegistryEntry(ctx, "switch.ro", homeassistant.EntityRegistryUpdateConfig{Name: strPtr("x")})
	errText(t, err, denialFor("switch.ro", "configure"))
}

func TestEntityRegistry_HiddenAnswersAsMissing(t *testing.T) {
	for _, id := range []string{"light.secret", "light.nope", "light.lblhidden_x"} {
		f := buildHelperFake()
		c, tr, ctx := wrapFake(t, regPolicy(t), f)
		_, err := c.UpdateEntityRegistryEntry(ctx, id, homeassistant.EntityRegistryUpdateConfig{Name: strPtr("x")})
		sameError(t, err, errRegistryUpdateMissing())
		sameError(t, c.RemoveEntityRegistryEntry(ctx, id), errRegistryRemoveMissing())
		noInnerWrite(t, f)
		noDenial(t, tr)
	}
}

func TestRemoveEntityRegistryEntry_NeedsDelete(t *testing.T) {
	f := buildHelperFake()
	p := regPolicy(t, config.AccessRule{Match: ents("light.hall"), Allow: caps("configure"), Delete: boolPtr(false)})
	c, _, ctx := wrapFake(t, p, f)
	errText(t, c.RemoveEntityRegistryEntry(ctx, "light.hall"), denialFor("light.hall", "delete"))
	errText(t, c.RemoveEntityRegistryEntry(ctx, "switch.ro"), denialFor("switch.ro", "configure"))
	if err := c.RemoveEntityRegistryEntry(ctx, "light.kitchen"); err != nil {
		t.Fatal(err)
	}
	callOnce(t, f, "RemoveEntityRegistryEntry", "", "", map[string]any{"id": "light.kitchen"})
}

// TestScriptDeleteFallback_EndToEnd follows the script handler: the storage
// delete answers not found, the handler finds the entity in the registry and
// removes the entry. A visible script with rights goes through; a hidden one
// is answered as missing at every step.
func TestScriptDeleteFallback_EndToEnd(t *testing.T) {
	f := buildConfigFake()
	f.cfg.writeErr = homeassistant.NewConfigNotFoundError("script", "ok")
	c, _, ctx := wrapFake(t, cfgPolicy(t), f)
	if err := c.DeleteScript(ctx, "ok"); !errors.Is(err, f.cfg.writeErr) {
		t.Fatalf("storage delete error = %v", err)
	}
	reg, err := c.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range reg {
		found = found || e.EntityID == "script.ok"
	}
	if !found {
		t.Fatal("visible script missing from the registry the fallback searches")
	}
	if err := c.RemoveEntityRegistryEntry(ctx, "script.ok"); err != nil {
		t.Fatalf("fallback removal of a visible script with rights: %v", err)
	}
	if got := f.writesOf("RemoveEntityRegistryEntry"); len(got) != 1 || got[0].data["id"] != "script.ok" {
		t.Fatalf("registry removal = %+v", got)
	}
	for _, id := range []string{"script.secret_script", "script.nope"} {
		sameError(t, c.RemoveEntityRegistryEntry(ctx, id), errRegistryRemoveMissing())
	}
	if got := len(f.writesOf("RemoveEntityRegistryEntry")); got != 1 {
		t.Fatalf("hidden or unknown script reached inner: %d removals", got)
	}
}

func TestDeviceUpdate_RequiresConfigureOnEveryEntity(t *testing.T) {
	upd := homeassistant.DeviceRegistryUpdateConfig{AreaID: strPtr("hall")}
	t.Run("default read", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, regPolicy(t), f)
		if _, err := c.UpdateDeviceRegistryEntry(ctx, "dev_k", upd); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "UpdateDeviceRegistryEntry", "", "", map[string]any{"id": "dev_k", "config": upd})
		for _, dev := range []string{"dev_m", "dev_s", "dev_empty", "dev_nope"} {
			_, err := c.UpdateDeviceRegistryEntry(ctx, dev, upd)
			errText(t, err, genericDenial())
		}
		if got := len(f.writeCalls()); got != 1 {
			t.Fatalf("inner writes = %d, want 1", got)
		}
	})
	t.Run("default write governs a device without entities", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, writeDefault(t), f)
		if _, err := c.UpdateDeviceRegistryEntry(ctx, "dev_empty", upd); err != nil {
			t.Fatal(err)
		}
		_, err := c.UpdateDeviceRegistryEntry(ctx, "dev_s", upd)
		errText(t, err, genericDenial())
	})
	t.Run("removing a config entry needs delete on every entity", func(t *testing.T) {
		f := buildHelperFake()
		p := regPolicy(t, config.AccessRule{Match: ents("light.kitchen"), Allow: caps("configure"), Delete: boolPtr(false)})
		c, _, ctx := wrapFake(t, p, f)
		errText(t, c.RemoveDeviceConfigEntry(ctx, "dev_k", "ce"), genericDenial())
		if err := c.RemoveDeviceConfigEntry(ctx, "dev_l", "ce"); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "RemoveDeviceConfigEntry", "", "", map[string]any{"id": "dev_l", "entry": "ce"})
	})
}

func TestDeleteLabel_RequiresConfigureOnCarriers(t *testing.T) {
	roSwitch := config.AccessRule{Match: ents("switch.ro"), Deny: caps("configure")}
	t.Run("direct carrier without configure", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, writeDefault(t, roSwitch), f)
		errText(t, c.DeleteLabel(ctx, "lbl_ro"), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("carrier through its device", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, writeDefault(t, config.AccessRule{Match: ents("light.devlight"), Deny: caps("configure")}), f)
		errText(t, c.DeleteLabel(ctx, "lbl_dev"), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("hidden carrier", func(t *testing.T) {
		f := buildHelperFake()
		for i := range f.registry {
			if f.registry[i].EntityID == "light.secret" {
				f.registry[i].Labels = []string{"lbl_s"}
			}
		}
		c, _, ctx := wrapFake(t, writeDefault(t), f)
		errText(t, c.DeleteLabel(ctx, "lbl_s"), genericDenial())
		noInnerWrite(t, f)
	})
	t.Run("carriers with configure pass", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, writeDefault(t), f)
		for _, l := range []string{"lbl_ro", "lbl_dev", "lbl_area"} {
			if err := c.DeleteLabel(ctx, l); err != nil {
				t.Fatalf("%s: %v", l, err)
			}
		}
	})
	t.Run("the default decides the label itself", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, regPolicy(t), f)
		errText(t, c.DeleteLabel(ctx, "lbl_area"), denialFor("label lbl_area", "configure"))
		noInnerWrite(t, f)
	})
}

func TestRenameLabelOrArea_RequiresConfigureOnCarriers(t *testing.T) {
	roSwitch := config.AccessRule{Match: ents("switch.ro"), Deny: caps("configure")}
	f := buildHelperFake()
	c, _, ctx := wrapFake(t, writeDefault(t, roSwitch), f)
	_, err := c.UpdateLabel(ctx, "lbl_ro", homeassistant.LabelConfig{Name: "Renamed"})
	errText(t, err, genericDenial())
	if _, err := c.UpdateLabel(ctx, "lbl_ro", homeassistant.LabelConfig{Color: "red"}); err != nil {
		t.Fatalf("a label update without a rename: %v", err)
	}
	_, err = c.UpdateArea(ctx, "kitchen", homeassistant.AreaConfig{Name: "Cook"})
	errText(t, err, genericDenial())
	if _, err := c.UpdateArea(ctx, "kitchen", homeassistant.AreaConfig{Name: "KITCHEN"}); err != nil {
		t.Fatalf("a case-only rename does not change rule reach: %v", err)
	}
	if _, err := c.UpdateArea(ctx, "kitchen", homeassistant.AreaConfig{Icon: "mdi:x"}); err != nil {
		t.Fatal(err)
	}
	errText(t, c.DeleteArea(ctx, "kitchen"), genericDenial())
	if err := c.DeleteArea(ctx, "hall"); err != nil {
		t.Fatal(err)
	}
	if got := len(f.writeCalls()); got != 4 {
		t.Fatalf("inner writes = %d, want 4", got)
	}
}

// writeFn runs one write through the wrapper.
type writeFn func(ctx context.Context, c *client) error

func errOf(_ any, err error) error { return err }

// registryWriteFns has one area/label/floor/zone/person/tag write per entry,
// keyed by the object a denial names.
func registryWriteFns() map[string]writeFn {
	return map[string]writeFn{
		"area": func(ctx context.Context, c *client) error {
			return errOf(c.CreateArea(ctx, homeassistant.AreaConfig{Name: "Attic"}))
		},
		"area x": func(ctx context.Context, c *client) error {
			return errOf(c.UpdateArea(ctx, "x", homeassistant.AreaConfig{Icon: "i"}))
		},
		"area y": func(ctx context.Context, c *client) error { return c.DeleteArea(ctx, "y") },
		"label": func(ctx context.Context, c *client) error {
			return errOf(c.CreateLabel(ctx, homeassistant.LabelConfig{Name: "New"}))
		},
		"label x": func(ctx context.Context, c *client) error {
			return errOf(c.UpdateLabel(ctx, "x", homeassistant.LabelConfig{Color: "red"}))
		},
		"label y": func(ctx context.Context, c *client) error { return c.DeleteLabel(ctx, "y") },
		"floor": func(ctx context.Context, c *client) error {
			return errOf(c.CreateFloor(ctx, homeassistant.FloorConfig{Name: "Top"}))
		},
		"floor x": func(ctx context.Context, c *client) error {
			return errOf(c.UpdateFloor(ctx, "x", homeassistant.FloorConfig{Name: "T"}))
		},
		"floor y": func(ctx context.Context, c *client) error { return c.DeleteFloor(ctx, "y") },
		"zone": func(ctx context.Context, c *client) error {
			return errOf(c.CreateZone(ctx, homeassistant.ZoneConfig{Name: "Work"}))
		},
		"zone x": func(ctx context.Context, c *client) error {
			return errOf(c.UpdateZone(ctx, "x", homeassistant.ZoneConfig{Name: "W"}))
		},
		"zone y": func(ctx context.Context, c *client) error { return c.DeleteZone(ctx, "y") },
		"person": func(ctx context.Context, c *client) error {
			return errOf(c.CreatePerson(ctx, homeassistant.PersonConfig{Name: "Al"}))
		},
		"person x": func(ctx context.Context, c *client) error {
			return errOf(c.UpdatePerson(ctx, "x", homeassistant.PersonConfig{Name: "A"}))
		},
		"person y": func(ctx context.Context, c *client) error { return c.DeletePerson(ctx, "y") },
		"tag": func(ctx context.Context, c *client) error {
			return errOf(c.CreateTag(ctx, homeassistant.TagConfig{Name: "T"}))
		},
		"tag x": func(ctx context.Context, c *client) error {
			return errOf(c.UpdateTag(ctx, "x", homeassistant.TagConfig{Name: "T"}))
		},
		"tag y": func(ctx context.Context, c *client) error { return c.DeleteTag(ctx, "y") },
	}
}

// registryWrites runs every registryWriteFns entry once.
func registryWrites(ctx context.Context, c *client) map[string]error {
	out := map[string]error{}
	for name, fn := range registryWriteFns() {
		out[name] = fn(ctx, c)
	}
	return out
}

func TestRegistryObjects_GovernedByDefault(t *testing.T) {
	t.Run("default read refuses with a denial naming the object", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, regPolicy(t), f)
		for object, err := range registryWrites(ctx, c) {
			errText(t, err, denialFor(object, "configure"))
		}
		noInnerWrite(t, f)
	})
	t.Run("default write allows each write once", func(t *testing.T) {
		f := buildHelperFake()
		c, _, ctx := wrapFake(t, writeDefault(t), f)
		for object, err := range registryWrites(ctx, c) {
			if err != nil {
				t.Errorf("%s: %v", object, err)
			}
		}
		if got := len(f.writeCalls()); got != 18 {
			t.Fatalf("inner writes = %d, want 18", got)
		}
	})
	t.Run("default none hides them", func(t *testing.T) {
		f := buildHelperFake()
		f.more.zones = []homeassistant.ZoneRegistryEntry{{ID: "z"}}
		f.more.persons = []homeassistant.PersonRegistryEntry{{ID: "p"}}
		f.more.tags = []homeassistant.TagRegistryEntry{{TagID: "t"}}
		c, _, ctx := wrapFake(t, mustCompile(t, config.AccessConfig{Default: "none"}), f)
		for object, err := range registryWrites(ctx, c) {
			if err == nil || err.Error() != genericDenial() {
				t.Errorf("%s: %v, want the generic denial", object, err)
			}
		}
		noInnerWrite(t, f)
		labels, _ := c.GetLabelRegistry(ctx)
		floors, _ := c.GetFloorRegistry(ctx)
		zones, _ := c.GetZones(ctx)
		persons, _ := c.GetPersons(ctx)
		tags, _ := c.GetTags(ctx)
		if len(labels)+len(floors)+len(zones)+len(persons)+len(tags) != 0 {
			t.Errorf("global registries visible under default none: %v %v %v %v %v", labels, floors, zones, persons, tags)
		}
	})
	t.Run("default read lists them", func(t *testing.T) {
		f := buildHelperFake()
		f.more.zones = []homeassistant.ZoneRegistryEntry{{ID: "z"}}
		f.more.tags = []homeassistant.TagRegistryEntry{{TagID: "t"}}
		c, _, ctx := wrapFake(t, regPolicy(t), f)
		labels, _ := c.GetLabelRegistry(ctx)
		zones, _ := c.GetZones(ctx)
		tags, _ := c.GetTags(ctx)
		floors, _ := c.GetFloorRegistry(ctx)
		if len(labels) != len(f.labels) || len(zones) != 1 || len(tags) != 1 || len(floors) != len(f.floors) {
			t.Errorf("lists = %v %v %v %v", labels, zones, tags, floors)
		}
	})
}

func TestPerson_DeviceTrackersMustBeVisible(t *testing.T) {
	f := buildHelperFake()
	f.states = append(f.states,
		homeassistant.Entity{EntityID: "device_tracker.phone"},
		homeassistant.Entity{EntityID: "device_tracker.secret_t"},
	)
	f.more.persons = []homeassistant.PersonRegistryEntry{{ID: "p", DeviceTrackers: []string{"device_tracker.phone", "device_tracker.secret_t"}}}
	c, _, ctx := wrapFake(t, writeDefault(t), f)
	for _, tracker := range []string{"device_tracker.secret_t", "device_tracker.nope", "Device_Tracker.Phone"} {
		_, err := c.CreatePerson(ctx, homeassistant.PersonConfig{Name: "Al", DeviceTrackers: []string{tracker}})
		errText(t, err, genericDenial())
		_, err = c.UpdatePerson(ctx, "p", homeassistant.PersonConfig{DeviceTrackers: []string{tracker}})
		errText(t, err, genericDenial())
	}
	noInnerWrite(t, f)
	if _, err := c.CreatePerson(ctx, homeassistant.PersonConfig{Name: "Al", DeviceTrackers: []string{"device_tracker.phone"}}); err != nil {
		t.Fatal(err)
	}
	persons, err := c.GetPersons(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(persons) != 1 || len(persons[0].DeviceTrackers) != 1 || persons[0].DeviceTrackers[0] != "device_tracker.phone" {
		t.Errorf("persons = %+v, want the hidden tracker dropped", persons)
	}
	if len(f.more.persons[0].DeviceTrackers) != 2 {
		t.Error("the inner result was mutated")
	}
}
