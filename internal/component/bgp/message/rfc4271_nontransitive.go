// Design: docs/architecture/wire/attributes.md -- path attribute encoding
// RFC: rfc/short/rfc4271.md -- Section 5, unrecognized optional attribute handling
// Related: attr_discard.go -- StripAttrRanges and RebuildUpdateBody, the rewrite this feeds
// Related: ../../../core/bgp/attribute/partial.go -- the transitive half of the same sentence

package message

import "github.com/ze-software/ze/internal/core/bgp/attribute"

// UnrecognizedNonTransitiveRanges returns the byte range of every optional
// NON-transitive attribute whose code ze does not implement.
//
// RFC 4271 Section 5 states both halves of the rule in one paragraph, and they
// are opposites: "If a path with an unrecognized transitive optional attribute
// is accepted and passed to other BGP peers, then the unrecognized transitive
// optional attribute of that path MUST be passed, along with the path, to other
// BGP peers with the Partial bit in the Attribute Flags octet set to 1 ...
// Unrecognized non-transitive optional attributes MUST be quietly ignored and
// not passed along to other BGP peers." Section 9 says the same of the second:
// "If an optional non-transitive attribute is unrecognized, it is quietly
// ignored."
//
// attribute.SetPartialOnUnrecognizedTransitive answers the first half in place,
// because stamping a bit changes no length. This half cannot: removing an
// attribute shortens the section, so the caller rewrites the UPDATE body with
// StripAttrRanges and RebuildUpdateBody.
//
// Three classes are left alone:
//
//   - Well-known attributes. Section 5: a speaker MUST recognize all of them, so
//     an unrecognized one is a malformed UPDATE that RFC 7606 has already ruled
//     on, not an attribute to drop quietly.
//   - Optional TRANSITIVE attributes, recognized or not. The first half of the
//     sentence requires those to be passed along.
//   - RECOGNIZED optional non-transitive attributes. The rule is about what ze
//     could not read. A recognized one is ze's own to propagate or not, and
//     MULTI_EXIT_DISC is the example Section 5 itself points at.
//
// Ranges are returned in ascending order and never overlap, which is what
// StripAttrRanges requires of them.
//
// A section that stops parsing returns what it found. This runs after RFC 7606
// validation has accepted the UPDATE, so a section that does not walk cleanly is
// not reachable from the wire; the early return keeps an in-process caller from
// walking off the end.
func UnrecognizedNonTransitiveRanges(pathAttrs []byte) []AttrRange {
	var ranges []AttrRange

	for pos := 0; pos+3 <= len(pathAttrs); {
		start := pos
		flags := attribute.AttributeFlags(pathAttrs[pos])
		code := attribute.AttributeCode(pathAttrs[pos+1])

		hdrLen, valLen := 3, int(pathAttrs[pos+2])
		if flags.IsExtLength() {
			if pos+4 > len(pathAttrs) {
				return ranges
			}
			hdrLen, valLen = 4, int(pathAttrs[pos+2])<<8|int(pathAttrs[pos+3])
		}
		if pos+hdrLen+valLen > len(pathAttrs) {
			return ranges
		}
		pos += hdrLen + valLen

		// Optional AND NOT transitive. The two bits are tested together so no
		// ordering of them can make a partial match pass, the same way the
		// transitive half tests its three.
		optionalNonTransitive := flags&(attribute.FlagOptional|attribute.FlagTransitive) == attribute.FlagOptional
		if optionalNonTransitive && !code.Recognized() {
			ranges = append(ranges, AttrRange{Start: start, End: pos})
		}
	}

	return ranges
}
