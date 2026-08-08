// RFC: rfc/short/rfc4271.md
// Design: docs/architecture/core-design.md — BGP UPDATE sending
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"errors"
	"fmt"
	"net/netip"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/ze-software/ze/internal/component/bgp/message"
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
//
// RFC 2545 Section 3: the link-local half of the Next Hop field is decided here,
// because the section's condition is a property of this session and this next hop
// rather than of the route the caller describes. linkLocalNextHopFor
// (link_scope.go) answers it against the host interface table, and returns the
// zero Addr for every case that must carry the global address alone.
func (p *Peer) SendAnnounce(route bgptypes.RouteSpec, localAS uint32) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	isIBGP := p.settings.IsIBGP()
	fam := family.IPv4Unicast
	if route.Prefix.Addr().Is6() {
		fam = family.IPv6Unicast
	}
	asn4 := p.asn4()
	addPath := p.addPathFor(fam)
	linkLocal := p.linkLocalNextHopFor(route.NextHop.Addr)

	if err := session.SendAnnounce(route, linkLocal, localAS, isIBGP, asn4, addPath); err != nil {
		return err
	}
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

	fam := family.IPv4Unicast
	if prefix.Addr().Is6() {
		fam = family.IPv6Unicast
	}
	addPath := p.addPathFor(fam)

	if err := session.SendWithdraw(prefix, addPath); err != nil {
		return err
	}
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

// sendUpdateWithSplit sends an UPDATE, splitting via Splitter.Split when it
// exceeds maxSize. The addPath parameter must match the encoding used to build
// the UPDATE's NLRIs. Returns nil on success, the first error encountered
// otherwise.
//
// Each chunk is handed to p.SendUpdate (which copies synchronously through
// WriteTo) before the next chunk is built -- this satisfies the splitter's
// scratch-aliasing lifetime invariant (see Update type doc).
//
// RFC 4271 Section 4.3: Each split UPDATE is self-contained with full attributes.
// RFC 7911: Add-Path requires 4-byte path identifier before each NLRI.
// RFC 8654: Respects peer's max message size (4096 or 65535).
//
// A nil update means the builder REJECTED the build: the message did not fit its
// pooled 4096-byte slot and no bytes were produced (buildRIBRouteUpdate,
// buildWithdrawNLRI, buildBatchAnnounceUpdate). Splitter.Split would dereference
// it, so the nil is turned into errBuildRejected here, at the single choke point
// every send passes through, rather than repeated at each builder's call site.
func (p *Peer) sendUpdateWithSplit(update *message.Update, maxSize int, addPath bool) error {
	if update == nil {
		return errBuildRejected
	}
	s := message.GetSplitter()
	defer message.PutSplitter(s)
	if err := s.Split(update, maxSize, addPath, p.SendUpdate); err != nil {
		return fmt.Errorf("splitting update: %w", err)
	}
	return nil
}

// errBuildRejected reports that a builder could not encode an UPDATE into its
// pooled build buffer, so nothing was sent. The route is lost, the SESSION is not:
// the failure belongs to this one message (see isRouteScopedSendError).
var errBuildRejected = errors.New("update build rejected: message does not fit the build buffer")

// isRouteScopedSendError reports whether err condemns only the message that was
// being sent, as opposed to the connection carrying it. The queue drains skip the
// offending route and keep going for these, and tear the session down for anything
// else -- so mis-classifying a connection error as route-scoped spins the drain
// loop against a dead socket, and the reverse drops a session over one unencodable
// route.
func isRouteScopedSendError(err error) bool {
	return errors.Is(err, message.ErrAttributesTooLarge) ||
		errors.Is(err, message.ErrNLRITooLarge) ||
		errors.Is(err, errBuildRejected)
}

// sendBodyWithSplit reconstructs a *message.Update from a flat UPDATE body
// (RFC 4271 Section 4.3: withdrawn-length + withdrawn + attr-length + attrs +
// NLRI) and sends it with message splitting. Used by the LLGR stale-readvertise
// path, which applies attribute modifications on the flat body (buildModifiedPayload)
// and then needs the same size-aware split send as the batch announce rail. The
// section slices are copied because the caller's flat body is pooled/transient.
func (p *Peer) sendBodyWithSplit(body []byte, maxSize int, addPath bool) error {
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		return fmt.Errorf("parse modified update body: %w", err)
	}
	u := &message.Update{
		WithdrawnRoutes: append([]byte(nil), sec.Withdrawn(body)...),
		PathAttributes:  append([]byte(nil), sec.Attrs(body)...),
		NLRI:            append([]byte(nil), sec.NLRI(body)...),
	}
	return p.sendUpdateWithSplit(u, maxSize, addPath)
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
