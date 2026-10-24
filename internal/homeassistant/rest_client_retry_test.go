package homeassistant

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newRetryTestClient starts a test server around handler and returns a REST
// client with fast retries (MaxRetries 2) plus a counter of requests that
// reached the handler.
func newRetryTestClient(t *testing.T, handler http.HandlerFunc) (*RESTClient, *atomic.Int32) {
	t.Helper()

	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	client := NewRESTClientWithConfig(srv.URL, "tok", RESTClientConfig{
		RetryConfig: RetryConfig{
			MaxRetries:    2,
			InitialDelay:  time.Millisecond,
			MaxDelay:      5 * time.Millisecond,
			BackoffFactor: 2,
		},
	})
	return client, &count
}

func TestRESTClient_GET_RetriedOn503(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client, count := newRetryTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("{}"))
	})

	if _, err := client.GetConfig(context.Background()); err != nil {
		t.Fatalf("GetConfig() error = %v, want nil", err)
	}
	if got := count.Load(); got != 3 {
		t.Errorf("request count = %d, want 3", got)
	}
}

func TestRESTClient_POST_NotRetriedOn503(t *testing.T) {
	t.Parallel()

	client, count := newRetryTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("down"))
	})

	err := client.CreateScript(context.Background(), "s1", ScriptConfig{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("CreateScript() error = %v, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusServiceUnavailable || apiErr.Message != "down" {
		t.Errorf("APIError = {%d, %q}, want {503, \"down\"}", apiErr.StatusCode, apiErr.Message)
	}
	if got := count.Load(); got != 1 {
		t.Errorf("request count = %d, want 1", got)
	}
}

func TestRESTClient_DELETE_NotRetriedOn500(t *testing.T) {
	t.Parallel()

	client, count := newRetryTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if err := client.DeleteScript(context.Background(), "s1"); err == nil {
		t.Fatal("DeleteScript() error = nil, want error")
	}
	if got := count.Load(); got != 1 {
		t.Errorf("request count = %d, want 1", got)
	}
}

func TestRESTClient_POST_RetriedOn429_ResendsBody(t *testing.T) {
	t.Parallel()

	var (
		mu     sync.Mutex
		bodies []string
	)
	client, count := newRetryTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		n := len(bodies)
		mu.Unlock()
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := client.CreateScript(context.Background(), "s1", ScriptConfig{Alias: "x"}); err != nil {
		t.Fatalf("CreateScript() error = %v, want nil", err)
	}
	if got := count.Load(); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 || bodies[0] == "" || bodies[0] != bodies[1] {
		t.Errorf("bodies = %q, want two equal non-empty bodies", bodies)
	}
}

func TestRESTClient_DialError_Retried(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(c *RESTClient) error
	}{
		{"GET", func(c *RESTClient) error {
			_, err := c.GetConfig(context.Background())
			return err
		}},
		{"POST", func(c *RESTClient) error {
			return c.CreateScript(context.Background(), "s1", ScriptConfig{})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, count := newRetryTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("{}"))
			})

			var dials atomic.Int32
			dialer := &net.Dialer{}
			client.httpClient.Transport = &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					if dials.Add(1) == 1 {
						return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("connection refused")}
					}
					return dialer.DialContext(ctx, network, addr)
				},
			}

			if err := tt.call(client); err != nil {
				t.Fatalf("call error = %v, want nil", err)
			}
			if got := count.Load(); got != 1 {
				t.Errorf("handler request count = %d, want 1", got)
			}
		})
	}
}

func TestRESTClient_MaxRetriesZero_SingleAttempt(t *testing.T) {
	t.Parallel()

	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("down"))
	}))
	t.Cleanup(srv.Close)

	client := NewRESTClientWithConfig(srv.URL, "tok", RESTClientConfig{})

	_, err := client.GetConfig(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("GetConfig() error = %v, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusServiceUnavailable || apiErr.Message != "down" {
		t.Errorf("APIError = {%d, %q}, want {503, \"down\"}", apiErr.StatusCode, apiErr.Message)
	}
	if got := count.Load(); got != 1 {
		t.Errorf("request count = %d, want 1", got)
	}
}
