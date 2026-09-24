// RFC: rfc/short/rfc8955.md -- Section 6, path selection over Flow Specification routes
// Related: rib_bestchange.go -- checkBestPathChange, the producer under test
// Related: rfc4364_vpn_bestchange_test.go -- the VPN-IPv4 sibling these mirror

package rib

import (
	"bytes"
	"net/netip"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

// flowspecFamily is AFI 1 / SAFI 133 (RFC 8955 Section 11).
var flowspecFamily = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

// flowspecNLRI builds one RFC 8955 Section 4.1 Flow Specification NLRI whose
// only component is a Type 1 Destination Prefix of prefixLen bits. The leading
// length octet counts the component bytes, so it is 2 plus the prefix octets.
func flowspecNLRI(prefixLen byte, prefixOctets ...byte) []byte {
	nlri := []byte{byte(2 + len(prefixOctets)), 0x01, prefixLen} //nolint:gosec // test NLRI is short
	return append(nlri, prefixOctets...)
}

// flowspecAnnounceBody builds an UPDATE body announcing nlri via MP_REACH_NLRI
// (AFI 1 / SAFI 133) with ORIGIN, a one-hop AS_PATH and a MED.
//
// RFC 8955 Section 4.1: "the Length of the Next-Hop Network Address MUST be
// set to 0", so the MP_REACH carries no next hop at all.
func flowspecAnnounceBody(med uint32, nlri []byte) []byte {
	mpReach := []byte{0x00, 0x01, 0x85, 0x00, 0x00} // AFI 1, SAFI 133, NH length 0, Reserved
	mpReach = append(mpReach, nlri...)

	// AS_PATH carries one AS_SEQUENCE of a single 4-octet ASN. RFC 4271
	// Section 9.1.2.2 (c) compares MED only between paths whose neighbor AS is
	// the same, so both peers announce from 65001.
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9, // AS_PATH = [65001]
		0x80, 0x04, 0x04, byte(med >> 24), byte(med >> 16), byte(med >> 8), byte(med), // MED
		0x80, 0x09, 0x04, 0x01, 0x01, 0x01, 0x01, // same ORIGINATOR_ID as covering unicast
	}
	attrs = append(attrs, 0x80, 0x0e, byte(len(mpReach))) //nolint:gosec // test NLRI is short
	attrs = append(attrs, mpReach...)

	body := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))} //nolint:gosec // test attrs are short
	return append(body, attrs...)
}

// flowspecWithdrawBody builds an UPDATE body withdrawing nlri via
// MP_UNREACH_NLRI (AFI 1 / SAFI 133).
func flowspecWithdrawBody(nlri []byte) []byte {
	mpValue := []byte{0x00, 0x01, 0x85}
	mpValue = append(mpValue, nlri...)

	attrs := []byte{0x80, 0x0f, byte(len(mpValue))} //nolint:gosec // test NLRI is short
	attrs = append(attrs, mpValue...)

	body := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))} //nolint:gosec // test attrs are short
	return append(body, attrs...)
}

// TestRFC8955FlowSpecPathSelectionPicksOneSetOfAttributes drives two peers'
// paths for one Flow Specification NLRI through the RIB ingest path and reads
// which path's attributes the Loc-RIB publishes at each step.
//
// VALIDATES: a Flow Specification route reaches the RFC 4271 Section 9.1
// decision process like any other family. Two paths for the same NLRI are
// ranked, the lower MED wins, and a withdrawal of the winner hands the best
// back to the survivor, each step published on (bgp-rib, best-change) with
// the winning path's MED as the entry metric.
// PREVENTS: the RIB storing a Flow Specification route without ranking it,
// or ranking it by arrival order, so that the attributes advertised are
// whichever path landed last.
//
// RFC requirement: RFC8955-6-5 positive -- two Flow Specification paths for the same NLRI (AFI 1 / SAFI 133) from two peers go through path selection: the lower-MED path is published as best with its own MED, and when it is withdrawn the surviving path is published as best with its MED (Section 6).
func TestRFC8955FlowSpecPathSelectionPicksOneSetOfAttributes(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerA := netip.MustParseAddr("192.0.2.1")
	peerB := netip.MustParseAddr("192.0.2.2")
	r.peerMeta[peerA] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	r.peerMeta[peerB] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	nlri := flowspecNLRI(24, 10, 0, 0) // destination 10.0.0.0/24
	flowValidationReceive(t, r, peerA, 65001, 65000, 1, family.IPv4Unicast, []byte{8, 10},
		flowValidationAttrs(flowValidationPath(2, 65001), netip.MustParseAddr("1.1.1.1"), 0, nil), false)

	// Peer A announces with MED 200.
	feedReceived(r, peerA, ctxID, flowspecAnnounceBody(200, nlri))

	changes := vpnBestChanges(bus, flowspecFamily)
	if len(changes) != 1 {
		t.Fatalf("published %d best-changes for %v, want 1 (add)", len(changes), flowspecFamily)
	}
	add := changes[0]
	if add.Action != routeaction.Add {
		t.Errorf("action = %v, want add", add.Action)
	}
	if !bytes.Equal(add.NLRI, nlri) {
		t.Errorf("NLRI = %x, want %x", add.NLRI, nlri)
	}
	if add.Prefix.IsValid() {
		t.Errorf("Prefix = %v, want the zero prefix: a Flow Specification NLRI names no CIDR prefix", add.Prefix)
	}
	if add.Metric != 200 {
		t.Errorf("Metric = %d, want 200: the only path's MED", add.Metric)
	}

	// Peer B announces the same NLRI with a lower MED, so it wins.
	feedReceived(r, peerB, ctxID, flowspecAnnounceBody(100, nlri))

	changes = vpnBestChanges(bus, flowspecFamily)
	if len(changes) != 2 {
		t.Fatalf("published %d best-changes after the winning UPDATE, want 2", len(changes))
	}
	upd := changes[1]
	if upd.Action != routeaction.Update {
		t.Errorf("action = %v, want update", upd.Action)
	}
	if upd.Metric != 100 {
		t.Errorf("Metric = %d, want 100: the lower-MED path did not take the best", upd.Metric)
	}
	if !bytes.Equal(upd.NLRI, nlri) {
		t.Errorf("NLRI = %x, want %x", upd.NLRI, nlri)
	}

	// Peer B withdraws: peer A's path is best again.
	feedReceived(r, peerB, ctxID, flowspecWithdrawBody(nlri))

	changes = vpnBestChanges(bus, flowspecFamily)
	if len(changes) != 3 {
		t.Fatalf("published %d best-changes after B withdrew, want 3", len(changes))
	}
	back := changes[2]
	if back.Action != routeaction.Update {
		t.Errorf("action = %v, want update", back.Action)
	}
	if back.Metric != 200 {
		t.Errorf("Metric = %d, want 200: the surviving path did not take the best", back.Metric)
	}
}

// TestRFC8955FlowSpecLosingPathIsNeverPublished is the counter-case: a second
// path that loses the comparison changes nothing in the Loc-RIB, and its
// attributes never reach (bgp-rib, best-change).
//
// VALIDATES: path selection runs on the second path and rejects it, rather
// than the RIB publishing every received Flow Specification path.
// PREVENTS: a RIB that treats each received path for the same NLRI as the
// new best, which advertises the last-received attributes rather than the
// selected ones.
//
// RFC requirement: RFC8955-6-5 negative -- a second Flow Specification path for the same NLRI with a HIGHER MED loses path selection: no best-change is published for it and the published best keeps the first path's MED (Section 6).
func TestRFC8955FlowSpecLosingPathIsNeverPublished(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerA := netip.MustParseAddr("192.0.2.1")
	peerB := netip.MustParseAddr("192.0.2.2")
	r.peerMeta[peerA] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	r.peerMeta[peerB] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	nlri := flowspecNLRI(24, 10, 0, 0)
	flowValidationReceive(t, r, peerA, 65001, 65000, 1, family.IPv4Unicast, []byte{8, 10},
		flowValidationAttrs(flowValidationPath(2, 65001), netip.MustParseAddr("1.1.1.1"), 0, nil), false)

	feedReceived(r, peerA, ctxID, flowspecAnnounceBody(100, nlri))
	feedReceived(r, peerB, ctxID, flowspecAnnounceBody(200, nlri))

	changes := vpnBestChanges(bus, flowspecFamily)
	if len(changes) != 1 {
		t.Fatalf("published %d best-changes, want 1: the higher-MED path must not displace the best", len(changes))
	}
	if changes[0].Metric != 100 {
		t.Errorf("Metric = %d, want 100: the published best is not the selected path", changes[0].Metric)
	}
	if changes[0].Action != routeaction.Add {
		t.Errorf("action = %v, want add", changes[0].Action)
	}
}
