package bgp_nlri_rtc

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRTCBasic verifies basic RTC NLRI creation.
func TestRTCBasic(t *testing.T) {
	rt := RouteTarget{
		Type:  0,
		Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100},
	}
	rtc := NewRTC(65001, rt)

	assert.Equal(t, uint32(65001), rtc.OriginAS())
}

// TestRTCFamily verifies RTC address family.
func TestRTCFamily(t *testing.T) {
	rtc := NewRTC(65001, RouteTarget{})

	assert.Equal(t, AFIIPv4, rtc.Family().AFI)
	assert.Equal(t, SAFIRTC, rtc.Family().SAFI)
}

// TestRTCBytes verifies RTC wire format.
func TestRTCBytes(t *testing.T) {
	rtc := NewRTC(65001, RouteTarget{
		Type:  0,
		Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100},
	})

	data := rtc.Bytes()
	require.NotEmpty(t, data)
	// Full RTC NLRI: 1 prefix-len + 4 origin AS + 8 RT = 13 bytes
	assert.Equal(t, 13, len(data))
}

// TestRTCDefault verifies default RTC (matches all RTs).
func TestRTCDefault(t *testing.T) {
	rtc := NewRTC(0, RouteTarget{})

	assert.True(t, rtc.IsDefault())
	assert.Equal(t, []byte{0}, rtc.Bytes())
}

// TestRTCRoundTrip verifies encode/decode cycle.
func TestRTCRoundTrip(t *testing.T) {
	rt := RouteTarget{
		Type:  0x0002,
		Value: [6]byte{0, 0, 0xFD, 0xE9, 0, 100},
	}
	original := NewRTC(65001, rt)
	data := original.Bytes()

	parsed, remaining, err := ParseRTC(data)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Equal(t, original.OriginAS(), parsed.OriginAS())
	assert.Equal(t, original.RouteTargetValue().Type, parsed.RouteTargetValue().Type)
}

// TestRTCParseDefault verifies parsing default RTC.
func TestRTCParseDefault(t *testing.T) {
	data := []byte{0} // prefix-length = 0

	parsed, remaining, err := ParseRTC(data)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.True(t, parsed.IsDefault())
}

// TestRTCParseErrors verifies error handling.
func TestRTCParseErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseRTC(tt.data)
			assert.Error(t, err)
		})
	}
}

// TestRouteTargetString verifies RT string formatting.
func TestRouteTargetString(t *testing.T) {
	tests := []struct {
		name     string
		rt       RouteTarget
		expected string
	}{
		{
			name:     "2-byte ASN",
			rt:       RouteTarget{Type: 0x0002, Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100}},
			expected: "65001:100",
		},
		{
			name:     "4-byte ASN",
			rt:       RouteTarget{Type: 0x0200, Value: [6]byte{0, 0, 0xFD, 0xE9, 0, 100}},
			expected: "65001:100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rt.String())
		})
	}
}

// TestRTCStringCommandStyle verifies command-style string representation.
//
// VALIDATES: RTC String() outputs command-style format for API round-trip.
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestRTCStringCommandStyle(t *testing.T) {
	tests := []struct {
		name     string
		rtc      *RTC
		expected string
	}{
		{
			name:     "default rtc",
			rtc:      NewRTC(0, RouteTarget{}),
			expected: "default",
		},
		{
			name: "rtc with 2-byte asn rt",
			rtc: NewRTC(65001, RouteTarget{
				Type:  0x0002,
				Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100},
			}),
			expected: "origin-as set 65001 rt set 65001:100",
		},
		{
			name: "rtc with 4-byte asn rt",
			rtc: NewRTC(65002, RouteTarget{
				Type:  0x0200,
				Value: [6]byte{0, 0, 0xFD, 0xE9, 0, 200},
			}),
			expected: "origin-as set 65002 rt set 65001:200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rtc.String())
		})
	}
}

// TestRTCWriteToMatchesBytes verifies RTC.WriteTo matches Bytes().
//
// VALIDATES: WriteTo produces identical wire format to Bytes() for RTC NLRI.
// PREVENTS: Origin AS encoding errors, route target data corruption.
func TestRTCWriteToMatchesBytes(t *testing.T) {
	tests := []struct {
		name string
		rtc  *RTC
	}{
		{
			name: "default rtc",
			rtc:  NewRTC(0, RouteTarget{}),
		},
		{
			name: "rtc with route target",
			rtc: NewRTC(65001, RouteTarget{
				Type:  0,
				Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.rtc.Bytes()

			buf := make([]byte, len(expected)+10)
			n := tt.rtc.WriteTo(buf, 0)

			assert.Equal(t, len(expected), n, "WriteTo returned wrong length")
			assert.Equal(t, expected, buf[:n], "WriteTo output differs from Bytes()")
		})
	}
}

// TestRTCRouteTargetValueAccessor verifies the renamed accessor.
//
// VALIDATES: RouteTargetValue() returns the expected route target.
// PREVENTS: Regression from method rename (RouteTarget→RouteTargetValue).
func TestRTCRouteTargetValueAccessor(t *testing.T) {
	rt := RouteTarget{
		Type:  0x0002,
		Value: [6]byte{0xFD, 0xE9, 0, 0, 0, 100},
	}
	rtc := NewRTC(65001, rt)

	got := rtc.RouteTargetValue()
	assert.Equal(t, rt.Type, got.Type)
	assert.Equal(t, rt.Value, got.Value)

	// Also verify via round-trip
	data := rtc.Bytes()
	parsed, _, err := ParseRTC(data)
	require.NoError(t, err)

	binary.BigEndian.PutUint16(rt.Value[:2], 65001)
	assert.Equal(t, rt.Type, parsed.RouteTargetValue().Type)
}
