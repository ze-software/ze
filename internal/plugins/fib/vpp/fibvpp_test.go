package fibvpp

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"testing"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	sysribevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/events"
)

// parseBatch is a test helper that builds a typed (system-rib, best-change)
// batch from a JSON literal. The bus delivers typed batches at runtime; tests
// keep using JSON literals for readability.
func parseBatch(t *testing.T, payload string) *incomingBatch {
	t.Helper()
	var b incomingBatch
	if err := json.Unmarshal([]byte(payload), &b); err != nil {
		t.Fatalf("parseBatch: %v\npayload: %s", err, payload)
	}
	return &b
}

func TestProcessEventAdd(t *testing.T) {
	// VALIDATES: AC-1 -- add action programs VPP FIB route
	// PREVENTS: route not installed on add event
	mock := &mockBackend{}
	f := newFibVPP(mock)

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[{"action":"add","prefix":"10.0.0.0/24","next-hop":"192.168.1.1","protocol":"bgp"}]}`))

	if len(mock.adds) != 1 {
		t.Fatalf("expected 1 add, got %d", len(mock.adds))
	}
	if mock.adds[0].prefix != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("wrong prefix: %v", mock.adds[0].prefix)
	}
	if mock.adds[0].nextHop != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("wrong next-hop: %v", mock.adds[0].nextHop)
	}
	if f.installed["10.0.0.0/24"].nextHop != "192.168.1.1" {
		t.Errorf("installed map not updated")
	}
}

func TestProcessEventDel(t *testing.T) {
	// VALIDATES: AC-2 -- withdraw action removes VPP FIB route
	// PREVENTS: route lingering after withdraw
	mock := &mockBackend{}
	f := newFibVPP(mock)
	f.installed["10.0.0.0/24"] = installedRoute{nextHop: "192.168.1.1"}

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[{"action":"withdraw","prefix":"10.0.0.0/24","protocol":"bgp"}]}`))

	if len(mock.dels) != 1 {
		t.Fatalf("expected 1 del, got %d", len(mock.dels))
	}
	if mock.dels[0] != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("wrong prefix: %v", mock.dels[0])
	}
	if _, ok := f.installed["10.0.0.0/24"]; ok {
		t.Error("installed map should not contain deleted prefix")
	}
}

func TestProcessEventReplace(t *testing.T) {
	// VALIDATES: AC-3 -- update action replaces VPP FIB route
	// PREVENTS: stale next-hop after update
	mock := &mockBackend{}
	f := newFibVPP(mock)
	f.installed["10.0.0.0/24"] = installedRoute{nextHop: "192.168.1.1"}

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[{"action":"update","prefix":"10.0.0.0/24","next-hop":"192.168.2.2","protocol":"bgp"}]}`))

	if len(mock.replaces) != 1 {
		t.Fatalf("expected 1 replace, got %d", len(mock.replaces))
	}
	if mock.replaces[0].nextHop != netip.MustParseAddr("192.168.2.2") {
		t.Errorf("wrong next-hop: %v", mock.replaces[0].nextHop)
	}
	if f.installed["10.0.0.0/24"].nextHop != "192.168.2.2" {
		t.Errorf("installed map not updated to new next-hop")
	}
}

func TestProcessEventBatch(t *testing.T) {
	// VALIDATES: AC-4 -- multiple changes in one event processed
	// PREVENTS: only first change processed
	mock := &mockBackend{}
	f := newFibVPP(mock)

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[
		{"action":"add","prefix":"10.0.0.0/24","next-hop":"1.1.1.1","protocol":"bgp"},
		{"action":"add","prefix":"10.0.1.0/24","next-hop":"2.2.2.2","protocol":"bgp"},
		{"action":"add","prefix":"10.0.2.0/24","next-hop":"3.3.3.3","protocol":"bgp"}
	]}`))

	if len(mock.adds) != 3 {
		t.Fatalf("expected 3 adds, got %d", len(mock.adds))
	}
	if len(f.installed) != 3 {
		t.Errorf("expected 3 installed, got %d", len(f.installed))
	}
}

func TestProcessEventReplay(t *testing.T) {
	// VALIDATES: AC-5 -- replay flag processes all routes as adds
	// PREVENTS: replay routes treated differently
	mock := &mockBackend{}
	f := newFibVPP(mock)

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","replay":true,"changes":[
		{"action":"add","prefix":"10.0.0.0/24","next-hop":"1.1.1.1","protocol":"bgp"},
		{"action":"add","prefix":"10.0.1.0/24","next-hop":"2.2.2.2","protocol":"bgp"}
	]}`))

	if len(mock.adds) != 2 {
		t.Fatalf("expected 2 adds on replay, got %d", len(mock.adds))
	}
}

func TestProcessEventIPv6(t *testing.T) {
	// VALIDATES: AC-8 -- IPv6 prefix programmed correctly
	// PREVENTS: IPv6 addresses mishandled
	mock := &mockBackend{}
	f := newFibVPP(mock)

	f.processEvent(parseBatch(t, `{"family":"ipv6/unicast","changes":[{"action":"add","prefix":"2001:db8::/32","next-hop":"fe80::1","protocol":"bgp"}]}`))

	if len(mock.adds) != 1 {
		t.Fatalf("expected 1 add, got %d", len(mock.adds))
	}
	if mock.adds[0].prefix != netip.MustParsePrefix("2001:db8::/32") {
		t.Errorf("wrong IPv6 prefix: %v", mock.adds[0].prefix)
	}
	if mock.adds[0].nextHop != netip.MustParseAddr("fe80::1") {
		t.Errorf("wrong IPv6 next-hop: %v", mock.adds[0].nextHop)
	}
}

func TestInstalledMapTracking(t *testing.T) {
	// VALIDATES: installed map correctly tracks state
	// PREVENTS: stale entries, missing entries
	mock := &mockBackend{}
	f := newFibVPP(mock)

	// Add two routes.
	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[
		{"action":"add","prefix":"10.0.0.0/24","next-hop":"1.1.1.1","protocol":"bgp"},
		{"action":"add","prefix":"10.0.1.0/24","next-hop":"2.2.2.2","protocol":"bgp"}
	]}`))
	if len(f.installed) != 2 {
		t.Fatalf("expected 2 installed, got %d", len(f.installed))
	}

	// Withdraw one.
	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[{"action":"withdraw","prefix":"10.0.0.0/24","protocol":"bgp"}]}`))
	if len(f.installed) != 1 {
		t.Fatalf("expected 1 installed after withdraw, got %d", len(f.installed))
	}
	if _, ok := f.installed["10.0.1.0/24"]; !ok {
		t.Error("remaining route should still be installed")
	}
}

func TestProcessEventInvalidPrefix(t *testing.T) {
	// VALIDATES: invalid prefix rejected at JSON parse (netip.Prefix rejects malformed values)
	// PREVENTS: malformed prefix reaching backend
	var b incomingBatch
	err := json.Unmarshal([]byte(`{"family":"ipv4/unicast","changes":[{"action":"add","prefix":"not-a-prefix","next-hop":"1.1.1.1","protocol":"bgp"}]}`), &b)
	if err == nil {
		t.Error("should fail to unmarshal invalid prefix")
	}
}

func TestProcessEventEmptyPrefix(t *testing.T) {
	// VALIDATES: empty prefix skipped
	// PREVENTS: empty prefix reaching backend
	mock := &mockBackend{}
	f := newFibVPP(mock)

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[{"action":"add","prefix":"","next-hop":"1.1.1.1","protocol":"bgp"}]}`))

	if len(mock.adds) != 0 {
		t.Error("should not add route with empty prefix")
	}
}

func TestProcessEventNilBatch(t *testing.T) {
	// VALIDATES: nil batch handled gracefully
	// PREVENTS: panic on nil payload (no-op contract for typed delivery)
	mock := &mockBackend{}
	f := newFibVPP(mock)

	f.processEvent(nil)

	if len(mock.adds) != 0 {
		t.Error("should not process anything from nil batch")
	}
}

func TestProcessEventBackendError(t *testing.T) {
	// VALIDATES: backend errors logged, processing continues
	// PREVENTS: one failed route blocking the rest
	mock := &mockBackend{err: fmt.Errorf("vpp api error")}
	f := newFibVPP(mock)

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[
		{"action":"add","prefix":"10.0.0.0/24","next-hop":"1.1.1.1","protocol":"bgp"},
		{"action":"add","prefix":"10.0.1.0/24","next-hop":"2.2.2.2","protocol":"bgp"}
	]}`))

	// Both adds attempted despite errors.
	if len(mock.adds) != 0 {
		t.Error("mock with error should not record adds")
	}
	// Installed map should not contain failed routes.
	if len(f.installed) != 0 {
		t.Error("installed map should be empty when backend fails")
	}
}

func TestFlushRoutes(t *testing.T) {
	// VALIDATES: AC-10 -- clean shutdown flushes routes
	// PREVENTS: stale routes in VPP after plugin stop
	mock := &mockBackend{}
	f := newFibVPP(mock)

	// Add routes.
	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[
		{"action":"add","prefix":"10.0.0.0/24","next-hop":"1.1.1.1","protocol":"bgp"},
		{"action":"add","prefix":"10.0.1.0/24","next-hop":"2.2.2.2","protocol":"bgp"}
	]}`))

	f.flushRoutes()

	if len(mock.dels) != 2 {
		t.Errorf("expected 2 deletes on flush, got %d", len(mock.dels))
	}
	if len(f.installed) != 0 {
		t.Error("installed map should be empty after flush")
	}
}

func TestShowInstalled(t *testing.T) {
	// VALIDATES: show command returns JSON
	// PREVENTS: empty or malformed show output
	mock := &mockBackend{}
	f := newFibVPP(mock)

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[{"action":"add","prefix":"10.0.0.0/24","next-hop":"1.1.1.1","protocol":"bgp"}]}`))

	out := f.showInstalled()
	if out == "[]" || out == "" {
		t.Errorf("expected non-empty show output, got %q", out)
	}
}

func TestToVPPPrefixIPv4(t *testing.T) {
	// VALIDATES: AC-7 -- IPv4 prefix conversion
	// PREVENTS: wrong AF or prefix length
	p := netip.MustParsePrefix("10.0.0.0/24")
	vp := toVPPPrefix(p)

	if vp.Address.Af != 0 { // ADDRESS_IP4 = 0
		t.Errorf("expected AF=0 (IPv4), got %d", vp.Address.Af)
	}
	if vp.Len != 24 {
		t.Errorf("expected prefix length 24, got %d", vp.Len)
	}
}

func TestToVPPPrefixIPv6(t *testing.T) {
	// VALIDATES: AC-8 -- IPv6 prefix conversion
	// PREVENTS: wrong AF or prefix length
	p := netip.MustParsePrefix("2001:db8::/32")
	vp := toVPPPrefix(p)

	if vp.Address.Af != 1 { // ADDRESS_IP6 = 1
		t.Errorf("expected AF=1 (IPv6), got %d", vp.Address.Af)
	}
	if vp.Len != 32 {
		t.Errorf("expected prefix length 32, got %d", vp.Len)
	}
}

// VALIDATES: AC-5 -- ECMP with multiple paths uses rich route.
func TestVPPMultiPath(t *testing.T) {
	mock := &mockBackend{}
	f := newFibVPP(mock)

	batch := &incomingBatch{
		Family: family.IPv4Unicast,
		Changes: []incomingChange{
			{
				Action:   bgptypes.RouteActionAdd,
				Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
				NextHop:  netip.MustParseAddr("192.168.1.1"),
				Protocol: "bgp",
				ECMPPaths: []sysribevents.ECMPPath{
					{NextHop: netip.MustParseAddr("192.168.1.2"), Weight: 1},
					{NextHop: netip.MustParseAddr("192.168.1.3"), Weight: 1},
				},
			},
		},
	}
	f.processEvent(batch)

	if len(mock.richAdds) != 1 {
		t.Fatalf("expected 1 rich add, got %d", len(mock.richAdds))
	}
	op := mock.richAdds[0]
	if len(op.ecmpPaths) != 2 {
		t.Errorf("expected 2 ECMP paths, got %d", len(op.ecmpPaths))
	}
	if op.prefix != netip.MustParsePrefix("10.0.0.0/24") {
		t.Errorf("wrong prefix: %v", op.prefix)
	}
	if op.nextHop != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("wrong primary next-hop: %v", op.nextHop)
	}
}

// VALIDATES: AC-9 -- TableID passed through to rich route.
func TestVPPTable(t *testing.T) {
	mock := &mockBackend{}
	f := newFibVPP(mock)

	batch := &incomingBatch{
		Family: family.IPv4Unicast,
		Changes: []incomingChange{
			{
				Action:   bgptypes.RouteActionAdd,
				Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
				NextHop:  netip.MustParseAddr("192.168.1.1"),
				Protocol: "bgp",
				TableID:  42,
				ECMPPaths: []sysribevents.ECMPPath{
					{NextHop: netip.MustParseAddr("192.168.1.2"), Weight: 1},
				},
			},
		},
	}
	f.processEvent(batch)

	if len(mock.richAdds) != 1 {
		t.Fatalf("expected 1 rich add, got %d", len(mock.richAdds))
	}
	if mock.richAdds[0].tableID != 42 {
		t.Errorf("tableID = %d, want 42", mock.richAdds[0].tableID)
	}
}

// VALIDATES: AC-6/7 -- RouteType blackhole/unreachable/prohibit uses rich route with correct type.
func TestVPPRouteType(t *testing.T) {
	tests := []struct {
		name      string
		routeType sysribevents.RouteType
	}{
		{"blackhole", sysribevents.RouteTypeBlackhole},
		{"unreachable", sysribevents.RouteTypeUnreachable},
		{"prohibit", sysribevents.RouteTypeProhibit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockBackend{}
			f := newFibVPP(mock)

			batch := &incomingBatch{
				Family: family.IPv4Unicast,
				Changes: []incomingChange{
					{
						Action:    bgptypes.RouteActionAdd,
						Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
						Protocol:  "bgp",
						RouteType: tt.routeType,
					},
				},
			}
			f.processEvent(batch)

			if len(mock.richAdds) != 1 {
				t.Fatalf("expected 1 rich add, got %d", len(mock.richAdds))
			}
			if mock.richAdds[0].routeType != tt.routeType {
				t.Errorf("routeType = %d, want %d", mock.richAdds[0].routeType, tt.routeType)
			}
		})
	}
}

// VALIDATES: AC-8 -- Metric propagated to rich route.
func TestVPPMetric(t *testing.T) {
	mock := &mockBackend{}
	f := newFibVPP(mock)

	batch := &incomingBatch{
		Family: family.IPv4Unicast,
		Changes: []incomingChange{
			{
				Action:   bgptypes.RouteActionAdd,
				Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
				NextHop:  netip.MustParseAddr("192.168.1.1"),
				Protocol: "bgp",
				Metric:   100,
			},
		},
	}
	f.processEvent(batch)

	if len(mock.richAdds) != 1 {
		t.Fatalf("expected 1 rich add, got %d", len(mock.richAdds))
	}
	if mock.richAdds[0].metric != 100 {
		t.Errorf("metric = %d, want 100", mock.richAdds[0].metric)
	}
}

// VALIDATES: AC-9 -- TableID on single-path route uses rich route.
func TestVPPTableSinglePath(t *testing.T) {
	mock := &mockBackend{}
	f := newFibVPP(mock)

	batch := &incomingBatch{
		Family: family.IPv4Unicast,
		Changes: []incomingChange{
			{
				Action:   bgptypes.RouteActionAdd,
				Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
				NextHop:  netip.MustParseAddr("192.168.1.1"),
				Protocol: "bgp",
				TableID:  99,
			},
		},
	}
	f.processEvent(batch)

	if len(mock.richAdds) != 1 {
		t.Fatalf("expected 1 rich add, got %d", len(mock.richAdds))
	}
	if mock.richAdds[0].tableID != 99 {
		t.Errorf("tableID = %d, want 99", mock.richAdds[0].tableID)
	}
	if len(mock.adds) != 0 {
		t.Error("single-path with tableID should not use legacy addRoute")
	}
}

// VALIDATES: AC-9 -- TableID on withdraw uses rich delete.
func TestVPPTableDelete(t *testing.T) {
	mock := &mockBackend{}
	f := newFibVPP(mock)
	f.installed["10.0.0.0/24"] = installedRoute{nextHop: "192.168.1.1"}

	batch := &incomingBatch{
		Family: family.IPv4Unicast,
		Changes: []incomingChange{
			{
				Action:   bgptypes.RouteActionWithdraw,
				Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
				Protocol: "bgp",
				TableID:  99,
			},
		},
	}
	f.processEvent(batch)

	if len(mock.richDels) != 1 {
		t.Fatalf("expected 1 rich del, got %d", len(mock.richDels))
	}
	if mock.richDels[0].tableID != 99 {
		t.Errorf("tableID = %d, want 99", mock.richDels[0].tableID)
	}
	if len(mock.dels) != 0 {
		t.Error("withdraw with tableID should not use legacy delRoute")
	}
}

// VALIDATES: AC-9 -- Withdraw with TableID=0 uses stored tableID from install.
func TestVPPTableDeleteStoredTableID(t *testing.T) {
	mock := &mockBackend{}
	f := newFibVPP(mock)
	f.installed["10.0.0.0/24"] = installedRoute{nextHop: "192.168.1.1", tableID: 42}

	batch := &incomingBatch{
		Family: family.IPv4Unicast,
		Changes: []incomingChange{
			{
				Action:   bgptypes.RouteActionWithdraw,
				Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
				Protocol: "bgp",
			},
		},
	}
	f.processEvent(batch)

	if len(mock.richDels) != 1 {
		t.Fatalf("expected 1 rich del, got %d", len(mock.richDels))
	}
	if mock.richDels[0].tableID != 42 {
		t.Errorf("tableID = %d, want 42 (from stored install)", mock.richDels[0].tableID)
	}
	if len(mock.dels) != 0 {
		t.Error("should use rich delete, not legacy")
	}
}

// VALIDATES: AC-6 -- RouteType update uses rich replace.
func TestVPPRouteTypeUpdate(t *testing.T) {
	mock := &mockBackend{}
	f := newFibVPP(mock)
	f.installed["10.0.0.0/24"] = installedRoute{nextHop: "192.168.1.1"}

	batch := &incomingBatch{
		Family: family.IPv4Unicast,
		Changes: []incomingChange{
			{
				Action:    bgptypes.RouteActionUpdate,
				Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
				Protocol:  "bgp",
				RouteType: sysribevents.RouteTypeBlackhole,
			},
		},
	}
	f.processEvent(batch)

	if len(mock.richReplaces) != 1 {
		t.Fatalf("expected 1 rich replace, got %d", len(mock.richReplaces))
	}
	if mock.richReplaces[0].routeType != sysribevents.RouteTypeBlackhole {
		t.Errorf("routeType = %d, want blackhole(%d)", mock.richReplaces[0].routeType, sysribevents.RouteTypeBlackhole)
	}
}

// VALIDATES: plain single-path route still uses addRoute.
func TestVPPSinglePathLegacy(t *testing.T) {
	mock := &mockBackend{}
	f := newFibVPP(mock)

	batch := &incomingBatch{
		Family: family.IPv4Unicast,
		Changes: []incomingChange{
			{
				Action:   bgptypes.RouteActionAdd,
				Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
				NextHop:  netip.MustParseAddr("192.168.1.1"),
				Protocol: "bgp",
			},
		},
	}
	f.processEvent(batch)

	if len(mock.adds) != 1 {
		t.Fatalf("expected 1 add, got %d", len(mock.adds))
	}
	if len(mock.richAdds) != 0 {
		t.Error("single-path route should not use rich route")
	}
}
