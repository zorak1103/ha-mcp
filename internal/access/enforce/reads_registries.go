package enforce

import (
	"context"
	"maps"
	"slices"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Reads of the global registries, config entries and system information.
//
//   - Labels, floors, zones, persons and tags are global objects governed by
//     the default: hidden (an empty list) only under default none. A person's
//     device trackers that are hidden are dropped from the copy returned.
//   - A config entry is hidden when it has registered entities and all of
//     them are hidden, or, without entities, under default none (as for
//     devices). Lists drop hidden entries; a single lookup answers a hidden or
//     (under a hiding policy) unknown entry with HA's missing-entry error. Its
//     options are refused for a hidden or unknown entry and returned without
//     the hidden entity ids they name.
//   - ValidateStatistics drops the issues of hidden statistic ids.
//   - The system log can quote any entity, so under a policy that hides
//     anything it needs the logs system value.
//   - GetServices, GetConfig and CheckConfig pass through (see table.go).

func (c *client) GetLabelRegistry(ctx context.Context) ([]homeassistant.LabelRegistryEntry, error) {
	return globalList(c, func() ([]homeassistant.LabelRegistryEntry, error) { return c.inner.GetLabelRegistry(ctx) })
}

func (c *client) GetFloorRegistry(ctx context.Context) ([]homeassistant.FloorRegistryEntry, error) {
	return globalList(c, func() ([]homeassistant.FloorRegistryEntry, error) { return c.inner.GetFloorRegistry(ctx) })
}

func (c *client) GetZones(ctx context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
	return globalList(c, func() ([]homeassistant.ZoneRegistryEntry, error) { return c.inner.GetZones(ctx) })
}

func (c *client) GetTags(ctx context.Context) ([]homeassistant.TagRegistryEntry, error) {
	return globalList(c, func() ([]homeassistant.TagRegistryEntry, error) { return c.inner.GetTags(ctx) })
}

func (c *client) GetPersons(ctx context.Context) ([]homeassistant.PersonRegistryEntry, error) {
	list, err := globalList(c, func() ([]homeassistant.PersonRegistryEntry, error) { return c.inner.GetPersons(ctx) })
	if err != nil || !c.p.HidesAnything() {
		return list, err
	}
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("GetPersons", err)
	}
	out := make([]homeassistant.PersonRegistryEntry, len(list))
	for i, p := range list {
		out[i] = p
		out[i].DeviceTrackers = filterCopy(p.DeviceTrackers, func(id string) bool { return !hidden(id) })
	}
	return out, nil
}

// globalList returns an empty list when global objects are hidden, else the
// inner list unchanged.
func globalList[T any](c *client, call func() ([]T, error)) ([]T, error) {
	if c.globalHidden() {
		return []T{}, nil
	}
	return call()
}

// Config entries.

// entryHiddenFunc returns a predicate telling whether a config entry is hidden.
func (c *client) entryHiddenFunc(ctx context.Context) (func(entryID string) bool, error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, err
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return nil, err
	}
	return func(entryID string) bool {
		ids := data.entitiesOfConfigEntry(entryID)
		if len(ids) == 0 {
			return c.globalHidden()
		}
		return !slices.ContainsFunc(ids, func(id string) bool { return !hidden(id) })
	}, nil
}

func (c *client) GetConfigEntries(ctx context.Context, domain string) ([]homeassistant.ConfigEntryFull, error) {
	list, err := c.inner.GetConfigEntries(ctx, domain)
	if err != nil || !c.p.HidesAnything() {
		return list, err
	}
	hidden, err := c.entryHiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("GetConfigEntries", err)
	}
	return filterCopy(list, func(e homeassistant.ConfigEntryFull) bool { return !hidden(e.EntryID) }), nil
}

// entryVisible reports whether the entry exists (in the unfiltered list) and
// is not hidden. Only called under a hiding policy.
func (c *client) entryVisible(ctx context.Context, entryID string) (bool, error) {
	hidden, err := c.entryHiddenFunc(ctx)
	if err != nil {
		return false, err
	}
	if hidden(entryID) {
		return false, nil
	}
	all, err := c.inner.GetConfigEntries(ctx, "")
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(all, func(e homeassistant.ConfigEntryFull) bool { return e.EntryID == entryID }), nil
}

func (c *client) GetConfigEntry(ctx context.Context, entryID string) (*homeassistant.ConfigEntryFull, error) {
	if c.p.HidesAnything() {
		visible, err := c.entryVisible(ctx, entryID)
		if err != nil {
			return nil, c.undecidable("GetConfigEntry", err)
		}
		if !visible {
			return nil, errConfigEntryMissing()
		}
	}
	return c.inner.GetConfigEntry(ctx, entryID)
}

// GetConfigEntryOptions walks the entry's options flow. Under a hiding policy
// a hidden or unknown entry is refused generically (alike, so no oracle), and
// the hidden entity ids the options name are dropped from the copy returned.
func (c *client) GetConfigEntryOptions(ctx context.Context, entryID string) (map[string]any, error) {
	if !c.p.HidesAnything() {
		return c.inner.GetConfigEntryOptions(ctx, entryID)
	}
	visible, err := c.entryVisible(ctx, entryID)
	if err != nil {
		return nil, c.undecidable("GetConfigEntryOptions", err)
	}
	if !visible {
		return nil, c.refuse("GetConfigEntryOptions")
	}
	opts, err := c.inner.GetConfigEntryOptions(ctx, entryID)
	if err != nil {
		return nil, err
	}
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("GetConfigEntryOptions", err)
	}
	return scrubHidden(opts, hidden), nil
}

// scrubHidden returns a copy of opts without the hidden entity ids it names:
// a string value naming one drops its key, a list drops the element. Only
// strings shaped like entity ids are judged.
func scrubHidden(opts map[string]any, hidden func(string) bool) map[string]any {
	isHidden := func(v any) bool {
		s, ok := v.(string)
		return ok && entityIDRE.MatchString(s) && hidden(s)
	}
	out := make(map[string]any, len(opts))
	for k, v := range opts {
		switch x := v.(type) {
		case string:
			if !isHidden(x) {
				out[k] = x
			}
		case []any:
			out[k] = slices.DeleteFunc(slices.Clone(x), isHidden)
		case []string:
			out[k] = slices.DeleteFunc(slices.Clone(x), func(s string) bool { return isHidden(s) })
		default:
			out[k] = v
		}
	}
	return out
}

// Statistics and system.

func (c *client) ValidateStatistics(ctx context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
	issues, err := c.inner.ValidateStatistics(ctx)
	if err != nil || !c.p.HidesAnything() {
		return issues, err
	}
	hidden, err := c.statisticHiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("ValidateStatistics", err)
	}
	out := maps.Clone(issues)
	maps.DeleteFunc(out, func(id string, _ []homeassistant.StatisticValidationIssue) bool { return hidden(id) })
	return out, nil
}

func (c *client) GetSystemLog(ctx context.Context) ([]homeassistant.SystemLogEntry, error) {
	if c.p.HidesAnything() {
		if err := c.checkSystem(access.SystemLogs); err != nil {
			return nil, err
		}
	}
	return c.inner.GetSystemLog(ctx)
}

// Pass-through reads (classPassThrough, justified in table.go).

func (c *client) GetServices(ctx context.Context) ([]homeassistant.Service, error) {
	return c.inner.GetServices(ctx)
}

func (c *client) GetConfig(ctx context.Context) (*homeassistant.Config, error) {
	return c.inner.GetConfig(ctx)
}

func (c *client) CheckConfig(ctx context.Context) (*homeassistant.ConfigCheckResult, error) {
	return c.inner.CheckConfig(ctx)
}
