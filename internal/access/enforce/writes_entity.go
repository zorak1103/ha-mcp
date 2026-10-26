package enforce

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Service calls and entity writes. A call is decided in full before the inner
// client sees it: classify it, collect every object it reaches, check all of
// them, and only then call inner exactly once. Any blocked or undecidable
// object refuses the whole call; nothing is done partially.
//
//   - Explicit entity ids (target, the data keys of ServiceClass.EntityDataKeys,
//     script.<service> for a script called as a service) must be plain valid
//     ids, visible, and present in the state machine. Otherwise the answer is
//     the missing-entity error of GetState, for hidden and nonexistent ids
//     alike, and no denial is recorded. All ids are judged in one pass in
//     request order, so the first failing id is the same as in a home without
//     the hidden entity.
//   - Entities reached by expansion (targets.go, filtered to the service's
//     domain or platform) need the capability too. A hidden one refuses the
//     call with the generic denial naming only the service; a visible blocked
//     entity is named in the denial. An expansion that reaches nothing of the
//     service's own (All reaching nothing, or only other domains) refuses the
//     call generically; it is never left to the default.
//   - The object a configure service names in its data (scene.create scene_id,
//     group.set/remove object_id) needs configure, create when it does not
//     exist yet, delete for group.remove. It is refused with the generic
//     denial so a hidden object reads like a visible read-only one.
//   - A call that reaches nothing is object-less and decided by the default.
//     Under a hiding policy a target that names device, area, floor or label
//     ids yet reaches nothing is refused generically, like one that reaches a
//     hidden entity, so an unknown id cannot be told from a hidden one.
//   - Read services need read on every object; control services control;
//     configure services configure; entity data always needs control.
//
// Residual: an entity that is itself a group (group.x, a light group) is
// expanded by HA to its members, which are not checked here.

// slugRE is the shape of a bare object id named in call data.
var slugRE = regexp.MustCompile(`^[a-z0-9_]+$`)

// deletingServices remove the object named in their data.
var deletingServices = map[string]bool{"group.remove": true}

const (
	domainHomeAssistant = "homeassistant"
	reloadIDKey         = "id"
)

// callRequest is one service call reduced to what the policy decides on.
// object is "<domain>.<id>" for a ServiceClass.ObjectFromData object and need
// not exist; mustExist requires every explicit entity id to be in the state
// machine.
type callRequest struct {
	domain, service string
	targets         access.TargetSpec
	entityData      []string
	object          string
	mustExist       bool
}

// name is the normalised service name ("domain.service"), or the method name
// for an entity write without a service.
func (r callRequest) name() string {
	if r.domain == "" {
		return r.service
	}
	return normName(r.domain) + "." + normName(r.service)
}

func normName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (c *client) CallService(ctx context.Context, domain, service string, data map[string]any) ([]homeassistant.Entity, error) {
	if err := c.checkServiceCall(ctx, domain, service, data); err != nil {
		return nil, err
	}
	return c.inner.CallService(ctx, domain, service, data)
}

func (c *client) CallServiceWithResponse(ctx context.Context, domain, service string, data map[string]any) (map[string]any, error) {
	if err := c.checkServiceCall(ctx, domain, service, data); err != nil {
		return nil, err
	}
	return c.inner.CallServiceWithResponse(ctx, domain, service, data)
}

// SetState writes a raw state. It needs control on an existing entity: under
// this policy layer it never creates a new entity.
func (c *client) SetState(ctx context.Context, entityID string, state homeassistant.StateUpdate) (*homeassistant.Entity, error) {
	if err := c.checkEntityControl(ctx, "SetState", entityID); err != nil {
		return nil, err
	}
	return c.inner.SetState(ctx, entityID, state)
}

func (c *client) SetHelperValue(ctx context.Context, entityID string, value any) error {
	if err := c.checkEntityControl(ctx, "SetHelperValue", entityID); err != nil {
		return err
	}
	// The real client merges a map value into the service data
	// (input_datetime), so a key such as entity_id would retarget the call.
	// Only the datetime fields are accepted. Checked after the entity, so a
	// hidden helper still answers as missing.
	if m, ok := value.(map[string]any); ok {
		for k := range m {
			if !helperValueKeys[k] {
				return c.denyObject(normName(entityID), access.CapControl)
			}
		}
	}
	return c.inner.SetHelperValue(ctx, entityID, value)
}

// helperValueKeys are the keys a map helper value may carry
// (input_datetime.set_datetime fields).
var helperValueKeys = map[string]bool{"date": true, "time": true, "datetime": true, "timestamp": true}

func (c *client) ToggleAutomation(ctx context.Context, entityID string, enabled bool) error {
	if err := c.checkEntityControl(ctx, "ToggleAutomation", entityID); err != nil {
		return err
	}
	return c.inner.ToggleAutomation(ctx, entityID, enabled)
}

// checkEntityControl decides a write on one entity: control, existing,
// visible.
func (c *client) checkEntityControl(ctx context.Context, method, entityID string) error {
	req := callRequest{
		service:   method,
		targets:   access.TargetSpec{EntityIDs: []string{normName(entityID)}},
		mustExist: true,
	}
	return c.checkObjects(ctx, req, access.CapControl)
}

// checkServiceCall decides a CallService request. It returns an error only to
// refuse; a recorded denial (if any) is already on the tracker.
func (c *client) checkServiceCall(ctx context.Context, domain, service string, data map[string]any) error {
	if c.internalReload(domain, service, data) {
		return nil
	}
	class := access.ClassifyService(domain, service)
	if class.Kind == access.ServiceSystem {
		return c.checkSystem(class.System)
	}
	req, ok := requestFromData(domain, service, class, data)
	if !ok {
		return c.refuse(req.name())
	}
	return c.checkCall(ctx, req)
}

// internalReload reports whether the call is the reload a handler sends after
// a configure operation of the same domain succeeded in this request: service
// reload with no data or only a string "id".
func (c *client) internalReload(domain, service string, data map[string]any) bool {
	if service != "reload" {
		return false
	}
	switch len(data) {
	case 0:
	case 1:
		if _, ok := data[reloadIDKey].(string); !ok {
			return false
		}
	default:
		return false
	}
	return c.tr.ReloadAllowed(domain)
}

// requestFromData builds the request for a non-system call from its data. ok
// is false when the objects cannot be decided: a malformed target or entity
// data value, or a missing or non-slug ObjectFromData value.
func requestFromData(domain, service string, class access.ServiceClass, data map[string]any) (callRequest, bool) {
	req := callRequest{domain: domain, service: service, targets: access.ParseTarget(data), mustExist: true}
	if req.targets.Malformed {
		return req, false
	}
	var malformed bool
	if req.entityData, malformed = access.EntityIDsFromData(data, class.EntityDataKeys); malformed {
		return req, false
	}
	if key := class.ObjectFromData; key != "" {
		v, ok := data[key].(string)
		if !ok || !slugRE.MatchString(v) {
			return req, false
		}
		req.object = normName(domain) + "." + v
	}
	return req, true
}

// checkCall performs the classify-and-check steps for one service call
// without calling inner. It returns an error only to refuse (any denial is
// already recorded on the tracker). Tasks that check the calls embedded in a
// definition reuse it with mustExist chosen by the caller.
func (c *client) checkCall(ctx context.Context, req callRequest) error {
	class := access.ClassifyService(req.domain, req.service)
	switch class.Kind {
	case access.ServiceSystem:
		return c.checkSystem(class.System)
	case access.ServiceScriptCall:
		script := domainScript + "." + normName(req.service)
		req.targets.EntityIDs = append([]string{script}, req.targets.EntityIDs...)
	}
	if req.targets.Malformed {
		return c.refuse(req.name())
	}
	return c.checkObjects(ctx, req, primaryCapability(class.Kind))
}

// primaryCapability is what a service kind needs on its target entities.
func primaryCapability(k access.ServiceKind) access.Capability {
	switch k {
	case access.ServiceRead:
		return access.CapRead
	case access.ServiceConfigure:
		return access.CapConfigure
	default:
		return access.CapControl
	}
}

// checkObjects checks every object req reaches against primary (entity data:
// control).
func (c *client) checkObjects(ctx context.Context, req callRequest, primary access.Capability) error {
	grant, err := c.grantFunc(ctx)
	if err != nil {
		return c.undecidable(req.name(), err)
	}
	states := &stateSet{}
	if refusal := c.checkExplicit(ctx, req, grant, states); refusal != nil {
		return refusal
	}
	x, err := c.expandTargets(ctx, req.domain, req.targets, states)
	if err != nil {
		return c.undecidable(req.name(), err)
	}
	if x.unresolved(req.targets) {
		return c.refuse(req.name())
	}
	if len(req.targets.EntityIDs)+len(req.entityData)+len(x.ids) == 0 && req.object == "" {
		return c.checkObjectLess(req, primary)
	}
	return c.checkGrants(ctx, req, primary, grant, x.ids, states)
}

// checkExplicit answers the first explicit id that is not plain, hidden or
// (with mustExist) absent from the state machine with the missing-entity
// error, judging all ids in one pass in request order.
func (c *client) checkExplicit(ctx context.Context, req callRequest, grant func(string) access.Grant, states *stateSet) error {
	ids := append(append([]string(nil), req.targets.EntityIDs...), req.entityData...)
	if len(ids) == 0 {
		return nil
	}
	var exists map[string]bool
	if req.mustExist {
		var err error
		if exists, err = states.load(ctx, c.inner); err != nil {
			return c.undecidable(req.name(), err)
		}
	}
	for _, id := range ids {
		if !isPlainEntityID(id) || grant(id).Hidden() || (req.mustExist && !exists[id]) {
			return errEntityMissing(id)
		}
	}
	return nil
}

// checkObjectLess decides a call that reaches no object: by the default.
func (c *client) checkObjectLess(req callRequest, primary access.Capability) error {
	if expandable(req.targets) && c.p.HidesAnything() {
		return c.refuse(req.name())
	}
	if c.p.Decide(access.Object{Kind: access.KindGlobal}).Has(primary) {
		return nil
	}
	return c.denyObject(req.name(), primary)
}

// checkGrants requires primary on every target entity, explicit or expanded,
// control on every entity data id, and the object rule on req.object. The
// first visible blocked entity is named in the denial; a hidden entity or a
// blocked data object refuses with the generic denial.
func (c *client) checkGrants(
	ctx context.Context, req callRequest, primary access.Capability,
	grant func(string) access.Grant, expanded []string, states *stateSet,
) error {
	var blockedID string
	var blockedCap access.Capability
	hiddenHit := false
	consider := func(ids []string, need access.Capability) {
		for _, id := range ids {
			g := grant(id)
			switch {
			case g.Hidden():
				hiddenHit = true
			case !g.Has(need) && blockedID == "":
				blockedID, blockedCap = id, need
			}
		}
	}
	consider(req.targets.EntityIDs, primary)
	consider(req.entityData, access.CapControl)
	consider(expanded, primary)
	if blockedID != "" {
		return c.denyObject(blockedID, blockedCap)
	}
	if hiddenHit {
		return c.refuse(req.name())
	}
	if req.object == "" {
		return nil
	}
	ok, err := c.objectAllowed(ctx, req, grant, states)
	if err != nil {
		return c.undecidable(req.name(), err)
	}
	if !ok {
		return c.refuse(req.name())
	}
	return nil
}

// objectAllowed applies the configure rule to the object named in the data.
func (c *client) objectAllowed(ctx context.Context, req callRequest, grant func(string) access.Grant, states *stateSet) (bool, error) {
	if !isPlainEntityID(req.object) {
		return false, nil
	}
	g := grant(req.object)
	if !g.Configure {
		return false, nil
	}
	if deletingServices[req.name()] {
		return g.Delete, nil
	}
	ids, err := states.load(ctx, c.inner)
	if err != nil {
		return false, err
	}
	return ids[req.object] || g.Create, nil
}

// checkSystem allows a system operation of class v or records its denial.
func (c *client) checkSystem(v access.SystemValue) error {
	if c.p.SystemAllowed(v) {
		return nil
	}
	d := access.Denial{Tool: c.tr.Tool(), System: v}
	c.tr.Deny(d)
	return errors.New(d.Message())
}

// denyObject records that object lacks capability and returns the caller's
// error. object must be visible to the caller (never a hidden id).
func (c *client) denyObject(object string, capability access.Capability) error {
	d := access.Denial{Tool: c.tr.Tool(), Object: object, Capability: string(capability)}
	c.tr.Deny(d)
	return errors.New(d.Message())
}
