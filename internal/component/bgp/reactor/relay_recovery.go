// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
// RFC: rfc/short/rfc4271.md -- Decision and Update-Send Process (Sections 9.1.3, 9.2)
// RFC: rfc/short/rfc4456.md -- reflection rules a recovery candidate is refused by
package reactor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errRecoveryNotWritten = errors.New("source-DOWN recovery operation was not written")

// errRecoveryDestinationClosed: the destination had no Established session
// when the admission reached its worker, so nothing was written. The session
// owning the keys is closing, and its close drops the owed set.
var errRecoveryDestinationClosed = errors.New("source-DOWN recovery destination session closed before the write")

// errRecoveryBufferPartial: the read buffer pool had no buffer for a key of
// the admission. The keys staged before it were decided; the rest stay owed,
// and the worker's next round asks for them once this admission's buffers are
// back in the pool.
var errRecoveryBufferPartial = errors.New("source-DOWN recovery deferred keys: read buffer pool exhausted")

// errRecoveryBufferStalled: the read buffer pool had no buffer for the first
// key of the admission, so nothing was decided. Other writes hold the buffers,
// and the worker backs off before its next round (recoveryStallMax).
var errRecoveryBufferStalled = errors.New("source-DOWN recovery decided nothing: read buffer pool exhausted")

// recoveryAdmission owns every route and post-policy section from one lookup
// of one destination session's owed keys. Producers MUST stage all items
// before dispatch; the destination worker MUST admit each key once under
// writeMu and complete the operation once at release. No IPC or RIB lookup
// runs under writeMu. This cold receipt is bounded by the lookup response and
// its policy output, not another persistent route inventory.
type recoveryAdmission struct {
	peer    *Peer
	session *Session
	// nlri is the advertisement of the key being staged, which an
	// egress-denied survivor withdraws.
	nlri  []byte
	items []fwdItem
	// itemKeys holds, per item, the index of the key it decides.
	itemKeys []int
	keys     []recoveryAdmitKey
	ctx      context.Context
	settled  bool
	err      error
	done     chan struct{}
	sources  map[*Peer]recoverySourceReceipt
}

// recoveryKeyOutcome is one owed key's outcome at its admission.
type recoveryKeyOutcome uint8

const (
	recoveryKeyUnspecified recoveryKeyOutcome = iota
	// recoveryKeyWrite: the decision is written (a survivor, or the owed
	// withdrawal), and the record is removed once the write is flushed. A
	// decision with no section (an ADD-PATH WITHDRAW naming no identifier)
	// is written by writing nothing, and its record is removed the same way.
	recoveryKeyWrite
	// recoveryKeyGone: the record left before the admission (a new owner, a
	// decision, a table reset); nothing is left owed.
	recoveryKeyGone
	// recoveryKeyLeftOwed: the epoch moved, or the chosen candidate withdrew
	// the key since the snapshot; the next round decides the record as it
	// stands.
	recoveryKeyLeftOwed
	// recoveryKeyReowed: the chosen survivor's session ended or restarted;
	// the key is owed again with that source excluded.
	recoveryKeyReowed
	// recoveryKeyHeld: decided with nothing to write, because the elected
	// candidate is the route the destination already holds (owner identity
	// and received message ID equal). The record is removed as for
	// recoveryKeyWrite.
	recoveryKeyHeld
)

// recoveryAdmitKey is one owed key of an admission, with the record and the
// epoch its round read.
type recoveryAdmitKey struct {
	key    adjOutKey
	record *adjOutOwed
	epoch  uint64
	// candidate is the survivor chosen for the key, with its received
	// message ID; a withdrawal decision has none (source nil).
	candidate adjOutPath
	outcome   recoveryKeyOutcome
	// gone is the source a re-owe excludes, for recoveryKeyReowed only.
	gone netip.Addr
}

// mark assigns key k to every item staged since the previous mark.
func (admission *recoveryAdmission) mark(k int) {
	for len(admission.itemKeys) < len(admission.items) {
		admission.itemKeys = append(admission.itemKeys, k)
	}
}

// settleGone settles the admission with every key gone: nothing it holds is
// written.
func (admission *recoveryAdmission) settleGone() {
	admission.settled = true
	for k := range admission.keys {
		admission.keys[k].outcome = recoveryKeyGone
	}
}

// admit decides, under the destination's writeMu, which keys of the admission
// are written. Per key the owed record must still be the one the round read,
// belong to this session, and carry the round's epoch; the chosen candidate
// must not have withdrawn the key since the snapshot, and its source must
// still be the live session the round captured. A candidate the destination
// already holds is decided with nothing to write. An announcement of the key
// since the snapshot does not stop the decision: settle re-owes it. It reports false
// when the admission belongs to another session, whose table reset dropped
// every key, or when the session is tearing down.
func (admission *recoveryAdmission) admit(session *Session) bool {
	if admission.session != session {
		admission.settleGone()
		return false
	}
	if session.tearingDown.Load() {
		admission.settleGone()
		return false
	}
	table := &admission.peer.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()

	for k := range admission.keys {
		key := &admission.keys[k]
		record := table.owed[key.key]
		if record != key.record {
			key.outcome = recoveryKeyGone
			continue
		}
		if record.session != session {
			// Unreachable while every reset drops the owed set; never left
			// owed, so it cannot start rounds forever.
			table.removeOwed(key.key, record)
			key.outcome = recoveryKeyGone
			continue
		}
		if record.epoch != key.epoch {
			key.outcome = recoveryKeyLeftOwed
			continue
		}
		if key.outcome == recoveryKeyReowed {
			continue
		}
		if key.candidate.source != nil && record.moved(key.candidate) {
			key.outcome = recoveryKeyLeftOwed
			continue
		}
		if adjOutHolds(table.routes[key.key], key.candidate) {
			// RFC 4271 Section 9.2: "All newly installed routes and all
			// newly unfeasible routes for which there is no replacement route
			// SHALL be advertised to its peers by means of an UPDATE
			// message." The elected route is not newly installed.
			key.outcome = recoveryKeyHeld
			continue
		}
		key.outcome = recoveryKeyWrite
	}
	// Fence every source before writing any section: a source whose session
	// ended or restarted re-owes its own key with that source excluded, and
	// touches that key only. A live source is current whatever it accepted
	// since the cut (receiveCutState).
	for j := range admission.items {
		section := &admission.items[j]
		if admission.keys[admission.itemKeys[j]].outcome != recoveryKeyWrite {
			continue
		}
		switch admission.sourceState(section) {
		case receiveCutCurrent:
		case receiveCutEnded:
			admission.reowe(j)
		case receiveCutUnspecified:
			panic("BUG: recovery source fence returned no cut state")
		}
	}
	return true
}

// adjOutHolds reports whether entry is candidate's route: the same owner
// identity and the same received revision. A withdrawal decision (no
// candidate) holds nothing.
func adjOutHolds(entry *adjRIBOutRoute, candidate adjOutPath) bool {
	if candidate.source == nil {
		return false
	}
	if entry == nil {
		return false
	}
	if !adjOutSameOwner(entry.owner, candidate) {
		return false
	}
	return entry.owner.message == candidate.message
}

// sourceState is the cut state of a section's source. A source whose session
// generation moved, or which the round never captured, has ended for this
// round. A withdrawal section has no source and is always current.
func (admission *recoveryAdmission) sourceState(section *fwdItem) receiveCutState {
	if !forwardSourceCurrent(section.receivedPeer, section.receivedGeneration) {
		return receiveCutEnded
	}
	if section.receivedPeer == nil {
		return receiveCutCurrent
	}
	receipt, present := admission.sources[section.receivedPeer]
	if !present {
		return receiveCutEnded
	}
	return receipt.cut.state(section.receivedPeer)
}

// writes reports whether item j's key is admitted for writing.
func (admission *recoveryAdmission) writes(j int) bool {
	return admission.keys[admission.itemKeys[j]].outcome == recoveryKeyWrite
}

// reowe re-owes item j's key with the item's source excluded. A key whose
// candidate has no source is a withdrawal and has no source to exclude.
func (admission *recoveryAdmission) reowe(j int) {
	key := &admission.keys[admission.itemKeys[j]]
	if admission.items[j].receivedPeer == nil {
		return
	}
	key.outcome = recoveryKeyReowed
	key.gone = admission.items[j].receivedPeer.Settings().Address
}

// settle applies every key's outcome once the admission's writes are flushed,
// under the destination's writeMu, so the record is removed in the same
// section that wrote its decision. A record whose epoch moved after admit (a
// departure merged) stays owed for the next round. A decided record that an
// announcement reached since the snapshot is RE-OWED BY A MOVER: the
// decision stands, as the RIB's best as drained, and the record stays owed
// with its epoch moved, so the next round elects with the newer route. The
// admission's own write notes no announcement, so no round re-triggers
// itself.
func (admission *recoveryAdmission) settle() {
	admission.settled = true
	table := &admission.peer.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()

	reowed := false
	for k := range admission.keys {
		key := &admission.keys[k]
		record := table.owed[key.key]
		if record != key.record {
			continue
		}
		switch key.outcome {
		case recoveryKeyWrite, recoveryKeyHeld:
			if record.epoch != key.epoch {
				continue
			}
			if record.announced {
				record.epoch++
				reowed = true
				continue
			}
			table.removeOwed(key.key, record)
		case recoveryKeyReowed:
			record.exclude(key.gone)
			reowed = true
		case recoveryKeyGone, recoveryKeyLeftOwed:
		case recoveryKeyUnspecified:
			panic("BUG: recovery key settled without an admission outcome")
		}
	}
	if reowed {
		admission.session.wakeRecovery()
	}
}

type recoverySourceReceipt struct {
	generation uint64
	cut        receivePublicationCut
}

// recoverySources binds selected bytes to the sessions present before lookup.
// The cold map is bounded by configured peers and covers both lookup transports.
func (a *reactorAPIAdapter) recoverySources() map[*Peer]recoverySourceReceipt {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()
	sources := make(map[*Peer]recoverySourceReceipt, len(a.r.peers))
	for _, peer := range a.r.peers {
		sources[peer] = recoverySourceReceipt{generation: peer.forwardGeneration.Load(),
			cut: peer.receiveCut()}
	}
	return sources
}

// drainRecoverySources closes the gap between a fast-path wire decision and
// publication of its received event. Only afterward may the process-level
// applied-event barrier make a cold RIB lookup causally current. It returns
// the sources whose publication failed: the delivery worker tore each one
// down (failReceivePublication), and the RIB still holds routes it will never
// see withdrawn, so every key of the round excludes it. A failed source is
// never a reason to retire the destination.
func drainRecoverySources(ctx context.Context, sources map[*Peer]recoverySourceReceipt) ([]netip.Addr, error) {
	var stopped []netip.Addr
	for peer, source := range sources {
		err := source.cut.wait(ctx)
		if errors.Is(err, errReceivePublicationFailed) {
			stopped = append(stopped, peer.Settings().Address)
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return stopped, nil
}

// recoverNLRIBatch owes a departed source's keys to every selected
// destination's recovery worker, then waits until each owed key is decided.
// RFC 4271 Section 6: "The local system recalculates its best routes for the
// destinations of the routes marked as invalid." The election is the RIB's,
// run by the destination's worker; this command only owes and waits, so its
// synchronous contract with the plugin holds. Canceling the wait stops only
// the waiter: the keys stay owed and the worker still decides them.
func (a *reactorAPIAdapter) recoverNLRIBatch(ctx context.Context, batch bgptypes.NLRIBatch, destinations []*Peer, _ plugin.Sender) error {
	if len(batch.NLRIs) == 0 {
		return nil
	}
	var owed []*adjOutOwed
	var ownerless error
	for _, destination := range destinations {
		if destination.State() != PeerStateEstablished {
			continue
		}
		nc := destination.negotiated.Load()
		if nc == nil {
			continue
		}
		if !nc.Has(batch.Family) {
			continue
		}
		session := destination.currentSession()
		if session == nil {
			continue
		}
		if !session.recoveryOwed() {
			// No process can run the election, so the destination cannot be
			// told which route replaces the departed one. AutomaticStop uses
			// the existing safe NOTIFICATION/close path; obsolete output MUST
			// disappear from the remote rather than be retained as stale.
			ownerless = errors.New("source-DOWN recovery requires the BGP RIB command owner")
			_ = session.teardownAutomatic(message.NotifyCeaseOutOfResources, "")
			continue
		}
		keys, err := oweDeparture(destination, session, &batch)
		if err != nil {
			return err
		}
		if len(keys) != 0 {
			owed = append(owed, keys...)
			session.wakeRecovery()
		}
	}
	for _, record := range owed {
		select {
		case <-record.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return ownerless
}

// oweDeparture owes, at one destination, each key of the batch whose adj-out
// entry the departed source owns through a session that is no longer current.
// Where another source owns the key, where the source's reconnected session
// owns it, or where no entry exists, nothing is owed: an old DOWN never
// overwrites a newer route. Each owe merges into an existing record and adds
// the departed source to its excluded set. At an ADD-PATH destination the key
// is owed per prefix (identifier zero), and the admission's writer tests each
// identifier's owner.
func oweDeparture(destination *Peer, session *Session, batch *bgptypes.NLRIBatch) ([]*adjOutOwed, error) {
	addPath := destination.addPathFor(batch.Family)
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	table := &destination.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()

	if table.session != session {
		return nil, nil
	}
	var owed []*adjOutOwed
	for _, route := range batch.NLRIs {
		raw := make([]byte, nlri.LenWithContext(route, false))
		nlri.WriteNLRI(route, raw, 0, false)
		key := adjOutKey{addPath: addPath}
		if err := fwdPathKeyFor(&key.path, batch.Family, 0, raw, false, scratch[:]); err != nil {
			return nil, err
		}
		// An ADD-PATH destination's identifiers are the RIB's to name; the
		// writer refuses each one a current session owns.
		if !addPath && !departedOwner(table.routes[key], batch.RecoverySource) {
			continue
		}
		raw, err := departureOweFraming(batch.Family, raw)
		if err != nil {
			return nil, err
		}
		record := table.owe(key, session, batch.Family, raw)
		record.exclude(batch.RecoverySource)
		owed = append(owed, record)
	}
	return owed, nil
}

// departureOweFraming rewrites, in place, one departure NLRI into the framing
// every owed record holds: the native announcement with a one-label stack
// (oweAnnouncementFraming). A departure names its routes as the source
// announced them, and a labeled route can carry a stack of several labels.
// The withdrawal written from the record reads its label field as the one
// Compatibility field, so a deeper stack left in the record put its lower
// labels where the receiver reads the RD and prefix: the write keyed a route
// the destination never held and withdrew nothing, and the destination kept
// the departed route. raw is the caller's own copy. A family whose
// announcement and withdrawal share their framing is returned unchanged.
// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD be
// set to 0x800000."
//
//	announced: [Length:1][Label 1:3]...[Label n, S set:3][RD:8, SAFI 128 only][Prefix]
//	owed:      [Length:1][Label, S set:3][RD:8, SAFI 128 only][Prefix]
func departureOweFraming(fam family.Family, raw []byte) ([]byte, error) {
	write := nlrisplit.GetWithdrawalWriter(fam)
	if write == nil {
		return raw, nil
	}
	n, err := write(raw, raw, false, false)
	if err != nil {
		return nil, err
	}
	raw = raw[:n]
	oweAnnouncementFraming(fam, raw)
	return raw, nil
}

// departedOwner reports whether entry is owned by source through a session
// that is no longer current, or through a binding a targeted down revoked on
// a session that stays up. The generation and the epoch are read under the
// table mutex every write takes, so the test is exact.
func departedOwner(entry *adjRIBOutRoute, source netip.Addr) bool {
	if entry == nil {
		return false
	}
	if entry.owner.source == nil {
		return false
	}
	if entry.owner.source.Settings().Address != source {
		return false
	}
	return !adjOutOwnerCurrent(entry.owner)
}

// adjOutOwnerCurrent reports whether owner still owns its path: its source's
// session is the one that wrote it (forwardSourceCurrent) and no targeted
// down revoked the source's binding since (Peer.forwardRevocation). An
// originated path has no source and is always current.
func adjOutOwnerCurrent(owner adjOutPath) bool {
	if !forwardSourceCurrent(owner.source, owner.generation) {
		return false
	}
	if owner.source == nil {
		return true
	}
	return owner.source.forwardRevocation.Load() == owner.revocation
}

// lookupRecovery asks the RIB for one outcome per requested key, in process or
// over IPC. A key with no outcome is a lookup failure, never "no survivor".
func (a *reactorAPIAdapter) lookupRecovery(ctx context.Context, request *ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
	var outcomes []ribevents.RecoveryOutcome
	var err error
	if owner := ribevents.RecoveryProvider(); owner != nil {
		// RFC 4271 Section 6: use the existing selection producer in-process.
		outcomes, err = owner.Lookup(*request)
	} else {
		if a.r.api == nil {
			return nil, errServerNotReady
		}
		args := make([]string, 0, 4+len(request.Keys))
		args = append(args, request.Destination.String(), request.Family.String(),
			strconv.FormatBool(request.AddPath), strconv.FormatBool(request.SentAddPath))
		for i := range request.Keys {
			args = append(args, ribevents.FormatRecoveryKey(&request.Keys[i]))
		}
		commandContext := &pluginserver.CommandContext{Server: a.r.api, RequestContext: ctx}
		var response *plugin.Response
		response, err = a.r.api.Dispatcher().ForwardToPlugin(commandContext, "request bgp rib recovery", args, "*")
		if err == nil {
			switch {
			case response == nil:
				err = fmt.Errorf("source-DOWN recovery returned no RIB answer")
			case response.Error != "":
				err = errors.New(response.Error)
			default:
				data, ok := response.Data.(plugin.RawJSON)
				if ok {
					err = json.Unmarshal([]byte(data), &outcomes)
				} else {
					err = fmt.Errorf("source-DOWN recovery returned an invalid RIB document")
				}
			}
		}
	}
	if err == nil {
		// A key with no outcome is a lookup failure, never "no survivor".
		err = ribevents.CheckRecoveryAnswer(request, outcomes)
	}
	if err != nil {
		return nil, err
	}
	return outcomes, nil
}

// admitRecovery stages one outcome per owed key on the destination's forward
// worker as one admission, and waits for it. Outcomes are in key order.
// RFC 4271 Section 6: "Before the invalid routes are deleted from the system,
// it advertises, to its peers, either withdraws for the routes marked as
// invalid, or the new best routes before the invalid routes are deleted from
// the system." Selection is the RIB's; export runs on the ordinary rail.
func (a *reactorAPIAdapter) admitRecovery(admission *recoveryAdmission, outcomes []ribevents.RecoveryOutcome, fam family.Family, sender plugin.Sender) error {
	// On every pre-dispatch exit this producer MUST release staged references.
	// After dispatch, releaseItem owns them and the single completion instead.
	dispatched := false
	defer func() {
		if !dispatched {
			a.r.fwdPool.releaseRecoveryItems(admission)
		}
	}()
	partial := false
	for k := range outcomes {
		err := a.queueRecoveryOutcome(&outcomes[k], k, fam, admission, sender)
		if errors.Is(err, errRelayBufferPool) {
			// Each staged key holds a read buffer until its write completes,
			// so a pool shared with every session's reads can run dry inside
			// one chunk. A buffer shortage is transient: the keys not staged
			// stay owed for a later round, and only the decided ones leave.
			a.releaseRecoveryTail(admission)
			admission.keys = admission.keys[:k]
			partial = true
			break
		}
		if err != nil {
			return err
		}
		admission.mark(k)
	}
	if len(admission.keys) == 0 {
		return errRecoveryBufferStalled
	}
	// One queue entry decides every key under one writeMu hold, so a key
	// with nothing to write is settled in the same section as its siblings.
	dispatched = true
	if !a.r.fwdPool.dispatchOverflow(fwdKey{peerAddr: admission.peer.Settings().PeerKey()},
		&fwdItem{peer: admission.peer, recovery: admission}) {
		return errForwardPoolStopped
	}
	select {
	case <-admission.done:
	case <-admission.ctx.Done():
		return admission.ctx.Err()
	}
	if admission.err != nil {
		return admission.err
	}
	if partial {
		return errRecoveryBufferPartial
	}
	return nil
}

// releaseRecoveryTail releases the items staged for a key that was not
// marked: a key whose staging failed part way, such as an ADD-PATH withdrawal
// whose second identifier found no buffer, is decided by a later round.
func (a *reactorAPIAdapter) releaseRecoveryTail(admission *recoveryAdmission) {
	marked := len(admission.itemKeys)
	for i := marked; i < len(admission.items); i++ {
		if done := admission.items[i].done; done != nil {
			done()
		}
		a.r.fwdPool.releaseItem(&admission.items[i])
	}
	admission.items = admission.items[:marked]
}

// queueRecoveryOutcome stages key k's decision. WITHDRAW withdraws the
// record's advertisement, or at an ADD-PATH destination each identifier the
// RIB names; SURVIVORS writes the first candidate, in election order, that
// the destination's export policy permits, and the withdrawal when it permits
// none. A survivor whose session ended or restarted since the round's receipt
// is written nowhere: the key is re-owed with that source excluded, so the
// next round elects over the survivors without it, and the admission's other
// keys are still decided.
// RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible
// routes for which there is no replacement route SHALL be advertised to its
// peers by means of an UPDATE message."
func (a *reactorAPIAdapter) queueRecoveryOutcome(outcome *ribevents.RecoveryOutcome, k int, fam family.Family, admission *recoveryAdmission, sender plugin.Sender) error {
	key := &admission.keys[k]
	admission.nlri = key.record.nlri // Immutable once the record exists.
	if outcome.Withdraw {
		sent := [][]byte{admission.nlri}
		if key.key.addPath {
			sent = outcome.Sent
		}
		for _, raw := range sent {
			withdraw := rpc.StoredRoute{Family: fam.String(), NLRIHex: hex.EncodeToString(raw), Withdraw: true}
			if err := a.queueRecoveryWithdrawal(admission.peer, &withdraw, fam, admission); err != nil {
				return err
			}
		}
		return nil
	}
	// Bounded by the RIB's candidates for one key.
	for i := range outcome.Survivors {
		survivor := &outcome.Survivors[i]
		stored := rpc.StoredRoute{SourcePeer: survivor.Source.String(), Family: fam.String(),
			NLRIHex: hex.EncodeToString(survivor.NLRI), AttrHex: hex.EncodeToString(survivor.Attributes),
			NextHopHex: hex.EncodeToString(survivor.NextHop), MsgID: survivor.MessageID,
			NLRIFraming: rpc.NLRIFramingSourceWire}
		first := len(admission.items)
		err := a.queueRecovery(admission.peer, &stored, admission, admission.sources, sender)
		if errors.Is(err, errAllDestinationsSuppressed) {
			// The forward rail refused the candidate outright (AC-25), as
			// reflection refuses a non-client route to a non-client. RFC 4456
			// Section 6: "A route from a Non-Client IBGP peer: Reflect to all
			// the Clients." The next candidate in election order is tried.
			continue
		}
		if errors.Is(err, errRelayNoSource) {
			key.outcome, key.gone = recoveryKeyReowed, survivor.Source
			return nil
		}
		if errors.Is(err, errForwardNoSource) {
			key.outcome, key.gone = recoveryKeyReowed, survivor.Source
			return nil
		}
		if err != nil {
			return err
		}
		if !recoveryStagedAnnouncement(admission.items[first:]) {
			// The destination may not receive this candidate (an export
			// policy, a well-known community, a next-hop refusal): the rail
			// staged only its synthesized withdrawal, which is no
			// replacement. RFC 7947 Section 2.1: "The route server SHOULD
			// forward UPDATE messages from its Loc-RIB or Loc-RIBs to its
			// clients as determined by local policy." The next candidate in
			// election order is tried (AC-5).
			a.releaseRecoveryTail(admission)
			continue
		}
		n := len(admission.items)
		key.candidate = adjOutPath{source: admission.items[n-1].receivedPeer,
			received: survivor.PathID, addPath: survivor.AddPath, message: survivor.MessageID}
		return nil
	}
	// RFC 4271 Section 9.1.3: "If a route in Loc-RIB is excluded from a
	// particular Adj-RIB-Out, the previously advertised route in that
	// Adj-RIB-Out MUST be withdrawn from service by means of an UPDATE
	// message (see 9.2)." Here no candidate may be sent.
	withdraw := rpc.StoredRoute{Family: fam.String(), NLRIHex: hex.EncodeToString(admission.nlri), Withdraw: true}
	return a.queueRecoveryWithdrawal(admission.peer, &withdraw, fam, admission)
}

// recoveryStagedAnnouncement reports whether a candidate's staged sections
// carry its announcement. The forward rail stages a policy-denied candidate as
// a synthesized withdrawal, whose provenance says so, and a permitted one as
// the candidate's own announcement; both carry adjOutRecovery authority.
func recoveryStagedAnnouncement(items []fwdItem) bool {
	for i := range items {
		if items[i].provenance == nil {
			return true
		}
		if !items[i].provenance.synthesizedWithdrawal {
			return true
		}
	}
	return false
}

// queueRecovery admits a replacement on its surviving source's ordinary rail.
// A replacement the destination's export policy denies is staged as its
// synthesized withdrawal, or not at all (errAllDestinationsSuppressed); the
// caller then tries the next candidate.
// RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible
// routes for which there is no replacement route SHALL be advertised to its
// peers by means of an UPDATE message."
// Queue order retains earlier forwards before this replacement or withdrawal.
func (a *reactorAPIAdapter) queueRecovery(destination *Peer, route *rpc.StoredRoute, admission *recoveryAdmission, sources map[*Peer]recoverySourceReceipt, sender plugin.Sender) error {
	sourceAddr, err := netip.ParseAddr(route.SourcePeer)
	if err != nil {
		return err
	}
	source := a.resolveRelaySource(sourceAddr)
	if !source.ok {
		return errRelayNoSource
	}
	receipt, present := sources[source.info.peer]
	if !present {
		return errRelayNoSource
	}
	if receipt.generation != source.generation {
		return errRelayNoSource
	}
	switch receipt.cut.state(source.info.peer) {
	case receiveCutCurrent:
	case receiveCutEnded:
		return errRelayNoSource
	case receiveCutUnspecified:
		panic("BUG: recovery source receipt returned no cut state")
	}
	var spans []relayAttrSpan
	update, id, _, err := a.buildRelayUpdate([]rpc.StoredRoute{*route}, source, &spans)
	if err != nil {
		return err
	}
	defer a.r.recentUpdates.Release(id)
	source.info.sender, source.info.recovery = sender, admission
	return a.forwardUpdateCore(update, id, []*Peer{destination}, source.info)
}

// queueRecoveryWithdrawal uses destination framing, including the advertised
// ADD-PATH identifier. The admission's per-key fence captures its obsolete owner.
// RFC 7911 Section 3: "The combination of the address prefix and the Path
// Identifier can be used to identify a route advertised by a BGP speaker.".
func (a *reactorAPIAdapter) queueRecoveryWithdrawal(destination *Peer, route *rpc.StoredRoute, fam family.Family, admission *recoveryAdmission) error {
	source := relaySource{addr: destination.Settings().Address, ctxID: destination.sendContextID()}
	update, id, _, err := a.buildRelayWithdrawal(route, source, fam, 0)
	if err != nil {
		return err
	}
	// The staged item owns the cache reference until the operation is released.
	admission.items = append(admission.items, fwdItem{peer: destination,
		session: admission.session, authority: adjOutRecovery,
		rawBodies: [][]byte{update.WireUpdate.Payload()},
		done:      func() { a.r.recentUpdates.Release(id) }})
	return nil
}
