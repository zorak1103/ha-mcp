// Package homeassistant provides a WebSocket client for Home Assistant API.
package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// maxWSMessageSize is the maximum WebSocket message size (16MB).
// Large responses like get_states with many entities require this limit.
const maxWSMessageSize = 16 * 1024 * 1024

// WSClientConfig holds configuration options for WSClient.
type WSClientConfig struct {
	// ReconnectConfig configures automatic reconnection behavior.
	ReconnectConfig ReconnectConfig
	// OnReconnect is called after a successful reconnection.
	OnReconnect OnReconnectFunc
	// OnDisconnect is called when a disconnect is detected.
	OnDisconnect OnDisconnectFunc
	// AutoReconnect enables automatic reconnection on disconnect.
	AutoReconnect bool
	// PingInterval is the interval between health check pings (0 = disabled).
	PingInterval time.Duration
	// PingTimeout is the timeout for ping responses.
	PingTimeout time.Duration
	// WriteTimeout is the timeout for write operations.
	WriteTimeout time.Duration
}

// DefaultWSClientConfig returns the default WSClient configuration.
func DefaultWSClientConfig() WSClientConfig {
	return WSClientConfig{
		ReconnectConfig: DefaultReconnectConfig(),
		AutoReconnect:   true,
		PingInterval:    30 * time.Second,
		PingTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
	}
}

// WSClient manages a WebSocket connection to Home Assistant.
type WSClient struct {
	baseURL   string
	token     string
	conn      *websocket.Conn
	msgID     atomic.Int64
	pendingMu sync.RWMutex
	pending   map[int64]chan *WSResultMessage
	ctx       context.Context
	cancel    context.CancelFunc
	connected atomic.Bool

	// Reconnection fields
	config       WSClientConfig
	reconnectMgr *ReconnectManager
	reconnectMu  sync.Mutex
	reconnecting atomic.Bool

	// Health monitoring fields
	pingCancel context.CancelFunc
	lastPong   atomic.Value // time.Time
}

// NewWSClient creates a new WebSocket client for Home Assistant.
func NewWSClient(baseURL, token string) *WSClient {
	return NewWSClientWithConfig(baseURL, token, DefaultWSClientConfig())
}

// NewWSClientWithConfig creates a new WebSocket client with custom configuration.
func NewWSClientWithConfig(baseURL, token string, config WSClientConfig) *WSClient {
	return &WSClient{
		baseURL:      baseURL,
		token:        token,
		pending:      make(map[int64]chan *WSResultMessage),
		config:       config,
		reconnectMgr: NewReconnectManager(config.ReconnectConfig),
	}
}

// Connect establishes a WebSocket connection to Home Assistant.
func (c *WSClient) Connect(ctx context.Context) error {
	// Build WebSocket URL
	wsURL, err := c.buildWSURL()
	if err != nil {
		return fmt.Errorf("building WebSocket URL: %w", err)
	}

	// Create context for connection lifecycle
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Dial WebSocket
	conn, resp, err := websocket.Dial(c.ctx, wsURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("dialing WebSocket: %w", err)
	}
	c.conn = conn

	c.conn.SetReadLimit(maxWSMessageSize)

	// Perform authentication
	if err := c.authenticate(); err != nil {
		_ = c.conn.Close(websocket.StatusProtocolError, "auth failed")
		return fmt.Errorf("authentication: %w", err)
	}

	// Mark as connected and start read loop
	c.connected.Store(true)

	// Reset reconnection manager on successful connection
	c.reconnectMgr.Reset()

	go c.readLoop()

	// Start health monitoring if enabled
	if c.config.PingInterval > 0 {
		c.startHealthMonitor()
	}

	return nil
}

// startHealthMonitor starts the periodic ping goroutine.
func (c *WSClient) startHealthMonitor() {
	ctx, cancel := context.WithCancel(c.ctx)
	c.pingCancel = cancel
	c.lastPong.Store(time.Now())

	go c.healthLoop(ctx)
}

// healthLoop periodically sends pings to check connection health.
func (c *WSClient) healthLoop(ctx context.Context) {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.connected.Load() {
				continue
			}

			// Check if last pong is too old
			if lastPong, ok := c.lastPong.Load().(time.Time); ok {
				if time.Since(lastPong) > c.config.PingInterval+c.config.PingTimeout {
					// Connection appears dead, trigger reconnect
					if c.config.OnDisconnect != nil {
						c.config.OnDisconnect(errors.New("ping timeout"))
					}
					if c.config.AutoReconnect {
						go func() {
							_ = c.reconnect()
						}()
					}
					return
				}
			}

			// Send ping
			pingCtx, pingCancel := context.WithTimeout(ctx, c.config.PingTimeout)
			err := c.conn.Ping(pingCtx)
			pingCancel()

			if err != nil {
				// Ping failed
				if ctx.Err() != nil {
					return // Context cancelled, clean shutdown
				}
				// Connection might be dead
				if c.config.OnDisconnect != nil {
					c.config.OnDisconnect(fmt.Errorf("ping failed: %w", err))
				}
				if c.config.AutoReconnect {
					go func() {
						_ = c.reconnect()
					}()
				}
				return
			}

			// Ping successful, update last pong time
			c.lastPong.Store(time.Now())
		}
	}
}

// buildWSURL converts the base URL to a WebSocket URL.
func (c *WSClient) buildWSURL() (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}

	// Convert http(s) to ws(s)
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// Already WebSocket scheme
	default:
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	u.Path = "/api/websocket"
	return u.String(), nil
}

// authenticate performs the Home Assistant WebSocket authentication flow.
func (c *WSClient) authenticate() error {
	// Read auth_required message
	_, data, err := c.conn.Read(c.ctx)
	if err != nil {
		return fmt.Errorf("reading auth_required: %w", err)
	}

	msgType, err := ParseMessageType(data)
	if err != nil {
		return fmt.Errorf("parsing auth_required type: %w", err)
	}

	if msgType != "auth_required" {
		return fmt.Errorf("expected auth_required, got %s", msgType)
	}

	// Send auth message
	authMsg := WSAuthMessage{
		Type:        "auth",
		AccessToken: c.token,
	}
	authData, err := json.Marshal(authMsg)
	if err != nil {
		return fmt.Errorf("marshaling auth message: %w", err)
	}

	if err := c.conn.Write(c.ctx, websocket.MessageText, authData); err != nil {
		return fmt.Errorf("sending auth message: %w", err)
	}

	// Read auth response
	_, data, err = c.conn.Read(c.ctx)
	if err != nil {
		return fmt.Errorf("reading auth response: %w", err)
	}

	msgType, err = ParseMessageType(data)
	if err != nil {
		return fmt.Errorf("parsing auth response type: %w", err)
	}

	switch msgType {
	case "auth_ok":
		return nil
	case "auth_invalid":
		var invalid WSAuthInvalid
		if err := json.Unmarshal(data, &invalid); err != nil {
			return errors.New("authentication failed: invalid credentials")
		}
		return fmt.Errorf("authentication failed: %s", invalid.Message)
	default:
		return fmt.Errorf("unexpected auth response type: %s", msgType)
	}
}

// readLoop continuously reads messages from the WebSocket connection.
func (c *WSClient) readLoop() {
	defer func() {
		c.connected.Store(false)
		c.closePendingChannels()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			// Connection closed or error
			if c.ctx.Err() != nil {
				// Context cancelled, clean shutdown
				return
			}

			// Notify disconnect callback
			if c.config.OnDisconnect != nil {
				c.config.OnDisconnect(err)
			}

			// Attempt reconnection if enabled
			if c.config.AutoReconnect {
				if reconnectErr := c.reconnect(); reconnectErr != nil {
					// Reconnection failed, exit read loop
					return
				}
				// Reconnection successful, continue read loop
				continue
			}

			// Auto-reconnect disabled, exit
			return
		}

		// Parse message type
		msgType, err := ParseMessageType(data)
		if err != nil {
			continue // Skip malformed messages
		}

		switch msgType {
		case "result":
			c.handleResultMessage(data)
		case "event", "pong":
			// Events and pong responses are handled by their respective subsystems
		}
	}
}

// reconnect attempts to re-establish the WebSocket connection with exponential backoff.
// It is idempotent: concurrent calls are serialized via the reconnecting atomic flag.
// Pending requests will fail during reconnection; callers should retry on connection errors.
func (c *WSClient) reconnect() error {
	// Prevent concurrent reconnection attempts
	if !c.reconnecting.CompareAndSwap(false, true) {
		// Another goroutine is already reconnecting
		return nil
	}
	defer c.reconnecting.Store(false)

	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	c.connected.Store(false)

	// Close existing connection if still open
	if c.conn != nil {
		_ = c.conn.Close(websocket.StatusGoingAway, "reconnecting")
		c.conn = nil
	}

	for c.reconnectMgr.ShouldReconnect() {
		// Wait for backoff duration
		if err := c.reconnectMgr.WaitForReconnect(c.ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			if errors.Is(err, ErrMaxReconnectAttempts) {
				return err
			}
			// Other errors, continue trying
			continue
		}

		// Attempt to reconnect
		if err := c.connectInternal(); err != nil {
			// Connection failed, will retry
			continue
		}

		// Reconnection successful
		attempts := c.reconnectMgr.GetAttempts()
		c.reconnectMgr.Reset()

		// Restart health monitoring
		if c.config.PingInterval > 0 {
			c.startHealthMonitor()
		}

		// Notify reconnect callback
		if c.config.OnReconnect != nil {
			c.config.OnReconnect(attempts)
		}

		return nil
	}

	return ErrMaxReconnectAttempts
}

// connectInternal performs the actual connection without starting readLoop.
// The caller (readLoop during reconnection) continues to handle message reading.
func (c *WSClient) connectInternal() error {
	// Build WebSocket URL
	wsURL, err := c.buildWSURL()
	if err != nil {
		return fmt.Errorf("building WebSocket URL: %w", err)
	}

	// Dial WebSocket
	conn, resp, err := websocket.Dial(c.ctx, wsURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("dialing WebSocket: %w", err)
	}
	c.conn = conn

	c.conn.SetReadLimit(maxWSMessageSize)

	// Perform authentication
	if err := c.authenticate(); err != nil {
		_ = c.conn.Close(websocket.StatusProtocolError, "auth failed")
		c.conn = nil
		return fmt.Errorf("authentication: %w", err)
	}

	// Mark as connected
	c.connected.Store(true)

	return nil
}

// handleResultMessage routes a result message to the appropriate pending channel.
func (c *WSClient) handleResultMessage(data []byte) {
	msgID, err := ParseMessageID(data)
	if err != nil {
		return
	}

	var result WSResultMessage
	if err := json.Unmarshal(data, &result); err != nil {
		return
	}

	c.pendingMu.RLock()
	ch, ok := c.pending[msgID]
	c.pendingMu.RUnlock()

	if ok {
		select {
		case ch <- &result:
		default:
			// Channel full or closed, skip
		}
	}
}

// closePendingChannels closes all pending response channels on disconnect.
func (c *WSClient) closePendingChannels() {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
}

// SendCommand sends a command to Home Assistant and waits for a response.
func (c *WSClient) SendCommand(ctx context.Context, msgType string, payload map[string]any) (*WSResultMessage, error) {
	if !c.connected.Load() {
		return nil, errors.New("not connected")
	}

	// Generate new message ID
	id := c.msgID.Add(1)

	// Create response channel
	responseChan := make(chan *WSResultMessage, 1)

	// Register pending request
	c.pendingMu.Lock()
	c.pending[id] = responseChan
	c.pendingMu.Unlock()

	// Ensure cleanup
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	// Build and send command
	cmd := &WSCommandWithPayload{
		ID:      id,
		Type:    msgType,
		Payload: payload,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("marshaling command: %w", err)
	}

	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		return nil, fmt.Errorf("sending command: %w", err)
	}

	// Wait for response with timeout
	select {
	case result, ok := <-responseChan:
		if !ok {
			return nil, errors.New("connection closed while waiting for response")
		}
		if !result.Success && result.Error != nil {
			return nil, fmt.Errorf("command failed: %s - %s", result.Error.Code, result.Error.Message)
		}
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendSimpleCommand sends a command without additional payload.
func (c *WSClient) SendSimpleCommand(ctx context.Context, msgType string) (*WSResultMessage, error) {
	return c.SendCommand(ctx, msgType, nil)
}

// Close closes the WebSocket connection and stops reconnection attempts.
func (c *WSClient) Close() error {
	// Stop health monitoring
	if c.pingCancel != nil {
		c.pingCancel()
	}

	// Stop reconnection attempts
	c.reconnectMgr.Stop()

	if c.cancel != nil {
		c.cancel()
	}
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "client closing")
	}
	return nil
}

// SetOnReconnect sets the callback function called after successful reconnection.
func (c *WSClient) SetOnReconnect(fn OnReconnectFunc) {
	c.config.OnReconnect = fn
}

// SetOnDisconnect sets the callback function called when disconnect is detected.
func (c *WSClient) SetOnDisconnect(fn OnDisconnectFunc) {
	c.config.OnDisconnect = fn
}

// SetAutoReconnect enables or disables automatic reconnection.
func (c *WSClient) SetAutoReconnect(enabled bool) {
	c.config.AutoReconnect = enabled
}

// IsConnected returns true if the client is currently connected.
func (c *WSClient) IsConnected() bool {
	return c.connected.Load()
}

// IsHealthy returns true if the connection is connected and has received
// a recent pong response (within PingInterval + PingTimeout).
func (c *WSClient) IsHealthy() bool {
	if !c.connected.Load() {
		return false
	}

	if c.config.PingInterval == 0 {
		// Health monitoring disabled, just check connected status
		return true
	}

	lastPong, ok := c.lastPong.Load().(time.Time)
	if !ok {
		return true // No pong recorded yet, assume healthy
	}

	return time.Since(lastPong) <= c.config.PingInterval+c.config.PingTimeout
}

// GetLastPongTime returns the time of the last successful pong response.
func (c *WSClient) GetLastPongTime() time.Time {
	if t, ok := c.lastPong.Load().(time.Time); ok {
		return t
	}
	return time.Time{}
}

// SetPingInterval sets the interval between health check pings.
// Set to 0 to disable health monitoring.
func (c *WSClient) SetPingInterval(interval time.Duration) {
	c.config.PingInterval = interval
}
