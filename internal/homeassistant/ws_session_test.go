package homeassistant

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/coder/websocket"
)

func mustRegister(t *testing.T, s *wsSession, id int64) <-chan wsResponse {
	t.Helper()
	ch, err := s.register(id)
	if err != nil {
		t.Fatalf("register(%d): %v", id, err)
	}
	return ch
}

func TestWSSession_DeliverRoutesToRegisteredID(t *testing.T) {
	t.Parallel()
	s := newWSSession(context.Background(), nil)
	ch1 := mustRegister(t, s, 1)
	ch2 := mustRegister(t, s, 2)

	msg := &WSResultMessage{ID: 2, Success: true}
	s.deliver(2, msg)
	s.deliver(99, &WSResultMessage{ID: 99}) // unknown id: dropped, no panic

	select {
	case r := <-ch2:
		if r.err != nil || r.msg != msg {
			t.Fatalf("unexpected response: %+v", r)
		}
	default:
		t.Fatal("channel 2 has no value")
	}
	if len(ch1) != 0 {
		t.Fatalf("channel 1 should be empty, has %d", len(ch1))
	}
}

func TestWSSession_ExpireSendsTimeout(t *testing.T) {
	t.Parallel()
	s := newWSSession(context.Background(), nil)
	ch := mustRegister(t, s, 1)

	s.expire(1)
	r := <-ch
	if !errors.Is(r.err, ErrRequestTimeout) {
		t.Fatalf("err = %v, want ErrRequestTimeout", r.err)
	}

	s.deliver(1, &WSResultMessage{ID: 1})
	if len(ch) != 0 {
		t.Fatal("late deliver must be dropped")
	}
}

func TestWSSession_EndFailsAllPending(t *testing.T) {
	t.Parallel()
	s := newWSSession(context.Background(), nil)
	chans := make([]<-chan wsResponse, 0, 5)
	for i := int64(1); i <= 5; i++ {
		chans = append(chans, mustRegister(t, s, i))
	}

	if !s.end(1000, "bye") {
		t.Fatal("first end should return true")
	}
	if s.end(1000, "bye") {
		t.Fatal("second end should return false")
	}
	for i, ch := range chans {
		r := <-ch
		if !errors.Is(r.err, errConnectionLost) {
			t.Fatalf("chan %d err = %v, want errConnectionLost", i, r.err)
		}
	}
	select {
	case <-s.done:
	default:
		t.Fatal("done should be closed")
	}
	if s.ctx.Err() == nil {
		t.Fatal("ctx should be canceled")
	}
}

func TestWSSession_RegisterAfterEnd(t *testing.T) {
	t.Parallel()
	s := newWSSession(context.Background(), nil)
	s.end(websocket.StatusGoingAway, "bye")

	_, err := s.register(1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.As(err, new(*notSentError)) {
		t.Fatalf("err %v is not a notSentError", err)
	}
	if !errors.Is(err, errConnectionLost) {
		t.Fatalf("err %v does not wrap errConnectionLost", err)
	}
}

func TestWSSession_UnregisterDropsEntry(t *testing.T) {
	t.Parallel()
	s := newWSSession(context.Background(), nil)
	ch := mustRegister(t, s, 1)

	s.unregister(1)
	s.unregister(1) // idempotent
	s.deliver(1, &WSResultMessage{ID: 1})
	s.end(1000, "bye")
	if len(ch) != 0 {
		t.Fatal("unregistered channel must never receive")
	}
}

func TestWSSession_HandleResultMessage(t *testing.T) {
	t.Parallel()
	s := newWSSession(context.Background(), nil)
	ch := mustRegister(t, s, 7)

	s.handleResultMessage([]byte("garbage"))
	s.handleResultMessage([]byte(`{"id":"x"}`))
	if len(ch) != 0 {
		t.Fatal("malformed data must be ignored")
	}

	s.handleResultMessage([]byte(`{"id":7,"type":"result","success":true,"result":{}}`))
	r := <-ch
	if r.err != nil || r.msg == nil || r.msg.ID != 7 || !r.msg.Success {
		t.Fatalf("unexpected response: %+v", r)
	}
}

func TestWSSession_DeliverExpireEndRace(t *testing.T) {
	t.Parallel()
	for i := 0; i < 2000; i++ {
		s := newWSSession(context.Background(), nil)
		id := int64(i + 1)
		ch, err := s.register(id)
		if err != nil {
			t.Fatalf("iteration %d: register: %v", i, err)
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); s.deliver(id, &WSResultMessage{ID: id}) }()
		go func() { defer wg.Done(); s.expire(id) }()
		if i%100 == 0 {
			wg.Add(1)
			go func() { defer wg.Done(); s.end(1000, "race") }()
		}
		wg.Wait()

		if got := len(ch); got != 1 {
			t.Fatalf("iteration %d: channel holds %d values, want exactly 1", i, got)
		}
		<-ch
		// A straggler would only show up as a second value.
		s.deliver(id, &WSResultMessage{ID: id})
		s.expire(id)
		s.end(1000, "cleanup")
		if got := len(ch); got != 0 {
			t.Fatalf("iteration %d: channel received a second value", i)
		}
	}
}
