package rib

import (
	"bytes"
	"net/netip"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
)

// TestVPNWithdrawPromotesTheOtherPE proves that a VPN withdrawal re-elects the
// route it names whatever its Compatibility field holds.
//
// VALIDATES: two PEs announce one RD and prefix under different labels; PE A
// withdraws it with the Compatibility field set to 0x800000 (the value RFC 8277
// Section 2.4 says a sender SHOULD use) and to 0x000000. PE B's path is still
// stored, and PE A held the best on its lower MED, so the RIB keeps exactly one
// best for the route and the change it publishes promotes PE B.
// PREVENTS: the election gathering the withdrawal with the announcement
// framing, while the stored best is keyed with the withdrawal framing: the
// gather then keys the Compatibility field into the Route Distinguisher, finds
// no candidate, and withdraws the route PE B still carries.
//
// RFC requirement: RFC8277-2.4-1 positive -- a VPN withdrawal from PE A whose Compatibility field is 0x800000 or 0x000000 removes PE A's route, keeps PE B's, and the best change it publishes promotes PE B rather than withdrawing the route.
// RFC requirement: RFC8277-2.4-1 negative -- valid nonrecommended Compatibility cannot withdraw the other source's VPNv4/VPNv6 route; promotion publishes exactly Update and that source's native NLRI and negotiated Path Identifier.
func TestVPNWithdrawPromotesTheOtherPE(t *testing.T) {
	compatibilityVPNPromotion(t)
	for _, tc := range []struct {
		name          string
		compatibility [3]byte
	}{
		{"compatibility-0x800000", [3]byte{0x80, 0x00, 0x00}},
		{"compatibility-0x000000", [3]byte{0x00, 0x00, 0x00}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bus := newTestEventBus()
			r := newTestRIBManagerWithBus(bus)
			peA := netip.MustParseAddr("192.0.2.1")
			peB := netip.MustParseAddr("192.0.2.2")
			r.peerMeta[peA] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
			r.peerMeta[peB] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
			ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			if err != nil {
				t.Fatalf("register context: %v", err)
			}

			nlriA := vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)
			nlriB := vpnv4NLRI(101, vpnRouteKeyRD, 0x0a)
			feedReceived(r, peA, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 1}, 100, nlriA))
			feedReceived(r, peB, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 2}, 200, nlriB))
			before := len(vpnBestChanges(bus, vpnv4Family))

			withdrawn := vpnv4NLRI(0, vpnRouteKeyRD, 0x0a)
			withdrawn[1], withdrawn[2], withdrawn[3] = tc.compatibility[0], tc.compatibility[1], tc.compatibility[2]
			feedReceived(r, peA, ctxID, vpnv4WithdrawBody(withdrawn))

			if got := r.bgpPeers[peA].FamilyLen(vpnv4Family); got != 0 {
				t.Fatalf("PE A stores %d routes after its withdrawal, want 0", got)
			}
			if got := r.bgpPeers[peB].FamilyLen(vpnv4Family); got != 1 {
				t.Fatalf("PE B stores %d routes, want 1", got)
			}
			if n := bestRecordCount(r, vpnv4Family); n != 1 {
				t.Fatalf("best records after PE A's withdrawal = %d, want 1: PE B still carries the route", n)
			}
			changes := vpnBestChanges(bus, vpnv4Family)
			if len(changes) != before+1 {
				t.Fatal("PE A's withdrawal published no best change, yet PE A held the best on its lower MED")
			}
			last := changes[len(changes)-1]
			if last.Action != routeaction.Update {
				t.Fatalf("replacement action = %v, want %v", last.Action, routeaction.Update)
			}
			if !bytes.Equal(last.NLRI, nlriB) {
				t.Fatalf("replacement NLRI = %x, want PE B's %x", last.NLRI, nlriB)
			}
			if want := netip.MustParseAddr("10.0.0.2"); last.NextHop != want {
				t.Fatalf("best change after the withdrawal names next hop %s, want PE B's %s", last.NextHop, want)
			}
		})
	}
}
