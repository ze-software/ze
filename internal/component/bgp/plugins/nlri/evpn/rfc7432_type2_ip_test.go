package evpn

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// type2IPFieldOffset is where the IP Address Length byte sits in a MAC/IP
// Advertisement NLRI: route type (1) + length (1) + RD (8) + ESI (10) +
// Ethernet Tag (4) + MAC Address Length (1) + MAC (6).
const type2IPFieldOffset = 2 + 8 + 10 + 4 + 1 + 6

// TestRFC7432Type2IPAddressOctets verifies the IP Address field of a MAC/IP
// Advertisement route is encoded as exactly 4 octets for an IPv4 address and
// exactly 16 octets for an IPv6 address, on the wire and back.
//
// VALIDATES: RFC 7432 Section 9.2.1 - the IP Address field is 4 octets for
// IPv4 and 16 octets for IPv6, and the Length field counts them.
// PREVENTS: An encoder writing a 16-octet mapped form for an IPv4 address, or
// a decoder reading a different octet count than the address family needs,
// which would shift the label that follows.
//
// RFC requirement: RFC7432-9.2.1-6 positive -- an IPv4 address encodes as exactly 4 octets and an IPv6 address as exactly 16 octets after the IP Address Length byte, the label follows immediately, the NLRI Length counts them, and the decoder reads the same 4 or 16 octets back.
func TestRFC7432Type2IPAddressOctets(t *testing.T) {
	t.Parallel()

	rd := RouteDistinguisher{Type: 0, Value: [6]byte{0xFD, 0xE8, 0, 0, 0, 0x64}}
	esi := [10]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	labels := []uint32{100}
	// A single label entry with the bottom-of-stack bit: 100 << 4 | 1.
	labelBytes := []byte{0x00, 0x06, 0x41}

	cases := []struct {
		name     string
		ip       netip.Addr
		ipLen    byte
		ipOctets []byte
	}{
		{"ipv4", netip.MustParseAddr("10.0.0.1"), 32, []byte{10, 0, 0, 1}},
		{"ipv6", netip.MustParseAddr("2001:db8::1"), 128,
			[]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			route := NewEVPNType2(rd, esi, 0, mac, tc.ip, labels)
			encoded := route.Bytes()

			ipStart := type2IPFieldOffset + 1
			ipEnd := ipStart + len(tc.ipOctets)
			require.Len(t, encoded, ipEnd+len(labelBytes),
				"NLRI must hold header, fixed fields, %d IP octets and one label", len(tc.ipOctets))
			assert.Equal(t, byte(len(encoded)-2), encoded[1],
				"NLRI Length must count the %d IP octets", len(tc.ipOctets))
			assert.Equal(t, tc.ipLen, encoded[type2IPFieldOffset], "IP Address Length in bits")
			assert.Equal(t, tc.ipOctets, encoded[ipStart:ipEnd],
				"IP Address must be exactly %d octets", len(tc.ipOctets))
			assert.Equal(t, labelBytes, encoded[ipEnd:],
				"the label must follow immediately after the IP octets")

			parsed, remaining, err := ParseEVPN(encoded, false)
			require.NoError(t, err)
			require.Empty(t, remaining, "the decoder must consume exactly the encoded octets")
			decoded, ok := parsed.(*EVPNType2)
			require.True(t, ok, "expected EVPNType2, got %T", parsed)
			assert.Equal(t, tc.ip, decoded.IP())
			assert.Equal(t, len(tc.ipOctets), decoded.IP().BitLen()/8,
				"decoded address family must match the octet count")
			assert.Equal(t, labels, decoded.Labels(), "the label after the IP octets must decode intact")
		})
	}
}

// TestRFC7432Type2IPAddressOctetsShort verifies a MAC/IP Advertisement route
// whose IP Address field holds fewer octets than its IP Address Length
// announces is refused, instead of being read as a shorter address or as
// label octets.
//
// VALIDATES: RFC 7432 Section 9.2.1 - the IP Address field is 4 or 16 octets.
// PREVENTS: A short IP Address field being accepted, with the missing octets
// read from the label stack or from the next NLRI.
//
// RFC requirement: RFC7432-9.2.1-6 negative -- an IP Address Length of 32 followed by fewer than 4 octets, or of 128 followed by fewer than 16 octets, is rejected with ErrEVPNTruncated and yields no route.
func TestRFC7432Type2IPAddressOctetsShort(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		ipLen   byte
		ipBytes []byte
	}{
		{"ipv4_three_octets", 32, []byte{10, 0, 0}},
		{"ipv4_no_octets", 32, nil},
		{"ipv6_fifteen_octets", 128, []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"ipv6_four_octets", 128, []byte{10, 0, 0, 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := evpnT2Body(48, tc.ipLen, tc.ipBytes)
			data := buildEVPNData(EVPNRouteType2, byte(len(body)), body)

			parsed, remaining, err := ParseEVPN(data, false)
			require.ErrorIs(t, err, ErrEVPNTruncated,
				"IP Address Length %d over %d octets must be refused", tc.ipLen, len(tc.ipBytes))
			assert.Nil(t, parsed, "a refused NLRI must yield no route")
			assert.Nil(t, remaining, "a refused NLRI must yield no remainder")
		})
	}
}
