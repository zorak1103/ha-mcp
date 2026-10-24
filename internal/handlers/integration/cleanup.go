//go:build integration

package integration

import (
	"context"
	stderrors "errors" // aliased: this file already names local variables "errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// CleanupTimeout is the default timeout for cleanup operations.
const CleanupTimeout = 30 * time.Second

// HelperPlatforms lists all helper platforms that can be cleaned up.
var HelperPlatforms = []string{
	"input_boolean",
	"input_number",
	"input_text",
	"input_select",
	"input_datetime",
	"input_button",
	"counter",
	"timer",
	"schedule",
	"group",
}

// SensorPlatforms lists sensor-type helpers that need special cleanup.
var SensorPlatforms = []string{
	"sensor",        // template sensors, derivative, integral
	"binary_sensor", // template binary sensors, threshold
}

// CleanupAllTestEntities removes all entities with the test prefix.
// This should be called at the start of test suites to ensure a clean state.
func CleanupAllTestEntities(ctx context.Context, client homeassistant.Client) error {
	ctx, cancel := context.WithTimeout(ctx, CleanupTimeout)
	defer cancel()

	var errors []string

	// Clean up helpers
	if err := cleanupTestHelpers(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("helpers: %v", err))
	}

	// Clean up automations
	if err := cleanupTestAutomations(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("automations: %v", err))
	}

	// Clean up scripts
	if err := cleanupTestScripts(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("scripts: %v", err))
	}

	// Clean up scenes
	if err := cleanupTestScenes(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("scenes: %v", err))
	}

	// Clean up areas
	if err := cleanupTestAreas(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("areas: %v", err))
	}

	// Clean up dashboards
	if err := cleanupTestDashboards(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("dashboards: %v", err))
	}

	// Clean up labels
	if err := cleanupTestLabels(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("labels: %v", err))
	}

	// Clean up floors
	if err := cleanupTestFloors(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("floors: %v", err))
	}

	// Clean up tags
	if err := cleanupTestTags(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("tags: %v", err))
	}

	// Clean up zones
	if err := cleanupTestZones(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("zones: %v", err))
	}

	// Clean up persons
	if err := cleanupTestPersons(ctx, client); err != nil {
		errors = append(errors, fmt.Sprintf("persons: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// isHelperDomain checks if a domain is a helper or sensor platform.
func isHelperDomain(domain string) bool {
	for _, platform := range HelperPlatforms {
		if domain == platform {
			return true
		}
	}
	for _, platform := range SensorPlatforms {
		if domain == platform {
			return true
		}
	}
	return false
}

// cleanupTestHelpers removes all test helper entities.
func cleanupTestHelpers(ctx context.Context, client homeassistant.Client) error {
	states, err := client.GetStates(ctx)
	if err != nil {
		return fmt.Errorf("failed to get states: %w", err)
	}

	var errors []string

	for _, entity := range states {
		if !IsTestEntity(entity.EntityID) {
			continue
		}

		domain := strings.SplitN(entity.EntityID, ".", 2)[0]
		if !isHelperDomain(domain) {
			continue
		}

		if err := deleteHelperWithRetry(ctx, client, entity.EntityID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", entity.EntityID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete helpers: %s", strings.Join(errors, "; "))
	}

	return nil
}

// cleanupTestAutomations removes all test automations.
func cleanupTestAutomations(ctx context.Context, client homeassistant.Client) error {
	automations, err := client.ListAutomations(ctx)
	if err != nil {
		return fmt.Errorf("failed to list automations: %w", err)
	}

	var errors []string

	for _, auto := range automations {
		// Check entity_id for test prefix
		entityID := auto.EntityID

		// Also check automation config ID if available
		var autoID string
		if auto.Config != nil {
			autoID = auto.Config.ID
		}

		if !IsTestEntity(autoID) && !IsTestEntity(entityID) {
			continue
		}

		// Use the config ID if available, otherwise extract from entity_id
		idToDelete := autoID
		if idToDelete == "" {
			idToDelete = strings.TrimPrefix(entityID, "automation.")
		}

		if idToDelete != "" {
			if _, err := deleteAutomationWithRetry(ctx, client, idToDelete, entityID); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", idToDelete, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete automations: %s", strings.Join(errors, "; "))
	}

	return nil
}

// cleanupTestScripts removes all test scripts.
func cleanupTestScripts(ctx context.Context, client homeassistant.Client) error {
	scripts, err := client.ListScripts(ctx)
	if err != nil {
		return fmt.Errorf("failed to list scripts: %w", err)
	}

	var errors []string

	for _, script := range scripts {
		entityID := script.EntityID
		if entityID == "" {
			continue
		}

		if !IsTestEntity(entityID) {
			continue
		}

		// Script ID is the part after "script."
		scriptID := strings.TrimPrefix(entityID, "script.")
		if _, err := deleteScriptWithRetry(ctx, client, scriptID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", scriptID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete scripts: %s", strings.Join(errors, "; "))
	}

	return nil
}

// cleanupTestScenes removes all test scenes.
func cleanupTestScenes(ctx context.Context, client homeassistant.Client) error {
	scenes, err := client.ListScenes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list scenes: %w", err)
	}

	var errors []string

	for _, scene := range scenes {
		entityID := scene.EntityID
		if entityID == "" {
			continue
		}

		if !IsTestEntity(entityID) {
			continue
		}

		// Scene ID is the part after "scene."
		sceneID := strings.TrimPrefix(entityID, "scene.")
		if err := deleteSceneWithRetry(ctx, client, sceneID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", sceneID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete scenes: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deleteHelperWithRetry attempts to delete a helper with retry logic.
func deleteHelperWithRetry(ctx context.Context, client homeassistant.Client, entityID string) error {
	if err := ValidateTestEntityID(entityID); err != nil {
		return err
	}

	var lastErr error
	for i := range 3 {
		if err := client.DeleteHelper(ctx, entityID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}

	// Fallback: Config Entry groups return unknown_command for DeleteHelper.
	// Try RemoveEntityRegistryEntry to clean up the orphaned entity.
	if err := client.RemoveEntityRegistryEntry(ctx, entityID); err == nil {
		return nil
	} else if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
		// Entity already removed from registry, consider this a success
		return nil
	}

	return lastErr
}

// isNotFoundError reports whether err means the target does not exist. HA answers 400
// {"message":"Resource not found"} (not 404) for config ids missing from the config file, so
// a status-code-only check would miss it. Mirrors internal/handlers.isNotFoundError, which is
// unexported in another package.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *homeassistant.APIError
	if stderrors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

// deleteConfigWithRegistryFallback runs del with retries. When HA reports the config as not
// found, the entity can still be alive without a config behind it (see #305), so it is removed
// through the entity registry instead of retrying a delete that cannot succeed. viaRegistry is
// true when that fallback fired; each firing is logged so an orphaning bug stays visible
// instead of quietly turning the suite green.
func deleteConfigWithRegistryFallback(
	ctx context.Context,
	client homeassistant.Client,
	entityID string,
	del func(context.Context) error,
) (viaRegistry bool, err error) {
	var lastErr error
	for i := range 3 {
		delErr := del(ctx)
		if delErr == nil {
			return false, nil
		}
		if isNotFoundError(delErr) {
			return removeOrphanViaRegistry(ctx, client, entityID, delErr)
		}
		lastErr = delErr
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return false, lastErr
}

// removeOrphanViaRegistry removes entityID from the entity registry after the config delete
// failed with deleteErr. Only test entities are ever removed this way.
func removeOrphanViaRegistry(ctx context.Context, client homeassistant.Client, entityID string, deleteErr error) (bool, error) {
	if err := ValidateTestEntityID(entityID); err != nil {
		return false, fmt.Errorf("%w (registry fallback refused: %w)", deleteErr, err)
	}

	if regErr := client.RemoveEntityRegistryEntry(ctx, entityID); regErr != nil &&
		!isNotFoundError(regErr) && !strings.Contains(regErr.Error(), "does not exist") {
		return false, fmt.Errorf("%w (registry removal also failed: %w)", deleteErr, regErr)
	}

	log.Printf("WARNING: integration cleanup removed %s via the entity registry after the config delete reported not found; the test that created it left an orphan", entityID)
	return true, nil
}

// deleteAutomationWithRetry attempts to delete an automation with retry logic. entityID is the
// automation's real entity_id (it can differ from the config id) and is only used for the
// registry fallback; an empty entityID falls back to automation.<automationID>.
func deleteAutomationWithRetry(ctx context.Context, client homeassistant.Client, automationID, entityID string) (bool, error) {
	if err := ValidateTestEntityID(automationID); err != nil {
		return false, err
	}

	if entityID == "" {
		entityID = BuildEntityID("automation", automationID)
	}

	return deleteConfigWithRegistryFallback(ctx, client, entityID, func(ctx context.Context) error {
		return client.DeleteAutomation(ctx, automationID)
	})
}

// deleteScriptWithRetry attempts to delete a script with retry logic.
func deleteScriptWithRetry(ctx context.Context, client homeassistant.Client, scriptID string) (bool, error) {
	if err := ValidateTestEntityID(scriptID); err != nil {
		return false, err
	}

	return deleteConfigWithRegistryFallback(ctx, client, BuildEntityID("script", scriptID), func(ctx context.Context) error {
		return client.DeleteScript(ctx, scriptID)
	})
}

// deleteSceneWithRetry attempts to delete a scene with retry logic.
func deleteSceneWithRetry(ctx context.Context, client homeassistant.Client, sceneID string) error {
	if err := ValidateTestEntityID(sceneID); err != nil {
		return err
	}

	var lastErr error
	for i := range 3 {
		if err := client.DeleteScene(ctx, sceneID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// cleanupTestAreas removes all test areas.
func cleanupTestAreas(ctx context.Context, client homeassistant.Client) error {
	areas, err := client.GetAreaRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to get area registry: %w", err)
	}

	var errors []string

	for _, area := range areas {
		if !IsTestEntity(area.AreaID) {
			continue
		}

		if err := deleteAreaWithRetry(ctx, client, area.AreaID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", area.AreaID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete areas: %s", strings.Join(errors, "; "))
	}

	return nil
}

// cleanupTestDashboards removes all test dashboards.
func cleanupTestDashboards(ctx context.Context, client homeassistant.Client) error {
	dashboards, err := client.ListDashboards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list dashboards: %w", err)
	}

	var errors []string

	for _, dashboard := range dashboards {
		// Use URLPath for test identification (user-controlled), not dashboard ID (HA-generated)
		if !IsTestEntity(dashboard.URLPath) {
			continue
		}

		// Use dashboard ID for API delete calls
		if err := deleteDashboardWithRetry(ctx, client, dashboard.ID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", dashboard.URLPath, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete dashboards: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deleteAreaWithRetry attempts to delete an area with retry logic.
func deleteAreaWithRetry(ctx context.Context, client homeassistant.Client, areaID string) error {
	if err := ValidateTestEntityID(areaID); err != nil {
		return err
	}

	var lastErr error
	for i := range 3 {
		if err := client.DeleteArea(ctx, areaID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// deleteDashboardWithRetry attempts to delete a dashboard with retry logic.
func deleteDashboardWithRetry(ctx context.Context, client homeassistant.Client, dashboardID string) error {
	// Note: We don't validate dashboardID here because it's HA-generated (like "lovelace-xxxx")
	// The validation was done on URLPath in cleanupTestDashboards

	var lastErr error
	for i := range 3 {
		if err := client.DeleteDashboard(ctx, dashboardID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// CountTestEntities returns the number of test entities still present.
// Used for verification after cleanup.
func CountTestEntities(ctx context.Context, client homeassistant.Client) (int, []string, error) {
	states, err := client.GetStates(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get states: %w", err)
	}

	var testEntities []string
	for _, entity := range states {
		if IsTestEntity(entity.EntityID) {
			testEntities = append(testEntities, entity.EntityID)
		}
	}

	return len(testEntities), testEntities, nil
}

// CountTestAreas returns the number of test areas still present.
// Used for verification after cleanup.
func CountTestAreas(ctx context.Context, client homeassistant.Client) (int, []string, error) {
	areas, err := client.GetAreaRegistry(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get area registry: %w", err)
	}

	var testAreas []string
	for _, area := range areas {
		if IsTestEntity(area.AreaID) {
			testAreas = append(testAreas, area.AreaID)
		}
	}

	return len(testAreas), testAreas, nil
}

// CountTestDashboards returns the number of test dashboards still present.
// Used for verification after cleanup.
func CountTestDashboards(ctx context.Context, client homeassistant.Client) (int, []string, error) {
	dashboards, err := client.ListDashboards(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to list dashboards: %w", err)
	}

	var testDashboards []string
	for _, dashboard := range dashboards {
		if IsTestEntity(dashboard.URLPath) {
			testDashboards = append(testDashboards, dashboard.URLPath)
		}
	}

	return len(testDashboards), testDashboards, nil
}

// cleanupTestLabels removes all test labels.
func cleanupTestLabels(ctx context.Context, client homeassistant.Client) error {
	labels, err := client.GetLabelRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to get label registry: %w", err)
	}

	var errors []string

	for _, label := range labels {
		if !IsTestEntity(label.LabelID) {
			continue
		}

		if err := deleteLabelWithRetry(ctx, client, label.LabelID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", label.LabelID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete labels: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deleteLabelWithRetry attempts to delete a label with retry logic.
func deleteLabelWithRetry(ctx context.Context, client homeassistant.Client, labelID string) error {
	if err := ValidateTestEntityID(labelID); err != nil {
		return err
	}

	var lastErr error
	for i := range 3 {
		if err := client.DeleteLabel(ctx, labelID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// CountTestLabels returns the number of test labels still present.
// Used for verification after cleanup.
func CountTestLabels(ctx context.Context, client homeassistant.Client) (int, []string, error) {
	labels, err := client.GetLabelRegistry(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get label registry: %w", err)
	}

	var testLabels []string
	for _, label := range labels {
		if IsTestEntity(label.LabelID) {
			testLabels = append(testLabels, label.LabelID)
		}
	}

	return len(testLabels), testLabels, nil
}

// cleanupTestFloors removes all test floors.
func cleanupTestFloors(ctx context.Context, client homeassistant.Client) error {
	floors, err := client.GetFloorRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to get floor registry: %w", err)
	}

	var errors []string

	for _, floor := range floors {
		if !IsTestEntity(floor.FloorID) {
			continue
		}

		if err := deleteFloorWithRetry(ctx, client, floor.FloorID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", floor.FloorID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete floors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deleteFloorWithRetry attempts to delete a floor with retry logic.
func deleteFloorWithRetry(ctx context.Context, client homeassistant.Client, floorID string) error {
	if err := ValidateTestEntityID(floorID); err != nil {
		return err
	}

	var lastErr error
	for i := range 3 {
		if err := client.DeleteFloor(ctx, floorID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// CountTestFloors returns the number of test floors still present.
// Used for verification after cleanup.
func CountTestFloors(ctx context.Context, client homeassistant.Client) (int, []string, error) {
	floors, err := client.GetFloorRegistry(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get floor registry: %w", err)
	}

	var testFloors []string
	for _, floor := range floors {
		if IsTestEntity(floor.FloorID) {
			testFloors = append(testFloors, floor.FloorID)
		}
	}

	return len(testFloors), testFloors, nil
}

// cleanupTestTags removes all test tags.
func cleanupTestTags(ctx context.Context, client homeassistant.Client) error {
	tags, err := client.GetTags(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	var errors []string

	for _, tag := range tags {
		if !IsTestEntity(tag.TagID) {
			continue
		}

		if err := deleteTagWithRetry(ctx, client, tag.TagID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", tag.TagID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete tags: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deleteTagWithRetry attempts to delete a tag with retry logic.
func deleteTagWithRetry(ctx context.Context, client homeassistant.Client, tagID string) error {
	if err := ValidateTestEntityID(tagID); err != nil {
		return err
	}

	var lastErr error
	for i := range 3 {
		if err := client.DeleteTag(ctx, tagID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// CountTestTags returns the number of test tags still present.
// Used for verification after cleanup.
func CountTestTags(ctx context.Context, client homeassistant.Client) (int, []string, error) {
	tags, err := client.GetTags(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get tags: %w", err)
	}

	var testTags []string
	for _, tag := range tags {
		if IsTestEntity(tag.TagID) {
			testTags = append(testTags, tag.TagID)
		}
	}

	return len(testTags), testTags, nil
}

// cleanupTestZones removes all test zones.
func cleanupTestZones(ctx context.Context, client homeassistant.Client) error {
	zones, err := client.GetZones(ctx)
	if err != nil {
		return fmt.Errorf("failed to get zones: %w", err)
	}

	var errors []string

	for _, zone := range zones {
		if !IsTestEntity(zone.ID) {
			continue
		}

		if err := deleteZoneWithRetry(ctx, client, zone.ID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", zone.ID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete zones: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deleteZoneWithRetry attempts to delete a zone with retry logic.
func deleteZoneWithRetry(ctx context.Context, client homeassistant.Client, zoneID string) error {
	if err := ValidateTestEntityID(zoneID); err != nil {
		return err
	}

	var lastErr error
	for i := range 3 {
		if err := client.DeleteZone(ctx, zoneID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// CountTestZones returns the number of test zones still present.
// Used for verification after cleanup.
func CountTestZones(ctx context.Context, client homeassistant.Client) (int, []string, error) {
	zones, err := client.GetZones(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get zones: %w", err)
	}

	var testZones []string
	for _, zone := range zones {
		if IsTestEntity(zone.ID) {
			testZones = append(testZones, zone.ID)
		}
	}

	return len(testZones), testZones, nil
}

// cleanupTestPersons removes all test persons.
func cleanupTestPersons(ctx context.Context, client homeassistant.Client) error {
	persons, err := client.GetPersons(ctx)
	if err != nil {
		return fmt.Errorf("failed to get persons: %w", err)
	}

	var errors []string

	for _, person := range persons {
		if !IsTestEntity(person.ID) {
			continue
		}

		if err := deletePersonWithRetry(ctx, client, person.ID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", person.ID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete persons: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deletePersonWithRetry attempts to delete a person with retry logic.
func deletePersonWithRetry(ctx context.Context, client homeassistant.Client, personID string) error {
	if err := ValidateTestEntityID(personID); err != nil {
		return err
	}

	var lastErr error
	for i := range 3 {
		if err := client.DeletePerson(ctx, personID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// CountTestPersons returns the number of test persons still present.
// Used for verification after cleanup.
func CountTestPersons(ctx context.Context, client homeassistant.Client) (int, []string, error) {
	persons, err := client.GetPersons(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get persons: %w", err)
	}

	var testPersons []string
	for _, person := range persons {
		if IsTestEntity(person.ID) {
			testPersons = append(testPersons, person.ID)
		}
	}

	return len(testPersons), testPersons, nil
}
