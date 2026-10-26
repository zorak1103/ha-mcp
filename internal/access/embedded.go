package access

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Call is one service call that an automation, script or scene definition
// would make. ExtractCalls is for those definition shapes only; callers must
// not pass other shapes (a template helper keeps its actions under its own
// keys, which ExtractCalls does not walk, so it would yield no calls and no
// error): ExtractCallsFromActionKeys is the entry point for those.
//
// Targets holds the objects named by the node's target, its top-level id keys,
// the id keys of its data and, for services that carry the object id in their
// data (ServiceClass.ObjectFromData), that object. EntityData holds the
// entities named under the service's EntityDataKeys (see ClassifyService).
// DataMalformed is always false in a result of ExtractCalls, which returns an
// error instead; the field exists so a caller never has to assume that.
//
// EntityRegistryIDs holds entity-registry ids (32 lowercase hex characters, not
// entity ids) that a device action names in its entity_id field. The caller
// MUST resolve each through the entity registry to an entity id and refuse the
// definition when one cannot be resolved.
//
// Data is the node's service data: a shallow copy of its "data" map with the
// legacy "data_template" map merged over it (nil when the node has neither).
// No key at any depth of either map, and no key of the node itself, holds a
// template: HA renders keys too, so a templated key could become a target key
// at run time, and ExtractCalls refuses it.
//
// Raw is the canonical JSON encoding of the action node (object keys sorted)
// and is the identity NewCalls compares by. A device action is modeled as a
// call whose Domain is the device action's domain, whose Service is
// deviceActionService and whose Targets.DeviceIDs names the device.
type Call struct {
	Domain            string
	Service           string
	Targets           TargetSpec
	EntityData        []string
	EntityRegistryIDs []string
	DataMalformed     bool
	Data              map[string]any
	Raw               string
}

const (
	// maxActionDepth bounds how deep action nodes may nest so a hostile
	// definition cannot exhaust the stack.
	maxActionDepth = 32

	deviceActionService = DeviceActionService

	keyService   = "service"
	keyAction    = "action"
	keyActions   = "actions"
	keySequence  = "sequence"
	keyData      = "data"
	keyScene     = "scene"
	keyEvent     = "event"
	keyDeviceID  = "device_id"
	keyDomain    = "domain"
	keyType      = "type"
	keyEntityID  = "entity_id"
	keyCondition = "condition"
	keyIf        = "if"
	keyThen      = "then"
	keyElse      = "else"
	keyChoose    = "choose"
	keyDefault   = "default"
	keyRepeat    = "repeat"
	keyParallel  = "parallel"
	keyBlueprint = "use_blueprint"

	sceneEntityPrefix = "scene."
)

// DeviceActionService is the Service of a Call that models a device action.
const DeviceActionService = "device_action"

// templateMarkers open a Jinja expression or statement.
var templateMarkers = []string{"{{", "{%", "{#"}

// noCallKeys mark action nodes that make no service call.
var noCallKeys = []string{
	"delay", "wait_template", "wait_for_trigger", "variables", "stop",
	"set_conversation_response", keyCondition,
}

// templateOnlyKeys are legacy keys that carry a template in place of a value;
// their presence makes a node undecidable.
var templateOnlyKeys = []string{"service_template", "action_template"}

// targetFieldKeys are the call keys that select objects; a template in any of
// them cannot be resolved at save time.
var targetFieldKeys = []string{keyEntityID, keyDeviceID, "area_id", "floor_id", "label_id", targetNestedKey}

// ExtractCalls returns every service call the definition would make. An error
// means the definition cannot be fully decided (a template in a service name or
// target, a blueprint instance, an unrecognized action node, malformed targets,
// excessive nesting) and the caller must refuse it.
func ExtractCalls(def map[string]any) ([]Call, error) {
	if _, ok := def[keyBlueprint]; ok {
		return nil, errors.New("definition uses a blueprint, its service calls cannot be determined")
	}
	e := &extractor{}
	for _, key := range []string{keyActions, keyAction, keySequence} {
		node, ok := def[key]
		if !ok || node == nil {
			continue
		}
		if err := e.walkNode(node, 0); err != nil {
			return nil, err
		}
	}
	return e.calls, nil
}

// ExtractCallsFromActionKeys returns every service call made by the action
// lists stored under keys of def, the shape of a template helper (turn_on,
// press, set_value, ...). Each present, non-null value is walked exactly like
// an automation's action list, with the same fail-closed rules: an error means
// the actions cannot be fully decided and the caller must refuse. Keys of def
// that are not named are not looked at; the caller decides which keys hold
// actions.
func ExtractCallsFromActionKeys(def map[string]any, keys []string) ([]Call, error) {
	e := &extractor{}
	for _, key := range keys {
		node, ok := def[key]
		if !ok || node == nil {
			continue
		}
		if err := e.walkNode(node, 0); err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
	}
	return e.calls, nil
}

// NewCalls returns the calls in newDef that oldDef does not already contain,
// comparing by Raw as a multiset: reordering untouched calls yields none, adding
// a second copy of an existing call yields one.
func NewCalls(oldDef, newDef []Call) []Call {
	seen := make(map[string]int, len(oldDef))
	for _, c := range oldDef {
		seen[c.Raw]++
	}
	var added []Call
	for _, c := range newDef {
		if seen[c.Raw] > 0 {
			seen[c.Raw]--
			continue
		}
		added = append(added, c)
	}
	return added
}

type extractor struct {
	calls []Call
}

// walkNode visits an action node: a map, or a list (of maps and lists).
func (e *extractor) walkNode(node any, depth int) error {
	if depth > maxActionDepth {
		return errors.New("definition nests actions too deeply to analyze")
	}
	switch n := node.(type) {
	case map[string]any:
		return e.walkMap(n, depth)
	case []any:
		for _, item := range n {
			if err := e.walkNode(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case []map[string]any:
		for _, item := range n {
			if err := e.walkNode(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unrecognized action node of type %T", node)
	}
}

// walkMap handles one action node. Every recognized key is processed, so a node
// that mixes keys cannot hide a call behind another one; a node with no
// recognized key is undecidable.
func (e *extractor) walkMap(m map[string]any, depth int) error {
	for _, key := range templateOnlyKeys {
		if _, ok := m[key]; ok {
			return fmt.Errorf("action node uses %s, its service cannot be determined", key)
		}
	}
	steps := []func(map[string]any, int) (bool, error){
		e.serviceNode, e.sceneNode, e.eventNode, e.deviceNode, noCallNode, e.containerNode,
	}
	recognized := false
	for _, step := range steps {
		ok, err := step(m, depth)
		if err != nil {
			return err
		}
		recognized = recognized || ok
	}
	if !recognized {
		return fmt.Errorf("unrecognized action node with keys %v", sortedKeys(m))
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func noCallNode(m map[string]any, _ int) (bool, error) {
	for _, key := range noCallKeys {
		if _, ok := m[key]; ok {
			return true, nil
		}
	}
	return false, nil
}

// serviceNode handles the service/action keys. A string value is a service
// call; a non-string "action" value is a nested node list (legacy automations).
func (e *extractor) serviceNode(m map[string]any, depth int) (bool, error) {
	recognized := false
	for _, key := range []string{keyService, keyAction} {
		v, ok := m[key]
		if !ok {
			continue
		}
		recognized = true
		name, isStr := v.(string)
		switch {
		case isStr:
			if err := e.addServiceCall(m, name); err != nil {
				return true, err
			}
		case key == keyAction:
			if err := e.walkNode(v, depth+1); err != nil {
				return true, err
			}
		default:
			return true, fmt.Errorf("service name is a %T, not a string", v)
		}
	}
	return recognized, nil
}

func (e *extractor) addServiceCall(node map[string]any, name string) error {
	domain, service, err := parseServiceName(name)
	if err != nil {
		return err
	}
	call := Call{Domain: domain, Service: service}
	if err := fillTargets(&call, node); err != nil {
		return fmt.Errorf("%s.%s: %w", domain, service, err)
	}
	return e.appendCall(call, node)
}

func (e *extractor) appendCall(call Call, node map[string]any) error {
	raw, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("encoding action node: %w", err)
	}
	call.Raw = string(raw)
	e.calls = append(e.calls, call)
	return nil
}

func parseServiceName(name string) (domain, service string, err error) {
	if hasTemplate(name) {
		return "", "", errors.New("service name is a template")
	}
	domain, service, found := strings.Cut(strings.ToLower(strings.TrimSpace(name)), ".")
	if !found || !isSlug(domain) || !isSlug(service) {
		return "", "", fmt.Errorf("service name %q is not domain.service", name)
	}
	return domain, service, nil
}

// isSlug reports whether s is non-empty and made of [a-z0-9_] only.
func isSlug(s string) bool {
	return s != "" && !strings.ContainsFunc(s, func(r rune) bool {
		return !isSlugRune(r)
	})
}

func isSlugRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_'
}

// isEntityID reports whether s has the shape [a-z0-9_]+.[a-z0-9_]+.
func isEntityID(s string) bool {
	domain, object, found := strings.Cut(s, ".")
	return found && isSlug(domain) && isSlug(object)
}

// isRefID reports whether s is non-empty and made of [a-z0-9_-] only, the
// shape of area, floor, label and device ids.
func isRefID(s string) bool {
	return s != "" && !strings.ContainsFunc(s, func(r rune) bool {
		return !isSlugRune(r) && r != '-'
	})
}

// isRegistryID reports whether s is an entity-registry id: 32 lowercase hex
// characters.
func isRegistryID(s string) bool {
	const registryIDLen = 32
	if len(s) != registryIDLen {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool {
		return (r < '0' || r > '9') && (r < 'a' || r > 'f')
	})
}

// validateTargets fails closed on any id that does not have the shape HA
// itself would use, so no id can hide behind characters the policy globs would
// not see. Entity ids must be domain.object slugs; the literal "all" is already
// folded into TargetSpec.All by ParseTarget.
func validateTargets(t TargetSpec) error {
	for _, id := range t.EntityIDs {
		if !isEntityID(id) {
			return fmt.Errorf("entity id %q has an unsupported form", id)
		}
	}
	groups := [][]string{t.DeviceIDs, t.AreaIDs, t.FloorIDs, t.LabelIDs}
	for _, ids := range groups {
		for _, id := range ids {
			if !isRefID(id) {
				return fmt.Errorf("id %q has an unsupported form", id)
			}
		}
	}
	return nil
}

// fillTargets resolves the objects of a service call from the node itself
// (top-level id keys and "target"), "data" and the legacy "data_template".
func fillTargets(call *Call, node map[string]any) error {
	if err := rejectTemplatedFields(node, nil); err != nil {
		return err
	}
	if templatedKey(node, 0) {
		return errors.New("a key of the action is a template")
	}
	mergeTargets(&call.Targets, ParseTarget(node))
	class := ClassifyService(call.Domain, call.Service)
	objectFound := false
	for _, dataKey := range []string{keyData, "data_template"} {
		data, present, err := dataMap(node, dataKey)
		if err != nil {
			return err
		}
		if !present {
			continue
		}
		if call.Data == nil {
			call.Data = make(map[string]any, len(data))
		}
		maps.Copy(call.Data, data)
		found, err := fillFromData(call, data, class)
		if err != nil {
			return err
		}
		objectFound = objectFound || found
	}
	if class.ObjectFromData != "" && !objectFound {
		return fmt.Errorf("%s is missing", class.ObjectFromData)
	}
	return validateCallIDs(call)
}

// validateCallIDs checks the ids a call resolved to.
func validateCallIDs(call *Call) error {
	if call.Targets.Malformed {
		return errors.New("target is malformed")
	}
	if err := validateTargets(call.Targets); err != nil {
		return err
	}
	for _, id := range call.EntityData {
		if !isEntityID(id) {
			return fmt.Errorf("entity id %q has an unsupported form", id)
		}
	}
	return nil
}

// fillFromData merges the targets, entity-data ids and object id found in one
// data map into call. objectFound reports whether the service's ObjectFromData
// key was present.
func fillFromData(call *Call, data map[string]any, class ServiceClass) (objectFound bool, err error) {
	if err := rejectTemplatedFields(data, class.EntityDataKeys); err != nil {
		return false, err
	}
	mergeTargets(&call.Targets, ParseTarget(data))
	ids, malformed := EntityIDsFromData(data, class.EntityDataKeys)
	if malformed {
		return false, errors.New("entity data is malformed")
	}
	call.EntityData = mergeIDs(call.EntityData, ids)
	if class.ObjectFromData == "" {
		return false, nil
	}
	raw, ok := data[class.ObjectFromData]
	if !ok {
		return false, nil
	}
	object, isStr := raw.(string)
	object = strings.ToLower(strings.TrimSpace(object))
	if !isStr || hasTemplate(object) || !isSlug(object) {
		return true, fmt.Errorf("%s is not a plain object id", class.ObjectFromData)
	}
	call.Targets.EntityIDs = mergeIDs(call.Targets.EntityIDs, []string{call.Domain + "." + object})
	return true, nil
}

// dataMap returns node[key] as a map; present is false when the key is absent
// or null. A value of any other type (including a template string) is
// undecidable.
func dataMap(node map[string]any, key string) (m map[string]any, present bool, err error) {
	v, ok := node[key]
	if !ok || v == nil {
		return nil, false, nil
	}
	m, isMap := v.(map[string]any)
	if !isMap {
		return nil, false, fmt.Errorf("%s is a %T, not a mapping", key, v)
	}
	return m, true, nil
}

func mergeTargets(dst *TargetSpec, src TargetSpec) {
	dst.EntityIDs = mergeIDs(dst.EntityIDs, src.EntityIDs)
	dst.DeviceIDs = mergeIDs(dst.DeviceIDs, src.DeviceIDs)
	dst.AreaIDs = mergeIDs(dst.AreaIDs, src.AreaIDs)
	dst.FloorIDs = mergeIDs(dst.FloorIDs, src.FloorIDs)
	dst.LabelIDs = mergeIDs(dst.LabelIDs, src.LabelIDs)
	dst.All = dst.All || src.All
	dst.Malformed = dst.Malformed || src.Malformed
}

func mergeIDs(dst, src []string) []string {
	for _, id := range src {
		if !slices.Contains(dst, id) {
			dst = append(dst, id)
		}
	}
	return dst
}

// rejectTemplatedFields fails when a target field of m, or an entity-data key
// listed in entityDataKeys, holds a template.
func rejectTemplatedFields(m map[string]any, entityDataKeys []string) error {
	for _, key := range targetFieldKeys {
		if v, ok := m[key]; ok && containsTemplate(v) {
			return fmt.Errorf("%s is a template", key)
		}
	}
	for _, key := range entityDataKeys {
		if v, ok := m[key]; ok && entityDataHasTemplate(v) {
			return fmt.Errorf("%s is a template", key)
		}
	}
	return nil
}

// templatedKey reports whether any map key inside v (at any depth, through maps
// and lists) holds a template marker. HA renders keys of service data too, so
// such a key could turn into a target key at run time. Nesting deeper than
// maxActionDepth counts as templated (fail closed).
func templatedKey(v any, depth int) bool {
	if depth > maxActionDepth {
		return true
	}
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if hasTemplate(k) || templatedKey(val, depth+1) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(x, func(e any) bool { return templatedKey(e, depth+1) })
	case []map[string]any:
		return slices.ContainsFunc(x, func(e map[string]any) bool { return templatedKey(e, depth+1) })
	case map[any]any:
		return anyMapHasTemplate(x)
	}
	return false
}

// entityDataHasTemplate is containsTemplate for an entity-data value, where a
// map contributes its keys (the entity ids) but not its values (the states).
func entityDataHasTemplate(v any) bool {
	m, isMap := v.(map[string]any)
	if !isMap {
		return containsTemplate(v)
	}
	for k := range m {
		if hasTemplate(k) {
			return true
		}
	}
	return false
}

func hasTemplate(s string) bool {
	for _, marker := range templateMarkers {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

// ContainsTemplate reports whether any string inside v, or any map key, holds a
// Jinja template marker ({{, {% or {#). It fails closed: a value of a type it
// does not know how to inspect counts as a template.
func ContainsTemplate(v any) bool { return containsTemplate(v) }

// containsTemplate reports whether any string inside v, or any map key, holds
// a template marker. It fails closed: a value of a type it does not know how to
// inspect counts as a template.
func containsTemplate(v any) bool {
	switch x := v.(type) {
	case nil, bool, int, int64, float64, json.Number:
		return false
	case string:
		return hasTemplate(x)
	case []string:
		return slices.ContainsFunc(x, hasTemplate)
	case []any:
		return slices.ContainsFunc(x, containsTemplate)
	case []map[string]any:
		for _, m := range x {
			if containsTemplate(m) {
				return true
			}
		}
		return false
	case map[string]any:
		return stringMapHasTemplate(x)
	case map[any]any:
		return anyMapHasTemplate(x)
	default:
		return true
	}
}

func stringMapHasTemplate(m map[string]any) bool {
	for k, val := range m {
		if hasTemplate(k) || containsTemplate(val) {
			return true
		}
	}
	return false
}

// anyMapHasTemplate treats a non-string key as a template: it cannot be an id.
func anyMapHasTemplate(m map[any]any) bool {
	for k, val := range m {
		if _, isStr := k.(string); !isStr || containsTemplate(k) || containsTemplate(val) {
			return true
		}
	}
	return false
}

// sceneNode handles the "scene: x" shorthand, a scene.turn_on on that scene.
func (e *extractor) sceneNode(m map[string]any, _ int) (bool, error) {
	v, ok := m[keyScene]
	if !ok {
		return false, nil
	}
	name, isStr := v.(string)
	if !isStr || hasTemplate(name) {
		return true, errors.New("scene shorthand is not a plain scene id")
	}
	var ids []string
	for _, id := range splitIDs(name) {
		if !strings.HasPrefix(id, sceneEntityPrefix) {
			id = sceneEntityPrefix + id
		}
		if !isEntityID(id) {
			return true, fmt.Errorf("scene %q has an unsupported form", id)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return true, errors.New("scene shorthand names no scene")
	}
	call := Call{Domain: "scene", Service: "turn_on", Targets: TargetSpec{EntityIDs: ids}}
	return true, e.appendCall(call, m)
}

// eventNode handles "event:", a homeassistant.event call without objects.
func (e *extractor) eventNode(m map[string]any, _ int) (bool, error) {
	v, ok := m[keyEvent]
	if !ok {
		return false, nil
	}
	if _, isStr := v.(string); !isStr {
		return true, errors.New("event type is not a string")
	}
	return true, e.appendCall(Call{Domain: "homeassistant", Service: keyEvent}, m)
}

// deviceNode handles a device action (device_id + domain + type). A node with a
// "condition" key is a device condition and makes no call.
func (e *extractor) deviceNode(m map[string]any, _ int) (bool, error) {
	_, hasCondition := m[keyCondition]
	if hasCondition || !hasKeys(m, keyDeviceID, keyDomain, keyType) {
		return false, nil
	}
	domain, isStr := m[keyDomain].(string)
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !isStr || !isSlug(domain) {
		return true, errors.New("device action domain is not a plain domain")
	}
	if err := rejectTemplatedFields(m, nil); err != nil {
		return true, err
	}
	if templatedKey(m, 0) {
		return true, errors.New("a key of the device action is a template")
	}
	t := ParseTarget(map[string]any{keyDeviceID: m[keyDeviceID]})
	if t.Malformed || (len(t.DeviceIDs) == 0 && !t.All) {
		return true, errors.New("device action names no usable device")
	}
	call := Call{Domain: domain, Service: deviceActionService, Targets: t}
	if err := fillDeviceEntity(&call, m); err != nil {
		return true, err
	}
	if err := validateCallIDs(&call); err != nil {
		return true, err
	}
	return true, e.appendCall(call, m)
}

// fillDeviceEntity reads the optional entity_id of a device action: a real
// entity id goes to Targets.EntityIDs, an entity-registry id to
// EntityRegistryIDs; anything else is undecidable.
func fillDeviceEntity(call *Call, m map[string]any) error {
	v, ok := m[keyEntityID]
	if !ok {
		return nil
	}
	s, isStr := v.(string)
	id := strings.ToLower(strings.TrimSpace(s))
	switch {
	case !isStr || hasTemplate(id):
		return errors.New("device action entity_id is not a plain id")
	case isEntityID(id):
		call.Targets.EntityIDs = []string{id}
	case isRegistryID(id):
		call.EntityRegistryIDs = []string{id}
	default:
		return fmt.Errorf("device action entity_id %q is neither an entity id nor a registry id", id)
	}
	return nil
}

func hasKeys(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

// containerNode walks the nested action lists of if/then/else, choose/default,
// repeat, parallel and sequence nodes. Conditions hold no calls and are skipped.
func (e *extractor) containerNode(m map[string]any, depth int) (bool, error) {
	_, hasIf := m[keyIf]
	_, hasChoose := m[keyChoose]
	_, hasRepeat := m[keyRepeat]
	_, hasParallel := m[keyParallel]
	_, hasSequence := m[keySequence]
	recognized := hasIf || hasChoose || hasRepeat || hasParallel || hasSequence

	for _, key := range []string{keyThen, keyElse} {
		if err := e.walkKey(m, key, depth); err != nil {
			return recognized, err
		}
	}
	if hasChoose {
		if err := e.walkChoose(m[keyChoose], depth); err != nil {
			return true, err
		}
	}
	if hasRepeat {
		if err := e.walkRepeat(m[keyRepeat], depth); err != nil {
			return true, err
		}
	}
	for _, key := range []string{keyDefault, keyParallel, keySequence} {
		if err := e.walkKey(m, key, depth); err != nil {
			return recognized, err
		}
	}
	return recognized, nil
}

// walkKey walks m[key] as an action node when the key is present.
func (e *extractor) walkKey(m map[string]any, key string, depth int) error {
	v, ok := m[key]
	if !ok {
		return nil
	}
	return e.walkNode(v, depth+1)
}

// walkChoose walks the sequence of every choose option (a map or a list of maps).
func (e *extractor) walkChoose(v any, depth int) error {
	var options []any
	switch x := v.(type) {
	case map[string]any:
		options = []any{x}
	case []any:
		options = x
	case []map[string]any:
		for _, o := range x {
			options = append(options, o)
		}
	default:
		return fmt.Errorf("choose is a %T, not a list of options", v)
	}
	for _, opt := range options {
		m, isMap := opt.(map[string]any)
		if !isMap {
			return fmt.Errorf("choose option is a %T, not a mapping", opt)
		}
		if err := e.walkKey(m, keySequence, depth); err != nil {
			return err
		}
	}
	return nil
}

func (e *extractor) walkRepeat(v any, depth int) error {
	m, isMap := v.(map[string]any)
	if !isMap {
		return fmt.Errorf("repeat is a %T, not a mapping", v)
	}
	return e.walkKey(m, keySequence, depth)
}
