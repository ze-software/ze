// Design: docs/architecture/core-design.md — TCP accept and collision detection (RFC 4271 §6.8)
// Overview: reactor.go — BGP reactor event loop and peer management

package reactor

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// handleConnection handles an incoming TCP connection.
// RFC 4271 §6.8: Connection collision detection.
//
// Architecture:
//
//	handleConnection()
//	├── ESTABLISHED → rejectConnectionCollision() [NOTIFICATION 6/7]
//	├── OpenConfirm → SetPendingConnection() + go handlePendingCollision()
//	│                  └── Read OPEN → ResolvePendingCollision()
//	│                       ├── Local wins → rejectConnectionCollision()
//	│                       └── Remote wins → CloseWithNotification() existing
//	│                                        + acceptPendingConnection()
//	└── Other states → normal AcceptConnection()
func (r *Reactor) handleConnection(conn net.Conn) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		closeConnQuietly(conn)
		return
	}
	peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
	peerIP = peerIP.Unmap() // Handle IPv4-mapped IPv6

	r.mu.RLock()
	peer, exists := r.findPeerByAddr(peerIP)
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		closeConnQuietly(conn)
		return
	}

	r.acceptOrReject(conn, peer, cb)
}

// handleConnectionWithContext handles an incoming TCP connection with listener context.
// listenerAddr is the local address the listener is bound to.
// This validates that the connection arrived on the expected listener for RFC compliance.
func (r *Reactor) handleConnectionWithContext(conn net.Conn, listenerAddr netip.Addr) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		closeConnQuietly(conn)
		return
	}
	peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
	peerIP = peerIP.Unmap() // Handle IPv4-mapped IPv6

	r.mu.RLock()
	peer, exists := r.findPeerByAddr(peerIP)
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		closeConnQuietly(conn)
		return
	}

	settings := peer.Settings()

	// RFC compliance: verify connection arrived on expected listener
	if settings.LocalAddress.IsValid() && settings.LocalAddress != listenerAddr {
		closeConnQuietly(conn)
		return
	}

	r.acceptOrReject(conn, peer, cb)
}

// handleDirectConnection handles a connection on a per-peer-port listener.
// The peerKey directly identifies the target peer (no remote IP matching needed).
// Used when peers have custom ports — the listener port uniquely identifies the peer.
func (r *Reactor) handleDirectConnection(conn net.Conn, peerKey netip.AddrPort) {
	r.mu.RLock()
	peer, exists := r.peers[peerKey]
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		closeConnQuietly(conn)
		return
	}

	r.acceptOrReject(conn, peer, cb)
}

// acceptOrReject performs collision detection and accepts or rejects an incoming connection.
// Shared by handleConnection, handleConnectionWithContext, and handleDirectConnection.
func (r *Reactor) acceptOrReject(conn net.Conn, peer *Peer, cb ConnectionCallback) {
	settings := peer.Settings()

	if cb != nil {
		cb(conn, settings)
		return
	}

	// Reject inbound if passive bit is not set (active-only peer).
	if !settings.Connection.IsPassive() {
		closeConnQuietly(conn)
		return
	}

	// RFC 4271 §6.8: Check for collision with ESTABLISHED session.
	// "collision with existing BGP connection that is in the Established
	// state causes closing of the newly created connection"
	if peer.State() == PeerStateEstablished {
		r.rejectConnectionCollision(conn)
		return
	}

	// RFC 4271 §6.8: Check for collision with OpenConfirm session.
	// Queue the connection and wait for OPEN to compare BGP IDs.
	if peer.SessionState() == fsm.StateOpenConfirm {
		if err := peer.SetPendingConnection(conn); err != nil {
			r.rejectConnectionCollision(conn)
			return
		}
		go r.handlePendingCollision(peer, conn)
		return
	}

	// Accept connection on peer's session.
	if err := peer.AcceptConnection(conn); err != nil {
		// If the session can't accept right now and the peer is passive, buffer the
		// connection for the next runOnce() cycle instead of closing it. This handles
		// the race where the remote reconnects faster than our session teardown:
		// - ErrNotConnected: session is nil (not yet created or already cleaned up)
		// - ErrSessionTearingDown: session is shutting down
		// - ErrAlreadyConnected: session still has stale conn ref from previous connection
		if (errors.Is(err, ErrNotConnected) || errors.Is(err, ErrSessionTearingDown) || errors.Is(err, ErrAlreadyConnected)) && peer.Settings().Connection.IsPassive() {
			peer.SetInboundConnection(conn)
			return
		}
		closeConnQuietly(conn)
	}
}

// closeConnQuietly closes a connection, logging any error at debug level.
func closeConnQuietly(conn net.Conn) {
	if err := conn.Close(); err != nil {
		reactorLogger().Debug("close connection", "error", err)
	}
}

// rejectConnectionCollision sends NOTIFICATION Cease/Connection Collision (6/7)
// and closes the connection. RFC 4271 §6.8.
func (r *Reactor) rejectConnectionCollision(conn net.Conn) {
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseConnectionCollision,
	}
	data := message.PackTo(notif, nil)
	_, _ = conn.Write(data)
	_ = conn.Close()
}

// handlePendingCollision reads OPEN from a pending connection and resolves collision.
// RFC 4271 §6.8: Upon receipt of OPEN, compare BGP IDs and close the loser.
func (r *Reactor) handlePendingCollision(peer *Peer, conn net.Conn) {
	buf := make([]byte, message.MaxMsgLen)

	// Set read deadline - use hold time or 90s default
	holdTime := peer.Settings().ReceiveHoldTime
	if holdTime == 0 {
		holdTime = 90 * time.Second
	}
	_ = conn.SetReadDeadline(r.clock.Now().Add(holdTime))

	// Read BGP header
	_, err := io.ReadFull(conn, buf[:message.HeaderLen])
	if err != nil {
		peer.ClearPendingConnection()
		_ = conn.Close()
		return
	}

	hdr, err := message.ParseHeader(buf[:message.HeaderLen])
	if err != nil {
		peer.ClearPendingConnection()
		r.rejectConnectionCollision(conn)
		return
	}

	// Must be OPEN message
	if hdr.Type != message.TypeOPEN {
		peer.ClearPendingConnection()
		r.rejectConnectionCollision(conn)
		return
	}

	// Read OPEN body
	_, err = io.ReadFull(conn, buf[message.HeaderLen:hdr.Length])
	if err != nil {
		peer.ClearPendingConnection()
		_ = conn.Close()
		return
	}

	// Parse OPEN
	open, err := message.UnpackOpen(buf[message.HeaderLen:hdr.Length])
	if err != nil {
		peer.ClearPendingConnection()
		r.rejectConnectionCollision(conn)
		return
	}

	// Resolve collision using BGP ID from OPEN
	acceptPending, pendingConn, pendingOpen, waitSession := peer.ResolvePendingCollision(open)

	if !acceptPending {
		// Local wins: close pending with NOTIFICATION
		r.rejectConnectionCollision(pendingConn)
		return
	}

	// Remote wins: existing session is being closed, accept pending
	// We need to wait a bit for the existing session to close, then
	// start a new session with the pending connection
	r.acceptPendingConnection(peer, pendingConn, pendingOpen, waitSession)
}

// acceptPendingConnection accepts a pending connection after collision resolution.
// The existing session has been closed, so we accept the pending connection with its pre-received OPEN.
func (r *Reactor) acceptPendingConnection(peer *Peer, conn net.Conn, open *message.Open, waitSession <-chan struct{}) {
	// Wait for existing session to fully close
	// The CloseWithNotification was called in ResolvePendingCollision
	if waitSession != nil {
		timer := r.clock.NewTimer(collisionResolutionTimeout)
		defer timer.Stop()
		select {
		case <-waitSession:
			// Session closed
		case <-timer.C():
			reactorLogger().Warn("session teardown timed out during collision resolution", "peer", peer.Settings().Address)
		}
	}

	// Accept connection with the pre-received OPEN
	if err := peer.AcceptConnectionWithOpen(conn, open); err != nil {
		// Failed to accept - peer may have been stopped or old session not yet closed
		_ = conn.Close()
	}
}

// PausePeer pauses reading from a specific peer's session.
// Returns ErrPeerNotFound if the peer address is unknown.
func (r *Reactor) PausePeer(addr netip.Addr) error {
	r.mu.RLock()
	_, peer, ok := r.findPeerKeyByAddr(addr)
	r.mu.RUnlock()

	if !ok {
		return ErrPeerNotFound
	}

	peer.PauseReading()
	return nil
}

// ResumePeer resumes reading from a specific peer's session.
// Returns ErrPeerNotFound if the peer address is unknown.
func (r *Reactor) ResumePeer(addr netip.Addr) error {
	r.mu.RLock()
	_, peer, ok := r.findPeerKeyByAddr(addr)
	r.mu.RUnlock()

	if !ok {
		return ErrPeerNotFound
	}

	peer.ResumeReading()
	return nil
}

// PauseAllReads pauses reading from all peers' sessions.
func (r *Reactor) PauseAllReads() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, peer := range r.peers {
		peer.PauseReading()
	}
}

// ResumeAllReads resumes reading from all peers' sessions.
func (r *Reactor) ResumeAllReads() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, peer := range r.peers {
		peer.ResumeReading()
	}
}
