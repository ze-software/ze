// Design: docs/architecture/forward-congestion-pool.md -- the replay fence
// Related: peer.go -- forwardOrderHold, initialUpdateOwed, SignalAPIReady
// Related: forward_pool.go -- takeOverflowReleased, the per-item hold

package reactor

import (
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/selector"
)

// replayFenceOwner is the process the fenced destination waits for.
const replayFenceOwner = "route-server"

// TestLiveForwardWaitsForPeerUpReplay proves that a destination whose initial
// update is still owed receives the replay before any live change, whichever
// order the two rails dispatched them in.
//
// VALIDATES: the replay fence. While a process still owes the destination its
// share of the initial update (Peer.initialUpdateOwed), a live forward parks in
// the destination's overflow on both forwarding rails, an item marked as part
// of the initial update passes it, and the report that completes the update
// (SignalAPIReady) releases the parked items in their order.
//
// PREVENTS: a live WITHDRAW overtaking a peer-up replay that still carries the
// route, which left a withdrawn route installed at the late peer, and a live
// change reaching the peer before the replayed first announcement
// (test/plugin/forward-overflow-two-tier.ci, ordered= needles).
//
// Method: the live changes are dispatched FIRST, as when the route server
// forwards while its replay is still running, then the replay's announce. The
// destination's connection is the record of what reached the wire.
func TestLiveForwardWaitsForPeerUpReplay(t *testing.T) {
	type send struct {
		body   []byte
		replay bool // part of the initial update (StoredRoute.InitialUpdate)
		rsRail bool // live, through the route-server fast path
	}
	for _, tt := range []struct {
		name  string
		sends []send
		// want is the wire order: true for an announce, false for a withdraw.
		want []bool
	}{
		{
			name: "withdraw during replay leaves nothing installed",
			sends: []send{
				{body: syncOrderWithdrawBody, rsRail: true},
				{body: syncOrderAnnounceBody, replay: true},
			},
			want: []bool{true, false},
		},
		{
			name: "first-announced route arrives first",
			sends: []send{
				{body: syncOrderWithdrawBody},
				{body: syncOrderAnnounceBody},
				{body: syncOrderAnnounceBody, replay: true},
			},
			want: []bool{true, false, true},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, src, dst, conn, ctxID := newSyncOrderRail(t)
			adapter := &reactorAPIAdapter{r: r}

			// The reactor's own initial sync is over; the route server still
			// owes this destination its peer-up replay.
			dst.sendingInitialRoutes.Store(0)
			dst.resetAPISync([]string{replayFenceOwner})
			raiseTestReplayFence(dst, replayFenceOwner)

			for i, s := range tt.sends {
				id := uint64(7600 + i)
				update := syncOrderPublish(t, r, ctxID, id, s.body)
				if s.rsRail {
					// The fast path hands a fenced destination back, and the
					// route server's rail forwards it as a live change.
					skipped, dispatched := reactorForwardRS(r, update, id, netip.MustParseAddr(forwardSourceAddr), src)
					require.Equal(t, 0, dispatched, "the fast path must not decide for a fenced destination")
					require.Equal(t, []netip.AddrPort{dst.Settings().PeerKey()}, skipped, "the fenced destination must be handed back")
				}
				require.NoError(t, adapter.forwardUpdateCore(update, id, []*Peer{dst}, replayFenceSource(src, s.replay)))
			}

			// The replay reaches the wire while the fence is up; no live change
			// does. Never, not a single sample: a missing hold drains the live
			// items microseconds after the dispatch.
			require.Eventually(t, func() bool {
				return len(parseWireUpdates(t, conn.written())) > 0
			}, 5*time.Second, 5*time.Millisecond, "the replay must pass the fence")
			require.Never(t, func() bool {
				return len(parseWireUpdates(t, conn.written())) > 1
			}, 200*time.Millisecond, 5*time.Millisecond, "no live change may pass the fence")

			require.NoError(t, dst.SignalAPIReady(plugin.ProcessSender(replayFenceOwner), dst.session.initialReplay.Load()))

			var seen []wireUpdate
			require.Eventually(t, func() bool {
				seen = parseWireUpdates(t, conn.written())
				return len(seen) >= len(tt.want)
			}, 5*time.Second, 5*time.Millisecond, "the live changes must follow once the replay is reported")
			got := make([]bool, 0, len(seen))
			for _, u := range seen {
				got = append(got, u.announces)
			}
			require.Equal(t, tt.want, got, "wire order (true = announce, false = withdraw)")
		})
	}
}

// TestAnnounceRailWithdrawJoinsTheFence proves that a withdrawal sent on the
// announce rail, the rail a route server's peer-down withdrawal takes by
// selector, is ordered with the forwarding rails for a fenced destination.
//
// VALIDATES: while the replay fence is up, an announce-rail withdrawal joins the
// destination's forward queue as a live change (Peer.withdrawBehindForwards,
// reactorAPIAdapter.queueBehindForwards). It follows a live announce parked
// before it, and a replayed announce passes it.
//
// PREVENTS: the withdrawal reaching the wire at once while the announce of the
// same prefix waits behind the fence or rides the replay, which left the late
// peer holding a route whose source had gone down.
//
// Method: the destination's connection is the record of what reached the wire.
// Nothing live may reach it while the fence is up, and the order after the
// report is the assertion.
func TestAnnounceRailWithdrawJoinsTheFence(t *testing.T) {
	for _, tt := range []struct {
		name string
		// liveFirst forwards a live announce before the withdrawal; otherwise a
		// replayed announce follows the withdrawal.
		liveFirst bool
		// passes is how many UPDATEs reach the wire while the fence is up.
		passes int
	}{
		{name: "withdraw follows a parked live announce", liveFirst: true, passes: 0},
		{name: "replayed announce passes the withdraw", liveFirst: false, passes: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, src, dst, conn, ctxID := newSyncOrderRail(t)
			adapter := &reactorAPIAdapter{r: r}
			dst.sendingInitialRoutes.Store(0)
			dst.resetAPISync([]string{replayFenceOwner})
			raiseTestReplayFence(dst, replayFenceOwner)
			// Nothing has reached the connection yet, as when the replay and
			// the live changes are all still queued: the withdrawal must not be
			// withheld as never advertised (splitOnAdvertised).

			withdraw := func() {
				require.NoError(t, adapter.WithdrawNLRIBatch(t.Context(), selector.Addr(dst.Settings().Address),
					adjOutBatch("192.0.2.0/24", "10.0.0.1"), plugin.OperatorSender()))
			}
			if tt.liveFirst {
				update := syncOrderPublish(t, r, ctxID, 7700, syncOrderAnnounceBody)
				require.NoError(t, adapter.forwardUpdateCore(update, 7700, []*Peer{dst}, replayFenceSource(src, false)))
				withdraw()
			} else {
				withdraw()
				update := syncOrderPublish(t, r, ctxID, 7701, syncOrderAnnounceBody)
				require.NoError(t, adapter.forwardUpdateCore(update, 7701, []*Peer{dst}, replayFenceSource(src, true)))
			}

			if tt.passes > 0 {
				require.Eventually(t, func() bool {
					return len(parseWireUpdates(t, conn.written())) >= tt.passes
				}, 5*time.Second, 5*time.Millisecond, "the replay must pass the fence")
			}
			require.Never(t, func() bool {
				return len(parseWireUpdates(t, conn.written())) > tt.passes
			}, 200*time.Millisecond, 5*time.Millisecond, "no live change may pass the fence, the withdrawal included")

			require.NoError(t, dst.SignalAPIReady(plugin.ProcessSender(replayFenceOwner), dst.session.initialReplay.Load()))

			var seen []wireUpdate
			require.Eventually(t, func() bool {
				seen = parseWireUpdates(t, conn.written())
				return len(seen) >= 2
			}, 5*time.Second, 5*time.Millisecond, "the withdrawal must follow once the replay is reported")
			got := make([]bool, 0, len(seen))
			for _, u := range seen {
				got = append(got, u.announces)
			}
			require.Equal(t, []bool{true, false}, got, "wire order (true = announce, false = withdraw)")
		})
	}
}

// wireKinds names each UPDATE on a destination's wire: "announce", "withdraw"
// or "end-of-rib".
func wireKinds(seen []wireUpdate) []string {
	kinds := make([]string, 0, len(seen))
	for _, u := range seen {
		switch {
		case u.endOfRIB:
			kinds = append(kinds, "end-of-rib")
		case u.announces:
			kinds = append(kinds, "announce")
		default:
			kinds = append(kinds, "withdraw")
		}
	}
	return kinds
}

// TestReplayEndOfRIBFollowsTheReplay proves that the End-of-RIB marker a
// process sends when its peer-up replay ends reaches the wire after the replay
// items still queued for that peer, and before the live changes the fence holds.
//
// VALIDATES: AnnounceEOR queues the marker at the tail of the destination's
// forward queue (reactorAPIAdapter.queueEndOfRIB), marked as part of the
// initial update, and the worker meters it once written (settleEndOfRIB).
// RFC 4724 Section 2: the marker indicates "the completion of the initial
// routing update after the session is established".
//
// PREVENTS: the marker written at once, ahead of replayed announces still parked
// in the destination's overflow, so a graceful-restart helper acting on it
// purged routes the replay was still delivering.
//
// Method: the replayed announce and a live withdrawal park while the reactor's
// own sync runs. The sync then ends WITHOUT waking the worker, so the replay is
// still queued when the marker is asked for, which is the window a direct write
// raced. The destination's connection is the record of what reached the wire.
func TestReplayEndOfRIBFollowsTheReplay(t *testing.T) {
	r, src, dst, conn, ctxID := newSyncOrderRail(t)
	adapter := &reactorAPIAdapter{r: r}
	dst.resetAPISync([]string{replayFenceOwner})
	raiseTestReplayFence(dst, replayFenceOwner)

	replay := syncOrderPublish(t, r, ctxID, 7800, syncOrderAnnounceBody)
	require.NoError(t, adapter.forwardUpdateCore(replay, 7800, []*Peer{dst}, replayFenceSource(src, true)))
	live := syncOrderPublish(t, r, ctxID, 7801, syncOrderWithdrawBody)
	require.NoError(t, adapter.forwardUpdateCore(live, 7801, []*Peer{dst}, replayFenceSource(src, false)))
	require.True(t, dst.forwardOverflowPending(), "the replay and the live change must be parked")

	dst.sendingInitialRoutes.Store(0)
	require.NoError(t, adapter.AnnounceEOR(selector.Addr(dst.Settings().Address), 1, 1, plugin.OperatorSender()))
	dst.wakeForwardOverflow()

	require.Eventually(t, func() bool {
		return len(parseWireUpdates(t, conn.written())) >= 2
	}, 5*time.Second, 5*time.Millisecond, "the replay and its marker must pass the fence")
	require.Never(t, func() bool {
		return len(parseWireUpdates(t, conn.written())) > 2
	}, 200*time.Millisecond, 5*time.Millisecond, "no live change may pass the fence")

	require.NoError(t, dst.SignalAPIReady(plugin.ProcessSender(replayFenceOwner), dst.session.initialReplay.Load()))

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		return len(seen) >= 3
	}, 5*time.Second, 5*time.Millisecond, "the live change must follow once the replay is reported")
	require.Equal(t, []string{"announce", "end-of-rib", "withdraw"}, wireKinds(seen), "wire order")
	require.Equal(t, uint32(1), dst.counters.eorSent.Load(), "the marker is metered once, when written")
}

// TestAnnounceRailWithdrawFollowsOverflowForwards proves the congestion half of
// Peer.withdrawBehindForwards: with no replay fence, a withdrawal on the
// announce rail to a peer still owed forwarded items through overflow joins the
// queue behind them.
//
// VALIDATES: forwardOverflowPending alone diverts the withdrawal to
// queueBehindForwards, and splitOnAdvertised treats the peer as armed, so the
// withdrawal follows the parked announce of the same prefix.
//
// PREVENTS: the withdrawal reaching the wire first, or being withheld as never
// advertised, while the announce it withdraws still waits in overflow: the
// peer then holds a route whose source withdrew it.
//
// Method: the announce parks while the reactor's own sync runs, and the sync
// ends without waking the worker, so the announce is still owed through overflow
// when the withdrawal is sent.
func TestAnnounceRailWithdrawFollowsOverflowForwards(t *testing.T) {
	r, src, dst, conn, ctxID := newSyncOrderRail(t)
	adapter := &reactorAPIAdapter{r: r}
	dst.resetAPISync(nil)

	update := syncOrderPublish(t, r, ctxID, 7900, syncOrderAnnounceBody)
	require.NoError(t, adapter.forwardUpdateCore(update, 7900, []*Peer{dst}, replayFenceSource(src, false)))
	require.True(t, dst.forwardOverflowPending(), "the announce must be owed through overflow")
	require.False(t, dst.initialUpdateOwed.Load(), "no fence: only the overflow owes the peer")

	dst.sendingInitialRoutes.Store(0)
	require.NoError(t, adapter.WithdrawNLRIBatch(t.Context(), selector.Addr(dst.Settings().Address),
		adjOutBatch("192.0.2.0/24", "10.0.0.1"), plugin.OperatorSender()))
	dst.wakeForwardOverflow()

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		return len(seen) >= 2
	}, 5*time.Second, 5*time.Millisecond, "the announce and the withdrawal must both reach the wire")
	require.Equal(t, []string{"announce", "withdraw"}, wireKinds(seen), "wire order")
}

// TestReplayFenceHoldsOnlyItsOwnPeer proves the fence is per destination: a
// fenced peer's hold answers for that peer alone, and the report that
// completes its initial update lowers it.
//
// VALIDATES: forwardOrderHold reads the peer's own fence, never a shared one,
// and SignalAPIReady lowers it only when every owing process has reported.
// PREVENTS: a replay on one destination stalling live forwards to every other
// destination, the head-of-line block a plugin-wide wait produced.
func TestReplayFenceHoldsOnlyItsOwnPeer(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)
	fenced, _ := newSyncOrderDest(t, ctx, ctxID)
	other, _ := newSyncOrderDest(t, ctx, ctxID)
	fenced.sendingInitialRoutes.Store(0)
	other.sendingInitialRoutes.Store(0)

	fenced.resetAPISync([]string{replayFenceOwner, "receive-store", "never-reports"})
	raiseTestReplayFence(fenced, replayFenceOwner, "receive-store")
	other.resetAPISync(nil)

	require.True(t, fenced.forwardOrderHold(false), "a live change to the replaying peer must be held")
	require.False(t, fenced.forwardOrderHold(true), "the replay itself must pass its own fence")
	require.False(t, other.forwardOrderHold(false), "another peer must not be held by this peer's replay")

	require.NoError(t, fenced.SignalAPIReady(plugin.ProcessSender(replayFenceOwner), fenced.session.initialReplay.Load()))
	require.True(t, fenced.forwardOrderHold(false), "the fence stays up while another process still owes the update")

	require.NoError(t, fenced.SignalAPIReady(plugin.ProcessSender("receive-store"), fenced.session.initialReplay.Load()))
	require.False(t, fenced.forwardOrderHold(false),
		"the last fence owner's report must lower the fence, whatever a reporter outside it does")
}

// raiseTestReplayFence raises p's fence over owners, as raiseReplayFence does
// for the reporters whose registration declares FencesLiveForwards.
func raiseTestReplayFence(p *Peer, owners ...string) {
	p.mu.Lock()
	p.replayFence = owners
	p.initialUpdateOwed.Store(true)
	p.mu.Unlock()
}

// replayFenceSource resolves the test source peer the way ForwardUpdate does,
// with the initial-update mark the relay rail carries.
func replayFenceSource(src *Peer, initialUpdate bool) forwardSourceInfo {
	s := src.Settings()
	return forwardSourceInfo{
		isIBGP:         src.IsIBGP(),
		isRRClient:     s.RouteReflectorClient,
		remoteRouterID: src.RemoteRouterID(),
		globalLocalAS:  s.GlobalLocalAS,
		resolved:       true,
		peer:           src,
		sender:         plugin.OperatorSender(),
		initialUpdate:  initialUpdate,
	}
}

// TestRSFastPathHandsAFencedDestinationBack proves that the route-server fast
// path leaves a destination whose replay fence is up to the route server.
//
// VALIDATES: reactorForwardRSSection puts a destination whose initial update is
// still owed (Peer.initialUpdateOwed) on the skipped list, dispatches nothing to
// it and writes nothing for it, so bgp-rs decides it against its own peer-up
// cut (rs batchForwardUpdateSkipped). Once the fence is down the fast path
// writes to it again.
//
// PREVENTS: an UPDATE at or below the route server's cut reaching the peer
// twice, once in the peer-up replay and once from the fast path's fence
// parking, the second copy landing after a later withdrawal was due
// (test/plugin/role-otc-rs-withdraw-eor.ci under load: the peer read the
// announce where the withdrawal belonged).
//
// Method: one destination, fence raised, one fast-path forward; then the fence
// lowered by its owner's report and the same forward again. The destination's
// connection shows the fence released nothing the fast path held.
func TestRSFastPathHandsAFencedDestinationBack(t *testing.T) {
	r, src, dst, conn, ctxID := newSyncOrderRail(t)
	dst.sendingInitialRoutes.Store(0)
	dst.resetAPISync([]string{replayFenceOwner})
	raiseTestReplayFence(dst, replayFenceOwner)

	update := syncOrderPublish(t, r, ctxID, 7700, syncOrderAnnounceBody)
	skipped, dispatched := reactorForwardRS(r, update, 7700, netip.MustParseAddr(forwardSourceAddr), src)
	require.Equal(t, 0, dispatched, "nothing may be dispatched to a fenced destination")
	require.Equal(t, []netip.AddrPort{dst.Settings().PeerKey()}, skipped, "the fenced destination must be handed back")
	require.False(t, dst.forwardOverflowPending(), "nothing may be parked for the route server's replay to duplicate")

	require.NoError(t, dst.SignalAPIReady(plugin.ProcessSender(replayFenceOwner), dst.session.initialReplay.Load()))
	require.Empty(t, parseWireUpdates(t, conn.written()), "lowering the fence must release nothing the fast path held")

	update = syncOrderPublish(t, r, ctxID, 7701, syncOrderAnnounceBody)
	skipped, dispatched = reactorForwardRS(r, update, 7701, netip.MustParseAddr(forwardSourceAddr), src)
	require.Empty(t, skipped, "an unfenced destination is the fast path's")
	// Dispatched is the claim: the direct write leaves the bytes in the
	// session's buffer for the source's read loop to flush.
	require.Equal(t, 1, dispatched, "the fast path must forward once the fence is down")
}

// TestRSFastPathRedoHandsAFencedDestinationBack proves that the fast path's
// one-field-at-a-time redo leaves a destination to the route server when its
// replay fence went up after the first pass chose it.
//
// VALIDATES: a client a withhold gate sent to the redo (withdrawalBySection)
// whose fence is up by the time the redo runs is handed back on the skipped
// list, once, with nothing dispatched to it and nothing parked for it.
//
// PREVENTS: the redo parking a section in the destination's overflow while
// the route server's peer-up replay also carries the UPDATE, so the released
// copy reaches the peer a second time, behind the replay (the defect
// TestRSFastPathHandsAFencedDestinationBack covers for the first pass).
//
// Method: an egress step refuses the destination's announcement of an UPDATE
// that names two families, which sends the destination to the redo, and
// raises the destination's fence while it runs: the session coming up between
// the two passes, with no timing involved. The redo's withdrawals run no
// egress step, so the fence stays up for both sections.
func TestRSFastPathRedoHandsAFencedDestinationBack(t *testing.T) {
	forwardRSFencedMidPass(t, false)
}

// TestRSFastPathDispatchHandsAFencedDestinationBack proves the same hand-back
// for a fence that went up after selection, while the pass built the item.
//
// VALIDATES: a destination selected with its fence down whose fence is up by
// dispatch is handed back, once, with nothing dispatched and nothing parked.
//
// PREVENTS: the dispatch gate parking the item behind the fence, where the
// route server's replay duplicates it.
//
// Method: an egress step that passes the destination raises its fence while
// it runs, between selection and dispatch.
func TestRSFastPathDispatchHandsAFencedDestinationBack(t *testing.T) {
	forwardRSFencedMidPass(t, true)
}

// forwardRSFencedMidPass forwards one UPDATE naming two families on the fast
// path to one destination whose replay fence an egress step raises on its
// first call, answering pass, and asserts the destination is handed back once.
func forwardRSFencedMidPass(t *testing.T, pass bool) {
	t.Helper()
	destAddr := netip.MustParseAddr("2001:db8:1::5")
	payload := buildUpdatePayload(mixedAttrs(
		mixedAttr(15, mixedUnreachValue(1, 1, mixedV4MPPfx)),
		mixedReach(1, mixedGlobalNH, mixedV6Prefix),
	), nil)
	dst := mixedDest(t, destAddr.String())
	source := llnhExternalPeer(t, llnhAdvertiserAddr, llnhSegment, NextHopUnchanged)

	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)
	update.SourcePeerIP = source.Settings().Address

	pool := newFwdPool(func(fwdKey, []fwdItem) {}, fwdPoolConfig{chanSize: 64, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)
	key := fwdKey{peerAddr: dst.Settings().PeerKey()}
	pool.registerOutgoingPool(key, 4096)

	raised := false
	step := func(_, dest filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
		if dest.Address != destAddr {
			return true
		}
		if !raised {
			raised = true
			dst.resetAPISync([]string{replayFenceOwner})
			raiseTestReplayFence(dst, replayFenceOwner)
		}
		return pass
	}
	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			key.peerAddr:                dst,
			source.Settings().PeerKey(): source,
		},
		fwdPool:             pool,
		rsForwardingEnabled: true,
	}
	r.orderedEgressSteps = orderedEgressStepsFromFuncs(step)
	r.egressFilters = []filterapi.EgressFilterFunc{step}

	skipped, dispatched := reactorForwardRS(r, update, id, source.Settings().Address, source)
	require.True(t, raised, "the first pass must have met the egress step")
	require.Equal(t, []netip.AddrPort{key.peerAddr}, skipped, "the fenced destination must be handed back, once")
	require.Equal(t, 0, dispatched, "nothing may be dispatched to a fenced destination")
	require.False(t, dst.forwardOverflowPending(), "nothing may be parked for the route server's replay to duplicate")
}

// rsFenceFixture is a route server that holds the replay fence: registered
// with FencesLiveForwards and SignalsSessionReady, as bgp-rs is, and with
// RelaysOnRSFastPath so the fast path reaches a destination granting it
// `send [ update ]`.
const rsFenceFixture = "rs-fence-fixture"

// TestEstablishedIsPublishedBehindTheReplayFence proves that a destination's
// replay fence is up before its session becomes Established to the fast path.
//
// VALIDATES: enterEstablished raises the replay fence (Peer.initialUpdateOwed)
// for a peer whose route server declares it, before setState publishes
// PeerStateEstablished. A fast-path forward that runs the instant Established
// is visible hands the destination back to the route server, dispatches
// nothing to it and parks nothing for it.
//
// PREVENTS: the window between the publication and the fence. The initial-sync
// hold (Peer.forwardOrderHold) was up and the fence was not, so the fast path
// parked the UPDATE in the destination's overflow, and the route server's
// peer-up replay, whose cut was taken later, carried it too: the destination
// received the announcement twice (RFC 4271 Section 9.2).
//
// Method: the forward runs inside the peer's state callback, which setState
// calls right after it publishes the new state, so the interleaving is the
// first moment another goroutine can see Established, with no timing involved.
func TestEstablishedIsPublishedBehindTheReplayFence(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:                rsFenceFixture,
		Description:         "route-server fixture that holds the replay fence",
		RunEngine:           func(net.Conn) int { return 0 },
		CLIHandler:          func([]string) int { return 0 },
		RelaysOnRSFastPath:  true,
		FencesLiveForwards:  true,
		SignalsSessionReady: true,
	}))

	r, src, dst, conn, ctxID := newSyncOrderRail(t)
	dst.mu.Lock()
	dst.settings.ProcessBindings = []ProcessBinding{{
		PluginName: rsFenceFixture,
		Send:       map[string]bool{bgpevents.SendUpdate: true},
		Receive:    map[string]events.Direction{bgpevents.EventState: events.DirReceived},
	}}
	dst.mu.Unlock()
	dst.refreshForwardFacts()
	dst.state.Store(int32(PeerStateConnecting))
	dst.sendingInitialRoutes.Store(0)

	update := syncOrderPublish(t, r, ctxID, 7750, syncOrderAnnounceBody)
	var skipped []netip.AddrPort
	dispatched := -1
	dst.SetCallback(func(_, to PeerState) {
		if to != PeerStateEstablished {
			return
		}
		handed, sent := reactorForwardRS(r, update, 7750, netip.MustParseAddr(forwardSourceAddr), src)
		skipped, dispatched = slices.Clone(handed), sent
	})
	dst.enterEstablished()

	require.Equal(t, 0, dispatched, "nothing may be dispatched to a destination whose fence is owed")
	require.Equal(t, []netip.AddrPort{dst.Settings().PeerKey()}, skipped, "the destination must be handed back to the route server")
	require.False(t, dst.forwardOverflowPending(), "nothing may be parked for the route server's replay to duplicate")
	require.True(t, dst.initialUpdateOwed.Load(), "the fence must be up once Established is published")

	require.NoError(t, dst.SignalAPIReady(plugin.ProcessSender(rsFenceFixture), dst.session.initialReplay.Load()))
	require.Empty(t, parseWireUpdates(t, conn.written()), "lowering the fence must release nothing the fast path held")
}

// TestRSFastPathHandsBackAnUpdateOlderThanTheSession proves that an UPDATE
// numbered below a destination's initial-replay receipt is the route server's
// to forward, even once the replay fence is down.
//
// VALIDATES: enterEstablished takes the session's initial-replay receipt again,
// above every MessageID handed out before it, and the RS fast path hands the
// destination back for an UPDATE numbered below it (Peer.replayOwns) rather
// than dispatching it. An UPDATE numbered above it is dispatched as before.
//
// PREVENTS: the lost route. An UPDATE that met the fast path before the
// destination was a destination was handed to nobody, and when the route
// server saw the peer up before it took delivery of that UPDATE, its cut put
// the UPDATE on the live rail, which trusted the fast path: the destination
// received its End-of-RIB and never the route (RFC 4724 Section 2). The route
// server now forwards such an UPDATE by the receipt, so a fast-path copy as
// well would reach the peer twice.
//
// Method: the receipt is read back after enterEstablished, the fence is
// lowered through the route server's own report, and the fast path is run
// directly with one MessageID on each side of the receipt; no timing.
func TestRSFastPathHandsBackAnUpdateOlderThanTheSession(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:                rsFenceFixture,
		Description:         "route-server fixture that holds the replay fence",
		RunEngine:           func(net.Conn) int { return 0 },
		CLIHandler:          func([]string) int { return 0 },
		RelaysOnRSFastPath:  true,
		FencesLiveForwards:  true,
		SignalsSessionReady: true,
	}))

	r, src, dst, _, ctxID := newSyncOrderRail(t)
	dst.mu.Lock()
	dst.settings.ProcessBindings = []ProcessBinding{{
		PluginName: rsFenceFixture,
		Send:       map[string]bool{bgpevents.SendUpdate: true},
		Receive:    map[string]events.Direction{bgpevents.EventState: events.DirReceived},
	}}
	dst.mu.Unlock()
	dst.refreshForwardFacts()
	dst.state.Store(int32(PeerStateConnecting))
	dst.sendingInitialRoutes.Store(0)

	before := nextMsgID()
	dst.enterEstablished()
	receipt := dst.session.initialReplay.Load()
	require.Greater(t, receipt, before, "the receipt must be taken at establishment, above every earlier MessageID")
	require.NoError(t, dst.SignalAPIReady(plugin.ProcessSender(rsFenceFixture), receipt))
	require.False(t, dst.initialUpdateOwed.Load(), "the fence must be down for this test to judge the receipt alone")

	older := syncOrderPublish(t, r, ctxID, before, syncOrderAnnounceBody)
	skipped, dispatched := reactorForwardRS(r, older, before, netip.MustParseAddr(forwardSourceAddr), src)
	require.Equal(t, 0, dispatched, "an UPDATE older than the session is the route server's to forward")
	require.Equal(t, []netip.AddrPort{dst.Settings().PeerKey()}, skipped, "the destination must be handed back to the route server")

	newer := nextMsgID()
	update := syncOrderPublish(t, r, ctxID, newer, syncOrderAnnounceBody)
	skipped, dispatched = reactorForwardRS(r, update, newer, netip.MustParseAddr(forwardSourceAddr), src)
	require.Equal(t, 1, dispatched, "an UPDATE newer than the session is the fast path's")
	require.Empty(t, skipped, "nothing is handed back for an UPDATE newer than the session")
}
