package homeassistant

import (
	"context"
	"testing"
	"time"
)

func TestNewClientPool(t *testing.T) {
	pool := NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	if pool == nil {
		t.Fatal("NewClientPool returned nil")
	}

	if pool.Size() != 0 {
		t.Errorf("Expected empty pool, got size %d", pool.Size())
	}
}

func TestClientPool_Size(t *testing.T) {
	pool := NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	if pool.Size() != 0 {
		t.Errorf("Expected size 0, got %d", pool.Size())
	}
}

func TestClientPool_Close(t *testing.T) {
	pool := NewClientPool("http://localhost:8123", 30*time.Minute)

	err := pool.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Pool should be empty after close
	if pool.Size() != 0 {
		t.Errorf("Expected empty pool after close, got size %d", pool.Size())
	}
}

func TestClientPool_GetOrCreate_InvalidToken(t *testing.T) {
	pool := NewClientPool("http://localhost:8123", 30*time.Minute)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// This should fail because we can't connect to localhost:8123
	_, err := pool.GetOrCreate(ctx, "invalid-token")
	if err == nil {
		t.Error("Expected error for invalid connection, got nil")
	}
}

// Note: TestExtractBearerToken is in server_test.go since extractBearerToken is in mcp package

// connectionChecker is a minimal type for testing IsConnected behavior.
type connectionChecker struct {
	connected bool
}

func (c *connectionChecker) IsConnected() bool {
	return c.connected
}

// noConnectionChecker is a type without IsConnected method.
type noConnectionChecker struct{}

func TestIsClientConnected_WithConnectedClient(t *testing.T) {
	// Use type assertion test - connectionChecker has IsConnected
	checker := &connectionChecker{connected: true}
	// Test the type assertion directly
	if c, ok := interface{}(checker).(interface{ IsConnected() bool }); ok {
		if !c.IsConnected() {
			t.Error("Expected connected checker to return true")
		}
	} else {
		t.Error("Expected connectionChecker to implement IsConnected interface")
	}
}

func TestIsClientConnected_WithDisconnectedClient(t *testing.T) {
	checker := &connectionChecker{connected: false}
	if c, ok := interface{}(checker).(interface{ IsConnected() bool }); ok {
		if c.IsConnected() {
			t.Error("Expected disconnected checker to return false")
		}
	} else {
		t.Error("Expected connectionChecker to implement IsConnected interface")
	}
}

func TestIsClientConnected_WithoutIsConnectedMethod(t *testing.T) {
	// noConnectionChecker doesn't have IsConnected method
	checker := &noConnectionChecker{}
	// Should not satisfy the interface
	if _, ok := interface{}(checker).(interface{ IsConnected() bool }); ok {
		t.Error("Expected noConnectionChecker to NOT implement IsConnected interface")
	}
}
