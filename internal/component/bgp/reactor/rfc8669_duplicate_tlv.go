// Design: docs/architecture/wire/attributes.md -- BGP_PREFIX_SID (Code 40), the repeated-TLV discard
// Overview: session_validation.go -- publishBase, the ingest step that calls this
// RFC: rfc/short/rfc8669.md -- BGP Prefix-SID, Section 6 error handling
// Related: forward_prefix_sid.go -- prefixSIDTLVOctets, the TLV framing this walk shares
// Related: rfc8092_large_community.go -- removeRedundantLargeCommunities, the same ingest shape

package reactor

import (
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// discardRepeatedPrefixSIDTLVs returns wu unchanged when its Prefix-SID
// attribute holds no single-occurrence TLV twice, and otherwise a new
// WireUpdate whose attribute keeps the first TLV of each such type and every
// other TLV, in the received order.
//
// RFC 8669 Section 6: "if a recognized TLV appears more than once in a BGP
// Prefix-SID attribute while the specification only allows for a single
// occurrence, then all the occurrences of the TLV other than the first one
// SHALL be discarded and the Prefix-SID attribute will continue to be
// processed."
// RFC 9252 Section 7: "If multiple instances of the SRv6 L3 Service TLV are
// encountered, all but the first instance MUST be ignored."
//
// attribute.PrefixSIDTLVSingleOccurrence names the types, for the family the
// attribute is attached to: the SRv6 L3 and L2 Service TLVs on every family,
// the Label-Index TLV on IPv4 and IPv6 labeled unicast only (RFC 8669 Section
// 3.1). A TLV of any other type, or a Label-Index TLV on another family, is
// kept however often it repeats, because RFC 8669 Section 6 says "unknown TLVs
// MUST be ignored and propagated unmodified".
//
// It is an ingest step of publishBase: the bytes it returns are the bytes the
// RIB retains, a route server relays zero-copy, every rebuild copies from and
// the JSON encoder renders, so the discard reaches every consumer. The common
// case, no repeat, only reads the attribute and allocates nothing; the rebuild
// is paid by the UPDATE that carries a repeat.
//
// It runs after the RFC 7606 walk, which validates every TLV, the repeats
// included. A malformed second SRv6 Service TLV is therefore still
// treat-as-withdraw (RFC 9252 Section 7) and never reaches here, and a
// framing error discards the attribute before it does. Only the first
// Prefix-SID attribute is examined: the RFC 7606 Section 3.g strip has already
// removed any later copy of the attribute itself.
//
// Prefix-SID attribute, as the peer sent it (RFC 4271 Section 4.3, RFC 8669
// Section 3):
//
//	 0                   1                   2                   3
//	+---------------+---------------+---------------+---------------+
//	|  Attr. Flags  |Attr. Type (40)| Length (1 octet, or 2 when    |
//	+---------------+---------------+  Extended Length is set)      |
//	| TLV[0]: Type (1) | Length (2) | Value (Length octets)         |
//	| TLV[1]: Type (1) | Length (2) | Value ...                     |
//	+---------------------------------------------------------------+
func discardRepeatedPrefixSIDTLVs(wu *wireu.WireUpdate, peer netip.Addr) *wireu.WireUpdate {
	payload := wu.Payload()
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		// publishBase reports an unparseable payload through wu.Attrs(); nothing to
		// discard from a section that cannot be located.
		return wu
	}
	attrs := sections.Attrs(payload)
	hdrStart, flags, value, found := attribute.AttrFind(attrs, attribute.AttrPrefixSID)
	if !found {
		return wu
	}
	afi, safi, ok := prefixSIDRouteFamily(attrs)
	if !ok {
		// The RFC 7606 walk refuses an MP_REACH_NLRI too short to name its family
		// before publishBase runs; without a family no TLV can be judged single.
		return wu
	}
	kept, discarded := prefixSIDAttrsFirstOnly(attrs, hdrStart, flags, value, afi, safi)
	if !discarded {
		return wu
	}

	rebuilt := wireu.NewWireUpdate(message.RebuildUpdateBody(payload, kept), wu.SourceCtxID())
	rebuilt.SetSourceID(wu.SourceID())
	sessionLogger().Debug("RFC 8669 Section 6: discarded repeated Prefix-SID TLVs",
		"peer", peer, "octets-received", len(value))
	return rebuilt
}

// prefixSIDRouteFamily returns the family of the routes the UPDATE whose
// attribute section is attrs announces: the AFI and SAFI of its MP_REACH_NLRI
// (RFC 4760 Section 3), or IPv4 unicast for an UPDATE without one, whose
// routes travel in the RFC 4271 NLRI field. It returns false for an
// MP_REACH_NLRI shorter than its AFI and SAFI.
//
// An UPDATE carrying both is read by its MP_REACH_NLRI family. The UPDATE has
// one Prefix-SID for both sets of routes, so one reading must win, and the
// labeled-unicast routes are the ones RFC 8669 Section 3.1 requires the
// Label-Index TLV for.
func prefixSIDRouteFamily(attrs []byte) (attribute.AFI, attribute.SAFI, bool) {
	_, _, reach, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if !found {
		return attribute.AFIIPv4, attribute.SAFIUnicast, true
	}
	if len(reach) < 3 {
		return 0, 0, false
	}
	return attribute.AFI(binary.BigEndian.Uint16(reach)), attribute.SAFI(reach[2]), true
}

// prefixSIDTLVSeen records which Prefix-SID TLV types a walk has met, one bit
// per type code. It lives on the stack of the walk that owns it.
type prefixSIDTLVSeen [4]uint64

// keep reports whether the TLV of type tlvType, met next in the walk of an
// attribute attached to afi/safi routes, stays in the attribute: every TLV of
// a type outside the single-occurrence set for that family, and the first TLV
// of a type inside it.
func (s *prefixSIDTLVSeen) keep(afi attribute.AFI, safi attribute.SAFI, tlvType byte) bool {
	if !attribute.PrefixSIDTLVSingleOccurrence(afi, safi, tlvType) {
		return true
	}
	word, mask := tlvType>>6, uint64(1)<<(tlvType&63)
	if s[word]&mask != 0 {
		return false
	}
	s[word] |= mask
	return true
}

// prefixSIDAttrsFirstOnly returns a copy of the attribute section attrs in
// which the Prefix-SID attribute at hdrStart, whose value is value, has lost
// every repeat of a TLV that is single-occurrence on family afi/safi, and true. When nothing repeats it
// returns nil and false and allocates nothing: the copy starts at the first
// repeat. The attribute keeps the peer's flags and header width; only its
// length changes.
//
// A TLV whose framing does not fit the value also returns nil and false, which
// leaves the attribute as the peer sent it. The RFC 7606 walk refuses that
// framing before publishBase runs, so the branch is a guard against a caller
// that skipped it, not a verdict on the attribute.
//
// The loop is bounded by the value length, itself bounded by the message
// length: each step advances by at least the 3-octet TLV header.
func prefixSIDAttrsFirstOnly(attrs []byte, hdrStart int, flags attribute.AttributeFlags, value []byte, afi attribute.AFI, safi attribute.SAFI) ([]byte, bool) {
	headerOctets := 3
	if flags.IsExtLength() {
		headerOctets = 4
	}
	valueStart := hdrStart + headerOctets
	valueEnd := valueStart + len(value)

	var seen prefixSIDTLVSeen
	var out []byte
	for off := 0; off < len(value); {
		tlvOctets, ok := prefixSIDTLVOctets(value[off:])
		if !ok {
			return nil, false
		}
		kept := seen.keep(afi, safi, value[off])
		if kept && out != nil {
			out = append(out, value[off:off+tlvOctets]...)
		}
		if !kept && out == nil {
			out = make([]byte, 0, len(attrs)-tlvOctets)
			out = append(out, attrs[:valueStart+off]...)
		}
		off += tlvOctets
	}
	if out == nil {
		return nil, false
	}

	keptOctets := len(out) - valueStart
	out = append(out, attrs[valueEnd:]...)
	if flags.IsExtLength() {
		//nolint:gosec // keptOctets is below the received length, which fit in 16 bits.
		binary.BigEndian.PutUint16(out[hdrStart+2:], uint16(keptOctets))
	} else {
		//nolint:gosec // keptOctets is below the received length, which fit in 8 bits.
		out[hdrStart+2] = byte(keptOctets)
	}
	return out, true
}
