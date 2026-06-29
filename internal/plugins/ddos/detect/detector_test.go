package detect

import (
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/trafficstat"
	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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
