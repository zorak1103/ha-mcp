package homeassistant

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestFlexibleString_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    FlexibleString
		wantErr bool
	}{
		{
			name:    "simple string",
			input:   `"2024.1.0"`,
			want:    FlexibleString("2024.1.0"),
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   `""`,
			want:    FlexibleString(""),
			wantErr: false,
		},
		{
			name:    "string array single element",
			input:   `["1.0.0"]`,
			want:    FlexibleString("1.0.0"),
			wantErr: false,
		},
		{
			name:    "string array multiple elements",
			input:   `["1.0", "2.0", "3.0"]`,
			want:    FlexibleString("1.0, 2.0, 3.0"),
			wantErr: false,
		},
		{
			name:    "empty array",
			input:   `[]`,
			want:    FlexibleString(""),
			wantErr: false,
		},
		{
			name:    "null value",
			input:   `null`,
			want:    FlexibleString(""),
			wantErr: false,
		},
		{
			name:    "number value",
			input:   `123`,
			want:    FlexibleString(""),
			wantErr: false,
		},
		{
			name:    "boolean value",
			input:   `true`,
			want:    FlexibleString(""),
			wantErr: false,
		},
		{
			name:    "object value",
			input:   `{"key": "value"}`,
			want:    FlexibleString(""),
			wantErr: false,
		},
		{
			name:    "string with spaces",
			input:   `"version 1.0.0 beta"`,
			want:    FlexibleString("version 1.0.0 beta"),
			wantErr: false,
		},
		{
			name:    "string with unicode",
			input:   `"版本 1.0"`,
			want:    FlexibleString("版本 1.0"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got FlexibleString
			err := json.Unmarshal([]byte(tt.input), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("UnmarshalJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlexibleString_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fs   FlexibleString
		want string
	}{
		{
			name: "normal string",
			fs:   FlexibleString("test"),
			want: "test",
		},
		{
			name: "empty string",
			fs:   FlexibleString(""),
			want: "",
		},
		{
			name: "string with special chars",
			fs:   FlexibleString("test äöü"),
			want: "test äöü",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.fs.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlexibleString_MarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fs      FlexibleString
		want    string
		wantErr bool
	}{
		{
			name:    "normal string",
			fs:      FlexibleString("test"),
			want:    `"test"`,
			wantErr: false,
		},
		{
			name:    "empty string",
			fs:      FlexibleString(""),
			want:    `""`,
			wantErr: false,
		},
		{
			name:    "string with quotes",
			fs:      FlexibleString(`test "quoted"`),
			want:    `"test \"quoted\""`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.fs.MarshalJSON()

			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if string(got) != tt.want {
				t.Errorf("MarshalJSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFlexibleString_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fs   FlexibleString
	}{
		{
			name: "simple string",
			fs:   FlexibleString("2024.1.0"),
		},
		{
			name: "empty string",
			fs:   FlexibleString(""),
		},
		{
			name: "complex string",
			fs:   FlexibleString("test, version, 1.0.0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.fs)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var got FlexibleString
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if got != tt.fs {
				t.Errorf("Roundtrip mismatch: got %q, want %q", got, tt.fs)
			}
		})
	}
}

func TestFlexibleIdentifier_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    FlexibleIdentifier
		wantErr bool
	}{
		{
			name:    "string identifier",
			input:   `"device_123"`,
			want:    FlexibleIdentifier("device_123"),
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   `""`,
			want:    FlexibleIdentifier(""),
			wantErr: false,
		},
		{
			name:    "integer number",
			input:   `12345`,
			want:    FlexibleIdentifier("12345"),
			wantErr: false,
		},
		{
			name:    "float number",
			input:   `123.45`,
			want:    FlexibleIdentifier("123.45"),
			wantErr: false,
		},
		{
			name:    "zero",
			input:   `0`,
			want:    FlexibleIdentifier("0"),
			wantErr: false,
		},
		{
			name:    "negative number",
			input:   `-123`,
			want:    FlexibleIdentifier("-123"),
			wantErr: false,
		},
		{
			name:    "large number",
			input:   `9223372036854775807`,
			want:    FlexibleIdentifier("9223372036854775807"),
			wantErr: false,
		},
		{
			name:    "null value",
			input:   `null`,
			want:    FlexibleIdentifier(""),
			wantErr: false,
		},
		{
			name:    "boolean value",
			input:   `true`,
			want:    FlexibleIdentifier(""),
			wantErr: false,
		},
		{
			name:    "array value",
			input:   `["a", "b"]`,
			want:    FlexibleIdentifier(""),
			wantErr: false,
		},
		{
			name:    "object value",
			input:   `{"id": 123}`,
			want:    FlexibleIdentifier(""),
			wantErr: false,
		},
		{
			name:    "string with numbers",
			input:   `"abc123def"`,
			want:    FlexibleIdentifier("abc123def"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got FlexibleIdentifier
			err := json.Unmarshal([]byte(tt.input), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("UnmarshalJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlexibleIdentifier_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fi   FlexibleIdentifier
		want string
	}{
		{
			name: "normal string",
			fi:   FlexibleIdentifier("test_id"),
			want: "test_id",
		},
		{
			name: "empty string",
			fi:   FlexibleIdentifier(""),
			want: "",
		},
		{
			name: "numeric string",
			fi:   FlexibleIdentifier("12345"),
			want: "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.fi.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlexibleIdentifier_MarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fi      FlexibleIdentifier
		want    string
		wantErr bool
	}{
		{
			name:    "normal string",
			fi:      FlexibleIdentifier("test_id"),
			want:    `"test_id"`,
			wantErr: false,
		},
		{
			name:    "empty string",
			fi:      FlexibleIdentifier(""),
			want:    `""`,
			wantErr: false,
		},
		{
			name:    "numeric string",
			fi:      FlexibleIdentifier("12345"),
			want:    `"12345"`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.fi.MarshalJSON()

			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if string(got) != tt.want {
				t.Errorf("MarshalJSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFlexibleIdentifier_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fi   FlexibleIdentifier
	}{
		{
			name: "simple string",
			fi:   FlexibleIdentifier("device_abc"),
		},
		{
			name: "empty string",
			fi:   FlexibleIdentifier(""),
		},
		{
			name: "numeric string",
			fi:   FlexibleIdentifier("12345"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.fi)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var got FlexibleIdentifier
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if got != tt.fi {
				t.Errorf("Roundtrip mismatch: got %q, want %q", got, tt.fi)
			}
		})
	}
}

func TestHistoryEntry_LastChangedTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry HistoryEntry
		want  time.Time
	}{
		{
			name: "unix timestamp in seconds",
			entry: HistoryEntry{
				LastChanged: 1704067200, // 2024-01-01 00:00:00 UTC
			},
			want: time.Unix(1704067200, 0),
		},
		{
			name: "unix timestamp in milliseconds",
			entry: HistoryEntry{
				LastChanged: 1704067200000, // ms timestamp
			},
			want: time.UnixMilli(1704067200000),
		},
		{
			name: "zero LastChanged with LastUpdated fallback",
			entry: HistoryEntry{
				LastChanged: 0,
				LastUpdated: 1704067200,
			},
			want: time.Unix(1704067200, 0),
		},
		{
			name: "zero LastChanged with LastUpdated in ms",
			entry: HistoryEntry{
				LastChanged: 0,
				LastUpdated: 1704067200000,
			},
			want: time.UnixMilli(1704067200000),
		},
		{
			name: "both zero",
			entry: HistoryEntry{
				LastChanged: 0,
				LastUpdated: 0,
			},
			want: time.Unix(0, 0),
		},
		{
			name: "LastChanged takes precedence over LastUpdated",
			entry: HistoryEntry{
				LastChanged: 1704067200,
				LastUpdated: 1704153600,
			},
			want: time.Unix(1704067200, 0),
		},
		{
			name: "fractional seconds",
			entry: HistoryEntry{
				LastChanged: 1704067200.5,
			},
			want: time.Unix(1704067200, 0), // truncates to seconds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.entry.LastChangedTime()
			if !got.Equal(tt.want) {
				t.Errorf("LastChangedTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntity_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	entity := Entity{
		EntityID: "light.living_room",
		State:    "on",
		Attributes: map[string]any{
			"friendly_name": "Living Room Light",
			"brightness":    float64(255),
		},
		LastChanged: now,
		LastUpdated: now,
		Context: Context{
			ID:       "context_123",
			ParentID: "parent_456",
			UserID:   "user_789",
		},
	}

	data, err := json.Marshal(entity)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got Entity
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Compare fields individually due to time comparison complexity
	if got.EntityID != entity.EntityID {
		t.Errorf("EntityID mismatch: got %q, want %q", got.EntityID, entity.EntityID)
	}
	if got.State != entity.State {
		t.Errorf("State mismatch: got %q, want %q", got.State, entity.State)
	}
	if got.Context.ID != entity.Context.ID {
		t.Errorf("Context.ID mismatch: got %q, want %q", got.Context.ID, entity.Context.ID)
	}
}

func TestStateUpdate_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	update := StateUpdate{
		State: "on",
		Attributes: map[string]any{
			"brightness": float64(200),
			"color":      "red",
		},
	}

	data, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got StateUpdate
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(update, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestHistoryEntry_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	entry := HistoryEntry{
		EntityID: "sensor.temperature",
		State:    "23.5",
		Attributes: map[string]any{
			"unit_of_measurement": "°C",
		},
		LastChanged: 1704067200,
		LastUpdated: 1704067200,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got HistoryEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(entry, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestAutomation_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	automation := Automation{
		EntityID:      "automation.morning_lights",
		State:         "on",
		FriendlyName:  "Morning Lights",
		LastTriggered: "2024-01-01T08:00:00+00:00",
		Config: &AutomationConfig{
			ID:          "morning_lights",
			Alias:       "Morning Lights",
			Description: "Turn on lights in the morning",
			Mode:        "single",
			Triggers:    []any{map[string]any{"platform": "time", "at": "08:00:00"}},
			Conditions:  []any{map[string]any{"condition": "state", "entity_id": "input_boolean.home_mode", "state": "on"}},
			Actions:     []any{map[string]any{"service": "light.turn_on", "target": map[string]any{"entity_id": "light.living_room"}}},
		},
	}

	data, err := json.Marshal(automation)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got Automation
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(automation, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestAutomationConfig_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	config := AutomationConfig{
		ID:          "test_automation",
		Alias:       "Test Automation",
		Description: "A test automation",
		Mode:        "queued",
		Triggers:    []any{map[string]any{"platform": "state"}},
		Conditions:  []any{map[string]any{"condition": "time"}},
		Actions:     []any{map[string]any{"service": "notify.notify"}},
		Variables:   map[string]any{"var1": "value1"},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got AutomationConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(config, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestHelperConfig_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	config := HelperConfig{
		Platform: "input_boolean",
		ID:       "my_switch",
		Config: map[string]any{
			"name":    "My Switch",
			"icon":    "mdi:lightbulb",
			"initial": true,
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got HelperConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(config, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestScriptConfig_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	config := ScriptConfig{
		Alias:       "My Script",
		Description: "A test script",
		Mode:        "parallel",
		Icon:        "mdi:script",
		Fields: map[string]any{
			"brightness": map[string]any{
				"description": "Brightness level",
				"example":     float64(100),
			},
		},
		Variables: map[string]any{
			"delay": float64(5),
		},
		Sequence: []any{
			map[string]any{"service": "light.turn_on"},
			map[string]any{"delay": map[string]any{"seconds": float64(5)}},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got ScriptConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(config, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestSceneConfig_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	config := SceneConfig{
		Name: "Movie Night",
		Icon: "mdi:movie",
		Entities: map[string]SceneState{
			"light.living_room": {
				State: "on",
				Attributes: map[string]any{
					"brightness": float64(50),
				},
			},
			"switch.tv": {
				State: "on",
			},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got SceneConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(config, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestEntityRegistryEntry_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	entry := EntityRegistryEntry{
		EntityID:      "light.living_room",
		Platform:      "hue",
		ConfigEntryID: "config_123",
		DeviceID:      "device_456",
		AreaID:        "living_room",
		DisabledBy:    "",
		HiddenBy:      "",
		Name:          "Living Room Light",
		Icon:          "mdi:lightbulb",
		UniqueID:      "hue_light_001",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got EntityRegistryEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(entry, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestDeviceRegistryEntry_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	entry := DeviceRegistryEntry{
		ID:            "device_123",
		ConfigEntries: []string{"config_1", "config_2"},
		Connections:   [][]FlexibleIdentifier{{"mac", "00:11:22:33:44:55"}},
		Identifiers:   [][]FlexibleIdentifier{{"hue", "bridge_001"}},
		Manufacturer:  "Philips",
		Model:         FlexibleString("Hue Bridge"),
		Name:          "Living Room Hub",
		SWVersion:     FlexibleString("1.55.0"),
		HWVersion:     FlexibleString("BSB002"),
		AreaID:        "living_room",
		NameByUser:    "My Hub",
		DisabledBy:    "",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got DeviceRegistryEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(entry, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestAreaRegistryEntry_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	entry := AreaRegistryEntry{
		AreaID:  "living_room",
		Name:    "Living Room",
		Picture: "/local/images/living_room.jpg",
		Aliases: []string{"Wohnzimmer", "Salon"},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got AreaRegistryEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(entry, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestStreamInfo_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	info := StreamInfo{
		URL: "http://homeassistant.local:8123/api/hls/abc123/master_playlist.m3u8",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got StreamInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(info, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestMediaBrowseResult_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	result := MediaBrowseResult{
		Title:            "Music Library",
		MediaClass:       "directory",
		MediaContentID:   "media-source://media_source/local/music",
		MediaContentType: "music",
		CanPlay:          false,
		CanExpand:        true,
		Thumbnail:        "/local/images/music.png",
		Children: []*MediaBrowseResult{
			{
				Title:            "Album 1",
				MediaClass:       "album",
				MediaContentID:   "media-source://media_source/local/music/album1",
				MediaContentType: "music",
				CanPlay:          true,
				CanExpand:        true,
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got MediaBrowseResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(result, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestStatisticsResult_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	mean := 23.5
	minStatVal := 20.0
	maxStatVal := 27.0
	sum := 1000.0
	state := 25.0
	change := 5.0

	result := StatisticsResult{
		StatisticID: "sensor.temperature",
		Start:       1704067200,
		End:         1704153600,
		Mean:        &mean,
		Min:         &minStatVal,
		Max:         &maxStatVal,
		Sum:         &sum,
		State:       &state,
		Change:      &change,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got StatisticsResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(result, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestStatisticsResult_NilPointers(t *testing.T) {
	t.Parallel()

	result := StatisticsResult{
		StatisticID: "sensor.temperature",
		Start:       1704067200,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got StatisticsResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.Mean != nil {
		t.Errorf("Mean should be nil, got %v", *got.Mean)
	}
	if got.Min != nil {
		t.Errorf("Min should be nil, got %v", *got.Min)
	}
	if got.Max != nil {
		t.Errorf("Max should be nil, got %v", *got.Max)
	}
}

func TestTarget_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target Target
	}{
		{
			name: "entity only",
			target: Target{
				EntityID: []string{"light.living_room", "light.bedroom"},
			},
		},
		{
			name: "device only",
			target: Target{
				DeviceID: []string{"device_123"},
			},
		},
		{
			name: "area only",
			target: Target{
				AreaID: []string{"living_room"},
			},
		},
		{
			name: "label only",
			target: Target{
				LabelID: []string{"downstairs"},
			},
		},
		{
			name: "all targets",
			target: Target{
				EntityID: []string{"light.a"},
				DeviceID: []string{"dev_1"},
				AreaID:   []string{"area_1"},
				LabelID:  []string{"label_1"},
			},
		},
		{
			name:   "empty target",
			target: Target{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.target)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var got Target
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if diff := cmp.Diff(tt.target, got); diff != "" {
				t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExtractFromTargetResult_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	result := ExtractFromTargetResult{
		ReferencedEntities: []string{"light.a", "light.b"},
		ReferencedDevices:  []string{"device_1"},
		ReferencedAreas:    []string{"living_room"},
		MissingDevices:     []string{"missing_device"},
		MissingAreas:       []string{"missing_area"},
		MissingFloors:      []string{"missing_floor"},
		MissingLabels:      []string{"missing_label"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got ExtractFromTargetResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if diff := cmp.Diff(result, got); diff != "" {
		t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
	}
}

func TestContext_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  Context
	}{
		{
			name: "full context",
			ctx: Context{
				ID:       "ctx_123",
				ParentID: "parent_456",
				UserID:   "user_789",
			},
		},
		{
			name: "minimal context",
			ctx: Context{
				ID: "ctx_abc",
			},
		},
		{
			name: "empty context",
			ctx:  Context{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.ctx)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var got Context
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if diff := cmp.Diff(tt.ctx, got); diff != "" {
				t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSceneState_UnmarshalJSON_FlatFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantState string
		wantAttrs map[string]any
		wantErr   bool
	}{
		{
			name:      "flat format with state and attributes",
			input:     `{"state": "on", "brightness": 255, "color_temp": 300}`,
			wantState: "on",
			wantAttrs: map[string]any{"brightness": float64(255), "color_temp": float64(300)},
		},
		{
			name:      "flat format state only",
			input:     `{"state": "off"}`,
			wantState: "off",
			wantAttrs: nil,
		},
		{
			name:      "empty object",
			input:     `{}`,
			wantState: "",
			wantAttrs: nil,
		},
		{
			name:      "only attributes no state",
			input:     `{"brightness": 128}`,
			wantState: "",
			wantAttrs: map[string]any{"brightness": float64(128)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got SceneState
			err := json.Unmarshal([]byte(tt.input), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if diff := cmp.Diff(tt.wantAttrs, got.Attributes); diff != "" {
				t.Errorf("Attributes mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSceneState_MarshalJSON_FlatFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		state     SceneState
		wantKeys  []string
		wantNoKey string
	}{
		{
			name:  "state and attributes produce flat output",
			state: SceneState{State: "on", Attributes: map[string]any{"brightness": float64(200)}},
			// Should NOT contain an "attributes" key - all keys at top level
			wantNoKey: `"attributes"`,
			wantKeys:  []string{`"state"`, `"brightness"`},
		},
		{
			name:     "state only",
			state:    SceneState{State: "off"},
			wantKeys: []string{`"state"`},
		},
		{
			name:  "empty state",
			state: SceneState{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.state)
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			output := string(data)

			if tt.wantNoKey != "" && strings.Contains(output, tt.wantNoKey) {
				t.Errorf("output contains nested %q key, want flat format: %s", tt.wantNoKey, output)
			}
			for _, key := range tt.wantKeys {
				if !strings.Contains(output, key) {
					t.Errorf("output missing key %q in: %s", key, output)
				}
			}
		})
	}
}

func TestSceneState_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state SceneState
	}{
		{
			name: "with state and attributes",
			state: SceneState{
				State: "on",
				Attributes: map[string]any{
					"brightness": float64(100),
				},
			},
		},
		{
			name: "state only",
			state: SceneState{
				State: "off",
			},
		},
		{
			name:  "empty state",
			state: SceneState{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.state)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var got SceneState
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if diff := cmp.Diff(tt.state, got); diff != "" {
				t.Errorf("Roundtrip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAutomationConfig_UnmarshalJSON_PluralKeys(t *testing.T) {
	t.Parallel()

	input := `{
		"id": "abc123",
		"alias": "Morning Lights",
		"description": "Turn on lights",
		"mode": "single",
		"triggers": [{"platform": "time", "at": "08:00:00"}],
		"conditions": [{"condition": "state", "entity_id": "input_boolean.home"}],
		"actions": [{"service": "light.turn_on"}],
		"variables": {"brightness": 255}
	}`

	var got AutomationConfig
	if err := json.Unmarshal([]byte(input), &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.ID != "abc123" {
		t.Errorf("ID = %q, want %q", got.ID, "abc123")
	}
	if got.Alias != "Morning Lights" {
		t.Errorf("Alias = %q, want %q", got.Alias, "Morning Lights")
	}
	if got.Mode != "single" {
		t.Errorf("Mode = %q, want %q", got.Mode, "single")
	}
	if len(got.Triggers) != 1 {
		t.Errorf("Triggers len = %d, want 1", len(got.Triggers))
	}
	if len(got.Conditions) != 1 {
		t.Errorf("Conditions len = %d, want 1", len(got.Conditions))
	}
	if len(got.Actions) != 1 {
		t.Errorf("Actions len = %d, want 1", len(got.Actions))
	}
	if got.Variables == nil {
		t.Error("Variables should not be nil")
	}
}

func TestAutomationConfig_UnmarshalJSON_SingularKeys(t *testing.T) {
	t.Parallel()

	// This is the actual bug scenario: HA WebSocket returns singular keys
	input := `{
		"id": "abc123",
		"alias": "Morning Lights",
		"description": "Turn on lights",
		"mode": "single",
		"trigger": [{"platform": "time", "at": "08:00:00"}],
		"condition": [{"condition": "state", "entity_id": "input_boolean.home"}],
		"action": [{"service": "light.turn_on"}]
	}`

	var got AutomationConfig
	if err := json.Unmarshal([]byte(input), &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.ID != "abc123" {
		t.Errorf("ID = %q, want %q", got.ID, "abc123")
	}
	if got.Alias != "Morning Lights" {
		t.Errorf("Alias = %q, want %q", got.Alias, "Morning Lights")
	}
	if len(got.Triggers) != 1 {
		t.Errorf("Triggers len = %d, want 1 (singular 'trigger' key not mapped)", len(got.Triggers))
	}
	if len(got.Conditions) != 1 {
		t.Errorf("Conditions len = %d, want 1 (singular 'condition' key not mapped)", len(got.Conditions))
	}
	if len(got.Actions) != 1 {
		t.Errorf("Actions len = %d, want 1 (singular 'action' key not mapped)", len(got.Actions))
	}
}

func TestAutomationConfig_UnmarshalJSON_MixedKeys(t *testing.T) {
	t.Parallel()

	// Plural takes precedence; singular used as fallback
	input := `{
		"id": "mixed",
		"triggers": [{"platform": "time"}],
		"condition": [{"condition": "state"}],
		"action": [{"service": "light.turn_on"}]
	}`

	var got AutomationConfig
	if err := json.Unmarshal([]byte(input), &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(got.Triggers) != 1 {
		t.Errorf("Triggers len = %d, want 1", len(got.Triggers))
	}
	if len(got.Conditions) != 1 {
		t.Errorf("Conditions len = %d, want 1 (fallback to singular 'condition')", len(got.Conditions))
	}
	if len(got.Actions) != 1 {
		t.Errorf("Actions len = %d, want 1 (fallback to singular 'action')", len(got.Actions))
	}
}

func TestAutomationConfig_UnmarshalJSON_EmptyArrays(t *testing.T) {
	t.Parallel()

	// Empty arrays [] must be preserved (not treated as absent)
	input := `{
		"id": "empty",
		"triggers": [],
		"conditions": [],
		"actions": []
	}`

	var got AutomationConfig
	if err := json.Unmarshal([]byte(input), &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.Triggers == nil {
		t.Error("Triggers should be empty slice, not nil")
	}
	if len(got.Triggers) != 0 {
		t.Errorf("Triggers len = %d, want 0", len(got.Triggers))
	}
	if got.Conditions == nil {
		t.Error("Conditions should be empty slice, not nil")
	}
	if got.Actions == nil {
		t.Error("Actions should be empty slice, not nil")
	}
}

func TestAutomationConfig_UnmarshalJSON_AbsentKeys(t *testing.T) {
	t.Parallel()

	// Absent keys should leave fields nil (omitempty handles this in marshal)
	input := `{"id": "minimal", "alias": "Minimal"}`

	var got AutomationConfig
	if err := json.Unmarshal([]byte(input), &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.Triggers != nil {
		t.Errorf("Triggers should be nil for absent key, got %v", got.Triggers)
	}
	if got.Conditions != nil {
		t.Errorf("Conditions should be nil for absent key, got %v", got.Conditions)
	}
	if got.Actions != nil {
		t.Errorf("Actions should be nil for absent key, got %v", got.Actions)
	}
}

func TestAutomationConfig_UnmarshalJSON_RoundTrip_SingularToPlural(t *testing.T) {
	t.Parallel()

	// The key fix: unmarshal WS singular keys, marshal back → plural keys for REST
	wsInput := `{
		"id": "roundtrip",
		"alias": "Test",
		"trigger": [{"platform": "time"}],
		"condition": [{"condition": "state"}],
		"action": [{"service": "light.turn_on"}]
	}`

	var cfg AutomationConfig
	if err := json.Unmarshal([]byte(wsInput), &cfg); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Check top-level keys by decoding into a map — string-contains is unreliable
	// because "condition" appears inside the conditions array values themselves.
	var topLevel map[string]json.RawMessage
	if err := json.Unmarshal(out, &topLevel); err != nil {
		t.Fatalf("unmarshal output to check keys: %v", err)
	}

	for _, plural := range []string{"triggers", "conditions", "actions"} {
		if _, ok := topLevel[plural]; !ok {
			t.Errorf("output missing plural top-level key %q, got keys: %v", plural, topLevel)
		}
	}
	for _, singular := range []string{"trigger", "condition", "action"} {
		if _, ok := topLevel[singular]; ok {
			t.Errorf("output has unexpected singular top-level key %q", singular)
		}
	}
}

func TestAutomationConfig_Marshal_OmitsNilSlices(t *testing.T) {
	t.Parallel()

	// nil slices must be omitted (omitempty) so REST API doesn't get null fields
	cfg := AutomationConfig{
		ID:    "niltest",
		Alias: "No slices",
	}

	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	output := string(out)
	if strings.Contains(output, `"triggers"`) {
		t.Errorf("nil Triggers should be omitted, got: %s", output)
	}
	if strings.Contains(output, `"conditions"`) {
		t.Errorf("nil Conditions should be omitted, got: %s", output)
	}
	if strings.Contains(output, `"actions"`) {
		t.Errorf("nil Actions should be omitted, got: %s", output)
	}
}

func TestAutomationConfig_Marshal_OmitsEmptySlices(t *testing.T) {
	t.Parallel()

	// In Go, omitempty omits both nil and empty (len==0) slices.
	// Populated slices (len>0) are what matter for the REST API.
	cfg := AutomationConfig{
		ID:         "emptyslice",
		Triggers:   []any{},
		Conditions: []any{},
		Actions:    []any{},
	}

	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	output := string(out)
	// omitempty omits empty slices just like nil slices
	if strings.Contains(output, `"triggers"`) {
		t.Errorf("empty Triggers slice should be omitted by omitempty, got: %s", output)
	}
	if strings.Contains(output, `"conditions"`) {
		t.Errorf("empty Conditions slice should be omitted by omitempty, got: %s", output)
	}
	if strings.Contains(output, `"actions"`) {
		t.Errorf("empty Actions slice should be omitted by omitempty, got: %s", output)
	}
}

func TestAutomationConfig_Max_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantMax int
	}{
		{
			name:    "max field is preserved when set",
			input:   `{"id":"a1","mode":"parallel","max":5}`,
			wantMax: 5,
		},
		{
			name:    "max defaults to zero when absent",
			input:   `{"id":"a2","mode":"parallel"}`,
			wantMax: 0,
		},
		{
			name:    "max preserved with singular keys (WebSocket format)",
			input:   `{"id":"a3","mode":"queued","max":3,"trigger":[],"condition":[],"action":[]}`,
			wantMax: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var cfg AutomationConfig
			if err := json.Unmarshal([]byte(tt.input), &cfg); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if cfg.Max != tt.wantMax {
				t.Errorf("Max = %d, want %d", cfg.Max, tt.wantMax)
			}
		})
	}
}

func TestAutomationConfig_Max_MarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("max is included when non-zero", func(t *testing.T) {
		t.Parallel()
		cfg := AutomationConfig{ID: "a1", Mode: "parallel", Max: 5}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if !strings.Contains(string(data), `"max":5`) {
			t.Errorf("expected JSON to contain max:5, got: %s", data)
		}
	})

	t.Run("max is omitted when zero", func(t *testing.T) {
		t.Parallel()
		cfg := AutomationConfig{ID: "a2", Mode: "single"}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if strings.Contains(string(data), `"max"`) {
			t.Errorf("expected JSON to omit max, got: %s", data)
		}
	})
}

func TestScriptConfig_Max_RoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("max is preserved through marshal/unmarshal", func(t *testing.T) {
		t.Parallel()
		original := ScriptConfig{Alias: "My Script", Mode: "parallel", Max: 7}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if !strings.Contains(string(data), `"max":7`) {
			t.Errorf("expected JSON to contain max:7, got: %s", data)
		}

		var restored ScriptConfig
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if restored.Max != 7 {
			t.Errorf("Max = %d, want 7", restored.Max)
		}
	})

	t.Run("max is omitted when zero", func(t *testing.T) {
		t.Parallel()
		cfg := ScriptConfig{Alias: "My Script", Mode: "single"}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if strings.Contains(string(data), `"max"`) {
			t.Errorf("expected JSON to omit max, got: %s", data)
		}
	})
}

func TestStatisticsResult_StartTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result StatisticsResult
		want   time.Time
	}{
		{
			name: "unix timestamp in seconds",
			result: StatisticsResult{
				Start: 1704067200, // 2024-01-01 00:00:00 UTC
			},
			want: time.Unix(1704067200, 0),
		},
		{
			name: "unix timestamp in milliseconds",
			result: StatisticsResult{
				Start: 1704067200000, // ms timestamp
			},
			want: time.UnixMilli(1704067200000),
		},
		{
			name: "very large millisecond timestamp",
			result: StatisticsResult{
				Start: 1770000000000, // year 2026 in ms
			},
			want: time.UnixMilli(1770000000000),
		},
		{
			name: "zero timestamp",
			result: StatisticsResult{
				Start: 0,
			},
			want: time.Unix(0, 0),
		},
		{
			name: "fractional seconds",
			result: StatisticsResult{
				Start: 1704067200.5,
			},
			want: time.Unix(1704067200, 0), // truncates to seconds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.result.StartTime()
			if !got.Equal(tt.want) {
				t.Errorf("StartTime() = %v, want %v", got, tt.want)
			}
		})
	}
}
