// Design: plan/learned/923-isis-8-dis-broadcast.md -- Designated IS (DIS) election on
// broadcast circuits, with per-level state and election damping.
//
// ISO/IEC 10589 clause 8.4.5: on a broadcast (multi-access LAN) circuit one IS
// per level is elected the Designated IS. The election compares (priority, MAC):
// the highest DIS priority (0..127) wins; an equal priority is broken by the
// higher SNPA (source MAC). The election runs independently per level, so one
// node can be DIS for L1, L2, both, or neither on the same circuit.
//
// This file is the PURE election logic and the per-level DIS state the broadcast
// circuit holds. It performs NO I/O and reads NO live table: the circuit gathers
// the candidate set (its own (priority, MAC) plus each Up LAN neighbor's) and
// calls Elect; the engine (root package) reacts to a role change by originating
// or purging the pseudo-node LSP (lsdb/pseudonode.go) and driving the LAN CSNP
// cadence (spec-isis-7 CSNP). Election is damped (R-1): a transient candidate
// change inside the damping window does not flip the role and re-churn the
// pseudo-node LSP.

package circuit

import (
	"bytes"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// Candidate is one participant in a DIS election on a circuit at a level: a
// router that advertised a (priority, SNPA) in its LAN IIH, or the local node
// itself. The election compares Candidates by (Priority desc, SNPA desc); the
// SystemID identifies the winner so the engine can name the pseudo-node and the
// member list. Local marks the local node's own candidate (so Elect can report
// whether WE won without comparing System IDs).
type Candidate struct {
	// SystemID is the candidate's System ID (the pseudo-node System ID when it
	// wins, and a member of the pseudo-node LSP regardless).
	SystemID types.SystemID
	// SNPA is the candidate's source MAC: the DIS-election tiebreak key and the
	// LAN three-way identity. The local node's own SNPA is its circuit MAC.
	SNPA adjacency.SNPA
	// Priority is the advertised DIS priority (0..127). A higher value wins; 0 is
	// valid and only lowers preference (AC-8), it does not forbid winning.
	Priority uint8
	// Local marks the local node's own candidate in the set.
	Local bool
}

// ElectionResult is the outcome of one election pass at one level.
type ElectionResult struct {
	// Winner is the elected DIS candidate. HasWinner is false only when the
	// candidate set was empty (no local node and no neighbors), which cannot
	// happen for a live circuit (the local node is always a candidate).
	Winner    Candidate
	HasWinner bool
	// IsLocalDIS reports whether the local node is the elected DIS at this level.
	IsLocalDIS bool
	// Changed reports whether the elected DIS (by System ID + pseudonode identity)
	// differs from the previously committed winner -- i.e. a role transition that
	// the engine must act on (originate / purge the pseudo-node LSP). It is false
	// when the same DIS is re-elected (a no-op refresh) or when damping holds the
	// previous winner across a transient change.
	Changed bool
	// LostRole reports that the local node WAS the DIS and is no longer: the engine
	// must purge the pseudo-node LSP it originated before yielding (R-2).
	LostRole bool
	// GainedRole reports that the local node was NOT the DIS and now is: the engine
	// must originate the pseudo-node LSP.
	GainedRole bool
}

// better reports whether candidate a beats candidate b in the DIS election:
// higher priority wins; an equal priority is broken by the higher SNPA (ISO/IEC
// 10589 clause 8.4.5). The comparison is total and deterministic so a stable
// candidate set always elects the same DIS on every node.
func (a Candidate) better(b Candidate) bool {
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	// Equal priority: the numerically higher SNPA (big-endian MAC) wins.
	return bytes.Compare(a.SNPA[:], b.SNPA[:]) > 0
}

// elect picks the winning Candidate from cands by (priority, SNPA). It returns
// the winner and whether one exists (false only for an empty set). It is the
// pure comparison with no state; DISState.Elect wraps it with damping and role
// transition tracking.
func elect(cands []Candidate) (Candidate, bool) {
	if len(cands) == 0 {
		return Candidate{}, false
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.better(best) {
			best = c
		}
	}
	return best, true
}

// DISState is the per-level DIS election state a broadcast circuit holds. There
// is one DISState per level the circuit forms (L1 and L2 are independent, R-4).
// It records the currently committed DIS and the damping deadline so a transient
// candidate change does not immediately re-elect and re-churn the pseudo-node
// LSP (R-1). A point-to-point circuit holds no DISState (no DIS, no pseudo-node).
type DISState struct {
	// committed is the DIS election result currently in effect (the DIS whose
	// pseudo-node LSP, if any, is live). Zero value = no DIS elected yet.
	committed  Candidate
	hasCommit  bool
	localIsDIS bool
	// pendingSince is when a candidate set first elected a winner DIFFERENT from
	// the committed one. The new winner is only committed once it has persisted for
	// the damping window (Elect called again past pendingSince+damp with the same
	// pending winner). Zero when no change is pending.
	pending      Candidate
	hasPending   bool
	pendingSince time.Time
}

// DISIdentity is the comparable identity of an elected DIS at a level: the
// winner's System ID. Two election results name the same DIS iff their winners'
// System IDs match (the pseudo-node System ID is the DIS System ID; clause
// 8.4.5). Used to decide whether a role actually changed.
func disIdentity(c Candidate) types.SystemID { return c.SystemID }

// Elect runs one election pass over cands at this state's level and applies
// damping. damp is the damping window (R-1): a winner that differs from the
// committed DIS must persist across damp before it is committed; a flap back to
// the committed winner inside the window cancels the pending change. now is the
// current time (a fake clock in tests).
//
// Election order is (priority desc, SNPA desc), ISO/IEC 10589 clause 8.4.5. The
// returned ElectionResult reports the live winner, whether the local node is the
// DIS, and the role-transition flags (Changed / GainedRole / LostRole) the
// engine acts on. A damp of 0 commits immediately (no damping), used by tests
// that assert the raw election.
func (s *DISState) Elect(cands []Candidate, damp time.Duration, now time.Time) ElectionResult {
	winner, ok := elect(cands)
	if !ok {
		// Empty candidate set: a live circuit always includes the local node, so
		// this only happens for a torn-down circuit. Report no winner without
		// disturbing the committed state.
		return ElectionResult{}
	}

	// First election: commit immediately (there is nothing to damp against).
	if !s.hasCommit {
		return s.commit(winner)
	}

	// Same DIS re-elected: refresh the committed winner's (priority, MAC) in case
	// they changed without changing the elected node, and clear any pending change.
	if disIdentity(winner) == disIdentity(s.committed) {
		s.committed = winner
		s.localIsDIS = winner.Local
		s.hasPending = false
		return ElectionResult{
			Winner:     winner,
			HasWinner:  true,
			IsLocalDIS: winner.Local,
			Changed:    false,
		}
	}

	// A DIFFERENT winner is elected. With no damping, commit at once.
	if damp <= 0 {
		return s.commit(winner)
	}

	// Damping: start (or continue) the pending window. Commit only once the same
	// pending winner has persisted past the window.
	if !s.hasPending || disIdentity(s.pending) != disIdentity(winner) {
		s.pending = winner
		s.hasPending = true
		s.pendingSince = now
		// Hold the committed DIS for now (the transient change does not flip).
		return s.hold()
	}
	// The pending winner is unchanged; commit it once the window has elapsed.
	if now.Sub(s.pendingSince) >= damp {
		s.hasPending = false
		return s.commit(winner)
	}
	// Still inside the window: keep holding the committed DIS.
	return s.hold()
}

// commit installs winner as the committed DIS and returns the transition result,
// computing GainedRole / LostRole against the prior local-DIS state.
func (s *DISState) commit(winner Candidate) ElectionResult {
	prevLocalDIS := s.hasCommit && s.localIsDIS
	prevWinner := s.committed
	prevHad := s.hasCommit

	s.committed = winner
	s.hasCommit = true
	s.localIsDIS = winner.Local
	s.hasPending = false

	changed := !prevHad || disIdentity(prevWinner) != disIdentity(winner)
	return ElectionResult{
		Winner:     winner,
		HasWinner:  true,
		IsLocalDIS: winner.Local,
		Changed:    changed,
		GainedRole: winner.Local && !prevLocalDIS,
		LostRole:   !winner.Local && prevLocalDIS,
	}
}

// hold returns the committed DIS unchanged (damping is suppressing a transient
// change). No role transition is reported.
func (s *DISState) hold() ElectionResult {
	return ElectionResult{
		Winner:     s.committed,
		HasWinner:  s.hasCommit,
		IsLocalDIS: s.localIsDIS,
		Changed:    false,
	}
}

// Reset clears the committed and pending state (used when a circuit is torn down
// or a level stops forming so a fresh election starts clean).
func (s *DISState) Reset() { *s = DISState{} }

// IsLocalDIS reports whether the local node is the committed DIS at this level.
func (s *DISState) IsLocalDIS() bool { return s.hasCommit && s.localIsDIS }

// DIS returns the committed DIS candidate and whether one is committed.
func (s *DISState) DIS() (Candidate, bool) { return s.committed, s.hasCommit }

// ---- Circuit-facing election API (broadcast only) ----

// IsBroadcast reports whether this is a broadcast circuit (the only kind that
// runs a DIS election). The engine checks it before calling the election API.
func (c *Circuit) IsBroadcast() bool { return c.kind == adjacency.KindBroadcast }

// candidates gathers the DIS election candidate set for level: the local node's
// own (priority, MAC) plus every Up LAN neighbor's (priority, MAC) at that level
// from the adjacency table (spec-isis-5). localPriority is the resolved per-level
// DIS priority the engine passes (the per-level override, else the circuit-wide
// value). A neighbor not in the Up state is not a candidate (ISO/IEC 10589 clause
// 8.4.5: only Up adjacencies participate; security review: a node cannot force
// itself DIS beyond a live, Up adjacency advertising a higher (priority, MAC)).
func (c *Circuit) candidates(level adjacency.Level, localPriority uint8) []Candidate {
	cands := []Candidate{{
		SystemID: c.systemID,
		SNPA:     c.snpa,
		Priority: localPriority,
		Local:    true,
	}}
	c.table.Each(func(a *adjacency.Adjacency) {
		if a.Level != level || a.State != adjacency.StateUp {
			return
		}
		cands = append(cands, Candidate{
			SystemID: a.SystemID,
			SNPA:     a.SNPA,
			Priority: a.Priority,
			Local:    false,
		})
	})
	return cands
}

// RunElection runs (and damps) the DIS election for level on this broadcast
// circuit and returns the result. localPriority is the resolved per-level DIS
// priority; damp is the damping window (R-1); now is the current time. It is a
// no-op (HasWinner false) on a point-to-point circuit or a level the circuit
// does not form. The engine calls it on every relevant trigger (a LAN Hello
// receipt, a neighbor loss, a local priority change) and reacts to the
// transition flags (GainedRole / LostRole) by originating or purging the
// pseudo-node LSP.
func (c *Circuit) RunElection(level adjacency.Level, localPriority uint8, damp time.Duration, now time.Time) ElectionResult {
	c.mu.Lock()
	st := c.dis[level]
	c.mu.Unlock()
	if st == nil {
		return ElectionResult{} // P2P, or a level this circuit does not form.
	}
	cands := c.candidates(level, localPriority)
	c.mu.Lock()
	res := st.Elect(cands, damp, now)
	c.mu.Unlock()
	return res
}

// LocalIsDIS reports whether the local node is the committed DIS for level on
// this circuit (false for P2P or an unformed level). Used by the engine's LAN
// CSNP cadence and the own-LSP star encoding to know which circuits this node is
// the DIS for.
func (c *Circuit) LocalIsDIS(level adjacency.Level) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.dis[level]
	if st == nil {
		return false
	}
	return st.IsLocalDIS()
}

// DISLevels returns the levels this circuit runs a DIS election for (its formed
// levels on a broadcast circuit; empty on P2P). Used by the engine to iterate the
// per-level election after a trigger.
func (c *Circuit) DISLevels() []adjacency.Level {
	if c.kind != adjacency.KindBroadcast {
		return nil
	}
	return c.levels
}

// MembersSnapshot returns the System IDs of every router on the segment at level
// -- the local node plus each Up LAN neighbor -- in deterministic (sorted) order.
// It is the member list the DIS puts in the pseudo-node LSP (every router at
// metric 0, including the DIS itself; ISO/IEC 10589 clause 8.4.5). The local node
// is always included. Returned by value (no live pointer crosses the boundary).
func (c *Circuit) MembersSnapshot(level adjacency.Level) []types.SystemID {
	members := []types.SystemID{c.systemID}
	seen := map[types.SystemID]struct{}{c.systemID: {}}
	c.table.Each(func(a *adjacency.Adjacency) {
		if a.Level != level || a.State != adjacency.StateUp {
			return
		}
		if _, dup := seen[a.SystemID]; dup {
			return
		}
		seen[a.SystemID] = struct{}{}
		members = append(members, a.SystemID)
	})
	sortSystemIDs(members)
	return members
}

// sortSystemIDs sorts a System ID slice in ascending byte order so the
// pseudo-node member list (and thus the originated LSP bytes) is deterministic.
func sortSystemIDs(ids []types.SystemID) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && bytes.Compare(ids[j-1][:], ids[j][:]) > 0; j-- {
			ids[j-1], ids[j] = ids[j], ids[j-1]
		}
	}
}
