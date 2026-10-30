// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owners
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection

package adj_rib_in

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC8212SourcePolicyAdjRIBInAbortBeforeApply proves the zero-target abort.
//
// VALIDATES: an abort of a transaction that was prepared and never applied
// only releases the pins: every route keeps the epoch it was admitted under
// and stays eligible and replayable, and no denial bound appears; the source
// is free for the next prepare; after apply a zero target is refused and the
// denial stands.
// PREVENTS: an abort before apply restamping routes to an epoch the engine
// never issued.
func TestRFC8212SourcePolicyAdjRIBInAbortBeforeApply(t *testing.T) {
	f := newAdjSourcePolicyFixture(t)
	r := f.r
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 0))
	r.ribIn[f.source].Range(func(_ compactRouteKey, _ uint64, route *RawRoute) bool {
		require.Equal(t, uint64(5), route.SourcePolicyEpoch, "an abort before apply restamped a route")
		return true
	})
	require.True(t, f.eligible(f.source, 0))
	require.Equal(t, 3, f.replayed(t))
	_, denied := r.policyDenied[f.source]
	require.False(t, denied, "an abort before apply left a denial bound")
	require.Nil(t, r.sourcePolicy[f.source].installed, "abort releases the pins")
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 0), "duplicate abort")

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx2", f.source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx2", f.source, 5, 6))
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx2", f.source, 5, 0), "a zero target after apply")
	require.False(t, f.eligible(f.source, 0), "the refused abort changed the denial")
}
