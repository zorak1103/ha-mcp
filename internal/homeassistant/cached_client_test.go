//nolint:revive // Mock client methods intentionally ignore ctx parameters for testing.
package homeassistant

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

// mockClient implements Client interface for testing caching behavior.
type mockClient struct {
	servicesCallCount       int
	configCallCount         int
	entityRegistryCallCount int
	deviceRegistryCallCount int
	areaRegistryCallCount   int
	createAreaCallCount     int
	updateAreaCallCount     int
	deleteAreaCallCount     int
	createHelperCallCount   int
	deleteHelperCallCount   int
	mu                      sync.Mutex

	configFileEntryExistsFn func(ctx context.Context, domain, configID string) (bool, error)
	createHelperEntityFn    func(ctx context.Context, helper HelperConfig) (string, error)
}

func (m *mockClient) GetServices(ctx context.Context) ([]Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servicesCallCount++
	return []Service{{Domain: "light"}}, nil
}

func (m *mockClient) GetConfig(ctx context.Context) (*Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configCallCount++
	return &Config{Version: "2024.1.0"}, nil
}

func (m *mockClient) GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entityRegistryCallCount++
	return []EntityRegistryEntry{{EntityID: "light.test"}}, nil
}

func (m *mockClient) GetEntityRegistryEntry(_ context.Context, entityID string) (*EntityRegistryEntry, error) {
	return &EntityRegistryEntry{EntityID: entityID}, nil
}

func (m *mockClient) GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deviceRegistryCallCount++
	return []DeviceRegistryEntry{{ID: "device1"}}, nil
}

func (m *mockClient) GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.areaRegistryCallCount++
	return []AreaRegistryEntry{{AreaID: "area1", Name: "Living Room"}}, nil
}

func (m *mockClient) CreateArea(ctx context.Context, config AreaConfig) (*AreaRegistryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createAreaCallCount++
	return &AreaRegistryEntry{AreaID: "area_new", Name: config.Name}, nil
}

func (m *mockClient) UpdateArea(ctx context.Context, areaID string, config AreaConfig) (*AreaRegistryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateAreaCallCount++
	return &AreaRegistryEntry{AreaID: areaID, Name: config.Name}, nil
}

func (m *mockClient) DeleteArea(ctx context.Context, areaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteAreaCallCount++
	return nil
}

func (m *mockClient) GetLabelRegistry(ctx context.Context) ([]LabelRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) CreateLabel(ctx context.Context, config LabelConfig) (*LabelRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) UpdateLabel(ctx context.Context, labelID string, config LabelConfig) (*LabelRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) DeleteLabel(ctx context.Context, labelID string) error {
	return nil
}

func (m *mockClient) GetFloorRegistry(_ context.Context) ([]FloorRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) CreateFloor(_ context.Context, _ FloorConfig) (*FloorRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) UpdateFloor(_ context.Context, _ string, _ FloorConfig) (*FloorRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) DeleteFloor(_ context.Context, _ string) error {
	return nil
}

func (m *mockClient) GetZones(_ context.Context) ([]ZoneRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) CreateZone(_ context.Context, _ ZoneConfig) (*ZoneRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) UpdateZone(_ context.Context, _ string, _ ZoneConfig) (*ZoneRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) DeleteZone(_ context.Context, _ string) error {
	return nil
}

func (m *mockClient) GetPersons(_ context.Context) ([]PersonRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) CreatePerson(_ context.Context, _ PersonConfig) (*PersonRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) UpdatePerson(_ context.Context, _ string, _ PersonConfig) (*PersonRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) DeletePerson(_ context.Context, _ string) error {
	return nil
}

func (m *mockClient) GetTags(_ context.Context) ([]TagRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) CreateTag(_ context.Context, _ TagConfig) (*TagRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) UpdateTag(_ context.Context, _ string, _ TagConfig) (*TagRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) DeleteTag(_ context.Context, _ string) error {
	return nil
}

func (m *mockClient) RemoveEntityRegistryEntry(ctx context.Context, entityID string) error {
	return nil
}

func (m *mockClient) UpdateEntityRegistryEntry(_ context.Context, _ string, _ EntityRegistryUpdateConfig) (*EntityRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) RemoveDeviceConfigEntry(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockClient) UpdateDeviceRegistryEntry(_ context.Context, _ string, _ DeviceRegistryUpdateConfig) (*DeviceRegistryEntry, error) {
	return nil, nil
}

func (m *mockClient) CreateHelper(ctx context.Context, helper HelperConfig) error {
	_, err := m.CreateHelperEntity(ctx, helper)
	return err
}

func (m *mockClient) CreateHelperEntity(ctx context.Context, helper HelperConfig) (string, error) {
	m.mu.Lock()
	m.createHelperCallCount++
	fn := m.createHelperEntityFn
	m.mu.Unlock()
	if fn != nil {
		return fn(ctx, helper)
	}
	return "", nil
}

func (m *mockClient) DeleteHelper(ctx context.Context, helperID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteHelperCallCount++
	return nil
}

// Stub implementations for remaining Client interface methods
func (m *mockClient) GetStates(ctx context.Context) ([]Entity, error) { return nil, nil }
func (m *mockClient) GetState(ctx context.Context, entityID string) (*Entity, error) {
	return nil, nil
}
func (m *mockClient) SetState(ctx context.Context, entityID string, state StateUpdate) (*Entity, error) {
	return nil, nil
}
func (m *mockClient) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error) {
	return nil, nil
}
func (m *mockClient) ListAutomations(ctx context.Context) ([]Automation, error) { return nil, nil }
func (m *mockClient) GetAutomation(ctx context.Context, automationID string) (*Automation, error) {
	return nil, nil
}
func (m *mockClient) CreateAutomation(ctx context.Context, automation AutomationConfig) error {
	return nil
}
func (m *mockClient) UpdateAutomation(ctx context.Context, automationID string, automation AutomationConfig) error {
	return nil
}
func (m *mockClient) DeleteAutomation(ctx context.Context, automationID string) error { return nil }
func (m *mockClient) ToggleAutomation(ctx context.Context, entityID string, enabled bool) error {
	return nil
}
func (m *mockClient) ListHelpers(ctx context.Context) ([]Entity, error) { return nil, nil }
func (m *mockClient) UpdateHelper(ctx context.Context, helperID string, helper HelperConfig) error {
	return nil
}
func (m *mockClient) SetHelperValue(ctx context.Context, entityID string, value any) error {
	return nil
}
func (m *mockClient) ListScripts(ctx context.Context) ([]Entity, error) { return nil, nil }
func (m *mockClient) GetScript(ctx context.Context, scriptID string) (*Script, error) {
	return nil, nil
}
func (m *mockClient) CreateScript(ctx context.Context, scriptID string, script ScriptConfig) error {
	return nil
}
func (m *mockClient) UpdateScript(ctx context.Context, scriptID string, script ScriptConfig) error {
	return nil
}
func (m *mockClient) DeleteScript(ctx context.Context, scriptID string) error { return nil }
func (m *mockClient) ListScenes(ctx context.Context) ([]Entity, error)        { return nil, nil }
func (m *mockClient) GetScene(ctx context.Context, sceneID string) (*Scene, error) {
	return nil, nil
}
func (m *mockClient) CreateScene(ctx context.Context, sceneID string, scene SceneConfig) error {
	return nil
}
func (m *mockClient) UpdateScene(ctx context.Context, sceneID string, scene SceneConfig) error {
	return nil
}
func (m *mockClient) DeleteScene(ctx context.Context, sceneID string) error { return nil }

func (m *mockClient) ConfigFileEntryExists(ctx context.Context, domain, configID string) (bool, error) {
	if m.configFileEntryExistsFn != nil {
		return m.configFileEntryExistsFn(ctx, domain, configID)
	}
	return true, nil
}
func (m *mockClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error) {
	return nil, nil
}
func (m *mockClient) CallServiceWithResponse(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error) {
	return nil, nil
}
func (m *mockClient) GetCalendars(ctx context.Context) ([]CalendarEntry, error) {
	return nil, nil
}
func (m *mockClient) GetCalendarEvents(ctx context.Context, entityID, start, end string) ([]CalendarEvent, error) {
	return nil, nil
}
func (m *mockClient) GetCameraSnapshot(ctx context.Context, entityID string) ([]byte, string, error) {
	return nil, "", nil
}
func (m *mockClient) SignPath(ctx context.Context, path string, expires int) (string, error) {
	return "", nil
}
func (m *mockClient) GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error) {
	return nil, nil
}
func (m *mockClient) BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error) {
	return nil, nil
}
func (m *mockClient) GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error) {
	return nil, nil
}

func (m *mockClient) SaveLovelaceConfig(ctx context.Context, urlPath string, config map[string]any) error {
	return nil
}

func (m *mockClient) ListDashboards(ctx context.Context) ([]DashboardEntry, error) {
	return nil, nil
}

func (m *mockClient) CreateDashboard(ctx context.Context, config DashboardConfig) (*DashboardEntry, error) {
	return nil, nil
}

func (m *mockClient) UpdateDashboard(ctx context.Context, dashboardID string, config DashboardConfig) (*DashboardEntry, error) {
	return nil, nil
}

func (m *mockClient) DeleteDashboard(ctx context.Context, dashboardID string) error {
	return nil
}
func (m *mockClient) GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error) {
	return nil, nil
}
func (m *mockClient) GetTriggersForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return nil, nil
}
func (m *mockClient) GetConditionsForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return nil, nil
}
func (m *mockClient) GetServicesForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	return nil, nil
}
func (m *mockClient) ExtractFromTarget(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error) {
	return nil, nil
}
func (m *mockClient) GetHelperConfig(ctx context.Context, platform, entityID string) (map[string]any, error) {
	return nil, nil
}
func (m *mockClient) RenderTemplate(ctx context.Context, template string) (string, error) {
	return "", nil
}
func (m *mockClient) GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error) {
	return nil, nil
}
func (m *mockClient) GetSystemLog(context.Context) ([]SystemLogEntry, error)      { return nil, nil }
func (m *mockClient) ClearSystemLog(context.Context) error                        { return nil }
func (m *mockClient) CheckConfig(ctx context.Context) (*ConfigCheckResult, error) { return nil, nil }
func (m *mockClient) GetConfigEntries(ctx context.Context, domain string) ([]ConfigEntryFull, error) {
	return nil, nil
}
func (m *mockClient) GetConfigEntry(ctx context.Context, entryID string) (*ConfigEntryFull, error) {
	return nil, nil
}

func (m *mockClient) GetConfigEntryOptions(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (m *mockClient) DeleteConfigEntry(ctx context.Context, entryID string) (bool, error) {
	return false, nil
}

func (m *mockClient) SendHACSCommand(ctx context.Context, command string, data map[string]any) (any, error) {
	return nil, nil
}

func TestCachedClient_CacheHit(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError) // Suppress logs during test
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// First call should hit the API
	_, err := client.GetServices(ctx)
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}
	if mock.servicesCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.servicesCallCount)
	}

	// Second call should use cache
	_, err = client.GetServices(ctx)
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}
	if mock.servicesCallCount != 1 {
		t.Errorf("Expected 1 API call (cached), got %d", mock.servicesCallCount)
	}
}

func TestCachedClient_CacheExpired(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  0, // Will use default but we'll manually expire
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	rawClient := NewCachedClient(mock, cfg, logger)
	client := rawClient.(*CachedClient)

	ctx := context.Background()

	// First call
	_, err := client.GetServices(ctx)
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}
	if mock.servicesCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.servicesCallCount)
	}

	// Manually expire the cache
	client.mu.Lock()
	client.servicesCache.expiresAt = time.Now().Add(-1 * time.Second)
	client.mu.Unlock()

	// Call should hit API again
	_, err = client.GetServices(ctx)
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}
	if mock.servicesCallCount != 2 {
		t.Errorf("Expected 2 API calls (cache expired), got %d", mock.servicesCallCount)
	}
}

func TestCachedClient_GetEntityRegistryEntry_PassesThroughUncached(t *testing.T) {
	t.Parallel()

	mock := &mockClient{}
	cfg := config.CacheConfig{Enabled: true, EntityRegTTLMin: 10}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()
	entry, err := client.GetEntityRegistryEntry(ctx, "automation.morning_routine")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil || entry.EntityID != "automation.morning_routine" {
		t.Errorf("got %+v, want EntityID = automation.morning_routine", entry)
	}
}

func TestCachedClient_InvalidationAfterCreateHelper(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Populate entity registry cache
	_, err := client.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatalf("GetEntityRegistry failed: %v", err)
	}
	if mock.entityRegistryCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.entityRegistryCallCount)
	}

	// Create a helper - should invalidate cache
	err = client.CreateHelper(ctx, HelperConfig{Platform: "input_boolean", ID: "test"})
	if err != nil {
		t.Fatalf("CreateHelper failed: %v", err)
	}

	// Next call should hit API again (cache invalidated)
	_, err = client.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatalf("GetEntityRegistry failed: %v", err)
	}
	if mock.entityRegistryCallCount != 2 {
		t.Errorf("Expected 2 API calls (cache invalidated), got %d", mock.entityRegistryCallCount)
	}
}

// TestCachedClient_InvalidationOnNonNilNonPartialErrorWithResolvedID guards
// CreateHelperEntity's defensive OR-clause: entityID != "" is checked
// independently of err == nil / errors.As(err, &PartialApplyError), since
// CachedClient wraps whatever Client it's given, not only HybridClient -
// any implementation that reports a non-empty entity id must still bust the
// cache even if it also returns a plain (non-partial) error.
func TestCachedClient_InvalidationOnNonNilNonPartialErrorWithResolvedID(t *testing.T) {
	mock := &mockClient{
		createHelperEntityFn: func(context.Context, HelperConfig) (string, error) {
			return "sensor.created_anyway", errors.New("some unrelated non-fatal error")
		},
	}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	if _, err := client.GetEntityRegistry(ctx); err != nil {
		t.Fatalf("GetEntityRegistry failed: %v", err)
	}
	if mock.entityRegistryCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.entityRegistryCallCount)
	}

	entityID, err := client.CreateHelperEntity(ctx, HelperConfig{Platform: "threshold", ID: "test"})
	if entityID != "sensor.created_anyway" {
		t.Errorf("expected entityID to pass through, got %q", entityID)
	}
	if err == nil {
		t.Fatal("expected the underlying error to pass through")
	}

	if _, err := client.GetEntityRegistry(ctx); err != nil {
		t.Fatalf("GetEntityRegistry failed: %v", err)
	}
	if mock.entityRegistryCallCount != 2 {
		t.Errorf("expected cache invalidation because entityID != \"\", got %d API calls", mock.entityRegistryCallCount)
	}
}

func TestCachedClient_InvalidationAfterDeleteHelper(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Populate device registry cache
	_, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		t.Fatalf("GetDeviceRegistry failed: %v", err)
	}
	if mock.deviceRegistryCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.deviceRegistryCallCount)
	}

	// Delete a helper - should invalidate cache
	err = client.DeleteHelper(ctx, "input_boolean.test")
	if err != nil {
		t.Fatalf("DeleteHelper failed: %v", err)
	}

	// Next call should hit API again (cache invalidated)
	_, err = client.GetDeviceRegistry(ctx)
	if err != nil {
		t.Fatalf("GetDeviceRegistry failed: %v", err)
	}
	if mock.deviceRegistryCallCount != 2 {
		t.Errorf("Expected 2 API calls (cache invalidated), got %d", mock.deviceRegistryCallCount)
	}
}

func TestCachedClient_ThreadSafety(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Run concurrent requests
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.GetServices(ctx)
			_, _ = client.GetConfig(ctx)
			_, _ = client.GetEntityRegistry(ctx)
			_, _ = client.GetDeviceRegistry(ctx)
			_, _ = client.GetAreaRegistry(ctx)
		}()
	}
	wg.Wait()

	// Each cache should have been populated exactly once initially
	// (with possible additional calls due to race conditions, but it should be minimal)
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.servicesCallCount > 5 { // Allow some slack for race conditions
		t.Errorf("Too many services API calls: %d (expected ~1-5)", mock.servicesCallCount)
	}
	if mock.configCallCount > 5 {
		t.Errorf("Too many config API calls: %d (expected ~1-5)", mock.configCallCount)
	}
}

func TestCachedClient_DisabledBypassesCache(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled: false, // Cache disabled
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	// When cache is disabled, NewCachedClient should return the underlying client
	if _, ok := client.(*CachedClient); ok {
		t.Error("Expected underlying client when cache disabled, got CachedClient")
	}
}

func TestCachedClient_AllCachedMethods(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Test all cached methods
	tests := []struct {
		name     string
		call     func() error
		getCount func() int
	}{
		{
			name: "GetServices",
			call: func() error {
				_, err := client.GetServices(ctx)
				return err
			},
			getCount: func() int { return mock.servicesCallCount },
		},
		{
			name: "GetConfig",
			call: func() error {
				_, err := client.GetConfig(ctx)
				return err
			},
			getCount: func() int { return mock.configCallCount },
		},
		{
			name: "GetEntityRegistry",
			call: func() error {
				_, err := client.GetEntityRegistry(ctx)
				return err
			},
			getCount: func() int { return mock.entityRegistryCallCount },
		},
		{
			name: "GetDeviceRegistry",
			call: func() error {
				_, err := client.GetDeviceRegistry(ctx)
				return err
			},
			getCount: func() int { return mock.deviceRegistryCallCount },
		},
		{
			name: "GetAreaRegistry",
			call: func() error {
				_, err := client.GetAreaRegistry(ctx)
				return err
			},
			getCount: func() int { return mock.areaRegistryCallCount },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First call
			if err := tt.call(); err != nil {
				t.Fatalf("%s failed: %v", tt.name, err)
			}
			initialCount := tt.getCount()
			if initialCount != 1 {
				t.Errorf("%s: expected 1 call initially, got %d", tt.name, initialCount)
			}

			// Second call (should be cached)
			if err := tt.call(); err != nil {
				t.Fatalf("%s failed: %v", tt.name, err)
			}
			cachedCount := tt.getCount()
			if cachedCount != 1 {
				t.Errorf("%s: expected 1 call (cached), got %d", tt.name, cachedCount)
			}
		})
	}
}

func TestCachedClient_InvalidationAfterCreateArea(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Populate area registry cache
	_, err := client.GetAreaRegistry(ctx)
	if err != nil {
		t.Fatalf("GetAreaRegistry failed: %v", err)
	}
	if mock.areaRegistryCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.areaRegistryCallCount)
	}

	// Create an area - should invalidate cache
	_, err = client.CreateArea(ctx, AreaConfig{Name: "New Room"})
	if err != nil {
		t.Fatalf("CreateArea failed: %v", err)
	}

	// Next call should hit API again (cache invalidated)
	_, err = client.GetAreaRegistry(ctx)
	if err != nil {
		t.Fatalf("GetAreaRegistry failed: %v", err)
	}
	if mock.areaRegistryCallCount != 2 {
		t.Errorf("Expected 2 API calls (cache invalidated), got %d", mock.areaRegistryCallCount)
	}
}

func TestCachedClient_InvalidationAfterUpdateArea(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Populate area registry cache
	_, err := client.GetAreaRegistry(ctx)
	if err != nil {
		t.Fatalf("GetAreaRegistry failed: %v", err)
	}
	if mock.areaRegistryCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.areaRegistryCallCount)
	}

	// Update an area - should invalidate cache
	_, err = client.UpdateArea(ctx, "living_room", AreaConfig{Name: "Living Room Updated"})
	if err != nil {
		t.Fatalf("UpdateArea failed: %v", err)
	}

	// Next call should hit API again (cache invalidated)
	_, err = client.GetAreaRegistry(ctx)
	if err != nil {
		t.Fatalf("GetAreaRegistry failed: %v", err)
	}
	if mock.areaRegistryCallCount != 2 {
		t.Errorf("Expected 2 API calls (cache invalidated), got %d", mock.areaRegistryCallCount)
	}
}

func TestCachedClient_InvalidationAfterDeleteArea(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Populate area registry cache
	_, err := client.GetAreaRegistry(ctx)
	if err != nil {
		t.Fatalf("GetAreaRegistry failed: %v", err)
	}
	if mock.areaRegistryCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.areaRegistryCallCount)
	}

	// Delete an area - should invalidate cache
	err = client.DeleteArea(ctx, "old_room")
	if err != nil {
		t.Fatalf("DeleteArea failed: %v", err)
	}

	// Next call should hit API again (cache invalidated)
	_, err = client.GetAreaRegistry(ctx)
	if err != nil {
		t.Fatalf("GetAreaRegistry failed: %v", err)
	}
	if mock.areaRegistryCallCount != 2 {
		t.Errorf("Expected 2 API calls (cache invalidated), got %d", mock.areaRegistryCallCount)
	}
}

func TestCachedClient_InvalidationAfterUpdateEntityRegistry(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Populate entity registry cache
	_, err := client.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatalf("GetEntityRegistry failed: %v", err)
	}
	if mock.entityRegistryCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.entityRegistryCallCount)
	}

	// Update an entity - should invalidate cache
	name := "New Name"
	_, err = client.UpdateEntityRegistryEntry(ctx, "light.living_room", EntityRegistryUpdateConfig{
		Name: &name,
	})
	if err != nil {
		t.Fatalf("UpdateEntityRegistryEntry failed: %v", err)
	}

	// Next call should hit API again (cache invalidated)
	_, err = client.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatalf("GetEntityRegistry failed: %v", err)
	}
	if mock.entityRegistryCallCount != 2 {
		t.Errorf("Expected 2 API calls (cache invalidated), got %d", mock.entityRegistryCallCount)
	}
}

func TestCachedClient_InvalidationAfterUpdateDeviceRegistry(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Populate device registry cache
	_, err := client.GetDeviceRegistry(ctx)
	if err != nil {
		t.Fatalf("GetDeviceRegistry failed: %v", err)
	}
	if mock.deviceRegistryCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.deviceRegistryCallCount)
	}

	// Update a device - should invalidate cache
	name := "New Device Name"
	_, err = client.UpdateDeviceRegistryEntry(ctx, "abc123", DeviceRegistryUpdateConfig{
		NameByUser: &name,
	})
	if err != nil {
		t.Fatalf("UpdateDeviceRegistryEntry failed: %v", err)
	}

	// Next call should hit API again (cache invalidated)
	_, err = client.GetDeviceRegistry(ctx)
	if err != nil {
		t.Fatalf("GetDeviceRegistry failed: %v", err)
	}
	if mock.deviceRegistryCallCount != 2 {
		t.Errorf("Expected 2 API calls (cache invalidated), got %d", mock.deviceRegistryCallCount)
	}
}

func TestCachedClient_InvalidationAfterDeleteConfigEntry(t *testing.T) {
	mock := &mockClient{}
	cfg := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  60,
		ConfigTTLMin:    30,
		EntityRegTTLMin: 10,
		DeviceRegTTLMin: 10,
		AreaRegTTLMin:   30,
	}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()

	// Populate entity registry cache
	_, err := client.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatalf("GetEntityRegistry failed: %v", err)
	}
	if mock.entityRegistryCallCount != 1 {
		t.Errorf("Expected 1 API call, got %d", mock.entityRegistryCallCount)
	}

	// Delete a config entry - should invalidate registry caches
	_, err = client.DeleteConfigEntry(ctx, "abc123")
	if err != nil {
		t.Fatalf("DeleteConfigEntry failed: %v", err)
	}

	// Next call should hit API again (cache invalidated)
	_, err = client.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatalf("GetEntityRegistry failed: %v", err)
	}
	if mock.entityRegistryCallCount != 2 {
		t.Errorf("Expected 2 API calls (cache invalidated), got %d", mock.entityRegistryCallCount)
	}
}

func TestCachedClient_ConfigFileEntryExists_NotCached(t *testing.T) {
	t.Parallel()
	callCount := 0
	mock := &mockClient{
		configFileEntryExistsFn: func(context.Context, string, string) (bool, error) {
			callCount++
			return true, nil
		},
	}
	cfg := config.CacheConfig{Enabled: true, ServicesTTLMin: 60, ConfigTTLMin: 30, EntityRegTTLMin: 10, DeviceRegTTLMin: 10, AreaRegTTLMin: 30}
	logger := logging.New(logging.LevelError)
	client := NewCachedClient(mock, cfg, logger)

	ctx := context.Background()
	if _, err := client.ConfigFileEntryExists(ctx, "automation", "morning_routine"); err != nil {
		t.Fatalf("ConfigFileEntryExists failed: %v", err)
	}
	if _, err := client.ConfigFileEntryExists(ctx, "automation", "morning_routine"); err != nil {
		t.Fatalf("ConfigFileEntryExists failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected every call to hit the underlying client (no caching), got %d calls for 2 requests", callCount)
	}
}
