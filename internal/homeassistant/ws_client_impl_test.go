package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

			impl := newWSClientImplWithSender(mock)

			ctx := context.Background()
			entities, err := impl.GetStates(ctx)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
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

			impl := newWSClientImplWithSender(&mockWSClientSender{
				sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
					if cmdType == cmdGetStates {
						return makeWSResultMsg(entities), nil
					}
					return nil, errors.New("unexpected command")
				},
			})

			ctx := context.Background()
			entity, err := impl.GetState(ctx, tt.entityID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
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

func TestWSClientImpl_SetState(t *testing.T) {
	t.Parallel()

	ws := &WSClient{}
	impl := &wsClientImpl{ws: ws}

	ctx := context.Background()
	_, err := impl.SetState(ctx, "light.test", StateUpdate{State: "on"})

	if err == nil {
		t.Fatal("expected error for SetState via WebSocket")
	}

	if !strings.Contains(err.Error(), "not supported via WebSocket") {
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

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			return nil, errors.New("unexpected command")
		},
	})

	ctx := context.Background()
	automations, err := impl.ListAutomations(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(automations) != 2 {
		t.Errorf("got %d automations, want 2", len(automations))
	}
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

			impl := newWSClientImplWithSender(&mockWSClientSender{
				sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
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
			})

			ctx := context.Background()
			_, err := impl.CallService(ctx, tt.domain, tt.service, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
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

func TestWSClientImpl_GetHistory(t *testing.T) {
	t.Parallel()

	// HistoryEntry uses float64 for timestamps (Unix seconds)
	historyData := map[string][]HistoryEntry{
		"sensor.temperature": {
			{State: "22.0", LastChanged: 1704110400.0}, // 2024-01-01T10:00:00
			{State: "22.5", LastChanged: 1704114000.0}, // 2024-01-01T11:00:00
		},
	}

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "history/history_during_period" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["entity_ids"] == nil {
				t.Error("entity_ids not set")
			}
			return makeWSResultMsg(historyData), nil
		},
	})

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

func TestWSClientImpl_GetEntityRegistry(t *testing.T) {
	t.Parallel()

	registryEntries := []EntityRegistryEntry{
		{EntityID: "light.living_room", Platform: "hue", DeviceID: "device1"},
		{EntityID: "sensor.temp", Platform: "mqtt", DeviceID: "device2"},
	}

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/entity_registry/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(registryEntries), nil
		},
	})

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

func TestWSClientImpl_GetDeviceRegistry(t *testing.T) {
	t.Parallel()

	deviceEntries := []DeviceRegistryEntry{
		{ID: "device1", Name: "Living Room Hub", Manufacturer: "Philips"},
		{ID: "device2", Name: "Temperature Sensor", Manufacturer: "Xiaomi"},
	}

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/device_registry/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(deviceEntries), nil
		},
	})

	ctx := context.Background()
	entries, err := impl.GetDeviceRegistry(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
}

func TestWSClientImpl_GetAreaRegistry(t *testing.T) {
	t.Parallel()

	areaEntries := []AreaRegistryEntry{
		{AreaID: "area1", Name: "Living Room"},
		{AreaID: "area2", Name: "Kitchen"},
	}

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/area_registry/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(areaEntries), nil
		},
	})

	ctx := context.Background()
	entries, err := impl.GetAreaRegistry(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
}

// TestWSClientImpl_ZonePersonCommands guards the exact WebSocket command
// strings for the zone and person collection APIs against the real
// wsClientImpl (not a test re-implementation). Home Assistant registers these
// under the bare domain prefix (zone/list, person/list, ...) via its
// collection helper - NOT under config/ like the entity/device/area
// registries. A regression to a config/-prefixed command fails against every
// HA version with unknown_command, so this runs in normal CI without a live
// Home Assistant.
func TestWSClientImpl_ZonePersonCommands(t *testing.T) {
	t.Parallel()

	const (
		zoneID   = "home"
		personID = "abc123"
	)

	tests := []struct {
		name       string
		wantCmd    string
		wantParams map[string]any // subset that must be present; nil to skip
		mockResult any
		invoke     func(ctx context.Context, c *wsClientImpl) error
	}{
		{
			name:       "GetZones",
			wantCmd:    "zone/list",
			mockResult: []ZoneRegistryEntry{{ID: zoneID, Name: "Home"}},
			invoke: func(ctx context.Context, c *wsClientImpl) error {
				_, err := c.GetZones(ctx)
				return err
			},
		},
		{
			name:       "CreateZone",
			wantCmd:    "zone/create",
			mockResult: ZoneRegistryEntry{ID: zoneID, Name: "Home"},
			invoke: func(ctx context.Context, c *wsClientImpl) error {
				_, err := c.CreateZone(ctx, ZoneConfig{Name: "Home"})
				return err
			},
		},
		{
			name:       "UpdateZone",
			wantCmd:    "zone/update",
			wantParams: map[string]any{"zone_id": zoneID},
			mockResult: ZoneRegistryEntry{ID: zoneID, Name: "Home"},
			invoke: func(ctx context.Context, c *wsClientImpl) error {
				_, err := c.UpdateZone(ctx, zoneID, ZoneConfig{Name: "Home"})
				return err
			},
		},
		{
			name:       "DeleteZone",
			wantCmd:    "zone/delete",
			wantParams: map[string]any{"zone_id": zoneID},
			invoke: func(ctx context.Context, c *wsClientImpl) error {
				return c.DeleteZone(ctx, zoneID)
			},
		},
		{
			name:    "GetPersons",
			wantCmd: "person/list",
			// Home Assistant's person/list command returns an object with
			// separate "storage" and "config" (YAML) arrays, not a bare
			// list, unlike zone/list.
			mockResult: map[string]any{
				"storage": []PersonRegistryEntry{{ID: personID, Name: "Alice"}},
				"config":  []PersonRegistryEntry{},
			},
			invoke: func(ctx context.Context, c *wsClientImpl) error {
				_, err := c.GetPersons(ctx)
				return err
			},
		},
		{
			name:       "CreatePerson",
			wantCmd:    "person/create",
			mockResult: PersonRegistryEntry{ID: personID, Name: "Alice"},
			invoke: func(ctx context.Context, c *wsClientImpl) error {
				_, err := c.CreatePerson(ctx, PersonConfig{Name: "Alice"})
				return err
			},
		},
		{
			name:       "UpdatePerson",
			wantCmd:    "person/update",
			wantParams: map[string]any{"person_id": personID},
			mockResult: PersonRegistryEntry{ID: personID, Name: "Alice"},
			invoke: func(ctx context.Context, c *wsClientImpl) error {
				_, err := c.UpdatePerson(ctx, personID, PersonConfig{Name: "Alice"})
				return err
			},
		},
		{
			name:       "DeletePerson",
			wantCmd:    "person/delete",
			wantParams: map[string]any{"person_id": personID},
			invoke: func(ctx context.Context, c *wsClientImpl) error {
				return c.DeletePerson(ctx, personID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				gotCmd    string
				gotParams map[string]any
			)
			c := newWSClientImplWithSender(&mockWSClientSender{
				sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
					gotCmd = cmdType
					gotParams = params
					return makeWSResultMsg(tt.mockResult), nil
				},
			})

			if err := tt.invoke(context.Background(), c); err != nil {
				t.Fatalf("%s returned error: %v", tt.name, err)
			}

			if gotCmd != tt.wantCmd {
				t.Errorf("%s sent command %q, want %q (a config/-prefixed value is the regression)", tt.name, gotCmd, tt.wantCmd)
			}

			for k, want := range tt.wantParams {
				got, ok := gotParams[k]
				if !ok {
					t.Errorf("%s params missing %q", tt.name, k)
					continue
				}
				if got != want {
					t.Errorf("%s params[%q] = %v, want %v", tt.name, k, got, want)
				}
			}
		})
	}
}

func TestWSClientImpl_CreateDashboard_OmitsEmptyIcon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		icon     string
		wantIcon bool
	}{
		{name: "empty icon omitted", icon: "", wantIcon: false},
		{name: "non-empty icon included", icon: "mdi:home", wantIcon: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotParams map[string]any
			c := newWSClientImplWithSender(&mockWSClientSender{
				sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
					if cmdType != "lovelace/dashboards/create" {
						t.Errorf("sent command %q, want %q", cmdType, "lovelace/dashboards/create")
					}
					gotParams = params
					return makeWSResultMsg(DashboardEntry{URLPath: "test"}), nil
				},
			})

			_, err := c.CreateDashboard(context.Background(), DashboardConfig{
				URLPath: "test",
				Title:   "Test",
				Icon:    tt.icon,
			})
			if err != nil {
				t.Fatalf("CreateDashboard returned error: %v", err)
			}

			got, ok := gotParams["icon"]
			if ok != tt.wantIcon {
				t.Errorf("params[%q] present = %v, want %v (got value %v)", "icon", ok, tt.wantIcon, got)
			}
			if tt.wantIcon && got != tt.icon {
				t.Errorf("params[%q] = %v, want %q", "icon", got, tt.icon)
			}
		})
	}
}

func TestWSClientImpl_CreateDashboard_OmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()

	t.Run("all optional fields unset", func(t *testing.T) {
		t.Parallel()

		var gotParams map[string]any
		c := newWSClientImplWithSender(&mockWSClientSender{
			sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
				if cmdType != "lovelace/dashboards/create" {
					t.Errorf("sent command %q, want %q", cmdType, "lovelace/dashboards/create")
				}
				gotParams = params
				return makeWSResultMsg(DashboardEntry{URLPath: "test"}), nil
			},
		})

		_, err := c.CreateDashboard(context.Background(), DashboardConfig{
			URLPath: "test",
			Title:   "Test",
		})
		if err != nil {
			t.Fatalf("CreateDashboard returned error: %v", err)
		}

		if gotParams["url_path"] != "test" {
			t.Errorf("params[%q] = %v, want %q", "url_path", gotParams["url_path"], "test")
		}
		if gotParams["title"] != "Test" {
			t.Errorf("params[%q] = %v, want %q", "title", gotParams["title"], "Test")
		}
		for _, key := range []string{"mode", "require_admin", "show_in_sidebar", "icon"} {
			if got, ok := gotParams[key]; ok {
				t.Errorf("params[%q] present = %v, want absent", key, got)
			}
		}
	})

	t.Run("all optional fields set", func(t *testing.T) {
		t.Parallel()

		var gotParams map[string]any
		c := newWSClientImplWithSender(&mockWSClientSender{
			sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
				gotParams = params
				return makeWSResultMsg(DashboardEntry{URLPath: "test"}), nil
			},
		})

		requireAdmin := true
		showInSidebar := false
		_, err := c.CreateDashboard(context.Background(), DashboardConfig{
			URLPath:       "test",
			Title:         "Test",
			Mode:          "storage",
			RequireAdmin:  &requireAdmin,
			ShowInSidebar: &showInSidebar,
		})
		if err != nil {
			t.Fatalf("CreateDashboard returned error: %v", err)
		}

		if gotParams["mode"] != "storage" {
			t.Errorf("params[%q] = %v, want %q", "mode", gotParams["mode"], "storage")
		}
		if gotParams["require_admin"] != true {
			t.Errorf("params[%q] = %v, want %v (dereferenced bool, not pointer)", "require_admin", gotParams["require_admin"], true)
		}
		if gotParams["show_in_sidebar"] != false {
			t.Errorf("params[%q] = %v, want %v (dereferenced bool, not pointer)", "show_in_sidebar", gotParams["show_in_sidebar"], false)
		}
	})
}

// TestWSClientImpl_GetPersons_MergesStorageAndConfig guards against a second,
// distinct bug in the person WebSocket API discovered once the command
// prefix fix let requests actually reach Home Assistant: person/list uses a
// custom collection handler that responds with {"storage": [...], "config":
// [...]} - separating storage-managed persons from YAML-configured ones -
// unlike zone/list and the other collection APIs, which return a plain
// array. Naively unmarshalling the response into []PersonRegistryEntry fails
// with a JSON type error. GetPersons must merge both sources so YAML-defined
// persons are still visible to callers.
func TestWSClientImpl_GetPersons_MergesStorageAndConfig(t *testing.T) {
	t.Parallel()

	c := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg(map[string]any{
				"storage": []PersonRegistryEntry{{ID: "storage1", Name: "Alice"}},
				"config":  []PersonRegistryEntry{{ID: "config1", Name: "Bob"}},
			}), nil
		},
	})

	entries, err := c.GetPersons(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (1 storage + 1 config): %+v", len(entries), entries)
	}

	var gotIDs []string
	for _, e := range entries {
		gotIDs = append(gotIDs, e.ID)
	}
	want := []string{"storage1", "config1"}
	if diff := cmp.Diff(want, gotIDs); diff != "" {
		t.Errorf("person IDs mismatch (-want +got):\n%s", diff)
	}
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

			impl := newWSClientImplWithSender(&mockWSClientSender{
				sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
					if cmdType != "auth/sign_path" {
						t.Errorf("unexpected command: %s", cmdType)
					}
					if params["path"] != tt.path {
						t.Errorf("path mismatch: got %v, want %v", params["path"], tt.path)
					}
					return makeWSResultMsg(map[string]string{"path": tt.wantPath}), nil
				},
			})

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

func TestWSClientImpl_GetLovelaceConfig(t *testing.T) {
	t.Parallel()

	lovelaceConfig := map[string]any{
		"title": "Home",
		"views": []any{
			map[string]any{"title": "Main", "path": "main"},
		},
	}

	tests := []struct {
		name        string
		urlPath     string
		wantURLPath bool
	}{
		{name: "default dashboard", urlPath: "", wantURLPath: false},
		{name: "named dashboard", urlPath: "lovelace-energy", wantURLPath: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotParams map[string]any
			impl := newWSClientImplWithSender(&mockWSClientSender{
				sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
					if cmdType != "lovelace/config" {
						t.Errorf("unexpected command: %s", cmdType)
					}
					gotParams = params
					return makeWSResultMsg(lovelaceConfig), nil
				},
			})

			ctx := context.Background()
			config, err := impl.GetLovelaceConfig(ctx, tt.urlPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if config["title"] != "Home" {
				t.Errorf("title mismatch: got %v, want Home", config["title"])
			}

			gotURLPath, ok := gotParams["url_path"]
			if tt.wantURLPath {
				if !ok || gotURLPath != tt.urlPath {
					t.Errorf("params[\"url_path\"] = %v, ok=%v, want %q", gotURLPath, ok, tt.urlPath)
				}
			} else if ok {
				t.Errorf("params[\"url_path\"] = %v, want key absent", gotURLPath)
			}
		})
	}
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

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "media_source/browse_media" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(browseResult), nil
		},
	})

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

func TestWSClientImpl_GetStatistics(t *testing.T) {
	t.Parallel()

	// Simulate real HA API response: statistic_id is only in map key, not in entries
	statsData := map[string][]StatisticsResult{
		"sensor.energy": {
			{Mean: float64Ptr(100.5)}, // StatisticID intentionally empty
			{Mean: float64Ptr(102.3)}, // StatisticID intentionally empty
		},
		"sensor.power": {
			{Mean: float64Ptr(50.0)}, // StatisticID intentionally empty
		},
	}

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "recorder/statistics_during_period" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["period"] != "hour" {
				t.Errorf("period mismatch: got %v, want hour", params["period"])
			}
			return makeWSResultMsg(statsData), nil
		},
	})

	ctx := context.Background()
	stats, err := impl.GetStatistics(ctx, []string{"sensor.energy", "sensor.power"}, "hour")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stats) != 3 {
		t.Errorf("got %d stats, want 3", len(stats))
	}

	// Verify that StatisticID is populated from map key
	energyCount, powerCount := 0, 0
	for _, stat := range stats {
		if stat.StatisticID == "" {
			t.Errorf("StatisticID is empty for stat with mean %v", stat.Mean)
		}
		if stat.StatisticID == "sensor.energy" {
			energyCount++
		}
		if stat.StatisticID == "sensor.power" {
			powerCount++
		}
	}

	if energyCount != 2 {
		t.Errorf("got %d energy stats, want 2", energyCount)
	}
	if powerCount != 1 {
		t.Errorf("got %d power stats, want 1", powerCount)
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

func TestWSClientImpl_GetCameraStream(t *testing.T) {
	t.Parallel()

	streamInfo := StreamInfo{
		URL: "http://192.168.1.100:8123/api/hls/abc123/playlist.m3u8",
	}

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "camera/stream" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["entity_id"] != "camera.front_door" {
				t.Errorf("entity_id mismatch: got %v", params["entity_id"])
			}
			return makeWSResultMsg(streamInfo), nil
		},
	})

	ctx := context.Background()
	info, err := impl.GetCameraStream(ctx, "camera.front_door")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.URL != streamInfo.URL {
		t.Errorf("URL mismatch: got %q, want %q", info.URL, streamInfo.URL)
	}
}

func TestWSClientImpl_ListScripts(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "script.morning_routine", State: "off"},
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "script.goodnight", State: "off"},
	}

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			return nil, errors.New("unexpected command")
		},
	})

	ctx := context.Background()
	scripts, err := impl.ListScripts(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scripts) != 2 {
		t.Errorf("got %d scripts, want 2", len(scripts))
	}
}

func TestWSClientImpl_ListScenes(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "scene.movie_night", State: "scening"},
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "scene.dinner", State: "scening"},
	}

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			return nil, errors.New("unexpected command")
		},
	})

	ctx := context.Background()
	scenes, err := impl.ListScenes(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(scenes))
	}
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

	impl := newWSClientImplWithSender(&mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			return nil, errors.New("unexpected command")
		},
	})

	ctx := context.Background()
	helpers, err := impl.ListHelpers(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(helpers) != 5 {
		t.Errorf("got %d helpers, want 5", len(helpers))
	}
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

// =============================================================================
// Tests using newWSClientImplWithSender (actual implementation testing)
// =============================================================================

func TestWSClientImplWithSender_GetStates(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "sensor.temperature", State: "22.5"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != cmdGetStates {
				t.Errorf("unexpected command type: %s", cmdType)
			}
			return makeWSResultMsg(entities), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.GetStates(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d entities, want 2", len(result))
	}
}

func TestWSClientImplWithSender_GetStates_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("connection failed")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetStates(context.Background())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "get_states command failed") {
		t.Errorf("error should contain 'get_states command failed', got: %v", err)
	}
}

func TestWSClientImplWithSender_GetStates_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{
				Success: true,
				Result:  []byte("invalid json"),
			}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetStates(context.Background())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("error should contain 'failed to unmarshal', got: %v", err)
	}
}

func TestWSClientImplWithSender_GetState(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "light.living_room", State: "on"},
		{EntityID: "sensor.temperature", State: "22.5"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg(entities), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	entity, err := client.GetState(context.Background(), "sensor.temperature")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity.State != "22.5" {
		t.Errorf("got state %q, want 22.5", entity.State)
	}
}

func TestWSClientImplWithSender_GetState_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg([]Entity{}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetState(context.Background(), "nonexistent.entity")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "entity not found") {
		t.Errorf("error should contain 'entity not found', got: %v", err)
	}
}

func TestWSClientImplWithSender_GetHistory(t *testing.T) {
	t.Parallel()

	historyData := map[string][]HistoryEntry{
		"sensor.temp": {
			{State: "22.0", LastChanged: 1704110400.0},
			{State: "22.5", LastChanged: 1704114000.0},
		},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "history/history_during_period" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["start_time"] == nil {
				t.Error("start_time not set")
			}
			return makeWSResultMsg(historyData), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	start := time.Now().Add(-24 * time.Hour)
	history, err := client.GetHistory(context.Background(), "sensor.temp", start, time.Time{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 1 || len(history[0]) != 2 {
		t.Errorf("unexpected history length")
	}
}

func TestWSClientImplWithSender_GetHistory_WithEndTime(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
			if params["end_time"] == nil {
				t.Error("end_time should be set")
			}
			return makeWSResultMsg(map[string][]HistoryEntry{}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()
	_, err := client.GetHistory(context.Background(), "sensor.temp", start, end)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_CallService(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "call_service" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["domain"] != "light" {
				t.Errorf("domain mismatch: %v", params["domain"])
			}
			if params["service"] != "turn_on" {
				t.Errorf("service mismatch: %v", params["service"])
			}
			return makeWSResultMsg(map[string]any{"context": map[string]any{"id": "123"}}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.CallService(context.Background(), "light", "turn_on", map[string]any{"entity_id": "light.test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_CallService_NilData(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
			if params["service_data"] != nil {
				t.Error("service_data should be nil")
			}
			return makeWSResultMsg(map[string]any{}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.CallService(context.Background(), "homeassistant", "restart", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_ClearSystemLog(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "call_service" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["domain"] != "system_log" {
				t.Errorf("domain mismatch: %v", params["domain"])
			}
			if params["service"] != "clear" {
				t.Errorf("service mismatch: %v", params["service"])
			}
			if params["service_data"] != nil {
				t.Error("service_data should be nil")
			}
			return makeWSResultMsg(map[string]any{"context": map[string]any{"id": "123"}}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.ClearSystemLog(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_ClearSystemLog_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("unknown_command")
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.ClearSystemLog(context.Background())

	if err == nil || !strings.Contains(err.Error(), "clear system log failed") {
		t.Errorf("expected 'clear system log failed' error, got: %v", err)
	}
}

func TestWSClientImplWithSender_CallService_NilResult(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: nil}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	entities, err := client.CallService(context.Background(), "light", "turn_on", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When result is nil, entities can also be nil (no error is expected)
	_ = entities
}

func TestWSClientImplWithSender_ListAutomations(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "automation.morning", State: "on", Attributes: map[string]any{"friendly_name": "Morning"}},
		{EntityID: "light.test", State: "off"},
		{EntityID: "automation.night", State: "off", Attributes: map[string]any{"last_triggered": "2024-01-01"}},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg(entities), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	automations, err := client.ListAutomations(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(automations) != 2 {
		t.Errorf("got %d automations, want 2", len(automations))
	}
	if automations[0].FriendlyName != "Morning" {
		t.Errorf("FriendlyName mismatch: %v", automations[0].FriendlyName)
	}
}

func TestWSClientImplWithSender_GetAutomation(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "automation/config" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["entity_id"] != "automation.test" {
				t.Errorf("entity_id mismatch: %v", params["entity_id"])
			}
			return makeWSResultMsg(map[string]any{
				"config": map[string]any{
					"alias":       "Test Automation",
					"description": "A test",
				},
			}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	automation, err := client.GetAutomation(context.Background(), "test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if automation.EntityID != "automation.test" {
		t.Errorf("EntityID mismatch: %v", automation.EntityID)
	}
}

func TestWSClientImplWithSender_GetAutomation_WithPrefix(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
			if params["entity_id"] != "automation.test" {
				t.Errorf("entity_id should remain unchanged: %v", params["entity_id"])
			}
			return makeWSResultMsg(map[string]any{"config": map[string]any{}}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetAutomation(context.Background(), "automation.test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_ToggleAutomation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		enabled bool
		service string
	}{
		{"enable", true, "turn_on"},
		{"disable", false, "turn_off"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockWSClientSender{
				sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
					if params["service"] != tt.service {
						t.Errorf("service mismatch: got %v, want %v", params["service"], tt.service)
					}
					return makeWSResultMsg(map[string]any{}), nil
				},
			}

			client := newWSClientImplWithSender(mock)
			err := client.ToggleAutomation(context.Background(), "automation.test", tt.enabled)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWSClientImplWithSender_ListHelpers(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "input_boolean.test", State: "on"},
		{EntityID: "input_number.value", State: "50"},
		{EntityID: "input_text.msg", State: "hello"},
		{EntityID: "input_select.mode", State: "home"},
		{EntityID: "input_datetime.alarm", State: "08:00"},
		{EntityID: "light.test", State: "on"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg(entities), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	helpers, err := client.ListHelpers(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(helpers) != 5 {
		t.Errorf("got %d helpers, want 5", len(helpers))
	}
}

func TestWSClientImplWithSender_CreateHelper(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "input_boolean/create" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			// CreateHelper no longer sends platform_id for create - HA derives entity ID from name
			if _, hasID := params["input_boolean_id"]; hasID {
				t.Errorf("unexpected input_boolean_id in create params")
			}
			if params["name"] != "Test Helper" {
				t.Errorf("name mismatch: %v", params["name"])
			}
			return makeWSResultMsg(nil), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.CreateHelper(context.Background(), HelperConfig{
		Platform: "input_boolean",
		Config:   map[string]any{"name": "Test Helper"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_UpdateHelper(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "input_number/update" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["input_number_id"] != "test" {
				t.Errorf("id mismatch: %v", params["input_number_id"])
			}
			return makeWSResultMsg(nil), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.UpdateHelper(context.Background(), "test", HelperConfig{
		Platform: "input_number",
		Config:   map[string]any{"min": 0, "max": 100},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_UpdateHelper_FullEntityID(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "input_number/update" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			// Full entity_id ("input_number.test") must be stripped to the bare id
			// before being sent as "input_number_id" - HA rejects a prefixed value.
			if params["input_number_id"] != "test" {
				t.Errorf("id mismatch: %v", params["input_number_id"])
			}
			return makeWSResultMsg(nil), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.UpdateHelper(context.Background(), "input_number.test", HelperConfig{
		Platform: "input_number",
		Config:   map[string]any{"min": 0, "max": 100},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_UpdateHelper_ConfigEntryPlatformRejected(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			t.Fatalf("SendCommand should not be called for config-entry platform, got cmdType: %s", cmdType)
			return nil, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.UpdateHelper(context.Background(), "sensor.my_template", HelperConfig{
		Platform: "sensor",
		Config:   map[string]any{"state": "{{ 42 }}"},
	})

	if err == nil {
		t.Fatal("expected error for config-entry platform")
	}
	if !strings.Contains(err.Error(), "sensor") || !strings.Contains(err.Error(), "options flow") {
		t.Errorf("error should explain config-entry helpers require options flow: %v", err)
	}
}

func TestWSClientImplWithSender_DeleteHelper(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "input_boolean/delete" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["input_boolean_id"] != "test" {
				t.Errorf("id mismatch: %v", params["input_boolean_id"])
			}
			return makeWSResultMsg(nil), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.DeleteHelper(context.Background(), "input_boolean.test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_DeleteHelper_UnknownPlatform(t *testing.T) {
	t.Parallel()

	client := newWSClientImplWithSender(&mockWSClientSender{})
	err := client.DeleteHelper(context.Background(), "unknown.test")

	if err == nil {
		t.Fatal("expected error for unknown platform")
	}
	if !strings.Contains(err.Error(), "unable to determine platform") {
		t.Errorf("error should mention platform: %v", err)
	}
}

func TestWSClientImplWithSender_DeleteHelper_ConfigEntryPlatformRejected(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			t.Fatalf("SendCommand should not be called for config-entry platform, got cmdType: %s", cmdType)
			return nil, nil
		},
	}

	// "group" is a Config Entry platform (see configEntryPlatforms) even though
	// extractPlatform recognizes its entity_id prefix - there is no group/delete
	// WS command, so this must be rejected rather than sent as unknown_command.
	client := newWSClientImplWithSender(mock)
	err := client.DeleteHelper(context.Background(), "group.my_group")

	if err == nil {
		t.Fatal("expected error for config-entry platform")
	}
	if !strings.Contains(err.Error(), "group") {
		t.Errorf("error should mention platform: %v", err)
	}
}

func TestWSClientImplWithSender_SetHelperValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		entityID string
		value    any
		service  string
	}{
		{"input_boolean_on", "input_boolean.test", true, "turn_on"},
		{"input_boolean_off", "input_boolean.test", false, "turn_off"},
		{"input_number", "input_number.test", 50.0, "set_value"},
		{"input_text", "input_text.test", "hello", "set_value"},
		{"input_select", "input_select.test", "option1", "select_option"},
		{"input_datetime_string", "input_datetime.test", "2024-01-01 08:00:00", "set_datetime"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockWSClientSender{
				sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
					if params["service"] != tt.service {
						t.Errorf("service mismatch: got %v, want %v", params["service"], tt.service)
					}
					return makeWSResultMsg(map[string]any{}), nil
				},
			}

			client := newWSClientImplWithSender(mock)
			err := client.SetHelperValue(context.Background(), tt.entityID, tt.value)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWSClientImplWithSender_SetHelperValue_DatetimeMap(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
			serviceData := params["service_data"].(map[string]any)
			if serviceData["time"] != "08:00" {
				t.Errorf("time mismatch: %v", serviceData["time"])
			}
			return makeWSResultMsg(map[string]any{}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.SetHelperValue(context.Background(), "input_datetime.test", map[string]any{"time": "08:00"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_SetHelperValue_InvalidBoolean(t *testing.T) {
	t.Parallel()

	client := newWSClientImplWithSender(&mockWSClientSender{})
	err := client.SetHelperValue(context.Background(), "input_boolean.test", "not_a_bool")

	if err == nil {
		t.Fatal("expected error for invalid boolean value")
	}
	if !strings.Contains(err.Error(), "requires a boolean value") {
		t.Errorf("error should mention boolean: %v", err)
	}
}

func TestWSClientImplWithSender_SetHelperValue_UnknownPlatform(t *testing.T) {
	t.Parallel()

	client := newWSClientImplWithSender(&mockWSClientSender{})
	err := client.SetHelperValue(context.Background(), "unknown.test", "value")

	if err == nil {
		t.Fatal("expected error for unknown platform")
	}
}

func TestWSClientImplWithSender_ListScripts(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "script.morning", State: "off"},
		{EntityID: "light.test", State: "on"},
		{EntityID: "script.night", State: "off"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg(entities), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	scripts, err := client.ListScripts(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scripts) != 2 {
		t.Errorf("got %d scripts, want 2", len(scripts))
	}
}

func TestWSClientImplWithSender_GetScript(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "script.test", State: "off", Attributes: map[string]any{"friendly_name": "Test Script"}},
	}

	callCount := 0
	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			callCount++
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			if cmdType == "script/config" {
				if params["entity_id"] != "script.test" {
					t.Errorf("entity_id mismatch: %v", params["entity_id"])
				}
				return makeWSResultMsg(map[string]any{
					"config": map[string]any{
						"alias": "Test Script",
						"mode":  "single",
					},
				}), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	client := newWSClientImplWithSender(mock)
	script, err := client.GetScript(context.Background(), "test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if script.EntityID != "script.test" {
		t.Errorf("EntityID mismatch: %v", script.EntityID)
	}
	if script.FriendlyName != "Test Script" {
		t.Errorf("FriendlyName mismatch: %v", script.FriendlyName)
	}
}

func TestWSClientImplWithSender_ListScenes(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "scene.movie", State: "scening"},
		{EntityID: "light.test", State: "on"},
		{EntityID: "scene.dinner", State: "scening"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg(entities), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	scenes, err := client.ListScenes(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(scenes))
	}
}

func TestWSClientImplWithSender_GetScheduleConfig(t *testing.T) {
	t.Parallel()

	schedules := []map[string]any{
		{"id": "test", "name": "Test Schedule", "monday": []map[string]any{{"from": "08:00", "to": "09:00"}}},
		{"id": "other", "name": "Other"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "schedule/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(schedules), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	config, err := client.GetScheduleConfig(context.Background(), "schedule.test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config["name"] != "Test Schedule" {
		t.Errorf("name mismatch: %v", config["name"])
	}
}

func TestWSClientImplWithSender_GetScheduleConfig_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg([]map[string]any{}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetScheduleConfig(context.Background(), "nonexistent")

	if err == nil {
		t.Fatal("expected error for not found schedule")
	}
	if !strings.Contains(err.Error(), "schedule not found") {
		t.Errorf("error should mention not found: %v", err)
	}
}

func TestWSClientImplWithSender_GetEntityRegistry(t *testing.T) {
	t.Parallel()

	entries := []EntityRegistryEntry{
		{EntityID: "light.test", Platform: "hue"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/entity_registry/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(entries), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.GetEntityRegistry(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d entries, want 1", len(result))
	}
}

func TestWSClientImplWithSender_GetDeviceRegistry(t *testing.T) {
	t.Parallel()

	entries := []DeviceRegistryEntry{
		{ID: "device1", Name: "Test Device"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/device_registry/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(entries), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.GetDeviceRegistry(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d entries, want 1", len(result))
	}
}

func TestWSClientImplWithSender_GetAreaRegistry(t *testing.T) {
	t.Parallel()

	entries := []AreaRegistryEntry{
		{AreaID: "area1", Name: "Living Room"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/area_registry/list" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(entries), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.GetAreaRegistry(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d entries, want 1", len(result))
	}
}

func TestWSClientImplWithSender_SignPath(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "auth/sign_path" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["path"] != "/api/test" {
				t.Errorf("path mismatch: %v", params["path"])
			}
			if params["expires"] != 30 {
				t.Errorf("expires mismatch: %v", params["expires"])
			}
			return makeWSResultMsg(map[string]string{"path": "/api/test?sig=abc"}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	path, err := client.SignPath(context.Background(), "/api/test", 30)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/api/test?sig=abc" {
		t.Errorf("path mismatch: %v", path)
	}
}

func TestWSClientImplWithSender_SignPath_NoExpires(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
			if params["expires"] != nil {
				t.Error("expires should not be set")
			}
			return makeWSResultMsg(map[string]string{"path": "/api/test"}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.SignPath(context.Background(), "/api/test", 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_GetCameraStream(t *testing.T) {
	t.Parallel()

	streamInfo := StreamInfo{URL: "http://test/stream.m3u8"}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "camera/stream" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["entity_id"] != "camera.test" {
				t.Errorf("entity_id mismatch: %v", params["entity_id"])
			}
			return makeWSResultMsg(streamInfo), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	info, err := client.GetCameraStream(context.Background(), "camera.test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.URL != streamInfo.URL {
		t.Errorf("URL mismatch: %v", info.URL)
	}
}

func TestWSClientImplWithSender_BrowseMedia(t *testing.T) {
	t.Parallel()

	browseResult := MediaBrowseResult{Title: "Media", MediaClass: "directory"}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "media_source/browse_media" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["media_content_id"] != "media-source://test" {
				t.Errorf("media_content_id mismatch: %v", params["media_content_id"])
			}
			return makeWSResultMsg(browseResult), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.BrowseMedia(context.Background(), "media-source://test")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Media" {
		t.Errorf("Title mismatch: %v", result.Title)
	}
}

func TestWSClientImplWithSender_BrowseMedia_EmptyID(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
			if params["media_content_id"] != nil {
				t.Error("media_content_id should not be set for empty ID")
			}
			return makeWSResultMsg(MediaBrowseResult{}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.BrowseMedia(context.Background(), "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_GetLovelaceConfig(t *testing.T) {
	t.Parallel()

	config := map[string]any{"title": "Home", "views": []any{}}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "lovelace/config" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(config), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.GetLovelaceConfig(context.Background(), "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["title"] != "Home" {
		t.Errorf("title mismatch: %v", result["title"])
	}
}

func TestWSClientImplWithSender_GetStatistics(t *testing.T) {
	t.Parallel()

	// Simulate real HA API response: statistic_id is only in map key, not in entries
	statsData := map[string][]StatisticsResult{
		"sensor.energy": {{Mean: float64Ptr(100.5)}}, // StatisticID intentionally empty
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "recorder/statistics_during_period" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["period"] != "hour" {
				t.Errorf("period mismatch: %v", params["period"])
			}
			return makeWSResultMsg(statsData), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	stats, err := client.GetStatistics(context.Background(), []string{"sensor.energy"}, "hour")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 1 {
		t.Errorf("got %d stats, want 1", len(stats))
	}
	// Verify StatisticID was populated from map key
	if stats[0].StatisticID != "sensor.energy" {
		t.Errorf("got StatisticID %q, want sensor.energy", stats[0].StatisticID)
	}
}

func TestWSClientImplWithSender_GetTriggersForTarget(t *testing.T) {
	t.Parallel()

	triggers := []string{"trigger1", "trigger2"}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "get_triggers_for_target" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["target"] == nil {
				t.Error("target should be set")
			}
			return makeWSResultMsg(triggers), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.GetTriggersForTarget(context.Background(), Target{EntityID: []string{"light.test"}}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d triggers, want 2", len(result))
	}
}

func TestWSClientImplWithSender_GetConditionsForTarget(t *testing.T) {
	t.Parallel()

	conditions := []string{"condition1"}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "get_conditions_for_target" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(conditions), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.GetConditionsForTarget(context.Background(), Target{}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d conditions, want 1", len(result))
	}
}

func TestWSClientImplWithSender_GetServicesForTarget(t *testing.T) {
	t.Parallel()

	services := []string{"light.turn_on", "light.turn_off"}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType != "get_services_for_target" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			return makeWSResultMsg(services), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.GetServicesForTarget(context.Background(), Target{}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d services, want 2", len(result))
	}
}

func TestWSClientImplWithSender_ExtractFromTarget(t *testing.T) {
	t.Parallel()

	extractResult := ExtractFromTargetResult{
		ReferencedEntities: []string{"light.test"},
		ReferencedDevices:  []string{"device1"},
	}

	expandGroup := true
	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "extract_from_target" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["expand_group"] != true {
				t.Errorf("expand_group mismatch: %v", params["expand_group"])
			}
			return makeWSResultMsg(extractResult), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.ExtractFromTarget(context.Background(), Target{EntityID: []string{"light.test"}}, &expandGroup)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ReferencedEntities) != 1 {
		t.Errorf("got %d entities, want 1", len(result.ReferencedEntities))
	}
}

func TestNewWSClientImplWithSender(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{}
	client := newWSClientImplWithSender(mock)

	if client == nil {
		t.Fatal("newWSClientImplWithSender returned nil")
	}
}

// =============================================================================
// Additional Error Path Tests
// =============================================================================

func TestWSClientImplWithSender_GetHistory_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("history failed")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetHistory(context.Background(), "sensor.temp", time.Now(), time.Time{})

	if err == nil || !strings.Contains(err.Error(), "history command failed") {
		t.Errorf("expected history command failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetHistory_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid json")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetHistory(context.Background(), "sensor.temp", time.Now(), time.Time{})

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_CallService_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("service error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.CallService(context.Background(), "light", "turn_on", nil)

	if err == nil || !strings.Contains(err.Error(), "call_service failed") {
		t.Errorf("expected call_service failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_ListAutomations_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("get states error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.ListAutomations(context.Background())

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestWSClientImplWithSender_GetAutomation_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("automation error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetAutomation(context.Background(), "test")

	if err == nil || !strings.Contains(err.Error(), "get automation failed") {
		t.Errorf("expected get automation failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetAutomation_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetAutomation(context.Background(), "test")

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_ListHelpers_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("get states error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.ListHelpers(context.Background())

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestWSClientImplWithSender_CreateHelper_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("create failed")
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.CreateHelper(context.Background(), HelperConfig{Platform: "input_boolean"})

	if err == nil || !strings.Contains(err.Error(), "create helper failed") {
		t.Errorf("expected create helper failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_UpdateHelper_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("update failed")
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.UpdateHelper(context.Background(), "test", HelperConfig{Platform: "input_boolean"})

	if err == nil || !strings.Contains(err.Error(), "update helper failed") {
		t.Errorf("expected update helper failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_DeleteHelper_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("delete failed")
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.DeleteHelper(context.Background(), "input_boolean.test")

	if err == nil || !strings.Contains(err.Error(), "delete helper failed") {
		t.Errorf("expected delete helper failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_ListScripts_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("get states error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.ListScripts(context.Background())

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestWSClientImplWithSender_GetScript_StateError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return nil, errors.New("get states error")
			}
			return nil, errors.New("unexpected call")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetScript(context.Background(), "test")

	if err == nil || !strings.Contains(err.Error(), "get script state failed") {
		t.Errorf("expected get script state failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetScript_ConfigError(t *testing.T) {
	t.Parallel()

	entities := []Entity{{EntityID: "script.test", State: "off"}}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			if cmdType == "script/config" {
				return nil, errors.New("config error")
			}
			return nil, errors.New("unexpected call")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetScript(context.Background(), "test")

	if err == nil || !strings.Contains(err.Error(), "get script config failed") {
		t.Errorf("expected get script config failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetScript_UnmarshalError(t *testing.T) {
	t.Parallel()

	entities := []Entity{{EntityID: "script.test", State: "off"}}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, _ map[string]any) (*WSResultMessage, error) {
			if cmdType == cmdGetStates {
				return makeWSResultMsg(entities), nil
			}
			if cmdType == "script/config" {
				return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
			}
			return nil, errors.New("unexpected call")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetScript(context.Background(), "script.test")

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_ListScenes_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("get states error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.ListScenes(context.Background())

	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestWSClientImplWithSender_GetScheduleConfig_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("list failed")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetScheduleConfig(context.Background(), "test")

	if err == nil || !strings.Contains(err.Error(), "get schedule list failed") {
		t.Errorf("expected get schedule list failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetScheduleConfig_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetScheduleConfig(context.Background(), "test")

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetEntityRegistry_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("registry error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetEntityRegistry(context.Background())

	if err == nil || !strings.Contains(err.Error(), "get entity registry failed") {
		t.Errorf("expected get entity registry failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetEntityRegistry_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetEntityRegistry(context.Background())

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetDeviceRegistry_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("registry error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetDeviceRegistry(context.Background())

	if err == nil || !strings.Contains(err.Error(), "get device registry failed") {
		t.Errorf("expected get device registry failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetDeviceRegistry_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetDeviceRegistry(context.Background())

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetAreaRegistry_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("registry error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetAreaRegistry(context.Background())

	if err == nil || !strings.Contains(err.Error(), "get area registry failed") {
		t.Errorf("expected get area registry failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetAreaRegistry_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetAreaRegistry(context.Background())

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_SignPath_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("sign error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.SignPath(context.Background(), "/api/test", 30)

	if err == nil || !strings.Contains(err.Error(), "sign path failed") {
		t.Errorf("expected sign path failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_SignPath_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.SignPath(context.Background(), "/api/test", 30)

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetCameraStream_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("stream error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetCameraStream(context.Background(), "camera.test")

	if err == nil || !strings.Contains(err.Error(), "get camera stream failed") {
		t.Errorf("expected get camera stream failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetCameraStream_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetCameraStream(context.Background(), "camera.test")

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_BrowseMedia_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("browse error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.BrowseMedia(context.Background(), "")

	if err == nil || !strings.Contains(err.Error(), "browse media failed") {
		t.Errorf("expected browse media failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_BrowseMedia_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.BrowseMedia(context.Background(), "")

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetLovelaceConfig_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("lovelace error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetLovelaceConfig(context.Background(), "")

	if err == nil || !strings.Contains(err.Error(), "get lovelace config failed") {
		t.Errorf("expected get lovelace config failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetLovelaceConfig_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetLovelaceConfig(context.Background(), "")

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetStatistics_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("statistics error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetStatistics(context.Background(), []string{"sensor.test"}, "hour")

	if err == nil || !strings.Contains(err.Error(), "get statistics failed") {
		t.Errorf("expected get statistics failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetStatistics_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetStatistics(context.Background(), []string{"sensor.test"}, "hour")

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetTriggersForTarget_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("target error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetTriggersForTarget(context.Background(), Target{}, nil)

	if err == nil || !strings.Contains(err.Error(), "get_triggers_for_target failed") {
		t.Errorf("expected get_triggers_for_target failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetTriggersForTarget_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetTriggersForTarget(context.Background(), Target{}, nil)

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_ExtractFromTarget_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("extract error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.ExtractFromTarget(context.Background(), Target{}, nil)

	if err == nil || !strings.Contains(err.Error(), "extract_from_target failed") {
		t.Errorf("expected extract_from_target failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_ExtractFromTarget_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.ExtractFromTarget(context.Background(), Target{}, nil)

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetState_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("get states error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetState(context.Background(), "light.test")

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// =============================================================================
// Full Config Branch Coverage Tests
// =============================================================================

func TestWSClientImplWithSender_GetHistory_BothStartAndEnd(t *testing.T) {
	t.Parallel()

	historyData := map[string][]HistoryEntry{
		"sensor.temp": {{State: "22.5"}, {State: "23.0"}},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "history/history_during_period" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["end_time"] == nil {
				t.Error("end_time should be set when provided")
			}
			return makeWSResultMsg(historyData), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()
	history, err := client.GetHistory(context.Background(), "sensor.temp", start, end)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 1 || len(history[0]) != 2 {
		t.Errorf("expected 1 entity with 2 entries, got %d entities", len(history))
	}
}

func TestWSClientImplWithSender_CallService_WithData(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "call_service" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["service_data"] == nil {
				t.Error("service_data should be set when data is provided")
			}
			serviceData := params["service_data"].(map[string]any)
			if serviceData["entity_id"] != "light.test" {
				t.Errorf("entity_id mismatch: %v", serviceData["entity_id"])
			}
			return makeWSResultMsg(map[string]any{
				"context":  Context{ID: "123"},
				"response": []Entity{{EntityID: "light.test", State: "on"}},
			}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	entities, err := client.CallService(context.Background(), "light", "turn_on", map[string]any{
		"entity_id":  "light.test",
		"brightness": 255,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(entities))
	}
}

func TestWSClientImplWithSender_CreateHelper_NoID(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "input_text/create" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			// ID should not be present when not provided
			if _, hasID := params["input_text_id"]; hasID {
				t.Error("input_text_id should not be set when not provided")
			}
			if params["name"] != "Test Text" {
				t.Errorf("name mismatch: %v", params["name"])
			}
			return makeWSResultMsg(nil), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	err := client.CreateHelper(context.Background(), HelperConfig{
		Platform: "input_text",
		Config:   map[string]any{"name": "Test Text"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_GetTriggersForTarget_WithExpandGroup(t *testing.T) {
	t.Parallel()

	triggers := []string{"state_trigger", "time_trigger"}
	expandGroup := false

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "get_triggers_for_target" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["expand_group"] != false {
				t.Errorf("expand_group should be false: %v", params["expand_group"])
			}
			return makeWSResultMsg(triggers), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	result, err := client.GetTriggersForTarget(context.Background(), Target{EntityID: []string{"light.test"}}, &expandGroup)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 triggers, got %d", len(result))
	}
}

func TestWSClientImplWithSender_GetConditionsForTarget_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("conditions error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetConditionsForTarget(context.Background(), Target{}, nil)

	if err == nil || !strings.Contains(err.Error(), "get_conditions_for_target failed") {
		t.Errorf("expected get_conditions_for_target failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetConditionsForTarget_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetConditionsForTarget(context.Background(), Target{}, nil)

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetServicesForTarget_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("services error")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetServicesForTarget(context.Background(), Target{}, nil)

	if err == nil || !strings.Contains(err.Error(), "get_services_for_target failed") {
		t.Errorf("expected get_services_for_target failed error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetServicesForTarget_UnmarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return &WSResultMessage{Success: true, Result: []byte("invalid")}, nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetServicesForTarget(context.Background(), Target{}, nil)

	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestWSClientImplWithSender_SetHelperValue_UnsupportedPlatform(t *testing.T) {
	t.Parallel()

	client := newWSClientImplWithSender(&mockWSClientSender{})
	err := client.SetHelperValue(context.Background(), "schedule.test", "value")

	if err == nil || !strings.Contains(err.Error(), "unsupported helper platform") {
		t.Errorf("expected unsupported helper platform error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetScheduleConfig_NoMatchingID(t *testing.T) {
	t.Parallel()

	schedules := []map[string]any{
		{"id": "other", "name": "Other Schedule"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg(schedules), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetScheduleConfig(context.Background(), "schedule.nonexistent")

	if err == nil || !strings.Contains(err.Error(), "schedule not found") {
		t.Errorf("expected schedule not found error, got: %v", err)
	}
}

func TestWSClientImplWithSender_SetState(t *testing.T) {
	t.Parallel()

	client := newWSClientImplWithSender(&mockWSClientSender{})
	_, err := client.SetState(context.Background(), "light.test", StateUpdate{State: "on"})

	if err == nil || !strings.Contains(err.Error(), "not supported via WebSocket") {
		t.Errorf("expected not supported error, got: %v", err)
	}
}

func TestWSClientImplWithSender_GetState_EntityMissing(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{EntityID: "light.other", State: "on"},
	}

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg(entities), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.GetState(context.Background(), "light.nonexistent")

	if err == nil || !strings.Contains(err.Error(), "entity not found") {
		t.Errorf("expected entity not found error, got: %v", err)
	}
}

func TestWSClientImplWithSender_SetHelperValue_DatetimeOtherValue(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, params map[string]any) (*WSResultMessage, error) {
			serviceData := params["service_data"].(map[string]any)
			// The int value should be set as datetime
			if serviceData["datetime"] != 123456789 {
				t.Errorf("datetime mismatch: %v", serviceData["datetime"])
			}
			return makeWSResultMsg(map[string]any{}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	// Test with an int value (not string or map)
	err := client.SetHelperValue(context.Background(), "input_datetime.test", 123456789)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_CallService_NoData(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "call_service" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			// service_data should NOT be present when data is nil
			if _, hasServiceData := params["service_data"]; hasServiceData {
				t.Error("service_data should not be set when data is nil")
			}
			return makeWSResultMsg(map[string]any{"context": Context{ID: "test"}}), nil
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.CallService(context.Background(), "homeassistant", "restart", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWSClientImplWithSender_UpdateEntityRegistryEntry(t *testing.T) {
	t.Parallel()

	// Home Assistant wraps entity registry update responses in an "entity_entry" key.
	// This test verifies the unwrapping is correct and fields are populated.
	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/entity_registry/update" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["entity_id"] != "light.living_room" {
				t.Errorf("unexpected entity_id: %v", params["entity_id"])
			}
			return makeWSResultMsg(map[string]any{
				"entity_entry": map[string]any{
					"entity_id": "light.living_room",
					"name":      "Updated Name",
					"platform":  "hue",
				},
			}), nil
		},
	}

	name := "Updated Name"
	client := newWSClientImplWithSender(mock)
	result, err := client.UpdateEntityRegistryEntry(context.Background(), "light.living_room", EntityRegistryUpdateConfig{
		Name: &name,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EntityID != "light.living_room" {
		t.Errorf("got EntityID %q, want %q", result.EntityID, "light.living_room")
	}
	if result.Name != "Updated Name" {
		t.Errorf("got Name %q, want %q", result.Name, "Updated Name")
	}
}

func TestWSClientImplWithSender_UpdateEntityRegistryEntry_Rename(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return makeWSResultMsg(map[string]any{
				"entity_entry": map[string]any{
					"entity_id": "light.main_room",
					"name":      "Main Room Light",
					"platform":  "hue",
				},
			}), nil
		},
	}

	newID := "light.main_room"
	client := newWSClientImplWithSender(mock)
	result, err := client.UpdateEntityRegistryEntry(context.Background(), "light.living_room", EntityRegistryUpdateConfig{
		NewEntityID: &newID,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EntityID != "light.main_room" {
		t.Errorf("got EntityID %q, want %q", result.EntityID, "light.main_room")
	}
}

func TestWSClientImplWithSender_UpdateEntityRegistryEntry_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("connection failed")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.UpdateEntityRegistryEntry(context.Background(), "light.living_room", EntityRegistryUpdateConfig{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestWSClientImplWithSender_UpdateDeviceRegistryEntry(t *testing.T) {
	t.Parallel()

	// Home Assistant wraps device registry update responses in a "device_entry" key.
	// This test verifies the unwrapping is correct and fields are populated.
	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, cmdType string, params map[string]any) (*WSResultMessage, error) {
			if cmdType != "config/device_registry/update" {
				t.Errorf("unexpected command: %s", cmdType)
			}
			if params["device_id"] != "abc123" {
				t.Errorf("unexpected device_id: %v", params["device_id"])
			}
			return makeWSResultMsg(map[string]any{
				"device_entry": map[string]any{
					"id":           "abc123",
					"name_by_user": "My Custom Name",
					"area_id":      "bedroom",
				},
			}), nil
		},
	}

	nameByUser := "My Custom Name"
	areaID := "bedroom"
	client := newWSClientImplWithSender(mock)
	result, err := client.UpdateDeviceRegistryEntry(context.Background(), "abc123", DeviceRegistryUpdateConfig{
		NameByUser: &nameByUser,
		AreaID:     &areaID,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "abc123" {
		t.Errorf("got ID %q, want %q", result.ID, "abc123")
	}
	if result.NameByUser != "My Custom Name" {
		t.Errorf("got NameByUser %q, want %q", result.NameByUser, "My Custom Name")
	}
	if result.AreaID != "bedroom" {
		t.Errorf("got AreaID %q, want %q", result.AreaID, "bedroom")
	}
}

func TestWSClientImplWithSender_UpdateDeviceRegistryEntry_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWSClientSender{
		sendCommandFunc: func(_ context.Context, _ string, _ map[string]any) (*WSResultMessage, error) {
			return nil, errors.New("connection failed")
		},
	}

	client := newWSClientImplWithSender(mock)
	_, err := client.UpdateDeviceRegistryEntry(context.Background(), "abc123", DeviceRegistryUpdateConfig{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
