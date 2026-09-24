package adj_rib_in

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rr"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Removing the RPKI policy releases a pending route, but not a predecessor
// already superseded by a known newer UPDATE. Neither action destroys bytes
// needed when validation is enabled again.
func TestRPKIDisablePreservesReceivedGenerations(t *testing.T) {
	r := newTestManager(t)
	_, _, err := r.handleCommand("request bgp adj-rib-in enable-validation", nil, "")
	require.NoError(t, err)
	body := rfc4271Announce(1,
		0, 0, 0, 0, 24, 203, 0, 113,
		0, 0, 0, 1, 24, 203, 0, 113)
	retainedReceive(t, r, 40, body)
	retainedDecision(t, r, "i", 0, 41)
	_, _, err = r.handleCommand("request bgp adj-rib-in disable-validation", nil, "")
	require.NoError(t, err)
	routes, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Equal(t, []rpc.StoredRoute{{
		SourcePeer: "192.0.2.1", Family: "ipv4/unicast", MsgID: 40, PathID: 1,
		AttrHex: hex.EncodeToString(body[4:18]), NextHopHex: "0a000001",
		NLRIHex: "18cb0071", NLRIFraming: rpc.NLRIFramingPrefixOnly, InitialUpdate: true,
	}}, routes, "only the unsuperseded pending sibling is released")
	// A queued decision after disable must not recreate its removed policy.
	retainedDecision(t, r, "i", 1, 40)
	afterLateDecision, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Equal(t, routes, afterLateDecision)
	newBody := rfc4271Announce(2, 0, 0, 0, 0, 24, 203, 0, 113)
	retainedReceive(t, r, 41, newBody)
	_, snapshot, err := r.handleCommand("request bgp adj-rib-in enable-validation", []string{"refresh"}, "")
	require.NoError(t, err)
	pending, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Empty(t, pending, "reload holds received routes until the new policy decides")
	snapshotBody, ok := snapshot.(map[string]any)
	require.True(t, ok, "snapshot is %T", snapshot)
	rows, ok := snapshotBody["routes"].([]map[string]any)
	require.True(t, ok, "routes are %T", snapshotBody["routes"])
	var latest map[string]any
	for _, row := range rows {
		if row["path-id"] == uint32(0) {
			latest = row
		}
	}
	require.NotNil(t, latest)
	require.Equal(t, uint64(41), latest["msg-id"])
	require.Equal(t, hex.EncodeToString(newBody[4:18]), latest["attr-hex"])
	retainedDecision(t, r, "a", 0, 41)
	recovered, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Equal(t, []rpc.StoredRoute{{
		SourcePeer: "192.0.2.1", Family: "ipv4/unicast", MsgID: 41,
		AttrHex: hex.EncodeToString(newBody[4:18]), NextHopHex: "0a000002",
		NLRIHex: "18cb0071", NLRIFraming: rpc.NLRIFramingPrefixOnly, InitialUpdate: true,
	}}, recovered, "re-enabled validation replays only the accepted current generation")
}

// A configured deadline controls fail-open for undecided routes; a rejected
// retained sibling stays unavailable when that deadline expires.
func TestValidationTimeoutControlsPendingReplay(t *testing.T) {
	r := newTestManager(t)
	_, _, err := r.handleCommand("request bgp adj-rib-in enable-validation",
		[]string{"refresh", "timeout", "60"}, "")
	require.NoError(t, err)
	retainedReceive(t, r, 40, rfc4271Announce(1,
		0, 0, 0, 0, 24, 203, 0, 113,
		0, 0, 0, 1, 24, 203, 0, 113))
	retainedDecision(t, r, "i", 1, 40)
	r.mu.Lock()
	for _, route := range r.pending {
		route.receivedAt = time.Now().Add(-45 * time.Second)
	}
	r.sweepExpiredPending()
	r.unlockValidation()
	held, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Empty(t, held, "the configured minute has not elapsed")

	_, _, err = r.handleCommand("request bgp adj-rib-in enable-validation",
		[]string{"timeout", "30"}, "")
	require.NoError(t, err)
	r.mu.Lock()
	r.sweepExpiredPending()
	r.unlockValidation()
	released, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Len(t, released, 1)
	require.Equal(t, uint32(0), released[0].PathID)
	require.Equal(t, uint64(40), released[0].MsgID)
}

// An optional store can retain a FlowSpec copy when the sender carries a next
// hop. That copy must not duplicate the mandatory selecting RIB's peer-up replay.
func TestReplayDelegatesFlowSpecToSelectingRIB(t *testing.T) {
	r := newTestManager(t)
	attrs := []byte{
		0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 254,
		0x80, 14, 15, 0, 1, 133, 4, 192, 0, 2, 254, 0, 5, 1, 24, 10, 0, 0,
	}
	body := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(attrs)))
	body = append(body, attrs...)
	body = append(body, 0, 0, 0, 0, 24, 198, 51, 100)
	retainedReceive(t, r, 42, body)
	_, snapshot, err := r.handleCommand("show bgp adj-rib-in status", nil, "")
	require.NoError(t, err)
	statusBody, ok := snapshot.(map[string]any)
	require.True(t, ok, "status is %T", snapshot)
	peerCounts, ok := statusBody["peers"].(map[string]int)
	require.True(t, ok, "peers are %T", statusBody["peers"])
	require.Equal(t, 2, peerCounts["192.0.2.1"],
		"both the unicast and FlowSpec routes must be present before replay")
	routes, _ := retainedRelay(t, r, "request bgp adj-rib-in replay", "192.0.2.99")
	require.Len(t, routes, 1)
	require.Equal(t, "ipv4/unicast", routes[0].Family)
	require.Equal(t, "18c63364", routes[0].NLRIHex)
	require.Equal(t, uint64(42), routes[0].MsgID)
}

func TestSelfOwnedPeerUpReplaysFlowSpecBeforeReady(t *testing.T) {
	for _, delivery := range []string{"structured", "event"} {
		for _, ownership := range []string{"standalone", "route-server", "route-reflector", "unheld-reflector"} {
			t.Run(delivery+"/"+ownership, func(t *testing.T) {
				const source, target = "192.0.2.1", "192.0.2.2"
				fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
				key := ribevents.ValidationRoute{Peer: netip.MustParseAddr(source), Family: fam,
					PathID: 7, NLRI: string([]byte{5, 1, 24, 10, 0, 0})}
				own := key
				own.Peer = netip.MustParseAddr(target)
				attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, 14, 11,
					0, 1, 133, 0, 0, 5, 1, 24, 10, 0, 0}
				ribevents.RegisterFlowSpecLookup(
					func(route ribevents.ValidationRoute, id uint64) bool {
						return (route == key || route == own) && id == 42
					},
					func(route ribevents.ValidationRoute) bool { return route == key || route == own },
					func(route ribevents.ValidationRoute) (ribevents.FlowSpecPath, bool) {
						return ribevents.FlowSpecPath{Attributes: attrs, MsgID: 42}, route == key || route == own
					},
					func() []ribevents.ValidationRoute { return []ribevents.ValidationRoute{key, own} })
				t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
				bridge := rpc.NewDirectBridge()
				pluginEnd, engineEnd := net.Pipe()
				t.Cleanup(func() { _ = engineEnd.Close() })
				plugin := sdk.NewWithConn("adj-rib-in-flowspec-test", rpc.NewBridgedConn(pluginEnd, bridge))
				t.Cleanup(func() { _ = plugin.Close() })
				var delivered []string
				bridge.SetRelayStoredRoute(func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
					if destination != target || len(routes) != 1 || routes[0].SourcePeer != source ||
						routes[0].MsgID != 42 || routes[0].PathID != 7 || routes[0].Withdraw ||
						routes[0].NLRIFraming != rpc.NLRIFramingPrefixOnly || routes[0].NLRIHex != "0501180a0000" ||
						routes[0].AttrHex != hex.EncodeToString(attrs) {
						t.Errorf("wrong authoritative FlowSpec replay: %s %+v", destination, routes)
					}
					delivered = append(delivered, "rule")
					return nil
				})
				bridge.SetDispatchCommand(func(_ context.Context, command string) (*rpc.DispatchCommandOutput, error) {
					if command != "request peer "+target+" plugin session ready" {
						t.Errorf("unexpected peer action %q", command)
					}
					delivered = append(delivered, "ready")
					return &rpc.DispatchCommandOutput{Status: statusDone}, nil
				})
				bridge.SetReady()
				r := newTestManager(t)
				r.plugin = plugin
				switch ownership {
				case "route-server":
					_, _, err := r.handleCommand("request bgp adj-rib-in claim-replay", nil, "")
					require.NoError(t, err)
				case "route-reflector", "unheld-reflector":
					// Feed the real RR registration through the same claim
					// decision used during the Stage-2 configure callback.
					claims := registry.ClaimsFor("bgp-rr")
					r.applyStartupClaims(func(role string) bool { return slices.Contains(claims, role) })
				}
				var unheld []string
				if ownership == "unheld-reflector" {
					unheld = []string{claimPeerUpReplay}
				}
				if delivery == "structured" {
					r.handleStructuredState(&rpc.StructuredEvent{
						PeerAddress: target, State: rpc.SessionStateUp, UnheldRoles: unheld,
					})
				} else {
					r.handleState(&bgp.Event{Type: "state", State: stateUp, UnheldRoles: unheld,
						Peer: mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: target, AS: 65001}})})
				}
				want := []string{"rule", "ready"}
				if ownership == "route-server" || ownership == "route-reflector" {
					want = []string{"ready"}
				}
				require.Equal(t, want, delivered, "exactly the peer's replay owner must finish before readiness")
			})
		}
	}
}

// A reflector loaded after the receive store must claim the existing replay
// owner through its real post-startup callback, not only its static registration.
func TestLateReflectorClaimsExistingReplayOwner(t *testing.T) {
	r := newTestManager(t)
	retainedReceive(t, r, 42, rfc4271Announce(1, 0, 0, 0, 0, 24, 203, 0, 113))
	var destinations []string
	r.routeRelayer = func(destination string, _ []rpc.StoredRoute) error {
		destinations = append(destinations, destination)
		return nil
	}
	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: "192.0.2.2", State: rpc.SessionStateUp})
	require.Equal(t, []string{"192.0.2.2"}, destinations, "the existing store initially owns replay")

	runner := registry.Lookup("bgp-rr")
	require.NotNil(t, runner)
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	t.Cleanup(func() {
		_ = pluginEnd.Close()
		_ = engineEnd.Close()
		_ = mux.Close()
	})
	finished := make(chan int, 1)
	go func() { finished <- runner.RunEngine(pluginEnd) }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request := func(method string) *rpc.Request {
		t.Helper()
		select {
		case req := <-mux.Requests():
			require.NotNil(t, req)
			require.Equal(t, method, req.Method)
			return req
		case <-ctx.Done():
			t.Fatal("reflector did not reach " + method)
			return nil
		}
	}
	req := request(rpc.MethodDeclareRegistration)
	require.NoError(t, mux.SendOK(ctx, req.ID))
	_, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", map[string]any{})
	require.NoError(t, err)
	req = request("ze-plugin-engine:declare-capabilities")
	require.NoError(t, mux.SendOK(ctx, req.ID))
	_, err = mux.CallRPC(ctx, "ze-plugin-callback:share-registry", map[string]any{})
	require.NoError(t, err)
	req = request("ze-plugin-engine:ready")
	require.NoError(t, mux.SendOK(ctx, req.ID))

	postStartup := make(chan error, 1)
	go func() {
		_, err := mux.CallRPC(ctx, "ze-plugin-callback:post-startup", map[string]any{})
		postStartup <- err
	}()
	req = request(rpc.MethodDispatchCommandArgs)
	var input rpc.DispatchCommandArgsInput
	require.NoError(t, json.Unmarshal(req.Params, &input))
	status, result, err := r.handleCommand(input.Command, input.Args, input.Peer)
	require.NoError(t, err)
	require.Equal(t, rpc.StatusDone, status)
	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.NoError(t, rpc.WriteDocumentAnswer(mux.AnswerWriter(ctx), req.ID, rpc.AnswerTail{}, data))
	select {
	case err := <-postStartup:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("reflector replay claim never completed")
	}

	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: "192.0.2.3", State: rpc.SessionStateUp})
	require.Equal(t, []string{"192.0.2.2"}, destinations,
		"an existing store must stand down after the late reflector takes ownership")
	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: "192.0.2.4", State: rpc.SessionStateUp,
		UnheldRoles: []string{claimPeerUpReplay}})
	require.Equal(t, []string{"192.0.2.2", "192.0.2.4"}, destinations,
		"late ownership remains scoped to peers the reflector can serve")

	_, err = mux.CallRPC(ctx, "ze-plugin-callback:bye", map[string]any{"reason": "test-complete"})
	require.NoError(t, err)
	select {
	case code := <-finished:
		require.Zero(t, code)
	case <-ctx.Done():
		t.Fatal("reflector did not stop")
	}
}
