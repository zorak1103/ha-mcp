// Package mcp implements the Model Context Protocol (MCP) server.
//
//nolint:errcheck,nilnil // Test file - mock implementations return nil,nil and defer body.Close() is acceptable
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/homeassistant"
	"github.com/zorak1103/ha-mcp/internal/logging"
)

// newTestServer creates a test server with a default client (for backwards compatibility tests).
//
//nolint:unparam // port is always 8080 in tests, which is intentional for test consistency
func newTestServer(defaultClient homeassistant.Client, registry *Registry, port int, logger *logging.Logger) *Server {
	// Create a dummy client pool for tests (won't be used when defaultClient is set)
	pool := homeassistant.NewClientPool("http://localhost:8123", 30*time.Minute)
	return NewServer(pool, defaultClient, registry, port, logger)
}

// mockHAClient implements homeassistant.Client for testing.
type mockHAClient struct{}

func (m *mockHAClient) GetStates(_ context.Context) ([]homeassistant.Entity, error) {
	return nil, nil
}

func (m *mockHAClient) GetState(_ context.Context, _ string) (*homeassistant.Entity, error) {
	return nil, nil
}

func (m *mockHAClient) SetState(_ context.Context, _ string, _ homeassistant.StateUpdate) (*homeassistant.Entity, error) {
	return nil, nil
}

func (m *mockHAClient) GetHistory(_ context.Context, _ string, _, _ time.Time) ([][]homeassistant.HistoryEntry, error) {
	return nil, nil
}

func (m *mockHAClient) ListAutomations(_ context.Context) ([]homeassistant.Automation, error) {
	return nil, nil
}

func (m *mockHAClient) GetAutomation(_ context.Context, _ string) (*homeassistant.Automation, error) {
	return nil, nil
}

func (m *mockHAClient) CreateAutomation(_ context.Context, _ homeassistant.AutomationConfig) error {
	return nil
}

func (m *mockHAClient) UpdateAutomation(_ context.Context, _ string, _ homeassistant.AutomationConfig) error {
	return nil
}

func (m *mockHAClient) DeleteAutomation(_ context.Context, _ string) error {
	return nil
}

func (m *mockHAClient) ToggleAutomation(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *mockHAClient) ListHelpers(_ context.Context) ([]homeassistant.Entity, error) {
	return nil, nil
}

func (m *mockHAClient) CreateHelper(_ context.Context, _ homeassistant.HelperConfig) error {
	return nil
}

func (m *mockHAClient) UpdateHelper(_ context.Context, _ string, _ homeassistant.HelperConfig) error {
	return nil
}

func (m *mockHAClient) DeleteHelper(_ context.Context, _ string) error {
	return nil
}

func (m *mockHAClient) SetHelperValue(_ context.Context, _ string, _ any) error {
	return nil
}

func (m *mockHAClient) ListScripts(_ context.Context) ([]homeassistant.Entity, error) {
	return nil, nil
}

func (m *mockHAClient) GetScript(_ context.Context, _ string) (*homeassistant.Script, error) {
	return nil, nil
}

func (m *mockHAClient) CreateScript(_ context.Context, _ string, _ homeassistant.ScriptConfig) error {
	return nil
}

func (m *mockHAClient) UpdateScript(_ context.Context, _ string, _ homeassistant.ScriptConfig) error {
	return nil
}

func (m *mockHAClient) DeleteScript(_ context.Context, _ string) error {
	return nil
}

func (m *mockHAClient) ListScenes(_ context.Context) ([]homeassistant.Entity, error) {
	return nil, nil
}

func (m *mockHAClient) CreateScene(_ context.Context, _ string, _ homeassistant.SceneConfig) error {
	return nil
}

func (m *mockHAClient) UpdateScene(_ context.Context, _ string, _ homeassistant.SceneConfig) error {
	return nil
}

func (m *mockHAClient) DeleteScene(_ context.Context, _ string) error {
	return nil
}

func (m *mockHAClient) CallService(_ context.Context, _, _ string, _ map[string]any) ([]homeassistant.Entity, error) {
	return nil, nil
}

func (m *mockHAClient) GetEntityRegistry(_ context.Context) ([]homeassistant.EntityRegistryEntry, error) {
	return nil, nil
}

func (m *mockHAClient) GetDeviceRegistry(_ context.Context) ([]homeassistant.DeviceRegistryEntry, error) {
	return nil, nil
}

func (m *mockHAClient) GetAreaRegistry(_ context.Context) ([]homeassistant.AreaRegistryEntry, error) {
	return nil, nil
}

func (m *mockHAClient) SignPath(_ context.Context, _ string, _ int) (string, error) {
	return "", nil
}

func (m *mockHAClient) GetCameraStream(_ context.Context, _ string) (*homeassistant.StreamInfo, error) {
	return nil, nil
}

func (m *mockHAClient) BrowseMedia(_ context.Context, _ string) (*homeassistant.MediaBrowseResult, error) {
	return nil, nil
}

func (m *mockHAClient) GetLovelaceConfig(_ context.Context) (map[string]any, error) {
	return nil, nil
}

func (m *mockHAClient) GetStatistics(_ context.Context, _ []string, _ string) ([]homeassistant.StatisticsResult, error) {
	return nil, nil
}

func (m *mockHAClient) GetTriggersForTarget(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
	return nil, nil
}

func (m *mockHAClient) GetConditionsForTarget(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
	return nil, nil
}

func (m *mockHAClient) GetServicesForTarget(_ context.Context, _ homeassistant.Target, _ *bool) ([]string, error) {
	return nil, nil
}

func (m *mockHAClient) ExtractFromTarget(_ context.Context, _ homeassistant.Target, _ *bool) (*homeassistant.ExtractFromTargetResult, error) {
	return nil, nil
}

func (m *mockHAClient) GetScheduleConfig(_ context.Context, _ string) (map[string]any, error) {
	return nil, nil
}

func (m *mockHAClient) GetServices(_ context.Context) ([]homeassistant.Service, error) {
	return nil, nil
}

func (m *mockHAClient) GetConfig(_ context.Context) (*homeassistant.Config, error) {
	return nil, nil
}

func (m *mockHAClient) RenderTemplate(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockHAClient) GetLogbook(_ context.Context, _, _, _ string) ([]homeassistant.LogbookEntry, error) {
	return nil, nil
}

func (m *mockHAClient) CheckConfig(_ context.Context) (*homeassistant.ConfigCheckResult, error) {
	return nil, nil
}

func TestNewServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		defaultClient homeassistant.Client
		registry      *Registry
		port          int
		logger        *logging.Logger
	}{
		{
			name:          "with all parameters",
			defaultClient: &mockHAClient{},
			registry:      NewRegistry(),
			port:          8080,
			logger:        logging.New(logging.LevelOff),
		},
		{
			name:          "with nil logger",
			defaultClient: &mockHAClient{},
			registry:      NewRegistry(),
			port:          9090,
			logger:        nil,
		},
		{
			name:          "with nil default client",
			defaultClient: nil,
			registry:      NewRegistry(),
			port:          8080,
			logger:        logging.New(logging.LevelOff),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pool := homeassistant.NewClientPool("http://localhost:8123", 30*time.Minute)
			defer pool.Close()
			s := NewServer(pool, tt.defaultClient, tt.registry, tt.port, tt.logger)

			if s == nil {
				t.Fatal("NewServer() returned nil")
			}
			if s.defaultClient != tt.defaultClient {
				t.Error("defaultClient not set correctly")
			}
			if s.registry != tt.registry {
				t.Error("registry not set correctly")
			}
			if s.port != tt.port {
				t.Errorf("port = %d, want %d", s.port, tt.port)
			}
			if s.logger == nil {
				t.Error("logger is nil (should have default)")
			}
		})
	}
}

func TestServer_HandleHealth(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if string(body) != `{"status":"ok"}` {
		t.Errorf("body = %q, want %q", string(body), `{"status":"ok"}`)
	}
}

func TestServer_HandleMCP_InvalidMethod(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	tests := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			s.handleMCP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			var jsonResp Response
			if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}

			if jsonResp.Error == nil {
				t.Fatal("Expected error response")
			}
			if jsonResp.Error.Code != InvalidRequest {
				t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, InvalidRequest)
			}
		})
	}
}

func TestServer_HandleMCP_InvalidJSON(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not valid json"))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error == nil {
		t.Fatal("Expected error response")
	}
	if jsonResp.Error.Code != ParseError {
		t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, ParseError)
	}
}

func TestServer_HandleMCP_InvalidJSONRPCVersion(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	reqBody := `{"jsonrpc":"1.0","id":1,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error == nil {
		t.Fatal("Expected error response")
	}
	if jsonResp.Error.Code != InvalidRequest {
		t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, InvalidRequest)
	}
}

func TestServer_HandleInitialize(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}
	paramsJSON, _ := json.Marshal(params)

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodInitialize,
		Params:  paramsJSON,
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error != nil {
		t.Fatalf("Unexpected error: %+v", jsonResp.Error)
	}

	resultJSON, _ := json.Marshal(jsonResp.Result)
	var result InitializeResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if result.ProtocolVersion != ProtocolVersion {
		t.Errorf("ProtocolVersion = %q, want %q", result.ProtocolVersion, ProtocolVersion)
	}
	if result.ServerInfo.Name != ServerName {
		t.Errorf("ServerInfo.Name = %q, want %q", result.ServerInfo.Name, ServerName)
	}
	if result.ServerInfo.Version != ServerVersion {
		t.Errorf("ServerInfo.Version = %q, want %q", result.ServerInfo.Version, ServerVersion)
	}
}

func TestServer_HandleInitialize_InvalidParams(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodInitialize,
		Params:  json.RawMessage(`"invalid"`),
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error == nil {
		t.Fatal("Expected error response")
	}
	if jsonResp.Error.Code != InvalidParams {
		t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, InvalidParams)
	}
}

func TestServer_HandleInitialized(t *testing.T) {
	t.Parallel()

	t.Run("notification without id", func(t *testing.T) {
		t.Parallel()

		s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			Method:  MethodInitialized,
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		// Notifications should not receive a response body
		body, _ := io.ReadAll(resp.Body)
		if len(body) != 0 {
			t.Errorf("Expected empty body for notification, got: %s", body)
		}

		if !s.IsInitialized() {
			t.Error("Server should be initialized after initialized notification")
		}
	})

	t.Run("request with id", func(t *testing.T) {
		t.Parallel()

		s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage(`1`),
			Method:  MethodInitialized,
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		// Should respond when sent as request
		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}

		if jsonResp.Error != nil {
			t.Errorf("Unexpected error: %+v", jsonResp.Error)
		}

		if !s.IsInitialized() {
			t.Error("Server should be initialized")
		}
	})
}

func TestServer_HandlePing(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodPing,
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error != nil {
		t.Errorf("Unexpected error: %+v", jsonResp.Error)
	}
}

func TestServer_HandleToolsList(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterTool(Tool{Name: "tool_a", Description: "Tool A"}, nil)
	registry.RegisterTool(Tool{Name: "tool_b", Description: "Tool B"}, nil)

	s := newTestServer(&mockHAClient{}, registry, 8080, logging.New(logging.LevelOff))

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodToolsList,
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error != nil {
		t.Fatalf("Unexpected error: %+v", jsonResp.Error)
	}

	resultJSON, _ := json.Marshal(jsonResp.Result)
	var result ToolsListResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(result.Tools) != 2 {
		t.Errorf("len(Tools) = %d, want 2", len(result.Tools))
	}
}

func TestServer_HandleToolsCall(t *testing.T) {
	t.Parallel()

	t.Run("successful tool call", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		registry.RegisterTool(
			Tool{Name: "test_tool"},
			func(_ context.Context, _ homeassistant.Client, _ map[string]any) (*ToolsCallResult, error) {
				return &ToolsCallResult{
					Content: []ContentBlock{NewTextContent("success")},
				}, nil
			},
		)

		s := newTestServer(&mockHAClient{}, registry, 8080, logging.New(logging.LevelOff))

		params := ToolsCallParams{
			Name:      "test_tool",
			Arguments: map[string]any{"key": "value"},
		}
		paramsJSON, _ := json.Marshal(params)

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage(`1`),
			Method:  MethodToolsCall,
			Params:  paramsJSON,
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}

		if jsonResp.Error != nil {
			t.Fatalf("Unexpected error: %+v", jsonResp.Error)
		}
	})

	t.Run("tool not found", func(t *testing.T) {
		t.Parallel()

		s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

		params := ToolsCallParams{Name: "nonexistent"}
		paramsJSON, _ := json.Marshal(params)

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage(`1`),
			Method:  MethodToolsCall,
			Params:  paramsJSON,
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}

		if jsonResp.Error == nil {
			t.Fatal("Expected error response")
		}
		if jsonResp.Error.Code != ToolNotFound {
			t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, ToolNotFound)
		}
	})

	t.Run("tool execution error", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		registry.RegisterTool(
			Tool{Name: "failing_tool"},
			func(_ context.Context, _ homeassistant.Client, _ map[string]any) (*ToolsCallResult, error) {
				return nil, errors.New("execution failed")
			},
		)

		s := newTestServer(&mockHAClient{}, registry, 8080, logging.New(logging.LevelOff))

		params := ToolsCallParams{Name: "failing_tool"}
		paramsJSON, _ := json.Marshal(params)

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage(`1`),
			Method:  MethodToolsCall,
			Params:  paramsJSON,
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}

		if jsonResp.Error == nil {
			t.Fatal("Expected error response")
		}
		if jsonResp.Error.Code != ToolExecutionErr {
			t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, ToolExecutionErr)
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		t.Parallel()

		s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage(`1`),
			Method:  MethodToolsCall,
			Params:  json.RawMessage(`"invalid"`),
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}

		if jsonResp.Error == nil {
			t.Fatal("Expected error response")
		}
		if jsonResp.Error.Code != InvalidParams {
			t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, InvalidParams)
		}
	})
}

func TestServer_HandleResourcesList(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterResource(Resource{URI: "test://a", Name: "Resource A"}, nil)
	registry.RegisterResource(Resource{URI: "test://b", Name: "Resource B"}, nil)

	s := newTestServer(&mockHAClient{}, registry, 8080, logging.New(logging.LevelOff))

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodResourcesList,
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error != nil {
		t.Fatalf("Unexpected error: %+v", jsonResp.Error)
	}

	resultJSON, _ := json.Marshal(jsonResp.Result)
	var result ResourcesListResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(result.Resources) != 2 {
		t.Errorf("len(Resources) = %d, want 2", len(result.Resources))
	}
}

func TestServer_HandleResourcesRead(t *testing.T) {
	t.Parallel()

	t.Run("successful resource read", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		registry.RegisterResource(
			Resource{URI: "test://resource", Name: "Test"},
			func(_ context.Context, _ homeassistant.Client, uri string) (*ResourcesReadResult, error) {
				return &ResourcesReadResult{
					Contents: []ResourceContent{{URI: uri, Text: "content"}},
				}, nil
			},
		)

		s := newTestServer(&mockHAClient{}, registry, 8080, logging.New(logging.LevelOff))

		params := ResourcesReadParams{URI: "test://resource"}
		paramsJSON, _ := json.Marshal(params)

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage(`1`),
			Method:  MethodResourcesRead,
			Params:  paramsJSON,
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}

		if jsonResp.Error != nil {
			t.Fatalf("Unexpected error: %+v", jsonResp.Error)
		}
	})

	t.Run("resource not found", func(t *testing.T) {
		t.Parallel()

		s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

		params := ResourcesReadParams{URI: "test://nonexistent"}
		paramsJSON, _ := json.Marshal(params)

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage(`1`),
			Method:  MethodResourcesRead,
			Params:  paramsJSON,
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}

		if jsonResp.Error == nil {
			t.Fatal("Expected error response")
		}
		if jsonResp.Error.Code != ResourceNotFound {
			t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, ResourceNotFound)
		}
	})

	t.Run("resource read error", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		registry.RegisterResource(
			Resource{URI: "test://failing", Name: "Failing"},
			func(_ context.Context, _ homeassistant.Client, _ string) (*ResourcesReadResult, error) {
				return nil, errors.New("read failed")
			},
		)

		s := newTestServer(&mockHAClient{}, registry, 8080, logging.New(logging.LevelOff))

		params := ResourcesReadParams{URI: "test://failing"}
		paramsJSON, _ := json.Marshal(params)

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage(`1`),
			Method:  MethodResourcesRead,
			Params:  paramsJSON,
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}

		if jsonResp.Error == nil {
			t.Fatal("Expected error response")
		}
		if jsonResp.Error.Code != InternalError {
			t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, InternalError)
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		t.Parallel()

		s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

		reqBody := Request{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage(`1`),
			Method:  MethodResourcesRead,
			Params:  json.RawMessage(`"invalid"`),
		}
		reqBodyJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
		w := httptest.NewRecorder()

		s.handleMCP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		var jsonResp Response
		if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("json.Decode() error = %v", err)
		}

		if jsonResp.Error == nil {
			t.Fatal("Expected error response")
		}
		if jsonResp.Error.Code != InvalidParams {
			t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, InvalidParams)
		}
	})
}

func TestServer_UnknownMethod(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  "unknown/method",
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error == nil {
		t.Fatal("Expected error response")
	}
	if jsonResp.Error.Code != MethodNotFound {
		t.Errorf("Error.Code = %d, want %d", jsonResp.Error.Code, MethodNotFound)
	}
}

func TestServer_IsInitialized(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	if s.IsInitialized() {
		t.Error("IsInitialized() = true, want false for new server")
	}

	// Simulate initialization
	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		Method:  MethodInitialized,
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()
	s.handleMCP(w, req)

	if !s.IsInitialized() {
		t.Error("IsInitialized() = false after initialized notification")
	}
}

func TestServer_DefaultClient(t *testing.T) {
	t.Parallel()

	client := &mockHAClient{}
	s := newTestServer(client, NewRegistry(), 8080, logging.New(logging.LevelOff))

	if s.DefaultClient() != client {
		t.Error("DefaultClient() did not return the expected client")
	}

	// Test with nil default client
	sNoDefault := newTestServer(nil, NewRegistry(), 8080, logging.New(logging.LevelOff))
	if sNoDefault.DefaultClient() != nil {
		t.Error("DefaultClient() should return nil when no default client is configured")
	}
}

func TestServer_Shutdown(t *testing.T) {
	t.Parallel()

	t.Run("shutdown nil server", func(t *testing.T) {
		t.Parallel()

		s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

		// httpServer is nil before Start() is called
		err := s.Shutdown(context.Background())
		if err != nil {
			t.Errorf("Shutdown() error = %v, want nil", err)
		}
	})
}

func TestFormatID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   json.RawMessage
		want string
	}{
		{
			name: "nil id",
			id:   nil,
			want: "<notification>",
		},
		{
			name: "numeric id",
			id:   json.RawMessage(`1`),
			want: "1",
		},
		{
			name: "string id",
			id:   json.RawMessage(`"abc-123"`),
			want: `"abc-123"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatID(tt.id)
			if got != tt.want {
				t.Errorf("formatID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummarizeArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "empty arguments",
			args: nil,
			want: "(no arguments)",
		},
		{
			name: "empty map",
			args: map[string]any{},
			want: "(no arguments)",
		},
		{
			name: "few arguments",
			args: map[string]any{"a": 1, "b": 2},
			want: "keys=[a b]",
		},
		{
			name: "many arguments",
			args: map[string]any{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5},
			want: "keys=[a b c]... (5 total)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := summarizeArguments(tt.args)
			// For maps with few arguments, order may vary
			switch {
			case len(tt.args) == 0:
				if got != tt.want {
					t.Errorf("summarizeArguments() = %q, want %q", got, tt.want)
				}
			case len(tt.args) > 3:
				if !strings.Contains(got, "total)") {
					t.Errorf("summarizeArguments() = %q, expected to contain 'total)'", got)
				}
			default:
				if !strings.HasPrefix(got, "keys=[") {
					t.Errorf("summarizeArguments() = %q, expected to start with 'keys=['", got)
				}
			}
		})
	}
}

func TestServer_Constants(t *testing.T) {
	t.Parallel()

	if ServerName != "ha-mcp" {
		t.Errorf("ServerName = %q, want %q", ServerName, "ha-mcp")
	}
	if ServerVersion != "1.0.0" {
		t.Errorf("ServerVersion = %q, want %q", ServerVersion, "1.0.0")
	}
	if ProtocolVersion != "2024-11-05" {
		t.Errorf("ProtocolVersion = %q, want %q", ProtocolVersion, "2024-11-05")
	}
}

func TestServer_HandleInitialize_NilParams(t *testing.T) {
	t.Parallel()

	s := newTestServer(&mockHAClient{}, NewRegistry(), 8080, logging.New(logging.LevelOff))

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodInitialize,
		// Params is nil
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	// Should succeed with nil params (uses default values)
	if jsonResp.Error != nil {
		t.Fatalf("Unexpected error: %+v", jsonResp.Error)
	}
}

func TestExtractBearerToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		authHdr string
		want    string
	}{
		{
			name:    "valid bearer token",
			authHdr: "Bearer my-secret-token",
			want:    "my-secret-token",
		},
		{
			name:    "empty header",
			authHdr: "",
			want:    "",
		},
		{
			name:    "no bearer prefix",
			authHdr: "my-token",
			want:    "",
		},
		{
			name:    "basic auth header",
			authHdr: "Basic dXNlcjpwYXNz",
			want:    "",
		},
		{
			name:    "lowercase bearer",
			authHdr: "bearer my-token",
			want:    "",
		},
		{
			name:    "bearer with space only",
			authHdr: "Bearer ",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.authHdr != "" {
				req.Header.Set("Authorization", tt.authHdr)
			}

			got := extractBearerToken(req)
			if got != tt.want {
				t.Errorf("extractBearerToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServer_ToolsCall_NoAuth(t *testing.T) {
	t.Parallel()

	// Server with no default client - requires auth header
	pool := homeassistant.NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	registry := NewRegistry()
	registry.RegisterTool(
		Tool{Name: "test_tool"},
		func(_ context.Context, _ homeassistant.Client, _ map[string]any) (*ToolsCallResult, error) {
			return &ToolsCallResult{Content: []ContentBlock{NewTextContent("success")}}, nil
		},
	)

	s := NewServer(pool, nil, registry, 8080, logging.New(logging.LevelOff))

	params := ToolsCallParams{Name: "test_tool"}
	paramsJSON, _ := json.Marshal(params)

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodToolsCall,
		Params:  paramsJSON,
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	// Request without Authorization header
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error == nil {
		t.Fatal("Expected Unauthorized error")
	}
	if jsonResp.Error.Code != Unauthorized {
		t.Errorf("Error.Code = %d, want %d (Unauthorized)", jsonResp.Error.Code, Unauthorized)
	}
	if !strings.Contains(jsonResp.Error.Message, "authorization header") {
		t.Errorf("Error.Message = %q, expected to mention authorization header", jsonResp.Error.Message)
	}
}

func TestServer_ToolsCall_WithDefaultClient(t *testing.T) {
	t.Parallel()

	// Server with default client - no auth header required
	registry := NewRegistry()
	registry.RegisterTool(
		Tool{Name: "test_tool"},
		func(_ context.Context, _ homeassistant.Client, _ map[string]any) (*ToolsCallResult, error) {
			return &ToolsCallResult{Content: []ContentBlock{NewTextContent("success")}}, nil
		},
	)

	s := newTestServer(&mockHAClient{}, registry, 8080, logging.New(logging.LevelOff))

	params := ToolsCallParams{Name: "test_tool"}
	paramsJSON, _ := json.Marshal(params)

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodToolsCall,
		Params:  paramsJSON,
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	// Request without Authorization header but with default client configured
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error != nil {
		t.Fatalf("Unexpected error: %+v", jsonResp.Error)
	}
}

func TestServer_ResourcesRead_NoAuth(t *testing.T) {
	t.Parallel()

	// Server with no default client - requires auth header
	pool := homeassistant.NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	registry := NewRegistry()
	registry.RegisterResource(
		Resource{URI: "test://resource", Name: "Test"},
		func(_ context.Context, _ homeassistant.Client, uri string) (*ResourcesReadResult, error) {
			return &ResourcesReadResult{Contents: []ResourceContent{{URI: uri, Text: "content"}}}, nil
		},
	)

	s := NewServer(pool, nil, registry, 8080, logging.New(logging.LevelOff))

	params := ResourcesReadParams{URI: "test://resource"}
	paramsJSON, _ := json.Marshal(params)

	reqBody := Request{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Method:  MethodResourcesRead,
		Params:  paramsJSON,
	}
	reqBodyJSON, _ := json.Marshal(reqBody)

	// Request without Authorization header
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(reqBodyJSON))
	w := httptest.NewRecorder()

	s.handleMCP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("json.Decode() error = %v", err)
	}

	if jsonResp.Error == nil {
		t.Fatal("Expected Unauthorized error")
	}
	if jsonResp.Error.Code != Unauthorized {
		t.Errorf("Error.Code = %d, want %d (Unauthorized)", jsonResp.Error.Code, Unauthorized)
	}
}
