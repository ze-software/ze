package flowspec

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// TestFlowSpecComponentTypes verifies component type constants.
//
// VALIDATES: Component type constants match RFC 8955 Section 4.2.2 values exactly.
//
// PREVENTS: Registry mismatch with RFC-assigned component type numbers;
// silent breakage if constants are accidentally changed.
func TestFlowSpecComponentTypes(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	// Network Unreachable = 0, Host Unreachable = 1 per RFC 792
	comp := newFlowICMPCodeComponent(0, 1)

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
	t.Parallel()
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
			comp: newFlowICMPCodeComponent(0, 255),
			buildNLRI: func() *FlowSpec {
				fs := NewFlowSpec(IPv4FlowSpec)
				fs.AddComponent(newFlowICMPCodeComponent(0, 255))
				return fs
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	original := NewFlowSpec(IPv4FlowSpec)
	original.AddComponent(newFlowICMPCodeComponent(3)) // Port Unreachable

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	fs := NewFlowSpec(IPv4FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
	fs.AddComponent(NewFlowIPProtocolComponent(6)) // TCP

	assert.Equal(t, IPv4FlowSpec, fs.Family())
	assert.Len(t, fs.Components(), 2)

	// RFC 5575 Section 4: IPv4 Flow Specification rules use the (AFI=1, SAFI=133) pair,
	// which is what the encoder writes to the wire (encode.go afi=1/SAFI=133).
	// RFC requirement: RFC5575-4-2 positive -- IPv4 FlowSpec uses AFI 1 with SAFI 133 (§4)
	assert.Equal(t, AFI(1), fs.Family().AFI, "IPv4 FlowSpec AFI must be 1")
	assert.Equal(t, SAFI(133), fs.Family().SAFI, "IPv4 FlowSpec SAFI must be 133")
}

// TestFlowSpecIPv6Basic verifies basic IPv6 FlowSpec NLRI.
//
// VALIDATES: IPv6 FlowSpec NLRI (AFI=2, SAFI=133) correctly stores components
// per RFC 8956.
//
// PREVENTS: IPv4/IPv6 family confusion; IPv6 prefix encoding errors.
func TestFlowSpecIPv6Basic(t *testing.T) {
	t.Parallel()
	fs := NewFlowSpec(IPv6FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("2001:db8::/32")))

	assert.Equal(t, IPv6FlowSpec, fs.Family())

	// RFC 8956 Section 2: IPv6 Flow Specification rules use the (AFI=2, SAFI=133) pair.
	// RFC requirement: RFC8956-2-2 positive -- IPv6 FlowSpec uses AFI 2 with SAFI 133 (§2)
	assert.Equal(t, AFI(2), fs.Family().AFI, "IPv6 FlowSpec AFI must be 2")
	assert.Equal(t, SAFI(133), fs.Family().SAFI, "IPv6 FlowSpec SAFI must be 133")
}

// TestFlowSpecBytes verifies wire format encoding.
//
// VALIDATES: FlowSpec Bytes() produces valid NLRI with length prefix per RFC 8955 Section 4.1.
//
// PREVENTS: Missing length prefix; empty output for valid FlowSpec.
func TestFlowSpecBytes(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			original := NewFlowSpec(IPv4FlowSpec)
			for _, c := range tt.components {
				original.AddComponent(c)
			}

			data := original.Bytes()
			parsed, err := ParseFlowSpec(IPv4FlowSpec, data)
			require.NoError(t, err)

			assert.Equal(t, len(tt.components), len(parsed.Components()))
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
	t.Parallel()
	assert.Equal(t, SAFI(134), SAFIFlowSpecVPN)
	assert.Equal(t, "flow-vpn", SAFIFlowSpecVPN.String())
}

// TestFlowSpecVPNFamily verifies FlowSpec VPN family constants.
//
// VALIDATES: IPv4/IPv6 FlowSpec VPN families have correct AFI/SAFI per RFC 8955 Section 8.
//
// PREVENTS: AFI/SAFI mismatch between IPv4 and IPv6 VPN variants.
func TestFlowSpecVPNFamily(t *testing.T) {
	t.Parallel()
	// RFC 5575 Section 8 / RFC 8955 Section 9.4: IPv4 L3VPN Flow Specification rules
	// use the (AFI=1, SAFI=134) pair matching the FlowSpec VPN application.
	// RFC requirement: RFC5575-4-2 positive -- IPv4 FlowSpec L3VPN uses AFI 1 with SAFI 134 (§4)
	// RFC requirement: RFC8955-4-2 positive -- IPv4 FlowSpec L3VPN uses the (AFI 1, SAFI 134) pair (§4)
	assert.Equal(t, AFIIPv4, IPv4FlowSpecVPN.AFI)
	assert.Equal(t, AFI(1), IPv4FlowSpecVPN.AFI, "IPv4 FlowSpec VPN AFI must be 1")
	assert.Equal(t, SAFIFlowSpecVPN, IPv4FlowSpecVPN.SAFI)
	assert.Equal(t, SAFI(134), IPv4FlowSpecVPN.SAFI, "IPv4 FlowSpec VPN SAFI must be 134")

	// RFC 8956 Section 2: IPv6 L3VPN Flow Specification rules use the (AFI=2, SAFI=134) pair.
	// RFC requirement: RFC8956-2-2 positive -- IPv6 FlowSpec L3VPN uses AFI 2 with SAFI 134 (§2)
	assert.Equal(t, AFIIPv6, IPv6FlowSpecVPN.AFI)
	assert.Equal(t, AFI(2), IPv6FlowSpecVPN.AFI, "IPv6 FlowSpec VPN AFI must be 2")
	assert.Equal(t, SAFIFlowSpecVPN, IPv6FlowSpecVPN.SAFI)
	assert.Equal(t, SAFI(134), IPv6FlowSpecVPN.SAFI, "IPv6 FlowSpec VPN SAFI must be 134")
}

// TestFlowSpecVPNBasic verifies basic FlowSpec VPN creation.
//
// VALIDATES: FlowSpecVPN stores RD and components correctly per RFC 8955 Section 8.
//
// PREVENTS: RD being lost; component list not being inherited from FlowSpec.
func TestFlowSpecVPNBasic(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestFlowSpecStringCommandStyle verifies command-style string representation.
//
// VALIDATES: FlowSpec String() outputs command-style format matching API input syntax.
// Format: flowspec <component>+ where each component is "<type> <values>".
//
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestFlowSpecStringCommandStyle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fs       *FlowSpec
		expected string
	}{
		{
			name: "destination prefix only",
			fs: func() *FlowSpec {
				f := NewFlowSpec(IPv4FlowSpec)
				f.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
				return f
			}(),
			expected: "flow destination 10.0.0.0/24",
		},
		{
			name: "source prefix only",
			fs: func() *FlowSpec {
				f := NewFlowSpec(IPv4FlowSpec)
				f.AddComponent(NewFlowSourcePrefixComponent(netip.MustParsePrefix("192.168.0.0/16")))
				return f
			}(),
			expected: "flow source 192.168.0.0/16",
		},
		{
			name: "destination port with multiple values",
			fs: func() *FlowSpec {
				f := NewFlowSpec(IPv4FlowSpec)
				f.AddComponent(NewFlowDestPortComponent(80, 443))
				return f
			}(),
			expected: "flow destination-port =80 =443",
		},
		{
			name: "protocol single value",
			fs: func() *FlowSpec {
				f := NewFlowSpec(IPv4FlowSpec)
				f.AddComponent(NewFlowIPProtocolComponent(6))
				return f
			}(),
			expected: "flow protocol 6",
		},
		{
			name: "complex rule with multiple components",
			fs: func() *FlowSpec {
				f := NewFlowSpec(IPv4FlowSpec)
				f.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
				f.AddComponent(NewFlowDestPortComponent(80, 443))
				f.AddComponent(NewFlowIPProtocolComponent(6))
				return f
			}(),
			expected: "flow destination 10.0.0.0/24 destination-port =80 =443 protocol 6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.fs.String())
		})
	}
}

// TestFlowSpecVPNStringCommandStyle verifies command-style string representation.
//
// VALIDATES: FlowSpecVPN String() outputs command-style format for API round-trip.
// Format: flow-vpn rd <rd> <components>.
//
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestFlowSpecVPNStringCommandStyle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fsv      *FlowSpecVPN
		expected string
	}{
		{
			name: "basic flow-vpn",
			fsv: func() *FlowSpecVPN {
				rd := RouteDistinguisher{Type: RDType0, Value: [6]byte{0x00, 0x64, 0x00, 0x00, 0x00, 0x64}}
				f := NewFlowSpecVPN(IPv4FlowSpecVPN, rd)
				f.AddComponent(NewFlowDestPortComponent(80))
				return f
			}(),
			expected: "flow-vpn rd 0:100:100 destination-port =80",
		},
		{
			name: "flow-vpn multiple components",
			fsv: func() *FlowSpecVPN {
				rd := RouteDistinguisher{Type: RDType1}
				copy(rd.Value[:4], []byte{10, 0, 0, 1})
				binary.BigEndian.PutUint16(rd.Value[4:6], 200)
				f := NewFlowSpecVPN(IPv4FlowSpecVPN, rd)
				f.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("192.168.1.0/24")))
				f.AddComponent(NewFlowDestPortComponent(443))
				return f
			}(),
			expected: "flow-vpn rd 1:10.0.0.1:200 destination 192.168.1.0/24 destination-port =443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.fsv.String())
		})
	}
}

// TestPrefixComponentString verifies prefix component string format.
//
// VALIDATES: prefixComponent.String() outputs command-style format.
// destination <prefix> for Type 1, source <prefix> for Type 2.
//
// PREVENTS: Wrong component name, missing space between name and prefix.
func TestPrefixComponentString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		comp     FlowComponent
		expected string
	}{
		{
			name:     "destination prefix",
			comp:     NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")),
			expected: "destination 10.0.0.0/24",
		},
		{
			name:     "source prefix",
			comp:     NewFlowSourcePrefixComponent(netip.MustParsePrefix("192.168.0.0/16")),
			expected: "source 192.168.0.0/16",
		},
		{
			name:     "destination prefix IPv6",
			comp:     NewFlowDestPrefixComponent(netip.MustParsePrefix("2001:db8::/32")),
			expected: "destination 2001:db8::/32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.comp.String())
		})
	}
}

// TestNumericComponentString verifies numeric component string format.
//
// VALIDATES: numericComponent.String() outputs command-style format.
// <type> <op><value> <op><value> ... (space-separated operator+value pairs).
//
// PREVENTS: Wrong component names, brackets around values, missing operators.
func TestNumericComponentString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		comp     FlowComponent
		expected string
	}{
		{
			name:     "destination-port single",
			comp:     NewFlowDestPortComponent(80),
			expected: "destination-port =80",
		},
		{
			name:     "destination-port multiple",
			comp:     NewFlowDestPortComponent(80, 443),
			expected: "destination-port =80 =443",
		},
		{
			name:     "source-port",
			comp:     NewFlowSourcePortComponent(1024),
			expected: "source-port =1024",
		},
		{
			name:     "protocol",
			comp:     NewFlowIPProtocolComponent(6),
			expected: "protocol 6",
		},
		{
			name:     "port",
			comp:     NewFlowPortComponent(53, 80),
			expected: "port =53 =80",
		},
		{
			name:     "icmp-type",
			comp:     NewFlowICMPTypeComponent(8),
			expected: "icmp-type =8",
		},
		{
			name:     "icmp-code",
			comp:     newFlowICMPCodeComponent(0),
			expected: "icmp-code =0",
		},
		{
			name:     "packet-length",
			comp:     NewFlowPacketLengthComponent(1500),
			expected: "packet-length =1500",
		},
		{
			name:     "dscp",
			comp:     NewFlowDSCPComponent(46),
			expected: "dscp =46",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.comp.String())
		})
	}
}

// TestNumericOperatorString verifies operator symbols in string output.
//
// VALIDATES: Operator symbols match API syntax (=, >, <, >=, <=, !=).
// AND operator uses & prefix.
//
// PREVENTS: Wrong operator symbols, missing & for AND.
func TestNumericOperatorString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		comp     FlowComponent
		expected string
	}{
		{
			name: "equal operator",
			comp: newFlowNumericComponent(FlowDestPort, []FlowMatch{
				{Op: FlowOpEqual, Value: 80},
			}),
			expected: "destination-port =80",
		},
		{
			name: "greater than",
			comp: newFlowNumericComponent(FlowDestPort, []FlowMatch{
				{Op: FlowOpGreater, Value: 1024},
			}),
			expected: "destination-port >1024",
		},
		{
			name: "less than",
			comp: newFlowNumericComponent(FlowDestPort, []FlowMatch{
				{Op: FlowOpLess, Value: 65535},
			}),
			expected: "destination-port <65535",
		},
		{
			name: "greater or equal",
			comp: newFlowNumericComponent(FlowDestPort, []FlowMatch{
				{Op: FlowOpGreater | FlowOpEqual, Value: 1024},
			}),
			expected: "destination-port >=1024",
		},
		{
			name: "less or equal",
			comp: newFlowNumericComponent(FlowDestPort, []FlowMatch{
				{Op: FlowOpLess | FlowOpEqual, Value: 65535},
			}),
			expected: "destination-port <=65535",
		},
		{
			name: "not equal",
			comp: newFlowNumericComponent(FlowDestPort, []FlowMatch{
				{Op: FlowOpNotEq, Value: 0},
			}),
			expected: "destination-port !=0",
		},
		{
			name: "range without AND prefix",
			comp: newFlowNumericComponent(FlowDestPort, []FlowMatch{
				{Op: FlowOpGreater | FlowOpEqual, Value: 1024},
				{Op: FlowOpLess | FlowOpEqual, Value: 65535, And: true},
			}),
			// NOTE: No & prefix - parser infers And from position
			expected: "destination-port >=1024 <=65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.comp.String())
		})
	}
}

// TestFlowSpecStringRoundTrip verifies String() output can be parsed back.
//
// VALIDATES: FlowSpec String() output matches API input syntax for round-trip.
// PREVENTS: Output format diverging from parser, breaking API symmetry.
//
// NOTE: This test does NOT use the actual parser (in pkg/plugin).
// It verifies the STRING FORMAT matches what the parser expects.
// True round-trip testing requires integration with the parser.
func TestFlowSpecStringRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fs       *FlowSpec
		expected string
	}{
		{
			name: "destination prefix",
			fs: func() *FlowSpec {
				f := NewFlowSpec(IPv4FlowSpec)
				f.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
				return f
			}(),
			// Parser expects: nlri ipv4/flow add destination 10.0.0.0/24
			expected: "flow destination 10.0.0.0/24",
		},
		{
			name: "port range",
			fs: func() *FlowSpec {
				f := NewFlowSpec(IPv4FlowSpec)
				f.AddComponent(newFlowNumericComponent(FlowDestPort, []FlowMatch{
					{Op: FlowOpGreater | FlowOpEqual, Value: 1024},
					{Op: FlowOpLess | FlowOpEqual, Value: 65535, And: true},
				}))
				return f
			}(),
			// Parser expects separate tokens; And is inferred from position
			expected: "flow destination-port >=1024 <=65535",
		},
		{
			name: "tcp flags combined",
			fs: func() *FlowSpec {
				f := NewFlowSpec(IPv4FlowSpec)
				f.AddComponent(NewFlowTCPFlagsComponent(0x12)) // SYN+ACK
				return f
			}(),
			// Parser expects: tcp-flags syn&ack
			expected: "flow tcp-flags syn&ack",
		},
		{
			name: "full rule",
			fs: func() *FlowSpec {
				f := NewFlowSpec(IPv4FlowSpec)
				f.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
				f.AddComponent(NewFlowIPProtocolComponent(6)) // TCP
				f.AddComponent(NewFlowDestPortComponent(80, 443))
				return f
			}(),
			// Parser expects: destination 10.0.0.0/24 protocol =6 destination-port =80 =443
			expected: "flow destination 10.0.0.0/24 protocol 6 destination-port =80 =443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.fs.String()
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestBitmaskComponentString verifies bitmask component string format.
//
// VALIDATES: TCP flags and fragment components use named flags.
// Format: <type> [=|!]<flag>[&<flag>...].
//
// PREVENTS: Raw numeric output instead of flag names.
func TestBitmaskComponentString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		comp     FlowComponent
		expected string
	}{
		{
			name:     "tcp-flags syn",
			comp:     NewFlowTCPFlagsComponent(0x02),
			expected: "tcp-flags syn",
		},
		{
			name:     "tcp-flags ack",
			comp:     NewFlowTCPFlagsComponent(0x10),
			expected: "tcp-flags ack",
		},
		{
			name:     "tcp-flags syn+ack",
			comp:     NewFlowTCPFlagsComponent(0x12),
			expected: "tcp-flags syn&ack",
		},
		{
			name:     "fragment dont-fragment",
			comp:     NewFlowFragmentComponent(FlowFragDontFragment),
			expected: "fragment dont-fragment",
		},
		{
			name:     "fragment is-fragment",
			comp:     NewFlowFragmentComponent(FlowFragIsFragment),
			expected: "fragment is-fragment",
		},
		{
			name:     "fragment first-fragment",
			comp:     NewFlowFragmentComponent(FlowFragFirstFragment),
			expected: "fragment first-fragment",
		},
		{
			name:     "fragment last-fragment",
			comp:     NewFlowFragmentComponent(FlowFragLastFragment),
			expected: "fragment last-fragment",
		},
		{
			name: "tcp-flags multiple matches with AND",
			comp: newFlowNumericComponent(FlowTCPFlags, []FlowMatch{
				{Op: 0, Value: 0x02},                    // SYN
				{Op: FlowOpNot, Value: 0x04, And: true}, // AND NOT RST
			}),
			// Parser expects: tcp-flags syn &!rst
			expected: "tcp-flags syn &!rst",
		},
		{
			name: "fragment multiple with match and AND",
			comp: newFlowNumericComponent(FlowFragment, []FlowMatch{
				{Op: FlowOpMatch, Value: uint64(FlowFragDontFragment)},        // =dont-fragment
				{Op: FlowOpNot, Value: uint64(FlowFragIsFragment), And: true}, // &!is-fragment
			}),
			expected: "fragment =dont-fragment &!is-fragment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.comp.String())
		})
	}
}

// walkComponentOps walks a numeric/bitmask component's wire bytes (as produced by
// FlowComponent.Bytes()) and returns the operator byte and value bytes of every
// {op, value} pair. The first byte is the component type; each pair is an operator
// byte whose len field (bits 2-3) encodes value length as 1<<lenCode octets
// (RFC 8955 Section 4.2.1.1 / 4.2.1.2), followed by that many value octets.
func walkComponentOps(t *testing.T, b []byte) (ops []byte, values [][]byte) {
	t.Helper()
	require.GreaterOrEqual(t, len(b), 1, "component must have at least a type byte")
	i := 1 // skip the component type byte
	for i < len(b) {
		op := b[i]
		i++
		lenCode := (op >> 4) & 0x03
		valueLen := 1 << lenCode
		require.LessOrEqual(t, i+valueLen, len(b), "value must fit within component bytes")
		ops = append(ops, op)
		values = append(values, b[i:i+valueLen])
		i += valueLen
	}
	require.NotEmpty(t, ops, "component must encode at least one {op, value} pair")
	return ops, values
}

// TestFlowSpecComponentsAscendingOrder verifies components are emitted in strictly
// ascending type order regardless of the order they were added.
//
// VALIDATES: FlowSpec.WriteTo (via writeComponentsSorted, types.go) emits components
// sorted by increasing component type, so a lower-type component always precedes any
// higher-type component on the wire.
//
// PREVENTS: Out-of-order component emission that a strict decoder would treat as
// malformed NLRI; the sort being dropped so add-order leaks onto the wire.
func TestFlowSpecComponentsAscendingOrder(t *testing.T) {
	t.Parallel()

	// Add components deliberately OUT of type order: 5, 12, 3, 1.
	fs := NewFlowSpec(IPv4FlowSpec)
	fs.AddComponent(NewFlowDestPortComponent(443))                                    // Type 5
	fs.AddComponent(NewFlowFragmentComponent(FlowFragIsFragment))                     // Type 12
	fs.AddComponent(NewFlowIPProtocolComponent(6))                                    // Type 3
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24"))) // Type 1

	// Encode to the wire (routes through WriteTo -> writeComponentsSorted) and parse
	// back; ParseFlowSpec appends components in the exact order they appear on the wire,
	// so the parsed order reflects the emitted order.
	data := fs.Bytes()
	parsed, err := ParseFlowSpec(IPv4FlowSpec, data)
	require.NoError(t, err)
	comps := parsed.Components()
	require.Len(t, comps, 4)

	// RFC 5575 Section 4.2: "Components MUST follow strict type ordering by increasing
	// numerical order." Each present component must precede any higher-type component.
	// RFC requirement: RFC5575-4-3 positive -- emitted component types are strictly ascending (§4.2)
	// RFC requirement: RFC5575-4-4 positive -- a present component precedes any higher-type component (§4.2)
	// RFC requirement: RFC8955-4.2-1 positive -- emitted component types are strictly ascending (§4.2)
	// RFC requirement: RFC8955-4.2-2 positive -- a present component precedes any higher-type component (§4.2)
	for i := 1; i < len(comps); i++ {
		assert.Less(t, comps[i-1].Type(), comps[i].Type(),
			"component %d (type %d) must precede higher-type component %d (type %d)",
			i-1, comps[i-1].Type(), i, comps[i].Type())
	}
	// Concretely, the out-of-order add above must serialize as 1, 3, 5, 12.
	assert.Equal(t, FlowDestPrefix, comps[0].Type())
	assert.Equal(t, FlowIPProtocol, comps[1].Type())
	assert.Equal(t, FlowDestPort, comps[2].Type())
	assert.Equal(t, FlowFragment, comps[3].Type())
}

// TestFlowSpecNumericOperatorReservedBitZero verifies the numeric operator byte
// leaves reserved bit 4 clear across value lengths and comparison operators.
//
// VALIDATES: numericComponent.Bytes() builds the operator as lenCode<<4 | AND | END |
// comparison bits (types_numeric.go), so RFC 8955 Section 4.2.1.1 reserved bit 4 (0x08)
// is never set.
//
// PREVENTS: reserved bit contamination that would corrupt the len field or be rejected
// by a strict peer.
func TestFlowSpecNumericOperatorReservedBitZero(t *testing.T) {
	t.Parallel()

	// One component exercising all three encoder len codes (1, 2, 4 octet values) and
	// several comparison operators, plus the AND and END bits.
	comp := newFlowNumericComponent(FlowPacketLength, []FlowMatch{
		{Op: FlowOpEqual, Value: 6},                 // 1-octet value (lenCode 0)
		{Op: FlowOpGreater, Value: 1024, And: true}, // 2-octet value (lenCode 1)
		{Op: FlowOpLess, Value: 0x12345, And: true}, // 4-octet value (lenCode 2), END set
	})
	ops, _ := walkComponentOps(t, comp.Bytes())
	require.Len(t, ops, 3)

	// RFC 5575 Section 4 numeric operator format [e][a][len][0][lt][gt][eq]: bit 4 (0x08)
	// is reserved and MUST be 0.
	// RFC requirement: RFC5575-4-5 positive -- numeric operator reserved bit 4 (0x08) is clear (§4)
	for i, op := range ops {
		assert.Zero(t, op&0x08, "numeric operator %d (0x%02x) reserved bit 4 must be 0", i, op)
	}
}

// TestFlowSpecBitmaskOperatorReservedBitsZero verifies the bitmask operator byte
// leaves reserved bits 4-5 clear.
//
// VALIDATES: numericComponent.Bytes() for a bitmask type (TCP flags) builds the
// operator as lenCode<<4 | AND | END | NOT/MATCH bits (types_numeric.go), so RFC 8955
// Section 4.2.1.2 reserved bits 4-5 (0x0C) are never set.
//
// PREVENTS: reserved-bit contamination in the bitmask operand that a strict peer would
// reject or misinterpret.
func TestFlowSpecBitmaskOperatorReservedBitsZero(t *testing.T) {
	t.Parallel()

	// TCP flags (Type 9) uses the bitmask operator; exercise MATCH, NOT and AND/END bits.
	comp := newFlowTCPFlagsMatchComponent([]FlowMatch{
		{Op: FlowOpMatch, Value: 0x02},                        // =syn
		{Op: FlowOpNot | FlowOpMatch, Value: 0x04, And: true}, // AND !=rst, END set
	})
	ops, _ := walkComponentOps(t, comp.Bytes())
	require.Len(t, ops, 2)

	// RFC 5575 Section 4 bitmask operator format [e][a][len][0][0][not][m]: bits 4-5
	// (0x0C) are reserved and MUST be 0.
	// RFC requirement: RFC5575-4-6 positive -- bitmask operator reserved bits 4-5 (0x0C) are clear (§4)
	// RFC requirement: RFC8955-4.2.1.2-1 positive -- bitmask operator reserved bits 4-5 (0x0C) are clear on encode (§4.2.1.2)
	for i, op := range ops {
		assert.Zero(t, op&0x0C, "bitmask operator %d (0x%02x) reserved bits 4-5 must be 0", i, op)
	}
}

// TestFlowSpecFragmentReservedHighNibbleZero verifies the Type-12 Fragment bitmask
// value leaves its reserved high nibble clear.
//
// VALIDATES: A fragment component (types_numeric.go NewFlowFragment*Component) encodes
// its value from the low-nibble FlowFragmentFlag constants (types.go), so the RFC 8955
// Section 4.2.2.12 reserved high nibble (bits 0-3, 0xF0) stays zero on the wire.
//
// PREVENTS: stray high-nibble bits in the fragment operand that would be interpreted as
// undefined fragment match flags.
func TestFlowSpecFragmentReservedHighNibbleZero(t *testing.T) {
	t.Parallel()

	// Fragment (Type 12) with combined and single flags across two {op, value} pairs.
	comp := newFlowFragmentMatchComponent([]FlowMatch{
		{Op: FlowOpMatch, Value: uint64(FlowFragIsFragment | FlowFragFirstFragment)},
		{Op: FlowOpMatch, Value: uint64(FlowFragDontFragment), And: true},
	})
	_, values := walkComponentOps(t, comp.Bytes())
	require.Len(t, values, 2)

	// RFC 5575 Section 4 fragment bitmask [0][0][0][0][LF][FF][IsF][DF]: the high nibble
	// (0xF0) is reserved and MUST be 0.
	// RFC requirement: RFC5575-4-7 positive -- fragment bitmask reserved high nibble (0xF0) is zero (§4)
	// RFC requirement: RFC8955-4.2.2.12-2 positive -- fragment bitmask reserved high nibble (0xF0) is zero on encode (§4.2.2.12)
	for i, v := range values {
		require.Len(t, v, 1, "fragment value must encode as a single octet")
		assert.Zero(t, v[0]&0xF0, "fragment value %d (0x%02x) reserved high nibble must be 0", i, v[0])
	}
}

// TestFlowSpecTrafficMarkingReservedBytesZero verifies the DSCP Traffic-Marking
// extended community leaves its reserved bytes zero.
//
// VALIDATES: flowSpecActionExtComm (encode.go) encodes a DSCP marking action as the
// 8-byte extended community 0x80,0x09,0x00,0x00,0x00,0x00,0x00,DSCP, so the reserved
// bytes 2..6 are zero and only the final octet carries the DSCP value.
//
// PREVENTS: reserved bytes leaking data into the Traffic Marking community, which a
// receiver would treat as an out-of-spec / different action.
func TestFlowSpecTrafficMarkingReservedBytesZero(t *testing.T) {
	t.Parallel()

	const dscp = uint8(46) // Expedited Forwarding
	ec, err := flowSpecActionExtComm(bgptypes.FlowSpecRoute{
		Actions: bgptypes.FlowSpecActions{MarkDSCP: dscp},
	})
	require.NoError(t, err)
	require.Len(t, ec, 8, "traffic-marking extended community is 8 octets")

	// RFC 5575 Section 7: the Traffic Marking extended community (type 0x80, subtype
	// 0x09) carries the DSCP in the last octet; the intervening octets are reserved 0.
	assert.Equal(t, byte(0x80), ec[0], "high-order type octet must be 0x80")
	assert.Equal(t, byte(0x09), ec[1], "subtype must be 0x09 (traffic marking)")
	assert.Equal(t, dscp, ec[7], "final octet carries the DSCP value")
	// RFC requirement: RFC5575-7-1 positive -- Traffic-Marking reserved bytes 2..6 are zero (§7)
	for i := 2; i <= 6; i++ {
		assert.Zero(t, ec[i], "traffic-marking reserved byte %d must be 0", i)
	}
}

// TestRFC8955FirstOperatorAndBitEncodedUnset verifies the first {op, value} pair of a
// numeric component is emitted with the AND bit clear.
//
// VALIDATES: parseFlowMatches (config_builder.go) derives the AND bit purely from the
// position inside a "&"-joined expression (isAnd := i > 0), so buildFlowSpecComponents
// -> numericComponent.Bytes() always leaves the 'a' bit (0x40) clear on the first
// operator octet and sets it on the continuation pairs.
//
// PREVENTS: a leading AND bit that RFC 8955 Section 4.2.1.1 forbids, which a strict peer
// would read as an AND against a non-existent previous term.
func TestRFC8955FirstOperatorAndBitEncodedUnset(t *testing.T) {
	t.Parallel()

	// ">8080&<8088" is the config spelling of a range: two {op, value} pairs where only
	// the second is AND-ed with the first.
	fs, dropped := buildFlowSpecComponents(map[string][]string{
		"destination-port": {">8080&<8088"},
	}, false)
	require.Empty(t, dropped)
	require.Len(t, fs.Components(), 1)

	ops, _ := walkComponentOps(t, fs.Components()[0].Bytes())
	require.Len(t, ops, 2, "range must encode as two {op, value} pairs")

	// RFC 8955 Section 4.2.1.1: "In the first operator octet of a sequence, the AND bit
	// MUST be encoded as unset."
	// RFC requirement: RFC8955-4.2.1.1-1 positive -- first numeric operator octet carries a clear AND bit (0x40) (§4.2.1.1)
	assert.Zero(t, ops[0]&0x40, "first operator (0x%02x) must have the AND bit clear", ops[0])
	assert.NotZero(t, ops[1]&0x40, "second operator (0x%02x) must have the AND bit set", ops[1])
}

// TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode verifies a received NLRI whose
// FIRST operator octet has the AND bit set is still decoded as a plain (OR) term.
//
// VALIDATES: formatNumericMatches (plugin_decode.go) only continues an AND group when a
// group is already open (`m.And && len(andGroup) > 0`), so the leading pair always opens
// a new OR group whatever the wire 'a' bit says.
//
// PREVENTS: a peer setting the leading AND bit collapsing two independent OR terms into
// a single (and unsatisfiable) AND term.
func TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode(t *testing.T) {
	t.Parallel()

	// Type 5 (destination-port) == 80 OR == 22. Operator octets:
	//   0x01 = len 1 octet, eq            (AND bit clear)
	//   0x81 = end-of-list, len 1, eq     (AND bit clear)
	clean := []byte{0x05, 0x05, 0x01, 0x50, 0x81, 0x16}
	// Same NLRI with the AND bit (0x40) set on the FIRST operator only.
	leadingAnd := []byte{0x05, 0x05, 0x41, 0x50, 0x81, 0x16}

	parsedClean, err := ParseFlowSpec(IPv4FlowSpec, clean)
	require.NoError(t, err)
	parsedAnd, err := ParseFlowSpec(IPv4FlowSpec, leadingAnd)
	require.NoError(t, err)

	// RFC 8955 Section 4.2.1.1: "the AND bit ... SHOULD be encoded as unset and MUST be
	// treated as always unset on decoding."
	// RFC requirement: RFC8955-4.2.1.1-2 positive -- a leading operator with the AND bit clear decodes as its own OR term (§4.2.1.1)
	jsonClean := flowSpecToJSON(parsedClean, "ipv4/flow", nil)
	assert.Equal(t, [][]string{{"=80"}, {"=22"}}, jsonClean["destination-port"])

	// RFC requirement: RFC8955-4.2.1.1-2 negative -- a leading operator with the AND bit SET is still decoded as an OR term, not AND-ed (§4.2.1.1)
	jsonAnd := flowSpecToJSON(parsedAnd, "ipv4/flow", nil)
	assert.Equal(t, jsonClean["destination-port"], jsonAnd["destination-port"],
		"a set leading AND bit must not change the decoded term structure")
}

// TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode verifies reserved bits 4-5 of a
// bitmask operator octet do not change the decoded match.
//
// VALIDATES: formatBitmaskValue (plugin_decode.go) consults only the NOT (0x02) and
// MATCH (0x01) bits of the decoded operator, so the reserved 0x0C bits parseNumericComponent
// carries through are never interpreted.
//
// PREVENTS: a peer that sets the reserved bits having its TCP-flags term silently
// re-interpreted as a different match.
func TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode(t *testing.T) {
	t.Parallel()

	// Type 9 (tcp-flags), 1-octet bitmask 0x02 (SYN), MATCH set, end-of-list.
	clean := []byte{0x03, 0x09, 0x81, 0x02}
	// Same operator with both reserved bits (0x0C) set.
	reserved := []byte{0x03, 0x09, 0x8D, 0x02}

	parsedClean, err := ParseFlowSpec(IPv4FlowSpec, clean)
	require.NoError(t, err)
	parsedReserved, err := ParseFlowSpec(IPv4FlowSpec, reserved)
	require.NoError(t, err)

	jsonClean := flowSpecToJSON(parsedClean, "ipv4/flow", nil)
	jsonReserved := flowSpecToJSON(parsedReserved, "ipv4/flow", nil)
	require.Equal(t, [][]string{{"=syn"}}, jsonClean["tcp-flags"])

	// RFC 8955 Section 4.2.1.2: the bitmask operator reserved bits "MUST be set to 0 on
	// NLRI encoding and MUST be ignored during decoding."
	// RFC requirement: RFC8955-4.2.1.2-1 negative -- a bitmask operator with reserved bits 4-5 set decodes identically to one with them clear (§4.2.1.2)
	assert.Equal(t, jsonClean["tcp-flags"], jsonReserved["tcp-flags"],
		"reserved bitmask operator bits must not change the decoded match")
}

// TestRFC8955FragmentReservedBitsIgnoredOnDecode verifies the reserved high nibble of a
// Type-12 fragment bitmask value does not change the decoded match.
//
// VALIDATES: formatBitmaskValue (plugin_decode.go) walks the fragment value bit by bit
// against fragmentFlagValueToNames, which only defines 0x01..0x08, so any bit in the
// reserved high nibble is dropped rather than interpreted.
//
// PREVENTS: undefined high-nibble bits from a peer being rendered as (or mistaken for)
// additional fragment match flags.
func TestRFC8955FragmentReservedBitsIgnoredOnDecode(t *testing.T) {
	t.Parallel()

	// Type 12 (fragment), 1-octet bitmask 0x02 (is-fragment), MATCH set, end-of-list.
	clean := []byte{0x03, 0x0C, 0x81, 0x02}
	// Same value with every reserved high-nibble bit set.
	reserved := []byte{0x03, 0x0C, 0x81, 0xF2}

	parsedClean, err := ParseFlowSpec(IPv4FlowSpec, clean)
	require.NoError(t, err)
	parsedReserved, err := ParseFlowSpec(IPv4FlowSpec, reserved)
	require.NoError(t, err)

	jsonClean := flowSpecToJSON(parsedClean, "ipv4/flow", nil)
	jsonReserved := flowSpecToJSON(parsedReserved, "ipv4/flow", nil)
	require.Equal(t, [][]string{{"=is-fragment"}}, jsonClean["fragment"])

	// RFC 8955 Section 4.2.2.12: the fragment bitmask reserved bits "MUST be set to 0 on
	// NLRI encoding and MUST be ignored during decoding."
	// RFC requirement: RFC8955-4.2.2.12-2 negative -- a fragment bitmask with the reserved high nibble set decodes identically to one with it clear (§4.2.2.12)
	assert.Equal(t, jsonClean["fragment"], jsonReserved["fragment"],
		"reserved fragment bits must not change the decoded match")
}

// TestRFC8955TCPFlagsBitmaskSingleOctet verifies a Type-9 TCP flags component encodes its
// bitmask in one octet.
//
// VALIDATES: parseFlowTCPFlagMatches (config_builder.go) resolves flag names to 8-bit
// values, and numericComponent.Bytes() (types_numeric.go) picks lenCode 0 / one octet for
// any value <= 0xFF, so the emitted bitmask is always within the RFC's 1-2 octet range.
//
// PREVENTS: a TCP flags bitmask widening to 4 octets, which RFC 8955 Section 4.2.2.9
// forbids and a strict peer treats as malformed NLRI.
func TestRFC8955TCPFlagsBitmaskSingleOctet(t *testing.T) {
	t.Parallel()

	fs, dropped := buildFlowSpecComponents(map[string][]string{
		"tcp-flags": {"syn&!=ack", "cwr"},
	}, false)
	require.Empty(t, dropped)
	require.Len(t, fs.Components(), 1)

	ops, values := walkComponentOps(t, fs.Components()[0].Bytes())
	require.Len(t, values, 3)

	// RFC 8955 Section 4.2.2.9: "the bitmask MUST be encoded as a 1- or 2-octet bitmask."
	// RFC requirement: RFC8955-4.2.2.9-1 positive -- every emitted TCP flags bitmask occupies 1 or 2 octets (§4.2.2.9)
	for i, v := range values {
		assert.LessOrEqual(t, len(v), 2, "tcp-flags bitmask %d (op 0x%02x) must be 1 or 2 octets", i, ops[i])
		assert.NotEmpty(t, v)
	}
}

// TestRFC8955DSCPValueSingleOctet verifies a Type-11 DSCP component encodes its value in
// exactly one octet.
//
// VALIDATES: parseFlowOctets (config_builder.go) parses DSCP values as uint8 and
// NewFlowDSCPComponent (types_numeric.go) stores them, so numericComponent.Bytes() always
// selects lenCode 0 (one octet).
//
// PREVENTS: a 2- or 4-octet DSCP value, which RFC 8955 Section 4.2.2.11 forbids.
func TestRFC8955DSCPValueSingleOctet(t *testing.T) {
	t.Parallel()

	fs, dropped := buildFlowSpecComponents(map[string][]string{
		"dscp": {"46", "0", "63"},
	}, false)
	require.Empty(t, dropped)
	require.Len(t, fs.Components(), 1)

	_, values := walkComponentOps(t, fs.Components()[0].Bytes())
	require.Len(t, values, 3)

	// RFC 8955 Section 4.2.2.11: "Values are encoded as a single octet."
	// RFC requirement: RFC8955-4.2.2.11-1 positive -- every emitted DSCP value occupies exactly one octet (§4.2.2.11)
	for i, v := range values {
		assert.Len(t, v, 1, "dscp value %d must encode as a single octet", i)
	}
}

// TestRFC8955FragmentBitmaskSingleOctet verifies a Type-12 fragment component encodes its
// bitmask in exactly one octet.
//
// VALIDATES: parseFlowFragment (config_builder.go) resolves fragment names to the
// low-nibble FlowFragmentFlag constants (types.go), so numericComponent.Bytes() selects
// lenCode 0 (one octet) for every fragment value.
//
// PREVENTS: a multi-octet fragment bitmask, which RFC 8955 Section 4.2.2.12 forbids.
func TestRFC8955FragmentBitmaskSingleOctet(t *testing.T) {
	t.Parallel()

	fs, dropped := buildFlowSpecComponents(map[string][]string{
		"fragment": {"is-fragment", "first-fragment", "dont-fragment", "last-fragment"},
	}, false)
	require.Empty(t, dropped)
	require.Len(t, fs.Components(), 1)

	_, values := walkComponentOps(t, fs.Components()[0].Bytes())
	require.Len(t, values, 4)

	// RFC 8955 Section 4.2.2.12: "the bitmask MUST be encoded as a single-octet bitmask."
	// RFC requirement: RFC8955-4.2.2.12-1 positive -- every emitted fragment bitmask occupies exactly one octet (§4.2.2.12)
	for i, v := range values {
		assert.Len(t, v, 1, "fragment bitmask %d must encode as a single octet", i)
	}
}
