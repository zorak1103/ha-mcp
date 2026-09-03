package homeassistant

import "testing"

func TestDetermineTemplateSubtype(t *testing.T) {
	t.Parallel()

	c := NewHybridClientWithInterfaces(&mockWSOperations{}, &mockRESTOperations{})

	tests := []struct {
		name   string
		config map[string]any
		want   string
	}{
		{
			name:   "explicit type field wins",
			config: map[string]any{"type": "binary_sensor"},
			want:   "binary_sensor",
		},
		{
			name:   "template_type field is honored (issue #211)",
			config: map[string]any{"template_type": "binary_sensor"},
			want:   "binary_sensor",
		},
		{
			name:   "template_type wins over a non-binary device_class",
			config: map[string]any{"template_type": "binary_sensor", "device_class": "temperature"},
			want:   "binary_sensor",
		},
		{
			name:   "template_type sensor is honored even with a binary device_class",
			config: map[string]any{"template_type": "sensor", "device_class": "temperature"},
			want:   "sensor",
		},
		{
			name:   "falls back to device_class inference when neither type key is set",
			config: map[string]any{"device_class": "motion"},
			want:   "binary_sensor",
		},
		{
			name:   "falls back to sensor when nothing indicates otherwise",
			config: map[string]any{},
			want:   "sensor",
		},
		{
			name:   "explicit type still takes precedence over template_type",
			config: map[string]any{"type": "binary_sensor", "template_type": "sensor"},
			want:   "binary_sensor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := c.determineTemplateSubtype(HelperConfig{Platform: platformTemplate, Config: tt.config})
			if got != tt.want {
				t.Errorf("determineTemplateSubtype(%v) = %q, want %q", tt.config, got, tt.want)
			}
		})
	}
}

func TestPredictEntityIDForConfigEntry_TemplateBinarySensor(t *testing.T) {
	t.Parallel()

	c := NewHybridClientWithInterfaces(&mockWSOperations{}, &mockRESTOperations{})

	// A template_binary_sensor helper (created via manage_helper, which writes
	// template_type, not device_class-inferrable info) must predict a
	// binary_sensor.* entity id, matching what createConfigEntryHelper reports
	// to the caller and what HA actually creates. Regression test for #211's
	// second victim: the icon-write target derived here shares the same
	// determineTemplateSubtype call as the entity-id report.
	got, err := c.predictEntityIDForConfigEntry(t.Context(), HelperConfig{
		Platform: platformTemplate,
		Config: map[string]any{
			"name":          "My Door Sensor",
			"template_type": "binary_sensor",
		},
	})
	if err != nil {
		t.Fatalf("predictEntityIDForConfigEntry() error = %v", err)
	}
	want := "binary_sensor.my_door_sensor"
	if got != want {
		t.Errorf("predictEntityIDForConfigEntry() = %q, want %q", got, want)
	}
}
