package static

import (
	"net/netip"
	"testing"
)

type mockStaticBackend struct {
	applied []staticRoute
	removed []staticRoute
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
	rm := newRouteManager(mb)

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

	rm.applyRoutes(routes)

	if len(mb.applied) != 2 {
		t.Fatalf("applied %d routes, want 2", len(mb.applied))
	}
}

func TestRouteManagerRemoveOnReload(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newRouteManager(mb)

	rm.applyRoutes([]staticRoute{
		{
			Prefix: netip.MustParsePrefix("10.0.0.0/8"),
			Action: actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
		{
			Prefix: netip.MustParsePrefix("172.16.0.0/12"),
			Action: actionBlackhole,
		},
	})

	mb.applied = nil
	mb.removed = nil

	rm.applyRoutes([]staticRoute{
		{
			Prefix: netip.MustParsePrefix("10.0.0.0/8"),
			Action: actionForward,
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

func TestRouteManagerSkipsUnchangedRoutes(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newRouteManager(mb)

	route := []staticRoute{
		{
			Prefix: netip.MustParsePrefix("10.0.0.0/8"),
			Action: actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
	}

	rm.applyRoutes(route)
	if len(mb.applied) != 1 {
		t.Fatalf("initial apply: %d routes, want 1", len(mb.applied))
	}

	mb.applied = nil
	rm.applyRoutes(route)
	if len(mb.applied) != 0 {
		t.Errorf("second apply: %d routes, want 0 (unchanged)", len(mb.applied))
	}
}

func TestRouteManagerShutdownRemovesRoutes(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newRouteManager(mb)

	rm.applyRoutes([]staticRoute{
		{
			Prefix: netip.MustParsePrefix("10.0.0.0/8"),
			Action: actionForward,
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
