package homeassistant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHybridClient_DeleteAutomation verifies that DeleteAutomation
// uses the REST client instead of WebSocket.
func TestHybridClient_DeleteAutomation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		automationID   string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
	}{
		{
			name:         "successful deletion via REST",
			automationID: "test_automation",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify this is a REST DELETE request
				if r.Method != http.MethodDelete {
					t.Errorf("expected DELETE method, got %s", r.Method)
				}
				expectedPath := "/api/config/automation/config/test_automation"
				if r.URL.Path != expectedPath {
					t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
				}
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name:         "REST error propagates",
			automationID: "nonexistent",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange - Create mock REST server
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			restClient := NewRESTClient(server.URL, "test-token")

			// Create HybridClient with nil WebSocket (we're only testing REST)
			// Note: In real usage, wsClientImpl would be initialized
			hybridClient := &HybridClient{
				ws:   nil, // Not used for DeleteAutomation
				rest: restClient,
			}

			// Act
			err := hybridClient.DeleteAutomation(context.Background(), tt.automationID)

			// Assert
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteAutomation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestHybridClient_DeleteScript verifies that DeleteScript
// uses the REST client instead of WebSocket.
func TestHybridClient_DeleteScript(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		expectedPath := "/api/config/script/config/test_script"
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	hybridClient := &HybridClient{
		ws:   nil,
		rest: restClient,
	}

	err := hybridClient.DeleteScript(context.Background(), "test_script")

	if err != nil {
		t.Errorf("DeleteScript() error = %v, want nil", err)
	}
}

// TestHybridClient_DeleteScene verifies that DeleteScene
// uses the REST client instead of WebSocket.
func TestHybridClient_DeleteScene(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		expectedPath := "/api/config/scene/config/test_scene"
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	hybridClient := &HybridClient{
		ws:   nil,
		rest: restClient,
	}

	err := hybridClient.DeleteScene(context.Background(), "test_scene")

	if err != nil {
		t.Errorf("DeleteScene() error = %v, want nil", err)
	}
}

// TestNewHybridClient verifies the constructor creates a properly initialized client.
func TestNewHybridClient(t *testing.T) {
	t.Parallel()

	// Create mock REST server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")

	// Create a minimal WSClient for testing (won't connect)
	wsClient := NewWSClient("ws://localhost:8123", "test-token")

	hybridClient := NewHybridClient(wsClient, restClient)

	// Verify the client was created
	if hybridClient == nil {
		t.Fatal("NewHybridClient returned nil")
	}
	if hybridClient.ws == nil {
		t.Error("ws client is nil")
	}
	if hybridClient.rest == nil {
		t.Error("rest client is nil")
	}
}

// TestNewHybridClientCloser verifies the closer variant works correctly.
func TestNewHybridClientCloser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	wsClient := NewWSClient("ws://localhost:8123", "test-token")

	hybridCloser := NewHybridClientCloser(wsClient, restClient)

	// Verify it implements both interfaces
	var _ Client = hybridCloser
	var _ ClientCloser = hybridCloser

	if hybridCloser == nil {
		t.Fatal("NewHybridClientCloser returned nil")
	}

	// Test Close method (should not panic even without connection)
	// Note: This may error because there's no actual connection,
	// but it should not panic
	_ = hybridCloser.Close()
}

// TestHybridClient_InterfaceCompliance verifies that HybridClient
// implements the Client interface at compile time.
func TestHybridClient_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	// This test is primarily for compile-time verification
	// The var _ declarations in hybrid_client.go also verify this
	var _ Client = (*HybridClient)(nil)
	var _ Client = (*HybridClientCloser)(nil)
	var _ ClientCloser = (*HybridClientCloser)(nil)
}

// TestHybridClient_GetServices verifies that GetServices uses REST client.
func TestHybridClient_GetServices(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/api/services" {
			t.Errorf("path = %q, want /api/services", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"domain": "light", "services": {"turn_on": {}}}]`))
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	hybridClient := &HybridClient{
		ws:   nil,
		rest: restClient,
	}

	services, err := hybridClient.GetServices(context.Background())
	if err != nil {
		t.Fatalf("GetServices() error = %v, want nil", err)
	}

	if len(services) != 1 {
		t.Errorf("got %d services, want 1", len(services))
	}

	if services[0].Domain != "light" {
		t.Errorf("domain = %q, want light", services[0].Domain)
	}
}

// TestHybridClient_GetConfig verifies that GetConfig uses REST client.
func TestHybridClient_GetConfig(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/api/config" {
			t.Errorf("path = %q, want /api/config", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version": "2024.1.0", "location_name": "Home"}`))
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	hybridClient := &HybridClient{
		ws:   nil,
		rest: restClient,
	}

	config, err := hybridClient.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig() error = %v, want nil", err)
	}

	if config.Version != "2024.1.0" {
		t.Errorf("version = %q, want 2024.1.0", config.Version)
	}
}

// TestHybridClient_RenderTemplate verifies that RenderTemplate uses REST client.
func TestHybridClient_RenderTemplate(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/template" {
			t.Errorf("path = %q, want /api/template", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("22.5"))
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	hybridClient := &HybridClient{
		ws:   nil,
		rest: restClient,
	}

	result, err := hybridClient.RenderTemplate(context.Background(), "{{ states('sensor.temperature') }}")
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v, want nil", err)
	}

	if result != "22.5" {
		t.Errorf("result = %q, want 22.5", result)
	}
}

// TestHybridClient_GetLogbook verifies that GetLogbook uses REST client.
func TestHybridClient_GetLogbook(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		expectedPath := "/api/logbook/2024-01-01T00:00:00Z"
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"name": "Test", "message": "turned on", "entity_id": "light.test"}]`))
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	hybridClient := &HybridClient{
		ws:   nil,
		rest: restClient,
	}

	entries, err := hybridClient.GetLogbook(context.Background(), "2024-01-01T00:00:00Z", "", "")
	if err != nil {
		t.Fatalf("GetLogbook() error = %v, want nil", err)
	}

	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1", len(entries))
	}
}

// TestHybridClient_CheckConfig verifies that CheckConfig uses REST client.
func TestHybridClient_CheckConfig(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/config/core/check_config" {
			t.Errorf("path = %q, want /api/config/core/check_config", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result": "valid", "errors": null}`))
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	hybridClient := &HybridClient{
		ws:   nil,
		rest: restClient,
	}

	result, err := hybridClient.CheckConfig(context.Background())
	if err != nil {
		t.Fatalf("CheckConfig() error = %v, want nil", err)
	}

	if result.Result != "valid" {
		t.Errorf("result = %q, want valid", result.Result)
	}
}

// TestHybridClientCloser_IsConnected verifies IsConnected delegates to WSClient.
func TestHybridClientCloser_IsConnected(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	wsClient := NewWSClient("ws://localhost:8123", "test-token")

	hybridCloser := NewHybridClientCloser(wsClient, restClient)

	// Without actual connection, IsConnected should return false
	if hybridCloser.IsConnected() {
		t.Error("IsConnected() = true, want false (no connection)")
	}
}

// TestHybridClientCloser_WaitForConnection verifies WaitForConnection delegates to WSClient.
func TestHybridClientCloser_WaitForConnection(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	wsClient := NewWSClient("ws://localhost:8123", "test-token")

	hybridCloser := NewHybridClientCloser(wsClient, restClient)

	// Create a context that will timeout quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Should timeout/error since there's no real connection
	err := hybridCloser.WaitForConnection(ctx)
	if err == nil {
		t.Error("WaitForConnection() = nil, want error (context deadline)")
	}
}

// =============================================================================
// Mock implementations for comprehensive HybridClient testing
// =============================================================================

// mockWSOperations implements WSOperations for testing.
type mockWSOperations struct {
	getStatesFunc              func(ctx context.Context) ([]Entity, error)
	getStateFunc               func(ctx context.Context, entityID string) (*Entity, error)
	setStateFunc               func(ctx context.Context, entityID string, state StateUpdate) (*Entity, error)
	getHistoryFunc             func(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error)
	callServiceFunc            func(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error)
	listAutomationsFunc        func(ctx context.Context) ([]Automation, error)
	getAutomationFunc          func(ctx context.Context, automationID string) (*Automation, error)
	createAutomationFunc       func(ctx context.Context, config AutomationConfig) error
	updateAutomationFunc       func(ctx context.Context, automationID string, config AutomationConfig) error
	toggleAutomationFunc       func(ctx context.Context, entityID string, enabled bool) error
	listHelpersFunc            func(ctx context.Context) ([]Entity, error)
	createHelperFunc           func(ctx context.Context, config HelperConfig) error
	updateHelperFunc           func(ctx context.Context, helperID string, config HelperConfig) error
	deleteHelperFunc           func(ctx context.Context, helperID string) error
	setHelperValueFunc         func(ctx context.Context, entityID string, value any) error
	listScriptsFunc            func(ctx context.Context) ([]Entity, error)
	getScriptFunc              func(ctx context.Context, scriptID string) (*Script, error)
	createScriptFunc           func(ctx context.Context, scriptID string, config ScriptConfig) error
	updateScriptFunc           func(ctx context.Context, scriptID string, config ScriptConfig) error
	listScenesFunc             func(ctx context.Context) ([]Entity, error)
	createSceneFunc            func(ctx context.Context, sceneID string, config SceneConfig) error
	updateSceneFunc            func(ctx context.Context, sceneID string, config SceneConfig) error
	getEntityRegistryFunc      func(ctx context.Context) ([]EntityRegistryEntry, error)
	getDeviceRegistryFunc      func(ctx context.Context) ([]DeviceRegistryEntry, error)
	getAreaRegistryFunc        func(ctx context.Context) ([]AreaRegistryEntry, error)
	signPathFunc               func(ctx context.Context, path string, expires int) (string, error)
	getCameraStreamFunc        func(ctx context.Context, entityID string) (*StreamInfo, error)
	browseMediaFunc            func(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error)
	getLovelaceConfigFunc      func(ctx context.Context) (map[string]any, error)
	getStatisticsFunc          func(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error)
	getTriggersForTargetFunc   func(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	getConditionsForTargetFunc func(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	getServicesForTargetFunc   func(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	extractFromTargetFunc      func(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error)
	getScheduleConfigFunc      func(ctx context.Context, scheduleID string) (map[string]any, error)
}

func (m *mockWSOperations) GetStates(ctx context.Context) ([]Entity, error) {
	if m.getStatesFunc != nil {
		return m.getStatesFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) GetState(ctx context.Context, entityID string) (*Entity, error) {
	if m.getStateFunc != nil {
		return m.getStateFunc(ctx, entityID)
	}
	return nil, nil
}

func (m *mockWSOperations) SetState(ctx context.Context, entityID string, state StateUpdate) (*Entity, error) {
	if m.setStateFunc != nil {
		return m.setStateFunc(ctx, entityID, state)
	}
	return nil, nil
}

func (m *mockWSOperations) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error) {
	if m.getHistoryFunc != nil {
		return m.getHistoryFunc(ctx, entityID, start, end)
	}
	return nil, nil
}

func (m *mockWSOperations) CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error) {
	if m.callServiceFunc != nil {
		return m.callServiceFunc(ctx, domain, service, data)
	}
	return nil, nil
}

func (m *mockWSOperations) ListAutomations(ctx context.Context) ([]Automation, error) {
	if m.listAutomationsFunc != nil {
		return m.listAutomationsFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) GetAutomation(ctx context.Context, automationID string) (*Automation, error) {
	if m.getAutomationFunc != nil {
		return m.getAutomationFunc(ctx, automationID)
	}
	return nil, nil
}

func (m *mockWSOperations) CreateAutomation(ctx context.Context, config AutomationConfig) error {
	if m.createAutomationFunc != nil {
		return m.createAutomationFunc(ctx, config)
	}
	return nil
}

func (m *mockWSOperations) UpdateAutomation(ctx context.Context, automationID string, config AutomationConfig) error {
	if m.updateAutomationFunc != nil {
		return m.updateAutomationFunc(ctx, automationID, config)
	}
	return nil
}

func (m *mockWSOperations) ToggleAutomation(ctx context.Context, entityID string, enabled bool) error {
	if m.toggleAutomationFunc != nil {
		return m.toggleAutomationFunc(ctx, entityID, enabled)
	}
	return nil
}

func (m *mockWSOperations) ListHelpers(ctx context.Context) ([]Entity, error) {
	if m.listHelpersFunc != nil {
		return m.listHelpersFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) CreateHelper(ctx context.Context, config HelperConfig) error {
	if m.createHelperFunc != nil {
		return m.createHelperFunc(ctx, config)
	}
	return nil
}

func (m *mockWSOperations) UpdateHelper(ctx context.Context, helperID string, config HelperConfig) error {
	if m.updateHelperFunc != nil {
		return m.updateHelperFunc(ctx, helperID, config)
	}
	return nil
}

func (m *mockWSOperations) DeleteHelper(ctx context.Context, helperID string) error {
	if m.deleteHelperFunc != nil {
		return m.deleteHelperFunc(ctx, helperID)
	}
	return nil
}

func (m *mockWSOperations) SetHelperValue(ctx context.Context, entityID string, value any) error {
	if m.setHelperValueFunc != nil {
		return m.setHelperValueFunc(ctx, entityID, value)
	}
	return nil
}

func (m *mockWSOperations) ListScripts(ctx context.Context) ([]Entity, error) {
	if m.listScriptsFunc != nil {
		return m.listScriptsFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) GetScript(ctx context.Context, scriptID string) (*Script, error) {
	if m.getScriptFunc != nil {
		return m.getScriptFunc(ctx, scriptID)
	}
	return nil, nil
}

func (m *mockWSOperations) CreateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	if m.createScriptFunc != nil {
		return m.createScriptFunc(ctx, scriptID, config)
	}
	return nil
}

func (m *mockWSOperations) UpdateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	if m.updateScriptFunc != nil {
		return m.updateScriptFunc(ctx, scriptID, config)
	}
	return nil
}

func (m *mockWSOperations) ListScenes(ctx context.Context) ([]Entity, error) {
	if m.listScenesFunc != nil {
		return m.listScenesFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) CreateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	if m.createSceneFunc != nil {
		return m.createSceneFunc(ctx, sceneID, config)
	}
	return nil
}

func (m *mockWSOperations) UpdateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	if m.updateSceneFunc != nil {
		return m.updateSceneFunc(ctx, sceneID, config)
	}
	return nil
}

func (m *mockWSOperations) GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error) {
	if m.getEntityRegistryFunc != nil {
		return m.getEntityRegistryFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error) {
	if m.getDeviceRegistryFunc != nil {
		return m.getDeviceRegistryFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error) {
	if m.getAreaRegistryFunc != nil {
		return m.getAreaRegistryFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) SignPath(ctx context.Context, path string, expires int) (string, error) {
	if m.signPathFunc != nil {
		return m.signPathFunc(ctx, path, expires)
	}
	return "", nil
}

func (m *mockWSOperations) GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error) {
	if m.getCameraStreamFunc != nil {
		return m.getCameraStreamFunc(ctx, entityID)
	}
	return nil, nil
}

func (m *mockWSOperations) BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error) {
	if m.browseMediaFunc != nil {
		return m.browseMediaFunc(ctx, mediaContentID)
	}
	return nil, nil
}

func (m *mockWSOperations) GetLovelaceConfig(ctx context.Context) (map[string]any, error) {
	if m.getLovelaceConfigFunc != nil {
		return m.getLovelaceConfigFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error) {
	if m.getStatisticsFunc != nil {
		return m.getStatisticsFunc(ctx, statIDs, period)
	}
	return nil, nil
}

func (m *mockWSOperations) GetTriggersForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	if m.getTriggersForTargetFunc != nil {
		return m.getTriggersForTargetFunc(ctx, target, expandGroup)
	}
	return nil, nil
}

func (m *mockWSOperations) GetConditionsForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	if m.getConditionsForTargetFunc != nil {
		return m.getConditionsForTargetFunc(ctx, target, expandGroup)
	}
	return nil, nil
}

func (m *mockWSOperations) GetServicesForTarget(ctx context.Context, target Target, expandGroup *bool) ([]string, error) {
	if m.getServicesForTargetFunc != nil {
		return m.getServicesForTargetFunc(ctx, target, expandGroup)
	}
	return nil, nil
}

func (m *mockWSOperations) ExtractFromTarget(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error) {
	if m.extractFromTargetFunc != nil {
		return m.extractFromTargetFunc(ctx, target, expandGroup)
	}
	return nil, nil
}

func (m *mockWSOperations) GetScheduleConfig(ctx context.Context, scheduleID string) (map[string]any, error) {
	if m.getScheduleConfigFunc != nil {
		return m.getScheduleConfigFunc(ctx, scheduleID)
	}
	return nil, nil
}

// mockRESTOperations implements RESTOperations for testing.
type mockRESTOperations struct {
	createAutomationFunc          func(ctx context.Context, config AutomationConfig) error
	updateAutomationFunc          func(ctx context.Context, automationID string, config AutomationConfig) error
	deleteAutomationFunc          func(ctx context.Context, automationID string) error
	createScriptFunc              func(ctx context.Context, scriptID string, config ScriptConfig) error
	updateScriptFunc              func(ctx context.Context, scriptID string, config ScriptConfig) error
	deleteScriptFunc              func(ctx context.Context, scriptID string) error
	createSceneFunc               func(ctx context.Context, sceneID string, config SceneConfig) error
	updateSceneFunc               func(ctx context.Context, sceneID string, config SceneConfig) error
	deleteSceneFunc               func(ctx context.Context, sceneID string) error
	initConfigEntryFlowFunc       func(ctx context.Context, handler string) (*ConfigEntryFlowResult, error)
	submitConfigEntryFlowStepFunc func(ctx context.Context, flowID string, data map[string]any) (*ConfigEntryFlowResult, error)
	deleteConfigEntryFunc         func(ctx context.Context, entryID string) error
	getServicesFunc               func(ctx context.Context) ([]Service, error)
	getConfigFunc                 func(ctx context.Context) (*Config, error)
	renderTemplateFunc            func(ctx context.Context, template string) (string, error)
	getLogbookFunc                func(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error)
	checkConfigFunc               func(ctx context.Context) (*ConfigCheckResult, error)
}

func (m *mockRESTOperations) CreateAutomation(ctx context.Context, config AutomationConfig) error {
	if m.createAutomationFunc != nil {
		return m.createAutomationFunc(ctx, config)
	}
	return nil
}

func (m *mockRESTOperations) UpdateAutomation(ctx context.Context, automationID string, config AutomationConfig) error {
	if m.updateAutomationFunc != nil {
		return m.updateAutomationFunc(ctx, automationID, config)
	}
	return nil
}

func (m *mockRESTOperations) DeleteAutomation(ctx context.Context, automationID string) error {
	if m.deleteAutomationFunc != nil {
		return m.deleteAutomationFunc(ctx, automationID)
	}
	return nil
}

func (m *mockRESTOperations) CreateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	if m.createScriptFunc != nil {
		return m.createScriptFunc(ctx, scriptID, config)
	}
	return nil
}

func (m *mockRESTOperations) UpdateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	if m.updateScriptFunc != nil {
		return m.updateScriptFunc(ctx, scriptID, config)
	}
	return nil
}

func (m *mockRESTOperations) DeleteScript(ctx context.Context, scriptID string) error {
	if m.deleteScriptFunc != nil {
		return m.deleteScriptFunc(ctx, scriptID)
	}
	return nil
}

func (m *mockRESTOperations) CreateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	if m.createSceneFunc != nil {
		return m.createSceneFunc(ctx, sceneID, config)
	}
	return nil
}

func (m *mockRESTOperations) UpdateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	if m.updateSceneFunc != nil {
		return m.updateSceneFunc(ctx, sceneID, config)
	}
	return nil
}

func (m *mockRESTOperations) DeleteScene(ctx context.Context, sceneID string) error {
	if m.deleteSceneFunc != nil {
		return m.deleteSceneFunc(ctx, sceneID)
	}
	return nil
}

func (m *mockRESTOperations) InitConfigEntryFlow(ctx context.Context, handler string) (*ConfigEntryFlowResult, error) {
	if m.initConfigEntryFlowFunc != nil {
		return m.initConfigEntryFlowFunc(ctx, handler)
	}
	return nil, nil
}

func (m *mockRESTOperations) SubmitConfigEntryFlowStep(ctx context.Context, flowID string, data map[string]any) (*ConfigEntryFlowResult, error) {
	if m.submitConfigEntryFlowStepFunc != nil {
		return m.submitConfigEntryFlowStepFunc(ctx, flowID, data)
	}
	return nil, nil
}

func (m *mockRESTOperations) DeleteConfigEntry(ctx context.Context, entryID string) error {
	if m.deleteConfigEntryFunc != nil {
		return m.deleteConfigEntryFunc(ctx, entryID)
	}
	return nil
}

func (m *mockRESTOperations) GetServices(ctx context.Context) ([]Service, error) {
	if m.getServicesFunc != nil {
		return m.getServicesFunc(ctx)
	}
	return nil, nil
}

func (m *mockRESTOperations) GetConfig(ctx context.Context) (*Config, error) {
	if m.getConfigFunc != nil {
		return m.getConfigFunc(ctx)
	}
	return nil, nil
}

func (m *mockRESTOperations) RenderTemplate(ctx context.Context, template string) (string, error) {
	if m.renderTemplateFunc != nil {
		return m.renderTemplateFunc(ctx, template)
	}
	return "", nil
}

func (m *mockRESTOperations) GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error) {
	if m.getLogbookFunc != nil {
		return m.getLogbookFunc(ctx, startTime, endTime, entityID)
	}
	return nil, nil
}

func (m *mockRESTOperations) CheckConfig(ctx context.Context) (*ConfigCheckResult, error) {
	if m.checkConfigFunc != nil {
		return m.checkConfigFunc(ctx)
	}
	return nil, nil
}

// Ensure mocks implement interfaces
var (
	_ WSOperations   = (*mockWSOperations)(nil)
	_ RESTOperations = (*mockRESTOperations)(nil)
)

// =============================================================================
// Comprehensive HybridClient tests using mocks
// =============================================================================

func TestHybridClient_WSOperations_GetStates(t *testing.T) {
	t.Parallel()

	expectedEntities := []Entity{
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "sensor.temperature", State: "22.5"},
	}

	mockWS := &mockWSOperations{
		getStatesFunc: func(_ context.Context) ([]Entity, error) {
			return expectedEntities, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	entities, err := client.GetStates(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 2 {
		t.Errorf("got %d entities, want 2", len(entities))
	}
}

func TestHybridClient_WSOperations_GetState(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		getStateFunc: func(_ context.Context, entityID string) (*Entity, error) {
			if entityID == "light.test" {
				return &Entity{EntityID: "light.test", State: "on"}, nil
			}
			return nil, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	entity, err := client.GetState(context.Background(), "light.test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity.State != "on" {
		t.Errorf("got state %q, want on", entity.State)
	}
}

func TestHybridClient_WSOperations_CallService(t *testing.T) {
	t.Parallel()

	called := false
	mockWS := &mockWSOperations{
		callServiceFunc: func(_ context.Context, domain, service string, _ map[string]any) ([]Entity, error) {
			called = true
			if domain != "light" || service != "turn_on" {
				t.Errorf("domain/service mismatch: %s.%s", domain, service)
			}
			return []Entity{}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	_, err := client.CallService(context.Background(), "light", "turn_on", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("CallService was not called")
	}
}

func TestHybridClient_WSOperations_ListAutomations(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		listAutomationsFunc: func(_ context.Context) ([]Automation, error) {
			return []Automation{
				{EntityID: "automation.test1"},
				{EntityID: "automation.test2"},
			}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	automations, err := client.ListAutomations(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(automations) != 2 {
		t.Errorf("got %d automations, want 2", len(automations))
	}
}

func TestHybridClient_WSOperations_GetAutomation(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		getAutomationFunc: func(_ context.Context, automationID string) (*Automation, error) {
			return &Automation{EntityID: "automation." + automationID}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	automation, err := client.GetAutomation(context.Background(), "test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if automation.EntityID != "automation.test" {
		t.Errorf("got entity ID %q, want automation.test", automation.EntityID)
	}
}

func TestHybridClient_RESTOperations_CreateAutomation(t *testing.T) {
	t.Parallel()

	called := false
	mockREST := &mockRESTOperations{
		createAutomationFunc: func(_ context.Context, config AutomationConfig) error {
			called = true
			if config.Alias != "Test Automation" {
				t.Errorf("got alias %q, want Test Automation", config.Alias)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(&mockWSOperations{}, mockREST)
	err := client.CreateAutomation(context.Background(), AutomationConfig{Alias: "Test Automation"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("REST CreateAutomation was not called")
	}
}

func TestHybridClient_RESTOperations_UpdateAutomation(t *testing.T) {
	t.Parallel()

	called := false
	mockREST := &mockRESTOperations{
		updateAutomationFunc: func(_ context.Context, automationID string, _ AutomationConfig) error {
			called = true
			if automationID != "test_automation" {
				t.Errorf("got automation ID %q, want test_automation", automationID)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(&mockWSOperations{}, mockREST)
	err := client.UpdateAutomation(context.Background(), "test_automation", AutomationConfig{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("REST UpdateAutomation was not called")
	}
}

func TestHybridClient_WSOperations_ToggleAutomation(t *testing.T) {
	t.Parallel()

	called := false
	mockWS := &mockWSOperations{
		toggleAutomationFunc: func(_ context.Context, entityID string, enabled bool) error {
			called = true
			if entityID != "automation.test" || !enabled {
				t.Errorf("got entityID=%s enabled=%v, want automation.test true", entityID, enabled)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	err := client.ToggleAutomation(context.Background(), "automation.test", true)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("ToggleAutomation was not called")
	}
}

func TestHybridClient_WSOperations_ListHelpers(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		listHelpersFunc: func(_ context.Context) ([]Entity, error) {
			return []Entity{{EntityID: "input_boolean.test"}}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	helpers, err := client.ListHelpers(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(helpers) != 1 {
		t.Errorf("got %d helpers, want 1", len(helpers))
	}
}

func TestHybridClient_WSOperations_CreateHelper(t *testing.T) {
	t.Parallel()

	called := false
	mockWS := &mockWSOperations{
		createHelperFunc: func(_ context.Context, _ HelperConfig) error {
			called = true
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	err := client.CreateHelper(context.Background(), HelperConfig{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("CreateHelper was not called")
	}
}

func TestHybridClient_WSOperations_UpdateHelper(t *testing.T) {
	t.Parallel()

	called := false
	mockWS := &mockWSOperations{
		updateHelperFunc: func(_ context.Context, helperID string, _ HelperConfig) error {
			called = true
			if helperID != "test_helper" {
				t.Errorf("got helperID %q, want test_helper", helperID)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	err := client.UpdateHelper(context.Background(), "test_helper", HelperConfig{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("UpdateHelper was not called")
	}
}

func TestHybridClient_WSOperations_DeleteHelper(t *testing.T) {
	t.Parallel()

	called := false
	mockWS := &mockWSOperations{
		deleteHelperFunc: func(_ context.Context, helperID string) error {
			called = true
			if helperID != "test_helper" {
				t.Errorf("got helperID %q, want test_helper", helperID)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	err := client.DeleteHelper(context.Background(), "test_helper")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("DeleteHelper was not called")
	}
}

func TestHybridClient_WSOperations_SetHelperValue(t *testing.T) {
	t.Parallel()

	called := false
	mockWS := &mockWSOperations{
		setHelperValueFunc: func(_ context.Context, entityID string, value any) error {
			called = true
			if entityID != "input_number.test" {
				t.Errorf("got entityID %q, want input_number.test", entityID)
			}
			if value != 42 {
				t.Errorf("got value %v, want 42", value)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	err := client.SetHelperValue(context.Background(), "input_number.test", 42)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("SetHelperValue was not called")
	}
}

func TestHybridClient_WSOperations_ListScripts(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		listScriptsFunc: func(_ context.Context) ([]Entity, error) {
			return []Entity{{EntityID: "script.test"}}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	scripts, err := client.ListScripts(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scripts) != 1 {
		t.Errorf("got %d scripts, want 1", len(scripts))
	}
}

func TestHybridClient_WSOperations_GetScript(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		getScriptFunc: func(_ context.Context, scriptID string) (*Script, error) {
			return &Script{EntityID: scriptID}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	script, err := client.GetScript(context.Background(), "test_script")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if script.EntityID != "test_script" {
		t.Errorf("got script EntityID %q, want test_script", script.EntityID)
	}
}

func TestHybridClient_RESTOperations_CreateScript(t *testing.T) {
	t.Parallel()

	called := false
	mockREST := &mockRESTOperations{
		createScriptFunc: func(_ context.Context, scriptID string, _ ScriptConfig) error {
			called = true
			if scriptID != "new_script" {
				t.Errorf("got scriptID %q, want new_script", scriptID)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(&mockWSOperations{}, mockREST)
	err := client.CreateScript(context.Background(), "new_script", ScriptConfig{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("REST CreateScript was not called")
	}
}

func TestHybridClient_RESTOperations_UpdateScript(t *testing.T) {
	t.Parallel()

	called := false
	mockREST := &mockRESTOperations{
		updateScriptFunc: func(_ context.Context, scriptID string, _ ScriptConfig) error {
			called = true
			if scriptID != "test_script" {
				t.Errorf("got scriptID %q, want test_script", scriptID)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(&mockWSOperations{}, mockREST)
	err := client.UpdateScript(context.Background(), "test_script", ScriptConfig{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("REST UpdateScript was not called")
	}
}

func TestHybridClient_WSOperations_ListScenes(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		listScenesFunc: func(_ context.Context) ([]Entity, error) {
			return []Entity{{EntityID: "scene.test"}}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	scenes, err := client.ListScenes(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scenes) != 1 {
		t.Errorf("got %d scenes, want 1", len(scenes))
	}
}

func TestHybridClient_RESTOperations_CreateScene(t *testing.T) {
	t.Parallel()

	called := false
	mockREST := &mockRESTOperations{
		createSceneFunc: func(_ context.Context, sceneID string, _ SceneConfig) error {
			called = true
			if sceneID != "new_scene" {
				t.Errorf("got sceneID %q, want new_scene", sceneID)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(&mockWSOperations{}, mockREST)
	err := client.CreateScene(context.Background(), "new_scene", SceneConfig{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("REST CreateScene was not called")
	}
}

func TestHybridClient_RESTOperations_UpdateScene(t *testing.T) {
	t.Parallel()

	called := false
	mockREST := &mockRESTOperations{
		updateSceneFunc: func(_ context.Context, sceneID string, _ SceneConfig) error {
			called = true
			if sceneID != "test_scene" {
				t.Errorf("got sceneID %q, want test_scene", sceneID)
			}
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(&mockWSOperations{}, mockREST)
	err := client.UpdateScene(context.Background(), "test_scene", SceneConfig{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("REST UpdateScene was not called")
	}
}

func TestHybridClient_WSOperations_Registries(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		getEntityRegistryFunc: func(_ context.Context) ([]EntityRegistryEntry, error) {
			return []EntityRegistryEntry{{EntityID: "light.test"}}, nil
		},
		getDeviceRegistryFunc: func(_ context.Context) ([]DeviceRegistryEntry, error) {
			return []DeviceRegistryEntry{{ID: "device1"}}, nil
		},
		getAreaRegistryFunc: func(_ context.Context) ([]AreaRegistryEntry, error) {
			return []AreaRegistryEntry{{AreaID: "area1"}}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})

	t.Run("GetEntityRegistry", func(t *testing.T) {
		entries, err := client.GetEntityRegistry(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("got %d entries, want 1", len(entries))
		}
	})

	t.Run("GetDeviceRegistry", func(t *testing.T) {
		entries, err := client.GetDeviceRegistry(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("got %d entries, want 1", len(entries))
		}
	})

	t.Run("GetAreaRegistry", func(t *testing.T) {
		entries, err := client.GetAreaRegistry(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("got %d entries, want 1", len(entries))
		}
	})
}

func TestHybridClient_WSOperations_Media(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		signPathFunc: func(_ context.Context, path string, _ int) (string, error) {
			return path + "?signed=true", nil
		},
		getCameraStreamFunc: func(_ context.Context, _ string) (*StreamInfo, error) {
			return &StreamInfo{URL: "http://stream"}, nil
		},
		browseMediaFunc: func(_ context.Context, _ string) (*MediaBrowseResult, error) {
			return &MediaBrowseResult{Title: "Media"}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})

	t.Run("SignPath", func(t *testing.T) {
		path, err := client.SignPath(context.Background(), "/api/test", 30)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "/api/test?signed=true" {
			t.Errorf("got path %q, want /api/test?signed=true", path)
		}
	})

	t.Run("GetCameraStream", func(t *testing.T) {
		stream, err := client.GetCameraStream(context.Background(), "camera.test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stream.URL != "http://stream" {
			t.Errorf("got URL %q, want http://stream", stream.URL)
		}
	})

	t.Run("BrowseMedia", func(t *testing.T) {
		result, err := client.BrowseMedia(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Title != "Media" {
			t.Errorf("got title %q, want Media", result.Title)
		}
	})
}

func TestHybridClient_WSOperations_Config(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		getLovelaceConfigFunc: func(_ context.Context) (map[string]any, error) {
			return map[string]any{"title": "Home"}, nil
		},
		getScheduleConfigFunc: func(_ context.Context, _ string) (map[string]any, error) {
			return map[string]any{"name": "Test Schedule"}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})

	t.Run("GetLovelaceConfig", func(t *testing.T) {
		config, err := client.GetLovelaceConfig(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config["title"] != "Home" {
			t.Errorf("got title %v, want Home", config["title"])
		}
	})

	t.Run("GetScheduleConfig", func(t *testing.T) {
		config, err := client.GetScheduleConfig(context.Background(), "schedule.test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config["name"] != "Test Schedule" {
			t.Errorf("got name %v, want Test Schedule", config["name"])
		}
	})
}

func TestHybridClient_WSOperations_Statistics(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		getStatisticsFunc: func(_ context.Context, statIDs []string, _ string) ([]StatisticsResult, error) {
			results := make([]StatisticsResult, len(statIDs))
			for i, id := range statIDs {
				results[i] = StatisticsResult{StatisticID: id}
			}
			return results, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	stats, err := client.GetStatistics(context.Background(), []string{"sensor.energy", "sensor.power"}, "hour")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 2 {
		t.Errorf("got %d stats, want 2", len(stats))
	}
}

func TestHybridClient_WSOperations_Targets(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		getTriggersForTargetFunc: func(_ context.Context, _ Target, _ *bool) ([]string, error) {
			return []string{"trigger1", "trigger2"}, nil
		},
		getConditionsForTargetFunc: func(_ context.Context, _ Target, _ *bool) ([]string, error) {
			return []string{"condition1"}, nil
		},
		getServicesForTargetFunc: func(_ context.Context, _ Target, _ *bool) ([]string, error) {
			return []string{"service1", "service2", "service3"}, nil
		},
		extractFromTargetFunc: func(_ context.Context, _ Target, _ *bool) (*ExtractFromTargetResult, error) {
			return &ExtractFromTargetResult{
				ReferencedEntities: []string{"light.test"},
			}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	target := Target{EntityID: []string{"light.test"}}

	t.Run("GetTriggersForTarget", func(t *testing.T) {
		triggers, err := client.GetTriggersForTarget(context.Background(), target, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(triggers) != 2 {
			t.Errorf("got %d triggers, want 2", len(triggers))
		}
	})

	t.Run("GetConditionsForTarget", func(t *testing.T) {
		conditions, err := client.GetConditionsForTarget(context.Background(), target, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(conditions) != 1 {
			t.Errorf("got %d conditions, want 1", len(conditions))
		}
	})

	t.Run("GetServicesForTarget", func(t *testing.T) {
		services, err := client.GetServicesForTarget(context.Background(), target, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(services) != 3 {
			t.Errorf("got %d services, want 3", len(services))
		}
	})

	t.Run("ExtractFromTarget", func(t *testing.T) {
		result, err := client.ExtractFromTarget(context.Background(), target, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.ReferencedEntities) != 1 {
			t.Errorf("got %d entities, want 1", len(result.ReferencedEntities))
		}
	})
}

func TestHybridClient_WSOperations_SetState(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		setStateFunc: func(_ context.Context, entityID string, state StateUpdate) (*Entity, error) {
			return &Entity{EntityID: entityID, State: state.State}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	entity, err := client.SetState(context.Background(), "sensor.test", StateUpdate{State: "42"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity.State != "42" {
		t.Errorf("got state %q, want 42", entity.State)
	}
}

func TestHybridClient_WSOperations_GetHistory(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		getHistoryFunc: func(_ context.Context, _ string, _, _ time.Time) ([][]HistoryEntry, error) {
			return [][]HistoryEntry{
				{{State: "on"}, {State: "off"}},
			}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})
	history, err := client.GetHistory(context.Background(), "light.test", time.Now().Add(-24*time.Hour), time.Now())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 1 || len(history[0]) != 2 {
		t.Errorf("unexpected history structure")
	}
}

func TestNewHybridClientWithInterfaces(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)

	if client == nil {
		t.Fatal("NewHybridClientWithInterfaces returned nil")
	}
	if client.ws != mockWS {
		t.Error("ws client mismatch")
	}
	if client.rest != mockREST {
		t.Error("rest client mismatch")
	}
}
