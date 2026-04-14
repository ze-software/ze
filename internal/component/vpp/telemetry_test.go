package vpp

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.fd.io/govpp/api"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// mockStatsProvider implements statsProvider for testing.
type mockStatsProvider struct {
	mu    sync.Mutex
	iface api.InterfaceStats
	node  api.NodeStats
	sys   api.SystemStats
	err   error
	calls int
}

func (m *mockStatsProvider) GetInterfaceStats(s *api.InterfaceStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return m.err
	}
	*s = m.iface
	return nil
}

func (m *mockStatsProvider) GetNodeStats(s *api.NodeStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	*s = m.node
	return nil
}

func (m *mockStatsProvider) GetSystemStats(s *api.SystemStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	*s = m.sys
	return nil
}

// collectableGauge records Set calls for test assertions.
type collectableGauge struct {
	mu  sync.Mutex
	val float64
}

func (g *collectableGauge) Set(v float64) { g.mu.Lock(); g.val = v; g.mu.Unlock() }
func (g *collectableGauge) Inc()          { g.mu.Lock(); g.val++; g.mu.Unlock() }
func (g *collectableGauge) Dec()          { g.mu.Lock(); g.val--; g.mu.Unlock() }
func (g *collectableGauge) Add(v float64) { g.mu.Lock(); g.val += v; g.mu.Unlock() }
func (g *collectableGauge) get() float64  { g.mu.Lock(); defer g.mu.Unlock(); return g.val }

// collectableCounter records Add/Inc calls for test assertions.
type collectableCounter struct {
	mu  sync.Mutex
	val float64
}

func (c *collectableCounter) Inc()          { c.mu.Lock(); c.val++; c.mu.Unlock() }
func (c *collectableCounter) Add(v float64) { c.mu.Lock(); c.val += v; c.mu.Unlock() }
func (c *collectableCounter) get() float64  { c.mu.Lock(); defer c.mu.Unlock(); return c.val }

// collectableGaugeVec records per-label gauge values.
type collectableGaugeVec struct {
	mu     sync.Mutex
	gauges map[string]*collectableGauge
}

func newCollectableGaugeVec() *collectableGaugeVec {
	return &collectableGaugeVec{gauges: make(map[string]*collectableGauge)}
}

func (v *collectableGaugeVec) With(labels ...string) metrics.Gauge {
	key := labels[0]
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.gauges[key]
	if !ok {
		g = &collectableGauge{}
		v.gauges[key] = g
	}
	return g
}

func (v *collectableGaugeVec) Delete(_ ...string) bool { return true }

func (v *collectableGaugeVec) get(label string) float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	g, ok := v.gauges[label]
	if !ok {
		return 0
	}
	return g.get()
}

// collectableCounterVec records per-label counter values.
type collectableCounterVec struct {
	mu       sync.Mutex
	counters map[string]*collectableCounter
}

func newCollectableCounterVec() *collectableCounterVec {
	return &collectableCounterVec{counters: make(map[string]*collectableCounter)}
}

func (v *collectableCounterVec) With(labels ...string) metrics.Counter {
	key := labels[0]
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.counters[key]
	if !ok {
		c = &collectableCounter{}
		v.counters[key] = c
	}
	return c
}

func (v *collectableCounterVec) Delete(_ ...string) bool { return true }

func (v *collectableCounterVec) get(label string) float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.counters[label]
	if !ok {
		return 0
	}
	return c.get()
}

// testRegistry returns collectable metrics for assertions.
type testRegistry struct {
	gauges      map[string]*collectableGauge
	counters    map[string]*collectableCounter
	gaugeVecs   map[string]*collectableGaugeVec
	counterVecs map[string]*collectableCounterVec
}

func newTestRegistry() *testRegistry {
	return &testRegistry{
		gauges:      make(map[string]*collectableGauge),
		counters:    make(map[string]*collectableCounter),
		gaugeVecs:   make(map[string]*collectableGaugeVec),
		counterVecs: make(map[string]*collectableCounterVec),
	}
}

func (r *testRegistry) Counter(name, _ string) metrics.Counter {
	c := &collectableCounter{}
	r.counters[name] = c
	return c
}

func (r *testRegistry) Gauge(name, _ string) metrics.Gauge {
	g := &collectableGauge{}
	r.gauges[name] = g
	return g
}

func (r *testRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	v := newCollectableCounterVec()
	r.counterVecs[name] = v
	return v
}

func (r *testRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	v := newCollectableGaugeVec()
	r.gaugeVecs[name] = v
	return v
}

func (r *testRegistry) Histogram(_, _ string, _ []float64) metrics.Histogram { return nil }
func (r *testRegistry) HistogramVec(_, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return nil
}

// VALIDATES: AC-1 — stats client connected to stats socket.
// VALIDATES: AC-2 — poll timer fires, reads InterfaceStats/NodeStats/SystemStats.
// PREVENTS: stats poller not calling provider methods.
func TestStatsPollerRun(t *testing.T) {
	provider := &mockStatsProvider{
		iface: api.InterfaceStats{
			Interfaces: []api.InterfaceCounters{
				{InterfaceName: "eth0", Rx: api.InterfaceCounterCombined{Packets: 100, Bytes: 5000}},
			},
		},
		sys: api.SystemStats{VectorRate: 42},
	}

	reg := newTestRegistry()
	m := newVPPMetrics(reg)
	poller := newStatsPoller(provider, m, time.Millisecond*50)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go poller.run(ctx)

	// Wait for at least one poll cycle.
	time.Sleep(time.Millisecond * 150)
	cancel()

	provider.mu.Lock()
	calls := provider.calls
	provider.mu.Unlock()

	if calls < 1 {
		t.Fatalf("expected at least 1 poll call, got %d", calls)
	}
}

// VALIDATES: AC-3 — ze_vpp_interface_rx_packets counter present per interface.
// VALIDATES: AC-4 — ze_vpp_interface_tx_bytes counter present per interface.
// VALIDATES: AC-5 — ze_vpp_interface_drops counter present per interface.
// PREVENTS: interface metrics not converted from VPP stats.
func TestInterfaceStatsToMetrics(t *testing.T) {
	provider := &mockStatsProvider{
		iface: api.InterfaceStats{
			Interfaces: []api.InterfaceCounters{
				{
					InterfaceName: "GigE0/0/0",
					Rx:            api.InterfaceCounterCombined{Packets: 1000, Bytes: 64000},
					Tx:            api.InterfaceCounterCombined{Packets: 500, Bytes: 32000},
					Drops:         7,
					RxErrors:      3,
					TxErrors:      1,
				},
				{
					InterfaceName: "local0",
					Rx:            api.InterfaceCounterCombined{Packets: 0, Bytes: 0},
					Tx:            api.InterfaceCounterCombined{Packets: 0, Bytes: 0},
					Drops:         0,
				},
			},
		},
	}

	reg := newTestRegistry()
	m := newVPPMetrics(reg)
	poller := newStatsPoller(provider, m, time.Millisecond*50)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go poller.run(ctx)
	time.Sleep(time.Millisecond * 150)
	cancel()

	// Check per-interface metrics via GaugeVec.
	rxPkts := reg.counterVecs["ze_vpp_interface_rx_packets"]
	if rxPkts == nil {
		t.Fatal("ze_vpp_interface_rx_packets not registered")
	}
	if got := rxPkts.get("GigE0/0/0"); got != 1000 {
		t.Errorf("rx_packets for GigE0/0/0: got %v, want 1000", got)
	}

	txBytes := reg.counterVecs["ze_vpp_interface_tx_bytes"]
	if txBytes == nil {
		t.Fatal("ze_vpp_interface_tx_bytes not registered")
	}
	if got := txBytes.get("GigE0/0/0"); got != 32000 {
		t.Errorf("tx_bytes for GigE0/0/0: got %v, want 32000", got)
	}

	drops := reg.counterVecs["ze_vpp_interface_drops"]
	if drops == nil {
		t.Fatal("ze_vpp_interface_drops not registered")
	}
	if got := drops.get("GigE0/0/0"); got != 7 {
		t.Errorf("drops for GigE0/0/0: got %v, want 7", got)
	}

	rxErrs := reg.counterVecs["ze_vpp_interface_rx_errors"]
	if rxErrs == nil {
		t.Fatal("ze_vpp_interface_rx_errors not registered")
	}
	if got := rxErrs.get("GigE0/0/0"); got != 3 {
		t.Errorf("rx_errors for GigE0/0/0: got %v, want 3", got)
	}

	txErrs := reg.counterVecs["ze_vpp_interface_tx_errors"]
	if txErrs == nil {
		t.Fatal("ze_vpp_interface_tx_errors not registered")
	}
	if got := txErrs.get("GigE0/0/0"); got != 1 {
		t.Errorf("tx_errors for GigE0/0/0: got %v, want 1", got)
	}

	// Verify second interface (local0) has zero counters.
	if got := rxPkts.get("local0"); got != 0 {
		t.Errorf("rx_packets for local0: got %v, want 0", got)
	}
}

// VALIDATES: AC-6 — ze_vpp_node_clocks gauge present per graph node.
// VALIDATES: AC-7 — ze_vpp_node_vectors gauge present per graph node.
// PREVENTS: node metrics not extracted from VPP stats.
func TestNodeStatsToMetrics(t *testing.T) {
	provider := &mockStatsProvider{
		node: api.NodeStats{
			Nodes: []api.NodeCounters{
				{NodeName: "ip4-input", Clocks: 9999, Vectors: 5000, Calls: 100},
				{NodeName: "ip4-lookup", Clocks: 3000, Vectors: 2000, Calls: 50},
			},
		},
	}

	reg := newTestRegistry()
	m := newVPPMetrics(reg)
	poller := newStatsPoller(provider, m, time.Millisecond*50)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go poller.run(ctx)
	time.Sleep(time.Millisecond * 150)
	cancel()

	clocks := reg.gaugeVecs["ze_vpp_node_clocks"]
	if clocks == nil {
		t.Fatal("ze_vpp_node_clocks not registered")
	}
	if got := clocks.get("ip4-input"); got != 9999 {
		t.Errorf("node clocks for ip4-input: got %v, want 9999", got)
	}

	vectors := reg.gaugeVecs["ze_vpp_node_vectors"]
	if vectors == nil {
		t.Fatal("ze_vpp_node_vectors not registered")
	}
	if got := vectors.get("ip4-lookup"); got != 2000 {
		t.Errorf("node vectors for ip4-lookup: got %v, want 2000", got)
	}

	calls := reg.gaugeVecs["ze_vpp_node_calls"]
	if calls == nil {
		t.Fatal("ze_vpp_node_calls not registered")
	}
	if got := calls.get("ip4-input"); got != 100 {
		t.Errorf("node calls for ip4-input: got %v, want 100", got)
	}
}

// VALIDATES: AC-8 — ze_vpp_system_vector_rate gauge present.
// PREVENTS: system metrics not read from VPP stats.
func TestSystemStatsToMetrics(t *testing.T) {
	provider := &mockStatsProvider{
		sys: api.SystemStats{
			VectorRate: 1500,
			InputRate:  800,
		},
	}

	reg := newTestRegistry()
	m := newVPPMetrics(reg)
	poller := newStatsPoller(provider, m, time.Millisecond*50)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go poller.run(ctx)
	time.Sleep(time.Millisecond * 150)
	cancel()

	vectorRate := reg.gauges["ze_vpp_system_vector_rate"]
	if vectorRate == nil {
		t.Fatal("ze_vpp_system_vector_rate not registered")
	}
	if got := vectorRate.get(); got != 1500 {
		t.Errorf("vector_rate: got %v, want 1500", got)
	}

	inputRate := reg.gauges["ze_vpp_system_input_rate"]
	if inputRate == nil {
		t.Fatal("ze_vpp_system_input_rate not registered")
	}
	if got := inputRate.get(); got != 800 {
		t.Errorf("input_rate: got %v, want 800", got)
	}
}

// VALIDATES: AC-12 — poll frequency matches config.
// PREVENTS: poll interval not respected.
func TestStatsPollInterval(t *testing.T) {
	provider := &mockStatsProvider{}

	reg := newTestRegistry()
	m := newVPPMetrics(reg)

	// Use 100ms interval, check that 3 polls happen within ~350ms.
	poller := newStatsPoller(provider, m, time.Millisecond*100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go poller.run(ctx)
	time.Sleep(time.Millisecond * 350)
	cancel()

	provider.mu.Lock()
	calls := provider.calls
	provider.mu.Unlock()

	// With 100ms interval and 350ms wait, expect 3-4 calls.
	if calls < 3 || calls > 5 {
		t.Errorf("expected 3-5 poll calls in 350ms at 100ms interval, got %d", calls)
	}
}

// VALIDATES: AC-13 — ze_vpp_stats_up gauge is 1 when connected.
// VALIDATES: AC-14 — ze_vpp_stats_up gauge is 0 when disconnected.
// PREVENTS: stats_up gauge not tracking connection state.
func TestStatsUpGauge(t *testing.T) {
	provider := &mockStatsProvider{}

	reg := newTestRegistry()
	m := newVPPMetrics(reg)
	poller := newStatsPoller(provider, m, time.Millisecond*50)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go poller.run(ctx)
	time.Sleep(time.Millisecond * 100)

	statsUp := reg.gauges["ze_vpp_stats_up"]
	if statsUp == nil {
		t.Fatal("ze_vpp_stats_up not registered")
	}
	if got := statsUp.get(); got != 1 {
		t.Errorf("stats_up while connected: got %v, want 1", got)
	}

	// Simulate disconnect by setting error.
	provider.mu.Lock()
	provider.err = api.VPPApiError(1)
	provider.mu.Unlock()

	time.Sleep(time.Millisecond * 100)

	if got := statsUp.get(); got != 0 {
		t.Errorf("stats_up while disconnected: got %v, want 0", got)
	}
	cancel()
}

// VALIDATES: AC-3 — counter delta correct across multiple polls and VPP restart.
// PREVENTS: counters double-counting or losing data on VPP counter reset.
func TestInterfaceCounterDelta(t *testing.T) {
	provider := &mockStatsProvider{
		iface: api.InterfaceStats{
			Interfaces: []api.InterfaceCounters{
				{InterfaceName: "eth0", Rx: api.InterfaceCounterCombined{Packets: 100}},
			},
		},
	}

	reg := newTestRegistry()
	m := newVPPMetrics(reg)
	poller := newStatsPoller(provider, m, time.Millisecond*50)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go poller.run(ctx)
	time.Sleep(time.Millisecond * 80)

	rxPkts := reg.counterVecs["ze_vpp_interface_rx_packets"]
	if rxPkts == nil {
		t.Fatal("ze_vpp_interface_rx_packets not registered")
	}
	got1 := rxPkts.get("eth0")
	if got1 != 100 {
		t.Fatalf("first poll rx_packets: got %v, want 100", got1)
	}

	// Simulate counter increment (200 total = +100 delta).
	provider.mu.Lock()
	provider.iface = api.InterfaceStats{
		Interfaces: []api.InterfaceCounters{
			{InterfaceName: "eth0", Rx: api.InterfaceCounterCombined{Packets: 200}},
		},
	}
	provider.mu.Unlock()
	time.Sleep(time.Millisecond * 80)

	got2 := rxPkts.get("eth0")
	if got2 != 200 {
		t.Errorf("second poll rx_packets: got %v, want 200 (100 + 100 delta)", got2)
	}

	// Simulate VPP restart: counter resets to 50 (less than previous 200).
	provider.mu.Lock()
	provider.iface = api.InterfaceStats{
		Interfaces: []api.InterfaceCounters{
			{InterfaceName: "eth0", Rx: api.InterfaceCounterCombined{Packets: 50}},
		},
	}
	provider.mu.Unlock()
	time.Sleep(time.Millisecond * 80)

	got3 := rxPkts.get("eth0")
	if got3 != 250 {
		t.Errorf("after counter reset rx_packets: got %v, want 250 (200 + 50 post-reset)", got3)
	}
	cancel()
}

// VALIDATES: AC-11 — stats client reconnects, metrics resume after recovery.
// PREVENTS: stats poller not recovering after transient VPP failure.
func TestStatsReconnect(t *testing.T) {
	provider := &mockStatsProvider{
		sys: api.SystemStats{VectorRate: 100},
	}

	reg := newTestRegistry()
	m := newVPPMetrics(reg)
	poller := newStatsPoller(provider, m, time.Millisecond*50)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go poller.run(ctx)
	time.Sleep(time.Millisecond * 100)

	// Stats should be up.
	statsUp := reg.gauges["ze_vpp_stats_up"]
	if statsUp == nil {
		t.Fatal("ze_vpp_stats_up not registered")
	}
	if got := statsUp.get(); got != 1 {
		t.Fatalf("stats_up initially: got %v, want 1", got)
	}

	// Simulate VPP crash.
	provider.mu.Lock()
	provider.err = api.VPPApiError(1)
	provider.mu.Unlock()
	time.Sleep(time.Millisecond * 100)

	if got := statsUp.get(); got != 0 {
		t.Errorf("stats_up during error: got %v, want 0", got)
	}

	// Simulate VPP recovery.
	provider.mu.Lock()
	provider.err = nil
	provider.sys = api.SystemStats{VectorRate: 200}
	provider.mu.Unlock()
	time.Sleep(time.Millisecond * 100)

	if got := statsUp.get(); got != 1 {
		t.Errorf("stats_up after recovery: got %v, want 1", got)
	}

	vectorRate := reg.gauges["ze_vpp_system_vector_rate"]
	if got := vectorRate.get(); got != 200 {
		t.Errorf("vector_rate after recovery: got %v, want 200", got)
	}
	cancel()
}
