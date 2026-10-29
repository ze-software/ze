// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection

package rib

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC8212SourcePolicyRIBAbortRestoresPreviousBound proves an abort puts
// back the PeerRIB's denial bound from before its apply. Goal: an earlier
// accepted denial survives a later aborted transaction, and an abort before
// apply changes no bound. Method: finalize one denial, admit a newer path,
// apply and abort a second transaction, then abort a third before apply.
func TestRFC8212SourcePolicyRIBAbortRestoresPreviousBound(t *testing.T) {
	f := newSourcePolicyFixture(t)
	r := f.r
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx1", f.source, 5, 6))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyFinalize, "tx1", f.source, 5, 0))
	peerRIB := r.bgpPeers[f.source]
	require.Equal(t, uint64(5), peerRIB.SourcePolicyDeniedThrough())

	receiveIPv4Route(t, r, f.source.String(), 103, "10.3.0.0/24", "180a0300")
	peerRIB.ModifyFamilyEntry(family.IPv4Unicast, []byte{24, 10, 3, 0}, func(entry *storage.RouteEntry) { entry.SourcePolicyEpoch = 6 })
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx2", f.source, 6, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx2", f.source, 6, 7))
	require.Equal(t, uint64(6), peerRIB.SourcePolicyDeniedThrough())
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx2", f.source, 6, 8))
	require.Equal(t, uint64(5), peerRIB.SourcePolicyDeniedThrough(), "abort restores the earlier accepted bound")
	require.Equal(t, 1, f.sourceCandidates(t, []byte{24, 10, 3, 0}), "the restored path is a candidate again")

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx3", f.source, 8, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx3", f.source, 8, 9))
	require.Equal(t, uint64(5), peerRIB.SourcePolicyDeniedThrough(), "an abort before apply leaves the bound alone")
}
