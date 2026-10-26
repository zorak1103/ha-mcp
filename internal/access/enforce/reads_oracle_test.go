package enforce

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Padded or mixed-case ids are what HA's own validation treats differently
// from clean ones (invalid_format versus not_found), so under a hiding policy
// they are answered as missing and never forwarded, whether or not the clean
// form is hidden or even exists.
func TestPaddedAndMixedCaseIDs_AnswerMissingAndAreNeverForwarded(t *testing.T) {
	odd := []string{"light.secret ", " light.secret", "LIGHT.SECRET", "light.gone ", "Light.Gone", "light.kitchen ", "Light.Kitchen"}
	calls := map[string]func(ctx context.Context, c homeassistant.Client, id string) error{
		"GetState": func(ctx context.Context, c homeassistant.Client, id string) error {
			_, err := c.GetState(ctx, id)
			return err
		},
		"GetEntityRegistryEntry": func(ctx context.Context, c homeassistant.Client, id string) error {
			_, err := c.GetEntityRegistryEntry(ctx, id)
			return err
		},
		"GetCalendarEvents": func(ctx context.Context, c homeassistant.Client, id string) error {
			_, err := c.GetCalendarEvents(ctx, id, "a", "b")
			return err
		},
		"GetCameraSnapshot": func(ctx context.Context, c homeassistant.Client, id string) error {
			_, _, err := c.GetCameraSnapshot(ctx, id)
			return err
		},
		"GetCameraStream": func(ctx context.Context, c homeassistant.Client, id string) error {
			_, err := c.GetCameraStream(ctx, id)
			return err
		},
		"GetAutomation": func(ctx context.Context, c homeassistant.Client, id string) error {
			_, err := c.GetAutomation(ctx, "automation."+strings.TrimPrefix(id, "light."))
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			full := buildFake(true)
			w, tr, ctx := wrapFake(t, hidingPolicy(t), full)
			var texts []string
			for _, id := range odd {
				err := call(ctx, w, id)
				if err == nil {
					t.Fatalf("%q: must fail", id)
				}
				texts = append(texts, strings.ReplaceAll(err.Error(), strings.TrimSpace(id), "ID"))
			}
			for i, tx := range texts {
				if tx != texts[0] && name != "GetState" {
					t.Errorf("%q answers %q, differs from %q", odd[i], tx, texts[0])
				}
			}
			for _, seen := range full.seen() {
				if slices.Contains(odd, seen) {
					t.Errorf("odd id %q reached the inner client", seen)
				}
			}
			if tr.First() != nil {
				t.Errorf("recorded a denial: %+v", tr.First())
			}
		})
	}
}

func TestPaddedIDs_HiddenAndMissingGiveIdenticalError(t *testing.T) {
	w, _, ctx := wrapFake(t, hidingPolicy(t), buildFake(true))
	_, errHidden := w.GetEntityRegistryEntry(ctx, "sensor.secret_temp ")
	_, errMissing := w.GetEntityRegistryEntry(ctx, "light.gone ")
	sameError(t, errHidden, errMissing)
	_, errHidden = w.GetState(ctx, "sensor.secret_temp ")
	_, errMissing = w.GetState(ctx, "light.gone ")
	if errHidden == nil || errMissing == nil || strings.ReplaceAll(errHidden.Error(), "sensor.secret_temp ", "X") != strings.ReplaceAll(errMissing.Error(), "light.gone ", "X") {
		t.Errorf("GetState: %v vs %v", errHidden, errMissing)
	}
}

func TestPaddedIDs_ListsAreDroppedAndRequestsFiltered(t *testing.T) {
	full := buildFake(true)
	full.logbook = []homeassistant.LogbookEntry{{Name: "a", EntityID: "light.kitchen"}}
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	if _, err := w.GetLogbook(ctx, "s", "e", []string{"light.kitchen", "light.kitchen ", "Light.Kitchen", "light.gone "}); err != nil {
		t.Fatal(err)
	}
	if got := full.logbookIDs[0]; len(got) != 1 || got[0] != "light.kitchen" {
		t.Errorf("inner got entity ids %v, want only the clean one", got)
	}
	if h, err := w.GetHistory(ctx, "light.kitchen ", time.Now(), time.Now()); err != nil || h != nil || len(full.historyIDs) != 0 {
		t.Errorf("padded history id: %v, %v, inner calls %v", h, err, full.historyIDs)
	}
}

func TestNonHidingPolicy_PaddedIDsPassThrough(t *testing.T) {
	full := buildFake(true)
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"light.*"}}, Allow: config.CapList{"control"}}},
	})
	w, _, ctx := wrapFake(t, p, full)
	_, _ = w.GetState(ctx, "light.kitchen ")
	_, _ = w.GetEntityRegistryEntry(ctx, "Light.Gone")
	if !slices.Contains(full.seen(), "light.kitchen ") || !slices.Contains(full.seen(), "Light.Gone") {
		t.Errorf("a policy that hides nothing must not alter ids: %v", full.seen())
	}
}

// Areas and entity-less devices are governed by the default alone: under
// default none they are hidden.
func defaultNonePolicy(t *testing.T) *config.AccessConfig {
	t.Helper()
	return &config.AccessConfig{
		Default: "none",
		Rules: []config.AccessRule{{
			Match: config.AccessMatch{Entities: []string{"light.kitchen", "sensor.temp"}},
			Allow: config.CapList{"read"},
		}},
	}
}

func TestAreaAndDeviceRegistry_FollowDefault(t *testing.T) {
	// default none: areas are hidden, entity-less devices too.
	cfg := defaultNonePolicy(t)
	w, _, ctx := wrapFake(t, mustCompile(t, *cfg), buildFake(true))
	areas, err := w.GetAreaRegistry(ctx)
	if err != nil || len(areas) != 0 {
		t.Errorf("default none: areas = %v, %v; want empty", areas, err)
	}
	devs, err := w.GetDeviceRegistry(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(devs, func(d homeassistant.DeviceRegistryEntry) string { return d.ID })
	slices.Sort(got)
	if want := []string{"dev_mixed", "dev_visible"}; !slices.Equal(got, want) {
		t.Errorf("default none: devices = %v, want %v", got, want)
	}

	// default read: areas unfiltered, entity-less device visible.
	w2, _, ctx2 := wrapFake(t, hidingPolicy(t), buildFake(true))
	if areas, err := w2.GetAreaRegistry(ctx2); err != nil || len(areas) != 2 {
		t.Errorf("default read: areas = %v, %v; want all", areas, err)
	}
	devs, _ = w2.GetDeviceRegistry(ctx2)
	if !slices.ContainsFunc(devs, func(d homeassistant.DeviceRegistryEntry) bool { return d.ID == "dev_empty" }) {
		t.Error("default read: entity-less device must stay visible")
	}
}

func TestExtractFromTarget_AreasAndEmptyDevicesFollowDefault(t *testing.T) {
	cfg := defaultNonePolicy(t)
	full := buildFake(true)
	full.extractFn = full.echoExtract
	w, _, ctx := wrapFake(t, mustCompile(t, *cfg), full)

	absent := buildFake(false)
	absent.extractFn = absent.echoExtract
	other := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"zzz.*"}}, Allow: config.CapList{"none"}}},
	})
	// An area and a device that do not exist: the missing answer.
	wAbsent, _, ctxAbsent := wrapFake(t, other, absent)
	absent.areas, absent.devices = nil, nil

	in := homeassistant.Target{AreaID: []string{"kitchen"}, DeviceID: []string{"dev_empty"}}
	got, err := w.ExtractFromTarget(ctx, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := wAbsent.ExtractFromTarget(ctxAbsent, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.MissingAreas, want.MissingAreas) || !slices.Equal(got.MissingDevices, want.MissingDevices) ||
		len(got.ReferencedAreas) != 0 || len(got.ReferencedDevices) != 0 {
		t.Errorf("hidden area/device differ from missing\n got %+v\nwant %+v", got, want)
	}
	if !slices.Equal(got.MissingAreas, []string{"kitchen"}) || !slices.Equal(got.MissingDevices, []string{"dev_empty"}) {
		t.Errorf("hidden ids must be reported missing: %+v", got)
	}
}

// A config id without a registry entry cannot be mapped to its entity (the
// automation or scene entity id comes from the alias), so under a hiding policy
// it is treated as hidden. Scripts keep script.<key>: the key is the entity id.
func TestUnregisteredConfigIDs_HiddenUnderHidingPolicy(t *testing.T) {
	full := buildFake(true)
	full.states = append(full.states, homeassistant.Entity{EntityID: "scene.fresh"}, homeassistant.Entity{EntityID: "script.fresh"})
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)

	if _, err := w.GetScene(ctx, "fresh"); err == nil || err.Error() != errSceneMissing("fresh").Error() {
		t.Errorf("GetScene(unregistered) = %v, want the missing-scene error", err)
	}
	if ok, err := w.ConfigFileEntryExists(ctx, "automation", "fresh_cfg"); err != nil || ok {
		t.Errorf("automation unregistered = %v, %v; want false, nil", ok, err)
	}
	if ok, err := w.ConfigFileEntryExists(ctx, "scene", "fresh"); err != nil || ok {
		t.Errorf("scene unregistered = %v, %v; want false, nil", ok, err)
	}
	for _, id := range full.seen() {
		if id == "fresh" || id == "automation/fresh_cfg" || id == "scene/fresh" {
			t.Errorf("unregistered config id %q reached the inner client", id)
		}
	}
	// Scripts: fallback script.<key>, visible here, so the call goes through.
	if _, err := w.ConfigFileEntryExists(ctx, "script", "fresh"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(full.seen(), "script/fresh") {
		t.Error("an unregistered script with a visible fallback entity must be forwarded")
	}
}

func TestUnregisteredConfigIDs_FallbackKeptWithoutHiding(t *testing.T) {
	full := buildFake(true)
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"light.*"}}, Allow: config.CapList{"control"}}},
	})
	w, _, ctx := wrapFake(t, p, full)
	_, _ = w.GetScene(ctx, "fresh")
	_, _ = w.ConfigFileEntryExists(ctx, "automation", "fresh_cfg")
	if !slices.Contains(full.seen(), "fresh") || !slices.Contains(full.seen(), "automation/fresh_cfg") {
		t.Errorf("without hiding, unregistered ids pass through: %v", full.seen())
	}
}
