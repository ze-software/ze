// VALIDATES: RFC 5250 Section 3.1 AS-wide (Type-11) retransmit bookkeeping and the Type-9
// link-local O-bit flood gate -- an AS-wide opaque purge acked in one area is NOT deleted
// while another area's retransmit is still pending, and a self-originated Type-9 opaque LSA
// is flooded (floodLink) only to opaque-capable neighbors.
// PREVENTS: a Type-11 opaque purge being dropped as if area-scoped (leaving peers in other
// areas with a stale instance), and a Type-9 opaque LSA queued to a non-opaque neighbor.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// twoAreaOpaqueTopology has eth0 in area 0.0.0.0 and eth1 in area 0.0.0.1, each with one
// opaque-capable Full neighbor, so an AS-wide (Type-11) flood queues a retransmit per area.
func twoAreaOpaqueTopology() []InterfaceInfo {
	return []InterfaceInfo{
		{
			Name: "eth0", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
			NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull, OpaqueCapable: true}},
		},
		{
			Name: "eth1", AreaID: area("0.0.0.1"), AreaType: AreaTypeNormal,
			NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.1.3"), State: NeighborStateFull, OpaqueCapable: true}},
		},
	}
}

// TestType11PurgeRetainedWhileOtherAreaPending pins fix B: a self-originated Type-11 opaque
// purge flooded into two areas must survive an ack from one area while the other area's
// retransmit is still outstanding (the AS-wide instance spans every area at once).
func TestType11PurgeRetainedWhileOtherAreaPending(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTx((&txRecorder{}).Send)
	db.SetTopology(twoAreaOpaqueTopology)

	in := OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 4, OpaqueID: 0x77, Scope: types.LSTypeOpaqueAS,
		Options: types.OptionO, Body: []byte{1, 2, 3, 4},
	}
	if _, ok := db.OriginateOpaque(in); !ok {
		t.Fatalf("Type-11 origination failed")
	}
	key := types.LSAKey{Type: types.LSTypeOpaqueAS, LinkStateID: packet.OpaqueLinkStateID(4, 0x77), AdvertisingRouter: rid("1.1.1.1")}

	clock.Add(2 * time.Second) // clear MinLSInterval so the withdraw flush installs + floods
	in.Withdraw = true
	purged, ok := db.OriginateOpaque(in)
	if !ok || !purged.Age.IsMaxAge() {
		t.Fatalf("withdraw did not flood a MaxAge purge: ok=%v hdr=%+v", ok, purged)
	}

	nbr0 := NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}
	nbr1 := NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}
	if db.retransmit[nbr0][key] == nil || db.retransmit[nbr1][key] == nil {
		t.Fatalf("purge not queued for retransmit in both areas")
	}

	// Area 0 acks first: clears eth0's retransmit, then deletePurgedIfAcked(area0). Area 1's
	// retransmit is still pending, so the AS-wide purge MUST be retained.
	db.ReceiveAck(AckInput{Interface: "eth0", AreaID: area("0.0.0.0"), RouterID: rid("2.2.2.2"), Ack: packet.LSAck{Headers: []packet.LSAHeader{purged}}})
	if _, ok := db.LookupLSA(types.BackboneArea, key); !ok {
		t.Fatalf("AS-wide purge deleted while area-1 retransmit still pending (Type-11 treated as area-scoped)")
	}
	if db.retransmit[nbr1][key] == nil {
		t.Fatalf("area-1 retransmit unexpectedly cleared by an area-0 ack")
	}

	// Area 1 acks: no retransmit remains, so the purge is finally removed.
	db.ReceiveAck(AckInput{Interface: "eth1", AreaID: area("0.0.0.1"), RouterID: rid("3.3.3.3"), Ack: packet.LSAck{Headers: []packet.LSAHeader{purged}}})
	if _, ok := db.LookupLSA(types.BackboneArea, key); ok {
		t.Fatalf("AS-wide purge not removed after both areas acknowledged it")
	}
}

// TestOpaqueLinkFloodOnlyToOpaqueNeighbor pins fix C: floodLink queues a Type-9 link-local
// opaque LSA only for opaque-capable neighbors (RFC 5250 Section 3.1).
func TestOpaqueLinkFloodOnlyToOpaqueNeighbor(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTx((&txRecorder{}).Send)
	a0 := area("0.0.0.0")
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "eth0", AreaID: a0, AreaType: AreaTypeNormal,
			NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{
				{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull, OpaqueCapable: true},
				{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.0.3"), State: NeighborStateFull, OpaqueCapable: false},
			},
		}}
	})

	if _, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 3, OpaqueID: 0x01, Scope: types.LSTypeOpaqueLink,
		Interface: "eth0", Area: a0, Options: types.OptionO, Body: []byte{9, 9, 9, 9},
	}); !ok {
		t.Fatalf("Type-9 opaque origination failed")
	}
	key := types.LSAKey{Type: types.LSTypeOpaqueLink, LinkStateID: packet.OpaqueLinkStateID(3, 0x01), AdvertisingRouter: rid("1.1.1.1")}
	if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}][key] == nil {
		t.Fatalf("Type-9 opaque not queued for the opaque-capable neighbor")
	}
	if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid("3.3.3.3")}][key] != nil {
		t.Fatalf("Type-9 opaque wrongly queued for a non-opaque neighbor (RFC 5250 Section 3.1)")
	}
}
