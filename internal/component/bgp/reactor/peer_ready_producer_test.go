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
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/events"
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
	// afterRelay runs on the relay consumer's goroutine, after the real
	// consumer answered and before that answer reaches the plugin. A step
	// placed here is ordered before everything the plugin does next (its
	// readiness report included), which a step taken after reading
	// relayAttempts is not.
	afterRelay    func(error)
	beforeReady   func()
	attempts      chan error
	beforeInitial func()
}

type readyProducerReceipt struct {
	sender plugin.Sender
	token  uint64
	err    error
}

func (a *readyProducerAPI) SignalPeerAPIReady(ctx context.Context, peer string, sender plugin.Sender, token uint64) error {
	if a.beforeReady != nil {
		before := a.beforeReady
		a.beforeReady = nil
		before()
	}
	err := a.reactorAPIAdapter.SignalPeerAPIReady(ctx, peer, sender, token)
	a.ready <- readyProducerReceipt{sender: sender, token: token, err: err}
	return err
}

func (a *readyProducerAPI) AnnounceNLRIBatchResult(ctx context.Context, sel *selector.Selector, batch bgptypes.NLRIBatch, sender plugin.Sender) (*rpc.UpdateRouteOutput, error) {
	if batch.InitialReplay != 0 {
		if a.beforeInitial != nil {
			before := a.beforeInitial
			a.beforeInitial = nil
			before()
		}
	}
	output, err := a.reactorAPIAdapter.AnnounceNLRIBatchResult(ctx, sel, batch, sender)
	if batch.InitialReplay != 0 {
		a.attempts <- err
	}
	return output, err
}

func (a *readyProducerAPI) RelayStoredRouteBound(ctx context.Context, destination netip.Addr, routes []rpc.StoredRoute, sender plugin.Sender, owner plugin.SelectedOwner) error {
	err := a.reactorAPIAdapter.RelayStoredRouteBound(ctx, destination, routes, sender, owner)
	if a.afterRelay != nil {
		a.afterRelay(err)
	}
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
	r.api = srv
	require.NoError(t, events.RegisterNamespace("bgp", "update", "state", "refresh"))
	originalBus := registry.GetEventBus()
	registry.SetEventBus(srv)
	t.Cleanup(func() { registry.SetEventBus(originalBus) })
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
	publishTestBusGraph(t, r)
	return api, proc
}

// readyProducerSession publishes the generation of the fixture's original
// connection through the same state transition as enterEstablished. Its test
// EoR and replay fences are set independently by each caller.
func readyProducerSession(peer *Peer) *Session {
	session := peer.testInitialSync()
	if session.publishedGeneration.Load() == 0 {
		peer.setState(PeerStateEstablished)
	}
	return session
}

func readyProducerReplace(peer *Peer, session *Session) {
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	readyProducerSession(peer).sendingInitialRoutes.Store(0)
}

func readyProducerOutputGrant(name string) testPeerEndpointGrant {
	return testPeerEndpointGrant{
		Name:    name,
		Receive: []string{"state", "update-sent"},
		Send:    []string{"update"},
	}
}

func readyProducerState(t *testing.T, proc *process.Process, peer *Peer, session *Session, state rpc.SessionState, token uint64) {
	t.Helper()
	readyProducerStateWithReason(t, proc, peer, session, state, token, "")
}

func readyProducerStateWithReason(t *testing.T, proc *process.Process, peer *Peer, session *Session, state rpc.SessionState, token uint64, reason string) {
	t.Helper()
	info := boundPeerInfo(peer, session)
	require.NotZero(t, info.PeerOwner)
	require.NotZero(t, info.ConnectionID)
	require.NotZero(t, info.SessionGeneration)
	require.NotZero(t, info.DestinationSession)
	require.NoError(t, proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
		PeerAddress: info.AddrStr(), PeerOwner: info.PeerOwner, ConnectionID: info.ConnectionID,
		PeerAS: info.PeerAS, SessionGeneration: info.SessionGeneration, DestinationSession: info.DestinationSession,
		IsIBGP: info.IsIBGP, RSClient: info.RSClient, RouteReflectorClient: info.RouteReflectorClient,
		EventType: rpc.EventKindState, State: state, InitialReplay: token, Reason: reason,
	}}))
}

func readyProducerAttachSource(t *testing.T, peer *Peer, procs ...*process.Process) *Session {
	t.Helper()
	for _, proc := range procs {
		addTestPeerBusGrant(peer.settings, sourceLeaseGrant(proc.Name()))
	}
	if peer.reactor != nil {
		publishTestBusGraph(t, peer.reactor)
	}
	peer.advanceSourcePolicyEpoch()
	session := readyProducerSession(peer)
	for _, proc := range procs {
		readyProducerStateWithReason(t, proc, peer, session, rpc.SessionStateUp, 0, rpc.ReasonAttached)
	}
	return session
}

func readyProducerReceive(t *testing.T, proc *process.Process, peer *Peer, session *Session, update *ReceivedUpdate) {
	t.Helper()
	attrs, err := update.WireUpdate.Attrs()
	require.NoError(t, err)
	require.Equal(t, peer.sourceOwner, update.SourceOwner)
	require.Equal(t, session.sourceReceiptID(), update.SourceReceipt)
	require.NotZero(t, update.SourceReceipt)
	require.NoError(t, proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
		PeerAddress: peer.addrString, PeerAS: rpc.RemoteASOf(peer.PeerAS()), PeerOwner: update.SourceOwner,
		ConnectionID: session.connectionID, EventType: rpc.EventKindUpdate,
		IsIBGP: !peer.IsEBGP(), RSClient: peer.Settings().RSClient, RouteReflectorClient: peer.Settings().RouteReflectorClient,
		Direction: rpc.DirectionReceived, MessageID: update.WireUpdate.MessageID(),
		SourceReceipt: update.SourceReceipt, SourcePolicyEpoch: update.SourcePolicyEpoch,
		RawMessage: &bgptypes.RawMessage{WireUpdate: update.WireUpdate, AttrsWire: attrs,
			RawBytes: update.WireUpdate.Payload(), MessageID: update.WireUpdate.MessageID(),
			SourcePolicyEpoch: update.SourcePolicyEpoch, SourceOwner: update.SourceOwner,
			SourceConnectionID: session.connectionID, SourceReceipt: update.SourceReceipt},
	}}))
}

func readyProducerSent(proc *process.Process, peer *plugin.PeerInfo, msg *bgptypes.RawMessage) error {
	copied := *msg
	return proc.Bridge().DeliverStructured([]any{&rpc.StructuredEvent{
		PeerAddress: peer.AddrStr(), PeerAS: peer.PeerAS, PeerOwner: peer.PeerOwner,
		ConnectionID: peer.ConnectionID, DestinationSession: peer.DestinationSession, EventType: rpc.EventKindUpdate,
		Direction: rpc.DirectionSent, MessageID: copied.MessageID, RawMessage: &copied,
		SourcePeerStr: copied.SourcePeerStr, SourceID: copied.SourceID, Meta: copied.Meta,
	}})
}

func readyProducerAwait(t *testing.T, api *readyProducerAPI, token uint64, rejected bool) {
	t.Helper()
	readyProducerAwaitSender(t, api, plugin.ProcessSender("bgp-rib"), token, rejected)
}

func readyProducerAwaitSender(t *testing.T, api *readyProducerAPI, sender plugin.Sender, token uint64, rejected bool) {
	t.Helper()
	select {
	case receipt := <-api.ready:
		require.Equal(t, sender, receipt.sender)
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
			destination.settings.Bus = testBusBlockForGrants(readyProducerOutputGrant(owner))
			api, proc := readyProducerServer(t, r, owner)
			oldSession := readyProducerSession(destination)
			oldToken := oldSession.initialReplay.Load()
			replacement, conn := newSyncOrderDest(t, destination.sendCtx.Load(), ctxID)
			readyProducerReplace(destination, replacement.session)
			destination.resetAPISync([]string{owner})
			raiseTestReplayFence(destination, owner)
			current := destination.initialReplayToken()
			senderFor := func(token uint64) plugin.Sender {
				sender := plugin.ProcessSender(owner)
				if owner == "bgp-rs" || owner == "bgp-rr" {
					bound, err := sender.WithDestination(destination.sourceOwner, token)
					require.NoError(t, err)
					return bound
				}
				return sender
			}
			live := syncOrderPublish(t, r, ctxID, 8900, syncOrderAnnounceBody)
			// RFC 4271 Section 9.2: forward the queued announcement to the peer.
			require.NoError(t, api.forwardUpdateCore(live, 8900, []*Peer{destination}, replayFenceSource(source, false)))

			readyProducerState(t, proc, destination, oldSession, rpc.SessionStateUp, oldToken)
			readyProducerAwaitSender(t, api, senderFor(oldToken), oldToken, true)
			require.False(t, apiSyncReleased(destination))
			require.True(t, destination.forwardOrderHold(false))
			require.Empty(t, conn.written())

			readyProducerState(t, proc, destination, oldSession, rpc.SessionStateDown, 0)
			readyProducerState(t, proc, destination, replacement.session, rpc.SessionStateUp, current)
			readyProducerAwaitSender(t, api, senderFor(current), current, false)
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
			f.destination.settings.Bus = testBusBlockForGrants(readyProducerOutputGrant(owner))
			api, proc := readyProducerServer(t, f.r, owner)
			f.destination.resetAPISync([]string{owner})
			raiseTestReplayFence(f.destination, owner)
			session := readyProducerSession(f.destination)
			readyProducerState(t, proc, f.destination, session, rpc.SessionStateUp, f.destination.initialReplayToken())
			readyProducerAwait(t, api, f.destination.session.initialReplay.Load(), false)

			// The real final writer and notifyMessageReceiver produce the sent
			// ownership receipt consumed by the running RIB SDK handler.
			sent := make(chan error, 2)
			f.r.messageReceiver = &testMessageReceiver{onSent: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
				sent <- readyProducerSent(proc, &peer, &msg)
			}}
			f.destination.session.SetMessageCallback(f.r.capturedMessageReceiver)
			seed := func(source *Peer, body []byte) {
				sourceSession := readyProducerAttachSource(t, source, proc)
				update := f.publish(t, source, f.ctxID, body)
				// RFC 4271 Section 4.3.
				readyProducerReceive(t, proc, source, sourceSession, update)
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
			readyProducerState(t, proc, f.destination, session, rpc.SessionStateDown, 0)
			replacement, conn := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.ctxID)
			session = replacement.session
			readyProducerReplace(f.destination, session)
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
					readyProducerReplace(f.destination, next.session)
				}
				readyProducerState(t, proc, f.destination, session, rpc.SessionStateUp, token)
				readyProducerAwait(t, api, token, true)
				require.Error(t, readyProducerAttempt(t, api))
				require.True(t, f.destination.forwardOrderHold(false))
				require.False(t, apiSyncReleased(f.destination))
				require.Empty(t, conn.written())
				require.Empty(t, nextConn.written())
				conn = nextConn
				token = f.destination.initialReplayToken()
				readyProducerState(t, proc, f.destination, session, rpc.SessionStateDown, 0)
				session = next.session
			}
			live := f.publish(t, f.source, f.ctxID, syncOrderWithdrawBody)
			// RFC 4271 Section 9.2.
			require.NoError(t, api.forwardUpdateCore(live, f.id, []*Peer{f.destination}, replayFenceSource(f.source, false)))
			readyProducerState(t, proc, f.destination, session, rpc.SessionStateUp, token)
			readyProducerAwait(t, api, token, false)
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
	f.destination.settings.Bus = testBusBlockForGrants(readyProducerOutputGrant(owner))
	api, proc := readyProducerServer(t, f.r, owner)
	f.destination.resetAPISync([]string{owner})
	raiseTestReplayFence(f.destination, owner)
	session := readyProducerSession(f.destination)
	readyProducerState(t, proc, f.destination, session, rpc.SessionStateUp, f.destination.initialReplayToken())
	readyProducerAwait(t, api, f.destination.session.initialReplay.Load(), false)

	sent := make(chan error, 1)
	f.r.messageReceiver = &testMessageReceiver{onSent: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
		sent <- readyProducerSent(proc, &peer, &msg)
	}}
	f.destination.session.SetMessageCallback(f.r.capturedMessageReceiver)
	sourceSession := readyProducerAttachSource(t, f.source, proc)
	update := f.publish(t, f.source, f.ctxID, syncOrderAnnounceBody)
	readyProducerReceive(t, proc, f.source, sourceSession, update)
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

	readyProducerState(t, proc, f.destination, session, rpc.SessionStateDown, 0)
	replacement, conn := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.ctxID)
	session = replacement.session
	readyProducerReplace(f.destination, session)
	f.destination.resetAPISync([]string{owner})
	raiseTestReplayFence(f.destination, owner)
	token := f.destination.initialReplayToken()
	live := f.publish(t, f.source, f.ctxID, syncOrderWithdrawBody)
	require.NoError(t, api.forwardUpdateCore(live, f.id, []*Peer{f.destination}, replayFenceSource(f.source, false)))
	beforeRelease := make(chan []byte, 1)
	api.beforeReady = func() { beforeRelease <- conn.written() }
	readyProducerState(t, proc, f.destination, session, rpc.SessionStateUp, token)
	readyProducerAwait(t, api, token, false)
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
				f.destination.settings.Capabilities = []capability.Capability{&capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI}}
				f.destination.negotiated.Load().families[fam] = true
				f.destination.session.negotiated = forwardFixtureNegotiation(f.destination)
				f.destination.settings.Bus = testBusBlockForGrants(readyProducerOutputGrant(owner))
				api, proc := readyProducerServer(t, f.r, owner)
				f.destination.refreshForwardFacts()
				f.destination.resetAPISync([]string{owner})
				raiseTestReplayFence(f.destination, owner)
				initial := f.destination.initialReplayToken()
				session := readyProducerSession(f.destination)
				readyProducerState(t, proc, f.destination, session, rpc.SessionStateUp, initial)
				readyProducerAwait(t, api, initial, false)

				sent := make(chan error, 1)
				f.r.messageReceiver = &testMessageReceiver{onSent: func(peer plugin.PeerInfo, msg bgptypes.RawMessage) {
					sent <- readyProducerSent(proc, &peer, &msg)
				}}
				f.destination.session.SetMessageCallback(f.r.capturedMessageReceiver)
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
				sourceSession := readyProducerAttachSource(t, f.source, proc)
				for i, path := range paths {
					// RFC 7911 Section 3 and RFC 8277 Section 2.4.
					raw := ownershipRailNative(fam, path.id, path.rd, false, 0)
					update := f.publish(t, f.source, ctxID, ownershipRailBody(fam, false, raw, byte(i+1)))
					readyProducerReceive(t, proc, f.source, sourceSession, update)
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

				readyProducerState(t, proc, f.destination, session, rpc.SessionStateDown, 0)
				replacement, conn := newSyncOrderDest(t, ctx, ctxID)
				replacement.session.negotiated = forwardFixtureNegotiation(f.destination)
				session = replacement.session
				readyProducerReplace(f.destination, session)
				f.destination.resetAPISync([]string{owner})
				raiseTestReplayFence(f.destination, owner)
				token := f.destination.initialReplayToken()
				if mode == "stale" {
					next, nextConn := newSyncOrderDest(t, ctx, ctxID)
					next.session.negotiated = forwardFixtureNegotiation(f.destination)
					api.beforeInitial = func() {
						readyProducerReplace(f.destination, next.session)
					}
					readyProducerState(t, proc, f.destination, session, rpc.SessionStateUp, token)
					readyProducerAwait(t, api, token, true)
					for range paths {
						require.Error(t, readyProducerAttempt(t, api))
					}
					require.True(t, f.destination.forwardOrderHold(false))
					require.False(t, apiSyncReleased(f.destination))
					require.Empty(t, conn.written())
					require.Empty(t, nextConn.written())
					conn = nextConn
					token = f.destination.initialReplayToken()
					readyProducerState(t, proc, f.destination, session, rpc.SessionStateDown, 0)
					session = next.session
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
				readyProducerState(t, proc, f.destination, session, rpc.SessionStateUp, token)
				readyProducerAwait(t, api, token, false)
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

// TestPeerReadyRRReplayErrorCompletion observes the reflector's genuine
// source-authority refusal and the resulting held replay fence. A replacement
// can report ready only after its own stored replay reaches the wire.
func TestPeerReadyRRReplayErrorCompletion(t *testing.T) {
	for _, stale := range []bool{false, true} {
		name := "current"
		if stale {
			name = "stale"
		}
		t.Run(name, func(t *testing.T) {
			const owner = "bgp-rr"
			f := ownershipRailNew(t, nil)
			f.destination.settings.RouteReflectorClient = true
			f.destination.refreshForwardFacts()
			f.destination.settings.Bus = testBusBlockForGrants(
				readyProducerOutputGrant(owner),
				testPeerEndpointGrant{Name: rsFixtureRelay, Send: []string{"update"}},
			)
			api, proc := readyProducerServer(t, f.r, owner)
			adj := api.server.ProcessManager().GetProcess("bgp-adj-rib-in")
			require.NotNil(t, adj)
			sourceSession := readyProducerAttachSource(t, f.source, adj, proc)
			update := f.publish(t, f.source, f.ctxID, syncOrderAnnounceBody)
			// RFC 4271 Section 4.3: seed the actual receive store and reflector.
			readyProducerReceive(t, adj, f.source, sourceSession, update)
			readyProducerReceive(t, proc, f.source, sourceSession, update)

			original := readyProducerSession(f.destination)
			originalToken := original.initialReplay.Load()
			replacement, successorWire := newSyncOrderDest(t, f.destination.sendCtx.Load(), f.ctxID)
			successor := replacement.session
			require.NotEqual(t, original.connectionID, successor.connectionID)
			require.NotEqual(t, originalToken, successor.initialReplay.Load())

			f.destination.resetAPISync([]string{owner})
			raiseTestReplayFence(f.destination, owner)
			queueLive := func(last byte) ownershipRailEvent {
				t.Helper()
				body := append([]byte(nil), syncOrderAnnounceBody...)
				// The final byte is the third significant octet of the /24.
				body[len(body)-1] = last
				item := f.publish(t, f.source, f.ctxID, body)
				require.NoError(t, api.forwardUpdateCore(item, f.id, []*Peer{f.destination}, replayFenceSource(f.source, false)))
				return ownershipRailEvent{family: family.IPv4Unicast, key: string(body[len(body)-len(syncOrderPrefixWire):])}
			}
			queueLive(4)
			require.Empty(t, f.conn.written())

			installSuccessor := func() {
				readyProducerReplace(f.destination, successor)
				f.destination.resetAPISync([]string{owner})
				raiseTestReplayFence(f.destination, owner)
			}
			if stale {
				// The original writer has already returned its actual failure
				// when the replacement is installed. RR has yet to consume it.
				api.afterRelay = func(err error) {
					if err != nil {
						installSuccessor()
					}
				}
			}

			// Keep the admitted row and original receipt while making the live
			// source fail the writer's check. It has no retained membership.
			f.source.state.Store(int32(PeerStateStopped))
			readyProducerState(t, proc, f.destination, original, rpc.SessionStateUp, originalToken)
			select {
			case err := <-api.relayAttempts:
				require.ErrorContains(t, err, "ended without retained membership")
			case <-time.After(5 * time.Second):
				t.Fatal("RR replay never reached the actual relay consumer")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			require.ErrorContains(t, proc.SettleEventsApplied(ctx), "stored relay remains owed")
			cancel()

			// Settlement joined the replay lifecycle. An attempted readiness
			// receipt would have reached the observation channel by now.
			require.Empty(t, api.ready)
			require.False(t, apiSyncReleased(f.destination))
			require.True(t, f.destination.forwardOrderHold(false))
			require.Empty(t, f.conn.written())
			require.Empty(t, successorWire.written())
			if stale {
				require.Same(t, successor, f.destination.currentSession())
			} else {
				require.Same(t, original, f.destination.currentSession())
			}

			readyProducerState(t, proc, f.destination, original, rpc.SessionStateDown, 0)
			if !stale {
				installSuccessor()
			}
			// Restore independently of the refusal and attempt the replacement's
			// own replay. It does not certify the predecessor's failed output.
			f.source.state.Store(int32(PeerStateEstablished))
			live := queueLive(3)
			beforeReady := make(chan []byte, 1)
			api.beforeReady = func() { beforeReady <- successorWire.written() }
			successorToken := successor.initialReplay.Load()
			readyProducerState(t, proc, f.destination, successor, rpc.SessionStateUp, successorToken)
			select {
			case err := <-api.relayAttempts:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("replacement replay never reached the actual relay consumer")
			}
			sender, err := plugin.ProcessSender(owner).WithDestination(f.destination.sourceOwner, successorToken)
			require.NoError(t, err)
			select {
			case receipt := <-api.ready:
				require.Equal(t, sender, receipt.sender)
				require.Equal(t, successorToken, receipt.token)
				require.NoError(t, receipt.err)
			case <-time.After(5 * time.Second):
				t.Fatal("replacement replay never reached the actual readiness consumer")
			}

			ctxSend := f.destination.sendCtx.Load()
			replay := ownershipRailEvent{family: family.IPv4Unicast, key: string(syncOrderPrefixWire)}
			// RFC 4271 Section 4.3: actual announcements on distinct prefixes
			// identify the stored replay and the independently queued live delta.
			require.Equal(t, []ownershipRailEvent{replay}, ownershipRailRead(t, <-beforeReady, ctxSend))
			require.True(t, apiSyncReleased(f.destination))
			require.False(t, f.destination.forwardOrderHold(false))
			require.Eventually(t, func() bool {
				return len(ownershipRailRead(t, successorWire.written(), ctxSend)) == 2
			}, 5*time.Second, time.Millisecond)
			require.Equal(t, []ownershipRailEvent{replay, live}, ownershipRailRead(t, successorWire.written(), ctxSend))
			require.Empty(t, f.conn.written(), "the predecessor's queued delta must not reach either writer")
		})
	}
}
