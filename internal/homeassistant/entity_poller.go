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

// WaitForConfigEntryEntity polls the entity registry until an entry with the
// given config_entry_id appears, returning its entity_id.
// Returns ("", false) on timeout or persistent registry errors — never an error.
func WaitForConfigEntryEntity(ctx context.Context, getRegistry GetEntityRegistryFn, entryID string, cfg EntityPollerConfig) (string, bool) {
	pollCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			return "", false
		case <-ticker.C:
			registry, err := getRegistry(pollCtx)
			if err != nil {
				continue
			}
			for _, entry := range registry {
				if entry.ConfigEntryID == entryID {
					return entry.EntityID, true
				}
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
