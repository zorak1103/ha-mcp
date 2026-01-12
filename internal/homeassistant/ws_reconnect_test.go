package homeassistant

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultReconnectConfig(t *testing.T) {
	t.Parallel()

	config := DefaultReconnectConfig()

	if config.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay = %v, want %v", config.InitialDelay, 1*time.Second)
	}
	if config.MaxDelay != 60*time.Second {
		t.Errorf("MaxDelay = %v, want %v", config.MaxDelay, 60*time.Second)
	}
	if config.BackoffFactor != 2.0 {
		t.Errorf("BackoffFactor = %v, want %v", config.BackoffFactor, 2.0)
	}
	if config.MaxAttempts != 0 {
		t.Errorf("MaxAttempts = %v, want %v (unlimited)", config.MaxAttempts, 0)
	}
}

func TestNewReconnectManager(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  500 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 1.5,
		MaxAttempts:   5,
	}

	mgr := NewReconnectManager(config)

	if mgr.config.InitialDelay != config.InitialDelay {
		t.Errorf("config.InitialDelay = %v, want %v", mgr.config.InitialDelay, config.InitialDelay)
	}
	if mgr.config.MaxDelay != config.MaxDelay {
		t.Errorf("config.MaxDelay = %v, want %v", mgr.config.MaxDelay, config.MaxDelay)
	}
	if mgr.config.BackoffFactor != config.BackoffFactor {
		t.Errorf("config.BackoffFactor = %v, want %v", mgr.config.BackoffFactor, config.BackoffFactor)
	}
	if mgr.config.MaxAttempts != config.MaxAttempts {
		t.Errorf("config.MaxAttempts = %v, want %v", mgr.config.MaxAttempts, config.MaxAttempts)
	}
	if mgr.currentWait != config.InitialDelay {
		t.Errorf("currentWait = %v, want %v", mgr.currentWait, config.InitialDelay)
	}
	if mgr.attempts != 0 {
		t.Errorf("attempts = %v, want 0", mgr.attempts)
	}
}

func TestReconnectManager_Reset(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   10,
	}

	mgr := NewReconnectManager(config)

	// Simulate some attempts
	mgr.mu.Lock()
	mgr.attempts = 5
	mgr.currentWait = 500 * time.Millisecond
	mgr.mu.Unlock()

	// Reset
	mgr.Reset()

	if mgr.GetAttempts() != 0 {
		t.Errorf("attempts after Reset() = %v, want 0", mgr.GetAttempts())
	}
	if mgr.GetCurrentDelay() != config.InitialDelay {
		t.Errorf("currentWait after Reset() = %v, want %v", mgr.GetCurrentDelay(), config.InitialDelay)
	}
}

func TestReconnectManager_ShouldReconnect_Unlimited(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   0, // Unlimited
	}

	mgr := NewReconnectManager(config)

	// Should always return true with unlimited attempts
	for i := 0; i < 100; i++ {
		if !mgr.ShouldReconnect() {
			t.Errorf("ShouldReconnect() = false at attempt %d with unlimited attempts", i)
		}
		mgr.mu.Lock()
		mgr.attempts++
		mgr.mu.Unlock()
	}
}

func TestReconnectManager_ShouldReconnect_Limited(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   3,
	}

	mgr := NewReconnectManager(config)

	// First 3 attempts should return true
	for i := 0; i < 3; i++ {
		if !mgr.ShouldReconnect() {
			t.Errorf("ShouldReconnect() = false at attempt %d, want true", i)
		}
		mgr.mu.Lock()
		mgr.attempts++
		mgr.mu.Unlock()
	}

	// 4th attempt should return false
	if mgr.ShouldReconnect() {
		t.Error("ShouldReconnect() = true after max attempts, want false")
	}
}

func TestReconnectManager_GetAttempts(t *testing.T) {
	t.Parallel()

	mgr := NewReconnectManager(DefaultReconnectConfig())

	if mgr.GetAttempts() != 0 {
		t.Errorf("GetAttempts() = %v, want 0", mgr.GetAttempts())
	}

	mgr.mu.Lock()
	mgr.attempts = 42
	mgr.mu.Unlock()

	if mgr.GetAttempts() != 42 {
		t.Errorf("GetAttempts() = %v, want 42", mgr.GetAttempts())
	}
}

func TestReconnectManager_GetCurrentDelay(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  200 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   0,
	}

	mgr := NewReconnectManager(config)

	if mgr.GetCurrentDelay() != 200*time.Millisecond {
		t.Errorf("GetCurrentDelay() = %v, want %v", mgr.GetCurrentDelay(), 200*time.Millisecond)
	}

	mgr.mu.Lock()
	mgr.currentWait = 1600 * time.Millisecond
	mgr.mu.Unlock()

	if mgr.GetCurrentDelay() != 1600*time.Millisecond {
		t.Errorf("GetCurrentDelay() = %v, want %v", mgr.GetCurrentDelay(), 1600*time.Millisecond)
	}
}

func TestReconnectManager_WaitForReconnect_Success(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  10 * time.Millisecond, // Very short for testing
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		MaxAttempts:   0,
	}

	mgr := NewReconnectManager(config)
	ctx := context.Background()

	start := time.Now()
	err := mgr.WaitForReconnect(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("WaitForReconnect() error = %v, want nil", err)
	}

	// Should have waited at least InitialDelay
	if elapsed < 10*time.Millisecond {
		t.Errorf("WaitForReconnect() elapsed = %v, want >= 10ms", elapsed)
	}

	// Attempts should have been incremented
	if mgr.GetAttempts() != 1 {
		t.Errorf("GetAttempts() = %v, want 1", mgr.GetAttempts())
	}
}

func TestReconnectManager_WaitForReconnect_ExponentialBackoff(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   0,
	}

	mgr := NewReconnectManager(config)
	ctx := context.Background()

	// First wait: 10ms
	_ = mgr.WaitForReconnect(ctx)
	// After first wait, currentWait should be 20ms (10 * 2)
	if mgr.GetCurrentDelay() != 20*time.Millisecond {
		t.Errorf("currentWait after 1st wait = %v, want 20ms", mgr.GetCurrentDelay())
	}

	// Second wait: 20ms
	_ = mgr.WaitForReconnect(ctx)
	// After second wait, currentWait should be 40ms (20 * 2)
	if mgr.GetCurrentDelay() != 40*time.Millisecond {
		t.Errorf("currentWait after 2nd wait = %v, want 40ms", mgr.GetCurrentDelay())
	}

	// Third wait: 40ms
	_ = mgr.WaitForReconnect(ctx)
	// After third wait, currentWait should be 80ms (40 * 2)
	if mgr.GetCurrentDelay() != 80*time.Millisecond {
		t.Errorf("currentWait after 3rd wait = %v, want 80ms", mgr.GetCurrentDelay())
	}
}

func TestReconnectManager_WaitForReconnect_MaxDelay(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  50 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 4.0, // Aggressive backoff
		MaxAttempts:   0,
	}

	mgr := NewReconnectManager(config)
	ctx := context.Background()

	// First wait: 50ms, next would be 200ms but capped to 100ms
	_ = mgr.WaitForReconnect(ctx)
	if mgr.GetCurrentDelay() != 100*time.Millisecond {
		t.Errorf("currentWait after 1st wait = %v, want 100ms (capped)", mgr.GetCurrentDelay())
	}

	// Second wait: 100ms (capped), next still 100ms (capped)
	_ = mgr.WaitForReconnect(ctx)
	if mgr.GetCurrentDelay() != 100*time.Millisecond {
		t.Errorf("currentWait after 2nd wait = %v, want 100ms (capped)", mgr.GetCurrentDelay())
	}
}

func TestReconnectManager_WaitForReconnect_MaxAttemptsReached(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		MaxAttempts:   2,
	}

	mgr := NewReconnectManager(config)
	ctx := context.Background()

	// First attempt - should succeed
	err := mgr.WaitForReconnect(ctx)
	if err != nil {
		t.Errorf("1st WaitForReconnect() error = %v, want nil", err)
	}

	// Second attempt - should succeed
	err = mgr.WaitForReconnect(ctx)
	if err != nil {
		t.Errorf("2nd WaitForReconnect() error = %v, want nil", err)
	}

	// Third attempt - should fail with ErrMaxReconnectAttempts
	err = mgr.WaitForReconnect(ctx)
	if !errors.Is(err, ErrMaxReconnectAttempts) {
		t.Errorf("3rd WaitForReconnect() error = %v, want ErrMaxReconnectAttempts", err)
	}
}

func TestReconnectManager_WaitForReconnect_ContextCancelled(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  1 * time.Second, // Long delay to ensure cancellation works
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   0,
	}

	mgr := NewReconnectManager(config)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := mgr.WaitForReconnect(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("WaitForReconnect() error = %v, want context.Canceled", err)
	}

	// Should have been cancelled before the full 1s delay
	if elapsed >= 500*time.Millisecond {
		t.Errorf("WaitForReconnect() elapsed = %v, should have been cancelled earlier", elapsed)
	}
}

func TestReconnectManager_WaitForReconnect_ContextDeadlineExceeded(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  1 * time.Second, // Long delay to ensure timeout works
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   0,
	}

	mgr := NewReconnectManager(config)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := mgr.WaitForReconnect(ctx)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WaitForReconnect() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestReconnectManager_Stop(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  1 * time.Second,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   0,
	}

	mgr := NewReconnectManager(config)

	// Start a wait in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- mgr.WaitForReconnect(context.Background())
	}()

	// Give time for the timer to be created
	time.Sleep(50 * time.Millisecond)

	// Stop the manager
	mgr.Stop()

	// Wait should have been cancelled
	select {
	case <-done:
		// Timer might have completed or been cancelled - both are acceptable
		// if it happened fast enough. We don't check the error value because
		// the outcome depends on timing.
	case <-time.After(500 * time.Millisecond):
		t.Error("WaitForReconnect() did not return after Stop()")
	}
}

func TestCalculateBackoff(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   0,
	}

	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{
			name:    "attempt 0 (invalid, returns initial)",
			attempt: 0,
			want:    100 * time.Millisecond,
		},
		{
			name:    "attempt 1",
			attempt: 1,
			want:    100 * time.Millisecond,
		},
		{
			name:    "attempt 2",
			attempt: 2,
			want:    200 * time.Millisecond,
		},
		{
			name:    "attempt 3",
			attempt: 3,
			want:    400 * time.Millisecond,
		},
		{
			name:    "attempt 4",
			attempt: 4,
			want:    800 * time.Millisecond,
		},
		{
			name:    "attempt 5 (capped)",
			attempt: 5,
			want:    1 * time.Second, // 1600ms would exceed MaxDelay
		},
		{
			name:    "attempt 10 (capped)",
			attempt: 10,
			want:    1 * time.Second,
		},
		{
			name:    "negative attempt",
			attempt: -1,
			want:    100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CalculateBackoff(tt.attempt, config)
			if got != tt.want {
				t.Errorf("CalculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestCalculateBackoff_DifferentFactors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  ReconnectConfig
		attempt int
		want    time.Duration
	}{
		{
			name: "factor 1.5",
			config: ReconnectConfig{
				InitialDelay:  100 * time.Millisecond,
				MaxDelay:      10 * time.Second,
				BackoffFactor: 1.5,
			},
			attempt: 3,
			want:    225 * time.Millisecond, // 100 * 1.5 * 1.5 = 225
		},
		{
			name: "factor 3",
			config: ReconnectConfig{
				InitialDelay:  100 * time.Millisecond,
				MaxDelay:      10 * time.Second,
				BackoffFactor: 3.0,
			},
			attempt: 3,
			want:    900 * time.Millisecond, // 100 * 3 * 3 = 900
		},
		{
			name: "factor 1 (no backoff)",
			config: ReconnectConfig{
				InitialDelay:  100 * time.Millisecond,
				MaxDelay:      10 * time.Second,
				BackoffFactor: 1.0,
			},
			attempt: 5,
			want:    100 * time.Millisecond, // Always stays at initial
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CalculateBackoff(tt.attempt, tt.config)
			if got != tt.want {
				t.Errorf("CalculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestErrMaxReconnectAttempts(t *testing.T) {
	t.Parallel()

	// Verify the error is defined and has expected message
	if ErrMaxReconnectAttempts == nil {
		t.Fatal("ErrMaxReconnectAttempts is nil")
	}

	expected := "maximum reconnection attempts reached"
	if ErrMaxReconnectAttempts.Error() != expected {
		t.Errorf("ErrMaxReconnectAttempts.Error() = %q, want %q", ErrMaxReconnectAttempts.Error(), expected)
	}
}

func TestOnReconnectFunc(t *testing.T) {
	t.Parallel()

	// Test that OnReconnectFunc can be assigned and called
	var called bool
	var receivedAttempts int

	var fn OnReconnectFunc = func(attempts int) {
		called = true
		receivedAttempts = attempts
	}

	fn(5)

	if !called {
		t.Error("OnReconnectFunc was not called")
	}
	if receivedAttempts != 5 {
		t.Errorf("receivedAttempts = %d, want 5", receivedAttempts)
	}
}

func TestOnDisconnectFunc(t *testing.T) {
	t.Parallel()

	// Test that OnDisconnectFunc can be assigned and called
	var called bool
	var receivedErr error

	var fn OnDisconnectFunc = func(err error) {
		called = true
		receivedErr = err
	}

	testErr := errors.New("test error")
	fn(testErr)

	if !called {
		t.Error("OnDisconnectFunc was not called")
	}
	if !errors.Is(receivedErr, testErr) {
		t.Errorf("receivedErr = %v, want %v", receivedErr, testErr)
	}
}

func TestReconnectManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  1 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		MaxAttempts:   0,
	}

	mgr := NewReconnectManager(config)

	// Run multiple goroutines accessing the manager concurrently
	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func() {
			_ = mgr.ShouldReconnect()
			_ = mgr.GetAttempts()
			_ = mgr.GetCurrentDelay()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// No race conditions should have occurred (test with -race flag)
}

func TestReconnectManager_ResetDuringWait(t *testing.T) {
	t.Parallel()

	config := ReconnectConfig{
		InitialDelay:  500 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		MaxAttempts:   0,
	}

	mgr := NewReconnectManager(config)

	// Start a wait in a goroutine
	go func() {
		_ = mgr.WaitForReconnect(context.Background())
	}()

	// Give time for the wait to start
	time.Sleep(50 * time.Millisecond)

	// Reset should work without deadlock
	mgr.Reset()

	// Verify reset worked
	if mgr.GetAttempts() != 0 {
		t.Errorf("GetAttempts() after Reset = %v, want 0", mgr.GetAttempts())
	}
}
