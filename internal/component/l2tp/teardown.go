// Design: docs/guide/l2tp.md -- operator-initiated teardown
// RFC: rfc/short/rfc2661.md -- StopCCN and CDN Result Codes
// RFC: rfc/short/rfc2866.md -- Acct-Terminate-Cause named by each teardown
// Related: reactor.go -- owns tunnelsByLocalID and tunnelsByPeer
// Related: snapshot.go -- read-side API, sibling to this write-side API

package l2tp

import (
	"errors"
	"fmt"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// Operator-initiated teardown errors. CLI handlers translate these into
// plugin.StatusError responses with the wrapped message.
var (
	// ErrTunnelNotFound is returned by TeardownTunnelByID when no
	// tunnel is registered under the requested local TID.
	ErrTunnelNotFound = errors.New("l2tp: tunnel not found")
	// ErrSessionNotFound is returned by TeardownSessionByID when no
	// session with the requested local SID exists on any tunnel.
	ErrSessionNotFound = errors.New("l2tp: session not found")
	// ErrInvalidID is returned when the caller supplies SID or TID 0,
	// which are reserved per RFC 2661 and never allocated.
	ErrInvalidID = errors.New("l2tp: invalid id (must be 1..65535)")
)

// tornSession is one session a tunnel teardown removed. The teardown reads it
// out of the tunnel map before that map is cleared, so the notifications below
// the unlock still know which sessions ended and who was on them.
type tornSession struct {
	localSID uint16
	username string
}

// teardownTunnelByID sends a StopCCN Result Code 6 (administrative
// shutdown) for the tunnel with the given local TID. The tunnel's
// sessions are cleared and any kernel resources drained the same way
// peer-initiated StopCCN does. Returns ErrTunnelNotFound when the TID
// is unknown.
//
// Every session the tunnel carried gets one (l2tp, session-down) naming RFC
// 2866 Section 5.10 value 6, "Administrator reset the port or session". Both
// callers are operator clears: Subsystem.TeardownTunnel behind `clear l2tp
// tunnel id <tid>`, and TeardownAllTunnels behind `clear l2tp tunnel all`. A
// caller that ends a tunnel for another reason MUST take a cause parameter
// here rather than inherit this one, the way teardownSessionByID does for the
// two RADIUS timers.
//
// Caller MUST NOT hold tunnelsMu; this method acquires it internally
// and releases it before writing to the UDP socket (matching the
// existing reactor pattern).
func (r *l2tpReactor) teardownTunnelByID(localTID uint16) error {
	if localTID == 0 {
		return ErrInvalidID
	}
	r.tunnelsMu.Lock()
	t, ok := r.tunnelsByLocalID[localTID]
	if !ok {
		r.tunnelsMu.Unlock()
		return fmt.Errorf("%w: local-tid=%d", ErrTunnelNotFound, localTID)
	}
	// Collect each session before teardownStopCCN clears the session map, so
	// the route observer and the event bus both hear about every session that
	// was live when the operator requested the teardown. The username is read
	// in this same pass because it lives only on the session struct the clear
	// is about to drop, and an empty one reaches RADIUS as a Stop record with
	// no User-Name.
	torn := make([]tornSession, 0, len(t.sessions))
	for sid, sess := range t.sessions {
		torn = append(torn, tornSession{localSID: sid, username: sess.username})
	}
	now := r.params.Clock()
	outbound := t.teardownStopCCN(now, resultAdministrative)
	teardowns := t.drainPendingKernelTeardowns()
	r.tunnelsMu.Unlock()

	if r.routeObserver != nil {
		for _, s := range torn {
			r.routeObserver.OnSessionDown(localTID, s.localSID)
		}
	}
	for _, s := range torn {
		r.emitSessionDown(localTID, s.localSID, s.username, l2tpevents.TerminateCauseAdminReset)
		ClearSessionMetadata(localTID, s.localSID)
	}
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (operator tunnel teardown)",
				"to", req.to.String(), "error", err.Error())
		}
	}
	r.enqueueKernelEvents(nil, teardowns)
	return nil
}

// teardownSessionByID sends a CDN Result Code 3 (administrative) for
// the first session with the given local SID found on any tunnel.
// Returns ErrSessionNotFound when no session carries the SID.
//
// cause is why the caller is ending this session, and it reaches RADIUS
// accounting as Acct-Terminate-Cause. Every caller MUST name one that is true
// of its own path: the timers name their timer, the operator and CoA paths
// name Admin Reset.
//
// Caller MUST NOT hold tunnelsMu.
func (r *l2tpReactor) teardownSessionByID(localSID uint16, cause l2tpevents.TerminateCause) error {
	if localSID == 0 {
		return ErrInvalidID
	}
	r.tunnelsMu.Lock()
	var (
		tunnel *L2TPTunnel
		sess   *L2TPSession
	)
	for _, t := range r.tunnelsByLocalID {
		if s, ok := t.sessions[localSID]; ok {
			tunnel = t
			sess = s
			break
		}
	}
	if sess == nil {
		r.tunnelsMu.Unlock()
		return fmt.Errorf("%w: local-sid=%d", ErrSessionNotFound, localSID)
	}
	cancelSessionTimeouts(sess)
	tid := tunnel.localTID
	username := sess.username
	now := r.params.Clock()
	outbound := tunnel.teardownSession(sess, cdnResultAdministrative, now, r.logger)
	teardowns := tunnel.drainPendingKernelTeardowns()
	r.tunnelsMu.Unlock()

	if r.routeObserver != nil {
		r.routeObserver.OnSessionDown(tid, localSID)
	}
	r.emitSessionDown(tid, localSID, username, cause)
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (operator session teardown)",
				"to", req.to.String(), "error", err.Error())
		}
	}
	r.enqueueKernelEvents(nil, teardowns)
	ClearSessionMetadata(tid, localSID)
	return nil
}

// TeardownAllTunnels sends an administrative StopCCN to every tunnel.
// Returns the count of tunnels actually torn down. Idempotent: calling
// with zero live tunnels returns 0 and is not an error.
//
// Caller MUST NOT hold tunnelsMu.
func (r *l2tpReactor) TeardownAllTunnels() int {
	r.tunnelsMu.Lock()
	tids := make([]uint16, 0, len(r.tunnelsByLocalID))
	for tid := range r.tunnelsByLocalID {
		tids = append(tids, tid)
	}
	r.tunnelsMu.Unlock()

	n := 0
	for _, tid := range tids {
		if err := r.teardownTunnelByID(tid); err == nil {
			n++
		} else if !errors.Is(err, ErrTunnelNotFound) {
			r.logger.Warn("l2tp: teardown-all per-tunnel failure",
				"local-tid", tid, "error", err.Error())
		}
	}
	return n
}

// TeardownAllSessions sends an administrative CDN to every session on
// every tunnel. Tunnels themselves are left in place. Returns the count
// of sessions actually torn down.
//
// Caller MUST NOT hold tunnelsMu.
func (r *l2tpReactor) TeardownAllSessions() int {
	r.tunnelsMu.Lock()
	type key struct {
		tid uint16
		sid uint16
	}
	keys := make([]key, 0)
	for tid, t := range r.tunnelsByLocalID {
		for sid := range t.sessions {
			keys = append(keys, key{tid: tid, sid: sid})
		}
	}
	r.tunnelsMu.Unlock()

	n := 0
	for _, k := range keys {
		// An operator clearing every session is RFC 2866 Section 5.10 value 6:
		// "Administrator reset the port or session."
		if err := r.teardownSessionOnTunnel(k.tid, k.sid, l2tpevents.TerminateCauseAdminReset); err == nil {
			n++
		}
	}
	return n
}

// emitSessionDown publishes (l2tp, session-down) for a session the reactor
// itself tore down. The PPP-driven path emits from handlePPPEvent; a locally
// initiated teardown removes the session from the tunnel map first, so that
// path finds nothing and returns, and this is the only emission the
// subscribers of the event get. Exactly one of the two fires, because both
// look the session up under tunnelsMu before emitting.
//
// Caller MUST NOT hold tunnelsMu.
func (r *l2tpReactor) emitSessionDown(localTID, localSID uint16, username string, cause l2tpevents.TerminateCause) {
	if r.eventBus == nil {
		return
	}
	if _, err := l2tpevents.SessionDown.Emit(r.eventBus, &l2tpevents.SessionDownPayload{
		TunnelID:  localTID,
		SessionID: localSID,
		Username:  username,
		Cause:     cause,
	}); err != nil {
		r.logger.Warn("l2tp: session-down emit failed", "error", err)
	}
}

// teardownSessionOnTunnel is the tunnel-scoped variant used by
// TeardownAllSessions. Distinct from TeardownSessionByID because the
// latter walks every tunnel looking for the SID; here the caller
// already knows the tuple.
//
// cause carries the same obligation teardownSessionByID states.
func (r *l2tpReactor) teardownSessionOnTunnel(localTID, localSID uint16, cause l2tpevents.TerminateCause) error {
	r.tunnelsMu.Lock()
	t, ok := r.tunnelsByLocalID[localTID]
	if !ok {
		r.tunnelsMu.Unlock()
		return fmt.Errorf("%w: local-tid=%d", ErrTunnelNotFound, localTID)
	}
	sess, ok := t.sessions[localSID]
	if !ok {
		r.tunnelsMu.Unlock()
		return fmt.Errorf("%w: local-sid=%d", ErrSessionNotFound, localSID)
	}
	cancelSessionTimeouts(sess)
	username := sess.username
	now := r.params.Clock()
	outbound := t.teardownSession(sess, cdnResultAdministrative, now, r.logger)
	teardowns := t.drainPendingKernelTeardowns()
	r.tunnelsMu.Unlock()

	if r.routeObserver != nil {
		r.routeObserver.OnSessionDown(localTID, localSID)
	}
	r.emitSessionDown(localTID, localSID, username, cause)
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (teardown-all session)",
				"to", req.to.String(), "error", err.Error())
		}
	}
	r.enqueueKernelEvents(nil, teardowns)
	ClearSessionMetadata(localTID, localSID)
	return nil
}

// drainPendingKernelTeardowns returns any kernel teardown events that
// session-clearing queued on the tunnel, resetting the slice. Caller
// MUST hold the owning reactor's tunnelsMu.
func (t *L2TPTunnel) drainPendingKernelTeardowns() []kernelTeardownEvent {
	if len(t.pendingKernelTeardowns) == 0 {
		return nil
	}
	out := t.pendingKernelTeardowns
	t.pendingKernelTeardowns = nil
	return out
}
