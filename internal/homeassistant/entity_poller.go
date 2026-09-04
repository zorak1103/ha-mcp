// Package homeassistant provides low-level entity polling utilities.
package homeassistant

import (
	"context"
	"time"
)

// GetStateFn is a function type for looking up an entity by ID.
// Returns (entity, nil) if found, (nil, error) if not found or on error.
type GetStateFn func(ctx context.Context, entityID string) (*Entity, error)

// EntityPollerConfig configures polling behavior for entity state checks.
type EntityPollerConfig struct {
	Timeout      time.Duration
	PollInterval time.Duration
}

// DefaultEntityPollerConfig returns sensible defaults: 5s timeout, 100ms poll interval.
func DefaultEntityPollerConfig() EntityPollerConfig {
	return EntityPollerConfig{
		Timeout:      5 * time.Second,
		PollInterval: 100 * time.Millisecond,
	}
}

// entityPollerConfigCtxKey is an unexported context key type, avoiding
// collisions with keys defined by other packages.
type entityPollerConfigCtxKey struct{}

// WithEntityPollerConfig stamps an EntityPollerConfig onto ctx so callers
// deeper in the stack (e.g. CreateHelperEntity's config-entry resolution)
// can honor a caller-configured wait instead of a hardcoded default. Used
// because internal/homeassistant cannot import internal/mcp's WaitConfig
// directly - the handler layer converts and stamps it here.
func WithEntityPollerConfig(ctx context.Context, cfg EntityPollerConfig) context.Context {
	return context.WithValue(ctx, entityPollerConfigCtxKey{}, cfg)
}

// EntityPollerConfigFromContext retrieves a config stamped by
// WithEntityPollerConfig. Returns (cfg, false) when none was stamped.
func EntityPollerConfigFromContext(ctx context.Context) (EntityPollerConfig, bool) {
	cfg, ok := ctx.Value(entityPollerConfigCtxKey{}).(EntityPollerConfig)
	return cfg, ok
}

// WaitForEntityAppear polls until the entity with entityID exists.
// Returns (entity, true) when the entity appears, or (nil, false) on timeout.
// Never returns an error — timeouts are reported via the bool return value.
func WaitForEntityAppear(ctx context.Context, getState GetStateFn, entityID string, cfg EntityPollerConfig) (*Entity, bool) {
	pollCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			return nil, false
		case <-ticker.C:
			entity, err := getState(pollCtx, entityID)
			if err == nil && entity != nil {
				return entity, true
			}
		}
	}
}

// GetEntityRegistryFn is a function type for fetching the full entity registry.
type GetEntityRegistryFn func(ctx context.Context) ([]EntityRegistryEntry, error)

// ConfigEntryResolvePollerConfig returns defaults for resolving a Config
// Entry's assigned entity_id: 5s timeout, 500ms poll interval. Each tick is a
// FULL entity-registry fetch (unlike WaitForEntityAppear's single-entity
// GetState), so this is deliberately coarser than DefaultEntityPollerConfig
// to avoid hammering Home Assistant with full registry reads.
func ConfigEntryResolvePollerConfig() EntityPollerConfig {
	return EntityPollerConfig{
		Timeout:      5 * time.Second,
		PollInterval: 500 * time.Millisecond,
	}
}

// findConfigEntryEntity returns the entity_id of the registry entry whose
// ConfigEntryID matches entryID. A config entry can register more than one
// entity - e.g. a tariffed utility_meter registers a select.* tariff
// selector plus one sensor.* per tariff, select first - so "first match
// wins" would silently return the wrong domain. When preferDomain is
// non-empty, only candidates in that domain are eligible. Returns "" when no
// eligible candidate exists yet. Multiple eligible candidates resolve to the
// lexicographically smallest entity_id, for a deterministic result.
func findConfigEntryEntity(registry []EntityRegistryEntry, entryID, preferDomain string) string {
	var best string
	for _, entry := range registry {
		if entry.ConfigEntryID != entryID {
			continue
		}
		if preferDomain != "" && extractEntityDomain(entry.EntityID) != preferDomain {
			continue
		}
		if best == "" {
			best = entry.EntityID
			continue
		}
		best = min(best, entry.EntityID)
	}
	return best
}

// WaitForConfigEntryEntity polls the entity registry until an eligible entry
// with the given config_entry_id appears, returning its entity_id. When
// preferDomain is non-empty, an entity in a different domain does not
// satisfy the wait (see findConfigEntryEntity) - the caller falls back to
// its own prediction instead of receiving a wrong-domain id.
// Returns ("", false, lastErr) on timeout; lastErr is the most recent
// registry fetch error, if any, so a persistent failure isn't silently
// indistinguishable from "not created yet". Never returns a non-nil error
// directly - timeouts are reported via the bool return value.
func WaitForConfigEntryEntity(
	ctx context.Context, getRegistry GetEntityRegistryFn, entryID, preferDomain string, cfg EntityPollerConfig,
) (string, bool, error) {
	pollCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	var lastErr error
	check := func() (string, bool) {
		registry, err := getRegistry(pollCtx)
		if err != nil {
			lastErr = err
			return "", false
		}
		lastErr = nil
		if entityID := findConfigEntryEntity(registry, entryID, preferDomain); entityID != "" {
			return entityID, true
		}
		return "", false
	}

	// Check immediately so an already-registered entity doesn't pay a
	// guaranteed extra PollInterval before being noticed.
	if entityID, ok := check(); ok {
		return entityID, true, nil
	}

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			return "", false, lastErr
		case <-ticker.C:
			if entityID, ok := check(); ok {
				return entityID, true, nil
			}
		}
	}
}

// notFoundThreshold is the number of consecutive not-found errors required to
// confirm an entity has disappeared. This prevents a single network blip from
// being misinterpreted as successful deletion (transient errors also return error
// from GetState and cannot be distinguished from "entity not found").
const notFoundThreshold = 2

// WaitForEntityDisappear polls until the entity with entityID no longer exists.
// Returns true when the entity is gone, or false on timeout.
// Requires notFoundThreshold consecutive not-found errors to confirm disappearance,
// preventing transient network errors from producing false positives.
// Never returns an error — timeouts are reported via the bool return value.
func WaitForEntityDisappear(ctx context.Context, getState GetStateFn, entityID string, cfg EntityPollerConfig) bool {
	pollCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	consecutiveNotFound := 0
	for {
		select {
		case <-pollCtx.Done():
			return false
		case <-ticker.C:
			_, err := getState(pollCtx, entityID)
			if err != nil {
				consecutiveNotFound++
				if consecutiveNotFound >= notFoundThreshold {
					return true
				}
			} else {
				// Entity still exists — reset the counter
				consecutiveNotFound = 0
			}
		}
	}
}
