package enforce

import (
	"context"
	"maps"

	"github.com/zorak1103/ha-mcp/internal/access"
)

// System operations and the WebSocket command passthrough.
//
// ClearStatistics, ClearSystemLog and DeleteConfigEntry need their system
// value (statistics, logs, config_entries). SendHACSCommand classifies the
// command: an unknown one is refused, a write needs its system value, a read
// passes. The reads that name an object (trace/list, trace/get by config id,
// update/release_notes by entity id) treat a hidden object as missing: the
// command is sent with an absent twin id in its place (see reads_targets.go),
// so Home Assistant itself gives its answer for a missing object, and the
// hidden id never reaches it.

const kindItem = "item"

func (c *client) ClearStatistics(ctx context.Context, statisticIDs []string) error {
	if err := c.checkSystem(access.SystemStatistics); err != nil {
		return err
	}
	if !c.p.HidesAnything() {
		return c.inner.ClearStatistics(ctx, statisticIDs)
	}
	hidden, err := c.statisticHiddenFunc(ctx)
	if err != nil {
		return c.undecidable("ClearStatistics", err)
	}
	// recorder/clear_statistics ignores unknown ids, so a hidden id is dropped
	// and a request left empty succeeds without a call: exactly HA's answer.
	kept := filterCopy(statisticIDs, func(id string) bool { return !hidden(id) })
	if len(kept) == 0 && len(statisticIDs) > 0 {
		return nil
	}
	return c.inner.ClearStatistics(ctx, kept)
}

func (c *client) ClearSystemLog(ctx context.Context) error {
	if err := c.checkSystem(access.SystemLogs); err != nil {
		return err
	}
	return c.inner.ClearSystemLog(ctx)
}

func (c *client) DeleteConfigEntry(ctx context.Context, entryID string) (bool, error) {
	if err := c.checkSystem(access.SystemConfigEntries); err != nil {
		return false, err
	}
	return c.inner.DeleteConfigEntry(ctx, entryID)
}

func (c *client) SendHACSCommand(ctx context.Context, command string, data map[string]any) (any, error) {
	cls, ok := access.ClassifyWSCommand(command)
	if !ok {
		return nil, c.refuse(command)
	}
	switch {
	case cls.Kind == access.ServiceSystem:
		if err := c.checkSystem(cls.System); err != nil {
			return nil, err
		}
	case cls.Kind != access.ServiceRead:
		return nil, c.refuse(command)
	case cls.DomainArg != "":
		return c.traceCommand(ctx, command, cls, data)
	case cls.EntityArg != "":
		return c.entityCommand(ctx, command, cls.EntityArg, data)
	}
	return c.inner.SendHACSCommand(ctx, command, data)
}

// traceCommand decides trace/list and trace/get. The domain must be
// automation or script. The config id resolves to its entity through the
// registry (configEntityHidden); a hidden or, under a hiding policy, an
// unregistered automation answers like a missing one. Without a config id the
// command lists the whole domain, allowed only when nothing in it can be
// hidden.
func (c *client) traceCommand(ctx context.Context, command string, cls access.CommandClass, data map[string]any) (any, error) {
	domain, ok := data[cls.DomainArg].(string)
	if !ok || (domain != domainAutomation && domain != domainScript) {
		return nil, c.refuse(command)
	}
	raw, present := data[cls.ConfigIDArg]
	if !present {
		if c.p.HidesAnything() && !c.p.DomainWide(domain, access.CapRead) {
			return nil, c.refuse(command)
		}
		return c.inner.SendHACSCommand(ctx, command, data)
	}
	itemID, ok := raw.(string)
	if !ok {
		return nil, c.refuse(command)
	}
	hidden, err := c.configEntityHidden(ctx, domain, itemID)
	if err != nil {
		return nil, c.undecidable(command, err)
	}
	if hidden {
		return c.twinnedCommand(ctx, command, data, cls.ConfigIDArg, kindItem)
	}
	return c.inner.SendHACSCommand(ctx, command, data)
}

// entityCommand decides a read command naming one entity id under key.
func (c *client) entityCommand(ctx context.Context, command, key string, data map[string]any) (any, error) {
	entityID, ok := data[key].(string)
	if !ok {
		return nil, c.refuse(command)
	}
	hidden, err := c.entityHidden(ctx, entityID)
	if err != nil {
		return nil, c.undecidable(command, err)
	}
	if hidden {
		return c.twinnedCommand(ctx, command, data, key, kindEntity)
	}
	return c.inner.SendHACSCommand(ctx, command, data)
}

// twinnedCommand sends command with data[key] replaced by an absent twin of
// the given kind and swaps the twin back out of an error.
func (c *client) twinnedCommand(ctx context.Context, command string, data map[string]any, key, kind string) (any, error) {
	snap, err := c.snap.get(ctx)
	if err != nil {
		return nil, c.undecidable(command, err)
	}
	raw, _ := data[key].(string)
	nonce, err := chooseNonce(append(snap.knownStrings(), raw))
	if err != nil {
		return nil, c.undecidable(command, err)
	}
	tw := newTwins(nonce)
	masked := maps.Clone(data)
	masked[key] = tw.twin(kind, raw)
	res, err := c.inner.SendHACSCommand(ctx, command, masked)
	return res, tw.swapError(err)
}
