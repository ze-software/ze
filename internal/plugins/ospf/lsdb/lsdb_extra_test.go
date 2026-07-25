// VALIDATES: LSDB store + wiring surface that the flooding path never exercises: Delete
// (store removal + not-found), SetNSSATranslatorAreas / isNSSATranslatorArea (RFC 3101
// Nt-bit area set), SetOnChange (SPF trigger fires on install), SetSelfFlushSuppress
// (RFC 3623 graceful-restart self-flush suppression flips handleSelfReceived), and
// HigherRIDType5LSIDExists (RFC 3101 Section 3.6 OSPFv3 higher-Router-ID Type 5 test).
// PREVENTS: a Delete that reports success for a missing LSA, a translator set that keeps
// on=false entries, an SPF trigger that never fires, a GR suppression that still flushes,
// and a higher-RID test that ignores the strict Router-ID ordering.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestDeleteRemovesAndReportsMissing(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	lsa := routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)
	if !db.Install(a0, lsa) {
		t.Fatalf("install rejected")
	}
	if !db.Delete(a0, lsa.Header.Key()) {
		t.Fatalf("Delete of an installed LSA returned false")
	}
	if _, ok := db.Lookup(a0, lsa.Header.Key()); ok {
		t.Fatalf("LSA still present after Delete")
	}
	// A second Delete (now missing) reports false.
	if db.Delete(a0, lsa.Header.Key()) {
		t.Fatalf("Delete of a missing LSA returned true")
	}
	// Delete of a never-installed key reports false.
	if db.Delete(a0, types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: lsid("10.0.0.1"), AdvertisingRouter: rid("9.9.9.9")}) {
		t.Fatalf("Delete of a never-installed LSA returned true")
	}
}

func TestSetNSSATranslatorAreasFiltersOff(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a1, a2, a3 := area("0.0.0.1"), area("0.0.0.2"), area("0.0.0.3")
	db.SetNSSATranslatorAreas(map[types.AreaID]bool{a1: true, a2: false})
	if !db.isNSSATranslatorArea(a1) {
		t.Fatalf("area a1 (on=true) not recorded as a translator area")
	}
	// on=false entries are filtered out, not stored as false.
	if db.isNSSATranslatorArea(a2) {
		t.Fatalf("area a2 (on=false) wrongly recorded as a translator area")
	}
	if db.isNSSATranslatorArea(a3) {
		t.Fatalf("unset area a3 reported as a translator area")
	}
	// A replacement set clears the previous membership.
	db.SetNSSATranslatorAreas(map[types.AreaID]bool{a3: true})
	if db.isNSSATranslatorArea(a1) || !db.isNSSATranslatorArea(a3) {
		t.Fatalf("SetNSSATranslatorAreas did not replace the previous set")
	}
}

func TestSetOnChangeFiresOnInstall(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	var changed []types.AreaID
	db.SetOnChange(func(a types.AreaID) { changed = append(changed, a) })
	if !db.Install(a0, routerLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber, 10)) {
		t.Fatalf("install rejected")
	}
	if len(changed) != 1 || changed[0] != a0 {
		t.Fatalf("SetOnChange callback fired with %+v, want [%v]", changed, a0)
	}
}

func TestSetSelfFlushSuppressGatesFightBack(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock) // self router id is 1.1.1.1
	a0 := area("0.0.0.0")
	suppress := true
	db.SetSelfFlushSuppress(func() bool { return suppress })

	// A received self-originated LSA (adv == self) we hold no local record of.
	received := routerLSA(t, rid("1.1.1.1"), types.InitialSequenceNumber, 10)

	// Suppression ON (RFC 3623 sec 2): the restarting router does NOT fight back; it lets
	// the normal install path keep the received LSA, so handleSelfReceived returns false.
	if db.handleSelfReceived(a0, received) {
		t.Fatalf("handleSelfReceived returned true while GR self-flush suppression is active")
	}
	if _, ok := db.Lookup(a0, received.Header.Key()); ok {
		t.Fatalf("suppressed path installed a flush instance")
	}

	// Suppression OFF: the router fights back, re-originating the received body at MaxAge.
	suppress = false
	if !db.handleSelfReceived(a0, received) {
		t.Fatalf("handleSelfReceived returned false with suppression off")
	}
	flushed, ok := db.Lookup(a0, received.Header.Key())
	if !ok || !flushed.Age.IsMaxAge() {
		t.Fatalf("fight-back did not install a MaxAge flush: %+v ok=%v", flushed, ok)
	}
}

func TestHigherRIDType5LSIDExistsStrictOrdering(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	a0 := area("0.0.0.0")
	// Two Type 5 AS-External LSAs for the same Link State ID from different routers.
	if !db.Install(a0, externalLSA(t, rid("2.2.2.2"), types.InitialSequenceNumber)) {
		t.Fatalf("install external adv 2.2.2.2 rejected")
	}
	if !db.Install(a0, externalLSA(t, rid("5.5.5.5"), types.InitialSequenceNumber)) {
		t.Fatalf("install external adv 5.5.5.5 rejected")
	}
	target := lsid("203.0.113.0") // the fixed Link State ID externalLSA advertises

	// A router lower than 5.5.5.5 sees a strictly-higher-RID Type 5 -> true.
	if !db.HigherRIDType5LSIDExists(types.LSTypeASExternal, target, rid("3.3.3.3")) {
		t.Fatalf("HigherRIDType5LSIDExists(self 3.3.3.3) = false, want true (5.5.5.5 is higher)")
	}
	// self == the highest advertiser: strictly-greater, so no higher exists -> false.
	if db.HigherRIDType5LSIDExists(types.LSTypeASExternal, target, rid("5.5.5.5")) {
		t.Fatalf("HigherRIDType5LSIDExists(self 5.5.5.5) = true, want false (no strictly higher RID)")
	}
	// self above every advertiser -> false.
	if db.HigherRIDType5LSIDExists(types.LSTypeASExternal, target, rid("9.9.9.9")) {
		t.Fatalf("HigherRIDType5LSIDExists(self 9.9.9.9) = true, want false")
	}
	// A different Link State ID has no matching Type 5 -> false.
	if db.HigherRIDType5LSIDExists(types.LSTypeASExternal, lsid("198.51.100.0"), rid("3.3.3.3")) {
		t.Fatalf("HigherRIDType5LSIDExists(other LSID) = true, want false")
	}
	// A non-external LS type does not match the AS-External entries -> false.
	if db.HigherRIDType5LSIDExists(types.LSTypeRouter, target, rid("3.3.3.3")) {
		t.Fatalf("HigherRIDType5LSIDExists(router type) = true, want false")
	}
}
