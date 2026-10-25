package main

import (
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

func TestBuildRetryConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		maxRetries   int
		initialDelay int
		maxDelay     int
		want         homeassistant.RetryConfig
	}{
		{
			name: "all values taken as given", maxRetries: 3, initialDelay: 100, maxDelay: 5000,
			want: homeassistant.RetryConfig{MaxRetries: 3, InitialDelay: 100 * time.Millisecond, MaxDelay: 5 * time.Second, BackoffFactor: 2.0},
		},
		{
			name: "zero retries disables retrying", maxRetries: 0, initialDelay: 100, maxDelay: 5000,
			want: homeassistant.RetryConfig{MaxRetries: 0, InitialDelay: 100 * time.Millisecond, MaxDelay: 5 * time.Second, BackoffFactor: 2.0},
		},
		{
			name: "negative retries stays non-positive", maxRetries: -1, initialDelay: 100, maxDelay: 5000,
			want: homeassistant.RetryConfig{MaxRetries: -1, InitialDelay: 100 * time.Millisecond, MaxDelay: 5 * time.Second, BackoffFactor: 2.0},
		},
		{
			name: "zero initial delay keeps default", maxRetries: 5, initialDelay: 0, maxDelay: 5000,
			want: homeassistant.RetryConfig{MaxRetries: 5, InitialDelay: 100 * time.Millisecond, MaxDelay: 5 * time.Second, BackoffFactor: 2.0},
		},
		{
			name: "zero max delay keeps default", maxRetries: 5, initialDelay: 100, maxDelay: 0,
			want: homeassistant.RetryConfig{MaxRetries: 5, InitialDelay: 100 * time.Millisecond, MaxDelay: 5 * time.Second, BackoffFactor: 2.0},
		},
		{
			name: "negative delays keep defaults", maxRetries: 5, initialDelay: -10, maxDelay: -10,
			want: homeassistant.RetryConfig{MaxRetries: 5, InitialDelay: 100 * time.Millisecond, MaxDelay: 5 * time.Second, BackoffFactor: 2.0},
		},
		{
			name: "max delay below initial is clamped up", maxRetries: 5, initialDelay: 2000, maxDelay: 500,
			want: homeassistant.RetryConfig{MaxRetries: 5, InitialDelay: 2 * time.Second, MaxDelay: 2 * time.Second, BackoffFactor: 2.0},
		},
		{
			name: "custom delays", maxRetries: 5, initialDelay: 250, maxDelay: 9000,
			want: homeassistant.RetryConfig{MaxRetries: 5, InitialDelay: 250 * time.Millisecond, MaxDelay: 9 * time.Second, BackoffFactor: 2.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildRetryConfig(tt.maxRetries, tt.initialDelay, tt.maxDelay)
			if got != tt.want {
				t.Errorf("buildRetryConfig(%d, %d, %d) = %+v, want %+v",
					tt.maxRetries, tt.initialDelay, tt.maxDelay, got, tt.want)
			}
		})
	}
}

func TestBuildRESTClientConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.HomeAssistant.REST = config.RESTConfig{
		RateLimit:           20,
		RateBurst:           10,
		MaxRetries:          5,
		RetryInitialDelayMs: 200,
		RetryMaxDelayMs:     3000,
	}

	got := buildRESTClientConfig(cfg, nil)

	if got.RateLimit != 20 {
		t.Errorf("RateLimit = %v, want 20", got.RateLimit)
	}
	if got.RateBurst != 10 {
		t.Errorf("RateBurst = %d, want 10", got.RateBurst)
	}
	want := homeassistant.RetryConfig{
		MaxRetries:    5,
		InitialDelay:  200 * time.Millisecond,
		MaxDelay:      3 * time.Second,
		BackoffFactor: 2.0,
	}
	if got.RetryConfig != want {
		t.Errorf("RetryConfig = %+v, want %+v", got.RetryConfig, want)
	}
	if def := homeassistant.DefaultRESTClientConfig(); got.Timeout != def.Timeout {
		t.Errorf("Timeout = %v, want default %v", got.Timeout, def.Timeout)
	}
}

func TestBuildWSClientConfig_KeepsDefaults(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.HomeAssistant.WebSocket = config.WebSocketConfig{
		MaxRetries:          7,
		RetryInitialDelayMs: 150,
		RetryMaxDelayMs:     2500,
	}

	got := buildWSClientConfig(cfg, nil)
	def := homeassistant.DefaultWSClientConfig()

	wantRetry := homeassistant.RetryConfig{
		MaxRetries:    7,
		InitialDelay:  150 * time.Millisecond,
		MaxDelay:      2500 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	if got.RetryConfig != wantRetry {
		t.Errorf("RetryConfig = %+v, want %+v", got.RetryConfig, wantRetry)
	}

	// A zero WSClientConfig would disable auto-reconnect and pings, so every
	// field outside RetryConfig must still carry its default.
	if got.AutoReconnect != def.AutoReconnect {
		t.Errorf("AutoReconnect = %v, want %v", got.AutoReconnect, def.AutoReconnect)
	}
	if got.PingInterval != def.PingInterval {
		t.Errorf("PingInterval = %v, want %v", got.PingInterval, def.PingInterval)
	}
	if got.PingTimeout != def.PingTimeout {
		t.Errorf("PingTimeout = %v, want %v", got.PingTimeout, def.PingTimeout)
	}
	if got.WriteTimeout != def.WriteTimeout {
		t.Errorf("WriteTimeout = %v, want %v", got.WriteTimeout, def.WriteTimeout)
	}
	if got.RequestTimeout != def.RequestTimeout {
		t.Errorf("RequestTimeout = %v, want %v", got.RequestTimeout, def.RequestTimeout)
	}
	if got.ConnectTimeout != def.ConnectTimeout {
		t.Errorf("ConnectTimeout = %v, want %v", got.ConnectTimeout, def.ConnectTimeout)
	}
	if got.ReconnectConfig != def.ReconnectConfig {
		t.Errorf("ReconnectConfig = %+v, want %+v", got.ReconnectConfig, def.ReconnectConfig)
	}
}

// TestBuildClientConfigs_FromLoadedConfig proves that values set through the
// environment reach the client configs built from a loaded config.
func TestBuildClientConfigs_FromLoadedConfig(t *testing.T) {
	// Every variable the assertions depend on is set explicitly so a developer's
	// real environment cannot leak in.
	t.Setenv("HA_URL", "http://test.local:8123")
	t.Setenv("HA_WS_MAX_RETRIES", "7")
	t.Setenv("HA_WS_RETRY_INITIAL_DELAY_MS", "150")
	t.Setenv("HA_WS_RETRY_MAX_DELAY_MS", "2500")
	t.Setenv("HA_REST_MAX_RETRIES", "5")
	t.Setenv("HA_REST_RETRY_INITIAL_DELAY_MS", "250")
	t.Setenv("HA_REST_RETRY_MAX_DELAY_MS", "5000")
	t.Setenv("HA_REST_RATE_LIMIT", "7")
	t.Setenv("HA_REST_RATE_BURST", "3")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	ws := buildWSClientConfig(cfg, nil)
	wantWS := homeassistant.RetryConfig{
		MaxRetries:    7,
		InitialDelay:  150 * time.Millisecond,
		MaxDelay:      2500 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	if ws.RetryConfig != wantWS {
		t.Errorf("WS RetryConfig = %+v, want %+v", ws.RetryConfig, wantWS)
	}

	rest := buildRESTClientConfig(cfg, nil)
	if rest.RetryConfig.MaxRetries != 5 {
		t.Errorf("REST MaxRetries = %d, want 5", rest.RetryConfig.MaxRetries)
	}
	if rest.RetryConfig.InitialDelay != 250*time.Millisecond {
		t.Errorf("REST InitialDelay = %v, want 250ms", rest.RetryConfig.InitialDelay)
	}
	if rest.RateLimit != 7 {
		t.Errorf("REST RateLimit = %v, want 7", rest.RateLimit)
	}
	if rest.RateBurst != 3 {
		t.Errorf("REST RateBurst = %d, want 3", rest.RateBurst)
	}
}

// TestBuildWSClientConfig_CarriesLogger guards #300: the logger built from
// HA_MCP_LOG_LEVEL must reach the WebSocket client and its retry manager, which
// otherwise fall back to a private logger that ignores the configured level.
func TestBuildWSClientConfig_CarriesLogger(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	logger := logging.New(logging.LevelError)

	if got := buildWSClientConfig(cfg, logger).Logger; got != logger {
		t.Errorf("WS Logger = %p, want the supplied logger %p", got, logger)
	}
	if got := buildWSClientConfig(cfg, nil).Logger; got != nil {
		t.Errorf("WS Logger with nil logger = %p, want nil", got)
	}
}

// TestBuildRESTClientConfig_CarriesLogger is the REST counterpart of
// TestBuildWSClientConfig_CarriesLogger (#300).
func TestBuildRESTClientConfig_CarriesLogger(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	logger := logging.New(logging.LevelError)

	if got := buildRESTClientConfig(cfg, logger).Logger; got != logger {
		t.Errorf("REST Logger = %p, want the supplied logger %p", got, logger)
	}
	if got := buildRESTClientConfig(cfg, nil).Logger; got != nil {
		t.Errorf("REST Logger with nil logger = %p, want nil", got)
	}
}

// TestNewClientPool_CarriesLogger proves the logger reaches the pool's WS and
// REST configs, not just the builders (#300).
func TestNewClientPool_CarriesLogger(t *testing.T) {
	t.Setenv("HA_URL", "http://test.local:8123")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	logger := logging.New(logging.LevelError)
	pool := newClientPool(cfg, logger)
	defer func() {
		if closeErr := pool.Close(); closeErr != nil {
			t.Errorf("pool.Close: %v", closeErr)
		}
	}()

	ws := pool.WSConfig()
	if ws == nil {
		t.Fatal("pool.WSConfig() = nil, want the built WebSocket config")
	}
	if ws.Logger != logger {
		t.Errorf("pool WS Logger = %p, want the supplied logger %p", ws.Logger, logger)
	}

	rest := pool.RESTConfig()
	if rest == nil {
		t.Fatal("pool.RESTConfig() = nil, want the built REST config")
	}
	if rest.Logger != logger {
		t.Errorf("pool REST Logger = %p, want the supplied logger %p", rest.Logger, logger)
	}
}

// TestBuildCacheConfig_FromLoadedConfig guards #299: every HA_CACHE_* value
// must arrive in the cache config handed to the clients.
func TestBuildCacheConfig_FromLoadedConfig(t *testing.T) {
	t.Setenv("HA_URL", "http://test.local:8123")
	t.Setenv("HA_CACHE_ENABLED", "true")
	t.Setenv("HA_CACHE_SERVICES_TTL_MIN", "15")
	t.Setenv("HA_CACHE_CONFIG_TTL_MIN", "16")
	t.Setenv("HA_CACHE_ENTITY_REG_TTL_MIN", "17")
	t.Setenv("HA_CACHE_DEVICE_REG_TTL_MIN", "18")
	t.Setenv("HA_CACHE_AREA_REG_TTL_MIN", "19")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	got := buildCacheConfig(cfg)
	if got == nil {
		t.Fatal("buildCacheConfig() = nil, want a config")
	}
	want := config.CacheConfig{
		Enabled:         true,
		ServicesTTLMin:  15,
		ConfigTTLMin:    16,
		EntityRegTTLMin: 17,
		DeviceRegTTLMin: 18,
		AreaRegTTLMin:   19,
	}
	if *got != want {
		t.Errorf("buildCacheConfig() = %+v, want %+v", *got, want)
	}
	if got == &cfg.HomeAssistant.Cache {
		t.Error("buildCacheConfig() aliases the loaded config, want a copy")
	}
}

func TestBuildCacheConfig_DisabledByDefault(t *testing.T) {
	t.Setenv("HA_URL", "http://test.local:8123")
	t.Setenv("HA_CACHE_ENABLED", "false")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	got := buildCacheConfig(cfg)
	if got == nil {
		t.Fatal("buildCacheConfig() = nil, want a non-nil disabled config")
	}
	if got.Enabled {
		t.Error("Enabled = true, want false")
	}
}

func TestBuildClientOptions_CarriesEverything(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.HomeAssistant.Cache = config.CacheConfig{Enabled: true, ServicesTTLMin: 5}
	logger := logging.New(logging.LevelError)

	got := buildClientOptions(cfg, logger)

	if got.Logger != logger {
		t.Errorf("Logger = %p, want the supplied logger %p", got.Logger, logger)
	}
	if got.WSConfig == nil || got.WSConfig.Logger != logger {
		t.Errorf("WSConfig = %+v, want a config carrying the supplied logger", got.WSConfig)
	}
	if got.RESTConfig == nil || got.RESTConfig.Logger != logger {
		t.Errorf("RESTConfig = %+v, want a config carrying the supplied logger", got.RESTConfig)
	}
	if got.CacheConfig == nil || !got.CacheConfig.Enabled || got.CacheConfig.ServicesTTLMin != 5 {
		t.Errorf("CacheConfig = %+v, want the enabled cache config", got.CacheConfig)
	}
}

// TestNewClientPool_CarriesCacheConfig proves the cache config reaches the pool
// (#299): it used to be passed as a literal nil, so per-request-auth clients
// were never cached.
func TestNewClientPool_CarriesCacheConfig(t *testing.T) {
	t.Setenv("HA_URL", "http://test.local:8123")
	t.Setenv("HA_CACHE_ENABLED", "true")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	pool := newClientPool(cfg, logging.New(logging.LevelError))
	defer func() {
		if closeErr := pool.Close(); closeErr != nil {
			t.Errorf("pool.Close: %v", closeErr)
		}
	}()

	if pool.CacheConfig() == nil || !pool.CacheConfig().Enabled {
		t.Errorf("pool.CacheConfig() = %+v, want an enabled cache config", pool.CacheConfig())
	}
}

// TestNewClientPool_CarriesRetryConfig guards the original #294 bug: the retry
// settings were built from config but never handed to the pool.
func TestNewClientPool_CarriesRetryConfig(t *testing.T) {
	t.Setenv("HA_URL", "http://test.local:8123")
	t.Setenv("HA_WS_MAX_RETRIES", "7")
	t.Setenv("HA_WS_RETRY_INITIAL_DELAY_MS", "150")
	t.Setenv("HA_WS_RETRY_MAX_DELAY_MS", "2500")
	t.Setenv("HA_REST_MAX_RETRIES", "5")
	t.Setenv("HA_REST_RETRY_INITIAL_DELAY_MS", "250")
	t.Setenv("HA_REST_RETRY_MAX_DELAY_MS", "4000")
	t.Setenv("HA_REST_RATE_LIMIT", "7")
	t.Setenv("HA_REST_RATE_BURST", "3")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	pool := newClientPool(cfg, logging.New(logging.LevelError))
	defer func() {
		if closeErr := pool.Close(); closeErr != nil {
			t.Errorf("pool.Close: %v", closeErr)
		}
	}()

	ws := pool.WSConfig()
	if ws == nil {
		t.Fatal("pool.WSConfig() = nil, want the built WebSocket config")
	}
	wantWS := homeassistant.RetryConfig{
		MaxRetries:    7,
		InitialDelay:  150 * time.Millisecond,
		MaxDelay:      2500 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	if ws.RetryConfig != wantWS {
		t.Errorf("pool WS RetryConfig = %+v, want %+v", ws.RetryConfig, wantWS)
	}
	if !ws.AutoReconnect {
		t.Error("pool WS AutoReconnect = false, want true (builder must start from defaults)")
	}

	rest := pool.RESTConfig()
	if rest == nil {
		t.Fatal("pool.RESTConfig() = nil, want the built REST config")
	}
	wantREST := homeassistant.RetryConfig{
		MaxRetries:    5,
		InitialDelay:  250 * time.Millisecond,
		MaxDelay:      4 * time.Second,
		BackoffFactor: 2.0,
	}
	if rest.RetryConfig != wantREST {
		t.Errorf("pool REST RetryConfig = %+v, want %+v", rest.RetryConfig, wantREST)
	}
	if rest.RateLimit != 7 || rest.RateBurst != 3 {
		t.Errorf("pool REST rate = %v/%d, want 7/3", rest.RateLimit, rest.RateBurst)
	}
}
