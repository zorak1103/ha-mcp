package handlers

import (
	"strings"
	"testing"
)

func TestValidateEntityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		entityID    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid entity ID",
			entityID: "light.living_room",
			wantErr:  false,
		},
		{
			name:     "valid entity ID with numbers",
			entityID: "sensor.temperature_1",
			wantErr:  false,
		},
		{
			name:     "valid input_number entity",
			entityID: "input_number.target_temp",
			wantErr:  false,
		},
		{
			name:        "empty entity ID",
			entityID:    "",
			wantErr:     true,
			errContains: "required",
		},
		{
			name:        "missing dot separator",
			entityID:    "lightliving_room",
			wantErr:     true,
			errContains: "domain.object_id",
		},
		{
			name:        "uppercase characters",
			entityID:    "Light.Living_Room",
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:        "spaces in entity ID",
			entityID:    "light.living room",
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:        "special characters",
			entityID:    "light.living-room",
			wantErr:     true,
			errContains: "invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEntityID(tt.entityID)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateEntityIDWithPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		entityID         string
		expectedPlatform string
		wantErr          bool
		errContains      string
	}{
		{
			name:             "valid input_number entity",
			entityID:         "input_number.target_temp",
			expectedPlatform: "input_number",
			wantErr:          false,
		},
		{
			name:             "valid counter entity",
			entityID:         "counter.daily_count",
			expectedPlatform: "counter",
			wantErr:          false,
		},
		{
			name:             "wrong platform",
			entityID:         "input_boolean.guest_mode",
			expectedPlatform: "input_number",
			wantErr:          true,
			errContains:      "must be a input_number helper",
		},
		{
			name:             "invalid entity ID format",
			entityID:         "invalid",
			expectedPlatform: "input_number",
			wantErr:          true,
			errContains:      "domain.object_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEntityIDWithPlatform(tt.entityID, tt.expectedPlatform)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		min       float64
		max       float64
		fieldName string
		wantErr   bool
	}{
		{
			name:      "valid range min < max",
			min:       0,
			max:       100,
			fieldName: "input_number",
			wantErr:   false,
		},
		{
			name:      "valid range min = max",
			min:       50,
			max:       50,
			fieldName: "input_number",
			wantErr:   false,
		},
		{
			name:      "valid range with negative values",
			min:       -100,
			max:       100,
			fieldName: "temperature",
			wantErr:   false,
		},
		{
			name:      "invalid range min > max",
			min:       100,
			max:       0,
			fieldName: "input_number",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRange(tt.min, tt.max, tt.fieldName)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateValueInRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     float64
		min       float64
		max       float64
		fieldName string
		wantErr   bool
	}{
		{
			name:      "value in range",
			value:     50,
			min:       0,
			max:       100,
			fieldName: "temperature",
			wantErr:   false,
		},
		{
			name:      "value at min boundary",
			value:     0,
			min:       0,
			max:       100,
			fieldName: "temperature",
			wantErr:   false,
		},
		{
			name:      "value at max boundary",
			value:     100,
			min:       0,
			max:       100,
			fieldName: "temperature",
			wantErr:   false,
		},
		{
			name:      "value below min",
			value:     -1,
			min:       0,
			max:       100,
			fieldName: "temperature",
			wantErr:   true,
		},
		{
			name:      "value above max",
			value:     101,
			min:       0,
			max:       100,
			fieldName: "temperature",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateValueInRange(tt.value, tt.min, tt.max, tt.fieldName)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetRequiredString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    map[string]any
		key     string
		want    string
		wantErr bool
	}{
		{
			name:    "key present with value",
			args:    map[string]any{"name": "test"},
			key:     "name",
			want:    "test",
			wantErr: false,
		},
		{
			name:    "key missing",
			args:    map[string]any{"other": "value"},
			key:     "name",
			want:    "",
			wantErr: true,
		},
		{
			name:    "key present but empty",
			args:    map[string]any{"name": ""},
			key:     "name",
			want:    "",
			wantErr: true,
		},
		{
			name:    "key present but wrong type",
			args:    map[string]any{"name": 123},
			key:     "name",
			want:    "",
			wantErr: true,
		},
		{
			name:    "nil args",
			args:    nil,
			key:     "name",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetRequiredString(tt.args, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("got %q, want %q", got, tt.want)
				}
			}
		})
	}
}

func TestGetOptionalString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   map[string]any
		key    string
		want   string
		wantOK bool
	}{
		{
			name:   "key present with value",
			args:   map[string]any{"name": "test"},
			key:    "name",
			want:   "test",
			wantOK: true,
		},
		{
			name:   "key missing",
			args:   map[string]any{"other": "value"},
			key:    "name",
			want:   "",
			wantOK: false,
		},
		{
			name:   "key present but empty",
			args:   map[string]any{"name": ""},
			key:    "name",
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := GetOptionalString(tt.args, tt.key)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, wantOK = %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetOptionalFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   map[string]any
		key    string
		want   float64
		wantOK bool
	}{
		{
			name:   "key present with value",
			args:   map[string]any{"value": 42.5},
			key:    "value",
			want:   42.5,
			wantOK: true,
		},
		{
			name:   "key missing",
			args:   map[string]any{"other": 10.0},
			key:    "value",
			want:   0,
			wantOK: false,
		},
		{
			name:   "key present but wrong type",
			args:   map[string]any{"value": "not a number"},
			key:    "value",
			want:   0,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := GetOptionalFloat64(tt.args, tt.key)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, wantOK = %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOptionalInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   map[string]any
		key    string
		want   int
		wantOK bool
	}{
		{
			name:   "key present with integer value",
			args:   map[string]any{"count": float64(42)},
			key:    "count",
			want:   42,
			wantOK: true,
		},
		{
			name:   "key present with float truncated",
			args:   map[string]any{"count": 42.9},
			key:    "count",
			want:   42,
			wantOK: true,
		},
		{
			name:   "key missing",
			args:   map[string]any{"other": float64(10)},
			key:    "count",
			want:   0,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := GetOptionalInt(tt.args, tt.key)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, wantOK = %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOptionalBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   map[string]any
		key    string
		want   bool
		wantOK bool
	}{
		{
			name:   "key present with true",
			args:   map[string]any{"enabled": true},
			key:    "enabled",
			want:   true,
			wantOK: true,
		},
		{
			name:   "key present with false",
			args:   map[string]any{"enabled": false},
			key:    "enabled",
			want:   false,
			wantOK: true,
		},
		{
			name:   "key missing",
			args:   map[string]any{"other": true},
			key:    "enabled",
			want:   false,
			wantOK: false,
		},
		{
			name:   "key present but wrong type",
			args:   map[string]any{"enabled": "true"},
			key:    "enabled",
			want:   false,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := GetOptionalBool(tt.args, tt.key)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, wantOK = %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
