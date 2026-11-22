// Design: docs/architecture/api/process-protocol.md -- every admitted carrier joins before cache release.
package reactor

import (
	"bufio"
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type boundPanicConn struct{ *recordingConn }

func (*boundPanicConn) Write([]byte) (int, error) { panic("injected actual target writer panic") }

// A panicking socket cannot bypass the other admitted socket's actual final
// flush. A healthy third captured target remains useful on both SDK rails.
func TestBoundCachedPanicStillJoinsEveryOtherFinalWriter(t *testing.T) {
	for _, socket := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "framed"}[socket], func(t *testing.T) {
			f := newBoundRelayWireFixture(t, false)
			hostWork, stopHost := context.WithTimeout(t.Context(), 15*time.Second)
			defer stopHost()
			work, cancel := context.WithTimeout(hostWork, 15*time.Second)
			defer cancel()
			_, client, proc := selectedRefreshCleanupSDK(t, f.target, socket, hostWork)
			heldPeer, _ := boundRelaySecondTarget(t, f, "192.0.2.186", 2179)
			usefulPeer, usefulConn := boundRelaySecondTarget(t, f, "192.0.2.187", 3179)
			panicWire := &boundPanicConn{f.target.conn}
			f.target.peer.session.conn = panicWire
			f.target.peer.session.bufWriter = bufio.NewWriterSize(panicWire, 4096)
			left, right := net.Pipe()
			t.Cleanup(func() { _ = left.Close(); _ = right.Close() })
			release := make(chan struct{})
			var once sync.Once
			finish := func() { once.Do(func() { close(release) }) }
			t.Cleanup(finish)
			held := &selectedRefreshHeldConn{Conn: left, writing: make(chan struct{}), interrupted: make(chan struct{}), release: release}
			heldPeer.session.conn = held
			heldPeer.session.bufWriter = bufio.NewWriterSize(held, 4096)
			useful := &boundNotifyConn{recordingConn: usefulConn, written: make(chan struct{})}
			usefulPeer.session.conn = useful
			usefulPeer.session.bufWriter = bufio.NewWriterSize(useful, 4096)
			source, err := f.target.api.originalRelayFacts(f.source.settings.Address, &f.route, f.sender)
			require.NoError(t, err)
			cache := f.target.api.r.recentUpdates
			cache.RegisterConsumer(proc.Name())
			defer cache.UnregisterConsumer(proc.Name())
			cache.mu.RLock()
			floor := cache.pluginLastAck[proc.Name()]
			cache.mu.RUnlock()
			var spans []relayAttrSpan
			_, id, _, err := f.target.api.buildRelayUpdate([]rpc.StoredRoute{f.route}, &source, &spans)
			require.NoError(t, err)
			defer cache.Release(id)
			targets := make([]rpc.ForwardTarget, 0, 3)
			for _, peer := range []*Peer{f.target.peer, heldPeer, usefulPeer} {
				targets = append(targets, rpc.ForwardTarget{Destination: peer.settings.PeerKey().String(), DestinationOwner: peer.sourceOwner, DestinationSession: peer.session.initialReplay.Load()})
			}
			result := make(chan error, 1)
			go func() {
				result <- client.ForwardCached(work, rpc.ForwardCachedInput{IDs: []uint64{id}, Targets: targets})
			}()
			select {
			case <-held.writing:
			case err := <-result:
				t.Fatalf("returned before other final flush: %v", err)
			case <-work.Done():
				t.Fatal("held writer never started")
			}
			select {
			case <-useful.written:
			case <-work.Done():
				t.Fatal("panic or held target blocked healthy third target")
			}
			require.Len(t, updateBodies(t, usefulConn.written()), 1)
			cache.mu.RLock()
			acked := cache.pluginLastAck[proc.Name()]
			cache.mu.RUnlock()
			require.Equal(t, floor, acked)
			select {
			case err := <-result:
				t.Fatalf("panicking target skipped still-admitted final writer: %v", err)
			default:
			}
			cancel()
			select {
			case <-held.interrupted:
			case <-time.After(5 * time.Second):
				t.Fatal("cancel did not reach held socket")
			}
			select {
			case err := <-result:
				t.Fatalf("returned while interrupted final writer still owned bytes: %v", err)
			default:
			}
			finish()
			select {
			case err := <-result:
				require.Error(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("final carrier failed to settle")
			}
			cache.mu.RLock()
			acked = cache.pluginLastAck[proc.Name()]
			entry, ok := cache.entries.Get(id)
			cache.mu.RUnlock()
			require.Equal(t, id, acked)
			require.True(t, ok)
			require.Equal(t, int32(1), entry.retainCount.Load())
		})
	}
}
