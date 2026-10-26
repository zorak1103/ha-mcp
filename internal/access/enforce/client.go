// Package enforce wraps a homeassistant.Client so every call is checked against
// an access.Policy. The wrapper lists every Client method explicitly (no
// embedding): a method added to the interface breaks the build here until it
// is given an enforcement decision, so no new operation can slip past the
// policy. A method that has no decision yet fails closed.
package enforce

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/zorak1103/ha-mcp/internal/access"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// client enforces p on every call to inner. It is created per MCP tool call,
// so snap is a per-request view of the registries. It deliberately does not
// implement Close: the factory closes the unwrapped client.
type client struct {
	inner homeassistant.Client
	p     *access.Policy
	tr    *access.Tracker
	snap  *snapshot
	// created holds the entity ids this request created (helper creates), so
	// a follow-up write in the same request finds them before the registry
	// lists them.
	created createdSet
}

// createdSet is a concurrency-safe set of entity ids; the zero value is empty.
type createdSet struct {
	mu  sync.Mutex
	ids map[string]bool
}

func (s *createdSet) add(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids == nil {
		s.ids = map[string]bool{}
	}
	s.ids[id] = true
}

func (s *createdSet) has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ids[id]
}

var _ homeassistant.Client = (*client)(nil)

// Wrap returns a client that enforces p on inner and records denials on tr.
// An unrestricted (or nil) policy returns inner itself, so the common
// default-write configuration pays no cost.
func Wrap(p *access.Policy, inner homeassistant.Client, tr *access.Tracker) homeassistant.Client {
	if p == nil || p.Unrestricted() {
		return inner
	}
	return &client{inner: inner, p: p, tr: tr, snap: &snapshot{inner: inner}}
}

// InvalidateLabelRegistryCache forwards to the inner client when it supports
// the optional method and drops the registry snapshot, whose label view is
// stale after a label write.
func (c *client) InvalidateLabelRegistryCache() {
	if r, ok := c.inner.(interface{ InvalidateLabelRegistryCache() }); ok {
		r.InvalidateLabelRegistryCache()
	}
	c.snap.invalidate()
}

// unclassified records a denial for a method that has no enforcement decision
// yet and returns the error the caller sees. The method name goes to the log
// only; the message reads "operation not permitted".
func (c *client) unclassified(method string) error {
	return c.refuse(method)
}

// refuse records a denial without capability for object (a method or
// operation name, shown in the log only) and returns the error the caller sees:
// "operation not permitted". It is the refusal for anything the policy cannot
// decide or does not allow without naming an object.
func (c *client) refuse(object string) error {
	d := access.Denial{Tool: c.tr.Tool(), Object: object}
	c.tr.Deny(d)
	return errors.New(d.Message())
}

// undecidable is the fail-closed answer when a decision needs registry data
// that could not be fetched: the cause goes to the log, the caller sees the
// generic refusal (never snapshot internals, never "not found"). Whether the
// fetch fails does not depend on the requested object, so this is no oracle.
func (c *client) undecidable(method string, cause error) error {
	slog.Warn("access decision failed, refusing", "method", method, "error", cause)
	return c.refuse(method)
}

// entityIDRE is the shape of a Home Assistant entity id after normalisation.
var entityIDRE = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)

// normalizeEntityID trims and lower-cases s and reports whether the result is
// a valid entity id. The policy engine does not validate id shapes, so every
// decision goes through here first.
func normalizeEntityID(s string) (string, bool) {
	id := strings.ToLower(strings.TrimSpace(s))
	return id, entityIDRE.MatchString(id)
}

// decideEntity returns the grant on the entity entityID. An id that is not a
// valid entity id is hidden (the zero Grant). Registry attributes are
// fetched only when the policy has area, label or device matchers; a failed
// fetch returns the zero Grant and the error so the caller fails closed.
func (c *client) decideEntity(ctx context.Context, entityID string) (access.Grant, error) {
	if _, ok := normalizeEntityID(entityID); !ok {
		return access.Grant{}, nil
	}
	grant, err := c.grantFunc(ctx)
	if err != nil {
		return access.Grant{}, err
	}
	return grant(entityID), nil
}

// grantFunc returns a function that decides entity ids with the registry view
// fetched once, for callers that decide many ids (list filters). Same rules as
// decideEntity: an invalid id is hidden; the registry is fetched only when the
// policy has attribute matchers, and a failed fetch is returned as an error.
func (c *client) grantFunc(ctx context.Context) (func(entityID string) access.Grant, error) {
	var data *snapData
	if c.p.NeedsRegistry() {
		var err error
		if data, err = c.snap.get(ctx); err != nil {
			return nil, err
		}
	}
	return func(entityID string) access.Grant {
		id, ok := normalizeEntityID(entityID)
		if !ok {
			return access.Grant{}
		}
		obj := access.Object{Kind: access.KindEntity, ID: id}
		if data != nil {
			obj = data.objectFor(id)
		}
		return c.p.Decide(obj)
	}, nil
}

// decideDashboard returns the grant on the dashboard with the given url_path;
// the empty path is the default dashboard.
func (c *client) decideDashboard(urlPath string) access.Grant {
	id := strings.ToLower(strings.TrimSpace(urlPath))
	if id == "" {
		id = access.DefaultDashboardID
	}
	return c.p.Decide(access.Object{Kind: access.KindDashboard, ID: id})
}

// filterCopy returns the elements of in for which keep is true in a new
// slice. It never modifies in or its backing array: cached clients hand out
// shared slices.
func filterCopy[T any](in []T, keep func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}
