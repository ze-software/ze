// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- RFC 5250 Type-11 originator reachability.
// Related: opaque.go -- deliverOpaque and spfASBRReachable, the production reachability seam.
// Related: rfc3623_prepare_fib_test.go -- rfc3623PendingEngine, the SPF-wired engine reused here.
//
// VALIDATES: RFC 5250 Section 5, "When processing a received type-11 Opaque LSA, the router
// MUST look up the routing table entries (potentially one per attached area) for the ASBR
// that originated the LSA", through the engine's own seam (no injected predicate): the
// verdict on a delivered Type-11 LSA follows the AS boundary router entries SPF computed
// (RFC 2328 Section 11: router entries exist only for area border and AS boundary routers).
// PREVENTS: a reachability predicate that answers from LSDB presence, from any router SPF
// reaches (an internal router has no ASBR routing table entry), or true for every originator.
package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestRFC5250Type11LooksUpOriginatorRoutingEntry delivers Type-11 opaque LSAs to a registered
// consumer on an engine whose SPF computer holds a point-to-point pair 10.0.0.1 - 10.0.0.2:
// before SPF runs, after it runs while 10.0.0.2 is an internal router (no E-bit), after
// 10.0.0.2 re-originates its Router-LSA with the E-bit set, and from an originator SPF never
// reaches.
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

	// RFC requirement: RFC5250-5-1 negative -- the verdict comes from the ASBR routing table
	// entries, not from the LSDB or from SPF reachability alone: while the SPF run is pending,
	// a Type-11 LSA from 10.0.0.2 (Router-LSA installed) is delivered not usable; after SPF has
	// run, one from 10.0.0.2 is still not usable, because 10.0.0.2 is reached but carries no
	// E-bit and so has no ASBR entry; one from 10.0.0.9 (no Router-LSA, no entry) is not usable.
	if deliver(peer) {
		t.Fatal("a Type-11 LSA was delivered usable before SPF computed any entry for its originator")
	}
	e.spf.Run()
	if deliver(peer) {
		t.Fatal("a Type-11 LSA from a reached router with no ASBR routing table entry was delivered usable")
	}
	if deliver(types.RouterID{10, 0, 0, 9}) {
		t.Fatal("a Type-11 LSA from an originator with no routing table entry was delivered usable")
	}

	// RFC requirement: RFC5250-5-1 positive -- once 10.0.0.2 announces itself an AS boundary
	// router (Router-LSA E-bit) and SPF has computed its ASBR routing table entry, the router
	// finds that entry: a Type-11 LSA from 10.0.0.2 is delivered usable.
	toSelf := packet.RouterLink{Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{10, 0, 0, 1},
		LinkData: [4]byte{10, 0, 0, 2}, Metric: 10}
	asbr := bgplsReachabilityRouter(peer, toSelf)
	asbr.Header.Sequence++
	asbr.Router.Flags = packet.RouterFlagE
	if !e.lsdb.Install(types.BackboneArea, asbr) {
		t.Fatal("install the peer's E-bit router-LSA")
	}
	e.spf.Run()
	if !deliver(peer) {
		t.Fatal("a Type-11 LSA from an originator with an ASBR routing table entry was delivered not usable")
	}
}
