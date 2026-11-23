// Design: docs/architecture/plugin/rib-storage-design.md -- original recovery fixture provenance.
// Related: source_session.go -- issuing constructor; relay_recovery.go -- original-source guards.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// recoveryFixtureOrigin holds immutable original source facts; zero is invalid.
type recoveryFixtureOrigin struct {
	address                    netip.Addr
	owner, receipt, generation uint64
}

// captureRecoveryFixtureOrigin runs before a lookup callback or lifecycle cut.
// It uses the explicitly supplied Peer and Session, never an address lookup.
// A primitive fixture already marked Established starts its first generation
// here; no nonzero generation is changed or an old receipt relabeled.
// The test MUST retain this value across the cut and close the same delivery
// reference after its last use. This helper registers that paired cleanup.
func captureRecoveryFixtureOrigin(t testing.TB, peer *Peer, session *Session, context bgpctx.ContextID) recoveryFixtureOrigin {
	t.Helper()
	require.NotNil(t, session)
	require.Equal(t, PeerStateEstablished, peer.State())
	if peer.forwardGeneration.Load() == 0 {
		peer.advanceForwardGeneration()
	}
	session.setRecvCtxID(context)
	if session.sourceReceiptID() == 0 {
		require.NoError(t, session.captureSourceReceipt(peer, &sourceReceipts))
		// closeSourceReceipt MUST follow the fixture's last use of these facts.
		t.Cleanup(session.closeSourceReceipt)
	}
	original := sourceReceipts.recoverySource(session.sourceReceiptID(), peer.sourceOwner)
	require.True(t, original.ok)
	require.Same(t, peer, original.info.peer)
	require.Equal(t, peer.forwardGeneration.Load(), original.generation, "an older Session cannot become a later generation")
	require.Equal(t, session.connectionID, sourceReceipts.originalConnectionID(session.sourceReceiptID(), peer.sourceOwner))
	return recoveryFixtureOrigin{address: original.addr, owner: peer.sourceOwner, receipt: session.sourceReceiptID(), generation: original.generation}
}

// recoveryFixtureSession creates a connection only at the caller's explicit
// fixture lifecycle boundary. It never advances or infers a generation.
func recoveryFixtureSession(t testing.TB, peer *Peer, context bgpctx.ContextID) *Session {
	t.Helper()
	session := peer.newSession()
	session.setSendCtxID(context)
	session.setRecvCtxID(context)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	return session
}
