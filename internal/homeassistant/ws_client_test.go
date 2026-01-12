package homeassistant

import (
	"errors"
	"testing"
	"time"
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
