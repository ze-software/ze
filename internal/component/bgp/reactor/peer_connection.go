// Design: docs/architecture/core-design.md — BGP peer connection management and collision resolution
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"errors"
	"net"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// errCollisionHandoff means the single selected inbound reservation is
// occupied. No independent Session or OPEN exists for a later arrival.
var errCollisionHandoff = errors.New("collision winner already waiting for the next session")

// acceptConnection accepts an incoming TCP connection for this peer.
// Used by the reactor to hand incoming connections to passive peers.
func (p *Peer) acceptConnection(conn net.Conn) error {
	p.mu.RLock()
	if p.collisionHandoff {
		p.mu.RUnlock()
		return errCollisionHandoff
	}
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.Accept(conn)
}

// currentSession returns the peer's live session under p.mu, or nil when the peer
// has none: it has never connected, or its run loop is between sessions.
//
// The reload path uses it to take the swap-or-restart decision against what the
// running session actually negotiated (peer_settings_negotiation.go). A nil result
// is the honest answer that there is no negotiation to preserve, which restarts.
func (p *Peer) currentSession() *Session {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.session
}

// SessionState returns the current FSM state of the session.
// Returns StateIdle if no session exists.
func (p *Peer) SessionState() fsm.State {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return fsm.StateIdle
	}
	return session.State()
}

// setPendingConnection queues an incoming connection for collision resolution.
// RFC 4271 Sections 6.8 and 8.2.2: retain the second connection until OPEN.
// Returns error if there's already a pending connection.
func (p *Peer) setPendingConnection(conn net.Conn) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.incomingClosed {
		return ErrSessionTearingDown
	}
	if p.pending != nil {
		return errors.New("pending connection already exists")
	}
	if p.collisionHandoff {
		return errCollisionHandoff
	}
	p.pending = &pendingSocket{conn: conn}
	return nil
}

// clearPendingConnection clears any pending connection.
func (p *Peer) clearPendingConnection() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending = nil
}

// setInboundConnection stores a connection that arrived between sessions.
// An already-selected collision winner takes precedence over a later arrival.
func (p *Peer) setInboundConnection(conn net.Conn) {
	p.mu.Lock()
	if p.collisionHandoff {
		p.mu.Unlock()
		closeConnQuietly(conn)
		return
	}
	old := p.storeInboundLocked(&incomingSocket{conn: conn})
	p.mu.Unlock()
	if old != nil {
		discardInbound(old)
	}
}

// storeInboundLocked transfers ownership into the single inbound slot.
// Caller MUST hold p.mu and close the returned displaced socket after unlocking.
func (p *Peer) storeInboundLocked(inbound inboundConnection) inboundConnection {
	old := p.inbound
	p.inbound = inbound
	select {
	case p.inboundNotify <- struct{}{}:
	default: // A wakeup for this slot is already pending.
	}
	return old
}

// takeInboundConnection transfers and clears the slot atomically, but MUST NOT
// release the collision reservation. The caller MUST accept or close its socket
// and clear collisionHandoff under p.mu only after receipt installation finishes.
func (p *Peer) takeInboundConnection() inboundConnection {
	p.mu.Lock()
	defer p.mu.Unlock()
	inbound := p.inbound
	p.inbound = nil
	return inbound
}

// hasPendingConnection reports an undecided incoming exchange. A decided
// loser retains the admission gate through its refusal and joined retirement.
func (p *Peer) hasPendingConnection() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	switch p.pending.(type) {
	case *pendingSocket, *incomingCandidate:
		return true
	case nil, *closingIncoming:
		return false
	}
	return false
}

// finishCollisionHandoff runs outside p.mu because closing can invoke a Peer
// callback. Publishing the winner MUST precede this call, so runOnce cannot
// miss it when retirement wakes the reconnect loop.
func (p *Peer) finishCollisionHandoff(session *Session, conn net.Conn, remoteID uint32) {
	if session.closeHeldForCollision() {
		return
	}
	// An OPEN or Event 12 may have won while the inbound was published.
	// Re-ask Section 6.8 rather than closing a now-preferred connection.
	accept, closeExisting := session.detectCollision(remoteID)
	if accept && closeExisting {
		if err := session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision); err != nil {
			sessionLogger().Debug("collision close failed", "error", err)
		}
		return
	}
	p.mu.Lock()
	if inboundConn(p.inbound) != conn {
		// runOnce already owns it; only that owner may close it now.
		p.mu.Unlock()
		return
	}
	arrival := p.inbound
	p.inbound = nil
	p.collisionHandoff = false
	select {
	case <-p.inboundNotify:
	default: // The run loop may already have consumed the wakeup.
	}
	p.mu.Unlock()
	// Unlike the OPEN-less held loser, this connection already sent OPEN.
	// RFC 4271 Section 6.8 requires Cease; no Peer lock is held while writing.
	if candidate := inboundCandidate(arrival); candidate != nil {
		_ = candidate.session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision)
		candidate.retire()
	} else {
		rejectConnectionCollision(conn, p.clock)
	}
}
