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

func TestRESTClient_GetServices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantErrMsg     string
		wantDomains    []string
	}{
		{
			name: "successful response",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[
					{"domain": "light", "services": {"turn_on": {"name": "Turn on"}, "turn_off": {"name": "Turn off"}}},
					{"domain": "switch", "services": {"toggle": {"name": "Toggle"}}}
				]`))
			},
			wantErr:     false,
			wantDomains: []string{"light", "switch"},
		},
		{
			name: "empty services",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
			},
			wantErr:     false,
			wantDomains: []string{},
		},
		{
			name: "unauthorized",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message": "invalid token"}`))
			},
			wantErr:    true,
			wantErrMsg: "failed to get services",
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:    true,
			wantErrMsg: "failed to get services",
		},
		{
			name: "invalid JSON",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`invalid json`))
			},
			wantErr:    true,
			wantErrMsg: "parsing services response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewRESTClient(server.URL, "test-token")
			services, err := client.GetServices(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify request
			if capturedRequest != nil {
				if capturedRequest.Method != http.MethodGet {
					t.Errorf("method = %q, want %q", capturedRequest.Method, http.MethodGet)
				}
				if capturedRequest.URL.Path != "/api/services" {
					t.Errorf("path = %q, want %q", capturedRequest.URL.Path, "/api/services")
				}
				if auth := capturedRequest.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
				}
			}

			if tt.wantErr {
				return
			}

			// Check returned domains
			gotDomains := make([]string, len(services))
			for i, svc := range services {
				gotDomains[i] = svc.Domain
			}
			if diff := cmp.Diff(tt.wantDomains, gotDomains); diff != "" {
				t.Errorf("domains mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRESTClient_GetConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantErrMsg     string
		wantVersion    string
	}{
		{
			name: "successful response",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"version": "2024.1.0",
					"state": "RUNNING",
					"location_name": "Home",
					"time_zone": "Europe/Berlin",
					"latitude": 52.52,
					"longitude": 13.405,
					"elevation": 34,
					"unit_system": {
						"length": "km",
						"temperature": "°C"
					},
					"components": ["light", "switch"]
				}`))
			},
			wantErr:     false,
			wantVersion: "2024.1.0",
		},
		{
			name: "unauthorized",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:    true,
			wantErrMsg: "failed to get config",
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:    true,
			wantErrMsg: "failed to get config",
		},
		{
			name: "invalid JSON",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`not json`))
			},
			wantErr:    true,
			wantErrMsg: "parsing config response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewRESTClient(server.URL, "test-token")
			config, err := client.GetConfig(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("GetConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify request
			if capturedRequest != nil {
				if capturedRequest.Method != http.MethodGet {
					t.Errorf("method = %q, want %q", capturedRequest.Method, http.MethodGet)
				}
				if capturedRequest.URL.Path != "/api/config" {
					t.Errorf("path = %q, want %q", capturedRequest.URL.Path, "/api/config")
				}
				if auth := capturedRequest.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
				}
			}

			if tt.wantErr {
				return
			}

			if config.Version != tt.wantVersion {
				t.Errorf("version = %q, want %q", config.Version, tt.wantVersion)
			}
		})
	}
}

func TestRESTClient_GetServices_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewRESTClient(server.URL, "test-token")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetServices(ctx)
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}

func TestRESTClient_GetConfig_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewRESTClient(server.URL, "test-token")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetConfig(ctx)
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}

func TestRESTClient_RenderTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		template       string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantResult     string
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name:     "successful template render",
			template: "{{ states('sensor.temperature') }}",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				if r.Method != http.MethodPost {
					t.Errorf("method = %q, want POST", r.Method)
				}
				if r.URL.Path != "/api/template" {
					t.Errorf("path = %q, want /api/template", r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("22.5"))
			},
			wantResult: "22.5",
			wantErr:    false,
		},
		{
			name:     "template with state attributes",
			template: "{{ state_attr('sensor.temperature', 'unit_of_measurement') }}",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("°C"))
			},
			wantResult: "°C",
			wantErr:    false,
		},
		{
			name:     "empty template result",
			template: "{{ '' }}",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(""))
			},
			wantResult: "",
			wantErr:    false,
		},
		{
			name:     "template error - invalid syntax",
			template: "{{ invalid syntax }}",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("TemplateSyntaxError: unexpected end of template"))
			},
			wantErr:    true,
			wantErrMsg: "failed to render template",
		},
		{
			name:     "unauthorized",
			template: "{{ states('sensor.temperature') }}",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("unauthorized"))
			},
			wantErr:    true,
			wantErrMsg: "failed to render template",
		},
		{
			name:     "server error",
			template: "{{ states('sensor.temperature') }}",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal error"))
			},
			wantErr:    true,
			wantErrMsg: "failed to render template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewRESTClient(server.URL, "test-token")
			result, err := client.RenderTemplate(context.Background(), tt.template)

			if (err != nil) != tt.wantErr {
				t.Errorf("RenderTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify request
			if capturedRequest != nil {
				if auth := capturedRequest.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
				}
				if ct := capturedRequest.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %q, want %q", ct, "application/json")
				}
			}

			if tt.wantErr {
				return
			}

			if result != tt.wantResult {
				t.Errorf("result = %q, want %q", result, tt.wantResult)
			}
		})
	}
}

func TestRESTClient_RenderTemplate_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewRESTClient(server.URL, "test-token")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.RenderTemplate(ctx, "{{ states('sensor.test') }}")
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}

func TestRESTClient_GetLogbook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		startTime      string
		endTime        string
		entityID       string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantCount      int
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name:      "successful logbook retrieval",
			startTime: "2024-01-01T00:00:00Z",
			endTime:   "2024-01-01T23:59:59Z",
			entityID:  "light.living_room",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify path contains start_time
				if r.URL.Path != "/api/logbook/2024-01-01T00:00:00Z" {
					t.Errorf("path = %q, want /api/logbook/2024-01-01T00:00:00Z", r.URL.Path)
				}
				// Verify query parameters
				if r.URL.Query().Get("end_time") != "2024-01-01T23:59:59Z" {
					t.Errorf("end_time = %q, want 2024-01-01T23:59:59Z", r.URL.Query().Get("end_time"))
				}
				if r.URL.Query().Get("entity") != "light.living_room" {
					t.Errorf("entity = %q, want light.living_room", r.URL.Query().Get("entity"))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[
					{"name": "Living Room Light", "message": "turned on", "entity_id": "light.living_room", "when": "2024-01-01T08:00:00Z"},
					{"name": "Living Room Light", "message": "turned off", "entity_id": "light.living_room", "when": "2024-01-01T22:00:00Z"}
				]`))
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "logbook without entity filter",
			startTime: "2024-01-01T00:00:00Z",
			endTime:   "",
			entityID:  "",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify no query parameters
				if r.URL.Query().Get("entity") != "" {
					t.Errorf("entity should be empty, got %q", r.URL.Query().Get("entity"))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[{"name": "Test", "message": "test"}]`))
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "empty logbook",
			startTime: "2024-01-01T00:00:00Z",
			endTime:   "2024-01-01T01:00:00Z",
			entityID:  "sensor.nonexistent",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "unauthorized",
			startTime: "2024-01-01T00:00:00Z",
			endTime:   "",
			entityID:  "",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("unauthorized"))
			},
			wantErr:    true,
			wantErrMsg: "failed to get logbook",
		},
		{
			name:      "server error",
			startTime: "2024-01-01T00:00:00Z",
			endTime:   "",
			entityID:  "",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:    true,
			wantErrMsg: "failed to get logbook",
		},
		{
			name:      "invalid JSON response",
			startTime: "2024-01-01T00:00:00Z",
			endTime:   "",
			entityID:  "",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("not valid json"))
			},
			wantErr:    true,
			wantErrMsg: "parsing logbook response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewRESTClient(server.URL, "test-token")
			entries, err := client.GetLogbook(context.Background(), tt.startTime, tt.endTime, tt.entityID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetLogbook() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify request
			if capturedRequest != nil {
				if capturedRequest.Method != http.MethodGet {
					t.Errorf("method = %q, want GET", capturedRequest.Method)
				}
				if auth := capturedRequest.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
				}
			}

			if tt.wantErr {
				return
			}

			if len(entries) != tt.wantCount {
				t.Errorf("got %d entries, want %d", len(entries), tt.wantCount)
			}
		})
	}
}

func TestRESTClient_GetLogbook_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewRESTClient(server.URL, "test-token")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetLogbook(ctx, "2024-01-01T00:00:00Z", "", "")
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}

func TestRESTClient_CheckConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantResult     string
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name: "config valid",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %q, want POST", r.Method)
				}
				if r.URL.Path != "/api/config/core/check_config" {
					t.Errorf("path = %q, want /api/config/core/check_config", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"result": "valid", "errors": null}`))
			},
			wantResult: "valid",
			wantErr:    false,
		},
		{
			name: "config invalid with errors",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"result": "invalid", "errors": "Invalid config for [automation]: required key not provided"}`))
			},
			wantResult: "invalid",
			wantErr:    false,
		},
		{
			name: "unauthorized",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("unauthorized"))
			},
			wantErr:    true,
			wantErrMsg: "failed to check config",
		},
		{
			name: "forbidden",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("forbidden"))
			},
			wantErr:    true,
			wantErrMsg: "failed to check config",
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:    true,
			wantErrMsg: "failed to check config",
		},
		{
			name: "invalid JSON response",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid json"))
			},
			wantErr:    true,
			wantErrMsg: "parsing check config response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewRESTClient(server.URL, "test-token")
			result, err := client.CheckConfig(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify request
			if capturedRequest != nil {
				if auth := capturedRequest.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
				}
			}

			if tt.wantErr {
				return
			}

			if result.Result != tt.wantResult {
				t.Errorf("result = %q, want %q", result.Result, tt.wantResult)
			}
		})
	}
}

func TestRESTClient_CheckConfig_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewRESTClient(server.URL, "test-token")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.CheckConfig(ctx)
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}
