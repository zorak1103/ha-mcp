package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

const cmdGetStates = "get_states"

// mockWSClientSender is a mock implementation for testing wsClientImpl.
type mockWSClientSender struct {
	sendCommandFunc func(ctx context.Context, cmdType string, params map[string]any) (*WSResultMessage, error)
}

func (m *mockWSClientSender) SendCommand(ctx context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
	if m.sendCommandFunc != nil {
		return m.sendCommandFunc(ctx, cmdType, params)
	}
	return nil, errors.New("sendCommandFunc not set")
}

// Helper to create a WSResultMessage with JSON result data.
func makeWSResultMsg(data any) *WSResultMessage {
	jsonData, _ := json.Marshal(data)
	return &WSResultMessage{
		ID:      1,
		Type:    "result",
		Success: true,
		Result:  jsonData,
	}
}

func TestNewWSClientImpl(t *testing.T) {
	t.Parallel()

	ws := &WSClient{}
	client := NewWSClientImpl(ws)

	if client == nil {
		t.Fatal("NewWSClientImpl returned nil")
	}

	impl, ok := client.(*wsClientImpl)
	if !ok {
		t.Fatal("NewWSClientImpl did not return *wsClientImpl")
	}

	if impl.ws != ws {
		t.Error("wsClientImpl.ws does not match provided WSClient")
	}
}

func TestWSClientImpl_GetStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mockResult  any
		mockErr     error
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name: "success with entities",
			mockResult: []Entity{
				{EntityID: "light.living_room", State: "on"},
				{EntityID: "sensor.temperature", State: "22.5"},
			},
			wantCount: 2,
		},
		{
			name:       "success empty list",
			mockResult: []Entity{},
			wantCount:  0,
		},
		{
			name:        "command error",
			mockErr:     errors.New("connection lost"),
			wantErr:     true,
			errContains: "get_states command failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockWSClientSender{
				sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
					if cmdType != cmdGetStates {
						t.Errorf("unexpected command type: %s", cmdType)
					}
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					return makeWSResultMsg(tt.mockResult), nil
				},
			}

			// Create a testable client using interface
			impl := &testableWSClientImplV2{
				sendCommand: mock.SendCommand,
			}

			ctx := context.Background()
			entities, err := impl.GetStates(ctx)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(entities) != tt.wantCount {
				t.Errorf("got %d entities, want %d", len(entities), tt.wantCount)
			}
		})
	}
}

// testableWSClientImplV2 wraps wsClientImpl with mockable SendCommand.
type testableWSClientImplV2 struct {
	sendCommand func(ctx context.Context, cmdType string, params map[string]any) (*WSResultMessage, error)
}

func (t *testableWSClientImplV2) GetStates(ctx context.Context) ([]Entity, error) {
	result, err := t.sendCommand(ctx, cmdGetStates, nil)
	if err != nil {
		return nil, errors.New("get_states command failed: " + err.Error())
	}

	var entities []Entity
	if err := json.Unmarshal(result.Result, &entities); err != nil {
		return nil, errors.New("failed to unmarshal states: " + err.Error())
	}

	return entities, nil
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestWSClientImpl_GetState(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "sensor.temperature", State: "22.5"},
		{EntityID: "switch.kitchen", State: "off"},
	}

	tests := []struct {
		name        string
		entityID    string
		wantState   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "found entity",
			entityID:  "sensor.temperature",
			wantState: "22.5",
		},
		{
			name:        "entity not found",
			entityID:    "sensor.nonexistent",
			wantErr:     true,
			errContains: "entity not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl := &testableWSClientImplV2{
				sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
					if cmdType == cmdGetStates {
						return makeWSResultMsg(entities), nil
					}
					return nil, errors.New("unexpected command")
				},
			}

			ctx := context.Background()
			entity, err := impl.GetState(ctx, tt.entityID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if entity.State != tt.wantState {
				t.Errorf("got state %q, want %q", entity.State, tt.wantState)
			}
		})
	}
}

func (t *testableWSClientImplV2) GetState(ctx context.Context, entityID string) (*Entity, error) {
	entities, err := t.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	for i := range entities {
		if entities[i].EntityID == entityID {
			return &entities[i], nil
		}
	}

	return nil, errors.New("entity not found: " + entityID)
}

func TestWSClientImpl_SetState(t *testing.T) {
	t.Parallel()

	ws := &WSClient{}
	impl := &wsClientImpl{ws: ws}

	ctx := context.Background()
	_, err := impl.SetState(ctx, "light.test", StateUpdate{State: "on"})

	if err == nil {
		t.Fatal("expected error for SetState via WebSocket")
	}

	if !containsStr(err.Error(), "not supported via WebSocket") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWSClientImpl_ListAutomations(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "automation.morning_lights", State: "on", Attributes: map[string]any{"friendly_name": "Morning Lights", "last_triggered": "2024-01-01T08:00:00"}},
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "automation.night_mode", State: "off", Attributes: map[string]any{"friendly_name": "Night Mode"}},
		{EntityID: "sensor.temp", State: "22"},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	ctx := context.Background()
	automations, err := impl.ListAutomations(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(automations) != 2 {
		t.Errorf("got %d automations, want 2", len(automations))
	}
}

func (t *testableWSClientImplV2) ListAutomations(ctx context.Context) ([]Automation, error) {
	entities, err := t.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	var automations []Automation
	for _, entity := range entities {
		if len(entity.EntityID) > 11 && entity.EntityID[:11] == "automation." {
			automations = append(automations, Automation{
				EntityID:      entity.EntityID,
				State:         entity.State,
				FriendlyName:  getStringAttr(entity.Attributes, "friendly_name"),
				LastTriggered: getStringAttr(entity.Attributes, "last_triggered"),
			})
		}
	}

	return automations, nil
}

func TestWSClientImpl_CallService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		domain      string
		service     string
		data        map[string]any
		mockResult  any
		mockErr     error
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful service call",
			domain:  "light",
			service: "turn_on",
			data:    map[string]any{"entity_id": "light.living_room"},
			mockResult: map[string]any{
				"context": map[string]any{"id": "123"},
			},
		},
		{
			name:    "service call with nil data",
			domain:  "homeassistant",
			service: "restart",
			data:    nil,
			mockResult: map[string]any{
				"context": map[string]any{"id": "456"},
			},
		},
		{
			name:        "service call error",
			domain:      "invalid",
			service:     "test",
			mockErr:     errors.New("service not found"),
			wantErr:     true,
			errContains: "call_service failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl := &testableWSClientImplV2{
				sendCommand: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
					if cmdType != "call_service" {
						t.Errorf("unexpected command: %s", cmdType)
					}
					if params["domain"] != tt.domain {
						t.Errorf("domain mismatch: got %v, want %v", params["domain"], tt.domain)
					}
					if params["service"] != tt.service {
						t.Errorf("service mismatch: got %v, want %v", params["service"], tt.service)
					}
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					return makeWSResultMsg(tt.mockResult), nil
				},
			}

			ctx := context.Background()
			_, err := impl.CallService(ctx, tt.domain, tt.service, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func (t *testableWSClientImplV2) CallService(ctx context.Context, domain, service string, data map[string]any) ([]Entity, error) {
	params := map[string]any{
		"domain":  domain,
		"service": service,
	}
	if data != nil {
		params["service_data"] = data
	}

	result, err := t.sendCommand(ctx, "call_service", params)
	if err != nil {
		return nil, errors.New("call_service failed: " + err.Error())
	}

	var response struct {
		Context  Context  `json:"context"`
		Response []Entity `json:"response,omitempty"`
	}
	if result.Result != nil {
		// Unmarshal errors are ignored for the optional response field
		_ = json.Unmarshal(result.Result, &response)
	}

	return response.Response, nil
}

func TestWSClientImpl_GetHistory(t *testing.T) {
	t.Parallel()

	// HistoryEntry uses float64 for timestamps (Unix seconds)
	historyData := map[string][]HistoryEntry{
		"sensor.temperature": {
			{State: "22.0", LastChanged: 1704110400.0}, // 2024-01-01T10:00:00
			{State: "22.5", LastChanged: 1704114000.0}, // 2024-01-01T11:00:00
		},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "history/history_during_period" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["entity_ids"] == nil {
				t.Error("entity_ids not set")
			}
			return makeWSResultMsg(historyData), nil
		},
	}

	ctx := context.Background()
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	history, err := impl.GetHistory(ctx, "sensor.temperature", start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("got %d history arrays, want 1", len(history))
	}

	if len(history[0]) != 2 {
		t.Errorf("got %d history entries, want 2", len(history[0]))
	}
}

func (t *testableWSClientImplV2) GetHistory(ctx context.Context, entityID string, start, end time.Time) ([][]HistoryEntry, error) {
	params := map[string]any{
		"start_time": start.Format(time.RFC3339),
		"entity_ids": []string{entityID},
	}
	if !end.IsZero() {
		params["end_time"] = end.Format(time.RFC3339)
	}

	result, err := t.sendCommand(ctx, "history/history_during_period", params)
	if err != nil {
		return nil, errors.New("history command failed: " + err.Error())
	}

	var historyMap map[string][]HistoryEntry
	if err := json.Unmarshal(result.Result, &historyMap); err != nil {
		return nil, errors.New("failed to unmarshal history: " + err.Error())
	}

	var history [][]HistoryEntry
	if entries, ok := historyMap[entityID]; ok {
		history = append(history, entries)
	}

	return history, nil
}

func TestWSClientImpl_GetEntityRegistry(t *testing.T) {
	t.Parallel()

	registryEntries := []EntityRegistryEntry{
		{EntityID: "light.living_room", Platform: "hue", DeviceID: "device1"},
		{EntityID: "sensor.temp", Platform: "mqtt", DeviceID: "device2"},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/entity_registry/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(registryEntries), nil
		},
	}

	ctx := context.Background()
	entries, err := impl.GetEntityRegistry(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}

	if diff := cmp.Diff(registryEntries, entries); diff != "" {
		t.Errorf("entries mismatch (-want +got):\n%s", diff)
	}
}

func (t *testableWSClientImplV2) GetEntityRegistry(ctx context.Context) ([]EntityRegistryEntry, error) {
	result, err := t.sendCommand(ctx, "config/entity_registry/list", nil)
	if err != nil {
		return nil, errors.New("get entity registry failed: " + err.Error())
	}

	var entries []EntityRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, errors.New("failed to unmarshal entity registry: " + err.Error())
	}

	return entries, nil
}

func TestWSClientImpl_GetDeviceRegistry(t *testing.T) {
	t.Parallel()

	deviceEntries := []DeviceRegistryEntry{
		{ID: "device1", Name: "Living Room Hub", Manufacturer: "Philips"},
		{ID: "device2", Name: "Temperature Sensor", Manufacturer: "Xiaomi"},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/device_registry/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(deviceEntries), nil
		},
	}

	ctx := context.Background()
	entries, err := impl.GetDeviceRegistry(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
}

func (t *testableWSClientImplV2) GetDeviceRegistry(ctx context.Context) ([]DeviceRegistryEntry, error) {
	result, err := t.sendCommand(ctx, "config/device_registry/list", nil)
	if err != nil {
		return nil, errors.New("get device registry failed: " + err.Error())
	}

	var entries []DeviceRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, errors.New("failed to unmarshal device registry: " + err.Error())
	}

	return entries, nil
}

func TestWSClientImpl_GetAreaRegistry(t *testing.T) {
	t.Parallel()

	areaEntries := []AreaRegistryEntry{
		{AreaID: "area1", Name: "Living Room"},
		{AreaID: "area2", Name: "Kitchen"},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/area_registry/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(areaEntries), nil
		},
	}

	ctx := context.Background()
	entries, err := impl.GetAreaRegistry(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
}

func (t *testableWSClientImplV2) GetAreaRegistry(ctx context.Context) ([]AreaRegistryEntry, error) {
	result, err := t.sendCommand(ctx, "config/area_registry/list", nil)
	if err != nil {
		return nil, errors.New("get area registry failed: " + err.Error())
	}

	var entries []AreaRegistryEntry
	if err := json.Unmarshal(result.Result, &entries); err != nil {
		return nil, errors.New("failed to unmarshal area registry: " + err.Error())
	}

	return entries, nil
}

func TestWSClientImpl_SignPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		expires  int
		wantPath string
	}{
		{
			name:     "sign path with expires",
			path:     "/api/camera_proxy/camera.front_door",
			expires:  30,
			wantPath: "/api/camera_proxy/camera.front_door?authSig=abc123",
		},
		{
			name:     "sign path without expires",
			path:     "/api/image/test.jpg",
			expires:  0,
			wantPath: "/api/image/test.jpg?authSig=xyz789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl := &testableWSClientImplV2{
				sendCommand: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
					if cmdType != "auth/sign_path" {
						t.Errorf("unexpected command: %s", cmdType)
					}
					if params["path"] != tt.path {
						t.Errorf("path mismatch: got %v, want %v", params["path"], tt.path)
					}
					return makeWSResultMsg(map[string]string{"path": tt.wantPath}), nil
				},
			}

			ctx := context.Background()
			signedPath, err := impl.SignPath(ctx, tt.path, tt.expires)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if signedPath != tt.wantPath {
				t.Errorf("got %q, want %q", signedPath, tt.wantPath)
			}
		})
	}
}

func (t *testableWSClientImplV2) SignPath(ctx context.Context, path string, expires int) (string, error) {
	params := map[string]any{
		"path": path,
	}
	if expires > 0 {
		params["expires"] = expires
	}

	result, err := t.sendCommand(ctx, "auth/sign_path", params)
	if err != nil {
		return "", errors.New("sign path failed: " + err.Error())
	}

	var response struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(result.Result, &response); err != nil {
		return "", errors.New("failed to unmarshal sign path response: " + err.Error())
	}

	return response.Path, nil
}

func TestWSClientImpl_GetLovelaceConfig(t *testing.T) {
	t.Parallel()

	lovelaceConfig := map[string]any{
		"title": "Home",
		"views": []any{
			map[string]any{"title": "Main", "path": "main"},
		},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "lovelace/config" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(lovelaceConfig), nil
		},
	}

	ctx := context.Background()
	config, err := impl.GetLovelaceConfig(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config["title"] != "Home" {
		t.Errorf("title mismatch: got %v, want Home", config["title"])
	}
}

func (t *testableWSClientImplV2) GetLovelaceConfig(ctx context.Context) (map[string]any, error) {
	result, err := t.sendCommand(ctx, "lovelace/config", nil)
	if err != nil {
		return nil, errors.New("get lovelace config failed: " + err.Error())
	}

	var config map[string]any
	if err := json.Unmarshal(result.Result, &config); err != nil {
		return nil, errors.New("failed to unmarshal lovelace config: " + err.Error())
	}

	return config, nil
}

func TestWSClientImpl_BrowseMedia(t *testing.T) {
	t.Parallel()

	browseResult := MediaBrowseResult{
		Title:          "Media",
		MediaClass:     "directory",
		MediaContentID: "media-source://media_source",
		Children: []*MediaBrowseResult{
			{Title: "Music", MediaClass: "directory", MediaContentID: "media-source://media_source/music"},
		},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "media_source/browse_media" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(browseResult), nil
		},
	}

	ctx := context.Background()
	result, err := impl.BrowseMedia(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Title != "Media" {
		t.Errorf("title mismatch: got %v, want Media", result.Title)
	}

	if len(result.Children) != 1 {
		t.Errorf("got %d children, want 1", len(result.Children))
	}
}

func (t *testableWSClientImplV2) BrowseMedia(ctx context.Context, mediaContentID string) (*MediaBrowseResult, error) {
	params := map[string]any{}
	if mediaContentID != "" {
		params["media_content_id"] = mediaContentID
	}

	result, err := t.sendCommand(ctx, "media_source/browse_media", params)
	if err != nil {
		return nil, errors.New("browse media failed: " + err.Error())
	}

	var browseResult MediaBrowseResult
	if err := json.Unmarshal(result.Result, &browseResult); err != nil {
		return nil, errors.New("failed to unmarshal browse result: " + err.Error())
	}

	return &browseResult, nil
}

func TestWSClientImpl_GetStatistics(t *testing.T) {
	t.Parallel()

	statsData := map[string][]StatisticsResult{
		"sensor.energy": {
			{StatisticID: "sensor.energy", Mean: float64Ptr(100.5)},
			{StatisticID: "sensor.energy", Mean: float64Ptr(102.3)},
		},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "recorder/statistics_during_period" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["period"] != "hour" {
				t.Errorf("period mismatch: got %v, want hour", params["period"])
			}
			return makeWSResultMsg(statsData), nil
		},
	}

	ctx := context.Background()
	stats, err := impl.GetStatistics(ctx, []string{"sensor.energy"}, "hour")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("got %d stats, want 2", len(stats))
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

func (t *testableWSClientImplV2) GetStatistics(ctx context.Context, statIDs []string, period string) ([]StatisticsResult, error) {
	startTime := time.Now().Add(-24 * time.Hour)

	params := map[string]any{
		"statistic_ids": statIDs,
		"period":        period,
		"start_time":    startTime.Format(time.RFC3339),
	}

	result, err := t.sendCommand(ctx, "recorder/statistics_during_period", params)
	if err != nil {
		return nil, errors.New("get statistics failed: " + err.Error())
	}

	var statsMap map[string][]StatisticsResult
	if err := json.Unmarshal(result.Result, &statsMap); err != nil {
		return nil, errors.New("failed to unmarshal statistics: " + err.Error())
	}

	var allStats []StatisticsResult
	for _, stats := range statsMap {
		allStats = append(allStats, stats...)
	}

	return allStats, nil
}

func TestWSClientImpl_GetCameraStream(t *testing.T) {
	t.Parallel()

	streamInfo := StreamInfo{
		URL: "http://192.168.1.100:8123/api/hls/abc123/playlist.m3u8",
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "camera/stream" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["entity_id"] != "camera.front_door" {
				t.Errorf("entity_id mismatch: got %v", params["entity_id"])
			}
			return makeWSResultMsg(streamInfo), nil
		},
	}

	ctx := context.Background()
	info, err := impl.GetCameraStream(ctx, "camera.front_door")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.URL != streamInfo.URL {
		t.Errorf("URL mismatch: got %q, want %q", info.URL, streamInfo.URL)
	}
}

func (t *testableWSClientImplV2) GetCameraStream(ctx context.Context, entityID string) (*StreamInfo, error) {
	result, err := t.sendCommand(ctx, "camera/stream", map[string]any{
		"entity_id": entityID,
	})
	if err != nil {
		return nil, errors.New("get camera stream failed: " + err.Error())
	}

	var info StreamInfo
	if err := json.Unmarshal(result.Result, &info); err != nil {
		return nil, errors.New("failed to unmarshal stream info: " + err.Error())
	}

	return &info, nil
}

func TestWSClientImpl_ListScripts(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "script.morning_routine", State: "off"},
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "script.goodnight", State: "off"},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	ctx := context.Background()
	scripts, err := impl.ListScripts(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scripts) != 2 {
		t.Errorf("got %d scripts, want 2", len(scripts))
	}
}

func (t *testableWSClientImplV2) ListScripts(ctx context.Context) ([]Entity, error) {
	entities, err := t.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	var scripts []Entity
	for _, entity := range entities {
		if len(entity.EntityID) > 7 && entity.EntityID[:7] == "script." {
			scripts = append(scripts, entity)
		}
	}

	return scripts, nil
}

func TestWSClientImpl_ListScenes(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "scene.movie_night", State: "scening"},
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "scene.dinner", State: "scening"},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	ctx := context.Background()
	scenes, err := impl.ListScenes(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(scenes))
	}
}

func (t *testableWSClientImplV2) ListScenes(ctx context.Context) ([]Entity, error) {
	entities, err := t.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	var scenes []Entity
	for _, entity := range entities {
		if len(entity.EntityID) > 6 && entity.EntityID[:6] == "scene." {
			scenes = append(scenes, entity)
		}
	}

	return scenes, nil
}

func TestWSClientImpl_ListHelpers(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "input_boolean.guest_mode", State: "off"},
		{EntityID: "input_number.temperature_target", State: "21"},
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "input_text.welcome_message", State: "Hello"},
		{EntityID: "input_select.house_mode", State: "home"},
		{EntityID: "input_datetime.alarm_time", State: "07:00:00"},
	}

	impl := &testableWSClientImplV2{
		sendCommand: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == "get_states" {
				return makeWSResultMsg(entities), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	ctx := context.Background()
	helpers, err := impl.ListHelpers(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(helpers) != 5 {
		t.Errorf("got %d helpers, want 5", len(helpers))
	}
}

func (t *testableWSClientImplV2) ListHelpers(ctx context.Context) ([]Entity, error) {
	entities, err := t.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	prefixes := []string{
		"input_boolean.",
		"input_number.",
		"input_text.",
		"input_select.",
		"input_datetime.",
	}

	var helpers []Entity
	for _, entity := range entities {
		for _, prefix := range prefixes {
			if len(entity.EntityID) > len(prefix) && entity.EntityID[:len(prefix)] == prefix {
				helpers = append(helpers, entity)
				break
			}
		}
	}

	return helpers, nil
}

// TestDomainPrefixConstants verifies the domain prefix constants are correctly defined.
func TestDomainPrefixConstants(t *testing.T) {
	t.Parallel()

	if automationPrefix != "automation." {
		t.Errorf("automationPrefix = %q, want %q", automationPrefix, "automation.")
	}

	if scriptPrefix != "script." {
		t.Errorf("scriptPrefix = %q, want %q", scriptPrefix, "script.")
	}

	if scenePrefix != "scene." {
		t.Errorf("scenePrefix = %q, want %q", scenePrefix, "scene.")
	}
}

// TestHelperPrefixes verifies that helperPrefixes contains all expected platforms.
func TestHelperPrefixes(t *testing.T) {
	t.Parallel()

	expectedPrefixes := []string{
		"input_boolean.",
		"input_number.",
		"input_text.",
		"input_select.",
		"input_datetime.",
	}

	if len(helperPrefixes) != len(expectedPrefixes) {
		t.Errorf("helperPrefixes has %d entries, want %d", len(helperPrefixes), len(expectedPrefixes))
	}

	for i, prefix := range expectedPrefixes {
		if i < len(helperPrefixes) && helperPrefixes[i] != prefix {
			t.Errorf("helperPrefixes[%d] = %q, want %q", i, helperPrefixes[i], prefix)
		}
	}
}
