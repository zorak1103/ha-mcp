package enforce

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// Target lookups (applicable triggers, conditions, services, referenced
// objects). Home Assistant expands areas, devices and labels itself, so the
// hidden entities behind them would shape the answer, and an id the caller
// names explicitly must not look different from a missing one.
//
// Explicit hidden ids are never sent to HA. Each one is replaced by an absent
// twin, an id of the same kind that does not exist (see twins), so HA answers
// exactly as it does for a missing id without the wrapper having to know how;
// for ExtractFromTarget the twin is swapped back to the caller's own id in the
// result (and in an error text), so the hidden id is reported the way a missing
// one is.
//
// The three name lists (triggers, conditions, services) return names such as
// "lock.unlock" that cannot be filtered per entity. With a hiding policy their
// target is rewritten to entity ids only: areas, devices and labels are
// expanded from the registry snapshot, hidden entities are removed, hidden
// explicit ids become twins. A target left without any entity (an area with no
// visible entity, say) is answered [] without a call, because an empty target
// would mean everything.
//
// ExtractFromTarget reports the expansion itself, so its target keeps areas,
// devices and labels (hidden explicit ones become twins) and the result is
// filtered: entities and devices the caller did not name and that are hidden
// are dropped, without echo. HA builds the result lists from sets, so their
// order is hash-determined and a twin would sit where its own hash puts it, not
// where the hidden id would; every list is therefore sorted and de-duplicated
// after the swap-back, making the answer a function of its content alone.
//
// LIVE-CHECK: the name lists rely on HA answering a twin entity id of a domain
// like any other missing id of that domain.
//
// Residual: a visible entity group can contain hidden members and HA expands
// the group; for the name lists that can only add a domain's names.

const (
	twinMarker    = "ha_mcp_"
	maxNonceTries = 5
	kindEntity    = "entity"
	kindDevice    = "device"
	kindArea      = "area"
)

// newNonce returns a fresh random nonce for the twin ids of one request. It is
// a variable so tests can inject values; an empty result counts as a failure.
var newNonce = func() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// errNoNonce is returned when no nonce without a clash could be found; callers
// fail closed with the generic refusal.
var errNoNonce = errors.New("no collision-free twin nonce")

// reservedID reports whether a caller-supplied id contains the twin marker
// literally. Such an id is treated as hidden (and twinned itself) so a caller
// can never craft an id that could be mistaken for a twin.
func reservedID(id string) bool {
	return strings.Contains(strings.ToLower(id), twinMarker)
}

// chooseNonce draws a nonce whose twin pattern "ha_mcp_<nonce>_" occurs in none
// of the known strings (the ids of the request and every id and name of the
// registry snapshot), retrying a bounded number of times.
func chooseNonce(known []string) (string, error) {
	lower := make([]string, len(known))
	for i, k := range known {
		lower[i] = strings.ToLower(k)
	}
	for range maxNonceTries {
		n := newNonce()
		if n == "" {
			continue
		}
		marker := twinMarker + strings.ToLower(n) + "_"
		if !slices.ContainsFunc(lower, func(k string) bool { return strings.Contains(k, marker) }) {
			return n, nil
		}
	}
	return "", errNoNonce
}

// twins maps the hidden ids of one request to absent twins and back.
type twins struct {
	nonce string
	fwd   map[string]string // "<kind>/<raw id>" -> twin
	back  map[string]string // twin -> raw id
}

func newTwins(nonce string) *twins {
	return &twins{nonce: nonce, fwd: map[string]string{}, back: map[string]string{}}
}

// twin returns the twin of raw for the kind (entity, device, area). An entity
// twin keeps the domain of the id (sensor when it has none) so domain-specific
// answers stay the same as for a missing id of that domain. The nonce makes
// twins unguessable; chooseNonce made sure none exists already.
func (t *twins) twin(kind, raw string) string {
	key := kind + "/" + raw
	if tw, ok := t.fwd[key]; ok {
		return tw
	}
	tw := fmt.Sprintf("%s%s_%s_%d", twinMarker, t.nonce, kind, len(t.back))
	if kind == kindEntity {
		tw = fmt.Sprintf("%s.%s%s_%d", entityDomainOf(raw), twinMarker, t.nonce, len(t.back))
	}
	t.fwd[key] = tw
	t.back[tw] = raw
	return tw
}

// entityDomainOf returns the domain of an entity id, lower-cased and trimmed;
// "sensor" when the id has no usable one.
func entityDomainOf(id string) string {
	d, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(id)), ".")
	if d == "" || strings.ContainsFunc(d, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_'
	}) {
		return "sensor"
	}
	return d
}

func (t *twins) isTwin(id string) bool {
	_, ok := t.back[id]
	return ok
}

// restore swaps every twin in list back to the id it stands for.
func (t *twins) restore(list []string) []string {
	if list == nil {
		return nil
	}
	out := slices.Clone(list)
	for i, id := range out {
		if raw, ok := t.back[id]; ok {
			out[i] = raw
		}
	}
	return out
}

// restoreText swaps twins in free text back to the caller's ids, longest twin
// first so "..._1" cannot corrupt "..._10".
func (t *twins) restoreText(s string) string {
	ordered := make([]string, 0, len(t.back))
	for tw := range t.back {
		ordered = append(ordered, tw)
	}
	slices.SortFunc(ordered, func(a, b string) int { return len(b) - len(a) })
	for _, tw := range ordered {
		s = strings.ReplaceAll(s, tw, t.back[tw])
	}
	return s
}

// swappedError is an inner error whose text had twins swapped back. It keeps
// an APIError (copied, with the swapped message) reachable through errors.As.
type swappedError struct {
	msg string
	api *homeassistant.APIError
}

func (e *swappedError) Error() string { return e.msg }

func (e *swappedError) Unwrap() error {
	if e.api == nil {
		return nil
	}
	return e.api
}

// swapError returns err with twins replaced by the caller's ids; an error that
// names no twin is returned as it is.
func (t *twins) swapError(err error) error {
	if err == nil || len(t.back) == 0 {
		return err
	}
	msg := t.restoreText(err.Error())
	if msg == err.Error() {
		return err
	}
	out := &swappedError{msg: msg}
	var api *homeassistant.APIError
	if errors.As(err, &api) {
		cp := *api
		cp.Message = t.restoreText(api.Message)
		out.api = &cp
	}
	return out
}

// keepOrTwin filters list: twins always stay (they stand for ids the caller
// named), other ids stay when keep says so.
func (t *twins) keepOrTwin(list []string, keep func(string) bool) []string {
	return filterCopy(list, func(id string) bool { return t.isTwin(id) || keep(id) })
}

// canonical returns list sorted ascending and de-duplicated, never nil.
func canonical(list []string) []string {
	out := slices.Clone(list)
	if out == nil {
		return []string{}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// mapIDs returns a copy of in with f applied; nil stays nil.
func mapIDs(in []string, f func(string) string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, id := range in {
		out[i] = f(id)
	}
	return out
}

// hideContext holds the decisions one target request needs.
type hideContext struct {
	hidden      func(entityID string) bool
	hiddenDevs  map[string]bool
	areasHidden bool
	tw          *twins
}

// newHideContext decides for one request: hidden entities, devices and the
// global (area) decision, plus a twin generator whose nonce clashes with
// nothing in the request or the registry snapshot.
func (c *client) newHideContext(ctx context.Context, t homeassistant.Target) (*hideContext, error) {
	hidden, err := c.hiddenFunc(ctx)
	if err != nil {
		return nil, err
	}
	devs, err := c.hiddenDevices(ctx)
	if err != nil {
		return nil, err
	}
	data, err := c.snap.get(ctx)
	if err != nil {
		return nil, err
	}
	nonce, err := chooseNonce(slices.Concat(t.EntityID, t.DeviceID, t.AreaID, t.LabelID, data.knownStrings()))
	if err != nil {
		return nil, err
	}
	return &hideContext{hidden: hidden, hiddenDevs: devs, areasHidden: c.globalHidden(), tw: newTwins(nonce)}, nil
}

func (h *hideContext) entityHidden(id string) bool { return reservedID(id) || h.hidden(id) }
func (h *hideContext) deviceHidden(id string) bool { return reservedID(id) || h.hiddenDevs[id] }
func (h *hideContext) areaHidden(id string) bool   { return reservedID(id) || h.areasHidden }

// twinned returns id, or its twin when the caller named it and it is hidden.
func (h *hideContext) twinned(kind string, hidden func(string) bool) func(string) string {
	return func(id string) string {
		if hidden(id) {
			return h.tw.twin(kind, id)
		}
		return id
	}
}

// masked returns t with every hidden explicit id replaced by its twin.
func (h *hideContext) masked(t homeassistant.Target) homeassistant.Target {
	return homeassistant.Target{
		EntityID: mapIDs(t.EntityID, h.twinned(kindEntity, h.entityHidden)),
		DeviceID: mapIDs(t.DeviceID, h.twinned(kindDevice, h.deviceHidden)),
		AreaID:   mapIDs(t.AreaID, h.twinned(kindArea, h.areaHidden)),
		LabelID:  slices.Clone(t.LabelID),
	}
}

// expandedTarget rewrites target for the name-list methods. skip is true when
// nothing is left to ask about. The twins are returned so an error can be
// swapped back.
func (c *client) expandedTarget(ctx context.Context, t homeassistant.Target) (target homeassistant.Target, tw *twins, skip bool, err error) {
	if !c.p.HidesAnything() || isEmptyTarget(t) {
		return t, nil, false, nil
	}
	h, err := c.newHideContext(ctx, t)
	if err != nil {
		return t, nil, false, err
	}
	entities := mapIDs(t.EntityID, h.twinned(kindEntity, h.entityHidden))
	// A device or area id carrying the twin marker is a twinned (missing) id:
	// it expands to nothing.
	rest := homeassistant.Target{
		DeviceID: filterCopy(t.DeviceID, func(id string) bool { return !reservedID(id) }),
		AreaID:   filterCopy(t.AreaID, func(id string) bool { return !reservedID(id) }),
		LabelID:  t.LabelID,
	}
	if len(rest.DeviceID)+len(rest.AreaID)+len(rest.LabelID) > 0 {
		data, err := c.snap.get(ctx)
		if err != nil {
			return t, nil, false, err
		}
		for _, id := range expandTarget(data, rest, h.areasHidden) {
			if !h.hidden(id) {
				entities = append(entities, id)
			}
		}
		slices.Sort(entities)
		entities = slices.Compact(entities)
	}
	if len(entities) == 0 {
		return homeassistant.Target{}, h.tw, true, nil
	}
	return homeassistant.Target{EntityID: entities}, h.tw, false, nil
}

func isEmptyTarget(t homeassistant.Target) bool {
	return len(t.EntityID)+len(t.DeviceID)+len(t.AreaID)+len(t.LabelID) == 0
}

// expandTarget lists the entities behind the target's areas, devices and
// labels. A label also reaches the entities of every area that carries it.
// Hidden areas (default none) expand to nothing, like a missing area.
func expandTarget(d *snapData, t homeassistant.Target, areasHidden bool) []string {
	var out []string
	if !areasHidden {
		for _, id := range t.AreaID {
			out = append(out, d.entitiesOfArea(id)...)
		}
	}
	for _, id := range t.DeviceID {
		out = append(out, d.entitiesOfDevice(id)...)
	}
	for _, label := range t.LabelID {
		out = append(out, d.entitiesOfLabel(label)...)
		if areasHidden {
			continue
		}
		for _, a := range d.areas {
			if slices.Contains(a.Labels, label) {
				out = append(out, d.entitiesOfArea(a.AreaID)...)
			}
		}
	}
	return out
}

// namesForTarget runs one of the name-list methods against the rewritten
// target.
func (c *client) namesForTarget(
	ctx context.Context, method string, t homeassistant.Target,
	call func(homeassistant.Target) ([]string, error),
) ([]string, error) {
	target, tw, skip, err := c.expandedTarget(ctx, t)
	if err != nil {
		return nil, c.undecidable(method, err)
	}
	if skip {
		return []string{}, nil
	}
	names, err := call(target)
	if tw != nil {
		err = tw.swapError(err)
	}
	return names, err
}

func (c *client) GetTriggersForTarget(ctx context.Context, t homeassistant.Target, expandGroup *bool) ([]string, error) {
	return c.namesForTarget(ctx, "GetTriggersForTarget", t, func(tt homeassistant.Target) ([]string, error) {
		return c.inner.GetTriggersForTarget(ctx, tt, expandGroup)
	})
}

func (c *client) GetConditionsForTarget(ctx context.Context, t homeassistant.Target, expandGroup *bool) ([]string, error) {
	return c.namesForTarget(ctx, "GetConditionsForTarget", t, func(tt homeassistant.Target) ([]string, error) {
		return c.inner.GetConditionsForTarget(ctx, tt, expandGroup)
	})
}

func (c *client) GetServicesForTarget(ctx context.Context, t homeassistant.Target, expandGroup *bool) ([]string, error) {
	return c.namesForTarget(ctx, "GetServicesForTarget", t, func(tt homeassistant.Target) ([]string, error) {
		return c.inner.GetServicesForTarget(ctx, tt, expandGroup)
	})
}

// ExtractFromTarget masks hidden explicit ids, asks HA, drops hidden objects
// the caller did not name from the result, swaps twins back to the caller's
// ids and sorts every list, so a hidden explicit entity is echoed in
// referenced_entities and a hidden explicit device or area is listed in
// missing_devices or missing_areas, the way HA reports a missing one.
// LIVE-CHECK: HA reports an explicit entity id in referenced_entities whether
// or not it exists, and lists unknown device and area ids in missing_devices
// and missing_areas; pin both against a live instance. The twin approach copies
// whatever HA really does, but the restore step assumes twins appear in exactly
// those lists.
func (c *client) ExtractFromTarget(ctx context.Context, t homeassistant.Target, expandGroup *bool) (*homeassistant.ExtractFromTargetResult, error) {
	if !c.p.HidesAnything() {
		return c.inner.ExtractFromTarget(ctx, t, expandGroup)
	}
	h, err := c.newHideContext(ctx, t)
	if err != nil {
		return nil, c.undecidable("ExtractFromTarget", err)
	}
	res, err := c.inner.ExtractFromTarget(ctx, h.masked(t), expandGroup)
	if err != nil {
		return nil, h.tw.swapError(err)
	}
	if res == nil {
		return nil, nil //nolint:nilnil // the inner client returned neither result nor error: pass that on
	}
	out := *res
	out.ReferencedEntities = canonical(h.tw.restore(h.tw.keepOrTwin(res.ReferencedEntities, func(id string) bool { return !h.hidden(id) })))
	out.ReferencedDevices = canonical(h.tw.restore(h.tw.keepOrTwin(res.ReferencedDevices, func(id string) bool { return !h.hiddenDevs[id] })))
	out.ReferencedAreas = canonical(h.tw.restore(h.tw.keepOrTwin(res.ReferencedAreas, func(string) bool { return !h.areasHidden })))
	out.MissingDevices = canonical(h.tw.restore(res.MissingDevices))
	out.MissingAreas = canonical(h.tw.restore(res.MissingAreas))
	out.MissingFloors = canonical(h.tw.restore(res.MissingFloors))
	out.MissingLabels = canonical(h.tw.restore(res.MissingLabels))
	return &out, nil
}

// knownStrings lists every entity, device, area and label id and name in the
// snapshot, for the twin nonce clash check.
func (d *snapData) knownStrings() []string {
	out := slices.Clone(d.entityIDs)
	for id, dev := range d.devices {
		out = append(out, id, dev.Name, dev.NameByUser)
	}
	for _, a := range d.areas {
		out = append(out, a.AreaID, a.Name)
	}
	for _, l := range d.labels {
		out = append(out, l.LabelID, l.Name)
	}
	return out
}
