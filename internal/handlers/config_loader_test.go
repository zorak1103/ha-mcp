package handlers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// testAutomations returns n automations named automation.a0 .. automation.a<n-1>.
func testAutomations(n int) []homeassistant.Automation {
	list := make([]homeassistant.Automation, n)
	for i := range list {
		list[i] = homeassistant.Automation{EntityID: fmt.Sprintf("automation.a%d", i)}
	}
	return list
}

// automationConfigFor returns a GetAutomation result whose config alias is the automation ID.
func automationConfigFor(automationID string) *homeassistant.Automation {
	return &homeassistant.Automation{
		EntityID: "automation." + automationID,
		Config:   &homeassistant.AutomationConfig{ID: automationID, Alias: automationID},
	}
}

// blockingAutomationClient returns a client whose ListAutomations blocks until release
// is closed, counting ListAutomations and GetAutomation calls.
func blockingAutomationClient(release <-chan struct{}, n int, lists, gets *atomic.Int32) *UniversalMockClient {
	return &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			lists.Add(1)
			<-release
			return testAutomations(n), nil
		},
		GetAutomationFn: func(_ context.Context, automationID string) (*homeassistant.Automation, error) {
			gets.Add(1)
			return automationConfigFor(automationID), nil
		},
	}
}

// joinSignal installs an onJoin hook on l that signals each join on the returned
// channel. The send never blocks, so a join past the generous buffer fails the
// test instead of hanging it; expectNoMoreJoins reports joins nobody waited for.
func joinSignal(t *testing.T, l *ConfigLoader) <-chan struct{} {
	t.Helper()
	joined := make(chan struct{}, 64)
	l.onJoin = func() {
		select {
		case joined <- struct{}{}:
		default:
			t.Error("more loader joins than the join buffer holds")
		}
	}
	return joined
}

// expectNoMoreJoins fails the test when joins arrived beyond those waitJoins consumed.
func expectNoMoreJoins(t *testing.T, joined <-chan struct{}) {
	t.Helper()
	if n := len(joined); n != 0 {
		t.Errorf("%d unexpected extra loader join(s)", n)
	}
}

func waitJoins(t *testing.T, joined <-chan struct{}, n int) {
	t.Helper()
	for range n {
		select {
		case <-joined:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for callers to join the load")
		}
	}
}

func TestConfigLoader_NewConfigLoaderDefaults(t *testing.T) {
	t.Parallel()

	l := NewConfigLoader()
	if l.fetchLimit != configFetchLimit || l.loadTimeout != defaultConfigScanTimeout {
		t.Errorf("NewConfigLoader() = limit %d timeout %v, want %d %v", l.fetchLimit, l.loadTimeout, configFetchLimit, defaultConfigScanTimeout)
	}
	if defaultConfigScanTimeout != 25*time.Second || configFetchLimit != 8 {
		t.Errorf("defaults changed: timeout %v limit %d", defaultConfigScanTimeout, configFetchLimit)
	}
}

func TestLoadBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		remaining   time.Duration
		hasDeadline bool
		loadTimeout time.Duration
		want        time.Duration
	}{
		{"no caller deadline uses the load timeout", 0, false, 25 * time.Second, 25 * time.Second},
		{"production scan deadline leaves headroom", 25 * time.Second, true, 25 * time.Second, 23 * time.Second},
		{"load timeout shorter than the caller's budget wins", 10 * time.Second, true, 50 * time.Millisecond, 50 * time.Millisecond},
		{"just above the headroom keeps the rest", 2500 * time.Millisecond, true, time.Minute, 500 * time.Millisecond},
		{"exactly the headroom halves the remaining time", 2 * time.Second, true, time.Minute, time.Second},
		{"below the headroom halves the remaining time", 600 * time.Millisecond, true, time.Minute, 300 * time.Millisecond},
		{"halved remaining time is still capped by the load timeout", time.Second, true, 100 * time.Millisecond, 100 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := loadBudget(tt.remaining, tt.hasDeadline, tt.loadTimeout); got != tt.want {
				t.Errorf("loadBudget(%v, %v, %v) = %v, want %v", tt.remaining, tt.hasDeadline, tt.loadTimeout, got, tt.want)
			}
		})
	}
}

func TestConfigLoader_AutomationsInListOrderWithPerItemErrors(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			return testAutomations(3), nil
		},
		GetAutomationFn: func(_ context.Context, automationID string) (*homeassistant.Automation, error) {
			if automationID == "a1" {
				return nil, errors.New("boom")
			}
			return automationConfigFor(automationID), nil
		},
	}

	items, err := newConfigLoader(8, time.Minute).Automations(context.Background(), client)
	if err != nil {
		t.Fatalf("Automations() error = %v, want nil", err)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	for i, it := range items {
		if want := fmt.Sprintf("automation.a%d", i); it.Entity.EntityID != want {
			t.Errorf("items[%d].Entity.EntityID = %q, want %q", i, it.Entity.EntityID, want)
		}
	}
	if items[1].Err == nil || !strings.Contains(items[1].Err.Error(), "boom") {
		t.Errorf("items[1].Err = %v, want boom", items[1].Err)
	}
	if items[1].Config != nil {
		t.Errorf("items[1].Config = %+v, want nil", items[1].Config)
	}
	for _, i := range []int{0, 2} {
		if items[i].Err != nil || items[i].Config == nil || items[i].Config.ID != fmt.Sprintf("a%d", i) {
			t.Errorf("items[%d] = %+v, want config a%d without error", i, items[i], i)
		}
	}
}

func TestConfigLoader_ScriptsUseGetScriptConfigOnly(t *testing.T) {
	t.Parallel()

	var configCalls atomic.Int32
	client := &UniversalMockClient{
		ListScriptsFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{{EntityID: "script.one"}, {EntityID: "script.two"}, {EntityID: "script.three"}}, nil
		},
		GetScriptFn: func(context.Context, string) (*homeassistant.Script, error) {
			t.Error("GetScript must not be called")
			return nil, errors.New("unexpected GetScript")
		},
		GetStateFn: func(context.Context, string) (*homeassistant.Entity, error) {
			t.Error("GetState must not be called")
			return nil, errors.New("unexpected GetState")
		},
		GetStatesFn: func(context.Context) ([]homeassistant.Entity, error) {
			t.Error("GetStates must not be called")
			return nil, errors.New("unexpected GetStates")
		},
		GetScriptConfigFn: func(_ context.Context, entityID string) (*homeassistant.ScriptConfig, error) {
			configCalls.Add(1)
			return &homeassistant.ScriptConfig{Alias: entityID}, nil
		},
	}

	items, err := newConfigLoader(8, time.Minute).Scripts(context.Background(), client)
	if err != nil {
		t.Fatalf("Scripts() error = %v", err)
	}
	if got := configCalls.Load(); got != 3 {
		t.Errorf("GetScriptConfig calls = %d, want 3", got)
	}
	want := []string{"script.one", "script.two", "script.three"}
	if len(items) != len(want) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(want))
	}
	for i, id := range want {
		if items[i].Entity.EntityID != id || items[i].Config == nil || items[i].Config.Alias != id || items[i].Err != nil {
			t.Errorf("items[%d] = %+v, want %s with its config", i, items[i], id)
		}
	}
}

func TestConfigLoader_ParallelCallsSameClientShareOneLoad(t *testing.T) {
	t.Parallel()

	var lists, gets atomic.Int32
	release := make(chan struct{})
	client := blockingAutomationClient(release, 3, &lists, &gets)
	l := newConfigLoader(8, time.Minute)
	joined := joinSignal(t, l)

	var wg sync.WaitGroup
	results := make([][]LoadedAutomation, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Go(func() { results[i], errs[i] = l.Automations(context.Background(), client) })
	}
	waitJoins(t, joined, 2)
	close(release)
	wg.Wait()
	expectNoMoreJoins(t, joined)

	if got := lists.Load(); got != 1 {
		t.Errorf("ListAutomations calls = %d, want 1", got)
	}
	if got := gets.Load(); got != 3 {
		t.Errorf("GetAutomation calls = %d, want 3", got)
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d error = %v", i, err)
		}
	}
	if len(results[0]) != 3 || !reflect.DeepEqual(results[0], results[1]) {
		t.Errorf("callers got different results: %+v vs %+v", results[0], results[1])
	}
	// Each caller gets its own slice: writing one caller's item leaves the other's alone.
	results[0][0].Err = errors.New("mutated by caller 0")
	if results[1][0].Err != nil {
		t.Errorf("caller 1 item 0 Err = %v after caller 0 changed its own copy, want nil", results[1][0].Err)
	}
}

func TestConfigLoader_DifferentClientsDoNotShare(t *testing.T) {
	t.Parallel()

	var lists1, lists2, gets1, gets2 atomic.Int32
	release := make(chan struct{})
	c1 := blockingAutomationClient(release, 2, &lists1, &gets1)
	c2 := blockingAutomationClient(release, 2, &lists2, &gets2)
	l := newConfigLoader(8, time.Minute)
	joined := joinSignal(t, l)

	var wg sync.WaitGroup
	for _, c := range []*UniversalMockClient{c1, c2} {
		wg.Go(func() {
			if _, err := l.Automations(context.Background(), c); err != nil {
				t.Errorf("Automations() error = %v", err)
			}
		})
	}
	waitJoins(t, joined, 2)
	close(release)
	wg.Wait()
	expectNoMoreJoins(t, joined)

	if lists1.Load() != 1 || lists2.Load() != 1 {
		t.Errorf("ListAutomations calls = %d/%d, want 1/1", lists1.Load(), lists2.Load())
	}
	if gets1.Load() != 2 || gets2.Load() != 2 {
		t.Errorf("GetAutomation calls = %d/%d, want 2/2", gets1.Load(), gets2.Load())
	}
}

func TestConfigLoader_CallerGivingUpDoesNotCancelSharedLoad(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	var fetchCtxErrs sync.Map
	client := &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			<-release
			return testAutomations(2), nil
		},
		GetAutomationFn: func(ctx context.Context, automationID string) (*homeassistant.Automation, error) {
			fetchCtxErrs.Store(automationID, ctx.Err())
			return automationConfigFor(automationID), nil
		},
	}
	l := newConfigLoader(8, time.Minute)
	joined := joinSignal(t, l)

	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()
	var errA, errB error
	var itemsB []LoadedAutomation
	doneA := make(chan struct{})
	var wg sync.WaitGroup
	go func() {
		defer close(doneA)
		_, errA = l.Automations(ctxA, client)
	}()
	wg.Go(func() { itemsB, errB = l.Automations(context.Background(), client) })
	waitJoins(t, joined, 2)

	cancelA()
	<-doneA
	if errA == nil || !errors.Is(errA, context.Canceled) || !strings.Contains(errA.Error(), "automation scan interrupted") {
		t.Errorf("caller A error = %v, want an automation scan interrupted error wrapping context.Canceled", errA)
	}

	close(release)
	wg.Wait()
	expectNoMoreJoins(t, joined)
	if errB != nil {
		t.Errorf("caller B error = %v, want nil", errB)
	}
	if len(itemsB) != 2 {
		t.Fatalf("caller B got %d items, want 2", len(itemsB))
	}
	for _, it := range itemsB {
		if it.Config == nil || it.Err != nil {
			t.Errorf("caller B item %+v, want config without error", it)
		}
	}
	fetchCtxErrs.Range(func(key, value any) bool {
		if value != nil {
			t.Errorf("load ctx for %v was done: %v", key, value)
		}
		return true
	})
}

func TestConfigLoader_LoadDeadlineKeepsFetchedItems(t *testing.T) {
	t.Parallel()

	var gets atomic.Int32
	client := &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			return testAutomations(3), nil
		},
		GetAutomationFn: func(ctx context.Context, automationID string) (*homeassistant.Automation, error) {
			gets.Add(1)
			if automationID == "a0" {
				return automationConfigFor(automationID), nil
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	items, err := newConfigLoader(1, 200*time.Millisecond).Automations(context.Background(), client)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Automations() error = %v, want one wrapping context.DeadlineExceeded", err)
	}
	if !strings.Contains(err.Error(), "automation scan interrupted") {
		t.Errorf("error = %q, want automation scan interrupted", err)
	}
	if len(items) != 1 || items[0].Entity.EntityID != "automation.a0" || items[0].Config == nil {
		t.Errorf("items = %+v, want only the fetched automation.a0", items)
	}
	if got := gets.Load(); got != 2 {
		t.Errorf("GetAutomation calls = %d, want 2 (no fetch launched after the deadline)", got)
	}
}

// TestConfigLoader_CallerDeadlineKeepsPartialItems uses production-like ordering:
// the loader's own timeout outlasts the caller's deadline, so the load must end
// before the caller gives up for the caller to get the configs read so far.
func TestConfigLoader_CallerDeadlineKeepsPartialItems(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			return testAutomations(2), nil
		},
		GetAutomationFn: func(ctx context.Context, automationID string) (*homeassistant.Automation, error) {
			if automationID == "a0" {
				return automationConfigFor(automationID), nil
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	items, err := newConfigLoader(1, time.Minute).Automations(ctx, client)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "automation scan interrupted") {
		t.Errorf("Automations() error = %v, want an automation scan interrupted error wrapping context.DeadlineExceeded", err)
	}
	if len(items) != 1 || items[0].Entity.EntityID != "automation.a0" || items[0].Config == nil {
		t.Errorf("items = %+v, want the automation.a0 read before the deadline", items)
	}
}

func TestConfigLoader_BoundsConcurrentFetches(t *testing.T) {
	t.Parallel()

	var inFlight, maxInFlight atomic.Int32
	client := &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			return testAutomations(6), nil
		},
		GetAutomationFn: func(_ context.Context, automationID string) (*homeassistant.Automation, error) {
			n := inFlight.Add(1)
			defer inFlight.Add(-1)
			for {
				cur := maxInFlight.Load()
				if n <= cur || maxInFlight.CompareAndSwap(cur, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			return automationConfigFor(automationID), nil
		},
	}

	items, err := newConfigLoader(2, time.Minute).Automations(context.Background(), client)
	if err != nil || len(items) != 6 {
		t.Fatalf("Automations() = %d items, %v; want 6, nil", len(items), err)
	}
	if got := maxInFlight.Load(); got > 2 {
		t.Errorf("max in-flight fetches = %d, want <= 2", got)
	}
}

func TestConfigLoader_DashboardsIncludeDefault(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		ListDashboardsFn: func(context.Context) ([]homeassistant.DashboardEntry, error) {
			return []homeassistant.DashboardEntry{{URLPath: "dash-a"}, {URLPath: "dash-b"}}, nil
		},
		GetLovelaceConfigFn: func(_ context.Context, urlPath string) (map[string]any, error) {
			if urlPath == "dash-b" {
				return nil, errors.New("no config")
			}
			return map[string]any{"title": urlPath}, nil
		},
	}

	items, err := newConfigLoader(8, time.Minute).Dashboards(context.Background(), client)
	if err != nil {
		t.Fatalf("Dashboards() error = %v", err)
	}
	want := []string{"", "dash-a", "dash-b"}
	if len(items) != len(want) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(want))
	}
	for i, p := range want {
		if items[i].URLPath != p {
			t.Errorf("items[%d].URLPath = %q, want %q", i, items[i].URLPath, p)
		}
	}
	if items[0].Config["title"] != "" || items[1].Config["title"] != "dash-a" {
		t.Errorf("configs = %v / %v", items[0].Config, items[1].Config)
	}
	if items[2].Err == nil || items[2].Config != nil {
		t.Errorf("items[2] = %+v, want fetch error", items[2])
	}
}

func TestConfigLoader_HelperTemplateOptionsFiltersTemplatePlatform(t *testing.T) {
	t.Parallel()

	var fetched []string
	var mu sync.Mutex
	client := &UniversalMockClient{
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
			return []homeassistant.EntityRegistryEntry{
				{EntityID: "sensor.t1", Platform: platformTemplate, ConfigEntryID: "e1"},
				{EntityID: "sensor.yaml_template", Platform: platformTemplate},
				{EntityID: "sensor.hue", Platform: "hue", ConfigEntryID: "e2"},
				{EntityID: "binary_sensor.t2", Platform: platformTemplate, ConfigEntryID: "e3"},
			}, nil
		},
		GetConfigEntryOptionsFn: func(_ context.Context, entryID string) (map[string]any, error) {
			mu.Lock()
			fetched = append(fetched, entryID)
			mu.Unlock()
			return map[string]any{"entry": entryID}, nil
		},
	}

	items, err := newConfigLoader(8, time.Minute).HelperTemplateOptions(context.Background(), client)
	if err != nil {
		t.Fatalf("HelperTemplateOptions() error = %v", err)
	}
	if len(items) != 2 || items[0].EntityID != "sensor.t1" || items[1].EntityID != "binary_sensor.t2" {
		t.Fatalf("items = %+v, want sensor.t1 then binary_sensor.t2", items)
	}
	if items[0].Options["entry"] != "e1" || items[1].Options["entry"] != "e3" {
		t.Errorf("options = %v / %v", items[0].Options, items[1].Options)
	}
	if len(fetched) != 2 {
		t.Errorf("fetched options for %v, want only e1 and e3", fetched)
	}
}

func TestConfigLoader_ListErrorsAreReturned(t *testing.T) {
	t.Parallel()

	listErr := errors.New("list failed")
	client := &UniversalMockClient{
		ListAutomationsFn:   func(context.Context) ([]homeassistant.Automation, error) { return nil, listErr },
		ListScriptsFn:       func(context.Context) ([]homeassistant.Entity, error) { return nil, listErr },
		ListDashboardsFn:    func(context.Context) ([]homeassistant.DashboardEntry, error) { return nil, listErr },
		GetEntityRegistryFn: func(context.Context) ([]homeassistant.EntityRegistryEntry, error) { return nil, listErr },
	}
	l := newConfigLoader(8, time.Minute)
	ctx := context.Background()

	checks := map[string]func() (int, error){
		"automation": func() (int, error) { items, err := l.Automations(ctx, client); return len(items), err },
		"script":     func() (int, error) { items, err := l.Scripts(ctx, client); return len(items), err },
		"dashboard":  func() (int, error) { items, err := l.Dashboards(ctx, client); return len(items), err },
		"helper":     func() (int, error) { items, err := l.HelperTemplateOptions(ctx, client); return len(items), err },
	}
	for name, check := range checks {
		n, err := check()
		if !errors.Is(err, listErr) || n != 0 {
			t.Errorf("%s: %d items, error %v; want 0 items and the list error", name, n, err)
		}
	}
}

func TestConfigLoader_PanicBecomesError(t *testing.T) {
	t.Parallel()

	t.Run("list panic", func(t *testing.T) {
		t.Parallel()
		client := &UniversalMockClient{
			ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) { panic("list exploded") },
		}
		items, err := newConfigLoader(8, time.Minute).Automations(context.Background(), client)
		if err == nil || !strings.Contains(err.Error(), "automation load panicked") || items != nil {
			t.Errorf("Automations() = %v, %v; want nil items and an automation load panicked error", items, err)
		}
	})

	t.Run("fetch panic", func(t *testing.T) {
		t.Parallel()
		client := &UniversalMockClient{
			ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) { return testAutomations(3), nil },
			GetAutomationFn: func(_ context.Context, automationID string) (*homeassistant.Automation, error) {
				if automationID == "a1" {
					panic("fetch exploded")
				}
				return automationConfigFor(automationID), nil
			},
		}
		items, err := newConfigLoader(8, time.Minute).Automations(context.Background(), client)
		if err != nil || len(items) != 3 {
			t.Fatalf("Automations() = %d items, %v; want 3 items, nil", len(items), err)
		}
		if items[1].Err == nil || !strings.Contains(items[1].Err.Error(), "fetch exploded") {
			t.Errorf("items[1].Err = %v, want the recovered panic", items[1].Err)
		}
		if items[0].Err != nil || items[2].Err != nil || items[0].Config == nil || items[2].Config == nil {
			t.Errorf("items 0/2 = %+v / %+v, want configs without errors", items[0], items[2])
		}
	})
}

func TestConfigLoader_DoneContextDoesNotStartLoad(t *testing.T) {
	t.Parallel()

	var lists atomic.Int32
	client := &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			lists.Add(1)
			return testAutomations(1), nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items, err := newConfigLoader(8, time.Minute).Automations(ctx, client)
	if !errors.Is(err, context.Canceled) || items != nil {
		t.Errorf("Automations() = %v, %v; want nil items and context.Canceled", items, err)
	}
	if got := lists.Load(); got != 0 {
		t.Errorf("ListAutomations calls = %d, want 0", got)
	}
}

func TestConfigLoader_EmptySourceReturnsNoItems(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	l := newConfigLoader(8, time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if items, err := l.Automations(ctx, client); err != nil || len(items) != 0 {
			t.Errorf("Automations() = %v, %v", items, err)
		}
		if items, err := l.Scripts(ctx, client); err != nil || len(items) != 0 {
			t.Errorf("Scripts() = %v, %v", items, err)
		}
		if items, err := l.Dashboards(ctx, client); err != nil || len(items) != 1 || items[0].URLPath != "" {
			t.Errorf("Dashboards() = %v, %v; want only the default dashboard", items, err)
		}
		if items, err := l.HelperTemplateOptions(ctx, client); err != nil || len(items) != 0 {
			t.Errorf("HelperTemplateOptions() = %v, %v", items, err)
		}
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("empty sources did not return within 2 s")
	}
}

func TestConfigLoader_NilResultsAreSkippable(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) { return testAutomations(1), nil },
		GetAutomationFn:   func(context.Context, string) (*homeassistant.Automation, error) { return nil, nil },
		ListScriptsFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{{EntityID: "script.one"}}, nil
		},
		GetScriptConfigFn: func(context.Context, string) (*homeassistant.ScriptConfig, error) { return nil, nil },
	}
	l := newConfigLoader(8, time.Minute)

	autos, err := l.Automations(context.Background(), client)
	if err != nil || len(autos) != 1 || autos[0].Config != nil || autos[0].Err != nil {
		t.Errorf("Automations() = %+v, %v; want one item without config or error", autos, err)
	}
	scripts, err := l.Scripts(context.Background(), client)
	if err != nil || len(scripts) != 1 || scripts[0].Config != nil || scripts[0].Err != nil {
		t.Errorf("Scripts() = %+v, %v; want one item without config or error", scripts, err)
	}
}

func TestConfigLoader_SequentialCallsReload(t *testing.T) {
	t.Parallel()

	var lists atomic.Int32
	client := &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			lists.Add(1)
			return testAutomations(1), nil
		},
	}
	l := newConfigLoader(8, time.Minute)
	for range 2 {
		if _, err := l.Automations(context.Background(), client); err != nil {
			t.Fatalf("Automations() error = %v", err)
		}
	}
	if got := lists.Load(); got != 2 {
		t.Errorf("ListAutomations calls = %d, want 2", got)
	}
}
