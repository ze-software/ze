package ls

// RFC 8571 (BGP-LS IGP TE Performance Metric extensions) reserved-field receive behavior.
//
// VALIDATES: RFC8571-x-2 -- a receiver MUST ignore the Reserved fields of the TE-metric
// Link Attribute TLVs (1114 Unidirectional Link Delay, 1115 Min/Max Unidirectional Link
// Delay, 1116 Unidirectional Delay Variation). The decoders read only the meaningful bits
// and never reject or reinterpret a TLV because reserved bits are set.
// PREVENTS: reserved bits leaking into the decoded value (e.g. the 7 reserved bits of the
// flag byte bleeding into the 24-bit delay), and a spurious decode error on a TLV whose
// reserved bits an upstream speaker left non-zero.
//
// ze runs a decode-only BGP-LS codec: the plugin registers Mode "decode" and sets only
// OnDecodeNLRI (plugin.go:45,70-71), so RFC 8571's transmit-side reserved obligation
// (x-1) has no encoder to exercise. Reserved-on-receipt (x-2) is the only side ze can
// enforce, and RFC 8571 requires ignore-on-receipt (not reject), so a negative case would
// have nothing to assert -- see the single-polarity annotation in rfc/short/rfc8571.md.
// The producers exercised here are decodeUnidirectionalDelay (attr_link.go:755-763),
// decodeMinMaxDelay (attr_link.go:804-813), and decodeDelayVariation (attr_link.go:838-845).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC8571-x-2 positive -- reserved bits set on receipt are ignored; meaningful fields decode intact.
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

// RFC requirement: RFC8571-x-2 positive -- reserved bits set on receipt are ignored; meaningful fields decode intact.
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

// RFC requirement: RFC8571-x-2 positive -- reserved bits set on receipt are ignored; meaningful fields decode intact.
func TestRFC8571ReservedIgnoredDelayVariation(t *testing.T) {
	// TLV 1116 value bytes: reserved byte 0xFF (byte 0), Variation 0x0001F4 = 500.
	tlv, err := decodeDelayVariation([]byte{0xFF, 0x00, 0x01, 0xF4})
	require.NoError(t, err)
	d, ok := tlv.(*lsDelayVariation)
	require.True(t, ok)
	assert.Equal(t, uint32(500), d.Variation) // reserved byte 0 (0xFF) ignored.
}
