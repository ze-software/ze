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
// withdrawal, and returns (newBody, changed). It is the single-body form of
// SynthesizeWithdrawFamilies and accepts every MP family (nil predicate); newBody is the
// primary synthesized body.
//
// A treat-as-withdraw UPDATE carrying two DIFFERENT MP families cannot be represented as a
// single body: the RIB reads only the first MP_UNREACH of an UPDATE and RFC 7606 Section
// 3.g forbids more than one, so the second family must ride its own UPDATE. Callers on the
// receive path that must withdraw every family therefore use SynthesizeWithdrawFamilies;
// this helper returns only the primary body.
func SynthesizeWithdraw(body []byte) ([]byte, bool) {
	bodies := SynthesizeWithdrawFamilies(body, nil)
	if len(bodies) == 0 {
		return body, false
	}
	return bodies[0], true
}

// SynthesizeWithdrawFamilies rewrites a treat-as-withdraw UPDATE body into one or more
// withdraw-only UPDATE bodies to be dispatched through the normal received-UPDATE path.
//
// RFC 7606 Section 2: an UPDATE handled with treat-as-withdraw "MUST be handled as though
// all of the routes contained in [it] ... had been withdrawn from service", "thus causing
// them to be removed from the Adj-RIB-In according to the procedures of [RFC4271]". Not
// dispatching the UPDATE is NOT that: a prefix already in the Adj-RIB-In, re-announced with
// a malformed attribute, would keep its previous entry and go stale. So the announced routes
// are converted into withdrawals and dispatched, which is what actually removes them.
//
// The transformation:
//   - trailing IPv4 NLRI moves into the Withdrawn Routes field
//   - MP_REACH_NLRI becomes MP_UNREACH_NLRI for the same AFI/SAFI (next hop dropped)
//   - an existing MP_UNREACH_NLRI is kept as-is
//   - every other path attribute is dropped: a withdrawal carries none, and those
//     attributes are the very thing that failed validation
//
// Two receive-path refinements over a naive rewrite:
//   - accept filters MP families by negotiation (nil accepts every family). A malformed
//     UPDATE whose MP family was never negotiated must not gain a teardown path it did not
//     have before treat-as-withdraw synthesis existed, so such a family is skipped here
//     rather than surfacing to the strict-mode family check downstream; nothing was
//     installed for it, so there is nothing to withdraw (D-5).
//   - each returned body carries at most one MP_UNREACH_NLRI. The RIB reads only the first
//     MP_UNREACH of an UPDATE (a single first-match lookup), and RFC 7606 Section 3.g makes
//     more than one a session-reset shape, so two different MP families are split across two
//     UPDATEs: the primary body carries the legacy IPv4 field plus the first family, and
//     each further family rides its own body (D-3/D-8).
//
// Returns an empty slice when there is nothing to withdraw: an End-of-RIB, a body whose
// section lengths are structurally unusable (Section 3(b)/3(j) session-reset those upstream,
// so refusing to guess is the fail-closed choice rather than manufacturing a withdrawal from
// untrustworthy offsets), or an UPDATE whose only content was a non-negotiated MP family.
// The caller drops such an UPDATE without dispatching it.
func SynthesizeWithdrawFamilies(body []byte, accept func(afi uint16, safi uint8) bool) [][]byte {
	if len(body) < 4 {
		return nil
	}

	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	withdrawnEnd := 2 + withdrawnLen
	if withdrawnEnd+2 > len(body) {
		return nil
	}

	attrLen := int(binary.BigEndian.Uint16(body[withdrawnEnd : withdrawnEnd+2]))
	attrStart := withdrawnEnd + 2
	if attrStart+attrLen > len(body) {
		return nil
	}

	withdrawn := body[2:withdrawnEnd]
	pathAttrs := body[attrStart : attrStart+attrLen]
	nlri := body[attrStart+attrLen:]

	mpAttrs := mpUnreachAttrList(pathAttrs, accept)

	// Nothing to withdraw: no IPv4 Withdrawn/NLRI and no admissible MP family. An End-of-RIB
	// (MP_UNREACH only, no NLRI) lands here too -- but a treat-as-withdraw EOR is a
	// contradiction (an EOR has no attributes to be malformed), so an empty result is the
	// correct "drop" signal for the caller.
	if len(withdrawn)+len(nlri) == 0 && len(mpAttrs) == 0 {
		return nil
	}

	// Primary body: the legacy IPv4 Withdrawn Routes (original withdrawals ++ announced NLRI)
	// plus the first MP family, if any.
	var firstMP []byte
	if len(mpAttrs) > 0 {
		firstMP = mpAttrs[0]
	}
	bodies := [][]byte{buildWithdrawBody(withdrawn, nlri, firstMP)}
	// Every further MP family rides its own withdraw-only UPDATE (one MP_UNREACH each).
	for i := 1; i < len(mpAttrs); i++ {
		bodies = append(bodies, buildWithdrawBody(nil, nil, mpAttrs[i]))
	}
	return bodies
}

// buildWithdrawBody assembles a withdraw-only UPDATE body from the IPv4 Withdrawn Routes
// (original withdrawals followed by the now-withdrawn announced NLRI, which share the RFC
// 4271 Section 4.3 encoding) and at most one MP_UNREACH_NLRI attribute (nil for none).
func buildWithdrawBody(withdrawn, nlri, mpAttr []byte) []byte {
	newWithdrawnLen := len(withdrawn) + len(nlri)
	out := make([]byte, 2+newWithdrawnLen+2+len(mpAttr))

	//nolint:gosec // bounded by BGP message size (max 65535)
	binary.BigEndian.PutUint16(out[0:2], uint16(newWithdrawnLen))
	n := 2
	n += copy(out[n:], withdrawn)
	// The announced prefixes become withdrawals. IPv4 unicast NLRI and Withdrawn Routes
	// share an encoding (RFC 4271 Section 4.3), so this is a straight append.
	n += copy(out[n:], nlri)
	//nolint:gosec // bounded by BGP message size (max 65535)
	binary.BigEndian.PutUint16(out[n:n+2], uint16(len(mpAttr)))
	n += 2
	copy(out[n:], mpAttr)

	return out
}

// mpUnreachAttrList returns each MP_UNREACH_NLRI attribute (wire-encoded, one attribute per
// element) that the withdrawal must carry: an existing MP_UNREACH_NLRI is kept, an
// MP_REACH_NLRI is converted (next hop dropped, RFC 4760 Sections 3 and 4), every other
// attribute is dropped. A family rejected by accept (not negotiated) is skipped -- see
// SynthesizeWithdrawFamilies.
//
// Dropping the other attributes is the point, not an oversight: RFC 4271 Section 4.3 gives
// a withdrawal no path attributes, and these are the attributes RFC 7606 just judged
// untrustworthy.
func mpUnreachAttrList(pathAttrs []byte, accept func(afi uint16, safi uint8) bool) [][]byte {
	if len(pathAttrs) == 0 {
		return nil
	}

	var out [][]byte

	it := attribute.NewAttrIterator(pathAttrs)
	for {
		code, _, value, ok := it.Next()
		if !ok {
			break
		}
		if code == attribute.AttrMPUnreachNLRI {
			if mpFamilyAccepted(value, accept) {
				out = append(out, appendMPUnreach(nil, value))
			}
			continue
		}
		if code == attribute.AttrMPReachNLRI {
			unreach, converted := mpReachToUnreach(value)
			if !converted {
				// A malformed MP_REACH cannot legitimately reach here: Section 5.3 makes
				// it "incorrect" and Section 3(j) escalates that to session reset. Skip it
				// rather than emit a withdrawal for prefixes we could not identify.
				continue
			}
			if mpFamilyAccepted(unreach, accept) {
				out = append(out, appendMPUnreach(nil, unreach))
			}
			continue
		}
		// Any other attribute is deliberately not carried into the withdrawal.
	}
	return out
}

// mpFamilyAccepted reports whether accept admits the AFI/SAFI at the head of an
// MP_UNREACH_NLRI value (AFI(2) SAFI(1); RFC 4760 Section 4). A nil predicate admits every
// family; a value too short to carry an AFI/SAFI is rejected.
func mpFamilyAccepted(unreachValue []byte, accept func(afi uint16, safi uint8) bool) bool {
	if accept == nil {
		return true
	}
	if len(unreachValue) < mpReachAFISAFILen {
		return false
	}
	afi := binary.BigEndian.Uint16(unreachValue[0:2])
	safi := unreachValue[2]
	return accept(afi, safi)
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
