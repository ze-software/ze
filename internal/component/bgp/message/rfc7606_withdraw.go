// Design: docs/architecture/core-design.md — RFC 7606 treat-as-withdraw synthesis
// Overview: rfc7606.go — RFC 7606 UPDATE validation and action selection
// RFC: rfc/short/rfc7606.md — revised UPDATE error handling

package message

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
)

// MP_UNREACH_NLRI is Optional, non-transitive (RFC 4760 Section 4): flags 0x80.
const (
	mpUnreachFlags          = 0x80
	mpUnreachExtendedFlags  = 0x90 // Optional + Extended Length
	extendedLengthThreshold = 255

	// MP_REACH_NLRI value layout (RFC 4760 Section 3):
	//   AFI(2) SAFI(1) NextHopLen(1) NextHop(n) Reserved(1) NLRI...
	mpReachAFISAFILen    = 3
	mpReachNextHopLenOff = 3
	mpReachFixedPrefix   = 4 // AFI(2) + SAFI(1) + NextHopLen(1)
)

// SynthesizeWithdraw rewrites an UPDATE body so every route it announces becomes a
// withdrawal, and returns (newBody, changed).
//
// RFC 7606 Section 2: an UPDATE handled with treat-as-withdraw "MUST be handled as though
// all of the routes contained in [it] ... had been withdrawn from service", "thus causing
// them to be removed from the Adj-RIB-In according to the procedures of [RFC4271]".
//
// Not dispatching the UPDATE is NOT that, and the difference is operational rather than
// academic: a prefix already in the Adj-RIB-In, re-announced with a malformed attribute,
// would simply keep its previous entry and go stale. The RFC's whole point is that the
// route goes away. So the announced routes are converted into withdrawals and the UPDATE
// is dispatched normally, which is what actually removes them.
//
// The transformation:
//   - trailing IPv4 NLRI moves into the Withdrawn Routes field
//   - MP_REACH_NLRI becomes MP_UNREACH_NLRI for the same AFI/SAFI (next hop dropped)
//   - an existing MP_UNREACH_NLRI is kept as-is
//   - every other path attribute is dropped: a withdrawal carries none, and those
//     attributes are the very thing that failed validation
//
// Returns changed=false, body unchanged, when there is nothing to withdraw (e.g. an
// End-of-RIB) or when the section lengths are structurally unusable. The latter cannot
// legitimately reach here -- Section 3(b) session-resets a length conflict, and Section
// 3(j) forbids treat-as-withdraw unless the NLRI parsed -- so refusing to guess is the
// fail-closed choice rather than manufacturing a withdrawal from untrustworthy offsets.
func SynthesizeWithdraw(body []byte) ([]byte, bool) {
	if len(body) < 4 {
		return body, false
	}

	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	withdrawnEnd := 2 + withdrawnLen
	if withdrawnEnd+2 > len(body) {
		return body, false
	}

	attrLen := int(binary.BigEndian.Uint16(body[withdrawnEnd : withdrawnEnd+2]))
	attrStart := withdrawnEnd + 2
	if attrStart+attrLen > len(body) {
		return body, false
	}

	withdrawn := body[2:withdrawnEnd]
	pathAttrs := body[attrStart : attrStart+attrLen]
	nlri := body[attrStart+attrLen:]

	newAttrs, mpChanged := withdrawMPAttrs(pathAttrs)

	// Nothing was announced: no IPv4 NLRI and no MP_REACH to convert. An End-of-RIB
	// (MP_UNREACH only, no NLRI) lands here and must pass through untouched -- inventing
	// a withdrawal for it would misreport what the peer actually said.
	if len(nlri) == 0 && !mpChanged {
		return body, false
	}

	newWithdrawnLen := len(withdrawn) + len(nlri)
	out := make([]byte, 2+newWithdrawnLen+2+len(newAttrs))

	//nolint:gosec // bounded by BGP message size (max 65535)
	binary.BigEndian.PutUint16(out[0:2], uint16(newWithdrawnLen))
	n := 2
	n += copy(out[n:], withdrawn)
	// The announced prefixes become withdrawals. IPv4 unicast NLRI and Withdrawn Routes
	// share an encoding (RFC 4271 Section 4.3), so this is a straight append.
	n += copy(out[n:], nlri)
	//nolint:gosec // bounded by BGP message size (max 65535)
	binary.BigEndian.PutUint16(out[n:n+2], uint16(len(newAttrs)))
	n += 2
	copy(out[n:], newAttrs)

	return out, true
}

// withdrawMPAttrs keeps MP_UNREACH_NLRI, rewrites MP_REACH_NLRI into MP_UNREACH_NLRI, and
// drops every other attribute. Returns (attrs, changed) where changed reports whether an
// MP_REACH was converted -- i.e. whether the UPDATE announced multiprotocol routes.
//
// Dropping the other attributes is the point, not an oversight: RFC 4271 Section 4.3 gives
// a withdrawal no path attributes, and these are the attributes RFC 7606 just judged
// untrustworthy.
func withdrawMPAttrs(pathAttrs []byte) ([]byte, bool) {
	if len(pathAttrs) == 0 {
		return nil, false
	}

	var out []byte
	converted := false

	it := attribute.NewAttrIterator(pathAttrs)
	for {
		code, _, value, ok := it.Next()
		if !ok {
			break
		}
		if code == attribute.AttrMPUnreachNLRI {
			out = appendMPUnreach(out, value)
			continue
		}
		if code == attribute.AttrMPReachNLRI {
			unreach, converted2 := mpReachToUnreach(value)
			if !converted2 {
				// A malformed MP_REACH cannot legitimately reach here: Section 5.3 makes
				// it "incorrect" and Section 3(j) escalates that to session reset. Skip it
				// rather than emit a withdrawal for prefixes we could not identify.
				continue
			}
			out = appendMPUnreach(out, unreach)
			converted = true
			continue
		}
		// Any other attribute is deliberately not carried into the withdrawal.
	}
	return out, converted
}

// mpReachToUnreach converts an MP_REACH_NLRI value to an MP_UNREACH_NLRI value: keep
// AFI/SAFI and the NLRI, drop the next hop and reserved byte (RFC 4760 Sections 3 and 4).
func mpReachToUnreach(value []byte) ([]byte, bool) {
	if len(value) < mpReachFixedPrefix {
		return nil, false
	}
	nextHopLen := int(value[mpReachNextHopLenOff])
	nlriStart := mpReachFixedPrefix + nextHopLen + 1 // + reserved octet
	if nlriStart > len(value) {
		return nil, false
	}

	out := make([]byte, 0, mpReachAFISAFILen+len(value)-nlriStart)
	out = append(out, value[:mpReachAFISAFILen]...) // AFI + SAFI
	out = append(out, value[nlriStart:]...)         // NLRI
	return out, true
}

// appendMPUnreach appends a well-formed MP_UNREACH_NLRI attribute carrying value.
func appendMPUnreach(dst, value []byte) []byte {
	if len(value) > extendedLengthThreshold {
		dst = append(dst, mpUnreachExtendedFlags, byte(attribute.AttrMPUnreachNLRI))
		var l [2]byte
		//nolint:gosec // bounded by BGP message size (max 65535)
		binary.BigEndian.PutUint16(l[:], uint16(len(value)))
		dst = append(dst, l[0], l[1])
		return append(dst, value...)
	}
	dst = append(dst, mpUnreachFlags, byte(attribute.AttrMPUnreachNLRI), byte(len(value)))
	return append(dst, value...)
}
