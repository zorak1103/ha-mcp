package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// mockScheduleClient implements homeassistant.Client for testing.
type mockScheduleClient struct {
	homeassistant.Client
	createHelperFn      func(ctx context.Context, helper homeassistant.HelperConfig) error
	deleteHelperFn      func(ctx context.Context, entityID string) error
	callServiceFn       func(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error)
	getStateFn          func(ctx context.Context, entityID string) (*homeassistant.Entity, error)
	getScheduleConfigFn func(ctx context.Context, entityID string) (map[string]any, error)
}

func (m *mockScheduleClient) CreateHelper(ctx context.Context, helper homeassistant.HelperConfig) error {
	if m.createHelperFn != nil {
		return m.createHelperFn(ctx, helper)
	}
	return nil
}

func (m *mockScheduleClient) DeleteHelper(ctx context.Context, entityID string) error {
	if m.deleteHelperFn != nil {
		return m.deleteHelperFn(ctx, entityID)
	}
	return nil
}

func (m *mockScheduleClient) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if m.callServiceFn != nil {
		return m.callServiceFn(ctx, domain, service, data)
	}
	return nil, nil
}

func (m *mockScheduleClient) GetState(ctx context.Context, entityID string) (*homeassistant.Entity, error) {
	if m.getStateFn != nil {
		return m.getStateFn(ctx, entityID)
	}
	return &homeassistant.Entity{
		EntityID: entityID,
		State:    "on",
		Attributes: map[string]any{
			"friendly_name": "Test Schedule",
		},
	}, nil
}

func (m *mockScheduleClient) GetScheduleConfig(ctx context.Context, entityID string) (map[string]any, error) {
	if m.getScheduleConfigFn != nil {
		return m.getScheduleConfigFn(ctx, entityID)
	}
	return map[string]any{}, nil
}

func TestNewScheduleHandlers(t *testing.T) {
	t.Parallel()

	h := NewScheduleHandlers()
	if h == nil {
		t.Error("NewScheduleHandlers() returned nil")
	}
}

func TestScheduleHandlers_RegisterTools(t *testing.T) {
	t.Parallel()

	h := NewScheduleHandlers()
	registry := mcp.NewRegistry()

	h.RegisterTools(registry)

	tools := registry.ListTools()
	const expectedToolCount = 4
	if len(tools) != expectedToolCount {
		t.Errorf("RegisterTools() registered %d tools, want %d", len(tools), expectedToolCount)
	}

	expectedTools := map[string]bool{
		"get_schedule_details": false,
		"create_schedule":      false,
		"delete_schedule":      false,
		"reload_schedule":      false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("Tool %q not registered", name)
		}
	}
}

func TestScheduleHandlers_createScheduleTool(t *testing.T) {
	t.Parallel()

	h := NewScheduleHandlers()
	tool := h.createScheduleTool()

	if tool.Name != "create_schedule" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "create_schedule")
	}

	if tool.InputSchema.Type != testSchemaTypeObject {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, testSchemaTypeObject)
	}

	const expectedRequiredCount = 2
	if len(tool.InputSchema.Required) != expectedRequiredCount {
		t.Errorf("InputSchema.Required length = %d, want %d", len(tool.InputSchema.Required), expectedRequiredCount)
	}

	requiredFields := map[string]bool{"id": false, "name": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}

	// Check day properties exist
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for _, day := range days {
		if _, ok := tool.InputSchema.Properties[day]; !ok {
			t.Errorf("Day property %q not found in schema", day)
		}
	}
}

func TestScheduleHandlers_getScheduleDetailsTool(t *testing.T) {
	t.Parallel()

	h := NewScheduleHandlers()
	tool := h.getScheduleDetailsTool()

	if tool.Name != "get_schedule_details" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "get_schedule_details")
	}

	requiredFields := map[string]bool{"entity_id": false}
	for _, field := range tool.InputSchema.Required {
		requiredFields[field] = true
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %q not found", field)
		}
	}
}

func TestScheduleHandlers_reloadScheduleTool(t *testing.T) {
	t.Parallel()

	h := NewScheduleHandlers()
	tool := h.reloadScheduleTool()

	if tool.Name != "reload_schedule" {
		t.Errorf("Tool name = %q, want %q", tool.Name, "reload_schedule")
	}

	// No required fields for reload
	if len(tool.InputSchema.Required) != 0 {
		t.Errorf("InputSchema.Required length = %d, want 0", len(tool.InputSchema.Required))
	}
}

func TestScheduleHandlers_handleGetScheduleDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		getStateErr     error
		getConfigErr    error
		getStateResult  *homeassistant.Entity
		getConfigResult map[string]any
		wantError       bool
		wantContains    string
		wantNotContains string
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": "schedule.work_hours",
			},
			getStateResult: &homeassistant.Entity{
				EntityID: "schedule.work_hours",
				State:    "on",
				Attributes: map[string]any{
					"friendly_name": "Work Hours",
					"icon":          "mdi:clock",
					"next_event":    "2024-01-15T08:00:00",
				},
			},
			getConfigResult: map[string]any{
				"monday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
				"tuesday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
			},
			wantError:    false,
			wantContains: "schedule.work_hours",
		},
		{
			name: "success with config error fallback",
			args: map[string]any{
				"entity_id": "schedule.work_hours",
			},
			getStateResult: &homeassistant.Entity{
				EntityID: "schedule.work_hours",
				State:    "off",
				Attributes: map[string]any{
					"friendly_name": "Work Hours",
				},
			},
			getConfigErr: errors.New("config not available"),
			wantError:    false,
			wantContains: "schedule.work_hours",
		},
		{
			name:         "missing entity_id",
			args:         map[string]any{},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "timer.test_timer",
			},
			wantError:    true,
			wantContains: "must be a schedule entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "schedule.work_hours",
			},
			getStateErr:  errors.New("entity not found"),
			wantError:    true,
			wantContains: "Error getting schedule state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScheduleClient{
				getStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
					if tt.getStateErr != nil {
						return nil, tt.getStateErr
					}
					if tt.getStateResult != nil {
						return tt.getStateResult, nil
					}
					return &homeassistant.Entity{
						EntityID:   "schedule.work_hours",
						State:      "on",
						Attributes: map[string]any{},
					}, nil
				},
				getScheduleConfigFn: func(_ context.Context, _ string) (map[string]any, error) {
					if tt.getConfigErr != nil {
						return nil, tt.getConfigErr
					}
					if tt.getConfigResult != nil {
						return tt.getConfigResult, nil
					}
					return map[string]any{}, nil
				},
			}

			h := NewScheduleHandlers()
			result, err := h.handleGetScheduleDetails(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleGetScheduleDetails() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestScheduleHandlers_handleCreateSchedule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		createHelperErr error
		wantError       bool
		wantContains    string
	}{
		{
			name: "success",
			args: map[string]any{
				"id":   "work_hours",
				"name": "Work Hours",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "success with schedule",
			args: map[string]any{
				"id":   "work_hours",
				"name": "Work Hours",
				"monday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
				"tuesday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
				"icon": "mdi:calendar-clock",
			},
			wantError:    false,
			wantContains: "created successfully",
		},
		{
			name: "missing id",
			args: map[string]any{
				"name": "Work Hours",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "empty id",
			args: map[string]any{
				"id":   "",
				"name": "Work Hours",
			},
			wantError:    true,
			wantContains: "id is required",
		},
		{
			name: "missing name",
			args: map[string]any{
				"id": "work_hours",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "empty name",
			args: map[string]any{
				"id":   "work_hours",
				"name": "",
			},
			wantError:    true,
			wantContains: "name is required",
		},
		{
			name: "client error",
			args: map[string]any{
				"id":   "work_hours",
				"name": "Work Hours",
			},
			createHelperErr: errors.New("connection failed"),
			wantError:       true,
			wantContains:    "Error creating schedule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScheduleClient{
				createHelperFn: func(_ context.Context, _ homeassistant.HelperConfig) error {
					return tt.createHelperErr
				},
			}

			h := NewScheduleHandlers()
			result, err := h.handleCreateSchedule(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleCreateSchedule() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestScheduleHandlers_handleDeleteSchedule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            map[string]any
		deleteHelperErr error
		wantError       bool
		wantContains    string
	}{
		{
			name: "success",
			args: map[string]any{
				"entity_id": "schedule.work_hours",
			},
			wantError:    false,
			wantContains: "deleted successfully",
		},
		{
			name:         "missing entity_id",
			args:         map[string]any{},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "empty entity_id",
			args: map[string]any{
				"entity_id": "",
			},
			wantError:    true,
			wantContains: "entity_id is required",
		},
		{
			name: "invalid platform",
			args: map[string]any{
				"entity_id": "counter.test_counter",
			},
			wantError:    true,
			wantContains: "must be a schedule entity",
		},
		{
			name: "client error",
			args: map[string]any{
				"entity_id": "schedule.work_hours",
			},
			deleteHelperErr: errors.New("not found"),
			wantError:       true,
			wantContains:    "Error deleting schedule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScheduleClient{
				deleteHelperFn: func(_ context.Context, _ string) error {
					return tt.deleteHelperErr
				},
			}

			h := NewScheduleHandlers()
			result, err := h.handleDeleteSchedule(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleDeleteSchedule() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestScheduleHandlers_handleReloadSchedule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           map[string]any
		callServiceErr error
		wantError      bool
		wantContains   string
	}{
		{
			name:         "success",
			args:         map[string]any{},
			wantError:    false,
			wantContains: "reloaded successfully",
		},
		{
			name:           "client error",
			args:           map[string]any{},
			callServiceErr: errors.New("service unavailable"),
			wantError:      true,
			wantContains:   "Error reloading schedules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &mockScheduleClient{
				callServiceFn: func(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
					return nil, tt.callServiceErr
				},
			}

			h := NewScheduleHandlers()
			result, err := h.handleReloadSchedule(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("handleReloadSchedule() returned error: %v", err)
				return
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Error("Content is empty")
				return
			}

			content := result.Content[0].Text
			if tt.wantContains != "" && !contains(content, tt.wantContains) {
				t.Errorf("Content = %q, want to contain %q", content, tt.wantContains)
			}
		})
	}
}

func TestValidateScheduleEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		entityID  string
		wantError bool
		wantMsg   string
	}{
		{
			name:      "valid schedule entity",
			entityID:  "schedule.work_hours",
			wantError: false,
		},
		{
			name:      "empty entity_id",
			entityID:  "",
			wantError: true,
			wantMsg:   "entity_id is required",
		},
		{
			name:      "wrong platform - timer",
			entityID:  "timer.test_timer",
			wantError: true,
			wantMsg:   "must be a schedule entity",
		},
		{
			name:      "wrong platform - input_boolean",
			entityID:  "input_boolean.test",
			wantError: true,
			wantMsg:   "must be a schedule entity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateScheduleEntityID(tt.entityID)

			if tt.wantError {
				if err == nil {
					t.Error("validateScheduleEntityID() expected error, got nil")
					return
				}
				if tt.wantMsg != "" && !contains(err.Error(), tt.wantMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantMsg)
				}
			} else if err != nil {
				t.Errorf("validateScheduleEntityID() unexpected error: %v", err)
			}
		})
	}
}

func TestExtractScheduleAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		attributes       map[string]any
		wantFriendlyName string
		wantIcon         string
		wantNextEvent    string
	}{
		{
			name:             "all attributes present",
			attributes:       map[string]any{"friendly_name": "Work Hours", "icon": "mdi:clock", "next_event": "2024-01-15T08:00:00"},
			wantFriendlyName: "Work Hours",
			wantIcon:         "mdi:clock",
			wantNextEvent:    "2024-01-15T08:00:00",
		},
		{
			name:             "empty attributes",
			attributes:       map[string]any{},
			wantFriendlyName: "",
			wantIcon:         "",
			wantNextEvent:    "",
		},
		{
			name:             "nil attributes",
			attributes:       nil,
			wantFriendlyName: "",
			wantIcon:         "",
			wantNextEvent:    "",
		},
		{
			name:             "partial attributes",
			attributes:       map[string]any{"friendly_name": "Test Schedule"},
			wantFriendlyName: "Test Schedule",
			wantIcon:         "",
			wantNextEvent:    "",
		},
		{
			name:             "wrong type for friendly_name",
			attributes:       map[string]any{"friendly_name": 123},
			wantFriendlyName: "",
			wantIcon:         "",
			wantNextEvent:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			friendlyName, icon, nextEvent := extractScheduleAttributes(tt.attributes)

			if friendlyName != tt.wantFriendlyName {
				t.Errorf("friendlyName = %q, want %q", friendlyName, tt.wantFriendlyName)
			}
			if icon != tt.wantIcon {
				t.Errorf("icon = %q, want %q", icon, tt.wantIcon)
			}
			if nextEvent != tt.wantNextEvent {
				t.Errorf("nextEvent = %q, want %q", nextEvent, tt.wantNextEvent)
			}
		})
	}
}

func TestParseBlocksForDay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		blocks []any
		want   []TimeBlock
	}{
		{
			name: "single block",
			blocks: []any{
				map[string]any{"from": "08:00:00", "to": "17:00:00"},
			},
			want: []TimeBlock{{From: "08:00:00", To: "17:00:00"}},
		},
		{
			name: "multiple blocks",
			blocks: []any{
				map[string]any{"from": "08:00:00", "to": "12:00:00"},
				map[string]any{"from": "13:00:00", "to": "17:00:00"},
			},
			want: []TimeBlock{
				{From: "08:00:00", To: "12:00:00"},
				{From: "13:00:00", To: "17:00:00"},
			},
		},
		{
			name:   "empty blocks",
			blocks: []any{},
			want:   nil,
		},
		{
			name:   "nil blocks",
			blocks: nil,
			want:   nil,
		},
		{
			name: "invalid block type ignored",
			blocks: []any{
				"invalid",
				map[string]any{"from": "08:00:00", "to": "17:00:00"},
			},
			want: []TimeBlock{{From: "08:00:00", To: "17:00:00"}},
		},
		{
			name: "missing from field",
			blocks: []any{
				map[string]any{"to": "17:00:00"},
			},
			want: []TimeBlock{{From: "", To: "17:00:00"}},
		},
		{
			name: "missing to field",
			blocks: []any{
				map[string]any{"from": "08:00:00"},
			},
			want: []TimeBlock{{From: "08:00:00", To: ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseBlocksForDay(tt.blocks)

			if len(got) != len(tt.want) {
				t.Errorf("parseBlocksForDay() returned %d blocks, want %d", len(got), len(tt.want))
				return
			}

			for i, block := range got {
				if block.From != tt.want[i].From {
					t.Errorf("block[%d].From = %q, want %q", i, block.From, tt.want[i].From)
				}
				if block.To != tt.want[i].To {
					t.Errorf("block[%d].To = %q, want %q", i, block.To, tt.want[i].To)
				}
			}
		})
	}
}

func TestParseTimeBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config map[string]any
		want   map[string][]TimeBlock
	}{
		{
			name: "single day",
			config: map[string]any{
				"monday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
			},
			want: map[string][]TimeBlock{
				"monday": {{From: "08:00:00", To: "17:00:00"}},
			},
		},
		{
			name: "multiple days",
			config: map[string]any{
				"monday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
				"friday": []any{
					map[string]any{"from": "08:00:00", "to": "12:00:00"},
				},
			},
			want: map[string][]TimeBlock{
				"monday": {{From: "08:00:00", To: "17:00:00"}},
				"friday": {{From: "08:00:00", To: "12:00:00"}},
			},
		},
		{
			name:   "empty config",
			config: map[string]any{},
			want:   map[string][]TimeBlock{},
		},
		{
			name: "non-day keys ignored",
			config: map[string]any{
				"name": "Test",
				"monday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
			},
			want: map[string][]TimeBlock{
				"monday": {{From: "08:00:00", To: "17:00:00"}},
			},
		},
		{
			name: "wrong type for day value ignored",
			config: map[string]any{
				"monday": "not an array",
			},
			want: map[string][]TimeBlock{},
		},
		{
			name: "empty day array ignored",
			config: map[string]any{
				"monday": []any{},
			},
			want: map[string][]TimeBlock{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseTimeBlocks(tt.config)

			if len(got) != len(tt.want) {
				t.Errorf("parseTimeBlocks() returned %d days, want %d", len(got), len(tt.want))
			}

			for day, wantBlocks := range tt.want {
				gotBlocks, ok := got[day]
				if !ok {
					t.Errorf("parseTimeBlocks() missing day %q", day)
					continue
				}
				if len(gotBlocks) != len(wantBlocks) {
					t.Errorf("parseTimeBlocks()[%q] has %d blocks, want %d", day, len(gotBlocks), len(wantBlocks))
					continue
				}
				for i, block := range gotBlocks {
					if block.From != wantBlocks[i].From || block.To != wantBlocks[i].To {
						t.Errorf("parseTimeBlocks()[%q][%d] = %+v, want %+v", day, i, block, wantBlocks[i])
					}
				}
			}
		})
	}
}

func TestBuildScheduleDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		state      *homeassistant.Entity
		timeBlocks map[string][]TimeBlock
		want       ScheduleDetails
	}{
		{
			name: "full details",
			state: &homeassistant.Entity{
				EntityID: "schedule.work_hours",
				State:    "on",
				Attributes: map[string]any{
					"friendly_name": "Work Hours",
					"icon":          "mdi:clock",
					"next_event":    "2024-01-15T08:00:00",
				},
			},
			timeBlocks: map[string][]TimeBlock{
				"monday": {{From: "08:00:00", To: "17:00:00"}},
			},
			want: ScheduleDetails{
				EntityID:     "schedule.work_hours",
				State:        "on",
				FriendlyName: "Work Hours",
				Icon:         "mdi:clock",
				NextEvent:    "2024-01-15T08:00:00",
				Days: map[string][]TimeBlock{
					"monday": {{From: "08:00:00", To: "17:00:00"}},
				},
			},
		},
		{
			name: "minimal details",
			state: &homeassistant.Entity{
				EntityID:   "schedule.test",
				State:      "off",
				Attributes: map[string]any{},
			},
			timeBlocks: map[string][]TimeBlock{},
			want: ScheduleDetails{
				EntityID:     "schedule.test",
				State:        "off",
				FriendlyName: "",
				Icon:         "",
				NextEvent:    "",
				Days:         map[string][]TimeBlock{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildScheduleDetails(tt.state, tt.timeBlocks)

			if got.EntityID != tt.want.EntityID {
				t.Errorf("EntityID = %q, want %q", got.EntityID, tt.want.EntityID)
			}
			if got.State != tt.want.State {
				t.Errorf("State = %q, want %q", got.State, tt.want.State)
			}
			if got.FriendlyName != tt.want.FriendlyName {
				t.Errorf("FriendlyName = %q, want %q", got.FriendlyName, tt.want.FriendlyName)
			}
			if got.Icon != tt.want.Icon {
				t.Errorf("Icon = %q, want %q", got.Icon, tt.want.Icon)
			}
			if got.NextEvent != tt.want.NextEvent {
				t.Errorf("NextEvent = %q, want %q", got.NextEvent, tt.want.NextEvent)
			}
			if len(got.Days) != len(tt.want.Days) {
				t.Errorf("Days length = %d, want %d", len(got.Days), len(tt.want.Days))
			}
		})
	}
}

func TestBuildScheduleHelperConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputName  string
		args       map[string]any
		wantKeys   []string
		wantValues map[string]any
	}{
		{
			name:      "name only",
			inputName: "Work Hours",
			args:      map[string]any{},
			wantKeys:  []string{"name"},
			wantValues: map[string]any{
				"name": "Work Hours",
			},
		},
		{
			name:      "with icon",
			inputName: "Work Hours",
			args: map[string]any{
				"icon": "mdi:calendar-clock",
			},
			wantKeys: []string{"name", "icon"},
			wantValues: map[string]any{
				"name": "Work Hours",
				"icon": "mdi:calendar-clock",
			},
		},
		{
			name:      "with monday schedule",
			inputName: "Work Hours",
			args: map[string]any{
				"monday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
			},
			wantKeys: []string{"name", "monday"},
		},
		{
			name:      "with weekday schedules",
			inputName: "Work Hours",
			args: map[string]any{
				"monday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
				"tuesday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
				"wednesday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
				"thursday": []any{
					map[string]any{"from": "08:00:00", "to": "17:00:00"},
				},
				"friday": []any{
					map[string]any{"from": "08:00:00", "to": "12:00:00"},
				},
			},
			wantKeys: []string{"name", "monday", "tuesday", "wednesday", "thursday", "friday"},
		},
		{
			name:      "empty icon ignored",
			inputName: "Work Hours",
			args: map[string]any{
				"icon": "",
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Work Hours",
			},
		},
		{
			name:      "empty day schedule ignored",
			inputName: "Work Hours",
			args: map[string]any{
				"monday": []any{},
			},
			wantKeys: []string{"name"},
			wantValues: map[string]any{
				"name": "Work Hours",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := buildScheduleHelperConfig(tt.inputName, tt.args)

			for _, key := range tt.wantKeys {
				if _, ok := config[key]; !ok {
					t.Errorf("Config missing key %q", key)
				}
			}

			for key, wantVal := range tt.wantValues {
				if gotVal, ok := config[key]; !ok {
					t.Errorf("Config missing key %q", key)
				} else if gotVal != wantVal {
					t.Errorf("Config[%q] = %v, want %v", key, gotVal, wantVal)
				}
			}
		})
	}
}
