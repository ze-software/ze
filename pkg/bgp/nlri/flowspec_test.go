package nlri

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFlowSpecComponentTypes verifies component type constants.
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
}

// TestFlowSpecDestPrefix verifies destination prefix component.
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
func TestFlowSpecSourcePrefix(t *testing.T) {
	prefix := netip.MustParsePrefix("192.168.1.0/24")
	comp := NewFlowSourcePrefixComponent(prefix)

	assert.Equal(t, FlowSourcePrefix, comp.Type())
	pc, ok := comp.(interface{ Prefix() netip.Prefix })
	require.True(t, ok)
	assert.Equal(t, prefix, pc.Prefix())
}

// TestFlowSpecIPProtocol verifies IP protocol component.
func TestFlowSpecIPProtocol(t *testing.T) {
	// TCP = 6
	comp := NewFlowIPProtocolComponent(6)

	assert.Equal(t, FlowIPProtocol, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(6))
}

// TestFlowSpecPort verifies port component (src or dst).
func TestFlowSpecPort(t *testing.T) {
	comp := NewFlowPortComponent(80, 443)

	assert.Equal(t, FlowPort, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(80))
	assert.Contains(t, nc.Values(), uint64(443))
}

// TestFlowSpecDestPort verifies destination port component.
func TestFlowSpecDestPort(t *testing.T) {
	comp := NewFlowDestPortComponent(22)

	assert.Equal(t, FlowDestPort, comp.Type())
	nc, ok := comp.(interface{ Values() []uint64 })
	require.True(t, ok)
	assert.Contains(t, nc.Values(), uint64(22))
}

// TestFlowSpecSourcePort verifies source port component.
func TestFlowSpecSourcePort(t *testing.T) {
	comp := NewFlowSourcePortComponent(1024, 65535)

	assert.Equal(t, FlowSourcePort, comp.Type())
}

// TestFlowSpecTCPFlags verifies TCP flags component.
func TestFlowSpecTCPFlags(t *testing.T) {
	// SYN flag
	comp := NewFlowTCPFlagsComponent(0x02)

	assert.Equal(t, FlowTCPFlags, comp.Type())
}

// TestFlowSpecPacketLength verifies packet length component.
func TestFlowSpecPacketLength(t *testing.T) {
	comp := NewFlowPacketLengthComponent(64, 1500)

	assert.Equal(t, FlowPacketLength, comp.Type())
}

// TestFlowSpecDSCP verifies DSCP component.
func TestFlowSpecDSCP(t *testing.T) {
	// EF = 46
	comp := NewFlowDSCPComponent(46)

	assert.Equal(t, FlowDSCP, comp.Type())
}

// TestFlowSpecFragment verifies fragment component.
func TestFlowSpecFragment(t *testing.T) {
	comp := NewFlowFragmentComponent(FlowFragDontFragment)

	assert.Equal(t, FlowFragment, comp.Type())
}

// TestFlowSpecIPv4Basic verifies basic IPv4 FlowSpec NLRI.
func TestFlowSpecIPv4Basic(t *testing.T) {
	fs := NewFlowSpec(IPv4FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
	fs.AddComponent(NewFlowIPProtocolComponent(6)) // TCP

	assert.Equal(t, IPv4FlowSpec, fs.Family())
	assert.Len(t, fs.Components(), 2)
}

// TestFlowSpecIPv6Basic verifies basic IPv6 FlowSpec NLRI.
func TestFlowSpecIPv6Basic(t *testing.T) {
	fs := NewFlowSpec(IPv6FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("2001:db8::/32")))

	assert.Equal(t, IPv6FlowSpec, fs.Family())
}

// TestFlowSpecBytes verifies wire format encoding.
func TestFlowSpecBytes(t *testing.T) {
	fs := NewFlowSpec(IPv4FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/8")))

	data := fs.Bytes()
	require.NotEmpty(t, data)

	// First byte is NLRI length, then components
	// Component format: type (1 byte) + prefix encoding
}

// TestFlowSpecString verifies string representation.
func TestFlowSpecString(t *testing.T) {
	fs := NewFlowSpec(IPv4FlowSpec)
	fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
	fs.AddComponent(NewFlowDestPortComponent(80))

	s := fs.String()
	assert.Contains(t, s, "10.0.0.0/24")
	assert.Contains(t, s, "80")
}

// TestFlowSpecComplexRule verifies complex FlowSpec rule.
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
