package enforce

import (
	"context"
	"slices"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Target lookups (applicable triggers, conditions, services, referenced
// objects). Home Assistant expands areas, devices and labels itself, so the
// hidden entities behind them would shape the answer.
//
// For the three name lists (triggers, conditions, services) the answer is a set
// of names such as "lock.unlock", which cannot be filtered per entity. The
// target is therefore rewritten before the call: explicit hidden entity ids are
// dropped, and with a hiding policy every area, device and label is expanded
// from the registry snapshot into its entities, the hidden ones are removed and
// only the remaining entity ids are sent. A target left without any entity is
// answered empty and never sent (an empty target would mean "everything").
//
// ExtractFromTarget reports the expansion itself (referenced entities, devices,
// areas, missing ids), so there the target is only stripped of hidden entity
// ids and the result is filtered: hidden entities and fully hidden devices are
// dropped, areas and the missing_* lists are kept.
//
// Residual: an entity group that is visible can contain hidden members, and
// HA expands the group; for the name lists that can only add a domain's names.

// expandedTarget rewrites target for the name-list methods. skip is true when
// nothing is left to ask about.
func (c *client) expandedTarget(ctx context.Context, t homeassistant.Target) (target homeassistant.Target, skip bool, err error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return t, false, err
	}
	if !c.p.HidesAnything() || isEmptyTarget(t) {
		return t, false, nil
	}
	entities := filterCopy(t.EntityID, func(id string) bool { return !hidden(id) })
	if len(t.DeviceID)+len(t.AreaID)+len(t.LabelID) > 0 {
		data, err := c.snap.get(ctx)
		if err != nil {
			return t, false, err
		}
		for _, id := range expandTarget(data, t) {
			if !hidden(id) {
				entities = append(entities, id)
			}
		}
		slices.Sort(entities)
		entities = slices.Compact(entities)
	}
	if len(entities) == 0 {
		return homeassistant.Target{}, true, nil
	}
	return homeassistant.Target{EntityID: entities}, false, nil
}

func isEmptyTarget(t homeassistant.Target) bool {
	return len(t.EntityID)+len(t.DeviceID)+len(t.AreaID)+len(t.LabelID) == 0
}

// expandTarget lists the entities behind the target's areas, devices and
// labels. A label also reaches the entities of every area that carries it.
func expandTarget(d *snapData, t homeassistant.Target) []string {
	var out []string
	for _, id := range t.AreaID {
		out = append(out, d.entitiesOfArea(id)...)
	}
	for _, id := range t.DeviceID {
		out = append(out, d.entitiesOfDevice(id)...)
	}
	for _, label := range t.LabelID {
		out = append(out, d.entitiesOfLabel(label)...)
		for _, a := range d.areas {
			if slices.Contains(a.Labels, label) {
				out = append(out, d.entitiesOfArea(a.AreaID)...)
			}
		}
	}
	return out
}

// namesForTarget runs one of the name-list methods against the rewritten
// target.
func (c *client) namesForTarget(
	ctx context.Context, method string, t homeassistant.Target,
	call func(homeassistant.Target) ([]string, error),
) ([]string, error) {
	target, skip, err := c.expandedTarget(ctx, t)
	if err != nil {
		return nil, c.undecidable(method, err)
	}
	if skip {
		return []string{}, nil
	}
	return call(target)
}

func (c *client) GetTriggersForTarget(ctx context.Context, t homeassistant.Target, expandGroup *bool) ([]string, error) {
	return c.namesForTarget(ctx, "GetTriggersForTarget", t, func(tt homeassistant.Target) ([]string, error) {
		return c.inner.GetTriggersForTarget(ctx, tt, expandGroup)
	})
}

func (c *client) GetConditionsForTarget(ctx context.Context, t homeassistant.Target, expandGroup *bool) ([]string, error) {
	return c.namesForTarget(ctx, "GetConditionsForTarget", t, func(tt homeassistant.Target) ([]string, error) {
		return c.inner.GetConditionsForTarget(ctx, tt, expandGroup)
	})
}

func (c *client) GetServicesForTarget(ctx context.Context, t homeassistant.Target, expandGroup *bool) ([]string, error) {
	return c.namesForTarget(ctx, "GetServicesForTarget", t, func(tt homeassistant.Target) ([]string, error) {
		return c.inner.GetServicesForTarget(ctx, tt, expandGroup)
	})
}

func (c *client) ExtractFromTarget(ctx context.Context, t homeassistant.Target, expandGroup *bool) (*homeassistant.ExtractFromTargetResult, error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("ExtractFromTarget", err)
	}
	if !c.p.HidesAnything() {
		return c.inner.ExtractFromTarget(ctx, t, expandGroup)
	}
	stripped := t
	if len(t.EntityID) > 0 {
		stripped.EntityID = filterCopy(t.EntityID, func(id string) bool { return !hidden(id) })
		if len(stripped.EntityID) == 0 && len(t.DeviceID)+len(t.AreaID)+len(t.LabelID) == 0 {
			return emptyExtract(), nil
		}
		if len(stripped.EntityID) == 0 {
			stripped.EntityID = nil
		}
	}
	res, err := c.inner.ExtractFromTarget(ctx, stripped, expandGroup)
	if err != nil || res == nil {
		return res, err
	}
	hiddenDevs, err := c.hiddenDevices(ctx)
	if err != nil {
		return nil, c.undecidable("ExtractFromTarget", err)
	}
	out := *res
	out.ReferencedEntities = filterCopy(res.ReferencedEntities, func(id string) bool { return !hidden(id) })
	out.ReferencedDevices = filterCopy(res.ReferencedDevices, func(id string) bool { return !hiddenDevs[id] })
	return &out, nil
}

func emptyExtract() *homeassistant.ExtractFromTargetResult {
	return &homeassistant.ExtractFromTargetResult{
		ReferencedEntities: []string{}, ReferencedDevices: []string{}, ReferencedAreas: []string{},
		MissingDevices: []string{}, MissingAreas: []string{}, MissingFloors: []string{}, MissingLabels: []string{},
	}
}
