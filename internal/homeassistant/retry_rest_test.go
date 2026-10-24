package homeassistant

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestRetryManager_RetryIf_UsesPredicate(t *testing.T) {
	t.Parallel()

	newManager := func() *RetryManager {
		return NewRetryManager(RetryConfig{
			MaxRetries:    2,
			InitialDelay:  time.Millisecond,
			MaxDelay:      time.Millisecond,
			BackoffFactor: 2,
		}, nil)
	}
	apiErr := &APIError{StatusCode: http.StatusServiceUnavailable, Message: "down"}

	t.Run("false predicate stops after one attempt", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		err := newManager().RetryIf(context.Background(), func() error {
			attempts++
			return apiErr
		}, func(error) bool { return false })

		if !errors.Is(err, apiErr) {
			t.Errorf("error = %v, want %v", err, apiErr)
		}
		if attempts != 1 {
			t.Errorf("attempts = %d, want 1", attempts)
		}
	})

	t.Run("true predicate retries up to MaxRetries", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		err := newManager().RetryIf(context.Background(), func() error {
			attempts++
			return apiErr
		}, func(error) bool { return true })

		if !errors.Is(err, apiErr) {
			t.Errorf("error = %v, want %v", err, apiErr)
		}
		if attempts != 3 {
			t.Errorf("attempts = %d, want 3", attempts)
		}
	})
}

func TestIsRetryableRESTError(t *testing.T) {
	t.Parallel()

	dialErr := &url.Error{
		Op:  "Post",
		URL: "http://x",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
	}
	api := func(code int) error { return &APIError{StatusCode: code} }

	tests := []struct {
		name   string
		method string
		err    error
		want   bool
	}{
		{"GET 503", http.MethodGet, api(503), true},
		{"HEAD 503", http.MethodHead, api(503), true},
		{"empty method 503", "", api(503), true},
		{"GET 404", http.MethodGet, api(404), false},
		{"GET dial error", http.MethodGet, dialErr, true},
		{"GET connection reset", http.MethodGet, errors.New("connection reset by peer"), true},
		{"POST 503", http.MethodPost, api(503), false},
		{"DELETE 503", http.MethodDelete, api(503), false},
		{"POST 500", http.MethodPost, api(500), false},
		{"DELETE 500", http.MethodDelete, api(500), false},
		{"POST 429", http.MethodPost, api(429), true},
		{"DELETE 429", http.MethodDelete, api(429), true},
		{"POST dial error", http.MethodPost, dialErr, true},
		{"POST connection reset", http.MethodPost, errors.New("connection reset by peer"), false},
		{"POST read EOF", http.MethodPost, &url.Error{Err: &net.OpError{Op: "read", Err: io.EOF}}, false},
		{"canceled dial", http.MethodPost, &url.Error{Err: &net.OpError{Op: "dial", Err: context.Canceled}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isRetryableRESTError(tt.method, tt.err); got != tt.want {
				t.Errorf("isRetryableRESTError(%q, %v) = %v, want %v", tt.method, tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryableError_RealDialError(t *testing.T) {
	t.Parallel()

	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, doErr := (&http.Client{}).Do(req)
	if doErr == nil {
		_ = resp.Body.Close()
		t.Fatal("expected dial error, got nil")
	}

	if !IsRetryableError(doErr) {
		t.Errorf("IsRetryableError(%v) = false, want true", doErr)
	}
	if IsRetryableError(&mockNetError{timeout: false}) {
		t.Error("non-timeout mockNetError with unrelated message must stay non-retryable")
	}
}
