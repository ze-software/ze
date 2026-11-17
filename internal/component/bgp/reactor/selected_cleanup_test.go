// Design: docs/architecture/update-building.md -- exact selecting-owner retirement.
// Related: register_source_lease.go -- process-generation teardown barrier.
package reactor

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func selectedCleanupProcess(t *testing.T, f *selectedWriterFixture, makeProcess func(string) *process.Process) *process.Process {
	t.Helper()
	proc := makeProcess(selectedWriterProcess)
	proc.Bridge().SetCurrentSelection(func(_ context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		answer := &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession,
			Cut: f.cut, Results: make([]rpc.CurrentSelectionResult, len(input.Keys))}
		for i, key := range input.Keys {
			result := rpc.CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI, Revision: key.Revision, Presence: rpc.CurrentSelectionAbsent}
			if revision := f.current[string(key.KeyNLRI)]; revision != 0 {
				result.Presence, result.CurrentRevision = rpc.CurrentSelectionPresent, revision
			}
			answer.Results[i] = result
		}
		return answer, nil
	})
	proc.Bridge().SetReady()
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	f.owner = owner
	return proc
}

// TestSelectedWriterStoppedOwnerRetiresOnlyItsCurrentOutput sends useful wire
// output from two actual same-name process incarnations. A successor's absence
// cannot retire its predecessor; the engine teardown removes only the ended
// owner's output without asking its dead connection or adopting its identity.
func TestSelectedWriterStoppedOwnerRetiresOnlyItsCurrentOutput(t *testing.T) {
	makeProcess := sourceLeaseProcessFactory(t)
	f := newSelectedWriterFixture(t, false)
	old := selectedCleanupProcess(t, f, makeProcess)
	left := f.route(t, 1, "192.0.2.79", 0, false)
	answer, err := f.send(t, t.Context(), left)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	oldOwner := f.owner
	selectedCleanupProcess(t, f, makeProcess)
	right := f.route(t, 2, "192.0.2.79", 0, false)
	right.KeyNLRI[3] = 8
	right.Route.NLRIHex = "18c61208"
	answer, err = f.send(t, t.Context(), right)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	old.Stop()
	require.Error(t, oldOwner.Current())
	absent := rpc.SelectedRoute{Family: left.Family, KeyNLRI: left.KeyNLRI, Revision: 3, Kind: rpc.SelectedRouteAbsent}
	answer, err = f.send(t, t.Context(), absent)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedAbsent, answer.Results[0].Outcome)
	require.Len(t, updateBodies(t, f.conn.written()), 2)
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, f.api)
	require.NoError(t, err)
	sourceGenerationCleanup(host, old)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 3)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[2], true))
	require.Len(t, f.peer.adjOut.routes, 1)
	_, key, err := selectedNativeKey(right)
	require.NoError(t, err)
	_, present := f.peer.adjOut.selectedPaths[key]
	require.True(t, present)
	require.NoError(t, f.owner.Current())
}

// TestSelectedWriterRevokedSourceCleanupPreservesOtherSource proves a grant
// cut retires only that exact source's current output, including retained
// output whose physical source is already gone.
func TestSelectedWriterRevokedSourceCleanupPreservesOtherSource(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	left := f.route(t, 1, "192.0.2.79", 0, false)
	answer, err := f.send(t, t.Context(), left)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	right := f.route(t, 2, "192.0.2.79", 0, false)
	right.KeyNLRI[3] = 8
	right.Route.NLRIHex = "18c61208"
	answer, err = f.send(t, t.Context(), right)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	sourceReceipts.mu.Lock()
	lease := sourceReceipts.entries[left.SourceReceipt].leases[f.owner]
	sourceReceipts.mu.Unlock()
	source := lease.record.facts.source.info.peer
	require.NoError(t, lease.retain())
	source.advanceForwardGeneration()
	source.state.Store(int32(PeerStateStopped))
	require.NoError(t, sourceReceipts.revokePeerOwner(t.Context(), source, f.owner))
	require.NoError(t, f.api.r.retireSelectedOwner(t.Context(), f.owner, source))
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 3)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[2], true))
	require.Len(t, f.peer.adjOut.routes, 1)
	_, key, err := selectedNativeKey(right)
	require.NoError(t, err)
	_, present := f.peer.adjOut.selectedPaths[key]
	require.True(t, present)
}

// TestSelectedWriterCleanupCancellationRetiresDestination holds the removal's
// actual socket flush. Cancellation must join that write and retire the exact
// destination, leaving no advertised frontier for a replacement to inherit.
func TestSelectedWriterCleanupCancellationRetiresDestination(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	route := f.route(t, 1, "192.0.2.79", 0, false)
	answer, err := f.send(t, t.Context(), route)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	require.Len(t, updateBodies(t, f.conn.written()), 1)
	writer, remote := net.Pipe()
	t.Cleanup(func() { _ = writer.Close(); _ = remote.Close() })
	connection := &observedWriteConn{Conn: writer, writing: make(chan struct{})}
	f.peer.session.conn = connection
	f.peer.session.bufWriter = bufio.NewWriterSize(connection, 4096)
	work, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- f.api.r.retireSelectedOwner(work, f.owner, nil) }()
	select {
	case <-connection.writing:
	case err := <-finished:
		t.Fatalf("cleanup did not reach its held withdrawal flush: %v", err)
	case <-work.Done():
		t.Fatal("cleanup never reached its destination socket")
	}
	cancel()
	select {
	case err := <-finished:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("canceled selected cleanup did not join its destination writer")
	}
	require.True(t, f.peer.session.tearingDown.Load())
	require.Empty(t, f.peer.adjOut.routes)
	require.Empty(t, f.peer.adjOut.selectedPaths)
}
