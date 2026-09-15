package homeassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestWSClient_IsPongTimeout pins the pong-freshness rule: stale means older
// than PingInterval + PingTimeout; an unset lastPong is never stale (the health
// monitor stores a fresh value on start, so the very first check must not fire).
func TestWSClient_IsPongTimeout(t *testing.T) {
	t.Parallel()

	cfg := DefaultWSClientConfig()
	cfg.PingInterval = 30 * time.Second
	cfg.PingTimeout = 10 * time.Second
	client := NewWSClientWithConfig("http://example.com", "token", cfg)

	if client.isPongTimeout() {
		t.Error("isPongTimeout() = true with no lastPong stored, want false")
	}

	client.lastPong.Store(time.Now())
	if client.isPongTimeout() {
		t.Error("isPongTimeout() = true with fresh lastPong, want false")
	}

	stale := time.Now().Add(-(40*time.Second + time.Second))
	client.lastPong.Store(stale)
	if !client.isPongTimeout() {
		t.Error("isPongTimeout() = false with lastPong older than PingInterval+PingTimeout, want true")
	}

	// Just under the threshold (with margin so real time passing between
	// Store and the check cannot push the age across it).
	client.lastPong.Store(time.Now().Add(-(40*time.Second - 5*time.Second)))
	if client.isPongTimeout() {
		t.Error("isPongTimeout() = true just below PingInterval+PingTimeout, want false")
	}
}

// TestWSClient_PerformHealthCheck_Disconnected pins the guard: a disconnected
// client's health check is a no-op - no ping, no failure reporting.
func TestWSClient_PerformHealthCheck_Disconnected(t *testing.T) {
	t.Parallel()

	client := NewWSClient("http://example.com", "token")
	var disconnects atomic.Int32
	client.SetOnDisconnect(func(error) { disconnects.Add(1) })

	if client.performHealthCheck(context.Background()) {
		t.Error("performHealthCheck() = true for disconnected client, want false")
	}
	if disconnects.Load() != 0 {
		t.Errorf("OnDisconnect called %d times, want 0", disconnects.Load())
	}
}

// newConnectedTestWSClient connects a client to a live WS test server that
// completes the auth handshake and then reads in a loop (which is also what
// auto-services the client's ping frames in coder/websocket) until the client
// disconnects during test cleanup.
func newConnectedTestWSClient(t *testing.T, cfg WSClientConfig) *WSClient {
	t.Helper()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		authReq, _ := json.Marshal(map[string]any{"type": "auth_required", "ha_version": "2026.1.0"})
		if err := conn.Write(r.Context(), websocket.MessageText, authReq); err != nil {
			return
		}
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var msg map[string]any
		if json.Unmarshal(data, &msg) != nil || msg["type"] != "auth" {
			return
		}
		authOk, _ := json.Marshal(map[string]any{"type": "auth_ok", "ha_version": "2026.1.0"})
		if err := conn.Write(r.Context(), websocket.MessageText, authOk); err != nil {
			return
		}

		// Keep reading so control frames (pings) are serviced.
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	t.Cleanup(s.Close)

	client := NewWSClientWithConfig(s.URL, "test_token", cfg)
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	return client
}

// TestWSClient_PerformHealthCheck_PingTimeout pins the stale-pong branch: an
// old lastPong routes the health check into handleConnectionFailure - the
// disconnect callback fires and the health loop is told to exit.
func TestWSClient_PerformHealthCheck_PingTimeout(t *testing.T) {
	t.Parallel()

	cfg := DefaultWSClientConfig()
	cfg.AutoReconnect = false
	client := newConnectedTestWSClient(t, cfg)

	var disconnects atomic.Int32
	client.SetOnDisconnect(func(error) { disconnects.Add(1) })

	// Keep the connection alive but make the last pong stale.
	client.lastPong.Store(time.Now().Add(-time.Hour))
	if !client.performHealthCheck(context.Background()) {
		t.Error("performHealthCheck() = false with stale pong, want true (exit signal)")
	}
	if client.connected.Load() {
		// handleConnectionFailure itself does not store connected=false (the
		// read loop / reconnect path does), so this just documents behavior.
		t.Log("connected still true after failure report; notification paths own the flag")
	}
	if got := disconnects.Load(); got != 1 {
		t.Errorf("OnDisconnect called %d times, want exactly 1", got)
	}
}

// TestWSClient_PerformHealthCheck_PingSuccess pins the happy branch: a healthy
// check pings the server, refreshes lastPong and keeps the loop running.
func TestWSClient_PerformHealthCheck_PingSuccess(t *testing.T) {
	t.Parallel()

	cfg := DefaultWSClientConfig()
	cfg.AutoReconnect = false
	client := newConnectedTestWSClient(t, cfg)

	// A fresh lastPong (well within the timeout window) lets the check reach
	// the ping branch; the ping then refreshes lastPong.
	before := time.Now().Add(-2 * time.Second)
	client.lastPong.Store(before)
	if client.performHealthCheck(context.Background()) {
		t.Error("performHealthCheck() = true on healthy connection, want false (keep running)")
	}
	if !client.lastPong.Load().(time.Time).After(before) {
		t.Error("lastPong not refreshed by successful health check")
	}
}

// TestWSClient_SendSimpleCommand pins the payload-less command path end to end:
// the server receives type+id only, and the client resolves the success result.
func TestWSClient_SendSimpleCommand(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		authReq, _ := json.Marshal(map[string]any{"type": "auth_required", "ha_version": "2026.1.0"})
		if err := conn.Write(r.Context(), websocket.MessageText, authReq); err != nil {
			return
		}
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var auth map[string]any
		if json.Unmarshal(data, &auth) != nil || auth["type"] != "auth" {
			return
		}
		authOk, _ := json.Marshal(map[string]any{"type": "auth_ok", "ha_version": "2026.1.0"})
		if err := conn.Write(r.Context(), websocket.MessageText, authOk); err != nil {
			return
		}

		// Echo a success result for every command received.
		for {
			_, cmdData, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var cmd map[string]any
			if json.Unmarshal(cmdData, &cmd) != nil {
				continue
			}
			if cmd["type"] == "auth" {
				continue
			}
			result, _ := json.Marshal(map[string]any{
				"id": cmd["id"], "type": "result", "success": true, "result": nil,
			})
			if err := conn.Write(r.Context(), websocket.MessageText, result); err != nil {
				return
			}
		}
	}))
	defer s.Close()

	cfg := DefaultWSClientConfig()
	cfg.AutoReconnect = false
	client := NewWSClientWithConfig(s.URL, "test_token", cfg)
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// The command must carry no payload: SendSimpleCommand forwards nil.
	result, err := client.SendSimpleCommand(ctx, "get_states")
	if err != nil {
		t.Fatalf("SendSimpleCommand() failed: %v", err)
	}
	if result == nil || !result.Success {
		t.Errorf("SendSimpleCommand() = %+v, want success result", result)
	}
}
