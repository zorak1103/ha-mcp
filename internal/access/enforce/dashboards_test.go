package enforce

import (
	"testing"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

func dashFake() *readFake {
	f := buildHelperFake()
	f.dashboards = []homeassistant.DashboardEntry{
		{ID: "d1", URLPath: "my-dash"},
		{ID: "d2", URLPath: "secret-dash"},
		{ID: "d3", URLPath: "ro-dash"},
		{ID: "d4", URLPath: "nodel-dash"},
	}
	return f
}

// dashPolicy: default read; configure on the default dashboard, my-dash,
// nodel-dash (without delete) and new-*; secret-dash hidden.
func dashPolicy(t *testing.T) *access.Policy {
	t.Helper()
	return mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
		{Match: config.AccessMatch{Dashboards: []string{"lovelace", "my-dash", "new-dash", "new-two"}}, Allow: caps("configure")},
		{Match: config.AccessMatch{Dashboards: []string{"nodel-dash"}}, Allow: caps("configure"), Delete: boolPtr(false)},
		{Match: config.AccessMatch{Dashboards: []string{"secret-dash"}}, Allow: caps("none")},
	}})
}

func TestDashboard_IDMappedAndDefaultIsLovelace(t *testing.T) {
	t.Run("empty url_path is the default dashboard", func(t *testing.T) {
		f := dashFake()
		c, _, ctx := wrapFake(t, dashPolicy(t), f)
		cfg := map[string]any{"views": []any{}}
		if err := c.SaveLovelaceConfig(ctx, "", cfg); err != nil {
			t.Fatal(err)
		}
		callOnce(t, f, "SaveLovelaceConfig", "", "", map[string]any{"url_path": "", "config": cfg})

		ro := mustCompile(t, config.AccessConfig{Default: "read", Rules: []config.AccessRule{
			{Match: config.AccessMatch{Dashboards: []string{"my-dash"}}, Allow: caps("configure")},
		}})
		c2, _, ctx2 := wrapFake(t, ro, dashFake())
		errText(t, c2.SaveLovelaceConfig(ctx2, "", cfg), denialFor("lovelace", "configure"))
	})
	t.Run("save checks the url_path", func(t *testing.T) {
		f := dashFake()
		c, _, ctx := wrapFake(t, dashPolicy(t), f)
		if err := c.SaveLovelaceConfig(ctx, "my-dash", map[string]any{}); err != nil {
			t.Fatal(err)
		}
		errText(t, c.SaveLovelaceConfig(ctx, "ro-dash", map[string]any{}), denialFor("ro-dash", "configure"))
		sameError(t, c.SaveLovelaceConfig(ctx, "secret-dash", map[string]any{}), errLovelaceSaveMissing("secret-dash"))
		sameError(t, c.SaveLovelaceConfig(ctx, "nope-dash", map[string]any{}), errLovelaceSaveMissing("nope-dash"))
		errText(t, c.SaveLovelaceConfig(ctx, "My Dash", map[string]any{}), genericDenial())
		if got := len(f.writeCalls()); got != 1 {
			t.Fatalf("inner writes = %d, want 1", got)
		}
	})
	t.Run("update and delete map the id through the unfiltered list", func(t *testing.T) {
		f := dashFake()
		c, _, ctx := wrapFake(t, dashPolicy(t), f)
		upd := homeassistant.DashboardConfig{Title: "T"}
		if _, err := c.UpdateDashboard(ctx, "d1", upd); err != nil {
			t.Fatal(err)
		}
		_, err := c.UpdateDashboard(ctx, "d3", upd)
		errText(t, err, denialFor("ro-dash", "configure"))
		for _, id := range []string{"d2", "nope"} {
			_, err := c.UpdateDashboard(ctx, id, upd)
			sameError(t, err, errDashboardMissing("update", id))
			sameError(t, c.DeleteDashboard(ctx, id), errDashboardMissing("delete", id))
		}
		errText(t, c.DeleteDashboard(ctx, "d4"), denialFor("nodel-dash", "delete"))
		if err := c.DeleteDashboard(ctx, "d1"); err != nil {
			t.Fatal(err)
		}
		if got := len(f.writeCalls()); got != 2 {
			t.Fatalf("inner writes = %d, want 2", got)
		}
	})
}

func TestCreateDashboard_GrantFirstThenCollision(t *testing.T) {
	cases := map[string]struct {
		path string
		want string
	}{
		"free with rights":     {"new-two", ""},
		"taken with rights":    {"my-dash", errUnavailable("my-dash").Error()},
		"hidden taken":         {"secret-dash", genericDenial()},
		"free without rights":  {"free-x", denialFor("free-x", "configure")},
		"taken without rights": {"ro-dash", denialFor("ro-dash", "configure")},
		"no hyphen":            {"nodash", genericDenial()},
		"underscore":           {"a_b-c", genericDenial()},
		"upper case":           {"New-Dash", genericDenial()},
		"empty":                {"", genericDenial()},
		"trailing hyphen":      {"new-", genericDenial()},
		"slash":                {"new-dash/x", genericDenial()},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := dashFake()
			c, _, ctx := wrapFake(t, dashPolicy(t), f)
			cfg := homeassistant.DashboardConfig{URLPath: tc.path, Title: "x"}
			_, err := c.CreateDashboard(ctx, cfg)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				callOnce(t, f, "CreateDashboard", "", "", map[string]any{"config": cfg})
				return
			}
			errText(t, err, tc.want)
			noInnerWrite(t, f)
		})
	}
}
