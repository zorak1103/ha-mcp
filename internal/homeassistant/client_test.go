package homeassistant

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAPIError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        APIError
		wantString string
	}{
		{
			name: "standard error",
			err: APIError{
				StatusCode: 404,
				Message:    "Entity not found",
			},
			wantString: "Home Assistant API error (status 404): Entity not found",
		},
		{
			name: "unauthorized error",
			err: APIError{
				StatusCode: 401,
				Message:    "Invalid access token",
			},
			wantString: "Home Assistant API error (status 401): Invalid access token",
		},
		{
			name: "internal server error",
			err: APIError{
				StatusCode: 500,
				Message:    "Internal server error",
			},
			wantString: "Home Assistant API error (status 500): Internal server error",
		},
		{
			name: "empty message",
			err: APIError{
				StatusCode: 400,
				Message:    "",
			},
			wantString: "Home Assistant API error (status 400): ",
		},
		{
			name: "zero status code",
			err: APIError{
				StatusCode: 0,
				Message:    "Unknown error",
			},
			wantString: "Home Assistant API error (status 0): Unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.err.Error()
			if got != tt.wantString {
				t.Errorf("APIError.Error() = %q, want %q", got, tt.wantString)
			}
		})
	}
}

func TestAPIError_ImplementsError(t *testing.T) {
	t.Parallel()

	// Verify APIError implements the error interface
	var _ error = &APIError{}
}

func TestGetStringAttr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		attrs map[string]any
		key   string
		want  string
	}{
		{
			name: "existing string value",
			attrs: map[string]any{
				"friendly_name": "Living Room Light",
				"icon":          "mdi:lightbulb",
			},
			key:  "friendly_name",
			want: "Living Room Light",
		},
		{
			name: "key not found",
			attrs: map[string]any{
				"friendly_name": "Living Room Light",
			},
			key:  "icon",
			want: "",
		},
		{
			name: "value is not string (int)",
			attrs: map[string]any{
				"brightness": 255,
			},
			key:  "brightness",
			want: "",
		},
		{
			name: "value is not string (bool)",
			attrs: map[string]any{
				"is_on": true,
			},
			key:  "is_on",
			want: "",
		},
		{
			name: "value is not string (float)",
			attrs: map[string]any{
				"temperature": 23.5,
			},
			key:  "temperature",
			want: "",
		},
		{
			name: "value is not string (nil)",
			attrs: map[string]any{
				"optional": nil,
			},
			key:  "optional",
			want: "",
		},
		{
			name: "value is not string (slice)",
			attrs: map[string]any{
				"options": []string{"a", "b", "c"},
			},
			key:  "options",
			want: "",
		},
		{
			name:  "nil map",
			attrs: nil,
			key:   "any_key",
			want:  "",
		},
		{
			name:  "empty map",
			attrs: map[string]any{},
			key:   "any_key",
			want:  "",
		},
		{
			name: "empty string value",
			attrs: map[string]any{
				"empty": "",
			},
			key:  "empty",
			want: "",
		},
		{
			name: "string with special characters",
			attrs: map[string]any{
				"name": "Test äöü 日本語 emoji 🏠",
			},
			key:  "name",
			want: "Test äöü 日本語 emoji 🏠",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := getStringAttr(tt.attrs, tt.key)
			if got != tt.want {
				t.Errorf("getStringAttr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		entityID string
		want     string
	}{
		{
			name:     "input_boolean",
			entityID: "input_boolean.my_switch",
			want:     "input_boolean",
		},
		{
			name:     "input_number",
			entityID: "input_number.temperature_setpoint",
			want:     "input_number",
		},
		{
			name:     "input_text",
			entityID: "input_text.user_name",
			want:     "input_text",
		},
		{
			name:     "input_select",
			entityID: "input_select.hvac_mode",
			want:     "input_select",
		},
		{
			name:     "input_datetime",
			entityID: "input_datetime.alarm_time",
			want:     "input_datetime",
		},
		{
			name:     "non-helper entity (light)",
			entityID: "light.living_room",
			want:     "",
		},
		{
			name:     "non-helper entity (switch)",
			entityID: "switch.kitchen_fan",
			want:     "",
		},
		{
			name:     "non-helper entity (sensor)",
			entityID: "sensor.temperature",
			want:     "",
		},
		{
			name:     "non-helper entity (automation)",
			entityID: "automation.morning_routine",
			want:     "",
		},
		{
			name:     "empty entity ID",
			entityID: "",
			want:     "",
		},
		{
			name:     "only prefix without dot",
			entityID: "input_boolean",
			want:     "",
		},
		{
			name:     "only prefix with dot",
			entityID: "input_boolean.",
			want:     "",
		},
		{
			name:     "input_boolean with complex ID",
			entityID: "input_boolean.my_complex_switch_123",
			want:     "input_boolean",
		},
		{
			name:     "similar but not matching prefix",
			entityID: "input_bool.something",
			want:     "",
		},
		{
			name:     "partial match at start",
			entityID: "input.something",
			want:     "",
		},
		{
			name:     "counter helper (not in list)",
			entityID: "counter.my_counter",
			want:     "",
		},
		{
			name:     "timer helper (not in list)",
			entityID: "timer.my_timer",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractPlatform(tt.entityID)
			if got != tt.want {
				t.Errorf("extractPlatform(%q) = %q, want %q", tt.entityID, got, tt.want)
			}
		})
	}
}

func TestServiceConstants(t *testing.T) {
	t.Parallel()

	// Verify service constants have expected values
	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{
			name:     "serviceTurnOn",
			constant: serviceTurnOn,
			want:     "turn_on",
		},
		{
			name:     "serviceTurnOff",
			constant: serviceTurnOff,
			want:     "turn_off",
		},
		{
			name:     "serviceSetValue",
			constant: serviceSetValue,
			want:     "set_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.constant != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.want)
			}
		})
	}
}

func TestAPIError_IsError(t *testing.T) {
	t.Parallel()

	// Test that APIError can be used as an error value
	err := &APIError{
		StatusCode: 404,
		Message:    "Not found",
	}

	// Use errors.As to check for specific error type (errorlint compliant)
	var e error = err
	var apiErr *APIError
	if !errors.As(e, &apiErr) {
		t.Error("APIError should be extractable via errors.As")
	}

	if diff := cmp.Diff(err, apiErr); diff != "" {
		t.Errorf("Type assertion result mismatch (-want +got):\n%s", diff)
	}
}
