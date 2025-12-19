package attribute

import (
	"encoding/binary"
)

// ASPathSegmentType identifies the type of AS path segment.
type ASPathSegmentType uint8

// AS path segment types per RFC 4271.
const (
	ASSet            ASPathSegmentType = 1 // Unordered set of ASNs
	ASSequence       ASPathSegmentType = 2 // Ordered sequence of ASNs
	ASConfedSet      ASPathSegmentType = 3 // Confederation set (RFC 5065)
	ASConfedSequence ASPathSegmentType = 4 // Confederation sequence (RFC 5065)
)

// ASPathSegment represents a segment in an AS path.
type ASPathSegment struct {
	Type ASPathSegmentType
	ASNs []uint32
}

// ASPath represents the AS_PATH attribute (RFC 4271 Section 5.1.2).
type ASPath struct {
	Segments []ASPathSegment
}

// Code returns AttrASPath.
func (p *ASPath) Code() AttributeCode { return AttrASPath }

// Flags returns FlagTransitive (AS_PATH is well-known mandatory).
func (p *ASPath) Flags() AttributeFlags { return FlagTransitive }

// Len returns the packed length in bytes (4-byte ASN format).
func (p *ASPath) Len() int {
	length := 0
	for _, seg := range p.Segments {
		// type(1) + count(1) + ASNs(4 each)
		length += 2 + len(seg.ASNs)*4
	}
	return length
}

// Pack serializes the AS path (4-byte ASN format).
func (p *ASPath) Pack() []byte {
	if len(p.Segments) == 0 {
		return []byte{}
	}

	buf := make([]byte, p.Len())
	offset := 0

	for _, seg := range p.Segments {
		buf[offset] = byte(seg.Type)
		buf[offset+1] = byte(len(seg.ASNs))
		offset += 2

		for _, asn := range seg.ASNs {
			binary.BigEndian.PutUint32(buf[offset:], asn)
			offset += 4
		}
	}

	return buf
}

// PathLength returns the AS path length for BGP path selection.
// AS_SET counts as 1 regardless of size. AS_SEQUENCE counts each ASN.
// Confederation segments are not counted.
func (p *ASPath) PathLength() int {
	length := 0
	for _, seg := range p.Segments {
		switch seg.Type {
		case ASSequence:
			length += len(seg.ASNs)
		case ASSet:
			if len(seg.ASNs) > 0 {
				length++ // Set counts as 1
			}
			// Confederation segments don't count
		}
	}
	return length
}

// Contains returns true if the AS path contains the given ASN.
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
// If the first segment is AS_SEQUENCE, prepends to it.
// Otherwise, creates a new AS_SEQUENCE segment.
func (p *ASPath) Prepend(asn uint32) {
	if len(p.Segments) > 0 && p.Segments[0].Type == ASSequence {
		p.Segments[0].ASNs = append([]uint32{asn}, p.Segments[0].ASNs...)
	} else {
		seg := ASPathSegment{Type: ASSequence, ASNs: []uint32{asn}}
		p.Segments = append([]ASPathSegment{seg}, p.Segments...)
	}
}

// ParseASPath parses an AS_PATH attribute value.
// fourByte indicates whether 4-byte ASN format is used.
func ParseASPath(data []byte, fourByte bool) (*ASPath, error) {
	path := &ASPath{}

	asnSize := 2
	if fourByte {
		asnSize = 4
	}

	offset := 0
	for offset < len(data) {
		// Need at least type + count
		if offset+2 > len(data) {
			return nil, ErrShortData
		}

		segType := ASPathSegmentType(data[offset])
		count := int(data[offset+1])
		offset += 2

		// Check we have enough data for ASNs
		needed := count * asnSize
		if offset+needed > len(data) {
			return nil, ErrShortData
		}

		asns := make([]uint32, count)
		for i := 0; i < count; i++ {
			if fourByte {
				asns[i] = binary.BigEndian.Uint32(data[offset:])
				offset += 4
			} else {
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
