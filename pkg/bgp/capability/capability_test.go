package capability

import (
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
		{CodeSoftwareVersion, 75, "Software Version"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed := tt.cap.Pack()
			parsed, err := Parse(packed)
			require.NoError(t, err)
			require.Len(t, parsed, 1)
			assert.Equal(t, tt.cap.Code(), parsed[0].Code())
		})
	}
}
