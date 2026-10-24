package homeassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// newAuthStubWSServer returns a server that completes the Home Assistant
// WebSocket auth handshake and then keeps the connection open until the
// client disconnects.
func newAuthStubWSServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		authReq, _ := json.Marshal(map[string]any{"type": "auth_required", "ha_version": "2026.1.0"})
		if err := conn.Write(r.Context(), websocket.MessageText, authReq); err != nil {
			return
		}
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
		authOK, _ := json.Marshal(map[string]any{"type": "auth_ok", "ha_version": "2026.1.0"})
		if err := conn.Write(r.Context(), websocket.MessageText, authOK); err != nil {
			return
		}
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClientPool_ClientOptions(t *testing.T) {
	t.Parallel()

	restCfg := &RESTClientConfig{RetryConfig: RetryConfig{MaxRetries: 5}}

	t.Run("without option leaves WSConfig nil", func(t *testing.T) {
		t.Parallel()
		pool := NewClientPoolWithFullConfig("http://ha.invalid", time.Minute, 0, restCfg, nil, nil)
		defer pool.Close()

		opts := pool.clientOptions()
		if opts.WSConfig != nil {
			t.Errorf("WSConfig = %+v, want nil", opts.WSConfig)
		}
		if opts.RESTConfig != restCfg {
			t.Error("RESTConfig is not the pool's REST config pointer")
		}
	})

	t.Run("with WithWSConfig carries the config", func(t *testing.T) {
		t.Parallel()
		ws := DefaultWSClientConfig()
		ws.RetryConfig.MaxRetries = 7
		pool := NewClientPoolWithFullConfig("http://ha.invalid", time.Minute, 0, restCfg, nil, nil, WithWSConfig(&ws))
		defer pool.Close()

		opts := pool.clientOptions()
		if opts.WSConfig == nil {
			t.Fatal("WSConfig = nil, want configured value")
		}
		if got := opts.WSConfig.RetryConfig.MaxRetries; got != 7 {
			t.Errorf("WSConfig.RetryConfig.MaxRetries = %d, want 7", got)
		}
	})

	t.Run("nil option value means client defaults", func(t *testing.T) {
		t.Parallel()
		pool := NewClientPoolWithFullConfig("http://ha.invalid", time.Minute, 0, restCfg, nil, nil, WithWSConfig(nil))
		defer pool.Close()

		if pool.clientOptions().WSConfig != nil {
			t.Error("WSConfig != nil, want nil")
		}
	})
}

func TestClientPool_GetOrCreate_PooledClientHasConfiguredRetries(t *testing.T) {
	t.Parallel()

	srv := newAuthStubWSServer(t)

	ws := DefaultWSClientConfig()
	ws.RetryConfig.MaxRetries = 7
	restCfg := &RESTClientConfig{RetryConfig: RetryConfig{
		MaxRetries: 5, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond, BackoffFactor: 2,
	}}

	pool := NewClientPoolWithFullConfig(srv.URL, time.Minute, 0, restCfg, nil, nil, WithWSConfig(&ws))
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := pool.GetOrCreate(ctx, "retry-token")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	hc, ok := c.(*HybridClientCloser)
	if !ok {
		t.Fatalf("client type = %T, want *HybridClientCloser", c)
	}
	if got := hc.wsClient.config.RetryConfig.MaxRetries; got != 7 {
		t.Errorf("WS MaxRetries = %d, want 7", got)
	}
	rest, ok := hc.rest.(*RESTClient)
	if !ok {
		t.Fatalf("rest type = %T, want *RESTClient", hc.rest)
	}
	if got := rest.retryManager.config.MaxRetries; got != 5 {
		t.Errorf("REST MaxRetries = %d, want 5", got)
	}
}
