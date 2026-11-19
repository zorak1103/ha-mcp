package handlers

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// defaultConfigScanTimeout bounds one shared config load, and with it the
// reference scans of analyze_entity and find_references. It must stay below the
// HTTP server's write timeout (httpWriteTimeout in internal/mcp/server.go, 30 s):
// a scan that outlasts it makes the server close the connection with an empty
// reply instead of an explicit result, and leaves no time to render the response.
const defaultConfigScanTimeout = 25 * time.Second

// loadDeadlineHeadroom is the time a load leaves its starting caller between the
// load's end and the caller's own deadline, so the caller still receives the
// configs read so far (and has time to scan them) instead of giving up first.
const loadDeadlineHeadroom = 2 * time.Second

// configFetchLimit caps the per-config fetches one source load keeps in flight.
const configFetchLimit = 8

// Source names of the config loads; the same words find_references reports in
// failed_sources.
const (
	configSourceAutomation     = "automation"
	configSourceScript         = "script"
	configSourceDashboard      = "dashboard"
	configSourceHelperTemplate = "helper_template"
)

// LoadedAutomation is one automation of a ConfigLoader load.
type LoadedAutomation struct {
	Entity homeassistant.Automation        // from ListAutomations (EntityID, State, FriendlyName, LastTriggered)
	Config *homeassistant.AutomationConfig // nil when Err != nil or HA returned no config
	Err    error                           // per-config fetch error that is not interruption
}

// LoadedScript is one script of a ConfigLoader load.
type LoadedScript struct {
	Entity homeassistant.Entity // from ListScripts (friendly_name in Attributes)
	Config *homeassistant.ScriptConfig
	Err    error
}

// LoadedDashboard is one dashboard of a ConfigLoader load.
type LoadedDashboard struct {
	URLPath string // "" = default dashboard
	Config  map[string]any
	Err     error
}

// LoadedHelperOptions is one template helper's config entry options.
type LoadedHelperOptions struct {
	EntityID string // template-platform registry entry with a config entry
	Options  map[string]any
	Err      error
}

// ConfigLoader shares in-flight config loads between concurrent callers that use
// the same client (pointer identity), so parallel reference scans list and fetch
// each source once. Nothing is cached: a load that has finished is forgotten, and
// the next call loads again. Loaded configs are shared between the callers of one
// load and must be treated as read-only.
//
// A load runs detached from its callers' cancellation, with a deadline taken from
// the caller that starts it (see loadBudget); callers that join later inherit the
// remaining deadline of the load they join. A load ends only when every fetch
// honors its ctx: a fetch that ignores ctx keeps the load, and the callers
// waiting for it, running until it returns or the caller's own ctx ends.
type ConfigLoader struct {
	group       singleflight.Group
	fetchLimit  int
	loadTimeout time.Duration
	// onJoin, when set, runs right after a caller started or joined a load (test hook).
	onJoin func()
}

// NewConfigLoader returns a ConfigLoader with the production fetch limit and load timeout.
func NewConfigLoader() *ConfigLoader {
	return newConfigLoader(configFetchLimit, defaultConfigScanTimeout)
}

func newConfigLoader(fetchLimit int, loadTimeout time.Duration) *ConfigLoader {
	return &ConfigLoader{fetchLimit: fetchLimit, loadTimeout: loadTimeout}
}

// errScanInterrupted returns a non-nil error when ctx has ended, so a scanner that
// walks configs one by one stops and reports its source as failed. Per-config
// fetch errors are otherwise skipped, and a fetch that failed only because ctx
// ended would silently drop that config and look like a complete scan.
func errScanInterrupted(ctx context.Context, source string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s scan interrupted before all configs were read: %w", source, err)
	}
	return nil
}

// Automations loads every automation and its config, in list order.
func (l *ConfigLoader) Automations(ctx context.Context, client homeassistant.Client) ([]LoadedAutomation, error) {
	return sharedLoad(ctx, l, client, configSourceAutomation, func(loadCtx context.Context) ([]LoadedAutomation, error) {
		list, err := client.ListAutomations(loadCtx)
		if err != nil {
			return nil, err
		}
		items := make([]LoadedAutomation, len(list))
		for i, a := range list {
			items[i].Entity = a
		}
		fetch := func(fetchCtx context.Context, it *LoadedAutomation) error {
			full, err := client.GetAutomation(fetchCtx, strings.TrimPrefix(it.Entity.EntityID, "automation."))
			if err == nil && full != nil {
				it.Config = full.Config
			}
			return err
		}
		return loadItems(loadCtx, l, configSourceAutomation, items, fetch, func(it *LoadedAutomation, err error) { it.Err = err })
	})
}

// Scripts loads every script and its config (script/config only), in list order.
func (l *ConfigLoader) Scripts(ctx context.Context, client homeassistant.Client) ([]LoadedScript, error) {
	return sharedLoad(ctx, l, client, configSourceScript, func(loadCtx context.Context) ([]LoadedScript, error) {
		list, err := client.ListScripts(loadCtx)
		if err != nil {
			return nil, err
		}
		items := make([]LoadedScript, len(list))
		for i, s := range list {
			items[i].Entity = s
		}
		fetch := func(fetchCtx context.Context, it *LoadedScript) error {
			cfg, err := client.GetScriptConfig(fetchCtx, it.Entity.EntityID)
			if err == nil {
				it.Config = cfg
			}
			return err
		}
		return loadItems(loadCtx, l, configSourceScript, items, fetch, func(it *LoadedScript, err error) { it.Err = err })
	})
}

// Dashboards loads the default dashboard followed by every registered dashboard, in list order.
func (l *ConfigLoader) Dashboards(ctx context.Context, client homeassistant.Client) ([]LoadedDashboard, error) {
	return sharedLoad(ctx, l, client, configSourceDashboard, func(loadCtx context.Context) ([]LoadedDashboard, error) {
		urlPaths, err := allDashboardURLPaths(loadCtx, client)
		if err != nil {
			return nil, err
		}
		items := make([]LoadedDashboard, len(urlPaths))
		for i, p := range urlPaths {
			items[i].URLPath = p
		}
		fetch := func(fetchCtx context.Context, it *LoadedDashboard) error {
			cfg, err := client.GetLovelaceConfig(fetchCtx, it.URLPath)
			if err == nil {
				it.Config = cfg
			}
			return err
		}
		return loadItems(loadCtx, l, configSourceDashboard, items, fetch, func(it *LoadedDashboard, err error) { it.Err = err })
	})
}

// HelperTemplateOptions loads the config entry options of every template-platform
// registry entry that has a config entry, in registry order.
func (l *ConfigLoader) HelperTemplateOptions(ctx context.Context, client homeassistant.Client) ([]LoadedHelperOptions, error) {
	return sharedLoad(ctx, l, client, configSourceHelperTemplate, func(loadCtx context.Context) ([]LoadedHelperOptions, error) {
		entries, err := client.GetEntityRegistry(loadCtx)
		if err != nil {
			return nil, err
		}
		var items []LoadedHelperOptions
		entryIDs := make(map[string]string)
		for _, e := range entries {
			if e.Platform != platformTemplate || e.ConfigEntryID == "" {
				continue
			}
			items = append(items, LoadedHelperOptions{EntityID: e.EntityID})
			entryIDs[e.EntityID] = e.ConfigEntryID
		}
		fetch := func(fetchCtx context.Context, it *LoadedHelperOptions) error {
			options, err := client.GetConfigEntryOptions(fetchCtx, entryIDs[it.EntityID])
			if err == nil {
				it.Options = options
			}
			return err
		}
		return loadItems(loadCtx, l, configSourceHelperTemplate, items, fetch, func(it *LoadedHelperOptions, err error) { it.Err = err })
	})
}

// sharedLoad starts or joins the load of source for client and waits for it or for
// ctx to end. The load's deadline comes from the starting caller's ctx (loadBudget),
// so it normally ends before that caller gives up and hands back the configs read
// so far. A caller that gives up gets errScanInterrupted while the load runs on for
// the other callers. Each caller gets its own copy of the item slice.
func sharedLoad[T any](ctx context.Context, l *ConfigLoader, client homeassistant.Client, source string, load func(context.Context) ([]T, error)) ([]T, error) {
	if err := errScanInterrupted(ctx, source); err != nil {
		return nil, err
	}
	key := source + "|" + fmt.Sprintf("%p", client)
	results := l.group.DoChan(key, func() (any, error) {
		deadline, hasDeadline := ctx.Deadline()
		budget := loadBudget(time.Until(deadline), hasDeadline, l.loadTimeout)
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), budget)
		defer cancel()
		items, err := runLoad(loadCtx, source, load)
		return items, err
	})
	if l.onJoin != nil {
		l.onJoin()
	}
	select {
	case res := <-results:
		items, _ := res.Val.([]T)
		return slices.Clone(items), res.Err
	case <-ctx.Done():
		return nil, errScanInterrupted(ctx, source)
	}
}

// loadBudget returns how long a load may run for a starting caller with
// callerRemaining time left: loadTimeout, cut to end loadDeadlineHeadroom before
// the caller's deadline, or to half the remaining time when that is shorter than
// the headroom. A caller without a deadline gets loadTimeout.
func loadBudget(callerRemaining time.Duration, hasDeadline bool, loadTimeout time.Duration) time.Duration {
	if !hasDeadline {
		return loadTimeout
	}
	budget := callerRemaining - loadDeadlineHeadroom
	if budget <= 0 {
		budget = callerRemaining / 2
	}
	return min(loadTimeout, budget)
}

// runLoad runs load and turns a panic into the load's error: singleflight.DoChan
// runs the load in its own goroutine, where an unrecovered panic ends the process.
func runLoad[T any](ctx context.Context, source string, load func(context.Context) ([]T, error)) (items []T, err error) {
	defer func() {
		if r := recover(); r != nil {
			items, err = nil, fmt.Errorf("%s load panicked: %v", source, r)
		}
	}()
	return load(ctx)
}

// loadItems fetches every item's config and records per-item errors with setErr.
// If ctx has ended when the fetches finish, it drops the items that were not
// fetched successfully and returns the rest, in order, with errScanInterrupted.
func loadItems[T any](ctx context.Context, l *ConfigLoader, source string, items []T, fetch func(context.Context, *T) error, setErr func(*T, error)) ([]T, error) {
	errs := fetchAll(ctx, l.fetchLimit, source, items, fetch)
	if ctx.Err() != nil {
		kept := make([]T, 0, len(items))
		for i := range items {
			if errs[i] == nil {
				kept = append(kept, items[i])
			}
		}
		return kept, errScanInterrupted(ctx, source)
	}
	for i, err := range errs {
		if err != nil {
			setErr(&items[i], err)
		}
	}
	return items, nil
}

// fetchAll runs fetch for every item with at most limit fetches in flight and
// returns each item's error. Once ctx is done it launches no more fetches; an item
// never fetched gets ctx's error.
func fetchAll[T any](ctx context.Context, limit int, source string, items []T, fetch func(context.Context, *T) error) []error {
	errs := make([]error, len(items))
	sem := make(chan struct{}, max(limit, 1))
	var wg sync.WaitGroup
	for i := range items {
		if !acquireFetchSlot(ctx, sem) {
			for j := i; j < len(items); j++ {
				errs[j] = ctx.Err()
			}
			break
		}
		wg.Go(func() {
			defer func() { <-sem }()
			errs[i] = runFetch(ctx, source, &items[i], fetch)
		})
	}
	wg.Wait()
	return errs
}

// acquireFetchSlot takes a slot of sem, or reports false once ctx is done.
func acquireFetchSlot(ctx context.Context, sem chan struct{}) bool {
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return false
	}
	if ctx.Err() != nil {
		<-sem
		return false
	}
	return true
}

// runFetch runs one fetch and turns a panic into that item's error.
func runFetch[T any](ctx context.Context, source string, item *T, fetch func(context.Context, *T) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s config fetch panicked: %v", source, r)
		}
	}()
	return fetch(ctx, item)
}
