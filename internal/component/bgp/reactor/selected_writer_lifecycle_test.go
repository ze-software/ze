// Design: docs/architecture/bgp/structural-forwarding.md -- independent destination writers.
// Related: selected_writer_test.go -- real selected-operation and socket fixture.
package reactor

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedWriterBlockedDestinationDoesNotBlockSibling holds A's actual
// socket flush, observes B's complete selected UPDATE, then cancels and joins A.
func TestSelectedWriterBlockedDestinationDoesNotBlockSibling(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	other, otherWire := newGroupUpdatesPeer(t, "192.0.2.81", "absent")
	other.settings.ExportPolicy = f.peer.settings.ExportPolicy
	other.settings.LocalAddress = f.peer.settings.LocalAddress
	other.settings.ProcessBindings = f.peer.settings.ProcessBindings
	other.sendCtx.Store(f.peer.sendCtx.Load())
	other.sendCtxID, other.session.sendCtxID = f.peer.sendCtxID, f.peer.sendCtxID
	other.session.initialReplay.Store(72)
	other.reactor = f.api.r
	other.refreshForwardFacts()
	other.session.egressFinalAccepts = func(kind filterapi.ExportKind) bool {
		return other.forwardFacts().exportPolicy.AcceptsExportAtEnd(true, kind)
	}
	f.api.r.peers[other.Settings().PeerKey()] = other
	f.api.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: other.Settings().PeerKey()}, 4096)
	destinationGrant := plugin.PeerProcessBinding{PluginName: selectedWriterProcess, Explicit: true,
		Receive: map[string]events.Direction{"state": events.DirBoth, "refresh": events.DirBoth, "update": events.DirSent},
		Send:    map[string]bool{"update": true}}
	f.api.r.api.UpdateDeliveryGraph("bgp", []pluginserver.DeliveryPeer{
		{Addr: "192.0.2.79", Bindings: []plugin.PeerProcessBinding{{PluginName: selectedWriterProcess, Explicit: true,
			Receive: map[string]events.Direction{"update": events.DirReceived, "state": events.DirBoth}}}},
		{Addr: "192.0.2.80", Bindings: []plugin.PeerProcessBinding{destinationGrant}},
		{Addr: "192.0.2.81", Bindings: []plugin.PeerProcessBinding{destinationGrant}},
	})
	writer, remote := net.Pipe()
	t.Cleanup(func() { _ = writer.Close(); _ = remote.Close() })
	connection := &observedWriteConn{Conn: writer, writing: make(chan struct{})}
	f.peer.session.conn = connection
	f.peer.session.bufWriter = bufio.NewWriterSize(connection, 4096)
	work, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	route := f.route(t, 1, "192.0.2.79", 0, false)
	blocked := make(chan error, 1)
	go func() { _, err := f.send(t, work, route); blocked <- err }()
	select {
	case <-connection.writing:
	case err := <-blocked:
		t.Fatalf("A did not reach its held flush: %v", err)
	case <-work.Done():
		t.Fatal("A never reached its socket")
	}
	batch, err := plugin.NewSelectedBatch(rpc.SelectRoutesInput{Destination: other.addrString,
		DestinationSession: other.session.initialReplay.Load(), Routes: []rpc.SelectedRoute{route}})
	require.NoError(t, err)
	ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	answer, err := f.api.SelectRoutes(ctx, batch, f.owner)
	require.NoError(t, err, "B's writer must not wait for A's flush")
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	require.Len(t, updateBodies(t, otherWire.written()), 1)
	select {
	case err := <-blocked:
		t.Fatalf("A stopped blocking before its cancellation: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-blocked:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("A's canceled final writer did not join")
	}
}

// TestSelectedWriterRetainedRevocationJoinsBlockedFlush proves the original
// source lease can cancel and join a real write after physical source DOWN.
func TestSelectedWriterRetainedRevocationJoinsBlockedFlush(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	route := f.route(t, 1, "192.0.2.79", 0, false)
	sourceReceipts.mu.Lock()
	lease := sourceReceipts.entries[route.SourceReceipt].leases[f.owner]
	sourceReceipts.mu.Unlock()
	require.NoError(t, lease.retain())
	lease.record.facts.source.info.peer.state.Store(int32(PeerStateStopped))
	route.Retained, route.StaleLevel = true, rpc.SelectedStale
	writer, remote := net.Pipe()
	t.Cleanup(func() { _ = writer.Close(); _ = remote.Close() })
	connection := &observedWriteConn{Conn: writer, writing: make(chan struct{})}
	f.peer.session.conn = connection
	f.peer.session.bufWriter = bufio.NewWriterSize(connection, 4096)
	work, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := f.send(t, work, route); finished <- err }()
	select {
	case <-connection.writing:
	case err := <-finished:
		t.Fatalf("valid retained route failed before its flush: %v", err)
	case <-work.Done():
		t.Fatal("retained selection never reached its socket")
	}
	revoke, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	require.NoError(t, lease.revoke(revoke), "revocation must join the actual writer outcome")
	select {
	case err := <-finished:
		require.Error(t, err)
	default:
		// Completion can be delivered to the caller just after its guard's
		// final release, but another socket write cannot still be running.
		select {
		case err := <-finished:
			require.Error(t, err)
		case <-revoke.Done():
			t.Fatal("revocation left the selected request running")
		}
	}
	require.Empty(t, f.peer.adjOut.routes)
	_, err := sourceReceipts.admitWrite(route.SourceReceipt, f.owner, route.Route.SourcePolicyEpoch)
	require.Error(t, err, "an expired retained receipt cannot restore delayed output")
}
