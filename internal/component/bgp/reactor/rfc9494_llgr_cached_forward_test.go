// Design: docs/architecture/meta/README.md -- route metadata on a received event
// RFC: rfc/short/rfc9494.md -- Section 4.3, a received LLGR_STALE route is not advertised onward
// Related: reactor_notify.go -- notifyMessageReceiver, the ingress meta stored on the cached ReceivedUpdate
// Related: reactor_api_forward.go -- forwardUpdateSection, the general rail's egress pass
// Related: forward_rs.go -- reactorForwardRSSection, the route-server rail's egress pass

package reactor

import (
	"encoding/json"
	"net/netip"
	"sync"
	"testing"
	"time"

	routepolicy "github.com/ze-software/ze/internal/component/policy"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
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

// llgrFieldStale is the route metadata field the ingress step writes and the
// export filter reads: the GR plugin's own "stale" field, which gr/register.go
// registers in this test binary (llgr_down_writer_test.go links the plugin).
const llgrFieldStale = "stale"

// llgrStaleExportFilter stands in for the GR plugin's LLGR export filter,
// which the reactor may not import, toward a destination that never sent the
// LLGR Capability. It is the destination's export-chain member probe:f,
// reading the stale field: it records the stale level the forward rail sent
// it (nil when the route carries none), and at level 2 and above it answers
// withdraw, the way that filter does.
func llgrStaleExportFilter(t *testing.T, mu *sync.Mutex, seen *[]any) *externalTransport {
	t.Helper()
	return &externalTransport{reads: []string{llgrFieldStale}, answer: func(in rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
		mu.Lock()
		defer mu.Unlock()
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(in.Fields, &fields); err != nil {
			t.Errorf("the filter's fields do not decode: %v (%s)", err, in.Fields)
			return rpc.FilterUpdateOutput{Action: rpc.FilterReject}
		}
		raw, carried := fields[llgrFieldStale]
		if !carried {
			*seen = append(*seen, nil)
			return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
		}
		var level uint8
		if err := json.Unmarshal(raw, &level); err != nil {
			t.Errorf("the stale field %s is not a level: %v", raw, err)
			return rpc.FilterUpdateOutput{Action: rpc.FilterReject}
		}
		*seen = append(*seen, level)
		if level >= 2 {
			return rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}
		}
		return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
	}}
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
		// RFC 8212 Section 3 rejects at an absent policy on an EBGP session.
		ImportPolicy: mustPolicy(t, nil, routepolicy.FinalAccept),
		ExportPolicy: mustPolicy(t, nil, routepolicy.FinalAccept),
		RouterID:     0xcb007147,
		RSFastPath:   rs,
		// The route server's receive grant: the RS fast path relays only a
		// source the route server's cached forward would relay.
		ProcessBindings: []ProcessBinding{rsRelayReceiveBinding()},
	})
	source.state.Store(int32(PeerStateEstablished))
	session := NewSession(source.Settings())
	session.localLLGR.Store(localLLGR)
	source.session = session
	// An established session holds a forwarding snapshot (setEncodingContexts);
	// the RS fast-path gate reads the source's receive grant from it.
	source.refreshForwardFacts()

	dest := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       netip.MustParseAddr("198.51.100.9"),
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65002,
		// RFC 8212 Section 3 rejects at an absent policy on an EBGP session.
		ImportPolicy: mustPolicy(t, nil, routepolicy.FinalAccept),
		ExportPolicy: mustPolicy(t, nil, routepolicy.FinalAccept),
		RouterID:     0x0a000001,
		// The route server's send grant: the RS fast path reaches no
		// destination without it, as the cached rail does not.
		ProcessBindings: []ProcessBinding{rsRelaySendBinding()},
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

	// The filter is the destination's export chain, compiled at bgp export
	// as bgp/config compiles it, which both rails run.
	chain, err := policychain.Compile("bgp export", nil, []policychain.Member{{Name: "f", Plugin: "probe"}},
		routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	require.NoError(t, err)
	dest.settings.ExportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "probe:f"}}, routepolicy.FinalAccept).WithLibrary(chain)
	dest.refreshForwardFacts()
	transport := llgrStaleExportFilter(t, &mu, &run.stepSaw)
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
		orderedEgressSteps:  buildOrderedEgressSteps(),
		filterTransportSeam: transport,
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
// it with ForwardUpdate, and read the stale field the destination's export
// filter (probe:f, reading stale) was sent and what the destination was asked
// to write. The export filter answers withdraw for a
// level-2 route, as LLGREgressFilter does toward a neighbor that never sent the
// LLGR Capability.
//
// VALIDATES: the cached ReceivedUpdate keeps the ingress meta, and
// forwardUpdateSection hands it to the egress pass.
// PREVENTS: a received LLGR_STALE route forwarded from the cache with no stale
// level, so the egress filter sees a fresh route and advertises it.
//
// RFC requirement: RFC9494-4.3-3 positive -- an LLGR_STALE UPDATE received on a session whose own OPEN carried the LLGR Capability, forwarded from the cache by ForwardUpdate, sends the destination's export filter stale level 2, and the destination that filter withdraws it from is written only the withdrawal of the route.
//
// RFC requirement: RFC9494-4.3-3 negative -- the same UPDATE on a session whose own OPEN carried no LLGR Capability sends the export filter no stale level, and the destination is written the announcement.
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
