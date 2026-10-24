// Design: docs/architecture/core-design.md -- reactor peer opQueue and the initial-sync gate.
// Related: reactor_api_batch.go -- AnnounceNLRIBatch, WithdrawNLRIBatch, queueAnnounceForPeer.
// Related: peer_queue_gate_test.go -- the peer-level refusal these rails act on.
// Overview: group_updates_framing_test.go -- the harness these tests borrow.
//
// TestQueueRefusedAfterGateOpens proves the queue refuses once the initial
// sync's final drain has opened the gate. It cannot see what the batch rails do
// with that refusal: a rail that dropped the refused NLRIs would leave it green.
// These tests drive the rails through the refusal and read the peer's wire.
package reactor

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// gateRailPrefixes is the batch both rails send. Three NLRIs let the drain take
// the first while the queue refuses the last two, so a rail that sent only one
// refused NLRI, or none, is told apart from one that sent them all.
var gateRailPrefixes = []netip.Prefix{
	netip.MustParsePrefix("10.20.0.0/24"),
	netip.MustParsePrefix("10.20.1.0/24"),
	netip.MustParsePrefix("10.20.2.0/24"),
}

// drainBetweenContext is the seam that replays the interleaving inside one rail
// call. The rails ask ctx.Err before each queue append, so the first Err that
// finds an operation already queued runs the initial sync's final drain there:
// the queued NLRI goes to the wire, the gate opens, and the rail's next append
// meets errQueueGateOpen. Err answers nil throughout, so the rail never stops on
// the context. Not safe for concurrent use: the rail calls it from one goroutine.
type drainBetweenContext struct {
	context.Context
	peer    *Peer
	drained bool
}

func (c *drainBetweenContext) Err() error {
	if c.drained {
		return nil
	}
	c.peer.mu.RLock()
	queued := len(c.peer.opQueue)
	c.peer.mu.RUnlock()
	if queued == 0 {
		return nil
	}
	c.drained = true
	c.peer.drainAndCloseQueueGate(c.peer.addrString, message.MaxMsgLen)
	return nil
}

// gateRailBatch is the three-prefix IPv4 unicast batch under test.
func gateRailBatch() bgptypes.NLRIBatch {
	nlris := make([]nlri.NLRI, 0, len(gateRailPrefixes))
	for _, prefix := range gateRailPrefixes {
		nlris = append(nlris, nlri.NewINET(family.IPv4Unicast, prefix, 0, false))
	}
	return bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   nlris,
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("1.1.1.1")),
	}
}

// wirePrefixes reads, in wire order, the prefixes every UPDATE in stream carries
// in its NLRI section, or in its Withdrawn Routes field when withdrawn is true.
func wirePrefixes(t *testing.T, stream []byte, withdrawn bool) []netip.Prefix {
	t.Helper()
	var prefixes []netip.Prefix
	for _, body := range updateBodies(t, stream) {
		prefixes = append(prefixes, updatePrefixes(t, body, withdrawn)...)
	}
	return prefixes
}

// requireGateSettled checks the state the refusal MUST leave: nothing parked
// behind the finished drain, and the gate open for the next operation.
func requireGateSettled(t *testing.T, peer *Peer) {
	t.Helper()
	peer.mu.RLock()
	parked := len(peer.opQueue)
	peer.mu.RUnlock()
	require.Zero(t, parked, "no operation may be parked behind a finished drain")
	require.False(t, peer.shouldQueue(), "the gate stays open after the final drain")
}

// TestAnnounceBatchSendsRefusedAfterGateOpens drives AnnounceNLRIBatch with the
// gate closed when the rail reads shouldQueue, and opens it after the first NLRI
// is queued (drainBetweenContext). The drain sends the first prefix; the queue
// refuses the other two, and the rail MUST send them directly, after it. The
// assertion reads the peer's connection, so a rail that counted the batch as
// accepted and dropped the refused NLRIs goes red. Both settings of update
// groups run, because a partial batch MUST leave the group path.
//
// VALIDATES: the announce rail sends batch.NLRIs[refused:] after a refusal.
// PREVENTS: a route counted as accepted that the peer never receives.
func TestAnnounceBatchSendsRefusedAfterGateOpens(t *testing.T) {
	for _, groups := range []bool{false, true} {
		name := "groups-off"
		if groups {
			name = "groups-on"
		}
		t.Run(name, func(t *testing.T) {
			peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
			adapter := groupUpdatesReactor([]*Peer{peer}, groups)
			peer.sendingInitialRoutes.Store(1)
			require.True(t, peer.shouldQueue(), "the rail reads the gate closed while the sync is owed")

			ctx := &drainBetweenContext{Context: t.Context(), peer: peer}
			require.NoError(t, adapter.AnnounceNLRIBatch(ctx, selector.All(), gateRailBatch(), plugin.OperatorSender()))
			require.True(t, ctx.drained, "the drain MUST run between the first queued NLRI and the next")

			require.Equal(t, gateRailPrefixes, wirePrefixes(t, conn.written(), false),
				"the drained prefix first, then every prefix the queue refused")
			requireGateSettled(t, peer)
		})
	}
}

// TestWithdrawBatchSendsRefusedAfterGateOpens is the withdraw rail's twin. The
// prefixes are first announced with the gate open, so the session has advertised
// a route and the withdrawal is not withheld (splitOnAdvertised). The gate then
// closes, and WithdrawNLRIBatch runs through the same interleaving: the drain
// sends the first withdrawal, and the rail MUST send the two the queue refused,
// after it.
//
// VALIDATES: the withdraw rail sends batch.NLRIs[refused:] after a refusal.
// PREVENTS: a withdrawal reported done while the peer keeps the route.
func TestWithdrawBatchSendsRefusedAfterGateOpens(t *testing.T) {
	for _, groups := range []bool{false, true} {
		name := "groups-off"
		if groups {
			name = "groups-on"
		}
		t.Run(name, func(t *testing.T) {
			peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
			adapter := groupUpdatesReactor([]*Peer{peer}, groups)
			require.False(t, peer.shouldQueue(), "the announce goes out directly")
			require.NoError(t, adapter.AnnounceNLRIBatch(t.Context(), selector.All(), gateRailBatch(), plugin.OperatorSender()))
			announced := len(conn.written())
			require.NotZero(t, announced, "the announce MUST reach the wire before the withdrawal")

			peer.sendingInitialRoutes.Store(1)
			require.True(t, peer.shouldQueue(), "the rail reads the gate closed while the sync is owed")

			ctx := &drainBetweenContext{Context: t.Context(), peer: peer}
			require.NoError(t, adapter.WithdrawNLRIBatch(ctx, selector.All(), gateRailBatch(), plugin.OperatorSender()))
			require.True(t, ctx.drained, "the drain MUST run between the first queued NLRI and the next")

			require.Equal(t, gateRailPrefixes, wirePrefixes(t, conn.written()[announced:], true),
				"the drained withdrawal first, then every withdrawal the queue refused")
			requireGateSettled(t, peer)
		})
	}
}
