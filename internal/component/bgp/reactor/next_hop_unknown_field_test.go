// Design: docs/architecture/bgp/structural-forwarding.md -- effective next-hop identity
// Related: forward_next_hop.go -- payloadNextHop preserves generic field interpretation
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// Unknown profiles keep the classifier's complete-field interpretation for both
// NLRI AFIs. Literal plain and RD-framed fields pin address offsets, mapped wire
// identity and both peer-own slots without claiming support for an unknown NLRI.
func TestPayloadNextHopUnknownFieldCompatibility(t *testing.T) {
	for _, fc := range []struct {
		name string
		fam  family.Family
	}{
		{name: "afi1", fam: family.Family{AFI: family.AFIIPv4, SAFI: 250}},
		{name: "afi2", fam: family.Family{AFI: family.AFIIPv6, SAFI: 250}},
	} {
		t.Run(fc.name, func(t *testing.T) {
			for _, tc := range []struct {
				name          string
				field         []byte
				first, second netip.Addr
				ipv6          bool
			}{
				{name: "plain4", field: []byte{192, 0, 2, 9}, first: netip.MustParseAddr("192.0.2.9")},
				{name: "rd12", field: []byte{1, 2, 3, 4, 5, 6, 7, 8, 192, 0, 2, 9}, first: netip.MustParseAddr("192.0.2.9")},
				{name: "plain16-mapped", field: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 192, 0, 2, 9}, first: netip.MustParseAddr("192.0.2.9"), ipv6: true},
				{name: "rd24-mapped", field: []byte{1, 2, 3, 4, 5, 6, 7, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 192, 0, 2, 9}, first: netip.MustParseAddr("192.0.2.9"), ipv6: true},
				{name: "plain32", field: []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9}, first: netip.MustParseAddr("2001:db8::9"), second: netip.MustParseAddr("fe80::9"), ipv6: true},
				{name: "rd48", field: []byte{1, 2, 3, 4, 5, 6, 7, 8, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9}, first: netip.MustParseAddr("2001:db8::9"), second: netip.MustParseAddr("fe80::9"), ipv6: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					body := lastSetNextHopBody(fc.fam, tc.field, false, netip.Addr{})
					original := bytes.Clone(body)
					got := payloadNextHop(body)
					require.Equal(t, fc.fam, got.mpFamily)
					require.Equal(t, uint8(len(tc.field)), got.mpWireLen)
					require.Equal(t, tc.first, got.mp)
					require.Equal(t, tc.second, got.mpLL)
					require.Equal(t, tc.ipv6, got.mpIPv6)
					require.Equal(t, nextHopClassicAbsent, got.classicPresence)
					require.False(t, got.legacy.IsValid())
					require.False(t, got.mpIgnored)
					require.True(t, got.has(tc.first))
					if tc.first.Is4() {
						require.True(t, got.has(netip.MustParseAddr("::ffff:192.0.2.9")))
					}
					if tc.second.IsValid() {
						require.True(t, got.has(tc.second.WithZone("recipient")))
					}
					require.False(t, got.has(netip.MustParseAddr("192.0.2.8")))
					require.Equal(t, original, body)
				})
			}
		})
	}
}
