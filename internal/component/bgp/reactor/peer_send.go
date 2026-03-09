// Design: docs/architecture/core-design.md — BGP UPDATE sending
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"fmt"
	"net/netip"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// SendUpdate sends a BGP UPDATE message to this peer.
// Returns ErrNotConnected if no session is active.
// Returns an error if the session is not in ESTABLISHED state.
func (p *Peer) SendUpdate(update *message.Update) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendUpdate(update)
}

// SendAnnounce sends a BGP UPDATE message for announcing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrNotConnected if no session is active.
//
// RFC 4271 Section 4.3 - UPDATE Message Format.
// RFC 4760 Section 3 - MP_REACH_NLRI for IPv6 routes.
// RFC 7911 - ADD-PATH encoding based on negotiated capabilities.
func (p *Peer) SendAnnounce(route bgptypes.RouteSpec, localAS uint32) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	isIBGP := p.settings.IsIBGP()
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	if route.Prefix.Addr().Is6() {
		family = nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
	}
	asn4 := p.asn4()
	addPath := p.addPathFor(family)

	if err := session.SendAnnounce(route, localAS, isIBGP, asn4, addPath); err != nil {
		return err
	}
	p.IncrRoutesSent(1)
	return nil
}

// SendWithdraw sends a BGP UPDATE message for withdrawing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrNotConnected if no session is active.
//
// RFC 4271 Section 4.3 - UPDATE Message Format (Withdrawn Routes for IPv4).
// RFC 4760 Section 4 - MP_UNREACH_NLRI for IPv6 withdrawals.
// RFC 7911 - ADD-PATH encoding based on negotiated capabilities.
func (p *Peer) SendWithdraw(prefix netip.Prefix) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	if prefix.Addr().Is6() {
		family = nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
	}
	addPath := p.addPathFor(family)

	if err := session.SendWithdraw(prefix, addPath); err != nil {
		return err
	}
	p.IncrWithdrawsSent(1)
	return nil
}

// SendRawUpdateBody sends a pre-encoded UPDATE message body (without BGP header).
// Used for zero-copy forwarding when encoding contexts match.
// Returns ErrNotConnected if no session is active.
func (p *Peer) SendRawUpdateBody(body []byte) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendRawUpdateBody(body)
}

// SendRawMessage sends raw bytes to the peer.
// If msgType is 0, payload is a full BGP packet.
// If msgType is non-zero, payload is message body only.
func (p *Peer) SendRawMessage(msgType uint8, payload []byte) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendRawMessage(msgType, payload)
}

// sendUpdateWithSplit sends an UPDATE, splitting if it exceeds maxSize.
// Uses SplitUpdateWithAddPath to chunk oversized messages into multiple UPDATEs.
// The addPath parameter must match the encoding used to build the UPDATE's NLRIs.
// Returns nil on success, first error encountered on failure.
//
// RFC 4271 Section 4.3: Each split UPDATE is self-contained with full attributes.
// RFC 7911: Add-Path requires 4-byte path identifier before each NLRI.
// RFC 8654: Respects peer's max message size (4096 or 65535).
func (p *Peer) sendUpdateWithSplit(update *message.Update, maxSize int, addPath bool) error {
	chunks, err := message.SplitUpdateWithAddPath(update, maxSize, addPath)
	if err != nil {
		// Attributes too large or single NLRI too large - cannot send
		return fmt.Errorf("splitting update: %w", err)
	}

	for _, chunk := range chunks {
		if err := p.SendUpdate(chunk); err != nil {
			return err
		}
	}
	return nil
}

// PauseReading pauses reading from this peer's session.
// If no active session exists, this is a no-op.
func (p *Peer) PauseReading() {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return
	}

	session.Pause()
	peerLogger().Debug("read paused", "peer", p.settings.Address)
}

// ResumeReading resumes reading from this peer's session.
// If no active session exists, this is a no-op.
func (p *Peer) ResumeReading() {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return
	}

	session.Resume()
	peerLogger().Debug("read resumed", "peer", p.settings.Address)
}

// IsReadPaused reports whether this peer's session read loop is paused.
// Returns false if no active session exists.
func (p *Peer) IsReadPaused() bool {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return false
	}

	return session.IsPaused()
}
