// VALIDATES: spec-fixit-static-per-route-isolation AC-1/AC-2/AC-5 -- a route the
// backend cannot program is skipped (recorded in rm.skipped, kept out of the FIB
// and the diff baseline) while the rest of the section stays programmed and
// applyRoutes returns nil; the skipped route is re-attempted and clears once it
// programs; an unchanged re-apply reprograms nothing (routesEqual short-circuit).
// PREVENTS: a single unresolvable next-hop tearing down the whole static section
// (the 650 "Blast radius" whole-section-fail this spec replaces), and a
// regression of the diff short-circuit that would reprogram every route on an
// unrelated edit (650 R-10).

package static

import (
	"errors"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
)

// selectiveFailBackend fails applyRoute for exactly one prefix (failPrefix) while
// the failing flag is set, and succeeds for every other route. It counts calls so
// tests can assert which routes were (re)programmed. Clearing failing lets the
// skipped route apply on retry.
type selectiveFailBackend struct {
	failPrefix  netip.Prefix
	failing     bool
	applyErr    error
	applied     []staticRoute
	removed     []staticRoute
	applyCalls  int
	removeCalls int
}

func (b *selectiveFailBackend) applyRoute(r staticRoute) error {
	b.applyCalls++
	if b.failing && r.Prefix == b.failPrefix {
		return b.applyErr
	}
	b.applied = append(b.applied, r)
	return nil
}

func (b *selectiveFailBackend) removeRoute(r staticRoute) error {
	b.removeCalls++
	b.removed = append(b.removed, r)
	return nil
}

func (b *selectiveFailBackend) listRoutes() ([]installedStaticRoute, error) { return nil, nil }
func (b *selectiveFailBackend) close() error                                { return nil }

func fwd(prefix, nh string) staticRoute {
	return staticRoute{
		Prefix:   netip.MustParsePrefix(prefix),
		Action:   actionForward,
		NextHops: []nextHop{{Address: netip.MustParseAddr(nh), Weight: 1}},
	}
}

func TestApplyRoutesSkipsUnresolvableKeepsRest(t *testing.T) {
	badPfx := "203.0.113.0/24"
	badKey := routeKey{prefix: netip.MustParsePrefix(badPfx)}
	be := &selectiveFailBackend{
		failPrefix: netip.MustParsePrefix(badPfx),
		failing:    true,
		applyErr:   errors.New("network unreachable"),
	}
	rm := newRouteManager(be)

	routes := []staticRoute{
		fwd("10.0.0.0/8", "1.1.1.1"),
		fwd(badPfx, "2.2.2.2"),
		{Prefix: netip.MustParsePrefix("192.0.2.0/24"), Action: actionBlackhole},
	}

	// AC-1: a per-route failure must not be section-fatal.
	if err := rm.applyRoutes(routes); err != nil {
		t.Fatalf("applyRoutes returned %v, want nil (per-route skip must not fail the section)", err)
	}

	// The two good routes are programmed; the bad one is not.
	if len(be.applied) != 2 {
		t.Fatalf("programmed %d good routes, want 2: %+v", len(be.applied), be.applied)
	}
	for _, r := range be.applied {
		if r.Prefix == netip.MustParsePrefix(badPfx) {
			t.Errorf("bad route %s must not reach the FIB", badPfx)
		}
	}

	// The good routes are in the baseline, the bad one is not.
	if len(rm.routes) != 2 {
		t.Errorf("rm.routes has %d entries, want 2 (the good routes)", len(rm.routes))
	}
	if _, ok := rm.routes[badKey]; ok {
		t.Error("bad route must not be in rm.routes (diff baseline)")
	}

	// The bad route is recorded as skipped, with its reason.
	sk, ok := rm.skipped[badKey]
	if !ok {
		t.Fatalf("bad route %s must be recorded in rm.skipped", badPfx)
	}
	if sk.reason == "" {
		t.Error("skipped route must carry a non-empty reason")
	}
}

func TestApplyRoutesSkipPreservesDiffBaseline(t *testing.T) {
	badPfx := "203.0.113.0/24"
	badKey := routeKey{prefix: netip.MustParsePrefix(badPfx)}
	be := &selectiveFailBackend{
		failPrefix: netip.MustParsePrefix(badPfx),
		failing:    true,
		applyErr:   errors.New("network unreachable"),
	}
	rm := newRouteManager(be)

	routes := []staticRoute{
		fwd("10.0.0.0/8", "1.1.1.1"),
		fwd(badPfx, "2.2.2.2"),
	}

	// First apply: good programmed, bad skipped.
	_ = rm.applyRoutes(routes)
	if _, ok := rm.skipped[badKey]; !ok {
		t.Fatalf("setup: bad route not skipped on first apply")
	}
	appliedBefore := len(be.applied)
	callsBefore := be.applyCalls

	// Second apply of the SAME routes (A-3 diff baseline): the good route must
	// NOT be reprogrammed (routesEqual short-circuit), the bad route IS retried.
	_ = rm.applyRoutes(routes)

	if len(be.applied) != appliedBefore {
		t.Errorf("good route reprogrammed on unchanged re-apply: applied grew %d->%d", appliedBefore, len(be.applied))
	}
	if delta := be.applyCalls - callsBefore; delta != 1 {
		t.Errorf("second apply made %d applyRoute calls, want 1 (only the skipped route retried)", delta)
	}
	if _, ok := rm.skipped[badKey]; !ok {
		t.Error("bad route still failing must remain skipped after retry")
	}

	// Now the dependency appears: the backend stops failing. Re-apply retries the
	// skipped route, it programs, and clears from rm.skipped (A-2 auto-resolve).
	be.failing = false
	_ = rm.applyRoutes(routes)

	if _, ok := rm.skipped[badKey]; ok {
		t.Error("bad route must clear from rm.skipped once it programs")
	}
	if _, ok := rm.routes[badKey]; !ok {
		t.Error("resolved route must enter the diff baseline (rm.routes)")
	}
}

func TestUnrelatedInterfaceEditReprogramsNothing(t *testing.T) {
	// AC-5 / R-3: a config apply that does not change the static routes (e.g. an
	// unrelated interface edit reaching static because WantsConfig includes
	// "interface") must reprogram NO static route.
	be := &selectiveFailBackend{}
	rm := newRouteManager(be)

	routes := []staticRoute{
		fwd("10.0.0.0/8", "1.1.1.1"),
		{Prefix: netip.MustParsePrefix("192.0.2.0/24"), Action: actionBlackhole},
	}

	_ = rm.applyRoutes(routes)
	if be.applyCalls != 2 {
		t.Fatalf("setup: initial apply made %d applyRoute calls, want 2", be.applyCalls)
	}
	callsBefore := be.applyCalls
	removesBefore := be.removeCalls

	// Re-apply the identical set.
	_ = rm.applyRoutes(routes)

	if be.applyCalls != callsBefore {
		t.Errorf("identical re-apply reprogrammed routes: applyCalls %d->%d", callsBefore, be.applyCalls)
	}
	if be.removeCalls != removesBefore {
		t.Errorf("identical re-apply removed routes: removeCalls %d->%d", removesBefore, be.removeCalls)
	}
}

// TestSkippedReplaceWithdrawsOldEmittedRoute
// VALIDATES: a forward->forward replace whose replacement is skipped (new next-hop
// unresolvable) does not orphan the old route: the old FIB entry is removed and its
// redistribute announcement is withdrawn, so the FIB, the announcement, and
// `static show` all agree the prefix is UNROUTED and skipped. PREVENTS: the
// regression where the old route silently kept forwarding (blackhole) while
// `static show` reported the prefix skipped (defeats AC-3).
func TestSkippedReplaceWithdrawsOldEmittedRoute(t *testing.T) {
	bus := &staticRecordingBus{}
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	p := "10.0.0.0/8"
	pfx := netip.MustParsePrefix(p)
	pKey := routeKey{prefix: pfx}
	be := &selectiveFailBackend{}
	rm := newRouteManager(be)

	// Seed an emitted forward route P->N1 (programmed + announced).
	if err := rm.applyRoutes([]staticRoute{fwd(p, "1.1.1.1")}); err != nil {
		t.Fatalf("seed apply: %v", err)
	}
	existing := rm.routes[pKey]
	if existing == nil || !existing.emitted {
		t.Fatalf("setup: P->N1 must be programmed and emitted")
	}
	// Observe only the replace phase.
	be.applied = nil
	be.removed = nil
	be.removeCalls = 0
	bus.reset()

	// Replace with P->N2 (different next-hop) whose program FAILS.
	be.failing = true
	be.failPrefix = pfx
	be.applyErr = errors.New("network unreachable")
	if err := rm.applyRoutes([]staticRoute{fwd(p, "2.2.2.2")}); err != nil {
		t.Fatalf("replace apply returned %v, want nil (skip must not be section-fatal)", err)
	}

	// P is skipped and out of the diff baseline.
	if _, ok := rm.skipped[pKey]; !ok {
		t.Error("P must be recorded in rm.skipped")
	}
	if _, ok := rm.routes[pKey]; ok {
		t.Error("P must not be in rm.routes after a skipped replace")
	}

	// The orphaned old FIB entry (P->N1) was removed from the backend.
	removedOld := false
	for _, r := range be.removed {
		if r.Prefix == pfx {
			removedOld = true
		}
	}
	if !removedOld {
		t.Error("the replaced old route P->N1 must be removed from the FIB (orphan reclaimed)")
	}

	// The old announcement was withdrawn (ActionRemove for P).
	withdrew := false
	for _, batch := range bus.events() {
		for _, e := range batch.Entries {
			if e.Action == redistevents.ActionRemove && e.Prefix == pfx {
				withdrew = true
			}
		}
	}
	if !withdrew {
		t.Error("a redistribute ActionRemove must be emitted for the withdrawn P->N1")
	}

	// static show marks P skipped (consistent with FIB + announcement).
	rows := rm.showRoutes()
	if len(rows) != 1 || !rows[0].Skipped || rows[0].Prefix != p {
		t.Errorf("static show must mark P skipped, got %+v", rows)
	}
}

func TestShowRoutesMarksSkipped(t *testing.T) {
	badPfx := "203.0.113.0/24"
	be := &selectiveFailBackend{
		failPrefix: netip.MustParsePrefix(badPfx),
		failing:    true,
		applyErr:   errors.New("network unreachable"),
	}
	rm := newRouteManager(be)

	_ = rm.applyRoutes([]staticRoute{
		fwd("10.0.0.0/8", "1.1.1.1"),
		fwd(badPfx, "2.2.2.2"),
	})

	rows := rm.showRoutes()
	if len(rows) != 2 {
		t.Fatalf("showRoutes returned %d rows, want 2 (one good, one skipped)", len(rows))
	}

	var good, skipped *showRoute
	for i := range rows {
		switch rows[i].Prefix {
		case "10.0.0.0/8":
			good = &rows[i]
		case badPfx:
			skipped = &rows[i]
		}
	}
	if good == nil || skipped == nil {
		t.Fatalf("missing rows: good=%v skipped=%v", good, skipped)
	}
	if good.Skipped {
		t.Error("programmed route must not be marked skipped")
	}
	if !skipped.Skipped {
		t.Error("unresolvable route must be marked skipped")
	}
	if skipped.SkipReason == "" {
		t.Error("skipped route must expose a skip-reason")
	}
}
