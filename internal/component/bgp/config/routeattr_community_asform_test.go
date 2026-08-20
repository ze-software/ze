// Design: routeattr_community.go — which extended community form Ze writes for
// an AS number, and which numbers that form can carry.
// Related: routeattr_community.go, internal/core/bgp/attribute/extcomm_decoded.go

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestParseExtendedCommunityASForm verifies that the administrator width of the
// AS number picks the extended community type.
//
// The goal is one wire form for each pair of administrator widths, and the
// method is a byte comparison against the layouts the RFCs draw. RFC 4360
// Section 3.1 gives type 0x00 a 2-octet global administrator and a 4-octet
// local administrator. Section 3.2 gives type 0x01 an IPv4 unicast address and
// a 2-octet local administrator. RFC 5668 Section 2 gives type 0x02 a 4-octet
// AS number and a 2-octet local administrator.
//
// VALIDATES: a four-byte AS number goes in the type 0x02 global administrator,
// and a two-byte one still goes in the type 0x00 global administrator.
//
// PREVENTS: a four-byte AS number written into the type 0x01 global
// administrator, which RFC 4360 Section 3.2 defines as "an IPv4 unicast address
// assigned by one of the Internet registries". A peer reads target:65536:100
// sent that way as target:0.1.0.0:100, so the two ends disagree on the route
// target and the VPN route lands in no VRF.
func TestParseExtendedCommunityASForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []byte
	}{
		{
			name:  "two-byte AS keeps the type 0x00 form",
			input: "target:65000:1",
			want:  []byte{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x01},
		},
		{
			name:  "last two-byte AS keeps the type 0x00 form",
			input: "target:65535:1",
			want:  []byte{0x00, 0x02, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x01},
		},
		{
			name:  "first four-byte AS takes the type 0x02 form",
			input: "target:65536:100",
			want:  []byte{0x02, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x64},
		},
		{
			name:  "four-byte AS route origin takes the type 0x02 form",
			input: "origin:130000:1234",
			want:  []byte{0x02, 0x03, 0x00, 0x01, 0xFB, 0xD0, 0x04, 0xD2},
		},
		{
			name:  "high four-byte AS takes the type 0x02 form",
			input: "target:4200000001:100",
			want:  []byte{0x02, 0x02, 0xFA, 0x56, 0xEA, 0x01, 0x00, 0x64},
		},
		{
			name:  "unqualified pair takes the same form as target",
			input: "65536:100",
			want:  []byte{0x02, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x64},
		},
		{
			name:  "L suffix takes the type 0x02 form",
			input: "target:120000L:123",
			want:  []byte{0x02, 0x02, 0x00, 0x01, 0xD4, 0xC0, 0x00, 0x7B},
		},
		{
			name:  "target4 takes the type 0x02 form for a two-byte AS",
			input: "target4:65000:100",
			want:  []byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE8, 0x00, 0x64},
		},
		{
			name:  "origin4 takes the type 0x02 form for a two-byte AS",
			input: "origin4:65000:100",
			want:  []byte{0x02, 0x03, 0x00, 0x00, 0xFD, 0xE8, 0x00, 0x64},
		},
		{
			name:  "an IPv4 administrator still takes the type 0x01 form",
			input: "target:1.2.3.4:100",
			want:  []byte{0x01, 0x02, 0x01, 0x02, 0x03, 0x04, 0x00, 0x64},
		},
		{
			name:  "two-byte AS redirect takes the type 0x80 form",
			input: "redirect:65500:12345",
			want:  []byte{0x80, 0x08, 0xFF, 0xDC, 0x00, 0x00, 0x30, 0x39},
		},
		{
			name:  "four-byte AS redirect takes the type 0x82 form",
			input: "redirect:65536:100",
			want:  []byte{0x82, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00, 0x64},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ec, err := ParseExtendedCommunity(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ec.Bytes)
		})
	}
}

// TestParseExtendedCommunityFourByteASNumberLimit verifies that a number too
// wide for the type 0x02 local administrator is refused.
//
// The goal is that no configured value reaches the wire with octets removed,
// and the method is to assert the error and the limit it names. RFC 5668
// Section 2 makes the local administrator sub-field 2 octets, and no form in
// RFC 4360 or RFC 5668 carries a four-octet AS number beside a four-octet
// number, so the pair has no encoding at all.
//
// VALIDATES: a number above 65535 beside a four-byte AS number is rejected with
// an error naming 65535.
//
// PREVENTS: the silent truncation the type 0x02 branch used to perform.
// target:65536:70000 became 0x0202 00010000 1170, so the peer received route
// target 65536:4464 and the operator saw no error.
func TestParseExtendedCommunityFourByteASNumberLimit(t *testing.T) {
	t.Parallel()

	t.Run("last number the type 0x02 form carries", func(t *testing.T) {
		t.Parallel()
		ec, err := ParseExtendedCommunity("target:65536:65535")
		require.NoError(t, err)
		assert.Equal(t, []byte{0x02, 0x02, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xFF}, ec.Bytes)
	})

	refused := []struct {
		name  string
		input string
	}{
		{name: "first number above the limit", input: "target:65536:65536"},
		{name: "route origin above the limit", input: "origin:65536:100000"},
		{name: "unqualified pair above the limit", input: "65536:70000"},
		{name: "L suffix above the limit", input: "target:120000L:65536"},
		{name: "target4 above the limit", input: "target4:1:65536"},
		{name: "origin4 above the limit", input: "origin4:1:70000"},
		{name: "redirect above the limit", input: "redirect:65536:65536"},
	}

	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseExtendedCommunity(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "65535")
		})
	}

	t.Run("four-byte AS beside an IPv4 value has no form", func(t *testing.T) {
		t.Parallel()
		_, err := ParseExtendedCommunity("target:65536:1.2.3.4")
		require.ErrorIs(t, err, err4ByteAsnWithIpValue)

		_, err = ParseExtendedCommunity("65536:1.2.3.4")
		require.ErrorIs(t, err, err4ByteAsnWithIpValue)
	})
}

// TestParseExtendedCommunityASFormRoundTrip verifies that a configured
// extended community reads back as the string that was configured.
//
// The goal is that the encoder and the decoder agree on where the AS number
// sits in the 6-octet value field, and the method is to run the config parser
// and the wire decoder back to back. AppendDecoded
// (internal/core/bgp/attribute/extcomm_decoded.go) reads the type octet and
// splits the value field 2+4, 4+2 or address+2 accordingly.
//
// VALIDATES: target:65536:100 encodes and decodes to target:65536:100.
//
// PREVENTS: the round trip that renamed the community. The type 0x01 encoding
// of a four-byte AS number decoded to target:0.1.0.0:100, so an operator
// reading `show` back saw a route target nobody had configured.
func TestParseExtendedCommunityASFormRoundTrip(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"target:65000:1",
		"target:65536:100",
		"target:65536:65535",
		"target:4200000001:100",
		"origin:130000:1234",
		"target:1.2.3.4:100",
		"origin:192.168.1.1:200",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			ec, err := ParseExtendedCommunity(input)
			require.NoError(t, err)
			require.Len(t, ec.Bytes, 8)

			var wire attribute.ExtendedCommunity
			copy(wire[:], ec.Bytes)
			assert.Equal(t, input, wire.String())
		})
	}
}
