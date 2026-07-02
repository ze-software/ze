// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- upstream FlowSpec/RTBH responder

package flowspec

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

// blackholeAction is the flowspec traffic action for the critical-severity
// fallback: discard everything to the victim (RTBH-style), engaged without
// waiting for characterization.
const blackholeAction = "discard"

// onDetected does NOT announce in the normal case (AC-8): announcing upstream
// blinds the box behind the filter, so the rule must be precise -- flowspec waits
// for AttackCharacterized ("get it right once"). The blackhole-fallback policy is
// the sole exception (AC-14): a critical fast signal engages an immediate discard.
func (r *responder) onDetected(e *ddosevent.AttackDetected) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cfg.ResponseLevel != responseEnforce || r.active {
		return
	}
	if !r.cfg.BlackholeFallback || e.Severity != ddosevent.SeverityCritical {
		logger().Info("ddos-flowspec: awaiting characterization before announcing",
			"target", e.Target.DstPrefix, "severity", e.Severity)
		return
	}
	if !shouldAnnounce(e.Target, r.cfg.Allowlist) {
		logger().Info("ddos-flowspec: target allowlisted, skipping", "target", e.Target.DstPrefix)
		return
	}
	r.announce(e.Target, blackholeAction, "blackhole-fallback (critical)")
}

// onCharacterized announces exactly one precise upstream rule from the narrowed
// vector (AC-7). If a blackhole fallback already fired we are blind behind the
// filter and cannot refine, so the existing rule is kept.
func (r *responder) onCharacterized(e *ddosevent.AttackCharacterized) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cfg.ResponseLevel != responseEnforce {
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
	r.announce(e.Target, r.cfg.Action, "characterized")
}

// announce builds the flowspec match for target, announces it with action, and
// starts the leak-probe. Caller holds r.mu and has already checked
// enforce/allowlist/!active.
func (r *responder) announce(target ddosevent.VectorTuple, action, reason string) {
	r.match = buildMatch(target)
	if err := announceFunc(r.match, action); err != nil {
		logger().Error("ddos-flowspec: announce failed", "error", err)
		return
	}
	r.active = true
	r.target = target
	r.probe = newProbe(r.cfg.HoldDown, r.cfg.ProbeInterval, r.cfg.ProbeWindow, r.cfg.ProbeRate, r.cfg.BackoffCap)
	r.probe.Start()
	logger().Info("ddos-flowspec: announced",
		"target", target.DstPrefix, "action", action, "reason", reason)
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

// status returns a mutex-safe snapshot for the show handler: whether an upstream
// FlowSpec rule is currently announced, the target vector it covers, and whether
// the leak-probe is running.
func (r *responder) status() (active bool, target ddosevent.VectorTuple, probing bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active, r.target, r.active && r.probe != nil
}
