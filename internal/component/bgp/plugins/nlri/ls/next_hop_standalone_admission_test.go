// Design: docs/architecture/wire/nlri-bgpls.md -- LS per-address framing and IPv6 roles
// Related: docs/architecture/update-building.md -- checked plugin builder admission
// Related: next_hop_wire_test.go -- independent literal LS Node NLRI oracles
package ls_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// The common field validator may defer capability77 permission for plain16,
// but its exception must not admit an RD-framed standalone link-local. Useful
// native/global/pair forms and the separately owned VPN128 profile are controls.
func TestLSStandaloneLinkLocalFieldAdmission(t *testing.T) {
	global := netip.MustParseAddr("2001:db8::9")
	linkLocal := netip.MustParseAddr("fe80::9")
	ipv4 := netip.MustParseAddr("192.0.2.9")
	for _, tc := range []struct {
		name          string
		family        family.Family
		first, second netip.Addr
		octets        int
		refused       bool
	}{
		{"ls72-framed-link-local", ls.BGPLSVPNFamily, linkLocal, netip.Addr{}, 24, true},
		{"ls71-plain-link-local", ls.BGPLSFamily, linkLocal, netip.Addr{}, 16, false},
		{"ls72-native-ipv4", ls.BGPLSVPNFamily, ipv4, netip.Addr{}, 12, false},
		{"ls71-native-ipv4", ls.BGPLSFamily, ipv4, netip.Addr{}, 4, false},
		{"ls72-global", ls.BGPLSVPNFamily, global, netip.Addr{}, 24, false},
		{"ls71-global", ls.BGPLSFamily, global, netip.Addr{}, 16, false},
		{"ls72-pair", ls.BGPLSVPNFamily, global, linkLocal, 48, false},
		{"ls71-pair", ls.BGPLSFamily, global, linkLocal, 32, false},
		{"ls72-unspecified-global-pair", ls.BGPLSVPNFamily, netip.IPv6Unspecified(), linkLocal, 48, true},
		{"ls72-mapped-single", ls.BGPLSVPNFamily, netip.MustParseAddr("::ffff:192.0.2.9"), netip.Addr{}, 24, true},
		{"vpn128-separate-unspecified-pair", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}, netip.IPv6Unspecified(), linkLocal, 48, false},
		{"unknown-profile-unchanged", family.Family{AFI: family.AFIIPv6, SAFI: 250}, linkLocal, netip.Addr{}, 16, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := attribute.MPNextHopProfile(attribute.AFI(tc.family.AFI), attribute.SAFI(tc.family.SAFI))
			err := message.ValidateMPNextHop(profile, tc.octets, tc.first, tc.second)
			familyErr := message.ValidateFamilyNextHop(tc.family, tc.first, tc.second)
			if tc.refused {
				require.ErrorIs(t, err, message.ErrUnicastNextHopUnusable)
				require.ErrorIs(t, familyErr, message.ErrUnicastNextHopUnusable)
				return
			}
			require.NoError(t, err)
			require.NoError(t, familyErr)
		})
	}
}

// A checked plugin producer must refuse standalone LL24 before constructing
// an UPDATE, while plain LL16 remains available for a caller that establishes
// capability77 permission later. Exact useful global singles prevent blanket refusal.
func TestLSStandaloneLinkLocalPluginBuilder(t *testing.T) {
	for _, base := range lsNextHopWireCases() {
		if len(base.nextHops) != 1 {
			continue
		}
		if !base.nextHops[0].Is6() {
			continue
		}
		for _, standalone := range []bool{false, true} {
			name := base.name + "/global"
			if standalone {
				name = base.name + "/standalone-link-local"
			}
			t.Run(name, func(t *testing.T) {
				tc := base
				if standalone {
					tc.nextHops = []netip.Addr{netip.MustParseAddr("fe80::9")}
					tc.wantNextHop = []byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9}
					if tc.family == ls.BGPLSVPNFamily {
						tc.wantNextHop = append([]byte{0, 0, 0, 0, 0, 0, 0, 0}, tc.wantNextHop...)
					}
				}
				builder := message.GetUpdateBuilder(65001, false, true, false)
				defer message.PutUpdateBuilder(builder)
				update := builder.BuildPlugin(message.PluginParams{
					AFI: uint16(tc.family.AFI), SAFI: byte(tc.family.SAFI),
					NLRI: tc.nlri, NextHop: tc.nextHops[0],
				})
				if standalone {
					if tc.family == ls.BGPLSVPNFamily {
						require.Nil(t, update, "capability77 cannot license an RD-framed standalone field")
						return
					}
				}
				require.NotNil(t, update)
				_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
				require.True(t, found)
				checkLSNextHopWire(t, value, tc)
			})
		}
	}
}
