// Package homeassistant provides client factories for Home Assistant API.
package homeassistant

import (
	"context"
	"fmt"
)

// ClientOptions configures client creation.
type ClientOptions struct {
	// WSConfig provides WebSocket-specific configuration.
	WSConfig *WSClientConfig
}

// DefaultClientOptions returns the default client options.
func DefaultClientOptions() ClientOptions {
	defaultWSConfig := DefaultWSClientConfig()
	return ClientOptions{
		WSConfig: &defaultWSConfig,
	}
}

// NewClientWithOptions creates and connects a Home Assistant WebSocket client with custom options.
// The connection is established before returning; use CloseClient() for cleanup.
func NewClientWithOptions(ctx context.Context, baseURL, token string, opts ClientOptions) (Client, error) {
	return NewConnectedWSClient(ctx, baseURL, token, opts.WSConfig)
}

// NewConnectedWSClient creates a new WebSocket client and establishes a connection.
// This is the recommended way to create a client for production use.
//
// The returned Client is a HybridClient that uses WebSocket for most operations
// but falls back to REST API for operations not supported via WebSocket
// (e.g., deleting automations, scripts, and scenes).
//
// The provided context is used for the initial connection. For the client's
// lifecycle, use the CloseClient() function to disconnect.
func NewConnectedWSClient(ctx context.Context, baseURL, token string, config *WSClientConfig) (Client, error) {
	var wsClient *WSClient

	if config != nil {
		wsClient = NewWSClientWithConfig(baseURL, token, *config)
	} else {
		wsClient = NewWSClient(baseURL, token)
	}

	// Establish WebSocket connection
	if err := wsClient.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connecting to Home Assistant WebSocket API: %w", err)
	}

	// Create REST client for operations not supported via WebSocket
	restClient := NewRESTClient(baseURL, token)

	// Return hybrid client that combines both
	return NewHybridClientCloser(wsClient, restClient), nil
}

// NewDefaultWSClient creates a connected WebSocket client using default configuration.
// This is the recommended factory function for most use cases.
func NewDefaultWSClient(ctx context.Context, baseURL, token string) (Client, error) {
	return NewConnectedWSClient(ctx, baseURL, token, nil)
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
}

// Close closes the underlying WebSocket connection.
func (c *wsClientImplCloser) Close() error {
	return c.ws.Close()
}

// NewWSClientImplWithCloser creates a WebSocket Client that also implements ClientCloser.
func NewWSClientImplWithCloser(ws *WSClient) Client {
	return &wsClientImplCloser{
		wsClientImpl: &wsClientImpl{ws: ws},
	}
}

// Ensure wsClientImplCloser implements both Client and ClientCloser.
var (
	_ Client       = (*wsClientImplCloser)(nil)
	_ ClientCloser = (*wsClientImplCloser)(nil)
)
