package enforce

import (
	"context"
	"strings"
)

// Domains whose stored configuration is addressed by a config id.
const (
	domainAutomation = "automation"
	domainScript     = "script"
	domainScene      = "scene"
)

// platformOfDomain is the entity-registry platform that registers the
// entities of a config domain; scenes created through the config API belong to
// the homeassistant platform.
func platformOfDomain(domain string) string {
	if domain == domainScene {
		return "homeassistant"
	}
	return domain
}

// entityForConfigID returns the entity id that belongs to a stored config
// (automation/script/scene config id). It resolves through the registry
// (platform plus unique_id equal to the config id), never by trusting an entity
// id a client reports. registered is false when no registry entry matches; id
// is then the guess <domain>.<config id>. That guess is exact for scripts (the
// config key is the entity id) but only a guess for automations and scenes,
// whose entity id comes from the alias.
func (d *snapData) entityForConfigID(domain, configID string) (id string, registered bool) {
	platform := platformOfDomain(domain)
	prefix := domain + "."
	for _, id := range d.entityIDs {
		e := d.entities[id]
		if e.Platform == platform && e.UniqueID == configID && strings.HasPrefix(id, prefix) {
			return id, true
		}
	}
	return prefix + configID, false
}

// configEntityHidden reports whether the entity behind a stored config is
// hidden from reads. Under a hiding policy an automation or scene config
// without a registry entry cannot be mapped to its entity, so it is treated as
// hidden (fail closed); a script falls back to script.<key>. A policy that
// hides nothing never reaches the lookup.
func (c *client) configEntityHidden(ctx context.Context, domain, configID string) (bool, error) {
	if !c.p.HidesAnything() {
		return false, nil
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return false, err
	}
	entityID, registered := data.entityForConfigID(domain, configID)
	if !registered && domain != domainScript {
		return true, nil
	}
	return c.entityHidden(ctx, entityID)
}

// entitiesByDevice groups the registered entity ids by device id; entities
// without a device are left out.
func (d *snapData) entitiesByDevice() map[string][]string {
	out := map[string][]string{}
	for _, id := range d.entityIDs {
		if dev := d.entities[id].DeviceID; dev != "" {
			out[dev] = append(out[dev], id)
		}
	}
	return out
}
