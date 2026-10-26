// Package homeassistant provides types for the Home Assistant REST API.
package homeassistant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/jsonorder"
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

	// KeyOrder records the document key order of the decoded config (see
	// package jsonorder). It is nil for configs built in code, which marshal
	// exactly like a plain struct.
	KeyOrder *jsonorder.Order `json:"-"`

	// extra holds the keys of the decoded config that this struct does not
	// model (trigger_variables, initial_state, max_exceeded, trace, keys of
	// future HA versions), verbatim, so a read-modify-write round trip does not
	// delete them. Unexported so code cannot put a modeled key in here (it would
	// be written twice); treat as immutable - struct copies share the map.
	extra map[string]json.RawMessage `json:"-"`
}

// automationKnownKeys are the top-level keys AutomationConfig models, derived
// from the struct tags so they cannot drift. The singular trigger/condition/
// action are modeled too: UnmarshalJSON normalizes them to the plural fields.
var automationKnownKeys = knownConfigKeys(reflect.TypeOf(automationConfigAlias{}), "trigger", "condition", "action")

// knownConfigKeys returns the JSON key names declared by the exported fields of
// struct type t, plus the given extra names. Fields tagged "-" or without a
// name in their json tag are skipped.
func knownConfigKeys(t reflect.Type, extra ...string) map[string]struct{} {
	keys := make(map[string]struct{}, t.NumField()+len(extra))
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		keys[name] = struct{}{}
	}
	for _, name := range extra {
		keys[name] = struct{}{}
	}
	return keys
}

// collectExtraKeys returns the entries of raw whose key is not in known, or nil
// when there are none.
func collectExtraKeys(raw map[string]json.RawMessage, known map[string]struct{}) map[string]json.RawMessage {
	var extra map[string]json.RawMessage
	for k, v := range raw {
		if _, ok := known[k]; ok {
			continue
		}
		if extra == nil {
			extra = make(map[string]json.RawMessage)
		}
		extra[k] = v
	}
	return extra
}

// decodeConfigList decodes the value of a trigger/condition/action key. HA
// validates these with ensure_list, so besides a list the key may hold a single
// mapping (or a template string) that stands for a one-element list; wrapped
// reports that case. null decodes to no items.
func decodeConfigList(raw json.RawMessage) (items []any, wrapped bool, err error) {
	switch raw[0] {
	case '[':
		err = json.Unmarshal(raw, &items)
		return items, false, err
	case 'n':
		return nil, false, nil
	}
	var item any
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, false, err
	}
	return []any{item}, true, nil
}

// pickDualKey returns the plural key's value when present, else the singular
// key's (nil when neither is).
func pickDualKey(plural, singular json.RawMessage) json.RawMessage {
	if plural != nil {
		return plural
	}
	return singular
}

// automationConfigWire decodes an automation config in one pass. The embedded
// alias supplies every modeled field through its json tag, so a new field needs
// no decode code. The shallower list fields below shadow the alias's by name,
// arriving undecoded for decodeConfigList, and add the singular spellings.
type automationConfigWire struct {
	*automationConfigAlias
	Triggers   json.RawMessage `json:"triggers"`
	Conditions json.RawMessage `json:"conditions"`
	Actions    json.RawMessage `json:"actions"`
	Trigger    json.RawMessage `json:"trigger"`
	Condition  json.RawMessage `json:"condition"`
	Action     json.RawMessage `json:"action"`
}

// decodeLists fills the trigger/condition/action fields from wire and returns
// the plural names of the lists that were wrapped from a single value.
func (w *automationConfigWire) decodeLists() (wrapped []string, err error) {
	for _, l := range []struct {
		name             string
		plural, singular json.RawMessage
		dest             *[]any
	}{
		{"triggers", w.Triggers, w.Trigger, &w.automationConfigAlias.Triggers},
		{"conditions", w.Conditions, w.Condition, &w.automationConfigAlias.Conditions},
		{"actions", w.Actions, w.Action, &w.automationConfigAlias.Actions},
	} {
		raw := pickDualKey(l.plural, l.singular)
		if raw == nil {
			continue
		}
		items, wasWrapped, decErr := decodeConfigList(raw)
		if decErr != nil {
			return nil, fmt.Errorf("decoding %s: %w", l.name, decErr)
		}
		*l.dest = items
		if wasWrapped {
			wrapped = append(wrapped, l.name)
		}
	}
	return wrapped, nil
}

// UnmarshalJSON implements json.Unmarshaler for AutomationConfig.
// The HA WebSocket API (automation/config command) returns singular keys:
// "trigger", "condition", "action" — while the REST API and struct tags use
// plural forms: "triggers", "conditions", "actions". This method accepts both,
// and a single mapping or string in place of a list (HA's ensure_list), which
// it keeps as a one-element list. Every other modeled field is decoded by its
// json tag, so a new field needs no change here.
func (c *AutomationConfig) UnmarshalJSON(data []byte) error {
	wire := automationConfigWire{automationConfigAlias: (*automationConfigAlias)(c)}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	// Accept both plural (REST/newer HA) and singular (WebSocket/older HA).
	wrapped, err := wire.decodeLists()
	if err != nil {
		return err
	}

	// A capture failure leaves KeyOrder nil (sorted-key output) and never
	// fails the decode: the order is a best-effort addition.
	c.KeyOrder, _ = jsonorder.Capture(data)
	normalizeDualKeyOrder(c.KeyOrder)
	for _, name := range wrapped {
		wrapListOrder(c.KeyOrder, name)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.extra = collectExtraKeys(raw, automationKnownKeys)
	return nil
}

// automationConfigAlias has AutomationConfig's fields and tags but none of its
// methods, so marshaling it cannot recurse into MarshalJSON.
type automationConfigAlias AutomationConfig

// MarshalJSON implements json.Marshaler for AutomationConfig. Without a
// KeyOrder and no kept unmodeled keys the output is exactly that of the plain
// struct; with an order, object keys inside the config are emitted in their
// recorded order, and kept unmodeled keys are written along with the modeled ones.
func (c AutomationConfig) MarshalJSON() ([]byte, error) {
	return marshalWithOrder(automationConfigAlias(c), c.KeyOrder, c.extra)
}

// marshalWithOrder marshals v like json.Marshal. When order is non-nil the
// resulting object is re-emitted with its keys in the recorded order, and the
// unmodeled keys in extra are added to it (a key v already wrote wins). Kept
// values are decoded rather than inserted raw so their nested key order follows
// order too.
func marshalWithOrder(v any, order *jsonorder.Order, extra map[string]json.RawMessage) ([]byte, error) {
	plain, err := json.Marshal(v)
	if err != nil || (order == nil && len(extra) == 0) {
		return plain, err
	}

	generic, err := decodeNumbers[map[string]any](plain)
	if err != nil {
		return nil, fmt.Errorf("decoding config for ordered encoding: %w", err)
	}
	for k, raw := range extra {
		if _, written := generic[k]; written {
			continue
		}
		val, decErr := decodeNumbers[any](raw)
		if decErr != nil {
			return nil, fmt.Errorf("decoding kept key %q for ordered encoding: %w", k, decErr)
		}
		generic[k] = val
	}

	ordered, err := jsonorder.Marshal(generic, order)
	if err != nil {
		return nil, fmt.Errorf("encoding config in key order: %w", err)
	}
	return ordered, nil
}

// decodeNumbers decodes data into a T, keeping numeric literals exactly as
// written (json.Number) instead of squeezing them through float64.
func decodeNumbers[T any](data []byte) (T, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out T
	err := dec.Decode(&out)
	return out, err
}

// wrapListOrder turns the order of the object that was wrapped into the
// one-element list name into the order of that list, so the object's key order
// still applies to the element. o is modified in place.
func wrapListOrder(o *jsonorder.Order, name string) {
	if o == nil {
		return
	}
	if sub, ok := o.Fields[name]; ok { // a scalar value has no sub-order
		o.Fields[name] = &jsonorder.Order{Items: []*jsonorder.Order{sub}}
	}
}

// normalizeDualKeyOrder rewrites the singular trigger/condition/action entries
// of a freshly captured order to their plural names, mirroring
// pickDualKey: the plural key wins and the singular one is dropped
// when both are present. o is modified in place.
func normalizeDualKeyOrder(o *jsonorder.Order) {
	if o == nil {
		return
	}
	for _, names := range [][2]string{{"triggers", "trigger"}, {"conditions", "condition"}, {"actions", "action"}} {
		plural, singular := names[0], names[1]
		idx := slices.Index(o.Keys, singular)
		if idx < 0 {
			continue
		}
		sub, hasSub := o.Fields[singular]
		delete(o.Fields, singular)
		if slices.Contains(o.Keys, plural) {
			o.Keys = slices.Delete(o.Keys, idx, idx+1)
			continue
		}
		o.Keys[idx] = plural
		if hasSub {
			o.Fields[plural] = sub
		}
	}
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

	// KeyOrder records the document key order of the decoded config (see
	// package jsonorder). It is nil for configs built in code, which marshal
	// exactly like a plain struct.
	KeyOrder *jsonorder.Order `json:"-"`

	// extra holds the keys of the decoded config that this struct does not
	// model (max_exceeded, trace, keys of future HA versions), verbatim, so a
	// read-modify-write round trip does not delete them. Unexported so code
	// cannot put a modeled key in here (it would be written twice); treat as
	// immutable - struct copies share the map.
	extra map[string]json.RawMessage `json:"-"`
}

// scriptConfigAlias has ScriptConfig's fields and tags but none of its
// methods, so (un)marshaling it cannot recurse into the methods below.
type scriptConfigAlias ScriptConfig

// scriptKnownKeys are the top-level keys ScriptConfig models, derived from the
// struct tags so they cannot drift.
var scriptKnownKeys = knownConfigKeys(reflect.TypeOf(scriptConfigAlias{}))

// UnmarshalJSON implements json.Unmarshaler for ScriptConfig. It decodes like
// the plain struct and additionally records the document key order and keeps
// the keys the struct does not model.
func (c *ScriptConfig) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, (*scriptConfigAlias)(c)); err != nil {
		return err
	}
	// A capture failure leaves KeyOrder nil and never fails the decode.
	c.KeyOrder, _ = jsonorder.Capture(data)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.extra = collectExtraKeys(raw, scriptKnownKeys)
	return nil
}

// MarshalJSON implements json.Marshaler for ScriptConfig. Without a KeyOrder
// and kept unmodeled keys the output is exactly that of the plain struct.
func (c ScriptConfig) MarshalJSON() ([]byte, error) {
	return marshalWithOrder(scriptConfigAlias(c), c.KeyOrder, c.extra)
}

// SceneConfig represents the configuration of a scene.
type SceneConfig struct {
	Name     string                `json:"name"`
	Icon     string                `json:"icon,omitempty"`
	Entities map[string]SceneState `json:"entities"`
	// Metadata is passed through verbatim (e.g. HA's scene editor writes
	// {"<entity_id>": {"entity_only": true}}) - never synthesized or pruned by this client.
	Metadata map[string]any `json:"metadata,omitempty"`
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
// attributes are at the top level alongside the state key. HA also accepts
// shorthand entity values in scenes.yaml: a bare string ("on") or bool (true/false)
// instead of an object.
func (s *SceneState) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		s.State = str
		return nil
	}

	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		if b {
			s.State = "on"
		} else {
			s.State = "off"
		}
		return nil
	}

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
	// ID is the registry's own id of the entry (32 hex characters), which
	// device actions use in place of an entity id.
	ID            string   `json:"id,omitempty"`
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

// StatisticMeta is one statistics metadata entry from the recorder's
// statistics_meta table, as returned by recorder/list_statistic_ids.
// HA >= 2024.11 returns mean_type (StatisticMeanType: NONE=0, ARITHMETIC=1,
// CIRCULAR=2) and deprecated has_mean; both are kept optional so old and new
// HA versions parse into the same struct.
type StatisticMeta struct {
	StatisticID    string  `json:"statistic_id"`
	DisplayUnit    *string `json:"display_unit_of_measurement,omitempty"`
	StatisticsUnit *string `json:"statistics_unit_of_measurement,omitempty"`
	UnitClass      *string `json:"unit_class,omitempty"`
	HasMean        *bool   `json:"has_mean,omitempty"` // pre-2026.4 HA versions
	MeanType       *int    `json:"mean_type,omitempty"`
	HasSum         *bool   `json:"has_sum,omitempty"`
	Name           *string `json:"name,omitempty"`
	Source         string  `json:"source"`
}

// StatisticValidationIssue is one recorder statistics validation issue, as
// returned by recorder/validate_statistics. Issue type "no_state" marks a
// statistic whose backing entity no longer exists (an orphaned statistic).
type StatisticValidationIssue struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
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
	// ContextEntityID is the entity whose activity caused this entry (HA's
	// context_entity_id). The object-level access layer drops entries whose
	// cause is hidden; it is not part of any tool output.
	ContextEntityID string `json:"context_entity_id,omitempty"`
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
	LastStep    *bool              `json:"last_step,omitempty"`
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
	LastStep    *bool              `json:"last_step,omitempty"`
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
	Name        string             `json:"name"`
	Type        string             `json:"type,omitempty"` // "expandable" for sections
	Required    bool               `json:"required,omitempty"`
	Optional    bool               `json:"optional,omitempty"`
	Default     any                `json:"default,omitempty"`
	Selector    map[string]any     `json:"selector,omitempty"`    // e.g. {"duration":{...}}, {"entity":{"read_only":true}}
	Schema      []OptionsFlowField `json:"schema,omitempty"`      // section children (HA nests sections one level deep)
	Description map[string]any     `json:"description,omitempty"` // Contains "suggested_value"
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
