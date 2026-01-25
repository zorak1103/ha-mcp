// Package homeassistant provides client factories and management for Home Assistant API.
package homeassistant

import (
	"context"
	"sync"
	"time"
)

// ClientPool manages a pool of Home Assistant clients, one per token.
// It provides thread-safe access to clients and automatic cleanup of idle connections.
type ClientPool struct {
	baseURL string
	clients map[string]*pooledClient
	mu      sync.RWMutex
	maxIdle time.Duration
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// pooledClient wraps a Client with last-used timestamp for idle cleanup.
type pooledClient struct {
	client   Client
	lastUsed time.Time
}

// NewClientPool creates a new client pool for the given Home Assistant URL.
// maxIdle specifies how long an idle connection is kept before cleanup.
func NewClientPool(baseURL string, maxIdle time.Duration) *ClientPool {
	p := &ClientPool{
		baseURL: baseURL,
		clients: make(map[string]*pooledClient),
		maxIdle: maxIdle,
		stopCh:  make(chan struct{}),
	}

	// Start idle cleanup goroutine
	p.wg.Add(1)
	go p.cleanupLoop()

	return p
}

// GetOrCreate returns an existing client for the token or creates a new one.
// The client is connected before being returned.
func (p *ClientPool) GetOrCreate(ctx context.Context, token string) (Client, error) {
	// Use write lock to safely check and potentially modify lastUsed or remove disconnected clients
	p.mu.Lock()
	if pc, exists := p.clients[token]; exists {
		// Check if client is still connected
		if !isClientConnected(pc.client) {
			// Client disconnected, remove from pool and create new one
			_ = CloseClient(pc.client)
			delete(p.clients, token)
			p.mu.Unlock()
			// Fall through to create new client below
		} else {
			pc.lastUsed = time.Now()
			client := pc.client
			p.mu.Unlock()
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
	client, err := NewDefaultWSClient(ctx, p.baseURL, token)
	if err != nil {
		return nil, err
	}

	p.clients[token] = &pooledClient{
		client:   client,
		lastUsed: time.Now(),
	}

	return client, nil
}

// Close closes all pooled clients and stops the cleanup goroutine.
func (p *ClientPool) Close() error {
	close(p.stopCh)
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for token, pc := range p.clients {
		if err := CloseClient(pc.client); err != nil {
			lastErr = err
		}
		delete(p.clients, token)
	}

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
	for token, pc := range p.clients {
		if now.Sub(pc.lastUsed) > p.maxIdle {
			_ = CloseClient(pc.client)
			delete(p.clients, token)
		}
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
