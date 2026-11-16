// RFC: rfc/short/rfc4271.md — Section 6.8, connection collision detection
// Design: docs/architecture/core-design.md — TCP accept and collision detection (RFC 4271 §6.8)
// Overview: reactor.go — BGP reactor event loop and peer management

package reactor

import (
	"errors"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/core/clock"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// handleConnection handles an incoming TCP connection.
// RFC 4271 §6.8: Connection collision detection.
//
// Architecture:
//
//	handleConnection()
//	├── OpenSent/OpenConfirm/Established → setPendingConnection() + go handlePendingCollision()
//	│                  └── independent Session/FSM reads OPEN → resolveIncoming()
//	│                       ├── Local wins → retain closing admission through refusal/retirement
//	│                       ├── Remote wins → transfer same Session/socket/OPEN receipt
//	│                       │                + retire incumbent before selected adoption
//	│                       └── OpenSent without known remote ID → continue both handshakes
//	└── Other states → normal acceptConnection()
func (r *Reactor) handleConnection(conn net.Conn) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		closeConnQuietly(conn)
		return
	}
	peerIP := remoteAddr.AddrPort().Addr().Unmap()

	peer, cb := r.peerForConnection(conn, peerIP)
	if peer == nil {
		closeConnQuietly(conn)
		return
	}

	r.acceptOrReject(conn, peer, cb)
}

// handleConnectionWithContext handles an incoming TCP connection with listener context.
// listenerAddr is the local address the listener is bound to.
// Static peers must match that binding. Dynamic peers are already qualified by
// the actual accepted endpoint, which can differ from a wildcard declaration.
func (r *Reactor) handleConnectionWithContext(conn net.Conn, listenerAddr netip.Addr) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		closeConnQuietly(conn)
		return
	}
	peerIP := remoteAddr.AddrPort().Addr().Unmap()

	peer, cb := r.peerForConnection(conn, peerIP)
	if peer == nil {
		closeConnQuietly(conn)
		return
	}

	settings := peer.Settings()

	// A dynamic template's wildcard declaration need not match the listener's
	// declaration: peerForConnection has already checked its actual endpoint.
	if !settings.IsDynamic {
		if settings.LocalAddress.IsValid() {
			if !sameTransportAddress(settings.LocalAddress, listenerAddr) {
				closeConnQuietly(conn)
				return
			}
		}
	}

	r.acceptOrReject(conn, peer, cb)
}

// peerForConnection preserves configured static-peer attribution, then qualifies
// every dynamic admission and reuse by the accepted socket's actual local
// endpoint. It MUST supply that endpoint to tryCreateDynamicPeerAt before any
// member can be published. A caller receiving nil MUST close the connection.
// Thread-safe: peer lookup and creation acquire r.mu internally.
func (r *Reactor) peerForConnection(conn net.Conn, addr netip.Addr) (*Peer, connectionCallback) {
	r.mu.RLock()
	peer, exists := r.findPeerByAddr(addr)
	cb := r.connCallback
	r.mu.RUnlock()
	if exists {
		if !peer.Settings().IsDynamic {
			return peer, cb
		}
	}

	local, ok := conn.LocalAddr().(*net.TCPAddr)
	if !ok {
		return nil, nil
	}
	endpoint := local.AddrPort()
	endpoint = netip.AddrPortFrom(endpoint.Addr().Unmap(), endpoint.Port())
	peer = r.tryCreateDynamicPeerAt(addr, endpoint)
	if peer == nil {
		return nil, nil
	}
	// Read the callback again after creation or its existing-member race recheck.
	r.mu.RLock()
	cb = r.connCallback
	r.mu.RUnlock()
	return peer, cb
}

// handleListenerConnection attributes an inbound connection that arrived on the
// listener bound to addr:port.
//
// The attribution is decided HERE, per connection, and not when the socket was
// created. A listener outlives the configuration it was born under. A reload can
// add a peer or a dynamic group that claims the same address and port. A socket
// carrying a peer key captured at birth would then serve every later connection
// under the departed configuration's peer.
//
// listenerPeerKey returns a key only while exactly one claimant owns the socket.
// The direct route below can therefore never take a connection from a claimant
// it was not opened for. Every other case attributes by remote IP, which is what
// the default-port listener has always done.
func (r *Reactor) handleListenerConnection(conn net.Conn, listenerAddr netip.Addr, port int) {
	r.mu.RLock()
	peerKey := r.listenerPeerKey(listenerAddr, port)
	r.mu.RUnlock()

	if peerKey.IsValid() {
		r.handleDirectConnection(conn, peerKey)
		return
	}
	r.handleConnectionWithContext(conn, listenerAddr)
}

// listenerPeerKey returns the peer a listener on addr:port routes every inbound
// connection to, or the zero AddrPort when the listener MUST attribute by remote
// IP instead.
//
// A key is returned only when both hold:
//   - exactly one object in the configuration claims that address and port, and
//     it is a passive peer. listenerClaimants counts a dynamic group as a
//     claimant too, because a group accepts on the same socket.
//   - the socket is not on the well-known BGP port. A peer that names no local
//     address gets no listener here at all: its inbound arrives on the daemon's
//     global socket, which attributes by remote IP (handleConnection). On port
//     179 that global socket and this one can both be bound, the specific
//     address wins, and a peer expecting the global one lands here.
//
// The test is the LISTEN port, not the peer's own Port. Port is the remote
// endpoint, the port Ze dials, and it says nothing about which socket an inbound
// connection arrived on. It was read here until 2026-09-06, when one field
// carried both endpoints (peer_settings.go).
//
// handleDirectConnection reads no remote address at all. A second claimant on
// the same socket would therefore be served under the first one's policy.
// Refusing the key is what stops that: the caller then attributes by remote IP
// and closes what it cannot match.
// Must be called with r.mu held.
func (r *Reactor) listenerPeerKey(addr netip.Addr, port int) netip.AddrPort {
	count, only := r.listenerClaimants(addr, port)
	if count != 1 || only == nil {
		return netip.AddrPort{}
	}
	if port == DefaultBGPPort {
		return netip.AddrPort{}
	}
	return only.PeerKey()
}

// listenerClaimants reports how many configured objects need a listener on
// addr:port, and the settings of the single passive peer among them when there
// is exactly one claimant and no dynamic group.
//
// A claimant is a passive peer whose local address is addr and whose listen port
// is port, or a dynamic group with the same pair. A dynamic group accepts
// connections nothing in the configuration names. That is why it opens a
// listener of its own (startMultiListeners), and why a listener it shares must
// attribute by remote IP.
// Must be called with r.mu held.
func (r *Reactor) listenerClaimants(addr netip.Addr, port int) (int, *PeerSettings) {
	count := 0
	var only *PeerSettings
	for _, peer := range r.peers {
		s := peer.Settings()
		if !s.LocalAddress.IsValid() || !s.Connection.IsPassive() {
			continue
		}
		if r.peerListenPort(s) != port {
			continue
		}
		if !sameTransportAddress(s.LocalAddress, addr) {
			continue
		}
		count++
		only = s
	}
	for _, dg := range r.dynamicGroups {
		if dg.Settings == nil || !dg.Settings.LocalAddress.IsValid() || !dg.Settings.Connection.IsPassive() {
			continue
		}
		if r.peerListenPort(dg.Settings) != port {
			continue
		}
		if !sameTransportAddress(dg.Settings.LocalAddress, addr) {
			continue
		}
		count++
		only = nil
	}
	if count != 1 {
		return count, nil
	}
	return count, only
}

// handleDirectConnection handles a connection on a per-peer-port listener.
// The peerKey directly identifies the target peer (no remote IP matching needed).
// Reached only through handleListenerConnection, which establishes that this
// peer is the sole claimant of the socket the connection arrived on.
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
func (r *Reactor) acceptOrReject(conn net.Conn, peer *Peer, cb connectionCallback) {
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

	// RFC 4271 Section 8.2.2, including Established: "the second
	// connection SHALL be tracked until it sends an OPEN message".
	// Section 6.8 chooses the loser only after that OPEN arrives.
	state := peer.SessionState()
	held := false
	if session := peer.currentSession(); session != nil {
		held = session.holdsOpen()
	}
	if held || state == fsm.StateOpenSent || state == fsm.StateOpenConfirm || state == fsm.StateEstablished {
		if err := peer.setPendingConnection(conn); err != nil {
			if errors.Is(err, errCollisionHandoff) {
				closeConnQuietly(conn)
				return
			}
			if errors.Is(err, ErrSessionTearingDown) {
				closeConnQuietly(conn)
				return
			}
			r.rejectConnectionCollisionWithSettings(conn, settings)
			return
		}
		go r.handlePendingCollision(peer, conn)
		return
	}

	// Accept connection on peer's session.
	if err := peer.acceptConnection(conn); err != nil {
		// If the session can't accept right now and the peer is passive, buffer the
		// connection for the next runOnce() cycle instead of closing it. This handles
		// the race where the remote reconnects faster than our session teardown:
		// - ErrNotConnected: session is nil (not yet created or already cleaned up)
		// - ErrSessionTearingDown: session is shutting down
		// - ErrAlreadyConnected: session still has stale conn ref from previous connection
		if (errors.Is(err, ErrNotConnected) || errors.Is(err, ErrSessionTearingDown) || errors.Is(err, ErrAlreadyConnected)) && peer.Settings().Connection.IsPassive() {
			peer.setInboundConnection(conn)
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

func (r *Reactor) rejectConnectionCollisionWithSettings(conn net.Conn, settings *PeerSettings) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		if err := tuneTCPConnectionForSettings(tcp, settings); err != nil {
			reactorLogger().Debug("collision notification socket tuning failed", "peer", settings.Address, "err", err)
		}
	}
	rejectConnectionCollision(conn, r.clock)
}

// rejectConnectionCollision sends NOTIFICATION Cease/Connection Collision (6/7)
// and closes the connection, bounded by the normal control-write maximum.
// RFC 4271 Section 6.8 requires Cease for this OPEN-bearing collision loser.
func rejectConnectionCollision(conn net.Conn, clk clock.Clock) {
	rejectPendingConnection(conn, clk, message.NotifyCease, message.NotifyCeaseConnectionCollision)
}

// rejectPendingConnection bounds a refusal on a connection with no Session
// writer. OPEN decode errors retain their own code before collision resolution.
// It MUST close conn after every outcome; reportPendingRefusalFailure reports
// write failures but leaves that ownership here.
func rejectPendingConnection(conn net.Conn, clk clock.Clock, code message.NotifyErrorCode, subcode uint8) {
	defer closeConnQuietly(conn)
	if err := conn.SetWriteDeadline(clk.Now().Add(fwdWriteDeadlineDefault)); err != nil {
		reportPendingRefusalFailure(conn, "set write deadline", err)
		return
	}
	notif := &message.Notification{
		ErrorCode:    code,
		ErrorSubcode: subcode,
	}
	data := message.PackTo(notif, nil)
	if _, err := conn.Write(data); err != nil {
		reportPendingRefusalFailure(conn, "write NOTIFICATION", err)
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

	peer.pauseReading()
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

	peer.resumeReading()
	return nil
}

// pauseAllReads pauses reading from all peers' sessions.
func (r *Reactor) pauseAllReads() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, peer := range r.peers {
		peer.pauseReading()
	}
}

// resumeAllReads resumes reading from all peers' sessions.
func (r *Reactor) resumeAllReads() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, peer := range r.peers {
		peer.resumeReading()
	}
}
