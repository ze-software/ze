// Design: rfc/short/rfc7606.md -- Section 5.4, typed NLRI
// RFC: rfc/short/rfc7606.md -- Section 5.4; rfc/short/rfc7432.md -- Section 7.1 EVPN NLRI
// Related: types.go -- EVPNRouteType constants and ParseEVPN, the set this reports on
//
// RFC 7606 Section 5.4 binds EVPN, and names it as one of its own examples of a
// typed address family. RFC 7432 defines the typed NLRI framing (Section 7.1) and
// creates the "EVPN Route Types" IANA registry (Section 20, IANA Considerations)
// but states no deviation from RFC 7606, so the default applies: a route whose
// type ze does not implement is discarded.
//
// This is where that ruling is recorded, beside the family registration it
// belongs to. Nothing central names EVPN.

package evpn

// Implemented reports whether ze implements this EVPN route type.
//
// This is Section 5.4's "recognized" set. It is the single source of truth for
// it: RecognizeNLRI reads it, and TestImplementedMatchesParseEVPN holds it
// against ParseEVPN's own dispatch so the two cannot drift.
func (t EVPNRouteType) Implemented() bool {
	switch t {
	case EVPNRouteType1, EVPNRouteType2, EVPNRouteType3, EVPNRouteType4, EVPNRouteType5:
		return true
	}
	return false
}

// RecognizeNLRI reports whether one EVPN NLRI carries a route type ze
// implements, per RFC 7606 Section 5.4.
//
// nlriBytes is one NLRI as it appears on the wire. RFC 7432 Section 7.1 frames
// it as [route-type:1][length:1][body], preceded by a 4-byte RFC 7911 path
// identifier when addPath is negotiated.
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
	return EVPNRouteType(nlriBytes[off]).Implemented()
}
