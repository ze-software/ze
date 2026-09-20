// Design: docs/architecture/wire/attributes.md -- ATTR_TOMBSTONE wire marker
// RFC: rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt -- in-place attribute discard marker

package wireu

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// Tombstone reason codes per draft-mangin-idr-attr-tombstone-00 Section 4.4.
const (
	TombstoneUnspecified   byte = 0
	TombstoneEBGPInvalid   byte = 1
	TombstoneInvalidLength byte = 2
	TombstoneMalformedVal  byte = 3
	TombstoneLocalPolicy   byte = 4
)

// TombstoneMinValueLen is the smallest ATTR_TOMBSTONE value, one (code, reason)
// pair, per draft-mangin-idr-attr-tombstone-00 Section 4.3. It is also the
// smallest attribute WriteTombstone can mark IN PLACE, because the marker there
// inherits the discarded attribute's own length.
const TombstoneMinValueLen = 2

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
//
// The zero return is a GUARD, and it says one thing only: the IN-PLACE form does
// not fit, because a value shorter than TombstoneMinValueLen cannot hold the
// (code, reason) pair. It does not say the discard may be skipped, and a caller
// MUST NOT read it as leave to forward the attribute. RFC 7606 Section 2 makes
// the discard unconditional, so the caller answers a zero by rebuilding the
// marker at its own fixed size (draft-mangin-idr-attr-tombstone-00 Section 5.1),
// which is what aspath_transcode.go does.
func WriteTombstone(dst []byte, n int, origFlags byte, origCode attribute.AttributeCode, hdrLen, valueLen int, reason byte) int {
	if valueLen < TombstoneMinValueLen {
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
