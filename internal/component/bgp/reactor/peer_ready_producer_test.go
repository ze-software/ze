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
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/persist"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rr"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rs"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/watchdog"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// readyProducerAPI observes results AFTER invoking the real reactor consumer.
// It never substitutes a command handler or acknowledges a receipt itself.
// beforeInitial is a one-shot scheduling boundary before real writer admission.
type readyProducerAPI struct {
	*reactorAPIAdapter
	ready         chan readyProducerReceipt
	attempts      chan error
	beforeInitial func()
}

type readyProducerReceipt struct {
	sender plugin.Sender
	token  uint64
	err    error
}

func (a *readyProducerAPI) SignalPeerAPIReady(peer string, sender plugin.Sender, token uint64) error {
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

// readyProducerServer starts the registered plugin's real SDK runner, handshake,
// engine bridge and dispatcher. Tests inject events, never completion commands.
func readyProducerServer(t *testing.T, r *Reactor, name string) (*readyProducerAPI, *process.Process) {
	t.Helper()
	r.config = &Config{LocalAS: 65000}
	r.clock = clock.RealClock{}
	api := &readyProducerAPI{reactorAPIAdapter: &reactorAPIAdapter{r: r},
		ready: make(chan readyProducerReceipt, 8), attempts: make(chan error, 8)}
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
	for _, owner := range []string{"bgp-rib", "bgp-rs", "bgp-rr", "bgp-adj-rib-in", "bgp-persist", "bgp-watchdog"} {
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
