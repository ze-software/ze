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

import "github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"

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
	//exhaustive:ignore // Recognizes only implemented route types under the supported MUP architecture.
	switch t {
	case MUPISD, MUPDSD, MUPT1ST, MUPT2ST:
		return true
	}
	return false
}

// RecognizeNLRI answers RFC 7606 Section 5.4's TYPE question for one BGP-MUP
// NLRI: Keep when ze implements its architecture and route type, Discard otherwise.
//
// nlriBytes is one NLRI as it appears on the wire. draft-ietf-bess-mup-safi
// Section 3.1 frames it as [architecture-type:1][route-type:2][length:1][body],
// preceded by a 4-byte RFC 7911 path identifier when addPath is negotiated.
//
// A slice too short to hold the architecture and route type is discarded.
// That is the fail-closed answer, and it costs nothing real: the framing walk
// that carved this slice would have reported the truncation first.
func RecognizeNLRI(nlriBytes []byte, addPath bool) nlritype.Verdict {
	off := 0
	if addPath {
		off = 4
	}
	if off+3 > len(nlriBytes) {
		return nlritype.Discard
	}
	arch := MUPArchType(nlriBytes[off])
	routeType := MUPRouteType(uint16(nlriBytes[off+1])<<8 | uint16(nlriBytes[off+2]))
	if routeType.Implemented(arch) {
		return nlritype.Keep
	}
	return nlritype.Discard
}

// recognizerFor returns the receive-path recognizer registered for the MUP
// family of afi: RecognizeNLRI's type verdict, then ValidateNLRI's body verdict
// over an implemented type. The body checks need the AFI, which is why each
// family registers its own.
//
// draft-ietf-bess-mup-safi Section 3.1.3.1 answers a malformed Type 1 Session
// Transformed route with treat-as-withdraw, by the count of octets left after the
// mandatory fields: "< 0: the mandatory fields exceed the declared Length; the
// NLRI is malformed; MUST be treated as Treat-as-withdraw.", "1: encoding is
// invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be
// treated as Treat-as-withdraw.", and "any TLV parsing error MUST result in
// Treat-as-withdraw". Every other malformed shape ParseMUP refuses gets the same
// answer (owner ruling Q-1 (a), spec-bgp-update-propagation-rfc-defects).
func recognizerFor(afi AFI) nlritype.Recognizer {
	return func(nlriBytes []byte, addPath bool) nlritype.Verdict {
		if v := RecognizeNLRI(nlriBytes, addPath); v != nlritype.Keep {
			return v
		}
		one := nlriBytes
		if addPath {
			one = nlriBytes[4:]
		}
		if ValidateNLRI(afi, one) != nil {
			return nlritype.Withdraw
		}
		return nlritype.Keep
	}
}
