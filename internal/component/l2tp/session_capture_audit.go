// Design: docs/architecture/l2tp/subscriber-session-model.md -- original-session disconnect audit

package l2tp

// RecordDisconnect records the request only while this exact original row is
// current. A cleanup retry for an already removed original records nothing and
// remains eligible for Teardown. Safe for concurrent use.
func (captured *capturedSession) RecordDisconnect(actor, reason string, cause uint32) error {
	if captured == nil {
		return ErrSessionNotFound
	}
	if captured.reactor == nil {
		return ErrSessionNotFound
	}
	captured.reactor.tunnelsMu.Lock()
	defer captured.reactor.tunnelsMu.Unlock()
	if captured.teardownStarted.Load() {
		return nil
	}
	if !captured.currentLocked() {
		return ErrSessionNotFound
	}
	if captured.observer == nil {
		return nil
	}
	captured.observer.recordDisconnect(captured.generation, captured.birth,
		captured.tunnel.localTID, captured.session.localSID, actor, reason, cause)
	return nil
}
