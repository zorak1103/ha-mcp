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
// Device, area, floor and label expansions are deliberately not narrowed to the
// service's domain: a platform service acts on entities of another domain, so
// a domain filter could skip an entity the call really reaches.

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

// expandTargets returns the entities the expandable part of t reaches, sorted
// and de-duplicated. The explicit entity ids of t are not included.
func (c *client) expandTargets(ctx context.Context, domain string, t access.TargetSpec, states *stateSet) ([]string, error) {
	if !expandable(t) {
		return nil, nil
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return nil, err
	}
	out := data.expandRegistry(t)
	if t.All {
		all, err := c.allOfDomain(ctx, domain, data, states)
		if err != nil {
			return nil, err
		}
		out = append(out, all...)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// expandRegistry lists the registered entities reached by the device, area,
// floor and label ids of t.
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

// allOfDomain lists what entity_id "all" reaches for a service of domain.
func (c *client) allOfDomain(ctx context.Context, domain string, d *snapData, states *stateSet) ([]string, error) {
	dom := strings.ToLower(strings.TrimSpace(domain))
	every := dom == domainHomeAssistant
	prefix := dom + "."
	out := d.selectEntities(func(e homeassistant.EntityRegistryEntry) bool {
		return every || strings.HasPrefix(strings.ToLower(e.EntityID), prefix) || strings.EqualFold(e.Platform, dom)
	})
	ids, err := states.load(ctx, c.inner)
	if err != nil {
		return nil, err
	}
	for id := range ids {
		if every || strings.HasPrefix(id, prefix) {
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
