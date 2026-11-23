// Design: docs/architecture/update-building.md -- selected exclusion across framed SDK, mixed origins, refresh/stale replay, and RS
// Related: rfc4271_selected_advertisement_test.go -- internal-plugin ordinary selected exclusion witness
// Related: rfc4271_egress_exclusion_withdraw_proof_test.go -- general and RS relay exclusion controls
// Related: selected_refresh_socket_test.go -- public framed JSON socket RPC fixture
// RFC: rfc/short/rfc4271.md -- Section 9.1.3

package reactor

import (
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
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	policy "github.com/ze-software/ze/internal/component/policy"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func selectedExclusionReceivedRoute(t *testing.T, owner plugin.SelectedOwner, key []byte, revision uint64, sourceAddress string, localPref byte, staleLevel rpc.SelectedStaleLevel) rpc.SelectedRoute {
	t.Helper()
	address := netip.MustParseAddr(sourceAddress)
	admission, err := filterapi.NewPolicy(nil, policy.FinalAccept)
	require.NoError(t, err)
	peer := NewPeer(&PeerSettings{Address: address, LocalAS: 65000, PeerAS: 65000, ImportPolicy: admission})
	peer.state.Store(int32(PeerStateEstablished))
	peer.advanceForwardGeneration()
	source := relaySource{
		addr: address, strAdr: sourceAddress, srcID: peer.SourceID(),
		generation: peer.forwardGeneration.Load(), ok: true,
		info: forwardSourceInfo{
			resolved: true, peer: peer, remoteRouterID: 71, globalLocalAS: 65000,
			filterInfo: filterapi.PeerFilterInfo{Address: address, PeerAS: 65000, LocalAS: 65000},
		},
	}
	contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: false}))
	require.NoError(t, err)
	source.ctxID = contextID
	facts, err := newSourceReceiptFacts(&source, peer.newSession().connectionID)
	require.NoError(t, err)
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	lease, err := sourceReceipts.acquire(live.ID(), owner)
	require.NoError(t, err)
	t.Cleanup(func() {
		live.close()
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()
		require.NoError(t, lease.revoke(cleanup))
	})
	attrs := ordinarySelectedAttrs(sourceAddress, localPref)
	keyCopy := append([]byte(nil), key...)
	return rpc.SelectedRoute{
		Family:        family.IPv4Unicast.String(),
		KeyNLRI:       keyCopy,
		Revision:      revision,
		Kind:          rpc.SelectedRouteReceived,
		SourceReceipt: live.ID(),
		Retained:      staleLevel != rpc.SelectedFresh,
		StaleLevel:    staleLevel,
		Route: &rpc.StoredRoute{
			Family:            family.IPv4Unicast.String(),
			SourcePeer:        sourceAddress,
			MsgID:             19 + revision,
			SourcePolicyEpoch: peer.sourcePolicyEpoch.Load(),
			NLRIFraming:       rpc.NLRIFramingPrefixOnly,
			NLRIHex:           hex.EncodeToString(keyCopy),
			AttrHex:           hex.EncodeToString(attrs),
			NextHopHex:        hex.EncodeToString(address.AsSlice()),
		},
	}
}

func wireFrameTypesAndBodies(t *testing.T, raw []byte) ([]string, [][]byte) {
	t.Helper()
	var labels []string
	var updatePayloads [][]byte
	for len(raw) >= message.HeaderLen {
		length := int(binary.BigEndian.Uint16(raw[16:18]))
		require.GreaterOrEqual(t, length, message.HeaderLen)
		require.LessOrEqual(t, length, len(raw))
		msgType := msgtype.MessageType(raw[18])
		body := raw[message.HeaderLen:length]
		switch msgType {
		case msgtype.TypeUPDATE:
			labels = append(labels, "update")
			updatePayloads = append(updatePayloads, body)
		case msgtype.TypeROUTEREFRESH:
			if len(body) >= 4 {
				switch body[2] {
				case 1:
					labels = append(labels, "borr")
				case 2:
					labels = append(labels, "eorr")
				default:
					labels = append(labels, "refresh")
				}
			}
		default:
			labels = append(labels, "other")
		}
		raw = raw[length:]
	}
	require.Empty(t, raw)
	return labels, updatePayloads
}

// TestRFC4271FramedForkedSelectRoutesExclusionWithdrawsPriorRoute crosses the
// real framed JSON socket SDK transport (pkg/plugin/sdk/sdk_selected.go::
// Plugin.SelectRoutes -> engineMux.CallRPCCancellable -> opSelectRoutes ->
// reactorAPIAdapter.SelectRoutes -> Session.writeSelected/retireSelected ->
// BGP wire socket) with no in-process bridge. After advertising two routes,
// excluding one via export policy and withdrawing the sibling via
// SelectedRouteAbsent MUST both emit exact wire UPDATE withdrawals while an
// accepted sibling remains advertised.
//
// RFC requirement: RFC4271-9.2-3 positive -- across the framed forked
// Plugin.SelectRoutes socket boundary, excluding a previously advertised route
// (by export policy or absence) emits an UPDATE withdrawing exactly that
// prefix on the peer wire socket while accepted siblings stay announced.
func TestRFC4271FramedForkedSelectRoutesExclusionWithdrawsPriorRoute(t *testing.T) {
	fixture := newSelectedWriterFixture(t, false)
	fixture.api.r.orderedEgressSteps = buildOrderedEgressSteps()
	fixture.peer.settings.ExportPolicy = libraryPolicy(t, libraryTestExport, libraryTypeLP100, policy.FinalAccept)
	fixture.peer.refreshForwardFacts()

	work, stop := context.WithTimeout(t.Context(), 15*time.Second)
	defer stop()

	declaration := []rpc.DisseminationDecl{{
		Namespace:          "bgp",
		Role:               SelectedDisseminationRole,
		SourceReceive:      []string{"update-received", "state"},
		DestinationReceive: []string{"state", "refresh", "update-sent"},
		DestinationSend:    []string{"update"},
	}}
	var selMu sync.Mutex
	selCut := uint64(0)
	selCurrent := make(map[string]uint64)
	setCurrent := func(route rpc.SelectedRoute) {
		selMu.Lock()
		selCut = route.Revision
		if route.Kind == rpc.SelectedRouteAbsent {
			delete(selCurrent, string(route.KeyNLRI))
		} else {
			selCurrent[string(route.KeyNLRI)] = route.Revision
		}
		selMu.Unlock()
	}

	ready := make(chan *sdk.Plugin, 1)
	runnerDone := make(chan struct{})
	const implementation = "rfc4271-selected-exclusion-socket-runner"
	require.NoError(t, registry.Register(registry.Registration{
		Name:          implementation,
		Dissemination: declaration,
		CLIHandler:    func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			defer close(runnerDone)
			client := sdk.NewWithConn(selectedWriterProcess, selectedRefreshSocketConn{conn})
			defer func() { _ = client.Close() }()
			client.OnConfigure(func([]sdk.ConfigSection) error { return nil })
			client.OnCurrentSelection(func(_ context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
				selMu.Lock()
				defer selMu.Unlock()
				out := &rpc.CurrentSelectionOutput{
					Destination:        input.Destination,
					DestinationSession: input.DestinationSession,
					Cut:                selCut,
					Results:            make([]rpc.CurrentSelectionResult, len(input.Keys)),
				}
				for i, key := range input.Keys {
					res := rpc.CurrentSelectionResult{
						Family:   key.Family,
						KeyNLRI:  key.KeyNLRI,
						Revision: key.Revision,
						Presence: rpc.CurrentSelectionAbsent,
					}
					if rev := selCurrent[string(key.KeyNLRI)]; rev != 0 {
						res.Presence = rpc.CurrentSelectionPresent
						res.CurrentRevision = rev
					}
					out.Results[i] = res
				}
				return out, nil
			})
			client.OnStarted(func(context.Context) error { ready <- client; return nil })
			_ = client.Run(work, &sdk.Registration{Dissemination: declaration})
			return 0
		},
	}))

	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{
		ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{}),
		Plugins: []plugin.PluginConfig{{
			Name:     selectedWriterProcess,
			Run:      implementation,
			Internal: true,
			Encoder:  plugin.EncodingJSON,
		}},
	}, fixture.api)
	require.NoError(t, err)
	spawner := &selectedRefreshSpawner{ctx: work}
	host.SetProcessSpawner(spawner)
	host.UpdateDeliveryGraph("bgp", []pluginserver.DeliveryPeer{
		{Addr: "192.0.2.79", Bindings: []plugin.PeerProcessBinding{{
			PluginName: selectedWriterProcess,
			Explicit:   true,
			Receive:    map[string]events.Direction{"update": events.DirReceived, "state": events.DirBoth},
		}}},
		{Addr: fixture.peer.addrString, Bindings: []plugin.PeerProcessBinding{{
			PluginName: selectedWriterProcess,
			Explicit:   true,
			Receive:    map[string]events.Direction{"state": events.DirBoth, "refresh": events.DirBoth, "update": events.DirSent},
			Send:       map[string]bool{"update": true},
		}}},
	})
	fixture.api.r.api = host
	require.NoError(t, host.StartWithContext(work))
	t.Cleanup(func() {
		host.Stop()
		join, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()
		require.NoError(t, host.Wait(join))
		select {
		case <-runnerDone:
		case <-join.Done():
			t.Error("socket SDK runner did not join")
		}
	})
	require.NoError(t, host.WaitForStartupComplete(work))

	var client *sdk.Plugin
	select {
	case client = <-ready:
	case <-work.Done():
		t.Fatal("socket SDK never became ready")
	}
	proc := host.ProcessManager().GetProcess(selectedWriterProcess)
	require.NotNil(t, proc)
	require.False(t, proc.Conn().HasBridge(), "proof must cross framed forked JSON socket RPC")
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	fixture.peer.refreshForwardFacts()

	prefixA := netip.MustParsePrefix("198.18.70.0/24")
	prefixB := netip.MustParsePrefix("198.18.71.0/24")
	keyA := []byte{24, 198, 18, 70}
	keyB := []byte{24, 198, 18, 71}

	sendRoute := func(route rpc.SelectedRoute) *rpc.SelectRoutesOutput {
		t.Helper()
		setCurrent(route)
		out, callErr := client.SelectRoutes(work, rpc.SelectRoutesInput{
			Destination:        fixture.peer.addrString,
			DestinationSession: fixture.peer.session.initialReplay.Load(),
			Routes:             []rpc.SelectedRoute{route},
		})
		require.NoError(t, callErr)
		require.Len(t, out.Results, 1)
		return out
	}

	// Advertise A and sibling control B with accepted LOCAL_PREF 100.
	outA := sendRoute(selectedExclusionReceivedRoute(t, owner, keyA, 1, "192.0.2.79", 100, rpc.SelectedFresh))
	require.Equal(t, rpc.SelectedWritten, outA.Results[0].Outcome)
	outB := sendRoute(selectedExclusionReceivedRoute(t, owner, keyB, 2, "192.0.2.79", 100, rpc.SelectedFresh))
	require.Equal(t, rpc.SelectedWritten, outB.Results[0].Outcome)
	require.Len(t, updateBodies(t, fixture.conn.written()), 2)

	// Exclude A via export policy (LOCAL_PREF 200 rejected by libraryTypeLP100);
	// wire socket MUST withdraw A with empty NLRI while B stays advertised.
	outDeniedA := sendRoute(selectedExclusionReceivedRoute(t, owner, keyA, 3, "192.0.2.79", 200, rpc.SelectedFresh))
	require.Equal(t, rpc.SelectedSuppressed, outDeniedA.Results[0].Outcome)
	bodies := updateBodies(t, fixture.conn.written())
	require.Len(t, bodies, 3)
	require.Empty(t, updatePrefixes(t, bodies[2], false))
	require.Equal(t, []netip.Prefix{prefixA}, updatePrefixes(t, bodies[2], true))

	// Accepted sibling B continues to update over the same framed socket.
	outB2 := sendRoute(selectedExclusionReceivedRoute(t, owner, keyB, 4, "192.0.2.78", 100, rpc.SelectedFresh))
	require.Equal(t, rpc.SelectedWritten, outB2.Results[0].Outcome)
	bodies = updateBodies(t, fixture.conn.written())
	require.Len(t, bodies, 4)
	require.Equal(t, []netip.Prefix{prefixB}, updatePrefixes(t, bodies[3], false))
	require.Empty(t, updatePrefixes(t, bodies[3], true))

	// Explicit absence of B across Plugin.SelectRoutes MUST emit exact wire withdrawal.
	outAbsentB := sendRoute(rpc.SelectedRoute{
		Family:   family.IPv4Unicast.String(),
		KeyNLRI:  keyB,
		Revision: 5,
		Kind:     rpc.SelectedRouteAbsent,
	})
	require.Equal(t, rpc.SelectedAbsent, outAbsentB.Results[0].Outcome)
	bodies = updateBodies(t, fixture.conn.written())
	require.Len(t, bodies, 5)
	require.Empty(t, updatePrefixes(t, bodies[4], false))
	require.Equal(t, []netip.Prefix{prefixB}, updatePrefixes(t, bodies[4], true))
	require.Empty(t, fixture.peer.adjOut.routes)
}

// TestRFC4271SelectedMixedOriginExclusionWithdrawsPriorRoute proves that
// excluding a previously advertised route withdraws it from the peer socket
// across mixed SelectedRouteReceived and SelectedRouteOrigin transitions:
// received -> excluded local origin, local origin -> excluded received, and
// local origin -> excluded local origin, while an accepted sibling control
// remains advertised.
//
// RFC requirement: RFC4271-9.2-3 positive -- mixed-origin selected transitions
// (received to excluded local origin, local origin to excluded received, and
// local origin to excluded local origin) emit exact wire UPDATE withdrawals
// for the previously advertised prefix without suppressing accepted siblings.
func TestRFC4271SelectedMixedOriginExclusionWithdrawsPriorRoute(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	prefixA := netip.MustParsePrefix("198.18.7.0/24")
	prefixB := netip.MustParsePrefix("198.18.8.0/24")
	keyA := []byte{24, 198, 18, 7}
	keyB := []byte{24, 198, 18, 8}

	setFinalPolicy := func(final policy.Final) {
		t.Helper()
		var err error
		f.peer.settings.ExportPolicy, err = filterapi.NewPolicy(nil, final)
		require.NoError(t, err)
		f.peer.refreshForwardFacts()
	}

	// Phase 1: Advertise received route A, then replace with export-excluded local origin A.
	ans, err := f.send(t, t.Context(), selectedExclusionReceivedRoute(t, f.owner, keyA, 1, "192.0.2.79", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)
	require.Len(t, updateBodies(t, f.conn.written()), 1)

	setFinalPolicy(policy.FinalReject)
	originA1, _ := f.issue(t, selectedOriginBuildInput(), 2)
	ans, err = f.send(t, t.Context(), originA1)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, ans.Results[0].Outcome)
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 2, "excluded local origin must withdraw prior received advertisement")
	require.Empty(t, updatePrefixes(t, bodies[1], false))
	require.Equal(t, []netip.Prefix{prefixA}, updatePrefixes(t, bodies[1], true))
	require.Empty(t, f.peer.adjOut.routes)

	// Phase 2: Advertise local origin A, then replace with export-excluded received route A.
	setFinalPolicy(policy.FinalAccept)
	originA2, _ := f.issue(t, selectedOriginBuildInput(), 3)
	ans, err = f.send(t, t.Context(), originA2)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)
	require.Len(t, updateBodies(t, f.conn.written()), 3)

	setFinalPolicy(policy.FinalReject)
	ans, err = f.send(t, t.Context(), selectedExclusionReceivedRoute(t, f.owner, keyA, 4, "192.0.2.78", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, ans.Results[0].Outcome)
	bodies = updateBodies(t, f.conn.written())
	require.Len(t, bodies, 4, "excluded received route must withdraw prior local-origin advertisement")
	require.Empty(t, updatePrefixes(t, bodies[3], false))
	require.Equal(t, []netip.Prefix{prefixA}, updatePrefixes(t, bodies[3], true))
	require.Empty(t, f.peer.adjOut.routes)

	// Phase 3: Advertise local origin A and received sibling B, then exclude
	// newer local origin A while sibling B remains accepted.
	setFinalPolicy(policy.FinalAccept)
	originA3, _ := f.issue(t, selectedOriginBuildInput(), 5)
	ans, err = f.send(t, t.Context(), originA3)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)
	ans, err = f.send(t, t.Context(), selectedExclusionReceivedRoute(t, f.owner, keyB, 6, "192.0.2.79", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)
	require.Len(t, updateBodies(t, f.conn.written()), 6)

	setFinalPolicy(policy.FinalReject)
	originA4, _ := f.issue(t, selectedOriginBuildInput(), 7)
	ans, err = f.send(t, t.Context(), originA4)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, ans.Results[0].Outcome)
	bodies = updateBodies(t, f.conn.written())
	require.Len(t, bodies, 7, "excluded local origin must withdraw prior local-origin advertisement")
	require.Empty(t, updatePrefixes(t, bodies[6], false))
	require.Equal(t, []netip.Prefix{prefixA}, updatePrefixes(t, bodies[6], true))
	require.Len(t, f.peer.adjOut.routes, 1, "accepted sibling B must remain in Adj-RIB-Out")

	setFinalPolicy(policy.FinalAccept)
	ans, err = f.send(t, t.Context(), selectedExclusionReceivedRoute(t, f.owner, keyB, 8, "192.0.2.78", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)
	bodies = updateBodies(t, f.conn.written())
	require.Len(t, bodies, 8)
	require.Equal(t, []netip.Prefix{prefixB}, updatePrefixes(t, bodies[7], false))
}

// TestRFC4271SelectedRefreshAndStaleReplayExclusionWithdrawsPriorRoute proves
// that policy exclusion during GR stale replay, LLGR stale replay, and
// Enhanced Route Refresh (BoRR..EoRR) replay emits exact wire UPDATE
// withdrawals for excluded previously advertised routes while accepted
// siblings are re-advertised normally.
//
// RFC requirement: RFC4271-9.2-3 positive -- excluding a previously advertised
// route during GR stale replay, LLGR stale replay, or BoRR/EoRR route-refresh
// replay writes an exact wire withdrawal while accepted siblings stay announced.
func TestRFC4271SelectedRefreshAndStaleReplayExclusionWithdrawsPriorRoute(t *testing.T) {
	fixture, refreshInput := selectedRefreshFixture(t)
	fixture.api.r.orderedEgressSteps = buildOrderedEgressSteps()
	fixture.peer.settings.ExportPolicy = libraryPolicy(t, libraryTestExport, libraryTypeLP100, policy.FinalAccept)
	fixture.peer.refreshForwardFacts()

	prefixA := netip.MustParsePrefix("198.18.80.0/24")
	prefixB := netip.MustParsePrefix("198.18.81.0/24")
	keyA := []byte{24, 198, 18, 80}
	keyB := []byte{24, 198, 18, 81}

	// Initial fresh advertisements for A and B (LP 100, accepted).
	ans, err := fixture.send(t, t.Context(), selectedExclusionReceivedRoute(t, fixture.owner, keyA, 1, "192.0.2.79", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)
	ans, err = fixture.send(t, t.Context(), selectedExclusionReceivedRoute(t, fixture.owner, keyB, 2, "192.0.2.79", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)

	// GR stale replay where A is excluded by export policy (LP 200) -> exact withdrawal.
	ans, err = fixture.send(t, t.Context(), selectedExclusionReceivedRoute(t, fixture.owner, keyA, 3, "192.0.2.79", 200, rpc.SelectedStale))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, ans.Results[0].Outcome)
	_, updates := wireFrameTypesAndBodies(t, fixture.conn.written())
	require.Len(t, updates, 3)
	require.Empty(t, updatePrefixes(t, updates[2], false))
	require.Equal(t, []netip.Prefix{prefixA}, updatePrefixes(t, updates[2], true))

	// Accepted GR stale replay of sibling B stays announced.
	ans, err = fixture.send(t, t.Context(), selectedExclusionReceivedRoute(t, fixture.owner, keyB, 4, "192.0.2.79", 100, rpc.SelectedStale))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)

	// Re-advertise A (fresh, accepted), then exclude during LLGR stale replay -> exact withdrawal.
	ans, err = fixture.send(t, t.Context(), selectedExclusionReceivedRoute(t, fixture.owner, keyA, 5, "192.0.2.79", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)
	ans, err = fixture.send(t, t.Context(), selectedExclusionReceivedRoute(t, fixture.owner, keyA, 6, "192.0.2.79", 200, rpc.SelectedLLGR))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, ans.Results[0].Outcome)
	_, updates = wireFrameTypesAndBodies(t, fixture.conn.written())
	require.Len(t, updates, 6)
	require.Empty(t, updatePrefixes(t, updates[5], false))
	require.Equal(t, []netip.Prefix{prefixA}, updatePrefixes(t, updates[5], true))

	// Re-advertise A, then run an Enhanced Route Refresh bracket (BoRR ->
	// excluded A withdrawal + accepted B re-advertisement -> EoRR).
	ans, err = fixture.send(t, t.Context(), selectedExclusionReceivedRoute(t, fixture.owner, keyA, 7, "192.0.2.79", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)
	beforeRefresh := len(fixture.conn.written())

	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, refreshInput).Outcome)
	ans, err = fixture.send(t, t.Context(), selectedExclusionReceivedRoute(t, fixture.owner, keyA, 8, "192.0.2.79", 200, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedSuppressed, ans.Results[0].Outcome)
	ans, err = fixture.send(t, t.Context(), selectedExclusionReceivedRoute(t, fixture.owner, keyB, 9, "192.0.2.78", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)
	endInput := refreshInput
	endInput.Action = rpc.SelectedRefreshEnd
	require.Equal(t, rpc.SelectedRefreshWritten, selectedRefreshCall(t, fixture, endInput).Outcome)

	refreshBytes := fixture.conn.written()[beforeRefresh:]
	labels, refreshUpdates := wireFrameTypesAndBodies(t, refreshBytes)
	require.Equal(t, []string{"borr", "update", "update", "eorr"}, labels)
	require.Len(t, refreshUpdates, 2)
	require.Empty(t, updatePrefixes(t, refreshUpdates[0], false))
	require.Equal(t, []netip.Prefix{prefixA}, updatePrefixes(t, refreshUpdates[0], true))
	require.Equal(t, []netip.Prefix{prefixB}, updatePrefixes(t, refreshUpdates[1], false))
	require.Empty(t, updatePrefixes(t, refreshUpdates[1], true))
}

// TestRFC4271RouteServerExportExclusionWithdrawsPreviouslyAdvertisedRoute
// isolates the route-server forwarding rail (reactorForwardRS, rs=true only)
// so its export-rejection withdrawal conversion has independent mutant
// discrimination from the general forwarding rail.
//
// RFC requirement: RFC4271-9.2-3 positive -- on the route-server forwarding
// rail alone, rejecting a newer generation of a previously advertised route
// emits a wire UPDATE withdrawing exactly that prefix, while an accepted
// sibling generation remains announced with its updated MED.
func TestRFC4271RouteServerExportExclusionWithdrawsPreviouslyAdvertisedRoute(t *testing.T) {
	excluded := []byte{24, 10, 44, 0}
	control := []byte{24, 10, 44, 1}

	f := newAIGPReplayFixture(t, nil)
	rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(excluded, false, 10), true)
	rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(control, false, 11), true)
	initial := aigpSocketBodies(t, f.conn)
	require.Len(t, initial, 2)
	announced, err := message.UnpackUpdate(initial[0])
	require.NoError(t, err)
	require.Equal(t, excluded, announced.NLRI)
	require.Empty(t, announced.WithdrawnRoutes)

	exportLibraryAnswer(t, f.r, libraryReject, f.destination)
	rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(excluded, false, 20), true)
	afterReject := aigpSocketBodies(t, f.conn)
	require.Len(t, afterReject, 3, "route-server export rejection must withdraw the previously advertised route")
	withdrawn, err := message.UnpackUpdate(afterReject[2])
	require.NoError(t, err)
	require.Equal(t, excluded, withdrawn.WithdrawnRoutes)
	require.Empty(t, withdrawn.NLRI)
	require.Empty(t, withdrawn.PathAttributes)

	exportLibraryAnswer(t, f.r, medLibraryAnswer(88), f.destination)
	rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(control, false, 21), true)
	afterControl := aigpSocketBodies(t, f.conn)
	require.Len(t, afterControl, 4)
	latest, err := message.UnpackUpdate(afterControl[3])
	require.NoError(t, err)
	require.Equal(t, control, latest.NLRI)
	require.Empty(t, latest.WithdrawnRoutes)
	med := payloadMED(afterControl[3])
	require.True(t, med.present)
	require.Len(t, med.raw, 4)
	require.Equal(t, uint32(88), binary.BigEndian.Uint32(med.raw))
}

// TestRFC4271SelectedExclusionAllowsAcceptedControl is the negative control
// for the selected-route writer: routes accepted by export policy (both
// received and local-origin, fresh and stale) MUST be written as announcements
// and MUST NOT be suppressed or withdrawn.
//
// RFC requirement: RFC4271-9.2-3 negative -- accepted selected routes across
// received, local-origin, and stale replay inputs are advertised as UPDATE
// announcements without spurious suppression or withdrawal.
func TestRFC4271SelectedExclusionAllowsAcceptedControl(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	prefixOrigin := netip.MustParsePrefix("198.18.7.0/24")
	prefixRecv := netip.MustParsePrefix("198.18.8.0/24")
	keyRecv := []byte{24, 198, 18, 8}

	originRoute, _ := f.issue(t, selectedOriginBuildInput(), 1)
	ans, err := f.send(t, t.Context(), originRoute)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)

	ans, err = f.send(t, t.Context(), selectedExclusionReceivedRoute(t, f.owner, keyRecv, 2, "192.0.2.79", 100, rpc.SelectedFresh))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)

	ans, err = f.send(t, t.Context(), selectedExclusionReceivedRoute(t, f.owner, keyRecv, 3, "192.0.2.78", 100, rpc.SelectedStale))
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, ans.Results[0].Outcome)

	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 3)
	require.Equal(t, []netip.Prefix{prefixOrigin}, updatePrefixes(t, bodies[0], false))
	require.Empty(t, updatePrefixes(t, bodies[0], true))
	require.Equal(t, []netip.Prefix{prefixRecv}, updatePrefixes(t, bodies[1], false))
	require.Empty(t, updatePrefixes(t, bodies[1], true))
	require.Equal(t, []netip.Prefix{prefixRecv}, updatePrefixes(t, bodies[2], false))
	require.Empty(t, updatePrefixes(t, bodies[2], true))
	require.Len(t, f.peer.adjOut.routes, 2)
}
