package enforce

import (
	"context"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Service call targets. Besides the entity ids it names, a call reaches the
// entities Home Assistant expands from its device, area, floor and label ids
// and from entity_id "all". The wrapper computes that expansion itself from
// the unfiltered registry snapshot, so every reached entity is checked:
//
//   - device_id: the entities of the device;
//   - area_id: entities in the area, their own or inherited from the device;
//   - floor_id: the entities of every area on the floor (AreaRegistryEntry.FloorID);
//   - label_id: entities carrying the label directly or through their device,
//     and the entities of every area carrying it;
//   - all: every entity of the service's domain or registered by the service's
//     integration (platform services such as sonos.join act on media_player
//     entities), every entity at all for the homeassistant domain; entities
//     known only to the state machine are included.
//
// Ids are compared case-insensitively (ParseTarget lower-cases them; matching
// more than HA would only adds checks). Unknown ids expand to nothing, as in HA.
// An area or label id literally named "all" is folded into All by ParseTarget:
// a superset, so safe.
//
// An expanded entity is kept only when it belongs to the service: its entity
// domain or its registry platform equals the service domain (the platform
// covers platform services such as sonos.* or squeezebox.* acting on
// media_player entities); homeassistant.* services act across domains and are
// not filtered. Explicit ids are never filtered. When the unfiltered expansion
// reached entities but none belongs to the service, or All reaches nothing,
// the call is refused (checkObjects): a non-entity service may resolve such a
// target itself, so it must not fall to the object-less default.

// stateSet is the set of entity ids in the state machine, fetched from the
// inner client at most once per checked call.
type stateSet struct {
	ids map[string]bool
}

func (s *stateSet) load(ctx context.Context, inner homeassistant.Client) (map[string]bool, error) {
	if s.ids != nil {
		return s.ids, nil
	}
	list, err := inner.GetStates(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(list))
	for _, e := range list {
		ids[strings.ToLower(e.EntityID)] = true
	}
	s.ids = ids
	return ids, nil
}

// expandable reports whether t names anything that HA expands to entities.
func expandable(t access.TargetSpec) bool {
	return t.All || len(t.DeviceIDs)+len(t.AreaIDs)+len(t.FloorIDs)+len(t.LabelIDs) > 0
}

// expansion is what the expandable part of a target reaches. ids are the
// entities that belong to the service, sorted and de-duplicated; reached
// reports whether the device, area, floor and label ids reached any entity
// before that filter.
type expansion struct {
	ids     []string
	reached bool
}

// unresolved reports whether the target expands but nothing of the service's
// own was found: All reaching nothing, or entities reached only outside the
// service. Such a call is refused.
func (x expansion) unresolved(t access.TargetSpec) bool {
	return len(x.ids) == 0 && (t.All || x.reached)
}

// expandTargets returns what the expandable part of t reaches for a service of
// domain. The explicit entity ids of t are not included.
func (c *client) expandTargets(ctx context.Context, domain string, t access.TargetSpec, states *stateSet) (expansion, error) {
	if !expandable(t) {
		return expansion{}, nil
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return expansion{}, err
	}
	belongs := serviceFilter(domain)
	reached := data.expandRegistry(t)
	x := expansion{reached: len(reached) > 0}
	for _, id := range reached {
		if e, _ := data.entity(id); belongs(id, e.Platform) {
			x.ids = append(x.ids, id)
		}
	}
	if t.All {
		all, err := c.allOfDomain(ctx, data, states, belongs)
		if err != nil {
			return expansion{}, err
		}
		x.ids = append(x.ids, all...)
	}
	slices.Sort(x.ids)
	x.ids = slices.Compact(x.ids)
	return x, nil
}

// serviceFilter returns the predicate telling whether an entity (id, registry
// platform) belongs to a service of domain; everything belongs to
// homeassistant.
func serviceFilter(domain string) func(entityID, platform string) bool {
	dom := strings.ToLower(strings.TrimSpace(domain))
	if dom == domainHomeAssistant {
		return func(string, string) bool { return true }
	}
	prefix := dom + "."
	return func(entityID, platform string) bool {
		return strings.HasPrefix(strings.ToLower(entityID), prefix) || strings.EqualFold(platform, dom)
	}
}

// expandRegistry lists the registered entities reached by the device, area,
// floor and label ids of t, unfiltered.
func (d *snapData) expandRegistry(t access.TargetSpec) []string {
	areas := slices.Clone(t.AreaIDs)
	for _, a := range d.areas {
		if hasFold(t.FloorIDs, a.FloorID) || anyHasFold(t.LabelIDs, a.Labels) {
			areas = append(areas, a.AreaID)
		}
	}
	return d.selectEntities(func(e homeassistant.EntityRegistryEntry) bool {
		return hasFold(t.DeviceIDs, e.DeviceID) ||
			hasFold(areas, d.effectiveArea(e)) ||
			anyHasFold(t.LabelIDs, d.effectiveLabels(e))
	})
}

// allOfDomain lists what entity_id "all" reaches: every registered or
// state-machine entity that belongs to the service.
func (c *client) allOfDomain(ctx context.Context, d *snapData, states *stateSet, belongs func(string, string) bool) ([]string, error) {
	out := d.selectEntities(func(e homeassistant.EntityRegistryEntry) bool {
		return belongs(e.EntityID, e.Platform)
	})
	ids, err := states.load(ctx, c.inner)
	if err != nil {
		return nil, err
	}
	for id := range ids {
		e, _ := d.entity(id)
		if belongs(id, e.Platform) {
			out = append(out, id)
		}
	}
	return out, nil
}

// hasFold reports whether the non-empty v is in list, ignoring case.
func hasFold(list []string, v string) bool {
	return v != "" && slices.ContainsFunc(list, func(s string) bool { return strings.EqualFold(s, v) })
}

// anyHasFold reports whether any of vals is in list, ignoring case.
func anyHasFold(list, vals []string) bool {
	return slices.ContainsFunc(vals, func(v string) bool { return hasFold(list, v) })
}
