// Design: docs/architecture/api/process-protocol.md -- all target joins before one cache ACK.
package reactor

import (
	"bufio"
	"context"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net"
	"sync"
	"testing"
	"time"
)

type boundNotifyConn struct {
	*recordingConn
	writtenOnce sync.Once
	written     chan struct{}
}

func (wire *boundNotifyConn) Write(data []byte) (int, error) {
	count, err := wire.recordingConn.Write(data)
	if err == nil {
		wire.writtenOnce.Do(func() { close(wire.written) })
	}
	return count, err
}
func TestBoundCachedServerHeldTargetKeepsSiblingUsefulBeforeAck(t *testing.T) {
	for _, socket := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "framed"}[socket], func(t *testing.T) {
			f := newBoundRelayWireFixture(t, false)
			work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			_, client, proc := selectedRefreshCleanupSDK(t, f.target, socket, work)
			sibling, siblingConn := boundRelaySecondTarget(t, f, f.target.peer.settings.Address.String(), 2179)
			cache := f.target.api.r.recentUpdates
			cache.RegisterConsumer(proc.Name())
			defer cache.UnregisterConsumer(proc.Name())
			cache.mu.RLock()
			floor := cache.pluginLastAck[proc.Name()]
			cache.mu.RUnlock()
			source, err := f.target.api.originalRelayFacts(f.source.settings.Address, &f.route, f.sender)
			require.NoError(t, err)
			var spans []relayAttrSpan
			_, id, _, err := f.target.api.buildRelayUpdate([]rpc.StoredRoute{f.route}, source, &spans)
			require.NoError(t, err)
			defer cache.Release(id)
			left, right := net.Pipe()
			release := make(chan struct{})
			var releaseOnce sync.Once
			releaseHeld := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(releaseHeld)
			held := &selectedRefreshHeldConn{Conn: left, writing: make(chan struct{}), interrupted: make(chan struct{}), release: release}
			t.Cleanup(func() { _ = left.Close(); _ = right.Close() })
			f.target.peer.session.conn = held
			f.target.peer.session.bufWriter = bufio.NewWriterSize(held, 4096)
			useful := &boundNotifyConn{recordingConn: siblingConn, written: make(chan struct{})}
			sibling.session.conn = useful
			sibling.session.bufWriter = bufio.NewWriterSize(useful, 4096)
			targets := []rpc.ForwardTarget{
				{Destination: f.target.peer.settings.PeerKey().String(), DestinationOwner: f.target.peer.sourceOwner, DestinationSession: f.target.peer.session.initialReplay.Load()},
				{Destination: sibling.settings.PeerKey().String(), DestinationOwner: sibling.sourceOwner, DestinationSession: sibling.session.initialReplay.Load()},
			}
			result := make(chan error, 1)
			go func() {
				result <- client.ForwardCached(work, rpc.ForwardCachedInput{IDs: []uint64{id, id}, Targets: targets})
			}()
			select {
			case <-held.writing:
			case err := <-result:
				t.Fatalf("cache operation returned before held A: %v", err)
			case <-work.Done():
				t.Fatal("A did not reach its actual flush")
			}
			select {
			case <-useful.written:
			case <-work.Done():
				t.Fatal("held A prevented useful B from writing")
			}
			require.Len(t, updateBodies(t, siblingConn.written()), 1)
			cache.mu.RLock()
			beforeAck := cache.pluginLastAck[proc.Name()]
			cache.mu.RUnlock()
			require.Equal(t, floor, beforeAck, "no cumulative ACK before every original target joins")
			cancel()
			select {
			case <-held.interrupted:
			case <-time.After(5 * time.Second):
				t.Fatal("cancellation did not reach A's socket")
			}
			select {
			case err := <-result:
				t.Fatalf("caller returned while final writer held: %v", err)
			default:
			}
			releaseHeld()
			select {
			case err := <-result:
				require.Error(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("cache caller did not join A")
			}
			cache.mu.RLock()
			afterAck := cache.pluginLastAck[proc.Name()]
			entry, found := cache.entries.Get(id)
			cache.mu.RUnlock()
			require.Equal(t, id, afterAck)
			require.True(t, found)
			require.Equal(t, int32(1), entry.retainCount.Load(), "only the explicit build-time buffer loan remains")
			require.Len(t, updateBodies(t, siblingConn.written()), 1, "B was written once despite duplicate input IDs and A cancellation")
		})
	}
}

func TestBoundCachedPartialFailureStillWritesHealthyCapturedTarget(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	sibling, siblingConn := boundRelaySecondTarget(t, f, f.target.peer.settings.Address.String(), 2179)
	source, err := f.target.api.originalRelayFacts(f.source.settings.Address, &f.route, f.sender)
	require.NoError(t, err)
	var spans []relayAttrSpan
	_, id, _, err := f.target.api.buildRelayUpdate([]rpc.StoredRoute{f.route}, source, &spans)
	require.NoError(t, err)
	defer f.target.api.r.recentUpdates.Release(id)
	targets := []rpc.ForwardTarget{
		{Destination: f.target.peer.settings.PeerKey().String(), DestinationOwner: f.target.peer.sourceOwner, DestinationSession: f.target.peer.session.initialReplay.Load() + 1},
		{Destination: sibling.settings.PeerKey().String(), DestinationOwner: sibling.sourceOwner, DestinationSession: sibling.session.initialReplay.Load()},
	}
	require.Error(t, f.target.api.ForwardUpdatesBound(t.Context(), []uint64{id}, targets, "", f.target.owner.Sender(), f.target.owner))
	require.Empty(t, f.target.conn.written())
	require.Len(t, updateBodies(t, siblingConn.written()), 1, "stale A does not suppress healthy B")
}

func TestBoundStoredRetirementKeepsOtherEVPNNativeKey(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	f.target.peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{family.IPv4Unicast: true, fam: true}})
	f.target.peer.session.negotiated = forwardFixtureNegotiation(f.target.peer)
	f.target.peer.refreshForwardFacts()
	native := make([]byte, 27)
	native[0], native[1], native[26] = 1, 25, 1
	first := f.route
	first.Family = fam.String()
	first.NLRIHex = hex.EncodeToString(native)
	second := first
	native[9] = 1
	second.NLRIHex = hex.EncodeToString(native)
	second.MsgID++
	require.NoError(t, f.relay(t, first))
	require.NoError(t, f.relay(t, second))
	require.Len(t, f.target.peer.adjOut.routes, 2)
	f.source.advanceSourcePolicyEpoch()
	first.Withdraw = true
	require.NoError(t, f.relay(t, first))
	bodies := updateBodies(t, f.target.conn.written())
	require.Len(t, bodies, 3)
	withdrawn, present := bodyPathAttr(t, bodies[2], 15)
	require.True(t, present)
	expected, err := hex.DecodeString(first.NLRIHex)
	require.NoError(t, err)
	require.Equal(t, expected, withdrawn[3:], "retirement uses the current exact native outgoing bytes")
	require.Len(t, f.target.peer.adjOut.routes, 1)
	require.NoError(t, f.relay(t, first))
	require.Len(t, updateBodies(t, f.target.conn.written()), 3, "exact repeat cannot erase the surviving RD")
}
