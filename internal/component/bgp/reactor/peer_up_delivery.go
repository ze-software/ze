// Design: docs/architecture/plugin/plugin-system.md — peer-up delivery and the initial sync that follows it
// Related: peer_run.go — the FSM Established callback, which calls startEstablishedSession
// Related: peer_initial_sync.go — sendInitialRoutes, the sync that waits on the delivery
// Related: forward_pool.go — tryDispatchBound, where a joined writer reports that it waits on the gate
// RFC: rfc/short/rfc4724.md — End-of-RIB marker: sent once the initial routing update completes

package reactor

import "sync"

// peerUpDelivery is one session's peer-up notification, as the initial sync of
// that session reads it. The FSM Established callback opens it before it starts
// the sync and finishes it when the plugin notification has returned
// (Peer.startEstablishedSession).
//
// The sync starts when the notification has returned, which keeps every route a
// plugin writes from its peer-up handler ahead of Ze's own table, as it was when
// the callback started the sync after the notification. It starts earlier in
// exactly one case: a writer the engine joins is parked behind this session's
// initial-sync gate (Peer.forwardOrderHold). Only the sync opens that gate, so a
// handler waiting on such a writer can return only once the sync has run, and a
// sync that waited for the handler to return would wait on itself.
//
// Safe for concurrent use.
type peerUpDelivery struct {
	// returned is closed when the plugin notification has returned.
	returned chan struct{}
	// gateWaiter is closed when a joined writer parked behind the session's
	// initial-sync gate while the notification was still running.
	gateWaiter     chan struct{}
	gateWaiterOnce sync.Once
}

// openPeerUpDelivery opens this session's peer-up notification. The caller MUST
// open it before it starts the session's initial sync, and MUST call finish on
// the answer once the notification has returned: the sync waits for one of the
// two. Called once per session, by Peer.startEstablishedSession.
func (s *Session) openPeerUpDelivery() *peerUpDelivery {
	delivery := &peerUpDelivery{returned: make(chan struct{}), gateWaiter: make(chan struct{})}
	s.peerUpDelivery.Store(delivery)
	return delivery
}

// finish records that the plugin notification has returned. MUST be called
// exactly once, by the caller of openPeerUpDelivery.
func (d *peerUpDelivery) finish() {
	close(d.returned)
}

// noteInitialGateWaiter records that a writer the engine joins is parked behind
// this session's initial-sync gate. It releases the sync's start when the
// peer-up notification is still running, and changes nothing otherwise: a
// session whose notification was never opened, or has returned, starts its sync
// without it. It takes no peer, session or pool lock, because the forward pool
// calls it for the carrier it parked (fwdPool.tryDispatchBound).
func (s *Session) noteInitialGateWaiter() {
	delivery := s.peerUpDelivery.Load()
	if delivery == nil {
		return
	}
	delivery.gateWaiterOnce.Do(func() { close(delivery.gateWaiter) })
}

// initialSyncStart is what let a session's initial sync start
// (Session.awaitInitialSyncStart). The values start at one, so the zero value is
// no outcome and the wait never answers it.
type initialSyncStart uint8

const (
	// The peer-up notification has returned, or none was opened for the session.
	initialSyncStartDelivered initialSyncStart = iota + 1
	// A joined writer waits on the session's gate while the notification runs.
	initialSyncStartGateWaiter
	// The session ended first.
	initialSyncStartSessionEnded
)

// awaitInitialSyncStart blocks until this session's initial sync may start, and
// reports what let it: the peer-up notification returned, a joined writer is
// parked behind the session's gate, or the session ended. A notification that
// has already returned wins over a waiting writer, so a sync started after the
// callback, by the startup convergence hold for one, reads as delivered.
//
// A session with no notification opened answers delivered at once: a sync
// driven without the FSM callback has nothing to wait for.
//
// The wait has no timeout on purpose. Before the sync could start during the
// notification, the callback started it only after the notification returned,
// however long that took, so this waits no longer than that did. MUST be called
// on the session's sync goroutine (Peer.sendInitialRoutes).
func (s *Session) awaitInitialSyncStart() initialSyncStart {
	delivery := s.peerUpDelivery.Load()
	if delivery == nil {
		return initialSyncStartDelivered
	}
	select {
	case <-delivery.returned:
		return initialSyncStartDelivered
	default:
	}
	select {
	case <-delivery.returned:
		return initialSyncStartDelivered
	case <-delivery.gateWaiter:
		return initialSyncStartGateWaiter
	case <-s.Done():
		return initialSyncStartSessionEnded
	}
}

// awaitPeerUpDeliveryReturn blocks until this session's peer-up notification
// has returned, and reports false when the session ended first. A session with
// no notification opened answers true at once.
func (s *Session) awaitPeerUpDeliveryReturn() bool {
	delivery := s.peerUpDelivery.Load()
	if delivery == nil {
		return true
	}
	select {
	case <-delivery.returned:
		return true
	case <-s.Done():
		return false
	}
}

// startEstablishedSession runs what the FSM Established callback owes a session
// once enterEstablished has published it: the peer-up barrier reset, the BFD
// client, the initial routing update and the plugin notification, in that
// order. session MUST be the one enterEstablished published, with its
// initial-sync gate still closed. The caller MUST hold neither r.mu nor p.mu.
//
// The BFD client starts first, so the peer's liveness detection depends on no
// plugin: a notification that takes long, or never returns, delays no BFD
// session. It is also live before the first UPDATE of the initial update leaves,
// so a BFD Down during the initial flood still tears the peer down. Starting the
// subscriber can deliver a state change through this same Peer, which is why it
// runs here, on the callback goroutine and outside p.mu, as it always did.
//
// The initial routing update starts before the notification and waits for it
// (Session.awaitInitialSyncStart). Started after it, the sync could not run
// while a plugin's peer-up handler was still running, and a handler that joins
// a replay behind the session's initial-sync gate waited on a gate only that
// sync opens: the callback held the gate shut until the handler's own deadline
// ended the replay, and the peer received neither the stored routes nor, with
// the BFD client started after the notification, a BFD session.
func (p *Peer) startEstablishedSession(session *Session) {
	// Reset the peer-up barrier for this session BEFORE plugins are notified
	// below: the dispatcher raises its expected count and the plugins
	// acknowledge inside notifyPeerEstablished, so the barrier state those
	// calls land on must already belong to this session.
	p.ResetPeerUpBarrier()

	// Open a BFD session for this peer if the operator opted in via
	// `bgp peer connection bfd { ... }` and the BFD plugin is loaded. No-op
	// otherwise.
	p.startBFDClient()

	// The initial-sync gate is already closed: enterEstablished closes the
	// session's gate before it publishes PeerStateEstablished, so shouldQueue()
	// is true here and stays true through the notification below.
	delivery := session.openPeerUpDelivery()
	// Deferred, so a notification that panics does not leave the sync waiting
	// for a return that cannot come.
	defer delivery.finish()

	// Send static routes from config (one-time per-session lifecycle goroutine),
	// unless the startup convergence hold owns this peer's initial routing
	// update and will run it when it releases (update_delay.go).
	peerLogger().Debug("spawning sendInitialRoutes", "peer", p.addrString)
	p.startInitialRoutes(session)

	// Notify the reactor that the session is established.
	p.mu.RLock()
	reactor := p.reactor
	p.mu.RUnlock()
	if reactor == nil {
		return
	}
	// announceEstablished marks the phases a reload racing this delivery reads
	// (applyHotSwappableSettings).
	p.announceEstablished(reactor, session)
}

// awaitPeerUpDeliveryForMarker holds the End-of-RIB of a sync that started
// while the peer-up notification was still running, until that notification has
// returned and the peer-up barrier is open. It reports false when the session
// ended first.
//
// RFC 4724 Section 2: "An UPDATE message with no reachable Network Layer
// Reachability Information (NLRI) and empty withdrawn NLRI is specified as the
// End-of-RIB marker that can be used by a BGP speaker to indicate to its peer
// the completion of the initial routing update after the session is
// established."
//
// A replay a plugin joins from its peer-up handler is on the wire when the
// handler returns, so waiting for the notification puts that replay ahead of
// the marker, where it was when the sync started only after the notification.
// The barrier is read after it for the same reason it is skipped earlier in
// such a sync: its acknowledgements are delivery results, and the delivery
// behind a handler that waits on the gate cannot produce one before the gate
// opens (Peer.waitPeerUpBarrier).
//
// The wait for the notification has no timeout, for the reason
// awaitInitialSyncStart has none: a handler that never returns used to keep the
// whole sync from starting, and here it keeps back the marker alone.
func (p *Peer) awaitPeerUpDeliveryForMarker(session *Session) bool {
	if !session.awaitPeerUpDeliveryReturn() {
		return false
	}
	return p.waitPeerUpBarrier(session) != peerUpBarrierSessionEnded
}
