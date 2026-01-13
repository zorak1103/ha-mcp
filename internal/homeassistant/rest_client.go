// Package homeassistant provides a REST client for Home Assistant API operations
// that are not supported via WebSocket.
package homeassistant

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// noResponseBody is the default message when server returns empty response.
const noResponseBody = "no response body"

// RESTClient provides REST API operations for Home Assistant.
// This client is used for operations that are not supported via WebSocket API,
// such as deleting automations.
type RESTClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// RESTClientConfig configures the REST client.
type RESTClientConfig struct {
	// Timeout for HTTP requests (default: 30 seconds)
	Timeout time.Duration
}

// DefaultRESTClientConfig returns the default REST client configuration.
func DefaultRESTClientConfig() RESTClientConfig {
	return RESTClientConfig{
		Timeout: 30 * time.Second,
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

	return &RESTClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// DeleteAutomation deletes an automation using the REST API.
// The WebSocket API does not support automation deletion, so we use REST.
// Endpoint: DELETE /api/config/automation/config/{automation_id}
func (c *RESTClient) DeleteAutomation(ctx context.Context, automationID string) error {
	// Build the URL for automation deletion
	url := fmt.Sprintf("%s/api/config/automation/config/%s", c.baseURL, automationID)

	// Create the DELETE request
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := c.httpClient.Do(req)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
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
