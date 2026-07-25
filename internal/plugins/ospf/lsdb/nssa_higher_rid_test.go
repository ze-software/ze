// VALIDATES: spec-ospf-14 AC-16 -- RFC 3101 §3.6 HigherRIDType5Exists: a translator must not
// translate a Type 7 when an equivalent Type 5 from a strictly-higher-Router-ID translator is
// already advertised, so only the highest-Router-ID translator injects the Type 5.
// PREVENTS: duplicate Type 5 injection when two NSSA translators overlap (e.g. while a deposed
// translator's stability grace overlaps a newly-elected one).
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

	// No Type 5 present -> false.
	db := newTestDB(clock)
	if db.HigherRIDType5Exists(net, self) {
		t.Fatalf("AC-16: no Type 5 present, must report false")
	}

	// A strictly-higher Router ID advertises a Type 5 for the network -> true.
	// RFC requirement: RFC3101-3.2-2 negative -- a strictly-higher-Router-ID Type-5 for the
	// same network is detected, which gates the translator to suppress its duplicate.
	_, _, _ = db.OriginateExternal(rid("9.9.9.9"), net, mask, types.OptionE, false, 10, [4]byte{}, 0)
	if !db.HigherRIDType5Exists(net, self) {
		t.Fatalf("AC-16: a higher-Router-ID Type 5 for the network must be detected")
	}

	// A lower Router ID does not count -> self stays the elected translator.
	// RFC requirement: RFC3101-3.2-2 positive -- a lower-Router-ID Type-5 does not suppress:
	// self remains the highest-Router-ID translator and translates.
	dbLower := newTestDB(clock)
	_, _, _ = dbLower.OriginateExternal(rid("0.0.0.1"), net, mask, types.OptionE, false, 10, [4]byte{}, 0)
	if dbLower.HigherRIDType5Exists(net, self) {
		t.Fatalf("AC-16: a lower-Router-ID Type 5 must not suppress translation")
	}

	// A higher-Router-ID Type 5 for a different network does not match.
	dbOther := newTestDB(clock)
	_, _, _ = dbOther.OriginateExternal(rid("9.9.9.9"), ip4("10.61.0.0"), mask, types.OptionE, false, 10, [4]byte{}, 0)
	if dbOther.HigherRIDType5Exists(net, self) {
		t.Fatalf("AC-16: a Type 5 for a different network must not match")
	}

	// A purged higher-Router-ID Type 5 no longer suppresses (the peer withdrew it).
	db.PurgeExternal(rid("9.9.9.9"), net)
	if db.HigherRIDType5Exists(net, self) {
		t.Fatalf("AC-16: a purged higher-Router-ID Type 5 must not suppress translation")
	}
}
