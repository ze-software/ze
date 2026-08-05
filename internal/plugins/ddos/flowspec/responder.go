// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- upstream FlowSpec/RTBH responder

package flowspec

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// routeDispatcher sends a rendered update-text command to the BGP engine. The
// production implementation (register.go) wraps the plugin SDK's UpdateRoute;
// tests inject a fake. The responder is a separate process and cannot reach the
// in-process tag registry, so origination goes through the update-text path.
type routeDispatcher interface {
	Dispatch(command string) error
}

// flowspecSelector announces to all peers; the engine only sends flow NLRI to
// peers that negotiated the flow family, so "*" self-scopes to flow-capable
// neighbors without a dedicated upstream config leaf.
const flowspecSelector = "*"

// tcpFlagBits maps each TCP-flag bit to its canonical FlowSpec token, low bit
// first. The parser (parseFlowTCPFlagMatches) accepts symbolic names ONLY -- no
// numeric form -- so tcp-flags MUST render as names, AND-joined with '&'.
var tcpFlagBits = []struct {
	bit  uint8
	name string
}{
	{0x01, "fin"}, {0x02, "syn"}, {0x04, "rst"}, {0x08, "psh"},
	{0x10, "ack"}, {0x20, "urg"}, {0x40, "ece"}, {0x80, "cwr"},
}

// renderFlowspecCommand builds the update-text command for one flowspec rule.
// mode is "add" or "del"; "del" omits the traffic-action extended community
// because the flowspec key is the NLRI alone, so a withdraw re-rendered from the
// same match byte-matches the announced components. Grammar is pinned against
// nlri/flowspec/config_builder.go and internal/exabgp/bridge/bridge_test.go.
func renderFlowspecCommand(m flowspecMatch, action string, rateBytes uint64, mode string) string {
	var b textbuf.Buffer
	b.Str("update text")
	if mode == "add" {
		rate := rateBytes
		if action == blackholeAction { // discard == traffic-rate 0
			rate = 0
		}
		b.Str(" extended-community [rate-limit:").Uint(rate).Byte(']')
		// A next-hop is REQUIRED for the FlowSpec MP_REACH_NLRI: ze drops an
		// origination with no next-hop before the wire (proven by
		// test/plugin/ddos-flowspec-announce.ci and the interop scenarios). The
		// action lives in the ext-community, so "self" is the correct originator
		// next-hop. Withdraw (MP_UNREACH) needs none.
		b.Str(" nhop self")
	}
	fam := "ipv4/flow"
	if m.DstPrefix.Addr().Is6() {
		fam = "ipv6/flow"
	}
	b.Str(" nlri ").Str(fam).Byte(' ').Str(mode)
	b.Str(" destination ").Str(m.DstPrefix.String())
	if m.Proto != 0 {
		b.Str(" protocol =").Uint(uint64(m.Proto))
	}
	if m.DstPort != 0 {
		b.Str(" destination-port =").Uint(uint64(m.DstPort))
	}
	if m.SrcPort != 0 {
		b.Str(" source-port =").Uint(uint64(m.SrcPort))
	}
	if m.TCPFlags != 0 {
		b.Str(" tcp-flags ")
		first := true
		for _, f := range tcpFlagBits {
			if m.TCPFlags&f.bit != 0 {
				if !first {
					b.Byte('&')
				}
				b.Str(f.name)
				first = false
			}
		}
	}
	return b.String()
}

type responder struct {
	mu         sync.Mutex
	cfg        *Config
	dispatcher routeDispatcher
	// active, target and probe are guarded by mu. setAnnouncement is their ONLY
	// writer, in production and in tests: it is what keeps `published` in step
	// with them (mirrors ddos/local setStatus and anomaly/shape gauge).
	active bool
	target ddosevent.VectorTuple
	match  flowspecMatch
	probe  *probe
	// published mirrors {active, target, probing} for readers that must not wait
	// on mu. mu is held across the announce and the withdraw so concurrent
	// mitigations stay ordered, and each of those is a dispatcher round trip:
	// the production dispatcher (register.go sdkDispatcher) sends the update
	// text to the BGP engine over the plugin SDK's UpdateRoute RPC, so a reader
	// taking mu waited that RPC out. show ddos flowspec is such a reader
	// (show.go handleShowDdosFlowspec -> status()), so a slow engine took the
	// management plane's read down with it, unbounded while a flood churns
	// announce and withdraw. Written under mu by setAnnouncement, read lock-free
	// by status(). Same defect and fix shape as D-3 (ddos/local) and D-4
	// (anomaly/shape) of plan/spec-fixit-firewall-concurrency-deadlock.md.
	published atomic.Pointer[announceStatus]
}

// announceStatus is one immutable snapshot of the responder's upstream
// announcement state. Never mutated after Store; a new value is published on
// every change.
type announceStatus struct {
	active  bool
	target  ddosevent.VectorTuple
	probing bool
}

func newResponder(cfg *Config, dispatcher routeDispatcher) *responder {
	r := &responder{cfg: cfg, dispatcher: dispatcher}
	// Publish the idle snapshot before the responder is reachable, so status()
	// never has to interpret a nil pointer as "no announcement".
	r.setAnnouncement(false, ddosevent.VectorTuple{}, nil)
	return r
}

// setAnnouncement records the announcement state and republishes the lock-free
// snapshot status() reads. It is the ONLY writer of active, target and probe, so
// the snapshot cannot fall out of step with them: an announce or a withdraw that
// forgets to republish is not writable. It derives probing here, under mu, so
// status() never needs the lock to compute it. Caller holds r.mu.
func (r *responder) setAnnouncement(active bool, target ddosevent.VectorTuple, p *probe) {
	r.active = active
	r.target = target
	r.probe = p
	r.published.Store(&announceStatus{
		active:  active,
		target:  target,
		probing: active && p != nil,
	})
}

// blackholeAction is the flowspec traffic action for the critical-severity
// fallback: discard everything to the victim (RTBH-style), engaged without
// waiting for characterization.
const blackholeAction = actionDiscard

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
	if e.SuppressMitigation {
		logger().Info("ddos-flowspec: policy exempts mitigation, not announcing", "target", e.Target.DstPrefix)
		return
	}
	if e.Direction == ddosevent.DirectionLocal {
		logger().Info("ddos-flowspec: local victim, leaving to on-host mitigation", "target", e.Target.DstPrefix)
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
	if e.SuppressMitigation {
		// The detector's traffic policy exempts this attack from the mitigation
		// ACTION (record-only). If the blackhole fallback already fired on
		// AttackDetected, withdraw it: the characterized decision is
		// authoritative, and an exempted destination must not stay blackholed
		// upstream because a faster, blinder path got there first.
		//
		// local.applyMitigation has always done this. Flowspec returned instead,
		// so the two responders disagreed and only the upstream one leaked.
		if r.active {
			r.withdraw()
		}
		logger().Info("ddos-flowspec: policy exempts mitigation, not announcing", "target", e.Target.DstPrefix)
		return
	}
	if e.Direction == ddosevent.DirectionLocal {
		// Same reason as the exemption above, reached a different way. Detect
		// classifies direction from the raw target prefix; characterization
		// re-classifies from the NARROWED victim (detect/characterize.go), so a
		// /24 that looked remote can narrow to a box-owned /32 and flip Remote
		// to Local after the blackhole fallback is already announced. On-host
		// mitigation owns it from here, and the upstream rule must not outlive
		// the classification that justified it.
		if r.active {
			r.withdraw()
		}
		logger().Info("ddos-flowspec: local victim, leaving to on-host mitigation", "target", e.Target.DstPrefix)
		return
	}
	if r.active {
		return
	}
	// Confidence gate (default 0 = disabled): do not announce an upstream rule for a
	// low-confidence characterization. The blackhole-fallback fast path (onDetected)
	// is never gated -- it acts on AttackDetected, which carries no confidence.
	if e.Confidence < r.cfg.ConfidenceMin {
		logger().Info("ddos-flowspec: confidence below minimum, not announcing",
			"target", e.Target.DstPrefix, "confidence", e.Confidence, "minimum", r.cfg.ConfidenceMin)
		return
	}
	r.announce(e.Target, r.cfg.Action, "characterized")
}

// announce builds the flowspec match for target, announces it with action, and
// starts the leak-probe. Caller holds r.mu and has already checked
// enforce/allowlist/!active.
func (r *responder) announce(target ddosevent.VectorTuple, action, reason string) {
	r.match = buildMatch(target)
	cmd := renderFlowspecCommand(r.match, action, r.cfg.RateLimitBytes, "add")
	if err := r.dispatcher.Dispatch(cmd); err != nil {
		logger().Error("ddos-flowspec: announce failed", "error", err)
		return
	}
	p := newProbe(r.cfg.HoldDown, r.cfg.ProbeInterval, r.cfg.ProbeWindow, r.cfg.ProbeRate, r.cfg.BackoffCap)
	p.Start()
	r.setAnnouncement(true, target, p)
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
	cmd := renderFlowspecCommand(r.match, "", 0, "del")
	if err := r.dispatcher.Dispatch(cmd); err != nil {
		logger().Error("ddos-flowspec: withdraw failed", "error", err)
	}
	if r.probe != nil {
		r.probe.Stop()
	}
	// The probe is dropped, not merely stopped: announce always builds a fresh
	// one, and probeTick reads it only while active. Keeping the target lets the
	// log line and a post-withdraw read still name what was announced.
	r.setAnnouncement(false, r.target, nil)
	logger().Info("ddos-flowspec: withdrawn", "target", r.target.DstPrefix)
}

// status returns the published snapshot for the show handler: whether an
// upstream FlowSpec rule is currently announced, the target vector it covers,
// and whether the leak-probe is running. It takes NO lock on purpose -- r.mu is
// held across the announce and the withdraw, each a dispatcher round trip to the
// BGP engine, so reading through it would make show ddos flowspec wait out an
// UpdateRoute RPC.
func (r *responder) status() (active bool, target ddosevent.VectorTuple, probing bool) {
	s := r.published.Load()
	if s == nil {
		// Unreachable: newResponder publishes the idle snapshot before the
		// responder is shared. Answering "nothing announced" for an unpublished
		// responder is the fail-closed reading -- it never claims an upstream
		// rule the BGP engine does not hold.
		return false, ddosevent.VectorTuple{}, false
	}
	return s.active, s.target, s.probing
}
