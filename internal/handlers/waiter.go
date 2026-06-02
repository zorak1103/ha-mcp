package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// entitySnapshot captures entity state before a service call for change detection.
type entitySnapshot struct {
	EntityID    string
	State       string
	LastChanged time.Time
}

// stateDiff records how an entity's state changed after a service call.
type stateDiff struct {
	EntityID string
	OldState string
	NewState string
	Changed  bool
}

// snapshotEntities captures the current state of each entity.
// Entities that don't exist (GetState error) are silently skipped.
func snapshotEntities(ctx context.Context, client homeassistant.Client, entityIDs []string) []entitySnapshot {
	var snapshots []entitySnapshot
	for _, id := range entityIDs {
		entity, err := client.GetState(ctx, id)
		if err != nil || entity == nil {
			continue
		}
		snapshots = append(snapshots, entitySnapshot{
			EntityID:    entity.EntityID,
			State:       entity.State,
			LastChanged: entity.LastChanged,
		})
	}
	return snapshots
}

// waitForStateChanges polls until all snapshotted entities have changed state or the timeout expires.
// Returns the collected diffs and whether all entities changed (true = all changed, false = timeout).
func waitForStateChanges(ctx context.Context, client homeassistant.Client, snapshots []entitySnapshot) ([]stateDiff, bool) {
	if len(snapshots) == 0 {
		return nil, true
	}

	cfg := pollerConfigFromContext(ctx)
	pollCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	// Build lookup maps for O(1) access
	remaining := make(map[string]entitySnapshot, len(snapshots))
	for _, s := range snapshots {
		remaining[s.EntityID] = s
	}

	diffs := make([]stateDiff, len(snapshots))
	diffIdx := make(map[string]int, len(snapshots))
	for i, s := range snapshots {
		diffs[i] = stateDiff{EntityID: s.EntityID, OldState: s.State}
		diffIdx[s.EntityID] = i
	}

	for {
		select {
		case <-pollCtx.Done():
			return diffs, false
		case <-ticker.C:
			for id, snap := range remaining {
				entity, err := client.GetState(pollCtx, id)
				if err != nil {
					continue
				}
				// Only consider the state changed when the state string actually differs.
				// Checking LastChanged alone can produce misleading "off → off" diffs.
				if entity.State != snap.State {
					i := diffIdx[id]
					diffs[i].NewState = entity.State
					diffs[i].Changed = true
					delete(remaining, id)
				}
			}
			if len(remaining) == 0 {
				return diffs, true
			}
		}
	}
}

// formatStateDiffs builds a string summary to append to service success messages.
// Returns empty string if there are no diffs to report.
func formatStateDiffs(diffs []stateDiff, timedOut bool) string {
	if len(diffs) == 0 {
		return ""
	}

	var changed []string
	var unchanged []string

	for _, d := range diffs {
		if d.Changed {
			changed = append(changed, fmt.Sprintf("%s: %s → %s", d.EntityID, d.OldState, d.NewState))
		} else {
			unchanged = append(unchanged, d.EntityID)
		}
	}

	var parts []string
	if len(changed) > 0 {
		parts = append(parts, "\nState changes: "+strings.Join(changed, ", "))
	}
	if len(unchanged) > 0 && timedOut {
		parts = append(parts, "\n(warning: state change not confirmed within timeout for: "+strings.Join(unchanged, ", ")+")")
	}
	return strings.Join(parts, "")
}

// waitForEntityAppear polls until the entity appears in Home Assistant state.
// Returns (entity, true) on success, (nil, false) on timeout.
func waitForEntityAppear(ctx context.Context, client homeassistant.Client, entityID string) (*homeassistant.Entity, bool) {
	cfg := pollerConfigFromContext(ctx)
	return homeassistant.WaitForEntityAppear(ctx, client.GetState, entityID, cfg)
}

// waitForEntityDisappear polls until the entity disappears from Home Assistant state.
// Returns true when gone, false on timeout.
func waitForEntityDisappear(ctx context.Context, client homeassistant.Client, entityID string) bool {
	cfg := pollerConfigFromContext(ctx)
	return homeassistant.WaitForEntityDisappear(ctx, client.GetState, entityID, cfg)
}

// reloadAndWaitForEntity triggers a domain reload then waits for the entity to appear.
// Reload errors are silently ignored — the entity may still appear even if reload has issues.
// Returns (entity, true) on success, (nil, false) on timeout.
func reloadAndWaitForEntity(ctx context.Context, client homeassistant.Client, domain, entityID string) (*homeassistant.Entity, bool) {
	_, _ = client.CallService(ctx, domain, "reload", nil)
	return waitForEntityAppear(ctx, client, entityID)
}

// waitForTraces polls trace/list until at least one trace is returned or the timeout expires.
// Returns the trace list and whether any were found before the timeout.
// Used when wait=true on manage_trace:list to handle async trace recording.
func waitForTraces(ctx context.Context, client homeassistant.Client, data map[string]any) ([]any, bool) {
	cfg := pollerConfigFromContext(ctx)
	pollCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			return nil, false
		case <-ticker.C:
			response, err := client.SendHACSCommand(pollCtx, "trace/list", data)
			if err != nil {
				continue
			}
			switch v := response.(type) {
			case []any:
				if len(v) > 0 {
					return v, true
				}
			case []map[string]any:
				if len(v) > 0 {
					traces := make([]any, len(v))
					for i, item := range v {
						traces[i] = item
					}
					return traces, true
				}
			}
		}
	}
}

// pollerConfigFromContext converts the MCP WaitConfig from the context into an EntityPollerConfig.
func pollerConfigFromContext(ctx context.Context) homeassistant.EntityPollerConfig {
	wc := mcp.WaitConfigFromContext(ctx)
	return homeassistant.EntityPollerConfig{
		Timeout:      wc.Timeout,
		PollInterval: wc.PollInterval,
	}
}
