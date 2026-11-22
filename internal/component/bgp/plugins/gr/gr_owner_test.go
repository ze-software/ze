// Design: docs/guide/graceful-restart.md -- original-owner GR state.
package gr

import (
	"encoding/json"
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Two actual event headers sharing an address retain independent capabilities,
// timer cycles and source-scoped command targets on both receiving rails.
func TestGROwnerEventsKeepSameAddressSibling(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "typed"}[typed], func(t *testing.T) {
			a, err := newGRSource("192.0.2.240", math.MaxUint64-1, math.MaxUint64-1)
			require.NoError(t, err)
			b, err := newGRSource("192.0.2.240", math.MaxUint64, math.MaxUint64)
			require.NoError(t, err)
			gp := &grPlugin{peerCaps: make(map[uint64]*grPeerCap), peerLLGRCaps: make(map[uint64]*llgrPeerCap)}
			gp.wireStateCallbacks()
			t.Cleanup(func() { gp.state.removePeer(a.grPeer); gp.state.removePeer(b.grPeer) })
			cap := &grPeerCap{connectionID: a.connectionID, RestartTime: 3600, Families: []grCapFamily{{Family: family.IPv4Unicast, ForwardState: true}}}
			gp.peerCaps[a.owner] = cap
			siblingCap := &grPeerCap{connectionID: b.connectionID, RestartTime: cap.RestartTime, Families: cap.Families}
			gp.peerCaps[b.owner] = siblingCap
			gp.recordSentLLGR(a, nil)
			gp.recordSentLLGR(b, nil)
			var commands []string
			gp.dispatchHook = func(command string, args ...string) {
				require.Contains(t, command, "request bgp rib source ")
				require.GreaterOrEqual(t, len(args), 2)
				require.Equal(t, "owner", args[0])
				require.Equal(t, strconv.FormatUint(a.owner, 10), args[1], "only A is changing state")
				commands = append(commands, command)
			}
			var ledger retention.Ledger
			require.NoError(t, ledger.Begin(a.owner, a.connectionID))
			require.NoError(t, ledger.Begin(b.owner, b.connectionID))
			gp.emitHook = func(payload string) {
				var verdict retention.Verdict
				require.NoError(t, json.Unmarshal([]byte(payload), &verdict))
				require.Equal(t, a.owner, verdict.PeerOwner, "the actual GR emitter preserves the original full-width owner")
				require.NoError(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(payload)}))
			}
			deliver := func(source grSource, state rpc.SessionState, reason string) {
				if typed {
					require.NoError(t, gp.handleStructuredEvent(&rpc.StructuredEvent{EventType: rpc.EventKindState, PeerAddress: source.peer(), PeerOwner: source.owner, ConnectionID: source.connectionID, State: state, Reason: reason}))
					return
				}
				body, err := json.Marshal(map[string]any{"type": "bgp", "bgp": map[string]any{"peer-owner": source.owner, "connection-id": strconv.FormatUint(source.connectionID, 10), "peer": map[string]any{"remote": map[string]any{"address": source.peer()}}, "message": map[string]any{"type": "state"}, "state": state.String(), "reason": reason}})
				require.NoError(t, err)
				require.NoError(t, gp.handleEvent(string(body)))
			}
			require.True(t, gp.state.onSessionDown(b, siblingCap, nil, false))
			sibling := gp.state.peers[b.owner]
			deliver(a, rpc.SessionStateDown, rpc.ReasonTCPFailure)
			require.NotEmpty(t, commands)
			require.True(t, gp.state.peerActive(a))
			kept, err := ledger.Take(a.owner, a.connectionID)
			require.NoError(t, err)
			require.True(t, kept[family.IPv4Unicast], "the actual GR emit reaches its original admitted owner")
			keptSibling, err := ledger.Take(b.owner, b.connectionID)
			require.NoError(t, err)
			require.Empty(t, keptSibling, "A's emitted verdict cannot become B's decision")
			require.Same(t, sibling, gp.state.peers[b.owner])
			require.True(t, gp.state.peerActive(b))
			deliver(a, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
			require.False(t, gp.state.peerActive(a))
			require.Same(t, sibling, gp.state.peers[b.owner])
			require.Same(t, siblingCap, gp.peerCaps[b.owner])
			require.NotContains(t, gp.peerCaps, a.owner)
			before := len(commands)
			deliver(a, rpc.SessionStateDown, rpc.ReasonTCPFailure)
			require.Len(t, commands, before, "a removed owner's late DOWN cannot borrow its sibling's capability")
			require.Same(t, sibling, gp.state.peers[b.owner])
		})
	}
}

// Timer.Stop cannot retract an already running callback. Hold that captured
// callback for a family whose LLST timer is stopped on reconnect (purged
// because the new LLGR capability omits it while a sibling family keeps the
// peer state alive), run the actual reconnect transition, then release its old
// body. Under RFC 9494 Section 4.2, surviving families keep their LLST timers
// running until EOR, whereas purged families have their timers stopped on
// reconnect; a queued callback from a stopped timer must be a no-op.
// MUTATION: omit the active-family LLST callback check after reconnect.
func TestGRReconnectRevokesQueuedLLSTCallback(t *testing.T) {
	source, err := newGRSource("192.0.2.241", 1231, 2231)
	require.NoError(t, err)
	manager := newGRStateManager(nil)
	var expiredCalls []family.Family
	manager.onLLGRFamilyExpired = func(_ grSource, fam family.Family) {
		expiredCalls = append(expiredCalls, fam)
	}
	gr := &grPeerCap{
		connectionID: source.connectionID,
		Families: []grCapFamily{
			{Family: family.IPv4Unicast, ForwardState: true},
			{Family: family.IPv6Unicast, ForwardState: true},
		},
	}
	llgr := &llgrPeerCap{
		connectionID: source.connectionID,
		Families: []llgrCapFamily{
			{Family: family.IPv4Unicast, ForwardState: true, LLST: 3600},
			{Family: family.IPv6Unicast, ForwardState: true, LLST: 3600},
		},
	}
	require.True(t, manager.onSessionDown(source, gr, llgr, false))
	state := manager.peers[source.owner]
	require.Contains(t, state.llgrFamilies, family.IPv4Unicast)
	require.Contains(t, state.llgrFamilies, family.IPv6Unicast)
	t.Cleanup(func() { manager.removePeer(source.grPeer) })
	entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }); <-joined })
	// A one-shot test timer holds the exact production callback's captured cycle
	// for ipv4/unicast, which reconnect will purge and stop.
	time.AfterFunc(0, func() {
		close(entered)
		<-release
		manager.handleLLSTExpired(source, family.IPv4Unicast, state)
		close(joined)
	})
	select {
	case <-entered:
	case <-t.Context().Done():
		t.Fatal("captured LLST callback did not enter")
	}
	fresh, err := newGRSource(source.peer(), source.owner, 2232)
	require.NoError(t, err)
	successorGR := &grPeerCap{connectionID: fresh.connectionID, Families: gr.Families}
	// Omit ipv4/unicast from successor LLGR so reconnect stops its LLST timer
	// and purges it while keeping ipv6/unicast (and its running LLST timer)
	// retained on the peer state per RFC 9494 Section 4.2.
	successorLLGR := &llgrPeerCap{
		connectionID: fresh.connectionID,
		Families:     []llgrCapFamily{{Family: family.IPv6Unicast, ForwardState: true, LLST: 3600}},
	}
	_, purged, _ := manager.onSessionReestablished(fresh, successorGR, successorLLGR)
	require.Equal(t, []family.Family{family.IPv4Unicast}, purged)
	require.NotContains(t, state.llgrFamilies, family.IPv4Unicast)
	require.Contains(t, state.llgrFamilies, family.IPv6Unicast)
	once.Do(func() { close(release) })
	select {
	case <-joined:
	case <-t.Context().Done():
		t.Fatal("captured LLST callback did not join")
	}
	require.Empty(t, expiredCalls, "a canceled old timer cannot fire LLGR family expiry after reconnect")
	require.True(t, manager.peerActive(source), "a canceled old timer cannot discard the retained reconnect survivor")
	require.Equal(t, []family.Family{family.IPv6Unicast}, manager.retainedFamilies(source))
}
