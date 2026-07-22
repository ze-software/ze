// VPP FIB apply: the install/withdraw/replace pipeline driven through scripted
// fakes (a mock api.Channel for the GoVPP backend, and the in-package mock
// backends) -- covering single-path, ECMP/rich, MPLS-labeled and SRv6 routes,
// the installed-route bookkeeping, flush-on-shutdown, and the route-count
// metrics that installs/updates/removals maintain. No real VPP daemon is used.
package fibvpp

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sysribevents "codeberg.org/thomas-mangin/ze/internal/component/sysrib/events"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

// --- shared fakes and helpers ---

// testChannel is a mock api.Channel that captures the last SendRequest message
// and returns a configurable reply via ReceiveReply.
type testChannel struct {
	lastRequest api.Message
	retval      int32 // reply retval for IPRouteAddDelReply
	sendErr     error // error returned by ReceiveReply
	closed      bool
}

var _ api.Channel = (*testChannel)(nil)

type testRequestCtx struct {
	ch *testChannel
}

func (r *testRequestCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	if reply, ok := msg.(*ip.IPRouteAddDelReply); ok {
		reply.Retval = r.ch.retval
	}
	return nil
}

func (c *testChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.lastRequest = msg
	return &testRequestCtx{ch: c}
}

func (c *testChannel) SendMultiRequest(api.Message) api.MultiRequestCtx { return nil }
func (c *testChannel) SubscribeNotification(chan api.Message, api.Message) (api.SubscriptionCtx, error) {
	return nil, nil //nolint:nilnil // test stub, never called
}
func (c *testChannel) SetReplyTimeout(time.Duration)          {}
func (c *testChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *testChannel) Close()                                 { c.closed = true }

// testGauge records Set calls for assertions.
type testGauge struct {
	mu  sync.Mutex
	val float64
}

func (g *testGauge) Set(v float64) { g.mu.Lock(); g.val = v; g.mu.Unlock() }
func (g *testGauge) Inc()          { g.mu.Lock(); g.val++; g.mu.Unlock() }
func (g *testGauge) Dec()          { g.mu.Lock(); g.val--; g.mu.Unlock() }
func (g *testGauge) Add(v float64) { g.mu.Lock(); g.val += v; g.mu.Unlock() }
func (g *testGauge) get() float64  { g.mu.Lock(); defer g.mu.Unlock(); return g.val }

// testCounter records Inc/Add calls for assertions.
type testCounter struct {
	mu  sync.Mutex
	val float64
}

func (c *testCounter) Inc()          { c.mu.Lock(); c.val++; c.mu.Unlock() }
func (c *testCounter) Add(v float64) { c.mu.Lock(); c.val += v; c.mu.Unlock() }
func (c *testCounter) get() float64  { c.mu.Lock(); defer c.mu.Unlock(); return c.val }

// fibTestRegistry returns collectable metrics for fibvpp test assertions.
type fibTestRegistry struct {
	gauges   map[string]*testGauge
	counters map[string]*testCounter
}

func newFibTestRegistry() *fibTestRegistry {
	return &fibTestRegistry{
		gauges:   make(map[string]*testGauge),
		counters: make(map[string]*testCounter),
	}
}

func (r *fibTestRegistry) Counter(name, _ string) metrics.Counter {
	c := &testCounter{}
	r.counters[name] = c
	return c
}

func (r *fibTestRegistry) Gauge(name, _ string) metrics.Gauge {
	g := &testGauge{}
	r.gauges[name] = g
	return g
}

func (r *fibTestRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec { return nil }
func (r *fibTestRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec     { return nil }
func (r *fibTestRegistry) Histogram(_, _ string, _ []float64) metrics.Histogram     { return nil }
func (r *fibTestRegistry) HistogramVec(_, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return nil
}

func makeBatch(changes ...incomingChange) *incomingBatch {
	return &incomingBatch{
		Family:  family.IPv4Unicast,
		Changes: changes,
	}
}

type mockSRv6Backend struct {
	steers    []srv6SteerOp
	delSteers []netip.Prefix
	err       error
}

type srv6SteerOp struct {
	prefix  netip.Prefix
	sid     netip.Addr
	tableID uint32
}

func (m *mockSRv6Backend) addSRv6Steer(prefix netip.Prefix, sid netip.Addr, tableID uint32) error {
	if m.err != nil {
		return m.err
	}
	m.steers = append(m.steers, srv6SteerOp{prefix, sid, tableID})
	return nil
}

func (m *mockSRv6Backend) delSRv6Steer(prefix netip.Prefix, _ uint32) error {
	if m.err != nil {
		return m.err
	}
	m.delSteers = append(m.delSteers, prefix)
	return nil
}

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

// newFibVPPWithMPLS creates a fibVPP with both IP and MPLS backends for testing.
func newFibVPPWithMPLS(ip vppBackend, mpls mplsBackend) *fibVPP {
	return &fibVPP{
		installed:     make(map[string]installedRoute),
		mplsInstalled: make(map[string]bool),
		backend:       ip,
		mplsBackend:   mpls,
	}
}

// --- govppBackend via mock channel tests ---

func TestBackendAddRoute(t *testing.T) {
	// VALIDATES: AC-1 -- addRoute sends IPRouteAddDel with IsAdd=true
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	err := b.addRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("192.168.1.1"))
	if err != nil {
		t.Fatalf("addRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T, want *ip.IPRouteAddDel", ch.lastRequest)
	}
	if !req.IsAdd {
		t.Error("IsAdd: got false, want true")
	}
	if req.Route.NPaths != 1 {
		t.Errorf("NPaths: got %d, want 1", req.Route.NPaths)
	}
	if len(req.Route.Paths) != 1 {
		t.Fatalf("Paths len: got %d, want 1", len(req.Route.Paths))
	}
}

func TestBackendDelRoute(t *testing.T) {
	// VALIDATES: AC-2 -- delRoute sends IPRouteAddDel with IsAdd=false, no paths
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	err := b.delRoute(netip.MustParsePrefix("10.0.0.0/24"))
	if err != nil {
		t.Fatalf("delRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.IsAdd {
		t.Error("IsAdd: got true, want false")
	}
	if req.Route.NPaths != 0 {
		t.Errorf("NPaths: got %d, want 0 for delete", req.Route.NPaths)
	}
	if req.Route.Paths != nil {
		t.Errorf("Paths: got %v, want nil for delete", req.Route.Paths)
	}
}

func TestBackendReplaceRoute(t *testing.T) {
	// VALIDATES: AC-3 -- replaceRoute sends IsAdd=true (VPP overwrites)
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	err := b.replaceRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("10.10.10.1"))
	if err != nil {
		t.Fatalf("replaceRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if !req.IsAdd {
		t.Error("replace should use IsAdd=true")
	}
}

func TestBackendVRFTableID(t *testing.T) {
	// VALIDATES: AC-9 -- table-id propagated to VPP request
	ch := &testChannel{}
	b := newGovppBackend(ch, 42)

	err := b.addRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("1.1.1.1"))
	if err != nil {
		t.Fatalf("addRoute: %v", err)
	}

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.Route.TableID != 42 {
		t.Errorf("TableID: got %d, want 42", req.Route.TableID)
	}
}

func TestBackendRetvalError(t *testing.T) {
	// VALIDATES: VPP retval != 0 produces error
	ch := &testChannel{retval: -1}
	b := newGovppBackend(ch, 0)

	err := b.addRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("1.1.1.1"))
	if err == nil {
		t.Fatal("expected error for retval=-1")
	}
}

func TestBackendSendError(t *testing.T) {
	// VALIDATES: GoVPP send error propagated
	ch := &testChannel{sendErr: fmt.Errorf("connection lost")}
	b := newGovppBackend(ch, 0)

	err := b.addRoute(netip.MustParsePrefix("10.0.0.0/24"), netip.MustParseAddr("1.1.1.1"))
	if err == nil {
		t.Fatal("expected error for send failure")
	}
}

func TestBackendClose(t *testing.T) {
	// VALIDATES: AC-10 -- close releases channel
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	err := b.close()
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if !ch.closed {
		t.Error("channel not closed")
	}
}

func TestBackendIPv4PrefixConversion(t *testing.T) {
	// VALIDATES: AC-7 -- IPv4 prefix in IPRouteAddDel has correct AF and length
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	_ = b.addRoute(netip.MustParsePrefix("172.16.0.0/12"), netip.MustParseAddr("10.0.0.1"))

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.Route.Prefix.Address.Af != ip_types.ADDRESS_IP4 {
		t.Errorf("Prefix AF: got %d, want ADDRESS_IP4", req.Route.Prefix.Address.Af)
	}
	if req.Route.Prefix.Len != 12 {
		t.Errorf("Prefix Len: got %d, want 12", req.Route.Prefix.Len)
	}
}

func TestBackendIPv6PrefixConversion(t *testing.T) {
	// VALIDATES: AC-8 -- IPv6 prefix in IPRouteAddDel has correct AF and length
	ch := &testChannel{}
	b := newGovppBackend(ch, 0)

	_ = b.addRoute(netip.MustParsePrefix("2001:db8::/32"), netip.MustParseAddr("fe80::1"))

	req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
	if !ok {
		t.Fatalf("lastRequest type: got %T", ch.lastRequest)
	}
	if req.Route.Prefix.Address.Af != ip_types.ADDRESS_IP6 {
		t.Errorf("Prefix AF: got %d, want ADDRESS_IP6", req.Route.Prefix.Address.Af)
	}
	if req.Route.Prefix.Len != 32 {
		t.Errorf("Prefix Len: got %d, want 32", req.Route.Prefix.Len)
	}
	if req.Route.Paths[0].Proto != fib_types.FIB_API_PATH_NH_PROTO_IP6 {
		t.Errorf("Path Proto: got %d, want FIB_API_PATH_NH_PROTO_IP6", req.Route.Paths[0].Proto)
	}
}

// --- processEvent pipeline tests ---

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

// VALIDATES: AC-5 -- ECMP with multiple paths uses rich route.
func TestVPPMultiPath(t *testing.T) {
	mock := &mockBackend{}
	f := newFibVPP(mock)

	batch := &incomingBatch{
		Family: family.IPv4Unicast,
		Changes: []incomingChange{
			{
				Action:   routeaction.Add,
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
				Action:   routeaction.Add,
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
						Action:    routeaction.Add,
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
				Action:   routeaction.Add,
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
				Action:   routeaction.Add,
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
				Action:   routeaction.Withdraw,
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
				Action:   routeaction.Withdraw,
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
				Action:    routeaction.Update,
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
				Action:   routeaction.Add,
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

// --- SRv6 steering tests ---

func TestSRv6SteerAdd(t *testing.T) {
	mock := &mockSRv6Backend{}
	f := newFibVPP(&mockBackend{})
	f.srv6Backend = mock

	sid := netip.MustParseAddr("2001:db8::1")
	prefix := netip.MustParsePrefix("10.0.0.0/24")

	f.processEvent(&sysribevents.BestChangeBatch{
		Changes: []sysribevents.BestChangeEntry{{
			Action:  routeaction.Add,
			Prefix:  prefix,
			NextHop: netip.MustParseAddr("192.0.2.1"),
			SRv6SID: sid,
		}},
	})

	if len(mock.steers) != 1 {
		t.Fatalf("expected 1 steer, got %d", len(mock.steers))
	}
	if mock.steers[0].sid != sid {
		t.Errorf("sid = %v, want %v", mock.steers[0].sid, sid)
	}
	if mock.steers[0].prefix != prefix {
		t.Errorf("prefix = %v, want %v", mock.steers[0].prefix, prefix)
	}
	if !f.srv6Installed[prefix.String()] {
		t.Error("prefix not tracked in srv6Installed")
	}
}

func TestSRv6SteerWithdraw(t *testing.T) {
	mock := &mockSRv6Backend{}
	f := newFibVPP(&mockBackend{})
	f.srv6Backend = mock

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	f.srv6Installed[prefix.String()] = true

	f.processEvent(&sysribevents.BestChangeBatch{
		Changes: []sysribevents.BestChangeEntry{{
			Action: routeaction.Withdraw,
			Prefix: prefix,
		}},
	})

	if len(mock.delSteers) != 1 {
		t.Fatalf("expected 1 del, got %d", len(mock.delSteers))
	}
	if mock.delSteers[0] != prefix {
		t.Errorf("del prefix = %v, want %v", mock.delSteers[0], prefix)
	}
	if f.srv6Installed[prefix.String()] {
		t.Error("prefix still tracked after withdraw")
	}
}

func TestSRv6ZeroSIDSkipped(t *testing.T) {
	mock := &mockSRv6Backend{}
	backend := &mockBackend{}
	f := newFibVPP(backend)
	f.srv6Backend = mock

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nh := netip.MustParseAddr("192.0.2.1")

	f.processEvent(&sysribevents.BestChangeBatch{
		Changes: []sysribevents.BestChangeEntry{{
			Action:  routeaction.Add,
			Prefix:  prefix,
			NextHop: nh,
		}},
	})

	if len(mock.steers) != 0 {
		t.Error("zero SID should not trigger SRv6 steer")
	}
	if len(backend.adds) != 1 {
		t.Error("expected plain route add")
	}
}

// --- MPLS apply tests ---

func TestMPLSPush(t *testing.T) {
	mb := &mockMPLSBackend{}
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	nh := netip.MustParseAddr("192.168.1.1")

	err := mb.addMPLSRoute(pfx, nh, []uint32{100})
	require.NoError(t, err)
	require.Len(t, mb.pushes, 1)
	assert.Equal(t, pfx, mb.pushes[0].prefix)
	assert.Equal(t, nh, mb.pushes[0].nextHop)
	assert.Equal(t, []uint32{100}, mb.pushes[0].labels)
}

func TestMPLSSwap(t *testing.T) {
	mb := &mockMPLSBackend{}
	nh := netip.MustParseAddr("192.168.1.1")

	err := mb.addMPLSSwap(100, 200, nh)
	require.NoError(t, err)
	require.Len(t, mb.swaps, 1)
	assert.Equal(t, uint32(100), mb.swaps[0].inLabel)
	assert.Equal(t, uint32(200), mb.swaps[0].outLabel)
	assert.Equal(t, nh, mb.swaps[0].nextHop)
}

func TestMPLSPop(t *testing.T) {
	mb := &mockMPLSBackend{}
	nh := netip.MustParseAddr("192.168.1.1")

	err := mb.addMPLSSwap(100, 3, nh)
	require.NoError(t, err)
	require.Len(t, mb.swaps, 1)
	assert.Equal(t, uint32(3), mb.swaps[0].outLabel)
}

func TestMPLSDelete(t *testing.T) {
	mb := &mockMPLSBackend{}
	pfx := netip.MustParsePrefix("10.0.0.0/24")

	err := mb.delMPLSRoute(pfx, nil)
	require.NoError(t, err)
	require.Len(t, mb.delPushes, 1)
	assert.Equal(t, pfx, mb.delPushes[0])

	err = mb.delMPLSSwap(100)
	require.NoError(t, err)
	require.Len(t, mb.delSwaps, 1)
	assert.Equal(t, uint32(100), mb.delSwaps[0])
}

func TestMPLSInterfaceEnable(t *testing.T) {
	mb := &mockMPLSBackend{}

	err := mb.enableMPLS(1)
	require.NoError(t, err)
	require.Len(t, mb.enables, 1)
	assert.Equal(t, uint32(1), mb.enables[0])

	err = mb.disableMPLS(1)
	require.NoError(t, err)
	require.Len(t, mb.disables, 1)
	assert.Equal(t, uint32(1), mb.disables[0])
}

func TestProcessEventWithLabels(t *testing.T) {
	ipBackend := &mockBackend{}
	mplsBack := &mockMPLSBackend{}
	f := newFibVPPWithMPLS(ipBackend, mplsBack)

	batch := &incomingBatch{
		Changes: []incomingChange{
			{
				Action:  routeaction.Add,
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.168.1.1"),
				Labels:  []uint32{100},
			},
		},
	}
	f.processEvent(batch)

	assert.Len(t, ipBackend.adds, 0, "labeled route should not go to IP backend")
	require.Len(t, mplsBack.pushes, 1, "labeled route should go to MPLS backend")
	assert.Equal(t, []uint32{100}, mplsBack.pushes[0].labels)
}

func TestProcessEventWithoutLabels(t *testing.T) {
	ipBackend := &mockBackend{}
	mplsBack := &mockMPLSBackend{}
	f := newFibVPPWithMPLS(ipBackend, mplsBack)

	batch := &incomingBatch{
		Changes: []incomingChange{
			{
				Action:  routeaction.Add,
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.168.1.1"),
			},
		},
	}
	f.processEvent(batch)

	require.Len(t, ipBackend.adds, 1, "unlabeled route should go to IP backend")
	assert.Len(t, mplsBack.pushes, 0, "unlabeled route should not go to MPLS backend")
}

func TestProcessEventWithdrawLabeled(t *testing.T) {
	ipBackend := &mockBackend{}
	mplsBack := &mockMPLSBackend{}
	f := newFibVPPWithMPLS(ipBackend, mplsBack)

	pfx := netip.MustParsePrefix("10.0.0.0/24")
	nh := netip.MustParseAddr("192.168.1.1")

	f.processEvent(&incomingBatch{
		Changes: []incomingChange{
			{Action: routeaction.Add, Prefix: pfx, NextHop: nh, Labels: []uint32{100}},
		},
	})
	require.Len(t, mplsBack.pushes, 1)

	f.processEvent(&incomingBatch{
		Changes: []incomingChange{
			{Action: routeaction.Withdraw, Prefix: pfx},
		},
	})
	require.Len(t, mplsBack.delPushes, 1)
	assert.Equal(t, pfx, mplsBack.delPushes[0])
}

// --- metrics ---

// VALIDATES: AC-9 — ze_fibvpp_routes_installed gauge present.
// VALIDATES: AC-10 — ze_fibvpp_route_installs_total counter present.
// PREVENTS: fibvpp metrics not tracking route changes.
func TestFibRouteCount(t *testing.T) {
	reg := newFibTestRegistry()
	SetMetricsRegistry(reg)
	defer fibVPPMetricsPtr.Store(nil)

	fib := newFibVPP(&mockBackend{})

	// Add two routes.
	fib.processEvent(makeBatch(
		incomingChange{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.1")},
		incomingChange{Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.0.1.0/24"), NextHop: netip.MustParseAddr("192.168.1.1")},
	))

	installed := reg.gauges["ze_fibvpp_routes_installed"]
	if installed == nil {
		t.Fatal("ze_fibvpp_routes_installed not registered")
	}
	if got := installed.get(); got != 2 {
		t.Errorf("routes_installed after 2 adds: got %v, want 2", got)
	}

	installs := reg.counters["ze_fibvpp_route_installs_total"]
	if installs == nil {
		t.Fatal("ze_fibvpp_route_installs_total not registered")
	}
	if got := installs.get(); got != 2 {
		t.Errorf("route_installs_total after 2 adds: got %v, want 2", got)
	}

	// Update one route.
	fib.processEvent(makeBatch(
		incomingChange{Action: routeaction.Update, Prefix: netip.MustParsePrefix("10.0.0.0/24"), NextHop: netip.MustParseAddr("192.168.1.2")},
	))

	updates := reg.counters["ze_fibvpp_route_updates_total"]
	if updates == nil {
		t.Fatal("ze_fibvpp_route_updates_total not registered")
	}
	if got := updates.get(); got != 1 {
		t.Errorf("route_updates_total after update: got %v, want 1", got)
	}
	// Installed count unchanged after update.
	if got := installed.get(); got != 2 {
		t.Errorf("routes_installed after update: got %v, want 2", got)
	}

	// Withdraw one route.
	fib.processEvent(makeBatch(
		incomingChange{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix("10.0.1.0/24")},
	))

	removals := reg.counters["ze_fibvpp_route_removals_total"]
	if removals == nil {
		t.Fatal("ze_fibvpp_route_removals_total not registered")
	}
	if got := removals.get(); got != 1 {
		t.Errorf("route_removals_total after withdraw: got %v, want 1", got)
	}
	if got := installed.get(); got != 1 {
		t.Errorf("routes_installed after withdraw: got %v, want 1", got)
	}

	// Flush all.
	fib.flushRoutes()
	if got := installed.get(); got != 0 {
		t.Errorf("routes_installed after flush: got %v, want 0", got)
	}
}
