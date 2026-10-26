package enforce

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// hidingPolicy hides every entity whose id contains ".secret" and the
// dashboard "secret-dash"; everything else is read-only.
func hidingPolicy(t *testing.T) *access.Policy {
	t.Helper()
	return mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{"*.secret*"}}, Allow: config.CapList{"none"}},
			{Match: config.AccessMatch{Dashboards: []string{"secret-dash"}}, Allow: config.CapList{"none"}},
		},
	})
}

// buildFake returns the shared fixture. Without secrets it is the same home
// minus every hidden object, i.e. the inner client "with the id absent".
func buildFake(withSecret bool) *readFake {
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
	add("light.kitchen", "demo", "u1", func(e *homeassistant.EntityRegistryEntry) {
		e.AreaID, e.DeviceID, e.Labels = "kitchen", "dev_mixed", []string{"lbl_x"}
	})
	add("light.secret", "demo", "u2", func(e *homeassistant.EntityRegistryEntry) {
		e.AreaID, e.DeviceID, e.Labels = "kitchen", "dev_mixed", []string{"lbl_x"}
	})
	add("sensor.temp", "demo", "u3", func(e *homeassistant.EntityRegistryEntry) { e.DeviceID = "dev_visible" })
	add("sensor.secret_temp", "demo", "u4", func(e *homeassistant.EntityRegistryEntry) { e.DeviceID = "dev_hidden" })
	add("camera.front", "demo", "u5", nil)
	add("camera.secret", "demo", "u6", func(e *homeassistant.EntityRegistryEntry) { e.DeviceID = "dev_hidden" })
	add("automation.heating", "automation", "cfg_heating", nil)
	add("automation.secret_auto", "automation", "cfg_secret", nil)
	add("script.ok", "script", "ok", nil)
	add("script.secret_script", "script", "secret_script", nil)
	add("scene.ok", "homeassistant", "scene_ok_cfg", nil)
	add("scene.secret_scene", "homeassistant", "scene_secret_cfg", nil)
	add("input_number.ok", "input_number", "ok", nil)
	add("input_number.secret_num", "input_number", "secret_num", nil)
	add("image.pic", "demo", "u9", nil)
	add("image.secret_pic", "demo", "u10", nil)
	add("calendar.ok", "demo", "u7", nil)
	add("calendar.secret", "demo", "u8", nil)

	for _, e := range f.states {
		switch {
		case strings.HasPrefix(e.EntityID, "automation."):
			f.automations = append(f.automations, homeassistant.Automation{EntityID: e.EntityID})
		case strings.HasPrefix(e.EntityID, "calendar."):
			f.calendars = append(f.calendars, homeassistant.CalendarEntry{EntityID: e.EntityID})
		}
	}
	f.devices = []homeassistant.DeviceRegistryEntry{
		{ID: "dev_visible"}, {ID: "dev_mixed"}, {ID: "dev_empty"},
	}
	if withSecret {
		f.devices = append(f.devices, homeassistant.DeviceRegistryEntry{ID: "dev_hidden"})
	}
	f.areas = []homeassistant.AreaRegistryEntry{
		{AreaID: "kitchen", Name: "Kitchen"},
		{AreaID: "labeled_area", Name: "Labeled", Labels: []string{"lbl_area"}},
	}
	f.dashboards = []homeassistant.DashboardEntry{{ID: "dash_ok", URLPath: "ok-dash", Title: "ok"}}
	if withSecret {
		f.dashboards = append(f.dashboards, homeassistant.DashboardEntry{ID: "dash_secret", URLPath: "secret-dash", Title: "s"})
	}
	return f
}

func wrapFake(t *testing.T, p *access.Policy, f *readFake) (*client, *access.Tracker, context.Context) {
	t.Helper()
	ctx, tr := access.WithTracker(context.Background(), "some_tool")
	w, ok := Wrap(p, f, tr).(*client)
	if !ok {
		t.Fatal("Wrap did not return the enforcing client")
	}
	return w, tr, ctx
}

func sameError(t *testing.T, got, want error) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("both must be errors: got %v, want %v", got, want)
	}
	if got.Error() != want.Error() {
		t.Errorf("error text\n got %q\nwant %q", got.Error(), want.Error())
	}
	if fmt.Sprintf("%T", got) != fmt.Sprintf("%T", want) {
		t.Errorf("error type got %T, want %T", got, want)
	}
	if fmt.Sprintf("%T", errors.Unwrap(got)) != fmt.Sprintf("%T", errors.Unwrap(want)) {
		t.Errorf("wrapped error type got %T, want %T", errors.Unwrap(got), errors.Unwrap(want))
	}
	var gotAPI, wantAPI *homeassistant.APIError
	gotIsAPI, wantIsAPI := errors.As(got, &gotAPI), errors.As(want, &wantAPI)
	if gotIsAPI != wantIsAPI || (gotIsAPI && gotAPI.StatusCode != wantAPI.StatusCode) {
		t.Errorf("APIError mismatch: got %v/%v, want %v/%v", gotIsAPI, gotAPI, wantIsAPI, wantAPI)
	}
	if errors.Is(got, homeassistant.ErrHelperNotFoundInStorage) != errors.Is(want, homeassistant.ErrHelperNotFoundInStorage) {
		t.Errorf("ErrHelperNotFoundInStorage mismatch: got %v, want %v", got, want)
	}
}

type lookupCall struct {
	name string
	call func(ctx context.Context, c homeassistant.Client) error
}

func hiddenLookups() []lookupCall {
	return []lookupCall{
		{"GetState", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetState(ctx, "light.secret")
			return err
		}},
		{"GetState upper case", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetState(ctx, "Light.Secret")
			return err
		}},
		{"GetEntityRegistryEntry", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetEntityRegistryEntry(ctx, "sensor.secret_temp")
			return err
		}},
		{"GetAutomation entity id", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetAutomation(ctx, "automation.secret_auto")
			return err
		}},
		{"GetAutomation bare id", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetAutomation(ctx, "secret_auto")
			return err
		}},
		{"GetScript entity id", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetScript(ctx, "script.secret_script")
			return err
		}},
		{"GetScript bare id", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetScript(ctx, "secret_script")
			return err
		}},
		{"GetScene by config id", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetScene(ctx, "scene_secret_cfg")
			return err
		}},
		{"GetHelperConfig", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetHelperConfig(ctx, "input_number", "input_number.secret_num")
			return err
		}},
		{"GetHelperConfig bare id", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetHelperConfig(ctx, "input_number", "secret_num")
			return err
		}},
		{"GetLovelaceConfig", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetLovelaceConfig(ctx, "secret-dash")
			return err
		}},
		{"GetCalendarEvents", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetCalendarEvents(ctx, "calendar.secret", "2026-01-01", "2026-01-02")
			return err
		}},
		{"GetCameraSnapshot", func(ctx context.Context, c homeassistant.Client) error {
			_, _, err := c.GetCameraSnapshot(ctx, "camera.secret")
			return err
		}},
		{"GetCameraStream", func(ctx context.Context, c homeassistant.Client) error {
			_, err := c.GetCameraStream(ctx, "camera.secret")
			return err
		}},
	}
}

func TestHiddenLookup_IndistinguishableFromMissing(t *testing.T) {
	for _, tc := range hiddenLookups() {
		t.Run(tc.name, func(t *testing.T) {
			full := buildFake(true)
			w, _, ctx := wrapFake(t, hidingPolicy(t), full)
			got := tc.call(ctx, w)
			want := tc.call(ctx, buildFake(false))
			sameError(t, got, want)
			for _, id := range full.seen() {
				if strings.Contains(id, "secret") {
					t.Errorf("hidden id %q reached the inner client", id)
				}
			}
		})
	}
}

func TestHiddenLookup_ConcreteShapes(t *testing.T) {
	w, _, ctx := wrapFake(t, hidingPolicy(t), buildFake(true))

	_, err := w.GetState(ctx, "light.secret")
	if err == nil || err.Error() != "entity not found: light.secret" {
		t.Errorf("GetState error = %v", err)
	}
	_, err = w.GetScene(ctx, "scene_secret_cfg")
	var apiErr *homeassistant.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound || apiErr.Message != "scene not found: scene_secret_cfg" {
		t.Errorf("GetScene error = %#v", err)
	}
	_, err = w.GetHelperConfig(ctx, "input_number", "input_number.secret_num")
	if !errors.Is(err, homeassistant.ErrHelperNotFoundInStorage) {
		t.Errorf("GetHelperConfig error must wrap ErrHelperNotFoundInStorage: %v", err)
	}
}

func TestVisibleLookup_PassesThrough(t *testing.T) {
	full := buildFake(true)
	w, tr, ctx := wrapFake(t, hidingPolicy(t), full)
	if e, err := w.GetState(ctx, "light.kitchen"); err != nil || e.EntityID != "light.kitchen" {
		t.Errorf("GetState visible = %v, %v", e, err)
	}
	if _, err := w.GetEntityRegistryEntry(ctx, "light.kitchen"); err != nil {
		t.Errorf("GetEntityRegistryEntry visible: %v", err)
	}
	if a, err := w.GetAutomation(ctx, "heating"); err != nil || a.EntityID != "automation.heating" {
		t.Errorf("GetAutomation visible = %v, %v", a, err)
	}
	if _, err := w.GetScript(ctx, "script.ok"); err != nil {
		t.Errorf("GetScript visible: %v", err)
	}
	if _, err := w.GetScene(ctx, "scene_ok_cfg"); err != nil {
		t.Errorf("GetScene visible: %v", err)
	}
	if _, err := w.GetHelperConfig(ctx, "input_number", "input_number.ok"); err != nil {
		t.Errorf("GetHelperConfig visible: %v", err)
	}
	if _, err := w.GetLovelaceConfig(ctx, "ok-dash"); err != nil {
		t.Errorf("GetLovelaceConfig visible: %v", err)
	}
	if _, err := w.GetCalendarEvents(ctx, "calendar.ok", "a", "b"); err != nil {
		t.Errorf("GetCalendarEvents visible: %v", err)
	}
	if _, _, err := w.GetCameraSnapshot(ctx, "camera.front"); err != nil {
		t.Errorf("GetCameraSnapshot visible: %v", err)
	}
	if _, err := w.GetCameraStream(ctx, "camera.front"); err != nil {
		t.Errorf("GetCameraStream visible: %v", err)
	}
	// A visible but absent id still gets the inner client's own error.
	if _, err := w.GetState(ctx, "light.ghost"); err == nil || err.Error() != "entity not found: light.ghost" {
		t.Errorf("absent visible id: %v", err)
	}
	if d := tr.First(); d != nil {
		t.Errorf("visible reads recorded a denial: %+v", d)
	}
}

func ids[T any](in []T, id func(T) string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, id(v))
	}
	return out
}

func assertNoSecret(t *testing.T, what string, got []string, wantAtLeast ...string) {
	t.Helper()
	for _, id := range got {
		if strings.Contains(id, "secret") {
			t.Errorf("%s leaked %q (all: %v)", what, id, got)
		}
	}
	for _, id := range wantAtLeast {
		if !slices.Contains(got, id) {
			t.Errorf("%s dropped visible %q (all: %v)", what, id, got)
		}
	}
}

func TestLists_DropHidden(t *testing.T) {
	full := buildFake(true)
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	entityID := func(e homeassistant.Entity) string { return e.EntityID }

	states, err := w.GetStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "GetStates", ids(states, entityID), "light.kitchen", "camera.front", "sensor.temp")

	reg, err := w.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "GetEntityRegistry", ids(reg, func(e homeassistant.EntityRegistryEntry) string { return e.EntityID }), "light.kitchen")

	autos, err := w.ListAutomations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "ListAutomations", ids(autos, func(a homeassistant.Automation) string { return a.EntityID }), "automation.heating")

	scripts, err := w.ListScripts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "ListScripts", ids(scripts, entityID), "script.ok")

	scenes, err := w.ListScenes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "ListScenes", ids(scenes, entityID), "scene.ok")

	helpers, err := w.ListHelpers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "ListHelpers", ids(helpers, entityID), "input_number.ok")

	cals, err := w.GetCalendars(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "GetCalendars", ids(cals, func(c homeassistant.CalendarEntry) string { return c.EntityID }), "calendar.ok")

	dashes, err := w.ListDashboards(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "ListDashboards", ids(dashes, func(d homeassistant.DashboardEntry) string { return d.URLPath }), "ok-dash")

	areas, err := w.GetAreaRegistry(ctx)
	if err != nil || len(areas) != len(full.areas) {
		t.Errorf("GetAreaRegistry must be unfiltered: %v, %v", areas, err)
	}
}

func TestLists_DoNotMutateInnerSlices(t *testing.T) {
	full := buildFake(true)
	before := slices.Clone(full.states)
	regBefore := slices.Clone(full.registry)
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	if _, err := w.GetStates(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := w.GetEntityRegistry(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, full.states) || !reflect.DeepEqual(regBefore, full.registry) {
		t.Error("a list method modified the inner client's shared slice")
	}
}

func TestGetEntityRegistry_AttributeHiding(t *testing.T) {
	// Hide by area: light.kitchen and light.secret sit in the kitchen.
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Areas: []string{"Kitchen"}}, Allow: config.CapList{"none"}}},
	})
	w, _, ctx := wrapFake(t, p, buildFake(true))
	states, err := w.GetStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(states, func(e homeassistant.Entity) string { return e.EntityID })
	if slices.Contains(got, "light.kitchen") || slices.Contains(got, "light.secret") || !slices.Contains(got, "sensor.temp") {
		t.Errorf("area hiding not applied to GetStates: %v", got)
	}
	if _, err := w.GetState(ctx, "light.kitchen"); err == nil || err.Error() != "entity not found: light.kitchen" {
		t.Errorf("area-hidden GetState = %v", err)
	}
}

func TestDeviceRegistry_HidesOnlyFullyHiddenDevices(t *testing.T) {
	w, _, ctx := wrapFake(t, hidingPolicy(t), buildFake(true))
	devs, err := w.GetDeviceRegistry(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(devs, func(d homeassistant.DeviceRegistryEntry) string { return d.ID })
	slices.Sort(got)
	// dev_hidden: all entities hidden; dev_mixed: one visible entity; dev_empty has no entities.
	if want := []string{"dev_empty", "dev_mixed", "dev_visible"}; !reflect.DeepEqual(got, want) {
		t.Errorf("devices = %v, want %v", got, want)
	}
}

func TestLogbook_FiltersContextEntity(t *testing.T) {
	full := buildFake(true)
	full.logbook = []homeassistant.LogbookEntry{
		{Name: "a", EntityID: "light.kitchen"},
		{Name: "b", EntityID: "light.secret"},
		{Name: "c", EntityID: "light.kitchen", ContextEntityID: "light.secret"},
		{Name: "d"},
		{Name: "e", EntityID: "sensor.temp", ContextEntityID: "automation.heating"},
		{Name: "f", EntityID: "light.kitchen", ContextEntityID: "not an entity"},
	}
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	got, err := w.GetLogbook(ctx, "2026-01-01", "2026-01-02", nil)
	if err != nil {
		t.Fatal(err)
	}
	names := ids(got, func(e homeassistant.LogbookEntry) string { return e.Name })
	if want := []string{"a", "d", "e"}; !reflect.DeepEqual(names, want) {
		t.Errorf("entries = %v, want %v", names, want)
	}
	if len(full.logbook) != 6 {
		t.Error("inner slice modified")
	}
}

func TestLogbook_RequestedIDsAreFiltered(t *testing.T) {
	full := buildFake(true)
	full.logbook = []homeassistant.LogbookEntry{{Name: "a", EntityID: "light.kitchen"}}
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)

	if _, err := w.GetLogbook(ctx, "s", "e", []string{"light.kitchen", "light.secret", "Light.Secret "}); err != nil {
		t.Fatal(err)
	}
	if got := full.logbookIDs[0]; !reflect.DeepEqual(got, []string{"light.kitchen"}) {
		t.Errorf("inner got entity ids %v", got)
	}

	full.logbookIDs = nil
	got, err := w.GetLogbook(ctx, "s", "e", []string{"light.secret"})
	if err != nil {
		t.Fatalf("only hidden ids must give an empty result, not an error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("result = %#v, want empty non-nil", got)
	}
	if len(full.logbookIDs) != 0 {
		t.Error("inner was called although every requested id is hidden (an empty filter would mean all entities)")
	}
}

func TestHistoryAndStatistics_DropHidden(t *testing.T) {
	full := buildFake(true)
	full.history = [][]homeassistant.HistoryEntry{{{EntityID: "light.kitchen"}}, {{EntityID: "light.secret"}}}
	full.statistics = []homeassistant.StatisticsResult{
		{StatisticID: "sensor.temp"}, {StatisticID: "sensor.secret_temp"}, {StatisticID: "recorder:external"},
	}
	full.statMetas = []homeassistant.StatisticMeta{
		{StatisticID: "sensor.temp"}, {StatisticID: "sensor.secret_temp"},
	}
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	start, end := time.Now().Add(-time.Hour), time.Now()

	h, err := w.GetHistory(ctx, "light.secret", start, end)
	if err != nil || len(h) != 0 || len(full.historyIDs) != 0 {
		t.Errorf("hidden history = %v, %v (inner calls %v); a missing entity gives nil, nil", h, err, full.historyIDs)
	}
	h, err = w.GetHistory(ctx, "light.kitchen", start, end)
	if err != nil || len(h) != 1 || h[0][0].EntityID != "light.kitchen" {
		t.Errorf("visible history = %v, %v (result groups of hidden ids must be dropped)", h, err)
	}

	st, err := w.GetStatistics(ctx, []string{"sensor.temp", "sensor.secret_temp"}, "hour")
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "GetStatistics", ids(st, func(s homeassistant.StatisticsResult) string { return s.StatisticID }), "sensor.temp", "recorder:external")
	if got := full.statIDs[0]; !reflect.DeepEqual(got, []string{"sensor.temp"}) {
		t.Errorf("inner statistic ids = %v", got)
	}
	full.statIDs = nil
	if st, err = w.GetStatistics(ctx, []string{"sensor.secret_temp"}, "hour"); err != nil || len(st) != 0 || len(full.statIDs) != 0 {
		t.Errorf("only hidden statistic ids: %v, %v, inner calls %v", st, err, full.statIDs)
	}

	metas, err := w.ListStatisticIDs(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "ListStatisticIDs", ids(metas, func(m homeassistant.StatisticMeta) string { return m.StatisticID }), "sensor.temp")
}

func TestRenderTemplate_RefusedWhenHiding_AllowedOtherwise(t *testing.T) {
	full := buildFake(true)
	w, tr, ctx := wrapFake(t, hidingPolicy(t), full)
	out, err := w.RenderTemplate(ctx, "{{ states('light.secret') }}")
	if err == nil || out != "" {
		t.Fatalf("a hiding policy must refuse, got %q, %v", out, err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("refusal must not read as not found: %v", err)
	}
	d := tr.First()
	if d == nil || d.Object != "render_template" || d.Tool != "some_tool" {
		t.Errorf("denial = %+v, want a recorded render_template denial", d)
	}
	if full.called("RenderTemplate") {
		t.Error("refused template reached the inner client")
	}

	noHide := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"light.x"}}, Allow: config.CapList{"control"}}},
	})
	w2, tr2, ctx2 := wrapFake(t, noHide, buildFake(true))
	if out, err := w2.RenderTemplate(ctx2, "{{ 1 }}"); err != nil || out != "rendered" {
		t.Errorf("non-hiding policy: %q, %v", out, err)
	}
	if tr2.First() != nil {
		t.Error("allowed template recorded a denial")
	}
}

func TestSignPath_HiddenCameraLooksMissing(t *testing.T) {
	w, tr, ctx := wrapFake(t, hidingPolicy(t), buildFake(true))
	// The same home without the camera, under a policy that hides nothing
	// relevant: the inner client's own answer for a missing camera.
	other := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"zzz.*"}}, Allow: config.CapList{"none"}}},
	})
	wAbsent, _, ctxAbsent := wrapFake(t, other, buildFake(false))

	for _, p := range []string{
		"/api/camera_proxy/camera.secret",
		"/api/camera_proxy/camera.secret?token=x",
		"/api/camera_proxy/Camera.Secret",
		"/api/camera_proxy/camera%2Esecret",
	} {
		_, got := w.SignPath(ctx, p, 60)
		_, want := wAbsent.SignPath(ctxAbsent, p, 60)
		if got == nil || want == nil {
			t.Fatalf("%s: hidden and missing cameras must both be errors: %v / %v", p, got, want)
		}
		if got.Error() != want.Error() || fmt.Sprintf("%T", got) != fmt.Sprintf("%T", want) {
			t.Errorf("%s: hidden %q differs from missing %q", p, got, want)
		}
	}
	if tr.First() != nil {
		t.Errorf("hidden camera recorded a denial: %+v", tr.First())
	}
	if got, err := w.SignPath(ctx, "/api/camera_proxy/camera.front", 60); err != nil || got != "signed:/api/camera_proxy/camera.front" {
		t.Errorf("visible camera: %q, %v", got, err)
	}
}

func TestSignPath_MediaAllowedOthersRefused(t *testing.T) {
	full := buildFake(true)
	w, tr, ctx := wrapFake(t, hidingPolicy(t), full)
	if got, err := w.SignPath(ctx, "/media/local/song.mp3", 60); err != nil || got != "signed:/media/local/song.mp3" {
		t.Errorf("/media/ must be allowed: %q, %v", got, err)
	}
	if tr.First() != nil {
		t.Fatal("allowed path recorded a denial")
	}
	refused := []string{
		"/api/states",
		"/media/../api/camera_proxy/camera.secret",
		"/media/%2e%2e/api/camera_proxy/camera.secret",
		"/api/camera_proxy/camera.secret/../camera.front",
		"/api/camera_proxy/",
		"/api/camera_proxy/..",
		"/api/camera_proxy/camera.front/extra",
		"",
	}
	for _, p := range refused {
		if got, err := w.SignPath(ctx, p, 60); err == nil || got != "" {
			t.Errorf("%q must be refused, got %q, %v", p, got, err)
		}
	}
	for _, id := range full.seen() {
		if slices.Contains(refused, id) {
			t.Errorf("refused path %q reached the inner SignPath", id)
		}
	}
	d := tr.First()
	if d == nil || d.Object != "sign_path" {
		t.Errorf("refusal must be recorded as a denial, got %+v", d)
	}
}

func TestConfigFileEntryExists_HiddenIsFalse(t *testing.T) {
	full := buildFake(true)
	w, tr, ctx := wrapFake(t, hidingPolicy(t), full)
	tests := []struct {
		domain, id string
		want       bool
	}{
		{"automation", "cfg_heating", true},
		{"automation", "cfg_secret", false},
		{"script", "secret_script", false},
		{"script", "ok", true},
		{"scene", "scene_secret_cfg", false},
		{"scene", "scene_ok_cfg", true},
		{"automation", "unknown_cfg", false},
	}
	for _, tc := range tests {
		got, err := w.ConfigFileEntryExists(ctx, tc.domain, tc.id)
		if err != nil || got != tc.want {
			t.Errorf("%s/%s = %v, %v; want %v, nil", tc.domain, tc.id, got, err, tc.want)
		}
	}
	for _, id := range full.seen() {
		if strings.Contains(id, "secret") {
			t.Errorf("hidden config id %q reached the inner client", id)
		}
	}
	if tr.First() != nil {
		t.Error("hidden existence check recorded a denial")
	}
}

func TestConfigFileEntryExists_UnregisteredFallsBackToDomainDotID(t *testing.T) {
	// A config entry with no registry entry yet: its entity id is predicted
	// as <domain>.<config id>, which must still be checked against the policy.
	full := buildFake(true)
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	got, err := w.ConfigFileEntryExists(ctx, "script", "secret_new")
	if err != nil || got {
		t.Errorf("= %v, %v; want false, nil", got, err)
	}
	if slices.Contains(full.seen(), "script/secret_new") {
		t.Error("hidden id reached the inner client")
	}
}

func TestHiddenReads_NotRecordedAsDenials(t *testing.T) {
	w, tr, ctx := wrapFake(t, hidingPolicy(t), buildFake(true))
	for _, tc := range hiddenLookups() {
		_ = tc.call(ctx, w)
	}
	_, _ = w.GetStates(ctx)
	_, _ = w.GetEntityRegistry(ctx)
	_, _ = w.GetDeviceRegistry(ctx)
	_, _ = w.ListDashboards(ctx)
	_, _ = w.GetLogbook(ctx, "s", "e", []string{"light.secret"})
	_, _ = w.GetHistory(ctx, "light.secret", time.Now(), time.Now())
	_, _ = w.GetStatistics(ctx, []string{"sensor.secret_temp"}, "hour")
	_, _ = w.ConfigFileEntryExists(ctx, "automation", "cfg_secret")
	_, _ = w.SignPath(ctx, "/api/camera_proxy/camera.secret", 60)
	_, _ = w.GetTriggersForTarget(ctx, homeassistant.Target{EntityID: []string{"light.secret"}}, nil)
	if d := tr.First(); d != nil {
		t.Errorf("hidden reads must never be recorded as denials, got %+v", d)
	}
}

func TestBrowseMedia_DropsHiddenCameras(t *testing.T) {
	full := buildFake(true)
	full.media = &homeassistant.MediaBrowseResult{
		Title: "root", MediaContentID: "media-source://",
		Children: []*homeassistant.MediaBrowseResult{
			{Title: "Front", MediaContentID: "media-source://camera/camera.front"},
			{Title: "Secret", MediaContentID: "media-source://camera/camera.secret"},
			{Title: "Upper", MediaContentID: "Media-Source://Camera/Camera.Secret"},
			{Title: "Folder", MediaContentID: "media-source://camera", Children: []*homeassistant.MediaBrowseResult{
				{Title: "Nested secret", MediaContentID: "media-source://camera/camera.secret"},
				{Title: "Nested front", MediaContentID: "media-source://camera/camera.front"},
			}},
			{Title: "Local", MediaContentID: "media-source://media_source/local"},
			{Title: "Odd", MediaContentID: "media-source://camera/not an entity"},
			{Title: "Pic", MediaContentID: "media-source://image/image.pic"},
			{Title: "Secret pic", MediaContentID: "media-source://image/image.secret_pic"},
			{Title: "Secret pic object id", MediaContentID: "media-source://image/secret_pic"},
			{Title: "Images", MediaContentID: "media-source://image", Children: []*homeassistant.MediaBrowseResult{
				{Title: "Nested secret pic", MediaContentID: "media-source://image/image.secret_pic?x=1"},
				{Title: "Nested pic", MediaContentID: "media-source://image/image.pic"},
				{Title: "Deeper", MediaContentID: "media-source://image/dir", Children: []*homeassistant.MediaBrowseResult{
					{Title: "Deep secret", MediaContentID: "media-source://camera/camera.secret"},
					{Title: "Deep pic", MediaContentID: "media-source://image/image.pic"},
				}},
			}},
		},
	}
	w, tr, ctx := wrapFake(t, hidingPolicy(t), full)
	got, err := w.BrowseMedia(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	var walk func(*homeassistant.MediaBrowseResult)
	walk = func(n *homeassistant.MediaBrowseResult) {
		titles = append(titles, n.Title)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(got)
	want := []string{"root", "Front", "Folder", "Nested front", "Local", "Pic", "Images", "Nested pic", "Deeper", "Deep pic"}
	if !reflect.DeepEqual(titles, want) {
		t.Errorf("tree = %v, want %v", titles, want)
	}
	if len(full.media.Children) != 10 || len(full.media.Children[3].Children) != 2 || len(full.media.Children[9].Children) != 3 {
		t.Error("the shared tree returned by the inner client was mutated")
	}
	if tr.First() != nil {
		t.Error("filtering recorded a denial")
	}
}

func TestBrowseMedia_HiddenCameraItemLooksMissing(t *testing.T) {
	full := buildFake(true)
	w, tr, ctx := wrapFake(t, hidingPolicy(t), full)
	// HA's camera source answers "Unknown item" for any camera id.
	wantMsg := "browse media failed: command failed: browse_media_failed - Unknown item"
	for _, id := range []string{"media-source://camera/camera.secret", "media-source://image/image.secret_pic"} {
		res, err := w.BrowseMedia(ctx, id)
		if err == nil || res != nil {
			t.Fatalf("browsing %s must fail, got %v, %v", id, res, err)
		}
		if err.Error() != wantMsg {
			t.Errorf("%s: error = %q, want %q", id, err, wantMsg)
		}
	}
	if full.called("BrowseMedia") {
		t.Error("hidden item browse reached the inner client")
	}
	if tr.First() != nil {
		t.Error("recorded a denial")
	}
}

func targetFixture(t *testing.T) (*client, *readFake, context.Context) {
	t.Helper()
	full := buildFake(true)
	full.triggers = []string{"light.turn_on"}
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	return w, full, ctx
}

func TestTargets_FilterInputs(t *testing.T) {
	tests := []struct {
		name      string
		in        homeassistant.Target
		wantCalls []homeassistant.Target
	}{
		// A hidden explicit id is sent as an absent twin of the same domain, so
		// HA answers exactly as for a missing id.
		{"only hidden entity", homeassistant.Target{EntityID: []string{"light.secret"}},
			[]homeassistant.Target{{EntityID: []string{"light.ha_mcp_absent_0"}}}},
		{"hidden replaced in mixed list", homeassistant.Target{EntityID: []string{"light.kitchen", "Light.Secret"}},
			[]homeassistant.Target{{EntityID: []string{"light.kitchen", "light.ha_mcp_absent_0"}}}},
		{"padded visible id is answered as missing too", homeassistant.Target{EntityID: []string{"light.kitchen "}},
			[]homeassistant.Target{{EntityID: []string{"light.ha_mcp_absent_0"}}}},
		{"area expands to visible entities only", homeassistant.Target{AreaID: []string{"kitchen"}},
			[]homeassistant.Target{{EntityID: []string{"light.kitchen"}}}},
		{"device whose entities are all hidden", homeassistant.Target{DeviceID: []string{"dev_hidden"}}, nil},
		{"device with a visible entity", homeassistant.Target{DeviceID: []string{"dev_mixed"}},
			[]homeassistant.Target{{EntityID: []string{"light.kitchen"}}}},
		{"label expands", homeassistant.Target{LabelID: []string{"lbl_x"}},
			[]homeassistant.Target{{EntityID: []string{"light.kitchen"}}}},
		{"label on an area", homeassistant.Target{LabelID: []string{"lbl_area"}}, nil},
		{"explicit plus area", homeassistant.Target{EntityID: []string{"sensor.temp"}, AreaID: []string{"kitchen"}},
			[]homeassistant.Target{{EntityID: []string{"light.kitchen", "sensor.temp"}}}},
		{"empty target passes through", homeassistant.Target{}, []homeassistant.Target{{}}},
	}
	methods := map[string]func(context.Context, homeassistant.Client, homeassistant.Target) ([]string, error){
		"triggers": func(ctx context.Context, c homeassistant.Client, tg homeassistant.Target) ([]string, error) {
			return c.GetTriggersForTarget(ctx, tg, nil)
		},
		"conditions": func(ctx context.Context, c homeassistant.Client, tg homeassistant.Target) ([]string, error) {
			return c.GetConditionsForTarget(ctx, tg, nil)
		},
		"services": func(ctx context.Context, c homeassistant.Client, tg homeassistant.Target) ([]string, error) {
			return c.GetServicesForTarget(ctx, tg, nil)
		},
	}
	for mname, call := range methods {
		for _, tc := range tests {
			t.Run(mname+"/"+tc.name, func(t *testing.T) {
				w, full, ctx := targetFixture(t)
				got, err := call(ctx, w, tc.in)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(full.targets, tc.wantCalls) {
					t.Errorf("inner targets = %+v, want %+v", full.targets, tc.wantCalls)
				}
				if tc.wantCalls == nil && len(got) != 0 {
					t.Errorf("result for an all-hidden target = %v, want empty", got)
				}
				if tc.wantCalls != nil && !reflect.DeepEqual(got, []string{"light.turn_on"}) {
					t.Errorf("result = %v", got)
				}
			})
		}
	}
}

func TestExtractFromTarget_FiltersResults(t *testing.T) {
	full := buildFake(true)
	full.extract = &homeassistant.ExtractFromTargetResult{
		ReferencedEntities: []string{"light.kitchen", "light.secret", "sensor.secret_temp"},
		ReferencedDevices:  []string{"dev_hidden", "dev_mixed", "dev_visible", "dev_empty"},
		ReferencedAreas:    []string{"kitchen"},
		MissingDevices:     []string{"dev_gone"},
	}
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)

	got, err := w.ExtractFromTarget(ctx, homeassistant.Target{LabelID: []string{"lbl_x"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.ReferencedEntities, []string{"light.kitchen"}) {
		t.Errorf("entities = %v", got.ReferencedEntities)
	}
	if !reflect.DeepEqual(got.ReferencedDevices, []string{"dev_mixed", "dev_visible", "dev_empty"}) {
		t.Errorf("devices = %v", got.ReferencedDevices)
	}
	if !reflect.DeepEqual(got.ReferencedAreas, []string{"kitchen"}) || !reflect.DeepEqual(got.MissingDevices, []string{"dev_gone"}) {
		t.Errorf("areas/missing changed: %+v", got)
	}
	if len(full.extract.ReferencedEntities) != 3 {
		t.Error("the inner result was mutated")
	}
}

// A hidden explicit entity, device or area must be answered exactly as HA
// answers for a missing one: the hidden id is swapped for an absent twin in the
// request and swapped back in the result.
func TestExtractFromTarget_HiddenExplicitIDsLookMissing(t *testing.T) {
	full := buildFake(true)
	full.extractFn = full.echoExtract
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)

	absent := buildFake(false)
	absent.extractFn = absent.echoExtract
	other := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"zzz.*"}}, Allow: config.CapList{"none"}}},
	})
	wAbsent, _, ctxAbsent := wrapFake(t, other, absent)

	in := homeassistant.Target{
		EntityID: []string{"light.kitchen", "light.secret", "sensor.secret_temp"},
		DeviceID: []string{"dev_hidden", "dev_visible"},
	}
	got, err := w.ExtractFromTarget(ctx, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := wAbsent.ExtractFromTarget(ctxAbsent, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("hidden answer differs from the missing answer\n got %+v\nwant %+v", got, want)
	}
	if !reflect.DeepEqual(got.MissingDevices, []string{"dev_hidden"}) || !slices.Contains(got.ReferencedEntities, "light.secret") {
		t.Errorf("hidden ids must be reported like missing ones: %+v", got)
	}
	for _, tg := range full.targets {
		for _, id := range append(slices.Clone(tg.EntityID), tg.DeviceID...) {
			if strings.Contains(id, "secret") || id == "dev_hidden" {
				t.Errorf("hidden id %q reached the inner client", id)
			}
		}
	}
}

func TestExtractFromTarget_AreaExpansionDropsHiddenWithoutEcho(t *testing.T) {
	full := buildFake(true)
	full.extractFn = full.echoExtract
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	got, err := w.ExtractFromTarget(ctx, homeassistant.Target{AreaID: []string{"kitchen"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// light.secret came from the area, not from the caller: dropped, not echoed.
	if !reflect.DeepEqual(got.ReferencedEntities, []string{"light.kitchen"}) || !reflect.DeepEqual(got.ReferencedAreas, []string{"kitchen"}) {
		t.Errorf("result = %+v", got)
	}
}

func TestExtractFromTarget_AllHiddenIsAMissingAnswer(t *testing.T) {
	full := buildFake(true)
	full.extractFn = full.echoExtract
	w, _, ctx := wrapFake(t, hidingPolicy(t), full)
	got, err := w.ExtractFromTarget(ctx, homeassistant.Target{EntityID: []string{"light.secret"}}, nil)
	if err != nil || got == nil {
		t.Fatalf("%+v, %v", got, err)
	}
	if !reflect.DeepEqual(got.ReferencedEntities, []string{"light.secret"}) {
		t.Errorf("a hidden id is reported like a missing id: %+v", got)
	}
	if len(full.targets) != 1 || !reflect.DeepEqual(full.targets[0].EntityID, []string{"light.ha_mcp_absent_0"}) {
		t.Errorf("inner must be asked about an absent twin: %+v", full.targets)
	}
}

func TestReads_SnapshotFailureFailsClosed(t *testing.T) {
	full := buildFake(true)
	full.registryFn = func() error { return errors.New("boom: secret registry detail") }
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Areas: []string{"kitchen"}}, Allow: config.CapList{"none"}}},
	})
	w, tr, ctx := wrapFake(t, p, full)

	check := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: must fail closed", name)
			return
		}
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "snapshot") || strings.Contains(msg, "boom") || strings.Contains(msg, "not found") {
			t.Errorf("%s: error leaks internals or reads as not found: %q", name, err)
		}
	}
	_, err := w.GetState(ctx, "light.kitchen")
	check("GetState", err)
	_, err = w.GetStates(ctx)
	check("GetStates", err)
	_, err = w.GetLogbook(ctx, "s", "e", []string{"light.kitchen"})
	check("GetLogbook", err)
	_, err = w.ListAutomations(ctx)
	check("ListAutomations", err)
	_, err = w.GetScene(ctx, "scene_ok_cfg")
	check("GetScene", err)
	if tr.First() == nil {
		t.Error("an undecidable read must be recorded as a denial")
	}
}

func TestReads_NoHidingPolicyDoesNotTouchRegistry(t *testing.T) {
	full := buildFake(true)
	full.registryFn = func() error { return errors.New("registry must not be fetched") }
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules:   []config.AccessRule{{Match: config.AccessMatch{Entities: []string{"light.*"}}, Allow: config.CapList{"control"}}},
	})
	w, tr, ctx := wrapFake(t, p, full)
	if _, err := w.GetState(ctx, "light.secret"); err != nil {
		t.Errorf("without hiding rules every existing object is readable: %v", err)
	}
	if s, err := w.GetStates(ctx); err != nil || len(s) != len(full.states) {
		t.Errorf("GetStates = %d, %v", len(s), err)
	}
	if _, err := w.GetScene(ctx, "scene_secret_cfg"); err != nil {
		t.Errorf("GetScene: %v", err)
	}
	if tr.First() != nil {
		t.Error("recorded a denial")
	}
}

func TestMethodClass_ReadsAreClassified(t *testing.T) {
	for _, name := range []string{
		"GetStates", "GetState", "GetHistory", "ListAutomations", "GetAutomation", "ListHelpers", "ListScripts",
		"GetScript", "ListScenes", "GetScene", "ConfigFileEntryExists", "GetEntityRegistry", "GetEntityRegistryEntry",
		"GetDeviceRegistry", "GetAreaRegistry", "SignPath", "GetCameraStream", "BrowseMedia", "GetLovelaceConfig",
		"ListDashboards", "GetStatistics", "GetTriggersForTarget", "GetConditionsForTarget", "GetServicesForTarget",
		"ExtractFromTarget", "GetHelperConfig", "RenderTemplate", "GetLogbook", "ListStatisticIDs", "GetCalendars",
		"GetCalendarEvents", "GetCameraSnapshot",
	} {
		if methodClass[name] == classUnclassified {
			t.Errorf("%s still unclassified", name)
		}
	}
}
