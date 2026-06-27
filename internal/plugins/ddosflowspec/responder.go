// Design: plan/spec-cp-survival-5-detect-3-flowspec-responder.md -- upstream FlowSpec/RTBH responder

package ddosflowspec

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
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

var announceFunc = func(_ flowspecMatch, _ string) error {
	logger().Warn("ddos-flowspec: announce stub (cp-survival-4 not yet wired)")
	return nil
}

var withdrawFunc = func(_ flowspecMatch) error {
	logger().Warn("ddos-flowspec: withdraw stub (cp-survival-4 not yet wired)")
	return nil
}

type responder struct {
	mu     sync.Mutex
	cfg    *Config
	active bool
	target ddosevent.VectorTuple
	match  flowspecMatch
	probe  *probe
}

func newResponder(cfg *Config) *responder {
	return &responder{cfg: cfg}
}

func (r *responder) onDetected(e *ddosevent.AttackDetected) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cfg.ResponseLevel != "enforce" {
		logger().Info("ddos-flowspec: alert mode, would announce",
			"target", e.Target.DstPrefix, "family", e.Family)
		return
	}

	if !shouldAnnounce(e.Target, r.cfg.Allowlist) {
		logger().Info("ddos-flowspec: target allowlisted, skipping", "target", e.Target.DstPrefix)
		return
	}

	if r.active {
		return
	}

	r.match = buildMatch(e.Target)
	if err := announceFunc(r.match, r.cfg.Action); err != nil {
		logger().Error("ddos-flowspec: announce failed", "error", err)
		return
	}

	r.active = true
	r.target = e.Target
	r.probe = newProbe(r.cfg.HoldDown, r.cfg.ProbeInterval, r.cfg.ProbeWindow, r.cfg.ProbeRate, r.cfg.BackoffCap)
	r.probe.Start()
	logger().Info("ddos-flowspec: announced", "target", e.Target.DstPrefix, "action", r.cfg.Action)
}

func (r *responder) onCleared(_ *ddosevent.AttackCleared) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.active {
		return
	}
	logger().Info("ddos-flowspec: ignoring detector clear while mitigating (leak-probe decides)")
}

func (r *responder) probeTick(observedBps float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active || r.probe == nil {
		return
	}
	switch r.probe.Tick(observedBps) {
	case probeActionWithdraw:
		r.withdraw()
	case probeActionReTighten:
		logger().Info("ddos-flowspec: probe saturated, re-tightening", "target", r.target.DstPrefix)
	case probeActionNone, probeActionProbe:
	}
}

func (r *responder) withdraw() {
	if err := withdrawFunc(r.match); err != nil {
		logger().Error("ddos-flowspec: withdraw failed", "error", err)
	}
	r.active = false
	if r.probe != nil {
		r.probe.Stop()
	}
	logger().Info("ddos-flowspec: withdrawn", "target", r.target.DstPrefix)
}
