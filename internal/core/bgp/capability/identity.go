// Design: docs/architecture/wire/capabilities.md — capability negotiation
// RFC: rfc/short/rfc5492.md — capability code registry

package capability

// PeerIdentity holds peer identification data from OPEN message exchange.
// Shared between Negotiated and EncodingContexts (recv/send).
// Immutable after session creation.
type PeerIdentity struct {
	// LocalASN is our AS number.
	LocalASN uint32
	// PeerASN is the peer's AS number from OPEN.
	PeerASN uint32
	// LocalRouterID is our router ID from config.
	LocalRouterID uint32
	// PeerRouterID is the peer's router ID from OPEN.
	PeerRouterID uint32
	// Internal is the session's own verdict on whether the peer is internal, decided by
	// the reactor before negotiation and carried here. IsIBGP returns it unchanged.
	Internal bool
}

// IsIBGP returns true if this is an iBGP session.
// RFC 4271: iBGP sessions have different path attribute rules.
//
// The verdict is CARRIED, not recomputed here. Equality of the two AS numbers is the
// ordinary rule but not the whole rule: RFC 7705 Section 4.2 lets a renumbering speaker
// form one iBGP session under either of its two AS numbers, and requires it to "treat
// UPDATEs sent and received to this peer as if this was a natively configured iBGP
// session". A migrating session therefore has LocalASN != PeerASN and is internal.
//
// PeerSettings.isIBGPWith (reactor/session_as_migration.go) is the ONE rule that decides
// it, and negotiateWith puts its answer in this field. Recomputing the equality here was a
// second declaration of the verdict, and it was fed a PeerASN of 0 on every session.
func (p *PeerIdentity) IsIBGP() bool {
	return p.Internal
}
