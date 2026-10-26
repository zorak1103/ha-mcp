package handlers

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// injectFastWait returns a context with a fast wait config for unit tests.
// Uses a fixed 10ms poll interval; pass the desired timeout.
func injectFastWait(timeout time.Duration) context.Context {
	return mcp.WithWaitConfig(context.Background(), mcp.WaitConfig{
		Timeout:      timeout,
		PollInterval: 10 * time.Millisecond,
	})
}

func TestSnapshotEntities_AllExist(t *testing.T) {
	baseTime := time.Now()
	client := &UniversalMockClient{
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			switch entityID {
			case "light.living_room":
				return &homeassistant.Entity{EntityID: entityID, State: "off", LastChanged: baseTime}, nil
			case "light.bedroom":
				return &homeassistant.Entity{EntityID: entityID, State: "on", LastChanged: baseTime}, nil
			}
			return nil, errors.New("not found")
		},
	}

	snapshots := snapshotEntities(context.Background(), client, []string{"light.living_room", "light.bedroom"})
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}
}

func TestSnapshotEntities_SkipsMissing(t *testing.T) {
	client := &UniversalMockClient{
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			if entityID == "light.living_room" {
				return &homeassistant.Entity{EntityID: entityID, State: "off"}, nil
			}
			return nil, errors.New("not found")
		},
	}

	snapshots := snapshotEntities(context.Background(), client, []string{"light.living_room", "light.nonexistent"})
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot (nonexistent skipped), got %d", len(snapshots))
	}
	if snapshots[0].EntityID != "light.living_room" {
		t.Errorf("expected light.living_room, got %s", snapshots[0].EntityID)
	}
}

func TestWaitForStateChanges_EmptySnapshots(t *testing.T) {
	client := &UniversalMockClient{}
	diffs, allChanged := waitForStateChanges(context.Background(), client, nil)
	if !allChanged {
		t.Error("expected allChanged=true for empty snapshots")
	}
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs, got %d", len(diffs))
	}
}

func TestWaitForStateChanges_StateChanges(t *testing.T) {
	beforeTime := time.Now().Add(-2 * time.Second)
	afterTime := time.Now()

	var callCount int32
	client := &UniversalMockClient{
		GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n < 3 {
				return &homeassistant.Entity{EntityID: "light.x", State: "off", LastChanged: beforeTime}, nil
			}
			return &homeassistant.Entity{EntityID: "light.x", State: "on", LastChanged: afterTime}, nil
		},
	}

	snap := entitySnapshot{EntityID: "light.x", State: "off", LastChanged: beforeTime}
	ctx := injectFastWait(500 * time.Millisecond)

	diffs, allChanged := waitForStateChanges(ctx, client, []entitySnapshot{snap})
	if !allChanged {
		t.Error("expected allChanged=true")
	}
	if len(diffs) != 1 || !diffs[0].Changed {
		t.Error("expected diff to show changed=true")
	}
	if diffs[0].NewState != "on" {
		t.Errorf("expected new state 'on', got %q", diffs[0].NewState)
	}
}

func TestWaitForStateChanges_Timeout(t *testing.T) {
	beforeTime := time.Now().Add(-2 * time.Second)

	client := &UniversalMockClient{
		GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
			// State never changes
			return &homeassistant.Entity{EntityID: "light.x", State: "off", LastChanged: beforeTime}, nil
		},
	}

	snap := entitySnapshot{EntityID: "light.x", State: "off", LastChanged: beforeTime}
	ctx := injectFastWait(50 * time.Millisecond)

	diffs, allChanged := waitForStateChanges(ctx, client, []entitySnapshot{snap})
	if allChanged {
		t.Error("expected allChanged=false on timeout")
	}
	if len(diffs) != 1 || diffs[0].Changed {
		t.Error("expected unchanged diff on timeout")
	}
}

func TestFormatStateDiffs_Empty(t *testing.T) {
	result := formatStateDiffs(nil, false)
	if result != "" {
		t.Errorf("expected empty string for nil diffs, got %q", result)
	}
}

func TestFormatStateDiffs_WithChanges(t *testing.T) {
	diffs := []stateDiff{
		{EntityID: "light.x", OldState: "off", NewState: "on", Changed: true},
	}
	result := formatStateDiffs(diffs, false)
	if !strings.Contains(result, "light.x: off → on") {
		t.Errorf("expected 'light.x: off → on' in output, got %q", result)
	}
}

func TestFormatStateDiffs_WithTimeout(t *testing.T) {
	diffs := []stateDiff{
		{EntityID: "light.x", OldState: "off", NewState: "", Changed: false},
	}
	result := formatStateDiffs(diffs, true)
	if !strings.Contains(result, "warning") {
		t.Errorf("expected warning in timeout output, got %q", result)
	}
	if !strings.Contains(result, "light.x") {
		t.Errorf("expected entity ID in warning, got %q", result)
	}
}

func TestFormatStateDiffs_MixedChangedAndTimeout(t *testing.T) {
	diffs := []stateDiff{
		{EntityID: "light.x", OldState: "off", NewState: "on", Changed: true},
		{EntityID: "light.y", OldState: "on", NewState: "", Changed: false},
	}
	result := formatStateDiffs(diffs, true)
	if !strings.Contains(result, "light.x: off → on") {
		t.Errorf("expected changed entity in output, got %q", result)
	}
	if !strings.Contains(result, "warning") || !strings.Contains(result, "light.y") {
		t.Errorf("expected warning with light.y in output, got %q", result)
	}
}

func TestWaitForEntityAppear_Integration(t *testing.T) {
	var callCount int32
	client := &UniversalMockClient{
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n < 4 {
				return nil, errors.New("not found")
			}
			return &homeassistant.Entity{EntityID: entityID, State: "on"}, nil
		},
	}

	ctx := injectFastWait(500 * time.Millisecond)
	entity, ok := waitForEntityAppear(ctx, client, "light.x")
	if !ok {
		t.Fatal("expected entity to appear")
	}
	if entity.EntityID != "light.x" {
		t.Errorf("expected light.x, got %s", entity.EntityID)
	}
}

func TestWaitForEntityAppear_Timeout(t *testing.T) {
	client := &UniversalMockClient{
		GetStateFn: func(context.Context, string) (*homeassistant.Entity, error) {
			return nil, errors.New("not found")
		},
	}

	ctx := injectFastWait(50 * time.Millisecond)
	_, ok := waitForEntityAppear(ctx, client, "light.x")
	if ok {
		t.Error("expected timeout (false)")
	}
}

func TestWaitForEntityDisappear_Integration(t *testing.T) {
	var callCount int32
	client := &UniversalMockClient{
		GetStateFn: func(_ context.Context, _ string) (*homeassistant.Entity, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n < 4 {
				return &homeassistant.Entity{EntityID: "light.x"}, nil
			}
			return nil, errors.New("not found")
		},
	}

	ctx := injectFastWait(500 * time.Millisecond)
	ok := waitForEntityDisappear(ctx, client, "light.x")
	if !ok {
		t.Fatal("expected entity to disappear")
	}
}

func TestWaitForEntityDisappear_Timeout(t *testing.T) {
	client := &UniversalMockClient{
		GetStateFn: func(context.Context, string) (*homeassistant.Entity, error) {
			return &homeassistant.Entity{EntityID: "light.x"}, nil
		},
	}

	ctx := injectFastWait(50 * time.Millisecond)
	ok := waitForEntityDisappear(ctx, client, "light.x")
	if ok {
		t.Error("expected timeout (false)")
	}
}

func TestReloadAndWaitForEntity(t *testing.T) {
	var callServiceCalled bool
	var callCount int32

	client := &UniversalMockClient{
		CallServiceFn: func(_ context.Context, domain, service string, _ map[string]any) ([]homeassistant.Entity, error) {
			if domain == "automation" && service == "reload" {
				callServiceCalled = true
			}
			return nil, nil
		},
		GetStateFn: func(_ context.Context, entityID string) (*homeassistant.Entity, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n < 3 {
				return nil, errors.New("not found")
			}
			return &homeassistant.Entity{EntityID: entityID, State: "on"}, nil
		},
	}

	ctx := injectFastWait(500 * time.Millisecond)
	entity, ok := reloadAndWaitForEntity(ctx, client, "automation", "automation.test")
	if !ok {
		t.Fatal("expected entity to appear after reload")
	}
	if !callServiceCalled {
		t.Error("expected reload service to be called")
	}
	if entity.EntityID != "automation.test" {
		t.Errorf("expected automation.test, got %s", entity.EntityID)
	}
}

func TestServiceChangesTargetState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		domain  string
		service string
		want    bool
	}{
		{"automation", "trigger", false},
		{"homeassistant", "update_entity", false},
		{"script", "turn_on", false},
		{"script", "toggle", false},
		{"script", "morning_routine", false},
		{"script", "turn_off", true},
		{"light", "turn_off", true},
		{"automation", "turn_on", true},
		{"automation", "turn_off", true},
		{"automation", "toggle", true},
		{"light", "turn_on", true},
		{"homeassistant", "turn_off", true},
	}
	for _, tt := range tests {
		t.Run(tt.domain+"."+tt.service, func(t *testing.T) {
			t.Parallel()
			if got := serviceChangesTargetState(tt.domain, tt.service); got != tt.want {
				t.Errorf("serviceChangesTargetState(%q, %q) = %v, want %v", tt.domain, tt.service, got, tt.want)
			}
		})
	}
}
