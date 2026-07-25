// Design: docs/architecture/wire/messages.md — reconstructing a received-shape UPDATE
// RFC: rfc/short/rfc4271.md — UPDATE message body layout (Section 4.3)
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI encoding (Section 3)
// Related: reactor_api_forward.go — forwardUpdateCore, the egress transform the relay reuses
// Related: reactor_wire.go — the originate-side zero-allocation UPDATE builders
//
// RFC 4271 Section 4.3 — UPDATE message body layout.
// RFC 4760 Section 3 — MP_REACH_NLRI encoding for non-IPv4-unicast families.
//
// A stored Adj-RIB-In route is (family, attribute block, next-hop, one NLRI).
// Replaying it through the forward rail needs the wire shape the SOURCE peer
// actually sent, because forwardUpdateCore's egress steps parse the payload as a
// received UPDATE. These builders reconstruct exactly that shape, writing into a
// caller-owned pooled buffer at an offset (ai/rules/buffer-first.md) — no append,
// no per-route allocation.

package reactor

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

const (
	// attrHdrMinLen is the smallest path-attribute header: flags + code + 1-byte length.
	attrHdrMinLen = 3
	// attrHdrExtLen is the extended-length header: flags + code + 2-byte length.
	attrHdrExtLen = 4
	// mpReachFixedLen is AFI(2) + SAFI(1) + NextHopLen(1) + Reserved(1), excluding
	// the variable next-hop and NLRI. RFC 4760 Section 3.
	mpReachFixedLen = 5
	// maxAttrValueLen bounds a single path attribute's value (2-byte extended length).
	maxAttrValueLen = 0xFFFF
	// maxUpdateBodyLen bounds the reconstructed UPDATE BODY. A BGP message is at
	// most 65535 octets INCLUDING the 19-octet header (RFC 4271 Section 4.1, and
	// RFC 8654 for the extended limit), so the body cannot reach 0xFFFF. Bounding
	// the body by the attribute limit accepted sizes that can never be framed;
	// they then died downstream as an anonymous "forward split failed" warning
	// instead of the named refusal this guard promises.
	maxUpdateBodyLen = message.ExtMsgLen - message.HeaderLen
	// maxNextHopLen bounds the MP_REACH next-hop, whose length is a single octet.
	maxNextHopLen = 0xFF
)

// decodeHexInto decodes the hex string s into dst, returning false unless s is
// well-formed hex that fills dst exactly.
//
// encoding/hex decodes from a []byte, so hex.Decode(dst, []byte(s)) converts and
// therefore ALLOCATES once per field. A peer-up replay runs this three times per
// stored route, which is the one place the reconstruction would allocate at all
// (ai/rules/no-sprintf-alloc.md, ai/rules/memory-architecture.md). Stored routes
// also arrive as hex across the plugin RPC boundary, so the input is untrusted:
// every nibble is validated and a bad one rejects the whole field (spec S-5).
func decodeHexInto(dst []byte, s string) bool {
	if len(s) != len(dst)*2 {
		return false
	}
	for i := range dst {
		hi, hiOK := hexNibble(s[i*2])
		lo, loOK := hexNibble(s[i*2+1])
		if !hiOK || !loOK {
			return false
		}
		dst[i] = hi<<4 | lo
	}
	return true
}

// hexNibble converts one hex digit to its value, accepting either case.
func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// relayAttrSpan describes one path attribute located inside a raw attribute block.
type relayAttrSpan struct {
	code  attribute.AttributeCode
	start int // offset of the attribute's first header byte within the block
	end   int // offset just past the attribute's value
}

// scanAttrBlock walks a raw path-attribute block and returns one span per
// attribute, in wire order. Returns ok=false on any truncation or malformed
// header: a partially-understood attribute block must never be re-emitted, since
// the bytes go straight back onto a peer's session (fail closed).
//
// spans is a caller-owned scratch slice; its contents are replaced.
func scanAttrBlock(spans []relayAttrSpan, block []byte) ([]relayAttrSpan, bool) {
	spans = spans[:0]
	for off := 0; off < len(block); {
		if off+attrHdrMinLen > len(block) {
			return spans, false
		}
		flags := block[off]
		code := attribute.AttributeCode(block[off+1])
		hdrLen := attrHdrMinLen
		valueLen := int(block[off+2])
		if flags&byte(attribute.FlagExtLength) != 0 {
			hdrLen = attrHdrExtLen
			if off+hdrLen > len(block) {
				return spans, false
			}
			valueLen = int(binary.BigEndian.Uint16(block[off+2:]))
		}
		end := off + hdrLen + valueLen
		if end > len(block) {
			return spans, false
		}
		spans = append(spans, relayAttrSpan{code: code, start: off, end: end})
		off = end
	}
	return spans, true
}

// isRelayStrippedAttr reports whether an attribute from the stored block must be
// dropped during reconstruction.
//
// MP_REACH_NLRI / MP_UNREACH_NLRI are stripped because the stored attribute block
// is the FULL attribute section of the originating UPDATE: reactor_notify.go
// assigns RawMessage.AttrsWire = WireUpdate.Attrs(), which spans every attribute
// including 14/15. Re-emitting it verbatim beside a synthesized MP_REACH would
// duplicate the attribute AND re-announce every NLRI the source UPDATE carried,
// not just the one route being replayed.
func isRelayStrippedAttr(code attribute.AttributeCode, fam family.Family) bool {
	if code == attribute.AttrMPReachNLRI || code == attribute.AttrMPUnreachNLRI {
		return true
	}
	// A legacy NEXT_HOP belongs only to the IPv4-unicast encoding. One UPDATE may
	// legally carry body NLRI (IPv4) AND MP_REACH (say IPv6), and Adj-RIB-In
	// stores the SAME attribute block for both families, so replaying the IPv6
	// route while keeping type-3 would attach a different route's IPv4 next hop.
	// The synthesized MP_REACH carries the correct next hop for these families.
	return code == attribute.AttrNextHop && fam != family.IPv4Unicast
}

// relayNeedsNextHopAttr reports whether the reconstruction must add a legacy
// NEXT_HOP (type 3) attribute. Only IPv4 unicast rides the legacy encoding, and
// only when the source did not already send a type-3 attribute — which happens
// when a peer announces IPv4 unicast inside MP_REACH_NLRI (RFC 4760) instead of
// the body NLRI field.
func relayNeedsNextHopAttr(spans []relayAttrSpan, fam family.Family) bool {
	if fam != family.IPv4Unicast {
		return false
	}
	for _, s := range spans {
		if s.code == attribute.AttrNextHop {
			return false
		}
	}
	return true
}

// writeAttrHeader writes a path-attribute header at buf[off] and returns the
// number of header bytes written. Uses the extended-length form when valueLen
// exceeds 255 (RFC 4271 Section 4.3).
func writeAttrHeader(buf []byte, off int, flags byte, code attribute.AttributeCode, valueLen int) int {
	if valueLen > 0xFF {
		buf[off] = flags | byte(attribute.FlagExtLength)
		buf[off+1] = byte(code)
		binary.BigEndian.PutUint16(buf[off+2:], uint16(valueLen)) //nolint:gosec // caller bounds valueLen to maxAttrValueLen
		return attrHdrExtLen
	}
	buf[off] = flags
	buf[off+1] = byte(code)
	buf[off+2] = byte(valueLen)
	return attrHdrMinLen
}

// attrHeaderLen returns the header size writeAttrHeader will emit for valueLen.
func attrHeaderLen(valueLen int) int {
	if valueLen > 0xFF {
		return attrHdrExtLen
	}
	return attrHdrMinLen
}

// mpReachValueLen returns the MP_REACH_NLRI value length for a synthesized
// single-NLRI attribute.
func mpReachValueLen(nextHop, nlri []byte) int {
	return mpReachFixedLen + len(nextHop) + len(nlri)
}

// writeMPReach writes a complete MP_REACH_NLRI attribute (type 14) carrying
// exactly one NLRI at buf[off], returning the bytes written.
//
// RFC 4760 Section 3:
//
//	+---------------------------------------------------------+
//	| Address Family Identifier (2 octets)                    |
//	| Subsequent Address Family Identifier (1 octet)          |
//	| Length of Next Hop Network Address (1 octet)            |
//	| Network Address of Next Hop (variable)                  |
//	| Reserved (1 octet)                                      |
//	| Network Layer Reachability Information (variable)       |
//	+---------------------------------------------------------+
func writeMPReach(buf []byte, off int, fam family.Family, nextHop, nlri []byte) int {
	start := off
	valueLen := mpReachValueLen(nextHop, nlri)
	// MP_REACH_NLRI is optional non-transitive (RFC 4760 Section 3).
	off += writeAttrHeader(buf, off, byte(attribute.FlagOptional), attribute.AttrMPReachNLRI, valueLen)

	binary.BigEndian.PutUint16(buf[off:], uint16(fam.AFI))
	off += 2
	buf[off] = byte(fam.SAFI)
	off++
	buf[off] = byte(len(nextHop))
	off++
	off += copy(buf[off:], nextHop)
	buf[off] = 0 // Reserved
	off++
	off += copy(buf[off:], nlri)

	return off - start
}

// relayPayloadLen returns the exact byte count writeRelayPayload will produce, so
// the caller can pick a standard or extended-message pool buffer and reject a
// route that cannot fit the UPDATE's 16-bit length fields before writing a byte.
//
// Returns ok=false when the route cannot be encoded. Every caller must treat that
// as "drop this route with a named error", never as "encode what fits".
func relayPayloadLen(spans []relayAttrSpan, nextHop, nlri []byte, fam family.Family, needNextHopAttr bool) (int, bool) {
	if len(nextHop) > maxNextHopLen || len(nlri) == 0 {
		return 0, false
	}
	// RFC 4271 Section 5.1.3: the legacy NEXT_HOP attribute is exactly 4 octets.
	// A route can reach here with a 16-byte next hop -- RFC 5549 carries IPv4
	// unicast with an IPv6 next hop inside MP_REACH, and the stored next-hop is
	// whatever the source sent. Emitting those 16 bytes as a well-known mandatory
	// type-3 attribute produces an attribute-length error at the peer, so reject
	// rather than encode something the destination must discard.
	if fam == family.IPv4Unicast && needNextHopAttr && len(nextHop) != 4 {
		return 0, false
	}
	// Withdrawn Routes Length (2) + Total Path Attribute Length (2).
	total := 4
	attrLen := 0
	for _, s := range spans {
		if isRelayStrippedAttr(s.code, fam) {
			continue
		}
		attrLen += s.end - s.start
	}
	if fam == family.IPv4Unicast {
		if needNextHopAttr {
			attrLen += attrHdrMinLen + len(nextHop)
		}
	} else {
		valueLen := mpReachValueLen(nextHop, nlri)
		if valueLen > maxAttrValueLen {
			return 0, false
		}
		attrLen += attrHeaderLen(valueLen) + valueLen
	}
	if attrLen > maxAttrValueLen {
		return 0, false
	}
	total += attrLen
	if fam == family.IPv4Unicast {
		total += len(nlri)
	}
	if total > maxUpdateBodyLen {
		return 0, false
	}
	return total, true
}

// writeRelayPayload reconstructs a received-shape UPDATE body for one stored
// route into buf at off, returning the bytes written. The caller MUST have sized
// the buffer with relayPayloadLen and honored its ok=false.
//
// The body is an announce, so Withdrawn Routes Length is always zero. Attribute
// order from the source is preserved for every surviving attribute: a real
// forward relays the source's own byte order, and the functional tests assert
// exact hex. Only the stripped MP attributes (and, for IPv4 unicast delivered via
// MP_REACH, a re-added NEXT_HOP) change the block.
func writeRelayPayload(buf []byte, off int, spans []relayAttrSpan, attrs, nextHop, nlri []byte, fam family.Family, needNextHopAttr bool) int {
	start := off

	// RFC 4271 Section 4.3: Withdrawn Routes Length = 0 (this is an announce).
	buf[off] = 0
	buf[off+1] = 0
	off += 2

	// Total Path Attribute Length placeholder, backfilled after the block.
	attrLenPos := off
	off += 2
	attrStart := off

	for _, s := range spans {
		if isRelayStrippedAttr(s.code, fam) {
			continue
		}
		off += copy(buf[off:], attrs[s.start:s.end])
	}

	if fam == family.IPv4Unicast {
		if needNextHopAttr {
			// RFC 4271 Section 5.1.3: NEXT_HOP is well-known mandatory, so the
			// legacy IPv4 unicast encoding the relay emits must carry one even
			// when the source delivered the route inside MP_REACH_NLRI.
			off += writeAttrHeader(buf, off, byte(attribute.FlagTransitive), attribute.AttrNextHop, len(nextHop))
			off += copy(buf[off:], nextHop)
		}
	} else {
		off += writeMPReach(buf, off, fam, nextHop, nlri)
	}

	binary.BigEndian.PutUint16(buf[attrLenPos:], uint16(off-attrStart)) //nolint:gosec // bounded by relayPayloadLen

	// RFC 4271 Section 4.3: IPv4 unicast NLRI rides the body field; every other
	// family is carried inside the MP_REACH_NLRI written above.
	if fam == family.IPv4Unicast {
		off += copy(buf[off:], nlri)
	}

	return off - start
}
