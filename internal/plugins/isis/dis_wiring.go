// Design: docs/architecture/isis/isis-8-dis-broadcast.md -- engine <-> DIS-election wiring.
// Related: server.go -- the engine struct, dispatcher, and lifecycle this extends
// Related: circuits.go -- the broadcast circuits whose election this drives
// Related: lsdb_wiring.go -- own-LSP origination (the star encoding reads the DIS state)
// Related: flooding_wiring.go -- the LAN CSNP cadence the DIS sources
//
// This file is the root-package glue between the per-circuit DIS election
// (internal/plugins/isis/circuit/dis.go) and the LSDB pseudo-node origination
// (internal/plugins/isis/lsdb/pseudonode.go). On a broadcast circuit it runs
// the per-level election when an adjacency transitions, allocates a non-zero
// pseudonode ID when the local node becomes DIS, originates the pseudo-node LSP
// (members at metric 0) via the spec-isis-6 origination path, purges it on role
// loss (R-2), records the elected pseudo-node so the own LSP points at it (the
// star, AC-7), and sources the periodic LAN CSNP while DIS (spec-isis-7 CSNP).
//
// ISO/IEC 10589 clause 8.4.5: one IS per level is elected DIS by (priority, MAC);
// the DIS originates the pseudo-node LSP and drives the LAN synchronization.

package isis

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// disDampWindow is the DIS-election damping window (umbrella R-1): a candidate
// change must persist this long before the role transfers and the pseudo-node LSP
// is re-originated, so a flapping LAN does not churn the pseudo-node. It is short
// relative to typical convergence but long enough to absorb a transient Hello
// reorder. v1 fixes it (not YANG-tunable; spec Known Limitations).
const disDampWindow = 3 * time.Second

// lanCSNPInterval is how often the DIS sources a CSNP on each broadcast circuit
// it is DIS for, to keep the segment synchronized (ISO/IEC 10589 clause 7.3.15.2:
// the DIS periodically multicasts a complete sequence-numbers PDU on the LAN). It
// is the LAN counterpart of the P2P periodic CSNP (flooding_wiring.go), driven
// here because only the DIS sources it (clause 8.4.5).
const lanCSNPInterval = 10 * time.Second

// disKey identifies one per-level DIS role on one circuit: the interface name and
// the routing level (L1 and L2 elect independent DIS, R-4).
type disKey struct {
	name  string
	level lsdb.Level
}

// adjToAdjacencyLevel maps an lsdb.Level to the adjacency.Level used by the
// circuit's election API (the inverse of adjToLSDBLevel).
func adjToAdjacencyLevel(l lsdb.Level) adjacency.Level {
	if l == lsdb.Level2 {
		return adjacency.Level2
	}
	return adjacency.Level1
}

// runElection runs the DIS election for every level a broadcast circuit forms and
// acts on each role transition. It is called from the adjacency transition hooks
// (circuits.go) on a Hello-driven up/down and on circuit open/close. A
// point-to-point circuit has no DIS, so it is skipped. The engine:
//
//   - runs the per-level election (damped) over the circuit's candidate set;
//   - on GainedRole, allocates a non-zero pseudonode ID and originates the
//     pseudo-node LSP listing all segment members at metric 0;
//   - on LostRole, purges the pseudo-node LSP it originated and releases the ID;
//   - records the elected pseudo-node Source ID (DIS or not) so the own LSP
//     advertises the LAN as a single TLV 22 neighbor (the star, AC-7);
//   - re-originates the own LSP when the membership or the elected pseudo-node
//     changed, and bumps ze_isis_dis_elections_total on a DIS change.
//
// returns whether the own LSP must be re-originated (the caller batches one
// re-origination after processing all levels so a multi-level circuit originates
// once).
func (e *engine) runElection(c *circuit.Circuit) (reoriginate bool) {
	if !c.IsBroadcast() {
		return false
	}
	// Serialize the whole election reaction for ALL circuits through one engine
	// lock. runElection is called concurrently for the same circuit from the
	// receive, hold-timer-sweep, and DIS-loop goroutines; the per-circuit Elect is
	// atomic under the circuit mutex, but the follow-on pseudo-node allocate /
	// originate / record and the own-LSP re-origination decision below run OUTSIDE
	// it and could otherwise interleave to a stale outcome (e.g. a later election
	// commit landing its reaction before an earlier one). Holding electMu across the
	// commit AND its reaction makes each election+reaction atomic and ordered, so the
	// last election to commit is the one whose reaction lands -- a consistent
	// pseudo-node / own-LSP set (ISO/IEC 10589 clause 8.4.5). electMu is acquired
	// here FIRST; the body then takes the circuit mutex / e.mu / e.disMu (no
	// inversion). Election is a control-plane event, so serializing it is cheap.
	e.electMu.Lock()
	defer e.electMu.Unlock()
	now := time.Now()
	name := c.Name()
	for _, lvl := range c.DISLevels() {
		dl := adjToLSDBLevel(lvl)
		prio := e.disPriority(name, lvl)
		res := c.RunElection(lvl, prio, disDampWindow, now)
		if !res.HasWinner {
			continue
		}

		if res.Changed {
			e.disElections.With(dl.String()).Inc()
		}

		switch {
		case res.GainedRole:
			// Local node became DIS for this level: allocate a stable non-zero
			// pseudonode ID and originate the pseudo-node LSP.
			pnid := e.allocatePseudonodeID(name, dl)
			e.originatePseudonode(c, lvl, dl, pnid)
			reoriginate = true
		case res.LostRole:
			// Local node lost the DIS role: purge the pseudo-node LSP it originated
			// before yielding (R-2), then record the NEW elected pseudo-node so the
			// own LSP points at it.
			e.purgeLocalPseudonode(dl, name)
			e.recordElectedPseudonode(name, dl, res.Winner.SystemID, e.electedPseudonodeID(name, dl, res))
			reoriginate = true
		default:
			// Role unchanged for the local node, but the elected DIS (and thus the
			// pseudo-node identity) or the membership may have changed.
			before, had := e.lookupElectedPseudonode(name, dl)
			pnSys := res.Winner.SystemID
			pnid := e.electedPseudonodeID(name, dl, res)
			e.recordElectedPseudonode(name, dl, pnSys, pnid)
			after, _ := e.lookupElectedPseudonode(name, dl)
			if res.IsLocalDIS {
				// We are (still) the DIS: refresh the pseudo-node LSP so a membership
				// change is reflected (AC-4).
				e.originatePseudonode(c, lvl, dl, after.PseudonodeID())
				reoriginate = true
			} else if !had || before != after {
				// The pseudo-node we point at changed: re-originate the own LSP.
				reoriginate = true
			}
		}
	}
	e.publishPseudonodeMetric()
	return reoriginate
}

// electedPseudonodeID resolves the pseudonode ID for the currently elected DIS at
// (name, level). When the local node is the DIS it is our allocated ID; otherwise
// we do not know the remote DIS's chosen pseudonode ID from the election alone, so
// we learn it from the remote DIS's LAN IIH (the LANID field). Until learned we
// fall back to a deterministic non-zero placeholder derived from the circuit so
// the own LSP can still point at <dis-system-id>.<pnid>; once the DIS's IIH/LSP is
// seen the LANID provides the authoritative pseudonode ID.
func (e *engine) electedPseudonodeID(name string, level lsdb.Level, res circuit.ElectionResult) uint8 {
	if res.IsLocalDIS {
		return e.allocatePseudonodeID(name, level)
	}
	// A remote DIS: prefer the LAN ID it advertised in its IIH (the authoritative
	// pseudonode ID); fall back to the circuit-derived non-zero placeholder.
	if pnid, ok := e.learnedLANPseudonodeID(name, level); ok && pnid != 0 {
		return pnid
	}
	return nonZeroCircuitPseudonode(e.circuitIDFor(name), level)
}

// learnedLANPseudonodeID returns the pseudonode ID the elected DIS advertised in
// its LAN IIH (the LANID field), as observed by the circuit's adjacency table.
// The circuit records the DIS's advertised LANID; until an IIH carrying a non-zero
// LANID is seen this returns ok=false and the caller uses the placeholder.
//
// NOTE: the spec-isis-5 circuit does not yet surface the neighbor's advertised
// LANID through the adjacency table, so v1 always uses the deterministic
// placeholder pseudonode ID. This keeps the star encoding well-defined (a single
// TLV 22 entry at <dis>.<pnid>) and interoperable for a Ze-only LAN; learning the
// exact remote LANID for cross-vendor pseudo-node-ID matching is a spec-isis-13
// follow-up (it only affects which pseudonode octet a non-DIS Ze node names for a
// FOREIGN DIS, not Ze-to-Ze convergence where the DIS originates the authoritative
// pseudo-node LSP). Returns ok=false in v1.
func (e *engine) learnedLANPseudonodeID(string, lsdb.Level) (uint8, bool) {
	return 0, false
}

// nonZeroCircuitPseudonode derives a deterministic non-zero pseudonode octet for
// a circuit+level so L1 and L2 on the same circuit get DISTINCT pseudo-node IDs
// (the LAN ID is per level, R-4). It folds the circuit ID and the level into the
// 1..255 range, never 0 (0 means a real router).
func nonZeroCircuitPseudonode(cid lsdb.CircuitID, level lsdb.Level) uint8 {
	base := uint16(cid)*2 + uint16(level)
	v := uint8(base & 0xff)
	if v == 0 {
		v = 1
	}
	return v
}

// allocatePseudonodeID returns the stable non-zero pseudonode ID this node uses
// when it is the DIS at (name, level). It is derived deterministically from the
// circuit ID and the level so a re-election reuses the same ID (avoiding a churn
// of distinct pseudo-node LSP IDs) and L1/L2 differ (R-4).
func (e *engine) allocatePseudonodeID(name string, level lsdb.Level) uint8 {
	return nonZeroCircuitPseudonode(e.circuitIDFor(name), level)
}

// pnInput is the INPUT to a pseudo-node LSP origination: the LSP attributes plus
// the member set. OriginatePseudonode is a pure function of this pair (the
// sequence number aside, which the originator always bumps), so an identical input
// deterministically yields identical pseudo-node LSP bytes. The engine compares a
// freshly-built input against the last-originated one to skip a redundant
// re-origination -- the pseudo-node twin of origInput (lsdb_wiring.go). members is
// a value-type slice (types.SystemID is a comparable array), built by
// MembersSnapshot in deterministic sorted order, so a positional compare is a
// correct change detector.
type pnInput struct {
	pnid       uint8
	lifetime   uint16
	maxLSPSize int
	members    []types.SystemID
}

// pnInputEqual reports whether two pseudo-node origination inputs are equal: same
// pseudonode ID, LSP attributes, and member set (in MembersSnapshot's stable sorted
// order). Equality means OriginatePseudonode would produce the identical LSP.
func pnInputEqual(a, b pnInput) bool {
	if a.pnid != b.pnid || a.lifetime != b.lifetime || a.maxLSPSize != b.maxLSPSize {
		return false
	}
	if len(a.members) != len(b.members) {
		return false
	}
	for i := range a.members {
		if a.members[i] != b.members[i] {
			return false
		}
	}
	return true
}

// originatePseudonode builds and stores the pseudo-node LSP for the circuit at
// level (the local node is the DIS), arms SRM so spec-isis-7 floods it, and
// records the elected pseudo-node so the own LSP points at it. members are every
// router on the segment (the DIS included) at metric 0.
//
// It coalesces a redundant re-origination the same way the own LSP does
// (lsdb_wiring.go originate): the per-second re-election tick (reelectTick ->
// runElection) calls this on EVERY IsLocalDIS pass, so without coalescing a stable
// DIS would bump the pseudo-node sequence and re-flood it once per second. When the
// input is byte-identical to the last origination AND no refresh is yet due this is
// a no-op (only the idempotent elected-pseudo-node record is refreshed). A real
// membership change (a differing input) OR an elapsed refresh interval both fall
// through and re-originate -- the refresh path MUST re-stamp the LSP (bump the
// sequence, reset the Remaining Lifetime to MaxAge) so the pseudo-node never ages
// out of peers' LSDBs (clause 7.3.16.1, Bundle E deferred item 2).
func (e *engine) originatePseudonode(c *circuit.Circuit, lvl adjacency.Level, dl lsdb.Level, pnid uint8) {
	if e.originator == nil || pnid == 0 {
		return
	}
	e.mu.Lock()
	sys := e.cfg.SystemID
	lifetime := e.cfg.LSPLifetime
	cfg := e.cfg
	e.mu.Unlock()

	in := pnInput{
		pnid:       pnid,
		lifetime:   lifetime,
		maxLSPSize: e.minCircuitMTU(),
		members:    c.MembersSnapshot(lvl),
	}
	key := disKey{name: c.Name(), level: dl}
	// Skip a redundant re-origination when the input is unchanged and no refresh is
	// due; still (re)record the elected pseudo-node (cheap, idempotent) so the own
	// LSP star encoding always points at it even on the coalesced path.
	if e.pnOriginationUnchanged(key, in, dl, sys, cfg, time.Now()) {
		e.recordElectedPseudonode(c.Name(), dl, sys, pnid)
		return
	}

	info := lsdb.PseudonodeInfo{
		SystemID:     sys,
		PseudonodeID: pnid,
		Members:      in.members,
		MaxLifetime:  lifetime,
		MaxLSPSize:   in.maxLSPSize,
	}
	res := e.originator.OriginatePseudonode(dl, info)
	e.setPNLastOrigInput(key, in)
	levelTok := dl.String()
	for _, id := range res.Originated {
		e.armFlood(dl, id)
		e.emitLSPChange(levelTok, id.String(), uint32(e.sequenceOf(dl, id)), "add")
	}
	for _, id := range res.Purged {
		e.armFlood(dl, id)
		e.emitLSPChange(levelTok, id.String(), uint32(e.sequenceOf(dl, id)), "purge")
	}
	// Record the pseudo-node we (the DIS) originated so the own LSP star encoding
	// points at it.
	e.recordElectedPseudonode(c.Name(), dl, sys, pnid)
}

// pnOriginationUnchanged reports whether re-originating the pseudo-node LSP for key
// right now would be a pure no-op: the input matches the last origination, the
// stored pseudo-node fragment 0 is present and LIVE, AND no refresh is yet due. A
// first origination (no recorded input/timestamp), a changed member set, a missing
// or PURGED stored fragment 0, or an elapsed refresh interval all return false so
// origination proceeds. It is the pseudo-node twin of originationUnchanged
// (lsdb_wiring.go) and shares the same refreshInterval resolution, so the own LSP
// and the pseudo-node LSP refresh on the same cadence. Elapsed time is measured
// against the recorded last-origination timestamp (not the stored entry's Remaining
// Lifetime, which the aging goroutine mutates under the LSDB lock).
//
// The liveness guard is essential under DIS-election churn: a brief LostRole/regain
// flap purges then re-originates the pseudo-node within one re-election. The purge
// leaves a zero-age (purged) fragment 0 retained in the LSDB for the grace period,
// and clearPNLastOrigInput drops the tracking -- but a follow-on coalesced path that
// only checked the recorded input could otherwise suppress the live re-origination
// and leave the pseudo-node stuck purged in the originator's own LSDB (and never
// re-flooded live to peers). Requiring a present, non-purged stored fragment 0
// before coalescing means a purged pseudo-node is ALWAYS re-originated live, while a
// stable DIS still coalesces away the per-second re-election re-flood.
func (e *engine) pnOriginationUnchanged(key disKey, in pnInput, dl lsdb.Level, sys types.SystemID, cfg Config, now time.Time) bool {
	e.disMu.Lock()
	prev, ok := e.pnLastInput[key]
	at, hadAt := e.pnLastOrigAt[key]
	e.disMu.Unlock()
	if !ok || !hadAt || !pnInputEqual(prev, in) {
		return false
	}
	// Never coalesce over a missing or purged own pseudo-node fragment 0: it must be
	// re-originated live so it returns to peers' LSDBs (the churn-purge recovery).
	frag0 := e.lsdb.Lookup(dl, types.NewLSPID(types.NewSourceID(sys, in.pnid), 0))
	if frag0 == nil || frag0.IsPurged() {
		return false
	}
	return now.Sub(at) < refreshInterval(cfg)
}

// setPNLastOrigInput records the pseudo-node origination input and timestamp for
// key so the next originatePseudonode can detect an unchanged input and skip the
// redundant re-flood (and force a refresh once the interval elapses). The member
// slice is freshly built by MembersSnapshot and not mutated afterward, so retaining
// it is safe.
func (e *engine) setPNLastOrigInput(key disKey, in pnInput) {
	e.disMu.Lock()
	if e.pnLastInput == nil {
		e.pnLastInput = make(map[disKey]pnInput)
	}
	if e.pnLastOrigAt == nil {
		e.pnLastOrigAt = make(map[disKey]time.Time)
	}
	e.pnLastInput[key] = in
	e.pnLastOrigAt[key] = time.Now()
	e.disMu.Unlock()
}

// clearPNLastOrigInput drops the recorded pseudo-node origination input/timestamp
// for key, so a later re-election that reuses the same pseudonode ID re-originates
// from scratch rather than coalescing against a stale input. Called when the local
// node yields or loses the DIS role (the pseudo-node it tracked is being purged).
func (e *engine) clearPNLastOrigInput(key disKey) {
	e.disMu.Lock()
	delete(e.pnLastInput, key)
	delete(e.pnLastOrigAt, key)
	e.disMu.Unlock()
}

// purgeLocalPseudonode purges the pseudo-node LSP this node originated as the DIS
// at (name, level) -- it has lost the role (R-2). It arms SRM so the purge floods
// and emits change events. A no-op when this node was not the DIS (no allocated
// pseudo-node).
func (e *engine) purgeLocalPseudonode(level lsdb.Level, name string) {
	if e.originator == nil {
		return
	}
	e.mu.Lock()
	sys := e.cfg.SystemID
	e.mu.Unlock()
	pnid := e.allocatePseudonodeID(name, level)
	purged := e.originator.PurgePseudonode(level, sys, pnid)
	// Drop the coalescing/refresh state for this pseudo-node: it no longer exists, so
	// a later re-election that reuses the pnid must re-originate from scratch (and not
	// coalesce against, or refresh from, the purged input).
	e.clearPNLastOrigInput(disKey{name: name, level: level})
	levelTok := level.String()
	for _, id := range purged {
		e.armFlood(level, id)
		e.emitLSPChange(levelTok, id.String(), uint32(e.sequenceOf(level, id)), "purge")
	}
}

// recordElectedPseudonode stores the elected pseudo-node Source ID for (name,
// level) so the own-LSP star encoding (lsdb_wiring.go levelState) advertises the
// LAN as a single TLV 22 neighbor pointing at it. A zero pnid records nothing (no
// pseudo-node yet).
func (e *engine) recordElectedPseudonode(name string, level lsdb.Level, disSys types.SystemID, pnid uint8) {
	if pnid == 0 {
		return
	}
	e.disMu.Lock()
	e.disPseudonode[disKey{name: name, level: level}] = types.NewSourceID(disSys, pnid)
	e.disMu.Unlock()
}

// lookupElectedPseudonode returns the elected pseudo-node Source ID for (name,
// level) and whether one is recorded.
func (e *engine) lookupElectedPseudonode(name string, level lsdb.Level) (types.SourceID, bool) {
	e.disMu.Lock()
	defer e.disMu.Unlock()
	src, ok := e.disPseudonode[disKey{name: name, level: level}]
	return src, ok
}

// clearElectedPseudonode drops the recorded pseudo-node for (name, level) (a
// circuit closing). The own LSP then reverts to per-peer for that circuit (none,
// since the circuit is gone).
func (e *engine) clearElectedPseudonode(name string, level lsdb.Level) {
	e.disMu.Lock()
	delete(e.disPseudonode, disKey{name: name, level: level})
	e.disMu.Unlock()
}

// clearCircuitDIS drops every per-level DIS record for a circuit and purges any
// pseudo-node LSP this node originated on it, on circuit close. Called from
// onCircuitDown before the own LSP is re-originated so a stale pseudo-node does
// not linger (R-2) and the star entry for the gone circuit disappears.
func (e *engine) clearCircuitDIS(name string) {
	for _, dl := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		e.purgeLocalPseudonode(dl, name)
		e.clearElectedPseudonode(name, dl)
	}
	e.publishPseudonodeMetric()
}

// circuitHasUpAt reports whether circuit c has at least one Up adjacency at the
// adjacency level. The own-LSP star encoding (lsdb_wiring.go) uses it so a
// broadcast circuit with a pseudo-node recorded but no Up neighbor at this level
// does not emit a dangling pseudo-node TLV 22 entry (the LAN is only represented
// once a member is actually Up). The local DIS itself is always Up-capable; an
// isolated DIS with no peers still has the pseudo-node recorded but lists only
// itself, which this guard correctly suppresses until a peer joins.
func (e *engine) circuitHasUpAt(c *circuit.Circuit, level adjacency.Level) bool {
	for _, row := range c.Table().Snapshot() {
		if row.Level == level.String() && row.State == adjacency.StateUp.String() {
			return true
		}
	}
	return false
}

// disPriority resolves the local node's DIS election priority for a circuit at a
// level: the per-level override when set (>0), else the circuit-wide priority,
// else the configured default. This lets a circuit prefer being DIS at one level
// and not the other (AC-9). 0 is a valid priority (lowest preference, AC-8); the
// "override when >0" rule mirrors levelMetric (a zero per-level field means "no
// override", not "priority 0").
func (e *engine) disPriority(name string, lvl adjacency.Level) uint8 {
	ic, ok := e.interfaceConfig(name)
	if !ok {
		return DefaultPriority
	}
	if lvl == adjacency.Level1 && ic.Level1.Priority > 0 {
		return ic.Level1.Priority
	}
	if lvl == adjacency.Level2 && ic.Level2.Priority > 0 {
		return ic.Level2.Priority
	}
	return ic.Priority
}

// startDISLoop launches the periodic LAN CSNP cadence: while the local node is
// DIS for a level on a broadcast circuit, it sources a CSNP on that circuit at
// lanCSNPInterval to keep the segment synchronized (ISO/IEC 10589 clause 7.3.15.2
// / clause 8.4.5). Re-running the election periodically also re-elects on DIS loss
// even if no Hello arrived to trigger a transition (a belt-and-braces re-check;
// the hold-timer sweep drops the lost DIS adjacency, and this pass observes it).
// The loop stops on ctx cancellation (shutdown).
func (e *engine) startDISLoop() {
	e.wg.Go(func() {
		csnp := time.NewTicker(lanCSNPInterval)
		reelect := time.NewTicker(sweepInterval)
		defer csnp.Stop()
		defer reelect.Stop()
		for {
			select {
			case <-e.ctx.Done():
				return
			case <-csnp.C:
				e.lanCSNPTick()
			case <-reelect.C:
				e.reelectTick()
			}
		}
	})
}

// reelectTick re-runs the DIS election on every broadcast circuit and re-originates
// the own LSP if any role/membership changed. It catches a DIS loss detected by
// the hold-timer sweep (an expired DIS adjacency removes a candidate) even when no
// new Hello triggered runElection, so re-election is prompt (AC-6).
func (e *engine) reelectTick() {
	e.circuitsMu.RLock()
	circuits := make([]*circuit.Circuit, 0, len(e.circuitByName))
	for _, c := range e.circuitByName {
		circuits = append(circuits, c)
	}
	e.circuitsMu.RUnlock()

	reorig := false
	for _, c := range circuits {
		if e.runElection(c) {
			reorig = true
		}
	}
	if reorig {
		e.originate()
	}
}

// pnRefreshTarget names one pseudo-node LSP due a periodic refresh: the broadcast
// circuit, its adjacency level, and the LSDB level. refreshDuePseudonodes returns
// these so refreshPseudonodes can re-originate each via the normal origination path.
type pnRefreshTarget struct {
	c   *circuit.Circuit
	lvl adjacency.Level
	dl  lsdb.Level
}

// refreshPseudonodes re-originates every pseudo-node LSP whose periodic refresh is
// due (ISO/IEC 10589 clause 7.3.16.1). It is called once per second by the aging
// loop (ageOnce): a DIS on a fully QUIESCENT LAN gets no Hello to drive
// reelectTick -> originatePseudonode, so without this the pseudo-node LSP it
// originated would age to MaxAge and purge -- the LAN would vanish from peers'
// SPF. refreshDuePseudonodes does the cheap timestamp compare; originatePseudonode
// then re-originates (its refresh-due path bumps the sequence and resets the
// lifetime, since the input is unchanged). A node that is not DIS anywhere finds no
// due targets and does nothing.
func (e *engine) refreshPseudonodes() {
	due := e.refreshDuePseudonodes(time.Now())
	if len(due) == 0 {
		return
	}
	e.electMu.Lock()
	defer e.electMu.Unlock()
	for _, t := range due {
		// Re-check the role under electMu: a concurrent re-election may have moved the
		// DIS between the probe and here. Only the current DIS re-originates.
		if !t.c.LocalIsDIS(t.lvl) {
			continue
		}
		e.originatePseudonode(t.c, t.lvl, t.dl, e.allocatePseudonodeID(t.c.Name(), t.dl))
	}
	e.publishPseudonodeMetric()
}

// refreshDuePseudonodes returns the pseudo-node LSPs due a periodic refresh as of
// now: a broadcast circuit at a level where the local node is the DIS and at least
// refreshInterval has elapsed since the last pseudo-node origination recorded for
// (circuit, level). A pseudo-node with no recorded origination timestamp is NOT
// reported (origination is driven by the election, not this probe). Exposed (rather
// than inlined in refreshPseudonodes) so a test can drive the refresh-due decision
// deterministically by planting pnLastOrigAt in the past, without sleeping for a
// real refresh interval. The decision reads only the role and the timestamps, so it
// is cheap enough to run every aging tick.
func (e *engine) refreshDuePseudonodes(now time.Time) []pnRefreshTarget {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	if !cfg.Present() {
		return nil
	}
	interval := refreshInterval(cfg)

	e.circuitsMu.RLock()
	circuits := make([]*circuit.Circuit, 0, len(e.circuitByName))
	for _, c := range e.circuitByName {
		circuits = append(circuits, c)
	}
	e.circuitsMu.RUnlock()

	var due []pnRefreshTarget
	for _, c := range circuits {
		if !c.IsBroadcast() {
			continue
		}
		for _, lvl := range c.DISLevels() {
			if !c.LocalIsDIS(lvl) {
				continue
			}
			dl := adjToLSDBLevel(lvl)
			e.disMu.Lock()
			at, ok := e.pnLastOrigAt[disKey{name: c.Name(), level: dl}]
			e.disMu.Unlock()
			if ok && now.Sub(at) >= interval {
				due = append(due, pnRefreshTarget{c: c, lvl: lvl, dl: dl})
			}
		}
	}
	return due
}

// lanCSNPTick sources a periodic CSNP on every broadcast circuit at each level the
// local node is the DIS for, using the spec-isis-7 CSNP build/send sourced from
// the pseudo-node Source ID (clause 8.4.5: the DIS sources the LAN CSNP). A
// non-DIS circuit (or P2P) is skipped (P2P periodic CSNP is in flooding_wiring.go).
func (e *engine) lanCSNPTick() {
	e.circuitsMu.RLock()
	circuits := make([]*circuit.Circuit, 0, len(e.circuitByName))
	for _, c := range e.circuitByName {
		circuits = append(circuits, c)
	}
	e.circuitsMu.RUnlock()

	for _, c := range circuits {
		if !c.IsBroadcast() {
			continue
		}
		fc, ok := e.floodCircuitFor(c.Name())
		if !ok || fc.Passive {
			continue
		}
		for _, lvl := range c.DISLevels() {
			if !c.LocalIsDIS(lvl) {
				continue
			}
			dl := adjToLSDBLevel(lvl)
			// Source the CSNP from the pseudo-node Source ID (the DIS's LAN ID), per
			// clause 8.4.5; fall back to the node's own Source ID if not recorded.
			src := e.ownSourceID()
			if pn, ok := e.lookupElectedPseudonode(c.Name(), dl); ok {
				src = pn
			}
			e.flooder.SendCSNP(fc, dl, src)
		}
	}
}

// publishPseudonodeMetric refreshes the ze_isis_pseudonode_lsps gauge per level
// from the count of pseudo-node Source IDs this node currently originates as the
// DIS. Only pseudo-nodes whose System ID is ours are counted (a recorded foreign
// DIS's pseudo-node is not one WE originate).
func (e *engine) publishPseudonodeMetric() {
	e.mu.Lock()
	sys := e.cfg.SystemID
	e.mu.Unlock()

	counts := map[lsdb.Level]int{lsdb.Level1: 0, lsdb.Level2: 0}
	e.disMu.Lock()
	for k, src := range e.disPseudonode {
		if src.SystemID() == sys {
			counts[k.level]++
		}
	}
	g := e.pseudonodeG
	e.disMu.Unlock()

	g.With(lsdb.Level1.String()).Set(float64(counts[lsdb.Level1]))
	g.With(lsdb.Level2.String()).Set(float64(counts[lsdb.Level2]))
}
