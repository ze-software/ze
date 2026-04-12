// Design: docs/architecture/wire/attributes.md — AS-PATH rewriting for EBGP forwarding
// RFC: rfc/short/rfc4271.md — AS_PATH prepend on EBGP (Section 5.1.2)
// RFC: rfc/short/rfc6793.md — 4-byte ASN AS_PATH rewriting

package wireu

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// RewriteASPath rewrites an UPDATE payload by prepending localASN to the AS_PATH.
//
// RFC 4271 Section 9.1.2: When propagating a route to an EBGP peer, a BGP speaker
// MUST prepend its own AS number to the AS_PATH attribute.
//
// RFC 6793 Section 4: When advertising to non-ASN4 peers, ASNs > 65535 are encoded
// as AS_TRANS (23456) in the 2-octet AS_PATH.
//
// Parameters:
//   - dst: destination buffer (must have room for patched payload)
//   - payload: UPDATE body (wdLen(2) + withdrawn + attrLen(2) + attrs + nlri)
//   - localASN: the local AS number to prepend
//   - srcASN4: whether the source encoded AS_PATH with 4-byte ASNs
//   - dstASN4: whether the destination wants 4-byte ASN encoding
//
// Returns the number of bytes written to dst, or an error.
func RewriteASPath(dst, payload []byte, localASN uint32, srcASN4, dstASN4 bool) (int, error) {
	// Stack-allocated single-element array avoids heap allocation on the hot path.
	// The EBGPWire cache amortizes this, but the fast path is free.
	asns := [1]uint32{localASN}
	return rewriteASPathPrepend(dst, payload, asns[:], srcASN4, dstASN4)
}

// RewriteASPathDual prepends two ASNs to AS_PATH: primaryASN ends up closest
// to the peer (outermost), secondaryASN sits behind it.
//
// Used for the local-as override "dual-AS" mode: primaryASN is the override
// the peer expects to see, secondaryASN is the router's real AS. With no
// local-as modifiers set, downstream peers see AS_PATH = [override, real, ...].
// When no-prepend or replace-as is set, the caller uses RewriteASPath with
// only the override and skips this dual variant.
//
// RFC 7705 references the "local-as" feature and its dual-AS semantics.
func RewriteASPathDual(dst, payload []byte, primaryASN, secondaryASN uint32, srcASN4, dstASN4 bool) (int, error) {
	// asns[0] is inserted first (innermost), asns[len-1] last (outermost closest to peer).
	// Prepend order below iterates the slice and calls Prepend one by one, so the
	// last element prepended ends up in front. Final order: [primaryASN, secondaryASN, ...].
	asns := [2]uint32{secondaryASN, primaryASN}
	return rewriteASPathPrepend(dst, payload, asns[:], srcASN4, dstASN4)
}

// rewriteASPathPrepend parses AS_PATH from payload, prepends asns (in order,
// so asns[len-1] ends up outermost), re-encodes, and writes the patched
// UPDATE body to dst.
func rewriteASPathPrepend(dst, payload []byte, asns []uint32, srcASN4, dstASN4 bool) (int, error) {
	if len(asns) == 0 {
		return 0, fmt.Errorf("rewrite AS_PATH: no ASNs to prepend")
	}
	// Parse UPDATE body layout: wdLen(2) + withdrawn(wdLen) + attrLen(2) + attrs(attrLen) + nlri
	if len(payload) < 4 {
		return 0, fmt.Errorf("rewrite AS_PATH: %w", ErrUpdateTruncated)
	}

	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	if len(payload) < 2+wdLen+2 {
		return 0, fmt.Errorf("rewrite AS_PATH: %w", ErrUpdateTruncated)
	}

	attrLenOff := 2 + wdLen
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2

	if len(payload) < attrsStart+attrLen {
		return 0, fmt.Errorf("rewrite AS_PATH: %w", ErrUpdateTruncated)
	}

	nlriStart := attrsStart + attrLen

	// Scan attributes to find AS_PATH (type code 2)
	aspAttrOff := -1 // offset of AS_PATH attribute relative to payload start
	aspHdrLen := 0   // header length (3 or 4)
	aspValueLen := 0 // value length
	off := attrsStart

	for off < attrsStart+attrLen {
		if off+3 > len(payload) {
			return 0, fmt.Errorf("rewrite AS_PATH: truncated attribute at offset %d: %w", off, ErrUpdateMalformed)
		}

		flags := attribute.AttributeFlags(payload[off])
		code := attribute.AttributeCode(payload[off+1])

		var length int
		var hdrLen int
		if flags.IsExtLength() {
			if off+4 > len(payload) {
				return 0, fmt.Errorf("rewrite AS_PATH: truncated ext-length attribute: %w", ErrUpdateMalformed)
			}
			length = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hdrLen = 4
		} else {
			length = int(payload[off+2])
			hdrLen = 3
		}

		if off+hdrLen+length > len(payload) {
			return 0, fmt.Errorf("rewrite AS_PATH: attribute value overflows payload: %w", ErrUpdateMalformed)
		}

		if code == attribute.AttrASPath {
			aspAttrOff = off
			aspHdrLen = hdrLen
			aspValueLen = length
			break
		}

		off += hdrLen + length
	}

	if aspAttrOff == -1 {
		// No AS_PATH found — insert one
		return rewriteInsertASPath(dst, payload, asns, dstASN4, attrLen, attrLenOff, nlriStart)
	}

	return rewritePrependASPath(dst, payload, asns, srcASN4, dstASN4,
		aspAttrOff, aspHdrLen, aspValueLen, attrLenOff, attrLen)
}

// rewriteInsertASPath handles the case where no AS_PATH exists in the payload.
// Inserts a complete AS_PATH attribute at the end of the attributes section.
// asns are placed in the new AS_SEQUENCE with asns[len-1] at the outermost
// position (matches the prepend order convention of rewriteASPathPrepend).
func rewriteInsertASPath(dst, payload []byte, asns []uint32, dstASN4 bool,
	attrLen, attrLenOff, nlriStart int) (int, error) {

	// Build the new AS_PATH: single AS_SEQUENCE segment with the prepended ASNs
	// in "outermost last" order. Reverse asns so asns[len-1] becomes segment[0].
	segASNs := make([]uint32, len(asns))
	for i, a := range asns {
		segASNs[len(asns)-1-i] = a
	}
	newPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: segASNs},
		},
	}

	// Calculate new attribute wire size (header + value)
	newValueLen := newPath.LenWithASN4(dstASN4)
	newHdrLen := 3
	if newValueLen > 255 {
		newHdrLen = 4
	}
	newAttrWireSize := newHdrLen + newValueLen

	// Copy everything before NLRI (includes wdLen, withdrawn, attrLen, all attrs)
	off := copy(dst, payload[:nlriStart])

	// Write new AS_PATH attribute at end of attrs section
	off += attribute.WriteHeaderTo(dst, off, attribute.FlagTransitive, attribute.AttrASPath, uint16(newValueLen)) //nolint:gosec // bounded by BGP max
	off += newPath.WriteToWithASN4(dst, off, dstASN4)

	// Copy NLRI (if any)
	off += copy(dst[off:], payload[nlriStart:])

	// Update global attrLen
	newAttrLen := attrLen + newAttrWireSize
	binary.BigEndian.PutUint16(dst[attrLenOff:attrLenOff+2], uint16(newAttrLen)) //nolint:gosec // bounded by BGP max

	return off, nil
}

// rewritePrependASPath handles the case where an AS_PATH exists.
// Parses it, prepends each asn in order (asns[len-1] ends up outermost),
// re-encodes, and adjusts lengths.
func rewritePrependASPath(dst, payload []byte, asns []uint32, srcASN4, dstASN4 bool,
	aspAttrOff, aspHdrLen, aspValueLen, attrLenOff, attrLen int) (int, error) {

	// Parse existing AS_PATH value
	aspValueStart := aspAttrOff + aspHdrLen
	aspValue := payload[aspValueStart : aspValueStart+aspValueLen]

	existingPath, err := attribute.ParseASPath(aspValue, srcASN4)
	if err != nil {
		return 0, fmt.Errorf("rewrite AS_PATH: parse existing: %w", err)
	}

	// Prepend each ASN in order: asns[0] first (innermost), asns[len-1] last (outermost).
	// Each Prepend call handles segment overflow at 255 and AS_SET cases.
	for _, asn := range asns {
		existingPath.Prepend(asn)
	}

	// Compute new sizes
	oldAttrWireSize := aspHdrLen + aspValueLen
	newValueLen := existingPath.LenWithASN4(dstASN4)
	newHdrLen := 3
	if newValueLen > 255 {
		newHdrLen = 4
	}
	newAttrWireSize := newHdrLen + newValueLen
	shift := newAttrWireSize - oldAttrWireSize

	// Write patched payload into dst
	off := 0

	// 1. Copy bytes before AS_PATH attribute
	off += copy(dst[off:], payload[:aspAttrOff])

	// 2. Write new AS_PATH attribute header
	off += attribute.WriteHeaderTo(dst, off, attribute.FlagTransitive, attribute.AttrASPath, uint16(newValueLen)) //nolint:gosec // bounded by BGP max

	// 3. Write new AS_PATH value
	off += existingPath.WriteToWithASN4(dst, off, dstASN4)

	// 4. Copy bytes after old AS_PATH attribute (remaining attrs + NLRI)
	aspAttrEnd := aspAttrOff + oldAttrWireSize
	off += copy(dst[off:], payload[aspAttrEnd:])

	// 5. Update global attrLen
	newAttrLen := attrLen + shift
	binary.BigEndian.PutUint16(dst[attrLenOff:attrLenOff+2], uint16(newAttrLen)) //nolint:gosec // bounded by BGP max

	return off, nil
}
