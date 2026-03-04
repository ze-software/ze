package reactor

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
)

// VALIDATES: Zero-allocation attribute writers produce correct RFC wire format.
// PREVENTS: Breaking changes to wire encoding that cause peer rejection.

// TestWriteOriginAttr verifies ORIGIN attribute wire encoding.
// RFC 4271 §5.1.1: Well-known mandatory, Transitive, code 1, 1-byte value.
func TestWriteOriginAttr(t *testing.T) {
	tests := []struct {
		name     string
		origin   uint8
		expected string // hex
	}{
		{"IGP", uint8(attribute.OriginIGP), "40010100"},
		{"EGP", uint8(attribute.OriginEGP), "40010101"},
		{"Incomplete", uint8(attribute.OriginIncomplete), "40010102"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := writeOriginAttr(buf, 0, tt.origin)
			assert.Equal(t, 4, n)

			expected, _ := hex.DecodeString(tt.expected)
			assert.Equal(t, expected, buf[:n])
		})
	}
}

// TestWriteOriginAttr_Offset verifies writing at non-zero offset.
// VALIDATES: Offset parameter correctly positions output in buffer.
func TestWriteOriginAttr_Offset(t *testing.T) {
	buf := make([]byte, 64)
	buf[0] = 0xAA // sentinel

	n := writeOriginAttr(buf, 10, uint8(attribute.OriginIGP))
	assert.Equal(t, 4, n)
	assert.Equal(t, byte(0xAA), buf[0], "sentinel untouched")
	assert.Equal(t, byte(0x40), buf[10], "flags at offset")
}

// TestWriteASPathAttr_Empty verifies empty AS_PATH (iBGP).
// RFC 4271 §5.1.2: iBGP sessions use empty AS_PATH.
func TestWriteASPathAttr_Empty(t *testing.T) {
	buf := make([]byte, 64)
	n := writeASPathAttr(buf, 0, nil, true)

	// Flags(0x40) + Code(2) + Len(0)
	expected, _ := hex.DecodeString("400200")
	assert.Equal(t, 3, n)
	assert.Equal(t, expected, buf[:n])
}

// TestWriteASPathAttr_SingleAS_ASN4 verifies single AS in 4-byte mode.
// RFC 6793: 4-byte ASN encoding.
func TestWriteASPathAttr_SingleAS_ASN4(t *testing.T) {
	buf := make([]byte, 64)
	n := writeASPathAttr(buf, 0, []uint32{65001}, true)

	// Flags(0x40) + Code(2) + Len(6) + Segment: Type(2=SEQ) + Count(1) + ASN(4 bytes)
	expected, _ := hex.DecodeString("40020602010000fde9")
	assert.Equal(t, len(expected), n)
	assert.Equal(t, expected, buf[:n])
}

// TestWriteASPathAttr_SingleAS_ASN2 verifies single AS in 2-byte mode.
// RFC 4271 §4.3: 2-byte AS number encoding.
func TestWriteASPathAttr_SingleAS_ASN2(t *testing.T) {
	buf := make([]byte, 64)
	n := writeASPathAttr(buf, 0, []uint32{65001}, false)

	// Flags(0x40) + Code(2) + Len(4) + Segment: Type(2=SEQ) + Count(1) + ASN(2 bytes: 65001=0xFDE9)
	expected, _ := hex.DecodeString("40020402" + "01" + "fde9")
	assert.Equal(t, len(expected), n)
	assert.Equal(t, expected, buf[:n])
}

// TestWriteASPathAttr_AS_TRANS verifies AS_TRANS mapping for large ASNs in 2-byte mode.
// RFC 6793: ASN > 65535 maps to 23456 (AS_TRANS) in 2-byte encoding.
func TestWriteASPathAttr_AS_TRANS(t *testing.T) {
	buf := make([]byte, 64)
	n := writeASPathAttr(buf, 0, []uint32{100000}, false)

	// ASN 100000 > 65535 → mapped to 23456 (0x5BA0)
	expected, _ := hex.DecodeString("40020402015ba0")
	assert.Equal(t, len(expected), n)
	assert.Equal(t, expected, buf[:n])
}

// TestWriteASPathAttr_MultipleASNs verifies multiple ASNs in path.
func TestWriteASPathAttr_MultipleASNs(t *testing.T) {
	buf := make([]byte, 128)
	n := writeASPathAttr(buf, 0, []uint32{65001, 65002, 65003}, true)

	// Header(3) + Segment header(2) + 3 * 4 bytes = 17
	assert.Equal(t, 17, n)

	// Verify segment: type=2(SEQ), count=3
	assert.Equal(t, byte(attribute.ASSequence), buf[3])
	assert.Equal(t, byte(3), buf[4])

	// Verify each ASN
	assert.Equal(t, uint32(65001), binary.BigEndian.Uint32(buf[5:9]))
	assert.Equal(t, uint32(65002), binary.BigEndian.Uint32(buf[9:13]))
	assert.Equal(t, uint32(65003), binary.BigEndian.Uint32(buf[13:17]))
}

// TestWriteNextHopAttr verifies NEXT_HOP attribute wire encoding.
// RFC 4271 §5.1.3: Well-known mandatory, 4 bytes for IPv4.
func TestWriteNextHopAttr(t *testing.T) {
	buf := make([]byte, 64)
	addr := netip.MustParseAddr("192.168.1.1")
	n := writeNextHopAttr(buf, 0, addr)

	expected, _ := hex.DecodeString("400304c0a80101")
	assert.Equal(t, 7, n)
	assert.Equal(t, expected, buf[:n])
}

// TestWriteMEDAttr verifies MED attribute wire encoding.
// RFC 4271 §5.1.4: Optional non-transitive, 4 bytes.
func TestWriteMEDAttr(t *testing.T) {
	tests := []struct {
		name     string
		med      uint32
		expected string
	}{
		{"zero", 0, "80040400000000"},
		{"100", 100, "80040400000064"},
		{"max", 0xFFFFFFFF, "8004040000ffff" + "ffff"}, // need to fix this
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 64)
			n := writeMEDAttr(buf, 0, tt.med)
			assert.Equal(t, 7, n)

			// Verify flags (Optional=0x80), code (4), len (4)
			assert.Equal(t, byte(0x80), buf[0])
			assert.Equal(t, byte(attribute.AttrMED), buf[1])
			assert.Equal(t, byte(4), buf[2])
			assert.Equal(t, tt.med, binary.BigEndian.Uint32(buf[3:7]))
		})
	}
}

// TestWriteLocalPrefAttr verifies LOCAL_PREF attribute wire encoding.
// RFC 4271 §5.1.5: Well-known for iBGP, Transitive, 4 bytes.
func TestWriteLocalPrefAttr(t *testing.T) {
	buf := make([]byte, 64)
	n := writeLocalPrefAttr(buf, 0, 200)

	expected, _ := hex.DecodeString("400504000000c8")
	assert.Equal(t, 7, n)
	assert.Equal(t, expected, buf[:n])
}

// TestWriteCommunitiesAttr_Single verifies single community encoding.
// RFC 1997: Optional transitive, 4 bytes per community.
func TestWriteCommunitiesAttr_Single(t *testing.T) {
	buf := make([]byte, 64)
	n := writeCommunitiesAttr(buf, 0, []uint32{0xFFFF0001})

	// Flags(0xC0=Optional|Transitive) + Code(8) + Len(4) + Community
	expected, _ := hex.DecodeString("c00804ffff0001")
	assert.Equal(t, 7, n)
	assert.Equal(t, expected, buf[:n])
}

// TestWriteCommunitiesAttr_Multiple verifies multiple communities.
func TestWriteCommunitiesAttr_Multiple(t *testing.T) {
	buf := make([]byte, 64)
	comms := []uint32{0xFFFF0001, 0xFFFF0002, 0x00010064}
	n := writeCommunitiesAttr(buf, 0, comms)

	// 3 communities × 4 = 12 bytes value
	assert.Equal(t, 3+12, n) // header(3) + value(12)
	assert.Equal(t, byte(12), buf[2])

	// Verify each community
	assert.Equal(t, uint32(0xFFFF0001), binary.BigEndian.Uint32(buf[3:7]))
	assert.Equal(t, uint32(0xFFFF0002), binary.BigEndian.Uint32(buf[7:11]))
	assert.Equal(t, uint32(0x00010064), binary.BigEndian.Uint32(buf[11:15]))
}

// TestWriteCommunitiesAttr_ExtendedLength verifies extended length for >63 communities.
// RFC 4271 §4.3: Extended length when value > 255 bytes.
func TestWriteCommunitiesAttr_ExtendedLength(t *testing.T) {
	// 64 communities × 4 = 256 bytes > 255, triggers extended length
	comms := make([]uint32, 64)
	for i := range comms {
		comms[i] = uint32(i + 1)
	}

	buf := make([]byte, 4096)
	n := writeCommunitiesAttr(buf, 0, comms)

	// Extended length: Flags(0xD0=Optional|Transitive|ExtLen) + Code(8) + Len(2 bytes) + value
	assert.Equal(t, 4+256, n)           // extended header(4) + value(256)
	assert.Equal(t, byte(0xD0), buf[0]) // Optional | Transitive | ExtLength
	assert.Equal(t, byte(attribute.AttrCommunity), buf[1])
	assert.Equal(t, uint16(256), binary.BigEndian.Uint16(buf[2:4]))
}

// TestWriteAnnounceUpdate_IPv4_iBGP verifies full IPv4 iBGP announce UPDATE.
// VALIDATES: Complete message format per RFC 4271 §4.3.
func TestWriteAnnounceUpdate_IPv4_iBGP(t *testing.T) {
	buf := make([]byte, 4096)

	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
	}

	n := WriteAnnounceUpdate(buf, 0, route, 65001, true, true, false)
	require.Greater(t, n, message.MarkerLen+2+1) // at minimum: marker + len + type

	// Verify marker (16 × 0xFF)
	for i := range message.MarkerLen {
		assert.Equal(t, byte(0xFF), buf[i], "marker byte %d", i)
	}

	// Verify message type = UPDATE (2)
	assert.Equal(t, byte(message.TypeUPDATE), buf[message.MarkerLen+2])

	// Verify withdrawn routes length = 0
	assert.Equal(t, byte(0), buf[message.MarkerLen+3])
	assert.Equal(t, byte(0), buf[message.MarkerLen+4])

	// Verify total length matches return value
	totalLen := int(buf[message.MarkerLen])<<8 | int(buf[message.MarkerLen+1])
	assert.Equal(t, n, totalLen)

	// iBGP should have empty AS_PATH and LOCAL_PREF=100
	// Look for AS_PATH: 0x40 0x02 0x00 (empty)
	found := false
	for i := range n - 2 {
		if buf[i] == 0x40 && buf[i+1] == byte(attribute.AttrASPath) && buf[i+2] == 0x00 {
			found = true
			break
		}
	}
	assert.True(t, found, "empty AS_PATH not found for iBGP")
}

// TestWriteAnnounceUpdate_IPv4_eBGP verifies full IPv4 eBGP announce UPDATE.
// RFC 4271 §5.1.2: eBGP MUST prepend local AS to AS_PATH.
func TestWriteAnnounceUpdate_IPv4_eBGP(t *testing.T) {
	buf := make([]byte, 4096)

	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
	}

	n := WriteAnnounceUpdate(buf, 0, route, 65001, false, true, false)
	require.Greater(t, n, 0)

	// eBGP should have AS_PATH with local AS 65001
	// Look for AS_PATH segment: Type(2=SEQ) + Count(1) + ASN(65001=0x0000FDE9)
	found := false
	for i := range n - 6 {
		if buf[i] == byte(attribute.ASSequence) && buf[i+1] == 1 {
			asn := binary.BigEndian.Uint32(buf[i+2 : i+6])
			if asn == 65001 {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "AS_PATH with local AS 65001 not found for eBGP")
}

// TestWriteAnnounceUpdate_IPv6 verifies IPv6 announce uses MP_REACH_NLRI.
// RFC 4760 §3: IPv6 routes encoded via MP_REACH_NLRI attribute.
func TestWriteAnnounceUpdate_IPv6(t *testing.T) {
	buf := make([]byte, 4096)

	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	n := WriteAnnounceUpdate(buf, 0, route, 65001, true, true, false)
	require.Greater(t, n, 0)

	// Look for MP_REACH_NLRI attribute header (code 14, optional)
	foundMP := false
	for i := message.MarkerLen + 5; i < n-3; i++ {
		if buf[i+1] == byte(attribute.AttrMPReachNLRI) {
			foundMP = true
			break
		}
	}
	assert.True(t, foundMP, "MP_REACH_NLRI attribute not found for IPv6")
}

// TestWriteWithdrawUpdate_IPv4 verifies IPv4 withdrawal UPDATE.
// RFC 4271 §4.3: IPv4 withdrawals use Withdrawn Routes field.
func TestWriteWithdrawUpdate_IPv4(t *testing.T) {
	buf := make([]byte, 4096)

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	n := WriteWithdrawUpdate(buf, 0, prefix, false)
	require.Greater(t, n, 0)

	// Verify marker
	for i := range message.MarkerLen {
		assert.Equal(t, byte(0xFF), buf[i])
	}

	// Verify type = UPDATE
	assert.Equal(t, byte(message.TypeUPDATE), buf[message.MarkerLen+2])

	// Verify withdrawn routes length > 0
	withdrawnLen := int(buf[message.MarkerLen+3])<<8 | int(buf[message.MarkerLen+4])
	assert.Greater(t, withdrawnLen, 0, "IPv4 withdrawal should have non-zero withdrawn routes length")

	// Verify total path attributes length = 0 (withdrawal only)
	attrLenPos := message.MarkerLen + 5 + withdrawnLen
	attrLen := int(buf[attrLenPos])<<8 | int(buf[attrLenPos+1])
	assert.Equal(t, 0, attrLen, "withdrawal should have zero path attributes")
}

// TestWriteWithdrawUpdate_IPv6 verifies IPv6 withdrawal uses MP_UNREACH_NLRI.
// RFC 4760 §4: IPv6 withdrawals use MP_UNREACH_NLRI attribute.
func TestWriteWithdrawUpdate_IPv6(t *testing.T) {
	buf := make([]byte, 4096)

	prefix := netip.MustParsePrefix("2001:db8::/32")
	n := WriteWithdrawUpdate(buf, 0, prefix, false)
	require.Greater(t, n, 0)

	// Verify withdrawn routes length = 0 (using MP_UNREACH instead)
	assert.Equal(t, byte(0), buf[message.MarkerLen+3])
	assert.Equal(t, byte(0), buf[message.MarkerLen+4])

	// Verify path attributes length > 0 (contains MP_UNREACH_NLRI)
	attrLenPos := message.MarkerLen + 5
	attrLen := int(buf[attrLenPos])<<8 | int(buf[attrLenPos+1])
	assert.Greater(t, attrLen, 0, "IPv6 withdrawal should have MP_UNREACH_NLRI attribute")
}

// TestWriteWithdrawUpdate_MessageLength verifies total length field consistency.
// VALIDATES: Length field in BGP header matches actual bytes written.
func TestWriteWithdrawUpdate_MessageLength(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		addPath bool
	}{
		{"IPv4/24", "10.0.0.0/24", false},
		{"IPv4/8", "10.0.0.0/8", false},
		{"IPv4/32", "10.0.0.1/32", false},
		{"IPv6/32", "2001:db8::/32", false},
		{"IPv6/128", "2001:db8::1/128", false},
		{"IPv4/24 AddPath", "10.0.0.0/24", true},
		{"IPv6/32 AddPath", "2001:db8::/32", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 4096)
			prefix := netip.MustParsePrefix(tt.prefix)
			n := WriteWithdrawUpdate(buf, 0, prefix, tt.addPath)

			headerLen := int(buf[message.MarkerLen])<<8 | int(buf[message.MarkerLen+1])
			assert.Equal(t, n, headerLen, "header length must match bytes written")
		})
	}
}

// TestWriteAnnounceUpdate_MessageLength verifies announce message length consistency.
func TestWriteAnnounceUpdate_MessageLength(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		nhop    string
		isIBGP  bool
		addPath bool
	}{
		{"IPv4 iBGP", "10.0.0.0/24", "192.168.1.1", true, false},
		{"IPv4 eBGP", "10.0.0.0/24", "192.168.1.1", false, false},
		{"IPv6 iBGP", "2001:db8::/32", "2001:db8::1", true, false},
		{"IPv4 AddPath", "10.0.0.0/24", "192.168.1.1", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 4096)
			route := bgptypes.RouteSpec{
				Prefix:  netip.MustParsePrefix(tt.prefix),
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(tt.nhop)),
			}

			n := WriteAnnounceUpdate(buf, 0, route, 65001, tt.isIBGP, true, tt.addPath)
			headerLen := int(buf[message.MarkerLen])<<8 | int(buf[message.MarkerLen+1])
			assert.Equal(t, n, headerLen, "header length must match bytes written")
		})
	}
}
