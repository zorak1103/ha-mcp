// Package mcp implements the Model Context Protocol (MCP) server.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"gitlab.com/zorak1103/ha-mcp/internal/homeassistant"
	"gitlab.com/zorak1103/ha-mcp/internal/logging"
)

const (
	// ServerName is the name reported in MCP initialize response.
	ServerName = "ha-mcp"
	// ServerVersion is the version reported in MCP initialize response.
	ServerVersion = "1.0.0"
	// ProtocolVersion is the MCP protocol version supported.
	ProtocolVersion = "2024-11-05"
)

// Server represents the MCP server.
type Server struct {
	haClient    homeassistant.Client
	registry    *Registry
	httpServer  *http.Server
	port        int
	logger      *logging.Logger
	mu          sync.RWMutex
	initialized bool
}

// NewServer creates a new MCP server instance.
func NewServer(haClient homeassistant.Client, registry *Registry, port int, logger *logging.Logger) *Server {
	if logger == nil {
		logger = logging.New(logging.LevelInfo)
	}
	return &Server{
		haClient: haClient,
		registry: registry,
		port:     port,
		logger:   logger,
	}
}

// Start starts the MCP HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleMCP)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
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

	// TRACE: Log full request body
	s.logger.Trace("Request received", "remote_addr", r.RemoteAddr, "body", string(body))

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

	resp := s.handleRequest(r.Context(), &req)

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

		// TRACE: Log full error details
		if resp.Error.Data != nil {
			s.logger.Trace("Error details", "data", resp.Error.Data)
		}
		return
	}

	// Success response
	s.logger.Info("Request completed", "method", req.Method, "id", formatID(req.ID), "duration", duration)

	// TRACE: Log full response
	if s.logger.IsTraceEnabled() {
		respJSON, err := json.MarshalIndent(resp.Result, "", "  ")
		if err == nil {
			s.logger.Trace("Response result", "result", string(respJSON))
		}
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
func (s *Server) handleRequest(ctx context.Context, req *Request) *Response {
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
		return s.handleToolsCall(ctx, req)
	case MethodResourcesList:
		return s.handleResourcesList(req)
	case MethodResourcesRead:
		return s.handleResourcesRead(ctx, req)
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
		Instructions: "Home Assistant MCP Server - provides tools for interacting with Home Assistant entities, automations, helpers, scripts, and scenes.",
	}

	return NewSuccessResponse(req.ID, result)
}

// handleInitialized handles the initialized notification.
// Per JSON-RPC 2.0, notifications (requests without id) must not receive a response.
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
func (s *Server) handleToolsCall(ctx context.Context, req *Request) *Response {
	var params ToolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, InvalidParams, "invalid tools/call params", err.Error())
	}

	s.logger.Info("Tool call", "tool", params.Name)

	// DEBUG: Log tool arguments summary
	if s.logger.IsDebugEnabled() {
		argSummary := summarizeArguments(params.Arguments)
		s.logger.Debug("Tool arguments", "summary", argSummary)
	}

	// TRACE: Log full arguments
	if s.logger.IsTraceEnabled() {
		argsJSON, err := json.MarshalIndent(params.Arguments, "", "  ")
		if err == nil {
			s.logger.Trace("Tool call arguments", "arguments", string(argsJSON))
		}
	}

	handler, exists := s.registry.GetHandler(params.Name)
	if !exists {
		s.logger.Warn("Tool not found", "tool", params.Name)
		return NewErrorResponse(req.ID, ToolNotFound, fmt.Sprintf("tool not found: %s", params.Name), nil)
	}

	result, err := handler(ctx, s.haClient, params.Arguments)
	if err != nil {
		s.logger.Error("Tool execution failed", "tool", params.Name, "error", err)
		return NewErrorResponse(req.ID, ToolExecutionErr, fmt.Sprintf("tool execution failed: %s", err.Error()), nil)
	}

	s.logger.Debug("Tool call successful", "tool", params.Name)
	return NewSuccessResponse(req.ID, result)
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
func (s *Server) handleResourcesRead(ctx context.Context, req *Request) *Response {
	var params ResourcesReadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, InvalidParams, "invalid resources/read params", err.Error())
	}

	s.logger.Info("Resource read", "uri", params.URI)

	handler, exists := s.registry.GetResourceHandler(params.URI)
	if !exists {
		s.logger.Warn("Resource not found", "uri", params.URI)
		return NewErrorResponse(req.ID, ResourceNotFound, fmt.Sprintf("resource not found: %s", params.URI), nil)
	}

	result, err := handler(ctx, s.haClient, params.URI)
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

	// TRACE: Log full response
	if s.logger.IsTraceEnabled() {
		respJSON, err := json.MarshalIndent(resp, "", "  ")
		if err == nil {
			s.logger.Trace("HTTP Response", "response", string(respJSON))
		}
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

// HAClient returns the Home Assistant client for use by handlers.
func (s *Server) HAClient() homeassistant.Client {
	return s.haClient
}
