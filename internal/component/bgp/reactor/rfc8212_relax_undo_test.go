// RFC naming: untagged -- tests the D-1 import relaxation undo RFC 8212 default reject relies on, not the Section 3 verdict itself, which rfc8212_default_reject_test.go and the test/plugin/rfc8212-* carriers prove.
package reactor

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// relaxWith is relax with the swap's settings copier supplied by the caller.
func relaxWith(fx *rfc8212ImportFixture, j configJournal, apply settingsCopier) error {
	next := fx.peer.settingsSnapshot()
	accept, err := filterapi.NewPolicy(nil, routepolicy.FinalAccept)
	if err != nil {
		return err
	}
	next.ImportPolicy = accept
	return (&reactorAPIAdapter{r: fx.r}).swapPeerSettingsJournaled([]peerSettingsSwap{{
		key: fx.peer.Settings().PeerKey(), next: next, apply: apply}}, j)
}

// TestRFC8212RelaxUndoRetriesUnappliedOwner proves a relaxation's undo
// completes the exclusion an owner's failed apply left undone. Method: the
// relaxation applies with Route Refresh; on rollback the owner refuses the
// invalidation's first apply only, as an owner whose call was lost does, and
// accepts the next. The fixture owner refuses a finalize no apply preceded,
// as every real owner's ledger does.
// VALIDATES: the rollback reports no failure; the owner applied on the retry,
// under the epoch the first apply named, and the receipt finalized it; the
// path is ineligible; the policy rejects; input stayed paused until the
// receipt and resumed after it.
// PREVENTS: an owner that never excluded the paths the undone relaxation
// admitted leaving them eligible under the restored rejecting policy, and a
// receipt finalizing an owner that never applied (RFC 8212 Section 3).
func TestRFC8212RelaxUndoRetriesUnappliedOwner(t *testing.T) {
	fx := newRelaxFixture(t, true)
	var applies atomic.Int32
	fx.owner.onApply = func() {
		first := applies.Add(1) == 1
		fx.owner.mu.Lock()
		fx.owner.pinned = !first
		fx.owner.mu.Unlock()
	}
	j := relaxJournal(fx, "tx-undo-retry")
	require.NoError(t, relax(fx, j))
	require.Equal(t, 2, refreshesOnWire(t, fx))

	require.Empty(t, j.Rollback(), "an undo the retry completed reported as failed")
	require.False(t, importAccepts(fx.peer))
	require.True(t, fx.f.s.IsPaused(), "input resumed before the transaction's receipt")
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	ops, targets, eligible, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply,
		rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}, ops)
	require.Equal(t, targets[1], targets[2], "the retry named another epoch than the apply")
	require.False(t, eligible, "a path the undone relaxation admitted stays eligible")
	require.False(t, fx.f.s.IsPaused(), "input left paused after the receipt")
}

// TestRFC8212RelaxUndoExpiryWithoutHoldTime proves the hold's expiry
// finalizes a relaxation undo's owners on a session with no HoldTimer. Method:
// HoldTime 0, so only the transaction deadline bounds the pause; the
// relaxation applies and is rolled back; no receipt is sent. Owner calls run
// under a real context while the hold fires on the fake clock, so the test
// first waits until the completion deadline has passed in real time, as it
// has in production when the hold fires.
// VALIDATES: input resumes at the expiry; the owner was finalized (prepare,
// apply, finalize) and holds no pin; the late receipt reports the expiry and
// no finalization failure.
// PREVENTS: an expiry finalizing under the completion deadline it fires at,
// which is already past, leaving every owner applied and refusing the
// source's next transaction.
func TestRFC8212RelaxUndoExpiryWithoutHoldTime(t *testing.T) {
	fx := newRelaxFixture(t, true)
	fx.f.s.timers.StopHoldTimer()
	fx.f.s.timers.SetHoldTime(0)
	j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-undo-hold-zero",
		deadline: fx.f.clock.Now().Add(3 * time.Second)}
	require.NoError(t, relax(fx, j))
	require.Equal(t, 2, refreshesOnWire(t, fx))

	require.Empty(t, j.Rollback())
	held := fx.r.sourcePolicy.transaction(j.txID)
	require.Len(t, held, 1)
	completion := held[0].receipt.CompletionDeadline()
	require.True(t, fx.f.s.IsPaused(), "input resumed before the transaction's receipt")
	time.Sleep(time.Until(completion) + 10*time.Millisecond)
	fx.f.clock.Add(completion.Sub(fx.f.clock.Now()) + 2*time.Second)
	require.Eventually(t, func() bool { return !fx.f.s.IsPaused() }, 5*time.Second, 5*time.Millisecond,
		"input stayed paused past the completion deadline")
	err := fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false)
	require.ErrorContains(t, err, "completion deadline")
	require.NotContains(t, err.Error(), "finalization failed", "the expiry finalized under a deadline already past")
	ops, _, eligible, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}, ops)
	require.False(t, eligible)
	fx.owner.mu.Lock()
	pinned := fx.owner.pinned
	fx.owner.mu.Unlock()
	require.False(t, pinned, "the owner still holds the transaction")
}

// TestRFC8212RelaxUndoReceiptBeatsExpiry proves the receipt and the hold's
// expiry never both end a relaxation's undo. Method: the relaxation applies
// and is rolled back; the rejection receipt arrives, and while its finalize
// is inside the owner's call, the hold expires (expire runs then).
// VALIDATES: the owner is finalized once; the receipt succeeds; no expiry
// is recorded for the transaction; input resumes.
// PREVENTS: an expiry that runs during the receipt's finalize finalizing the
// owners a second time and recording "sent no receipt" for a transaction
// whose receipt arrived.
func TestRFC8212RelaxUndoReceiptBeatsExpiry(t *testing.T) {
	fx := newRelaxFixture(t, true)
	j := relaxJournal(fx, "tx-undo-race")
	require.NoError(t, relax(fx, j))
	require.Equal(t, 2, refreshesOnWire(t, fx))
	require.Empty(t, j.Rollback())
	held := fx.r.sourcePolicy.transaction(j.txID)
	require.Len(t, held, 1)
	inv := held[0]

	expired := make(chan struct{})
	var once sync.Once
	fx.owner.onFinal = func() {
		once.Do(func() {
			go func() {
				inv.expire()
				close(expired)
			}()
			select {
			case <-expired:
			case <-time.After(300 * time.Millisecond):
			}
		})
	}
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	select {
	case <-expired:
	case <-time.After(5 * time.Second):
		t.Fatal("expire never returned")
	}
	ops, _, eligible, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply, rpc.SourcePolicyFinalize}, ops)
	require.False(t, eligible)
	require.Empty(t, fx.r.sourcePolicy.takeExpired(j.txID), "an expiry recorded for a transaction whose receipt arrived")
	require.False(t, fx.f.s.IsPaused(), "input left paused")
}

// TestRFC8212RelaxUndoPrepareFailureHoldsEstablishment proves a relaxation's
// undo whose session ended while its invalidation prepared restores the
// rejecting policy before any new session establishes. Method: the
// relaxation applies; on rollback, once prepare's frontier settled, the
// session tears down (Established left, then the session cleared), so prepare
// fails; while the undo restores the rejecting policy, a new session reaches
// Established (enterEstablished).
// VALIDATES: establishment completes only after the rejecting policy is
// restored; the rollback reports nothing; the policy rejects.
// PREVENTS: a session establishing between the undo's check and its restore
// admitting UPDATEs under the accepting candidate, whose paths then stay
// eligible under the restored rejecting policy (RFC 8212 Section 3).
func TestRFC8212RelaxUndoPrepareFailureHoldsEstablishment(t *testing.T) {
	fx := newRelaxFixture(t, true)
	var tearing, restored atomic.Bool
	var once sync.Once
	afterRestore := make(chan bool, 1)
	copyImport := func(dst, src *PeerSettings) {
		// src holds only what the copier moves, so the peer's EBGP
		// classification is named rather than read from it.
		restoring := tearing.Load() && !src.ImportPolicy.AcceptsAtEnd(true)
		if restoring {
			once.Do(func() {
				go func() {
					fx.peer.enterEstablished()
					afterRestore <- restored.Load()
				}()
				time.Sleep(200 * time.Millisecond)
			})
		}
		dst.ImportPolicy = src.ImportPolicy
		if restoring {
			restored.Store(true)
		}
	}
	prepareSettledHook = func(p *Peer) {
		p.setState(PeerStateConnecting)
		p.mu.Lock()
		p.session = nil
		p.mu.Unlock()
		tearing.Store(true)
	}
	t.Cleanup(func() { prepareSettledHook = nil })
	j := relaxJournal(fx, "tx-undo-establish")
	require.NoError(t, relaxWith(fx, j, copyImport))
	require.Equal(t, 2, refreshesOnWire(t, fx))

	require.Empty(t, j.Rollback())
	select {
	case wasRestored := <-afterRestore:
		require.True(t, wasRestored, "a session established before the undo restored the rejecting policy")
	case <-time.After(5 * time.Second):
		t.Fatal("establishment never completed")
	}
	require.False(t, importAccepts(fx.peer))
}

// TestRFC8212RelaxUndoAbortedExpiry proves the hold's expiry of a
// relaxation's undo that could not publish the rejecting policy reports the
// accepting policy in effect and finalizes nothing. Method: the relaxation
// applies; on rollback, once prepare's frontier settled, the fake clock
// reaches the last second of the work budget, so the undo's apply refuses
// before it publishes and aborts; no receipt is sent and the hold expires.
// VALIDATES: the rollback reports the accepting policy in effect; the late
// receipt reports an expiry naming the accepting policy and the hold-down
// sequence, and no finalization; the owner was prepared and aborted only;
// input resumes.
// PREVENTS: an expiry claiming input resumed under a restored rejecting
// policy that was never published, and finalizing owners that aborted.
func TestRFC8212RelaxUndoAbortedExpiry(t *testing.T) {
	fx := newRelaxFixture(t, true)
	j := relaxJournal(fx, "tx-undo-aborted")
	var armed atomic.Bool
	prepareSettledHook = func(*Peer) {
		if armed.Load() {
			fx.f.clock.Add(j.deadline.Sub(fx.f.clock.Now()) - 500*time.Millisecond)
		}
	}
	t.Cleanup(func() { prepareSettledHook = nil })
	require.NoError(t, relax(fx, j))
	require.Equal(t, 2, refreshesOnWire(t, fx))

	armed.Store(true)
	errs := j.Rollback()
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[0], "the rejecting import policy could not be restored, so the accepting policy stays in effect")
	require.True(t, importAccepts(fx.peer))
	held := fx.r.sourcePolicy.transaction(j.txID)
	require.Len(t, held, 1)
	fx.f.clock.Add(held[0].receipt.CompletionDeadline().Sub(fx.f.clock.Now()) + time.Millisecond)
	require.Eventually(t, func() bool { return !fx.f.s.IsPaused() }, 5*time.Second, 5*time.Millisecond,
		"input stayed paused past the completion deadline")
	err := fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false)
	require.ErrorContains(t, err, "input resumed under the accepting import policy, because restoring the rejecting one failed")
	require.ErrorContains(t, err, "`request peer 192.0.2.1 disable`, then `request peer 192.0.2.1 enable`")
	require.NotContains(t, err.Error(), "finalization failed", "the expiry finalized an aborted owner")
	ops, _, _, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyAbort}, ops)
}

// TestRFC8212RelaxUndoAppliedOwnerUnsettled proves a relaxation's undo reports
// an owner that withdrew the routes but failed to settle downstream as such,
// not as an owner whose routes stay eligible. Method: the relaxation applies
// with Route Refresh; on rollback the owner applies every time and returns a
// settle error, as bgp-rr and bgp-rs record a failed downstream settle
// (Record(input, StageApplied, settled)); its ledger then refuses the
// target-zero abort because apply landed (sourcepolicy Owner.abortable).
// VALIDATES: the undo retries the apply, its abort is refused, and the
// receipt finalizes the owner; the rollback names the owner as having
// withdrawn the routes, says a copy may stay advertised downstream and gives
// the disable/enable sequence; it never says the routes stay eligible; the
// path is ineligible and the policy rejects.
// PREVENTS: a report telling the operator that an owner which excluded the
// paths left them eligible (RFC 8212 Section 3), hiding that only its
// downstream copies may remain.
func TestRFC8212RelaxUndoAppliedOwnerUnsettled(t *testing.T) {
	fx := newRelaxFixture(t, true)
	j := relaxJournal(fx, "tx-undo-unsettled")
	require.NoError(t, relax(fx, j))
	require.Equal(t, 2, refreshesOnWire(t, fx))
	fx.owner.mu.Lock()
	fx.owner.settleErr = errors.New("downstream settle timed out")
	fx.owner.mu.Unlock()

	errs := j.Rollback()
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[0], "the rejecting import policy is restored and source-policy owner "+
		rfc8212ImportOwnerName+" withdrew the routes the undone change admitted, but it reported an error before "+
		"its change settled downstream, so a copy may stay advertised")
	require.ErrorContains(t, errs[0], "`request peer 192.0.2.1 disable`, then `request peer 192.0.2.1 enable`")
	require.NotContains(t, errs[0].Error(), "stay eligible", "an owner that withdrew the routes reported as leaving them eligible")
	require.False(t, importAccepts(fx.peer), "the failed undo re-published the accepting policy")
	require.True(t, fx.f.s.IsPaused(), "input resumed before the transaction's receipt")
	require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
	ops, targets, eligible, _ := fx.owner.seen()
	require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyApply,
		rpc.SourcePolicyApply, rpc.SourcePolicyAbort, rpc.SourcePolicyFinalize}, ops)
	require.Equal(t, targets[1], targets[2], "the retry named another epoch than the apply")
	require.Zero(t, targets[3], "the abort of an owner that may not have applied named a target epoch")
	require.False(t, eligible, "a path the owner withdrew is eligible again")
	require.False(t, fx.f.s.IsPaused(), "input left paused after the receipt")
}
