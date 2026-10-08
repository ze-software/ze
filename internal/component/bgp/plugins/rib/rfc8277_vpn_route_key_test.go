package rib

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

// vpnRouteKeyRD is the Route Distinguisher 0:1 every route in this file
// carries, so only the label differs between the routes a test compares.
var vpnRouteKeyRD = [8]byte{0, 0, 0, 0, 0, 0, 0, 1}

// TestVPNRouteKeyTwoPEsDifferentLabelsMeetInOneElection drives two PEs that
// announce RD 0:1 10.0.0.0/8 under labels 100 and 101 through the real ingest
// path.
//
// VALIDATES: the label is no part of the route key, so both announcements are
// candidates of ONE election and the lower MED wins; the published best names
// the winner's own NLRI, label included.
// PREVENTS: the label stack inside the opaque key, which made the two
// announcements two routes that never met, each published as its own best.
func TestVPNRouteKeyTwoPEsDifferentLabelsMeetInOneElection(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	peA := netip.MustParseAddr("192.0.2.1")
	peB := netip.MustParseAddr("192.0.2.2")
	r.peerMeta[peA] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	r.peerMeta[peB] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	nlriA := vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)
	nlriB := vpnv4NLRI(101, vpnRouteKeyRD, 0x0a)
	feedReceived(r, peA, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 1}, 200, nlriA))
	feedReceived(r, peB, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 2}, 100, nlriB))

	if got := len(gatherCandidatesHeld(t, r, vpnv4Family, nlriA, false)); got != 2 {
		t.Fatalf("candidates seen from PE A's NLRI = %d, want 2: one route under two labels", got)
	}
	if got := len(gatherCandidatesHeld(t, r, vpnv4Family, nlriB, false)); got != 2 {
		t.Fatalf("candidates seen from PE B's NLRI = %d, want 2: one route under two labels", got)
	}
	if got := bestRecordCount(r, vpnv4Family); got != 1 {
		t.Fatalf("best records = %d, want 1 for one RD and prefix", got)
	}
	changes := vpnBestChanges(bus, vpnv4Family)
	if len(changes) != 2 {
		t.Fatalf("published %d best-changes, want 2 (A's add, then B's win)", len(changes))
	}
	last := changes[1]
	if last.Action != routeaction.Update {
		t.Fatalf("second change action = %v, want update: B's win replaces the one route's best", last.Action)
	}
	if !bytes.Equal(last.NLRI, nlriB) {
		t.Fatalf("best NLRI = %x, want PE B's %x: the winner's own label", last.NLRI, nlriB)
	}
}

// TestRFC8277VPNRelabelReplacesTheRoute re-advertises one VPN route under a
// new label with unchanged attributes, without ADD-PATH.
//
// VALIDATES: the second UPDATE replaces the first: the Adj-RIB-In holds one
// route, it carries the new label, and the best-change publishes the new label.
// PREVENTS: a second route stored beside the first because the label was part
// of its key, and a relabel swallowed by the same-best short circuit.
//
// RFC requirement: RFC8277-2.5-1 positive -- without ADD-PATH, a VPN route re-advertised under a new label replaces the stored route: one route remains, it carries the new label, and the published best names the new label.
func TestRFC8277VPNRelabelReplacesTheRoute(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	pe := netip.MustParseAddr("192.0.2.1")
	r.peerMeta[pe] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	first := vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)
	second := vpnv4NLRI(200, vpnRouteKeyRD, 0x0a)
	feedReceived(r, pe, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 1}, 100, first))
	feedReceived(r, pe, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 1}, 100, second))

	peer := r.bgpPeers[pe]
	if got := peer.FamilyLen(vpnv4Family); got != 1 {
		t.Fatalf("PE stores %d routes, want 1: the new label replaces the old", got)
	}
	if _, ok := peer.Lookup(vpnv4Family, first); !ok {
		t.Fatalf("route not found by its first NLRI: the route key carries no label")
	}
	var walked [][]byte
	peer.IterateFamily(vpnv4Family, func(nlri []byte, _ storage.RouteEntry) bool {
		walked = append(walked, bytes.Clone(nlri))
		return true
	})
	if len(walked) != 1 || !bytes.Equal(walked[0], second) {
		t.Fatalf("walk hands back %x, want the latest wire route %x", walked, second)
	}
	changes := vpnBestChanges(bus, vpnv4Family)
	if len(changes) != 2 {
		t.Fatalf("published %d best-changes, want 2 (add, then the relabel)", len(changes))
	}
	if !bytes.Equal(changes[1].NLRI, second) {
		t.Fatalf("relabel best NLRI = %x, want %x", changes[1].NLRI, second)
	}
}

// TestVPNWithdrawIgnoresCompatibilityValue withdraws a VPN route with the
// Compatibility value 0x800000 in place of its label, through the real ingest
// path.
//
// VALIDATES: the withdrawal reaches the stored route and the published
// withdrawal names the route as it was announced.
// PREVENTS: a withdrawal keyed with its Compatibility field, which matched no
// stored route and removed nothing (RFC 8277 Section 2.4: "Upon reception, the
// value of the Compatibility field MUST be ignored.").
//
// RFC requirement: RFC8277-2.4-1 positive -- a VPN withdrawal whose Compatibility field is 0x800000 removes the route announced under label 100 from the Adj-RIB-In, and the published withdrawal names the route as announced.
// RFC requirement: RFC8277-2.4-1 negative -- IPv4/IPv6 VPN withdrawals ignore valid nonrecommended Compatibility values and preserve every distinct RD, prefix and ADD-PATH zero/17 sibling with exact native replacement identity.
func TestVPNWithdrawIgnoresCompatibilityValue(t *testing.T) {
	compatibilityRouteState(t, family.SAFIVPN, false)
	compatibilityRouteState(t, family.SAFIVPN, true)
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	pe := netip.MustParseAddr("192.0.2.1")
	r.peerMeta[pe] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	announced := vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)
	feedReceived(r, pe, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 1}, 100, announced))
	withdrawn := vpnv4NLRI(0, vpnRouteKeyRD, 0x0a)
	withdrawn[1], withdrawn[2], withdrawn[3] = 0x80, 0x00, 0x00
	feedReceived(r, pe, ctxID, vpnv4WithdrawBody(withdrawn))

	if got := r.bgpPeers[pe].FamilyLen(vpnv4Family); got != 0 {
		t.Fatalf("PE stores %d routes after the withdrawal, want 0", got)
	}
	changes := vpnBestChanges(bus, vpnv4Family)
	if len(changes) != 2 {
		t.Fatalf("published %d best-changes, want 2 (add, withdraw)", len(changes))
	}
	if changes[1].Action != routeaction.Withdraw {
		t.Fatalf("second change action = %v, want withdraw", changes[1].Action)
	}
	if !bytes.Equal(changes[1].NLRI, announced) {
		t.Fatalf("withdraw NLRI = %x, want the announced route %x", changes[1].NLRI, announced)
	}
}
