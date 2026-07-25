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

// as4PathForRewrite returns the AS4_PATH to emit after asns have been prepended
// to the outgoing AS_PATH, or nil when none is required.
//
// prepended is the outgoing AS path with asns already prepended. recvAS4 is the
// AS4_PATH parsed from the source UPDATE, or nil when the source carried none.
//
// When the source is an OLD speaker that supplied its own AS4_PATH, the local
// ASNs are prepended to THAT path rather than derived from AS_PATH: AS_PATH
// holds AS_TRANS placeholders whose real values only exist in the received
// AS4_PATH. Prepending the same ASNs to both attributes preserves the AS number
// count difference that RFC 6793 Section 4.2.3 reconstruction depends on.
//
// A recvAS4 from a NEW speaker is invalid (RFC 6793 Section 4.1) and ignored:
// with srcASN4 true, AS_PATH already carries the real four-octet ASNs.
func as4PathForRewrite(prepended *attribute.ASPath, recvAS4 *attribute.AS4Path, asns []uint32, srcASN4, dstASN4 bool) *attribute.AS4Path {
	if dstASN4 {
		// RFC 6793 Section 4.1: "The new attributes, AS4_PATH and
		// AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between
		// NEW BGP speakers."
		return nil
	}
	if srcASN4 || recvAS4 == nil {
		return as4PathForPath(prepended, dstASN4)
	}

	merged := &attribute.ASPath{Segments: recvAS4.Segments}
	for _, asn := range asns {
		merged.Prepend(asn)
	}
	return as4PathForPath(merged, dstASN4)
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
