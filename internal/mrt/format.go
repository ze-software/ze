// Design: docs/architecture/mrt.md -- attribute string rendering for display and matching

package mrt

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// FormatASPath renders parsed AS-path segments as a space-separated string.
// AS_SEQUENCE segments produce space-separated ASNs; AS_SET segments produce {asn,asn}.
func FormatASPath(segments []ASPathSegment) string {
	b := textbuf.Get()
	defer b.Release()
	first := true
	for _, seg := range segments {
		if seg.Type == 1 {
			if !first {
				b.Byte(' ')
			}
			first = false
			b.Byte('{')
			for j, asn := range seg.ASNs {
				if j > 0 {
					b.Byte(',')
				}
				b.Uint32(asn)
			}
			b.Byte('}')
		} else {
			for _, asn := range seg.ASNs {
				if !first {
					b.Byte(' ')
				}
				first = false
				b.Uint32(asn)
			}
		}
	}
	return b.String()
}

// MatchCommunityRegex tests whether any community in the attributes matches the
// given regex. Checks standard (type 8), large (type 32), and extended (type 16).
// Returns true on first match.
func MatchCommunityRegex(attrs []PathAttribute, match func(s string) bool) bool {
	b := textbuf.Get()
	defer b.Release()
	// Standard communities (type 8): "high:low"
	if a := FindAttribute(attrs, 8); a != nil && len(a.Value) >= 4 {
		for off := 0; off+4 <= len(a.Value); off += 4 {
			high := binary.BigEndian.Uint16(a.Value[off:])
			low := binary.BigEndian.Uint16(a.Value[off+2:])
			b.Reset().Uint16(high).Byte(':').Uint16(low)
			if match(b.String()) {
				return true
			}
		}
	}
	// Large communities (type 32): "global:local1:local2"
	if a := FindAttribute(attrs, 32); a != nil && len(a.Value) >= 12 {
		for off := 0; off+12 <= len(a.Value); off += 12 {
			g := binary.BigEndian.Uint32(a.Value[off:])
			l1 := binary.BigEndian.Uint32(a.Value[off+4:])
			l2 := binary.BigEndian.Uint32(a.Value[off+8:])
			b.Reset().Uint32(g).Byte(':').Uint32(l1).Byte(':').Uint32(l2)
			if match(b.String()) {
				return true
			}
		}
	}
	// Extended communities (type 16): 2-octet AS specific as "high:low"
	if a := FindAttribute(attrs, 16); a != nil && len(a.Value) >= 8 {
		for off := 0; off+8 <= len(a.Value); off += 8 {
			t := a.Value[off]
			if t == 0x00 || t == 0x40 {
				high := binary.BigEndian.Uint16(a.Value[off+2:])
				low := binary.BigEndian.Uint32(a.Value[off+4:])
				b.Reset().Uint16(high).Byte(':').Uint32(low)
				if match(b.String()) {
					return true
				}
			}
		}
	}
	return false
}
