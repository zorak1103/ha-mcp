package main

import (
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/homeassistant"
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

	got := buildRESTClientConfig(cfg)

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

	got := buildWSClientConfig(cfg)
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
	t.Setenv("HA_WS_RETRY_INITIAL_DELAY_MS", "100")
	t.Setenv("HA_WS_RETRY_MAX_DELAY_MS", "5000")
	t.Setenv("HA_REST_MAX_RETRIES", "5")
	t.Setenv("HA_REST_RETRY_INITIAL_DELAY_MS", "250")
	t.Setenv("HA_REST_RETRY_MAX_DELAY_MS", "5000")
	t.Setenv("HA_REST_RATE_LIMIT", "10")
	t.Setenv("HA_REST_RATE_BURST", "5")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	ws := buildWSClientConfig(cfg)
	if ws.RetryConfig.MaxRetries != 7 {
		t.Errorf("WS MaxRetries = %d, want 7", ws.RetryConfig.MaxRetries)
	}

	rest := buildRESTClientConfig(cfg)
	if rest.RetryConfig.MaxRetries != 5 {
		t.Errorf("REST MaxRetries = %d, want 5", rest.RetryConfig.MaxRetries)
	}
	if rest.RetryConfig.InitialDelay != 250*time.Millisecond {
		t.Errorf("REST InitialDelay = %v, want 250ms", rest.RetryConfig.InitialDelay)
	}
}
