// Design: docs/architecture/behavior/peer-lifecycle.md -- physical identity across delayed event delivery.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The actual factory binds both OPEN directions and EOR to the issuing
// Session even after replay rotation and replacement of the current Session.
// MUTATION: read connectionIDOf(peer.session) in notifyOneMessage.
func TestCapturedPhysicalConnectionSurvivesReplayRotationAndReplacement(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	settings := &PeerSettings{Address: netip.MustParseAddr("192.0.2.87"), Port: 1179, LocalAS: 65000, PeerAS: 65001}
	acceptingPolicies(t, settings)
	require.NoError(t, r.AddPeer(settings))
	peer := r.peers[settings.PeerKey()]
	original := sourceSessionFixture(t, peer)
	require.NotZero(t, original.connectionID)
	require.Equal(t, original.connectionID, original.initialReplay.Load())
	birth := original.connectionID
	peer.takeInitialReplayReceipt()
	require.Equal(t, birth, original.connectionID)
	require.NotEqual(t, birth, original.initialReplay.Load(), "physical identity is independent of rotated replay")
	var store sourceReceiptStore
	require.NoError(t, original.captureSourceReceipt(peer, &store))
	t.Cleanup(original.closeSourceReceipt)
	require.Equal(t, birth, store.entries[original.sourceReceiptID()].facts.connectionID)
	require.Equal(t, birth, store.entries[original.sourceReceiptID()].facts.source.info.filterInfo.ConnectionID)
	peer.refreshForwardFacts()
	oldFacts := peer.forwardFacts()
	require.Equal(t, birth, oldFacts.filterInfo.ConnectionID)
	replacement := sourceSessionFixture(t, peer)
	require.NotEqual(t, birth, replacement.connectionID)
	peer.refreshForwardFacts()
	require.Equal(t, birth, oldFacts.filterInfo.ConnectionID, "captured forward view remains original")
	require.Equal(t, replacement.connectionID, peer.forwardFacts().filterInfo.ConnectionID)
	var got plugin.PeerInfo
	r.setMessageReceiver(&testMessageReceiver{onReceived: func(p plugin.PeerInfo, _ bgptypes.RawMessage) { got = p }, onSent: func(p plugin.PeerInfo, _ bgptypes.RawMessage) { got = p }})
	for _, direction := range []rpc.MessageDirection{rpc.DirectionReceived, rpc.DirectionSent} {
		body := []byte{4, 253, 232, 0, 90, 192, 0, 2, 1, 0}
		original.onMessageReceived(replacement, settings.Address, msgtype.TypeOPEN, body, nil, 0, direction, BufHandle{}, nil, "", 0)
		require.Equal(t, birth, got.ConnectionID, "the callback cannot borrow the pointer supplied by its caller")
		require.Empty(t, got.SelectedOwner, "source-only delivery grants no ordinary output")
	}
	eor := []byte{0, 0, 0, 0}
	update := wireu.NewWireUpdate(eor, original.recvCtxID)
	original.onMessageReceived(replacement, settings.Address, msgtype.TypeUPDATE, eor, update, original.recvCtxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: eor}, nil, "", 0)
	require.Equal(t, birth, got.ConnectionID, "EOR carries the exact same original physical connection")
	r.recentUpdates.abortPublication(update.MessageID())
	current := establishedPeerInfo(peer)
	require.Equal(t, replacement.connectionID, current.ConnectionID)
	require.Equal(t, birth, store.entries[original.sourceReceiptID()].facts.connectionID, "replacement UP cannot rewrite original source facts")
	require.Zero(t, store.originalConnectionID(original.sourceReceiptID(), peer.sourceOwner+1))
	require.Equal(t, birth, store.originalConnectionID(original.sourceReceiptID(), peer.sourceOwner))
	require.NoError(t, replacement.captureSourceReceipt(peer, &store))
	t.Cleanup(replacement.closeSourceReceipt)
	original.closeSourceReceipt()
	require.Zero(t, store.originalConnectionID(original.sourceReceiptID(), peer.sourceOwner), "finished original leaves no history")
	require.Equal(t, replacement.connectionID, store.originalConnectionID(replacement.sourceReceiptID(), peer.sourceOwner), "cleanup preserves the useful replacement")
}
