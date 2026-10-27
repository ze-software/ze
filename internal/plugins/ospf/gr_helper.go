// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- GR helper (restart-aid) state machine.
// Related: gr.go (grManager), instance.go (lsdbTopology injects helped neighbors).
// RFC: rfc/short/rfc3623.md sec 3 (keep advertising X, keep X as DR), sec 3.1 (entry checks),
//
//	sec 3.2 (exit triggers incl. strict LSA checking + the stub-area exception);
//	rfc/short/rfc5187.md sec 2 (OSPFv3 keys X by Advertising Router).
package ospf

import (
	"net/netip"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

// helperEntry is the set of RFC 3623 sec 3.1 helper-entry checks, extracted so the decision is
// pure and table-testable (AC-16/AC-17).
type helperEntry struct {
	policyEnabled  bool // RFC 3623 App B.2 RestartHelperSupport
	selfRestarting bool // Y (this router) is itself in graceful restart
	graceRemaining bool // the Grace-LSA LS age is still below the Grace Period
	fullAdjacency  bool // a Full adjacency with X exists on the segment
	lsdbUnchanged  bool // no content change in the LSDB since X restarted
}

// helperEntryAllowed reports whether ALL RFC 3623 sec 3.1 checks pass; on failure it names the
// first failing check.
func helperEntryAllowed(e helperEntry) (bool, string) {
	switch {
	case !e.policyEnabled:
		return false, "helper-disabled"
	case e.selfRestarting:
		return false, "self-restarting"
	case !e.graceRemaining:
		return false, "grace-expired"
	case !e.fullAdjacency:
		return false, "no-full-adjacency"
	case !e.lsdbUnchanged:
		return false, "lsdb-changed"
	default:
		return true, ""
	}
}

// graceOnReceive is the IPv4 (ext-1) Grace-LSA OnReceive hook: it parses the Opaque Type 3
// body, identifies the restarting neighbor X, and dispatches to the shared helper. A malformed
// body (missing a mandatory TLV) is ignored (RFC 3623 sec A / AC-17), and so is a Grace-LSA
// that identifies no neighbor on the segment.
func (e *engine) graceOnReceive(r opaqueReceived) {
	if r.OpaqueType != ospfpacket.GraceOpaqueType {
		return
	}
	body, err := grV4Parse(r.Body)
	if err != nil && !r.Withdrawn {
		return
	}
	// RFC 3623 Section 3.1: a flushed Grace-LSA still names X by its body on shared media, so
	// the flush is matched to the helper session the same way the entry was.
	restarter, ok := e.graceRestarterV4(r.Interface, r.AdvertisingRouter, body)
	if !ok {
		return
	}
	g := graceReceived{iface: r.Interface, advRouter: restarter, withdrawn: r.Withdrawn}
	if !r.Withdrawn {
		g.gracePeriod = body.GracePeriod
		// RFC 3623 sec A grace clock: the remaining grace is Grace Period - LS age. The ext-1
		// opaque delivery now surfaces the received LSA's LS age (opaqueReceived.Age), so a
		// Grace-LSA arriving mid-window (e.g. a retransmit at a higher age) measures the true
		// remaining grace rather than resetting to a full period -- matching the IPv6 native
		// path (grInspectV6Update reads lsa.Header.Age).
		g.lsAge = r.Age
	}
	e.gr.onGraceReceived(g)
}

// graceRestarterV4 returns the Router ID of the neighbor X a received OSPFv2 Grace-LSA asks Ze
// to help on iface, and false when the Grace-LSA identifies no neighbor there. On shared media
// X is the neighbor whose interface address the IP interface address TLV carries, whatever
// the Advertising Router field says; a Grace-LSA without that TLV, or naming an address no
// neighbor holds, identifies nobody. FRR (ospf_process_grace_lsa, ospf_nbr_lookup_by_addr) and
// BIRD (ospf_neigh_notify_grace_lsa, find_neigh_by_ip) both identify X this way. A flushed
// (MaxAge) Grace-LSA is matched the same way, so on shared media a flush without the IP
// interface address TLV, or naming an address no neighbor holds, is ignored: no helper session
// ends, and the helper keeps helping until the grace period expires (RFC 3623 Section 3.2,
// exit event 2). FRR does the same (ospf_process_maxage_grace_lsa returns when
// ospf_nbr_lookup_by_addr finds no neighbor). OSPFv3 never reaches here: RFC 5340 identifies
// every OSPFv3 neighbor by Router ID.
func (e *engine) graceRestarterV4(iface string, advRouter ospftypes.RouterID, body ospfpacket.GraceLSA) (ospftypes.RouterID, bool) {
	_, ic, running := e.runningInterface(iface)
	// An interface Ze does not run has no neighbor to match by address, so the Advertising
	// Router stands; the Full-adjacency check in onGraceReceived then refuses helper entry.
	if !running {
		return advRouter, true
	}
	// RFC 3623 Section 3.1: "On broadcast, NBMA and Point-to-MultiPoint segments, the neighbor
	// relationship with X is identified by the IP interface address in the body of the
	// grace-LSA (see Appendix A).  On all other segment types, X is identified by the
	// grace-LSA's Advertising Router field."
	if !grSharedMedia(string(ic.NetworkType)) {
		return advRouter, true
	}
	if !body.HasInterfaceAddr {
		return ospftypes.RouterID{}, false
	}
	want := netip.AddrFrom4(body.InterfaceAddr)
	for _, n := range e.neighbors.FloodNeighbors(iface) {
		if n.Address == want {
			return n.RouterID, true
		}
	}
	return ospftypes.RouterID{}, false
}

// grInspectV6Update scans a just-installed OSPFv3 LSUpdate for native Grace-LSAs (LS Type
// 0x000B) and dispatches each to the shared helper (RFC 5187): a MaxAge instance is a flush
// (helper exit), a malformed body is ignored (AC-21), otherwise the (grace period, LS age)
// drives helper entry/update. IPv6-only; the IPv4 family delivers via graceOnReceive.
func (e *engine) grInspectV6Update(iface string, up ospfpacket.LSUpdate) {
	for i := range up.LSAs {
		lsa := &up.LSAs[i]
		if lsa.Header.Type != ospftypes.LSTypeGraceV6 {
			continue
		}
		g := graceReceived{iface: iface, advRouter: lsa.Header.AdvertisingRouter}
		if lsa.Header.Age.IsMaxAge() {
			g.withdrawn = true
		} else {
			period, ok := v3GraceFromLSA(lsa.RawBytes)
			if !ok {
				continue // malformed Grace-LSA: ignore (RFC 5187 sec 2.2 / AC-21)
			}
			g.gracePeriod = period
			g.lsAge = lsa.Header.Age.Age()
		}
		e.gr.onGraceReceived(g)
	}
}

// onGraceReceived is the shared (family-neutral) helper reaction to a received Grace-LSA: a
// flush exits helping; a re-receipt updates the grace period; otherwise the RFC 3623 sec 3.1
// entry checks decide whether to enter helper mode.
func (m *grManager) onGraceReceived(g graceReceived) {
	if m == nil {
		return
	}
	key := helperKey{iface: g.iface, router: g.advRouter}
	if g.withdrawn {
		m.helperExit(key, nil, grExitFlushed)
		return
	}
	if _, ok := m.helperGraceEnd(key); ok {
		m.updateGrace(key, g)
		return
	}
	m.mu.Lock()
	cfg := m.cfg
	selfRestart := m.restarting
	m.mu.Unlock()
	entry := helperEntry{
		policyEnabled:  cfg.HelperEnabled,
		selfRestarting: selfRestart,
		graceRemaining: g.gracePeriod > uint32(g.lsAge),
		fullAdjacency:  m.e.hasFullNeighbor(g.iface, g.advRouter),
		// NOTE (RFC 3623 sec 3.1, NOTE-4): the entry-time "no LSDB change since X restarted"
		// check is left permissive (a Full adjacency implies the databases were synchronized at
		// entry). This is deliberate: the sec 3.2 strict-LSA-checking exit (onContentChange)
		// tears the helper session down on any SUBSEQUENT change that would flood to X, which
		// mitigates a benign permissive entry. No behavior change.
		lsdbUnchanged: true,
	}
	if ok, _ := helperEntryAllowed(entry); !ok {
		return
	}
	addr, ifaceID, wasDR := m.e.neighborTopologyFacts(g.iface, g.advRouter)
	m.helperEnter(key, g, wasDR, addr, ifaceID)
}

// helperEnter records a new helper session and, per RFC 3623 sec 3, immediately re-originates
// self-LSAs so the topology builder starts advertising X's adjacency (and X as DR when wasDR).
func (m *grManager) helperEnter(key helperKey, g graceReceived, wasDR bool, addr netip.Addr, ifaceID uint32) {
	graceEnd := m.now().Add(time.Duration(int64(g.gracePeriod)-int64(g.lsAge)) * time.Second)
	m.addHelperSession(key, &helperSession{iface: key.iface, router: key.router, address: addr, ifaceID: ifaceID, graceEnd: graceEnd, wasDR: wasDR})
	// Advertise X's adjacency now (the topology builder consults isHelping).
	m.e.originateSelfLSAs()
}

// addHelperSession records s under key and publishes the helper gauges. It
// holds m.mu across the metric calls, so it releases it by defer: a Grace-LSA
// reaches this from the receive worker, and a panic it contains (task
// ospf/receive) must not leave m.mu held for the next Grace-LSA and for
// suppressOrigination.
func (m *grManager) addHelperSession(key helperKey, s *helperSession) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if d := s.graceEnd.Sub(m.now()); d > 0 {
		s.timer = time.AfterFunc(d, func() { m.helperGraceExpired(key) })
	}
	m.helping[key] = s
	m.metrics.helperSessions.With(m.e.grFamilyLabel(), key.iface).Set(float64(m.sessionsOnIfaceLocked(key.iface)))
	m.metrics.graceLSAs.With(m.e.grFamilyLabel(), "received").Set(float64(len(m.helping)))
}

// updateGrace extends an existing helper session's grace window on a re-received Grace-LSA
// (RFC 3623 sec 3.1: accept and update; no re-entry churn).
func (m *grManager) updateGrace(key helperKey, g graceReceived) {
	graceEnd := m.now().Add(time.Duration(int64(g.gracePeriod)-int64(g.lsAge)) * time.Second)
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.helping[key]
	if !ok {
		return
	}
	s.graceEnd = graceEnd
	if s.timer != nil {
		s.timer.Stop()
	}
	if d := graceEnd.Sub(m.now()); d > 0 {
		s.timer = time.AfterFunc(d, func() { m.helperGraceExpired(key) })
	}
}

// helperGraceExpired is the RFC 3623 sec 3.2 grace-expiry exit trigger (the grace clock, LS age
// vs Grace Period, elapsed).
func (m *grManager) helperGraceExpired(key helperKey) { m.helperExit(key, nil, grExitGraceExpiry) }

// helperExit ends a helper session (RFC 3623 sec 3.2): stop the timer, drop the session, then
// re-originate self-LSAs and recompute SPF so the frozen adjacency view is corrected (DR
// recalc + Router/Network-LSA re-origination). A non-nil expected session prevents
// an exit derived from an old snapshot from removing a replacement at the same key.
func (m *grManager) helperExit(key helperKey, expected *helperSession, reason string) {
	if !m.removeHelperSession(key, expected, reason) {
		return
	}
	m.e.originateSelfLSAs()
	m.e.triggerAllSPF()
}

// removeHelperSession drops the session at key, when it is still the expected
// one, publishes the helper gauges, and answers whether it removed one. It
// releases m.mu by defer for the reason addHelperSession gives: the LSDB
// content-change observer reaches it from the receive worker.
func (m *grManager) removeHelperSession(key helperKey, expected *helperSession, reason string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.helping[key]
	if !ok {
		return false
	}
	if expected != nil && s != expected {
		return false
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	delete(m.helping, key)
	m.metrics.helperSessions.With(m.e.grFamilyLabel(), key.iface).Set(float64(m.sessionsOnIfaceLocked(key.iface)))
	m.metrics.helperExits.With(m.e.grFamilyLabel(), reason).Inc()
	m.metrics.graceLSAs.With(m.e.grFamilyLabel(), "received").Set(float64(len(m.helping)))
	return true
}

// onContentChange is the RFC 3623 sec 3.2 strict-LSA-checking exit driver (fed by the LSDB
// post-install content-change observer). For every helper session it exits when a changed LSA
// would have flooded to X, honoring the stub-area external exception (AC-20).
func (m *grManager) onContentChange(area ospftypes.AreaID, lsType ospftypes.LSType) {
	if m == nil {
		return
	}
	m.mu.Lock()
	strict := m.cfg.StrictLSAChecking
	if !strict {
		m.mu.Unlock()
		return
	}
	type candidate struct {
		key     helperKey
		session *helperSession
	}
	candidates := make([]candidate, 0, len(m.helping))
	for key, session := range m.helping {
		candidates = append(candidates, candidate{key: key, session: session})
	}
	m.mu.Unlock()
	// Topology lookups acquire the engine lock and consult this manager again.
	// Never call them while holding the GR lock.
	for _, c := range candidates {
		areaType := m.e.interfaceAreaType(c.key.iface)
		if helperShouldExitOnChange(strict, areaType, lsType, wouldFloodToHelper(lsType, areaType, c.key.iface, area, m.e)) {
			m.helperExit(c.key, c.session, grExitTopologyChange)
		}
	}
}

// helperShouldExitOnChange is the pure RFC 3623 sec 3.2 strict-checking decision: exit only
// when strict checking is on AND the changed LSA would have flooded to X. A changed
// AS-external LSA in a stub/NSSA area never floods to X, so it does not terminate helping
// (the stub-area exception, AC-20).
func helperShouldExitOnChange(strict bool, helperAreaType string, changedType ospftypes.LSType, wouldFlood bool) bool {
	if !strict {
		return false
	}
	if changedType.ASExternal() && (helperAreaType == ospftypes.AreaTypeStub || helperAreaType == ospftypes.AreaTypeNSSA) {
		return false
	}
	return wouldFlood
}

// wouldFloodToHelper reports whether a changed LSA of lsType in area would have flooded to X on
// the helper's segment: an area-scoped LSA floods to X only in X's area; an AS-external floods
// to X in any non-stub/NSSA area.
func wouldFloodToHelper(lsType ospftypes.LSType, helperAreaType, iface string, area ospftypes.AreaID, e *engine) bool {
	if lsType.ASExternal() {
		return helperAreaType != ospftypes.AreaTypeStub && helperAreaType != ospftypes.AreaTypeNSSA
	}
	return sameHelperArea(e, iface, area)
}

// sameHelperArea reports whether the helper's interface is in area.
func sameHelperArea(e *engine, iface string, area ospftypes.AreaID) bool {
	topo := e.lsdbTopology()
	for i := range topo {
		if topo[i].Name == iface {
			return topo[i].AreaID == area
		}
	}
	return false
}

// isHelping reports whether (iface, router) is an active helper session (consulted by the
// topology builder to keep X's link advertised).
func (m *grManager) isHelping(iface string, router ospftypes.RouterID) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.helping[helperKey{iface: iface, router: router}]
	return ok
}

// helpingNeighbors returns forced-Full NeighborInfo for every helper session on iface, so
// lsdbTopology keeps X in the Router-LSA (and Network-LSA if DR) regardless of NSM state
// (RFC 3623 sec 3).
func (m *grManager) helpingNeighbors(iface string) []ospflsdb.NeighborInfo {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ospflsdb.NeighborInfo
	for key, s := range m.helping {
		if key.iface != iface {
			continue
		}
		out = append(out, ospflsdb.NeighborInfo{
			RouterID:      key.router,
			Address:       s.address,
			State:         ospflsdb.NeighborStateFull,
			InterfaceID:   s.ifaceID,
			OpaqueCapable: true,
		})
	}
	return out
}

// helperDR returns the Router ID this helper keeps as DR on iface (X, when X was DR at entry),
// so lsdbTopology can hold X as DR for the grace window (RFC 3623 sec 3).
func (m *grManager) helperDR(iface string) (ospftypes.RouterID, bool) {
	if m == nil {
		return ospftypes.RouterID{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, s := range m.helping {
		if key.iface == iface && s.wasDR {
			return key.router, true
		}
	}
	return ospftypes.RouterID{}, false
}

// helperGraceEnd returns the grace deadline for a helper session (for tests + show).
func (m *grManager) helperGraceEnd(key helperKey) (time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.helping[key]
	if !ok {
		return time.Time{}, false
	}
	return s.graceEnd, true
}

// helperSessionCount returns the number of active helper sessions (for tests + show).
func (m *grManager) helperSessionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.helping)
}

// sessionsOnIfaceLocked counts helper sessions on iface (caller holds mu).
func (m *grManager) sessionsOnIfaceLocked(iface string) int {
	n := 0
	for key := range m.helping {
		if key.iface == iface {
			n++
		}
	}
	return n
}
