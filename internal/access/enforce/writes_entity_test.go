package enforce

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// callOnce asserts the inner client received exactly one write, with the
// original arguments.
func callOnce(t *testing.T, f *readFake, method, domain, service string, data map[string]any) {
	t.Helper()
	got := f.writeCalls()
	want := []writeCall{{method: method, domain: domain, service: service, data: data}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inner writes = %+v, want %+v", got, want)
	}
}

func noInnerWrite(t *testing.T, f *readFake) {
	t.Helper()
	if got := f.writeCalls(); len(got) != 0 {
		t.Fatalf("inner was called: %+v", got)
	}
}

func errText(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %q, got nil", want)
	}
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestCallService_AnyBlockedTargetRefusesWhole(t *testing.T) {
	f := buildWriteFake(true)
	c, tr, ctx := wrapFake(t, writePolicy(t), f)
	data := map[string]any{"entity_id": []any{"light.kitchen", "switch.ro", "light.hall"}}
	_, err := c.CallService(ctx, "homeassistant", "turn_on", data)
	errText(t, err, denialFor("switch.ro", "control"))
	noInnerWrite(t, f)
	if d := tr.First(); d == nil || d.Object != "switch.ro" || d.Capability != "control" {
		t.Errorf("denial = %+v", d)
	}
}

func TestCallService_AllowedCallsInnerOnceWithOriginalData(t *testing.T) {
	f := buildWriteFake(true)
	c, tr, ctx := wrapFake(t, writePolicy(t), f)
	data := map[string]any{"entity_id": "light.kitchen", "brightness": 10}
	if _, err := c.CallService(ctx, "light", "turn_on", data); err != nil {
		t.Fatal(err)
	}
	callOnce(t, f, "CallService", "light", "turn_on", map[string]any{"entity_id": "light.kitchen", "brightness": 10})
	if tr.First() != nil {
		t.Errorf("denial recorded on an allowed call: %+v", tr.First())
	}
}

func TestCallService_AllCommaFloorAreaLabelExpansion(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		data    map[string]any
		wantErr string // "" = allowed
	}{
		{"comma string", "light", map[string]any{"entity_id": "light.kitchen, light.hall"}, ""},
		{"comma string with read-only", "homeassistant", map[string]any{"entity_id": "light.kitchen,switch.ro"}, denialFor("switch.ro", "control")},
		{"area", "light", map[string]any{"area_id": "hall"}, ""},
		{"area read-only", "light", map[string]any{"area_id": "office"}, denialFor("switch.office", "control")},
		{"floor", "light", map[string]any{"floor_id": "ground_hall"}, ""},
		{"floor read-only", "light", map[string]any{"floor_id": "upstairs"}, denialFor("switch.office", "control")},
		{"device", "light", map[string]any{"device_id": "dev_k"}, ""},
		{"device area inherited", "light", map[string]any{"area_id": "kitchen", "entity_id": "light.hall"}, genericDenial()},
		{"label on entity", "light", map[string]any{"label_id": "lbl_ro"}, denialFor("switch.ro", "control")},
		{"label on device", "light", map[string]any{"label_id": "lbl_dev"}, ""},
		{"label on area", "light", map[string]any{"label_id": "lbl_area"}, ""},
		{"label list mixed", "light", map[string]any{"label_id": []any{"lbl_dev", "lbl_ro"}}, denialFor("switch.ro", "control")},
		{"all of a domain incl. states-only", "input_boolean", map[string]any{"entity_id": "all"}, ""},
		{"all reaches states-only read-only", "cover", map[string]any{"entity_id": "all"}, denialFor("cover.yaml_ro", "control")},
		{"all with a hidden member", "media_player", map[string]any{"entity_id": "all"}, genericDenial()},
		{"nested target", "light", map[string]any{"entity_id": "light.kitchen", "target": map[string]any{"entity_id": "switch.ro"}}, denialFor("switch.ro", "control")},
		{"nested target area", "light", map[string]any{"target": map[string]any{"area_id": "office"}}, denialFor("switch.office", "control")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := buildWriteFake(true)
			c, _, ctx := wrapFake(t, writePolicy(t), f)
			_, err := c.CallService(ctx, tc.domain, "turn_on", tc.data)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				callOnce(t, f, "CallService", tc.domain, "turn_on", tc.data)
				return
			}
			errText(t, err, tc.wantErr)
			noInnerWrite(t, f)
		})
	}
}

func TestCallService_HiddenInExpansionIsNotRevealed(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"area":  {"area_id": "kitchen"},
		"floor": {"floor_id": "ground"},
		"all":   {"entity_id": "all"},
	} {
		t.Run(name, func(t *testing.T) {
			f := buildWriteFake(true)
			c, tr, ctx := wrapFake(t, writePolicy(t), f)
			_, err := c.CallService(ctx, "light", "turn_on", data)
			errText(t, err, genericDenial())
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("hidden id in error: %v", err)
			}
			d := tr.First()
			if d == nil || strings.Contains(d.Object, "secret") || d.Object != "light.turn_on" {
				t.Errorf("denial = %+v, want the service as object", d)
			}
			noInnerWrite(t, f)
		})
	}
}

func TestCallService_HiddenIDLooksMissing(t *testing.T) {
	tests := []struct {
		name            string
		domain, service string
		hidden, missing map[string]any
		wantHidden      string
		wantMissing     string
	}{
		{"target", "light", "turn_on",
			map[string]any{"entity_id": "light.secret"}, map[string]any{"entity_id": "light.secret_absent"},
			"light.secret", "light.secret_absent"},
		{"before a read-only id", "homeassistant", "turn_on",
			map[string]any{"entity_id": []any{"switch.ro", "light.secret"}}, map[string]any{"entity_id": []any{"switch.ro", "light.nothere"}},
			"light.secret", "light.nothere"},
		{"after a visible id", "light", "turn_on",
			map[string]any{"entity_id": "light.kitchen,light.secret"}, map[string]any{"entity_id": "light.kitchen,light.nothere"},
			"light.secret", "light.nothere"},
		{"padded and mixed case", "light", "turn_on",
			map[string]any{"entity_id": []any{"  LIGHT.Secret "}}, map[string]any{"entity_id": []any{"  LIGHT.Nothere "}},
			"light.secret", "light.nothere"},
		{"invalid shape", "light", "turn_on",
			map[string]any{"entity_id": "light.secret/x"}, map[string]any{"entity_id": "light.nothere/x"},
			"light.secret/x", "light.nothere/x"},
		{"scene.apply map key", "scene", "apply",
			map[string]any{"entities": map[string]any{"light.secret": "on"}}, map[string]any{"entities": map[string]any{"light.nothere": "on"}},
			"light.secret", "light.nothere"},
		{"tts media player", "tts", "speak",
			map[string]any{"entity_id": "tts.engine", "media_player_entity_id": "media_player.secret_mp", "message": "x"},
			map[string]any{"entity_id": "tts.engine", "media_player_entity_id": "media_player.nothere", "message": "x"},
			"media_player.secret_mp", "media_player.nothere"},
		{"camera stream player", "camera", "play_stream",
			map[string]any{"entity_id": "camera.front", "media_player": "media_player.secret_mp"},
			map[string]any{"entity_id": "camera.front", "media_player": "media_player.nothere"},
			"media_player.secret_mp", "media_player.nothere"},
		{"read service", "todo", "get_items",
			map[string]any{"entity_id": "todo.secret_list"}, map[string]any{"entity_id": "todo.nothere"},
			"todo.secret_list", "todo.nothere"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := buildWriteFake(true)
			c, tr, ctx := wrapFake(t, writePolicy(t), f)
			_, errHidden := c.CallService(ctx, tc.domain, tc.service, tc.hidden)
			_, errMissing := c.CallService(ctx, tc.domain, tc.service, tc.missing)
			errText(t, errHidden, homeassistant.NewEntityNotFoundError(tc.wantHidden).Error())
			errText(t, errMissing, homeassistant.NewEntityNotFoundError(tc.wantMissing).Error())
			if tr.First() != nil {
				t.Errorf("a hidden or missing id must not record a denial: %+v", tr.First())
			}
			noInnerWrite(t, f)
		})
	}
}

func TestCallService_HiddenAndMissingIdenticalAcrossWorlds(t *testing.T) {
	// The same request against a home with the hidden entity and one without it.
	for _, data := range []map[string]any{
		{"entity_id": "light.secret"},
		{"entity_id": []any{"switch.ro", "light.secret"}},
		{"entity_id": " Light.Secret"},
		{"entity_id": "light.secret", "area_id": "hall"},
	} {
		withSecret, _, ctxA := wrapFake(t, writePolicy(t), buildWriteFake(true))
		without, _, ctxB := wrapFake(t, writePolicy(t), buildWriteFake(false))
		_, errA := withSecret.CallService(ctxA, "light", "turn_on", data)
		_, errB := without.CallService(ctxB, "light", "turn_on", data)
		sameError(t, errA, errB)
	}
}

func TestCallService_ObjectLess_UsesDefault(t *testing.T) {
	data := map[string]any{"message": "hi"}
	f := buildWriteFake(true)
	c, tr, ctx := wrapFake(t, writePolicy(t), f)
	_, err := c.CallService(ctx, "notify", "x", data)
	errText(t, err, denialFor("notify.x", "control"))
	noInnerWrite(t, f)
	if d := tr.First(); d == nil || d.Object != "notify.x" {
		t.Errorf("denial = %+v", d)
	}

	writeDefault := mustCompile(t, config.AccessConfig{
		Default: "write",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"switch.ro"}}, Deny: config.CapList{"control"}}},
	})
	f = buildWriteFake(true)
	c, _, ctx = wrapFake(t, writeDefault, f)
	if _, err := c.CallService(ctx, "notify", "x", data); err != nil {
		t.Fatal(err)
	}
	callOnce(t, f, "CallService", "notify", "x", data)
}

func TestCallService_UnknownTargetIDs(t *testing.T) {
	// HA ignores unknown device/area/floor/label ids. Under a hiding policy a
	// target that names ids but reaches no entity is refused like one that
	// reaches a hidden entity, so the two cannot be told apart.
	unknown := map[string]any{"device_id": "no_such_device"}
	hiddenOnly := map[string]any{"area_id": "kitchen"}
	f := buildWriteFake(true)
	c, _, ctx := wrapFake(t, writePolicy(t), f)
	_, errUnknown := c.CallService(ctx, "light", "turn_on", unknown)
	_, errHidden := c.CallService(ctx, "light", "turn_on", hiddenOnly)
	errText(t, errUnknown, genericDenial())
	sameError(t, errUnknown, errHidden)
	noInnerWrite(t, f)

	// Without hiding, an unknown id is object-less: the default decides.
	writeDefault := mustCompile(t, config.AccessConfig{
		Default: "write",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"switch.ro"}}, Deny: config.CapList{"control"}}},
	})
	f = buildWriteFake(true)
	c, _, ctx = wrapFake(t, writeDefault, f)
	if _, err := c.CallService(ctx, "light", "turn_on", unknown); err != nil {
		t.Fatal(err)
	}
	callOnce(t, f, "CallService", "light", "turn_on", unknown)
}

func TestCallService_ScriptAsService(t *testing.T) {
	tests := []struct {
		service string
		wantErr string
	}{
		{"ok", ""},
		{"ro", denialFor("script.ro", "control")},
		{"secret_script", homeassistant.NewEntityNotFoundError("script.secret_script").Error()},
		{"nothere", homeassistant.NewEntityNotFoundError("script.nothere").Error()},
	}
	for _, tc := range tests {
		t.Run(tc.service, func(t *testing.T) {
			f := buildWriteFake(true)
			c, _, ctx := wrapFake(t, writePolicy(t), f)
			data := map[string]any{"var": 1}
			_, err := c.CallService(ctx, "script", tc.service, data)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				callOnce(t, f, "CallService", "script", tc.service, data)
				return
			}
			errText(t, err, tc.wantErr)
			noInnerWrite(t, f)
		})
	}
}

func TestCallService_EntityDataKeys(t *testing.T) {
	tests := []struct {
		name            string
		domain, service string
		data            map[string]any
		wantErr         string
	}{
		{"scene.apply ok", "scene", "apply", map[string]any{"entities": map[string]any{"light.kitchen": "on", "light.hall": map[string]any{"state": "off"}}}, ""},
		{"scene.apply read-only key", "scene", "apply", map[string]any{"entities": map[string]any{"light.kitchen": "on", "switch.ro": "off"}}, denialFor("switch.ro", "control")},
		{"scene.create ok", "scene", "create", map[string]any{"scene_id": "new_scene", "entities": map[string]any{"light.kitchen": "on"}}, ""},
		{"scene.create snapshot read-only", "scene", "create", map[string]any{"scene_id": "new_scene", "snapshot_entities": []any{"switch.ro"}}, denialFor("switch.ro", "control")},
		{"group.set entities read-only", "group", "set", map[string]any{"object_id": "existing", "entities": []any{"light.kitchen", "switch.ro"}}, denialFor("switch.ro", "control")},
		{"group.set add_entities ok", "group", "set", map[string]any{"object_id": "existing", "add_entities": "light.kitchen"}, ""},
		{"configure target needs configure", "input_select", "set_options", map[string]any{"entity_id": "light.kitchen", "options": []any{"a"}}, denialFor("light.kitchen", "configure")},
		{"media_player.join ok", "media_player", "join", map[string]any{"entity_id": "media_player.living", "group_members": []any{"media_player.ro"}}, ""},
		{"tts.speak ok", "tts", "speak", map[string]any{"entity_id": "tts.engine", "media_player_entity_id": "media_player.living", "message": "x"}, ""},
		{"camera.play_stream ok", "camera", "play_stream", map[string]any{"entity_id": "camera.front", "media_player": "media_player.living"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := buildWriteFake(true)
			c, _, ctx := wrapFake(t, writePolicy(t), f)
			_, err := c.CallService(ctx, tc.domain, tc.service, tc.data)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				callOnce(t, f, "CallService", tc.domain, tc.service, tc.data)
				return
			}
			errText(t, err, tc.wantErr)
			noInnerWrite(t, f)
		})
	}
}

func TestCallService_DataEntitiesNeedControl(t *testing.T) {
	// media_player.ro is controllable by the media_player.* rule; a read-only
	// player must be refused as a join member.
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{"media_player.living", "tts.*"}}, Allow: config.CapList{"control"}},
		},
	})
	for _, tc := range []struct {
		domain, service string
		data            map[string]any
	}{
		{"media_player", "join", map[string]any{"entity_id": "media_player.living", "group_members": []any{"media_player.ro"}}},
		{"tts", "speak", map[string]any{"entity_id": "tts.engine", "media_player_entity_id": "media_player.ro"}},
	} {
		f := buildWriteFake(true)
		c, _, ctx := wrapFake(t, p, f)
		_, err := c.CallService(ctx, tc.domain, tc.service, tc.data)
		errText(t, err, denialFor("media_player.ro", "control"))
		noInnerWrite(t, f)
	}
}

func TestCallService_ObjectFromData(t *testing.T) {
	noDelete := false
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{"scene.*", "group.*"}}, Allow: config.CapList{"configure"}},
			{Match: config.AccessMatch{Entities: []string{"group.keep*"}}, Allow: config.CapList{"configure"}, Delete: &noDelete},
			{Match: config.AccessMatch{Entities: []string{"*.secret*"}}, Allow: config.CapList{"none"}},
		},
	})
	tests := []struct {
		name            string
		domain, service string
		data            map[string]any
		allowed         bool
	}{
		{"new scene", "scene", "create", map[string]any{"scene_id": "fresh"}, true},
		{"missing scene_id", "scene", "create", map[string]any{"entities": map[string]any{}}, false},
		{"non-string scene_id", "scene", "create", map[string]any{"scene_id": 5}, false},
		{"scene_id not a slug", "scene", "create", map[string]any{"scene_id": "Bad Id"}, false},
		{"padded scene_id", "scene", "create", map[string]any{"scene_id": " fresh"}, false},
		{"hidden object", "scene", "create", map[string]any{"scene_id": "secret_x"}, false},
		{"group remove", "group", "remove", map[string]any{"object_id": "existing"}, true},
		{"group remove without delete", "group", "remove", map[string]any{"object_id": "keep_me"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := buildWriteFake(true)
			c, tr, ctx := wrapFake(t, p, f)
			_, err := c.CallService(ctx, tc.domain, tc.service, tc.data)
			if tc.allowed {
				if err != nil {
					t.Fatal(err)
				}
				callOnce(t, f, "CallService", tc.domain, tc.service, tc.data)
				return
			}
			errText(t, err, genericDenial())
			if d := tr.First(); d == nil || strings.Contains(d.Object, "secret") {
				t.Errorf("denial = %+v", d)
			}
			noInnerWrite(t, f)
		})
	}
}

func TestCallService_ObjectFromDataReadOnlyDefault(t *testing.T) {
	// Without configure, scene.create is refused whether or not the scene exists.
	f := buildWriteFake(true)
	c, _, ctx := wrapFake(t, readPolicy(t), f)
	for _, id := range []string{"ok_new", "ok"} {
		_, err := c.CallService(ctx, "scene", "create", map[string]any{"scene_id": id})
		errText(t, err, genericDenial())
	}
	noInnerWrite(t, f)
}

func TestCallService_MalformedRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		domain, service string
		data            map[string]any
	}{
		"number target":     {"light", "turn_on", map[string]any{"entity_id": 5}},
		"string target key": {"light", "turn_on", map[string]any{"target": "light.kitchen"}},
		"null area":         {"light", "turn_on", map[string]any{"area_id": nil}},
		"bad entities map":  {"scene", "apply", map[string]any{"entities": 5}},
		"bad list element":  {"media_player", "join", map[string]any{"entity_id": "media_player.living", "group_members": []any{1}}},
	} {
		t.Run(name, func(t *testing.T) {
			f := buildWriteFake(true)
			c, tr, ctx := wrapFake(t, writePolicy(t), f)
			_, err := c.CallService(ctx, tc.domain, tc.service, tc.data)
			errText(t, err, genericDenial())
			if tr.First() == nil {
				t.Error("refusal not recorded")
			}
			noInnerWrite(t, f)
		})
	}
}

func TestCallService_SnapshotFailureFailsClosed(t *testing.T) {
	f := buildWriteFake(true)
	f.registryFn = func() error { return errors.New("registry down") }
	c, _, ctx := wrapFake(t, writePolicy(t), f)
	_, err := c.CallService(ctx, "light", "turn_on", map[string]any{"area_id": "hall"})
	errText(t, err, genericDenial())
	if strings.Contains(err.Error(), "registry") || strings.Contains(err.Error(), "snapshot") {
		t.Errorf("internals leaked: %v", err)
	}
	noInnerWrite(t, f)
}

func TestInternalReload_AllowedOnlyAfterConfigure(t *testing.T) {
	f := buildWriteFake(true)
	c, tr, ctx := wrapFake(t, writePolicy(t), f)
	_, err := c.CallService(ctx, "automation", "reload", nil)
	errText(t, err, systemDenial(access.SystemReload))
	noInnerWrite(t, f)

	tr.MarkConfigured("automation")
	if _, err := c.CallService(ctx, "automation", "reload", nil); err != nil {
		t.Fatalf("reload after configure: %v", err)
	}
	if _, err := c.CallService(ctx, "automation", "reload", map[string]any{"id": "cfg_heating"}); err != nil {
		t.Fatalf("reload with id after configure: %v", err)
	}
	if got := len(f.writeCalls()); got != 2 {
		t.Fatalf("inner calls = %d, want 2", got)
	}
	// Extra data or another domain is not covered by the exception.
	_, err = c.CallService(ctx, "automation", "reload", map[string]any{"id": "x", "entity_id": "light.kitchen"})
	errText(t, err, systemDenial(access.SystemReload))
	_, err = c.CallService(ctx, "script", "reload", nil)
	errText(t, err, systemDenial(access.SystemReload))
	_, err = c.CallService(ctx, "automation", "reload", map[string]any{"id": 5})
	errText(t, err, systemDenial(access.SystemReload))
	if got := len(f.writeCalls()); got != 2 {
		t.Fatalf("inner calls = %d, want 2", got)
	}

	// With the reload system value no configure is needed.
	f = buildWriteFake(true)
	c, _, ctx = wrapFake(t, writePolicy(t, "reload"), f)
	if _, err := c.CallService(ctx, "script", "reload", nil); err != nil {
		t.Fatal(err)
	}
}

func TestSystemOps_RequireSystemValue(t *testing.T) {
	tests := []struct {
		name  string
		value access.SystemValue
		call  func(context.Context, homeassistant.Client) error
	}{
		{"homeassistant.restart", access.SystemRestart, func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.CallService(ctx, "homeassistant", "restart", nil)
			return err
		}},
		{"hassio.addon_start", access.SystemAddons, func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.CallService(ctx, "hassio", "addon_start", map[string]any{"addon": "core_ssh"})
			return err
		}},
		{"backup.create", access.SystemBackup, func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.CallServiceWithResponse(ctx, "backup", "create", nil)
			return err
		}},
		{"update.install", access.SystemUpdates, func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.CallService(ctx, "update", "install", map[string]any{"entity_id": "update.core"})
			return err
		}},
		{"ClearStatistics", access.SystemStatistics, func(ctx context.Context, c homeassistant.Client) error {
			return c.ClearStatistics(ctx, []string{"sensor.temp"})
		}},
		{"ClearSystemLog", access.SystemLogs, func(ctx context.Context, c homeassistant.Client) error {
			return c.ClearSystemLog(ctx)
		}},
		{"DeleteConfigEntry", access.SystemConfigEntries, func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.DeleteConfigEntry(ctx, "entry1")
			return err
		}},
		{"hacs download", access.SystemHACS, func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.SendHACSCommand(ctx, "hacs/repository/download", map[string]any{"repository": "1"})
			return err
		}},
		{"blueprint import", access.SystemBlueprints, func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.SendHACSCommand(ctx, "blueprint/import", map[string]any{"url": "https://x"})
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := buildWriteFake(true)
			c, tr, ctx := wrapFake(t, writePolicy(t), f)
			errText(t, tc.call(ctx, c), systemDenial(tc.value))
			noInnerWrite(t, f)
			if d := tr.First(); d == nil || d.System != tc.value || d.Object != "" {
				t.Errorf("denial = %+v", d)
			}

			other := buildWriteFake(true)
			c, _, ctx = wrapFake(t, writePolicy(t, "reload"), other)
			errText(t, tc.call(ctx, c), systemDenial(tc.value))

			f = buildWriteFake(true)
			c, _, ctx = wrapFake(t, writePolicy(t, string(tc.value)), f)
			if err := tc.call(ctx, c); err != nil {
				t.Fatalf("granted system value refused: %v", err)
			}
			if got := len(f.writeCalls()); got != 1 {
				t.Fatalf("inner calls = %d, want 1", got)
			}
		})
	}
}

func TestClearStatistics_HiddenIDsLookUnknown(t *testing.T) {
	f := buildWriteFake(true)
	c, _, ctx := wrapFake(t, writePolicy(t, "statistics"), f)
	if err := c.ClearStatistics(ctx, []string{"sensor.secret_temp", "recorder:total", "sensor.temp", " Sensor.Temp"}); err != nil {
		t.Fatal(err)
	}
	callOnce(t, f, "ClearStatistics", "", "", map[string]any{"ids": []string{"recorder:total", "sensor.temp"}})

	// Only hidden ids: HA ignores unknown ids, so the answer is success
	// without a call.
	f = buildWriteFake(true)
	c, tr, ctx := wrapFake(t, writePolicy(t, "statistics"), f)
	if err := c.ClearStatistics(ctx, []string{"sensor.secret_temp"}); err != nil {
		t.Fatal(err)
	}
	noInnerWrite(t, f)
	if tr.First() != nil {
		t.Errorf("hidden id recorded a denial: %+v", tr.First())
	}
}

func TestEntityWrites_CheckControl(t *testing.T) {
	tests := []struct {
		name   string
		method string
		call   func(context.Context, homeassistant.Client, string) error
	}{
		{"SetState", "SetState", func(ctx context.Context, c homeassistant.Client, id string) error {
			_, err := c.SetState(ctx, id, homeassistant.StateUpdate{State: "on"})
			return err
		}},
		{"SetHelperValue", "SetHelperValue", func(ctx context.Context, c homeassistant.Client, id string) error {
			return c.SetHelperValue(ctx, id, 3)
		}},
		{"ToggleAutomation", "ToggleAutomation", func(ctx context.Context, c homeassistant.Client, id string) error {
			return c.ToggleAutomation(ctx, id, false)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok := map[string]string{"SetState": "light.kitchen", "SetHelperValue": "input_number.ok", "ToggleAutomation": "automation.heating"}[tc.method]
			hidden := map[string]string{"SetState": "light.secret", "SetHelperValue": "input_number.secret_num", "ToggleAutomation": "automation.secret_auto"}[tc.method]

			f := buildWriteFake(true)
			c, _, ctx := wrapFake(t, writePolicy(t), f)
			if err := tc.call(ctx, c, ok); err != nil {
				t.Fatalf("allowed write refused: %v", err)
			}
			if got := f.writeCalls(); len(got) != 1 || got[0].method != tc.method || got[0].data["entity_id"] != ok {
				t.Fatalf("inner writes = %+v", got)
			}

			f = buildWriteFake(true)
			c, tr, ctx := wrapFake(t, writePolicy(t), f)
			errText(t, tc.call(ctx, c, "switch.ro"), denialFor("switch.ro", "control"))
			errText(t, tc.call(ctx, c, hidden), homeassistant.NewEntityNotFoundError(hidden).Error())
			errText(t, tc.call(ctx, c, "light.nothere"), homeassistant.NewEntityNotFoundError("light.nothere").Error())
			noInnerWrite(t, f)
			if d := tr.First(); d == nil || d.Object != "switch.ro" {
				t.Errorf("denial = %+v", d)
			}
		})
	}
}

func TestSendHACSCommand_UnknownDenied(t *testing.T) {
	for _, cmd := range []string{"config/auth/create", "HACS/info", "", "call_service"} {
		f := buildWriteFake(true)
		c, tr, ctx := wrapFake(t, mustCompile(t, config.AccessConfig{Default: "write", System: &config.CapList{"all"},
			Rules: []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"switch.ro"}}, Deny: config.CapList{"control"}}}}), f)
		_, err := c.SendHACSCommand(ctx, cmd, map[string]any{})
		errText(t, err, genericDenial())
		noInnerWrite(t, f)
		if tr.First() == nil {
			t.Errorf("%q: refusal not recorded", cmd)
		}
	}
}

func TestSendHACSCommand_ReadsPass(t *testing.T) {
	f := buildWriteFake(true)
	c, _, ctx := wrapFake(t, writePolicy(t), f)
	data := map[string]any{"repository": "1"}
	if _, err := c.SendHACSCommand(ctx, "hacs/repository/info", data); err != nil {
		t.Fatal(err)
	}
	callOnce(t, f, "SendHACSCommand", "hacs/repository/info", "", data)
}

func TestTrace_ResolvesConfigIDThroughRegistry(t *testing.T) {
	setNonces(t, "feedbeef")
	tests := []struct {
		name    string
		command string
		data    map[string]any
	}{
		{"list visible", "trace/list", map[string]any{"domain": "automation", "item_id": "cfg_heating"}},
		{"get visible", "trace/get", map[string]any{"domain": "automation", "item_id": "cfg_heating", "run_id": "r1"}},
		{"script visible", "trace/list", map[string]any{"domain": "script", "item_id": "ok"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := buildWriteFake(true)
			f.hacsFn = f.traceHA
			c, _, ctx := wrapFake(t, writePolicy(t), f)
			got, err := c.SendHACSCommand(ctx, tc.command, tc.data)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := f.traceHA(tc.command, tc.data)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("result = %v, want %v", got, want)
			}
			callOnce(t, f, "SendHACSCommand", tc.command, "", tc.data)
		})
	}
}

func TestTrace_HiddenLooksMissing(t *testing.T) {
	cases := []struct {
		name            string
		command         string
		hidden, missing map[string]any
	}{
		{"list hidden automation", "trace/list",
			map[string]any{"domain": "automation", "item_id": "cfg_secret"},
			map[string]any{"domain": "automation", "item_id": "cfg_nothere"}},
		{"get hidden automation", "trace/get",
			map[string]any{"domain": "automation", "item_id": "cfg_secret", "run_id": "r1"},
			map[string]any{"domain": "automation", "item_id": "cfg_nothere", "run_id": "r1"}},
		// The entity slug is not the config id: "heating" names no config.
		{"slug is not the config id", "trace/get",
			map[string]any{"domain": "automation", "item_id": "secret_auto", "run_id": "r1"},
			map[string]any{"domain": "automation", "item_id": "heating", "run_id": "r1"}},
		{"hidden script", "trace/get",
			map[string]any{"domain": "script", "item_id": "secret_script", "run_id": "r1"},
			map[string]any{"domain": "script", "item_id": "nothere", "run_id": "r1"}},
		{"padded script id", "trace/list",
			map[string]any{"domain": "script", "item_id": " secret_script"},
			map[string]any{"domain": "script", "item_id": " nothere"}},
		{"release notes", "update/release_notes",
			map[string]any{"entity_id": "update.secret_up"},
			map[string]any{"entity_id": "update.nothere"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setNonces(t, "feedbeef", "feedbeef")
			f := buildWriteFake(true)
			f.hacsFn = f.traceHA
			c, tr, ctx := wrapFake(t, writePolicy(t), f)
			gotHidden, errHidden := c.SendHACSCommand(ctx, tc.command, tc.hidden)
			absent := buildWriteFake(false)
			absent.hacsFn = absent.traceHA
			wantMissing, errMissing := absent.traceHA(tc.command, tc.missing)
			if (errHidden == nil) != (errMissing == nil) {
				t.Fatalf("hidden err %v, missing err %v", errHidden, errMissing)
			}
			if errHidden != nil {
				sameError(t, errHidden, errMissing)
			} else if !reflect.DeepEqual(gotHidden, wantMissing) {
				t.Errorf("hidden answer %v, missing answer %v", gotHidden, wantMissing)
			}
			for _, w := range f.writeCalls() {
				for _, v := range w.data {
					if s, _ := v.(string); strings.Contains(s, "secret") {
						t.Errorf("hidden id reached the inner client: %+v", w)
					}
				}
			}
			if tr.First() != nil {
				t.Errorf("hidden trace recorded a denial: %+v", tr.First())
			}
		})
	}
}

func TestTrace_DomainAndListing(t *testing.T) {
	refused := []map[string]any{
		{"item_id": "cfg_heating"},
		{"domain": "light", "item_id": "x"},
		{"domain": 5, "item_id": "x"},
		{"domain": "automation", "item_id": 5},
		{"domain": "automation"}, // domain-wide listing under a hiding policy
	}
	for _, data := range refused {
		f := buildWriteFake(true)
		c, _, ctx := wrapFake(t, writePolicy(t), f)
		_, err := c.SendHACSCommand(ctx, "trace/list", data)
		errText(t, err, genericDenial())
		noInnerWrite(t, f)
	}

	// Without hiding, a domain-wide listing passes.
	f := buildWriteFake(true)
	c, _, ctx := wrapFake(t, readPolicy(t), f)
	data := map[string]any{"domain": "automation"}
	if _, err := c.SendHACSCommand(ctx, "trace/list", data); err != nil {
		t.Fatal(err)
	}
	callOnce(t, f, "SendHACSCommand", "trace/list", "", data)

	// A hiding rule outside the domain keeps the domain-wide listing allowed.
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"light.secret"}}, Allow: config.CapList{"none"}}},
	})
	f = buildWriteFake(true)
	c, _, ctx = wrapFake(t, p, f)
	if _, err := c.SendHACSCommand(ctx, "trace/list", data); err != nil {
		t.Fatal(err)
	}
}

func TestResponseServices_AreReads(t *testing.T) {
	for _, tc := range []struct {
		domain, service string
		data            map[string]any
	}{
		{"todo", "get_items", map[string]any{"entity_id": "todo.list"}},
		{"weather", "get_forecasts", map[string]any{"entity_id": "weather.home", "type": "daily"}},
	} {
		f := buildWriteFake(true)
		c, tr, ctx := wrapFake(t, readPolicy(t), f)
		resp, err := c.CallServiceWithResponse(ctx, tc.domain, tc.service, tc.data)
		if err != nil {
			t.Fatalf("%s.%s under read-only: %v", tc.domain, tc.service, err)
		}
		if resp["response"] != "of "+tc.domain+"."+tc.service {
			t.Errorf("response = %v", resp)
		}
		callOnce(t, f, "CallServiceWithResponse", tc.domain, tc.service, tc.data)
		if tr.First() != nil {
			t.Errorf("denial on a read: %+v", tr.First())
		}
	}

	// A control service through the response variant is still control.
	f := buildWriteFake(true)
	c, _, ctx := wrapFake(t, readPolicy(t), f)
	_, err := c.CallServiceWithResponse(ctx, "todo", "add_item", map[string]any{"entity_id": "todo.list", "item": "x"})
	errText(t, err, denialFor("todo.list", "control"))
	noInnerWrite(t, f)
}

func TestMethodClass_WritesAreClassified(t *testing.T) {
	for _, m := range []string{
		"CallService", "CallServiceWithResponse", "SetState", "SetHelperValue", "ToggleAutomation",
		"ClearStatistics", "ClearSystemLog", "DeleteConfigEntry", "SendHACSCommand",
	} {
		if methodClass[m] == classUnclassified {
			t.Errorf("%s is still unclassified", m)
		}
	}
}
