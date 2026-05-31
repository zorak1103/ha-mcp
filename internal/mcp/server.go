// Package mcp implements the Model Context Protocol (MCP) server.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

const (
	// ServerName is the name reported in MCP initialize response.
	ServerName = "ha-mcp"
	// ServerVersion is the version reported in MCP initialize response.
	ServerVersion = "1.0.0"
	// ProtocolVersion is the MCP protocol version supported.
	ProtocolVersion = "2024-11-05"
)

// serverInstructions contains the comprehensive server instructions sent to AI clients.
const serverInstructions = `Home Assistant MCP Server - provides tools for managing and querying a Home Assistant instance.

Manageable objects: entities, devices, automations, scripts, scenes, helpers (14 types including input_boolean, input_number, input_text, input_select, input_datetime, input_button, counter, timer, schedule, template sensors, threshold, derivative, integral, group), areas, floors, zones, labels, persons, tags, dashboards, media, config entries, HACS repositories.

Query and analysis: entity state/history/statistics/presence/health, device health, logbook with correlation analysis, entity dependencies, automation coverage, target analysis (triggers/conditions/services), service discovery, system info, date/time, template rendering, config validation.

Format parameter: Most tools support format="natural" (default) and format="json". Use natural format for status checks, diagnostics, and general queries - it is LLM-optimized and token-efficient. Use json format when creating or updating entities (exact field structure needed), processing complex nested data, or extracting specific fields for subsequent API calls.

Useful patterns:
- get_state with entity_ids array to check multiple entities at once
- get_logbook(mode="correlation") to analyze timing and causal relationships across entities
- manage_script(action="get", format="json") to view full script config including sequence
- query_entities(mode="health") to find stale, unavailable, or orphaned entities
- query_devices(mode="health") to find disabled or orphaned devices
- analyze_entity to trace entity usage across automations, scripts, and scenes

Guidance resources are available as skill:// MCP resources (use resources/list then resources/read with a skill://ha-mcp/<slug> URI) and via the get_skill tool (action=list to discover topics, action=read to fetch guidance).`

// HTTP server timeout constants.
const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	httpWriteTimeout      = 30 * time.Second
	httpIdleTimeout       = 60 * time.Second
)

// Rate limiting constants.
const (
	mcpRateLimitPerIP          = 10.0            // sustained req/s per client IP
	mcpRateBurstPerIP          = 30              // burst capacity per client IP
	mcpRateLimiterMaxAge       = 5 * time.Minute // prune stale IP limiter entries
	mcpRateLimiterCleanupEvery = time.Minute     // how often to prune stale entries
	tokenMinLength             = 10              // minimum Bearer token length
)

// ipLimiterEntry holds a per-IP rate limiter and its last-seen time for cleanup.
type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Server represents the MCP server.
type Server struct {
	clientPool    *homeassistant.ClientPool // Pool of HA clients per token
	defaultClient homeassistant.Client      // Optional default client from --ha-token
	registry      *Registry
	httpServer    *http.Server
	port          int
	logger        *logging.Logger
	toolFilter    *ToolFilterEngine // Optional tool filter for access control
	waitConfig    WaitConfig        // Polling config injected into handler contexts
	mu            sync.RWMutex
	initialized   bool
	ipLimiters    map[string]*ipLimiterEntry
	ipLimitersMu  sync.Mutex
}

// NewServer creates a new MCP server instance.
// clientPool is required for per-request token authentication.
// defaultClient is optional - used when no Authorization header is provided.
func NewServer(
	clientPool *homeassistant.ClientPool,
	defaultClient homeassistant.Client,
	registry *Registry,
	port int,
	logger *logging.Logger,
) *Server {
	if logger == nil {
		logger = logging.New(logging.LevelInfo)
	}
	s := &Server{
		clientPool:    clientPool,
		defaultClient: defaultClient,
		registry:      registry,
		port:          port,
		logger:        logger,
		waitConfig:    DefaultWaitConfig(),
		ipLimiters:    make(map[string]*ipLimiterEntry),
	}
	go s.cleanupIPLimiters()
	return s
}

// Start starts the MCP HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleMCP)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           s.rateLimitMiddleware(mux),
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}

	s.logger.Info("MCP server starting", "port", s.port)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	s.logger.Info("MCP server shutting down...")
	return s.httpServer.Shutdown(ctx)
}

// getLimiter returns the per-IP rate limiter, creating one on first access.
func (s *Server) getLimiter(ip string) *rate.Limiter {
	s.ipLimitersMu.Lock()
	defer s.ipLimitersMu.Unlock()
	entry, ok := s.ipLimiters[ip]
	if !ok {
		entry = &ipLimiterEntry{
			limiter: rate.NewLimiter(mcpRateLimitPerIP, mcpRateBurstPerIP),
		}
		s.ipLimiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// cleanupIPLimiters periodically removes stale per-IP limiter entries.
func (s *Server) cleanupIPLimiters() {
	ticker := time.NewTicker(mcpRateLimiterCleanupEvery)
	defer ticker.Stop()
	for range ticker.C {
		s.ipLimitersMu.Lock()
		cutoff := time.Now().Add(-mcpRateLimiterMaxAge)
		for ip, entry := range s.ipLimiters {
			if entry.lastSeen.Before(cutoff) {
				delete(s.ipLimiters, ip)
			}
		}
		s.ipLimitersMu.Unlock()
	}
}

// rateLimitMiddleware enforces per-IP request rate limiting.
// The /health endpoint is exempt to allow liveness probes.
// Clients behind shared NAT share one IP bucket — this is a known trade-off.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !s.getLimiter(host).Allow() {
			s.logger.Warn("Rate limit exceeded", "remoteAddr", r.RemoteAddr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32429,"message":"rate limit exceeded"},"id":null}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleHealth handles health check requests.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug("Health check request", "remote_addr", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// handleMCP handles MCP JSON-RPC requests.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	if r.Method != http.MethodPost {
		s.logger.Warn("Invalid HTTP method", "method", r.Method, "remote_addr", r.RemoteAddr)
		s.writeError(w, nil, InvalidRequest, "method not allowed", nil)
		return
	}

	defer func() { _ = r.Body.Close() }()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("Failed to read request body", "remote_addr", r.RemoteAddr, "error", err)
		s.writeError(w, nil, ParseError, "failed to read request body", nil)
		return
	}

	// TRACE: Log redacted request summary (method + param keys only; never values)
	if s.logger.IsTraceEnabled() {
		s.logger.Trace("Request received", "remote_addr", r.RemoteAddr, "summary", summarizeBody(body), "size", len(body))
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		s.logger.Error("Invalid JSON", "remote_addr", r.RemoteAddr, "error", err)
		s.writeError(w, nil, ParseError, "invalid JSON", err.Error())
		return
	}

	if req.JSONRPC != JSONRPCVersion {
		s.logger.Warn("Invalid JSON-RPC version", "remote_addr", r.RemoteAddr, "version", req.JSONRPC)
		s.writeError(w, req.ID, InvalidRequest, "invalid jsonrpc version", nil)
		return
	}

	// DEBUG: Log method and summary
	s.logger.Debug("Request", "method", req.Method, "id", formatID(req.ID))

	resp := s.handleRequest(r.Context(), &req, r)

	duration := time.Since(startTime)
	s.logResponse(&req, resp, duration)

	s.writeResponse(w, resp)
}

// logResponse logs the response at appropriate levels.
func (s *Server) logResponse(req *Request, resp *Response, duration time.Duration) {
	if resp == nil {
		// Notification - no response
		s.logger.Debug("Notification processed", "method", req.Method, "duration", duration)
		return
	}

	if resp.Error != nil {
		// Error response
		s.logger.Error("Request failed",
			"method", req.Method,
			"id", formatID(req.ID),
			"error_code", resp.Error.Code,
			"error_message", resp.Error.Message,
			"duration", duration)

		// TRACE: Log redacted error detail shape (type only; never values)
		if resp.Error.Data != nil && s.logger.IsTraceEnabled() {
			s.logger.Trace("Error details", "shape", summarizeResult(resp.Error.Data))
		}
		return
	}

	// Success response
	s.logger.Info("Request completed", "method", req.Method, "id", formatID(req.ID), "duration", duration)

	// TRACE: Log redacted response summary (shape + size; never values)
	if s.logger.IsTraceEnabled() {
		respJSON, err := json.Marshal(resp.Result)
		size := 0
		if err == nil {
			size = len(respJSON)
		}
		s.logger.Trace("Response result", "summary", summarizeResult(resp.Result), "size", size)
	}
}

// formatID formats a request ID for logging.
func formatID(id json.RawMessage) string {
	if id == nil {
		return "<notification>"
	}
	return string(id)
}

// handleRequest routes the request to the appropriate handler.
func (s *Server) handleRequest(ctx context.Context, req *Request, r *http.Request) *Response {
	switch req.Method {
	case MethodInitialize:
		return s.handleInitialize(req)
	case MethodInitialized:
		return s.handleInitialized(req)
	case MethodPing:
		return s.handlePing(req)
	case MethodToolsList:
		return s.handleToolsList(req)
	case MethodToolsCall:
		return s.handleToolsCall(ctx, req, r)
	case MethodResourcesList:
		return s.handleResourcesList(req)
	case MethodResourcesRead:
		return s.handleResourcesRead(ctx, req, r)
	default:
		s.logger.Warn("Unknown method requested", "method", req.Method)
		return NewErrorResponse(req.ID, MethodNotFound, fmt.Sprintf("method not found: %s", req.Method), nil)
	}
}

// handleInitialize handles the initialize request.
func (s *Server) handleInitialize(req *Request) *Response {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, InvalidParams, "invalid initialize params", err.Error())
		}
	}

	s.logger.Info("MCP client connected",
		"client_name", params.ClientInfo.Name,
		"client_version", params.ClientInfo.Version,
		"protocol_version", params.ProtocolVersion)

	// DEBUG: Log client capabilities
	s.logger.Debug("Client info",
		"name", params.ClientInfo.Name,
		"version", params.ClientInfo.Version,
		"protocol", params.ProtocolVersion)

	// Add read-only mode notification to instructions if filter is active
	instructions := serverInstructions
	if s.toolFilter != nil && s.toolFilter.IsEnabled() {
		instructions = "⚠️ SERVER IN RESTRICTED MODE: Some tools or actions may be blocked by server policy.\n\n" + serverInstructions
	}

	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
			Resources: &ResourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
		},
		ServerInfo: Implementation{
			Name:    ServerName,
			Version: ServerVersion,
		},
		Instructions: instructions,
	}

	return NewSuccessResponse(req.ID, result)
}

// handleInitialized handles the initialized notification.
// Per JSON-RPC 2.0 spec section 4.1, notifications (requests without id) MUST NOT
// receive a response. However, some MCP clients incorrectly send initialized as a
// request with an id, so we handle both cases for compatibility.
func (s *Server) handleInitialized(req *Request) *Response {
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	s.logger.Info("MCP client initialization complete")

	// Notifications (no id) must not receive a response per JSON-RPC 2.0 spec
	if req.ID == nil {
		return nil
	}

	// If client sent this as a request (with id), respond with empty result
	return NewSuccessResponse(req.ID, struct{}{})
}

// handlePing handles ping requests.
func (s *Server) handlePing(req *Request) *Response {
	s.logger.Debug("Ping received")
	return NewSuccessResponse(req.ID, PingResult{})
}

// handleToolsList handles tools/list requests.
func (s *Server) handleToolsList(req *Request) *Response {
	tools := s.registry.ListTools()
	s.logger.Debug("Listed tools", "count", len(tools))
	result := ToolsListResult{
		Tools: tools,
	}
	return NewSuccessResponse(req.ID, result)
}

// handleToolsCall handles tools/call requests.
func (s *Server) handleToolsCall(ctx context.Context, req *Request, r *http.Request) *Response {
	var params ToolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, InvalidParams, "invalid tools/call params", err.Error())
	}

	s.logger.Info("Tool call", "tool", params.Name)

	// Get HA client for this request
	client, err := s.getClientForRequest(ctx, r)
	if err != nil {
		s.logger.Error("Failed to get HA client", "tool", params.Name, "error", err)
		return NewErrorResponse(req.ID, Unauthorized, err.Error(), nil)
	}

	// DEBUG: Log tool arguments summary
	if s.logger.IsDebugEnabled() {
		argSummary := summarizeArguments(params.Arguments)
		s.logger.Debug("Tool arguments", "summary", argSummary)
	}

	// TRACE: Log redacted tool-call argument summary (keys + size; never values)
	if s.logger.IsTraceEnabled() {
		argsJSON, marshalErr := json.Marshal(params.Arguments)
		size := 0
		if marshalErr == nil {
			size = len(argsJSON)
		}
		s.logger.Trace("Tool call arguments", "summary", summarizeArguments(params.Arguments), "size", size)
	}

	handler, exists := s.registry.GetHandler(params.Name)
	if !exists {
		s.logger.Warn("Tool not found", "tool", params.Name)
		return NewErrorResponse(req.ID, ToolNotFound, fmt.Sprintf("tool not found: %s", params.Name), nil)
	}

	// Check if action is allowed by filter
	if s.toolFilter != nil && !s.toolFilter.IsActionAllowed(params.Name, params.Arguments) {
		s.logger.Warn("Tool action blocked by filter", "tool", params.Name)
		return NewErrorResponse(req.ID, ToolExecutionErr,
			fmt.Sprintf("action blocked by server filter (tool: %s)", params.Name), nil)
	}

	// Inject wait config into context so handlers can access polling settings
	ctx = context.WithValue(ctx, waitContextKey{}, s.waitConfig)

	result, err := handler(ctx, client, params.Arguments)
	if err != nil {
		s.logger.Error("Tool execution failed", "tool", params.Name, "error", err)
		return NewErrorResponse(req.ID, ToolExecutionErr, fmt.Sprintf("tool execution failed: %s", err.Error()), nil)
	}

	// Auto-fallback: if format=json response exceeds size threshold, re-run with format=natural
	if format, _ := params.Arguments["format"].(string); format == formatJSON {
		if size := resultContentSize(result); size > maxJSONResponseBytes {
			s.logger.Info("Response too large for json format, falling back to natural",
				"tool", params.Name, "size_bytes", size)
			naturalArgs := copyArgsWithFormat(params.Arguments, formatNatural)
			if naturalResult, naturalErr := handler(ctx, client, naturalArgs); naturalErr == nil {
				result = prependSizeFallbackNote(naturalResult, size)
			}
		}
	}

	s.logger.Debug("Tool call successful", "tool", params.Name)
	return NewSuccessResponse(req.ID, result)
}

const (
	// maxJSONResponseBytes is the maximum allowed size for format=json responses.
	// Responses exceeding this threshold are automatically re-fetched in format=natural.
	maxJSONResponseBytes = 20 * 1024

	formatJSON    = "json"
	formatNatural = "natural"
)

// resultContentSize returns the total byte count of all text blocks in a result.
func resultContentSize(result *ToolsCallResult) int {
	if result == nil {
		return 0
	}
	total := 0
	for _, block := range result.Content {
		total += len(block.Text)
	}
	return total
}

// copyArgsWithFormat returns a shallow copy of args with the format key overridden.
func copyArgsWithFormat(args map[string]any, format string) map[string]any {
	out := make(map[string]any, len(args))
	maps.Copy(out, args)
	out["format"] = format
	return out
}

// prependSizeFallbackNote prepends a notice to the first text block of result.
func prependSizeFallbackNote(result *ToolsCallResult, originalBytes int) *ToolsCallResult {
	note := fmt.Sprintf("[Note: Response was too large for format=json (%d KB).\nFalling back to format=natural.]\n\n",
		originalBytes/1024)
	newContent := make([]ContentBlock, len(result.Content))
	copy(newContent, result.Content)
	if len(newContent) > 0 && newContent[0].Type == "text" {
		newContent[0] = NewTextContent(note + newContent[0].Text)
	} else {
		newContent = append([]ContentBlock{NewTextContent(note)}, newContent...)
	}
	return &ToolsCallResult{Content: newContent, IsError: result.IsError}
}

// summarizeArguments creates a brief summary of tool arguments for DEBUG logging.
func summarizeArguments(args map[string]any) string {
	if len(args) == 0 {
		return "(no arguments)"
	}

	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}

	if len(keys) <= 3 {
		return fmt.Sprintf("keys=%v", keys)
	}
	return fmt.Sprintf("keys=%v... (%d total)", keys[:3], len(keys))
}

// summarizeBody returns a redacted summary of a JSON-RPC request body for TRACE logging.
// Reports method, id, and top-level param keys — never param values.
func summarizeBody(body []byte) string {
	var envelope struct {
		Method string         `json:"method"`
		ID     any            `json:"id"`
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Sprintf("(unparseable, %d bytes)", len(body))
	}
	if len(envelope.Params) == 0 {
		return fmt.Sprintf("method=%s id=%v (no params)", envelope.Method, envelope.ID)
	}
	keys := make([]string, 0, len(envelope.Params))
	for k := range envelope.Params {
		keys = append(keys, k)
	}
	return fmt.Sprintf("method=%s id=%v param_keys=%v", envelope.Method, envelope.ID, keys)
}

// summarizeResult returns a redacted summary of a result payload for TRACE logging.
// Reports shape and top-level keys for maps — never values.
func summarizeResult(v any) string {
	if v == nil {
		return "(nil)"
	}
	switch typed := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for k := range typed {
			keys = append(keys, k)
		}
		return fmt.Sprintf("map keys=%v", keys)
	case []any:
		return fmt.Sprintf("array len=%d", len(typed))
	case string:
		return fmt.Sprintf("string len=%d", len(typed))
	default:
		return fmt.Sprintf("type=%T", v)
	}
}

// handleResourcesList handles resources/list requests.
func (s *Server) handleResourcesList(req *Request) *Response {
	resources := s.registry.ListResources()
	s.logger.Debug("Listed resources", "count", len(resources))
	result := ResourcesListResult{
		Resources: resources,
	}
	return NewSuccessResponse(req.ID, result)
}

// handleResourcesRead handles resources/read requests.
func (s *Server) handleResourcesRead(ctx context.Context, req *Request, r *http.Request) *Response {
	var params ResourcesReadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, InvalidParams, "invalid resources/read params", err.Error())
	}

	s.logger.Info("Resource read", "uri", params.URI)

	// Get HA client for this request
	client, err := s.getClientForRequest(ctx, r)
	if err != nil {
		s.logger.Error("Failed to get HA client", "uri", params.URI, "error", err)
		return NewErrorResponse(req.ID, Unauthorized, err.Error(), nil)
	}

	handler, exists := s.registry.GetResourceHandler(params.URI)
	if !exists {
		s.logger.Warn("Resource not found", "uri", params.URI)
		return NewErrorResponse(req.ID, ResourceNotFound, fmt.Sprintf("resource not found: %s", params.URI), nil)
	}

	result, err := handler(ctx, client, params.URI)
	if err != nil {
		s.logger.Error("Resource read failed", "uri", params.URI, "error", err)
		return NewErrorResponse(req.ID, InternalError, fmt.Sprintf("resource read failed: %s", err.Error()), nil)
	}

	s.logger.Debug("Resource read successful", "uri", params.URI)
	return NewSuccessResponse(req.ID, result)
}

// writeResponse writes a JSON-RPC response.
// For notifications (nil response), no response is written per JSON-RPC 2.0 spec.
func (s *Server) writeResponse(w http.ResponseWriter, resp *Response) {
	if resp == nil {
		return // Notifications don't get responses
	}
	w.Header().Set("Content-Type", "application/json")

	// TRACE: Log redacted HTTP response summary (shape + size; never values)
	if s.logger.IsTraceEnabled() {
		respJSON, err := json.Marshal(resp)
		size := 0
		if err == nil {
			size = len(respJSON)
		}
		s.logger.Trace("HTTP Response", "summary", summarizeResult(resp.Result), "size", size)
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("Failed to write response", "error", err)
	}
}

// writeError writes a JSON-RPC error response.
func (s *Server) writeError(w http.ResponseWriter, id json.RawMessage, code ErrorCode, message string, data any) {
	resp := NewErrorResponse(id, code, message, data)
	s.writeResponse(w, resp)
}

// IsInitialized returns whether the server has been initialized by a client.
func (s *Server) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

// extractBearerToken extracts the token from an Authorization: Bearer header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// getClientForRequest returns the appropriate HA client for the request.
// It uses the token from the Authorization header if present, otherwise falls back to defaultClient.
func (s *Server) getClientForRequest(ctx context.Context, r *http.Request) (homeassistant.Client, error) {
	token := extractBearerToken(r)

	if token != "" {
		if len(token) < tokenMinLength {
			return nil, fmt.Errorf("authorization token too short")
		}
		// Use token from header
		client, err := s.clientPool.GetOrCreate(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to Home Assistant: %w", err)
		}
		return client, nil
	}

	// Fall back to default client if configured
	if s.defaultClient != nil {
		// Check if client needs to wait for reconnection
		if err := waitForClientConnection(ctx, s.defaultClient); err != nil {
			return nil, fmt.Errorf("home assistant connection failed: %w", err)
		}
		return s.defaultClient, nil
	}

	return nil, errors.New("authorization header with Bearer token required")
}

// waitForClientConnection waits for the client to be connected if it supports connection waiting.
// This is used to handle cases where the client is reconnecting after a disconnect.
func waitForClientConnection(ctx context.Context, client homeassistant.Client) error {
	type connectionWaiter interface {
		IsConnected() bool
		WaitForConnection(ctx context.Context) error
	}

	if waiter, ok := client.(connectionWaiter); ok {
		if !waiter.IsConnected() {
			return waiter.WaitForConnection(ctx)
		}
	}
	return nil
}

// DefaultClient returns the default Home Assistant client (from --ha-token).
// Returns nil if no default client is configured.
func (s *Server) DefaultClient() homeassistant.Client {
	return s.defaultClient
}

// SetToolFilter sets the tool filter for access control.
func (s *Server) SetToolFilter(filter *ToolFilterEngine) {
	s.toolFilter = filter
}

// waitContextKey is the unexported context key type for WaitConfig.
type waitContextKey struct{}

// WaitConfig holds polling configuration injected into handler contexts.
// Handlers use WaitConfigFromContext to retrieve these values.
type WaitConfig struct {
	Timeout      time.Duration
	PollInterval time.Duration
}

// DefaultWaitConfig returns the default wait configuration (5s timeout, 100ms poll).
func DefaultWaitConfig() WaitConfig {
	return WaitConfig{
		Timeout:      5 * time.Second,
		PollInterval: 100 * time.Millisecond,
	}
}

// WaitConfigFromContext extracts the WaitConfig from the context.
// Falls back to DefaultWaitConfig if not set.
func WaitConfigFromContext(ctx context.Context) WaitConfig {
	if wc, ok := ctx.Value(waitContextKey{}).(WaitConfig); ok {
		return wc
	}
	return DefaultWaitConfig()
}

// WithWaitConfig returns a context with the given WaitConfig injected.
// Useful for testing handler functions that call WaitConfigFromContext.
func WithWaitConfig(ctx context.Context, wc WaitConfig) context.Context {
	return context.WithValue(ctx, waitContextKey{}, wc)
}

// SetWaitConfig stores the WaitConfig on the server for injection into handler contexts.
func (s *Server) SetWaitConfig(wc WaitConfig) {
	s.waitConfig = wc
}
