// VALIDATES: RFC 3101 Section 3.2 step (2) -- HigherRIDTranslatorExternals returns the Type-5s
// that NSSA translators with a strictly higher Router ID originated, and only those, so the
// translator can test them for a functionally equivalent Type-5 (owner decision D-12).
// PREVENTS: a Type-5 from a lower-Router-ID router, from a router that is not an NSSA
// translator, or a purged Type-5 counting toward suppression.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFHigherRIDType5Exists(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	self := rid("1.1.1.1") // newTestDB's self router
	net := ip4("10.60.0.0")
	mask := ip4("255.255.0.0")
	translators := []types.RouterID{rid("9.9.9.9"), rid("0.0.0.1")}

	// No Type 5 present -> nothing to yield to.
	db := newTestDB(clock)
	if got := db.HigherRIDTranslatorExternals(types.LSTypeASExternal, translators, self); len(got) != 0 {
		t.Fatalf("no Type 5 present: got %d, want 0", len(got))
	}

	// A strictly-higher-Router-ID translator advertises a Type 5 -> returned for comparison.
	// RFC requirement: RFC3101-3.2-2 negative -- a strictly-higher-Router-ID translator's Type-5
	// is offered for the functional-equivalence test that gates the translator's duplicate.
	_, _, _ = db.OriginateExternal(rid("9.9.9.9"), net, mask, types.OptionE, false, 10, ip4("10.5.0.2"), 0)
	got := db.HigherRIDTranslatorExternals(types.LSTypeASExternal, translators, self)
	if len(got) != 1 || got[0].Header.AdvertisingRouter != rid("9.9.9.9") {
		t.Fatalf("a higher-Router-ID translator's Type 5 must be returned, got %d", len(got))
	}

	// The same higher-Router-ID Type 5 from a router that is not an NSSA translator is not.
	// RFC requirement: RFC3101-3.2-2 positive -- only "NSSA translators" count; a Type-5 from
	// any other router does not suppress translation.
	if got := db.HigherRIDTranslatorExternals(types.LSTypeASExternal, []types.RouterID{rid("0.0.0.1")}, self); len(got) != 0 {
		t.Fatalf("a non-translator's Type 5 must not be returned, got %d", len(got))
	}

	// A lower Router ID does not count -> self stays the highest translator.
	// RFC requirement: RFC3101-3.2-2 positive -- a lower-Router-ID Type-5 does not suppress:
	// self remains the highest-Router-ID translator and translates.
	dbLower := newTestDB(clock)
	_, _, _ = dbLower.OriginateExternal(rid("0.0.0.1"), net, mask, types.OptionE, false, 10, ip4("10.5.0.2"), 0)
	if got := dbLower.HigherRIDTranslatorExternals(types.LSTypeASExternal, translators, self); len(got) != 0 {
		t.Fatalf("a lower-Router-ID Type 5 must not be returned, got %d", len(got))
	}

	// A purged higher-Router-ID Type 5 no longer counts (the peer withdrew it).
	db.PurgeExternal(rid("9.9.9.9"), net)
	if got := db.HigherRIDTranslatorExternals(types.LSTypeASExternal, translators, self); len(got) != 0 {
		t.Fatalf("a purged Type 5 must not be returned, got %d", len(got))
	}
}
