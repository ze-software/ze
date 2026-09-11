// Design: docs/architecture/wire/attributes.md — path attribute encoding
// RFC: rfc/short/rfc6793.md — AS4_PATH / AS4_AGGREGATOR (4-byte ASN)

package attribute

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
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

// ToASPath converts AS4Path to a regular ASPath, dropping the path segment
// types an AS4_PATH may not carry.
//
// RFC 6793 Section 4.2.3: Used when reconstructing the AS path from
// AS_PATH and AS4_PATH attributes received from an OLD BGP speaker.
func (p *AS4Path) ToASPath() *ASPath {
	return &ASPath{Segments: appendAS4Segments(nil, p.Segments)}
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
//
// RFC 6793 Section 6: "The AS4_AGGREGATOR attribute in an UPDATE message SHALL be
// considered malformed if the attribute length is not 8." That is the sender's
// obligation as well as the receiver's test, so 8 is a ceiling on this write and
// not only its floor. The address occupies the four octets writeIPv4AddressField
// owns, whatever form the Address holds.
func (a *AS4Aggregator) WriteTo(buf []byte, off int) int {
	binary.BigEndian.PutUint32(buf[off:], a.ASN)
	writeIPv4AddressField(buf, off+4, a.Address)
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
// RFC 6793 Section 4.2.2: "if the AS number is mappable, then the
// AS4_AGGREGATOR attribute MUST NOT be sent."
// Only non-mappable (high 2 octets non-zero) ASNs require AS4_AGGREGATOR.
//
// RFC 6793 Section 9 (IANA): AS_TRANS = 23456.
const ASTrans uint32 = 23456

// isNonMappable reports whether asn cannot be represented in two octets, so a
// two-octet encoding of it has to substitute AS_TRANS.
//
// RFC 6793 Section 4.2.1: a four-octet AS number is mappable only when its two
// high-order octets are zero.
func isNonMappable(asn uint32) bool { return asn > 0xFFFF }

// pathHasNonMappableAS reports whether path carries an AS number above 65535 in
// a segment that is eligible for AS4_PATH.
//
// RFC 6793 Section 4.2.2: "Whenever the AS path information contains the
// AS_CONFED_SEQUENCE or AS_CONFED_SET path segment, the NEW BGP speaker MUST
// exclude such path segments from the AS4_PATH attribute being constructed."
//
// Confederation segments are therefore not considered: a non-mappable AS number
// that only ever appears inside one cannot be carried in AS4_PATH, so it must
// not trigger the attribute either. The RFC's own generation algorithm agrees --
// it sets has_non_mappable only in the non-confederation branch (summarized in
// rfc/short/rfc6793.md, "Generating UPDATE to OLD Speaker").
//
// Counting them would also let a confederation-only path produce a zero-length
// AS4_PATH, which RFC 6793 Section 6 declares malformed (the attribute length
// must be at least 6).
func pathHasNonMappableAS(path *ASPath) bool {
	if path == nil {
		return false
	}
	for _, seg := range path.Segments {
		if seg.Type == ASConfedSequence || seg.Type == ASConfedSet {
			continue
		}
		if slices.ContainsFunc(seg.ASNs, isNonMappable) {
			return true
		}
	}
	return false
}

// AS4PathFor returns the AS4_PATH to emit alongside a two-octet AS_PATH carrying
// path, or nil when RFC 6793 does not require (or forbids) one.
//
// This is the one site that answers "is an AS4_PATH owed toward this peer, and
// what goes in it". Every sender asks here, whether it originates the route
// (message.UpdateBuilder) or re-encodes a received one (wireu), so the rule
// cannot drift between them.
//
// RFC 6793 Section 4.1: "The new attributes, AS4_PATH and AS4_AGGREGATOR, MUST
// NOT be carried in an UPDATE message between NEW BGP speakers."
//
// RFC 6793 Section 4.2.2: "The NEW BGP speaker MUST also send the AS path
// information in the AS4_PATH attribute (encoded with four-octet AS numbers),
// except for the case where all of the AS path information is composed of
// mappable four-octet AS numbers only. In this case, the NEW BGP speaker MUST
// NOT send the AS4_PATH attribute."
//
// The returned AS4Path aliases path's segments; AS4Path.Len and AS4Path.WriteTo
// drop confederation segments per RFC 6793 Section 3, so no copy is needed.
func AS4PathFor(path *ASPath, dstASN4 bool) *AS4Path {
	if dstASN4 || !pathHasNonMappableAS(path) {
		return nil
	}
	return &AS4Path{Segments: path.Segments}
}

// AS4AggregatorFor returns the AS4_AGGREGATOR to emit alongside an AGGREGATOR
// whose AS number a two-octet encoding replaced with AS_TRANS, or nil when none
// is owed.
//
// This is the one site that answers that question, for the sender that
// originates the route and for the sender that forwards one alike.
//
// RFC 6793 Section 4.2.2: "if the NEW BGP speaker has to send the AGGREGATOR
// attribute, and if the aggregating Autonomous System's AS number is a
// non-mappable four-octet AS number, then the speaker MUST use the
// AS4_AGGREGATOR attribute and set the AS number field in the existing
// AGGREGATOR attribute to the reserved AS number, AS_TRANS. Note that if the AS
// number is mappable, then the AS4_AGGREGATOR attribute MUST NOT be sent."
//
// RFC 6793 Section 4.1: a NEW speaker receives the four-octet AS number in the
// AGGREGATOR itself, so the companion MUST NOT be sent to one.
func AS4AggregatorFor(asn uint32, address netip.Addr, dstASN4 bool) *AS4Aggregator {
	if dstASN4 || !isNonMappable(asn) {
		return nil
	}
	return &AS4Aggregator{ASN: asn, Address: address}
}

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
//
//	"Note that a valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment
//	 SHALL be prepended if it is either the leading path segment or is
//	 adjacent to a path segment that is prepended."
func MergeAS4Path(asPath *ASPath, as4Path *AS4Path) *ASPath {
	if as4Path == nil || len(as4Path.Segments) == 0 {
		return asPath
	}
	if asPath == nil {
		// No AS_PATH attribute was received, so there is no AS number count to
		// compare against and no leading part to prepend.
		return as4Path.ToASPath()
	}

	asPathLen := countASNs(asPath.Segments)
	as4PathLen := countASNs(as4Path.Segments)

	// RFC 6793 Section 4.2.3: "If the number of AS numbers in the AS_PATH
	// attribute is less than the number of AS numbers in the AS4_PATH
	// attribute, then the AS4_PATH attribute SHALL be ignored, and the AS_PATH
	// attribute SHALL be taken as the AS path information."
	//
	// A received AS_PATH with no segment at all is counted here rather than
	// short-circuited: its count is zero, so any AS4_PATH carrying an AS number
	// is longer and is ignored. The AS4_PATH is peer-supplied and unverifiable,
	// which is why the count decides rather than the presence.
	if asPathLen < as4PathLen {
		return asPath
	}

	// RFC 6793 Section 4.2.3: "the AS path information SHALL be constructed by
	// taking as many AS numbers and path segments as necessary from the leading
	// part of the AS_PATH attribute, and then prepending them to the AS4_PATH
	// attribute so that the AS path information has a number of AS numbers
	// identical to that of the AS_PATH attribute."
	segments := appendLeadingSegments(nil, asPath.Segments, asPathLen-as4PathLen)
	return &ASPath{Segments: appendAS4Segments(segments, as4Path.Segments)}
}

// appendLeadingSegments appends to dst the leading path segments of source that
// carry the first wanted AS numbers, and returns the extended slice.
//
// RFC 6793 Section 4.2.3: "Note that a valid AS_CONFED_SEQUENCE or AS_CONFED_SET
// path segment SHALL be prepended if it is either the leading path segment or is
// adjacent to a path segment that is prepended."
//
// RFC 5065 counts no AS number for a confederation segment, so such a segment
// spends none of the budget and is taken whole. The walk stops at the first
// segment that is not prepended, which is what leaves a confederation segment
// further along the path unreached: it is then adjacent to nothing prepended.
func appendLeadingSegments(dst, source []ASPathSegment, wanted int) []ASPathSegment {
	for _, seg := range source {
		if isConfedSegment(seg.Type) {
			dst = append(dst, seg)
			continue
		}
		if wanted < 1 {
			return dst
		}
		taken, cost := leadingPart(seg, wanted)
		dst = append(dst, taken)
		wanted -= cost
	}
	return dst
}

// leadingPart returns the leading part of seg that fits in wanted AS numbers,
// and the number of AS numbers that part costs. wanted is at least one.
//
// RFC 4271 Section 9.1.2.2: "an AS_SET counts as 1, no matter how many ASes are
// in the set", so a set is taken whole and costs one. Cutting a set would drop
// the aggregated AS numbers it exists to carry and would not shorten the path.
func leadingPart(seg ASPathSegment, wanted int) (taken ASPathSegment, cost int) {
	if len(seg.ASNs) == 0 {
		return seg, 0
	}
	if seg.Type == ASSet {
		return seg, 1
	}
	if len(seg.ASNs) <= wanted {
		return seg, len(seg.ASNs)
	}
	return ASPathSegment{Type: seg.Type, ASNs: seg.ASNs[:wanted]}, wanted
}

// appendAS4Segments appends to dst the path segments of a received AS4_PATH
// that are valid in one, and returns the extended slice.
//
// RFC 6793 Section 6: "the path segment types AS_CONFED_SEQUENCE and
// AS_CONFED_SET [RFC5065] MUST NOT be carried in the AS4_PATH attribute of an
// UPDATE message. A NEW BGP speaker that receives these path segment types in
// the AS4_PATH attribute of an UPDATE message from an OLD BGP speaker MUST
// discard these path segments, adjust the relevant attribute fields
// accordingly, and continue processing the UPDATE message.".
func appendAS4Segments(dst, source []ASPathSegment) []ASPathSegment {
	for _, seg := range source {
		if isConfedSegment(seg.Type) {
			continue
		}
		dst = append(dst, seg)
	}
	return dst
}

// isConfedSegment reports whether t is one of the two confederation path
// segment types RFC 5065 defines.
func isConfedSegment(t ASPathSegmentType) bool {
	return t == ASConfedSequence || t == ASConfedSet
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

// ReceivedASPathFamily carries the four attribute values RFC 6793 Section 4.2.3
// reconciles, exactly as one received UPDATE carried them, beside the AS number
// width the sending session negotiated.
//
// A nil value says the UPDATE carried no such attribute. A non-nil empty value
// is a present attribute of zero length, which the reconciliation judges rather
// than treats as absent.
type ReceivedASPathFamily struct {
	ASPath        []byte
	AS4Path       []byte
	Aggregator    []byte
	AS4Aggregator []byte
	// SourceASN4 is what the SENDING session negotiated, which is what decides
	// whether the sender is a NEW or an OLD BGP speaker (RFC 6793 Section 4.1).
	SourceASN4 bool
}

// ASPathDiscard names one attribute the reconciliation dropped and why.
//
// It is a report for the operator log, never a failure: the canonical AS path
// beside it is always usable. RFC 6793 Section 6 asks for exactly this ("The
// error SHOULD be logged locally for analysis") and the attribute package holds
// no logger, so each caller writes the line under its own subsystem.
type ASPathDiscard struct {
	Code   AttributeCode
	Reason error
}

// CanonicalASPathFamily is the four-octet AS path information and aggregating
// node one received UPDATE reduces to.
//
// ASPath is an AS_PATH attribute value encoded with four-octet AS numbers, and
// Aggregator an AGGREGATOR attribute value carrying a four-octet AS number.
// Either is nil when the UPDATE carried no such attribute. No AS4_PATH and no
// AS4_AGGREGATOR survive: RFC 6793 Section 4.1 forbids carrying either between
// NEW BGP speakers, and everything downstream of the reconciliation is one.
type CanonicalASPathFamily struct {
	ASPath     []byte
	Aggregator []byte
	// Discards is nil whenever nothing was dropped, which is every UPDATE that
	// is well formed. Callers MUST read it and log each entry.
	Discards []ASPathDiscard
}

// errAS4FromNewSpeaker is the reason an AS4_PATH or AS4_AGGREGATOR sent by a
// NEW BGP speaker is dropped rather than used.
var errAS4FromNewSpeaker = errors.New("RFC 6793 Section 4.1: the attribute MUST NOT be sent between NEW BGP speakers")

// errAS4AggregatorNotChosen is the reason an AS4_AGGREGATOR that was not taken
// as the information about the aggregating node is dropped.
var errAS4AggregatorNotChosen = errors.New("RFC 6793 Section 4.2.3: the AS4_AGGREGATOR was not taken as the aggregating node")

// errAS4AggregatorMalformed is the reason an AS4_AGGREGATOR of the wrong length
// is dropped before the Section 4.2.3 choice can read it.
var errAS4AggregatorMalformed = errors.New("RFC 6793 Section 6: the AS4_AGGREGATOR value is not 8 octets")

// as4AggregatorValueLen is the octet count RFC 6793 Section 4.2.2 gives the
// AS4_AGGREGATOR value: a four-octet AS number beside a four-octet address.
const as4AggregatorValueLen = 8

// ErrASPathUnreadable reports an AS_PATH attribute value that cannot be read at
// the width its sender negotiated, so no four-octet form of it exists.
//
// A caller rewriting a payload MUST refuse the UPDATE on this error rather than
// publish one whose AS path is half rewritten.
var ErrASPathUnreadable = errors.New("AS_PATH is unreadable at the negotiated AS number width")

// ReconcileASPathFamily returns the four-octet AS path information and
// aggregating node RFC 6793 Section 4.2.3 constructs from one received UPDATE.
//
// It is the single declaration of the receive-side reconciliation, so the bytes
// ze relays and the routes ze stores are built by one rule rather than two.
// MergeAS4Path beside it owns the construction itself; this function owns which
// attributes that construction is given.
//
// The error is returned only when no canonical AS path exists, which is an
// AS_PATH that does not parse at the width its sender negotiated. Every other
// outcome is a usable CanonicalASPathFamily, and the discards it carries are a
// report for the log rather than a failure (ai/rules/principles.md).
func ReconcileASPathFamily(recv ReceivedASPathFamily) (CanonicalASPathFamily, error) {
	// RFC 6793 Section 4.1: "A NEW BGP speaker that receives the AS4_PATH
	// attribute or the AS4_AGGREGATOR attribute in an UPDATE message from
	// another NEW BGP speaker MUST discard the path attribute and continue
	// processing the UPDATE message."
	//
	// Both attribute values are already four-octet on this branch, so the
	// discard is the whole of the work: nothing is merged and nothing is
	// widened.
	if recv.SourceASN4 {
		out := CanonicalASPathFamily{ASPath: recv.ASPath, Aggregator: recv.Aggregator}
		if recv.AS4Path != nil {
			out.Discards = append(out.Discards, ASPathDiscard{Code: AttrAS4Path, Reason: errAS4FromNewSpeaker})
		}
		if recv.AS4Aggregator != nil {
			out.Discards = append(out.Discards, ASPathDiscard{Code: AttrAS4Aggregator, Reason: errAS4FromNewSpeaker})
		}
		return out, nil
	}

	var out CanonicalASPathFamily

	// RFC 6793 Section 6: "as the AS4_AGGREGATOR is just informational, the
	// 'attribute discard' approach is chosen to handle a malformed
	// AS4_AGGREGATOR attribute." A discarded attribute was never received, so
	// the Section 4.2.3 choice below runs as if the UPDATE had carried the
	// AGGREGATOR alone: the AGGREGATOR is the aggregating node and the AS path
	// information is constructed as in all other cases.
	//
	// The length is checked HERE and not inside selectAggregator, because the
	// value promoted by that choice is written straight back into the payload.
	// A seven-octet AS4_AGGREGATOR promoted into the AGGREGATOR slot is a
	// malformed AGGREGATOR toward every destination at once, which RFC 7606
	// Section 7.7 makes an attribute discard at each of them.
	as4Aggregator := recv.AS4Aggregator
	if as4Aggregator != nil && len(as4Aggregator) != as4AggregatorValueLen {
		out.Discards = append(out.Discards, ASPathDiscard{Code: AttrAS4Aggregator, Reason: errAS4AggregatorMalformed})
		as4Aggregator = nil
	}

	// RFC 6793 Section 4.2.3: "A NEW BGP speaker MUST also be prepared to
	// receive the AS4_AGGREGATOR attribute along with the AGGREGATOR attribute
	// from an OLD BGP speaker."
	aggregator, fromAS4, useAS4Path := selectAggregator(recv.Aggregator, as4Aggregator)

	if aggregator == nil && recv.Aggregator != nil {
		// The AGGREGATOR could not be read, so nothing chose between the pair.
		// It is optional transitive, so it travels on exactly as it arrived
		// (RFC 4271 Section 5.1.7) and nothing here reinterprets it.
		out.Aggregator = recv.Aggregator
	} else {
		out.Aggregator = canonicalAggregator(aggregator)
	}

	// An AS4_AGGREGATOR that was not taken as the aggregating node carries no
	// information forward. RFC 6793 Section 4.1 forbids relaying it to a NEW
	// BGP speaker, and everything downstream of this reconciliation is one, so
	// it is dropped and the drop is reported rather than performed in silence.
	// A malformed one is already reported above, so this reads the normalized
	// value and never reports the same attribute twice.
	if as4Aggregator != nil && !fromAS4 {
		out.Discards = append(out.Discards, ASPathDiscard{Code: AttrAS4Aggregator, Reason: errAS4AggregatorNotChosen})
	}

	// RFC 6793 Section 4.2.3: "the AS4_AGGREGATOR attribute and the AS4_PATH
	// attribute SHALL be ignored, ... and the AS_PATH attribute SHALL be taken
	// as the AS path information." An ignored AS4_PATH reaches the
	// reconstruction as an absent one.
	as4Path := recv.AS4Path
	if !useAS4Path {
		as4Path = nil
	}

	canonical, discards, err := canonicalizeASPath(recv.ASPath, as4Path)
	if err != nil {
		return CanonicalASPathFamily{}, err
	}
	out.ASPath = canonical
	out.Discards = append(out.Discards, discards...)
	return out, nil
}

// selectAggregator returns the attribute value that is the information about
// the aggregating node, whether that value came from the AS4_AGGREGATOR, and
// whether the received AS4_PATH is used. A nil value says nothing could be
// taken as the aggregating node.
//
// RFC 6793 Section 4.2.3: "When both of the attributes are received, if the AS
// number in the AGGREGATOR attribute is not AS_TRANS, then: - the AS4_AGGREGATOR
// attribute and the AS4_PATH attribute SHALL be ignored, - the AGGREGATOR
// attribute SHALL be taken as the information about the aggregating node, and
// - the AS_PATH attribute SHALL be taken as the AS path information."
//
// RFC 6793 Section 4.2.3: "Otherwise, - the AGGREGATOR attribute SHALL be
// ignored, - the AS4_AGGREGATOR attribute SHALL be taken as the information
// about the aggregating node, and - the AS path information would need to be
// constructed, as in all other cases."
//
// The rule is written for the pair, so one attribute arriving without the other
// leaves nothing to choose between: the received AGGREGATOR is the aggregating
// node and the AS4_PATH is used.
//
// AN AS4_AGGREGATOR WITH NO AGGREGATOR BESIDE IT IS DROPPED, AND NO RFC SAYS SO.
// Both rulings above open "When both of the attributes are received", so neither
// reaches this shape, and RFC 6793 Section 6 calls an AS4_AGGREGATOR malformed
// only on its LENGTH: "The AS4_AGGREGATOR attribute in an UPDATE message SHALL
// be considered malformed if the attribute length is not 8." A well-formed lone
// one is therefore undefined rather than malformed, and Ze's answer is a
// DECISION rather than conformance (Thomas, 2026-09-09).
//
// The shape is invalid at its source. RFC 6793 Section 4.2.2 obliges a sender to
// emit the pair: "if the NEW BGP speaker has to send the AGGREGATOR attribute,
// and if the aggregating Autonomous System's AS number is a non-mappable
// four-octet AS number, then the speaker MUST use the AS4_AGGREGATOR attribute
// and set the AS number field in the existing AGGREGATOR attribute to the
// reserved AS number, AS_TRANS." No conformant speaker produces a lone one, so
// the attribute carries an aggregating node that no companion corroborates.
//
// So it is dropped rather than read, and the UPDATE continues: the route is not
// withdrawn and the session is not reset, because nothing about the AS path
// information is in doubt. The drop is reported as an ASPathDiscard so an
// operator sees the attribute go rather than wondering where it went.
//
// The two implementations Ze interoperates with disagree, read from their source
// on 2026-09-09, which is why this is written down rather than assumed. BIRD
// drops it, unconditionally, before its own pairing test (`bgp_unset_attr(attrs,
// pool, BA_AS4_AGGREGATOR)` in `bgp_process_as4_attrs`). FRR keeps it and
// fabricates an AGGREGATOR around it (`bgp_attr_munge_as4_attrs`, under its own
// comment "That is bogus"), copying the AS but not the identifier, and then
// advertises that invented aggregating node downstream. Ze takes BIRD's answer:
// inventing a node from an uncorroborated attribute puts a claim on the wire
// that no speaker made.
//
// It is reached only for an UPDATE from an OLD BGP speaker, because the
// AS4_AGGREGATOR of a NEW one is discarded before the choice arises, so the
// AGGREGATOR under judgement is always the two-octet form.
func selectAggregator(aggregatorValue, as4AggregatorValue []byte) (aggregator []byte, fromAS4, useAS4Path bool) {
	if aggregatorValue == nil || as4AggregatorValue == nil {
		return aggregatorValue, false, true
	}

	isASTrans, ok := aggregatorASIsTrans(aggregatorValue)
	if !ok {
		// RFC 7606 Section 7.7 discards an AGGREGATOR whose length does not
		// match the negotiated AS width, so one that reaches here cannot be
		// read and decides nothing. Guessing its width would answer the choice
		// above on a value nobody parsed.
		return nil, false, true
	}
	if !isASTrans {
		return aggregatorValue, false, false
	}
	return as4AggregatorValue, true, true
}

// aggregatorASIsTrans reports whether the AS number leading an AGGREGATOR
// attribute received from an OLD BGP speaker is AS_TRANS, and whether that AS
// number could be read.
//
// RFC 6793 Section 3: the AGGREGATOR attribute carries a two-octet AS number
// toward an OLD speaker and a four-octet one between NEW speakers, so its width
// follows the negotiated four-octet AS capability. RFC 7606 Section 7.7 rejects
// every other length, which is what makes a disagreeing length unreadable here
// rather than a shorter form to accommodate.
func aggregatorASIsTrans(value []byte) (isASTrans, ok bool) {
	if len(value) != 6 {
		return false, false
	}
	return uint32(binary.BigEndian.Uint16(value[0:2])) == ASTrans, true
}

// canonicalAggregator returns an AGGREGATOR attribute value carrying a
// four-octet AS number, widening the two-octet form an OLD BGP speaker sends.
//
// RFC 6793 Section 3: the AGGREGATOR attribute carries a two-octet AS number
// toward an OLD speaker and a four-octet one between NEW speakers, and RFC 7606
// Section 7.7 rejects every other length. A value of any other length was never
// read, so it travels on unchanged: nothing here can tell which of its octets
// are the AS number.
func canonicalAggregator(value []byte) []byte {
	if len(value) != 6 {
		return value
	}
	out := make([]byte, 8)
	binary.BigEndian.PutUint32(out, uint32(binary.BigEndian.Uint16(value[0:2])))
	copy(out[4:], value[2:6])
	return out
}

// canonicalizeASPath returns AS_PATH value bytes in canonical four-octet
// encoding, and every attribute the construction discarded.
//
// With no AS4_PATH beside it the received AS_PATH is the AS path information,
// widened from the two-octet encoding its sender used. With an AS4_PATH beside
// it the two are merged per RFC 6793 Section 4.2.3.
func canonicalizeASPath(aspathValue, as4pathValue []byte) ([]byte, []ASPathDiscard, error) {
	if aspathValue == nil {
		// The UPDATE carried no AS_PATH attribute, so RFC 6793 Section 4.2.3
		// has no AS number count to compare an AS4_PATH against and no leading
		// part to prepend. The route records no AS path.
		return nil, nil, nil
	}

	widened := expandASPath2to4(aspathValue)
	if widened == nil {
		return nil, nil, fmt.Errorf("%w: %d octets", ErrASPathUnreadable, len(aspathValue))
	}
	if len(as4pathValue) == 0 {
		return widened, nil, nil
	}
	return reconstructASPath(aspathValue, widened, as4pathValue)
}

// reconstructASPath merges a received AS_PATH and AS4_PATH into the AS path
// information, in four-octet encoding. widened is the received AS_PATH already
// expanded to four octets, which is what the two ignore arms answer.
//
// This runs only for an UPDATE that carries an AS4_PATH, which an OLD speaker
// sends and a session between NEW speakers never does. Parsing both attributes
// into segments costs an allocation each; the common path above reaches none of
// it.
func reconstructASPath(aspathValue, widened, as4pathValue []byte) ([]byte, []ASPathDiscard, error) {
	as4Path, err := ParseAS4Path(as4pathValue)
	if err != nil {
		// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed
		// AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST
		// discard the attribute and continue processing the UPDATE message.
		// The error SHOULD be logged locally for analysis."
		//
		//nolint:nilerr // the discard IS the outcome: the UPDATE continues, and the reason is reported for the log
		return widened, []ASPathDiscard{{Code: AttrAS4Path, Reason: err}}, nil
	}

	asPath, err := ParseASPath(aspathValue, false)
	if err != nil {
		// A malformed AS_PATH is judged by the RFC 7606 validators ahead of the
		// reconciliation. Nothing can be merged into it here, so the widened
		// bytes stand as they did before an AS4_PATH was ever consulted.
		//
		//nolint:nilerr // the merge is skipped rather than failed: the widened AS_PATH is a usable answer
		return widened, []ASPathDiscard{{Code: AttrAS4Path, Reason: err}}, nil
	}

	merged := MergeAS4Path(asPath, as4Path)
	out := make([]byte, merged.LenWithASN4(true))
	merged.WriteToWithASN4(out, 0, true)
	return out, nil, nil
}

// expandASPath2to4 converts two-octet encoded AS_PATH segments to the
// four-octet encoding, and answers nil for a value that does not parse.
func expandASPath2to4(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	// Pre-scan to validate and compute output size.
	segments := 0
	totalASNs := 0
	offset := 0
	for offset+2 <= len(data) {
		segments++
		count := int(data[offset+1])
		offset += 2
		needed := count * 2
		if offset+needed > len(data) {
			return nil
		}
		totalASNs += count
		offset += needed
	}
	if offset != len(data) {
		return nil
	}

	out := make([]byte, 0, segments*2+totalASNs*4)
	offset = 0
	for offset+2 <= len(data) {
		segType := data[offset]
		count := int(data[offset+1])
		out = append(out, segType, data[offset+1])
		offset += 2
		for range count {
			asn16 := uint16(data[offset])<<8 | uint16(data[offset+1])
			out = append(out, 0, 0, byte(asn16>>8), byte(asn16))
			offset += 2
		}
	}
	return out
}
