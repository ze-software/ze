// VALIDATES: spec-ospf-ext-1 AC-5/R-2 -- an opaque LSA is queued for flooding ONLY to
// neighbors that advertised the O-bit (OpaqueCapable); a non-opaque neighbor on the same
// interface is never queued, while a non-opaque (Type 1) LSA still reaches both.
// PREVENTS: flooding an opaque LSA to a non-opaque peer (RFC 5250 §3.1 violation) that
// wastes its LSDB or triggers ack storms.
package lsdb

import (
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func TestOpaqueFloodOnlyToOpaqueNeighbor(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	a0 := area("0.0.0.0")
	db.SetTx(tx.Send)
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

	// A Type 11 opaque LSA: only the opaque-capable neighbor is queued (RFC 5250 §3.1).
	lsa11 := opaqueLSA(t, types.LSTypeOpaqueAS, 4, 0x50, rid("9.9.9.9"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})
	if !db.Install(types.BackboneArea, lsa11) {
		t.Fatalf("Type 11 install rejected")
	}
	db.floodExcept("", types.RouterID{}, types.BackboneArea, lsa11.Header.Key())
	// RFC requirement: RFC5250-3.1-4 positive -- an opaque LSA is flooded to an opaque-capable (O-bit) neighbor
	if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}][lsa11.Header.Key()] == nil {
		t.Fatalf("opaque LSA not queued for the opaque-capable neighbor")
	}
	// RFC requirement: RFC5250-3.1-4 negative -- an opaque LSA is not flooded to a non-opaque neighbor
	if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid("3.3.3.3")}][lsa11.Header.Key()] != nil {
		t.Fatalf("opaque LSA wrongly queued for a non-opaque neighbor (RFC 5250 §3.1)")
	}

	// A non-opaque (Router) LSA still reaches BOTH neighbors: the gate is opaque-only.
	router := routerLSA(t, rid("4.4.4.4"), types.InitialSequenceNumber, 10)
	if !db.Install(a0, router) {
		t.Fatalf("router LSA install rejected")
	}
	db.floodExcept("", types.RouterID{}, a0, router.Header.Key())
	for _, peer := range []string{"2.2.2.2", "3.3.3.3"} {
		if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid(peer)}][router.Header.Key()] == nil {
			t.Fatalf("non-opaque LSA should reach every full neighbor, missing %s", peer)
		}
	}
}
