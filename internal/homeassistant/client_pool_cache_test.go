package homeassistant

import (
	"context"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/config"
)

func newCachePoolTestRESTConfig() *RESTClientConfig {
	return &RESTClientConfig{RetryConfig: RetryConfig{
		MaxRetries: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond, BackoffFactor: 2,
	}}
}

func TestClientPool_CacheConfig(t *testing.T) {
	t.Parallel()

	cacheCfg := &config.CacheConfig{Enabled: true, ServicesTTLMin: 3}
	pool := NewClientPoolWithFullConfig("http://ha.invalid", time.Minute, 0, newCachePoolTestRESTConfig(), cacheCfg, nil)
	defer pool.Close()

	if pool.CacheConfig() != cacheCfg {
		t.Error("CacheConfig() is not the pool's cache config pointer")
	}

	bare := NewClientPoolWithFullConfig("http://ha.invalid", time.Minute, 0, newCachePoolTestRESTConfig(), nil, nil)
	defer bare.Close()

	if bare.CacheConfig() != nil {
		t.Errorf("CacheConfig() = %+v, want nil", bare.CacheConfig())
	}
}

// TestClientPool_GetOrCreate_WrapsInCachedClientWhenEnabled guards #299: a
// per-token pooled client must be cached when the pool's cache config is
// enabled and a plain hybrid client otherwise.
func TestClientPool_GetOrCreate_WrapsInCachedClientWhenEnabled(t *testing.T) {
	t.Parallel()

	srv := newAuthStubWSServer(t)

	t.Run("enabled returns CachedClient", func(t *testing.T) {
		t.Parallel()

		pool := NewClientPoolWithFullConfig(srv.URL, time.Minute, 0, newCachePoolTestRESTConfig(),
			&config.CacheConfig{Enabled: true}, nil)
		defer pool.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		c, err := pool.GetOrCreate(ctx, "t1")
		if err != nil {
			t.Fatalf("GetOrCreate() error = %v", err)
		}
		if _, ok := c.(*CachedClient); !ok {
			t.Errorf("client type = %T, want *CachedClient", c)
		}
	})

	t.Run("disabled returns HybridClientCloser", func(t *testing.T) {
		t.Parallel()

		pool := NewClientPoolWithFullConfig(srv.URL, time.Minute, 0, newCachePoolTestRESTConfig(),
			&config.CacheConfig{Enabled: false}, nil)
		defer pool.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		c, err := pool.GetOrCreate(ctx, "t1")
		if err != nil {
			t.Fatalf("GetOrCreate() error = %v", err)
		}
		if _, ok := c.(*HybridClientCloser); !ok {
			t.Errorf("client type = %T, want *HybridClientCloser", c)
		}
	})
}

func TestClientPool_GetOrCreate_CachePerToken(t *testing.T) {
	t.Parallel()

	srv := newAuthStubWSServer(t)

	pool := NewClientPoolWithFullConfig(srv.URL, time.Minute, 0, newCachePoolTestRESTConfig(),
		&config.CacheConfig{Enabled: true}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first, err := pool.GetOrCreate(ctx, "t1")
	if err != nil {
		t.Fatalf("GetOrCreate(t1) error = %v", err)
	}
	other, err := pool.GetOrCreate(ctx, "t2")
	if err != nil {
		t.Fatalf("GetOrCreate(t2) error = %v", err)
	}
	again, err := pool.GetOrCreate(ctx, "t1")
	if err != nil {
		t.Fatalf("GetOrCreate(t1) again error = %v", err)
	}

	firstCached, ok := first.(*CachedClient)
	if !ok {
		t.Fatalf("t1 client type = %T, want *CachedClient", first)
	}
	otherCached, ok := other.(*CachedClient)
	if !ok {
		t.Fatalf("t2 client type = %T, want *CachedClient", other)
	}
	if firstCached == otherCached {
		t.Error("tokens t1 and t2 share one CachedClient, want one per token")
	}
	if again != first {
		t.Error("second GetOrCreate(t1) returned a different client, want the identical pointer")
	}

	if err := pool.Close(); err != nil {
		t.Errorf("pool.Close() = %v, want nil", err)
	}
}
