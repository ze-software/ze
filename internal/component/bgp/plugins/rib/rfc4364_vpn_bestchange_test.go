// RFC: rfc/short/rfc4364.md -- Section 4.3.4 the VPN-IPv4 NLRI, Section 6.1 its next hop
// Related: rib_bestchange.go -- checkBestPathChange, the producer under test
// Related: srv6_transposition_test.go -- feedReceived, the ingest entry point these drive

package rib

import (
	"bytes"
	"net/netip"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

// vpnv4Family is the family every case here drives.
var vpnv4Family = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}

// vpnv4NLRI builds one RFC 4364 Section 4.3.4 VPN-IPv4 NLRI for an IPv4
// prefix. The single length octet counts the 24-bit label stack and the
// 64-bit Route Distinguisher as well as the prefix, which is exactly why
// store.NLRIToPrefix refuses these bytes: 10.0.0.0/8 gives 96, and no IPv4
// prefix length exceeds 32.
func vpnv4NLRI(label uint32, rd [8]byte, prefixLen byte, prefixOctets ...byte) []byte {
	nlri := []byte{24 + 64 + prefixLen}
	// RFC 8277 Section 2.1: the 20-bit Label Value sits in the high-order bits
	// of the three octets; the bottom-of-stack bit is the lowest.
	nlri = append(nlri, byte(label>>12), byte(label>>4), byte(label<<4)|0x01)
	nlri = append(nlri, rd[:]...)
	return append(nlri, prefixOctets...)
}

// vpnv4AnnounceBody builds an UPDATE body announcing nlri via MP_REACH_NLRI
// (AFI 1 / SAFI 128) with ORIGIN, a one-hop AS_PATH and a MED.
//
// RFC 4364 Section 6.1: the next hop "is encoded as a VPN-IPv4 address with an
// RD of 0", so the Network Address of Next Hop is 12 octets, not 4.
func vpnv4AnnounceBody(nextHop [4]byte, med uint32, nlri []byte) []byte {
	mpReach := []byte{0x00, 0x01, 0x80, 12,
		0, 0, 0, 0, 0, 0, 0, 0, // Route Distinguisher 0
		nextHop[0], nextHop[1], nextHop[2], nextHop[3],
		0x00, // Reserved
	}
	mpReach = append(mpReach, nlri...)

	// AS_PATH carries one AS_SEQUENCE of a single 4-octet ASN. RFC 4271
	// Section 9.1.2.2 (c) compares MED only between paths whose neighbor AS is
	// the same, so an empty AS_PATH would leave MED unread and the tie would
	// fall through to the peer address.
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

// vpnv4WithdrawBody builds an UPDATE body withdrawing nlri via
// MP_UNREACH_NLRI (AFI 1 / SAFI 128).
func vpnv4WithdrawBody(nlri []byte) []byte {
	mpValue := []byte{0x00, 0x01, 0x80}
	mpValue = append(mpValue, nlri...)

	attrs := []byte{0x80, 0x0f, byte(len(mpValue))} //nolint:gosec // test NLRI is short
	attrs = append(attrs, mpValue...)

	body := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))} //nolint:gosec // test attrs are short
	return append(body, attrs...)
}

// vpnBestChanges returns every best-change entry published for fam, in the
// order the bus saw them.
func vpnBestChanges(bus *testEventBus, fam family.Family) []ribevents.BestChangeEntry {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var out []ribevents.BestChangeEntry
	for i := range bus.events {
		batch, ok := bus.events[i].Payload.(*bestChangeBatch)
		if !ok || batch.Family != fam {
			continue
		}
		out = append(out, batch.Changes...)
	}
	return out
}

// TestVPNv4BestChangeIsPublished drives real VPNv4 UPDATEs through the RIB
// ingest path and follows the best path across an add, a win by a second peer,
// and both withdrawals.
//
// VALIDATES: a family whose NLRI is not a CIDR prefix reaches best-path
// selection and publishes on (bgp-rib, best-change), identified by its wire
// NLRI. RFC 4364 Section 4.3.4 gives that NLRI a length octet counting the
// label stack and the Route Distinguisher, and Section 6.1 gives it an
// RD-prefixed next hop; both are read here.
// PREVENTS: the silent fall-through this file was written for -- keying the
// best-prev record through store.NLRIToPrefix, which refuses a length octet
// above 32 for IPv4, so checkBestPathChange took its malformed-NLRI early
// return and NO best-path change was ever published for VPN, EVPN, MVPN, MUP,
// flowspec, VPLS or BGP-LS. The routes still landed in the Adj-RIB-In, so
// nothing was red.
func TestVPNv4BestChangeIsPublished(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerA := netip.MustParseAddr("192.0.2.1")
	peerB := netip.MustParseAddr("192.0.2.2")
	r.peerMeta[peerA] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	r.peerMeta[peerB] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	rd := [8]byte{0, 0, 0, 0, 0, 0, 0, 0}
	nlri := vpnv4NLRI(100, rd, 8, 0x0a) // label 100, RD 0:0, 10.0.0.0/8

	// Peer A announces with MED 200.
	feedReceived(r, peerA, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 1}, 200, nlri))

	changes := vpnBestChanges(bus, vpnv4Family)
	if len(changes) != 1 {
		t.Fatalf("published %d best-changes for %v, want 1 (add)", len(changes), vpnv4Family)
	}
	add := changes[0]
	if add.Action != routeaction.Add {
		t.Errorf("action = %v, want add", add.Action)
	}
	if !bytes.Equal(add.NLRI, nlri) {
		t.Errorf("NLRI = %x, want %x", add.NLRI, nlri)
	}
	if add.Prefix.IsValid() {
		t.Errorf("Prefix = %v, want the zero prefix: a VPN NLRI names no CIDR prefix", add.Prefix)
	}
	// RFC 4364 Section 6.1: the 8-octet RD ahead of the address is not part of it.
	if want := netip.MustParseAddr("10.0.0.1"); add.NextHop != want {
		t.Errorf("NextHop = %v, want %v", add.NextHop, want)
	}

	// Peer B announces the same route with a lower MED, so it wins.
	feedReceived(r, peerB, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 2}, 100, nlri))

	changes = vpnBestChanges(bus, vpnv4Family)
	if len(changes) != 2 {
		t.Fatalf("published %d best-changes after the winning UPDATE, want 2", len(changes))
	}
	upd := changes[1]
	if upd.Action != routeaction.Update {
		t.Errorf("action = %v, want update", upd.Action)
	}
	if want := netip.MustParseAddr("10.0.0.2"); upd.NextHop != want {
		t.Errorf("NextHop = %v, want %v: the lower-MED path did not take the best", upd.NextHop, want)
	}
	if !bytes.Equal(upd.NLRI, nlri) {
		t.Errorf("NLRI = %x, want %x", upd.NLRI, nlri)
	}

	// Peer B withdraws: peer A's path is best again.
	feedReceived(r, peerB, ctxID, vpnv4WithdrawBody(nlri))

	changes = vpnBestChanges(bus, vpnv4Family)
	if len(changes) != 3 {
		t.Fatalf("published %d best-changes after B withdrew, want 3", len(changes))
	}
	back := changes[2]
	if back.Action != routeaction.Update {
		t.Errorf("action = %v, want update", back.Action)
	}
	if want := netip.MustParseAddr("10.0.0.1"); back.NextHop != want {
		t.Errorf("NextHop = %v, want %v: the surviving path did not take the best", back.NextHop, want)
	}

	// Peer A withdraws the last path: the route leaves the Loc-RIB.
	feedReceived(r, peerA, ctxID, vpnv4WithdrawBody(nlri))

	changes = vpnBestChanges(bus, vpnv4Family)
	if len(changes) != 4 {
		t.Fatalf("published %d best-changes after A withdrew, want 4", len(changes))
	}
	gone := changes[3]
	if gone.Action != routeaction.Withdraw {
		t.Errorf("action = %v, want withdraw", gone.Action)
	}
	if !bytes.Equal(gone.NLRI, nlri) {
		t.Errorf("withdraw NLRI = %x, want %x", gone.NLRI, nlri)
	}
}

// TestVPNv4BestChangeKeysOnTheWholeNLRI feeds two VPN routes that differ only
// in Route Distinguisher, which is the case a netip.Prefix key cannot hold.
//
// VALIDATES: the best-prev record is keyed by the wire NLRI, so two VRFs
// advertising the same IPv4 prefix are two routes.
// PREVENTS: a repair that extracts the embedded IPv4 prefix and keys on that
// instead. Such a key passes the test above and collapses these two routes
// into one, which is the RFC 4364 Section 4.1 problem the RD exists to solve.
func TestVPNv4BestChangeKeysOnTheWholeNLRI(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peer := netip.MustParseAddr("192.0.2.1")
	r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	nlriVRF1 := vpnv4NLRI(100, [8]byte{0, 0, 0, 0, 0, 0, 0, 1}, 8, 0x0a)
	nlriVRF2 := vpnv4NLRI(200, [8]byte{0, 0, 0, 0, 0, 0, 0, 2}, 8, 0x0a)

	feedReceived(r, peer, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 1}, 100, nlriVRF1))
	feedReceived(r, peer, ctxID, vpnv4AnnounceBody([4]byte{10, 0, 0, 2}, 100, nlriVRF2))

	changes := vpnBestChanges(bus, vpnv4Family)
	if len(changes) != 2 {
		t.Fatalf("published %d best-changes for two Route Distinguishers, want 2", len(changes))
	}
	for i, c := range changes {
		if c.Action != routeaction.Add {
			t.Errorf("change %d action = %v, want add: each RD is its own route", i, c.Action)
		}
	}
	if bytes.Equal(changes[0].NLRI, changes[1].NLRI) {
		t.Errorf("both changes carry NLRI %x: the Route Distinguisher left the key", changes[0].NLRI)
	}
}
