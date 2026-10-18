// Design: docs/architecture/meta/README.md -- route metadata on a received event
// RFC: rfc/short/rfc9494.md -- Section 4.3, a received LLGR_STALE route is not advertised onward
// Related: reactor_notify.go -- notifyMessageReceiver, the ingress meta stored on the cached ReceivedUpdate
// Related: reactor_api_forward.go -- forwardUpdateSection, the general rail's egress pass
// Related: forward_rs.go -- reactorForwardRSSection, the route-server rail's egress pass

package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// llgrForwardPrefix is the NLRI the received UPDATE announces: 192.0.2.0/24.
var llgrForwardPrefix = []byte{24, 192, 0, 2}

// llgrStaleEgressStep stands in for the GR plugin's LLGREgressFilter, which the
// reactor may not import, toward a destination that never sent the LLGR
// Capability: it records the stale level the forward rail handed it, and at level
// 2 and above it asks for the route's withdrawal the way that filter does, by
// accepting it with SetWithdraw rather than refusing it.
func llgrStaleEgressStep(mu *sync.Mutex, seen *[]any) filterapi.EgressFilterFunc {
	return func(_, _ filterapi.PeerFilterInfo, _ []byte, meta map[string]any, mods *filterapi.ModAccumulator) bool {
		mu.Lock()
		defer mu.Unlock()
		stale := meta["stale"]
		*seen = append(*seen, stale)
		if level, isLevel := stale.(uint8); isLevel && level >= 2 {
			mods.SetWithdraw()
		}
		return true
	}
}

// llgrForwardRun is what one received-then-forwarded UPDATE produced: the stale
// level each egress call was handed, and every message the destination was asked
// to write.
type llgrForwardRun struct {
	stepSaw []any
	written []mixedMsg
}

// receiveAndForward receives one UPDATE carrying LLGR_STALE through the reactor's
// real receive path, from a source whose own OPEN did (localLLGR true) or did not
// carry the LLGR Capability, and forwards it to one external destination. With rs
// false the general rail forwards it from the cache, the way a plugin's forward
// command does; with rs true the route-server fast path forwards it from the
// cache inside notifyMessageReceiver.
func receiveAndForward(t *testing.T, localLLGR, rs bool) llgrForwardRun {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	sourceAddr := netip.MustParseAddr("203.0.113.71")
	source := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       sourceAddr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65001,
		RouterID:      0xcb007147,
		RSFastPath:    rs,
	})
	source.state.Store(int32(PeerStateEstablished))
	session := NewSession(source.Settings())
	session.localLLGR.Store(localLLGR)
	source.session = session

	dest := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       netip.MustParseAddr("198.51.100.9"),
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65002,
		RouterID:      0x0a000001,
	})
	dest.state.Store(int32(PeerStateEstablished))
	dest.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{family.IPv4Unicast: true}})
	dest.sendCtx.Store(ctx)
	dest.sendCtxID = ctxID
	dest.refreshForwardFacts()

	var mu sync.Mutex
	var run llgrForwardRun
	arrived := make(chan struct{}, 16)
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		defer mu.Unlock()
		for i := range items {
			for _, body := range items[i].rawBodies {
				u, unpackErr := message.UnpackUpdate(body)
				require.NoError(t, unpackErr)
				run.written = append(run.written, mixedMsgOf(t, u))
			}
			for _, u := range items[i].updates {
				run.written = append(run.written, mixedMsgOf(t, u))
			}
		}
		arrived <- struct{}{}
	}, fwdPoolConfig{chanSize: 16, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)
	pool.registerOutgoingPool(fwdKey{peerAddr: dest.Settings().PeerKey()}, 4096)

	step := llgrStaleEgressStep(&mu, &run.stepSaw)
	var dispatched []bgptypes.RawMessage
	r := &Reactor{
		clock:  source.clock,
		config: &Config{LocalAS: 65000},
		peers: map[netip.AddrPort]*Peer{
			source.Settings().PeerKey(): source,
			dest.Settings().PeerKey():   dest,
		},
		recentUpdates:   newRecentUpdateCache(100),
		attrModHandlers: attrModHandlersWithDefaults(),
		messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
			dispatched = append(dispatched, msg)
		}},
		orderedIngressSteps: []orderedIngressStep{{name: "llgr-ingress", inproc: llgrMetaStep(new([]bool))}},
		fwdPool:             pool,
		egressFilters:       []filterapi.EgressFilterFunc{step},
		orderedEgressSteps:  orderedEgressStepsFromFuncs(step),
		rsForwardingEnabled: rs,
	}

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x06, 0x02, 0x01, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x40, 0x03, 0x04, 203, 0, 113, 71, // NEXT_HOP
		0xc0, 0x08, 0x04, 0xff, 0xff, 0x00, 0x06, // COMMUNITIES LLGR_STALE
	}
	body := buildUpdatePayload(attrs, llgrForwardPrefix)
	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetSourceID(source.SourceID())

	r.notifyMessageReceiver(sourceAddr, msgtype.TypeUPDATE, body, wu, ctxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
	require.Len(t, dispatched, 1, "the UPDATE is dispatched to the plugins")

	if !rs {
		sel, selErr := selector.Parse("*")
		require.NoError(t, selErr)
		adapter := &reactorAPIAdapter{r: r}
		require.NoError(t, adapter.ForwardUpdate(sel, dispatched[0].MessageID, "test-plugin", plugin.OperatorSender()))
	}

	select {
	case <-arrived:
	case <-time.After(2 * time.Second):
		t.Fatal("the destination was asked to write nothing")
	}
	// Anything further the rail sends arrives inside this window.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	return llgrForwardRun{
		stepSaw: append([]any(nil), run.stepSaw...),
		written: append([]mixedMsg(nil), run.written...),
	}
}

// requireWithdrawnOnly asserts the destination was written only the withdrawal
// of llgrForwardPrefix, never its announcement.
func requireWithdrawnOnly(t *testing.T, written []mixedMsg) {
	t.Helper()
	require.NotEmpty(t, written, "the destination is written the withdrawal")
	for _, m := range written {
		require.Empty(t, m.nlri, "nothing is announced to the destination")
		require.False(t, m.hasReach, "nothing is announced to the destination")
		require.Equal(t, llgrForwardPrefix, m.withdrawn, "the route is withdrawn")
	}
}

// requireAnnounced asserts the destination was written the announcement of
// llgrForwardPrefix.
func requireAnnounced(t *testing.T, written []mixedMsg) {
	t.Helper()
	require.Len(t, written, 1, "the destination is written one UPDATE")
	require.Equal(t, llgrForwardPrefix, written[0].nlri, "the route is announced")
	require.Empty(t, written[0].withdrawn, "nothing is withdrawn")
}

// TestRFC9494CachedForwardWithholdsReceivedStale verifies the stale level the
// ingress step decided on receipt reaches the egress step when the route is
// forwarded later from the recent-update cache, which is how a plugin's forward
// command reaches the general rail.
//
// Method: receive an LLGR_STALE UPDATE through notifyMessageReceiver, forward
// it with ForwardUpdate, and read what the egress step was handed and what the
// destination was asked to write. The egress step asks for the withdrawal of a
// level-2 route, as LLGREgressFilter does toward a neighbor that never sent the
// LLGR Capability.
//
// VALIDATES: the cached ReceivedUpdate keeps the ingress meta, and
// forwardUpdateSection hands it to the egress pass.
// PREVENTS: a received LLGR_STALE route forwarded from the cache with no stale
// level, so the egress filter sees a fresh route and advertises it.
//
// RFC requirement: RFC9494-4.3-3 positive -- an LLGR_STALE UPDATE received on a session whose own OPEN carried the LLGR Capability, forwarded from the cache by ForwardUpdate, hands the egress step stale level uint8(2), and the destination that step withdraws it from is written only the withdrawal of the route.
//
// RFC requirement: RFC9494-4.3-3 negative -- the same UPDATE on a session whose own OPEN carried no LLGR Capability hands the egress step no stale level, and the destination is written the announcement.
//
// MUTATION: drop `Meta: routeMeta` from the cached ReceivedUpdate in
// notifyMessageReceiver, and the advertised case goes red.
func TestRFC9494CachedForwardWithholdsReceivedStale(t *testing.T) {
	t.Run("advertised", func(t *testing.T) {
		run := receiveAndForward(t, true, false)
		require.Equal(t, []any{uint8(2)}, run.stepSaw, "the egress step is handed stale level 2")
		requireWithdrawnOnly(t, run.written)
	})

	t.Run("not-advertised", func(t *testing.T) {
		run := receiveAndForward(t, false, false)
		require.Equal(t, []any{nil}, run.stepSaw, "the egress step is handed no stale level")
		requireAnnounced(t, run.written)
	})
}

// TestRFC9494RouteServerForwardWithholdsReceivedStale verifies the same through
// the route-server fast path, which forwards from the recent-update cache inside
// notifyMessageReceiver.
//
// Method: as TestRFC9494CachedForwardWithholdsReceivedStale, with the source on
// rs-fast-path and the reactor's route-server forwarding enabled, so
// reactorForwardRS forwards the cached entry.
//
// VALIDATES: reactorForwardRSSection hands the cached ingress meta to the
// egress filters.
// PREVENTS: the route-server rail advertising a received LLGR_STALE route as a
// fresh one.
//
// RFC requirement: RFC9494-4.3-3 positive -- an LLGR_STALE UPDATE received on a session whose own OPEN carried the LLGR Capability, forwarded by the route-server fast path, hands the egress filter stale level uint8(2), and the destination that filter withdraws it from is written only the withdrawal of the route.
//
// RFC requirement: RFC9494-4.3-3 negative -- the same UPDATE on a session whose own OPEN carried no LLGR Capability hands the egress filter no stale level, and the destination is written the announcement.
//
// MUTATION: drop `Meta: routeMeta` from the cached ReceivedUpdate in
// notifyMessageReceiver, and the advertised case goes red.
func TestRFC9494RouteServerForwardWithholdsReceivedStale(t *testing.T) {
	t.Run("advertised", func(t *testing.T) {
		run := receiveAndForward(t, true, true)
		require.Equal(t, []any{uint8(2)}, run.stepSaw, "the egress filter is handed stale level 2")
		requireWithdrawnOnly(t, run.written)
	})

	t.Run("not-advertised", func(t *testing.T) {
		run := receiveAndForward(t, false, true)
		require.Equal(t, []any{nil}, run.stepSaw, "the egress filter is handed no stale level")
		requireAnnounced(t, run.written)
	})
}
