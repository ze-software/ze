// RFC: rfc/short/rfc4271.md — Section 6.8, connection collision detection
// RFC 4486 Section 4 -- see rfc/short/rfc4486.md; admission refusal subcode.
// Design: docs/architecture/core-design.md — TCP accept and collision detection (RFC 4271 §6.8)
// Overview: reactor.go — BGP reactor event loop and peer management

package reactor

import (
	"context"
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
// Static peers must match that declaration or a verified address of the same
// stored socket. Dynamic peers are already qualified by the accepted endpoint,
// which can differ from a wildcard declaration.
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

	// A dynamic template's endpoint is already checked. For a static peer,
	// generic TCP may have translated the original wildcard declaration; only
	// the existing socket can establish that its other declaration is eligible.
	if !settings.IsDynamic {
		if settings.LocalAddress.IsValid() {
			if r.sameTransportAddress(settings.LocalAddress, listenerAddr) != transportAddressSame {
				local, ok := conn.LocalAddr().(*net.TCPAddr)
				if !ok {
					closeConnQuietly(conn)
					return
				}
				if local == nil {
					closeConnQuietly(conn)
					return
				}
				port := local.Port
				if r.peerListenPort(settings) != port {
					closeConnQuietly(conn)
					return
				}
				r.mu.RLock()
				binding := r.listenerBindingForAddressPort(listenerAddr, port)
				match := binding.matches(r, settings.LocalAddress)
				r.mu.RUnlock()
				if match != transportAddressSame {
					closeConnQuietly(conn)
					return
				}
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
//   - ownership is known and exactly one object in the configuration claims
//     that address and port, and it is a passive peer. listenerClaimants counts
//     a dynamic group too, because a group accepts on the same socket.
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
	count, only, known := r.listenerClaimants(addr, port)
	if !known {
		return netip.AddrPort{}
	}
	if count != 1 {
		return netip.AddrPort{}
	}
	if only == nil {
		return netip.AddrPort{}
	}
	if port == DefaultBGPPort {
		return netip.AddrPort{}
	}
	return only.PeerKey()
}

// listenerClaimants reports how many configured objects need the binding at
// addr:port, and the settings of the single passive peer among them when there
// is exactly one claimant and no dynamic group. The final result says whether
// the count is known: an unreadable interface table leaves possible aliases
// unknown, so callers MUST NOT infer exclusive ownership or an unused socket.
//
// A claimant is a passive peer or dynamic group on this listen port whose local
// declaration identifies this stored socket or its actual endpoint. With no
// stored socket, only the supplied address is known. A dynamic group accepts
// connections nothing in the configuration names. That is why it opens a
// listener of its own (startMultiListeners), and why a listener it shares must
// attribute by remote IP.
// Must be called with r.mu held.
func (r *Reactor) listenerClaimants(addr netip.Addr, port int) (int, *PeerSettings, bool) {
	return r.listenerClaimantsForBinding(r.listenerBindingForAddressPort(addr, port))
}

// listenerClaimantsForBinding counts each configured claimant once across the
// declaration and observed endpoint of one stored socket. The caller MUST hold
// r.mu while constructing and counting the binding.
func (r *Reactor) listenerClaimantsForBinding(binding listenerBinding) (int, *PeerSettings, bool) {
	if binding.declarationErr != nil {
		return 0, nil, false
	}
	count := 0
	var only *PeerSettings
	for _, peer := range r.peers {
		s := peer.Settings()
		if !s.LocalAddress.IsValid() || !s.Connection.IsPassive() {
			continue
		}
		if r.peerListenPort(s) != binding.port {
			continue
		}
		match := binding.matches(r, s.LocalAddress)
		if match == transportAddressUnknown {
			return 0, nil, false
		}
		if match != transportAddressSame {
			continue
		}
		count++
		only = s
	}
	for _, dg := range r.dynamicGroups {
		if dg.Settings == nil || !dg.Settings.LocalAddress.IsValid() || !dg.Settings.Connection.IsPassive() {
			continue
		}
		if r.peerListenPort(dg.Settings) != binding.port {
			continue
		}
		match := binding.matches(r, dg.Settings.LocalAddress)
		if match == transportAddressUnknown {
			return 0, nil, false
		}
		if match != transportAddressSame {
			continue
		}
		count++
		only = nil
	}
	if count != 1 {
		return count, nil, true
	}
	return count, only, true
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

// acceptOrReject admits incoming connections and reserves their collision exchange.
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
			r.rejectConnectionAdmissionWithSettings(conn, settings)
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

func (r *Reactor) rejectConnectionAdmissionWithSettings(conn net.Conn, settings *PeerSettings) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		if err := tuneTCPConnectionForSettings(tcp, settings); err != nil {
			reportPendingRefusalFailure(conn, "tune TCP admission-refusal settings", err)
		}
	}
	// RFC 4486 Section 4: "If a BGP speaker decides to disallow a BGP
	// connection (e.g., the peer is not configured locally) after the speaker
	// accepts a transport protocol connection, then the BGP speaker SHOULD send
	// a NOTIFICATION message with the Error Code Cease and the Error Subcode
	// "Connection Rejected"."
	rejectPendingConnection(conn, r.clock, message.NotifyCeaseConnectionRejected)
}

// rejectConnectionCollision sends NOTIFICATION Cease/Connection Collision (6/7)
// and closes the connection, bounded by the normal control-write maximum.
// RFC 4271 Section 6.8 requires Cease for this OPEN-bearing collision loser.
func rejectConnectionCollision(conn net.Conn, clk clock.Clock) {
	rejectPendingConnection(conn, clk, message.NotifyCeaseConnectionCollision)
}

// rejectPendingConnection bounds a refusal on a connection with no Session
// writer. OPEN decode errors retain their own code before collision resolution.
// It MUST close conn after every outcome; reportPendingRefusalFailure reports
// write failures but leaves that ownership here.
func rejectPendingConnection(conn net.Conn, clk clock.Clock, subcode uint8) {
	defer closeConnQuietly(conn)
	if err := conn.SetWriteDeadline(clk.Now().Add(fwdWriteDeadlineDefault)); err != nil {
		reportPendingRefusalFailure(conn, "set write deadline", err)
		return
	}
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: subcode,
	}
	data := message.PackTo(notif, nil)
	if _, err := conn.Write(data); err != nil {
		reportPendingRefusalFailure(conn, "write NOTIFICATION", err)
	}
}

// PausePeer pauses reading from a specific peer's session.
// Returns ErrPeerNotFound if the peer address is unknown.
func (r *Reactor) PausePeer(ctx context.Context, addr netip.Addr, owner, connection uint64) error {
	return r.peerReadControl(ctx, addr, owner, connection, true)
}

// ResumePeer resumes reading from a specific peer's session.
// Returns ErrPeerNotFound if the peer address is unknown.
func (r *Reactor) ResumePeer(ctx context.Context, addr netip.Addr, owner, connection uint64) error {
	return r.peerReadControl(ctx, addr, owner, connection, false)
}

// peerReadControl resolves an original owner before touching any Session. The
// reactor and peer locks keep that original map entry and physical session
// current until Pause/Resume finishes. An address is only a constraint beside
// numeric identity. The explicit zero pair retains operator address selection.
func (r *Reactor) peerReadControl(ctx context.Context, addr netip.Addr, owner, connection uint64, pause bool) error {
	if owner == 0 {
		if connection != 0 {
			return errors.New("physical peer flow control requires its original peer-owner")
		}
	} else {
		if connection == 0 {
			return errors.New("physical peer flow control requires its original connection-id")
		}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var original *Peer
	if owner == 0 {
		_, peer, ok := r.findPeerKeyByAddr(addr)
		if !ok {
			return ErrPeerNotFound
		}
		original = peer
	} else {
		for key, peer := range r.peers {
			if peer.sourceOwner != owner {
				continue
			}
			if key.Addr() != addr {
				continue
			}
			original = peer
			break
		}
		if original == nil {
			return errors.New("physical peer flow control refused: the original peer owner is no longer present")
		}
	}
	original.mu.RLock()
	defer original.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	session := original.session
	if owner != 0 {
		if original.stopping.Load() {
			return errors.New("physical peer flow control refused: the original peer is stopping")
		}
		if session == nil {
			return errors.New("physical peer flow control refused: the original physical session has ended")
		}
		if session.connectionID != connection {
			return errors.New("physical peer flow control refused: the original physical connection was replaced")
		}
	}
	if session == nil {
		return nil
	} // Explicit unbound operator form keeps its historical no-active-session no-op.
	return session.peerReadControl(ctx, pause)
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
