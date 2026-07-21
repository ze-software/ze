package wireu

import (
	"encoding/binary"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findAttrInPayload returns (flags, code, value) for the first attribute with
// the given code in a rewritten UPDATE payload, plus whether it was found.
func findAttrInPayload(t *testing.T, payload []byte, want attribute.AttributeCode) (byte, []byte, bool) {
	t.Helper()
	require.GreaterOrEqual(t, len(payload), 4, "payload too short")

	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	require.GreaterOrEqual(t, len(payload), attrLenOff+2, "payload too short for attrLen")
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2
	require.GreaterOrEqual(t, len(payload), attrsStart+attrLen, "payload too short for attrs")

	off := attrsStart
	for off < attrsStart+attrLen {
		flags := attribute.AttributeFlags(payload[off])
		code := attribute.AttributeCode(payload[off+1])
		var length, hl int
		if flags.IsExtLength() {
			length = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hl = 4
		} else {
			length = int(payload[off+2])
			hl = 3
		}
		if code == want {
			return payload[off], payload[off+hl : off+hl+length], true
		}
		off += hl + length
	}
	return 0, nil, false
}

// buildTombstoneAttr constructs an ATTR_TOMBSTONE attribute as a receiving
// speaker would stamp it in place over a discarded LOCAL_PREF.
//
// draft-mangin-idr-attr-tombstone-00 Section 4.3 Example 3: LOCAL_PREF (code 5,
// well-known transitive, flags 0x40, value length 4) received from an EBGP peer
// is discarded per RFC 7606 Section 7.5. The marker flags are
// 0x80 | (0x40 & 0x50) = 0xC0, the length field is inherited (4), value[0] is the
// original code (5), value[1] is the reason (1 = EBGP invalid), tail zeroed.
func buildTombstoneAttr(code attribute.AttributeCode, flags byte) []byte {
	return []byte{flags, byte(code), 0x04, byte(attribute.AttrLocalPref), TombstoneEBGPInvalid, 0x00, 0x00}
}

// TestRewriteASPath_ClearsTombstoneTransitiveAtEBGPBoundary asserts the
// forwarding-policy MUST at the EBGP boundary.
//
// VALIDATES: draft-mangin-idr-attr-tombstone-00 Section 5.3, "inherit" (default)
// policy: "At the originating AS's EBGP boundary, the sending speaker controls
// propagation. Under the "inherit" policy, a recognizing EBGP speaker MUST clear
// the Transitive bit before forwarding the marker to the EBGP peer. This prevents
// the peer from propagating the marker further."
//
// Expected flags derived from the draft, not from observed behavior:
//   - Received/IBGP-facing marker: 0x80 | (0x40 & 0x50) = 0xC0 (Section 4.2, Example 3).
//   - EBGP-facing marker: 0xC0 with the Transitive bit (0x40) cleared = 0x80 (Section 5.3).
//
// PREVENTS: a transitive marker escaping the AS boundary, where the EBGP peer
// would propagate it further per RFC 4271 Section 5.
func TestRewriteASPath_ClearsTombstoneTransitiveAtEBGPBoundary(t *testing.T) {
	// The tombstone code point is unified onto attribute.AttrTombstone (252); the retired
	// legacy code 253 is no longer recognized as a tombstone. That the egress leaves a 253
	// attribute's Transitive bit untouched is asserted by TestTombstoneCodePointIsUnified.
	for _, tc := range []struct {
		name string
		code attribute.AttributeCode
	}{
		{"AttrTombstone", attribute.AttrTombstone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marker := buildTombstoneAttr(tc.code, 0xC0)
			attrs := concatAttrs(
				buildOriginAttr(),
				buildASPathAttr([]attribute.ASPathSegment{}, true),
				marker,
			)
			payload := buildPayload(nil, attrs, []byte{0x18, 0x0A, 0x00, 0x00})

			dst := make([]byte, 512)
			n, err := RewriteASPath(dst, payload, 65000, true, true)
			require.NoError(t, err)

			flags, value, found := findAttrInPayload(t, dst[:n], tc.code)
			require.True(t, found, "tombstone marker must survive EBGP forwarding")

			assert.Equal(t, byte(0x80), flags,
				"Section 5.3: Transitive bit MUST be cleared before forwarding to an EBGP peer")
			assert.Equal(t, byte(0x80), flags&0x80, "Section 4.2: Optional bit MUST remain set")
			assert.Zero(t, flags&0x20, "Section 4.2: Partial bit stays clear")

			// The marker's payload is diagnostic content and MUST be forwarded intact.
			require.Len(t, value, 4, "Section 5.1 step 7: length field MUST NOT be modified")
			assert.Equal(t, byte(attribute.AttrLocalPref), value[0], "value[0]: original code preserved")
			assert.Equal(t, TombstoneEBGPInvalid, value[1], "value[1]: reason preserved")
			assert.Equal(t, []byte{0x00, 0x00}, value[2:], "tail stays zeroed")

			// The received buffer MUST NOT be touched: it is shared zero-copy with
			// IBGP peers, which per Section 5.3 still see the transitive marker.
			assert.Equal(t, byte(0xC0), marker[0],
				"Section 5.3: the IBGP-facing received wire keeps the transitive marker")
		})
	}
}

// TestRewriteASPath_NonTransitiveTombstoneUnchanged asserts an already
// non-transitive marker is forwarded byte-identical.
//
// VALIDATES: draft-mangin-idr-attr-tombstone-00 Section 5.3: clearing an
// already-clear Transitive bit is a no-op. Section 4.2: the Optional bit MUST
// stay set and the Extended Length bit MUST keep matching the length encoding.
// PREVENTS: the clear corrupting unrelated flag bits.
func TestRewriteASPath_NonTransitiveTombstoneUnchanged(t *testing.T) {
	marker := buildTombstoneAttr(attribute.AttrTombstone, 0x80)
	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{}, true),
		marker,
	)
	payload := buildPayload(nil, attrs, []byte{0x18, 0x0A, 0x00, 0x00})

	dst := make([]byte, 512)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)

	flags, value, found := findAttrInPayload(t, dst[:n], attribute.AttrTombstone)
	require.True(t, found)
	assert.Equal(t, byte(0x80), flags, "non-transitive marker forwarded unchanged")
	assert.Equal(t, byte(attribute.AttrLocalPref), value[0], "value[0]: original code preserved")
}

// TestRewriteASPath_ClearsTombstoneTransitiveExtendedLength asserts the clear
// preserves the Extended Length bit and the 2-byte length field.
//
// VALIDATES: draft-mangin-idr-attr-tombstone-00 Section 4.2: "Extended Length bit
// (bit 3): MUST match the length field encoding (set if length > 255)", together
// with the Section 5.3 Transitive clear. 0xD0 (Optional|Transitive|ExtLength)
// must become 0x90 (Optional|ExtLength), not 0x80.
// PREVENTS: clearing the wrong bit and desynchronising flags from the header size.
func TestRewriteASPath_ClearsTombstoneTransitiveExtendedLength(t *testing.T) {
	// Extended-length marker: flags 0xD0, code AttrTombstone, 2-byte length 256.
	marker := make([]byte, 4+256)
	marker[0] = 0xD0
	marker[1] = byte(attribute.AttrTombstone)
	binary.BigEndian.PutUint16(marker[2:4], 256)
	marker[4] = byte(attribute.AttrAggregator)
	marker[5] = TombstoneInvalidLength

	attrs := concatAttrs(
		buildOriginAttr(),
		buildASPathAttr([]attribute.ASPathSegment{}, true),
		marker,
	)
	payload := buildPayload(nil, attrs, []byte{0x18, 0x0A, 0x00, 0x00})

	dst := make([]byte, 1024)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)

	flags, value, found := findAttrInPayload(t, dst[:n], attribute.AttrTombstone)
	require.True(t, found)
	assert.Equal(t, byte(0x90), flags, "Transitive cleared, Optional and ExtLength preserved")
	require.Len(t, value, 256, "extended length value preserved")
	assert.Equal(t, byte(attribute.AttrAggregator), value[0], "original code preserved")
}
