package enforce

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// snapshot is a lazily built, per-request view of the unfiltered registries
// and dashboards. The first get fetches everything from inner; later calls
// reuse the result until invalidate is called after a registry write. A failed
// build is never cached, so a retry can succeed. It is safe for concurrent use.
type snapshot struct {
	inner homeassistant.Client

	mu   sync.Mutex
	data *snapData
}

// get returns the current snapshot data, building it on first use. The mutex
// is held across the fetch so concurrent first uses build it exactly once.
func (s *snapshot) get(ctx context.Context) (*snapData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data != nil {
		return s.data, nil
	}
	d, err := fetchSnapshot(ctx, s.inner)
	if err != nil {
		return nil, err
	}
	s.data = d
	return d, nil
}

// invalidate drops the cached data; the next get fetches afresh.
func (s *snapshot) invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = nil
}

// snapData is an immutable view of the registries at one point in time.
type snapData struct {
	entities   map[string]homeassistant.EntityRegistryEntry
	devices    map[string]homeassistant.DeviceRegistryEntry
	areas      []homeassistant.AreaRegistryEntry
	labels     []homeassistant.LabelRegistryEntry
	floors     []homeassistant.FloorRegistryEntry
	dashboards []homeassistant.DashboardEntry
	entityIDs  []string // sorted, for deterministic iteration
}

func fetchSnapshot(ctx context.Context, inner homeassistant.Client) (*snapData, error) {
	entities, err := inner.GetEntityRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("access snapshot: entity registry: %w", err)
	}
	devices, err := inner.GetDeviceRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("access snapshot: device registry: %w", err)
	}
	d := &snapData{
		entities: make(map[string]homeassistant.EntityRegistryEntry, len(entities)),
		devices:  make(map[string]homeassistant.DeviceRegistryEntry, len(devices)),
	}
	if d.areas, err = inner.GetAreaRegistry(ctx); err != nil {
		return nil, fmt.Errorf("access snapshot: area registry: %w", err)
	}
	if d.labels, err = inner.GetLabelRegistry(ctx); err != nil {
		return nil, fmt.Errorf("access snapshot: label registry: %w", err)
	}
	if d.floors, err = inner.GetFloorRegistry(ctx); err != nil {
		return nil, fmt.Errorf("access snapshot: floor registry: %w", err)
	}
	if d.dashboards, err = inner.ListDashboards(ctx); err != nil {
		return nil, fmt.Errorf("access snapshot: dashboards: %w", err)
	}
	for _, e := range entities {
		id := strings.ToLower(e.EntityID)
		d.entities[id] = e
		d.entityIDs = append(d.entityIDs, id)
	}
	for _, dev := range devices {
		d.devices[dev.ID] = dev
	}
	slices.Sort(d.entityIDs)
	return d, nil
}

// entity returns the registry entry of entityID.
func (d *snapData) entity(entityID string) (homeassistant.EntityRegistryEntry, bool) {
	e, ok := d.entities[strings.ToLower(entityID)]
	return e, ok
}

// device returns the registry entry of the device with the given id; the
// empty id (an entity without a device) never matches.
func (d *snapData) device(deviceID string) (homeassistant.DeviceRegistryEntry, bool) {
	if deviceID == "" {
		return homeassistant.DeviceRegistryEntry{}, false
	}
	dev, ok := d.devices[deviceID]
	return dev, ok
}

// areaByID finds an area by area_id only. Registry references (an entity's
// area_id) resolve through here so a dangling reference cannot pull in another
// area's name.
func (d *snapData) areaByID(areaID string) (homeassistant.AreaRegistryEntry, bool) {
	for _, a := range d.areas {
		if a.AreaID == areaID {
			return a, true
		}
	}
	return homeassistant.AreaRegistryEntry{}, false
}

// labelByID finds a label by label_id only; see areaByID.
func (d *snapData) labelByID(labelID string) (homeassistant.LabelRegistryEntry, bool) {
	for _, l := range d.labels {
		if l.LabelID == labelID {
			return l, true
		}
	}
	return homeassistant.LabelRegistryEntry{}, false
}

// area finds an area by area_id or, case-insensitively, by name. For user
// input; registry references use areaByID.
func (d *snapData) area(ref string) (homeassistant.AreaRegistryEntry, bool) {
	for _, a := range d.areas {
		if a.AreaID == ref {
			return a, true
		}
	}
	for _, a := range d.areas {
		if strings.EqualFold(a.Name, strings.TrimSpace(ref)) {
			return a, true
		}
	}
	return homeassistant.AreaRegistryEntry{}, false
}

// label finds a label by label_id or, case-insensitively, by name.
func (d *snapData) label(ref string) (homeassistant.LabelRegistryEntry, bool) {
	for _, l := range d.labels {
		if l.LabelID == ref {
			return l, true
		}
	}
	for _, l := range d.labels {
		if strings.EqualFold(l.Name, strings.TrimSpace(ref)) {
			return l, true
		}
	}
	return homeassistant.LabelRegistryEntry{}, false
}

// dashboardByID finds a dashboard by its registry id.
func (d *snapData) dashboardByID(id string) (homeassistant.DashboardEntry, bool) {
	for _, db := range d.dashboards {
		if db.ID == id {
			return db, true
		}
	}
	return homeassistant.DashboardEntry{}, false
}

// effectiveArea is the area of an entity: its own, else its device's.
func (d *snapData) effectiveArea(e homeassistant.EntityRegistryEntry) string {
	if e.AreaID != "" {
		return e.AreaID
	}
	if dev, ok := d.device(e.DeviceID); ok {
		return dev.AreaID
	}
	return ""
}

// effectiveLabels is the union of an entity's labels and its device's.
func (d *snapData) effectiveLabels(e homeassistant.EntityRegistryEntry) []string {
	labels := slices.Clone(e.Labels)
	if dev, ok := d.device(e.DeviceID); ok {
		labels = append(labels, dev.Labels...)
	}
	return labels
}

// entitiesOfDevice lists the entities that belong to the device.
func (d *snapData) entitiesOfDevice(deviceID string) []string {
	return d.selectEntities(func(e homeassistant.EntityRegistryEntry) bool {
		return deviceID != "" && e.DeviceID == deviceID
	})
}

// entitiesOfArea lists the entities in the area, directly or inherited from
// their device.
func (d *snapData) entitiesOfArea(areaID string) []string {
	return d.selectEntities(func(e homeassistant.EntityRegistryEntry) bool {
		return areaID != "" && d.effectiveArea(e) == areaID
	})
}

// entitiesOfLabel lists the entities carrying the label, directly or through
// their device.
func (d *snapData) entitiesOfLabel(labelID string) []string {
	return d.selectEntities(func(e homeassistant.EntityRegistryEntry) bool {
		return labelID != "" && slices.Contains(d.effectiveLabels(e), labelID)
	})
}

// entitiesOfConfigEntry lists the entities registered by the config entry.
func (d *snapData) entitiesOfConfigEntry(entryID string) []string {
	return d.selectEntities(func(e homeassistant.EntityRegistryEntry) bool {
		return entryID != "" && e.ConfigEntryID == entryID
	})
}

func (d *snapData) selectEntities(match func(homeassistant.EntityRegistryEntry) bool) []string {
	var out []string
	for _, id := range d.entityIDs {
		if match(d.entities[id]) {
			out = append(out, id)
		}
	}
	return out
}

// objectFor builds the access.Object for entityID. Areas hold the entity's
// area (else its device's) by id and name; Labels hold the union of entity and
// device labels by id and name; Devices hold the device id, name_by_user and the registry name, so a
// devices matcher selects by id or either name (a deny must always apply).
// Area and label references resolve by id only.
// All values are lower-cased. An entity without a registry entry has no
// attributes.
func (d *snapData) objectFor(entityID string) access.Object {
	id := strings.ToLower(entityID)
	obj := access.Object{Kind: access.KindEntity, ID: id}
	e, ok := d.entities[id]
	if !ok {
		return obj
	}
	if areaID := d.effectiveArea(e); areaID != "" {
		obj.Areas = addFold(obj.Areas, areaID)
		if a, found := d.areaByID(areaID); found {
			obj.Areas = addFold(obj.Areas, a.Name)
		}
	}
	for _, labelID := range d.effectiveLabels(e) {
		obj.Labels = addFold(obj.Labels, labelID)
		if l, found := d.labelByID(labelID); found {
			obj.Labels = addFold(obj.Labels, l.Name)
		}
	}
	if dev, found := d.device(e.DeviceID); found {
		obj.Devices = addFold(addFold(addFold(obj.Devices, dev.ID), dev.NameByUser), dev.Name)
	}
	return obj
}

// addFold appends v trimmed and lower-cased unless it is empty or present.
func addFold(list []string, v string) []string {
	n := strings.ToLower(strings.TrimSpace(v))
	if n == "" || slices.Contains(list, n) {
		return list
	}
	return append(list, n)
}
