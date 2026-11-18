// Design: docs/architecture/traffic/cos-dynamic.md -- dynamic CoS event handler
// Related: shared_access.go -- original maps shared by admitted legacy users

//go:build ze_l2tp

package cos

import (
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// updateQoSFunc updates the maps through the selected interface backend.
type updateQoSFunc func(ifaceName string, ingress, egress map[uint32]uint32) error

// cosHandler serializes all legacy notifications and owner transactions.
// accesses retains native map baselines through membership changes and failed
// cleanup. A stopped handler unsubscribes; no event is native retirement proof.
type cosHandler struct {
	mu            sync.Mutex
	updateQoS     updateQoSFunc
	captureTarget originalCoSTargetFactory
	readLink      func(string) (*iface.InterfaceInfo, error)
	unsubs        []func()
	pending       cosSessionChange
	completed     completedCoSChange
	accesses      map[string]sharedCoSAccessState
	reportIssue   func(rpc.ReportIssueInput) error
	stopped       bool
}

func newCosHandler(bus ze.EventBus, updateQoS updateQoSFunc, reportIssue func(rpc.ReportIssueInput) error) *cosHandler {
	return newCosHandlerFromStopped(bus, updateQoS, reportIssue, nil)
}

func (h *cosHandler) stop() {
	h.mu.Lock()
	h.stopped = true
	unsubs := h.unsubs
	h.unsubs = nil
	h.mu.Unlock()
	for _, unsub := range unsubs {
		unsub()
	}
}

func (h *cosHandler) onSessionUp(p *l2tpevents.SessionUpPayload) {
	if p == nil {
		return
	}
	if !p.Birth.IsValid() {
		return
	}
	if err := subscriber.RequireID(logger(), p.SubscriberID); err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return
	}
	if p.AccessInterface == "" {
		logger().Debug("cos: session-up without access interface, skipping",
			"tunnel", p.TunnelID, "session", p.SessionID)
		return
	}
	meta := l2tp.LoadSessionMetadata(p.SubscriberID)
	if meta == nil {
		return
	}
	if meta.CoSProfile == "" {
		return
	}
	profile, found := coreCos.Lookup(meta.CoSProfile)
	if !found {
		h.policyFailure(sessionDiagnostic{tunnelID: p.TunnelID, sessionID: p.SessionID}, p.AccessInterface, meta.CoSProfile, errors.New("the class-of-service profile does not exist")) //nolint:errcheck // Events have no response; the helper logs failed publication.
		return
	}
	key := sessionKey{birth: p.Birth}
	if err := h.applyLegacySharedProfile(key, p.AccessInterface, meta.CoSProfile, profile); err != nil {
		h.reportSharedPolicyFailure(sessionDiagnostic{tunnelID: p.TunnelID, sessionID: p.SessionID}, p.AccessInterface, meta.CoSProfile, err)
		return
	}
	recordApplied()
	logger().Info("cos: applied dynamic profile",
		"interface", p.AccessInterface, "profile", meta.CoSProfile,
		"tunnel", p.TunnelID, "session", p.SessionID)
}

func (h *cosHandler) onSessionDown(p *l2tpevents.SessionDownPayload) {
	if p == nil {
		return
	}
	if !p.Birth.IsValid() {
		return
	}
	if err := subscriber.RequireID(logger(), p.SubscriberID); err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return
	}
	key := sessionKey{birth: p.Birth}
	owner := h.sharedDepartureOwner(key)
	if owner == nil {
		h.forgetSessionChange(key)
		return
	}
	if err := h.departSharedSession(key); err != nil {
		h.cleanupFailure(owner, sessionDiagnostic{tunnelID: p.TunnelID, sessionID: p.SessionID}, err) //nolint:errcheck // Events have no response; the helper logs failed publication.
		return
	}
	h.clearSharedCleanupIfConfirmed(owner) //nolint:errcheck // Failed publication remains retained and the helper logs it.
	recordReverted()
	logger().Info("cos: released dynamic class-of-service use",
		"interface", owner.name, "tunnel", p.TunnelID, "session", p.SessionID)
}

func (h *cosHandler) onCoSChange(p *l2tpevents.SessionCoSChangePayload) {
	if p == nil {
		return
	}
	if !p.Birth.IsValid() {
		return
	}
	if err := subscriber.RequireID(logger(), p.SubscriberID); err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return
	}
	if p.AccessInterface == "" {
		return
	}
	key := sessionKey{birth: p.Birth}
	if h.pending != nil {
		if h.pending.prepared().key == key {
			return
		}
	}
	profile, found := coreCos.Lookup(p.ProfileName)
	if !found {
		h.policyFailure(sessionDiagnostic{tunnelID: p.TunnelID, sessionID: p.SessionID}, p.AccessInterface, p.ProfileName, errors.New("the class-of-service profile does not exist")) //nolint:errcheck // Events have no response; the helper logs failed publication.
		return
	}
	desired, err := newSharedCoSProfile(profile)
	if err != nil {
		h.policyFailure(sessionDiagnostic{tunnelID: p.TunnelID, sessionID: p.SessionID}, p.AccessInterface, p.ProfileName, err) //nolint:errcheck // Events have no response; the helper logs failed publication.
		return
	}
	if h.consumeCommandReceipt(p, desired) {
		return
	}
	if err := h.applyLegacySharedProfile(key, p.AccessInterface, p.ProfileName, profile); err != nil {
		h.reportSharedPolicyFailure(sessionDiagnostic{tunnelID: p.TunnelID, sessionID: p.SessionID}, p.AccessInterface, p.ProfileName, err)
		return
	}
	recordCoAChanged()
	logger().Info("cos: applied CoA profile change",
		"interface", p.AccessInterface, "profile", p.ProfileName,
		"tunnel", p.TunnelID, "session", p.SessionID)
}

// A receipt suppresses one notification of the already confirmed effective
// maps, never a later changed profile. Caller MUST hold h.mu.
func (h *cosHandler) consumeCommandReceipt(p *l2tpevents.SessionCoSChangePayload, desired sharedCoSProfile) bool {
	key := sessionKey{birth: p.Birth}
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
	if h.completed.change.desired != desired {
		return false
	}
	state.commandToken = ""
	sessionStore.Store(key, state)
	return true
}

type stoppedCosHandler struct{ handler *cosHandler }

func newCosHandlerFromStopped(bus ze.EventBus, updateQoS updateQoSFunc, reportIssue func(rpc.ReportIssueInput) error, previous *stoppedCosHandler) *cosHandler {
	if reportIssue == nil {
		panic("BUG: class-of-service handler has no issue publisher")
	}
	h := &cosHandler{updateQoS: updateQoS, readLink: readSessionLink, reportIssue: reportIssue}
	h.takeStoppedOwners(previous)
	h.unsubs = append(h.unsubs,
		l2tpevents.SessionUp.Subscribe(bus, h.onSessionUp),
		l2tpevents.SessionDown.Subscribe(bus, h.onSessionDown),
		l2tpevents.SessionCoSChange.Subscribe(bus, h.onCoSChange),
	)
	return h
}

func (h *cosHandler) stopOwners() *stoppedCosHandler {
	h.stop()
	return &stoppedCosHandler{handler: h}
}

// takeStoppedOwners transfers the exact records from this registration's
// stopped prior handler. It never scans sessionStore or resolves an access name.
// Unsubscription alone is not proof that a native recipient has retired.
// ConfigureEventBus MUST call this before publishing the replacement handler.
func (h *cosHandler) takeStoppedOwners(stopped *stoppedCosHandler) {
	if stopped == nil {
		return
	}
	previous := stopped.handler
	previous.mu.Lock()
	defer previous.mu.Unlock()
	for _, state := range previous.accesses {
		state.sharedOwner().target.stopOwner()
	}
	h.accesses = previous.accesses
	h.pending = previous.pending
	h.completed = previous.completed
	previous.accesses = nil
	previous.pending = nil
	previous.completed = completedCoSChange{}
}

// A failed unpublished write is cleanup work even when no session observation
// exists. A refused healthy policy remains an event, not a cleanup warning.
func (h *cosHandler) reportSharedPolicyFailure(labels sessionDiagnostic, name, profile string, cause error) {
	if state := h.accesses[name]; state != nil {
		if _, uncertain := state.(*sharedCoSUncertain); uncertain {
			h.cleanupFailure(state.sharedOwner(), labels, cause) //nolint:errcheck // Events have no response; the helper logs failed publication.
			return
		}
	}
	h.policyFailure(labels, name, profile, cause) //nolint:errcheck // Events have no response; the helper logs failed publication.
}

func (h *cosHandler) clearSharedCleanupIfConfirmed(access *sharedCoSAccess) error {
	if len(access.departures) != 0 {
		return nil
	}
	if state := h.accesses[access.name]; state != nil {
		if state.sharedOwner() != access {
			return errors.New("the retained cleanup warning belongs to another original access owner")
		}
		switch retained := state.(type) {
		case *sharedCoSCapturePending:
			return retained.cause
		case *sharedCoSReleasePending:
			return retained.cause
		case *sharedCoSReportPending:
		case *sharedCoSAccess:
		default:
			return nil
		}
	}
	if err := h.cleanupClear(access); err != nil {
		if h.accesses == nil {
			h.accesses = make(map[string]sharedCoSAccessState)
		}
		if h.accesses[access.name] == nil {
			h.accesses[access.name] = &sharedCoSReportPending{access: access}
		}
		return err
	}
	if _, reportOnly := h.accesses[access.name].(*sharedCoSReportPending); reportOnly {
		delete(h.accesses, access.name)
	}
	return nil
}
