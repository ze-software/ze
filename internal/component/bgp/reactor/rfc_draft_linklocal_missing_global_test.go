package reactor

import (
	"bytes"
	"fmt"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// The source is external, so the RR-only same-segment refusal cannot hide the
// ordinary forwarding path. Both destinations negotiated capability 77.
// Preserved red carrier for spec-bgp-update-propagation-rfc-defects D6.
// Draft Section 4, paragraphs 255-258, 293-295 and 397-398 require withdrawal
// rather than advertising a route left without a permitted IPv6 next hop.
// Both multihop cases currently leak fe80::1; the global controls pass.
func TestDraftLinkLocalOnlyRouteCannotCrossMultihopEgress(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprintf("external=%v", external), func(t *testing.T) {
			peer := llnhClient(t, llnhOffSegmentAddr, llnhSegment, false)
			peer.settings.NextHopMode = NextHopUnchanged
			if external {
				peer.settings.PeerAS = 65002
			}
			peer.negotiated.Load().LinkLocalNextHop = true
			peer.refreshForwardFacts()
			source := forwardSourceInfo{resolved: true, isIBGP: false, globalLocalAS: 65000}
			global := llnhForward(t, llnhExternalPayload(t, "2001:db8:1::1"), source, peer)
			if !bytes.Equal(global[peer.settings.Address], netip.MustParseAddr("2001:db8:1::1").AsSlice()) {
				t.Fatalf("external=%v: global-next-hop control did not reach destination", external)
			}
			linkLocal := llnhForward(t, llnhExternalPayload(t, "fe80::1"), source, peer)
			if field := linkLocal[peer.settings.Address]; len(field) != 0 {
				t.Fatalf("external=%v: multihop destination received next hop %x with no global address", external, field)
			}
		})
	}
}

func llnhExternalPayload(t *testing.T, nextHop string) []byte {
	t.Helper()
	update, err := message.UnpackUpdate(llnhReflectedPayload(nextHop))
	if err != nil {
		t.Fatal(err)
	}
	_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	if !found {
		t.Fatal("fixture has no MP_REACH")
	}
	// llnhForward registers a four-octet-AS context; unlike the RR fixture,
	// an external destination actually parses and prepends this AS_PATH.
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9}
	attrs = append(attrs, 0x80, 14, byte(len(mp)))
	attrs = append(attrs, mp...)
	return buildUpdatePayload(attrs, nil)
}
