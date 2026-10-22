// Design: plugin_encode_text.go — FlowSpec text components encoded to NLRI wire
// RFC: rfc/short/rfc8956.md — the IPv6 prefix offset (Section 3.1)

package flowspec

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC8956TextPrefixOffset drives the text encoder with the
// `<address>/<length>/<offset>` form ExaBGP writes and FormatPrefixWithOffset
// renders.
//
// VALIDATES: RFC 8956 Section 3.1, "The encoded pattern contains enough octets
// for the bits used in matching (length minus offset bits)": `::1/128/120`
// encodes as type 2, length 128, offset 120 and the one pattern octet, the
// bytes ExaBGP 5.0 sends; an offset of 0 is the plain prefix; an offset not
// below the length, or on an IPv4 prefix, is refused.
// PREVENTS: `update text` refusing an IPv6 FlowSpec prefix with an offset,
// which the config path already accepted.
func TestRFC8956TextPrefixOffset(t *testing.T) {
	wire, err := EncodeFlowSpecComponents(IPv6FlowSpec, []string{"source-ipv6", "::1/128/120"})
	require.NoError(t, err)
	require.Contains(t, hex.EncodeToString(wire), "02807801")

	plain, err := EncodeFlowSpecComponents(IPv6FlowSpec, []string{"destination-ipv6", "2001:db8::/32"})
	require.NoError(t, err)
	zero, err := EncodeFlowSpecComponents(IPv6FlowSpec, []string{"destination-ipv6", "2001:db8::/32/0"})
	require.NoError(t, err)
	require.Equal(t, plain, zero)

	for _, args := range [][]string{
		{"source-ipv6", "::1/128/128"},
		{"source-ipv6", "::1/64/200"},
		{"source-ipv6", "::1/128/x"},
		{"destination-ipv4", "10.0.0.0/24/8"},
	} {
		_, err := EncodeFlowSpecComponents(IPv6FlowSpec, args)
		require.Error(t, err, "%v", args)
	}
}
