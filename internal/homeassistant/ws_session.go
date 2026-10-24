package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/coder/websocket"
)

// errConnectionLost is delivered to requests that were waiting for a response
// when their connection ended. The text deliberately contains "connection
// closed" so IsRetryableError treats it as retryable - which wsRetryPredicate
// honors for replay-safe commands only; any other command surfaces it wrapped
// in errOutcomeUnknown instead of being re-sent.
var errConnectionLost = errors.New("connection closed while waiting for response")

// wsResponse is the single outcome delivered to a pending request: either a
// result message or an error.
type wsResponse struct {
	msg *WSResultMessage
	err error
}

// wsSession owns one authenticated websocket connection and the map of
// requests waiting on it.
//
// Invariant: a pending entry is deleted under mu in the same critical section
// that sends to its channel, and channels are never closed. Each channel
// therefore receives at most one value and no send can hit a closed channel.
type wsSession struct {
	conn    *websocket.Conn
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	endOnce sync.Once

	mu      sync.Mutex
	pending map[int64]chan wsResponse
	ended   bool
}

// newWSSession creates a session for conn. conn may be nil (unit tests).
func newWSSession(parent context.Context, conn *websocket.Conn) *wsSession {
	ctx, cancel := context.WithCancel(parent)
	return &wsSession{
		conn:    conn,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
		pending: make(map[int64]chan wsResponse),
	}
}

// register adds a pending request and returns the channel its response will
// arrive on. After the session ended it fails with a not-sent error, since
// nothing has been written to the wire yet.
func (s *wsSession) register(id int64) (<-chan wsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return nil, newNotSentError(errConnectionLost)
	}
	ch := make(chan wsResponse, 1)
	s.pending[id] = ch
	return ch, nil
}

// unregister removes a pending request without delivering anything.
func (s *wsSession) unregister(id int64) {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
}

// deliver sends msg to the request registered under id. Unknown ids are
// dropped silently (late response after timeout or unregister).
func (s *wsSession) deliver(id int64, msg *WSResultMessage) {
	s.finish(id, wsResponse{msg: msg})
}

// expire fails the request registered under id with ErrRequestTimeout.
func (s *wsSession) expire(id int64) {
	s.finish(id, wsResponse{err: ErrRequestTimeout})
}

// finish removes the pending entry and sends resp under one lock hold. The
// channel has capacity 1 and receives only this one value, so the send never
// blocks; select/default keeps that true even if the invariant were broken.
func (s *wsSession) finish(id int64, resp wsResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.pending[id]
	if !ok {
		return
	}
	delete(s.pending, id)
	select {
	case ch <- resp:
	default:
	}
}

// end terminates the session exactly once: it fails every pending request
// with errConnectionLost, cancels the session context, closes the connection
// and closes done. It returns true only for the call that actually ended it.
func (s *wsSession) end(code websocket.StatusCode, reason string) bool {
	first := false
	s.endOnce.Do(func() {
		first = true
		s.failAllPending()
		s.cancel()
		if s.conn != nil {
			// Network I/O happens outside mu.
			_ = s.conn.Close(code, reason)
		}
		close(s.done)
	})
	return first
}

// failAllPending marks the session ended and fails every waiting request.
func (s *wsSession) failAllPending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ended = true
	for id, ch := range s.pending {
		delete(s.pending, id)
		select {
		case ch <- wsResponse{err: errConnectionLost}:
		default:
		}
	}
}

// handleResultMessage routes a raw result frame to its waiting request.
// Malformed frames are ignored.
func (s *wsSession) handleResultMessage(data []byte) {
	id, err := ParseMessageID(data)
	if err != nil {
		return
	}
	var msg WSResultMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	s.deliver(id, &msg)
}
