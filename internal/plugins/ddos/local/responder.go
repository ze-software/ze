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
	// active and target are guarded by mu. setStatus is their ONLY writer, in
	// production and in tests: it is what keeps `published` in step with them.
	active bool
	target ddosevent.VectorTuple
	// installedAt is when the live drop rule went in, and now is the clock that
	// reads it. enforceMaxDuration compares the two against
	// cfg.MaxMitigationDuration. setStatus writes installedAt on the transition
	// into a live rule, so a refresh cannot restart the cap. now is a field so a
	// test can move time without sleeping; production gets time.Now from
	// newResponder.
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
func (r *responder) setStatus(active bool, target ddosevent.VectorTuple) {
	if active && !r.active {
		r.installedAt = r.clock()
	}
	r.active = active
	r.target = target
	r.published.Store(&mitigationStatus{active: active, target: target})
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
		r.setStatus(false, r.target)
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
		r.setStatus(false, r.target)
		logger().Error("ddos-local: failed to apply drop rule", "error", err, "phase", phase)
		return
	}

	r.setStatus(true, target)
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

// clock reads the responder's time source. A zero-value responder leaves now
// nil; newResponder sets time.Now.
func (r *responder) clock() time.Time {
	if r.now == nil {
		return time.Now()
	}
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

func (r *responder) removeMitigation() {
	_ = registerTables(tableName, nil) // a withdraw registers no name, so it cannot be refused
	if err := applyAll(); err != nil {
		logger().Error("ddos-local: failed to remove drop rule", "error", err)
	}
	logger().Info("ddos-local: drop rule removed", "target", r.target.DstPrefix)
	r.setStatus(false, r.target)
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
