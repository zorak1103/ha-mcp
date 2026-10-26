package enforce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// configStore holds the stored definitions the fake returns from GetAutomation
// (by entity id), GetScript (by key) and GetScene (by config id), and config
// file entries that have no registry entry ("domain/id").
type configStore struct {
	automations map[string]*homeassistant.AutomationConfig
	scripts     map[string]*homeassistant.ScriptConfig
	scenes      map[string]*homeassistant.SceneConfig
	files       map[string]bool
	writeErr    error // returned by every config write
}

func (f *readFake) CreateAutomation(_ context.Context, cfg homeassistant.AutomationConfig) error {
	f.recordWrite("CreateAutomation", domainAutomation, cfg.ID, map[string]any{"config": cfg})
	return f.cfg.writeErr
}

func (f *readFake) UpdateAutomation(_ context.Context, id string, cfg homeassistant.AutomationConfig) error {
	f.recordWrite("UpdateAutomation", domainAutomation, id, map[string]any{"config": cfg})
	return f.cfg.writeErr
}

func (f *readFake) DeleteAutomation(_ context.Context, id string) error {
	f.recordWrite("DeleteAutomation", domainAutomation, id, nil)
	return f.cfg.writeErr
}

func (f *readFake) CreateScript(_ context.Context, id string, cfg homeassistant.ScriptConfig) error {
	f.recordWrite("CreateScript", domainScript, id, map[string]any{"config": cfg})
	return f.cfg.writeErr
}

func (f *readFake) UpdateScript(_ context.Context, id string, cfg homeassistant.ScriptConfig) error {
	f.recordWrite("UpdateScript", domainScript, id, map[string]any{"config": cfg})
	return f.cfg.writeErr
}

func (f *readFake) DeleteScript(_ context.Context, id string) error {
	f.recordWrite("DeleteScript", domainScript, id, nil)
	return f.cfg.writeErr
}

func (f *readFake) CreateScene(_ context.Context, id string, cfg homeassistant.SceneConfig) error {
	f.recordWrite("CreateScene", domainScene, id, map[string]any{"config": cfg})
	return f.cfg.writeErr
}

func (f *readFake) UpdateScene(_ context.Context, id string, cfg homeassistant.SceneConfig) error {
	f.recordWrite("UpdateScene", domainScene, id, map[string]any{"config": cfg})
	return f.cfg.writeErr
}

func (f *readFake) DeleteScene(_ context.Context, id string) error {
	f.recordWrite("DeleteScene", domainScene, id, nil)
	return f.cfg.writeErr
}

const (
	regIDSwitchRO = "0123456789abcdef0123456789abcdef"
	regIDKitchen  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

// buildConfigFake is buildWriteFake(true) plus: device dev_m with light.devm
// and the read-only switch.devro, device dev_empty without entities,
// automation.nodel (cfg_nodel), the hidden scene.secret_scene
// (scene_secret_cfg) and registry ids for switch.ro and light.kitchen.
func buildConfigFake() *readFake {
	f := buildWriteFake(true)
	add := func(id, platform, unique, device string) {
		f.states = append(f.states, homeassistant.Entity{EntityID: id, State: "on"})
		f.registry = append(f.registry, homeassistant.EntityRegistryEntry{EntityID: id, Platform: platform, UniqueID: unique, DeviceID: device})
	}
	add("light.devm", "demo", "dm1", "dev_m")
	add("switch.devro", "demo", "dm2", "dev_m")
	add("automation.nodel", "automation", "cfg_nodel", "")
	add("scene.secret_scene", "homeassistant", "scene_secret_cfg", "")
	add("automation.labeled", "automation", "cfg_labeled", "")
	f.registry[len(f.registry)-1].Labels = []string{"lbl_hide"}
	f.labels = append(f.labels, homeassistant.LabelRegistryEntry{LabelID: "lbl_hide"})
	f.devices = append(f.devices,
		homeassistant.DeviceRegistryEntry{ID: "dev_m"},
		homeassistant.DeviceRegistryEntry{ID: "dev_empty"},
	)
	for i := range f.registry {
		switch f.registry[i].EntityID {
		case "switch.ro":
			f.registry[i].ID = regIDSwitchRO
		case "light.kitchen":
			f.registry[i].ID = regIDKitchen
		}
	}
	return f
}

func ents(globs ...string) config.AccessMatch { return config.AccessMatch{Entities: globs} }

func caps(c ...string) config.CapList { return config.CapList(c) }

func boolPtr(b bool) *bool { return &b }

// cfgPolicy: default read; control on lights; write on script.ok and
// script.new_*; configure on scenes; every *.secret* hidden; plus extra.
func cfgPolicy(t *testing.T, extra ...config.AccessRule) *access.Policy {
	t.Helper()
	rules := []config.AccessRule{
		{Match: ents("light.*"), Allow: caps("control")},
		{Match: ents("script.ok", "script.new_*"), Allow: caps("write")},
		{Match: ents("scene.*"), Allow: caps("configure")},
	}
	rules = append(rules, extra...)
	rules = append(rules, config.AccessRule{Match: ents("*.secret*"), Allow: caps("none")})
	return mustCompile(t, config.AccessConfig{Default: "read", Rules: rules})
}

func act(service, entity string) map[string]any {
	return map[string]any{"action": service, "target": map[string]any{"entity_id": entity}}
}

func script(seq ...any) homeassistant.ScriptConfig {
	return homeassistant.ScriptConfig{Alias: "s", Sequence: seq}
}

func noDenial(t *testing.T, tr *access.Tracker) {
	t.Helper()
	if d := tr.First(); d != nil {
		t.Fatalf("unexpected denial %+v", d)
	}
}

func TestCreateAutomation_ExistingIDTreatedAsUpdate(t *testing.T) {
	cfg := homeassistant.AutomationConfig{ID: "cfg_heating", Alias: "Renamed", Triggers: []any{}, Actions: []any{act("light.turn_on", "light.kitchen")}}
	t.Run("configure without create overwrites", func(t *testing.T) {
		f := buildConfigFake()
		p := cfgPolicy(t, config.AccessRule{Match: ents("automation.heating"), Allow: caps("configure"), Create: boolPtr(false)})
		c, tr, ctx := wrapFake(t, p, f)
		if err := c.CreateAutomation(ctx, cfg); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "CreateAutomation", domainAutomation, "cfg_heating", map[string]any{"config": cfg})
		noDenial(t, tr)
	})
	t.Run("without configure denied", func(t *testing.T) {
		f := buildConfigFake()
		p := cfgPolicy(t, config.AccessRule{Match: ents("automation.*"), Allow: caps("control")})
		c, _, ctx := wrapFake(t, p, f)
		errText(t, c.CreateAutomation(ctx, cfg), denialFor("automation.heating", "configure"))
		noInnerWrite(t, f)
	})
}

// TestCreate_CollisionNeutralForHiddenAndVisible: a caller that may create the
// predicted id gets the same neutral answer whatever the id collides with.
// automation.labeled is hidden by its label only: a new entity has no label,
// so the caller may create that id and the collision answer is the neutral one.
func TestCreate_CollisionNeutralForHiddenAndVisible(t *testing.T) {
	f := buildConfigFake()
	f.cfg.files = map[string]bool{"automation/justmade": true}
	p := cfgPolicy(t,
		config.AccessRule{Match: ents("automation.*"), Allow: caps("write")},
		config.AccessRule{Match: config.AccessMatch{Labels: []string{"lbl_hide"}}, Allow: caps("none")},
	)
	c, tr, ctx := wrapFake(t, p, f)
	auto := func(id, alias string) error {
		return c.CreateAutomation(ctx, homeassistant.AutomationConfig{ID: id, Alias: alias, Actions: []any{}})
	}
	tests := []struct {
		name string
		err  error
		x    string
	}{
		{"predicted id of a hidden entity", auto("new1", "Labeled"), "automation.labeled"},
		{"predicted id of a visible entity", auto("new2", "Heating"), "automation.heating"},
		{"existing hidden config id", auto("cfg_secret", "Whatever"), "cfg_secret"},
		{"existing label-hidden config id", auto("cfg_labeled", "Whatever"), "cfg_labeled"},
		{"config file entry without registry entry", auto("justmade", "Fresh Thing"), "justmade"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errText(t, tt.err, fmt.Sprintf("id %q is not available", tt.x))
		})
	}
	if err := c.CreateScript(ctx, "new_x", script()); err != nil {
		t.Errorf("free script key must be creatable: %v", err)
	}
	if got := f.writeCalls(); len(got) != 1 || got[0].method != "CreateScript" {
		t.Errorf("only the free create may reach inner: %+v", got)
	}
	noDenial(t, tr)
}

// TestCreate_NoRightsCannotProbeNames: a caller without create rights on the
// predicted id gets the same denial whether the id (or a hidden config id) is
// taken or free; collisions are only looked at once the grant is sufficient.
func TestCreate_NoRightsCannotProbeNames(t *testing.T) {
	probe := func(t *testing.T, write func(c *client, ctx context.Context) error) (string, *access.Denial, *readFake) {
		t.Helper()
		f := buildConfigFake()
		c, tr, ctx := wrapFake(t, cfgPolicy(t), f)
		err := write(c, ctx)
		if err == nil {
			t.Fatal("expected a refusal")
		}
		return err.Error(), tr.First(), f
	}
	autoAlias := func(id, alias string) func(*client, context.Context) error {
		return func(c *client, ctx context.Context) error {
			return c.CreateAutomation(ctx, homeassistant.AutomationConfig{ID: id, Alias: alias})
		}
	}
	scriptKey := func(key string) func(*client, context.Context) error {
		return func(c *client, ctx context.Context) error { return c.CreateScript(ctx, key, script()) }
	}
	pairs := []struct {
		name          string
		taken, free   func(*client, context.Context) error
		want          string
		sameDenialObj bool
	}{
		{"hidden script key vs free hidden-pattern key", scriptKey("secret_script"), scriptKey("secretx"), genericDenial(), true},
		{"hidden predicted automation vs free hidden-pattern alias", autoAlias("n1", "Secret Auto"), autoAlias("n1", "Secret Other"), genericDenial(), true},
		{"hidden config id vs free id, same alias", autoAlias("cfg_secret", "Whatever"), autoAlias("cfg_free", "Whatever"), denialFor("automation.whatever", "configure"), true},
		{"visible predicted automation vs free alias", autoAlias("n1", "Heating"), autoAlias("n1", "Heatingx"), "", false},
	}
	for _, tt := range pairs {
		t.Run(tt.name, func(t *testing.T) {
			takenErr, takenDenial, f1 := probe(t, tt.taken)
			freeErr, freeDenial, f2 := probe(t, tt.free)
			for _, e := range []string{takenErr, freeErr} {
				if strings.Contains(e, "not available") || strings.Contains(e, "not found") {
					t.Errorf("refusal reveals the collision: %q", e)
				}
			}
			if tt.want != "" && (takenErr != tt.want || freeErr != tt.want) {
				t.Errorf("errors %q / %q, want both %q", takenErr, freeErr, tt.want)
			}
			if tt.sameDenialObj && (takenDenial == nil || freeDenial == nil || *takenDenial != *freeDenial) {
				t.Errorf("recorded denials differ: %+v vs %+v", takenDenial, freeDenial)
			}
			noInnerWrite(t, f1)
			noInnerWrite(t, f2)
			for _, f := range []*readFake{f1, f2} {
				if f.called("ConfigFileEntryExists") {
					t.Error("the config file must not be probed before the grant is sufficient")
				}
			}
		})
	}
}

func TestPredictEntityID_RefusesUnmappableNames(t *testing.T) {
	c := &client{}
	tests := []struct {
		kind, name, want string
	}{
		{domainAutomation, "AI Test", "automation.ai_test"},
		{domainAutomation, "ai-test_1", "automation.ai_test_1"},
		{domainAutomation, "  Spaced  Out ", "automation.spaced_out"},
		{domainScene, "Movie Night 2", "scene.movie_night_2"},
		{domainScript, "my_key", "script.my_key"},
		{domainAutomation, "A.I", ""},
		{domainAutomation, "AI 日本", ""},
		{domainAutomation, "Straße", ""},
		{domainAutomation, "Café", ""},
		{domainScene, "Party 🎉", ""},
		{domainAutomation, "tab\tname", ""},
		{domainAutomation, "", ""},
		{domainAutomation, "---", ""},
	}
	for _, tt := range tests {
		if got := c.predictEntityID(tt.kind, tt.name); got != tt.want {
			t.Errorf("predictEntityID(%s, %q) = %q, want %q", tt.kind, tt.name, got, tt.want)
		}
	}
}

func TestCreate_UnmappableNameRefusedGenerically(t *testing.T) {
	f := buildConfigFake()
	c, _, ctx := wrapFake(t, cfgPolicy(t, config.AccessRule{Match: ents("automation.ai*"), Allow: caps("write")}), f)
	errText(t, c.CreateAutomation(ctx, homeassistant.AutomationConfig{ID: "n1", Alias: "A.I"}), genericDenial())
	errText(t, c.CreateScene(ctx, "1700000001", homeassistant.SceneConfig{Name: "Straße"}), genericDenial())
	noInnerWrite(t, f)
}

func TestCreate_PredictedIDDecidesRule(t *testing.T) {
	f := buildConfigFake()
	p := cfgPolicy(t, config.AccessRule{Match: ents("automation.ai_*"), Allow: caps("write")})
	c, _, ctx := wrapFake(t, p, f)
	ok := homeassistant.AutomationConfig{ID: "n1", Alias: "AI Test", Actions: []any{}}
	if err := c.CreateAutomation(ctx, ok); err != nil {
		t.Fatalf("alias AI Test -> automation.ai_test must be allowed: %v", err)
	}
	errText(t, c.CreateAutomation(ctx, homeassistant.AutomationConfig{ID: "n2", Alias: "Other"}), denialFor("automation.other", "configure"))
	errText(t, c.CreateAutomation(ctx, homeassistant.AutomationConfig{ID: "n3", Alias: "!!!"}), genericDenial())
	errText(t, c.CreateAutomation(ctx, homeassistant.AutomationConfig{Alias: "AI No ID"}), genericDenial())
	p2 := cfgPolicy(t, config.AccessRule{Match: ents("automation.ai_*"), Allow: caps("write"), Create: boolPtr(false)})
	c2, _, ctx2 := wrapFake(t, p2, buildConfigFake())
	errText(t, c2.CreateAutomation(ctx2, ok), denialFor("automation.ai_test", "create"))
}

func TestUpdateScript_EmbeddedNewUncontrolledServiceRefused(t *testing.T) {
	stored := script(act("light.turn_on", "light.kitchen"))
	tests := []struct {
		name  string
		added any
		want  string
	}{
		{"read-only switch", act("switch.turn_on", "switch.ro"), denialFor("switch.ro", "control")},
		{"script call", map[string]any{"action": "script.ro"}, denialFor("script.ro", "control")},
		{"hidden entity", act("light.turn_on", "light.secret"), genericDenial()},
		{"absent entity under a hiding policy", act("light.turn_on", "light.nowhere"), genericDenial()},
		{"system service", map[string]any{"action": "homeassistant.restart"}, systemDenial(access.SystemRestart)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := buildConfigFake()
			f.cfg.scripts = map[string]*homeassistant.ScriptConfig{"ok": &stored}
			c, _, ctx := wrapFake(t, cfgPolicy(t), f)
			errText(t, c.UpdateScript(ctx, "ok", script(stored.Sequence[0], tt.added)), tt.want)
			noInnerWrite(t, f)
		})
	}
}

func TestUpdate_UntouchedLegacyActionNotRechecked(t *testing.T) {
	f := buildConfigFake()
	stored := script(act("switch.turn_on", "switch.ro"))
	f.cfg.scripts = map[string]*homeassistant.ScriptConfig{"ok": &stored}
	c, tr, ctx := wrapFake(t, cfgPolicy(t), f)
	updated := script(act("light.turn_on", "light.kitchen"), act("switch.turn_on", "switch.ro"))
	if err := c.UpdateScript(ctx, "ok", updated); err != nil {
		t.Fatal(err)
	}
	callOnce(t, f, "UpdateScript", domainScript, "ok", map[string]any{"config": updated})
	noDenial(t, tr)
}

func TestUpdate_StoredDefinitionUnusableChecksEveryCall(t *testing.T) {
	var blueprint homeassistant.ScriptConfig
	if err := json.Unmarshal([]byte(`{"use_blueprint": {"path": "x.yaml"}, "sequence": [
	  {"action": "switch.turn_on", "target": {"entity_id": "switch.ro"}}]}`), &blueprint); err != nil {
		t.Fatal(err)
	}
	for name, stored := range map[string]*homeassistant.ScriptConfig{"no stored config": nil, "unanalysable stored config": &blueprint} {
		t.Run(name, func(t *testing.T) {
			f := buildConfigFake()
			f.cfg.scripts = map[string]*homeassistant.ScriptConfig{"ok": stored}
			c, _, ctx := wrapFake(t, cfgPolicy(t), f)
			errText(t, c.UpdateScript(ctx, "ok", script(act("switch.turn_on", "switch.ro"))), denialFor("switch.ro", "control"))
			noInnerWrite(t, f)
		})
	}
}

func TestEmbedded_TemplatedServiceRefused(t *testing.T) {
	f := buildConfigFake()
	c, tr, ctx := wrapFake(t, cfgPolicy(t), f)
	errText(t, c.CreateScript(ctx, "new_t", script(map[string]any{"action": "{{ svc }}"})), genericDenial())
	if d := tr.First(); d == nil || !strings.Contains(d.Object, "embedded action") {
		t.Errorf("denial must name the embedded action: %+v", d)
	}
	errText(t, c.UpdateScript(ctx, "ok", script(map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": "{{ x }}"}})), genericDenial())
	errText(t, c.CreateScript(ctx, "new_k", script(map[string]any{"action": "light.turn_on", "data": map[string]any{"{{ 'entity_id' }}": "switch.ro"}})), genericDenial())
	noInnerWrite(t, f)
}

func TestEmbedded_UseBlueprintRefused(t *testing.T) {
	var cfg homeassistant.AutomationConfig
	if err := json.Unmarshal([]byte(`{"id": "bp1", "alias": "AI BP", "use_blueprint": {"path": "x.yaml", "input": {}}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	f := buildConfigFake()
	c, _, ctx := wrapFake(t, cfgPolicy(t, config.AccessRule{Match: ents("automation.ai_*"), Allow: caps("write")}), f)
	errText(t, c.CreateAutomation(ctx, cfg), genericDenial())
	noInnerWrite(t, f)
}

func TestEmbedded_DeviceActionsAndRegistryIDs(t *testing.T) {
	device := func(dev string, extra map[string]any) map[string]any {
		m := map[string]any{"device_id": dev, "domain": "light", "type": "turn_on"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	tests := []struct {
		name   string
		action map[string]any
		want   string // "" = allowed
	}{
		{"every entity of the device is checked", device("dev_m", nil), denialFor("switch.devro", "control")},
		{"device with controllable entities only", device("dev_l", nil), ""},
		{"device without entities, default read", device("dev_empty", nil), genericDenial()},
		{"unknown device", device("dev_nowhere", nil), genericDenial()},
		{"registry id resolved", device("dev_k", map[string]any{"entity_id": regIDKitchen}), ""},
		{"registry id of a read-only entity", device("dev_k", map[string]any{"entity_id": regIDSwitchRO}), denialFor("switch.ro", "control")},
		{"unknown registry id", device("dev_k", map[string]any{"entity_id": "ffffffffffffffffffffffffffffffff"}), genericDenial()},
		{"device of a hidden entity", device("dev_k", map[string]any{"entity_id": "light.secret"}), genericDenial()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := buildConfigFake()
			c, _, ctx := wrapFake(t, cfgPolicy(t), f)
			err := c.CreateScript(ctx, "new_dev", script(tt.action))
			if tt.want == "" {
				if err != nil {
					t.Fatalf("allowed device action refused: %v", err)
				}
				return
			}
			errText(t, err, tt.want)
			noInnerWrite(t, f)
		})
	}
}

func TestEmbedded_DeviceTrackerSeeNeedsData(t *testing.T) {
	f := buildConfigFake()
	p := cfgPolicy(t, config.AccessRule{Match: ents("device_tracker.*"), Allow: caps("write")})
	c, _, ctx := wrapFake(t, p, f)
	see := map[string]any{"action": "device_tracker.see", "data": map[string]any{"dev_id": "phone", "mac": "aa"}}
	errText(t, c.CreateScript(ctx, "new_see", script(see)), genericDenial())
	ok := map[string]any{"action": "device_tracker.see", "data": map[string]any{"dev_id": "phone"}}
	if err := c.CreateScript(ctx, "new_see", script(ok)); err != nil {
		t.Fatalf("dev_id-only see must pass: %v", err)
	}
}

func TestSceneID_ResolvedViaRegistryUniqueID(t *testing.T) {
	cfg := homeassistant.SceneConfig{Name: "Anything", Entities: map[string]homeassistant.SceneState{"light.kitchen": {State: "on"}}}
	f := buildConfigFake()
	c, _, ctx := wrapFake(t, cfgPolicy(t), f)
	if err := c.UpdateScene(ctx, "scene_ok_cfg", cfg); err != nil {
		t.Fatalf("scene_ok_cfg resolves to scene.ok: %v", err)
	}
	callOnce(t, f, "UpdateScene", domainScene, "scene_ok_cfg", map[string]any{"config": cfg})
	sameError(t, c.UpdateScene(ctx, "ok", cfg), homeassistant.NewConfigNotFoundError("scene", "ok"))

	ro := cfgPolicy(t, config.AccessRule{Match: ents("scene.ok"), Deny: caps("configure")})
	c2, _, ctx2 := wrapFake(t, ro, buildConfigFake())
	errText(t, c2.UpdateScene(ctx2, "scene_ok_cfg", cfg), denialFor("scene.ok", "configure"))
}

func TestScene_EntitiesNeedControl(t *testing.T) {
	scene := func(id string) homeassistant.SceneConfig {
		return homeassistant.SceneConfig{Name: "Fresh", Entities: map[string]homeassistant.SceneState{id: {State: "on"}}}
	}
	tests := []struct {
		entity, want string
	}{
		{"light.kitchen", ""},
		{"switch.ro", denialFor("switch.ro", "control")},
		{"light.secret", genericDenial()},
		{"Light.Kitchen", genericDenial()},
	}
	for _, tt := range tests {
		t.Run(tt.entity, func(t *testing.T) {
			f := buildConfigFake()
			c, _, ctx := wrapFake(t, cfgPolicy(t), f)
			err := c.CreateScene(ctx, "1700000000", scene(tt.entity))
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			errText(t, err, tt.want)
			noInnerWrite(t, f)
		})
	}
}

func TestDelete_RespectsDeleteFalse(t *testing.T) {
	f := buildConfigFake()
	p := cfgPolicy(t,
		config.AccessRule{Match: ents("automation.nodel"), Allow: caps("configure"), Delete: boolPtr(false)},
		config.AccessRule{Match: ents("automation.heating"), Allow: caps("configure")},
	)
	c, _, ctx := wrapFake(t, p, f)
	errText(t, c.DeleteAutomation(ctx, "cfg_nodel"), denialFor("automation.nodel", "delete"))
	errText(t, c.DeleteScript(ctx, "ro"), denialFor("script.ro", "configure"))
	noInnerWrite(t, f)
	if err := c.DeleteAutomation(ctx, "cfg_heating"); err != nil {
		t.Fatal(err)
	}
	callOnce(t, f, "DeleteAutomation", domainAutomation, "cfg_heating", nil)
}

func TestConfigWrite_HiddenTargetsLookMissing(t *testing.T) {
	f := buildConfigFake()
	c, tr, ctx := wrapFake(t, cfgPolicy(t, config.AccessRule{Match: ents("automation.*"), Allow: caps("write")}), f)
	missing := homeassistant.NewConfigNotFoundError
	tests := []struct {
		name      string
		hidden    error
		absent    error
		kind      string
		hid, absK string
	}{
		{"update script", c.UpdateScript(ctx, "secret_script", script()), c.UpdateScript(ctx, "nope", script()), "script", "secret_script", "nope"},
		{"delete script", c.DeleteScript(ctx, "secret_script"), c.DeleteScript(ctx, "nope"), "script", "secret_script", "nope"},
		{"update automation", c.UpdateAutomation(ctx, "cfg_secret", homeassistant.AutomationConfig{}), c.UpdateAutomation(ctx, "cfg_nope", homeassistant.AutomationConfig{}), "automation", "cfg_secret", "cfg_nope"},
		{"delete automation", c.DeleteAutomation(ctx, "cfg_secret"), c.DeleteAutomation(ctx, "cfg_nope"), "automation", "cfg_secret", "cfg_nope"},
		{"update scene", c.UpdateScene(ctx, "scene_secret_cfg", homeassistant.SceneConfig{}), c.UpdateScene(ctx, "scene_nope", homeassistant.SceneConfig{}), "scene", "scene_secret_cfg", "scene_nope"},
		{"delete scene", c.DeleteScene(ctx, "scene_secret_cfg"), c.DeleteScene(ctx, "scene_nope"), "scene", "scene_secret_cfg", "scene_nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sameError(t, tt.hidden, missing(tt.kind, tt.hid))
			sameError(t, tt.absent, missing(tt.kind, tt.absK))
			var a, b *homeassistant.APIError
			if !errors.As(tt.hidden, &a) || !errors.As(tt.absent, &b) || a.StatusCode != b.StatusCode {
				t.Errorf("hidden and absent must have the same error type and status")
			}
		})
	}
	noInnerWrite(t, f)
	noDenial(t, tr)
}

// TestDeleteScript_HiddenFallbackCannotRemoveEntity follows the script
// handler's delete path: DeleteScript answers not-found, then the handler looks
// the entity up in the registry and removes it. Neither step reaches a hidden
// script.
func TestDeleteScript_HiddenFallbackCannotRemoveEntity(t *testing.T) {
	f := buildConfigFake()
	c, _, ctx := wrapFake(t, cfgPolicy(t), f)
	if err := c.DeleteScript(ctx, "secret_script"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("hidden script delete must answer not found, got %v", err)
	}
	reg, err := c.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range reg {
		if e.EntityID == "script.secret_script" {
			t.Fatal("hidden script listed by the registry the fallback searches")
		}
	}
	if err := c.RemoveEntityRegistryEntry(ctx, "script.secret_script"); err == nil {
		t.Fatal("registry removal of a hidden script must fail")
	}
	noInnerWrite(t, f)
}

func TestConfigWrite_MarkConfiguredEnablesReload(t *testing.T) {
	f := buildConfigFake()
	c, _, ctx := wrapFake(t, cfgPolicy(t), f)
	_, err := c.CallService(ctx, domainScript, "reload", nil)
	errText(t, err, systemDenial(access.SystemReload))
	errText(t, c.CreateScript(ctx, "new_t", script(map[string]any{"action": "{{ svc }}"})), genericDenial())
	if _, err := c.CallService(ctx, domainScript, "reload", nil); err == nil {
		t.Fatal("a refused write must not enable the reload")
	}
	if err := c.CreateScript(ctx, "new_ok", script(act("light.turn_on", "light.kitchen"))); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CallService(ctx, domainScript, "reload", nil); err != nil {
		t.Fatalf("reload after a successful create: %v", err)
	}
	if _, err := c.CallService(ctx, domainAutomation, "reload", nil); err == nil {
		t.Fatal("only the configured domain may reload")
	}
}

func TestConfigWrite_RegistryFailureRefuses(t *testing.T) {
	f := buildConfigFake()
	f.registryFn = func() error { return errors.New("boom") }
	c, _, ctx := wrapFake(t, cfgPolicy(t), f)
	errText(t, c.CreateScript(ctx, "new_x", script()), genericDenial())
	errText(t, c.UpdateScript(ctx, "ok", script()), genericDenial())
	errText(t, c.DeleteScene(ctx, "scene_ok_cfg"), genericDenial())
	noInnerWrite(t, f)
}

func TestConfigWrite_SnapshotInvalidatedAfterSuccess(t *testing.T) {
	f := buildConfigFake()
	c, _, ctx := wrapFake(t, cfgPolicy(t), f)
	snapCached := func() bool {
		c.snap.mu.Lock()
		defer c.snap.mu.Unlock()
		return c.snap.data != nil
	}
	_ = c.CreateScript(ctx, "new_t", script(map[string]any{"action": "{{ svc }}"}))
	if !snapCached() {
		t.Fatal("a refused write must keep the snapshot")
	}
	if err := c.CreateScript(ctx, "new_ok", script()); err != nil {
		t.Fatal(err)
	}
	if snapCached() {
		t.Fatal("a successful write must drop the snapshot")
	}
}

// A failed write (a timeout, say) may still have been applied by HA, so the
// snapshot is dropped whatever inner returns.
func TestConfigWrite_SnapshotInvalidatedAfterInnerError(t *testing.T) {
	writes := map[string]func(*client, context.Context) error{
		"CreateAutomation": func(c *client, ctx context.Context) error {
			return c.CreateAutomation(ctx, homeassistant.AutomationConfig{ID: "n1", Alias: "AI Test"})
		},
		"UpdateAutomation": func(c *client, ctx context.Context) error {
			return c.UpdateAutomation(ctx, "cfg_heating", homeassistant.AutomationConfig{})
		},
		"DeleteAutomation": func(c *client, ctx context.Context) error { return c.DeleteAutomation(ctx, "cfg_heating") },
		"CreateScript":     func(c *client, ctx context.Context) error { return c.CreateScript(ctx, "new_x", script()) },
		"UpdateScript":     func(c *client, ctx context.Context) error { return c.UpdateScript(ctx, "ok", script()) },
		"DeleteScript":     func(c *client, ctx context.Context) error { return c.DeleteScript(ctx, "ok") },
		"CreateScene": func(c *client, ctx context.Context) error {
			return c.CreateScene(ctx, "1700000002", homeassistant.SceneConfig{Name: "Fresh"})
		},
		"UpdateScene": func(c *client, ctx context.Context) error {
			return c.UpdateScene(ctx, "scene_ok_cfg", homeassistant.SceneConfig{Name: "OK"})
		},
		"DeleteScene": func(c *client, ctx context.Context) error { return c.DeleteScene(ctx, "scene_ok_cfg") },
	}
	p := func(t *testing.T) *access.Policy {
		t.Helper()
		return cfgPolicy(t, config.AccessRule{Match: ents("automation.ai_*", "automation.heating"), Allow: caps("write")})
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			f := buildConfigFake()
			boom := errors.New("timeout")
			f.cfg.writeErr = boom
			c, _, ctx := wrapFake(t, p(t), f)
			if err := write(c, ctx); !errors.Is(err, boom) {
				t.Fatalf("inner error must be returned, got %v", err)
			}
			c.snap.mu.Lock()
			cached := c.snap.data != nil
			c.snap.mu.Unlock()
			if cached {
				t.Fatal("snapshot must be dropped after a failed write")
			}
		})
	}
}

func TestConfigWrite_IDShapeAndBodyID(t *testing.T) {
	f := buildConfigFake()
	c, _, ctx := wrapFake(t, cfgPolicy(t, config.AccessRule{Match: ents("automation.*"), Allow: caps("write")}), f)
	errText(t, c.CreateScript(ctx, "New Key", script()), genericDenial())
	sameError(t, c.UpdateScript(ctx, "../ok", script()), homeassistant.NewConfigNotFoundError("script", "../ok"))
	errText(t, c.UpdateAutomation(ctx, "cfg_heating", homeassistant.AutomationConfig{ID: "cfg_other"}), genericDenial())
	noInnerWrite(t, f)
}

func TestCreateAllowedWhenPolicyGrantsEverything_SkipsEmbedded(t *testing.T) {
	templated := script(map[string]any{"action": "{{ svc }}"})
	t.Run("unrestricted skips the analysis", func(t *testing.T) {
		f := buildConfigFake()
		ctx, tr := access.WithTracker(context.Background(), "some_tool")
		c := &client{inner: f, p: mustCompile(t, config.AccessConfig{Default: "write"}), tr: tr, snap: &snapshot{inner: f}}
		if err := c.CreateScript(ctx, "templ", templated); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "CreateScript", domainScript, "templ", map[string]any{"config": templated})
	})
	t.Run("default write with a deny still analyses", func(t *testing.T) {
		f := buildConfigFake()
		p := mustCompile(t, config.AccessConfig{Default: "write", Rules: []config.AccessRule{
			{Match: ents("switch.ro"), Deny: caps("control")},
		}})
		c, _, ctx := wrapFake(t, p, f)
		errText(t, c.CreateScript(ctx, "templ", templated), genericDenial())
		errText(t, c.CreateScript(ctx, "templ", script(act("switch.turn_on", "switch.ro"))), denialFor("switch.ro", "control"))
		noInnerWrite(t, f)
		if err := c.CreateScript(ctx, "templ", script(act("light.turn_on", "light.kitchen"))); err != nil {
			t.Fatal(err)
		}
	})
}
