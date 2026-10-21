// VALIDATES: spec-ospf-ext-1 AC-11/A-5 -- a received Type-11 opaque LSA is delivered to
// its consumer with Reachable=false when the originating router is unreachable in the
// route table and Reachable=true when it is reachable (RFC 5250 §5); Type 9/10 are always
// reachable.
// PREVENTS: a consumer using a stale Type-11 opaque LSA from an originator that has become
// unreachable.
// RFC naming: untagged -- the reachability seam is stubbed, so this proves deliverOpaque honors it; the RFC 5250 sec 5 lookup and discontinue are tagged in rfc5250_asbr_lookup_test.go and rfc5250_discontinue_test.go.
package ospf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOpaqueType11UnreachableOriginatorNotUsable(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	eng := newEngine(transport.New(&fakeBackend{}))
	// Inject the §5 reachability seam: only 9.9.9.9 is reachable.
	reachable := mustRouterID(t, "9.9.9.9")
	eng.opaqueReachableFn = func(id types.RouterID) bool { return id == reachable }

	var got []opaqueReceived
	if err := registerOpaqueConsumer(4, OpaqueScopeAS, nil, func(r opaqueReceived) { got = append(got, r) }); err != nil {
		t.Fatalf("register: %v", err)
	}

	deliver := func(scope types.LSType, adv types.RouterID) {
		eng.deliverOpaque(ospflsdb.OpaqueDelivery{
			Scope:             scope,
			Area:              mustBackboneArea(t),
			AdvertisingRouter: adv,
			OpaqueType:        4,
			OpaqueID:          0x01,
			Body:              []byte{1, 2, 3, 4},
		})
	}

	// Type 11 from an unreachable originator -> delivered Reachable=false (§5).
	deliver(types.LSTypeOpaqueAS, mustRouterID(t, "2.2.2.2"))
	// The reachability seam is stubbed here, so this proves deliverOpaque honors it, not the
	// RFC 5250 sec 5 routing-table lookup: that is TestRFC5250Type11LooksUpOriginatorRoutingEntry.
	if len(got) != 1 || got[0].Reachable {
		t.Fatalf("unreachable Type-11 originator: got %+v, want one delivery with Reachable=false", got)
	}

	// Type 11 from a reachable originator -> Reachable=true.
	deliver(types.LSTypeOpaqueAS, reachable)
	// Reachability is read from the live seam on each delivery, never cached. Discontinuing the
	// LSAs already held (RFC 5250 sec 5) is TestRFC5250DiscontinueOpaqueFromUnreachableOriginator.
	if len(got) != 2 || !got[1].Reachable {
		t.Fatalf("reachable Type-11 originator: got %+v, want Reachable=true", got)
	}
}

func TestOpaqueType10AlwaysReachable(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	eng := newEngine(transport.New(&fakeBackend{}))
	// Reachability seam reports everything unreachable; Type 10 must ignore it (§5 applies
	// only to AS-wide Type 11).
	eng.opaqueReachableFn = func(types.RouterID) bool { return false }

	var got []opaqueReceived
	if err := registerOpaqueConsumer(10, OpaqueScopeArea, nil, func(r opaqueReceived) { got = append(got, r) }); err != nil {
		t.Fatalf("register: %v", err)
	}
	eng.deliverOpaque(ospflsdb.OpaqueDelivery{
		Scope:             types.LSTypeOpaqueArea,
		Area:              mustBackboneArea(t),
		AdvertisingRouter: mustRouterID(t, "2.2.2.2"),
		OpaqueType:        10,
		OpaqueID:          0x02,
		Body:              []byte{9},
	})
	if len(got) != 1 || !got[0].Reachable {
		t.Fatalf("Type-10 opaque should always be Reachable=true: got %+v", got)
	}
}
