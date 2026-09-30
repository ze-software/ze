// Design: docs/architecture/ospf/ospf-ext-7-virtual-links.md -- a transit cost change
// re-originates the backbone Router-LSA.
// Related: rfc2328_virtual_route_test.go -- virtualRouteEngine, establishVirtualRoute.
//
// VALIDATES: RFC 2328 Section 15: "When the cost of a virtual link changes, a new
// router-LSA should be originated for the backbone area."
// PREVENTS: a cost change that rewrites the stored metric without a new LSA instance.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// backboneRouterInstance returns the self-originated backbone Router-LSA as stored in the
// LSDB, so a test can compare LS sequence numbers across an origination.
func backboneRouterInstance(t *testing.T, eng *engine) packet.LSA {
	t.Helper()
	rid := eng.cfg.RouterID
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(rid), AdvertisingRouter: rid})
	if !ok {
		t.Fatal("self backbone Router-LSA missing")
	}
	return lsa
}

// RFC requirement: RFC2328-15-2 positive -- when the intra-area transit path cost of a
// Full virtual link changes (10 to 77), a NEW backbone Router-LSA is originated: its LS
// sequence number is newer than the instance before the change, and it carries the
// virtual link at the new cost 77 (onVirtualLinksResolved).
// Goal: prove a new instance, not only a rewritten stored metric. Method: establish the
// virtual adjacency, read the backbone Router-LSA, deliver the new transit result, read
// it again and compare sequence numbers.
func TestRFC2328VirtualCostChangeOriginatesNewInstance(t *testing.T) {
	eng, backend, result := virtualRouteEngine(t)
	establishVirtualRoute(t, eng, backend, result)
	before := backboneRouterInstance(t, eng)
	result.Cost = 77
	eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{result})
	after := backboneRouterInstance(t, eng)
	if !after.Header.Sequence.NewerThan(before.Header.Sequence) {
		t.Fatalf("backbone Router-LSA sequence %s after the cost change, want newer than %s", after.Header.Sequence, before.Header.Sequence)
	}
	if metric, present := virtualRouterMetric(t, eng, result.Neighbor); !present || metric != 77 {
		t.Fatalf("virtual metric = %d, present=%v, want 77", metric, present)
	}
}
