package enforce

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Target lookups (applicable triggers, conditions, services, referenced
// objects). Home Assistant expands areas, devices and labels itself, so the
// hidden entities behind them would shape the answer, and an id the caller
// names explicitly must not look different from a missing one.
//
// Explicit hidden ids are never sent to HA. Each one is replaced by an absent
// twin, an id of the same kind that does not exist (see twins), so HA answers
// exactly as it does for a missing id without the wrapper having to know how;
// for ExtractFromTarget the twin is swapped back to the caller's own id in the
// result, so the hidden id is reported the way a missing one is.
//
// The three name lists (triggers, conditions, services) return names such as
// "lock.unlock" that cannot be filtered per entity. With a hiding policy their
// target is rewritten to entity ids only: areas, devices and labels are
// expanded from the registry snapshot, hidden entities are removed, hidden
// explicit ids become twins. A target left without any entity (an area with no
// visible entity, say) is answered [] without a call, because an empty target
// would mean everything.
//
// ExtractFromTarget reports the expansion itself, so its target keeps areas,
// devices and labels (hidden explicit ones become twins) and the result is
// filtered: entities and devices the caller did not name and that are hidden
// are dropped, without echo.
//
// LIVE-CHECK: the name lists rely on HA answering a twin entity id of a domain
// like any other missing id of that domain.
//
// Residual: a visible entity group can contain hidden members and HA expands
// the group; for the name lists that can only add a domain's names.

// absentPrefix marks twin ids; it is unlikely to clash with a real object.
const absentPrefix = "ha_mcp_absent"

// twins maps the hidden ids of one request to absent twins and back.
type twins struct {
	fwd  map[string]string // "<kind>/<raw id>" -> twin
	back map[string]string // twin -> raw id
}

func newTwins() *twins {
	return &twins{fwd: map[string]string{}, back: map[string]string{}}
}

// twin returns the twin of raw for the kind (entity, device, area). An entity
// twin keeps the domain of the id (sensor when it has none) so domain-specific
// answers stay the same as for a missing id of that domain.
func (t *twins) twin(kind, raw string) string {
	key := kind + "/" + raw
	if tw, ok := t.fwd[key]; ok {
		return tw
	}
	tw := fmt.Sprintf("%s_%s_%d", absentPrefix, kind, len(t.back))
	if kind == kindEntity {
		tw = fmt.Sprintf("%s.%s_%d", entityDomainOf(raw), absentPrefix, len(t.back))
	}
	t.fwd[key] = tw
	t.back[tw] = raw
	return tw
}

const (
	kindEntity = "entity"
	kindDevice = "device"
	kindArea   = "area"
)

// entityDomainOf returns the domain of an entity id, lower-cased and trimmed;
// "sensor" when the id has no usable one.
func entityDomainOf(id string) string {
	d, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(id)), ".")
	if d == "" || strings.ContainsFunc(d, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_'
	}) {
		return "sensor"
	}
	return d
}

func (t *twins) isTwin(id string) bool {
	_, ok := t.back[id]
	return ok
}

// restore swaps every twin in list back to the id it stands for.
func (t *twins) restore(list []string) []string {
	if list == nil {
		return nil
	}
	out := slices.Clone(list)
	for i, id := range out {
		if raw, ok := t.back[id]; ok {
			out[i] = raw
		}
	}
	return out
}

// keepOrTwin filters list: twins always stay (they stand for ids the caller
// named), other ids stay when keep says so.
func (t *twins) keepOrTwin(list []string, keep func(string) bool) []string {
	return filterCopy(list, func(id string) bool { return t.isTwin(id) || keep(id) })
}

// mapIDs returns a copy of in with f applied; nil stays nil.
func mapIDs(in []string, f func(string) string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, id := range in {
		out[i] = f(id)
	}
	return out
}

// hideContext holds the decisions one target request needs.
type hideContext struct {
	hidden      func(entityID string) bool
	hiddenDevs  map[string]bool
	areasHidden bool
	tw          *twins
}

func (c *client) newHideContext(ctx context.Context) (*hideContext, error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, err
	}
	devs, err := c.hiddenDevices(ctx)
	if err != nil {
		return nil, err
	}
	return &hideContext{hidden: hidden, hiddenDevs: devs, areasHidden: c.globalHidden(), tw: newTwins()}, nil
}

// masked returns t with every hidden explicit id replaced by its twin.
func (h *hideContext) masked(t homeassistant.Target) homeassistant.Target {
	return homeassistant.Target{
		EntityID: mapIDs(t.EntityID, func(id string) string {
			if h.hidden(id) {
				return h.tw.twin(kindEntity, id)
			}
			return id
		}),
		DeviceID: mapIDs(t.DeviceID, func(id string) string {
			if h.hiddenDevs[id] {
				return h.tw.twin(kindDevice, id)
			}
			return id
		}),
		AreaID: mapIDs(t.AreaID, func(id string) string {
			if h.areasHidden {
				return h.tw.twin(kindArea, id)
			}
			return id
		}),
		LabelID: slices.Clone(t.LabelID),
	}
}

// expandedTarget rewrites target for the name-list methods. skip is true when
// nothing is left to ask about.
func (c *client) expandedTarget(ctx context.Context, t homeassistant.Target) (target homeassistant.Target, skip bool, err error) {
	if !c.p.HidesAnything() || isEmptyTarget(t) {
		return t, false, nil
	}
	h, err := c.newHideContext(ctx)
	if err != nil {
		return t, false, err
	}
	entities := mapIDs(t.EntityID, func(id string) string {
		if h.hidden(id) {
			return h.tw.twin(kindEntity, id)
		}
		return id
	})
	if len(t.DeviceID)+len(t.AreaID)+len(t.LabelID) > 0 {
		data, err := c.snap.get(ctx)
		if err != nil {
			return t, false, err
		}
		for _, id := range expandTarget(data, t, h.areasHidden) {
			if !h.hidden(id) {
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
// Hidden areas (default none) expand to nothing, like a missing area.
func expandTarget(d *snapData, t homeassistant.Target, areasHidden bool) []string {
	var out []string
	if !areasHidden {
		for _, id := range t.AreaID {
			out = append(out, d.entitiesOfArea(id)...)
		}
	}
	for _, id := range t.DeviceID {
		out = append(out, d.entitiesOfDevice(id)...)
	}
	for _, label := range t.LabelID {
		out = append(out, d.entitiesOfLabel(label)...)
		if areasHidden {
			continue
		}
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

// ExtractFromTarget masks hidden explicit ids, asks HA, drops hidden objects
// the caller did not name from the result and swaps twins back to the caller's
// ids, so a hidden explicit entity is echoed in referenced_entities and a
// hidden explicit device or area is listed in missing_devices or
// missing_areas, the way HA reports a missing one.
// LIVE-CHECK: HA reports an explicit entity id in referenced_entities whether
// or not it exists, and lists unknown device and area ids in missing_devices
// and missing_areas; pin both, and the order of the lists, against a live
// instance. The twin approach copies whatever HA really does, but the
// restore step assumes twins appear in exactly those lists.
func (c *client) ExtractFromTarget(ctx context.Context, t homeassistant.Target, expandGroup *bool) (*homeassistant.ExtractFromTargetResult, error) {
	if !c.p.HidesAnything() {
		return c.inner.ExtractFromTarget(ctx, t, expandGroup)
	}
	h, err := c.newHideContext(ctx)
	if err != nil {
		return nil, c.undecidable("ExtractFromTarget", err)
	}
	res, err := c.inner.ExtractFromTarget(ctx, h.masked(t), expandGroup)
	if err != nil || res == nil {
		return res, err
	}
	out := *res
	out.ReferencedEntities = h.tw.restore(h.tw.keepOrTwin(res.ReferencedEntities, func(id string) bool { return !h.hidden(id) }))
	out.ReferencedDevices = h.tw.restore(h.tw.keepOrTwin(res.ReferencedDevices, func(id string) bool { return !h.hiddenDevs[id] }))
	out.ReferencedAreas = h.tw.restore(h.tw.keepOrTwin(res.ReferencedAreas, func(string) bool { return !h.areasHidden }))
	out.MissingDevices = h.tw.restore(res.MissingDevices)
	out.MissingAreas = h.tw.restore(res.MissingAreas)
	out.MissingFloors = h.tw.restore(res.MissingFloors)
	out.MissingLabels = h.tw.restore(res.MissingLabels)
	return &out, nil
}
