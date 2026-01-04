// Package homeassistant provides a WebSocket client for Home Assistant API.
package homeassistant

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

// ErrMaxReconnectAttempts is returned when the maximum number of reconnection attempts is reached.
var ErrMaxReconnectAttempts = errors.New("maximum reconnection attempts reached")

// ReconnectConfig holds configuration for reconnection behavior.
type ReconnectConfig struct {
	// InitialDelay is the starting delay between reconnection attempts.
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between reconnection attempts.
	MaxDelay time.Duration
	// BackoffFactor is the multiplier applied to the delay after each attempt.
	BackoffFactor float64
	// MaxAttempts is the maximum number of reconnection attempts (0 = unlimited).
	MaxAttempts int
}

// DefaultReconnectConfig returns the default reconnection configuration.
func DefaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		InitialDelay:  1 * time.Second,
		MaxDelay:      60 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   0, // Unlimited
	}
}

// ReconnectManager handles automatic reconnection with exponential backoff.
type ReconnectManager struct {
	config      ReconnectConfig
	attempts    int
	currentWait time.Duration
	mu          sync.Mutex
	timer       *time.Timer
	cancelFunc  context.CancelFunc
}

// NewReconnectManager creates a new ReconnectManager with the given configuration.
func NewReconnectManager(config ReconnectConfig) *ReconnectManager {
	return &ReconnectManager{
		config:      config,
		currentWait: config.InitialDelay,
	}
}

// Reset resets the reconnection state to initial values.
// Call this after a successful connection.
func (r *ReconnectManager) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.attempts = 0
	r.currentWait = r.config.InitialDelay
	r.stopTimerLocked()
}

// stopTimerLocked stops any pending timer (must hold lock).
func (r *ReconnectManager) stopTimerLocked() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	if r.cancelFunc != nil {
		r.cancelFunc()
		r.cancelFunc = nil
	}
}

// Stop stops any pending reconnection attempt.
func (r *ReconnectManager) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopTimerLocked()
}

// ShouldReconnect returns true if another reconnection attempt should be made.
func (r *ReconnectManager) ShouldReconnect() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.config.MaxAttempts == 0 {
		return true // Unlimited attempts
	}
	return r.attempts < r.config.MaxAttempts
}

// GetAttempts returns the current number of reconnection attempts.
func (r *ReconnectManager) GetAttempts() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempts
}

// GetCurrentDelay returns the current wait time before the next reconnection attempt.
func (r *ReconnectManager) GetCurrentDelay() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentWait
}

// WaitForReconnect waits for the appropriate backoff duration before reconnecting.
// Returns an error if the context is cancelled or max attempts reached.
func (r *ReconnectManager) WaitForReconnect(ctx context.Context) error {
	r.mu.Lock()

	// Check max attempts
	if r.config.MaxAttempts > 0 && r.attempts >= r.config.MaxAttempts {
		r.mu.Unlock()
		return ErrMaxReconnectAttempts
	}

	// Increment attempt counter
	r.attempts++
	waitDuration := r.currentWait

	// Calculate next wait duration with exponential backoff
	nextWait := time.Duration(float64(r.currentWait) * r.config.BackoffFactor)
	if nextWait > r.config.MaxDelay {
		nextWait = r.config.MaxDelay
	}
	r.currentWait = nextWait

	// Create timer and cancellation context
	waitCtx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel
	r.timer = time.NewTimer(waitDuration)

	r.mu.Unlock()

	// Wait for timer or cancellation
	select {
	case <-r.timer.C:
		return nil
	case <-waitCtx.Done():
		r.Stop()
		return waitCtx.Err()
	}
}

// CalculateBackoff returns the backoff duration for a given attempt number.
// This is a utility function for external use without modifying state.
func CalculateBackoff(attempt int, config ReconnectConfig) time.Duration {
	if attempt <= 0 {
		return config.InitialDelay
	}

	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt-1))
	if delay > float64(config.MaxDelay) {
		return config.MaxDelay
	}
	return time.Duration(delay)
}

// OnReconnectFunc is a callback function called after successful reconnection.
type OnReconnectFunc func(attempts int)

// OnDisconnectFunc is a callback function called when a disconnect is detected.
type OnDisconnectFunc func(err error)
