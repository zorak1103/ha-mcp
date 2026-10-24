package homeassistant

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestWSClient_DropAfterSend_ReplaysOnlyReads pins that after a command was
// written and its connection dropped, a read-only command is re-sent on the new
// connection while a state-changing command is not and reports an unknown
// outcome instead.
func TestWSClient_DropAfterSend_ReplaysOnlyReads(t *testing.T) {
	tests := []struct {
		name         string
		msgType      string
		wantReplayed bool
	}{
		{name: "write_call_service", msgType: "call_service", wantReplayed: false},
		{name: "read_get_states", msgType: "get_states", wantReplayed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeHA(t, withHoldOnConn(1, tt.msgType))
			cfg := newConcurrencyTestConfig()
			cfg.RequestTimeout = 5 * time.Second
			client := connectFakeClient(t, fake, cfg)

			errs := make(chan error, 1)
			go func() {
				_, err := client.SendCommand(context.Background(), tt.msgType, nil)
				errs <- err
			}()

			if !waitFor(2*time.Second, func() bool { return fake.receivedCount(tt.msgType) == 1 }) {
				t.Fatalf("fake received %d %s commands, want 1", fake.receivedCount(tt.msgType), tt.msgType)
			}
			fake.dropAll()

			var err error
			select {
			case err = <-errs:
			case <-time.After(3 * time.Second):
				t.Fatal("SendCommand did not return within 3s after the drop")
			}

			if tt.wantReplayed {
				if err != nil {
					t.Fatalf("SendCommand(%s) error = %v, want success after replay", tt.msgType, err)
				}
				if got := fake.receivedCount(tt.msgType); got != 2 {
					t.Errorf("receivedCount(%s) = %d, want 2 (replayed once)", tt.msgType, got)
				}
				if got := fake.conns.Load(); got != 2 {
					t.Errorf("fake.conns = %d, want 2", got)
				}
				return
			}

			if !errors.Is(err, errOutcomeUnknown) {
				t.Fatalf("SendCommand(%s) error = %v, want errOutcomeUnknown", tt.msgType, err)
			}
			if !errors.Is(err, errConnectionLost) {
				t.Errorf("SendCommand(%s) error = %v, want it to wrap errConnectionLost", tt.msgType, err)
			}
			if !strings.Contains(err.Error(), "may or may not have been applied") {
				t.Errorf("SendCommand(%s) error = %q, want it to mention \"may or may not have been applied\"", tt.msgType, err.Error())
			}
			if !waitFor(2*time.Second, func() bool { return fake.conns.Load() >= 2 && client.IsConnected() }) {
				t.Fatalf("no reconnect within 2s (conns=%d)", fake.conns.Load())
			}
			if got := fake.receivedCount(tt.msgType); got != 1 {
				t.Errorf("receivedCount(%s) = %d after reconnect, want 1 (never re-sent)", tt.msgType, got)
			}
		})
	}
}

// TestWSClient_DropWithMixedInFlight_ReplaysOnlyReads pins that one drop with
// reads and writes in flight replays exactly the reads, on a single new
// connection, and leaves the client usable.
func TestWSClient_DropWithMixedInFlight_ReplaysOnlyReads(t *testing.T) {
	fake := newFakeHA(t, withHoldOnConn(1, "call_service", "get_states"))
	cfg := newConcurrencyTestConfig()
	cfg.RequestTimeout = 5 * time.Second
	client := connectFakeClient(t, fake, cfg)

	const perType = 5
	type outcome struct {
		msgType string
		err     error
	}
	results := make(chan outcome, 2*perType)
	for range perType {
		for _, msgType := range []string{"call_service", "get_states"} {
			go func() {
				_, err := client.SendCommand(context.Background(), msgType, nil)
				results <- outcome{msgType: msgType, err: err}
			}()
		}
	}

	if !waitFor(2*time.Second, func() bool { return fake.received.Load() == 2*perType }) {
		t.Fatalf("fake received %d commands, want %d", fake.received.Load(), 2*perType)
	}
	fake.dropAll()

	deadline := time.After(3 * time.Second)
	for i := range 2 * perType {
		select {
		case res := <-results:
			switch res.msgType {
			case "get_states":
				if res.err != nil {
					t.Errorf("get_states error = %v, want success after replay", res.err)
				}
			default:
				if !errors.Is(res.err, errOutcomeUnknown) {
					t.Errorf("%s error = %v, want errOutcomeUnknown", res.msgType, res.err)
				}
			}
		case <-deadline:
			t.Fatalf("only %d of %d commands returned within 3s", i, 2*perType)
		}
	}

	if got := fake.receivedCount("call_service"); got != perType {
		t.Errorf("receivedCount(call_service) = %d, want %d (never re-sent)", got, perType)
	}
	if got := fake.receivedCount("get_states"); got != 2*perType {
		t.Errorf("receivedCount(get_states) = %d, want %d (each replayed once)", got, 2*perType)
	}
	if got := fake.conns.Load(); got != 2 {
		t.Errorf("fake.conns = %d, want 2 (exactly one reconnect)", got)
	}
	if _, err := client.SendCommand(context.Background(), "test/echo", nil); err != nil {
		t.Errorf("SendCommand() after reconnect = %v, want success", err)
	}
}

// TestWSClient_WriteRequestTimeout_NotReplayed pins that a write whose answer
// never arrives is not re-sent when the request times out, and is not reported
// as a lost connection.
func TestWSClient_WriteRequestTimeout_NotReplayed(t *testing.T) {
	fake := newFakeHA(t, withHoldOnConn(1, "call_service"))
	cfg := newConcurrencyTestConfig()
	cfg.RequestTimeout = 100 * time.Millisecond
	client := connectFakeClient(t, fake, cfg)

	_, err := client.SendCommand(context.Background(), "call_service", nil)
	if !errors.Is(err, ErrRequestTimeout) {
		t.Fatalf("SendCommand() error = %v, want ErrRequestTimeout", err)
	}
	if errors.Is(err, errOutcomeUnknown) {
		t.Errorf("SendCommand() error = %v, must not be errOutcomeUnknown (the connection did not drop)", err)
	}

	// Allow a (wrong) retry to reach the server before counting.
	time.Sleep(300 * time.Millisecond)
	if got := fake.receivedCount("call_service"); got != 1 {
		t.Errorf("receivedCount(call_service) = %d, want 1 (a timed-out write is not re-sent)", got)
	}
	if got := fake.conns.Load(); got != 1 {
		t.Errorf("fake.conns = %d, want 1 (no reconnect)", got)
	}
}
