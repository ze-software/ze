// VALIDATES: spec-ospf-10 Type 5 AS-External-LSA origination -- OriginateExternal
// builds the Type 5 in the AS-wide store with mask/E-bit/metric/forwarding-address/
// tag; PurgeExternal MaxAge-purges it; selfOriginatesExternal drives the ASBR E-bit.
// PREVENTS: regressions where a redistributed route originates no Type 5, an ASBR
// keeps the E-bit after its last external is withdrawn, or the body fields are lost.
package lsdb

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func routerLSAEBit(t *testing.T, db *LSDB, router types.RouterID) bool {
	t.Helper()
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router), AdvertisingRouter: router}
	lsa, ok := db.LookupLSA(area("0.0.0.0"), key)
	if !ok {
		t.Fatalf("router LSA not found")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("decode router: %v", err)
	}
	return body.Flags&packet.RouterFlagE != 0
}

// TestOSPFASBRBitFromExternal proves the Router-LSA E-bit (ASBR status) tracks the
// presence of self-originated Type 5 LSAs through OriginateFromTopology (AC-6).
func TestOSPFASBRBitFromExternal(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(originTopology)
	router := rid("1.1.1.1")

	db.OriginateFromTopology(router, false)
	if routerLSAEBit(t, db, router) {
		t.Fatalf("E-bit set with no external originated")
	}

	_, _, _ = db.OriginateExternal(router, ip4("10.50.0.0"), ip4("255.255.255.0"), types.OptionE, true, 20, ip4("0.0.0.0"), 0)
	clock.now = clock.now.Add(6 * time.Second) // past MinLSInterval (5s)
	db.OriginateFromTopology(router, false)
	if !routerLSAEBit(t, db, router) {
		t.Fatalf("E-bit not set after external originated (AC-6 set)")
	}

	db.PurgeExternal(router, ip4("10.50.0.0"))
	clock.now = clock.now.Add(6 * time.Second)
	db.OriginateFromTopology(router, false)
	if routerLSAEBit(t, db, router) {
		t.Fatalf("E-bit not cleared after last external withdrawn (AC-6 clear)")
	}
}

// TestOSPFASBRBitFromNSSAType7 proves the Router-LSA E-bit (ASBR status) is set when this
// router originates a Type 7 NSSA-LSA -- not only a Type 5 -- and clears when the last Type 7 is
// withdrawn. A Type 7 originator IS an AS boundary router (RFC 2328 sec 12.4.1); without the
// E-bit a receiver rejects the Type 7 route ("originating router is not an ASBR"). Regresses the
// unified, AF-agnostic SelfIsASBR self-index check that drives the E-bit on both the OSPFv2 and
// OSPFv3 origination paths.
func TestOSPFASBRBitFromNSSAType7(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTopology(originTopology)
	router := rid("1.1.1.1")
	nssa := area("0.0.0.1")

	db.OriginateFromTopology(router, false)
	if db.SelfIsASBR(router) {
		t.Fatalf("SelfIsASBR true with nothing originated")
	}
	// RFC requirement: RFC3101-3.1-1 negative -- with no external/Type-7 originated, the
	// Router-LSA in the (non-stub) backbone area carries the E-bit clear.
	if routerLSAEBit(t, db, router) {
		t.Fatalf("E-bit set with no Type 5 or Type 7 originated")
	}

	if _, ok := db.OriginateNSSA(nssa, router, ip4("10.70.0.0"), ip4("255.255.0.0"), false, 25, ip4("10.0.0.9"), 0, true); !ok {
		t.Fatalf("OriginateNSSA returned false")
	}
	if !db.SelfIsASBR(router) {
		t.Fatalf("SelfIsASBR false after a Type 7 NSSA-LSA was originated")
	}
	clock.now = clock.now.Add(6 * time.Second) // past MinLSInterval (5s)
	db.OriginateFromTopology(router, false)
	// RFC requirement: RFC3101-3.1-1 positive -- once this NSSA border router originates a
	// Type-7, its Router-LSA in the (non-stub) backbone area sets the E-bit.
	if !routerLSAEBit(t, db, router) {
		t.Fatalf("E-bit not set after Type 7 originated (Type-7 originator is an ASBR)")
	}

	if !db.PurgeNSSA(nssa, router, ip4("10.70.0.0")) {
		t.Fatalf("PurgeNSSA returned false")
	}
	if db.SelfIsASBR(router) {
		t.Fatalf("SelfIsASBR true after the last Type 7 was withdrawn")
	}
	clock.now = clock.now.Add(6 * time.Second)
	db.OriginateFromTopology(router, false)
	if routerLSAEBit(t, db, router) {
		t.Fatalf("E-bit not cleared after last Type 7 withdrawn")
	}
}

func TestOSPFOriginateExternal(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	router := rid("1.1.1.1")

	_, ok, err := db.OriginateExternal(router, ip4("10.50.0.0"), ip4("255.255.255.0"), types.OptionE, true, 20, ip4("0.0.0.0"), 99)
	if err != nil {
		t.Fatalf("OriginateExternal error: %v", err)
	}
	if !ok {
		t.Fatalf("OriginateExternal returned false")
	}
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(ip4("10.50.0.0")), AdvertisingRouter: router}
	lsa, ok := db.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatalf("external LSA not found in AS-wide store")
	}
	body, err := lsa.DecodeExternal()
	if err != nil {
		t.Fatalf("decode external: %v", err)
	}
	if body.NetworkMask != ip4("255.255.255.0") {
		t.Errorf("mask = %v, want 255.255.255.0", body.NetworkMask)
	}
	if !body.ExternalType2 {
		t.Errorf("expected E2 (metric type 2) bit set")
	}
	if body.Metric != 20 {
		t.Errorf("metric = %d, want 20", body.Metric)
	}
	if body.ExternalRouteTag != 99 {
		t.Errorf("route tag = %d, want 99", body.ExternalRouteTag)
	}
	if !db.selfOriginatesExternal(router) {
		t.Errorf("selfOriginatesExternal = false, want true (ASBR after origination)")
	}
}

func TestOSPFPurgeExternal(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	router := rid("1.1.1.1")

	_, _, _ = db.OriginateExternal(router, ip4("10.60.0.0"), ip4("255.255.0.0"), types.OptionE, false, 30, ip4("0.0.0.0"), 0)
	clock.now = clock.now.Add(10 * time.Second)

	if !db.PurgeExternal(router, ip4("10.60.0.0")) {
		t.Fatalf("PurgeExternal returned false")
	}
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(ip4("10.60.0.0")), AdvertisingRouter: router}
	lsa, ok := db.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatalf("purged LSA should still be present at MaxAge until acked")
	}
	if !lsa.Header.Age.IsMaxAge() {
		t.Errorf("expected MaxAge (3600), got %d", lsa.Header.Age.Age())
	}
	if db.selfOriginatesExternal(router) {
		t.Errorf("selfOriginatesExternal = true after purge, want false (ASBR cleared, AC-6)")
	}
}

// TestOSPFOriginateExternalStoreFull pins the independent-review finding: when the
// AS-external store is at capacity, OriginateExternal MUST surface ErrExternalStoreFull
// (not silently drop) so the redistribution consumer logs the failure and does NOT
// count an uninstalled route as injected.
func TestOSPFOriginateExternalStoreFull(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	// Fill the AS-external store to capacity directly: distinct keys, no per-insert sort,
	// so the test stays fast (16384 inserts via OriginateExternal would be O(n^2 log n)).
	filler := rid("9.9.9.9")
	for i := range MaxASExternalLSAs {
		key := types.LSAKey{
			Type:              types.LSTypeASExternal,
			LinkStateID:       types.LinkStateID([4]byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)}),
			AdvertisingRouter: filler,
		}
		db.asExternal.entries[key] = &Entry{}
	}
	// A fresh network from a different router is a new insert -> hits the capacity limit.
	_, ok, err := db.OriginateExternal(rid("1.1.1.1"), ip4("10.99.0.0"), ip4("255.255.0.0"), types.OptionE, true, 20, ip4("0.0.0.0"), 0)
	if ok {
		t.Fatalf("OriginateExternal reported success with a full store")
	}
	if !errors.Is(err, ErrExternalStoreFull) {
		t.Fatalf("OriginateExternal err = %v, want ErrExternalStoreFull", err)
	}
}

func TestOSPFOriginateExternalE1(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	router := rid("2.2.2.2")
	_, _, _ = db.OriginateExternal(router, ip4("172.16.0.0"), ip4("255.255.0.0"), types.OptionE, false, 5, ip4("10.0.0.9"), 0)
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(ip4("172.16.0.0")), AdvertisingRouter: router}
	lsa, ok := db.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatalf("external LSA not found")
	}
	body, err := lsa.DecodeExternal()
	if err != nil {
		t.Fatalf("decode external: %v", err)
	}
	if body.ExternalType2 {
		t.Errorf("expected E1 (metric type 1), got E2")
	}
	if body.ForwardingAddr != ip4("10.0.0.9") {
		t.Errorf("forwarding address = %v, want 10.0.0.9", body.ForwardingAddr)
	}
}
