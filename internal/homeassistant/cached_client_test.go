package homeassistant

import (
	"context"
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
	createHelperCallCount   int
	deleteHelperCallCount   int
	mu                      sync.Mutex
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

func (m *mockClient) CreateHelper(ctx context.Context, helper HelperConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createHelperCallCount++
	return nil
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
func (m *mockClient) CreateScene(ctx context.Context, sceneID string, scene SceneConfig) error {
	return nil
}
func (m *mockClient) UpdateScene(ctx context.Context, sceneID string, scene SceneConfig) error {
	return nil
}
func (m *mockClient) DeleteScene(ctx context.Context, sceneID string) error { return nil }
func (m *mockClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error) {
	return nil, nil
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
func (m *mockClient) GetLovelaceConfig(ctx context.Context) (map[string]any, error) { return nil, nil }
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
func (m *mockClient) GetScheduleConfig(ctx context.Context, scheduleID string) (map[string]any, error) {
	return nil, nil
}
func (m *mockClient) RenderTemplate(ctx context.Context, template string) (string, error) {
	return "", nil
}
func (m *mockClient) GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error) {
	return nil, nil
}
func (m *mockClient) CheckConfig(ctx context.Context) (*ConfigCheckResult, error) { return nil, nil }

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
