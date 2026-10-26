package enforce

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// readFake is a fake inner client for the read tests. Its single lookups
// return, for an id that is absent, the error shape the real clients return
// for a missing object; the shapes are written out here independently of the
// production code so a drift shows up as a test failure. Every id handed to a
// single lookup is recorded, so a test can prove a hidden id never reached it.
type readFake struct {
	homeassistant.Client

	states      []homeassistant.Entity
	registry    []homeassistant.EntityRegistryEntry
	devices     []homeassistant.DeviceRegistryEntry
	areas       []homeassistant.AreaRegistryEntry
	dashboards  []homeassistant.DashboardEntry
	automations []homeassistant.Automation
	calendars   []homeassistant.CalendarEntry
	logbook     []homeassistant.LogbookEntry
	history     [][]homeassistant.HistoryEntry
	statistics  []homeassistant.StatisticsResult
	statMetas   []homeassistant.StatisticMeta
	media       *homeassistant.MediaBrowseResult
	triggers    []string
	extract     *homeassistant.ExtractFromTargetResult
	extractFn   func(homeassistant.Target) *homeassistant.ExtractFromTargetResult
	targetErrFn func(homeassistant.Target) error
	labels      []homeassistant.LabelRegistryEntry
	floors      []homeassistant.FloorRegistryEntry
	hacsFn      func(command string, data map[string]any) (any, error)
	writes      []writeCall // see writes_fake_test.go
	cfg         configStore // see writes_config_test.go

	mu         sync.Mutex
	seenIDs    []string
	logbookIDs [][]string
	historyIDs []string
	statIDs    [][]string
	targets    []homeassistant.Target
	calls      []string
	registryFn func() error
}

func (f *readFake) record(method, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seenIDs = append(f.seenIDs, id)
	f.calls = append(f.calls, method)
}

func (f *readFake) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.seenIDs)
}

func (f *readFake) called(method string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.calls, method)
}

func (f *readFake) hasState(id string) bool {
	return slices.ContainsFunc(f.states, func(e homeassistant.Entity) bool { return e.EntityID == id })
}

func (f *readFake) GetStates(context.Context) ([]homeassistant.Entity, error) { return f.states, nil }

func (f *readFake) GetState(_ context.Context, id string) (*homeassistant.Entity, error) {
	f.record("GetState", id)
	for i := range f.states {
		if f.states[i].EntityID == id {
			return &f.states[i], nil
		}
	}
	return nil, homeassistant.NewEntityNotFoundError(id)
}

func (f *readFake) GetEntityRegistry(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	if f.registryFn != nil {
		if err := f.registryFn(); err != nil {
			return nil, err
		}
	}
	return f.registry, nil
}

func (f *readFake) GetEntityRegistryEntry(_ context.Context, id string) (*homeassistant.EntityRegistryEntry, error) {
	f.record("GetEntityRegistryEntry", id)
	for i := range f.registry {
		if f.registry[i].EntityID == id {
			return &f.registry[i], nil
		}
	}
	return nil, fmt.Errorf("get entity registry entry failed: %w", errors.New("command failed: not_found - Entity not found"))
}

func (f *readFake) GetDeviceRegistry(context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	return f.devices, nil
}

func (f *readFake) GetAreaRegistry(context.Context) ([]homeassistant.AreaRegistryEntry, error) {
	return f.areas, nil
}

func (f *readFake) GetLabelRegistry(context.Context) ([]homeassistant.LabelRegistryEntry, error) {
	return f.labels, nil
}

func (f *readFake) GetFloorRegistry(context.Context) ([]homeassistant.FloorRegistryEntry, error) {
	return f.floors, nil
}

func (f *readFake) ListDashboards(context.Context) ([]homeassistant.DashboardEntry, error) {
	return f.dashboards, nil
}

func (f *readFake) ListAutomations(context.Context) ([]homeassistant.Automation, error) {
	return f.automations, nil
}

func (f *readFake) byPrefix(prefixes ...string) []homeassistant.Entity {
	var out []homeassistant.Entity
	for _, e := range f.states {
		for _, p := range prefixes {
			if strings.HasPrefix(e.EntityID, p) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

func (f *readFake) ListScripts(context.Context) ([]homeassistant.Entity, error) {
	return f.byPrefix("script."), nil
}

func (f *readFake) ListScenes(context.Context) ([]homeassistant.Entity, error) {
	return f.byPrefix("scene."), nil
}

func (f *readFake) ListHelpers(context.Context) ([]homeassistant.Entity, error) {
	return f.byPrefix("input_"), nil
}

func (f *readFake) GetCalendars(context.Context) ([]homeassistant.CalendarEntry, error) {
	return f.calendars, nil
}

func (f *readFake) GetAutomation(_ context.Context, id string) (*homeassistant.Automation, error) {
	f.record("GetAutomation", id)
	entityID := id
	if !strings.HasPrefix(id, "automation.") {
		entityID = "automation." + id
	}
	if f.hasState(entityID) {
		return &homeassistant.Automation{EntityID: entityID, Config: f.cfg.automations[entityID]}, nil
	}
	return nil, fmt.Errorf("get automation failed: %w", errors.New("command failed: not_found - Entity not found"))
}

func (f *readFake) GetScript(_ context.Context, id string) (*homeassistant.Script, error) {
	f.record("GetScript", id)
	entityID := id
	if !strings.HasPrefix(id, "script.") {
		entityID = "script." + id
	}
	if f.hasState(entityID) {
		return &homeassistant.Script{EntityID: entityID, Config: f.cfg.scripts[strings.TrimPrefix(entityID, "script.")]}, nil
	}
	return nil, fmt.Errorf("get script state failed: %w", homeassistant.NewEntityNotFoundError(entityID))
}

func (f *readFake) GetScene(_ context.Context, id string) (*homeassistant.Scene, error) {
	f.record("GetScene", id)
	for _, r := range f.registry {
		if r.Platform == "homeassistant" && r.UniqueID == id {
			return &homeassistant.Scene{EntityID: r.EntityID, Config: f.cfg.scenes[id]}, nil
		}
	}
	return nil, homeassistant.NewConfigNotFoundError("scene", id)
}

func (f *readFake) ConfigFileEntryExists(_ context.Context, domain, id string) (bool, error) {
	f.record("ConfigFileEntryExists", domain+"/"+id)
	if f.cfg.files[domain+"/"+id] {
		return true, nil
	}
	for _, r := range f.registry {
		if r.Platform == domain || (domain == "scene" && r.Platform == "homeassistant") {
			if r.UniqueID == id {
				return true, nil
			}
		}
	}
	return false, nil
}

func (f *readFake) GetHelperConfig(_ context.Context, platform, id string) (map[string]any, error) {
	f.record("GetHelperConfig", id)
	if f.hasState(id) {
		return map[string]any{"id": strings.TrimPrefix(id, platform+".")}, nil
	}
	return nil, fmt.Errorf("%s not found: %s: %w", platform, id, homeassistant.ErrHelperNotFoundInStorage)
}

func (f *readFake) GetLovelaceConfig(_ context.Context, urlPath string) (map[string]any, error) {
	f.record("GetLovelaceConfig", urlPath)
	for _, d := range f.dashboards {
		if d.URLPath == urlPath {
			return map[string]any{"title": d.Title}, nil
		}
	}
	return nil, fmt.Errorf("get lovelace config failed: %w",
		errors.New("command failed: config_not_found - Unknown config specified: "+urlPath))
}

func (f *readFake) GetCalendarEvents(_ context.Context, id, _, _ string) ([]homeassistant.CalendarEvent, error) {
	f.record("GetCalendarEvents", id)
	if f.hasState(id) {
		return []homeassistant.CalendarEvent{{Summary: "event of " + id}}, nil
	}
	return nil, &homeassistant.APIError{StatusCode: http.StatusBadRequest, Message: "failed to get calendar events: "}
}

func (f *readFake) GetCameraSnapshot(_ context.Context, id string) ([]byte, string, error) {
	f.record("GetCameraSnapshot", id)
	if f.hasState(id) {
		return []byte("jpeg"), "image/jpeg", nil
	}
	return nil, "", &homeassistant.APIError{StatusCode: http.StatusNotFound, Message: "failed to get camera snapshot: 404: Not Found"}
}

func (f *readFake) GetCameraStream(_ context.Context, id string) (*homeassistant.StreamInfo, error) {
	f.record("GetCameraStream", id)
	if f.hasState(id) {
		return &homeassistant.StreamInfo{URL: "/stream/" + id}, nil
	}
	return nil, fmt.Errorf("get camera stream failed: %w", errors.New("command failed: start_stream_failed - Camera not found"))
}

func (f *readFake) SignPath(_ context.Context, path string, _ int) (string, error) {
	f.record("SignPath", path)
	return "signed:" + path, nil
}

func (f *readFake) RenderTemplate(context.Context, string) (string, error) {
	f.record("RenderTemplate", "")
	return "rendered", nil
}

func (f *readFake) BrowseMedia(_ context.Context, id string) (*homeassistant.MediaBrowseResult, error) {
	f.record("BrowseMedia", id)
	return f.media, nil
}

func (f *readFake) GetLogbook(_ context.Context, _, _ string, ids []string) ([]homeassistant.LogbookEntry, error) {
	f.mu.Lock()
	f.logbookIDs = append(f.logbookIDs, slices.Clone(ids))
	f.mu.Unlock()
	return f.logbook, nil
}

func (f *readFake) GetHistory(_ context.Context, id string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
	f.mu.Lock()
	f.historyIDs = append(f.historyIDs, id)
	f.mu.Unlock()
	return f.history, nil
}

func (f *readFake) GetStatistics(_ context.Context, ids []string, _ string) ([]homeassistant.StatisticsResult, error) {
	f.mu.Lock()
	f.statIDs = append(f.statIDs, slices.Clone(ids))
	f.mu.Unlock()
	return f.statistics, nil
}

func (f *readFake) ListStatisticIDs(context.Context, string) ([]homeassistant.StatisticMeta, error) {
	return f.statMetas, nil
}

func (f *readFake) recordTarget(t homeassistant.Target) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.targets = append(f.targets, t)
}

// targetErr records t and returns the injected error, if any.
func (f *readFake) targetErr(t homeassistant.Target) error {
	f.recordTarget(t)
	if f.targetErrFn != nil {
		return f.targetErrFn(t)
	}
	return nil
}

func (f *readFake) GetTriggersForTarget(_ context.Context, t homeassistant.Target, _ *bool) ([]string, error) {
	if err := f.targetErr(t); err != nil {
		return nil, err
	}
	return f.triggers, nil
}

func (f *readFake) GetConditionsForTarget(_ context.Context, t homeassistant.Target, _ *bool) ([]string, error) {
	if err := f.targetErr(t); err != nil {
		return nil, err
	}
	return f.triggers, nil
}

func (f *readFake) GetServicesForTarget(_ context.Context, t homeassistant.Target, _ *bool) ([]string, error) {
	if err := f.targetErr(t); err != nil {
		return nil, err
	}
	return f.triggers, nil
}

// reverseLists mimics HA's set-order output: not the request order, not sorted.
func reverseLists(r *homeassistant.ExtractFromTargetResult) *homeassistant.ExtractFromTargetResult {
	for _, l := range [][]string{
		r.ReferencedEntities, r.ReferencedDevices, r.ReferencedAreas,
		r.MissingDevices, r.MissingAreas, r.MissingFloors, r.MissingLabels,
	} {
		slices.Reverse(l)
	}
	return r
}

func (f *readFake) ExtractFromTarget(_ context.Context, t homeassistant.Target, _ *bool) (*homeassistant.ExtractFromTargetResult, error) {
	if err := f.targetErr(t); err != nil {
		return nil, err
	}
	if f.extractFn != nil {
		return f.extractFn(t), nil
	}
	return f.extract, nil
}

// echoExtract answers like HA: explicit entity ids are referenced whether or
// not they exist; a known device or area is referenced (an area also brings
// its entities), an unknown one is reported missing.
func (f *readFake) echoExtract(t homeassistant.Target) *homeassistant.ExtractFromTargetResult {
	r := &homeassistant.ExtractFromTargetResult{ReferencedEntities: slices.Clone(t.EntityID)}
	for _, d := range t.DeviceID {
		if slices.ContainsFunc(f.devices, func(e homeassistant.DeviceRegistryEntry) bool { return e.ID == d }) {
			r.ReferencedDevices = append(r.ReferencedDevices, d)
		} else {
			r.MissingDevices = append(r.MissingDevices, d)
		}
	}
	for _, a := range t.AreaID {
		if slices.ContainsFunc(f.areas, func(e homeassistant.AreaRegistryEntry) bool { return e.AreaID == a }) {
			r.ReferencedAreas = append(r.ReferencedAreas, a)
			r.ReferencedEntities = append(r.ReferencedEntities, f.areaEntities(a)...)
		} else {
			r.MissingAreas = append(r.MissingAreas, a)
		}
	}
	r.MissingLabels = slices.Clone(t.LabelID) // the fake knows no labels
	return r
}

func (f *readFake) areaEntities(areaID string) []string {
	var out []string
	for _, e := range f.registry {
		if e.AreaID == areaID {
			out = append(out, e.EntityID)
		}
	}
	return out
}
