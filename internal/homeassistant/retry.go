// Package homeassistant provides retry logic for transient failures.
package homeassistant

import (
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/zorak1103/ha-mcp/internal/logging"
)

// RetryConfig holds configuration for request retry behavior.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries).
	MaxRetries int
	// InitialDelay is the starting delay between retry attempts.
	InitialDelay time.Duration
	// MaxDelay is the maximum delay cap between retry attempts.
	MaxDelay time.Duration
	// BackoffFactor is the multiplier applied to the delay after each attempt.
	BackoffFactor float64
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}

// RetryManager manages retry logic with exponential backoff.
type RetryManager struct {
	config RetryConfig
	logger *logging.Logger
}

// NewRetryManager creates a new RetryManager with the given configuration.
func NewRetryManager(config RetryConfig, logger *logging.Logger) *RetryManager {
	if logger == nil {
		logger = logging.New(logging.LevelInfo)
	}
	return &RetryManager{
		config: config,
		logger: logger,
	}
}

// Retry executes the operation with retry logic for transient failures.
// It will retry up to MaxRetries times for retryable errors, using exponential backoff.
// Non-retryable errors are returned immediately without retry.
func (r *RetryManager) Retry(ctx context.Context, operation func() error) error {
	if r.config.MaxRetries <= 0 {
		// No retries configured, execute once
		return operation()
	}

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Check context before each attempt
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Execute the operation
		err := operation()
		if err == nil {
			// Success
			if attempt > 0 {
				r.logger.Debug("Operation succeeded after retry", "attempt", attempt+1)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err) {
			r.logger.Debug("Non-retryable error, stopping retries", "error", err)
			return err
		}

		// Check if we have retries left
		if attempt >= r.config.MaxRetries {
			r.logger.Debug("Max retries exceeded", "attempts", attempt+1, "error", err)
			break
		}

		// Calculate backoff delay
		delay := r.CalculateBackoff(attempt)
		r.logger.Debug("Retrying operation",
			"attempt", attempt+1,
			"maxRetries", r.config.MaxRetries,
			"delay", delay,
			"error", err,
		)

		// Wait for backoff duration
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// CalculateBackoff computes the delay for a given attempt number (0-indexed).
func (r *RetryManager) CalculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return r.config.InitialDelay
	}

	delay := float64(r.config.InitialDelay) * math.Pow(r.config.BackoffFactor, float64(attempt))
	if delay > float64(r.config.MaxDelay) {
		return r.config.MaxDelay
	}
	return time.Duration(delay)
}

// IsRetryableError determines if an error is transient and should be retried.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for APIError with retryable status code
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return IsRetryableHTTPStatus(apiErr.StatusCode)
	}

	// Check for network errors (timeout, connection refused, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Timeout errors are retryable
		return netErr.Timeout()
	}

	// Check for specific error messages that indicate transient failures
	errMsg := err.Error()
	retryableMessages := []string{
		"connection reset",
		"connection refused",
		"connection closed",
		"broken pipe",
		"EOF",
		"no such host",
		"dial tcp",
		"i/o timeout",
		"TLS handshake",
	}

	for _, msg := range retryableMessages {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(msg)) {
			return true
		}
	}

	return false
}

// IsRetryableHTTPStatus determines if an HTTP status code indicates a retryable error.
func IsRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests: // 429 - Rate limited
		return true
	case http.StatusInternalServerError: // 500
		return true
	case http.StatusBadGateway: // 502
		return true
	case http.StatusServiceUnavailable: // 503
		return true
	case http.StatusGatewayTimeout: // 504
		return true
	default:
		return false
	}
}

// RetryableHTTPError is an error that wraps an HTTP response indicating a retryable failure.
type RetryableHTTPError struct {
	StatusCode int
	Body       string
}

// Error implements the error interface.
func (e *RetryableHTTPError) Error() string {
	return "retryable HTTP error: status " + http.StatusText(e.StatusCode)
}

// Unwrap returns nil as this error does not wrap another error.
func (e *RetryableHTTPError) Unwrap() error {
	return nil
}
