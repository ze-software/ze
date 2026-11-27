// Design: docs/architecture/api/json-format.md -- family-aware next-hop display.

package cli

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDecodeMPReachNextHopProfile reaches parseNextHop through its MP_REACH
// caller, so passing only the AFI cannot satisfy LS71 and LS72 together.
// Literal nonzero RDs must be skipped without modifying the stored field.
func TestDecodeMPReachNextHopProfile(t *testing.T) {
	const global = "20010db8000000000000000000000001"
	const local = "fe800000000000000000000000000002"
	const rd = "0000fde900000007"
	for _, tc := range []struct {
		name string
		afi  uint16
		safi byte
		nh   string
		want string
	}{
		{"ls71-ipv4", 16388, 71, "c0000201", "192.0.2.1"},
		{"ls71-ipv6", 16388, 71, global, "2001:db8::1"},
		{"ls71-pair", 16388, 71, global + local, "2001:db8::1"},
		{"ls72-ipv4", 16388, 72, rd + "c0000201", "192.0.2.1"},
		{"ls72-ipv6", 16388, 72, rd + global, "2001:db8::1"},
		{"ls72-pair", 16388, 72, rd + global + "0000fde900000008" + local, "2001:db8::1"},
		{"ls72-plain-ipv4", 16388, 72, "c0000201", "c0000201"},
		{"ls72-plain-ipv6", 16388, 72, global, global},
		{"ls72-plain-pair", 16388, 72, global + local, global + local},
		{"ls72-forty", 16388, 72, rd + global + local, rd + global + local},
		{"vpn4-ipv4", 1, 128, rd + "c0000201", "192.0.2.1"},
		{"vpn6-ipv6", 2, 128, rd + global, "2001:db8::1"},
		{"vpn6-pair", 2, 128, rd + global + rd + local, "2001:db8::1"},
		{"vpn6-legacy", 2, 128, global + local, "2001:db8::1"},
		{"unknown-ipv4", 65530, 250, "c0000201", "192.0.2.1"},
		{"unknown-ipv6", 65530, 250, global, "2001:db8::1"},
		{"unknown-malformed", 1, 250, "c0000201c6336401", "c0000201c6336401"},
		{"unknown-rd", 1, 250, rd + "c0000201", rd + "c0000201"},
		{"empty", 1, 1, "", "no-nexthop"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nh, err := hex.DecodeString(tc.nh)
			require.NoError(t, err)
			// A claimed family retains this undecoded NLRI as an opaque element;
			// an unclaimed family reads it as a prefix. Either keeps the operation,
			// isolating NH display while passing the actual family to the caller.
			nlri := []byte{24, 10, 0, 0}
			raw := []byte{byte(tc.afi >> 8), byte(tc.afi), tc.safi, byte(len(nh))}
			raw = append(raw, nh...)
			raw = append(raw, 0)
			raw = append(raw, nlri...)
			before := bytes.Clone(raw)
			_, operations, err := buildMPReachZe(nil, raw)
			require.NoError(t, err)
			require.Len(t, operations, 1)
			require.Equal(t, tc.want, operations[0]["next-hop"])
			require.Equal(t, "add", operations[0][jsonKeyAction])
			require.NotEmpty(t, operations[0][jsonKeyNLRI], "NH errors must retain the operation")
			require.Equal(t, before, raw, "display must not rewrite stored bytes")
		})
	}
}

// TestDecodeMPReachUnknownIPv6ProfileNativeIPv4 preserves generic native IPv4
// display and the raw diagnostic form for known IPv6-only profiles.
func TestDecodeMPReachUnknownIPv6ProfileNativeIPv4(t *testing.T) {
	for _, tc := range []struct {
		name string
		safi byte
		want string
	}{
		{"unknown", 250, "192.0.2.1"},
		{"mup", 85, "192.0.2.1"},
		{"unicast", 1, "c0000201"},
		{"multicast", 2, "c0000201"},
		{"labeled", 4, "c0000201"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte{0, 2, tc.safi, 4, 0xc0, 0, 2, 1, 0, 24, 10, 0, 0}
			before := bytes.Clone(raw)
			_, operations, err := buildMPReachZe(nil, raw)
			require.NoError(t, err)
			require.Len(t, operations, 1)
			require.Equal(t, tc.want, operations[0]["next-hop"])
			require.Equal(t, "add", operations[0][jsonKeyAction])
			require.NotEmpty(t, operations[0][jsonKeyNLRI], "NH refusal must retain the operation")
			require.Equal(t, before, raw, "display changed the received field")
		})
	}
}
