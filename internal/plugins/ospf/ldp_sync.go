// Design: plan/learned/1042-ospf-ext-11-ldp-igp-sync.md -- OSPF LDP-IGP synchronization.
// RFC: rfc/short/rfc5443.md -- Section 2 (cost-out at LSInfinity while LDP not
// operational; hold-down estimation), Section 3 (persistent-cost-out alert),
// Section 4 (IP link cost only, never TE cost).
// RFC: rfc/short/rfc6138.md -- Section 4 (broadcast transit-link withhold + cut-edge).
//
// LDP-IGP sync is a purely local metric-origination mechanism: RFC 5443/6138 add no
// wire format. A per-interface state machine holds the interface at LSInfinity (P2P)
// or withholds its transit link (broadcast, non-cut-edge) until LDP signals it is
// operational, then a hold-down timer estimates that all label bindings are exchanged
// before the configured cost is restored. The machine is AF-neutral and is reused by
// both the OSPFv2 and OSPFv3 engine instances through the shared InterfaceInfo model.
//
// This file must NOT import the ldp plugin package: OSPF and LDP are independent
// plugins (ai/rules/plugins.md), so the coupling is only the public
// ze.EventBus keyed by the ldp namespace/event-type strings below, and the interface
// name carried on the (JSON-tagged) session-event payload. Removing the ldp plugin
// leaves OSPF compiling; with no LDP events an ldp-sync interface simply never leaves
// the not-synchronized state.

package ospf

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	"github.com/ze-software/ze/pkg/ze"
)

// LDP event-bus contract consumed here. These mirror internal/plugins/ldp
// (Namespace, EventSessionUp, EventSessionDown) but are declared locally so OSPF does
// not import the ldp plugin (plugin self-containment). The payload is decoded from its
// JSON tags, so it also works when the event arrives from a plugin process.
const (
	ldpSyncNamespace        = "ldp"
	ldpSyncEventSessionUp   = "session-up"
	ldpSyncEventSessionDown = "session-down"
)

// LDP-sync per-interface state (RFC 5443 §2 two-state model plus the transient
// hold-down sub-state). The gauge value ze_ospf_ldp_sync_state uses these numbers.
const (
	ldpSyncNotSynchronized = 0 // cost forced to LSInfinity / transit withheld
	ldpSyncHoldDown        = 1 // LDP session up, awaiting the binding-exchange estimate
	ldpSyncSynchronized    = 2 // configured cost restored / transit advertised
)

func ldpSyncStateName(s int) string {
	switch s {
	case ldpSyncHoldDown:
		return "hold-down"
	case ldpSyncSynchronized:
		return "synchronized"
	default:
		return "not-synchronized"
	}
}

// ldpSyncTimer is the minimal timer surface the manager needs; time.Timer satisfies
// it. Injected so tests can drive hold-down / stuck expiry deterministically.
type ldpSyncTimer interface{ Stop() bool }

// ldpSyncMetrics is the four-series metric set this feature owns.
type ldpSyncMetrics struct {
	state           metrics.GaugeVec   // ze_ospf_ldp_sync_state{interface}
	transitions     metrics.CounterVec // ze_ospf_ldp_sync_transitions_total{interface,to}
	holddownExpired metrics.CounterVec // ze_ospf_ldp_sync_holddown_expired_total{interface}
	costOut         metrics.GaugeVec   // ze_ospf_ldp_sync_costout_seconds{interface}
}

func nopLDPSyncMetrics() ldpSyncMetrics {
	nop := metrics.NopRegistry{}
	return ldpSyncMetrics{
		state:           nop.GaugeVec("", "", nil),
		transitions:     nop.CounterVec("", "", nil),
		holddownExpired: nop.CounterVec("", "", nil),
		costOut:         nop.GaugeVec("", "", nil),
	}
}

// ldpSyncConfig is the per-interface intent the engine reconciles the machines to.
type ldpSyncConfig struct {
	HoldDown    time.Duration
	Cost        uint16
	NetworkType string
}

// ldpSyncMachine is one interface's LDP-sync state.
type ldpSyncMachine struct {
	name        string
	holddown    time.Duration
	cost        uint16
	networkType string

	state     int
	sessionUp bool
	wasSynced bool      // has ever reached Synchronized (arms the RFC 5443 §3 alert)
	stuck     bool      // §3 persistent-cost-out alert currently raised
	epoch     uint64    // invalidates stale hold-down / stuck timer callbacks
	holdUntil time.Time // hold-down deadline (remaining-time snapshot)
	costOut   time.Time // when the current cost-out began; zero when synchronized

	holdTimer  ldpSyncTimer
	stuckTimer ldpSyncTimer
}

// ldpSyncManager owns the per-interface machines, the LDP event subscription, and the
// hold-down / stuck timers. onChange re-originates this router's self-LSAs after every
// sync-state change (RFC 5443 §2: the Router-LSA must reflect the new metric / link
// presence immediately). It is always invoked OUTSIDE mu so it can re-enter the engine.
type ldpSyncManager struct {
	mu       sync.Mutex
	machines map[string]*ldpSyncMachine

	onChange  func()
	now       func() time.Time
	afterFunc func(time.Duration, func()) ldpSyncTimer
	alert     func(iface string)
	log       *slog.Logger
	metrics   ldpSyncMetrics

	unsub func()
}

func newLDPSyncManager(onChange func(), log *slog.Logger) *ldpSyncManager {
	if log == nil {
		log = slog.Default()
	}
	m := &ldpSyncManager{
		machines:  make(map[string]*ldpSyncMachine),
		onChange:  onChange,
		now:       time.Now,
		afterFunc: func(d time.Duration, f func()) ldpSyncTimer { return time.AfterFunc(d, f) },
		log:       log,
		metrics:   nopLDPSyncMetrics(),
	}
	m.alert = func(iface string) {
		// RFC 5443 §3: "an implementation should issue network management alerts to
		// report the error condition and enable the operator to address it."
		m.log.Warn("ospf ldp-sync: interface persistently not-synchronized after LDP was operational (RFC 5443 §3)",
			"interface", iface)
	}
	return m
}

func (m *ldpSyncManager) setMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = ldpSyncMetrics{
		state: reg.GaugeVec(
			"ze_ospf_ldp_sync_state",
			"OSPF LDP-IGP sync state per interface (0=not-synchronized, 1=hold-down, 2=synchronized).",
			[]string{"interface"},
		),
		transitions: reg.CounterVec(
			"ze_ospf_ldp_sync_transitions_total",
			"Total OSPF LDP-IGP sync state transitions, by interface and target state.",
			[]string{"interface", "to"},
		),
		holddownExpired: reg.CounterVec(
			"ze_ospf_ldp_sync_holddown_expired_total",
			"Total OSPF LDP-IGP sync hold-down timer expiries (link declared synchronized), by interface.",
			[]string{"interface"},
		),
		costOut: reg.GaugeVec(
			"ze_ospf_ldp_sync_costout_seconds",
			"Seconds the OSPF interface has been costed out (not synchronized) by LDP-IGP sync.",
			[]string{"interface"},
		),
	}
	// Reflect any machines that already exist (created before metrics were wired).
	for _, mc := range m.machines {
		m.metrics.state.With(mc.name).Set(float64(mc.state))
	}
}

// reconcileTo makes the managed machine set exactly the interfaces in desired. New
// enabled interfaces start Not-Synchronized (RFC 5443 §2: a link that has just come up
// is not synchronized); removed/disabled interfaces are torn down. It re-originates if
// any machine was created/removed so the Router-LSA reflects the new cost-out state.
func (m *ldpSyncManager) reconcileTo(desired map[string]ldpSyncConfig) {
	m.mu.Lock()
	changed := false
	for name, mc := range m.machines {
		if _, keep := desired[name]; keep {
			continue
		}
		m.stopTimersLocked(mc)
		m.metrics.state.With(name).Set(0)
		m.metrics.costOut.With(name).Set(0)
		delete(m.machines, name)
		changed = true
	}
	for name, cfg := range desired {
		mc, ok := m.machines[name]
		if !ok {
			mc = &ldpSyncMachine{name: name, holddown: cfg.HoldDown, cost: cfg.Cost, networkType: cfg.NetworkType, state: ldpSyncNotSynchronized, costOut: m.now()}
			m.machines[name] = mc
			m.metrics.state.With(name).Set(float64(ldpSyncNotSynchronized))
			m.metrics.transitions.With(name, ldpSyncStateName(ldpSyncNotSynchronized)).Inc()
			changed = true
			continue
		}
		// Update the retained restore cost / network type / hold-down without disturbing
		// the live sync state (the configured cost is the restore value, never stored as
		// LSInfinity -- R-2).
		mc.holddown = cfg.HoldDown
		mc.cost = cfg.Cost
		mc.networkType = cfg.NetworkType
	}
	m.mu.Unlock()
	if changed {
		m.fireChange()
	}
}

// reset returns an interface's machine to Not-Synchronized (interface down): the next
// bring-up starts costed-out again (RFC 5443 §2). No-op for an unmanaged interface.
func (m *ldpSyncManager) reset(name string) {
	m.mu.Lock()
	mc, ok := m.machines[name]
	if !ok {
		m.mu.Unlock()
		return
	}
	m.stopTimersLocked(mc)
	mc.sessionUp = false
	mc.stuck = false
	changed := mc.state != ldpSyncNotSynchronized
	m.enterLocked(mc, ldpSyncNotSynchronized)
	m.mu.Unlock()
	if changed {
		m.fireChange()
	}
}

// onSessionUp handles an LDP SessionUp for interface name: Not-Synchronized ->
// Hold-Down, arming the configured hold-down timer (RFC 5443 §2). The metric stays at
// LSInfinity until the timer expires (the estimate that all bindings are exchanged).
func (m *ldpSyncManager) onSessionUp(name string) {
	m.mu.Lock()
	mc, ok := m.machines[name]
	if !ok {
		m.mu.Unlock()
		m.log.Debug("ospf ldp-sync: session-up for interface without ldp-sync, ignored", "interface", name)
		return
	}
	mc.sessionUp = true
	// A returning session clears any pending stuck alert.
	if mc.stuckTimer != nil {
		mc.stuckTimer.Stop()
		mc.stuckTimer = nil
	}
	mc.stuck = false
	changed := false
	if mc.state == ldpSyncNotSynchronized {
		m.enterLocked(mc, ldpSyncHoldDown)
		m.armHoldDownLocked(mc)
		changed = true
	}
	m.mu.Unlock()
	if changed {
		m.fireChange()
	}
}

// onSessionDown handles an LDP SessionDown for interface name: any state ->
// Not-Synchronized, re-forcing the cost-out (RFC 5443 §2 state model). If the interface
// had reached Synchronized, a stuck timer is armed so a persistent cost-out (a genuine
// fault, not bring-up) raises the RFC 5443 §3 alert.
func (m *ldpSyncManager) onSessionDown(name string) {
	m.mu.Lock()
	mc, ok := m.machines[name]
	if !ok {
		m.mu.Unlock()
		m.log.Debug("ospf ldp-sync: session-down for interface without ldp-sync, ignored", "interface", name)
		return
	}
	mc.sessionUp = false
	if mc.holdTimer != nil {
		mc.holdTimer.Stop()
		mc.holdTimer = nil
	}
	changed := mc.state != ldpSyncNotSynchronized
	m.enterLocked(mc, ldpSyncNotSynchronized)
	if mc.wasSynced {
		m.armStuckLocked(mc)
	}
	m.mu.Unlock()
	if changed {
		m.fireChange()
	}
}

// onHoldDownExpiry is the hold-down timer callback: Hold-Down -> Synchronized, the
// RFC 5443 §2 estimation that all label bindings are exchanged. The configured cost is
// restored (computed at origination, so the stored cost is untouched -- R-2).
func (m *ldpSyncManager) onHoldDownExpiry(name string, epoch uint64) {
	m.mu.Lock()
	mc, ok := m.machines[name]
	if !ok || mc.epoch != epoch || mc.state != ldpSyncHoldDown {
		m.mu.Unlock()
		return
	}
	mc.holdTimer = nil
	mc.wasSynced = true
	m.metrics.holddownExpired.With(name).Inc()
	m.enterLocked(mc, ldpSyncSynchronized)
	m.mu.Unlock()
	m.fireChange()
}

// onStuckTimer is the stuck timer callback: if the interface is still Not-Synchronized
// (and was previously synchronized), raise the RFC 5443 §3 network-management alert.
func (m *ldpSyncManager) onStuckTimer(name string, epoch uint64) {
	m.mu.Lock()
	mc, ok := m.machines[name]
	if !ok || mc.epoch != epoch || mc.state != ldpSyncNotSynchronized || !mc.wasSynced {
		m.mu.Unlock()
		return
	}
	mc.stuckTimer = nil
	mc.stuck = true
	alert := m.alert
	m.mu.Unlock()
	if alert != nil {
		alert(name)
	}
}

// enterLocked transitions mc to newState and updates the state/costout gauges and the
// transitions counter. Caller holds mu.
func (m *ldpSyncManager) enterLocked(mc *ldpSyncMachine, newState int) {
	if mc.state == newState && newState == ldpSyncNotSynchronized && !mc.costOut.IsZero() {
		return // already not-synchronized; do not reset the cost-out clock
	}
	transitioned := mc.state != newState
	mc.state = newState
	mc.epoch++
	switch newState {
	case ldpSyncSynchronized:
		mc.costOut = time.Time{}
		m.metrics.costOut.With(mc.name).Set(0)
	default:
		if mc.costOut.IsZero() {
			mc.costOut = m.now()
		}
	}
	m.metrics.state.With(mc.name).Set(float64(newState))
	if transitioned {
		m.metrics.transitions.With(mc.name, ldpSyncStateName(newState)).Inc()
	}
}

func (m *ldpSyncManager) armHoldDownLocked(mc *ldpSyncMachine) {
	if mc.holdTimer != nil {
		mc.holdTimer.Stop()
	}
	mc.holdUntil = m.now().Add(mc.holddown)
	epoch := mc.epoch
	name := mc.name
	// RFC 5443 §2: run the hold-down after session establishment; a holddown of 0
	// (allowed but discouraged) fires immediately (no estimation wait).
	mc.holdTimer = m.afterFunc(mc.holddown, func() { m.onHoldDownExpiry(name, epoch) })
}

func (m *ldpSyncManager) armStuckLocked(mc *ldpSyncMachine) {
	if mc.stuckTimer != nil {
		mc.stuckTimer.Stop()
	}
	epoch := mc.epoch
	name := mc.name
	mc.stuckTimer = m.afterFunc(mc.holddown, func() { m.onStuckTimer(name, epoch) })
}

func (m *ldpSyncManager) stopTimersLocked(mc *ldpSyncMachine) {
	if mc.holdTimer != nil {
		mc.holdTimer.Stop()
		mc.holdTimer = nil
	}
	if mc.stuckTimer != nil {
		mc.stuckTimer.Stop()
		mc.stuckTimer = nil
	}
}

func (m *ldpSyncManager) fireChange() {
	if m.onChange != nil {
		m.onChange()
	}
}

// stateFor returns an interface's sync state and whether it is LDP-sync-managed. The
// origination path (lsdbTopology) reads this to decide the effective cost / withhold.
func (m *ldpSyncManager) stateFor(name string) (state int, managed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.machines[name]
	if !ok {
		return ldpSyncNotSynchronized, false
	}
	return mc.state, true
}

// refreshGauges updates the per-interface cost-out-seconds gauge; called from the
// engine's one-second tick so a persistent cost-out is visible without a transition.
func (m *ldpSyncManager) refreshGauges(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mc := range m.machines {
		if mc.state == ldpSyncSynchronized || mc.costOut.IsZero() {
			m.metrics.costOut.With(mc.name).Set(0)
			continue
		}
		m.metrics.costOut.With(mc.name).Set(now.Sub(mc.costOut).Seconds())
	}
}

// subscribe wires the LDP session-up / session-down events to the machines and returns
// (and remembers) a combined unsubscribe. A nil bus is a no-op (LDP not present) so an
// ldp-sync interface simply stays not-synchronized.
func (m *ldpSyncManager) subscribe(eb ze.EventBus) func() {
	if eb == nil {
		return func() {}
	}
	up := eb.Subscribe(ldpSyncNamespace, ldpSyncEventSessionUp, func(p any) { m.handleSessionEvent(true, p) })
	down := eb.Subscribe(ldpSyncNamespace, ldpSyncEventSessionDown, func(p any) { m.handleSessionEvent(false, p) })
	unsub := func() {
		up()
		down()
	}
	m.mu.Lock()
	m.unsub = unsub
	m.mu.Unlock()
	return unsub
}

func (m *ldpSyncManager) handleSessionEvent(up bool, payload any) {
	iface, ok := decodeLDPSessionInterface(payload)
	if !ok || iface == "" {
		// R-4 / security: an event with no interface cannot be matched to a managed
		// interface; log and drop rather than mutating an arbitrary interface.
		m.log.Debug("ospf ldp-sync: LDP session event without interface, ignored", "session-up", up)
		return
	}
	if up {
		m.onSessionUp(iface)
	} else {
		m.onSessionDown(iface)
	}
}

// decodeLDPSessionInterface extracts the interface name from an LDP session-event
// payload without importing the ldp package. In-process the payload is the ldp
// plugin's *SessionEvent; from a plugin process it is JSON. json.Marshal handles both
// and reads the "interface" tag. Session events are rare (up/down), so the round-trip
// is not on any hot path.
func decodeLDPSessionInterface(payload any) (string, bool) {
	if payload == nil {
		return "", false
	}
	// The bus may hand a raw subscriber the decoded Go value (in-process typed emit),
	// a JSON string, or JSON bytes (a plugin-process emit that was/was not typed-decoded
	// -- see server.deliverEvent). Handle all three so an LDP session event drives the
	// machine regardless of the delivery path.
	var b []byte
	switch v := payload.(type) {
	case string:
		b = []byte(v)
	case []byte:
		b = v
	default:
		var err error
		if b, err = json.Marshal(payload); err != nil {
			return "", false
		}
	}
	var info struct {
		Interface string `json:"interface"`
	}
	if err := json.Unmarshal(b, &info); err != nil {
		return "", false
	}
	return info.Interface, true
}

// stop unsubscribes from the event bus and stops every per-interface timer. Called on
// engine shutdown/reconnect so no stale handler reads freed state (R-7).
func (m *ldpSyncManager) stop() {
	m.mu.Lock()
	unsub := m.unsub
	m.unsub = nil
	for _, mc := range m.machines {
		m.stopTimersLocked(mc)
	}
	m.mu.Unlock()
	if unsub != nil {
		unsub()
	}
}

// subscribeLDPSyncEvents subscribes this engine's LDP-sync manager to the LDP
// SessionUp/SessionDown events on the public event bus and remembers the unsubscribe
// for shutdown (mirrors subscribeIfaceEvents). Wired from register.go OnStarted.
func (e *engine) subscribeLDPSyncEvents(eb ze.EventBus) {
	if eb == nil || e.ldpSync == nil {
		return
	}
	e.ldpSyncUnsub = e.ldpSync.subscribe(eb)
}

// updateLDPSyncMachines reconciles the per-interface LDP-sync machines to the running
// interfaces that enable ldp-sync. Reads the running set under e.mu, then reconciles
// outside the lock (reconcileTo re-originates, which re-enters e.mu).
func (e *engine) updateLDPSyncMachines() {
	if e.ldpSync == nil {
		return
	}
	e.mu.Lock()
	desired := make(map[string]ldpSyncConfig, len(e.running))
	for _, ic := range e.running {
		if !ic.LDPSyncEnabled {
			continue
		}
		cost := ic.Cost
		if !ic.HasCost {
			cost = 1
		}
		desired[ic.Name] = ldpSyncConfig{
			HoldDown:    time.Duration(ic.LDPSyncHoldDown) * time.Second,
			Cost:        cost,
			NetworkType: string(ic.NetworkType),
		}
	}
	e.mu.Unlock()
	e.ldpSync.reconcileTo(desired)
}

// applyLDPSyncOverride mutates the origination-time InterfaceInfo for an ldp-sync
// interface that is not yet synchronized: point-to-point links carry LSInfinity (RFC
// 5443 §2) through the per-interface max-metric flag (so ONLY the p2p/transit link is
// cost-out; the connected-subnet stub keeps the configured cost -- review FIX 2);
// broadcast links withhold the transit link unless the segment is a cut-edge (RFC 6138
// §4 -- a cut-edge MUST be advertised immediately). The cut-edge query flushes a pending
// SPF first (RFC 6138 Appendix A) and is default-safe (advertise on doubt). A
// synchronized or unmanaged interface is left untouched (byte-for-byte today).
func (e *engine) applyLDPSyncOverride(info *ospflsdb.InterfaceInfo, ic interfaceConfig) {
	if e.ldpSync == nil || !ic.LDPSyncEnabled {
		return
	}
	state, managed := e.ldpSync.stateFor(ic.Name)
	if !managed || state == ldpSyncSynchronized {
		return
	}
	switch ic.NetworkType {
	case networkPointToPoint:
		// RFC 5443 §2: cost the point-to-point link out to LSInfinity while not yet
		// synchronized, via the per-interface max-metric flag so ONLY the p2p/transit
		// link is raised and the connected-subnet stub link keeps its configured cost
		// (review FIX 2 -- a blanket info.Cost = LSInfinity would also cost out the stub).
		// Past the guard above the machine is managed and not synchronized, so the flag is
		// always set here.
		info.LDPSyncMaxMetric = true
	case networkBroadcast:
		// No transit link exists until a DR is elected, so there is nothing to withhold
		// yet (routerLinks only builds it when DR != 0). RFC 6138 §4 acts "just before
		// the adjacency is reflected in the LSA".
		if info.DR == (types.RouterID{}) {
			return
		}
		cutEdge := true
		if e.spf != nil {
			cutEdge = e.spf.IsCutEdge(info.AreaID, e.broadcastPseudonodeID(info))
		}
		info.LDPSyncWithholdTransit = ldpSyncWithholdTransit(state, managed, cutEdge)
	}
}

// ldpSyncSnapshot returns the `show ospf ldp-sync` rows for this engine instance.
func (e *engine) ldpSyncSnapshot() []any {
	if e.ldpSync == nil {
		return nil
	}
	rows := e.ldpSync.snapshot()
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, r)
	}
	return out
}

// broadcastPseudonodeID returns the Network-LSA Link State ID (DR interface address)
// for a broadcast segment, matching how routerLinks / SPF key the pseudonode vertex.
//
// NOTE (RFC 6138 broadcast withhold on OSPFv3): this keys on the IPv4 DR interface
// address (info.Address / nbr.Address.Is4()), which is zero on an OSPFv3 (IPv6) engine,
// so the cut-edge query returns cut-edge (advertise) and the RFC 6138 transit-link
// withhold is a no-op for the IPv6 family. IPv6 broadcast withhold is out of scope
// (spec-ospf-ext-11) and this is fail-safe: v6 never withholds, so it can never
// partition an IPv6 broadcast segment.
func (e *engine) broadcastPseudonodeID(info *ospflsdb.InterfaceInfo) types.LinkStateID {
	drAddr := info.Address
	if info.DR != info.RouterID {
		for _, nbr := range info.Neighbors {
			if nbr.RouterID == info.DR && nbr.Address.Is4() {
				drAddr = nbr.Address.As4()
				break
			}
		}
	}
	return types.LinkStateID(drAddr)
}

// effectiveP2PCost returns the Router-LSA link metric for a point-to-point interface
// under LDP-sync: LSInfinity (RFC 5443 §2) while the machine is managed and not yet
// synchronized, otherwise the configured cost (the restore value). It never mutates the
// configured cost, so restoration always yields the operator's value (R-2, A-9). This is
// the semantic definition of the p2p link metric; origination applies it to the p2p link
// only via the LDPSyncMaxMetric flag (review FIX 2), and the snapshot renders it.
func effectiveP2PCost(state int, managed bool, configured uint16) uint16 {
	if managed && state != ldpSyncSynchronized {
		return uint16(ospflsdb.LSInfinity)
	}
	return configured
}

// ldpSyncWithholdTransit reports whether a broadcast interface's transit (Link Type 2)
// link must be withheld: RFC 6138 §4 -- managed, not yet synchronized, and NOT a
// cut-edge (a cut-edge MUST be advertised immediately, the §4 MUST NOT-delay rule).
func ldpSyncWithholdTransit(state int, managed, cutEdge bool) bool {
	return managed && state != ldpSyncSynchronized && !cutEdge
}

// ldpSyncSnapshotEntry is one `show ospf ldp-sync` row.
type ldpSyncSnapshotEntry struct {
	Interface            string `json:"interface"`
	State                string `json:"state"`
	HoldDownSeconds      int    `json:"holddown-seconds"`
	RemainingHoldSeconds int    `json:"remaining-holddown-seconds"`
	EffectiveMetric      int    `json:"effective-metric"`
	Stuck                bool   `json:"stuck"`
}

// snapshot renders the per-interface LDP-sync state for `show ospf ldp-sync`.
func (m *ldpSyncManager) snapshot() []ldpSyncSnapshotEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	out := make([]ldpSyncSnapshotEntry, 0, len(m.machines))
	for _, mc := range m.machines {
		remaining := 0
		if mc.state == ldpSyncHoldDown && mc.holdUntil.After(now) {
			remaining = int(mc.holdUntil.Sub(now).Seconds())
		}
		// P2P not-synchronized advertises its link at LSInfinity (via LDPSyncMaxMetric on
		// the p2p link); broadcast keeps the configured cost (it withholds the transit link
		// rather than cost it out). effectiveP2PCost is the shared P2P metric definition.
		metric := int(mc.cost)
		if mc.networkType == networkPointToPoint {
			metric = int(effectiveP2PCost(mc.state, true, mc.cost))
		}
		out = append(out, ldpSyncSnapshotEntry{
			Interface:            mc.name,
			State:                ldpSyncStateName(mc.state),
			HoldDownSeconds:      int(mc.holddown / time.Second),
			RemainingHoldSeconds: remaining,
			EffectiveMetric:      metric,
			Stuck:                mc.stuck,
		})
	}
	return out
}
