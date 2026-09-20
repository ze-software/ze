// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- upstream FlowSpec/RTBH responder

package flowspec

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

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
	// The family and the component keyword answer the same question, so they are
	// decided together: a prefix component names its family in ze's vocabulary.
	fam := "ipv4/flow"
	destKeyword := "destination-ipv4"
	if m.DstPrefix.Addr().Is6() {
		fam = "ipv6/flow"
		destKeyword = "destination-ipv6"
	}
	b.Str(" nlri ").Str(fam).Byte(' ').Str(mode)
	b.Byte(' ').Str(destKeyword).Byte(' ').Str(m.DstPrefix.String())
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
	// retired says this responder is out of service: a config apply replaced it,
	// so it is no longer in activeResponder and no longer subscribed. It must
	// announce nothing from here, because the responder that replaced it owns
	// whatever rule is on the wire. Guarded by mu, written by retire.
	retired bool
	// limiter enforces announce-rate-limit. Guarded by mu, which announce holds.
	limiter announceLimiter
	// announcedAt is when the live announce went out, and now is the clock that
	// reads it. enforceMaxDuration compares the two against
	// cfg.MaxMitigationDuration. now is a field so a test can move time without
	// sleeping; production leaves it nil and gets time.Now.
	announcedAt time.Time
	now         func() time.Time
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

// announceWindow is the period announce-rate-limit is stated over. The leaf is
// "announcements per minute", so the window is the minute it names.
const announceWindow = time.Minute

// announceLimiter enforces announce-rate-limit: at most limit announcements in
// any announceWindow. It holds the announce times still inside the window, so
// its memory is bounded by the leaf's own maximum of 600 entries.
//
// What the limit protects is upstream, not local. A FlowSpec announce goes to
// every peer that carries the family, and a mitigation that churns announce and
// withdraw pushes that churn onto sessions Ze does not own. A peer that drops
// the session over it loses every rule it holds, not only the churning one.
//
// Not safe for concurrent use: the responder holds mu across allow.
type announceLimiter struct {
	limit  int
	recent []time.Time // announce times inside the window, oldest first
}

// newAnnounceLimiter builds the limiter for an announce-rate-limit of limit.
//
// Config.Validate refuses the leaf outside 1 to 600, so a non-positive limit can
// only reach here from a Config that skipped Validate. It takes the documented
// default: a budget of none would refuse every announce and disable upstream
// mitigation entirely, which reads as a broken responder rather than as a field
// nobody set (ai/rules/principles.md).
func newAnnounceLimiter(limit int) announceLimiter {
	if limit < 1 {
		limit = DefaultConfig().AnnounceRateLimit
	}
	return announceLimiter{limit: limit, recent: make([]time.Time, 0, limit)}
}

// adopt carries the announce times prev still holds inside the window into this
// limiter, so a config apply hands the operator the budget that was already
// spent rather than a fresh one.
//
// Without it, announce-rate-limit is a limit an operator walks around by
// committing: every apply builds a new responder, and a new responder with an
// empty window can announce the full budget again immediately. What the limit
// protects is the upstream peer's session, and a peer does not care which
// responder generated the churn.
//
// The LIMIT itself is not carried: the new config's announce-rate-limit governs
// from here, exactly as its max-mitigation-duration does. Only the record of
// what has already gone out crosses.
func (l *announceLimiter) adopt(prev announceLimiter) {
	l.recent = append(l.recent[:0], prev.recent...)
}

// allow reports whether an announce at now is inside the limit, and records it
// when it is. A refusal consumes no budget, so the next announce after the
// window rolls goes out.
func (l *announceLimiter) allow(now time.Time) bool {
	cutoff := now.Add(-announceWindow)
	kept := l.recent[:0]
	for _, at := range l.recent {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	l.recent = kept

	if len(l.recent) >= l.limit {
		return false
	}
	l.recent = append(l.recent, now)
	return true
}

func newResponder(cfg *Config, dispatcher routeDispatcher) *responder {
	r := &responder{
		cfg:        cfg,
		dispatcher: dispatcher,
		limiter:    newAnnounceLimiter(cfg.AnnounceRateLimit),
		now:        time.Now,
	}
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

// adoptAnnouncement carries a live upstream announcement from the responder a
// config apply replaces into the one that replaces it, and leaves prev claiming
// nothing.
//
// Without the carry a config apply orphans the announcement. The FlowSpec NLRI
// lives in the BGP engine's RIB and in every peer's, neither of which belongs to
// this plugin, so the rule outlives the responder that announced it. A fresh
// idle responder believes there is no announcement, and every removal path
// returns on !active: enforceMaxDuration, probeTick and onCleared alike. The
// peer would then hold a discard rule for the victim for the life of the daemon
// while `show ddos flowspec` reported nothing announced.
//
// What crosses, and why each one:
//
//   - active, target and match ARE the rule. match is what withdraw renders into
//     the MP_UNREACH, so a responder that carried active without it would send a
//     withdraw for a prefix nobody announced.
//   - announcedAt and now cross together. An instant is only meaningful against
//     the clock that produced it, and the rule is the same rule, so its age is
//     the same age. Resetting it would let a box under attack renew its own cap
//     on every unrelated commit. The new cfg governs from here, so a commit that
//     shortens max-mitigation-duration applies the shorter cap at once.
//   - the probe crosses as the running state machine, not as a fresh one built
//     from the new config. It is mid hold-down or mid probe window for THIS
//     announcement, and restarting it would push the withdrawal out by a whole
//     hold-down on every commit -- the same self-renewal the cap clock avoids.
//     Its timings are the ones the announcement was made under; the new config
//     builds the probe for the next announcement.
//
// The new cfg's action and rate-limit-bytes do NOT reach the rule already on the
// wire: nothing re-announces, so the announcement keeps the action it was made
// with until a withdrawal ends it.
//
// One rule then has one owner at every instant, and that holds in BOTH
// directions only because replaceResponder retires prev in the same critical
// section: this function stops prev WITHDRAWING the rule, and prev.retired stops
// it announcing another one over the top.
//
// Caller MUST call this before the new responder is published, on a responder
// that has announced nothing of its own, and MUST hold prev.mu from before this
// call until after the publish. The cap worker reads activeResponder, so a tick
// that loaded the old pointer before the swap runs on prev after it, and an
// AttackOngoing dispatched before unsubscribe does the same; the lock holds both
// off until prev has stopped owning the rule, after which both find !active and
// return. replaceResponder (register.go) is the one caller. prev MAY be nil,
// which is the first configure.
func (r *responder) adoptAnnouncement(prev *responder) {
	if prev == nil {
		return
	}

	// The budget crosses whether or not a rule is live: it records announcements
	// that already went to the peers, and a commit is not a reason to forget one.
	r.limiter.adopt(prev.limiter)

	if !prev.active {
		return
	}

	r.mu.Lock()
	// announcedAt and now are written directly because setAnnouncement does not
	// own them; it owns active, target and probe, and it stays their only writer.
	r.announcedAt = prev.announcedAt
	r.now = prev.now
	r.match = prev.match
	r.setAnnouncement(true, prev.target, prev.probe)
	r.mu.Unlock()

	// Ownership has moved, so prev stops claiming the rule. Every one of its
	// removal paths returns on !active, which is what keeps a late cap tick or a
	// late probe tick from withdrawing a rule r has just published as live. The
	// target is kept so a post-handover log line still names what was announced.
	prev.setAnnouncement(false, prev.target, nil)
}

// withdrawReason is the whole log line a config-driven withdrawal writes. It is
// a constant rather than a phrase assembled at the call site, so each outcome
// has one stable string an operator can grep back to the commit that caused it
// (ai/rules/cli.md).
type withdrawReason string

const (
	withdrawSectionRemoved withdrawReason = "ddos-flowspec: the ddos flowspec section was removed, withdrawing the announcement"
	withdrawNotEnforcing   withdrawReason = "ddos-flowspec: response-level left enforce, withdrawing the announcement"
)

// withdrawForConfig withdraws the live announcement because the operator's new
// config says the box must stop announcing. A no-op when nothing is announced.
// Caller holds r.mu, as replaceResponder does.
//
// It is the removal path for a config apply, beside enforceMaxDuration for the
// cap and probeTick for the attack ending. replaceResponder (register.go) is its
// one caller, and it calls it on the OUTGOING responder because the incoming one
// has announced nothing and its own removal paths return on !active.
func (r *responder) withdrawForConfig(reason withdrawReason) {
	if !r.active {
		return
	}
	logger().Info(string(reason), "target", r.target.DstPrefix)
	r.withdraw()
}

// retire takes this responder out of service, so it can neither announce a rule
// nor claim one, whatever reaches it afterwards. Caller holds r.mu.
//
// It is called once for each responder, at the moment the plugin stops being
// able to reach it: from replaceResponder (register.go), for the responder a
// config apply replaces. Ordering against a handler already in flight is settled
// by mu either way. A handler that wins the lock announces a rule this call's
// caller then withdraws; a handler that loses it finds retired and announces
// nothing.
func (r *responder) retire() {
	r.retired = true
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
//
// It is the ONE place an announcement leaves the plugin, which is why
// announce-rate-limit is enforced here: the blackhole fallback and the
// characterized path both arrive through it, and a limit either path could walk
// around is not a limit.
func (r *responder) announce(target ddosevent.VectorTuple, action, reason string) {
	// A retired responder announces nothing. It reads its OWN cfg, which a config
	// apply has already replaced, and the responder that replaced it owns
	// whatever rule is on the wire. An event dispatched before the apply
	// unsubscribed is exactly the holder this guard exists for: it wins prev.mu
	// after the handover, and without this it would put a second FlowSpec NLRI on
	// the wire that nothing left in the plugin can withdraw (the retired field,
	// above).
	if r.retired {
		logger().Info("ddos-flowspec: this responder was retired by a config apply, not announcing",
			"target", target.DstPrefix, "reason", reason)
		return
	}
	// A FlowSpec rule IS its destination prefix, so there is no rule to write for
	// an attack whose victim was never resolved. The detector emits exactly that
	// when no traffic source can name a victim (characterizeAndEmit leaves the
	// target empty), and blackhole-fallback then reaches here on the critical
	// severity. renderFlowspecCommand writes the prefix through Prefix.String(),
	// which is "invalid" for the zero value, and the engine answered
	// `ipv4/flow encode: invalid prefix` on every one of them.
	if !target.DstPrefix.IsValid() {
		logger().Warn("ddos-flowspec: no victim resolved, not announcing",
			"reason", reason, "effect", "no upstream rule; on-host mitigation and the detector are unaffected")
		return
	}
	if !r.limiter.allow(r.clock()) {
		// The responder stays idle, exactly as it does when Dispatch fails, so a
		// later AttackCharacterized announces once the window frees. It never
		// claims a rule the BGP engine does not hold.
		logger().Warn("ddos-flowspec: announce-rate-limit reached, not announcing",
			"target", target.DstPrefix, "announcements-per-minute", r.cfg.AnnounceRateLimit,
			"reason", reason)
		return
	}
	r.match = buildMatch(target)
	cmd := renderFlowspecCommand(r.match, action, r.cfg.RateLimitBytes, "add")
	if err := r.dispatcher.Dispatch(cmd); err != nil {
		logger().Error("ddos-flowspec: announce failed", "error", err)
		return
	}
	p := newProbe(r.cfg.HoldDown, r.cfg.ProbeInterval, r.cfg.ProbeWindow, r.cfg.ProbeRate, r.cfg.BackoffCap)
	p.Start()
	r.announcedAt = r.clock()
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

// clock reads the responder's time source. Production leaves now nil at
// construction only in a zero-value responder; newResponder sets time.Now.
func (r *responder) clock() time.Time {
	if r.now == nil {
		return time.Now()
	}
	return r.now()
}

// onOngoing feeds the leak probe its input.
//
// This is the probe's ONLY production driver. Without it probe.Tick was never
// called outside tests, and because onCleared deliberately ignores the
// detector's clear while mitigating ("leak-probe decides"), nothing withdrew a
// flowspec announce at all: the rule stayed on the wire until an operator
// removed it by hand.
//
// AttackOngoing is the right source rather than a synthetic ticker. It carries
// CurrentBps measured behind the filter, which is exactly what probeStateProbing
// compares against probe-rate, and the detector already emits it on its own
// cadence for every plugin that wants it (ddos/flowtriq and ddos/observe
// subscribe today). Driving the probe from a bare timer would have to invent an
// observed rate, and inventing 0 lifts mitigation during an active attack.
func (r *responder) onOngoing(e *ddosevent.AttackOngoing) {
	r.probeTick(e.CurrentBps)
}

// enforceMaxDuration withdraws an announce that has outlived
// max-mitigation-duration.
//
// The YANG for both ddos plugins promises this and defaults it to 3600, so
// every operator was told a live rule lifts after an hour. Neither plugin read
// the leaf. A validated config leaf that does nothing is a promise the box does
// not keep.
//
// Zero means no cap, as the YANG description states. It is checked explicitly
// rather than falling out of the arithmetic, because a zero read as a deadline
// would expire every announce on its first tick.
//
// It is a wall-clock cap on purpose, not a probe-tick count: the cap must fire
// when the attack goes quiet and AttackOngoing STOPS arriving, which is the case
// the probe cannot see.
func (r *responder) enforceMaxDuration() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.active || r.cfg.MaxMitigationDuration <= 0 {
		return
	}
	limit := time.Duration(r.cfg.MaxMitigationDuration) * time.Second
	if r.clock().Sub(r.announcedAt) < limit {
		return
	}
	logger().Info("ddos-flowspec: max-mitigation-duration reached, withdrawing",
		"target", r.target.DstPrefix, "seconds", r.cfg.MaxMitigationDuration)
	r.withdraw()
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
