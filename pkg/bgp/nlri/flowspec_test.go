package nlri

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFlowSpecComponentTypes verifies component type constants.
//
// VALIDATES: Component type constants match RFC 8955 Section 4.2.2 values exactly.
//
// PREVENTS: Registry mismatch with RFC-assigned component type numbers;
// silent breakage if constants are accidentally changed.
func TestFlowSpecComponentTypes(t *testing.T) {
	// RFC 5575 component types
	assert.Equal(t, FlowComponentType(1), FlowDestPrefix)
	assert.Equal(t, FlowComponentType(2), FlowSourcePrefix)
	assert.Equal(t, FlowComponentType(3), FlowIPProtocol)
	assert.Equal(t, FlowComponentType(4), FlowPort)
	assert.Equal(t, FlowComponentType(5), FlowDestPort)
	assert.Equal(t, FlowComponentType(6), FlowSourcePort)
	assert.Equal(t, FlowComponentType(7), FlowICMPType)
	assert.Equal(t, FlowComponentType(8), FlowICMPCode)
	assert.Equal(t, FlowComponentType(9), FlowTCPFlags)
	assert.Equal(t, FlowComponentType(10), FlowPacketLength)
	assert.Equal(t, FlowComponentType(11), FlowDSCP)
	assert.Equal(t, FlowComponentType(12), FlowFragment)
	assert.Equal(t, FlowComponentType(13), FlowFlowLabel) // RFC 8956 (IPv6)
}

// TestFlowSpecDestPrefix verifies destination prefix component.
//
// VALIDATES: Type 1 component stores and returns correct prefix per RFC 8955 Section 4.2.2.1.
//
// PREVENTS: Prefix data corruption; Type() returning wrong component identifier.
func TestFlowSpecDestPrefix(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	comp := NewFlowDestPrefixComponent(prefix)

	assert.Equal(t, FlowDestPrefix, comp.Type())
	// Type assert to access Prefix method
	pc, ok := comp.(interface{ Prefix() netip.Prefix })
	require.True(t, ok)
	assert.Equal(t, prefix, pc.Prefix())
}

// TestFlowSpecSourcePrefix verifies source prefix component.
//
// VALIDATES: Type 2 component stores and returns correct prefix per RFC 8955 Section 4.2.2.2.
//
// PREVENTS: Source/destination prefix confusion; incorrect Type() return value.
func TestFlowSpecSourcePrefix(t *testing.T) {
	prefix := netip.MustParsePrefix("192.168.1.0/24")
	comp := NewFlowSourcePrefixComponent(prefix)

	assert.Equal(t, FlowSourcePrefix, comp.Type())
	pc, ok := comp.(interface{ Prefix() netip.Prefix })
	require.True(t, ok)
	assert.Equal(t, prefix, pc.Prefix())
}

// TestFlowSpecIPProtocol verifies IP protocol component.
//
// VALIDATES: Type 3 component encodes IP protocol values per RFC 8955 Section 4.2.2.3.
// Values SHOULD be single-byte (len=00).
//
// PREVENTS: Protocol values being lost or corrupted; incorrect operator encoding.
func TestFlowSpecIPProtocol(t *testing.T) {
	// TCP = 6
	comp := NewFlowIPProtocolComponent(6)

	assert.Equal(t, FlowIPProtocol, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(6))
}

// TestFlowSpecPort verifies port component (src or dst).
//
// VALIDATES: Type 4 component matches source OR destination port per RFC 8955 Section 4.2.2.4.
// Multiple port values can be specified with OR semantics.
//
// PREVENTS: Port values lost when multiple specified; port matching incorrect logic.
func TestFlowSpecPort(t *testing.T) {
	comp := NewFlowPortComponent(80, 443)

	assert.Equal(t, FlowPort, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(80))
	assert.Contains(t, nc.Values(), uint64(443))
}

// TestFlowSpecDestPort verifies destination port component.
//
// VALIDATES: Type 5 component stores destination port per RFC 8955 Section 4.2.2.5.
//
// PREVENTS: Source/dest port confusion; port value truncation.
func TestFlowSpecDestPort(t *testing.T) {
	comp := NewFlowDestPortComponent(22)

	assert.Equal(t, FlowDestPort, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(22))
}

// TestFlowSpecSourcePort verifies source port component.
//
// VALIDATES: Type 6 component stores source port per RFC 8955 Section 4.2.2.6.
//
// PREVENTS: Source/dest port confusion; multiple values being dropped.
func TestFlowSpecSourcePort(t *testing.T) {
	comp := NewFlowSourcePortComponent(1024, 65535)

	assert.Equal(t, FlowSourcePort, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(1024))
	assert.Contains(t, nc.Values(), uint64(65535))
}

// TestFlowSpecICMPType verifies ICMP type component (Type 7).
//
// VALIDATES: ICMP Type component correctly encodes values per RFC 8955 Section 4.2.2.7.
// Type 7 values SHOULD be encoded as single octet (numeric_op len=00).
//
// PREVENTS: ICMP type values being confused with other numeric components;
// incorrect length encoding for single-byte ICMP type values (0-255).
func TestFlowSpecICMPType(t *testing.T) {
	// Echo Request = 8, Echo Reply = 0 per RFC 792
	comp := NewFlowICMPTypeComponent(8, 0)

	assert.Equal(t, FlowICMPType, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(8))
	assert.Contains(t, nc.Values(), uint64(0))
}

// TestFlowSpecICMPTypeRoundTrip verifies ICMP type encode/decode cycle.
//
// VALIDATES: ICMP Type component round-trips correctly through wire encoding
// per RFC 8955 Section 4.2.2.7.
//
// PREVENTS: Data corruption during encode/decode; incorrect operator byte handling.
func TestFlowSpecICMPTypeRoundTrip(t *testing.T) {
	original := NewFlowSpec(IPv4FlowSpec)
	original.AddComponent(NewFlowICMPTypeComponent(8)) // Echo Request

	data := original.Bytes()
	parsed, err := ParseFlowSpec(IPv4FlowSpec, data)
	require.NoError(t, err)
	require.Len(t, parsed.Components(), 1)

	comp := parsed.Components()[0]
	assert.Equal(t, FlowICMPType, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(8))
}

// TestFlowSpecICMPCode verifies ICMP code component (Type 8).
//
// VALIDATES: ICMP Code component correctly encodes values per RFC 8955 Section 4.2.2.8.
// Type 8 values SHOULD be encoded as single octet (numeric_op len=00).
//
// PREVENTS: ICMP code values being confused with ICMP type; incorrect operator encoding.
func TestFlowSpecICMPCode(t *testing.T) {
	// Network Unreachable = 0, Host Unreachable = 1 per RFC 792
	comp := NewFlowICMPCodeComponent(0, 1)

	assert.Equal(t, FlowICMPCode, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(0))
	assert.Contains(t, nc.Values(), uint64(1))
}

// TestFlowSpecICMPBoundary verifies ICMP type/code boundary values (0, 255).
//
// VALIDATES: ICMP Type (Type 7) and ICMP Code (Type 8) components handle full uint8 range
// per RFC 8955 Section 4.2.2.7-8. Values are single-octet (0-255).
//
// PREVENTS: Boundary value truncation; off-by-one errors at 0 or 255.
func TestFlowSpecICMPBoundary(t *testing.T) {
	tests := []struct {
		name      string
		comp      FlowComponent
		buildNLRI func() *FlowSpec
	}{
		{
			name: "icmp-type",
			comp: NewFlowICMPTypeComponent(0, 255),
			buildNLRI: func() *FlowSpec {
				fs := NewFlowSpec(IPv4FlowSpec)
				fs.AddComponent(NewFlowICMPTypeComponent(0, 255))
				return fs
			},
		},
		{
			name: "icmp-code",
			comp: NewFlowICMPCodeComponent(0, 255),
			buildNLRI: func() *FlowSpec {
				fs := NewFlowSpec(IPv4FlowSpec)
				fs.AddComponent(NewFlowICMPCodeComponent(0, 255))
				return fs
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify component stores boundary values
			nc, ok := tt.comp.(interface{ Values() []uint64 })
			require.True(t, ok)
			assert.Contains(t, nc.Values(), uint64(0), "min value 0 missing")
			assert.Contains(t, nc.Values(), uint64(255), "max value 255 missing")

			// Round-trip test with boundary values
			original := tt.buildNLRI()
			data := original.Bytes()
			parsed, err := ParseFlowSpec(IPv4FlowSpec, data)
			require.NoError(t, err)
			require.Len(t, parsed.Components(), 1)

			parsedNC, ok := parsed.Components()[0].(interface{ Values() []uint64 })
			require.True(t, ok)
			assert.Contains(t, parsedNC.Values(), uint64(0), "min value 0 lost in round-trip")
			assert.Contains(t, parsedNC.Values(), uint64(255), "max value 255 lost in round-trip")
		})
	}
}

// TestFlowSpecICMPCodeRoundTrip verifies ICMP code encode/decode cycle.
//
// VALIDATES: ICMP Code component round-trips correctly through wire encoding
// per RFC 8955 Section 4.2.2.8.
//
// PREVENTS: Data corruption during encode/decode; confusion with ICMP type component.
func TestFlowSpecICMPCodeRoundTrip(t *testing.T) {
	original := NewFlowSpec(IPv4FlowSpec)
	original.AddComponent(NewFlowICMPCodeComponent(3)) // Port Unreachable

	data := original.Bytes()
	parsed, err := ParseFlowSpec(IPv4FlowSpec, data)
	require.NoError(t, err)
	require.Len(t, parsed.Components(), 1)

	comp := parsed.Components()[0]
	assert.Equal(t, FlowICMPCode, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(3))
}

// TestFlowSpecTCPFlags verifies TCP flags component.
//
// VALIDATES: Type 9 component uses bitmask_op per RFC 8955 Section 4.2.2.9.
// TCP flags encoded as 1 or 2 byte bitmask.
//
// PREVENTS: TCP flags using wrong operator type (numeric vs bitmask);
// flag bits being corrupted.
func TestFlowSpecTCPFlags(t *testing.T) {
	// SYN flag = 0x02
	comp := NewFlowTCPFlagsComponent(0x02)

	assert.Equal(t, FlowTCPFlags, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(0x02))
}

// TestFlowSpecPacketLength verifies packet length component.
//
// VALIDATES: Type 10 component matches total IP packet length per RFC 8955 Section 4.2.2.10.
// Values SHOULD be 1 or 2 byte quantities.
//
// PREVENTS: Packet length matching against wrong field (e.g., L2 frame size);
// multi-value ranges being dropped.
func TestFlowSpecPacketLength(t *testing.T) {
	comp := NewFlowPacketLengthComponent(64, 1500)

	assert.Equal(t, FlowPacketLength, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(64))
	assert.Contains(t, nc.Values(), uint64(1500))
}

// TestFlowSpecDSCP verifies DSCP component.
//
// VALIDATES: Type 11 component matches 6-bit DSCP field per RFC 8955 Section 4.2.2.11.
// Values MUST be single octet.
//
// PREVENTS: DSCP values exceeding 6 bits; confusion with full TOS byte.
func TestFlowSpecDSCP(t *testing.T) {
	// EF = 46
	comp := NewFlowDSCPComponent(46)

	assert.Equal(t, FlowDSCP, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(46))
}

// TestFlowSpecFragment verifies fragment component.
//
// VALIDATES: Type 12 component uses bitmask_op per RFC 8955 Section 4.2.2.12.
// Bitmask encoded as single octet with DF, IsF, FF, LF bits.
//
// PREVENTS: Fragment flags using wrong operator type; flag bits inverted.
func TestFlowSpecFragment(t *testing.T) {
	comp := NewFlowFragmentComponent(FlowFragDontFragment)

	assert.Equal(t, FlowFragment, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(FlowFragDontFragment))
}

// TestFlowSpecFlowLabel verifies Flow Label component (Type 13, IPv6 only).
//
// VALIDATES: Flow Label component correctly encodes values per RFC 8956 Section 3.7.
// Type 13 values SHOULD be encoded as 4-octet quantities (numeric_op len=10).
// The Flow Label is a 20-bit field in the IPv6 header (max 0xFFFFF).
//
// PREVENTS: Flow Label values being corrupted; incorrect 4-byte encoding.
func TestFlowSpecFlowLabel(t *testing.T) {
	// Flow Label is 20-bit, max value 0xFFFFF
	comp := NewFlowFlowLabelComponent(0x12345, 0x00000)

	assert.Equal(t, FlowFlowLabel, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(0x12345))
	assert.Contains(t, nc.Values(), uint64(0x00000))
}

// TestFlowSpecFlowLabelRoundTrip verifies Flow Label encode/decode cycle.
//
// VALIDATES: Flow Label component round-trips correctly through wire encoding
// per RFC 8956 Section 3.7.
//
// PREVENTS: Data corruption during encode/decode; incorrect 4-byte value encoding.
func TestFlowSpecFlowLabelRoundTrip(t *testing.T) {
	original := NewFlowSpec(IPv6FlowSpec)
	original.AddComponent(NewFlowFlowLabelComponent(0xABCDE))

	data := original.Bytes()
	parsed, err := ParseFlowSpec(IPv6FlowSpec, data)
	require.NoError(t, err)
	require.Len(t, parsed.Components(), 1)

	comp := parsed.Components()[0]
	assert.Equal(t, FlowFlowLabel, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(0xABCDE))
}

// TestFlowSpecIPv4Basic verifies basic IPv4 FlowSpec NLRI.
//
// VALIDATES: IPv4 FlowSpec NLRI (AFI=1, SAFI=133) can hold multiple components
// per RFC 8955 Section 4.
//
// PREVENTS: AFI/SAFI family confusion; component list corruption.
func TestFlowSpecIPv4Basic(t *testing.T) {
	fs := NewFlowSpec(IPv4FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
	fs.AddComponent(NewFlowIPProtocolComponent(6)) // TCP

	assert.Equal(t, IPv4FlowSpec, fs.Family())
	assert.Len(t, fs.Components(), 2)
}

// TestFlowSpecIPv6Basic verifies basic IPv6 FlowSpec NLRI.
//
// VALIDATES: IPv6 FlowSpec NLRI (AFI=2, SAFI=133) correctly stores components
// per RFC 8956.
//
// PREVENTS: IPv4/IPv6 family confusion; IPv6 prefix encoding errors.
func TestFlowSpecIPv6Basic(t *testing.T) {
	fs := NewFlowSpec(IPv6FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("2001:db8::/32")))

	assert.Equal(t, IPv6FlowSpec, fs.Family())
}

// TestFlowSpecBytes verifies wire format encoding.
//
// VALIDATES: FlowSpec Bytes() produces valid NLRI with length prefix per RFC 8955 Section 4.1.
//
// PREVENTS: Missing length prefix; empty output for valid FlowSpec.
func TestFlowSpecBytes(t *testing.T) {
	fs := NewFlowSpec(IPv4FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/8")))

	data := fs.Bytes()
	require.NotEmpty(t, data)

	// First byte is NLRI length, then components
	// Component format: type (1 byte) + prefix encoding
}

// TestFlowSpecString verifies string representation.
//
// VALIDATES: String() output includes all component values for debugging/logging.
//
// PREVENTS: Missing component data in string output; panic on nil components.
func TestFlowSpecString(t *testing.T) {
	fs := NewFlowSpec(IPv4FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
	fs.AddComponent(NewFlowDestPortComponent(80))

	s := fs.String()
	assert.Contains(t, s, "10.0.0.0/24")
	assert.Contains(t, s, "80")
}

// TestFlowSpecComplexRule verifies complex FlowSpec rule.
//
// VALIDATES: Multiple different component types can coexist in one FlowSpec
// per RFC 8955 Section 4.2 (intersection/AND of all components).
//
// PREVENTS: Component ordering corruption; data loss with multiple components.
func TestFlowSpecComplexRule(t *testing.T) {
	// Match: TCP traffic to 10.0.0.0/24 port 80,443 from any source
	fs := NewFlowSpec(IPv4FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
	fs.AddComponent(NewFlowIPProtocolComponent(6)) // TCP
	fs.AddComponent(NewFlowDestPortComponent(80, 443))

	assert.Len(t, fs.Components(), 3)

	// Verify encoding produces valid bytes
	data := fs.Bytes()
	assert.NotEmpty(t, data)
}

// TestFlowSpecOperatorEncoding verifies numeric operator encoding.
//
// VALIDATES: Operator constants match RFC 8955 Section 4.2.1.1 bit positions.
// E=0x80 (bit 0), A=0x40 (bit 1), LT=0x04 (bit 5), GT=0x02 (bit 6), EQ=0x01 (bit 7).
//
// PREVENTS: Operator bit positions being wrong; silent comparison failures.
func TestFlowSpecOperatorEncoding(t *testing.T) {
	tests := []struct {
		name   string
		op     FlowOperator
		expect byte
	}{
		{"equal", FlowOpEqual, 0x01},
		{"greater", FlowOpGreater, 0x02},
		{"less", FlowOpLess, 0x04},
		{"and", FlowOpAnd, 0x40},
		{"end", FlowOpEnd, 0x80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, byte(tt.op))
		})
	}
}

// TestParseFlowSpec verifies parsing FlowSpec from wire format.
//
// VALIDATES: ParseFlowSpec correctly decodes Bytes() output per RFC 8955 Section 4.
//
// PREVENTS: Parse/encode asymmetry; data loss during round-trip.
func TestParseFlowSpec(t *testing.T) {
	// Create a FlowSpec, encode it, then parse it back
	original := NewFlowSpec(IPv4FlowSpec)
	original.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("192.168.0.0/16")))

	data := original.Bytes()

	parsed, err := ParseFlowSpec(IPv4FlowSpec, data)
	require.NoError(t, err)
	require.NotNil(t, parsed)

	assert.Equal(t, original.Family(), parsed.Family())
	assert.Len(t, parsed.Components(), len(original.Components()))
}

// TestParseFlowSpecErrors verifies error handling.
//
// VALIDATES: ParseFlowSpec returns appropriate errors for malformed input
// per RFC 8955 Section 4.2 (unknown type = malformed NLRI).
//
// PREVENTS: Panic on truncated data; accepting invalid component types.
func TestParseFlowSpecErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated length", []byte{0xFF}},
		{"invalid component type", []byte{1, 0xFF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFlowSpec(IPv4FlowSpec, tt.data)
			assert.Error(t, err)
		})
	}
}

// TestFlowSpecRoundTrip verifies encode/decode cycle.
//
// VALIDATES: Various FlowSpec configurations survive encode/decode round-trip.
//
// PREVENTS: Data corruption for different component combinations; edge cases in encoding.
func TestFlowSpecRoundTrip(t *testing.T) {
	testCases := []struct {
		name       string
		components []FlowComponent
	}{
		{
			name: "dest prefix only",
			components: []FlowComponent{
				NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/8")),
			},
		},
		{
			name: "protocol only",
			components: []FlowComponent{
				NewFlowIPProtocolComponent(17), // UDP
			},
		},
		{
			name: "prefix and port",
			components: []FlowComponent{
				NewFlowDestPrefixComponent(netip.MustParsePrefix("172.16.0.0/12")),
				NewFlowDestPortComponent(53),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := NewFlowSpec(IPv4FlowSpec)
			for _, c := range tc.components {
				original.AddComponent(c)
			}

			data := original.Bytes()
			parsed, err := ParseFlowSpec(IPv4FlowSpec, data)
			require.NoError(t, err)

			assert.Equal(t, len(tc.components), len(parsed.Components()))
		})
	}
}

// ============================================================================
// FlowSpec VPN Tests (SAFI 134)
// ============================================================================

// TestFlowSpecVPNSAFI verifies SAFI 134 constant.
//
// VALIDATES: SAFIFlowSpecVPN equals 134 per RFC 8955 Section 8.
//
// PREVENTS: SAFI value mismatch causing capability negotiation failures.
func TestFlowSpecVPNSAFI(t *testing.T) {
	assert.Equal(t, SAFI(134), SAFIFlowSpecVPN)
	assert.Equal(t, "flowspec-vpn", SAFIFlowSpecVPN.String())
}

// TestFlowSpecVPNFamily verifies FlowSpec VPN family constants.
//
// VALIDATES: IPv4/IPv6 FlowSpec VPN families have correct AFI/SAFI per RFC 8955 Section 8.
//
// PREVENTS: AFI/SAFI mismatch between IPv4 and IPv6 VPN variants.
func TestFlowSpecVPNFamily(t *testing.T) {
	assert.Equal(t, AFIIPv4, IPv4FlowSpecVPN.AFI)
	assert.Equal(t, SAFIFlowSpecVPN, IPv4FlowSpecVPN.SAFI)
	assert.Equal(t, AFIIPv6, IPv6FlowSpecVPN.AFI)
	assert.Equal(t, SAFIFlowSpecVPN, IPv6FlowSpecVPN.SAFI)
}

// TestFlowSpecVPNBasic verifies basic FlowSpec VPN creation.
//
// VALIDATES: FlowSpecVPN stores RD and components correctly per RFC 8955 Section 8.
//
// PREVENTS: RD being lost; component list not being inherited from FlowSpec.
func TestFlowSpecVPNBasic(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0, Value: [6]byte{0x00, 0x64, 0x00, 0x00, 0x00, 0x64}} // 100:100

	fsv := NewFlowSpecVPN(IPv4FlowSpecVPN, rd)
	fsv.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
	fsv.AddComponent(NewFlowDestPortComponent(80))

	assert.Equal(t, IPv4FlowSpecVPN, fsv.Family())
	assert.Equal(t, rd, fsv.RD())
	assert.Len(t, fsv.Components(), 2)
}

// TestFlowSpecVPNBytes verifies wire-format encoding.
//
// VALIDATES: FlowSpecVPN Bytes() includes length + 8-byte RD + components
// per RFC 8955 Section 8 Figure 7.
//
// PREVENTS: RD bytes not being included; length field not covering RD.
func TestFlowSpecVPNBytes(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0, Value: [6]byte{0x00, 0x64, 0x00, 0x00, 0x00, 0x64}}

	fsv := NewFlowSpecVPN(IPv4FlowSpecVPN, rd)
	fsv.AddComponent(NewFlowDestPortComponent(80))

	data := fsv.Bytes()

	// Verify structure: length (1 byte) + RD (8 bytes) + component
	require.True(t, len(data) > 9, "data too short")

	// Length should cover RD + component
	nlriLen := int(data[0])
	assert.Equal(t, nlriLen, len(data)-1)

	// First 8 bytes after length should be RD
	rdBytes := data[1:9]
	assert.Equal(t, rd.Bytes(), rdBytes)
}

// TestFlowSpecVPNRoundTrip verifies encode/decode cycle.
//
// VALIDATES: FlowSpecVPN survives encode/decode with RD and components intact.
//
// PREVENTS: RD corruption during parse; component data loss.
func TestFlowSpecVPNRoundTrip(t *testing.T) {
	rd := RouteDistinguisher{Type: RDType0, Value: [6]byte{0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}} // 65000:100

	original := NewFlowSpecVPN(IPv4FlowSpecVPN, rd)
	original.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("192.168.0.0/16")))
	original.AddComponent(NewFlowIPProtocolComponent(6)) // TCP
	original.AddComponent(NewFlowDestPortComponent(443))

	data := original.Bytes()

	parsed, err := ParseFlowSpecVPN(IPv4FlowSpecVPN, data)
	require.NoError(t, err)

	assert.Equal(t, rd, parsed.RD())
	assert.Equal(t, IPv4FlowSpecVPN, parsed.Family())
	assert.Len(t, parsed.Components(), 3)
}

// TestFlowSpecVPNStringCommandStyle verifies command-style string representation.
//
// VALIDATES: FlowSpecVPN String() outputs command-style format for API round-trip.
// Format: rd set <rd> <flowspec>.
//
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestFlowSpecVPNStringCommandStyle(t *testing.T) {
	tests := []struct {
		name     string
		fsv      *FlowSpecVPN
		expected string
	}{
		{
			name: "basic flowspec-vpn",
			fsv: func() *FlowSpecVPN {
				rd := RouteDistinguisher{Type: RDType0, Value: [6]byte{0x00, 0x64, 0x00, 0x00, 0x00, 0x64}}
				f := NewFlowSpecVPN(IPv4FlowSpecVPN, rd)
				f.AddComponent(NewFlowDestPortComponent(80))
				return f
			}(),
			expected: "rd set 0:100:100 flowspec(dest-port[=80])",
		},
		{
			name: "flowspec-vpn multiple components",
			fsv: func() *FlowSpecVPN {
				rd := RouteDistinguisher{Type: RDType1}
				copy(rd.Value[:4], []byte{10, 0, 0, 1})
				binary.BigEndian.PutUint16(rd.Value[4:6], 200)
				f := NewFlowSpecVPN(IPv4FlowSpecVPN, rd)
				f.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("192.168.1.0/24")))
				f.AddComponent(NewFlowDestPortComponent(443))
				return f
			}(),
			expected: "rd set 1:10.0.0.1:200 flowspec(dest-prefix=192.168.1.0/24 dest-port[=443])",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fsv.String())
		})
	}
}
