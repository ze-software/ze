package capability

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCapabilityCodeConstants verifies capability code values match RFCs.
//
// VALIDATES: Capability codes are correct per IANA assignments.
//
// PREVENTS: Protocol errors from wrong capability codes.
func TestCapabilityCodeConstants(t *testing.T) {
	tests := []struct {
		code Code
		val  uint8
		name string
	}{
		{CodeMultiprotocol, 1, "Multiprotocol"},
		{CodeRouteRefresh, 2, "Route Refresh"},
		{CodeExtendedNextHop, 5, "Extended Next Hop"},
		{CodeExtendedMessage, 6, "Extended Message"},
		{CodeGracefulRestart, 64, "Graceful Restart"},
		{CodeASN4, 65, "4-Byte AS"},
		{CodeAddPath, 69, "ADD-PATH"},
		{CodeFQDN, 73, "FQDN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.val, uint8(tt.code))
		})
	}
}

// TestCodeString verifies human-readable capability code names.
//
// VALIDATES: Debug output is readable.
//
// PREVENTS: Opaque numeric codes in logs.
func TestCodeString(t *testing.T) {
	assert.Equal(t, "Multiprotocol(1)", CodeMultiprotocol.String())
	assert.Equal(t, "ASN4(65)", CodeASN4.String())
	assert.Equal(t, "Unknown(99)", Code(99).String())
}

// TestParseCapabilities verifies parsing of capability TLVs.
//
// VALIDATES: Correct parsing of capability parameters.
//
// PREVENTS: Capability negotiation failures from parse errors.
func TestParseCapabilities(t *testing.T) {
	// Two capabilities: Multiprotocol IPv4/Unicast + ASN4
	data := []byte{
		// Capability 1: Multiprotocol
		0x01,       // Code = Multiprotocol
		0x04,       // Length = 4
		0x00, 0x01, // AFI = IPv4
		0x00, // Reserved
		0x01, // SAFI = Unicast
		// Capability 2: ASN4
		0x41,                   // Code = ASN4 (65)
		0x04,                   // Length = 4
		0x00, 0x01, 0x00, 0x01, // AS 65537
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 2)

	// Check first capability
	mp, ok := caps[0].(*Multiprotocol)
	require.True(t, ok, "first should be Multiprotocol")
	assert.Equal(t, AFIIPv4, mp.AFI)
	assert.Equal(t, SAFIUnicast, mp.SAFI)

	// Check second capability
	asn4, ok := caps[1].(*ASN4)
	require.True(t, ok, "second should be ASN4")
	assert.Equal(t, uint32(65537), asn4.ASN)
}

// TestParseEmpty verifies parsing empty capability data.
//
// VALIDATES: Edge case - no capabilities.
//
// PREVENTS: Panic on empty input.
func TestParseEmpty(t *testing.T) {
	caps, err := Parse(nil)
	require.NoError(t, err)
	require.Len(t, caps, 0)

	caps, err = Parse([]byte{})
	require.NoError(t, err)
	require.Len(t, caps, 0)
}

// TestParseTruncated verifies error on truncated data.
//
// VALIDATES: Malformed data detection.
//
// PREVENTS: Buffer overread from malicious/corrupted packets.
func TestParseTruncated(t *testing.T) {
	// Length says 4 bytes but only 2 provided
	data := []byte{0x01, 0x04, 0x00, 0x01}

	_, err := Parse(data)
	require.Error(t, err)
}

// TestParseUnknownCapability verifies unknown capabilities are preserved.
//
// VALIDATES: Forward compatibility with new capabilities.
//
// PREVENTS: Connection failures when peer sends unknown capability.
func TestParseUnknownCapability(t *testing.T) {
	data := []byte{
		0xFE,       // Unknown code 254
		0x02,       // Length = 2
		0xAB, 0xCD, // Random data
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	unknown, ok := caps[0].(*Unknown)
	require.True(t, ok)
	assert.Equal(t, Code(254), unknown.Code())
	assert.Equal(t, []byte{0xAB, 0xCD}, unknown.Data)
}

// TestAddPathCapability verifies ADD-PATH parsing and packing (RFC 7911).
//
// VALIDATES: ADD-PATH capability handling for path diversity.
//
// PREVENTS: Route selection issues when multiple paths are available.
func TestAddPathCapability(t *testing.T) {
	// ADD-PATH for IPv4 Unicast: send+receive
	data := []byte{
		0x45,       // Code = ADD-PATH (69)
		0x04,       // Length = 4
		0x00, 0x01, // AFI = IPv4
		0x01, // SAFI = Unicast
		0x03, // Flags = Send+Receive
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	addpath, ok := caps[0].(*AddPath)
	require.True(t, ok)
	require.Len(t, addpath.Families, 1)
	assert.Equal(t, AFIIPv4, addpath.Families[0].AFI)
	assert.Equal(t, SAFIUnicast, addpath.Families[0].SAFI)
	assert.Equal(t, AddPathBoth, addpath.Families[0].Mode)
}

// TestAddPathMultipleFamilies verifies ADD-PATH with multiple families.
//
// VALIDATES: Multiple AFI/SAFI in single capability.
//
// PREVENTS: Only first family being parsed.
func TestAddPathMultipleFamilies(t *testing.T) {
	data := []byte{
		0x45,       // Code = ADD-PATH (69)
		0x08,       // Length = 8 (2 families * 4 bytes)
		0x00, 0x01, // AFI = IPv4
		0x01,       // SAFI = Unicast
		0x03,       // Flags = Both
		0x00, 0x02, // AFI = IPv6
		0x01, // SAFI = Unicast
		0x01, // Flags = Receive only
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	addpath, ok := caps[0].(*AddPath)
	require.True(t, ok)
	require.Len(t, addpath.Families, 2)

	assert.Equal(t, AFIIPv4, addpath.Families[0].AFI)
	assert.Equal(t, AddPathBoth, addpath.Families[0].Mode)

	assert.Equal(t, AFIIPv6, addpath.Families[1].AFI)
	assert.Equal(t, AddPathReceive, addpath.Families[1].Mode)
}

// TestGracefulRestartCapability verifies Graceful Restart parsing (RFC 4724).
//
// VALIDATES: Graceful restart capability handling.
//
// PREVENTS: Session drops during BGP restart.
func TestGracefulRestartCapability(t *testing.T) {
	data := []byte{
		0x40,       // Code = Graceful Restart (64)
		0x06,       // Length = 6
		0x80, 0x78, // Flags=Restart, Time=120s
		0x00, 0x01, // AFI = IPv4
		0x01, // SAFI = Unicast
		0x80, // AFI Flags = Forwarding State preserved
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	gr, ok := caps[0].(*GracefulRestart)
	require.True(t, ok)
	assert.True(t, gr.RestartState)
	assert.Equal(t, uint16(120), gr.RestartTime)
	require.Len(t, gr.Families, 1)
	assert.Equal(t, AFIIPv4, gr.Families[0].AFI)
	assert.True(t, gr.Families[0].ForwardingState)
}

// TestCapabilityRoundTrip verifies pack/parse round-trip.
//
// VALIDATES: Serialization correctness.
//
// PREVENTS: Data corruption during pack/parse cycle.
func TestCapabilityRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		cap  Capability
	}{
		{"Multiprotocol", &Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast}},
		{"ASN4", &ASN4{ASN: 4200000001}},
		{"RouteRefresh", &RouteRefresh{}},
		{"ExtendedMessage", &ExtendedMessage{}},
		{"AddPath", &AddPath{Families: []AddPathFamily{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth},
		}}},
		{"ExtendedNextHop", &ExtendedNextHop{Families: []ExtendedNextHopFamily{
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
		}}},
		{"FQDN", &FQDN{Hostname: "router1", DomainName: "example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed := make([]byte, tt.cap.Len())
			tt.cap.WriteTo(packed, 0)
			parsed, err := Parse(packed)
			require.NoError(t, err)
			require.Len(t, parsed, 1)
			assert.Equal(t, tt.cap.Code(), parsed[0].Code())
		})
	}
}

// TestExtendedNextHopCapability verifies Extended Next Hop parsing (RFC 8950).
//
// VALIDATES: IPv4 NLRI with IPv6 next-hop capability.
//
// PREVENTS: Routing failures on IPv6-only networks.
func TestExtendedNextHopCapability(t *testing.T) {
	data := []byte{
		0x05,       // Code = Extended Next Hop (5)
		0x06,       // Length = 6
		0x00, 0x01, // NLRI AFI = IPv4
		0x00, 0x01, // NLRI SAFI = Unicast
		0x00, 0x02, // Next Hop AFI = IPv6
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	enh, ok := caps[0].(*ExtendedNextHop)
	require.True(t, ok)
	require.Len(t, enh.Families, 1)
	assert.Equal(t, AFIIPv4, enh.Families[0].NLRIAFI)
	assert.Equal(t, SAFIUnicast, enh.Families[0].NLRISAFI)
	assert.Equal(t, AFIIPv6, enh.Families[0].NextHopAFI)
}

// TestExtendedNextHopRoundTrip verifies Extended Next Hop pack/parse.
func TestExtendedNextHopRoundTrip(t *testing.T) {
	original := &ExtendedNextHop{
		Families: []ExtendedNextHopFamily{
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIVPN, NextHopAFI: AFIIPv6},
		},
	}

	packed := make([]byte, original.Len())
	original.WriteTo(packed, 0)
	parsed, err := Parse(packed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	enh, ok := parsed[0].(*ExtendedNextHop)
	require.True(t, ok)
	require.Len(t, enh.Families, 2)

	assert.Equal(t, original.Families[0].NLRIAFI, enh.Families[0].NLRIAFI)
	assert.Equal(t, original.Families[0].NextHopAFI, enh.Families[0].NextHopAFI)
	assert.Equal(t, original.Families[1].NLRIAFI, enh.Families[1].NLRIAFI)
	assert.Equal(t, original.Families[1].NLRISAFI, enh.Families[1].NLRISAFI)
}

// TestFQDNCapability verifies FQDN parsing (RFC 8516).
//
// VALIDATES: FQDN capability for hostname advertisement.
//
// PREVENTS: Missing hostname in BGP sessions.
func TestFQDNCapability(t *testing.T) {
	// FQDN: hostname="router1", domain="example.com"
	data := []byte{
		0x49,                              // Code = FQDN (73)
		0x14,                              // Length = 20
		0x07,                              // Hostname length = 7
		'r', 'o', 'u', 't', 'e', 'r', '1', // Hostname
		0x0b,                                                  // Domain length = 11
		'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', // Domain
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	fqdn, ok := caps[0].(*FQDN)
	require.True(t, ok)
	assert.Equal(t, "router1", fqdn.Hostname)
	assert.Equal(t, "example.com", fqdn.DomainName)
}

// TestFQDNRoundTrip verifies FQDN pack/parse.
func TestFQDNRoundTrip(t *testing.T) {
	original := &FQDN{
		Hostname:   "bgp-speaker-01",
		DomainName: "datacenter.internal",
	}

	packed := make([]byte, original.Len())
	original.WriteTo(packed, 0)
	parsed, err := Parse(packed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	fqdn, ok := parsed[0].(*FQDN)
	require.True(t, ok)
	assert.Equal(t, original.Hostname, fqdn.Hostname)
	assert.Equal(t, original.DomainName, fqdn.DomainName)
}

// TestFQDNEmpty verifies FQDN with empty fields.
func TestFQDNEmpty(t *testing.T) {
	original := &FQDN{
		Hostname:   "",
		DomainName: "",
	}

	packed := make([]byte, original.Len())
	original.WriteTo(packed, 0)
	parsed, err := Parse(packed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	fqdn, ok := parsed[0].(*FQDN)
	require.True(t, ok)
	assert.Equal(t, "", fqdn.Hostname)
	assert.Equal(t, "", fqdn.DomainName)
}

// TestCapabilityWriteTo verifies WriteTo produces correct parseable TLV bytes
// for all 12 capability types (11 standard + Plugin).
//
// VALIDATES: WriteTo(buf, off) writes correct TLV bytes matching Len().
// PREVENTS: Encoding errors in buffer-first WriteTo path.
func TestCapabilityWriteTo(t *testing.T) {
	caps := []Capability{
		&Unknown{code: 99, Data: []byte{0x01, 0x02, 0x03}},
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIEVPN},
		&ASN4{ASN: 65533},
		&ASN4{ASN: 4200000000},
		&RouteRefresh{},
		&ExtendedMessage{},
		&EnhancedRouteRefresh{},
		&AddPath{Families: []AddPathFamily{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth},
			{AFI: AFIIPv6, SAFI: SAFIUnicast, Mode: AddPathReceive},
		}},
		&GracefulRestart{
			RestartState: true, RestartTime: 120,
			Families: []GracefulRestartFamily{
				{AFI: AFIIPv4, SAFI: SAFIUnicast, ForwardingState: true},
				{AFI: AFIIPv6, SAFI: SAFIUnicast, ForwardingState: false},
			},
		},
		&ExtendedNextHop{Families: []ExtendedNextHopFamily{
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
		}},
		&FQDN{Hostname: "router1", DomainName: "example.com"},
		&FQDN{Hostname: "", DomainName: ""},
		NewPlugin(99, []byte{0xDE, 0xAD}),
	}

	for _, c := range caps {
		name := fmt.Sprintf("%T/code=%d", c, c.Code())
		t.Run(name, func(t *testing.T) {
			buf := make([]byte, c.Len())
			n := c.WriteTo(buf, 0)

			// WriteTo must write exactly Len() bytes
			assert.Equal(t, c.Len(), n, "WriteTo returned wrong count")

			// Result must be parseable back to same capability code
			parsed, err := Parse(buf[:n])
			require.NoError(t, err, "WriteTo output must be parseable")
			require.Len(t, parsed, 1, "WriteTo output must contain exactly one capability")
			assert.Equal(t, c.Code(), parsed[0].Code(), "parsed code mismatch")
		})
	}
}

// TestCapabilityWriteToAtOffset verifies WriteTo respects the offset parameter
// and doesn't corrupt surrounding bytes.
//
// VALIDATES: WriteTo writes at the specified offset, not at position 0.
// PREVENTS: Off-by-one or ignored offset in WriteTo implementations.
func TestCapabilityWriteToAtOffset(t *testing.T) {
	caps := []Capability{
		&ASN4{ASN: 65533},
		&FQDN{Hostname: "test", DomainName: "example.com"},
		&AddPath{Families: []AddPathFamily{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth},
		}},
		NewPlugin(42, []byte{0x01, 0x02, 0x03}),
	}

	for _, c := range caps {
		name := fmt.Sprintf("%T/offset", c)
		t.Run(name, func(t *testing.T) {
			// Get reference bytes from WriteTo at offset 0
			ref := make([]byte, c.Len())
			c.WriteTo(ref, 0)

			offset := 10

			buf := make([]byte, offset+c.Len()+5)
			// Fill with sentinel
			for i := range buf {
				buf[i] = 0xFF
			}

			n := c.WriteTo(buf, offset)
			assert.Equal(t, len(ref), n)
			assert.Equal(t, ref, buf[offset:offset+n])

			// Sentinels before and after must be preserved
			for i := range offset {
				assert.Equal(t, byte(0xFF), buf[i], "byte before offset corrupted at %d", i)
			}
			for i := offset + n; i < len(buf); i++ {
				assert.Equal(t, byte(0xFF), buf[i], "byte after data corrupted at %d", i)
			}
		})
	}
}
