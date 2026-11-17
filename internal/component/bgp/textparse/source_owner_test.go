// Design: docs/architecture/api/text-parser.md -- reserved Peer incarnation.
package textparse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSourceProvenanceKeepsPeerOwnerInHeader drives the shared reserved scan;
// malformed header owners fail, while later route content supplies no authority.
func TestSourceProvenanceKeepsPeerOwnerInHeader(t *testing.T) {
	const header = "peer 192.0.2.1 remote as 65001 received update 42"
	const owner = uint64(18446744073709551615)
	epoch, receipt, peerOwner, err := SourceProvenance(header + " source-policy-epoch 9 source-receipt 7 peer-owner 18446744073709551615 origin igp peer-owner 99")
	require.NoError(t, err)
	require.Equal(t, uint64(9), epoch)
	require.Equal(t, uint64(7), receipt)
	require.Equal(t, owner, peerOwner)
	_, _, peerOwner, err = SourceProvenance(header + " origin igp peer-owner 99")
	require.NoError(t, err)
	require.Zero(t, peerOwner, "route content cannot supply header authority")
	for _, suffix := range []string{" peer-owner", " peer-owner -1", " peer-owner 1.5", " peer-owner 7 peer-owner 8", " peer-owner 7 source-receipt bad"} {
		_, _, _, err = SourceProvenance(header + suffix)
		require.Error(t, err)
		_, err = SourcePolicyEpoch(header + suffix)
		require.Error(t, err, "the epoch reader validates all reserved provenance")
	}
}
