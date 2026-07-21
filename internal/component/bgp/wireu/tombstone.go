// Design: docs/architecture/wire/attributes.md -- ATTR_TOMBSTONE wire marker
// RFC: rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt -- in-place attribute discard marker

package wireu

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
)

// Tombstone reason codes per draft-mangin-idr-attr-tombstone-00 Section 4.4.
const (
	TombstoneUnspecified   byte = 0
	TombstoneEBGPInvalid   byte = 1
	TombstoneInvalidLength byte = 2
	TombstoneMalformedVal  byte = 3
	TombstoneLocalPolicy   byte = 4
)

// clearTombstoneTransitive clears the Transitive bit of an ATTR_TOMBSTONE marker
// whose flags byte sits at dst[flagsOff]. Zero-copy: masks one byte already
// written into the caller's buffer, no allocation and no data movement.
//
// draft-mangin-idr-attr-tombstone-00 Section 5.3 ("inherit", the default policy):
// "At the originating AS's EBGP boundary, the sending speaker controls
// propagation.  Under the "inherit" policy, a recognizing EBGP speaker MUST clear
// the Transitive bit before forwarding the marker to the EBGP peer.  This
// prevents the peer from propagating the marker further."
//
// Only the Transitive bit is touched: Section 4.2 requires the Optional bit to
// stay set and the Extended Length bit to keep matching the length encoding.
//
// The caller MUST only invoke this on a buffer destined for a true EBGP peer.
// Section 5.5 keeps confederation boundaries out of scope: markers there are
// "handled according to their Transitive bit, per standard RFC 5065 processing",
// so a confederation member-AS boundary is not an AS boundary for this rule.
func clearTombstoneTransitive(dst []byte, flagsOff int) {
	dst[flagsOff] &^= byte(attribute.FlagTransitive)
}

// WriteTombstone writes an ATTR_TOMBSTONE marker into dst at offset n,
// replacing a malformed or policy-discarded attribute. The marker occupies
// exactly the same wire space as the original attribute (no data movement).
//
// draft-mangin-idr-attr-tombstone-00 Section 5.1:
//   - Flags: 0x80 | (original_flags & 0x50) -- Optional, preserve Transitive + ExtLength
//   - Code: AttrTombstone (252)
//   - Length: unchanged
//   - Value[0]: original attribute type code
//   - Value[1]: reason code
//   - Value[2..]: zeroed
//
// Returns the number of bytes written (always hdrLen + valueLen).
// If valueLen < 2, the (code, reason) pair cannot fit; returns 0 to signal
// the caller should fall back to copy-verbatim.
func WriteTombstone(dst []byte, n int, origFlags byte, origCode attribute.AttributeCode, hdrLen, valueLen int, reason byte) int {
	if valueLen < 2 {
		return 0
	}

	// draft-mangin-idr-attr-tombstone-00 Section 4.2:
	// new_flags = 0x80 | (original_flags & 0x50)
	dst[n] = 0x80 | (origFlags & 0x50)
	dst[n+1] = byte(attribute.AttrTombstone)

	if hdrLen == 4 {
		binary.BigEndian.PutUint16(dst[n+2:], uint16(valueLen)) //nolint:gosec // bounded by BGP max
	} else {
		dst[n+2] = byte(valueLen) //nolint:gosec // bounded by BGP max
	}

	valStart := n + hdrLen
	dst[valStart] = byte(origCode)
	dst[valStart+1] = reason

	for i := valStart + 2; i < valStart+valueLen; i++ {
		dst[i] = 0
	}

	return hdrLen + valueLen
}
