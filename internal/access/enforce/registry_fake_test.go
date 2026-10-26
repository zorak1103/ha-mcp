package enforce

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// moreStore holds what the fake serves for the helper, registry, dashboard
// and system methods of Task 11.
type moreStore struct {
	configEntries []homeassistant.ConfigEntryFull
	entryOptions  map[string]map[string]any
	zones         []homeassistant.ZoneRegistryEntry
	persons       []homeassistant.PersonRegistryEntry
	tags          []homeassistant.TagRegistryEntry
	sysLog        []homeassistant.SystemLogEntry
	statIssues    map[string][]homeassistant.StatisticValidationIssue
	createdEntity string // returned by CreateHelperEntity
	writeErr      error  // returned by every write of this file
}

// write records a write and returns the injected error.
func (f *readFake) write(method string, data map[string]any) error {
	f.recordWrite(method, "", "", data)
	return f.more.writeErr
}

// Helpers.

func (f *readFake) CreateHelper(_ context.Context, cfg homeassistant.HelperConfig) error {
	return f.write("CreateHelper", map[string]any{"helper": cfg})
}

func (f *readFake) CreateHelperEntity(_ context.Context, cfg homeassistant.HelperConfig) (string, error) {
	return f.more.createdEntity, f.write("CreateHelperEntity", map[string]any{"helper": cfg})
}

func (f *readFake) UpdateHelper(_ context.Context, id string, cfg homeassistant.HelperConfig) error {
	return f.write("UpdateHelper", map[string]any{"id": id, "helper": cfg})
}

func (f *readFake) DeleteHelper(_ context.Context, id string) error {
	return f.write("DeleteHelper", map[string]any{"id": id})
}

// Config entries.

func (f *readFake) GetConfigEntries(context.Context, string) ([]homeassistant.ConfigEntryFull, error) {
	return f.more.configEntries, nil
}

func (f *readFake) GetConfigEntry(_ context.Context, id string) (*homeassistant.ConfigEntryFull, error) {
	f.record("GetConfigEntry", id)
	for i := range f.more.configEntries {
		if f.more.configEntries[i].EntryID == id {
			return &f.more.configEntries[i], nil
		}
	}
	return nil, fmt.Errorf("get config entry failed: %w", errors.New("command failed: not_found - Config entry not found"))
}

func (f *readFake) GetConfigEntryOptions(_ context.Context, id string) (map[string]any, error) {
	f.record("GetConfigEntryOptions", id)
	if o, ok := f.more.entryOptions[id]; ok {
		return o, nil
	}
	return nil, errors.New("init options flow: 404")
}

// Entity and device registry.

func (f *readFake) UpdateEntityRegistryEntry(_ context.Context, id string, cfg homeassistant.EntityRegistryUpdateConfig) (*homeassistant.EntityRegistryEntry, error) {
	if err := f.write("UpdateEntityRegistryEntry", map[string]any{"id": id, "config": cfg}); err != nil {
		return nil, err
	}
	return &homeassistant.EntityRegistryEntry{EntityID: id}, nil
}

func (f *readFake) RemoveEntityRegistryEntry(_ context.Context, id string) error {
	return f.write("RemoveEntityRegistryEntry", map[string]any{"id": id})
}

func (f *readFake) UpdateDeviceRegistryEntry(_ context.Context, id string, cfg homeassistant.DeviceRegistryUpdateConfig) (*homeassistant.DeviceRegistryEntry, error) {
	if err := f.write("UpdateDeviceRegistryEntry", map[string]any{"id": id, "config": cfg}); err != nil {
		return nil, err
	}
	return &homeassistant.DeviceRegistryEntry{ID: id}, nil
}

func (f *readFake) RemoveDeviceConfigEntry(_ context.Context, deviceID, entryID string) error {
	return f.write("RemoveDeviceConfigEntry", map[string]any{"id": deviceID, "entry": entryID})
}

// Areas, labels, floors, zones, persons, tags.

func (f *readFake) CreateArea(_ context.Context, cfg homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
	return &homeassistant.AreaRegistryEntry{Name: cfg.Name}, f.write("CreateArea", map[string]any{"config": cfg})
}

func (f *readFake) UpdateArea(_ context.Context, id string, cfg homeassistant.AreaConfig) (*homeassistant.AreaRegistryEntry, error) {
	return &homeassistant.AreaRegistryEntry{AreaID: id}, f.write("UpdateArea", map[string]any{"id": id, "config": cfg})
}

func (f *readFake) DeleteArea(_ context.Context, id string) error {
	return f.write("DeleteArea", map[string]any{"id": id})
}

func (f *readFake) CreateLabel(_ context.Context, cfg homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
	return &homeassistant.LabelRegistryEntry{Name: cfg.Name}, f.write("CreateLabel", map[string]any{"config": cfg})
}

func (f *readFake) UpdateLabel(_ context.Context, id string, cfg homeassistant.LabelConfig) (*homeassistant.LabelRegistryEntry, error) {
	return &homeassistant.LabelRegistryEntry{LabelID: id}, f.write("UpdateLabel", map[string]any{"id": id, "config": cfg})
}

func (f *readFake) DeleteLabel(_ context.Context, id string) error {
	return f.write("DeleteLabel", map[string]any{"id": id})
}

func (f *readFake) CreateFloor(_ context.Context, cfg homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
	return &homeassistant.FloorRegistryEntry{Name: cfg.Name}, f.write("CreateFloor", map[string]any{"config": cfg})
}

func (f *readFake) UpdateFloor(_ context.Context, id string, cfg homeassistant.FloorConfig) (*homeassistant.FloorRegistryEntry, error) {
	return &homeassistant.FloorRegistryEntry{FloorID: id}, f.write("UpdateFloor", map[string]any{"id": id, "config": cfg})
}

func (f *readFake) DeleteFloor(_ context.Context, id string) error {
	return f.write("DeleteFloor", map[string]any{"id": id})
}

func (f *readFake) GetZones(context.Context) ([]homeassistant.ZoneRegistryEntry, error) {
	return f.more.zones, nil
}

func (f *readFake) CreateZone(_ context.Context, cfg homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
	return &homeassistant.ZoneRegistryEntry{Name: cfg.Name}, f.write("CreateZone", map[string]any{"config": cfg})
}

func (f *readFake) UpdateZone(_ context.Context, id string, cfg homeassistant.ZoneConfig) (*homeassistant.ZoneRegistryEntry, error) {
	return &homeassistant.ZoneRegistryEntry{ID: id}, f.write("UpdateZone", map[string]any{"id": id, "config": cfg})
}

func (f *readFake) DeleteZone(_ context.Context, id string) error {
	return f.write("DeleteZone", map[string]any{"id": id})
}

func (f *readFake) GetPersons(context.Context) ([]homeassistant.PersonRegistryEntry, error) {
	return f.more.persons, nil
}

func (f *readFake) CreatePerson(_ context.Context, cfg homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
	return &homeassistant.PersonRegistryEntry{Name: cfg.Name}, f.write("CreatePerson", map[string]any{"config": cfg})
}

func (f *readFake) UpdatePerson(_ context.Context, id string, cfg homeassistant.PersonConfig) (*homeassistant.PersonRegistryEntry, error) {
	return &homeassistant.PersonRegistryEntry{ID: id}, f.write("UpdatePerson", map[string]any{"id": id, "config": cfg})
}

func (f *readFake) DeletePerson(_ context.Context, id string) error {
	return f.write("DeletePerson", map[string]any{"id": id})
}

func (f *readFake) GetTags(context.Context) ([]homeassistant.TagRegistryEntry, error) {
	return f.more.tags, nil
}

func (f *readFake) CreateTag(_ context.Context, cfg homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
	return &homeassistant.TagRegistryEntry{Name: cfg.Name}, f.write("CreateTag", map[string]any{"config": cfg})
}

func (f *readFake) UpdateTag(_ context.Context, id string, cfg homeassistant.TagConfig) (*homeassistant.TagRegistryEntry, error) {
	return &homeassistant.TagRegistryEntry{TagID: id}, f.write("UpdateTag", map[string]any{"id": id, "config": cfg})
}

func (f *readFake) DeleteTag(_ context.Context, id string) error {
	return f.write("DeleteTag", map[string]any{"id": id})
}

// Dashboards.

func (f *readFake) SaveLovelaceConfig(_ context.Context, urlPath string, cfg map[string]any) error {
	return f.write("SaveLovelaceConfig", map[string]any{"url_path": urlPath, "config": maps.Clone(cfg)})
}

func (f *readFake) CreateDashboard(_ context.Context, cfg homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
	if err := f.write("CreateDashboard", map[string]any{"config": cfg}); err != nil {
		return nil, err
	}
	return &homeassistant.DashboardEntry{ID: "new", URLPath: cfg.URLPath}, nil
}

func (f *readFake) UpdateDashboard(_ context.Context, id string, cfg homeassistant.DashboardConfig) (*homeassistant.DashboardEntry, error) {
	if err := f.write("UpdateDashboard", map[string]any{"id": id, "config": cfg}); err != nil {
		return nil, err
	}
	return &homeassistant.DashboardEntry{ID: id}, nil
}

func (f *readFake) DeleteDashboard(_ context.Context, id string) error {
	return f.write("DeleteDashboard", map[string]any{"id": id})
}

// System reads.

func (f *readFake) GetServices(context.Context) ([]homeassistant.Service, error) {
	f.record("GetServices", "")
	return []homeassistant.Service{}, nil
}

func (f *readFake) GetConfig(context.Context) (*homeassistant.Config, error) {
	f.record("GetConfig", "")
	return &homeassistant.Config{}, nil
}

func (f *readFake) CheckConfig(context.Context) (*homeassistant.ConfigCheckResult, error) {
	f.record("CheckConfig", "")
	return &homeassistant.ConfigCheckResult{}, nil
}

func (f *readFake) GetSystemLog(context.Context) ([]homeassistant.SystemLogEntry, error) {
	f.record("GetSystemLog", "")
	return f.more.sysLog, nil
}

func (f *readFake) ValidateStatistics(context.Context) (map[string][]homeassistant.StatisticValidationIssue, error) {
	return f.more.statIssues, nil
}

// writesOf returns the recorded writes of method.
func (f *readFake) writesOf(method string) []writeCall {
	return slices.DeleteFunc(f.writeCalls(), func(w writeCall) bool { return w.method != method })
}
