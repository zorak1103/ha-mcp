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
	Max         int            `json:"max,omitempty"`  // concurrent run limit; only meaningful for mode=parallel|queued (HA default: 10)
	Triggers    []any          `json:"triggers,omitempty"`
	Conditions  []any          `json:"conditions,omitempty"`
	Actions     []any          `json:"actions,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
}

// unmarshalDualKeySlice tries the plural key first, then the singular key, and
// unmarshals the found value into dest. Errors are silently ignored to allow
// graceful degradation when an individual array contains unexpected element types.
func unmarshalDualKeySlice(raw map[string]json.RawMessage, plural, singular string, dest *[]any) {
	if v, ok := raw[plural]; ok {
		_ = json.Unmarshal(v, dest)
	} else if v, ok := raw[singular]; ok {
		_ = json.Unmarshal(v, dest)
	}
}

// UnmarshalJSON implements json.Unmarshaler for AutomationConfig.
// The HA WebSocket API (automation/config command) returns singular keys:
// "trigger", "condition", "action" — while the REST API and struct tags use
// plural forms: "triggers", "conditions", "actions". This method accepts both.
func (c *AutomationConfig) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if v, ok := raw["id"]; ok {
		if err := json.Unmarshal(v, &c.ID); err != nil {
			return err
		}
	}
	if v, ok := raw["alias"]; ok {
		if err := json.Unmarshal(v, &c.Alias); err != nil {
			return err
		}
	}
	if v, ok := raw["description"]; ok {
		if err := json.Unmarshal(v, &c.Description); err != nil {
			return err
		}
	}
	if v, ok := raw["mode"]; ok {
		if err := json.Unmarshal(v, &c.Mode); err != nil {
			return err
		}
	}
	if v, ok := raw["max"]; ok {
		if err := json.Unmarshal(v, &c.Max); err != nil {
			return err
		}
	}
	if v, ok := raw["variables"]; ok {
		if err := json.Unmarshal(v, &c.Variables); err != nil {
			return err
		}
	}

	// Accept both plural (REST/newer HA) and singular (WebSocket/older HA).
	unmarshalDualKeySlice(raw, "triggers", "trigger", &c.Triggers)
	unmarshalDualKeySlice(raw, "conditions", "condition", &c.Conditions)
	unmarshalDualKeySlice(raw, "actions", "action", &c.Actions)

	return nil
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

// Script represents a Home Assistant script with state and configuration.
type Script struct {
	EntityID      string        `json:"entity_id"`
	State         string        `json:"state"`
	FriendlyName  string        `json:"friendly_name,omitempty"`
	LastTriggered string        `json:"last_triggered,omitempty"`
	Config        *ScriptConfig `json:"config,omitempty"`
}

// ScriptConfig represents the configuration of a script.
type ScriptConfig struct {
	Alias       string         `json:"alias,omitempty"`
	Description string         `json:"description,omitempty"`
	Mode        string         `json:"mode,omitempty"` // single, restart, queued, parallel
	Max         int            `json:"max,omitempty"`  // concurrent run limit; only meaningful for mode=parallel|queued (HA default: 10)
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
// The HA REST API uses a flat format: {"state": "on", "brightness": 255}
// instead of a nested format: {"state": "on", "attributes": {"brightness": 255}}.
type SceneState struct {
	State      string         `json:"state,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for SceneState.
// The HA REST API returns scene entity states in flat format where all
// attributes are at the top level alongside the state key.
func (s *SceneState) UnmarshalJSON(data []byte) error {
	var flat map[string]any
	if err := json.Unmarshal(data, &flat); err != nil {
		return err
	}
	if st, ok := flat["state"].(string); ok {
		s.State = st
	}
	delete(flat, "state")
	if len(flat) > 0 {
		s.Attributes = flat
	}
	return nil
}

// MarshalJSON implements json.Marshaler for SceneState.
// Flattens State and Attributes back to the format expected by the HA REST API.
func (s SceneState) MarshalJSON() ([]byte, error) {
	flat := make(map[string]any, len(s.Attributes)+1)
	for k, v := range s.Attributes {
		flat[k] = v
	}
	if s.State != "" {
		flat["state"] = s.State
	}
	return json.Marshal(flat)
}

// Scene represents a Home Assistant scene with state and configuration.
type Scene struct {
	EntityID     string       `json:"entity_id"`
	State        string       `json:"state"`
	FriendlyName string       `json:"friendly_name,omitempty"`
	Config       *SceneConfig `json:"config,omitempty"`
}

// EntityRegistryEntry represents an entry in the Home Assistant entity registry.
type EntityRegistryEntry struct {
	EntityID      string   `json:"entity_id"`
	Platform      string   `json:"platform"`
	ConfigEntryID string   `json:"config_entry_id,omitempty"`
	DeviceID      string   `json:"device_id,omitempty"`
	AreaID        string   `json:"area_id,omitempty"`
	DisabledBy    string   `json:"disabled_by,omitempty"`
	HiddenBy      string   `json:"hidden_by,omitempty"`
	Name          string   `json:"name,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	UniqueID      string   `json:"unique_id,omitempty"`
	Labels        []string `json:"labels,omitempty"`
	Aliases       []string `json:"aliases,omitempty"`
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
	Labels           []string               `json:"labels,omitempty"`
}

// AreaRegistryEntry represents an entry in the Home Assistant area registry.
type AreaRegistryEntry struct {
	AreaID  string   `json:"area_id"`
	Name    string   `json:"name"`
	Picture string   `json:"picture,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
	FloorID string   `json:"floor_id,omitempty"`
	Icon    string   `json:"icon,omitempty"`
	Labels  []string `json:"labels,omitempty"`
}

// EntityRegistryUpdateConfig represents configuration for updating an entity registry entry.
// Fields use pointer types to distinguish "not provided" from "set to empty".
type EntityRegistryUpdateConfig struct {
	Name        *string  `json:"name,omitempty"`
	Icon        *string  `json:"icon,omitempty"`
	AreaID      *string  `json:"area_id,omitempty"`
	DisabledBy  *string  `json:"disabled_by,omitempty"`
	HiddenBy    *string  `json:"hidden_by,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	NewEntityID *string  `json:"new_entity_id,omitempty"`
}

// DeviceRegistryUpdateConfig represents configuration for updating a device registry entry.
// Fields use pointer types to distinguish "not provided" from "set to empty".
type DeviceRegistryUpdateConfig struct {
	NameByUser *string  `json:"name_by_user,omitempty"`
	AreaID     *string  `json:"area_id,omitempty"`
	DisabledBy *string  `json:"disabled_by,omitempty"`
	Labels     []string `json:"labels,omitempty"`
}

// AreaConfig represents configuration for creating or updating an area.
type AreaConfig struct {
	Name    string   `json:"name,omitempty"`
	Icon    string   `json:"icon,omitempty"`
	Picture string   `json:"picture,omitempty"`
	FloorID string   `json:"floor_id,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
	Labels  []string `json:"labels,omitempty"`
}

// LabelRegistryEntry represents an entry in the Home Assistant label registry.
type LabelRegistryEntry struct {
	LabelID     string `json:"label_id"`
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

// LabelConfig represents configuration for creating or updating a label.
type LabelConfig struct {
	Name        string `json:"name,omitempty"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

// FloorRegistryEntry represents an entry in the Home Assistant floor registry.
type FloorRegistryEntry struct {
	FloorID string   `json:"floor_id"`
	Name    string   `json:"name"`
	Level   *int     `json:"level,omitempty"`
	Icon    string   `json:"icon,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
}

// FloorConfig represents configuration for creating or updating a floor.
type FloorConfig struct {
	Name    string   `json:"name,omitempty"`
	Level   *int     `json:"level,omitempty"`
	Icon    string   `json:"icon,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
}

// ZoneRegistryEntry represents an entry in the Home Assistant zone registry.
type ZoneRegistryEntry struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Radius    float64 `json:"radius"`
	Icon      string  `json:"icon,omitempty"`
	Passive   bool    `json:"passive,omitempty"`
}

// ZoneConfig represents configuration for creating or updating a zone.
type ZoneConfig struct {
	Name      string   `json:"name,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Radius    *float64 `json:"radius,omitempty"`
	Icon      string   `json:"icon,omitempty"`
	Passive   *bool    `json:"passive,omitempty"`
}

// PersonRegistryEntry represents an entry in the Home Assistant person registry.
type PersonRegistryEntry struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	UserID         string   `json:"user_id,omitempty"`
	DeviceTrackers []string `json:"device_trackers,omitempty"`
	Picture        string   `json:"picture,omitempty"`
}

// PersonConfig represents configuration for creating or updating a person.
type PersonConfig struct {
	Name           string   `json:"name,omitempty"`
	UserID         string   `json:"user_id,omitempty"`
	DeviceTrackers []string `json:"device_trackers,omitempty"`
	Picture        string   `json:"picture,omitempty"`
}

// TagRegistryEntry represents an entry in the Home Assistant tag registry.
type TagRegistryEntry struct {
	TagID       string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	LastScanned string `json:"last_scanned,omitempty"`
}

// TagConfig represents configuration for creating or updating a tag.
type TagConfig struct {
	TagID       string `json:"tag_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
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

// StartTime returns Start as time.Time.
// The WebSocket API may return timestamps in seconds or milliseconds.
// If Start > 1e12, assumes milliseconds and converts accordingly.
func (s StatisticsResult) StartTime() time.Time {
	if s.Start > 1e12 {
		return time.UnixMilli(int64(s.Start))
	}
	return time.Unix(int64(s.Start), 0)
}

// Target represents a target specification for entities, devices, areas, and labels.
// This is used for service calls and for querying triggers, conditions, and services.
type Target struct {
	EntityID []string `json:"entity_id,omitempty"`
	DeviceID []string `json:"device_id,omitempty"`
	AreaID   []string `json:"area_id,omitempty"`
	LabelID  []string `json:"label_id,omitempty"`
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

// Service represents a Home Assistant service domain with its services.
type Service struct {
	Domain   string                       `json:"domain"`
	Services map[string]ServiceDefinition `json:"services"`
}

// ServiceDefinition represents a single service definition.
type ServiceDefinition struct {
	Name        string                  `json:"name,omitempty"`
	Description string                  `json:"description,omitempty"`
	Fields      map[string]ServiceField `json:"fields,omitempty"`
	Target      *ServiceTarget          `json:"target,omitempty"`
}

// ServiceField represents a field in a service definition.
type ServiceField struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Example     any    `json:"example,omitempty"`
	Selector    any    `json:"selector,omitempty"`
}

// ServiceTarget represents the target selector for a service.
type ServiceTarget struct {
	Entity []TargetSelector `json:"entity,omitempty"`
	Device []TargetSelector `json:"device,omitempty"`
	Area   bool             `json:"area,omitempty"`
}

// TargetSelector represents entity/device selection criteria.
// Note: Domain, Integration, and DeviceClass use FlexibleString because
// Home Assistant can return these as either a string or an array of strings.
type TargetSelector struct {
	Domain            FlexibleString `json:"domain,omitempty"`
	Integration       FlexibleString `json:"integration,omitempty"`
	DeviceClass       FlexibleString `json:"device_class,omitempty"`
	SupportedFeatures []int          `json:"supported_features,omitempty"`
}

// Config represents Home Assistant system configuration.
type Config struct {
	Components       []string   `json:"components"`
	ConfigDir        string     `json:"config_dir"`
	ConfigSource     string     `json:"config_source"`
	Elevation        int        `json:"elevation"`
	Latitude         float64    `json:"latitude"`
	Longitude        float64    `json:"longitude"`
	LocationName     string     `json:"location_name"`
	TimeZone         string     `json:"time_zone"`
	UnitSystem       UnitSystem `json:"unit_system"`
	Version          string     `json:"version"`
	WhitelistExtDirs []string   `json:"whitelist_external_dirs"`
	AllowlistExtDirs []string   `json:"allowlist_external_dirs"`
	AllowlistExtURLs []string   `json:"allowlist_external_urls"`
	Currency         string     `json:"currency"`
	Country          string     `json:"country,omitempty"`
	Language         string     `json:"language,omitempty"`
	SafeMode         bool       `json:"safe_mode"`
	State            string     `json:"state"`
	InternalURL      string     `json:"internal_url,omitempty"`
	ExternalURL      string     `json:"external_url,omitempty"`
}

// UnitSystem represents the configured unit system.
type UnitSystem struct {
	Length            string `json:"length"`
	AccumulatedPrecip string `json:"accumulated_precipitation"`
	Mass              string `json:"mass"`
	Pressure          string `json:"pressure"`
	Temperature       string `json:"temperature"`
	Volume            string `json:"volume"`
	WindSpeed         string `json:"wind_speed"`
}

// TemplateRequest represents a request to render a Jinja2 template.
type TemplateRequest struct {
	Template string `json:"template"`
}

// LogbookEntry represents a single entry in the Home Assistant logbook.
type LogbookEntry struct {
	When          string  `json:"when"`
	Name          string  `json:"name"`
	Message       string  `json:"message,omitempty"`
	EntityID      string  `json:"entity_id,omitempty"`
	Domain        string  `json:"domain,omitempty"`
	State         string  `json:"state,omitempty"`
	ContextUserID *string `json:"context_user_id,omitempty"`
}

// SystemLogEntry represents a single entry in the Home Assistant system log ring buffer.
type SystemLogEntry struct {
	Name          string   `json:"name"`
	Message       []string `json:"message"`
	Level         string   `json:"level"`
	Source        []any    `json:"source"` // [filename, line_number] — heterogeneous tuple
	Timestamp     float64  `json:"timestamp"`
	Exception     string   `json:"exception"`
	Count         int      `json:"count"`
	FirstOccurred float64  `json:"first_occurred"`
}

// ConfigCheckResult represents the result of a configuration validation check.
type ConfigCheckResult struct {
	Result string  `json:"result"` // "valid" or "invalid"
	Errors *string `json:"errors"` // null if valid, error message if invalid
}

// ConfigEntryFlowResult represents a config entry flow step response.
// Config Entry Flow is Home Assistant's HTTP-based mechanism for creating
// certain helper types (threshold, derivative, integration, group, template).
type ConfigEntryFlowResult struct {
	FlowID      string             `json:"flow_id"`
	Type        string             `json:"type"` // "form", "menu", "create_entry", "abort"
	StepID      string             `json:"step_id,omitempty"`
	Handler     string             `json:"handler"`
	DataSchema  []OptionsFlowField `json:"data_schema,omitempty"`  // Schema for form fields
	MenuOptions []string           `json:"menu_options,omitempty"` // Available menu options
	Errors      map[string]string  `json:"errors,omitempty"`
	Result      *ConfigEntry       `json:"result,omitempty"`
	Description string             `json:"description,omitempty"`
}

// ConfigEntry represents a created config entry from the Config Entry Flow.
type ConfigEntry struct {
	EntryID string `json:"entry_id"`
	Domain  string `json:"domain"`
	Title   string `json:"title"`
}

// ConfigEntryFull represents a Home Assistant config entry with full details.
// This is returned by the config_entries/get and config_entries/get_single WebSocket commands.
type ConfigEntryFull struct {
	EntryID                string         `json:"entry_id"`
	Domain                 string         `json:"domain"`
	Title                  string         `json:"title"`
	Source                 string         `json:"source"`
	State                  string         `json:"state"`
	DisabledBy             string         `json:"disabled_by,omitempty"`
	Options                map[string]any `json:"options,omitempty"`
	PrefDisableNewEntities bool           `json:"pref_disable_new_entities,omitempty"`
	PrefDisablePolling     bool           `json:"pref_disable_polling,omitempty"`
	SupportsOptions        bool           `json:"supports_options,omitempty"`
	SupportsReconfigure    bool           `json:"supports_reconfigure,omitempty"`
	SupportsUnload         bool           `json:"supports_unload,omitempty"`
	SupportsRemoveDevice   bool           `json:"supports_remove_device,omitempty"`
}

// OptionsFlowResult represents a config entry options flow step response.
// Options Flow is used to read current option values from config entries.
type OptionsFlowResult struct {
	FlowID      string             `json:"flow_id"`
	Type        string             `json:"type"` // "form", "menu", "create_entry", "abort"
	StepID      string             `json:"step_id,omitempty"`
	Handler     string             `json:"handler"`
	DataSchema  []OptionsFlowField `json:"data_schema,omitempty"`
	Errors      map[string]string  `json:"errors,omitempty"`
	MenuOptions []string           `json:"menu_options,omitempty"` // For "menu" type
}

// CalendarEntry represents a Home Assistant calendar entity.
type CalendarEntry struct {
	EntityID string `json:"entity_id"`
	Name     string `json:"name"`
}

// CalendarEvent represents an event in a Home Assistant calendar.
type CalendarEvent struct {
	Start        CalendarDateTime `json:"start"`
	End          CalendarDateTime `json:"end"`
	Summary      string           `json:"summary"`
	Description  string           `json:"description,omitempty"`
	Location     string           `json:"location,omitempty"`
	UID          string           `json:"uid,omitempty"`
	RecurrenceID string           `json:"recurrence_id,omitempty"`
}

// CalendarDateTime represents a calendar date/time value.
// Home Assistant supports both date-only (all-day events) and datetime formats.
type CalendarDateTime struct {
	Date     string `json:"date,omitempty"`
	DateTime string `json:"dateTime,omitempty"`
}

// OptionsFlowField represents a field in an options flow data schema.
type OptionsFlowField struct {
	Name        string         `json:"name"`
	Type        string         `json:"type,omitempty"`
	Description map[string]any `json:"description,omitempty"` // Contains "suggested_value"
}

// DashboardEntry represents a Home Assistant Lovelace dashboard entry.
type DashboardEntry struct {
	ID            string `json:"id"`
	URLPath       string `json:"url_path"`
	Title         string `json:"title"`
	Icon          string `json:"icon,omitempty"`
	Mode          string `json:"mode"`
	RequireAdmin  bool   `json:"require_admin"`
	ShowInSidebar bool   `json:"show_in_sidebar"`
}

// DashboardConfig represents configuration for creating or updating a dashboard.
type DashboardConfig struct {
	URLPath       string `json:"url_path,omitempty"`
	Title         string `json:"title,omitempty"`
	Icon          string `json:"icon,omitempty"`
	Mode          string `json:"mode,omitempty"`
	RequireAdmin  *bool  `json:"require_admin,omitempty"`
	ShowInSidebar *bool  `json:"show_in_sidebar,omitempty"`
}
