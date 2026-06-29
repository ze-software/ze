package wireu

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteTombstone_Basic verifies the in-place overwrite of a transitive
// attribute with a tombstone marker.
//
// VALIDATES: draft-mangin-idr-attr-tombstone-00 Section 5.1 steps 2-7.
// PREVENTS: Wrong flags, missing original code, missing reason, non-zeroed tail.
func TestWriteTombstone_Basic(t *testing.T) {
	dst := make([]byte, 32)
	// Simulate AGGREGATOR: flags=0xC0 (optional transitive), code=7, hdrLen=3, valueLen=5
	n := WriteTombstone(dst, 0, 0xC0, attribute.AttrAggregator, 3, 5, TombstoneInvalidLength)
	require.Equal(t, 8, n, "should write hdrLen(3) + valueLen(5) = 8 bytes")

	// Flags: 0x80 | (0xC0 & 0x50) = 0x80 | 0x40 = 0xC0
	assert.Equal(t, byte(0xC0), dst[0], "flags: optional transitive preserved")
	assert.Equal(t, byte(attribute.AttrTombstone), dst[1], "code: AttrTombstone")
	assert.Equal(t, byte(5), dst[2], "length unchanged")
	assert.Equal(t, byte(attribute.AttrAggregator), dst[3], "value[0]: original code")
	assert.Equal(t, TombstoneInvalidLength, dst[4], "value[1]: reason")
	assert.Equal(t, byte(0), dst[5], "value[2]: zeroed")
	assert.Equal(t, byte(0), dst[6], "value[3]: zeroed")
	assert.Equal(t, byte(0), dst[7], "value[4]: zeroed")
}

// TestWriteTombstone_NonTransitive verifies that a non-transitive attribute
// produces a non-transitive tombstone.
//
// VALIDATES: Flags derivation: 0x80 | (0x80 & 0x50) = 0x80.
// PREVENTS: Promoting non-transitive attributes to transitive scope.
func TestWriteTombstone_NonTransitive(t *testing.T) {
	dst := make([]byte, 16)
	n := WriteTombstone(dst, 0, 0x80, attribute.AttrOriginatorID, 3, 4, TombstoneEBGPInvalid)
	require.Equal(t, 7, n)

	// 0x80 | (0x80 & 0x50) = 0x80 | 0x00 = 0x80
	assert.Equal(t, byte(0x80), dst[0], "non-transitive preserved")
}

// TestWriteTombstone_ExtendedLength verifies tombstone with 4-byte header.
//
// VALIDATES: Extended length bit preserved, 2-byte length field written.
// PREVENTS: Wrong header size for extended-length attributes.
func TestWriteTombstone_ExtendedLength(t *testing.T) {
	dst := make([]byte, 272)
	// AIGP: flags=0x90 (optional non-transitive, extended length), hdrLen=4, valueLen=256
	n := WriteTombstone(dst, 0, 0x90, 26, 4, 256, TombstoneMalformedVal)
	require.Equal(t, 260, n)

	// 0x80 | (0x90 & 0x50) = 0x80 | 0x10 = 0x90
	assert.Equal(t, byte(0x90), dst[0], "ext-length preserved")
	assert.Equal(t, byte(attribute.AttrTombstone), dst[1])
	assert.Equal(t, uint16(256), uint16(dst[2])<<8|uint16(dst[3]), "2-byte length")
	assert.Equal(t, byte(26), dst[4], "original AIGP code")
	assert.Equal(t, TombstoneMalformedVal, dst[5], "reason")
	for i := 6; i < 260; i++ {
		if dst[i] != 0 {
			t.Fatalf("value byte %d not zeroed: 0x%02x", i-4, dst[i])
		}
	}
}

// TestWriteTombstone_ValueTooShort verifies that value length < 2 returns 0.
//
// VALIDATES: draft Section 5.1 precondition: value length >= 2.
// PREVENTS: Writing partial tombstone pair into a 0 or 1 byte value.
func TestWriteTombstone_ValueTooShort(t *testing.T) {
	dst := make([]byte, 16)
	assert.Equal(t, 0, WriteTombstone(dst, 0, 0xC0, 7, 3, 1, TombstoneInvalidLength))
	assert.Equal(t, 0, WriteTombstone(dst, 0, 0xC0, 7, 3, 0, TombstoneInvalidLength))
}

// TestWriteTombstone_WellKnown verifies that a well-known attribute (flags 0x40)
// becomes optional (0xC0) in the tombstone.
//
// VALIDATES: Optional bit always set per draft Section 4.2.
// PREVENTS: Well-known tombstone triggering NOTIFICATION on non-recognizing speakers.
func TestWriteTombstone_WellKnown(t *testing.T) {
	dst := make([]byte, 16)
	// LOCAL_PREF: flags=0x40 (well-known transitive)
	n := WriteTombstone(dst, 0, 0x40, attribute.AttrLocalPref, 3, 4, TombstoneEBGPInvalid)
	require.Equal(t, 7, n)

	// 0x80 | (0x40 & 0x50) = 0x80 | 0x40 = 0xC0
	assert.Equal(t, byte(0xC0), dst[0], "well-known becomes optional transitive")
}

// TestWriteTombstone_MinimalValue verifies tombstone with exactly 2-byte value.
//
// VALIDATES: Minimum viable tombstone (code + reason, no tail to zero).
// PREVENTS: Off-by-one in zeroing loop when valueLen == 2.
func TestWriteTombstone_MinimalValue(t *testing.T) {
	dst := make([]byte, 8)
	n := WriteTombstone(dst, 0, 0xC0, 6, 3, 2, TombstoneInvalidLength)
	require.Equal(t, 5, n)

	assert.Equal(t, byte(6), dst[3], "original code")
	assert.Equal(t, TombstoneInvalidLength, dst[4], "reason")
}
