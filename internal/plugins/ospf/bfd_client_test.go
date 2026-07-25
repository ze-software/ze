// VALIDATES: spec-ospf-ext-10 AC-2..AC-13 (IPv4 + shared lifecycle) -- a Full neighbor on a
// BFD-enabled interface opens exactly one single-hop session; a BFD Down/AdminDown drives the
// AF-neutral NSM down seam; Up/Init are inert; leaving Full releases the session and the
// subscriber exits; nil service degrades gracefully; distinct neighbors get distinct keys;
// reload enable/disable converges without bouncing the adjacency.
// PREVENTS: mis-wired requests, a leaked subscriber, a double-release, a session opened before
// Full or when disabled, or a BFD Up wrongly bringing the adjacency up.
package ospf

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/core/metrics"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// ---- fake BFD engine (mirrors internal/component/bgp/reactor.fakeBFDService) ----

type fakeBFDHandle struct {
	key        api.Key
	ch         chan api.StateChange
	unsubbed   atomic.Bool
	subscribed atomic.Bool
}

func (h *fakeBFDHandle) Key() api.Key { return h.key }
func (h *fakeBFDHandle) Subscribe() <-chan api.StateChange {
	h.subscribed.Store(true)
	return h.ch
}
func (h *fakeBFDHandle) Unsubscribe(<-chan api.StateChange) { h.unsubbed.Store(true) }
func (h *fakeBFDHandle) Shutdown() error                    { return nil }
func (h *fakeBFDHandle) Enable() error                      { return nil }

func (h *fakeBFDHandle) emit(t *testing.T, state packet.State, diag packet.Diag) {
	t.Helper()
	select {
	case h.ch <- api.StateChange{Key: h.key, State: state, Diag: diag, When: time.Now()}:
	case <-time.After(time.Second):
		t.Fatal("timed out emitting StateChange")
	}
}

type fakeBFDService struct {
	mu       sync.Mutex
	ensure   atomic.Int32
	release  atomic.Int32
	requests []api.SessionRequest
	handles  map[api.Key]*fakeBFDHandle
}

func newFakeBFDService() *fakeBFDService {
	return &fakeBFDService{handles: make(map[api.Key]*fakeBFDHandle)}
}

func (s *fakeBFDService) EnsureSession(req api.SessionRequest) (api.SessionHandle, error) {
	s.ensure.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	if h, ok := s.handles[req.Key()]; ok {
		return h, nil
	}
	h := &fakeBFDHandle{key: req.Key(), ch: make(chan api.StateChange, 4)}
	s.handles[req.Key()] = h
	return h, nil
}

func (s *fakeBFDService) ReleaseSession(h api.SessionHandle) error {
	s.release.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if fh, ok := s.handles[h.Key()]; ok {
		close(fh.ch)
		delete(s.handles, h.Key())
	}
	return nil
}

func (s *fakeBFDService) Snapshot() []api.SessionState { return nil }
func (s *fakeBFDService) SessionDetail(string) (api.SessionState, bool) {
	return api.SessionState{}, false
}
func (s *fakeBFDService) Profiles() []api.ProfileState { return nil }

func (s *fakeBFDService) firstRequest(t *testing.T) api.SessionRequest {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		t.Fatal("no EnsureSession request recorded")
	}
	return s.requests[0]
}

func (s *fakeBFDService) handleFor(t *testing.T, key api.Key) *fakeBFDHandle {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.handles[key]
	if !ok {
		t.Fatalf("no handle for key %+v", key)
	}
	return h
}

// ---- counting metrics registry (labels ignored; totals by name) ----

type bfdMetricGauge struct{ v *int64 }

func (g bfdMetricGauge) Set(float64) {}
func (g bfdMetricGauge) Add(float64) {}
func (g bfdMetricGauge) Inc()        { atomic.AddInt64(g.v, 1) }
func (g bfdMetricGauge) Dec()        { atomic.AddInt64(g.v, -1) }

type bfdMetricCounter struct{ v *int64 }

func (c bfdMetricCounter) Inc()        { atomic.AddInt64(c.v, 1) }
func (c bfdMetricCounter) Add(float64) {}

type bfdMetricCounterVec struct{ v *int64 }

func (v bfdMetricCounterVec) With(...string) metrics.Counter { return bfdMetricCounter(v) }
func (v bfdMetricCounterVec) Delete(...string) bool          { return false }

type bfdMetricGaugeVec struct{ v *int64 }

func (v bfdMetricGaugeVec) With(...string) metrics.Gauge { return bfdMetricGauge(v) }
func (v bfdMetricGaugeVec) Delete(...string) bool        { return false }

type bfdMetricRegistry struct {
	metrics.NopRegistry
	mu     sync.Mutex
	values map[string]*int64
}

func newBFDMetricRegistry() *bfdMetricRegistry {
	return &bfdMetricRegistry{values: make(map[string]*int64)}
}

func (r *bfdMetricRegistry) slot(name string) *int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.values[name]; ok {
		return v
	}
	v := new(int64)
	r.values[name] = v
	return v
}

func (r *bfdMetricRegistry) get(name string) int64 { return atomic.LoadInt64(r.slot(name)) }

func (r *bfdMetricRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return bfdMetricCounterVec{r.slot(name)}
}

func (r *bfdMetricRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	return bfdMetricGaugeVec{r.slot(name)}
}

// ---- helpers ----

func ridMust(t *testing.T, s string) types.RouterID {
	t.Helper()
	id, err := types.ParseRouterID(s)
	if err != nil {
		t.Fatalf("ParseRouterID(%q): %v", s, err)
	}
	return id
}

const (
	bfdTestIface    = "eth0"
	bfdTestLocalRID = "10.0.0.1"
	bfdTestPeerRID  = "10.0.0.2"
	bfdTestPeerAddr = "192.0.2.2"
)

// bfdTestEngine builds a v4 engine (no transport) with counting BFD metrics, the bfdTestIface
// interface in its running config carrying the given BFD settings, and the neighbor table
// configured for that interface. The fake BFD service is published for the test's duration.
func bfdTestEngine(t *testing.T, bfd bfdInterfaceConfig, svc api.Service) (*engine, *bfdMetricRegistry) {
	t.Helper()
	e := newEngine(nil)
	reg := newBFDMetricRegistry()
	e.setBFDMetrics(reg)
	e.mu.Lock()
	e.running[bfdTestIface] = interfaceConfig{Name: bfdTestIface, AreaID: types.BackboneArea, Enabled: true, NetworkType: networkPointToPoint, BFD: bfd}
	e.mu.Unlock()
	e.neighbors.ConfigureInterface(ospfneighbor.InterfaceConfig{
		Name: bfdTestIface, AreaID: types.BackboneArea, RouterID: ridMust(t, bfdTestLocalRID),
		NetworkType: networkPointToPoint, Options: types.OptionE, DeadInterval: 40, InterfaceMTU: 1500,
	})
	api.SetService(svc)
	t.Cleanup(func() { api.SetService(nil); e.shutdown() })
	return e, reg
}

// seedNeighbor drives a single non-two-way Hello so the neighbor exists (Init) with a valid
// Address, which NeighborAddress returns for the BFD Peer.
func seedNeighbor(t *testing.T, e *engine, id types.RouterID, addr netip.Addr) {
	t.Helper()
	e.neighbors.Hello(ospfneighbor.HelloInput{
		InterfaceName: bfdTestIface, AreaID: types.BackboneArea, LocalRouterID: ridMust(t, bfdTestLocalRID),
		NeighborID: id, Address: addr, Priority: 1, TwoWay: false,
		NetworkType: networkPointToPoint, DeadInterval: 40, InterfaceMTU: 1500, Now: time.Now(),
	})
}

// driveNeighborFull drives the neighbor FSM in the engine's table to Full via the DBD
// exchange (mirroring neighbor/nsm_test.driveFull), so table-facing paths (FloodNeighbors,
// reconcileBFD) see a Full adjacency.
func driveNeighborFull(t *testing.T, e *engine, id types.RouterID, addr netip.Addr) {
	t.Helper()
	e.neighbors.Hello(ospfneighbor.HelloInput{
		InterfaceName: bfdTestIface, AreaID: types.BackboneArea, LocalRouterID: ridMust(t, bfdTestLocalRID),
		NeighborID: id, Address: addr, Priority: 1, TwoWay: true,
		NetworkType: networkPointToPoint, DeadInterval: 40, InterfaceMTU: 1500, Now: time.Now(),
	})
	if r := e.neighbors.HandleDBDesc(bfdTestIface, id, ospfpacket.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE,
		Flags:      ospfpacket.DDFlagInit | ospfpacket.DDFlagMore | ospfpacket.DDFlagMaster,
		DDSequence: 7,
	}); r != "" {
		t.Fatalf("ExStart DD: %s", r)
	}
	snap, ok := e.neighbors.Lookup(bfdTestIface, id)
	if !ok {
		t.Fatalf("neighbor %s missing after ExStart", id)
	}
	seq := snap.DDSequence
	flags := uint8(0)
	if !snap.Master {
		seq++
		flags = ospfpacket.DDFlagMaster
	}
	if r := e.neighbors.HandleDBDesc(bfdTestIface, id, ospfpacket.DBDesc{
		InterfaceMTU: 1500, Options: types.OptionE, Flags: flags, DDSequence: seq,
	}); r != "" {
		t.Fatalf("Exchange DD: %s", r)
	}
	waitFor(t, func() bool {
		s, ok := e.neighbors.Lookup(bfdTestIface, id)
		return ok && s.State == neighborStateFull
	})
}

func fullSnap(id types.RouterID) ospfneighbor.Snapshot {
	return ospfneighbor.Snapshot{Interface: bfdTestIface, Area: types.BackboneArea.String(), RouterID: id.String(), State: neighborStateFull}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

func enabledBFD() bfdInterfaceConfig {
	return bfdInterfaceConfig{Enabled: true, MinTxUs: 50000, MinRxUs: 50000, Multiplier: 3}
}

// ---- request builder (AC-2, AC-13, R-2) ----

func TestBFDRequestForNeighbor(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.2")
	local := netip.MustParseAddr("192.0.2.1")
	cfg := bfdInterfaceConfig{Enabled: true, MinTxUs: 50000, MinRxUs: 60000, Multiplier: 4}
	req := bfdRequestForNeighbor(peer, local, "eth0", cfg)
	if req.Peer != peer || req.Local != local {
		t.Fatalf("pair = %v/%v, want %v/%v", req.Peer, req.Local, peer, local)
	}
	if req.Mode != api.SingleHop {
		t.Fatalf("mode = %v, want SingleHop", req.Mode)
	}
	if req.Interface != "eth0" {
		t.Fatalf("interface = %q, want eth0", req.Interface)
	}
	if req.DesiredMinTxInterval != 50000 || req.RequiredMinRxInterval != 60000 || req.DetectMult != 4 {
		t.Fatalf("timers = %d/%d x%d, want 50000/60000 x4", req.DesiredMinTxInterval, req.RequiredMinRxInterval, req.DetectMult)
	}
	if req.Passive {
		t.Fatal("Passive = true, want false (RFC 5881 both ends Active)")
	}
}

// ---- session open on Full (AC-2) ----

func TestOSPFBFDSessionOpenedOnFull(t *testing.T) {
	svc := newFakeBFDService()
	e, reg := bfdTestEngine(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))

	e.bfdNeighborFull(fullSnap(id))

	if svc.ensure.Load() != 1 {
		t.Fatalf("EnsureSession calls = %d, want 1", svc.ensure.Load())
	}
	req := svc.firstRequest(t)
	if req.Peer != netip.MustParseAddr(bfdTestPeerAddr) || req.Mode != api.SingleHop {
		t.Fatalf("request = %+v, want peer %s single-hop", req, bfdTestPeerAddr)
	}
	if !svc.handleFor(t, req.Key()).subscribed.Load() {
		t.Fatal("Subscribe not called on the handle")
	}
	if reg.get("ze_ospf_bfd_sessions") != 1 {
		t.Fatalf("sessions gauge = %d, want 1", reg.get("ze_ospf_bfd_sessions"))
	}
}

// ---- not opened when disabled (AC-3) ----

func TestOSPFBFDNotOpenedWhenDisabled(t *testing.T) {
	svc := newFakeBFDService()
	e, _ := bfdTestEngine(t, bfdInterfaceConfig{Enabled: false}, svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	e.bfdNeighborFull(fullSnap(id))
	if svc.ensure.Load() != 0 {
		t.Fatalf("EnsureSession calls = %d, want 0 (BFD disabled)", svc.ensure.Load())
	}
}

// ---- only at Full / with a known address (AC-4/A-4) ----

func TestOSPFBFDOnlyAtFull(t *testing.T) {
	svc := newFakeBFDService()
	e, _ := bfdTestEngine(t, enabledBFD(), svc)
	// No neighbor seeded: NeighborAddress fails, so no session opens even for a full snapshot.
	e.bfdNeighborFull(fullSnap(ridMust(t, "10.0.0.9")))
	if svc.ensure.Load() != 0 {
		t.Fatalf("EnsureSession calls = %d, want 0 (no Full neighbor address)", svc.ensure.Load())
	}
}

// ---- graceful degradation when the BFD plugin is absent (AC-4) ----

func TestOSPFBFDGracefulWhenPluginAbsent(t *testing.T) {
	e, reg := bfdTestEngine(t, enabledBFD(), nil)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	e.bfdNeighborFull(fullSnap(id))
	if reg.get("ze_ospf_bfd_register_failures_total") != 1 {
		t.Fatalf("register failures = %d, want 1", reg.get("ze_ospf_bfd_register_failures_total"))
	}
	if reg.get("ze_ospf_bfd_sessions") != 0 {
		t.Fatal("a session was opened despite the missing plugin")
	}
}

// ---- BFD Down drives Table.NeighborDown (AC-5) ----

func TestOSPFBFDDownDrivesNeighborDown(t *testing.T) {
	svc := newFakeBFDService()
	e, reg := bfdTestEngine(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	e.bfdNeighborFull(fullSnap(id))

	svc.handleFor(t, svc.firstRequest(t).Key()).emit(t, packet.StateDown, packet.DiagControlDetectExpired)

	waitFor(t, func() bool {
		snap, ok := e.neighbors.Lookup("eth0", id)
		return ok && snap.State == "down"
	})
	if reg.get("ze_ospf_bfd_session_down_total") != 1 {
		t.Fatalf("session_down_total = %d, want 1", reg.get("ze_ospf_bfd_session_down_total"))
	}
	waitFor(t, func() bool { return svc.release.Load() == 1 })
	waitFor(t, func() bool { return reg.get("ze_ospf_bfd_sessions") == 0 })
}

// ---- AdminDown treated as Down (AC-6) ----

func TestOSPFBFDAdminDownTreatedAsDown(t *testing.T) {
	svc := newFakeBFDService()
	e, _ := bfdTestEngine(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	e.bfdNeighborFull(fullSnap(id))
	svc.handleFor(t, svc.firstRequest(t).Key()).emit(t, packet.StateAdminDown, packet.DiagAdminDown)
	waitFor(t, func() bool {
		snap, ok := e.neighbors.Lookup("eth0", id)
		return ok && snap.State == "down"
	})
}

// ---- Up/Init are inert (AC-7) ----

func TestOSPFBFDUpInitNoTeardown(t *testing.T) {
	svc := newFakeBFDService()
	e, _ := bfdTestEngine(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	e.bfdNeighborFull(fullSnap(id))
	h := svc.handleFor(t, svc.firstRequest(t).Key())
	h.emit(t, packet.StateInit, packet.DiagNone)
	h.emit(t, packet.StateUp, packet.DiagNone)
	waitFor(t, func() bool {
		st, ok := e.bfdSessionState(bfdClientKey{iface: "eth0", router: id})
		return ok && st == packet.StateUp.String()
	})
	if svc.release.Load() != 0 {
		t.Fatalf("ReleaseSession calls = %d, want 0 (Up/Init inert)", svc.release.Load())
	}
	if snap, ok := e.neighbors.Lookup("eth0", id); ok && snap.State == "down" {
		t.Fatal("neighbor dropped on BFD Up/Init")
	}
}

// ---- released on leaving Full; subscriber exits; idempotent (AC-8, R-3) ----

func TestOSPFBFDSessionReleasedOnDown(t *testing.T) {
	svc := newFakeBFDService()
	e, reg := bfdTestEngine(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	e.bfdNeighborFull(fullSnap(id))
	h := svc.handleFor(t, svc.firstRequest(t).Key())

	e.bfdNeighborLost(fullSnap(id))
	if svc.release.Load() != 1 {
		t.Fatalf("ReleaseSession calls = %d, want 1", svc.release.Load())
	}
	if !h.unsubbed.Load() {
		t.Fatal("Unsubscribe not called")
	}
	if reg.get("ze_ospf_bfd_sessions") != 0 {
		t.Fatalf("sessions gauge = %d, want 0", reg.get("ze_ospf_bfd_sessions"))
	}
	e.bfdNeighborLost(fullSnap(id)) // idempotent second release
	if svc.release.Load() != 1 {
		t.Fatalf("ReleaseSession calls = %d after double release, want 1", svc.release.Load())
	}
}

// ---- BFD down + timer down race is idempotent (AC-9, R-1) ----

func TestOSPFBFDDownIdempotentWithTimerDown(t *testing.T) {
	svc := newFakeBFDService()
	e, _ := bfdTestEngine(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	e.bfdNeighborFull(fullSnap(id))
	e.bfdNeighborLost(fullSnap(id)) // timer-driven down releases + joins
	e.bfdNeighborLost(fullSnap(id)) // racing BFD-driven down is a no-op
	if svc.release.Load() != 1 {
		t.Fatalf("ReleaseSession calls = %d, want exactly 1 (idempotent)", svc.release.Load())
	}
}

// ---- distinct keys per neighbor (AC-12 partial, A-9) ----

func TestOSPFBFDDistinctKeysPerNeighbor(t *testing.T) {
	svc := newFakeBFDService()
	e, reg := bfdTestEngine(t, enabledBFD(), svc)
	a := ridMust(t, bfdTestPeerRID)
	b := ridMust(t, "10.0.0.3")
	seedNeighbor(t, e, a, netip.MustParseAddr(bfdTestPeerAddr))
	seedNeighbor(t, e, b, netip.MustParseAddr("192.0.2.3"))
	e.bfdNeighborFull(fullSnap(a))
	e.bfdNeighborFull(fullSnap(b))
	if svc.ensure.Load() != 2 {
		t.Fatalf("EnsureSession calls = %d, want 2", svc.ensure.Load())
	}
	svc.mu.Lock()
	k0, k1 := svc.requests[0].Key(), svc.requests[1].Key()
	svc.mu.Unlock()
	if k0 == k1 {
		t.Fatal("two neighbors produced the same BFD Key")
	}
	e.bfdNeighborLost(fullSnap(a))
	if reg.get("ze_ospf_bfd_sessions") != 1 {
		t.Fatalf("sessions gauge = %d after releasing one, want 1", reg.get("ze_ospf_bfd_sessions"))
	}
}

// ---- reload enable opens sessions for already-Full neighbors (AC-11, A-10) ----

func TestOSPFBFDReloadEnablesSessionForFullNeighbor(t *testing.T) {
	svc := newFakeBFDService()
	e, reg := bfdTestEngine(t, bfdInterfaceConfig{Enabled: false}, svc)
	id := ridMust(t, bfdTestPeerRID)
	driveNeighborFull(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	// While disabled, a Full transition opens nothing.
	e.bfdNeighborFull(fullSnap(id))
	if svc.ensure.Load() != 0 {
		t.Fatalf("EnsureSession = %d before enable, want 0", svc.ensure.Load())
	}
	desired := map[string]interfaceConfig{
		"eth0": {Name: "eth0", AreaID: types.BackboneArea, Enabled: true, NetworkType: networkPointToPoint, BFD: enabledBFD()},
	}
	e.reconcileBFD(desired)
	waitFor(t, func() bool { return svc.ensure.Load() == 1 })
	if reg.get("ze_ospf_bfd_sessions") != 1 {
		t.Fatalf("sessions gauge = %d after reload-enable, want 1", reg.get("ze_ospf_bfd_sessions"))
	}
}

// ---- reload disable releases sessions, adjacency stays Full (AC-10) ----

func TestOSPFBFDReloadDisableKeepsAdjacency(t *testing.T) {
	svc := newFakeBFDService()
	e, _ := bfdTestEngine(t, enabledBFD(), svc)
	id := ridMust(t, bfdTestPeerRID)
	driveNeighborFull(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
	e.bfdNeighborFull(fullSnap(id))
	if svc.ensure.Load() != 1 {
		t.Fatalf("EnsureSession = %d, want 1", svc.ensure.Load())
	}
	desired := map[string]interfaceConfig{
		"eth0": {Name: "eth0", AreaID: types.BackboneArea, Enabled: true, NetworkType: networkPointToPoint, BFD: bfdInterfaceConfig{Enabled: false}},
	}
	e.reconcileBFD(desired)
	if svc.release.Load() != 1 {
		t.Fatalf("ReleaseSession = %d after reload-disable, want 1", svc.release.Load())
	}
	if snap, ok := e.neighbors.Lookup("eth0", id); !ok || snap.State != neighborStateFull {
		t.Fatalf("neighbor state = %v (ok=%v), want full (disable must not bounce)", snap.State, ok)
	}
}
