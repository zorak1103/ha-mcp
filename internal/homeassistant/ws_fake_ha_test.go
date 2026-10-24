package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/coder/websocket"
)

// fakeHA is a minimal Home Assistant websocket server for concurrency tests.
// It performs the auth handshake and answers every command with a success
// result, enforcing HA's rule that message ids must strictly increase per
// connection (a repeated or lower id gets an "id_reuse" error, like real HA).
type fakeHA struct {
	// URL is the http:// base URL of the server.
	URL string
	// conns counts authenticated connections so far; each connection gets the
	// 1-based number it was counted with.
	conns atomic.Int32
	// received counts non-auth commands received across all connections.
	received atomic.Int32

	attempts        atomic.Int32
	holdOnConn      int32
	pauseReads      <-chan struct{}
	dropAfterAuthOn int32
	acceptLimit     int32

	mu     sync.Mutex
	active map[*websocket.Conn]struct{}
	stop   chan struct{}
}

// fakeHAOption configures a fakeHA.
type fakeHAOption func(*fakeHA)

// withHoldOnConn makes "test/hold" commands on connection n get no reply.
func withHoldOnConn(n int32) fakeHAOption {
	return func(f *fakeHA) { f.holdOnConn = n }
}

// withPauseReads makes the server wait on ch after auth before reading commands.
func withPauseReads(ch <-chan struct{}) fakeHAOption {
	return func(f *fakeHA) { f.pauseReads = ch }
}

// withDropAfterAuthOnConn makes connection n close right after auth_ok.
func withDropAfterAuthOnConn(n int32) fakeHAOption {
	return func(f *fakeHA) { f.dropAfterAuthOn = n }
}

// withAcceptLimit rejects connection attempts beyond n with HTTP 503.
func withAcceptLimit(n int32) fakeHAOption {
	return func(f *fakeHA) { f.acceptLimit = n }
}

// newFakeHA starts a fake HA server that is shut down at test cleanup.
func newFakeHA(t *testing.T, opts ...fakeHAOption) *fakeHA {
	t.Helper()
	f := &fakeHA{
		active: make(map[*websocket.Conn]struct{}),
		stop:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(f)
	}
	srv := httptest.NewServer(http.HandlerFunc(f.handle))
	f.URL = srv.URL
	t.Cleanup(func() {
		close(f.stop)
		f.dropAll()
		srv.Close()
	})
	return f
}

// dropAll closes every active server connection with StatusGoingAway.
func (f *fakeHA) dropAll() {
	f.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(f.active))
	for c := range f.active {
		conns = append(conns, c)
	}
	f.mu.Unlock()

	var wg sync.WaitGroup
	for _, c := range conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Close(websocket.StatusGoingAway, "fake HA drop")
		}()
	}
	wg.Wait()
}

func (f *fakeHA) track(c *websocket.Conn) {
	f.mu.Lock()
	f.active[c] = struct{}{}
	f.mu.Unlock()
}

func (f *fakeHA) untrack(c *websocket.Conn) {
	f.mu.Lock()
	delete(f.active, c)
	f.mu.Unlock()
}

func (f *fakeHA) handle(w http.ResponseWriter, r *http.Request) {
	attempt := f.attempts.Add(1)
	if f.acceptLimit > 0 && attempt > f.acceptLimit {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(-1)

	ctx := r.Context()
	if !fakeHAHandshake(ctx, conn) {
		return
	}
	num := f.conns.Add(1)
	if num == f.dropAfterAuthOn {
		_ = conn.Close(websocket.StatusGoingAway, "fake HA drop after auth")
		return
	}

	f.track(conn)
	defer f.untrack(conn)

	if f.pauseReads != nil {
		select {
		case <-f.pauseReads:
		case <-f.stop:
			return
		case <-ctx.Done():
			return
		}
	}
	f.serve(ctx, conn, num)
}

// fakeHAHandshake runs auth_required -> auth -> auth_ok.
func fakeHAHandshake(ctx context.Context, conn *websocket.Conn) bool {
	if conn.Write(ctx, websocket.MessageText, []byte(`{"type":"auth_required","ha_version":"2026.1.0"}`)) != nil {
		return false
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		return false
	}
	var msg struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(data, &msg) != nil || msg.Type != "auth" {
		return false
	}
	return conn.Write(ctx, websocket.MessageText, []byte(`{"type":"auth_ok","ha_version":"2026.1.0"}`)) == nil
}

// serve is the per-connection reader: it tracks the last id seen and replies.
func (f *fakeHA) serve(ctx context.Context, conn *websocket.Conn, num int32) {
	var lastID int64
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var cmd struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &cmd) != nil {
			continue
		}
		f.received.Add(1)

		var reply string
		if cmd.ID <= lastID {
			reply = fmt.Sprintf(`{"id":%d,"type":"result","success":false,"error":{"code":"id_reuse","message":"Identifier values have to increase."}}`, cmd.ID)
		} else {
			lastID = cmd.ID
			if num == f.holdOnConn && cmd.Type == "test/hold" {
				continue
			}
			reply = fmt.Sprintf(`{"id":%d,"type":"result","success":true,"result":{}}`, cmd.ID)
		}
		if conn.Write(ctx, websocket.MessageText, []byte(reply)) != nil {
			return
		}
	}
}
