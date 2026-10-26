package enforce

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// panicInner is an inner client whose every method panics: embedding the
// nil interface makes any call that reaches it fail the test loudly.
type panicInner struct{ homeassistant.Client }

// regInner serves the registries the snapshot reads and counts the fetches.
type regInner struct {
	homeassistant.Client

	entityCalls atomic.Int32
	failFirst   atomic.Bool
	entities    []homeassistant.EntityRegistryEntry
	devices     []homeassistant.DeviceRegistryEntry
	areas       []homeassistant.AreaRegistryEntry
	labels      []homeassistant.LabelRegistryEntry
	floors      []homeassistant.FloorRegistryEntry
	dashboards  []homeassistant.DashboardEntry
	invalidated atomic.Int32
}

func (r *regInner) GetEntityRegistry(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	r.entityCalls.Add(1)
	if r.failFirst.CompareAndSwap(true, false) {
		return nil, errors.New("boom")
	}
	return r.entities, nil
}

func (r *regInner) GetDeviceRegistry(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	return r.devices, nil
}

func (r *regInner) GetAreaRegistry(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
	return r.areas, nil
}

func (r *regInner) GetLabelRegistry(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
	return r.labels, nil
}

func (r *regInner) GetFloorRegistry(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
	return r.floors, nil
}

func (r *regInner) ListDashboards(context.Context) ([]homeassistant.DashboardEntry, error) {
	return r.dashboards, nil
}

func (r *regInner) InvalidateLabelRegistryCache() { r.invalidated.Add(1) }

func mustCompile(t *testing.T, cfg config.AccessConfig) *access.Policy {
	t.Helper()
	p, err := access.Compile(cfg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return p
}

func readPolicy(t *testing.T) *access.Policy {
	t.Helper()
	return mustCompile(t, config.AccessConfig{Default: "read"})
}

func TestEveryClientMethodClassified(t *testing.T) {
	typ := reflect.TypeOf((*homeassistant.Client)(nil)).Elem()
	want := make([]string, 0, typ.NumMethod())
	for i := range typ.NumMethod() {
		want = append(want, typ.Method(i).Name)
	}
	got := make([]string, 0, len(methodClass))
	for name := range methodClass {
		got = append(got, name)
	}
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("methodClass keys differ from the Client method set\nwant %v\n got %v", want, got)
	}
}

func TestNoEmbeddedFields(t *testing.T) {
	typ := reflect.TypeOf(client{})
	for i := range typ.NumField() {
		if typ.Field(i).Anonymous {
			t.Errorf("client embeds %s; every method must be written out", typ.Field(i).Name)
		}
	}
}

func TestWrap_FastPathReturnsInnerIdentity(t *testing.T) {
	inner := &panicInner{}
	unrestricted := mustCompile(t, config.AccessConfig{Default: "write"})
	if got := Wrap(unrestricted, inner, nil); got != homeassistant.Client(inner) {
		t.Error("unrestricted policy must return the inner client itself")
	}
	if got := Wrap(nil, inner, nil); got != homeassistant.Client(inner) {
		t.Error("nil policy must return the inner client itself")
	}
	if got := Wrap(readPolicy(t), inner, nil); got == homeassistant.Client(inner) {
		t.Error("restricted policy must wrap the inner client")
	}
}

func TestWrapped_DoesNotImplementClose(t *testing.T) {
	w := Wrap(readPolicy(t), &panicInner{}, nil)
	if _, ok := w.(homeassistant.ClientCloser); ok {
		t.Error("the wrapper must not implement Close: the factory closes the unwrapped client")
	}
}

func TestUnclassified_RecordsMethodNameWithoutRevealingIt(t *testing.T) {
	_, tr := access.WithTracker(context.Background(), "manage_x")
	c := &client{p: readPolicy(t), tr: tr}
	err := c.unclassified("GetStates")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(err.Error(), "GetStates") {
		t.Errorf("message leaks internals or says not found: %q", err.Error())
	}
	d := tr.First()
	if d == nil || d.Object != "GetStates" || d.Capability != "" {
		t.Errorf("denial = %+v, want Object GetStates and no capability", d)
	}
}

func TestFilter_DoesNotMutateInnerSlices(t *testing.T) {
	in := []int{1, 2, 3, 4, 5, 6}
	orig := append([]int(nil), in...)
	out := filterCopy(in, func(v int) bool { return v%2 == 0 })
	if !reflect.DeepEqual(out, []int{2, 4, 6}) {
		t.Fatalf("out = %v", out)
	}
	if !reflect.DeepEqual(in, orig) {
		t.Fatalf("input mutated: %v", in)
	}
	out[0] = 99
	if in[0] != 1 || in[1] != 2 {
		t.Fatalf("output shares the backing array with the input: %v", in)
	}
	if got := filterCopy([]int(nil), func(int) bool { return true }); len(got) != 0 {
		t.Errorf("nil input gave %v", got)
	}
}

func TestSnapshot_ConcurrentFirstUse(t *testing.T) {
	inner := &regInner{entities: []homeassistant.EntityRegistryEntry{{EntityID: "light.a"}}}
	s := &snapshot{inner: inner}
	const n = 8
	var wg sync.WaitGroup
	results := make([]*snapData, n)
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = s.get(context.Background())
		}()
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if results[i] != results[0] {
			t.Errorf("goroutine %d got a different snapshot", i)
		}
	}
	if got := inner.entityCalls.Load(); got != 1 {
		t.Errorf("entity registry fetched %d times, want 1", got)
	}
}

func TestSnapshot_FailureIsNotCached(t *testing.T) {
	inner := &regInner{entities: []homeassistant.EntityRegistryEntry{{EntityID: "light.a"}}}
	inner.failFirst.Store(true)
	s := &snapshot{inner: inner}
	if _, err := s.get(context.Background()); err == nil {
		t.Fatal("first fetch must fail")
	}
	d, err := s.get(context.Background())
	if err != nil || d == nil {
		t.Fatalf("retry must succeed, got %v", err)
	}
	if _, ok := d.entity("light.a"); !ok {
		t.Error("retry snapshot lacks the entity")
	}
}

func TestSnapshot_InvalidateRefetches(t *testing.T) {
	inner := &regInner{}
	s := &snapshot{inner: inner}
	for range 2 {
		if _, err := s.get(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if inner.entityCalls.Load() != 1 {
		t.Fatalf("cached fetch count = %d", inner.entityCalls.Load())
	}
	s.invalidate()
	if _, err := s.get(context.Background()); err != nil {
		t.Fatal(err)
	}
	if inner.entityCalls.Load() != 2 {
		t.Fatalf("fetch count after invalidate = %d, want 2", inner.entityCalls.Load())
	}
}

func testInner() *regInner {
	return &regInner{
		entities: []homeassistant.EntityRegistryEntry{
			{EntityID: "light.own_area", AreaID: "kitchen_id", DeviceID: "dev1", Labels: []string{"lbl_a"}},
			{EntityID: "light.inherits", DeviceID: "dev1"},
			{EntityID: "switch.other", AreaID: "garage_id"},
			{EntityID: "sensor.devless", Labels: []string{"lbl_b"}},
		},
		devices: []homeassistant.DeviceRegistryEntry{
			{ID: "dev1", Name: "Orig", NameByUser: "My Lamp", AreaID: "living_id", Labels: []string{"lbl_dev"}},
		},
		areas: []homeassistant.AreaRegistryEntry{
			{AreaID: "kitchen_id", Name: "Kitchen"},
			{AreaID: "living_id", Name: "Living Room"},
			{AreaID: "garage_id", Name: "Garage"},
		},
		labels: []homeassistant.LabelRegistryEntry{
			{LabelID: "lbl_a", Name: "Alpha"},
			{LabelID: "lbl_b", Name: "Beta"},
			{LabelID: "lbl_dev", Name: "DevLabel"},
		},
		dashboards: []homeassistant.DashboardEntry{{ID: "dash1", URLPath: "my-dash"}},
	}
}

func TestSnapshot_ObjectFor(t *testing.T) {
	s := &snapshot{inner: testInner()}
	d, err := s.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		id                    string
		areas, labels, devs   []string
		wantAreas, wantLabels bool
	}{
		{"light.own_area", []string{"kitchen_id", "kitchen"}, []string{"lbl_a", "alpha", "lbl_dev", "devlabel"}, []string{"dev1", "my lamp", "orig"}, true, true},
		{"light.inherits", []string{"living_id", "living room"}, []string{"lbl_dev", "devlabel"}, []string{"dev1", "my lamp", "orig"}, true, true},
		{"switch.other", []string{"garage_id", "garage"}, nil, nil, true, false},
		{"sensor.devless", nil, []string{"lbl_b", "beta"}, nil, false, true},
		{"sensor.unknown", nil, nil, nil, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			o := d.objectFor(tc.id)
			if o.Kind != access.KindEntity || o.ID != tc.id {
				t.Fatalf("object = %+v", o)
			}
			if !sameSet(o.Areas, tc.areas) || !sameSet(o.Labels, tc.labels) || !sameSet(o.Devices, tc.devs) {
				t.Errorf("areas %v labels %v devices %v; want %v %v %v", o.Areas, o.Labels, o.Devices, tc.areas, tc.labels, tc.devs)
			}
		})
	}
}

func TestSnapshot_EntitiesOf(t *testing.T) {
	s := &snapshot{inner: testInner()}
	d, err := s.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := d.entitiesOfDevice("dev1"); !sameSet(got, []string{"light.own_area", "light.inherits"}) {
		t.Errorf("entitiesOfDevice = %v", got)
	}
	if got := d.entitiesOfArea("living_id"); !sameSet(got, []string{"light.inherits"}) {
		t.Errorf("entitiesOfArea(living) = %v", got)
	}
	if got := d.entitiesOfArea("kitchen_id"); !sameSet(got, []string{"light.own_area"}) {
		t.Errorf("entitiesOfArea(kitchen) = %v", got)
	}
	if got := d.entitiesOfLabel("lbl_dev"); !sameSet(got, []string{"light.own_area", "light.inherits"}) {
		t.Errorf("entitiesOfLabel(dev) = %v", got)
	}
	if got := d.entitiesOfLabel("lbl_b"); !sameSet(got, []string{"sensor.devless"}) {
		t.Errorf("entitiesOfLabel(b) = %v", got)
	}
	if a, ok := d.area("Living Room"); !ok || a.AreaID != "living_id" {
		t.Errorf("area by name = %+v %v", a, ok)
	}
	if l, ok := d.label("lbl_a"); !ok || l.Name != "Alpha" {
		t.Errorf("label by id = %+v %v", l, ok)
	}
	if dev, ok := d.device("dev1"); !ok || dev.NameByUser != "My Lamp" {
		t.Errorf("device by id = %+v %v", dev, ok)
	}
	if _, ok := d.device(""); ok {
		t.Error("empty device id must not match")
	}
	if db, ok := d.dashboardByID("dash1"); !ok || db.URLPath != "my-dash" {
		t.Errorf("dashboard by id = %+v %v", db, ok)
	}
	if _, ok := d.dashboardByID("nope"); ok {
		t.Error("unknown dashboard id matched")
	}
}

func sameSet(a, b []string) bool {
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return reflect.DeepEqual(x, y)
}

func TestInvalidateLabelRegistryCache_ForwardsAndDropsSnapshot(t *testing.T) {
	inner := testInner()
	w := Wrap(readPolicy(t), inner, nil)
	c, ok := w.(*client)
	if !ok {
		t.Fatalf("Wrap returned %T", w)
	}
	if _, err := c.snap.get(context.Background()); err != nil {
		t.Fatal(err)
	}
	c.InvalidateLabelRegistryCache()
	if inner.invalidated.Load() != 1 {
		t.Error("call was not forwarded to the inner client")
	}
	if _, err := c.snap.get(context.Background()); err != nil {
		t.Fatal(err)
	}
	if inner.entityCalls.Load() != 2 {
		t.Errorf("snapshot was not dropped: %d fetches", inner.entityCalls.Load())
	}
	// An inner client without the optional method must not break the call.
	Wrap(readPolicy(t), &panicInner{}, nil).(*client).InvalidateLabelRegistryCache()
}

func TestDecideEntity_InvalidIDs(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{"light.kitchen"}}, Allow: config.CapList{"control"}},
		},
	})
	c := &client{p: p, inner: &panicInner{}, snap: &snapshot{inner: &panicInner{}}}
	tests := []struct {
		name        string
		id          string
		wantHidden  bool
		wantControl bool
	}{
		{"valid", "light.kitchen", false, true},
		{"trailing space is trimmed", "light.kitchen ", false, true},
		{"upper case is normalised", "LIGHT.Kitchen", false, true},
		{"valid other entity", "light.hall", false, false},
		{"slash suffix", "light.kitchen/x", true, false},
		{"two dots", "a.b.c", true, false},
		{"empty", "", true, false},
		{"no dot", "light", true, false},
		{"glob chars", "light.*", true, false},
		{"embedded newline", "light.kit\nchen", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, err := c.decideEntity(context.Background(), tc.id)
			if err != nil {
				t.Fatal(err)
			}
			if g.Hidden() != tc.wantHidden || g.Control != tc.wantControl {
				t.Errorf("grant = %+v, want hidden=%v control=%v", g, tc.wantHidden, tc.wantControl)
			}
		})
	}
}

func TestDecideEntity_UsesRegistryOnlyWhenNeeded(t *testing.T) {
	inner := testInner()
	plain := &client{p: readPolicy(t), inner: inner, snap: &snapshot{inner: inner}}
	if _, err := plain.decideEntity(context.Background(), "light.own_area"); err != nil {
		t.Fatal(err)
	}
	if inner.entityCalls.Load() != 0 {
		t.Error("a glob-only policy must not fetch the registry")
	}

	attr := mustCompile(t, config.AccessConfig{
		Default: "write",
		Rules: []config.AccessRule{
			{Match: config.AccessMatch{Areas: []string{"Living Room"}}, Deny: config.CapList{"control"}},
		},
	})
	c := &client{p: attr, inner: inner, snap: &snapshot{inner: inner}}
	g, err := c.decideEntity(context.Background(), "light.inherits")
	if err != nil {
		t.Fatal(err)
	}
	if g.Control {
		t.Errorf("inherited area deny not applied: %+v", g)
	}
	g, err = c.decideEntity(context.Background(), "switch.other")
	if err != nil || !g.Control {
		t.Errorf("entity outside the denied area keeps the default: %+v %v", g, err)
	}
}

func TestDecideEntity_RegistryFailureFailsClosed(t *testing.T) {
	inner := testInner()
	inner.failFirst.Store(true)
	attr := mustCompile(t, config.AccessConfig{
		Default: "write",
		Rules: []config.AccessRule{
			{Match: config.AccessMatch{Areas: []string{"kitchen"}}, Allow: config.CapList{"read"}, Deny: config.CapList{"write"}},
		},
	})
	c := &client{p: attr, inner: inner, snap: &snapshot{inner: inner}}
	g, err := c.decideEntity(context.Background(), "light.own_area")
	if err == nil {
		t.Fatal("registry failure must surface as an error")
	}
	if g.Read || g.Control || g.Configure {
		t.Errorf("grant on failure = %+v, want none", g)
	}
	if _, err := c.decideEntity(context.Background(), "light.own_area"); err != nil {
		t.Errorf("retry must succeed: %v", err)
	}
}

func TestDecideDashboard(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{
		Default: "read",
		Rules: []config.AccessRule{
			{Match: config.AccessMatch{Dashboards: []string{"lovelace"}}, Allow: config.CapList{"configure"}},
			{Match: config.AccessMatch{Dashboards: []string{"secret"}}, Allow: config.CapList{"none"}},
		},
	})
	c := &client{p: p}
	if g := c.decideDashboard(""); !g.Configure {
		t.Errorf("empty url_path must map to the default dashboard: %+v", g)
	}
	if g := c.decideDashboard(" Lovelace "); !g.Configure {
		t.Errorf("url_path must be normalised: %+v", g)
	}
	if g := c.decideDashboard("secret"); !g.Hidden() {
		t.Errorf("hidden dashboard visible: %+v", g)
	}
	if g := c.decideDashboard("other"); g.Configure || g.Hidden() {
		t.Errorf("default read expected: %+v", g)
	}
}

func TestObjectFor_DevicesMatchByIDAndBothNames(t *testing.T) {
	inner := &regInner{
		entities: []homeassistant.EntityRegistryEntry{
			{EntityID: "light.only_name", DeviceID: "d_name"},
			{EntityID: "light.both", DeviceID: "d_both"},
		},
		devices: []homeassistant.DeviceRegistryEntry{
			{ID: "d_name", Name: "Living Room Lamp"},
			{ID: "d_both", Name: "Orig Name", NameByUser: "Reading Lamp"},
		},
	}
	s := &snapshot{inner: inner}
	d, err := s.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := d.objectFor("light.only_name").Devices; !sameSet(got, []string{"d_name", "living room lamp"}) {
		t.Errorf("devices = %v", got)
	}
	if got := d.objectFor("light.both").Devices; !sameSet(got, []string{"d_both", "orig name", "reading lamp"}) {
		t.Errorf("devices = %v", got)
	}

	for _, matcher := range []string{"Living Room Lamp", "LIVING ROOM LAMP", "d_name"} {
		p := mustCompile(t, config.AccessConfig{
			Default: "write",
			Rules: []config.AccessRule{
				{Match: config.AccessMatch{Devices: []string{matcher}}, Deny: config.CapList{"control"}},
			},
		})
		c := &client{p: p, inner: inner, snap: &snapshot{inner: inner}}
		g, err := c.decideEntity(context.Background(), "light.only_name")
		if err != nil || g.Control {
			t.Errorf("deny on device %q not applied through the device registry name: %+v %v", matcher, g, err)
		}
	}
	for _, matcher := range []string{"Orig Name", "reading lamp", "d_both"} {
		p := mustCompile(t, config.AccessConfig{
			Default: "write",
			Rules: []config.AccessRule{
				{Match: config.AccessMatch{Devices: []string{matcher}}, Deny: config.CapList{"control"}},
			},
		})
		c := &client{p: p, inner: inner, snap: &snapshot{inner: inner}}
		g, err := c.decideEntity(context.Background(), "light.both")
		if err != nil || g.Control {
			t.Errorf("deny on device %q not applied: %+v %v", matcher, g, err)
		}
	}
}

func TestObjectFor_DanglingReferencesAreNotResolvedByName(t *testing.T) {
	inner := &regInner{
		entities: []homeassistant.EntityRegistryEntry{
			// area_id and label id dangle but equal another object's name.
			{EntityID: "light.dangling", AreaID: "Kitchen", Labels: []string{"Alpha"}},
		},
		areas:  []homeassistant.AreaRegistryEntry{{AreaID: "kitchen_id", Name: "Kitchen"}},
		labels: []homeassistant.LabelRegistryEntry{{LabelID: "lbl_a", Name: "Alpha"}},
	}
	s := &snapshot{inner: inner}
	d, err := s.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	o := d.objectFor("light.dangling")
	if !sameSet(o.Areas, []string{"kitchen"}) {
		t.Errorf("areas = %v, want only the dangling id itself", o.Areas)
	}
	if !sameSet(o.Labels, []string{"alpha"}) {
		t.Errorf("labels = %v, want only the dangling id itself", o.Labels)
	}
	if a, ok := d.areaByID("kitchen_id"); !ok || a.Name != "Kitchen" {
		t.Errorf("areaByID = %+v %v", a, ok)
	}
	if _, ok := d.areaByID("Kitchen"); ok {
		t.Error("areaByID must not match by name")
	}
	if l, ok := d.labelByID("lbl_a"); !ok || l.Name != "Alpha" {
		t.Errorf("labelByID = %+v %v", l, ok)
	}
	if _, ok := d.labelByID("Alpha"); ok {
		t.Error("labelByID must not match by name")
	}
}
