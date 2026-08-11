package homeassistant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestDefaultClientOptions(t *testing.T) {
	t.Parallel()

	opts := DefaultClientOptions()

	if opts.WSConfig == nil {
		t.Fatal("WSConfig is nil, expected non-nil")
	}

	// Verify WSConfig has default values
	defaultWSConfig := DefaultWSClientConfig()

	if opts.WSConfig.AutoReconnect != defaultWSConfig.AutoReconnect {
		t.Errorf("AutoReconnect = %v, want %v", opts.WSConfig.AutoReconnect, defaultWSConfig.AutoReconnect)
	}
	if opts.WSConfig.PingInterval != defaultWSConfig.PingInterval {
		t.Errorf("PingInterval = %v, want %v", opts.WSConfig.PingInterval, defaultWSConfig.PingInterval)
	}
	if opts.WSConfig.PingTimeout != defaultWSConfig.PingTimeout {
		t.Errorf("PingTimeout = %v, want %v", opts.WSConfig.PingTimeout, defaultWSConfig.PingTimeout)
	}
	if opts.WSConfig.WriteTimeout != defaultWSConfig.WriteTimeout {
		t.Errorf("WriteTimeout = %v, want %v", opts.WSConfig.WriteTimeout, defaultWSConfig.WriteTimeout)
	}
}

func TestDefaultWSClientConfig(t *testing.T) {
	t.Parallel()

	config := DefaultWSClientConfig()

	if !config.AutoReconnect {
		t.Error("AutoReconnect = false, want true")
	}
	if config.PingInterval <= 0 {
		t.Errorf("PingInterval = %v, want > 0", config.PingInterval)
	}
	if config.PingTimeout <= 0 {
		t.Errorf("PingTimeout = %v, want > 0", config.PingTimeout)
	}
	if config.WriteTimeout <= 0 {
		t.Errorf("WriteTimeout = %v, want > 0", config.WriteTimeout)
	}

	// Verify reconnect config
	defaultReconnectConfig := DefaultReconnectConfig()
	if diff := cmp.Diff(defaultReconnectConfig, config.ReconnectConfig); diff != "" {
		t.Errorf("ReconnectConfig mismatch (-want +got):\n%s", diff)
	}
}

func TestCloseClient_NilClient(t *testing.T) {
	t.Parallel()

	// CloseClient should handle nil gracefully
	err := CloseClient(nil)
	if err != nil {
		t.Errorf("CloseClient(nil) error = %v, want nil", err)
	}
}

func TestCloseClient_NonCloser(t *testing.T) {
	t.Parallel()

	// Create a mock client that doesn't implement ClientCloser
	mockClient := &mockNonCloserClient{}

	err := CloseClient(mockClient)
	if err != nil {
		t.Errorf("CloseClient(non-closer) error = %v, want nil", err)
	}
}

func TestCloseClient_Closer(t *testing.T) {
	t.Parallel()

	// Create a mock client that implements ClientCloser
	mockClient := &mockCloserClient{closed: false}

	err := CloseClient(mockClient)
	if err != nil {
		t.Errorf("CloseClient(closer) error = %v, want nil", err)
	}

	if !mockClient.closed {
		t.Error("Close() was not called on ClientCloser")
	}
}

func TestCloseClient_CloserWithError(t *testing.T) {
	t.Parallel()

	// Create a mock client that returns an error on Close
	expectedErr := &testError{msg: "close error"}
	mockClient := &mockCloserClient{closed: false, closeErr: expectedErr}

	err := CloseClient(mockClient)
	if !errors.Is(err, expectedErr) {
		t.Errorf("CloseClient() error = %v, want %v", err, expectedErr)
	}
}

func TestWSClientImpl_Interfaces(t *testing.T) {
	t.Parallel()

	// Verify wsClientImpl implements WSOperations
	var _ WSOperations = (*wsClientImpl)(nil)
}

func TestClientOptions_CustomWSConfig(t *testing.T) {
	t.Parallel()

	customConfig := &WSClientConfig{
		AutoReconnect: false,
		PingInterval:  0, // Disabled
		PingTimeout:   5,
		WriteTimeout:  15,
	}

	opts := ClientOptions{
		WSConfig: customConfig,
	}

	if opts.WSConfig != customConfig {
		t.Error("WSConfig was not set correctly")
	}
	if opts.WSConfig.AutoReconnect != false {
		t.Errorf("AutoReconnect = %v, want false", opts.WSConfig.AutoReconnect)
	}
	if opts.WSConfig.PingInterval != 0 {
		t.Errorf("PingInterval = %v, want 0", opts.WSConfig.PingInterval)
	}
}

// Mock implementations for testing

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// mockNonCloserClient is a minimal Client implementation that doesn't implement ClientCloser
type mockNonCloserClient struct{}

func (m *mockNonCloserClient) GetStates(_ context.Context) ([]Entity, error) {
	return []Entity{}, nil
}
func (m *mockNonCloserClient) GetState(_ context.Context, _ string) (*Entity, error) {
	return &Entity{}, nil
}
func (m *mockNonCloserClient) SetState(_ context.Context, _ string, _ StateUpdate) (*Entity, error) {
	return &Entity{}, nil
}
func (m *mockNonCloserClient) GetHistory(_ context.Context, _ string, _, _ time.Time) ([][]HistoryEntry, error) {
	return [][]HistoryEntry{}, nil
}
func (m *mockNonCloserClient) ListAutomations(_ context.Context) ([]Automation, error) {
	return []Automation{}, nil
}
func (m *mockNonCloserClient) GetAutomation(_ context.Context, _ string) (*Automation, error) {
	return &Automation{}, nil
}
func (m *mockNonCloserClient) CreateAutomation(_ context.Context, _ AutomationConfig) error {
	return nil
}
func (m *mockNonCloserClient) UpdateAutomation(_ context.Context, _ string, _ AutomationConfig) error {
	return nil
}
func (m *mockNonCloserClient) DeleteAutomation(_ context.Context, _ string) error {
	return nil
}
func (m *mockNonCloserClient) ToggleAutomation(_ context.Context, _ string, _ bool) error {
	return nil
}
func (m *mockNonCloserClient) ListHelpers(_ context.Context) ([]Entity, error) {
	return []Entity{}, nil
}
func (m *mockNonCloserClient) CreateHelper(_ context.Context, _ HelperConfig) error {
	return nil
}
func (m *mockNonCloserClient) UpdateHelper(_ context.Context, _ string, _ HelperConfig) error {
	return nil
}
func (m *mockNonCloserClient) DeleteHelper(_ context.Context, _ string) error {
	return nil
}
func (m *mockNonCloserClient) SetHelperValue(_ context.Context, _ string, _ any) error {
	return nil
}
func (m *mockNonCloserClient) ListScripts(_ context.Context) ([]Entity, error) {
	return []Entity{}, nil
}
func (m *mockNonCloserClient) GetScript(_ context.Context, _ string) (*Script, error) {
	return &Script{}, nil
}
func (m *mockNonCloserClient) CreateScript(_ context.Context, _ string, _ ScriptConfig) error {
	return nil
}
func (m *mockNonCloserClient) UpdateScript(_ context.Context, _ string, _ ScriptConfig) error {
	return nil
}
func (m *mockNonCloserClient) DeleteScript(_ context.Context, _ string) error {
	return nil
}
func (m *mockNonCloserClient) ListScenes(_ context.Context) ([]Entity, error) {
	return []Entity{}, nil
}
func (m *mockNonCloserClient) GetScene(_ context.Context, _ string) (*Scene, error) {
	return nil, nil
}
func (m *mockNonCloserClient) CreateScene(_ context.Context, _ string, _ SceneConfig) error {
	return nil
}
func (m *mockNonCloserClient) UpdateScene(_ context.Context, _ string, _ SceneConfig) error {
	return nil
}
func (m *mockNonCloserClient) DeleteScene(_ context.Context, _ string) error {
	return nil
}
func (m *mockNonCloserClient) ConfigFileEntryExists(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}
func (m *mockNonCloserClient) CallService(_ context.Context, _, _ string, _ map[string]any) ([]Entity, error) {
	return []Entity{}, nil
}
func (m *mockNonCloserClient) CallServiceWithResponse(context.Context, string, string, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}
func (m *mockNonCloserClient) GetCalendars(context.Context) ([]CalendarEntry, error) {
	return []CalendarEntry{}, nil
}
func (m *mockNonCloserClient) GetCalendarEvents(context.Context, string, string, string) ([]CalendarEvent, error) {
	return []CalendarEvent{}, nil
}
func (m *mockNonCloserClient) GetCameraSnapshot(context.Context, string) ([]byte, string, error) {
	return []byte{}, "", nil
}
func (m *mockNonCloserClient) GetEntityRegistry(_ context.Context) ([]EntityRegistryEntry, error) {
	return []EntityRegistryEntry{}, nil
}
func (m *mockNonCloserClient) GetDeviceRegistry(_ context.Context) ([]DeviceRegistryEntry, error) {
	return []DeviceRegistryEntry{}, nil
}
func (m *mockNonCloserClient) GetAreaRegistry(_ context.Context) ([]AreaRegistryEntry, error) {
	return []AreaRegistryEntry{}, nil
}
func (m *mockNonCloserClient) CreateArea(_ context.Context, _ AreaConfig) (*AreaRegistryEntry, error) {
	return &AreaRegistryEntry{}, nil
}
func (m *mockNonCloserClient) UpdateArea(_ context.Context, _ string, _ AreaConfig) (*AreaRegistryEntry, error) {
	return &AreaRegistryEntry{}, nil
}
func (m *mockNonCloserClient) DeleteArea(_ context.Context, _ string) error {
	return nil
}
func (m *mockNonCloserClient) GetLabelRegistry(_ context.Context) ([]LabelRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) CreateLabel(_ context.Context, _ LabelConfig) (*LabelRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) UpdateLabel(_ context.Context, _ string, _ LabelConfig) (*LabelRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) DeleteLabel(_ context.Context, _ string) error {
	return nil
}

func (m *mockNonCloserClient) GetFloorRegistry(_ context.Context) ([]FloorRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) CreateFloor(_ context.Context, _ FloorConfig) (*FloorRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) UpdateFloor(_ context.Context, _ string, _ FloorConfig) (*FloorRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) DeleteFloor(_ context.Context, _ string) error {
	return nil
}

func (m *mockNonCloserClient) GetZones(_ context.Context) ([]ZoneRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) CreateZone(_ context.Context, _ ZoneConfig) (*ZoneRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) UpdateZone(_ context.Context, _ string, _ ZoneConfig) (*ZoneRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) DeleteZone(_ context.Context, _ string) error {
	return nil
}

func (m *mockNonCloserClient) GetPersons(_ context.Context) ([]PersonRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) CreatePerson(_ context.Context, _ PersonConfig) (*PersonRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) UpdatePerson(_ context.Context, _ string, _ PersonConfig) (*PersonRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) DeletePerson(_ context.Context, _ string) error {
	return nil
}

func (m *mockNonCloserClient) GetTags(_ context.Context) ([]TagRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) CreateTag(_ context.Context, _ TagConfig) (*TagRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) UpdateTag(_ context.Context, _ string, _ TagConfig) (*TagRegistryEntry, error) {
	return nil, nil
}

func (m *mockNonCloserClient) DeleteTag(_ context.Context, _ string) error {
	return nil
}

func (m *mockNonCloserClient) RemoveEntityRegistryEntry(_ context.Context, _ string) error {
	return nil
}
func (m *mockNonCloserClient) UpdateEntityRegistryEntry(_ context.Context, _ string, _ EntityRegistryUpdateConfig) (*EntityRegistryEntry, error) {
	return nil, nil
}
func (m *mockNonCloserClient) RemoveDeviceConfigEntry(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockNonCloserClient) UpdateDeviceRegistryEntry(_ context.Context, _ string, _ DeviceRegistryUpdateConfig) (*DeviceRegistryEntry, error) {
	return nil, nil
}
func (m *mockNonCloserClient) SignPath(_ context.Context, _ string, _ int) (string, error) {
	return "", nil
}
func (m *mockNonCloserClient) GetCameraStream(_ context.Context, _ string) (*StreamInfo, error) {
	return &StreamInfo{}, nil
}
func (m *mockNonCloserClient) BrowseMedia(_ context.Context, _ string) (*MediaBrowseResult, error) {
	return &MediaBrowseResult{}, nil
}
func (m *mockNonCloserClient) GetLovelaceConfig(_ context.Context, _ string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (m *mockNonCloserClient) SaveLovelaceConfig(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

func (m *mockNonCloserClient) ListDashboards(_ context.Context) ([]DashboardEntry, error) {
	return []DashboardEntry{}, nil
}

func (m *mockNonCloserClient) CreateDashboard(_ context.Context, _ DashboardConfig) (*DashboardEntry, error) {
	return &DashboardEntry{}, nil
}

func (m *mockNonCloserClient) UpdateDashboard(_ context.Context, _ string, _ DashboardConfig) (*DashboardEntry, error) {
	return &DashboardEntry{}, nil
}

func (m *mockNonCloserClient) DeleteDashboard(_ context.Context, _ string) error {
	return nil
}
func (m *mockNonCloserClient) GetStatistics(_ context.Context, _ []string, _ string) ([]StatisticsResult, error) {
	return []StatisticsResult{}, nil
}
func (m *mockNonCloserClient) GetTriggersForTarget(_ context.Context, _ Target, _ *bool) ([]string, error) {
	return []string{}, nil
}
func (m *mockNonCloserClient) GetConditionsForTarget(_ context.Context, _ Target, _ *bool) ([]string, error) {
	return []string{}, nil
}
func (m *mockNonCloserClient) GetServicesForTarget(_ context.Context, _ Target, _ *bool) ([]string, error) {
	return []string{}, nil
}
func (m *mockNonCloserClient) ExtractFromTarget(_ context.Context, _ Target, _ *bool) (*ExtractFromTargetResult, error) {
	return &ExtractFromTargetResult{}, nil
}
func (m *mockNonCloserClient) GetHelperConfig(_ context.Context, _, _ string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (m *mockNonCloserClient) GetServices(_ context.Context) ([]Service, error) {
	return []Service{}, nil
}
func (m *mockNonCloserClient) GetConfig(_ context.Context) (*Config, error) {
	return &Config{}, nil
}

func (m *mockNonCloserClient) RenderTemplate(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockNonCloserClient) GetLogbook(_ context.Context, _, _, _ string) ([]LogbookEntry, error) {
	return []LogbookEntry{}, nil
}

func (m *mockNonCloserClient) GetSystemLog(context.Context) ([]SystemLogEntry, error) {
	return nil, nil
}
func (m *mockNonCloserClient) ClearSystemLog(context.Context) error { return nil }

func (m *mockNonCloserClient) CheckConfig(_ context.Context) (*ConfigCheckResult, error) {
	return &ConfigCheckResult{Result: "valid"}, nil
}

func (m *mockNonCloserClient) GetConfigEntries(_ context.Context, _ string) ([]ConfigEntryFull, error) {
	return []ConfigEntryFull{}, nil
}

func (m *mockNonCloserClient) GetConfigEntry(_ context.Context, _ string) (*ConfigEntryFull, error) {
	return &ConfigEntryFull{}, nil
}

func (m *mockNonCloserClient) GetConfigEntryOptions(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (m *mockNonCloserClient) DeleteConfigEntry(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockNonCloserClient) SendHACSCommand(_ context.Context, _ string, _ map[string]any) (any, error) {
	return map[string]any{}, nil
}

// Ensure mockNonCloserClient implements Client but NOT ClientCloser
var _ Client = (*mockNonCloserClient)(nil)

// mockCloserClient implements both Client and ClientCloser
type mockCloserClient struct {
	mockNonCloserClient
	closed   bool
	closeErr error
}

func (m *mockCloserClient) Close() error {
	m.closed = true
	return m.closeErr
}

// Ensure mockCloserClient implements both Client and ClientCloser
var (
	_ Client       = (*mockCloserClient)(nil)
	_ ClientCloser = (*mockCloserClient)(nil)
)

func TestNewClientWithOptions_InvalidURL(t *testing.T) {
	t.Parallel()

	opts := DefaultClientOptions()

	// Try to connect with invalid URL - should fail
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := NewClientWithOptions(ctx, "ws://invalid-host:9999", "test-token", opts)
	if err == nil {
		t.Error("NewClientWithOptions with invalid URL should return error")
	}
}

func TestNewConnectedClient_WithNilConfig(t *testing.T) {
	t.Parallel()

	// Try to connect with nil config - should use defaults and fail due to invalid host
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := NewConnectedClient(ctx, "ws://invalid-host:9999", "test-token", nil, nil)
	if err == nil {
		t.Error("NewConnectedClient with invalid URL should return error")
	}
}

func TestNewConnectedClient_WithWSConfig(t *testing.T) {
	t.Parallel()

	wsConfig := &WSClientConfig{
		AutoReconnect: false,
		PingInterval:  10 * time.Second,
	}

	// Try to connect with config - should fail due to invalid host
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := NewConnectedClient(ctx, "ws://invalid-host:9999", "test-token", wsConfig, nil)
	if err == nil {
		t.Error("NewConnectedClient with invalid URL should return error")
	}
}

func TestNewConnectedClient_WithRESTConfig(t *testing.T) {
	t.Parallel()

	restConfig := &RESTClientConfig{
		Timeout:   5 * time.Second,
		RateLimit: 20,
		RateBurst: 10,
	}

	// Try to connect with REST config - should fail due to invalid host
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := NewConnectedClient(ctx, "ws://invalid-host:9999", "test-token", nil, restConfig)
	if err == nil {
		t.Error("NewConnectedClient with invalid URL should return error")
	}
}

func TestNewConnectedClient_WithBothConfigs(t *testing.T) {
	t.Parallel()

	wsConfig := &WSClientConfig{
		AutoReconnect: false,
		PingInterval:  10 * time.Second,
	}

	restConfig := &RESTClientConfig{
		Timeout:   5 * time.Second,
		RateLimit: 20,
		RateBurst: 10,
	}

	// Try to connect with both configs - should fail due to invalid host
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := NewConnectedClient(ctx, "ws://invalid-host:9999", "test-token", wsConfig, restConfig)
	if err == nil {
		t.Error("NewConnectedClient with invalid URL should return error")
	}
}

func TestNewDefaultWSClient_InvalidURL(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := NewDefaultWSClient(ctx, "ws://invalid-host:9999", "test-token")
	if err == nil {
		t.Error("NewDefaultWSClient with invalid URL should return error")
	}
}
