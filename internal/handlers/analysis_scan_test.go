package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
)

const scanTestArea = "living_room"

// analyzeEntityJSON runs handleAnalyzeEntity with format=json and decodes the
// result, failing the test on any error. Call it from the test goroutine only;
// other goroutines use analyzeEntityJSONResult.
func analyzeEntityJSON(ctx context.Context, t *testing.T, h *AnalysisHandlers, client homeassistant.Client, entityID string) EntityAnalysis {
	t.Helper()
	analysis, err := analyzeEntityJSONResult(ctx, h, client, entityID)
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}

// analyzeEntityJSONResult runs handleAnalyzeEntity with format=json and decodes the
// result, returning failures as an error so it is safe off the test goroutine.
func analyzeEntityJSONResult(ctx context.Context, h *AnalysisHandlers, client homeassistant.Client, entityID string) (EntityAnalysis, error) {
	result, err := h.handleAnalyzeEntity(ctx, client, map[string]any{attrEntityID: entityID, "format": formatJSON})
	if err != nil {
		return EntityAnalysis{}, fmt.Errorf("handleAnalyzeEntity(%s) error = %w", entityID, err)
	}
	if result.IsError {
		return EntityAnalysis{}, fmt.Errorf("handleAnalyzeEntity(%s) returned error result: %v", entityID, result.Content)
	}
	var analysis EntityAnalysis
	if err := json.Unmarshal([]byte(result.Content[0].Text), &analysis); err != nil {
		return EntityAnalysis{}, fmt.Errorf("decoding analysis JSON: %w\n%s", err, result.Content[0].Text)
	}
	if analysis.References == nil {
		return EntityAnalysis{}, fmt.Errorf("analysis has no references: %s", result.Content[0].Text)
	}
	return analysis, nil
}

// entityInArea returns an entity registry mock that puts entityID in area.
func entityInArea(entityID, area string) func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	return func(context.Context) ([]homeassistant.EntityRegistryEntry, error) {
		return []homeassistant.EntityRegistryEntry{{EntityID: entityID, AreaID: area}}, nil
	}
}

func areaAction(area string) []any {
	return []any{map[string]any{"action": "light.turn_off", "target": map[string]any{"area_id": area}}}
}

func entityAction(entityID string) []any {
	return []any{map[string]any{"action": "light.turn_on", "target": map[string]any{"entity_id": entityID}}}
}

func TestHandleAnalyzeEntity_FetchesEachConfigOnce(t *testing.T) {
	t.Parallel()

	const entityID = "light.lamp"
	var getAutomation, getScriptConfig, listAutomations, listScripts atomic.Int32
	client := &mockAnalysisClient{
		GetEntityRegistryFn: entityInArea(entityID, scanTestArea),
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			listAutomations.Add(1)
			return []homeassistant.Automation{{EntityID: "automation.a"}, {EntityID: "automation.b"}, {EntityID: "automation.c"}}, nil
		},
		GetAutomationFn: func(_ context.Context, id string) (*homeassistant.Automation, error) {
			getAutomation.Add(1)
			return &homeassistant.Automation{
				EntityID: "automation." + id,
				Config:   &homeassistant.AutomationConfig{Actions: append(entityAction(entityID), areaAction(scanTestArea)...)},
			}, nil
		},
		ListScriptsFn: func(context.Context) ([]homeassistant.Entity, error) {
			listScripts.Add(1)
			return []homeassistant.Entity{{EntityID: "script.a"}, {EntityID: "script.b"}}, nil
		},
		GetScriptFn: func(context.Context, string) (*homeassistant.Script, error) {
			t.Error("analyze_entity must not call GetScript")
			return nil, errors.New("unexpected GetScript")
		},
		GetScriptConfigFn: func(context.Context, string) (*homeassistant.ScriptConfig, error) {
			getScriptConfig.Add(1)
			return &homeassistant.ScriptConfig{Sequence: append(entityAction(entityID), areaAction(scanTestArea)...)}, nil
		},
	}

	analysis := analyzeEntityJSON(context.Background(), t, NewAnalysisHandlers(), client, entityID)

	if got := getAutomation.Load(); got != 3 {
		t.Errorf("GetAutomation calls = %d, want 3 (once per automation)", got)
	}
	if got := getScriptConfig.Load(); got != 2 {
		t.Errorf("GetScriptConfig calls = %d, want 2 (once per script)", got)
	}
	if got := listAutomations.Load(); got != 1 {
		t.Errorf("ListAutomations calls = %d, want 1", got)
	}
	if got := listScripts.Load(); got != 1 {
		t.Errorf("ListScripts calls = %d, want 1", got)
	}
	refs := analysis.References
	if len(refs.Automations) != 3 || len(refs.Scripts) != 2 || len(refs.AreaReferences) != 5 {
		t.Errorf("references = %d automations, %d scripts, %d area refs; want 3, 2, 5",
			len(refs.Automations), len(refs.Scripts), len(refs.AreaReferences))
	}
}

func TestHandleAnalyzeEntity_FindsAreaReferenceInScriptConfig(t *testing.T) {
	t.Parallel()

	const entityID = "light.lamp"
	client := &mockAnalysisClient{
		GetEntityRegistryFn: entityInArea(entityID, scanTestArea),
		ListScriptsFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{{EntityID: "script.x"}}, nil
		},
		GetScriptConfigFn: func(context.Context, string) (*homeassistant.ScriptConfig, error) {
			return &homeassistant.ScriptConfig{Sequence: areaAction(scanTestArea)}, nil
		},
	}

	analysis := analyzeEntityJSON(context.Background(), t, NewAnalysisHandlers(), client, entityID)

	want := []AreaReference{{EntityID: "script.x", Type: "script", AreaID: scanTestArea, UsedIn: []string{"action"}}}
	if !reflect.DeepEqual(analysis.References.AreaReferences, want) {
		t.Errorf("area_references = %+v, want %+v", analysis.References.AreaReferences, want)
	}
}

func TestFindAreaScriptReferences_ReportsConfigFetchFailure(t *testing.T) {
	t.Parallel()

	const entityID = "light.lamp"
	client := &mockAnalysisClient{
		GetEntityRegistryFn: entityInArea(entityID, scanTestArea),
		ListScriptsFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{{EntityID: "script.ok"}, {EntityID: "script.broken"}}, nil
		},
		GetScriptConfigFn: func(_ context.Context, id string) (*homeassistant.ScriptConfig, error) {
			if id == "script.broken" {
				return nil, errors.New("boom")
			}
			return &homeassistant.ScriptConfig{Sequence: areaAction("kitchen")}, nil
		},
	}

	err := NewAnalysisHandlers().findAreaScriptReferences(context.Background(), client, scanTestArea, &EntityReferences{})
	if err == nil || !strings.Contains(err.Error(), "getting script script.broken: boom") {
		t.Fatalf("expected the config fetch failure, got %v", err)
	}

	analysis := analyzeEntityJSON(context.Background(), t, NewAnalysisHandlers(), client, entityID)
	if !slices.Contains(analysis.References.FailedSources, "areas") {
		t.Errorf("failed_sources = %v, want it to contain areas", analysis.References.FailedSources)
	}
}

// TestHandleAnalyzeEntity_EnforcesOwnScanDeadline mirrors
// TestHandleFindReferences_EnforcesOwnScanDeadline: a stalled source is cut off by
// the handler's own deadline and reported in failed_sources, while a source that
// finished in time keeps its references.
func TestHandleAnalyzeEntity_EnforcesOwnScanDeadline(t *testing.T) {
	t.Parallel()

	const entityID = "device_tracker.example_phone"

	// The loader's own 50 ms load timeout keeps the stalled mock from leaking for 25 s.
	h := NewAnalysisHandlersWithLoader(newConfigLoader(configFetchLimit, 50*time.Millisecond))
	h.scanTimeout = 50 * time.Millisecond

	client := &mockAnalysisClient{
		ListAutomationsFn: func(ctx context.Context) ([]homeassistant.Automation, error) {
			<-ctx.Done() // a stalled upstream: only the deadline ends this call
			return nil, ctx.Err()
		},
		ListScenesFn: func(context.Context) ([]homeassistant.Entity, error) {
			return []homeassistant.Entity{{
				EntityID:   "scene.evening",
				Attributes: map[string]any{attrEntityID: []any{entityID}},
			}}, nil
		},
	}

	// The outer deadline is a safety net so a missing handler deadline fails the
	// test below instead of hanging it.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	began := time.Now()
	analysis := analyzeEntityJSON(ctx, t, h, client, entityID)
	if elapsed := time.Since(began); elapsed > 2*time.Second {
		t.Errorf("handler took %s, expected it to stop at its own %s scan deadline", elapsed, h.scanTimeout)
	}

	refs := analysis.References
	if !slices.Contains(refs.FailedSources, "automations") {
		t.Errorf("failed_sources = %v, want it to contain automations", refs.FailedSources)
	}
	if len(refs.Scenes) != 1 || refs.Scenes[0].EntityID != "scene.evening" {
		t.Errorf("expected the scene reference to survive the automation timeout, got %+v", refs.Scenes)
	}
}

// TestNewAnalysisHandlers_ScanDeadlineLeavesWriteTimeoutHeadroom pins that the
// default scan deadline ends before the server's 30s HTTP write timeout
// (internal/mcp/server.go), leaving time to render the response.
func TestNewAnalysisHandlers_ScanDeadlineLeavesWriteTimeoutHeadroom(t *testing.T) {
	t.Parallel()

	const httpWriteTimeout = 30 * time.Second
	got := NewAnalysisHandlers().scanTimeout
	if got <= 0 || got >= httpWriteTimeout {
		t.Errorf("default scanTimeout = %s, want 0 < timeout < %s", got, httpWriteTimeout)
	}
}

func TestHandleAnalyzeEntity_ParallelCallsShareLoads(t *testing.T) {
	t.Parallel()

	entityIDs := []string{"light.a", "light.b"}
	var lists, gets atomic.Int32
	release := make(chan struct{})
	client := &mockAnalysisClient{
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			lists.Add(1)
			<-release
			return []homeassistant.Automation{{EntityID: "automation.a"}, {EntityID: "automation.b"}}, nil
		},
		GetAutomationFn: func(_ context.Context, id string) (*homeassistant.Automation, error) {
			gets.Add(1)
			return &homeassistant.Automation{
				EntityID: "automation." + id,
				Config:   &homeassistant.AutomationConfig{Actions: entityAction("light." + id)},
			}, nil
		},
	}
	loader := newConfigLoader(configFetchLimit, time.Minute)
	joined := joinSignal(t, loader)
	h := NewAnalysisHandlersWithLoader(loader)

	analyses := make([]EntityAnalysis, len(entityIDs))
	var wg sync.WaitGroup
	for i, entityID := range entityIDs {
		wg.Go(func() {
			analysis, err := analyzeEntityJSONResult(context.Background(), h, client, entityID)
			if err != nil {
				t.Errorf("call %d: %v", i, err)
				return
			}
			analyses[i] = analysis
		})
	}
	// Each call joins four loads (automations, scripts, dashboards, helper
	// templates); the automation load cannot finish before release, so after
	// eight joins both calls are waiting on the same automation load.
	waitJoins(t, joined, 8)
	close(release)
	wg.Wait()
	expectNoMoreJoins(t, joined)

	if got := lists.Load(); got != 1 {
		t.Errorf("ListAutomations calls = %d, want 1", got)
	}
	if got := gets.Load(); got != 2 {
		t.Errorf("GetAutomation calls = %d, want 2 (once per automation in total)", got)
	}
	for i, entityID := range entityIDs {
		autos := analyses[i].References.Automations
		if len(autos) != 1 || autos[0].EntityID != "automation."+strings.TrimPrefix(entityID, "light.") {
			t.Errorf("%s: automation references = %+v, want exactly its own automation", entityID, autos)
		}
	}
}

func TestHandleAnalyzeEntity_SourcePanicIsReported(t *testing.T) {
	t.Parallel()

	const entityID = "light.lamp"
	client := &mockAnalysisClient{
		ListScenesFn: func(context.Context) ([]homeassistant.Entity, error) {
			panic("scene listing exploded")
		},
		ListAutomationsFn: func(context.Context) ([]homeassistant.Automation, error) {
			return []homeassistant.Automation{{EntityID: "automation.a"}}, nil
		},
		GetAutomationFn: func(context.Context, string) (*homeassistant.Automation, error) {
			return &homeassistant.Automation{
				EntityID: "automation.a",
				Config:   &homeassistant.AutomationConfig{Actions: entityAction(entityID)},
			}, nil
		},
	}

	analysis := analyzeEntityJSON(context.Background(), t, NewAnalysisHandlers(), client, entityID)

	refs := analysis.References
	if !slices.Contains(refs.FailedSources, "scenes") {
		t.Errorf("failed_sources = %v, want it to contain scenes", refs.FailedSources)
	}
	if !slices.Contains(refs.ScannedSources, "automations") || len(refs.Automations) != 1 {
		t.Errorf("other sources should still be scanned: scanned=%v automations=%+v", refs.ScannedSources, refs.Automations)
	}
}
