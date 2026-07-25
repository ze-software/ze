// VALIDATES: self-origination bookkeeping and the RFC 2328 Section 14 premature-aging
// withdraw/flush helpers in origination.go: SelfExternalCount (non-purged Type 5 count),
// WithdrawSelf / WithdrawLinkSelf (MaxAge flush of an area/AS or link-local self LSA),
// FlushStaleSelfLSAs and FlushStaleSummaryLSAs (sweep of self LSAs absent from the keep
// set), and OriginateSummary (Type 3/4 Summary-LSA build) with encodedSummaryBody.
// PREVENTS: a withdraw that leaves the LSA un-aged (route lingers domain-wide), a stale
// sweep that flushes kept LSAs or misses stale ones, and a Type 4 summary carrying a mask.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestSelfExternalCountTracksPurge(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	router := rid("1.1.1.1")
	if _, ok, err := db.OriginateExternal(router, ip4("10.1.0.0"), ip4("255.255.0.0"), types.OptionE, false, 20, ip4("0.0.0.0"), 0); err != nil || !ok {
		t.Fatalf("originate external 1: ok=%v err=%v", ok, err)
	}
	if _, ok, err := db.OriginateExternal(router, ip4("10.2.0.0"), ip4("255.255.0.0"), types.OptionE, false, 30, ip4("0.0.0.0"), 0); err != nil || !ok {
		t.Fatalf("originate external 2: ok=%v err=%v", ok, err)
	}
	if n := db.SelfExternalCount(router); n != 2 {
		t.Fatalf("SelfExternalCount after two originations = %d, want 2", n)
	}
	// An external from a different router is not counted for this router.
	if n := db.SelfExternalCount(rid("2.2.2.2")); n != 0 {
		t.Fatalf("SelfExternalCount(other router) = %d, want 0", n)
	}
	// Purging one drops the count: a MaxAge (purged) self LSA is not a live external.
	if !db.PurgeExternal(router, ip4("10.1.0.0")) {
		t.Fatalf("PurgeExternal reported no self LSA")
	}
	if n := db.SelfExternalCount(router); n != 1 {
		t.Fatalf("SelfExternalCount after one purge = %d, want 1", n)
	}
}

func TestWithdrawSelfFlushesToMaxAge(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	h, ok := db.OriginateRouter(OriginInput{AreaID: a0, RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: originTopology()})
	if !ok {
		t.Fatalf("OriginateRouter false")
	}
	fh, ok := db.WithdrawSelf(a0, h.Key())
	if !ok {
		t.Fatalf("WithdrawSelf reported no flush")
	}
	if !fh.Age.IsMaxAge() {
		t.Fatalf("WithdrawSelf returned non-MaxAge header: %+v", fh)
	}
	if fh.Sequence != h.Sequence.Next() {
		t.Fatalf("WithdrawSelf sequence = %v, want %v (one past the originated seq)", fh.Sequence, h.Sequence.Next())
	}
	// The stored instance is now the MaxAge purge.
	stored, ok := db.Lookup(a0, h.Key())
	if !ok || !stored.Age.IsMaxAge() {
		t.Fatalf("stored LSA not aged to MaxAge after withdraw: %+v ok=%v", stored, ok)
	}
	// A second withdraw finds only the already-flushed instance and reports nothing to do.
	if _, ok := db.WithdrawSelf(a0, h.Key()); ok {
		t.Fatalf("second WithdrawSelf reported a flush on an already-purged LSA")
	}
}

func TestWithdrawLinkSelfFlushesToMaxAge(t *testing.T) {
	db, _, clock := opaqueOriginateDB(t)
	a0 := area("0.0.0.0")
	h, ok := db.OriginateOpaque(OpaqueOriginateInput{
		Router: rid("1.1.1.1"), OpaqueType: 1, OpaqueID: 0x03, Scope: types.LSTypeOpaqueLink,
		Interface: "eth0", Area: a0, Options: types.OptionO, Body: []byte{9, 8, 7, 6},
	})
	if !ok {
		t.Fatalf("originate link opaque failed")
	}
	// The link-scope withdraw uses the (rate-limited) own-sequence path, so advance past
	// MinLSInterval before flushing.
	clock.Add(5 * time.Second)
	fh, ok := db.WithdrawLinkSelf("eth0", h.Key())
	if !ok {
		t.Fatalf("WithdrawLinkSelf reported no flush")
	}
	if !fh.Age.IsMaxAge() {
		t.Fatalf("WithdrawLinkSelf returned non-MaxAge header: %+v", fh)
	}
	stored, ok := db.LookupLink("eth0", h.Key())
	if !ok || !stored.Age.IsMaxAge() {
		t.Fatalf("link LSA not aged to MaxAge after withdraw: %+v ok=%v", stored, ok)
	}
}

func TestFlushStaleSelfLSAsSweepsDroppedAreas(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	router := rid("1.1.1.1")
	a0, a1 := area("0.0.0.0"), area("0.0.0.1")
	h0, ok := db.OriginateRouter(OriginInput{AreaID: a0, RouterID: router, Options: types.OptionE, Interfaces: originTopology()})
	if !ok {
		t.Fatalf("originate a0 router false")
	}
	h1, ok := db.OriginateRouter(OriginInput{AreaID: a1, RouterID: router, Options: types.OptionE, Interfaces: originTopology()})
	if !ok {
		t.Fatalf("originate a1 router false")
	}
	// Keep only the a0 Router-LSA; the a1 one is stale and must be flushed.
	manage := map[types.LSType]struct{}{types.LSTypeRouter: {}}
	keep := map[SelfLSARef]struct{}{{Area: a0, Key: h0.Key()}: {}}
	if n := db.FlushStaleSelfLSAs(router, manage, keep); n != 1 {
		t.Fatalf("FlushStaleSelfLSAs flushed %d, want 1", n)
	}
	kept, ok := db.Lookup(a0, h0.Key())
	if !ok || kept.Age.IsMaxAge() {
		t.Fatalf("kept a0 Router-LSA was flushed: %+v ok=%v", kept, ok)
	}
	flushed, ok := db.Lookup(a1, h1.Key())
	if !ok || !flushed.Age.IsMaxAge() {
		t.Fatalf("stale a1 Router-LSA not flushed: %+v ok=%v", flushed, ok)
	}
}

func TestOriginateSummaryTypes(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	router := rid("1.1.1.1")
	a1 := area("0.0.0.1")

	// Type 3 Summary-LSA: describes an IP network with its mask and metric.
	h3, ok := db.OriginateSummary(a1, router, types.OptionE, types.LSTypeSummaryNetwork, lsid("10.20.0.0"), ip4("255.255.255.0"), 100)
	if !ok {
		t.Fatalf("OriginateSummary(Type 3) false")
	}
	lsa3, ok := db.LookupLSA(a1, h3.Key())
	if !ok {
		t.Fatalf("Type 3 summary not installed")
	}
	body3, err := lsa3.DecodeSummary()
	if err != nil {
		t.Fatalf("DecodeSummary Type 3: %v", err)
	}
	if body3.NetworkMask != ip4("255.255.255.0") || body3.Metric != 100 {
		t.Fatalf("Type 3 summary body = %+v, want mask 255.255.255.0 metric 100", body3)
	}

	// Type 4 Summary-LSA (ASBR reachability): RFC 2328 12.4.3 forces a zero network mask.
	h4, ok := db.OriginateSummary(a1, router, types.OptionE, types.LSTypeSummaryASBR, lsid("9.9.9.9"), ip4("255.255.255.0"), 50)
	if !ok {
		t.Fatalf("OriginateSummary(Type 4) false")
	}
	lsa4, _ := db.LookupLSA(a1, h4.Key())
	body4, err := lsa4.DecodeSummary()
	if err != nil {
		t.Fatalf("DecodeSummary Type 4: %v", err)
	}
	if body4.NetworkMask != ([4]byte{}) {
		t.Fatalf("Type 4 summary carries a non-zero mask: %+v", body4)
	}
	if body4.Metric != 50 {
		t.Fatalf("Type 4 summary metric = %d, want 50", body4.Metric)
	}

	// An unsupported LS type or a zero Router ID is rejected.
	if _, ok := db.OriginateSummary(a1, router, types.OptionE, types.LSTypeRouter, lsid("10.20.0.0"), ip4("255.255.255.0"), 100); ok {
		t.Fatalf("OriginateSummary accepted a non-summary LS type")
	}
	if _, ok := db.OriginateSummary(a1, types.RouterID{}, types.OptionE, types.LSTypeSummaryNetwork, lsid("10.20.0.0"), ip4("255.255.255.0"), 100); ok {
		t.Fatalf("OriginateSummary accepted a zero Router ID")
	}

	// FlushStaleSummaryLSAs keeps the Type 3 and flushes the un-kept Type 4.
	keep := map[types.LSAKey]struct{}{h3.Key(): {}}
	if n := db.FlushStaleSummaryLSAs(a1, router, keep); n != 1 {
		t.Fatalf("FlushStaleSummaryLSAs flushed %d, want 1", n)
	}
	kept, ok := db.Lookup(a1, h3.Key())
	if !ok || kept.Age.IsMaxAge() {
		t.Fatalf("kept Type 3 summary flushed: %+v ok=%v", kept, ok)
	}
	gone, ok := db.Lookup(a1, h4.Key())
	if !ok || !gone.Age.IsMaxAge() {
		t.Fatalf("stale Type 4 summary not flushed: %+v ok=%v", gone, ok)
	}
}
