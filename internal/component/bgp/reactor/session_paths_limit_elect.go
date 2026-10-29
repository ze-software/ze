// Design: docs/guide/add-path.md -- the best paths under the peer's PATHS-LIMIT
// RFC: rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt -- Sections 1 and 3
// Related: session_paths_limit.go -- the send admission that marks a route contested
// Related: recovery_owed.go -- the worker that asks the RIB for the limited election
// Related: relay_recovery.go -- the staged admission that writes the election's result
package reactor

import (
	"cmp"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// An election for a contested route runs on the destination's recovery worker,
// never under writeMu: the send admission marks the route contested and owes
// it (owePathsLimitElections), the worker asks the RIB for at most the peer's
// limit of survivors in election order (recoveryWorker.decide), and the
// admission stages the demoted paths' withdrawals ahead of the survivors'
// announcements (queuePathsLimitElection). The owed record coalesces every
// trigger of one route, and its epoch is the per-route generation the
// admission re-checks under writeMu before it writes (recoveryAdmission.admit).

var errPathsLimitNoElector = errors.New("no RIB recovery owner is registered to rank the paths")

// pathsLimitOwesElection reports whether a committed journal entry owes the
// route an election: a send-side refusal or replacement on a contested route,
// or the release of a contested route's slot.
func pathsLimitOwesElection(change *pathsLimitChange) bool {
	if change.kind == pathsLimitContested {
		return true
	}
	return change.elect
}

// owePathsLimitElections owes, at this destination, an election for every
// route the committed admission contested, and wakes the recovery worker. It
// is a method of the write's ownership because it MUST run while that write
// holds the adj-RIB-Out table mutex (beginAdjOut to release): the writer
// MUST call it under writeMu and before ownership.release, once the admission
// that journaled changes is final and before releasing the journal, and MUST
// NOT call it after rollback. It MUST NOT take the table mutex itself, which
// the writer already holds: a sync.Mutex is not reentrant, and the writer
// parked on it forever with writeMu held. An election's own demotion owes one
// more election, which finds the survivors held and writes nothing: a
// recovery write cannot be told from the route server's own announcement
// election, which also writes under adjOutRecovery and must owe one.
//
// draft-abraitis-idr-addpath-paths-limit Section 1: "This indication allows
// the sender to optimize the transmission of BGP routes by selectively
// relaying pertinent routes instead of the entire set."
func (w *adjOutWrite) owePathsLimitElections(changes *pathsLimitChanges) {
	if changes.elections == 0 {
		return
	}
	s := w.session
	if !s.recoveryOwed() {
		for i := range changes.entries[:changes.n] {
			change := &changes.entries[i]
			if pathsLimitOwesElection(change) {
				// draft-abraitis-idr-addpath-paths-limit Section 3
				s.pathsLimitElectionFailed(change.family.family, errPathsLimitNoElector)
			}
		}
		return
	}
	if w.owePathsLimitRoutes(changes) {
		s.wakeRecovery()
	}
}

// owePathsLimitRoutes writes the owed records into the table the write holds
// and reports whether it owed any. A route is owed per prefix, identifier
// zero, as a departure owes it (oweDeparture), so both merge into one record.
// MUST be called with the table mutex held, as owePathsLimitElections is.
func (w *adjOutWrite) owePathsLimitRoutes(changes *pathsLimitChanges) bool {
	owed := false
	for i := range changes.entries[:changes.n] {
		change := &changes.entries[i]
		if !pathsLimitOwesElection(change) {
			continue
		}
		fam := change.family.family
		key := adjOutKey{addPath: true}
		if err := fwdPathKeyFor(&key.path, fam, 0, change.prefix.route, false, w.keyScratch[:]); err != nil {
			// draft-abraitis-idr-addpath-paths-limit Section 3
			w.session.pathsLimitElectionFailed(fam, err)
			continue
		}
		w.table.owe(key, w.session, fam, change.prefix.route)
		owed = true
	}
	return owed
}

// pathsLimitElectionFailed counts a contested route the RIB could not rank and
// warns once per session and family. The route keeps its arrival-order paths,
// which still honor the peer's limit (D-8). Safe for concurrent use.
func (s *Session) pathsLimitElectionFailed(fam family.Family, cause error) {
	if s.prefixMetrics != nil {
		s.prefixMetrics.pathsLimitElectionFailed.With(s.peerLabel(), fam.String()).Inc()
	}
	if _, warned := s.pathsLimitElectionWarned.LoadOrStore(fam, struct{}{}); warned {
		return
	}
	sessionLogger().Warn("PATHS-LIMIT: Ze could not rank the paths it sends under the peer's limit; the first paths to arrive are sent, not the best ones",
		"peer", s.settings.Address,
		"family", fam.String(),
		"cause", cause,
		"fix", "load the bgp-rib plugin and attach it to every source peer with receive [ update-received ]",
		"repeats", "suppressed for this family until the session restarts; ze_bgp_paths_limit_election_failed_total counts each route")
}

// pathsLimitBound is the peer's PATHS-LIMIT for fam on this session, read
// under writeMu. limited is false when this session enforces no limit for fam:
// the peer advertised none, ADD-PATH send is not negotiated for it, or Ze cannot
// count its paths. MUST NOT be called with writeMu held.
func (s *Session) pathsLimitBound(fam family.Family) (bound uint16, limited bool) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	state := s.pathsLimit[fam]
	if state == nil {
		return 0, false
	}
	return state.limit, true
}

// pathsLimitHeldPath is one path the destination holds for a contested route:
// the identifier Ze sent it under, and the ingress path it carries.
type pathsLimitHeldPath struct {
	id    uint32
	owner adjOutPath
	// rankable: a current source session owns the path, so the RIB ranks it
	// and an election may demote it.
	rankable bool
	// departed: the owed record excludes its ended source, as a departure
	// does, so the election withdraws it whatever the RIB answers.
	departed bool
}

// pathsLimitHeld reads, under writeMu and the table mutex, the paths this
// session holds for route in fam, in identifier order. settled clears the
// route's contested marker: the RIB holds no more paths than the bound, so no
// refusal left a better one unsent. A session whose state is gone answers no
// path; its admission is then settled gone (recoveryAdmission.admit).
func (s *Session) pathsLimitHeld(fam family.Family, route []byte, record *adjOutOwed, settled bool) ([]pathsLimitHeldPath, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	state := s.pathsLimit[fam]
	// refuse-by-default: guards recoveryAdmission.admit -- a session whose state is gone fails admit, which settles every staged item gone
	if state == nil {
		return nil, nil
	}
	key, err := state.key(route, state.keyScratch[:], false)
	if err != nil {
		return nil, err
	}
	prefix := state.prefixes[string(key)]
	// refuse-by-default: guards planPathsLimitElection -- a route holding no path has every slot free
	if prefix == nil {
		return nil, nil
	}
	if settled {
		prefix.contested = false
	}
	held := make([]pathsLimitHeldPath, 0, len(prefix.ids))
	table := s.adjOut
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	table.mu.Lock()
	defer table.mu.Unlock()

	for id := range prefix.ids {
		path := pathsLimitHeldPath{id: id}
		entryKey := adjOutKey{addPath: true}
		if err := fwdPathKeyFor(&entryKey.path, fam, id, route, false, scratch[:]); err != nil {
			return nil, err
		}
		if entry := table.routes[entryKey]; entry != nil {
			path.owner = entry.owner
			path.rankable, path.departed = pathsLimitHeldState(entry.owner, record)
		}
		held = append(held, path)
	}
	slices.SortFunc(held, func(a, b pathsLimitHeldPath) int { return cmp.Compare(a.id, b.id) })
	return held, nil
}

// pathsLimitHeldState classifies one held path. An originated path is neither
// rankable nor departed: the RIB does not report it, so it keeps its slot
// (D-7). A path an ended session owns is departed only when the record
// excludes its source; otherwise graceful restart retains it. Retention is a
// session fact (adjOutOwnerSessionCurrent): a live session's path whose import
// admission is superseded stays rankable, so the election can demote it, and
// never pins its slot as a retained path would. MUST be called with the table
// mutex held.
// RFC 4724 Section 4.2: "The router MUST NOT differentiate between stale and
// other routing information during forwarding."
func pathsLimitHeldState(owner adjOutPath, record *adjOutOwed) (rankable, departed bool) {
	if owner.source == nil {
		return false, false
	}
	if adjOutOwnerSessionCurrent(owner) {
		return true, false
	}
	_, excluded := record.exclusionOf(owner.source.Settings().Address)
	return false, excluded
}

// pinned reports a held path the election may not touch, so it holds its slot
// whatever the RIB answers: an originated path, which the RIB does not report
// (D-7), or one graceful restart retains for a source whose session ended.
// The RIB may still rank a retained path among its survivors once the source
// is back up, because RFC 4724 forbids telling stale routes apart; the
// election counts such a path once, as the slot it pins.
func (p *pathsLimitHeldPath) pinned() bool {
	if p.rankable {
		return false
	}
	return !p.departed
}

// pathsLimitElectionPlan is what one limited election changes at the
// destination: the survivors to announce, in election order, and the held
// paths to withdraw ahead of them.
type pathsLimitElectionPlan struct {
	announce  []int
	departed  []uint32
	outranked []uint32
	// ranked reports that the RIB named enough paths the election may place
	// to fill every slot it fills, so every rankable path it left out is
	// outranked.
	ranked bool
	// free is the slots the destination holds no path in.
	free int
}

// planPathsLimitElection decides one route. The slots the election fills are
// the peer's limit less the pinned paths; the RIB's first survivors that are
// not pinned take them. A survivor that is a pinned path already holds its
// slot, so it takes none of the rest: counting it twice left a slot empty
// until the next election, and the peer missed the next best path. A held path
// that is one of the survivors placed stays, and so keeps the identifier it
// was sent under.
//
// RFC 4724 Section 4.2: "The router MUST NOT differentiate between stale and
// other routing information during forwarding."
//
// draft-abraitis-idr-addpath-paths-limit Section 3: "A sender advertising
// multiple paths for the same prefix SHOULD send only the specified maximum
// number of paths indicated in the PATHS-LIMIT capability."
func planPathsLimitElection(held []pathsLimitHeldPath, survivors []ribevents.RecoveryRoute, limit uint16) pathsLimitElectionPlan {
	slots := int(limit)
	for i := range held {
		if held[i].pinned() {
			slots--
		}
	}
	slots = max(slots, 0)
	plan := pathsLimitElectionPlan{free: max(int(limit)-len(held), 0)}
	kept := make([]bool, len(held))
	placed := 0
	// Bounded by the RIB's answer, at most limit survivors.
	for i := range survivors {
		if placed == slots {
			break
		}
		j := pathsLimitHeldIndex(held, &survivors[i])
		if j < 0 {
			plan.announce = append(plan.announce, i)
			placed++
			continue
		}
		if held[j].pinned() {
			continue
		}
		kept[j] = true
		placed++
	}
	plan.ranked = placed == slots
	for j := range held {
		if kept[j] {
			continue
		}
		if held[j].departed {
			plan.departed = append(plan.departed, held[j].id)
			continue
		}
		if held[j].rankable {
			plan.outranked = append(plan.outranked, held[j].id)
		}
	}
	return plan
}

// pathsLimitHeldIndex is the held path that is survivor, or -1: the same
// source and the same received path identity.
func pathsLimitHeldIndex(held []pathsLimitHeldPath, survivor *ribevents.RecoveryRoute) int {
	for j := range held {
		owner := held[j].owner
		if owner.source == nil {
			continue
		}
		if owner.source.Settings().Address != survivor.Source {
			continue
		}
		if owner.received != survivor.PathID {
			continue
		}
		if owner.addPath != survivor.AddPath {
			continue
		}
		return j
	}
	return -1
}

// withdrawals names the held identifiers to withdraw ahead of the staged
// announcements: every departed one, and the outranked ones when the RIB
// filled the slots, else only enough of them to make room.
func (p *pathsLimitElectionPlan) withdrawals(staged int) []uint32 {
	withdraw := slices.Clone(p.departed)
	if p.ranked {
		return append(withdraw, p.outranked...)
	}
	room := staged - p.free - len(p.departed)
	return append(withdraw, p.outranked[:min(len(p.outranked), max(room, 0))]...)
}

// queuePathsLimitElection stages one contested route's election at a
// destination under a peer PATHS-LIMIT: each survivor the destination does not
// hold, through its source's ordinary rail, so it leaves under the identifier
// the forward rail minted for that source path (D-6), then the withdrawals,
// which it moves ahead of the announcements so the peer never holds more than
// its limit (D-5). A survivor the destination may not receive, whose source
// has no session to send it from, or whose original import admission is
// superseded (sourcePolicyCurrent), is skipped. Nothing staged is a decision too:
// the destination already holds the best paths.
//
// draft-abraitis-idr-addpath-paths-limit Section 1: "This indication allows
// the sender to optimize the transmission of BGP routes by selectively
// relaying pertinent routes instead of the entire set."
func (a *reactorAPIAdapter) queuePathsLimitElection(outcome *ribevents.RecoveryOutcome, key *recoveryAdmitKey, fam family.Family, admission *recoveryAdmission, sender plugin.Sender) error {
	route := key.record.nlri
	settled := len(outcome.Survivors) < int(admission.pathsLimit)
	held, err := admission.session.pathsLimitHeld(fam, route, key.record, settled)
	if err != nil {
		return err
	}
	// RFC 8212 Section 3 (sourcePolicyCurrent): a survivor admitted under a
	// superseded import policy is not eligible, so the election runs without
	// it, as without an export-refused one: its slot goes to the next path,
	// and a held path it was keeps no slot. The departed and outranked
	// withdrawals are still staged.
	survivors := outcome.Survivors
	if slices.ContainsFunc(survivors, a.policyStaleSurvivor) {
		survivors = slices.DeleteFunc(slices.Clone(survivors), a.policyStaleSurvivor)
	}
	// draft-abraitis-idr-addpath-paths-limit Section 3
	plan := planPathsLimitElection(held, survivors, admission.pathsLimit)
	first := len(admission.items)
	staged := 0
	for _, i := range plan.announce {
		survivor := &survivors[i]
		stored := rpc.StoredRoute{SourcePeer: survivor.Source.String(), Family: fam.String(),
			NLRIHex: hex.EncodeToString(survivor.NLRI), AttrHex: hex.EncodeToString(survivor.Attributes),
			NextHopHex: hex.EncodeToString(survivor.NextHop), MsgID: survivor.MessageID,
			SourcePolicyEpoch: survivor.SourcePolicyEpoch,
			NLRIFraming:       rpc.NLRIFramingSourceWire}
		start := len(admission.items)
		err := a.queueRecovery(admission.peer, &stored, admission, admission.sources.receipts, sender)
		if pathsLimitSurvivorSkipped(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !recoveryStagedAnnouncement(admission.items[start:]) {
			// An export policy denies it: the slot stays for the next path.
			a.releaseRecoveryItemsFrom(admission, start)
			continue
		}
		staged++
	}
	announced := len(admission.items)
	raw := make([]byte, 4+len(route))
	copy(raw[4:], route)
	for _, id := range plan.withdrawals(staged) {
		binary.BigEndian.PutUint32(raw, id)
		withdraw := rpc.StoredRoute{Family: fam.String(), NLRIHex: hex.EncodeToString(raw), Withdraw: true}
		if err := a.queueRecoveryWithdrawal(admission.peer, &withdraw, fam, admission, true); err != nil {
			return err
		}
	}
	// draft-abraitis-idr-addpath-paths-limit Section 3
	pathsLimitWithdrawFirst(admission.items[first:], announced-first)
	sessionLogger().Debug("PATHS-LIMIT election staged",
		"peer", admission.peer.Settings().Address, "family", fam.String(),
		"held", len(held), "survivors", len(outcome.Survivors), "announced", staged,
		"withdrawn", len(admission.items)-announced)
	return nil
}

// policyStaleSurvivor is recoverySurvivorPolicyStale over a survivor value,
// for the slices predicates of queuePathsLimitElection.
func (a *reactorAPIAdapter) policyStaleSurvivor(survivor ribevents.RecoveryRoute) bool {
	return a.recoverySurvivorPolicyStale(&survivor)
}

// pathsLimitSurvivorSkipped reports a survivor the forward rail cannot stage:
// every destination suppressed it, or its source has no session.
func pathsLimitSurvivorSkipped(err error) bool {
	if errors.Is(err, errAllDestinationsSuppressed) {
		return true
	}
	if errors.Is(err, errRelayNoSource) {
		return true
	}
	return errors.Is(err, errForwardNoSource)
}

// pathsLimitWithdrawFirst moves the items after the first announcements items
// ahead of them, keeping each group's order, so every write that frees a slot
// precedes every write that takes one: the peer never holds more paths than
// its limit, not even between two UPDATEs.
//
// draft-abraitis-idr-addpath-paths-limit Section 3: "A sender advertising
// multiple paths for the same prefix SHOULD send only the specified maximum
// number of paths indicated in the PATHS-LIMIT capability."
func pathsLimitWithdrawFirst(items []fwdItem, announcements int) {
	if announcements == 0 {
		return
	}
	if announcements == len(items) {
		return
	}
	slices.Reverse(items[:announcements])
	slices.Reverse(items[announcements:])
	slices.Reverse(items)
}
