package attribute

import (
	"encoding/binary"
	"net/netip"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wire"
)

// AS4Path represents the AS4_PATH attribute for 4-byte AS number support.
//
// RFC 6793 Section 3:
//
//	"This document defines a new BGP path attribute called AS4_PATH.
//	 This is an optional transitive attribute that contains the AS path
//	 encoded with four-octet AS numbers. The AS4_PATH attribute has the
//	 same semantics and the same encoding as the AS_PATH attribute,
//	 except that it is "optional transitive", and it carries four-octet
//	 AS numbers."
//
// RFC 6793 Section 9 (IANA): AS4_PATH attribute type code = 17
//
// RFC 6793 Section 3:
//
//	"To prevent the possible propagation of Confederation-related path
//	 segments outside of a Confederation, the path segment types
//	 AS_CONFED_SEQUENCE and AS_CONFED_SET [RFC5065] are declared invalid
//	 for the AS4_PATH attribute and MUST NOT be included in the AS4_PATH
//	 attribute of an UPDATE message."
type AS4Path struct {
	Segments []ASPathSegment
}

// Code returns AttrAS4Path.
func (p *AS4Path) Code() AttributeCode { return AttrAS4Path }

// Flags returns FlagOptional | FlagTransitive (AS4_PATH is optional transitive).
//
// RFC 6793 Section 3: AS4_PATH is "optional transitive".
func (p *AS4Path) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns the packed length in bytes (always 4-byte ASN format).
//
// RFC 6793 Section 3: AS4_PATH "carries four-octet AS numbers" (4 bytes each).
// Note: Confed segments are excluded per RFC 6793 Section 3.
func (p *AS4Path) Len() int {
	length := 0
	for _, seg := range p.Segments {
		// RFC 6793 Section 3: confed segments MUST NOT be included
		if seg.Type == ASConfedSequence || seg.Type == ASConfedSet {
			continue
		}
		// type(1) + count(1) + ASNs(4 each)
		length += 2 + len(seg.ASNs)*4
	}
	return length
}

// WriteTo writes the AS4 path (always 4-byte ASN format) into buf at offset.
func (p *AS4Path) WriteTo(buf []byte, off int) int {
	if len(p.Segments) == 0 {
		return 0
	}

	start := off
	for _, seg := range p.Segments {
		// RFC 6793 Section 3: confed segments MUST NOT be included
		if seg.Type == ASConfedSequence || seg.Type == ASConfedSet {
			continue
		}

		buf[off] = byte(seg.Type)
		buf[off+1] = byte(len(seg.ASNs))
		off += 2

		for _, asn := range seg.ASNs {
			binary.BigEndian.PutUint32(buf[off:], asn)
			off += 4
		}
	}
	return off - start
}

// WriteToWithContext writes AS4_PATH - always uses 4-byte ASNs.
func (p *AS4Path) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return p.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (p *AS4Path) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := p.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return p.WriteTo(buf, off), nil
}

// FilterConfedSegments returns a new AS4Path with confederation segments removed.
//
// RFC 6793 Section 4.2.2:
//
//	"Whenever the AS path information contains the AS_CONFED_SEQUENCE or
//	 AS_CONFED_SET path segment, the NEW BGP speaker MUST exclude such
//	 path segments from the AS4_PATH attribute being constructed."
//
// This is useful when receiving AS4_PATH that may contain confed segments
// (allowed per RFC 6793 Section 6 validation) but need to be filtered.
func (p *AS4Path) FilterConfedSegments() *AS4Path {
	if p == nil {
		return nil
	}

	filtered := &AS4Path{}
	for _, seg := range p.Segments {
		if seg.Type != ASConfedSequence && seg.Type != ASConfedSet {
			filtered.Segments = append(filtered.Segments, seg)
		}
	}
	return filtered
}

// PathLength returns the AS path length for BGP path selection.
//
// RFC 6793 Section 4.2.3: "it is necessary to first calculate the number
// of AS numbers in the AS_PATH and AS4_PATH attributes using the method
// specified in Section 9.1.2.2 of [RFC4271].".
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
//
// RFC 6793 Section 6: The AS4_PATH attribute SHALL be considered malformed if:
//   - "the attribute length is not a multiple of two or is too small
//     (i.e., less than 6) for the attribute to carry at least one AS number"
//   - "the path segment length in the attribute is either zero or is
//     inconsistent with the attribute length"
//   - "the path segment type in the attribute is not one of the types
//     defined: AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, and AS_CONFED_SET"
//
// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed AS4_PATH
// attribute in an UPDATE message from an OLD BGP speaker MUST discard the
// attribute and continue processing the UPDATE message.".
func ParseAS4Path(data []byte) (*AS4Path, error) {
	// RFC 6793 Section 6: Empty AS4_PATH is valid (no segments)
	if len(data) == 0 {
		return &AS4Path{}, nil
	}

	// RFC 6793 Section 6: "the attribute length is not a multiple of two"
	// Each segment is: type(1) + count(1) + count*4 bytes = always even
	if len(data)%2 != 0 {
		return nil, ErrInvalidLength
	}

	path := &AS4Path{}
	offset := 0
	for offset < len(data) {
		if offset+2 > len(data) {
			return nil, ErrShortData
		}

		segType := ASPathSegmentType(data[offset])
		count := int(data[offset+1])
		offset += 2

		// RFC 6793 Section 6: "the path segment length in the attribute is either zero"
		if count == 0 {
			return nil, ErrInvalidLength
		}

		// RFC 6793 Section 6: "the path segment type in the attribute is not one of
		// the types defined: AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, and AS_CONFED_SET"
		if !isValidSegmentType(segType) {
			return nil, ErrMalformedValue
		}

		needed := count * 4 // Always 4 bytes per ASN
		if offset+needed > len(data) {
			return nil, ErrShortData
		}

		asns := make([]uint32, count)
		for i := range count {
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

// isValidSegmentType checks if segment type is defined per RFC 4271 and RFC 5065.
// RFC 6793 Section 6: Valid types are AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE, AS_CONFED_SET.
func isValidSegmentType(t ASPathSegmentType) bool {
	switch t {
	case ASSet, ASSequence, ASConfedSequence, ASConfedSet:
		return true
	default:
		return false
	}
}

// ToASPath converts AS4Path to a regular ASPath.
//
// RFC 6793 Section 4.2.3: Used when reconstructing the AS path from
// AS_PATH and AS4_PATH attributes received from an OLD BGP speaker.
func (p *AS4Path) ToASPath() *ASPath {
	return &ASPath{
		Segments: p.Segments,
	}
}

// AS4Aggregator represents the AS4_AGGREGATOR attribute for 4-byte AS support.
//
// RFC 6793 Section 3:
//
//	"This document defines a new BGP path attribute called AS4_AGGREGATOR,
//	 which is optional transitive. The AS4_AGGREGATOR attribute has the
//	 same semantics and the same encoding as the AGGREGATOR attribute,
//	 except that it carries a four-octet AS number."
//
// RFC 6793 Section 9 (IANA): AS4_AGGREGATOR attribute type code = 18
//
// RFC 6793 Section 4.2.2:
//
//	"if the NEW BGP speaker has to send the AGGREGATOR attribute, and if
//	 the aggregating Autonomous System's AS number is a non-mappable
//	 four-octet AS number, then the speaker MUST use the AS4_AGGREGATOR
//	 attribute and set the AS number field in the existing AGGREGATOR
//	 attribute to the reserved AS number, AS_TRANS."
type AS4Aggregator struct {
	ASN     uint32
	Address netip.Addr
}

// Code returns AttrAS4Aggregator.
func (a *AS4Aggregator) Code() AttributeCode { return AttrAS4Aggregator }

// Flags returns FlagOptional | FlagTransitive (AS4_AGGREGATOR is optional transitive).
//
// RFC 6793 Section 3: AS4_AGGREGATOR is "optional transitive".
func (a *AS4Aggregator) Flags() AttributeFlags { return FlagOptional | FlagTransitive }

// Len returns 8 (4-byte AS + 4-byte IPv4 address).
//
// RFC 6793 Section 6: "The AS4_AGGREGATOR attribute in an UPDATE message
// SHALL be considered malformed if the attribute length is not 8.".
func (a *AS4Aggregator) Len() int { return 8 }

// WriteTo writes the AS4_AGGREGATOR into buf at offset.
func (a *AS4Aggregator) WriteTo(buf []byte, off int) int {
	binary.BigEndian.PutUint32(buf[off:], a.ASN)
	copy(buf[off+4:], a.Address.AsSlice())
	return 8
}

// WriteToWithContext writes AS4_AGGREGATOR - always uses 4-byte ASN.
func (a *AS4Aggregator) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return a.WriteTo(buf, off)
}

// CheckedWriteTo validates capacity before writing.
func (a *AS4Aggregator) CheckedWriteTo(buf []byte, off int) (int, error) {
	if len(buf) < off+8 {
		return 0, wire.ErrBufferTooSmall
	}
	return a.WriteTo(buf, off), nil
}

// ParseAS4Aggregator parses an AS4_AGGREGATOR attribute value.
//
// RFC 6793 Section 6: "The AS4_AGGREGATOR attribute in an UPDATE message
// SHALL be considered malformed if the attribute length is not 8."
//
// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed
// AS4_AGGREGATOR attribute in an UPDATE message from an OLD BGP speaker
// MUST discard the attribute and continue processing the UPDATE message.".
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
//
// RFC 6793 Section 4.2.3: When AGGREGATOR contains AS_TRANS, use
// AS4_AGGREGATOR as "the information about the aggregating node".
func (a *AS4Aggregator) ToAggregator() *Aggregator {
	return &Aggregator{
		ASN:     a.ASN,
		Address: a.Address,
	}
}

// ASTrans is the reserved AS number for 4-byte/2-byte AS interoperability.
//
// RFC 6793 Section 3:
//
//	"This document reserves a two-octet AS number called 'AS_TRANS'.
//	 AS_TRANS can be used to represent non-mappable four-octet AS numbers
//	 as two-octet AS numbers in AS path information that is encoded with
//	 two-octet AS numbers."
//
// RFC 6793 Section 9 (IANA): AS_TRANS = 23456.
const ASTrans uint32 = 23456

// MergeAS4Path merges AS_PATH and AS4_PATH per RFC 6793 Section 4.2.3.
//
// RFC 6793 Section 4.2.3:
//
//	"If the number of AS numbers in the AS_PATH attribute is less than the
//	 number of AS numbers in the AS4_PATH attribute, then the AS4_PATH
//	 attribute SHALL be ignored, and the AS_PATH attribute SHALL be taken
//	 as the AS path information."
//
//	"If the number of AS numbers in the AS_PATH attribute is larger than
//	 or equal to the number of AS numbers in the AS4_PATH attribute, then
//	 the AS path information SHALL be constructed by taking as many AS
//	 numbers and path segments as necessary from the leading part of the
//	 AS_PATH attribute, and then prepending them to the AS4_PATH attribute
//	 so that the AS path information has a number of AS numbers identical
//	 to that of the AS_PATH attribute."
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

// countASNs counts AS path length per RFC 4271 Section 9.1.2.2.
//
// RFC 6793 Section 4.2.3: "it is necessary to first calculate the number
// of AS numbers in the AS_PATH and AS4_PATH attributes using the method
// specified in Section 9.1.2.2 of [RFC4271] and in [RFC5065]"
//
// RFC 4271 Section 9.1.2.2: "an AS_SET counts as 1, no matter how many
// ASes are in the set"
//
// RFC 5065: Confederation segments (AS_CONFED_SEQUENCE, AS_CONFED_SET)
// are not counted in path length calculation.
func countASNs(segments []ASPathSegment) int {
	count := 0
	for _, seg := range segments {
		switch seg.Type {
		case ASSequence:
			count += len(seg.ASNs)
		case ASSet:
			// RFC 4271 Section 9.1.2.2: AS_SET counts as 1
			if len(seg.ASNs) > 0 {
				count++
			}
		case ASConfedSequence, ASConfedSet:
			// RFC 5065: Confederation segments don't count
		}
	}
	return count
}
