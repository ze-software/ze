// VALIDATES: RFC 3101 Section 3.1: an NSSA border router sets the E-bit in the router-LSA
// of its directly attached non-stub areas even when it originates no external and does
// not translate, so the other NSSA border routers see it as an ASBR over area 0.
// PREVENTS: the E-bit following only self-originated Type-5/Type-7 LSAs, which hides an
// idle NSSA border router from the translator election.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// nssaBorderTopology returns an ABR with one active interface in the backbone and one in
// area 0.0.0.1 of the given kind.
func nssaBorderTopology(kind string) func() []InterfaceInfo {
	return func() []InterfaceInfo {
		backbone := originTopology()[0]
		attached := originTopology()[0]
		attached.Name = "eth9"
		attached.AreaID = area("0.0.0.1")
		attached.AreaType = kind
		attached.Address = ip4("10.9.0.1")
		attached.Neighbors = []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: naddr4("10.9.0.3"), State: NeighborStateFull}}
		attached.BDR = rid("3.3.3.3")
		return []InterfaceInfo{backbone, attached}
	}
}

// TestRFC3101NSSABorderRouterSetsEBit originates the router-LSAs of an ABR that originates
// no external LSA, once attached to an NSSA and once to a normal area.
func TestRFC3101NSSABorderRouterSetsEBit(t *testing.T) {
	router := rid("1.1.1.1")

	// RFC requirement: RFC3101-3.1-1 positive -- an ABR attached to an NSSA sets the E-bit
	// in its backbone router-LSA with no Type-5 or Type-7 originated and no translation.
	nssaDB := newTestDB(&fakeClock{now: time.Unix(0, 0)})
	nssaDB.SetTopology(nssaBorderTopology(types.AreaTypeNSSA))
	nssaDB.OriginateFromTopology(router, false)
	if nssaDB.SelfIsASBR(router) {
		t.Fatalf("fixture error: the NSSA border router originates an external LSA")
	}
	if !routerLSAEBit(t, nssaDB, router) {
		t.Fatalf("NSSA border router left the E-bit clear in the backbone; RFC 3101 Section 3.1 requires it")
	}

	// RFC requirement: RFC3101-3.1-1 negative -- an ABR attached to no NSSA, originating no
	// external LSA, is not an NSSA border router and leaves the E-bit clear.
	normalDB := newTestDB(&fakeClock{now: time.Unix(0, 0)})
	normalDB.SetTopology(nssaBorderTopology(types.AreaTypeNormal))
	normalDB.OriginateFromTopology(router, false)
	if routerLSAEBit(t, normalDB, router) {
		t.Fatalf("ABR with no NSSA and no external set the E-bit")
	}
}
