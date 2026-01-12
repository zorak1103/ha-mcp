// Package homeassistant provides types for the Home Assistant REST API.
package homeassistant

import (
	"encoding/json"
	"strings"
	"time"
)

// FlexibleString is a type that can unmarshal from either a JSON string or an array of strings.
// Home Assistant sometimes returns version fields as arrays instead of strings.
type FlexibleString string

// UnmarshalJSON implements json.Unmarshaler for FlexibleString.
func (fs *FlexibleString) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*fs = FlexibleString(str)
		return nil
	}

	// Try to unmarshal as array of strings
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*fs = FlexibleString(strings.Join(arr, ", "))
		return nil
	}

	// If both fail, set to empty string
	*fs = ""
	return nil
}

// String returns the string value of FlexibleString.
func (fs FlexibleString) String() string {
	return string(fs)
}

// MarshalJSON implements json.Marshaler for FlexibleString.
func (fs FlexibleString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(fs))
}

// FlexibleIdentifier is a type that can unmarshal from either a JSON string or a number.
// Home Assistant sometimes returns identifiers as numbers instead of strings.
type FlexibleIdentifier string

// UnmarshalJSON implements json.Unmarshaler for FlexibleIdentifier.
func (fi *FlexibleIdentifier) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*fi = FlexibleIdentifier(str)
		return nil
	}

	// Try to unmarshal as number (float64 covers all JSON numbers)
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*fi = FlexibleIdentifier(json.Number(data).String())
		return nil
	}

	// If both fail, set to empty string
	*fi = ""
	return nil
}

// String returns the string value of FlexibleIdentifier.
func (fi FlexibleIdentifier) String() string {
	return string(fi)
}

// MarshalJSON implements json.Marshaler for FlexibleIdentifier.
func (fi FlexibleIdentifier) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(fi))
}

// Entity represents a Home Assistant entity state.
type Entity struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged time.Time      `json:"last_changed"`
	LastUpdated time.Time      `json:"last_updated"`
	Context     Context        `json:"context"`
}

// Context represents the context of a state change.
type Context struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id,omitempty"`
	UserID   string `json:"user_id,omitempty"`
}

// StateUpdate represents a request to update an entity's state.
type StateUpdate struct {
	State      string         `json:"state"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// HistoryEntry represents a single history entry for an entity.
// The WebSocket API returns a compact format with short field names.
type HistoryEntry struct {
	EntityID    string         `json:"entity_id,omitempty"`
	State       string         `json:"s"`            // "s" in WS API, "state" in REST API
	Attributes  map[string]any `json:"a,omitempty"`  // "a" in WS API, "attributes" in REST API
	LastChanged float64        `json:"lc"`           // Unix timestamp (seconds) in WS API
	LastUpdated float64        `json:"lu,omitempty"` // Unix timestamp (seconds) in WS API
}

// LastChangedTime returns LastChanged as time.Time.
// The WebSocket API returns timestamps in seconds (Unix epoch).
// If LastChanged is 0, falls back to LastUpdated.
func (h HistoryEntry) LastChangedTime() time.Time {
	ts := h.LastChanged
	// Fall back to LastUpdated if LastChanged is 0
	if ts == 0 && h.LastUpdated > 0 {
		ts = h.LastUpdated
	}
	// If timestamp looks like it's in milliseconds (very large number), convert
	if ts > 1e12 {
		return time.UnixMilli(int64(ts))
	}
	return time.Unix(int64(ts), 0)
}

// LastUpdatedTime returns LastUpdated as time.Time.
// The WebSocket API returns timestamps in seconds (Unix epoch).
func (h HistoryEntry) LastUpdatedTime() time.Time {
	// If timestamp looks like it's in milliseconds (very large number), convert
	if h.LastUpdated > 1e12 {
		return time.UnixMilli(int64(h.LastUpdated))
	}
	return time.Unix(int64(h.LastUpdated), 0)
}

// Automation represents a Home Assistant automation.
type Automation struct {
	EntityID      string            `json:"entity_id"`
	State         string            `json:"state"`
	FriendlyName  string            `json:"friendly_name,omitempty"`
	LastTriggered string            `json:"last_triggered,omitempty"`
	Config        *AutomationConfig `json:"config,omitempty"`
}

// AutomationConfig represents the configuration of an automation.
type AutomationConfig struct {
	ID          string         `json:"id,omitempty"`
	Alias       string         `json:"alias,omitempty"`
	Description string         `json:"description,omitempty"`
	Mode        string         `json:"mode,omitempty"` // single, restart, queued, parallel
	Triggers    []any          `json:"triggers,omitempty"`
	Conditions  []any          `json:"conditions,omitempty"`
	Actions     []any          `json:"actions,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
}

// HelperConfig represents the configuration for creating/updating an input helper.
type HelperConfig struct {
	// Platform is the helper type: input_boolean, input_number, input_text, input_select, input_datetime
	Platform string `json:"platform"`
	// ID is the unique identifier for the helper (without the platform prefix)
	ID string `json:"id"`
	// Config contains the platform-specific configuration
	Config map[string]any `json:"config"`
}

// InputBooleanConfig represents configuration for an input_boolean helper.
type InputBooleanConfig struct {
	Name    string `json:"name"`
	Icon    string `json:"icon,omitempty"`
	Initial bool   `json:"initial,omitempty"`
}

// InputNumberConfig represents configuration for an input_number helper.
type InputNumberConfig struct {
	Name    string  `json:"name"`
	Icon    string  `json:"icon,omitempty"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Step    float64 `json:"step,omitempty"`
	Initial float64 `json:"initial,omitempty"`
	Mode    string  `json:"mode,omitempty"` // box or slider
	Unit    string  `json:"unit_of_measurement,omitempty"`
}

// InputTextConfig represents configuration for an input_text helper.
type InputTextConfig struct {
	Name    string `json:"name"`
	Icon    string `json:"icon,omitempty"`
	Min     int    `json:"min,omitempty"`
	Max     int    `json:"max,omitempty"`
	Initial string `json:"initial,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	Mode    string `json:"mode,omitempty"` // text or password
}

// InputSelectConfig represents configuration for an input_select helper.
type InputSelectConfig struct {
	Name    string   `json:"name"`
	Icon    string   `json:"icon,omitempty"`
	Options []string `json:"options"`
	Initial string   `json:"initial,omitempty"`
}

// InputDateTimeConfig represents configuration for an input_datetime helper.
type InputDateTimeConfig struct {
	Name    string `json:"name"`
	Icon    string `json:"icon,omitempty"`
	HasDate bool   `json:"has_date"`
	HasTime bool   `json:"has_time"`
	Initial string `json:"initial,omitempty"`
}

// ScriptConfig represents the configuration of a script.
type ScriptConfig struct {
	Alias       string         `json:"alias,omitempty"`
	Description string         `json:"description,omitempty"`
	Mode        string         `json:"mode,omitempty"` // single, restart, queued, parallel
	Icon        string         `json:"icon,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
	Sequence    []any          `json:"sequence"`
}

// SceneConfig represents the configuration of a scene.
type SceneConfig struct {
	Name     string                `json:"name"`
	Icon     string                `json:"icon,omitempty"`
	Entities map[string]SceneState `json:"entities"`
}

// SceneState represents the desired state of an entity in a scene.
type SceneState struct {
	State      string         `json:"state,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// EntityRegistryEntry represents an entry in the Home Assistant entity registry.
type EntityRegistryEntry struct {
	EntityID      string `json:"entity_id"`
	Platform      string `json:"platform"`
	ConfigEntryID string `json:"config_entry_id,omitempty"`
	DeviceID      string `json:"device_id,omitempty"`
	AreaID        string `json:"area_id,omitempty"`
	DisabledBy    string `json:"disabled_by,omitempty"`
	HiddenBy      string `json:"hidden_by,omitempty"`
	Name          string `json:"name,omitempty"`
	Icon          string `json:"icon,omitempty"`
	UniqueID      string `json:"unique_id,omitempty"`
}

// DeviceRegistryEntry represents an entry in the Home Assistant device registry.
type DeviceRegistryEntry struct {
	ID               string                 `json:"id"`
	ConfigEntries    []string               `json:"config_entries,omitempty"`
	Connections      [][]FlexibleIdentifier `json:"connections,omitempty"`
	Identifiers      [][]FlexibleIdentifier `json:"identifiers,omitempty"`
	Manufacturer     string                 `json:"manufacturer,omitempty"`
	Model            FlexibleString         `json:"model,omitempty"`
	Name             string                 `json:"name,omitempty"`
	SWVersion        FlexibleString         `json:"sw_version,omitempty"`
	HWVersion        FlexibleString         `json:"hw_version,omitempty"`
	AreaID           string                 `json:"area_id,omitempty"`
	NameByUser       string                 `json:"name_by_user,omitempty"`
	DisabledBy       string                 `json:"disabled_by,omitempty"`
	ConfigurationURL string                 `json:"configuration_url,omitempty"`
}

// AreaRegistryEntry represents an entry in the Home Assistant area registry.
type AreaRegistryEntry struct {
	AreaID  string   `json:"area_id"`
	Name    string   `json:"name"`
	Picture string   `json:"picture,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
}

// StreamInfo represents camera stream information from Home Assistant.
type StreamInfo struct {
	URL string `json:"url"`
}

// MediaBrowseResult represents media browser results from Home Assistant.
type MediaBrowseResult struct {
	Title            string               `json:"title"`
	MediaClass       string               `json:"media_class"`
	MediaContentID   string               `json:"media_content_id,omitempty"`
	MediaContentType string               `json:"media_content_type,omitempty"`
	CanPlay          bool                 `json:"can_play"`
	CanExpand        bool                 `json:"can_expand"`
	Thumbnail        string               `json:"thumbnail,omitempty"`
	Children         []*MediaBrowseResult `json:"children,omitempty"`
}

// StatisticsResult represents statistics data from the Home Assistant recorder.
type StatisticsResult struct {
	StatisticID string   `json:"statistic_id"`
	Start       float64  `json:"start"`         // Unix timestamp
	End         float64  `json:"end,omitempty"` // Unix timestamp
	Mean        *float64 `json:"mean,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Sum         *float64 `json:"sum,omitempty"`
	State       *float64 `json:"state,omitempty"`
	Change      *float64 `json:"change,omitempty"`
}

// Target represents a target specification for entities, devices, areas, and labels.
// This is used for service calls and for querying triggers, conditions, and services.
type Target struct {
	EntityID []string `json:"entity_id,omitempty"`
	DeviceID []string `json:"device_id,omitempty"`
	AreaID   []string `json:"area_id,omitempty"`
	LabelID  []string `json:"label_id,omitempty"`
}

// TargetRequest represents a request to get triggers, conditions, or services for a target.
type TargetRequest struct {
	Target      Target `json:"target"`
	ExpandGroup *bool  `json:"expand_group,omitempty"`
}

// ExtractFromTargetResult represents the result of extracting entities, devices, and areas from a target.
type ExtractFromTargetResult struct {
	ReferencedEntities []string `json:"referenced_entities"`
	ReferencedDevices  []string `json:"referenced_devices"`
	ReferencedAreas    []string `json:"referenced_areas"`
	MissingDevices     []string `json:"missing_devices"`
	MissingAreas       []string `json:"missing_areas"`
	MissingFloors      []string `json:"missing_floors"`
	MissingLabels      []string `json:"missing_labels"`
}
