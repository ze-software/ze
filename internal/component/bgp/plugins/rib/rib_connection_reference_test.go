// Design: docs/architecture/plugin/rib-storage-design.md -- retained original physical connection.
package rib

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"testing"
)

// Actual SDK lease intake and owned clones keep old original provenance after
// another connection of the same Peer has acquired its own received reference.
// MUTATION: omit connectionID from the lease's NewSourceReference constructor.
func TestRIBSourceReferenceKeepsOriginalConnectionAfterReplacement(t *testing.T) {
	for _, native := range []bool{false, true} {
		r, calls := sourceLeaseTestRIB(t, native)
		peer := netip.MustParseAddr(sourceIdentityAddress)
		const connection uint64 = 1<<53 + 71
		original, err := r.sourceLeases.acquire(t.Context(), peer, 81, 701, connection)
		require.NoError(t, err)
		entry := storage.NewRouteEntry()
		entry.SetSourceReference(original)
		retained := entry.Clone()
		entry.Release()
		original.Release()
		defer retained.Release()
		replacement, err := r.sourceLeases.acquire(t.Context(), peer, 81, 702, connection+1)
		require.NoError(t, err)
		defer replacement.Release()
		require.Equal(t, connection, retained.SourceConnectionID())
		require.Equal(t, connection+1, replacement.ConnectionID())
		require.Equal(t, uint64(81), retained.SourcePeerOwner())
		require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, 701))
		require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, 702))
		invalid, err := r.sourceLeases.acquire(t.Context(), peer, 81, 701, connection+1)
		require.Error(t, err)
		require.Nil(t, invalid)
		require.Equal(t, connection, retained.SourceConnectionID())
		invalid, err = r.sourceLeases.acquire(t.Context(), peer, 81, 703, 0)
		require.Error(t, err)
		require.Nil(t, invalid)
		require.Zero(t, calls.count(rpc.SourceLeaseAcquire, 703))
	}
}
