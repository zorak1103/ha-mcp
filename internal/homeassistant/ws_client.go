// Package homeassistant provides a WebSocket client for Home Assistant API.
// coverage-exempt: real WebSocket I/O, connection management, and health loop require a live server
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
	"github.com/zorak1103/ha-mcp/internal/logging"
)

// maxWSMessageSize is the maximum WebSocket message size (16MB).
// Large responses like get_states with many entities require this limit.
const maxWSMessageSize = 16 * 1024 * 1024

// defaultRequestTimeout is the default timeout for pending requests.
// Requests not receiving a response within this time are cleaned up.
const defaultRequestTimeout = 30 * time.Second

// defaultWriteTimeout bounds a single command write when WriteTimeout is unset.
const defaultWriteTimeout = 10 * time.Second

// wsErrCodeIDReuse is HA's error code for a message id that did not exceed
// the previous one on the connection. With ordered sends it signals a bug.
const wsErrCodeIDReuse = "id_reuse"

// errClientClosed is returned by SendCommand and WaitForConnection after
// Close. It is deliberately not retryable.
var errClientClosed = errors.New("websocket client closed")

// WSClientConfig holds configuration options for WSClient.
type WSClientConfig struct {
	// ReconnectConfig configures automatic reconnection behavior.
	ReconnectConfig ReconnectConfig
	// RetryConfig configures retry behavior for transient command failures.
	RetryConfig RetryConfig
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
	// RequestTimeout is the timeout for pending requests. Requests not receiving
	// a response within this time are cleaned up to prevent memory leaks.
	RequestTimeout time.Duration
	// ConnectTimeout is the timeout for establishing connection and completing authentication.
	ConnectTimeout time.Duration
	// Logger for structured logging. If nil, a default logger is used.
	Logger *logging.Logger
}

// DefaultWSClientConfig returns the default WSClient configuration.
func DefaultWSClientConfig() WSClientConfig {
	return WSClientConfig{
		ReconnectConfig: DefaultReconnectConfig(),
		RetryConfig:     DefaultRetryConfig(),
		AutoReconnect:   true,
		PingInterval:    30 * time.Second,
		PingTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		RequestTimeout:  defaultRequestTimeout,
		ConnectTimeout:  defaultRequestTimeout,
	}
}

// WSClient manages a WebSocket connection to Home Assistant.
//
// Each authenticated connection is a wsSession; the client swaps sessions on
// reconnect. The client's lifetime (lifeCtx) is independent of any caller's
// context: only Close ends it.
type WSClient struct {
	baseURL string
	token   string

	// msgID is only incremented while holding sendSem, in the same step as
	// the wire write, so ids reach HA strictly increasing.
	msgID atomic.Int64
	// sendSem (capacity 1) serializes id allocation + write.
	sendSem chan struct{}

	// session is the live connection, nil while disconnected.
	session    atomic.Pointer[wsSession]
	lifeCtx    context.Context
	lifeCancel context.CancelFunc

	// mu guards closed, session install, the reconnect start/exit decision
	// and every wg.Add. It is never held during network I/O or while a user
	// callback runs.
	mu     sync.Mutex
	closed bool
	// wg tracks read loops, health loops and the reconnect loop.
	wg sync.WaitGroup

	// connected and reconnecting are written only under mu, read lock-free.
	connected    atomic.Bool
	reconnecting atomic.Bool

	config       WSClientConfig
	reconnectMgr *ReconnectManager

	// Retry manager for transient failures
	retryManager *RetryManager

	lastPong atomic.Value // time.Time

	logger *logging.Logger
}

// NewWSClient creates a new WebSocket client for Home Assistant.
func NewWSClient(baseURL, token string) *WSClient {
	return NewWSClientWithConfig(baseURL, token, DefaultWSClientConfig())
}

// NewWSClientWithConfig creates a new WebSocket client with custom configuration.
func NewWSClientWithConfig(baseURL, token string, config WSClientConfig) *WSClient {
	logger := config.Logger
	if logger == nil {
		logger = logging.New(logging.LevelInfo)
	}

	lifeCtx, lifeCancel := context.WithCancel(context.Background())
	return &WSClient{
		baseURL:      baseURL,
		token:        token,
		sendSem:      make(chan struct{}, 1),
		lifeCtx:      lifeCtx,
		lifeCancel:   lifeCancel,
		config:       config,
		reconnectMgr: NewReconnectManager(config.ReconnectConfig),
		retryManager: NewRetryManager(config.RetryConfig, logger),
		logger:       logger,
	}
}

// Connect establishes a WebSocket connection to Home Assistant. ctx bounds
// only the dial and authentication; the connection lives until Close.
func (c *WSClient) Connect(ctx context.Context) error {
	if err := c.dialSession(ctx); err != nil {
		return err
	}
	c.reconnectMgr.Reset()
	c.logger.Info("WebSocket connected successfully")
	return nil
}

// dialSession dials and authenticates a new connection (without mu) and
// installs it as the current session.
func (c *WSClient) dialSession(dialCtx context.Context) error {
	conn, err := c.dialAndAuthenticate(dialCtx)
	if err != nil {
		return err
	}
	return c.installSession(conn)
}

// dialAndAuthenticate opens and authenticates a connection, bounded by
// dialCtx plus ConnectTimeout.
func (c *WSClient) dialAndAuthenticate(dialCtx context.Context) (*websocket.Conn, error) {
	wsURL, err := c.buildWSURL()
	if err != nil {
		return nil, fmt.Errorf("building WebSocket URL: %w", err)
	}

	c.logger.Debug("Connecting to WebSocket", "url", sanitizeURL(wsURL))

	connectCtx := dialCtx
	if c.config.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(dialCtx, c.config.ConnectTimeout)
		defer cancel()
	}

	conn, resp, err := websocket.Dial(connectCtx, wsURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		c.logger.Warn("WebSocket dial failed", "error", err)
		return nil, fmt.Errorf("dialing WebSocket: %w", err)
	}
	conn.SetReadLimit(maxWSMessageSize)

	c.logger.Debug("Starting WebSocket authentication")
	if err := c.authenticate(connectCtx, conn); err != nil {
		c.logger.Error("WebSocket authentication failed", "error", err)
		_ = conn.Close(websocket.StatusProtocolError, "auth failed")
		return nil, fmt.Errorf("authentication: %w", err)
	}
	return conn, nil
}

// installSession makes conn the current session and starts its loops. After
// Close it closes conn instead and returns errClientClosed.
func (c *WSClient) installSession(conn *websocket.Conn) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = conn.Close(websocket.StatusNormalClosure, "client closing")
		return errClientClosed
	}
	s := newWSSession(c.lifeCtx, conn)
	c.session.Store(s)
	c.connected.Store(true)
	c.wg.Add(1)
	go c.readLoop(s)
	if c.config.PingInterval > 0 {
		c.lastPong.Store(time.Now())
		c.wg.Add(1)
		go c.healthLoop(s)
	}
	c.mu.Unlock()
	return nil
}

// healthLoop periodically pings the session; it ends with the session.
func (c *WSClient) healthLoop(s *wsSession) {
	defer c.wg.Done()

	c.logger.Trace("Health monitor loop started")
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			c.logger.Debug("Health monitor stopped")
			return
		case <-ticker.C:
			if shouldExit := c.performHealthCheck(s.ctx); shouldExit {
				return
			}
		}
	}
}

// performHealthCheck executes a single health check cycle.
// Returns true if the health loop should exit.
func (c *WSClient) performHealthCheck(ctx context.Context) bool {
	s := c.session.Load()
	if s == nil {
		return false
	}

	if c.isPongTimeout() {
		lastPong, _ := c.lastPong.Load().(time.Time)
		c.logger.Warn("Ping timeout detected", "lastPong", time.Since(lastPong))
		c.handleConnectionFailure(errors.New("ping timeout"))
		return true
	}

	if err := c.sendPing(ctx, s.conn); err != nil {
		if ctx.Err() != nil {
			return true // Session ended, clean shutdown
		}
		c.logger.Error("Ping failed", "error", err)
		c.handleConnectionFailure(fmt.Errorf("ping failed: %w", err))
		return true
	}

	c.logger.Trace("Ping successful")
	c.lastPong.Store(time.Now())
	return false
}

// isPongTimeout checks if the last pong response is too old.
func (c *WSClient) isPongTimeout() bool {
	lastPong, ok := c.lastPong.Load().(time.Time)
	if !ok {
		return false
	}
	return time.Since(lastPong) > c.config.PingInterval+c.config.PingTimeout
}

// sendPing sends a ping on conn with timeout.
func (c *WSClient) sendPing(ctx context.Context, conn *websocket.Conn) error {
	pingCtx, pingCancel := context.WithTimeout(ctx, c.config.PingTimeout)
	defer pingCancel()
	return conn.Ping(pingCtx)
}

// handleConnectionFailure ends the current session after a health failure.
func (c *WSClient) handleConnectionFailure(err error) {
	c.logger.Error("Connection failure detected", "error", err)
	if s := c.session.Load(); s != nil {
		c.endSession(s, err)
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

// authenticate performs the Home Assistant WebSocket authentication flow on conn.
func (c *WSClient) authenticate(ctx context.Context, conn *websocket.Conn) error {
	// Read auth_required message
	_, data, err := conn.Read(ctx)
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
	authData, err := json.Marshal(authMsg) //nolint:gosec // G117: AccessToken is intentionally marshaled for WebSocket authentication
	if err != nil {
		return fmt.Errorf("marshaling auth message: %w", err)
	}

	if writeErr := conn.Write(ctx, websocket.MessageText, authData); writeErr != nil {
		return fmt.Errorf("sending auth message: %w", writeErr)
	}

	// Read auth response
	_, data, err = conn.Read(ctx)
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

// readLoop reads messages from s until its connection fails. It never
// reconnects itself; endSession decides that.
func (c *WSClient) readLoop(s *wsSession) {
	defer c.wg.Done()

	for {
		_, data, err := s.conn.Read(s.ctx)
		if err != nil {
			c.endSession(s, err)
			return
		}

		msgType, err := ParseMessageType(data)
		if err != nil {
			continue // Skip malformed messages
		}
		// Events and pong responses are handled by their respective subsystems.
		if msgType == "result" {
			s.handleResultMessage(data)
		}
	}
}

// endSession is the only path that reacts to a dropped connection (read
// error, write error, ping failure or timeout). s.end returns true exactly
// once per session, so OnDisconnect fires and a reconnect starts at most once
// per drop no matter how many paths observe it.
func (c *WSClient) endSession(s *wsSession, cause error) {
	if !s.end(websocket.StatusGoingAway, "connection lost") {
		return
	}

	c.mu.Lock()
	if !c.session.CompareAndSwap(s, nil) && c.session.Load() != nil {
		// A newer session is already live; this one was stale.
		c.mu.Unlock()
		return
	}
	if c.closed {
		c.connected.Store(false)
		c.mu.Unlock()
		return
	}
	if c.config.AutoReconnect && !c.reconnecting.Load() {
		// reconnecting goes true before connected goes false, so IsUsable
		// never reports a false gap.
		c.reconnecting.Store(true)
		c.connected.Store(false)
		c.wg.Add(1)
		go c.reconnectLoop()
	} else {
		c.connected.Store(false)
	}
	c.mu.Unlock()

	c.logger.Warn("WebSocket connection lost", "error", cause)
	if cb := c.config.OnDisconnect; cb != nil {
		go cb(cause)
	}
}

// reconnectLoop re-establishes the connection with exponential backoff until
// it succeeds with a live session, gives up, or the client is closed.
func (c *WSClient) reconnectLoop() {
	defer c.wg.Done()

	c.logger.Warn("Connection lost, initiating reconnection")
	for {
		dialed := c.reconnectUntilDialed() == nil
		if c.finishReconnect(dialed) {
			return
		}
		c.logger.Warn("Reconnected session dropped immediately, reconnecting again")
	}
}

// reconnectUntilDialed runs backoff + dial attempts until one dial succeeds
// (nil), attempts are exhausted, or the client is closed.
func (c *WSClient) reconnectUntilDialed() error {
	for {
		attempt := c.reconnectMgr.GetAttempts() + 1
		delay := c.reconnectMgr.GetCurrentDelay()
		c.logger.Debug("Waiting for reconnect backoff", "attempt", attempt, "delay", delay)

		if err := c.reconnectMgr.WaitForReconnect(c.lifeCtx); err != nil {
			if errors.Is(err, ErrMaxReconnectAttempts) {
				c.logger.Error("Max reconnection attempts reached, giving up")
				return err
			}
			if c.lifeCtx.Err() != nil {
				c.logger.Debug("Reconnection canceled", "error", err)
				return err
			}
			// Wait interrupted by a Reset/Stop while the client is still
			// alive: try again.
			continue
		}

		c.logger.Debug("Attempting reconnection", "attempt", attempt)
		err := c.dialSession(c.lifeCtx)
		if err == nil {
			return nil
		}
		c.logger.Warn("Reconnection attempt failed", "attempt", attempt, "error", err)
		if errors.Is(err, errClientClosed) {
			return err
		}
	}
}

// finishReconnect makes the reconnect loop's exit decision under mu. It
// returns false when the loop must go on: the dial succeeded but that session
// already died, and its endSession saw reconnecting=true and so started no
// loop of its own.
func (c *WSClient) finishReconnect(dialed bool) bool {
	c.mu.Lock()
	if dialed && !c.closed && c.session.Load() == nil {
		c.mu.Unlock()
		return false
	}
	success := dialed && !c.closed
	attempts := 0
	if success {
		// Reset under mu: a session drop after unlock starts a new loop whose
		// backoff wait a later Reset would otherwise cancel.
		attempts = c.reconnectMgr.GetAttempts()
		c.reconnectMgr.Reset()
	}
	c.reconnecting.Store(false)
	c.mu.Unlock()

	if success {
		c.logger.Info("Successfully reconnected", "totalAttempts", attempts)
		if cb := c.config.OnReconnect; cb != nil {
			go cb(attempts)
		}
	}
	return true
}

// ErrRequestTimeout is returned when a request times out waiting for a response.
var ErrRequestTimeout = errors.New("request timeout: no response received")

// SendCommand sends a command to Home Assistant and waits for a response.
// It retries on transient failures like connection issues, waiting for reconnection if needed.
func (c *WSClient) SendCommand(ctx context.Context, msgType string, payload map[string]any) (*WSResultMessage, error) {
	var result *WSResultMessage

	err := c.retryManager.Retry(ctx, func() error {
		var attemptErr error
		result, attemptErr = c.sendAttempt(ctx, msgType, payload)
		return attemptErr
	})

	return result, err
}

// sendAttempt is one retry attempt: pick the session, write the command in
// id order, and wait for its result.
func (c *WSClient) sendAttempt(ctx context.Context, msgType string, payload map[string]any) (*WSResultMessage, error) {
	s, err := c.currentSession(ctx)
	if err != nil {
		return nil, err
	}
	id, ch, err := c.writeCommand(ctx, s, msgType, payload)
	if err != nil {
		return nil, err
	}
	return c.awaitResult(ctx, s, id, ch)
}

// currentSession returns the live session, waiting for a running reconnect
// if there is none.
func (c *WSClient) currentSession(ctx context.Context) (*wsSession, error) {
	if c.lifeCtx.Err() != nil {
		return nil, errClientClosed
	}
	if s := c.session.Load(); s != nil {
		return s, nil
	}
	if c.config.AutoReconnect && c.reconnecting.Load() {
		if err := c.WaitForConnection(ctx); err != nil {
			return nil, err
		}
		if s := c.session.Load(); s != nil {
			return s, nil
		}
	}
	return nil, newNotSentError(errors.New("not connected"))
}

// acquireSend takes the send slot, giving up when ctx ends or s ends.
func (c *WSClient) acquireSend(ctx context.Context, s *wsSession) error {
	select {
	case c.sendSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return newNotSentError(errConnectionLost)
	}
	if err := ctx.Err(); err != nil {
		// select picks randomly among ready cases; don't send for a caller
		// that already gave up.
		<-c.sendSem
		return err
	}
	return nil
}

// writeCommand allocates the id, registers it and writes the command, all
// under the send slot, so ids hit the wire strictly increasing. A failed
// write ends the session: the connection is unusable after it.
func (c *WSClient) writeCommand(ctx context.Context, s *wsSession, msgType string, payload map[string]any) (int64, <-chan wsResponse, error) {
	if err := c.acquireSend(ctx, s); err != nil {
		return 0, nil, err
	}

	id := c.msgID.Add(1)
	ch, err := s.register(id)
	if err != nil {
		<-c.sendSem
		return 0, nil, err
	}
	data, err := json.Marshal(&WSCommandWithPayload{ID: id, Type: msgType, Payload: payload})
	if err != nil {
		<-c.sendSem
		s.unregister(id)
		return 0, nil, fmt.Errorf("marshaling command: %w", err)
	}
	err = c.writeFrame(s, data)
	<-c.sendSem

	if err != nil {
		s.unregister(id)
		c.endSession(s, err)
		return 0, nil, newNotSentError(fmt.Errorf("sending command: %w", err))
	}
	return id, ch, nil
}

// writeFrame writes data bounded by the session (not the caller's ctx: a
// write abandoned mid-frame makes the library close the shared connection)
// and the write timeout.
func (c *WSClient) writeFrame(s *wsSession, data []byte) error {
	timeout := c.config.WriteTimeout
	if timeout <= 0 {
		timeout = defaultWriteTimeout
	}
	wctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()
	return s.conn.Write(wctx, websocket.MessageText, data)
}

// awaitResult waits for the response to id, expiring it after RequestTimeout.
func (c *WSClient) awaitResult(ctx context.Context, s *wsSession, id int64, ch <-chan wsResponse) (*WSResultMessage, error) {
	timeout := c.config.RequestTimeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	t := time.AfterFunc(timeout, func() { s.expire(id) })
	defer t.Stop()
	defer s.unregister(id)
	return c.waitForResponse(ctx, ch)
}

// waitForResponse waits for a response on ch or context cancellation.
func (c *WSClient) waitForResponse(ctx context.Context, ch <-chan wsResponse) (*WSResultMessage, error) {
	select {
	case resp := <-ch:
		if resp.err != nil {
			return nil, resp.err
		}
		result := resp.msg
		if !result.Success && result.Error != nil {
			if result.Error.Code == wsErrCodeIDReuse {
				c.logger.Error("Home Assistant rejected a reused message id (client bug)",
					"code", result.Error.Code, "message", result.Error.Message)
			}
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

// Close closes the WebSocket connection, stops reconnection attempts and
// waits for every client goroutine to exit. It is idempotent.
func (c *WSClient) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	c.logger.Info("Closing WebSocket connection")
	c.reconnectMgr.Stop()
	c.lifeCancel()
	if s := c.session.Swap(nil); s != nil {
		s.end(websocket.StatusNormalClosure, "client closing")
	}
	c.wg.Wait()

	c.mu.Lock()
	c.connected.Store(false)
	c.mu.Unlock()
	return nil
}

// SetOnReconnect sets the callback function called after successful reconnection.
// The callback runs on its own goroutine.
func (c *WSClient) SetOnReconnect(fn OnReconnectFunc) {
	c.config.OnReconnect = fn
}

// SetOnDisconnect sets the callback function called when disconnect is detected.
// Contract: at most once per connection drop, on its own goroutine with no
// client lock held (so it may call Close). Several paths can observe one drop
// (read loop, failed write, health monitor); all of them go through
// endSession, and wsSession.end lets only the first one through.
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

// IsUsable reports whether the client is connected or reconnecting, i.e.
// whether a command may still succeed without a new client.
func (c *WSClient) IsUsable() bool {
	return c.connected.Load() || c.reconnecting.Load()
}

// WaitForConnection waits until the client is connected or the context is canceled.
// This is useful when a reconnection is in progress and the caller wants to wait for it.
// Returns nil if connected, context error if canceled, errClientClosed after Close,
// or ErrMaxReconnectAttempts if reconnection failed.
func (c *WSClient) WaitForConnection(ctx context.Context) error {
	// Already connected, return immediately
	if c.connected.Load() {
		return nil
	}

	// Poll for connection with exponential backoff
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	maxWait := 10 * time.Second
	deadline := time.Now().Add(maxWait)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.lifeCtx.Done():
			return errClientClosed
		case <-ticker.C:
			if c.connected.Load() {
				return nil
			}
			// If not reconnecting and not connected, reconnection has failed
			if !c.reconnecting.Load() && !c.connected.Load() {
				// Check if we should give up
				if time.Now().After(deadline) {
					return ErrMaxReconnectAttempts
				}
			}
		}
	}
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

// sanitizeURL removes sensitive information from URLs for logging.
// It replaces the token parameter and path segments that look like tokens.
func sanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid-url>"
	}
	// Remove any query parameters that might contain tokens
	u.RawQuery = ""
	// Return just scheme + host + path
	return u.Scheme + "://" + u.Host + u.Path
}
