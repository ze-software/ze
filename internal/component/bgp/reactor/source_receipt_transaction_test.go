// Design: docs/architecture/api/process-protocol.md -- reversible source grants.
// Related: source_receipt_transaction.go -- transaction-owned original facts.
package reactor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestSourceGrantAbortRestoresRetainedAuthority proves an already-DOWN source
// returns through fresh authorization while old guards and unrelated scopes
// keep their original outcomes. No peer-wide policy epoch changes.
func TestSourceGrantAbortRestoresRetainedAuthority(t *testing.T) {
	makeProcess := sourceLeaseProcessFactory(t)
	first, second := makeProcess("rib-one"), makeProcess("rib-two")
	owner, err := first.SelectedOwner()
	require.NoError(t, err)
	otherOwner, err := second.SelectedOwner()
	require.NoError(t, err)
	peer, facts := sourceReceiptFixture(t)
	sibling, siblingFacts := sourceReceiptFixture(t)
	sibling.settings.Port = 1179
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	siblingLive, err := receipts.issue(siblingFacts)
	require.NoError(t, err)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	other, err := receipts.acquire(live.ID(), otherOwner)
	require.NoError(t, err)
	unrelated, err := receipts.acquire(siblingLive.ID(), owner)
	require.NoError(t, err)
	require.NoError(t, lease.retain())
	require.NoError(t, other.retain())
	peer.state.Store(int32(PeerStateStopped))
	live.close()
	epoch := peer.sourcePolicyEpoch.Load()
	old, err := lease.begin(epoch)
	require.NoError(t, err)
	old.complete()
	prepared, err := receipts.prepareGrant(peer, owner)
	require.NoError(t, err)
	applied, err := prepared.apply(t.Context())
	require.NoError(t, err)
	require.Error(t, prepared.abort(), "a stale prepared alias cannot discard applied pins")
	require.Error(t, old.current())
	_, err = receipts.acquire(live.ID(), owner)
	require.Error(t, err, "public acquisition cannot resurrect a closed receipt")
	for _, survivor := range []*sourceReceiptLease{other, unrelated} {
		write, beginErr := survivor.begin(survivor.record.facts.source.info.peer.sourcePolicyEpoch.Load())
		require.NoError(t, beginErr, "other owner and same-IP sibling source remain useful")
		require.NoError(t, write.current())
		write.complete()
	}
	peer.settings.ProcessBindings = []ProcessBinding{sourceLeaseGrant(first.Name())}
	restored, err := applied.restore(&pluginserver.Server{}, first)
	require.NoError(t, err)
	fresh, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	require.NotSame(t, lease, fresh)
	require.NotEqual(t, lease.lifetime, fresh.lifetime)
	require.Error(t, lease.lifetime.Err())
	require.Error(t, lease.current(), "restoring config never revives the old lease")
	require.NoError(t, fresh.lifetime.Err())
	write, err := fresh.begin(epoch)
	require.NoError(t, err)
	require.NoError(t, write.current())
	require.Equal(t, old.source(), write.source(), "restoration preserves original session facts")
	write.complete()
	require.Equal(t, epoch, peer.sourcePolicyEpoch.Load())
	require.Equal(t, PeerStateStopped, peer.State(), "source restoration invents no physical UP")
	require.NoError(t, restored.finish())
	require.Empty(t, receipts.grants)
	require.NoError(t, fresh.revoke(t.Context()))
	require.NoError(t, other.revoke(t.Context()))
	siblingLive.close()
	require.NoError(t, unrelated.revoke(t.Context()))
	require.Empty(t, receipts.entries)
}

// TestSourceGrantApplyJoinsOldGuard proves every old guard is canceled before
// acknowledgment, and even a canceled wait cannot make restoration overtake it.
func TestSourceGrantApplyJoinsOldGuard(t *testing.T) {
	proc := sourceLeaseProcessFactory(t)("rib-one")
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	require.NoError(t, lease.retain())
	peer.state.Store(int32(PeerStateStopped))
	live.close()
	write, err := lease.begin(peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	defer write.complete()
	prepared, err := receipts.prepareGrant(peer, owner)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	applied, err := prepared.apply(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, applied, "failed join still owns applied cleanup and rollback")
	select {
	case <-write.done():
	default:
		t.Fatal("apply failed to cancel its outstanding guard")
	}
	require.Error(t, write.current())
	peer.settings.ProcessBindings = []ProcessBinding{sourceLeaseGrant(proc.Name())}
	_, err = applied.restore(&pluginserver.Server{}, proc)
	require.Error(t, err, "restoration must not overtake an outstanding old writer")
	require.ErrorIs(t, applied.finalize(ctx), context.Canceled,
		"finalize cannot acknowledge while an old guard still owns its write")
	require.Len(t, receipts.entries, 1)
	require.Len(t, receipts.grants, 1)
	write.complete()
	require.NoError(t, applied.finalize(t.Context()))
	require.Empty(t, receipts.entries)
	require.Empty(t, receipts.grants)
}

// TestSourceGrantRestoreAuthenticatesExactGrant proves knowledge of pinned
// facts cannot substitute for the original Process and its restored grant.
func TestSourceGrantRestoreAuthenticatesExactGrant(t *testing.T) {
	makeProcess := sourceLeaseProcessFactory(t)
	proc := makeProcess("rib-one")
	replacement := makeProcess("rib-one")
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	lease, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	require.NoError(t, lease.retain())
	peer.state.Store(int32(PeerStateStopped))
	live.close()
	prepared, err := receipts.prepareGrant(peer, owner)
	require.NoError(t, err)
	applied, err := prepared.apply(t.Context())
	require.NoError(t, err)
	server := &pluginserver.Server{}
	_, err = applied.restore(server, proc)
	require.Error(t, err, "the original grant is still removed")
	derived := sourceLeaseGrant(proc.Name())
	derived.Explicit = false
	peer.settings.ProcessBindings = []ProcessBinding{derived}
	_, err = applied.restore(server, proc)
	require.Error(t, err, "derived grants cannot authorize restoration")
	peer.settings.ProcessBindings = []ProcessBinding{sourceLeaseGrant(proc.Name())}
	_, err = applied.restore(server, replacement)
	require.Error(t, err, "a same-name successor cannot restore old ownership")
	_, err = receipts.acquire(live.ID(), owner)
	require.Error(t, err, "failed restorations left no usable lease")
	restored, err := applied.restore(server, proc)
	require.NoError(t, err)
	fresh, err := receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	write, err := fresh.begin(peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	write.complete()
	require.NoError(t, restored.finish())
	require.NoError(t, fresh.revoke(t.Context()))
	require.Empty(t, receipts.entries)
}

// TestSourceGrantPinsTrackOnlyOpenTransactions churns actual prepare/abort and
// apply/finalize cycles, proving duplicate prepare cannot create history.
func TestSourceGrantPinsTrackOnlyOpenTransactions(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	owner, _ := sourceReceiptOwner(t)
	var receipts sourceReceiptStore
	for range 64 {
		live, err := receipts.issue(facts)
		require.NoError(t, err)
		lease, err := receipts.acquire(live.ID(), owner)
		require.NoError(t, err)
		live.close()
		prepared, err := receipts.prepareGrant(peer, owner)
		require.NoError(t, err)
		_, err = receipts.prepareGrant(peer, owner)
		require.Error(t, err, "one actual owner/source transaction owns the pins")
		require.NoError(t, prepared.abort())
		write, err := lease.begin(peer.sourcePolicyEpoch.Load())
		require.NoError(t, err, "prepare-only abort preserves useful old authority")
		write.complete()
		prepared, err = receipts.prepareGrant(peer, owner)
		require.NoError(t, err)
		applied, err := prepared.apply(t.Context())
		require.NoError(t, err)
		require.Len(t, receipts.entries, 1, "actual applied transaction owns original facts")
		require.NoError(t, applied.finalize(t.Context()))
		require.Empty(t, receipts.entries)
		require.Empty(t, receipts.grants)
	}
}

// TestSourceGrantRestoreRefusesRemovedSource proves transaction pins never
// revive a permanently removed peer, even after its attachment is restored.
func TestSourceGrantRestoreRefusesRemovedSource(t *testing.T) {
	proc := sourceLeaseProcessFactory(t)("rib-one")
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	_, err = receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	live.close()
	prepared, err := receipts.prepareGrant(peer, owner)
	require.NoError(t, err)
	applied, err := prepared.apply(t.Context())
	require.NoError(t, err)
	peer.settings.ProcessBindings = []ProcessBinding{sourceLeaseGrant(proc.Name())}
	peer.stopping.Store(true)
	_, err = applied.restore(&pluginserver.Server{}, proc)
	require.Error(t, err)
	require.NoError(t, applied.finalize(t.Context()))
	require.Empty(t, receipts.entries)
}

// TestSourceGrantRestoreRefusesEndedOwner proves real Process cancellation
// cannot be undone by config restoration or by an outstanding transaction pin.
func TestSourceGrantRestoreRefusesEndedOwner(t *testing.T) {
	proc := sourceLeaseProcessFactory(t)("rib-one")
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	peer, facts := sourceReceiptFixture(t)
	var receipts sourceReceiptStore
	live, err := receipts.issue(facts)
	require.NoError(t, err)
	_, err = receipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	live.close()
	prepared, err := receipts.prepareGrant(peer, owner)
	require.NoError(t, err)
	applied, err := prepared.apply(t.Context())
	require.NoError(t, err)
	peer.settings.ProcessBindings = []ProcessBinding{sourceLeaseGrant(proc.Name())}
	proc.Stop()
	_, err = applied.restore(&pluginserver.Server{}, proc)
	require.Error(t, err)
	require.NoError(t, applied.finalize(t.Context()))
	require.Empty(t, receipts.entries)
	require.Empty(t, receipts.grants)
}
