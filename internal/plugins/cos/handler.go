// Design: docs/architecture/traffic/cos-dynamic.md -- dynamic CoS event handler
// Related: session_state.go -- per-session state for revert

//go:build ze_l2tp

package cos

import (
	"maps"
	"sync"

	"github.com/ze-software/ze/internal/component/iface"
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

// updateQoSFunc is the signature for updating VLAN QoS maps on an interface.
// In production this is iface.GetBackend().UpdateVLANQoSMap.
type updateQoSFunc func(ifaceName string, ingress, egress map[uint32]uint32) error

// resolveStaticFunc returns the static CoS maps for an interface from config.
// Returns nil, nil when the interface has no static CoS profile.
type resolveStaticFunc func(ifaceName string) (ingress, egress map[uint32]uint32)

type cosHandler struct {
	mu            sync.Mutex
	updateQoS     updateQoSFunc
	readLink      func(string) (*iface.InterfaceInfo, error)
	resolveStatic resolveStaticFunc
	unsubs        []func()
	pending       cosSessionChange
	completed     completedCoSChange
}

func newCosHandler(bus ze.EventBus, updateQoS updateQoSFunc, resolveStatic resolveStaticFunc) *cosHandler {
	h := &cosHandler{updateQoS: updateQoS, resolveStatic: resolveStatic, readLink: readSessionLink}
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
	h.mu.Lock()
	defer h.mu.Unlock()
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
	if err := h.updateQoS(p.AccessInterface, profile.IngressMap, profile.EgressMap); err != nil {
		logger().Warn("cos: apply failed",
			"interface", p.AccessInterface, "profile", meta.CoSProfile, "error", err)
		return
	}
	sessionStore.Store(key, sessionCoSState{
		accessInterface: p.AccessInterface,
		profileName:     meta.CoSProfile,
		staticIngress:   staticIn,
		staticEgress:    staticOut,
	})

	recordApplied()
	logger().Info("cos: applied dynamic profile",
		"interface", p.AccessInterface, "profile", meta.CoSProfile,
		"tunnel", p.TunnelID, "session", p.SessionID)
}

func (h *cosHandler) onSessionDown(p *l2tpevents.SessionDownPayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := sessionKey{tunnelID: p.TunnelID, sessionID: p.SessionID}
	change := h.forgetSessionChange(key)
	v, ok := sessionStore.LoadAndDelete(key)
	if !ok {
		if change == nil {
			return
		}
		v = sessionCoSState{accessInterface: change.accessInterface,
			staticIngress: change.ingress, staticEgress: change.egress}
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
	h.mu.Lock()
	defer h.mu.Unlock()
	if p.AccessInterface == "" {
		return
	}
	key := sessionKey{tunnelID: p.TunnelID, sessionID: p.SessionID}
	if h.pending != nil {
		if h.pending.prepared().key == key {
			return
		}
	}
	profile, ok := coreCos.Lookup(p.ProfileName)
	if !ok {
		logger().Warn("cos: CoA profile not found", "profile", p.ProfileName)
		return
	}
	if h.consumeCommandReceipt(p, profile) {
		return
	}

	if err := h.updateQoS(p.AccessInterface, profile.IngressMap, profile.EgressMap); err != nil {
		logger().Warn("cos: CoA apply failed",
			"interface", p.AccessInterface, "profile", p.ProfileName, "error", err)
		return
	}
	if v, loaded := sessionStore.Load(key); loaded {
		if state, ok := v.(sessionCoSState); ok {
			state.profileName = p.ProfileName
			state.commandToken = ""
			sessionStore.Store(key, state)
		}
	}

	recordCoAChanged()
	logger().Info("cos: applied CoA profile change",
		"interface", p.AccessInterface, "profile", p.ProfileName,
		"tunnel", p.TunnelID, "session", p.SessionID)
}

// consumeCommandReceipt skips one notification of the maps the owner already
// installed. A lost notification must not suppress a later profile reload.
// The caller MUST hold h.mu while inspecting the committed snapshot and store.
func (h *cosHandler) consumeCommandReceipt(p *l2tpevents.SessionCoSChangePayload, profile coreCos.Profile) bool {
	key := sessionKey{tunnelID: p.TunnelID, sessionID: p.SessionID}
	value, loaded := sessionStore.Load(key)
	if !loaded {
		return false
	}
	state, valid := value.(sessionCoSState)
	if !valid {
		return false
	}
	if state.commandToken == "" {
		return false
	}
	if state.accessInterface != p.AccessInterface {
		return false
	}
	if state.profileName != p.ProfileName {
		return false
	}
	if h.completed.token != state.commandToken {
		return false
	}
	if h.completed.operation != completionCommitted {
		return false
	}
	if h.completed.change == nil {
		return false
	}
	confirmed := h.completed.change.profile
	if !maps.Equal(confirmed.IngressMap, profile.IngressMap) {
		return false
	}
	if !maps.Equal(confirmed.EgressMap, profile.EgressMap) {
		return false
	}
	state.commandToken = ""
	sessionStore.Store(key, state)
	return true
}
