// Design: docs/architecture/api/process-protocol.md -- live enrollment and actual source grants.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func boundSourceGrant(t *testing.T, peer *Peer, name string) {
	t.Helper()
	setTestPeerBus(t, peer, testBusSourceLease(name))
}
func TestBoundLiveSourceGrantCutRefusesNewEnrollment(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var store sourceReceiptStore
	live, err := store.issue(facts)
	require.NoError(t, err)
	defer live.close()
	owner, done := sourceReceiptOwner(t)
	defer close(done)
	boundSourceGrant(t, peer, "foreign")
	_, err = store.beginLive(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.Error(t, err)
	boundSourceGrant(t, peer, "rib")
	write, err := store.beginLive(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	require.NoError(t, write.current())
	boundSourceGrant(t, peer, "foreign")
	_, err = store.beginLive(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.Error(t, err, "an existing transient guard cannot grant another live write after the receive cut")
	prepared, err := store.prepareGrant(peer, owner)
	require.NoError(t, err)
	write.complete()
	applied, err := prepared.apply(t.Context())
	require.NoError(t, err)
	require.NoError(t, applied.finalize(t.Context()))
	require.Empty(t, live.record.leases)
}
