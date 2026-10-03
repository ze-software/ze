// Design: docs/architecture/core-design.md — BGP peer connection management and collision resolution
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"errors"
	"net"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// inboundConnection transfers one accepted socket to the next session epoch.
// A collision winner owns its parsed OPEN and complete original wire buffer;
// the producer MUST stop using both after handing them to the Peer.
type inboundConnection struct {
	conn net.Conn
	open *message.Open
	wire []byte
}

// acceptConnection accepts an incoming TCP connection for this peer.
// Used by the reactor to hand incoming connections to passive peers.
func (p *Peer) acceptConnection(conn net.Conn) error {
	p.mu.RLock()
	if p.collisionHandoff {
		p.mu.RUnlock()
		return ErrAlreadyConnected
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

	if p.pendingConn != nil {
		return errors.New("pending connection already exists")
	}
	if p.collisionHandoff {
		return errors.New("collision winner already waiting for the next session")
	}
	p.pendingConn = conn
	return nil
}

// clearPendingConnection clears any pending connection.
func (p *Peer) clearPendingConnection() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pendingConn = nil
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
	old := p.storeInboundLocked(inboundConnection{conn: conn})
	p.mu.Unlock()
	if old != nil {
		closeConnQuietly(old)
	}
}

// storeInboundLocked transfers ownership into the single inbound slot.
// Caller MUST hold p.mu and close the returned displaced socket after unlocking.
func (p *Peer) storeInboundLocked(inbound inboundConnection) net.Conn {
	old := p.inbound.conn
	p.inbound = inbound
	select {
	case p.inboundNotify <- struct{}{}:
	default: // A wakeup for this slot is already pending.
	}
	return old
}

// takeInboundConnection transfers and clears the slot atomically, but MUST NOT
// release the collision reservation. The caller MUST accept or close its socket
// and clear collisionHandoff under p.mu only after acceptWithOpen returns.
func (p *Peer) takeInboundConnection() inboundConnection {
	p.mu.Lock()
	defer p.mu.Unlock()
	inbound := p.inbound
	p.inbound = inboundConnection{}
	return inbound
}

// hasPendingConnection returns true if there's a pending incoming connection.
func (p *Peer) hasPendingConnection() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pendingConn != nil
}

// resolvePendingCollision transfers a winning socket and its original OPEN to
// the next Peer-owned session before closing the losing session (RFC 4271 §6.8).
// On acceptance the caller MUST release ownership of wire and pendingOpen.
// On refusal the caller MUST close the returned socket, if any.
func (p *Peer) resolvePendingCollision(pendingOpen *message.Open, wire []byte) (bool, net.Conn) {
	p.mu.Lock()
	conn := p.pendingConn
	p.pendingConn = nil
	session := p.session
	if conn == nil {
		p.mu.Unlock()
		return false, nil
	}
	if session == nil {
		p.mu.Unlock()
		return false, conn
	}
	if p.stopping.Load() {
		p.mu.Unlock()
		return false, conn
	}
	if p.ctx != nil && p.ctx.Err() != nil {
		p.mu.Unlock()
		return false, conn
	}
	shouldAccept, shouldCloseExisting := session.detectCollision(pendingOpen.BGPIdentifier)
	if !shouldAccept {
		p.mu.Unlock()
		return false, conn
	}
	if !shouldCloseExisting {
		p.mu.Unlock()
		return false, conn
	}

	// Publishing before CloseWithNotification prevents runOnce from passing
	// its inbound check without the winner. Its ordinary backoff wakeup then
	// starts the fresh epoch only after all old-session cleanup has finished.
	old := p.storeInboundLocked(inboundConnection{conn: conn, open: pendingOpen, wire: wire})
	p.collisionHandoff = true
	p.mu.Unlock()
	if old != nil {
		closeConnQuietly(old)
	}
	go func() {
		_ = session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision)
	}()
	return true, nil
}
