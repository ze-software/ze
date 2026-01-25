// Package attribute implements BGP path attributes per RFC 4271.
package attribute

import (
	"encoding/binary"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/wire"
)

// ASPathSegmentType identifies the type of AS path segment.
//
// RFC 4271 Section 4.3 (UPDATE Message Format):
//
//	"The path segment type is a 1-octet length field with the
//	 following values defined:
//	   Value  Segment Type
//	     1    AS_SET: unordered set of ASes a route in the UPDATE message has traversed
//	     2    AS_SEQUENCE: ordered set of ASes a route in the UPDATE message has traversed"
//
// RFC 5065 defines additional confederation segment types (AS_CONFED_SEQUENCE=3, AS_CONFED_SET=4).
type ASPathSegmentType uint8

// AS path segment types per RFC 4271 Section 4.3 and RFC 5065.
const (
	ASSet            ASPathSegmentType = 1 // RFC 4271: Unordered set of ASes
	ASSequence       ASPathSegmentType = 2 // RFC 4271: Ordered sequence of ASes
	ASConfedSequence ASPathSegmentType = 3 // RFC 5065: Confederation sequence
	ASConfedSet      ASPathSegmentType = 4 // RFC 5065: Confederation set
)

// MaxASPathSegmentLength is the maximum number of ASNs in a single segment.
// RFC 4271 Section 4.3: "The path segment length is a 1-octet length field"
// This means max value is 255.
const MaxASPathSegmentLength = 255

// MaxASPathTotalLength is the maximum total number of ASNs allowed in an AS path.
// RFC 4271 does not mandate a specific limit, but implementations should enforce
// a reasonable limit to prevent DoS attacks. Real-world paths rarely exceed 50 ASNs.
// Default is 1000 - large enough for segment splitting tests (255+255+...) while
// still protecting against memory exhaustion from malicious peers.
const MaxASPathTotalLength = 1000

// ASPathSegment represents a segment in an AS path.
//
// RFC 4271 Section 4.3:
//
//	"Each AS path segment is represented by a triple
//	 <path segment type, path segment length, path segment value>.
//	 The path segment length is a 1-octet length field, containing
//	 the number of ASes (not the number of octets) in the path segment value field.
//	 The path segment value field contains one or more AS numbers,
//	 each encoded as a 2-octet length field."
//
// RFC 6793 Section 3 updates this to allow 4-octet AS numbers when both
// peers support the four-octet AS number capability.
type ASPathSegment struct {
	Type ASPathSegmentType
	ASNs []uint32 // Stored as 4-byte ASNs per RFC 6793
}

// ASPath represents the AS_PATH attribute (Type Code 2).
//
// RFC 4271 Section 5.1.2:
//
//	"AS_PATH is a well-known mandatory attribute. This attribute
//	 identifies the autonomous systems through which routing information
//	 carried in this UPDATE message has passed. The components of this
//	 list can be AS_SETs or AS_SEQUENCEs."
//
// RFC 6793 extends AS_PATH to support 4-octet AS numbers when both
// peers have negotiated the four-octet AS number capability.
type ASPath struct {
	Segments []ASPathSegment
}

// Code returns AttrASPath (Type Code 2).
func (p *ASPath) Code() AttributeCode { return AttrASPath }

// Flags returns FlagTransitive.
// RFC 4271 Section 5.1.2: AS_PATH is a well-known mandatory attribute.
// Well-known attributes use the Transitive flag.
func (p *ASPath) Flags() AttributeFlags { return FlagTransitive }

// Len returns the packed length in bytes (4-byte ASN format per RFC 6793).
func (p *ASPath) Len() int {
	return p.LenWithASN4(true)
}

// LenWithASN4 returns the packed length in bytes.
//
// RFC 4271 Section 4.3: Each segment is encoded as type(1) + count(1) + ASNs.
// RFC 6793 Section 4.1: Between NEW speakers, AS numbers are 4-octet entities.
// RFC 6793 Section 4.2: Between NEW and OLD speakers, AS numbers are 2-octet.
//
// Accounts for segment splitting when segments exceed MaxASPathSegmentLength (255).
func (p *ASPath) LenWithASN4(asn4 bool) int {
	length := 0
	asnSize := 2
	if asn4 {
		asnSize = 4
	}
	for _, seg := range p.Segments {
		numASNs := len(seg.ASNs)
		if numASNs == 0 {
			continue
		}
		// Calculate number of segments needed after splitting
		numSegments := (numASNs + MaxASPathSegmentLength - 1) / MaxASPathSegmentLength
		// Each segment: type(1) + count(1) + ASNs
		length += numSegments*2 + numASNs*asnSize
	}
	return length
}

// Pack serializes the AS path (4-byte ASN format per RFC 6793).
func (p *ASPath) Pack() []byte {
	return p.PackWithASN4(true)
}

// PackWithContext serializes AS_PATH with context-dependent ASN size.
//
// RFC 6793 transcoding scenarios:
//
//	srcCtx.ASN4=true  → dstCtx.ASN4=true:  encode 4-byte
//	srcCtx.ASN4=true  → dstCtx.ASN4=false: encode 2-byte (AS_TRANS for large ASNs)
//	srcCtx.ASN4=false → dstCtx.ASN4=true:  encode 4-byte (after AS4_PATH merge)
//	srcCtx.ASN4=false → dstCtx.ASN4=false: encode 2-byte
//
// Note: AS4_PATH merge/generation is handled at UPDATE processing level.
// This method handles the encoding format based on dstCtx.ASN4.
func (p *ASPath) PackWithContext(_, dstCtx *bgpctx.EncodingContext) []byte {
	if dstCtx == nil || dstCtx.ASN4() {
		return p.PackWithASN4(true)
	}
	return p.PackWithASN4(false)
}

// PackWithASN4 serializes the AS path.
//
// RFC 6793 Section 4.1: Between NEW speakers, AS numbers are 4-octet.
// RFC 6793 Section 4.2.2: When sending to OLD speakers, non-mappable
// 4-octet AS numbers are represented by AS_TRANS (23456).
//
// RFC 6793 Section 3: "AS_TRANS can be used to represent non-mappable
// four-octet AS numbers as two-octet AS numbers in AS path information
// that is encoded with two-octet AS numbers."
//
// RFC 4271 Section 4.3: Segments with >255 ASNs are split into multiple
// segments of the same type.
func (p *ASPath) PackWithASN4(asn4 bool) []byte {
	if len(p.Segments) == 0 {
		return []byte{}
	}

	buf := make([]byte, p.LenWithASN4(asn4))
	offset := 0

	for _, seg := range p.Segments {
		// RFC 4271: Split segments that exceed 255 ASNs
		offset = packSegmentWithSplit(buf, offset, seg.Type, seg.ASNs, asn4)
	}

	return buf[:offset]
}

// WriteTo writes the AS path (4-byte ASN format per RFC 6793) into buf at offset.
func (p *ASPath) WriteTo(buf []byte, off int) int {
	return p.WriteToWithASN4(buf, off, true)
}

// WriteToWithContext writes AS_PATH with context-dependent ASN size.
func (p *ASPath) WriteToWithContext(buf []byte, off int, _, dstCtx *bgpctx.EncodingContext) int {
	if dstCtx == nil || dstCtx.ASN4() {
		return p.WriteToWithASN4(buf, off, true)
	}
	return p.WriteToWithASN4(buf, off, false)
}

// WriteToWithASN4 writes the AS path into buf at offset.
func (p *ASPath) WriteToWithASN4(buf []byte, off int, asn4 bool) int {
	if len(p.Segments) == 0 {
		return 0
	}

	start := off
	for _, seg := range p.Segments {
		off = writeSegmentWithSplit(buf, off, seg.Type, seg.ASNs, asn4)
	}
	return off - start
}

// CheckedWriteTo validates capacity before writing.
func (p *ASPath) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := p.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return p.WriteTo(buf, off), nil
}

// LenWithContext returns length based on encoding context.
// RFC 6793: 4-byte ASNs for ASN4 peers, 2-byte for legacy.
func (p *ASPath) LenWithContext(_, dstCtx *bgpctx.EncodingContext) int {
	if dstCtx == nil || dstCtx.ASN4() {
		return p.LenWithASN4(true)
	}
	return p.LenWithASN4(false)
}

// CheckedWriteToWithContext validates capacity before writing with context.
func (p *ASPath) CheckedWriteToWithContext(buf []byte, off int, srcCtx, dstCtx *bgpctx.EncodingContext) (int, error) {
	needed := p.LenWithContext(srcCtx, dstCtx)
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return p.WriteToWithContext(buf, off, srcCtx, dstCtx), nil
}

// writeSegmentWithSplit writes a segment, splitting if it exceeds MaxASPathSegmentLength.
//
//nolint:dupl // Intentionally parallel to packSegmentWithSplit - different output mechanism
func writeSegmentWithSplit(buf []byte, off int, segType ASPathSegmentType, asns []uint32, asn4 bool) int {
	if len(asns) == 0 {
		return off
	}

	count := len(asns)
	if count > MaxASPathSegmentLength {
		count = MaxASPathSegmentLength
	}

	buf[off] = byte(segType)
	buf[off+1] = byte(count)
	off += 2

	for i := 0; i < count; i++ {
		if asn4 {
			binary.BigEndian.PutUint32(buf[off:], asns[i])
			off += 4
		} else {
			var as16 uint16
			if asns[i] > 65535 {
				as16 = 23456 // AS_TRANS per RFC 6793 Section 9
			} else {
				as16 = uint16(asns[i]) //nolint:gosec // bounds checked above
			}
			binary.BigEndian.PutUint16(buf[off:], as16)
			off += 2
		}
	}

	if len(asns) > MaxASPathSegmentLength {
		return writeSegmentWithSplit(buf, off, segType, asns[MaxASPathSegmentLength:], asn4)
	}

	return off
}

// packSegmentWithSplit encodes a segment, splitting if it exceeds MaxASPathSegmentLength.
//
//nolint:dupl // Intentionally parallel to writeSegmentWithSplit - different output mechanism
func packSegmentWithSplit(buf []byte, offset int, segType ASPathSegmentType, asns []uint32, asn4 bool) int {
	if len(asns) == 0 {
		return offset
	}

	// Determine how many ASNs fit in this segment
	count := len(asns)
	if count > MaxASPathSegmentLength {
		count = MaxASPathSegmentLength
	}

	buf[offset] = byte(segType)
	buf[offset+1] = byte(count)
	offset += 2

	for i := 0; i < count; i++ {
		if asn4 {
			binary.BigEndian.PutUint32(buf[offset:], asns[i])
			offset += 4
		} else {
			// RFC 6793 Section 4.2.2: Use AS_TRANS for non-mappable ASNs
			var as16 uint16
			if asns[i] > 65535 {
				as16 = 23456 // AS_TRANS per RFC 6793 Section 9
			} else {
				as16 = uint16(asns[i]) // #nosec G115 -- bounds checked above
			}
			binary.BigEndian.PutUint16(buf[offset:], as16)
			offset += 2
		}
	}

	// Recursively encode remaining ASNs if segment was split
	if len(asns) > MaxASPathSegmentLength {
		return packSegmentWithSplit(buf, offset, segType, asns[MaxASPathSegmentLength:], asn4)
	}

	return offset
}

// PathLength returns the AS path length for BGP path selection.
//
// RFC 4271 Section 9.1.2.2 (Breaking Ties):
//
//	"Remove from consideration all routes that are not tied for having
//	 the smallest number of AS numbers present in their AS_PATH attributes.
//	 Note that when counting this number, an AS_SET counts as 1, no matter
//	 how many ASes are in the set."
//
// RFC 5065: Confederation segments (AS_CONFED_SEQUENCE, AS_CONFED_SET) are
// not counted in path length calculation.
func (p *ASPath) PathLength() int {
	length := 0
	for _, seg := range p.Segments {
		switch seg.Type {
		case ASSequence:
			length += len(seg.ASNs)
		case ASSet:
			// RFC 4271 Section 9.1.2.2: AS_SET counts as 1
			if len(seg.ASNs) > 0 {
				length++
			}
		case ASConfedSequence, ASConfedSet:
			// RFC 5065: Confederation segments don't count for path selection
		}
	}
	return length
}

// Contains returns true if the AS path contains the given ASN.
// Used for loop detection per RFC 4271 Section 9 (UPDATE Message Handling).
func (p *ASPath) Contains(asn uint32) bool {
	for _, seg := range p.Segments {
		for _, a := range seg.ASNs {
			if a == asn {
				return true
			}
		}
	}
	return false
}

// Prepend adds an ASN to the beginning of the AS path.
//
// RFC 4271 Section 5.1.2:
//
//	"1) if the first path segment of the AS_PATH is of type AS_SEQUENCE,
//	    the local system prepends its own AS number as the last element
//	    of the sequence (put it in the leftmost position with respect to
//	    the position of octets in the protocol message).
//	 2) if the first path segment of the AS_PATH is of type AS_SET, the
//	    local system prepends a new path segment of type AS_SEQUENCE to
//	    the AS_PATH, including its own AS number in that segment.
//	 3) if the AS_PATH is empty, the local system creates a path segment
//	    of type AS_SEQUENCE, places its own AS into that segment, and
//	    places that segment into the AS_PATH."
//
// RFC 4271 Section 5.1.2 (overflow handling):
//
//	"If the act of prepending will cause an overflow in the AS_PATH segment
//	 (i.e., more than 255 ASes), it SHOULD prepend a new segment of type
//	 AS_SEQUENCE and prepend its own AS number to this new segment."
func (p *ASPath) Prepend(asn uint32) {
	if len(p.Segments) > 0 && p.Segments[0].Type == ASSequence {
		// Check if prepending would overflow the segment
		if len(p.Segments[0].ASNs) >= MaxASPathSegmentLength {
			// RFC 4271: Create new segment to avoid overflow
			seg := ASPathSegment{Type: ASSequence, ASNs: []uint32{asn}}
			p.Segments = append([]ASPathSegment{seg}, p.Segments...)
		} else {
			// Case 1: Prepend to existing AS_SEQUENCE
			p.Segments[0].ASNs = append([]uint32{asn}, p.Segments[0].ASNs...)
		}
	} else {
		// Case 2 & 3: Create new AS_SEQUENCE segment
		seg := ASPathSegment{Type: ASSequence, ASNs: []uint32{asn}}
		p.Segments = append([]ASPathSegment{seg}, p.Segments...)
	}
}

// ParseASPath parses an AS_PATH attribute value.
//
// RFC 4271 Section 4.3: AS_PATH is composed of path segments, each as
// <type, length, value> where length is the count of AS numbers.
//
// RFC 4271 Section 6.3: Error Subcode 11 (Malformed AS_PATH) is returned
// when the path is syntactically incorrect.
//
// RFC 6793 Section 4.1: Between NEW speakers, AS numbers are 4-octet.
// RFC 6793 Section 4.2: Between NEW and OLD speakers, AS numbers are 2-octet.
//
// The fourByte parameter indicates whether the peer supports 4-byte ASNs
// (i.e., both speakers have the four-octet AS number capability).
//
// Validation performed:
//   - Segment types must be 1-4 (RFC 4271 Section 4.3, RFC 5065)
//   - Total ASN count must not exceed MaxASPathTotalLength
func ParseASPath(data []byte, fourByte bool) (*ASPath, error) {
	path := &ASPath{}

	asnSize := 2
	if fourByte {
		asnSize = 4
	}

	offset := 0
	totalASNs := 0
	for offset < len(data) {
		// RFC 4271: Need at least type(1) + count(1)
		if offset+2 > len(data) {
			return nil, ErrShortData
		}

		segType := ASPathSegmentType(data[offset]) //nolint:gosec // Bounds checked above
		count := int(data[offset+1])               //nolint:gosec // Bounds checked above
		offset += 2

		// RFC 4271 Section 4.3: Segment types 1 (AS_SET) and 2 (AS_SEQUENCE)
		// RFC 5065: Adds types 3 (AS_CONFED_SEQUENCE) and 4 (AS_CONFED_SET)
		// Any other value is malformed per RFC 4271 Section 6.3 (Error Subcode 11).
		if segType < ASSet || segType > ASConfedSet {
			return nil, ErrMalformedASPath
		}

		// Track total ASNs to enforce maximum path length (DoS protection)
		totalASNs += count
		if totalASNs > MaxASPathTotalLength {
			return nil, ErrMalformedASPath
		}

		// Check we have enough data for ASNs
		needed := count * asnSize
		if offset+needed > len(data) {
			return nil, ErrShortData
		}

		asns := make([]uint32, count)
		for i := 0; i < count; i++ {
			if fourByte {
				// RFC 6793: 4-octet AS numbers
				asns[i] = binary.BigEndian.Uint32(data[offset:])
				offset += 4
			} else {
				// RFC 4271: 2-octet AS numbers (stored as uint32 for uniformity)
				asns[i] = uint32(binary.BigEndian.Uint16(data[offset:]))
				offset += 2
			}
		}

		path.Segments = append(path.Segments, ASPathSegment{
			Type: segType,
			ASNs: asns,
		})
	}

	return path, nil
}
