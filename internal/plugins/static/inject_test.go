package static

import (
	"net/netip"
	"testing"

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
