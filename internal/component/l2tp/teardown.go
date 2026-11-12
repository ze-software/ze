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
// caller that ends a tunnel for another reason calls teardownTunnel with its
// own Result Code and cause, as clearTunnelAfterPanic does.
//
// Caller MUST NOT hold tunnelsMu.
func (r *l2tpReactor) teardownTunnelByID(localTID uint16) error {
	return r.teardownTunnel(localTID, ResultCodeValue{Result: resultAdministrative}, l2tpevents.TerminateCauseAdminReset)
}

// teardownTunnel sends a StopCCN carrying result for the tunnel with the given
// local TID, ends its sessions with cause, and drains its kernel resources the
// way a peer-initiated StopCCN does. Returns ErrTunnelNotFound when the TID is
// unknown.
//
// Caller MUST NOT hold tunnelsMu; this method acquires it internally
// and releases it before writing to the UDP socket (matching the
// existing reactor pattern).
func (r *l2tpReactor) teardownTunnel(localTID uint16, result ResultCodeValue, cause l2tpevents.TerminateCause) error {
	if localTID == 0 {
		return ErrInvalidID
	}
	r.tunnelsMu.Lock()
	t, ok := r.tunnelsByLocalID[localTID]
	if !ok {
		r.tunnelsMu.Unlock()
		return fmt.Errorf("%w: local-tid=%d", ErrTunnelNotFound, localTID)
	}
	now := r.params.Clock()
	outbound := t.teardownStopCCN(now, result, cause)
	teardowns, downs := t.drainPendingTeardowns()
	r.tunnelsMu.Unlock()

	r.deliverSessionDowns(downs)
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (tunnel teardown)",
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
	now := r.params.Clock()
	outbound := tunnel.teardownSession(sess, cdnResultAdministrative, cause, now, r.logger)
	teardowns, downs := tunnel.drainPendingTeardowns()
	r.tunnelsMu.Unlock()

	r.deliverSessionDowns(downs)
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (operator session teardown)",
				"to", req.to.String(), "error", err.Error())
		}
	}
	r.enqueueKernelEvents(nil, teardowns)
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

// deliverSessionDowns delivers every session a teardown removed, one drain's
// batch at a time, each in the turn of the order ticket the drain stamped on
// it: the route observer withdraws the subscriber route of each down whose
// session was established, then one (l2tp, session-down) is published per
// down. Its input is what removeSession queued, so a teardown path cannot
// reach the readers without one and cannot reach them twice. handleLCPDown
// publishes the only other (l2tp, session-down): the end of a network
// lifetime of a session that stays.
//
// This is what RADIUS accounting, the shaper, the CoS handler, the subscriber
// bridge and the CQM observer each key their cleanup on: no delivery means no
// Accounting-Stop, no traffic rule dropped and no subscriber entry removed.
// The address pool releases on the subscriber bridge's re-publication.
//
// A path that drains several tunnels (a tunnel teardown, retention reaping, a
// tie-breaker loss) passes their downs in one slice, each tunnel's batch
// contiguous, as the drains appended them. The batches are delivered one after
// another, and the call for the rest is deferred, so a reader that panics on
// one batch still lets the later batches deliver and finish their tickets.
// The recursion depth is the number of drains in one path: the tunnels that
// path ended, bounded by the tunnel count.
//
// Caller MUST NOT hold tunnelsMu, and MUST pass every down a drain returned,
// on every exit of its path (drainPendingTeardowns): each batch's ticket is
// finished only here. A reader runs on this goroutine
// and MUST NOT cause a publication for the same tunnel from inside its
// handler: that publication would wait behind the ticket this call holds
// (the reader contract on the events package).
func (r *l2tpReactor) deliverSessionDowns(downs []sessionDown) {
	if len(downs) == 0 {
		return
	}
	batchLen := 1
	for batchLen < len(downs) && downs[batchLen].ticket == downs[0].ticket {
		batchLen++
	}
	batch := downs[:batchLen]
	defer r.deliverSessionDowns(downs[batchLen:])
	batch[0].ticket.deliver(func() { r.notifySessionDowns(batch) })
}

// notifySessionDowns tells the route observer and the readers of
// (l2tp, session-down) about one drain's batch. Runs inside the batch's turn.
func (r *l2tpReactor) notifySessionDowns(batch []sessionDown) {
	if r.routeObserver != nil {
		for _, d := range batch {
			if d.established {
				r.routeObserver.OnSessionDown(d.localTID, d.localSID)
			}
		}
	}
	if r.eventBus == nil {
		return
	}
	for _, d := range batch {
		if _, err := l2tpevents.SessionDown.Emit(r.eventBus, &l2tpevents.SessionDownPayload{
			TunnelID:  d.localTID,
			SessionID: d.localSID,
			Username:  d.username,
			Cause:     d.cause,
		}); err != nil {
			r.logger.Warn("l2tp: session-down emit failed",
				"local-tid", d.localTID, "local-sid", d.localSID, "error", err)
		}
	}
}

// teardownSessionOnTunnel is the tunnel-scoped variant used by
// TeardownAllSessions. Distinct from TeardownSessionByID because the
// latter walks every tunnel looking for the SID; here the caller
// already knows the tuple.
//
// cause carries the same obligation teardownSessionByID states.
func (r *l2tpReactor) teardownSessionOnTunnel(localTID, localSID uint16, cause l2tpevents.TerminateCause) error {
	return r.teardownSessionOnTunnelWith(localTID, localSID, cdnResultAdministrative, cause)
}

// teardownRealmSession ends a bridged realm-relay session whose PPPoE
// subscriber left (PADT, or the PPPoE side stopped). It is the realm call's
// teardown hook (relay_sink.go realmCall.Cancel), so a subscriber hang-up
// reaches the LNS. Caller MUST NOT hold tunnelsMu. A session or tunnel
// already gone means the LNS ended the call first, which is the outcome
// wanted, so only another failure is logged.
func (r *l2tpReactor) teardownRealmSession(localTID, localSID uint16) {
	// RFC 2661 Section 6.12: "The Call-Disconnect-Notify (CDN) message is an
	// L2TP control message sent by either the LAC or LNS to request
	// disconnection of a specific call within the tunnel." The subscriber's
	// link is gone, so the Result Code is 1 (Section 4.4.2, "Call
	// disconnected due to loss of carrier").
	err := r.teardownSessionOnTunnelWith(localTID, localSID, cdnResultLossOfCarrier, l2tpevents.TerminateCauseUserRequest)
	if err == nil {
		return
	}
	if errors.Is(err, ErrSessionNotFound) {
		return
	}
	if errors.Is(err, ErrTunnelNotFound) {
		return
	}
	r.logger.Warn("l2tp: realm relay session not torn down after the subscriber left",
		"local-tid", localTID, "local-sid", localSID, "error", err)
}

// teardownSessionOnTunnelWith is teardownSessionOnTunnel with the CDN
// Result Code named by the caller.
func (r *l2tpReactor) teardownSessionOnTunnelWith(localTID, localSID, result uint16, cause l2tpevents.TerminateCause) error {
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
	now := r.params.Clock()
	outbound := t.teardownSession(sess, result, cause, now, r.logger)
	teardowns, downs := t.drainPendingTeardowns()
	r.tunnelsMu.Unlock()

	r.deliverSessionDowns(downs)
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (teardown-all session)",
				"to", req.to.String(), "error", err.Error())
		}
	}
	r.enqueueKernelEvents(nil, teardowns)
	return nil
}
