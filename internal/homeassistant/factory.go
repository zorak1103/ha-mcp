// Package homeassistant provides client factories for Home Assistant API.
package homeassistant

import (
	"context"
	"fmt"
	"time"

	"github.com/zorak1103/ha-mcp/internal/config"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

// ClientOptions configures client creation.
type ClientOptions struct {
	// WSConfig provides WebSocket-specific configuration.
	WSConfig *WSClientConfig
	// RESTConfig provides REST API-specific configuration.
	RESTConfig *RESTClientConfig
	// CacheConfig provides caching configuration.
	CacheConfig *config.CacheConfig
	// Logger for logging cache operations.
	Logger *logging.Logger
}

// DefaultClientOptions returns the default client options.
func DefaultClientOptions() ClientOptions {
	defaultWSConfig := DefaultWSClientConfig()
	defaultRESTConfig := DefaultRESTClientConfig()
	return ClientOptions{
		WSConfig:   &defaultWSConfig,
		RESTConfig: &defaultRESTConfig,
	}
}

// NewClientWithOptions creates and connects a Home Assistant WebSocket client with custom options.
// The connection is established before returning; use CloseClient() for cleanup.
// If CacheConfig is provided and enabled, the client will be wrapped with caching capabilities.
func NewClientWithOptions(ctx context.Context, baseURL, token string, opts ClientOptions) (Client, error) {
	client, err := NewConnectedClient(ctx, baseURL, token, opts.WSConfig, opts.RESTConfig)
	if err != nil {
		return nil, err
	}

	// Wrap with caching if enabled
	if opts.CacheConfig != nil && opts.CacheConfig.Enabled {
		return NewCachedClient(client, *opts.CacheConfig, opts.Logger), nil
	}

	return client, nil
}

// NewConnectedClient creates a new hybrid client with both WebSocket and REST capabilities.
// This is the recommended way to create a client for production use.
//
// The returned Client is a HybridClient that uses WebSocket for most operations
// but falls back to REST API for operations not supported via WebSocket
// (e.g., deleting automations, scripts, and scenes).
//
// The provided context is used for the initial connection. For the client's
// lifecycle, use the CloseClient() function to disconnect.
func NewConnectedClient(ctx context.Context, baseURL, token string, wsConfig *WSClientConfig, restConfig *RESTClientConfig) (Client, error) {
	var wsClient *WSClient

	if wsConfig != nil {
		wsClient = NewWSClientWithConfig(baseURL, token, *wsConfig)
	} else {
		wsClient = NewWSClient(baseURL, token)
	}

	// Establish WebSocket connection
	if err := wsClient.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connecting to Home Assistant WebSocket API: %w", err)
	}

	// Create REST client for operations not supported via WebSocket
	var restClient *RESTClient
	if restConfig != nil {
		restClient = NewRESTClientWithConfig(baseURL, token, *restConfig)
	} else {
		restClient = NewRESTClient(baseURL, token)
	}

	// Return hybrid client that combines both
	return NewHybridClientCloser(wsClient, restClient), nil
}

// NewDefaultWSClient creates a connected WebSocket client using default configuration.
// This is the recommended factory function for most use cases.
func NewDefaultWSClient(ctx context.Context, baseURL, token string) (Client, error) {
	return NewConnectedClient(ctx, baseURL, token, nil, nil)
}

// ClientCloser provides a way to close clients that support it.
// All clients implement this interface for graceful shutdown.
type ClientCloser interface {
	Close() error
}

// CloseClient attempts to close a client if it supports the ClientCloser interface.
// Returns nil if the client doesn't support closing.
func CloseClient(c Client) error {
	if closer, ok := c.(ClientCloser); ok {
		return closer.Close()
	}
	return nil
}

// wsClientImplCloser extends wsClientImpl to implement ClientCloser.
// This allows proper cleanup of WebSocket connections.
type wsClientImplCloser struct {
	*wsClientImpl
	wsClient *WSClient // Keep reference to concrete WSClient for Close()
}

// Close closes the underlying WebSocket connection.
func (c *wsClientImplCloser) Close() error {
	return c.wsClient.Close()
}

// NewWSClientImplWithCloser creates a WebSocket Client that also implements ClientCloser.
func NewWSClientImplWithCloser(ws *WSClient) Client {
	return &wsClientImplCloser{
		wsClientImpl: &wsClientImpl{ws: ws},
		wsClient:     ws,
	}
}

// Ensure wsClientImplCloser implements both Client and ClientCloser.
var (
	_ Client       = (*wsClientImplCloser)(nil)
	_ ClientCloser = (*wsClientImplCloser)(nil)
)

// RetryConfigFromSettings creates a RetryConfig from max retries, initial delay (ms), and max delay (ms).
// This is a helper function for building retry configuration from application config settings.
func RetryConfigFromSettings(maxRetries, initialDelayMs, maxDelayMs int) RetryConfig {
	retryCfg := DefaultRetryConfig()

	if maxRetries >= 0 {
		retryCfg.MaxRetries = maxRetries
	}
	if initialDelayMs > 0 {
		retryCfg.InitialDelay = time.Duration(initialDelayMs) * time.Millisecond
	}
	if maxDelayMs > 0 {
		retryCfg.MaxDelay = time.Duration(maxDelayMs) * time.Millisecond
	}

	return retryCfg
}
