package access

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestDenialMessage_Exact(t *testing.T) {
	tests := []struct {
		name string
		d    Denial
		want string
	}{
		{
			name: "object",
			d:    Denial{Tool: "manage_automation", Object: "automation.heating", Capability: string(CapConfigure)},
			want: "denied by access policy: `automation.heating` is read-only, `configure` not granted (tool: manage_automation)",
		},
		{
			name: "system",
			d:    Denial{Tool: "call_service", System: SystemRestart},
			want: "denied by access policy: system operation `restart` is not granted (tool: call_service)",
		},
		{
			name: "unclassified",
			d:    Denial{Tool: "call_service"},
			want: "denied by access policy: operation not permitted (tool: call_service)",
		},
		{
			name: "no tool",
			d:    Denial{Object: "light.kitchen", Capability: string(CapControl)},
			want: "denied by access policy: `light.kitchen` is read-only, `control` not granted",
		},
		{
			name: "no tool unclassified",
			d:    Denial{},
			want: "denied by access policy: operation not permitted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.d.Message()
			if got != tt.want {
				t.Fatalf("Message() = %q, want %q", got, tt.want)
			}
			lower := strings.ToLower(got)
			for _, bad := range []string{"not found", "configuration", "config file", "yaml", ".yml"} {
				if strings.Contains(lower, bad) {
					t.Errorf("message %q must not contain %q", got, bad)
				}
			}
		})
	}
}

func TestTracker_FirstWins(t *testing.T) {
	_, tr := WithTracker(context.Background(), "manage_automation")
	if tr.First() != nil {
		t.Fatal("First() = non-nil before any Deny")
	}
	tr.Deny(Denial{Object: "automation.a", Capability: string(CapConfigure)})
	tr.Deny(Denial{Object: "automation.b", Capability: string(CapConfigure)})
	got := tr.First()
	if got == nil || got.Object != "automation.a" {
		t.Fatalf("First() = %+v, want automation.a", got)
	}
	if got.Tool != "manage_automation" {
		t.Errorf("Tool = %q, want the tracker's tool when the denial has none", got.Tool)
	}
	if tr.Tool() != "manage_automation" {
		t.Errorf("Tracker.Tool() = %q", tr.Tool())
	}
}

func TestTracker_FirstWins_Concurrent(t *testing.T) {
	_, tr := WithTracker(context.Background(), "call_service")
	const n = 64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			tr.Deny(Denial{Object: fmt.Sprintf("light.l%d", i), Capability: string(CapControl)})
			_ = tr.First()
		}()
	}
	close(start)
	wg.Wait()

	first := tr.First()
	if first == nil {
		t.Fatal("First() = nil after concurrent denials")
	}
	tr.Deny(Denial{Object: "light.late"})
	if again := tr.First(); again == nil || again.Object != first.Object {
		t.Fatalf("First() changed from %q to %+v", first.Object, again)
	}
}

func TestReloadAllowed_OnlyAfterConfigure(t *testing.T) {
	_, tr := WithTracker(context.Background(), "manage_automation")
	if tr.ReloadAllowed("automation") {
		t.Fatal("ReloadAllowed before MarkConfigured = true")
	}
	tr.MarkConfigured("automation")
	if !tr.ReloadAllowed("automation") {
		t.Error("ReloadAllowed(automation) = false after MarkConfigured")
	}
	if !tr.ReloadAllowed("AUTOMATION") {
		t.Error("ReloadAllowed is not case-insensitive")
	}
	if tr.ReloadAllowed("script") {
		t.Error("ReloadAllowed(script) = true without MarkConfigured(script)")
	}
}

func TestTrackerFrom_NilSafe(t *testing.T) {
	if got := TrackerFrom(context.Background()); got != nil {
		t.Fatalf("TrackerFrom(empty ctx) = %v, want nil", got)
	}
	ctx, tr := WithTracker(context.Background(), "x")
	if TrackerFrom(ctx) != tr {
		t.Error("TrackerFrom did not return the tracker stored by WithTracker")
	}

	var nilTracker *Tracker
	nilTracker.Deny(Denial{Object: "light.a"})
	nilTracker.MarkConfigured("automation")
	if nilTracker.First() != nil {
		t.Error("nil tracker First() != nil")
	}
	if nilTracker.ReloadAllowed("automation") {
		t.Error("nil tracker ReloadAllowed = true")
	}
	if nilTracker.Tool() != "" {
		t.Error("nil tracker Tool() != empty")
	}
}
