// Design: rfc/short/rfc7606.md -- Section 5.4, typed NLRI
// RFC: rfc/short/rfc7606.md -- Section 5.4; RFC 6514 Section 4 -- MCAST-VPN NLRI
// Related: types.go -- MVPNRouteType constants and String, the set this reports on
//
// RFC 7606 Section 5.4 binds MCAST-VPN, and names it first: "Certain address
// families, for example, MCAST-VPN [RFC6514], MCAST-VPLS [RFC7117], and EVPN
// [RFC7432] have NLRI that are typed."
//
// RFC 6514 Section 4 defines the framing and enumerates the route types (1..5 for
// A-D routes, 6..7 for C-multicast routes). It never invokes Section 5.4's
// "unless the relevant specification for that address family specifies otherwise"
// clause, and states no other handling for a route type a speaker does not
// implement, so the Section 5.4 default applies: discard.
//
// This is where that ruling is recorded, beside the family registration it
// belongs to. Nothing central names MCAST-VPN.

package mvpn

// Implemented reports whether ze implements this MCAST-VPN route type.
//
// This is Section 5.4's "recognized" set for the family. RecognizeNLRI reads it,
// and TestImplementedMatchesRouteTypeNames holds it against String's own dispatch
// so the two cannot drift: a type ze can name is a type ze recognizes, and a type
// String renders as "type(N)" is one it does not.
func (t MVPNRouteType) Implemented() bool {
	switch t {
	case MVPNIntraASIPMSIAD, MVPNInterASIPMSIAD, MVPNSPMSIAD, MVPNLeafAD,
		MVPNSourceActive, MVPNSharedTreeJoin, MVPNSourceTreeJoin:
		return true
	}
	return false
}

// RecognizeNLRI reports whether one MCAST-VPN NLRI carries a route type ze
// implements, per RFC 7606 Section 5.4.
//
// nlriBytes is one NLRI as it appears on the wire. RFC 6514 Section 4 frames it
// as [route-type:1][length:1][route-type-specific:length], preceded by a 4-byte
// RFC 7911 path identifier when addPath is negotiated.
//
// A slice too short to hold a route type is not recognized. That is the
// fail-closed answer, and it costs nothing real: the framing walk that carved
// this slice would have reported the truncation first.
func RecognizeNLRI(nlriBytes []byte, addPath bool) bool {
	off := 0
	if addPath {
		off = 4
	}
	if off >= len(nlriBytes) {
		return false
	}
	return MVPNRouteType(nlriBytes[off]).Implemented()
}
