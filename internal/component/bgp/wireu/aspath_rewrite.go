// Design: docs/architecture/wire/attributes.md — AS-PATH rewriting for EBGP forwarding
// RFC: rfc/short/rfc4271.md — AS_PATH prepend on EBGP (Section 5.1.2)
// RFC: rfc/short/rfc6793.md — 4-byte ASN AS_PATH rewriting
// Related: aspath_transcode.go — transcode-only (no prepend) for RS-client forwarding

package wireu

import (
	"encoding/binary"
	"errors"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

var errRewriteAsPathNoAsnsTo = errors.New("rewrite AS_PATH: no ASNs to prepend")

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
		return 0, errRewriteAsPathNoAsnsTo
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

	// Scan attributes to find AS_PATH. Break early: the hot path
	// (tryDirectPrepend, same encoding) only needs AS_PATH position.
	aspAttrOff := -1
	aspHdrLen := 0
	aspValueLen := 0
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
		// No AS_PATH found -- insert one.
		return rewriteInsertASPath(dst, payload, asns, dstASN4, attrLen, attrLenOff, nlriStart)
	}

	// Fast path: same encoding, single prepend into AS_SEQUENCE with room.
	// No AGGREGATOR scan needed since encoding matches.
	if n, ok := tryDirectPrepend(dst, payload, asns, srcASN4, dstASN4,
		aspAttrOff, aspHdrLen, aspValueLen, attrLenOff, attrLen); ok {
		return n, nil
	}

	// Slow path: cross-encoding or complex AS_PATH. Scan for
	// AGGREGATOR/AS4_AGGREGATOR. Bounds already validated by the
	// AS_PATH scan above.
	aggAttrOff := -1
	aggHdrLen := 0
	aggValueLen := 0
	as4AggAttrOff := -1
	as4AggHdrLen := 0
	as4AggValueLen := 0

	off = attrsStart
	for off < attrsStart+attrLen {
		flags := attribute.AttributeFlags(payload[off])
		code := attribute.AttributeCode(payload[off+1])
		var length, hdrLen int
		if flags.IsExtLength() {
			length = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hdrLen = 4
		} else {
			length = int(payload[off+2])
			hdrLen = 3
		}

		if code == attribute.AttrAggregator {
			aggAttrOff = off
			aggHdrLen = hdrLen
			aggValueLen = length
		}
		if code == attribute.AttrAS4Aggregator {
			as4AggAttrOff = off
			as4AggHdrLen = hdrLen
			as4AggValueLen = length
		}

		off += hdrLen + length
	}

	return rewritePrependASPathFull(dst, payload, asns, srcASN4, dstASN4,
		aspAttrOff, aspHdrLen, aspValueLen, attrLenOff, attrLen, nlriStart,
		aggAttrOff, aggHdrLen, aggValueLen,
		as4AggAttrOff, as4AggHdrLen, as4AggValueLen)
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

// tryDirectPrepend attempts a zero-allocation offset-based AS_SEQUENCE prepend.
// Succeeds when: same ASN encoding, single prepend, first segment is AS_SEQUENCE
// with count < 255, and the new value length stays within the same header size class.
// Returns (bytesWritten, true) on success, (0, false) to fall back.
func tryDirectPrepend(dst, payload []byte, asns []uint32, srcASN4, dstASN4 bool,
	aspAttrOff, aspHdrLen, aspValueLen, attrLenOff, attrLen int) (int, bool) {

	if srcASN4 != dstASN4 || len(asns) != 1 {
		return 0, false
	}

	aspValueStart := aspAttrOff + aspHdrLen
	aspValue := payload[aspValueStart : aspValueStart+aspValueLen]
	if len(aspValue) < 2 {
		return 0, false
	}

	segType := attribute.ASPathSegmentType(aspValue[0])
	segCount := int(aspValue[1])
	if segType != attribute.ASSequence || segCount >= attribute.MaxASPathSegmentLength {
		return 0, false
	}

	asnSize := 4
	if !dstASN4 {
		asnSize = 2
	}

	newValueLen := aspValueLen + asnSize
	newHdrLen := 3
	if newValueLen > 255 {
		newHdrLen = 4
	}
	if newHdrLen != aspHdrLen {
		return 0, false
	}

	oldAttrWireSize := aspHdrLen + aspValueLen
	newAttrWireSize := newHdrLen + newValueLen
	shift := newAttrWireSize - oldAttrWireSize

	off := 0
	off += copy(dst[off:], payload[:aspAttrOff])

	off += attribute.WriteHeaderTo(dst, off, attribute.FlagTransitive, attribute.AttrASPath, uint16(newValueLen)) //nolint:gosec // bounded by BGP max

	dst[off] = byte(segType)
	dst[off+1] = byte(segCount + 1)
	off += 2

	asn := asns[0]
	if dstASN4 {
		binary.BigEndian.PutUint32(dst[off:], asn)
	} else {
		if asn > 0xFFFF {
			asn = 23456 // AS_TRANS per RFC 6793
		}
		binary.BigEndian.PutUint16(dst[off:], uint16(asn)) //nolint:gosec // AS_TRANS handles overflow
	}
	off += asnSize

	off += copy(dst[off:], aspValue[2:])

	aspAttrEnd := aspAttrOff + oldAttrWireSize
	off += copy(dst[off:], payload[aspAttrEnd:])

	newAttrLen := attrLen + shift
	binary.BigEndian.PutUint16(dst[attrLenOff:attrLenOff+2], uint16(newAttrLen)) //nolint:gosec // bounded by BGP max

	return off, true
}

// rewritePrependASPathFull is the fallback that parses the AS_PATH, prepends
// via the ASPath object, and re-serializes. Handles AS_SET, segment overflow,
// cross-ASN-encoding, multi-ASN prepend, and AGGREGATOR transcoding.
func rewritePrependASPathFull(dst, payload []byte, asns []uint32, srcASN4, dstASN4 bool,
	aspAttrOff, aspHdrLen, aspValueLen, attrLenOff, attrLen, nlriStart int,
	aggAttrOff, aggHdrLen, aggValueLen int,
	as4AggAttrOff, as4AggHdrLen, as4AggValueLen int) (int, error) {

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

	// Compute new AS_PATH sizes
	newValueLen := existingPath.LenWithASN4(dstASN4)
	newHdrLen := 3
	if newValueLen > 255 {
		newHdrLen = 4
	}

	// --- AGGREGATOR transcoding (RFC 6793 Section 4.2.2) ---

	var aggASN uint32
	var aggIP []byte
	var needAS4Agg bool
	var newAggValueLen int
	if aggAttrOff != -1 && srcASN4 != dstASN4 {
		aggValueStart := aggAttrOff + aggHdrLen
		if srcASN4 && aggValueLen == 8 {
			aggASN = binary.BigEndian.Uint32(payload[aggValueStart : aggValueStart+4])
			aggIP = payload[aggValueStart+4 : aggValueStart+8]
			needAS4Agg = aggASN > 65535
			newAggValueLen = 6
		} else if !srcASN4 && aggValueLen == 6 {
			aggASN = uint32(binary.BigEndian.Uint16(payload[aggValueStart : aggValueStart+2]))
			aggIP = payload[aggValueStart+2 : aggValueStart+6]
			newAggValueLen = 8
		}
	}

	// --- Compute new attrLen ---

	newAttrLen := attrLen
	newAttrLen += (newHdrLen + newValueLen) - (aspHdrLen + aspValueLen)

	if newAggValueLen != 0 {
		newAttrLen += (3 + newAggValueLen) - (aggHdrLen + aggValueLen)
	}
	if as4AggAttrOff != -1 && newAggValueLen != 0 {
		newAttrLen -= as4AggHdrLen + as4AggValueLen
	}
	if needAS4Agg {
		newAttrLen += 3 + 8
	}

	// --- Build output: iterate attributes ---

	attrsStart := attrLenOff + 2
	n := copy(dst, payload[:attrsStart])

	off := attrsStart
	for off < nlriStart {
		flags := attribute.AttributeFlags(payload[off])
		code := attribute.AttributeCode(payload[off+1])
		var length, hl int
		if flags.IsExtLength() {
			length = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hl = 4
		} else {
			length = int(payload[off+2])
			hl = 3
		}

		switch code {
		case attribute.AttrASPath:
			n += attribute.WriteHeaderTo(dst, n, attribute.FlagTransitive,
				attribute.AttrASPath, uint16(newValueLen)) //nolint:gosec // bounded by BGP max
			n += existingPath.WriteToWithASN4(dst, n, dstASN4)

		case attribute.AttrAggregator:
			if newAggValueLen != 0 {
				n += attribute.WriteHeaderTo(dst, n,
					attribute.FlagOptional|attribute.FlagTransitive,
					attribute.AttrAggregator, uint16(newAggValueLen)) //nolint:gosec // bounded by BGP max
				if newAggValueLen == 6 {
					// RFC 6793 Section 4.2.2: "set the AS number field in the
					// existing AGGREGATOR attribute to the reserved AS number, AS_TRANS"
					asn := aggASN
					if asn > 65535 {
						asn = 23456
					}
					binary.BigEndian.PutUint16(dst[n:], uint16(asn)) //nolint:gosec // AS_TRANS handles overflow
					n += 2
				} else {
					binary.BigEndian.PutUint32(dst[n:], aggASN)
					n += 4
				}
				n += copy(dst[n:], aggIP)
			} else {
				n += copy(dst[n:], payload[off:off+hl+length])
			}

		case attribute.AttrAS4Aggregator:
			if newAggValueLen == 0 {
				n += copy(dst[n:], payload[off:off+hl+length])
			}
			// Otherwise skip: replaced by new AS4_AGGREGATOR appended at end.

		default:
			n += copy(dst[n:], payload[off:off+hl+length])
		}

		off += hl + length
	}

	// Append new AS4_AGGREGATOR.
	if needAS4Agg {
		n += attribute.WriteHeaderTo(dst, n,
			attribute.FlagOptional|attribute.FlagTransitive,
			attribute.AttrAS4Aggregator, 8)
		binary.BigEndian.PutUint32(dst[n:], aggASN)
		n += 4
		n += copy(dst[n:], aggIP)
	}

	// Copy NLRI.
	n += copy(dst[n:], payload[nlriStart:])

	// Update attrLen.
	binary.BigEndian.PutUint16(dst[attrLenOff:attrLenOff+2], uint16(newAttrLen)) //nolint:gosec // bounded by BGP max

	return n, nil
}
