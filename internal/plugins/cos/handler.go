// Design: docs/architecture/traffic/cos-dynamic.md -- dynamic CoS event handler
// Related: session_state.go -- per-session state for revert

//go:build ze_l2tp

package cos

import (
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/ze"
)

type cosMetrics struct {
	applied    metrics.Counter
	reverted   metrics.Counter
	coaChanged metrics.Counter
}

var (
	cosMetricsMu    sync.Mutex
	cosMetricsBound *cosMetrics
)

// BindMetrics registers CoS dynamic counters with the metrics registry.
func BindMetrics(reg metrics.Registry) {
	cosMetricsMu.Lock()
	defer cosMetricsMu.Unlock()
	if cosMetricsBound != nil {
		return
	}
	cosMetricsBound = &cosMetrics{
		applied:    reg.Counter("ze_cos_dynamic_applied", "Dynamic CoS profiles applied on session-up."),
		reverted:   reg.Counter("ze_cos_dynamic_reverted", "Dynamic CoS profiles reverted on session-down."),
		coaChanged: reg.Counter("ze_cos_dynamic_coa_changed", "Dynamic CoS profiles changed via CoA."),
	}
}

func recordApplied() {
	cosMetricsMu.Lock()
	m := cosMetricsBound
	cosMetricsMu.Unlock()
	if m != nil {
		m.applied.Inc()
	}
}

func recordReverted() {
	cosMetricsMu.Lock()
	m := cosMetricsBound
	cosMetricsMu.Unlock()
	if m != nil {
		m.reverted.Inc()
	}
}

func recordCoAChanged() {
	cosMetricsMu.Lock()
	m := cosMetricsBound
	cosMetricsMu.Unlock()
	if m != nil {
		m.coaChanged.Inc()
	}
}

// UpdateQoSFunc is the signature for updating VLAN QoS maps on an interface.
// In production this is iface.GetBackend().UpdateVLANQoSMap.
type UpdateQoSFunc func(ifaceName string, ingress, egress map[uint32]uint32) error

// ResolveStaticFunc returns the static CoS maps for an interface from config.
// Returns nil, nil when the interface has no static CoS profile.
type ResolveStaticFunc func(ifaceName string) (ingress, egress map[uint32]uint32)

type cosHandler struct {
	updateQoS     UpdateQoSFunc
	resolveStatic ResolveStaticFunc
	unsubs        []func()
}

func newCosHandler(bus ze.EventBus, updateQoS UpdateQoSFunc, resolveStatic ResolveStaticFunc) *cosHandler {
	h := &cosHandler{updateQoS: updateQoS, resolveStatic: resolveStatic}
	h.unsubs = append(h.unsubs,
		l2tpevents.SessionUp.Subscribe(bus, h.onSessionUp),
		l2tpevents.SessionDown.Subscribe(bus, h.onSessionDown),
		l2tpevents.SessionCoSChange.Subscribe(bus, h.onCoSChange),
	)
	return h
}

func (h *cosHandler) stop() {
	for _, unsub := range h.unsubs {
		unsub()
	}
	h.unsubs = nil
}

func (h *cosHandler) onSessionUp(p *l2tpevents.SessionUpPayload) {
	if p.AccessInterface == "" {
		logger().Debug("cos: session-up without access interface, skipping",
			"tunnel", p.TunnelID, "session", p.SessionID)
		return
	}

	meta := l2tp.LoadSessionMetadata(p.TunnelID, p.SessionID)
	if meta == nil || meta.CoSProfile == "" {
		return
	}

	profile, ok := coreCos.Lookup(meta.CoSProfile)
	if !ok {
		logger().Warn("cos: profile not found",
			"profile", meta.CoSProfile, "tunnel", p.TunnelID, "session", p.SessionID)
		return
	}

	key := sessionKey{tunnelID: p.TunnelID, sessionID: p.SessionID}
	var staticIn, staticOut map[uint32]uint32
	if h.resolveStatic != nil {
		staticIn, staticOut = h.resolveStatic(p.AccessInterface)
	}
	sessionStore.Store(key, sessionCoSState{
		accessInterface: p.AccessInterface,
		profileName:     meta.CoSProfile,
		staticIngress:   staticIn,
		staticEgress:    staticOut,
	})

	if err := h.updateQoS(p.AccessInterface, profile.IngressMap, profile.EgressMap); err != nil {
		logger().Warn("cos: apply failed",
			"interface", p.AccessInterface, "profile", meta.CoSProfile, "error", err)
		return
	}

	recordApplied()
	logger().Info("cos: applied dynamic profile",
		"interface", p.AccessInterface, "profile", meta.CoSProfile,
		"tunnel", p.TunnelID, "session", p.SessionID)
}

func (h *cosHandler) onSessionDown(p *l2tpevents.SessionDownPayload) {
	key := sessionKey{tunnelID: p.TunnelID, sessionID: p.SessionID}
	v, ok := sessionStore.LoadAndDelete(key)
	if !ok {
		return
	}
	state, ok := v.(sessionCoSState)
	if !ok {
		return
	}

	if err := h.updateQoS(state.accessInterface, state.staticIngress, state.staticEgress); err != nil {
		logger().Warn("cos: revert failed",
			"interface", state.accessInterface, "error", err)
		return
	}

	recordReverted()
	logger().Info("cos: reverted to static config",
		"interface", state.accessInterface,
		"tunnel", p.TunnelID, "session", p.SessionID)
}

func (h *cosHandler) onCoSChange(p *l2tpevents.SessionCoSChangePayload) {
	if p.AccessInterface == "" {
		return
	}

	profile, ok := coreCos.Lookup(p.ProfileName)
	if !ok {
		logger().Warn("cos: CoA profile not found", "profile", p.ProfileName)
		return
	}

	key := sessionKey{tunnelID: p.TunnelID, sessionID: p.SessionID}
	if v, loaded := sessionStore.Load(key); loaded {
		if state, ok := v.(sessionCoSState); ok {
			state.profileName = p.ProfileName
			sessionStore.Store(key, state)
		}
	}

	if err := h.updateQoS(p.AccessInterface, profile.IngressMap, profile.EgressMap); err != nil {
		logger().Warn("cos: CoA apply failed",
			"interface", p.AccessInterface, "profile", p.ProfileName, "error", err)
		return
	}

	recordCoAChanged()
	logger().Info("cos: applied CoA profile change",
		"interface", p.AccessInterface, "profile", p.ProfileName,
		"tunnel", p.TunnelID, "session", p.SessionID)
}
