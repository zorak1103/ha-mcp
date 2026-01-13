package homeassistant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHybridClient_DeleteAutomation verifies that DeleteAutomation
// uses the REST client instead of WebSocket.
func TestHybridClient_DeleteAutomation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		automationID   string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
	}{
		{
			name:         "successful deletion via REST",
			automationID: "test_automation",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify this is a REST DELETE request
				if r.Method != http.MethodDelete {
					t.Errorf("expected DELETE method, got %s", r.Method)
				}
				expectedPath := "/api/config/automation/config/test_automation"
				if r.URL.Path != expectedPath {
					t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
				}
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name:         "REST error propagates",
			automationID: "nonexistent",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange - Create mock REST server
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			restClient := NewRESTClient(server.URL, "test-token")

			// Create HybridClient with nil WebSocket (we're only testing REST)
			// Note: In real usage, wsClientImpl would be initialized
			hybridClient := &HybridClient{
				ws:   nil, // Not used for DeleteAutomation
				rest: restClient,
			}

			// Act
			err := hybridClient.DeleteAutomation(context.Background(), tt.automationID)

			// Assert
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteAutomation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestHybridClient_DeleteScript verifies that DeleteScript
// uses the REST client instead of WebSocket.
func TestHybridClient_DeleteScript(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		expectedPath := "/api/config/script/config/test_script"
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	hybridClient := &HybridClient{
		ws:   nil,
		rest: restClient,
	}

	err := hybridClient.DeleteScript(context.Background(), "test_script")

	if err != nil {
		t.Errorf("DeleteScript() error = %v, want nil", err)
	}
}

// TestHybridClient_DeleteScene verifies that DeleteScene
// uses the REST client instead of WebSocket.
func TestHybridClient_DeleteScene(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		expectedPath := "/api/config/scene/config/test_scene"
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	hybridClient := &HybridClient{
		ws:   nil,
		rest: restClient,
	}

	err := hybridClient.DeleteScene(context.Background(), "test_scene")

	if err != nil {
		t.Errorf("DeleteScene() error = %v, want nil", err)
	}
}

// TestNewHybridClient verifies the constructor creates a properly initialized client.
func TestNewHybridClient(t *testing.T) {
	t.Parallel()

	// Create mock REST server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")

	// Create a minimal WSClient for testing (won't connect)
	wsClient := NewWSClient("ws://localhost:8123", "test-token")

	hybridClient := NewHybridClient(wsClient, restClient)

	// Verify the client was created
	if hybridClient == nil {
		t.Fatal("NewHybridClient returned nil")
	}
	if hybridClient.ws == nil {
		t.Error("ws client is nil")
	}
	if hybridClient.rest == nil {
		t.Error("rest client is nil")
	}
}

// TestNewHybridClientCloser verifies the closer variant works correctly.
func TestNewHybridClientCloser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restClient := NewRESTClient(server.URL, "test-token")
	wsClient := NewWSClient("ws://localhost:8123", "test-token")

	hybridCloser := NewHybridClientCloser(wsClient, restClient)

	// Verify it implements both interfaces
	var _ Client = hybridCloser
	var _ ClientCloser = hybridCloser

	if hybridCloser == nil {
		t.Fatal("NewHybridClientCloser returned nil")
	}

	// Test Close method (should not panic even without connection)
	// Note: This may error because there's no actual connection,
	// but it should not panic
	_ = hybridCloser.Close()
}

// TestHybridClient_InterfaceCompliance verifies that HybridClient
// implements the Client interface at compile time.
func TestHybridClient_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	// This test is primarily for compile-time verification
	// The var _ declarations in hybrid_client.go also verify this
	var _ Client = (*HybridClient)(nil)
	var _ Client = (*HybridClientCloser)(nil)
	var _ ClientCloser = (*HybridClientCloser)(nil)
}
