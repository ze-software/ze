// Design: docs/architecture/wire/attributes.md -- carrier-aware tunnel validation.
// Related: session_validation.go -- receive enforcement before publication.
// RFC 9012 Sections 3.1, 6 and 13 -- see rfc/short/rfc9012.md.
// RFC 9830 Sections 2.2 and 2.3 -- see rfc/short/rfc9830.md.
//
// Attribute value: [Tunnel Type:2][Length:2][sub-TLVs:Length] repeated.
// Sub-TLV: [Type:1][Length:1 for Type<128, otherwise 2][Value:Length].
// Endpoint Value: [Reserved:4][AFI:2][Address:0,4,16] (RFC 9012 Section 3.1).

package reactor

import (
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/ipregistry"
)

// applyTunnelEncap enforces tunnel framing and carrier constraints after attribute
// deduplication. The caller MUST supply validated UPDATE section boundaries and
// MUST use both returned values, so later attribute edits affect the rebuilt body.
// Valid attributes stay zero-copy; only endpoint-invalid TLV removal allocates.
// RFC 9012 Section 13: "If a Tunnel Encapsulation attribute does not have any
// valid TLVs, or it does not have the transitive bit set, the "Treat-as-withdraw"
// procedure of [RFC7606] is applied.".
func applyTunnelEncap(wu *wireu.WireUpdate, attrs []byte, hasNLRI bool, result *message.RFC7606ValidationResult) (*wireu.WireUpdate, []byte) {
	if result.Action >= message.RFC7606ActionTreatAsWithdraw {
		return wu, attrs
	}
	if !result.TunnelEncapPresent {
		return wu, attrs
	}
	start, flags, value, found := attribute.AttrFind(attrs, attribute.AttrTunnelEncap)
	if !found {
		return wu, attrs
	}
	policy := false
	requireEndpoint := hasNLRI
	// Withdrawn routes do not carry these path attributes.
	if loc := &result.MPReachNLRI; loc.Present {
		if loc.AFI == 1 || loc.AFI == 2 {
			if loc.SAFI == 73 {
				policy = true
			}
		}
		// RFC 9012 Section 6 lists these carriers; Section 13 requires exactly
		// one endpoint per TLV on them. Other SAFIs do not inherit that rule.
		if loc.AFI == 1 || loc.AFI == 2 {
			if loc.SAFI == 1 || loc.SAFI == 4 || loc.SAFI == 128 {
				requireEndpoint = true
			}
		}
		if loc.AFI == 25 && loc.SAFI == 70 {
			requireEndpoint = true
		}
	}

	kept := 0
	tunnels := 0
	// Bounded by the validated attribute length; each accepted TLV consumes >=4.
	for off := 0; off < len(value); {
		// RFC 9012 Section 13; RFC 9830 Section 2.3.
		size, keep := tunnelTLVLayout(value[off:], requireEndpoint, policy)
		if size == 0 {
			// RFC 9012 Section 13.
			tunnelReceiveError(result, "RFC 9012 Section 13: malformed tunnel TLV framing")
			return wu, attrs
		}
		tunnels++
		// RFC 9830 Section 2.2: "This document specifies the use of the Tunnel
		// Encapsulation Attribute with the SR Policy Tunnel Type and the use of
		// any other Tunnel Type with the SR Policy SAFI MUST be considered
		// malformed and handled by the "treat-as-withdraw" strategy [RFC7606]."
		if policy {
			if binary.BigEndian.Uint16(value[off:]) != 15 {
				// RFC 9830 Section 2.2.
				tunnelReceiveError(result, "RFC 9830 Section 2.2: non-SR-Policy tunnel type on SAFI 73")
				return wu, attrs
			}
			// RFC 9830 Section 2.2: "A Tunnel Encapsulation Attribute MUST NOT
			// contain more than one TLV of type "SR Policy"; such updates MUST
			// be considered malformed and handled by the "treat-as-withdraw"
			// strategy [RFC7606]."
			if tunnels > 1 {
				// RFC 9830 Section 2.2.
				tunnelReceiveError(result, "RFC 9830 Section 2.2: multiple SR Policy tunnel TLVs")
				return wu, attrs
			}
		}
		if keep {
			kept += size
		}
		off += size
	}
	if kept == 0 {
		// RFC 9012 Section 13.
		tunnelReceiveError(result, "RFC 9012 Section 13: no valid tunnel TLVs")
		return wu, attrs
	}
	if kept == len(value) {
		return wu, attrs
	}

	// RFC 9012 Section 13: "There is one exception to this rule: if a TLV
	// contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in
	// Section 3.1), the entire TLV MUST be ignored and MUST be removed from
	// the Tunnel Encapsulation attribute before the route carrying that
	// attribute is distributed."
	// One deliberate malformed-input copy: preserve the attribute header form,
	// unrelated attributes, route bytes and original diagnostic source buffer.
	body := wu.Payload()
	attrsOffset := 4 + int(binary.BigEndian.Uint16(body[:2]))
	headerLen := 3
	if flags&attribute.FlagExtLength != 0 {
		headerLen = 4
	}
	removed := len(value) - kept
	rebuilt := make([]byte, len(body)-removed)
	pos := attrsOffset + start + headerLen
	copy(rebuilt, body[:pos])
	binary.BigEndian.PutUint16(rebuilt[attrsOffset-2:], uint16(len(attrs)-removed))
	if headerLen == 4 {
		binary.BigEndian.PutUint16(rebuilt[attrsOffset+start+2:], uint16(kept))
	} else {
		rebuilt[attrsOffset+start+2] = byte(kept)
	}
	for off := 0; off < len(value); {
		// RFC 9012 Section 13. The first pass proved every TLV is framed.
		size, keep := tunnelTLVLayout(value[off:], requireEndpoint, policy)
		if keep {
			pos += copy(rebuilt[pos:], value[off:off+size])
		}
		off += size
	}
	copy(rebuilt[pos:], body[attrsOffset+start+headerLen+len(value):])
	out := wireu.NewWireUpdate(rebuilt, wu.SourceCtxID())
	out.SetSourceID(wu.SourceID())
	return out, rebuilt[attrsOffset : attrsOffset+len(attrs)-removed]
}

// tunnelTLVLayout returns the first TLV's size and whether its endpoints permit
// retaining it. Size zero means framing is unparseable, never an empty TLV.
// RFC 9012 Section 13: "The final octet of a TLV MUST also be the final octet of
// its final sub-TLV.".
func tunnelTLVLayout(data []byte, requireEndpoint, policy bool) (int, bool) {
	if len(data) < 4 {
		return 0, false
	}
	size := 4 + int(binary.BigEndian.Uint16(data[2:4]))
	if size > len(data) {
		return 0, false
	}
	endpoints := 0
	keep := true
	// Each sub-TLV advances by at least two octets, bounded by this TLV's end.
	for off := 4; off < size; {
		if size-off < 2 {
			return 0, false
		}
		headerLen := 2
		length := int(data[off+1])
		if data[off] >= 128 {
			if size-off < 3 {
				return 0, false
			}
			headerLen = 3
			length = int(binary.BigEndian.Uint16(data[off+1 : off+3]))
		}
		end := off + headerLen + length
		if end > size {
			return 0, false
		}
		// RFC 9830 Section 2.3: "If these sub-TLVs are present, a BGP speaker
		// MUST ignore them and MAY remove them from the Tunnel Encapsulation
		// Attribute during propagation."
		if data[off] == 6 && !policy {
			endpoints++
			// RFC 9012 Sections 3.1 and 13.
			if !tunnelEndpointValid(data[off+headerLen : end]) {
				keep = false
			}
		}
		off = end
	}
	// RFC 9012 Section 13: "Within a Tunnel Encapsulation attribute that is
	// carried by a BGP UPDATE whose AFI/SAFI is one of those explicitly listed
	// in the first paragraph of Section 6, a TLV that does not contain exactly
	// one Tunnel Egress Endpoint sub-TLV MUST be treated as if it contained a
	// malformed Tunnel Egress Endpoint sub-TLV."
	if requireEndpoint && !policy {
		if endpoints != 1 {
			keep = false
		}
	}
	return size, keep
}

// tunnelEndpointValid checks structural length and the IANA special-purpose
// address fields, not reachability or optional origin-AS ownership. Unknown
// AFIs remain opaque; AFI0 does not classify the route's NEXT_HOP.
// RFC 9012 Section 3.1 defines a malformed endpoint when: "The length of the
// sub-TLV's Value field is other than 6 added to the defined length for the
// address family given in its Address Family subfield.".
func tunnelEndpointValid(value []byte) bool {
	if len(value) < 6 {
		return false
	}
	var address netip.Addr
	switch binary.BigEndian.Uint16(value[4:6]) {
	case 0:
		return len(value) == 6
	case 1:
		if len(value) != 10 {
			return false
		}
		address = netip.AddrFrom4([4]byte(value[6:10]))
	case 2:
		if len(value) != 22 {
			return false
		}
		address = netip.AddrFrom16([16]byte(value[6:22]))
	default:
		// The wire AFI set is open; unrecognized endpoints are not malformed.
		return true
	}
	// RFC 9012 Section 3.1: "The IP address in the sub-TLV's Address subfield
	// lies within a block listed in the relevant Special-Purpose IP Address
	// registry [RFC6890] with either a \"destination\" attribute value or a
	// \"forwardable\" attribute value of \"false\"."
	record := ipregistry.Lookup(address)
	if record.Match != ipregistry.MatchListed {
		// Only a valid unlisted address is permitted without record fields.
		return record.Match == ipregistry.MatchUnlisted
	}
	if record.Destination == ipregistry.ValueFalse {
		return false
	}
	return record.Forwardable != ipregistry.ValueFalse
}

// tunnelReceiveError records a tunnel error without disturbing NLRI locations.
// RFC 7606 Section 2: "In this approach, the UPDATE message containing the path
// attribute in question MUST be treated as though all contained routes had been
// withdrawn just as if they had been listed in the WITHDRAWN ROUTES field (or in
// the MP_UNREACH_NLRI attribute if appropriate) of the UPDATE message, thus causing
// them to be removed from the Adj-RIB-In according to the procedures of [RFC4271].".
func tunnelReceiveError(result *message.RFC7606ValidationResult, description string) {
	result.Action = message.RFC7606ActionTreatAsWithdraw
	result.AttrCode = uint8(attribute.AttrTunnelEncap)
	result.Description = description
	// RFC 7606 Section 5.2: "For this reason, if any path attribute errors are
	// encountered in such an UPDATE message and if any encountered error
	// specifies an error-handling approach other than "attribute discard",
	// then the "session reset" approach MUST be used."
	// Attribute presence alone does not establish reachable NLRI: an empty
	// MP_REACH still needs escalation. Preserve the original validator's fact.
	if !result.HasReachableNLRI {
		result.Action = message.RFC7606ActionSessionReset
	}
}
