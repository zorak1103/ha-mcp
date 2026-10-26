package enforce

import (
	"context"
	"regexp"
	"slices"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Dashboard writes, decided by the dashboards matcher on the url_path (the
// empty url_path is the default dashboard, access.DefaultDashboardID).
//
//   - SaveLovelaceConfig needs configure on the dashboard; a url_path that is
//     not a plain path is refused. A hidden dashboard, and under a hiding
//     policy an unknown one, answers like HA does for an unknown dashboard.
//   - UpdateDashboard and DeleteDashboard name the dashboard by its registry
//     id, mapped to the url_path through the unfiltered dashboard list; an
//     unknown or hidden id answers like HA's storage collection for a missing
//     item. Update needs configure, delete also delete.
//   - CreateDashboard needs configure and create on the new url_path, which
//     must be what HA accepts (lower-case slug with a hyphen); only then is an
//     existing dashboard with that path reported, neutrally (errUnavailable).
//
// Every write that reaches inner drops the snapshot, failed or not.

var (
	// dashboardPathRE is the shape of an existing dashboard's url_path.
	dashboardPathRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	// newDashboardPathRE is what HA accepts for a new url_path: a slug of
	// lower-case letters and digits joined by single hyphens, with at least
	// one hyphen.
	newDashboardPathRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)+$`)
)

// dashboardObject names a dashboard in denials.
func dashboardObject(urlPath string) string {
	if urlPath == "" {
		return access.DefaultDashboardID
	}
	return urlPath
}

func (c *client) SaveLovelaceConfig(ctx context.Context, urlPath string, cfg map[string]any) error {
	if urlPath != "" && !dashboardPathRE.MatchString(urlPath) {
		return c.refuse("SaveLovelaceConfig")
	}
	g := c.decideDashboard(urlPath)
	if g.Hidden() {
		return errLovelaceSaveMissing(urlPath)
	}
	if urlPath != "" && c.p.HidesAnything() {
		known, err := c.dashboardKnown(ctx, urlPath)
		if err != nil {
			return c.undecidable("SaveLovelaceConfig", err)
		}
		if !known {
			return errLovelaceSaveMissing(urlPath)
		}
	}
	if !g.Configure {
		return c.denyObject(dashboardObject(urlPath), access.CapConfigure)
	}
	return c.callAndInvalidate(func() error { return c.inner.SaveLovelaceConfig(ctx, urlPath, cfg) })
}

// dashboardKnown reports whether a dashboard with urlPath exists.
func (c *client) dashboardKnown(ctx context.Context, urlPath string) (bool, error) {
	data, err := c.snap.get(ctx)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(data.dashboards, func(d homeassistant.DashboardEntry) bool { return d.URLPath == urlPath }), nil
}

func (c *client) CreateDashboard(ctx context.Context, cfg homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
	if !newDashboardPathRE.MatchString(cfg.URLPath) {
		return nil, c.refuse("CreateDashboard")
	}
	g := c.decideDashboard(cfg.URLPath)
	switch {
	case g.Hidden():
		return nil, c.refuse("CreateDashboard")
	case !g.Configure:
		return nil, c.denyObject(cfg.URLPath, access.CapConfigure)
	case !g.Create:
		return nil, c.denyObject(cfg.URLPath, capCreate)
	}
	taken, err := c.dashboardKnown(ctx, cfg.URLPath)
	if err != nil {
		return nil, c.undecidable("CreateDashboard", err)
	}
	if taken {
		return nil, errUnavailable(cfg.URLPath)
	}
	return callValueAndInvalidate(c, func() (*homeassistant.DashboardEntry, error) { return c.inner.CreateDashboard(ctx, cfg) })
}

func (c *client) UpdateDashboard(ctx context.Context, dashboardID string, cfg homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
	if err := c.checkDashboardByID(ctx, "update", dashboardID, false); err != nil {
		return nil, err
	}
	return callValueAndInvalidate(c, func() (*homeassistant.DashboardEntry, error) {
		return c.inner.UpdateDashboard(ctx, dashboardID, cfg)
	})
}

func (c *client) DeleteDashboard(ctx context.Context, dashboardID string) error {
	if err := c.checkDashboardByID(ctx, "delete", dashboardID, true); err != nil {
		return err
	}
	return c.callAndInvalidate(func() error { return c.inner.DeleteDashboard(ctx, dashboardID) })
}

// checkDashboardByID decides an update ("update") or delete ("delete") of the
// dashboard with the given registry id.
func (c *client) checkDashboardByID(ctx context.Context, op, dashboardID string, needDelete bool) error {
	data, err := c.snap.get(ctx)
	if err != nil {
		return c.undecidable(op+" dashboard", err)
	}
	db, ok := data.dashboardByID(dashboardID)
	if !ok {
		return errDashboardMissing(op, dashboardID)
	}
	g := c.decideDashboard(db.URLPath)
	switch {
	case g.Hidden():
		return errDashboardMissing(op, dashboardID)
	case !g.Configure:
		return c.denyObject(dashboardObject(db.URLPath), access.CapConfigure)
	case needDelete && !g.Delete:
		return c.denyObject(dashboardObject(db.URLPath), capDelete)
	}
	return nil
}
