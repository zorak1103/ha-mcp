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

// TestWSClient_OnDisconnect_CalledOncePerDrop pins that a single connection
// loss reports OnDisconnect exactly once, even though two independent paths
// can observe the drop: the health monitor (ping failure →
// handleConnectionFailure) and the read loop (conn.Read error). Both must
// deduplicate their notification.
func TestWSClient_OnDisconnect_CalledOncePerDrop(t *testing.T) {
	t.Parallel()

	drop := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		// Complete the auth handshake so Connect() succeeds.
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

		// Hold the connection open until the test orders the drop.
		<-drop
	}))
	defer s.Close()

	cfg := DefaultWSClientConfig()
	cfg.AutoReconnect = false
	cfg.PingInterval = 5 * time.Millisecond
	cfg.PingTimeout = 50 * time.Millisecond

	client := NewWSClientWithConfig(s.URL, "test_token", cfg)
	t.Cleanup(func() { _ = client.Close() })

	var disconnects atomic.Int32
	client.SetOnDisconnect(func(error) { disconnects.Add(1) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Server drops the connection out from under the client.
	close(drop)

	// Wait until the client noticed the drop, then give both notification
	// paths (health monitor + read loop) ample time to fire.
	deadline := time.Now().Add(2 * time.Second)
	for client.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	if got := disconnects.Load(); got != 1 {
		t.Errorf("OnDisconnect called %d times for a single connection drop, want exactly 1", got)
	}
}
