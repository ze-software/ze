package fibvpp

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"testing"
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
	if f.installed["10.0.0.0/24"] != "192.168.1.1" {
		t.Errorf("installed map not updated")
	}
}

func TestProcessEventDel(t *testing.T) {
	// VALIDATES: AC-2 -- withdraw action removes VPP FIB route
	// PREVENTS: route lingering after withdraw
	mock := &mockBackend{}
	f := newFibVPP(mock)
	f.installed["10.0.0.0/24"] = "192.168.1.1"

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
	f.installed["10.0.0.0/24"] = "192.168.1.1"

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[{"action":"update","prefix":"10.0.0.0/24","next-hop":"192.168.2.2","protocol":"bgp"}]}`))

	if len(mock.replaces) != 1 {
		t.Fatalf("expected 1 replace, got %d", len(mock.replaces))
	}
	if mock.replaces[0].nextHop != netip.MustParseAddr("192.168.2.2") {
		t.Errorf("wrong next-hop: %v", mock.replaces[0].nextHop)
	}
	if f.installed["10.0.0.0/24"] != "192.168.2.2" {
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
	// VALIDATES: invalid prefix logged and skipped
	// PREVENTS: panic on malformed prefix
	mock := &mockBackend{}
	f := newFibVPP(mock)

	f.processEvent(parseBatch(t, `{"family":"ipv4/unicast","changes":[{"action":"add","prefix":"not-a-prefix","next-hop":"1.1.1.1","protocol":"bgp"}]}`))

	if len(mock.adds) != 0 {
		t.Error("should not add route with invalid prefix")
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
