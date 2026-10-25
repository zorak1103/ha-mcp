package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

// disconnectedWaiterClient is a client that reports itself disconnected and
// records WaitForConnection calls.
type disconnectedWaiterClient struct {
	*mockHAClient
	waitCalls int
	waitErr   error
}

func (*disconnectedWaiterClient) IsConnected() bool { return false }

func (c *disconnectedWaiterClient) WaitForConnection(context.Context) error {
	c.waitCalls++
	return c.waitErr
}

// TestWaitForClientConnection_ThroughCachedClient pins that the reconnect wait
// still fires when the default client is wrapped in a CachedClient
// (HA_CACHE_ENABLED=true).
func TestWaitForClientConnection_ThroughCachedClient(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("still down")
	inner := &disconnectedWaiterClient{mockHAClient: &mockHAClient{}, waitErr: wantErr}
	cached := homeassistant.NewCachedClient(inner, config.CacheConfig{Enabled: true}, logging.New(logging.LevelError))

	if err := waitForClientConnection(context.Background(), cached); !errors.Is(err, wantErr) {
		t.Errorf("waitForClientConnection() = %v, want %v", err, wantErr)
	}
	if inner.waitCalls != 1 {
		t.Errorf("inner WaitForConnection calls = %d, want 1", inner.waitCalls)
	}
}
