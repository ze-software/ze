// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- on-host nft drop responder

package local

import (
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"

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
	mu     sync.Mutex
	cfg    *Config
	bus    eventBus
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
	return &responder{cfg: cfg, bus: bus}
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
	// phase already installed untouched. See ai/rules/fail-closed-guards.md.
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

	registerTables(tableName, []firewall.Table{table})
	if err := applyAll(); err != nil {
		// Roll the registry back to no ddos-local table and reconcile the kernel,
		// so a half-applied narrow does not leave the registry empty while the
		// kernel still holds the previous rule and r.active falsely claims a live
		// mitigation. Best-effort: log a second failure but do not spin.
		registerTables(tableName, nil)
		if rbErr := applyAll(); rbErr != nil {
			logger().Error("ddos-local: rollback after failed apply also failed", "error", rbErr, "phase", phase)
		}
		r.active = false
		logger().Error("ddos-local: failed to apply drop rule", "error", err, "phase", phase)
		return
	}

	r.active = true
	r.target = target
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

func (r *responder) removeMitigation() {
	registerTables(tableName, nil)
	if err := applyAll(); err != nil {
		logger().Error("ddos-local: failed to remove drop rule", "error", err)
	}
	r.active = false
	logger().Info("ddos-local: drop rule removed", "target", r.target.DstPrefix)
}

// status returns a mutex-safe snapshot for the show handler: whether an on-host
// drop is currently installed and, if so, the target vector it covers.
func (r *responder) status() (active bool, target ddosevent.VectorTuple) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active, r.target
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
