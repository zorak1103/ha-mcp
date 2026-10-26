package homeassistant

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

var errInnerWrite = errors.New("inner write failed")

// failingWriteClient makes every mutation covered by the issue #320 invalidation
// tests fail, to prove the registry caches are busted even when the outcome of
// the write is unknown.
type failingWriteClient struct{ *mockClient }

func (failingWriteClient) CreateAutomation(context.Context, AutomationConfig) error {
	return errInnerWrite
}

func (failingWriteClient) CallService(context.Context, string, string, map[string]any) ([]Entity, error) {
	return nil, errInnerWrite
}

func newInvalidationTestClient(t *testing.T, inner Client) *CachedClient {
	t.Helper()
	cfg := config.CacheConfig{Enabled: true}
	cc, ok := NewCachedClient(inner, cfg, logging.New(logging.LevelError)).(*CachedClient)
	if !ok {
		t.Fatal("NewCachedClient did not return a *CachedClient")
	}
	return cc
}

// TestCachedClient_WritesInvalidateRegistryCaches pins issue #320: every
// automation/script/scene write, UpdateHelper and every service call must
// force the next registry read to hit the inner client again.
func TestCachedClient_WritesInvalidateRegistryCaches(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		call func(c *CachedClient) error
	}{
		{"CreateAutomation", func(c *CachedClient) error { return c.CreateAutomation(ctx, AutomationConfig{}) }},
		{"UpdateAutomation", func(c *CachedClient) error { return c.UpdateAutomation(ctx, "a", AutomationConfig{}) }},
		{"DeleteAutomation", func(c *CachedClient) error { return c.DeleteAutomation(ctx, "a") }},
		{"CreateScript", func(c *CachedClient) error { return c.CreateScript(ctx, "s", ScriptConfig{}) }},
		{"UpdateScript", func(c *CachedClient) error { return c.UpdateScript(ctx, "s", ScriptConfig{}) }},
		{"DeleteScript", func(c *CachedClient) error { return c.DeleteScript(ctx, "s") }},
		{"CreateScene", func(c *CachedClient) error { return c.CreateScene(ctx, "sc", SceneConfig{}) }},
		{"UpdateScene", func(c *CachedClient) error { return c.UpdateScene(ctx, "sc", SceneConfig{}) }},
		{"DeleteScene", func(c *CachedClient) error { return c.DeleteScene(ctx, "sc") }},
		{"UpdateHelper", func(c *CachedClient) error { return c.UpdateHelper(ctx, "h", HelperConfig{}) }},
		{"CallService", func(c *CachedClient) error {
			_, err := c.CallService(ctx, "automation", "reload", nil)
			return err
		}},
		{"CallServiceWithResponse", func(c *CachedClient) error {
			_, err := c.CallServiceWithResponse(ctx, "script", "reload", nil)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClient{}
			c := newInvalidationTestClient(t, mock)

			if _, err := c.GetEntityRegistry(ctx); err != nil {
				t.Fatalf("GetEntityRegistry: %v", err)
			}
			if _, err := c.GetDeviceRegistry(ctx); err != nil {
				t.Fatalf("GetDeviceRegistry: %v", err)
			}
			if _, err := c.GetAreaRegistry(ctx); err != nil {
				t.Fatalf("GetAreaRegistry: %v", err)
			}

			if err := tt.call(c); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}

			if _, err := c.GetEntityRegistry(ctx); err != nil {
				t.Fatalf("GetEntityRegistry: %v", err)
			}
			if _, err := c.GetDeviceRegistry(ctx); err != nil {
				t.Fatalf("GetDeviceRegistry: %v", err)
			}
			if _, err := c.GetAreaRegistry(ctx); err != nil {
				t.Fatalf("GetAreaRegistry: %v", err)
			}

			if mock.entityRegistryCallCount != 2 {
				t.Errorf("entity registry fetched %d times, want 2 (cache not invalidated)", mock.entityRegistryCallCount)
			}
			if mock.deviceRegistryCallCount != 2 {
				t.Errorf("device registry fetched %d times, want 2 (cache not invalidated)", mock.deviceRegistryCallCount)
			}
			if mock.areaRegistryCallCount != 2 {
				t.Errorf("area registry fetched %d times, want 2 (cache not invalidated)", mock.areaRegistryCallCount)
			}
		})
	}
}

// TestCachedClient_FailedWriteStillInvalidates pins that an error from the
// inner write does not leave a possibly-stale registry cache in place: a
// transport error after the request was sent leaves the outcome unknown.
func TestCachedClient_FailedWriteStillInvalidates(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		call func(c *CachedClient) error
	}{
		{"CreateAutomation", func(c *CachedClient) error { return c.CreateAutomation(ctx, AutomationConfig{}) }},
		{"CallService", func(c *CachedClient) error {
			_, err := c.CallService(ctx, "automation", "reload", nil)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClient{}
			c := newInvalidationTestClient(t, failingWriteClient{mock})

			if _, err := c.GetEntityRegistry(ctx); err != nil {
				t.Fatalf("GetEntityRegistry: %v", err)
			}
			if err := tt.call(c); !errors.Is(err, errInnerWrite) {
				t.Fatalf("%s error = %v, want errInnerWrite", tt.name, err)
			}
			if _, err := c.GetEntityRegistry(ctx); err != nil {
				t.Fatalf("GetEntityRegistry: %v", err)
			}
			if mock.entityRegistryCallCount != 2 {
				t.Errorf("entity registry fetched %d times, want 2 (cache not invalidated after failed write)", mock.entityRegistryCallCount)
			}
		})
	}
}

// TestCachedClient_InvalidateAllCaches pins that the reconnect path also clears
// the services and config caches, which no write path ever invalidates.
func TestCachedClient_InvalidateAllCaches(t *testing.T) {
	ctx := context.Background()
	mock := &mockClient{}
	c := newInvalidationTestClient(t, mock)

	if _, err := c.GetServices(ctx); err != nil {
		t.Fatalf("GetServices: %v", err)
	}
	if _, err := c.GetConfig(ctx); err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if _, err := c.GetEntityRegistry(ctx); err != nil {
		t.Fatalf("GetEntityRegistry: %v", err)
	}

	c.invalidateAllCaches()

	if _, err := c.GetServices(ctx); err != nil {
		t.Fatalf("GetServices: %v", err)
	}
	if _, err := c.GetConfig(ctx); err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if _, err := c.GetEntityRegistry(ctx); err != nil {
		t.Fatalf("GetEntityRegistry: %v", err)
	}

	if mock.servicesCallCount != 2 {
		t.Errorf("services fetched %d times, want 2", mock.servicesCallCount)
	}
	if mock.configCallCount != 2 {
		t.Errorf("config fetched %d times, want 2", mock.configCallCount)
	}
	if mock.entityRegistryCallCount != 2 {
		t.Errorf("entity registry fetched %d times, want 2", mock.entityRegistryCallCount)
	}
}

// TestCachedClient_RegistryInvalidationKeepsServicesAndConfig pins the documented
// limit: write-triggered invalidation leaves the services/config caches alone.
func TestCachedClient_RegistryInvalidationKeepsServicesAndConfig(t *testing.T) {
	ctx := context.Background()
	mock := &mockClient{}
	c := newInvalidationTestClient(t, mock)

	if _, err := c.GetServices(ctx); err != nil {
		t.Fatalf("GetServices: %v", err)
	}
	if _, err := c.GetConfig(ctx); err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if err := c.CreateAutomation(ctx, AutomationConfig{}); err != nil {
		t.Fatalf("CreateAutomation: %v", err)
	}
	if _, err := c.GetServices(ctx); err != nil {
		t.Fatalf("GetServices: %v", err)
	}
	if _, err := c.GetConfig(ctx); err != nil {
		t.Fatalf("GetConfig: %v", err)
	}

	if mock.servicesCallCount != 1 || mock.configCallCount != 1 {
		t.Errorf("services/config fetched %d/%d times, want 1/1 (writes must not clear them)",
			mock.servicesCallCount, mock.configCallCount)
	}
}

func TestWithReconnectInvalidation(t *testing.T) {
	ctx := context.Background()

	t.Run("clears all caches and chains the previous callback", func(t *testing.T) {
		mock := &mockClient{}
		c := newInvalidationTestClient(t, mock)
		if _, err := c.GetServices(ctx); err != nil {
			t.Fatalf("GetServices: %v", err)
		}
		if _, err := c.GetConfig(ctx); err != nil {
			t.Fatalf("GetConfig: %v", err)
		}

		var prevAttempts atomic.Int32
		base := &WSClientConfig{OnReconnect: func(attempts int) { prevAttempts.Store(int32(attempts)) }}
		var target atomic.Pointer[CachedClient]
		target.Store(c)

		cfg := withReconnectInvalidation(base, &target)
		cfg.OnReconnect(3)

		if got := prevAttempts.Load(); got != 3 {
			t.Errorf("previous OnReconnect saw attempts=%d, want 3", got)
		}
		if _, err := c.GetServices(ctx); err != nil {
			t.Fatalf("GetServices: %v", err)
		}
		if _, err := c.GetConfig(ctx); err != nil {
			t.Fatalf("GetConfig: %v", err)
		}
		if mock.servicesCallCount != 2 || mock.configCallCount != 2 {
			t.Errorf("services/config fetched %d/%d times, want 2/2", mock.servicesCallCount, mock.configCallCount)
		}
	})

	t.Run("does not mutate the shared base config", func(t *testing.T) {
		base := &WSClientConfig{}
		var target atomic.Pointer[CachedClient]
		_ = withReconnectInvalidation(base, &target)
		if base.OnReconnect != nil {
			t.Error("withReconnectInvalidation mutated the caller's WSClientConfig")
		}
	})

	t.Run("nil base uses defaults", func(t *testing.T) {
		var target atomic.Pointer[CachedClient]
		cfg := withReconnectInvalidation(nil, &target)
		if cfg.OnReconnect == nil {
			t.Fatal("OnReconnect not set")
		}
		if cfg.ReconnectConfig != DefaultReconnectConfig() {
			t.Error("nil base did not fall back to DefaultWSClientConfig")
		}
	})

	t.Run("reconnect before the cache is attached is a no-op", func(*testing.T) {
		var target atomic.Pointer[CachedClient]
		cfg := withReconnectInvalidation(nil, &target)
		cfg.OnReconnect(1) // must not panic
	})
}
