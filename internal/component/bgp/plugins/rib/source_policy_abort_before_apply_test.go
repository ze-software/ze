// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owners
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC8212SourcePolicyRIBAbortBeforeApply proves the zero-target abort.
//
// VALIDATES: an abort of a transaction that was prepared and never applied
// only releases the pins: every path keeps the epoch it was admitted under,
// stays a candidate, and nothing is published or written downstream; the
// source is free for the next prepare; after apply a zero target is refused
// and the denial stands.
// PREVENTS: an abort before apply restamping paths, so the owner's later
// sent-owner writes carry an epoch the engine no longer issues.
func TestRFC8212SourcePolicyRIBAbortBeforeApply(t *testing.T) {
	f := newSourcePolicyFixture(t)
	r := f.r
	shared := []byte{24, 10, 1, 0}
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx1", f.source, 5, 0))
	events, commands := f.bus.eventCount(), len(f.commands)
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 0))
	r.bgpPeers[f.source].IterateFamily(family.IPv4Unicast, func(_ []byte, entry storage.RouteEntry) bool {
		require.Equal(t, uint64(5), entry.SourcePolicyEpoch, "an abort before apply restamped a path")
		return true
	})
	require.Equal(t, 1, f.sourceCandidates(t, shared))
	require.Equal(t, events, f.bus.eventCount(), "an abort before apply publishes nothing")
	require.Len(t, f.commands, commands, "an abort before apply writes nothing downstream")
	sent := r.ribOut[f.destination][family.IPv4Unicast][ribOutKey{Prefix: netip.MustParsePrefix("10.1.0.0/24")}]
	require.Equal(t, uint64(5), sent.SourcePolicyEpoch, "the sent inventory is untouched")
	require.Nil(t, r.sourcePolicy[f.source].received, "abort releases the pins")
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx1", f.source, 5, 0), "duplicate abort")

	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyPrepare, "tx2", f.source, 5, 0))
	require.NoError(t, runSourcePolicy(t, r, rpc.SourcePolicyApply, "tx2", f.source, 5, 6))
	require.Error(t, runSourcePolicy(t, r, rpc.SourcePolicyAbort, "tx2", f.source, 5, 0), "a zero target after apply")
	require.Equal(t, 0, f.sourceCandidates(t, shared), "the refused abort changed the denial")
}
