// Design: docs/architecture/behavior/peer-lifecycle.md — a connection held by a peer that has no run loop
// Overview: peer.go — Peer struct, accessors, lifecycle API
// Related: peer_connection.go — the inbound slot (setInboundConnection)
// Related: reactor_api.go — the reconcile that hands a held connection to the replacement peer
// Related: peer_run.go — cleanup, which empties the slot of a peer that ran

package reactor

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
)

// takeUnstartedInbound empties the inbound slot of a peer that has no run loop
// and returns the connection it held.
//
// A connection the listener accepts before the peer start barrier opens waits
// in the slot (acceptOrReject). Only the run loop takes it, and only the run
// loop's cleanup closes it. A peer that is stopped or replaced before it
// starts has neither, so its owner MUST take the connection here and then
// hand it on (adoptHeldInbound) or close it (rejectHeldInbound, discardInbound).
//
// It answers nil for a peer whose run loop is alive, because cleanup owns that
// slot, and for a peer that holds nothing. Safe for concurrent use.
func (p *Peer) takeUnstartedInbound() inboundConnection {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return nil
	}
	held := p.inbound
	p.inbound = nil
	return held
}

// adoptHeldInbound stores held, which takeUnstartedInbound took from the peer
// this one replaces, in this peer's inbound slot, and reports whether it did.
// The run loop then accepts it as it accepts any connection that waited for
// the start barrier (runOnce).
//
// It answers false, and stores nothing, when the slot already holds a
// connection: that one arrived at this peer after the replacement, so it is
// the one the neighbor opened last. It also answers false for a peer that is
// stopping with no run loop (setInboundConnection). The caller MUST close held
// when the answer is false. Safe for concurrent use.
func (p *Peer) adoptHeldInbound(held inboundConnection) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inbound != nil {
		return false
	}
	if p.cancel == nil {
		if p.stopping.Load() {
			return false
		}
	}
	p.storeInboundLocked(held)
	return true
}

// handHeldInbound gives the peer now stored under key the connection that the
// never-started peer it replaced was holding. The neighbor opened that
// connection to a peer that is still configured, so the replacement answers
// it. The caller MUST NOT hold r.mu.
func (r *Reactor) handHeldInbound(key netip.AddrPort, held inboundConnection) {
	r.mu.RLock()
	peer, exists := r.peers[key]
	r.mu.RUnlock()

	if !exists {
		rejectHeldInbound(held, r.clock)
		return
	}
	if !peer.adoptHeldInbound(held) {
		rejectHeldInbound(held, r.clock)
		return
	}
	reactorLogger().Info("inbound connection kept for the replaced peer", "peer", key)
}

// rejectHeldInbound closes a held connection that no peer will answer, because
// the peer it waited for left the configuration before it started. A nil held
// is nothing to close.
//
// RFC 4486 Section 4: "If a BGP speaker decides to disallow a BGP connection
// (e.g., the peer is not configured locally) after the speaker accepts a
// transport protocol connection, then the BGP speaker SHOULD send a
// NOTIFICATION message with the Error Code Cease and the Error Subcode
// "Connection Rejected"."
//
// The write is bounded (rejectPendingConnection). No OPEN was sent on this
// connection and nothing else was written to it, so its send buffer is empty.
func rejectHeldInbound(held inboundConnection, clk clock.Clock) {
	if held == nil {
		return
	}
	// A collision candidate owns a Session, which retires its own socket.
	if inboundCandidate(held) != nil {
		discardInbound(held)
		return
	}
	conn := inboundConn(held)
	if conn == nil {
		return
	}
	rejectPendingConnection(conn, clk, message.NotifyCeaseConnectionRejected)
}
