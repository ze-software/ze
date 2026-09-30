// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- flooding procedure (RFC 2328 Section 13.3).
// Related: flooding.go -- floodExcept, the normal flood whose outgoing copy carries the bumped age.
//
// VALIDATES: RFC 2328 Section 13.3 (5), "The LSA's LS age must be incremented by InfTransDelay
// (which must be > 0) when it is copied into the outgoing Link State Update packet (until the
// LS age field reaches the maximum value of MaxAge)", on the normal flood, not only on the
// retransmit and database-copy paths.
// PREVENTS: the first flooded copy carrying the unincremented LS age, and the bump counted
// twice (once into the queued retransmit copy, once more when it is retransmitted), and a
// retransmission leaving with the queue-time age rather than the age the LSA reached while
// held (RFC 2328 Section 14).
package lsdb

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// floodAgeTopology has eth0 (the receiving interface, neighbor 2.2.2.2) and eth1 (neighbor
// 3.3.3.3, InfTransDelay 7) in the backbone, both point-to-point and Full.
func floodAgeTopology() []InterfaceInfo {
	return []InterfaceInfo{
		{
			Name: "eth0", AreaID: area("0.0.0.0"), AreaType: types.AreaTypeNormal,
			NetworkType: types.NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			RetransmitInterval: 5,
			Neighbors:          []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull}},
		},
		{
			Name: "eth1", AreaID: area("0.0.0.0"), AreaType: types.AreaTypeNormal,
			NetworkType: types.NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 7,
			RetransmitInterval: 5,
			Neighbors:          []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.1.3"), State: NeighborStateFull}},
		},
	}
}

// agedRouterLSA is a stub-link Router-LSA from adv that arrives already aged (the LS age is
// not covered by the checksum, RFC 2328 Section 12.1.7).
func agedRouterLSA(t *testing.T, adv types.RouterID, age types.LSAge) packet.LSA {
	t.Helper()
	body := packet.RouterLSA{Links: []packet.RouterLink{{LinkID: lsid("10.0.0.0"), LinkData: ip4("255.255.255.0"), Type: packet.RouterLinkTypeStub, Metric: 1}}}
	lsa := packet.LSA{Header: packet.LSAHeader{Age: age, Options: types.OptionE, Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(adv), AdvertisingRouter: adv, Sequence: types.InitialSequenceNumber}, Router: &body}
	return encodeDecodeLSA(t, lsa)
}

// floodedAgeOn returns the LS age of the Router-LSA advertised by adv in the last LS Update
// sent out iface.
func floodedAgeOn(t *testing.T, tx *txRecorder, iface string, adv types.RouterID) types.LSAge {
	t.Helper()
	for i := range slices.Backward(tx.sends) {
		s := &tx.sends[i]
		if s.iface != iface || s.pkt.LSUpdate == nil {
			continue
		}
		for _, lsa := range s.pkt.LSUpdate.LSAs {
			if lsa.Header.AdvertisingRouter == adv {
				return lsa.Header.Age
			}
		}
	}
	t.Fatalf("no LS Update carrying %s's Router-LSA was flooded out %s", adv, iface)
	return 0
}

// TestRFC2328FloodIncrementsAgeByInfTransDelay receives a Router-LSA on eth0 and reads the LS
// age of the copy flooded out eth1 (InfTransDelay 7), then the copy retransmitted after the
// retransmit interval, then the copy of an LSA whose age plus InfTransDelay passes MaxAge.
func TestRFC2328FloodIncrementsAgeByInfTransDelay(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1000, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(floodAgeTopology)
	receive := func(adv string, age types.LSAge) {
		lsa := agedRouterLSA(t, rid(adv), age)
		db.ReceiveUpdate(ReceiveInput{
			Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"),
			Src:    netip.MustParseAddr("10.0.0.2"),
			Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}},
		})
	}

	receive("2.2.2.2", 10)
	// RFC requirement: RFC2328-13.3-2 positive -- the copy flooded out eth1 carries the stored
	// LS age (10) incremented by eth1's InfTransDelay (7), on the normal flood (§13.3).
	if got := floodedAgeOn(t, tx, "eth1", rid("2.2.2.2")); got != 17 {
		t.Fatalf("flooded LS age = %d, want 17 (stored 10 + InfTransDelay 7)", got)
	}
	// The retransmission five seconds later carries the age the LSA reached while held on
	// the list (10 + 5, RFC 2328 Section 14 ages it in the database) plus InfTransDelay once:
	// 22. The queue-time age (17) would mean the held copy never aged; 29 would mean the
	// flood's increment was counted into the queued copy as well.
	clock.Add(5 * time.Second)
	db.RetransmitTick(clock.now)
	if got := floodedAgeOn(t, tx, "eth1", rid("2.2.2.2")); got != 22 {
		t.Fatalf("retransmitted LS age = %d, want 22 (stored 10 + 5 s held + InfTransDelay 7)", got)
	}

	receive("4.4.4.4", types.LSAge(types.MaxAge-3))
	// RFC requirement: RFC2328-13.3-2 negative -- an LSA whose age plus InfTransDelay would pass
	// MaxAge leaves with MaxAge, never past it (§13.3 "until the LS age field reaches the
	// maximum value of MaxAge").
	if got := floodedAgeOn(t, tx, "eth1", rid("4.4.4.4")); got != types.LSAge(types.MaxAge) {
		t.Fatalf("flooded LS age = %d, want MaxAge %d (the increment stops at MaxAge)", got, types.MaxAge)
	}
}
