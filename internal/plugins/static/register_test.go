package static

import (
	"bytes"
	"log/slog"
	"maps"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestStaticRouteRegistration(t *testing.T) {
	reg := registry.Lookup("static")
	if reg == nil {
		t.Fatal("static plugin not registered")
	}
	if reg.Name != "static" {
		t.Errorf("name = %q, want %q", reg.Name, "static")
	}
	if len(reg.ConfigRoots) != 1 || reg.ConfigRoots[0] != "static" {
		t.Errorf("config roots = %v, want [static]", reg.ConfigRoots)
	}
	if reg.YANG == "" {
		t.Error("YANG schema is empty")
	}
}

// TestStaticDeclaresOptionalInterfaceDependency
// VALIDATES: AC-8 / A-1c -- static declares "interface" as an OPTIONAL
// dependency, so TopologicalTiers orders static after the iface component when
// both are present (loading the iface backend before static resolves a next-hop
// interface), while leaving static unconstrained when no interface stanza
// exists. Asserted on the registration because the startup-tier race itself is
// not reliably observable by re-running.
// PREVENTS: a regression to the tier race where static's resolve raced
// iface.LoadBackend and failed with "no backend loaded" nondeterministically.
func TestStaticDeclaresOptionalInterfaceDependency(t *testing.T) {
	reg := registry.Lookup("static")
	if reg == nil {
		t.Fatal("static plugin not registered")
	}
	if !slices.Contains(reg.OptionalDependencies, "interface") {
		t.Errorf("optional dependencies = %v, want to include \"interface\"", reg.OptionalDependencies)
	}
	// "interface" must be OPTIONAL, not a hard dependency: a static config with
	// no interface stanza must still load.
	if slices.Contains(reg.Dependencies, "interface") {
		t.Error("\"interface\" is a hard dependency; it must be optional so no-interface configs still load")
	}
}

// TestStaticDeclaresInterfaceNexthopDoctorCheck
// VALIDATES: D-2 (b) -- the static plugin owns a doctor readiness check for the
// interface-only next-hop's runtime dependency on a loaded iface backend.
func TestStaticDeclaresInterfaceNexthopDoctorCheck(t *testing.T) {
	reg := registry.Lookup("static")
	if reg == nil {
		t.Fatal("static plugin not registered")
	}
	found := false
	for _, dc := range reg.DoctorChecks {
		for _, code := range dc.Codes {
			if code == "doctor-static-interface-nexthop-no-backend" {
				found = true
			}
		}
	}
	if !found {
		t.Error("static plugin declares no doctor check emitting doctor-static-interface-nexthop-no-backend")
	}
}

// TestStaticDeclaresRouteSkippedDoctorCheck
// VALIDATES: spec-fixit-static-per-route-isolation AC-3 -- the static plugin owns
// a doctor readiness check emitting doctor-static-route-skipped, so a route the
// backend could not program is surfaced through `ze doctor`, never a silent drop.
func TestStaticDeclaresRouteSkippedDoctorCheck(t *testing.T) {
	reg := registry.Lookup("static")
	if reg == nil {
		t.Fatal("static plugin not registered")
	}
	found := false
	for _, dc := range reg.DoctorChecks {
		for _, code := range dc.Codes {
			if code == "doctor-static-route-skipped" {
				found = true
			}
		}
	}
	if !found {
		t.Error("static plugin declares no doctor check emitting doctor-static-route-skipped")
	}
}

// TestStaticRegistersRedistributeSource
// VALIDATES: the static plugin registers "static" as a redistribute source at init(),
// so `redistribute { destination <proto> { import static } }` resolves and is visible to
// `ze config validate` (which imports plugins but does not run their engines).
// PREVENTS: regression of the bug where static emitted route events but was not a
// registered config source, so `import static` was rejected at runtime ("unknown source").
func TestStaticRegistersRedistributeSource(t *testing.T) {
	src, ok := redistribute.LookupSource("static")
	if !ok {
		t.Fatal("redistribute source \"static\" not registered (import static would be rejected)")
	}
	if src.Name != "static" {
		t.Errorf("source name = %q, want %q", src.Name, "static")
	}
	if src.Protocol != "static" {
		t.Errorf("source protocol = %q, want %q", src.Protocol, "static")
	}
}

// TestWarnIfExternal
// VALIDATES: warnIfExternal logs a warning exactly when static is NOT running
// in-process (plan/spec-fixit-verify-stage-ssot.md, plugin-boundary gate).
// PREVENTS: an operator running `plugin { external static { ... } }` with
// interface next-hops getting the misleading error "no interface backend
// loaded" for every such route. The iface component lives in the HOST process,
// so resolveNexthopIndex's iface.GetBackend() (backend_linux.go) is nil for an
// external static plugin's entire lifetime no matter how the interface backend
// is configured -- and nothing else says so.
func TestWarnIfExternal(t *testing.T) {
	t.Cleanup(func() { setLogger(slogutil.DiscardLogger()) })

	var buf bytes.Buffer
	setLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	warnIfExternal(false)
	if !strings.Contains(buf.String(), "interface") {
		t.Errorf("external static must warn about interface next-hop resolution, got: %s", buf.String())
	}

	buf.Reset()
	setLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	warnIfExternal(true)
	if buf.String() != "" {
		t.Errorf("internal static must not log the external-mode warning, got: %s", buf.String())
	}
}

// TestPendingSectionSeparatesEmptyFromAbsent
// VALIDATES: a delivered section that declares no route reports delivered, and
// so is distinguishable from a reload that delivered no static section at all.
// The method is to drive the two cases through set/take and compare the flag,
// because both cases carry the same nil route slice.
// PREVENTS: the config-apply callback treating the deletion of the static
// section as "nothing changed", which left every route programmed in the FIB
// after the operator removed the configuration that put it there.
func TestPendingSectionSeparatesEmptyFromAbsent(t *testing.T) {
	var pending pendingSection

	if _, delivered := pending.take(); delivered {
		t.Error("no section was set, take must report not delivered")
	}

	pending.set(nil)
	routes, delivered := pending.take()
	if !delivered {
		t.Error("an empty section is delivered: it is how a deletion arrives")
	}
	if len(routes) != 0 {
		t.Errorf("routes = %d, want 0", len(routes))
	}
}

// TestPendingSectionResetDropsAnAbortedTransaction
// VALIDATES: reset drops a section that an earlier transaction delivered, so a
// later apply reports not delivered. Method: set a deletion, reset as the next
// transaction's verify does, then take.
// PREVENTS: the silent FIB wipe this sequence produces without the reset. A
// commit deleting the static routes is verified, another plugin fails the same
// transaction, and no apply runs and no plugin callback is told. Static is a
// participant in every reload carrying the "interface" root, so the next
// interface-only reload reaches apply, takes the stale delivered-empty section,
// and withdraws every route the running config still declares.
func TestPendingSectionResetDropsAnAbortedTransaction(t *testing.T) {
	var pending pendingSection

	pending.set(nil) // a deletion, verified in a transaction that then aborts.
	pending.reset()  // the next transaction's verify callback.

	if _, delivered := pending.take(); delivered {
		t.Error("a section from an aborted transaction must not reach a later apply")
	}
}

// TestPendingSectionTakeClearsState
// VALIDATES: take consumes the section, so a second apply that runs without a
// verify of its own reports not delivered. Method: set once, take twice.
// PREVENTS: replaying a stale section over a later reload, which would
// withdraw or re-add routes the current configuration never mentioned.
func TestPendingSectionTakeClearsState(t *testing.T) {
	var pending pendingSection

	pending.set([]staticRoute{{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Action: actionBlackhole}})
	if routes, delivered := pending.take(); !delivered || len(routes) != 1 {
		t.Fatalf("first take: delivered = %v, routes = %d, want true and 1", delivered, len(routes))
	}

	routes, delivered := pending.take()
	if delivered {
		t.Error("second take must report not delivered")
	}
	if routes != nil {
		t.Errorf("second take returned %d routes, want none", len(routes))
	}
}

// TestStaticRollbackRestoresThePreviousPathSet
// VALIDATES: AC-16 and A-7. A transaction that applies a new static set and is
// then rolled back leaves the Loc-RIB holding the set the previous transaction
// committed, and the plugin's own record of the live set agrees with it.
// Method: commit an old set, apply a new one that changes one route's next-hop,
// drops one route and adds another, check the Loc-RIB holds the new set, run the
// journal's undo the way OnConfigRollback does, and compare the Loc-RIB with the
// old set prefix by prefix.
// PREVENTS: an aborted commit leaving the new set's paths in the Loc-RIB, where
// the FIB plugin, the single writer of main-table routes, would keep programming
// a configuration the operator never committed.
func TestStaticRollbackRestoresThePreviousPathSet(t *testing.T) {
	loc := locrib.NewRIB()
	rm := newRouteManager(&mockStaticBackend{})
	rm.setLocRIB(loc, nil)

	oldRoutes := []staticRoute{
		fwd("10.0.0.0/8", "192.0.2.1"),
		fwd("172.16.0.0/12", "192.0.2.1"),
	}
	newRoutes := []staticRoute{
		fwd("10.0.0.0/8", "192.0.2.9"),
		{Prefix: netip.MustParsePrefix("198.51.100.0/24"), Action: actionBlackhole},
	}

	var mu sync.Mutex
	var current []staticRoute
	if err := rm.applyRoutes(oldRoutes); err != nil {
		t.Fatalf("committing the previous set: %v", err)
	}
	current = oldRoutes
	before := staticLocRIBSnapshot(loc)
	wantBefore := map[netip.Prefix]string{
		netip.MustParsePrefix("10.0.0.0/8"):    "192.0.2.1",
		netip.MustParsePrefix("172.16.0.0/12"): "192.0.2.1",
	}
	if !maps.Equal(before, wantBefore) {
		t.Fatalf("the previous set reached the Loc-RIB as %v, want %v", before, wantBefore)
	}

	j, err := applyRouteSet(rm, &mu, &current, oldRoutes, newRoutes)
	if err != nil {
		t.Fatalf("applyRouteSet: %v", err)
	}
	applied := staticLocRIBSnapshot(loc)
	wantApplied := map[netip.Prefix]string{
		netip.MustParsePrefix("10.0.0.0/8"):      "192.0.2.9",
		netip.MustParsePrefix("198.51.100.0/24"): netip.Addr{}.String(),
	}
	if !maps.Equal(applied, wantApplied) {
		t.Fatalf("after apply the Loc-RIB holds %v, want the new set %v", applied, wantApplied)
	}

	if errs := j.Rollback(); len(errs) > 0 {
		t.Fatalf("rollback: %v", errs)
	}
	if after := staticLocRIBSnapshot(loc); !maps.Equal(after, wantBefore) {
		t.Errorf("after rollback the Loc-RIB holds %v, want the previous set %v", after, wantBefore)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.EqualFunc(current, oldRoutes, func(a, b staticRoute) bool { return a.Prefix == b.Prefix }) {
		t.Errorf("after rollback the plugin records %v as live, want the previous set", current)
	}
}

// VALIDATES: an apply that receives no static section owns an empty undo set,
// so the rollback the SDK runs after it leaves the committed routes in place.
// PREVENTS: a transaction that changed only another root of static (the
// interface root) and failed elsewhere replaying the undo set static kept from
// the previous, committed apply, which returned the Loc-RIB to the routes from
// two commits back (journal row rollback-forgets-partial-apply).
func TestStaticApplyWithoutSectionOwnsNoUndo(t *testing.T) {
	loc := locrib.NewRIB()
	rm := newRouteManager(&mockStaticBackend{})
	rm.setLocRIB(loc, nil)

	var mu sync.Mutex
	var current []staticRoute
	var pending pendingSection
	tx := staticTx{pending: &pending, rm: rm, mu: &mu, current: &current}

	boot := []staticRoute{fwd("10.0.0.0/8", "192.0.2.1"), fwd("172.16.0.0/12", "192.0.2.1")}
	if err := rm.applyRoutes(boot); err != nil {
		t.Fatalf("boot routes: %v", err)
	}
	current = boot

	// T1 removes 172.16.0.0/12 and commits. Its undo set stays held, as in
	// the plugin, because a commit reaches no static handler.
	pending.set([]staticRoute{fwd("10.0.0.0/8", "192.0.2.1")})
	if err := tx.apply(); err != nil {
		t.Fatalf("T1 apply: %v", err)
	}
	committed := staticLocRIBSnapshot(loc)

	// T2 delivers no static section, is accepted, and is rolled back through
	// the same holder the plugin's handlers use.
	if err := tx.apply(); err != nil {
		t.Fatalf("T2 apply: %v", err)
	}
	if err := tx.rollback(); err != nil {
		t.Fatalf("T2 rollback: %v", err)
	}
	if after := staticLocRIBSnapshot(loc); !maps.Equal(after, committed) {
		t.Errorf("after rolling back an apply with no section the Loc-RIB holds %v, want the committed %v", after, committed)
	}
}

// staticLocRIBSnapshot maps every IPv4 prefix the Loc-RIB holds to its best
// path's next-hop, so two snapshots compare the whole installed set. The
// prefixes are collected first and looked up after Iterate returns, because
// Best takes the shard lock Iterate is holding.
func staticLocRIBSnapshot(loc *locrib.RIB) map[netip.Prefix]string {
	fam := familyOf(netip.MustParsePrefix("10.0.0.0/8"))
	var prefixes []netip.Prefix
	loc.Iterate(fam, func(prefix netip.Prefix, _ locrib.PathGroup) bool {
		prefixes = append(prefixes, prefix)
		return true
	})
	snapshot := map[netip.Prefix]string{}
	for _, prefix := range prefixes {
		if best, ok := loc.Best(fam, prefix); ok {
			snapshot[prefix] = best.NextHop.String()
		}
	}
	return snapshot
}

// The sections of a commit that adds pppoe-client pppoe0 and a default route
// through it, and of one that keeps the route and drops the client.
const (
	staticDefaultViaPPPoE0 = `{"static":{"table":{"default":{"route":{"0.0.0.0/0":{"next":{"interface":{"pppoe0":{}}}}}}}}}` //nolint:lll // JSON test fixture
	interfaceWithPPPoE0    = `{"interface":{"ethernet":{"eth0":{}},"pppoe-client":{"pppoe0":{"source-interface":"eth0"}}}}`  //nolint:lll // JSON test fixture
	interfaceWithoutPPPoE0 = `{"interface":{"ethernet":{"eth0":{}}}}`
)

var defaultRoutePrefix = netip.MustParsePrefix("0.0.0.0/0")

// verifyInto runs the verify callback's body (register.go OnConfigVerify) over
// sections and leaves the result in pending, as the plugin does.
func verifyInto(t *testing.T, pending *pendingSection, sections []sdk.ConfigSection) {
	t.Helper()
	pending.reset()
	routes, found, err := verifyCandidateStatic(sections)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if found {
		pending.set(routes)
	}
}

// commitSections is one commit's delivery: the static section beside the
// interface section, as WantsConfig delivers them.
func commitSections(ifaceData string) []sdk.ConfigSection {
	return []sdk.ConfigSection{
		{Root: pluginName, Data: staticDefaultViaPPPoE0},
		{Root: "interface", Data: ifaceData},
	}
}

// newTestTx builds the transaction holder the plugin's apply and rollback
// handlers use, over a route manager whose installs reach mb.
func newTestTx(mb *mockStaticBackend) (*staticTx, *pendingSection) {
	pending := &pendingSection{}
	tx := &staticTx{pending: pending, rm: newTestRouteManager(mb), mu: &sync.Mutex{}, current: &[]staticRoute{}}
	return tx, pending
}

// TestStaticVerifyReadsCandidateInterfaceSection
// VALIDATES: AC-10. A delivery holding both the pppoe-client and a route
// through it verifies, and the pending delivery carries pppoe0 as declared
// dynamic; a delivery with no interface section declares nothing.
// Method: run the verify callback's body over each delivery and read the
// classification of the route's next-hop from the pending section.
// PREVENTS: static reading which names are dynamic from installed state, so a
// client and its route added in one commit pass or fail by apply order.
func TestStaticVerifyReadsCandidateInterfaceSection(t *testing.T) {
	var pending pendingSection

	verifyInto(t, &pending, commitSections(interfaceWithPPPoE0))
	routes, delivered := pending.take()
	if !delivered || len(routes) != 1 || len(routes[0].NextHops) != 1 {
		t.Fatalf("pending = %+v (delivered %v), want one route with one next-hop", routes, delivered)
	}
	if nh := routes[0].NextHops[0]; nh.Interface != "pppoe0" || !nh.Dynamic {
		t.Errorf("next-hop = %+v, want pppoe0 declared dynamic by the candidate interface section", nh)
	}

	verifyInto(t, &pending, []sdk.ConfigSection{{Root: pluginName, Data: staticDefaultViaPPPoE0}})
	routes, delivered = pending.take()
	if !delivered || len(routes) != 1 {
		t.Fatalf("pending = %+v (delivered %v), want the route", routes, delivered)
	}
	if routes[0].NextHops[0].Dynamic {
		t.Error("a delivery with no interface section must declare no dynamic name")
	}
}

// TestStaticApplyIgnoresApplyOrder
// VALIDATES: AC-10. Apply classifies pppoe0 from the set verified with the
// route, so with the resolver still answering for the previous config (no
// pppoe0, no backend at all here) the route waits rather than being skipped.
// Method: verify the commit that adds client and route, apply it through the
// transaction holder, and read the route manager's state.
// PREVENTS: apply asking iface's published state, so the route is skipped
// whenever static happens to apply before iface in the same transaction.
func TestStaticApplyIgnoresApplyOrder(t *testing.T) {
	mb := &mockStaticBackend{}
	tx, pending := newTestTx(mb)

	verifyInto(t, pending, commitSections(interfaceWithPPPoE0))
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}

	assertWaiting(t, tx.rm, routeKey{prefix: defaultRoutePrefix})
	if len(mb.paths) != 0 {
		t.Errorf("inserted %d Paths, want none while the only member waits", len(mb.paths))
	}
}

// assertWaiting fails unless the route at key is held as a configured route,
// not skipped, with every member inactive.
func assertWaiting(t *testing.T, rm *routeManager, key routeKey) {
	t.Helper()
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if sk, skipped := rm.skipped[key]; skipped {
		t.Fatalf("route %v skipped (%s), want it waiting", key.prefix, sk.reason)
	}
	rs, held := rm.routes[key]
	if !held {
		t.Fatalf("route %v not held by the route manager, want it waiting", key.prefix)
	}
	for _, nhs := range rs.nhStates {
		if nhs.active {
			t.Errorf("member %+v active, want it inactive while it waits", nhs.nh)
		}
	}
}

// assertSkipped fails unless the route at key is skipped with a reason that
// names the interface.
func assertSkipped(t *testing.T, rm *routeManager, key routeKey, ifName string) {
	t.Helper()
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if _, held := rm.routes[key]; held {
		t.Fatalf("route %v held as configured, want it skipped", key.prefix)
	}
	sk, skipped := rm.skipped[key]
	if !skipped {
		t.Fatalf("route %v not skipped, want the undeclared name to skip it", key.prefix)
	}
	if !strings.Contains(sk.reason, ifName) {
		t.Errorf("skip reason %q does not name %q", sk.reason, ifName)
	}
}

// TestStaticRollbackRestoresDeclaredSet
// VALIDATES: AC-12. The journal's undo restores the previous routes together
// with the previous declared set. Method, two transactions: (1) a commit that
// adds the client and its route is applied and rolled back, and neither a
// waiting route nor a skipped one remains; (2) from a committed state where
// the route waits, a commit that drops the client (so the route is skipped) is
// rolled back, and the route waits again instead of staying skipped.
// PREVENTS: an undo that re-applies the old routes against the new
// classification, leaving a route skipped, or waiting, that the committed
// config does not have. Link subscriptions arrive with device presence and
// are not checked here.
func TestStaticRollbackRestoresDeclaredSet(t *testing.T) {
	key := routeKey{prefix: defaultRoutePrefix}

	t.Run("added client and route", func(t *testing.T) {
		tx, pending := newTestTx(&mockStaticBackend{})

		verifyInto(t, pending, commitSections(interfaceWithPPPoE0))
		if err := tx.apply(); err != nil {
			t.Fatalf("apply: %v", err)
		}
		assertWaiting(t, tx.rm, key)
		if err := tx.rollback(); err != nil {
			t.Fatalf("rollback: %v", err)
		}
		tx.rm.mu.Lock()
		defer tx.rm.mu.Unlock()
		if _, held := tx.rm.routes[key]; held {
			t.Error("after rollback the route is still held, want the previous empty set")
		}
		if _, skipped := tx.rm.skipped[key]; skipped {
			t.Error("after rollback the route is skipped, want it gone")
		}
		if len(*tx.current) != 0 {
			t.Errorf("after rollback the plugin records %v as live, want none", *tx.current)
		}
	})

	t.Run("dropped client", func(t *testing.T) {
		tx, pending := newTestTx(&mockStaticBackend{})

		verifyInto(t, pending, commitSections(interfaceWithPPPoE0))
		if err := tx.apply(); err != nil {
			t.Fatalf("committed apply: %v", err)
		}
		assertWaiting(t, tx.rm, key)

		verifyInto(t, pending, commitSections(interfaceWithoutPPPoE0))
		if err := tx.apply(); err != nil {
			t.Fatalf("apply: %v", err)
		}
		assertSkipped(t, tx.rm, key, "pppoe0")

		if err := tx.rollback(); err != nil {
			t.Fatalf("rollback: %v", err)
		}
		assertWaiting(t, tx.rm, key)
		tx.mu.Lock()
		defer tx.mu.Unlock()
		live := *tx.current
		if len(live) != 1 || !live[0].NextHops[0].Dynamic {
			t.Errorf("after rollback the plugin records %+v as live, want the route with pppoe0 declared", live)
		}
	})
}

// TestStaticDeclaredRemovedFallsBackToSkip
// VALIDATES: AC-4 and AC-11. A route through a name no candidate declares, and
// that does not resolve, is skipped at apply with a reason naming it while the
// commit succeeds, exactly as before; and a commit that removes the client but
// keeps the route takes the route from waiting to that same skip, although the
// static section did not change.
// Method: apply the route with the client declared, then the same static
// section beside an interface section without the client; then give a fresh
// manager the undeclared route alone.
// PREVENTS: the unchanged static section short-circuiting the reapply
// (routesEqual), which would leave a route waiting for a device no config
// declares any more.
func TestStaticDeclaredRemovedFallsBackToSkip(t *testing.T) {
	key := routeKey{prefix: defaultRoutePrefix}
	tx, pending := newTestTx(&mockStaticBackend{})

	verifyInto(t, pending, commitSections(interfaceWithPPPoE0))
	if err := tx.apply(); err != nil {
		t.Fatalf("apply with the client: %v", err)
	}
	assertWaiting(t, tx.rm, key)

	verifyInto(t, pending, commitSections(interfaceWithoutPPPoE0))
	if err := tx.apply(); err != nil {
		t.Fatalf("apply without the client must succeed (per-route isolation): %v", err)
	}
	assertSkipped(t, tx.rm, key, "pppoe0")

	// AC-4 alone: a fresh manager given the undeclared route skips it the same way.
	fresh, freshPending := newTestTx(&mockStaticBackend{})
	verifyInto(t, freshPending, []sdk.ConfigSection{{Root: pluginName, Data: staticDefaultViaPPPoE0}})
	if err := fresh.apply(); err != nil {
		t.Fatalf("apply must succeed (per-route isolation): %v", err)
	}
	assertSkipped(t, fresh.rm, key, "pppoe0")
}
