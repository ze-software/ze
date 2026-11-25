// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- RADIUS session/idle timeout
// Related: reactor.go -- handleSessionUp starts timers, teardown cancels them
// Related: session.go -- L2TPSession holds cancel funcs
// Related: session_metadata.go -- AuthMetadata carries timeout values

package l2tp

import (
	"context"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// sessionTimeoutLifetime identifies one network lifetime on a retained PPP
// attempt. Its nonzero size gives each live allocation a distinct identity.
// A canceled lifetime cannot arm timers after its Up readers return.
type sessionTimeoutLifetime struct {
	_        byte
	network  l2tpevents.NetworkLifetime
	counters ppp.NetworkCounterReceipt
}

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
func (r *l2tpReactor) startSessionTimeouts(tunnel *L2TPTunnel, sess *L2TPSession, lifetime *sessionTimeoutLifetime) {
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	if tunnel == nil {
		return
	}
	if sess == nil {
		return
	}
	if r.tunnelsByLocalID[tunnel.localTID] != tunnel {
		return
	}
	if tunnel.lookupSession(sess.localSID) != sess {
		return
	}
	if lifetime == nil {
		return
	}
	if sess.timeoutLifetime != lifetime {
		return
	}
	meta := LoadSessionMetadata(sess.SubscriberID())
	if meta == nil {
		return
	}

	if meta.SessionTimeout > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		if sess.sessionTimeoutCancel != nil {
			sess.sessionTimeoutCancel()
		}
		sess.sessionTimeoutCancel = cancel
		sess.sessionTimeoutContext = ctx
		go r.runSessionTimeout(tunnel, sess, time.Duration(meta.SessionTimeout)*time.Second, ctx)
	}

	if meta.IdleTimeout > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		if sess.idleTimeoutCancel != nil {
			sess.idleTimeoutCancel()
		}
		sess.idleTimeoutCancel = cancel
		sess.idleTimeoutContext = ctx
		iface := sess.pppInterface
		go r.runIdleTimeout(tunnel, sess, time.Duration(meta.IdleTimeout)*time.Second, iface, ctx)
	}
}

// runSessionTimeout waits for the session timeout duration then tears
// down the session. Exits early if ctx is canceled (session torn down
// by other means).
func (r *l2tpReactor) runSessionTimeout(tunnel *L2TPTunnel, sess *L2TPSession, timeout time.Duration, ctx context.Context) {
	tid, sid := tunnel.localTID, sess.localSID
	select {
	case <-time.After(timeout):
		r.logger.Info("l2tp: session-timeout expired; tearing down session",
			reportFieldTunnelID, tid, reportFieldSessionID, sid, "timeout", timeout)
		// RFC 2866 Section 5.10 value 5: "Maximum session length timer
		// expired."
		if !r.teardownCapturedSession(tunnel, sess, ctx, cdnResultAdministrative, l2tpevents.TerminateCauseSessionTimeout) {
			r.logger.Debug("l2tp: session-timeout teardown (session may already be gone)",
				reportFieldSessionID, sid)
		}
	case <-ctx.Done():
	}
}

// runIdleTimeout periodically checks whether the session's pppN
// interface has seen traffic. If no new bytes have been received for
// the idle timeout duration, the session is torn down.
//
// Traffic detection uses kernel interface RX byte counters via
// readIfaceRXBytes (rtnetlink on Linux, zero on other platforms).
// When counters are unavailable (non-Linux, interface gone), the
// timer fires unconditionally after the idle period.
func (r *l2tpReactor) runIdleTimeout(tunnel *L2TPTunnel, sess *L2TPSession, idleTimeout time.Duration, iface string, ctx context.Context) {
	tid, sid := tunnel.localTID, sess.localSID
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
				reportFieldTunnelID, tid, reportFieldSessionID, sid, "idle-timeout", idleTimeout, "interface", iface)
			// RFC 2866 Section 5.10 value 4: "Idle timer expired."
			if !r.teardownCapturedSession(tunnel, sess, ctx, cdnResultAdministrative, l2tpevents.TerminateCauseIdleTimeout) {
				r.logger.Debug("l2tp: idle-timeout teardown (session may already be gone)",
					reportFieldSessionID, sid)
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
	sess.timeoutLifetime = nil
	sess.sessionTimeoutContext = nil
	sess.idleTimeoutContext = nil
	if sess.sessionTimeoutCancel != nil {
		sess.sessionTimeoutCancel()
		sess.sessionTimeoutCancel = nil
	}
	if sess.idleTimeoutCancel != nil {
		sess.idleTimeoutCancel()
		sess.idleTimeoutCancel = nil
	}
}
