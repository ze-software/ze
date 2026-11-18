// Design: docs/architecture/l2tp/subscriber-session-model.md -- original-session disconnect audit

package l2tp

import "time"

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
	captured.observer.recordEvent(ObserverEvent{
		Timestamp: time.Now(), Type: ObserverEventDisconnectRequested,
		TunnelID: captured.tunnel.localTID, SessionID: captured.session.localSID,
		Actor: actor, Reason: reason, Cause: cause,
	})
	return nil
}
