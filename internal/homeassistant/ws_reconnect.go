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
	done        chan struct{} // Signal channel to cancel waiting
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
	if r.done != nil {
		// Signal any waiting goroutine to exit.
		// Use select to handle case where channel is already closed.
		select {
		case <-r.done:
			// Already closed, do nothing
		default:
			close(r.done)
		}
		r.done = nil
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
// Returns an error if the context is canceled or max attempts reached.
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

	// Calculate next wait duration with exponential backoff.
	// We update currentWait BEFORE waiting so that if this goroutine is
	// interrupted and another caller enters, they will see the increased delay.
	nextWait := time.Duration(float64(r.currentWait) * r.config.BackoffFactor)
	if nextWait > r.config.MaxDelay {
		nextWait = r.config.MaxDelay
	}
	r.currentWait = nextWait

	// Clean up any previous wait state before creating new one
	r.stopTimerLocked()

	// Create timer and done channel for safe cancellation
	timer := time.NewTimer(waitDuration)
	r.timer = timer
	done := make(chan struct{})
	r.done = done

	// Capture channels before unlocking to use in select.
	// The done channel provides a safe way to cancel from Stop().
	timerChan := timer.C

	r.mu.Unlock()

	// Wait for timer, context cancellation, or explicit stop
	select {
	case <-timerChan:
		return nil
	case <-done:
		// Stopped via Stop() or Reset()
		return context.Canceled
	case <-ctx.Done():
		r.Stop()
		return ctx.Err()
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
