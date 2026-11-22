// Design: docs/architecture/api/process-protocol.md -- complete original fixture hand-back targets.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// fixtureForwardTarget explicitly installs the fixture's actual Session through
// its existing NewSession-based initializer before the tested dispatch. It then
// freezes address, stable Peer owner and that Session's issued replay receipt.
// It MUST NOT be called to reinterpret a returned target after dispatch.
func fixtureForwardTarget(t testing.TB, peer *Peer) rpc.ForwardTarget {
	t.Helper()
	require.NotNil(t, peer)
	session := peer.testInitialSync()
	require.NotNil(t, session)
	require.NotZero(t, peer.sourceOwner)
	require.NotZero(t, session.initialReplay.Load())
	peer.fwdFacts.Store(peer.buildForwardFacts())
	return rpc.ForwardTarget{Destination: peer.Settings().PeerKey().String(), DestinationOwner: peer.sourceOwner, DestinationSession: session.initialReplay.Load()}
}

// fixtureForwardTargetForPool captures the original target for a pool-intercept
// fixture whose destination carries no socket, keeping reactorForwardRS on
// fwdPool.TryDispatch instead of dropping on tryDirectWriteNoFlush's idle check.
func fixtureForwardTargetForPool(t testing.TB, peer *Peer) rpc.ForwardTarget {
	t.Helper()
	target := fixtureForwardTarget(t, peer)
	peer.fwdChannelPending.Add(1)
	t.Cleanup(func() { peer.fwdChannelPending.Add(-1) })
	return target
}

// fixtureForwardOrigin binds a primitive forwarding fixture to the original
// Session explicitly supplied by its owner. The production receipt constructor
// and callback stamp the wire; no current-Session or address lookup supplies an
// identity after capture. The fixture MUST close that same receipt at cleanup.
func fixtureForwardOrigin(t testing.TB, update *ReceivedUpdate, peer *Peer, session *Session) {
	t.Helper()
	require.NotNil(t, update)
	require.NotNil(t, update.WireUpdate)
	require.NotNil(t, peer)
	require.NotNil(t, session)
	if peer.forwardGeneration.Load() == 0 {
		peer.advanceForwardGeneration()
	}
	session.setRecvCtxID(update.WireUpdate.SourceCtxID())
	session.SetSourceID(peer.SourceID())
	if session.sourceReceiptID() == 0 {
		require.NoError(t, session.captureSourceReceipt(peer, &sourceReceipts))
		t.Cleanup(session.closeSourceReceipt)
	}
	update.WireUpdate.SetSourceID(peer.SourceID())
	stamp := session.sourceMessageCallback(func(captured *Session, _ netip.Addr, _ msgtype.MessageType, _ []byte, wire *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		require.Same(t, session, captured)
		update.SourceOwner = wire.SourceOwner()
		update.SourceReceipt = wire.SourceReceipt()
		return false
	})
	stamp(session, peer.Settings().Address, msgtype.TypeUPDATE, update.WireUpdate.Payload(), update.WireUpdate, update.WireUpdate.SourceCtxID(), rpc.DirectionReceived, BufHandle{}, nil, "", 0)
	require.Equal(t, peer.sourceOwner, update.SourceOwner)
	require.NotZero(t, update.SourceReceipt)
	update.receivedPeer = peer
	update.receivedGeneration = peer.forwardGeneration.Load()
	update.SourcePolicyEpoch = peer.sourcePolicyEpoch.Load()
}
