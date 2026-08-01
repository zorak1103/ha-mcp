package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// TestManageCalendarSchema verifies the schema for manage_calendar tool.
func TestManageCalendarSchema(t *testing.T) {
	t.Parallel()

	registry := mcp.NewRegistry()
	RegisterCalendarTools(registry)

	tool, exists := registry.GetTool("manage_calendar")
	if !exists {
		t.Fatal("manage_calendar tool not registered")
	}

	// Verify basic properties
	if tool.Name != "manage_calendar" {
		t.Errorf("tool.Name = %q, want %q", tool.Name, "manage_calendar")
	}

	// Verify schema
	schema := tool.InputSchema
	props := schema.Properties

	// Check action field
	actionSchema, ok := props["action"]
	if !ok {
		t.Fatal("action property missing from schema")
	}
	if len(actionSchema.Enum) != 4 {
		t.Errorf("action enum count = %d, want 4 (list, get_events, create_event, delete_event)", len(actionSchema.Enum))
	}
}

// TestManageCalendar_List verifies list action.
func TestManageCalendar_List(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCalendarsFn: func(context.Context) ([]homeassistant.CalendarEntry, error) {
			return []homeassistant.CalendarEntry{
				{EntityID: "calendar.holidays", Name: "Holidays"},
				{EntityID: "calendar.personal", Name: "Personal"},
			}, nil
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action": "list",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Holidays") {
		t.Errorf("result text does not contain 'Holidays': %s", text)
	}
}

// TestManageCalendar_GetEvents verifies get_events action.
func TestManageCalendar_GetEvents(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCalendarEventsFn: func(context.Context, string, string, string) ([]homeassistant.CalendarEvent, error) {
			return []homeassistant.CalendarEvent{
				{
					Start: homeassistant.CalendarDateTime{
						DateTime: "2024-01-15T10:00:00Z",
					},
					End: homeassistant.CalendarDateTime{
						DateTime: "2024-01-15T11:00:00Z",
					},
					Summary:     "Team Meeting",
					Description: "Weekly sync",
				},
			}, nil
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":    "get_events",
		"entity_id": "calendar.work",
		"start":     "2024-01-15T00:00:00Z",
		"end":       "2024-01-16T00:00:00Z",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Team Meeting") {
		t.Errorf("result text does not contain 'Team Meeting': %s", text)
	}
}

// TestManageCalendar_GetEventsMissingParams verifies validation.
func TestManageCalendar_GetEventsMissingParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "missing entity_id",
			args: map[string]any{
				"action": "get_events",
				"start":  "2024-01-15T00:00:00Z",
				"end":    "2024-01-16T00:00:00Z",
			},
		},
		{
			name: "missing start",
			args: map[string]any{
				"action":    "get_events",
				"entity_id": "calendar.work",
				"end":       "2024-01-16T00:00:00Z",
			},
		},
		{
			name: "missing end",
			args: map[string]any{
				"action":    "get_events",
				"entity_id": "calendar.work",
				"start":     "2024-01-15T00:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{}
			handler := NewCalendarHandlers()

			result, err := handler.HandleManageCalendar(context.Background(), client, tt.args)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil || !result.IsError {
				t.Error("expected error result")
			}
		})
	}
}

// TestManageCalendar_CreateEvent verifies create_event action.
func TestManageCalendar_CreateEvent(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "calendar" {
				return nil, fmt.Errorf("wrong domain: %s", domain)
			}
			if service != "create_event" {
				return nil, fmt.Errorf("wrong service: %s", service)
			}
			if data["entity_id"] != "calendar.personal" {
				return nil, fmt.Errorf("data[entity_id] = %v, want %q", data["entity_id"], "calendar.personal")
			}
			if data["summary"] != "Doctor Appointment" {
				return nil, fmt.Errorf("data[summary] = %v, want %q", data["summary"], "Doctor Appointment")
			}
			return nil, nil
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":          "create_event",
		"entity_id":       "calendar.personal",
		"summary":         "Doctor Appointment",
		"start_date_time": "2024-01-20T14:00:00Z",
		"end_date_time":   "2024-01-20T15:00:00Z",
		"description":     "Annual checkup",
		"location":        "Medical Center",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "created") {
		t.Errorf("result does not indicate creation: %s", text)
	}
}

// TestManageCalendar_DeleteEvent verifies delete_event action.
func TestManageCalendar_DeleteEvent(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "calendar" {
				return nil, fmt.Errorf("wrong domain: %s", domain)
			}
			if service != "delete_event" {
				return nil, fmt.Errorf("wrong service: %s", service)
			}
			if data["entity_id"] != "calendar.personal" {
				return nil, fmt.Errorf("data[entity_id] = %v, want %q", data["entity_id"], "calendar.personal")
			}
			if data["uid"] != "event123" {
				return nil, fmt.Errorf("data[uid] = %v, want %q", data["uid"], "event123")
			}
			return nil, nil
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":    "delete_event",
		"entity_id": "calendar.personal",
		"uid":       "event123",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Error("expected success result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "deleted") {
		t.Errorf("result does not indicate deletion: %s", text)
	}
}

// TestManageCalendar_List_Error verifies list handles client error.
func TestManageCalendar_List_Error(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCalendarsFn: func(context.Context) ([]homeassistant.CalendarEntry, error) {
			return nil, fmt.Errorf("calendar API unavailable")
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action": "list",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageCalendar_List_JSONFormat verifies list action with JSON format.
func TestManageCalendar_List_JSONFormat(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCalendarsFn: func(context.Context) ([]homeassistant.CalendarEntry, error) {
			return []homeassistant.CalendarEntry{
				{EntityID: "calendar.work", Name: "Work"},
			}, nil
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action": "list",
		"format": "json",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "calendar.work") {
		t.Errorf("JSON result does not contain entity_id: %s", text)
	}
}

// TestManageCalendar_GetEvents_Error verifies get_events handles client error.
func TestManageCalendar_GetEvents_Error(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCalendarEventsFn: func(context.Context, string, string, string) ([]homeassistant.CalendarEvent, error) {
			return nil, fmt.Errorf("events fetch failed")
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":    "get_events",
		"entity_id": "calendar.work",
		"start":     "2024-01-15T00:00:00Z",
		"end":       "2024-01-16T00:00:00Z",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageCalendar_GetEvents_JSONFormat verifies get_events JSON format.
func TestManageCalendar_GetEvents_JSONFormat(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCalendarEventsFn: func(context.Context, string, string, string) ([]homeassistant.CalendarEvent, error) {
			return []homeassistant.CalendarEvent{
				{
					Start:   homeassistant.CalendarDateTime{DateTime: "2024-01-15T10:00:00Z"},
					End:     homeassistant.CalendarDateTime{DateTime: "2024-01-15T11:00:00Z"},
					Summary: "Team Meeting",
				},
			}, nil
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":    "get_events",
		"entity_id": "calendar.work",
		"start":     "2024-01-15T00:00:00Z",
		"end":       "2024-01-16T00:00:00Z",
		"format":    "json",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Team Meeting") {
		t.Errorf("JSON result does not contain summary: %s", text)
	}
}

// TestManageCalendar_CreateEvent_MissingSummary verifies create_event validation.
func TestManageCalendar_CreateEvent_MissingSummary(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewCalendarHandlers()

	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":    "create_event",
		"entity_id": "calendar.personal",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageCalendar_CreateEvent_Error verifies create_event handles client error.
func TestManageCalendar_CreateEvent_Error(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceFn: func(context.Context, string, string, map[string]any) ([]homeassistant.Entity, error) {
			return nil, fmt.Errorf("create failed")
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":    "create_event",
		"entity_id": "calendar.personal",
		"summary":   "New Event",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageCalendar_DeleteEvent_MissingUID verifies delete_event validation.
func TestManageCalendar_DeleteEvent_MissingUID(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewCalendarHandlers()

	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":    "delete_event",
		"entity_id": "calendar.personal",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageCalendar_DeleteEvent_Error verifies delete_event handles client error.
func TestManageCalendar_DeleteEvent_Error(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		CallServiceFn: func(context.Context, string, string, map[string]any) ([]homeassistant.Entity, error) {
			return nil, fmt.Errorf("delete failed")
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":    "delete_event",
		"entity_id": "calendar.personal",
		"uid":       "event123",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageCalendar_GetEvents_DateOnly verifies formatEventsNatural with date-only events.
func TestManageCalendar_GetEvents_DateOnly(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{
		GetCalendarEventsFn: func(context.Context, string, string, string) ([]homeassistant.CalendarEvent, error) {
			return []homeassistant.CalendarEvent{
				{
					// Date-only event (no DateTime)
					Start:   homeassistant.CalendarDateTime{Date: "2024-01-20"},
					End:     homeassistant.CalendarDateTime{Date: "2024-01-21"},
					Summary: "All-day Event",
				},
			}, nil
		},
	}

	handler := NewCalendarHandlers()
	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":    "get_events",
		"entity_id": "calendar.personal",
		"start":     "2024-01-20T00:00:00Z",
		"end":       "2024-01-21T00:00:00Z",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected result content")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "All-day Event") {
		t.Errorf("result does not contain event summary: %s", text)
	}
	if !strings.Contains(text, "2024-01-20") {
		t.Errorf("result does not contain date: %s", text)
	}
}

// TestManageCalendar_InvalidAction verifies invalid action returns error.
func TestManageCalendar_InvalidAction(t *testing.T) {
	t.Parallel()

	client := &UniversalMockClient{}
	handler := NewCalendarHandlers()

	result, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action": "invalid",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected error result")
	}
}

// TestManageCalendar_CreateEvent_WithStartDate verifies buildCreateEventData with start_date.
func TestManageCalendar_CreateEvent_WithStartDate(t *testing.T) {
	t.Parallel()

	var capturedData map[string]any
	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "calendar" || service != "create_event" {
				return nil, fmt.Errorf("wrong call: %s.%s", domain, service)
			}
			capturedData = data
			return nil, nil
		},
	}

	handler := NewCalendarHandlers()
	_, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":     "create_event",
		"entity_id":  "calendar.personal",
		"summary":    "Birthday",
		"start_date": "2024-02-01",
		"end_date":   "2024-02-02",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedData["start_date"] != "2024-02-01" {
		t.Errorf("start_date = %v, want '2024-02-01'", capturedData["start_date"])
	}
}

// TestManageCalendar_DeleteEvent_WithRecurrence verifies delete_event passes recurrence fields.
func TestManageCalendar_DeleteEvent_WithRecurrence(t *testing.T) {
	t.Parallel()

	var capturedData map[string]any
	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
			if domain != "calendar" || service != "delete_event" {
				return nil, fmt.Errorf("wrong call: %s.%s", domain, service)
			}
			capturedData = data
			return nil, nil
		},
	}

	handler := NewCalendarHandlers()
	_, err := handler.HandleManageCalendar(context.Background(), client, map[string]any{
		"action":           "delete_event",
		"entity_id":        "calendar.personal",
		"uid":              "event123",
		"recurrence_id":    "2024-01-15T10:00:00Z",
		"recurrence_range": "THIS",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedData["recurrence_id"] != "2024-01-15T10:00:00Z" {
		t.Errorf("recurrence_id = %v, want set", capturedData["recurrence_id"])
	}
	if capturedData["recurrence_range"] != "THIS" {
		t.Errorf("recurrence_range = %v, want 'THIS'", capturedData["recurrence_range"])
	}
}
