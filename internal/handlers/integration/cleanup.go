//go:build integration

package integration

import (
	"context"
	"fmt"
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
			if err := deleteAutomationWithRetry(ctx, client, idToDelete); err != nil {
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
		if err := deleteScriptWithRetry(ctx, client, scriptID); err != nil {
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
	for i := 0; i < 3; i++ {
		if err := client.DeleteHelper(ctx, entityID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// deleteAutomationWithRetry attempts to delete an automation with retry logic.
func deleteAutomationWithRetry(ctx context.Context, client homeassistant.Client, automationID string) error {
	if err := ValidateTestEntityID(automationID); err != nil {
		return err
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		if err := client.DeleteAutomation(ctx, automationID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// deleteScriptWithRetry attempts to delete a script with retry logic.
func deleteScriptWithRetry(ctx context.Context, client homeassistant.Client, scriptID string) error {
	if err := ValidateTestEntityID(scriptID); err != nil {
		return err
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		if err := client.DeleteScript(ctx, scriptID); err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

// deleteSceneWithRetry attempts to delete a scene with retry logic.
func deleteSceneWithRetry(ctx context.Context, client homeassistant.Client, sceneID string) error {
	if err := ValidateTestEntityID(sceneID); err != nil {
		return err
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		if err := client.DeleteScene(ctx, sceneID); err != nil {
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
