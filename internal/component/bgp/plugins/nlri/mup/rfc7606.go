// Design: rfc/short/rfc7606.md -- Section 5.4, typed NLRI
// RFC: rfc/short/rfc7606.md -- Section 5.4; rfc/short/draft-ietf-bess-mup-safi.md -- Section 3.1
// Related: types.go -- MUPRouteType and MUPArchType constants, the set this reports on
//
// RFC 7606 Section 5.4 binds BGP-MUP. The NLRI is typed in the sense Section 5.4
// describes: the supported type values are not carried in the RFC 4760
// multiprotocol capability, so a speaker can advertise ipv4/mup or ipv6/mup and
// still not implement a given type inside it.
//
// draft-ietf-bess-mup-safi Section 3.1 defines the envelope and enumerates
// Architecture Type 1 (3gpp-5g) and Route Types 1..4. It never invokes Section
// 5.4's "unless the relevant specification for that address family specifies
// otherwise" clause, so the Section 5.4 default applies: discard.
//
// This is where that ruling is recorded, beside the family registration it
// belongs to. Nothing central names MUP.

package mup

// Implemented reports whether ze implements this MUP architecture and route type
// pair.
//
// Both halves decide. draft-ietf-bess-mup-safi Section 3.1 says the Route Type
// specific encoding "depends on Architecture Type + Route Type", so the pair is
// what names an NLRI type, and route type 1 under an architecture ze does not
// implement is as unreadable as an unassigned route type.
//
// This is Section 5.4's "recognized" set for the family. RecognizeNLRI reads it,
// and TestImplementedMatchesRouteTypeNames holds the route-type half against
// String's own dispatch so the two cannot drift.
func (t MUPRouteType) Implemented(arch MUPArchType) bool {
	if arch != MUPArch3GPP5G {
		return false
	}
	switch t {
	case MUPISD, MUPDSD, MUPT1ST, MUPT2ST:
		return true
	}
	return false
}

// RecognizeNLRI reports whether one BGP-MUP NLRI carries an architecture and
// route type ze implements, per RFC 7606 Section 5.4.
//
// nlriBytes is one NLRI as it appears on the wire. draft-ietf-bess-mup-safi
// Section 3.1 frames it as [architecture-type:1][route-type:2][length:1][body],
// preceded by a 4-byte RFC 7911 path identifier when addPath is negotiated.
//
// A slice too short to hold the architecture and route type is not recognized.
// That is the fail-closed answer, and it costs nothing real: the framing walk
// that carved this slice would have reported the truncation first.
func RecognizeNLRI(nlriBytes []byte, addPath bool) bool {
	off := 0
	if addPath {
		off = 4
	}
	if off+3 > len(nlriBytes) {
		return false
	}
	arch := MUPArchType(nlriBytes[off])
	routeType := MUPRouteType(uint16(nlriBytes[off+1])<<8 | uint16(nlriBytes[off+2]))
	return routeType.Implemented(arch)
}
