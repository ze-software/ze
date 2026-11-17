// Design: docs/architecture/plugin/rib-storage-design.md -- immutable original owner on a shared source lease.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// These checked public views let the test run against the pre-carrier code:
// its failure names the missing provenance reader rather than a compile error.
type sourceOwnerReferenceView interface {
	PeerOwner() uint64
}

type sourceOwnerEntryView interface {
	SourcePeerOwner() uint64
}

// TestRIBSourceLeaseReferencePreservesPeerOwner acquires through the actual
// SDK operation, then holds an owned storage clone after its original entry
// releases. The one shared reference must retain its exact original owner.
func TestRIBSourceLeaseReferencePreservesPeerOwner(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "wire"
		if native {
			name = "native"
		}
		t.Run(name, func(t *testing.T) {
			r, calls := sourceLeaseTestRIB(t, native)
			const owner = ^uint64(0)
			const receipt uint64 = 8821
			source, err := r.sourceLeases.acquire(t.Context(), netip.MustParseAddr(sourceIdentityAddress), owner, receipt)
			require.NoError(t, err)
			defer source.Release()
			view, ok := any(source).(sourceOwnerReferenceView)
			require.True(t, ok, "a successfully acquired shared source reference must expose its immutable Peer owner")
			require.Equal(t, owner, view.PeerOwner())

			entry := storage.NewRouteEntry()
			entry.SetSourceReference(source)
			defer entry.Release()
			clone := entry.Clone()
			require.NotNil(t, clone)
			defer clone.Release()
			entry.Release()
			retained, ok := any(clone).(sourceOwnerEntryView)
			require.True(t, ok, "a retained entry must read its original shared reference")
			require.Equal(t, owner, retained.SourcePeerOwner())
			require.Equal(t, int64(2), source.References(), "the caller and clone own the same reference")
			require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, receipt))
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, receipt))
		})
	}
}
