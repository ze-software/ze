// Design: docs/architecture/behavior/peer-lifecycle.md -- in-place import invalidation
// Overview: peer_settings_apply.go -- the journaled swap that triggers it
// Related: session_flow.go -- the admission receipt this file holds
// Related: receive_publication.go -- the receive frontier prepare settles

package reactor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourcePolicyReserve is the interval an admission pause keeps back from its
// HoldTimer budget for abort and resumption. A pause whose session cannot
// spare it is refused before any policy is published.
const sourcePolicyReserve = time.Second

// sourcePolicyTxBudget bounds a transaction that carries no deadline of its
// own. It matches the BGP plugin's declared apply budget (register.go).
const sourcePolicyTxBudget = 30 * time.Second

// sourcePolicyTransaction is what a config journal carries when it belongs to
// one operation of a config transaction. The BGP plugin's operation journal
// implements it. A journal without it (the section path, the internal
// Reload/ApplyConfigDiff journal) has no transaction to pause input across,
// so a change that needs one is refused there.
type sourcePolicyTransaction interface {
	SourcePolicyTransaction() (txID string, deadline time.Time)
}

// sourcePolicyExpiredMax bounds the expiry reports kept for transactions whose
// rejection receipt has not arrived yet. A report past it is only logged.
const sourcePolicyExpiredMax = 64

// sourcePolicyCoordinator records the in-place import invalidations a config
// transaction holds, at most one per peer, and the expiry reports owed to a
// late rejection receipt. Its mutex is a leaf: it is never held across an
// owner call, a wait, or another lock. Safe for concurrent use.
type sourcePolicyCoordinator struct {
	mu      sync.Mutex
	active  map[*Peer]*sourceInvalidation
	expired map[string][]error
}

// sourceInvalidation is one peer's reversible import invalidation. It owns the
// session's admission receipt from prepare until finalize, the transaction's
// rejection receipt, or the hold's expiry releases it, exactly once. Its
// methods run on the transaction's goroutine, never concurrently with each
// other, except release and expire, which the hold timer may run.
type sourceInvalidation struct {
	r        *Reactor
	txID     string
	deadline time.Time
	peer     *Peer
	receipt  *AdmissionReceipt
	identity rpc.SourcePolicyIdentity
	owners   []*pluginserver.SourcePolicyOwner
	// window is the epoch apply issued, the stamp of every receipt a
	// destination released for this invalidation (adjOutWrite.keepReleased).
	// It is zero until apply advances the epoch.
	window uint64
	// aborted is set once the previous policy was restored. Input stays
	// paused after it until the rejection receipt or the hold's expiry.
	aborted bool
	// relaxationUndo marks the invalidation an import relaxation's undo ran
	// (undoImportRelaxation): its apply published the restored rejecting
	// policy, so it IS the rollback, and the receipt finalizes it.
	relaxationUndo bool
	released       atomic.Bool
	mu             sync.Mutex  // guards hold
	hold           clock.Timer // the expiry of a pause only the receipt ends (holdUntilCompletion)
}

// importDecisionFalls reports the D-2 transition: an effective import
// decision that accepts before the swap and does not after, whatever the
// session's state (swapImportFalling decides what the state needs). The
// decision is judged on what the swap actually publishes (copy applied to the
// running settings), never on the raw final value. A next whose import policy
// was never resolved (unresolved, peerSettingsSwap.policyUnresolved) cannot
// be judged, so a swap that would deliver its policy over an accepting one is
// refused instead of guessed.
func (p *Peer) importDecisionFalls(next *PeerSettings, copy settingsCopier, unresolved bool) (bool, error) {
	current := p.settingsSnapshot()
	if current == nil {
		return false, nil
	}
	candidate := *current
	copy(&candidate, next)
	external := current.IsEBGP()
	// RFC 8212 Section 3: a decision falls only when the current import
	// policy accepts at its end and the candidate does not.
	if !current.ImportPolicy.AcceptsAtEnd(external) {
		return false, nil
	}
	if candidate.ImportPolicy.AcceptsAtEnd(external) {
		return false, nil
	}
	if unresolved {
		return false, fmt.Errorf("peer %s: the change was built from the peer's config leaves alone, which carry no "+
			"filter chain, so its effective import policy is unknown and nothing was changed: removing an accepting "+
			"import policy is never guessed (RFC 8212 Section 3). The BGP reactor has no config file to resolve the "+
			"candidate from (config path or reload function unset); start ze with a config file, or report the "+
			"commit's diff if it has one", p.addrString)
	}
	return true, nil
}

// importSwapGateHook, nil outside tests, runs inside swapImportFalling's
// boundary once it found the peer not Established, before the publish.
var importSwapGateHook func(*Peer)

// enterEstablished publishes Established, then waits out an import swap
// holding the boundary: one that found the peer not Established publishes its
// rejecting policy before this returns, so no UPDATE of the new session is
// admitted under the accepting one it removed.
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (p *Peer) enterEstablished() {
	p.setState(PeerStateEstablished)
	p.establishGate.Lock()
	p.establishGate.Unlock() //nolint:gocritic,staticcheck // badLock, SA2001: the empty critical section is the barrier
}

// swapImportFalling delivers a swap that removes the peer's last accepting
// import policy. Establishment joins the boundary (enterEstablished): under
// establishGate, a peer that is not Established admits nothing, so the policy
// is published there and establishment waits for it, and the epoch stays.
// Such a peer can still have paths a previous session admitted under the old
// policy: Graceful Restart and LLGR keep them in bgp-rib, stale and eligible,
// advertised downstream. Those are purged once the policy is published
// (purgeRetained). An Established peer takes the in-place invalidation, whose
// pause covers every admission from here on.
func (a *reactorAPIAdapter) swapImportFalling(peer *Peer, next *PeerSettings, apply settingsCopier, previous *PeerSettings, j configJournal) error {
	peer.establishGate.Lock()
	if peer.State() != PeerStateEstablished {
		if importSwapGateHook != nil {
			importSwapGateHook(peer)
		}
		err := j.Record(
			func() error {
				if err := peer.applyHotSwappableSettings(next, apply); err != nil {
					return errors.Join(err, peer.restoreHotSwappableSettings(previous, apply))
				}
				return nil
			},
			func() error { return peer.restoreHotSwappableSettings(previous, apply) },
		)
		peer.establishGate.Unlock()
		if err != nil {
			return err
		}
		return a.r.purgeRetained(peer)
	}
	peer.establishGate.Unlock()
	return a.swapImportInvalidating(peer, next, apply, previous, j)
}

// retainedPurgeCommand is the RIB command that deletes a peer's stale paths,
// the ones Graceful Restart and LLGR retained from its previous session.
const retainedPurgeCommand = "request bgp rib purge-stale"

// purgeRetained deletes the paths a peer that is not Established still has
// retained from its previous session, after a swap published an import policy
// that rejects them. A retained path is stale (RFC 4724 Section 4.2), and
// purge-stale withdraws every copy of it the RIB sent and re-elects, as the
// restart timer's expiry would. The previous session's paths cannot be
// re-admitted under the new policy: only the peer's next session can send
// them again, and it admits under the published policy. A rollback that
// restores the accepting policy therefore does not bring them back: the
// peer's next session re-sends them, exactly as after a restart timer expiry.
// The purge is not deferred to the commit's acceptance, which would leave them
// eligible and advertised under the rejecting policy for the rest of the
// transaction. A path the next session already admitted is fresh, so a session
// that established after the publish keeps its routes.
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (r *Reactor) purgeRetained(peer *Peer) error {
	// refuse-by-default: guards purgeRetained -- with no plugin server no process retains a path for the peer
	if r.api == nil {
		return nil
	}
	command := r.api.Dispatcher().Registry().Lookup(retainedPurgeCommand)
	// refuse-by-default: guards purgeRetained -- only bgp-rib retains a previous session's paths, and it is not loaded
	if command == nil {
		return nil
	}
	request := &pluginserver.CommandContext{Server: r.api, RequestContext: context.Background()}
	response, err := r.api.Dispatcher().ForwardToPlugin(request, retainedPurgeCommand, []string{peer.addrString}, "*")
	if err != nil {
		return fmt.Errorf("peer %s: the import policy now rejects its routes, but the paths its previous session "+
			"left retained (Graceful Restart) could not be purged and stay advertised until the restart timer "+
			"expires: %w; run `request bgp rib purge-stale %s`", peer.addrString, err, peer.addrString)
	}
	if response == nil {
		return fmt.Errorf("peer %s: purge of its retained paths returned no answer; run `request bgp rib purge-stale %s`",
			peer.addrString, peer.addrString)
	}
	if response.Error != "" {
		return fmt.Errorf("peer %s: purge of its retained paths failed: %s; run `request bgp rib purge-stale %s`",
			peer.addrString, response.Error, peer.addrString)
	}
	return nil
}

// prepareImportInvalidation captures the owners, pauses the session's input,
// settles its receive frontier and prepares every owner, all before any policy
// is published. On failure nothing is published and the pause is released.
func (r *Reactor) prepareImportInvalidation(txID string, deadline time.Time, peer *Peer) (*sourceInvalidation, error) {
	label := peer.addrString
	if err := r.sourcePolicy.claim(peer, txID); err != nil {
		return nil, err
	}
	inv := &sourceInvalidation{r: r, txID: txID, deadline: deadline, peer: peer}
	if err := inv.capture(); err != nil {
		r.sourcePolicy.drop(peer, inv)
		return nil, err
	}
	session := peer.currentSession()
	if session == nil {
		r.sourcePolicy.drop(peer, inv)
		return nil, fmt.Errorf("peer %s: session ended before its input could be paused", label)
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	receipt, err := session.PauseAdmission(ctx, sourcePolicyReserve)
	if err != nil {
		r.sourcePolicy.drop(peer, inv)
		return nil, fmt.Errorf("peer %s: cannot pause input to remove its accepting import policy: %w", label, err)
	}
	inv.receipt = receipt
	work, cancelWork := inv.workContext()
	defer cancelWork()
	cut := peer.receiveCut()
	if err := cut.wait(work); err != nil {
		inv.release()
		return nil, fmt.Errorf("peer %s: input admitted before the pause did not settle: %w", label, err)
	}
	if prepareSettledHook != nil {
		prepareSettledHook(peer)
	}
	// The identity below names the session that holds the pause: one that
	// replaced it while the frontier settled is not paused, and its
	// generation would name it in every owner's token.
	if peer.currentSession() != session {
		inv.release()
		return nil, fmt.Errorf("peer %s: session replaced while its input was paused; nothing was changed, retry the commit", label)
	}
	peer.mu.RLock()
	template := peer.dynGeneration
	peer.mu.RUnlock()
	inv.identity = rpc.SourcePolicyIdentity{
		Peer: label, PeerOwner: peer.sourceOwner,
		SessionGeneration: peer.forwardGeneration.Load(), PolicyEpoch: peer.sourcePolicyEpoch.Load(),
		ReceiveFrontier: cut.messageID, TemplateGeneration: template,
	}
	if inv.identity.PolicyEpoch == 0 {
		inv.release()
		return nil, fmt.Errorf("peer %s: session has no admitted import policy epoch to invalidate", label)
	}
	prepared := inv.owners
	inv.owners = nil
	for _, owner := range prepared {
		if err := owner.Call(work, inv.request(rpc.SourcePolicyPrepare, 0, receipt.Deadline())); err != nil {
			cause := fmt.Errorf("source-policy owner %s: prepare for peer %s failed: %w", owner.Name(), label, err)
			// The failing owner takes the target-zero abort too: a call that
			// timed out may still finish its prepare and pin, and that pin
			// would refuse every later transaction for the source. An owner
			// that never prepared answers the abort as done (sourcepolicy
			// Owner.Admit). Nothing was published, so input resumes at once.
			inv.owners = append(inv.owners, owner)
			result := errors.Join(cause, inv.abort(nil))
			inv.release()
			return nil, result
		}
		inv.owners = append(inv.owners, owner)
	}
	inv.r.sourcePolicy.record(peer, inv)
	return inv, nil
}

// prepareSettledHook, nil outside tests, runs once prepare's receive
// frontier settled, before the session is confirmed.
var prepareSettledHook func(*Peer)

// capture names the owners from the delivery graph, the processes granted the
// source's received UPDATEs, and preflights their declared lifecycle support.
// An owner that retains source state without a handler refuses the whole change.
func (inv *sourceInvalidation) capture() error {
	if inv.r.api == nil {
		return fmt.Errorf("peer %s: plugin server unavailable, source-policy owners cannot be captured", inv.peer.addrString)
	}
	granted := inv.r.api.DeliveryGraph().Receivers(events.LookupNamespaceID(bgpevents.Namespace),
		events.LookupEventTypeID(bgpevents.EventUpdate), events.DirReceived, inv.peer.addrString)
	owners, err := inv.r.api.CaptureSourcePolicyOwners(slices.Clone(granted))
	if err != nil {
		return fmt.Errorf("peer %s: %w", inv.peer.addrString, err)
	}
	if err := inv.requireSentOwner(owners); err != nil {
		return err
	}
	inv.owners = owners
	return nil
}

// apply publishes the candidate settings, fences every earlier admission with a
// fresh epoch, joins the destination writers and has every owner exclude the
// source's paths. Any failure aborts before returning, because a failed journal
// apply is never undone by the journal, and keeps input paused for the rest of
// the transaction's rollback. A relaxation's undo (relaxationUndo) aborts only
// when its publish failed: once the rejecting policy is published it keeps the
// denial instead (fail).
func (inv *sourceInvalidation) apply(publish, restore func() error) error {
	if err := inv.receipt.Check(); err != nil {
		return errors.Join(fmt.Errorf("peer %s: no budget left to remove its accepting import policy: %w", inv.peer.addrString, err), inv.undo(nil))
	}
	if err := publish(); err != nil {
		return errors.Join(err, inv.undo(restore))
	}
	// RFC 8212 Section 3
	next := inv.peer.openSourcePolicyWindow()
	inv.window = next
	work, cancel := inv.workContext()
	defer cancel()
	if err := inv.r.joinSourceWriters(work, inv.peer); err != nil {
		return errors.Join(fmt.Errorf("peer %s: destination writers did not settle: %w", inv.peer.addrString, err), inv.fail(restore))
	}
	for _, owner := range inv.owners {
		if err := owner.Call(work, inv.request(rpc.SourcePolicyApply, next, inv.receipt.Deadline())); err != nil {
			cause := fmt.Errorf("source-policy owner %s: invalidation of peer %s failed: %w", owner.Name(), inv.peer.addrString, err)
			return errors.Join(cause, inv.fail(restore))
		}
		if err := inv.drainValidation(work); err != nil {
			return errors.Join(fmt.Errorf("peer %s: after owner %s: %w", inv.peer.addrString, owner.Name(), err), inv.fail(restore))
		}
	}
	if err := inv.settleDownstream(work); err != nil {
		return errors.Join(fmt.Errorf("peer %s: %w", inv.peer.addrString, err), inv.fail(restore))
	}
	return nil
}

// fail is apply's failure path once publish ran. An ordinary invalidation
// aborts (undo). A relaxation's undo (relaxationUndo) published the rejecting
// policy it restores, and its abort would make the paths the candidate
// admitted eligible again under a policy that rejects them, so it keeps the
// denial like rejectNotUndone: input stays paused, the transaction's receipt
// finalizes it (CompleteSourcePolicyTx), and the hold bounds the pause if no
// receipt arrives. Its caller reports the compensation incomplete.
func (inv *sourceInvalidation) fail(restore func() error) error {
	if inv.relaxationUndo {
		inv.holdUntilCompletion()
		return nil
	}
	return inv.undo(restore)
}

// undo is the journal's inverse and apply's own failure path: it aborts and
// keeps input paused, because the transaction's rollback is not over until
// every other participant, the section-owned filter definitions included,
// has restored its config. The rejection receipt (CompleteSourcePolicyTx with
// accepted false), sent after that whole rollback, resumes input; the hold's
// expiry at the receipt's completion deadline resumes it if none arrives.
func (inv *sourceInvalidation) undo(restore func() error) error {
	err := inv.abort(restore)
	inv.holdUntilCompletion()
	return err
}

// abort restores the previous settings. When apply advanced the source's
// epoch (window), it fences the tentative admissions with a newer epoch and
// has every prepared owner restore eligibility under it; before that, it
// leaves the epoch and the window alone and has every prepared owner release
// its pins with a zero target. It does not
// resume input: its caller releases the pause, or holds it for the rest of the
// rollback. restore is nil when nothing was published. It is idempotent. A
// failure is reported as incomplete compensation, never as a rollback.
func (inv *sourceInvalidation) abort(restore func() error) error {
	if inv.aborted {
		return nil
	}
	if inv.released.Load() {
		return nil
	}
	inv.aborted = true
	label := inv.peer.addrString
	deadline := inv.receipt.CompletionDeadline()
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	var result error
	if restore != nil {
		if err := restore(); err != nil {
			result = fmt.Errorf("restore previous settings: %w", err)
		}
	}
	if inv.window == 0 {
		// RFC 8212 Section 3: the epoch never advanced, so nothing was fenced
		// or excluded. The owners only release their pins (target zero), and
		// the epoch stays: a write an owner stamped with it is still current.
		for _, owner := range inv.owners {
			if err := owner.Call(ctx, inv.request(rpc.SourcePolicyAbort, 0, deadline)); err != nil {
				result = errors.Join(result, fmt.Errorf("source-policy owner %s lost: %w", owner.Name(), err))
			}
		}
		if result != nil {
			return fmt.Errorf("peer %s: import invalidation compensation incomplete: %w", label, result)
		}
		return nil
	}
	next := inv.peer.abortSourcePolicyWindow(inv.window)
	if err := inv.r.joinSourceWriters(ctx, inv.peer); err != nil {
		result = errors.Join(result, fmt.Errorf("destination writers did not settle: %w", err))
	}
	for _, owner := range inv.owners {
		if err := owner.Call(ctx, inv.request(rpc.SourcePolicyAbort, next, deadline)); err != nil {
			result = errors.Join(result, fmt.Errorf("source-policy owner %s lost: %w", owner.Name(), err))
		}
		if err := inv.drainValidation(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("after owner %s: %w", owner.Name(), err))
		}
	}
	if err := inv.settleDownstream(ctx); err != nil {
		result = errors.Join(result, err)
	}
	if result != nil {
		return fmt.Errorf("peer %s: import invalidation compensation incomplete: %w", label, result)
	}
	return nil
}

// finalize makes an accepted invalidation permanent on every owner and resumes
// input through the new policy. A failure keeps the epoch fence and still
// releases the pause, so the session never stays paused.
func (inv *sourceInvalidation) finalize(ctx context.Context) error {
	if inv.released.Load() {
		return nil
	}
	defer inv.release()
	return inv.finalizeOwners(ctx)
}

// finalizeOwners asks every owner to make the invalidation permanent. It
// leaves the pause to its caller.
func (inv *sourceInvalidation) finalizeOwners(ctx context.Context) error {
	deadline := inv.finalizeDeadline(ctx)
	work, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	var result error
	for _, owner := range inv.owners {
		if err := owner.Call(work, inv.request(rpc.SourcePolicyFinalize, 0, deadline)); err != nil {
			result = errors.Join(result, fmt.Errorf("source-policy owner %s: %w", owner.Name(), err))
		}
	}
	if result != nil {
		return fmt.Errorf("peer %s: import invalidation finalization failed: %w", inv.peer.addrString, result)
	}
	return nil
}

// release resumes admission once and forgets the invalidation. It reports
// whether this call was the one that released. Safe for concurrent use.
func (inv *sourceInvalidation) release() bool {
	if !inv.released.CompareAndSwap(false, true) {
		return false
	}
	inv.mu.Lock()
	if inv.hold != nil {
		inv.hold.Stop()
	}
	inv.mu.Unlock()
	if inv.receipt != nil {
		inv.receipt.Release()
	}
	// The window closes before its receipts are dropped, so no write keeps
	// a new one behind the sweep, and before the claim is dropped, so no
	// later transaction's receipt is swept: this transaction's abort restore,
	// if any, has landed (abort settles the writers), and nothing else may
	// re-install them.
	inv.peer.closeSourcePolicyWindow(inv.window)
	inv.r.dropReleasedSource(inv.peer)
	inv.r.sourcePolicy.drop(inv.peer, inv)
	return true
}

// sourcePolicyWindow is one source's open in-place import invalidation, as
// the final writer reads it. Immutable once published (Peer.sourcePolicyWindow).
type sourcePolicyWindow struct {
	// applied is the epoch the invalidation's apply issued, the stamp of the
	// receipts destinations released for it.
	applied uint64
	// aborted is set once the abort issued restore, the epoch under which
	// its owners re-send the released receipts.
	aborted bool
	restore uint64
}

// openSourcePolicyWindow advances the source's epoch for an invalidation's
// apply and opens its window. Only the coordinator calls it, with input
// paused.
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (p *Peer) openSourcePolicyWindow() uint64 {
	next := p.advanceSourcePolicyEpoch()
	p.sourcePolicyWindow.Store(&sourcePolicyWindow{applied: next})
	return next
}

// abortSourcePolicyWindow advances the source's epoch for the abort of an
// invalidation whose apply advanced it (an abort before that sends its owners
// target zero and calls nothing here). When the invalidation applied (window is its apply epoch and still
// open), the window records that this abort issued the new epoch, so its
// owners' re-sends of the receipts released under window are admitted
// (sentOwnerCurrent).
//
// RFC 8212 Section 3: "Routes contained in an Adj-RIB-In associated with an
// EBGP peer SHALL NOT be considered eligible in the Decision Process if no
// explicit Import Policy has been applied."
func (p *Peer) abortSourcePolicyWindow(window uint64) uint64 {
	next := p.advanceSourcePolicyEpoch()
	open := p.sourcePolicyWindow.Load()
	if open == nil {
		return next
	}
	if open.applied != window {
		return next
	}
	p.sourcePolicyWindow.Store(&sourcePolicyWindow{applied: window, aborted: true, restore: next})
	return next
}

// closeSourcePolicyWindow closes the window apply opened under window, and
// no later one.
func (p *Peer) closeSourcePolicyWindow(window uint64) {
	open := p.sourcePolicyWindow.Load()
	if open == nil {
		return
	}
	if open.applied != window {
		return
	}
	p.sourcePolicyWindow.CompareAndSwap(open, nil)
}

// dropReleasedSource drops every receipt source released at every other
// peer's Adj-RIB-Out, under each table's mu, the lock its final writer holds
// for the whole write. Its callers are the moments no abort restore may name
// them any more: the invalidation's release (finalize, the rejection receipt,
// the hold's expiry), the source session's departure, a targeted down's
// revocation, and the Peer's stop. The caller MUST NOT hold r.mu or a table's
// mu. A nil reactor holds no destination.
func (r *Reactor) dropReleasedSource(source *Peer) {
	if r == nil {
		return
	}
	r.mu.RLock()
	destinations := make([]*Peer, 0, len(r.peers))
	for _, peer := range r.peers {
		if peer != source {
			destinations = append(destinations, peer)
		}
	}
	r.mu.RUnlock()
	for _, destination := range destinations {
		destination.adjOut.dropReleasedSource(source)
	}
}

// holdUntilCompletion arms the bound on the pause of an aborted invalidation,
// or of a relaxation's undo, whose receipt alone ends it: at the receipt's
// completion deadline expire resumes input, so a receipt that never arrives
// cannot leave the session paused.
func (inv *sourceInvalidation) holdUntilCompletion() {
	if inv.receipt == nil {
		return
	}
	if inv.released.Load() {
		return
	}
	clk := inv.receipt.session.clock
	wait := max(inv.receipt.CompletionDeadline().Sub(clk.Now()), time.Nanosecond)
	inv.mu.Lock()
	defer inv.mu.Unlock()
	if inv.hold == nil {
		inv.hold = clk.AfterFunc(wait, inv.expire)
	}
}

// expire runs on the hold timer. It resumes input under the restored policy
// and, when it was the call that released, records the expiry for the late
// receipt to report and logs it. A relaxation's undo published the rejecting
// policy rather than aborting, so its owners are finalized here, as its
// receipt would have: the denial stays permanent and nothing is left prepared.
func (inv *sourceInvalidation) expire() {
	if inv.released.Load() {
		return
	}
	err := fmt.Errorf("peer %s: transaction %s sent no rejection receipt by the completion deadline %s; "+
		"input resumed under the restored import policy, possibly before every other participant restored its config",
		inv.peer.addrString, inv.txID, inv.receipt.CompletionDeadline().Format(time.RFC3339))
	// Only the call that released records the expiry: a receipt that won the
	// race already reported the outcome, so a record here would outlive it.
	if !inv.release() {
		return
	}
	if inv.relaxationUndo {
		err = errors.Join(err, inv.finalizeOwners(context.Background()))
	}
	inv.r.sourcePolicy.recordExpiry(inv.txID, err)
	reactorLogger().Error("import invalidation hold expired", "peer", inv.peer.addrString, "transaction", inv.txID, "error", err)
}

// finalizeDeadline bounds finalize, which runs once the whole reload was
// accepted: after the operation's apply deadline, which the receipt's
// CompletionDeadline is capped by, so that deadline is not finalize's. An
// accepted reload that commits, writes its config, migrates listeners and
// publishes after it would otherwise send owners a deadline they refuse as
// past (the server's owner call runs under it), and report "candidate is
// active but config finalization failed". Its bound is ctx's deadline, the
// config-complete receipt's, and the captured session's live HoldTimer expiry
// (input is paused, so no KEEPALIVE moves it), whichever comes first. A
// session with no armed HoldTimer (HoldTime 0) is bounded by ctx alone, and by
// the receipt's own bound when ctx has none.
func (inv *sourceInvalidation) finalizeDeadline(ctx context.Context) time.Time {
	var deadline time.Time
	if budget := inv.receipt.session.HoldBudget(); budget.State == fsm.HoldBudgetArmed {
		deadline = budget.Expires
	}
	if bound, ok := ctx.Deadline(); ok && (deadline.IsZero() || bound.Before(deadline)) {
		deadline = bound
	}
	if deadline.IsZero() {
		deadline = inv.receipt.CompletionDeadline()
	}
	return deadline
}

// workContext bounds a work phase by the earlier of the transaction deadline
// and the receipt's work deadline.
func (inv *sourceInvalidation) workContext() (context.Context, context.CancelFunc) {
	deadline := inv.receipt.Deadline()
	if inv.deadline.Before(deadline) {
		deadline = inv.deadline
	}
	return context.WithDeadline(context.Background(), deadline)
}

func (inv *sourceInvalidation) request(op rpc.SourcePolicyOperation, next uint64, deadline time.Time) rpc.SourcePolicyInput {
	if op == rpc.SourcePolicyApply && inv.deadline.Before(deadline) {
		deadline = inv.deadline
	}
	return rpc.SourcePolicyInput{
		TransactionID: inv.txID, Operation: op, Source: inv.identity,
		NextPolicyEpoch: next, DeadlineMS: deadline.UnixMilli(),
	}
}

// requireSentOwner refuses, before anything is published, an invalidation
// whose source's routes reach a forwarder (a captured owner declaring
// registry.FencesLiveForwards) while the process holding the sent inventory is
// not captured. That process answers the RIB recovery command; it is the only
// downstream writer that withdraws the copies forwarders sent, so without it
// those copies would stay at their destinations after the policy denied them.
// RFC 8212 Section 3.
func (inv *sourceInvalidation) requireSentOwner(owners []*pluginserver.SourcePolicyOwner) error {
	forwarder := ""
	for _, owner := range owners {
		if registry.FencesLiveForwards(owner.Name()) {
			forwarder = owner.Name()
			break
		}
	}
	// refuse-by-default: guards prepareImportInvalidation -- no captured owner forwards the source's routes, so no destination holds a copy to withdraw
	if forwarder == "" {
		return nil
	}
	holder := ""
	if command := inv.r.api.Dispatcher().Registry().Lookup(recoveryCommandName); command != nil && command.Process != nil {
		holder = command.Process.Name()
	}
	for _, owner := range owners {
		if holder != "" && owner.Name() == holder {
			return nil
		}
	}
	if holder == "" {
		return fmt.Errorf("peer %s: the import policy change is refused before anything is published: plugin %s "+
			"forwards routes received from this peer, but no RIB process is loaded to withdraw the copies it sent "+
			"when the policy stops accepting them, so they would stay at their destinations; load bgp-rib and "+
			"attach it to this peer with receive [ update-received ], or disable the peer, commit, change the "+
			"policy, re-enable the peer and commit again", inv.peer.addrString, forwarder)
	}
	return fmt.Errorf("peer %s: the import policy change is refused before anything is published: plugin %s "+
		"forwards routes received from this peer, but %s, the RIB that withdraws forwarded copies, does not "+
		"receive this peer's UPDATEs, so the copies would stay at their destinations; attach %s to this peer "+
		"with receive [ update-received ], or disable the peer, commit, change the policy, re-enable the peer "+
		"and commit again", inv.peer.addrString, forwarder, holder, holder)
}

// drainValidation waits until every plugin-process subscriber of the RIB
// validation-change event has applied each one queued before the call. An
// owner (bgp-adj-rib-in with its validation gate on) publishes it on apply and
// abort; bgp-rib answers by re-electing and publishing best changes, which a
// forwarder relays. Draining after each owner's call, before the next owner's,
// puts those relays ahead of a dependent owner's own settle barrier, which the
// dependency order of the captured owners places later.
func (inv *sourceInvalidation) drainValidation(ctx context.Context) error {
	if err := inv.r.api.DrainEventSubscribers(ctx, ribevents.ValidationChange.Namespace(),
		ribevents.ValidationChange.EventType()); err != nil {
		return fmt.Errorf("validation-change subscribers did not settle: %w", err)
	}
	return nil
}

// settleDownstream follows the owners' apply or abort. An owner recomputes
// best paths before it acknowledges and publishes each change as a RIB
// best-change and as its redistribution route change (bgp-rib
// publishBestChanges), and an eligibility change as a validation change
// (bgp-adj-rib-in), which bgp-rib answers with best changes. A subscriber
// running as a plugin process receives those queued and may write to
// destinations in answer, so its application is drained first, validation
// changes before the best changes they cause; the writer join then settles
// every write it issued. Engine-side subscribers write inside the emit, before
// the owner answers. It does not wait for a recovery round a destination owes:
// a round writes later, and the epoch fences (sourcePolicyCurrent) refuse any
// write it admits under a superseded import policy.
func (inv *sourceInvalidation) settleDownstream(ctx context.Context) error {
	published := [...]struct{ namespace, eventType string }{
		{ribevents.ValidationChange.Namespace(), ribevents.ValidationChange.EventType()},
		{ribevents.BestChange.Namespace(), ribevents.BestChange.EventType()},
		{bgpredist.RouteChange.Namespace(), bgpredist.RouteChange.EventType()},
	}
	for _, event := range published {
		if err := inv.r.api.DrainEventSubscribers(ctx, event.namespace, event.eventType); err != nil {
			return fmt.Errorf("%s subscribers did not settle: %w", event.eventType, err)
		}
	}
	if err := inv.r.joinSourceWriters(ctx, inv.peer); err != nil {
		return fmt.Errorf("destination writers did not settle: %w", err)
	}
	return nil
}

// joinSourceWriters settles every established destination's writer, the pool
// barrier then writeMu, as reconcileExportFinal does, so each write after it
// compares the source's current epoch. No reactor or peer lock is held.
func (r *Reactor) joinSourceWriters(ctx context.Context, source *Peer) error {
	r.mu.RLock()
	destinations := make([]*Peer, 0, len(r.peers))
	for _, peer := range r.peers {
		if peer != source {
			destinations = append(destinations, peer)
		}
	}
	r.mu.RUnlock()
	for _, destination := range destinations {
		if destination.State() != PeerStateEstablished {
			continue
		}
		session := destination.currentSession()
		if session == nil {
			continue
		}
		if r.fwdPool != nil {
			if err := r.fwdPool.barrierPeer(ctx, destination.Settings().PeerKey()); err != nil {
				return fmt.Errorf("destination %s: %w", destination.addrString, err)
			}
		}
		session.writeMu.Lock()
		session.writeMu.Unlock() //nolint:gocritic,staticcheck // badLock, SA2001: the empty critical section is the barrier
	}
	return nil
}

// claim refuses a second transaction while an earlier one still holds peer.
func (c *sourcePolicyCoordinator) claim(peer *Peer, txID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if held := c.active[peer]; held != nil {
		return fmt.Errorf("peer %s: transaction %s has not finalized its import invalidation; retry after it completes", peer.addrString, held.txID)
	}
	if c.active == nil {
		c.active = make(map[*Peer]*sourceInvalidation)
	}
	c.active[peer] = &sourceInvalidation{txID: txID}
	return nil
}

// drop forgets peer's invalidation, and its claim placeholder.
func (c *sourcePolicyCoordinator) drop(peer *Peer, inv *sourceInvalidation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if held := c.active[peer]; held != nil && held.txID == inv.txID {
		delete(c.active, peer)
	}
}

func (c *sourcePolicyCoordinator) record(peer *Peer, inv *sourceInvalidation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active[peer] = inv
}

// transaction lists txID's recorded invalidations.
func (c *sourcePolicyCoordinator) transaction(txID string) []*sourceInvalidation {
	c.mu.Lock()
	defer c.mu.Unlock()
	var held []*sourceInvalidation
	for _, inv := range c.active {
		if inv.txID == txID && inv.receipt != nil {
			held = append(held, inv)
		}
	}
	return held
}

// CheckSourcePolicyTx refuses an operation commit whose import invalidations
// exhausted their admission budget; the transaction then rolls back.
func (r *Reactor) CheckSourcePolicyTx(txID string) error {
	var result error
	for _, inv := range r.sourcePolicy.transaction(txID) {
		if err := inv.receipt.Check(); err != nil {
			result = errors.Join(result, fmt.Errorf("peer %s: import invalidation budget exhausted before commit: %w", inv.peer.addrString, err))
		}
	}
	return result
}

// CompleteSourcePolicyTx is txID's receipt. On acceptance it finalizes the
// transaction's invalidations. On rejection it resumes the input of every
// invalidation the rollback undid: the sender delivers it after the whole
// rollback, section-owned config included, so the first resumed UPDATE meets
// the restored filters. An invalidation the journal did not undo is aborted
// without its settings restore and reported as incomplete, never as rolled
// back, unless the published policy still rejects: then the denial is kept
// (rejectNotUndone). An expired hold is reported either way. The receipt also drops the
// transaction's shared candidate config (operationCandidateSlot).
func (r *Reactor) CompleteSourcePolicyTx(ctx context.Context, txID string, accepted bool) error {
	// The receipt ends the transaction, so its shared candidate config goes.
	r.operationCandidate.drop(txID)
	var result error
	for _, inv := range r.sourcePolicy.transaction(txID) {
		if inv.aborted {
			inv.release()
			if accepted && inv.relaxationUndo {
				result = errors.Join(result, fmt.Errorf("peer %s: transaction %s accepted after its import policy "+
					"relaxation could not be undone; the accepting policy stays in effect", inv.peer.addrString, txID))
				continue
			}
			if accepted {
				result = errors.Join(result, fmt.Errorf("peer %s: transaction %s accepted after its import invalidation was undone; the previous policy stays in effect", inv.peer.addrString, txID))
			}
			continue
		}
		if inv.relaxationUndo {
			// RFC 8212 Section 3: the undo restored the rejecting policy, so
			// the paths the relaxation admitted stay out of eligibility.
			err := inv.finalize(ctx)
			if accepted {
				err = errors.Join(fmt.Errorf("peer %s: transaction %s accepted after its import policy relaxation "+
					"was undone; the previous rejecting policy stays in effect", inv.peer.addrString, txID), err)
			}
			result = errors.Join(result, err)
			continue
		}
		if accepted {
			result = errors.Join(result, inv.finalize(ctx))
			continue
		}
		result = errors.Join(result, inv.rejectNotUndone(ctx))
	}
	return errors.Join(append([]error{result}, r.sourcePolicy.takeExpired(txID)...)...)
}

// rejectNotUndone answers a rejection receipt for an invalidation the
// journal never undid. No settings restore ran, so the published import
// policy decides: while it still rejects at its end, restoring eligibility
// would admit the source's routes under it, so the denial is kept (finalize)
// and the receipt reports that the rejected transaction's policy stays in
// effect. Only an accepting published policy lets the abort restore them.
func (inv *sourceInvalidation) rejectNotUndone(ctx context.Context) error {
	label := inv.peer.addrString
	settings := inv.peer.settingsSnapshot()
	// RFC 8212 Section 3: a peer with no published settings has no accepting
	// policy either, so it keeps the denial too.
	rejects := settings == nil || !settings.ImportPolicy.AcceptsAtEnd(settings.IsEBGP())
	if rejects {
		cause := fmt.Errorf("peer %s: transaction %s was rejected but its rollback did not restore the peer's import "+
			"policy, and the published policy still rejects its routes; they stay withdrawn under it (RFC 8212 "+
			"Section 3) until a commit applies an accepting import policy to the peer", label, inv.txID)
		return errors.Join(cause, inv.finalize(ctx))
	}
	cause := fmt.Errorf("peer %s: transaction %s rejected before its import invalidation was undone", label, inv.txID)
	err := errors.Join(cause, inv.abort(nil))
	inv.release()
	return err
}

// recordExpiry keeps err for txID's late receipt, up to sourcePolicyExpiredMax
// transactions.
func (c *sourcePolicyCoordinator) recordExpiry(txID string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.expired == nil {
		c.expired = make(map[string][]error)
	}
	if _, known := c.expired[txID]; !known && len(c.expired) >= sourcePolicyExpiredMax {
		return
	}
	c.expired[txID] = append(c.expired[txID], err)
}

// takeExpired returns and forgets txID's expiry reports.
func (c *sourcePolicyCoordinator) takeExpired(txID string) []error {
	c.mu.Lock()
	defer c.mu.Unlock()
	reports := c.expired[txID]
	delete(c.expired, txID)
	return reports
}
