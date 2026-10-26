package homeassistant

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewConfigNotFoundError_Is404(t *testing.T) {
	err := NewConfigNotFoundError("automation", "x")

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
	if apiErr.Message != "automation not found: x" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "automation not found: x")
	}
}

func TestNewEntityNotFoundError_Text(t *testing.T) {
	err := NewEntityNotFoundError("light.kitchen")
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "entity not found: light.kitchen"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Error("entity not-found error must be a plain error, not *APIError")
	}
}
