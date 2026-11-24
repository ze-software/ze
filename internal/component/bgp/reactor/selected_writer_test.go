// Design: docs/architecture/update-building.md -- selected output and final receipts.
// Related: reactor_api_selected.go -- the real selected-operation entry point.
package reactor

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	policy "github.com/ze-software/ze/internal/component/policy"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const selectedWriterProcess = "selected-writer-test"

// selectedWriterFixture owns actual source leases and an Established writer.
// Its published-current callback is independent of any process manager, just
// as an authenticated owner's captured connection is in production.
type selectedWriterFixture struct {
	api     *reactorAPIAdapter
	peer    *Peer
	conn    *recordingConn
	owner   plugin.SelectedOwner
	mu      sync.Mutex
	cut     uint64
	current map[string]uint64
}

func newSelectedWriterFixture(t *testing.T, addPath bool) *selectedWriterFixture {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, events.RegisterNamespace("bgp", "update", "state", "refresh"))
	require.NoError(t, registry.Register(registry.Registration{
		Name: selectedWriterProcess, RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 },
		Sends: testAllBGPSendDecls(), Receives: testAllBGPReceiveDecls(),
		Dissemination: []rpc.DisseminationDecl{{Namespace: "bgp", Role: SelectedDisseminationRole,
			SourceReceive:      []string{"update-received", "state"},
			DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"}}},
	}))
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: []plugin.PluginConfig{{Name: selectedWriterProcess, Internal: true}}}, nil)
	require.NoError(t, err)
	peer, conn := newGroupUpdatesPeer(t, "192.0.2.80", "absent")
	peer.settings.LocalAddress = netip.MustParseAddr("192.0.2.254")
	peer.settings.ExportPolicy, err = filterapi.NewPolicy(nil, policy.FinalAccept)
	require.NoError(t, err)
	peer.settings.Bus = testBusSelectedDest(selectedWriterProcess)
	encoding := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: addPath})
	contextID, err := bgpctx.Registry.Register(encoding)
	require.NoError(t, err)
	peer.sendCtx.Store(encoding)
	peer.sendCtxID, peer.session.sendCtxID = contextID, contextID
	peer.session.initialReplay.Store(71)
	peer.session.egressFinalAccepts = func(kind filterapi.ExportKind) bool {
		return peer.forwardFacts().exportPolicy.AcceptsExportAtEnd(true, kind)
	}
	host.UpdateDeliveryGraph("bgp", []pluginserver.DeliveryPeer{
		testDeliveryPeer("192.0.2.79", "", testBusSourceLease(selectedWriterProcess)),
		testDeliveryPeer("192.0.2.80", "", testBusSelectedDest(selectedWriterProcess)),
	})
	pool := newFwdPool(func(fwdKey, []fwdItem) { t.Error("selected work escaped to a detached forwarding worker") }, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)
	pool.registerOutgoingPool(fwdKey{peerAddr: peer.Settings().PeerKey()}, 4096)
	r := &Reactor{api: host, config: &Config{LocalAS: 65000}, clock: clock.RealClock{},
		attrModHandlers: attrModHandlersWithDefaults(), recentUpdates: newRecentUpdateCache(100),
		fwdPool: pool, peers: map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer}}
	peer.reactor = r
	peer.refreshForwardFacts()
	f := &selectedWriterFixture{api: &reactorAPIAdapter{r: r}, peer: peer, conn: conn, current: make(map[string]uint64)}
	done := make(chan struct{})
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), done, nil)
	require.NoError(t, err)
	t.Cleanup(func() { close(done) })
	f.owner, err = owner.WithCurrentSelection(func(_ context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		answer := &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession,
			Cut: f.cut, Results: make([]rpc.CurrentSelectionResult, len(input.Keys))}
		for i, key := range input.Keys {
			result := rpc.CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI, Revision: key.Revision, Presence: rpc.CurrentSelectionAbsent}
			if revision := f.current[string(key.KeyNLRI)]; revision != 0 {
				result.Presence, result.CurrentRevision = rpc.CurrentSelectionPresent, revision
			}
			answer.Results[i] = result
		}
		return answer, nil
	})
	require.NoError(t, err)
	return f
}

func (f *selectedWriterFixture) route(t *testing.T, revision uint64, sourceAddress string, pathID uint32, sourceAddPath bool) rpc.SelectedRoute {
	t.Helper()
	address := netip.MustParseAddr(sourceAddress)
	admission, err := filterapi.NewPolicy(nil, policy.FinalAccept)
	require.NoError(t, err)
	peer := NewPeer(&PeerSettings{Address: address, LocalAS: 65000, PeerAS: 65003, ImportPolicy: admission})
	peer.state.Store(int32(PeerStateEstablished))
	peer.advanceForwardGeneration()
	source := relaySource{addr: address, strAdr: sourceAddress, srcID: peer.SourceID(),
		generation: peer.forwardGeneration.Load(), ok: true,
		info: forwardSourceInfo{resolved: true, peer: peer, remoteRouterID: 71, globalLocalAS: 65000,
			filterInfo: filterapi.PeerFilterInfo{Address: address, PeerAS: 65003, LocalAS: 65000}}}
	contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: sourceAddPath}))
	require.NoError(t, err)
	source.ctxID = contextID
	facts, err := newSourceReceiptFacts(&source, peer.newSession().connectionID)
	require.NoError(t, err)
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	lease, err := sourceReceipts.acquire(live.ID(), f.owner)
	require.NoError(t, err)
	t.Cleanup(func() {
		live.close()
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()
		require.NoError(t, lease.revoke(cleanup))
	})
	attrs := synthesizedNextHopAttrs(netip.MustParseAddr(sourceAddress), 65003)
	key := []byte{24, 198, 18, 7}
	return rpc.SelectedRoute{Family: family.IPv4Unicast.String(), KeyNLRI: key, Revision: revision,
		Kind: rpc.SelectedRouteReceived, SourceReceipt: live.ID(), SourceAddPath: sourceAddPath, StaleLevel: rpc.SelectedFresh,
		Route: &rpc.StoredRoute{Family: family.IPv4Unicast.String(), SourcePeer: sourceAddress,
			MsgID: 19, SourcePolicyEpoch: peer.sourcePolicyEpoch.Load(), NLRIFraming: rpc.NLRIFramingPrefixOnly, PathID: pathID,
			NLRIHex: hex.EncodeToString(key), AttrHex: hex.EncodeToString(attrs), NextHopHex: hex.EncodeToString(netip.MustParseAddr(sourceAddress).AsSlice())}}
}

func (f *selectedWriterFixture) send(t *testing.T, ctx context.Context, route rpc.SelectedRoute) (*rpc.SelectRoutesOutput, error) {
	t.Helper()
	f.mu.Lock()
	f.cut = route.Revision
	if route.Kind == rpc.SelectedRouteAbsent {
		delete(f.current, string(route.KeyNLRI))
	} else {
		f.current[string(route.KeyNLRI)] = route.Revision
	}
	f.mu.Unlock()
	batch, err := plugin.NewSelectedBatch(rpc.SelectRoutesInput{Destination: f.peer.addrString,
		DestinationSession: f.peer.session.initialReplay.Load(), Routes: []rpc.SelectedRoute{route}})
	if err != nil {
		return nil, err
	}
	return f.api.SelectRoutes(ctx, batch, f.owner)
}

// TestSelectedWriterReplacementRetiresOriginalSource observes the real final
// socket: a globally selected replacement overwrites A, then absence withdraws B.
func TestSelectedWriterReplacementRetiresOriginalSource(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	first := f.route(t, 1, "192.0.2.79", 0, false)
	answer, err := f.send(t, t.Context(), first)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	second := f.route(t, 2, "192.0.2.78", 0, false)
	answer, err = f.send(t, t.Context(), second)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	body := updateBodies(t, f.conn.written())
	require.Len(t, body, 2)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, body[1], false))
	absent := rpc.SelectedRoute{Family: first.Family, KeyNLRI: first.KeyNLRI, Revision: 3, Kind: rpc.SelectedRouteAbsent}
	answer, err = f.send(t, t.Context(), absent)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedAbsent, answer.Results[0].Outcome)
	body = updateBodies(t, f.conn.written())
	require.Len(t, body, 3)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, body[2], true))
	require.Empty(t, f.peer.adjOut.routes)
	require.Empty(t, f.peer.adjOut.selectedPaths)
	require.Empty(t, f.peer.adjOut.tombstones)
}

// TestSelectedWriterPolicyExclusionRetiresOldOutput keeps the previously
// advertised winner useful, then proves a denied new winner removes it.
func TestSelectedWriterPolicyExclusionRetiresOldOutput(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	first := f.route(t, 1, "192.0.2.79", 0, false)
	_, err := f.send(t, t.Context(), first)
	require.NoError(t, err)
	require.Len(t, updateBodies(t, f.conn.written()), 1)
	f.peer.settings.ExportPolicy, err = filterapi.NewPolicy(nil, policy.FinalReject)
	require.NoError(t, err)
	f.peer.refreshForwardFacts()
	answer, err := f.send(t, t.Context(), f.route(t, 2, "192.0.2.78", 0, false))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, answer.Results[0].Outcome)
	body := updateBodies(t, f.conn.written())
	require.Len(t, body, 2)
	require.Empty(t, updatePrefixes(t, body[1], false))
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, body[1], true))
	require.Empty(t, f.peer.adjOut.routes)
}

// TestSelectedWriterSourcePathZeroReplacement retires the exact old outgoing
// ADD-PATH identifier even when its original source identifier was zero.
func TestSelectedWriterSourcePathZeroReplacement(t *testing.T) {
	f := newSelectedWriterFixture(t, true)
	_, err := f.send(t, t.Context(), f.route(t, 1, "192.0.2.79", 0, true))
	require.NoError(t, err)
	first := updateBodies(t, f.conn.written())
	require.Len(t, first, 1)
	oldID := binary.BigEndian.Uint32(first[0][4+int(binary.BigEndian.Uint16(first[0][2:4])):])
	_, err = f.send(t, t.Context(), f.route(t, 2, "192.0.2.78", 7, true))
	require.NoError(t, err)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 3, "replacement announce and exact old-path withdrawal")
	withdraw := bodies[2][2 : 2+int(binary.BigEndian.Uint16(bodies[2][:2]))]
	require.Equal(t, oldID, binary.BigEndian.Uint32(withdraw))
	require.Equal(t, []byte{24, 198, 18, 7}, withdraw[4:])
	require.Len(t, f.peer.adjOut.routes, 1)
}

// TestSelectedWriterCanceledFlushJoins uses an actual unbuffered socket flush;
// no final receipt may escape while its write remains live.
func TestSelectedWriterCanceledFlushJoins(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	writer, remote := net.Pipe()
	t.Cleanup(func() { _ = writer.Close(); _ = remote.Close() })
	connection := &observedWriteConn{Conn: writer, writing: make(chan struct{})}
	f.peer.session.conn = connection
	f.peer.session.bufWriter = bufio.NewWriterSize(connection, 4096)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	route := f.route(t, 1, "192.0.2.79", 0, false)
	go func() { _, err := f.send(t, ctx, route); finished <- err }()
	select {
	case <-connection.writing:
	case err := <-finished:
		t.Fatalf("selected request returned before reaching flush: %v", err)
	case <-ctx.Done():
		t.Fatal("selected request never reached its socket")
	}
	cancel()
	select {
	case err := <-finished:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("selected request left a socket writer running")
	}
	require.Empty(t, f.peer.adjOut.routes, "an interrupted frame retires its session ownership")
}
