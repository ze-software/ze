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
//
// RFC requirement: RFC5549-4-3 positive -- the Extended Next Hop Encoding capability is bound to
// code 5: this table asserts uint8(CodeExtendedNextHop) == 5 (internal/core/bgp/capability/capability.go:70).
func TestCapabilityCodeConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code Code
		val  uint8
		name string
	}{
		{CodeMultiprotocol, 1, "Multiprotocol"},
		{CodeRouteRefresh, 2, "Route Refresh"},
		{CodeExtendedNextHop, 5, "Extended Next Hop"},
		// RFC requirement: RFC8654-3-2 positive -- the Extended Message capability is bound to
		// code 6: this case asserts uint8(CodeExtendedMessage) == 6 (internal/core/bgp/capability/capability.go:71).
		{CodeExtendedMessage, 6, "Extended Message"},
		{CodeGracefulRestart, 64, "Graceful Restart"},
		{CodeASN4, 65, "4-Byte AS"},
		{CodeAddPath, 69, "ADD-PATH"},
		{CodeFQDN, 73, "FQDN"},
		{CodePathsLimit, 76, "PATHS-LIMIT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	assert.Equal(t, "Multiprotocol(1)", CodeMultiprotocol.String())
	assert.Equal(t, "ASN4(65)", CodeASN4.String())
	assert.Equal(t, "Unknown(99)", Code(99).String())
}

// TestParseCapabilities verifies parsing of capability TLVs.
//
// VALIDATES: Correct parsing of capability parameters.
//
// PREVENTS: Capability negotiation failures from parse errors.
//
// RFC requirement: RFC4760-8-1 positive -- a Multiprotocol Extensions capability (Code 1,
// Length 4, AFI/Reserved/SAFI per RFC 4760 Section 8) parses into AFI=IPv4/SAFI=Unicast; this is
// the Capability Advertisement wire form a speaker uses to announce an AFI/SAFI it supports
// (parseMultiprotocol internal/core/bgp/capability/capability.go:310-318, the same layout
// Multiprotocol.WriteTo emits at internal/core/bgp/capability/capability.go:299-305).
func TestParseCapabilities(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
//
// RFC requirement: RFC5492-3-2 positive -- an unrecognized capability code (254) is
// preserved as *Unknown with its value bytes and Parse returns no error, so the parser
// ignores the unknown code rather than rejecting it, per parseCapability's default arm
// (internal/core/bgp/capability/capability.go:239-242).
func TestParseUnknownCapability(t *testing.T) {
	t.Parallel()
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

// TestParseRejectsMalformedKnownCapabilityLength verifies zero-length known
// capabilities reject non-zero payloads.
//
// VALIDATES: AC-1 malformed Route Refresh, Extended Message, and Enhanced Route Refresh capabilities fail.
//
// PREVENTS: Negotiating malformed known capabilities that should be rejected before OPEN acceptance.
//
// RFC requirement: RFC5492-3-2 negative -- a KNOWN capability (Route Refresh, Extended
// Message, Enhanced Route Refresh) carrying a non-zero length is rejected with
// ErrInvalidLength. This proves the "ignore unrecognized code" rule is scoped to unknown
// codes only: a malformed known capability is not silently ignored (capability.go:245-252
// parseZeroLengthCapability).
func TestParseRejectsMalformedKnownCapabilityLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code Code
	}{
		{"route_refresh", CodeRouteRefresh},
		{"extended_message", CodeExtendedMessage},
		{"enhanced_route_refresh", CodeEnhancedRouteRefresh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse([]byte{byte(tt.code), 0x01, 0x00})
			require.ErrorIs(t, err, ErrInvalidLength)
		})
	}
}

// TestOptionalParamRejectsTruncatedCapabilityTLV verifies Type 2 parameter TLV
// parsing propagates malformed inner capability errors.
//
// VALIDATES: AC-2 truncated capability TLVs return an error.
//
// PREVENTS: Silently dropping malformed Type 2 optional parameters during OPEN parsing.
//
// RFC requirement: RFC5492-4-2 negative -- a Type-2 Capabilities Optional Parameter whose
// inner capability TLV is truncated is rejected with ErrShortRead. Acceptance of Type-2
// parameters is bounded: ParseFromOptionalParams validates each parameter's TLVs rather
// than blindly accepting them (internal/core/bgp/capability/capability.go:847).
func TestOptionalParamRejectsTruncatedCapabilityTLV(t *testing.T) {
	t.Parallel()

	_, err := ParseFromOptionalParams([]byte{
		0x02, 0x04, // Optional Parameter type=Capabilities, len=4
		0x01, 0x04, // Capability code=Multiprotocol, len=4, value truncated
	})
	require.ErrorIs(t, err, ErrShortRead)
}

// TestOptionalParamPreservesUnknownCapability verifies unknown capabilities
// remain syntactically parsed and available to the caller.
//
// VALIDATES: AC-3 syntactically valid unknown capabilities are preserved.
//
// PREVENTS: Treating ignorable unknown capabilities as malformed OPEN input.
//
// RFC requirement: RFC5492-3-3 positive -- an unknown capability inside an OPEN Type-2
// parameter is parsed without error, so the OPEN parse path yields no error for the
// session layer to reject on; the session is not terminated (capability.go:847 ->
// capability.go:239-242, terminate path in internal/component/bgp/reactor/session_handlers.go:190-197
// fires only on ErrInvalidLength/ErrShortRead).
// RFC requirement: RFC5492-3-4 positive -- the same unknown capability is preserved with
// no error, so no Unsupported Capability NOTIFICATION is generated for it.
// RFC requirement: RFC5492-5-2 positive -- a not-understood capability is preserved and
// ignored, never rejected, on the OPEN optional-parameter parse path.
func TestOptionalParamPreservesUnknownCapability(t *testing.T) {
	t.Parallel()

	caps, err := ParseFromOptionalParams([]byte{
		0x02, 0x04, // Optional Parameter type=Capabilities, len=4
		0xFE, 0x02, 0xAB, 0xCD, // Unknown capability code 254, len=2
	})
	require.NoError(t, err)
	require.Len(t, caps, 1)

	unknown, ok := caps[0].(*Unknown)
	require.True(t, ok)
	assert.Equal(t, Code(254), unknown.Code())
	assert.Equal(t, []byte{0xAB, 0xCD}, unknown.Data)
}

// TestParseAcceptsMultipleIdenticalCapabilityInstances verifies that two identical
// capability TLVs in one Capabilities parameter are both parsed and accepted.
//
// VALIDATES: RFC 5492 Section 4 duplicate-instance acceptance.
//
// PREVENTS: Rejecting or de-duplicating an OPEN that repeats a capability.
//
// RFC requirement: RFC5492-4-1 positive -- two identical Multiprotocol IPv4/Unicast
// capability TLVs are both returned by Parse with no error. Parse appends every TLV it
// reads (internal/core/bgp/capability/capability.go:177) with no dedup or reject path, so
// multiple identical instances of the same Capability Code+Length+Value are all accepted.
func TestParseAcceptsMultipleIdenticalCapabilityInstances(t *testing.T) {
	t.Parallel()
	// Two identical Multiprotocol IPv4/Unicast capability TLVs.
	data := []byte{
		0x01, 0x04, 0x00, 0x01, 0x00, 0x01, // Multiprotocol IPv4/Unicast
		0x01, 0x04, 0x00, 0x01, 0x00, 0x01, // identical duplicate
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 2, "both identical capability instances must be accepted")

	for i, c := range caps {
		mp, ok := c.(*Multiprotocol)
		require.Truef(t, ok, "instance %d should be Multiprotocol", i)
		assert.Equal(t, AFIIPv4, mp.AFI)
		assert.Equal(t, SAFIUnicast, mp.SAFI)
	}
}

// TestParseFromOptionalParamsMultipleCapabilitiesParameters verifies that an OPEN
// carrying two separate Type-2 Capabilities Optional Parameters has both processed.
//
// VALIDATES: RFC 5492 Section 4 multiple-Capabilities-Parameter acceptance.
//
// PREVENTS: Dropping capabilities carried in a second Capabilities Optional Parameter.
//
// RFC requirement: RFC5492-4-2 positive -- two distinct Type-2 optional parameters, one
// carrying Multiprotocol and one carrying ASN4, both contribute their capabilities.
// ParseFromOptionalParams loops over every optional parameter and parses each Type-2
// (internal/core/bgp/capability/capability.go:847), so capabilities split across multiple
// Capabilities Optional Parameters are all accepted.
func TestParseFromOptionalParamsMultipleCapabilitiesParameters(t *testing.T) {
	t.Parallel()
	optParams := []byte{
		// Parameter 1: type=2 (Capabilities), len=6 -- Multiprotocol IPv4/Unicast
		0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01,
		// Parameter 2: type=2 (Capabilities), len=6 -- ASN4 (AS 65002)
		0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA,
	}

	caps, err := ParseFromOptionalParams(optParams)
	require.NoError(t, err)
	require.Len(t, caps, 2, "capabilities from both Capabilities Optional Parameters must be parsed")

	mp, ok := caps[0].(*Multiprotocol)
	require.True(t, ok, "first parameter should yield Multiprotocol")
	assert.Equal(t, AFIIPv4, mp.AFI)
	assert.Equal(t, SAFIUnicast, mp.SAFI)

	asn4, ok := caps[1].(*ASN4)
	require.True(t, ok, "second parameter should yield ASN4")
	assert.Equal(t, uint32(65002), asn4.ASN)
}

// TestAddPathCapability verifies ADD-PATH parsing and packing (RFC 7911).
//
// VALIDATES: ADD-PATH capability handling for path diversity.
//
// PREVENTS: Route selection issues when multiple paths are available.
// RFC requirement: RFC7911-4-1 negative -- a single-family ADD-PATH parses to exactly one capability instance (len(caps)==1) carrying that one AFI/SAFI, so no extra instance is fabricated when only one family is present.
func TestAddPathCapability(t *testing.T) {
	t.Parallel()
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
// RFC requirement: RFC7911-4-1 positive -- two AFI/SAFIs (IPv4 and IPv6 unicast) are advertised in a single ADD-PATH capability instance: Parse yields exactly one capability (len(caps)==1) carrying both families.
func TestAddPathMultipleFamilies(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// TestGracefulRestartEncodeReservedBits verifies the sender zeroes the reserved bits of the
// Restart Flags and Address Family Flags fields (RFC 4724 Section 3).
func TestGracefulRestartEncodeReservedBits(t *testing.T) {
	t.Parallel()

	gr := &GracefulRestart{
		RestartState: true,
		RestartTime:  120,
		Families: []GracefulRestartFamily{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, ForwardingState: true},
		},
	}
	buf := make([]byte, gr.Len())
	gr.WriteTo(buf, 0)
	// Layout: [code, len, flags_hi, flags_lo, afi_hi, afi_lo, safi, af_flags].
	require.Len(t, buf, 8)

	// RFC requirement: RFC4724-3-2 positive -- WriteTo (internal/core/bgp/capability/capability.go:557-561)
	// sets only the R bit (0x8000) and masks Restart Time to 12 bits, so the three reserved bits of the
	// Restart Flags nibble (mask 0x70 of byte 0) are zero on send.
	assert.Equal(t, byte(0x00), buf[2]&0x70, "restart-flags reserved bits must be zero on send")
	assert.Equal(t, byte(0x80), buf[2]&0x80, "R bit set when RestartState is true")

	// RFC requirement: RFC4724-3-3 positive -- WriteTo (capability.go:567-571) sets only the F bit (0x80)
	// per AFI/SAFI, leaving the seven reserved Address Family Flag bits (mask 0x7F) zero on send.
	assert.Equal(t, byte(0x00), buf[7]&0x7F, "AF-flags reserved bits must be zero on send")
	assert.Equal(t, byte(0x80), buf[7]&0x80, "F bit set when ForwardingState is true")

	// RFC requirement: RFC4724-3-2 negative -- an over-range Restart Time does not leak into the reserved
	// Restart Flags bits: WriteTo masks it (val & 0x0FFF), so the reserved bits stay zero while the 12-bit
	// value is preserved.
	over := &GracefulRestart{RestartTime: 0xFFFF}
	obuf := make([]byte, over.Len())
	over.WriteTo(obuf, 0)
	assert.Equal(t, byte(0x00), obuf[2]&0x70, "over-range restart-time must not set reserved restart-flag bits")
	assert.Equal(t, byte(0x0F), obuf[2]&0x0F, "12-bit restart-time high nibble preserved")

	// RFC requirement: RFC4724-3-3 negative -- an AFI/SAFI with the F bit clear encodes an all-zero AF
	// flags byte, so a cleared forwarding state does not spuriously set any reserved AF-flag bit.
	noF := &GracefulRestart{
		RestartTime: 30,
		Families:    []GracefulRestartFamily{{AFI: AFIIPv6, SAFI: SAFIUnicast, ForwardingState: false}},
	}
	nbuf := make([]byte, noF.Len())
	noF.WriteTo(nbuf, 0)
	assert.Equal(t, byte(0x00), nbuf[7], "F-bit clear encodes an all-zero AF flags byte")
}

// TestCapabilityRoundTrip verifies pack/parse round-trip.
//
// VALIDATES: Serialization correctness.
//
// PREVENTS: Data corruption during pack/parse cycle.
func TestCapabilityRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cap  Capability
	}{
		{"Multiprotocol", &Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast}},
		{"ASN4", &ASN4{ASN: 4200000001}},
		{"RouteRefresh", &RouteRefresh{}},
		// RFC requirement: RFC8654-3-3 positive -- the Extended Message capability carries a
		// zero-length value: Len() is 2 (header only) and WriteTo emits Cap Len 0
		// (capability.go:383,385-387); this case round-trips that zero-length value through Parse.
		{"ExtendedMessage", &ExtendedMessage{}},
		{"AddPath", &AddPath{Families: []AddPathFamily{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Mode: AddPathBoth},
		}}},
		{"ExtendedNextHop", &ExtendedNextHop{Families: []ExtendedNextHopFamily{
			{NLRIAFI: AFIIPv4, NLRISAFI: SAFIUnicast, NextHopAFI: AFIIPv6},
		}}},
		{"FQDN", &FQDN{Hostname: "router1", DomainName: "example.com"}},
		{"PathsLimit", &PathsLimit{Entries: []PathsLimitEntry{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10},
		}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
//
// RFC requirement: RFC8950-4-3 positive -- a capability TLV with code 0x05 parses as an
// ExtendedNextHop capability; ze binds the Extended Next Hop Encoding capability to code 5
// (CodeExtendedNextHop = 5, internal/core/bgp/capability/capability.go:70).
//
// RFC requirement: RFC5549-4-2 positive -- the capability is carried via the RFC 5492 capability TLV
// framework: a code-5 TLV parses back into an ExtendedNextHop with its AFI/SAFI/NextHop-AFI tuple
// (internal/core/bgp/capability/capability.go:667 parseExtendedNextHop).
// RFC requirement: RFC5549-4-3 positive -- the parsed capability's on-wire Capability Code is 5 (0x05),
// matching CodeExtendedNextHop (internal/core/bgp/capability/capability.go:70).
func TestExtendedNextHopCapability(t *testing.T) {
	t.Parallel()
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
//
// RFC requirement: RFC8950-4-3 positive -- WriteTo emits the capability with code 5
// (writeCapabilityTo(..., CodeExtendedNextHop, ...) at internal/core/bgp/capability/capability.go:644)
// and Parse reads it back as an ExtendedNextHop, so the on-wire Capability Code is 5 in both directions.
//
// RFC requirement: RFC5549-4-2 positive -- WriteTo emits and Parse reads back the capability using the
// RFC 5492 capability TLV procedures, round-tripping the tuple (internal/core/bgp/capability/capability.go:644
// WriteTo, :667 parseExtendedNextHop).
// RFC requirement: RFC5549-4-3 positive -- WriteTo emits the capability with Code 5
// (writeCapabilityTo(..., CodeExtendedNextHop, ...) at internal/core/bgp/capability/capability.go:646) and
// Parse reads it back, so the Capability Code is 5 in both directions.
func TestExtendedNextHopRoundTrip(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	caps := []Capability{
		&Unknown{code: 99, Data: []byte{0x01, 0x02, 0x03}},
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIEVPN},
		&ASN4{ASN: 65533},
		&ASN4{ASN: 4200000000},
		// RFC requirement: RFC2918-2-1 positive -- RouteRefresh.WriteTo emits the
		// capability with Code 2 and Length 0 (capability.go RouteRefresh.Code/Len/WriteTo),
		// and the bytes parse back to the same capability code.
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
		&PathsLimit{Entries: []PathsLimitEntry{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10},
			{AFI: AFIIPv6, SAFI: SAFIUnicast, Limit: 65535},
		}},
		NewPlugin(99, []byte{0xDE, 0xAD}),
	}

	for _, c := range caps {
		name := fmt.Sprintf("%T/code=%d", c, c.Code())
		t.Run(name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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

// TestPathsLimitCode verifies capability code is 76.
//
// VALIDATES: Correct IANA capability code.
//
// PREVENTS: Wrong capability code in OPEN message.
func TestPathsLimitCode(t *testing.T) {
	t.Parallel()
	pl := &PathsLimit{}
	assert.Equal(t, CodePathsLimit, pl.Code())
	assert.Equal(t, Code(76), pl.Code())
}

// TestParsePathsLimit verifies parsing 5-byte entries from wire bytes.
//
// VALIDATES: draft-abraitis-idr-addpath-paths-limit wire parsing.
//
// PREVENTS: Interop failures with peers advertising PATHS-LIMIT.
func TestParsePathsLimit(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x4C,       // Code = PATHS-LIMIT (76)
		0x0A,       // Length = 10 (2 entries * 5 bytes)
		0x00, 0x01, // AFI = IPv4
		0x01,       // SAFI = Unicast
		0x00, 0x0A, // Limit = 10
		0x00, 0x02, // AFI = IPv6
		0x01,       // SAFI = Unicast
		0x00, 0x14, // Limit = 20
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	pl, ok := caps[0].(*PathsLimit)
	require.True(t, ok)
	require.Len(t, pl.Entries, 2)

	assert.Equal(t, AFIIPv4, pl.Entries[0].AFI)
	assert.Equal(t, SAFIUnicast, pl.Entries[0].SAFI)
	assert.Equal(t, uint16(10), pl.Entries[0].Limit)

	assert.Equal(t, AFIIPv6, pl.Entries[1].AFI)
	assert.Equal(t, SAFIUnicast, pl.Entries[1].SAFI)
	assert.Equal(t, uint16(20), pl.Entries[1].Limit)
}

// TestParsePathsLimitEmpty verifies empty data produces empty PathsLimit.
//
// VALIDATES: Edge case with no entries.
//
// PREVENTS: Panic on empty PATHS-LIMIT capability.
func TestParsePathsLimitEmpty(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x4C, // Code = PATHS-LIMIT (76)
		0x00, // Length = 0
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	pl, ok := caps[0].(*PathsLimit)
	require.True(t, ok)
	assert.Empty(t, pl.Entries)
}

// TestParsePathsLimitShortRead verifies truncated data returns ErrShortRead.
//
// VALIDATES: Malformed data detection.
//
// PREVENTS: Buffer overread from corrupted packets.
func TestParsePathsLimitShortRead(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x4C,       // Code = PATHS-LIMIT (76)
		0x03,       // Length = 3 (not multiple of 5)
		0x00, 0x01, // AFI
		0x01, // SAFI (truncated: missing 2-byte limit)
	}

	_, err := Parse(data)
	require.ErrorIs(t, err, ErrShortRead)
}

// TestParsePathsLimitSkipZero verifies entries with limit 0 are skipped.
//
// VALIDATES: draft-abraitis-idr-addpath-paths-limit: limit 0 means skip.
//
// PREVENTS: Accepting 0 as a valid limit (would mean "no paths").
func TestParsePathsLimitSkipZero(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x4C,       // Code = PATHS-LIMIT (76)
		0x0A,       // Length = 10
		0x00, 0x01, // AFI = IPv4
		0x01,       // SAFI = Unicast
		0x00, 0x00, // Limit = 0 (skip)
		0x00, 0x02, // AFI = IPv6
		0x01,       // SAFI = Unicast
		0x00, 0x05, // Limit = 5
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	pl, ok := caps[0].(*PathsLimit)
	require.True(t, ok)
	require.Len(t, pl.Entries, 1)
	assert.Equal(t, AFIIPv6, pl.Entries[0].AFI)
	assert.Equal(t, uint16(5), pl.Entries[0].Limit)
}

// TestParsePathsLimitDuplicateFirstWins verifies duplicate AFI/SAFI: first entry kept.
//
// VALIDATES: draft-abraitis-idr-addpath-paths-limit duplicate handling.
//
// PREVENTS: Later duplicates overwriting the first valid entry.
//
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4 positive -- for a repeated AFI/SAFI pair only the first tuple is considered (limit 10 kept).
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-4 negative -- all other duplicate tuples for that pair are ignored (the limit-20 duplicate is dropped, one entry remains).
func TestParsePathsLimitDuplicateFirstWins(t *testing.T) {
	t.Parallel()
	data := []byte{
		0x4C,       // Code
		0x0A,       // Length = 10
		0x00, 0x01, // AFI = IPv4
		0x01,       // SAFI = Unicast
		0x00, 0x0A, // Limit = 10 (first, kept)
		0x00, 0x01, // AFI = IPv4
		0x01,       // SAFI = Unicast
		0x00, 0x14, // Limit = 20 (duplicate, ignored)
	}

	caps, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	pl, ok := caps[0].(*PathsLimit)
	require.True(t, ok)
	require.Len(t, pl.Entries, 1)
	assert.Equal(t, uint16(10), pl.Entries[0].Limit)
}

// TestPathsLimitWriteTo verifies WriteTo produces correct wire bytes.
//
// VALIDATES: draft-abraitis-idr-addpath-paths-limit wire encoding.
//
// PREVENTS: Malformed OPEN messages sent to peers.
func TestPathsLimitWriteTo(t *testing.T) {
	t.Parallel()
	pl := &PathsLimit{
		Entries: []PathsLimitEntry{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10},
			{AFI: AFIIPv6, SAFI: SAFIUnicast, Limit: 65535},
		},
	}

	buf := make([]byte, pl.Len())
	n := pl.WriteTo(buf, 0)

	assert.Equal(t, pl.Len(), n)
	assert.Equal(t, 12, n) // 2 header + 2*5 data

	assert.Equal(t, byte(76), buf[0]) // Code
	assert.Equal(t, byte(10), buf[1]) // Length = 10

	// Entry 1: IPv4/unicast limit 10
	assert.Equal(t, byte(0x00), buf[2])
	assert.Equal(t, byte(0x01), buf[3]) // AFI = 1
	assert.Equal(t, byte(0x01), buf[4]) // SAFI = 1
	assert.Equal(t, byte(0x00), buf[5])
	assert.Equal(t, byte(0x0A), buf[6]) // Limit = 10

	// Entry 2: IPv6/unicast limit 65535
	assert.Equal(t, byte(0x00), buf[7])
	assert.Equal(t, byte(0x02), buf[8]) // AFI = 2
	assert.Equal(t, byte(0x01), buf[9]) // SAFI = 1
	assert.Equal(t, byte(0xFF), buf[10])
	assert.Equal(t, byte(0xFF), buf[11]) // Limit = 65535
}

// TestPathsLimitRoundTrip verifies encode then decode yields same struct.
//
// VALIDATES: Serialization correctness.
//
// PREVENTS: Data corruption during pack/parse cycle.
func TestPathsLimitRoundTrip(t *testing.T) {
	t.Parallel()
	original := &PathsLimit{
		Entries: []PathsLimitEntry{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10},
			{AFI: AFIIPv6, SAFI: SAFIUnicast, Limit: 100},
			{AFI: AFIIPv4, SAFI: SAFIVPN, Limit: 1},
		},
	}

	packed := make([]byte, original.Len())
	original.WriteTo(packed, 0)
	parsed, err := Parse(packed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	pl, ok := parsed[0].(*PathsLimit)
	require.True(t, ok)
	require.Len(t, pl.Entries, 3)

	for i, entry := range pl.Entries {
		assert.Equal(t, original.Entries[i].AFI, entry.AFI)
		assert.Equal(t, original.Entries[i].SAFI, entry.SAFI)
		assert.Equal(t, original.Entries[i].Limit, entry.Limit)
	}
}

// TestPathsLimitLen verifies Len returns 2 + 5*N.
//
// VALIDATES: Correct size calculation for buffer allocation.
//
// PREVENTS: Buffer overflows from wrong size.
func TestPathsLimitLen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		entries int
		want    int
	}{
		{"empty", 0, 0},
		{"one", 1, 7},
		{"two", 2, 12},
		{"max_50", 50, 252},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entries := make([]PathsLimitEntry, tt.entries)
			for i := range entries {
				entries[i] = PathsLimitEntry{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10}
			}
			pl := &PathsLimit{Entries: entries}
			assert.Equal(t, tt.want, pl.Len())
		})
	}
}

// TestPathsLimitConfigValues verifies ConfigValues returns scoped keys.
//
// VALIDATES: Plugin config delivery for PATHS-LIMIT capability.
//
// PREVENTS: Missing capability config in plugin Stage 2 delivery.
func TestPathsLimitConfigValues(t *testing.T) {
	t.Parallel()
	pl := &PathsLimit{
		Entries: []PathsLimitEntry{
			{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: 10},
		},
	}
	vals := pl.ConfigValues()
	assert.Equal(t, "true", vals["draft-abraitis-paths-limit:enabled"])

	empty := &PathsLimit{}
	assert.Nil(t, empty.ConfigValues())
}

// TestPathsLimitBoundary verifies boundary values for limit field.
//
// VALIDATES: uint16 boundary handling (1 and 65535).
//
// PREVENTS: Off-by-one in limit encoding/decoding.
func TestPathsLimitBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		limit uint16
		kept  bool
	}{
		{"zero_skipped", 0, false},
		{"min_valid", 1, true},
		{"max_valid", 65535, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			original := &PathsLimit{
				Entries: []PathsLimitEntry{
					{AFI: AFIIPv4, SAFI: SAFIUnicast, Limit: tt.limit},
				},
			}

			packed := make([]byte, original.Len())
			original.WriteTo(packed, 0)
			parsed, err := Parse(packed)
			require.NoError(t, err)
			require.Len(t, parsed, 1)

			pl, ok := parsed[0].(*PathsLimit)
			require.True(t, ok)

			if tt.kept {
				require.Len(t, pl.Entries, 1)
				assert.Equal(t, tt.limit, pl.Entries[0].Limit)
			} else {
				assert.Empty(t, pl.Entries)
			}
		})
	}
}
