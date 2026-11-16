// Design: docs/architecture/wire/nlri.md -- family-aware MP next-hop readers.

package wireu_test

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// TestMPReachWireNextHopProfile reads independent literal fields through the
// lazy wire view. Framing follows the actual family; borrowed fields and NLRI
// retain their original bytes, including nonzero received RDs.
func TestMPReachWireNextHopProfile(t *testing.T) {
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
		{"ls72-plain-ipv4", 16388, 72, "c0000201", ""},
		{"ls72-plain-ipv6", 16388, 72, global, ""},
		{"ls72-plain-pair", 16388, 72, global + local, ""},
		{"ls72-forty", 16388, 72, rd + global + local, ""},
		{"ls71-rd-ipv4", 16388, 71, rd + "c0000201", ""},
		{"ls71-rd-ipv6", 16388, 71, rd + global, ""},
		{"ls71-rd-pair", 16388, 71, rd + global + rd + local, ""},
		{"vpn4-ipv4", 1, 128, rd + "c0000201", "192.0.2.1"},
		{"vpn6-ipv6", 2, 128, rd + global, "2001:db8::1"},
		{"vpn6-pair", 2, 128, rd + global + rd + local, "2001:db8::1"},
		{"vpn6-legacy", 2, 128, global + local, "2001:db8::1"},
		{"extended-ipv4", 1, 1, global + local, "2001:db8::1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nh, err := hex.DecodeString(tc.nh)
			require.NoError(t, err)
			// This opaque NLRI deliberately has its own nonzero RD-like bytes;
			// address reading must neither decode nor alter it.
			nlri := []byte{0, 1, 0, 8, 0, 0, 0xfd, 0xe9, 0, 0, 0, 9}
			raw := []byte{byte(tc.afi >> 8), byte(tc.afi), tc.safi, byte(len(nh))}
			raw = append(raw, nh...)
			raw = append(raw, 0)
			raw = append(raw, nlri...)
			before := bytes.Clone(raw)
			view, err := wireu.NewMPReachWire(raw)
			require.NoError(t, err)
			got := view.NextHop()
			if tc.want == "" {
				require.False(t, got.IsValid(), "malformed family field decoded as %s", got)
			} else {
				require.Equal(t, netip.MustParseAddr(tc.want), got)
			}
			require.Equal(t, nh, view.NextHopBytes())
			require.Equal(t, nlri, view.NLRIBytes())
			require.Equal(t, &raw[4], &view.NextHopBytes()[0], "next-hop view must stay borrowed")
			require.Equal(t, &raw[5+len(nh)], &view.NLRIBytes()[0], "NLRI view must stay borrowed")
			require.Equal(t, before, raw, "reading must not normalize received bytes")
		})
	}
}

// TestMPReachWireUnknownIPv6ProfileNativeIPv4 keeps generic four-octet
// addresses readable while known IPv6-only profiles still refuse them.
func TestMPReachWireUnknownIPv6ProfileNativeIPv4(t *testing.T) {
	for _, tc := range []struct {
		name string
		safi byte
		want bool
	}{
		{"unknown", 250, true},
		{"mup", 85, true},
		{"unicast", 1, false},
		{"multicast", 2, false},
		{"labeled", 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte{0, 2, tc.safi, 4, 0xc0, 0, 2, 1, 0, 24, 10, 0, 0}
			before := bytes.Clone(raw)
			view, err := wireu.NewMPReachWire(raw)
			require.NoError(t, err)
			got := view.NextHop()
			if tc.want {
				require.Equal(t, netip.MustParseAddr("192.0.2.1"), got)
			} else {
				require.False(t, got.IsValid(), "known IPv6-only profile accepted native IPv4")
			}
			require.Equal(t, []byte{0xc0, 0, 2, 1}, view.NextHopBytes())
			require.Equal(t, []byte{24, 10, 0, 0}, view.NLRIBytes())
			require.Equal(t, &raw[4], &view.NextHopBytes()[0])
			require.Equal(t, &raw[9], &view.NLRIBytes()[0])
			require.Equal(t, before, raw, "reading changed the received field")
		})
	}
}
