// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- RFC 5250 Type-11 originator reachability.
// Related: opaque.go -- deliverOpaque and spfRouterReachable, the production reachability seam.
// Related: rfc3623_prepare_fib_test.go -- rfc3623PendingEngine, the SPF-wired engine reused here.
//
// VALIDATES: RFC 5250 Section 5, "When processing a received type-11 Opaque LSA, the router
// MUST look up the routing table entries (potentially one per attached area) for the ASBR
// that originated the LSA", through the engine's own seam (no injected predicate): the
// verdict on a delivered Type-11 LSA follows the entries SPF computed, not the LSDB alone.
// PREVENTS: a reachability predicate that never consults the routing table, such as one that
// answers from LSDB presence or answers true for every originator.
package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestRFC5250Type11LooksUpOriginatorRoutingEntry delivers Type-11 opaque LSAs to a registered
// consumer on an engine whose SPF computer holds a point-to-point pair 10.0.0.1 - 10.0.0.2,
// before and after SPF runs, and from an originator SPF never reaches.
func TestRFC5250Type11LooksUpOriginatorRoutingEntry(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	e := rfc3623PendingEngine(t)
	var got []opaqueReceived
	if err := registerOpaqueConsumer(4, OpaqueScopeAS, nil, func(r opaqueReceived) { got = append(got, r) }); err != nil {
		t.Fatalf("register: %v", err)
	}
	deliver := func(adv types.RouterID) bool {
		t.Helper()
		before := len(got)
		e.deliverOpaque(ospflsdb.OpaqueDelivery{
			Scope: types.LSTypeOpaqueAS, Area: types.BackboneArea, AdvertisingRouter: adv,
			OpaqueType: 4, OpaqueID: 1, Body: []byte{1, 2, 3, 4},
		})
		if len(got) != before+1 {
			t.Fatalf("the Type-11 LSA from %s was not delivered", adv)
		}
		return got[before].Reachable
	}
	peer := types.RouterID{10, 0, 0, 2}

	// RFC requirement: RFC5250-5-1 negative -- the verdict comes from the routing table, not
	// from the LSDB: while the SPF run that would compute 10.0.0.2's entry is still pending,
	// a Type-11 LSA from 10.0.0.2 (whose Router-LSA is installed) is delivered not usable, and
	// after SPF has run, one from 10.0.0.9 (no Router-LSA, so no routing table entry) is too.
	if deliver(peer) {
		t.Fatal("a Type-11 LSA was delivered usable before SPF computed any entry for its originator")
	}
	e.spf.Run()
	if deliver(types.RouterID{10, 0, 0, 9}) {
		t.Fatal("a Type-11 LSA from an originator with no routing table entry was delivered usable")
	}

	// RFC requirement: RFC5250-5-1 positive -- once SPF has computed the routing table entry
	// for the originator, the router finds it: a Type-11 LSA from 10.0.0.2 is delivered usable.
	if !deliver(peer) {
		t.Fatal("a Type-11 LSA from an originator SPF reaches was delivered not usable")
	}
}
