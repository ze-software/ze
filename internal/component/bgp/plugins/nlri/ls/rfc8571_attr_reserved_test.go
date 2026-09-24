package ls

// These tests check the reserved bits in RFC 8571 TLVs 1114, 1115, and 1116.
// RFC 8571 Sections 2.1-2.3 state: "The semantics and values of the fields in
// the TLV are described in [RFC8570] and [RFC7471]." RFC 8571 has no local
// RFC 2119 requirement corresponding to the former RFC8571-x-2 tag.
//
// RFC 8570 Sections 4.1-4.3 and RFC 7471 Sections 4.1.4, 4.2.4, 4.2.6, and
// 4.3.3 require reserved fields to be zero when sent and ignored when received.
// These behavioral checks preserve that receive-side meaning: reserved bits
// do not affect the 24-bit delay values or cause the decoder to reject a TLV.
// The producers are decodeUnidirectionalDelay, decodeMinMaxDelay, and
// decodeDelayVariation in attr_link.go.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRFC8571ReservedIgnoredUnidirectionalDelay(t *testing.T) {
	// TLV 1114 value bytes: flag byte 0xFF = A flag (bit 0) set AND all 7 reserved bits
	// set, followed by the 24-bit Delay 0x0001F4 = 500 microseconds.
	tlv, err := decodeUnidirectionalDelay([]byte{0xFF, 0x00, 0x01, 0xF4})
	require.NoError(t, err)
	d, ok := tlv.(*lsUnidirectionalDelay)
	require.True(t, ok)
	assert.True(t, d.Anomalous)           // bit 0 is the A flag and decodes to true.
	assert.Equal(t, uint32(500), d.Delay) // reserved bits 1-7 do not leak into the value.
}

func TestRFC8571ReservedIgnoredMinMaxDelay(t *testing.T) {
	// TLV 1115 value bytes: flag byte 0xFF (A flag + reserved bits set), MinDelay
	// 0x0001F4 = 500, reserved byte 0xFF (byte 4), MaxDelay 0x0003E8 = 1000.
	tlv, err := decodeMinMaxDelay([]byte{0xFF, 0x00, 0x01, 0xF4, 0xFF, 0x00, 0x03, 0xE8})
	require.NoError(t, err)
	d, ok := tlv.(*lsMinMaxDelay)
	require.True(t, ok)
	assert.True(t, d.Anomalous)               // bit 0 is the A flag.
	assert.Equal(t, uint32(500), d.MinDelay)  // reserved bits 1-7 of byte 0 ignored.
	assert.Equal(t, uint32(1000), d.MaxDelay) // reserved byte 4 (0xFF) ignored.
}

func TestRFC8571ReservedIgnoredDelayVariation(t *testing.T) {
	// TLV 1116 value bytes: reserved byte 0xFF (byte 0), Variation 0x0001F4 = 500.
	tlv, err := decodeDelayVariation([]byte{0xFF, 0x00, 0x01, 0xF4})
	require.NoError(t, err)
	d, ok := tlv.(*lsDelayVariation)
	require.True(t, ok)
	assert.Equal(t, uint32(500), d.Variation) // reserved byte 0 (0xFF) ignored.
}
