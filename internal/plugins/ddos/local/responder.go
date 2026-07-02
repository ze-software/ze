// Design: plan/learned/1011-cp-survival-5-detect-0-umbrella.md -- on-host nft drop responder

package local

import (
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	"codeberg.org/thomas-mangin/ze/internal/core/ddosevent"
)

const tableName = "ddos-local"

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
	r.applyMitigation(e.Target, e.Family, "detected")
}

// onCharacterized narrows the installed rule in place to the discriminating
// vector (proto / ports / TCP flags). It also installs from scratch when the
// coarse AttackDetected carried no valid prefix but characterization derived one
// from flow data -- so mitigation still engages.
func (r *responder) onCharacterized(e *ddosevent.AttackCharacterized) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.applyMitigation(e.Target, e.Family, "characterized")
}

// applyMitigation (re)installs the nft drop for target. Caller holds r.mu. Used
// by both the coarse (onDetected) and narrowed (onCharacterized) paths so the
// table is re-registered identically; the only difference is how surgical the
// term is.
func (r *responder) applyMitigation(target ddosevent.VectorTuple, family ddosevent.AttackFamily, phase string) {
	if r.cfg.ResponseLevel != "enforce" {
		logger().Info("ddos-local: alert mode, would mitigate",
			"target", target.DstPrefix, "family", family, "phase", phase)
		return
	}

	if !shouldMitigate(target, r.cfg.Allowlist) {
		logger().Info("ddos-local: target allowlisted, skipping", "target", target.DstPrefix)
		return
	}

	term := buildDropTerm("ddos-drop", target)
	table := firewall.Table{
		Name:   tableName,
		Family: familyFromPrefix(target.DstPrefix),
		Chains: []firewall.Chain{{
			Name:     "ingress",
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookInput,
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
	logger().Info("ddos-local: drop rule installed", "target", target.DstPrefix, "phase", phase)
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

func familyFromPrefix(p netip.Prefix) firewall.TableFamily {
	if p.Addr().Is4() {
		return firewall.FamilyIP
	}
	return firewall.FamilyIP6
}
