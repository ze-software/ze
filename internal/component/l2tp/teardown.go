// Design: docs/architecture/l2tp.md -- operator-initiated teardown
// Related: reactor.go -- owns tunnelsByLocalID and tunnelsByPeer
// Related: snapshot.go -- read-side API, sibling to this write-side API

package l2tp

import (
	"errors"
	"fmt"
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

// TeardownTunnelByID sends a StopCCN Result Code 6 (administrative
// shutdown) for the tunnel with the given local TID. The tunnel's
// sessions are cleared and any kernel resources drained the same way
// peer-initiated StopCCN does. Returns ErrTunnelNotFound when the TID
// is unknown.
//
// Caller MUST NOT hold tunnelsMu; this method acquires it internally
// and releases it before writing to the UDP socket (matching the
// existing reactor pattern).
func (r *L2TPReactor) TeardownTunnelByID(localTID uint16) error {
	if localTID == 0 {
		return ErrInvalidID
	}
	r.tunnelsMu.Lock()
	t, ok := r.tunnelsByLocalID[localTID]
	if !ok {
		r.tunnelsMu.Unlock()
		return fmt.Errorf("%w: local-tid=%d", ErrTunnelNotFound, localTID)
	}
	// Collect session IDs before teardownStopCCN clears the session map
	// so the route observer receives one OnSessionDown per session that
	// was live when the operator requested the teardown.
	torn := make([]uint16, 0, len(t.sessions))
	for sid := range t.sessions {
		torn = append(torn, sid)
	}
	now := r.params.Clock()
	outbound := t.teardownStopCCN(now, resultAdministrative)
	teardowns := t.drainPendingKernelTeardowns()
	r.tunnelsMu.Unlock()

	if r.routeObserver != nil {
		for _, sid := range torn {
			r.routeObserver.OnSessionDown(sid)
		}
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

// TeardownSessionByID sends a CDN Result Code 3 (administrative) for
// the first session with the given local SID found on any tunnel.
// Returns ErrSessionNotFound when no session carries the SID.
//
// Caller MUST NOT hold tunnelsMu.
func (r *L2TPReactor) TeardownSessionByID(localSID uint16) error {
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
	outbound := tunnel.teardownSession(sess, cdnResultAdministrative, now, r.logger)
	teardowns := tunnel.drainPendingKernelTeardowns()
	r.tunnelsMu.Unlock()

	if r.routeObserver != nil {
		r.routeObserver.OnSessionDown(localSID)
	}
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
func (r *L2TPReactor) TeardownAllTunnels() int {
	r.tunnelsMu.Lock()
	tids := make([]uint16, 0, len(r.tunnelsByLocalID))
	for tid := range r.tunnelsByLocalID {
		tids = append(tids, tid)
	}
	r.tunnelsMu.Unlock()

	n := 0
	for _, tid := range tids {
		if err := r.TeardownTunnelByID(tid); err == nil {
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
func (r *L2TPReactor) TeardownAllSessions() int {
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
		if err := r.teardownSessionOnTunnel(k.tid, k.sid); err == nil {
			n++
		}
	}
	return n
}

// teardownSessionOnTunnel is the tunnel-scoped variant used by
// TeardownAllSessions. Distinct from TeardownSessionByID because the
// latter walks every tunnel looking for the SID; here the caller
// already knows the tuple.
func (r *L2TPReactor) teardownSessionOnTunnel(localTID, localSID uint16) error {
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
	outbound := t.teardownSession(sess, cdnResultAdministrative, now, r.logger)
	teardowns := t.drainPendingKernelTeardowns()
	r.tunnelsMu.Unlock()

	if r.routeObserver != nil {
		r.routeObserver.OnSessionDown(localSID)
	}
	for _, req := range outbound {
		if err := r.listener.Send(req.to, req.bytes); err != nil {
			r.logger.Warn("l2tp: outbound send failed (teardown-all session)",
				"to", req.to.String(), "error", err.Error())
		}
	}
	r.enqueueKernelEvents(nil, teardowns)
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
