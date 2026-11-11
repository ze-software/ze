// Design: docs/guide/bmp.md -- fixtures carry the complete selected path.
package bmp

import "github.com/ze-software/ze/internal/core/bgp/attribute"

// locRIBPathAttrs is a complete stored ORIGIN IGP and four-octet AS_PATH.
// A zero-argument call writes the valid empty attribute, not an absent one.
func locRIBPathAttrs(asns ...uint32) []byte {
	path := attribute.ASPath{}
	if len(asns) != 0 {
		path.Segments = []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: asns}}
	}
	attrs := make([]byte, 8+path.Len())
	copy(attrs, []byte{0x40, attrORIGIN, 1, 0})
	// RFC 9069 Section 5.4.1: include the attribute header and four-octet value.
	n := attribute.WriteAttrTo(&path, attrs, 4)
	return attrs[:4+n]
}
