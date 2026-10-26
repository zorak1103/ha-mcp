package access

import (
	"strings"
	"sync"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/config"
)

func boolPtr(b bool) *bool { return &b }

func capList(v ...string) config.CapList { return config.CapList(v) }

func mustCompile(t *testing.T, cfg config.AccessConfig) *Policy {
	t.Helper()
	p, err := Compile(cfg)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return p
}

func entity(id string) Object { return Object{Kind: KindEntity, ID: id} }

func assertGrant(t *testing.T, name string, got, want Grant) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %+v, want %+v", name, got, want)
	}
}

var (
	grantNone  = Grant{}
	grantRead  = Grant{Read: true}
	grantCtl   = Grant{Read: true, Control: true}
	grantCfg   = Grant{Read: true, Configure: true, Create: true, Delete: true}
	grantWrite = Grant{Read: true, Control: true, Configure: true, Create: true, Delete: true}
)

// specConfig is the rules file example from the design spec.
func specConfig() config.AccessConfig {
	empty := config.CapList{}
	return config.AccessConfig{
		Default: "read",
		Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{"light.*", "switch.desk_*"}}, Allow: capList("control")},
			{
				Match:  config.AccessMatch{Entities: []string{"automation.*"}},
				Allow:  capList("control", "configure"),
				Create: boolPtr(false), Delete: boolPtr(false),
			},
			{Match: config.AccessMatch{Dashboards: []string{"lovelace-home"}}, Allow: capList("configure")},
			{Match: config.AccessMatch{Entities: []string{"lock.*"}}, Allow: capList("none")},
			{Match: config.AccessMatch{Labels: []string{"protected"}}, Deny: capList("control", "configure")},
		},
		System: &empty,
	}
}

func TestDecide_SpecExamples(t *testing.T) {
	p := mustCompile(t, specConfig())

	assertGrant(t, "light.kitchen", p.Decide(entity("light.kitchen")), grantCtl)
	assertGrant(t, "switch.desk_lamp", p.Decide(entity("switch.desk_lamp")), grantCtl)
	assertGrant(t, "switch.tv", p.Decide(entity("switch.tv")), grantRead)
	assertGrant(t, "automation.x", p.Decide(entity("automation.x")),
		Grant{Read: true, Control: true, Configure: true, Create: false, Delete: false})
	assertGrant(t, "dashboard lovelace-home", p.Decide(Object{Kind: KindDashboard, ID: "lovelace-home"}), grantCfg)
	assertGrant(t, "dashboard lovelace", p.Decide(Object{Kind: KindDashboard, ID: "lovelace"}), grantRead)

	lock := p.Decide(entity("lock.front"))
	assertGrant(t, "lock.front", lock, grantNone)
	if !lock.Hidden() {
		t.Error("lock.front must be hidden")
	}

	protected := Object{Kind: KindEntity, ID: "light.porch", Labels: []string{"protected"}}
	assertGrant(t, "protected light", p.Decide(protected), grantRead)
	protectedAuto := Object{Kind: KindEntity, ID: "automation.heating", Labels: []string{"protected"}}
	assertGrant(t, "protected automation", p.Decide(protectedAuto), grantRead)

	if p.SystemAllowed(SystemRestart) {
		t.Error("system: [] must allow nothing")
	}
}

func TestDeny_WinsRegardlessOfOrder(t *testing.T) {
	allow := config.AccessRule{Match: config.AccessMatch{Entities: []string{"light.*"}}, Allow: capList("write")}
	deny := config.AccessRule{Match: config.AccessMatch{Entities: []string{"light.kitchen"}}, Deny: capList("configure")}
	for name, rules := range map[string][]config.AccessRule{
		"allow first": {allow, deny},
		"deny first":  {deny, allow},
	} {
		t.Run(name, func(t *testing.T) {
			p := mustCompile(t, config.AccessConfig{Default: "read", Rules: rules})
			assertGrant(t, "light.kitchen", p.Decide(entity("light.kitchen")), grantCtl)
			assertGrant(t, "light.hall", p.Decide(entity("light.hall")), grantWrite)
		})
	}

	t.Run("deny below default write", func(t *testing.T) {
		p := mustCompile(t, config.AccessConfig{Default: "write", Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{"automation.*"}}, Deny: capList("write")},
		}})
		assertGrant(t, "automation.x", p.Decide(entity("automation.x")), grantRead)
		assertGrant(t, "light.x", p.Decide(entity("light.x")), grantWrite)
	})

	t.Run("allow and deny in the same rule", func(t *testing.T) {
		p := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{"light.*"}}, Allow: capList("write"), Deny: capList("control")},
		}})
		assertGrant(t, "light.x", p.Decide(entity("light.x")), grantCfg)
	})
}

func TestAllow_Unions(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"light.*"}}, Allow: capList("control")},
		{Match: config.AccessMatch{Areas: []string{"Kitchen"}}, Allow: capList("configure")},
	}})
	both := Object{Kind: KindEntity, ID: "light.kitchen", Areas: []string{"kitchen"}}
	assertGrant(t, "both rules", p.Decide(both), grantWrite)
	assertGrant(t, "only glob", p.Decide(entity("light.hall")), grantCtl)
	onlyArea := Object{Kind: KindEntity, ID: "switch.kettle", Areas: []string{"kitchen"}}
	assertGrant(t, "only area", p.Decide(onlyArea), grantCfg)
}

func TestNone_AlwaysWins(t *testing.T) {
	none := config.AccessRule{Match: config.AccessMatch{Entities: []string{"lock.*"}}, Allow: capList("none")}
	write := config.AccessRule{Match: config.AccessMatch{Entities: []string{"lock.front"}}, Allow: capList("write")}
	deny := config.AccessRule{Match: config.AccessMatch{Entities: []string{"lock.*"}}, Deny: capList("control")}
	for name, rules := range map[string][]config.AccessRule{
		"none first":     {none, write},
		"none last":      {write, none},
		"none plus deny": {write, deny, none},
	} {
		t.Run(name, func(t *testing.T) {
			p := mustCompile(t, config.AccessConfig{Default: "write", Rules: rules})
			assertGrant(t, "lock.front", p.Decide(entity("lock.front")), grantNone)
			assertGrant(t, "light.x", p.Decide(entity("light.x")), grantWrite)
			if !p.HidesAnything() {
				t.Error("HidesAnything must be true with an allow: none rule")
			}
		})
	}
}

func TestDefaultNone_HidesEverythingUnlessAllowed(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "none", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"sensor.*"}}, Allow: capList("read")},
		{Match: config.AccessMatch{Dashboards: []string{"lovelace"}}, Allow: capList("read")},
	}})
	assertGrant(t, "sensor.temp", p.Decide(entity("sensor.temp")), grantRead)
	assertGrant(t, "light.x", p.Decide(entity("light.x")), grantNone)
	assertGrant(t, "default dashboard", p.Decide(Object{Kind: KindDashboard, ID: "lovelace"}), grantRead)
	assertGrant(t, "other dashboard", p.Decide(Object{Kind: KindDashboard, ID: "energy"}), grantNone)
	assertGrant(t, "global", p.Decide(Object{Kind: KindGlobal, ID: "area:kitchen"}), grantNone)
	if !p.HidesAnything() {
		t.Error("HidesAnything must be true under default none")
	}
	if p.SystemAllowed(SystemReload) {
		t.Error("absent system under default none must allow nothing")
	}
}

func TestControlImpliesRead(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "none", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"light.*"}}, Allow: capList("control")},
		{Match: config.AccessMatch{Entities: []string{"script.*"}}, Allow: capList("configure"), Create: boolPtr(false)},
	}})
	g := p.Decide(entity("light.x"))
	assertGrant(t, "light.x", g, grantCtl)
	if !g.Has(CapRead) || !g.Has(CapControl) || g.Has(CapConfigure) {
		t.Errorf("Has mismatch for %+v", g)
	}
	if g.Hidden() {
		t.Error("control must imply read (not hidden)")
	}
	s := p.Decide(entity("script.x"))
	assertGrant(t, "script.x", s, Grant{Read: true, Configure: true, Create: false, Delete: true})
	if s.Has(Capability("bogus")) {
		t.Error("unknown capability must never be granted")
	}
}

func TestCreateDelete_FalseFromAnyMatchingRule(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"automation.*"}}, Allow: capList("write")},
		{Match: config.AccessMatch{Entities: []string{"automation.heat*"}}, Allow: capList("configure"), Delete: boolPtr(false)},
		{Match: config.AccessMatch{Entities: []string{"automation.x"}}, Allow: capList("configure"), Create: boolPtr(true)},
	}})
	assertGrant(t, "automation.heating", p.Decide(entity("automation.heating")),
		Grant{Read: true, Control: true, Configure: true, Create: true, Delete: false})
	assertGrant(t, "automation.x", p.Decide(entity("automation.x")), grantWrite)

	// create/delete are meaningless without configure.
	p2 := mustCompile(t, config.AccessConfig{Default: "write", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"light.*"}}, Deny: capList("configure")},
	}})
	assertGrant(t, "light.x", p2.Decide(entity("light.x")), grantCtl)
}

func TestSystem_AbsentFollowsDefault_PresentExact(t *testing.T) {
	all := []SystemValue{
		SystemReload, SystemRestart, SystemBackup, SystemUpdates, SystemConfigEntries, SystemHACS,
		SystemAddons, SystemBlueprints, SystemStatistics, SystemLogs, SystemHomeAssistant,
	}
	check := func(t *testing.T, p *Policy, want map[SystemValue]bool) {
		t.Helper()
		for _, v := range all {
			if got := p.SystemAllowed(v); got != want[v] {
				t.Errorf("SystemAllowed(%s) = %v, want %v", v, got, want[v])
			}
		}
	}
	everything := map[SystemValue]bool{}
	for _, v := range all {
		everything[v] = true
	}

	t.Run("absent read", func(t *testing.T) {
		check(t, mustCompile(t, config.AccessConfig{Default: "read"}), nil)
	})
	t.Run("absent write", func(t *testing.T) {
		check(t, mustCompile(t, config.AccessConfig{Default: "write"}), everything)
	})
	t.Run("present empty under write", func(t *testing.T) {
		empty := config.CapList{}
		check(t, mustCompile(t, config.AccessConfig{Default: "write", System: &empty}), nil)
	})
	t.Run("present exact", func(t *testing.T) {
		list := capList("reload", "backup")
		check(t, mustCompile(t, config.AccessConfig{Default: "write", System: &list}),
			map[SystemValue]bool{SystemReload: true, SystemBackup: true})
	})
	t.Run("present all under read", func(t *testing.T) {
		list := capList("all")
		check(t, mustCompile(t, config.AccessConfig{Default: "read", System: &list}), everything)
	})
	t.Run("unknown value never allowed", func(t *testing.T) {
		p := mustCompile(t, config.AccessConfig{Default: "write"})
		if p.SystemAllowed(SystemValue("")) || p.SystemAllowed(SystemValue("bogus")) {
			t.Error("unknown system value must be refused")
		}
	})
}

func TestUnrestricted_OnlyWriteNoRulesNoSystem(t *testing.T) {
	rule := config.AccessRule{Match: config.AccessMatch{Entities: []string{"lock.*"}}, Deny: capList("control")}
	empty := config.CapList{}
	allList := capList("all")
	someList := capList("reload")
	cases := []struct {
		name string
		cfg  config.AccessConfig
		want bool
	}{
		{"write no rules no system", config.AccessConfig{Default: "write"}, true},
		{"write system all", config.AccessConfig{Default: "write", System: &allList}, true},
		{"write system empty", config.AccessConfig{Default: "write", System: &empty}, false},
		{"write system partial", config.AccessConfig{Default: "write", System: &someList}, false},
		{"write with rule", config.AccessConfig{Default: "write", Rules: []config.AccessRule{rule}}, false},
		{"read", config.AccessConfig{Default: "read"}, false},
		{"empty default", config.AccessConfig{}, false},
		{"none", config.AccessConfig{Default: "none"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := mustCompile(t, tc.cfg)
			if got := p.Unrestricted(); got != tc.want {
				t.Errorf("Unrestricted() = %v, want %v", got, tc.want)
			}
			if got, want := p.DefaultIsWrite(), tc.cfg.Default == "write"; got != want {
				t.Errorf("DefaultIsWrite() = %v, want %v", got, want)
			}
		})
	}
}

func TestDomainWide_DenyGlobNarrowsDomain(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "write", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"light.kitchen*"}}, Deny: capList("configure")},
	}})
	if p.DomainWide("light", CapConfigure) {
		t.Error("light must not be domain-wide configure with a deny on light.kitchen*")
	}
	if !p.DomainWide("light", CapControl) {
		t.Error("light control is untouched by the configure deny")
	}
	if !p.DomainWide("switch", CapConfigure) {
		t.Error("switch must still be domain-wide configure")
	}
}

func TestDomainWide_NonLiteralDomainGlobCountsEverywhere(t *testing.T) {
	for _, glob := range []string{"*.kitchen", "*", "l?ght.x", "[a-z]*.x", "light*"} {
		t.Run(glob, func(t *testing.T) {
			p := mustCompile(t, config.AccessConfig{Default: "write", Rules: []config.AccessRule{
				{Match: config.AccessMatch{Entities: []string{glob}}, Allow: capList("none")},
			}})
			for _, d := range []string{"light", "switch"} {
				if p.DomainWide(d, CapRead) {
					t.Errorf("%s must not be domain-wide read under none on %q", d, glob)
				}
			}
		})
	}
}

func TestDomainWide_AttributeMatcherDenyFailsClosed(t *testing.T) {
	for name, m := range map[string]config.AccessMatch{
		"area":   {Areas: []string{"garage"}},
		"label":  {Labels: []string{"protected"}},
		"device": {Devices: []string{"abc"}},
	} {
		t.Run(name, func(t *testing.T) {
			p := mustCompile(t, config.AccessConfig{Default: "write", Rules: []config.AccessRule{
				{Match: m, Deny: capList("control")},
			}})
			if p.DomainWide("light", CapControl) {
				t.Error("attribute deny must make every domain not domain-wide")
			}
			if !p.DomainWide("light", CapConfigure) {
				t.Error("a control deny must not affect configure")
			}
		})
	}
}

func TestDomainWide_AllowNeedsWholeDomainGlob(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"input_boolean.*"}}, Allow: capList("configure")},
		{Match: config.AccessMatch{Entities: []string{"light.kitchen_*"}}, Allow: capList("configure")},
		{Match: config.AccessMatch{Entities: []string{"s*.*"}}, Allow: capList("control")},
		{Match: config.AccessMatch{Areas: []string{"kitchen"}}, Allow: capList("write")},
		{Match: config.AccessMatch{Dashboards: []string{"lovelace"}}, Allow: capList("write")},
	}})
	cases := []struct {
		domain string
		c      Capability
		want   bool
	}{
		{"input_boolean", CapConfigure, true},
		{"input_boolean", CapRead, true},
		{"input_boolean", CapControl, false},
		{"light", CapConfigure, false},
		{"switch", CapControl, true},
		{"sensor", CapControl, true},
		{"climate", CapControl, false},
		{"lovelace", CapConfigure, false},
	}
	for _, tc := range cases {
		if got := p.DomainWide(tc.domain, tc.c); got != tc.want {
			t.Errorf("DomainWide(%s, %s) = %v, want %v", tc.domain, tc.c, got, tc.want)
		}
	}
}

func TestDomainWide_GlobCoverage(t *testing.T) {
	cases := []struct {
		glob string
		want bool
	}{
		{"*", true},
		{"light.*", true},
		{"light.**", true},
		{"l*.*", true},
		{"light*", true},
		{"*.*", true},
		{`light.\*`, false},
		{"light.x*", false},
		{"light.[a-z]*", false},
		{"switch.*", false},
		{"light.?", false},
	}
	for _, tc := range cases {
		if got := globCoversDomain(tc.glob, "light"); got != tc.want {
			t.Errorf("globCoversDomain(%q, light) = %v, want %v", tc.glob, got, tc.want)
		}
	}
}

func TestDomainWide_CreateFalseBlocksConfigure(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"automation.*"}}, Allow: capList("write"), Create: boolPtr(false)},
	}})
	if p.DomainWide("automation", CapConfigure) {
		t.Error("create: false must make configure not domain-wide (used for creates)")
	}
	if !p.DomainWide("automation", CapControl) {
		t.Error("control is unaffected by create: false")
	}
}

func TestDomainWide_DefaultNoneAndUnknown(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "none"})
	if p.DomainWide("light", CapRead) {
		t.Error("default none grants nothing domain-wide")
	}
	w := mustCompile(t, config.AccessConfig{Default: "write"})
	if w.DomainWide("light", Capability("bogus")) || w.DomainWide("", CapRead) {
		t.Error("unknown capability or empty domain must be refused")
	}
}

func TestMatchers_AreaInheritedLabelOnDevice(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Areas: []string{"Living Room"}}, Allow: capList("control")},
		{Match: config.AccessMatch{Labels: []string{"Protected"}}, Deny: capList("control")},
		{Match: config.AccessMatch{Devices: []string{"Hall Sensor"}}, Allow: capList("configure")},
	}})
	// Caller supplies lower-cased ids and names; the area is inherited from the
	// device, the label sits on the device.
	inherited := Object{Kind: KindEntity, ID: "light.lamp", Areas: []string{"living_room", "living room"}}
	assertGrant(t, "area by name", p.Decide(inherited), grantCtl)
	byID := Object{Kind: KindEntity, ID: "light.lamp", Areas: []string{"living_room"}}
	assertGrant(t, "area id does not equal name", p.Decide(byID), grantRead)
	withLabel := Object{Kind: KindEntity, ID: "light.lamp", Areas: []string{"living room"}, Labels: []string{"protected"}}
	assertGrant(t, "device label denies", p.Decide(withLabel), grantRead)
	dev := Object{Kind: KindEntity, ID: "sensor.t", Devices: []string{"d1", "hall sensor"}}
	assertGrant(t, "device by name_by_user", p.Decide(dev), grantCfg)
	upper := Object{Kind: KindEntity, ID: "LIGHT.LAMP", Areas: []string{"LIVING ROOM"}}
	assertGrant(t, "case-insensitive", p.Decide(upper), grantCtl)
	if !p.NeedsRegistry() {
		t.Error("NeedsRegistry must be true with area/label/device matchers")
	}
	// Entity matchers never select dashboards or global objects.
	assertGrant(t, "dashboard", p.Decide(Object{Kind: KindDashboard, ID: "x", Areas: []string{"living room"}}), grantRead)
}

func TestMatchers_MultipleKindsInOneRuleAreAlternatives(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"automation.*"}, Dashboards: []string{"ops"}}, Allow: capList("configure")},
	}})
	assertGrant(t, "entity", p.Decide(entity("automation.a")), grantCfg)
	assertGrant(t, "dashboard", p.Decide(Object{Kind: KindDashboard, ID: "ops"}), grantCfg)
	assertGrant(t, "global", p.Decide(Object{Kind: KindGlobal, ID: "ops"}), grantRead)
}

func TestMatchers_GlobUsesPathSemantics(t *testing.T) {
	p := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"sensor.temp_?"}}, Allow: capList("control")},
	}})
	assertGrant(t, "match", p.Decide(entity("sensor.temp_1")), grantCtl)
	assertGrant(t, "no match", p.Decide(entity("sensor.temp_10")), grantRead)
	assertGrant(t, "backslash id", p.Decide(entity(`sensor.temp\1`)), grantRead)
}

func TestCompile_EmptyRules_ReadOnly(t *testing.T) {
	for name, cfg := range map[string]config.AccessConfig{
		"read":  {Default: "read"},
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			p := mustCompile(t, cfg)
			assertGrant(t, "entity", p.Decide(entity("light.x")), grantRead)
			assertGrant(t, "dashboard", p.Decide(Object{Kind: KindDashboard, ID: "lovelace"}), grantRead)
			assertGrant(t, "global", p.Decide(Object{Kind: KindGlobal, ID: "floor:1"}), grantRead)
			if p.Unrestricted() || p.HidesAnything() || p.NeedsRegistry() || p.DefaultIsWrite() {
				t.Error("read-only default policy flags wrong")
			}
			if p.SystemAllowed(SystemRestart) {
				t.Error("read-only default must allow no system operation")
			}
			if !p.DomainWide("light", CapRead) || p.DomainWide("light", CapControl) {
				t.Error("read-only default domain-wide flags wrong")
			}
		})
	}
}

func TestCompile_RejectsInvalidConfig(t *testing.T) {
	cases := map[string]config.AccessConfig{
		"bad default": {Default: "admin"},
		"deny read": {Default: "read", Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{"light.*"}}, Deny: capList("read")},
		}},
		"uppercase glob": {Default: "read", Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{"Light.*"}}, Allow: capList("control")},
		}},
		"blank area": {Default: "read", Rules: []config.AccessRule{
			{Match: config.AccessMatch{Areas: []string{"  "}}, Deny: capList("control")},
		}},
		"blank glob": {Default: "read", Rules: []config.AccessRule{
			{Match: config.AccessMatch{Entities: []string{" "}}, Deny: capList("control")},
		}},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := Compile(cfg)
			if err == nil || p != nil {
				t.Fatalf("Compile must fail, got policy %v err %v", p, err)
			}
		})
	}
}

func TestCompile_RejectsSeveralEntityMatcherKinds(t *testing.T) {
	multi := map[string]config.AccessMatch{
		"entities+areas":   {Entities: []string{"light.*"}, Areas: []string{"kitchen"}},
		"entities+labels":  {Entities: []string{"light.*"}, Labels: []string{"x"}},
		"areas+devices":    {Areas: []string{"kitchen"}, Devices: []string{"d"}},
		"labels+devices":   {Labels: []string{"x"}, Devices: []string{"d"}},
		"all four+dashbrd": {Entities: []string{"a.*"}, Areas: []string{"b"}, Labels: []string{"c"}, Devices: []string{"d"}, Dashboards: []string{"e"}},
	}
	for name, m := range multi {
		t.Run(name, func(t *testing.T) {
			r := config.AccessRule{Match: m, Allow: capList("write")}
			p, err := Compile(config.AccessConfig{Default: "read", Rules: []config.AccessRule{r}})
			if err == nil || p != nil || !strings.Contains(err.Error(), "split into separate rules") {
				t.Fatalf("Compile: policy %v, err %v; want split error", p, err)
			}
			// Defense in depth: the compiler itself rejects it, independent of Validate.
			if _, err := compileRule(r); err == nil || !strings.Contains(err.Error(), "split into separate rules") {
				t.Errorf("compileRule err %v; want split error", err)
			}
		})
	}
	// One entity-selecting kind with several values stays an OR over values.
	p := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Areas: []string{"kitchen", "hall"}}, Allow: capList("control")},
	}})
	assertGrant(t, "kitchen", p.Decide(Object{Kind: KindEntity, ID: "light.a", Areas: []string{"kitchen"}}), grantCtl)
	assertGrant(t, "hall", p.Decide(Object{Kind: KindEntity, ID: "light.b", Areas: []string{"hall"}}), grantCtl)
	assertGrant(t, "garage", p.Decide(Object{Kind: KindEntity, ID: "light.c", Areas: []string{"garage"}}), grantRead)
}

func TestCompile_DoesNotAliasInput(t *testing.T) {
	cfg := config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Entities: []string{"light.*"}}, Allow: capList("control")},
	}}
	p := mustCompile(t, cfg)
	cfg.Rules[0].Match.Entities[0] = "switch.*"
	cfg.Rules[0].Allow[0] = "none"
	assertGrant(t, "light.x", p.Decide(entity("light.x")), grantCtl)
}

func TestSummary_ShortAndEntityFree(t *testing.T) {
	list := capList("reload", "backup")
	empty := config.CapList{}
	cases := []struct {
		cfg  config.AccessConfig
		want []string
	}{
		{config.AccessConfig{Default: "read"}, []string{"default=read", "0 rules", "system=absent"}},
		{specConfig(), []string{"default=read", "5 rules", "system=none"}},
		{config.AccessConfig{Default: "write", System: &list}, []string{"default=write", "system=reload,backup"}},
		{config.AccessConfig{Default: "none", System: &empty}, []string{"default=none", "system=none"}},
	}
	for _, tc := range cases {
		s := mustCompile(t, tc.cfg).Summary()
		for _, w := range tc.want {
			if !strings.Contains(s, w) {
				t.Errorf("Summary() = %q, missing %q", s, w)
			}
		}
		if strings.Contains(s, "light") || strings.Contains(s, "protected") || strings.Contains(s, "\n") {
			t.Errorf("Summary() = %q leaks rule contents or spans lines", s)
		}
	}
}

func TestPolicy_ConcurrentDecide(t *testing.T) {
	p := mustCompile(t, specConfig())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				_ = p.Decide(Object{Kind: KindEntity, ID: "light.kitchen", Labels: []string{"protected"}})
				_ = p.DomainWide("light", CapControl)
				_ = p.SystemAllowed(SystemReload)
			}
		})
	}
	wg.Wait()
}
