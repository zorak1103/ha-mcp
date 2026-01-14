package homeassistant

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestNewRESTClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		baseURL         string
		token           string
		wantBaseURL     string
		wantTokenPrefix string
	}{
		{
			name:            "standard URL",
			baseURL:         "http://localhost:8123",
			token:           "test-token",
			wantBaseURL:     "http://localhost:8123",
			wantTokenPrefix: "test-token",
		},
		{
			name:            "URL with trailing slash",
			baseURL:         "http://localhost:8123/",
			token:           "my-token",
			wantBaseURL:     "http://localhost:8123",
			wantTokenPrefix: "my-token",
		},
		{
			name:            "URL with /api suffix",
			baseURL:         "http://localhost:8123/api",
			token:           "another-token",
			wantBaseURL:     "http://localhost:8123",
			wantTokenPrefix: "another-token",
		},
		{
			name:            "URL with /api/ suffix",
			baseURL:         "http://localhost:8123/api/",
			token:           "token123",
			wantBaseURL:     "http://localhost:8123",
			wantTokenPrefix: "token123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewRESTClient(tt.baseURL, tt.token)

			if client.baseURL != tt.wantBaseURL {
				t.Errorf("baseURL = %q, want %q", client.baseURL, tt.wantBaseURL)
			}
			if client.token != tt.wantTokenPrefix {
				t.Errorf("token = %q, want %q", client.token, tt.wantTokenPrefix)
			}
			if client.httpClient == nil {
				t.Error("httpClient is nil")
			}
		})
	}
}

func TestNewRESTClientWithConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      RESTClientConfig
		wantTimeout time.Duration
	}{
		{
			name:        "default timeout when zero",
			config:      RESTClientConfig{Timeout: 0},
			wantTimeout: 30 * time.Second,
		},
		{
			name:        "custom timeout",
			config:      RESTClientConfig{Timeout: 10 * time.Second},
			wantTimeout: 10 * time.Second,
		},
		{
			name:        "longer timeout",
			config:      RESTClientConfig{Timeout: 60 * time.Second},
			wantTimeout: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewRESTClientWithConfig("http://localhost:8123", "token", tt.config)

			if client.httpClient.Timeout != tt.wantTimeout {
				t.Errorf("timeout = %v, want %v", client.httpClient.Timeout, tt.wantTimeout)
			}
		})
	}
}

func TestRESTClient_DeleteAutomation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		automationID   string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantErrType    string
		wantErrMsg     string
	}{
		{
			name:         "successful deletion with 200",
			automationID: "test_automation",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name:         "successful deletion with 204",
			automationID: "another_automation",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			wantErr: false,
		},
		{
			name:         "automation not found",
			automationID: "nonexistent",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:     true,
			wantErrType: "*homeassistant.APIError",
			wantErrMsg:  "automation not found: nonexistent",
		},
		{
			name:         "unauthorized",
			automationID: "test",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:     true,
			wantErrType: "*homeassistant.APIError",
			wantErrMsg:  "unauthorized: invalid or expired token",
		},
		{
			name:         "forbidden",
			automationID: "test",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			wantErr:     true,
			wantErrType: "*homeassistant.APIError",
			wantErrMsg:  "forbidden: insufficient permissions to delete automation",
		},
		{
			name:         "server error",
			automationID: "test",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal error"))
			},
			wantErr:     true,
			wantErrType: "*homeassistant.APIError",
			wantErrMsg:  "unexpected status 500: internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewRESTClient(server.URL, "test-token")
			ctx := context.Background()

			// Act
			err := client.DeleteAutomation(ctx, tt.automationID)

			// Assert
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteAutomation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify request details
			if capturedRequest == nil {
				t.Fatal("no request was captured")
			}
			if capturedRequest.Method != http.MethodDelete {
				t.Errorf("method = %q, want %q", capturedRequest.Method, http.MethodDelete)
			}
			expectedPath := "/api/config/automation/config/" + tt.automationID
			if capturedRequest.URL.Path != expectedPath {
				t.Errorf("path = %q, want %q", capturedRequest.URL.Path, expectedPath)
			}
			if auth := capturedRequest.Header.Get("Authorization"); auth != "Bearer test-token" {
				t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
			}

			if tt.wantErr && err != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Errorf("error type = %T, want %s", err, tt.wantErrType)
					return
				}
				if apiErr.Message != tt.wantErrMsg {
					t.Errorf("error message = %q, want %q", apiErr.Message, tt.wantErrMsg)
				}
			}
		})
	}
}

func TestRESTClient_DeleteScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		scriptID       string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name:     "successful deletion",
			scriptID: "test_script",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name:     "script not found",
			scriptID: "nonexistent",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:    true,
			wantErrMsg: "script not found: nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify endpoint path
				expectedPath := "/api/config/script/config/" + tt.scriptID
				if r.URL.Path != expectedPath {
					t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
				}
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewRESTClient(server.URL, "test-token")
			err := client.DeleteScript(context.Background(), tt.scriptID)

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteScript() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Errorf("error type = %T, want *APIError", err)
					return
				}
				if apiErr.Message != tt.wantErrMsg {
					t.Errorf("error message = %q, want %q", apiErr.Message, tt.wantErrMsg)
				}
			}
		})
	}
}

func TestRESTClient_DeleteScene(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sceneID        string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name:    "successful deletion",
			sceneID: "test_scene",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name:    "scene not found",
			sceneID: "nonexistent",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr:    true,
			wantErrMsg: "scene not found: nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/config/scene/config/" + tt.sceneID
				if r.URL.Path != expectedPath {
					t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
				}
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewRESTClient(server.URL, "test-token")
			err := client.DeleteScene(context.Background(), tt.sceneID)

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteScene() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Errorf("error type = %T, want *APIError", err)
					return
				}
				if apiErr.Message != tt.wantErrMsg {
					t.Errorf("error message = %q, want %q", apiErr.Message, tt.wantErrMsg)
				}
			}
		})
	}
}

func TestRESTClient_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewRESTClient(server.URL, "test-token")

	// Create context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.DeleteAutomation(ctx, "test")

	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}

func TestDefaultRESTClientConfig(t *testing.T) {
	t.Parallel()

	config := DefaultRESTClientConfig()

	want := RESTClientConfig{
		Timeout: 30 * time.Second,
	}

	if diff := cmp.Diff(want, config); diff != "" {
		t.Errorf("DefaultRESTClientConfig() mismatch (-want +got):\n%s", diff)
	}
}
