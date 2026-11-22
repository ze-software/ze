// Design: docs/architecture/api/process-protocol.md -- original targets through final output.
package reactor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestBoundRelayEmptyWorkValidatesOriginalDestination(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	require.NoError(t, f.target.api.RelayStoredRouteBound(t.Context(), f.target.peer.settings.Address, nil, f.sender, f.target.owner))
	captured := rpc.ForwardTarget{Destination: f.target.peer.settings.PeerKey().String(), DestinationOwner: f.target.peer.sourceOwner, DestinationSession: f.target.peer.currentSession().initialReplay.Load()}
	require.NoError(t, f.target.api.ForwardUpdatesBound(t.Context(), nil, []rpc.ForwardTarget{captured}, "", f.target.owner.Sender(), f.target.owner))
	f.target.peer.currentSession().initialReplay.Add(1)
	require.Error(t, f.target.api.RelayStoredRouteBound(t.Context(), f.target.peer.settings.Address, nil, f.sender, f.target.owner))
	require.Error(t, f.target.api.ForwardUpdatesBound(t.Context(), nil, []rpc.ForwardTarget{captured}, "", f.target.owner.Sender(), f.target.owner))
	require.Empty(t, f.target.conn.written())
}

func TestBoundRelayPositiveReceiptRetiresAfterImportEpochChange(t *testing.T) {
	f := newBoundRelayWireFixture(t, true)
	require.NoError(t, f.relay(t, f.route))
	old := f.route
	old.Withdraw = true
	f.source.advanceSourcePolicyEpoch()
	require.NoError(t, f.relay(t, old), "exact acknowledged output retires independently of revoked announce admission")
	require.Len(t, updateBodies(t, f.target.conn.written()), 2)
	require.Empty(t, f.target.peer.adjOut.routes)
	fresh := f.route
	fresh.MsgID++
	fresh.SourcePolicyEpoch = f.source.sourcePolicyEpoch.Load()
	require.NoError(t, f.relay(t, fresh))
	require.NoError(t, f.relay(t, old))
	require.Len(t, updateBodies(t, f.target.conn.written()), 3, "older same physical message cannot erase fresh output")
	require.Len(t, f.target.peer.adjOut.routes, 1)
	old.MsgID = 0
	require.Error(t, f.relay(t, old), "unrecorded message cannot erase a known current received generation")
	require.Len(t, updateBodies(t, f.target.conn.written()), 3)
}

func TestBoundCachedMultipleTargetsShareOneAcknowledgment(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	other, otherConn := boundRelaySecondTarget(t, f, f.target.peer.settings.Address.String(), 2179)
	source, err := f.target.api.originalRelayFacts(f.source.settings.Address, &f.route, f.sender)
	require.NoError(t, err)
	cache := f.target.api.r.recentUpdates
	cache.RegisterConsumer(selectedWriterProcess)
	defer cache.UnregisterConsumer(selectedWriterProcess)
	var spans []relayAttrSpan
	_, id, _, err := f.target.api.buildRelayUpdate([]rpc.StoredRoute{f.route}, &source, &spans)
	require.NoError(t, err)
	defer cache.Release(id)
	targets := []rpc.ForwardTarget{
		{Destination: f.target.peer.settings.PeerKey().String(), DestinationOwner: f.target.peer.sourceOwner, DestinationSession: f.target.peer.currentSession().initialReplay.Load()},
		{Destination: other.settings.PeerKey().String(), DestinationOwner: other.sourceOwner, DestinationSession: other.currentSession().initialReplay.Load()},
	}
	require.NoError(t, f.target.api.ForwardUpdatesBound(context.Background(), []uint64{id, id}, targets, selectedWriterProcess, f.target.owner.Sender(), f.target.owner))
	require.Len(t, updateBodies(t, f.target.conn.written()), 1)
	require.Len(t, updateBodies(t, otherConn.written()), 1, "second target writes before the sole cumulative ACK")
	cache.mu.RLock()
	acked := cache.pluginLastAck[selectedWriterProcess]
	cache.mu.RUnlock()
	require.Equal(t, id, acked)
}
