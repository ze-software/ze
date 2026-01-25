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
}

// IsIBGP returns true if this is an iBGP session (same AS).
// RFC 4271: iBGP sessions have different path attribute rules.
func (p *PeerIdentity) IsIBGP() bool {
	return p.LocalASN == p.PeerASN
}
