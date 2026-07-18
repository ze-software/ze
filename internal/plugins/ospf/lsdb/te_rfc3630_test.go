// VALIDATES: RFC 3630 sec 1 -- a Traffic Engineering LSA is an Opaque LSA (Opaque type 1,
// LS type 10 area-local scope; RFC 3630 sec 2.2). This carrier interprets no opaque body and
// consults no consumer registry (opaque_as.go): eligibleInterface / floodExcept (flooding.go)
// decide flooding purely by LS-type scope, never by the opaque type. So a node with no TE code
// still floods a received Type-10 opaque type-1 LSA exactly like any other area-local Type-10
// opaque LSA, and bounds it to its area exactly the same way.
// PREVENTS: gating opaque flooding on TE-capability (a non-TE node dropping TE LSAs and
// blackholing the TE topology), or leaking an area-local Type-10 opaque LSA into a foreign area.
package lsdb

import (
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// TestRFC3630NonTECapableFloodsTELSAByScope drives the consumer-agnostic opaque carrier with a
// TE LSA (Opaque type 1) and asserts flooding is decided by area scope alone -- there is no TE
// code in this package at all, which is exactly why a "non-TE capable node" floods TE LSAs.
//
// RFC requirement: RFC3630-1-1 positive -- a Type-10 opaque LSA carrying the TE Opaque type
//
//	(packet.TEOpaqueType) is flooded to a Flood-eligible opaque-capable neighbor in its own
//	area by the TE-agnostic carrier, exactly as any other area-local Type-10 opaque LSA.
//
// RFC requirement: RFC3630-1-1 negative -- the same area-local Type-10 TE-opaque LSA is NOT
//
//	flooded out an interface in a different area; area-local scope bounds it exactly like any
//	other Type-10 opaque LSA rather than being treated specially because it carries TE data.
func TestRFC3630NonTECapableFloodsTELSAByScope(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTx((&txRecorder{}).Send)
	a0 := area("0.0.0.0")
	a1 := area("0.0.0.1")

	// Two point-to-point interfaces in DIFFERENT areas, each with a Full, opaque-capable
	// neighbor. The ONLY difference between them is the area, so a difference in flooding
	// outcome can only be the area-local scope decision.
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{
			{
				Name: "eth0", AreaID: a0, AreaType: AreaTypeNormal,
				NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
				Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull, OpaqueCapable: true}},
			},
			{
				Name: "eth1", AreaID: a1, AreaType: AreaTypeNormal,
				NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
				Neighbors: []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.1.3"), State: NeighborStateFull, OpaqueCapable: true}},
			},
		}
	})

	// A Traffic Engineering LSA: Opaque type 1 (packet.TEOpaqueType), LS type 10 area-local
	// (RFC 3630 sec 2.2). The carrier never inspects the opaque type; install it into area 0.
	teLSA := opaqueLSA(t, types.LSTypeOpaqueArea, packet.TEOpaqueType, 0x10, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{0xde, 0xad, 0xbe, 0xef})
	if !db.Install(a0, teLSA) {
		t.Fatalf("Type-10 TE opaque LSA install into area 0 rejected")
	}
	db.floodExcept("", types.RouterID{}, a0, teLSA.Header.Key())

	// positive: flooded to the same-area opaque-capable neighbor, purely by area scope, with
	// no TE consumer or TE-capability check anywhere in the path.
	if db.retransmit[NeighborKey{Interface: "eth0", RouterID: rid("2.2.2.2")}][teLSA.Header.Key()] == nil {
		t.Fatalf("carrier did not flood the Type-10 TE opaque LSA to a same-area neighbor (RFC 3630 sec 1)")
	}
	// negative: NOT flooded out eth1 (a different area) -- area-local scope bounds it exactly
	// like any other Type-10 opaque LSA, so a TE LSA does not leak into a foreign area.
	if db.retransmit[NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}][teLSA.Header.Key()] != nil {
		t.Fatalf("area-local Type-10 TE opaque LSA leaked into a foreign area (RFC 3630 sec 1)")
	}
}
