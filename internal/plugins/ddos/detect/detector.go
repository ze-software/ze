// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- two-stage DDoS detector

package detect

import (
	"context"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/trafficstat"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/pkg/ze"
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
	mu sync.Mutex
	// emitMu orders the event-bus fan-out. It is the ONLY lock held across an
	// Emit, and d.mu is never held across one.
	//
	// Why a second lock exists: the fan-out is synchronous and sequential on the
	// emitting goroutine (plugin/server engine_event.go invokeEngineHandler), and
	// it reaches responders that reconcile the kernel inside their handler
	// (ddos-local onDetected -> firewall.ApplyAll -> netlink). Holding d.mu
	// across that parked the detector's whole state -- the rate tick, Stop, and
	// the baseline save all take d.mu -- behind a netlink round trip.
	//
	// Why the generation guard survives the move: emitDetected and
	// emitCharacterized compare attackGen and emit under emitMu, and the tick
	// stages its Cleared under d.mu (advancing attackGen) but publishes it under
	// emitMu too. So a Cleared can never interleave between a stale check and its
	// emit -- it either wins the generation race (the check drops the emit) or
	// waits for emitMu and lands after. Either way ddos-local never keeps a drop
	// with no matching Cleared.
	//
	// What emitMu does NOT give back is the atomicity of a STATE WRITE with the
	// generation it belongs to: attackGen moves under d.mu, which the emitters
	// have released. Any state an emitter records after its fan-out must re-check
	// the generation under d.mu (markDetectedEmitted does, and it is the only
	// such write).
	//
	// LOCK ORDER: emitMu -> d.mu. Never take emitMu while holding d.mu.
	emitMu   sync.Mutex
	cfg      *Config
	bus      ze.EventBus
	dispatch dispatchFunc
	baseline *baseline
	// baselineBps is a parallel rolling baseline over bytes/sec, feeding the
	// bandwidth trigger. Same type and poisoning guard as the PPS baseline; its
	// floor is bytes/sec (cfg.BpsFloor is bits/sec, divided by 8 at construction).
	baselineBps *baseline
	// saveMu serializes baseline persistence so the periodic save (spawned off the
	// tick) never races the save-on-Stop on the same file.
	saveMu  sync.Mutex
	sm      *stateMachine
	tickNum int

	prevRxPackets map[string]uint64
	prevRxBytes   map[string]uint64
	currentRxPps  map[string]float64
	peakRxPps     float64
	peakRxBps     float64
	attackIface   string
	justTriggered bool
	bpsTriggered  bool // last active-transition tick was bandwidth-driven (PPS alone would not have fired)

	wg                   sync.WaitGroup
	sourceAbsentOnce     sync.Once // trafficusage (fast target) absent, logged once
	flowSourceAbsentOnce sync.Once // flowexport recent-flow (characterization) absent, logged once
	// detectedEmitted and attackGen are guarded by d.mu and MUST move together: a
	// plain bool, not an atomic, precisely so no writer can set the flag outside
	// the critical section that owns the generation. emitCleared advances the
	// generation and clears the flag; markDetectedEmitted sets it only while the
	// generation still matches. Setting it outside that pairing lets a Detected
	// whose fan-out spans a clear put the flag back to true for an attack that has
	// ended (see markDetectedEmitted).
	detectedEmitted bool
	attackGen       uint64 // bumped on each activate and clear; guards stale async emits

	// ctx bounds every characterization goroutine to the detector's lifetime;
	// Stop cancels it so in-flight source queries unwind promptly at shutdown or
	// reconfigure instead of running out their per-query timeout. stopped (under
	// d.mu) fences onAttackStart from spawning a new goroutine once Stop has begun
	// waiting, so wg.Go can never race wg.Wait.
	ctx     context.Context
	cancel  context.CancelFunc
	stopped bool

	// pending holds the events this tick decided to publish. Staged under d.mu,
	// emitted by emitPending after the tick releases it.
	pending pendingEmits
}

// pendingEmits carries the events a tick staged while holding d.mu. At most one
// of each per tick: the Ongoing branch runs only while the state machine is
// active, and the Cleared branch only on the transition out of it.
type pendingEmits struct {
	ongoing *ddosevent.AttackOngoing
	cleared *ddosevent.AttackCleared
}

func newDetector(cfg *Config, bus ze.EventBus, dispatch dispatchFunc) *detector {
	d := &detector{
		cfg:           cfg,
		bus:           bus,
		dispatch:      dispatch,
		baseline:      newBaseline(cfg.BaselineWindow, cfg.ThresholdMultiplier, cfg.AbsoluteFloor),
		baselineBps:   newBaseline(cfg.BaselineWindow, cfg.BpsThresholdMultiplier, cfg.BpsFloor/8.0),
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
	// Persist after in-flight work unwinds and (in production) after the caller has
	// unsubscribed, so the baselines are quiescent. Called on both reconfigure and
	// shutdown, so a config change also preserves the warmed baseline.
	d.saveBaseline()
}

// restore loads the persisted baselines from the shared zefs store into the live
// baselines so a restart/reconfigure resumes detection without re-warming over
// BaselineWindow. No-op when the store or key is absent or rejected (warm fresh).
// Called once after construction, before subscribing to rates.
func (d *detector) restore() {
	blob, ok := loadBaselines()
	if !ok {
		return
	}
	rp := d.baseline.restore(blob.Pps)
	rb := d.baselineBps.restore(blob.Bps)
	if rp || rb {
		logger().Info("ddos-detect: baseline restored from disk",
			"pps-ready", d.baseline.Ready(), "bps-ready", d.baselineBps.Ready())
	}
}

// saveBaseline persists both baselines. The snapshot is taken under d.mu; the file
// I/O runs off the lock so a slow disk never stalls the rate tick (R-4). Best-effort:
// saveBaselines is a no-op when no shared store is registered.
func (d *detector) saveBaseline() {
	d.mu.Lock()
	pps := d.baseline.snapshot()
	bps := d.baselineBps.snapshot()
	d.mu.Unlock()
	// Serialize concurrent saves (periodic vs on-stop) so they never overlap on the
	// store. Held only around the file I/O, never with d.mu, so the tick is free.
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	if err := saveBaselines(pps, bps); err != nil {
		logger().Debug("ddos-detect: baseline save failed", "error", err)
	}
}

// onRates accepts pre-computed per-interface rates from the trafficstat
// service, replacing the raw-counter diffing that onRate performs.
func (d *detector) onRates(entries []trafficstat.InterfaceEntry) {
	if !d.cfg.Enabled {
		return
	}
	// The fan-out runs after tickRates has released d.mu (see the emitMu field
	// doc). Splitting the locked part into its own function is what keeps the
	// unlock on a defer, so a later edit cannot add an early return that leaks it.
	d.emitPending(d.tickRates(entries))
}

func (d *detector) tickRates(entries []trafficstat.InterfaceEntry) pendingEmits {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.tickNum++

	// Track the peak packet rate and the peak bandwidth INDEPENDENTLY: on a
	// multi-interface box the amplified (high-BPS, moderate-PPS) interface is
	// often not the top-PPS one, so binding maxBps to the max-PPS interface would
	// blind the bandwidth trigger to exactly the amplification floods it exists to
	// catch. Each is attributed to its own interface.
	var maxPps, maxBps float64
	var maxPpsIface, maxBpsIface string
	for i := range entries {
		e := &entries[i]
		d.currentRxPps[e.Name] = e.RxPps

		if e.RxPps > maxPps {
			maxPps = e.RxPps
			maxPpsIface = e.Name
		}
		if e.RxBps > maxBps {
			maxBps = e.RxBps
			maxBpsIface = e.Name
		}
	}

	return d.applyTick(maxPps, maxBps, maxPpsIface, maxBpsIface)
}

func (d *detector) onRate(infos []iface.InterfaceInfo) {
	if !d.cfg.Enabled {
		return
	}
	// Fan out after tickInfos has released d.mu (see onRates).
	d.emitPending(d.tickInfos(infos))
}

func (d *detector) tickInfos(infos []iface.InterfaceInfo) pendingEmits {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.tickNum++

	var maxPps, maxBps float64
	var maxPpsIface, maxBpsIface string
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

		// Peak PPS and peak BPS are tracked per interface, independently: the
		// amplified interface need not be the top-PPS one (see onRates).
		if pps > maxPps {
			maxPps = pps
			maxPpsIface = info.Name
		}
		if bps > maxBps {
			maxBps = bps
			maxBpsIface = info.Name
		}
	}

	return d.applyTick(maxPps, maxBps, maxPpsIface, maxBpsIface)
}

// applyTick runs the baseline and the state machine for a single tick and
// returns the events the tick decided to publish. Caller must hold d.mu
// (tickRates / tickInfos do) and the result MUST reach emitPending after that
// lock is released: nothing is emitted here, because a fan-out under d.mu parks
// the detector behind the responders' kernel reconcile (see the emitMu field doc).
func (d *detector) applyTick(maxPps, maxBps float64, maxPpsIface, maxBpsIface string) pendingEmits {
	if d.tickNum <= d.cfg.StartupGrace {
		if maxPps < d.cfg.AbsoluteFloor*5 {
			// Drain like every other exit: returning the zero value here would
			// strand an event a previous tick staged. Inert today (nothing stages
			// across ticks), and the exit that breaks the discipline is the one a
			// later staging site will forget about.
			return d.drainPending()
		}
	}

	threshold := d.baseline.Threshold()
	ppsAbove := maxPps > threshold

	// Bandwidth trigger: catch low-PPS/high-bandwidth amplification (NTP/memcached/
	// CLDAP) that the PPS threshold misses. Gated on a ready BPS baseline so it never
	// fires during warm-up (bandwidth is more FP-prone than packet rate). bps-floor is
	// configured in bits/sec; the BPS baseline floor is bytes/sec (RxBps is bytes/sec),
	// so newDetector already divided it by 8.
	bpsAbove := false
	if d.cfg.BpsTriggerEnable && d.baselineBps.Ready() {
		bpsAbove = maxBps > d.baselineBps.Threshold()
	}
	above := ppsAbove || bpsAbove
	// Attribute a new detection to the bandwidth path only when the packet-rate path
	// would not have fired on its own (drives ze_ddos_detect_bps_trigger_total).
	d.bpsTriggered = bpsAbove && !ppsAbove

	st := d.sm.State()
	attacking := st == stateActive || st == stateClearing
	d.baseline.Add(maxPps, attacking || above)
	d.baselineBps.Add(maxBps, attacking || above)

	if above {
		if maxPps > d.peakRxPps {
			d.peakRxPps = maxPps
		}
		if maxBps > d.peakRxBps {
			d.peakRxBps = maxBps
		}
		// Attribute the incident to the interface that drove THIS detection so
		// characterization targets the right link: the bandwidth path points at the
		// amplified interface (which need not be the top-PPS one), the packet path
		// at the top-PPS interface. bpsTriggered means only the BPS path fired.
		if d.bpsTriggered {
			d.attackIface = maxBpsIface
		} else {
			d.attackIface = maxPpsIface
		}
	}

	d.justTriggered = false
	d.sm.Tick(above)

	if d.sm.State() == stateActive && !d.justTriggered && d.detectedEmitted {
		d.pending.ongoing = &ddosevent.AttackOngoing{
			Interface:  d.attackIface,
			Target:     ddosevent.VectorTuple{DstPrefix: netip.Prefix{}},
			CurrentPps: maxPps,
			CurrentBps: maxBps,
			Observable: true,
		}
	}

	// Periodic persistence: belt-and-braces against a hard crash between the
	// save-on-Stop points. Spawned off the tick -- saveBaseline re-acquires d.mu
	// after applyTick releases it, so file I/O never runs under the lock. Guarded
	// by d.stopped (mirrors onAttackStart) so it never races Stop's wg.Wait.
	if d.tickNum%baselineSaveInterval == 0 && !d.stopped {
		d.wg.Go(d.saveBaseline)
	}

	return d.drainPending()
}

// drainPending removes and returns the events this tick staged. Caller holds
// d.mu. Every exit from applyTick goes through it, so no staged event is left
// behind for a later tick to publish out of order.
func (d *detector) drainPending() pendingEmits {
	p := d.pending
	d.pending = pendingEmits{}
	return p
}

// emitPending publishes the events a tick staged. Caller must NOT hold d.mu:
// this is the fan-out, and it runs under emitMu only (see the emitMu field doc
// for why, and for how the generation guard survives).
func (d *detector) emitPending(p pendingEmits) {
	if p.ongoing == nil && p.cleared == nil {
		return
	}
	d.emitMu.Lock()
	defer d.emitMu.Unlock()
	if p.ongoing != nil {
		if _, err := ddosevent.Ongoing.Emit(d.bus, p.ongoing); err != nil {
			logger().Warn("ddos-detect: emit ongoing failed", "error", err)
		}
	}
	if p.cleared != nil {
		if _, err := ddosevent.Cleared.Emit(d.bus, p.cleared); err != nil {
			logger().Warn("ddos-detect: emit cleared failed", "error", err)
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
	if d.bpsTriggered {
		incBpsTrigger()
	}
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

// emitCleared advances the attack generation and STAGES the Cleared event. It
// runs under d.mu (the state machine calls it from applyTick), so it must not
// emit here: the tick publishes it through emitPending once d.mu is released.
// Advancing attackGen under d.mu while publishing under emitMu is what keeps the
// generation guard sound -- see the emitMu field doc.
func (d *detector) emitCleared() {
	d.attackGen++
	d.detectedEmitted = false
	d.pending.cleared = &ddosevent.AttackCleared{
		Interface:  d.attackIface,
		Target:     ddosevent.VectorTuple{DstPrefix: netip.Prefix{}},
		Observable: true,
	}
	d.peakRxPps = 0
	d.peakRxBps = 0
	d.attackIface = ""
}

// classifyDirection resolves whether the victim is an address this box terminates
// (local, INPUT hook) or one it forwards (remote, FORWARD hook), via the iface
// backend. An unresolved victim or a backend error yields remote -- the fail-safe,
// since a local INPUT drop cannot protect an address the box does not own.
func (d *detector) classifyDirection(victim netip.Prefix) ddosevent.Direction {
	if !victim.IsValid() {
		return ddosevent.DirectionRemote
	}
	local, err := iface.AddressIsLocal(victim.Addr())
	if err != nil {
		logger().Debug("ddos-detect: address-is-local lookup failed, assuming remote",
			"victim", victim, "error", err)
		return ddosevent.DirectionRemote
	}
	if local {
		return ddosevent.DirectionLocal
	}
	return ddosevent.DirectionRemote
}
