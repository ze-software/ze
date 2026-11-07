package static

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

type mockStaticBackend struct {
	applied []staticRoute
	removed []staticRoute
	paths   []locrib.Path
	err     error
}

func (m *mockStaticBackend) applyRoute(r staticRoute) error {
	if m.err != nil {
		return m.err
	}
	m.applied = append(m.applied, r)
	return nil
}

func (m *mockStaticBackend) removeRoute(r staticRoute) error {
	if m.err != nil {
		return m.err
	}
	m.removed = append(m.removed, r)
	return nil
}

func (m *mockStaticBackend) listRoutes() ([]installedStaticRoute, error) { return nil, nil }
func (m *mockStaticBackend) close() error                                { return nil }

// The route manager has two destinations: a MAIN-table route becomes a Loc-RIB
// Path and a NAMED-table route goes straight to the data plane. The mock stands
// in for both and records into one pair of slices, so a test asking what the
// manager installed reads one recorder whichever table the route is in.
func (m *mockStaticBackend) InsertForward(_ family.Family, prefix netip.Prefix, p locrib.Path) {
	m.applied = append(m.applied, staticRoute{Prefix: prefix, Metric: p.Metric})
	m.paths = append(m.paths, p)
}

func (m *mockStaticBackend) Remove(_ family.Family, prefix netip.Prefix, _ redistevents.ProtocolID, _ uint32) {
	m.removed = append(m.removed, staticRoute{Prefix: prefix})
}

func (m *mockStaticBackend) Flush() {}

// newTestRouteManager builds a route manager whose main-table installs reach mb
// as a remote sink, the shape a FORKED static plugin has in production
// (register.go wires routeinstall.New there). In-process production wires the
// shared Loc-RIB instead, and both go through insertPathLocked, so the manager
// behavior under test is the same either way.
func newTestRouteManager(mb *mockStaticBackend) *routeManager {
	rm := newRouteManager(mb)
	rm.setLocRIB(nil, mb)
	return rm
}

func TestActiveNextHops(t *testing.T) {
	rs := &routeState{
		route: staticRoute{
			Prefix: netip.MustParsePrefix("10.0.0.0/8"),
			Action: actionForward,
		},
		done: make(chan struct{}),
		nhStates: []nhState{
			{nh: nextHop{Address: netip.MustParseAddr("1.1.1.1"), Weight: 3}, active: true},
			{nh: nextHop{Address: netip.MustParseAddr("2.2.2.2"), Weight: 1}, active: false},
			{nh: nextHop{Address: netip.MustParseAddr("3.3.3.3"), Weight: 2}, active: true},
		},
	}

	active := activeNextHops(rs)
	if len(active) != 2 {
		t.Fatalf("got %d active NHs, want 2", len(active))
	}
	if active[0].Address != netip.MustParseAddr("1.1.1.1") {
		t.Errorf("active[0] = %s, want 1.1.1.1", active[0].Address)
	}
	if active[1].Address != netip.MustParseAddr("3.3.3.3") {
		t.Errorf("active[1] = %s, want 3.3.3.3", active[1].Address)
	}
}

func TestActiveNextHopsAllDown(t *testing.T) {
	rs := &routeState{
		done: make(chan struct{}),
		nhStates: []nhState{
			{nh: nextHop{Address: netip.MustParseAddr("1.1.1.1")}, active: false},
			{nh: nextHop{Address: netip.MustParseAddr("2.2.2.2")}, active: false},
		},
	}

	active := activeNextHops(rs)
	if len(active) != 0 {
		t.Errorf("got %d active NHs, want 0 (all down)", len(active))
	}
}

func TestRouteManagerApplyRoutes(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newTestRouteManager(mb)

	routes := []staticRoute{
		{
			Prefix: netip.MustParsePrefix("10.0.0.0/8"),
			Action: actionForward,
			NextHops: []nextHop{
				{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1},
				{Address: netip.MustParseAddr("2.2.2.2"), Weight: 1},
			},
		},
		{
			Prefix: netip.MustParsePrefix("192.0.2.0/24"),
			Action: actionBlackhole,
		},
	}

	_ = rm.applyRoutes(routes)

	if len(mb.applied) != 2 {
		t.Fatalf("applied %d routes, want 2", len(mb.applied))
	}
}

func TestRouteManagerRemoveOnReload(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newTestRouteManager(mb)

	_ = rm.applyRoutes([]staticRoute{
		{
			Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
			Action:   actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
		{
			Prefix: netip.MustParsePrefix("172.16.0.0/12"),
			Action: actionBlackhole,
		},
	})

	mb.applied = nil
	mb.removed = nil

	_ = rm.applyRoutes([]staticRoute{
		{
			Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
			Action:   actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
	})

	if len(mb.removed) != 1 {
		t.Fatalf("removed %d routes, want 1", len(mb.removed))
	}
	if mb.removed[0].Prefix != netip.MustParsePrefix("172.16.0.0/12") {
		t.Errorf("removed prefix = %s, want 172.16.0.0/12", mb.removed[0].Prefix)
	}
}

// TestRouteManagerWithdrawsAllOnEmptyConfig
// VALIDATES: applying an empty route set removes every route the manager had
// programmed. Method: apply two routes, then apply none, and read the backend's
// removals. This is the second half of a static-section deletion: the callback
// decides that the deletion was delivered, and this decides what that means.
// PREVENTS: a deletion that reaches applyRoutes and still leaves the routes in
// the FIB.
func TestRouteManagerWithdrawsAllOnEmptyConfig(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newTestRouteManager(mb)

	_ = rm.applyRoutes([]staticRoute{
		{
			Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
			Action:   actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
		{
			Prefix: netip.MustParsePrefix("172.16.0.0/12"),
			Action: actionBlackhole,
		},
	})

	mb.applied = nil
	mb.removed = nil

	_ = rm.applyRoutes(nil)

	if len(mb.removed) != 2 {
		t.Fatalf("removed %d routes, want 2", len(mb.removed))
	}
	if len(rm.routes) != 0 {
		t.Errorf("route manager still holds %d routes, want 0", len(rm.routes))
	}
}

func TestRouteManagerSkipsUnchangedRoutes(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newTestRouteManager(mb)

	route := []staticRoute{
		{
			Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
			Action:   actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
	}

	_ = rm.applyRoutes(route)
	if len(mb.applied) != 1 {
		t.Fatalf("initial apply: %d routes, want 1", len(mb.applied))
	}

	mb.applied = nil
	_ = rm.applyRoutes(route)
	if len(mb.applied) != 0 {
		t.Errorf("second apply: %d routes, want 0 (unchanged)", len(mb.applied))
	}
}

func TestRouteManagerShutdownRemovesRoutes(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newTestRouteManager(mb)

	_ = rm.applyRoutes([]staticRoute{
		{
			Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
			Action:   actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
		{
			Prefix: netip.MustParsePrefix("192.0.2.0/24"),
			Action: actionBlackhole,
		},
	})

	mb.removed = nil
	rm.shutdown()

	if len(mb.removed) != 2 {
		t.Fatalf("shutdown removed %d routes, want 2", len(mb.removed))
	}
	if len(rm.routes) != 0 {
		t.Errorf("routes map has %d entries after shutdown, want 0", len(rm.routes))
	}
}

// namedOps indexes one event's named-table writes by table, failing the test
// when a table was written twice: each route owes one write per event.
func namedOps(t *testing.T, ops []*staticOp) map[uint32]*staticOp {
	t.Helper()
	byTable := make(map[uint32]*staticOp, len(ops))
	for _, op := range ops {
		if _, dup := byTable[op.route.Table]; dup {
			t.Fatalf("table %d written twice in one event: %+v", op.route.Table, ops)
		}
		byTable[op.route.Table] = op
	}
	return byTable
}

// requireNamedWrite checks one named-table write: its verb and, for an
// apply, the members it carried in order, each named by its interface, or by
// its gateway address when it names no interface.
func requireNamedWrite(t *testing.T, step string, op *staticOp, verb string, members ...string) {
	t.Helper()
	if op == nil {
		t.Fatalf("%s: a route owed a %s and its table was not written", step, verb)
	}
	if op.verb != verb {
		t.Fatalf("%s: table %d verb = %q, want %q (%+v)", step, op.route.Table, op.verb, verb, op.route)
	}
	if verb == "remove" {
		return
	}
	got := make([]string, 0, len(op.route.NextHops))
	for _, nh := range op.route.NextHops {
		if nh.Interface != "" {
			got = append(got, nh.Interface)
			continue
		}
		got = append(got, nh.Address.String())
	}
	if len(got) != len(members) {
		t.Fatalf("%s: table %d members = %v, want %v", step, op.route.Table, got, members)
	}
	for i := range got {
		if got[i] != members[i] {
			t.Fatalf("%s: table %d members = %v, want %v", step, op.route.Table, got, members)
		}
	}
}

// TestStaticNamedTableWaitingMember is AC-13 and AC-15 at the route manager:
// link events on a declared on-demand interface drive static's own backend
// for named-table routes. Method: a fake linkSource delivers the events, and a
// recording backend reports each write on a channel, so every step waits for
// the write its event owes. Two routes name pppoe0: one alone (table 100,
// AC-13) and one mixed with gateway 192.0.2.1 (table 101, AC-15). The device
// starts absent, comes up, is deleted, comes back under a new ifindex, and
// moves again with its down event lost. Then the routes are removed, which
// must end the subscription, and shutdown must return, which proves the
// watcher exited.
// PREVENTS: a waiting member that stays inactive forever; a session drop that
// leaves the named-table route installed, or withdraws the mixed route
// instead of shrinking it; a reconnect that is ignored because the member was
// already marked active; a subscription or watcher that outlives its routes.
func TestStaticNamedTableWaitingMember(t *testing.T) {
	const dev = "pppoe0"
	gateway := netip.MustParseAddr("192.0.2.1")
	waiting := nextHop{Interface: dev, Weight: 3, Dynamic: true}
	alone := staticRoute{
		Prefix:   netip.MustParsePrefix("0.0.0.0/0"),
		Table:    100,
		Action:   actionForward,
		NextHops: []nextHop{waiting},
	}
	mixed := staticRoute{
		Prefix:   netip.MustParsePrefix("0.0.0.0/0"),
		Table:    101,
		Action:   actionForward,
		NextHops: []nextHop{{Address: gateway, Weight: 1}, waiting},
	}

	rec := newOpRecorder()
	links := newFakeLinks()
	mgr := newLinkTestManager(rec, links)

	if err := mgr.applyRoutes([]staticRoute{alone, mixed}); err != nil {
		t.Fatalf("applyRoutes: %v", err)
	}
	ops := rec.next(t, 1)
	requireNamedWrite(t, "commit, device absent", ops[0], "apply", gateway.String())
	if ops[0].route.Table != mixed.Table {
		t.Fatalf("commit, device absent: wrote table %d, want only the mixed route's %d", ops[0].route.Table, mixed.Table)
	}
	rec.none(t)
	mgr.mu.Lock()
	skipped := len(mgr.skipped)
	mgr.mu.Unlock()
	if skipped != 0 {
		t.Fatalf("skipped %d routes, want none: a waiting member is never a skip", skipped)
	}

	links.send(t, dev, iface.LinkUp, 7)
	byTable := namedOps(t, rec.next(t, 2))
	requireNamedWrite(t, "device up", byTable[alone.Table], "apply", dev)
	requireNamedWrite(t, "device up", byTable[mixed.Table], "apply", gateway.String(), dev)

	links.send(t, dev, iface.LinkDown, 7)
	byTable = namedOps(t, rec.next(t, 2))
	requireNamedWrite(t, "device deleted", byTable[alone.Table], "remove")
	requireNamedWrite(t, "device deleted", byTable[mixed.Table], "apply", gateway.String())

	links.send(t, dev, iface.LinkAppeared, 9)
	links.send(t, dev, iface.LinkUp, 9)
	byTable = namedOps(t, rec.next(t, 2))
	requireNamedWrite(t, "device recreated", byTable[alone.Table], "apply", dev)
	requireNamedWrite(t, "device recreated", byTable[mixed.Table], "apply", gateway.String(), dev)

	// The down event of a second reconnect is lost (a full subscriber channel
	// keeps the newest): only the new ifindex says the device moved.
	links.send(t, dev, iface.LinkUp, 11)
	byTable = namedOps(t, rec.next(t, 2))
	requireNamedWrite(t, "device moved, down lost", byTable[alone.Table], "apply", dev)
	requireNamedWrite(t, "device moved, down lost", byTable[mixed.Table], "apply", gateway.String(), dev)
	rec.none(t)

	if err := mgr.applyRoutes(nil); err != nil {
		t.Fatalf("applyRoutes removing both routes: %v", err)
	}
	rec.next(t, 2)
	if links.subscribed(dev) {
		t.Errorf("%s still subscribed after the last route naming it was removed", dev)
	}
	if got := links.cancels(dev); got != 1 {
		t.Errorf("%s subscription ended %d times, want once", dev, got)
	}
	mgr.shutdown()
}
