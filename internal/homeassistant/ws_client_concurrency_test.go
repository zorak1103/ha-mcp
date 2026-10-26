package homeassistant

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newConcurrencyTestConfig returns the client config shared by the
// concurrency tests: health pings off, fast reconnect backoff.
func newConcurrencyTestConfig() WSClientConfig {
	cfg := DefaultWSClientConfig()
	cfg.PingInterval = 0
	cfg.ReconnectConfig = ReconnectConfig{
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      50 * time.Millisecond,
		BackoffFactor: 2,
	}
	return cfg
}

// connectFakeClient connects a client to fake and closes it at cleanup
// (registered after the fake's cleanup, so it runs first).
func connectFakeClient(t *testing.T, fake *fakeHA, cfg WSClientConfig) *WSClient {
	t.Helper()
	client := NewWSClientWithConfig(fake.URL, "test_token", cfg)
	t.Cleanup(func() { _ = client.Close() })
	// Background, not a canceled-after-return ctx: the Connect-lifetime bug has
	// its own test (TestWSClient_ConnectOutlivesContext) and must not mask the
	// other tests' failure modes. ConnectTimeout bounds the dial.
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	return client
}

// waitFor polls cond until it is true or timeout elapses.
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return cond()
}

// TestWSClient_ConcurrentSendCommand_IDsStrictlyIncreasing pins that id
// allocation and the wire write happen in one ordered step: HA rejects any id
// that does not exceed the previous one on the connection with "id_reuse".
func TestWSClient_ConcurrentSendCommand_IDsStrictlyIncreasing(t *testing.T) {
	fake := newFakeHA(t)
	client := connectFakeClient(t, fake, newConcurrencyTestConfig())

	const goroutines, perGoroutine = 200, 5
	errs := make(chan error, goroutines*perGoroutine)
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perGoroutine {
				if _, err := client.SendCommand(context.Background(), "test/echo", nil); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)

	count := 0
	for err := range errs {
		if count < 5 {
			t.Errorf("SendCommand() error: %v", err)
		}
		count++
	}
	if count > 0 {
		t.Fatalf("%d of %d commands failed, want 0", count, goroutines*perGoroutine)
	}
}

// callerCancelBlobSize must exceed what the OS socket buffers absorb while the
// fake server is not reading, so A's write blocks. 32 MiB is far above any
// common loopback buffering (Linux autotunes send+receive up to ~10 MiB
// combined; Windows blocked at 1 MiB). callerCancelADeadline must comfortably
// exceed the time to marshal that blob under -race (~850ms measured), so A
// really is blocked in the write when its deadline expires - otherwise old code
// fails A at the library's lock acquire and never tears the conn down.
const (
	callerCancelBlobSize   = 32 << 20
	callerCancelADeadline  = 3 * time.Second
	callerCancelCheckpoint = callerCancelADeadline + 500*time.Millisecond
)

// TestWSClient_CallerCancelDoesNotCloseSharedConn pins that a caller's context
// expiring - while its write is blocked or while it waits for the send slot -
// never tears down the connection shared with other callers.
func TestWSClient_CallerCancelDoesNotCloseSharedConn(t *testing.T) {
	release := make(chan struct{})
	fake := newFakeHA(t, withPauseReads(release))
	cfg := newConcurrencyTestConfig()
	// Must outlast A's blocked write until release plus the drain after it.
	cfg.WriteTimeout = 15 * time.Second
	client := connectFakeClient(t, fake, cfg)

	// Build the blob before A starts: under -race it takes long enough that B
	// or C could otherwise take the send slot first.
	blob := map[string]any{"blob": strings.Repeat("x", callerCancelBlobSize)}
	start := time.Now()
	aErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), callerCancelADeadline)
		defer cancel()
		_, err := client.SendCommand(ctx, "test/echo", blob)
		aErr <- err
	}()

	// A holds the send slot through its marshal and its blocked write. Ids are
	// allocated only under the slot, so msgID == 1 means A holds it; B and C
	// then queue behind it, and C gives up while still waiting.
	if !waitFor(2*time.Second, func() bool { return client.msgID.Load() == 1 }) {
		t.Fatal("A did not take the send slot within 2s")
	}
	bErr := make(chan error, 1)
	go func() {
		_, err := client.SendCommand(context.Background(), "test/echo", nil)
		bErr <- err
	}()
	cErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := client.SendCommand(ctx, "test/echo", nil)
		cErr <- err
	}()

	time.Sleep(callerCancelCheckpoint - time.Since(start))
	if !client.IsConnected() || fake.conns.Load() != 1 {
		t.Errorf("at checkpoint: IsConnected()=%v conns=%d, want true/1 (a caller's ctx must not close the shared conn)",
			client.IsConnected(), fake.conns.Load())
	}
	// Guard: the test only means something if A's write is still blocked
	// (server reads paused) past A's deadline.
	select {
	case err := <-aErr:
		t.Fatalf("A returned (%v) before release: its write did not block (raise callerCancelBlobSize) or its ctx tore down the write", err)
	default:
	}
	select {
	case err := <-cErr:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("C error = %v, want context.DeadlineExceeded", err)
		}
	default:
		t.Error("C has not returned by the checkpoint, want context.DeadlineExceeded while waiting for the send slot")
	}

	close(release)

	select {
	case err := <-bErr:
		if err != nil {
			t.Errorf("B error = %v, want success", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("B did not return within 10s")
	}
	// C gave up while waiting for the send slot, so it must have consumed no
	// message id and written nothing. Both counts are exact here: ids are
	// allocated only under the send slot (A=1, B=2), and B's reply means the
	// server has read A's frame and then B's frame.
	if got := client.msgID.Load(); got != 2 {
		t.Errorf("msgID = %d after A and B, want 2 (C must consume no id)", got)
	}
	if got := fake.received.Load(); got != 2 {
		t.Errorf("fake.received = %d after B's reply, want 2 (C must write nothing)", got)
	}
	select {
	case err := <-aErr:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("A error = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("A did not return within 10s")
	}
	if !client.IsConnected() {
		t.Error("IsConnected() = false after release, want true")
	}
	if got := fake.conns.Load(); got != 1 {
		t.Errorf("fake.conns = %d, want 1 (no reconnect)", got)
	}
}

// TestWSClient_ConnectOutlivesContext pins that the connection's lifetime is
// the client's, not the context passed to Connect.
func TestWSClient_ConnectOutlivesContext(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t)
	client := NewWSClientWithConfig(fake.URL, "test_token", newConcurrencyTestConfig())
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Connect(ctx); err != nil {
		cancel()
		t.Fatalf("Connect() failed: %v", err)
	}
	cancel()
	time.Sleep(50 * time.Millisecond)

	if _, err := client.SendCommand(context.Background(), "test/echo", nil); err != nil {
		t.Errorf("SendCommand() after Connect ctx canceled = %v, want success", err)
	}
	if got := fake.conns.Load(); got != 1 {
		t.Errorf("fake.conns = %d, want 1", got)
	}
}

// TestWSClient_ReconnectUnderConcurrentLoad pins that requests in flight when
// the connection drops fail fast and are retried on the single new connection.
func TestWSClient_ReconnectUnderConcurrentLoad(t *testing.T) {
	fake := newFakeHA(t, withHoldOnConn(1, "get_states"))
	cfg := newConcurrencyTestConfig()
	cfg.RequestTimeout = 5 * time.Second
	client := connectFakeClient(t, fake, cfg)

	const n = 20
	errs := make(chan error, n)
	for range n {
		go func() {
			_, err := client.SendCommand(context.Background(), "get_states", nil)
			errs <- err
		}()
	}
	if !waitFor(2*time.Second, func() bool { return fake.received.Load() == n }) {
		t.Fatalf("fake received %d commands, want %d", fake.received.Load(), n)
	}
	fake.dropAll()

	deadline := time.After(3 * time.Second)
	for i := range n {
		select {
		case err := <-errs:
			if err != nil {
				t.Errorf("held command %d error = %v, want success after reconnect", i, err)
			}
		case <-deadline:
			t.Fatalf("only %d of %d held commands returned within 3s", i, n)
		}
	}

	for i := range n {
		if _, err := client.SendCommand(context.Background(), "test/echo", nil); err != nil {
			t.Errorf("command %d after reconnect error = %v", i, err)
		}
	}
	if got := fake.conns.Load(); got != 2 {
		t.Errorf("fake.conns = %d, want 2 (exactly one reconnect)", got)
	}
}

// TestWSClient_HealthAndReadPathsReconnectOnce pins that the health loop and
// the read loop observing the same drop produce one reconnect and one
// OnDisconnect.
func TestWSClient_HealthAndReadPathsReconnectOnce(t *testing.T) {
	fake := newFakeHA(t)
	cfg := newConcurrencyTestConfig()
	// Fast pings so the health path races the read path for the same drop;
	// a generous PingTimeout so a scheduler stall on a loaded -race runner
	// cannot fake a pong timeout (a second "drop").
	cfg.PingInterval = 5 * time.Millisecond
	cfg.PingTimeout = 500 * time.Millisecond
	var disconnects atomic.Int32
	cfg.OnDisconnect = func(error) { disconnects.Add(1) }
	client := connectFakeClient(t, fake, cfg)

	fake.dropAll()
	if !waitFor(2*time.Second, func() bool { return fake.conns.Load() >= 2 && client.IsConnected() }) {
		t.Fatalf("no reconnect within 2s (conns=%d)", fake.conns.Load())
	}
	// Give a late second notification path time to (wrongly) fire.
	time.Sleep(200 * time.Millisecond)

	if got := fake.conns.Load(); got != 2 {
		t.Errorf("fake.conns = %d, want 2", got)
	}
	if _, err := client.SendCommand(context.Background(), "test/echo", nil); err != nil {
		t.Errorf("SendCommand() after reconnect = %v, want success", err)
	}
	if got := disconnects.Load(); got != 1 {
		t.Errorf("OnDisconnect called %d times, want 1", got)
	}
}

// TestWSClient_DropRightAfterReconnect_ReconnectsAgain pins that a session
// that dies right after the reconnect dial does not leave the client
// disconnected with no reconnect running.
func TestWSClient_DropRightAfterReconnect_ReconnectsAgain(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t, withDropAfterAuthOnConn(2))
	client := connectFakeClient(t, fake, newConcurrencyTestConfig())

	fake.dropAll()

	ok := waitFor(2*time.Second, func() bool {
		if fake.conns.Load() < 3 || !client.IsConnected() {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, err := client.SendCommand(ctx, "test/echo", nil)
		return err == nil
	})
	if !ok {
		t.Fatalf("no successful command within 2s (conns=%d, connected=%v, reconnecting=%v)",
			fake.conns.Load(), client.IsConnected(), client.reconnecting.Load())
	}
	if got := fake.conns.Load(); got != 3 {
		t.Errorf("fake.conns = %d, want 3", got)
	}
}

// TestWSClient_GiveUpAfterMaxAttempts_NotUsable pins that once reconnection
// gives up the client reports itself unusable.
func TestWSClient_GiveUpAfterMaxAttempts_NotUsable(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t, withAcceptLimit(1))
	cfg := newConcurrencyTestConfig()
	cfg.ReconnectConfig.MaxAttempts = 2
	client := connectFakeClient(t, fake, cfg)

	fake.dropAll()

	if !waitFor(2*time.Second, func() bool { return !client.IsUsable() && !client.IsConnected() }) {
		t.Fatalf("after giving up: IsUsable()=%v IsConnected()=%v, want both false",
			client.IsUsable(), client.IsConnected())
	}
}

// TestWSClient_AuthInvalidOnReconnect_GivesUp pins that an auth_invalid on a
// reconnect attempt is terminal: the client stops dialing and reports itself
// unusable, so a pool evicts it and the caller sees the auth error from a
// fresh Connect instead of waiting on an endless reconnect.
func TestWSClient_AuthInvalidOnReconnect_GivesUp(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t, withAuthInvalidOnAttempt(2))
	client := connectFakeClient(t, fake, newConcurrencyTestConfig())

	fake.dropAll()

	if !waitFor(2*time.Second, func() bool {
		return fake.authAttempts.Load() >= 2 && !client.IsUsable() && !client.IsConnected()
	}) {
		t.Fatalf("after auth_invalid: authAttempts=%d IsUsable()=%v IsConnected()=%v, want >=2/false/false",
			fake.authAttempts.Load(), client.IsUsable(), client.IsConnected())
	}

	// With a 10ms backoff, a client that kept dialing would add attempts here.
	time.Sleep(300 * time.Millisecond)
	if got := fake.authAttempts.Load(); got != 2 {
		t.Errorf("authAttempts = %d, want 2 (no dialing after auth_invalid)", got)
	}
	if client.IsUsable() {
		t.Error("IsUsable() = true after auth_invalid, want false")
	}
}

// newSlowReconnectConfig returns the concurrency test config with a fixed
// reconnect backoff of delay, leaving a window in which a caller can start
// waiting before the reconnect attempt is made.
func newSlowReconnectConfig(delay time.Duration) WSClientConfig {
	cfg := newConcurrencyTestConfig()
	cfg.ReconnectConfig.InitialDelay = delay
	cfg.ReconnectConfig.MaxDelay = delay
	return cfg
}

// waitForConnectionAsync runs WaitForConnection with a ctx far below its own
// 10s limit, so a regression fails fast with a context error instead of
// stalling the suite.
func waitForConnectionAsync(client *WSClient) <-chan error {
	res := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		res <- client.WaitForConnection(ctx)
	}()
	return res
}

// TestWSClient_WaitForConnection_ReleasedByAuthInvalid pins that a caller
// already inside WaitForConnection when the reconnect loop stops on
// auth_invalid gets the auth error right away, not ErrMaxReconnectAttempts
// after the full 10s limit.
func TestWSClient_WaitForConnection_ReleasedByAuthInvalid(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t, withAuthInvalidOnAttempt(2))
	client := connectFakeClient(t, fake, newSlowReconnectConfig(300*time.Millisecond))

	fake.dropAll()
	if !waitFor(2*time.Second, func() bool { return client.reconnecting.Load() }) {
		t.Fatal("client did not start reconnecting within 2s")
	}

	start := time.Now()
	err := <-waitForConnectionAsync(client)

	if !errors.Is(err, errAuthFailed) {
		t.Fatalf("WaitForConnection() = %v, want an error wrapping errAuthFailed", err)
	}
	if errors.Is(err, ErrMaxReconnectAttempts) {
		t.Errorf("WaitForConnection() = %v, must not report exhausted reconnect attempts", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("WaitForConnection() took %v, want well under its 10s limit", elapsed)
	}
}

// TestWSClient_WaitForConnection_AfterAuthInvalid pins that a caller arriving
// after the reconnect loop stopped on auth_invalid gets the auth error
// immediately instead of polling toward the 10s limit.
func TestWSClient_WaitForConnection_AfterAuthInvalid(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t, withAuthInvalidOnAttempt(2))
	client := connectFakeClient(t, fake, newConcurrencyTestConfig())

	fake.dropAll()
	if !waitFor(2*time.Second, func() bool { return fake.authAttempts.Load() >= 2 && !client.IsUsable() }) {
		t.Fatalf("client still usable after auth_invalid (authAttempts=%d)", fake.authAttempts.Load())
	}

	start := time.Now()
	err := <-waitForConnectionAsync(client)

	if !errors.Is(err, errAuthFailed) {
		t.Fatalf("WaitForConnection() = %v, want an error wrapping errAuthFailed", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("WaitForConnection() took %v, want an immediate return", elapsed)
	}
}

// TestWSClient_WaitForConnection_ClosedAfterAuthInvalid pins that Close wins
// over a recorded auth failure: WaitForConnection documents errClientClosed
// after Close, and the stored auth error must not mask it.
func TestWSClient_WaitForConnection_ClosedAfterAuthInvalid(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t, withAuthInvalidOnAttempt(2))
	client := connectFakeClient(t, fake, newConcurrencyTestConfig())

	fake.dropAll()
	if !waitFor(2*time.Second, func() bool { return fake.authAttempts.Load() >= 2 && !client.IsUsable() }) {
		t.Fatalf("client still usable after auth_invalid (authAttempts=%d)", fake.authAttempts.Load())
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	if err := <-waitForConnectionAsync(client); !errors.Is(err, errClientClosed) {
		t.Fatalf("WaitForConnection() after Close = %v, want errClientClosed", err)
	}
}

// TestWSClient_WaitForConnection_TransientReconnectStillWaits pins that a
// caller waiting out an ordinary reconnect is unaffected by the auth_invalid
// release and returns nil once the new session is up.
func TestWSClient_WaitForConnection_TransientReconnectStillWaits(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t)
	client := connectFakeClient(t, fake, newSlowReconnectConfig(150*time.Millisecond))

	fake.dropAll()
	if !waitFor(2*time.Second, func() bool { return client.reconnecting.Load() }) {
		t.Fatal("client did not start reconnecting within 2s")
	}

	if err := <-waitForConnectionAsync(client); err != nil {
		t.Fatalf("WaitForConnection() = %v, want nil after a transient reconnect", err)
	}
	if !client.IsConnected() {
		t.Error("IsConnected() = false after WaitForConnection returned nil")
	}
}

// TestWSClient_WaitForConnection_AuthFailureClearedByConnect pins that the
// recorded auth failure is dropped once a later Connect succeeds, so a drop of
// that new session waits for its reconnect instead of returning the old error.
func TestWSClient_WaitForConnection_AuthFailureClearedByConnect(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t, withAuthInvalidOnAttempt(2))
	client := connectFakeClient(t, fake, newSlowReconnectConfig(150*time.Millisecond))

	fake.dropAll()
	if !waitFor(2*time.Second, func() bool { return fake.authAttempts.Load() >= 2 && !client.IsUsable() }) {
		t.Fatalf("client still usable after auth_invalid (authAttempts=%d)", fake.authAttempts.Load())
	}
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() after auth_invalid failed: %v", err)
	}

	fake.dropAll()
	if !waitFor(2*time.Second, func() bool { return client.reconnecting.Load() }) {
		t.Fatal("client did not start reconnecting after the second drop")
	}
	if err := <-waitForConnectionAsync(client); err != nil {
		t.Fatalf("WaitForConnection() = %v, want nil (stale auth failure must be cleared)", err)
	}
}

// TestWSClient_StaleHealthCheck_DoesNotTouchNewSession pins that a health
// check for an ended session acts on that session only: it must neither ping
// the new connection with the old session's canceled ctx (coder/websocket
// closes a conn whose write ctx is done) nor end the new session.
func TestWSClient_StaleHealthCheck_DoesNotTouchNewSession(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t)
	client := connectFakeClient(t, fake, newConcurrencyTestConfig())
	old := client.session.Load()

	fake.dropAll()
	if !waitFor(2*time.Second, func() bool { return fake.conns.Load() == 2 && client.IsConnected() }) {
		t.Fatalf("no reconnect within 2s (conns=%d)", fake.conns.Load())
	}
	current := client.session.Load()

	// Several stale ticks: one canceled-ctx ping may lose the library's lock
	// race harmlessly, so repeat to make a regression near-certain to show.
	for range 10 {
		if !client.checkSession(old.ctx, old) {
			t.Fatal("checkSession(old) = false, want true (the old session's loop must exit)")
		}
	}
	time.Sleep(50 * time.Millisecond)

	if client.session.Load() != current || !client.IsConnected() {
		t.Fatal("a stale health check disturbed the new session")
	}
	if _, err := client.SendCommand(context.Background(), "test/echo", nil); err != nil {
		t.Errorf("SendCommand() on the new session = %v, want success", err)
	}
	if got := fake.conns.Load(); got != 2 {
		t.Errorf("fake.conns = %d, want 2 (no extra reconnect)", got)
	}
}

// TestWSClient_HandleConnectionFailure_EndsCurrentSession pins the thin
// wrapper: it ends whichever session is current.
func TestWSClient_HandleConnectionFailure_EndsCurrentSession(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t)
	cfg := newConcurrencyTestConfig()
	cfg.AutoReconnect = false
	client := connectFakeClient(t, fake, cfg)
	s := client.session.Load()

	client.handleConnectionFailure(errors.New("test failure"))

	if client.session.Load() != nil || client.IsConnected() {
		t.Error("session still current after handleConnectionFailure, want it ended")
	}
	if s.ctx.Err() == nil {
		t.Error("ended session's ctx not canceled")
	}
}

// TestWSClient_SendAfterClose_FailsFast pins that a closed client rejects
// commands immediately with errClientClosed instead of waiting or retrying.
func TestWSClient_SendAfterClose_FailsFast(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t)
	client := connectFakeClient(t, fake, newConcurrencyTestConfig())

	if err := client.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	start := time.Now()
	_, err := client.SendCommand(context.Background(), "test/echo", nil)
	elapsed := time.Since(start)
	if !errors.Is(err, errClientClosed) {
		t.Errorf("SendCommand() after Close = %v, want errClientClosed", err)
	}
	// The real bound is "no retry backoff" (>= 100ms default); 250ms leaves
	// headroom for a loaded -race runner.
	if elapsed > 250*time.Millisecond {
		t.Errorf("SendCommand() after Close took %v, want <= 250ms (no retry)", elapsed)
	}
	if client.IsUsable() {
		t.Error("IsUsable() = true after Close, want false")
	}
	if err := client.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil", err)
	}
}

// TestWSClient_CloseInsideOnDisconnect_NoDeadlock pins that OnDisconnect is
// invoked without client locks held, so it may call Close.
func TestWSClient_CloseInsideOnDisconnect_NoDeadlock(t *testing.T) {
	t.Parallel()

	fake := newFakeHA(t)
	cfg := newConcurrencyTestConfig()
	cfg.AutoReconnect = false
	client := NewWSClientWithConfig(fake.URL, "test_token", cfg)
	t.Cleanup(func() { _ = client.Close() })

	done := make(chan struct{})
	client.SetOnDisconnect(func(error) {
		_ = client.Close()
		close(done)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	fake.dropAll()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() inside OnDisconnect did not finish within 2s (deadlock?)")
	}
}

// TestWSClient_IsUsable pins IsUsable = connected || reconnecting.
func TestWSClient_IsUsable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		connected, reconnecting, want bool
	}{
		{false, false, false},
		{true, false, true},
		{false, true, true},
		{true, true, true},
	}
	for _, tt := range tests {
		client := NewWSClient("http://example.com", "token")
		client.connected.Store(tt.connected)
		client.reconnecting.Store(tt.reconnecting)
		if got := client.IsUsable(); got != tt.want {
			t.Errorf("connected=%v reconnecting=%v: IsUsable() = %v, want %v",
				tt.connected, tt.reconnecting, got, tt.want)
		}
	}
}
