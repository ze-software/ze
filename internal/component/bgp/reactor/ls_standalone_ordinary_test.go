// Design: docs/architecture/bgp/structural-forwarding.md -- final emitted next-hop admission
// Related: session_write.go -- public SendUpdate and final ordinary writer gate
// Related: docs/architecture/wire/nlri-bgpls.md -- LS71 plain16 versus LS72 framed24
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// Public SendUpdate must judge the complete emitted field: capability77 permits
// plain LS71 LL16, never RD-framed LS72 LL24. Every refusal observes zero queued
// or flushed output and then a useful global correction on that same session.
func TestLSStandaloneLinkLocalOrdinaryWriter(t *testing.T) {
	for _, fam := range []family.Family{ls.BGPLSFamily, ls.BGPLSVPNFamily} {
		for _, enabled := range []bool{false, true} {
			for _, kind := range []string{"standalone-link-local", "global", "native-ipv4", "global-pair"} {
				name := "ls71/cap77-off/" + kind
				if fam == ls.BGPLSVPNFamily {
					name = "ls72/cap77-off/" + kind
				}
				if enabled {
					name = "ls71/cap77-on/" + kind
					if fam == ls.BGPLSVPNFamily {
						name = "ls72/cap77-on/" + kind
					}
				}
				t.Run(name, func(t *testing.T) {
					peer, conn := rawOriginatedNextHopPeer(t)
					caps := []capability.Capability{
						&capability.ASN4{ASN: peer.settings.LocalAS},
						&capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI},
					}
					if enabled {
						caps = append(caps, &capability.LinkLocalNextHop{})
					}
					negotiated := capability.Negotiate(caps, caps, capability.PeerIdentity{
						LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
					})
					peer.session.negotiated = negotiated
					peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
					peer.setEncodingContexts(negotiated)
					body := lsStandaloneOrdinaryBody(t, fam, kind)
					original := bytes.Clone(body)
					update, err := message.UnpackUpdate(body)
					require.NoError(t, err)
					err = peer.session.SendUpdate(update)
					require.Equal(t, original, body, "the caller's supplied bytes remain unchanged")
					refused := false
					if kind == "standalone-link-local" {
						refused = fam == ls.BGPLSVPNFamily || !enabled
					}
					if refused {
						require.ErrorIs(t, err, message.ErrUnicastNextHopUnusable)
						require.Empty(t, aigpSocketBodies(t, conn), "no forbidden announcement or invented withdrawal")
						require.False(t, peer.session.writePending, "no forbidden bytes remain queued")
						require.Zero(t, peer.session.updateSendTotals.updates)
						require.Zero(t, peer.session.updateSendTotals.announcedRoutes)
						require.Zero(t, peer.session.updateSendTotals.withdrawnRoutes)
						body = lsStandaloneOrdinaryBody(t, fam, "global")
						update, err = message.UnpackUpdate(body)
						require.NoError(t, err)
						require.NoError(t, peer.session.SendUpdate(update), "a corrected route remains usable on this session")
					} else {
						require.NoError(t, err)
					}
					require.Equal(t, [][]byte{body}, aigpSocketBodies(t, conn), "exact useful field, zero RDs, reserved octet and NLRI")
				})
			}
		}
	}
}

// The body is MP-first and ASN4-shaped, with a complete literal Node NLRI.
// Each RD is inserted independently of the production next-hop encoder.
func lsStandaloneOrdinaryBody(t *testing.T, fam family.Family, kind string) []byte {
	t.Helper()
	primary := netip.MustParseAddr("2001:db8:1::9").AsSlice()
	var second []byte
	switch kind {
	case "standalone-link-local":
		primary = netip.MustParseAddr("fe80::9").AsSlice()
	case "native-ipv4":
		primary = []byte{192, 0, 2, 9}
	case "global-pair":
		second = netip.MustParseAddr("fe80::9").AsSlice()
	case "global":
	default:
		t.Fatalf("unknown next-hop test form %q", kind)
	}
	field := append(bytes.Clone(primary), second...)
	nlri := ls71PairNLRI(t)
	if fam == ls.BGPLSVPNFamily {
		field = ls72Framed(primary, second)
		nlri = ls72NLRI(t)
	}
	value := append([]byte{0x40, 0x04, byte(fam.SAFI), byte(len(field))}, field...)
	value = append(value, 0)
	value = append(value, nlri...)
	attrs := append(mixedAttr(byte(attribute.AttrMPReachNLRI), value),
		0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe8)
	return makeUpdateBody(nil, attrs, nil)
}
