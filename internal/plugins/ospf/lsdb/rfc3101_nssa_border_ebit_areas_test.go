// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- NSSA border router E-bit.
//
// VALIDATES: RFC 3101 sec 3.1 for every directly attached area, not only the backbone: an
// area border router attached to an NSSA, originating no external LSA and not translating,
// sets the E-bit in the router-LSA it originates into the backbone, into a normal area and
// into the NSSA itself; with no NSSA attached the same router leaves it clear in each.
// PREVENTS: the NSSA border E-bit reaching only the backbone router-LSA, which the
// backbone-only TestRFC3101NSSABorderRouterSetsEBit cannot see.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// threeAreaBorderTopology is ABR 1.1.1.1 with one Full neighbor in the backbone (eth0), in
// area 0.0.0.1 of the given kind (eth9) and in the normal area 0.0.0.2 (eth8).
func threeAreaBorderTopology(kind string) func() []InterfaceInfo {
	return func() []InterfaceInfo {
		withKind := nssaBorderTopology(kind)()
		normal := originTopology()[0]
		normal.Name = "eth8"
		normal.AreaID = area("0.0.0.2")
		normal.AreaType = types.AreaTypeNormal
		normal.Address = ip4("10.8.0.1")
		normal.Neighbors = []NeighborInfo{{RouterID: rid("4.4.4.4"), Address: naddr4("10.8.0.4"), State: NeighborStateFull}}
		normal.BDR = rid("4.4.4.4")
		return append(withKind, normal)
	}
}

// routerLSAEBitIn reports the E-bit of router's own router-LSA in areaID.
func routerLSAEBitIn(t *testing.T, db *LSDB, areaID types.AreaID, router types.RouterID) bool {
	t.Helper()
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router), AdvertisingRouter: router}
	lsa, ok := db.LookupLSA(areaID, key)
	if !ok {
		t.Fatalf("no router-LSA of %v in area %v", router, areaID)
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("decode router-LSA in area %v: %v", areaID, err)
	}
	return body.Flags&packet.RouterFlagE != 0
}

// originateThreeAreaBorder originates every self LSA of the three-area ABR from the topology.
func originateThreeAreaBorder(t *testing.T, kind string) *LSDB {
	t.Helper()
	router := rid("1.1.1.1")
	db := newTestDB(&fakeClock{now: time.Unix(0, 0)})
	db.SetTopology(threeAreaBorderTopology(kind))
	db.OriginateFromTopology(router, false)
	if db.SelfIsASBR(router) {
		t.Fatal("fixture error: the border router originates an external LSA")
	}
	return db
}

// RFC requirement: RFC3101-3.1-1 positive -- an ABR attached to NSSA 0.0.0.1, the backbone
// and normal area 0.0.0.2, originating no external LSA and not translating, sets the E-bit
// in its router-LSA of each directly attached non-stub area: 0.0.0.0, 0.0.0.2 and 0.0.0.1.
func TestRFC3101NSSABorderEBitInEveryNonStubArea(t *testing.T) {
	db := originateThreeAreaBorder(t, types.AreaTypeNSSA)
	for _, a := range []string{"0.0.0.0", "0.0.0.2", "0.0.0.1"} {
		if !routerLSAEBitIn(t, db, area(a), rid("1.1.1.1")) {
			t.Errorf("NSSA border router left the E-bit clear in its area %s router-LSA", a)
		}
	}
}

// RFC requirement: RFC3101-3.1-1 negative -- the same three-area ABR with area 0.0.0.1
// normal (no NSSA attached, no external LSA) is not an NSSA border router: the E-bit is
// clear in its router-LSA of every area, 0.0.0.0, 0.0.0.2 and 0.0.0.1.
func TestRFC3101NoNSSAAttachedEBitClearEverywhere(t *testing.T) {
	db := originateThreeAreaBorder(t, types.AreaTypeNormal)
	for _, a := range []string{"0.0.0.0", "0.0.0.2", "0.0.0.1"} {
		if routerLSAEBitIn(t, db, area(a), rid("1.1.1.1")) {
			t.Errorf("ABR with no NSSA and no external set the E-bit in its area %s router-LSA", a)
		}
	}
}
