package enforce

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// The functions below build the error the real client returns when a single
// lookup finds no such object. A hidden object must answer with exactly this
// error, so a caller cannot tell hidden from missing by text, type, wrapped
// error or HTTP status.
//
// Shapes the client builds itself (entity state, stored scene config, helper
// storage) come from the homeassistant constructors. The others are Home
// Assistant's own error for the missing object, wrapped by the client with a
// fixed prefix: those texts mirror what HA answers over WebSocket/REST and
// have to be kept in step with the client methods named on each function.

// wsCommandError mirrors the error WSClient returns for a failed command.
func wsCommandError(code, message string) error {
	return errors.New("command failed: " + code + " - " + message)
}

// errEntityMissing is GetState's error for an unknown entity.
func errEntityMissing(entityID string) error {
	return homeassistant.NewEntityNotFoundError(entityID)
}

// errRegistryEntryMissing is GetEntityRegistryEntry's error for an unknown entity.
func errRegistryEntryMissing() error {
	return fmt.Errorf("get entity registry entry failed: %w", wsCommandError("not_found", "Entity not found"))
}

// errAutomationMissing is GetAutomation's error for an unknown automation.
func errAutomationMissing() error {
	return fmt.Errorf("get automation failed: %w", wsCommandError("not_found", "Entity not found"))
}

// errScriptMissing is GetScript's error for an unknown script: it looks the
// state up first, so the entity id (with prefix) appears in the text.
func errScriptMissing(entityID string) error {
	return fmt.Errorf("get script state failed: %w", homeassistant.NewEntityNotFoundError(entityID))
}

// errSceneMissing is GetScene's error for an unknown scene config id.
func errSceneMissing(configID string) error {
	return homeassistant.NewConfigNotFoundError("scene", configID)
}

// errLovelaceMissing is GetLovelaceConfig's error for an unknown dashboard.
// HA names the url_path it did not find; the default dashboard has none.
func errLovelaceMissing(urlPath string) error {
	if urlPath == "" {
		urlPath = "None"
	}
	return fmt.Errorf("get lovelace config failed: %w",
		wsCommandError("config_not_found", "Unknown config specified: "+urlPath))
}

// errLovelaceSaveMissing is SaveLovelaceConfig's error for an unknown
// dashboard: the same HA answer as for a read (errLovelaceMissing), wrapped by
// the save method. LIVE-CHECK.
func errLovelaceSaveMissing(urlPath string) error {
	if urlPath == "" {
		urlPath = "None"
	}
	return fmt.Errorf("save lovelace config failed: %w",
		wsCommandError("config_not_found", "Unknown config specified: "+urlPath))
}

// errDashboardMissing is UpdateDashboard's ("update") or DeleteDashboard's
// ("delete") error for an unknown dashboard id: HA's storage collection
// answers not_found "Unable to find dashboard_id <id>". LIVE-CHECK.
func errDashboardMissing(op, dashboardID string) error {
	return fmt.Errorf("%s dashboard failed: %w",
		op, wsCommandError("not_found", "Unable to find dashboard_id "+dashboardID))
}

// errRegistryUpdateMissing is UpdateEntityRegistryEntry's error for an
// entity not in the registry (HA: not_found "Entity not found"). LIVE-CHECK.
func errRegistryUpdateMissing() error {
	return fmt.Errorf("failed to update entity: %w", wsCommandError("not_found", "Entity not found"))
}

// errRegistryRemoveMissing is RemoveEntityRegistryEntry's error for an
// entity not in the registry. LIVE-CHECK.
func errRegistryRemoveMissing() error {
	return fmt.Errorf("remove entity registry entry failed: %w", wsCommandError("not_found", "Entity not found"))
}

// errConfigEntryMissing is GetConfigEntry's error for an unknown entry id
// (HA: not_found "Config entry not found"). LIVE-CHECK.
func errConfigEntryMissing() error {
	return fmt.Errorf("get config entry failed: %w", wsCommandError("not_found", "Config entry not found"))
}

// errHelperMissing is the answer for a helper update or delete whose entity is
// hidden or, under a hiding policy, does not exist: the client's own
// missing-helper error for the entity's domain, so it depends on the id alone.
func errHelperMissing(entityID string) error {
	domain, _, _ := strings.Cut(entityID, ".")
	return homeassistant.NewHelperConfigMissingError(domain, entityID)
}

// errCalendarMissing is GetCalendarEvents' error for an unknown calendar: HA
// answers 400 with an empty body.
func errCalendarMissing() error {
	return &homeassistant.APIError{StatusCode: http.StatusBadRequest, Message: "failed to get calendar events: "}
}

// errCameraSnapshotMissing is GetCameraSnapshot's error for an unknown camera.
func errCameraSnapshotMissing() error {
	return &homeassistant.APIError{StatusCode: http.StatusNotFound, Message: "failed to get camera snapshot: 404: Not Found"}
}

// errCameraStreamMissing is GetCameraStream's error for an unknown camera.
func errCameraStreamMissing() error {
	return fmt.Errorf("get camera stream failed: %w", wsCommandError("start_stream_failed", "Camera not found"))
}

// errMediaItemMissing is BrowseMedia's error for an entity-backed media item
// of a hidden entity. HA's camera source raises BrowseError("Unknown item")
// for any media-source://camera/<id>, whether or not the camera exists, so
// that is also the answer for a camera that does exist but is hidden.
// LIVE-CHECK: verify the text, the error code and, for the image source, that
// it answers the same way.
func errMediaItemMissing() error {
	return fmt.Errorf("browse media failed: %w", wsCommandError("browse_media_failed", "Unknown item"))
}
