// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- OSPF Graceful Restart control plane.
// Related: gr_nvs.go (restart fact), gr_lsa.go + packet/grace_lsa.go (IPv4 wire),
//
//	v3/packet/lsa_grace.go (IPv6 wire).
//
// RFC: rfc/short/rfc3623.md (OSPFv2 GR restarter + helper, the shared control plane),
//
//	rfc/short/rfc5187.md (OSPFv3 GR wire delta + sec 3.1/3.2 preservation).
//
// ONE feature, ONE control plane, TWO wire encodings. grManager holds the shared restarter
// and helper state machines that drive BOTH address families; only the Grace-LSA origination
// and decode fork per family (IPv4 = ext-1 Opaque Type 3; IPv6 = native LS Type 0x000B). The
// restarter FSM lives in gr_restarter.go, the helper FSM in gr_helper.go. The engine owns one
// grManager per instance; it is nil-safe/inert until the operator enables graceful-restart.
package ospf

import (
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/textbuf"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// Graceful Restart reason codes (RFC 3623 sec A / RFC 5187 sec 2.2).
const (
	grReasonUnknown         uint8 = 0
	grReasonSoftwareRestart uint8 = 1
	grReasonReload          uint8 = 2
	grReasonRedundantCP     uint8 = 3
)

// Restarter / helper exit-reason labels for the ze_ospf_gr_*_exits_total metrics.
const (
	grExitAdjacencies    = "adjacencies"
	grExitInconsistent   = "inconsistent-lsa"
	grExitGraceExpiry    = "grace-expiry"
	grExitFlushed        = "flushed"
	grExitTopologyChange = "topology-change"
)

// grMetrics is the ze_ospf_gr_* series (spec-ospf-ext-9). One set with a family label drives
// both address families (matching how the unified engine reports per-family state).
type grMetrics struct {
	restarterActive metrics.GaugeVec   // labels: family
	restarterExits  metrics.CounterVec // labels: family, reason
	helperSessions  metrics.GaugeVec   // labels: family, interface
	helperExits     metrics.CounterVec // labels: family, reason
	graceLSAs       metrics.GaugeVec   // labels: family, direction
}

func nopGRMetrics() grMetrics {
	nop := metrics.NopRegistry{}
	return grMetrics{
		restarterActive: nop.GaugeVec("", "", nil),
		restarterExits:  nop.CounterVec("", "", nil),
		helperSessions:  nop.GaugeVec("", "", nil),
		helperExits:     nop.CounterVec("", "", nil),
		graceLSAs:       nop.GaugeVec("", "", nil),
	}
}

// helperKey identifies one helper relationship: the interface the Grace-LSA arrived on and
// the restarting router's ID (RFC 3623 sec 3: helper state is per-segment, per restarting X).
type helperKey struct {
	iface  string
	router ospftypes.RouterID
}

// helperSession is one active helper relationship (RFC 3623 sec 3.1/3.2). graceEnd is derived
// from the Grace-LSA LS age (the grace clock): graceEnd = now + (Grace Period - LS age). wasDR
// records whether X was DR so the helper keeps advertising X as DR for the window.
type helperSession struct {
	iface    string
	router   ospftypes.RouterID
	address  netip.Addr
	ifaceID  uint32
	graceEnd time.Time
	wasDR    bool
	timer    *time.Timer
}

// graceReceived is the family-neutral decoded Grace-LSA the helper reacts to. IPv4 fills it
// from the ext-1 opaque OnReceive; IPv6 fills it from the decoded native LS Type 0x000B LSA.
type graceReceived struct {
	iface        string
	advRouter    ospftypes.RouterID
	ifaceAddr    [4]byte // v4 type-3 IP address (shared media); zero for IPv6
	hasIfaceAddr bool
	gracePeriod  uint32
	lsAge        uint16 // current LS age (the grace clock, RFC 3623 sec A)
	withdrawn    bool   // a flushed/MaxAge Grace-LSA -> helper exit (RFC 3623 sec 3.2)
}

// grManager is the per-engine Graceful Restart control plane. Its mutex guards all restarter
// and helper state; timers fire callbacks that re-acquire it. It is family-neutral: the
// codec.IsV6() branch appears only at the Grace-LSA origination/decode wire seam (R-13).
type grManager struct {
	e   *engine
	now func() time.Time

	resumeOnce sync.Once // resumeFromNVS runs at most once per engine lifecycle

	mu  sync.Mutex
	cfg gracefulRestartConfig

	// restarter runtime (guarded by mu)
	restarting   bool
	graceEnd     time.Time
	reason       uint8
	expected     map[ospftypes.RouterID]bool // pre-restart adjacencies -> reached Full again
	restartTimer *time.Timer
	gracefulStop bool // a graceful-restart stop is in progress: skip RemoveAll

	// helper runtime (guarded by mu)
	helping map[helperKey]*helperSession

	// preservedIfaceIDs pins the RFC 5187 sec 3.2 OSPFv3 Interface IDs restored from the
	// restart fact so re-originated LSAs match pre-restart neighbor state (guarded by mu).
	preservedIfaceIDs map[string]uint32

	metrics grMetrics
}

// newGRManager builds the inert per-engine GR manager. It becomes active only when the
// operator configures graceful-restart (configure) and the engine starts (start/resume).
func newGRManager(e *engine) *grManager {
	return &grManager{
		e:        e,
		now:      time.Now,
		expected: map[ospftypes.RouterID]bool{},
		helping:  map[helperKey]*helperSession{},
		metrics:  nopGRMetrics(),
	}
}

// grFamilyLabel is the ze_ospf_gr_* family label: ipv4 for OSPFv2, ipv6 for OSPFv3.
func (e *engine) grFamilyLabel() string {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return interfaceFamilyIPv6
	}
	return interfaceFamilyIPv4
}

// grStorageKey is the per-engine NVS restart-fact key suffix, distinct per address family and
// (for the RFC 6549 IPv4 multi-instance) per instance, so engines in one process do not
// clobber each other's fact.
func (e *engine) grStorageKey() string {
	var tb textbuf.Buffer
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return tb.Str("v6-").Str(e.af.String()).String()
	}
	return tb.Str("v4-").Uint8(e.cfg.InstanceID).String()
}

// grFactKey is the full zefs blob key for this engine's restart fact.
func (e *engine) grFactKey() string {
	var tb textbuf.Buffer
	return tb.Str(grRestartFactKeyPrefix).Str(e.grStorageKey()).String()
}

// configure stores the resolved GR config on the manager (called from setConfig/reconcile) and
// wires the RFC 7770 informational-capability seam so the RI LSA advertises GR bits 0/1.
func (m *grManager) configure(cfg gracefulRestartConfig) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	m.e.riGRState = m.riGRCapabilities
}

// setGRMetrics binds the ze_ospf_gr_* series (called from the engine setMetrics).
func (m *grManager) setGRMetrics(reg metrics.Registry) {
	if m == nil || reg == nil {
		return
	}
	gm := grMetrics{
		restarterActive: reg.GaugeVec("ze_ospf_gr_restarter_active", "Whether this OSPF engine is currently in graceful restart (1) or not (0), by address family.", []string{labelFamily}),
		restarterExits:  reg.CounterVec("ze_ospf_gr_restarter_exits_total", "Total graceful-restart restarter exits, by address family and trigger reason.", []string{labelFamily, labelReason}),
		helperSessions:  reg.GaugeVec("ze_ospf_gr_helper_sessions", "Current graceful-restart helper sessions, by address family and interface.", []string{labelFamily, labelInterface}),
		helperExits:     reg.CounterVec("ze_ospf_gr_helper_exits_total", "Total graceful-restart helper exits, by address family and trigger reason.", []string{labelFamily, labelReason}),
		graceLSAs:       reg.GaugeVec("ze_ospf_gr_grace_lsas", "Current graceful-restart Grace-LSAs, by address family and direction (originated/received).", []string{labelFamily, labelDirection}),
	}
	m.mu.Lock()
	m.metrics = gm
	active := 0.0
	if m.restarting {
		active = 1.0
	}
	m.mu.Unlock()
	gm.restarterActive.With(m.e.grFamilyLabel()).Set(active)
}

// stop cancels any pending GR timers on engine shutdown so no callback fires after teardown.
func (m *grManager) stop() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.restartTimer != nil {
		m.restartTimer.Stop()
		m.restartTimer = nil
	}
	for _, s := range m.helping {
		if s.timer != nil {
			s.timer.Stop()
		}
	}
}

// suppressOrigination reports whether self-LSA origination must be suppressed (RFC 3623 sec 2:
// the restarting router originates no self-LSAs while in graceful restart). Read by the shared
// originateSelfLSAs chokepoint (both families).
func (m *grManager) suppressOrigination() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restarting
}

// suppressInstall reports whether SPF route install/removal must be suppressed: true while in
// graceful restart (no route churn, RFC 3623 sec 2) or during the graceful stop (do not
// RemoveAll, RFC 3623 sec 2.1 -- retain the FIB).
func (m *grManager) suppressInstall() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restarting || m.gracefulStop
}

// suppressSelfFlush reports whether the LSDB must NOT flush the restarter's own pre-restart
// self-LSAs (RFC 3623 sec 2: "does NOT modify or flush received self-originated LSAs").
func (m *grManager) suppressSelfFlush() bool {
	return m.suppressOrigination()
}

// inRestart reports the in-restart flag (for gates and tests).
func (m *grManager) inRestart() bool { return m.suppressOrigination() }

// shouldReElectSelfDR reports whether the restarting router must re-assert itself as DR on a
// segment (RFC 3623 sec 2): while in graceful restart, if the interface is still in the
// Waiting state and a received Hello already lists this router as DR (it was DR before the
// restart), it keeps the DR role rather than deferring to a fresh election. Consulted by the
// topology builder so the re-originated Router/Network-LSAs reflect the retained DR role.
func (m *grManager) shouldReElectSelfDR(interfaceWaiting, helloListsSelfAsDR bool) bool {
	return m.inRestart() && interfaceWaiting && helloListsSelfAsDR
}

// riGRCapabilities feeds the RFC 7770 sec 2.5 informational bits 0/1 (GR capable / GR helper),
// wired into e.riGRState. Capable = the restarter is configured; helper = helper support on.
func (m *grManager) riGRCapabilities() (capable, helper bool) {
	if m == nil {
		return false, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg.restarterEnabled(), m.cfg.HelperEnabled
}

// v3GraceFromLSA decodes an installed OSPFv3 Grace-LSA (native LS Type 0x000B, RawBytes on the
// wire) into its (grace period, reason) pair. It returns false on a malformed body (missing a
// mandatory TLV): the caller ignores it (AC-21).
func v3GraceFromLSA(raw []byte) (gracePeriod uint32, ok bool) {
	lsa, err := ospfv3packet.DecodeLSA(raw)
	if err != nil {
		return 0, false
	}
	if lsa.Header.Type != ospfv3types.LSTypeGrace {
		return 0, false
	}
	g, err := lsa.DecodeGrace()
	if err != nil {
		return 0, false
	}
	return g.GracePeriod, true
}

// registerGraceConsumer registers the IPv4 (RFC 3623) Grace-LSA opaque consumer (Opaque Type
// 3, link scope) bound to this engine. Origination is driven directly by the restarter
// (prepare/exit), so only the receive hook is wired; the helper reacts in graceOnReceive.
// Called once per IPv4 engine from register.go.
func registerGraceConsumer(e *engine) error {
	// spec-ospf-ext-14: wire the Grace-LSA body decoder into the debug detail registry so
	// `show ospf database opaque-link detail` renders Grace LSAs typed (RFC 3623).
	registerOpaqueDetailDecoder(ospfpacket.GraceOpaqueType, "grace", func(b []byte) (any, error) {
		v, err := ospfpacket.DecodeGraceLSA(b)
		return v, err
	})
	return registerOpaqueConsumer(ospfpacket.GraceOpaqueType, OpaqueScopeLink, nil, e.graceOnReceive)
}

// currentFullNeighbors returns the Router IDs of neighbors currently at Full, used to seed the
// restarter's expected-adjacency set at prepare time (RFC 3623 sec 2.2 exit trigger 1).
func (e *engine) currentFullNeighbors() []ospftypes.RouterID {
	var out []ospftypes.RouterID
	snap := e.neighbors.Snapshot()
	for i := range snap {
		if snap[i].State != ospflsdb.NeighborStateFull {
			continue
		}
		rid, err := ospftypes.ParseRouterID(snap[i].RouterID)
		if err != nil {
			continue
		}
		out = append(out, rid)
	}
	return out
}

// hasFullNeighbor reports whether a Full adjacency with router exists on iface (the RFC 3623
// sec 3.1 helper-entry Full-adjacency check).
func (e *engine) hasFullNeighbor(iface string, router ospftypes.RouterID) bool {
	for _, n := range e.neighbors.FloodNeighbors(iface) {
		if n.RouterID == router && n.State == ospflsdb.NeighborStateFull {
			return true
		}
	}
	return false
}

// neighborTopologyFacts returns X's reachable address, OSPFv3 Interface ID, and whether X is
// currently DR on iface, used to seed a helper session so X stays advertised (and DR).
func (e *engine) neighborTopologyFacts(iface string, router ospftypes.RouterID) (netip.Addr, uint32, bool) {
	var addr netip.Addr
	var ifaceID uint32
	for _, n := range e.neighbors.FloodNeighbors(iface) {
		if n.RouterID == router {
			addr = n.Address
			ifaceID = n.InterfaceID
			break
		}
	}
	wasDR := false
	e.mu.Lock()
	if ifc := e.interfaces[iface]; ifc != nil {
		wasDR = ifc.DR() == router
	}
	e.mu.Unlock()
	return addr, ifaceID, wasDR
}

// interfaceAreaType returns the area type (normal/stub/nssa) of iface's area, for the RFC 3623
// sec 3.2 stub-area exception in strict LSA checking.
func (e *engine) interfaceAreaType(iface string) string {
	e.mu.Lock()
	cfg := e.cfg
	ic, ok := e.running[iface]
	e.mu.Unlock()
	if !ok {
		return areaTypeNormal
	}
	return string(areaTypeFor(cfg, ic.AreaID))
}

// mergeHelpingNeighbors merges helper-session (forced-Full) neighbors into the live neighbor
// set for the topology builder, deduping by Router ID: a helper entry keeps X at Full even
// when the live NSM regressed (RFC 3623 sec 3).
func (m *grManager) mergeHelpingNeighbors(iface string, live []ospflsdb.NeighborInfo) []ospflsdb.NeighborInfo {
	if m == nil {
		return live
	}
	helped := m.helpingNeighbors(iface)
	if len(helped) == 0 {
		return live
	}
	byID := make(map[ospftypes.RouterID]int, len(live))
	for i, n := range live {
		byID[n.RouterID] = i
	}
	for _, h := range helped {
		if idx, ok := byID[h.RouterID]; ok {
			live[idx].State = ospflsdb.NeighborStateFull
			continue
		}
		live = append(live, h)
	}
	return live
}
