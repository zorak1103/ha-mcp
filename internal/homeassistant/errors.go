package homeassistant

import (
	"fmt"
	"net/http"
)

// NewEntityNotFoundError returns the error for an entity that does not exist
// in Home Assistant's state machine.
func NewEntityNotFoundError(entityID string) error {
	return fmt.Errorf("entity not found: %s", entityID)
}

// NewConfigNotFoundError returns a 404 APIError for a stored configuration
// object (e.g. kind "automation", "script", "scene") that does not exist.
func NewConfigNotFoundError(kind, id string) error {
	return &APIError{
		StatusCode: http.StatusNotFound,
		Message:    fmt.Sprintf("%s not found: %s", kind, id),
	}
}
