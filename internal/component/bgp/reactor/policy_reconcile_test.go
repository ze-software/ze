// Design: docs/guide/bgp-policy.md -- final transitions preserve current wire state.
package reactor

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/sim"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// mustApplyHotSwappableSettings keeps success-path tests sensitive to apply
// failures; failure-path tests exercise the transaction's returned error instead.
func mustApplyHotSwappableSettings(t *testing.T, peer *Peer, next *PeerSettings, copy settingsCopier) {
	t.Helper()
	if err := peer.applyHotSwappableSettings(next, copy); err != nil {
		t.Fatal(err)
	}
}

// policyReplayServer installs its retained-page callback before the SDK starts
// serving commands. Tests never mutate a live bridge handler.
func policyReplayServer(t *testing.T, r *Reactor, execute func() (any, error)) {
	t.Helper()
	const name = "policy-retained-page"
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	if err := registry.Register(registry.Registration{
		Name: name, Description: "Policy retained-page barrier",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			p.OnExecuteCommand(func(_, _ string, _ []string, _ string) (string, any, error) {
				data, err := execute()
				return rpc.StatusDone, data, err
			})
			if err := p.Run(context.Background(), &sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: ribevents.ReplayRoutesCommand}},
			}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}); err != nil {
		t.Fatal(err)
	}
	srv := policyFinalPluginServer(t, r, name)
	process := srv.ProcessManager().GetProcess(name)
	if process == nil || process.Bridge() == nil {
		t.Fatal("retained-page process did not start")
	}
	lowEventually(t, process.Bridge().HasExecuteCommand, "retained-page callback published")
}

func policyFinalPluginServer(t *testing.T, r *Reactor, names ...string) *pluginserver.Server {
	t.Helper()
	if r.config == nil {
		r.config = &Config{LocalAS: 65000}
	}
	r.clock = clock.RealClock{}
	api := &reactorAPIAdapter{r: r}
	configs := make([]plugin.PluginConfig, 0, len(names))
	for _, name := range names {
		configs = append(configs, plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"})
	}
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: configs}, api)
	if err != nil {
		t.Fatal(err)
	}
	r.api = srv
	originalBus := registry.GetEventBus()
	registry.SetEventBus(srv)
	t.Cleanup(func() { registry.SetEventBus(originalBus) })
	manager := pluginmgr.NewManager()
	if err := manager.StartAll(t.Context(), nil, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.StopAll(context.Background()); err != nil {
			t.Error(err)
		}
	})
	srv.SetProcessSpawner(manager)
	t.Cleanup(func() {
		srv.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Wait(ctx); err != nil {
			t.Error(err)
		}
	})
	if err := srv.StartWithContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := srv.WaitForStartupComplete(t.Context()); err != nil {
		t.Fatal(err)
	}
	return srv
}

// policyStopReplayStore leaves a registered command with a genuinely unavailable
// provider. Stopping only bridge dispatch does not stop native answer transport.
func policyStopReplayStore(t *testing.T, server *pluginserver.Server, destination netip.Addr) {
	t.Helper()
	command := server.Dispatcher().Registry().Lookup(ribevents.ReplayRoutesCommand)
	if command == nil {
		t.Fatal("fixture receive-store command missing")
	}
	command.Process.Stop()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := command.Process.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	command.Process.WaitRuntimeCleanup()
	server.Dispatcher().Registry().Register(command.Process, []pluginserver.CommandDef{
		{Name: ribevents.ReplayRoutesCommand},
	})
	request := &pluginserver.CommandContext{Server: server, RequestContext: t.Context(), Sender: plugin.OperatorSender()}
	_, err := server.Dispatcher().ForwardToPlugin(request, ribevents.ReplayRoutesCommand,
		[]string{destination.String(), "0"}, "*")
	if err == nil || !strings.Contains(err.Error(), pluginserver.ErrPluginProcessNotRunning.Error()) {
		t.Fatalf("registered stopped-store execution did not fail: %v", err)
	}
}

// TestPolicyFinalAcceptRemovalWithdrawsRemovedStatic checks the actual output
// when one reload removes both an accepting policy and a configured route.
func TestPolicyFinalAcceptRemovalWithdrawsRemovedStatic(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.reactor = &Reactor{clock: clock.RealClock{}}
	peer.settings.ExportPolicy = mustPolicy(t, nil, filterapi.FinalAccept)
	peer.settings.StaticRoutes = []StaticRoute{staticRouteAt("10.17.2.0/24", "10.9.9.9")}
	peer.sendInitialRoutes()
	initial := len(conn.written())
	if initial == 0 {
		t.Fatal("initial static advertisement did not reach the wire")
	}
	next := peer.settingsSnapshot()
	next.ExportPolicy = filterapi.Policy{}
	next.StaticRoutes = nil
	mustApplyHotSwappableSettings(t, peer, next, hotSwappableSettings)
	frames := bgpFrames(t, conn.written()[initial:])
	want := mustHex(t, "0004180a11020000")
	for _, frame := range frames {
		if frame[18] == byte(msgtype.TypeUPDATE) && bytes.Equal(frame[19:], want) {
			return
		}
	}
	t.Fatalf("removed static route remains advertised: subsequent frames %x", frames)
}

// TestPolicyFinalPureWithdrawalConsumerAllocations compares the full ingress
// delivery path with accepting and rejecting final policies. Rejecting additions
// must not allocate a replacement WireUpdate for an unchanged withdrawal.
func TestPolicyFinalPureWithdrawalConsumerAllocations(t *testing.T) {
	for _, bodyHex := range []string{"000418c000020000", "0000000d800f0a0002013020010db80001"} {
		t.Run(bodyHex, func(t *testing.T) {
			addr := netip.MustParseAddr("203.0.113.9")
			source := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001})
			source.state.Store(int32(PeerStateEstablished))
			body := mustHex(t, bodyHex)
			update := wireu.NewWireUpdate(body, 0)
			update.SetSourceID(source.SourceID())
			delivered := 0
			r := &Reactor{clock: source.clock, config: &Config{LocalAS: 65000},
				peers:               map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
				recentUpdates:       newRecentUpdateCache(100),
				orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
				messageReceiver: &testDeliveryReceiver{onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
					if !bytes.Equal(msg.RawBytes, body) {
						t.Fatalf("withdrawal consumer received %x, want %x", msg.RawBytes, body)
					}
					delivered++
				}}}
			measure := func(final filterapi.FinalDisposition) float64 {
				source.settings.ImportPolicy = mustPolicy(t, nil, final)
				return testing.AllocsPerRun(100, func() {
					r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, update, 0, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
				})
			}
			accept := measure(filterapi.FinalAccept)
			reject := measure(filterapi.FinalReject)
			if delivered != 202 {
				t.Fatalf("withdrawal delivery count %d, want 202", delivered)
			}
			if reject > accept {
				t.Fatalf("unchanged withdrawal adds allocations: reject=%g accept=%g", reject, accept)
			}
		})
	}
}

// TestPolicyFinalPausedWriterCannotEscapeReject pauses an originated writer
// after its old accepting decision. Reconciliation must wait for that writer
// before taking the withdrawal snapshot. A stack barrier observes the blocked
// mutex rather than assuming a scheduler delay has elapsed.
func TestPolicyFinalPausedWriterCannotEscapeReject(t *testing.T) {
	r, _, destination, conn, _ := newSyncOrderRail(t)
	r.clock = clock.RealClock{}
	destination.sendingInitialRoutes.Store(0)
	session := destination.currentSession()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	session.egressFinalAccepts = func() bool {
		accept := destination.ExportPolicy().AcceptsAtEnd()
		once.Do(func() {
			close(entered)
			<-release
		})
		return accept
	}
	update, err := message.UnpackUpdate(syncOrderAnnounceBody)
	if err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() {
		session.writeMu.Lock()
		err := session.writeUpdatePreFiltered(update)
		if err == nil {
			err = session.flushWrites()
		}
		session.writeMu.Unlock()
		written <- err
	}()
	<-entered
	destination.mu.Lock()
	destination.settings.ExportPolicy = mustPolicy(t, nil, filterapi.FinalReject)
	destination.mu.Unlock()
	destination.refreshForwardFacts()
	reconciled := make(chan error, 1)
	go func() { reconciled <- destination.reconcileExportFinal() }()
	var reconcileErr error
	finished := false
	stacks := make([]byte, 1<<20)
	lowEventually(t, func() bool {
		select {
		case reconcileErr = <-reconciled:
			finished = true
			return true
		default:
		}
		n := runtime.Stack(stacks, true)
		for stack := range strings.SplitSeq(string(stacks[:n]), "\n\n") {
			if strings.Contains(stack, "(*Peer).reconcileExportFinal(") &&
				strings.Contains(stack, "(*sessionWriteMutex).Lock(") {
				return true
			}
		}
		return false
	}, "reconciliation completes or waits on the paused writer")
	unblock.Do(func() { close(release) })
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if !finished {
		reconcileErr = <-reconciled
	}
	if reconcileErr != nil {
		t.Fatal(reconcileErr)
	}
	held := false
	for _, update := range parseWireUpdates(t, conn.written()) {
		if update.announces {
			held = true
		}
		if update.withdraws {
			held = false
		}
	}
	if held {
		t.Fatalf("paused writer left denied route advertised: %x", conn.written())
	}
}

// TestPolicyFinalRestoreFailureRollsBack drives the journal's static apply
// consumer with a failed retained-source command. The failed transition
// must not commit acceptance or discard its inverse operation.
func TestPolicyFinalRestoreFailureRollsBack(t *testing.T) {
	r, _, destination, _, _ := newSyncOrderRail(t)
	destination.sendingInitialRoutes.Store(0)
	api, _ := readyProducerServer(t, r, "bgp-rs")
	r.api = api.server
	destination.settings.ProcessBindings = []ProcessBinding{
		operatorBinding(t, "bgp-rs", "state", "update"),
	}
	destination.settings.ExportPolicy = mustPolicy(t, nil, filterapi.FinalReject)
	destination.refreshForwardFacts()
	policyStopReplayStore(t, r.api, destination.Settings().Address)
	next := destination.settingsSnapshot()
	next.ExportPolicy = mustPolicy(t, nil, filterapi.FinalAccept)
	journal := &internalJournal{}
	err := api.swapPeerSettingsJournaled([]peerSettingsSwap{{
		key: destination.Settings().PeerKey(), next: next, apply: hotSwappableSettings,
	}}, journal)
	if err == nil {
		t.Fatal("reload committed despite unavailable retained-source replay")
	}
	if !strings.Contains(err.Error(), pluginserver.ErrPluginProcessNotRunning.Error()) {
		t.Fatalf("reload lost reconciliation failure: %v", err)
	}
	if destination.ExportPolicy().AcceptsAtEnd() {
		t.Fatal("failed apply left accepting policy installed instead of rolling back")
	}
}

// TestPolicyFinalFlowSpecRestoration observes an authorized FlowSpec rule on
// the real TCP consumer, then withdraws and restores it without source updates.
func TestPolicyFinalFlowSpecRestoration(t *testing.T) {
	assertPolicyFinalFlowSpecRestoration(t, flowForwardLiveRouter(t, "self"))
}

func TestPolicyFinalFlowSpecOnlyRestorationWithoutReceiveStore(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	source := lowLiveSettings("192.0.2.1", 65000, 65002)
	destination := lowLiveSettings("192.0.2.2", 65000, 65003)
	source.Capabilities = append(source.Capabilities, &capability.Multiprotocol{AFI: 1, SAFI: 133})
	destination.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65000}, &capability.Multiprotocol{AFI: 1, SAFI: 133},
	}
	settings := []*PeerSettings{source, destination}
	for _, setting := range settings {
		setting.RSClient = true
		for _, binding := range []struct{ name, receive string }{
			{"bgp-rib", "update state refresh"},
			{"bgp-rs", "update-received state open-received refresh"},
		} {
			if err := EnsureProcessBinding(setting, binding.name, binding.receive, "update"); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.AddPeer(setting); err != nil {
			t.Fatal(err)
		}
	}
	srv := policyFinalPluginServer(t, r, "bgp-rib", "bgp-rs")
	r.SetPluginServer(srv)
	if err := r.StartWithContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	var peers []*lowLivePeer
	for _, setting := range settings {
		peer := r.peers[setting.PeerKey()]
		lowEventually(t, func() bool { return peer.currentSession() != nil && peer.SessionState() == fsm.StateActive }, "FlowSpec-only peer active")
		server, remote := net.Pipe()
		capture := &lowLivePeer{peer: peer, remote: remote, stopped: make(chan struct{})}
		go capture.readFrames()
		t.Cleanup(func() {
			_ = remote.Close()
			_ = server.Close()
			<-capture.stopped
		})
		if err := peer.acceptConnection(server); err != nil {
			t.Fatal(err)
		}
		open := &message.Open{Version: 4, MyAS: uint16(setting.PeerAS), HoldTime: 90,
			BGPIdentifier: 0x0a000001 + uint32(len(peers)), OptionalParams: []byte{
				2, 6, 65, 4, 0, 0, byte(setting.PeerAS >> 8), byte(setting.PeerAS),
				2, 6, 1, 4, 0, 1, 0, 1, 2, 6, 1, 4, 0, 1, 0, 133,
			}}
		capture.send(t, message.PackTo(open, nil))
		lowEventually(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm }, "FlowSpec-only OPEN accepted")
		capture.send(t, message.PackTo(message.NewKeepalive(), nil))
		lowEventually(t, func() bool { return peer.State() == PeerStateEstablished && !peer.pendingSync() }, "FlowSpec-only initial sync")
		peers = append(peers, capture)
	}
	if srv.ProcessManager().GetProcess("bgp-adj-rib-in") != nil ||
		srv.Dispatcher().Registry().Lookup(ribevents.ReplayRoutesCommand) != nil {
		t.Fatal("optional receive store unexpectedly present")
	}
	assertPolicyFinalFlowSpecRestoration(t, peers)
}

func assertPolicyFinalFlowSpecRestoration(t *testing.T, peers []*lowLivePeer) {
	t.Helper()
	source, destination := peers[0], peers[1]
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	raw := mustHex(t, "050118c00002")
	source.send(t, flowForwardFrame(mustHex(t, "000000144001010040020602010000fdea400304c000020118c00002")))
	source.send(t, flowForwardFrame(flowForwardPayload(t, fam, raw, mustHex(t, "c0000202"))))
	lowEventually(t, func() bool { return flowForwardReceived(destination, fam) != nil }, "authorized FlowSpec announcement")
	if !destination.peer.negotiated.Load().families[family.IPv4Unicast] {
		// The FlowSpec wire event fences the earlier covering-route forward.
		// Its remote OPEN offered IPv4, but the local side did not: that
		// unnegotiated route must never have reached this consumer.
		destination.mu.Lock()
		for _, frame := range destination.frames {
			if frame[18] != byte(msgtype.TypeUPDATE) {
				continue
			}
			update, err := message.UnpackUpdate(frame[message.HeaderLen:])
			if err != nil {
				t.Error(err)
				continue
			}
			if len(update.NLRI) != 0 || len(update.WithdrawnRoutes) != 0 {
				t.Errorf("FlowSpec-only consumer received unnegotiated IPv4 routes: %x", frame)
			}
		}
		destination.mu.Unlock()
	}
	initial := destination.peer.currentSession()
	next := destination.peer.settingsSnapshot()
	next.ExportPolicy = mustPolicy(t, nil, filterapi.FinalReject)
	mustApplyHotSwappableSettings(t, destination.peer, next, hotSwappableSettings)
	lowEventually(t, func() bool {
		destination.mu.Lock()
		defer destination.mu.Unlock()
		for _, frame := range destination.frames {
			if frame[18] != byte(msgtype.TypeUPDATE) {
				continue
			}
			update, err := message.UnpackUpdate(frame[message.HeaderLen:])
			if err != nil {
				t.Fatal(err)
			}
			_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
			if found && len(mp) >= 3 && bytes.Equal(mp[3:], raw) {
				return true
			}
		}
		return false
	}, "final rejection withdraws the authorized FlowSpec rule")
	destination.mu.Lock()
	destination.frames = nil
	destination.mu.Unlock()
	next = destination.peer.settingsSnapshot()
	next.ExportPolicy = mustPolicy(t, nil, filterapi.FinalAccept)
	mustApplyHotSwappableSettings(t, destination.peer, next, hotSwappableSettings)
	lowEventually(t, func() bool { return flowForwardReceived(destination, fam) != nil }, "retained authorized FlowSpec restoration")
	assertFlowForwardWire(t, flowForwardReceived(destination, fam), fam, raw)
	if destination.peer.currentSession() != initial {
		t.Fatal("final-only restoration replaced the session")
	}
}

// TestPolicyFinalReplayCannotReviveWithdrawnAddPath supplies a retained page
// while a same-session withdrawal overtakes its delivery. The page producer is
// paused at its existing command boundary; all relays and wire writes are real.
func TestPolicyFinalReplayCannotReviveWithdrawnAddPath(t *testing.T) {
	testPolicyFinalReplayWithdrawnAddPath(t, false)
}

// TestPolicyFinalReplayWithdrawalSurvivesGapEviction evicts the live withdrawal's
// cache entry after replay reaches the wire but before the held withdrawal runs.
// The recipient's advertised identity must outlive the cache allocator mapping.
func TestPolicyFinalReplayWithdrawalSurvivesGapEviction(t *testing.T) {
	testPolicyFinalReplayWithdrawnAddPath(t, true)
}

func testPolicyFinalReplayWithdrawnAddPath(t *testing.T, gapEviction bool) {
	t.Helper()
	fc := sim.NewFakeClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	var r *Reactor
	var eviction sync.Once
	evicted := make(chan bool, 1)
	r, source, destination, conn, _ := newSyncOrderRailWith(t, func(key fwdKey, items []fwdItem) {
		replayed := false
		for i := range items {
			if items[i].initialUpdate {
				replayed = replayed || len(items[i].rawBodies)+len(items[i].updates) != 0
			}
		}
		fwdBatchHandler(key, items)
		if !gapEviction {
			return
		}
		if !replayed {
			return
		}
		eviction.Do(func() {
			// A later fully acknowledged entry makes the retained live
			// withdrawal gap-evictable. The real scanner releases its
			// identifier even though the overflow item still owns a retain.
			r.recentUpdates.Add(newTestUpdate(^uint64(0)))
			r.recentUpdates.Activate(^uint64(0), 0)
			fc.Add(6 * time.Minute)
			r.recentUpdates.runGapScan()
			evicted <- !r.recentUpdates.Contains(9101)
		})
	})
	r.recentUpdates.SetClock(fc)
	destination.sendingInitialRoutes.Store(0)
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	source.recvCtxID = ctxID
	destination.sendCtx.Store(ctx)
	destination.sendCtxID = ctxID
	destination.session.setSendCtxID(ctxID)
	destination.refreshForwardFacts()
	api := &reactorAPIAdapter{r: r}
	// Consume any initial outgoing identifier zero on an unrelated path. The
	// target has never been exported: its unknown withdrawal must not pass
	// merely because replay happens to allocate the lookup's zero placeholder.
	for i, withdraw := range []bool{false, true} {
		id := uint64(9099 + i)
		body := ownershipRailBody(family.IPv4Unicast, withdraw, pathsLimitNLRI(8, "198.51.100.0/24"), 0)
		update := syncOrderPublish(t, r, ctxID, id, body)
		if err := api.forwardUpdateCore(update, id, []*Peer{destination}, replayFenceSource(source, false)); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.fwdPool.barrierPeer(t.Context(), destination.Settings().PeerKey()); err != nil {
		t.Fatal(err)
	}
	if events := ownershipRailRead(t, conn.written(), ctx); len(events) != 2 {
		t.Fatalf("identifier setup did not announce and withdraw its unrelated path: %+v", events)
	}
	wireOffset := len(conn.written())
	destination.settings.ProcessBindings = []ProcessBinding{
		operatorBinding(t, "policy-retained-page", "state", "update"),
	}
	route := storedIPv4Route(source.Settings().Address.String())
	route.PathID = 7
	route.NLRIHex = "18c00002"
	route.NLRIFraming = rpc.NLRIFramingPrefixOnly
	route.InitialUpdate = true
	page := ribevents.ReplayPage{Routes: []rpc.StoredRoute{route}, LastIndex: 1}
	policyReplayServer(t, r, func() (any, error) {
		body := ownershipRailBody(family.IPv4Unicast, true, pathsLimitNLRI(7, "192.0.2.0/24"), 0)
		live := syncOrderPublish(t, r, ctxID, 9101, body)
		if err := api.forwardUpdateCore(live, 9101, []*Peer{destination}, replayFenceSource(source, false)); err != nil {
			return nil, err
		}
		// Without a fence the live withdrawal must finish before the stale
		// page is returned. With one, its queued delivery follows the replay.
		if !destination.forwardOrderHold(false) {
			if err := r.fwdPool.barrierPeer(t.Context(), destination.Settings().PeerKey()); err != nil {
				return nil, err
			}
		}
		return page, nil
	})
	if err := destination.reconcileExportFinal(); err != nil {
		t.Fatal(err)
	}
	if err := r.fwdPool.barrierPeer(t.Context(), destination.Settings().PeerKey()); err != nil {
		t.Fatal(err)
	}
	if gapEviction {
		select {
		case removed := <-evicted:
			if !removed {
				t.Fatal("positive control: safety valve did not evict the held withdrawal")
			}
		default:
			t.Fatal("positive control: replay did not reach the eviction boundary")
		}
	}
	wire := conn.written()[wireOffset:]
	held := false
	announced := false
	for _, update := range parseWireUpdates(t, wire) {
		if update.announces {
			held, announced = true, true
		}
		if update.withdraws {
			held = false
		}
	}
	if !announced {
		t.Fatal("positive control: retained page never reached the destination")
	}
	if held {
		t.Fatalf("same-session withdrawn ADD-PATH route revived by stale replay: %x", wire)
	}
	var pathID uint32
	targetEvents := 0
	for _, event := range ownershipRailRead(t, wire, ctx) {
		if event.key != string(syncOrderPrefixWire) {
			continue
		}
		targetEvents++
		if event.withdraw {
			if event.path != pathID {
				t.Fatalf("withdrawal path identifier %d differs from replayed identifier %d", event.path, pathID)
			}
			continue
		}
		pathID = event.path
	}
	if targetEvents != 2 {
		t.Fatalf("target announcement and matching withdrawal count = %d, want 2", targetEvents)
	}
}

// TestPolicyFinalReplayPreservesPreAdmittedBatchOrder holds two already admitted
// worker batches across restoration. Their withdraw-then-announce order must
// survive parking in overflow, rather than reversing when each batch returns.
func TestPolicyFinalReplayPreservesPreAdmittedBatchOrder(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	completed := make(chan uint64, 2)
	var first sync.Once
	r, source, destination, conn, ctxID := newSyncOrderRailWith(t, func(key fwdKey, items []fwdItem) {
		var ids []uint64
		for i := range items {
			if id := items[i].sourceMessageID; id == 9201 || id == 9202 {
				ids = append(ids, id)
			}
		}
		first.Do(func() {
			close(entered)
			<-release
		})
		fwdBatchHandler(key, items)
		for _, id := range ids {
			completed <- id
		}
	})
	t.Cleanup(unblock)
	destination.sendingInitialRoutes.Store(0)
	api := &reactorAPIAdapter{r: r}
	session := destination.currentSession()
	func() {
		session.writeMu.Lock()
		defer session.writeMu.Unlock()
		withdraw := syncOrderPublish(t, r, ctxID, 9201, syncOrderWithdrawBody)
		if err := api.forwardUpdateCore(withdraw, 9201, []*Peer{destination}, replayFenceSource(source, false)); err != nil {
			t.Fatal(err)
		}
		<-entered
		announce := syncOrderPublish(t, r, ctxID, 9202, syncOrderAnnounceBody)
		if err := api.forwardUpdateCore(announce, 9202, []*Peer{destination}, replayFenceSource(source, false)); err != nil {
			t.Fatal(err)
		}
	}()
	destination.beginPolicyReplay(session)
	releaseHold := sync.OnceFunc(destination.endPolicyReplay)
	t.Cleanup(releaseHold)
	unblock()
	if firstID, secondID := <-completed, <-completed; firstID != 9201 || secondID != 9202 {
		t.Fatalf("fixture did not admit distinct ordered batches: %d, %d", firstID, secondID)
	}
	releaseHold()
	if err := r.fwdPool.barrierPeer(t.Context(), destination.Settings().PeerKey()); err != nil {
		t.Fatal(err)
	}
	held, announced := false, false
	for _, update := range parseWireUpdates(t, conn.written()) {
		if update.announces {
			held, announced = true, true
		}
		if update.withdraws {
			held = false
		}
	}
	if !announced || !held {
		t.Fatalf("pre-admitted withdraw/announce batches were reversed during replay: %x", conn.written())
	}
}

// TestPolicyFinalDynamicRestoreFailureRollsBack proves the dynamic transaction
// restores both its template and running peer when retained-source replay fails.
func TestPolicyFinalDynamicRestoreFailureRollsBack(t *testing.T) {
	group := dynamicReloadGroup(t)
	group.Settings.ExportPolicy = mustPolicy(t, nil, filterapi.FinalReject)
	group.Settings.ProcessBindings = []ProcessBinding{operatorBinding(t, "bgp-rs", "state", "update")}
	r, destination := newDynamicReloadReactor(t, group, netip.MustParseAddr("185.1.69.42"), nil)
	establishDynamicPeer(t, r, destination)
	api, _ := readyProducerServer(t, r, "bgp-rs")
	r.api = api.server
	policyStopReplayStore(t, r.api, destination.Settings().Address)
	next := dynamicReloadGroup(t)
	next.Settings.ExportPolicy = mustPolicy(t, nil, filterapi.FinalAccept)
	next.Settings.ProcessBindings = group.Settings.ProcessBindings
	err := api.reconcileDynamicGroupsJournaled([]*DynamicGroupConfig{next}, "test", &internalJournal{})
	if err == nil {
		t.Fatal("dynamic reload committed despite unavailable retained-source replay")
	}
	if !strings.Contains(err.Error(), pluginserver.ErrPluginProcessNotRunning.Error()) {
		t.Fatalf("dynamic reload lost reconciliation failure: %v", err)
	}
	if destination.ExportPolicy().AcceptsAtEnd() {
		t.Fatal("failed dynamic apply left accepting policy installed")
	}
	if len(r.dynamicGroups) != 1 || r.dynamicGroups[0] != group {
		t.Fatal("failed dynamic apply left the new group installed")
	}
}
