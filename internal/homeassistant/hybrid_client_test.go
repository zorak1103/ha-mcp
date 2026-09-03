package homeassistant

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
	getStatesFunc                 func(ctx context.Context) ([]Entity, error)
	getStateFunc                  func(ctx context.Context, entityID string) (*Entity, error)
	setStateFunc                  func(ctx context.Context, entityID string, state StateUpdate) (*Entity, error)
	getHistoryFunc                func(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error)
	callServiceFunc               func(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error)
	callServiceWithResponseFunc   func(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error)
	listAutomationsFunc           func(ctx context.Context) ([]Automation, error)
	getAutomationFunc             func(ctx context.Context, automationID string) (*Automation, error)
	createAutomationFunc          func(ctx context.Context, config AutomationConfig) error
	updateAutomationFunc          func(ctx context.Context, automationID string, config AutomationConfig) error
	toggleAutomationFunc          func(ctx context.Context, entityID string, enabled bool) error
	listHelpersFunc               func(ctx context.Context) ([]Entity, error)
	createHelperFunc              func(ctx context.Context, config HelperConfig) error
	updateHelperFunc              func(ctx context.Context, helperID string, config HelperConfig) error
	deleteHelperFunc              func(ctx context.Context, helperID string) error
	setHelperValueFunc            func(ctx context.Context, entityID string, value any) error
	listScriptsFunc               func(ctx context.Context) ([]Entity, error)
	getScriptFunc                 func(ctx context.Context, scriptID string) (*Script, error)
	createScriptFunc              func(ctx context.Context, scriptID string, config ScriptConfig) error
	updateScriptFunc              func(ctx context.Context, scriptID string, config ScriptConfig) error
	listScenesFunc                func(ctx context.Context) ([]Entity, error)
	createSceneFunc               func(ctx context.Context, sceneID string, config SceneConfig) error
	updateSceneFunc               func(ctx context.Context, sceneID string, config SceneConfig) error
	getEntityRegistryFunc         func(ctx context.Context) ([]EntityRegistryEntry, error)
	getEntityRegistryEntryFunc    func(ctx context.Context, entityID string) (*EntityRegistryEntry, error)
	updateEntityRegistryEntryFunc func(ctx context.Context, entityID string, config EntityRegistryUpdateConfig) (*EntityRegistryEntry, error)
	getDeviceRegistryFunc         func(ctx context.Context) ([]DeviceRegistryEntry, error)
	getAreaRegistryFunc           func(ctx context.Context) ([]AreaRegistryEntry, error)
	createAreaFunc                func(ctx context.Context, config AreaConfig) (*AreaRegistryEntry, error)
	updateAreaFunc                func(ctx context.Context, areaID string, config AreaConfig) (*AreaRegistryEntry, error)
	deleteAreaFunc                func(ctx context.Context, areaID string) error
	getLabelRegistryFunc          func(ctx context.Context) ([]LabelRegistryEntry, error)
	createLabelFunc               func(ctx context.Context, config LabelConfig) (*LabelRegistryEntry, error)
	updateLabelFunc               func(ctx context.Context, labelID string, config LabelConfig) (*LabelRegistryEntry, error)
	deleteLabelFunc               func(ctx context.Context, labelID string) error
	getFloorRegistryFunc          func(ctx context.Context) ([]FloorRegistryEntry, error)
	createFloorFunc               func(ctx context.Context, config FloorConfig) (*FloorRegistryEntry, error)
	updateFloorFunc               func(ctx context.Context, floorID string, config FloorConfig) (*FloorRegistryEntry, error)
	deleteFloorFunc               func(ctx context.Context, floorID string) error
	getZonesFunc                  func(ctx context.Context) ([]ZoneRegistryEntry, error)
	createZoneFunc                func(ctx context.Context, config ZoneConfig) (*ZoneRegistryEntry, error)
	updateZoneFunc                func(ctx context.Context, zoneID string, config ZoneConfig) (*ZoneRegistryEntry, error)
	deleteZoneFunc                func(ctx context.Context, zoneID string) error
	getPersonsFunc                func(ctx context.Context) ([]PersonRegistryEntry, error)
	createPersonFunc              func(ctx context.Context, config PersonConfig) (*PersonRegistryEntry, error)
	updatePersonFunc              func(ctx context.Context, personID string, config PersonConfig) (*PersonRegistryEntry, error)
	deletePersonFunc              func(ctx context.Context, personID string) error
	getTagsFunc                   func(ctx context.Context) ([]TagRegistryEntry, error)
	createTagFunc                 func(ctx context.Context, config TagConfig) (*TagRegistryEntry, error)
	updateTagFunc                 func(ctx context.Context, tagID string, config TagConfig) (*TagRegistryEntry, error)
	deleteTagFunc                 func(ctx context.Context, tagID string) error
	signPathFunc                  func(ctx context.Context, path string, expires int) (string, error)
	getCameraStreamFunc           func(ctx context.Context, entityID string) (*StreamInfo, error)
	browseMediaFunc               func(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error)
	getLovelaceConfigFunc         func(ctx context.Context, urlPath string) (map[string]any, error)
	saveLovelaceConfigFunc        func(ctx context.Context, urlPath string, config map[string]any) error
	listDashboardsFunc            func(ctx context.Context) ([]DashboardEntry, error)
	createDashboardFunc           func(ctx context.Context, config DashboardConfig) (*DashboardEntry, error)
	updateDashboardFunc           func(ctx context.Context, dashboardID string, config DashboardConfig) (*DashboardEntry, error)
	deleteDashboardFunc           func(ctx context.Context, dashboardID string) error
	getStatisticsFunc             func(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error)
	getTriggersForTargetFunc      func(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	getConditionsForTargetFunc    func(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	getServicesForTargetFunc      func(ctx context.Context, target Target, expandGroup *bool) ([]string, error)
	extractFromTargetFunc         func(ctx context.Context, target Target, expandGroup *bool) (*ExtractFromTargetResult, error)
	getHelperConfigFunc           func(ctx context.Context, platform, entityID string) (map[string]any, error)
	getConfigEntriesFunc          func(ctx context.Context, domain string) ([]ConfigEntryFull, error)
	getConfigEntryFunc            func(ctx context.Context, entryID string) (*ConfigEntryFull, error)
	sendHACSCommandFunc           func(ctx context.Context, command string, data map[string]any) (any, error)
	getSystemLogFunc              func(ctx context.Context) ([]SystemLogEntry, error)
	clearSystemLogFunc            func(ctx context.Context) error
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

func (m *mockWSOperations) CallServiceWithResponse(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error) {
	if m.callServiceWithResponseFunc != nil {
		return m.callServiceWithResponseFunc(ctx, domain, service, data)
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

func (m *mockWSOperations) GetEntityRegistryEntry(ctx context.Context, entityID string) (*EntityRegistryEntry, error) {
	if m.getEntityRegistryEntryFunc != nil {
		return m.getEntityRegistryEntryFunc(ctx, entityID)
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

func (m *mockWSOperations) CreateArea(ctx context.Context, config AreaConfig) (*AreaRegistryEntry, error) {
	if m.createAreaFunc != nil {
		return m.createAreaFunc(ctx, config)
	}
	return nil, nil
}

func (m *mockWSOperations) UpdateArea(ctx context.Context, areaID string, config AreaConfig) (*AreaRegistryEntry, error) {
	if m.updateAreaFunc != nil {
		return m.updateAreaFunc(ctx, areaID, config)
	}
	return nil, nil
}

func (m *mockWSOperations) DeleteArea(ctx context.Context, areaID string) error {
	if m.deleteAreaFunc != nil {
		return m.deleteAreaFunc(ctx, areaID)
	}
	return nil
}

func (m *mockWSOperations) GetLabelRegistry(ctx context.Context) ([]LabelRegistryEntry, error) {
	if m.getLabelRegistryFunc != nil {
		return m.getLabelRegistryFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) CreateLabel(ctx context.Context, config LabelConfig) (*LabelRegistryEntry, error) {
	if m.createLabelFunc != nil {
		return m.createLabelFunc(ctx, config)
	}
	return nil, nil
}

func (m *mockWSOperations) UpdateLabel(ctx context.Context, labelID string, config LabelConfig) (*LabelRegistryEntry, error) {
	if m.updateLabelFunc != nil {
		return m.updateLabelFunc(ctx, labelID, config)
	}
	return nil, nil
}

func (m *mockWSOperations) DeleteLabel(ctx context.Context, labelID string) error {
	if m.deleteLabelFunc != nil {
		return m.deleteLabelFunc(ctx, labelID)
	}
	return nil
}

func (m *mockWSOperations) GetFloorRegistry(ctx context.Context) ([]FloorRegistryEntry, error) {
	if m.getFloorRegistryFunc != nil {
		return m.getFloorRegistryFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) CreateFloor(ctx context.Context, config FloorConfig) (*FloorRegistryEntry, error) {
	if m.createFloorFunc != nil {
		return m.createFloorFunc(ctx, config)
	}
	return nil, nil
}

func (m *mockWSOperations) UpdateFloor(ctx context.Context, floorID string, config FloorConfig) (*FloorRegistryEntry, error) {
	if m.updateFloorFunc != nil {
		return m.updateFloorFunc(ctx, floorID, config)
	}
	return nil, nil
}

func (m *mockWSOperations) DeleteFloor(ctx context.Context, floorID string) error {
	if m.deleteFloorFunc != nil {
		return m.deleteFloorFunc(ctx, floorID)
	}
	return nil
}

func (m *mockWSOperations) GetZones(ctx context.Context) ([]ZoneRegistryEntry, error) {
	if m.getZonesFunc != nil {
		return m.getZonesFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) CreateZone(ctx context.Context, config ZoneConfig) (*ZoneRegistryEntry, error) {
	if m.createZoneFunc != nil {
		return m.createZoneFunc(ctx, config)
	}
	return nil, nil
}

func (m *mockWSOperations) UpdateZone(ctx context.Context, zoneID string, config ZoneConfig) (*ZoneRegistryEntry, error) {
	if m.updateZoneFunc != nil {
		return m.updateZoneFunc(ctx, zoneID, config)
	}
	return nil, nil
}

func (m *mockWSOperations) DeleteZone(ctx context.Context, zoneID string) error {
	if m.deleteZoneFunc != nil {
		return m.deleteZoneFunc(ctx, zoneID)
	}
	return nil
}

func (m *mockWSOperations) GetPersons(ctx context.Context) ([]PersonRegistryEntry, error) {
	if m.getPersonsFunc != nil {
		return m.getPersonsFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) CreatePerson(ctx context.Context, config PersonConfig) (*PersonRegistryEntry, error) {
	if m.createPersonFunc != nil {
		return m.createPersonFunc(ctx, config)
	}
	return &PersonRegistryEntry{ID: "person_123"}, nil
}

func (m *mockWSOperations) UpdatePerson(ctx context.Context, personID string, config PersonConfig) (*PersonRegistryEntry, error) {
	if m.updatePersonFunc != nil {
		return m.updatePersonFunc(ctx, personID, config)
	}
	return nil, nil
}

func (m *mockWSOperations) DeletePerson(ctx context.Context, personID string) error {
	if m.deletePersonFunc != nil {
		return m.deletePersonFunc(ctx, personID)
	}
	return nil
}

func (m *mockWSOperations) GetTags(ctx context.Context) ([]TagRegistryEntry, error) {
	if m.getTagsFunc != nil {
		return m.getTagsFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) CreateTag(ctx context.Context, config TagConfig) (*TagRegistryEntry, error) {
	if m.createTagFunc != nil {
		return m.createTagFunc(ctx, config)
	}
	return &TagRegistryEntry{TagID: "tag_123"}, nil
}

func (m *mockWSOperations) UpdateTag(ctx context.Context, tagID string, config TagConfig) (*TagRegistryEntry, error) {
	if m.updateTagFunc != nil {
		return m.updateTagFunc(ctx, tagID, config)
	}
	return nil, nil
}

func (m *mockWSOperations) DeleteTag(ctx context.Context, tagID string) error {
	if m.deleteTagFunc != nil {
		return m.deleteTagFunc(ctx, tagID)
	}
	return nil
}

func (m *mockWSOperations) RemoveEntityRegistryEntry(_ context.Context, _ string) error {
	return nil
}

func (m *mockWSOperations) UpdateEntityRegistryEntry(ctx context.Context, entityID string, config EntityRegistryUpdateConfig) (*EntityRegistryEntry, error) {
	if m.updateEntityRegistryEntryFunc != nil {
		return m.updateEntityRegistryEntryFunc(ctx, entityID, config)
	}
	return nil, nil
}

func (m *mockWSOperations) RemoveDeviceConfigEntry(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockWSOperations) UpdateDeviceRegistryEntry(_ context.Context, _ string, _ DeviceRegistryUpdateConfig) (*DeviceRegistryEntry, error) {
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

func (m *mockWSOperations) GetLovelaceConfig(ctx context.Context, urlPath string) (map[string]any, error) {
	if m.getLovelaceConfigFunc != nil {
		return m.getLovelaceConfigFunc(ctx, urlPath)
	}
	return nil, nil
}

func (m *mockWSOperations) SaveLovelaceConfig(ctx context.Context, urlPath string, config map[string]any) error {
	if m.saveLovelaceConfigFunc != nil {
		return m.saveLovelaceConfigFunc(ctx, urlPath, config)
	}
	return nil
}

func (m *mockWSOperations) ListDashboards(ctx context.Context) ([]DashboardEntry, error) {
	if m.listDashboardsFunc != nil {
		return m.listDashboardsFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) CreateDashboard(ctx context.Context, config DashboardConfig) (*DashboardEntry, error) {
	if m.createDashboardFunc != nil {
		return m.createDashboardFunc(ctx, config)
	}
	return nil, nil
}

func (m *mockWSOperations) UpdateDashboard(ctx context.Context, dashboardID string, config DashboardConfig) (*DashboardEntry, error) {
	if m.updateDashboardFunc != nil {
		return m.updateDashboardFunc(ctx, dashboardID, config)
	}
	return nil, nil
}

func (m *mockWSOperations) DeleteDashboard(ctx context.Context, dashboardID string) error {
	if m.deleteDashboardFunc != nil {
		return m.deleteDashboardFunc(ctx, dashboardID)
	}
	return nil
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

func (m *mockWSOperations) GetHelperConfig(ctx context.Context, platform, entityID string) (map[string]any, error) {
	if m.getHelperConfigFunc != nil {
		return m.getHelperConfigFunc(ctx, platform, entityID)
	}
	return nil, nil
}

func (m *mockWSOperations) GetConfigEntries(ctx context.Context, domain string) ([]ConfigEntryFull, error) {
	if m.getConfigEntriesFunc != nil {
		return m.getConfigEntriesFunc(ctx, domain)
	}
	return nil, nil
}

func (m *mockWSOperations) GetConfigEntry(ctx context.Context, entryID string) (*ConfigEntryFull, error) {
	if m.getConfigEntryFunc != nil {
		return m.getConfigEntryFunc(ctx, entryID)
	}
	return nil, nil
}

func (m *mockWSOperations) SendHACSCommand(ctx context.Context, command string, data map[string]any) (any, error) {
	if m.sendHACSCommandFunc != nil {
		return m.sendHACSCommandFunc(ctx, command, data)
	}
	return nil, nil
}

func (m *mockWSOperations) GetSystemLog(ctx context.Context) ([]SystemLogEntry, error) {
	if m.getSystemLogFunc != nil {
		return m.getSystemLogFunc(ctx)
	}
	return nil, nil
}

func (m *mockWSOperations) ClearSystemLog(ctx context.Context) error {
	if m.clearSystemLogFunc != nil {
		return m.clearSystemLogFunc(ctx)
	}
	return nil
}

// mockRESTOperations implements RESTOperations for testing.
type mockRESTOperations struct {
	createAutomationFunc                 func(ctx context.Context, config AutomationConfig) error
	updateAutomationFunc                 func(ctx context.Context, automationID string, config AutomationConfig) error
	deleteAutomationFunc                 func(ctx context.Context, automationID string) error
	createScriptFunc                     func(ctx context.Context, scriptID string, config ScriptConfig) error
	updateScriptFunc                     func(ctx context.Context, scriptID string, config ScriptConfig) error
	deleteScriptFunc                     func(ctx context.Context, scriptID string) error
	getSceneFunc                         func(ctx context.Context, sceneID string) (*Scene, error)
	createSceneFunc                      func(ctx context.Context, sceneID string, config SceneConfig) error
	updateSceneFunc                      func(ctx context.Context, sceneID string, config SceneConfig) error
	deleteSceneFunc                      func(ctx context.Context, sceneID string) error
	initConfigEntryFlowFunc              func(ctx context.Context, handler string) (*ConfigEntryFlowResult, error)
	submitConfigEntryFlowStepFunc        func(ctx context.Context, flowID string, data map[string]any) (*ConfigEntryFlowResult, error)
	deleteConfigEntryFunc                func(ctx context.Context, entryID string) (bool, error)
	getServicesFunc                      func(ctx context.Context) ([]Service, error)
	getConfigFunc                        func(ctx context.Context) (*Config, error)
	renderTemplateFunc                   func(ctx context.Context, template string) (string, error)
	getLogbookFunc                       func(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error)
	checkConfigFunc                      func(ctx context.Context) (*ConfigCheckResult, error)
	initConfigEntryOptionsFlowFunc       func(ctx context.Context, entryID string) (*OptionsFlowResult, error)
	submitConfigEntryOptionsFlowStepFunc func(ctx context.Context, flowID string, data map[string]any) (*OptionsFlowResult, error)
	abortConfigEntryOptionsFlowFunc      func(ctx context.Context, flowID string) error
	abortConfigEntryFlowFunc             func(ctx context.Context, flowID string) error
	getCalendarsFunc                     func(ctx context.Context) ([]CalendarEntry, error)
	getCalendarEventsFunc                func(ctx context.Context, entityID, start, end string) ([]CalendarEvent, error)
	getCameraSnapshotFunc                func(ctx context.Context, entityID string) ([]byte, string, error)
	configFileEntryExistsFunc            func(ctx context.Context, domain, configID string) (bool, error)
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

func (m *mockRESTOperations) GetScene(ctx context.Context, sceneID string) (*Scene, error) {
	if m.getSceneFunc != nil {
		return m.getSceneFunc(ctx, sceneID)
	}
	return nil, nil
}

func (m *mockRESTOperations) ConfigFileEntryExists(ctx context.Context, domain, configID string) (bool, error) {
	if m.configFileEntryExistsFunc != nil {
		return m.configFileEntryExistsFunc(ctx, domain, configID)
	}
	return true, nil
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

func (m *mockRESTOperations) DeleteConfigEntry(ctx context.Context, entryID string) (bool, error) {
	if m.deleteConfigEntryFunc != nil {
		return m.deleteConfigEntryFunc(ctx, entryID)
	}
	return false, nil
}

func (m *mockRESTOperations) InitConfigEntryOptionsFlow(ctx context.Context, entryID string) (*OptionsFlowResult, error) {
	if m.initConfigEntryOptionsFlowFunc != nil {
		return m.initConfigEntryOptionsFlowFunc(ctx, entryID)
	}
	return nil, nil
}

func (m *mockRESTOperations) SubmitConfigEntryOptionsFlowStep(ctx context.Context, flowID string, data map[string]any) (*OptionsFlowResult, error) {
	if m.submitConfigEntryOptionsFlowStepFunc != nil {
		return m.submitConfigEntryOptionsFlowStepFunc(ctx, flowID, data)
	}
	return nil, nil
}

func (m *mockRESTOperations) AbortConfigEntryFlow(ctx context.Context, flowID string) error {
	if m.abortConfigEntryFlowFunc != nil {
		return m.abortConfigEntryFlowFunc(ctx, flowID)
	}
	return nil
}

func (m *mockRESTOperations) AbortConfigEntryOptionsFlow(ctx context.Context, flowID string) error {
	if m.abortConfigEntryOptionsFlowFunc != nil {
		return m.abortConfigEntryOptionsFlowFunc(ctx, flowID)
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

func (m *mockRESTOperations) GetCalendars(ctx context.Context) ([]CalendarEntry, error) {
	if m.getCalendarsFunc != nil {
		return m.getCalendarsFunc(ctx)
	}
	return nil, nil
}

func (m *mockRESTOperations) GetCalendarEvents(ctx context.Context, entityID, start, end string) ([]CalendarEvent, error) {
	if m.getCalendarEventsFunc != nil {
		return m.getCalendarEventsFunc(ctx, entityID, start, end)
	}
	return nil, nil
}

func (m *mockRESTOperations) GetCameraSnapshot(ctx context.Context, entityID string) ([]byte, string, error) {
	if m.getCameraSnapshotFunc != nil {
		return m.getCameraSnapshotFunc(ctx, entityID)
	}
	return nil, "", nil
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

func TestHybridClient_UpdateHelper_ConfigEntryRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		entityID      string
		config        HelperConfig
		registryEntry []EntityRegistryEntry
		expectWS      bool
		expectREST    bool
	}{
		{
			name:     "config entry helper routes to options flow",
			entityID: "sensor.my_template",
			config:   HelperConfig{Platform: "template", Config: map[string]any{"state": "{{ 42 }}"}},
			registryEntry: []EntityRegistryEntry{
				{EntityID: "sensor.my_template", ConfigEntryID: "config123"},
			},
			expectWS:   false,
			expectREST: true,
		},
		{
			name:     "websocket helper uses websocket path",
			entityID: "counter.test",
			config:   HelperConfig{Platform: "counter", Config: map[string]any{"step": 1}},
			registryEntry: []EntityRegistryEntry{
				{EntityID: "counter.test", ConfigEntryID: ""},
			},
			expectWS:   true,
			expectREST: false,
		},
		{
			name:          "registry lookup failure falls back to websocket",
			entityID:      "input_boolean.test",
			config:        HelperConfig{Platform: "input_boolean", Config: map[string]any{}},
			registryEntry: nil, // Will cause GetEntityRegistry to fail
			expectWS:      true,
			expectREST:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wsCalled := false
			restCalled := false

			mockWS := &mockWSOperations{
				getEntityRegistryFunc: func(context.Context) ([]EntityRegistryEntry, error) {
					if tt.registryEntry == nil {
						return nil, fmt.Errorf("registry error")
					}
					return tt.registryEntry, nil
				},
				updateHelperFunc: func(context.Context, string, HelperConfig) error {
					wsCalled = true
					return nil
				},
				getConfigEntryFunc: func(context.Context, string) (*ConfigEntryFull, error) {
					return &ConfigEntryFull{EntryID: "config123", Domain: "template"}, nil
				},
			}

			mockREST := &mockRESTOperations{
				initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
					restCalled = true
					return &OptionsFlowResult{
						FlowID: "flow123",
						Type:   flowTypeForm,
						DataSchema: []OptionsFlowField{
							{Name: "state", Description: map[string]any{"suggested_value": "{{ old }}"}},
						},
					}, nil
				},
				submitConfigEntryOptionsFlowStepFunc: func(context.Context, string, map[string]any) (*OptionsFlowResult, error) {
					return &OptionsFlowResult{Type: "create_entry"}, nil
				},
			}

			client := NewHybridClientWithInterfaces(mockWS, mockREST)
			err := client.UpdateHelper(context.Background(), tt.entityID, tt.config)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectWS && !wsCalled {
				t.Error("expected WebSocket UpdateHelper to be called, but it wasn't")
			}
			if !tt.expectWS && wsCalled {
				t.Error("WebSocket UpdateHelper was called unexpectedly")
			}
			if tt.expectREST && !restCalled {
				t.Error("expected REST Options Flow to be called, but it wasn't")
			}
			if !tt.expectREST && restCalled {
				t.Error("REST Options Flow was called unexpectedly")
			}
		})
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

func TestHybridClient_RESTOperations_GetScene(t *testing.T) {
	t.Parallel()

	called := false
	mockREST := &mockRESTOperations{
		getSceneFunc: func(_ context.Context, sceneID string) (*Scene, error) {
			called = true
			if sceneID != "morning_scene" {
				t.Errorf("got sceneID %q, want morning_scene", sceneID)
			}
			return &Scene{EntityID: "scene.morning_scene", Config: &SceneConfig{Name: "Morning Scene"}}, nil
		},
	}

	client := NewHybridClientWithInterfaces(&mockWSOperations{}, mockREST)
	scene, err := client.GetScene(context.Background(), "morning_scene")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("REST GetScene was not called")
	}
	if scene == nil || scene.Config == nil || scene.Config.Name != "Morning Scene" {
		t.Errorf("unexpected scene: %+v", scene)
	}
}

func TestHybridClient_ConfigFileEntryExists_DelegatesToREST(t *testing.T) {
	t.Parallel()
	var gotDomain, gotConfigID string
	mockREST := &mockRESTOperations{
		configFileEntryExistsFunc: func(_ context.Context, domain, configID string) (bool, error) {
			gotDomain, gotConfigID = domain, configID
			return false, nil
		},
	}
	client := NewHybridClientWithInterfaces(&mockWSOperations{}, mockREST)

	exists, err := client.ConfigFileEntryExists(context.Background(), "script", "example_toggle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected exists=false to propagate from the REST layer")
	}
	if gotDomain != "script" || gotConfigID != "example_toggle" {
		t.Errorf("expected delegation with (script, example_toggle), got (%s, %s)", gotDomain, gotConfigID)
	}
}

func TestHybridClient_WSOperations_Registries(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{
		getEntityRegistryFunc: func(_ context.Context) ([]EntityRegistryEntry, error) {
			return []EntityRegistryEntry{{EntityID: "light.test"}}, nil
		},
		getEntityRegistryEntryFunc: func(_ context.Context, entityID string) (*EntityRegistryEntry, error) {
			return &EntityRegistryEntry{EntityID: entityID, UniqueID: "1700000000001"}, nil
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

	t.Run("GetEntityRegistryEntry", func(t *testing.T) {
		entry, err := client.GetEntityRegistryEntry(context.Background(), "automation.morning_routine")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry == nil || entry.UniqueID != "1700000000001" {
			t.Errorf("got %+v, want UniqueID = 1700000000001", entry)
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
		getLovelaceConfigFunc: func(_ context.Context, _ string) (map[string]any, error) {
			return map[string]any{"title": "Home"}, nil
		},
		getHelperConfigFunc: func(_ context.Context, _, _ string) (map[string]any, error) {
			return map[string]any{"name": "Test Schedule"}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, &mockRESTOperations{})

	t.Run("GetLovelaceConfig", func(t *testing.T) {
		config, err := client.GetLovelaceConfig(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config["title"] != "Home" {
			t.Errorf("got title %v, want Home", config["title"])
		}
	})

	t.Run("GetHelperConfig", func(t *testing.T) {
		config, err := client.GetHelperConfig(context.Background(), "schedule", "schedule.test")
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

// =============================================================================
// Group helper type detection tests
// =============================================================================

func TestDetermineGroupSubtype(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   HelperConfig
		expected string
	}{
		// Test with []string entity types
		{
			name: "light entities as []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"light.one", "light.two"}},
			},
			expected: domainLight,
		},
		{
			name: "sensor entities as []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"sensor.temp", "sensor.humidity"}},
			},
			expected: domainSensor,
		},
		{
			name: "binary_sensor entities as []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"binary_sensor.door", "binary_sensor.window"}},
			},
			expected: domainBinarySensor,
		},
		{
			name: "switch entities as []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"switch.plug1"}},
			},
			expected: domainSwitch,
		},
		{
			name: "cover entities as []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"cover.blind"}},
			},
			expected: domainCover,
		},
		{
			name: "fan entities as []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"fan.bedroom"}},
			},
			expected: domainFan,
		},
		{
			name: "lock entities as []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"lock.front_door"}},
			},
			expected: domainLock,
		},
		{
			name: "input_number entities as []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"input_number.value"}},
			},
			expected: domainSensor,
		},
		{
			name: "number entities as []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"number.value"}},
			},
			expected: domainSensor,
		},
		// Test with []any entity types (from JSON unmarshaling)
		{
			name: "light entities as []any",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []any{"light.one", "light.two"}},
			},
			expected: domainLight,
		},
		{
			name: "sensor entities as []any",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []any{"sensor.temp"}},
			},
			expected: domainSensor,
		},
		{
			name: "binary_sensor entities as []any",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []any{"binary_sensor.motion"}},
			},
			expected: domainBinarySensor,
		},
		{
			name: "cover entities as []any",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []any{"cover.garage"}},
			},
			expected: domainCover,
		},
		// Edge cases
		{
			name: "empty entities",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{}},
			},
			expected: domainSensor,
		},
		{
			name: "nil entities",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{},
			},
			expected: domainSensor,
		},
		{
			name: "unknown domain",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"unknown.entity"}},
			},
			expected: domainSensor,
		},
		{
			name: "invalid entity format",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"no_dot_here"}},
			},
			expected: domainSensor,
		},
	}

	client := &HybridClient{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := client.determineGroupSubtype(tt.config)
			if result != tt.expected {
				t.Errorf("determineGroupSubtype() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestAddSensorGroupDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		config        HelperConfig
		result        map[string]any
		wantTypeAdded bool
		wantTypeValue string
	}{
		// Sensor groups should get default type
		{
			name: "sensor group without type - []string",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"sensor.temp"}},
			},
			result:        map[string]any{},
			wantTypeAdded: true,
			wantTypeValue: "sum",
		},
		{
			name: "sensor group without type - []any",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []any{"sensor.temp"}},
			},
			result:        map[string]any{},
			wantTypeAdded: true,
			wantTypeValue: "sum",
		},
		{
			name: "input_number group without type",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"input_number.val"}},
			},
			result:        map[string]any{},
			wantTypeAdded: true,
			wantTypeValue: "sum",
		},
		{
			name: "number group without type",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"number.val"}},
			},
			result:        map[string]any{},
			wantTypeAdded: true,
			wantTypeValue: "sum",
		},
		// Sensor group with existing type should not be overridden
		{
			name: "sensor group with existing type",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"sensor.temp"}},
			},
			result:        map[string]any{"type": "mean"},
			wantTypeAdded: false,
			wantTypeValue: "mean",
		},
		// Non-sensor groups should NOT get type added
		{
			name: "light group",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"light.one"}},
			},
			result:        map[string]any{},
			wantTypeAdded: false,
		},
		{
			name: "light group - []any",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []any{"light.one"}},
			},
			result:        map[string]any{},
			wantTypeAdded: false,
		},
		{
			name: "binary_sensor group",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"binary_sensor.door"}},
			},
			result:        map[string]any{},
			wantTypeAdded: false,
		},
		{
			name: "switch group",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"switch.plug"}},
			},
			result:        map[string]any{},
			wantTypeAdded: false,
		},
		{
			name: "cover group",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{"cover.blind"}},
			},
			result:        map[string]any{},
			wantTypeAdded: false,
		},
		// Non-group platform should be ignored
		{
			name: "non-group platform",
			config: HelperConfig{
				Platform: "threshold",
				Config:   map[string]any{"entities": []string{"sensor.temp"}},
			},
			result:        map[string]any{},
			wantTypeAdded: false,
		},
		// Empty entities
		{
			name: "empty entities",
			config: HelperConfig{
				Platform: "group",
				Config:   map[string]any{"entities": []string{}},
			},
			result:        map[string]any{},
			wantTypeAdded: false,
		},
	}

	client := &HybridClient{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Make a copy of result to avoid test interference
			resultCopy := make(map[string]any)
			for k, v := range tt.result {
				resultCopy[k] = v
			}

			client.addSensorGroupDefaults(tt.config, resultCopy)

			typeVal, hasType := resultCopy["type"]
			if tt.wantTypeAdded {
				if !hasType {
					t.Errorf("expected type to be added, but it wasn't")
				} else if typeVal != tt.wantTypeValue {
					t.Errorf("type = %q, want %q", typeVal, tt.wantTypeValue)
				}
			} else {
				if hasType && tt.result["type"] == nil {
					t.Errorf("type was added when it shouldn't have been: %v", typeVal)
				}
				// If tt.result had a type originally, verify it wasn't changed
				if originalType, hadOriginal := tt.result["type"]; hadOriginal {
					if typeVal != originalType {
						t.Errorf("type was changed from %v to %v", originalType, typeVal)
					}
				}
			}
		})
	}
}

// TestExtractOptionsFromSchema_SkipsNilSuggestedValue is a regression test for a
// bug where a field whose suggested_value is JSON null (key present, value nil -
// which Home Assistant reports for an optional field left unset at creation)
// was included in the extracted options map as a literal nil, which then got
// resubmitted to Home Assistant's Options Flow API as an explicit null and
// rejected as a type error. A field must be omitted from the result exactly
// like a genuinely-absent field would be - but a real zero value (e.g. 0.0)
// must NOT be treated the same as nil and must still be included.
// TestGetConfigEntryOptions_WalksAllStepsAndAborts is a regression test for
// #202's underlying visibility gap: reading only the first form step's
// schema made every value living on a later step (e.g. generic_thermostat's
// presets) invisible to manage_helper get_details, even though update could
// (after this fix) write them. This asserts every step's values are merged
// into the result. Neither mock response sets LastStep (simulating an HA
// version, or a step, that doesn't report it), so the walk must submit the
// final step to discover there is nothing more - which does commit it
// (create_entry), a harmless no-op since only round-tripped current values
// are ever sent. The flow is aborted afterward regardless.
func TestGetConfigEntryOptions_WalksAllStepsAndAborts(t *testing.T) {
	t.Parallel()

	var abortedFlowID string
	step := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow-read",
				Type:   flowTypeForm,
				StepID: "init",
				DataSchema: []OptionsFlowField{
					{Name: "heater", Description: map[string]any{"suggested_value": "switch.heater"}},
				},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(context.Context, string, map[string]any) (*OptionsFlowResult, error) {
			step++
			if step == 1 {
				return &OptionsFlowResult{
					FlowID: "flow-read",
					Type:   flowTypeForm,
					StepID: "presets",
					DataSchema: []OptionsFlowField{
						{Name: "away_temp", Description: map[string]any{"suggested_value": 16.0}},
					},
				}, nil
			}
			return &OptionsFlowResult{FlowID: "flow-read", Type: flowTypeCreateEntry}, nil
		},
		abortConfigEntryOptionsFlowFunc: func(_ context.Context, flowID string) error {
			abortedFlowID = flowID
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	options, err := client.GetConfigEntryOptions(context.Background(), "config-read")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{"heater": "switch.heater", "away_temp": 16.0}
	if !reflect.DeepEqual(options, want) {
		t.Errorf("GetConfigEntryOptions() = %v, want %v (values from both steps merged)", options, want)
	}
	if step != 2 {
		t.Fatalf("got %d submissions, want 2 (submitting the last step is how its absence of a successor is discovered)", step)
	}
	if abortedFlowID != "flow-read" {
		t.Errorf("aborted flow id = %q, want %q", abortedFlowID, "flow-read")
	}
}

// TestGetConfigEntryOptions_StopsAtLastStepWithoutSubmittingIt is the
// side-effect-free counterpart: when HA reports last_step:true on a step's
// own response, that step's values can be read directly from its schema
// without ever submitting it - avoiding the harmless-but-unnecessary
// create_entry commit the fallback path above accepts.
func TestGetConfigEntryOptions_StopsAtLastStepWithoutSubmittingIt(t *testing.T) {
	t.Parallel()

	trueVal := true
	submitCount := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow-last",
				Type:   flowTypeForm,
				StepID: "init",
				DataSchema: []OptionsFlowField{
					{Name: "heater", Description: map[string]any{"suggested_value": "switch.heater"}},
				},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(context.Context, string, map[string]any) (*OptionsFlowResult, error) {
			submitCount++
			return &OptionsFlowResult{
				FlowID:   "flow-last",
				Type:     flowTypeForm,
				StepID:   "presets",
				LastStep: &trueVal,
				DataSchema: []OptionsFlowField{
					{Name: "away_temp", Description: map[string]any{"suggested_value": 16.0}},
				},
			}, nil
		},
		abortConfigEntryOptionsFlowFunc: func(context.Context, string) error { return nil },
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	options, err := client.GetConfigEntryOptions(context.Background(), "config-last")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{"heater": "switch.heater", "away_temp": 16.0}
	if !reflect.DeepEqual(options, want) {
		t.Errorf("GetConfigEntryOptions() = %v, want %v", options, want)
	}
	if submitCount != 1 {
		t.Errorf("submit called %d times, want 1 (the last step's values are read from its own response, never submitted)", submitCount)
	}
}

func TestExtractOptionsFromSchema_SkipsNilSuggestedValue(t *testing.T) {
	t.Parallel()

	schema := []OptionsFlowField{
		{Name: "upper", Description: map[string]any{"suggested_value": 50.0}},
		{Name: "lower", Description: map[string]any{"suggested_value": nil}},
		{Name: "hysteresis", Description: map[string]any{"suggested_value": 0.0}},
	}

	options := extractOptionsFromSchema(schema)

	if _, present := options["lower"]; present {
		t.Errorf("extractOptionsFromSchema should omit a field whose suggested_value is nil (JSON null), got: %v", options)
	}
	if options["upper"] != 50.0 {
		t.Errorf("upper should be 50.0, got %v", options["upper"])
	}
	if options["hysteresis"] != 0.0 {
		t.Errorf("hysteresis (a real zero value, not nil) must still be included, got %v", options["hysteresis"])
	}
}

// TestUpdateHelperViaOptionsFlow_NameAppliedViaRegistry is a regression test
// for a bug where "name" was forwarded into the Options Flow submission like
// any other config field. Most config-entry helper types (min_max, filter,
// ...) declare no "name" field in their Options Flow schema at all, so the
// display name must be applied via the Entity Registry - the same place
// "icon" is already applied - not submitted as part of the flow.
func TestUpdateHelperViaOptionsFlow_NameAppliedViaRegistry(t *testing.T) {
	t.Parallel()

	var registryUpdateConfig EntityRegistryUpdateConfig
	registryUpdateCalled := false

	mockWS := &mockWSOperations{
		getStateFunc: func(context.Context, string) (*Entity, error) {
			return &Entity{EntityID: "sensor.my_min_max"}, nil
		},
		updateEntityRegistryEntryFunc: func(_ context.Context, entityID string, config EntityRegistryUpdateConfig) (*EntityRegistryEntry, error) {
			registryUpdateCalled = true
			registryUpdateConfig = config
			if entityID != "sensor.my_min_max" {
				t.Errorf("got entityID %q, want sensor.my_min_max", entityID)
			}
			return nil, nil
		},
	}

	var submittedData map[string]any
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow123",
				Type:   "form",
				DataSchema: []OptionsFlowField{
					{Name: "round_digits", Description: map[string]any{"suggested_value": 2.0}},
				},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*OptionsFlowResult, error) {
			submittedData = data
			return &OptionsFlowResult{Type: "create_entry"}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "sensor.my_min_max", "config123", HelperConfig{
		Config: map[string]any{"name": "New Name", "round_digits": 3.0},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registryUpdateCalled {
		t.Fatal("UpdateEntityRegistryEntry was not called for the name change")
	}
	if registryUpdateConfig.Name == nil || *registryUpdateConfig.Name != "New Name" {
		t.Errorf("registry update Name = %v, want \"New Name\"", registryUpdateConfig.Name)
	}
	if _, present := submittedData["name"]; present {
		t.Errorf("submitted options flow data should not include \"name\", got: %v", submittedData)
	}
}

// TestUpdateHelperViaOptionsFlow_UnsupportedFieldFailsLoudly is a regression
// test for a bug where a field this helper's Options Flow schema doesn't
// declare was silently dropped and the update still reported success -
// discarding a change the caller explicitly asked for without any
// indication in the response. The unconsumed-field check now runs after
// every step has had a chance to claim the field (so a later step's field
// isn't mistaken for unsupported - see the sibling test below), which means
// a single-step flow's one submission does go out to HA before the error is
// returned; the check still fails loudly rather than reporting success.
func TestUpdateHelperViaOptionsFlow_UnsupportedFieldFailsLoudly(t *testing.T) {
	t.Parallel()

	submitCount := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow123",
				Type:   flowTypeForm,
				DataSchema: []OptionsFlowField{
					{Name: "round_digits", Description: map[string]any{"suggested_value": 2.0}},
				},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(context.Context, string, map[string]any) (*OptionsFlowResult, error) {
			submitCount++
			return &OptionsFlowResult{Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "sensor.my_min_max", "config123", HelperConfig{
		Config: map[string]any{"min_max_type": "max"},
	})

	if err == nil {
		t.Fatal("expected an error for an unsupported field, got nil")
	}
	if !strings.Contains(err.Error(), "min_max_type") {
		t.Errorf("error should name the unsupported field, got: %v", err)
	}
	if submitCount > 1 {
		t.Errorf("submit called %d times, want at most 1 (the flow's only step)", submitCount)
	}
}

// TestUpdateHelperViaOptionsFlow_UnsupportedFieldFailsAfterAllStepsTried
// proves the other side of the same fix (issue #202's underlying cause): a
// field that belongs to the flow's SECOND step must not be reported as
// unsupported just because the FIRST step's schema doesn't declare it.
func TestUpdateHelperViaOptionsFlow_UnsupportedFieldFailsAfterAllStepsTried(t *testing.T) {
	t.Parallel()

	step := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID:     "flow200",
				Type:       flowTypeForm,
				StepID:     "init",
				DataSchema: []OptionsFlowField{{Name: "entity_id"}},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*OptionsFlowResult, error) {
			step++
			if step == 1 {
				return &OptionsFlowResult{
					FlowID:     "flow200",
					Type:       flowTypeForm,
					StepID:     "second",
					DataSchema: []OptionsFlowField{{Name: "second_field"}},
				}, nil
			}
			if _, present := data["second_field"]; !present {
				t.Errorf("second step payload = %#v, want second_field routed there", data)
			}
			return &OptionsFlowResult{Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "sensor.my_helper", "config200", HelperConfig{
		Config: map[string]any{"second_field": "value"},
	})

	if err != nil {
		t.Fatalf("second_field belongs to the flow's second step and must not be reported unsupported: %v", err)
	}
	if step != 2 {
		t.Fatalf("got %d submissions, want 2", step)
	}
}

// TestUpdateHelperViaOptionsFlow_GenericThermostatPresetsStepSubmitsEmptyConfig
// guards issue #194's "related, unverified risk": generic_thermostat's
// OPTIONS_FLOW has the same "init" -> "presets" shape as its CONFIG_FLOW.
// Before this fix, updateHelperViaOptionsFlow submitted once and hard-failed
// on any non-create_entry result ("unexpected options flow result type:
// form") - every generic_thermostat update hit this, since the flow always
// advances to "presets" after "init".
// TestUpdateHelperViaOptionsFlow_GenericThermostatPresetsStepPreservesExistingValues
// guards issue #194's related critical bug found in adversarial review:
// generic_thermostat's OPTIONS_FLOW has the same "init" -> "presets" shape as
// its CONFIG_FLOW, and HA's SchemaCommonFlowHandler deletes any vol.Optional
// key of a step's schema that is absent from that step's submission
// (_update_and_remove_omitted_optional_keys). Submitting an empty map to the
// presets step - safe on create, where there is nothing to delete - would
// silently erase every stored preset temperature (away/eco/home/...) on
// every single update. This asserts the presets step is resubmitted with its
// own current suggested_value fields instead of an empty map, and that a
// genuinely-unset preset (nil suggested_value) is correctly omitted rather
// than fabricated.
func TestUpdateHelperViaOptionsFlow_GenericThermostatPresetsStepPreservesExistingValues(t *testing.T) {
	t.Parallel()

	var submittedPayloads []map[string]any
	step := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow194",
				Type:   flowTypeForm,
				StepID: "init",
				DataSchema: []OptionsFlowField{
					{Name: "heater", Description: map[string]any{"suggested_value": "switch.heater"}},
					{Name: "target_sensor", Description: map[string]any{"suggested_value": "sensor.temp"}},
					{Name: "ac_mode", Description: map[string]any{"suggested_value": false}},
				},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*OptionsFlowResult, error) {
			submittedPayloads = append(submittedPayloads, data)
			step++
			if step == 1 {
				return &OptionsFlowResult{
					FlowID: "flow194",
					Type:   flowTypeForm,
					StepID: "presets",
					DataSchema: []OptionsFlowField{
						{Name: "away_temp", Description: map[string]any{"suggested_value": 16.0}},
						{Name: "comfort_temp", Description: map[string]any{"suggested_value": nil}},
					},
				}, nil
			}
			return &OptionsFlowResult{FlowID: "flow194", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "climate.my_thermostat", "config194", HelperConfig{
		Platform: "climate", // update path's Platform is the entity domain, not the helper type
		Config: map[string]any{
			"ac_mode": true,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(submittedPayloads) != 2 {
		t.Fatalf("got %d submissions, want 2", len(submittedPayloads))
	}
	presetsPayload := submittedPayloads[1]
	if presetsPayload["away_temp"] != 16.0 {
		t.Errorf("presets step payload = %#v, want away_temp preserved as 16.0", presetsPayload)
	}
	if _, present := presetsPayload["comfort_temp"]; present {
		t.Errorf("presets step payload = %#v, want unset comfort_temp omitted, not fabricated", presetsPayload)
	}
	if len(presetsPayload) != 1 {
		t.Errorf("presets step payload = %#v, want exactly 1 field (away_temp)", presetsPayload)
	}
}

// TestUpdateHelperViaOptionsFlow_PresetFieldReachesPresetsStep is issue
// #202's core regression test: away_temp belongs to the "presets" step,
// not "init", and must be routed there instead of being rejected against
// "init"'s schema (which doesn't declare it) - the bug the deferred
// unconsumed-field check in PR2's flow engine exists to fix.
func TestUpdateHelperViaOptionsFlow_PresetFieldReachesPresetsStep(t *testing.T) {
	t.Parallel()

	var submittedPayloads []map[string]any
	step := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow202",
				Type:   flowTypeForm,
				StepID: "init",
				DataSchema: []OptionsFlowField{
					{Name: "heater", Description: map[string]any{"suggested_value": "switch.heater"}},
					{Name: "target_sensor", Description: map[string]any{"suggested_value": "sensor.temp"}},
					{Name: "ac_mode", Description: map[string]any{"suggested_value": false}},
				},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*OptionsFlowResult, error) {
			submittedPayloads = append(submittedPayloads, data)
			step++
			if step == 1 {
				return &OptionsFlowResult{
					FlowID: "flow202",
					Type:   flowTypeForm,
					StepID: "presets",
					DataSchema: []OptionsFlowField{
						{Name: "away_temp", Description: map[string]any{"suggested_value": 16.0}},
						{Name: "eco_temp", Description: map[string]any{"suggested_value": nil}},
					},
				}, nil
			}
			return &OptionsFlowResult{FlowID: "flow202", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "climate.my_thermostat", "config202", HelperConfig{
		Platform: "climate",
		Config:   map[string]any{"away_temp": 15.0},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(submittedPayloads) != 2 {
		t.Fatalf("got %d submissions, want 2", len(submittedPayloads))
	}
	if _, present := submittedPayloads[0]["away_temp"]; present {
		t.Errorf("init step payload = %#v, want away_temp NOT present - init's schema doesn't declare it", submittedPayloads[0])
	}
	initPayload := submittedPayloads[0]
	if initPayload["heater"] != "switch.heater" || initPayload["target_sensor"] != "sensor.temp" || initPayload["ac_mode"] != false {
		t.Errorf("init step payload = %#v, want heater/target_sensor/ac_mode round-tripped from their suggested_value", initPayload)
	}
	presetsPayload := submittedPayloads[1]
	if presetsPayload["away_temp"] != 15.0 {
		t.Errorf("presets step payload = %#v, want away_temp overridden to 15.0", presetsPayload)
	}
	if _, present := presetsPayload["eco_temp"]; present {
		t.Errorf("presets step payload = %#v, want eco_temp NOT present - its suggested_value is nil (never set), must not be fabricated", presetsPayload)
	}
}

// TestUpdateHelperViaOptionsFlow_UnknownFieldStillFailsAfterPresetsStep
// guards the other side: a field no step's schema declares (not even the
// presets step) must still be rejected, proving the deferred check doesn't
// degrade into accepting anything.
func TestUpdateHelperViaOptionsFlow_UnknownFieldStillFailsAfterPresetsStep(t *testing.T) {
	t.Parallel()

	step := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID:     "flow202b",
				Type:       flowTypeForm,
				StepID:     "init",
				DataSchema: []OptionsFlowField{{Name: "heater"}},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(context.Context, string, map[string]any) (*OptionsFlowResult, error) {
			step++
			if step == 1 {
				return &OptionsFlowResult{
					FlowID:     "flow202b",
					Type:       flowTypeForm,
					StepID:     "presets",
					DataSchema: []OptionsFlowField{{Name: "away_temp"}},
				}, nil
			}
			return &OptionsFlowResult{FlowID: "flow202b", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "climate.my_thermostat", "config202b", HelperConfig{
		Platform: "climate",
		Config:   map[string]any{"nonsense_temp": 1.0},
	})

	if err == nil || !strings.Contains(err.Error(), "nonsense_temp") {
		t.Fatalf("got error %v, want it to name the unrecognized field nonsense_temp", err)
	}
}

// TestUpdateHelperViaOptionsFlow_UnconsumedFieldErrorSaysUpdateWasApplied
// pins W1's remediated wording: by the time the unconsumed-field check
// runs, HA has already committed the fields every step DID claim (e.g.
// heater below) - the error must say the update was applied and only the
// named field(s) were not, not read like nothing happened at all (a
// caller retrying the whole update on a "does not support updating"-style
// message could plausibly resubmit already-applied fields unnecessarily,
// or worse, treat the helper as unchanged).
func TestUpdateHelperViaOptionsFlow_UnconsumedFieldErrorSaysUpdateWasApplied(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID:     "flowW1",
				Type:       flowTypeForm,
				StepID:     "init",
				DataSchema: []OptionsFlowField{{Name: "heater"}},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(context.Context, string, map[string]any) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{FlowID: "flowW1", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "climate.my_thermostat", "configW1", HelperConfig{
		Platform: "climate",
		Config:   map[string]any{"heater": "switch.heater", "nonsense_temp": 1.0},
	})

	if err == nil {
		t.Fatal("expected an error for an unsupported field, got nil")
	}
	if !strings.Contains(err.Error(), "nonsense_temp") {
		t.Errorf("error should name the unaccepted field, got: %v", err)
	}
	if !strings.Contains(err.Error(), "was updated") {
		t.Errorf("error should state the update was applied, not read as if nothing happened, got: %v", err)
	}
	if !strings.Contains(err.Error(), "NOT been applied") {
		t.Errorf("error should clarify the unaccepted field specifically was NOT applied, got: %v", err)
	}
}

// TestUpdateHelperViaOptionsFlow_GenericThermostatPresetsStepSubmitErrorAborts
// guards submitOptionsFlowPresetsStep's own abort path, uncovered before this
// test: if the presets-step submission itself fails, the flow must be
// aborted with the flow id rather than leaked.
func TestUpdateHelperViaOptionsFlow_GenericThermostatPresetsStepSubmitErrorAborts(t *testing.T) {
	t.Parallel()

	step := 0
	var abortedFlowID string
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{FlowID: "flow194", Type: flowTypeForm, StepID: "init"}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(context.Context, string, map[string]any) (*OptionsFlowResult, error) {
			step++
			if step == 1 {
				return &OptionsFlowResult{FlowID: "flow194", Type: flowTypeForm, StepID: "presets"}, nil
			}
			return nil, fmt.Errorf("simulated HA error")
		},
		abortConfigEntryOptionsFlowFunc: func(_ context.Context, flowID string) error {
			abortedFlowID = flowID
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "climate.my_thermostat", "config194", HelperConfig{
		Platform: "climate",
		Config:   map[string]any{},
	})

	if err == nil {
		t.Fatal("expected an error when the presets step submission fails")
	}
	if abortedFlowID != "flow194" {
		t.Errorf("aborted flow id = %q, want \"flow194\"", abortedFlowID)
	}
}

// TestUpdateHelperViaOptionsFlow_UnexpectedResultTypeAbortsInitFlowID is a
// regression test for an adversarial-review finding: the abort call on an
// unexpected terminal result type used submitResult.FlowID (whatever HA's
// last response happened to contain) instead of the flow id captured at
// init/menu-navigation time, which is guaranteed non-empty and stable across
// every step of the flow.
func TestUpdateHelperViaOptionsFlow_UnexpectedResultTypeAbortsInitFlowID(t *testing.T) {
	t.Parallel()

	var abortedFlowID string
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{FlowID: "flow-init", Type: flowTypeForm, StepID: "init"}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(context.Context, string, map[string]any) (*OptionsFlowResult, error) {
			// A second, unrelated form step (not "presets") - the result
			// type stays "form", so the caller's create_entry check must
			// fail, and the abort must use the stable init flow id, not
			// this response's own (possibly empty) flow id.
			return &OptionsFlowResult{FlowID: "", Type: flowTypeForm, StepID: "unexpected_step"}, nil
		},
		abortConfigEntryOptionsFlowFunc: func(_ context.Context, flowID string) error {
			abortedFlowID = flowID
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "climate.my_thermostat", "config194", HelperConfig{
		Platform: "climate",
		Config:   map[string]any{},
	})

	if err == nil || !strings.Contains(err.Error(), "unexpected options flow result type") {
		t.Fatalf("got error %v, want \"unexpected options flow result type\"", err)
	}
	if abortedFlowID != "flow-init" {
		t.Errorf("aborted flow id = %q, want \"flow-init\" (the stable init flow id, not the last response's)", abortedFlowID)
	}
}

// TestUpdateHelperViaOptionsFlow_ValidationErrorsSurfaceHAReason is a
// regression test for an adversarial-review finding: unlike
// createHelperViaConfigFlow (which surfaces HA's validation errors
// verbatim), updateHelperViaOptionsFlow discarded submitResult.Errors on a
// validation failure, reporting only the opaque "unexpected options flow
// result type: form" instead of HA's actual reason (e.g.
// generic_thermostat's "min_max_runtime" check).
func TestUpdateHelperViaOptionsFlow_ValidationErrorsSurfaceHAReason(t *testing.T) {
	t.Parallel()

	var abortedFlowID string
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{FlowID: "flow-init", Type: flowTypeForm, StepID: "init"}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(context.Context, string, map[string]any) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow-init",
				Type:   flowTypeForm,
				StepID: "init",
				Errors: map[string]string{"base": "min_max_runtime"},
			}, nil
		},
		abortConfigEntryOptionsFlowFunc: func(_ context.Context, flowID string) error {
			abortedFlowID = flowID
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "climate.my_thermostat", "config194", HelperConfig{
		Platform: "climate",
		Config:   map[string]any{},
	})

	if err == nil || !strings.Contains(err.Error(), "min_max_runtime") {
		t.Fatalf("got error %v, want it to name HA's validation reason (min_max_runtime)", err)
	}
	if abortedFlowID != "flow-init" {
		t.Errorf("aborted flow id = %q, want \"flow-init\"", abortedFlowID)
	}
}

// TestCreateHelperViaConfigFlow_StepSubmitErrorAbortsFlow is a regression
// test for the asymmetry an adversarial review found between this function
// and updateHelperViaOptionsFlow: every error return inside the multi-step
// loop used to return immediately without aborting the flow, leaving an
// orphaned flow object in Home Assistant until its own timeout reaped it.
func TestCreateHelperViaConfigFlow_StepSubmitErrorAbortsFlow(t *testing.T) {
	t.Parallel()

	var abortedFlowID string
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{FlowID: "flow123", Type: flowTypeForm, StepID: "user"}, nil
		},
		submitConfigEntryFlowStepFunc: func(context.Context, string, map[string]any) (*ConfigEntryFlowResult, error) {
			return nil, fmt.Errorf("boom")
		},
		abortConfigEntryFlowFunc: func(_ context.Context, flowID string) error {
			abortedFlowID = flowID
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: "threshold",
		Config:   map[string]any{"entity_id": "sensor.x", "lower": 5.0},
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if abortedFlowID != "flow123" {
		t.Errorf("abortedFlowID = %q, want %q - the flow must be aborted, not leaked", abortedFlowID, "flow123")
	}
}

// TestCreateHelperViaConfigFlow_ValidationErrorsAbortsFlow guards the same
// leak for the validation-errors exit path.
func TestCreateHelperViaConfigFlow_ValidationErrorsAbortsFlow(t *testing.T) {
	t.Parallel()

	var abortedFlowID string
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{FlowID: "flow456", Type: flowTypeForm, StepID: "user"}, nil
		},
		submitConfigEntryFlowStepFunc: func(context.Context, string, map[string]any) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{
				FlowID: "flow456", Type: flowTypeForm, StepID: "user",
				Errors: map[string]string{"lower": "invalid"},
			}, nil
		},
		abortConfigEntryFlowFunc: func(_ context.Context, flowID string) error {
			abortedFlowID = flowID
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: "threshold",
		Config:   map[string]any{"entity_id": "sensor.x", "lower": 5.0},
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if abortedFlowID != "flow456" {
		t.Errorf("abortedFlowID = %q, want %q - the flow must be aborted, not leaked", abortedFlowID, "flow456")
	}
}

// TestCreateHelperViaConfigFlow_MaxStepsExceededAbortsFlow guards the same
// leak for the max-steps-exceeded exit path.
func TestCreateHelperViaConfigFlow_MaxStepsExceededAbortsFlow(t *testing.T) {
	t.Parallel()

	var abortedFlowID string
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{FlowID: "flow789", Type: flowTypeForm, StepID: "user"}, nil
		},
		submitConfigEntryFlowStepFunc: func(context.Context, string, map[string]any) (*ConfigEntryFlowResult, error) {
			// Never advances past "form", forcing the maxSteps safety limit.
			return &ConfigEntryFlowResult{FlowID: "flow789", Type: flowTypeForm, StepID: "user"}, nil
		},
		abortConfigEntryFlowFunc: func(_ context.Context, flowID string) error {
			abortedFlowID = flowID
			return nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: "threshold",
		Config:   map[string]any{"entity_id": "sensor.x", "lower": 5.0},
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if abortedFlowID != "flow789" {
		t.Errorf("abortedFlowID = %q, want %q - the flow must be aborted, not leaked", abortedFlowID, "flow789")
	}
}

// TestCreateStepSubmission_PresetsStepIsEmptyWhenNoPresetFieldsSupplied
// guards issue #194: generic_thermostat's config flow has a trailing
// "presets" step whose PRESETS_SCHEMA is PREVENT_EXTRA and declares none of
// the core fields (name/heater/target_sensor/ac_mode/...) - resubmitting the
// full config there fails with "extra keys not allowed". An empty
// submission is the only valid payload for that step.
func TestCreateStepSubmission_PresetsStepIsEmptyWhenNoPresetFieldsSupplied(t *testing.T) {
	t.Parallel()

	userConfig := map[string]any{
		"name":          "my_thermostat",
		"heater":        "switch.heater",
		"target_sensor": "sensor.temp",
		"ac_mode":       false,
	}
	consumed := map[string]bool{}

	// Simulates HA's real "user" step schema for generic_thermostat.
	userStepSchema := indexStepSchema([]OptionsFlowField{
		{Name: "name"}, {Name: "heater"}, {Name: "target_sensor"}, {Name: "ac_mode"},
	})
	userStep := buildStepSubmission(flowModeCreate, userStepSchema, userConfig, consumed, "user")
	if userStep["heater"] != "switch.heater" {
		t.Errorf("user step config = %#v, want full config with heater field", userStep)
	}

	// Simulates HA's real "presets" step schema (PRESETS_SCHEMA): none of
	// its fields were supplied, and every field the user did supply was
	// already consumed by the "user" step above.
	presetsStepSchema := indexStepSchema([]OptionsFlowField{
		{Name: "away_temp"}, {Name: "eco_temp"}, {Name: "home_temp"},
		{Name: "comfort_temp"}, {Name: "sleep_temp"}, {Name: "activity_temp"},
	})
	presetsStep := buildStepSubmission(flowModeCreate, presetsStepSchema, userConfig, consumed, "presets")
	if len(presetsStep) != 0 {
		t.Errorf("presets step config = %#v, want empty map", presetsStep)
	}
}

// TestCreateHelperViaConfigFlow_GenericThermostatPresetsStepSubmitsEmptyConfig
// simulates HA's real two-step generic_thermostat CONFIG_FLOW ("user" then
// "presets") and asserts the second submission's payload is empty rather
// than the full config.
// TestCreateHelperViaConfigFlow_StatisticsDefaultsStateCharacteristicToMean
// is a regression test: HA's statistics "state_characteristic" step is
// vol.Required with no HA-side default, but callers of the HybridClient API
// directly (bypassing manage_helper's handler-level default in
// buildStatisticsConfig - integration tests call client.CreateHelper()
// directly, and any external caller of the Client interface can too) have
// always been able to omit it. The default used to live in the deleted
// buildConfigForFlowStep's statistics branch; it must still apply at this
// layer, not only in the handler.
// TestCreateHelperViaConfigFlow_UnclaimedNameIsNotReportedAsUnsupported is a
// regression test: buildHelperConfig unconditionally injects "name" into
// every config-entry helper's config map, but not every platform's flow
// schema wants it (switch_as_x has none - it derives the wrapped entity's
// name from the source entity, not a submitted field). An unclaimed "name"
// must not be reported as an unsupported field the caller explicitly
// requested - unlike a genuinely-unrecognized field, which still must be.
func TestCreateHelperViaConfigFlow_UnclaimedNameIsNotReportedAsUnsupported(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{
				FlowID: "flowsax", Type: flowTypeForm, StepID: "user",
				// HA's real switch_as_x "user" step schema has no "name" field.
				DataSchema: []OptionsFlowField{{Name: "entity_id"}, {Name: "target_domain"}},
			}, nil
		},
		submitConfigEntryFlowStepFunc: func(context.Context, string, map[string]any) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{FlowID: "flowsax", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: platformSwitchAsX,
		Config:   map[string]any{"name": "my_switch_as_x", "entity_id": "switch.x", "target_domain": "light"},
	})

	if err != nil {
		t.Errorf("unclaimed \"name\" should not fail the create: %v", err)
	}
}

// TestCreateHelperViaConfigFlow_UnrecognizedFieldStillFailsAfterCreation
// guards the other side: a genuinely-unrecognized field (not "name") that
// no step claims must still be reported, even though the entry was already
// created by the time the check runs.
func TestCreateHelperViaConfigFlow_UnrecognizedFieldStillFailsAfterCreation(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{
				FlowID: "flowunrec", Type: flowTypeForm, StepID: "user",
				DataSchema: []OptionsFlowField{{Name: "name"}, {Name: "entity_id"}},
			}, nil
		},
		submitConfigEntryFlowStepFunc: func(context.Context, string, map[string]any) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{FlowID: "flowunrec", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: "threshold",
		Config:   map[string]any{"name": "my_helper", "entity_id": "sensor.x", "bogus_field": "x"},
	})

	if err == nil || !strings.Contains(err.Error(), "bogus_field") {
		t.Errorf("got error %v, want it to name the unrecognized field bogus_field", err)
	}
}

// TestCreateHelperViaConfigFlow_SensorGroupDefaultsTypeToSum is a regression
// test: addSensorGroupDefaults (group's sensor-domain aggregation default)
// used to be invoked by the deleted transformConfigForFlow on every create.
// Its own unit test (TestAddSensorGroupDefaults below) kept it referenced
// even after that call site was removed, so `go vet`/the unused linter
// never caught that production code stopped calling it at all.
func TestCreateHelperViaConfigFlow_SensorGroupDefaultsTypeToSum(t *testing.T) {
	t.Parallel()

	var submittedData map[string]any
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{
				FlowID: "flowgroup", Type: flowTypeForm, StepID: "sensor",
				DataSchema: []OptionsFlowField{{Name: "name"}, {Name: "entities"}, {Name: "type"}},
			}, nil
		},
		submitConfigEntryFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*ConfigEntryFlowResult, error) {
			submittedData = data
			return &ConfigEntryFlowResult{FlowID: "flowgroup", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: "group",
		Config:   map[string]any{"name": "my_group", "entities": []string{"sensor.temp"}},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if submittedData["type"] != "sum" {
		t.Errorf("submitted type = %v, want \"sum\" default for a sensor-domain group", submittedData["type"])
	}
}

func TestCreateHelperViaConfigFlow_StatisticsDefaultsStateCharacteristicToMean(t *testing.T) {
	t.Parallel()

	var submittedPayloads []map[string]any
	step := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{
				FlowID: "flowstats", Type: flowTypeForm, StepID: "user",
				DataSchema: []OptionsFlowField{{Name: "name"}, {Name: "entity_id"}},
			}, nil
		},
		submitConfigEntryFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*ConfigEntryFlowResult, error) {
			submittedPayloads = append(submittedPayloads, data)
			step++
			if step == 1 {
				return &ConfigEntryFlowResult{
					FlowID: "flowstats", Type: flowTypeForm, StepID: "state_characteristic",
					DataSchema: []OptionsFlowField{{Name: "state_characteristic"}},
				}, nil
			}
			return &ConfigEntryFlowResult{FlowID: "flowstats", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: platformStatistics,
		Config:   map[string]any{"name": "my_stats", "entity_id": "sensor.x"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(submittedPayloads) != 2 {
		t.Fatalf("got %d submissions, want 2", len(submittedPayloads))
	}
	if submittedPayloads[1]["state_characteristic"] != "mean" {
		t.Errorf("state_characteristic step payload = %#v, want state_characteristic=mean when omitted", submittedPayloads[1])
	}
}

func TestCreateHelperViaConfigFlow_GenericThermostatPresetsStepSubmitsEmptyConfig(t *testing.T) {
	t.Parallel()

	var submittedPayloads []map[string]any
	step := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{FlowID: "flow194", Type: flowTypeForm, StepID: "user"}, nil
		},
		submitConfigEntryFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*ConfigEntryFlowResult, error) {
			submittedPayloads = append(submittedPayloads, data)
			step++
			if step == 1 {
				return &ConfigEntryFlowResult{FlowID: "flow194", Type: flowTypeForm, StepID: "presets"}, nil
			}
			return &ConfigEntryFlowResult{FlowID: "flow194", Type: "create_entry"}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: "generic_thermostat",
		Config: map[string]any{
			"name":          "my_thermostat",
			"heater":        "switch.heater",
			"target_sensor": "sensor.temp",
			"ac_mode":       false,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(submittedPayloads) != 2 {
		t.Fatalf("got %d submissions, want 2", len(submittedPayloads))
	}
	if len(submittedPayloads[1]) != 0 {
		t.Errorf("presets step payload = %#v, want empty map", submittedPayloads[1])
	}
}

// TestCreateHelperViaConfigFlow_PresetFieldReachesPresetsStep is issue
// #202's create-path counterpart to
// TestUpdateHelperViaOptionsFlow_PresetFieldReachesPresetsStep: unlike
// TestCreateHelperViaConfigFlow_GenericThermostatPresetsStepSubmitsEmptyConfig
// above (which uses steps with no DataSchema at all, so
// forwardEverythingUnnested's degrade path handles them), this uses a
// realistic schema for both steps - matching what HA actually returns - to
// prove buildStepSubmission's flowModeCreate branch routes a field
// declared only by a later step there instead of dropping/misrouting it,
// the same property already pinned for flowModeUpdate.
func TestCreateHelperViaConfigFlow_PresetFieldReachesPresetsStep(t *testing.T) {
	t.Parallel()

	var submittedPayloads []map[string]any
	step := 0
	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{
				FlowID: "flow202create",
				Type:   flowTypeForm,
				StepID: "user",
				DataSchema: []OptionsFlowField{
					{Name: "name"},
					{Name: "heater"},
					{Name: "target_sensor"},
					{Name: "ac_mode"},
				},
			}, nil
		},
		submitConfigEntryFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*ConfigEntryFlowResult, error) {
			submittedPayloads = append(submittedPayloads, data)
			step++
			if step == 1 {
				return &ConfigEntryFlowResult{
					FlowID:     "flow202create",
					Type:       flowTypeForm,
					StepID:     "presets",
					DataSchema: []OptionsFlowField{{Name: "away_temp"}, {Name: "eco_temp"}},
				}, nil
			}
			return &ConfigEntryFlowResult{FlowID: "flow202create", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: "generic_thermostat",
		Config: map[string]any{
			"name":          "my_thermostat",
			"heater":        "switch.heater",
			"target_sensor": "sensor.temp",
			"ac_mode":       false,
			"away_temp":     16.0,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(submittedPayloads) != 2 {
		t.Fatalf("got %d submissions, want 2", len(submittedPayloads))
	}
	if _, present := submittedPayloads[0]["away_temp"]; present {
		t.Errorf("user step payload = %#v, want away_temp NOT present - user's schema doesn't declare it", submittedPayloads[0])
	}
	presetsPayload := submittedPayloads[1]
	if presetsPayload["away_temp"] != 16.0 {
		t.Errorf("presets step payload = %#v, want away_temp = 16.0", presetsPayload)
	}
	if _, present := presetsPayload["eco_temp"]; present {
		t.Errorf("presets step payload = %#v, want eco_temp NOT present - caller never supplied it", presetsPayload)
	}
}

// TestCreateHelperViaConfigFlow_PresetsStepAbsentReportsUnaccepted pins
// W1's current, pre-remediation observable behavior: on an HA version whose
// generic_thermostat config flow has no trailing "presets" step, the flow
// completes (create_entry) before away_temp is ever claimed by any step's
// schema, and the resulting error must still name the field - the entry
// already exists at that point (see CLAUDE.md's #202 entry for why this is
// a report-only failure, not a rollback).
func TestCreateHelperViaConfigFlow_PresetsStepAbsentReportsUnaccepted(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{}
	mockREST := &mockRESTOperations{
		initConfigEntryFlowFunc: func(context.Context, string) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{
				FlowID: "flow202noPresets",
				Type:   flowTypeForm,
				StepID: "user",
				DataSchema: []OptionsFlowField{
					{Name: "name"},
					{Name: "heater"},
					{Name: "target_sensor"},
					{Name: "ac_mode"},
				},
			}, nil
		},
		submitConfigEntryFlowStepFunc: func(context.Context, string, map[string]any) (*ConfigEntryFlowResult, error) {
			return &ConfigEntryFlowResult{FlowID: "flow202noPresets", Type: flowTypeCreateEntry}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.createHelperViaConfigFlow(context.Background(), HelperConfig{
		Platform: "generic_thermostat",
		Config: map[string]any{
			"name":          "my_thermostat",
			"heater":        "switch.heater",
			"target_sensor": "sensor.temp",
			"ac_mode":       false,
			"away_temp":     16.0,
		},
	})

	if err == nil || !strings.Contains(err.Error(), "away_temp") {
		t.Fatalf("got error %v, want it to name the unaccepted field away_temp", err)
	}
	if !strings.Contains(err.Error(), "helper exists") || !strings.Contains(err.Error(), "do not retry create") {
		t.Errorf("error should warn the helper already exists and must not be recreated, got: %v", err)
	}
}

// TestUpdateHelperViaOptionsFlow_ConvertsUnsetDurationFieldOnFirstSet is a
// regression test: a duration field with no current value (e.g.
// template_binary_sensor's delay_on/delay_off, unset by default) never
// appears in currentValues, since extractOptionsFromSchema omits fields
// whose suggested_value is nil. The dict-shape heuristic alone can't detect
// such a field is duration-shaped, so setting it for the first time via
// update must still convert it via isDurationField's name list.
func TestUpdateHelperViaOptionsFlow_ConvertsUnsetDurationFieldOnFirstSet(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{}
	var submittedData map[string]any
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow123",
				Type:   "form",
				DataSchema: []OptionsFlowField{
					{Name: "state", Description: map[string]any{"suggested_value": "{{ true }}"}},
					{Name: "delay_on", Description: map[string]any{"suggested_value": nil}},
				},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*OptionsFlowResult, error) {
			submittedData = data
			return &OptionsFlowResult{Type: "create_entry"}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "binary_sensor.my_template", "config123", HelperConfig{
		Config: map[string]any{"delay_on": "00:00:05"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]int{"hours": 0, "minutes": 0, "seconds": 5}
	got, ok := submittedData["delay_on"].(map[string]int)
	if !ok {
		t.Fatalf("submitted delay_on = %#v (%T), want a duration dict", submittedData["delay_on"], submittedData["delay_on"])
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("submitted delay_on = %v, want %v", got, want)
	}
}

// TestUpdateHelperViaOptionsFlow_ConvertsWindowSizeOnFirstSetForDurationFilterType
// is a regression test: window_size is deliberately excluded from
// isDurationField (it's a duration only for two of filter's seven
// subtypes, and isDurationField is keyed on field name alone - see its doc
// comment). That's correct for the dict-shape heuristic, but it also
// disabled the "no current value yet" fallback specifically for
// window_size: a time_simple_moving_average/time_throttle filter whose
// window_size was never previously set would have a first-time update's
// raw value forwarded unconverted. The filter's step_id - the actual
// filter subtype, immutable after creation - is what should decide this,
// not the field-name list.
func TestUpdateHelperViaOptionsFlow_ConvertsWindowSizeOnFirstSetForDurationFilterType(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{}
	var submittedData map[string]any
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow999",
				Type:   "form",
				StepID: "time_simple_moving_average",
				DataSchema: []OptionsFlowField{
					{Name: "entity_id", Description: map[string]any{"suggested_value": "sensor.x"}},
					{Name: "window_size", Description: map[string]any{"suggested_value": nil}},
				},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*OptionsFlowResult, error) {
			submittedData = data
			return &OptionsFlowResult{Type: "create_entry"}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "sensor.my_filter", "config999", HelperConfig{
		Config: map[string]any{"window_size": "00:00:30"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]int{"hours": 0, "minutes": 0, "seconds": 30}
	got, ok := submittedData["window_size"].(map[string]int)
	if !ok {
		t.Fatalf("submitted window_size = %#v (%T), want a duration dict", submittedData["window_size"], submittedData["window_size"])
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("submitted window_size = %v, want %v", got, want)
	}
}

// TestUpdateHelperViaOptionsFlow_LeavesWindowSizeAloneForNonDurationFilterType
// guards the other side of the same fix: a filter subtype whose window_size
// is a plain sample-count number (e.g. outlier) must NOT be run through
// duration conversion just because the field name matches.
func TestUpdateHelperViaOptionsFlow_LeavesWindowSizeAloneForNonDurationFilterType(t *testing.T) {
	t.Parallel()

	mockWS := &mockWSOperations{}
	var submittedData map[string]any
	mockREST := &mockRESTOperations{
		initConfigEntryOptionsFlowFunc: func(context.Context, string) (*OptionsFlowResult, error) {
			return &OptionsFlowResult{
				FlowID: "flow998",
				Type:   "form",
				StepID: "outlier",
				DataSchema: []OptionsFlowField{
					{Name: "entity_id", Description: map[string]any{"suggested_value": "sensor.x"}},
					{Name: "window_size", Description: map[string]any{"suggested_value": nil}},
				},
			}, nil
		},
		submitConfigEntryOptionsFlowStepFunc: func(_ context.Context, _ string, data map[string]any) (*OptionsFlowResult, error) {
			submittedData = data
			return &OptionsFlowResult{Type: "create_entry"}, nil
		},
	}

	client := NewHybridClientWithInterfaces(mockWS, mockREST)
	err := client.updateHelperViaOptionsFlow(context.Background(), "sensor.my_filter", "config998", HelperConfig{
		Config: map[string]any{"window_size": 4.0},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := submittedData["window_size"]; got != 4.0 {
		t.Errorf("submitted window_size = %v (%T), want 4.0 untouched", got, got)
	}
}

func TestHybridClient_DeleteConfigEntry(t *testing.T) {
	t.Parallel()

	var gotEntryID string
	rest := &mockRESTOperations{
		deleteConfigEntryFunc: func(_ context.Context, entryID string) (bool, error) {
			gotEntryID = entryID
			return true, nil
		},
	}
	ws := &mockWSOperations{}
	client := NewHybridClientWithInterfaces(ws, rest)

	requireRestart, err := client.DeleteConfigEntry(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("DeleteConfigEntry failed: %v", err)
	}
	if gotEntryID != "abc123" {
		t.Errorf("expected entry_id 'abc123' forwarded to REST client, got %q", gotEntryID)
	}
	if !requireRestart {
		t.Error("expected requireRestart to be forwarded from REST client, got false")
	}
}
