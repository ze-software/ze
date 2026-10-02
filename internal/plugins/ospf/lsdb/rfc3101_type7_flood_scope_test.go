// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- Type-7 flooding scope.
//
// VALIDATES: RFC 3101 sec 2.3 on the LSDB flood path: a Type-7 LSA received in an NSSA is
// flooded out the other interface of that same NSSA, and never out an interface of a
// different NSSA or of the backbone.
// PREVENTS: an NSSA flood check that tests only the area type of the outgoing interface,
// which the helper-level TestOSPFType7FloodScope (backbone negative only) cannot see.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// twoNSSATopology is an area border router 1.1.1.1 with one Full neighbor per interface:
// eth0 backbone (2.2.2.2), eth1 and eth4 in NSSA 0.0.0.1 (3.3.3.3, 7.7.7.7), eth2 in a
// second NSSA 0.0.0.2 (5.5.5.5).
func twoNSSATopology() []InterfaceInfo {
	one := func(name, areaID, areaType, addr, nbr, nbrAddr string) InterfaceInfo {
		return InterfaceInfo{
			Name: name, AreaID: area(areaID), AreaType: areaType,
			NetworkType: types.NetworkBroadcast, State: InterfaceStateDR,
			Address: ip4(addr), NetworkMask: ip4("255.255.255.0"), RouterID: rid("1.1.1.1"), DR: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid(nbr), Address: naddr4(nbrAddr), State: NeighborStateFull}},
		}
	}
	return []InterfaceInfo{
		one("eth0", "0.0.0.0", types.AreaTypeNormal, "10.0.0.1", "2.2.2.2", "10.0.0.2"),
		one("eth1", "0.0.0.1", types.AreaTypeNSSA, "10.0.1.1", "3.3.3.3", "10.0.1.3"),
		one("eth2", "0.0.0.2", types.AreaTypeNSSA, "10.0.2.1", "5.5.5.5", "10.0.2.5"),
		one("eth4", "0.0.0.1", types.AreaTypeNSSA, "10.0.4.1", "7.7.7.7", "10.0.4.7"),
	}
}

// receiveType7OnEth1 builds the two-NSSA router and delivers one Type-7 LSA from 3.3.3.3
// on eth1 (NSSA 0.0.0.1) through ReceiveUpdate.
func receiveType7OnEth1(t *testing.T) (*LSDB, *txRecorder, types.LSAKey) {
	t.Helper()
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetAreaTypes(map[types.AreaID]string{area("0.0.0.1"): types.AreaTypeNSSA, area("0.0.0.2"): types.AreaTypeNSSA})
	db.SetTopology(twoNSSATopology)
	lsa := nssaLSA(t, rid("3.3.3.3"), types.InitialSequenceNumber)
	reason := db.ReceiveUpdate(ReceiveInput{Interface: "eth1", AreaID: area("0.0.0.1"), RouterID: rid("3.3.3.3"), Src: netip.MustParseAddr("10.0.1.3"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	return db, tx, lsa.Header.Key()
}

// RFC requirement: RFC3101-2.3-2 positive -- a Type-7 LSA received from the NSSA 0.0.0.1
// neighbor on eth1 is installed in area 0.0.0.1 and flooded in an LS Update out eth4, the
// router's other interface in that same NSSA (floodExcept/eligibleInterface, flooding.go).
func TestRFC3101Type7FloodedWithinItsNSSA(t *testing.T) {
	db, tx, key := receiveType7OnEth1(t)
	if _, ok := db.Lookup(area("0.0.0.1"), key); !ok {
		t.Fatal("the Type-7 LSA was not installed in its NSSA 0.0.0.1")
	}
	if !lsaFloodedOn(tx, "eth4", key) {
		t.Fatalf("the Type-7 LSA was not flooded out eth4, the other NSSA 0.0.0.1 interface: %+v", tx.sends)
	}
}

// RFC requirement: RFC3101-2.3-2 negative -- the same Type-7 LSA never leaves NSSA 0.0.0.1:
// no LS Update carries it out eth2 (a DIFFERENT NSSA, 0.0.0.2) or eth0 (backbone), the
// eth2 and eth0 neighbors hold no retransmission entry for it, and area 0.0.0.2 holds no copy.
func TestRFC3101Type7NotFloodedIntoAnotherNSSA(t *testing.T) {
	db, tx, key := receiveType7OnEth1(t)
	if !lsaFloodedOn(tx, "eth4", key) {
		t.Fatalf("setup: the Type-7 LSA did not reach eth4, so its absence elsewhere proves nothing: %+v", tx.sends)
	}
	for _, out := range []string{"eth2", "eth0"} {
		if lsaFloodedOn(tx, out, key) {
			t.Errorf("an LS Update carrying the NSSA 0.0.0.1 Type-7 LSA was flooded out %s", out)
		}
	}
	for _, nk := range []NeighborKey{{Interface: "eth2", RouterID: rid("5.5.5.5")}, {Interface: "eth0", RouterID: rid("2.2.2.2")}} {
		if db.retransmit[nk][key] != nil {
			t.Errorf("the NSSA 0.0.0.1 Type-7 LSA is on %s's retransmission list", nk.RouterID)
		}
	}
	if _, ok := db.Lookup(area("0.0.0.2"), key); ok {
		t.Error("the NSSA 0.0.0.1 Type-7 LSA was installed in NSSA 0.0.0.2")
	}
}
