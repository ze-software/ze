package detect

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/trafficstat"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/pkg/ze"
)

type dtestBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func newDTestBus() *dtestBus {
	return &dtestBus{subs: make(map[string][]func(any))}
}

func (b *dtestBus) Emit(namespace, eventType string, payload any) (int, error) {
	key := namespace + "/" + eventType
	b.mu.Lock()
	handlers := make([]func(any), len(b.subs[key]))
	copy(handlers, b.subs[key])
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return 0, nil
}

func (b *dtestBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.subs[key] = append(b.subs[key], handler)
	b.mu.Unlock()
	return func() {}
}

var _ ze.EventBus = (*dtestBus)(nil)

// VALIDATES: a sustained flood above the adaptive threshold drives the state
// machine to active and emits AttackDetected for the hottest interface.
// PREVENTS: regression where the detector stops firing on real volumetric floods.
func TestDetectorEmitsOnFlood(t *testing.T) {
	bus := newDTestBus()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ConfirmDuration = 1
	cfg.AbsoluteFloor = 100
	cfg.BaselineWindow = 10
	cfg.StartupGrace = 0

	d := newDetector(cfg, bus, nil)

	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) {
		detected = e
	})

	// Feed normal traffic to build baseline (cumulative counters, 50 pps)
	var cumPkts uint64
	for range 20 {
		cumPkts += 50
		d.onRate([]iface.InterfaceInfo{
			{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cumPkts}},
		})
	}

	// Spike above threshold (100000 pps per tick)
	for range 5 {
		cumPkts += 100000
		d.onRate([]iface.InterfaceInfo{
			{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: cumPkts}},
		})
	}

	d.wg.Wait()

	if detected == nil {
		t.Fatal("AttackDetected not emitted after flood")
	}
	if detected.Interface != "xe0" {
		t.Errorf("Interface: got %q, want xe0", detected.Interface)
	}
	if !detected.Observable {
		t.Error("Observable should be true")
	}
}

// VALIDATES: detector triggers correctly when consuming pre-computed rates from
// the trafficstat service instead of raw iface counters (AC-8).
// PREVENTS: regression after the Depth-1 refactor that swaps the data source.
func TestDetectorConsumesTrafficstat(t *testing.T) {
	bus := newDTestBus()
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ConfirmDuration = 1
	cfg.AbsoluteFloor = 100
	cfg.BaselineWindow = 10
	cfg.StartupGrace = 0

	d := newDetector(cfg, bus, nil)

	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) {
		detected = e
	})

	// Feed normal traffic via onRates (pre-computed, as trafficstat delivers)
	for range 20 {
		d.onRates([]trafficstat.InterfaceEntry{
			{Name: "xe0", RxPps: 50, RxBps: 5000},
		})
	}

	// Spike above threshold
	for range 5 {
		d.onRates([]trafficstat.InterfaceEntry{
			{Name: "xe0", RxPps: 100000, RxBps: 10000000},
		})
	}

	d.wg.Wait()

	if detected == nil {
		t.Fatal("AttackDetected not emitted via onRates path")
	}
	if detected.Interface != "xe0" {
		t.Errorf("Interface: got %q, want xe0", detected.Interface)
	}
}

// bpsWarmAndAttack drives a detector through baseline warm-up then an attack burst,
// all at a fixed low RxPps so the PPS path (with a huge AbsoluteFloor) never fires,
// isolating the bandwidth trigger. Returns the captured AttackDetected (or nil).
func bpsWarmAndAttack(cfg *Config, warmBps, attackBps float64) *ddosevent.AttackDetected {
	bus := newDTestBus()
	d := newDetector(cfg, bus, nil)
	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })
	for range cfg.BaselineWindow + 5 {
		d.onRates([]trafficstat.InterfaceEntry{{Name: "xe0", RxPps: 50, RxBps: warmBps}})
	}
	for range 3 {
		d.onRates([]trafficstat.InterfaceEntry{{Name: "xe0", RxPps: 50, RxBps: attackBps}})
	}
	d.wg.Wait()
	return detected
}

func bpsTestConfig() *Config {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ConfirmDuration = 1
	cfg.BaselineWindow = 10
	cfg.StartupGrace = 0
	cfg.AbsoluteFloor = 1e9 // huge PPS floor: the PPS path can never fire at RxPps=50
	cfg.BpsTriggerEnable = true
	cfg.BpsThresholdMultiplier = 3.0
	cfg.BpsFloor = 8000 // 1000 bytes/s floor: low, so the p99 multiplier governs
	return cfg
}

// VALIDATES: AC-1 -- a low-PPS / high-bandwidth flow (amplification) trips the BPS
// trigger even though PPS stays far below its threshold.
// PREVENTS: amplification floods (NTP/memcached/CLDAP) going undetected.
func TestApplyTick_BpsTriggerFiresBelowPpsThreshold(t *testing.T) {
	// warm bps ~10000 B/s -> bps threshold ~30000 B/s; attack 500000 B/s (=4 Mbps) trips it.
	detected := bpsWarmAndAttack(bpsTestConfig(), 10000, 500000)
	if detected == nil {
		t.Fatal("BPS trigger did not fire on high-bandwidth low-PPS flow")
	}
	if detected.Interface != "xe0" {
		t.Errorf("Interface: got %q, want xe0", detected.Interface)
	}
}

// VALIDATES: AC-3b -- with bps-trigger-enable=false the bandwidth path never fires.
func TestApplyTick_BpsTriggerDisabled(t *testing.T) {
	cfg := bpsTestConfig()
	cfg.BpsTriggerEnable = false
	if detected := bpsWarmAndAttack(cfg, 10000, 500000); detected != nil {
		t.Fatal("BPS trigger fired while bps-trigger-enable=false")
	}
}

// VALIDATES: AC-3 -- traffic below the bps-floor never trips the BPS trigger,
// regardless of the baseline (a busy-but-legitimate low-bandwidth node).
func TestApplyTick_BpsBelowFloorInert(t *testing.T) {
	cfg := bpsTestConfig()
	cfg.BpsFloor = 50_000_000 // 50 Mbps => 6.25 MB/s floor
	// attack 500000 B/s = 4 Mbps, below the 50 Mbps floor -> inert
	if detected := bpsWarmAndAttack(cfg, 10000, 500000); detected != nil {
		t.Fatal("BPS trigger fired for traffic below the bps-floor")
	}
}

// VALIDATES: AC-1 (multi-interface) -- an amplification flood on a NON-top-PPS
// interface trips the BPS trigger and is attributed to the amplified interface.
// PREVENTS: on a multi-interface router, binding maxBps to the max-PPS interface
// blinded the bandwidth trigger, so a high-Gbps/low-PPS amplification on the
// uplink went undetected whenever a LAN port carried more packets/s.
func TestApplyTick_BpsTriggerFiresOnNonTopPpsInterface(t *testing.T) {
	cfg := bpsTestConfig()
	bus := newDTestBus()
	d := newDetector(cfg, bus, nil)
	var detected *ddosevent.AttackDetected
	ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) { detected = e })

	// Warm-up: "lan" is always the top-PPS interface (5000 pps) but low bandwidth;
	// "uplink" is quiet. Both ~10000 B/s so the BPS threshold settles ~30000 B/s.
	// The huge AbsoluteFloor keeps the PPS path from ever firing (isolates BPS).
	for range cfg.BaselineWindow + 5 {
		d.onRates([]trafficstat.InterfaceEntry{
			{Name: "lan", RxPps: 5000, RxBps: 10000},
			{Name: "uplink", RxPps: 50, RxBps: 10000},
		})
	}
	// Attack: "lan" unchanged (still top PPS); "uplink" gets a low-PPS, high-
	// bandwidth amplification flood (=4 Mbps) only the BPS path can see.
	for range 3 {
		d.onRates([]trafficstat.InterfaceEntry{
			{Name: "lan", RxPps: 5000, RxBps: 10000},
			{Name: "uplink", RxPps: 50, RxBps: 500000},
		})
	}
	d.wg.Wait()

	if detected == nil {
		t.Fatal("BPS trigger did not fire for amplification on a non-top-PPS interface")
	}
	if detected.Interface != "uplink" {
		t.Errorf("attack attributed to %q, want uplink (the amplified interface)", detected.Interface)
	}
}

// VALIDATES: AC-5 -- a restart with a valid persisted baseline restores it (both
// series Ready, PPS p99 preserved) so the window re-warm is skipped.
func TestDetectorRestoresBaselineFromDisk(t *testing.T) {
	useBaselineStore(t)
	pps := baselineState{Samples: makeSamples(300, 1000), Count: 300, P99Cache: 1000}
	bps := baselineState{Samples: makeSamples(300, 20000), Count: 300, P99Cache: 20000}
	if err := saveBaselines(pps, bps); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Enabled = true
	d := newDetector(cfg, newDTestBus(), nil)
	d.restore()
	if !d.baseline.Ready() || !d.baselineBps.Ready() {
		t.Error("both baselines should be Ready after restore (no warm-up)")
	}
	if d.baseline.P99() != 1000 {
		t.Errorf("restored PPS p99 = %v, want 1000", d.baseline.P99())
	}
}

// VALIDATES: Stop persists a loadable baseline (save-on-shutdown/reconfigure), so the
// next construct + restore resumes without a re-warm.
func TestDetectorSavesBaselineOnStop(t *testing.T) {
	useBaselineStore(t)
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.BaselineWindow = 10
	cfg.StartupGrace = 0
	d := newDetector(cfg, newDTestBus(), nil)
	for range 15 {
		d.onRates([]trafficstat.InterfaceEntry{{Name: "xe0", RxPps: 100, RxBps: 20000}})
	}
	d.Stop()
	if _, ok := loadBaselines(); !ok {
		t.Error("Stop should have persisted a loadable baseline")
	}
}

// VALIDATES: when the detector is disabled no AttackDetected is emitted regardless
// of traffic level.
// PREVENTS: the opt-in detector acting (and mitigating) without being configured on.
func TestDetectorNoEventWhenDisabled(t *testing.T) {
	bus := newDTestBus()
	cfg := DefaultConfig()
	cfg.Enabled = false

	d := newDetector(cfg, bus, nil)

	var detected bool
	ddosevent.Detected.Subscribe(bus, func(_ *ddosevent.AttackDetected) {
		detected = true
	})

	var pkts uint64
	for range 50 {
		pkts += 100000
		d.onRate([]iface.InterfaceInfo{
			{Name: "xe0", Stats: &iface.InterfaceStats{RxPackets: pkts}},
		})
	}

	if detected {
		t.Error("should not emit AttackDetected when disabled")
	}
}
