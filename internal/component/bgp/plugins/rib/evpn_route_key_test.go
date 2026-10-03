// Design: docs/architecture/plugin/rib-storage-design.md -- non-CIDR route keys
// RFC: rfc/short/rfc8277.md -- a new label replaces the route it is bound to
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

var evpnFamily = family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}

// evpnMACRoute builds one RFC 7432 Section 7.2 MAC/IP Advertisement route:
// RD 0:1, the ESI given, a zero Ethernet Tag, MAC 00:00:5e:00:53:01, no IP
// address, and MPLS Label1. The ESI and the label are the two fields RFC 7432
// Section 7.2 leaves out of the route's identity.
func evpnMACRoute(esi byte, label uint32) []byte {
	return []byte{
		2, 33, // Route Type 2, length
		0, 0, 0, 0, 0, 0, 0, 1, // RD
		esi, 0, 0, 0, 0, 0, 0, 0, 0, 0, // ESI
		0, 0, 0, 0, // Ethernet Tag
		48, 0x00, 0x00, 0x5e, 0x00, 0x53, 0x01, // MAC Address Length, MAC Address
		0,                                                       // IP Address Length
		byte(label >> 12), byte(label >> 4), byte(label<<4) | 1, // MPLS Label1, bottom of stack
	}
}

// evpnAnnounceBody builds an UPDATE body announcing nlri via MP_REACH_NLRI
// (AFI 25 / SAFI 70) with an IPv4 next hop, ORIGIN, AS_PATH [65001] and a MED.
func evpnAnnounceBody(nextHop [4]byte, med uint32, nlri []byte) []byte {
	mpReach := []byte{0x00, 0x19, 0x46, 4, nextHop[0], nextHop[1], nextHop[2], nextHop[3], 0x00}
	mpReach = append(mpReach, nlri...)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9, // AS_PATH = [65001]
		0x80, 0x04, 0x04, byte(med >> 24), byte(med >> 16), byte(med >> 8), byte(med), // MED
	}
	attrs = append(attrs, 0x80, 0x0e, byte(len(mpReach))) //nolint:gosec // test NLRI is short
	attrs = append(attrs, mpReach...)

	body := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))} //nolint:gosec // test attrs are short
	return append(body, attrs...)
}

// evpnWithdrawBody builds an UPDATE body withdrawing nlri via MP_UNREACH_NLRI
// (AFI 25 / SAFI 70).
func evpnWithdrawBody(nlri []byte) []byte {
	mpValue := []byte{0x00, 0x19, 0x46}
	mpValue = append(mpValue, nlri...)

	attrs := []byte{0x80, 0x0f, byte(len(mpValue))} //nolint:gosec // test NLRI is short
	attrs = append(attrs, mpValue...)

	body := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))} //nolint:gosec // test attrs are short
	return append(body, attrs...)
}

// TestEVPNRelabelReplacesTheRoute re-advertises one EVPN MAC/IP route under a
// new label with unchanged attributes, without ADD-PATH, through the real
// ingest path.
//
// VALIDATES: the second UPDATE replaces the first: the Adj-RIB-In holds one
// route, the walk hands back the new label, and the best-change publishes the
// route under the new label.
// PREVENTS: the label inside the EVPN route key, which stored the relabelled
// route beside the old one, so both stayed installed and both were published.
func TestEVPNRelabelReplacesTheRoute(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	pe := netip.MustParseAddr("192.0.2.31")
	r.peerMeta[pe] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	first := evpnMACRoute(0, 100)
	second := evpnMACRoute(0, 200)
	feedReceived(r, pe, ctxID, evpnAnnounceBody([4]byte{10, 0, 0, 1}, 100, first))
	feedReceived(r, pe, ctxID, evpnAnnounceBody([4]byte{10, 0, 0, 1}, 100, second))

	peer := r.bgpPeers[pe]
	if got := peer.FamilyLen(evpnFamily); got != 1 {
		t.Fatalf("PE stores %d EVPN routes, want 1: the new label replaces the old", got)
	}
	var walked [][]byte
	peer.IterateFamily(evpnFamily, func(nlri []byte, _ storage.RouteEntry) bool {
		walked = append(walked, bytes.Clone(nlri))
		return true
	})
	if len(walked) != 1 || !bytes.Equal(walked[0], second) {
		t.Fatalf("walk hands back %x, want the latest wire route %x", walked, second)
	}
	changes := vpnBestChanges(bus, evpnFamily)
	if len(changes) != 2 {
		t.Fatalf("published %d best-changes, want 2 (add, then the relabel)", len(changes))
	}
	if changes[1].Action == routeaction.Withdraw {
		t.Fatalf("relabel published a withdrawal, want the route kept under its new label")
	}
	if !bytes.Equal(changes[1].NLRI, second) {
		t.Fatalf("relabel best NLRI = %x, want %x", changes[1].NLRI, second)
	}
}

// TestEVPNWithdrawWithOtherLabelAndESIRemovesTheRoute withdraws one EVPN
// MAC/IP route with a label and an ESI other than the ones it was announced
// with, through the real ingest path.
//
// VALIDATES: the withdrawal reaches the stored route, the Adj-RIB-In is empty,
// and the published withdrawal names the route as it was announced.
// PREVENTS: a withdrawal keyed with fields that do not identify the route,
// which matched nothing and left the route installed.
func TestEVPNWithdrawWithOtherLabelAndESIRemovesTheRoute(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	pe := netip.MustParseAddr("192.0.2.32")
	r.peerMeta[pe] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	announced := evpnMACRoute(0, 100)
	feedReceived(r, pe, ctxID, evpnAnnounceBody([4]byte{10, 0, 0, 1}, 100, announced))
	feedReceived(r, pe, ctxID, evpnWithdrawBody(evpnMACRoute(7, 0)))

	if got := r.bgpPeers[pe].FamilyLen(evpnFamily); got != 0 {
		t.Fatalf("PE stores %d EVPN routes after the withdrawal, want 0", got)
	}
	changes := vpnBestChanges(bus, evpnFamily)
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
