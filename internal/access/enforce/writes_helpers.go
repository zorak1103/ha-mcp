package enforce

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Helper writes. Two kinds of helpers exist: WebSocket helpers (input_*,
// counter, timer, schedule; homeassistant.IsWSHelperPlatform) and Config Entry
// helpers (template, group, threshold, ...; homeassistant.RequiresConfigEntryFlow).
// A platform that is neither is refused: the real client would send it as a
// "<platform>/create" WebSocket command.
//
// Create is a create on the entity id HA will assign (helperCreateGrant),
// decided before anything about existing objects is looked at (R27); only a
// caller holding configure and create there learns that the id is taken
// (errUnavailable). Platforms whose entity ids cannot be predicted
// (switch_as_x, a utility_meter with tariffs) need configure across every
// domain they create entities in (Policy.DomainWide). Then:
//
//   - every source entity the config names (helperSourceKeys) needs control,
//     a hidden or (when the call requires it) nonexistent one answers as a
//     missing entity, like a direct service call;
//   - a device_id attaches the new entity to a device, so every entity of
//     that device needs configure (as for a device edit);
//   - a template helper's action lists are checked like an automation's
//     actions (ExtractCallsFromActionKeys); on update only the calls the stored
//     options do not already make;
//   - under a policy that hides anything, no value may contain a template: a
//     template can read any state, so a helper built from one would publish
//     hidden states (the RenderTemplate rule).
//
// Update and delete need configure (and delete) on the helper's entity; a
// hidden helper, and under a hiding policy one that does not exist, answers
// with the client's missing-helper error (errHelperMissing). A Config Entry
// helper's update rewrites, and its delete removes, the whole config entry, so
// the entity must belong to a helper platform and every entity of that entry
// needs the same rights (refused generically otherwise, hidden ones included).
//
// Every write that reaches inner drops the snapshot, failed or not.

const (
	methodCreateHelper       = "CreateHelper"
	methodCreateHelperEntity = "CreateHelperEntity"
	methodUpdateHelper       = "UpdateHelper"
	methodDeleteHelper       = "DeleteHelper"

	platformTemplate     = "template"
	platformGroup        = "group"
	platformRandom       = "random"
	platformSwitchAsX    = "switch_as_x"
	platformUtilityMeter = "utility_meter"

	keyName       = "name"
	keyGroupType  = "group_type"
	keyDeviceID   = "device_id"
	keyTariffs    = "tariffs"
	keyTargetDom  = "target_domain"
	domainSensor  = "sensor"
	domainSelect  = "select"
	keyFanSpeeds  = "fan_speeds"
	keyAdvancedOp = "advanced_options"
)

// helperSourceKeys are the config keys that name source entities (a string or
// a list of strings), from the manage_helper builders and HA's helper flows.
var helperSourceKeys = []string{
	"entity_id", "entity_ids", "entities", "source",
	"heater", "heater_entity_id", "humidifier", "humidifier_entity_id",
	"target_sensor", "target_sensor_entity_id",
}

// menuPlatforms are the Config Entry platforms whose flow menu (group_type
// first) picks the entity domain.
var menuPlatforms = map[string]bool{platformTemplate: true, platformGroup: true, platformRandom: true}

func (c *client) CreateHelper(ctx context.Context, cfg homeassistant.HelperConfig) error {
	predicted, err := c.checkHelperCreate(ctx, methodCreateHelper, cfg)
	if err != nil {
		return err
	}
	err = c.callAndInvalidate(func() error { return c.inner.CreateHelper(ctx, cfg) })
	c.created.add(predicted)
	return err
}

func (c *client) CreateHelperEntity(ctx context.Context, cfg homeassistant.HelperConfig) (string, error) {
	predicted, err := c.checkHelperCreate(ctx, methodCreateHelperEntity, cfg)
	if err != nil {
		return "", err
	}
	id, err := callValueAndInvalidate(c, func() (string, error) { return c.inner.CreateHelperEntity(ctx, cfg) })
	c.created.add(predicted)
	if predicted != "" && id != "" && id != predicted {
		// HA assigned another id than the one decided on (a race with a
		// concurrent create). It cannot be undone here; later writes to it
		// are decided on their own.
		slog.Warn("helper created under an unexpected entity id", "decided", predicted, "assigned", id)
	}
	return id, err
}

func (c *client) UpdateHelper(ctx context.Context, helperID string, cfg homeassistant.HelperConfig) error {
	if err := c.checkHelperUpdate(ctx, helperID, cfg); err != nil {
		return err
	}
	return c.callAndInvalidate(func() error { return c.inner.UpdateHelper(ctx, helperID, cfg) })
}

func (c *client) DeleteHelper(ctx context.Context, helperID string) error {
	if err := c.checkHelperDelete(ctx, helperID); err != nil {
		return err
	}
	return c.callAndInvalidate(func() error { return c.inner.DeleteHelper(ctx, helperID) })
}

// callValueAndInvalidate is callAndInvalidate for a call with a result.
func callValueAndInvalidate[T any](c *client, call func() (T, error)) (T, error) {
	v, err := call()
	c.snap.invalidate()
	return v, err
}

// checkHelperCreate decides a helper create and returns the predicted entity
// id ("" for an unpredictable platform). It returns an error only to refuse.
func (c *client) checkHelperCreate(ctx context.Context, method string, cfg homeassistant.HelperConfig) (string, error) {
	if !homeassistant.IsWSHelperPlatform(cfg.Platform) && !homeassistant.RequiresConfigEntryFlow(cfg.Platform) {
		return "", c.refuse(method)
	}
	conf, ok := c.helperConfig(cfg.Config)
	if !ok {
		return "", c.refuse(method)
	}
	predicted, err := c.helperCreateGrant(method, cfg.Platform, conf)
	if err != nil {
		return "", err
	}
	owner := predicted
	if owner == "" {
		owner = method
	}
	if err := c.checkHelperContent(ctx, method, owner, cfg.Platform, conf, nil, false); err != nil {
		return "", err
	}
	if predicted == "" {
		return "", nil
	}
	return predicted, c.checkHelperFree(ctx, method, cfg.Platform, predicted)
}

// helperConfig returns the config as plain JSON values (so every check sees
// the shapes HA will see), or false when it cannot be encoded or, under a
// hiding policy, contains a template.
func (c *client) helperConfig(conf map[string]any) (map[string]any, bool) {
	out, ok := plainJSON(conf)
	if !ok {
		return nil, false
	}
	if c.p.HidesAnything() && access.ContainsTemplate(out) {
		return nil, false
	}
	return out, true
}

// helperCreateGrant decides the create right before anything else (R27).
func (c *client) helperCreateGrant(method, platform string, conf map[string]any) (string, error) {
	if domains, unpredictable := unpredictableDomains(platform, conf); unpredictable {
		if len(domains) == 0 {
			return "", c.refuse(method)
		}
		for _, d := range domains {
			if !c.p.DomainWide(d, access.CapConfigure) {
				return "", c.refuse(method)
			}
		}
		return "", nil
	}
	predicted := predictHelperEntityID(platform, conf)
	if predicted == "" {
		return "", c.refuse(method)
	}
	g := c.p.Decide(access.Object{Kind: access.KindEntity, ID: predicted})
	switch {
	case g.Hidden():
		return "", c.refuse(method)
	case !g.Configure:
		return "", c.denyObject(predicted, access.CapConfigure)
	case !g.Create:
		return "", c.denyObject(predicted, capCreate)
	}
	return predicted, nil
}

// unpredictableDomains reports whether the platform's entity ids cannot be
// predicted and, if so, the domains it creates entities in (nil when even
// those cannot be told).
func unpredictableDomains(platform string, conf map[string]any) ([]string, bool) {
	switch platform {
	case platformSwitchAsX:
		d, ok := conf[keyTargetDom].(string)
		if !ok || !slugRE.MatchString(d) {
			return nil, true
		}
		return []string{d}, true
	case platformUtilityMeter:
		if tariffs, ok := conf[keyTariffs].([]any); ok && len(tariffs) > 0 {
			return []string{domainSensor, domainSelect}, true
		}
		if v, present := conf[keyTariffs]; present && v != nil && !isEmptyList(v) {
			return nil, true
		}
	}
	return nil, false
}

func isEmptyList(v any) bool {
	l, ok := v.([]any)
	return ok && len(l) == 0
}

// predictHelperEntityID is the entity id HA assigns a new helper:
// <domain>.<slug(name)>, the domain being the platform for a WebSocket helper
// and the one the real client predicts for a Config Entry helper. A group_type
// overrides the domain of menu platforms, so it must agree with the
// prediction; on other platforms it is refused. Names outside the ASCII-safe
// set are refused (R28).
func predictHelperEntityID(platform string, conf map[string]any) string {
	name, ok := conf[keyName].(string)
	if !ok || !asciiSafeName(name) {
		return ""
	}
	domain := platform
	if !homeassistant.IsWSHelperPlatform(platform) {
		domain = homeassistant.ConfigEntryEntityDomain(homeassistant.HelperConfig{Platform: platform, Config: conf})
	}
	if gt, present := conf[keyGroupType]; present {
		if s, isStr := gt.(string); !isStr || !menuPlatforms[platform] || s != domain {
			return ""
		}
	}
	object := homeassistant.Slugify(name)
	id := domain + "." + object
	if object == "" || !isPlainEntityID(id) {
		return ""
	}
	return id
}

// checkHelperFree refuses a create whose predicted id is already taken (HA
// would pick <id>_2, an entity nobody decided on): by an entity, or for a
// WebSocket helper by a stored helper whose id is that slug.
func (c *client) checkHelperFree(ctx context.Context, method, platform, predicted string) error {
	taken, err := c.entityTaken(ctx, predicted)
	if err != nil {
		return c.undecidable(method, err)
	}
	if !taken && homeassistant.IsWSHelperPlatform(platform) {
		data, err := c.snap.get(ctx)
		if err != nil {
			return c.undecidable(method, err)
		}
		_, object, _ := strings.Cut(predicted, ".")
		taken = slices.ContainsFunc(data.entityIDs, func(id string) bool {
			e := data.entities[id]
			return e.Platform == platform && e.UniqueID == object
		})
	}
	if taken {
		return errUnavailable(predicted)
	}
	return nil
}

// checkHelperContent applies the source, device and template-action rules to
// a helper config. owner names the helper in denial logs. For an update,
// stored holds the calls the stored options make (hasStored false: check all).
func (c *client) checkHelperContent(
	ctx context.Context, method, owner, platform string, conf map[string]any, stored []access.Call, hasStored bool,
) error {
	if err := c.checkHelperSources(ctx, method, conf); err != nil {
		return err
	}
	if err := c.checkHelperDevice(ctx, method, conf); err != nil {
		return err
	}
	if platform != platformTemplate {
		return nil
	}
	return c.checkTemplateActions(ctx, owner, conf, stored, hasStored)
}

// checkHelperSources requires control on every source entity of conf.
func (c *client) checkHelperSources(ctx context.Context, method string, conf map[string]any) error {
	ids, ok := helperSourceIDs(conf)
	if !ok {
		return c.refuse(method)
	}
	if len(ids) == 0 {
		return nil
	}
	req := callRequest{service: method, targets: access.TargetSpec{EntityIDs: ids}, mustExist: true}
	return c.checkObjects(ctx, req, access.CapControl)
}

// helperSourceIDs collects the source entity ids of conf. ok is false for a
// source key of an unsupported shape, or a key that looks like it names
// entities but is not a known source key.
func helperSourceIDs(conf map[string]any) ([]string, bool) {
	for k := range conf {
		if strings.Contains(k, "entit") && !slices.Contains(helperSourceKeys, k) {
			return nil, false
		}
	}
	var ids []string
	for _, key := range helperSourceKeys {
		v, present := conf[key]
		if !present || v == nil {
			continue
		}
		vals, ok := stringList(v)
		if !ok {
			return nil, false
		}
		ids = append(ids, vals...)
	}
	return ids, true
}

// stringList returns v as a list of strings: a string or a list of strings.
func stringList(v any) ([]string, bool) {
	switch x := v.(type) {
	case string:
		return []string{x}, true
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			s, ok := e.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

// checkHelperDevice requires configure on every entity of the device a helper
// is attached to (device_id); an unknown or undecidable device refuses.
func (c *client) checkHelperDevice(ctx context.Context, method string, conf map[string]any) error {
	v, present := conf[keyDeviceID]
	if !present || v == nil || v == "" {
		return nil
	}
	deviceID, ok := v.(string)
	if !ok {
		return c.refuse(method)
	}
	return c.checkDeviceEntities(ctx, method, deviceID, false)
}

// checkTemplateActions checks the calls a template helper's action keys make.
func (c *client) checkTemplateActions(ctx context.Context, owner string, conf map[string]any, stored []access.Call, hasStored bool) error {
	keys, err := templateActionKeys(conf)
	if err != nil {
		return c.embeddedRefusal(owner, err.Error())
	}
	calls, err := access.ExtractCallsFromActionKeys(conf, keys)
	if err != nil {
		return c.embeddedRefusal(owner, err.Error())
	}
	if hasStored {
		calls = access.NewCalls(stored, calls)
	}
	for _, call := range calls {
		if err := c.checkEmbeddedCall(ctx, owner, call); err != nil {
			return err
		}
	}
	return nil
}

// templateActionKeys returns the keys of a template helper config that may
// hold actions: every key whose value is a mapping or a list, except the few
// known list or mapping fields that hold no actions (validated here). A known
// action key with any other value cannot be decided. Sorted for determinism.
func templateActionKeys(conf map[string]any) ([]string, error) {
	var keys []string
	for k, v := range conf {
		kind := reflect.ValueOf(v).Kind()
		composite := kind == reflect.Map || kind == reflect.Slice
		switch {
		case k == keyFanSpeeds && composite:
			if _, ok := stringList(v); !ok {
				return nil, fmt.Errorf("%s is not a list of strings", k)
			}
		case k == keyAdvancedOp && composite:
			if !scalarMap(v) {
				return nil, fmt.Errorf("%s holds more than plain values", k)
			}
		case composite:
			keys = append(keys, k)
		case v != nil && templateActionNames[k]:
			return nil, fmt.Errorf("%s is not an action list", k)
		}
	}
	slices.Sort(keys)
	return keys, nil
}

// scalarMap reports whether v is a mapping of strings, numbers, booleans and nulls.
func scalarMap(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for _, val := range m {
		switch val.(type) {
		case nil, string, float64, bool:
		default:
			return false
		}
	}
	return true
}

// templateActionNames are the action keys of HA's template platforms (the
// manage_helper template subtypes plus HA's other template action options).
var templateActionNames = map[string]bool{
	"turn_on": true, "turn_off": true, "press": true, "lock": true, "unlock": true, "open": true,
	"open_cover": true, "close_cover": true, "stop_cover": true, "set_cover_position": true,
	"set_cover_tilt_position": true, "open_cover_tilt": true, "close_cover_tilt": true,
	"set_value": true, "select_option": true, "set_percentage": true, "set_preset_mode": true,
	"set_oscillating": true, "set_direction": true, "set_level": true, "set_hs": true,
	"set_temperature": true, "set_rgb": true, "set_rgbw": true, "set_rgbww": true, "set_effect": true,
	"install": true, "start": true, "stop": true, "pause": true, "return_to_base": true,
	"clean_spot": true, "locate": true, "set_fan_speed": true, "disarm": true, "arm_away": true,
	"arm_custom_bypass": true, "arm_home": true, "arm_night": true, "arm_vacation": true, "trigger": true,
}

// checkHelperUpdate decides an UpdateHelper. The real client updates a Config
// Entry helper (registry entry with a config entry) through its options flow,
// anything else as the WebSocket helper <cfg.Platform>.<object id>.
func (c *client) checkHelperUpdate(ctx context.Context, helperID string, cfg homeassistant.HelperConfig) error {
	data, err := c.snap.get(ctx)
	if err != nil {
		return c.undecidable(methodUpdateHelper, err)
	}
	if e, ok := data.entity(helperID); ok && e.ConfigEntryID != "" && isPlainEntityID(helperID) {
		return c.checkConfigEntryHelperUpdate(ctx, data, e, cfg)
	}
	entityID, ok := wsHelperTarget(helperID, cfg.Platform)
	if !ok {
		return c.refuse(methodUpdateHelper)
	}
	g, err := c.helperGrant(ctx, methodUpdateHelper, entityID)
	if err != nil {
		return err
	}
	if err := c.helperRights(g, entityID, false); err != nil {
		return err
	}
	conf, ok := c.helperConfig(cfg.Config)
	if !ok {
		return c.refuse(methodUpdateHelper)
	}
	return c.checkHelperContent(ctx, methodUpdateHelper, entityID, cfg.Platform, conf, nil, false)
}

// wsHelperTarget is the entity a WebSocket helper update acts on: the
// platform's helper with helperID's object id. A prefixed id must carry the
// platform itself.
func wsHelperTarget(helperID, platform string) (string, bool) {
	if !homeassistant.IsWSHelperPlatform(platform) {
		return "", false
	}
	id := helperID
	if !strings.Contains(helperID, ".") {
		id = platform + "." + helperID
	}
	domain, _, _ := strings.Cut(id, ".")
	return id, domain == platform && isPlainEntityID(id)
}

// checkConfigEntryHelperUpdate decides the options-flow update of the config
// entry behind e.
func (c *client) checkConfigEntryHelperUpdate(ctx context.Context, data *snapData, e homeassistant.EntityRegistryEntry, cfg homeassistant.HelperConfig) error {
	entityID := strings.ToLower(e.EntityID)
	g, err := c.helperGrant(ctx, methodUpdateHelper, entityID)
	if err != nil {
		return err
	}
	if !homeassistant.RequiresConfigEntryFlow(e.Platform) || homeassistant.IsWSHelperPlatform(cfg.Platform) {
		return c.refuse(methodUpdateHelper)
	}
	if err := c.helperRights(g, entityID, false); err != nil {
		return err
	}
	if err := c.checkEntrySiblings(ctx, methodUpdateHelper, data, e.ConfigEntryID, false); err != nil {
		return err
	}
	conf, ok := c.helperConfig(cfg.Config)
	if !ok {
		return c.refuse(methodUpdateHelper)
	}
	var stored []access.Call
	hasStored := false
	if e.Platform == platformTemplate {
		stored, hasStored = c.storedTemplateCalls(ctx, e.ConfigEntryID)
	}
	return c.checkHelperContent(ctx, methodUpdateHelper, entityID, e.Platform, conf, stored, hasStored)
}

// storedTemplateCalls returns the calls of a template helper's stored options,
// read unfiltered from inner; ok is false when they cannot be read or analyzed
// (every new call is then checked).
func (c *client) storedTemplateCalls(ctx context.Context, entryID string) ([]access.Call, bool) {
	opts, err := c.inner.GetConfigEntryOptions(ctx, entryID)
	if err != nil {
		return nil, false
	}
	conf, ok := plainJSON(opts)
	if !ok {
		return nil, false
	}
	keys, err := templateActionKeys(conf)
	if err != nil {
		return nil, false
	}
	calls, err := access.ExtractCallsFromActionKeys(conf, keys)
	if err != nil {
		return nil, false
	}
	return calls, true
}

// plainJSON returns conf as plain JSON values (an empty map for nil), or
// false when it cannot be encoded.
func plainJSON(conf map[string]any) (map[string]any, bool) {
	out := map[string]any{}
	if conf == nil {
		return out, true
	}
	raw, err := json.Marshal(conf)
	if err != nil || json.Unmarshal(raw, &out) != nil {
		return nil, false
	}
	return out, true
}

// checkHelperDelete decides a DeleteHelper. A Config Entry helper's delete
// removes its whole config entry.
func (c *client) checkHelperDelete(ctx context.Context, helperID string) error {
	if !isPlainEntityID(helperID) {
		return c.refuse(methodDeleteHelper)
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return c.undecidable(methodDeleteHelper, err)
	}
	g, err := c.helperGrant(ctx, methodDeleteHelper, helperID)
	if err != nil {
		return err
	}
	e, ok := data.entity(helperID)
	entryHelper := ok && e.ConfigEntryID != ""
	domain, _, _ := strings.Cut(helperID, ".")
	if entryHelper && !homeassistant.RequiresConfigEntryFlow(e.Platform) || !entryHelper && !homeassistant.IsWSHelperPlatform(domain) {
		return c.refuse(methodDeleteHelper)
	}
	if err := c.helperRights(g, helperID, true); err != nil {
		return err
	}
	if entryHelper {
		return c.checkEntrySiblings(ctx, methodDeleteHelper, data, e.ConfigEntryID, true)
	}
	return nil
}

// helperGrant returns the grant on the helper entity of an update or delete;
// a hidden one, and under a hiding policy an unknown one, is answered as a
// missing helper.
func (c *client) helperGrant(ctx context.Context, method, entityID string) (access.Grant, error) {
	grant, err := c.grantFunc(ctx)
	if err != nil {
		return access.Grant{}, c.undecidable(method, err)
	}
	g := grant(entityID)
	exists := true
	if c.p.HidesAnything() {
		if exists, err = c.helperExists(ctx, entityID); err != nil {
			return access.Grant{}, c.undecidable(method, err)
		}
	}
	if g.Hidden() || !exists {
		return access.Grant{}, errHelperMissing(entityID)
	}
	return g, nil
}

// helperRights requires configure (and delete) on a visible helper entity.
func (c *client) helperRights(g access.Grant, entityID string, needDelete bool) error {
	switch {
	case !g.Configure:
		return c.denyObject(entityID, access.CapConfigure)
	case needDelete && !g.Delete:
		return c.denyObject(entityID, capDelete)
	}
	return nil
}

// helperExists reports whether entityID is registered, in the state machine,
// or was created by this request.
func (c *client) helperExists(ctx context.Context, entityID string) (bool, error) {
	if c.created.has(entityID) {
		return true, nil
	}
	return c.entityTaken(ctx, entityID)
}

// checkEntrySiblings requires configure (and delete) on every entity of the
// config entry, hidden ones included; any failure refuses generically.
func (c *client) checkEntrySiblings(ctx context.Context, method string, data *snapData, entryID string, needDelete bool) error {
	ok, err := c.allConfigurable(ctx, data.entitiesOfConfigEntry(entryID), needDelete)
	if err != nil {
		return c.undecidable(method, err)
	}
	if !ok {
		return c.refuse(method)
	}
	return nil
}
