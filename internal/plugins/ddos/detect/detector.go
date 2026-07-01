// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- two-stage DDoS detector

package detect

import (
	"context"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/trafficstat"
	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var loggerPtr atomic.Pointer[slog.Logger]

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func logger() *slog.Logger {
	if l := loggerPtr.Load(); l != nil {
		return l
	}
	return slog.Default()
}

type detector struct {
	mu       sync.Mutex
	cfg      *Config
	bus      ze.EventBus
	dispatch dispatchFunc
	baseline *baseline
	sm       *stateMachine
	tickNum  int

	prevRxPackets map[string]uint64
	prevRxBytes   map[string]uint64
	currentRxPps  map[string]float64
	peakRxPps     float64
	peakRxBps     float64
	attackIface   string
	justTriggered bool

	wg                   sync.WaitGroup
	sourceAbsentOnce     sync.Once // trafficusage (fast target) absent, logged once
	flowSourceAbsentOnce sync.Once // flowexport recent-flow (characterization) absent, logged once
	detectedEmitted      atomic.Bool
	attackGen            uint64 // bumped on each activate and clear; guards stale async emits

	// ctx bounds every characterization goroutine to the detector's lifetime;
	// Stop cancels it so in-flight source queries unwind promptly at shutdown or
	// reconfigure instead of running out their per-query timeout. stopped (under
	// d.mu) fences onAttackStart from spawning a new goroutine once Stop has begun
	// waiting, so wg.Go can never race wg.Wait.
	ctx     context.Context
	cancel  context.CancelFunc
	stopped bool
}

func newDetector(cfg *Config, bus ze.EventBus, dispatch dispatchFunc) *detector {
	d := &detector{
		cfg:           cfg,
		bus:           bus,
		dispatch:      dispatch,
		baseline:      newBaseline(cfg.BaselineWindow, cfg.ThresholdMultiplier, cfg.AbsoluteFloor),
		prevRxPackets: make(map[string]uint64),
		prevRxBytes:   make(map[string]uint64),
		currentRxPps:  make(map[string]float64),
	}
	d.sm = newStateMachine(cfg.ConfirmDuration, cfg.ClearConsecutive)
	d.sm.OnDetected = d.onAttackStart
	d.sm.OnCleared = d.emitCleared
	d.ctx, d.cancel = context.WithCancel(context.Background())
	return d
}

// Stop cancels in-flight characterization and waits for the goroutines to unwind.
// Called at plugin shutdown and before a reconfigure replaces the detector, so a
// slow source query cannot outlive the detector that spawned it. Setting stopped
// under d.mu before Wait fences onAttackStart (which spawns under d.mu) from
// adding to the WaitGroup after Wait has started -- a late trafficstat tick
// delivered after Unsubscribe would otherwise panic "WaitGroup reused".
func (d *detector) Stop() {
	d.mu.Lock()
	d.stopped = true
	d.mu.Unlock()
	if d.cancel != nil {
		d.cancel()
	}
	d.wg.Wait()
}

// onRates accepts pre-computed per-interface rates from the trafficstat
// service, replacing the raw-counter diffing that onRate performs.
func (d *detector) onRates(entries []trafficstat.InterfaceEntry) {
	if !d.cfg.Enabled {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.tickNum++

	var maxPps, maxBps float64
	var maxIface string
	for i := range entries {
		e := &entries[i]
		d.currentRxPps[e.Name] = e.RxPps

		if e.RxPps > maxPps {
			maxPps = e.RxPps
			maxIface = e.Name
			maxBps = e.RxBps
		}
	}

	d.applyTick(maxPps, maxBps, maxIface)
}

func (d *detector) onRate(infos []iface.InterfaceInfo) {
	if !d.cfg.Enabled {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.tickNum++

	var maxPps, maxBps float64
	var maxIface string
	for i := range infos {
		info := &infos[i]
		if info.Stats == nil {
			continue
		}

		prevPkts, hasPrev := d.prevRxPackets[info.Name]
		prevBytes := d.prevRxBytes[info.Name]
		d.prevRxPackets[info.Name] = info.Stats.RxPackets
		d.prevRxBytes[info.Name] = info.Stats.RxBytes

		if !hasPrev {
			continue
		}
		var pps, bps float64
		if info.Stats.RxPackets >= prevPkts {
			pps = float64(info.Stats.RxPackets - prevPkts)
		}
		if info.Stats.RxBytes >= prevBytes {
			bps = float64(info.Stats.RxBytes - prevBytes)
		}
		d.currentRxPps[info.Name] = pps

		if pps > maxPps {
			maxPps = pps
			maxIface = info.Name
			maxBps = bps
		}
	}

	d.applyTick(maxPps, maxBps, maxIface)
}

// applyTick runs the baseline, state machine, and event emission for a
// single tick. Caller must hold d.mu.
func (d *detector) applyTick(maxPps, maxBps float64, maxIface string) {
	if d.tickNum <= d.cfg.StartupGrace {
		if maxPps < d.cfg.AbsoluteFloor*5 {
			return
		}
	}

	threshold := d.baseline.Threshold()
	above := maxPps > threshold

	st := d.sm.State()
	attacking := st == stateActive || st == stateClearing
	d.baseline.Add(maxPps, attacking || above)

	if above && maxPps > d.peakRxPps {
		d.peakRxPps = maxPps
		d.peakRxBps = maxBps
		d.attackIface = maxIface
	}

	d.justTriggered = false
	d.sm.Tick(above)

	if d.sm.State() == stateActive && !d.justTriggered && d.detectedEmitted.Load() {
		if _, err := ddosevent.Ongoing.Emit(d.bus, &ddosevent.AttackOngoing{
			Interface:  d.attackIface,
			Target:     ddosevent.VectorTuple{DstPrefix: netip.Prefix{}},
			CurrentPps: maxPps,
			CurrentBps: maxBps,
			Observable: true,
		}); err != nil {
			logger().Warn("ddos-detect: emit ongoing failed", "error", err)
		}
	}
}

// onAttackStart fires once on the idle/confirming -> active transition, under
// d.mu (called from onRate via the state machine). It snapshots the attack
// context and launches characterization on its own goroutine so the rate tick
// and the detector mutex are never blocked by the engine round-trip. The emit
// happens from characterizeAndEmit once the target is resolved (or falls back).
func (d *detector) onAttackStart() {
	d.justTriggered = true
	d.attackGen++
	// Do not spawn once Stop has begun waiting on the WaitGroup (a late
	// trafficstat tick can still reach here after Unsubscribe returns, because
	// trafficstat invokes subscribers outside its lock). Both this and Stop run
	// under d.mu, so the flag fences wg.Go against wg.Wait.
	if d.stopped {
		return
	}
	gen := d.attackGen
	ifaceName := d.attackIface
	peakPps := d.peakRxPps
	peakBps := d.peakRxBps
	// Snapshot the threshold under d.mu so severity grades against the baseline
	// as it was at trigger time, not whatever it drifts to during the query.
	threshold := d.baseline.Threshold()
	d.wg.Go(func() {
		d.characterizeAndEmit(gen, ifaceName, peakPps, peakBps, threshold)
	})
}

func (d *detector) emitCleared() {
	d.attackGen++
	d.detectedEmitted.Store(false)
	if _, err := ddosevent.Cleared.Emit(d.bus, &ddosevent.AttackCleared{
		Interface:  d.attackIface,
		Target:     ddosevent.VectorTuple{DstPrefix: netip.Prefix{}},
		Observable: true,
	}); err != nil {
		logger().Warn("ddos-detect: emit cleared failed", "error", err)
	}
	d.peakRxPps = 0
	d.peakRxBps = 0
	d.attackIface = ""
}
