package enforce

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func TestNoMethodLeftUnclassified(t *testing.T) {
	var passThrough []string
	for name, cl := range methodClass {
		if cl == classUnclassified {
			t.Errorf("%s is still unclassified", name)
		}
		if cl == classPassThrough {
			passThrough = append(passThrough, name)
		}
	}
	slices.Sort(passThrough)
	// The explicit pass-through set: object-less reads (see table.go). A
	// method may only be added here with a justification next to its entry.
	want := []string{"CheckConfig", "GetConfig", "GetServices"}
	if !reflect.DeepEqual(passThrough, want) {
		t.Errorf("pass-through methods = %v, want exactly %v", passThrough, want)
	}
}

// TestNoClassifiedMethodCallsUnclassified is the inverse guard: a method that
// has an enforcement class must not still answer with the fail-closed
// placeholder. It scans the package source for calls of c.unclassified.
func TestNoClassifiedMethodCallsUnclassified(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil || fn.Name.Name == "unclassified" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "unclassified" {
					if cl, known := methodClass[fn.Name.Name]; !known || cl != classUnclassified {
						t.Errorf("%s: %s calls unclassified but is classified %v", name, fn.Name.Name, cl)
					}
				}
				return true
			})
		}
	}
	if _, err := os.Stat("stubs.go"); err == nil {
		t.Error("stubs.go still exists; every method has its enforcement now")
	}
}

func TestPassThroughMethods_ReachInner(t *testing.T) {
	f := buildHelperFake()
	c, tr, ctx := wrapFake(t, helperPolicy(t), f)
	if _, err := c.GetServices(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CheckConfig(ctx); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"GetServices", "GetConfig", "CheckConfig"} {
		if !f.called(m) {
			t.Errorf("%s did not reach inner", m)
		}
	}
	noDenial(t, tr)
}

func TestGetSystemLog_GatedUnderAHidingPolicy(t *testing.T) {
	f := buildHelperFake()
	f.more.sysLog = []homeassistant.SystemLogEntry{{Name: "x"}}
	c, _, ctx := wrapFake(t, helperPolicy(t), f)
	_, err := c.GetSystemLog(ctx)
	errText(t, err, systemDenial(access.SystemLogs))
	if f.called("GetSystemLog") {
		t.Error("refused read reached inner")
	}

	logs := mustCompile(t, config.AccessConfig{Default: "read", System: &config.CapList{"logs"}, Rules: []config.AccessRule{
		{Match: ents("*.secret*"), Allow: caps("none")},
	}})
	c2, _, ctx2 := wrapFake(t, logs, f)
	if got, err := c2.GetSystemLog(ctx2); err != nil || len(got) != 1 {
		t.Fatalf("with system logs: %v %v", got, err)
	}

	c3, _, ctx3 := wrapFake(t, readPolicyWith(t), f)
	if _, err := c3.GetSystemLog(ctx3); err != nil {
		t.Fatalf("a policy that hides nothing reads the log: %v", err)
	}
}

func configEntryFake() *readFake {
	f := buildHelperFake()
	for i := range f.registry {
		if f.registry[i].EntityID == "light.secret" {
			f.registry[i].ConfigEntryID = "ce_sec"
		}
	}
	f.more.configEntries = append(f.more.configEntries,
		homeassistant.ConfigEntryFull{EntryID: "ce_sec"},
		homeassistant.ConfigEntryFull{EntryID: "ce_none"},
	)
	f.more.entryOptions["ce_um2"] = map[string]any{
		"source": "sensor.secret_um",
		"list":   []any{"sensor.secret_um", "sensor.um_vis"},
		"name":   "Meter",
	}
	f.more.entryOptions["ce_sec"] = map[string]any{"x": 1}
	return f
}

func TestConfigEntries_HiddenEntriesAnswerAsMissing(t *testing.T) {
	f := configEntryFake()
	c, tr, ctx := wrapFake(t, helperPolicy(t), f)
	list, err := c.GetConfigEntries(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range list {
		if e.EntryID == "ce_sec" {
			t.Error("entry whose entities are all hidden is listed")
		}
	}
	if len(list) != len(f.more.configEntries)-1 {
		t.Errorf("listed %d of %d entries", len(list), len(f.more.configEntries))
	}
	for _, id := range []string{"ce_sec", "ce_unknown"} {
		_, err := c.GetConfigEntry(ctx, id)
		sameError(t, err, errConfigEntryMissing())
		_, err = c.GetConfigEntryOptions(ctx, id)
		errText(t, err, genericDenial())
	}
	if slices.ContainsFunc(f.seen(), func(id string) bool { return id == "ce_sec" || id == "ce_unknown" }) {
		t.Error("hidden or unknown entry id reached inner")
	}
	if e, err := c.GetConfigEntry(ctx, "ce_thr"); err != nil || e.EntryID != "ce_thr" {
		t.Fatalf("visible entry: %v %v", e, err)
	}
	if d := tr.First(); d == nil {
		t.Error("the options refusal must be recorded")
	}
}

func TestConfigEntryOptions_HiddenReferencesDropped(t *testing.T) {
	f := configEntryFake()
	c, _, ctx := wrapFake(t, helperPolicy(t), f)
	opts, err := c.GetConfigEntryOptions(ctx, "ce_um2")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"list": []any{"sensor.um_vis"}, "name": "Meter"}
	if !reflect.DeepEqual(opts, want) {
		t.Errorf("options = %v, want %v", opts, want)
	}
	if _, ok := f.more.entryOptions["ce_um2"]["source"]; !ok {
		t.Error("the inner result was mutated")
	}

	c2, _, ctx2 := wrapFake(t, readPolicyWith(t), f)
	opts, err = c2.GetConfigEntryOptions(ctx2, "ce_um2")
	if err != nil || opts["source"] != "sensor.secret_um" {
		t.Errorf("a policy that hides nothing passes options through: %v %v", opts, err)
	}
}

func TestValidateStatistics_HiddenIDsDropped(t *testing.T) {
	f := buildHelperFake()
	f.more.statIssues = map[string][]homeassistant.StatisticValidationIssue{
		"sensor.secret_temp": {{Type: "x"}},
		"sensor.um_a":        {{Type: "y"}},
	}
	c, _, ctx := wrapFake(t, helperPolicy(t), f)
	got, err := c.ValidateStatistics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["sensor.secret_temp"]; ok || len(got) != 1 {
		t.Errorf("issues = %v", got)
	}
	if len(f.more.statIssues) != 2 {
		t.Error("the inner result was mutated")
	}
}

// task11WriteFns has every write this task enforces, with arguments the
// writeDefault policy allows on dashFake.
func task11WriteFns() map[string]writeFn {
	out := registryWriteFns()
	more := map[string]writeFn{
		"CreateHelper": func(ctx context.Context, c *client) error {
			return c.CreateHelper(ctx, helper("input_boolean", map[string]any{"name": "brand new"}))
		},
		"CreateHelperEntity": func(ctx context.Context, c *client) error {
			return errOf(c.CreateHelperEntity(ctx, helper("input_boolean", map[string]any{"name": "brand new"})))
		},
		"UpdateHelper": func(ctx context.Context, c *client) error {
			return c.UpdateHelper(ctx, "ok", helper("input_number", map[string]any{"name": "x"}))
		},
		"DeleteHelper": func(ctx context.Context, c *client) error { return c.DeleteHelper(ctx, "input_number.ok") },
		"UpdateEntityRegistryEntry": func(ctx context.Context, c *client) error {
			return errOf(c.UpdateEntityRegistryEntry(ctx, "light.kitchen", homeassistant.EntityRegistryUpdateConfig{Name: strPtr("x")}))
		},
		"RemoveEntityRegistryEntry": func(ctx context.Context, c *client) error {
			return c.RemoveEntityRegistryEntry(ctx, "light.hall")
		},
		"UpdateDeviceRegistryEntry": func(ctx context.Context, c *client) error {
			return errOf(c.UpdateDeviceRegistryEntry(ctx, "dev_k", homeassistant.DeviceRegistryUpdateConfig{}))
		},
		"RemoveDeviceConfigEntry": func(ctx context.Context, c *client) error {
			return c.RemoveDeviceConfigEntry(ctx, "dev_l", "ce")
		},
		"SaveLovelaceConfig": func(ctx context.Context, c *client) error {
			return c.SaveLovelaceConfig(ctx, "", map[string]any{})
		},
		"CreateDashboard": func(ctx context.Context, c *client) error {
			return errOf(c.CreateDashboard(ctx, homeassistant.DashboardConfig{URLPath: "brand-new"}))
		},
		"UpdateDashboard": func(ctx context.Context, c *client) error {
			return errOf(c.UpdateDashboard(ctx, "d1", homeassistant.DashboardConfig{}))
		},
		"DeleteDashboard": func(ctx context.Context, c *client) error { return c.DeleteDashboard(ctx, "d3") },
	}
	for k, v := range more {
		out[k] = v
	}
	return out
}

func TestTask11Writes_SnapshotInvalidatedAfterInnerError(t *testing.T) {
	boom := errors.New("boom")
	for name, fn := range task11WriteFns() {
		t.Run(name, func(t *testing.T) {
			f := dashFake()
			f.more.writeErr = boom
			c, _, ctx := wrapFake(t, writeDefault(t), f)
			if _, err := c.snap.get(ctx); err != nil {
				t.Fatal(err)
			}
			if err := fn(ctx, c); !errors.Is(err, boom) {
				t.Fatalf("error = %v, want the inner error", err)
			}
			c.snap.mu.Lock()
			cached := c.snap.data != nil
			c.snap.mu.Unlock()
			if cached {
				t.Error("snapshot must be dropped after a failed write")
			}
			if got := len(f.writeCalls()); got != 1 {
				t.Errorf("inner writes = %d, want exactly 1", got)
			}
		})
	}
}
