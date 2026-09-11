// Design: docs/architecture/wire/attributes.md -- AS4_PATH construction for OLD-speaker peers
// RFC: rfc/short/rfc6793.md -- AS4_PATH (Section 3, Section 4.2.2)
// Related: aspath_slot.go (ASPathEdit, the EBGP egress rail), aspath_transcode.go
// (the narrowing encoder), aspath_collapse.go (the ingest reconciliation)
//
// The forwarding rails' entry to the "does this UPDATE need an AS4_PATH, and
// what goes in it" question. The answer itself is attribute.AS4PathFor, which
// the originating encoders ask too, so the rule cannot drift between what ze
// relays and what ze originates.

package wireu

import (
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// as4PathForPath returns the AS4_PATH to emit alongside a 2-octet AS_PATH
// carrying path, or nil when RFC 6793 does not require (or forbids) one.
//
// The rule itself is attribute.AS4PathFor, which the originating encoders in
// internal/component/bgp/message ask as well. This name stays because the three
// wireu rails read better against it, and because a rule stated twice is a
// future disagreement with nothing to arbitrate it (ai/rules/principles.md).
func as4PathForPath(path *attribute.ASPath, dstASN4 bool) *attribute.AS4Path {
	return attribute.AS4PathFor(path, dstASN4)
}

// AS4PathForRewrite returns the AS4_PATH to emit alongside an outgoing AS_PATH
// that the local AS numbers have already been prepended to, or nil when none is
// required.
//
// prepended is that outgoing AS path, carrying the real four-octet AS numbers.
// recvAS4 is the AS4_PATH parsed from the source UPDATE, or nil when the source
// carried none.
//
// No FORWARDING RAIL reaches the merged branch below. The ingest
// collapse reconciles the AS-path family once per received UPDATE
// (aspath_collapse.go, CollapseAS4Family) and relabels the payload's encoding
// context, so every rail passes srcASN4 true and every rail takes the first
// branch. The branch stays because ONE caller still reaches it, and that caller
// is not a rail: the reactor's policy prepend, prependAS4PathValue through
// ExtractASPathPrependOps (filter_delta.go), which edits ONE payload rather
// than transcoding between two and so passes the same width for srcASN4 and
// dstASN4. At two octets with an AS4_PATH on the body it holds, srcASN4 is
// false and recvAS4 is not nil, which is the merge.
//
// What the merge emits, when it runs, is the WHOLE path rather than recvAS4
// with the local AS numbers on top. RFC 6793 Section 4.2.3: "the AS path
// information SHALL be constructed by taking as many AS numbers and path
// segments as necessary from the leading part of the AS_PATH attribute, and
// then prepending them to the AS4_PATH attribute so that the AS path
// information has a number of AS numbers identical to that of the AS_PATH
// attribute." Prepending to both attributes keeps that leading count unchanged,
// so the receiver takes it from the head of the AS_PATH ze just wrote, where
// the AS_TRANS placeholders now sit: the local AS reconstructs as AS_TRANS and
// one received AS number per prepended copy is lost. attribute.MergeAS4Path is
// ze's declaration of the receiver's rule, so running the path through it here
// emits what the receiver must arrive at.
//
// A recvAS4 from a NEW speaker is invalid (RFC 6793 Section 4.1) and ignored:
// with srcASN4 true, AS_PATH already carries the real four-octet ASNs.
//
// The answer is nil for every case that owes no AS4_PATH, which is what lets the
// policy-prepend caller carry no condition of its own (ai/rules/principles.md).
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
