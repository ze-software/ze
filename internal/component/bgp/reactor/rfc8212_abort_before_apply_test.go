// Design: docs/architecture/behavior/peer-lifecycle.md -- in-place import invalidation
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection

package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRFC8212ImportAbortBeforeApplyKeepsEpoch proves that a transaction which
// never applied leaves the source's policy epoch alone. Goal: when an owner's
// prepare fails, or apply refuses before it publishes, the coordinator does
// not advance the epoch, opens no policy window, sends every prepared owner,
// and the owner whose prepare failed, an abort with target zero, and an
// owner's sent-owner re-send stamped with
// the unchanged epoch still passes the final writer's fence. Method: the real
// SDK owner fixture; prepare fails on an identity mismatch, and apply's
// budget is exhausted from inside the owner's prepare.
func TestRFC8212ImportAbortBeforeApplyKeepsEpoch(t *testing.T) {
	stillCurrent := func(t *testing.T, fx *rfc8212ImportFixture) {
		t.Helper()
		require.Equal(t, fx.original, fx.peer.sourcePolicyEpoch.Load(), "epoch advanced although the policy never changed")
		require.Nil(t, fx.peer.sourcePolicyWindow.Load(), "policy window opened by an abort before apply")
		owner := adjOutPath{source: fx.peer, policyEpoch: fx.original}
		require.True(t, sentOwnerCurrent(owner, 0, fx.original), "owner re-send under the unchanged epoch refused")
		s := fx.peer.settingsSnapshot()
		require.True(t, s.ImportPolicy.AcceptsAtEnd(s.IsEBGP()), "accepting policy not in place")
	}
	t.Run("prepare-fails", func(t *testing.T) {
		fx := newRFC8212ImportFixture(t, false)
		fx.owner.epoch = fx.original + 1
		j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-prepare-fails", deadline: time.Now().Add(30 * time.Second)}
		err := fx.swap(j)
		require.ErrorContains(t, err, "prepare for peer")
		require.NotContains(t, err.Error(), "compensation incomplete", "the failing owner's abort was reported as lost")
		ops, targets, _, _ := fx.owner.seen()
		// The failing owner takes the target-zero abort: its prepare may still pin.
		require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyAbort}, ops)
		require.Equal(t, uint64(0), targets[1], "abort before apply carries a target")
		require.False(t, fx.f.s.IsPaused(), "input still paused after a refused prepare")
		stillCurrent(t, fx)
	})
	t.Run("apply-refused-before-publish", func(t *testing.T) {
		fx := newRFC8212ImportFixture(t, false)
		pin := fx.owner.onPrepare
		fx.owner.onPrepare = func() {
			pin()
			fx.f.clock.Add(89500 * time.Millisecond)
		}
		j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-apply-refused", deadline: time.Now().Add(10 * time.Minute)}
		require.ErrorContains(t, fx.swap(j), "no budget left")
		ops, targets, eligible, _ := fx.owner.seen()
		require.Equal(t, []rpc.SourcePolicyOperation{rpc.SourcePolicyPrepare, rpc.SourcePolicyAbort}, ops)
		require.Equal(t, uint64(0), targets[1], "abort before apply carries a target")
		require.True(t, eligible)
		stillCurrent(t, fx)
		require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
		require.False(t, fx.f.s.IsPaused(), "input still paused after the rejection receipt")
	})
}
