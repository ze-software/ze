// Design: docs/architecture/api/process-protocol.md -- held original target and successor.
package reactor

import (
	"bufio"
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"sync"
	"testing"
	"time"
)

// This wrapper holds the final socket call AFTER the real recording transport
// accepted bytes. A target change must fail the old request even though neither
// its context nor its actual socket failed. It cannot pass on deadline alone.
type boundSuccessorHeldConn struct {
	*recordingConn
	accepted chan struct{}
	release  <-chan struct{}
}

func (wire *boundSuccessorHeldConn) Write(data []byte) (int, error) {
	count, err := wire.recordingConn.Write(data)
	close(wire.accepted)
	<-wire.release
	return count, err
}

func TestBoundRelayServerJoinsHeldOriginalBeforeTargetSuccessor(t *testing.T) {
	for _, socket := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "framed"}[socket], func(t *testing.T) {
			f := newBoundRelayWireFixture(t, false)
			work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			_, client, _ := selectedRefreshCleanupSDK(t, f.target, socket, work)
			original := f.target.peer.currentSession()
			input := rpc.RelayStoredRouteInput{Destination: f.target.peer.settings.Address.String(), DestinationOwner: f.target.peer.sourceOwner, DestinationSession: original.initialReplay.Load(), Routes: []rpc.StoredRoute{f.route}}
			require.NoError(t, client.RelayStoredRoute(work, input))
			require.Len(t, updateBodies(t, f.target.conn.written()), 1)
			release := make(chan struct{})
			var once sync.Once
			releaseHeld := func() { once.Do(func() { close(release) }) }
			t.Cleanup(releaseHeld)
			held := &boundSuccessorHeldConn{recordingConn: f.target.conn, accepted: make(chan struct{}), release: release}
			original.conn = held
			original.bufWriter = bufio.NewWriterSize(held, 4096)
			input.Routes[0].MsgID++
			result := make(chan error, 1)
			go func() { result <- client.RelayStoredRoute(work, input) }()
			select {
			case <-held.accepted:
			case err := <-result:
				t.Fatalf("old target returned before held flush: %v", err)
			case <-work.Done():
				t.Fatal("old target never reached final socket")
			}
			replacement, replacementConn := newGroupUpdatesPeer(t, f.target.peer.settings.Address.String(), "absent")
			successor := replacement.session
			successor.initialReplay.Store(original.initialReplay.Load() + 1)
			successor.sendCtxID = original.sendCtxID
			successor.egressFinalAccepts = original.egressFinalAccepts
			f.target.peer.mu.Lock()
			f.target.peer.session = successor
			f.target.peer.mu.Unlock()
			f.target.peer.establishedSession.Store(successor)
			f.target.peer.refreshForwardFacts()
			select {
			case err := <-result:
				t.Fatalf("old request settled while original writer held: %v", err)
			default:
			}
			require.Empty(t, replacementConn.written(), "old request cannot write into the successor")
			releaseHeld()
			select {
			case err := <-result:
				require.Error(t, err)
				require.Contains(t, err.Error(), errAdjOutSession.Error())
				_, joined := errors.AsType[*rpc.CachedWriteJoinedError](err)
				require.True(t, joined, "server attests join only after original actual writer returns")
			case <-work.Done():
				t.Fatal("old request did not join original writer")
			}
			require.NoError(t, work.Err(), "deadline expiration cannot satisfy this race")
			require.Len(t, updateBodies(t, f.target.conn.written()), 2)
			require.Empty(t, replacementConn.written())
			fresh := input
			fresh.DestinationSession = successor.initialReplay.Load()
			fresh.Routes = append([]rpc.StoredRoute(nil), input.Routes...)
			fresh.Routes[0].MsgID++
			require.NoError(t, client.RelayStoredRoute(work, fresh))
			require.Len(t, updateBodies(t, replacementConn.written()), 1, "fresh independently captured successor remains useful")
		})
	}
}

func TestBoundForwardMissingCapturedTargetNeverBecomesUnbound(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	stage := &boundForwardStage{request: &boundForwardRequest{ctx: t.Context(), owner: f.target.owner, resolved: make(map[*Peer]plugin.Sender)}}
	require.Error(t, stage.current(f.target.peer, f.target.peer.currentSession()), "missing original target is refused before source guard access")
}
