package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Action constants for manage_calendar tool.
const (
	calendarActionList        = "list"
	calendarActionGetEvents   = "get_events"
	calendarActionCreateEvent = "create_event"
	calendarActionDeleteEvent = "delete_event"

	calendarDomain = "calendar"
)

// CalendarHandlers provides handlers for calendar operations.
type CalendarHandlers struct{}

// NewCalendarHandlers creates a new calendar handlers instance.
func NewCalendarHandlers() *CalendarHandlers {
	return &CalendarHandlers{}
}

// RegisterCalendarTools registers calendar-related tools with the MCP registry.
func RegisterCalendarTools(registry *mcp.Registry) {
	handler := NewCalendarHandlers()
	registry.RegisterTool(buildManageCalendarTool(), handler.HandleManageCalendar)
}

// buildManageCalendarTool builds the schema for manage_calendar tool.
func buildManageCalendarTool() mcp.Tool {
	return mcp.Tool{
		Name:        "manage_calendar",
		Description: "Manage Home Assistant calendars and events. Supports list (view calendars), get_events (view events in date range), create_event, and delete_event.",
		InputSchema: mcp.JSONSchema{
			Type:       "object",
			Properties: buildCalendarSchemaProperties(),
			Required:   []string{"action"},
		},
	}
}

// buildCalendarSchemaProperties builds the properties map for manage_calendar schema.
//
//nolint:funlen // Schema properties require comprehensive field definitions
func buildCalendarSchemaProperties() map[string]mcp.JSONSchema {
	return map[string]mcp.JSONSchema{
		"action": {
			Type:        "string",
			Description: "Action to perform: 'list', 'get_events', 'create_event', or 'delete_event'.",
			Enum:        []string{calendarActionList, calendarActionGetEvents, calendarActionCreateEvent, calendarActionDeleteEvent},
		},
		attrEntityID:       {Type: "string", Description: "Calendar entity ID (required for get_events, create_event, delete_event, e.g., 'calendar.personal')."},
		"start":            {Type: "string", Description: "Start date/time in ISO 8601 format (required for 'get_events', e.g., '2024-01-15T00:00:00Z')."},
		"end":              {Type: "string", Description: "End date/time in ISO 8601 format (required for 'get_events', e.g., '2024-01-16T00:00:00Z')."},
		"summary":          {Type: "string", Description: "Event title/summary (required for 'create_event')."},
		"start_date_time":  {Type: "string", Description: "Event start datetime in ISO 8601 format (for 'create_event')."},
		"end_date_time":    {Type: "string", Description: "Event end datetime in ISO 8601 format (for 'create_event')."},
		"start_date":       {Type: "string", Description: "Event start date in YYYY-MM-DD format for all-day events (for 'create_event')."},
		"end_date":         {Type: "string", Description: "Event end date in YYYY-MM-DD format for all-day events (for 'create_event')."},
		"description":      {Type: "string", Description: "Event description (for 'create_event')."},
		"location":         {Type: "string", Description: "Event location (for 'create_event')."},
		"uid":              {Type: "string", Description: "Event unique ID (required for 'delete_event')."},
		"recurrence_id":    {Type: "string", Description: "Recurrence ID for deleting specific occurrence (for 'delete_event')."},
		"recurrence_range": {Type: "string", Description: "Recurrence deletion range: 'THIS_AND_FUTURE' (for 'delete_event').", Enum: []string{"THIS_AND_FUTURE"}},
		"format":           {Type: "string", Description: "Output format: 'natural' (default, human-readable) or 'json' (structured JSON).", Enum: []string{formatNatural, formatJSON}, Default: formatNatural},
	}
}

// HandleManageCalendar handles the manage_calendar tool invocation.
func (h *CalendarHandlers) HandleManageCalendar(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	// Extract action
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action parameter is required and must be 'list', 'get_events', 'create_event', or 'delete_event'"), nil
	}

	// Extract format (default: natural)
	format, _ := args["format"].(string)
	if format == "" {
		format = formatNatural
	}

	// Route to action handler
	switch action {
	case calendarActionList:
		return h.handleListCalendars(ctx, client, args, format)
	case calendarActionGetEvents:
		return h.handleGetEvents(ctx, client, args, format)
	case calendarActionCreateEvent:
		return h.handleCreateEvent(ctx, client, args, format)
	case calendarActionDeleteEvent:
		return h.handleDeleteEvent(ctx, client, args, format)
	default:
		return errorResult(fmt.Sprintf("invalid action %q, must be one of: list, get_events, create_event, delete_event", action)), nil
	}
}

// handleListCalendars lists all calendars.
func (h *CalendarHandlers) handleListCalendars(ctx context.Context, client homeassistant.Client, _ map[string]any, format string) (*mcp.ToolsCallResult, error) {
	// Get calendars via REST API
	calendars, err := client.GetCalendars(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get calendars: %v", err)), nil
	}

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(calendars, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal calendars: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format
	return successResult(h.formatCalendarsNatural(calendars)), nil
}

// handleGetEvents retrieves calendar events within a date range.
func (h *CalendarHandlers) handleGetEvents(ctx context.Context, client homeassistant.Client, args map[string]any, format string) (*mcp.ToolsCallResult, error) {
	// Validate required parameters
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for get_events action"), nil
	}

	start, _ := args["start"].(string)
	if start == "" {
		return errorResult("start is required for get_events action"), nil
	}

	end, _ := args["end"].(string)
	if end == "" {
		return errorResult("end is required for get_events action"), nil
	}

	// Get events via REST API
	events, err := client.GetCalendarEvents(ctx, entityID, start, end)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get calendar events: %v", err)), nil
	}

	// Format output
	if format == formatJSON {
		jsonData, err := json.MarshalIndent(events, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("failed to marshal events: %v", err)), nil
		}
		return successResult(string(jsonData)), nil
	}

	// Natural format
	return successResult(h.formatEventsNatural(events)), nil
}

// handleCreateEvent creates a new calendar event.
func (h *CalendarHandlers) handleCreateEvent(ctx context.Context, client homeassistant.Client, args map[string]any, _ string) (*mcp.ToolsCallResult, error) {
	// Validate required parameters
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for create_event action"), nil
	}

	summary, _ := args["summary"].(string)
	if summary == "" {
		return errorResult("summary is required for create_event action"), nil
	}

	// Build service data with required fields and optional parameters
	data := buildCreateEventData(entityID, summary, args)

	// Call calendar.create_event service
	_, err := client.CallService(ctx, calendarDomain, "create_event", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to create event: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Event created in %s: %s", entityID, summary)), nil
}

// buildCreateEventData builds service data for calendar event creation.
func buildCreateEventData(entityID, summary string, args map[string]any) map[string]any {
	data := map[string]any{
		attrEntityID: entityID,
		"summary":    summary,
	}

	// Add start/end datetime or date
	if startDateTime, ok := args["start_date_time"].(string); ok && startDateTime != "" {
		data["start_date_time"] = startDateTime
	}
	if endDateTime, ok := args["end_date_time"].(string); ok && endDateTime != "" {
		data["end_date_time"] = endDateTime
	}
	if startDate, ok := args["start_date"].(string); ok && startDate != "" {
		data["start_date"] = startDate
	}
	if endDate, ok := args["end_date"].(string); ok && endDate != "" {
		data["end_date"] = endDate
	}

	// Add optional fields
	if desc, ok := args["description"].(string); ok && desc != "" {
		data["description"] = desc
	}
	if location, ok := args["location"].(string); ok && location != "" {
		data["location"] = location
	}

	return data
}

// handleDeleteEvent deletes a calendar event.
func (h *CalendarHandlers) handleDeleteEvent(ctx context.Context, client homeassistant.Client, args map[string]any, _ string) (*mcp.ToolsCallResult, error) {
	// Validate required parameters
	entityID, _ := args[attrEntityID].(string)
	if entityID == "" {
		return errorResult("entity_id is required for delete_event action"), nil
	}

	uid, _ := args["uid"].(string)
	if uid == "" {
		return errorResult("uid is required for delete_event action"), nil
	}

	// Build service data
	data := map[string]any{
		attrEntityID: entityID,
		"uid":        uid,
	}

	// Add optional recurrence fields
	if recurrenceID, ok := args["recurrence_id"].(string); ok && recurrenceID != "" {
		data["recurrence_id"] = recurrenceID
	}
	if recurrenceRange, ok := args["recurrence_range"].(string); ok && recurrenceRange != "" {
		data["recurrence_range"] = recurrenceRange
	}

	// Call calendar.delete_event service
	_, err := client.CallService(ctx, calendarDomain, "delete_event", data)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to delete event: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Event deleted from %s (uid: %s)", entityID, uid)), nil
}

// formatCalendarsNatural formats calendars in natural language.
func (h *CalendarHandlers) formatCalendarsNatural(calendars []homeassistant.CalendarEntry) string {
	if len(calendars) == 0 {
		return "No calendars found."
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d calendar(s):", len(calendars)))

	for i, cal := range calendars {
		parts = append(parts,
			fmt.Sprintf("\n%d. %s", i+1, cal.Name),
			fmt.Sprintf("   Entity ID: %s", cal.EntityID))
	}

	return strings.Join(parts, "\n")
}

// formatEventsNatural formats calendar events in natural language.
func (h *CalendarHandlers) formatEventsNatural(events []homeassistant.CalendarEvent) string {
	if len(events) == 0 {
		return "No events found in the specified date range."
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d event(s):", len(events)))

	for i, event := range events {
		eventParts := []string{fmt.Sprintf("\n%d. %s", i+1, event.Summary)}

		// Format start/end times
		startTime := event.Start.DateTime
		if startTime == "" {
			startTime = event.Start.Date
		}
		endTime := event.End.DateTime
		if endTime == "" {
			endTime = event.End.Date
		}

		if startTime != "" && endTime != "" {
			eventParts = append(eventParts, fmt.Sprintf("   Time: %s to %s", startTime, endTime))
		}
		if event.Location != "" {
			eventParts = append(eventParts, fmt.Sprintf("   Location: %s", event.Location))
		}
		if event.Description != "" {
			eventParts = append(eventParts, fmt.Sprintf("   Description: %s", event.Description))
		}
		if event.UID != "" {
			eventParts = append(eventParts, fmt.Sprintf("   UID: %s", event.UID))
		}

		parts = append(parts, eventParts...)
	}

	return strings.Join(parts, "\n")
}
