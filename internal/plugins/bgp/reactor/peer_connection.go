// Design: docs/architecture/core-design.md — BGP peer connection management and collision resolution
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"errors"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
)

// AcceptConnection accepts an incoming TCP connection for this peer.
// Used by the reactor to hand incoming connections to passive peers.
func (p *Peer) AcceptConnection(conn net.Conn) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.Accept(conn)
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

// SetPendingConnection queues an incoming connection for collision resolution.
// RFC 4271 §6.8: Used when we're in OpenConfirm and an incoming connection arrives.
// Returns error if there's already a pending connection.
func (p *Peer) SetPendingConnection(conn net.Conn) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pendingConn != nil {
		return errors.New("pending connection already exists")
	}
	p.pendingConn = conn
	p.pendingOpen = nil
	return nil
}

// ClearPendingConnection clears any pending connection.
func (p *Peer) ClearPendingConnection() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pendingConn = nil
	p.pendingOpen = nil
}

// SetInboundConnection stores a connection that arrived while the session was nil.
// Used for passive peers where the remote reconnects before our backoff expires.
// If a previous inbound connection exists, it is closed and replaced.
func (p *Peer) SetInboundConnection(conn net.Conn) {
	p.mu.Lock()
	old := p.inboundConn
	p.inboundConn = conn
	p.mu.Unlock()

	if old != nil {
		closeConnQuietly(old)
	}

	// Wake up the run() backoff — buffered channel (size 1), skip if already signaled.
	if len(p.inboundNotify) == 0 {
		p.inboundNotify <- struct{}{}
	}
}

// takeInboundConnection returns and clears any stored inbound connection.
func (p *Peer) takeInboundConnection() net.Conn {
	p.mu.Lock()
	defer p.mu.Unlock()
	conn := p.inboundConn
	p.inboundConn = nil
	return conn
}

// HasPendingConnection returns true if there's a pending incoming connection.
func (p *Peer) HasPendingConnection() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pendingConn != nil
}

// ResolvePendingCollision resolves a collision after receiving OPEN on pending connection.
// RFC 4271 §6.8: Compare BGP IDs and close the loser.
//
// Returns:
//   - acceptPending: true if pending connection should be accepted
//   - the pending connection (caller must handle it)
//   - the pending OPEN message (if acceptPending is true)
//   - wait: channel to wait for existing session teardown (if acceptPending is true)
func (p *Peer) ResolvePendingCollision(pendingOpen *message.Open) (acceptPending bool, conn net.Conn, open *message.Open, wait <-chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pendingConn == nil {
		return false, nil, nil, nil
	}

	conn = p.pendingConn
	session := p.session

	if session == nil {
		// Session gone, reject pending
		p.pendingConn = nil
		p.pendingOpen = nil
		return false, conn, nil, nil
	}

	shouldAccept, shouldCloseExisting := session.DetectCollision(pendingOpen.BGPIdentifier)

	if shouldAccept && shouldCloseExisting {
		// Remote wins: close existing, accept pending
		// Store the OPEN so we can replay it
		p.pendingOpen = pendingOpen
		p.pendingConn = nil
		wait = session.Done()

		// Close existing session with NOTIFICATION
		// The session's Run loop will exit and we can accept pending
		go func() {
			_ = session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision)
		}()

		return true, conn, pendingOpen, wait
	}

	// Local wins: reject pending, keep existing
	p.pendingConn = nil
	p.pendingOpen = nil
	return false, conn, nil, nil
}

// AcceptConnectionWithOpen accepts an incoming connection with a pre-received OPEN.
// RFC 4271 §6.8: Used after collision resolution when the pending connection wins.
func (p *Peer) AcceptConnectionWithOpen(conn net.Conn, peerOpen *message.Open) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.AcceptWithOpen(conn, peerOpen)
}
