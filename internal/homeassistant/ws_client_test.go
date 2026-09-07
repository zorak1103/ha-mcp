package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// testError is defined in factory_test.go

func TestNewWSClient(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://homeassistant.local:8123", "test_token")

	if client == nil {
		t.Fatal("NewWSClient returned nil")
	}

	if client.baseURL != "http://homeassistant.local:8123" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "http://homeassistant.local:8123")
	}
	if client.token != "test_token" {
		t.Errorf("token = %q, want %q", client.token, "test_token")
	}
	if client.pending == nil {
		t.Error("pending map is nil")
	}
	if client.reconnectMgr == nil {
		t.Error("reconnectMgr is nil")
	}
	if !client.config.AutoReconnect {
		t.Error("AutoReconnect should be true by default")
	}
}

func TestNewWSClientWithConfig(t *testing.T) {
	t.Parallel()

	config := WSClientConfig{
		AutoReconnect: false,
		PingInterval:  60 * time.Second,
		PingTimeout:   20 * time.Second,
		WriteTimeout:  30 * time.Second,
		ReconnectConfig: ReconnectConfig{
			InitialDelay:  2 * time.Second,
			MaxDelay:      120 * time.Second,
			BackoffFactor: 3.0,
			MaxAttempts:   10,
		},
	}

	client := NewWSClientWithConfig("http://example.com", "token123", config)

	if client == nil {
		t.Fatal("NewWSClientWithConfig returned nil")
	}

	if client.baseURL != "http://example.com" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "http://example.com")
	}
	if client.token != "token123" {
		t.Errorf("token = %q, want %q", client.token, "token123")
	}
	if client.config.AutoReconnect != false {
		t.Errorf("AutoReconnect = %v, want false", client.config.AutoReconnect)
	}
	if client.config.PingInterval != 60*time.Second {
		t.Errorf("PingInterval = %v, want %v", client.config.PingInterval, 60*time.Second)
	}
	if client.config.PingTimeout != 20*time.Second {
		t.Errorf("PingTimeout = %v, want %v", client.config.PingTimeout, 20*time.Second)
	}
}

func TestWSClient_BuildWSURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		want    string
		wantErr bool
	}{
		{
			name:    "http to ws",
			baseURL: "http://homeassistant.local:8123",
			want:    "ws://homeassistant.local:8123/api/websocket",
			wantErr: false,
		},
		{
			name:    "https to wss",
			baseURL: "https://homeassistant.local:8123",
			want:    "wss://homeassistant.local:8123/api/websocket",
			wantErr: false,
		},
		{
			name:    "ws stays ws",
			baseURL: "ws://homeassistant.local:8123",
			want:    "ws://homeassistant.local:8123/api/websocket",
			wantErr: false,
		},
		{
			name:    "wss stays wss",
			baseURL: "wss://homeassistant.local:8123",
			want:    "wss://homeassistant.local:8123/api/websocket",
			wantErr: false,
		},
		{
			name:    "with path (ignored)",
			baseURL: "http://homeassistant.local:8123/some/path",
			want:    "ws://homeassistant.local:8123/api/websocket",
			wantErr: false,
		},
		{
			name:    "unsupported scheme",
			baseURL: "ftp://homeassistant.local:8123",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			baseURL: "://invalid",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewWSClient(tt.baseURL, "token")
			got, err := client.buildWSURL()

			if (err != nil) != tt.wantErr {
				t.Errorf("buildWSURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("buildWSURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWSClient_IsConnected(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Initially not connected
	if client.IsConnected() {
		t.Error("IsConnected() = true, want false (before connection)")
	}

	// Manually set connected state
	client.connected.Store(true)

	if !client.IsConnected() {
		t.Error("IsConnected() = false, want true")
	}

	// Manually clear connected state
	client.connected.Store(false)

	if client.IsConnected() {
		t.Error("IsConnected() = true after disconnect, want false")
	}
}

func TestWSClient_IsHealthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		connected    bool
		pingInterval time.Duration
		lastPong     time.Time
		want         bool
	}{
		{
			name:         "not connected",
			connected:    false,
			pingInterval: 30 * time.Second,
			lastPong:     time.Now(),
			want:         false,
		},
		{
			name:         "connected, ping disabled",
			connected:    true,
			pingInterval: 0,
			lastPong:     time.Time{},
			want:         true,
		},
		{
			name:         "connected, recent pong",
			connected:    true,
			pingInterval: 30 * time.Second,
			lastPong:     time.Now(),
			want:         true,
		},
		{
			name:         "connected, old pong",
			connected:    true,
			pingInterval: 30 * time.Second,
			lastPong:     time.Now().Add(-60 * time.Second), // 60s ago, threshold is 40s (30+10)
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := DefaultWSClientConfig()
			config.PingInterval = tt.pingInterval
			config.PingTimeout = 10 * time.Second

			client := NewWSClientWithConfig("http://example.com", "token", config)
			client.connected.Store(tt.connected)
			if !tt.lastPong.IsZero() {
				client.lastPong.Store(tt.lastPong)
			}

			got := client.IsHealthy()
			if got != tt.want {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWSClient_GetLastPongTime(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Initially should return zero time
	pongTime := client.GetLastPongTime()
	if !pongTime.IsZero() {
		t.Errorf("GetLastPongTime() = %v, want zero time", pongTime)
	}

	// Set a pong time
	now := time.Now()
	client.lastPong.Store(now)

	pongTime = client.GetLastPongTime()
	if !pongTime.Equal(now) {
		t.Errorf("GetLastPongTime() = %v, want %v", pongTime, now)
	}
}

func TestWSClient_SetOnReconnect(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	var called bool
	var receivedAttempts int

	client.SetOnReconnect(func(attempts int) {
		called = true
		receivedAttempts = attempts
	})

	// Verify callback is set
	if client.config.OnReconnect == nil {
		t.Fatal("OnReconnect callback is nil")
	}

	// Call the callback
	client.config.OnReconnect(5)

	if !called {
		t.Error("OnReconnect callback was not called")
	}
	if receivedAttempts != 5 {
		t.Errorf("receivedAttempts = %d, want 5", receivedAttempts)
	}
}

func TestWSClient_SetOnDisconnect(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	var called bool
	var receivedErr error

	client.SetOnDisconnect(func(err error) {
		called = true
		receivedErr = err
	})

	// Verify callback is set
	if client.config.OnDisconnect == nil {
		t.Fatal("OnDisconnect callback is nil")
	}

	// Call the callback
	testErr := &testError{msg: "disconnect error"}
	client.config.OnDisconnect(testErr)

	if !called {
		t.Error("OnDisconnect callback was not called")
	}
	if !errors.Is(receivedErr, testErr) {
		t.Errorf("receivedErr = %v, want %v", receivedErr, testErr)
	}
}

func TestWSClient_SetAutoReconnect(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Default should be true
	if !client.config.AutoReconnect {
		t.Error("AutoReconnect default should be true")
	}

	// Disable
	client.SetAutoReconnect(false)
	if client.config.AutoReconnect {
		t.Error("AutoReconnect should be false after SetAutoReconnect(false)")
	}

	// Enable
	client.SetAutoReconnect(true)
	if !client.config.AutoReconnect {
		t.Error("AutoReconnect should be true after SetAutoReconnect(true)")
	}
}

func TestWSClient_SetPingInterval(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Set to new value
	client.SetPingInterval(45 * time.Second)
	if client.config.PingInterval != 45*time.Second {
		t.Errorf("PingInterval = %v, want %v", client.config.PingInterval, 45*time.Second)
	}

	// Disable by setting to 0
	client.SetPingInterval(0)
	if client.config.PingInterval != 0 {
		t.Errorf("PingInterval = %v, want 0", client.config.PingInterval)
	}
}

func TestWSClient_Close_NotConnected(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Close without connecting should not panic
	err := client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestMaxWSMessageSize(t *testing.T) {
	t.Parallel()

	// Verify the constant is set to 16MB
	expected := 16 * 1024 * 1024
	if maxWSMessageSize != expected {
		t.Errorf("maxWSMessageSize = %d, want %d", maxWSMessageSize, expected)
	}
}

func TestWSClient_InitialState(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Verify initial state
	if client.connected.Load() {
		t.Error("connected should be false initially")
	}
	if client.reconnecting.Load() {
		t.Error("reconnecting should be false initially")
	}
	if client.msgID.Load() != 0 {
		t.Errorf("msgID should be 0 initially, got %d", client.msgID.Load())
	}
	if client.conn != nil {
		t.Error("conn should be nil initially")
	}
	if client.ctx != nil {
		t.Error("ctx should be nil initially")
	}
	if client.cancel != nil {
		t.Error("cancel should be nil initially")
	}
}

func TestWSClient_PendingMapInitialized(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Verify pending map is initialized and empty
	if client.pending == nil {
		t.Fatal("pending map should not be nil")
	}
	if len(client.pending) != 0 {
		t.Errorf("pending map should be empty, has %d entries", len(client.pending))
	}

	// Verify we can add to the map without panic
	client.pendingMu.Lock()
	client.pending[1] = make(chan *WSResultMessage, 1)
	client.pendingMu.Unlock()

	client.pendingMu.RLock()
	if len(client.pending) != 1 {
		t.Errorf("pending map should have 1 entry, has %d", len(client.pending))
	}
	client.pendingMu.RUnlock()
}

func TestWSClient_MessageIDIncrement(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Verify message ID increments
	id1 := client.msgID.Add(1)
	if id1 != 1 {
		t.Errorf("first msgID = %d, want 1", id1)
	}

	id2 := client.msgID.Add(1)
	if id2 != 2 {
		t.Errorf("second msgID = %d, want 2", id2)
	}

	id3 := client.msgID.Add(1)
	if id3 != 3 {
		t.Errorf("third msgID = %d, want 3", id3)
	}
}

func TestWSClient_ConcurrentMsgIDIncrement(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Run concurrent increments
	done := make(chan int64, 100)
	for i := 0; i < 100; i++ {
		go func() {
			done <- client.msgID.Add(1)
		}()
	}

	// Collect all IDs
	ids := make(map[int64]bool)
	for i := 0; i < 100; i++ {
		id := <-done
		if ids[id] {
			t.Errorf("duplicate ID: %d", id)
		}
		ids[id] = true
	}

	// Verify final count
	if client.msgID.Load() != 100 {
		t.Errorf("final msgID = %d, want 100", client.msgID.Load())
	}
}

func TestWSClient_RequestTimeoutConfig(t *testing.T) {
	t.Parallel()

	// Default config should have request timeout
	config := DefaultWSClientConfig()
	if config.RequestTimeout != defaultRequestTimeout {
		t.Errorf("Default RequestTimeout = %v, want %v", config.RequestTimeout, defaultRequestTimeout)
	}

	// Custom config
	customConfig := WSClientConfig{
		RequestTimeout: 60 * time.Second,
	}
	client := NewWSClientWithConfig("http://example.com", "token", customConfig)
	if client.config.RequestTimeout != 60*time.Second {
		t.Errorf("Custom RequestTimeout = %v, want 60s", client.config.RequestTimeout)
	}
}

func TestDefaultRequestTimeout(t *testing.T) {
	t.Parallel()

	// Verify the constant is set to 30 seconds
	expected := 30 * time.Second
	if defaultRequestTimeout != expected {
		t.Errorf("defaultRequestTimeout = %v, want %v", defaultRequestTimeout, expected)
	}
}

func TestErrRequestTimeout(t *testing.T) {
	t.Parallel()

	// Verify the error is defined and has expected message
	if ErrRequestTimeout == nil {
		t.Fatal("ErrRequestTimeout is nil")
	}

	expected := "request timeout: no response received"
	if ErrRequestTimeout.Error() != expected {
		t.Errorf("ErrRequestTimeout.Error() = %q, want %q", ErrRequestTimeout.Error(), expected)
	}
}

func TestWSClient_PendingMapCleanup(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Verify pending map starts empty
	client.pendingMu.RLock()
	initialLen := len(client.pending)
	client.pendingMu.RUnlock()

	if initialLen != 0 {
		t.Errorf("pending map should start empty, has %d entries", initialLen)
	}

	// Add an entry and verify it's there
	client.pendingMu.Lock()
	client.pending[1] = make(chan *WSResultMessage, 1)
	client.pendingMu.Unlock()

	client.pendingMu.RLock()
	if len(client.pending) != 1 {
		t.Errorf("pending map should have 1 entry, has %d", len(client.pending))
	}
	client.pendingMu.RUnlock()

	// Simulate cleanup
	client.pendingMu.Lock()
	delete(client.pending, 1)
	client.pendingMu.Unlock()

	client.pendingMu.RLock()
	if len(client.pending) != 0 {
		t.Errorf("pending map should be empty after cleanup, has %d entries", len(client.pending))
	}
	client.pendingMu.RUnlock()
}

func TestWSClient_ClosePendingChannels(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Add several pending entries
	channels := make([]chan *WSResultMessage, 5)
	for i := 0; i < 5; i++ {
		ch := make(chan *WSResultMessage, 1)
		channels[i] = ch
		client.pendingMu.Lock()
		client.pending[int64(i+1)] = ch
		client.pendingMu.Unlock()
	}

	// Verify all entries are present
	client.pendingMu.RLock()
	if len(client.pending) != 5 {
		t.Errorf("pending map should have 5 entries, has %d", len(client.pending))
	}
	client.pendingMu.RUnlock()

	// Call closePendingChannels
	client.closePendingChannels()

	// Verify map is empty
	client.pendingMu.RLock()
	if len(client.pending) != 0 {
		t.Errorf("pending map should be empty, has %d entries", len(client.pending))
	}
	client.pendingMu.RUnlock()

	// Verify channels are closed
	for i, ch := range channels {
		select {
		case _, ok := <-ch:
			if ok {
				t.Errorf("channel %d should be closed", i)
			}
		default:
			t.Errorf("channel %d should be closed and readable", i)
		}
	}
}

func TestWSClient_StopHealthMonitor_Safe(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Calling stopHealthMonitor on client without active monitor should not panic
	client.stopHealthMonitor()
	client.stopHealthMonitor() // Multiple calls should be safe
}

func TestWSClient_StartHealthMonitor_StopsPrevious(t *testing.T) {
	t.Parallel()

	config := WSClientConfig{
		PingInterval: 50 * time.Millisecond,
		PingTimeout:  10 * time.Millisecond,
	}
	client := NewWSClientWithConfig("http://example.com", "token", config)

	// Create a mock connection context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.ctx = ctx

	// Track exits to verify goroutines are stopped
	exitCh := make(chan struct{}, 10)

	// Manually start and stop health monitors multiple times
	for i := 0; i < 3; i++ {
		// Stop any existing one
		client.stopHealthMonitor()

		// Create a new cancellable context for this "health monitor"
		monitorCtx, monitorCancel := context.WithCancel(ctx)
		client.pingCancel = monitorCancel
		client.pingWg.Add(1)
		go func() {
			defer client.pingWg.Done()
			<-monitorCtx.Done()
			exitCh <- struct{}{}
		}()
	}

	// Final stop should clean up the last one
	client.stopHealthMonitor()

	// Verify all 3 goroutines exited
	for i := 0; i < 3; i++ {
		select {
		case <-exitCh:
			// OK
		case <-time.After(500 * time.Millisecond):
			t.Errorf("Goroutine %d did not exit in time", i)
		}
	}
}

func TestWSClient_HealthMonitorCleanupOnClose(t *testing.T) {
	t.Parallel()

	config := WSClientConfig{
		PingInterval: 100 * time.Millisecond,
		PingTimeout:  50 * time.Millisecond,
	}
	client := NewWSClientWithConfig("http://example.com", "token", config)

	// Create a context for the "connection"
	ctx, cancel := context.WithCancel(context.Background())
	client.ctx = ctx
	client.cancel = cancel

	// Simulate starting health monitor
	client.pingWg.Add(1)
	pingCtx, pingCancel := context.WithCancel(ctx)
	client.pingCancel = pingCancel
	healthDone := make(chan struct{})
	go func() {
		defer client.pingWg.Done()
		defer close(healthDone)
		<-pingCtx.Done()
	}()

	// Close should stop health monitor and wait for it
	done := make(chan error, 1)
	go func() {
		done <- client.Close()
	}()

	// Should complete without hanging
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Close() did not complete in time - possible goroutine leak")
	}

	// Health loop should have exited
	select {
	case <-healthDone:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Health loop did not exit after Close()")
	}
}

func TestPendingRequest_Cleanup(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Add a pending request
	responseChan := make(chan *WSResultMessage, 1)
	client.pendingMu.Lock()
	client.pending[42] = responseChan
	client.pendingMu.Unlock()

	// Create pendingRequest
	req := &pendingRequest{
		id:           42,
		responseChan: responseChan,
		timer:        time.NewTimer(1 * time.Hour), // Long timer to avoid firing
		cleaned:      false,
	}

	// Verify entry exists
	client.pendingMu.RLock()
	if _, ok := client.pending[42]; !ok {
		t.Fatal("pending entry should exist before cleanup")
	}
	client.pendingMu.RUnlock()

	// Run cleanup
	req.cleanup(client)

	// Verify entry was removed
	client.pendingMu.RLock()
	if _, ok := client.pending[42]; ok {
		t.Error("pending entry should be removed after cleanup")
	}
	client.pendingMu.RUnlock()

	// Verify cleaned flag is set
	if !req.cleaned {
		t.Error("cleaned flag should be true after cleanup")
	}

	// Second cleanup should be idempotent
	req.cleanup(client)
	if !req.cleaned {
		t.Error("cleaned flag should remain true after second cleanup")
	}
}

func TestPendingRequest_CleanupIdempotent(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	// Create pendingRequest already marked as cleaned
	req := &pendingRequest{
		id:           99,
		responseChan: make(chan *WSResultMessage, 1),
		timer:        time.NewTimer(1 * time.Hour),
		cleaned:      true, // Already cleaned
	}

	// Add an entry to verify cleanup doesn't run
	client.pendingMu.Lock()
	client.pending[99] = req.responseChan
	client.pendingMu.Unlock()

	// Run cleanup on already-cleaned request
	req.cleanup(client)

	// Verify entry was NOT removed (because cleanup was already done)
	client.pendingMu.RLock()
	if _, ok := client.pending[99]; !ok {
		t.Error("pending entry should NOT be removed when already cleaned")
	}
	client.pendingMu.RUnlock()
}

func TestWSClient_WaitForResponse_Success(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")
	client.connected.Store(true)

	responseChan := make(chan *WSResultMessage, 1)

	// Send a successful response
	responseChan <- &WSResultMessage{
		Success: true,
		Result:  []byte(`{"test": "data"}`),
	}

	ctx := context.Background()
	result, err := client.waitForResponse(ctx, responseChan)

	if err != nil {
		t.Errorf("waitForResponse() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("waitForResponse() result is nil")
	}
	if !result.Success {
		t.Error("waitForResponse() result.Success = false, want true")
	}
}

func TestWSClient_WaitForResponse_Error(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")
	client.connected.Store(true)

	responseChan := make(chan *WSResultMessage, 1)

	// Send an error response
	responseChan <- &WSResultMessage{
		Success: false,
		Error: &WSError{
			Code:    "test_error",
			Message: "test error message",
		},
	}

	ctx := context.Background()
	result, err := client.waitForResponse(ctx, responseChan)

	if err == nil {
		t.Fatal("waitForResponse() error is nil, want error")
	}
	if result != nil {
		t.Error("waitForResponse() result should be nil on error")
	}
	expectedErr := "command failed: test_error - test error message"
	if err.Error() != expectedErr {
		t.Errorf("waitForResponse() error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestWSClient_WaitForResponse_ChannelClosed_Connected(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")
	client.connected.Store(true)

	responseChan := make(chan *WSResultMessage, 1)
	close(responseChan) // Simulate timeout cleanup

	ctx := context.Background()
	result, err := client.waitForResponse(ctx, responseChan)

	if err == nil {
		t.Fatal("waitForResponse() error is nil, want error")
	}
	if result != nil {
		t.Error("waitForResponse() result should be nil on closed channel")
	}
	if !errors.Is(err, ErrRequestTimeout) {
		t.Errorf("waitForResponse() error = %v, want ErrRequestTimeout", err)
	}
}

func TestWSClient_WaitForResponse_ChannelClosed_Disconnected(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")
	client.connected.Store(false) // Not connected

	responseChan := make(chan *WSResultMessage, 1)
	close(responseChan) // Simulate disconnect

	ctx := context.Background()
	result, err := client.waitForResponse(ctx, responseChan)

	if err == nil {
		t.Fatal("waitForResponse() error is nil, want error")
	}
	if result != nil {
		t.Error("waitForResponse() result should be nil on closed channel")
	}
	expectedErr := "connection closed while waiting for response"
	if err.Error() != expectedErr {
		t.Errorf("waitForResponse() error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestWSClient_WaitForResponse_ContextCanceled(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")
	client.connected.Store(true)

	responseChan := make(chan *WSResultMessage, 1)
	// Don't send anything - context will be canceled

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := client.waitForResponse(ctx, responseChan)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("waitForResponse() error = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Error("waitForResponse() result should be nil on context cancel")
	}
}

func TestWSClient_RegisterPendingRequest(t *testing.T) {
	t.Parallel()

	config := DefaultWSClientConfig()
	config.RequestTimeout = 50 * time.Millisecond // Short timeout for testing
	client := NewWSClientWithConfig("http://example.com", "token", config)

	// Register a request
	req := client.registerPendingRequest()

	if req == nil {
		t.Fatal("registerPendingRequest() returned nil")
	}

	if req.id != 1 {
		t.Errorf("registerPendingRequest() id = %d, want 1", req.id)
	}

	if req.responseChan == nil {
		t.Error("registerPendingRequest() responseChan is nil")
	}

	if req.timer == nil {
		t.Error("registerPendingRequest() timer is nil")
	}

	if req.cleaned {
		t.Error("registerPendingRequest() cleaned should be false initially")
	}

	// Verify it was added to pending map
	client.pendingMu.RLock()
	if _, ok := client.pending[req.id]; !ok {
		t.Error("pending map should contain the request")
	}
	client.pendingMu.RUnlock()

	// Stop timer to avoid affecting other tests
	req.timer.Stop()
}

func TestWSClient_RegisterPendingRequest_TimeoutCleanup(t *testing.T) {
	t.Parallel()

	config := DefaultWSClientConfig()
	config.RequestTimeout = 20 * time.Millisecond // Very short timeout
	client := NewWSClientWithConfig("http://example.com", "token", config)

	// Register a request
	req := client.registerPendingRequest()

	// Verify it's in the pending map
	client.pendingMu.RLock()
	if _, ok := client.pending[req.id]; !ok {
		t.Fatal("pending map should contain the request initially")
	}
	client.pendingMu.RUnlock()

	// Wait for timeout to trigger cleanup
	time.Sleep(50 * time.Millisecond)

	// Verify it was removed from pending map by timeout
	client.pendingMu.RLock()
	if _, ok := client.pending[req.id]; ok {
		t.Error("pending map should NOT contain the request after timeout")
	}
	client.pendingMu.RUnlock()

	// Channel should be closed
	select {
	case _, ok := <-req.responseChan:
		if ok {
			t.Error("response channel should be closed after timeout")
		}
	default:
		t.Error("response channel should be readable (closed) after timeout")
	}
}

func TestWSClient_SendCommand_NotConnected(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")
	// Don't connect - connected will be false

	ctx := context.Background()
	result, err := client.SendCommand(ctx, "test", nil)

	if err == nil {
		t.Fatal("SendCommand() error is nil, want error")
	}
	if result != nil {
		t.Error("SendCommand() result should be nil when not connected")
	}
	expectedErr := "not connected"
	if err.Error() != expectedErr {
		t.Errorf("SendCommand() error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestWSClient_Connect_TimeoutDuringAuth(t *testing.T) {
	t.Parallel()

	// Server that accepts WebSocket connection but never sends any message (auth_required never sent).
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		// Block until client disconnects
		<-r.Context().Done()
	}))
	defer s.Close()

	cfg := DefaultWSClientConfig()
	cfg.ConnectTimeout = 100 * time.Millisecond
	client := NewWSClientWithConfig(s.URL, "test_token", cfg)

	ctx := context.Background()
	start := time.Now()
	err := client.Connect(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Connect() expected error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Connect() error = %v, want errors.Is(err, context.DeadlineExceeded)", err)
	}

	// Must fail quickly within bounded time, not hang.
	if elapsed > 1*time.Second {
		t.Errorf("Connect() took %v, expected failure within ~100ms", elapsed)
	}
}

func TestWSClient_Connect_SuccessWithDefaultTimeout(t *testing.T) {
	t.Parallel()

	// Server that properly sends auth_required and handles auth_ok
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		// 1. Send auth_required
		authReq := map[string]any{"type": "auth_required", "ha_version": "2026.1.0"}
		data, _ := json.Marshal(authReq)
		if err := conn.Write(r.Context(), websocket.MessageText, data); err != nil {
			return
		}

		// 2. Read auth message
		_, authData, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var msg map[string]any
		if err := json.Unmarshal(authData, &msg); err != nil || msg["type"] != "auth" {
			return
		}

		// 3. Send auth_ok
		authOk := map[string]any{"type": "auth_ok", "ha_version": "2026.1.0"}
		okData, _ := json.Marshal(authOk)
		_ = conn.Write(r.Context(), websocket.MessageText, okData)

		// Stay open until client disconnects
		<-r.Context().Done()
	}))
	defer s.Close()

	client := NewWSClient(s.URL, "test_token")
	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("IsConnected() = false, want true")
	}
}

func TestWSClient_Connect_ZeroConnectTimeout(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		authReq := map[string]any{"type": "auth_required", "ha_version": "2026.1.0"}
		data, _ := json.Marshal(authReq)
		_ = conn.Write(r.Context(), websocket.MessageText, data)

		_, authData, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var msg map[string]any
		if err := json.Unmarshal(authData, &msg); err != nil || msg["type"] != "auth" {
			return
		}

		authOk := map[string]any{"type": "auth_ok", "ha_version": "2026.1.0"}
		okData, _ := json.Marshal(authOk)
		_ = conn.Write(r.Context(), websocket.MessageText, okData)

		<-r.Context().Done()
	}))
	defer s.Close()

	// Zero ConnectTimeout means no timeout is applied
	cfg := DefaultWSClientConfig()
	cfg.ConnectTimeout = 0
	client := NewWSClientWithConfig(s.URL, "test_token", cfg)

	ctx := context.Background()
	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() with zero ConnectTimeout failed: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("IsConnected() = false, want true")
	}
}

func TestWSClient_ConnectInternal_TimeoutAndSuccess(t *testing.T) {
	t.Parallel()

	t.Run("auth timeout during reconnect", func(t *testing.T) {
		t.Parallel()
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "done")
			<-r.Context().Done()
		}))
		defer s.Close()

		cfg := DefaultWSClientConfig()
		cfg.ConnectTimeout = 100 * time.Millisecond
		client := NewWSClientWithConfig(s.URL, "test_token", cfg)
		client.ctx = context.Background()

		err := client.connectInternal()
		if err == nil {
			t.Fatal("connectInternal() expected error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("connectInternal() error = %v, want errors.Is(err, context.DeadlineExceeded)", err)
		}
		if client.IsConnected() {
			t.Error("client should not be connected after failed connectInternal")
		}
	})

	t.Run("zero ConnectTimeout succeeds", func(t *testing.T) {
		t.Parallel()
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "done")

			authReq := map[string]any{"type": "auth_required", "ha_version": "2026.1.0"}
			data, _ := json.Marshal(authReq)
			_ = conn.Write(r.Context(), websocket.MessageText, data)

			_, authData, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var msg map[string]any
			if err := json.Unmarshal(authData, &msg); err != nil || msg["type"] != "auth" {
				return
			}

			authOk := map[string]any{"type": "auth_ok", "ha_version": "2026.1.0"}
			okData, _ := json.Marshal(authOk)
			_ = conn.Write(r.Context(), websocket.MessageText, okData)

			<-r.Context().Done()
		}))
		defer s.Close()

		cfg := DefaultWSClientConfig()
		cfg.ConnectTimeout = 0
		client := NewWSClientWithConfig(s.URL, "test_token", cfg)
		client.ctx = context.Background()

		err := client.connectInternal()
		if err != nil {
			t.Fatalf("connectInternal() failed: %v", err)
		}
		defer client.Close()

		if !client.IsConnected() {
			t.Error("client should be connected after successful connectInternal")
		}
	})
}
