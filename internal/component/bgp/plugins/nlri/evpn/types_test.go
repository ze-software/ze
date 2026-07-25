package evpn

import (
	"bytes"
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// buildEVPNData builds EVPN test data with pre-allocated capacity.
func buildEVPNData(routeType EVPNRouteType, length byte, components ...[]byte) []byte {
	total := 2 // type + length bytes
	for _, c := range components {
		total += len(c)
	}
	data := make([]byte, 0, total)
	data = append(data, byte(routeType), length)
	for _, c := range components {
		data = append(data, c...)
	}
	return data
}

// TestEVPNType2MACOnly verifies Type 2 MAC-only route parsing.
//
// VALIDATES: Basic MAC advertisement without IP.
// PREVENTS: MAC-only route parsing failures.
func TestEVPNType2MACOnly(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64} // 65000:100
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(0)
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, label)

	parsed, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := parsed.(*EVPNType2)
	require.True(t, ok)

	assert.Equal(t, EVPNRouteType2, evpn.RouteType())
	assert.Equal(t, "0:65000:100", evpn.RD().String())
	assert.Equal(t, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, evpn.MAC())
	assert.False(t, evpn.IP().IsValid())
	assert.Equal(t, L2VPNEVPN, evpn.Family())
}

// TestEVPNType2MACIPv4 verifies Type 2 MAC+IPv4 route parsing.
//
// VALIDATES: MAC+IPv4 advertisement.
// PREVENTS: IPv4 parsing errors in MAC/IP routes.
func TestEVPNType2MACIPv4(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+4+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, ip, label)

	parsed, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType2)
	require.True(t, ok, "expected EVPNType2")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), evpn.IP())
}

// TestEVPNType2MACIPv6 verifies Type 2 MAC+IPv6 route parsing.
//
// VALIDATES: MAC+IPv6 advertisement.
// PREVENTS: IPv6 parsing errors in MAC/IP routes.
func TestEVPNType2MACIPv6(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(128)
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+16+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, ip, label)

	parsed, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType2)
	require.True(t, ok, "expected EVPNType2")
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), evpn.IP())
}

// TestEVPNType3 verifies Type 3 Inclusive Multicast route parsing.
//
// VALIDATES: IMET route for BUM traffic.
// PREVENTS: Multicast route parsing failures.
func TestEVPNType3(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}

	data := buildEVPNData(EVPNRouteType3, byte(8+4+1+4),
		rd, ethTag, []byte{ipLen}, ip)

	parsed, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType3)
	require.True(t, ok)

	assert.Equal(t, EVPNRouteType3, evpn.RouteType())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), evpn.OriginatorIP())
}

// TestEVPNType5IPv4 verifies Type 5 IP Prefix route parsing for IPv4.
//
// VALIDATES: RFC 9136 Section 3.1 - IPv4 IP Prefix route.
// PREVENTS: Incorrect variable-length prefix parsing that violates RFC 9136.
func TestEVPNType5IPv4(t *testing.T) {
	t.Parallel()
	// RFC requirement: RFC9136-3.1-1 positive -- a valid 34-octet IPv4 RT-5 NLRI decodes
	// RFC requirement: RFC9136-3.1-5 positive -- IP prefix length 24 (<= 32) decodes to a valid IPv4 prefix
	// RFC requirement: RFC9136-3.1-3 positive -- RD and Ethernet Tag fields decode per RFC 7432/8365
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x0A}
	ipLen := byte(24)
	ip := []byte{10, 1, 2, 0}
	gw := []byte{0, 0, 0, 0}
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType5, byte(34),
		rd, esi, ethTag, []byte{ipLen}, ip, gw, label)

	parsed, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := parsed.(*EVPNType5)
	require.True(t, ok, "expected EVPNType5, got %T", parsed)

	assert.Equal(t, EVPNRouteType5, evpn.RouteType())
	assert.Equal(t, "0:65000:100", evpn.RD().String())
	assert.Equal(t, uint32(10), evpn.EthernetTag())
	assert.Equal(t, netip.MustParsePrefix("10.1.2.0/24"), evpn.Prefix())
	assert.Equal(t, netip.MustParseAddr("0.0.0.0"), evpn.Gateway())
	assert.Equal(t, L2VPNEVPN, evpn.Family())
}

// TestEVPNType5IPv6 verifies Type 5 IP Prefix route parsing for IPv6.
//
// VALIDATES: RFC 9136 Section 3.1 - IPv6 IP Prefix route.
// PREVENTS: Incorrect variable-length prefix parsing for IPv6.
func TestEVPNType5IPv6(t *testing.T) {
	t.Parallel()
	// RFC requirement: RFC9136-3.1-1 positive -- a valid 58-octet IPv6 RT-5 NLRI decodes
	// RFC requirement: RFC9136-3.1-5 positive -- IP prefix length 64 (<= 128) decodes to a valid IPv6 prefix
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(64)
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	gw := make([]byte, 16)
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType5, byte(58),
		rd, esi, ethTag, []byte{ipLen}, ip, gw, label)

	parsed, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType5)
	require.True(t, ok, "expected EVPNType5")

	assert.Equal(t, netip.MustParsePrefix("2001:db8::/64"), evpn.Prefix())
}

// TestEVPNType5InvalidLength verifies rejection of invalid NLRI lengths.
//
// VALIDATES: RFC 9136 requirement that length MUST be 34 (IPv4) or 58 (IPv6).
// PREVENTS: Accepting malformed Type 5 routes with incorrect lengths.
func TestEVPNType5InvalidLength(t *testing.T) {
	t.Parallel()
	// RFC requirement: RFC9136-3.1-1 negative -- an RT-5 NLRI whose length is neither 34 nor 58 is rejected
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(24)
	ip := []byte{10, 1, 2}
	gw := []byte{0, 0, 0, 0}
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType5, byte(8+10+4+1+3+4+3),
		rd, esi, ethTag, []byte{ipLen}, ip, gw, label)

	_, _, err := ParseEVPN(data, false)
	require.Error(t, err, "should reject non-standard Type 5 length")
}

// TestEVPNType5PrefixLengthTooLong verifies rejection of an IP Prefix Length that
// exceeds the host address width, even when the total NLRI length is well-formed.
//
// VALIDATES: RFC 9136 Section 3.1 - IP Prefix Length MUST NOT exceed 32 (IPv4) / 128 (IPv6).
// PREVENTS: Accepting an RT-5 route whose prefix length field is longer than the address.
func TestEVPNType5PrefixLengthTooLong(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	label := []byte{0x00, 0x01, 0x01}

	t.Run("ipv4", func(t *testing.T) {
		t.Parallel()
		// RFC requirement: RFC9136-3.1-5 negative -- IPv4 RT-5 (length 34) with prefix length 33 (> 32) is rejected
		ip := []byte{10, 1, 2, 0}
		gw := []byte{0, 0, 0, 0}
		data := buildEVPNData(EVPNRouteType5, byte(34),
			rd, esi, ethTag, []byte{byte(33)}, ip, gw, label)

		_, _, err := ParseEVPN(data, false)
		require.ErrorIs(t, err, ErrEVPNInvalidPrefix,
			"IPv4 prefix length 33 must be rejected as invalid")
	})

	t.Run("ipv6", func(t *testing.T) {
		t.Parallel()
		// RFC requirement: RFC9136-3.1-5 negative -- IPv6 RT-5 (length 58) with prefix length 129 (> 128) is rejected
		ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		gw := make([]byte, 16)
		data := buildEVPNData(EVPNRouteType5, byte(58),
			rd, esi, ethTag, []byte{byte(129)}, ip, gw, label)

		_, _, err := ParseEVPN(data, false)
		require.ErrorIs(t, err, ErrEVPNInvalidPrefix,
			"IPv6 prefix length 129 must be rejected as invalid")
	})
}

// evpnT2Body builds the route-type-specific body of a MAC/IP Advertisement
// route with caller-chosen MAC Address Length and IP Address Length fields.
// Everything else is well formed, so a rejection can only come from the two
// length fields under test.
func evpnT2Body(macLen, ipLen byte, ipBytes []byte) []byte {
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	body := make([]byte, 0, 8+10+4+1+6+1+len(ipBytes))
	body = append(body, rd...)
	body = append(body, esi...)
	body = append(body, ethTag...)
	body = append(body, macLen)
	body = append(body, mac...)
	body = append(body, ipLen)
	body = append(body, ipBytes...)
	return body
}

// TestEVPNMACAddressLength verifies that the MAC Address Length field of a
// MAC/IP Advertisement route is honored as the 48-bit / 6-octet IEEE 802.1Q
// encoding and nothing else.
//
// VALIDATES: RFC 7432 Section 9.2.1 - MAC addresses are the 6-octet 802.1Q form.
// PREVENTS: A MAC/IP route with a non-48-bit MAC length being decoded, which
// would misalign every field that follows the MAC address.
func TestEVPNMACAddressLength(t *testing.T) {
	t.Parallel()

	t.Run("accepts48", func(t *testing.T) {
		t.Parallel()
		// RFC requirement: RFC7432-9.2.1-1 positive -- MAC Address Length 48 decodes to the 6-octet 802.1Q MAC and re-encodes with the length field back at 48
		body := evpnT2Body(48, 0, nil)
		data := buildEVPNData(EVPNRouteType2, byte(len(body)), body)

		parsed, remaining, err := ParseEVPN(data, false)
		require.NoError(t, err)
		require.Empty(t, remaining)

		e, ok := parsed.(*EVPNType2)
		require.True(t, ok, "expected EVPNType2, got %T", parsed)
		assert.Equal(t, [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, e.MAC())

		// MAC Address Length sits after route-type(1) + length(1) + RD(8) + ESI(10) + Ethernet Tag(4).
		encoded := e.Bytes()
		assert.Equal(t, byte(48), encoded[2+8+10+4], "encoder must write a 48-bit MAC Address Length")
		assert.Equal(t, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, encoded[2+8+10+4+1:2+8+10+4+1+6],
			"encoder must write exactly 6 MAC octets")
	})

	for _, macLen := range []byte{0, 32, 47, 49, 64, 255} {
		t.Run("rejects"+strconv.Itoa(int(macLen)), func(t *testing.T) {
			t.Parallel()
			// RFC requirement: RFC7432-9.2.1-1 negative -- a MAC Address Length other than 48 bits is rejected instead of being decoded as a MAC address
			body := evpnT2Body(macLen, 0, nil)
			data := buildEVPNData(EVPNRouteType2, byte(len(body)), body)

			_, _, err := ParseEVPN(data, false)
			require.ErrorIs(t, err, ErrEVPNInvalidAddress,
				"MAC Address Length %d must be rejected", macLen)
		})
	}
}

// TestEVPNIPAddressLength verifies the IP Address Length field of a MAC/IP
// Advertisement route is one of the three values RFC 7432 defines, in bits.
//
// VALIDATES: RFC 7432 Section 10 - IP Address Length is 0, 32 or 128 bits.
// PREVENTS: A length above 128 falling through the decoder and being read as
// "no IP address", which silently accepts a malformed NLRI.
func TestEVPNIPAddressLength(t *testing.T) {
	t.Parallel()

	v4 := []byte{10, 0, 0, 1}
	v6 := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	valid := []struct {
		name    string
		ipLen   byte
		ipBytes []byte
		wantIP  string
	}{
		{"absent", 0, nil, ""},
		{"ipv4", 32, v4, "10.0.0.1"},
		{"ipv6", 128, v6, "2001:db8::1"},
	}

	for _, tt := range valid {
		t.Run("accepts_"+tt.name, func(t *testing.T) {
			t.Parallel()
			// RFC requirement: RFC7432-10-1 positive -- IP Address Length 0, 32 and 128 bits each decode, with the IP octet count matching the advertised bit count
			body := evpnT2Body(48, tt.ipLen, tt.ipBytes)
			data := buildEVPNData(EVPNRouteType2, byte(len(body)), body)

			parsed, remaining, err := ParseEVPN(data, false)
			require.NoError(t, err)
			require.Empty(t, remaining)

			e, ok := parsed.(*EVPNType2)
			require.True(t, ok, "expected EVPNType2, got %T", parsed)
			if tt.wantIP == "" {
				assert.False(t, e.IP().IsValid(), "IP Address Length 0 must decode to no IP address")
				return
			}
			assert.Equal(t, netip.MustParseAddr(tt.wantIP), e.IP())
			assert.Equal(t, int(tt.ipLen)/8, len(tt.ipBytes), "test vector length must match the bit count")
		})
	}

	for _, ipLen := range []byte{1, 4, 31, 33, 64, 127, 129, 160, 255} {
		t.Run("rejects"+strconv.Itoa(int(ipLen)), func(t *testing.T) {
			t.Parallel()
			// RFC requirement: RFC7432-10-1 negative -- an IP Address Length that is not 0, 32 or 128 bits is rejected, including values above 128
			body := evpnT2Body(48, ipLen, nil)
			data := buildEVPNData(EVPNRouteType2, byte(len(body)), body)

			_, _, err := ParseEVPN(data, false)
			require.ErrorIs(t, err, ErrEVPNInvalidAddress,
				"IP Address Length %d must be rejected", ipLen)
		})
	}
}

// TestEVPNESIIsTenOctets verifies the Ethernet Segment Identifier occupies
// exactly ten octets on the wire, in both the Ethernet Segment route and the
// Ethernet Auto-Discovery route.
//
// VALIDATES: RFC 7432 Section 5 / 8.2.1 - the ESI is a 10-octet entity.
// PREVENTS: A short ESI shifting the Originating Router's IP Address, or a
// decoded ESI that does not round-trip byte-for-byte.
func TestEVPNESIIsTenOctets(t *testing.T) {
	t.Parallel()

	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}

	t.Run("ethernetSegmentCarriesTenOctets", func(t *testing.T) {
		t.Parallel()
		// RFC requirement: RFC7432-5-1 positive -- the ESI decodes from and re-encodes to exactly ten octets, immediately after the 8-octet RD
		data := buildEVPNData(EVPNRouteType4, byte(8+10+1+4),
			rd, esi, []byte{32}, []byte{10, 0, 0, 1})

		parsed, remaining, err := ParseEVPN(data, false)
		require.NoError(t, err)
		require.Empty(t, remaining)

		e, ok := parsed.(*EVPNType4)
		require.True(t, ok, "expected EVPNType4, got %T", parsed)
		got := e.ESI()
		assert.Len(t, got[:], 10, "ESI must be a 10-octet entity")
		assert.Equal(t, esi, got[:])

		encoded := e.Bytes()
		assert.Equal(t, esi, encoded[2+8:2+8+10], "encoder must write the ESI as ten octets after the RD")
		assert.Equal(t, byte(32), encoded[2+8+10], "IP Address Length must follow the tenth ESI octet")
	})

	t.Run("ethernetADCarriesTenOctets", func(t *testing.T) {
		t.Parallel()
		// RFC requirement: RFC7432-5-1 positive -- the Ethernet A-D route carries the same 10-octet ESI, with the Ethernet Tag ID starting at the eleventh octet
		data := buildEVPNData(EVPNRouteType1, byte(8+10+4+3),
			rd, esi, []byte{0x00, 0x00, 0x00, 0x0A}, []byte{0x00, 0x01, 0x01})

		parsed, _, err := ParseEVPN(data, false)
		require.NoError(t, err)

		e, ok := parsed.(*EVPNType1)
		require.True(t, ok, "expected EVPNType1, got %T", parsed)
		got := e.ESI()
		assert.Equal(t, esi, got[:])
		assert.Equal(t, uint32(10), e.EthernetTag(), "Ethernet Tag ID must be read after ten ESI octets")

		encoded := e.Bytes()
		assert.Equal(t, esi, encoded[2+8:2+8+10])
	})

	t.Run("rejectsBodyTooShortForTenOctetESI", func(t *testing.T) {
		t.Parallel()
		// RFC requirement: RFC7432-5-1 negative -- an Ethernet Segment route whose body cannot hold a 10-octet ESI is rejected rather than decoded from a shorter ESI
		short := make([]byte, 0, 8+9+1)
		short = append(short, rd...)
		short = append(short, esi[:9]...) // only nine ESI octets
		short = append(short, 32)
		data := buildEVPNData(EVPNRouteType4, byte(len(short)), short)

		_, _, err := ParseEVPN(data, false)
		require.ErrorIs(t, err, ErrEVPNTruncated,
			"a body too short for a 10-octet ESI must be rejected")
	})

	t.Run("rejectsADBodyTooShortForTenOctetESI", func(t *testing.T) {
		t.Parallel()
		// RFC requirement: RFC7432-5-1 negative -- an Ethernet A-D route whose body cannot hold a 10-octet ESI plus the Ethernet Tag ID is rejected
		short := make([]byte, 0, 8+9+4)
		short = append(short, rd...)
		short = append(short, esi[:9]...)
		short = append(short, 0x00, 0x00, 0x00, 0x0A)
		data := buildEVPNData(EVPNRouteType1, byte(len(short)), short)

		_, _, err := ParseEVPN(data, false)
		require.ErrorIs(t, err, ErrEVPNTruncated,
			"a body too short for a 10-octet ESI must be rejected")
	})
}

// TestEVPNRouteTypeString verifies route type string representation.
func TestEVPNRouteTypeString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ethernet-auto-discovery", EVPNRouteType1.String())
	assert.Equal(t, "mac-ip-advertisement", EVPNRouteType2.String())
	assert.Equal(t, "inclusive-multicast", EVPNRouteType3.String())
	assert.Equal(t, "ethernet-segment", EVPNRouteType4.String())
	assert.Equal(t, "ip-prefix", EVPNRouteType5.String())
}

// TestEVPNType1 verifies Type 1 Ethernet Auto-Discovery route parsing.
//
// VALIDATES: RFC 7432 Section 7.1 - Ethernet A-D route format.
// PREVENTS: Parsing failures for Ethernet Auto-Discovery routes.
func TestEVPNType1(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ethTag := []byte{0x00, 0x00, 0x00, 0x0A}
	label := []byte{0x00, 0x01, 0x01}

	data := buildEVPNData(EVPNRouteType1, byte(8+10+4+3),
		rd, esi, ethTag, label)

	parsed, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := parsed.(*EVPNType1)
	require.True(t, ok, "expected EVPNType1, got %T", parsed)

	assert.Equal(t, EVPNRouteType1, evpn.RouteType())
	assert.Equal(t, "0:65000:100", evpn.RD().String())
	assert.Equal(t, ESI{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}, evpn.ESI())
	assert.Equal(t, uint32(10), evpn.EthernetTag())
	assert.Equal(t, []uint32{16}, evpn.Labels())
	assert.Equal(t, L2VPNEVPN, evpn.Family())
}

// TestEVPNType1WithAddPath verifies Type 1 with Add-Path ID.
//
// VALIDATES: Ethernet Auto-Discovery with path ID (RFC 7911 support).
// PREVENTS: Add-path parsing errors for Type 1 routes.
func TestEVPNType1WithAddPath(t *testing.T) {
	t.Parallel()
	pathID := []byte{0x00, 0x00, 0x00, 0x05}
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	label := []byte{0x00, 0x01, 0x01}

	data := pathID
	data = append(data, byte(EVPNRouteType1), byte(8+10+4+3))
	data = append(data, rd...)
	data = append(data, esi...)
	data = append(data, ethTag...)
	data = append(data, label...)

	parsed, _, err := ParseEVPN(data, true)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType1)
	require.True(t, ok, "expected EVPNType1")
	assert.True(t, evpn.PathID() != 0)
	assert.Equal(t, uint32(5), evpn.PathID())
}

// TestEVPNType4IPv4 verifies Type 4 Ethernet Segment route with IPv4.
//
// VALIDATES: RFC 7432 Section 7.4 - Ethernet Segment route format.
// PREVENTS: Parsing failures for Ethernet Segment routes.
func TestEVPNType4IPv4(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}

	data := buildEVPNData(EVPNRouteType4, byte(8+10+1+4),
		rd, esi, []byte{ipLen}, ip)

	parsed, remaining, err := ParseEVPN(data, false)
	require.NoError(t, err)
	require.Empty(t, remaining)

	evpn, ok := parsed.(*EVPNType4)
	require.True(t, ok, "expected EVPNType4, got %T", parsed)

	assert.Equal(t, EVPNRouteType4, evpn.RouteType())
	assert.Equal(t, "0:65000:100", evpn.RD().String())
	assert.Equal(t, ESI{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}, evpn.ESI())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), evpn.OriginatorIP())
	assert.Equal(t, L2VPNEVPN, evpn.Family())
}

// TestEVPNType4IPv6 verifies Type 4 Ethernet Segment route with IPv6.
//
// VALIDATES: RFC 7432 Section 7.4 - Ethernet Segment route with IPv6.
// PREVENTS: IPv6 parsing errors in Ethernet Segment routes.
func TestEVPNType4IPv6(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ipLen := byte(128)
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	data := buildEVPNData(EVPNRouteType4, byte(8+10+1+16),
		rd, esi, []byte{ipLen}, ip)

	parsed, _, err := ParseEVPN(data, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType4)
	require.True(t, ok, "expected EVPNType4")
	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), evpn.OriginatorIP())
}

// TestEVPNType4InvalidIPLen verifies error handling for invalid IP length.
//
// VALIDATES: Rejection of invalid IP address lengths per RFC 7432.
// PREVENTS: Accepting malformed Ethernet Segment routes.
func TestEVPNType4InvalidIPLen(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ipLen := byte(64)
	ip := make([]byte, 8)

	data := buildEVPNData(EVPNRouteType4, byte(8+10+1+8),
		rd, esi, []byte{ipLen}, ip)

	_, _, err := ParseEVPN(data, false)
	require.Error(t, err, "should reject invalid IP length")
}

// TestEVPNErrors verifies error handling.
func TestEVPNErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"no length", []byte{byte(EVPNRouteType2)}},
		{"truncated", []byte{byte(EVPNRouteType2), 50, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := ParseEVPN(tt.data, false)
			require.Error(t, err)
		})
	}
}

// TestEVPNType1RoundTrip verifies Type 1 encode/decode symmetry.
//
// VALIDATES: Bytes() produces wire format that ParseEVPN() can read back.
// PREVENTS: Asymmetric encoding that would corrupt routes.
func TestEVPNType1RoundTrip(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ethTag := []byte{0x00, 0x00, 0x00, 0x0A}
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType1, byte(8+10+4+3),
		rd, esi, ethTag, label)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType1)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType2RoundTripMACOnly verifies Type 2 MAC-only round-trip.
//
// VALIDATES: Bytes() correctly encodes MAC-only routes.
// PREVENTS: MAC-only routes being corrupted.
func TestEVPNType2RoundTripMACOnly(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(0)
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, label)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType2)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType2RoundTripWithIPv4 verifies Type 2 with IPv4 round-trip.
//
// VALIDATES: Bytes() correctly encodes MAC+IPv4 routes.
// PREVENTS: IPv4 address corruption in MAC/IP advertisement routes.
func TestEVPNType2RoundTripWithIPv4(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+4+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, ip, label)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType2)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType2RoundTripWithIPv6 verifies Type 2 with IPv6 round-trip.
//
// VALIDATES: Bytes() correctly encodes MAC+IPv6 routes.
// PREVENTS: IPv6 address corruption in MAC/IP advertisement routes.
func TestEVPNType2RoundTripWithIPv6(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	macLen := byte(48)
	mac := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ipLen := byte(128)
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType2, byte(8+10+4+1+6+1+16+3),
		rd, esi, ethTag, []byte{macLen}, mac, []byte{ipLen}, ip, label)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType2)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType3RoundTripIPv4 verifies Type 3 with IPv4 round-trip.
//
// VALIDATES: Bytes() correctly encodes Inclusive Multicast routes.
// PREVENTS: BUM traffic flooding issues from malformed Type 3 routes.
func TestEVPNType3RoundTripIPv4(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}

	original := buildEVPNData(EVPNRouteType3, byte(8+4+1+4),
		rd, ethTag, []byte{ipLen}, ip)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType3)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType3RoundTripIPv6 verifies Type 3 with IPv6 round-trip.
//
// VALIDATES: Bytes() correctly encodes Inclusive Multicast routes with IPv6.
// PREVENTS: IPv6 originator address corruption in IMET routes.
func TestEVPNType3RoundTripIPv6(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	ipLen := byte(128)
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	original := buildEVPNData(EVPNRouteType3, byte(8+4+1+16),
		rd, ethTag, []byte{ipLen}, ip)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType3)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType4RoundTripIPv4 verifies Type 4 with IPv4 round-trip.
//
// VALIDATES: Bytes() correctly encodes Ethernet Segment routes.
// PREVENTS: DF election failures from malformed Type 4 routes.
func TestEVPNType4RoundTripIPv4(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ipLen := byte(32)
	ip := []byte{10, 0, 0, 1}

	original := buildEVPNData(EVPNRouteType4, byte(8+10+1+4),
		rd, esi, []byte{ipLen}, ip)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType4)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType4RoundTripIPv6 verifies Type 4 with IPv6 round-trip.
//
// VALIDATES: Bytes() correctly encodes Ethernet Segment routes with IPv6.
// PREVENTS: IPv6 originator address corruption in ES routes.
func TestEVPNType4RoundTripIPv6(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ipLen := byte(128)
	ip := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	original := buildEVPNData(EVPNRouteType4, byte(8+10+1+16),
		rd, esi, []byte{ipLen}, ip)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType4)
	require.True(t, ok)

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType5RoundTripIPv4 verifies Type 5 IPv4 round-trip.
//
// VALIDATES: Bytes() correctly encodes IP Prefix routes per RFC 9136.
// PREVENTS: IP prefix routes being malformed.
func TestEVPNType5RoundTripIPv4(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	prefixLen := byte(24)
	prefix := []byte{10, 1, 2, 0}
	gw := []byte{0, 0, 0, 0}
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType5, byte(34),
		rd, esi, ethTag, []byte{prefixLen}, prefix, gw, label)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType5)
	require.True(t, ok)

	// RFC requirement: RFC9136-3.1-2 positive -- gateway IP and IP prefix share the same address family (both IPv4)
	assert.Equal(t, evpn.Prefix().Addr().Is4(), evpn.Gateway().Is4(),
		"gateway IP address family must match the IP prefix address family")

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType5RoundTripIPv6 verifies Type 5 IPv6 round-trip.
//
// VALIDATES: Bytes() correctly encodes IPv6 prefix routes per RFC 9136.
// PREVENTS: IPv6 prefix routes being malformed.
func TestEVPNType5RoundTripIPv6(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := make([]byte, 10)
	ethTag := []byte{0x00, 0x00, 0x00, 0x00}
	prefixLen := byte(64)
	prefix := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	gw := make([]byte, 16)
	label := []byte{0x00, 0x01, 0x01}

	original := buildEVPNData(EVPNRouteType5, byte(58),
		rd, esi, ethTag, []byte{prefixLen}, prefix, gw, label)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType5)
	require.True(t, ok)

	// RFC requirement: RFC9136-3.1-2 positive -- gateway IP and IP prefix share the same address family (both IPv6)
	assert.Equal(t, evpn.Prefix().Addr().Is4(), evpn.Gateway().Is4(),
		"gateway IP address family must match the IP prefix address family")

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestEVPNType1RoundTripMultiLabel verifies Type 1 with label stack.
//
// VALIDATES: Bytes() correctly encodes multiple MPLS labels with BOS bit.
// PREVENTS: Label stack corruption breaking EVPN-MPLS forwarding.
func TestEVPNType1RoundTripMultiLabel(t *testing.T) {
	t.Parallel()
	rd := []byte{0x00, 0x00, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	esi := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ethTag := []byte{0x00, 0x00, 0x00, 0x0A}
	labels := []byte{0x00, 0x06, 0x40, 0x00, 0x0C, 0x81}

	original := buildEVPNData(EVPNRouteType1, byte(8+10+4+6),
		rd, esi, ethTag, labels)

	parsed, _, err := ParseEVPN(original, false)
	require.NoError(t, err)

	evpn, ok := parsed.(*EVPNType1)
	require.True(t, ok)
	assert.Equal(t, []uint32{100, 200}, evpn.Labels())

	encoded := evpn.Bytes()
	assert.Equal(t, original, encoded, "round-trip encoding mismatch")
}

// TestParseESIString verifies ESI string parsing.
//
// VALIDATES: ParseESIString handles all formats.
// PREVENTS: ESI string parsing bugs.
func TestParseESIString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected ESI
		wantErr  bool
	}{
		{"0", ESI{}, false},
		{"", ESI{}, false},
		{"00112233445566778899", ESI{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}, false},
		{"00:11:22:33:44:55:66:77:88:99", ESI{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}, false},
		{"00:00:00:00:00:00:00:00:00:00", ESI{}, false},
		{"ff:ff:ff:ff:ff:ff:ff:ff:ff:ff", ESI{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, false},
		{"FFFFFFFFFFFFFFFFFFFF", ESI{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, false},
		{"invalid", ESI{}, true},
		{"00:11:22", ESI{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result, err := ParseESIString(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEVPNType2StringCommandStyle verifies Type 2 String() outputs command-style format.
//
// VALIDATES: Type 2 output uses "mac-ip rd ... mac ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType2StringCommandStyle(t *testing.T) {
	t.Parallel()
	rd, _ := ParseRDString("65000:100")
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	e := NewEVPNType2(rd, [10]byte{}, 0, mac, netip.Addr{}, []uint32{1000})
	s := e.String()
	assert.Contains(t, s, "mac-ip", "should start with route type")
	assert.Contains(t, s, "rd 0:65000:100", "rd field should be present")
	assert.Contains(t, s, "mac 00:11:22:33:44:55", "mac field should be present")
}

// TestEVPNType2WithIPStringCommandStyle verifies Type 2 with IP uses command-style.
//
// VALIDATES: Type 2 with IP outputs proper format.
// PREVENTS: Missing IP in output when present.
func TestEVPNType2WithIPStringCommandStyle(t *testing.T) {
	t.Parallel()
	rd, _ := ParseRDString("65000:100")
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ip := netip.MustParseAddr("192.168.1.1")

	e := NewEVPNType2(rd, [10]byte{}, 100, mac, ip, []uint32{1000})
	s := e.String()
	assert.Contains(t, s, "mac-ip", "should start with route type")
	assert.Contains(t, s, "rd 0:65000:100", "rd field should be present")
	assert.Contains(t, s, "mac 00:11:22:33:44:55", "mac field should be present")
	assert.Contains(t, s, "ip 192.168.1.1", "ip field should be present")
	assert.Contains(t, s, "etag 100", "etag field should be present")
	assert.Contains(t, s, "label 1000", "label field should be present")
}

// TestEVPNType3StringCommandStyle verifies Type 3 String() outputs command-style format.
//
// VALIDATES: Type 3 output uses "multicast rd ... ip ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType3StringCommandStyle(t *testing.T) {
	t.Parallel()
	rd, _ := ParseRDString("65000:100")
	ip := netip.MustParseAddr("10.0.0.1")

	e := NewEVPNType3(rd, 200, ip)
	s := e.String()
	assert.Contains(t, s, "multicast", "should start with route type")
	assert.Contains(t, s, "rd 0:65000:100", "rd field should be present")
	assert.Contains(t, s, "ip 10.0.0.1", "ip field should be present")
	assert.Contains(t, s, "etag 200", "etag field should be present")
}

// TestEVPNType5StringCommandStyle verifies Type 5 String() outputs command-style format.
//
// VALIDATES: Type 5 output uses "ip-prefix rd ... prefix ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType5StringCommandStyle(t *testing.T) {
	t.Parallel()
	rd, _ := ParseRDString("65000:100")
	prefix := netip.MustParsePrefix("10.0.0.0/24")

	e := NewEVPNType5(rd, [10]byte{}, 0, prefix, netip.Addr{}, []uint32{1000})
	s := e.String()
	assert.Contains(t, s, "ip-prefix", "should start with route type")
	assert.Contains(t, s, "rd 0:65000:100", "rd field should be present")
	assert.Contains(t, s, "prefix 10.0.0.0/24", "prefix field should be present")
	assert.Contains(t, s, "label 1000", "label field should be present")
}

// TestEVPNType1StringCommandStyle verifies Type 1 String() outputs command-style format.
//
// VALIDATES: Type 1 output uses "ethernet-ad rd ... esi ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType1StringCommandStyle(t *testing.T) {
	t.Parallel()
	rd, _ := ParseRDString("65000:100")
	esi := [10]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}

	e := NewEVPNType1(rd, esi, 10, []uint32{1000})
	s := e.String()
	assert.Contains(t, s, "ethernet-ad", "should start with route type")
	assert.Contains(t, s, "rd 0:65000:100", "rd field should be present")
	assert.Contains(t, s, "esi 00:01:02:03:04:05:06:07:08:09", "esi field should be present")
	assert.Contains(t, s, "etag 10", "etag field should be present")
	assert.Contains(t, s, "label 1000", "label field should be present")
}

// TestEVPNType4StringCommandStyle verifies Type 4 String() outputs command-style format.
//
// VALIDATES: Type 4 output uses "ethernet-segment rd ... esi ..." format.
// PREVENTS: Output mismatch with input parser expectations.
func TestEVPNType4StringCommandStyle(t *testing.T) {
	t.Parallel()
	rd, _ := ParseRDString("65000:100")
	esi := [10]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	ip := netip.MustParseAddr("10.0.0.1")

	e := NewEVPNType4(rd, esi, ip)
	s := e.String()
	assert.Contains(t, s, "ethernet-segment", "should start with route type")
	assert.Contains(t, s, "rd 0:65000:100", "rd field should be present")
	assert.Contains(t, s, "esi 00:01:02:03:04:05:06:07:08:09", "esi field should be present")
	assert.Contains(t, s, "ip 10.0.0.1", "ip field should be present")
}

// TestLenWithContext_EVPN verifies that LenWithContext returns the same
// length as WriteNLRI actually writes for EVPN types.
//
// VALIDATES: Buffer size from LenWithContext is exactly what WriteNLRI needs.
// PREVENTS: Buffer overflow when WriteNLRI writes more than LenWithContext predicted.
func TestLenWithContext_EVPN(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		nlri nlri.NLRI
	}{
		{"EVPNType2_MAC", mustParseEVPNType2(t)},
		{"EVPNType5_Prefix", mustParseEVPNType5(t)},
	}

	addPathValues := []bool{false, true}

	for _, tt := range testCases {
		for _, addPath := range addPathValues {
			ctxName := "AddPath=false"
			if addPath {
				ctxName = "AddPath=true"
			}
			name := tt.name + "_" + ctxName

			t.Run(name, func(t *testing.T) {
				t.Parallel()
				predictedLen := nlri.LenWithContext(tt.nlri, addPath)
				buf := make([]byte, predictedLen+10)
				written := nlri.WriteNLRI(tt.nlri, buf, 0, addPath)

				if written != predictedLen {
					t.Errorf("LenWithContext=%d but WriteNLRI wrote %d bytes",
						predictedLen, written)
				}
			})
		}
	}
}

// TestWireFormat_EVPN verifies EVPN wire format with ADD-PATH.
//
// VALIDATES: EVPN wire format is [pathID][type][length][payload].
// PREVENTS: ADD-PATH encoding bugs.
func TestWireFormat_EVPN(t *testing.T) {
	t.Parallel()
	rd := RouteDistinguisher{Type: 0, Value: [6]byte{0, 0, 0, 0, 0, 1}}
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	tests := []struct {
		name    string
		nlri    nlri.NLRI
		addPath bool
		wantLen int
	}{
		{
			name:    "EVPNType2_noAddPath",
			nlri:    NewEVPNType2(rd, ESI{}, 0, mac, netip.MustParseAddr("10.0.0.1"), []uint32{100}),
			addPath: false,
			wantLen: 39,
		},
		{
			name:    "EVPNType2_withAddPath",
			nlri:    NewEVPNType2(rd, ESI{}, 0, mac, netip.MustParseAddr("10.0.0.1"), []uint32{100}),
			addPath: true,
			wantLen: 43,
		},
		{
			name:    "EVPNType5_noAddPath",
			nlri:    NewEVPNType5(rd, ESI{}, 0, netip.MustParsePrefix("10.0.0.0/24"), netip.Addr{}, []uint32{100}),
			addPath: false,
			wantLen: 36,
		},
		{
			name:    "EVPNType5_withAddPath",
			nlri:    NewEVPNType5(rd, ESI{}, 0, netip.MustParsePrefix("10.0.0.0/24"), netip.Addr{}, []uint32{100}),
			addPath: true,
			wantLen: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := make([]byte, 100)
			n := nlri.WriteNLRI(tt.nlri, buf, 0, tt.addPath)

			if n != tt.wantLen {
				t.Errorf("length = %d, want %d", n, tt.wantLen)
			}

			if tt.addPath {
				pathID := buf[0:4]
				if !bytes.Equal(pathID, []byte{0, 0, 0, 0}) {
					t.Errorf("path ID = %x, want 00000000", pathID)
				}
				if buf[4] != 2 && buf[4] != 5 {
					t.Errorf("EVPN type at wrong position, got %d", buf[4])
				}
			} else if buf[0] != 2 && buf[0] != 5 {
				t.Errorf("EVPN type = %d, want 2 or 5", buf[0])
			}
		})
	}
}

// TestRoundTrip_EVPN verifies EVPN encode -> decode -> encode.
//
// VALIDATES: EVPN routes preserve RD, MAC, IP, labels.
// PREVENTS: EVPN encoding corruption.
func TestRoundTrip_EVPN(t *testing.T) {
	t.Parallel()
	rd := RouteDistinguisher{Type: 0, Value: [6]byte{0, 0, 0, 0, 0, 1}}
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}

	tests := []struct {
		name    string
		nlri    nlri.NLRI
		addPath bool
	}{
		{"Type2_noPath", NewEVPNType2(rd, ESI{}, 0, mac, netip.MustParseAddr("10.0.0.1"), []uint32{100}), false},
		{"Type2_withPath", NewEVPNType2(rd, ESI{}, 0, mac, netip.MustParseAddr("10.0.0.1"), []uint32{100}), true},
		{"Type5_noPath", NewEVPNType5(rd, ESI{}, 0, netip.MustParsePrefix("10.0.0.0/24"), netip.Addr{}, []uint32{100}), false},
		{"Type5_withPath", NewEVPNType5(rd, ESI{}, 0, netip.MustParsePrefix("10.0.0.0/24"), netip.Addr{}, []uint32{100}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := make([]byte, 100)
			n := nlri.WriteNLRI(tt.nlri, buf, 0, tt.addPath)
			wire := buf[:n]

			parsed, remaining, err := ParseEVPN(wire, tt.addPath)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			consumed := n - len(remaining)
			if consumed != n {
				t.Fatalf("consumed %d bytes, wrote %d", consumed, n)
			}

			buf2 := make([]byte, 100)
			n2 := nlri.WriteNLRI(parsed, buf2, 0, tt.addPath)
			wire2 := buf2[:n2]

			if !bytes.Equal(wire, wire2) {
				t.Errorf("round-trip mismatch:\n  orig: %x\n  trip: %x", wire, wire2)
			}
		})
	}
}

// mustParseEVPNType2 creates an EVPN Type 2 (MAC/IP) NLRI for testing.
func mustParseEVPNType2(t *testing.T) *EVPNType2 {
	t.Helper()
	rd := RouteDistinguisher{Type: 0, Value: [6]byte{0, 0, 0, 0, 0, 1}}
	mac := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	ip := netip.MustParseAddr("10.0.0.1")
	return NewEVPNType2(rd, ESI{}, 0, mac, ip, []uint32{100})
}

// mustParseEVPNType5 creates an EVPN Type 5 (IP Prefix) NLRI for testing.
func mustParseEVPNType5(t *testing.T) *EVPNType5 {
	t.Helper()
	rd := RouteDistinguisher{Type: 0, Value: [6]byte{0, 0, 0, 0, 0, 1}}
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	return NewEVPNType5(rd, ESI{}, 0, prefix, netip.Addr{}, []uint32{100})
}
