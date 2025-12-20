package attribute

import (
	"encoding/binary"
	"net/netip"
)

// AS4Path represents the AS4_PATH attribute (RFC 6793).
//
// This attribute is used for backward compatibility when a BGP speaker
// that supports 4-byte AS numbers communicates with one that doesn't.
// It carries the actual 4-byte AS path when AS_PATH contains AS_TRANS.
type AS4Path struct {
	Segments []ASPathSegment
}

// Code returns AttrAS4Path.
func (p *AS4Path) Code() AttributeCode { return AttrAS4Path }

// Flags returns FlagOptional | FlagTransitive (AS4_PATH is optional transitive).
func (p *AS4Path) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the packed length in bytes (always 4-byte ASN format).
func (p *AS4Path) Len() int {
	length := 0
	for _, seg := range p.Segments {
		// type(1) + count(1) + ASNs(4 each)
		length += 2 + len(seg.ASNs)*4
	}
	return length
}

// Pack serializes the AS4 path (always 4-byte ASN format).
func (p *AS4Path) Pack() []byte {
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
func (p *AS4Path) PathLength() int {
	length := 0
	for _, seg := range p.Segments {
		switch seg.Type {
		case ASSequence:
			length += len(seg.ASNs)
		case ASSet:
			if len(seg.ASNs) > 0 {
				length++
			}
		case ASConfedSequence, ASConfedSet:
			// Confederation segments don't count
		}
	}
	return length
}

// ParseAS4Path parses an AS4_PATH attribute value.
// AS4_PATH always uses 4-byte AS numbers.
func ParseAS4Path(data []byte) (*AS4Path, error) {
	path := &AS4Path{}

	offset := 0
	for offset < len(data) {
		if offset+2 > len(data) {
			return nil, ErrShortData
		}

		segType := ASPathSegmentType(data[offset])
		count := int(data[offset+1])
		offset += 2

		needed := count * 4 // Always 4 bytes per ASN
		if offset+needed > len(data) {
			return nil, ErrShortData
		}

		asns := make([]uint32, count)
		for i := 0; i < count; i++ {
			asns[i] = binary.BigEndian.Uint32(data[offset:])
			offset += 4
		}

		path.Segments = append(path.Segments, ASPathSegment{
			Type: segType,
			ASNs: asns,
		})
	}

	return path, nil
}

// ToASPath converts AS4Path to a regular ASPath.
// Used when merging AS4_PATH with AS_PATH per RFC 6793.
func (p *AS4Path) ToASPath() *ASPath {
	return &ASPath{
		Segments: p.Segments,
	}
}

// AS4Aggregator represents the AS4_AGGREGATOR attribute (RFC 6793).
//
// This attribute is used for backward compatibility when AGGREGATOR
// contains AS_TRANS (23456). It carries the actual 4-byte aggregator AS.
type AS4Aggregator struct {
	ASN     uint32
	Address netip.Addr
}

// Code returns AttrAS4Aggregator.
func (a *AS4Aggregator) Code() AttributeCode { return AttrAS4Aggregator }

// Flags returns FlagOptional | FlagTransitive (AS4_AGGREGATOR is optional transitive).
func (a *AS4Aggregator) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns 8 (4-byte AS + 4-byte IPv4 address).
func (a *AS4Aggregator) Len() int { return 8 }

// Pack serializes the AS4_AGGREGATOR attribute.
func (a *AS4Aggregator) Pack() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], a.ASN)
	copy(buf[4:8], a.Address.AsSlice())
	return buf
}

// ParseAS4Aggregator parses an AS4_AGGREGATOR attribute value.
func ParseAS4Aggregator(data []byte) (*AS4Aggregator, error) {
	if len(data) != 8 {
		return nil, ErrInvalidLength
	}

	addr, ok := netip.AddrFromSlice(data[4:8])
	if !ok {
		return nil, ErrMalformedValue
	}

	return &AS4Aggregator{
		ASN:     binary.BigEndian.Uint32(data[0:4]),
		Address: addr,
	}, nil
}

// ToAggregator converts AS4Aggregator to a regular Aggregator.
func (a *AS4Aggregator) ToAggregator() *Aggregator {
	return &Aggregator{
		ASN:     a.ASN,
		Address: a.Address,
	}
}

// ASTrans is the special AS number used for 4-byte/2-byte interop.
const ASTrans uint32 = 23456

// MergeAS4Path merges AS_PATH and AS4_PATH per RFC 6793.
//
// If the AS_PATH contains AS_TRANS, it replaces those entries with
// the corresponding entries from AS4_PATH. This reconstructs the
// original 4-byte AS path from a speaker that went through 2-byte
// AS speakers.
func MergeAS4Path(asPath *ASPath, as4Path *AS4Path) *ASPath {
	if as4Path == nil || len(as4Path.Segments) == 0 {
		return asPath
	}
	if asPath == nil || len(asPath.Segments) == 0 {
		return as4Path.ToASPath()
	}

	// Count ASNs in each path
	asPathLen := countASNs(asPath.Segments)
	as4PathLen := countASNs(as4Path.Segments)

	// If AS4_PATH is longer, something is wrong - use AS_PATH
	if as4PathLen > asPathLen {
		return asPath
	}

	// Create merged path
	// Take the first (asPathLen - as4PathLen) ASNs from AS_PATH,
	// then append all of AS4_PATH
	skip := asPathLen - as4PathLen
	merged := &ASPath{}

	remaining := skip
	for _, seg := range asPath.Segments {
		if remaining <= 0 {
			break
		}

		if len(seg.ASNs) <= remaining {
			merged.Segments = append(merged.Segments, seg)
			remaining -= len(seg.ASNs)
		} else {
			// Partial segment
			merged.Segments = append(merged.Segments, ASPathSegment{
				Type: seg.Type,
				ASNs: seg.ASNs[:remaining],
			})
			remaining = 0
		}
	}

	// Append AS4_PATH segments
	merged.Segments = append(merged.Segments, as4Path.Segments...)

	return merged
}

// countASNs counts total ASNs in path segments.
func countASNs(segments []ASPathSegment) int {
	count := 0
	for _, seg := range segments {
		count += len(seg.ASNs)
	}
	return count
}
