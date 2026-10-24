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
		buildRESTClientConfig(cfg), nil, logger,
		homeassistant.WithWSConfig(buildWSClientConfig(cfg)),
	)
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
func buildRESTClientConfig(cfg *config.Config) *homeassistant.RESTClientConfig {
	rest := cfg.HomeAssistant.REST

	restCfg := homeassistant.DefaultRESTClientConfig()
	restCfg.RateLimit = rest.RateLimit
	restCfg.RateBurst = rest.RateBurst
	restCfg.RetryConfig = buildRetryConfig(rest.MaxRetries, rest.RetryInitialDelayMs, rest.RetryMaxDelayMs)

	return &restCfg
}

// buildWSClientConfig builds the WebSocket client configuration from cfg.
//
// It starts from the WebSocket defaults and overrides only RetryConfig; a zero
// WSClientConfig would disable auto-reconnect and health pings.
func buildWSClientConfig(cfg *config.Config) *homeassistant.WSClientConfig {
	ws := cfg.HomeAssistant.WebSocket

	wsCfg := homeassistant.DefaultWSClientConfig()
	wsCfg.RetryConfig = buildRetryConfig(ws.MaxRetries, ws.RetryInitialDelayMs, ws.RetryMaxDelayMs)

	return &wsCfg
}
