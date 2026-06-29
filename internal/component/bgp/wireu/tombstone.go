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
