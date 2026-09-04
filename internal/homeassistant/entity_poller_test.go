package homeassistant

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// fastConfig returns a poller config with short intervals suitable for unit tests.
func fastConfig() EntityPollerConfig {
	return EntityPollerConfig{
		Timeout:      500 * time.Millisecond,
		PollInterval: 10 * time.Millisecond,
	}
}

func TestWaitForEntityAppear_ImmediateAppear(t *testing.T) {
	entity := &Entity{EntityID: "light.living_room", State: "on"}
	fn := func(context.Context, string) (*Entity, error) {
		return entity, nil
	}

	got, ok := WaitForEntityAppear(context.Background(), fn, "light.living_room", fastConfig())
	if !ok {
		t.Fatal("expected entity to appear")
	}
	if got.EntityID != "light.living_room" {
		t.Errorf("expected entity_id light.living_room, got %s", got.EntityID)
	}
}

func TestWaitForEntityAppear_DelayedAppear(t *testing.T) {
	var callCount int32

	fn := func(context.Context, string) (*Entity, error) {
		n := atomic.AddInt32(&callCount, 1)
		if n < 5 {
			return nil, errors.New("not found")
		}
		return &Entity{EntityID: "light.living_room", State: "on"}, nil
	}

	_, ok := WaitForEntityAppear(context.Background(), fn, "light.living_room", fastConfig())
	if !ok {
		t.Fatal("expected entity to appear after delay")
	}
	if atomic.LoadInt32(&callCount) < 5 {
		t.Errorf("expected at least 5 poll attempts, got %d", callCount)
	}
}

func TestWaitForEntityAppear_Timeout(t *testing.T) {
	fn := func(context.Context, string) (*Entity, error) {
		return nil, errors.New("not found")
	}

	cfg := EntityPollerConfig{Timeout: 50 * time.Millisecond, PollInterval: 10 * time.Millisecond}
	_, ok := WaitForEntityAppear(context.Background(), fn, "light.living_room", cfg)
	if ok {
		t.Fatal("expected timeout (false), but got true")
	}
}

func TestWaitForEntityAppear_ContextCancelled(t *testing.T) {
	fn := func(context.Context, string) (*Entity, error) {
		return nil, errors.New("not found")
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, ok := WaitForEntityAppear(ctx, fn, "light.living_room", fastConfig())
	if ok {
		t.Fatal("expected false when context is canceled")
	}
}

func TestWaitForEntityDisappear_ImmediateGone(t *testing.T) {
	// Entity not found on every poll — requires notFoundThreshold consecutive errors.
	fn := func(context.Context, string) (*Entity, error) {
		return nil, errors.New("not found")
	}

	ok := WaitForEntityDisappear(context.Background(), fn, "light.living_room", fastConfig())
	if !ok {
		t.Fatal("expected entity to be confirmed gone after consecutive not-found errors")
	}
}

func TestWaitForEntityDisappear_SingleErrorNotEnough(t *testing.T) {
	// One error followed by entity appearing again should NOT confirm disappearance.
	var callCount int32

	fn := func(context.Context, string) (*Entity, error) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			return nil, errors.New("transient error")
		}
		// Entity comes back — resets the counter
		return &Entity{EntityID: "light.living_room"}, nil
	}

	cfg := EntityPollerConfig{Timeout: 100 * time.Millisecond, PollInterval: 10 * time.Millisecond}
	ok := WaitForEntityDisappear(context.Background(), fn, "light.living_room", cfg)
	if ok {
		t.Error("single transient error should not confirm disappearance")
	}
}

func TestWaitForEntityDisappear_DelayedGone(t *testing.T) {
	var callCount int32

	fn := func(context.Context, string) (*Entity, error) {
		n := atomic.AddInt32(&callCount, 1)
		if n < 5 {
			return &Entity{EntityID: "light.living_room"}, nil
		}
		return nil, errors.New("not found")
	}

	ok := WaitForEntityDisappear(context.Background(), fn, "light.living_room", fastConfig())
	if !ok {
		t.Fatal("expected entity to disappear after delay")
	}
}

func TestWaitForEntityDisappear_Timeout(t *testing.T) {
	fn := func(context.Context, string) (*Entity, error) {
		return &Entity{EntityID: "light.living_room"}, nil
	}

	cfg := EntityPollerConfig{Timeout: 50 * time.Millisecond, PollInterval: 10 * time.Millisecond}
	ok := WaitForEntityDisappear(context.Background(), fn, "light.living_room", cfg)
	if ok {
		t.Fatal("expected timeout (false), but got true")
	}
}

func TestWaitForConfigEntryEntity_ImmediateMatch(t *testing.T) {
	fn := func(context.Context) ([]EntityRegistryEntry, error) {
		return []EntityRegistryEntry{
			{EntityID: "sensor.other", ConfigEntryID: "entry_other"},
			{EntityID: "light.my_switch", ConfigEntryID: "entry_123"},
		}, nil
	}

	got, ok := WaitForConfigEntryEntity(context.Background(), fn, "entry_123", fastConfig())
	if !ok {
		t.Fatal("expected entity to be found")
	}
	if got != "light.my_switch" {
		t.Errorf("expected light.my_switch, got %s", got)
	}
}

func TestWaitForConfigEntryEntity_DelayedMatch(t *testing.T) {
	var callCount int32

	fn := func(context.Context) ([]EntityRegistryEntry, error) {
		n := atomic.AddInt32(&callCount, 1)
		if n < 5 {
			return []EntityRegistryEntry{{EntityID: "sensor.other", ConfigEntryID: "entry_other"}}, nil
		}
		return []EntityRegistryEntry{{EntityID: "cover.my_switch", ConfigEntryID: "entry_123"}}, nil
	}

	got, ok := WaitForConfigEntryEntity(context.Background(), fn, "entry_123", fastConfig())
	if !ok {
		t.Fatal("expected entity to appear after delay")
	}
	if got != "cover.my_switch" {
		t.Errorf("expected cover.my_switch, got %s", got)
	}
	if atomic.LoadInt32(&callCount) < 5 {
		t.Errorf("expected at least 5 poll attempts, got %d", callCount)
	}
}

func TestWaitForConfigEntryEntity_Timeout(t *testing.T) {
	fn := func(context.Context) ([]EntityRegistryEntry, error) {
		return []EntityRegistryEntry{{EntityID: "sensor.other", ConfigEntryID: "entry_other"}}, nil
	}

	cfg := EntityPollerConfig{Timeout: 50 * time.Millisecond, PollInterval: 10 * time.Millisecond}
	got, ok := WaitForConfigEntryEntity(context.Background(), fn, "entry_123", cfg)
	if ok {
		t.Fatal("expected timeout (false), but got true")
	}
	if got != "" {
		t.Errorf("expected empty string on timeout, got %s", got)
	}
}

func TestWaitForConfigEntryEntity_RegistryErrorIsTreatedAsMiss(t *testing.T) {
	fn := func(context.Context) ([]EntityRegistryEntry, error) {
		return nil, errors.New("registry unavailable")
	}

	cfg := EntityPollerConfig{Timeout: 50 * time.Millisecond, PollInterval: 10 * time.Millisecond}
	got, ok := WaitForConfigEntryEntity(context.Background(), fn, "entry_123", cfg)
	if ok {
		t.Fatal("expected timeout (false) on persistent registry error")
	}
	if got != "" {
		t.Errorf("expected empty string, got %s", got)
	}
}

func TestWaitForConfigEntryEntity_ContextCancelled(t *testing.T) {
	fn := func(context.Context) ([]EntityRegistryEntry, error) {
		return []EntityRegistryEntry{{EntityID: "sensor.other", ConfigEntryID: "entry_other"}}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	got, ok := WaitForConfigEntryEntity(ctx, fn, "entry_123", fastConfig())
	if ok {
		t.Fatal("expected false when context is canceled")
	}
	if got != "" {
		t.Errorf("expected empty string, got %s", got)
	}
}

func TestWaitForEntityDisappear_ContextCancelled(t *testing.T) {
	fn := func(context.Context, string) (*Entity, error) {
		return &Entity{EntityID: "light.living_room"}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	ok := WaitForEntityDisappear(ctx, fn, "light.living_room", fastConfig())
	if ok {
		t.Fatal("expected false when context is canceled")
	}
}
