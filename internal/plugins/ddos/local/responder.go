// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- on-host nft drop responder

package local

import (
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/ddosevent"
)

// tableName carries the "ze_" ownership prefix the firewall backend uses to
// recognize ze-managed kernel tables (mirrors copp's "ze_copp" and the firewall
// engine's tableNamePrefix). Without it the kernel table is named "ddos-local":
// the backend's reconcile never sees it as ze-owned, so a cleared mitigation
// leaves the drop rule behind (removeMitigation registers nil + ApplyAll, but
// ApplyAll's shouldDeleteTable only sweeps ze_* names). It doubles as the
// registry owner key, which is internal so the rename is inert there.
const tableName = "ze_ddos-local"

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

type responder struct {
	mu  sync.Mutex
	cfg *Config
	bus eventBus
	// active, target and hook are guarded by mu. setStatus is their ONLY writer
	// once this responder is live, in production and in tests: it is what keeps
	// `published` in step with them. adoptMitigation is the exception at both
	// ends of a config apply: it seeds them here, with installedAt and the clock
	// beside them, from the responder the apply replaces, and it clears them on
	// that responder, which is how ownership of one rule stays with one
	// responder. It runs before this one is published.
	//
	// hook is the netfilter hook the live rule sits on, and it is read once the
	// rule exists: replaceResponder (register.go) needs it to tell a FORWARD drop,
	// which `forward-mitigation false` says to stop, from an INPUT drop, which
	// that leaf says nothing about.
	active bool
	target ddosevent.VectorTuple
	hook   firewall.ChainHook
	// retired says this responder is out of service: a config apply replaced it,
	// or the engine that owns it is stopping. Guarded by mu, and applyMitigation
	// refuses to install once it is set.
	//
	// It exists because an event dispatched BEFORE the unsubscribe still reaches
	// this responder AFTER it. The plugin server snapshots the handler list under
	// a read lock and invokes outside it, and its own contract says so: "register
	// and unregister take effect on the NEXT dispatch, not the current one"
	// ((*engineEventSubscribers).dispatch,
	// internal/component/plugin/server/engine_event.go). So a handler blocked on
	// mu across a config apply runs when the apply releases it, on a responder
	// nothing can reach any more. The two REMOVAL paths are already closed by
	// !active; the INSTALL is not, and a rule installed here has no owner, no cap
	// worker and no show handler left to reach it, which blackholes the victim
	// for the life of the daemon.
	retired bool
	// installedAt is when the live drop rule went in, and now is the clock that
	// stamped it. enforceMaxDuration compares the two against
	// cfg.MaxMitigationDuration. setStatus writes installedAt on the transition
	// into a live rule, so a refresh cannot restart the cap. now is a field so a
	// test can move time without sleeping; newResponder MUST set it, and does,
	// so clock() never has to answer for a nil. adoptMitigation is the second
	// writer of now, and it copies prev.now, which newResponder set on prev: the
	// invariant survives the copy because every responder in the chain came from
	// that constructor.
	installedAt time.Time
	now         func() time.Time
	// published mirrors {active, target} for readers that must not wait on mu.
	// mu is held across the whole firewall reconcile so concurrent mitigations
	// stay ordered, and that reconcile is a netlink round trip: a reader taking
	// mu waited it out. show ddos local is such a reader (show.go
	// handleShowDdosLocal -> status()), so a wedged kernel took the management
	// plane down with it. Written under mu by setStatus, read lock-free by
	// status(). Design: plan/spec-fixit-firewall-concurrency-deadlock.md D-3.
	published atomic.Pointer[mitigationStatus]
}

// mitigationStatus is one immutable snapshot of the responder's mitigation
// state. Never mutated after Store; a new value is published on every change.
type mitigationStatus struct {
	active bool
	target ddosevent.VectorTuple
}

type eventBus interface {
	Emit(namespace, eventType string, payload any) (int, error)
	Subscribe(namespace, eventType string, handler func(payload any)) (unsubscribe func())
}

var registerTables = firewall.RegisterTables
var applyAll = firewall.ApplyAll

func newResponder(cfg *Config, bus eventBus) *responder {
	r := &responder{cfg: cfg, bus: bus, now: time.Now}
	// Publish the idle snapshot before the responder is reachable, so status()
	// never has to interpret a nil pointer as "no mitigation".
	r.published.Store(&mitigationStatus{})
	return r
}

// setStatus records the mitigation state and republishes the lock-free snapshot
// status() reads. Caller holds r.mu.
//
// It is also the only writer of installedAt, the instant the
// max-mitigation-duration cap counts from. The write happens on the transition
// from no rule to a live rule and nowhere else: applyMitigation re-installs in
// place on every AttackCharacterized while the rule is already live, so a clock
// written on each install would restart the cap on every characterization and
// an attack that re-characterizes inside its own cap would never expire.
//
// hook travels with target because the two describe one rule: the caller that
// clears a mitigation passes r.hook back, so the pair keeps naming the rule that
// was last installed.
func (r *responder) setStatus(active bool, target ddosevent.VectorTuple, hook firewall.ChainHook) {
	if active && !r.active {
		r.installedAt = r.clock()
	}
	r.active = active
	r.target = target
	r.hook = hook
	r.published.Store(&mitigationStatus{active: active, target: target})
}

// adoptMitigation carries a live drop rule from the responder a config apply
// replaces into the one that replaces it, and leaves prev claiming nothing.
//
// Without the carry a config apply orphans the rule. The firewall registry is
// keyed by table name rather than by responder, so the kernel keeps the drop
// while the fresh responder believes there is none, and both removal paths
// return on !active: enforceMaxDuration and onCleared alike.
//
// One rule then has one owner at every instant, and that holds in BOTH
// directions only because replaceResponder retires prev in the same critical
// section: this function stops prev REMOVING the rule, and prev.retired stops it
// INSTALLING another one. Without the second half, a handler already in flight
// when the apply started could re-install after the handover, on a responder
// nothing can reach (the retired field, above).
//
// The cap clock comes across unchanged, and so does the time source that stamped
// it: an instant is only meaningful against the clock that produced it, and the
// rule is the same rule, so its age is the same age. Resetting it would let a box
// under attack renew its own cap on every unrelated commit. The new cfg governs
// from here, so a reload that shortens max-mitigation-duration applies the
// shorter cap to the rule already installed.
//
// Caller MUST call this before the new responder is published, on a responder
// that has installed nothing of its own, and MUST hold prev.mu from before this
// call until after the publish. The cap worker reads activeResponder, so a tick
// that loaded the old pointer before the swap runs on prev after it, and an
// AttackCleared dispatched before unsubscribe does the same; the lock holds both
// off until prev has stopped owning the rule, after which both find !active and
// return. replaceResponder (register.go) is the one caller. prev MAY be nil and
// MAY be idle, which are the first configure and the common case; a nil prev
// needs no lock.
func (r *responder) adoptMitigation(prev *responder) {
	if prev == nil {
		return
	}
	if !prev.active {
		return
	}

	r.mu.Lock()
	// Written here rather than through setStatus: setStatus stamps installedAt
	// with the current clock on the transition into a live rule, which is right
	// for an install this responder performs and wrong for one it inherits.
	r.active = true
	r.target = prev.target
	r.hook = prev.hook
	r.installedAt = prev.installedAt
	r.now = prev.now
	r.published.Store(&mitigationStatus{active: true, target: prev.target})
	r.mu.Unlock()

	// Ownership has moved, so prev stops claiming the rule. Both of its removal
	// paths return on !active, which is what keeps a late tick or a late clear
	// from taking out of the kernel a rule r has just published as live.
	prev.active = false
	prev.published.Store(&mitigationStatus{})
}

// withdrawReason is the whole log line a config-driven withdrawal writes. It is
// a constant rather than a phrase assembled at the call site, so each outcome
// has one stable string an operator can grep back to the commit that caused it
// (ai/rules/cli.md).
type withdrawReason string

const (
	withdrawSectionRemoved  withdrawReason = "ddos-local: the ddos local section was removed, removing the drop rule"
	withdrawNotEnforcing    withdrawReason = "ddos-local: response-level left enforce, removing the drop rule"
	withdrawForwardDisabled withdrawReason = "ddos-local: forward-mitigation was disabled, removing the drop rule"
	withdrawEngineStopped   withdrawReason = "ddos-local: the plugin is stopping, removing the drop rule"
)

// withdrawMitigation removes the drop rule this responder installed because the
// operator's new config says the box must stop dropping. A no-op when no rule is
// installed. Caller holds r.mu, as applyMitigation and removeMitigation do.
//
// It is the removal path for a config apply and for the engine's own stop,
// beside enforceMaxDuration for the cap and onCleared for the attack ending. Its
// two callers are in register.go: replaceResponder, on the OUTGOING responder
// because the incoming one has installed nothing, and stopResponder on the
// engine's exit path.
func (r *responder) withdrawMitigation(reason withdrawReason) {
	if !r.active {
		return
	}
	logger().Info(string(reason), "target", r.target.DstPrefix)
	r.removeMitigation()
}

// retire takes this responder out of service, so it can neither install a rule
// nor claim one, whatever reaches it afterwards. Caller holds r.mu.
//
// It is called once for each responder, at the moment the plugin stops being
// able to reach it: from replaceResponder for a responder a config apply
// replaces, and from stopResponder for the one the engine holds when it stops
// (both register.go). Ordering against a handler already in flight is settled by
// mu either way. A handler that wins the lock installs a rule this call then
// removes; a handler that loses it finds retired and installs nothing.
func (r *responder) retire() {
	r.retired = true
}

// onDetected installs the fast coarse drop for the victim (all traffic to the
// attacked destination). The box keeps observing the attack (packets are dropped
// on ingress), so onCharacterized can narrow the rule in place while still
// protecting.
func (r *responder) onDetected(e *ddosevent.AttackDetected) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.applyMitigation(e.Target, e.Family, e.Direction, e.SuppressMitigation, "detected")
}

// onCharacterized narrows the installed rule in place to the discriminating
// vector (proto / ports / TCP flags). It also installs from scratch when the
// coarse AttackDetected carried no valid prefix but characterization derived one
// from flow data -- so mitigation still engages.
func (r *responder) onCharacterized(e *ddosevent.AttackCharacterized) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Confidence gate (default 0 = disabled): suppress mitigation for a
	// low-confidence characterization so a borderline spike does not install a drop.
	// Only the characterized path is gated; the coarse onDetected carries no
	// confidence.
	if e.Confidence < r.cfg.ConfidenceMin {
		logger().Info("ddos-local: confidence below minimum, not mitigating",
			"target", e.Target.DstPrefix, "confidence", e.Confidence, "minimum", r.cfg.ConfidenceMin)
		return
	}
	r.applyMitigation(e.Target, e.Family, e.Direction, e.SuppressMitigation, "characterized")
}

// applyMitigation (re)installs the nft drop for target. Caller holds r.mu. Used
// by both the coarse (onDetected) and narrowed (onCharacterized) paths so the
// table is re-registered identically; the only difference is how surgical the
// term is.
func (r *responder) applyMitigation(target ddosevent.VectorTuple, family ddosevent.AttackFamily, direction ddosevent.Direction, suppressMitigation bool, phase string) {
	// A retired responder installs nothing. It reads its OWN cfg, which is the
	// config the operator has just replaced, so the checks below would answer for
	// a commit that no longer applies -- and the rule they installed would have no
	// owner, no cap worker and no show handler left to reach it. Logged rather
	// than silent, because a mitigation that does not happen is what an operator
	// comes looking for (the retired field, above).
	if r.retired {
		logger().Info("ddos-local: this responder was retired by a config apply or a plugin stop, not mitigating",
			"target", target.DstPrefix, "phase", phase)
		return
	}

	if r.cfg.ResponseLevel != responseEnforce {
		logger().Info("ddos-local: alert mode, would mitigate",
			"target", target.DstPrefix, "family", family, "phase", phase)
		return
	}

	if suppressMitigation {
		// The detector's traffic policy exempts this attack from the mitigation ACTION
		// (record-only). If the fast path already installed a drop, withdraw it -- the
		// characterized decision is authoritative.
		if r.active {
			r.removeMitigation()
		}
		logger().Info("ddos-local: policy exempts mitigation, not blocking",
			"target", target.DstPrefix, "phase", phase)
		return
	}

	hook, ok := r.hookForDirection(direction)
	if !ok {
		// Remote (transit) victim with forward-mitigation disabled: an on-host INPUT drop
		// cannot touch forwarded traffic, so leave this to the flowspec upstream announce.
		logger().Info("ddos-local: remote victim, forward-mitigation disabled, deferring to flowspec",
			"target", target.DstPrefix, "phase", phase)
		return
	}

	// Fail closed on an unresolved victim. Two things are derived from the victim
	// prefix and both degrade silently without it: familyFromPrefix GUESSES ip6 for
	// the zero prefix (netip.Addr{}.Is4() is false), and buildDropTerm emits NO
	// match at all for a zero VectorTuple. Together they render an unconditional
	// `counter drop` on a base hook in a guessed address family -- a blackhole for
	// every packet that hook sees, not a mitigation. An attack whose victim never
	// resolved must therefore install NOTHING and say so, leaving any drop a prior
	// phase already installed untouched. See ai/rules/evidence.md.
	if !target.DstPrefix.IsValid() {
		logger().Error("ddos-local: victim prefix unresolved, refusing to install a drop (an unscoped rule would blackhole the hook)",
			"phase", phase, "direction", direction, "hook", hookChainName(hook), "family", family)
		return
	}

	term := buildDropTerm("ddos-drop", target)
	table := firewall.Table{
		Name:   tableName,
		Family: familyFromPrefix(target.DstPrefix),
		Chains: []firewall.Chain{{
			Name:     hookChainName(hook),
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     hook,
			Priority: -200,
			Policy:   firewall.PolicyAccept,
			Terms:    []firewall.Term{term},
		}},
	}

	if err := registerTables(tableName, []firewall.Table{table}); err != nil {
		// The registry refused the table, so nothing was staged and there is
		// nothing to roll back. Report no live mitigation, as the apply-failure
		// path below does: r.active must never claim a rule the kernel lacks.
		r.setStatus(false, r.target, r.hook)
		logger().Error("ddos-local: failed to register the drop rule", "error", err, "phase", phase)
		return
	}
	if err := applyAll(); err != nil {
		// Roll the registry back to no ddos-local table and reconcile the kernel,
		// so a half-applied narrow does not leave the registry empty while the
		// kernel still holds the previous rule and r.active falsely claims a live
		// mitigation. Best-effort: log a second failure but do not spin.
		_ = registerTables(tableName, nil) // a withdraw registers no name, so it cannot be refused
		// A wedged kernel is the one case where re-reconciling is worse than
		// doing nothing: the registry rollback above has already made the
		// desired state correct, and a second apply can only burn another full
		// netlink deadline before failing the same way. The detector re-fires
		// about once a second, so spending two deadlines here would leave an
		// attack unmitigated for far longer than one. Registry state is right
		// either way; the kernel catches up on the next successful reconcile.
		if errors.Is(err, firewall.ErrKernelTimeout) {
			logger().Error("ddos-local: kernel wedged, skipping rollback reconcile", "error", err, "phase", phase)
		} else if rbErr := applyAll(); rbErr != nil {
			logger().Error("ddos-local: rollback after failed apply also failed", "error", rbErr, "phase", phase)
		}
		r.setStatus(false, r.target, r.hook)
		logger().Error("ddos-local: failed to apply drop rule", "error", err, "phase", phase)
		return
	}

	r.setStatus(true, target, hook)
	logger().Info("ddos-local: drop rule installed",
		"target", target.DstPrefix, "hook", hookChainName(hook), "phase", phase)
}

// hookForDirection maps the victim's direction to the netfilter hook that can actually
// drop its traffic: INPUT for a local (box-owned) victim, FORWARD for a remote/transit
// victim -- the latter only when forward-mitigation is enabled. An empty/unknown
// direction is treated as local (INPUT), harmless if the guess is wrong.
func (r *responder) hookForDirection(direction ddosevent.Direction) (firewall.ChainHook, bool) {
	if direction == ddosevent.DirectionRemote {
		if !r.cfg.ForwardMitigation {
			return 0, false
		}
		return firewall.HookForward, true
	}
	return firewall.HookInput, true
}

func hookChainName(hook firewall.ChainHook) string {
	if hook == firewall.HookForward {
		return "forward"
	}
	return "ingress"
}

func (r *responder) onCleared(_ *ddosevent.AttackCleared) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.active {
		return
	}
	r.removeMitigation()
}

// clock reads the responder's time source. newResponder is the only constructor
// and always sets it, so there is no nil to guard: a branch for one would make a
// zero-value responder look serviceable when it is not.
func (r *responder) clock() time.Time {
	return r.now()
}

// enforceMaxDuration removes a drop rule that has outlived
// max-mitigation-duration.
//
// removeMitigation has two other callers and neither is a timer: the suppress
// branch of applyMitigation, and onCleared. So without this an attack that
// never clears, or a detector that stops before it emits the clear, leaves an
// nftables drop installed for the life of the daemon, while the operator has
// set a cap that says it will not be.
//
// It is a wall-clock cap on purpose. The one case the cap exists for is the
// attack whose clear never arrives, so it cannot be counted in events the
// detector may stop sending.
//
// Called by the plugin's one worker (register.go startMaxDurationWorker).
func (r *responder) enforceMaxDuration() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.active {
		return
	}
	// Zero means no cap, as the leaf's description states and as the flowspec
	// twin implements. It is a guard, checked explicitly rather than falling out
	// of the arithmetic: a zero read as a deadline expires every rule on the
	// first tick and leaves the box unprotected under a flood.
	if r.cfg.MaxMitigationDuration <= 0 {
		return
	}
	limit := time.Duration(r.cfg.MaxMitigationDuration) * time.Second
	if r.clock().Sub(r.installedAt) < limit {
		return
	}
	logger().Info("ddos-local: max-mitigation-duration reached, removing the drop rule",
		"target", r.target.DstPrefix, "seconds", r.cfg.MaxMitigationDuration)
	r.removeMitigation()
}

// removeMitigation withdraws the drop rule and reports what the kernel did.
// Caller holds r.mu.
//
// The two outcomes are two log lines and never both. A failed reconcile used to
// be followed by "drop rule removed" on the next line, which reads as a success
// an operator can act on. Most callers have a later reconcile that repairs a
// failure -- the detector re-fires about once a second -- but the engine's exit
// path has none, so there the line is the only witness the operator ever gets
// (ai/rules/evidence.md).
//
// r.active is cleared either way, and the registry is empty either way, so the
// responder never claims a rule the desired state no longer holds. What survives
// a failure is the KERNEL's copy, which is what the error line names.
func (r *responder) removeMitigation() {
	_ = registerTables(tableName, nil) // a withdraw registers no name, so it cannot be refused
	err := applyAll()
	r.setStatus(false, r.target, r.hook)
	if err != nil {
		logger().Error("ddos-local: failed to remove drop rule, it is still in the kernel",
			"error", err, "target", r.target.DstPrefix)
		return
	}
	logger().Info("ddos-local: drop rule removed", "target", r.target.DstPrefix)
}

// status returns the published snapshot for the show handler: whether an on-host
// drop is currently installed and, if so, the target vector it covers. It takes
// NO lock on purpose -- r.mu is held across the firewall reconcile, so reading
// through it would make show ddos local wait out a netlink round trip.
func (r *responder) status() (active bool, target ddosevent.VectorTuple) {
	s := r.published.Load()
	if s == nil {
		// Unreachable: newResponder publishes the idle snapshot before the
		// responder is shared. Answering "no mitigation" for an unpublished
		// responder is the fail-closed reading -- it never claims a drop that
		// the kernel does not hold.
		return false, ddosevent.VectorTuple{}
	}
	return s.active, s.target
}

// familyFromPrefix maps the victim prefix to the nft table's address family.
//
// Caller MUST pass a valid prefix: the zero netip.Prefix answers Is4() false and
// would be reported as ip6, silently placing an IPv4 mitigation in an IPv6 table.
// applyMitigation's unresolved-victim guard is what makes that unreachable.
func familyFromPrefix(p netip.Prefix) firewall.TableFamily {
	if p.Addr().Is4() {
		return firewall.FamilyIP
	}
	return firewall.FamilyIP6
}
