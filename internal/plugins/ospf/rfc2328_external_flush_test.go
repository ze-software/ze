// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- withdrawing a redistributed route.
// Related: redist_wiring.go -- WithdrawExternal; lsdb/origination.go -- PurgeExternal.
//
// VALIDATES: RFC 2328 Section 12.4.4, "If a router advertises an AS-external-LSA for a
// destination which then becomes unreachable, the router must then flush the LSA from the
// routing domain by setting its age to MaxAge and reflooding (see Section 14.1).", from the
// redistribution entry point to the LS Update an adjacent neighbor receives.
// PREVENTS: a withdrawn external route whose AS-external-LSA stays in the domain until it
// ages out, and a withdrawal that flushes the LSAs of destinations still reachable.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// externalFlushKey is the AS-external-LSA key 1.1.1.1 originates for prefix.
func externalFlushKey(prefix string) types.LSAKey {
	pfx := netip.MustParsePrefix(prefix)
	return types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(pfx.Addr().As4()), AdvertisingRouter: types.RouterID{1, 1, 1, 1}}
}

// externalFlushEngine is an engine with 2.2.2.2 adjacent in Exchange on eth0 and two
// redistributed externals, 10.6.0.0/16 and 10.8.0.0/16, then 10.6.0.0/16 withdrawn.
func externalFlushEngine(t *testing.T) (*engine, *lsaSends) {
	t.Helper()
	eng, _, sends, dispatch := activeNeighborEngine(t)
	bringToExchange(t, eng, dispatch, packet.Header{RouterID: mustRouterID(t, "2.2.2.2"), AreaID: types.BackboneArea})
	for _, prefix := range []string{"10.6.0.0/16", "10.8.0.0/16"} {
		if err := eng.InjectExternal(netip.MustParsePrefix(prefix), "connected", defaultExternal(0)); err != nil {
			t.Fatalf("InjectExternal %s: %v", prefix, err)
		}
		if sends.carried(externalFlushKey(prefix)) == 0 {
			t.Fatalf("setup: the AS-external-LSA for %s was not flooded to the adjacent neighbor", prefix)
		}
	}
	removed, err := eng.WithdrawExternal(netip.MustParsePrefix("10.6.0.0/16"))
	if err != nil {
		t.Fatalf("WithdrawExternal: %v", err)
	}
	if !removed {
		t.Fatal("WithdrawExternal reported nothing removed for an advertised destination")
	}
	return eng, sends
}

// RFC requirement: RFC2328-12.4.4-1 positive -- when the redistributed destination 10.6.0.0/16 is withdrawn, the AS-external-LSA this router advertised for it is flushed: the database instance has LS age MaxAge and an instance with LS age MaxAge is reflooded to the neighbor 2.2.2.2 in Exchange (WithdrawExternal, PurgeExternal).
func TestRFC2328UnreachableExternalFlushed(t *testing.T) {
	eng, sends := externalFlushEngine(t)
	key := externalFlushKey("10.6.0.0/16")
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatal("the withdrawn destination's AS-external-LSA left the database before the neighbor acknowledged the flush")
	}
	if !lsa.Header.Age.IsMaxAge() {
		t.Fatalf("the withdrawn destination's AS-external-LSA kept LS age %d, want MaxAge", lsa.Header.Age)
	}
	if n := sends.carriedMaxAge(key); n == 0 {
		t.Fatal("no MaxAge instance of the withdrawn destination's AS-external-LSA was reflooded")
	}
}

// RFC requirement: RFC2328-12.4.4-1 negative -- the flush is confined to the destination that became unreachable: after 10.6.0.0/16 is withdrawn, the AS-external-LSA for the still-redistributed 10.8.0.0/16 stays in the database below MaxAge and no MaxAge instance of it is flooded (WithdrawExternal, PurgeExternal).
func TestRFC2328ReachableExternalNotFlushed(t *testing.T) {
	eng, sends := externalFlushEngine(t)
	key := externalFlushKey("10.8.0.0/16")
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatal("the AS-external-LSA for the still-reachable 10.8.0.0/16 left the database")
	}
	if lsa.Header.Age.IsMaxAge() {
		t.Fatal("the AS-external-LSA for the still-reachable 10.8.0.0/16 was aged to MaxAge")
	}
	if n := sends.carriedMaxAge(key); n != 0 {
		t.Fatalf("%d MaxAge instances of the still-reachable 10.8.0.0/16 were flooded", n)
	}
}
