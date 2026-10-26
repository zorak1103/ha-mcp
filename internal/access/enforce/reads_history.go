package enforce

import (
	"context"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// History, logbook and statistics: hidden ids are dropped from the request
// (an empty entity filter would mean "all entities", so a request left without
// ids is answered empty and never sent) and from the result.

// GetHistory answers a hidden entity like a missing one: no history (nil, nil),
// which is what the real client returns for an entity without history.
func (c *client) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]homeassistant.HistoryEntry, error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("GetHistory", err)
	}
	if hidden(entityID) {
		return nil, nil
	}
	groups, err := c.inner.GetHistory(ctx, entityID, start, end)
	if err != nil || !c.p.HidesAnything() {
		return groups, err
	}
	return filterCopy(groups, func(g []homeassistant.HistoryEntry) bool {
		for _, e := range g {
			if e.EntityID != "" && hidden(e.EntityID) {
				return false
			}
		}
		return true
	}), nil
}

// GetLogbook drops hidden ids from the requested entities and the entries whose
// entity or whose cause (context entity) is hidden. A request that names only
// hidden entities is answered with an empty result, not an error.
func (c *client) GetLogbook(ctx context.Context, startTime, endTime string, entityIDs []string) ([]homeassistant.LogbookEntry, error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("GetLogbook", err)
	}
	if !c.p.HidesAnything() {
		return c.inner.GetLogbook(ctx, startTime, endTime, entityIDs)
	}
	kept := entityIDs
	if len(entityIDs) > 0 {
		kept = filterCopy(entityIDs, func(id string) bool { return !hidden(id) })
		if len(kept) == 0 {
			return []homeassistant.LogbookEntry{}, nil
		}
	}
	entries, err := c.inner.GetLogbook(ctx, startTime, endTime, kept)
	if err != nil {
		return nil, err
	}
	return filterCopy(entries, func(e homeassistant.LogbookEntry) bool {
		return !hiddenIfSet(hidden, e.EntityID) && !hiddenIfSet(hidden, e.ContextEntityID)
	}), nil
}

// hiddenIfSet is hidden(id) for a non-empty id; an empty id (an entry without
// an entity) is not hidden.
func hiddenIfSet(hidden func(string) bool, id string) bool {
	return id != "" && hidden(id)
}

// statisticHiddenFunc decides statistic ids: an id shaped like an entity id
// follows that entity; any other id (external statistics such as
// "recorder:total") belongs to no entity and is governed by the default alone.
func (c *client) statisticHiddenFunc(ctx context.Context) (func(string) bool, error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, err
	}
	return func(id string) bool {
		if _, ok := normalizeEntityID(id); ok {
			return hidden(id)
		}
		return c.p.Decide(access.Object{Kind: access.KindGlobal, ID: strings.ToLower(strings.TrimSpace(id))}).Hidden()
	}, nil
}

func (c *client) GetStatistics(ctx context.Context, statIDs []string, period string) ([]homeassistant.StatisticsResult, error) {
	hidden, err := c.statisticHiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("GetStatistics", err)
	}
	if !c.p.HidesAnything() {
		return c.inner.GetStatistics(ctx, statIDs, period)
	}
	kept := statIDs
	if len(statIDs) > 0 {
		if kept = filterCopy(statIDs, func(id string) bool { return !hidden(id) }); len(kept) == 0 {
			return nil, nil
		}
	}
	results, err := c.inner.GetStatistics(ctx, kept, period)
	if err != nil {
		return nil, err
	}
	return filterCopy(results, func(r homeassistant.StatisticsResult) bool { return !hidden(r.StatisticID) }), nil
}

func (c *client) ListStatisticIDs(ctx context.Context, statisticType string) ([]homeassistant.StatisticMeta, error) {
	metas, err := c.inner.ListStatisticIDs(ctx, statisticType)
	if err != nil {
		return nil, err
	}
	hidden, err := c.statisticHiddenFunc(ctx)
	if err != nil {
		return nil, c.undecidable("ListStatisticIDs", err)
	}
	if !c.p.HidesAnything() {
		return metas, nil
	}
	return filterCopy(metas, func(m homeassistant.StatisticMeta) bool { return !hidden(m.StatisticID) }), nil
}
