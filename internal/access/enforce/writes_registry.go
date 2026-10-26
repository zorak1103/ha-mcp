package enforce

import (
	"context"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Registry writes.
//
//   - Entity registry: an edit needs configure on that entity, a removal also
//     delete. Moving the entity into or out of an area or label changes which
//     rules reach it, which is why it is a configure on that entity alone. A
//     rename (NewEntityID) also needs delete on the old id and configure plus
//     create on the new one, decided with the entity's current area, labels
//     and device before any collision is looked at; a taken new id answers
//     errUnavailable. A hidden entity, and under a hiding policy an
//     unregistered one, answers as missing.
//   - Device registry: an edit needs configure on every entity of the device,
//     removing a config entry from it (which may delete it) also delete;
//     hidden entities count and a refusal never says which entity blocked. A
//     device without entities is decided by the default.
//   - Areas, labels, floors, zones, persons and tags are global objects,
//     governed by the default (KindGlobal): create needs configure and create,
//     update configure, delete configure and delete. Rules match areas and
//     labels by id and name, so renaming or deleting one moves entities in or
//     out of rule reach: that also needs configure on every entity it holds
//     (areas: own or inherited from the device; labels: on the entity or its
//     device). A person's device trackers must be visible entities.
//
// Multi-object refusals are generic. Every write that reaches inner drops the
// snapshot, failed or not.

// Global object kinds named in denials (kindArea is shared with the twins).
const (
	kindLabel  = "label"
	kindFloor  = "floor"
	kindZone   = "zone"
	kindPerson = "person"
	kindTag    = "tag"

	methodUpdateEntityRegistry = "UpdateEntityRegistryEntry"
	methodRemoveEntityRegistry = "RemoveEntityRegistryEntry"
)

// globalOp is what a write does to a global object.
type globalOp int

const (
	opCreate globalOp = iota
	opUpdate
	opDelete
)

// Entity registry.

func (c *client) UpdateEntityRegistryEntry(ctx context.Context, entityID string, cfg homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
	if err := c.checkEntityRegistryUpdate(ctx, entityID, cfg); err != nil {
		return nil, err
	}
	return callValueAndInvalidate(c, func() (*homeassistant.EntityRegistryEntry, error) {
		return c.inner.UpdateEntityRegistryEntry(ctx, entityID, cfg)
	})
}

func (c *client) RemoveEntityRegistryEntry(ctx context.Context, entityID string) error {
	g, err := c.registryGrant(ctx, methodRemoveEntityRegistry, entityID, errRegistryRemoveMissing())
	if err != nil {
		return err
	}
	if err := c.helperRights(g, entityID, true); err != nil {
		return err
	}
	return c.callAndInvalidate(func() error { return c.inner.RemoveEntityRegistryEntry(ctx, entityID) })
}

func (c *client) checkEntityRegistryUpdate(ctx context.Context, entityID string, cfg homeassistant.EntityRegistryUpdateConfig) error {
	g, err := c.registryGrant(ctx, methodUpdateEntityRegistry, entityID, errRegistryUpdateMissing())
	if err != nil {
		return err
	}
	if err := c.helperRights(g, entityID, false); err != nil {
		return err
	}
	if cfg.NewEntityID == nil || *cfg.NewEntityID == entityID {
		return nil
	}
	if !g.Delete {
		return c.denyObject(entityID, capDelete)
	}
	return c.checkRename(ctx, entityID, *cfg.NewEntityID)
}

// registryGrant returns the grant on a registry entity; a non-plain, hidden or
// (under a hiding policy) unregistered one answers with missing.
func (c *client) registryGrant(ctx context.Context, method, entityID string, missing error) (access.Grant, error) {
	if !isPlainEntityID(entityID) {
		return access.Grant{}, missing
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return access.Grant{}, c.undecidable(method, err)
	}
	grant, err := c.grantFunc(ctx)
	if err != nil {
		return access.Grant{}, c.undecidable(method, err)
	}
	g := grant(entityID)
	_, registered := data.entity(entityID)
	if g.Hidden() || (!registered && c.p.HidesAnything()) {
		return access.Grant{}, missing
	}
	return g, nil
}

// checkRename decides the new entity id of a rename of the visible oldID.
func (c *client) checkRename(ctx context.Context, oldID, newID string) error {
	oldDomain, _, _ := strings.Cut(oldID, ".")
	newDomain, _, _ := strings.Cut(newID, ".")
	if !isPlainEntityID(newID) || newDomain != oldDomain {
		return c.refuse(methodUpdateEntityRegistry)
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return c.undecidable(methodUpdateEntityRegistry, err)
	}
	obj := data.objectFor(oldID)
	obj.ID = newID
	g := c.p.Decide(obj)
	switch {
	case g.Hidden():
		return c.refuse(methodUpdateEntityRegistry)
	case !g.Configure:
		return c.denyObject(newID, access.CapConfigure)
	case !g.Create:
		return c.denyObject(newID, capCreate)
	}
	taken, err := c.entityTaken(ctx, newID)
	if err != nil {
		return c.undecidable(methodUpdateEntityRegistry, err)
	}
	if taken {
		return errUnavailable(newID)
	}
	return nil
}

// Device registry.

func (c *client) UpdateDeviceRegistryEntry(ctx context.Context, deviceID string, cfg homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
	if err := c.checkDeviceEntities(ctx, "UpdateDeviceRegistryEntry", deviceID, false); err != nil {
		return nil, err
	}
	return callValueAndInvalidate(c, func() (*homeassistant.DeviceRegistryEntry, error) {
		return c.inner.UpdateDeviceRegistryEntry(ctx, deviceID, cfg)
	})
}

func (c *client) RemoveDeviceConfigEntry(ctx context.Context, deviceID, configEntryID string) error {
	if err := c.checkDeviceEntities(ctx, "RemoveDeviceConfigEntry", deviceID, true); err != nil {
		return err
	}
	return c.callAndInvalidate(func() error { return c.inner.RemoveDeviceConfigEntry(ctx, deviceID, configEntryID) })
}

// checkDeviceEntities requires configure (and delete) on every entity of a
// known device, or for a device without entities on the global object. Any
// refusal is generic.
func (c *client) checkDeviceEntities(ctx context.Context, method, deviceID string, needDelete bool) error {
	data, err := c.snap.get(ctx)
	if err != nil {
		return c.undecidable(method, err)
	}
	if _, ok := data.device(deviceID); !ok {
		return c.refuse(method)
	}
	ids := data.entitiesOfDevice(deviceID)
	if len(ids) == 0 {
		g := c.p.Decide(access.Object{Kind: access.KindGlobal})
		if !g.Configure || (needDelete && !g.Delete) {
			return c.refuse(method)
		}
		return nil
	}
	ok, err := c.allConfigurable(ctx, ids, needDelete)
	if err != nil {
		return c.undecidable(method, err)
	}
	if !ok {
		return c.refuse(method)
	}
	return nil
}

// allConfigurable reports whether every entity of ids is granted configure
// (and delete); a hidden entity never is.
func (c *client) allConfigurable(ctx context.Context, ids []string, needDelete bool) (bool, error) {
	grant, err := c.grantFunc(ctx)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		g := grant(id)
		if !g.Configure || (needDelete && !g.Delete) {
			return false, nil
		}
	}
	return true, nil
}

// Global objects.

// checkGlobal decides op on a global object of kind; id is "" for a create.
// Under default none the objects are hidden and the refusal is generic;
// otherwise a denial names the object.
func (c *client) checkGlobal(kind, id string, op globalOp) error {
	g := c.p.Decide(access.Object{Kind: access.KindGlobal})
	object := strings.TrimSpace(kind + " " + id)
	switch {
	case g.Hidden():
		return c.refuse(object)
	case !g.Configure:
		return c.denyObject(object, access.CapConfigure)
	case op == opCreate && !g.Create:
		return c.denyObject(object, capCreate)
	case op == opDelete && !g.Delete:
		return c.denyObject(object, capDelete)
	}
	return nil
}

// checkCarriers requires configure on every entity ids lists (the entities an
// area or label holds); any failure refuses generically.
func (c *client) checkCarriers(ctx context.Context, method string, ids func(*snapData) []string) error {
	data, err := c.snap.get(ctx)
	if err != nil {
		return c.undecidable(method, err)
	}
	ok, err := c.allConfigurable(ctx, ids(data), false)
	if err != nil {
		return c.undecidable(method, err)
	}
	if !ok {
		return c.refuse(method)
	}
	return nil
}

// Areas.

func (c *client) CreateArea(ctx context.Context, cfg homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
	if err := c.checkGlobal(kindArea, "", opCreate); err != nil {
		return nil, err
	}
	return callValueAndInvalidate(c, func() (*homeassistant.AreaRegistryEntry, error) { return c.inner.CreateArea(ctx, cfg) })
}

func (c *client) UpdateArea(ctx context.Context, areaID string, cfg homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
	if err := c.checkGlobal(kindArea, areaID, opUpdate); err != nil {
		return nil, err
	}
	if cfg.Name != "" {
		if err := c.checkCarriers(ctx, "UpdateArea", func(d *snapData) []string {
			if a, ok := d.areaByID(areaID); ok && strings.EqualFold(a.Name, cfg.Name) {
				return nil
			}
			return d.entitiesOfArea(areaID)
		}); err != nil {
			return nil, err
		}
	}
	return callValueAndInvalidate(c, func() (*homeassistant.AreaRegistryEntry, error) { return c.inner.UpdateArea(ctx, areaID, cfg) })
}

func (c *client) DeleteArea(ctx context.Context, areaID string) error {
	if err := c.checkGlobal(kindArea, areaID, opDelete); err != nil {
		return err
	}
	if err := c.checkCarriers(ctx, "DeleteArea", func(d *snapData) []string { return d.entitiesOfArea(areaID) }); err != nil {
		return err
	}
	return c.callAndInvalidate(func() error { return c.inner.DeleteArea(ctx, areaID) })
}

// Labels.

func (c *client) CreateLabel(ctx context.Context, cfg homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
	if err := c.checkGlobal(kindLabel, "", opCreate); err != nil {
		return nil, err
	}
	return callValueAndInvalidate(c, func() (*homeassistant.LabelRegistryEntry, error) { return c.inner.CreateLabel(ctx, cfg) })
}

func (c *client) UpdateLabel(ctx context.Context, labelID string, cfg homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
	if err := c.checkGlobal(kindLabel, labelID, opUpdate); err != nil {
		return nil, err
	}
	if cfg.Name != "" {
		if err := c.checkCarriers(ctx, "UpdateLabel", func(d *snapData) []string {
			if l, ok := d.labelByID(labelID); ok && strings.EqualFold(l.Name, cfg.Name) {
				return nil
			}
			return d.entitiesOfLabel(labelID)
		}); err != nil {
			return nil, err
		}
	}
	return callValueAndInvalidate(c, func() (*homeassistant.LabelRegistryEntry, error) { return c.inner.UpdateLabel(ctx, labelID, cfg) })
}

func (c *client) DeleteLabel(ctx context.Context, labelID string) error {
	if err := c.checkGlobal(kindLabel, labelID, opDelete); err != nil {
		return err
	}
	if err := c.checkCarriers(ctx, "DeleteLabel", func(d *snapData) []string { return d.entitiesOfLabel(labelID) }); err != nil {
		return err
	}
	return c.callAndInvalidate(func() error { return c.inner.DeleteLabel(ctx, labelID) })
}

// Floors, zones, tags: governed by the default alone.

func (c *client) CreateFloor(ctx context.Context, cfg homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
	return globalWrite(c, kindFloor, "", opCreate, func() (*homeassistant.FloorRegistryEntry, error) { return c.inner.CreateFloor(ctx, cfg) })
}

func (c *client) UpdateFloor(ctx context.Context, floorID string, cfg homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
	return globalWrite(c, kindFloor, floorID, opUpdate, func() (*homeassistant.FloorRegistryEntry, error) {
		return c.inner.UpdateFloor(ctx, floorID, cfg)
	})
}

func (c *client) DeleteFloor(ctx context.Context, floorID string) error {
	return globalDelete(c, kindFloor, floorID, func() error { return c.inner.DeleteFloor(ctx, floorID) })
}

func (c *client) CreateZone(ctx context.Context, cfg homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
	return globalWrite(c, kindZone, "", opCreate, func() (*homeassistant.ZoneRegistryEntry, error) { return c.inner.CreateZone(ctx, cfg) })
}

func (c *client) UpdateZone(ctx context.Context, zoneID string, cfg homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
	return globalWrite(c, kindZone, zoneID, opUpdate, func() (*homeassistant.ZoneRegistryEntry, error) {
		return c.inner.UpdateZone(ctx, zoneID, cfg)
	})
}

func (c *client) DeleteZone(ctx context.Context, zoneID string) error {
	return globalDelete(c, kindZone, zoneID, func() error { return c.inner.DeleteZone(ctx, zoneID) })
}

func (c *client) CreateTag(ctx context.Context, cfg homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
	return globalWrite(c, kindTag, "", opCreate, func() (*homeassistant.TagRegistryEntry, error) { return c.inner.CreateTag(ctx, cfg) })
}

func (c *client) UpdateTag(ctx context.Context, tagID string, cfg homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
	return globalWrite(c, kindTag, tagID, opUpdate, func() (*homeassistant.TagRegistryEntry, error) {
		return c.inner.UpdateTag(ctx, tagID, cfg)
	})
}

func (c *client) DeleteTag(ctx context.Context, tagID string) error {
	return globalDelete(c, kindTag, tagID, func() error { return c.inner.DeleteTag(ctx, tagID) })
}

// Persons: global, and every device tracker named must be a visible entity.

func (c *client) CreatePerson(ctx context.Context, cfg homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
	if err := c.checkPerson(ctx, "CreatePerson", "", opCreate, cfg.DeviceTrackers); err != nil {
		return nil, err
	}
	return callValueAndInvalidate(c, func() (*homeassistant.PersonRegistryEntry, error) { return c.inner.CreatePerson(ctx, cfg) })
}

func (c *client) UpdatePerson(ctx context.Context, personID string, cfg homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
	if err := c.checkPerson(ctx, "UpdatePerson", personID, opUpdate, cfg.DeviceTrackers); err != nil {
		return nil, err
	}
	return callValueAndInvalidate(c, func() (*homeassistant.PersonRegistryEntry, error) {
		return c.inner.UpdatePerson(ctx, personID, cfg)
	})
}

// checkPerson decides a person create or update: the global rule, then the
// device trackers.
func (c *client) checkPerson(ctx context.Context, method, personID string, op globalOp, trackers []string) error {
	if err := c.checkGlobal(kindPerson, personID, op); err != nil {
		return err
	}
	return c.checkTrackers(ctx, method, trackers)
}

func (c *client) DeletePerson(ctx context.Context, personID string) error {
	return globalDelete(c, kindPerson, personID, func() error { return c.inner.DeletePerson(ctx, personID) })
}

// checkTrackers refuses a person whose device trackers include an id that is
// not plain, hidden, or under a hiding policy unknown: a person's state
// follows its trackers, so it would publish a hidden tracker. Hidden and
// unknown are refused alike.
func (c *client) checkTrackers(ctx context.Context, method string, trackers []string) error {
	if !c.p.HidesAnything() || len(trackers) == 0 {
		return nil
	}
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return c.undecidable(method, err)
	}
	for _, id := range trackers {
		if hidden(id) {
			return c.refuse(method)
		}
		known, err := c.entityTaken(ctx, id)
		if err != nil {
			return c.undecidable(method, err)
		}
		if !known {
			return c.refuse(method)
		}
	}
	return nil
}

// globalWrite decides op on a global object and calls inner once.
func globalWrite[T any](c *client, kind, id string, op globalOp, call func() (T, error)) (T, error) {
	if err := c.checkGlobal(kind, id, op); err != nil {
		var zero T
		return zero, err
	}
	return callValueAndInvalidate(c, call)
}

// globalDelete decides the delete of a global object and calls inner once.
func globalDelete(c *client, kind, id string, call func() error) error {
	if err := c.checkGlobal(kind, id, opDelete); err != nil {
		return err
	}
	return c.callAndInvalidate(call)
}
