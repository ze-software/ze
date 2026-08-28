// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- RADIUS session/idle timeout
// Related: reactor.go -- handleSessionUp starts timers, teardown cancels them
// Related: session.go -- L2TPSession holds cancel funcs
// Related: session_metadata.go -- AuthMetadata carries timeout values

package l2tp

import (
	"context"
	"time"
)

// startSessionTimeouts reads RADIUS metadata for the session and starts
// timeout goroutines if Session-Timeout or Idle-Timeout are present.
// Called from handleSessionUp after the session-up event is emitted.
// Caller MUST NOT hold tunnelsMu.
//
// RFC 2865 Section 5.27: Session-Timeout is the maximum session
// duration in seconds. The session is forcibly disconnected on expiry.
//
// RFC 2865 Section 5.28: Idle-Timeout is the maximum idle time in
// seconds. The session is disconnected when no traffic has been
// observed for this duration.
func (r *l2tpReactor) startSessionTimeouts(tid, sid uint16) {
	meta := LoadSessionMetadata(tid, sid)
	if meta == nil {
		return
	}

	r.tunnelsMu.Lock()
	tunnel, ok := r.tunnelsByLocalID[tid]
	if !ok {
		r.tunnelsMu.Unlock()
		return
	}
	sess := tunnel.lookupSession(sid)
	if sess == nil {
		r.tunnelsMu.Unlock()
		return
	}

	if meta.SessionTimeout > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		sess.sessionTimeoutCancel = cancel
		go r.runSessionTimeout(tid, sid, time.Duration(meta.SessionTimeout)*time.Second, ctx)
	}

	if meta.IdleTimeout > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		sess.idleTimeoutCancel = cancel
		iface := sess.pppInterface
		go r.runIdleTimeout(tid, sid, time.Duration(meta.IdleTimeout)*time.Second, iface, ctx)
	}
	r.tunnelsMu.Unlock()
}

// runSessionTimeout waits for the session timeout duration then tears
// down the session. Exits early if ctx is canceled (session torn down
// by other means).
func (r *l2tpReactor) runSessionTimeout(tid, sid uint16, timeout time.Duration, ctx context.Context) {
	select {
	case <-time.After(timeout):
		r.logger.Info("l2tp: session-timeout expired; tearing down session",
			"tunnel-id", tid, "session-id", sid, "timeout", timeout)
		if err := r.teardownSessionByID(sid); err != nil {
			r.logger.Debug("l2tp: session-timeout teardown (session may already be gone)",
				"session-id", sid, "error", err)
		}
	case <-ctx.Done():
	}
}

// runIdleTimeout periodically checks whether the session's pppN
// interface has seen traffic. If no new bytes have been received for
// the idle timeout duration, the session is torn down.
//
// Traffic detection uses kernel interface RX byte counters via
// readIfaceRXBytes (netlink on Linux, zero on other platforms).
// When counters are unavailable (non-Linux, interface gone), the
// timer fires unconditionally after the idle period.
func (r *l2tpReactor) runIdleTimeout(tid, sid uint16, idleTimeout time.Duration, iface string, ctx context.Context) {
	lastRX := readIfaceRXBytes(iface)
	ticker := time.NewTicker(idleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentRX := readIfaceRXBytes(iface)
			if currentRX > lastRX {
				lastRX = currentRX
				continue
			}
			r.logger.Info("l2tp: idle-timeout expired; tearing down session",
				"tunnel-id", tid, "session-id", sid, "idle-timeout", idleTimeout, "interface", iface)
			if err := r.teardownSessionByID(sid); err != nil {
				r.logger.Debug("l2tp: idle-timeout teardown (session may already be gone)",
					"session-id", sid, "error", err)
			}
			return
		case <-ctx.Done():
			return
		}
	}
}

// cancelSessionTimeouts cancels any active timeout goroutines for the
// session. Safe to call with nil cancel funcs. Called from session
// teardown paths while holding tunnelsMu.
func cancelSessionTimeouts(sess *L2TPSession) {
	if sess.sessionTimeoutCancel != nil {
		sess.sessionTimeoutCancel()
		sess.sessionTimeoutCancel = nil
	}
	if sess.idleTimeoutCancel != nil {
		sess.idleTimeoutCancel()
		sess.idleTimeoutCancel = nil
	}
}
