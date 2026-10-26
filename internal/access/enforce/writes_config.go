package enforce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Automation, script and scene configuration writes. The REST writes are
// upserts: Create* with an existing id overwrites it, Update* with an unknown
// id creates it. So every write first resolves whether the config id exists
// (resolve.go, unfiltered registry):
//
//   - It exists and its entity is visible: the write needs configure on that
//     entity, whatever the method (a create is an update then).
//   - It exists but its entity is hidden: an update or delete answers as for
//     a missing config (NewConfigNotFoundError, never a denial); a create is
//     first decided like a create of a free id (checkCreateGrant) and only a
//     caller with those rights gets errUnavailable (create.go), the answer for
//     any taken id, so it can never overwrite a config the caller cannot see.
//   - It does not exist: an update or delete answers as for a missing config;
//     a create needs configure and create on the entity id HA will derive
//     (create.go).
//   - A delete needs configure and delete.
//
// After the permission check the definition's embedded service calls are
// checked (checkEmbedded) unless the policy is unrestricted. Only then is the
// configured domain marked on the tracker, so the handler's follow-up
// <domain>.reload passes, and inner is called exactly once with the original
// config. Every write that reached inner drops the registry snapshot, failed
// or not.

// configWrite is one create or update of a stored config.
type configWrite struct {
	method string // Client method name, for refusals
	domain string // automation, script or scene
	id     string // config id (script: key)
	name   string // what the new entity id derives from: alias, key or name
	def    any    // the new definition, as passed to inner
	create bool   // Create* rather than Update*
}

func (c *client) CreateAutomation(ctx context.Context, cfg homeassistant.AutomationConfig) error {
	if cfg.ID == "" {
		return c.refuse("CreateAutomation")
	}
	w := configWrite{method: "CreateAutomation", domain: domainAutomation, id: cfg.ID, name: cfg.Alias, def: cfg, create: true}
	return c.runConfigWrite(ctx, w, func() error { return c.inner.CreateAutomation(ctx, cfg) })
}

// UpdateAutomation refuses a body id that differs from the path id: HA would
// store the entry under the body id, a config the decision was not about.
func (c *client) UpdateAutomation(ctx context.Context, automationID string, cfg homeassistant.AutomationConfig) error {
	if cfg.ID != "" && cfg.ID != automationID {
		return c.refuse("UpdateAutomation")
	}
	w := configWrite{method: "UpdateAutomation", domain: domainAutomation, id: automationID, name: cfg.Alias, def: cfg}
	return c.runConfigWrite(ctx, w, func() error { return c.inner.UpdateAutomation(ctx, automationID, cfg) })
}

func (c *client) DeleteAutomation(ctx context.Context, automationID string) error {
	return c.runConfigDelete(ctx, "DeleteAutomation", domainAutomation, automationID, func() error {
		return c.inner.DeleteAutomation(ctx, automationID)
	})
}

func (c *client) CreateScript(ctx context.Context, scriptID string, cfg homeassistant.ScriptConfig) error {
	w := configWrite{method: "CreateScript", domain: domainScript, id: scriptID, name: scriptID, def: cfg, create: true}
	return c.runConfigWrite(ctx, w, func() error { return c.inner.CreateScript(ctx, scriptID, cfg) })
}

func (c *client) UpdateScript(ctx context.Context, scriptID string, cfg homeassistant.ScriptConfig) error {
	w := configWrite{method: "UpdateScript", domain: domainScript, id: scriptID, name: scriptID, def: cfg}
	return c.runConfigWrite(ctx, w, func() error { return c.inner.UpdateScript(ctx, scriptID, cfg) })
}

// DeleteScript answers a hidden or unknown script with the missing-config
// error. The script handler then falls back to removing the registry entry,
// which goes through RemoveEntityRegistryEntry of this wrapper and its own
// decision; a hidden script is not even listed by GetEntityRegistry.
func (c *client) DeleteScript(ctx context.Context, scriptID string) error {
	return c.runConfigDelete(ctx, "DeleteScript", domainScript, scriptID, func() error {
		return c.inner.DeleteScript(ctx, scriptID)
	})
}

func (c *client) CreateScene(ctx context.Context, sceneID string, cfg homeassistant.SceneConfig) error {
	w := configWrite{method: "CreateScene", domain: domainScene, id: sceneID, name: cfg.Name, def: cfg, create: true}
	return c.runConfigWrite(ctx, w, func() error { return c.inner.CreateScene(ctx, sceneID, cfg) })
}

func (c *client) UpdateScene(ctx context.Context, sceneID string, cfg homeassistant.SceneConfig) error {
	w := configWrite{method: "UpdateScene", domain: domainScene, id: sceneID, name: cfg.Name, def: cfg}
	return c.runConfigWrite(ctx, w, func() error { return c.inner.UpdateScene(ctx, sceneID, cfg) })
}

func (c *client) DeleteScene(ctx context.Context, sceneID string) error {
	return c.runConfigDelete(ctx, "DeleteScene", domainScene, sceneID, func() error {
		return c.inner.DeleteScene(ctx, sceneID)
	})
}

// runConfigWrite decides w, then calls inner once and drops the snapshot.
func (c *client) runConfigWrite(ctx context.Context, w configWrite, call func() error) error {
	if err := c.writeConfig(ctx, w); err != nil {
		return err
	}
	return c.callAndInvalidate(call)
}

func (c *client) runConfigDelete(ctx context.Context, method, domain, id string, call func() error) error {
	if err := c.deleteConfig(ctx, method, domain, id); err != nil {
		return err
	}
	return c.callAndInvalidate(call)
}

// callAndInvalidate calls inner and drops the snapshot whatever the outcome: a
// failed write (a timeout, say) may still have been applied.
func (c *client) callAndInvalidate(call func() error) error {
	err := call()
	c.snap.invalidate()
	return err
}

// writeConfig decides a create or update and, when it is allowed, marks the
// domain as configured. It returns an error only to refuse.
func (c *client) writeConfig(ctx context.Context, w configWrite) error {
	if !validConfigID(w.domain, w.id) {
		if w.create {
			return c.refuse(w.method)
		}
		return homeassistant.NewConfigNotFoundError(w.domain, w.id)
	}
	entityID, exists, err := c.entityForConfigID(ctx, w.domain, w.id)
	if err != nil {
		return c.undecidable(w.method, err)
	}
	switch {
	case exists:
		err = c.checkExisting(ctx, w, entityID)
	case w.create:
		entityID, err = c.checkNew(ctx, w)
	default:
		err = homeassistant.NewConfigNotFoundError(w.domain, w.id)
	}
	if err != nil {
		return err
	}
	if err := c.checkEmbedded(ctx, w, entityID, exists); err != nil {
		return err
	}
	c.tr.MarkConfigured(w.domain)
	return nil
}

// checkExisting decides a write to a config whose entity is entityID.
func (c *client) checkExisting(ctx context.Context, w configWrite, entityID string) error {
	grant, err := c.grantFunc(ctx)
	if err != nil {
		return c.undecidable(w.method, err)
	}
	g := grant(entityID)
	switch {
	case g.Hidden() && w.create:
		// Answer like a create of a free id until the caller is proven to
		// have the rights such a create needs; only then is the id reported
		// as taken.
		if _, err := c.checkCreateGrant(w); err != nil {
			return err
		}
		return errUnavailable(w.id)
	case g.Hidden():
		return homeassistant.NewConfigNotFoundError(w.domain, w.id)
	case !g.Configure:
		return c.denyObject(entityID, access.CapConfigure)
	}
	return nil
}

// deleteConfig decides the delete of a stored config: a hidden, unknown or
// unregistered id answers as a missing config; a visible one needs configure
// and delete.
func (c *client) deleteConfig(ctx context.Context, method, domain, id string) error {
	missing := homeassistant.NewConfigNotFoundError(domain, id)
	if !validConfigID(domain, id) {
		return missing
	}
	entityID, exists, err := c.entityForConfigID(ctx, domain, id)
	if err != nil {
		return c.undecidable(method, err)
	}
	if !exists {
		return missing
	}
	grant, err := c.grantFunc(ctx)
	if err != nil {
		return c.undecidable(method, err)
	}
	g := grant(entityID)
	switch {
	case g.Hidden():
		return missing
	case !g.Configure:
		return c.denyObject(entityID, access.CapConfigure)
	case !g.Delete:
		return c.denyObject(entityID, capDelete)
	}
	c.tr.MarkConfigured(domain)
	return nil
}

// checkEmbedded checks the service calls the new definition would make. For a
// write to an existing config only the calls the stored definition does not
// already make are checked; when the stored definition cannot be read or
// analyzed, every call is checked. Any call that cannot be decided refuses the
// write. Only an unrestricted policy skips the check.
func (c *client) checkEmbedded(ctx context.Context, w configWrite, entityID string, exists bool) error {
	if c.p.Unrestricted() {
		return nil
	}
	calls, err := callsOf(w.domain, w.def)
	if err != nil {
		return c.embeddedRefusal(entityID, err.Error())
	}
	if exists {
		calls = access.NewCalls(c.storedCalls(ctx, w.domain, w.id, entityID), calls)
	}
	for _, call := range calls {
		if err := c.checkEmbeddedCall(ctx, entityID, call); err != nil {
			return err
		}
	}
	return nil
}

// embeddedRefusal records a generic denial whose log object names the
// configured entity (visible to the caller) and the reason.
func (c *client) embeddedRefusal(entityID, reason string) error {
	return c.refuse(entityID + ": embedded action: " + reason)
}

// storedCalls returns the calls of the stored definition, read unfiltered from
// inner, or nil when it cannot be read or analyzed (fail closed: every new
// call is then checked).
func (c *client) storedCalls(ctx context.Context, domain, id, entityID string) []access.Call {
	def, err := c.storedDefinition(ctx, domain, id, entityID)
	if err != nil {
		return nil
	}
	calls, err := callsOf(domain, def)
	if err != nil {
		return nil
	}
	return calls
}

var errNoStoredConfig = errors.New("stored definition has no configuration")

// storedDefinition reads the current definition of an existing config.
func (c *client) storedDefinition(ctx context.Context, domain, id, entityID string) (any, error) {
	switch domain {
	case domainAutomation:
		a, err := c.inner.GetAutomation(ctx, entityID)
		if err != nil || a == nil || a.Config == nil {
			return nil, errors.Join(err, errNoStoredConfig)
		}
		return *a.Config, nil
	case domainScript:
		s, err := c.inner.GetScript(ctx, id)
		if err != nil || s == nil || s.Config == nil {
			return nil, errors.Join(err, errNoStoredConfig)
		}
		return *s.Config, nil
	default:
		s, err := c.inner.GetScene(ctx, id)
		if err != nil || s == nil || s.Config == nil {
			return nil, errors.Join(err, errNoStoredConfig)
		}
		return *s.Config, nil
	}
}

// callsOf returns the service calls a definition makes. Automations and
// scripts are analyzed by access.ExtractCalls on their JSON form (so keys the
// typed config does not model, such as use_blueprint, are seen); a scene is a
// scene.apply of each of its entities.
func callsOf(domain string, def any) ([]access.Call, error) {
	if sc, ok := def.(homeassistant.SceneConfig); ok && domain == domainScene {
		return sceneCalls(sc)
	}
	raw, err := json.Marshal(def)
	if err != nil {
		return nil, fmt.Errorf("encoding definition: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decoding definition: %w", err)
	}
	return access.ExtractCalls(m)
}

// sceneCalls models a scene as one scene.apply per entity: activating the
// scene sets every entity, so each needs control. Raw is the entity with its
// state, so an update rechecks only added or changed entities.
func sceneCalls(sc homeassistant.SceneConfig) ([]access.Call, error) {
	ids := make([]string, 0, len(sc.Entities))
	for id := range sc.Entities {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	calls := make([]access.Call, 0, len(ids))
	for _, id := range ids {
		if !isPlainEntityID(id) {
			return nil, fmt.Errorf("scene entity %q has an unsupported form", id)
		}
		state := sc.Entities[id]
		raw, err := json.Marshal(map[string]any{"scene_entity": id, "state": state})
		if err != nil {
			return nil, fmt.Errorf("encoding scene entity: %w", err)
		}
		calls = append(calls, access.Call{
			Domain: domainScene, Service: "apply", EntityData: []string{id},
			Data: map[string]any{"entities": map[string]any{id: state}}, Raw: string(raw),
		})
	}
	return calls, nil
}

// checkEmbeddedCall applies the service-call rules (checkCall) to one embedded
// call. An explicit entity that is hidden, or absent under a hiding policy,
// refuses the write generically: the missing-entity error a direct call gets
// would tell a hidden id from an absent one only by comparison, but here the
// write must fail either way and the generic denial names neither.
func (c *client) checkEmbeddedCall(ctx context.Context, owner string, call access.Call) error {
	req, err := c.embeddedRequest(ctx, call)
	if err != nil {
		return c.embeddedRefusal(owner, err.Error())
	}
	if err := c.checkCall(ctx, req); err != nil {
		if isEntityMissing(err) {
			return c.embeddedRefusal(owner, "unavailable entity")
		}
		return err
	}
	return nil
}

// embeddedRequest builds the callRequest for an embedded call. The
// ObjectFromData object (which ExtractCalls also lists as a target) is decided
// by the object rule instead. A device action is decided without the domain
// filter, so every entity of the device needs control, and the registry ids it
// names are resolved to entity ids through the unfiltered registry.
func (c *client) embeddedRequest(ctx context.Context, call access.Call) (callRequest, error) {
	req := callRequest{
		domain: call.Domain, service: call.Service, targets: call.Targets,
		entityData: call.EntityData, mustExist: c.p.HidesAnything(), data: call.Data,
	}
	req.targets.EntityIDs = slices.Clone(call.Targets.EntityIDs)
	class := access.ClassifyService(call.Domain, call.Service)
	if class.Kind != access.ServiceSystem && class.ObjectFromData != "" {
		r, ok := requestFromData(call.Domain, call.Service, class, call.Data)
		if !ok {
			return req, errors.New("object of the call cannot be decided")
		}
		req.object = r.object
		req.targets.EntityIDs = slices.DeleteFunc(req.targets.EntityIDs, func(id string) bool { return id == r.object })
	}
	if call.Service == access.DeviceActionService {
		req.domain = domainHomeAssistant
	}
	if len(call.EntityRegistryIDs) == 0 {
		return req, nil
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return req, errors.New("registry unavailable")
	}
	for _, rid := range call.EntityRegistryIDs {
		id, ok := data.entityForRegistryID(rid)
		if !ok {
			return req, errors.New("unknown entity registry id")
		}
		req.targets.EntityIDs = append(req.targets.EntityIDs, id)
	}
	return req, nil
}

// isEntityMissing reports whether err is the missing-entity error checkCall
// returns for a hidden or absent explicit id (denials never contain the
// phrase, see access.Denial.Message).
func isEntityMissing(err error) bool {
	return strings.HasPrefix(err.Error(), homeassistant.NewEntityNotFoundError("").Error())
}
