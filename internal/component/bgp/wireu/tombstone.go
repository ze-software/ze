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

// clearTombstoneTransitiveInBody clears the Transitive bit on every
// ATTR_TOMBSTONE marker in a freshly built UPDATE body.
//
// draft-mangin-idr-attr-tombstone-00 Section 5.3: "At the originating AS's EBGP
// boundary, the sending speaker controls propagation. Under the "inherit"
// policy, a recognizing EBGP speaker MUST clear the Transitive bit before
// forwarding the marker to the EBGP peer. This prevents the peer from
// propagating the marker further."
//
// It exists because that MUST used to ride on ONE of the three prepend paths.
// Only the re-encoding slow path cleared the bit; the byte-shifting fast path
// and the insert path copied the marker through with its Transitive bit intact,
// so whether a peer could propagate the marker onward depended on which prepend
// path the route happened to take. This walks the body the caller just wrote, so
// every path clears it the same way.
//
// dst must be a complete UPDATE body the caller has finished writing, and n its
// length. A body that does not parse is left alone: it is the caller's own
// output, and a malformed one is a caller bug rather than something to repair
// here.
func clearTombstoneTransitiveInBody(dst []byte, n int) {
	if n < 4 {
		return
	}
	wdLen := int(binary.BigEndian.Uint16(dst[0:2]))
	attrLenOff := 2 + wdLen
	if attrLenOff+2 > n {
		return
	}
	attrLen := int(binary.BigEndian.Uint16(dst[attrLenOff : attrLenOff+2]))
	off := attrLenOff + 2
	end := off + attrLen
	if end > n {
		return
	}
	for off < end {
		if off+3 > end {
			return
		}
		flags := attribute.AttributeFlags(dst[off])
		code := attribute.AttributeCode(dst[off+1])
		length := int(dst[off+2])
		hdrLen := 3
		if flags.IsExtLength() {
			if off+4 > end {
				return
			}
			length = int(binary.BigEndian.Uint16(dst[off+2 : off+4]))
			hdrLen = 4
		}
		if off+hdrLen+length > end {
			return
		}
		if code == attribute.AttrTombstone {
			clearTombstoneTransitive(dst, off)
		}
		off += hdrLen + length
	}
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
