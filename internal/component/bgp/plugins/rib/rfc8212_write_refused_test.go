// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: source_policy.go -- applySourcePolicy, abortSourcePolicy

package rib

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// errSendRefusedForTest is the refusal the engine answers when a destination's
// attach block grants this process no `send [ update ]`.
var errSendRefusedForTest = errors.New("send refused: no peer this selector names attaches this process with the required send permission")

// TestRFC8212SourcePolicyRIBWriteRefusedFailsPhase proves a destination's
// refusal of a source-policy write fails the phase that issued it.
//
// VALIDATES: an apply whose withdrawal of the forwarded copy the engine refuses
// answers an error naming the source, the destination, the engine's reason and
// the `send [ update ]` grant that fixes it; the abort that follows restores the
// copy to the sent inventory. An abort whose re-announcement is refused fails
// the same way.
// PREVENTS: the reload acknowledged as applied while the destination still
// advertises a route the new import policy rejects (fail-open), and an abort
// reported done while a destination lacks the restored route.
func TestRFC8212SourcePolicyRIBWriteRefusedFailsPhase(t *testing.T) {
	refuseDestination := func(f *sourcePolicyFixture) {
		f.r.updateRefuseHook = func(input rpc.UpdateRouteInput) error {
			if input.PeerSelector == f.destination.String() {
				return errSendRefusedForTest
			}
			return nil
		}
	}

	t.Run("apply", func(t *testing.T) {
		f := newSourcePolicyFixture(t)
		r := f.r
		refuseDestination(f)
		require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
		err := runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6)
		require.Error(t, err, "a refused withdrawal must fail the apply")
		require.ErrorIs(t, err, errSendRefusedForTest, "the receipt carries the engine's reason")
		require.Contains(t, err.Error(), "peer "+f.source.String())
		require.Contains(t, err.Error(), "peer "+f.destination.String()+" refused the write")
		require.Contains(t, err.Error(), "`send [ update ]`", "the receipt names the missing grant")

		r.updateRefuseHook = nil
		require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 7))
		require.Len(t, r.ribOut[f.destination], 1, "the abort returns the copy to the sent inventory")
	})

	t.Run("abort", func(t *testing.T) {
		f := newSourcePolicyFixture(t)
		r := f.r
		require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
		require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6))
		refuseDestination(f)
		err := runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 7)
		require.Error(t, err, "a refused re-announcement must fail the abort")
		require.ErrorIs(t, err, errSendRefusedForTest)
		require.Contains(t, err.Error(), "peer "+f.destination.String()+" refused the write")
		require.Contains(t, err.Error(), "`send [ update ]`")
	})
}
