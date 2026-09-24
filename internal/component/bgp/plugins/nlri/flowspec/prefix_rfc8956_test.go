package flowspec

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC8956-3.1-1 positive -- native prefix serialization clears sub-byte padding (§3.1).
// RFC requirement: RFC8956-3.1-1 negative -- unmasked configured address bits cannot leak into the encoded pattern (§3.1).
// RFC requirement: RFC8956-3.1-2 positive -- byte-aligned and unaligned patterns reconstruct their address bits (§3.1).
// RFC requirement: RFC8956-3.1-2 negative -- received padding bits cannot change the match or retransmitted pattern (§3.1).
func TestIPv6FlowPrefixPatternAndPadding(t *testing.T) {
	for _, tc := range []struct {
		wire, canonical []byte
		prefix          string
		offset          uint8
	}{
		{[]byte{4, 1, 72, 65, 0xff}, []byte{4, 1, 72, 65, 0xfe}, "::7f00:0:0:0/72", 65},
		{[]byte{5, 2, 17, 7, 0x81, 0xff}, []byte{5, 2, 17, 7, 0x81, 0xc0}, "103:8000::/17", 7},
		{[]byte{5, 1, 80, 64, 0x12, 0x34}, []byte{5, 1, 80, 64, 0x12, 0x34}, "::1234:0:0:0/80", 64},
		{[]byte{3, 1, 0, 0}, []byte{3, 1, 0, 0}, "::/0", 0},
	} {
		fs, err := ParseFlowSpec(IPv6FlowSpec, tc.wire)
		require.NoError(t, err)
		require.Len(t, fs.Components(), 1)
		comp := fs.Components()[0].(*prefixComponent)
		assert.Equal(t, netip.MustParsePrefix(tc.prefix), comp.Prefix())
		assert.Equal(t, tc.offset, comp.Offset())
		assert.Equal(t, tc.canonical, fs.Bytes(), "ignored padding must not change the next encoded pattern")
		buf := make([]byte, fs.Len()+2)
		n := fs.WriteTo(buf, 2)
		assert.Equal(t, tc.canonical, buf[2:2+n])
	}
	// Outbound callers may supply host bits: both native writers must zero them.
	fs := NewFlowSpec(IPv6FlowSpec)
	require.NoError(t, fs.AddComponent(NewFlowDestPrefixComponent(netip.MustParsePrefix("2001:db8:ffff::/33"))))
	assert.Equal(t, []byte{8, 1, 33, 0, 0x20, 1, 0x0d, 0xb8, 0x80}, fs.Bytes())
}

// RFC requirement: RFC8956-3.1-3 positive -- valid nonzero offsets encode only the requested pattern (§3.1).
// RFC requirement: RFC8956-3.1-3 negative -- invalid bounds and truncated patterns are refused (§3.1).
func TestIPv6FlowPrefixRejectsMalformedBounds(t *testing.T) {
	for _, wire := range [][]byte{
		{3, 1, 0, 1}, {3, 1, 64, 64}, {3, 1, 63, 64}, {3, 1, 129, 0},
		{4, 1, 80, 64, 0x12}, // A two-octet pattern truncated to one.
	} {
		_, err := ParseFlowSpec(IPv6FlowSpec, wire)
		require.Error(t, err, "wire %x", wire)
	}
	for _, input := range []string{"::/64/64", "::/64/65", "::/0/1", "::/129", "::/64/nonsense", "::/64/-1"} {
		_, dropped := buildFlowSpecComponents(map[string][]string{kwDestinationIPv6: {input}}, true)
		assert.Equal(t, []string{kwDestinationIPv6}, dropped, "invalid configured prefix must refuse, not lose the offset")
	}
	fs, dropped := buildFlowSpecComponents(map[string][]string{kwDestinationIPv6: {"2001:db8:abcd:1:1234::/80/64"}}, true)
	require.Empty(t, dropped)
	assert.Equal(t, []byte{5, 1, 80, 64, 0x12, 0x34}, fs.Bytes())
	received, err := ParseFlowSpec(IPv6FlowSpec, []byte{5, 1, 80, 64, 0x12, 0x34})
	require.NoError(t, err)
	assert.Zero(t, Compare(fs, received), "skipped configured address bits cannot change precedence")
}
