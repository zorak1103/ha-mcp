package handlers

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

// decodeFindReferencesJSON runs find_references with format=json and decodes the payload.
func decodeFindReferencesJSON(ctx context.Context, t *testing.T, h *FindReferencesHandlers, client homeassistant.Client, args map[string]any) findReferencesResult {
	t.Helper()

	args["format"] = "json"
	result, err := h.handleFindReferences(ctx, client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected a success result, got error: %s", result.Content[0].Text)
	}

	var payload findReferencesResult
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, result.Content[0].Text)
	}
	return payload
}

// TestFindReferences_ContextExpiryMidScanIsReported verifies that a context that
// ends while a source is still being walked config by config (#321) is reported
// in failed_sources instead of looking like a complete scan, and that the hits
// found before the expiry are still returned.
func TestFindReferences_ContextExpiryMidScanIsReported(t *testing.T) {
	t.Parallel()

	const search = "device_tracker.example_phone"
	matchingSteps := []any{map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": search}}}

	tests := []struct {
		name      string
		source    string
		wantHitID string
		setupMock func(t *testing.T, m *UniversalMockClient)
	}{
		{
			name:      "automation",
			source:    "automation",
			wantHitID: "automation.first",
			setupMock: func(_ *testing.T, m *UniversalMockClient) {
				m.ListAutomationsFn = func(context.Context) ([]homeassistant.Automation, error) {
					return []homeassistant.Automation{{EntityID: "automation.first"}, {EntityID: "automation.second"}}, nil
				}
				m.GetAutomationFn = func(ctx context.Context, id string) (*homeassistant.Automation, error) {
					if id == "first" {
						return &homeassistant.Automation{
							EntityID: "automation.first",
							Config:   &homeassistant.AutomationConfig{Actions: matchingSteps},
						}, nil
					}
					<-ctx.Done()
					return nil, ctx.Err()
				}
			},
		},
		{
			name:      "script",
			source:    "script",
			wantHitID: "script.first",
			setupMock: func(t *testing.T, m *UniversalMockClient) {
				t.Helper()
				m.ListScriptsFn = func(context.Context) ([]homeassistant.Entity, error) {
					return []homeassistant.Entity{{EntityID: "script.first"}, {EntityID: "script.second"}}, nil
				}
				m.GetScriptFn = func(context.Context, string) (*homeassistant.Script, error) {
					t.Error("find_references must not call GetScript")
					return nil, nil
				}
				m.GetScriptConfigFn = func(ctx context.Context, id string) (*homeassistant.ScriptConfig, error) {
					if id == "script.first" {
						return &homeassistant.ScriptConfig{Sequence: matchingSteps}, nil
					}
					<-ctx.Done()
					return nil, ctx.Err()
				}
			},
		},
		{
			name:      "dashboard",
			source:    "dashboard",
			wantHitID: "",
			setupMock: func(_ *testing.T, m *UniversalMockClient) {
				m.ListDashboardsFn = func(context.Context) ([]homeassistant.DashboardEntry, error) {
					return []homeassistant.DashboardEntry{{URLPath: "second"}}, nil
				}
				m.GetLovelaceConfigFn = func(ctx context.Context, urlPath string) (map[string]any, error) {
					if urlPath == "" {
						return map[string]any{"views": []any{map[string]any{"cards": []any{
							map[string]any{"type": "entity", "entity": search},
						}}}}, nil
					}
					<-ctx.Done()
					return nil, ctx.Err()
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &UniversalMockClient{}
			tt.setupMock(t, client)

			// The caller's context stays alive: the expiry comes from the loader's own
			// deadline, because the shared load runs detached from the caller's context.
			// A fetch limit of 1 keeps the fetches sequential, so the first config is
			// read before the second one blocks until the deadline.
			h := NewFindReferencesHandlersWithLoader(newConfigLoader(1, 50*time.Millisecond))
			payload := decodeFindReferencesJSON(context.Background(), t, h, client, map[string]any{
				"search": search,
				"types":  []any{tt.source},
			})

			if !slices.Equal(payload.FailedSources, []string{tt.source}) {
				t.Errorf("failed_sources = %v, want [%s]", payload.FailedSources, tt.source)
			}
			if slices.Contains(payload.ScannedSources, tt.source) {
				t.Errorf("scanned_sources = %v, must not list %s as scanned", payload.ScannedSources, tt.source)
			}
			if len(payload.Hits) != 1 {
				t.Fatalf("expected the one hit found before the expiry, got %+v", payload.Hits)
			}
			if tt.wantHitID != "" && payload.Hits[0].ObjectID != tt.wantHitID {
				t.Errorf("hit object_id = %q, want %q", payload.Hits[0].ObjectID, tt.wantHitID)
			}
		})
	}
}

// TestHandleFindReferences_EnforcesOwnScanDeadline verifies that a scan that
// would outlast the HTTP write timeout (#321) is cut off by the handler's own
// deadline: the slow source is reported in failed_sources while a source that
// finished in time keeps its hits and its scanned status.
func TestHandleFindReferences_EnforcesOwnScanDeadline(t *testing.T) {
	t.Parallel()

	const search = "device_tracker.example_phone"

	// The loader's own 50 ms load timeout keeps the stalled mock from leaking for 25 s.
	h := NewFindReferencesHandlersWithLoader(newConfigLoader(configFetchLimit, 50*time.Millisecond))
	h.scanTimeout = 50 * time.Millisecond

	client := &UniversalMockClient{
		ListAutomationsFn: func(ctx context.Context) ([]homeassistant.Automation, error) {
			<-ctx.Done() // a stalled upstream: only the deadline ends this call
			return nil, ctx.Err()
		},
		ListScenesFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{{
				EntityID:   "scene.evening",
				Attributes: map[string]any{attrEntityID: []any{search}},
			}}, nil
		},
	}

	// The outer deadline is a safety net so a missing handler deadline fails the
	// test below instead of hanging it.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	began := time.Now()
	payload := decodeFindReferencesJSON(ctx, t, h, client, map[string]any{
		"search": search,
		"types":  []any{"automation", "scene"},
	})
	if elapsed := time.Since(began); elapsed > 2*time.Second {
		t.Errorf("handler took %s, expected it to stop at its own %s scan deadline", elapsed, h.scanTimeout)
	}

	if !slices.Equal(payload.FailedSources, []string{"automation"}) {
		t.Errorf("failed_sources = %v, want [automation]", payload.FailedSources)
	}
	if !slices.Equal(payload.ScannedSources, []string{"scene"}) {
		t.Errorf("scanned_sources = %v, want [scene]", payload.ScannedSources)
	}
	if len(payload.Hits) != 1 || payload.Hits[0].ObjectID != "scene.evening" {
		t.Errorf("expected the scene hit to survive the automation timeout, got %+v", payload.Hits)
	}
}

// TestHandleFindReferences_CallerDeadlineKeepsPartialHits uses production-like
// ordering: the handler's scan deadline starts before the shared load and ends
// before the loader's own timeout, and the hit read before the deadline is kept.
func TestHandleFindReferences_CallerDeadlineKeepsPartialHits(t *testing.T) {
	t.Parallel()

	const search = "device_tracker.example_phone"
	h := NewFindReferencesHandlersWithLoader(newConfigLoader(1, time.Minute))
	h.scanTimeout = 600 * time.Millisecond

	client := &UniversalMockClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			return []homeassistant.Automation{{EntityID: "automation.first"}, {EntityID: "automation.second"}}, nil
		},
		GetAutomationFn: func(ctx context.Context, id string) (*homeassistant.Automation, error) {
			if id == "first" {
				return &homeassistant.Automation{
					EntityID: "automation.first",
					Config: &homeassistant.AutomationConfig{Actions: []any{
						map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": search}},
					}},
				}, nil
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	payload := decodeFindReferencesJSON(context.Background(), t, h, client, map[string]any{
		"search": search,
		"types":  []any{"automation"},
	})

	if !slices.Equal(payload.FailedSources, []string{"automation"}) {
		t.Errorf("failed_sources = %v, want [automation]", payload.FailedSources)
	}
	if len(payload.Hits) != 1 || payload.Hits[0].ObjectID != "automation.first" {
		t.Errorf("hits = %+v, want the automation.first hit read before the deadline", payload.Hits)
	}
}

// TestNewFindReferencesHandlers_ScanDeadlineLeavesWriteTimeoutHeadroom pins that the
// default scan deadline ends before the server's 30s HTTP write timeout
// (internal/mcp/server.go), leaving time to render the response; otherwise
// the connection is closed with an empty reply instead of an explicit failure.
func TestNewFindReferencesHandlers_ScanDeadlineLeavesWriteTimeoutHeadroom(t *testing.T) {
	t.Parallel()

	const httpWriteTimeout = 30 * time.Second
	got := NewFindReferencesHandlers().scanTimeout
	if got <= 0 || got >= httpWriteTimeout {
		t.Errorf("default scanTimeout = %s, want 0 < timeout < %s", got, httpWriteTimeout)
	}
}
