// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- two-stage DDoS detector

package detect

import (
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
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
}

func newDetector(cfg *Config, bus ze.EventBus) *detector {
	d := &detector{
		cfg:           cfg,
		bus:           bus,
		baseline:      newBaseline(cfg.BaselineWindow, cfg.ThresholdMultiplier, cfg.AbsoluteFloor),
		prevRxPackets: make(map[string]uint64),
		prevRxBytes:   make(map[string]uint64),
		currentRxPps:  make(map[string]float64),
	}
	d.sm = newStateMachine(cfg.ConfirmDuration, cfg.ClearConsecutive)
	d.sm.OnDetected = d.emitDetected
	d.sm.OnCleared = d.emitCleared
	return d
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

	if d.sm.State() == stateActive && !d.justTriggered {
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

func (d *detector) emitDetected() {
	d.justTriggered = true
	if _, err := ddosevent.Detected.Emit(d.bus, &ddosevent.AttackDetected{
		Interface:  d.attackIface,
		Target:     ddosevent.VectorTuple{DstPrefix: netip.Prefix{}},
		Family:     ddosevent.FamilyGenericFlood,
		PeakRxPps:  d.peakRxPps,
		PeakRxBps:  d.peakRxBps,
		Observable: true,
	}); err != nil {
		logger().Warn("ddos-detect: emit detected failed", "error", err)
	}
}

func (d *detector) emitCleared() {
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
