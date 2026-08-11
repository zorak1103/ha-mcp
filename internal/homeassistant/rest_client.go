// Package homeassistant provides a REST client for Home Assistant API operations
// that are not supported via WebSocket.
// coverage-exempt: HTTP client with rate limiting, retry logic, and 25 endpoints require a live HA server
package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/logging"
	"golang.org/x/time/rate"
)

// noResponseBody is the default message when server returns empty response.
const noResponseBody = "no response body"

// configErrorInfo describes error handling for a specific config type.
type configErrorInfo struct {
	typeName   string // e.g., "automation", "script", "scene"
	entityID   string // e.g., the automation ID for not found errors
	actionName string // e.g., "create", "update"
}

// handleConfigError maps HTTP status codes to appropriate APIErrors.
func handleConfigError(statusCode int, body []byte, info configErrorInfo) error {
	bodyStr := string(body)
	if bodyStr == "" {
		bodyStr = noResponseBody
	}

	switch statusCode {
	case http.StatusBadRequest:
		return &APIError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("invalid %s config: %s", info.typeName, bodyStr),
		}
	case http.StatusNotFound:
		return &APIError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("%s not found: %s", info.typeName, info.entityID),
		}
	case http.StatusUnauthorized:
		return &APIError{
			StatusCode: statusCode,
			Message:    "unauthorized: invalid or expired token",
		}
	case http.StatusForbidden:
		return &APIError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("forbidden: insufficient permissions to %s %s", info.actionName, info.typeName),
		}
	default:
		return &APIError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("unexpected status %d: %s", statusCode, bodyStr),
		}
	}
}

// RESTClient provides REST API operations for Home Assistant.
// This client is used for operations that are not supported via WebSocket API,
// such as deleting automations.
type RESTClient struct {
	baseURL      string
	token        string
	httpClient   *http.Client
	limiter      *rate.Limiter
	retryManager *RetryManager
}

// RESTClientConfig configures the REST client.
type RESTClientConfig struct {
	// Timeout for HTTP requests (default: 30 seconds)
	Timeout time.Duration
	// RateLimit is the number of requests per second (0 = unlimited, default: 10)
	RateLimit float64
	// RateBurst is the maximum burst size for rate limiting (default: 5)
	RateBurst int
	// RetryConfig configures retry behavior for transient failures
	RetryConfig RetryConfig
	// Logger for structured logging. If nil, a default logger is used.
	Logger *logging.Logger
}

// DefaultRESTClientConfig returns the default REST client configuration.
func DefaultRESTClientConfig() RESTClientConfig {
	return RESTClientConfig{
		Timeout:     30 * time.Second,
		RateLimit:   10, // 10 requests per second
		RateBurst:   5,  // Allow burst of 5 requests
		RetryConfig: DefaultRetryConfig(),
	}
}

// NewRESTClient creates a new REST client with default configuration.
func NewRESTClient(baseURL, token string) *RESTClient {
	return NewRESTClientWithConfig(baseURL, token, DefaultRESTClientConfig())
}

// NewRESTClientWithConfig creates a new REST client with custom configuration.
func NewRESTClientWithConfig(baseURL, token string, config RESTClientConfig) *RESTClient {
	// Normalize base URL - remove trailing slash and ensure no /api suffix
	baseURL = strings.TrimSuffix(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/api")

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Initialize rate limiter
	var limiter *rate.Limiter
	if config.RateLimit > 0 {
		burst := config.RateBurst
		if burst <= 0 {
			burst = 1
		}
		limiter = rate.NewLimiter(rate.Limit(config.RateLimit), burst)
	}

	// Initialize retry manager
	retryManager := NewRetryManager(config.RetryConfig, config.Logger)

	return &RESTClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		limiter:      limiter,
		retryManager: retryManager,
	}
}

// doRequest executes an HTTP request with rate limiting and retry logic.
// If a rate limiter is configured, it waits for permission before executing.
// Retries are performed for transient failures (5xx, 429, network errors).
func (c *RESTClient) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response

	err := c.retryManager.Retry(ctx, func() error {
		// Wait for rate limiter if configured
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limiter: %w", err)
			}
		}

		// Execute the request
		var reqErr error
		resp, reqErr = c.httpClient.Do(req) //nolint:bodyclose,gosec // Body is closed in all error paths below; G704 SSRF false positive
		if reqErr != nil {
			// Close response body if it exists
			if resp != nil && resp.Body != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				resp = nil
			}
			return reqErr
		}

		// Check for retryable status codes
		if IsRetryableHTTPStatus(resp.StatusCode) {
			statusCode := resp.StatusCode
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			resp = nil // Clear response so caller doesn't use stale data
			return &APIError{StatusCode: statusCode, Message: string(body)}
		}

		return nil
	})

	if err != nil && resp != nil {
		// Ensure response body is closed on error
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return nil, err
	}

	return resp, err
}

// CreateAutomation creates a new automation using the REST API.
// The WebSocket API does not support automation creation reliably.
// Endpoint: POST /api/config/automation/config/{automation_id}
func (c *RESTClient) CreateAutomation(ctx context.Context, config AutomationConfig) error {
	if config.ID == "" {
		return fmt.Errorf("automation ID is required")
	}

	url := fmt.Sprintf("%s/api/config/automation/config/%s", c.baseURL, config.ID)

	bodyBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling automation config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("creating create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing create request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return handleConfigError(resp.StatusCode, body, configErrorInfo{
		typeName:   "automation",
		entityID:   config.ID,
		actionName: "create",
	})
}

// UpdateAutomation updates an existing automation using the REST API.
// The WebSocket API does not support automation updates reliably.
// Endpoint: POST /api/config/automation/config/{automation_id}
func (c *RESTClient) UpdateAutomation(ctx context.Context, automationID string, config AutomationConfig) error {
	// Only set config.ID if empty (preserve correct ID from handler)
	if config.ID == "" {
		config.ID = automationID
	}
	url := fmt.Sprintf("%s/api/config/automation/config/%s", c.baseURL, automationID)

	bodyBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling automation config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("creating update request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing update request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return handleConfigError(resp.StatusCode, body, configErrorInfo{
		typeName:   "automation",
		entityID:   automationID,
		actionName: "update",
	})
}

// DeleteAutomation deletes an automation using the REST API.
// The WebSocket API does not support automation deletion, so we use REST.
// Endpoint: DELETE /api/config/automation/config/{automation_id}
func (c *RESTClient) DeleteAutomation(ctx context.Context, automationID string) error {
	// Build the URL for automation deletion
	url := fmt.Sprintf("%s/api/config/automation/config/%s", c.baseURL, automationID)

	// Create the DELETE request
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing delete request: %w", err)
	}
	defer func() {
		// Drain and close the response body to enable connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// Check response status
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	// Read error response body for better error messages
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if bodyStr == "" {
		bodyStr = noResponseBody
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("automation not found: %s", automationID),
		}
	case http.StatusUnauthorized:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "unauthorized: invalid or expired token",
		}
	case http.StatusForbidden:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "forbidden: insufficient permissions to delete automation",
		}
	default:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, bodyStr),
		}
	}
}

// CreateScript creates a new script using the REST API.
// The WebSocket API does not support script creation reliably.
// Endpoint: POST /api/config/script/config/{script_id}
func (c *RESTClient) CreateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	url := fmt.Sprintf("%s/api/config/script/config/%s", c.baseURL, scriptID)

	bodyBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling script config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("creating create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing create request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return handleConfigError(resp.StatusCode, body, configErrorInfo{
		typeName:   "script",
		entityID:   scriptID,
		actionName: "create",
	})
}

// UpdateScript updates an existing script using the REST API.
// The WebSocket API does not support script updates reliably.
// Endpoint: POST /api/config/script/config/{script_id}
func (c *RESTClient) UpdateScript(ctx context.Context, scriptID string, config ScriptConfig) error {
	url := fmt.Sprintf("%s/api/config/script/config/%s", c.baseURL, scriptID)

	bodyBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling script config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("creating update request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing update request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return handleConfigError(resp.StatusCode, body, configErrorInfo{
		typeName:   "script",
		entityID:   scriptID,
		actionName: "update",
	})
}

// DeleteScript deletes a script using the REST API.
// Endpoint: DELETE /api/config/script/config/{script_id}
func (c *RESTClient) DeleteScript(ctx context.Context, scriptID string) error {
	url := fmt.Sprintf("%s/api/config/script/config/%s", c.baseURL, scriptID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing delete request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if bodyStr == "" {
		bodyStr = noResponseBody
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("script not found: %s", scriptID),
		}
	case http.StatusUnauthorized:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "unauthorized: invalid or expired token",
		}
	case http.StatusForbidden:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "forbidden: insufficient permissions to delete script",
		}
	default:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, bodyStr),
		}
	}
}

// buildSceneData transforms SceneConfig to the format expected by HA REST API.
func buildSceneData(sceneID string, config SceneConfig) map[string]any {
	sceneData := map[string]any{"id": sceneID, "name": config.Name}
	if config.Icon != "" {
		sceneData["icon"] = config.Icon
	}
	if config.Entities != nil {
		entities := make(map[string]any)
		for entityID, state := range config.Entities {
			entityData := make(map[string]any)
			if state.State != "" {
				entityData["state"] = state.State
			}
			for k, v := range state.Attributes {
				entityData[k] = v
			}
			entities[entityID] = entityData
		}
		sceneData["entities"] = entities
	}
	return sceneData
}

// GetScene retrieves the full configuration of a scene by ID.
// Endpoint: GET /api/config/scene/config/{scene_id}
func (c *RESTClient) GetScene(ctx context.Context, sceneID string) (*Scene, error) {
	url := fmt.Sprintf("%s/api/config/scene/config/%s", c.baseURL, sceneID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating get scene request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing get scene request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading get scene response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, handleConfigError(resp.StatusCode, body, configErrorInfo{
			typeName:   "scene",
			entityID:   sceneID,
			actionName: "get",
		})
	}

	var cfg SceneConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("parsing get scene response: %w", err)
	}

	return &Scene{
		EntityID: "scene." + sceneID,
		Config:   &cfg,
	}, nil
}

// CreateScene creates a new scene using the REST API.
// The WebSocket API does not support scene creation reliably.
// Endpoint: POST /api/config/scene/config/{scene_id}
func (c *RESTClient) CreateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	url := fmt.Sprintf("%s/api/config/scene/config/%s", c.baseURL, sceneID)
	sceneData := buildSceneData(sceneID, config)

	bodyBytes, err := json.Marshal(sceneData)
	if err != nil {
		return fmt.Errorf("marshaling scene config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("creating create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing create request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return handleConfigError(resp.StatusCode, body, configErrorInfo{
		typeName:   "scene",
		entityID:   sceneID,
		actionName: "create",
	})
}

// UpdateScene updates an existing scene using the REST API.
// The WebSocket API does not support scene updates reliably.
// Endpoint: POST /api/config/scene/config/{scene_id}
func (c *RESTClient) UpdateScene(ctx context.Context, sceneID string, config SceneConfig) error {
	url := fmt.Sprintf("%s/api/config/scene/config/%s", c.baseURL, sceneID)
	sceneData := buildSceneData(sceneID, config)

	bodyBytes, err := json.Marshal(sceneData)
	if err != nil {
		return fmt.Errorf("marshaling scene config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("creating update request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing update request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return handleConfigError(resp.StatusCode, body, configErrorInfo{
		typeName:   "scene",
		entityID:   sceneID,
		actionName: "update",
	})
}

// DeleteScene deletes a scene using the REST API.
// Endpoint: DELETE /api/config/scene/config/{scene_id}
func (c *RESTClient) DeleteScene(ctx context.Context, sceneID string) error {
	url := fmt.Sprintf("%s/api/config/scene/config/%s", c.baseURL, sceneID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing delete request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if bodyStr == "" {
		bodyStr = noResponseBody
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("scene not found: %s", sceneID),
		}
	case http.StatusUnauthorized:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "unauthorized: invalid or expired token",
		}
	case http.StatusForbidden:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "forbidden: insufficient permissions to delete scene",
		}
	default:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, bodyStr),
		}
	}
}

// GetServices retrieves all available services from Home Assistant.
// Endpoint: GET /api/services
func (c *RESTClient) GetServices(ctx context.Context) ([]Service, error) {
	url := fmt.Sprintf("%s/api/services", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating get services request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing get services request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to get services: %s", string(body)),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading services response: %w", err)
	}

	var services []Service
	if err := json.Unmarshal(body, &services); err != nil {
		return nil, fmt.Errorf("parsing services response: %w", err)
	}

	return services, nil
}

// GetConfig retrieves the Home Assistant system configuration.
// Endpoint: GET /api/config
func (c *RESTClient) GetConfig(ctx context.Context) (*Config, error) {
	url := fmt.Sprintf("%s/api/config", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating get config request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing get config request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to get config: %s", string(body)),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading config response: %w", err)
	}

	var config Config
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("parsing config response: %w", err)
	}

	return &config, nil
}

// RenderTemplate renders a Jinja2 template using Home Assistant state.
// Endpoint: POST /api/template
func (c *RESTClient) RenderTemplate(ctx context.Context, template string) (string, error) {
	url := fmt.Sprintf("%s/api/template", c.baseURL)

	reqBody := TemplateRequest{Template: template}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling template request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("creating template request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return "", fmt.Errorf("executing template request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading template response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to render template: %s", string(body)),
		}
	}

	return string(body), nil
}

// GetLogbook retrieves logbook entries from Home Assistant.
// Endpoint: GET /api/logbook/<timestamp>
// Query params: entity=<entity_id>, end_time=<timestamp>
func (c *RESTClient) GetLogbook(ctx context.Context, startTime, endTime, entityID string) ([]LogbookEntry, error) {
	// Build URL with start_time in path
	url := fmt.Sprintf("%s/api/logbook/%s", c.baseURL, startTime)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating logbook request: %w", err)
	}

	// Add query parameters
	query := req.URL.Query()
	if endTime != "" {
		query.Set("end_time", endTime)
	}
	if entityID != "" {
		query.Set("entity", entityID)
	}
	req.URL.RawQuery = query.Encode()

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing logbook request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to get logbook: %s", string(body)),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading logbook response: %w", err)
	}

	var entries []LogbookEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing logbook response: %w", err)
	}

	return entries, nil
}

// CheckConfig validates the Home Assistant configuration.
// Endpoint: POST /api/config/core/check_config
func (c *RESTClient) CheckConfig(ctx context.Context) (*ConfigCheckResult, error) {
	url := fmt.Sprintf("%s/api/config/core/check_config", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating check config request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing check config request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to check config: %s", string(body)),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading check config response: %w", err)
	}

	var result ConfigCheckResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing check config response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Config Entry Flow Operations
// Used for creating helpers that require the HTTP Config Entry Flow mechanism
// (threshold, derivative, integration, group, template helpers)
// =============================================================================

// InitConfigEntryFlow initializes a config entry flow for the given handler.
// Endpoint: POST /api/config/config_entries/flow
// Returns the flow result with flow_id for subsequent steps.
func (c *RESTClient) InitConfigEntryFlow(ctx context.Context, handler string) (*ConfigEntryFlowResult, error) {
	url := fmt.Sprintf("%s/api/config/config_entries/flow", c.baseURL)

	reqBody := map[string]string{"handler": handler}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling init flow request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("creating init flow request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing init flow request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading init flow response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to init config entry flow for %s: %s", handler, string(body)),
		}
	}

	var result ConfigEntryFlowResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing init flow response: %w", err)
	}

	return &result, nil
}

// SubmitConfigEntryFlowStep submits data for a config entry flow step.
// Endpoint: POST /api/config/config_entries/flow/{flow_id}
// Returns the flow result which may be another form step or create_entry on success.
func (c *RESTClient) SubmitConfigEntryFlowStep(ctx context.Context, flowID string, data map[string]any) (*ConfigEntryFlowResult, error) {
	url := fmt.Sprintf("%s/api/config/config_entries/flow/%s", c.baseURL, flowID)

	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshaling flow step data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("creating flow step request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing flow step request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading flow step response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to submit config entry flow step: %s", string(body)),
		}
	}

	var result ConfigEntryFlowResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing flow step response: %w", err)
	}

	return &result, nil
}

// DeleteConfigEntry deletes a config entry by its entry ID.
// Endpoint: DELETE /api/config/config_entries/entry/{entry_id}
// Home Assistant's async_remove returns {"require_restart": bool} on success: true means the
// integration could not be unloaded cleanly, so its devices/entities remain
// registered until the next Home Assistant restart. A missing/unparseable body
// (older HA versions, a 204 response) degrades to false rather than erroring,
// since the delete itself already succeeded.
func (c *RESTClient) DeleteConfigEntry(ctx context.Context, entryID string) (bool, error) {
	url := fmt.Sprintf("%s/api/config/config_entries/entry/%s", c.baseURL, entryID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return false, fmt.Errorf("creating delete config entry request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return false, fmt.Errorf("executing delete config entry request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		var result struct {
			RequireRestart bool `json:"require_restart"`
		}
		_ = json.Unmarshal(body, &result)
		return result.RequireRestart, nil
	}

	bodyStr := string(body)
	if bodyStr == "" {
		bodyStr = noResponseBody
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return false, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("config entry not found: %s", entryID),
		}
	case http.StatusUnauthorized:
		return false, &APIError{
			StatusCode: resp.StatusCode,
			Message:    "unauthorized: invalid or expired token",
		}
	case http.StatusForbidden:
		return false, &APIError{
			StatusCode: resp.StatusCode,
			Message:    "forbidden: insufficient permissions to delete config entry",
		}
	default:
		return false, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, bodyStr),
		}
	}
}

// Config Entry Options Flow Operations
// Used for reading current option values from config entries
// =============================================================================

// InitConfigEntryOptionsFlow initializes an options flow for the given config entry.
// Endpoint: POST /api/config/config_entries/options/flow
// Returns the options flow result with flow_id for subsequent steps.
func (c *RESTClient) InitConfigEntryOptionsFlow(ctx context.Context, entryID string) (*OptionsFlowResult, error) {
	url := fmt.Sprintf("%s/api/config/config_entries/options/flow", c.baseURL)

	reqBody := map[string]string{"handler": entryID}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling init options flow request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("creating init options flow request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing init options flow request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading init options flow response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to init config entry options flow for %s: %s", entryID, string(body)),
		}
	}

	var result OptionsFlowResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing init options flow response: %w", err)
	}

	return &result, nil
}

// SubmitConfigEntryOptionsFlowStep submits data for an options flow step.
// Endpoint: POST /api/config/config_entries/options/flow/{flow_id}
// Returns the options flow result which may be another form step or create_entry on success.
func (c *RESTClient) SubmitConfigEntryOptionsFlowStep(ctx context.Context, flowID string, data map[string]any) (*OptionsFlowResult, error) {
	url := fmt.Sprintf("%s/api/config/config_entries/options/flow/%s", c.baseURL, flowID)

	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshaling options flow step data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("creating options flow step request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing options flow step request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading options flow step response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to submit options flow step: %s", string(body)),
		}
	}

	var result OptionsFlowResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing options flow step response: %w", err)
	}

	return &result, nil
}

// AbortConfigEntryOptionsFlow aborts an active options flow.
// Endpoint: DELETE /api/config/config_entries/options/flow/{flow_id}
func (c *RESTClient) AbortConfigEntryOptionsFlow(ctx context.Context, flowID string) error {
	url := fmt.Sprintf("%s/api/config/config_entries/options/flow/%s", c.baseURL, flowID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating abort options flow request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing abort options flow request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    fmt.Sprintf("failed to abort options flow: %s", string(body)),
	}
}

// =============================================================================
// Calendar Operations
// =============================================================================

// GetCalendars retrieves all calendars.
// Endpoint: GET /api/calendars
func (c *RESTClient) GetCalendars(ctx context.Context) ([]CalendarEntry, error) {
	url := fmt.Sprintf("%s/api/calendars", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating get calendars request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing get calendars request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to get calendars: %s", string(body)),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading calendars response: %w", err)
	}

	var calendars []CalendarEntry
	if err := json.Unmarshal(body, &calendars); err != nil {
		return nil, fmt.Errorf("parsing calendars response: %w", err)
	}

	return calendars, nil
}

// GetCalendarEvents retrieves events for a specific calendar within a date range.
// Endpoint: GET /api/calendars/{entity_id}?start={start}&end={end}
func (c *RESTClient) GetCalendarEvents(ctx context.Context, entityID, start, end string) ([]CalendarEvent, error) {
	url := fmt.Sprintf("%s/api/calendars/%s?start=%s&end=%s", c.baseURL, entityID, start, end)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating get calendar events request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing get calendar events request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to get calendar events: %s", string(body)),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading calendar events response: %w", err)
	}

	var events []CalendarEvent
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("parsing calendar events response: %w", err)
	}

	return events, nil
}

// =============================================================================
// Camera Operations
// =============================================================================

// GetCameraSnapshot retrieves a camera snapshot as binary image data.
// Endpoint: GET /api/camera_proxy/{entity_id}
// Returns the image bytes, content type, and any error.
func (c *RESTClient) GetCameraSnapshot(ctx context.Context, entityID string) ([]byte, string, error) {
	url := fmt.Sprintf("%s/api/camera_proxy/%s", c.baseURL, entityID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, "", fmt.Errorf("creating get camera snapshot request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("executing get camera snapshot request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("failed to get camera snapshot: %s", string(body)),
		}
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading camera snapshot response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg" // default
	}

	return imageData, contentType, nil
}

// GetSystemLog is not supported via REST API.
func (c *RESTClient) GetSystemLog(_ context.Context) ([]SystemLogEntry, error) {
	return nil, fmt.Errorf("GetSystemLog not supported via REST API, use WebSocket or HybridClient")
}

// ClearSystemLog is not supported via REST API.
func (c *RESTClient) ClearSystemLog(_ context.Context) error {
	return fmt.Errorf("ClearSystemLog not supported via REST API, use WebSocket or HybridClient")
}
