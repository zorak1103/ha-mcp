package handlers

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/handlers/formatter"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// PresenceReport represents presence tracking analysis.
type PresenceReport struct {
	Persons               []PersonInfo       `json:"persons"`
	TrackersWithoutPerson []TrackerInfo      `json:"trackers_without_person"`
	PersonsWithoutTracker []string           `json:"persons_without_tracker"`
	Statistics            PresenceStatistics `json:"statistics"`
}

// PersonInfo represents a person entity with their device trackers.
type PersonInfo struct {
	EntityID     string   `json:"entity_id"`
	Name         string   `json:"name"`
	State        string   `json:"state"`
	Trackers     []string `json:"trackers"`
	TrackerCount int      `json:"tracker_count"`
}

// TrackerInfo represents a device tracker entity.
type TrackerInfo struct {
	EntityID string `json:"entity_id"`
	Name     string `json:"name"`
	State    string `json:"state"`
}

// PresenceStatistics represents presence tracking statistics.
type PresenceStatistics struct {
	TotalPersons           int `json:"total_persons"`
	TotalTrackers          int `json:"total_trackers"`
	PersonsWithTrackers    int `json:"persons_with_trackers"`
	PersonsWithoutTrackers int `json:"persons_without_trackers"`
	TrackersWithoutPerson  int `json:"trackers_without_person"`
	TrackersUnavailable    int `json:"trackers_unavailable"`
}

// handlePresence analyzes person entities and device tracker correlation.
func (h *ConsolidatedEntityQueryHandlers) handlePresence(
	ctx context.Context,
	client homeassistant.Client,
	args map[string]any,
) (*mcp.ToolsCallResult, error) {
	// Parse format parameter
	formatStr, _ := args["format"].(string)
	format := formatter.ParseFormat(formatStr)

	// Get all states
	states, err := client.GetStates(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error getting states: %v", err)), nil
	}

	// Build presence report
	report := buildPresenceReport(states)

	// Format output
	var output string
	if format == formatter.FormatJSON {
		data, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			return errorResult(fmt.Sprintf("Error formatting presence report: %v", marshalErr)), nil
		}
		output = string(data)
	} else {
		output = formatPresenceNatural(report)
	}

	return successResult(output), nil
}

// buildPresenceReport builds the presence analysis report from entity states.
func buildPresenceReport(states []homeassistant.Entity) PresenceReport {
	// Separate persons and device trackers
	persons, trackers := separatePersonsAndTrackers(states)

	// Build person info and assigned trackers set
	personInfos, assignedTrackers, personsWithoutTrackers := buildPersonInfoList(persons)

	// Find trackers without person assignment
	trackersWithoutPerson, trackersUnavailable := findUnassignedTrackers(trackers, assignedTrackers)

	// Sort results
	sortPresenceResults(&personInfos, &trackersWithoutPerson, &personsWithoutTrackers)

	// Calculate statistics
	stats := calculatePresenceStatistics(persons, trackers, personsWithoutTrackers, trackersWithoutPerson, trackersUnavailable)

	return PresenceReport{
		Persons:               personInfos,
		TrackersWithoutPerson: trackersWithoutPerson,
		PersonsWithoutTracker: personsWithoutTrackers,
		Statistics:            stats,
	}
}

// separatePersonsAndTrackers separates person and device_tracker entities.
func separatePersonsAndTrackers(states []homeassistant.Entity) ([]homeassistant.Entity, []homeassistant.Entity) {
	persons := make([]homeassistant.Entity, 0)
	trackers := make([]homeassistant.Entity, 0)

	for _, entity := range states {
		if strings.HasPrefix(entity.EntityID, "person.") {
			persons = append(persons, entity)
		} else if strings.HasPrefix(entity.EntityID, "device_tracker.") {
			trackers = append(trackers, entity)
		}
	}

	return persons, trackers
}

// buildPersonInfoList builds person information and tracks assigned trackers.
func buildPersonInfoList(persons []homeassistant.Entity) ([]PersonInfo, map[string]bool, []string) {
	assignedTrackers := make(map[string]bool)
	personInfos := make([]PersonInfo, 0, len(persons))
	personsWithoutTrackers := make([]string, 0)

	for _, person := range persons {
		name := getFriendlyNameFromEntity(person)
		trackerList := extractTrackerList(person)

		for _, tracker := range trackerList {
			assignedTrackers[tracker] = true
		}

		personInfo := PersonInfo{
			EntityID:     person.EntityID,
			Name:         name,
			State:        person.State,
			Trackers:     trackerList,
			TrackerCount: len(trackerList),
		}
		personInfos = append(personInfos, personInfo)

		if len(trackerList) == 0 {
			personsWithoutTrackers = append(personsWithoutTrackers, person.EntityID)
		}
	}

	return personInfos, assignedTrackers, personsWithoutTrackers
}

// findUnassignedTrackers finds device trackers not assigned to any person.
func findUnassignedTrackers(
	trackers []homeassistant.Entity,
	assignedTrackers map[string]bool,
) ([]TrackerInfo, int) {
	trackersWithoutPerson := make([]TrackerInfo, 0)
	trackersUnavailable := 0

	for _, tracker := range trackers {
		if !assignedTrackers[tracker.EntityID] {
			name := getFriendlyNameFromEntity(tracker)
			trackersWithoutPerson = append(trackersWithoutPerson, TrackerInfo{
				EntityID: tracker.EntityID,
				Name:     name,
				State:    tracker.State,
			})
		}

		if tracker.State == "unavailable" || tracker.State == "unknown" {
			trackersUnavailable++
		}
	}

	return trackersWithoutPerson, trackersUnavailable
}

// sortPresenceResults sorts all presence result slices.
func sortPresenceResults(
	personInfos *[]PersonInfo,
	trackersWithoutPerson *[]TrackerInfo,
	personsWithoutTrackers *[]string,
) {
	slices.SortFunc(*personInfos, func(a, b PersonInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(*trackersWithoutPerson, func(a, b TrackerInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})
	sort.Strings(*personsWithoutTrackers)
}

// calculatePresenceStatistics calculates presence tracking statistics.
func calculatePresenceStatistics(
	persons, trackers []homeassistant.Entity,
	personsWithoutTrackers []string,
	trackersWithoutPerson []TrackerInfo,
	trackersUnavailable int,
) PresenceStatistics {
	return PresenceStatistics{
		TotalPersons:           len(persons),
		TotalTrackers:          len(trackers),
		PersonsWithTrackers:    len(persons) - len(personsWithoutTrackers),
		PersonsWithoutTrackers: len(personsWithoutTrackers),
		TrackersWithoutPerson:  len(trackersWithoutPerson),
		TrackersUnavailable:    trackersUnavailable,
	}
}

// extractTrackerList extracts device_trackers array from person attributes.
func extractTrackerList(person homeassistant.Entity) []string {
	trackersAttr, ok := person.Attributes["device_trackers"]
	if !ok {
		return []string{}
	}

	trackersAny, ok := trackersAttr.([]any)
	if !ok {
		return []string{}
	}

	trackers := make([]string, 0, len(trackersAny))
	for _, t := range trackersAny {
		if str, ok := t.(string); ok {
			trackers = append(trackers, str)
		}
	}

	return trackers
}

// getFriendlyNameFromEntity gets friendly_name or falls back to entity_id.
func getFriendlyNameFromEntity(entity homeassistant.Entity) string {
	if name, ok := entity.Attributes["friendly_name"].(string); ok && name != "" {
		return name
	}
	return entity.EntityID
}

// formatPresenceNatural formats presence report in natural language.
func formatPresenceNatural(report PresenceReport) string {
	var result strings.Builder

	fmt.Fprintf(&result, "Presence Tracking Analysis\n\n")

	// Check if there are any persons
	if report.Statistics.TotalPersons == 0 {
		result.WriteString("No persons found in your Home Assistant configuration.")
		return result.String()
	}

	// Summary statistics
	fmt.Fprintf(&result, "Summary:\n")
	fmt.Fprintf(&result, "  Total Persons: %d\n", report.Statistics.TotalPersons)
	fmt.Fprintf(&result, "  Total Device Trackers: %d\n", report.Statistics.TotalTrackers)
	fmt.Fprintf(&result, "  Persons with Trackers: %d\n", report.Statistics.PersonsWithTrackers)
	fmt.Fprintf(&result, "  Persons without Trackers: %d\n", report.Statistics.PersonsWithoutTrackers)
	fmt.Fprintf(&result, "  Unassigned Trackers: %d\n", report.Statistics.TrackersWithoutPerson)
	if report.Statistics.TrackersUnavailable > 0 {
		fmt.Fprintf(&result, "  Unavailable Trackers: %d\n", report.Statistics.TrackersUnavailable)
	}
	result.WriteString("\n")

	// Person details
	if len(report.Persons) > 0 {
		result.WriteString("Persons:\n")
		for _, person := range report.Persons {
			fmt.Fprintf(&result, "  %s (%s) is %s\n", person.Name, person.EntityID, person.State)
			if len(person.Trackers) > 0 {
				fmt.Fprintf(&result, "    Trackers (%d): %s\n", person.TrackerCount, strings.Join(person.Trackers, ", "))
			} else {
				result.WriteString("    No trackers assigned\n")
			}
		}
		result.WriteString("\n")
	}

	// Issues
	hasIssues := len(report.PersonsWithoutTracker) > 0 || len(report.TrackersWithoutPerson) > 0

	if hasIssues {
		result.WriteString("Potential Issues:\n")

		if len(report.PersonsWithoutTracker) > 0 {
			fmt.Fprintf(&result, "  Persons without trackers (%d):\n", len(report.PersonsWithoutTracker))
			for _, personID := range report.PersonsWithoutTracker {
				fmt.Fprintf(&result, "    - %s\n", personID)
			}
		}

		if len(report.TrackersWithoutPerson) > 0 {
			fmt.Fprintf(&result, "  Trackers not assigned to any person (%d):\n", len(report.TrackersWithoutPerson))
			for _, tracker := range report.TrackersWithoutPerson {
				fmt.Fprintf(&result, "    - %s (%s) is %s\n", tracker.Name, tracker.EntityID, tracker.State)
			}
		}
	}

	return strings.TrimSuffix(result.String(), "\n")
}
