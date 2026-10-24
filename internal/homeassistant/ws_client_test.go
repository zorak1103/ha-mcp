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
	if client.session.Load() != nil {
		t.Error("session should be nil initially")
	}
	if client.lifeCtx.Err() != nil {
		t.Errorf("lifeCtx should be live initially, got %v", client.lifeCtx.Err())
	}
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

func TestWSClient_WaitForResponse_Success(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	responseChan := make(chan wsResponse, 1)

	// Send a successful response
	responseChan <- wsResponse{msg: &WSResultMessage{
		Success: true,
		Result:  []byte(`{"test": "data"}`),
	}}

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

	responseChan := make(chan wsResponse, 1)

	// Send an error response
	responseChan <- wsResponse{msg: &WSResultMessage{
		Success: false,
		Error: &WSError{
			Code:    "test_error",
			Message: "test error message",
		},
	}}

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

func TestWSClient_WaitForResponse_RequestTimeout(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	responseChan := make(chan wsResponse, 1)
	responseChan <- wsResponse{err: ErrRequestTimeout} // What wsSession.expire delivers

	ctx := context.Background()
	result, err := client.waitForResponse(ctx, responseChan)

	if err == nil {
		t.Fatal("waitForResponse() error is nil, want error")
	}
	if result != nil {
		t.Error("waitForResponse() result should be nil on timeout")
	}
	if !errors.Is(err, ErrRequestTimeout) {
		t.Errorf("waitForResponse() error = %v, want ErrRequestTimeout", err)
	}
}

func TestWSClient_WaitForResponse_ConnectionLost(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	responseChan := make(chan wsResponse, 1)
	responseChan <- wsResponse{err: errConnectionLost} // What wsSession.end delivers

	ctx := context.Background()
	result, err := client.waitForResponse(ctx, responseChan)

	if err == nil {
		t.Fatal("waitForResponse() error is nil, want error")
	}
	if result != nil {
		t.Error("waitForResponse() result should be nil on connection loss")
	}
	expectedErr := "connection closed while waiting for response"
	if err.Error() != expectedErr {
		t.Errorf("waitForResponse() error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestWSClient_WaitForResponse_ContextCanceled(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")

	responseChan := make(chan wsResponse, 1)
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

	// Server that properly sends auth_required, handles auth_ok, and then echoes a
	// successful result for any command it receives.
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

		// 4. Echo a successful result for every command received - proves the
		// connection is still usable after Connect() returns and the ConnectTimeout's
		// deferred cancel() has fired, not just that IsConnected() reports true.
		for {
			_, cmdData, readErr := conn.Read(r.Context())
			if readErr != nil {
				return
			}
			var cmd map[string]any
			if unmarshalErr := json.Unmarshal(cmdData, &cmd); unmarshalErr != nil {
				return
			}
			result := map[string]any{
				"id": cmd["id"], "type": "result", "success": true, "result": map[string]any{},
			}
			resData, _ := json.Marshal(result)
			if writeErr := conn.Write(r.Context(), websocket.MessageText, resData); writeErr != nil {
				return
			}
		}
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

	result, err := client.SendCommand(ctx, "get_config", nil)
	if err != nil {
		t.Fatalf("SendCommand() after Connect failed (connection may have been torn down by the dial context's deferred cancel): %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected a successful command result, got: %+v", result)
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

func TestWSClient_DialSession_TimeoutAndSuccess(t *testing.T) {
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
		defer client.Close()

		err := client.dialSession(context.Background())
		if err == nil {
			t.Fatal("dialSession() expected error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("dialSession() error = %v, want errors.Is(err, context.DeadlineExceeded)", err)
		}
		if client.IsConnected() {
			t.Error("client should not be connected after failed dialSession")
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

			// Echo a successful result for every command received - proves the
			// session dialSession() installed (with its own read loop) is still
			// usable after the dial context's deferred cancel, not just that
			// IsConnected() is true.
			for {
				_, cmdData, readErr := conn.Read(r.Context())
				if readErr != nil {
					return
				}
				var cmd map[string]any
				if unmarshalErr := json.Unmarshal(cmdData, &cmd); unmarshalErr != nil {
					return
				}
				result := map[string]any{
					"id": cmd["id"], "type": "result", "success": true, "result": map[string]any{},
				}
				resData, _ := json.Marshal(result)
				if writeErr := conn.Write(r.Context(), websocket.MessageText, resData); writeErr != nil {
					return
				}
			}
		}))
		defer s.Close()

		cfg := DefaultWSClientConfig()
		cfg.ConnectTimeout = 0
		client := NewWSClientWithConfig(s.URL, "test_token", cfg)

		err := client.dialSession(context.Background())
		if err != nil {
			t.Fatalf("dialSession() failed: %v", err)
		}
		defer client.Close()

		if !client.IsConnected() {
			t.Error("client should be connected after successful dialSession")
		}

		result, err := client.SendCommand(context.Background(), "get_config", nil)
		if err != nil {
			t.Fatalf("SendCommand() after dialSession failed (connection may have been torn down by the dial context's deferred cancel): %v", err)
		}
		if result == nil || !result.Success {
			t.Fatalf("expected a successful command result, got: %+v", result)
		}
	})
}
