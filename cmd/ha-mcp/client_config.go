package main

import (
	"time"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

// newClientPool creates the per-token client pool with the REST and WebSocket
// client configurations built from cfg. It lives here, not in main.go, so a
// test can prove the built configs actually reach the pool.
func newClientPool(cfg *config.Config, logger *logging.Logger) *homeassistant.ClientPool {
	return homeassistant.NewClientPoolWithFullConfig(
		cfg.HomeAssistant.URL, 30*time.Minute, 100,
		buildRESTClientConfig(cfg, logger), buildCacheConfig(cfg), logger,
		homeassistant.WithWSConfig(buildWSClientConfig(cfg, logger)),
	)
}

// buildClientOptions bundles the WebSocket, REST and cache configurations and
// the app logger into the options for the default (HA_TOKEN) client, so it is
// built the same way as every pooled per-token client.
func buildClientOptions(cfg *config.Config, logger *logging.Logger) homeassistant.ClientOptions {
	return homeassistant.ClientOptions{
		WSConfig:    buildWSClientConfig(cfg, logger),
		RESTConfig:  buildRESTClientConfig(cfg, logger),
		CacheConfig: buildCacheConfig(cfg),
		Logger:      logger,
	}
}

// buildCacheConfig returns a copy of the cache settings in cfg. It is never
// nil: a disabled cache is passed through with Enabled=false, and
// NewClientWithOptions only wraps in a CachedClient when Enabled is true.
func buildCacheConfig(cfg *config.Config) *config.CacheConfig {
	cacheCfg := cfg.HomeAssistant.Cache

	return &cacheCfg
}

// buildRetryConfig converts millisecond retry settings into a RetryConfig.
//
// It starts from homeassistant.DefaultRetryConfig(). maxRetries is taken as
// given (a value <= 0 means no retries). A delay <= 0 keeps the default for
// that delay. If the resulting MaxDelay is smaller than InitialDelay it is
// raised to InitialDelay, so the backoff cap can never undercut its start.
// BackoffFactor always stays at the default.
func buildRetryConfig(maxRetries, initialDelayMs, maxDelayMs int) homeassistant.RetryConfig {
	retry := homeassistant.DefaultRetryConfig()
	retry.MaxRetries = maxRetries

	if initialDelayMs > 0 {
		retry.InitialDelay = time.Duration(initialDelayMs) * time.Millisecond
	}
	if maxDelayMs > 0 {
		retry.MaxDelay = time.Duration(maxDelayMs) * time.Millisecond
	}
	if retry.MaxDelay < retry.InitialDelay {
		retry.MaxDelay = retry.InitialDelay
	}

	return retry
}

// buildRESTClientConfig builds the REST client configuration from cfg.
//
// It starts from the REST defaults (so Timeout stays set) and overrides the
// rate limit, burst and retry settings. A RateLimit or RateBurst of 0 is kept
// as configured and means "unlimited".
//
// logger is handed to the REST client and its retry manager; a nil logger
// leaves Logger nil, so the client falls back to a private logger that ignores
// HA_MCP_LOG_LEVEL.
func buildRESTClientConfig(cfg *config.Config, logger *logging.Logger) *homeassistant.RESTClientConfig {
	rest := cfg.HomeAssistant.REST

	restCfg := homeassistant.DefaultRESTClientConfig()
	restCfg.Logger = logger
	restCfg.RateLimit = rest.RateLimit
	restCfg.RateBurst = rest.RateBurst
	restCfg.RetryConfig = buildRetryConfig(rest.MaxRetries, rest.RetryInitialDelayMs, rest.RetryMaxDelayMs)

	return &restCfg
}

// buildWSClientConfig builds the WebSocket client configuration from cfg.
//
// It starts from the WebSocket defaults and overrides only RetryConfig and
// Logger; a zero WSClientConfig would disable auto-reconnect and health pings.
// A nil logger leaves Logger nil (the client then uses a private logger that
// ignores HA_MCP_LOG_LEVEL).
func buildWSClientConfig(cfg *config.Config, logger *logging.Logger) *homeassistant.WSClientConfig {
	ws := cfg.HomeAssistant.WebSocket

	wsCfg := homeassistant.DefaultWSClientConfig()
	wsCfg.Logger = logger
	wsCfg.RetryConfig = buildRetryConfig(ws.MaxRetries, ws.RetryInitialDelayMs, ws.RetryMaxDelayMs)

	return &wsCfg
}
