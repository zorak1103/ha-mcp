package homeassistant

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zorak1103/ha-mcp/internal/logging"
)

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", config.MaxRetries)
	}
	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay=100ms, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 5*time.Second {
		t.Errorf("expected MaxDelay=5s, got %v", config.MaxDelay)
	}
	if config.BackoffFactor != 2.0 {
		t.Errorf("expected BackoffFactor=2.0, got %f", config.BackoffFactor)
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		retryable  bool
		name       string
	}{
		{http.StatusOK, false, "200 OK"},
		{http.StatusBadRequest, false, "400 Bad Request"},
		{http.StatusUnauthorized, false, "401 Unauthorized"},
		{http.StatusForbidden, false, "403 Forbidden"},
		{http.StatusNotFound, false, "404 Not Found"},
		{http.StatusTooManyRequests, true, "429 Too Many Requests"},
		{http.StatusInternalServerError, true, "500 Internal Server Error"},
		{http.StatusBadGateway, true, "502 Bad Gateway"},
		{http.StatusServiceUnavailable, true, "503 Service Unavailable"},
		{http.StatusGatewayTimeout, true, "504 Gateway Timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableHTTPStatus(tt.statusCode)
			if result != tt.retryable {
				t.Errorf("IsRetryableHTTPStatus(%d) = %v, want %v", tt.statusCode, result, tt.retryable)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		err       error
		retryable bool
		name      string
	}{
		{nil, false, "nil error"},
		{context.Canceled, false, "context canceled"},
		{context.DeadlineExceeded, false, "context deadline exceeded"},
		{&APIError{StatusCode: http.StatusNotFound, Message: "not found"}, false, "404 API error"},
		{&APIError{StatusCode: http.StatusUnauthorized, Message: "unauthorized"}, false, "401 API error"},
		{&APIError{StatusCode: http.StatusServiceUnavailable, Message: "service unavailable"}, true, "503 API error"},
		{&APIError{StatusCode: http.StatusTooManyRequests, Message: "rate limited"}, true, "429 API error"},
		{&APIError{StatusCode: http.StatusInternalServerError, Message: "server error"}, true, "500 API error"},
		{errors.New("connection reset by peer"), true, "connection reset"},
		{errors.New("connection refused"), true, "connection refused"},
		{errors.New("dial tcp: i/o timeout"), true, "dial timeout"},
		{errors.New("EOF"), true, "EOF"},
		{errors.New("broken pipe"), true, "broken pipe"},
		{errors.New("invalid json"), false, "generic error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, result, tt.retryable)
			}
		})
	}
}

func TestIsRetryableError_NetError(t *testing.T) {
	// Test timeout net.Error
	timeoutErr := &mockNetError{timeout: true}
	if !IsRetryableError(timeoutErr) {
		t.Error("timeout net.Error should be retryable")
	}

	// Test non-timeout net.Error (not retryable via net.Error interface,
	// but may be caught by error message matching)
	permanentErr := &mockNetError{timeout: false}
	// This returns false because no timeout and error message doesn't match retryable patterns
	if IsRetryableError(permanentErr) {
		t.Error("non-timeout net.Error should not be retryable")
	}
}

// mockNetError implements net.Error for testing.
type mockNetError struct {
	timeout bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return false } // Deprecated but required by interface

var _ net.Error = (*mockNetError)(nil)

func TestRetryManager_CalculateBackoff(t *testing.T) {
	config := RetryConfig{
		MaxRetries:    5,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      2 * time.Second,
		BackoffFactor: 2.0,
	}
	rm := NewRetryManager(config, nil)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1600 * time.Millisecond},
		{5, 2 * time.Second},  // Capped at MaxDelay
		{10, 2 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := rm.CalculateBackoff(tt.attempt)
			if result != tt.expected {
				t.Errorf("CalculateBackoff(%d) = %v, want %v", tt.attempt, result, tt.expected)
			}
		})
	}
}

func TestRetryManager_Retry_SuccessOnFirstAttempt(t *testing.T) {
	logger := logging.New(logging.LevelDebug)
	config := RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	rm := NewRetryManager(config, logger)

	var attempts int32
	err := rm.Retry(context.Background(), func() error {
		atomic.AddInt32(&attempts, 1)
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetryManager_Retry_SuccessAfterFailures(t *testing.T) {
	logger := logging.New(logging.LevelDebug)
	config := RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	rm := NewRetryManager(config, logger)

	var attempts int32
	// Fail twice with retryable error, then succeed
	retryableErr := &APIError{StatusCode: http.StatusServiceUnavailable, Message: "temporary failure"}

	err := rm.Retry(context.Background(), func() error {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			return retryableErr
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryManager_Retry_MaxAttemptsExceeded(t *testing.T) {
	logger := logging.New(logging.LevelDebug)
	config := RetryConfig{
		MaxRetries:    2,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	rm := NewRetryManager(config, logger)

	var attempts int32
	retryableErr := &APIError{StatusCode: http.StatusServiceUnavailable, Message: "always fails"}

	err := rm.Retry(context.Background(), func() error {
		atomic.AddInt32(&attempts, 1)
		return retryableErr
	})

	if err == nil {
		t.Error("expected error after max retries")
	}
	// Initial attempt + 2 retries = 3 total attempts
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", attempts)
	}
}

func TestRetryManager_Retry_NonRetryableError(t *testing.T) {
	logger := logging.New(logging.LevelDebug)
	config := RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	rm := NewRetryManager(config, logger)

	var attempts int32
	nonRetryableErr := &APIError{StatusCode: http.StatusNotFound, Message: "not found"}

	err := rm.Retry(context.Background(), func() error {
		atomic.AddInt32(&attempts, 1)
		return nonRetryableErr
	})

	if err == nil {
		t.Error("expected error")
	}
	// Should only attempt once for non-retryable error
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retries for non-retryable error), got %d", attempts)
	}
}

func TestRetryManager_Retry_ContextCancellation(t *testing.T) {
	logger := logging.New(logging.LevelDebug)
	config := RetryConfig{
		MaxRetries:    5,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
	}
	rm := NewRetryManager(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	var attempts int32
	retryableErr := &APIError{StatusCode: http.StatusServiceUnavailable, Message: "temporary failure"}

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := rm.Retry(ctx, func() error {
		atomic.AddInt32(&attempts, 1)
		return retryableErr
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRetryManager_Retry_NoRetriesConfigured(t *testing.T) {
	logger := logging.New(logging.LevelDebug)
	config := RetryConfig{
		MaxRetries:    0, // No retries
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	rm := NewRetryManager(config, logger)

	var attempts int32
	retryableErr := &APIError{StatusCode: http.StatusServiceUnavailable, Message: "temporary failure"}

	err := rm.Retry(context.Background(), func() error {
		atomic.AddInt32(&attempts, 1)
		return retryableErr
	})

	if err == nil {
		t.Error("expected error")
	}
	// Should only attempt once when MaxRetries=0
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retries configured), got %d", attempts)
	}
}

func TestRetryManager_Retry_ContextCanceledBeforeStart(t *testing.T) {
	logger := logging.New(logging.LevelDebug)
	config := DefaultRetryConfig()
	rm := NewRetryManager(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var attempts int32
	err := rm.Retry(ctx, func() error {
		atomic.AddInt32(&attempts, 1)
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if atomic.LoadInt32(&attempts) != 0 {
		t.Errorf("expected 0 attempts when context already canceled, got %d", attempts)
	}
}

func TestRetryableHTTPError(t *testing.T) {
	err := &RetryableHTTPError{
		StatusCode: http.StatusServiceUnavailable,
		Body:       "service unavailable",
	}

	if err.Error() != "retryable HTTP error: status Service Unavailable" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if err.Unwrap() != nil {
		t.Error("expected Unwrap to return nil")
	}
}

func TestNewRetryManager_NilLogger(t *testing.T) {
	config := DefaultRetryConfig()
	rm := NewRetryManager(config, nil)

	if rm.logger == nil {
		t.Error("expected default logger to be created")
	}
}
