// Design: docs/architecture/update-building.md -- original received provenance at final output.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The admitted retained source reaches the real final writer and useful BGP
// bytes after its Peer has installed a replacement connection. The sent event
// must identify the original source separately from the destination socket.
// MUTATION: pass receipt zero to originalConnectionID in session_write.go.
func TestFinalWriterKeepsOriginalRetainedSourceConnection(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	route := f.route(t, 1, "192.0.2.79", 0, false)
	sourceReceipts.mu.Lock()
	record := sourceReceipts.entries[route.SourceReceipt]
	original := record.facts.connectionID
	source := record.facts.source.info.peer
	lease := record.leases[f.owner]
	sourceReceipts.mu.Unlock()
	require.NotZero(t, original)
	require.NoError(t, lease.retain())
	replacement := source.newSession()
	source.mu.Lock()
	source.session = replacement
	source.mu.Unlock()
	source.advanceForwardGeneration()
	require.NotEqual(t, original, replacement.connectionID)
	var delivered plugin.PeerInfo
	var message bgptypes.RawMessage
	f.api.r.setMessageReceiver(&testMessageReceiver{onSent: func(peer plugin.PeerInfo, raw bgptypes.RawMessage) { delivered, message = peer, raw }})
	session := f.peer.currentSession()
	require.NotNil(t, session)
	session.onMessageReceived = session.sourceMessageCallback(f.api.r.messageReceiverForPeer(f.peer))
	route.Retained = true
	route.StaleLevel = rpc.SelectedStale
	answer, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[0], false))
	require.Equal(t, original, message.SourceConnectionID)
	require.Equal(t, session.connectionID, delivered.ConnectionID)
	require.NotEqual(t, replacement.connectionID, message.SourceConnectionID)
	require.NotEqual(t, delivered.ConnectionID, message.SourceConnectionID)
	require.Equal(t, source.sourceOwner, message.SourceOwner)
}
