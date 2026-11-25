package rr

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRRLiveForwardDoesNotBlockDeliveryLoopDuringPeerReplay proves that live
// UPDATE reflection does not block deliveryLoop while a destination peer is
// awaiting peer-UP readiness (reactor forwardOrderHold active), preserves per-
// source FIFO order across subsequent UPDATEs, and still keeps rr.forwards
// enrolled until the async ForwardCached writes complete.
func TestRRLiveForwardDoesNotBlockDeliveryLoopDuringPeerReplay(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { require.NoError(t, engineEnd.Close()) })
	p := sdk.NewWithConn("rr-async-forward", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { require.NoError(t, p.Close()) })

	firstEntered := make(chan struct{})
	releaseHold := make(chan struct{})
	var mu sync.Mutex
	var forwardedIDs []uint64
	bridge.SetForwardCached(func(_ context.Context, input rpc.ForwardCachedInput) (*rpc.CachedWriteOutput, error) {
		if len(input.IDs) == 1 && input.IDs[0] == 101 {
			close(firstEntered)
			<-releaseHold
		}
		mu.Lock()
		forwardedIDs = append(forwardedIDs, input.IDs...)
		mu.Unlock()
		return rpc.CompletedCachedWrite(input.DeadlineMS, nil), nil
	})
	rrFixtureLease(bridge)
	bridge.SetReady()

	downCtx, stopDown := context.WithCancel(context.Background())
	rr := &routeReflector{
		plugin:      p,
		peers:       make(map[peerIdentity]*peerState),
		withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo),
		downCtx:     downCtx,
	}
	rr.forwardDispatch = newRRForwardDispatcher(rr.runForwardTask)
	t.Cleanup(func() {
		stopDown()
		rr.forwardDispatch.stop()
	})

	source := rrOriginalFixture(t, "192.0.2.1", 701, 41)
	dest := rrOriginalFixture(t, "192.0.2.2", 702, 42)
	rrFixtureStart(t, rr, source, 70, 7, 141)
	rrFixtureStart(t, rr, dest, 0, 7, 142)
	rr.mu.Lock()
	rr.peers[dest].awaitingSessionReady = true
	rr.mu.Unlock()

	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, nil))
	require.NoError(t, err)

	makeEvent := func(msgID uint64, lastOctet byte) (*rpc.StructuredEvent, *bgptypes.RawMessage) {
		body := []byte{
			0, 0, // withdrawn len = 0
			0, 4, // total path attr len = 4
			0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
			24, 10, 0, lastOctet, // NLRI 10.0.<lastOctet>.0/24
		}
		wu := wireu.NewWireUpdate(body, ctxID)
		attrsWire, err := wu.Attrs()
		require.NoError(t, err)
		se := &rpc.StructuredEvent{
			EventType:     rpc.EventKindUpdate,
			PeerAddress:   "192.0.2.1",
			PeerOwner:     701,
			ConnectionID:  41,
			SourceReceipt: 70,
		}
		msg := &bgptypes.RawMessage{
			MessageID:     msgID,
			SourceOwner:   701,
			SourceReceipt: 70,
			RawBytes:      body,
			AttrsWire:     attrsWire,
			WireUpdate:    wu,
		}
		return se, msg
	}

	// First UPDATE must return immediately even while ForwardCached blocks on dest's hold.
	se1, msg1 := makeEvent(101, 1)
	require.NoError(t, rr.reflectWire(se1, msg1))
	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("async ForwardCached worker did not start for msg 101")
	}

	// Even after awaitingSessionReady clears, a second UPDATE from the same source
	// must stay queued behind msg 101 to preserve strict per-source FIFO order.
	rr.mu.Lock()
	rr.peers[dest].awaitingSessionReady = false
	rr.mu.Unlock()
	se2, msg2 := makeEvent(102, 2)
	require.NoError(t, rr.reflectWire(se2, msg2))

	close(releaseHold)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	require.NoError(t, rr.forwards.wait(ctx))

	mu.Lock()
	got := append([]uint64(nil), forwardedIDs...)
	mu.Unlock()
	require.Equal(t, []uint64{101, 102}, got, "per-source FIFO order must be preserved across async/sync transition")
}

// TestRRReplayRetriesWhenSourceDepartsMidPage verifies that a joined
// RelayStoredRoute error caused by another source disconnecting mid-page does
// not poison live sources' sourceOutputDebt and allows replayForPeer to retry
// and complete signalSessionReady.
func TestRRReplayRetriesWhenSourceDepartsMidPage(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { require.NoError(t, engineEnd.Close()) })
	p := sdk.NewWithConn("rr-replay-departed-retry", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { require.NoError(t, p.Close()) })

	liveSource := rrOriginalFixture(t, "192.0.2.1", 701, 41)
	departingSource := rrOriginalFixture(t, "192.0.2.3", 703, 43)
	recipient := rrOriginalFixture(t, "192.0.2.2", 702, 42)

	liveRow := rpc.StoredRoute{
		SourcePeer: "192.0.2.1", SourceOwner: 701, SourceConnectionID: 41, SourceReceipt: 70,
		MsgID: 80, SourcePolicyEpoch: 5, Family: "ipv4/unicast", NLRIHex: "180a0000",
		NLRIFraming: rpc.NLRIFramingPrefixOnly, InitialUpdate: true,
	}
	departingRow := rpc.StoredRoute{
		SourcePeer: "192.0.2.3", SourceOwner: 703, SourceConnectionID: 43, SourceReceipt: 73,
		MsgID: 81, SourcePolicyEpoch: 5, Family: "ipv4/unicast", NLRIHex: "180a0001",
		NLRIFraming: rpc.NLRIFramingPrefixOnly, InitialUpdate: true,
	}

	var rr *routeReflector
	var relayCalls atomic.Int32
	var readyCalls atomic.Int32
	var eorCalls atomic.Int32

	bridge.SetDispatchCommandArgs(func(_ context.Context, command string, _ []string, _ string) (*rpc.DispatchCommandOutput, error) {
		if command == "test-current" {
			return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(`{"answers":[true]}`)}, nil
		}
		routes := []rpc.StoredRoute{liveRow, departingRow}
		if relayCalls.Load() > 0 {
			routes = []rpc.StoredRoute{liveRow}
		}
		data, err := json.Marshal(ribevents.ReplayPage{Routes: routes})
		return &rpc.DispatchCommandOutput{Status: statusDone, Data: data}, err
	})
	bridge.SetRelayStoredRoute(func(_ context.Context, input rpc.RelayStoredRouteInput) (*rpc.CachedWriteOutput, error) {
		if relayCalls.Add(1) == 1 {
			rr.mu.Lock()
			rr.peers[departingSource].Up = false
			rr.peers[departingSource].DownSeen = true
			rr.mu.Unlock()
			return rpc.CompletedCachedWrite(input.DeadlineMS, errors.New("original cold relay source ended without retained membership")), nil
		}
		require.Equal(t, []rpc.StoredRoute{liveRow}, input.Routes)
		return rpc.CompletedCachedWrite(input.DeadlineMS, nil), nil
	})
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if result, handled, err := rrFixtureLeaseRPC(method, params); handled {
			return result, err
		}
		if method == rpc.MethodUpdateRoute {
			eorCalls.Add(1)
			return json.Marshal(rpc.UpdateRouteOutput{Announced: 1})
		}
		var input rpc.DispatchCommandInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		readyCalls.Add(1)
		return json.Marshal(rpc.DispatchCommandOutput{
			Status: statusDone, DestinationOwner: input.DestinationOwner,
			DestinationSession: input.DestinationSession, DeadlineMS: input.DeadlineMS,
		})
	})
	bridge.SetReady()

	rr = &routeReflector{
		plugin:      p,
		peers:       make(map[peerIdentity]*peerState),
		withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo),
		receiveGate: "test-current",
	}
	rrFixtureStart(t, rr, liveSource, 70, 7, 141)
	rrFixtureStart(t, rr, departingSource, 73, 7, 143)
	rrFixtureStart(t, rr, recipient, 0, 7, 142)
	rr.peers[recipient].Families = map[family.Family]bool{family.IPv4Unicast: true}
	rr.peers[recipient].ReplayGen = 1
	rr.peers[recipient].awaitingSessionReady = true

	target := peerTarget{identity: recipient, session: 142, generation: 1}
	rr.replayForPeer(t.Context(), target, 17, make(chan struct{}))

	require.Equal(t, int32(2), relayCalls.Load(), "replayForPeer must retry after a joined departed-source failure")
	require.Equal(t, int32(1), eorCalls.Load(), "replayForPeer must send EOR after retry succeeds")
	require.Equal(t, int32(1), readyCalls.Load(), "replayForPeer must signal session ready after retry succeeds")
	require.False(t, rr.peers[recipient].awaitingSessionReady, "awaitingSessionReady must clear after signalSessionReady succeeds")
	require.Nil(t, rr.peers[recipient].replayDebt)
	require.NoError(t, rr.unsettledWork(), "joined departed-source error must not poison live sources' sourceOutputDebt")
}
