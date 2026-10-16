package flowspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// RFC requirement: RFC8956-3.1-3 positive -- valid nonzero offsets encode only the requested pattern (§3.1).
// RFC requirement: RFC8956-3.1-3 negative -- invalid bounds and truncated patterns are refused (§3.1).
func TestIPv6FlowPrefixRejectsMalformedBounds(t *testing.T) {
	for _, wire := range [][]byte{
		{3, 1, 0, 1}, {3, 1, 64, 64}, {3, 1, 63, 64}, {3, 1, 129, 0},
		{4, 1, 80, 64, 0x12}, // A two-octet pattern truncated to one.
	} {
		_, err := flowspec.ParseFlowSpec(IPv6FlowSpec, wire)
		require.Error(t, err, "wire %x", wire)
	}
	for _, input := range []string{"::/64/64", "::/64/65", "::/0/1", "::/129", "::/64/nonsense", "::/64/-1"} {
		_, dropped := buildFlowSpecComponents(map[string][]string{flowspec.KwDestinationIPv6: {input}}, true)
		assert.Equal(t, []string{flowspec.KwDestinationIPv6}, dropped, "invalid configured prefix must refuse, not lose the offset")
	}
	fs, dropped := buildFlowSpecComponents(map[string][]string{flowspec.KwDestinationIPv6: {"2001:db8:abcd:1:1234::/80/64"}}, true)
	require.Empty(t, dropped)
	assert.Equal(t, []byte{5, 1, 80, 64, 0x12, 0x34}, fs.Bytes())
	received, err := flowspec.ParseFlowSpec(IPv6FlowSpec, []byte{5, 1, 80, 64, 0x12, 0x34})
	require.NoError(t, err)
	assert.Zero(t, flowspec.Compare(fs, received), "skipped configured address bits cannot change precedence")
}
