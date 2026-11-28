// Design: docs/architecture/wire/attributes.md -- attribute flags, RFC 8097 egress strip, and LARGE_COMMUNITY on send
// Overview: session_write.go -- writeUpdateBody, the writer every UPDATE funnels through
// Related: session_prefix_sid.go -- clearLabelIndexTLVs, the Prefix-SID step
// Related: session_origin_validation.go -- originValidationStateSendAllowedHeld, the RFC 8097 EBGP boundary check
// Related: rfc8092_large_community.go -- largeCommunityRedundant, the repeat check
// RFC: rfc/short/rfc4271.md, rfc/short/rfc8092.md, rfc/short/rfc8097.md, rfc/short/rfc8669.md, rfc/short/rfc8277.md

package reactor

import (
	"bytes"
	"encoding/binary"
	"hash/maphash"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

const (
	// attrFlagsUnusedMask is the lower-order four bits of the Attribute Flags octet.
	attrFlagsUnusedMask = 0x0F
	// extCommunityOctets is the RFC 4360 Section 2 wire length of one extended community.
	extCommunityOctets = 8
)

// normalizeTransmittedUpdate applies normalizeTransmittedUpdateWith without
// stripping RFC 8097 Origin Validation State extended communities (preserving
// the non-EBGP default for direct callers and tests).
func normalizeTransmittedUpdate(body []byte, encoding *bgpctx.EncodingContext) {
	normalizeTransmittedUpdateWith(body, encoding, false)
}

// normalizeTransmittedUpdateWith applies, in one walk over the path attributes,
// the rules that bind what Ze transmits whatever produced the attribute: a
// local encoder, a policy edit, a relay of the received bytes, or a plugin's raw
// body. It returns the body's new length; LARGE_COMMUNITY deduplication and
// RFC 8097 EBGP stripping of 0x43/0x00 can shorten it. encoding is the session's
// send context, read for the ADD-PATH framing of a labeled MP_UNREACH_NLRI
// (setWithdrawalCompatibility). When stripOriginValidationState is true, every
// RFC 8097 Origin Validation State extended community (0x43/0x00) is removed in
// place (omitting EXTENDED_COMMUNITIES altogether when it was the sole entry);
// when false, its Reserved octets [2..6] are zeroed in place.
//
// body MUST be the session's private write buffer: received bytes are shared by
// every destination and the forward cache, and are never edited.
//
// The walk is bounded by the total path attribute length. On an attribute whose
// header or length overruns the section it stops, leaving that attribute and the
// rest as they are: malformed framing is the validator's, never repaired here.
//
// UPDATE body (RFC 4271 Section 4.3) and one path attribute:
//
//	+-----------------------------------------------------+
//	|   Withdrawn Routes Length (2 octets)                |
//	|   Withdrawn Routes (variable)                       |
//	|   Total Path Attribute Length (2 octets)            | <- rewritten on shrink
//	|   Path Attributes (variable)                        |
//	|   Network Layer Reachability Information (variable) | <- moved up on shrink
//	+-----------------------------------------------------+
//
//	 0                   1                   2
//	+---------------+---------------+---------------+---------------+
//	|  Attr. Flags  |Attr. Type Code| Length (1, or 2 when Extended |
//	|O|T|P|E|0 0 0 0|               |  Length is set)               |
//	+---------------+---------------+---------------+---------------+
func normalizeTransmittedUpdateWith(body []byte, encoding *bgpctx.EncodingContext, stripOriginValidationState bool) int {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return len(body)
	}
	attrStart := 2 + sections.WithdrawnLen() + 2
	attrsLen := sections.AttrsLen()
	end := len(body)
	for pos := 0; pos+3 <= attrsLen; {
		attrs := body[attrStart : attrStart+attrsLen]
		flags := attribute.AttributeFlags(attrs[pos])
		header := 3
		length := int(attrs[pos+2])
		if flags.IsExtLength() {
			if pos+4 > attrsLen {
				break
			}
			header = 4
			length = int(binary.BigEndian.Uint16(attrs[pos+2:]))
		}
		valueStart := pos + header
		if valueStart+length > attrsLen {
			break
		}
		// RFC 4271 Section 4.3: "The lower-order four bits of the Attribute
		// Flags octet are unused. They MUST be zero when sent and MUST be
		// ignored when received."
		attrs[pos] &^= attrFlagsUnusedMask
		value := attrs[valueStart : valueStart+length]
		code := attribute.AttributeCode(attrs[pos+1])
		if code == attribute.AttrPrefixSID {
			// RFC 8669 Section 3.1
			clearLabelIndexTLVs(attrs, value)
		}
		if code == attribute.AttrMPUnreachNLRI {
			// RFC 8277 Section 2.4
			setWithdrawalCompatibility(value, encoding)
		}
		if code == attribute.AttrExtCommunity {
			if length%extCommunityOctets == 0 {
				if stripOriginValidationState {
					// RFC 8097 Section 2: "Similarly, by default, an implementation
					// SHOULD NOT send the community to EBGP peers."
					kept := compactWithoutOriginValidationState(value)
					if kept < length {
						if kept == 0 {
							gap := header + length
							attrEnd := attrStart + valueStart + length
							copy(body[attrStart+pos:], body[attrEnd:end])
							end -= gap
							attrsLen -= gap
							//nolint:gosec // attrsLen only shrank from a 16-bit length.
							binary.BigEndian.PutUint16(body[attrStart-2:], uint16(attrsLen))
							continue
						}
						gap := length - kept
						valueEnd := attrStart + valueStart + length
						copy(body[valueEnd-gap:], body[valueEnd:end])
						end -= gap
						attrsLen -= gap
						length = kept
						if header == 4 {
							//nolint:gosec // kept is at most the received length, which fit in 16 bits.
							binary.BigEndian.PutUint16(body[attrStart+pos+2:], uint16(kept))
						} else {
							//nolint:gosec // kept is at most the received length, which fit in 8 bits.
							body[attrStart+pos+2] = byte(kept)
						}
						//nolint:gosec // attrsLen only shrank from a 16-bit length.
						binary.BigEndian.PutUint16(body[attrStart-2:], uint16(attrsLen))
					}
				} else {
					// RFC 8097 Section 2: "The Reserved field MUST be set to 0
					// and ignored upon the receipt of this community."
					attribute.ZeroOriginValidationStateReservedBytes(value)
				}
			}
		}
		if code == attribute.AttrLargeCommunity && length%largeCommunityOctets == 0 && largeCommunityRedundant(value) {
			// RFC 8092 Section 3: "Duplicate BGP Large Community values MUST NOT
			// be transmitted."
			kept := compactLargeCommunities(value)
			gap := length - kept
			valueEnd := attrStart + valueStart + length
			copy(body[valueEnd-gap:], body[valueEnd:end])
			end -= gap
			attrsLen -= gap
			length = kept
			if header == 4 {
				//nolint:gosec // kept is at most the received length, which fit in 16 bits.
				binary.BigEndian.PutUint16(body[attrStart+pos+2:], uint16(kept))
			} else {
				//nolint:gosec // kept is at most the received length, which fit in 8 bits.
				body[attrStart+pos+2] = byte(kept)
			}
			//nolint:gosec // attrsLen only shrank from a 16-bit length.
			binary.BigEndian.PutUint16(body[attrStart-2:], uint16(attrsLen))
		}
		pos = valueStart + length
	}
	return end
}

// compactWithoutOriginValidationState removes every RFC 8097 Origin Validation
// State extended community (type 0x43, sub-type 0x00) from value in place,
// preserving the order of all other 8-octet extended communities, and returns
// the number of octets kept. It allocates nothing.
func compactWithoutOriginValidationState(value []byte) int {
	kept := 0
	for off := 0; off+extCommunityOctets <= len(value); off += extCommunityOctets {
		if value[off] == attribute.ExtCommTypeNonTransOpaque {
			if value[off+1] == attribute.ExtCommSubTypeOriginValidation {
				continue
			}
		}
		if kept != off {
			copy(value[kept:kept+extCommunityOctets], value[off:off+extCommunityOctets])
		}
		kept += extCommunityOctets
	}
	return kept
}

// compactLargeCommunities keeps the first occurrence of each 12-octet value in
// value, in order, moving the kept values to the front in place, and returns the
// octets kept. It allocates nothing: up to largeCommunityPairScanMax values it
// compares against the kept prefix, above it a stack filter whose set bits are
// confirmed against the kept prefix, so the answer is exact either way.
//
// RFC 8092 Section 3: "Duplicate BGP Large Community values MUST NOT be
// transmitted.".
func compactLargeCommunities(value []byte) int {
	count := len(value) / largeCommunityOctets
	filtered := count > largeCommunityPairScanMax
	var seen [1024]uint64
	kept := 0
	for i := range count {
		current := value[i*largeCommunityOctets : (i+1)*largeCommunityOctets]
		check := true
		if filtered {
			bit := uint16(maphash.Bytes(largeCommunitySeed, current))
			word, mask := bit>>6, uint64(1)<<(bit&63)
			check = seen[word]&mask != 0
			seen[word] |= mask
		}
		if check && largeCommunityHeld(value[:kept], current) {
			continue
		}
		copy(value[kept:], current)
		kept += largeCommunityOctets
	}
	return kept
}

// largeCommunityHeld reports whether the whole 12-octet values in held contain
// current.
func largeCommunityHeld(held, current []byte) bool {
	for j := 0; j < len(held); j += largeCommunityOctets {
		if bytes.Equal(current, held[j:j+largeCommunityOctets]) {
			return true
		}
	}
	return false
}

// setWithdrawalCompatibility writes 0x800000 into the Compatibility field of
// every NLRI an MP_UNREACH_NLRI value withdraws, when its family carries the
// field (nlrisplit.CompatibilitySplitter: labeled unicast and VPN). It binds
// every UPDATE Ze transmits, whatever produced it: an API or config withdrawal
// naming a label, a relay of a received withdrawal, or a plugin's raw body.
// normalizeTransmittedUpdate calls it from its one attribute walk. The length
// never changes. encoding is the session's send context, which says whether
// each NLRI leads with an ADD-PATH identifier.
//
// value MUST lie in the session's private write buffer, as for
// normalizeTransmittedUpdate. A section whose framing does not parse is left
// as it is: malformed framing is the validator's, never repaired here. It
// allocates nothing: the marking callbacks capture no variable, so passing one
// to the family's splitter puts nothing on the heap.
//
// MP_UNREACH_NLRI value (RFC 4760 Section 4) and one withdrawn NLRI (RFC 8277
// Section 2.4, Figure 4), after the four-octet Path Identifier when ADD-PATH is
// negotiated for the family (RFC 7911 Section 3):
//
//	+---------------------------------------------------------+
//	| AFI (2 octets) | SAFI (1 octet) | Withdrawn Routes (var) |
//	+---------------------------------------------------------+
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|    Length     |        Compatibility                          |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                          Prefix (RD first for SAFI 128)       ~
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Offset 0 is the Length octet, offsets 1 to 3 the Compatibility field.
func setWithdrawalCompatibility(value []byte, encoding *bgpctx.EncodingContext) {
	if len(value) < 3 {
		return
	}
	fam := family.Family{AFI: family.AFI(binary.BigEndian.Uint16(value)), SAFI: family.SAFI(value[2])}
	split := nlrisplit.CompatibilitySplitter(fam)
	if split == nil {
		return
	}
	section := value[3:]
	addPath := encoding.AddPathFor(fam)
	// The counting pass: nothing is rewritten unless the whole section frames,
	// so a malformed section reaches the wire unchanged.
	if _, err := split(section, addPath, nil); err != nil {
		return
	}
	mark := markCompatibility
	if addPath {
		mark = markCompatibilityAddPath
	}
	_, _ = split(section, addPath, mark) //nolint:errcheck // the counting pass above framed this section without error
}

// markCompatibility sets the Compatibility field of one withdrawn NLRI framed
// with no ADD-PATH identifier: [length:1][Compatibility:3][prefix].
func markCompatibility(route []byte) bool {
	writeCompatibility(route, 1)
	return true
}

// markCompatibilityAddPath is markCompatibility after the four-octet Path
// Identifier: [path id:4][length:1][Compatibility:3][prefix].
func markCompatibilityAddPath(route []byte) bool {
	writeCompatibility(route, 5)
	return true
}

// writeCompatibility writes 0x800000 at route[field:field+3], when route holds
// it.
func writeCompatibility(route []byte, field int) {
	if len(route) < field+3 {
		return
	}
	// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD
	// be set to 0x800000."
	route[field], route[field+1], route[field+2] = 0x80, 0x00, 0x00
}
