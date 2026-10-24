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
// fake server is not reading (so A's write blocks), yet marshal well inside
// A's 100ms ctx even under -race (so A actually reaches the blocked write).
// Measured on Windows/-race: 1 MiB already blocks and marshals in ~27ms,
// while 32 MiB takes ~850ms to marshal - A's ctx expired before it wrote.
const callerCancelBlobSize = 1 << 20

// TestWSClient_CallerCancelDoesNotCloseSharedConn pins that a caller's context
// expiring - while its write is blocked or while it waits for the send slot -
// never tears down the connection shared with other callers.
func TestWSClient_CallerCancelDoesNotCloseSharedConn(t *testing.T) {
	release := make(chan struct{})
	fake := newFakeHA(t, withPauseReads(release))
	cfg := newConcurrencyTestConfig()
	cfg.WriteTimeout = 5 * time.Second
	client := connectFakeClient(t, fake, cfg)

	aErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err := client.SendCommand(ctx, "test/echo", map[string]any{"blob": strings.Repeat("x", callerCancelBlobSize)})
		aErr <- err
	}()

	time.Sleep(20 * time.Millisecond)
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

	time.Sleep(180 * time.Millisecond)
	select {
	case err := <-cErr:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("C error = %v, want context.DeadlineExceeded", err)
		}
	default:
		t.Error("C has not returned by 200ms, want context.DeadlineExceeded while waiting for the send slot")
	}
	if !client.IsConnected() {
		t.Error("IsConnected() = false at 200ms, want true (a caller's ctx must not close the shared conn)")
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
	fake := newFakeHA(t, withHoldOnConn(1))
	cfg := newConcurrencyTestConfig()
	cfg.RequestTimeout = 5 * time.Second
	client := connectFakeClient(t, fake, cfg)

	const n = 20
	errs := make(chan error, n)
	for range n {
		go func() {
			_, err := client.SendCommand(context.Background(), "test/hold", nil)
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
	cfg.PingInterval = 5 * time.Millisecond
	cfg.PingTimeout = 50 * time.Millisecond
	var disconnects atomic.Int32
	cfg.OnDisconnect = func(error) { disconnects.Add(1) }
	client := connectFakeClient(t, fake, cfg)

	fake.dropAll()
	time.Sleep(500 * time.Millisecond)

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
	if elapsed > 50*time.Millisecond {
		t.Errorf("SendCommand() after Close took %v, want <= 50ms", elapsed)
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
