// Design: docs/architecture/wire/attributes.md -- AS4_PATH construction for OLD-speaker peers
// RFC: rfc/short/rfc6793.md -- AS4_PATH (Section 3, Section 4.2.2)
// Related: aspath_rewrite.go (prepend + transcode), aspath_transcode.go (transcode only)
//
// Single owner of the "does this UPDATE need an AS4_PATH, and what goes in it"
// question. Both wireu egress paths (RewriteASPath and TranscodeASPath) route
// through here so the rule cannot drift between them.

package wireu

import (
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// hasNonMappableASN reports whether the path carries an ASN above 65535 in a
// segment that is eligible for AS4_PATH.
//
// RFC 6793 Section 4.2.2: "Whenever the AS path information contains the
// AS_CONFED_SEQUENCE or AS_CONFED_SET path segment, the NEW BGP speaker MUST
// exclude such path segments from the AS4_PATH attribute being constructed."
//
// Confederation segments are therefore not considered: a non-mappable ASN that
// only ever appears inside one cannot be carried in AS4_PATH, so it must not
// trigger the attribute either. The RFC's own generation algorithm agrees --
// it sets has_non_mappable only in the non-confederation branch (summarized in
// rfc/short/rfc6793.md, "Generating UPDATE to OLD Speaker").
//
// Counting them would also let a confederation-only path produce a zero-length
// AS4_PATH, which RFC 6793 Section 6 declares malformed (the attribute length
// must be at least 6).
func hasNonMappableASN(p *attribute.ASPath) bool {
	for _, seg := range p.Segments {
		if seg.Type == attribute.ASConfedSequence || seg.Type == attribute.ASConfedSet {
			continue
		}
		for _, asn := range seg.ASNs {
			if asn > 65535 {
				return true
			}
		}
	}
	return false
}

// as4PathForPath returns the AS4_PATH to emit alongside a 2-octet AS_PATH
// carrying path, or nil when RFC 6793 does not require (or forbids) one.
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
func as4PathForPath(path *attribute.ASPath, dstASN4 bool) *attribute.AS4Path {
	if dstASN4 || path == nil || !hasNonMappableASN(path) {
		return nil
	}
	return &attribute.AS4Path{Segments: path.Segments}
}

// AS4PathForRewrite returns the AS4_PATH to emit alongside an outgoing AS_PATH
// that the local AS numbers have already been prepended to, or nil when none is
// required.
//
// prepended is that outgoing AS path, carrying the real four-octet AS numbers.
// recvAS4 is the AS4_PATH parsed from the source UPDATE, or nil when the source
// carried none.
//
// When the source is an OLD speaker that supplied its own AS4_PATH, the emitted
// AS4_PATH carries the WHOLE path rather than recvAS4 with the local AS numbers
// on top. RFC 6793 Section 4.2.3: "the AS path information SHALL be constructed
// by taking as many AS numbers and path segments as necessary from the leading
// part of the AS_PATH attribute, and then prepending them to the AS4_PATH
// attribute so that the AS path information has a number of AS numbers
// identical to that of the AS_PATH attribute." Prepending to both attributes
// keeps that leading count unchanged, so the receiver takes it from the head of
// the AS_PATH ze just wrote, where the AS_TRANS placeholders now sit: the local
// AS reconstructs as AS_TRANS and one received AS number per prepended copy is
// lost. attribute.MergeAS4Path is ze's declaration of the receiver's rule, so
// running the path through it here emits what the receiver must arrive at.
//
// A recvAS4 from a NEW speaker is invalid (RFC 6793 Section 4.1) and ignored:
// with srcASN4 true, AS_PATH already carries the real four-octet ASNs.
//
// Exported because the reactor's policy prepend (ExtractASPathPrependOps,
// filter_delta.go) asks the same question about the same attribute family. It
// edits ONE payload rather than transcoding between two, so it passes the same
// width for srcASN4 and dstASN4; the answer is then nil for every case that
// needs no AS4_PATH, which is what lets that caller carry no condition of its
// own (ai/rules/principles.md).
func AS4PathForRewrite(prepended *attribute.ASPath, recvAS4 *attribute.AS4Path, srcASN4, dstASN4 bool) *attribute.AS4Path {
	if dstASN4 {
		// RFC 6793 Section 4.1: "The new attributes, AS4_PATH and
		// AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between
		// NEW BGP speakers."
		return nil
	}
	if srcASN4 || recvAS4 == nil {
		return as4PathForPath(prepended, dstASN4)
	}
	return as4PathForPath(joinSequences(attribute.MergeAS4Path(prepended, recvAS4)), dstASN4)
}

// joinSequences answers path with each run of adjacent AS_SEQUENCE segments
// written as one segment, up to the 255 AS numbers a segment can hold.
//
// The reconstruction the caller runs cuts the path where the received AS4_PATH
// began, so a path that arrived as one sequence leaves it as two. The AS number
// order is identical either way and RFC 4271 Section 4.3 counts both the same,
// so this changes no meaning. It keeps the bytes ze emits for a path that needs
// no cut identical to the bytes ze emitted before the cut existed.
func joinSequences(path *attribute.ASPath) *attribute.ASPath {
	if path == nil {
		return nil
	}
	joined := make([]attribute.ASPathSegment, 0, len(path.Segments))
	for _, seg := range path.Segments {
		last := len(joined) - 1
		if seg.Type != attribute.ASSequence || last < 0 ||
			joined[last].Type != attribute.ASSequence ||
			len(joined[last].ASNs)+len(seg.ASNs) > attribute.MaxASPathSegmentLength {
			joined = append(joined, seg)
			continue
		}
		// The run is copied rather than appended in place: a leading segment
		// MergeAS4Path cut carries the source's spare capacity, so appending to
		// it would write over AS numbers the source still owns.
		run := make([]uint32, 0, len(joined[last].ASNs)+len(seg.ASNs))
		run = append(run, joined[last].ASNs...)
		run = append(run, seg.ASNs...)
		joined[last].ASNs = run
	}
	return &attribute.ASPath{Segments: joined}
}

// as4PathWireSize returns the full wire size (header + value) of the AS4_PATH
// attribute, or 0 when p is nil.
func as4PathWireSize(p *attribute.AS4Path) int {
	if p == nil {
		return 0
	}
	valueLen := p.Len()
	if valueLen > 255 {
		return 4 + valueLen
	}
	return 3 + valueLen
}

// writeAS4PathAttr writes the AS4_PATH attribute (header + value) into dst at
// off and returns the number of bytes written, or 0 when p is nil.
//
// RFC 6793 Section 3: AS4_PATH is an optional transitive attribute, type 17.
func writeAS4PathAttr(dst []byte, off int, p *attribute.AS4Path) int {
	if p == nil {
		return 0
	}
	n := attribute.WriteHeaderTo(dst, off,
		attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Path, uint16(p.Len())) //nolint:gosec // bounded by BGP max
	n += p.WriteTo(dst, off+n)
	return n
}
