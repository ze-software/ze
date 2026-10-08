// Design: docs/architecture/api/architecture.md -- captured peer-UP readiness.
// Related: peer_ready_receipt_test.go -- malformed and retired consumer receipts.
package reactor

import (
	"context"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/adj_rib_in"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rr"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rs"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/watchdog"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// readyProducerAPI observes results AFTER invoking the real reactor consumer.
// It never substitutes a command handler or acknowledges a receipt itself.
// beforeInitial is a one-shot scheduling boundary before real writer admission.
type readyProducerAPI struct {
	*reactorAPIAdapter
	ready         chan readyProducerReceipt
	server        *pluginserver.Server
	relayAttempts chan error
	beforeReady   func()
	attempts      chan error
	beforeInitial func()
}

type readyProducerReceipt struct {
	sender plugin.Sender
	token  uint64
	err    error
}

func (a *readyProducerAPI) SignalPeerAPIReady(peer string, sender plugin.Sender, token uint64) error {
	if a.beforeReady != nil {
		before := a.beforeReady
		a.beforeReady = nil
		before()
	}
	err := a.reactorAPIAdapter.SignalPeerAPIReady(peer, sender, token)
	a.ready <- readyProducerReceipt{sender: sender, token: token, err: err}
	return err
}

func (a *readyProducerAPI) AnnounceNLRIBatch(ctx context.Context, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) error {
	if batch.InitialReplay != 0 {
		if a.beforeInitial != nil {
			before := a.beforeInitial
			a.beforeInitial = nil
			before()
		}
	}
	err := a.reactorAPIAdapter.AnnounceNLRIBatch(ctx, sel, batch, sender)
	if batch.InitialReplay != 0 {
		a.attempts <- err
	}
	return err
}

func (a *readyProducerAPI) RelayStoredRoute(destination netip.Addr, routes []rpc.StoredRoute, sender plugin.Sender) error {
	err := a.reactorAPIAdapter.RelayStoredRoute(destination, routes, sender)
	a.relayAttempts <- err
	return err
}

// readyProducerServer starts the registered plugin's real SDK runner, handshake,
// engine bridge and dispatcher. Tests inject events, never completion commands.
func readyProducerServer(t *testing.T, r *Reactor, name string) (*readyProducerAPI, *process.Process) {
	t.Helper()
	r.config = &Config{LocalAS: 65000}
	r.clock = clock.RealClock{}
	api := &readyProducerAPI{reactorAPIAdapter: &reactorAPIAdapter{r: r},
		ready: make(chan readyProducerReceipt, 8), attempts: make(chan error, 8),
		relayAttempts: make(chan error, 8)}
	// RS and RR claim replay ownership from the real Adj-RIB-In provider at
	// startup. Keep those prerequisites running, as the live reactor fixture does.
	names := []string{"bgp-rib", "bgp-adj-rib-in"}
	if name != "bgp-rib" && name != "bgp-adj-rib-in" {
		names = append(names, name)
	}
	configs := make([]plugin.PluginConfig, 0, len(names))
	for _, selected := range names {
		configs = append(configs, plugin.PluginConfig{Name: selected, Internal: true, Encoder: "json"})
	}
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: configs}, api)
	require.NoError(t, err)
	api.server = srv
	mgr := pluginmgr.NewManager()
	require.NoError(t, mgr.StartAll(t.Context(), nil, nil))
	t.Cleanup(func() { require.NoError(t, mgr.StopAll(context.Background())) })
	srv.SetProcessSpawner(mgr)
	t.Cleanup(func() {
		srv.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, srv.Wait(ctx))
	})
	require.NoError(t, srv.StartWithContext(t.Context()))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	require.NoError(t, srv.WaitForStartupComplete(ctx))
	// Engine startup completes after sending the final OK. The SDK still has
	// to consume it and publish its direct handlers before this fixture can
	// inject events, including events into the selected producer's dependencies.
	for _, selected := range names {
		proc := srv.ProcessManager().GetProcess(selected)
		require.NotNil(t, proc)
		require.NotNil(t, proc.Bridge())
		require.Eventually(t, proc.Bridge().Ready, 5*time.Second, time.Millisecond,
			"SDK must publish its handlers before direct delivery: %s", selected)
	}
	proc := srv.ProcessManager().GetProcess(name)
	require.NotNil(t, proc)
	require.NotNil(t, proc.Bridge())
	require.True(t, proc.Bridge().Ready())
	return api, proc
}

func readyProducerState(t *testing.T, proc *process.Process, peer *Peer, state rpc.SessionState, token uint64) {
	t.Helper()
	require.NoError(t, proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
		PeerAddress: peer.Settings().Address.String(), EventType: rpc.EventKindState,
		State: state, InitialReplay: token,
	}}))
}

func readyProducerAwait(t *testing.T, api *readyProducerAPI, owner string, token uint64, rejected bool) {
	t.Helper()
	select {
	case receipt := <-api.ready:
		require.Equal(t, plugin.ProcessSender(owner), receipt.sender)
		require.Equal(t, token, receipt.token)
		if rejected {
			require.Error(t, receipt.err)
		} else {
			require.NoError(t, receipt.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("producer never reached the registered readiness consumer")
	}
}

func readyProducerAttempt(t *testing.T, api *readyProducerAPI) error {
	t.Helper()
	select {
	case err := <-api.attempts:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("populated replay never reached the actual UPDATE consumer")
		return nil
	}
}

// TestPeerReadySDKProducers exercises every actual producer's empty replay.
// A delayed old event must reach the real consumer and be refused; a current
// completion must release actual queued wire output. No expected command string
// is manufactured by this test.
func TestPeerReadySDKProducers(t *testing.T) {
	for _, owner := range []string{"bgp-rib", "bgp-rs", "bgp-rr", "bgp-adj-rib-in", "bgp-watchdog"} {
		t.Run(owner, func(t *testing.T) {
			r, source, destination, _, ctxID := newSyncOrderRail(t)
			api, proc := readyProducerServer(t, r, owner)
			oldToken := destination.session.initialReplay
			replacement, conn := newSyncOrderDest(t, destination.sendCtx.Load(), ctxID)
			destination.mu.Lock()
			destination.session = replacement.session
			destination.mu.Unlock()
			destination.sendingInitialRoutes.Store(0)
			destination.resetAPISync([]string{owner})
			raiseTestReplayFence(destination, owner)
			current := destination.initialReplayToken()
			live := syncOrderPublish(t, r, ctxID, 8900, syncOrderAnnounceBody)
			// RFC 4271 Section 9.2: forward the queued announcement to the peer.
			require.NoError(t, api.forwardUpdateCore(live, 8900, []*Peer{destination}, replayFenceSource(source, false)))

			readyProducerState(t, proc, destination, rpc.SessionStateUp, oldToken)
			readyProducerAwait(t, api, owner, oldToken, true)
			require.False(t, apiSyncReleased(destination))
			require.True(t, destination.forwardOrderHold(false))
			require.Empty(t, conn.written())

			readyProducerState(t, proc, destination, rpc.SessionStateDown, 0)
			readyProducerState(t, proc, destination, rpc.SessionStateUp, current)
			readyProducerAwait(t, api, owner, current, false)
			require.True(t, apiSyncReleased(destination))
			require.False(t, destination.forwardOrderHold(false))
			require.Eventually(t, func() bool {
				// RFC 4271 Section 4.3.
				return len(parseWireUpdates(t, conn.written())) == 1
			}, 5*time.Second, time.Millisecond)
			// RFC 4271 Section 4.3.
			require.Equal(t, []string{"announce"}, wireKinds(parseWireUpdates(t, conn.written())))
		})
	}
}

// TestPeerReadyRIBReplayCompletion proves populated replay crosses the SDK,
// update command and final writer before readiness releases a live withdrawal.
// A source removed after collection is refused without abandoning the remaining
// group or its fence. A reconnect at that same boundary rejects both the stale
// write and its completion; only the new event releases the replacement.
func TestPeerReadyRIBReplayCompletion(t *testing.T) {
	for _, mode := range []string{"populated", "source-disappears", "stale-failure"} {
		t.Run(mode, func(t *testing.T) {
			const owner = "bgp-rib"
			f := ownershipRailNew(t, nil)
			f.source.settings.PeerAS = 65002
			f.source.refreshForwardFacts()
			f.destination.settings.ProcessBindings = []ProcessBinding{sendUpdateOnly(owner)}
			api, proc := readyProducerServer(t, f.r, owner)
			f.destination.resetAPISync([]string{owner})
			raiseTestReplayFence(f.destination, owner)
			readyProducerState(t, proc, f.destination, rpc.SessionStateUp, f.destination.initialReplayToken())
			readyProducerAwait(t, api, owner, f.destination.session.initialReplay, false)

			// The real final writer and notifyMessageReceiver produce the sent
			// ownership receipt consumed by the running RIB SDK handler.
			sent := make(chan error, 2)
			f.r.messageReceiver = &testMessageReceiver{onSent: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
				sent <- proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
					PeerAddress: peer.Address.String(), EventType: rpc.EventKindUpdate,
					Direction: rpc.DirectionSent, MessageID: msg.MessageID, RawMessage: &msg,
					SourcePeerStr: msg.SourcePeerStr, SourceID: msg.SourceID, Meta: msg.Meta,
				}})
			}}
			f.destination.session.SetMessageCallback(f.r.notifyMessageReceiver)
			seed := func(source *Peer, body []byte) {
				update := f.publish(t, source, f.ctxID, body)
				// RFC 4271 Section 4.3.
				attrs, err := update.WireUpdate.Attrs()
				require.NoError(t, err)
				require.NoError(t, proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
					PeerAddress: source.Settings().Address.String(), EventType: rpc.EventKindUpdate,
					Direction: rpc.DirectionReceived, MessageID: f.id,
					RawMessage: &bgptypes.RawMessage{WireUpdate: update.WireUpdate, AttrsWire: attrs, MessageID: f.id},
				}}))
				// RFC 4271 Section 9.2.
				require.NoError(t, api.ForwardUpdatesDirect([]uint64{f.id}, []netip.AddrPort{f.destination.Settings().PeerKey()}, owner, plugin.OperatorSender()))
				select {
				case err := <-sent:
					require.NoError(t, err)
				case <-time.After(5 * time.Second):
					t.Fatal("history never reached the actual sent writer callback")
				}
			}
			if mode == "source-disappears" {
				body := append([]byte(nil), syncOrderAnnounceBody...)
				// RFC 4271 Section 4.3: "The Prefix field contains an IP address
				// prefix, followed by enough trailing bits to make the end of
				// the field fall on an octet boundary."
				// The final three body octets hold the /24 prefix.
				body[len(body)-1] = 3
				seed(f.other, body)
			}
			body := append([]byte(nil), syncOrderAnnounceBody...)
			// RFC 6793 Section 3: "The AS path information exchanged between NEW
			// BGP speakers is carried in the existing AS_PATH attribute, except
			// that each AS number in the attribute is encoded as a four-octet
			// entity (instead of a two-octet entity)."
			// Body offsets 13..16 hold the first AS. Equal other attributes make
			// the disappearing source's AS65001 group sort before this AS65002.
			binary.BigEndian.PutUint32(body[13:17], 65002)
			seed(f.source, body)
			wantHistory := 1
			if mode == "source-disappears" {
				wantHistory = 2
			}
			require.Eventually(t, func() bool {
				// RFC 4271 Section 4.3.
				return len(parseWireUpdates(t, f.conn.written())) == wantHistory
			}, 5*time.Second, time.Millisecond, "history must actually flush before reconnect")
			readyProducerState(t, proc, f.destination, rpc.SessionStateDown, 0)
			replacement, conn := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.ctxID)
			f.destination.mu.Lock()
			f.destination.session = replacement.session
			f.destination.mu.Unlock()
			f.destination.sendingInitialRoutes.Store(0)
			f.destination.resetAPISync([]string{owner})
			raiseTestReplayFence(f.destination, owner)
			token := f.destination.initialReplayToken()
			if mode == "source-disappears" {
				api.beforeInitial = func() {
					f.r.mu.Lock()
					delete(f.r.peers, f.other.Settings().PeerKey())
					f.r.mu.Unlock()
				}
			}
			if mode == "stale-failure" {
				next, nextConn := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.ctxID)
				api.beforeInitial = func() {
					f.destination.mu.Lock()
					f.destination.session = next.session
					f.destination.mu.Unlock()
				}
				readyProducerState(t, proc, f.destination, rpc.SessionStateUp, token)
				readyProducerAwait(t, api, owner, token, true)
				require.Error(t, readyProducerAttempt(t, api))
				require.True(t, f.destination.forwardOrderHold(false))
				require.False(t, apiSyncReleased(f.destination))
				require.Empty(t, conn.written())
				require.Empty(t, nextConn.written())
				conn = nextConn
				token = f.destination.initialReplayToken()
				readyProducerState(t, proc, f.destination, rpc.SessionStateDown, 0)
			}
			live := f.publish(t, f.source, f.ctxID, syncOrderWithdrawBody)
			// RFC 4271 Section 9.2.
			require.NoError(t, api.forwardUpdateCore(live, f.id, []*Peer{f.destination}, replayFenceSource(f.source, false)))
			readyProducerState(t, proc, f.destination, rpc.SessionStateUp, token)
			readyProducerAwait(t, api, owner, token, false)
			if mode == "source-disappears" {
				require.ErrorIs(t, readyProducerAttempt(t, api), errAdjOutProvenance)
			}
			require.NoError(t, readyProducerAttempt(t, api), "independent history after a rejection must still be attempted")
			require.False(t, f.destination.forwardOrderHold(false))
			require.Eventually(t, func() bool {
				// RFC 4271 Section 4.3.
				return len(parseWireUpdates(t, conn.written())) == 2
			}, 5*time.Second, time.Millisecond)
			// RFC 4271 Section 4.3.
			require.Equal(t, []string{"announce", "withdraw"}, wireKinds(parseWireUpdates(t, conn.written())))
		})
	}
}

// TestPeerReadyRIBReconnectBeforeRelease retains populated reconnect coverage
// through the authoritative RIB rather than the retired sent-message cache.
// Its captured receipt cannot release a queued withdrawal before restoration.
func TestPeerReadyRIBReconnectBeforeRelease(t *testing.T) {
	const owner = "bgp-rib"
	f := ownershipRailNew(t, nil)
	f.destination.settings.ProcessBindings = []ProcessBinding{sendUpdateOnly(owner)}
	api, proc := readyProducerServer(t, f.r, owner)
	f.destination.resetAPISync([]string{owner})
	raiseTestReplayFence(f.destination, owner)
	readyProducerState(t, proc, f.destination, rpc.SessionStateUp, f.destination.initialReplayToken())
	readyProducerAwait(t, api, owner, f.destination.session.initialReplay, false)

	sent := make(chan error, 1)
	f.r.messageReceiver = &testMessageReceiver{onSent: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
		sent <- proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
			PeerAddress: peer.Address.String(), EventType: rpc.EventKindUpdate,
			Direction: rpc.DirectionSent, MessageID: msg.MessageID, RawMessage: &msg,
			SourcePeerStr: msg.SourcePeerStr, SourceID: msg.SourceID, Meta: msg.Meta,
		}})
	}}
	f.destination.session.SetMessageCallback(f.r.notifyMessageReceiver)
	update := f.publish(t, f.source, f.ctxID, syncOrderAnnounceBody)
	attrs, err := update.WireUpdate.Attrs()
	require.NoError(t, err)
	require.NoError(t, proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
		PeerAddress: f.source.Settings().Address.String(), EventType: rpc.EventKindUpdate,
		Direction: rpc.DirectionReceived, MessageID: f.id,
		RawMessage: &bgptypes.RawMessage{WireUpdate: update.WireUpdate, AttrsWire: attrs, MessageID: f.id},
	}}))
	// RFC 4271 Section 9.2: create history through the actual final writer.
	require.NoError(t, api.ForwardUpdatesDirect([]uint64{update.WireUpdate.MessageID()},
		[]netip.AddrPort{f.destination.Settings().PeerKey()}, "", plugin.OperatorSender()))
	select {
	case err := <-sent:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("RIB never received the actual sent history")
	}
	require.Eventually(t, func() bool {
		// RFC 4271 Section 4.3.
		return len(parseWireUpdates(t, f.conn.written())) == 1
	}, 5*time.Second, time.Millisecond)

	readyProducerState(t, proc, f.destination, rpc.SessionStateDown, 0)
	replacement, conn := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.ctxID)
	f.destination.mu.Lock()
	f.destination.session = replacement.session
	f.destination.mu.Unlock()
	f.destination.sendingInitialRoutes.Store(0)
	f.destination.resetAPISync([]string{owner})
	raiseTestReplayFence(f.destination, owner)
	token := f.destination.initialReplayToken()
	live := f.publish(t, f.source, f.ctxID, syncOrderWithdrawBody)
	require.NoError(t, api.forwardUpdateCore(live, f.id, []*Peer{f.destination}, replayFenceSource(f.source, false)))
	beforeRelease := make(chan []byte, 1)
	api.beforeReady = func() { beforeRelease <- conn.written() }
	readyProducerState(t, proc, f.destination, rpc.SessionStateUp, token)
	readyProducerAwait(t, api, owner, token, false)
	require.NoError(t, readyProducerAttempt(t, api))
	// RFC 4271 Section 4.3: bytes must precede readiness, not be scheduled by it.
	require.Equal(t, []string{"announce"}, wireKinds(parseWireUpdates(t, <-beforeRelease)))
	require.Eventually(t, func() bool {
		return len(parseWireUpdates(t, conn.written())) == 2
	}, 5*time.Second, time.Millisecond)
	require.Equal(t, []string{"announce", "withdraw"}, wireKinds(parseWireUpdates(t, conn.written())))
	require.False(t, f.destination.forwardOrderHold(false))
}

// TestPeerReadyRIBNativeReconnect preserves source/path/native identity through
// the actual received and sent SDK handlers, then the initial-replay writer.
// Wrong-source, wrong-path and wrong-RD withdrawals cannot erase restored paths.
func TestPeerReadyRIBNativeReconnect(t *testing.T) {
	families := []family.Family{
		family.IPv4Unicast, family.IPv6Unicast,
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
	}
	for _, fam := range families {
		for _, mode := range []string{"current", "stale"} {
			t.Run(fam.String()+"/"+mode, func(t *testing.T) {
				const owner = "bgp-rib"
				f := ownershipRailNew(t, nil)
				ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: true})
				ctxID, err := bgpctx.Registry.Register(ctx)
				require.NoError(t, err)
				f.destination.sendCtx.Store(ctx)
				f.destination.sendCtxID = ctxID
				f.destination.session.setSendCtxID(ctxID)
				f.destination.negotiated.Load().families[fam] = true
				f.destination.settings.ProcessBindings = []ProcessBinding{sendUpdateOnly(owner)}
				f.destination.refreshForwardFacts()
				api, proc := readyProducerServer(t, f.r, owner)
				f.destination.resetAPISync([]string{owner})
				raiseTestReplayFence(f.destination, owner)
				initial := f.destination.initialReplayToken()
				readyProducerState(t, proc, f.destination, rpc.SessionStateUp, initial)
				readyProducerAwait(t, api, owner, initial, false)

				sent := make(chan error, 1)
				f.r.messageReceiver = &testMessageReceiver{onSent: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
					sent <- proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
						PeerAddress: peer.Address.String(), EventType: rpc.EventKindUpdate,
						Direction: rpc.DirectionSent, MessageID: msg.MessageID, RawMessage: &msg,
						SourcePeerStr: msg.SourcePeerStr, SourceID: msg.SourceID, Meta: msg.Meta,
					}})
				}}
				f.destination.session.SetMessageCallback(f.r.notifyMessageReceiver)
				paths := []struct {
					id uint32
					rd byte
				}{{0, 1}, {7, 1}}
				if fam.SAFI == family.SAFIVPN {
					paths = append(paths, struct {
						id uint32
						rd byte
					}{7, 2})
				}
				for i, path := range paths {
					// RFC 7911 Section 3 and RFC 8277 Section 2.4.
					raw := ownershipRailNative(fam, path.id, path.rd, false, 0)
					update := f.publish(t, f.source, ctxID, ownershipRailBody(fam, false, raw, byte(i+1)))
					attrs, err := update.WireUpdate.Attrs()
					require.NoError(t, err)
					require.NoError(t, proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
						PeerAddress: f.source.Settings().Address.String(), EventType: rpc.EventKindUpdate,
						Direction: rpc.DirectionReceived, MessageID: f.id,
						RawMessage: &bgptypes.RawMessage{WireUpdate: update.WireUpdate, AttrsWire: attrs, MessageID: f.id},
					}}))
					require.NoError(t, api.ForwardUpdatesDirect([]uint64{f.id},
						[]netip.AddrPort{f.destination.Settings().PeerKey()}, owner, plugin.OperatorSender()))
					select {
					case err := <-sent:
						require.NoError(t, err)
					case <-time.After(5 * time.Second):
						t.Fatal("native history never reached the actual sent writer callback")
					}
				}
				require.Eventually(t, func() bool {
					return len(ownershipRailRead(t, f.conn.written(), ctx)) == len(paths)
				}, 5*time.Second, time.Millisecond)
				history := ownershipRailRead(t, f.conn.written(), ctx)

				readyProducerState(t, proc, f.destination, rpc.SessionStateDown, 0)
				replacement, conn := newSyncOrderDest(t, ctx, ctxID)
				f.destination.mu.Lock()
				f.destination.session = replacement.session
				f.destination.mu.Unlock()
				f.destination.sendingInitialRoutes.Store(0)
				f.destination.resetAPISync([]string{owner})
				raiseTestReplayFence(f.destination, owner)
				token := f.destination.initialReplayToken()
				if mode == "stale" {
					next, nextConn := newSyncOrderDest(t, ctx, ctxID)
					api.beforeInitial = func() {
						f.destination.mu.Lock()
						f.destination.session = next.session
						f.destination.mu.Unlock()
					}
					readyProducerState(t, proc, f.destination, rpc.SessionStateUp, token)
					readyProducerAwait(t, api, owner, token, true)
					for range paths {
						require.Error(t, readyProducerAttempt(t, api))
					}
					require.True(t, f.destination.forwardOrderHold(false))
					require.False(t, apiSyncReleased(f.destination))
					require.Empty(t, conn.written())
					require.Empty(t, nextConn.written())
					conn = nextConn
					token = f.destination.initialReplayToken()
					readyProducerState(t, proc, f.destination, rpc.SessionStateDown, 0)
				}

				queueWithdrawal := func(source *Peer, path uint32, rd byte) {
					// RFC 4760 Section 4, RFC 7911 Section 3 and RFC 8277 Section 2.4.
					raw := ownershipRailNative(fam, path, rd, true, 0xde)
					update := f.publish(t, source, ctxID, ownershipRailBody(fam, true, raw, 0))
					require.NoError(t, api.forwardUpdateCore(update, f.id, []*Peer{f.destination}, replayFenceSource(source, false)))
				}
				queueWithdrawal(f.other, 7, 1)
				queueWithdrawal(f.source, 99, 1)
				if fam.SAFI == family.SAFIVPN {
					queueWithdrawal(f.source, 7, 3)
				}
				queueWithdrawal(f.source, 7, 1)
				beforeRelease := make(chan []byte, 1)
				api.beforeReady = func() { beforeRelease <- conn.written() }
				readyProducerState(t, proc, f.destination, rpc.SessionStateUp, token)
				readyProducerAwait(t, api, owner, token, false)
				for range paths {
					require.NoError(t, readyProducerAttempt(t, api))
				}
				require.ElementsMatch(t, history, ownershipRailRead(t, <-beforeRelease, ctx),
					"all native paths must reach the wire before the readiness receipt")
				require.Eventually(t, func() bool {
					return len(ownershipRailRead(t, conn.written(), ctx)) == len(paths)+1
				}, 5*time.Second, time.Millisecond)
				after := ownershipRailRead(t, conn.written(), ctx)
				require.ElementsMatch(t, history, after[:len(paths)])
				var withdrawn ownershipRailEvent
				for _, event := range history {
					if event.origin == 2 {
						withdrawn = event
					}
				}
				withdrawn.origin = 0
				withdrawn.withdraw = true
				require.Equal(t, withdrawn, after[len(paths)])
				require.True(t, apiSyncReleased(f.destination))
				require.False(t, f.destination.forwardOrderHold(false))
			})
		}
	}
}

// TestPeerReadyRRReplayErrorCompletion makes the real Adj-RIB-In replay reach
// the reactor's send-permission refusal. RR must finish that failed attempt and
// release only the event's Session; a stale receipt cannot credit its successor.
func TestPeerReadyRRReplayErrorCompletion(t *testing.T) {
	for _, stale := range []bool{false, true} {
		name := "current"
		if stale {
			name = "stale"
		}
		t.Run(name, func(t *testing.T) {
			const owner = "bgp-rr"
			f := ownershipRailNew(t, nil)
			// The RR can send; its optional replay store cannot. This is a real
			// authority failure, not a substituted command or fabricated error.
			f.destination.settings.ProcessBindings = []ProcessBinding{sendUpdateOnly(owner)}
			api, proc := readyProducerServer(t, f.r, owner)
			adj := api.server.ProcessManager().GetProcess("bgp-adj-rib-in")
			require.NotNil(t, adj)
			update := f.publish(t, f.source, f.ctxID, syncOrderAnnounceBody)
			// RFC 4271 Section 4.3: seed the actual receive-store SDK consumer.
			attrs, err := update.WireUpdate.Attrs()
			require.NoError(t, err)
			require.NoError(t, adj.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
				PeerAddress: f.source.Settings().Address.String(), EventType: rpc.EventKindUpdate,
				Direction: rpc.DirectionReceived, MessageID: f.id,
				RawMessage: &bgptypes.RawMessage{WireUpdate: update.WireUpdate, AttrsWire: attrs, MessageID: f.id},
			}}))
			token := f.destination.session.initialReplay
			if stale {
				replacement, conn := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.ctxID)
				f.destination.mu.Lock()
				f.destination.session = replacement.session
				f.destination.mu.Unlock()
				f.conn = conn
			}
			f.destination.sendingInitialRoutes.Store(0)
			f.destination.resetAPISync([]string{owner})
			raiseTestReplayFence(f.destination, owner)
			// A genuine live announcement proves completed error exits drain the
			// fence, independently of the replay's deliberately refused route.
			require.NoError(t, api.forwardUpdateCore(update, f.id, []*Peer{f.destination}, replayFenceSource(f.source, false)))
			awaitRelayError := func() {
				t.Helper()
				select {
				case err := <-api.relayAttempts:
					require.ErrorIs(t, err, errSendNotPermitted)
				case <-time.After(5 * time.Second):
					t.Fatal("RR replay never reached the actual relay consumer")
				}
			}
			readyProducerState(t, proc, f.destination, rpc.SessionStateUp, token)
			awaitRelayError()
			readyProducerAwait(t, api, owner, token, stale)
			if stale {
				require.False(t, apiSyncReleased(f.destination))
				require.True(t, f.destination.forwardOrderHold(false))
				require.Empty(t, f.conn.written())
				readyProducerState(t, proc, f.destination, rpc.SessionStateDown, 0)
				token = f.destination.initialReplayToken()
				readyProducerState(t, proc, f.destination, rpc.SessionStateUp, token)
				awaitRelayError()
				readyProducerAwait(t, api, owner, token, false)
			}
			require.True(t, apiSyncReleased(f.destination))
			require.False(t, f.destination.forwardOrderHold(false))
			require.Eventually(t, func() bool {
				// RFC 4271 Section 4.3.
				return len(parseWireUpdates(t, f.conn.written())) == 1
			}, 5*time.Second, time.Millisecond)
			require.Equal(t, []string{"announce"}, wireKinds(parseWireUpdates(t, f.conn.written())))
		})
	}
}
