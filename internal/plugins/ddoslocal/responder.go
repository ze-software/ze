// Design: plan/spec-cp-survival-5-detect-2-local-responder.md -- on-host nft drop responder

package ddoslocal

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

func (r *responder) onDetected(e *ddosevent.AttackDetected) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cfg.ResponseLevel != "enforce" {
		logger().Info("ddos-local: alert mode, would mitigate",
			"target", e.Target.DstPrefix, "family", e.Family)
		return
	}

	if !shouldMitigate(e.Target, r.cfg.Allowlist) {
		logger().Info("ddos-local: target allowlisted, skipping", "target", e.Target.DstPrefix)
		return
	}

	term := buildDropTerm("ddos-drop", e.Target)
	table := firewall.Table{
		Name:   tableName,
		Family: familyFromPrefix(e.Target.DstPrefix),
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
		registerTables(tableName, nil)
		logger().Error("ddos-local: failed to apply drop rule", "error", err)
		return
	}

	r.active = true
	r.target = e.Target
	logger().Info("ddos-local: drop rule installed", "target", e.Target.DstPrefix)
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

func familyFromPrefix(p netip.Prefix) firewall.TableFamily {
	if p.Addr().Is4() {
		return firewall.FamilyIP
	}
	return firewall.FamilyIP6
}
