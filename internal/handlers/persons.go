// Package handlers provides MCP tool handlers for Home Assistant operations.
package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// Person action constants.
const (
	personActionList   = "list"
	personActionGet    = "get"
	personActionCreate = "create"
	personActionUpdate = "update"
	personActionDelete = "delete"
)

// PersonHandlers provides handlers for person-related MCP tools.
type PersonHandlers struct{}

// NewPersonHandlers creates a new PersonHandlers instance.
func NewPersonHandlers() *PersonHandlers {
	return &PersonHandlers{}
}

// RegisterTools registers the consolidated manage_person tool with the registry.
func (h *PersonHandlers) RegisterTools(registry *mcp.Registry) {
	registry.RegisterTool(h.managePersonTool(), h.handleManagePerson)
}

// =============================================================================
// Tool Definition
// =============================================================================

func (h *PersonHandlers) managePersonTool() mcp.Tool {
	schema := h.buildPersonSchema()
	return mcp.Tool{
		Name: "manage_person",
		Description: `Manage Home Assistant persons - list, get, create, update, or delete.

Persons represent people in your home and can be linked to device trackers for presence detection.

Actions:
- list: List all persons (optional filters: name_contains)
- get: Get details of a specific person (requires person_id)
- create: Create a new person (requires name)
- update: Update an existing person (requires person_id)
- delete: Delete a person (requires person_id)`,
		InputSchema: schema,
	}
}

func (h *PersonHandlers) buildPersonSchema() mcp.JSONSchema {
	return mcp.JSONSchema{
		Type:        "object",
		Description: "Person management operation",
		Properties: map[string]mcp.JSONSchema{
			"action": {
				Type:        "string",
				Description: "Operation to perform: list, get, create, update, delete",
				Enum:        []string{"list", "get", "create", "update", "delete"},
			},
			"person_id": {
				Type:        "string",
				Description: "Person identifier or name. Required for get/update/delete. Accepts exact person_id or case-insensitive name search.",
			},
			"name": {
				Type:        "string",
				Description: "Person name (required for create, optional for update)",
			},
			"user_id": {
				Type:        "string",
				Description: "Associated Home Assistant user ID",
			},
			"device_trackers": {
				Type:        "array",
				Description: "List of device tracker entity IDs for presence detection",
				Items: &mcp.JSONSchema{
					Type: "string",
				},
			},
			"picture": {
				Type:        "string",
				Description: "URL or path to person's picture",
			},
			"name_contains": {
				Type:        "string",
				Description: "Filter by person name containing this string (for list action, case-insensitive)",
			},
			"format": {
				Type:        "string",
				Description: "Output format: 'natural' (default) for LLM-optimized text, 'json' for structured data",
				Enum:        []string{"natural", "json"},
			},
		},
		Required: []string{"action"},
	}
}

// =============================================================================
// Handler: manage_person
// =============================================================================

func (h *PersonHandlers) handleManagePerson(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return errorResult("action is required"), nil
	}

	switch action {
	case personActionList:
		return h.handleList(ctx, client, args)
	case personActionGet:
		return h.handleGet(ctx, client, args)
	case personActionCreate:
		return h.handleCreate(ctx, client, args)
	case personActionUpdate:
		return h.handleUpdate(ctx, client, args)
	case personActionDelete:
		return h.handleDelete(ctx, client, args)
	default:
		return errorResult(fmt.Sprintf("invalid action: %s (must be list, get, create, update, or delete)", action)), nil
	}
}

func (h *PersonHandlers) handleList(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	nameContains, errMsg := stringArgValue("name_contains", args["name_contains"])
	if errMsg != "" {
		return errorResult(errMsg), nil
	}

	persons, err := client.GetPersons(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error listing persons: %v", err)), nil
	}

	// Apply name filter if provided
	if nameContains != "" {
		persons = h.filterPersonsByName(persons, nameContains)
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatListJSON(persons)
	}
	return h.formatListNatural(persons)
}

func (h *PersonHandlers) handleGet(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	personID, _ := args["person_id"].(string)
	if personID == "" {
		return errorResult("person_id is required for get action"), nil
	}

	persons, err := client.GetPersons(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("error getting persons: %v", err)), nil
	}

	person, findErr := h.findPersonByInput(persons, personID)
	if findErr != nil {
		return errorResult(findErr.Error()), nil
	}

	formatStr, _ := args["format"].(string)
	if formatStr == formatJSON {
		return h.formatGetJSON(person)
	}
	return h.formatGetNatural(person)
}

func (h *PersonHandlers) handleCreate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return errorResult("name is required for create action"), nil
	}

	config := h.buildPersonConfig(args)
	config.Name = name

	person, err := client.CreatePerson(ctx, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error creating person: %v", err)), nil
	}

	successMsg := fmt.Sprintf("Person '%s' created successfully with ID: %s", person.Name, person.ID)
	entityID := "person." + person.ID
	if _, appeared := waitForEntityAppear(ctx, client, entityID); !appeared {
		successMsg += " (warning: person entity not yet visible)"
	}

	return successResult(successMsg), nil
}

func (h *PersonHandlers) handleUpdate(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	personID, _ := args["person_id"].(string)
	if personID == "" {
		return errorResult("person_id is required for update action"), nil
	}

	resolvedID, err := h.resolvePersonID(ctx, client, personID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	config := h.buildPersonConfig(args)

	person, err := client.UpdatePerson(ctx, resolvedID, config)
	if err != nil {
		return errorResult(fmt.Sprintf("error updating person: %v", err)), nil
	}

	return successResult(fmt.Sprintf("Person '%s' updated successfully", person.Name)), nil
}

func (h *PersonHandlers) handleDelete(ctx context.Context, client homeassistant.Client, args map[string]any) (*mcp.ToolsCallResult, error) {
	personID, _ := args["person_id"].(string)
	if personID == "" {
		return errorResult("person_id is required for delete action"), nil
	}

	resolvedID, err := h.resolvePersonID(ctx, client, personID)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	if err := client.DeletePerson(ctx, resolvedID); err != nil {
		return errorResult(fmt.Sprintf("error deleting person: %v", err)), nil
	}

	entityID := "person." + resolvedID
	successMsg := fmt.Sprintf("Person '%s' deleted successfully", resolvedID)
	if !waitForEntityDisappear(ctx, client, entityID) {
		successMsg += " (warning: person entity may still be visible briefly)"
	}

	return successResult(successMsg), nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// findPersonByInput performs two-phase lookup: exact ID match, then case-insensitive name substring match.
func (h *PersonHandlers) findPersonByInput(persons []homeassistant.PersonRegistryEntry, input string) (*homeassistant.PersonRegistryEntry, error) {
	// Phase 1: Exact ID match
	for i := range persons {
		if persons[i].ID == input {
			return &persons[i], nil
		}
	}

	// Phase 2: Case-insensitive name substring match
	lowerInput := strings.ToLower(input)
	for i := range persons {
		if strings.Contains(strings.ToLower(persons[i].Name), lowerInput) {
			return &persons[i], nil
		}
	}

	return nil, fmt.Errorf("person not found: %s (tried as person_id and name)", input)
}

// resolvePersonID resolves a person input (ID or name) to the actual person ID.
func (h *PersonHandlers) resolvePersonID(ctx context.Context, client homeassistant.Client, input string) (string, error) {
	persons, err := client.GetPersons(ctx)
	if err != nil {
		return "", fmt.Errorf("error fetching persons: %w", err)
	}

	person, err := h.findPersonByInput(persons, input)
	if err != nil {
		return "", err
	}

	return person.ID, nil
}

func (h *PersonHandlers) buildPersonConfig(args map[string]any) homeassistant.PersonConfig {
	config := homeassistant.PersonConfig{}

	if name, ok := args["name"].(string); ok && name != "" {
		config.Name = name
	}
	if userID, ok := args["user_id"].(string); ok && userID != "" {
		config.UserID = userID
	}
	if deviceTrackers, ok := args["device_trackers"].([]any); ok {
		config.DeviceTrackers = convertToStringSlice(deviceTrackers)
	}
	if picture, ok := args["picture"].(string); ok && picture != "" {
		config.Picture = picture
	}

	return config
}

func (h *PersonHandlers) filterPersonsByName(persons []homeassistant.PersonRegistryEntry, nameContains string) []homeassistant.PersonRegistryEntry {
	filtered := make([]homeassistant.PersonRegistryEntry, 0)
	lowerSearch := strings.ToLower(nameContains)

	for _, person := range persons {
		if strings.Contains(strings.ToLower(person.Name), lowerSearch) {
			filtered = append(filtered, person)
		}
	}

	return filtered
}

// =============================================================================
// Formatting Methods
// =============================================================================

func (h *PersonHandlers) formatListJSON(persons []homeassistant.PersonRegistryEntry) (*mcp.ToolsCallResult, error) {
	return jsonResult(map[string]any{
		"persons": persons,
		"count":   len(persons),
	})
}

func (h *PersonHandlers) formatListNatural(persons []homeassistant.PersonRegistryEntry) (*mcp.ToolsCallResult, error) {
	if len(persons) == 0 {
		return successResult("No persons found."), nil
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Found %d person(s):\n", len(persons)))

	for _, person := range persons {
		line := fmt.Sprintf("• %s (ID: %s)", person.Name, person.ID)
		if person.UserID != "" {
			line += fmt.Sprintf(" - User: %s", person.UserID)
		}
		if len(person.DeviceTrackers) > 0 {
			line += fmt.Sprintf("\n  Device trackers: %s", strings.Join(person.DeviceTrackers, ", "))
		}
		if person.Picture != "" {
			line += fmt.Sprintf("\n  Picture: %s", person.Picture)
		}
		parts = append(parts, line)
	}

	return successResult(strings.Join(parts, "\n")), nil
}

func (h *PersonHandlers) formatGetJSON(person *homeassistant.PersonRegistryEntry) (*mcp.ToolsCallResult, error) {
	return jsonResult(person)
}

func (h *PersonHandlers) formatGetNatural(person *homeassistant.PersonRegistryEntry) (*mcp.ToolsCallResult, error) {
	var parts []string
	parts = append(parts,
		fmt.Sprintf("Person: %s", person.Name),
		fmt.Sprintf("ID: %s", person.ID))

	if person.UserID != "" {
		parts = append(parts, fmt.Sprintf("User ID: %s", person.UserID))
	}
	if len(person.DeviceTrackers) > 0 {
		parts = append(parts, fmt.Sprintf("Device trackers: %s", strings.Join(person.DeviceTrackers, ", ")))
	}
	if person.Picture != "" {
		parts = append(parts, fmt.Sprintf("Picture: %s", person.Picture))
	}

	return successResult(strings.Join(parts, "\n")), nil
}
