package access

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"unicode"
	"unicode/utf8"
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
			tr.MarkConfigured(fmt.Sprintf("domain%d", i%4))
			_ = tr.ReloadAllowed("domain0")
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

// captureHandler records every slog record so tests can inspect raw attribute
// values.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (*captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

func captureLogs(t *testing.T) *captureHandler {
	t.Helper()
	h := &captureHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

var hostileValues = map[string]string{
	"backtick":       "light.a` and more `",
	"newline":        "light.a\n2026-10-26 INFO forged",
	"carriage":       "light.a\rforged",
	"nul":            "light.a\x00b",
	"long":           strings.Repeat("a", 300),
	"mixed case":     "x NOT Found y",
	"not found":      "not found",
	"bidi override":  "light.a" + string(rune(0x202e)) + "b",
	"invalid utf8":   "light.a\xff\xfeb",
	"adjacent split": "not fnot foundound",
}

func TestDenialMessage_Hostile(t *testing.T) {
	for name, v := range hostileValues {
		t.Run(name, func(t *testing.T) {
			for _, d := range []Denial{
				{Tool: v, Object: v, Capability: v},
				{Tool: v, System: SystemValue(v)},
			} {
				msg := d.Message()
				if d.Object != "" && strings.Count(msg, "`") != 4 {
					t.Errorf("object message %q: want exactly 4 backticks", msg)
				}
				if d.Object == "" && strings.Count(msg, "`") != 2 {
					t.Errorf("system message %q: want exactly 2 backticks", msg)
				}
				for _, r := range msg {
					if !unicode.IsPrint(r) {
						t.Errorf("message %q contains non-printable %U", msg, r)
					}
				}
				if strings.Contains(strings.ToLower(msg), "not found") {
					t.Errorf("message %q contains \"not found\"", msg)
				}
				// 3 fields of at most 128 runes + "..." plus fixed text.
				if n := utf8.RuneCountInString(msg); n > 3*(maxFieldRunes+3)+120 {
					t.Errorf("message length %d not bounded", n)
				}
			}
		})
	}
}

func TestDenialMessage_EmptyCapability(t *testing.T) {
	got := Denial{Tool: "t", Object: "light.a"}.Message()
	want := "denied by access policy: operation not permitted (tool: t)"
	if got != want {
		t.Fatalf("Message() = %q, want %q", got, want)
	}
}

func TestTracker_FirstKeepsRecordedValues(t *testing.T) {
	_, tr := WithTracker(context.Background(), "t")
	tr.Deny(Denial{Object: "light.a`\nb", Capability: string(CapControl)})
	if got := tr.First(); got == nil || got.Object != "light.a`\nb" {
		t.Fatalf("First() = %+v, want the denial as recorded", got)
	}
}

func TestTracker_DenyLogsSanitizedWarnForEveryDenial(t *testing.T) {
	h := captureLogs(t)
	_, tr := WithTracker(context.Background(), "call_service")
	for name, v := range hostileValues {
		tr.Deny(Denial{Tool: v, Object: v, Capability: v})
		_ = name
	}
	tr.Deny(Denial{System: SystemRestart})

	h.mu.Lock()
	defer h.mu.Unlock()
	if want := len(hostileValues) + 1; len(h.records) != want {
		t.Fatalf("got %d WARN records, want %d (first and later denials)", len(h.records), want)
	}
	for _, r := range h.records {
		if r.Level != slog.LevelWarn || r.Message != "access denied" {
			t.Errorf("record = %v %q, want WARN \"access denied\"", r.Level, r.Message)
		}
		seen := map[string]bool{}
		r.Attrs(func(a slog.Attr) bool {
			seen[a.Key] = true
			s := a.Value.String()
			for _, c := range s {
				if !unicode.IsPrint(c) {
					t.Errorf("attr %s=%q contains non-printable %U", a.Key, s, c)
				}
			}
			if strings.ContainsAny(s, "`\n\r") || strings.Contains(strings.ToLower(s), "not found") {
				t.Errorf("attr %s=%q not sanitized", a.Key, s)
			}
			if utf8.RuneCountInString(s) > maxFieldRunes+3 {
				t.Errorf("attr %s too long: %d runes", a.Key, utf8.RuneCountInString(s))
			}
			return true
		})
		for _, k := range []string{"tool", "object", "capability"} {
			if !seen[k] {
				t.Errorf("record lacks attribute %q", k)
			}
		}
	}
}

func TestTracker_ZeroValueMarkConfigured(t *testing.T) {
	tr := &Tracker{}
	tr.MarkConfigured("Automation")
	if !tr.ReloadAllowed("automation") {
		t.Error("ReloadAllowed = false after MarkConfigured on a literal Tracker")
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
