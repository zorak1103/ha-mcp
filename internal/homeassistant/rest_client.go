// Package homeassistant provides a REST client for Home Assistant API operations
// that are not supported via WebSocket.
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
		resp, reqErr = c.httpClient.Do(req) //nolint:bodyclose // Body is closed in all error paths below
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
