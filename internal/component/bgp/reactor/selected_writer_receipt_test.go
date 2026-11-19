// Design: docs/architecture/api/process-protocol.md -- original sent source receipts.
// Related: selected_writer_test.go -- real selected operation and final socket.
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

// TestSelectedWriterSentReceiptKeepsRetainedOriginal reconnects the source
// before output and observes the actual writer's borrowed sent receipt. It
// must retain the old source identity, policy admission and AS_PATH bytes.
func TestSelectedWriterSentReceiptKeepsRetainedOriginal(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	route := f.route(t, 1, "192.0.2.79", 0, false)
	sourceReceipts.mu.Lock()
	lease := sourceReceipts.entries[route.SourceReceipt].leases[f.owner]
	sourceReceipts.mu.Unlock()
	require.NoError(t, lease.retain())
	source := lease.record.facts.source.info.peer
	source.advanceForwardGeneration()
	source.settings.PeerAS = 65004
	route.Retained, route.StaleLevel = true, rpc.SelectedStale
	var receipt, epoch, message uint64
	var sourceAddress string
	f.peer.session.onMessageReceived = func(_ *Session, _ netip.Addr, _ msgtype.MessageType, _ []byte, update *wireu.WireUpdate,
		_ bgpctx.ContextID, direction rpc.MessageDirection, _ BufHandle, _ map[string]any, original string, id uint64) bool {
		require.Equal(t, rpc.DirectionSent, direction)
		origin := update.SentOrigin()
		require.NotNil(t, origin)
		receipt, epoch, message, sourceAddress = origin.SourceReceipt, origin.SourcePolicyEpoch, id, original
		return false
	}
	answer, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	require.Equal(t, route.SourceReceipt, receipt)
	require.Equal(t, route.Route.SourcePolicyEpoch, epoch)
	require.Equal(t, route.Route.MsgID, message)
	require.Equal(t, route.Route.SourcePeer, sourceAddress)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 1)
	path, present := bodyPathAttr(t, bodies[0], 2)
	require.True(t, present)
	require.Equal(t, []uint32{65000, 65003}, aspathASNs(t, path, 4))
}

// TestSelectedWriterWithdrawalReceiptKeepsOriginalAdmission verifies the
// exact retirement's sent callback names the removed output's original
// admission, even though the absence selection itself has no source lease.
func TestSelectedWriterWithdrawalReceiptKeepsOriginalAdmission(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	route := f.route(t, 1, "192.0.2.79", 0, false)
	sourceReceipts.mu.Lock()
	sourceOwner := sourceReceipts.entries[route.SourceReceipt].facts.source.info.peer.sourceOwner
	sourceReceipts.mu.Unlock()
	var origins []wireu.SentOrigin
	var messages []uint64
	var addresses []string
	f.peer.session.onMessageReceived = func(_ *Session, _ netip.Addr, _ msgtype.MessageType, _ []byte, update *wireu.WireUpdate,
		_ bgpctx.ContextID, direction rpc.MessageDirection, _ BufHandle, _ map[string]any, original string, id uint64) bool {
		require.Equal(t, rpc.DirectionSent, direction)
		origin := update.SentOrigin()
		require.NotNil(t, origin)
		origins = append(origins, wireu.SentOrigin{SourceOwner: origin.SourceOwner,
			SourcePolicyEpoch: origin.SourcePolicyEpoch, SourceReceipt: origin.SourceReceipt, Local: origin.Local})
		messages, addresses = append(messages, id), append(addresses, original)
		return false
	}
	answer, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	absent := rpc.SelectedRoute{Family: route.Family, KeyNLRI: route.KeyNLRI, Revision: 2, Kind: rpc.SelectedRouteAbsent}
	answer, err = f.send(t, t.Context(), absent)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedAbsent, answer.Results[0].Outcome)
	require.Len(t, origins, 2)
	for _, origin := range origins {
		require.Equal(t, sourceOwner, origin.SourceOwner)
		require.Equal(t, route.Route.SourcePolicyEpoch, origin.SourcePolicyEpoch)
		require.Equal(t, route.SourceReceipt, origin.SourceReceipt)
		require.False(t, origin.Local)
	}
	require.Equal(t, []uint64{route.Route.MsgID, route.Route.MsgID}, messages)
	require.Equal(t, []string{route.Route.SourcePeer, route.Route.SourcePeer}, addresses)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 2)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[0], false))
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[1], true))
}
