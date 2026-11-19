// Design: docs/architecture/bgp/structural-forwarding.md -- complete final next-hop field admission
// Related: session_write.go -- public SendUpdate and final ordinary field checks
// Related: rfc4659_link_local_peering_test.go -- qualified absent-Global VPN pair
package reactor

import (
	"bufio"
	"bytes"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// A VPN profile does not use the ordinary IPv6-role validator. That separation
// must not allow a framed standalone LL24 when the plain16 capability classifier
// declines to classify it. Useful global24 and qualified absent-Global48 fields
// survive, including correction on the same session after every refusal.
func TestVPNStandaloneLinkLocalOrdinaryWriter(t *testing.T) {
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		for _, enabled := range []bool{false, true} {
			for _, kind := range []string{"standalone-link-local", "global", "unspecified-pair"} {
				if afi == family.AFIIPv4 {
					if kind == "unspecified-pair" {
						continue
					}
				}
				name := "vpnv4/cap77-off/" + kind
				if afi == family.AFIIPv6 {
					name = "vpnv6/cap77-off/" + kind
				}
				if enabled {
					name = "vpnv4/cap77-on/" + kind
					if afi == family.AFIIPv6 {
						name = "vpnv6/cap77-on/" + kind
					}
				}
				t.Run(name, func(t *testing.T) {
					peer, conn := rawOriginatedNextHopPeer(t)
					if afi == family.AFIIPv6 {
						peer.settings.Address = netip.MustParseAddr("fe80::2%if0")
						peer.settings.LocalAddress = netip.MustParseAddr("fe80::1%if0")
						local := peer.settings.LocalAddress.WithZone("")
						peer.session.transport.Store(&sessionTransport{local: local, localString: local.String()})
						scoped := &ls71PairScopedConn{
							recordingConn: conn,
							local:         &net.TCPAddr{IP: local.AsSlice(), Port: 179, Zone: "if0"},
							remote:        &net.TCPAddr{IP: peer.settings.Address.AsSlice(), Port: 179, Zone: "if0"},
						}
						peer.session.mu.Lock()
						peer.session.conn = scoped
						peer.session.bufWriter = bufio.NewWriterSize(scoped, 4096)
						peer.session.mu.Unlock()
					}
					local := []capability.Capability{
						&capability.ASN4{ASN: peer.settings.LocalAS},
						&capability.Multiprotocol{AFI: afi, SAFI: family.SAFIVPN},
					}
					if afi == family.AFIIPv4 {
						local = append(local, &capability.ExtendedNextHop{Families: []capability.ExtendedNextHopFamily{{
							NLRIAFI: afi, NLRISAFI: family.SAFIVPN, NextHopAFI: capability.AFIIPv6,
						}}})
					}
					if enabled {
						local = append(local, &capability.LinkLocalNextHop{})
					}
					remote := append([]capability.Capability(nil), local...)
					remote[0] = &capability.ASN4{ASN: peer.settings.PeerAS}
					negotiated := capability.Negotiate(local, remote, capability.PeerIdentity{
						LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
					})
					peer.session.negotiated = negotiated
					peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
					peer.setEncodingContexts(negotiated)
					if afi == family.AFIIPv6 {
						peer.refreshLinkScopeWith([]netip.Prefix{netip.MustParsePrefix("fe80::1/64")}, neighborView{})
						peer.fwdFacts.Store(peer.buildForwardFacts())
						require.True(t, destOnLink(peer))
						require.True(t, vpnIPv6LinkLocalPeering(peer.forwardFacts(), family.Family{AFI: afi, SAFI: family.SAFIVPN}))
					}
					body := vpnStandaloneOrdinaryBody(t, afi, kind)
					original := bytes.Clone(body)
					update, err := message.UnpackUpdate(body)
					require.NoError(t, err)
					err = peer.session.SendUpdate(update)
					require.Equal(t, original, body)
					if kind == "standalone-link-local" {
						require.ErrorIs(t, err, message.ErrUnicastNextHopUnusable)
						require.Empty(t, aigpSocketBodies(t, conn))
						require.False(t, peer.session.writePending)
						require.Zero(t, peer.session.updateSendTotals.updates)
						body = vpnStandaloneOrdinaryBody(t, afi, "global")
						update, err = message.UnpackUpdate(body)
						require.NoError(t, err)
						require.NoError(t, peer.session.SendUpdate(update))
					} else {
						require.NoError(t, err)
					}
					require.Equal(t, [][]byte{body}, aigpSocketBodies(t, conn))
				})
			}
		}
	}
}

// The common framing boundary must recognize native IPv6 Link-Local rather
// than every address whose unmapped interpretation is link-local. Native and
// mapped IPv4 link-local next hops retain their VPN profile's valid forms.
func TestVPNStandaloneLinkLocalFieldAdmission(t *testing.T) {
	for _, tc := range []struct {
		name          string
		afi           family.AFI
		first, second netip.Addr
		octets        int
		refused       bool
	}{
		{"vpnv4-real-ipv6-link-local", family.AFIIPv4, netip.MustParseAddr("fe80::9"), netip.Addr{}, 24, true},
		{"vpnv6-real-ipv6-link-local", family.AFIIPv6, netip.MustParseAddr("fe80::9"), netip.Addr{}, 24, true},
		{"vpnv4-native-ipv4-link-local", family.AFIIPv4, netip.MustParseAddr("169.254.0.9"), netip.Addr{}, 12, false},
		{"vpnv6-mapped-ipv4-link-local", family.AFIIPv6, netip.MustParseAddr("::ffff:169.254.0.9"), netip.Addr{}, 24, false},
		{"vpnv6-unspecified-global-pair", family.AFIIPv6, netip.IPv6Unspecified(), netip.MustParseAddr("fe80::9"), 48, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fam := family.Family{AFI: tc.afi, SAFI: family.SAFIVPN}
			profile := attribute.MPNextHopProfile(attribute.AFI(tc.afi), attribute.SAFIVPN)
			err := message.ValidateMPNextHop(profile, tc.octets, tc.first, tc.second)
			familyErr := message.ValidateFamilyNextHop(fam, tc.first, tc.second)
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

// Complete independent RFC4364/RFC4659 NLRI literals have label16 with BOS,
// RD65000:7, and 198.51.100.0/24 or 2001:db8:100::/48. Next-hop RDs are zero.
func vpnStandaloneOrdinaryBody(t *testing.T, afi family.AFI, kind string) []byte {
	t.Helper()
	nlri := mustHex(t, "700001010000fde800000007c63364")
	if afi == family.AFIIPv6 {
		nlri = mustHex(t, "880001010000fde80000000720010db80100")
	}
	first := netip.MustParseAddr("2001:db8:1::9")
	var second netip.Addr
	switch kind {
	case "standalone-link-local":
		first = netip.MustParseAddr("fe80::9")
	case "unspecified-pair":
		first = netip.IPv6Unspecified()
		second = netip.MustParseAddr("fe80::9")
	case "global":
	default:
		t.Fatalf("unknown VPN next-hop form %q", kind)
	}
	field := append([]byte{0, 0, 0, 0, 0, 0, 0, 0}, first.AsSlice()...)
	if second.IsValid() {
		field = append(field, 0, 0, 0, 0, 0, 0, 0, 0)
		field = append(field, second.AsSlice()...)
	}
	value := append([]byte{0, byte(afi), 128, byte(len(field))}, field...)
	value = append(value, 0)
	value = append(value, nlri...)
	attrs := append(mixedAttr(14, value), 0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe8)
	return makeUpdateBody(nil, attrs, nil)
}
