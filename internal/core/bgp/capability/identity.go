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

// OpenPeerAS returns the AS a received OPEN names for a session, given the OPEN's My AS
// field, its parsed capabilities and whether this side advertises the Four-octet AS
// capability (localASN4).
//
// RFC 6793 Section 4.1: "When a NEW BGP speaker processes an OPEN message from another
// NEW BGP speaker, it MUST use the AS number encoded in the Capability Value field of the
// "support for four-octet AS number capability" in lieu of the "My Autonomous System"
// field of the OPEN message."
//
// The obligation binds only when BOTH ends are NEW toward each other, and the same
// section says how a speaker is NEW toward a peer: "A BGP speaker that supports
// four-octet AS numbers SHALL advertise this to its peers using BGP Capabilities
// Advertisements." localASN4 is that advertisement. A session configured with
// `capability { asn4 disable; }` (or refuse) never offers the capability, so toward
// that peer ze is an OLD BGP speaker (Section 3): the session is two-octet, and the peer
// AS is the "My Autonomous System" field, AS_TRANS for a peer with no two-octet AS
// (Section 4.2.1). Reading the capability there judged the peer by a value the session
// never negotiated, and refused a peer configured as remote 23456 with Bad Peer AS.
// ExaBGP reads the capability under the same condition (Negotiated.received,
// `if self.asn4:`, where asn4 is sent AND received).
//
// A capability AS of zero is returned as zero, so the RFC 7607 zero check in
// validateOpenPeerAS (reactor/session_open_as.go) sees it.
//
// This is the one declaration of the rule. The reactor reaches it through
// openAdvertisedAS (reactor/peer.go) and validateOpenPeerAS, and the OPEN-validator RPC
// through remoteOpenAS (bgp/server/validate.go), so a plugin is told the AS the session
// judged.
func OpenPeerAS(myAS uint16, caps []Capability, localASN4 bool) uint32 {
	if !localASN4 {
		return uint32(myAS)
	}
	for _, entry := range caps {
		if asn4, ok := entry.(*ASN4); ok {
			return asn4.ASN
		}
	}
	return uint32(myAS)
}

// AdvertisesASN4 reports whether caps carries the Four-octet AS capability: the localASN4
// argument OpenPeerAS takes, read from the OPEN this side sent.
func AdvertisesASN4(caps []Capability) bool {
	for _, entry := range caps {
		if _, ok := entry.(*ASN4); ok {
			return true
		}
	}
	return false
}
