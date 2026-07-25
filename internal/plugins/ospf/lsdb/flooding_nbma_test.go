// VALIDATES: NBMA and non-broadcast point-to-multipoint flooding fans out a unicast
// LSUpdate to each Flood-eligible neighbor (IPv4 address / IPv6 link-local) instead of a
// multicast group, and PtMP re-floods a received LSA to the other neighbors (no DR-relay
// suppression) but never back to the sender.
// PREVENTS: a non-broadcast interface silently multicasting a flood its neighbors never
// receive.
package lsdb

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func floodDsts(tx *txRecorder) []netip.Addr {
	out := make([]netip.Addr, 0, len(tx.sends))
	for i := range tx.sends {
		s := &tx.sends[i]
		if s.pkt.LSUpdate != nil {
			out = append(out, s.dst)
		}
	}
	return out
}

func hasAddr(list []netip.Addr, a netip.Addr) bool {
	return slices.Contains(list, a)
}

func TestOSPFNBMAFloodUnicast(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	a := area("0.0.0.0")
	db.SetTx(tx.Send)
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "nb0", AreaID: a, AreaType: AreaTypeNormal, NetworkType: NetworkNBMA, State: InterfaceStateDR,
			Address: ip4("10.0.0.1"), NetworkMask: ip4("255.255.255.0"), RouterID: rid("1.1.1.1"), DR: rid("1.1.1.1"),
			Neighbors: []NeighborInfo{
				{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull},
				{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.0.3"), State: NeighborStateFull},
			},
		}}
	})
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.Install(a, lsa)
	db.floodExcept("", types.RouterID{}, a, lsa.Header.Key())
	dsts := floodDsts(tx)
	if len(dsts) != 2 || !hasAddr(dsts, naddr4("10.0.0.2")) || !hasAddr(dsts, naddr4("10.0.0.3")) {
		t.Fatalf("NBMA flood dsts = %v, want unicast to 10.0.0.2 and 10.0.0.3", dsts)
	}
	if hasAddr(dsts, transport.AllSPFRouters) {
		t.Fatalf("NBMA flood sent to the AllSPFRouters multicast group")
	}
}

func TestOSPFv3NBMAFloodUnicast(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	a := area("0.0.0.0")
	db.SetTx(tx.Send)
	ll2 := netip.MustParseAddr("fe80::2")
	ll3 := netip.MustParseAddr("fe80::3")
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "nb0", AreaID: a, AreaType: AreaTypeNormal, NetworkType: NetworkNBMA, State: InterfaceStateDR,
			RouterID: rid("1.1.1.1"), DR: rid("1.1.1.1"), IsV6: true, IPv6LinkLocal: netip.MustParseAddr("fe80::1"),
			Neighbors: []NeighborInfo{
				{RouterID: rid("2.2.2.2"), Address: ll2, State: NeighborStateFull},
				{RouterID: rid("3.3.3.3"), Address: ll3, State: NeighborStateFull},
			},
		}}
	})
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.Install(a, lsa)
	db.floodExcept("", types.RouterID{}, a, lsa.Header.Key())
	dsts := floodDsts(tx)
	if len(dsts) != 2 || !hasAddr(dsts, ll2) || !hasAddr(dsts, ll3) {
		t.Fatalf("v3 NBMA flood dsts = %v, want link-locals fe80::2/::3", dsts)
	}
	if hasAddr(dsts, netip.MustParseAddr("ff02::5")) {
		t.Fatalf("v3 NBMA flood sent to the ff02::5 multicast group")
	}
}

func TestOSPFPtMPFloodUnicast(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	a := area("0.0.0.0")
	db.SetTx(tx.Send)
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "ptp0", AreaID: a, AreaType: AreaTypeNormal, NetworkType: NetworkPointToMultipoint, State: "point-to-point",
			Address: ip4("10.0.0.1"), RouterID: rid("1.1.1.1"),
			Neighbors: []NeighborInfo{
				{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull},
				{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.0.3"), State: NeighborStateFull},
			},
		}}
	})
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	db.Install(a, lsa)
	db.floodExcept("", types.RouterID{}, a, lsa.Header.Key())
	dsts := floodDsts(tx)
	if len(dsts) != 2 || !hasAddr(dsts, naddr4("10.0.0.2")) || !hasAddr(dsts, naddr4("10.0.0.3")) {
		t.Fatalf("PtMP flood dsts = %v, want unicast to both neighbors", dsts)
	}
	if hasAddr(dsts, transport.AllSPFRouters) {
		t.Fatalf("PtMP flood sent to a multicast group")
	}
}

// TestOSPFPtMPFloodNoDRRelaySuppression: an LSA received on a PtMP interface is
// re-flooded to the OTHER neighbors (no DR-relay suppression), never back to the sender.
func TestOSPFPtMPFloodNoDRRelaySuppression(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	a := area("0.0.0.0")
	db.SetTx(tx.Send)
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "ptp0", AreaID: a, AreaType: AreaTypeNormal, NetworkType: NetworkPointToMultipoint, State: "point-to-point",
			Address: ip4("10.0.0.1"), RouterID: rid("1.1.1.1"),
			Neighbors: []NeighborInfo{
				{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull},
				{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.0.3"), State: NeighborStateFull},
			},
		}}
	})
	lsa := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	reason := db.ReceiveUpdate(ReceiveInput{Interface: "ptp0", AreaID: a, RouterID: rid("2.2.2.2"), Src: naddr4("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	dsts := floodDsts(tx)
	if !hasAddr(dsts, naddr4("10.0.0.3")) {
		t.Fatalf("PtMP did not re-flood to the other neighbor 10.0.0.3; dsts=%v", dsts)
	}
	if hasAddr(dsts, naddr4("10.0.0.2")) {
		t.Fatalf("PtMP re-flooded back to the sender 10.0.0.2; dsts=%v", dsts)
	}
}
