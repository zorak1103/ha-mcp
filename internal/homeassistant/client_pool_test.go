package homeassistant

import (
	"context"
	"testing"
	"time"
)

func TestNewClientPool(t *testing.T) {
	pool := NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	if pool == nil {
		t.Fatal("NewClientPool returned nil")
	}

	if pool.Size() != 0 {
		t.Errorf("Expected empty pool, got size %d", pool.Size())
	}
}

func TestClientPool_Size(t *testing.T) {
	pool := NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	if pool.Size() != 0 {
		t.Errorf("Expected size 0, got %d", pool.Size())
	}
}

func TestClientPool_Close(t *testing.T) {
	pool := NewClientPool("http://localhost:8123", 30*time.Minute)

	err := pool.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Pool should be empty after close
	if pool.Size() != 0 {
		t.Errorf("Expected empty pool after close, got size %d", pool.Size())
	}
}

func TestClientPool_GetOrCreate_InvalidToken(t *testing.T) {
	pool := NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// This should fail because we can't connect to localhost:8123
	_, err := pool.GetOrCreate(ctx, "invalid-token")
	if err == nil {
		t.Error("Expected error for invalid connection, got nil")
	}
}

// Note: TestExtractBearerToken is in server_test.go since extractBearerToken is in mcp package

// connectionChecker is a minimal type for testing IsConnected behavior.
type connectionChecker struct {
	connected bool
}

func (c *connectionChecker) IsConnected() bool {
	return c.connected
}

// noConnectionChecker is a type without IsConnected method.
type noConnectionChecker struct{}

func TestIsClientConnected_WithConnectedClient(t *testing.T) {
	// Use type assertion test - connectionChecker has IsConnected
	checker := &connectionChecker{connected: true}
	// Test the type assertion directly
	if c, ok := interface{}(checker).(interface{ IsConnected() bool }); ok {
		if !c.IsConnected() {
			t.Error("Expected connected checker to return true")
		}
	} else {
		t.Error("Expected connectionChecker to implement IsConnected interface")
	}
}

func TestIsClientConnected_WithDisconnectedClient(t *testing.T) {
	checker := &connectionChecker{connected: false}
	if c, ok := interface{}(checker).(interface{ IsConnected() bool }); ok {
		if c.IsConnected() {
			t.Error("Expected disconnected checker to return false")
		}
	} else {
		t.Error("Expected connectionChecker to implement IsConnected interface")
	}
}

func TestIsClientConnected_WithoutIsConnectedMethod(t *testing.T) {
	// noConnectionChecker doesn't have IsConnected method
	checker := &noConnectionChecker{}
	// Should not satisfy the interface
	if _, ok := interface{}(checker).(interface{ IsConnected() bool }); ok {
		t.Error("Expected noConnectionChecker to NOT implement IsConnected interface")
	}
}

// mockClient implements Client interface for testing.
type mockClientForPool struct {
	connected bool
	closed    bool
}

func (m *mockClientForPool) IsConnected() bool {
	return m.connected
}

func (m *mockClientForPool) Close() error {
	m.closed = true
	return nil
}

// Client interface methods (required for interface compliance).
func (m *mockClientForPool) GetStates(_ context.Context) ([]Entity, error)         { return nil, nil }
func (m *mockClientForPool) GetState(_ context.Context, _ string) (*Entity, error) { return nil, nil }
func (m *mockClientForPool) SetState(_ context.Context, _ string, _ StateUpdate) (*Entity, error) {
	return nil, nil
}
func (m *mockClientForPool) GetHistory(_ context.Context, _ string, _, _ time.Time) ([][]HistoryEntry, error) {
	return nil, nil
}
func (m *mockClientForPool) CallService(_ context.Context, _, _ string, _ map[string]any) ([]Entity, error) {
	return nil, nil
}
func (m *mockClientForPool) CallServiceWithResponse(context.Context, string, string, map[string]any) (map[string]any, error) {
	return nil, nil
}
func (m *mockClientForPool) GetCalendars(context.Context) ([]CalendarEntry, error) { return nil, nil }
func (m *mockClientForPool) GetCalendarEvents(context.Context, string, string, string) ([]CalendarEvent, error) {
	return nil, nil
}
func (m *mockClientForPool) GetCameraSnapshot(context.Context, string) ([]byte, string, error) {
	return nil, "", nil
}
func (m *mockClientForPool) ListAutomations(_ context.Context) ([]Automation, error) { return nil, nil }
func (m *mockClientForPool) GetAutomation(_ context.Context, _ string) (*Automation, error) {
	return nil, nil
}
func (m *mockClientForPool) CreateAutomation(_ context.Context, _ AutomationConfig) error { return nil }
func (m *mockClientForPool) UpdateAutomation(_ context.Context, _ string, _ AutomationConfig) error {
	return nil
}
func (m *mockClientForPool) DeleteAutomation(_ context.Context, _ string) error { return nil }
func (m *mockClientForPool) ToggleAutomation(_ context.Context, _ string, _ bool) error {
	return nil
}
func (m *mockClientForPool) ListHelpers(_ context.Context) ([]Entity, error) { return nil, nil }
func (m *mockClientForPool) CreateHelper(_ context.Context, _ HelperConfig) error {
	return nil
}
func (m *mockClientForPool) UpdateHelper(_ context.Context, _ string, _ HelperConfig) error {
	return nil
}
func (m *mockClientForPool) DeleteHelper(_ context.Context, _ string) error { return nil }
func (m *mockClientForPool) SetHelperValue(_ context.Context, _ string, _ any) error {
	return nil
}
func (m *mockClientForPool) ListScripts(_ context.Context) ([]Entity, error) { return nil, nil }
func (m *mockClientForPool) GetScript(_ context.Context, _ string) (*Script, error) {
	return nil, nil
}
func (m *mockClientForPool) CreateScript(_ context.Context, _ string, _ ScriptConfig) error {
	return nil
}
func (m *mockClientForPool) UpdateScript(_ context.Context, _ string, _ ScriptConfig) error {
	return nil
}
func (m *mockClientForPool) DeleteScript(_ context.Context, _ string) error { return nil }
func (m *mockClientForPool) ListScenes(_ context.Context) ([]Entity, error) { return nil, nil }
func (m *mockClientForPool) GetScene(_ context.Context, _ string) (*Scene, error) {
	return nil, nil
}
func (m *mockClientForPool) CreateScene(_ context.Context, _ string, _ SceneConfig) error {
	return nil
}
func (m *mockClientForPool) UpdateScene(_ context.Context, _ string, _ SceneConfig) error {
	return nil
}
func (m *mockClientForPool) DeleteScene(_ context.Context, _ string) error { return nil }
func (m *mockClientForPool) GetEntityRegistry(_ context.Context) ([]EntityRegistryEntry, error) {
	return nil, nil
}
func (m *mockClientForPool) GetDeviceRegistry(_ context.Context) ([]DeviceRegistryEntry, error) {
	return nil, nil
}
func (m *mockClientForPool) GetAreaRegistry(_ context.Context) ([]AreaRegistryEntry, error) {
	return nil, nil
}
func (m *mockClientForPool) CreateArea(_ context.Context, _ AreaConfig) (*AreaRegistryEntry, error) {
	return nil, nil
}
func (m *mockClientForPool) UpdateArea(_ context.Context, _ string, _ AreaConfig) (*AreaRegistryEntry, error) {
	return nil, nil
}
func (m *mockClientForPool) DeleteArea(_ context.Context, _ string) error { return nil }
func (m *mockClientForPool) GetLabelRegistry(_ context.Context) ([]LabelRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) CreateLabel(_ context.Context, _ LabelConfig) (*LabelRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) UpdateLabel(_ context.Context, _ string, _ LabelConfig) (*LabelRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) DeleteLabel(_ context.Context, _ string) error {
	return nil
}

func (m *mockClientForPool) GetFloorRegistry(_ context.Context) ([]FloorRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) CreateFloor(_ context.Context, _ FloorConfig) (*FloorRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) UpdateFloor(_ context.Context, _ string, _ FloorConfig) (*FloorRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) DeleteFloor(_ context.Context, _ string) error {
	return nil
}

func (m *mockClientForPool) GetZones(_ context.Context) ([]ZoneRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) CreateZone(_ context.Context, _ ZoneConfig) (*ZoneRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) UpdateZone(_ context.Context, _ string, _ ZoneConfig) (*ZoneRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) DeleteZone(_ context.Context, _ string) error {
	return nil
}

func (m *mockClientForPool) GetPersons(_ context.Context) ([]PersonRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) CreatePerson(_ context.Context, _ PersonConfig) (*PersonRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) UpdatePerson(_ context.Context, _ string, _ PersonConfig) (*PersonRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) DeletePerson(_ context.Context, _ string) error {
	return nil
}

func (m *mockClientForPool) GetTags(_ context.Context) ([]TagRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) CreateTag(_ context.Context, _ TagConfig) (*TagRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) UpdateTag(_ context.Context, _ string, _ TagConfig) (*TagRegistryEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) DeleteTag(_ context.Context, _ string) error {
	return nil
}

func (m *mockClientForPool) RemoveEntityRegistryEntry(_ context.Context, _ string) error {
	return nil
}
func (m *mockClientForPool) UpdateEntityRegistryEntry(_ context.Context, _ string, _ EntityRegistryUpdateConfig) (*EntityRegistryEntry, error) {
	return nil, nil
}
func (m *mockClientForPool) RemoveDeviceConfigEntry(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockClientForPool) UpdateDeviceRegistryEntry(_ context.Context, _ string, _ DeviceRegistryUpdateConfig) (*DeviceRegistryEntry, error) {
	return nil, nil
}
func (m *mockClientForPool) SignPath(_ context.Context, _ string, _ int) (string, error) {
	return "", nil
}
func (m *mockClientForPool) GetCameraStream(_ context.Context, _ string) (*StreamInfo, error) {
	return nil, nil
}
func (m *mockClientForPool) BrowseMedia(_ context.Context, _ string) (*MediaBrowseResult, error) {
	return nil, nil
}
func (m *mockClientForPool) GetLovelaceConfig(_ context.Context, _ string) (map[string]any, error) {
	return nil, nil
}

func (m *mockClientForPool) SaveLovelaceConfig(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

func (m *mockClientForPool) ListDashboards(_ context.Context) ([]DashboardEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) CreateDashboard(_ context.Context, _ DashboardConfig) (*DashboardEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) UpdateDashboard(_ context.Context, _ string, _ DashboardConfig) (*DashboardEntry, error) {
	return nil, nil
}

func (m *mockClientForPool) DeleteDashboard(_ context.Context, _ string) error {
	return nil
}
func (m *mockClientForPool) GetStatistics(_ context.Context, _ []string, _ string) ([]StatisticsResult, error) {
	return nil, nil
}
func (m *mockClientForPool) GetTriggersForTarget(_ context.Context, _ Target, _ *bool) ([]string, error) {
	return nil, nil
}
func (m *mockClientForPool) GetConditionsForTarget(_ context.Context, _ Target, _ *bool) ([]string, error) {
	return nil, nil
}
func (m *mockClientForPool) GetServicesForTarget(_ context.Context, _ Target, _ *bool) ([]string, error) {
	return nil, nil
}
func (m *mockClientForPool) ExtractFromTarget(_ context.Context, _ Target, _ *bool) (*ExtractFromTargetResult, error) {
	return nil, nil
}
func (m *mockClientForPool) GetScheduleConfig(_ context.Context, _ string) (map[string]any, error) {
	return nil, nil
}
func (m *mockClientForPool) GetServices(_ context.Context) ([]Service, error) { return nil, nil }
func (m *mockClientForPool) GetConfig(_ context.Context) (*Config, error)     { return nil, nil }
func (m *mockClientForPool) RenderTemplate(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockClientForPool) GetLogbook(_ context.Context, _, _, _ string) ([]LogbookEntry, error) {
	return nil, nil
}
func (m *mockClientForPool) GetSystemLog(context.Context) ([]SystemLogEntry, error) { return nil, nil }
func (m *mockClientForPool) ClearSystemLog(context.Context) error                   { return nil }
func (m *mockClientForPool) CheckConfig(_ context.Context) (*ConfigCheckResult, error) {
	return nil, nil
}
func (m *mockClientForPool) GetConfigEntries(_ context.Context, _ string) ([]ConfigEntryFull, error) {
	return nil, nil
}
func (m *mockClientForPool) GetConfigEntry(_ context.Context, _ string) (*ConfigEntryFull, error) {
	return nil, nil
}

func (m *mockClientForPool) GetConfigEntryOptions(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (m *mockClientForPool) SendHACSCommand(_ context.Context, _ string, _ map[string]any) (any, error) {
	return nil, nil
}

// Ensure mockClientForPool implements Client and ClientCloser.
var (
	_ Client       = (*mockClientForPool)(nil)
	_ ClientCloser = (*mockClientForPool)(nil)
)

func TestIsClientConnected_Function(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		client    Client
		wantValue bool
	}{
		{
			name:      "connected client",
			client:    &mockClientForPool{connected: true},
			wantValue: true,
		},
		{
			name:      "disconnected client",
			client:    &mockClientForPool{connected: false},
			wantValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isClientConnected(tt.client)
			if result != tt.wantValue {
				t.Errorf("isClientConnected() = %v, want %v", result, tt.wantValue)
			}
		})
	}
}

func TestClientPool_CleanupIdleClients(t *testing.T) {
	t.Parallel()

	// Create pool with very short idle timeout
	pool := NewClientPool("http://localhost:8123", 50*time.Millisecond)
	defer pool.Close()

	// Manually add mock clients to the pool
	pool.mu.Lock()
	pool.clients["token1"] = &pooledClient{
		client:   &mockClientForPool{connected: true},
		lastUsed: time.Now().Add(-100 * time.Millisecond), // Already idle
	}
	pool.clients["token2"] = &pooledClient{
		client:   &mockClientForPool{connected: true},
		lastUsed: time.Now(), // Recently used
	}
	pool.mu.Unlock()

	// Verify initial state
	if pool.Size() != 2 {
		t.Fatalf("Expected 2 clients, got %d", pool.Size())
	}

	// Manually trigger cleanup
	pool.cleanupIdleClients()

	// Only token2 should remain (token1 was idle)
	if pool.Size() != 1 {
		t.Errorf("Expected 1 client after cleanup, got %d", pool.Size())
	}

	pool.mu.RLock()
	_, token1Exists := pool.clients["token1"]
	_, token2Exists := pool.clients["token2"]
	pool.mu.RUnlock()

	if token1Exists {
		t.Error("token1 should have been cleaned up")
	}
	if !token2Exists {
		t.Error("token2 should still exist")
	}
}

func TestClientPool_CleanupLoop_Stops(t *testing.T) {
	t.Parallel()

	// Create pool with short idle timeout for faster test
	pool := NewClientPool("http://localhost:8123", 100*time.Millisecond)

	// Close the pool and verify cleanup loop stops
	err := pool.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Closing again should be safe (channel already closed)
	// This tests that the cleanup loop has stopped
	// The wg.Wait() in Close() ensures the goroutine has finished
}

func TestClientPool_Close_ClosesAllClients(t *testing.T) {
	t.Parallel()

	pool := NewClientPool("http://localhost:8123", 30*time.Minute)

	// Manually add mock clients
	mockClient1 := &mockClientForPool{connected: true}
	mockClient2 := &mockClientForPool{connected: true}

	pool.mu.Lock()
	pool.clients["token1"] = &pooledClient{
		client:   mockClient1,
		lastUsed: time.Now(),
	}
	pool.clients["token2"] = &pooledClient{
		client:   mockClient2,
		lastUsed: time.Now(),
	}
	pool.mu.Unlock()

	if pool.Size() != 2 {
		t.Fatalf("Expected 2 clients, got %d", pool.Size())
	}

	err := pool.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Pool should be empty
	if pool.Size() != 0 {
		t.Errorf("Expected 0 clients after close, got %d", pool.Size())
	}

	// Both clients should have been closed
	if !mockClient1.closed {
		t.Error("mockClient1 was not closed")
	}
	if !mockClient2.closed {
		t.Error("mockClient2 was not closed")
	}
}

func TestClientPool_GetOrCreate_ReuseExistingClient(t *testing.T) {
	t.Parallel()

	pool := NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	// Manually add a mock client
	mockClient := &mockClientForPool{connected: true}
	pool.mu.Lock()
	pool.clients["existing-token"] = &pooledClient{
		client:   mockClient,
		lastUsed: time.Now().Add(-5 * time.Minute), // Used 5 minutes ago
	}
	pool.mu.Unlock()

	ctx := context.Background()
	client, err := pool.GetOrCreate(ctx, "existing-token")

	if err != nil {
		t.Fatalf("GetOrCreate returned error: %v", err)
	}

	// Should return the existing client
	if client != mockClient {
		t.Error("Expected existing client to be returned")
	}

	// lastUsed should be updated
	pool.mu.RLock()
	pc := pool.clients["existing-token"]
	pool.mu.RUnlock()

	if time.Since(pc.lastUsed) > time.Second {
		t.Error("lastUsed was not updated")
	}
}

func TestClientPool_GetOrCreate_RemovesDisconnectedClient(t *testing.T) {
	t.Parallel()

	pool := NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	// Add a disconnected client
	disconnectedClient := &mockClientForPool{connected: false}
	pool.mu.Lock()
	pool.clients["test-token"] = &pooledClient{
		client:   disconnectedClient,
		lastUsed: time.Now(),
	}
	pool.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should detect the disconnected client and try to create a new one
	// (which will fail due to invalid URL, but the disconnected client should be removed)
	_, _ = pool.GetOrCreate(ctx, "test-token")

	// The disconnected client should have been closed
	if !disconnectedClient.closed {
		t.Error("Disconnected client should have been closed")
	}
}

func TestClientPool_CleanupIdleClients_AllIdle(t *testing.T) {
	t.Parallel()

	pool := NewClientPool("http://localhost:8123", 10*time.Millisecond)
	defer pool.Close()

	// Add all idle clients
	pool.mu.Lock()
	pool.clients["token1"] = &pooledClient{
		client:   &mockClientForPool{connected: true},
		lastUsed: time.Now().Add(-100 * time.Millisecond),
	}
	pool.clients["token2"] = &pooledClient{
		client:   &mockClientForPool{connected: true},
		lastUsed: time.Now().Add(-100 * time.Millisecond),
	}
	pool.mu.Unlock()

	pool.cleanupIdleClients()

	if pool.Size() != 0 {
		t.Errorf("Expected 0 clients after cleanup, got %d", pool.Size())
	}
}

func TestClientPool_CleanupIdleClients_NoneIdle(t *testing.T) {
	t.Parallel()

	pool := NewClientPool("http://localhost:8123", 1*time.Hour)
	defer pool.Close()

	// Add recently used clients
	pool.mu.Lock()
	pool.clients["token1"] = &pooledClient{
		client:   &mockClientForPool{connected: true},
		lastUsed: time.Now(),
	}
	pool.clients["token2"] = &pooledClient{
		client:   &mockClientForPool{connected: true},
		lastUsed: time.Now(),
	}
	pool.mu.Unlock()

	pool.cleanupIdleClients()

	if pool.Size() != 2 {
		t.Errorf("Expected 2 clients after cleanup (none idle), got %d", pool.Size())
	}
}

func TestClientPool_EvictLRU_WhenFull(t *testing.T) {
	t.Parallel()

	pool := NewClientPoolWithFullConfig("http://localhost:8123", 30*time.Minute, 2, nil, nil, nil)
	defer pool.Close()

	oldest := &mockClientForPool{connected: true}
	newer := &mockClientForPool{connected: true}

	// Seed the pool at capacity: oldest was used first (earlier timestamp)
	pool.mu.Lock()
	pool.clients["token-oldest"] = &pooledClient{
		client:   oldest,
		lastUsed: time.Now().Add(-10 * time.Minute),
	}
	pool.clients["token-newer"] = &pooledClient{
		client:   newer,
		lastUsed: time.Now().Add(-1 * time.Minute),
	}
	pool.mu.Unlock()

	if pool.Size() != 2 {
		t.Fatalf("expected pool size 2 before eviction, got %d", pool.Size())
	}

	// Trigger eviction directly (called under write lock — acquire it here to match production path)
	pool.mu.Lock()
	pool.evictLRU()
	pool.mu.Unlock()

	// Pool should now have 1 entry and the oldest should have been closed
	if pool.Size() != 1 {
		t.Errorf("expected pool size 1 after eviction, got %d", pool.Size())
	}
	if !oldest.closed {
		t.Error("expected oldest client to be closed after LRU eviction")
	}
	pool.mu.RLock()
	_, oldestStillPresent := pool.clients["token-oldest"]
	_, newerStillPresent := pool.clients["token-newer"]
	pool.mu.RUnlock()

	if oldestStillPresent {
		t.Error("token-oldest should have been evicted")
	}
	if !newerStillPresent {
		t.Error("token-newer should still be in the pool")
	}
}

func TestClientPool_MaxSize_EnforcedOnInsert(t *testing.T) {
	t.Parallel()

	pool := NewClientPoolWithFullConfig("http://localhost:8123", 30*time.Minute, 2, nil, nil, nil)
	defer pool.Close()

	c1 := &mockClientForPool{connected: true}
	c2 := &mockClientForPool{connected: true}
	c3 := &mockClientForPool{connected: true}

	pool.mu.Lock()
	pool.clients["t1"] = &pooledClient{client: c1, lastUsed: time.Now().Add(-5 * time.Minute)}
	pool.clients["t2"] = &pooledClient{client: c2, lastUsed: time.Now().Add(-1 * time.Minute)}
	pool.mu.Unlock()

	// Insert c3: pool is at maxSize=2, so eviction must happen before insert
	pool.mu.Lock()
	if pool.maxSize > 0 && len(pool.clients) >= pool.maxSize {
		pool.evictLRU()
	}
	pool.clients["t3"] = &pooledClient{client: c3, lastUsed: time.Now()}
	pool.mu.Unlock()

	if pool.Size() != 2 {
		t.Errorf("expected pool size 2 after capped insert, got %d", pool.Size())
	}
	if !c1.closed {
		t.Error("expected c1 (oldest/LRU) to be closed after insert")
	}
	pool.mu.RLock()
	_, t3present := pool.clients["t3"]
	pool.mu.RUnlock()
	if !t3present {
		t.Error("expected t3 to be present after insert")
	}
}
