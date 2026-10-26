package enforce

import (
	"context"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Read enforcement. Rules shared by every method in the reads_*.go files:
//
//   - A hidden object is invisible: list results drop it, a single lookup
//     answers with the error of a missing object (notfound.go) without the
//     inner client ever seeing the id, and none of this is recorded as a
//     denial (a denial would tell the caller the object exists).
//   - A readable object is always allowed: control and configure imply read.
//   - When the policy cannot hide anything (HidesAnything is false) there is
//     nothing to decide and every read passes through untouched.
//   - When a decision needs registry data that cannot be fetched, the read is
//     refused with the generic "operation not permitted" denial (undecidable);
//     snapshot internals never reach the caller. The fetch fails independent
//     of the requested object, so this is no existence oracle.
//   - Inner results are shared (CachedClient): filters copy, never mutate.

// hiddenFunc returns a predicate reporting whether an entity id is hidden from
// reads, with the registry view fetched at most once. Without any way to hide
// an object it is always false and fetches nothing.
func (c *client) hiddenFunc(ctx context.Context) (func(entityID string) bool, error) {
	if !c.p.HidesAnything() {
		return func(string) bool { return false }, nil
	}
	grant, err := c.grantFunc(ctx)
	if err != nil {
		return nil, err
	}
	return func(entityID string) bool { return grant(entityID).Hidden() }, nil
}

// entityHidden reports whether the entity is hidden from reads.
func (c *client) entityHidden(ctx context.Context, entityID string) (bool, error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return false, err
	}
	return hidden(entityID), nil
}

// guardEntity runs call unless the entity is hidden, in which case it returns
// the missing-object error instead and never calls through.
func guardEntity[T any](ctx context.Context, c *client, method, entityID string, missing error, call func() (T, error)) (T, error) {
	var zero T
	hidden, err := c.entityHidden(ctx, entityID)
	if err != nil {
		return zero, c.undecidable(method, err)
	}
	if hidden {
		return zero, missing
	}
	return call()
}

// filterByEntity drops the elements whose entity id (idOf) is hidden.
func filterByEntity[T any](ctx context.Context, c *client, method string, in []T, idOf func(T) string) ([]T, error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable(method, err)
	}
	if !c.p.HidesAnything() {
		return in, nil
	}
	return filterCopy(in, func(v T) bool { return !hidden(idOf(v)) }), nil
}

func entityIDOf(e homeassistant.Entity) string { return e.EntityID }

// objectEntityID turns a script or automation id into an entity id the way the
// real client does: a bare id gets the domain prefix.
func objectEntityID(prefix, id string) string {
	if strings.HasPrefix(id, prefix) {
		return id
	}
	return prefix + id
}

// Entity states.

func (c *client) GetStates(ctx context.Context) ([]homeassistant.Entity, error) {
	states, err := c.inner.GetStates(ctx)
	if err != nil {
		return nil, err
	}
	return filterByEntity(ctx, c, "GetStates", states, entityIDOf)
}

func (c *client) GetState(ctx context.Context, entityID string) (*homeassistant.Entity, error) {
	return guardEntity(ctx, c, "GetState", entityID, errEntityMissing(entityID), func() (*homeassistant.Entity, error) {
		return c.inner.GetState(ctx, entityID)
	})
}

// Automations, scripts, scenes, helpers.

func (c *client) ListAutomations(ctx context.Context) ([]homeassistant.Automation, error) {
	list, err := c.inner.ListAutomations(ctx)
	if err != nil {
		return nil, err
	}
	return filterByEntity(ctx, c, "ListAutomations", list, func(a homeassistant.Automation) string { return a.EntityID })
}

func (c *client) GetAutomation(ctx context.Context, automationID string) (*homeassistant.Automation, error) {
	entityID := objectEntityID("automation.", automationID)
	return guardEntity(ctx, c, "GetAutomation", entityID, errAutomationMissing(), func() (*homeassistant.Automation, error) {
		return c.inner.GetAutomation(ctx, automationID)
	})
}

func (c *client) ListScripts(ctx context.Context) ([]homeassistant.Entity, error) {
	list, err := c.inner.ListScripts(ctx)
	if err != nil {
		return nil, err
	}
	return filterByEntity(ctx, c, "ListScripts", list, entityIDOf)
}

func (c *client) GetScript(ctx context.Context, scriptID string) (*homeassistant.Script, error) {
	entityID := objectEntityID("script.", scriptID)
	return guardEntity(ctx, c, "GetScript", entityID, errScriptMissing(entityID), func() (*homeassistant.Script, error) {
		return c.inner.GetScript(ctx, scriptID)
	})
}

func (c *client) ListScenes(ctx context.Context) ([]homeassistant.Entity, error) {
	list, err := c.inner.ListScenes(ctx)
	if err != nil {
		return nil, err
	}
	return filterByEntity(ctx, c, "ListScenes", list, entityIDOf)
}

// GetScene takes the scene's config id, not its entity id, so the entity is
// resolved through the registry (never through Scene.EntityID, which is only
// a guess).
func (c *client) GetScene(ctx context.Context, sceneID string) (*homeassistant.Scene, error) {
	hidden, err := c.configEntityHidden(ctx, domainScene, sceneID)
	if err != nil {
		return nil, c.undecidable("GetScene", err)
	}
	if hidden {
		return nil, errSceneMissing(sceneID)
	}
	return c.inner.GetScene(ctx, sceneID)
}

func (c *client) ListHelpers(ctx context.Context) ([]homeassistant.Entity, error) {
	list, err := c.inner.ListHelpers(ctx)
	if err != nil {
		return nil, err
	}
	return filterByEntity(ctx, c, "ListHelpers", list, entityIDOf)
}

func (c *client) GetHelperConfig(ctx context.Context, platform, entityID string) (map[string]any, error) {
	missing := homeassistant.NewHelperConfigMissingError(platform, entityID)
	// The client also accepts a bare object id for entityID; decide on the
	// full entity id it stands for.
	decided := entityID
	if !strings.HasPrefix(entityID, platform+".") {
		decided = platform + "." + entityID
	}
	return guardEntity(ctx, c, "GetHelperConfig", decided, missing, func() (map[string]any, error) {
		return c.inner.GetHelperConfig(ctx, platform, entityID)
	})
}

// ConfigFileEntryExists reports false for a config entry whose entity is
// hidden, exactly as for one that does not exist. Domains other than
// automation, script and scene have no entity mapping and pass through.
func (c *client) ConfigFileEntryExists(ctx context.Context, domain, configID string) (bool, error) {
	switch domain {
	case domainAutomation, domainScript, domainScene:
		hidden, err := c.configEntityHidden(ctx, domain, configID)
		if err != nil {
			return false, c.undecidable("ConfigFileEntryExists", err)
		}
		if hidden {
			return false, nil
		}
	}
	return c.inner.ConfigFileEntryExists(ctx, domain, configID)
}

// Calendars.

func (c *client) GetCalendars(ctx context.Context) ([]homeassistant.CalendarEntry, error) {
	list, err := c.inner.GetCalendars(ctx)
	if err != nil {
		return nil, err
	}
	return filterByEntity(ctx, c, "GetCalendars", list, func(e homeassistant.CalendarEntry) string { return e.EntityID })
}

func (c *client) GetCalendarEvents(ctx context.Context, entityID, start, end string) ([]homeassistant.CalendarEvent, error) {
	return guardEntity(ctx, c, "GetCalendarEvents", entityID, errCalendarMissing(), func() ([]homeassistant.CalendarEvent, error) {
		return c.inner.GetCalendarEvents(ctx, entityID, start, end)
	})
}

// Registries.

func (c *client) GetEntityRegistry(ctx context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	list, err := c.inner.GetEntityRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return filterByEntity(ctx, c, "GetEntityRegistry", list, func(e homeassistant.EntityRegistryEntry) string { return e.EntityID })
}

func (c *client) GetEntityRegistryEntry(ctx context.Context, entityID string) (*homeassistant.EntityRegistryEntry, error) {
	return guardEntity(ctx, c, "GetEntityRegistryEntry", entityID, errRegistryEntryMissing(), func() (*homeassistant.EntityRegistryEntry, error) {
		return c.inner.GetEntityRegistryEntry(ctx, entityID)
	})
}

// GetDeviceRegistry hides a device only when it has at least one entity and
// every one of them is hidden. A device without entities has nothing to
// inherit a decision from and stays visible: it is governed by the default.
func (c *client) GetDeviceRegistry(ctx context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	devices, err := c.inner.GetDeviceRegistry(ctx)
	if err != nil {
		return nil, err
	}
	hidden, err := c.hiddenDevices(ctx)
	if err != nil {
		return nil, c.undecidable("GetDeviceRegistry", err)
	}
	if len(hidden) == 0 {
		return devices, nil
	}
	return filterCopy(devices, func(d homeassistant.DeviceRegistryEntry) bool { return !hidden[d.ID] }), nil
}

// GetAreaRegistry is unfiltered: areas are objects without a matcher, governed
// by the default alone.
func (c *client) GetAreaRegistry(ctx context.Context) ([]homeassistant.AreaRegistryEntry, error) {
	return c.inner.GetAreaRegistry(ctx)
}

// hiddenDevices returns the set of device ids hidden from reads (empty when
// the policy cannot hide anything).
func (c *client) hiddenDevices(ctx context.Context) (map[string]bool, error) {
	if !c.p.HidesAnything() {
		return map[string]bool{}, nil
	}
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, err
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for deviceID, entityIDs := range data.entitiesByDevice() {
		all := true
		for _, id := range entityIDs {
			if !hidden(id) {
				all = false
				break
			}
		}
		if all {
			out[deviceID] = true
		}
	}
	return out, nil
}

// Dashboards.

func (c *client) ListDashboards(ctx context.Context) ([]homeassistant.DashboardEntry, error) {
	list, err := c.inner.ListDashboards(ctx)
	if err != nil {
		return nil, err
	}
	return filterCopy(list, func(d homeassistant.DashboardEntry) bool {
		return !c.decideDashboard(d.URLPath).Hidden()
	}), nil
}

func (c *client) GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error) {
	if c.decideDashboard(urlPath).Hidden() {
		return nil, errLovelaceMissing(urlPath)
	}
	return c.inner.GetLovelaceConfig(ctx, urlPath)
}
