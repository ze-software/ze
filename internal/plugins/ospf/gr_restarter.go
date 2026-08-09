// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- GR restarting-router state machine.
// Related: gr.go (grManager), gr_nvs.go (restart fact), origination_v6.go (v6 self-LSA gate).
// RFC: rfc/short/rfc3623.md sec 2 (in-restart suppression), sec 2.1 (originate Grace-LSAs,
//
//	keep FIB, store NVS), sec 2.2 (three exit triggers), sec 2.3 (exit actions);
//	rfc/short/rfc5187.md sec 3.1/3.2 (OSPFv3 LSA-ID + Interface-ID preservation).
package ospf

import (
	"errors"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

// errGRRestarterDisabled is returned by prepareRestart when the restarter is not configured.
var errGRRestarterDisabled = errors.New("ospf: graceful-restart restarter is disabled")

// prepareRestart begins a planned graceful restart (RFC 3623 sec 2.1): it ensures the FIB is
// left in place (the ensuing engine stop skips RemoveAll via gracefulStop), records the
// pre-restart Full adjacencies + the OSPFv3 preservation maps, persists the NVS restart fact,
// and originates one Grace-LSA per interface (LS age 0). The engine then stops; on resume,
// resumeFromNVS enters in-restart mode. It refuses when the restarter is disabled (AC-25).
func (m *grManager) prepareRestart(reason uint8) error { //nolint:unparam // RFC 3623 sec A / RFC 5187 sec 2.2 grace reason code: it is persisted in restartFact.Reason and encoded into the Grace-LSA by grOriginateGraceLSAs. Only grReasonReload has a caller today, but grReasonSoftwareRestart/RedundantCP are protocol-valid; dropping the parameter would hardcode a wire field
	m.mu.Lock()
	cfg := m.cfg
	m.mu.Unlock()
	if !cfg.restarterEnabled() {
		return errGRRestarterDisabled
	}
	graceEnd := m.now().Add(time.Duration(cfg.RestartInterval) * time.Second)
	expected := m.e.currentFullNeighbors()

	fact := restartFact{
		Restarting:   true,
		GraceEndUnix: graceEnd.Unix(),
		Reason:       reason,
		Expected:     routerIDsToStrings(expected),
		InterfaceIDs: m.e.captureInterfaceIDs(),
		PrefixLSIDs:  m.e.capturePrefixLSIDs(),
	}
	m.persistFact(fact)

	// Retain the FIB across the ensuing stop, and suppress route churn from now on.
	m.mu.Lock()
	m.gracefulStop = true
	m.mu.Unlock()

	// Originate one Grace-LSA per interface (LS age 0). The LSDB reliably retransmits the
	// same instance; it is never re-originated, so LS age is not reset (RFC 3623 sec A, R-5).
	ifs := m.e.grOriginateGraceLSAs(cfg.RestartInterval, reason, false)
	m.mu.Lock()
	m.metrics.graceLSAs.With(m.e.grFamilyLabel(), "originated").Set(float64(len(ifs)))
	m.mu.Unlock()
	return nil
}

// grPrepareResult is the JSON payload the operator `request ospf graceful-restart` command
// returns: the address family it ran on and whether the planned restart (RFC 3623 sec 2.1) was
// prepared, or a refusal message when graceful-restart is not configured (AC-25).
type grPrepareResult struct {
	Action   string `json:"action"`
	Family   string `json:"family"`
	Prepared bool   `json:"prepared"`
	Error    string `json:"error,omitempty"`
}

// grPrepare runs the operator-triggered planned graceful restart (RFC 3623 sec 2.1, reason =
// reload) invoked by the `request ospf graceful-restart` command: prepareRestart originates one
// Grace-LSA per interface, persists the NVS restart fact, and enters the graceful-stop
// suppression state so the ensuing engine stop retains the FIB. A disabled restarter is reported
// (not errored) so the CLI prints a clean refusal rather than a transport-level error.
func (e *engine) grPrepare() grPrepareResult {
	res := grPrepareResult{Action: cmdGRPrepare, Family: e.grFamilyLabel()}
	if err := e.gr.prepareRestart(grReasonReload); err != nil {
		res.Error = err.Error()
		return res
	}
	res.Prepared = true
	return res
}

// grUnplannedReason is the RFC 3623 sec 5 restart reason for an unplanned (cold) restart: it
// MUST be 0 (unknown) or 3 (switch to redundant control processor). Ze uses 3.
func grUnplannedReason() uint8 { return grReasonRedundantCP }

// maybeUnplannedRestart handles the RFC 3623 sec 5 unplanned-outage path: when the operator
// opted into planned-and-unplanned support and the engine came up WITHOUT a planned restart
// fact, it enters in-restart mode and originates Grace-LSAs (before any Hello) so neighbors
// keep advertising this router across the unexpected restart. Disabled by default (AC-22): a
// crashed router cannot guarantee FIB sanity, so unplanned support is opt-in only.
func (m *grManager) maybeUnplannedRestart() {
	m.mu.Lock()
	cfg := m.cfg
	already := m.restarting
	m.mu.Unlock()
	if already || !cfg.unplannedEnabled() {
		return
	}
	reason := grUnplannedReason()
	graceEnd := m.now().Add(time.Duration(cfg.RestartInterval) * time.Second)
	// Enter in-restart FIRST so origination is suppressed, then flood the Grace-LSAs before
	// interfaces start emitting Hellos (RFC 3623 sec 5).
	m.enterRestart(graceEnd, reason, m.e.currentFullNeighbors())
	ifs := m.e.grOriginateGraceLSAs(cfg.RestartInterval, reason, false)
	m.mu.Lock()
	m.metrics.graceLSAs.With(m.e.grFamilyLabel(), "originated").Set(float64(len(ifs)))
	m.mu.Unlock()
}

// resumeFromNVS reads the restart fact on engine start; if it represents an in-flight restart
// whose grace window is still open, it restores the OSPFv3 preservation maps and enters
// in-restart mode. A stale (expired) or cleared fact is ignored and the engine boots normally
// (AC-6, R-10). Called once from the engine start path.
func (m *grManager) resumeFromNVS() {
	if m == nil {
		return
	}
	m.resumeOnce.Do(m.resumeFromNVSLocked)
}

func (m *grManager) resumeFromNVSLocked() {
	store, ok := openGRStore()
	if !ok {
		return
	}
	defer func() { _ = store.Close() }()
	fact, ok := readRestartFact(store, m.e.grFactKey())
	if !ok || !fact.active(m.now()) {
		return
	}
	// RFC 5187 sec 3.1/3.2: restore the preserved LSA-ID -> prefix map and Interface IDs so
	// re-originated LSAs match neighbor adjacency state and do not churn.
	m.e.restorePrefixLSIDs(fact.PrefixLSIDs)
	m.e.restoreInterfaceIDs(fact.InterfaceIDs)
	expected := stringsToRouterIDs(fact.Expected)
	m.enterRestart(time.Unix(fact.GraceEndUnix, 0), fact.Reason, expected)
}

// enterRestart puts the engine into RFC 3623 sec 2 in-restart mode: origination and route
// install are suppressed, the pre-restart FIB is retained, and the grace timer arms the
// grace-expiry exit trigger.
func (m *grManager) enterRestart(graceEnd time.Time, reason uint8, expected []ospftypes.RouterID) {
	m.mu.Lock()
	m.restarting = true
	m.graceEnd = graceEnd
	m.reason = reason
	m.expected = make(map[ospftypes.RouterID]bool, len(expected))
	for _, r := range expected {
		m.expected[r] = false
	}
	if m.restartTimer != nil {
		m.restartTimer.Stop()
	}
	d := graceEnd.Sub(m.now())
	if d > 0 {
		m.restartTimer = time.AfterFunc(d, m.graceExpired)
	}
	m.metrics.restarterActive.With(m.e.grFamilyLabel()).Set(1)
	m.mu.Unlock()
}

// noteAdjacencyFull records a pre-restart neighbor re-reaching Full. Once every expected
// adjacency is back, the restarter exits (RFC 3623 sec 2.2 trigger 1). A restart with no
// recorded expected adjacencies relies on the inconsistent-LSA or grace-expiry triggers.
func (m *grManager) noteAdjacencyFull(router ospftypes.RouterID) {
	m.mu.Lock()
	if !m.restarting {
		m.mu.Unlock()
		return
	}
	if _, tracked := m.expected[router]; tracked {
		m.expected[router] = true
	}
	all := len(m.expected) > 0
	for _, reached := range m.expected {
		if !reached {
			all = false
			break
		}
	}
	m.mu.Unlock()
	if all {
		m.exitRestart(grExitAdjacencies)
	}
}

// grNeighborFull bridges the engine's neighbor-reached-Full event (the AF-neutral onFull sink,
// bfd_client.go neighborEventSinkValue) into the restarter's RFC 3623 sec 2.2 trigger-1 exit:
// once every pre-restart adjacency is Full again the restarter exits early instead of waiting
// for the grace timer. Inert unless this engine is itself restarting (noteAdjacencyFull returns
// early). Nil-safe so a pre-config engine costs nothing.
func (e *engine) grNeighborFull(snap ospfneighbor.Snapshot) {
	if e.gr == nil {
		return
	}
	id, err := ospftypes.ParseRouterID(snap.RouterID)
	if err != nil {
		return
	}
	e.gr.noteAdjacencyFull(id)
}

// noteInconsistentLSA triggers the RFC 3623 sec 2.2 exit trigger 2: an LSA inconsistent with
// the pre-restart Router-LSA was received (e.g. a neighbor no longer lists the link to us).
//
// DEFERRED wiring (intentional): this trigger has no production caller. Reliably detecting a
// TRULY inconsistent received LSA means comparing each neighbor's re-originated Router-LSA
// against this router's pre-restart topology (did it drop the link back to us?). The generic
// LSDB content-change observer (onContentChange) fires on ANY change and cannot distinguish a
// benign unrelated update from a real inconsistency, so wiring it here would cause FALSE early
// exits during a healthy restart -- exactly the outcome RFC 3623 sec 2.2 warns against. Until a
// topology-diff heuristic is validated end-to-end (QEMU interop), the trigger stays a decision
// method: the adjacency-re-Full trigger (trigger 1, wired via grNeighborFull) and the
// grace-expiry timer (trigger 3) are the production backstops.
func (m *grManager) noteInconsistentLSA() {
	if m.inRestart() {
		m.exitRestart(grExitInconsistent)
	}
}

// graceExpired is the RFC 3623 sec 2.2 exit trigger 3: the grace period elapsed before all
// adjacencies were re-established. Fired by the grace timer.
func (m *grManager) graceExpired() {
	if m.inRestart() {
		m.exitRestart(grExitGraceExpiry)
	}
}

// exitRestart runs the RFC 3623 sec 2.3 exit actions: clear the in-restart flag, re-originate
// self-LSAs (all areas; the OSPFv3 path reuses the preserved Interface-IDs / LSA-IDs), resume
// route install so SPF re-programs the FIB (the fib-kernel sweep refreshes the RTPROT_ZE
// routes instead of deleting them), flush the router's own Grace-LSAs, and clear the NVS fact.
func (m *grManager) exitRestart(reason string) {
	m.mu.Lock()
	if !m.restarting {
		m.mu.Unlock()
		return
	}
	m.restarting = false
	m.gracefulStop = false
	if m.restartTimer != nil {
		m.restartTimer.Stop()
		m.restartTimer = nil
	}
	// Snapshot the fields the post-unlock exit actions read WHILE the lock is held: a
	// concurrent configure() can rewrite m.cfg and enterRestart() can rewrite m.reason, so
	// reading them after the Unlock would be a data race (they feed grOriginateGraceLSAs below).
	restartInterval := m.cfg.RestartInterval
	exitReason := m.reason
	m.metrics.restarterActive.With(m.e.grFamilyLabel()).Set(0)
	m.metrics.restarterExits.With(m.e.grFamilyLabel(), reason).Inc()
	m.metrics.graceLSAs.With(m.e.grFamilyLabel(), "originated").Set(0)
	m.mu.Unlock()

	// Re-originate self-LSAs now that suppression is cleared (RFC 3623 sec 2.3). This also
	// re-runs the normal stale-flush of self-LSAs the origination path performs.
	m.e.originateSelfLSAs()
	// Flush the router's own Grace-LSAs at MaxAge (RFC 3623 sec 2.3).
	m.e.grOriginateGraceLSAs(restartInterval, exitReason, true)
	// Resume route install: recompute SPF for every area so Apply (no longer suppressed)
	// re-programs the FIB and the fib-kernel sweep refreshes the retained routes.
	m.e.triggerAllSPF()
	// Clear the NVS fact so a later unrelated restart does not re-enter in-restart (R-10).
	m.clearFact()
}

// persistFact writes the restart fact to NVS (best-effort: a failure only means a subsequent
// process restart boots normally instead of resuming, which is safe).
func (m *grManager) persistFact(fact restartFact) {
	store, ok := openGRStore()
	if !ok {
		return
	}
	defer func() { _ = store.Close() }()
	_ = writeRestartFact(store, m.e.grFactKey(), fact)
}

// clearFact records that no restart is in flight.
func (m *grManager) clearFact() {
	store, ok := openGRStore()
	if !ok {
		return
	}
	defer func() { _ = store.Close() }()
	_ = clearRestartFact(store, m.e.grFactKey())
}

// grOriginateGraceLSAs originates (or, when withdraw, MaxAge-flushes) one Grace-LSA per active
// OSPF interface and returns the interface names touched. IPv4 rides the ext-1 opaque
// link-store origination (Opaque Type 3 / ID 0); IPv6 rides the native link-scope origination
// (LS Type 0x000B, LS ID = Interface ID). This is the ONLY per-family fork in the restarter.
func (e *engine) grOriginateGraceLSAs(period uint16, reason uint8, withdraw bool) []string {
	router := e.currentRouterID()
	if router == (ospftypes.RouterID{}) {
		return nil
	}
	var ifs []string
	v6 := e.dispatch != nil && e.dispatch.codec.IsV6()
	topo := e.lsdbTopology()
	for i := range topo {
		info := &topo[i]
		if info.Passive || info.Name == "" {
			continue
		}
		if v6 {
			if !e.v6OriginateGraceLSA(router, info, uint32(period), reason, withdraw) {
				continue
			}
		} else {
			body := grV4Body(uint32(period), reason, info.Address, grSharedMedia(info.NetworkType))
			e.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
				Router:     router,
				OpaqueType: ospfpacket.GraceOpaqueType,
				OpaqueID:   0,
				Scope:      ospftypes.LSTypeOpaqueLink,
				Interface:  info.Name,
				Options:    ospftypes.OptionO,
				Body:       body,
				Withdraw:   withdraw,
			})
		}
		ifs = append(ifs, info.Name)
	}
	return ifs
}

// currentRouterID returns the engine's configured Router ID under the engine lock.
func (e *engine) currentRouterID() ospftypes.RouterID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.RouterID
}

// routerIDsToStrings / stringsToRouterIDs convert the expected-adjacency set to/from the NVS
// dotted-string form.
func routerIDsToStrings(ids []ospftypes.RouterID) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func stringsToRouterIDs(ss []string) []ospftypes.RouterID {
	out := make([]ospftypes.RouterID, 0, len(ss))
	for _, s := range ss {
		if id, err := ospftypes.ParseRouterID(s); err == nil {
			out = append(out, id)
		}
	}
	return out
}
