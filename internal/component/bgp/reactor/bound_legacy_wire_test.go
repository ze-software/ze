// Design: docs/architecture/api/process-protocol.md -- exact legacy SENT destination.
// Related: forward_provenance_ownership_test.go -- original forwarded SENT receipts.
package reactor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/route_refresh"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const boundLegacyProcess = "bound-legacy-maintainer"

// boundLegacyWireFixture runs one real SDK lifecycle and the production cached
// forward writer. Two original Peers share an address but have distinct ports,
// owners, Sessions and real pipes. It owns no extra control map or worker.
// Cleanup MUST stop/join the SDK before the borrowed existing peer readers end.
type boundLegacyWireFixture struct {
	rail          *ownershipRailFixture
	host          *pluginserver.Server
	client        *sdk.Plugin
	process       *process.Process
	peers         [2]*Peer
	wires         [2]*selectedRefreshCleanupWire
	first         [2][]byte
	requests      [2]rpc.UpdateRouteInput
	work          context.Context
	rib           *process.Process
	sourceSession *Session
}

func newBoundLegacyWireFixture(t *testing.T, socket, holdSecondClose bool) *boundLegacyWireFixture {
	return newBoundLegacyWireRIBFixture(t, socket, holdSecondClose, false)
}

func newBoundLegacyWireRIBFixture(t *testing.T, socket, holdSecondClose, withRIB bool) *boundLegacyWireFixture {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, events.RegisterNamespace("bgp", "update", "state", "refresh"))
	rail := ownershipRailNew(t, nil)
	rail.r.config = &Config{LocalAS: 65000}
	rail.r.clock = clock.RealClock{}
	first := rail.destination
	first.settings.Name = "bound-A"
	first.session.initialReplay.Store(301)
	twin, _ := newSyncOrderDest(t, first.sendCtx.Load(), rail.ctxID)
	twin.settings.Port = 1179
	twin.settings.Name = "bound-B"
	twin.session.initialReplay.Store(302)
	twin.testInitialSync().sendingInitialRoutes.Store(0)
	twin.SetReactor(rail.r)
	rail.r.peers[twin.Settings().PeerKey()] = twin
	work, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)
	fixture := &boundLegacyWireFixture{rail: rail, peers: [2]*Peer{first, twin}, work: work}
	grant := ProcessBinding{PluginName: boundLegacyProcess, Explicit: true,
		Receive: map[string]events.Direction{"state": events.DirBoth, "refresh": events.DirBoth, "update": events.DirSent},
		Send:    map[string]bool{"update": true, "refresh": true}}
	ribGrant := ProcessBinding{PluginName: "bgp-rib", Explicit: true,
		Receive: map[string]events.Direction{"state": events.DirBoth, "refresh": events.DirBoth, "update": events.DirSent},
		Send:    map[string]bool{"update": true, "refresh": true}}
	if withRIB {
		rail.source.settings.Name = "bound-source"
		rail.source.settings.ProcessBindings = append(rail.source.settings.ProcessBindings, sourceLeaseGrant("bgp-rib"))
		rail.source.advanceSourcePolicyEpoch()
		fixture.sourceSession = sourceSessionFixture(t, rail.source)
		require.NoError(t, fixture.sourceSession.captureSourceReceipt(rail.source, &sourceReceipts))
		t.Cleanup(fixture.sourceSession.closeSourceReceipt)
	}
	for index, peer := range fixture.peers {
		peer.settings.ProcessBindings = append(peer.settings.ProcessBindings, grant)
		if withRIB {
			peer.settings.ProcessBindings = append(peer.settings.ProcessBindings, ribGrant)
		}
		peer.negotiated.Load().EnhancedRouteRefresh = true
		peer.negotiated.Load().RouteRefresh = true
		peer.session.negotiated.EnhancedRouteRefresh = true
		peer.session.negotiated.RouteRefresh = true
		fixture.wires[index] = selectedRefreshCleanupPeerWire(t, peer.session, holdSecondClose && index == 1)
	}
	declaration := []rpc.DisseminationDecl{{Namespace: "bgp", Role: SelectedDisseminationRole,
		SourceReceive:      []string{"update-received", "state"},
		DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"}}}
	ready := make(chan *sdk.Plugin, 1)
	done := make(chan struct{})
	const implementation = "bound-legacy-maintenance-runner"
	require.NoError(t, registry.Register(registry.Registration{Name: implementation,
		Claims: []string{SelectedDisseminationRole}, Dissemination: declaration,
		CLIHandler: func([]string) int { return 0 }, RunEngine: func(conn net.Conn) int {
			defer close(done)
			if socket {
				conn = selectedRefreshSocketConn{conn}
			}
			client := sdk.NewWithConn(boundLegacyProcess, conn)
			defer func() { _ = client.Close() }() //nolint:errcheck // Host shutdown owns this connection.
			client.OnConfigure(func([]sdk.ConfigSection) error { return nil })
			client.OnStarted(func(context.Context) error { ready <- client; return nil })
			_ = client.Run(work, &sdk.Registration{Claims: []string{SelectedDisseminationRole}, Dissemination: declaration}) //nolint:errcheck // The joined host stop is this fixture's terminal path.
			return 0
		}}))
	plugins := []plugin.PluginConfig{{Name: boundLegacyProcess, Run: implementation, Internal: true, Encoder: plugin.EncodingJSON}}
	var ribDone chan struct{}
	if withRIB {
		actual := registry.Lookup("bgp-rib")
		require.NotNil(t, actual, "this is the registered production RIB implementation")
		require.NotNil(t, actual.RunEngine)
		ribDone = make(chan struct{})
		require.NoError(t, registry.Register(registry.Registration{Name: "bound-actual-rib-runner",
			CLIHandler: func([]string) int { return 0 }, Commands: actual.Commands, Dissemination: actual.Dissemination,
			SignalsSessionReady: actual.SignalsSessionReady, FencesLiveForwards: actual.FencesLiveForwards,
			RunEngine: func(conn net.Conn) int {
				defer close(ribDone)
				if socket {
					conn = selectedRefreshSocketConn{conn}
				}
				return actual.RunEngine(conn)
			}}))
		plugins = append(plugins, plugin.PluginConfig{Name: "bgp-rib", Run: "bound-actual-rib-runner", Internal: true, Encoder: plugin.EncodingJSON})
	}
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{}),
		Plugins: plugins}, &reactorAPIAdapter{r: rail.r})
	require.NoError(t, err)
	host.SetProcessSpawner(&selectedRefreshSpawner{ctx: work})
	graph := []pluginserver.DeliveryPeer{
		{Addr: rail.source.addrString, Bindings: []plugin.PeerProcessBinding{{PluginName: boundLegacyProcess, Explicit: true,
			Receive: map[string]events.Direction{"update": events.DirReceived, "state": events.DirBoth}}}},
		{Name: first.settings.Name, Addr: first.addrString, Bindings: []plugin.PeerProcessBinding{plugin.PeerProcessBinding(grant)}},
		{Name: twin.settings.Name, Addr: twin.addrString, Bindings: []plugin.PeerProcessBinding{plugin.PeerProcessBinding(grant)}},
	}
	if withRIB {
		graph[0].Bindings = append(graph[0].Bindings, plugin.PeerProcessBinding(sourceLeaseGrant("bgp-rib")))
		graph[1].Bindings = append(graph[1].Bindings, plugin.PeerProcessBinding(ribGrant))
		graph[2].Bindings = append(graph[2].Bindings, plugin.PeerProcessBinding(ribGrant))
	}
	host.UpdateDeliveryGraph("bgp", graph)
	rail.r.api = host
	if withRIB {
		rail.r.setMessageReceiver(bgpserver.NewEventDispatcher(host))
	}
	require.NoError(t, host.StartWithContext(work))
	t.Cleanup(func() {
		host.Stop()
		joined, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer stop()
		require.NoError(t, host.Wait(joined))
		select {
		case <-done:
		case <-joined.Done():
			t.Error("bound legacy SDK runner did not join")
		}
		if ribDone != nil {
			select {
			case <-ribDone:
			case <-joined.Done():
				t.Error("actual registered RIB runner did not join")
			}
		}
	})
	require.NoError(t, host.WaitForStartupComplete(work))
	select {
	case fixture.client = <-ready:
	case <-work.Done():
		t.Fatal("bound legacy SDK never became ready")
	}
	fixture.host = host
	fixture.process = host.ProcessManager().GetProcess(boundLegacyProcess)
	require.NotNil(t, fixture.process)
	require.Equal(t, !socket, fixture.process.Conn().HasBridge())
	for _, peer := range fixture.peers {
		peer.refreshForwardFacts()
		state := peer.selectedOwnerSnapshot()
		require.NotNil(t, state)
		require.NoError(t, state.err)
		require.True(t, state.owner.Explicit(), "legacy role maintenance must not borrow ordinary selected authority")
	}
	body := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 41)
	rail.publish(t, rail.source, rail.ctxID, body)
	if withRIB {
		fixture.rib = host.ProcessManager().GetProcess("bgp-rib")
		require.NotNil(t, fixture.rib)
		require.Equal(t, !socket, fixture.rib.Conn().HasBridge())
		for _, peer := range fixture.peers {
			fixture.deliverRIBUp(t, peer, peer.session)
		}
		fixture.deliverRIBUp(t, rail.source, fixture.sourceSession)
		parsed, parseErr := message.UnpackUpdate(body)
		require.NoError(t, parseErr)
		source := rail.source
		event := fmt.Sprintf(`{"type":"bgp","bgp":{"message":{"type":"update","id":%d,"direction":"received"},"peer":{"name":%q,"remote":{"address":%q,"as":65001}},"connection-id":%q,"source-policy-epoch":%d,"source-receipt":%d,"peer-owner":%d,"raw":{"attributes":%q,"nlri":{"ipv4/unicast":%q}},"update":{"attr":{"origin":"igp"},"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":[%q]}]}}}`,
			rail.id, source.settings.Name, source.addrString, strconv.FormatUint(fixture.sourceSession.connectionID, 10), source.sourcePolicyEpoch.Load(), fixture.sourceSession.sourceReceiptID(), source.sourceOwner, hex.EncodeToString(parsed.PathAttributes), hex.EncodeToString(parsed.NLRI), syncOrderPrefix)
		require.NoError(t, fixture.rib.Conn().SendDeliverEvent(work, event))
		require.NoError(t, fixture.rib.DrainEventsApplied(work))
		require.Equal(t, 1, fixture.ribCount(t, "received", source.settings.Name), "the actual registered RIB acquired useful original source state")
	}
	api := &reactorAPIAdapter{r: rail.r}
	require.NoError(t, api.ForwardUpdatesDirect([]uint64{rail.id}, []netip.AddrPort{first.Settings().PeerKey(), twin.Settings().PeerKey()}, "route-server", plugin.OperatorSender()))
	for index, peer := range fixture.peers {
		frame := fixture.wires[index].next(t, work)
		require.NoError(t, frame.err)
		require.GreaterOrEqual(t, len(frame.data), 19)
		parsed, err := message.UnpackUpdate(frame.data[19:])
		require.NoError(t, err)
		require.Equal(t, syncOrderPrefixWire, parsed.NLRI, "both original peers must have useful actual SENT state")
		fixture.first[index] = append([]byte(nil), frame.data...)
		view := *rail
		view.destination = peer
		captured := ownershipReplayBatch(t, &view, body)
		command := bgp.FormatAnnounceCommand(&bgp.Route{Family: family.IPv4Unicast, Prefix: syncOrderPrefix,
			NextHop: "10.0.0.1", RawAttrs: hex.EncodeToString(parsed.PathAttributes)})
		fixture.requests[index] = rpc.UpdateRouteInput{PeerSelector: peer.addrString, Command: command,
			DestinationOwner: peer.sourceOwner, DestinationSession: peer.session.initialReplay.Load(),
			SourcePolicyEpoch: captured.SourcePolicyEpoch, Meta: map[string]any{"replay": true, "sent-owner-message": strconv.FormatUint(captured.SentOwnerMessage, 10)}}
	}
	if withRIB {
		require.NoError(t, fixture.rib.DrainEventsApplied(work))
		for _, peer := range fixture.peers {
			require.Equal(t, 1, fixture.ribCount(t, "sent", peer.settings.Name), "the final writer's actual SENT event populated this exact RIB destination")
		}
	}
	return fixture
}

func (fixture *boundLegacyWireFixture) deliverRIBUp(t *testing.T, peer *Peer, session *Session) {
	t.Helper()
	event := fmt.Sprintf(`{"type":"state","state":"up","reason":"attached","peer-owner":%d,"connection-id":%q,"bgp.destination-session":%q,"peer":{"name":%q,"remote":{"address":%q,"as":65001}}}`,
		peer.sourceOwner, strconv.FormatUint(session.connectionID, 10), strconv.FormatUint(session.initialReplay.Load(), 10), peer.settings.Name, peer.addrString)
	require.NoError(t, fixture.rib.Conn().SendDeliverEvent(fixture.work, event))
	require.NoError(t, fixture.rib.DrainEventsApplied(fixture.work))
}

func (fixture *boundLegacyWireFixture) ribCount(t *testing.T, direction, name string) int {
	t.Helper()
	output, err := fixture.rib.Conn().SendExecuteCommand(fixture.work, &rpc.ExecuteCommandInput{Serial: "bound-rib-count", Command: "show bgp rib", Args: []string{direction, "peer", name, "count"}, Peer: "*"})
	require.NoError(t, err)
	require.Equal(t, rpc.StatusDone, output.Status, string(output.Data))
	var answer struct {
		Count int `json:"count"`
	}
	require.NoError(t, json.Unmarshal(output.Data, &answer))
	return answer.Count
}

func (fixture *boundLegacyWireFixture) marker(t *testing.T, index int, action string) {
	t.Helper()
	route := fixture.requests[index]
	_, err := fixture.client.UpdateRouteRequestResult(fixture.work, rpc.UpdateRouteInput{PeerSelector: route.PeerSelector,
		DestinationOwner: route.DestinationOwner, DestinationSession: route.DestinationSession, PeerMarker: action, MarkerFamily: "ipv4/unicast"})
	require.NoError(t, err)
	frame := fixture.wires[index].next(t, fixture.work)
	require.NoError(t, frame.err)
	require.Equal(t, []string{action}, refreshWireLabels(t, frame.data))
}

// A bound legacy/SENT request for same-address B must reach only its original
// pipe. A's later same-pipe marker fence detects even wrongly queued output.
// The generic explicit role is real; no claim about the RS selection algorithm
// is made by this maintenance boundary question.
// MUTATION: drop destination narrowing before peer selection or final write.
func TestBoundLegacyReplayKeepsSameAddressSiblingWire(t *testing.T) {
	for _, socket := range []bool{false, true} {
		name := "native"
		if socket {
			name = "framed"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newBoundLegacyWireFixture(t, socket, false)
			fixture.marker(t, 1, "borr")
			output, err := fixture.client.UpdateRouteRequestResult(fixture.work, fixture.requests[1])
			require.NoError(t, err)
			require.NotNil(t, output)
			frame := fixture.wires[1].next(t, fixture.work)
			require.NoError(t, frame.err)
			require.Equal(t, fixture.first[1], frame.data, "the original useful attributes and prefix survive SENT replay")
			fixture.marker(t, 1, "eorr")
			fixture.marker(t, 0, "borr")
			fixture.marker(t, 0, "eorr")
			require.Nil(t, fixture.peers[0].session.selectedRefresh.Load())
			require.Nil(t, fixture.peers[1].session.selectedRefresh.Load())
		})
	}
}

// Complete and half-stale constraints must refuse before touching either
// physical destination; the valid B request and A marker fence stay useful.
func TestBoundLegacyReplayRefusesStaleConstraintWithoutRebinding(t *testing.T) {
	for _, socket := range []bool{false, true} {
		name := "native"
		if socket {
			name = "framed"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newBoundLegacyWireFixture(t, socket, false)
			for _, mutate := range []func(*rpc.UpdateRouteInput){
				func(input *rpc.UpdateRouteInput) { input.DestinationOwner = 0 },
				func(input *rpc.UpdateRouteInput) { input.DestinationSession = 0 },
				func(input *rpc.UpdateRouteInput) { input.DestinationOwner = fixture.requests[0].DestinationOwner },
				func(input *rpc.UpdateRouteInput) { input.DestinationSession++ },
				func(input *rpc.UpdateRouteInput) { input.PeerSelector = "192.0.2.250" },
			} {
				invalid := fixture.requests[1]
				mutate(&invalid)
				_, err := fixture.client.UpdateRouteRequestResult(fixture.work, invalid)
				require.Error(t, err)
			}
			output, err := fixture.client.UpdateRouteRequestResult(fixture.work, fixture.requests[1])
			require.NoError(t, err)
			require.NotNil(t, output)
			frame := fixture.wires[1].next(t, fixture.work)
			require.NoError(t, frame.err)
			require.Equal(t, fixture.first[1], frame.data)
			fixture.marker(t, 1, "borr")
			fixture.marker(t, 1, "eorr")
			fixture.marker(t, 0, "borr")
			fixture.marker(t, 0, "eorr")
		})
	}
}

// A completed legacy Begin retains its authenticated requesting Process. Stop
// must join the original actual Close while a useful same-address sibling's
// independent pipe remains live. The source-owned SENT routes predate Begin.
// MUTATION: omit recordSelectedRefresh from the bound legacy marker writer.
func TestBoundLegacyBeginProcessStopJoinsOnlyOriginalWire(t *testing.T) {
	for _, socket := range []bool{false, true} {
		name := "native"
		if socket {
			name = "framed"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newBoundLegacyWireFixture(t, socket, true)
			wire := fixture.wires[1]
			fixture.marker(t, 1, "borr")
			require.NotNil(t, fixture.peers[1].session.selectedRefresh.Load())
			completed := make(chan error, 1)
			joined := make(chan struct{})
			fixture.process.Stop()
			go func() { defer close(joined); completed <- fixture.process.WaitRuntimeCleanupContext(fixture.work) }()
			t.Cleanup(func() {
				wire.release()
				cleanup, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
				defer stop()
				select {
				case <-joined:
				case <-cleanup.Done():
					t.Error("bound Process cleanup did not join")
				}
			})
			select {
			case <-wire.closing:
			case err := <-completed:
				t.Fatalf("bound Begin cleanup ended without original Close: %v", err)
			case <-fixture.work.Done():
				t.Fatal("bound Begin cleanup never entered Close")
			}
			select {
			case err := <-completed:
				t.Fatalf("bound Begin cleanup returned before held original Close completed: %v", err)
			default:
			}
			require.NoError(t, fixture.peers[0].session.writeMessage(fixture.wires[0], &message.Keepalive{}))
			useful := fixture.wires[0].next(t, fixture.work)
			require.NoError(t, useful.err)
			require.Equal(t, byte(4), useful.data[18])
			wire.release()
			select {
			case err := <-completed:
				require.NoError(t, err)
			case <-fixture.work.Done():
				t.Fatal("bound Begin cleanup did not join actual Close")
			}
			ended := wire.next(t, fixture.work)
			require.ErrorIs(t, ended.err, io.EOF)
			require.NoError(t, fixture.host.Context().Err())
			require.NoError(t, fixture.peers[0].session.writeMessage(fixture.wires[0], &message.Keepalive{}))
			survivor := fixture.wires[0].next(t, fixture.work)
			require.NoError(t, survivor.err)
			require.Equal(t, byte(4), survivor.data[18])
		})
	}
}

// A captured old write meets its final producer only after successor
// publication. The old socket remains physically useful, so refusal must come
// from current-session identity; the successor then receives a useful route.
// This final-writer seam is distinct from the server-admission/SDK-UP schedule.
// MUTATION: remove the establishedSession comparison from final bound writers.
func TestBoundFinalWriterRefusesRetiredOpenSession(t *testing.T) {
	for _, socket := range []bool{false, true} {
		name := "native"
		if socket {
			name = "framed"
		}
		t.Run(name, func(t *testing.T) {
			for _, kind := range []string{"route", "marker"} {
				t.Run(kind, func(t *testing.T) {
					fixture := newBoundLegacyWireFixture(t, socket, false)
					peer := fixture.peers[1]
					original := peer.session
					target := announceTarget{peer: peer, session: original, destinationOwner: peer.sourceOwner,
						destinationSession: original.initialReplay.Load(), exportGeneration: peer.forwardFacts().exportGeneration}
					oldBody := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 41)
					view := *fixture.rail
					view.destination = peer
					captured := ownershipReplayBatch(t, &view, oldBody)
					update, err := message.UnpackUpdate(oldBody)
					require.NoError(t, err)
					successor := NewSession(peer.settingsSnapshot())
					successor.adjOut = &peer.adjOut
					successor.SetSourceID(peer.SourceID())
					successor.setSendCtxID(original.sendCtxID)
					successor.negotiated = original.negotiated
					successor.initialReplay.Store(303)
					require.NoError(t, successor.fsm.Event(fsm.EventManualStart))
					require.NoError(t, successor.fsm.Event(fsm.EventTCPConnectionConfirmed))
					require.NoError(t, successor.fsm.Event(fsm.EventBGPOpen))
					require.NoError(t, successor.fsm.Event(fsm.EventKeepaliveMsg))
					t.Cleanup(successor.timers.StopAll)
					nextWire := selectedRefreshCleanupPeerWire(t, successor, false)
					peer.mu.Lock()
					peer.session = successor
					peer.mu.Unlock()
					peer.setTestInitialSync(0, false)
					peer.refreshForwardFacts()
					require.False(t, original.tearingDown.Load(), "the final identity guard must work before the old socket closes")
					if kind == "route" {
						err = target.sendBatchUpdate(fixture.work, update, 4096, false, captured)
					} else {
						owner, ownerErr := fixture.process.SelectedOwner()
						require.NoError(t, ownerErr)
						err = original.sendBoundMarker(fixture.work, &message.RouteRefresh{AFI: 1, SAFI: 1, Subtype: message.RouteRefreshBoRR},
							target.destinationSession, owner, peer)
					}
					require.ErrorIs(t, err, errAdjOutSession)
					require.NoError(t, original.writeMessage(fixture.wires[1], &message.Keepalive{}))
					oldControl := fixture.wires[1].next(t, fixture.work)
					require.NoError(t, oldControl.err)
					require.Equal(t, byte(4), oldControl.data[18], "no stale route or marker may precede the old-socket control")
					body := ownershipRailBody(family.IPv4Unicast, false, syncOrderPrefixWire, 42)
					fixture.rail.publish(t, fixture.rail.source, fixture.rail.ctxID, body)
					api := &reactorAPIAdapter{r: fixture.rail.r}
					require.NoError(t, api.ForwardUpdatesDirect([]uint64{fixture.rail.id}, []netip.AddrPort{peer.Settings().PeerKey()}, "route-server", plugin.OperatorSender()))
					frame := nextWire.next(t, fixture.work)
					require.NoError(t, frame.err)
					useful, decodeErr := message.UnpackUpdate(frame.data[19:])
					require.NoError(t, decodeErr)
					require.Equal(t, syncOrderPrefixWire, useful.NLRI)
					require.Same(t, successor, peer.currentSession())
				})
			}
		})
	}
}

// A real SDK EOF between successful Begin and End must retire and join the
// original pipe exactly as Stop does. The same-address sibling remains useful.
// MUTATION: omit recordSelectedRefresh from the bound legacy marker writer.
func TestBoundLegacyBeginEOFJoinsOnlyOriginalWire(t *testing.T) {
	for _, socket := range []bool{false, true} {
		name := "native"
		if socket {
			name = "framed"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newBoundLegacyWireFixture(t, socket, true)
			wire := fixture.wires[1]
			fixture.marker(t, 1, "borr")
			require.NotNil(t, fixture.peers[1].session.selectedRefresh.Load())
			completed := make(chan error, 1)
			joined := make(chan struct{})
			require.NoError(t, fixture.client.Close())
			go func() { defer close(joined); completed <- fixture.process.WaitRuntimeCleanupContext(fixture.work) }()
			t.Cleanup(func() {
				wire.release()
				cleanup, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
				defer stop()
				select {
				case <-joined:
				case <-cleanup.Done():
					t.Error("bound Process cleanup did not join")
				}
			})
			select {
			case <-wire.closing:
			case err := <-completed:
				t.Fatalf("bound Begin cleanup ended without original Close: %v", err)
			case <-fixture.work.Done():
				t.Fatal("bound Begin cleanup never entered Close")
			}
			select {
			case err := <-completed:
				t.Fatalf("bound Begin cleanup returned before held original Close completed: %v", err)
			default:
			}
			require.NoError(t, fixture.peers[0].session.writeMessage(fixture.wires[0], &message.Keepalive{}))
			useful := fixture.wires[0].next(t, fixture.work)
			require.NoError(t, useful.err)
			require.Equal(t, byte(4), useful.data[18])
			wire.release()
			select {
			case err := <-completed:
				require.NoError(t, err)
			case <-fixture.work.Done():
				t.Fatal("bound Begin cleanup did not join actual Close")
			}
			ended := wire.next(t, fixture.work)
			require.ErrorIs(t, ended.err, io.EOF)
			require.NoError(t, fixture.host.Context().Err())
			require.NoError(t, fixture.peers[0].session.writeMessage(fixture.wires[0], &message.Keepalive{}))
			survivor := fixture.wires[0].next(t, fixture.work)
			require.NoError(t, survivor.err)
			require.Equal(t, byte(4), survivor.data[18])
		})
	}
}
