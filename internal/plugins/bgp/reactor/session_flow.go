// Design: docs/architecture/core-design.md — BGP session backpressure flow control

package reactor

import "context"

// Pause stops the read loop from calling readAndProcessMessage.
// The read loop blocks on a resume signal or context cancel.
// Write path (KEEPALIVE timer) is independent and continues during pause.
// Idempotent: calling Pause on an already-paused session is a no-op.
// Logging is handled by the caller (Peer.PauseReading) which has peer context.
func (s *Session) Pause() {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()

	if s.paused.Load() {
		return
	}

	s.resumeCh = make(chan struct{})
	s.paused.Store(true)
}

// Resume unblocks the read loop after a Pause.
// Idempotent: calling Resume on a non-paused session is a no-op.
// Also called by the cancel goroutine during shutdown to unblock the pause gate.
func (s *Session) Resume() {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()

	if !s.paused.Load() {
		return
	}

	s.paused.Store(false)
	close(s.resumeCh)
	s.resumeCh = nil
}

// IsPaused reports whether the session's read loop is paused.
func (s *Session) IsPaused() bool {
	return s.paused.Load()
}

// waitForResume blocks until Resume() is called or context is canceled.
// The cancel goroutine handles errChan and calls Resume() to unblock us.
// Returns nil to continue reading, or an error to exit Run().
func (s *Session) waitForResume(ctx context.Context) error {
	s.pauseMu.Lock()
	ch := s.resumeCh
	s.pauseMu.Unlock()

	if ch == nil {
		return nil
	}

	select {
	case <-ch:
		// Unblocked by Resume(). Check if it was a real resume or
		// the cancel goroutine unblocking us after shutdown.
		if reason := s.closeReason.Load(); reason != nil {
			return *reason
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
