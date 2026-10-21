// Design: docs/architecture/core-design.md — peer run loop and session lifecycle
// Overview: peer.go — Peer struct, accessors, lifecycle API
// Related: peer_run.go — the run loop parks in holdAdminDown
// RFC: rfc/short/rfc4486.md -- Section 4, Administrative Shutdown and Connection Rejected
// RFC: rfc/short/rfc9003.md -- Section 2, Shutdown Communication

package reactor

import (
	"net"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// AdminShutdown is the peer `shutdown` container (ze-bgp-conf.yang). Its zero
// value is a peer with no container, which is a legitimate state: the peer
// connects as usual.
type AdminShutdown struct {
	// Held is true when the container is present: the peer starts
	// administratively down and opens no session until `request peer
	// <selector> enable`.
	Held bool
	// Message is the RFC 9003 Shutdown Communication sent in the Cease when a
	// reload adds the container to a peer whose session is up. Empty sends the
	// Cease with no communication.
	Message string
}

// Disable holds the peer administratively down: it closes a running session
// with Cease, Administrative Shutdown and the given Shutdown Communication,
// and opens no session, outbound or inbound, until Enable. A peer that has no
// session is held all the same, so it does not connect until Enable.
//
// The flag and the session are read in one p.mu hold, and runOnce publishes a
// new session in a p.mu hold that reads the flag, so either this call sees the
// session and closes it, or runOnce sees the flag and abandons the attempt.
// Safe for concurrent use.
func (p *Peer) Disable(shutdownMsg string) error {
	p.mu.Lock()
	p.adminDown.Store(true)
	session := p.session
	p.mu.Unlock()
	p.wakeAdmin()

	if session == nil {
		return nil
	}
	// RFC 4486 Section 4: "If a BGP speaker decides to administratively shut
	// down its peering with a neighbor, then the speaker SHOULD send a
	// NOTIFICATION message with the Error Code Cease and the Error Subcode
	// "Administrative Shutdown"."
	// RFC 9003 Section 2 (the communication is carried by sessionTeardown).
	return p.Teardown(message.NotifyCeaseAdminShutdown, shutdownMsg)
}

// Enable releases an administrative hold, so the peer connects straight away.
// Enabling a peer that is not held changes nothing. Safe for concurrent use.
func (p *Peer) Enable() {
	p.adminDown.Store(false)
	p.wakeAdmin()
}

// AdminDown reports whether the peer is held administratively down.
func (p *Peer) AdminDown() bool { return p.adminDown.Load() }

// wakeAdmin tells the run loop the hold changed. The channel holds one token,
// and a token already waiting carries the same news, so a full channel is not
// an error.
func (p *Peer) wakeAdmin() {
	select {
	case p.adminWake <- struct{}{}:
	default: // a wake is already pending
	}
}

// holdAdminDown parks the run loop while the peer is held administratively
// down, and returns when Enable released it or the peer stopped. The caller
// re-reads p.ctx.
//
// The wait is a blocking select, never a timer loop: a held peer costs nothing.
// An inbound connection is answered with Cease, Connection Rejected, because a
// peer that gets back in through its own retry is not held down at all.
func (p *Peer) holdAdminDown() {
	p.setState(PeerStateIdleHold)
	peerLogger().Info("peer held administratively down",
		"peer", p.settings.Address,
		"action", "request peer <selector> enable, or delete the peer shutdown container and commit",
	)
	for p.adminDown.Load() {
		select {
		case <-p.ctx.Done():
			return
		case <-p.adminWake:
		case <-p.inboundNotify:
			if inbound := p.takeInboundConnection(); inbound.conn != nil {
				rejectAdminDownConnection(inbound.conn)
			}
		}
	}
	p.setState(PeerStateConnecting)
	peerLogger().Info("peer administrative hold released", "peer", p.settings.Address)
}

// rejectAdminDownConnection answers a connection that reached a held peer.
//
// RFC 4486 Section 4: "If a BGP speaker decides to disallow a BGP connection
// (e.g., the peer is not configured locally) after the speaker accepts a
// transport protocol connection, then the BGP speaker SHOULD send a
// NOTIFICATION message with the Error Code Cease and the Error Subcode
// "Connection Rejected"."
func rejectAdminDownConnection(conn net.Conn) {
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseConnectionRejected,
	}
	if _, err := conn.Write(message.PackTo(notif, nil)); err != nil {
		peerLogger().Debug("connection rejected write failed", "error", err)
	}
	closeConnQuietly(conn)
}
