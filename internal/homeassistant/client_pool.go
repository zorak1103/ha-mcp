// Package homeassistant provides client factories and management for Home Assistant API.
package homeassistant

import (
	"context"
	"sync"
	"time"

	"github.com/zorak1103/ha-mcp/internal/logging"
)

// ClientPool manages a pool of Home Assistant clients, one per token.
// It provides thread-safe access to clients and automatic cleanup of idle connections.
type ClientPool struct {
	baseURL    string
	restConfig *RESTClientConfig
	clients    map[string]*pooledClient
	mu         sync.RWMutex
	maxIdle    time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
	logger     *logging.Logger
}

// pooledClient wraps a Client with last-used timestamp for idle cleanup.
type pooledClient struct {
	client   Client
	lastUsed time.Time
}

// NewClientPool creates a new client pool for the given Home Assistant URL.
// maxIdle specifies how long an idle connection is kept before cleanup.
func NewClientPool(baseURL string, maxIdle time.Duration) *ClientPool {
	return NewClientPoolWithConfig(baseURL, maxIdle, nil, nil)
}

// NewClientPoolWithConfig creates a new client pool with REST configuration.
func NewClientPoolWithConfig(baseURL string, maxIdle time.Duration, restConfig *RESTClientConfig, logger *logging.Logger) *ClientPool {
	if logger == nil {
		logger = logging.New(logging.LevelInfo)
	}

	p := &ClientPool{
		baseURL:    baseURL,
		restConfig: restConfig,
		clients:    make(map[string]*pooledClient),
		maxIdle:    maxIdle,
		stopCh:     make(chan struct{}),
		logger:     logger,
	}

	// Start idle cleanup goroutine
	p.wg.Add(1)
	go p.cleanupLoop()

	logger.Debug("Client pool created", "baseURL", baseURL, "maxIdle", maxIdle)
	return p
}

// NewClientPoolWithLogger creates a new client pool with a custom logger.
//
// Deprecated: Use NewClientPoolWithConfig for full configuration control.
func NewClientPoolWithLogger(baseURL string, maxIdle time.Duration, logger *logging.Logger) *ClientPool {
	return NewClientPoolWithConfig(baseURL, maxIdle, nil, logger)
}

// GetOrCreate returns an existing client for the token or creates a new one.
// The client is connected before being returned.
func (p *ClientPool) GetOrCreate(ctx context.Context, token string) (Client, error) {
	tokenHash := hashToken(token)

	// Use write lock to safely check and potentially modify lastUsed or remove disconnected clients
	p.mu.Lock()
	if pc, exists := p.clients[token]; exists {
		// Check if client is still connected
		if !isClientConnected(pc.client) {
			// Client disconnected, remove from pool and create new one
			p.logger.Debug("Removing disconnected client from pool", "tokenHash", tokenHash)
			_ = CloseClient(pc.client)
			delete(p.clients, token)
			p.mu.Unlock()
			// Fall through to create new client below
		} else {
			pc.lastUsed = time.Now()
			client := pc.client
			p.mu.Unlock()
			p.logger.Trace("Returning cached client", "tokenHash", tokenHash)
			return client, nil
		}
	} else {
		p.mu.Unlock()
	}

	// Need write lock to create new client
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have created it)
	if pc, exists := p.clients[token]; exists {
		// Re-check connection status
		if !isClientConnected(pc.client) {
			_ = CloseClient(pc.client)
			delete(p.clients, token)
			// Fall through to create new client
		} else {
			pc.lastUsed = time.Now()
			return pc.client, nil
		}
	}

	// Create new client
	p.logger.Info("Creating new client for pool", "tokenHash", tokenHash)
	client, err := NewConnectedClient(ctx, p.baseURL, token, nil, p.restConfig)
	if err != nil {
		p.logger.Error("Failed to create client", "tokenHash", tokenHash, "error", err)
		return nil, err
	}

	p.clients[token] = &pooledClient{
		client:   client,
		lastUsed: time.Now(),
	}

	p.logger.Debug("Client added to pool", "tokenHash", tokenHash, "poolSize", len(p.clients))
	return client, nil
}

// Close closes all pooled clients and stops the cleanup goroutine.
func (p *ClientPool) Close() error {
	p.logger.Info("Closing client pool", "clientCount", len(p.clients))
	close(p.stopCh)
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for token, pc := range p.clients {
		p.logger.Debug("Closing pooled client", "tokenHash", hashToken(token))
		if err := CloseClient(pc.client); err != nil {
			p.logger.Warn("Error closing client", "tokenHash", hashToken(token), "error", err)
			lastErr = err
		}
		delete(p.clients, token)
	}

	p.logger.Debug("Client pool closed")
	return lastErr
}

// cleanupLoop periodically removes idle clients from the pool.
func (p *ClientPool) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.maxIdle / 2)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.cleanupIdleClients()
		}
	}
}

// cleanupIdleClients removes clients that haven't been used within maxIdle duration.
func (p *ClientPool) cleanupIdleClients() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	removedCount := 0
	for token, pc := range p.clients {
		if now.Sub(pc.lastUsed) > p.maxIdle {
			p.logger.Debug("Removing idle client", "tokenHash", hashToken(token), "idleFor", now.Sub(pc.lastUsed))
			_ = CloseClient(pc.client)
			delete(p.clients, token)
			removedCount++
		}
	}
	if removedCount > 0 {
		p.logger.Info("Idle cleanup completed", "removed", removedCount, "remaining", len(p.clients))
	}
}

// Size returns the current number of clients in the pool.
func (p *ClientPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clients)
}

// isClientConnected checks if the client still has an active connection.
// Uses type assertion to check for IsConnected method (implemented by WSClient).
func isClientConnected(client Client) bool {
	if checker, ok := client.(interface{ IsConnected() bool }); ok {
		return checker.IsConnected()
	}
	// If no connection check available, assume connected (optimistic)
	return true
}

// hashToken creates a short hash of the token for safe logging.
// Never logs the actual token to prevent security issues.
func hashToken(token string) string {
	if len(token) < 8 {
		return "***"
	}
	// Use first 4 and last 4 characters as identifier
	return token[:4] + "..." + token[len(token)-4:]
}
